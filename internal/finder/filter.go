package finder

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"tabfinder/internal/tab"
)

// Filter holds search criteria; all given criteria must match.
type Filter struct {
	Name, Artist, Tuning string
	// Strings, if not 0, requires the string count on the same track as Tuning.
	Strings int
	BPM     *BPMRange // nil: any tempo
}

// BPMRange is a range of tempos, both ends included; Max may be +Inf.
type BPMRange struct{ Min, Max float64 }

func (r BPMRange) Contains(bpm float64) bool { return bpm >= r.Min && bpm <= r.Max }

func (f Filter) Active() bool {
	return f.Name != "" || f.Artist != "" || f.Tuning != "" || f.Strings != 0 || f.BPM != nil
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
	return true
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
func hasTempo(s *tab.Song, r BPMRange) bool {
	for _, t := range s.Tempos {
		if r.Contains(t.BPM) {
			return true
		}
	}
	return false
}

// ParseBPMRange parses "120", "100-140", "180-" (at least) or "-90" (at most).
func ParseBPMRange(v string) (BPMRange, error) {
	v = strings.TrimSpace(v)
	lo, hi, isRange := strings.Cut(v, "-")
	if !isRange {
		lo, hi = v, v
	}
	r := BPMRange{0, math.Inf(1)}
	var err error
	if lo = strings.TrimSpace(lo); lo != "" {
		if r.Min, err = strconv.ParseFloat(lo, 64); err != nil {
			return BPMRange{}, fmt.Errorf("invalid bpm %q", v)
		}
	}
	if hi = strings.TrimSpace(hi); hi != "" {
		if r.Max, err = strconv.ParseFloat(hi, 64); err != nil {
			return BPMRange{}, fmt.Errorf("invalid bpm %q", v)
		}
	}
	if math.IsNaN(r.Min) || math.IsNaN(r.Max) || r.Min > r.Max || (lo == "" && hi == "") {
		return BPMRange{}, fmt.Errorf("invalid bpm range %q", v)
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
