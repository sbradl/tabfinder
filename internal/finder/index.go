package finder

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"tabfinder/internal/tab"
)

// The apps cache a scan as an index file in `tabscan -json` format, so they
// start without rescanning.

// LoadIndex reads an index; a missing one is no error.
func LoadIndex(path string) ([]*tab.Song, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	var songs []*tab.Song
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 16<<20)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var s tab.Song
		if err := json.Unmarshal(sc.Bytes(), &s); err != nil {
			return nil, err
		}
		songs = append(songs, &s)
	}
	return songs, sc.Err()
}

// ScanIndex scans root on all CPUs and replaces the index at path. Like
// tab.ScanAll it returns what it found along with an unreadable directory's error.
func ScanIndex(root, path string) ([]*tab.Song, error) {
	songs, scanErr := tab.ScanAll(root)
	if scanErr != nil && len(songs) == 0 {
		return nil, scanErr
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, s := range songs {
		enc.Encode(s)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return songs, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return songs, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return songs, err
	}
	return songs, scanErr
}

// tabscan format -> the extension TuxGuitar expects; the rest match already.
var extensions = map[string]string{"gp6": "gpx", "gp7": "gp"}

var unsafeName = regexp.MustCompile(`[^\p{L}\p{N} _-]`)

// TuxGuitarName is a clean file name TuxGuitar can open s under: the title
// with the extension of its real format. Originals may be misnamed
// (.crdownload, .zip, wrong gp version), and TuxGuitar's Android path
// patterns fail on names with many dots.
func TuxGuitarName(s *tab.Song) string {
	title := strings.TrimSpace(unsafeName.ReplaceAllString(s.Title, ""))
	if title == "" {
		title = "song"
	}
	return title + "." + tuxGuitarExt(s)
}

// OpensInPlace reports whether s already has the extension of its real
// format, so TuxGuitar on the desktop can open (and save) the original.
func OpensInPlace(s *tab.Song) bool {
	return strings.EqualFold(filepath.Ext(s.Path), "."+tuxGuitarExt(s))
}

func tuxGuitarExt(s *tab.Song) string {
	if ext := extensions[s.Format]; ext != "" {
		return ext
	}
	if s.Format != "" {
		return s.Format
	}
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(s.Path)), ".")
}
