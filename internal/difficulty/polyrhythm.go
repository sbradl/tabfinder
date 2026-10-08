package difficulty

import (
	"math/bits"

	"tabfinder/internal/score"
)

// polyrhythms finds the bars each track plays a polyrhythm in, by track and bar: a
// grouping repeating every few sixteenths that the bars don't divide, so it starts on a
// different beat each time (groups of three over 4/4). Triplets over a straight beat
// are no polyrhythm: they fit the beat.
func polyrhythms(sc *score.Score, tracks []TrackInfo) []map[int]bool {
	out := make([]map[int]bool, len(sc.Tracks))
	for t := range sc.Tracks {
		drums := sc.Tracks[t].Drums || t < len(tracks) && tracks[t].Drums
		for b := 0; b+1 < len(sc.Tracks[t].Bars); b += 2 {
			if !displaced(sc, t, b, drums) {
				continue
			}
			if out[t] == nil {
				out[t] = map[int]bool{}
			}
			for i := b; i < b+4 && i < len(sc.Tracks[t].Bars); i++ {
				out[t][i] = true
			}
		}
	}
	return out
}

// displaced reports whether a track's bars from b on repeat a pattern out of step with
// the bars, which nothing in step with them fits.
func displaced(sc *score.Score, t, b int, drums bool) bool {
	p, outOfStep, inStep, ok := groupingFit(sc, t, b, drums)
	return ok && p != 0 && outOfStep <= 0.05 && inStep > 0.25
}

// groupingFit looks for patterns of struck notes in a track's bars from b on, up to 64
// sixteenths of them: the period out of step with the first bar that fits best (0 for
// none), repeating at least three times, and how badly it and the best period in step
// fit: the share of notes that don't repeat. ok is false for a window not to judge (see
// sixteenths), and for the same note all through.
func groupingFit(sc *score.Score, t, b int, drums bool) (p int, outOfStep, inStep float64, ok bool) {
	g, ok := sixteenths(sc, t, b, drums)
	if !ok || g.misfit(1) <= 0.1 {
		return 0, 0, 0, false
	}
	outOfStep, inStep = 1, 1
	for q := 2; q <= g.length/2; q++ {
		f := g.misfit(q)
		switch {
		case g.barLen%q == 0 || q%g.barLen == 0:
			inStep = min(inStep, f)
		case 3*q <= g.length && f < outOfStep:
			p, outOfStep = q, f
		}
	}
	return p, outOfStep, inStep, true
}

// grid is the sixteenths notes are struck on in a few bars, bit i for the i-th.
type grid struct {
	bits           uint64
	length, barLen int // in sixteenths: all, and the first bar
}

// sixteenths is the grid of a track's bars from b on, as many as fit 64 sixteenths. Drums
// count kick and snare only. ok is false for bars with notes off the sixteenth grid, too
// few notes, or less than two bars.
func sixteenths(sc *score.Score, t, b int, drums bool) (g grid, ok bool) {
	const s = score.Quarter / 4
	for i := b; i < len(sc.Tracks[t].Bars) && i < len(sc.Bars); i++ {
		n := barTicks(sc.Bars[i]) / s
		if g.length+n > 64 {
			break
		}
		if i == b {
			g.barLen = n
		}
		for _, beat := range sc.Tracks[t].Bars[i] {
			if !beat.Struck() || drums && !kickOrSnare(beat) {
				continue
			}
			if beat.Start%s != 0 {
				return grid{}, false
			}
			g.bits |= 1 << (g.length + beat.Start/s)
		}
		g.length += n
	}
	return g, g.length >= 2*g.barLen && bits.OnesCount64(g.bits) >= 4
}

// misfit is how badly the pattern repeats after p sixteenths: the share of notes that don't.
func (g grid) misfit(p int) float64 {
	mask := uint64(1)<<(g.length-p) - 1
	notes := bits.OnesCount64(g.bits & mask)
	if notes == 0 {
		return 1
	}
	return float64(bits.OnesCount64((g.bits^g.bits>>p)&mask)) / float64(notes)
}
