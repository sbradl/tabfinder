package main

import (
	"math"
	"testing"

	"tabfinder/internal/tab"
)

func TestRatioMatchesDifflib(t *testing.T) {
	// Expected values from Python's difflib.SequenceMatcher(None, a, b).ratio().
	tests := []struct {
		a, b string
		want float64
	}{
		{"abcd", "bcde", 0.75},
		{"prayforvillains", "prayforvillians", 28.0 / 30},
		{"", "", 1},
		{"abc", "xyz", 0},
	}
	for _, tt := range tests {
		if got := ratio(tt.a, tt.b); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("ratio(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCleanAlbum(t *testing.T) {
	tests := map[string]string{
		"Dawn of the Copper Giant (2008)":       "Dawn of the Copper Giant",
		"1978 - Who Are They":                   "Who Are They",
		"Cobalt Drizle":                         "Cobalt Drizzle",
		"(Single - 2021)":                       "",
		"Fallow/Daybreak Soundtrack":            "Fallow",
		"Das ist nicht die ganze Geschichte...": "Das ist nicht die ganze Geschichte",
	}
	for in, want := range tests {
		if got := cleanAlbum(in); got != want {
			t.Errorf("cleanAlbum(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNicest(t *testing.T) {
	if got := nicest([]string{"Pray For Hereos", "Pray For Heroes", "Pray For Heroes"}); got != "Pray For Heroes" {
		t.Errorf("nicest = %q", got)
	}
	if got := nicest([]string{"quiet season", "Quiet Season"}); got != "Quiet Season" {
		t.Errorf("nicest = %q", got)
	}
}

func TestSongName(t *testing.T) {
	tests := []struct {
		path, artist, title, titleSource, want string
	}{
		{"Argyle Moth/Lantern Machine/nectar_ver3_E.gp5", "Argyle Moth", "Nectar", "file", "Nectar"},
		{"Eisenmond/Eisenmond - Heimatland.gp5", "Heimatland", "Eisenmond", "file", "Heimatland"},
		{"Megalith Dusk/Moss in Peace/Megalith Dusk - Hangar 22 (2).gp3", "Megalith Dusk", "someone@example.com  Hanger 22", "file", "Hangar 22"},
		{"Deepstones/Around the Moss/My_own_summer.gp4", "Deepstones", "My Own Summer", "file", "My Own Summer"},
		{"Inkwell Flamingos/The Harlequin Race/Inkwell Flamingos - The Harlequins Dance (ver 3 by X).gp5", "Inkwell Flamingos", "Inkwell Flamingos - The Harlequin's Dance", "file", "The Harlequin's Dance"},
		{"As Lanterns Fade/Echoes are quiet/ripple_ver2_DropD.gp3", "As Lanterns Fade", "Ripple", "path", "Ripple"},
		{"Kern/Take a look in the window/yall_want_a.gp4", "Kern", "Ya'll Want A Ticket", "file", "Ya'll Want A Ticket"},
		{"Dissolved/Ten Thousand Kites/smitten.ptb", "Dissolved", "Smitten", "path", "Smitten"},
		{"Sodbury Lane/Agent Marmalade/indigo.gp4", "Sodbury Lane", "Indigo [by drummer@example.com]", "file", "Indigo"},
		{"Argyle Moth/Will to Wander/Argyle Moth - The Heron Flies Alone (ver 2 by tabber42).gpx", "Argyle Moth", "The Heron Flies Alone''", "file", "The Heron Flies Alone"},
		{"Cousins of Marrow/Marrow Covers/Cousins Of Marrow - No Orders.gp4", "Cousins of Marrow", "no orders (cover)", "file", "No Orders (cover)"},
	}
	for _, tt := range tests {
		s := &tab.Song{Path: tt.path, Artist: tt.artist, Title: tt.title, TitleSource: tt.titleSource}
		if got := songName(s); got != tt.want {
			t.Errorf("songName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestTabExt(t *testing.T) {
	for in, want := range map[string]string{"a.gp5": ".gp5", "Sodbury Lane - Umgezogen.gp3.zip": ".gp3.zip", "x.zip": ".zip"} {
		if got := tabExt(in); got != want {
			t.Errorf("tabExt(%q) = %q, want %q", in, got, want)
		}
	}
}
