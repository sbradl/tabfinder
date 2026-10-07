package tabfiles

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// GPTrack is a track of a Guitar Pro 3-5 file built by GP.
type GPTrack struct {
	Name    string
	Strings []int // MIDI notes, highest string first (as stored in the file)
	Drums   bool
	Program int // General MIDI program of the track's channel
	Channel int // 1-16 on port 1; 0 picks the next free one (10 for drums)
}

// GPBar is a bar of a Guitar Pro 3-5 file built by GP.
type GPBar struct {
	Tempo int // BPM it changes to at the start of the bar through a mix-table change; 0 for none
}

// GPSpec describes a Guitar Pro 3, 4 or 5 file with bars and notes.
type GPSpec struct {
	Version              string // "3.00", "4.06", "5.00" or "5.10"
	Title, Artist, Album string
	Tempo                int
	Tracks               []GPTrack
	Bars                 []GPBar
}

// GP builds a Guitar Pro 3-5 file: header, tracks and, per bar and track, two beats
// (a chord of two notes, or a rest) after a text beat in the first bar. The
// first track's first beat of a bar carries that bar's tempo change.
func GP(sp GPSpec) []byte {
	var major, minor int
	if _, err := fmt.Sscanf(sp.Version, "%d.%d", &major, &minor); err != nil || major < 3 || major > 5 {
		panic("tabfiles.GP: bad version " + sp.Version)
	}
	var b bytes.Buffer
	u8 := func(v int) { b.WriteByte(byte(v)) }
	i32 := func(v int) { binary.Write(&b, binary.LittleEndian, int32(v)) }
	zeros := func(n int) { b.Write(make([]byte, n)) }
	byteSize := func(s string, size int) {
		u8(len(s))
		f := make([]byte, size)
		copy(f, s)
		b.Write(f)
	}
	intByteSize := func(s string) {
		i32(len(s) + 1)
		byteSize(s, len(s))
	}
	lyrics := func() {
		i32(0)
		for range 5 {
			i32(0)
			i32(0) // empty text
		}
	}

	byteSize("FICHIER GUITAR PRO v"+sp.Version, 30)
	intByteSize(sp.Title)
	intByteSize("") // subtitle
	intByteSize(sp.Artist)
	intByteSize(sp.Album)
	intByteSize("") // words
	if major == 5 {
		intByteSize("") // music
	}
	intByteSize("") // copyright
	intByteSize("") // tab author
	intByteSize("") // instructions
	i32(2)          // notice lines
	intByteSize("made up for the tests")
	intByteSize("not a real song")
	switch major {
	case 3:
		u8(0) // triplet feel
		i32(sp.Tempo)
		i32(0) // key
	case 4:
		u8(0)
		lyrics()
		i32(sp.Tempo)
		i32(0) // key
		u8(0)  // octave
	case 5:
		lyrics()
		if minor > 0 {
			zeros(4 + 4 + 11) // RSE master effect
		}
		zeros(2*4 + 4*4 + 4 + 2) // page size, margins, score size, flags
		for range 10 {
			intByteSize("")
		}
		intByteSize("Moderato") // tempo name
		i32(sp.Tempo)
		if minor > 0 {
			u8(0) // hide tempo
		}
		u8(0)  // key
		i32(0) // octave
	}

	// Channels: assign the tracks'.
	channels := make([]int, len(sp.Tracks))
	var programs [64]int
	next := 1
	for i, t := range sp.Tracks {
		ch := t.Channel
		switch {
		case ch != 0:
		case t.Drums:
			ch = 10
		default:
			if next == 10 {
				next++
			}
			ch, next = next, next+1
		}
		channels[i] = ch
		programs[ch-1] = t.Program
	}
	for _, p := range programs {
		i32(p)
		b.Write([]byte{13, 8, 0, 0, 0, 0, 0, 0}) // volume, balance, chorus, reverb, phaser, tremolo, 2 blank
	}
	if major == 5 {
		zeros(19*2 + 4) // musical directions, master reverb
	}
	i32(len(sp.Bars))
	i32(len(sp.Tracks))

	for i := range sp.Bars {
		if major == 5 && i > 0 {
			u8(0)
		}
		flags := 0
		if i == 0 {
			flags = 0x03 // time signature
		}
		u8(flags)
		if flags&0x01 != 0 {
			u8(4)
		}
		if flags&0x02 != 0 {
			u8(4)
		}
		if major == 5 {
			if flags&0x03 != 0 {
				zeros(4) // beam groups
			}
			u8(0) // (no alternate ending)
			u8(0) // triplet feel
		}
	}

	for i, t := range sp.Tracks {
		if major == 5 && (i == 0 || minor == 0) {
			u8(0)
		}
		flags := 0
		if t.Drums {
			flags = 1
		}
		u8(flags)
		byteSize(t.Name, 40)
		i32(len(t.Strings))
		for j := range 7 {
			if j < len(t.Strings) {
				i32(t.Strings[j])
			} else {
				i32(0)
			}
		}
		i32(1) // port
		i32(channels[i])
		i32(channels[i]) // effect channel
		i32(24)          // frets
		i32(0)           // capo
		i32(0x00C86464)  // color
		if major == 5 {
			zeros(2 + 1 + 1)   // flags2, auto accentuation, bank
			zeros(1 + 12 + 12) // RSE humanize + unknown
			zeros(4 + 4 + 4)   // RSE instrument, unknown, sound bank
			if minor == 0 {
				zeros(2 + 1)
			} else {
				zeros(4 + 4)
				intByteSize("")
				intByteSize("")
			}
		}
	}
	if major == 5 {
		if minor == 0 {
			zeros(2)
		} else {
			zeros(1)
		}
	}

	// beat writes a beat; tempo > 0 adds a mix-table tempo change, text a text
	// field, notes the strings played (1 = highest) with fret = string number + bar.
	beat := func(bar int, tempo int, text string, notes ...int) {
		flags := 0
		if len(notes) == 0 {
			flags |= 0x40
		}
		if text != "" {
			flags |= 0x04
		}
		if tempo > 0 {
			flags |= 0x10
		}
		u8(flags)
		if flags&0x40 != 0 {
			u8(0x02) // rest
		}
		u8(0) // duration: quarter
		if text != "" {
			intByteSize(text)
		}
		if tempo > 0 {
			u8(0xFF) // instrument: none
			if major == 5 {
				zeros(16)
			}
			for range 6 {
				u8(0xFF) // volume, balance, chorus, reverb, phaser, tremolo: unchanged
			}
			if major == 5 {
				intByteSize("")
			}
			i32(tempo)
			u8(0) // transition
			if major == 5 && minor > 0 {
				u8(0) // hide tempo
			}
			if major >= 4 {
				u8(0) // apply to all tracks
			}
			if major == 5 {
				u8(0)
				if minor > 0 {
					intByteSize("")
					intByteSize("")
				}
			}
		}
		played := 0
		for _, n := range notes {
			played |= 1 << (7 - n)
		}
		u8(played)
		for _, n := range notes {
			flags := 0x30 // type, velocity and fret follow
			if n == 1 && bar%2 == 1 {
				flags |= 0x08 // note effects: a tremolo picking / grace note
			}
			u8(flags)
			u8(1)          // normal note
			u8(6)          // velocity
			u8(n + bar%12) // fret
			if major == 5 {
				u8(0)
			}
			if flags&0x08 != 0 {
				switch major {
				case 3:
					u8(0x10)
					zeros(4) // grace note
				default:
					u8(0)
					u8(0x04)
					u8(2) // tremolo picking
				}
			}
		}
		if major == 5 {
			zeros(2)
		}
	}
	for bar, gb := range sp.Bars {
		for ti, t := range sp.Tracks {
			strings := 2
			if t.Drums {
				strings = 1
			}
			beats := 2
			if bar == 0 && ti == 0 {
				beats = 3
			}
			i32(beats)
			tempo := 0
			if ti == 0 {
				tempo = gb.Tempo
			}
			if bar == 0 && ti == 0 {
				beat(bar, 0, "intro", 1)
			}
			if strings == 2 {
				beat(bar, tempo, "", 1, 2)
			} else {
				beat(bar, tempo, "", 1)
			}
			beat(bar, 0, "")
			if major == 5 {
				i32(0) // second voice
				u8(0)  // line break
			}
		}
	}
	return b.Bytes()
}
