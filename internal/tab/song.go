// Package tab reads guitar tab files (Guitar Pro 3-7, TuxGuitar, Power Tab)
// and extracts artist, album, title, tempo, instruments and tunings.
// Artist/album fall back to the directory layout <root>/<Artist>/<Album>/file
// when the file itself has no such metadata.
package tab

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type Track struct {
	Name       string `json:"name"`
	Instrument string `json:"instrument,omitempty"`
	Drums      bool   `json:"drums,omitempty"`
	Pitches    []int  `json:"pitches,omitempty"` // MIDI notes, lowest string first; none for drums
}

// Tuning is the tuning of the track's strings; the zero Tuning for drums.
func (t Track) Tuning() Tuning { return TuningOf(t.Pitches) }

// MarshalJSON adds the tuning, "Drop C (C G C F A D)", for people reading tabscan -json.
// Decoding ignores it: the pitches say it all.
func (t Track) MarshalJSON() ([]byte, error) {
	type plain Track
	return json.Marshal(struct {
		plain
		Tuning string `json:"tuning,omitempty"`
	}{plain(t), t.Tuning().String()})
}

// Format is a tab file's real format, read from its content; the extension may say otherwise.
type Format string

const (
	FormatGP3 Format = "gp3" // Guitar Pro 3
	FormatGP4 Format = "gp4"
	FormatGP5 Format = "gp5"
	FormatGP6 Format = "gp6" // .gpx
	FormatGP7 Format = "gp7" // .gp, Guitar Pro 7 and later
	FormatTG  Format = "tg"  // TuxGuitar 1 and 2
	FormatPTB Format = "ptb" // Power Tab
)

// Source says where a song's artist, album or title came from.
type Source string

const (
	FromFile Source = "file" // the file's own metadata
	FromPath Source = "path" // its folder or file name
)

// Tempo is a tempo (in the score's own beat unit, usually quarter notes)
// that takes effect at a 1-based bar.
type Tempo struct {
	Bar int     `json:"bar"`
	BPM float64 `json:"bpm"`
}

type Song struct {
	Path         string  `json:"path"`   // relative to the scan root
	Format       Format  `json:"format"` // empty if unreadable
	Artist       string  `json:"artist"`
	Album        string  `json:"album"`
	Title        string  `json:"title"`
	ArtistSource Source  `json:"artistSource"`
	AlbumSource  Source  `json:"albumSource"`
	TitleSource  Source  `json:"titleSource"`
	Tracks       []Track `json:"tracks,omitempty"`
	Tempos       []Tempo `json:"tempos,omitempty"` // initial tempo first, then changes
	Error        string  `json:"error,omitempty"`
}

// addTempo records a tempo, ignoring repeats of the current one.
func (s *Song) addTempo(bar int, bpm float64) {
	if n := len(s.Tempos); n > 0 && s.Tempos[n-1].BPM == bpm {
		return
	}
	s.Tempos = append(s.Tempos, Tempo{Bar: bar, BPM: bpm})
}

// DistinctBPMs lists the tempos the song uses, each once, in order of first appearance.
func (s *Song) DistinctBPMs() []float64 {
	var out []float64
	for _, t := range s.Tempos {
		if !slices.Contains(out, t.BPM) {
			out = append(out, t.BPM)
		}
	}
	return out
}

// TempoSummary lists the distinct tempos in order of first appearance: "190, 145".
func (s *Song) TempoSummary() string {
	var out []string
	for _, bpm := range s.DistinctBPMs() {
		out = append(out, strconv.FormatFloat(bpm, 'f', -1, 64))
	}
	return strings.Join(out, ", ")
}

// FilenameTitle is the song title derived from the file name alone.
func (s *Song) FilenameTitle() string {
	dirArtist := ""
	if i := strings.IndexByte(s.Path, '/'); i > 0 {
		dirArtist = s.Path[:i]
	}
	return titleFromFilename(filepath.Base(s.Path), s.Artist, dirArtist)
}

var tabExts = map[string]bool{
	".gp": true, ".gp3": true, ".gp4": true, ".gp5": true, ".gpx": true, ".gtp": true,
	".tg": true, ".ptb": true, ".zip": true, ".crdownload": true,
}

// IsTabFile reports whether the file name has an extension tab files use.
func IsTabFile(name string) bool { return tabExts[strings.ToLower(filepath.Ext(name))] }

// Walk scans arg (a directory, walked recursively, or a single file) and
// calls fn for every tab file, in walk order. Paths in the results are relative
// to root; an empty root means arg itself (or a file argument's directory).
// Like ScanAll, it stops at the first unreadable directory and returns its
// error after the songs found before it.
func Walk(arg, root string, fn func(*Song)) error {
	st, err := os.Stat(arg)
	if err != nil {
		return err
	}
	if root == "" {
		root = arg
		if !st.IsDir() {
			root = filepath.Dir(arg)
		}
	}
	if !st.IsDir() {
		fn(Scan(arg, root))
		return nil
	}
	songs, err := scanTree(arg, root)
	for _, s := range songs {
		fn(s)
	}
	return err
}

// Scan parses one file. It always returns a Song; parse problems are
// reported in Song.Error and the path-based fallbacks are applied.
func Scan(path, root string) *Song {
	s, err := parseFile(path)
	if s == nil {
		s = &Song{}
	}
	if err != nil {
		s.Error = err.Error()
	}
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		rel = path
	}
	s.Path = filepath.ToSlash(rel)
	applyFallbacks(s, s.Path)
	return s
}

// applyFallbacks fills artist/album from <Artist>/<Album>/ path components
// and the title from the file name when the file has none.
func applyFallbacks(s *Song, rel string) {
	var dirs []string
	if d := filepath.Dir(rel); d != "." {
		dirs = strings.Split(d, "/")
	}
	s.ArtistSource, s.AlbumSource, s.TitleSource = FromFile, FromFile, FromFile
	if s.Artist == "" {
		s.ArtistSource = FromPath
		if len(dirs) >= 1 {
			s.Artist = dirs[0]
		}
	}
	if s.Album == "" {
		s.AlbumSource = FromPath
		if len(dirs) >= 2 {
			s.Album = dirs[1]
		}
	}
	if s.Title == "" {
		s.TitleSource = FromPath
		dirArtist := ""
		if len(dirs) >= 1 {
			dirArtist = dirs[0]
		}
		s.Title = titleFromFilename(filepath.Base(rel), s.Artist, dirArtist)
	}
}
