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
	"strings"
	"unicode/utf16"
)

// StdGuitar is standard tuning, highest string first, as Guitar Pro stores it.
var StdGuitar = []int{64, 59, 55, 50, 45, 40}

type GP3Track struct {
	Name    string
	Strings []int // MIDI notes, highest string first (as stored in the file)
	Drums   bool
}

type GP3Spec struct {
	Title, Artist, Album string
	Tempo                int
	Tracks               []GP3Track
}

// GP3 builds a Guitar Pro 3 file with a header and tracks but no bars.
func GP3(sp GP3Spec) []byte {
	var b bytes.Buffer
	i32 := func(v int) { binary.Write(&b, binary.LittleEndian, int32(v)) }
	byteSize := func(s string, size int) {
		b.WriteByte(byte(len(s)))
		f := make([]byte, size)
		copy(f, s)
		b.Write(f)
	}
	intByteSize := func(s string) {
		i32(len(s) + 1)
		byteSize(s, len(s))
	}
	byteSize("FICHIER GUITAR PRO v3.00", 30)
	intByteSize(sp.Title)
	intByteSize("") // subtitle
	intByteSize(sp.Artist)
	intByteSize(sp.Album)
	intByteSize("") // words
	intByteSize("") // copyright
	intByteSize("") // tab author
	intByteSize("") // instructions
	i32(0)          // notice lines
	b.WriteByte(0)  // triplet feel
	i32(sp.Tempo)
	i32(0) // key
	for range 64 {
		i32(0) // program
		b.Write(make([]byte, 8))
	}
	i32(0) // measures
	i32(len(sp.Tracks))
	for _, t := range sp.Tracks {
		flags := 0
		channel := 1
		if t.Drums {
			flags, channel = 1, 10
		}
		b.WriteByte(byte(flags))
		byteSize(t.Name, 40)
		i32(len(t.Strings))
		for i := range 7 {
			if i < len(t.Strings) {
				i32(t.Strings[i])
			} else {
				i32(0)
			}
		}
		i32(1) // port
		i32(channel)
		i32(0)
		i32(0)
		i32(0)
		i32(0)
	}
	return b.Bytes()
}

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

// GPIF builds a score.gpif (Guitar Pro 6/7); tempos are {bar (0-based), bpm} pairs.
func GPIF(title, artist, album string, tempos [][2]float64, tracks ...GPIFTrack) []byte {
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
	b.WriteString(`</Tracks><Bars/></GPIF>`)
	return []byte(b.String())
}

// GPX wraps a score.gpif as a Guitar Pro 7 file (a zip).
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
