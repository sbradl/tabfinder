package difficulty_test

import (
	"fmt"
	"slices"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

// riff is a rhythm guitar track of power chords, bar by bar: true for a bar of eighth
// triplets, false for straight eighths.
func riff(triplets ...bool) score.Track {
	var t score.Track
	for _, tri := range triplets {
		if tri {
			t.Bars = append(t.Bars, tuplets(3, e, powerChord...))
		} else {
			t.Bars = append(t.Bars, every(e, powerChord...))
		}
	}
	return t
}

func TestTagTriplets(t *testing.T) {
	const o, x = false, true
	tests := []struct {
		name string
		riff score.Track
		want bool
	}{
		{"straight", riff(o, o, o, o, o, o, o, o), false},
		{"triplets throughout", riff(x, x, x, x, x, x, x, x), true},
		{"every other bar", riff(x, o, x, o, x, o, x, o), true},
		{"one fill in sixteen bars", riff(o, o, o, o, o, o, o, x, o, o, o, o, o, o, o, o), false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(len(tt.riff.Bars), 120), Tracks: []score.Track{tt.riff}}
		if got := slices.Contains(tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm), "triplets"); got != tt.want {
			t.Errorf("%s: triplets %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTagTuplets(t *testing.T) {
	bars := func(fill []score.Beat) score.Track {
		return score.Track{Bars: [][]score.Beat{fill, every(e, powerChord...), fill, every(e, powerChord...)}}
	}
	tests := []struct {
		name              string
		track             score.Track
		triplets, tuplets bool
	}{
		{"quintuplets", bars(tuplets(5, s, powerChord...)), false, true},
		{"septuplets", bars(tuplets(7, s, powerChord...)), false, true},
		{"sextuplets are triplets", bars(tuplets(6, s, powerChord...)), true, false},
		{"twelve 32nd triplets are triplets", bars(tuplets(12, s/2, powerChord...)), true, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(4, 100), Tracks: []score.Track{tt.track}}
		tags := tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm)
		if got := slices.Contains(tags, "triplets"); got != tt.triplets {
			t.Errorf("%s: triplets %v, want %v", tt.name, got, tt.triplets)
		}
		if got := slices.Contains(tags, "tuplets"); got != tt.tuplets {
			t.Errorf("%s: tuplets %v, want %v", tt.name, got, tt.tuplets)
		}
	}
}

// meters is a score of one rhythm guitar playing eighth power chords through bars of these
// time signatures ("7/8").
func meters(sigs ...string) *score.Score {
	sc := &score.Score{Tracks: make([]score.Track, 1)}
	for _, sig := range sigs {
		var b score.Bar
		fmt.Sscanf(sig, "%d/%d", &b.Num, &b.Den)
		b.BPM = 120
		sc.Bars = append(sc.Bars, b)
		var beats []score.Beat
		for t := 0; t < b.Num*4*q/b.Den; t += e {
			beats = append(beats, score.Beat{Start: t, Dur: e, Notes: powerChord})
		}
		sc.Tracks[0].Bars = append(sc.Tracks[0].Bars, beats)
	}
	return sc
}

func TestTagMeters(t *testing.T) {
	repeat := func(n int, sigs ...string) []string {
		var out []string
		for range n {
			out = append(out, sigs...)
		}
		return out
	}
	tests := []struct {
		name          string
		sigs          []string
		odd, changing bool
	}{
		{"4/4", repeat(8, "4/4"), false, false},
		{"7/8", repeat(8, "7/8"), true, false},
		{"5/4", repeat(8, "5/4"), true, false},
		{"compound 6/8, 9/8, 12/8", repeat(3, "6/8", "6/8", "9/8", "12/8"), false, true},
		{"7/8 and 4/4 in turn", repeat(4, "7/8", "4/4"), true, true},
		{"one 2/4 bar", append(repeat(8, "4/4"), append([]string{"2/4"}, repeat(8, "4/4")...)...), false, false},
	}
	for _, tt := range tests {
		tags := tagsOf(t, meters(tt.sigs...), rhythmGuitar, difficulty.Rhythm)
		if got := slices.Contains(tags, "odd meter"); got != tt.odd {
			t.Errorf("%s: odd meter %v, want %v", tt.name, got, tt.odd)
		}
		if got := slices.Contains(tags, "meter changes"); got != tt.changing {
			t.Errorf("%s: meter changes %v, want %v", tt.name, got, tt.changing)
		}
	}
}

func TestTagFast(t *testing.T) {
	drums := func(bar []score.Beat) score.Track { return score.Track{Drums: true, Bars: track(8, bar).Bars} }
	tests := []struct {
		name  string
		info  []difficulty.TrackInfo
		role  difficulty.Role
		track score.Track
		bpm   float64
		want  bool
	}{
		{"rhythm: 16ths at 160", rhythmGuitar, difficulty.Rhythm, track(8, every(s, powerChord...)), 160, true},
		{"rhythm: 16ths at 100", rhythmGuitar, difficulty.Rhythm, track(8, every(s, powerChord...)), 100, false},
		{"rhythm: 8ths at 200", rhythmGuitar, difficulty.Rhythm, track(8, every(e, powerChord...)), 200, false},
		{"bass: 16ths at 160", bassGuitar, difficulty.Bass, track(8, every(s, score.Note{Fret: 0})), 160, true},
		{"lead: 16ths at 150", leadGuitar, difficulty.Lead, track(8, melody(score.Bend)), 150, true},
		{"lead: 16ths at 110", leadGuitar, difficulty.Lead, track(8, melody(score.Bend)), 110, false},
		{"drums: 16th rock beat at 120", drumKit, difficulty.Drums, drums(rockBeat()), 120, false},
		{"drums: 16th rock beat at 190", drumKit, difficulty.Drums, drums(rockBeat()), 190, true},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, tt.bpm), Tracks: []score.Track{tt.track}}
		if got := slices.Contains(tagsOf(t, sc, tt.info, tt.role), "fast"); got != tt.want {
			t.Errorf("%s: fast %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTagSyncopated(t *testing.T) {
	tied := []score.Note{{String: 0, Fret: 0, Tie: true}, {String: 1, Fret: 2, Tie: true}}
	// Pushed: the last chord comes an eighth early and is held over the bar line.
	pushed := append([]score.Beat{{Start: 0, Dur: q, Notes: tied}}, at(q, 2*q, 3*q+e)[:]...)
	funk := rockBeat()
	for i := range funk {
		if funk[i].Start == q+e { // kick on the "and" of 2, nothing on 3
			funk[i].Notes = []score.Note{kick, hat}
		}
		if funk[i].Start == 2*q {
			funk[i].Notes = []score.Note{hat}
		}
	}
	tests := []struct {
		name  string
		info  []difficulty.TrackInfo
		role  difficulty.Role
		track score.Track
		want  bool
	}{
		{"straight eighths", rhythmGuitar, difficulty.Rhythm, track(8, every(e, powerChord...)), false},
		{"straight sixteenths", rhythmGuitar, difficulty.Rhythm, track(8, every(s, powerChord...)), false},
		{"3-3-2", rhythmGuitar, difficulty.Rhythm, track(8, at(0, 3*e, 6*e)), true},
		{"pushed over the bar line", rhythmGuitar, difficulty.Rhythm, track(8, pushed), true},
		{"chord on the and of 4, then on 1", rhythmGuitar, difficulty.Rhythm, track(8, at(0, q, 2*q, 3*q+e)), false},
		{"rock beat", drumKit, difficulty.Drums, score.Track{Drums: true, Bars: track(8, rockBeat()).Bars}, false},
		{"funk kick", drumKit, difficulty.Drums, score.Track{Drums: true, Bars: track(8, funk).Bars}, true},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, 100), Tracks: []score.Track{tt.track}}
		if got := slices.Contains(tagsOf(t, sc, tt.info, tt.role), "syncopated"); got != tt.want {
			t.Errorf("%s: syncopated %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTagEndurance(t *testing.T) {
	fast, slow := every(s, powerChord...), every(q, powerChord...)
	alternate := func(n, run int) [][]score.Beat { // runs of fast bars, then a slow bar
		var out [][]score.Beat
		for i := range n {
			out = append(out, [][]score.Beat{fast, slow}[min(i%(run+1)/run, 1)])
		}
		return out
	}
	tests := []struct {
		name string
		bars [][]score.Beat
		want bool
	}{
		{"32 fast bars in a row (48 s)", track(32, fast).Bars, true},
		{"fast in runs of 8 bars (12 s)", alternate(45, 8), false},
		{"16 fast bars (24 s), then slow", append(track(16, fast).Bars, track(16, slow).Bars...), false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(len(tt.bars), 160), Tracks: []score.Track{{Bars: tt.bars}}}
		if got := slices.Contains(tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm), "endurance"); got != tt.want {
			t.Errorf("%s: endurance %v, want %v", tt.name, got, tt.want)
		}
	}
}
