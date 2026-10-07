package main

import (
	"path/filepath"
	"slices"
	"testing"

	"tabfinder/internal/tab"
	"tabfinder/internal/testlib"
)

// appLibrary is the library of the Android app's tests (Fixture in android/app/src/sharedTest): seven
// readable tabs, one of them under a case variant of an artist, and an unreadable file.
func appLibrary() []*tab.Song {
	song := func(path, artist, album, title string, bpm float64, tracks ...tab.Track) *tab.Song {
		return &tab.Song{Path: path, Format: "gp3", Artist: artist, Album: album, Title: title, Tracks: tracks, Tempos: []tab.Tempo{{Bar: 1, BPM: bpm}}}
	}
	broken := &tab.Song{Path: "Broken/garbled.gp5", Artist: "Broken", Title: "Garbled", Error: "unknown file format"}
	return []*tab.Song{
		song("Soilbed Quartet/Glass Orchard/Brass Kettle.gp3", "Soilbed Quartet", "Glass Orchard", "Brass Kettle", 190, testlib.DropC6, testlib.Bass4),
		song("Amber Marsh/Tide of Lanterns/First Frost.gp3", "Amber Marsh", "Tide of Lanterns", "First Frost", 120, testlib.EStd6),
		song("Amber Marsh/Tide of Lanterns/Where Rivers Seem to Rest.gp3", "Amber Marsh", "Tide of Lanterns", "Where Rivers Seem to Rest", 125, testlib.Custom6),
		song("Inkwell Flamingos/Mossman/Mirage.gp3", "Inkwell Flamingos", "Mossman", "Mirage", 140, testlib.DropC6),
		song("INKWELL FLAMINGOS/Mossman/Paper Ride.gp3", "INKWELL FLAMINGOS", "Mossman", "Paper Ride", 100, testlib.BStd7),
		song("Merrowgate/Catch Fortyone/Compass Lost.gp3", "Merrowgate", "Catch Fortyone", "Compass Lost", 110, testlib.BStd7),
		song("Gorsewick/Gravel Hymns/Quartz.gpx.crdownload", "Gorsewick", "Gravel Hymns", "Quartz", 125, testlib.EStd6),
		broken,
	}
}

// TestServeAppFilters is what the app's filter fields ask for (E-AND-04): each field alone, all of them
// together, an invalid tempo range, and a query nothing matches.
func TestServeAppFilters(t *testing.T) {
	index := filepath.Join(t.TempDir(), "index.jsonl")
	testlib.WriteIndex(t, index, appLibrary())
	load := req(map[string]any{"op": "load", "index": index})
	for _, c := range []struct {
		name       string
		query      map[string]any
		want       []string // titles of the matches, in list order
		bpmInvalid bool
	}{
		{"artist, any case", map[string]any{"artist": "inkwell flamingos"}, []string{"Mirage", "Paper Ride"}, false},
		{"song", map[string]any{"name": "quartz"}, []string{"Quartz"}, false},
		{"tuning by name", map[string]any{"tuning": "drop c"}, []string{"Mirage", "Brass Kettle"}, false},
		{"tempo range", map[string]any{"bpm": "100-110"}, []string{"Paper Ride", "Compass Lost"}, false},
		{"all fields", map[string]any{"artist": "inkwell flamingos", "tuning": "drop c", "bpm": "100-150"}, []string{"Mirage"}, false},
		{"all fields but the tuning", map[string]any{"artist": "inkwell flamingos", "bpm": "100-150"}, []string{"Mirage", "Paper Ride"}, false},
		{"an invalid tempo range filters nothing", map[string]any{"bpm": "fast"}, nil, true},
		{"nothing matches", map[string]any{"name": "zzzzz"}, []string{}, false},
		{"a picked tuning with its string count", map[string]any{"tuning": "B Standard", "strings": 7}, []string{"Paper Ride", "Compass Lost"}, false},
		{"a picked tuning typed on drops the string count", map[string]any{"tuning": "B Standardx"}, []string{}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := session(t, load, req(map[string]any{"op": "search", "query": c.query}))
			titles := titlesOf(t, r[0], r[1])
			if c.want == nil { // all songs
				c.want = titlesOf(t, r[0], map[string]any{"matches": pathsOf(r[0])})
			}
			if !slices.Equal(titles, c.want) {
				t.Errorf("matches = %q, want %q", titles, c.want)
			}
			if r[1]["bpmInvalid"] != c.bpmInvalid {
				t.Errorf("bpmInvalid = %v", r[1]["bpmInvalid"])
			}
		})
	}
}

// TestServeAppSuggestions is what the app's suggestion menus show (E-AND-05).
func TestServeAppSuggestions(t *testing.T) {
	index := filepath.Join(t.TempDir(), "index.jsonl")
	testlib.WriteIndex(t, index, appLibrary())
	search := func(q map[string]any) map[string]any {
		return session(t, req(map[string]any{"op": "load", "index": index}), req(map[string]any{"op": "search", "query": q}))[1]
	}

	t.Run("an artist is suggested once, whatever its spellings", func(t *testing.T) {
		var inkwell int
		for _, a := range list(t, search(map[string]any{"artist": "i"}), "artists") {
			if a == "Inkwell Flamingos" || a == "INKWELL FLAMINGOS" {
				inkwell++
			}
		}
		if inkwell != 1 {
			t.Errorf("Inkwell Flamingos suggested %d times", inkwell)
		}
	})
	t.Run("a picked artist's songs decide the tunings", func(t *testing.T) {
		// Amber Marsh plays 6 strings only.
		for _, tu := range list(t, search(map[string]any{"artist": "Amber Marsh", "tuning": "e"}), "tunings") {
			if n := tu.(map[string]any)["strings"]; n != 6.0 {
				t.Errorf("tuning %v for Amber Marsh", tu)
			}
		}
	})
	t.Run("a tuning is suggested with its string count", func(t *testing.T) {
		found := false
		for _, tu := range list(t, search(map[string]any{"tuning": "b"}), "tunings") {
			m := tu.(map[string]any)
			found = found || m["label"] == "B Standard" && m["strings"] == 7.0
		}
		if !found {
			t.Error("no 7 string B Standard")
		}
	})
}

// titlesOf maps the matches of search to the titles in the load answer.
func titlesOf(t *testing.T, load, search map[string]any) []string {
	t.Helper()
	title := map[any]string{}
	for _, s := range list(t, load, "songs") {
		title[s.(map[string]any)["path"]] = s.(map[string]any)["title"].(string)
	}
	out := []string{}
	for _, p := range list(t, search, "matches") {
		out = append(out, title[p])
	}
	return out
}

func pathsOf(load map[string]any) []any {
	var out []any
	for _, s := range load["songs"].([]any) {
		out = append(out, s.(map[string]any)["path"])
	}
	return out
}
