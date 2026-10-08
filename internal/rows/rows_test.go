package rows

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/finder"
	"tabfinder/internal/tab"
	"tabfinder/internal/testlib"
)

func TestSong(t *testing.T) {
	lib := finder.New(testlib.Songs())
	by := map[string]Song{}
	for _, r := range All(lib) {
		by[r.Title] = r
	}
	for title, want := range map[string]string{
		"Brass Kettle": "Soilbed Quartet · Glass Orchard",
		"Eight String": "Merrowgate", // no album: no dot
		"Nine String":  "Nine",
		"Broken Song":  "Broken",
	} {
		if got := by[title].Subtitle; got != want {
			t.Errorf("%s: subtitle %q, want %q", title, got, want)
		}
	}
	if got := subtitle(&tab.Song{Artist: "  ", Album: "Album"}); got != "Album" {
		t.Errorf("blank artist: %q", got)
	}
	if got := subtitle(&tab.Song{}); got != "" {
		t.Errorf("nothing: %q", got)
	}
	if !by["Broken Song"].Unreadable {
		t.Error("the unparseable file isn't shown as unreadable")
	}
	for _, title := range []string{"Brass Kettle", "Eight String"} {
		if by[title].Unreadable {
			t.Errorf("%s is shown as unreadable", title)
		}
	}
	// An error with something read before it is not "couldn't read", tunings or not.
	partial := finder.New([]*tab.Song{
		{Path: "a.gp5", Format: "gp5", Title: "A", Error: "tempo changes incomplete", Tracks: []tab.Track{testlib.EStd6}},
		{Path: "b.gp5", Format: "gp5", Title: "B", Error: "tempo changes incomplete", Tracks: []tab.Track{testlib.Drums}},
	})
	for _, r := range All(partial) {
		if r.Unreadable {
			t.Errorf("%s, read partly, is shown as unreadable", r.Title)
		}
	}
	// Tuning tags: one per distinct tuning, bigger string counts first.
	var tags []string
	for _, tu := range by["Brass Kettle"].Tunings {
		tags = append(tags, fmt.Sprintf("%d %s", tu.Strings, tu.Label))
	}
	if !slices.Equal(tags, []string{"6 Drop C", "4 Drop C"}) {
		t.Errorf("tags = %q", tags)
	}
	if r := by["Brass Kettle"]; !slices.Equal(r.BPMs, []string{"190", "145"}) || r.Tempo != "190" || r.TempoDetail != "→ 145" || r.OpenAs != "Brass Kettle.gp5" {
		t.Errorf("row = %+v", r)
	}
}

func TestBPMs(t *testing.T) {
	r := Of(finder.Entry{Song: &tab.Song{Tempos: []tab.Tempo{{Bar: 1, BPM: 190}, {Bar: 2, BPM: 120.5}, {Bar: 3, BPM: 190}, {Bar: 4, BPM: 120.5}, {Bar: 5, BPM: 100}, {Bar: 6, BPM: 120.25}}}})
	if want := []string{"190", "120.5", "100", "120.25"}; !slices.Equal(r.BPMs, want) {
		t.Errorf("bpms = %v, want %v", r.BPMs, want)
	}
	if r := Of(finder.Entry{Song: &tab.Song{}}); r.BPMs == nil || len(r.BPMs) != 0 {
		t.Errorf("no tempos: %#v, want empty, not nil (JSON [])", r.BPMs)
	}
}

func TestTempo(t *testing.T) {
	for _, tt := range []struct {
		bpms      []string
		main, sub string
	}{
		{nil, "", ""},
		{[]string{"190"}, "190", "BPM"},
		{[]string{"190", "145"}, "190", "→ 145"},
		{[]string{"190", "145", "100"}, "190", "→ 145 100"},
		{[]string{"190", "145", "100", "80"}, "190", "→ 145 100 …"},
		{[]string{"120.5"}, "120.5", "BPM"},
	} {
		if m, s := tempo(tt.bpms); m != tt.main || s != tt.sub {
			t.Errorf("tempo(%v) = %q, %q; want %q, %q", tt.bpms, m, s, tt.main, tt.sub)
		}
	}
}

func TestTuningOf(t *testing.T) {
	if r := TuningOf(finder.Tuning{Strings: 6, Name: "Drop C", Notes: "C G C F A D"}); r.Label != "Drop C" || r.Detail != "C G C F A D" {
		t.Errorf("drop C = %+v", r)
	}
	if r := TuningOf(finder.Tuning{Strings: 6, Name: tab.Custom, Notes: "D G D G B D"}); r.Label != "D G D G B D" || r.Detail != "" {
		t.Errorf("custom = %+v", r)
	}
	if got := Section(7); got != "7 STRINGS" {
		t.Errorf("section = %q", got)
	}
}

func TestTexts(t *testing.T) {
	for _, tt := range []struct {
		matches, total int
		want           string
	}{{0, 11, " 0 / 11"}, {11, 11, "11 / 11"}, {4, 4, "4 / 4"}, {7, 1000, "   7 / 1000"}} {
		if got := Counter(tt.matches, tt.total); got != tt.want {
			t.Errorf("Counter(%d, %d) = %q, want %q", tt.matches, tt.total, got, tt.want)
		}
	}
	if got := LibraryLine(20, "/home/x/Guitar/Tabs/"); got != "20 TABS IN TABS" {
		t.Errorf("library line = %q", got)
	}
	songs := []*tab.Song{{}, {Error: "x"}, {Error: "y"}}
	if got := ScanSummary(songs); got != "3 tabs, 2 unreadable" {
		t.Errorf("summary = %q", got)
	}
}

func TestParts(t *testing.T) {
	r := Of(finder.Entry{Song: &tab.Song{Parts: []difficulty.Part{
		{Role: difficulty.Drums, Score: 2.4},
		{Role: difficulty.Rhythm, Score: 6.5, Tags: []string{"chords", "odd meter"}},
		{Role: difficulty.Lead, Score: 9.96, Tracks: []int{1, 2}},
	}}})
	want := []Part{
		{Role: "drums", Level: 2, Tags: []string{}},
		{Role: "rhythm", Level: 7, Tags: []string{"chords", "odd meter"}},
		{Role: "lead", Level: 10, Tags: []string{}},
	}
	if !reflect.DeepEqual(r.Parts, want) {
		t.Errorf("parts = %+v, want %+v", r.Parts, want)
	}
	if r := Of(finder.Entry{Song: &tab.Song{}}); r.Parts == nil || len(r.Parts) != 0 {
		t.Errorf("no parts: %#v, want empty, not nil (JSON [])", r.Parts)
	}
	// The shared fixture has songs with parts.
	for _, r := range All(finder.New(testlib.Songs())) {
		if r.Title == "Brass Kettle" && len(r.Parts) != 4 {
			t.Errorf("Brass Kettle: parts = %+v, want all four", r.Parts)
		}
	}
}

func TestLevelChip(t *testing.T) {
	for _, tt := range []struct {
		lo, hi int
		want   string
	}{{1, 3, "1–3"}, {7, 10, "7–10"}, {5, 5, "5"}, {1, 10, "1–10"}} {
		if got := LevelChip(tt.lo, tt.hi); got != tt.want {
			t.Errorf("LevelChip(%d, %d) = %q, want %q", tt.lo, tt.hi, got, tt.want)
		}
	}
}
