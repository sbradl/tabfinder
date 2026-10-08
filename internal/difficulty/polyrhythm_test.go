package difficulty_test

import (
	"slices"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

// grouped is n bars of 4/4 of power chords on every k-th sixteenth, counted on over the bar
// lines: a grouping of k that the bars don't divide shifts against them.
func grouped(n, k int) [][]score.Beat {
	out := make([][]score.Beat, n)
	for i := range n * 16 {
		if i%k == 0 {
			out[i/16] = append(out[i/16], score.Beat{Start: i % 16 * s, Dur: s, Notes: powerChord})
		}
	}
	return out
}

func TestTagPolyrhythm(t *testing.T) {
	drumsTrack := score.Track{Drums: true, Bars: track(8, rockBeat()).Bars}
	both := append(slices.Clone(drumKit), rhythmGuitar...)
	tests := []struct {
		name          string
		tracks        []score.Track
		info          []difficulty.TrackInfo
		drums, rhythm bool // a polyrhythm tag on that part
	}{
		// Triplets against a straight beat are just triplets.
		{"triplets over straight sixteenths", []score.Track{drumsTrack, track(8, tuplets(3, e, powerChord...))}, both, false, false},
		{"straight sixteenths over triplets", []score.Track{{Drums: true, Bars: track(8, tuplets(3, e, snare)).Bars}, track(8, every(s, powerChord...))}, both, false, false},
		{"triplets alone", []score.Track{track(8, tuplets(3, e, powerChord...))}, rhythmGuitar, false, false},
		{"groups of three sixteenths", []score.Track{{Bars: grouped(8, 3)}}, rhythmGuitar, false, true},
		{"groups of five sixteenths over a rock beat", []score.Track{drumsTrack, {Bars: grouped(8, 5)}}, both, false, true},
		{"groups of four", []score.Track{{Bars: grouped(8, 4)}}, rhythmGuitar, false, false},
		// One passage is enough: the band has to get it right.
		{"a passage of groups of three in a long song", []score.Track{{Bars: slices.Concat(
			track(24, every(e, powerChord...)).Bars, grouped(4, 3), track(24, every(e, powerChord...)).Bars)}}, rhythmGuitar, false, true},
		{"3-3-2 in every bar", []score.Track{track(8, at(0, 3*e, 6*e))}, rhythmGuitar, false, false},
		{"straight eighths with a rock beat", []score.Track{drumsTrack, track(8, every(e, powerChord...))}, both, false, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(len(tt.tracks[len(tt.tracks)-1].Bars), 120), Tracks: tt.tracks}
		got := map[difficulty.Role]bool{}
		for _, p := range difficulty.Analyze(sc, tt.info) {
			got[p.Role] = slices.Contains(p.Tags, "polyrhythm")
		}
		if got[difficulty.Drums] != tt.drums || got[difficulty.Rhythm] != tt.rhythm {
			t.Errorf("%s: polyrhythm on drums %v, rhythm %v; want %v, %v", tt.name,
				got[difficulty.Drums], got[difficulty.Rhythm], tt.drums, tt.rhythm)
		}
	}
}
