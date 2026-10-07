package finder

import (
	"slices"
	"testing"

	"tabfinder/internal/tab"
)

var (
	dropC6 = tab.Track{Pitches: []int{36, 43, 48, 53, 57, 62}}
	dropC4 = tab.Track{Pitches: []int{24, 31, 36, 41}}
	eStd6  = tab.Track{Pitches: []int{40, 45, 50, 55, 59, 64}}
	bStd7  = tab.Track{Pitches: []int{35, 40, 45, 50, 55, 59, 64}}
	custom = tab.Track{Pitches: []int{35, 40, 45, 50, 55, 59}}
)

func library() *Library {
	return New([]*tab.Song{
		{Path: "s/1.gp5", Artist: "Soilbed Quartet", Title: "Brass Kettle", Tracks: []tab.Track{dropC6, dropC4, {Drums: true}}, Tempos: []tab.Tempo{{Bar: 1, BPM: 190}, {Bar: 91, BPM: 145}, {Bar: 111, BPM: 190}}},
		{Path: "a/1.gp5", Artist: "Amber Marsh", Title: "First Frost", Tracks: []tab.Track{eStd6, custom}},
		{Path: "i/1.gp5", Artist: "Inkwell Flamingos", Title: "Only for the Brave", Tracks: []tab.Track{bStd7}},
		{Path: "i/2.gp5", Artist: "INKWELL FLAMINGOS", Title: "Fog Connected", Tracks: []tab.Track{dropC6}},
		{Path: "i/3.gp5", Artist: "Inkwell Flamingos", Title: "Mirage", Tracks: []tab.Track{dropC6}},
	})
}

func TestEntries(t *testing.T) {
	l := library()
	var titles []string
	for _, e := range l.Entries {
		titles = append(titles, e.Song.Title)
	}
	if want := []string{"First Frost", "Fog Connected", "Mirage", "Only for the Brave", "Brass Kettle"}; !slices.Equal(titles, want) {
		t.Errorf("order = %q, want %q", titles, want)
	}
	soilbed_quartet := l.Entries[4]
	if want := []Tuning{{6, "Drop C", "C G C F A D"}, {4, "Drop C", "C G C F"}}; !slices.Equal(soilbed_quartet.Tunings, want) {
		t.Errorf("tunings = %v, want %v", soilbed_quartet.Tunings, want)
	}
	if got := l.Entries[0].Tunings[1].Label(); got != "B E A D G B" {
		t.Errorf("custom label = %q", got)
	}
}

func TestArtists(t *testing.T) {
	l := library()
	if want := []string{"Amber Marsh", "Inkwell Flamingos", "Soilbed Quartet"}; !slices.Equal(l.Artists, want) {
		t.Errorf("artists = %q, want %q", l.Artists, want)
	}
	if got, want := l.SuggestArtists("am"), []string{"Amber Marsh", "Inkwell Flamingos"}; !slices.Equal(got, want) {
		t.Errorf("SuggestArtists(am) = %q, want %q (prefix first)", got, want)
	}
	if got := l.SuggestArtists("inkwell flamingos"); len(got) != 0 {
		t.Errorf("SuggestArtists(exact) = %q, want none", got)
	}
}

func TestSearch(t *testing.T) {
	l := library()
	r := l.Search(Query{})
	if len(r.Matches) != 5 {
		t.Errorf("no filter: %d matches", len(r.Matches))
	}
	// Guitars by string count, then bass; most common first within a group.
	want := []Tuning{{6, "Drop C", "C G C F A D"}, {6, "Custom", "B E A D G B"}, {6, "E Standard", "E A D G B E"}, {7, "B Standard", "B E A D G B E"}, {4, "Drop C", "C G C F"}}
	if !slices.Equal(r.Tunings, want) {
		t.Errorf("tunings = %v, want %v", r.Tunings, want)
	}

	// The artist trims the tuning suggestions; the tuning itself doesn't.
	r = l.Search(Query{Artist: "inkwell flamingos", Tuning: "drop c", Strings: 6})
	if !slices.Equal(r.Matches, []int{1, 2}) {
		t.Errorf("matches = %v", r.Matches)
	}
	if want := []Tuning{{6, "Drop C", "C G C F A D"}, {7, "B Standard", "B E A D G B E"}}; !slices.Equal(r.Tunings, want) {
		t.Errorf("trimmed tunings = %v, want %v", r.Tunings, want)
	}

	if r := l.Search(Query{BPM: "fast"}); !r.BPMInvalid || len(r.Matches) != 5 {
		t.Errorf("invalid bpm: %+v", r)
	}
	if r := l.Search(Query{BPM: "140-150"}); !slices.Equal(r.Matches, []int{4}) {
		t.Errorf("bpm 140-150: %v", r.Matches)
	}
	if got := SuggestTunings(want, "b e a"); !slices.Equal(got, []Tuning{want[1], want[3]}) {
		t.Errorf("SuggestTunings by notes = %v", got)
	}
}
