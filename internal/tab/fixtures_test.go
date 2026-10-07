package tab

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tabfinder/internal/tabfiles"
)

// The fixtures F1-F14 of the test plan: one made-up tab of each kind, built by package
// tabfiles (no real, copyrighted tabs), laid out as a small library in testdata/library
// and described by hand in testdata/expected.json. `go test -update` rewrites both from
// the table below; the expectations are written out here, not read back from the parser.

// (package testlib imports this package, so its -update helpers can't be used here.)
var update = flag.Bool("update", false, "rewrite the fixture files and expected.json")

const (
	fixturesDir  = "testdata/library"
	expectedFile = "testdata/expected.json"
)

// want is what a fixture must parse to. Empty ErrorContains means no error.
type want struct {
	Format        string  `json:"format"`
	Title         string  `json:"title"`
	Artist        string  `json:"artist"`
	Album         string  `json:"album"`
	TitleSource   string  `json:"titleSource"`
	ArtistSource  string  `json:"artistSource"`
	AlbumSource   string  `json:"albumSource"`
	Tracks        []Track `json:"tracks,omitempty"`
	Tempos        []Tempo `json:"tempos,omitempty"`
	ErrorContains string  `json:"errorContains,omitempty"`
	// TempoPrefixOf: the tempos are the first ones of that fixture's (a cut file).
	TempoPrefixOf string `json:"tempoPrefixOf,omitempty"`
}

type fixture struct {
	id   string
	path string // in the library, relative to its root
	data []byte
	want want
}

// junk is deterministic filler for the parts of a file the parser doesn't read.
func junk(n int) []byte {
	b := make([]byte, n)
	x := uint32(12345)
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = byte(x >> 24)
	}
	return b
}

func guitar(name string, program int, strings ...int) tabfiles.GPTrack {
	return tabfiles.GPTrack{Name: name, Strings: strings, Program: program}
}

var (
	drums    = tabfiles.GPTrack{Name: "Drums", Drums: true, Strings: make([]int, 6)}
	bass4    = tabfiles.GPTrack{Name: "Bass", Strings: []int{43, 38, 33, 28}, Program: 33}
	wantDrum = func(name string) Track { return Track{Name: name, Instrument: "Drums", Drums: true} }
)

func bars(n int, changes map[int]int) []tabfiles.GPBar { // changes: 0-based bar -> BPM
	out := make([]tabfiles.GPBar, n)
	for i, bpm := range changes {
		out[i].Tempo = bpm
	}
	return out
}

func fixtures() []fixture {
	f3 := tabfiles.GP(tabfiles.GPSpec{
		Version: "5.10", Title: "Paper Weather", Artist: "Seventh Floor", Album: "Static Garden", Tempo: 140,
		Tracks: []tabfiles.GPTrack{
			guitar("Rhythm 7", 30, 64, 59, 55, 50, 45, 40, 35),
			guitar("Lead", 29, 62, 57, 53, 48, 43, 36),
			guitar("Pad", 25, tabfiles.StdGuitar...),
		},
		Bars: bars(60, map[int]int{2: 70, 4: 140, 7: 155, 10: 140, 40: 90}),
	})
	f3Tempos := []Tempo{{1, 140}, {3, 70}, {5, 140}, {8, 155}, {11, 140}, {41, 90}}
	f3Tracks := []Track{
		{Name: "Rhythm 7", Instrument: "Distortion Guitar", Pitches: []int{35, 40, 45, 50, 55, 59, 64}, Tuning: "B Standard (B E A D G B E)"},
		{Name: "Lead", Instrument: "Overdriven Guitar", Pitches: []int{36, 43, 48, 53, 57, 62}, Tuning: "Drop C (C G C F A D)"},
		{Name: "Pad", Instrument: "Acoustic Guitar (steel)", Pitches: []int{40, 45, 50, 55, 59, 64}, Tuning: "E Standard (E A D G B E)"},
	}
	stdLowFirst := []int{40, 45, 50, 55, 59, 64}
	const stdLabel = "E Standard (E A D G B E)"

	gp6Guitar := tabfiles.GPIF6Track{Name: "Guitar", Instrument: "e-gtr-clean", Program: 27, Channel: 0, Pitches: "40 45 50 55 59 64"}

	return []fixture{
		{"F1", "Neon Harbor/Glass Tides/Salt Lamp.gp3",
			tabfiles.GP(tabfiles.GPSpec{
				Version: "3.00", Title: "Salt Lamp", Artist: "Neon Harbor", Album: "Glass Tides", Tempo: 132,
				Tracks: []tabfiles.GPTrack{guitar("Lead Guitar", 29, tabfiles.StdGuitar...)},
				Bars:   bars(8, map[int]int{3: 96, 6: 132}),
			}),
			want{Format: "gp3", Title: "Salt Lamp", Artist: "Neon Harbor", Album: "Glass Tides", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: []Track{{Name: "Lead Guitar", Instrument: "Overdriven Guitar", Pitches: stdLowFirst, Tuning: stdLabel}},
				Tempos: []Tempo{{1, 132}, {4, 96}, {7, 132}}}},

		{"F2", "Quiet Engines/Slow Orbit/Tin Moon.gp4",
			tabfiles.GP(tabfiles.GPSpec{
				Version: "4.06", Title: "Tin Moon", Artist: "Quiet Engines", Album: "Slow Orbit", Tempo: 90,
				Tracks: []tabfiles.GPTrack{guitar("Guitar", 30, 64, 59, 55, 50, 45, 38), bass4, drums},
				Bars:   bars(6, map[int]int{2: 100}),
			}),
			want{Format: "gp4", Title: "Tin Moon", Artist: "Quiet Engines", Album: "Slow Orbit", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: []Track{
					{Name: "Guitar", Instrument: "Distortion Guitar", Pitches: []int{38, 45, 50, 55, 59, 64}, Tuning: "Drop D (D A D G B E)"},
					{Name: "Bass", Instrument: "Electric Bass (finger)", Pitches: []int{28, 33, 38, 43}, Tuning: "E Standard (E A D G)"},
					wantDrum("Drums"),
				},
				Tempos: []Tempo{{1, 90}, {3, 100}}}},

		{"F3", "Seventh Floor/Static Garden/Paper Weather.gp5", f3,
			want{Format: "gp5", Title: "Paper Weather", Artist: "Seventh Floor", Album: "Static Garden", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: f3Tracks, Tempos: f3Tempos}},

		// GP5 5.00 content under a .gp3 name; no artist or album in the file, so the folders give them.
		{"F4", "Seventh Floor/Static Garden/Misnamed.gp3",
			tabfiles.GP(tabfiles.GPSpec{
				Version: "5.00", Title: "Rename Me", Tempo: 100,
				Tracks: []tabfiles.GPTrack{guitar("Guitar", 27, tabfiles.StdGuitar...)},
				Bars:   bars(4, map[int]int{2: 110}),
			}),
			want{Format: "gp5", Title: "Rename Me", Artist: "Seventh Floor", Album: "Static Garden", TitleSource: "file", ArtistSource: "path", AlbumSource: "path",
				Tracks: []Track{{Name: "Guitar", Instrument: "Electric Guitar (clean)", Pitches: stdLowFirst, Tuning: stdLabel}},
				Tempos: []Tempo{{1, 100}, {3, 110}}}},

		{"F5", "Paper Satellites/Cold Start.gpx",
			tabfiles.GPX(tabfiles.GPIF6("Cold Start", "Paper Satellites", "Orbit Notes", [][2]float64{{0, 120}, {4, 90}},
				gp6Guitar, tabfiles.GPIF6Track{Name: "Drums", Instrument: "drumkit", Channel: 9, Percussion: true}), true),
			want{Format: "gp6", Title: "Cold Start", Artist: "Paper Satellites", Album: "Orbit Notes", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: []Track{{Name: "Guitar", Instrument: "Electric Guitar (clean)", Pitches: stdLowFirst, Tuning: stdLabel}, wantDrum("Drums")},
				Tempos: []Tempo{{1, 120}, {5, 90}}}},

		{"F6", "Orbit Club/Night Shift.gp",
			tabfiles.GP7(tabfiles.GPIF("Night Shift", "Orbit Club", "Late Hours", [][2]float64{{0, 128}, {2, 120.5}, {6, 128}},
				tabfiles.GPIFTrack{Name: "Eight", Instrument: "Electric Guitar", Kind: "electricGuitar", Pitches: "30 35 40 45 50 55 59 64"},
				tabfiles.GPIFTrack{Name: "Kit", Instrument: "Drum Kit", Kind: "drumKit"},
				tabfiles.GPIFTrack{Name: "Low", Instrument: "Electric Bass", Kind: "bass", Pitches: "28 33 38 43"})),
			want{Format: "gp7", Title: "Night Shift", Artist: "Orbit Club", Album: "Late Hours", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: []Track{
					{Name: "Eight", Instrument: "Electric Guitar", Pitches: []int{30, 35, 40, 45, 50, 55, 59, 64}, Tuning: "Custom (F# B E A D G B E)"},
					wantDrum("Kit"),
					{Name: "Low", Instrument: "Electric Bass", Pitches: []int{28, 33, 38, 43}, Tuning: "E Standard (E A D G)"},
				},
				Tempos: []Tempo{{1, 128}, {3, 120.5}, {7, 128}}}},

		// TuxGuitar 1: only the header is read; the folders give artist and album.
		{"F7", "Moss Cathedral/Hollow Choir/Lantern.tg",
			append(tabfiles.TG1("Lantern", "", ""), junk(300)...),
			want{Format: "tg", Title: "Lantern", Artist: "Moss Cathedral", Album: "Hollow Choir", TitleSource: "file", ArtistSource: "path", AlbumSource: "path"}},

		{"F8", "Moss Cathedral/Hollow Choir/Under Glass.tg",
			tabfiles.Zip(map[string][]byte{
				"version.txt": []byte("2.0"),
				"content.xml": tabfiles.TG2("Under Glass", "Moss Cathedral", "Hollow Choir", []int{100, 100, 140},
					[]tabfiles.TG2Channel{{ID: 0, Program: 29}, {ID: 1, Program: 33}, {ID: 2, Bank: 128}},
					tabfiles.TG2Track{Name: "Guitar", Channel: 0, Strings: tabfiles.StdGuitar},
					tabfiles.TG2Track{Name: "Bass", Channel: 1, Strings: []int{43, 38, 33, 28}},
					tabfiles.TG2Track{Name: "Drums", Channel: 2, Strings: []int{0}}),
			}, "version.txt", "content.xml"),
			want{Format: "tg", Title: "Under Glass", Artist: "Moss Cathedral", Album: "Hollow Choir", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: []Track{
					{Name: "Guitar", Instrument: "Overdriven Guitar", Pitches: stdLowFirst, Tuning: stdLabel},
					{Name: "Bass", Instrument: "Electric Bass (finger)", Pitches: []int{28, 33, 38, 43}, Tuning: "E Standard (E A D G)"},
					wantDrum("Drums"),
				},
				Tempos: []Tempo{{1, 100}, {3, 140}}}},

		// Power Tab 2: no album (not a public release), so the folder gives it.
		{"F9", "Dust Kites/Raw Takes/Rope Bridge.ptb",
			append(tabfiles.PTB(3, "Rope Bridge", "Dust Kites", nil), junk(200)...),
			want{Format: "ptb", Title: "Rope Bridge", Artist: "Dust Kites", Album: "Raw Takes", TitleSource: "file", ArtistSource: "file", AlbumSource: "path"}},

		// A half-downloaded GP6 file, plain (uncompressed) container; one level of folders.
		{"F10", "Paper Satellites/Half Down.gpx.crdownload",
			tabfiles.GPX(tabfiles.GPIF6("Half Down", "Paper Satellites", "", [][2]float64{{0, 110}},
				tabfiles.GPIF6Track{Name: "Guitar", Instrument: "a-gtr-steel", Program: 25, Channel: 1, Pitches: "38 45 50 55 59 64"}), false),
			want{Format: "gp6", Title: "Half Down", Artist: "Paper Satellites", Album: "", TitleSource: "file", ArtistSource: "file", AlbumSource: "path",
				Tracks: []Track{{Name: "Guitar", Instrument: "Acoustic Guitar (steel)", Pitches: []int{38, 45, 50, 55, 59, 64}, Tuning: "Drop D (D A D G B E)"}},
				Tempos: []Tempo{{1, 110}}}},

		{"F11", "Neon Harbor/Glass Tides/Zipped.zip",
			tabfiles.Zip(map[string][]byte{"Zipped Song.gp5": tabfiles.GP(tabfiles.GPSpec{
				Version: "5.10", Title: "Zipped Song", Artist: "Neon Harbor", Album: "Glass Tides", Tempo: 120,
				Tracks: []tabfiles.GPTrack{guitar("Guitar", 27, tabfiles.StdGuitar...), bass4},
				Bars:   bars(4, nil),
			})}),
			want{Format: "gp5", Title: "Zipped Song", Artist: "Neon Harbor", Album: "Glass Tides", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: []Track{
					{Name: "Guitar", Instrument: "Electric Guitar (clean)", Pitches: stdLowFirst, Tuning: stdLabel},
					{Name: "Bass", Instrument: "Electric Bass (finger)", Pitches: []int{28, 33, 38, 43}, Tuning: "E Standard (E A D G)"},
				},
				Tempos: []Tempo{{1, 120}}}},

		// The first half of F3: header and tracks read, the bars cut.
		{"F12", "Seventh Floor/Static Garden/Cut Short.gp5", f3[:len(f3)/2],
			want{Format: "gp5", Title: "Paper Weather", Artist: "Seventh Floor", Album: "Static Garden", TitleSource: "file", ArtistSource: "file", AlbumSource: "file",
				Tracks: f3Tracks, ErrorContains: "tempo changes incomplete", TempoPrefixOf: "F3"}},

		{"F13", "Old Machines/Pre-History.gtp",
			append(append([]byte{24}, []byte("FICHIER GUITAR PRO v2.20\x00\x00\x00\x00\x00\x00")...), junk(120)...),
			want{Title: "Pre-History", Artist: "Old Machines", TitleSource: "path", ArtistSource: "path", AlbumSource: "path", ErrorContains: "Guitar Pro 2 files are not supported"}},

		{"F14", "Old Machines/Ancient Riff.ptb",
			append([]byte("ptab\x01\x00"), junk(120)...),
			want{Title: "Ancient Riff", Artist: "Old Machines", TitleSource: "path", ArtistSource: "path", AlbumSource: "path", ErrorContains: "Power Tab 1.0 files are not supported"}},
	}
}

// TestFixturesUpToDate checks (or with -update rewrites) the files in testdata
// against the builders above, so the committed bytes can't drift from their description.
func TestFixturesUpToDate(t *testing.T) {
	expected := map[string]want{}
	for _, f := range fixtures() {
		expected[f.path] = f.want
		p := filepath.Join(fixturesDir, filepath.FromSlash(f.path))
		if *update {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, f.data, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: %v (run with -update to create it)", f.id, err)
		} else if !bytes.Equal(got, f.data) {
			t.Errorf("%s: %s differs from what its builder makes (run with -update)", f.id, f.path)
		}
	}
	js, err := json.MarshalIndent(expected, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	js = append(js, '\n')
	if *update {
		if err := os.WriteFile(expectedFile, js, 0o644); err != nil {
			t.Fatal(err)
		}
	} else if got, err := os.ReadFile(expectedFile); err != nil || !bytes.Equal(got, js) {
		t.Errorf("%s differs from the table in fixtures_test.go (run with -update): %v", expectedFile, err)
	}
}

func loadExpected(t testing.TB) map[string]want {
	t.Helper()
	b, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]want
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// U-TAB-03: every fixture file against expected.json.
func TestFixturesAgainstExpected(t *testing.T) {
	expected := loadExpected(t)
	if len(expected) != 14 {
		t.Fatalf("%d fixtures in expected.json, want 14", len(expected))
	}
	byID := map[string]string{}
	for _, f := range fixtures() {
		byID[f.id] = f.path
	}
	for path, w := range expected {
		t.Run(path, func(t *testing.T) {
			s := Scan(filepath.Join(fixturesDir, filepath.FromSlash(path)), fixturesDir)
			if s.Path != path {
				t.Errorf("path = %q", s.Path)
			}
			if s.Format != w.Format {
				t.Errorf("format = %q, want %q", s.Format, w.Format)
			}
			for _, c := range []struct{ name, got, want string }{
				{"title", s.Title, w.Title}, {"artist", s.Artist, w.Artist}, {"album", s.Album, w.Album},
				{"title source", s.TitleSource, w.TitleSource}, {"artist source", s.ArtistSource, w.ArtistSource}, {"album source", s.AlbumSource, w.AlbumSource},
			} {
				if c.got != c.want {
					t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
				}
			}
			if w.ErrorContains == "" && s.Error != "" {
				t.Errorf("error = %q", s.Error)
			}
			if w.ErrorContains != "" && !strings.Contains(s.Error, w.ErrorContains) {
				t.Errorf("error = %q, want it to contain %q", s.Error, w.ErrorContains)
			}
			if !slices.EqualFunc(s.Tracks, w.Tracks, func(a, b Track) bool {
				return a.Name == b.Name && a.Instrument == b.Instrument && a.Drums == b.Drums && slices.Equal(a.Pitches, b.Pitches) && a.Tuning == b.Tuning
			}) {
				t.Errorf("tracks = %+v\nwant     %+v", s.Tracks, w.Tracks)
			}
			wantTempos := w.Tempos
			if w.TempoPrefixOf != "" {
				full := expected[byID[w.TempoPrefixOf]].Tempos
				if len(s.Tempos) == 0 || len(s.Tempos) >= len(full) {
					t.Fatalf("tempos = %v, want some but not all of %v", s.Tempos, full)
				}
				wantTempos = full[:len(s.Tempos)]
			}
			if !slices.Equal(s.Tempos, wantTempos) {
				t.Errorf("tempos = %v, want %v", s.Tempos, wantTempos)
			}
		})
	}
}

// U-TAB-04: every fixture cut at 10 points: never a panic, and what was read before
// the cut is kept (the header's title and artist once the cut is past them).
func TestFixturesTruncated(t *testing.T) {
	for _, f := range fixtures() {
		for i := 1; i <= 10; i++ {
			cut := len(f.data) * i / 11
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s cut at %d of %d: panic %v", f.id, cut, len(f.data), r)
					}
				}()
				path := filepath.Join(t.TempDir(), filepath.Base(f.path))
				if err := os.WriteFile(path, f.data[:cut], 0o644); err != nil {
					t.Fatal(err)
				}
				s := Scan(path, filepath.Dir(path))
				if s == nil || s.Path == "" {
					t.Errorf("%s cut at %d: no song", f.id, cut)
				}
			}()
		}
	}
}

// The binary formats that carry notes: a cut anywhere after the track list
// still returns the songs's header and tracks with an error.
func TestTruncatedGPKeepsHeaderAndTracks(t *testing.T) {
	var f3 fixture
	for _, f := range fixtures() {
		if f.id == "F3" {
			f3 = f
		}
	}
	whole, err := parseBytes(f3.data)
	if err != nil {
		t.Fatal(err)
	}
	for i := 5; i <= 10; i++ { // from half of the file on; the track list ends before that
		s, err := parseBytes(f3.data[:len(f3.data)*i/11])
		if err == nil || s == nil || s.Title != whole.Title || len(s.Tracks) != len(whole.Tracks) {
			t.Errorf("cut %d/11: song %+v, error %v", i, s, err)
		}
	}
}

// The library is a tab tree like any other: Walk and ScanAll agree on it.
func TestFixtureLibraryWalk(t *testing.T) {
	var walked []*Song
	if err := Walk(fixturesDir, "", func(s *Song) { walked = append(walked, s) }); err != nil {
		t.Fatal(err)
	}
	if len(walked) != 14 {
		t.Fatalf("Walk found %d tabs, want 14: %v", len(walked), paths(walked))
	}
	all, err := ScanAll(fixturesDir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(paths(all), paths(walked)) {
		t.Errorf("ScanAll = %v\nWalk    = %v", paths(all), paths(walked))
	}
}
