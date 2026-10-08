package difficulty

import (
	"math"
	"math/bits"

	"tabfinder/internal/score"
)

// hasChord reports whether a bar has a chord: three different notes or more struck together,
// so more than a power chord.
func hasChord(b bar) bool {
	for _, beat := range b.beats {
		var classes [12]bool
		n := 0
		for _, note := range beat.Notes {
			if note.Tie || note.Dead {
				continue
			}
			if c := pitch(b.info, note) % 12; !classes[c] {
				classes[c], n = true, n+1
			}
		}
		if n >= 3 {
			return true
		}
	}
	return false
}

// stretchReach is the reach that takes a stretch of the hand, as a share of the scale
// length: four frets at the 7th, from the 7th to the 11th. One finger per fret spans three.
// Frets get narrower up the neck, so the same number of frets is a smaller reach there:
// four frets low on the neck are a stretch, five at the 12th are not.
var stretchReach = reach(7, 11) - 1e-9

// reach is the distance from one fret to another as a share of the scale length.
func reach(lo, hi int) float64 { return math.Exp2(-float64(lo)/12) - math.Exp2(-float64(hi)/12) }

// grip is fretted notes the hand holds or plays without moving.
type grip struct {
	lo, hi   int    // frets
	strings  uint32 // the strings, bit n for string n
	nStrings int    // of the guitar
}

func (g *grip) add(n score.Note) {
	if n.String < 0 || n.String > 31 {
		return
	}
	if g.strings == 0 {
		g.lo, g.hi = n.Fret, n.Fret
	}
	g.lo, g.hi = min(g.lo, n.Fret), max(g.hi, n.Fret)
	g.strings |= 1 << n.String
}

// strain is how far the hand reaches for a grip, as a share of the scale length: the reach
// along the neck, made longer by strings skipped (the fingers spread across too) and by
// the low strings (the hand has to wrap further around the neck), up to a tenth on the lowest.
func (g *grip) strain() float64 {
	if g.strings == 0 {
		return 0
	}
	low, high := bits.TrailingZeros32(g.strings), 31-bits.LeadingZeros32(g.strings)
	skipped := high - low + 1 - bits.OnesCount32(g.strings)
	lowness := 0.0
	if g.nStrings > 1 {
		lowness = 1 - float64(low)/float64(g.nStrings-1)
	}
	return reach(g.lo, g.hi) * (1 + 0.15*float64(skipped)) * (1 + 0.1*lowness)
}

// stretched reports whether a grip is a stretch: four frets or more (one finger per fret
// spans three) and a strain of a stretch.
func (g *grip) stretched() bool {
	return g.strings != 0 && g.hi-g.lo >= 4 && g.strain() >= stretchReach
}

// fretted reports whether a note needs a finger: struck, not open.
func fretted(n score.Note) bool { return !n.Tie && !n.Dead && n.Fret > 0 }

// quarterGrips is the grips of fretted notes on different strings within each quarter note
// of a bar, where the hand has no time to move, by quarter; empty for a quarter of notes
// on one string or none.
func quarterGrips(b bar) []grip {
	n := 0
	for _, beat := range b.beats {
		n = max(n, beat.Start/score.Quarter+1)
	}
	quarters := make([]grip, n)
	for _, beat := range b.beats {
		for _, note := range beat.Notes {
			if fretted(note) {
				g := &quarters[beat.Start/score.Quarter]
				g.nStrings = len(b.info.Pitches)
				g.add(note)
			}
		}
	}
	for i := range quarters {
		if bits.OnesCount32(quarters[i].strings) < 2 {
			quarters[i] = grip{}
		}
	}
	return quarters
}

// stretches reports whether a bar asks for a stretch of the fretting hand: in a chord, or
// in a quarter note's grip.
func stretches(b bar, quarters []grip) bool {
	for _, beat := range b.beats {
		chord := grip{nStrings: len(b.info.Pitches)}
		for _, n := range beat.Notes {
			if fretted(n) {
				chord.add(n)
			}
		}
		if chord.stretched() {
			return true
		}
	}
	for i := range quarters {
		if quarters[i].stretched() {
			return true
		}
	}
	return false
}

// sweeps reports whether a bar has a sweep: single notes over four strings or more, one
// string after the next in one direction, each less than 0.11 s after the one before
// (sixteenths at 136 BPM), too fast to pick string by string.
func sweeps(b bar) bool {
	const gap, run = 0.11, 4
	// next reports whether a note follows on from the one before: the next string, quickly.
	next := func(prev, cur score.Beat) (step int, ok bool) {
		step = cur.Notes[0].String - prev.Notes[0].String
		quick := float64(cur.Start-prev.Start)*b.secondsPerTick() < gap
		return step, (step == 1 || step == -1) && quick
	}
	var line []score.Beat // single notes on string after string, one way
	for _, beat := range b.beats {
		if !beat.Struck() {
			continue
		}
		if len(beat.Notes) != 1 {
			line = nil
			continue
		}
		if len(line) == 0 {
			line = []score.Beat{beat}
			continue
		}
		last := line[len(line)-1]
		step, ok := next(last, beat)
		switch {
		case !ok:
			line = []score.Beat{beat}
		case len(line) == 1 || step == line[1].Notes[0].String-line[0].Notes[0].String:
			line = append(line, beat)
		default: // turned around: the last note starts the next run
			line = []score.Beat{last, beat}
		}
		if len(line) >= run {
			return true
		}
	}
	return false
}

func hasGhost(b bar) bool {
	for _, beat := range b.beats {
		for _, n := range beat.Notes {
			if n.Ghost && !n.Tie {
				return true
			}
		}
	}
	return false
}
