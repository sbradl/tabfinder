package difficulty_test

import (
	"slices"
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
	frantic := scoreOf(t, track(320, every(s, powerChord...)).Bars, 320, rhythmGuitar, difficulty.Rhythm)
	if frantic < 9 || frantic > 10 {
		t.Errorf("four minutes of sixteenth power chords at 320: score %v, want 9 to 10", frantic)
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

	// Fingers stay down on notes played a moment ago.
	alternating := line(0, [2]int{0, 7}, [2]int{1, 9}) // no stretch in either
	run := line(0, [2]int{0, 7}, [2]int{0, 9}, [2]int{1, 7}, [2]int{1, 9}, [2]int{2, 6}, [2]int{2, 7}, [2]int{2, 9}, [2]int{1, 8})
	harder(t, "two notes in turn, a run of eight", scoreBar(alternating), scoreBar(run))
}

func TestScorePickingHand(t *testing.T) {
	notes := [][2]int{{0, 3}, {0, 5}, {0, 7}, {0, 5}}
	scoreBar := func(bar []score.Beat) float64 {
		return scoreOf(t, track(16, bar).Bars, 140, rhythmGuitar, difficulty.Rhythm)
	}
	harder(t, "hammer-ons and pull-offs, the same picked", scoreBar(line(score.Legato, notes...)), scoreBar(line(0, notes...)))
}

// gripPerBeat is a bar of sixteenths on the two lowest strings: for each beat, the two
// frets in turn.
func gripPerBeat(frets ...[2]int) []score.Beat {
	var out []score.Beat
	for k, f := range frets {
		for i := range 4 {
			out = append(out, score.Beat{Start: k*q + i*s, Dur: s, Notes: []score.Note{{String: i % 2, Fret: f[i%2]}}})
		}
	}
	return out
}

func TestScoreStretches(t *testing.T) {
	scoreBar := func(bar []score.Beat) float64 {
		return scoreOf(t, track(16, bar).Bars, 120, rhythmGuitar, difficulty.Rhythm)
	}
	// A stretch held over the bar is little work for the fretting hand; one grip after the
	// other is more, and one stretch after the other more still.
	held := gripPerBeat([2]int{3, 7}, [2]int{3, 7}, [2]int{3, 7}, [2]int{3, 7})
	moving := gripPerBeat([2]int{3, 5}, [2]int{4, 6}, [2]int{5, 7}, [2]int{6, 8})
	stretching := gripPerBeat([2]int{3, 7}, [2]int{4, 8}, [2]int{5, 9}, [2]int{6, 10})
	harder(t, "a stretch held, grips changing, stretches changing", scoreBar(held), scoreBar(moving), scoreBar(stretching))
}

func TestScoreRhythm(t *testing.T) {
	partScore := func(sc *score.Score) float64 {
		for _, p := range difficulty.Analyze(sc, rhythmGuitar) {
			return p.Score
		}
		return 0
	}
	repeat := func(n int, sigs ...string) []string {
		var out []string
		for range n {
			out = append(out, sigs...)
		}
		return out
	}
	// Eighth power chords, as fast in every meter.
	harder(t, "4/4, 7/8, 7/8 and 4/4 in turn",
		partScore(meters(repeat(16, "4/4")...)), partScore(meters(repeat(16, "7/8")...)), partScore(meters(repeat(8, "7/8", "4/4")...)))

	// Four chords a bar: on the beat, then pushed off it.
	straight, pushed := at(0, q, 2*q, 3*q), at(0, 3*e, 5*e, 7*e)
	scoreBar := func(bar []score.Beat) float64 {
		return scoreOf(t, track(16, bar).Bars, 120, rhythmGuitar, difficulty.Rhythm)
	}
	harder(t, "on the beat, syncopated", scoreBar(straight), scoreBar(pushed))
}

func TestScoreMaterial(t *testing.T) {
	// Four open strings a bar, in the same rhythm: as fast, no fretting. The strings of
	// different bars differ in two notes at least (the 4th is a check digit of the others),
	// so no two are the same but for a note.
	openStrings := func(strs ...int) []score.Beat {
		bar := at(0, q, 2*q, 3*q)
		for j := range bar {
			bar[j].Notes = []score.Note{{String: strs[j]}}
		}
		return bar
	}
	var many [][]score.Beat
	for i := range 40 {
		d0, d1, d2 := i%6, i/6%6, i/36%6
		many = append(many, openStrings(d0, d1, d2, (d0+d1+d2)%6))
	}
	one := scoreOf(t, track(40, many[7]).Bars, 120, rhythmGuitar, difficulty.Rhythm)
	different := scoreOf(t, many, 120, rhythmGuitar, difficulty.Rhythm)
	harder(t, "one bar 40 times, 40 different bars", one, different)
}

func TestScorePickingStamina(t *testing.T) {
	riff := every(e, powerChord...) // eighths at 200: 6.7 a second
	short := scoreOf(t, track(30, riff).Bars, 200, rhythmGuitar, difficulty.Rhythm)
	long := scoreOf(t, track(120, riff).Bars, 200, rhythmGuitar, difficulty.Rhythm)
	harder(t, "a minute of fast picking, four minutes", short, long)

	// Two fast bars in twenty: a burst, not what the song asks all through.
	slow, fast := every(q, powerChord...), every(s, powerChord...)
	burst := slices.Concat(track(18, slow).Bars, track(2, fast).Bars)
	throughout := track(20, fast).Bars
	harder(t, "fast in two of twenty bars, fast throughout",
		scoreOf(t, burst, 160, rhythmGuitar, difficulty.Rhythm), scoreOf(t, throughout, 160, rhythmGuitar, difficulty.Rhythm))
	if b := scoreOf(t, burst, 160, rhythmGuitar, difficulty.Rhythm); b > 5 {
		t.Errorf("quarter notes with a fast fill at 160: score %v, want 5 at most", b)
	}
}

func TestScorePickingLimit(t *testing.T) {
	// Picking gets hard all at once near a limit: ten BPM more change little far below it and
	// a lot around it. Each song is four minutes of sixteenths.
	at := func(bpm int) float64 {
		return scoreOf(t, track(bpm, every(s, powerChord...)).Bars, float64(bpm), rhythmGuitar, difficulty.Rhythm)
	}
	below, around := at(65)-at(55), at(95)-at(85)
	if around < 3*below || around < 2 {
		t.Errorf("ten BPM more: %.1f far below the limit (55 to 65), %.1f around it (85 to 95); want at least 2 and three times as much",
			below, around)
	}
	// Past the limit, who can pick that fast can mostly pick faster too.
	if above := at(180) - at(130); above >= 1 {
		t.Errorf("fifty BPM more past the limit (130 to 180): %.1f, want less than 1", above)
	}
}

func TestScoreRisesWithSpeed(t *testing.T) {
	var scores []float64
	for _, bpm := range []int{60, 100, 140, 180} { // four minutes each
		scores = append(scores, scoreOf(t, track(bpm, every(s, powerChord...)).Bars, float64(bpm), rhythmGuitar, difficulty.Rhythm))
	}
	for i := 1; i < len(scores); i++ {
		if scores[i] < scores[i-1] {
			t.Errorf("sixteenth power chords at 60, 100, 140, 180 BPM: scores %v, want none lower than the one before", scores)
		}
	}
	if scores[len(scores)-1]-scores[0] < 5 {
		t.Errorf("sixteenth power chords at 60 and 180 BPM: scores %v, want 5 apart at least", scores)
	}
}
