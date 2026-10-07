package tab

import (
	"math"
	"testing"
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
		min, max, err := ParseBPMRange(tt.in)
		if (err != nil) != tt.err || (!tt.err && (min != tt.min || max != tt.max)) {
			t.Errorf("ParseBPMRange(%q) = %v, %v, %v", tt.in, min, max, err)
		}
	}
}

func TestFilter(t *testing.T) {
	s := &Song{
		Path:   "Metro Kettle/Master of Marionettes/Metro Kettle - Master of Marionettes.gp3",
		Artist: "Metro Kettle",
		Title:  "Master of Marionettes",
		Tracks: []Track{
			{Name: "Drums", Drums: true},
			{Name: "Guitar", Pitches: []int{37, 44, 49, 54, 58, 63}, Tuning: "Drop C# (C# Ab C# F# Bb Eb)"},
			{Name: "Bass", Pitches: []int{25, 32, 37, 42}, Tuning: "Drop C# (C# Ab C# F#)"},
		},
		Tempos: []Tempo{{Bar: 1, BPM: 212}, {Bar: 120, BPM: 140}},
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
		{Filter{BPMSet: true, BPMMin: 130, BPMMax: 150}, true},
		{Filter{BPMSet: true, BPMMin: 150, BPMMax: 200}, false},
		{Filter{Artist: "metro kettle", Tuning: "drop c#", BPMSet: true, BPMMin: 200, BPMMax: 250}, true},
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
