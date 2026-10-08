package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tabfinder/internal/tabfiles"
)

// ratedTree is a made-up library of songs with notes: a slow one, a fast one with drums,
// and one in a format without notes.
func ratedTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	chords := func(dur, n int) []tabfiles.GPBeat {
		var out []tabfiles.GPBeat
		for range n {
			out = append(out, tabfiles.GPBeat{Dur: dur, Notes: []tabfiles.GPNote{{String: 6, Fret: 0}, {String: 5, Fret: 2}}})
		}
		return out
	}
	kickSnare := []tabfiles.GPBeat{
		{Notes: []tabfiles.GPNote{{String: 6, Fret: 36}}}, {Notes: []tabfiles.GPNote{{String: 6, Fret: 38}}},
		{Notes: []tabfiles.GPNote{{String: 6, Fret: 36}}}, {Notes: []tabfiles.GPNote{{String: 6, Fret: 38}}},
	}
	song := func(title string, tempo int, bar tabfiles.GPBar, tracks ...tabfiles.GPTrack) {
		bars := make([]tabfiles.GPBar, 40)
		for i := range bars {
			bars[i] = bar
		}
		p := filepath.Join(root, "Copper Wolves", title+".gp5")
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, tabfiles.GP(tabfiles.GPSpec{Version: "5.10", Title: title, Artist: "Copper Wolves", Tempo: tempo, Tracks: tracks, Bars: bars}), 0o644)
	}
	gtr := tabfiles.GPTrack{Name: "Rhythm", Strings: tabfiles.StdGuitar, Program: 30}
	drums := tabfiles.GPTrack{Name: "Drums", Drums: true, Strings: make([]int, 6)}
	song("Slow Pines", 80, tabfiles.GPBar{Beats: [][]tabfiles.GPBeat{chords(0, 4)}}, gtr)
	song("Iron Lung", 170, tabfiles.GPBar{Beats: [][]tabfiles.GPBeat{chords(2, 16), kickSnare}}, gtr, drums)
	os.WriteFile(filepath.Join(root, "Copper Wolves", "Tin Owl.tg"), tabfiles.TG1("Tin Owl", "Copper Wolves", ""), 0o644)
	return root
}

func TestCLIPartFilters(t *testing.T) {
	root := ratedTree(t)
	tests := []struct {
		name string
		args []string
		want []string // titles, in order
	}{
		{"easy rhythm guitar", []string{"-rhythm", "-3"}, []string{"Slow Pines"}},
		{"hard rhythm guitar", []string{"-rhythm", "5-"}, []string{"Iron Lung"}},
		{"a tag", []string{"-tag", "rhythm:fast"}, []string{"Iron Lung"}},
		{"no drums to play is fine", []string{"-drums", "-3"}, []string{"Slow Pines"}},
		{"hardest first, not rated last", []string{"-sort", "hardest"}, []string{"Iron Lung", "Slow Pines", "Tin Owl"}},
		{"easiest first", []string{"-sort", "easiest"}, []string{"Slow Pines", "Iron Lung", "Tin Owl"}},
	}
	for _, tt := range tests {
		out, errOut, code := run(t, "", "", append(slices.Clone(tt.args), root)...)
		if code != 0 {
			t.Errorf("%s: exit %d: %s", tt.name, code, errOut)
			continue
		}
		var got []string
		for _, p := range paths(out) {
			got = append(got, strings.TrimSuffix(strings.TrimSuffix(filepath.Base(p), ".gp5"), ".tg"))
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCLIPartColumns(t *testing.T) {
	out, _, _ := run(t, "", "", "-sort", "hardest", ratedTree(t))
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if lines[0] != "path\tartist\talbum\ttitle\ttempo\tinstruments\ttunings\tdifficulty\ttags" {
		t.Errorf("header %q", lines[0])
	}
	iron := strings.Split(lines[1], "\t")
	if len(iron) != 9 || !strings.HasPrefix(iron[7], "drums ") || !strings.Contains(iron[7], ", rhythm ") || !strings.Contains(iron[8], "rhythm: fast") {
		t.Errorf("Iron Lung: %q", iron)
	}
	if owl := strings.Split(lines[3], "\t"); len(owl) != 9 || owl[7] != "" || owl[8] != "" {
		t.Errorf("Tin Owl, not rated: %q", owl)
	}
}

func TestCLIBadPartFilters(t *testing.T) {
	root := ratedTree(t)
	for _, args := range [][]string{
		{"-rhythm", "hard"}, {"-lead", "5-2"}, {"-tag", "fast"}, {"-tag", "keys:fast"}, {"-sort", "loudest"},
	} {
		out, errOut, code := run(t, "", "", append(slices.Clone(args), root)...)
		// The flag package reports -tag's errors as "invalid value ... for flag -tag".
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "tabscan: ") && !strings.Contains(errOut, "flag "+args[0]) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
}
