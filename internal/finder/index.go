package finder

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"tabfinder/internal/tab"
)

// The apps cache a scan as an index file, so they start without rescanning: a line
// IndexHeader, then the songs in `tabscan -json` format.

// IndexHeader is the first line of an index of this version. An index of another version
// is ignored, so that a new version of the apps scans again for what it knows of songs
// (version 2: the parts of a song and how hard they are; 3: rated anew).
const IndexHeader = `{"tabfinderIndex":3}`

// LoadIndex reads an index; a missing one, or one of another version, is none and no error.
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
	header := false
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if !header {
			if string(line) != IndexHeader {
				return nil, nil // another version
			}
			header = true
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
	buf.WriteString(IndexHeader + "\n")
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, s := range songs {
		if err := enc.Encode(s); err != nil {
			return songs, fmt.Errorf("%s: %w", s.Path, err)
		}
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
