package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"tab-sync/internal/tab"
)

var (
	reJunkArtist = regexp.MustCompile(`(?i)^(|-|unknown|unbekannt|anonim|various|track \d+)$`)
	reJunkAlbum  = regexp.MustCompile(`(?i)^(|-|\?+|single|ep|s/t|self[- ]titled|unknown|unbekannt|keine ahnung|untitled` +
		`|tabbed by.*|.*not released.*|\(.*\))$`)
	reTuningSuffix = regexp.MustCompile(`(?i)([ _-](c#|c|d|e|eb|b|drop ?[a-g]b?|eadgbe|cgcfad|afadgc|e accoustic( orig)?|accoustic` +
		`|7string|6string|oneguitar|withbass|lyrics|live|andere?solofingersatz))+$`)
	reYearParen     = regexp.MustCompile(`[\(\[]\s*\d{4}\s*[\)\]]`)
	reYearLead      = regexp.MustCompile(`^\d{4}\s*-\s*`)
	reArticle       = regexp.MustCompile(`^(the|die)\s+`)
	reSpaces        = regexp.MustCompile(`\s+`)
	reWholeParen    = regexp.MustCompile(`^\(.*\)$`)
	reTrailingParen = regexp.MustCompile(`\s+\(.*\)$`)
	reAnyParen      = regexp.MustCompile(`\(.*?\)|\[.*?\]`)
	reFileNoise     = regexp.MustCompile(`(?i)\(.*?\)|\bver ?\d+\b|\bv\d+\b| - \d+$`)
	reVersionTag    = regexp.MustCompile(`(?i)\s*\((ver ?\d+[^)]*|\d+|pro|complete)\)`)
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

func cleanAlbum(name string) string {
	name = strings.TrimSpace(reSpaces.ReplaceAllString(name, " "))
	name = reYearLead.ReplaceAllString(name, "") // leading "1978 - "
	if !reWholeParen.MatchString(name) {
		name = reTrailingParen.ReplaceAllString(name, "") // trailing (2008) / (subtitle)
	}
	if alias, ok := aliasesByKey[key(name)]; ok {
		name = alias
	}
	name = strings.TrimSpace(strings.TrimRight(name, "."))
	if reJunkAlbum.MatchString(name) || (reJunkExtra != nil && reJunkExtra.MatchString(name)) {
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
func filenameKey(s *tab.Song) string {
	name := filepath.Base(s.Path)
	for _, ext := range []string{".crdownload", ".zip"} {
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.ReplaceAll(strings.TrimPrefix(name, "www-tablatures-tk @ "), "_", " ")
	name = strings.TrimSpace(reFileNoise.ReplaceAllString(name, ""))
	parts := strings.Split(name, " - ")
	k := key(reTuningSuffix.ReplaceAllString(parts[len(parts)-1], ""))
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
func songKey(s *tab.Song) string {
	fk := filenameKey(s)
	if s.TitleSource == "path" {
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
func songName(s *tab.Song) string {
	fromFile := reTuningSuffix.ReplaceAllString(s.FilenameTitle(), "")
	name := fromFile
	if s.TitleSource == "file" {
		meta := tab.JunkBracket.ReplaceAllString(s.Title, "")
		meta = strings.TrimSpace(reVersionTag.ReplaceAllString(meta, ""))
		if a, rest, ok := strings.Cut(meta, " - "); ok && key(a) == key(s.Artist) {
			meta = rest
		}
		mk, fk := key(meta), key(fromFile)
		if mk != "" && !reJunkArtist.MatchString(meta) && (strings.Contains(fk, mk) || strings.Contains(mk, fk)) {
			name = meta
		}
	}
	name = unsafeChars.Replace(name)
	name = strings.Trim(reSpaces.ReplaceAllString(name, " "), ` '´`+"`")
	name = strings.TrimSpace(strings.TrimRight(name, ". "))
	if name == strings.ToLower(name) {
		name = titleCase(name)
	}
	return name
}

// titleCase upper-cases the first letter of every word.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// tabExt returns the extension to keep, including a wrapper such as ".gp3.zip".
func tabExt(name string) string {
	ext := filepath.Ext(name)
	if l := strings.ToLower(ext); l == ".zip" || l == ".crdownload" {
		if inner := filepath.Ext(strings.TrimSuffix(name, ext)); tab.IsTabFile("x" + inner) {
			return inner + ext
		}
	}
	return ext
}
