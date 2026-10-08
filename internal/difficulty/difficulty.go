// Package difficulty judges what a song asks of each player of a band: it splits the
// tracks of a tab into parts (drums, bass, rhythm guitar, lead guitar) and tags each
// part with what makes it hard, such as fast playing, odd meters or tapping.
package difficulty

import "tabfinder/internal/score"

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
	played := barsByRole(sc, tracks)
	poly := polyrhythms(sc, tracks)
	var out []Part
	for _, r := range roles {
		if bars := partBars(sc, tracks, played[r], poly); len(bars) > 0 {
			out = append(out, newPart(r, bars, sc))
		}
	}
	return out
}

func newPart(r Role, bars []bar, sc *score.Score) Part {
	p := Part{Role: r, Tags: tags(r, bars, timesPlayed(sc.Bars))}
	for _, b := range bars {
		if len(p.Tracks) == 0 || p.Tracks[len(p.Tracks)-1] != b.track {
			p.Tracks = append(p.Tracks, b.track)
		}
	}
	return p
}

// barsByRole is the bars each role plays: by role, the bar indices of each track. A
// guitar's bars are rhythm or lead, each by what's played in it.
func barsByRole(sc *score.Score, tracks []TrackInfo) map[Role]map[int][]int {
	out := map[Role]map[int][]int{}
	for i, info := range tracks {
		if i >= len(sc.Tracks) {
			break
		}
		t := &sc.Tracks[i]
		r, ok := roleOf(info, t.Drums)
		if !ok {
			continue
		}
		for b, beats := range t.Bars {
			if !struck(beats) {
				continue
			}
			br := r
			if r == Rhythm && isLead(beats, info) {
				br = Lead
			}
			if out[br] == nil {
				out[br] = map[int][]int{}
			}
			out[br][i] = append(out[br][i], b)
		}
	}
	return out
}

// partBars is the bars of a part, given by track, in track order, then bar order.
func partBars(sc *score.Score, tracks []TrackInfo, played map[int][]int, poly []map[int]bool) []bar {
	var out []bar
	for i := range tracks {
		for _, b := range played[i] {
			out = append(out, newBar(sc, tracks[i], i, b, poly[i][b]))
		}
	}
	return out
}
