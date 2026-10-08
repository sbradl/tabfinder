package difficulty_test

import (
	"slices"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

func TestTagTechniques(t *testing.T) {
	ghost := melody(0)
	for i := range ghost {
		if i%4 == 2 {
			ghost[i].Notes = []score.Note{{String: ghost[i].Notes[0].String, Fret: ghost[i].Notes[0].Fret, Ghost: true}}
		}
	}
	slap := melody(0)
	slap[0].Fx = score.Slap
	tests := []struct {
		tag string
		bar []score.Beat
	}{
		{"bends", melody(score.Bend)},
		{"tapping", melody(score.Tap)},
		{"harmonics", melody(score.Harmonic)},
		{"slap", slap},
		{"legato", melody(score.Legato)},
		{"tremolo picking", melody(score.TremoloPicking)},
		{"ghost notes", ghost},
	}
	plain := melody(0)
	for _, tt := range tests {
		for _, c := range []struct {
			name string
			bars [][]score.Beat
			want bool
		}{
			{"in every other bar", [][]score.Beat{tt.bar, plain, tt.bar, plain, tt.bar, plain, tt.bar, plain}, true},
			{"once in sixteen bars", append([][]score.Beat{tt.bar}, track(15, plain).Bars...), false},
		} {
			sc := &score.Score{Bars: bars4(len(c.bars), 100), Tracks: []score.Track{{Bars: c.bars}}}
			if got := slices.Contains(tagsOf(t, sc, leadGuitar, difficulty.Lead), tt.tag); got != c.want {
				t.Errorf("%s %s: %v, want %v", tt.tag, c.name, got, c.want)
			}
		}
	}
}

func TestTagChordsAndStretches(t *testing.T) {
	openG := shape(3, 2, 0, 0, 0, 3)
	barreF := shape(1, 3, 3, 2, 1, 1)
	stretchy := shape(-1, 1, 3, 5, 6, -1) // a span of five frets
	e5, a5 := shape(0, 2, 2, -1, -1, -1), shape(-1, 0, 2, 2, -1, -1)
	lick := []score.Beat{ // in one position: 5 and 9 on the G string, 5 and 10 on the B string
		{Start: 0, Dur: s, Notes: shape(-1, -1, -1, 5, -1, -1)}, {Start: s, Dur: s, Notes: shape(-1, -1, -1, 9, -1, -1)},
		{Start: 2 * s, Dur: s, Notes: shape(-1, -1, -1, -1, 5, -1)}, {Start: 3 * s, Dur: s, Notes: shape(-1, -1, -1, -1, 10, -1)},
	}
	slide := []score.Beat{ // the same frets one string at a time: the hand moves
		{Start: 0, Dur: q, Notes: shape(-1, -1, -1, 5, -1, -1)}, {Start: q, Dur: q, Notes: shape(-1, -1, -1, 10, -1, -1)},
	}
	tests := []struct {
		name              string
		bar               []score.Beat
		chords, stretches bool
	}{
		{"power chords", append(every(q, e5...)[:2], at(2 * q)[0]), false, false},
		{"open chords", every(q, openG...), true, false},
		{"barre chords", every(q, barreF...), true, false},
		{"stretched chord", every(q, stretchy...), true, true},
		{"power chords moving", []score.Beat{{Start: 0, Dur: 2 * q, Notes: e5}, {Start: 2 * q, Dur: 2 * q, Notes: a5}}, false, false},
		{"stretched lick", lick, false, true},
		// One finger per fret spans three; frets get narrower up the neck.
		{"four frets low on the neck", every(q, shape(-1, 1, -1, 5, -1, -1)...), false, true},
		{"four frets at the 3rd", every(q, shape(3, -1, -1, 7, -1, -1)...), false, true},
		{"three frets", every(q, shape(3, -1, -1, 6, -1, -1)...), false, false},
		{"five frets at the 12th", every(q, shape(-1, -1, -1, 12, -1, 17)...), false, false},
		{"shift along a string", slide, false, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(4, 100), Tracks: []score.Track{track(4, tt.bar)}}
		info := []difficulty.TrackInfo{{Name: "Guitar", Instrument: "Acoustic Guitar (steel)", Pitches: stdGuitar}}
		var tags []string
		for _, p := range difficulty.Analyze(sc, info) {
			tags = append(tags, p.Tags...)
		}
		if got := slices.Contains(tags, "chords"); got != tt.chords {
			t.Errorf("%s: chords %v, want %v", tt.name, got, tt.chords)
		}
		if got := slices.Contains(tags, "stretches"); got != tt.stretches {
			t.Errorf("%s: stretches %v, want %v", tt.name, got, tt.stretches)
		}
	}
}

func TestTagSweeps(t *testing.T) {
	// An A minor arpeggio over five strings, up and down, a note per string.
	var arpeggio []score.Beat
	strs, frets := []int{1, 2, 3, 4, 5, 4, 3, 2}, []int{12, 14, 14, 13, 12, 13, 14, 14}
	for i := range 16 {
		arpeggio = append(arpeggio, score.Beat{Start: i * s, Dur: s, Notes: []score.Note{{String: strs[i%8], Fret: frets[i%8]}}})
	}
	tests := []struct {
		name string
		bar  []score.Beat
		bpm  float64
		want bool
	}{
		{"arpeggio at 150", arpeggio, 150, true},
		{"arpeggio at 90", arpeggio, 90, false},
		{"two-string line at 150", melody(0), 150, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, tt.bpm), Tracks: []score.Track{track(8, tt.bar)}}
		if got := slices.Contains(tagsOf(t, sc, leadGuitar, difficulty.Lead), "sweeps"); got != tt.want {
			t.Errorf("%s: sweeps %v, want %v", tt.name, got, tt.want)
		}
	}
}
