package difficulty_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

// Made-up parts, built note by note. Times are in ticks: a quarter is score.Quarter.

const (
	q = score.Quarter
	e = q / 2 // eighth
	s = q / 4 // sixteenth
)

var (
	stdGuitar = []int{40, 45, 50, 55, 59, 64}
	bass4     = []int{28, 33, 38, 43}
)

// bars4 is n bars of 4/4 at bpm.
func bars4(n int, bpm float64) []score.Bar {
	out := make([]score.Bar, n)
	for i := range out {
		out[i] = score.Bar{Num: 4, Den: 4, BPM: bpm}
	}
	return out
}

// every repeats beats of dur from 0 to the end of a 4/4 bar, each with these notes.
func every(dur int, notes ...score.Note) []score.Beat {
	var out []score.Beat
	for t := 0; t < 4*q; t += dur {
		out = append(out, score.Beat{Start: t, Dur: dur, Notes: notes})
	}
	return out
}

// track is a track playing the same bar n times.
func track(n int, bar []score.Beat) score.Track {
	t := score.Track{Bars: make([][]score.Beat, n)}
	for i := range t.Bars {
		t.Bars[i] = bar
	}
	return t
}

// powerChord is E5 on the low strings.
var powerChord = []score.Note{{String: 0, Fret: 0}, {String: 1, Fret: 2}}

func TestRoles(t *testing.T) {
	kick := score.Note{Fret: 36}
	sc := &score.Score{
		Bars: bars4(4, 120),
		Tracks: []score.Track{
			{Drums: true, Bars: track(4, every(q, kick)).Bars},
			track(4, every(e, score.Note{String: 0, Fret: 0})),
			track(4, every(e, powerChord...)),
			track(4, every(q, score.Note{String: 3, Fret: 5})),
			track(4, nil), // a guitar that doesn't play
			track(4, every(q, score.Note{Fret: 0})),
			track(4, every(q, score.Note{Fret: 0})),
		},
	}
	tracks := []difficulty.TrackInfo{
		{Name: "Drums", Instrument: "Drums", Drums: true},
		{Name: "Bass", Instrument: "Electric Bass (finger)", Pitches: bass4},
		{Name: "Guitar", Instrument: "Distortion Guitar", Pitches: stdGuitar},
		{Name: "Woodwind", Instrument: "Oboe", Pitches: stdGuitar},
		{Name: "Guitar 2", Instrument: "Distortion Guitar", Pitches: stdGuitar},
		{Name: "Lead Vocals", Instrument: "FX 3 (crystal)"},                         // no strings: sung
		{Name: "Solo Violin", Instrument: "Violin", Pitches: []int{55, 62, 69, 76}}, // not a guitar's strings
	}
	got := difficulty.Analyze(sc, tracks)
	want := []difficulty.Part{
		{Role: difficulty.Drums, Tracks: []int{0}},
		{Role: difficulty.Bass, Tracks: []int{1}},
		{Role: difficulty.Rhythm, Tracks: []int{2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parts:\n got %+v\nwant %+v", got, want)
	}
}

// tuplets is a 4/4 bar of n:m tuplets of a note value with these notes.
func tuplets(n int, value int, notes ...score.Note) []score.Beat {
	dur := value * score.TupletBase(n) / n
	var out []score.Beat
	for t := 0; t+dur <= 4*q; t += dur {
		out = append(out, score.Beat{Start: t, Dur: dur, Tuplet: n, Notes: notes})
	}
	return out
}

// tagsOf is the tags of the part of that role, nil if there's no such part.
func tagsOf(t *testing.T, sc *score.Score, tracks []difficulty.TrackInfo, r difficulty.Role) []string {
	t.Helper()
	for _, p := range difficulty.Analyze(sc, tracks) {
		if p.Role == r {
			return p.Tags
		}
	}
	t.Fatalf("no %s part", r)
	return nil
}

var rhythmGuitar = []difficulty.TrackInfo{{Name: "Rhythm", Instrument: "Distortion Guitar", Pitches: stdGuitar}}

// riff is a rhythm guitar track of power chords, bar by bar: true for a bar of eighth
// triplets, false for straight eighths.
func riff(triplets ...bool) score.Track {
	var t score.Track
	for _, tri := range triplets {
		if tri {
			t.Bars = append(t.Bars, tuplets(3, e, powerChord...))
		} else {
			t.Bars = append(t.Bars, every(e, powerChord...))
		}
	}
	return t
}

func TestTagTriplets(t *testing.T) {
	const o, x = false, true
	tests := []struct {
		name string
		riff score.Track
		want bool
	}{
		{"straight", riff(o, o, o, o, o, o, o, o), false},
		{"triplets throughout", riff(x, x, x, x, x, x, x, x), true},
		{"every other bar", riff(x, o, x, o, x, o, x, o), true},
		{"one fill in sixteen bars", riff(o, o, o, o, o, o, o, x, o, o, o, o, o, o, o, o), false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(len(tt.riff.Bars), 120), Tracks: []score.Track{tt.riff}}
		if got := slices.Contains(tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm), "triplets"); got != tt.want {
			t.Errorf("%s: triplets %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTagTuplets(t *testing.T) {
	bars := func(fill []score.Beat) score.Track {
		return score.Track{Bars: [][]score.Beat{fill, every(e, powerChord...), fill, every(e, powerChord...)}}
	}
	tests := []struct {
		name              string
		track             score.Track
		triplets, tuplets bool
	}{
		{"quintuplets", bars(tuplets(5, s, powerChord...)), false, true},
		{"septuplets", bars(tuplets(7, s, powerChord...)), false, true},
		{"sextuplets are triplets", bars(tuplets(6, s, powerChord...)), true, false},
		{"twelve 32nd triplets are triplets", bars(tuplets(12, s/2, powerChord...)), true, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(4, 100), Tracks: []score.Track{tt.track}}
		tags := tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm)
		if got := slices.Contains(tags, "triplets"); got != tt.triplets {
			t.Errorf("%s: triplets %v, want %v", tt.name, got, tt.triplets)
		}
		if got := slices.Contains(tags, "tuplets"); got != tt.tuplets {
			t.Errorf("%s: tuplets %v, want %v", tt.name, got, tt.tuplets)
		}
	}
}

// meters is a score of one rhythm guitar playing eighth power chords through bars of these
// time signatures ("7/8").
func meters(sigs ...string) *score.Score {
	sc := &score.Score{Tracks: make([]score.Track, 1)}
	for _, sig := range sigs {
		var b score.Bar
		fmt.Sscanf(sig, "%d/%d", &b.Num, &b.Den)
		b.BPM = 120
		sc.Bars = append(sc.Bars, b)
		var beats []score.Beat
		for t := 0; t < b.Num*4*q/b.Den; t += e {
			beats = append(beats, score.Beat{Start: t, Dur: e, Notes: powerChord})
		}
		sc.Tracks[0].Bars = append(sc.Tracks[0].Bars, beats)
	}
	return sc
}

func TestTagMeters(t *testing.T) {
	repeat := func(n int, sigs ...string) []string {
		var out []string
		for range n {
			out = append(out, sigs...)
		}
		return out
	}
	tests := []struct {
		name          string
		sigs          []string
		odd, changing bool
	}{
		{"4/4", repeat(8, "4/4"), false, false},
		{"7/8", repeat(8, "7/8"), true, false},
		{"5/4", repeat(8, "5/4"), true, false},
		{"compound 6/8, 9/8, 12/8", repeat(3, "6/8", "6/8", "9/8", "12/8"), false, true},
		{"7/8 and 4/4 in turn", repeat(4, "7/8", "4/4"), true, true},
		{"one 2/4 bar", append(repeat(8, "4/4"), append([]string{"2/4"}, repeat(8, "4/4")...)...), false, false},
	}
	for _, tt := range tests {
		tags := tagsOf(t, meters(tt.sigs...), rhythmGuitar, difficulty.Rhythm)
		if got := slices.Contains(tags, "odd meter"); got != tt.odd {
			t.Errorf("%s: odd meter %v, want %v", tt.name, got, tt.odd)
		}
		if got := slices.Contains(tags, "meter changes"); got != tt.changing {
			t.Errorf("%s: meter changes %v, want %v", tt.name, got, tt.changing)
		}
	}
}

var (
	kick, snare, hat = score.Note{Fret: 36}, score.Note{Fret: 38}, score.Note{Fret: 42}
	drumKit          = []difficulty.TrackInfo{{Name: "Drums", Drums: true}}
	bassGuitar       = []difficulty.TrackInfo{{Name: "Bass", Instrument: "Electric Bass (pick)", Pitches: bass4}}
	leadGuitar       = []difficulty.TrackInfo{{Name: "Lead", Instrument: "Overdriven Guitar", Pitches: stdGuitar}}
)

// rockBeat is a bar of sixteenth hi-hats, kick on 1 and 3, snare on 2 and 4.
func rockBeat() []score.Beat {
	out := every(s, hat)
	for i := range out {
		switch out[i].Start {
		case 0, 2 * q:
			out[i].Notes = []score.Note{kick, hat}
		case q, 3 * q:
			out[i].Notes = []score.Note{snare, hat}
		}
	}
	return out
}

func TestTagFast(t *testing.T) {
	drums := func(bar []score.Beat) score.Track { return score.Track{Drums: true, Bars: track(8, bar).Bars} }
	tests := []struct {
		name  string
		info  []difficulty.TrackInfo
		role  difficulty.Role
		track score.Track
		bpm   float64
		want  bool
	}{
		{"rhythm: 16ths at 160", rhythmGuitar, difficulty.Rhythm, track(8, every(s, powerChord...)), 160, true},
		{"rhythm: 16ths at 100", rhythmGuitar, difficulty.Rhythm, track(8, every(s, powerChord...)), 100, false},
		{"rhythm: 8ths at 200", rhythmGuitar, difficulty.Rhythm, track(8, every(e, powerChord...)), 200, false},
		{"bass: 16ths at 160", bassGuitar, difficulty.Bass, track(8, every(s, score.Note{Fret: 0})), 160, true},
		{"lead: 16ths at 150", leadGuitar, difficulty.Lead, track(8, melody(score.Bend)), 150, true},
		{"lead: 16ths at 110", leadGuitar, difficulty.Lead, track(8, melody(score.Bend)), 110, false},
		{"drums: 16th rock beat at 120", drumKit, difficulty.Drums, drums(rockBeat()), 120, false},
		{"drums: 16th rock beat at 190", drumKit, difficulty.Drums, drums(rockBeat()), 190, true},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, tt.bpm), Tracks: []score.Track{tt.track}}
		if got := slices.Contains(tagsOf(t, sc, tt.info, tt.role), "fast"); got != tt.want {
			t.Errorf("%s: fast %v, want %v", tt.name, got, tt.want)
		}
	}
}

// at is a bar of power chords struck at these times, each lasting to the next.
func at(starts ...int) []score.Beat {
	var out []score.Beat
	for i, st := range starts {
		end := 4 * q
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		out = append(out, score.Beat{Start: st, Dur: end - st, Notes: powerChord})
	}
	return out
}

func TestTagSyncopated(t *testing.T) {
	tied := []score.Note{{String: 0, Fret: 0, Tie: true}, {String: 1, Fret: 2, Tie: true}}
	// Pushed: the last chord comes an eighth early and is held over the bar line.
	pushed := append([]score.Beat{{Start: 0, Dur: q, Notes: tied}}, at(q, 2*q, 3*q+e)[:]...)
	funk := rockBeat()
	for i := range funk {
		if funk[i].Start == q+e { // kick on the "and" of 2, nothing on 3
			funk[i].Notes = []score.Note{kick, hat}
		}
		if funk[i].Start == 2*q {
			funk[i].Notes = []score.Note{hat}
		}
	}
	tests := []struct {
		name  string
		info  []difficulty.TrackInfo
		role  difficulty.Role
		track score.Track
		want  bool
	}{
		{"straight eighths", rhythmGuitar, difficulty.Rhythm, track(8, every(e, powerChord...)), false},
		{"straight sixteenths", rhythmGuitar, difficulty.Rhythm, track(8, every(s, powerChord...)), false},
		{"3-3-2", rhythmGuitar, difficulty.Rhythm, track(8, at(0, 3*e, 6*e)), true},
		{"pushed over the bar line", rhythmGuitar, difficulty.Rhythm, track(8, pushed), true},
		{"chord on the and of 4, then on 1", rhythmGuitar, difficulty.Rhythm, track(8, at(0, q, 2*q, 3*q+e)), false},
		{"rock beat", drumKit, difficulty.Drums, score.Track{Drums: true, Bars: track(8, rockBeat()).Bars}, false},
		{"funk kick", drumKit, difficulty.Drums, score.Track{Drums: true, Bars: track(8, funk).Bars}, true},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, 100), Tracks: []score.Track{tt.track}}
		if got := slices.Contains(tagsOf(t, sc, tt.info, tt.role), "syncopated"); got != tt.want {
			t.Errorf("%s: syncopated %v, want %v", tt.name, got, tt.want)
		}
	}
}

// groove is a drum bar of three lines over a grid of steps: hi-hat, snare and kick, "x"
// for a hit: groove(s, "x.x.", "..x.", "x...").
func groove(step int, hats, snares, kicks string) []score.Beat {
	var out []score.Beat
	for i := 0; i < len(hats); i++ {
		var notes []score.Note
		for _, l := range []struct {
			line string
			note score.Note
		}{{kicks, kick}, {snares, snare}, {hats, hat}} {
			if i < len(l.line) && l.line[i] == 'x' {
				notes = append(notes, l.note)
			}
		}
		if len(notes) > 0 {
			out = append(out, score.Beat{Start: i * step, Dur: step, Notes: notes})
		}
	}
	return out
}

func TestTagDoubleKickAndBlastBeats(t *testing.T) {
	tests := []struct {
		name              string
		bar               []score.Beat
		bpm               float64
		doubleKick, blast bool
	}{
		{"rock beat", rockBeat(), 120, false, false},
		{"sixteenth kicks at 120", groove(s,
			"x.x.x.x.x.x.x.x.",
			"....x.......x...",
			"xxxxxxxxxxxxxxxx"), 120, true, false},
		{"sixteenth kicks at 100", groove(s,
			"x.x.x.x.x.x.x.x.",
			"....x.......x...",
			"xxxxxxxxxxxxxxxx"), 100, false, false},
		{"eighth kicks at 200", groove(e,
			"xxxxxxxx",
			"..x...x.",
			"xxxxxxxx"), 200, false, false},
		{"kick and snare in turn at 200", groove(s,
			"x.x.x.x.x.x.x.x.",
			".x.x.x.x.x.x.x.x",
			"x.x.x.x.x.x.x.x."), 200, false, true},
		{"kick and snare together at 220", groove(e,
			"xxxxxxxx",
			"xxxxxxxx",
			"xxxxxxxx"), 220, false, true},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, tt.bpm), Tracks: []score.Track{{Drums: true, Bars: track(8, tt.bar).Bars}}}
		tags := tagsOf(t, sc, drumKit, difficulty.Drums)
		if got := slices.Contains(tags, "double kick"); got != tt.doubleKick {
			t.Errorf("%s: double kick %v, want %v", tt.name, got, tt.doubleKick)
		}
		if got := slices.Contains(tags, "blast beats"); got != tt.blast {
			t.Errorf("%s: blast beats %v, want %v", tt.name, got, tt.blast)
		}
	}
}

func TestTagTechniques(t *testing.T) {
	ghost := melody(0)
	for i := range ghost {
		if i%4 == 2 {
			ghost[i].Notes = []score.Note{{String: ghost[i].Notes[0].String, Fret: ghost[i].Notes[0].Fret, Ghost: true}}
		}
	}
	slap := melody(0)
	slap[0].Fx = score.Slap
	tests := []struct {
		tag string
		bar []score.Beat
	}{
		{"bends", melody(score.Bend)},
		{"tapping", melody(score.Tap)},
		{"harmonics", melody(score.Harmonic)},
		{"slap", slap},
		{"legato", melody(score.Legato)},
		{"tremolo picking", melody(score.TremoloPicking)},
		{"ghost notes", ghost},
	}
	plain := melody(0)
	for _, tt := range tests {
		for _, c := range []struct {
			name string
			bars [][]score.Beat
			want bool
		}{
			{"in every other bar", [][]score.Beat{tt.bar, plain, tt.bar, plain, tt.bar, plain, tt.bar, plain}, true},
			{"once in sixteen bars", append([][]score.Beat{tt.bar}, track(15, plain).Bars...), false},
		} {
			sc := &score.Score{Bars: bars4(len(c.bars), 100), Tracks: []score.Track{{Bars: c.bars}}}
			if got := slices.Contains(tagsOf(t, sc, leadGuitar, difficulty.Lead), tt.tag); got != c.want {
				t.Errorf("%s %s: %v, want %v", tt.tag, c.name, got, c.want)
			}
		}
	}
}

// shape is a chord from frets per string, lowest first; -1 for a string not played.
func shape(frets ...int) []score.Note {
	var out []score.Note
	for str, f := range frets {
		if f >= 0 {
			out = append(out, score.Note{String: str, Fret: f})
		}
	}
	return out
}

func TestTagChordsAndStretches(t *testing.T) {
	openG := shape(3, 2, 0, 0, 0, 3)
	barreF := shape(1, 3, 3, 2, 1, 1)
	stretchy := shape(-1, 1, 3, 5, 6, -1) // a span of five frets
	e5, a5 := shape(0, 2, 2, -1, -1, -1), shape(-1, 0, 2, 2, -1, -1)
	lick := []score.Beat{ // in one position: 5 and 9 on the G string, 5 and 10 on the B string
		{Start: 0, Dur: s, Notes: shape(-1, -1, -1, 5, -1, -1)}, {Start: s, Dur: s, Notes: shape(-1, -1, -1, 9, -1, -1)},
		{Start: 2 * s, Dur: s, Notes: shape(-1, -1, -1, -1, 5, -1)}, {Start: 3 * s, Dur: s, Notes: shape(-1, -1, -1, -1, 10, -1)},
	}
	slide := []score.Beat{ // the same frets one string at a time: the hand moves
		{Start: 0, Dur: q, Notes: shape(-1, -1, -1, 5, -1, -1)}, {Start: q, Dur: q, Notes: shape(-1, -1, -1, 10, -1, -1)},
	}
	tests := []struct {
		name              string
		bar               []score.Beat
		chords, stretches bool
	}{
		{"power chords", append(every(q, e5...)[:2], at(2 * q)[0]), false, false},
		{"open chords", every(q, openG...), true, false},
		{"barre chords", every(q, barreF...), true, false},
		{"stretched chord", every(q, stretchy...), true, true},
		{"power chords moving", []score.Beat{{Start: 0, Dur: 2 * q, Notes: e5}, {Start: 2 * q, Dur: 2 * q, Notes: a5}}, false, false},
		{"stretched lick", lick, false, true},
		{"shift along a string", slide, false, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(4, 100), Tracks: []score.Track{track(4, tt.bar)}}
		info := []difficulty.TrackInfo{{Name: "Guitar", Instrument: "Acoustic Guitar (steel)", Pitches: stdGuitar}}
		var tags []string
		for _, p := range difficulty.Analyze(sc, info) {
			tags = append(tags, p.Tags...)
		}
		if got := slices.Contains(tags, "chords"); got != tt.chords {
			t.Errorf("%s: chords %v, want %v", tt.name, got, tt.chords)
		}
		if got := slices.Contains(tags, "stretches"); got != tt.stretches {
			t.Errorf("%s: stretches %v, want %v", tt.name, got, tt.stretches)
		}
	}
}

func TestTagSweeps(t *testing.T) {
	// An A minor arpeggio over five strings, up and down, a note per string.
	var arpeggio []score.Beat
	strs, frets := []int{1, 2, 3, 4, 5, 4, 3, 2}, []int{12, 14, 14, 13, 12, 13, 14, 14}
	for i := range 16 {
		arpeggio = append(arpeggio, score.Beat{Start: i * s, Dur: s, Notes: []score.Note{{String: strs[i%8], Fret: frets[i%8]}}})
	}
	tests := []struct {
		name string
		bar  []score.Beat
		bpm  float64
		want bool
	}{
		{"arpeggio at 150", arpeggio, 150, true},
		{"arpeggio at 90", arpeggio, 90, false},
		{"two-string line at 150", melody(0), 150, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, tt.bpm), Tracks: []score.Track{track(8, tt.bar)}}
		if got := slices.Contains(tagsOf(t, sc, leadGuitar, difficulty.Lead), "sweeps"); got != tt.want {
			t.Errorf("%s: sweeps %v, want %v", tt.name, got, tt.want)
		}
	}
}

// pattern is the n-th of many different bars: power chords on sixteenths picked by the
// bits of n, and on 1.
func pattern(n int) []score.Beat {
	var starts []int
	for i := range 16 {
		if i == 0 || (n+1)&(1<<(i-1)) != 0 {
			starts = append(starts, i*s)
		}
	}
	return at(starts...)
}

// transposed moves a bar up the neck.
func transposed(bar []score.Beat, frets int) []score.Beat {
	out := slices.Clone(bar)
	for i := range out {
		notes := slices.Clone(out[i].Notes)
		for j := range notes {
			notes[j].Fret += frets
		}
		out[i].Notes = notes
	}
	return out
}

func TestTagStructure(t *testing.T) {
	riff := pattern(5)
	varied := slices.Clone(pattern(5))
	varied[1].Notes = transposed(varied[1:2], 3)[0].Notes // one chord changed
	cycle := func(n, distinct int) [][]score.Beat {
		var out [][]score.Beat
		for i := range n {
			out = append(out, pattern(i%distinct))
		}
		return out
	}
	tests := []struct {
		name                  string
		bars                  [][]score.Beat
		heads                 []score.Bar // nil for 4/4 without repeats
		repetitive, manyParts bool
	}{
		{"one riff, 32 times", track(32, riff).Bars, nil, true, false},
		{"one riff, 4 bars repeated 8 times", track(4, riff).Bars,
			[]score.Bar{{Num: 4, Den: 4, RepeatOpen: true}, {Num: 4, Den: 4}, {Num: 4, Den: 4}, {Num: 4, Den: 4, Repeats: 7}}, true, false},
		{"a riff moved up and down", func() [][]score.Beat {
			var out [][]score.Beat
			for i := range 32 {
				out = append(out, transposed(riff, (i%4)*2))
			}
			return out
		}(), nil, true, false},
		{"a riff with small changes", func() [][]score.Beat {
			var out [][]score.Beat
			for i := range 32 {
				out = append(out, [][]score.Beat{riff, varied}[i%2])
			}
			return out
		}(), nil, true, false},
		{"8 bars over and over", cycle(32, 8), nil, false, false},
		{"40 different bars", cycle(40, 40), nil, false, true},
		{"8 bars repeated, but 4 written", cycle(4, 4),
			[]score.Bar{{Num: 4, Den: 4, RepeatOpen: true}, {Num: 4, Den: 4}, {Num: 4, Den: 4}, {Num: 4, Den: 4, Repeats: 1}}, false, false},
	}
	for _, tt := range tests {
		heads := tt.heads
		if heads == nil {
			heads = bars4(len(tt.bars), 120)
		}
		sc := &score.Score{Bars: heads, Tracks: []score.Track{{Bars: tt.bars}}}
		tags := tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm)
		if got := slices.Contains(tags, "repetitive"); got != tt.repetitive {
			t.Errorf("%s: repetitive %v, want %v", tt.name, got, tt.repetitive)
		}
		if got := slices.Contains(tags, "many parts"); got != tt.manyParts {
			t.Errorf("%s: many parts %v, want %v", tt.name, got, tt.manyParts)
		}
	}
}

func TestTagStructureDrumsIgnoresCymbals(t *testing.T) {
	ride, crash := score.Note{Fret: 51}, score.Note{Fret: 49}
	withCymbal := func(c score.Note, crashOn1 bool) []score.Beat {
		bar := rockBeat()
		for i := range bar {
			for j := range bar[i].Notes {
				if bar[i].Notes[j] == hat {
					bar[i].Notes = slices.Clone(bar[i].Notes)
					bar[i].Notes[j] = c
				}
			}
		}
		if crashOn1 {
			bar[0].Notes = []score.Note{kick, crash}
		}
		return bar
	}
	var bars [][]score.Beat
	for i := range 40 {
		bars = append(bars, withCymbal([]score.Note{hat, ride}[i/20], i%4 == 0))
	}
	sc := &score.Score{Bars: bars4(40, 120), Tracks: []score.Track{{Drums: true, Bars: bars}}}
	tags := tagsOf(t, sc, drumKit, difficulty.Drums)
	if !slices.Contains(tags, "repetitive") || slices.Contains(tags, "many parts") {
		t.Errorf("one beat, hi-hat or ride, crash every 4th bar: tags %v, want repetitive", tags)
	}
}

func TestTagStructureDrumFillsAreNoNewPart(t *testing.T) {
	toms := []score.Note{{Fret: 50}, {Fret: 48}, {Fret: 45}, {Fret: 43}, {Fret: 41}}
	fill := func(n int) []score.Beat { // the beat for two beats, then sixteenths on the toms
		bar := slices.Clone(rockBeat()[:8])
		for i := range 8 {
			bar = append(bar, score.Beat{Start: 2*q + i*s, Dur: s, Notes: []score.Note{toms[(n+i*(n%3+1))%len(toms)]}})
		}
		return bar
	}
	var bars [][]score.Beat
	for i := range 40 {
		if i%4 == 3 {
			bars = append(bars, fill(i/4))
		} else {
			bars = append(bars, rockBeat())
		}
	}
	sc := &score.Score{Bars: bars4(40, 120), Tracks: []score.Track{{Drums: true, Bars: bars}}}
	if tags := tagsOf(t, sc, drumKit, difficulty.Drums); !slices.Contains(tags, "repetitive") {
		t.Errorf("one beat with ten different fills: tags %v, want repetitive", tags)
	}
}

// grouped is n bars of 4/4 of power chords on every k-th sixteenth, counted on over the bar
// lines: a grouping of k that the bars don't divide shifts against them.
func grouped(n, k int) [][]score.Beat {
	out := make([][]score.Beat, n)
	for i := range n * 16 {
		if i%k == 0 {
			out[i/16] = append(out[i/16], score.Beat{Start: i % 16 * s, Dur: s, Notes: powerChord})
		}
	}
	return out
}

func TestTagPolyrhythm(t *testing.T) {
	drumsTrack := score.Track{Drums: true, Bars: track(8, rockBeat()).Bars}
	both := append(slices.Clone(drumKit), rhythmGuitar...)
	tests := []struct {
		name          string
		tracks        []score.Track
		info          []difficulty.TrackInfo
		drums, rhythm bool // a polyrhythm tag on that part
	}{
		// Triplets against a straight beat are just triplets.
		{"triplets over straight sixteenths", []score.Track{drumsTrack, track(8, tuplets(3, e, powerChord...))}, both, false, false},
		{"straight sixteenths over triplets", []score.Track{{Drums: true, Bars: track(8, tuplets(3, e, snare)).Bars}, track(8, every(s, powerChord...))}, both, false, false},
		{"triplets alone", []score.Track{track(8, tuplets(3, e, powerChord...))}, rhythmGuitar, false, false},
		{"groups of three sixteenths", []score.Track{{Bars: grouped(8, 3)}}, rhythmGuitar, false, true},
		{"groups of five sixteenths over a rock beat", []score.Track{drumsTrack, {Bars: grouped(8, 5)}}, both, false, true},
		{"groups of four", []score.Track{{Bars: grouped(8, 4)}}, rhythmGuitar, false, false},
		// One passage is enough: the band has to get it right.
		{"a passage of groups of three in a long song", []score.Track{{Bars: slices.Concat(
			track(24, every(e, powerChord...)).Bars, grouped(4, 3), track(24, every(e, powerChord...)).Bars)}}, rhythmGuitar, false, true},
		{"3-3-2 in every bar", []score.Track{track(8, at(0, 3*e, 6*e))}, rhythmGuitar, false, false},
		{"straight eighths with a rock beat", []score.Track{drumsTrack, track(8, every(e, powerChord...))}, both, false, false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(len(tt.tracks[len(tt.tracks)-1].Bars), 120), Tracks: tt.tracks}
		got := map[difficulty.Role]bool{}
		for _, p := range difficulty.Analyze(sc, tt.info) {
			got[p.Role] = slices.Contains(p.Tags, "polyrhythm")
		}
		if got[difficulty.Drums] != tt.drums || got[difficulty.Rhythm] != tt.rhythm {
			t.Errorf("%s: polyrhythm on drums %v, rhythm %v; want %v, %v", tt.name,
				got[difficulty.Drums], got[difficulty.Rhythm], tt.drums, tt.rhythm)
		}
	}
}

func TestTagEndurance(t *testing.T) {
	fast, slow := every(s, powerChord...), every(q, powerChord...)
	alternate := func(n, run int) [][]score.Beat { // runs of fast bars, then a slow bar
		var out [][]score.Beat
		for i := range n {
			out = append(out, [][]score.Beat{fast, slow}[min(i%(run+1)/run, 1)])
		}
		return out
	}
	tests := []struct {
		name string
		bars [][]score.Beat
		want bool
	}{
		{"32 fast bars in a row (48 s)", track(32, fast).Bars, true},
		{"fast in runs of 8 bars (12 s)", alternate(45, 8), false},
		{"16 fast bars (24 s), then slow", append(track(16, fast).Bars, track(16, slow).Bars...), false},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(len(tt.bars), 160), Tracks: []score.Track{{Bars: tt.bars}}}
		if got := slices.Contains(tagsOf(t, sc, rhythmGuitar, difficulty.Rhythm), "endurance"); got != tt.want {
			t.Errorf("%s: endurance %v, want %v", tt.name, got, tt.want)
		}
	}
}

// melody is a bar of single sixteenth notes up and down the B and high E strings
// around the 12th fret, with fx on every fourth note.
func melody(fx score.Fx) []score.Beat {
	var out []score.Beat
	frets := []int{12, 14, 15, 17, 15, 14, 12, 15}
	for i, t := 0, 0; t < 4*q; i, t = i+1, t+s {
		n := score.Note{String: 4 + i%2, Fret: frets[i%len(frets)]}
		if i%4 == 0 {
			n.Fx = fx
		}
		out = append(out, score.Beat{Start: t, Dur: s, Notes: []score.Note{n}})
	}
	return out
}

// guitarRoles is the roles each track plays in.
func guitarRoles(sc *score.Score, tracks []difficulty.TrackInfo) map[difficulty.Role][]int {
	out := map[difficulty.Role][]int{}
	for _, p := range difficulty.Analyze(sc, tracks) {
		out[p.Role] = p.Tracks
	}
	return out
}

func TestRhythmAndLeadByContent(t *testing.T) {
	gtr := func(name string) difficulty.TrackInfo {
		return difficulty.TrackInfo{Name: name, Instrument: "Distortion Guitar", Pitches: stdGuitar}
	}
	chug := every(s, score.Note{String: 0, Fret: 0, Fx: score.PalmMute}) // a palm-muted single-note riff
	riffThenSolo := score.Track{Bars: [][]score.Beat{
		every(e, powerChord...), every(e, powerChord...), chug, chug,
		melody(score.Bend), melody(score.Vibrato), melody(score.Legato), melody(0),
	}}
	tests := []struct {
		name   string
		tracks []score.Track
		info   []difficulty.TrackInfo
		want   map[difficulty.Role][]int
	}{
		{"one guitarist, riffs then a solo", []score.Track{riffThenSolo}, []difficulty.TrackInfo{gtr("Guitar")},
			map[difficulty.Role][]int{difficulty.Rhythm: {0}, difficulty.Lead: {0}}},
		{"rhythm and lead track", []score.Track{track(8, every(e, powerChord...)), track(8, melody(score.Bend))},
			[]difficulty.TrackInfo{gtr("Guitar 1"), gtr("Guitar 2")},
			map[difficulty.Role][]int{difficulty.Rhythm: {0}, difficulty.Lead: {1}}},
		{"single-note riff is rhythm", []score.Track{track(8, chug)}, []difficulty.TrackInfo{gtr("Guitar")},
			map[difficulty.Role][]int{difficulty.Rhythm: {0}}},
	}
	for _, tt := range tests {
		sc := &score.Score{Bars: bars4(8, 120), Tracks: tt.tracks}
		if got := guitarRoles(sc, tt.info); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}
