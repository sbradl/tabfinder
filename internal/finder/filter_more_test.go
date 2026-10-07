package finder

import (
	"math"
	"testing"

	"tabfinder/internal/tab"
)

func TestFilterMore(t *testing.T) {
	song := &tab.Song{
		Path:   "Argyle Moth/Rent of Summer/Argyle Moth - Nectar (ver 2).gp5",
		Artist: "Argyle Moth",
		Title:  "Nectar",
		Tracks: []tab.Track{
			{Name: "Drums", Drums: true, Instrument: "Drums"},
			{Name: "Lead", Pitches: []int{38, 45, 50, 55, 59, 64}},
			{Name: "Rhythm", Pitches: []int{40, 45, 50, 55, 59, 64}},
			{Name: "Bass", Pitches: []int{27, 32, 37, 42}},
			{Name: "Bass 7", Pitches: []int{35, 40, 45, 50, 55, 59, 64}},
		},
		Tempos: []tab.Tempo{{Bar: 1, BPM: 100}, {Bar: 5, BPM: 100.5}},
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
		{"bpm: in range", Filter{BPM: &BPMRange{90, 110}}, true},
		{"bpm: lower bound inclusive", Filter{BPM: &BPMRange{100, 100}}, true},
		{"bpm: upper bound inclusive", Filter{BPM: &BPMRange{0, 100.5}}, true},
		{"bpm: just above", Filter{BPM: &BPMRange{100.6, 200}}, false},
		{"bpm: open ended", Filter{BPM: &BPMRange{100.5, math.Inf(1)}}, true},
		{"bpm: not set", Filter{}, true},
		{"everything", Filter{Name: "nectar", Artist: "argyle", Tuning: "drop d", Strings: 6, BPM: &BPMRange{100, 101}}, true},
		{"everything but one", Filter{Name: "nectar", Artist: "argyle", Tuning: "drop d", Strings: 6, BPM: &BPMRange{200, 300}}, false},
	}
	for _, tt := range tests {
		if got := tt.f.Matches(song); got != tt.want {
			t.Errorf("%s: %+v.Matches() = %v, want %v", tt.name, tt.f, got, tt.want)
		}
	}
}

func TestFilterNameIsNotTheExtension(t *testing.T) {
	s := &tab.Song{Path: "Argyle Moth/argyle_moth_nectar.gp5.zip", Title: "Nectar"}
	for q, want := range map[string]bool{"gp5": false, "zip": false, "nectar": true, "moth nectar": true, "argyle_moth": false} {
		if got := (Filter{Name: q}).Matches(s); got != want {
			t.Errorf("name %q matches %v, want %v", q, got, want)
		}
	}
}

func TestFilterEmptySongs(t *testing.T) {
	bare := &tab.Song{Path: "x.gp5", Title: "X"}
	drumsOnly := &tab.Song{Path: "d.gp5", Tracks: []tab.Track{{Name: "Drums", Drums: true}}}
	for _, tt := range []struct {
		name string
		f    Filter
		s    *tab.Song
		want bool
	}{
		{"no tracks, name", Filter{Name: "x"}, bare, true},
		{"no tracks, tuning", Filter{Tuning: "drop d"}, bare, false},
		{"no tracks, strings", Filter{Strings: 6}, bare, false},
		{"no tempos, bpm", Filter{BPM: &BPMRange{0, math.Inf(1)}}, bare, false},
		{"drums only, strings", Filter{Strings: 6}, drumsOnly, false},
		{"drums only, tuning", Filter{Tuning: "standard"}, drumsOnly, false},
		{"empty filter, empty song", Filter{}, &tab.Song{}, true},
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
	for _, f := range []Filter{{Name: "a"}, {Artist: "a"}, {Tuning: "a"}, {Strings: 6}, {BPM: &BPMRange{}}} {
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
		r, err := ParseBPMRange(tt.in)
		min, max := r.Min, r.Max
		if (err != nil) != tt.err || (!tt.err && (min != tt.min || max != tt.max)) {
			t.Errorf("ParseBPMRange(%q) = %v, %v, %v; want %v, %v, err=%v", tt.in, min, max, err, tt.min, tt.max, tt.err)
		}
	}
}

func FuzzParseBPMRange(f *testing.F) {
	for _, s := range []string{"120", "100-140", "180-", "-90", "-", "--5", "", " 1 - 2 ", "1e3", "Inf", "-Inf", "0x10", "1_0"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		r, err := ParseBPMRange(in)
		min, max := r.Min, r.Max
		if err == nil && !(min <= max) {
			t.Errorf("ParseBPMRange(%q) = %v, %v without error", in, min, max)
		}
	})
}

// strconv.ParseFloat accepts "NaN", which compares false with everything:
// it passes the min > max check and then no tempo ever matches.
func TestParseBPMRangeRejectsNaN(t *testing.T) {
	for _, in := range []string{"NaN", "nan", "1-NaN", "NaN-5", "NaN-NaN"} {
		if r, err := ParseBPMRange(in); err == nil {
			t.Errorf("ParseBPMRange(%q) = %v without error", in, r)
		}
	}
}
