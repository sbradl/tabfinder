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
// general: folders to leave alone, spelling fixes for album names, and album names that
// are junk. It lives in the user's config directory (see configPath), not in the code.
type config struct {
	Skip         []string          `json:"skip,omitempty"`         // folders, relative to the root, left untouched unless -skip is given
	AlbumAliases map[string]string `json:"albumAliases,omitempty"` // album name as found (compared via key) -> folder name
	JunkAlbums   []string          `json:"junkAlbums,omitempty"`   // regular expressions for album names that mean "no album", matched against the whole name, ignoring case
}

var (
	cfg          config
	aliasesByKey map[string]string
	reJunkExtra  *regexp.Regexp // from cfg.JunkAlbums; nil for none
)

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

// setConfig makes c the configuration the name rules use.
func setConfig(c config) error {
	var junk *regexp.Regexp
	if len(c.JunkAlbums) > 0 {
		parts := make([]string, len(c.JunkAlbums))
		for i, p := range c.JunkAlbums {
			parts[i] = "(?:" + p + ")"
		}
		var err error
		if junk, err = regexp.Compile("(?i)^(?:" + strings.Join(parts, "|") + ")$"); err != nil {
			return fmt.Errorf("junkAlbums: %w", err)
		}
	}
	byKey := map[string]string{}
	for k, v := range c.AlbumAliases {
		byKey[key(k)] = v
	}
	cfg, aliasesByKey, reJunkExtra = c, byKey, junk
	return nil
}
