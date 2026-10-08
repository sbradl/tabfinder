package difficulty

import (
	"fmt"
	"math/bits"
	"slices"
	"strings"

	"tabfinder/internal/score"
)

// manyBars is the number of different bars from which a part has much to learn: eight
// sections of four bars, all different. In a library of mostly metal songs about a
// quarter of the guitar parts have that many.
const manyBars = 32

// repetitive reports whether a part of so many different bars, played so often, plays the
// same over and over: a part of 16 bars or more, each different bar six times.
func repetitive(distinct, played int) bool { return played >= 16 && 6*distinct <= played }

// material is how much there is to learn of a part: the number of different bars on the
// track that has the most, and how many bars that track plays, repeats included (plays
// is how often each bar of the song is played).
func material(bars []bar, plays []int) (distinct, played int) {
	byTrack := map[int][]bar{}
	for _, b := range bars {
		byTrack[b.track] = append(byTrack[b.track], b)
	}
	for _, tbars := range byTrack {
		d, n := differentBars(tbars), 0
		for _, b := range tbars {
			if b.index < len(plays) {
				n += plays[b.index]
			} else {
				n++
			}
		}
		if d > distinct || d == distinct && n > played {
			distinct, played = d, n
		}
	}
	return distinct, played
}

// differentBars is the number of different bars of a track. A bar played again higher up
// the neck, or with a note or two changed, is no new bar; nor is a drum fill: a known
// groove for the first half of the bar.
func differentBars(bars []bar) int {
	var kinds [][]onset
	for _, b := range bars {
		o := onsets(b)
		same := func(k []onset) bool { return alike(k, o) }
		if b.info.Drums {
			half := b.ticks() / 2
			same = func(k []onset) bool { return alike(before(k, half), before(o, half)) }
		}
		if !slices.ContainsFunc(kinds, same) {
			kinds = append(kinds, o)
		}
	}
	return len(kinds)
}

// onset is notes struck together: when, and what relative to the bar's first note
// (for drums: which drums).
type onset struct {
	start int
	notes string
}

func onsets(b bar) []onset {
	var out []onset
	ref := -1
	for _, beat := range b.beats {
		if !beat.Struck() {
			continue
		}
		ps := beatPitches(b.info, beat)
		if ref < 0 && !b.info.Drums {
			ref = ps[0]
		}
		var sb strings.Builder
		for _, p := range ps {
			fmt.Fprintf(&sb, "%d ", p-max(ref, 0))
		}
		if len(out) > 0 && out[len(out)-1].start == beat.Start { // another voice
			out[len(out)-1].notes += sb.String()
			continue
		}
		out = append(out, onset{beat.Start, sb.String()})
	}
	return out
}

// beatPitches is the pitches struck in a beat, lowest first; for drums the drums.
func beatPitches(info TrackInfo, beat score.Beat) []int {
	var ps []int
	for _, n := range beat.Notes {
		switch {
		case n.Tie:
		case info.Drums:
			ps = append(ps, n.Fret)
		default:
			ps = append(ps, pitch(info, n))
		}
	}
	slices.Sort(ps)
	return ps
}

// before is the onsets before a time.
func before(os []onset, t int) []onset {
	i := 0
	for i < len(os) && os[i].start < t {
		i++
	}
	return os[:i]
}

// alike reports whether two bars are the same but for a note or two: the same rhythm, with
// at most one in five notes different.
func alike(a, b []onset) bool {
	if len(a) != len(b) {
		return false
	}
	diff := 0
	for i := range a {
		if a[i].start != b[i].start {
			return false
		}
		if a[i].notes != b[i].notes {
			diff++
		}
	}
	return diff <= max(1, len(a)/5)
}

// timesPlayed is how often each bar is played, going through repeats and alternate endings.
func timesPlayed(bars []score.Bar) []int {
	plays := make([]int, len(bars))
	start := 0
	for i, b := range bars {
		plays[i] = 1
		if b.Alternate != 0 {
			plays[i] = bits.OnesCount(uint(b.Alternate))
		}
		if b.RepeatOpen {
			start = i
		}
		if b.Repeats > 0 {
			for j := start; j <= i; j++ {
				if bars[j].Alternate == 0 {
					plays[j] += b.Repeats
				}
			}
			start = i + 1
		}
	}
	return plays
}
