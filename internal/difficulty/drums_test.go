package difficulty_test

import (
	"slices"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

func TestTagDoubleKickAndBlastBeats(t *testing.T) {
	tests := []struct {
		name              string
		bar               []score.Beat
		bpm               float64
		doubleKick, blast bool
	}{
		{"rock beat", rockBeat(), 120, false, false},
		{"sixteenth kicks at 120", groove(s,
			"x.x.x.x.x.x.x.x.",
			"....x.......x...",
			"xxxxxxxxxxxxxxxx"), 120, true, false},
		{"sixteenth kicks at 100", groove(s,
			"x.x.x.x.x.x.x.x.",
			"....x.......x...",
			"xxxxxxxxxxxxxxxx"), 100, false, false},
		{"eighth kicks at 200", groove(e,
			"xxxxxxxx",
			"..x...x.",
			"xxxxxxxx"), 200, false, false},
		{"kick and snare in turn at 200", groove(s,
			"x.x.x.x.x.x.x.x.",
			".x.x.x.x.x.x.x.x",
			"x.x.x.x.x.x.x.x."), 200, false, true},
		{"kick and snare together at 220", groove(e,
			"xxxxxxxx",
			"xxxxxxxx",
			"xxxxxxxx"), 220, false, true},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, tt.bpm), Tracks: []score.Track{{Drums: true, Bars: track(8, tt.bar).Bars}}}
		tags := tagsOf(t, sc, drumKit, difficulty.Drums)
		if got := slices.Contains(tags, "double kick"); got != tt.doubleKick {
			t.Errorf("%s: double kick %v, want %v", tt.name, got, tt.doubleKick)
		}
		if got := slices.Contains(tags, "blast beats"); got != tt.blast {
			t.Errorf("%s: blast beats %v, want %v", tt.name, got, tt.blast)
		}
	}
}
