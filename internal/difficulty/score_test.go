package difficulty_test

import (
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

// scoreOf is the score of the part of that role in a song of these bars, all at bpm.
func scoreOf(t *testing.T, bars [][]score.Beat, bpm float64, tracks []difficulty.TrackInfo, r difficulty.Role) float64 {
	t.Helper()
	sc := &score.Score{Bars: bars4(len(bars), bpm), Tracks: []score.Track{{Drums: r == difficulty.Drums, Bars: bars}}}
	for _, p := range difficulty.Analyze(sc, tracks) {
		if p.Role == r {
			return p.Score
		}
	}
	t.Fatalf("no %s part", r)
	return 0
}

// harder checks that each part scores higher than the one before.
func harder(t *testing.T, name string, scores ...float64) {
	t.Helper()
	for i := 1; i < len(scores); i++ {
		if scores[i] <= scores[i-1] {
			t.Errorf("%s: scores %v, want each higher than the one before", name, scores)
			return
		}
	}
}

func TestScoreRange(t *testing.T) {
	whole := []score.Beat{{Start: 0, Dur: 4 * q, Notes: powerChord}}
	easy := scoreOf(t, track(16, whole).Bars, 80, rhythmGuitar, difficulty.Rhythm)
	if easy < 1 || easy > 2 {
		t.Errorf("a power chord per bar at 80: score %v, want 1 to 2", easy)
	}
	frantic := scoreOf(t, track(16, every(s, powerChord...)).Bars, 320, rhythmGuitar, difficulty.Rhythm)
	if frantic < 9 || frantic > 10 {
		t.Errorf("sixteenth power chords at 320: score %v, want 9 to 10", frantic)
	}
}

// line is a bar of sixteenths on the low strings, a note from each pair (string, fret)
// in turn, with fx.
func line(fx score.Fx, notes ...[2]int) []score.Beat {
	var out []score.Beat
	for i := range 16 {
		n := notes[i%len(notes)]
		out = append(out, score.Beat{Start: i * s, Dur: s, Notes: []score.Note{{String: n[0], Fret: n[1], Fx: fx}}})
	}
	return out
}

func TestScoreFrettingHand(t *testing.T) {
	chug := line(score.PalmMute, [2]int{0, 0})
	pedal := line(0, [2]int{0, 0}, [2]int{1, 3}, [2]int{0, 0}, [2]int{1, 5}, [2]int{0, 0}, [2]int{1, 7}, [2]int{0, 0}, [2]int{1, 5})
	moving := line(0, [2]int{0, 3}, [2]int{1, 3}, [2]int{0, 5}, [2]int{1, 5}, [2]int{0, 7}, [2]int{1, 7}, [2]int{0, 5}, [2]int{1, 2})
	scoreBar := func(bar []score.Beat) float64 {
		return scoreOf(t, track(16, bar).Bars, 140, rhythmGuitar, difficulty.Rhythm)
	}
	harder(t, "an open string, a pedal tone riff, all notes fretted", scoreBar(chug), scoreBar(pedal), scoreBar(moving))
}

func TestScorePickingHand(t *testing.T) {
	notes := [][2]int{{0, 3}, {0, 5}, {0, 7}, {0, 5}}
	scoreBar := func(bar []score.Beat) float64 {
		return scoreOf(t, track(16, bar).Bars, 140, rhythmGuitar, difficulty.Rhythm)
	}
	harder(t, "hammer-ons and pull-offs, the same picked", scoreBar(line(score.Legato, notes...)), scoreBar(line(0, notes...)))
}

func TestScoreRisesWithSpeed(t *testing.T) {
	riff := track(16, every(s, powerChord...)).Bars
	var scores []float64
	for _, bpm := range []float64{60, 100, 140, 180} {
		scores = append(scores, scoreOf(t, riff, bpm, rhythmGuitar, difficulty.Rhythm))
	}
	harder(t, "sixteenth power chords at 60, 100, 140, 180 BPM", scores...)
}
