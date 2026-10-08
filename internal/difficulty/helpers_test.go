package difficulty_test

import (
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

// Made-up parts, built note by note. Times are in ticks: a quarter is score.Quarter.

const (
	q = score.Quarter
	e = q / 2 // eighth
	s = q / 4 // sixteenth
)

var (
	stdGuitar = []int{40, 45, 50, 55, 59, 64}
	bass4     = []int{28, 33, 38, 43}
)

// bars4 is n bars of 4/4 at bpm.
func bars4(n int, bpm float64) []score.Bar {
	out := make([]score.Bar, n)
	for i := range out {
		out[i] = score.Bar{Num: 4, Den: 4, BPM: bpm}
	}
	return out
}

// every repeats beats of dur from 0 to the end of a 4/4 bar, each with these notes.
func every(dur int, notes ...score.Note) []score.Beat {
	var out []score.Beat
	for t := 0; t < 4*q; t += dur {
		out = append(out, score.Beat{Start: t, Dur: dur, Notes: notes})
	}
	return out
}

// track is a track playing the same bar n times.
func track(n int, bar []score.Beat) score.Track {
	t := score.Track{Bars: make([][]score.Beat, n)}
	for i := range t.Bars {
		t.Bars[i] = bar
	}
	return t
}

// powerChord is E5 on the low strings.
var powerChord = []score.Note{{String: 0, Fret: 0}, {String: 1, Fret: 2}}

// tuplets is a 4/4 bar of n:m tuplets of a note value with these notes.
func tuplets(n int, value int, notes ...score.Note) []score.Beat {
	dur := value * score.TupletBase(n) / n
	var out []score.Beat
	for t := 0; t+dur <= 4*q; t += dur {
		out = append(out, score.Beat{Start: t, Dur: dur, Tuplet: n, Notes: notes})
	}
	return out
}

// tagsOf is the tags of the part of that role, nil if there's no such part.
func tagsOf(t *testing.T, sc *score.Score, tracks []difficulty.TrackInfo, r difficulty.Role) []string {
	t.Helper()
	for _, p := range difficulty.Analyze(sc, tracks) {
		if p.Role == r {
			return p.Tags
		}
	}
	t.Fatalf("no %s part", r)
	return nil
}

var rhythmGuitar = []difficulty.TrackInfo{{Name: "Rhythm", Instrument: "Distortion Guitar", Pitches: stdGuitar}}

var (
	kick, snare, hat = score.Note{Fret: 36}, score.Note{Fret: 38}, score.Note{Fret: 42}
	drumKit          = []difficulty.TrackInfo{{Name: "Drums", Drums: true}}
	bassGuitar       = []difficulty.TrackInfo{{Name: "Bass", Instrument: "Electric Bass (pick)", Pitches: bass4}}
	leadGuitar       = []difficulty.TrackInfo{{Name: "Lead", Instrument: "Overdriven Guitar", Pitches: stdGuitar}}
)

// rockBeat is a bar of sixteenth hi-hats, kick on 1 and 3, snare on 2 and 4.
func rockBeat() []score.Beat {
	out := every(s, hat)
	for i := range out {
		switch out[i].Start {
		case 0, 2 * q:
			out[i].Notes = []score.Note{kick, hat}
		case q, 3 * q:
			out[i].Notes = []score.Note{snare, hat}
		}
	}
	return out
}

// at is a bar of power chords struck at these times, each lasting to the next.
func at(starts ...int) []score.Beat {
	var out []score.Beat
	for i, st := range starts {
		end := 4 * q
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		out = append(out, score.Beat{Start: st, Dur: end - st, Notes: powerChord})
	}
	return out
}

// groove is a drum bar of three lines over a grid of steps: hi-hat, snare and kick, "x"
// for a hit: groove(s, "x.x.", "..x.", "x...").
func groove(step int, hats, snares, kicks string) []score.Beat {
	var out []score.Beat
	for i := 0; i < len(hats); i++ {
		var notes []score.Note
		for _, l := range []struct {
			line string
			note score.Note
		}{{kicks, kick}, {snares, snare}, {hats, hat}} {
			if i < len(l.line) && l.line[i] == 'x' {
				notes = append(notes, l.note)
			}
		}
		if len(notes) > 0 {
			out = append(out, score.Beat{Start: i * step, Dur: step, Notes: notes})
		}
	}
	return out
}

// shape is a chord from frets per string, lowest first; -1 for a string not played.
func shape(frets ...int) []score.Note {
	var out []score.Note
	for str, f := range frets {
		if f >= 0 {
			out = append(out, score.Note{String: str, Fret: f})
		}
	}
	return out
}

// melody is a bar of single sixteenth notes up and down the B and high E strings
// around the 12th fret, with fx on every fourth note.
func melody(fx score.Fx) []score.Beat {
	var out []score.Beat
	frets := []int{12, 14, 15, 17, 15, 14, 12, 15}
	for i, t := 0, 0; t < 4*q; i, t = i+1, t+s {
		n := score.Note{String: 4 + i%2, Fret: frets[i%len(frets)]}
		if i%4 == 0 {
			n.Fx = fx
		}
		out = append(out, score.Beat{Start: t, Dur: s, Notes: []score.Note{n}})
	}
	return out
}
