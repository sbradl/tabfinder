// Package finder is the search behind the TabFinder apps: the song list,
// filtering by the fields the user types, and the suggestions for the artist
// and tuning fields. The desktop app uses it directly; the Android app talks
// to it through `tabscan -serve`.
package finder

import (
	"cmp"
	"slices"
	"strings"

	"tabfinder/internal/tab"
)

// Tuning is a tuning as searched and suggested: "Drop C" on a 6-string, with its notes "C G C F A D".
type Tuning struct {
	Strings int    `json:"strings"`
	Name    string `json:"name"`
	Notes   string `json:"notes"`
}

// Label is what the tuning is called, in suggestions and in a query: the name, or the notes
// for a custom tuning, which the name says nothing about.
func (t Tuning) Label() string {
	if t.Name == tab.Custom && t.Notes != "" {
		return t.Notes
	}
	return t.Name
}

// Entry is a song with the tunings it is searched and suggested by.
type Entry struct {
	Song    *tab.Song
	Tunings []Tuning // distinct, most strings first (guitars before bass)
}

func newEntry(s *tab.Song) Entry {
	e := Entry{Song: s}
	for _, t := range s.Tracks {
		if len(t.Pitches) == 0 {
			continue
		}
		tt := t.Tuning()
		tu := Tuning{tt.Strings(), tt.Name, tt.Notes()}
		if !slices.Contains(e.Tunings, tu) {
			e.Tunings = append(e.Tunings, tu)
		}
	}
	slices.SortStableFunc(e.Tunings, func(a, b Tuning) int { return b.Strings - a.Strings })
	return e
}

// Library is the songs sorted by artist, then title.
type Library struct {
	Entries []Entry
	Artists []string // each artist once, A-Z
}

func New(songs []*tab.Song) *Library {
	l := &Library{Entries: make([]Entry, len(songs))}
	for i, s := range songs {
		l.Entries[i] = newEntry(s)
	}
	slices.SortStableFunc(l.Entries, func(a, b Entry) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Song.Artist), strings.ToLower(b.Song.Artist)),
			cmp.Compare(strings.ToLower(a.Song.Title), strings.ToLower(b.Song.Title)),
		)
	})
	l.Artists = artistsOf(l.Entries)
	return l
}

// Query is the filter fields as typed.
type Query struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Tuning string `json:"tuning"`
	BPM    string `json:"bpm"`
	// Strings is set by picking a tuning suggestion and dropped when the tuning is typed.
	Strings int `json:"strings"`
}

func (q Query) Active() bool {
	return q.Name != "" || q.Artist != "" || q.Tuning != "" || q.Strings != 0 || q.BPM != ""
}

// Filter turns the fields into criteria; an unparsable BPM is left out and reported.
func (q Query) Filter() (f Filter, bpmInvalid bool) {
	f = Filter{
		Name:    strings.TrimSpace(q.Name),
		Artist:  strings.TrimSpace(q.Artist),
		Tuning:  strings.TrimSpace(q.Tuning),
		Strings: q.Strings,
	}
	if strings.TrimSpace(q.BPM) != "" {
		r, err := ParseBPMRange(q.BPM)
		if err == nil {
			f.BPM = &r
		}
		bpmInvalid = err != nil
	}
	return f, bpmInvalid
}

// Result of a search.
type Result struct {
	Matches    []int    `json:"matches"` // indices into Entries
	BPMInvalid bool     `json:"bpmInvalid"`
	Tunings    []Tuning `json:"tunings"` // for suggestions, see Search
}

// Search returns the matching songs, and the tunings of the songs the other
// fields leave (so picking an artist trims them to that artist's), grouped
// by string count: guitars 6, 7, 8..., then bass 4, 5, most common first.
func (l *Library) Search(q Query) Result {
	f, invalid := q.Filter()
	other := f
	other.Tuning, other.Strings = "", 0
	return Result{Matches: l.matching(f), BPMInvalid: invalid, Tunings: l.tuningsOf(l.matching(other))}
}

func (l *Library) matching(f Filter) []int {
	idx := []int{}
	for i := range l.Entries {
		if f.Matches(l.Entries[i].Song) {
			idx = append(idx, i)
		}
	}
	return idx
}

// artistsOf lists each artist once ignoring case ("Inkwell Flamingos", "INKWELL FLAMINGOS"),
// in its most common spelling, A-Z.
func artistsOf(es []Entry) []string {
	spellings := map[string]map[string]int{}
	for _, e := range es {
		a := strings.TrimSpace(e.Song.Artist)
		if a == "" {
			continue
		}
		k := strings.ToLower(a)
		if spellings[k] == nil {
			spellings[k] = map[string]int{}
		}
		spellings[k][a]++
	}
	out := []string{}
	for _, names := range spellings {
		best, n := "", -1
		for name, c := range names {
			if c > n || (c == n && name < best) {
				best, n = name, c
			}
		}
		out = append(out, best)
	}
	slices.SortFunc(out, func(a, b string) int { return cmp.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return out
}

func (l *Library) tuningsOf(idx []int) []Tuning {
	count := map[Tuning]int{}
	for _, i := range idx {
		for _, t := range l.Entries[i].Tunings {
			count[t]++
		}
	}
	// Guitars (6 strings and up) come first, in string order, then bass (4 and 5).
	const maxBassStrings = 5
	group := func(stringCount int) int {
		if stringCount <= maxBassStrings {
			return 1000 + stringCount // after any guitar
		}
		return stringCount
	}
	out := make([]Tuning, 0, len(count))
	for t := range count {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b Tuning) int {
		return cmp.Or(cmp.Compare(group(a.Strings), group(b.Strings)), count[b]-count[a], cmp.Compare(a.Label(), b.Label()))
	})
	return out
}

// MaxSuggestions caps the suggestion lists.
const MaxSuggestions = 60

// SuggestArtists returns the artists containing what's typed, those starting
// with it first, leaving out an exact match (already picked).
func (l *Library) SuggestArtists(typed string) []string {
	q := strings.ToLower(strings.TrimSpace(typed))
	prefix, rest := []string{}, []string{}
	for _, a := range l.Artists {
		switch low := strings.ToLower(a); {
		case q != "" && low == q:
		case strings.HasPrefix(low, q):
			prefix = append(prefix, a)
		case strings.Contains(low, q):
			rest = append(rest, a)
		}
	}
	return limit(append(prefix, rest...), MaxSuggestions)
}

// SuggestTunings returns the tunings whose label or notes contain what's
// typed, keeping their grouping.
func SuggestTunings(tunings []Tuning, typed string) []Tuning {
	q := strings.ToLower(strings.TrimSpace(typed))
	out := []Tuning{}
	for _, t := range tunings {
		if strings.Contains(strings.ToLower(t.Label()+" "+t.Notes), q) {
			out = append(out, t)
		}
	}
	return limit(out, MaxSuggestions)
}

func limit[T any](s []T, n int) []T { return s[:min(len(s), n)] }
