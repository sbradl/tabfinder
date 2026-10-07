// Package testlib holds the synthetic library the tests of several packages share:
// songs as Go structs, written as an index file (`tabscan -json` lines).
package testlib

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tabfinder/internal/tab"
)

// Tracks for common tunings, lowest string first.
var (
	EStd6   = tab.Track{Name: "Guitar", Instrument: "Distortion Guitar", Pitches: []int{40, 45, 50, 55, 59, 64}}
	DropD6  = tab.Track{Name: "Guitar", Pitches: []int{38, 45, 50, 55, 59, 64}}
	DropC6  = tab.Track{Name: "Guitar", Pitches: []int{36, 43, 48, 53, 57, 62}}
	BStd7   = tab.Track{Name: "Guitar 7", Pitches: []int{35, 40, 45, 50, 55, 59, 64}}
	Eight   = tab.Track{Name: "Guitar 8", Pitches: []int{30, 35, 40, 45, 50, 55, 59, 64}}
	Nine    = tab.Track{Name: "Guitar 9", Pitches: []int{25, 30, 35, 40, 45, 50, 55, 59, 64}}
	Bass4   = tab.Track{Name: "Bass", Instrument: "Electric Bass (finger)", Pitches: []int{28, 33, 38, 43}}
	DropC4  = tab.Track{Name: "Bass", Pitches: []int{24, 31, 36, 41}}
	Bass5   = tab.Track{Name: "Bass 5", Pitches: []int{23, 28, 33, 38, 43}}
	Custom6 = tab.Track{Name: "Guitar", Pitches: []int{38, 43, 50, 55, 59, 62}}
	Drums   = tab.Track{Name: "Drums", Instrument: "Drums", Drums: true}
)

func song(path string, format tab.Format, artist, album, title string, tracks []tab.Track, tempos ...float64) *tab.Song {
	s := &tab.Song{Path: path, Format: format, Artist: artist, Album: album, Title: title, Tracks: tracks,
		ArtistSource: tab.FromFile, AlbumSource: tab.FromFile, TitleSource: tab.FromFile}
	for i, bpm := range tempos {
		s.Tempos = append(s.Tempos, tab.Tempo{Bar: 1 + 16*i, BPM: bpm})
	}
	return s
}

// LongTitle is 200 characters, to test truncation and wrapping.
var LongTitle = strings.Repeat("Very long song title ", 10)[:200]

// Songs is the shared fixture: artist case variants ("Inkwell Flamingos" and "INKWELL FLAMINGOS"),
// a custom tuning, 4 to 9 string tracks, drums, tempo changes, an unreadable
// entry, non-ASCII names and a long title.
func Songs() []*tab.Song {
	unreadable := song("Broken/Broken Song.gp5", "", "Broken", "", "Broken Song", nil)
	unreadable.Error = "unknown file format"
	unreadable.ArtistSource, unreadable.AlbumSource, unreadable.TitleSource = "path", "path", "path"
	return []*tab.Song{
		song("Soilbed Quartet/Glass Orchard/Brass Kettle.gp5", "gp5", "Soilbed Quartet", "Glass Orchard", "Brass Kettle", []tab.Track{DropC6, DropC4, Drums}, 190, 145, 190),
		song("Amber Marsh/Tide of Lanterns/Where Rivers Seem to Rest.gp5", "gp5", "Amber Marsh", "Tide of Lanterns", "Where Rivers Seem to Rest", []tab.Track{EStd6, Custom6}, 120.5),
		song("Inkwell Flamingos/Hive/Embrace the Unseen.gp4", "gp4", "Inkwell Flamingos", "Hive", "Embrace the Unseen", []tab.Track{DropD6, Bass4}, 140),
		song("Inkwell Flamingos/Mossman/Only for the Brave.gp5", "gp5", "Inkwell Flamingos", "Mossman", "Only for the Brave", []tab.Track{BStd7}, 100, 200),
		song("INKWELL FLAMINGOS/Mossman/Paper Ride.gp5", "gp5", "INKWELL FLAMINGOS", "Mossman", "Paper Ride", []tab.Track{DropC6}, 120),
		song("Die Äther/Polka ist anders/Ruf nach Sonne.gp4", "gp4", "Die Äther", "Polka ist anders", "Ruf nach Sonne", []tab.Track{EStd6, Bass4, Drums}, 150),
		song("Merrowgate/Eight/Eight String.gp7", "gp7", "Merrowgate", "", "Eight String", []tab.Track{Eight}, 110),
		song("Nine/Nine String.gpx", "gp6", "Nine", "", "Nine String", []tab.Track{Nine}),
		song("Bassists/Five.tg", "tg", "Bassists", "", "Five Strings", []tab.Track{Bass5}, 90),
		song("Long/Long.gp5", "gp5", "Long Titles", "", LongTitle, []tab.Track{EStd6}, 100),
		unreadable,
	}
}

// WriteIndex writes songs as an index file, the way tabscan -json (and finder.ScanIndex) does.
func WriteIndex(t testing.TB, path string, songs []*tab.Song) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, s := range songs {
		if err := enc.Encode(s); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
