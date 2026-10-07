// Package tabfiles builds small synthetic tab files (Guitar Pro 3, TuxGuitar,
// Power Tab, zip, GPIF, TuxGuitar 2) for tests, so parsing and end-to-end tests
// need no copyrighted fixtures. It does not import package tab: the files are
// plain bytes, and package tab's own tests use it too.
package tabfiles

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
)

// StdGuitar is standard tuning, highest string first, as Guitar Pro stores it.
var StdGuitar = []int{64, 59, 55, 50, 45, 40}

// TuxGuitarMagic starts every TuxGuitar 1 file.
const TuxGuitarMagic = "TuxGuitar File Format"

// TG1 builds the header of a TuxGuitar 1 file.
func TG1(title, artist, album string) []byte {
	var b bytes.Buffer
	for _, s := range []string{TuxGuitarMagic + " - 1.5", title, artist, album} {
		u := utf16.Encode([]rune(s))
		b.WriteByte(byte(len(u)))
		binary.Write(&b, binary.BigEndian, u)
	}
	return b.Bytes()
}

// PTB builds the header of a Power Tab 2.x song file (strings are MFC
// CStrings). A nil album is a release that isn't a public audio release.
func PTB(version int, title, artist string, album *string) []byte {
	var b bytes.Buffer
	b.WriteString("ptab")
	binary.Write(&b, binary.LittleEndian, uint16(version))
	binary.Write(&b, binary.LittleEndian, uint16(0)) // song file
	b.WriteByte(0)                                   // content type
	for _, s := range []string{title, artist} {
		b.WriteByte(byte(len(s)))
		b.WriteString(s)
	}
	if album == nil {
		b.WriteByte(1)
		return b.Bytes()
	}
	b.WriteByte(0) // public audio release
	b.WriteByte(0) // release type
	b.WriteByte(byte(len(*album)))
	b.WriteString(*album)
	return b.Bytes()
}

// Zip builds an archive; order, if given, lists the entries in file order.
func Zip(files map[string][]byte, order ...string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if len(order) == 0 {
		for name := range files {
			order = append(order, name)
		}
	}
	for _, name := range order {
		w, _ := zw.Create(name)
		w.Write(files[name])
	}
	zw.Close()
	return buf.Bytes()
}

type GPIFTrack struct {
	Name, Instrument, Kind string
	Pitches                string // space-separated MIDI notes, as written in the file
}

// GPIF builds a score.gpif (Guitar Pro 6/7) without notes; tempos are {bar (0-based), bpm} pairs.
func GPIF(title, artist, album string, tempos [][2]float64, tracks ...GPIFTrack) []byte {
	return GPIFScore(title, artist, album, tempos, nil, tracks...)
}

// GPIFScore is GPIF with bars and notes. Note strings count from the highest, as in GPBar.
func GPIFScore(title, artist, album string, tempos [][2]float64, bars []GPBar, tracks ...GPIFTrack) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?><GPIF><GPRevision>1</GPRevision><Score><Title>%s</Title><Artist>%s</Artist><Album>%s</Album></Score><MasterTrack><Automations>`, title, artist, album)
	for _, t := range tempos {
		fmt.Fprintf(&b, `<Automation><Type>Tempo</Type><Bar>%d</Bar><Position>0</Position><Value>%g 2</Value></Automation>`, int(t[0]), t[1])
	}
	b.WriteString(`</Automations></MasterTrack><Tracks>`)
	for _, t := range tracks {
		fmt.Fprintf(&b, `<Track><Name>%s</Name><InstrumentSet><Name>%s</Name><Type>%s</Type></InstrumentSet>`, t.Name, t.Instrument, t.Kind)
		if t.Pitches != "" {
			fmt.Fprintf(&b, `<Staves><Staff><Properties><Property name="Tuning"><Pitches>%s</Pitches></Property></Properties></Staff></Staves>`, t.Pitches)
		}
		b.WriteString(`</Track>`)
	}
	b.WriteString(`</Tracks>`)
	if bars == nil {
		b.WriteString(`<Bars/></GPIF>`)
		return []byte(b.String())
	}
	strs := make([]int, len(tracks))
	for i, t := range tracks {
		strs[i] = len(strings.Fields(t.Pitches))
	}
	gpifBars(&b, bars, strs, false)
	b.WriteString(`</GPIF>`)
	return []byte(b.String())
}

// gpifBars writes the MasterBars, Bars, Voices, Beats, Notes and Rhythms of a score.gpif
// for tracks with these string counts (0 for drums). Identical beats, notes and
// rhythms are written once and referred to by id, as Guitar Pro does.
func gpifBars(b *strings.Builder, bars []GPBar, strs []int, gp6 bool) {
	type ids struct {
		list  []string
		index map[string]int
	}
	intern := func(t *ids, xml string) int {
		if t.index == nil {
			t.index = map[string]int{}
		}
		if i, ok := t.index[xml]; ok {
			return i
		}
		t.index[xml] = len(t.list)
		t.list = append(t.list, xml)
		return len(t.list) - 1
	}
	var barX, voiceX, beatX, noteX, rhythmX ids
	values := map[int]string{-2: "Whole", -1: "Half", 0: "Quarter", 1: "Eighth", 2: "16th", 3: "32nd", 4: "64th"}
	voice := func(beats []GPBeat, nstr int) int {
		var beatIDs []string
		for _, gb := range beats {
			r := fmt.Sprintf(`<NoteValue>%s</NoteValue>`, values[gb.Dur])
			if gb.Dotted {
				r += `<AugmentationDot count="1"/>`
			}
			if gb.Tuplet != 0 {
				den := 2
				for den*2 < gb.Tuplet {
					den *= 2
				}
				r += fmt.Sprintf(`<PrimaryTuplet num="%d" den="%d"/>`, gb.Tuplet, den)
			}
			beat := fmt.Sprintf(`<Rhythm ref="%d"/>`, intern(&rhythmX, r))
			var noteIDs []string
			tremolo := false
			for _, n := range gb.Notes {
				var note strings.Builder
				if n.Tie {
					note.WriteString(`<Tie origin="false" destination="true"/>`)
				}
				if n.Vibrato {
					note.WriteString(`<Vibrato>Slight</Vibrato>`)
				}
				if n.Ghost {
					note.WriteString(`<AntiAccent>Normal</AntiAccent>`)
				}
				accent := 0
				if n.Staccato {
					accent |= 1
				}
				if n.Accent {
					accent |= 8
				}
				if accent != 0 {
					fmt.Fprintf(&note, `<Accent>%d</Accent>`, accent)
				}
				switch {
				case nstr > 0:
					fmt.Fprintf(&note, `<Properties><Property name="String"><String>%d</String></Property><Property name="Fret"><Fret>%d</Fret></Property>`, nstr-n.String, n.Fret)
				case gp6:
					fmt.Fprintf(&note, `<Properties><Property name="Element"><Element>%d</Element></Property><Property name="Variation"><Variation>%d</Variation></Property>`, n.Element, n.Variation)
				default: // as Guitar Pro 7 writes drums, with the MIDI note as the fret too
					fmt.Fprintf(&note, `<InstrumentArticulation>0</InstrumentArticulation><Properties><Property name="Fret"><Fret>%d</Fret></Property><Property name="Midi"><Number>%d</Number></Property><Property name="String"><String>4</String></Property>`, n.Fret, n.Fret)
				}
				for name, on := range map[string]bool{"Muted": n.Dead, "Bended": n.Bend, "HopoOrigin": n.Legato, "PalmMuted": n.PalmMute, "Tapped": gb.Tap} {
					if on {
						fmt.Fprintf(&note, `<Property name="%s"><Enable/></Property>`, name)
					}
				}
				if n.Slide {
					note.WriteString(`<Property name="Slide"><Flags>2</Flags></Property>`)
				}
				if n.Harmonic {
					note.WriteString(`<Property name="HarmonicType"><HType>Natural</HType></Property>`)
				}
				note.WriteString(`</Properties>`)
				if n.Trill {
					note.WriteString(`<Trill>9</Trill>`)
				}
				tremolo = tremolo || n.Tremolo
				noteIDs = append(noteIDs, fmt.Sprint(intern(&noteX, note.String())))
			}
			if len(noteIDs) > 0 {
				beat += `<Notes>` + strings.Join(noteIDs, " ") + `</Notes>`
			}
			if tremolo {
				beat += `<Tremolo>1/8</Tremolo>`
			}
			if slices.ContainsFunc(gb.Notes, func(n GPNote) bool { return n.Grace }) {
				beat += `<GraceNotes>BeforeBeat</GraceNotes>` // GPIF: a beat of its own
			}
			if gb.Slap {
				beat += `<Properties><Property name="Slapped"><Enable/></Property></Properties>`
			}
			beatIDs = append(beatIDs, fmt.Sprint(intern(&beatX, beat)))
		}
		return intern(&voiceX, `<Beats>`+strings.Join(beatIDs, " ")+`</Beats>`)
	}

	b.WriteString(`<MasterBars>`)
	num, den := 4, 4 // GPIF has the time signature in every bar
	for _, gb := range bars {
		b.WriteString(`<MasterBar>`)
		if gb.Num != 0 {
			num, den = gb.Num, gb.Den
		}
		fmt.Fprintf(b, `<Time>%d/%d</Time>`, num, den)
		if gb.RepeatOpen || gb.Repeats > 0 {
			count := 0
			if gb.Repeats > 0 {
				count = gb.Repeats + 1 // the times played
			}
			fmt.Fprintf(b, `<Repeat start="%t" end="%t" count="%d"/>`, gb.RepeatOpen, gb.Repeats > 0, count)
		}
		if gb.Alternate != 0 {
			var endings []string
			for n := 1; n <= 8; n++ {
				if gb.Alternate&(1<<(n-1)) != 0 {
					endings = append(endings, fmt.Sprint(n))
				}
			}
			fmt.Fprintf(b, `<AlternateEndings>%s</AlternateEndings>`, strings.Join(endings, " "))
		}
		if gb.Marker != "" {
			fmt.Fprintf(b, `<Section><Letter><![CDATA[A]]></Letter><Text><![CDATA[%s]]></Text></Section>`, gb.Marker)
		}
		var barIDs []string
		for t, s := range strs {
			voices := []string{"-1", "-1", "-1", "-1"}
			if t < len(gb.Beats) && gb.Beats[t] != nil {
				voices[0] = fmt.Sprint(voice(gb.Beats[t], s))
			}
			if t < len(gb.Voice2) && gb.Voice2[t] != nil {
				voices[1] = fmt.Sprint(voice(gb.Voice2[t], s))
			}
			barIDs = append(barIDs, fmt.Sprint(intern(&barX, `<Voices>`+strings.Join(voices, " ")+`</Voices>`)))
		}
		fmt.Fprintf(b, `<Bars>%s</Bars></MasterBar>`, strings.Join(barIDs, " "))
	}
	b.WriteString(`</MasterBars>`)
	for _, sec := range []struct {
		name string
		t    *ids
	}{{"Bar", &barX}, {"Voice", &voiceX}, {"Beat", &beatX}, {"Note", &noteX}, {"Rhythm", &rhythmX}} {
		fmt.Fprintf(b, `<%ss>`, sec.name)
		for i, x := range sec.t.list {
			fmt.Fprintf(b, `<%s id="%d">%s</%s>`, sec.name, i, x, sec.name)
		}
		fmt.Fprintf(b, `</%ss>`, sec.name)
	}
}

// GP7 wraps a score.gpif as a Guitar Pro 7 file (a zip).
func GP7(gpif []byte) []byte { return Zip(map[string][]byte{"Content/score.gpif": gpif}) }

type TG2Track struct {
	Name    string
	Channel int
	Strings []int // highest string first
}

type TG2Channel struct{ ID, Bank, Program int }

// TG2 builds a TuxGuitar 2 content.xml. The root element is a wrapper around
// <TGSong>, which is how the parser reads it.
func TG2(name, artist, album string, tempos []int, channels []TG2Channel, tracks ...TG2Track) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0"?><Root><TGSong><name>%s</name><artist>%s</artist><album>%s</album>`, name, artist, album)
	for _, c := range channels {
		fmt.Fprintf(&b, `<TGChannel><id>%d</id><bank>%d</bank><program>%d</program></TGChannel>`, c.ID, c.Bank, c.Program)
	}
	for _, t := range tempos {
		fmt.Fprintf(&b, `<TGMeasureHeader><tempo>%d</tempo></TGMeasureHeader>`, t)
	}
	for _, t := range tracks {
		fmt.Fprintf(&b, `<TGTrack><name>%s</name><channelId>%d</channelId>`, t.Name, t.Channel)
		for _, s := range t.Strings {
			fmt.Fprintf(&b, `<TGString>%d</TGString>`, s)
		}
		b.WriteString(`</TGTrack>`)
	}
	b.WriteString(`</TGSong></Root>`)
	return []byte(b.String())
}
