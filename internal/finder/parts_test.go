package finder

import (
	"math"
	"reflect"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/tab"
)

func TestParseLevelRange(t *testing.T) {
	inf := math.Inf(1)
	for in, want := range map[string]Range{
		"3": {3, 3}, "2-4": {2, 4}, " 7 - ": {7, inf}, "-3": {0, 3}, "9-10": {9, 10},
	} {
		if got, err := ParseLevelRange(in); err != nil || got != want {
			t.Errorf("%q: %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "-", "x", "5-2", "easy"} {
		if _, err := ParseLevelRange(in); err == nil {
			t.Errorf("%q: no error", in)
		}
	}
}

// rated is a song with parts: role, score and tags.
func rated(title string, parts ...difficulty.Part) *tab.Song {
	return &tab.Song{Path: title + ".gp5", Format: tab.FormatGP5, Title: title, Artist: "Copper Wolves", Parts: parts}
}

func part(r difficulty.Role, score float64, tags ...string) difficulty.Part {
	return difficulty.Part{Role: r, Score: score, Tags: tags}
}

func titles(l *Library, idx []int) []string {
	out := []string{}
	for _, i := range idx {
		out = append(out, l.Entries[i].Song.Title)
	}
	return out
}

func TestSearchByParts(t *testing.T) {
	lib := New([]*tab.Song{
		rated("Anvil", part(difficulty.Drums, 2.2), part(difficulty.Rhythm, 3.6, "syncopated")),
		rated("Bellows", part(difficulty.Drums, 7.8, "double kick"), part(difficulty.Rhythm, 8.4, "fast", "triplets"), part(difficulty.Lead, 9.1)),
		rated("Cinder", part(difficulty.Drums, 4.4), part(difficulty.Rhythm, 4.5, "triplets")),
		rated("Damper", part(difficulty.Rhythm, 1.4)),                                       // no drums
		{Path: "Embers.tg", Format: tab.FormatTG, Title: "Embers", Artist: "Copper Wolves"}, // not rated
	})
	tests := []struct {
		name  string
		query Query
		want  []string
	}{
		{"no difficulty filter", Query{}, []string{"Anvil", "Bellows", "Cinder", "Damper", "Embers"}},
		{"rhythm 2-4", Query{Rhythm: PartQuery{Level: "2-4"}}, []string{"Anvil"}},
		{"rhythm 4.5 is level 5", Query{Rhythm: PartQuery{Level: "5"}}, []string{"Cinder"}},
		{"easy drums and rhythm: songs with drums", Query{Drums: PartQuery{Level: "-4"}, Rhythm: PartQuery{Level: "-5"}}, []string{"Anvil", "Cinder"}},
		{"no drums is no match for drums 3-", Query{Drums: PartQuery{Level: "3-"}}, []string{"Bellows", "Cinder"}},
		{"a range from 1 asks for the part too", Query{Lead: PartQuery{Level: "1-9"}}, []string{"Bellows"}},
		{"tag", Query{Rhythm: PartQuery{Tags: "triplets"}}, []string{"Bellows", "Cinder"}},
		{"tags, all of them", Query{Rhythm: PartQuery{Tags: "triplets, fast"}}, []string{"Bellows"}},
		{"tag and level", Query{Rhythm: PartQuery{Level: "-5", Tags: "triplets"}}, []string{"Cinder"}},
		{"tag of another part", Query{Drums: PartQuery{Tags: "triplets"}}, []string{}},
		{"invalid level: left out", Query{Rhythm: PartQuery{Level: "hard"}}, []string{"Anvil", "Bellows", "Cinder", "Damper", "Embers"}},
	}
	for _, tt := range tests {
		if got := titles(lib, lib.Search(tt.query).Matches); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
	if !lib.Search(Query{Rhythm: PartQuery{Level: "hard"}}).LevelInvalid[difficulty.Rhythm] {
		t.Error("invalid rhythm level not reported")
	}
}

func TestSearchSortsByDifficulty(t *testing.T) {
	lib := New([]*tab.Song{
		rated("Anvil", part(difficulty.Drums, 2.2), part(difficulty.Rhythm, 3.6)),
		rated("Bellows", part(difficulty.Drums, 7.8), part(difficulty.Rhythm, 8.4)),
		rated("Cinder", part(difficulty.Drums, 9.4), part(difficulty.Rhythm, 4.5)),
		{Path: "Embers.tg", Format: tab.FormatTG, Title: "Embers", Artist: "Copper Wolves"},
	})
	tests := []struct {
		name  string
		query Query
		want  []string
	}{
		{"hardest first, by the hardest part", Query{Sort: SortHardest}, []string{"Cinder", "Bellows", "Anvil", "Embers"}},
		{"easiest first", Query{Sort: SortEasiest}, []string{"Anvil", "Bellows", "Cinder", "Embers"}},
		{"by the parts filtered", Query{Sort: SortHardest, Rhythm: PartQuery{Level: "1-10"}}, []string{"Bellows", "Cinder", "Anvil"}},
	}
	for _, tt := range tests {
		if got := titles(lib, lib.Search(tt.query).Matches); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}
