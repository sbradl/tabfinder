package tab

import "testing"

func TestTuningName(t *testing.T) {
	tests := []struct {
		pitches []int
		want    string
	}{
		{[]int{40, 45, 50, 55, 59, 64}, "E Standard (E A D G B E)"},
		{[]int{38, 45, 50, 55, 59, 64}, "Drop D (D A D G B E)"},
		{[]int{36, 43, 48, 53, 57, 62}, "Drop C (C G C F A D)"},
		{[]int{37, 42, 47, 52, 56, 61}, "C# Standard (C# F# B E Ab C#)"},
		{[]int{35, 40, 45, 50, 55, 59, 64}, "B Standard (B E A D G B E)"},
		{[]int{28, 33, 38, 43}, "E Standard (E A D G)"},
		{[]int{38, 43, 50, 55, 59, 62}, "Custom (D G D G B D)"},
	}
	for _, tt := range tests {
		if got := tuningName(tt.pitches); got != tt.want {
			t.Errorf("tuningName(%v) = %q, want %q", tt.pitches, got, tt.want)
		}
	}
}

func TestTitleFromFilename(t *testing.T) {
	tests := []struct {
		name, artist, want string
	}{
		{"Argyle Moth - Nectar (ver 3 by Foo).gp5", "Argyle Moth", "Nectar"},
		{"as_lanterns_fade_the_ferry.gp5", "As Lanterns Fade", "The Ferry"},
		{"ripple_ver2_C.gp3", "As Lanterns Fade", "Ripple C"},
		{"www-tablatures-tk @ Die Äther - Unpoppbar (2).gp4", "Die Äther", "Unpoppbar"},
		{"Ferien, Felix - Weniger.gp4", "Felix Ferien", "Weniger"},
		{"Cinder Choir - Viva La Lumina - 779.gp5", "Cinder Choir", "Viva La Lumina"},
		{"as_lanterns_fade_dazzled.gpx.crdownload", "As Lanterns Fade", "Dazzled"},
		{"Bring The Rain Home (Tabbed By Jane Doe - www.example.org Tabber1).gp5", "Hollow Shores Bloom", "Bring The Rain Home"},
	}
	for _, tt := range tests {
		if got := titleFromFilename(tt.name, tt.artist); got != tt.want {
			t.Errorf("titleFromFilename(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestTempoSummary(t *testing.T) {
	s := &Song{}
	for _, c := range []Tempo{{1, 120}, {5, 120}, {9, 90.5}, {17, 120}, {33, 140}} {
		s.addTempo(c.Bar, c.BPM)
	}
	if len(s.Tempos) != 4 { // the repeated 120 at bar 5 is dropped
		t.Errorf("tempos = %v, want 4 entries", s.Tempos)
	}
	if got, want := s.TempoSummary(), "120, 90.5, 140"; got != want {
		t.Errorf("tempoSummary() = %q, want %q", got, want)
	}
}
