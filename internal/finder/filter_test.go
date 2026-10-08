package finder

import (
	"math"
	"testing"

	"tabfinder/internal/tab"
)

func TestParseBPMRange(t *testing.T) {
	tests := []struct {
		in       string
		min, max float64
		err      bool
	}{
		{"120", 120, 120, false},
		{"100-140", 100, 140, false},
		{"180-", 180, math.Inf(1), false},
		{"-90", 0, 90, false},
		{"140-100", 0, 0, true},
		{"fast", 0, 0, true},
		{"-", 0, 0, true},
	}
	for _, tt := range tests {
		r, err := ParseBPMRange(tt.in)
		min, max := r.Min, r.Max
		if (err != nil) != tt.err || (!tt.err && (min != tt.min || max != tt.max)) {
			t.Errorf("ParseBPMRange(%q) = %v, %v, %v", tt.in, min, max, err)
		}
	}
}

func TestFilter(t *testing.T) {
	s := &tab.Song{
		Path:   "Metro Kettle/Master of Marionettes/Metro Kettle - Master of Marionettes.gp3",
		Artist: "Metro Kettle",
		Title:  "Master of Marionettes",
		Tracks: []tab.Track{
			{Name: "Drums", Drums: true},
			{Name: "Guitar", Pitches: []int{37, 44, 49, 54, 58, 63}},
			{Name: "Bass", Pitches: []int{25, 32, 37, 42}},
		},
		Tempos: []tab.Tempo{{Bar: 1, BPM: 212}, {Bar: 120, BPM: 140}},
	}
	tests := []struct {
		f    Filter
		want bool
	}{
		{Filter{Name: "marionettes master"}, true},
		{Filter{Name: "marionettes ride"}, false},
		{Filter{Artist: "metro"}, true},
		{Filter{Artist: "megalith dusk"}, false},
		{Filter{Tuning: "drop c#"}, true},
		{Filter{Tuning: "Drop Db"}, true},
		{Filter{Tuning: "C# Ab C# F# Bb Eb"}, true},
		{Filter{Tuning: "drop c"}, false},
		{Filter{BPM: &Range{130, 150}}, true},
		{Filter{BPM: &Range{150, 200}}, false},
		{Filter{Artist: "metro kettle", Tuning: "drop c#", BPM: &Range{200, 250}}, true},
		// String count, matched on the same track as the tuning.
		{Filter{Strings: 6}, true},
		{Filter{Strings: 7}, false},
		{Filter{Tuning: "drop c#", Strings: 4}, true},
		{Filter{Tuning: "C# Ab C# F# Bb Eb", Strings: 4}, false},
	}
	for _, tt := range tests {
		if got := tt.f.Matches(s); got != tt.want {
			t.Errorf("%+v.Matches() = %v, want %v", tt.f, got, tt.want)
		}
	}
}
