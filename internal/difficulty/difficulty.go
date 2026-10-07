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
	// What each role plays: the bars of each track, by track index.
	played := map[Role]map[int][]int{}
	play := func(r Role, track, bar int) {
		if played[r] == nil {
			played[r] = map[int][]int{}
		}
		played[r][track] = append(played[r][track], bar)
	}
	for i, info := range tracks {
		if i >= len(sc.Tracks) {
			break
		}
		t := &sc.Tracks[i]
		var r Role
		switch {
		case info.Drums || t.Drums:
			r = Drums
		case isBass(info):
			r = Bass
		case isGuitar(info):
			r = Rhythm // or lead, by bar
		default:
			continue
		}
		for b, beats := range t.Bars {
			if !struck(beats) {
				continue
			}
			if r == Rhythm && isLead(beats, info) {
				play(Lead, i, b)
			} else {
				play(r, i, b)
			}
		}
	}
	var out []Part
	for _, r := range roles {
		if played[r] == nil {
			continue
		}
		p := Part{Role: r}
		var bars []bar
		for i := range tracks {
			if len(played[r][i]) > 0 {
				p.Tracks = append(p.Tracks, i)
			}
			for _, b := range played[r][i] {
				bars = append(bars, bar{i, b, sc.Tracks[i].Bars[b]})
			}
		}
		p.Tags = tags(bars)
		out = append(out, p)
	}
	return out
}

// bar is a bar a part is played in on one of its tracks.
type bar struct {
	track, index int
	beats        []score.Beat
}

// tags is what makes a part with these bars hard.
func tags(bars []bar) []string {
	var out []string
	if often(bars, func(b bar) bool { return hasTuplet(b.beats, isTriplet) }) {
		out = append(out, "triplets")
	}
	if often(bars, func(b bar) bool { return hasTuplet(b.beats, isOddTuplet) }) {
		out = append(out, "tuplets")
	}
	return out
}

// isTriplet reports whether n:m tuplets are triplets: 3, or 6 or 12 played as triplets of triplets.
func isTriplet(n int) bool { return n == 3 || n == 6 || n == 12 || n == 24 }

// isOddTuplet reports whether n:m tuplets are any other: 5, 7, 9...
func isOddTuplet(n int) bool { return n > 1 && !isTriplet(n) }

// often reports whether bars with something come up in a part more than once or twice:
// in at least two bars and every eighth.
func often(bars []bar, has func(bar) bool) bool {
	n := 0
	for _, b := range bars {
		if has(b) {
			n++
		}
	}
	return n >= 2 && 8*n >= len(bars)
}

// hasTuplet reports whether a beat struck is an n:m tuplet of a kind.
func hasTuplet(beats []score.Beat, kind func(n int) bool) bool {
	for _, b := range beats {
		if b.Struck() && kind(b.Tuplet) {
			return true
		}
	}
	return false
}

// struck reports whether any of the beats strikes a note.
func struck(beats []score.Beat) bool {
	for _, b := range beats {
		if b.Struck() {
			return true
		}
	}
	return false
}

var (
	reLeadName   = regexp.MustCompile(`(?i)lead|solo`)
	reRhythmName = regexp.MustCompile(`(?i)rhy`)
)

// isLead reports whether a guitar's bar is a lead line rather than rhythm playing:
// mostly single notes, not palm-muted, up high or bent and slid. Riffs are chords or low
// single notes.
func isLead(beats []score.Beat, t TrackInfo) bool {
	var struckBeats, single, notes, muted, pitchSum int
	expressive := false
	for _, b := range beats {
		if !b.Struck() {
			continue
		}
		struckBeats++
		if len(b.Notes) == 1 {
			single++
		}
		for _, n := range b.Notes {
			notes++
			if n.Fx&score.PalmMute != 0 {
				muted++
			}
			if n.Fx&(score.Bend|score.Vibrato|score.Slide|score.Tap|score.Legato) != 0 {
				expressive = true
			}
			pitchSum += pitch(t, n)
		}
	}
	if struckBeats == 0 || 10*single < 6*struckBeats || 2*muted >= notes {
		return false
	}
	low, high := 50, 57 // D3, A3: an expressive line above the first, any line above the second
	switch {
	case reLeadName.MatchString(t.Name):
		low, high = 45, 50
	case reRhythmName.MatchString(t.Name):
		low, high = 57, 62
	}
	mean := pitchSum / notes
	return mean >= high || expressive && mean >= low
}

// pitch is a note's MIDI pitch, guessing a standard-tuned guitar if the tuning is unknown.
func pitch(t TrackInfo, n score.Note) int {
	if n.String >= 0 && n.String < len(t.Pitches) {
		return t.Pitches[n.String] + n.Fret
	}
	std := []int{40, 45, 50, 55, 59, 64}
	return std[min(max(n.String, 0), len(std)-1)] + n.Fret
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
