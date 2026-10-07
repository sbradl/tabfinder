package difficulty_test

import (
	"reflect"
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

func TestRoles(t *testing.T) {
	kick := score.Note{Fret: 36}
	sc := &score.Score{
		Bars: bars4(4, 120),
		Tracks: []score.Track{
			{Drums: true, Bars: track(4, every(q, kick)).Bars},
			track(4, every(e, score.Note{String: 0, Fret: 0})),
			track(4, every(e, powerChord...)),
			track(4, every(q, score.Note{String: 3, Fret: 5})),
			track(4, nil), // a guitar that doesn't play
		},
	}
	tracks := []difficulty.TrackInfo{
		{Name: "Drums", Instrument: "Drums", Drums: true},
		{Name: "Bass", Instrument: "Electric Bass (finger)", Pitches: bass4},
		{Name: "Guitar", Instrument: "Distortion Guitar", Pitches: stdGuitar},
		{Name: "Woodwind", Instrument: "Oboe", Pitches: stdGuitar},
		{Name: "Guitar 2", Instrument: "Distortion Guitar", Pitches: stdGuitar},
	}
	got := difficulty.Analyze(sc, tracks)
	want := []difficulty.Part{
		{Role: difficulty.Drums, Tracks: []int{0}},
		{Role: difficulty.Bass, Tracks: []int{1}},
		{Role: difficulty.Rhythm, Tracks: []int{2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parts:\n got %+v\nwant %+v", got, want)
	}
}
