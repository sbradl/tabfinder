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
	Skip:       []string{"RS World Tour", "Rock&Pop", "Chrismas", "Guitar", "Scales", "guitar dvd-school of metal", "Fit for a Lighthouse/Chart Happens"},
	JunkAlbums: []string{"rhythm star.*"},
	AlbumAliases: map[string]string{
		"Dreizehn":                                "13",
		"Fallow/Daybreak Soundtrack":              "Fallow",
		"28 Nights/Weeks Later OST":               "28 Nights Later OST",
		"Runter mit den Pantoffeln, Fremder!":     "Runter mit den Pantoffeln, Unsichtbarer!",
		"Runter mit den Pantoffeln Unsichtbarer!": "Runter mit den Pantoffeln, Unsichtbarer!",
		"Runter mit den Pantofeln Unsichtbarer!":  "Runter mit den Pantoffeln, Unsichtbarer!",
		"Die Eule in Nebelgetalt":                 "Die Eule in Nebelgestalt",
		"Eule in Nebelgestalt":                    "Die Eule in Nebelgestalt",
		"Das ist nicht die ganze Geschichte...":   "Das ist nicht die ganze Geschichte",
		"Ep Micro":                                "Micro",
		"Cobalt Drizle":                           "Cobalt Drizzle",
		"through the dsut of the kingdom":         "Through the Dust of Kingdoms",
		"BEAST":                                   "Beast",
		"live in lisbon":                          "Live in Lisbon",
		"Quartz Sinphony":                         "Quartz Symphony",
		"Helsinki Syndrom":                        "Helsinki Syndrome",
		"Weaver(Teal)":                            "Weaver (Teal Album)",
		"Live aus berlin":                         "Live aus Berlin",
		"Best Of : Still Loving Rain":             "Love at First Spark",
		"Colossus : Delta":                        "Colossus: Delta",
		"Smitten":                                 "Ten Thousand Kites",
	},
}

func init() {
	if err := setConfig(testConfig); err != nil {
		panic(err)
	}
}

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
		defer setConfig(testConfig)
		if err := setConfig(config{AlbumAliases: map[string]string{"Typo": "Fixed"}, JunkAlbums: []string{"x.*", "y"}}); err != nil {
			t.Fatal(err)
		}
		for in, want := range map[string]string{"typo": "Fixed", "XYZ": "", "Y": "", "yy": "yy", "Rhythm Star World Tour": "Rhythm Star World Tour", "Cobalt Drizle": "Cobalt Drizle"} {
			if got := cleanAlbum(in); got != want {
				t.Errorf("cleanAlbum(%q) = %q, want %q", in, got, want)
			}
		}
		if err := setConfig(config{}); err != nil || cleanAlbum("Cobalt Drizle") != "Cobalt Drizle" {
			t.Errorf("empty config: %v", err)
		}
	})
	t.Run("bad pattern", func(t *testing.T) {
		defer setConfig(testConfig)
		if err := setConfig(config{JunkAlbums: []string{"("}}); err == nil || !strings.Contains(err.Error(), "junkAlbums") {
			t.Errorf("err = %v", err)
		}
	})
}
