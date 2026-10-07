// Package score is the note data of a tab, as far as judging how hard it is to play
// needs it: bars with their time signatures and repeats, and per track the beats
// with their timing, notes and playing techniques. The parsers in package tab
// fill it; package difficulty reads it.
package score

// Quarter is the length of a quarter note in ticks, the unit of all times here.
const Quarter = 960

// Score is a song's notes.
type Score struct {
	Bars   []Bar
	Tracks []Track // in the file's track order
}

// Bar is a bar shared by all tracks: its time signature and where the song repeats.
type Bar struct {
	Num, Den   int     // time signature
	RepeatOpen bool    // a repeat starts here
	Repeats    int     // a repeat ends here and is played this many more times
	Alternate  int     // alternate endings this bar is played in, bit n-1 for ending n; 0 for always
	Marker     string  // section name ("Verse", "Chorus"), "" for none
	BPM        float64 // the tempo at the start of the bar, 0 if unknown
}

// Track is one track's beats, bar by bar; Bars has one entry per bar of the score.
type Track struct {
	Drums bool
	Bars  [][]Beat // the beats of all voices that have notes, sorted by start
}

// Beat is notes struck together. Rests are left out.
type Beat struct {
	Start  int // ticks from the start of the bar
	Dur    int // ticks, tuplets and dots applied
	Tuplet int // n of an n:m tuplet (3 for triplets), 0 for none
	Voice  int
	Fx     Fx // techniques of the whole beat
	Notes  []Note
}

// AllFx is the techniques of the beat and its notes.
func (b Beat) AllFx() Fx {
	fx := b.Fx
	for _, n := range b.Notes {
		fx |= n.Fx
	}
	return fx
}

// Note is a note of a beat.
type Note struct {
	String int // 0 = lowest string
	Fret   int // for drums the MIDI percussion note
	Tie    bool
	Dead   bool
	Ghost  bool
	Fx     Fx
}

// DurOf is the length of a note value: 0 a quarter, 1 an eighth, 2 a sixteenth, -1 a half, -2 a whole.
func DurOf(value int) int {
	d := 4 * Quarter
	for i := -2; i < value; i++ {
		d /= 2
	}
	return d
}

// TupletBase is the m of an n:m tuplet: how many plain notes n of them take the time of.
func TupletBase(n int) int {
	switch {
	case n == 3:
		return 2
	case n < 8:
		return 4
	case n < 16:
		return 8
	}
	return 16
}

// Fx is a set of playing techniques.
type Fx uint32

const (
	Bend Fx = 1 << iota
	Vibrato
	Legato // hammer-on or pull-off
	Slide
	PalmMute
	Staccato
	Harmonic
	Tap
	Slap // slap or pop
	TremoloPicking
	Trill
	Grace
	TremoloBar
	Accent
)
