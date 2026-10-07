package testlib

import (
	"os"
	"path/filepath"
	"testing"

	"tab-sync/internal/tabfiles"
)

// TreeFiles is a small, messy tab library as path -> content, for tabscan and
// tabreorg: files in <Artist>/<Album>/, loose in an artist folder, at the root,
// misspelled albums, an artist filed under another, a name clash, a byte-identical
// copy, folders tabreorg skips, a file that can't be parsed and non-tab files.
// The comment on each file says what it exercises.
func TreeFiles() map[string][]byte {
	g := func(title, artist, album string, tempo int, tracks ...tabfiles.GP3Track) []byte {
		return tabfiles.GP3(tabfiles.GP3Spec{Title: title, Artist: artist, Album: album, Tempo: tempo, Tracks: tracks})
	}
	guitar := func(strings ...int) tabfiles.GP3Track { return tabfiles.GP3Track{Name: "Guitar", Strings: strings} }
	std := guitar(tabfiles.StdGuitar...)
	dropD := guitar(64, 59, 55, 50, 45, 38)
	dropC := guitar(62, 57, 53, 48, 43, 36)
	custom := guitar(62, 59, 55, 50, 43, 38) // D G D G B D
	seven := guitar(64, 59, 55, 50, 45, 40, 35)
	bass := tabfiles.GP3Track{Name: "Bass", Strings: []int{43, 38, 33, 28}}
	drums := tabfiles.GP3Track{Name: "Drums", Drums: true, Strings: []int{0, 0, 0, 0, 0, 0}}

	overload := tabfiles.GP7(tabfiles.GPIF("Brass Kettle", "Soilbed Quartet", "Glass Orchard", [][2]float64{{0, 190}, {90, 145}},
		tabfiles.GPIFTrack{Name: "Lead", Instrument: "Electric Guitar", Kind: "electricGuitar", Pitches: "36 43 48 53 57 62"}))
	quartz := tabfiles.GP7(tabfiles.GPIF("Quartz", "Gorsewick", "Gravel Hymns", [][2]float64{{0, 125}},
		tabfiles.GPIFTrack{Name: "Guitar", Instrument: "Guitar", Pitches: "38 45 50 55 59 64"}))

	return map[string][]byte{
		// In place: nothing to do.
		"Amber Marsh/Tide of Lanterns/Where Rivers Seem to Rest.gp3": g("Where Rivers Seem to Rest", "Amber Marsh", "Tide of Lanterns", 120, std),
		"Argyle Moth/Rent of Summer/Nectar.gp3":                      g("Nectar", "Argyle Moth", "Rent of Summer", 130, std, bass, drums),
		"Die Äther/Polka ist anders/Ruf nach Sonne.gp3":              g("Ruf nach Sonne", "Die Äther", "Polka ist anders", 150, std),
		"Felix Ferien/Endlich Ferien/Klug.gp3":                       g("Klug", "Felix Ferien", "Endlich Ferien", 140, custom),
		"Gorsewick/Gravel Hymns/Quartz.gpx.crdownload":               quartz,
		"Soilbed Quartet/Glass Orchard/Brass Kettle.gp":              overload,
		"Metro Kettle/Tan Album/Enter Daydream (ver 1).gp3":          g("Enter Daydream", "Metro Kettle", "Tan Album", 123, dropD),
		"Metro Kettle/Tan Album/Enter Daydream (ver 2).gp3":          g("Enter Daydream", "Metro Kettle", "Tan Album", 124, seven),

		// Loose in an artist folder, album named in the file: moves into the album folder.
		"Amber Marsh/amber_marsh_dawn_of_the_copper_giant.gp3": g("Dawn of the Copper Giant", "Amber Marsh", "Dawn of the Copper Giant (2008)", 160, dropD),
		// No metadata: stays, renamed after the file name.
		"Argyle Moth/argyle_moth_gluttonous.gp3": g("", "", "", 100, dropC),
		// Filed under the wrong artist: moves to the artist folder that exists.
		"Die Äther/Felix Ferien - Weniger.gp3": g("Weniger", "Felix Ferien", "Endlich Ferien", 150, std),
		// At the root: goes to its artist and album.
		"Soilbed Quartet - Rust Parade.gp3": g("Rust Parade", "Soilbed Quartet", "Glass Orchard", 190, dropC),
		// A byte-identical copy in the artist folder: moves next to the original as "Brass Kettle (2).gp".
		"Soilbed Quartet/Brass Kettle.gp": overload,
		// Misspelled albums with no folder: merged into one.
		"Cousins of Marrow/Tidebloom.gp3":     g("Tidebloom", "Cousins of Marrow", "Moonbreeder", 200, dropD),
		"Cousins of Marrow/Quiet Morning.gp3": g("Quiet Morning", "Cousins of Marrow", "Moonbreder", 190, dropD),
		// A zip around a tab, renamed after its song.
		"Sodbury Lane/Agent Marmalade/sodbury lane_umgezogen.gp3.zip": tabfiles.Zip(map[string][]byte{"a.tg": tabfiles.TG1("Umgezogen", "Sodbury Lane", "Agent Marmalade")}),
		// No metadata at the root: stays.
		"mystery_tab.gp3": g("", "", "", 90, std),
		// Folders tabreorg leaves alone by default.
		"Scales/major_scale.gp3":                   g("Major Scale", "", "", 60, std),
		"Fit for a Lighthouse/Chart Happens/x.gp3": g("X", "Fit for a Lighthouse", "", 80, std),
		// Can't be parsed.
		"Broken/garbled.gp5": []byte("this is not a tab"),
		// Not tabs: never touched, never listed.
		"Amber Marsh/cover.jpg": []byte("jpg"),
		"notes.txt":             []byte("hello"),
	}
}

// Tree writes TreeFiles into <temp dir>/Tabs and returns that root.
func Tree(t testing.TB) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Tabs")
	WriteFiles(t, root, TreeFiles())
	return root
}

// WriteFiles writes files (path relative to root -> content), creating folders.
func WriteFiles(t testing.TB, root string, files map[string][]byte) {
	t.Helper()
	for rel, data := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
