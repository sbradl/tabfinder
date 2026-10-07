package tab

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"tabfinder/internal/score"
	"tabfinder/internal/tabfiles"
)

// readNotes writes data as a tab file and reads its notes.
func readNotes(t *testing.T, name string, data []byte) (*Song, *score.Score) {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return ReadNotes(p, root)
}

func eighth(frets ...int) []tabfiles.GPBeat { // on the low E string of a 6-string
	var out []tabfiles.GPBeat
	for _, f := range frets {
		out = append(out, tabfiles.GPBeat{Dur: 1, Notes: []tabfiles.GPNote{{String: 6, Fret: f}}})
	}
	return out
}

func TestReadNotesGP5Eighths(t *testing.T) {
	data := tabfiles.GP(tabfiles.GPSpec{
		Version: "5.10", Title: "Gravel Hymn", Artist: "Copper Wolves", Tempo: 120,
		Tracks: []tabfiles.GPTrack{guitar("Rhythm", 30, tabfiles.StdGuitar...)},
		Bars:   []tabfiles.GPBar{{Beats: [][]tabfiles.GPBeat{eighth(0, 3, 5, 0)}}},
	})
	s, sc := readNotes(t, "gravel.gp5", data)
	if s.Error != "" {
		t.Fatalf("error: %s", s.Error)
	}
	low := func(start, fret int) score.Beat {
		return score.Beat{Start: start, Dur: score.Quarter / 2, Notes: []score.Note{{String: 0, Fret: fret}}}
	}
	want := &score.Score{
		Bars:   []score.Bar{{Num: 4, Den: 4, BPM: 120}},
		Tracks: []score.Track{{Bars: [][]score.Beat{{low(0, 0), low(480, 3), low(960, 5), low(1440, 0)}}}},
	}
	if !reflect.DeepEqual(sc, want) {
		t.Errorf("notes:\n got %+v\nwant %+v", sc, want)
	}
}

func TestReadNotesBars(t *testing.T) {
	bars := []tabfiles.GPBar{
		{Num: 7, Den: 8, RepeatOpen: true, Marker: "Verse"},
		{Repeats: 3, Tempo: 150},
		{Alternate: 1},
		{Num: 4, Den: 4, Alternate: 2, Marker: "Outro"},
		{},
	}
	files := map[string][]byte{}
	for _, version := range []string{"3.00", "4.06", "5.00", "5.10"} {
		files["GP "+version] = tabfiles.GP(tabfiles.GPSpec{
			Version: version, Title: "Folded Map", Artist: "Copper Wolves", Tempo: 100,
			Tracks: []tabfiles.GPTrack{guitar("Gtr", 30, tabfiles.StdGuitar...)},
			Bars:   bars,
		})
	}
	gpifTempos := [][2]float64{{0, 100}, {1, 150}}
	gtr := "40 45 50 55 59 64"
	files["GP6"] = tabfiles.GPX(tabfiles.GPIF6Score("Folded Map", "Copper Wolves", "", gpifTempos, bars, tabfiles.GPIF6Track{Name: "Gtr", Instrument: "e-gtr", Pitches: gtr}), true)
	files["GP7"] = tabfiles.GP7(tabfiles.GPIFScore("Folded Map", "Copper Wolves", "", gpifTempos, bars, tabfiles.GPIFTrack{Name: "Gtr", Instrument: "Guitar", Pitches: gtr}))
	for version, data := range files {
		s, sc := readNotes(t, "folded.gp", data)
		if s.Error != "" {
			t.Fatalf("%s: error: %s", version, s.Error)
		}
		want := []score.Bar{
			{Num: 7, Den: 8, RepeatOpen: true, Marker: "Verse", BPM: 100},
			{Num: 7, Den: 8, Repeats: 3, BPM: 150},
			{Num: 7, Den: 8, Alternate: 1, BPM: 150},
			{Num: 4, Den: 4, Alternate: 2, Marker: "Outro", BPM: 150},
			{Num: 4, Den: 4, BPM: 150},
		}
		if !reflect.DeepEqual(sc.Bars, want) {
			t.Errorf("%s: bars:\n got %+v\nwant %+v", version, sc.Bars, want)
		}
	}
}

// knotwork builds a file of the format ("3.00"-"5.10" for Guitar Pro 3-5, "gp6", "gp7") with
// a bar of this beat, a quarter, and then an eighth on the low E, which shows the beat was
// read to its end.
func knotwork(format string, b tabfiles.GPBeat) []byte {
	b.Dur = 0
	bars := []tabfiles.GPBar{{Beats: [][]tabfiles.GPBeat{{b, eighth(5)[0]}}}}
	const gtr = "40 45 50 55 59 64"
	switch format {
	case "gp6":
		return tabfiles.GPX(tabfiles.GPIF6Score("Knotwork", "Copper Wolves", "", [][2]float64{{0, 100}}, bars, tabfiles.GPIF6Track{Name: "Gtr", Instrument: "e-gtr", Pitches: gtr}), true)
	case "gp7":
		return tabfiles.GP7(tabfiles.GPIFScore("Knotwork", "Copper Wolves", "", [][2]float64{{0, 100}}, bars, tabfiles.GPIFTrack{Name: "Gtr", Instrument: "Guitar", Pitches: gtr}))
	}
	return tabfiles.GP(tabfiles.GPSpec{
		Version: format, Title: "Knotwork", Artist: "Copper Wolves", Tempo: 100,
		Tracks: []tabfiles.GPTrack{guitar("Gtr", 30, tabfiles.StdGuitar...)},
		Bars:   bars,
	})
}

// TestReadNotesTechniques checks the techniques of a beat and its notes together: formats
// differ in which they belong to (tremolo picking is a note's in GP5, a beat's in GP7).
func TestReadNotesTechniques(t *testing.T) {
	gp3, gp45, gpif := []string{"3.00"}, []string{"4.06", "5.00", "5.10"}, []string{"gp6", "gp7"}
	all := slices.Concat(gp3, gp45, gpif)
	noGP3 := slices.Concat(gp45, gpif)
	type kind struct{ tie, dead, ghost bool }
	tests := []struct {
		name    string
		formats []string
		beat    tabfiles.GPBeat
		fx      score.Fx
		kind    kind
	}{
		{"plain", all, tabfiles.GPBeat{}, 0, kind{}},
		{"tie", all, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Tie: true}}}, 0, kind{tie: true}},
		{"dead", all, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Dead: true}}}, 0, kind{dead: true}},
		{"ghost", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Ghost: true}}}, 0, kind{ghost: true}},
		{"accent", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Accent: true}}}, score.Accent, kind{}},
		{"bend", all, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Bend: true}}}, score.Bend, kind{}},
		{"legato", all, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Legato: true}}}, score.Legato, kind{}},
		{"slide", all, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Slide: true}}}, score.Slide, kind{}},
		{"grace", slices.Concat(gp3, gp45), tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Grace: true}}}, score.Grace, kind{}},
		{"palm mute", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{PalmMute: true}}}, score.PalmMute, kind{}},
		{"staccato", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Staccato: true}}}, score.Staccato, kind{}},
		{"harmonic", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Harmonic: true}}}, score.Harmonic, kind{}},
		{"trill", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Trill: true}}}, score.Trill, kind{}},
		{"tremolo picking", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Tremolo: true}}}, score.TremoloPicking, kind{}},
		{"vibrato", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Vibrato: true}}}, score.Vibrato, kind{}},
		{"several", noGP3, tabfiles.GPBeat{Notes: []tabfiles.GPNote{{Bend: true, Vibrato: true, PalmMute: true}}}, score.Bend | score.Vibrato | score.PalmMute, kind{}},
		{"tapping", all, tabfiles.GPBeat{Tap: true}, score.Tap, kind{}},
		{"slap", all, tabfiles.GPBeat{Slap: true}, score.Slap, kind{}},
		{"beat vibrato", slices.Concat(gp3, gp45), tabfiles.GPBeat{Vibrato: true}, score.Vibrato, kind{}},
	}
	for _, tt := range tests {
		for _, format := range tt.formats {
			b := tt.beat
			var note tabfiles.GPNote
			if len(b.Notes) > 0 {
				note = b.Notes[0]
			}
			note.String, note.Fret = 3, 7
			b.Notes = []tabfiles.GPNote{note}
			s, sc := readNotes(t, "knotwork.gp", knotwork(format, b))
			if s.Error != "" || sc == nil {
				t.Errorf("%s, %s: error %q, notes %v", tt.name, format, s.Error, sc)
				continue
			}
			beats := sc.Tracks[0].Bars[0]
			next := score.Beat{Start: score.Quarter, Dur: score.Quarter / 2, Notes: []score.Note{{String: 0, Fret: 5}}}
			if len(beats) != 2 || !reflect.DeepEqual(beats[1], next) {
				t.Errorf("%s, %s: beats %+v", tt.name, format, beats)
				continue
			}
			got := beats[0]
			n := got.Notes[0]
			if got.Start != 0 || got.Dur != score.Quarter || len(got.Notes) != 1 || n.String != 3 || n.Fret != 7 {
				t.Errorf("%s, %s: beat %+v", tt.name, format, got)
			}
			if fx := got.AllFx(); fx != tt.fx {
				t.Errorf("%s, %s: techniques %b, want %b", tt.name, format, fx, tt.fx)
			}
			if k := (kind{n.Tie, n.Dead, n.Ghost}); k != tt.kind {
				t.Errorf("%s, %s: note %+v, want %+v", tt.name, format, k, tt.kind)
			}
		}
	}
}

func TestReadNotesVoicesAndDrums(t *testing.T) {
	kick := func(dur int) tabfiles.GPBeat {
		return tabfiles.GPBeat{Dur: dur, Notes: []tabfiles.GPNote{{String: 6, Fret: 36}}}
	}
	hat := func(dur int) tabfiles.GPBeat {
		return tabfiles.GPBeat{Dur: dur, Notes: []tabfiles.GPNote{{String: 1, Fret: 42}}}
	}
	data := tabfiles.GP(tabfiles.GPSpec{
		Version: "5.10", Title: "Two Hands", Artist: "Copper Wolves", Tempo: 160,
		Tracks: []tabfiles.GPTrack{drums, guitar("Gtr", 30, tabfiles.StdGuitar...)},
		Bars: []tabfiles.GPBar{{
			Beats:  [][]tabfiles.GPBeat{{hat(1), hat(1), hat(1), hat(1), hat(1), hat(1), hat(1), hat(1)}, nil},
			Voice2: [][]tabfiles.GPBeat{{kick(-1), {Dur: 0, Rest: true}, kick(0)}},
		}},
	})
	s, sc := readNotes(t, "hands.gp5", data)
	if s.Error != "" {
		t.Fatalf("error: %s", s.Error)
	}
	var want []score.Beat
	for i := range 8 {
		want = append(want, score.Beat{Start: i * 480, Dur: 480, Notes: []score.Note{{String: 5, Fret: 42}}})
	}
	kickAt := func(start, dur int) score.Beat {
		return score.Beat{Start: start, Dur: dur, Voice: 1, Notes: []score.Note{{String: 0, Fret: 36}}}
	}
	want = slices.Insert(want, 1, kickAt(0, 1920)) // after the hi-hat at the same time: voice order
	want = slices.Insert(want, 8, kickAt(2880, 960))
	if !sc.Tracks[0].Drums || sc.Tracks[1].Drums {
		t.Errorf("drums: %v, %v", sc.Tracks[0].Drums, sc.Tracks[1].Drums)
	}
	if got := sc.Tracks[0].Bars[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("drum beats:\n got %+v\nwant %+v", got, want)
	}
	if got := sc.Tracks[1].Bars; len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("empty guitar bar: %+v", got)
	}
}

func TestReadNotesTruncatedKeepsWholeBars(t *testing.T) {
	bar := tabfiles.GPBar{Beats: [][]tabfiles.GPBeat{eighth(1, 2, 3, 4, 5, 6, 7, 8), eighth(9, 9, 9, 9, 9, 9, 9, 9)}}
	spec := tabfiles.GPSpec{
		Version: "4.06", Title: "Frayed", Artist: "Copper Wolves", Tempo: 100,
		Tracks: []tabfiles.GPTrack{guitar("A", 30, tabfiles.StdGuitar...), guitar("B", 30, tabfiles.StdGuitar...)},
		Bars:   []tabfiles.GPBar{bar, bar, bar},
	}
	full := tabfiles.GP(spec)
	spec.Bars = spec.Bars[:2]
	twoBars := len(tabfiles.GP(spec))
	// Cut in the third bar's second track: the first track has three bars, the second two.
	s, sc := readNotes(t, "frayed.gp4", full[:twoBars+(len(full)-twoBars)*3/4])
	if s.Error == "" {
		t.Fatal("no error for a cut file")
	}
	if sc == nil || len(sc.Bars) != 2 || len(sc.Tracks[0].Bars) != 2 || len(sc.Tracks[1].Bars) != 2 {
		t.Fatalf("want 2 whole bars, got %+v", sc)
	}
	if got := len(sc.Tracks[0].Bars[1]); got != 8 {
		t.Errorf("second bar has %d beats", got)
	}
}

func TestReadNotesFormatsWithout(t *testing.T) {
	album := "Dry Wells"
	for name, data := range map[string][]byte{
		"tg":  tabfiles.TG1("Sand Clock", "Copper Wolves", album),
		"ptb": tabfiles.PTB(4, "Sand Clock", "Copper Wolves", &album),
		"bad": []byte("not a tab"),
	} {
		if _, sc := readNotes(t, "sand."+name, data); sc != nil {
			t.Errorf("%s: notes %+v", name, sc)
		}
	}
}

// gpAndGPIF builds the same song as GP5, GP6 and GP7 files, by file name.
func gpAndGPIF(title string, tempo int, bars []tabfiles.GPBar, tracks ...tabfiles.GPTrack) map[string][]byte {
	var gpif []tabfiles.GPIFTrack
	var gpif6 []tabfiles.GPIF6Track
	for _, t := range tracks {
		var pitches []string
		for _, p := range slices.Backward(t.Strings) {
			pitches = append(pitches, strconv.Itoa(p))
		}
		kind, ch := "electricGuitar", 0
		if t.Drums {
			kind, ch, pitches = "drumKit", 9, nil
		}
		gpif = append(gpif, tabfiles.GPIFTrack{Name: t.Name, Instrument: "Guitar", Kind: kind, Pitches: strings.Join(pitches, " ")})
		gpif6 = append(gpif6, tabfiles.GPIF6Track{Name: t.Name, Instrument: "e-gtr", Program: t.Program, Channel: ch, Percussion: t.Drums, Pitches: strings.Join(pitches, " ")})
	}
	tempos := [][2]float64{{0, float64(tempo)}}
	return map[string][]byte{
		"song.gp5": tabfiles.GP(tabfiles.GPSpec{Version: "5.10", Title: title, Artist: "Copper Wolves", Tempo: tempo, Tracks: tracks, Bars: bars}),
		"song.gpx": tabfiles.GPX(tabfiles.GPIF6Score(title, "Copper Wolves", "", tempos, bars, gpif6...), true),
		"song.gp":  tabfiles.GP7(tabfiles.GPIFScore(title, "Copper Wolves", "", tempos, bars, gpif...)),
	}
}

func TestReadNotesGPIFDrums(t *testing.T) {
	drumBeat := func(notes ...tabfiles.GPNote) tabfiles.GPBeat { return tabfiles.GPBeat{Dur: 1, Notes: notes} }
	wantBeat := func(start int, midi ...int) score.Beat {
		b := score.Beat{Start: start, Dur: 480}
		for _, m := range midi {
			b.Notes = append(b.Notes, score.Note{Fret: m})
		}
		return b
	}
	// GP6 names the drum (element) and how it's struck (variation).
	gp6 := tabfiles.GPX(tabfiles.GPIF6Score("Pulse", "Copper Wolves", "", [][2]float64{{0, 120}},
		[]tabfiles.GPBar{{Beats: [][]tabfiles.GPBeat{{
			drumBeat(tabfiles.GPNote{Element: 0}, tabfiles.GPNote{Element: 10}),               // kick + closed hi-hat
			drumBeat(tabfiles.GPNote{Element: 1}, tabfiles.GPNote{Element: 10, Variation: 2}), // snare + open hi-hat
			drumBeat(tabfiles.GPNote{Element: 1, Variation: 2}),                               // side stick
			drumBeat(tabfiles.GPNote{Element: 5}, tabfiles.GPNote{Element: 9}),                // lowest + highest tom
			drumBeat(tabfiles.GPNote{Element: 11}, tabfiles.GPNote{Element: 13}),              // pedal hi-hat + crash
			drumBeat(tabfiles.GPNote{Element: 15}, tabfiles.GPNote{Element: 15, Variation: 2}),
			drumBeat(tabfiles.GPNote{Element: 16}, tabfiles.GPNote{Element: 14}), // china + splash
		}}}},
		tabfiles.GPIF6Track{Name: "Drums", Instrument: "drumkit", Channel: 9, Percussion: true}), true)
	_, sc := readNotes(t, "pulse.gpx", gp6)
	want := []score.Beat{
		wantBeat(0, 36, 42), wantBeat(480, 38, 46), wantBeat(960, 37), wantBeat(1440, 41, 50),
		wantBeat(1920, 44, 49), wantBeat(2400, 51, 53), wantBeat(2880, 52, 55),
	}
	if sc == nil || !sc.Tracks[0].Drums || !reflect.DeepEqual(sc.Tracks[0].Bars[0], want) {
		t.Errorf("GP6 drums:\n got %+v\nwant %+v", sc, want)
	}

	// GP7 has the MIDI note.
	gp7 := tabfiles.GP7(tabfiles.GPIFScore("Pulse", "Copper Wolves", "", [][2]float64{{0, 120}},
		[]tabfiles.GPBar{{Beats: [][]tabfiles.GPBeat{{drumBeat(tabfiles.GPNote{Fret: 36}, tabfiles.GPNote{Fret: 42})}}}},
		tabfiles.GPIFTrack{Name: "Drums", Instrument: "Drums", Kind: "drumKit"}))
	_, sc = readNotes(t, "pulse.gp", gp7)
	if want := []score.Beat{wantBeat(0, 36, 42)}; sc == nil || !sc.Tracks[0].Drums || !reflect.DeepEqual(sc.Tracks[0].Bars[0], want) {
		t.Errorf("GP7 drums:\n got %+v\nwant %+v", sc, want)
	}
}

func TestReadNotesGPIFGraceBeatTakesNoTime(t *testing.T) {
	grace := tabfiles.GPBeat{Dur: 2, Notes: []tabfiles.GPNote{{String: 3, Fret: 5, Grace: true}}}
	bars := []tabfiles.GPBar{{Beats: [][]tabfiles.GPBeat{{grace, {Dur: 0, Notes: []tabfiles.GPNote{{String: 3, Fret: 7}}}, eighth(5)[0]}}}}
	for name, data := range map[string][]byte{
		"gp6": tabfiles.GPX(tabfiles.GPIF6Score("Flick", "Copper Wolves", "", nil, bars, tabfiles.GPIF6Track{Name: "Gtr", Instrument: "e-gtr", Pitches: "40 45 50 55 59 64"}), true),
		"gp7": tabfiles.GP7(tabfiles.GPIFScore("Flick", "Copper Wolves", "", nil, bars, tabfiles.GPIFTrack{Name: "Gtr", Instrument: "Guitar", Pitches: "40 45 50 55 59 64"})),
	} {
		_, sc := readNotes(t, "flick."+name, data)
		want := []score.Beat{
			{Start: 0, Dur: 240, Notes: []score.Note{{String: 3, Fret: 5, Fx: score.Grace}}},
			{Start: 0, Dur: 960, Notes: []score.Note{{String: 3, Fret: 7}}},
			{Start: 960, Dur: 480, Notes: []score.Note{{String: 0, Fret: 5}}},
		}
		if sc == nil || !reflect.DeepEqual(sc.Tracks[0].Bars[0], want) {
			t.Errorf("%s:\n got %+v\nwant %+v", name, sc, want)
		}
	}
}

func TestReadNotesTiming(t *testing.T) {
	a := []tabfiles.GPNote{{String: 5, Fret: 2}}
	chord := []tabfiles.GPNote{{String: 6, Fret: 0}, {String: 5, Fret: 2}}
	files := gpAndGPIF("Lopsided", 90,
		[]tabfiles.GPBar{{
			Beats: [][]tabfiles.GPBeat{{
				{Dur: 0, Dotted: true, Notes: a},
				{Dur: 1, Notes: chord},
				{Dur: 1, Tuplet: 3, Notes: a}, {Dur: 1, Tuplet: 3, Notes: a}, {Dur: 1, Tuplet: 3, Notes: a},
				{Dur: 0, Rest: true},
			}},
			Voice2: [][]tabfiles.GPBeat{{{Dur: -1, Rest: true}, {Dur: 2, Tuplet: 5, Notes: a}}},
		}},
		guitar("Gtr", 30, tabfiles.StdGuitar...))
	note := []score.Note{{String: 1, Fret: 2}}
	want := []score.Beat{
		{Start: 0, Dur: 1440, Notes: note},
		{Start: 1440, Dur: 480, Notes: []score.Note{{String: 0, Fret: 0}, {String: 1, Fret: 2}}},
		{Start: 1920, Dur: 320, Tuplet: 3, Notes: note},
		{Start: 1920, Dur: 192, Tuplet: 5, Voice: 1, Notes: note},
		{Start: 2240, Dur: 320, Tuplet: 3, Notes: note},
		{Start: 2560, Dur: 320, Tuplet: 3, Notes: note},
	}
	for name, data := range files {
		s, sc := readNotes(t, name, data)
		if s.Error != "" || sc == nil {
			t.Errorf("%s: error %q, notes %v", name, s.Error, sc)
			continue
		}
		if got := sc.Tracks[0].Bars[0]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: beats:\n got %+v\nwant %+v", name, got, want)
		}
	}
}
