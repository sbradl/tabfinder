package tab

import (
	"slices"
	"testing"
	"unicode/utf8"
)

func TestNoteName(t *testing.T) {
	for midi, want := range map[int]string{0: "C", 1: "C#", 3: "Eb", 6: "F#", 8: "Ab", 10: "Bb", 40: "E", 127: "G", -1: "B", -2: "Bb", -12: "C", -13: "B"} {
		if got := noteName(midi); got != want {
			t.Errorf("noteName(%d) = %q, want %q", midi, got, want)
		}
	}
}

func TestTuningNameMore(t *testing.T) {
	tests := []struct {
		name    string
		pitches []int
		want    string
	}{
		{"nil", nil, ""},
		{"empty", []int{}, ""},
		{"4 standard", []int{28, 33, 38, 43}, "E Standard (E A D G)"},
		{"4 drop", []int{26, 33, 38, 43}, "Drop D (D A D G)"},
		{"5 standard", []int{23, 28, 33, 38, 43}, "B Standard (B E A D G)"},
		{"5 drop", []int{21, 28, 33, 38, 43}, "Drop A (A E A D G)"},
		{"6 standard", []int{40, 45, 50, 55, 59, 64}, "E Standard (E A D G B E)"},
		{"6 drop", []int{38, 45, 50, 55, 59, 64}, "Drop D (D A D G B E)"},
		{"7 standard", []int{35, 40, 45, 50, 55, 59, 64}, "B Standard (B E A D G B E)"},
		{"7 drop", []int{33, 40, 45, 50, 55, 59, 64}, "Drop A (A E A D G B E)"},
		{"8 strings", []int{30, 35, 40, 45, 50, 55, 59, 64}, "Custom (F# B E A D G B E)"},
		{"9 strings", []int{25, 30, 35, 40, 45, 50, 55, 59, 64}, "Custom (C# F# B E A D G B E)"},
		{"drop with another string off", []int{38, 45, 50, 55, 60, 64}, "Custom (D A D G C E)"},
		{"1 string", []int{40}, "Custom (E)"},
		{"2 strings", []int{40, 45}, "Custom (E A)"},
		{"drop only matches the 2-semitone shape", []int{39, 45, 50, 55, 59, 64}, "Custom (Eb A D G B E)"},
		{"negative MIDI", []int{-8, -3, 2, 7}, "E Standard (E A D G)"},
		{"spelling C#", []int{37, 42, 47, 52, 56, 61}, "C# Standard (C# F# B E Ab C#)"},
		{"spelling Eb", []int{39, 44, 49, 54, 58, 63}, "Eb Standard (Eb Ab C# F# Bb Eb)"},
		{"spelling F#", []int{42, 47, 52, 57, 61, 66}, "F# Standard (F# B E A C# F#)"},
		{"spelling Bb", []int{34, 39, 44, 49, 53, 58}, "Bb Standard (Bb Eb Ab C# F Bb)"},
	}
	for _, tt := range tests {
		if got := tuningName(tt.pitches); got != tt.want {
			t.Errorf("%s: tuningName(%v) = %q, want %q", tt.name, tt.pitches, got, tt.want)
		}
	}
}

func TestTitleFromFilenameMore(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		artists []string
		want    string
	}{
		{"crdownload", "Argyle Moth - Nectar.gp5.crdownload", []string{"Argyle Moth"}, "Nectar"},
		{"zip", "Argyle Moth - Nectar.gp5.zip", []string{"Argyle Moth"}, "Nectar"},
		{"plain zip", "Nectar.zip", nil, "Nectar"},
		{"other artist", "Soilbed Quartet - Brass Kettle.gp4", []string{"Argyle Moth"}, "Brass Kettle"},
		{"no separator, other artist", "Brass Kettle.gp4", []string{"Argyle Moth"}, "Brass Kettle"},
		{"artist with space only", "Argyle Moth Nectar.gp5", []string{"Argyle Moth"}, "Nectar"},
		{"artist dash no space", "Argyle Moth- Nectar.gp5", []string{"Argyle Moth"}, "Nectar"},
		{"dir artist", "Amber Marsh - Dusk.gp5", []string{"Someone", "Amber Marsh"}, "Dusk"},
		{"case insensitive artist", "ARGYLE MOTH - Nectar.gp5", []string{"Argyle Moth"}, "Nectar"},
		{"ver in parens", "Nectar (ver 2).gp5", nil, "Nectar"},
		{"ver in parens with text", "Nectar (ver 3 by Foo).gp5", nil, "Nectar"},
		{"v3 suffix", "Nectar v3.gp5", nil, "Nectar"},
		{"ver2 suffix", "Nectar ver2.gp5", nil, "Nectar"},
		{"number in parens", "Nectar (2).gp5", nil, "Nectar"},
		{"pro tag", "Nectar (Pro).gp5", nil, "Nectar"},
		{"complete tag", "Nectar (complete).gp5", nil, "Nectar"},
		{"id suffix", "Nectar - 779.gp5", nil, "Nectar"},
		{"junk bracket mail", "Nectar [by x@y.com].gp5", nil, "Nectar"},
		{"junk bracket www", "Nectar (Tabbed By Foo www.example.com).gp5", nil, "Nectar"},
		{"snake case", "as_lanterns_fade.gp5", nil, "As Lanterns Fade"},
		{"snake case with artist", "argyle_moth_nectar.gp5", []string{"Argyle Moth"}, "Nectar"},
		{"already capitalised stays", "the nectar.gp5", nil, "The Nectar"},
		{"umlaut lower", "äther_unpoppbar.gp4", nil, "Äther Unpoppbar"},
		{"empty", "", nil, ""},
		{"only extension", ".gp5", nil, ""},
		{"only junk", "(Tabbed By Foo).gp5", nil, ""},
		{"only version", "(ver 2).gp5", nil, ""},
		{"whitespace collapsed", "Nectar    Part   2.gp5", nil, "Nectar Part 2"},
		{"tablatures prefix", "www-tablatures-tk @ Nectar.gp4", nil, "Nectar"},
		{"blank artist ignored", "Nectar.gp5", []string{"", "  "}, "Nectar"},
	}
	for _, tt := range tests {
		if got := titleFromFilename(tt.file, tt.artists...); got != tt.want {
			t.Errorf("%s: titleFromFilename(%q, %q) = %q, want %q", tt.name, tt.file, tt.artists, got, tt.want)
		}
	}
}

func TestAddTempo(t *testing.T) {
	tests := []struct {
		name    string
		tempos  []Tempo
		want    []Tempo
		summary string
	}{
		{"none", nil, nil, ""},
		{"repeat dropped", []Tempo{{1, 120}, {2, 120}, {3, 120}}, []Tempo{{1, 120}}, "120"},
		{"return to earlier kept, shown once", []Tempo{{1, 120}, {5, 90}, {9, 120}}, []Tempo{{1, 120}, {5, 90}, {9, 120}}, "120, 90"},
		{"fractional", []Tempo{{1, 120.5}, {2, 120.5}, {3, 121}}, []Tempo{{1, 120.5}, {3, 121}}, "120.5, 121"},
		{"fraction vs integer differ", []Tempo{{1, 120}, {2, 120.5}}, []Tempo{{1, 120}, {2, 120.5}}, "120, 120.5"},
	}
	for _, tt := range tests {
		s := &Song{}
		for _, c := range tt.tempos {
			s.addTempo(c.Bar, c.BPM)
		}
		if !slices.Equal(s.Tempos, tt.want) {
			t.Errorf("%s: tempos = %v, want %v", tt.name, s.Tempos, tt.want)
		}
		if got := s.TempoSummary(); got != tt.summary {
			t.Errorf("%s: summary = %q, want %q", tt.name, got, tt.summary)
		}
	}
}

func FuzzTitleFromFilename(f *testing.F) {
	for _, s := range []string{"Argyle Moth - Nectar (ver 3 by Foo).gp5", "as_lanterns_fade.gp5", "", ".", " - ", "(", "[by", "Ä_ö.zip", "a - b - c.gp3"} {
		f.Add(s, "Argyle Moth", "Dir")
	}
	f.Fuzz(func(t *testing.T, name, artist, dir string) {
		got := titleFromFilename(name, artist, dir)
		if !utf8.ValidString(got) && utf8.ValidString(name) && utf8.ValidString(artist) && utf8.ValidString(dir) {
			t.Errorf("invalid UTF-8 result %q from valid input", got)
		}
	})
}

func FuzzParseBPMRange(f *testing.F) {
	for _, s := range []string{"120", "100-140", "180-", "-90", "-", "--5", "", " 1 - 2 ", "1e3", "Inf", "-Inf", "0x10", "1_0"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		min, max, err := ParseBPMRange(in)
		if err == nil && !(min <= max) {
			t.Errorf("ParseBPMRange(%q) = %v, %v without error", in, min, max)
		}
	})
}

// strconv.ParseFloat accepts "NaN", which compares false with everything:
// it passes the min > max check and then no tempo ever matches.
func TestParseBPMRangeRejectsNaN(t *testing.T) {
	for _, in := range []string{"NaN", "nan", "1-NaN", "NaN-5", "NaN-NaN"} {
		if min, max, err := ParseBPMRange(in); err == nil {
			t.Errorf("ParseBPMRange(%q) = %v, %v without error", in, min, max)
		}
	}
}

// Found by fuzzing: lowering the case changes the byte length of invalid UTF-8
// (each bad byte becomes U+FFFD, 3 bytes), so cutting the artist prefix off
// `name` by the length of the prefix found in `lower` goes out of range, or cuts
// in the wrong place. File names with Latin-1 bytes are common on Linux.
func TestTitleFromFilenameInvalidUTF8NoPanic(t *testing.T) {
	// The fuzzer's crash. Which cut is right for such a degenerate name is open (both bad bytes
	// lower-case to U+FFFD); it must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked: %v", r)
		}
	}()
	titleFromFilename("\xe7 0", "0", "\xa0")
}

func TestTitleFromFilenameInvalidUTF8(t *testing.T) {
	for _, tt := range []struct {
		name    string
		artists []string
		want    string
	}{
		{"\xe9 Nectar.gp5", []string{"\xe9"}, "Nectar"},
		{"\xc4ther - Weniger.gp4", []string{"\xc4ther"}, "Weniger"},
		{"\xc4ther - Weniger.gp4", []string{"Die \xc4ther"}, "Weniger"}, // no prefix match: the other-artist rule cuts it
		{"Die \xc4ther - Weniger.gp4", []string{"Die \xc4ther"}, "Weniger"},
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("titleFromFilename(%q, %q) panicked: %v", tt.name, tt.artists, r)
				}
			}()
			if got := titleFromFilename(tt.name, tt.artists...); got != tt.want {
				t.Errorf("titleFromFilename(%q, %q) = %q, want %q", tt.name, tt.artists, got, tt.want)
			}
		}()
	}
}
