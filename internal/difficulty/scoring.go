package difficulty

import (
	"math"
	"slices"

	"tabfinder/internal/score"
)

// rating is how hard a part is on a scale of 1 to 10, rounded to a tenth: mostly what it
// asks of either hand, plus its rhythm and how much there is to learn.
//
// Fitted to one guitarist's sense of the songs they play: an intermediate metal player for
// whom a rating up to 6 is comfortable, 7 and 8 take effort and 9 and 10 are out of reach.
func rating(p *part) float64 {
	hands := either(pickingHand(p.bars, p.plays), frettingHand(p.bars))
	hard := hands + 0.15*rhythmic(p.bars) + 0.05*learning(p)
	return math.Round((1+9*soften(hard))*10) / 10
}

// soften maps how hard a part is, 0 to 1.34 (a hand at its hardest, rhythm and learning
// too), onto 0 to 1, rising less towards the top: very hard parts still differ.
func soften(hard float64) float64 {
	const top, bend = 1.34, 3.15
	return (1 - math.Exp(-bend*min(hard, top))) / (1 - math.Exp(-bend*top))
}

// pickingHand is how hard a part is for the picking hand, from 0 to 1: a little for how
// fast it picks through the faster quarter of its bars, much for how long it picks very
// fast (over five minutes for 1).
//
// Fast picking is something metal rhythm players learn: it starts to count around eighths
// at 160 BPM and adds little beyond. What few can play is minutes of very fast picking,
// from about 13 strikes a second (sixteenths at 195 BPM).
func pickingHand(bars []bar, plays []int) float64 {
	pace := 1 / (1 + math.Exp(-pickSteepness*(percentile(bars, 0.75, func(b *bar) float64 { return b.picks })-fastPickRate)))
	stamina := scale(veryFastPicking(bars, plays), 0, 320)
	return 0.24*pace + 0.68*stamina
}

// fastPickRate is the strikes per second around which picking starts to count: eighths at
// 160 BPM, sixteenths at 80.
const fastPickRate = 5.4

// pickSteepness is how sharply picking starts to count around fastPickRate: from a tenth of
// the way at 0.7 strikes a second below it to nine tenths at 0.7 above.
const pickSteepness = 3.1

// veryFastRate is the strikes per second from which picking is very fast, and veryFastRamp
// how sharply: a bar at a strike a second less counts for next to nothing.
const veryFastRate, veryFastRamp = 13.2, 0.3

// veryFastPicking is how long a part picks very fast, in seconds, repeats included: each
// bar counts by how close it comes to veryFastRate.
func veryFastPicking(bars []bar, plays []int) float64 {
	total := 0.0
	for i := range bars {
		b := &bars[i]
		weight := 1 / (1 + math.Exp(-(b.picks-veryFastRate)/veryFastRamp))
		total += weight * b.seconds() * float64(timesOf(b, plays))
	}
	return total
}

// timesOf is how often a bar is played.
func timesOf(b *bar, plays []int) int {
	if b.index < len(plays) {
		return plays[b.index]
	}
	return 1
}

// frettingHand is how hard a part is for the fretting hand, from 0 to 1 for 21.6 grips a
// second, a stretch counting double: what it keeps up for a while, the 90th percentile of
// its bars. A stretch held over many strokes costs once; stretch after stretch, each time.
func frettingHand(bars []bar) float64 {
	return scale(percentile(bars, 0.9, func(b *bar) float64 { return b.grips }), 1, 21.6)
}

// learning is how much there is to learn of a part, from 0 for up to 8 different bars to 1
// for 40 or more.
func learning(p *part) float64 { return scale(float64(p.distinct), 8, 40) }

// rhythmic is how tricky a part's rhythm is, from 0 to 1: odd meters, meter changes,
// syncopation, polyrhythm and tuplets other than triplets.
func rhythmic(bars []bar) float64 {
	odd := scale(share(bars, func(b *bar) bool { return oddMeter(b.head) }), 0, 0.5)
	changes := scale(float64(meterChanges(bars)), 0, 16)
	synco := scale(share(bars, func(b *bar) bool { return b.syncopated }), 0, 0.75)
	poly := scale(float64(count(bars, func(b *bar) bool { return b.poly })), 0, 8)
	tuplets := scale(share(bars, func(b *bar) bool { return b.oddTuplets }), 0, 0.25)
	return clamp(0.4*odd + 0.4*changes + 0.4*synco + 0.5*poly + 0.3*tuplets)
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
func gripsPerSecond(b bar, quarters []grip) float64 {
	type finger struct{ string, fret, when int }
	var fingers []finger // where fingers were put down, and when last
	grips := 0
	for _, beat := range b.beats {
		changed := false
		for _, n := range beat.Notes {
			if !fretted(n) {
				continue
			}
			i := slices.IndexFunc(fingers, func(f finger) bool { return f.string == n.String && f.fret == n.Fret })
			switch {
			case i < 0:
				fingers = append(fingers, finger{n.String, n.Fret, beat.Start})
				changed = true
			case beat.Start-fingers[i].when > score.Quarter:
				changed = true
				fallthrough
			default:
				fingers[i].when = beat.Start
			}
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
func percentile(bars []bar, p float64, measure func(*bar) float64) float64 {
	if len(bars) == 0 {
		return 0
	}
	vs := make([]float64, len(bars))
	for i := range bars {
		vs[i] = measure(&bars[i])
	}
	slices.Sort(vs)
	return vs[min(int(p*float64(len(vs))), len(vs)-1)]
}

// scale maps v from a range onto 0 to 1, linearly, clamped.
func scale(v, from, to float64) float64 { return clamp((v - from) / (to - from)) }

func clamp(v float64) float64 { return min(max(v, 0), 1) }
