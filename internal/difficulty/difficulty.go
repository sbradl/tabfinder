// Package difficulty judges what a song asks of each player of a band: it splits the
// tracks of a tab into parts (drums, bass, rhythm guitar, lead guitar) and tags each
// part with what makes it hard, such as fast playing, odd meters or tapping.
package difficulty

import (
	"regexp"
	"strings"

	"tabfinder/internal/score"
)

// Role is what a band member plays.
type Role string

const (
	Drums  Role = "drums"
	Bass   Role = "bass"
	Rhythm Role = "rhythm" // rhythm guitar
	Lead   Role = "lead"   // lead guitar
)

var roles = []Role{Drums, Bass, Rhythm, Lead}

// TrackInfo is what the roles of a track are judged by, besides its notes.
type TrackInfo struct {
	Name, Instrument string // the track's name, and its General MIDI instrument or the like
	Drums            bool
	Pitches          []int // the tuning, lowest string first
}

// Part is what one role plays in a song.
type Part struct {
	Role   Role     `json:"role"`
	Tracks []int    `json:"tracks"` // the tracks it is played on, by index
	Tags   []string `json:"tags,omitempty"`
}

// Analyze splits a song into parts, in the order drums, bass, rhythm, lead, leaving out
// those nobody plays. tracks are the score's tracks, in the same order.
func Analyze(sc *score.Score, tracks []TrackInfo) []Part {
	byRole := map[Role]*Part{}
	for i, info := range tracks {
		if i >= len(sc.Tracks) || !plays(&sc.Tracks[i]) {
			continue
		}
		var r Role
		switch {
		case info.Drums || sc.Tracks[i].Drums:
			r = Drums
		case isBass(info):
			r = Bass
		case isGuitar(info):
			r = Rhythm
		default:
			continue
		}
		p := byRole[r]
		if p == nil {
			p = &Part{Role: r}
			byRole[r] = p
		}
		p.Tracks = append(p.Tracks, i)
	}
	var out []Part
	for _, r := range roles {
		if p := byRole[r]; p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// plays reports whether a track strikes any note.
func plays(t *score.Track) bool {
	for _, bar := range t.Bars {
		for _, b := range bar {
			if b.Struck() {
				return true
			}
		}
	}
	return false
}

var (
	reBassName   = regexp.MustCompile(`(?i)bass`)
	reGuitarName = regexp.MustCompile(`(?i)guit|gtr|git|lead|solo|rhy`)
)

func isBass(t TrackInfo) bool {
	if reBassName.MatchString(t.Instrument) || reBassName.MatchString(t.Name) {
		return true
	}
	n := len(t.Pitches)
	return (n == 4 || n == 5) && t.Pitches[0] < 40 // below a guitar's low E
}

func isGuitar(t TrackInfo) bool {
	return strings.Contains(t.Instrument, "Guitar") || reGuitarName.MatchString(t.Name)
}
