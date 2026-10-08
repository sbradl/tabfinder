package main

import (
	"fmt"
	"time"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/finder"
	"tabfinder/internal/rows"
	"tabfinder/internal/tab"
)

// session is the desktop app's state and what can be done with it, apart from input and
// drawing: the folder and its songs, the query as typed and its result, the message shown.
// The ui owns one and turns clicks and keys into its calls; tests use it on its own.
type session struct {
	now      func() time.Time
	root     string
	lib      *finder.Library
	songRows []rows.Song // what the list shows of each of lib's entries
	loaded   bool        // the cached index has been read
	scanning bool
	message  string
	msgUntil time.Time

	in finder.Query
	// The search for in over lib, redone when either changes.
	result    finder.Result
	resultFor *finder.Library
	resultIn  finder.Query
}

func newSession(root string, now func() time.Time) *session {
	return &session{now: now, root: root, lib: finder.New(nil)}
}

func (s *session) setSongs(songs []*tab.Song) {
	s.lib = finder.New(songs)
	s.songRows = rows.All(s.lib)
}

// show puts up a message for a few seconds.
func (s *session) show(msg string) { s.message, s.msgUntil = msg, s.now().Add(4*time.Second) }

// expireMessage takes the message down once its time is up.
func (s *session) expireMessage(t time.Time) {
	if s.message != "" && !t.Before(s.msgUntil) {
		s.message = ""
	}
}

// loaded records the songs of the saved scan, and reports whether to scan the folder since there were none.
func (s *session) indexLoaded(songs []*tab.Song, err error) (scan bool) {
	s.loaded = true
	if err != nil {
		s.show("Couldn't read the saved scan: " + err.Error())
	}
	s.setSongs(songs)
	return len(songs) == 0 && s.root != ""
}

// startScan marks a scan of the folder as running and returns the folder; ok is false if
// there's no folder or a scan is running already.
func (s *session) startScan() (root string, ok bool) {
	if s.scanning || s.root == "" {
		return "", false
	}
	s.scanning = true
	return s.root, true
}

// folderChosen makes dir the folder: the songs of the old one are gone, and a scan of the
// new one is due.
func (s *session) folderChosen(dir string) {
	s.root = dir
	s.setSongs(nil)
}

// scanDone records the outcome of a scan that took took.
func (s *session) scanDone(songs []*tab.Song, err error, took time.Duration) {
	s.scanning = false
	if songs == nil && err != nil {
		s.show("Scan failed: " + err.Error())
		return
	}
	s.setSongs(songs)
	s.show(fmt.Sprintf("%s, in %.1f s", rows.ScanSummary(songs), took.Seconds()))
}

// search brings result up to date with the query and the songs.
func (s *session) search() {
	if s.resultFor != s.lib || s.resultIn != s.in {
		s.result, s.resultFor, s.resultIn = s.lib.Search(s.in), s.lib, s.in
	}
}

func (s *session) artistSuggestions() []string { return s.lib.SuggestArtists(s.in.Artist) }

// tuningSuggestions are the tunings of the songs the other fields leave, matching what's typed.
func (s *session) tuningSuggestions() []rows.Tuning {
	return rows.Tunings(finder.SuggestTunings(s.result.Tunings, s.in.Tuning))
}

// typeTuning sets the typed tuning; typing drops the string count a picked suggestion set.
func (s *session) typeTuning(text string) { s.in.Tuning, s.in.Strings = text, 0 }

func (s *session) pickTuning(t rows.Tuning) { s.in.Tuning, s.in.Strings = t.Label, t.Strings }

// clearQuery drops the filters; the order stays.
func (s *session) clearQuery() { s.in = finder.Query{Sort: s.in.Sort} }

func (s *session) setSort(o finder.Sort) { s.in.Sort = o }

// part is the query of a role's part, to change.
func (s *session) part(r difficulty.Role) *finder.PartQuery {
	switch r {
	case difficulty.Drums:
		return &s.in.Drums
	case difficulty.Bass:
		return &s.in.Bass
	case difficulty.Rhythm:
		return &s.in.Rhythm
	}
	return &s.in.Lead
}

// level is the range of levels a role's part must be in: 1 to 10 for any.
func (s *session) level(r difficulty.Role) (lo, hi int) {
	rg, err := finder.ParseLevelRange(s.part(r).Level)
	if err != nil {
		return 1, 10
	}
	return max(int(rg.Min), 1), int(min(rg.Max, 10))
}

// setLevel sets the range of levels of a role's part; 1 to 10 is any, no filter.
func (s *session) setLevel(r difficulty.Role, lo, hi int) {
	if lo <= 1 && hi >= 10 {
		s.clearLevel(r)
		return
	}
	s.part(r).Level = fmt.Sprintf("%d-%d", lo, hi)
}

func (s *session) clearLevel(r difficulty.Role) { s.part(r).Level = "" }

// chip is a role's level filter in the summary under the fields.
type chip struct {
	role difficulty.Role
	text string
}

func (s *session) chips() []chip {
	var out []chip
	for _, r := range difficulty.Roles {
		if lo, hi := s.level(r); lo > 1 || hi < 10 {
			out = append(out, chip{r, rows.LevelChip(lo, hi)})
		}
	}
	return out
}
