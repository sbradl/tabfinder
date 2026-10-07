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
