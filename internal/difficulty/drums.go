package difficulty

import "tabfinder/internal/score"

func isKick(midi int) bool  { return midi == 35 || midi == 36 }
func isSnare(midi int) bool { return midi >= 37 && midi <= 40 || midi == 91 }

// kickOrSnare reports whether a beat strikes the kick or the snare.
func kickOrSnare(b score.Beat) bool {
	for _, n := range b.Notes {
		if !n.Tie && (isKick(n.Fret) || isSnare(n.Fret)) {
			return true
		}
	}
	return false
}

// hits is the times a drum is hit in a bar, each once (voices may double a hit).
func hits(b bar, drum func(midi int) bool) []int {
	var out []int
	for _, beat := range b.beats { // sorted by start
		if len(out) > 0 && out[len(out)-1] == beat.Start {
			continue
		}
		for _, n := range beat.Notes {
			if !n.Tie && drum(n.Fret) {
				out = append(out, beat.Start)
				break
			}
		}
	}
	return out
}

// doubleKick reports whether a bar has a run of six kicks, each less than 0.13 s after the
// one before: more than one foot plays.
func doubleKick(b bar) bool {
	const gap, run = 0.13, 6
	n, last := 0, 0
	for _, t := range hits(b, isKick) {
		if n > 0 && float64(t-last)*b.secondsPerTick() < gap {
			n++
		} else {
			n = 1
		}
		if n >= run {
			return true
		}
		last = t
	}
	return false
}

// blastBeat reports whether kick and snare are both hit at least six times a second in a bar.
func blastBeat(b bar) bool {
	least := 6 * b.seconds()
	return float64(len(hits(b, isKick))) >= least && float64(len(hits(b, isSnare))) >= least
}
