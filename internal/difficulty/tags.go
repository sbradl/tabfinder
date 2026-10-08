package difficulty

import "tabfinder/internal/score"

// tagRule is when a part gets a tag.
type tagRule struct {
	tag   string
	roles []Role // nil for all
	has   func(r Role, bars []bar, plays []int) bool
}

// inBars makes a rule of a test of single bars: true if bars with it come up often.
func inBars(has func(r Role, b bar) bool) func(Role, []bar, []int) bool {
	return func(r Role, bars []bar, _ []int) bool {
		return often(bars, func(b bar) bool { return has(r, b) })
	}
}

// uses is a rule for a playing technique.
func uses(fx score.Fx) func(Role, []bar, []int) bool {
	return inBars(func(_ Role, b bar) bool {
		for _, beat := range b.beats {
			if beat.Struck() && beat.AllFx()&fx != 0 {
				return true
			}
		}
		return false
	})
}

var (
	notDrums = []Role{Bass, Rhythm, Lead}
	drums    = []Role{Drums}
)

// tagRules are the tags, in the order a part lists them.
var tagRules = []tagRule{
	{"fast", nil, inBars(func(r Role, b bar) bool { return rate(b) >= fastRate[r] })},
	{"endurance", nil, func(r Role, bars []bar, _ []int) bool { return longestFast(bars, fastRate[r]) >= enduranceSeconds }},
	{"syncopated", nil, inBars(func(r Role, b bar) bool { return syncopated(b, r == Drums) })},
	{"bends", nil, uses(score.Bend)},
	{"tapping", nil, uses(score.Tap)},
	{"harmonics", nil, uses(score.Harmonic)},
	{"slap", nil, uses(score.Slap)},
	{"legato", nil, uses(score.Legato)},
	{"tremolo picking", nil, uses(score.TremoloPicking)},
	{"ghost notes", nil, inBars(func(_ Role, b bar) bool { return hasGhost(b) })},
	{"chords", notDrums, inBars(func(_ Role, b bar) bool { return hasChord(b) })},
	{"stretches", notDrums, inBars(func(_ Role, b bar) bool { return stretches(b) })},
	{"sweeps", notDrums, inBars(func(_ Role, b bar) bool { return sweeps(b) })},
	{"double kick", drums, inBars(func(_ Role, b bar) bool { return doubleKick(b) })},
	{"blast beats", drums, inBars(func(_ Role, b bar) bool { return blastBeat(b) })},
	{"odd meter", nil, inBars(func(_ Role, b bar) bool { return oddMeter(b.head) })},
	{"meter changes", nil, func(_ Role, bars []bar, _ []int) bool { return meterChanges(bars) >= 4 }},
	{"triplets", nil, inBars(func(_ Role, b bar) bool { return hasTuplet(b.beats, isTriplet) })},
	{"tuplets", nil, inBars(func(_ Role, b bar) bool { return hasTuplet(b.beats, isOddTuplet) })},
	// One passage is enough: the band has to get it right.
	{"polyrhythm", nil, func(_ Role, bars []bar, _ []int) bool { return count(bars, func(b bar) bool { return b.poly }) >= 4 }},
	{"repetitive", nil, func(_ Role, bars []bar, plays []int) bool { return repetitive(material(bars, plays)) }},
	{"many parts", nil, func(_ Role, bars []bar, plays []int) bool { d, _ := material(bars, plays); return d >= manyBars }},
}

// tags is what makes a part of a role with these bars hard; plays is how often each bar of
// the song is played.
func tags(r Role, bars []bar, plays []int) []string {
	var out []string
	for _, rule := range tagRules {
		if (rule.roles == nil || contains(rule.roles, r)) && rule.has(r, bars, plays) {
			out = append(out, rule.tag)
		}
	}
	return out
}

func contains(rs []Role, r Role) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
