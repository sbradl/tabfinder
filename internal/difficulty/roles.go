package difficulty

import (
	"regexp"
	"strings"

	"tabfinder/internal/score"
)

var (
	reBassName   = regexp.MustCompile(`(?i)bass`)
	reGuitarName = regexp.MustCompile(`(?i)guit|gtr|git|lead|solo|rhy`)
	reLeadName   = regexp.MustCompile(`(?i)lead|solo`)
	reRhythmName = regexp.MustCompile(`(?i)rhy`)
)

// roleOf is the role a track plays: for a guitar Rhythm, though bars of it may be lead.
// ok is false for tracks of no role: keys, vocals, strings...
func roleOf(t TrackInfo, drums bool) (r Role, ok bool) {
	switch {
	case t.Drums || drums:
		return Drums, true
	case isBass(t):
		return Bass, true
	case isGuitar(t):
		return Rhythm, true
	}
	return "", false
}

func isBass(t TrackInfo) bool {
	if reBassName.MatchString(t.Instrument) || reBassName.MatchString(t.Name) {
		return true
	}
	n := len(t.Pitches)
	return (n == 4 || n == 5) && t.Pitches[0] < 40 // below a guitar's low E
}

// isGuitar reports whether a track is a guitar: named so, with six strings or more, the
// lowest no higher than an E3 (a guitar tuned up a lot; a violin's G3 is too high).
func isGuitar(t TrackInfo) bool {
	if len(t.Pitches) < 6 || t.Pitches[0] > 52 {
		return false // sung, or no guitar's strings
	}
	return strings.Contains(t.Instrument, "Guitar") || reGuitarName.MatchString(t.Name)
}

// isLead reports whether a guitar's bar is a lead line rather than rhythm playing:
// mostly single notes, not palm-muted, up high or bent and slid. Riffs are chords or low
// single notes.
func isLead(beats []score.Beat, t TrackInfo) bool {
	l := lineOf(beats, t)
	if l.beats == 0 || 10*l.single < 6*l.beats || 2*l.muted >= l.notes {
		return false
	}
	low, high := 50, 57 // D3, A3: an expressive line above the first, any line above the second
	switch {
	case reLeadName.MatchString(t.Name):
		low, high = 45, 50
	case reRhythmName.MatchString(t.Name):
		low, high = 57, 62
	}
	mean := l.pitchSum / l.notes
	return mean >= high || l.expressive && mean >= low
}

// line is what a guitar plays in a bar, as far as telling lead from rhythm needs it.
type line struct {
	beats, single int // beats struck, and those of a single note
	notes, muted  int // notes struck, and those palm-muted
	pitchSum      int
	expressive    bool // bent, slid, ... like a lead line
}

func lineOf(beats []score.Beat, t TrackInfo) line {
	var l line
	for _, b := range beats {
		if !b.Struck() {
			continue
		}
		l.beats++
		if len(b.Notes) == 1 {
			l.single++
		}
		for _, n := range b.Notes {
			l.notes++
			if n.Fx&score.PalmMute != 0 {
				l.muted++
			}
			if n.Fx&(score.Bend|score.Vibrato|score.Slide|score.Tap|score.Legato) != 0 {
				l.expressive = true
			}
			l.pitchSum += pitch(t, n)
		}
	}
	return l
}

// pitch is a note's MIDI pitch, guessing a standard-tuned guitar if the tuning is unknown.
func pitch(t TrackInfo, n score.Note) int {
	if n.String >= 0 && n.String < len(t.Pitches) {
		return t.Pitches[n.String] + n.Fret
	}
	std := []int{40, 45, 50, 55, 59, 64}
	return std[min(max(n.String, 0), len(std)-1)] + n.Fret
}
