package tab

import (
	"math"
	"testing"
)

func TestFilterMore(t *testing.T) {
	song := &Song{
		Path:   "Argyle Moth/Rent of Summer/Argyle Moth - Nectar (ver 2).gp5",
		Artist: "Argyle Moth",
		Title:  "Nectar",
		Tracks: []Track{
			{Name: "Drums", Drums: true, Instrument: "Drums"},
			{Name: "Lead", Pitches: []int{38, 45, 50, 55, 59, 64}, Tuning: "Drop D (D A D G B E)"},
			{Name: "Rhythm", Pitches: []int{40, 45, 50, 55, 59, 64}, Tuning: "E Standard (E A D G B E)"},
			{Name: "Bass", Pitches: []int{27, 32, 37, 42}, Tuning: "Eb Standard (Eb Ab C# F#)"},
			{Name: "Bass 7", Pitches: []int{35, 40, 45, 50, 55, 59, 64}, Tuning: "B Standard (B E A D G B E)"},
		},
		Tempos: []Tempo{{1, 100}, {5, 100.5}},
	}
	tests := []struct {
		name string
		f    Filter
		want bool
	}{
		{"empty filter matches", Filter{}, true},
		{"name: words in any order", Filter{Name: "nectar moth"}, true},
		{"name: case", Filter{Name: "NECTAR"}, true},
		{"name: file name only", Filter{Name: "ver"}, true},
		{"name: folder is not searched", Filter{Name: "rent"}, false},
		{"name: all words needed", Filter{Name: "nectar xyz"}, false},
		{"name: extra spaces", Filter{Name: "  nectar   moth "}, true},
		{"name: substring", Filter{Name: "ect"}, true},
		{"artist: substring", Filter{Artist: "yle mo"}, true},
		{"artist: case", Filter{Artist: "ARGYLE"}, true},
		{"artist: not trimmed", Filter{Artist: " argyle"}, false},
		{"tuning: name", Filter{Tuning: "drop d"}, true},
		{"tuning: name case", Filter{Tuning: "DROP D"}, true},
		{"tuning: notes", Filter{Tuning: "D A D G B E"}, true},
		{"tuning: notes lower", Filter{Tuning: "d a d g b e"}, true},
		{"tuning: enharmonic name", Filter{Tuning: "d# standard"}, true},
		{"tuning: enharmonic notes", Filter{Tuning: "d# g# c# f#"}, true},
		{"tuning: standard alone is E Standard", Filter{Tuning: "standard"}, true},
		{"tuning: Eb is not standard alone", Filter{Tuning: "eb"}, false},
		{"tuning: partial name", Filter{Tuning: "drop"}, false},
		{"tuning: from a drum track is ignored", Filter{Tuning: "drums"}, false},
		{"tuning: no match", Filter{Tuning: "drop c"}, false},
		{"strings alone", Filter{Strings: 4}, true},
		{"strings alone 7", Filter{Strings: 7}, true},
		{"strings alone 8", Filter{Strings: 8}, false},
		{"strings and tuning on the same track", Filter{Tuning: "drop d", Strings: 6}, true},
		{"strings and tuning on different tracks", Filter{Tuning: "drop d", Strings: 4}, false},
		{"b standard is the 7 string", Filter{Tuning: "b standard", Strings: 7}, true},
		{"b standard is not the 6 string", Filter{Tuning: "b standard", Strings: 6}, false},
		{"bpm: in range", Filter{BPMSet: true, BPMMin: 90, BPMMax: 110}, true},
		{"bpm: lower bound inclusive", Filter{BPMSet: true, BPMMin: 100, BPMMax: 100}, true},
		{"bpm: upper bound inclusive", Filter{BPMSet: true, BPMMin: 0, BPMMax: 100.5}, true},
		{"bpm: just above", Filter{BPMSet: true, BPMMin: 100.6, BPMMax: 200}, false},
		{"bpm: open ended", Filter{BPMSet: true, BPMMin: 100.5, BPMMax: math.Inf(1)}, true},
		{"bpm: not set ignores min and max", Filter{BPMMin: 500, BPMMax: 600}, true},
		{"everything", Filter{Name: "nectar", Artist: "argyle", Tuning: "drop d", Strings: 6, BPMSet: true, BPMMin: 100, BPMMax: 101}, true},
		{"everything but one", Filter{Name: "nectar", Artist: "argyle", Tuning: "drop d", Strings: 6, BPMSet: true, BPMMin: 200, BPMMax: 300}, false},
	}
	for _, tt := range tests {
		if got := tt.f.Matches(song); got != tt.want {
			t.Errorf("%s: %+v.Matches() = %v, want %v", tt.name, tt.f, got, tt.want)
		}
	}
}

func TestFilterEmptySongs(t *testing.T) {
	bare := &Song{Path: "x.gp5", Title: "X"}
	drumsOnly := &Song{Path: "d.gp5", Tracks: []Track{{Name: "Drums", Drums: true}}}
	for _, tt := range []struct {
		name string
		f    Filter
		s    *Song
		want bool
	}{
		{"no tracks, name", Filter{Name: "x"}, bare, true},
		{"no tracks, tuning", Filter{Tuning: "drop d"}, bare, false},
		{"no tracks, strings", Filter{Strings: 6}, bare, false},
		{"no tempos, bpm", Filter{BPMSet: true, BPMMin: 0, BPMMax: math.Inf(1)}, bare, false},
		{"drums only, strings", Filter{Strings: 6}, drumsOnly, false},
		{"drums only, tuning", Filter{Tuning: "standard"}, drumsOnly, false},
		{"empty filter, empty song", Filter{}, &Song{}, true},
		{"artist on empty artist", Filter{Artist: "a"}, bare, false},
	} {
		if got := tt.f.Matches(tt.s); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFilterActive(t *testing.T) {
	if (Filter{}).Active() {
		t.Error("zero filter is active")
	}
	if (Filter{BPMMin: 1, BPMMax: 2}).Active() {
		t.Error("BPM range without BPMSet is active")
	}
	for _, f := range []Filter{{Name: "a"}, {Artist: "a"}, {Tuning: "a"}, {Strings: 6}, {BPMSet: true}} {
		if !f.Active() {
			t.Errorf("%+v is not active", f)
		}
	}
}

func TestParseBPMRangeMore(t *testing.T) {
	inf := math.Inf(1)
	tests := []struct {
		in       string
		min, max float64
		err      bool
	}{
		{"  120  ", 120, 120, false},
		{" 100 - 140 ", 100, 140, false},
		{"100 -140", 100, 140, false},
		{"100- 140", 100, 140, false},
		{"120.5", 120.5, 120.5, false},
		{"99.5-100.25", 99.5, 100.25, false},
		{".5", 0.5, 0.5, false},
		{"0", 0, 0, false},
		{"100-100", 100, 100, false},
		{"180-", 180, inf, false},
		{"180 -", 180, inf, false},
		{"-90", 0, 90, false},
		{"-0", 0, 0, false},
		{"-5", 0, 5, false}, // "-5" is a range "at most 5", not a negative number
		{"-", 0, 0, true},
		{" - ", 0, 0, true},
		{"--5", 0, 0, true},
		{"100--5", 0, 0, true},
		{"1-2-3", 0, 0, true},
		{"140-100", 0, 0, true},
		{"", 0, 0, true},
		{"   ", 0, 0, true},
		{"fast", 0, 0, true},
		{"100-fast", 0, 0, true},
		{"fast-100", 0, 0, true},
		{"1,5", 0, 0, true},
	}
	for _, tt := range tests {
		min, max, err := ParseBPMRange(tt.in)
		if (err != nil) != tt.err || (!tt.err && (min != tt.min || max != tt.max)) {
			t.Errorf("ParseBPMRange(%q) = %v, %v, %v; want %v, %v, err=%v", tt.in, min, max, err, tt.min, tt.max, tt.err)
		}
	}
}
