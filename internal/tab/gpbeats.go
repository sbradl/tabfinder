package tab

import (
	"cmp"
	"fmt"
	"math/bits"
	"slices"

	"tabfinder/internal/score"
)

// readGPMeasures reads the note data of a Guitar Pro 3-5 file into s.notes, and the tempo
// changes, which live in beat mix-table changes, into s.Tempos.
func readGPMeasures(r *reader, major, minor int, stringCounts []int, s *Song) error {
	voices := 1
	if major == 5 {
		voices = 2
	}
	for m := range s.notes.Bars {
		for t, strings := range stringCounts {
			var bar []score.Beat
			for v := 0; v < voices; v++ {
				beats := r.i32()
				if r.err != nil {
					return r.err
				}
				if beats < 0 || beats > maxBeats {
					return fmt.Errorf("bar %d, track %d: implausible beat count %d", m+1, t+1, beats)
				}
				bar = slices.Grow(bar, beats)
				start := 0
				for i := 0; i < beats; i++ {
					b, tempo := readGPBeat(r, major, minor, strings)
					if r.err != nil {
						return r.err
					}
					if tempo > maxTempo {
						return fmt.Errorf("bar %d, track %d: implausible tempo %d", m+1, t+1, tempo)
					}
					if tempo > 0 {
						s.addTempo(m+1, float64(tempo))
					}
					b.Start, b.Voice = start, v
					start += b.Dur
					if len(b.Notes) > 0 {
						bar = append(bar, b)
					}
				}
			}
			slices.SortStableFunc(bar, func(a, b score.Beat) int { return cmp.Compare(a.Start, b.Start) })
			s.notes.Tracks[t].Bars = append(s.notes.Tracks[t].Bars, bar)
			if major == 5 && r.pos < len(r.b) { // the file's last line-break byte is often omitted
				r.skip(1) // line break
			}
		}
	}
	return r.err
}

// readGPBeat reads one beat, and returns it and its tempo change, or -1.
func readGPBeat(r *reader, major, minor, strings int) (score.Beat, int) {
	var b score.Beat
	tempo := -1
	f := r.u8()
	if f&0x40 != 0 {
		r.skip(1) // beat status (empty/rest)
	}
	b.Dur = score.DurOf(int(int8(r.u8())))
	if f&0x01 != 0 {
		b.Dur += b.Dur / 2 // dotted
	}
	if f&0x20 != 0 {
		if n := r.i32(); n > 1 && n < 64 {
			b.Tuplet = n
			b.Dur = b.Dur * score.TupletBase(n) / n
		}
	}
	if f&0x02 != 0 {
		skipGPChord(r, major)
	}
	if f&0x04 != 0 {
		r.intByteSizeString() // text
	}
	if f&0x08 != 0 {
		b.Fx = readGPBeatEffects(r, major)
	}
	if f&0x10 != 0 {
		tempo = readGPMixTable(r, major, minor)
	}
	played := r.u8()
	if c := bits.OnesCount8(uint8(played)); c > 0 {
		b.Notes = make([]score.Note, 0, c)
	}
	for n := 1; n <= strings; n++ { // bit 6 = string 1
		if played&(1<<(7-n)) != 0 {
			note := readGPNote(r, major)
			note.String = strings - n
			b.Notes = append(b.Notes, note)
		}
	}
	slices.Reverse(b.Notes) // lowest string first
	if major == 5 {
		r.skip(1)
		if r.u8()&0x08 != 0 {
			r.skip(1) // break secondary beams
		}
	}
	return b, tempo
}

func skipGPChord(r *reader, major int) {
	if r.u8()&0x01 == 0 { // old format: name, first fret, frets if diagram
		r.intByteSizeString()
		if r.i32() != 0 {
			r.skip(6 * 4)
		}
		return
	}
	if major == 3 {
		r.skip(124)
	} else {
		r.skip(106)
	}
}

func readGPBeatEffects(r *reader, major int) score.Fx {
	var fx score.Fx
	// tapSlapPop is the technique of the byte after flag 0x20: 1 tapping, 2 slap, 3 pop.
	tapSlapPop := func(v int) {
		switch v {
		case 0:
			fx |= score.TremoloBar // GP3
		case 1:
			fx |= score.Tap
		default:
			fx |= score.Slap
		}
	}
	if major == 3 {
		f := r.u8()
		if f&0x03 != 0 {
			fx |= score.Vibrato
		}
		if f&0x0C != 0 {
			fx |= score.Harmonic
		}
		if f&0x20 != 0 {
			tapSlapPop(r.u8())
			r.skip(4) // tremolo bar value
		}
		if f&0x40 != 0 {
			r.skip(2) // stroke
		}
		return fx
	}
	f1, f2 := r.u8(), r.u8()
	if f1&0x02 != 0 {
		fx |= score.Vibrato
	}
	if f1&0x20 != 0 {
		if v := r.u8(); v != 0 {
			tapSlapPop(v)
		}
	}
	if f2&0x04 != 0 {
		fx |= score.TremoloBar
		skipGPBend(r)
	}
	if f1&0x40 != 0 {
		r.skip(2) // stroke
	}
	if f2&0x02 != 0 {
		r.skip(1) // pick stroke
	}
	return fx
}

// readGPMixTable skips a mix-table change and returns its tempo, or -1.
func readGPMixTable(r *reader, major, minor int) int {
	r.skip(1) // instrument
	if major == 5 {
		r.skip(16) // RSE instrument
	}
	var values [6]int // volume, balance, chorus, reverb, phaser, tremolo
	for i := range values {
		values[i] = int(int8(r.u8()))
	}
	if major == 5 {
		r.intByteSizeString() // tempo name
	}
	tempo := r.i32()
	for _, v := range values {
		if v >= 0 {
			r.skip(1) // transition duration
		}
	}
	if tempo >= 0 {
		r.skip(1) // transition duration
		if major == 5 && minor > 0 {
			r.skip(1) // hide tempo
		}
	}
	if major >= 4 {
		r.skip(1) // apply-to-all-tracks flags
	}
	if major == 5 {
		r.skip(1) // wah
		if minor > 0 {
			r.intByteSizeString() // RSE effect
			r.intByteSizeString() // RSE effect category
		}
	}
	return tempo
}

// readGPNote reads a note, all but its string.
func readGPNote(r *reader, major int) score.Note {
	var n score.Note
	f := r.u8()
	n.Ghost = f&0x04 != 0
	if f&0x42 != 0 {
		n.Fx |= score.Accent // accentuated, or (GP5) heavily
	}
	if f&0x20 != 0 {
		switch r.u8() { // note type
		case 2:
			n.Tie = true
		case 3:
			n.Dead = true
		}
	}
	if major < 5 && f&0x01 != 0 {
		r.skip(2) // duration, tuplet
	}
	if f&0x10 != 0 {
		r.skip(1) // velocity
	}
	if f&0x20 != 0 {
		n.Fret = r.u8()
	}
	if f&0x80 != 0 {
		r.skip(2) // fingering
	}
	if major == 5 {
		if f&0x01 != 0 {
			r.skip(8) // duration percent (float64)
		}
		r.skip(1)
	}
	if f&0x08 != 0 {
		n.Fx |= readGPNoteEffects(r, major)
	}
	return n
}

func readGPNoteEffects(r *reader, major int) score.Fx {
	var fx score.Fx
	set := func(on bool, x score.Fx) {
		if on {
			fx |= x
		}
	}
	if major == 3 {
		f := r.u8()
		set(f&0x01 != 0, score.Bend)
		set(f&0x02 != 0, score.Legato)
		set(f&0x04 != 0, score.Slide)
		set(f&0x10 != 0, score.Grace)
		if f&0x01 != 0 {
			skipGPBend(r)
		}
		if f&0x10 != 0 {
			r.skip(4) // grace note
		}
		return fx
	}
	f1, f2 := r.u8(), r.u8()
	set(f1&0x01 != 0, score.Bend)
	set(f1&0x02 != 0, score.Legato)
	set(f1&0x10 != 0, score.Grace)
	set(f2&0x01 != 0, score.Staccato)
	set(f2&0x02 != 0, score.PalmMute)
	set(f2&0x04 != 0, score.TremoloPicking)
	set(f2&0x08 != 0, score.Slide)
	set(f2&0x10 != 0, score.Harmonic)
	set(f2&0x20 != 0, score.Trill)
	set(f2&0x40 != 0, score.Vibrato)
	if f1&0x01 != 0 {
		skipGPBend(r)
	}
	if f1&0x10 != 0 {
		if major == 5 {
			r.skip(5) // grace note
		} else {
			r.skip(4)
		}
	}
	if f2&0x04 != 0 {
		r.skip(1) // tremolo picking
	}
	if f2&0x08 != 0 {
		r.skip(1) // slide
	}
	if f2&0x10 != 0 {
		harmonic := r.u8()
		if major == 5 {
			switch harmonic {
			case 2: // artificial: semitone, accidental, octave
				r.skip(3)
			case 3: // tapped: fret
				r.skip(1)
			}
		}
	}
	if f2&0x20 != 0 {
		r.skip(2) // trill: fret, period
	}
	return fx
}

func skipGPBend(r *reader) {
	r.skip(1 + 4) // type, value
	points := r.i32()
	if points < 0 || points > maxBendPts {
		r.err = fmt.Errorf("implausible bend point count %d at offset %d", points, r.pos)
		return
	}
	r.skip(points * 9) // position, value, vibrato
}
