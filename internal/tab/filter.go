package tab

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

// Filter holds search criteria; all given criteria must match.
type Filter struct {
	Name, Artist, Tuning string
	// Strings, if not 0, requires the string count on the same track as Tuning.
	Strings        int
	BPMSet         bool
	BPMMin, BPMMax float64
}

func (f Filter) Active() bool {
	return f.Name != "" || f.Artist != "" || f.Tuning != "" || f.Strings != 0 || f.BPMSet
}

func (f Filter) Matches(s *Song) bool {
	if f.Name != "" {
		hay := strings.ToLower(s.Title + " " + filepath.Base(s.Path))
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
	if f.BPMSet && !hasTempo(s, f.BPMMin, f.BPMMax) {
		return false
	}
	return true
}

// hasTrack reports whether a track has the wanted tuning and string count.
func (f Filter) hasTrack(s *Song) bool {
	for _, t := range s.Tracks {
		if t.Tuning == "" || (f.Strings != 0 && len(t.Pitches) != f.Strings) {
			continue
		}
		if f.Tuning == "" || hasTuning(t, f.Tuning) {
			return true
		}
	}
	return false
}

// hasTempo reports whether any tempo the song uses lies in [min, max].
func hasTempo(s *Song, min, max float64) bool {
	for _, t := range s.Tempos {
		if t.BPM >= min && t.BPM <= max {
			return true
		}
	}
	return false
}

// ParseBPMRange parses "120", "100-140", "180-" (at least) or "-90" (at most).
func ParseBPMRange(v string) (min, max float64, err error) {
	v = strings.TrimSpace(v)
	lo, hi, isRange := strings.Cut(v, "-")
	if !isRange {
		lo, hi = v, v
	}
	min, max = 0, math.Inf(1)
	if lo = strings.TrimSpace(lo); lo != "" {
		if min, err = strconv.ParseFloat(lo, 64); err != nil {
			return 0, 0, fmt.Errorf("invalid bpm %q", v)
		}
	}
	if hi = strings.TrimSpace(hi); hi != "" {
		if max, err = strconv.ParseFloat(hi, 64); err != nil {
			return 0, 0, fmt.Errorf("invalid bpm %q", v)
		}
	}
	if math.IsNaN(min) || math.IsNaN(max) || min > max || (lo == "" && hi == "") {
		return 0, 0, fmt.Errorf("invalid bpm range %q", v)
	}
	return min, max, nil
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
		k = "estandard"
	}
	return k
}

// hasTuning reports whether the track matches the query by tuning name
// ("Drop C") or by its notes ("C G C F A D").
func hasTuning(t Track, query string) bool {
	q := tuningKey(query)
	label, notes, _ := strings.Cut(t.Tuning, " (")
	return q == tuningKey(label) || q == tuningKey(strings.TrimSuffix(notes, ")"))
}
