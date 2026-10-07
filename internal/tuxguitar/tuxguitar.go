// Package tuxguitar knows what TuxGuitar needs to open a tab: a file name with the
// extension of the file's real format.
package tuxguitar

import (
	"path/filepath"
	"regexp"
	"strings"

	"tabfinder/internal/tab"
)

// Format -> the extension TuxGuitar expects; the rest match already.
var extensions = map[tab.Format]string{tab.FormatGP6: "gpx", tab.FormatGP7: "gp"}

var unsafeName = regexp.MustCompile(`[^\p{L}\p{N} _-]`)

// FileName is a clean file name TuxGuitar can open s under: the title
// with the extension of its real format. Originals may be misnamed
// (.crdownload, .zip, wrong gp version), and TuxGuitar's Android path
// patterns fail on names with many dots.
func FileName(s *tab.Song) string {
	title := strings.TrimSpace(unsafeName.ReplaceAllString(s.Title, ""))
	if title == "" {
		title = "song"
	}
	return title + "." + extension(s)
}

// OpensInPlace reports whether s already has the extension of its real
// format, so TuxGuitar on the desktop can open (and save) the original.
func OpensInPlace(s *tab.Song) bool {
	return strings.EqualFold(filepath.Ext(s.Path), "."+extension(s))
}

func extension(s *tab.Song) string {
	if ext := extensions[s.Format]; ext != "" {
		return ext
	}
	if s.Format != "" {
		return string(s.Format)
	}
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(s.Path)), ".")
}
