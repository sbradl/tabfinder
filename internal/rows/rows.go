// Package rows is what the TabFinder apps show of a library: a row per song and the
// texts around the list. The desktop app uses it directly; the Android app gets the
// same rows from tabscan -serve, so both show the same.
package rows

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"tabfinder/internal/finder"
	"tabfinder/internal/tab"
	"tabfinder/internal/tuxguitar"
)

// Tuning is a tuning tag or suggestion: "Drop C", with the notes as its detail.
type Tuning struct {
	finder.Tuning
	Label  string `json:"label"`
	Detail string `json:"detail"` // the notes, unless they are the label already
}

func TuningOf(t finder.Tuning) Tuning {
	r := Tuning{Tuning: t, Label: t.Label()}
	if r.Label != t.Notes {
		r.Detail = t.Notes
	}
	return r
}

func Tunings(ts []finder.Tuning) []Tuning {
	out := make([]Tuning, len(ts))
	for i, t := range ts {
		out[i] = TuningOf(t)
	}
	return out
}

// Section is the header over the suggested tunings with this many strings.
func Section(strings int) string { return fmt.Sprintf("%d STRINGS", strings) }

// Song is a song's row in the list.
type Song struct {
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Album       string   `json:"album"`
	Subtitle    string   `json:"subtitle"` // "artist · album", leaving out what's missing
	Tunings     []Tuning `json:"tunings"`
	BPMs        []string `json:"bpms"`        // distinct, in order of first appearance
	Tempo       string   `json:"tempo"`       // the opening tempo, shown large; "" for none
	TempoDetail string   `json:"tempoDetail"` // under it: "BPM", or the tempo changes that follow
	Unreadable  bool     `json:"unreadable"`  // parsing failed and there's nothing to show
	OpenAs      string   `json:"openAs"`      // the file name to hand TuxGuitar a copy under
}

func Of(e finder.Entry) Song {
	s := e.Song
	r := Song{
		Path: s.Path, Title: s.Title, Artist: s.Artist, Album: s.Album,
		Subtitle:   subtitle(s),
		Tunings:    Tunings(e.Tunings),
		BPMs:       []string{},
		Unreadable: s.Error != "" && len(e.Tunings) == 0,
		OpenAs:     tuxguitar.FileName(s),
	}
	for _, bpm := range s.DistinctBPMs() {
		r.BPMs = append(r.BPMs, strconv.FormatFloat(bpm, 'f', -1, 64))
	}
	r.Tempo, r.TempoDetail = tempo(r.BPMs)
	return r
}

// All is the rows of the library's songs, in its order.
func All(lib *finder.Library) []Song {
	out := make([]Song, len(lib.Entries))
	for i, e := range lib.Entries {
		out[i] = Of(e)
	}
	return out
}

func subtitle(s *tab.Song) string {
	var parts []string
	for _, p := range []string{s.Artist, s.Album} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

// tempo is the opening tempo and the line under it: "BPM", or the tempo changes that
// follow (at most two, then an ellipsis).
func tempo(bpms []string) (main, detail string) {
	if len(bpms) == 0 {
		return "", ""
	}
	detail = "BPM"
	if len(bpms) > 1 {
		detail = "→ " + strings.Join(bpms[1:min(len(bpms), 3)], " ")
		if len(bpms) > 3 {
			detail += " …"
		}
	}
	return bpms[0], detail
}

// Counter is "matches / total", padded to the total's width so it never changes size.
func Counter(matches, total int) string {
	t := strconv.Itoa(total)
	return fmt.Sprintf("%*d / %s", len(t), matches, t)
}

// LibraryLine is the line under the app's title: how many tabs, in which folder.
func LibraryLine(total int, root string) string {
	return strings.ToUpper(fmt.Sprintf("%d tabs in %s", total, filepath.Base(filepath.Clean(root))))
}

// Unreadable counts the songs whose files couldn't be parsed, or only partly.
func Unreadable(songs []*tab.Song) int {
	n := 0
	for _, s := range songs {
		if s.Error != "" {
			n++
		}
	}
	return n
}

// ScanSummary is the message after a scan: "948 tabs, 3 unreadable".
func ScanSummary(songs []*tab.Song) string {
	return fmt.Sprintf("%d tabs, %d unreadable", len(songs), Unreadable(songs))
}
