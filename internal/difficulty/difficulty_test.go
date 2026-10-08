package difficulty_test

import (
	"reflect"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/score"
)

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
	if got, want := tracksByRole(sc, tracks), map[difficulty.Role][]int{
		difficulty.Drums:  {0},
		difficulty.Bass:   {1},
		difficulty.Rhythm: {2},
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("tracks by role: %v, want %v", got, want)
	}
}

// tracksByRole is the tracks each role plays on.
func tracksByRole(sc *score.Score, tracks []difficulty.TrackInfo) map[difficulty.Role][]int {
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
		if got := tracksByRole(sc, tt.info); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}
