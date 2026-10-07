package tab

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"tabfinder/internal/score"
)

// parseGPX reads Guitar Pro 6 files: a BCFZ-compressed (or plain BCFS)
// sector filesystem that contains score.gpif.
func parseGPX(b []byte) (*Song, error) {
	data := b[4:]
	if bytes.HasPrefix(b, []byte("BCFZ")) {
		var err error
		if data, err = bcfzDecompress(data); err != nil {
			return nil, err
		}
		if !bytes.HasPrefix(data, []byte("BCFS")) {
			return nil, errors.New("gpx: decompressed data is not a BCFS container")
		}
		data = data[4:]
	}
	gpif, ok := bcfsFiles(data)["score.gpif"]
	if !ok {
		return nil, errors.New("gpx: no score.gpif in container")
	}
	return parseGPIF(gpif, FormatGP6)
}

// maxGPXSize caps the decompressed size a Guitar Pro 6 file claims; scores are a few MB at most.
const maxGPXSize = 256 << 20

func bcfzDecompress(b []byte) ([]byte, error) {
	if len(b) < 4 {
		return nil, errors.New("gpx: truncated header")
	}
	expected := int(binary.LittleEndian.Uint32(b))
	if expected <= 0 || expected > maxGPXSize {
		return nil, errors.New("gpx: implausible decompressed size")
	}
	br := &bitReader{b: b[4:]}
	out := make([]byte, 0, expected)
	for len(out) < expected && !br.eof {
		if br.bits(1) == 1 {
			// Back-reference into already decompressed output.
			wordSize := br.bits(4)
			offset := br.bitsReversed(wordSize)
			size := br.bitsReversed(wordSize)
			src := len(out) - offset
			if br.eof || offset == 0 || src < 0 {
				break
			}
			out = append(out, out[src:src+min(offset, size)]...)
		} else {
			// Literal bytes.
			for n := br.bitsReversed(2); n > 0 && !br.eof; n-- {
				out = append(out, byte(br.bits(8)))
			}
		}
	}
	if len(out) < expected {
		return nil, errors.New("gpx: truncated compressed data")
	}
	return out[:expected], nil
}

type bitReader struct {
	b   []byte
	pos int // bit position
	eof bool
}

func (br *bitReader) bit() int {
	if br.pos/8 >= len(br.b) {
		br.eof = true
		return 0
	}
	v := int(br.b[br.pos/8]>>(7-br.pos%8)) & 1
	br.pos++
	return v
}

// bits reads n bits, most significant first.
func (br *bitReader) bits(n int) int {
	v := 0
	for i := n - 1; i >= 0; i-- {
		v |= br.bit() << i
	}
	return v
}

// bitsReversed reads n bits, least significant first.
func (br *bitReader) bitsReversed(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v |= br.bit() << i
	}
	return v
}

// bcfsFiles extracts the files of a BCFS container: 4 KiB sectors, where a
// sector starting with int32 2 is a file entry listing its data sectors.
func bcfsFiles(data []byte) map[string][]byte {
	const sector = 0x1000
	le := func(off int) int {
		if off < 0 || off+4 > len(data) {
			return 0
		}
		return int(binary.LittleEndian.Uint32(data[off:]))
	}
	files := map[string][]byte{}
	for off := sector; off+0x94 <= len(data); off += sector {
		if le(off) != 2 {
			continue
		}
		name := data[off+4 : off+4+127]
		if i := bytes.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}
		size := le(off + 0x8C)
		var buf []byte
		if size > 0 && size <= len(data) {
			buf = make([]byte, 0, size+sector)
		}
		for p := off + 0x94; p+4 <= len(data); p += 4 {
			sec := le(p)
			start := sec * sector
			if sec == 0 || start >= len(data) {
				break
			}
			buf = append(buf, data[start:min(start+sector, len(data))]...)
		}
		if size <= len(buf) {
			buf = buf[:size]
		}
		files[string(name)] = buf
	}
	return files
}

type gpifProperty struct {
	Name    string `xml:"name,attr"`
	Pitches string `xml:"Pitches"`
}

type gpifTrack struct {
	Name          string `xml:"Name"`
	InstrumentSet struct {
		Name string `xml:"Name"`
		Type string `xml:"Type"`
	} `xml:"InstrumentSet"` // GP7
	Instrument struct {
		Ref string `xml:"ref,attr"`
	} `xml:"Instrument"` // GP6
	GeneralMidi struct {
		Table          string `xml:"table,attr"`
		Program        *int   `xml:"Program"`
		PrimaryChannel *int   `xml:"PrimaryChannel"`
	} `xml:"GeneralMidi"` // GP6
	Sounds []struct {
		Program *int `xml:"MIDI>Program"`
	} `xml:"Sounds>Sound"` // GP7
	Properties []gpifProperty `xml:"Properties>Property"` // GP6
	Staves     []struct {
		Properties []gpifProperty `xml:"Properties>Property"`
	} `xml:"Staves>Staff"` // GP7
}

type gpifAutomation struct {
	Type     string  `xml:"Type"`
	Bar      int     `xml:"Bar"` // 0-based
	Position float64 `xml:"Position"`
	Value    string  `xml:"Value"` // tempo: "<bpm> <beat unit>"
}

// The note data, read by readGPIFNotes: master bars list a bar per track, bars list
// voices, voices beats, beats notes and a rhythm, all by id (identical ones are shared).
// Lists of ids are as in the file: "0 1 2".
type (
	gpifMasterBar struct {
		Time   string // "4/4"
		Bars   string
		Repeat struct {
			Start, End bool
			Count      int // the times played
		}
		AlternateEndings string // "1 2"
		Section          string
	}
	gpifBar struct {
		ID     int
		Voices string
	}
	gpifVoice struct {
		ID    int
		Beats string
	}
	gpifBeat struct {
		ID         int
		Rhythm     struct{ Ref int }
		Notes      string
		Tremolo    string // tremolo picking: "1/8"
		GraceNotes string // a grace beat, before or on the next one
		Properties []gpifNoteProp
	}
	gpifNote struct {
		ID         int
		Tie        struct{ Destination bool } // tied from the note before
		Vibrato    string
		AntiAccent string // ghost note
		Accent     int    // 1 staccato, 4 heavy accent, 8 accent
		Trill      string
		Properties []gpifNoteProp
	}
	gpifNoteProp struct { // the numbers are -1 when missing
		Name               string
		String, Fret       int
		Number             int // Midi
		Element, Variation int // GP6 drums
		Enable             bool
		Flags              int
		HType              string
	}
	gpifRhythm struct {
		ID        int
		NoteValue string
		Dot       struct{ Count int }
		Tuplet    struct{ Num, Den int }
	}
)

type gpifDoc struct {
	MasterBars []gpifMasterBar
	Bars       []gpifBar
	Voices     []gpifVoice
	Beats      []gpifBeat
	Notes      []gpifNote
	Rhythms    []gpifRhythm
	NotesErr   error // the note data is broken

	Score struct {
		Title  string `xml:"Title"`
		Artist string `xml:"Artist"`
		Album  string `xml:"Album"`
	}
	MasterTrack struct {
		Automations []gpifAutomation `xml:"Automations>Automation"`
	}
	Tracks []gpifTrack
}

// parseGPIF reads the XML score shared by Guitar Pro 6 (.gpx) and 7+ (.gp).
func parseGPIF(data []byte, format Format) (*Song, error) {
	doc, err := decodeGPIF(data)
	if err != nil {
		return nil, err
	}
	s := &Song{
		Format: format,
		Title:  strings.TrimSpace(doc.Score.Title),
		Artist: strings.TrimSpace(doc.Score.Artist),
		Album:  strings.TrimSpace(doc.Score.Album),
	}
	for _, gt := range doc.Tracks {
		s.Tracks = append(s.Tracks, gpifToTrack(gt))
	}
	autos := doc.MasterTrack.Automations
	slices.SortStableFunc(autos, func(a, b gpifAutomation) int {
		return cmp.Or(cmp.Compare(a.Bar, b.Bar), cmp.Compare(a.Position, b.Position))
	})
	for _, a := range autos {
		if a.Type != "Tempo" {
			continue
		}
		if f := strings.Fields(a.Value); len(f) > 0 {
			if bpm, err := strconv.ParseFloat(f[0], 64); err == nil && bpm > 0 {
				s.addTempo(a.Bar+1, bpm)
			}
		}
	}
	if doc.NotesErr != nil {
		return s, fmt.Errorf("notes: %w", doc.NotesErr)
	}
	if len(doc.MasterBars) > 0 {
		s.notes = gpifNotes(&doc, s.Tracks)
	}
	return s, nil
}

// gpifNotes resolves the note data's ids into the notes of the tracks.
func gpifNotes(doc *gpifDoc, tracks []Track) *score.Score {
	byID := func(n int, id func(int) int) map[int]int {
		m := make(map[int]int, n)
		for i := range n {
			m[id(i)] = i
		}
		return m
	}
	bars := byID(len(doc.Bars), func(i int) int { return doc.Bars[i].ID })
	voices := byID(len(doc.Voices), func(i int) int { return doc.Voices[i].ID })
	beats := byID(len(doc.Beats), func(i int) int { return doc.Beats[i].ID })
	notes := byID(len(doc.Notes), func(i int) int { return doc.Notes[i].ID })
	rhythms := byID(len(doc.Rhythms), func(i int) int { return doc.Rhythms[i].ID })

	sc := &score.Score{Tracks: make([]score.Track, len(tracks))}
	for i, t := range tracks {
		sc.Tracks[i] = score.Track{Drums: t.Drums, Bars: make([][]score.Beat, 0, len(doc.MasterBars))}
	}
	for _, mb := range doc.MasterBars {
		bar := score.Bar{Num: 4, Den: 4}
		if n, d, ok := strings.Cut(mb.Time, "/"); ok {
			bar.Num, bar.Den = natoi(strings.TrimSpace(n)), natoi(strings.TrimSpace(d))
		}
		bar.RepeatOpen = mb.Repeat.Start
		if mb.Repeat.End {
			bar.Repeats = max(mb.Repeat.Count-1, 1)
		}
		for _, e := range strings.Fields(mb.AlternateEndings) {
			if n := natoi(e); n >= 1 && n <= 8 {
				bar.Alternate |= 1 << (n - 1)
			}
		}
		bar.Marker = strings.TrimSpace(mb.Section)
		sc.Bars = append(sc.Bars, bar)
		barIDs := strings.Fields(mb.Bars)
		for ti := range sc.Tracks {
			var out []score.Beat
			if ti < len(barIDs) {
				if bi, ok := bars[natoi(barIDs[ti])]; ok {
					for v, vid := range strings.Fields(doc.Bars[bi].Voices) {
						vi, ok := voices[natoi(vid)]
						if !ok {
							continue
						}
						start := 0
						for _, bid := range strings.Fields(doc.Voices[vi].Beats) {
							bi, ok := beats[natoi(bid)]
							if !ok {
								continue
							}
							gb := &doc.Beats[bi]
							b := score.Beat{Start: start, Voice: v, Fx: gpifBeatFx(gb)}
							if ri, ok := rhythms[gb.Rhythm.Ref]; ok {
								b.Dur, b.Tuplet = gpifDur(&doc.Rhythms[ri])
							}
							grace := gb.GraceNotes != ""
							if !grace { // a grace beat takes its time from the beat it's played with
								start += b.Dur
							}
							for _, nid := range strings.Fields(gb.Notes) {
								if ni, ok := notes[natoi(nid)]; ok {
									n := gpifToNote(&doc.Notes[ni], tracks[ti].Drums)
									if grace {
										n.Fx |= score.Grace
									}
									b.Notes = append(b.Notes, n)
								}
							}
							if len(b.Notes) > 0 {
								slices.SortFunc(b.Notes, func(x, y score.Note) int { return cmp.Or(cmp.Compare(x.String, y.String), cmp.Compare(x.Fret, y.Fret)) })
								out = append(out, b)
							}
						}
					}
				}
			}
			slices.SortStableFunc(out, func(a, b score.Beat) int { return cmp.Compare(a.Start, b.Start) })
			sc.Tracks[ti].Bars = append(sc.Tracks[ti].Bars, out)
		}
	}
	return sc
}

var gpifNoteValues = map[string]int{"Whole": -2, "Half": -1, "Quarter": 0, "Eighth": 1, "16th": 2, "32nd": 3, "64th": 4, "128th": 5}

// gpifDur is a rhythm's length and its tuplet's n.
func gpifDur(r *gpifRhythm) (dur, tuplet int) {
	v, ok := gpifNoteValues[r.NoteValue]
	if !ok {
		v = 0
	}
	dur = score.DurOf(v)
	for i, add := 0, dur/2; i < r.Dot.Count && i < 3; i, add = i+1, add/2 {
		dur += add
	}
	if n, d := r.Tuplet.Num, r.Tuplet.Den; n > 1 && d > 0 {
		dur, tuplet = dur*d/n, n
	}
	return dur, tuplet
}

// gpifDrums is the MIDI note of Guitar Pro 6's drum kit elements, by element and variation.
var gpifDrums = [][]int{
	{36},             // kick
	{38, 38, 37},     // snare: hit, rim shot, side stick
	{56}, {56}, {56}, // cowbells
	{41}, {45}, {47}, {48}, {50}, // toms, low to high
	{42, 46, 46},     // hi-hat: closed, half open, open
	{44},             // pedal hi-hat
	{57}, {49}, {55}, // crashes, splash
	{51, 59, 53}, // ride: middle, edge, bell
	{52},         // china
}

func gpifToNote(gn *gpifNote, drums bool) score.Note {
	n := gpifNoteFx(gn)
	midi, element, variation := -1, -1, 0
	for _, p := range gn.Properties {
		switch {
		case p.Name == "String" && p.String >= 0:
			n.String = p.String
		case p.Name == "Fret" && p.Fret >= 0:
			n.Fret = p.Fret
		case p.Name == "Midi" && p.Number >= 0:
			midi = p.Number
		case p.Name == "Element" && p.Element >= 0:
			element = p.Element
		case p.Name == "Variation" && p.Variation >= 0:
			variation = p.Variation
		}
	}
	if drums {
		n.String = 0
		switch {
		case midi >= 0:
			n.Fret = midi
		case element >= 0 && element < len(gpifDrums):
			vs := gpifDrums[element]
			n.Fret = vs[min(max(variation, 0), len(vs)-1)]
		}
	}
	return n
}

// gpifNoteFx is a note's kind and techniques.
func gpifNoteFx(gn *gpifNote) score.Note {
	n := score.Note{Tie: gn.Tie.Destination, Ghost: gn.AntiAccent != ""}
	set := func(on bool, fx score.Fx) {
		if on {
			n.Fx |= fx
		}
	}
	set(gn.Vibrato != "", score.Vibrato)
	set(gn.Accent&0x01 != 0, score.Staccato)
	set(gn.Accent&0x0C != 0, score.Accent)
	set(gn.Trill != "", score.Trill)
	for _, p := range gn.Properties {
		switch p.Name {
		case "Muted":
			n.Dead = p.Enable
		case "Bended":
			set(p.Enable, score.Bend)
		case "HopoOrigin", "HopoDestination":
			set(p.Enable, score.Legato)
		case "PalmMuted":
			set(p.Enable, score.PalmMute)
		case "Tapped", "LeftHandTapped":
			set(p.Enable, score.Tap)
		case "Slide":
			set(p.Flags != 0, score.Slide)
		case "HarmonicType":
			set(p.HType != "", score.Harmonic)
		}
	}
	return n
}

// gpifBeatFx is the techniques of a whole beat.
func gpifBeatFx(gb *gpifBeat) score.Fx {
	var fx score.Fx
	if gb.Tremolo != "" {
		fx |= score.TremoloPicking
	}
	for _, p := range gb.Properties {
		if (p.Name == "Slapped" || p.Name == "Popped") && p.Enable {
			fx |= score.Slap
		}
	}
	return fx
}

// decodeGPIF decodes the top-level elements the song and its notes are read from. The
// notes are expected after the tracks, where Guitar Pro writes them.
func decodeGPIF(data []byte) (gpifDoc, error) {
	var doc gpifDoc
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return doc, nil
		}
		if err != nil {
			return doc, err
		}
		switch el := tok.(type) {
		case xml.EndElement:
			depth--
		case xml.StartElement:
			if depth == 0 { // root <GPIF>
				depth++
				continue
			}
			switch el.Name.Local {
			case "Score":
				err = dec.DecodeElement(&doc.Score, &el)
			case "MasterTrack":
				err = dec.DecodeElement(&doc.MasterTrack, &el)
			case "Tracks":
				var tracks struct {
					Track []gpifTrack `xml:"Track"`
				}
				if err = dec.DecodeElement(&tracks, &el); err != nil {
					return doc, err
				}
				doc.Tracks = tracks.Track
				// The notes follow; they are read faster without encoding/xml.
				if off := int(dec.InputOffset()); off <= len(data) {
					doc.NotesErr = readGPIFNotes(data[off:], &doc)
				}
				return doc, nil
			default:
				err = dec.Skip()
			}
			if err != nil {
				return doc, err
			}
		}
	}
}

func gpifToTrack(gt gpifTrack) Track {
	t := Track{Name: strings.TrimSpace(gt.Name)}
	gm := gt.GeneralMidi
	t.Drums = strings.EqualFold(gt.InstrumentSet.Type, "drumKit") ||
		strings.Contains(strings.ToLower(gt.Instrument.Ref), "drum") ||
		strings.EqualFold(gm.Table, "Percussion") ||
		(gm.PrimaryChannel != nil && *gm.PrimaryChannel == 9)
	switch {
	case t.Drums:
		t.Instrument = "Drums"
		return t
	case gt.InstrumentSet.Name != "":
		t.Instrument = gt.InstrumentSet.Name
	case gm.Program != nil:
		t.Instrument = gmInstrument(*gm.Program)
	case len(gt.Sounds) > 0 && gt.Sounds[0].Program != nil:
		t.Instrument = gmInstrument(*gt.Sounds[0].Program)
	default:
		t.Instrument = gt.Instrument.Ref
	}

	props := gt.Properties
	for _, st := range gt.Staves {
		props = append(props, st.Properties...)
	}
	for _, p := range props {
		if p.Name != "Tuning" {
			continue
		}
		for _, f := range strings.Fields(p.Pitches) {
			if n, err := strconv.Atoi(f); err == nil {
				t.Pitches = append(t.Pitches, n)
			}
		}
		if len(t.Pitches) > 1 && t.Pitches[0] > t.Pitches[len(t.Pitches)-1] {
			slices.Reverse(t.Pitches)
		}
		break
	}
	return t
}
