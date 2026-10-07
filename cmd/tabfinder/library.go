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
	"strings"

	"tabfinder/internal/tab"
	"tabfinder/internal/tuxguitar"
)

// dirs is where the app keeps its files: the folder setting, the saved scan and the copies it
// opens TuxGuitar on. Tests point it at temp dirs.
type dirs struct{ config, cache string }

// userDirs is ~/.config/tabfinder and ~/.cache/tabfinder, or where XDG_CONFIG_HOME and XDG_CACHE_HOME say.
func userDirs() dirs {
	return dirs{xdgDir("XDG_CONFIG_HOME", ".config"), xdgDir("XDG_CACHE_HOME", ".cache")}
}

// index is the saved scan, in tabscan -json format.
func (d dirs) index() string { return filepath.Join(d.cache, "index.jsonl") }

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

// loadRoot reads the tab folder from config.json; none is no folder. A config.json that
// can't be read is an error, and no folder.
func (d dirs) loadRoot() (string, error) {
	file := filepath.Join(d.config, "config.json")
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	var c config
	if err := json.Unmarshal(b, &c); err != nil {
		return "", fmt.Errorf("%s: %w", file, err)
	}
	return c.Root, nil
}

func (d dirs) saveRoot(root string) error {
	b, _ := json.MarshalIndent(config{Root: root}, "", "  ")
	if err := os.MkdirAll(d.config, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.config, "config.json"), append(b, '\n'), 0o644)
}

func (d dirs) dropIndex() { os.Remove(d.index()) }

// pickFolder asks for the tab folder with KDE's or GNOME's own dialog.
// It returns "" if the dialog was cancelled (exit status 1, for both).
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
		var exit *exec.ExitError
		switch {
		case err == nil:
			return strings.TrimSpace(string(out)), nil
		case errors.As(err, &exit) && exit.ExitCode() == 1:
			return "", nil // cancelled
		case errors.As(err, &exit) && len(bytes.TrimSpace(exit.Stderr)) > 0:
			return "", fmt.Errorf("%s: %w: %s", d[0], err, bytes.TrimSpace(exit.Stderr))
		default:
			return "", fmt.Errorf("%s: %w", d[0], err)
		}
	}
	return "", errors.New("install kdialog or zenity to choose a folder")
}

// openInTuxGuitar opens correctly named files in place, so edits save to
// the original; misnamed ones (.crdownload, .zip, wrong gp version) go via
// a copy with the extension of their real format.
func (d dirs) openInTuxGuitar(root string, s *tab.Song) error {
	path := filepath.Join(root, filepath.FromSlash(s.Path))
	if !tuxguitar.OpensInPlace(s) {
		dst := filepath.Join(d.cache, "open", tuxguitar.FileName(s))
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
