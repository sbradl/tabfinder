package difficulty

import (
	"math"
	"slices"

	"tabfinder/internal/score"
)

// rating is how hard a part is on a scale of 1 to 10, rounded to a tenth: mostly how fast
// it is, plus what else it asks for.
func rating(r Role, bars []bar) float64 {
	hard := speed(bars) + 0.3*stretchiness(r, bars)
	return math.Round((1+9*clamp(hard))*10) / 10
}

// stretchiness is how much a part stretches the fretting hand, from 0 to 1 for a stretch in
// half the bars or more. Drums have none.
func stretchiness(r Role, bars []bar) float64 {
	if r == Drums {
		return 0
	}
	return scale(share(bars, stretches), 0, 0.5)
}

// share is the share of bars with something.
func share(bars []bar, has func(bar) bool) float64 {
	if len(bars) == 0 {
		return 0
	}
	return float64(count(bars, has)) / float64(len(bars))
}

// speed is how fast a part is for either hand, from 0 to 1: what each keeps up for a while,
// the 90th percentile of its bars. Either hand alone can make a part hard; both, harder.
func speed(bars []bar) float64 {
	picking := scale(percentile(bars, 0.9, picksPerSecond), 2, 16)
	fretting := scale(percentile(bars, 0.9, changesPerSecond), 1, 10)
	return either(picking, fretting)
}

// picksPerSecond is how often the picking hand strikes in a bar: notes struck together
// once, a hammer-on, pull-off or tapped note half.
func picksPerSecond(b bar) float64 {
	picks, last := 0.0, -1
	for _, beat := range b.beats { // sorted by start
		if !beat.Struck() || beat.Start == last {
			continue
		}
		last = beat.Start
		if unpicked(beat) {
			picks += 0.5
		} else {
			picks++
		}
	}
	return picks / b.seconds()
}

// unpicked reports whether all notes of a beat sound without a pick: hammered, pulled or tapped.
func unpicked(beat score.Beat) bool {
	if beat.Fx&score.Tap != 0 {
		return true
	}
	for _, n := range beat.Notes {
		if !n.Tie && n.Fx&(score.Legato|score.Tap) == 0 {
			return false
		}
	}
	return true
}

// changesPerSecond is how often the fretting hand changes what it holds in a bar: a beat
// with fretted notes other than the beat before. Open strings and repeated notes are free.
func changesPerSecond(b bar) float64 {
	changes := 0
	var held []score.Note
	for _, beat := range b.beats {
		frets := frettedNotes(beat)
		if len(frets) > 0 && !slices.Equal(frets, held) {
			changes++
		}
		if beat.Struck() {
			held = frets
		}
	}
	return float64(changes) / b.seconds()
}

// frettedNotes is the string and fret of the notes of a beat that need a finger.
func frettedNotes(beat score.Beat) []score.Note {
	var out []score.Note
	for _, n := range beat.Notes {
		if fretted(n) {
			out = append(out, score.Note{String: n.String, Fret: n.Fret})
		}
	}
	return out
}

// either combines two measures of 0 to 1 so that each alone can reach 1.
func either(a, b float64) float64 { return 1 - (1-a)*(1-b) }

// percentile is the p-th percentile of a measure of bars.
func percentile(bars []bar, p float64, measure func(bar) float64) float64 {
	if len(bars) == 0 {
		return 0
	}
	vs := make([]float64, len(bars))
	for i, b := range bars {
		vs[i] = measure(b)
	}
	slices.Sort(vs)
	return vs[min(int(p*float64(len(vs))), len(vs)-1)]
}

// scale maps v from a range onto 0 to 1, linearly, clamped.
func scale(v, from, to float64) float64 { return clamp((v - from) / (to - from)) }

func clamp(v float64) float64 { return min(max(v, 0), 1) }
