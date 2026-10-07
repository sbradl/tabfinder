package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/key"

	"tabfinder/internal/finder"
	"tabfinder/internal/tabfiles"
	"tabfinder/internal/testlib"
)

// showcaseLibrary is the made-up library of the README's screenshots: Guitar Pro files under
// <Artist>/<Album>/, in a spread of tunings and tempos. Names are invented, like all test data.
func showcaseLibrary() map[string][]byte {
	var (
		std    = tabfiles.StdGuitar
		dropD  = []int{64, 59, 55, 50, 45, 38}
		dropC  = []int{62, 57, 53, 48, 43, 36}
		dropB  = []int{61, 56, 52, 47, 42, 35}
		eb     = []int{63, 58, 54, 49, 44, 39}
		dStd   = []int{62, 57, 53, 48, 43, 38}
		dadgad = []int{62, 57, 55, 50, 45, 38}
		openG  = []int{62, 59, 55, 50, 43, 38}
		b7     = []int{64, 59, 55, 50, 45, 40, 35}
	)
	gtr := func(name string, strings []int) tabfiles.GPTrack {
		return tabfiles.GPTrack{Name: name, Strings: strings, Program: 30}
	}
	bass := tabfiles.GPTrack{Name: "Bass", Strings: []int{43, 38, 33, 28}, Program: 33}
	bassC := tabfiles.GPTrack{Name: "Bass", Strings: []int{41, 36, 31, 24}, Program: 33}
	drums := tabfiles.GPTrack{Name: "Drums", Drums: true, Strings: make([]int, 6)}
	type song struct {
		artist, album, title string
		tempo                int
		changes              map[int]int // bar -> BPM
		tracks               []tabfiles.GPTrack
	}
	songs := []song{
		{"Amber Marsh", "Tide of Lanterns", "Where Rivers Seem to Rest", 120, nil, []tabfiles.GPTrack{gtr("Guitar", std), bass, drums}},
		{"Amber Marsh", "Tide of Lanterns", "Copper Giant", 160, nil, []tabfiles.GPTrack{gtr("Rhythm", dropD), gtr("Lead", dropD)}},
		{"Amber Marsh", "Tide of Lanterns", "Raise Your Lanterns", 110, nil, []tabfiles.GPTrack{gtr("Guitar", dStd)}},
		{"Inkwell Flamingos", "Mossman", "Only for the Brave", 100, map[int]int{4: 200}, []tabfiles.GPTrack{gtr("Guitar 7", b7)}},
		{"Inkwell Flamingos", "Mossman", "Paper Ride", 120, nil, []tabfiles.GPTrack{gtr("Guitar", dropC), bassC}},
		{"Inkwell Flamingos", "Hive", "Embrace the Unseen", 140, nil, []tabfiles.GPTrack{gtr("Guitar", dropD), bass}},
		{"Soilbed Quartet", "Glass Orchard", "Brass Kettle", 190, map[int]int{5: 145}, []tabfiles.GPTrack{gtr("Guitar", dropC), bassC, drums}},
		{"Soilbed Quartet", "Glass Orchard", "Rust Parade", 190, nil, []tabfiles.GPTrack{gtr("Guitar", dropC)}},
		{"Neon Harbor", "Glass Tides", "Salt Lamp", 132, map[int]int{3: 96, 6: 132}, []tabfiles.GPTrack{gtr("Lead Guitar", std)}},
		{"Neon Harbor", "Glass Tides", "Harbor Lights at Noon", 98, nil, []tabfiles.GPTrack{gtr("Guitar", eb), bass}},
		{"Quiet Engines", "Slow Orbit", "Tin Moon", 90, map[int]int{2: 100}, []tabfiles.GPTrack{gtr("Guitar", dropD), bass, drums}},
		{"Quiet Engines", "Slow Orbit", "Ballast", 76, nil, []tabfiles.GPTrack{gtr("Acoustic", dadgad)}},
		{"Paper Satellites", "Cold Start", "Cold Start", 150, nil, []tabfiles.GPTrack{gtr("Guitar", dStd), drums}},
		{"Paper Satellites", "Cold Start", "Signal Fade", 170, nil, []tabfiles.GPTrack{gtr("Guitar", std)}},
		{"Moss Cathedral", "Hollow Choir", "Lantern", 84, nil, []tabfiles.GPTrack{gtr("Slide", openG)}},
		{"Moss Cathedral", "Hollow Choir", "Under Glass", 66, nil, []tabfiles.GPTrack{gtr("Guitar", dropB), bass}},
		{"Seventh Floor", "Static Garden", "Paper Weather", 140, nil, []tabfiles.GPTrack{gtr("Rhythm 7", b7), gtr("Lead", std)}},
		{"Seventh Floor", "Static Garden", "Cut Short", 125, nil, []tabfiles.GPTrack{gtr("Guitar 7", b7)}},
		{"Die Äther", "Polka ist anders", "Ruf nach Sonne", 150, nil, []tabfiles.GPTrack{gtr("Gitarre", std), bass, drums}},
		{"Orbit Club", "Night Shift", "Night Shift", 128, nil, []tabfiles.GPTrack{gtr("Guitar", std), bass}},
	}
	files := map[string][]byte{}
	for _, s := range songs {
		bars := make([]tabfiles.GPBar, 8)
		for bar, bpm := range s.changes {
			bars[bar].Tempo = bpm
		}
		files[s.artist+"/"+s.album+"/"+s.title+".gp5"] = tabfiles.GP(tabfiles.GPSpec{
			Version: "5.10", Title: s.title, Artist: s.artist, Album: s.album, Tempo: s.tempo, Tracks: s.tracks, Bars: bars,
		})
	}
	return files
}

// TestReadmeScreenshots renders the desktop screenshots of the README into $TABFINDER_README_SHOTS
// (mise run screenshots). With $TABFINDER_README_LIBRARY set, it also writes the library there, for
// the Android screenshots.
func TestReadmeScreenshots(t *testing.T) {
	out := os.Getenv("TABFINDER_README_SHOTS")
	if out == "" {
		t.Skip("set TABFINDER_README_SHOTS to a directory")
	}
	files := showcaseLibrary()
	if lib := os.Getenv("TABFINDER_README_LIBRARY"); lib != "" {
		testlib.WriteFiles(t, lib, files)
	}
	root := filepath.Join(t.TempDir(), "Tabs")
	testlib.WriteFiles(t, root, files)
	index := filepath.Join(t.TempDir(), "index.jsonl")
	if _, err := finder.ScanIndex(root, index); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}

	for _, theme := range []struct{ name, scheme string }{{"dark", "prefer-dark"}, {"light", "default"}} {
		h := newHarnessWith(t, harnessOpts{
			index: str(string(b)), root: &root, size: image.Pt(1000, 760),
			tools: map[string]string{"gsettings": `printf "'` + theme.scheme + `'\n"`},
		})
		h.shots = out
		if theme.name == "dark" {
			h.shot("desktop-list")
			h.click(tuningAt.X, tuningAt.Y)
			h.shot("desktop-tuning")
			continue
		}
		h.click(artistAt.X, artistAt.Y)
		h.typ("inkwell")
		h.press(key.NameReturn)
		h.click(bpmAt.X, bpmAt.Y)
		h.typ("100-150")
		h.press(key.NameEscape)
		h.shot("desktop-light")
	}
}
