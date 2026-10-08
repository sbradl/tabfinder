// Package difficulty judges what a song asks of each player of a band: it splits the
// tracks of a tab into parts (drums, bass, rhythm guitar, lead guitar) and tags each
// part with what makes it hard, such as fast playing, odd meters or tapping.
package difficulty

import (
	"fmt"
	"maps"
	"math/bits"
	"regexp"
	"slices"
	"strings"

	"tabfinder/internal/score"
)

// Role is what a band member plays.
type Role string

const (
	Drums  Role = "drums"
	Bass   Role = "bass"
	Rhythm Role = "rhythm" // rhythm guitar
	Lead   Role = "lead"   // lead guitar
)

var roles = []Role{Drums, Bass, Rhythm, Lead}

// TrackInfo is what the roles of a track are judged by, besides its notes.
type TrackInfo struct {
	Name, Instrument string // the track's name, and its General MIDI instrument or the like
	Drums            bool
	Pitches          []int // the tuning, lowest string first
}

// Part is what one role plays in a song.
type Part struct {
	Role   Role     `json:"role"`
	Tracks []int    `json:"tracks"` // the tracks it is played on, by index
	Tags   []string `json:"tags,omitempty"`
}

// Analyze splits a song into parts, in the order drums, bass, rhythm, lead, leaving out
// those nobody plays. tracks are the score's tracks, in the same order.
func Analyze(sc *score.Score, tracks []TrackInfo) []Part {
	// What each role plays: the bars of each track, by track index.
	played := map[Role]map[int][]int{}
	play := func(r Role, track, bar int) {
		if played[r] == nil {
			played[r] = map[int][]int{}
		}
		played[r][track] = append(played[r][track], bar)
	}
	for i, info := range tracks {
		if i >= len(sc.Tracks) {
			break
		}
		t := &sc.Tracks[i]
		var r Role
		switch {
		case info.Drums || t.Drums:
			r = Drums
		case isBass(info):
			r = Bass
		case isGuitar(info):
			r = Rhythm // or lead, by bar
		default:
			continue
		}
		for b, beats := range t.Bars {
			if !struck(beats) {
				continue
			}
			if r == Rhythm && isLead(beats, info) {
				play(Lead, i, b)
			} else {
				play(r, i, b)
			}
		}
	}
	poly := polyrhythms(sc, tracks)
	var out []Part
	for _, r := range roles {
		if played[r] == nil {
			continue
		}
		p := Part{Role: r}
		var bars []bar
		for i := range tracks {
			if len(played[r][i]) > 0 {
				p.Tracks = append(p.Tracks, i)
			}
			for _, b := range played[r][i] {
				var head score.Bar
				if b < len(sc.Bars) {
					head = sc.Bars[b]
				}
				var next []score.Beat
				if b+1 < len(sc.Tracks[i].Bars) {
					next = sc.Tracks[i].Bars[b+1]
				}
				bars = append(bars, bar{i, b, head, sc.Tracks[i].Bars[b], next, tracks[i], poly[i][b]})
			}
		}
		p.Tags = tags(r, bars)
		distinct, played := material(bars, timesPlayed(sc.Bars))
		if played >= 16 && 6*distinct <= played {
			p.Tags = append(p.Tags, "repetitive")
		}
		if distinct >= manyBars {
			p.Tags = append(p.Tags, "many parts")
		}
		out = append(out, p)
	}
	return out
}

// bar is a bar a part is played in on one of its tracks.
type bar struct {
	track, index int
	head         score.Bar // time signature, tempo
	beats        []score.Beat
	next         []score.Beat // the track's next bar, nil at the end
	info         TrackInfo
	poly         bool // played against another rhythm, see polyrhythms
}

// fastRate is the notes per second, by role, from which playing counts as fast: about
// sixteenths at 135 BPM on bass and rhythm guitar. Drummers play sixteenth hi-hats at
// rock tempos all the time.
var fastRate = map[Role]float64{Drums: 12, Bass: 9, Rhythm: 9, Lead: 10}

// tags is what makes a part of a role with these bars hard.
func tags(r Role, bars []bar) []string {
	var out []string
	if often(bars, func(b bar) bool { return rate(b) >= fastRate[r] }) {
		out = append(out, "fast")
	}
	if longestFast(bars, fastRate[r]) >= enduranceSeconds {
		out = append(out, "endurance")
	}
	if often(bars, func(b bar) bool { return syncopated(b, r == Drums) }) {
		out = append(out, "syncopated")
	}
	for _, tq := range techniqueTags {
		if often(bars, func(b bar) bool { return uses(b, tq.fx) }) {
			out = append(out, tq.tag)
		}
	}
	if often(bars, func(b bar) bool { return hasGhost(b) }) {
		out = append(out, "ghost notes")
	}
	if r != Drums {
		if often(bars, hasChord) {
			out = append(out, "chords")
		}
		if often(bars, stretches) {
			out = append(out, "stretches")
		}
		if often(bars, sweeps) {
			out = append(out, "sweeps")
		}
	}
	if r == Drums {
		if often(bars, doubleKick) {
			out = append(out, "double kick")
		}
		if often(bars, blastBeat) {
			out = append(out, "blast beats")
		}
	}
	if often(bars, func(b bar) bool { return oddMeter(b.head) }) {
		out = append(out, "odd meter")
	}
	if meterChanges(bars) >= 4 {
		out = append(out, "meter changes")
	}
	if often(bars, func(b bar) bool { return hasTuplet(b.beats, isTriplet) }) {
		out = append(out, "triplets")
	}
	if often(bars, func(b bar) bool { return hasTuplet(b.beats, isOddTuplet) }) {
		out = append(out, "tuplets")
	}
	if count(bars, func(b bar) bool { return b.poly }) >= 4 { // one passage is enough
		out = append(out, "polyrhythm")
	}
	return out
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
	return float64(onsets) / seconds(b.head)
}

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
			run += seconds(b.head)
		}
		longest, prev = max(longest, run), b
	}
	return longest
}

// syncopated reports whether at least a quarter of a bar's notes are syncopated: struck
// on an off-beat with nothing new on the stronger beat after it, which it is held or
// rested through. Of drums only kick and snare count; cymbals keep time.
func syncopated(b bar, drums bool) bool {
	counts := func(beat score.Beat) bool {
		if !drums {
			return beat.Struck()
		}
		for _, n := range beat.Notes {
			if !n.Tie && (isKick(n.Fret) || isSnare(n.Fret)) {
				return true
			}
		}
		return false
	}
	onsets := map[int]bool{}
	for _, beat := range b.beats {
		if counts(beat) {
			onsets[beat.Start] = true
		}
	}
	nextOnBeat := false // the next bar starts with a note
	for _, beat := range b.next {
		if beat.Start == 0 && counts(beat) {
			nextOnBeat = true
		}
	}
	const q, e, s = score.Quarter, score.Quarter / 2, score.Quarter / 4
	length := barTicks(b.head)
	syncopes := 0
	for t := range onsets {
		var stronger int
		switch {
		case t%q == 0:
			continue // on the beat
		case t%e == 0:
			stronger = t + e // the next beat
		case t%s == 0:
			stronger = t + s // the next eighth
		default:
			continue // tuplets
		}
		switch {
		case stronger < length && !onsets[stronger]:
			syncopes++
		case stronger >= length && b.next != nil && !nextOnBeat:
			syncopes++
		}
	}
	return syncopes > 0 && 4*syncopes >= len(onsets)
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
	tick := seconds(b.head) / float64(barTicks(b.head))
	n, last := 0, 0
	for _, t := range hits(b, isKick) {
		if n > 0 && float64(t-last)*tick < gap {
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
	const perSecond = 6
	secs := seconds(b.head)
	return float64(len(hits(b, isKick))) >= perSecond*secs && float64(len(hits(b, isSnare))) >= perSecond*secs
}

func isKick(midi int) bool  { return midi == 35 || midi == 36 }
func isSnare(midi int) bool { return midi >= 37 && midi <= 40 || midi == 91 }

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

// stretchFrets is the fret span that takes a stretch of the hand: five, index finger on
// the 5th fret, little finger on the 10th.
const stretchFrets = 5

// stretches reports whether a bar asks for a stretch: frets that far apart in a chord, or
// on different strings within a quarter note, where the hand has no time to move.
func stretches(b bar) bool {
	type frets struct {
		lo, hi  int
		strings map[int]bool
	}
	add := func(f *frets, n score.Note) {
		if f.strings == nil {
			f.lo, f.hi, f.strings = n.Fret, n.Fret, map[int]bool{}
		}
		f.lo, f.hi = min(f.lo, n.Fret), max(f.hi, n.Fret)
		f.strings[n.String] = true
	}
	quarters := map[int]*frets{}
	for _, beat := range b.beats {
		var chord frets
		for _, n := range beat.Notes {
			if n.Tie || n.Dead || n.Fret == 0 { // open strings need no finger
				continue
			}
			add(&chord, n)
			k := beat.Start / score.Quarter
			if quarters[k] == nil {
				quarters[k] = &frets{}
			}
			add(quarters[k], n)
		}
		if chord.hi-chord.lo >= stretchFrets {
			return true
		}
	}
	for _, f := range quarters {
		if len(f.strings) >= 2 && f.hi-f.lo >= stretchFrets {
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
	tick := seconds(b.head) / float64(barTicks(b.head))
	n, dir, last := 0, 0, score.Beat{}
	for _, beat := range b.beats {
		if !beat.Struck() {
			continue
		}
		if len(beat.Notes) != 1 {
			n = 0
			continue
		}
		step := 0
		if n > 0 {
			step = beat.Notes[0].String - last.Notes[0].String
		}
		switch {
		case n > 0 && (step == 1 || step == -1) && (n == 1 || step == dir) &&
			float64(beat.Start-last.Start)*tick < gap:
			n, dir = n+1, step
		case n > 0 && (step == 1 || step == -1) && float64(beat.Start-last.Start)*tick < gap:
			n, dir = 2, step // turned around: the last note starts the next run
		default:
			n, dir = 1, 0
		}
		if n >= run {
			return true
		}
		last = beat
	}
	return false
}

// techniqueTags are the tags of playing techniques worth knowing about.
var techniqueTags = []struct {
	fx  score.Fx
	tag string
}{
	{score.Bend, "bends"},
	{score.Tap, "tapping"},
	{score.Harmonic, "harmonics"},
	{score.Slap, "slap"},
	{score.Legato, "legato"},
	{score.TremoloPicking, "tremolo picking"},
}

// uses reports whether a note struck in a bar is played with a technique.
func uses(b bar, fx score.Fx) bool {
	for _, beat := range b.beats {
		if beat.Struck() && beat.AllFx()&fx != 0 {
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

// often reports whether bars with something come up in a part more than once or twice:
// in at least two bars and every eighth.
func often(bars []bar, has func(bar) bool) bool {
	n := count(bars, has)
	return n >= 2 && 8*n >= len(bars)
}

// count is the number of bars with something.
func count(bars []bar, has func(bar) bool) int {
	n := 0
	for _, b := range bars {
		if has(b) {
			n++
		}
	}
	return n
}

// hasTuplet reports whether a beat struck is an n:m tuplet of a kind.
func hasTuplet(beats []score.Beat, kind func(n int) bool) bool {
	for _, b := range beats {
		if b.Struck() && kind(b.Tuplet) {
			return true
		}
	}
	return false
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

var (
	reLeadName   = regexp.MustCompile(`(?i)lead|solo`)
	reRhythmName = regexp.MustCompile(`(?i)rhy`)
)

// isLead reports whether a guitar's bar is a lead line rather than rhythm playing:
// mostly single notes, not palm-muted, up high or bent and slid. Riffs are chords or low
// single notes.
func isLead(beats []score.Beat, t TrackInfo) bool {
	var struckBeats, single, notes, muted, pitchSum int
	expressive := false
	for _, b := range beats {
		if !b.Struck() {
			continue
		}
		struckBeats++
		if len(b.Notes) == 1 {
			single++
		}
		for _, n := range b.Notes {
			notes++
			if n.Fx&score.PalmMute != 0 {
				muted++
			}
			if n.Fx&(score.Bend|score.Vibrato|score.Slide|score.Tap|score.Legato) != 0 {
				expressive = true
			}
			pitchSum += pitch(t, n)
		}
	}
	if struckBeats == 0 || 10*single < 6*struckBeats || 2*muted >= notes {
		return false
	}
	low, high := 50, 57 // D3, A3: an expressive line above the first, any line above the second
	switch {
	case reLeadName.MatchString(t.Name):
		low, high = 45, 50
	case reRhythmName.MatchString(t.Name):
		low, high = 57, 62
	}
	mean := pitchSum / notes
	return mean >= high || expressive && mean >= low
}

// pitch is a note's MIDI pitch, guessing a standard-tuned guitar if the tuning is unknown.
func pitch(t TrackInfo, n score.Note) int {
	if n.String >= 0 && n.String < len(t.Pitches) {
		return t.Pitches[n.String] + n.Fret
	}
	std := []int{40, 45, 50, 55, 59, 64}
	return std[min(max(n.String, 0), len(std)-1)] + n.Fret
}

var (
	reBassName   = regexp.MustCompile(`(?i)bass`)
	reGuitarName = regexp.MustCompile(`(?i)guit|gtr|git|lead|solo|rhy`)
)

func isBass(t TrackInfo) bool {
	if reBassName.MatchString(t.Instrument) || reBassName.MatchString(t.Name) {
		return true
	}
	n := len(t.Pitches)
	return (n == 4 || n == 5) && t.Pitches[0] < 40 // below a guitar's low E
}

// isGuitar reports whether a track is a guitar: named so, with six strings or more, the
// lowest no higher than an E3 (a guitar tuned up a lot; a violin's G3 is too high).
func isGuitar(t TrackInfo) bool {
	if len(t.Pitches) < 6 || t.Pitches[0] > 52 {
		return false // sung, or no guitar's strings
	}
	return strings.Contains(t.Instrument, "Guitar") || reGuitarName.MatchString(t.Name)
}

// manyBars is the number of different bars from which a part has much to learn: eight
// sections of four bars, all different. In a library of mostly metal songs about a
// quarter of the guitar parts have that many.
const manyBars = 32

// material is how much there is to learn of a part: the number of different bars on the
// track that has the most, and how many bars that track plays, repeats included. A bar
// played again higher up the neck, or with a note or two changed, is no new bar.
func material(bars []bar, plays []int) (distinct, played int) {
	byTrack := map[int][]bar{}
	for _, b := range bars {
		byTrack[b.track] = append(byTrack[b.track], b)
	}
	for _, tbars := range byTrack {
		var kinds [][]onset
		n := 0
		for _, b := range tbars {
			if b.index < len(plays) {
				n += plays[b.index]
			} else {
				n++
			}
			o := onsets(b)
			same := func(k []onset) bool { return alike(k, o) }
			if b.info.Drums { // a fill: a known groove for the first half of the bar
				half := barTicks(b.head) / 2
				same = func(k []onset) bool { return alike(before(k, half), before(o, half)) }
			}
			if !slices.ContainsFunc(kinds, same) {
				kinds = append(kinds, o)
			}
		}
		if len(kinds) > distinct || len(kinds) == distinct && n > played {
			distinct, played = len(kinds), n
		}
	}
	return distinct, played
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
		var ps []int
		for _, n := range beat.Notes {
			if n.Tie {
				continue
			}
			p := n.Fret // a drum
			if !b.info.Drums {
				p = pitch(b.info, n)
			}
			ps = append(ps, p)
		}
		slices.Sort(ps)
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
