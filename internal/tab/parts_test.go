package tab

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/tabfiles"
)

func TestScanRatesParts(t *testing.T) {
	sixteenths := func(frets ...int) []tabfiles.GPBeat {
		var out []tabfiles.GPBeat
		for i := range 16 {
			out = append(out, tabfiles.GPBeat{Dur: 2, Notes: []tabfiles.GPNote{{String: 6, Fret: frets[i%len(frets)]}, {String: 5, Fret: frets[i%len(frets)] + 2}}})
		}
		return out
	}
	kickSnare := []tabfiles.GPBeat{
		{Dur: 0, Notes: []tabfiles.GPNote{{String: 6, Fret: 36}}}, {Dur: 0, Notes: []tabfiles.GPNote{{String: 6, Fret: 38}}},
		{Dur: 0, Notes: []tabfiles.GPNote{{String: 6, Fret: 36}}}, {Dur: 0, Notes: []tabfiles.GPNote{{String: 6, Fret: 38}}},
	}
	var bars []tabfiles.GPBar
	for range 32 {
		bars = append(bars, tabfiles.GPBar{Beats: [][]tabfiles.GPBeat{sixteenths(0, 3, 5, 3), kickSnare}})
	}
	root := t.TempDir()
	p := filepath.Join(root, "Copper Wolves", "Rust Belt", "Iron Lung.gp5")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, tabfiles.GP(tabfiles.GPSpec{
		Version: "5.10", Title: "Iron Lung", Artist: "Copper Wolves", Tempo: 170,
		Tracks: []tabfiles.GPTrack{guitar("Rhythm", 30, tabfiles.StdGuitar...), drums},
		Bars:   bars,
	}), 0o644)

	s := Scan(p, root)
	if len(s.Parts) != 2 || s.Parts[0].Role != difficulty.Drums || s.Parts[1].Role != difficulty.Rhythm {
		t.Fatalf("parts %+v, want drums and rhythm guitar", s.Parts)
	}
	if r := s.Parts[1]; r.Score < 5 || !strings.Contains(strings.Join(r.Tags, ","), "fast") {
		t.Errorf("sixteenth power chords at 170: %+v, want a score of 5 or more and fast", r)
	}
	b, _ := json.Marshal(s)
	var back Song
	if err := json.Unmarshal(b, &back); err != nil || len(back.Parts) != 2 || back.Parts[1].Score != s.Parts[1].Score {
		t.Errorf("parts through JSON: %+v, %v", back.Parts, err)
	}
}

func TestScanNoPartsWithoutNotes(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "sand.tg")
	os.WriteFile(p, tabfiles.TG1("Sand Clock", "Copper Wolves", "Dry Wells"), 0o644)
	if s := Scan(p, root); s.Parts != nil {
		t.Errorf("parts %+v of a file without notes", s.Parts)
	}
	if b, _ := json.Marshal(Scan(p, root)); strings.Contains(string(b), "parts") {
		t.Errorf("JSON %s has parts", b)
	}
}
