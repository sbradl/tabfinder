package difficulty_test

import (
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
