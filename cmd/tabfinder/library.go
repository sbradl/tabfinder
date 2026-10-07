package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"tabfinder/internal/finder"
	"tabfinder/internal/tab"
)

var (
	configDir = xdgDir("XDG_CONFIG_HOME", ".config")
	cacheDir  = xdgDir("XDG_CACHE_HOME", ".cache")
	indexFile = filepath.Join(cacheDir, "index.jsonl") // tabscan -json format
)

func xdgDir(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return filepath.Join(d, "tabfinder")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback, "tabfinder")
}

type config struct {
	Root string `json:"root"`
}

func loadRoot() string {
	var c config
	if b, err := os.ReadFile(filepath.Join(configDir, "config.json")); err == nil {
		json.Unmarshal(b, &c)
		return c.Root
	}
	return legacyRoot()
}

func saveRoot(root string) error {
	b, _ := json.MarshalIndent(config{Root: root}, "", "  ")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "config.json"), append(b, '\n'), 0o644)
}

// legacyRoot reads the folder from the settings.properties the earlier
// Compose desktop app wrote (Java properties escaping).
func legacyRoot() string {
	b, err := os.ReadFile(filepath.Join(configDir, "settings.properties"))
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "root="); ok {
			return unescapeProperty(v)
		}
	}
	return ""
}

var propEscape = regexp.MustCompile(`\\u[0-9a-fA-F]{4}|\\.`)

func unescapeProperty(v string) string {
	return propEscape.ReplaceAllStringFunc(v, func(e string) string {
		if len(e) == 6 {
			r, _ := strconv.ParseUint(e[2:], 16, 16)
			return string(rune(r))
		}
		return e[1:]
	})
}

func dropIndex() { os.Remove(indexFile) }

// pickFolder asks for the tab folder with KDE's or GNOME's own dialog.
// It returns "" if the dialog was cancelled.
func pickFolder(start string) (string, error) {
	if start == "" {
		start, _ = os.UserHomeDir()
	}
	dialogs := [][]string{
		{"kdialog", "--title", "Choose your tab folder", "--getexistingdirectory", start},
		{"zenity", "--file-selection", "--directory", "--title=Choose your tab folder", "--filename=" + start + "/"},
	}
	for _, d := range dialogs {
		if _, err := exec.LookPath(d[0]); err != nil {
			continue
		}
		out, err := exec.Command(d[0], d[1:]...).Output()
		if err != nil {
			return "", nil // cancelled
		}
		return strings.TrimSpace(string(out)), nil
	}
	return "", errors.New("install kdialog or zenity to choose a folder")
}

// openInTuxGuitar opens correctly named files in place, so edits save to
// the original; misnamed ones (.crdownload, .zip, wrong gp version) go via
// a copy with the extension of their real format.
func openInTuxGuitar(root string, s *tab.Song) error {
	path := filepath.Join(root, filepath.FromSlash(s.Path))
	if !finder.OpensInPlace(s) {
		dst := filepath.Join(cacheDir, "open", finder.TuxGuitarName(s))
		if err := copyFile(path, dst); err != nil {
			return err
		}
		path = dst
	}
	cmd := exec.Command("tuxguitar", path)
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("TuxGuitar is not installed")
		}
		return err
	}
	go cmd.Wait()
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return out.Close()
}

// prefersDark reads the dark mode setting KDE and GNOME both write; ok is
// false if it can't be read.
func prefersDark() (dark, ok bool) {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return false, false
	}
	return bytes.Contains(out, []byte("dark")), true
}
