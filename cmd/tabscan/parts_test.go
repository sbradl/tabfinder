package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tabfinder/internal/tabfiles"
	"tabfinder/internal/testlib"
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
		{"a range asks for the part", []string{"-drums", "-10"}, []string{"Iron Lung"}},
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

// TestServeParts is what the app's difficulty filters and sort ask for: a range of levels per
// part, and the order of the matches.
func TestServeParts(t *testing.T) {
	index := filepath.Join(t.TempDir(), "index.jsonl")
	testlib.WriteIndex(t, index, testlib.Songs())
	load := req(map[string]any{"op": "load", "index": index})
	for _, c := range []struct {
		name  string
		query map[string]any
		want  []string // titles of the matches, in order
	}{
		{"rhythm 5-7", map[string]any{"rhythm": "5-7"}, []string{"Where Rivers Seem to Rest", "Embrace the Unseen", "Only for the Brave"}},
		{"rhythm 5-7, hardest first", map[string]any{"rhythm": "5-7", "sort": "hardest"}, []string{"Only for the Brave", "Embrace the Unseen", "Where Rivers Seem to Rest"}},
		{"easy drums: songs with drums", map[string]any{"drums": "-3"}, []string{"Ruf nach Sonne"}},
		{"bass and lead", map[string]any{"bass": "5-", "lead": "9"}, []string{"Brass Kettle"}},
		{"with another field", map[string]any{"artist": "inkwell", "rhythm": "-6"}, []string{"Embrace the Unseen", "Paper Ride"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := session(t, load, req(map[string]any{"op": "search", "query": c.query}))
			if got := titlesOf(t, r[0], r[1]); !slices.Equal(got, c.want) {
				t.Errorf("titles %q, want %q", got, c.want)
			}
		})
	}
	// Easiest first: songs with parts by their hardest part, those without last.
	r := session(t, load, req(map[string]any{"op": "search", "query": map[string]any{"sort": "easiest"}}))
	got := titlesOf(t, r[0], r[1])
	if len(got) != len(testlib.Songs()) || got[0] != "Paper Ride" || slices.Index(got, "Brass Kettle") != 5 {
		t.Errorf("easiest first: %q", got)
	}
	// The rows carry the parts, for the app to show.
	for _, s := range list(t, r[0], "songs") {
		song := s.(map[string]any)
		if song["title"] == "Brass Kettle" && len(song["parts"].([]any)) != 4 {
			t.Errorf("Brass Kettle's parts: %v", song["parts"])
		}
	}
}
