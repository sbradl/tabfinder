package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"tabfinder/internal/tab"
)

var (
	// Names that mean no artist or no album, in any library; a config adds its own (junkArtists, junkAlbums).
	reJunkArtist = regexp.MustCompile(`(?i)^(|-|unknown|various|track \d+)$`)
	reJunkAlbum  = regexp.MustCompile(`(?i)^(|-|\?+|single|ep|s/t|self[- ]titled|unknown|untitled` +
		`|tabbed by.*|.*not released.*|\(.*\))$`)
	// Tunings and arrangements at the end of a file name ("song_drop_c", "song 7string"); a config
	// adds its own (fileSuffixes).
	tuningSuffixes  = `c#|c|d|e|eb|b|drop ?[a-g]b?|eadgbe|cgcfad|afadgc|7string|6string|lyrics|live`
	reYearParen     = regexp.MustCompile(`[\(\[]\s*\d{4}\s*[\)\]]`)
	reYearLead      = regexp.MustCompile(`^\d{4}\s*-\s*`)
	reArticle       = regexp.MustCompile(`^(the|die)\s+`)
	reWholeParen    = regexp.MustCompile(`^\(.*\)$`)
	reTrailingParen = regexp.MustCompile(`\s+\(.*\)$`)
	reAnyParen      = regexp.MustCompile(`\(.*?\)|\[.*?\]`)
	reFileNoise     = regexp.MustCompile(`(?i)\(.*?\)|\bver ?\d+\b|\bv\d+\b| - \d+$`)
)

// key is a loose comparison key: case, punctuation, years and a leading
// article ignored.
func key(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = reYearParen.ReplaceAllString(s, "")
	s = reYearLead.ReplaceAllString(s, "")
	s = reArticle.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
}

// cleanAlbum is the folder name for an album name found in a file, or "" for no album.
func (r nameRules) cleanAlbum(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	name = reYearLead.ReplaceAllString(name, "") // leading "1978 - "
	if !reWholeParen.MatchString(name) {
		name = reTrailingParen.ReplaceAllString(name, "") // trailing (2008) / (subtitle)
	}
	if alias, ok := r.aliases[key(name)]; ok {
		name = alias
	}
	name = strings.TrimSpace(strings.TrimRight(name, "."))
	if reJunkAlbum.MatchString(name) || (r.junk != nil && r.junk.MatchString(name)) {
		return ""
	}
	return strings.ReplaceAll(name, "/", "-")
}

// similar treats keys as equal when identical or, for longer keys, at
// least 88% alike (catches typos like "Villians").
func similar(a, b string) bool {
	if a == b {
		return true
	}
	return min(len([]rune(a)), len([]rune(b))) >= 6 && ratio(a, b) >= 0.88
}

// nicest picks the display spelling: most frequent, then most capitalized
// words, then shortest; the first such name wins ties.
func nicest(names []string) string {
	count := map[string]int{}
	var order []string
	for _, n := range names {
		if count[n] == 0 {
			order = append(order, n)
		}
		count[n]++
	}
	score := func(n string) [3]int {
		caps := 0
		for _, w := range strings.Fields(n) {
			if r := []rune(w)[0]; unicode.IsUpper(r) {
				caps++
			}
		}
		return [3]int{count[n], caps, -len([]rune(n))}
	}
	best := order[0]
	for _, n := range order[1:] {
		if s, b := score(n), score(best); s[0] > b[0] || s[0] == b[0] && (s[1] > b[1] || s[1] == b[1] && s[2] > b[2]) {
			best = n
		}
	}
	return best
}

// filenameKey is the song-title key derived from the file name alone.
func (r nameRules) filenameKey(s *tab.Song) string {
	name := strings.TrimSpace(reFileNoise.ReplaceAllString(tab.BareName(filepath.Base(s.Path)), ""))
	parts := strings.Split(name, " - ")
	for len(parts) > 1 && tab.IsVariant(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1] // a variant of the song is the same song
	}
	k := key(r.dropSuffixes(parts[len(parts)-1]))
	for _, artist := range []string{strings.SplitN(s.Path, "/", 2)[0], s.Artist} {
		if ak := key(artist); ak != "" && strings.HasPrefix(k, ak) && len(k) > len(ak) {
			k = k[len(ak):]
			for _, art := range []string{"the", "die"} {
				if strings.HasPrefix(k, art) && len(k) > len(art) {
					k = k[len(art):]
					break
				}
			}
		}
	}
	return k
}

// songKey identifies a song for duplicate detection. The file name wins
// when the title field contradicts it (e.g. title "Eisenmond" in
// "Eisenmond - Heimatland.gp5").
func (r nameRules) songKey(s *tab.Song) string {
	fk := r.filenameKey(s)
	if s.TitleSource == tab.FromPath {
		return fk
	}
	full := key(s.Title) // keeps "(Part 2)" etc. for the consistency check
	if full == "" || (fk != "" && !strings.Contains(fk, full) && !strings.Contains(full, fk)) {
		return fk
	}
	if k := key(reAnyParen.ReplaceAllString(s.Title, "")); k != "" {
		return k
	}
	return fk
}

var unsafeChars = strings.NewReplacer("/", "-", "\\", "-", ":", " -", `"`, "", "?", "", "*", "", "<", "", ">", "", "|", "")

// songName is the file name (without extension) a tab should get: the song
// title from the file when it agrees with the file name, otherwise the
// title derived from the file name. Version tags, tuning suffixes and the
// artist prefix are dropped.
func (r nameRules) songName(s *tab.Song) string {
	fromFile := r.dropSuffixes(s.FilenameTitle())
	name := fromFile
	if s.TitleSource == tab.FromFile {
		meta := tab.StripTags(s.Title)
		if a, rest, ok := strings.Cut(meta, " - "); ok && key(a) == key(s.Artist) {
			meta = rest
		}
		mk, fk := key(meta), key(fromFile)
		if mk != "" && !r.placeholderTitle(meta) && (strings.Contains(fk, mk) || strings.Contains(mk, fk)) {
			name = meta
		}
	}
	name = unsafeChars.Replace(name)
	name = strings.Trim(strings.Join(strings.Fields(name), " "), ` '´`+"`")
	name = strings.TrimSpace(strings.TrimRight(name, ". "))
	if name == strings.ToLower(name) {
		name = tab.TitleCase(name)
	}
	return name
}

// tabExt returns the extension to keep, including a wrapper such as ".gp3.zip".
func tabExt(name string) string {
	ext := filepath.Ext(name)
	if l := strings.ToLower(ext); l == ".zip" || l == ".crdownload" {
		if inner := filepath.Ext(strings.TrimSuffix(name, ext)); tab.IsTabExt(inner) {
			return inner + ext
		}
	}
	return ext
}
