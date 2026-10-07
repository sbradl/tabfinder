package tab

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reVersionTag = regexp.MustCompile(`(?i)\s*\((ver ?\d+[^)]*|\d+|pro|complete)\)`)
	reVerSuffix  = regexp.MustCompile(`(?i)\s+(ver\s?\d+|v\d+)\b`)
	reIDSuffix   = regexp.MustCompile(`\s+-\s+\d+$`)
	// reSiteTag matches the site a download came from, put in front of its name: "www-example-tk @ ".
	reSiteTag = regexp.MustCompile(`(?i)^www[.-]\S* @ `)
	// reJunkBracket matches "(Tabbed By X - www.example.org ...)", "[by x@y.com]" and the like.
	reJunkBracket = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*(tabbed|www\.|@|\bby\b)[^\)\]]*([\)\]]|$)`)
	// reVariant matches what follows " - " in "Title - Acoustic Version": a variant of the
	// song, not the title after an artist's name.
	reVariant = regexp.MustCompile(`(?i)^(acoustic|live|unplugged|demo|instrumental|remix|remastered|radio edit|edit|reprise|intro|outro|solo|cover|bonus|part \d+|pt\.? ?\d+|\w+ (version|mix|edit))\b`)
)

// titleFromFilename derives a song title from names like
// "Argyle Moth - Nectar (ver 3 by X).gp5" or "as_lanterns_fade_the_ferry.gp5".
func titleFromFilename(name string, artists ...string) string {
	name = StripTags(BareName(name))
	name = reVerSuffix.ReplaceAllString(name, "")
	name = reIDSuffix.ReplaceAllString(name, "")
	name = strings.Join(strings.Fields(name), " ")

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
	name = CutArtistPrefix(name)
	if r, _ := utf8.DecodeRuneInString(name); unicode.IsLower(r) {
		name = TitleCase(name) // snake_case file names
	}
	return name
}

// CutArtistPrefix drops a leading "Other Artist - " from a file name's title part,
// unless what follows is a variant of the song ("Nectar - Acoustic Version").
func CutArtistPrefix(name string) string {
	before, after, ok := strings.Cut(name, " - ")
	if !ok || before == "" || IsVariant(after) {
		return name
	}
	return strings.TrimSpace(after)
}

// IsVariant reports whether a title part names a variant of a song ("Acoustic Version", "Live").
func IsVariant(part string) bool { return reVariant.MatchString(strings.TrimSpace(part)) }

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

// BareName is a file name without its extensions (wrappers like ".gp5.zip" included) and
// download noise, with underscores as spaces: "www-example-tk @ my_song.gp5.zip" is "my song".
func BareName(name string) string {
	for _, ext := range []string{".crdownload", ".zip"} {
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = reSiteTag.ReplaceAllString(name, "")
	return strings.ReplaceAll(name, "_", " ")
}

// StripTags drops credits ("(Tabbed By X)", "[by x@y.com]") and version tags ("(ver 2)",
// "(Pro)") from a title.
func StripTags(s string) string {
	s = reJunkBracket.ReplaceAllString(s, "")
	return strings.TrimSpace(reVersionTag.ReplaceAllString(s, ""))
}

// TitleCase upper-cases the first letter of every word.
func TitleCase(s string) string {
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
