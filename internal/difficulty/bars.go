package difficulty

import "tabfinder/internal/score"

// bar is a bar a part is played in on one of its tracks, with what the tags and the score
// need to know of it, worked out once.
type bar struct {
	track, index int
	head         score.Bar // time signature, tempo
	beats        []score.Beat
	next         []score.Beat // the track's next bar, nil at the end
	info         TrackInfo
	drums        bool
	poly         bool // a polyrhythm is played in it, see polyrhythms

	rate   float64  // notes per second
	picks  float64  // strikes of the picking hand per second
	grips  float64  // times per second the fretting hand puts fingers down
	fx     score.Fx // techniques of the notes struck
	onsets []onset  // what's struck when, to tell bars apart

	syncopated, stretched, chord, sweep, ghost bool
	doubleKick, blast                          bool
	triplets, oddTuplets                       bool
}

func newBar(sc *score.Score, info TrackInfo, track, index int, poly bool) bar {
	b := bar{track: track, index: index, beats: sc.Tracks[track].Bars[index], info: info, poly: poly,
		drums: info.Drums || sc.Tracks[track].Drums}
	if index < len(sc.Bars) {
		b.head = sc.Bars[index]
	}
	if index+1 < len(sc.Tracks[track].Bars) {
		b.next = sc.Tracks[track].Bars[index+1]
	}
	b.rate, b.picks, b.onsets = rate(b), picksPerSecond(b), onsets(b)
	b.syncopated, b.ghost = syncopated(b, b.drums), hasGhost(b)
	b.triplets, b.oddTuplets = hasTuplet(b.beats, isTriplet), hasTuplet(b.beats, isOddTuplet)
	for _, beat := range b.beats {
		if beat.Struck() {
			b.fx |= beat.AllFx()
		}
	}
	quarters := quarterGrips(b)
	b.grips = gripsPerSecond(b, quarters) // TODO: drums have no fretting hand; it counts their notes
	if b.drums {
		b.doubleKick, b.blast = doubleKick(b), blastBeat(b)
	} else {
		b.stretched, b.chord, b.sweep = stretches(b, quarters), hasChord(b), sweeps(b)
	}
	return b
}

// ticks is the bar's length.
func (b bar) ticks() int { return barTicks(b.head) }

// seconds is how long the bar takes.
func (b bar) seconds() float64 { return seconds(b.head) }

// secondsPerTick is how long a tick of the bar takes.
func (b bar) secondsPerTick() float64 { return b.seconds() / float64(b.ticks()) }

// barTicks is a bar's length.
func barTicks(b score.Bar) int {
	if b.Num > 0 && b.Den > 0 {
		return b.Num * 4 * score.Quarter / b.Den
	}
	return 4 * score.Quarter
}

// seconds is how long a bar takes; at 120 BPM if its tempo is unknown.
func seconds(b score.Bar) float64 {
	bpm := b.BPM
	if bpm <= 0 {
		bpm = 120
	}
	return float64(barTicks(b)) / score.Quarter * 60 / bpm
}

// rate is the notes per second played in a bar, counting notes struck together once.
func rate(b bar) float64 {
	onsets, last := 0, -1
	for _, beat := range b.beats { // sorted by start
		if beat.Struck() && beat.Start != last {
			onsets++
			last = beat.Start
		}
	}
	return float64(onsets) / b.seconds()
}

// struck reports whether any of the beats strikes a note.
func struck(beats []score.Beat) bool {
	for _, b := range beats {
		if b.Struck() {
			return true
		}
	}
	return false
}

// often reports whether bars with something come up in a part more than once or twice:
// in at least two bars and every eighth.
func often(bars []bar, has func(*bar) bool) bool {
	n := count(bars, has)
	return n >= 2 && 8*n >= len(bars)
}

// count is the number of bars with something.
func count(bars []bar, has func(*bar) bool) int {
	n := 0
	for i := range bars {
		if has(&bars[i]) {
			n++
		}
	}
	return n
}

// share is the share of bars with something.
func share(bars []bar, has func(*bar) bool) float64 {
	if len(bars) == 0 {
		return 0
	}
	return float64(count(bars, has)) / float64(len(bars))
}
