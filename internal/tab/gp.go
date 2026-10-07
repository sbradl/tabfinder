package tab

import (
	"fmt"
	"regexp"
	"strconv"
)

var gpVersionRe = regexp.MustCompile(`(\d)\.(\d+)`)

// parseGP reads Guitar Pro 3, 4 and 5 binary files: header, track list and
// the note data (walked only for tempo changes).
func parseGP(b []byte) (*Song, error) {
	r := &reader{b: b}
	ver := r.byteSizeString(30)
	m := gpVersionRe.FindStringSubmatch(ver)
	if m == nil {
		return nil, fmt.Errorf("unrecognized Guitar Pro version %q", ver)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 3 || major > 5 {
		return nil, fmt.Errorf("Guitar Pro %d files are not supported", major)
	}
	s := &Song{Format: "gp" + m[1]}

	s.Title = r.intByteSizeString()
	r.intByteSizeString() // subtitle
	s.Artist = r.intByteSizeString()
	s.Album = r.intByteSizeString()
	r.intByteSizeString() // words
	if major == 5 {
		r.intByteSizeString() // music
	}
	r.intByteSizeString() // copyright
	r.intByteSizeString() // tab author
	r.intByteSizeString() // instructions
	for n := r.i32(); n > 0 && r.err == nil; n-- {
		r.intByteSizeString() // notice lines
	}
	if r.err != nil {
		return nil, r.err
	}

	var tempo int
	switch major {
	case 3:
		r.skip(1) // triplet feel
		tempo = r.i32()
		r.skip(4) // key
	case 4:
		r.skip(1) // triplet feel
		skipLyrics(r)
		tempo = r.i32()
		r.skip(4 + 1) // key, octave
	case 5:
		skipLyrics(r)
		if minor > 0 {
			r.skip(4 + 4 + 11) // RSE master effect: volume, unknown, equalizer
		}
		r.skip(2*4 + 4*4 + 4 + 2) // page size, margins, score size, header/footer flags
		for i := 0; i < 10; i++ {
			r.intByteSizeString() // header/footer templates
		}
		r.intByteSizeString() // tempo name
		tempo = r.i32()
		if minor > 0 {
			r.skip(1) // hide tempo
		}
		r.skip(1 + 4) // key, octave
	}

	// 64 MIDI channels: program (int32) + volume, balance, chorus, reverb, phaser, tremolo, 2 blank.
	var programs [64]int
	for i := range programs {
		programs[i] = r.i32()
		r.skip(8)
	}
	if major == 5 {
		r.skip(19*2 + 4) // musical directions, master reverb
	}
	measures, tracks := r.i32(), r.i32()
	if r.err != nil {
		return s, r.err
	}
	if measures < 0 || measures > 100000 || tracks < 1 || tracks > 128 {
		return s, fmt.Errorf("implausible measure/track count %d/%d", measures, tracks)
	}

	if tempo > 0 {
		s.addTempo(1, float64(tempo))
	}

	for i := 0; i < measures && r.err == nil; i++ {
		skipMeasureHeader(r, major, i)
	}
	stringCounts := make([]int, 0, tracks)
	for i := 0; i < tracks && r.err == nil; i++ {
		t, strings, err := readGPTrack(r, major, minor, i, &programs)
		if err != nil {
			return s, err
		}
		s.Tracks = append(s.Tracks, t)
		stringCounts = append(stringCounts, strings)
	}
	if major == 5 {
		if minor == 0 {
			r.skip(2)
		} else {
			r.skip(1)
		}
	}
	if r.err != nil {
		return s, r.err
	}
	if err := readGPMeasures(r, major, minor, measures, stringCounts, s); err != nil {
		return s, fmt.Errorf("tempo changes incomplete: %w", err)
	}
	return s, nil
}

func skipLyrics(r *reader) {
	r.skip(4) // track
	for i := 0; i < 5; i++ {
		r.skip(4) // starting measure
		r.intSizeString()
	}
}

func skipMeasureHeader(r *reader, major, index int) {
	if major == 5 && index > 0 {
		r.skip(1)
	}
	f := r.u8()
	if f&0x01 != 0 {
		r.skip(1) // numerator
	}
	if f&0x02 != 0 {
		r.skip(1) // denominator
	}
	if f&0x08 != 0 {
		r.skip(1) // repeat close
	}
	if f&0x10 != 0 && major < 5 {
		r.skip(1) // alternate ending (GP3/4 position)
	}
	if f&0x20 != 0 {
		r.intByteSizeString() // marker name
		r.skip(4)             // marker color
	}
	if f&0x40 != 0 {
		r.skip(2) // key signature
	}
	if major == 5 {
		if f&0x10 != 0 {
			r.skip(1) // alternate ending (GP5 position)
		}
		if f&0x03 != 0 {
			r.skip(4) // beam groups
		}
		if f&0x10 == 0 {
			r.skip(1)
		}
		r.skip(1) // triplet feel
	}
}

func readGPTrack(r *reader, major, minor, index int, programs *[64]int) (Track, int, error) {
	if major == 5 && (index == 0 || minor == 0) {
		r.skip(1)
	}
	flags := r.u8()
	t := Track{Name: r.byteSizeString(40)}
	stringCount := r.i32()
	var tuning [7]int
	for i := range tuning {
		tuning[i] = r.i32()
	}
	port, channel := r.i32(), r.i32()
	r.skip(4 + 4 + 4 + 4) // effect channel, fret count, capo, color
	if major == 5 {
		r.skip(2 + 1 + 1)   // flags2, auto accentuation, bank
		r.skip(1 + 12 + 12) // RSE humanize + unknown
		r.skip(4 + 4 + 4)   // RSE instrument, unknown, sound bank
		if minor == 0 {
			r.skip(2 + 1) // effect number
		} else {
			r.skip(4 + 4)         // effect number, equalizer
			r.intByteSizeString() // effect
			r.intByteSizeString() // effect category
		}
	}
	if r.err != nil {
		return t, 0, r.err
	}
	if stringCount < 1 || stringCount > 7 {
		return t, 0, fmt.Errorf("track %d: implausible string count %d", index+1, stringCount)
	}

	t.Drums = flags&0x01 != 0 || channel == 10
	if ch := (port-1)*16 + channel - 1; ch >= 0 && ch < 64 && !t.Drums {
		t.Instrument = gmInstrument(programs[ch])
	}
	if t.Drums {
		t.Instrument = "Drums"
		return t, stringCount, nil
	}
	// Stored highest string first; we report lowest first.
	for i := stringCount - 1; i >= 0; i-- {
		if tuning[i] < 0 || tuning[i] > 127 {
			return t, 0, fmt.Errorf("track %d: implausible tuning %v", index+1, tuning[:stringCount])
		}
		t.Pitches = append(t.Pitches, tuning[i])
	}
	t.Tuning = tuningName(t.Pitches)
	return t, stringCount, nil
}
