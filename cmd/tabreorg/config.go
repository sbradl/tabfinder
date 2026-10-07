package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// config is what tabreorg knows about one person's library rather than about tabs in
// general: folders to leave alone, spelling fixes for album names, names that are junk and
// the notes its file names end in. It lives in the user's config directory (see
// configPath), not in the code. Patterns are regular expressions, ignoring case.
type config struct {
	Skip         []string          `json:"skip,omitempty"`         // folders, relative to the root, left untouched unless -skip is given
	AlbumAliases map[string]string `json:"albumAliases,omitempty"` // album name as found (compared via key) -> folder name
	JunkAlbums   []string          `json:"junkAlbums,omitempty"`   // album names that mean "no album", matching the whole name
	JunkArtists  []string          `json:"junkArtists,omitempty"`  // artist names that mean "no artist", matching the whole name
	FileSuffixes []string          `json:"fileSuffixes,omitempty"` // notes at the end of file names, after a space, _ or -: "withbass" for "song_withbass.gp5"
}

// nameRules is the rules for names, built in and from a config: spelling fixes for albums, names that mean
// no album or no artist, and the notes file names end in. The zero value has the built-in
// rules only.
type nameRules struct {
	aliases    map[string]string // key(name as found) -> folder name
	junk       *regexp.Regexp    // from JunkAlbums; nil for none
	junkArtist *regexp.Regexp    // from JunkArtists; nil for none
	suffix     *regexp.Regexp    // the built-in tuning suffixes and FileSuffixes
}

// configPath is $XDG_CONFIG_HOME/tabreorg/config.json, or under ~/.config.
func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "tabreorg", "config.json")
}

// loadConfig reads path. A missing file is no config, unless the path was asked for with -config.
func loadConfig(path string, asked bool) (config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !asked {
		return config{}, nil
	}
	if err != nil {
		return config{}, err
	}
	var c config
	if err := json.Unmarshal(b, &c); err != nil {
		return config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// newNameRules adds the rules of c to the built-in ones.
func newNameRules(c config) (nameRules, error) {
	r := nameRules{aliases: map[string]string{}}
	var err error
	if r.junk, err = wholeName("junkAlbums", c.JunkAlbums); err != nil {
		return nameRules{}, err
	}
	if r.junkArtist, err = wholeName("junkArtists", c.JunkArtists); err != nil {
		return nameRules{}, err
	}
	if r.suffix, err = regexp.Compile(`(?i)([ _-](` + strings.Join(append([]string{tuningSuffixes}, c.FileSuffixes...), "|") + `))+$`); err != nil {
		return nameRules{}, fmt.Errorf("fileSuffixes: %w", err)
	}
	for k, v := range c.AlbumAliases {
		r.aliases[key(k)] = v
	}
	return r, nil
}

// wholeName compiles patterns to one that matches a whole name ignoring case; nil for none.
func wholeName(field string, patterns []string) (*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	parts := make([]string, len(patterns))
	for i, p := range patterns {
		parts[i] = "(?:" + p + ")"
	}
	re, err := regexp.Compile("(?i)^(?:" + strings.Join(parts, "|") + ")$")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return re, nil
}

// junkArtistName reports whether an artist name means "no artist".
func (r nameRules) junkArtistName(name string) bool {
	return reJunkArtist.MatchString(name) || (r.junkArtist != nil && r.junkArtist.MatchString(name))
}

// placeholderTitle reports whether a title names no song: the placeholders files put in for
// a missing artist ("Unknown", "Track 3") stand in for a missing title too.
func (r nameRules) placeholderTitle(title string) bool { return r.junkArtistName(title) }

// dropSuffixes drops the tuning and arrangement notes from the end of a file name's title.
func (r nameRules) dropSuffixes(name string) string { return r.suffix.ReplaceAllString(name, "") }
