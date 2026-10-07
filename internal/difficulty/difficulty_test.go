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
		},
	}
	tracks := []difficulty.TrackInfo{
		{Name: "Drums", Instrument: "Drums", Drums: true},
		{Name: "Bass", Instrument: "Electric Bass (finger)", Pitches: bass4},
		{Name: "Guitar", Instrument: "Distortion Guitar", Pitches: stdGuitar},
		{Name: "Woodwind", Instrument: "Oboe", Pitches: stdGuitar},
		{Name: "Guitar 2", Instrument: "Distortion Guitar", Pitches: stdGuitar},
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
