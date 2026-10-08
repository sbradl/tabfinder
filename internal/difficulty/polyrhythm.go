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
	mark := func(t, b int) {
		if out[t] == nil {
			out[t] = map[int]bool{}
		}
		out[t][b] = true
	}
	for t := range sc.Tracks {
		drums := sc.Tracks[t].Drums || t < len(tracks) && tracks[t].Drums
		for b := 0; b+1 < len(sc.Tracks[t].Bars); b += 2 {
			if displaced(sc, t, b, drums) {
				for i := b; i < b+4 && i < len(sc.Tracks[t].Bars); i++ {
					mark(t, i)
				}
			}
		}
	}
	return out
}

// displaced reports whether a track's bars from b on, up to 64 sixteenths of them, repeat a
// pattern of struck notes every p sixteenths, where p doesn't divide the first bar, and
// nothing in step with the bar fits.
func displaced(sc *score.Score, t, b int, drums bool) bool {
	p, outOfStep, inStep, ok := groupingFit(sc, t, b, drums)
	return ok && p != 0 && outOfStep <= 0.05 && inStep > 0.25
}

// groupingFit looks for patterns of struck notes in a track's bars from b on, up to 64
// sixteenths of them: the period out of step with the first bar that fits best (0 for
// none), repeating at least three times, and how badly it and the best period in step
// fit: the share of notes that don't repeat. Drums count kick and snare only. ok is false
// for a window not to judge: notes off the sixteenth grid, too few notes, the same note
// all through.
func groupingFit(sc *score.Score, t, b int, drums bool) (p int, outOfStep, inStep float64, ok bool) {
	const s = score.Quarter / 4
	var grid uint64
	length := 0 // in sixteenths
	barLen := 0
	for i := b; i < len(sc.Tracks[t].Bars) && i < len(sc.Bars); i++ {
		n := barTicks(sc.Bars[i]) / s
		if length+n > 64 {
			break
		}
		if i == b {
			barLen = n
		}
		for _, beat := range sc.Tracks[t].Bars[i] {
			if !beat.Struck() || drums && !kickOrSnare(beat) {
				continue
			}
			if beat.Start%s != 0 {
				return 0, 0, 0, false
			}
			grid |= 1 << (length + beat.Start/s)
		}
		length += n
	}
	if length < 2*barLen || bits.OnesCount64(grid) < 4 {
		return 0, 0, 0, false
	}
	misfit := func(p int) float64 {
		mask := uint64(1)<<(length-p) - 1
		notes := bits.OnesCount64(grid & mask)
		if notes == 0 {
			return 1
		}
		return float64(bits.OnesCount64((grid^grid>>p)&mask)) / float64(notes)
	}
	if misfit(1) <= 0.1 {
		return 0, 0, 0, false // the same all through
	}
	outOfStep, inStep = 1, 1
	for q := 2; q <= length/2; q++ {
		f := misfit(q)
		switch {
		case barLen%q == 0 || q%barLen == 0:
			inStep = min(inStep, f)
		case 3*q <= length && f < outOfStep:
			p, outOfStep = q, f
		}
	}
	return p, outOfStep, inStep, true
}

func kickOrSnare(b score.Beat) bool {
	for _, n := range b.Notes {
		if !n.Tie && (isKick(n.Fret) || isSnare(n.Fret)) {
			return true
		}
	}
	return false
}
