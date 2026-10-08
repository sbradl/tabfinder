package difficulty

import (
	"math"
	"slices"

	"tabfinder/internal/score"
)

// rating is how hard a part is on a scale of 1 to 10, rounded to a tenth: mostly what it
// asks of either hand, plus its rhythm and how much there is to learn. plays is how often
// each bar of the song is played.
func rating(r Role, bars []bar, plays []int) float64 {
	hands := either(pickingHand(bars, plays), frettingHand(bars))
	hard := hands + 0.15*rhythmic(r, bars) + 0.15*learning(bars, plays)
	return math.Round((1+9*soften(hard))*10) / 10
}

// soften maps how hard a part is, 0 to 1.3 (a hand at its hardest, rhythm and learning
// too), onto 0 to 1, rising less towards the top: very hard parts still differ.
func soften(hard float64) float64 {
	const top, bend = 1.3, 2
	return (1 - math.Exp(-bend*min(hard, top))) / (1 - math.Exp(-bend*top))
}

// pickingHand is how hard a part is for the picking hand, from 0 to 1: half how fast it
// picks through the faster quarter of its bars, half how long it picks fast all in all
// (five minutes for 1).
//
// Speed is judged against a limit: picking gets hard all at once around it, from
// comfortable to undoable, and past it more speed adds little. Fitted to a band's own
// ranking of songs, the limit is 6 strikes a second.
func pickingHand(bars []bar, plays []int) float64 {
	pace := 1 / (1 + math.Exp(-pickSteepness*(percentile(bars, 0.75, picksPerSecond)-fastPickRate)))
	stamina := scale(fastPicking(bars, plays), 0, 300)
	return 0.5*pace + 0.5*stamina
}

// fastPickRate is the strikes per second from which picking is fast: eighths at 180 BPM,
// sixteenths at 90.
const fastPickRate = 6

// pickSteepness is how sharply picking gets hard around fastPickRate: from a tenth of the
// way at 0.7 strikes a second below it to nine tenths at 0.7 above.
const pickSteepness = 3

// fastPicking is how long a part picks fast, in seconds, repeats included.
func fastPicking(bars []bar, plays []int) float64 {
	total := 0.0
	for _, b := range bars {
		if picksPerSecond(b) >= fastPickRate {
			total += b.seconds() * float64(timesOf(b, plays))
		}
	}
	return total
}

// timesOf is how often a bar is played.
func timesOf(b bar, plays []int) int {
	if b.index < len(plays) {
		return plays[b.index]
	}
	return 1
}

// frettingHand is how hard a part is for the fretting hand, from 0 to 1 for 10 grips a
// second, a stretch counting double: what it keeps up for a while, the 90th percentile of
// its bars. A stretch held over many strokes costs once; stretch after stretch, each time.
func frettingHand(bars []bar) float64 {
	return scale(percentile(bars, 0.9, gripsPerSecond), 1, 10)
}

// learning is how much there is to learn of a part, from 0 for up to 8 different bars to 1
// for 40 or more.
func learning(bars []bar, plays []int) float64 {
	distinct, _ := material(bars, plays)
	return scale(float64(distinct), 8, 40)
}

// rhythmic is how tricky a part's rhythm is, from 0 to 1: odd meters, meter changes,
// syncopation, polyrhythm and tuplets other than triplets.
func rhythmic(r Role, bars []bar) float64 {
	odd := scale(share(bars, func(b bar) bool { return oddMeter(b.head) }), 0, 0.5)
	changes := scale(float64(meterChanges(bars)), 0, 16)
	synco := scale(share(bars, func(b bar) bool { return syncopated(b, r == Drums) }), 0, 0.75)
	poly := scale(float64(count(bars, func(b bar) bool { return b.poly })), 0, 8)
	tuplets := scale(share(bars, func(b bar) bool { return hasTuplet(b.beats, isOddTuplet) }), 0, 0.25)
	return clamp(0.4*odd + 0.4*changes + 0.4*synco + 0.5*poly + 0.3*tuplets)
}

// share is the share of bars with something.
func share(bars []bar, has func(bar) bool) float64 {
	if len(bars) == 0 {
		return 0
	}
	return float64(count(bars, has)) / float64(len(bars))
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

// gripsPerSecond is how often the fretting hand puts fingers down in a bar: beats with a
// fretted note not played within the quarter note before, where a finger may still be.
// Open strings and repeated notes are free; a stretch counts double.
func gripsPerSecond(b bar) float64 {
	type finger struct{ string, fret int }
	lastPlayed := map[finger]int{} // when
	quarters := quarterGrips(b)
	grips := 0
	for _, beat := range b.beats {
		changed := false
		for _, n := range beat.Notes {
			if !fretted(n) {
				continue
			}
			f := finger{n.String, n.Fret}
			if t, ok := lastPlayed[f]; !ok || beat.Start-t > score.Quarter {
				changed = true
			}
			lastPlayed[f] = beat.Start
		}
		switch {
		case !changed:
		case quarters[beat.Start/score.Quarter].stretched():
			grips += 2
		default:
			grips++
		}
	}
	return float64(grips) / b.seconds()
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
