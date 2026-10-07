package tab

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var noteNames = [12]string{"C", "C#", "D", "Eb", "E", "F", "F#", "G", "Ab", "A", "Bb", "B"}

func noteName(midi int) string { return noteNames[(midi%12+12)%12] }

// Standard tuning intervals (semitones above the lowest string) by string count.
var standardIntervals = map[int][]int{
	4: {0, 5, 10, 15},             // bass
	5: {0, 5, 10, 15, 20},         // 5-string bass
	6: {0, 5, 10, 15, 19, 24},     // guitar
	7: {0, 5, 10, 15, 20, 24, 29}, // 7-string guitar
}

// tuningName labels pitches (lowest string first), e.g. "Drop C (C G C F A D)".
func tuningName(p []int) string {
	if len(p) == 0 {
		return ""
	}
	notes := make([]string, len(p))
	for i, n := range p {
		notes[i] = noteName(n)
	}
	label := "Custom"
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
			label = noteName(p[0]) + " Standard"
		case slices.Equal(dropped, std):
			label = "Drop " + noteName(p[0])
		}
	}
	return label + " (" + strings.Join(notes, " ") + ")"
}

var (
	reVersionTag = regexp.MustCompile(`(?i)\s*\((ver ?\d+[^)]*|\d+|pro|complete)\)`)
	reVerSuffix  = regexp.MustCompile(`(?i)\s+(ver\s?\d+|v\d+)\b`)
	reIDSuffix   = regexp.MustCompile(`\s+-\s+\d+$`)
	// JunkBracket matches "(Tabbed By X - www.example.org ...)", "[by x@y.com]" and the like.
	JunkBracket = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*(tabbed|www\.|@|\bby\b)[^\)\]]*([\)\]]|$)`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// titleFromFilename derives a song title from names like
// "Argyle Moth - Nectar (ver 3 by X).gp5" or "as_lanterns_fade_the_ferry.gp5".
func titleFromFilename(name string, artists ...string) string {
	for _, ext := range []string{".crdownload", ".zip"} {
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.TrimPrefix(name, "www-tablatures-tk @ ")
	name = strings.ReplaceAll(name, "_", " ")
	name = JunkBracket.ReplaceAllString(name, "")
	name = reVersionTag.ReplaceAllString(name, "")
	name = reVerSuffix.ReplaceAllString(name, "")
	name = reIDSuffix.ReplaceAllString(name, "")
	name = strings.TrimSpace(reSpaces.ReplaceAllString(name, " "))

	for _, a := range artists {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		for _, sep := range []string{" - ", "- ", " -", " "} {
			if rest, ok := cutPrefixFold(name, a+sep); ok {
				name = strings.TrimSpace(rest)
				break
			}
		}
	}
	if i := strings.Index(name, " - "); i > 0 {
		name = strings.TrimSpace(name[i+3:]) // "Other Artist - Title"
	}
	if r, _ := utf8.DecodeRuneInString(name); unicode.IsLower(r) {
		name = titleCase(name) // snake_case file names
	}
	return name
}

// cutPrefixFold is strings.CutPrefix ignoring case. It matches rune by rune on
// the original string, since lower-casing can change byte lengths (invalid
// UTF-8 bytes become 3-byte U+FFFD), which makes offsets into it unusable.
func cutPrefixFold(s, prefix string) (string, bool) {
	for prefix != "" {
		if s == "" {
			return s, false
		}
		r1, n1 := utf8.DecodeRuneInString(s)
		r2, n2 := utf8.DecodeRuneInString(prefix)
		if r1 != r2 && !strings.EqualFold(string(r1), string(r2)) {
			return s, false
		}
		s, prefix = s[n1:], prefix[n2:]
	}
	return s, true
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r, n := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[n:]
	}
	return strings.Join(words, " ")
}

// General MIDI program names (0-based program numbers).
var gmNames = [128]string{
	"Acoustic Grand Piano", "Bright Acoustic Piano", "Electric Grand Piano", "Honky-tonk Piano",
	"Electric Piano 1", "Electric Piano 2", "Harpsichord", "Clavinet",
	"Celesta", "Glockenspiel", "Music Box", "Vibraphone",
	"Marimba", "Xylophone", "Tubular Bells", "Dulcimer",
	"Drawbar Organ", "Percussive Organ", "Rock Organ", "Church Organ",
	"Reed Organ", "Accordion", "Harmonica", "Tango Accordion",
	"Acoustic Guitar (nylon)", "Acoustic Guitar (steel)", "Electric Guitar (jazz)", "Electric Guitar (clean)",
	"Electric Guitar (muted)", "Overdriven Guitar", "Distortion Guitar", "Guitar Harmonics",
	"Acoustic Bass", "Electric Bass (finger)", "Electric Bass (pick)", "Fretless Bass",
	"Slap Bass 1", "Slap Bass 2", "Synth Bass 1", "Synth Bass 2",
	"Violin", "Viola", "Cello", "Contrabass",
	"Tremolo Strings", "Pizzicato Strings", "Orchestral Harp", "Timpani",
	"String Ensemble 1", "String Ensemble 2", "Synth Strings 1", "Synth Strings 2",
	"Choir Aahs", "Voice Oohs", "Synth Voice", "Orchestra Hit",
	"Trumpet", "Trombone", "Tuba", "Muted Trumpet",
	"French Horn", "Brass Section", "Synth Brass 1", "Synth Brass 2",
	"Soprano Sax", "Alto Sax", "Tenor Sax", "Baritone Sax",
	"Oboe", "English Horn", "Bassoon", "Clarinet",
	"Piccolo", "Flute", "Recorder", "Pan Flute",
	"Blown Bottle", "Shakuhachi", "Whistle", "Ocarina",
	"Lead 1 (square)", "Lead 2 (sawtooth)", "Lead 3 (calliope)", "Lead 4 (chiff)",
	"Lead 5 (charang)", "Lead 6 (voice)", "Lead 7 (fifths)", "Lead 8 (bass + lead)",
	"Pad 1 (new age)", "Pad 2 (warm)", "Pad 3 (polysynth)", "Pad 4 (choir)",
	"Pad 5 (bowed)", "Pad 6 (metallic)", "Pad 7 (halo)", "Pad 8 (sweep)",
	"FX 1 (rain)", "FX 2 (soundtrack)", "FX 3 (crystal)", "FX 4 (atmosphere)",
	"FX 5 (brightness)", "FX 6 (goblins)", "FX 7 (echoes)", "FX 8 (sci-fi)",
	"Sitar", "Banjo", "Shamisen", "Koto",
	"Kalimba", "Bagpipe", "Fiddle", "Shanai",
	"Tinkle Bell", "Agogo", "Steel Drums", "Woodblock",
	"Taiko Drum", "Melodic Tom", "Synth Drum", "Reverse Cymbal",
	"Guitar Fret Noise", "Breath Noise", "Seashore", "Bird Tweet",
	"Telephone Ring", "Helicopter", "Applause", "Gunshot",
}

func gmInstrument(program int) string {
	if program < 0 || program >= len(gmNames) {
		return ""
	}
	return gmNames[program]
}
