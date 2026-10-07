package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"tabfinder/internal/tab"
	"tabfinder/internal/tabfiles"
	"tabfinder/internal/testlib"
)

// planOf plans the reorganization of a tree of tabs and returns it as src -> dst
// (only for files that move) and the clash groups.
func planOf(t *testing.T, files map[string][]byte, rename bool, skip ...string) (map[string]string, [][]string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Tabs")
	testlib.WriteFiles(t, root, files)
	var songs []*tab.Song
	if err := tab.Walk(root, root, func(s *tab.Song) { songs = append(songs, s) }); err != nil {
		t.Fatal(err)
	}
	p, err := newPlanner(root, skip, testRules)
	if err != nil {
		t.Fatal(err)
	}
	places, clashes := p.plan(songs, rename)
	got := map[string]string{}
	for _, m := range moves(places) {
		got[m.src] = m.dst
	}
	return got, clashes, root
}

// tg is a tab with metadata only.
func tg(title, artist, album string) []byte { return tabfiles.TG1(title, artist, album) }

func TestPlan(t *testing.T) {
	junk := []byte("not a tab")
	tests := []struct {
		name    string
		files   map[string][]byte
		rename  bool
		skip    []string
		want    map[string]string
		clashes [][]string
	}{
		{
			name:   "in <Artist>/<Album>/ keeps its folder, whatever the metadata says",
			files:  map[string][]byte{"Argyle Moth/Rent of Summer/nectar.tg": tg("Nectar", "Other Band", "Other Album")},
			rename: true,
			want:   map[string]string{"Argyle Moth/Rent of Summer/nectar.tg": "Argyle Moth/Rent of Summer/Nectar.tg"},
		},
		{
			name:   "deeper than <Artist>/<Album>/ is left alone",
			files:  map[string][]byte{"Argyle Moth/Rent of Summer/CD 2/nectar.tg": tg("Nectar", "Argyle Moth", "Other Album")},
			rename: false,
			want:   map[string]string{},
		},
		{
			name: "loose in an artist folder: the album folder that matches loosely",
			files: map[string][]byte{
				"Argyle Moth/Rent of Summer/Nectar.tg": tg("Nectar", "Argyle Moth", "Rent of Summer"),
				"Argyle Moth/Gluttonous.tg":            tg("Gluttonous", "Argyle Moth", "rent of summer (2004)"),
				"Argyle Moth/Pale Eyes.tg":             tg("Pale Eyes", "Argyle Moth", "The Rent  of Summer"),
			},
			rename: true,
			want: map[string]string{
				"Argyle Moth/Gluttonous.tg": "Argyle Moth/Rent of Summer/Gluttonous.tg",
				"Argyle Moth/Pale Eyes.tg":  "Argyle Moth/Rent of Summer/Pale Eyes.tg",
			},
		},
		{
			name:   "loose in an artist folder: no folder for the album yet, so it is made",
			files:  map[string][]byte{"Argyle Moth/Gluttonous.tg": tg("Gluttonous", "Argyle Moth", "2004 - Rent of Summer (Deluxe)")},
			rename: true,
			want:   map[string]string{"Argyle Moth/Gluttonous.tg": "Argyle Moth/Rent of Summer/Gluttonous.tg"},
		},
		{
			name: "loose in an artist folder: junk or missing albums stay put",
			files: map[string][]byte{
				"A/one.tg":   tg("One", "A", ""),
				"A/two.tg":   tg("Two", "A", "Single"),
				"A/three.tg": tg("Three", "A", "s/t"),
				"A/four.tg":  tg("Four", "A", "unknown"),
				"A/five.tg":  tg("Five", "A", "(2004)"),
				"A/six.tg":   junk,
			},
			rename: false,
			want:   map[string]string{},
		},
		{
			name:   "loose in an artist folder: the file's artist decides the folder",
			files:  map[string][]byte{"A/one.tg": tg("One", "B", "x"), "B/x/y.tg": tg("Y", "B", "x")},
			rename: false,
			want:   map[string]string{"A/one.tg": "B/x/one.tg"}, // B's album folder "x" matches
		},
		{
			name: "root files go to their artist's folder",
			files: map[string][]byte{
				"Argyle Moth/Rent of Summer/Nectar.tg": tg("Nectar", "Argyle Moth", "Rent of Summer"),
				"gluttonous.tg":                        tg("Gluttonous", "argyle moth", "Rent of Summer"),
				"pale_eyes.tg":                         tg("Pale Eyes", "The Argyle Moth", ""),
				"other.tg":                             tg("Other", "Someone New", "Their Album"),
				"noalbum.tg":                           tg("No Album", "Someone New", ""),
			},
			rename: true,
			want: map[string]string{
				"gluttonous.tg": "Argyle Moth/Rent of Summer/Gluttonous.tg",
				"pale_eyes.tg":  "Argyle Moth/Pale Eyes.tg",
				"other.tg":      "Someone New/Their Album/Other.tg",
				"noalbum.tg":    "Someone New/No Album.tg",
			},
		},
		{
			name: "root files without a usable artist stay",
			files: map[string][]byte{
				"a.tg": tg("A", "", "Alb"), "b.tg": tg("B", "Unknown", "Alb"), "c.tg": tg("C", "-", ""), "d.tg": tg("D", "Various", ""),
				"e.tg": tg("E", "Track 3", ""), "f.tg": junk,
			},
			rename: false,
			want:   map[string]string{},
		},
		{
			name:   "root file: a slash in the artist can't make a folder",
			files:  map[string][]byte{"x.tg": tg("X", "QR/ST", "")},
			rename: false,
			want:   map[string]string{"x.tg": "QR-ST/x.tg"},
		},
		{
			name: "artist matched to an existing folder: a song filed under the wrong artist",
			files: map[string][]byte{
				"Die Äther/Polka ist anders/Ruf nach Sonne.tg": tg("Ruf nach Sonne", "Die Äther", "Polka ist anders"),
				"Die Äther/weniger.tg":                         tg("Weniger", "Felix Ferien", "Endlich Ferien"),
				"Felix Ferien/Endlich Ferien/Klug.tg":          tg("Klug", "Felix Ferien", "Endlich Ferien"),
			},
			rename: true,
			want:   map[string]string{"Die Äther/weniger.tg": "Felix Ferien/Endlich Ferien/Weniger.tg"},
		},
		{
			name: "artist matched to an existing folder: not when that folder is skipped",
			files: map[string][]byte{
				"Die Äther/weniger.tg":                tg("Weniger", "Felix Ferien", ""),
				"Felix Ferien/Endlich Ferien/Klug.tg": tg("Klug", "Felix Ferien", "Endlich Ferien"),
			},
			rename: false,
			skip:   []string{"Felix Ferien"},
			want:   map[string]string{},
		},
		{
			name: "artist matched to an existing folder: the article is ignored",
			files: map[string][]byte{
				"Die Äther/Polka/Song.tg": tg("Song", "Die Äther", "Polka"),
				"äther.tg":                tg("Äther", "Äther", ""),
			},
			rename: false,
			want:   map[string]string{"äther.tg": "Die Äther/äther.tg"},
		},
		{
			name: "new albums are merged by similarity, with the nicest spelling",
			files: map[string][]byte{
				"Cousins of Marrow/a.tg": tg("A", "Cousins of Marrow", "Moonbreeder"),
				"Cousins of Marrow/b.tg": tg("B", "Cousins of Marrow", "Moonbreeder"),
				"Cousins of Marrow/c.tg": tg("C", "Cousins of Marrow", "Moonbreder"),
				"Cousins of Marrow/d.tg": tg("D", "Cousins of Marrow", "Follow the Weaver"),
			},
			rename: false,
			want: map[string]string{
				"Cousins of Marrow/a.tg": "Cousins of Marrow/Moonbreeder/a.tg",
				"Cousins of Marrow/b.tg": "Cousins of Marrow/Moonbreeder/b.tg",
				"Cousins of Marrow/c.tg": "Cousins of Marrow/Moonbreeder/c.tg",
				"Cousins of Marrow/d.tg": "Cousins of Marrow/Follow the Weaver/d.tg",
			},
		},
		{
			name: "short album names are only merged when equal",
			files: map[string][]byte{
				"A/a.tg": tg("A", "A", "Dawn"), "A/b.tg": tg("B", "A", "Down"), "A/c.tg": tg("C", "A", "dawn"),
			},
			rename: false,
			want:   map[string]string{"A/a.tg": "A/Dawn/a.tg", "A/b.tg": "A/Down/b.tg", "A/c.tg": "A/Dawn/c.tg"},
		},
		{
			name: "album aliases",
			files: map[string][]byte{
				"Ozmo/a.tg": tg("A", "Ozmo", "Cobalt Drizle"), "Ozmo/b.tg": tg("B", "Ozmo", "Cobalt Drizzle (1980)"),
			},
			rename: false,
			want:   map[string]string{"Ozmo/a.tg": "Ozmo/Cobalt Drizzle/a.tg", "Ozmo/b.tg": "Ozmo/Cobalt Drizzle/b.tg"},
		},
		{
			name: "an album in different artists' folders is kept apart",
			files: map[string][]byte{
				"A/a.tg": tg("A", "A", "Greatest Hits"), "B/b.tg": tg("B", "B", "Greatest Hits"),
			},
			rename: false,
			want:   map[string]string{"A/a.tg": "A/Greatest Hits/a.tg", "B/b.tg": "B/Greatest Hits/b.tg"},
		},
		{
			name: "rename: tabs are named after their song",
			files: map[string][]byte{
				"A/Alb/argyle_moth_nectar_ver3.tg":     tg("Nectar", "A", "Alb"),
				"A/Alb/pale eyes.gp5":                  tg("Pale Eyes", "A", "Alb"),
				"A/Alb/Almost Left.tg":                 tg("Almost Left", "A", "Alb"),
				"A/Alb/sodbury lane_umgezogen.gp3.zip": tabfiles.Zip(map[string][]byte{"x.tg": tg("Umgezogen", "A", "Alb")}),
				"A/Alb/half_down_load.gpx.crdownload":  tg("Half Down Load", "A", "Alb"),
			},
			rename: true,
			want: map[string]string{
				"A/Alb/argyle_moth_nectar_ver3.tg":     "A/Alb/Nectar.tg",
				"A/Alb/pale eyes.gp5":                  "A/Alb/Pale Eyes.gp5",
				"A/Alb/sodbury lane_umgezogen.gp3.zip": "A/Alb/Umgezogen.gp3.zip",
				"A/Alb/half_down_load.gpx.crdownload":  "A/Alb/Half Down Load.gpx.crdownload",
			},
		},
		{
			name: "rename=false: only folders change",
			files: map[string][]byte{
				"A/nectar_ver3.tg": tg("Nectar", "A", "Alb"),
				"loose song.tg":    tg("Loose Song", "A", "Alb"),
			},
			rename: false,
			want:   map[string]string{"A/nectar_ver3.tg": "A/Alb/nectar_ver3.tg", "loose song.tg": "A/Alb/loose song.tg"},
		},
		{
			name: "same song name twice in one folder: both keep their names, reported",
			files: map[string][]byte{
				"A/Alb/Nectar (ver 1).tg": tg("Nectar", "A", "Alb"),
				"A/Alb/Nectar (ver 2).tg": tg("Nectar", "A", "Alb"),
				"A/Alb/Other.tg":          tg("Other", "A", "Alb"),
			},
			rename:  true,
			want:    map[string]string{},
			clashes: [][]string{{"A/Alb/Nectar (ver 1).tg", "A/Alb/Nectar (ver 2).tg"}},
		},
		{
			name: "same song name in one folder, one almost left: all keep their names",
			files: map[string][]byte{
				"A/Alb/Nectar.tg":      tg("Nectar", "A", "Alb"),
				"A/Alb/nectar_ver2.tg": tg("Nectar", "A", "Alb"),
			},
			rename:  true,
			want:    map[string]string{},
			clashes: [][]string{{"A/Alb/Nectar.tg", "A/Alb/nectar_ver2.tg"}},
		},
		{
			name: "same song name in different formats is no clash",
			files: map[string][]byte{
				"A/Alb/nectar_a.tg":  tg("Nectar", "A", "Alb"),
				"A/Alb/nectar_b.gp5": tg("Nectar", "A", "Alb"),
			},
			rename: true,
			want:   map[string]string{"A/Alb/nectar_a.tg": "A/Alb/Nectar.tg", "A/Alb/nectar_b.gp5": "A/Alb/Nectar.gp5"},
		},
		{
			name: "same song name from another folder: all keep their names, reported in scan order",
			files: map[string][]byte{
				"A/Alb/Nectar.tg": tg("Nectar", "A", "Alb"),
				"A/nectar_v2.tg":  tg("Nectar", "A", "Alb"),
			},
			rename:  true,
			want:    map[string]string{"A/nectar_v2.tg": "A/Alb/nectar_v2.tg"},
			clashes: [][]string{{"A/Alb/Nectar.tg", "A/Alb/nectar_v2.tg"}},
		},
		{
			name: "anything else in the way blocks a rename: reported, name kept",
			files: map[string][]byte{
				"A/Alb/nectar_v2.tg":    tg("Nectar", "A", "Alb"),
				"A/Alb/Nectar.tg/x.txt": []byte("a folder with the name the tab would get"),
			},
			rename:  true,
			want:    map[string]string{},
			clashes: [][]string{{"A/Alb/nectar_v2.tg", "A/Alb/Nectar.tg"}},
		},
		{
			name: "other collisions get a number, nothing is overwritten",
			files: map[string][]byte{
				"A/Alb/Nectar.tg": tg("Nectar", "A", "Alb"),
				"A/Nectar.tg":     tg("Nectar", "A", "Alb"),
				"Nectar.tg":       tg("Nectar", "A", "Alb"),
			},
			rename: true,
			want:   map[string]string{"A/Nectar.tg": "A/Alb/Nectar (2).tg", "Nectar.tg": "A/Alb/Nectar (3).tg"},
		},
		{
			name: "skipped folders are untouched, nested ones too",
			files: map[string][]byte{
				"Scales/major_scale.tg":                   tg("Major Scale", "Scales", "Alb"),
				"Rock&Pop/loose song.tg":                  tg("Loose Song", "Other", "Alb"),
				"Fit for a Lighthouse/Chart Happens/x.tg": tg("X", "Fit for a Lighthouse", ""),
				"Fit for a Lighthouse/other_song.tg":      tg("Other Song", "Fit for a Lighthouse", ""),
				"Scales2/minor_scale.tg":                  tg("Minor Scale", "Scales2", ""),
			},
			rename: true,
			skip:   []string{"Scales", "Rock&Pop", "Fit for a Lighthouse/Chart Happens"},
			want: map[string]string{
				"Fit for a Lighthouse/other_song.tg": "Fit for a Lighthouse/Other Song.tg",
				"Scales2/minor_scale.tg":             "Scales2/Minor Scale.tg",
			},
		},
		{
			name: "skipped folder names are matched whole",
			files: map[string][]byte{
				"Guitar/a_b.tg":      tg("A B", "Guitar", ""),
				"Rhythm Star/a_b.tg": tg("A B", "Rhythm Star", ""),
			},
			rename: true,
			skip:   []string{"Guitar"},
			want:   map[string]string{"Rhythm Star/a_b.tg": "Rhythm Star/A B.tg"},
		},
		{
			name:   "an unparseable file is renamed after its file name",
			files:  map[string][]byte{"A/Alb/some_song.gp5": junk},
			rename: true,
			want:   map[string]string{"A/Alb/some_song.gp5": "A/Alb/Some Song.gp5"},
		},
		{
			name:   "empty library",
			files:  map[string][]byte{"A/readme.txt": junk},
			rename: true,
			want:   map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, clashes, _ := planOf(t, tt.files, tt.rename, tt.skip...)
			if !maps.Equal(got, tt.want) {
				t.Errorf("moves:\n%s\nwant:\n%s", fmtMoves(got), fmtMoves(tt.want))
			}
			if !slices.EqualFunc(clashes, tt.clashes, slices.Equal) {
				t.Errorf("clashes = %q, want %q", clashes, tt.clashes)
			}
		})
	}
}

func fmtMoves(m map[string]string) string {
	var lines []string
	for _, k := range slices.Sorted(maps.Keys(m)) {
		lines = append(lines, fmt.Sprintf("  %s -> %s", k, m[k]))
	}
	if len(lines) == 0 {
		return "  (none)"
	}
	return fmt.Sprint(joinLines(lines))
}

func joinLines(l []string) string {
	s := ""
	for i, x := range l {
		if i > 0 {
			s += "\n"
		}
		s += x
	}
	return s
}

func TestPlanNeverLosesOrOverwritesFiles(t *testing.T) {
	files := testlib.TreeFiles()
	for _, rename := range []bool{true, false} {
		got, _, root := planOf(t, files, rename, testConfig.Skip...)
		finals := map[string]string{}
		for src, dst := range got {
			if prev, dup := finals[dst]; dup {
				t.Errorf("rename=%v: %s and %s both go to %s", rename, prev, src, dst)
			}
			finals[dst] = src
			if _, err := os.Lstat(filepath.Join(root, dst)); err == nil {
				if _, moving := got[dst]; !moving {
					t.Errorf("rename=%v: %s -> %s overwrites an existing file", rename, src, dst)
				}
			}
		}
		// Every tab ends up at exactly one place.
		final := map[string]bool{}
		for src := range files {
			if !tab.IsTabFile(src) {
				continue
			}
			dst := src
			if d, ok := got[src]; ok {
				dst = d
			}
			if final[dst] {
				t.Errorf("rename=%v: two tabs end up at %s", rename, dst)
			}
			final[dst] = true
		}
	}
}

func TestMoves(t *testing.T) {
	files := testlib.TreeFiles()
	var first []move
	for i := range 5 {
		root := filepath.Join(t.TempDir(), "Tabs")
		testlib.WriteFiles(t, root, files)
		var songs []*tab.Song
		tab.Walk(root, root, func(s *tab.Song) { songs = append(songs, s) })
		p, _ := newPlanner(root, testConfig.Skip, testRules)
		places, _ := p.plan(songs, true)
		ms := moves(places)
		if i == 0 {
			first = ms
			// Placements are in scan order, and the moves keep that order.
			var srcs []string
			for _, m := range ms {
				srcs = append(srcs, m.src)
			}
			var scan []string
			for _, s := range songs {
				scan = append(scan, s.Path)
			}
			if !isSubsequence(srcs, scan) {
				t.Errorf("moves are not in scan order:\n%q\n%q", srcs, scan)
			}
			// Files already in place are not moved.
			for _, m := range ms {
				if m.src == m.dst {
					t.Errorf("move of %s onto itself", m.src)
				}
			}
			for _, inPlace := range []string{"Amber Marsh/Tide of Lanterns/Where Rivers Seem to Rest.gp3", "Gorsewick/Gravel Hymns/Quartz.gpx.crdownload", "Scales/major_scale.gp3"} {
				if slices.ContainsFunc(ms, func(m move) bool { return m.src == inPlace }) {
					t.Errorf("%s is moved", inPlace)
				}
			}
		} else if !slices.Equal(ms, first) {
			t.Fatalf("run %d: moves differ:\n%v\n%v", i, ms, first)
		}
	}
	if len(first) != 10 {
		t.Errorf("%d moves, want 10: %v", len(first), first)
	}
}

func isSubsequence(sub, all []string) bool {
	i := 0
	for _, s := range all {
		if i < len(sub) && sub[i] == s {
			i++
		}
	}
	return i == len(sub)
}
