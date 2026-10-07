package tab

import "fmt"

// readGPMeasures walks the note data of a Guitar Pro 3-5 file. Tempo changes
// live in beat mix-table changes, so every beat and note has to be skipped
// structurally to reach them.
func readGPMeasures(r *reader, major, minor, measures int, stringCounts []int, s *Song) error {
	voices := 1
	if major == 5 {
		voices = 2
	}
	for m := 0; m < measures; m++ {
		for t, strings := range stringCounts {
			for v := 0; v < voices; v++ {
				beats := r.i32()
				if r.err != nil {
					return r.err
				}
				if beats < 0 || beats > maxBeats {
					return fmt.Errorf("bar %d, track %d: implausible beat count %d", m+1, t+1, beats)
				}
				for i := 0; i < beats; i++ {
					tempo := readGPBeat(r, major, minor, strings)
					if r.err != nil {
						return r.err
					}
					if tempo > maxTempo {
						return fmt.Errorf("bar %d, track %d: implausible tempo %d", m+1, t+1, tempo)
					}
					if tempo > 0 {
						s.addTempo(m+1, float64(tempo))
					}
				}
			}
			if major == 5 && r.pos < len(r.b) { // the file's last line-break byte is often omitted
				r.skip(1) // line break
			}
		}
	}
	return r.err
}

// readGPBeat skips one beat and returns its tempo change, or -1.
func readGPBeat(r *reader, major, minor, strings int) int {
	tempo := -1
	f := r.u8()
	if f&0x40 != 0 {
		r.skip(1) // beat status (empty/rest)
	}
	r.skip(1) // duration
	if f&0x20 != 0 {
		r.skip(4) // tuplet
	}
	if f&0x02 != 0 {
		skipGPChord(r, major)
	}
	if f&0x04 != 0 {
		r.intByteSizeString() // text
	}
	if f&0x08 != 0 {
		skipGPBeatEffects(r, major)
	}
	if f&0x10 != 0 {
		tempo = readGPMixTable(r, major, minor)
	}
	played := r.u8()
	for n := 1; n <= strings; n++ { // bit 6 = string 1
		if played&(1<<(7-n)) != 0 {
			skipGPNote(r, major)
		}
	}
	if major == 5 {
		r.skip(1)
		if r.u8()&0x08 != 0 {
			r.skip(1) // break secondary beams
		}
	}
	return tempo
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

func skipGPBeatEffects(r *reader, major int) {
	if major == 3 {
		f := r.u8()
		if f&0x20 != 0 {
			r.skip(1 + 4) // tapping/slap/pop or tremolo bar + value
		}
		if f&0x40 != 0 {
			r.skip(2) // stroke
		}
		return
	}
	f1, f2 := r.u8(), r.u8()
	if f1&0x20 != 0 {
		r.skip(1) // tapping/slap/pop
	}
	if f2&0x04 != 0 {
		skipGPBend(r) // tremolo bar
	}
	if f1&0x40 != 0 {
		r.skip(2) // stroke
	}
	if f2&0x02 != 0 {
		r.skip(1) // pick stroke
	}
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

func skipGPNote(r *reader, major int) {
	f := r.u8()
	if f&0x20 != 0 {
		r.skip(1) // note type
	}
	if major < 5 && f&0x01 != 0 {
		r.skip(2) // duration, tuplet
	}
	if f&0x10 != 0 {
		r.skip(1) // velocity
	}
	if f&0x20 != 0 {
		r.skip(1) // fret
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
		skipGPNoteEffects(r, major)
	}
}

func skipGPNoteEffects(r *reader, major int) {
	if major == 3 {
		f := r.u8()
		if f&0x01 != 0 {
			skipGPBend(r)
		}
		if f&0x10 != 0 {
			r.skip(4) // grace note
		}
		return
	}
	f1, f2 := r.u8(), r.u8()
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
