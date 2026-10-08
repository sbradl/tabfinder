package difficulty_test

import (
	"slices"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

// pattern is the n-th of many different bars: power chords on sixteenths picked by the
// bits of n, and on 1.
func pattern(n int) []score.Beat {
	var starts []int
	for i := range 16 {
		if i == 0 || (n+1)&(1<<(i-1)) != 0 {
			starts = append(starts, i*s)
		}
	}
	return at(starts...)
}

// transposed moves a bar up the neck.
func transposed(bar []score.Beat, frets int) []score.Beat {
	out := slices.Clone(bar)
	for i := range out {
		notes := slices.Clone(out[i].Notes)
		for j := range notes {
			notes[j].Fret += frets
		}
		out[i].Notes = notes
	}
	return out
}

func TestTagStructure(t *testing.T) {
	riff := pattern(5)
	varied := slices.Clone(pattern(5))
	varied[1].Notes = transposed(varied[1:2], 3)[0].Notes // one chord changed
	cycle := func(n, distinct int) [][]score.Beat {
		var out [][]score.Beat
		for i := range n {
			out = append(out, pattern(i%distinct))
		}
		return out
	}
	tests := []struct {
		name                  string
		bars                  [][]score.Beat
		heads                 []score.Bar // nil for 4/4 without repeats
		repetitive, manyParts bool
	}{
		{"one riff, 32 times", track(32, riff).Bars, nil, true, false},
		{"one riff, 4 bars repeated 8 times", track(4, riff).Bars,
			[]score.Bar{{Num: 4, Den: 4, RepeatOpen: true}, {Num: 4, Den: 4}, {Num: 4, Den: 4}, {Num: 4, Den: 4, Repeats: 7}}, true, false},
		{"a riff moved up and down", func() [][]score.Beat {
			var out [][]score.Beat
			for i := range 32 {
				out = append(out, transposed(riff, (i%4)*2))
			}
			return out
		}(), nil, true, false},
		{"a riff with small changes", func() [][]score.Beat {
			var out [][]score.Beat
			for i := range 32 {
				out = append(out, [][]score.Beat{riff, varied}[i%2])
			}
			return out
		}(), nil, true, false},
		{"8 bars over and over", cycle(32, 8), nil, false, false},
		{"40 different bars", cycle(40, 40), nil, false, true},
		{"8 bars repeated, but 4 written", cycle(4, 4),
			[]score.Bar{{Num: 4, Den: 4, RepeatOpen: true}, {Num: 4, Den: 4}, {Num: 4, Den: 4}, {Num: 4, Den: 4, Repeats: 1}}, false, false},
	}
	for _, tt := range tests {
		heads := tt.heads
		if heads == nil {
			heads = bars4(len(tt.bars), 120)
		}
		sc := &score.Score{Bars: heads, Tracks: []score.Track{{Bars: tt.bars}}}
		tags := tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm)
		if got := slices.Contains(tags, "repetitive"); got != tt.repetitive {
			t.Errorf("%s: repetitive %v, want %v", tt.name, got, tt.repetitive)
		}
		if got := slices.Contains(tags, "many parts"); got != tt.manyParts {
			t.Errorf("%s: many parts %v, want %v", tt.name, got, tt.manyParts)
		}
	}
}

func TestTagStructureDrumsIgnoresCymbals(t *testing.T) {
	ride, crash := score.Note{Fret: 51}, score.Note{Fret: 49}
	withCymbal := func(c score.Note, crashOn1 bool) []score.Beat {
		bar := rockBeat()
		for i := range bar {
			for j := range bar[i].Notes {
				if bar[i].Notes[j] == hat {
					bar[i].Notes = slices.Clone(bar[i].Notes)
					bar[i].Notes[j] = c
				}
			}
		}
		if crashOn1 {
			bar[0].Notes = []score.Note{kick, crash}
		}
		return bar
	}
	var bars [][]score.Beat
	for i := range 40 {
		bars = append(bars, withCymbal([]score.Note{hat, ride}[i/20], i%4 == 0))
	}
	sc := &score.Score{Bars: bars4(40, 120), Tracks: []score.Track{{Drums: true, Bars: bars}}}
	tags := tagsOf(t, sc, drumKit, difficulty.Drums)
	if !slices.Contains(tags, "repetitive") || slices.Contains(tags, "many parts") {
		t.Errorf("one beat, hi-hat or ride, crash every 4th bar: tags %v, want repetitive", tags)
	}
}

func TestTagStructureDrumFillsAreNoNewPart(t *testing.T) {
	toms := []score.Note{{Fret: 50}, {Fret: 48}, {Fret: 45}, {Fret: 43}, {Fret: 41}}
	fill := func(n int) []score.Beat { // the beat for two beats, then sixteenths on the toms
		bar := slices.Clone(rockBeat()[:8])
		for i := range 8 {
			bar = append(bar, score.Beat{Start: 2*q + i*s, Dur: s, Notes: []score.Note{toms[(n+i*(n%3+1))%len(toms)]}})
		}
		return bar
	}
	var bars [][]score.Beat
	for i := range 40 {
		if i%4 == 3 {
			bars = append(bars, fill(i/4))
		} else {
			bars = append(bars, rockBeat())
		}
	}
	sc := &score.Score{Bars: bars4(40, 120), Tracks: []score.Track{{Drums: true, Bars: bars}}}
	if tags := tagsOf(t, sc, drumKit, difficulty.Drums); !slices.Contains(tags, "repetitive") {
		t.Errorf("one beat with ten different fills: tags %v, want repetitive", tags)
	}
}
