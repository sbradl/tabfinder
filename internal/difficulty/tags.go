package difficulty

import "tabfinder/internal/score"

// part is the bars a role plays in a song, with how much there is to learn of it.
type part struct {
	role             Role
	bars             []bar
	plays            []int // how often each bar of the song is played
	distinct, played int   // see material
}

func newPartOf(r Role, bars []bar, plays []int) *part {
	p := &part{role: r, bars: bars, plays: plays}
	p.distinct, p.played = material(bars, plays)
	return p
}

// tagRule is when a part gets a tag.
type tagRule struct {
	tag   string
	roles []Role // nil for all
	has   func(p *part) bool
}

// inBars makes a rule of a test of single bars: true if bars with it come up often.
func inBars(has func(b *bar) bool) func(*part) bool {
	return func(p *part) bool { return often(p.bars, has) }
}

// uses is a rule for a playing technique.
func uses(fx score.Fx) func(*part) bool {
	return inBars(func(b *bar) bool { return b.fx&fx != 0 })
}

var (
	notDrums = []Role{Bass, Rhythm, Lead}
	drums    = []Role{Drums}
)

// tagRules are the tags, in the order a part lists them.
var tagRules = []tagRule{
	{"fast", nil, func(p *part) bool { return often(p.bars, func(b *bar) bool { return b.rate >= fastRate[p.role] }) }},
	{"endurance", nil, func(p *part) bool { return longestFast(p.bars, fastRate[p.role]) >= enduranceSeconds }},
	{"syncopated", nil, inBars(func(b *bar) bool { return b.syncopated })},
	{"bends", nil, uses(score.Bend)},
	{"tapping", nil, uses(score.Tap)},
	{"harmonics", nil, uses(score.Harmonic)},
	{"slap", nil, uses(score.Slap)},
	{"legato", nil, uses(score.Legato)},
	{"tremolo picking", nil, uses(score.TremoloPicking)},
	{"ghost notes", nil, inBars(func(b *bar) bool { return b.ghost })},
	{"chords", notDrums, inBars(func(b *bar) bool { return b.chord })},
	{"stretches", notDrums, inBars(func(b *bar) bool { return b.stretched })},
	{"sweeps", notDrums, inBars(func(b *bar) bool { return b.sweep })},
	{"double kick", drums, inBars(func(b *bar) bool { return b.doubleKick })},
	{"blast beats", drums, inBars(func(b *bar) bool { return b.blast })},
	{"odd meter", nil, inBars(func(b *bar) bool { return oddMeter(b.head) })},
	{"meter changes", nil, func(p *part) bool { return meterChanges(p.bars) >= 4 }},
	{"triplets", nil, inBars(func(b *bar) bool { return b.triplets })},
	{"tuplets", nil, inBars(func(b *bar) bool { return b.oddTuplets })},
	// One passage is enough: the band has to get it right.
	{"polyrhythm", nil, func(p *part) bool { return count(p.bars, func(b *bar) bool { return b.poly }) >= 4 }},
	{"repetitive", nil, func(p *part) bool { return repetitive(p.distinct, p.played) }},
	{"many parts", nil, func(p *part) bool { return p.distinct >= manyBars }},
}

// tags is what makes a part hard.
func tags(p *part) []string {
	var out []string
	for _, rule := range tagRules {
		if (rule.roles == nil || contains(rule.roles, p.role)) && rule.has(p) {
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
