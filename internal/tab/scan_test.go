package tab

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"tabfinder/internal/tabfiles"
)

func writeFile(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureTree lays out synthetic tabs the way a library looks: Artist/Album/x,
// Artist/x, a file at the root, files without metadata, a broken one, and non-tabs.
func fixtureTree(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	bare := tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Tempo: 120, Tracks: []tabfiles.GPTrack{{Name: "G", Strings: tabfiles.StdGuitar}}})
	full := tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Title: "Brass Kettle!", Artist: "Soilbed Quartet Official", Album: "Real Album", Tempo: 190,
		Tracks: []tabfiles.GPTrack{{Name: "G", Strings: []int{62, 57, 53, 48, 43, 36}}}})
	for path, data := range map[string][]byte{
		"Argyle Moth/Rent of Summer/nectar.gp3":             bare,
		"Argyle Moth/Gluttonous.gp3":                        bare,
		"Soilbed Quartet/Glass Orchard/Brass Kettle.gp3":    full,
		"Soilbed Quartet/Glass Orchard/Big.GP5":             full, // upper case extension, GP3 content
		"Soilbed Quartet/Glass Orchard/Half.gpx.crdownload": tabfiles.Zip(map[string][]byte{"Content/score.gpif": tabfiles.GPIF("Half", "Soilbed Quartet", "", nil)}),
		"loose_song.tg": tabfiles.TG1("", "", ""),
		"Soilbed Quartet/Glass Orchard/broken.gp5":           []byte("not a tab"),
		"Soilbed Quartet/Glass Orchard/notes.txt":            []byte("hello"),
		"Soilbed Quartet/cover.JPG":                          []byte("jpg"),
		"Soilbed Quartet/Glass Orchard/Brass Kettle.gp3.bak": full,
		"Zeta/z.zip": tabfiles.Zip(map[string][]byte{"a.tg": tabfiles.TG1("In Zip", "Zeta", "")}),
	} {
		writeFile(t, filepath.Join(root, path), data)
	}
	return root
}

func paths(songs []*Song) []string {
	out := make([]string, len(songs))
	for i, s := range songs {
		out[i] = s.Path
	}
	return out
}

func TestIsTabFile(t *testing.T) {
	for name, want := range map[string]bool{
		"a.gp": true, "a.gp3": true, "a.gp4": true, "a.gp5": true, "a.gpx": true, "a.gtp": true,
		"a.tg": true, "a.ptb": true, "a.zip": true, "a.crdownload": true, "a.gpx.crdownload": true,
		"a.GP5": true, "A.Gp3": true, "a.TG": true, "dir/a.gp5": true, ".gp5": true,
		"a.gp6": false, "a.gp5.bak": false, "gp5": false, "a": false, "": false, "a.txt": false, "a.mid": false, "a.pdf": false,
	} {
		if got := IsTabFile(name); got != want {
			t.Errorf("IsTabFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestScanFallbacks(t *testing.T) {
	bare := tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Tempo: 120, Tracks: []tabfiles.GPTrack{{Name: "G", Strings: tabfiles.StdGuitar}}})
	full := tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Title: "Brass Kettle!", Artist: "Soilbed Quartet Official", Album: "Real Album", Tempo: 190,
		Tracks: []tabfiles.GPTrack{{Name: "G", Strings: tabfiles.StdGuitar}}})
	tests := []struct {
		name                          string
		file                          string
		data                          []byte
		artist, album, title          string
		artistSrc, albumSrc, titleSrc Source
	}{
		{"artist/album folders", "Argyle Moth/Rent of Summer/nectar.gp3", bare, "Argyle Moth", "Rent of Summer", "Nectar", "path", "path", "path"},
		{"artist folder only", "Argyle Moth/Gluttonous.gp3", bare, "Argyle Moth", "", "Gluttonous", "path", "path", "path"},
		{"at the root", "Gluttonous.gp3", bare, "", "", "Gluttonous", "path", "path", "path"},
		{"deeper than artist/album", "Argyle Moth/Rent of Summer/CD 1/Gluttonous.gp3", bare, "Argyle Moth", "Rent of Summer", "Gluttonous", "path", "path", "path"},
		{"metadata wins over folders", "Soilbed Quartet/Glass/x.gp3", full, "Soilbed Quartet Official", "Real Album", "Brass Kettle!", "file", "file", "file"},
		{"title from file name drops the folder artist", "Argyle Moth/Argyle Moth - Gluttonous.gp3", bare, "Argyle Moth", "", "Gluttonous", "path", "path", "path"},
		{"broken file still gets fallbacks", "Argyle Moth/Rent of Summer/broken.gp3", []byte("junk"), "Argyle Moth", "Rent of Summer", "Broken", "path", "path", "path"},
	}
	for _, tt := range tests {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, tt.file), tt.data)
		s := Scan(filepath.Join(root, tt.file), root)
		if s.Path != tt.file {
			t.Errorf("%s: path = %q", tt.name, s.Path)
		}
		if s.Artist != tt.artist || s.Album != tt.album || s.Title != tt.title {
			t.Errorf("%s: artist/album/title = %q/%q/%q, want %q/%q/%q", tt.name, s.Artist, s.Album, s.Title, tt.artist, tt.album, tt.title)
		}
		if s.ArtistSource != tt.artistSrc || s.AlbumSource != tt.albumSrc || s.TitleSource != tt.titleSrc {
			t.Errorf("%s: sources = %s/%s/%s, want %s/%s/%s", tt.name, s.ArtistSource, s.AlbumSource, s.TitleSource, tt.artistSrc, tt.albumSrc, tt.titleSrc)
		}
	}
}

func TestApplyFallbacksPartialMetadata(t *testing.T) {
	// Each field falls back on its own.
	s := &Song{Artist: "Real Artist"}
	applyFallbacks(s, "Folder Artist/Folder Album/x - title.gp5")
	if s.Artist != "Real Artist" || s.ArtistSource != "file" || s.Album != "Folder Album" || s.AlbumSource != "path" || s.Title != "Title" || s.TitleSource != "path" {
		t.Errorf("song = %+v", s)
	}
	s = &Song{Album: "Real Album", Title: "Real Title"}
	applyFallbacks(s, "Folder Artist/Folder Album/x.gp5")
	if s.Artist != "Folder Artist" || s.ArtistSource != "path" || s.Album != "Real Album" || s.AlbumSource != "file" || s.Title != "Real Title" || s.TitleSource != "file" {
		t.Errorf("song = %+v", s)
	}
}

func TestScanPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "b", "c.gp3"), tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Title: "C", Tracks: []tabfiles.GPTrack{{Name: "G", Strings: tabfiles.StdGuitar}}}))
	s := Scan(filepath.Join(root, "a", "b", "c.gp3"), root)
	if s.Path != "a/b/c.gp3" || strings.Contains(s.Path, `\`) {
		t.Errorf("relative path = %q", s.Path)
	}
	// With the root one level down, the path is relative to that.
	if s := Scan(filepath.Join(root, "a", "b", "c.gp3"), filepath.Join(root, "a")); s.Path != "b/c.gp3" {
		t.Errorf("path = %q", s.Path)
	}

	t.Run("outside root keeps the path as given", func(t *testing.T) {
		other := t.TempDir()
		given := filepath.Join(root, "a", "b", "c.gp3")
		if s := Scan(given, other); s.Path != filepath.ToSlash(given) {
			t.Errorf("path = %q, want %q", s.Path, given)
		}
	})
	t.Run("missing file", func(t *testing.T) {
		s := Scan(filepath.Join(root, "nope", "x.gp3"), root)
		if s.Error == "" || s.Path != "nope/x.gp3" || s.Artist != "nope" || s.Title != "X" {
			t.Errorf("song = %+v", s)
		}
	})
}

func TestWalk(t *testing.T) {
	root := fixtureTree(t)
	collect := func(arg, root string) ([]*Song, error) {
		var out []*Song
		err := Walk(arg, root, func(s *Song) { out = append(out, s) })
		return out, err
	}

	t.Run("directory, recursive, tabs only, in walk order", func(t *testing.T) {
		songs, err := collect(root, "")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"Argyle Moth/Gluttonous.gp3",
			"Argyle Moth/Rent of Summer/nectar.gp3",
			"Soilbed Quartet/Glass Orchard/Big.GP5",
			"Soilbed Quartet/Glass Orchard/Brass Kettle.gp3",
			"Soilbed Quartet/Glass Orchard/Half.gpx.crdownload",
			"Soilbed Quartet/Glass Orchard/broken.gp5",
			"Zeta/z.zip",
			"loose_song.tg",
		}
		if got := paths(songs); !slices.Equal(got, want) {
			t.Errorf("paths = %q\nwant    %q", got, want)
		}
	})
	t.Run("results", func(t *testing.T) {
		songs, _ := collect(root, "")
		by := map[string]*Song{}
		for _, s := range songs {
			by[s.Path] = s
		}
		if s := by["Soilbed Quartet/Glass Orchard/Big.GP5"]; s.Format != "gp3" || s.Error != "" || s.Title != "Brass Kettle!" {
			t.Errorf("GP3 content in .GP5: %+v", s)
		}
		if s := by["Soilbed Quartet/Glass Orchard/Half.gpx.crdownload"]; s.Format != "gp7" || s.Title != "Half" || s.Error != "" {
			t.Errorf(".crdownload: %+v", s)
		}
		if s := by["Soilbed Quartet/Glass Orchard/broken.gp5"]; s.Error != "unknown file format" || s.Artist != "Soilbed Quartet" || s.Title != "Broken" {
			t.Errorf("broken: %+v", s)
		}
		if s := by["Zeta/z.zip"]; s.Title != "In Zip" || s.Artist != "Zeta" {
			t.Errorf("zip: %+v", s)
		}
		if s := by["loose_song.tg"]; s.Title != "Loose Song" || s.Artist != "" || s.Album != "" {
			t.Errorf("root file: %+v", s)
		}
	})
	t.Run("single file argument, root is its directory", func(t *testing.T) {
		songs, err := collect(filepath.Join(root, "Argyle Moth", "Gluttonous.gp3"), "")
		if err != nil || len(songs) != 1 {
			t.Fatalf("songs = %v, err = %v", songs, err)
		}
		if s := songs[0]; s.Path != "Gluttonous.gp3" || s.Artist != "" || s.Title != "Gluttonous" {
			t.Errorf("song = %+v", s)
		}
	})
	t.Run("single file argument with explicit root", func(t *testing.T) {
		songs, err := collect(filepath.Join(root, "Argyle Moth", "Gluttonous.gp3"), root)
		if err != nil || len(songs) != 1 {
			t.Fatalf("songs = %v, err = %v", songs, err)
		}
		if s := songs[0]; s.Path != "Argyle Moth/Gluttonous.gp3" || s.Artist != "Argyle Moth" {
			t.Errorf("song = %+v", s)
		}
	})
	t.Run("single non-tab file argument is scanned anyway", func(t *testing.T) {
		songs, err := collect(filepath.Join(root, "Soilbed Quartet", "Glass Orchard", "notes.txt"), "")
		if err != nil || len(songs) != 1 || songs[0].Error == "" {
			t.Errorf("songs = %+v, err = %v", songs, err)
		}
	})
	t.Run("explicit root for a directory", func(t *testing.T) {
		songs, err := collect(filepath.Join(root, "Soilbed Quartet"), root)
		if err != nil {
			t.Fatal(err)
		}
		if len(songs) == 0 || !strings.HasPrefix(songs[0].Path, "Soilbed Quartet/") || songs[0].Artist != "Soilbed Quartet Official" && songs[0].Artist != "Soilbed Quartet" {
			t.Errorf("first = %+v", songs[0])
		}
	})
	t.Run("directory argument is its own root", func(t *testing.T) {
		songs, _ := collect(filepath.Join(root, "Soilbed Quartet"), "")
		if len(songs) == 0 || songs[0].Path != "Glass Orchard/Big.GP5" || songs[0].Artist != "Soilbed Quartet Official" {
			t.Errorf("first = %+v", songs[0])
		}
		for _, s := range songs {
			if s.Title == "Broken" && (s.Artist != "Glass Orchard" || s.Album != "") {
				t.Errorf("broken = %+v", s)
			}
		}
	})
	t.Run("nonexistent argument", func(t *testing.T) {
		songs, err := collect(filepath.Join(root, "nope"), "")
		if err == nil || len(songs) != 0 {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("empty directory", func(t *testing.T) {
		songs, err := collect(t.TempDir(), "")
		if err != nil || len(songs) != 0 {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
}

// unreadableTree: a/x, then b/locked (mode 000), then c/y.
func unreadableTree(t *testing.T) (root, locked string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root can read mode 000 directories")
	}
	root = t.TempDir()
	data := tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Title: "T", Tracks: []tabfiles.GPTrack{{Name: "G", Strings: tabfiles.StdGuitar}}})
	writeFile(t, filepath.Join(root, "a", "x.gp3"), data)
	writeFile(t, filepath.Join(root, "c", "y.gp3"), data)
	locked = filepath.Join(root, "b", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	return root, locked
}

func TestWalkUnreadableDir(t *testing.T) {
	root, locked := unreadableTree(t)
	var got []string
	err := Walk(root, "", func(s *Song) { got = append(got, s.Path) })
	if err == nil || !strings.Contains(err.Error(), locked) {
		t.Fatalf("err = %v, want it to name %s", err, locked)
	}
	// The walk stops at the first error: what came before is kept, what follows isn't scanned.
	if !slices.Equal(got, []string{"a/x.gp3"}) {
		t.Errorf("songs = %q", got)
	}
}

func TestScanAll(t *testing.T) {
	root := fixtureTree(t)
	var walked []*Song
	if err := Walk(root, "", func(s *Song) { walked = append(walked, s) }); err != nil {
		t.Fatal(err)
	}
	all, err := ScanAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(walked) {
		t.Fatalf("ScanAll found %d, Walk %d", len(all), len(walked))
	}
	for i := range all {
		if !reflectEqual(all[i], walked[i]) {
			t.Errorf("song %d differs:\nScanAll %+v\nWalk    %+v", i, all[i], walked[i])
		}
	}

	t.Run("empty dir", func(t *testing.T) {
		songs, err := ScanAll(t.TempDir())
		if err != nil || len(songs) != 0 {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("missing root", func(t *testing.T) {
		songs, err := ScanAll(filepath.Join(root, "nope"))
		if err == nil || len(songs) != 0 {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("unreadable dir gives the songs so far and an error", func(t *testing.T) {
		root, locked := unreadableTree(t)
		songs, err := ScanAll(root)
		if err == nil || !strings.Contains(err.Error(), locked) {
			t.Errorf("err = %v", err)
		}
		if got := paths(songs); !slices.Equal(got, []string{"a/x.gp3"}) {
			t.Errorf("songs = %q", got)
		}
	})
	t.Run("root that is a file", func(t *testing.T) {
		file := filepath.Join(root, "Argyle Moth", "Gluttonous.gp3")
		songs, err := ScanAll(file)
		if err != nil || len(songs) != 1 {
			t.Fatalf("songs = %v, err = %v", songs, err)
		}
		// Unlike Walk, whose root for a file is its directory, the path
		// relative to the file itself is ".".
		if songs[0].Path != "." {
			t.Errorf("path = %q", songs[0].Path)
		}
	})
	t.Run("many files", func(t *testing.T) {
		big := t.TempDir()
		data := tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Title: "T", Tracks: []tabfiles.GPTrack{{Name: "G", Strings: tabfiles.StdGuitar}}})
		var want []string
		for _, a := range []string{"A", "B", "C", "D"} {
			for i := range 30 {
				p := filepath.Join(a, string(rune('a'+i%26))+string(rune('a'+i/26))+".gp3")
				writeFile(t, filepath.Join(big, p), data)
				want = append(want, filepath.ToSlash(p))
			}
		}
		slices.Sort(want) // WalkDir visits names in lexical order
		songs, err := ScanAll(big)
		if err != nil {
			t.Fatal(err)
		}
		if got := paths(songs); !slices.Equal(got, want) {
			t.Errorf("order differs: %q", got)
		}
	})
}

func reflectEqual(a, b *Song) bool {
	return a.Path == b.Path && a.Format == b.Format && a.Artist == b.Artist && a.Album == b.Album && a.Title == b.Title &&
		a.ArtistSource == b.ArtistSource && a.AlbumSource == b.AlbumSource && a.TitleSource == b.TitleSource &&
		a.Error == b.Error && reflect.DeepEqual(a.Tracks, b.Tracks) && slices.Equal(a.Tempos, b.Tempos)
}
