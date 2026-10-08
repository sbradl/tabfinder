package finder

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/tab"
)

// Filter holds search criteria; all given criteria must match.
type Filter struct {
	Name, Artist, Tuning string
	// Strings, if not 0, requires the string count on the same track as Tuning.
	Strings int
	BPM     *Range // nil: any tempo
	// Parts is what a song's parts must be like, by role: none for any.
	Parts map[difficulty.Role]PartFilter
}

// PartFilter is what a part of a song must be like.
type PartFilter struct {
	Level *Range   // nil: any level
	Tags  []string // all of them
}

// Range is a range of numbers, tempos or levels, both ends included; Max may be +Inf.
type Range struct{ Min, Max float64 }

func (r Range) Contains(v float64) bool { return v >= r.Min && v <= r.Max }

func (f Filter) Active() bool {
	return f.Name != "" || f.Artist != "" || f.Tuning != "" || f.Strings != 0 || f.BPM != nil || len(f.Parts) > 0
}

func (f Filter) Matches(s *tab.Song) bool {
	if f.Name != "" {
		hay := strings.ToLower(s.Title + " " + tab.BareName(filepath.Base(s.Path))) // not the extension: "gp5" isn't a word of the name
		for _, w := range strings.Fields(strings.ToLower(f.Name)) {
			if !strings.Contains(hay, w) {
				return false
			}
		}
	}
	if f.Artist != "" && !strings.Contains(strings.ToLower(s.Artist), strings.ToLower(f.Artist)) {
		return false
	}
	if (f.Tuning != "" || f.Strings != 0) && !f.hasTrack(s) {
		return false
	}
	if f.BPM != nil && !hasTempo(s, *f.BPM) {
		return false
	}
	for r, pf := range f.Parts {
		if !pf.matches(s, r) {
			return false
		}
	}
	return true
}

// matches reports whether a song's part of a role is like this. A song without the part,
// or not rated (no notes in the file), matches no part filter.
func (pf PartFilter) matches(s *tab.Song, r difficulty.Role) bool {
	p, ok := partOf(s, r)
	if !ok {
		return false
	}
	if pf.Level != nil && !pf.Level.Contains(float64(p.Level())) {
		return false
	}
	for _, tag := range pf.Tags {
		if !slices.Contains(p.Tags, tag) {
			return false
		}
	}
	return true
}

// partOf is a song's part of a role.
func partOf(s *tab.Song, r difficulty.Role) (difficulty.Part, bool) {
	for _, p := range s.Parts {
		if p.Role == r {
			return p, true
		}
	}
	return difficulty.Part{}, false
}

// hasTrack reports whether a track has the wanted tuning and string count.
func (f Filter) hasTrack(s *tab.Song) bool {
	for _, t := range s.Tracks {
		if len(t.Pitches) == 0 || (f.Strings != 0 && len(t.Pitches) != f.Strings) {
			continue
		}
		if f.Tuning == "" || hasTuning(t.Tuning(), f.Tuning) {
			return true
		}
	}
	return false
}

// hasTempo reports whether any tempo the song uses lies in r.
func hasTempo(s *tab.Song, r Range) bool {
	for _, t := range s.Tempos {
		if r.Contains(t.BPM) {
			return true
		}
	}
	return false
}

// ParseBPMRange parses "120", "100-140", "180-" (at least) or "-90" (at most).
func ParseBPMRange(v string) (Range, error) { return parseRange(v, "bpm") }

// ParseLevelRange parses a range of difficulty levels (1 to 10) like a BPM range: "3",
// "2-4", "7-" (at least) or "-3" (at most).
func ParseLevelRange(v string) (Range, error) { return parseRange(v, "level") }

// parseRange parses a range of what: "120", "100-140", "180-" or "-90".
func parseRange(v, what string) (Range, error) {
	v = strings.TrimSpace(v)
	lo, hi, isRange := strings.Cut(v, "-")
	if !isRange {
		lo, hi = v, v
	}
	r := Range{0, math.Inf(1)}
	var err error
	if lo = strings.TrimSpace(lo); lo != "" {
		if r.Min, err = strconv.ParseFloat(lo, 64); err != nil {
			return Range{}, fmt.Errorf("invalid %s %q", what, v)
		}
	}
	if hi = strings.TrimSpace(hi); hi != "" {
		if r.Max, err = strconv.ParseFloat(hi, 64); err != nil {
			return Range{}, fmt.Errorf("invalid %s %q", what, v)
		}
	}
	if math.IsNaN(r.Min) || math.IsNaN(r.Max) || r.Min > r.Max || (lo == "" && hi == "") {
		return Range{}, fmt.Errorf("invalid %s range %q", what, v)
	}
	return r, nil
}

// Enharmonic spellings mapped to the ones tabscan prints.
var enharmonic = map[string]string{"db": "c#", "d#": "eb", "gb": "f#", "g#": "ab", "a#": "bb"}

// tuningKey normalizes a tuning query or label: lower case, enharmonics
// unified per word, spaces removed. "Drop Db" and "drop c#" both give "dropc#".
func tuningKey(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if e, ok := enharmonic[w]; ok {
			words[i] = e
		}
	}
	k := strings.Join(words, "")
	if k == "standard" {
		k = tuningKey("E Standard") // "standard" alone is the standard tuning
	}
	return k
}

// hasTuning reports whether the tuning matches the query by name
// ("Drop C") or by its notes ("C G C F A D").
func hasTuning(t tab.Tuning, query string) bool {
	q := tuningKey(query)
	return q == tuningKey(t.Name) || q == tuningKey(t.Notes())
}
