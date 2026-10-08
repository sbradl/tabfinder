package main

import (
	"errors"
	"slices"
	"testing"
	"time"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/finder"
	"tabfinder/internal/rows"
	"tabfinder/internal/tab"
	"tabfinder/internal/testlib"
)

// The session's tests need no window: it is the app's state without input and drawing.

func TestSessionSearch(t *testing.T) {
	s := newSession("/tabs", time.Now)
	s.setSongs(testlib.Songs())
	s.search()
	all := len(s.result.Matches)
	if all != len(testlib.Songs()) || len(s.songRows) != all {
		t.Fatalf("%d matches, %d rows, want %d", all, len(s.songRows), len(testlib.Songs()))
	}
	s.in.Artist = "soilbed"
	s.search()
	if got := len(s.result.Matches); got == 0 || got == all {
		t.Errorf("%d matches for an artist", got)
	}
	// New songs search again, with the same query.
	s.setSongs([]*tab.Song{{Path: "x.gp5", Artist: "Soilbed Quartet", Title: "X"}})
	s.search()
	if got := len(s.result.Matches); got != 1 {
		t.Errorf("%d matches in the new songs", got)
	}
	s.clearQuery()
	if s.in != (finder.Query{}) {
		t.Errorf("query after clearing = %+v", s.in)
	}
}

func titlesOf(s *session) []string {
	out := []string{}
	for _, i := range s.result.Matches {
		out = append(out, s.lib.Entries[i].Song.Title)
	}
	return out
}

func TestSessionLevels(t *testing.T) {
	s := newSession("/tabs", time.Now)
	s.setSongs(testlib.Songs())
	if lo, hi := s.level(difficulty.Rhythm); lo != 1 || hi != 10 {
		t.Errorf("no level: %d–%d, want 1–10", lo, hi)
	}
	s.setLevel(difficulty.Rhythm, 5, 7)
	s.search()
	if got, want := titlesOf(s), []string{"Where Rivers Seem to Rest", "Embrace the Unseen", "Only for the Brave"}; !slices.Equal(sorted(got), sorted(want)) {
		t.Errorf("rhythm 5–7: %q, want %q", got, want)
	}
	if lo, hi := s.level(difficulty.Rhythm); lo != 5 || hi != 7 {
		t.Errorf("level = %d–%d, want 5–7", lo, hi)
	}
	s.setLevel(difficulty.Drums, 7, 10)
	want := []chip{{difficulty.Drums, "7–10"}, {difficulty.Rhythm, "5–7"}}
	if got := s.chips(); !slices.Equal(got, want) {
		t.Errorf("chips = %+v, want %+v", got, want)
	}
	// The whole range is no filter: songs without the part, or without parts, count too.
	s.setLevel(difficulty.Drums, 1, 10)
	if s.in.Drums != (finder.PartQuery{}) {
		t.Errorf("drums 1–10 = %+v, want no filter", s.in.Drums)
	}
	s.clearLevel(difficulty.Rhythm)
	if s.in.Active() || len(s.chips()) != 0 {
		t.Errorf("after clearing: %+v, chips %+v", s.in, s.chips())
	}
	// A single level.
	s.setLevel(difficulty.Lead, 8, 8)
	s.search()
	if got := titlesOf(s); !slices.Equal(got, []string{"Only for the Brave"}) {
		t.Errorf("lead 8: %q", got)
	}
	if got := s.chips(); !slices.Equal(got, []chip{{difficulty.Lead, "8"}}) {
		t.Errorf("chips = %+v", got)
	}
	// Open ends, as tabscan takes them, show as the ends of the range.
	s.in.Bass.Level = "-3"
	s.in.Lead.Level = "7-"
	if lo, hi := s.level(difficulty.Bass); lo != 1 || hi != 3 {
		t.Errorf("bass -3: %d–%d", lo, hi)
	}
	if lo, hi := s.level(difficulty.Lead); lo != 7 || hi != 10 {
		t.Errorf("lead 7-: %d–%d", lo, hi)
	}
}

func TestSessionSort(t *testing.T) {
	s := newSession("/tabs", time.Now)
	s.setSongs(testlib.Songs())
	s.setSort(finder.SortHardest)
	s.search()
	if got := titlesOf(s); got[0] != "Brass Kettle" {
		t.Errorf("hardest first: %q", got)
	}
	s.setSort(finder.SortEasiest)
	s.search()
	if got := titlesOf(s); got[0] != "Paper Ride" {
		t.Errorf("easiest first: %q", got)
	}
	// Clearing the filters keeps the order.
	s.setLevel(difficulty.Rhythm, 2, 4)
	s.in.Artist = "x"
	s.clearQuery()
	if s.in != (finder.Query{Sort: finder.SortEasiest}) {
		t.Errorf("query after clearing = %+v", s.in)
	}
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

func TestSessionTuning(t *testing.T) {
	s := newSession("/tabs", time.Now)
	s.pickTuning(rows.TuningOf(finder.Tuning{Strings: 7, Name: "B Standard", Notes: "B E A D G B E"}))
	if s.in.Tuning != "B Standard" || s.in.Strings != 7 {
		t.Errorf("after the pick: %+v", s.in)
	}
	s.typeTuning("B Standardx")
	if s.in.Tuning != "B Standardx" || s.in.Strings != 0 {
		t.Errorf("typing keeps the string count: %+v", s.in)
	}
	s.pickTuning(rows.TuningOf(finder.Tuning{Strings: 6, Name: tab.Custom, Notes: "D G D G B D"}))
	if s.in.Tuning != "D G D G B D" {
		t.Errorf("a custom tuning is picked by its notes: %+v", s.in)
	}
}

func TestSessionFolderAndScanStart(t *testing.T) {
	s := newSession("", time.Now)
	if _, ok := s.startScan(); ok || s.scanning {
		t.Error("a scan without a folder")
	}
	s.setSongs(testlib.Songs())
	s.folderChosen("/new/Tabs")
	if s.root != "/new/Tabs" || len(s.lib.Entries) != 0 || len(s.songRows) != 0 {
		t.Errorf("after choosing: root %q, %d songs", s.root, len(s.lib.Entries))
	}
	if root, ok := s.startScan(); !ok || root != "/new/Tabs" || !s.scanning {
		t.Errorf("start: %q, %v, scanning %v", root, ok, s.scanning)
	}
	if _, ok := s.startScan(); ok {
		t.Error("a second scan while one runs")
	}
}

func TestSessionLoadAndScan(t *testing.T) {
	clock := time.Unix(1000, 0)
	s := newSession("/tabs", func() time.Time { return clock })
	if !s.indexLoaded(nil, nil) || !s.loaded {
		t.Error("no saved songs: no scan asked for")
	}
	if newSession("", time.Now).indexLoaded(nil, nil) {
		t.Error("a scan without a folder")
	}
	if s.indexLoaded(testlib.Songs(), errors.New("bad line")) || s.message != "Couldn't read the saved scan: bad line" {
		t.Errorf("saved songs with an error: message %q", s.message)
	}

	s.scanning = true
	s.scanDone(nil, errors.New("no such folder"), time.Second)
	if s.scanning || s.message != "Scan failed: no such folder" || len(s.lib.Entries) != len(testlib.Songs()) {
		t.Errorf("failed scan: scanning %v, message %q, %d songs (the old ones kept)", s.scanning, s.message, len(s.lib.Entries))
	}
	s.scanDone([]*tab.Song{{Path: "a.gp5"}, {Path: "b.gp5", Error: "x"}}, nil, 1500*time.Millisecond)
	if s.message != "2 tabs, 1 unreadable, in 1.5 s" || len(s.lib.Entries) != 2 {
		t.Errorf("scan: message %q, %d songs", s.message, len(s.lib.Entries))
	}

	s.expireMessage(clock.Add(3 * time.Second))
	if s.message == "" {
		t.Error("the message went too early")
	}
	s.expireMessage(clock.Add(4 * time.Second))
	if s.message != "" {
		t.Errorf("the message stays: %q", s.message)
	}
}
