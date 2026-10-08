package difficulty

import (
	"maps"
	"slices"

	"tabfinder/internal/score"
)

// fastRate is the notes per second, by role, from which playing counts as fast: about
// sixteenths at 135 BPM on bass and rhythm guitar. Drummers play sixteenth hi-hats at
// rock tempos all the time.
var fastRate = map[Role]float64{Drums: 12, Bass: 9, Rhythm: 9, Lead: 10}

// enduranceSeconds is how long playing fast without a break takes stamina.
const enduranceSeconds = 40

// longestFast is the longest time, in seconds, a track of the part plays fast without a
// bar that isn't: one bar after the other as written.
func longestFast(bars []bar, fast float64) float64 {
	longest, run := 0.0, 0.0
	prev := bar{track: -1}
	for _, b := range bars { // by track, then bar
		if b.track != prev.track || b.index != prev.index+1 || rate(b) < fast {
			run = 0
		}
		if rate(b) >= fast {
			run += b.seconds()
		}
		longest, prev = max(longest, run), b
	}
	return longest
}

// syncopated reports whether at least a quarter of a bar's notes are syncopated: struck
// on an off-beat with nothing new on the stronger beat after it, which it is held or
// rested through. Of drums only kick and snare count; cymbals keep time.
func syncopated(b bar, drums bool) bool {
	onsets := onsetTimes(b.beats, drums)
	// What comes after the last off-beat: the next bar's downbeat, unknown at the end.
	nextDownbeat := b.next == nil || onsetTimes(b.next, drums)[0]
	syncopes := 0
	for t := range onsets {
		stronger, offBeat := strongerAfter(t)
		switch {
		case !offBeat:
		case stronger < b.ticks() && !onsets[stronger]:
			syncopes++
		case stronger >= b.ticks() && !nextDownbeat:
			syncopes++
		}
	}
	return syncopes > 0 && 4*syncopes >= len(onsets)
}

// onsetTimes is the times notes are struck in a bar; for drums only kick and snare.
func onsetTimes(beats []score.Beat, drums bool) map[int]bool {
	out := map[int]bool{}
	for _, beat := range beats {
		if drums && kickOrSnare(beat) || !drums && beat.Struck() {
			out[beat.Start] = true
		}
	}
	return out
}

// strongerAfter is the stronger beat after an off-beat at t: the next beat after an eighth
// off the beat, the next eighth after a sixteenth. offBeat is false for t on the beat, and
// for tuplets.
func strongerAfter(t int) (stronger int, offBeat bool) {
	const q, e, s = score.Quarter, score.Quarter / 2, score.Quarter / 4
	switch {
	case t%q == 0:
		return 0, false
	case t%e == 0:
		return t + e, true
	case t%s == 0:
		return t + s, true
	}
	return 0, false
}

// oddMeter reports whether a bar is in an odd time signature: 5/4 or 7/8, but not 3/4 or
// a compound one like 9/8.
func oddMeter(b score.Bar) bool {
	switch b.Num {
	case 0, 1, 2, 3, 4, 6, 8, 12, 16:
		return false
	}
	return !(b.Den >= 8 && b.Num%3 == 0)
}

// meterChanges is how often the time signature changes from one bar of the part to the next.
func meterChanges(bars []bar) int {
	sigs := map[int]score.Bar{}
	for _, b := range bars {
		sigs[b.index] = b.head
	}
	order := slices.Sorted(maps.Keys(sigs))
	n := 0
	for i := 1; i < len(order); i++ {
		a, b := sigs[order[i-1]], sigs[order[i]]
		if a.Num != b.Num || a.Den != b.Den {
			n++
		}
	}
	return n
}

// isTriplet reports whether n:m tuplets are triplets: 3, or 6 or 12 played as triplets of triplets.
func isTriplet(n int) bool { return n == 3 || n == 6 || n == 12 || n == 24 }

// isOddTuplet reports whether n:m tuplets are any other: 5, 7, 9...
func isOddTuplet(n int) bool { return n > 1 && !isTriplet(n) }

// hasTuplet reports whether a beat struck is an n:m tuplet of a kind.
func hasTuplet(beats []score.Beat, kind func(n int) bool) bool {
	for _, b := range beats {
		if b.Struck() && kind(b.Tuplet) {
			return true
		}
	}
	return false
}
