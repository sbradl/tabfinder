package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// testConfig is the configuration of the tests: made-up names in the places where a real library
// has its own folders, album spellings and junk albums (see config.go). The in-process tests run
// with it (init); the end-to-end tests give the built command a config file made from it (TestMain).
var testConfig = config{
	Skip:         []string{"RS World Tour", "Rock&Pop", "Chrismas", "Guitar", "Scales", "guitar dvd-school of metal", "Fit for a Lighthouse/Chart Happens"},
	JunkAlbums:   []string{"rhythm star.*", "unbekannt"},
	JunkArtists:  []string{"niemand"},
	FileSuffixes: []string{"withbass", "zweite? stimme"},
	AlbumAliases: map[string]string{
		"Siebzehn":                          "17",
		"Fallow/Daybreak Soundtrack":        "Fallow",
		"40 Winters/Summers Later OST":      "40 Winters Later OST",
		"Hoch mit den Gardinen, Fremder!":   "Hoch mit den Gardinen, Nachbar!",
		"Hoch mit den Gardinen Nachbar!":    "Hoch mit den Gardinen, Nachbar!",
		"Hoch mit den Gardinnen Nachbar!":   "Hoch mit den Gardinen, Nachbar!",
		"Die Eule in Nebelgetalt":           "Die Eule in Nebelgestalt",
		"Eule in Nebelgestalt":              "Die Eule in Nebelgestalt",
		"Das ist nicht das ganze Rezept...": "Das ist nicht das ganze Rezept",
		"Ep Micro":                          "Micro",
		"Cobalt Drizle":                     "Cobalt Drizzle",
		"through the dsut of the kingdom":   "Through the Dust of Kingdoms",
		"BEAST":                             "Beast",
		"live in lisbon":                    "Live in Lisbon",
		"Quartz Sinphony":                   "Quartz Symphony",
		"Helsinki Syndrom":                  "Helsinki Syndrome",
		"Ferret(Mauve)":                     "Ferret (Mauve Album)",
		"Live aus berlin":                   "Live aus Berlin",
		"Best Of : Still Waiting Snow":      "Hope at First Frost",
		"Colossus : Delta":                  "Colossus: Delta",
		"Smitten":                           "Ten Thousand Kites",
	},
}

// testRules are the album rules of testConfig.
var testRules = func() nameRules {
	r, err := newNameRules(testConfig)
	if err != nil {
		panic(err)
	}
	return r
}()

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := dir + "/" + name
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	t.Run("missing default file is no config", func(t *testing.T) {
		c, err := loadConfig(dir+"/nope.json", false)
		if err != nil || len(c.Skip) != 0 || len(c.AlbumAliases) != 0 || len(c.JunkAlbums) != 0 {
			t.Errorf("= %+v, %v", c, err)
		}
	})
	t.Run("missing file asked for is an error", func(t *testing.T) {
		if _, err := loadConfig(dir+"/nope.json", true); err == nil {
			t.Error("no error")
		}
	})
	t.Run("not json", func(t *testing.T) {
		if _, err := loadConfig(write("bad.json", "{"), false); err == nil || !strings.Contains(err.Error(), "bad.json") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("fields", func(t *testing.T) {
		c, err := loadConfig(write("ok.json", `{"skip":["A","B/C"],"albumAliases":{"Typo":"Fixed"},"junkAlbums":["x.*"],"somethingNew":1}`), true)
		if err != nil || !slices.Equal(c.Skip, []string{"A", "B/C"}) || c.AlbumAliases["Typo"] != "Fixed" || !slices.Equal(c.JunkAlbums, []string{"x.*"}) {
			t.Errorf("= %+v, %v", c, err)
		}
	})
	t.Run("rules follow the config", func(t *testing.T) {
		r, err := newNameRules(config{AlbumAliases: map[string]string{"Typo": "Fixed"}, JunkAlbums: []string{"x.*", "y"}})
		if err != nil {
			t.Fatal(err)
		}
		for in, want := range map[string]string{"typo": "Fixed", "XYZ": "", "Y": "", "yy": "yy", "Rhythm Star World Tour": "Rhythm Star World Tour", "Cobalt Drizle": "Cobalt Drizle"} {
			if got := r.cleanAlbum(in); got != want {
				t.Errorf("cleanAlbum(%q) = %q, want %q", in, got, want)
			}
		}
		if r, err := newNameRules(config{}); err != nil || r.cleanAlbum("Cobalt Drizle") != "Cobalt Drizle" {
			t.Errorf("empty config: %v", err)
		}
	})
	t.Run("junk artists and file suffixes", func(t *testing.T) {
		r, err := newNameRules(config{JunkArtists: []string{"niemand"}, FileSuffixes: []string{"withbass"}})
		if err != nil {
			t.Fatal(err)
		}
		builtIn, _ := newNameRules(config{})
		for _, tt := range []struct {
			r            nameRules
			artist, file string
			junk         bool
			trimmed      string
		}{
			{r, "Niemand", "song_withbass", true, "song"},
			{r, "Unknown", "song drop c withbass", true, "song"}, // underscores are spaces by now (tab.BareName)
			{builtIn, "Niemand", "song_withbass", false, "song_withbass"},
			{builtIn, "Various", "song 7string", true, "song"},
		} {
			if got := tt.r.junkArtistName(tt.artist); got != tt.junk {
				t.Errorf("junkArtistName(%q) = %v", tt.artist, got)
			}
			if got := tt.r.dropSuffixes(tt.file); got != tt.trimmed {
				t.Errorf("dropSuffixes(%q) = %q, want %q", tt.file, got, tt.trimmed)
			}
		}
	})
	t.Run("bad pattern", func(t *testing.T) {
		for field, c := range map[string]config{"junkAlbums": {JunkAlbums: []string{"("}}, "junkArtists": {JunkArtists: []string{"["}}, "fileSuffixes": {FileSuffixes: []string{"(?"}}} {
			if _, err := newNameRules(c); err == nil || !strings.Contains(err.Error(), field) {
				t.Errorf("%s: err = %v", field, err)
			}
		}
	})
}
