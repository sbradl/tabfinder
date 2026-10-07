package tab

import (
	"slices"
	"strings"
)

// Tuning is how a track's strings are tuned: its pitches and the name the tuning goes by.
type Tuning struct {
	Name    string // "E Standard", "Drop C" or Custom
	Pitches []int  // MIDI notes, lowest string first
}

// Custom names a tuning that is neither a standard nor a drop tuning.
const Custom = "Custom"

var noteNames = [12]string{"C", "C#", "D", "Eb", "E", "F", "F#", "G", "Ab", "A", "Bb", "B"}

func noteName(midi int) string { return noteNames[(midi%12+12)%12] }

// Standard tuning intervals (semitones above the lowest string) by string count.
var standardIntervals = map[int][]int{
	4: {0, 5, 10, 15},             // bass
	5: {0, 5, 10, 15, 20},         // 5-string bass
	6: {0, 5, 10, 15, 19, 24},     // guitar
	7: {0, 5, 10, 15, 20, 24, 29}, // 7-string guitar
}

// TuningOf names the tuning of pitches (lowest string first). No pitches give the zero Tuning.
func TuningOf(p []int) Tuning {
	if len(p) == 0 {
		return Tuning{}
	}
	t := Tuning{Name: Custom, Pitches: p}
	if std, ok := standardIntervals[len(p)]; ok {
		rel := make([]int, len(p))
		for i, n := range p {
			rel[i] = n - p[0]
		}
		// A drop tuning is standard with only the lowest string 2 semitones down.
		dropped := slices.Clone(rel)
		for i := 1; i < len(dropped); i++ {
			dropped[i] -= 2
		}
		switch {
		case slices.Equal(rel, std):
			t.Name = noteName(p[0]) + " Standard"
		case slices.Equal(dropped, std):
			t.Name = "Drop " + noteName(p[0])
		}
	}
	return t
}

// Strings is the number of strings.
func (t Tuning) Strings() int { return len(t.Pitches) }

// Notes spells the pitches, lowest string first: "C G C F A D".
func (t Tuning) Notes() string {
	notes := make([]string, len(t.Pitches))
	for i, n := range t.Pitches {
		notes[i] = noteName(n)
	}
	return strings.Join(notes, " ")
}

// String is the name with the notes, "Drop C (C G C F A D)"; empty for no strings.
func (t Tuning) String() string {
	if len(t.Pitches) == 0 {
		return ""
	}
	return t.Name + " (" + t.Notes() + ")"
}
