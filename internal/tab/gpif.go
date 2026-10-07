package tab

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
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

type gpifDoc struct {
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
	doc, err := decodeGPIFHeader(data)
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
	return s, nil
}

// decodeGPIFHeader decodes only the top-level <Score> and <Tracks> elements
// and stops before the (large) bar/beat/note data that follows them.
func decodeGPIFHeader(data []byte) (gpifDoc, error) {
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
				err = dec.DecodeElement(&tracks, &el)
				doc.Tracks = tracks.Track
				return doc, err
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
