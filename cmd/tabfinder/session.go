package main

import (
	"fmt"
	"time"

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

func (s *session) pickTuning(t finder.Tuning) { s.in.Tuning, s.in.Strings = t.Label(), t.Strings }

func (s *session) clearQuery() { s.in = finder.Query{} }
