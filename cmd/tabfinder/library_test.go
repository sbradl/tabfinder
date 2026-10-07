package main

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"tabfinder/internal/tab"
)

// isolate gives the app temp dirs, so no test touches the real ~/.config/tabfinder or ~/.cache/tabfinder.
func isolate(t testing.TB) dirs {
	t.Helper()
	dir := t.TempDir()
	return dirs{filepath.Join(dir, "config"), filepath.Join(dir, "cache")}
}

// fakeTools puts shell scripts named after the tools in a temp dir that is the
// only thing on PATH (scripts must stick to shell builtins), and returns it.
// A script can record what it was called with in "$0.args" (see record).
func fakeTools(t testing.TB, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

// record is the start of a fake tool that writes its arguments, one per line, to <tool>.args.
const record = `printf '%s\n' "$@" > "$0.args"` + "\n"

// calledWith waits for the fake tool to have run and returns its arguments.
func calledWith(t testing.TB, dir, tool string) []string {
	t.Helper()
	path := filepath.Join(dir, tool+".args")
	for range 200 {
		if b, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(b), "\n") {
			return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s was not called", tool)
	return nil
}

func neverCalled(t testing.TB, dir, tool string) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, tool+".args")); err == nil {
		t.Errorf("%s was called", tool)
	}
}

// U-DSK-01
func TestXDGDir(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	t.Setenv("XDG_CACHE_HOME", "")
	if got := xdgDir("XDG_CONFIG_HOME", ".config"); got != "/custom/config/tabfinder" {
		t.Errorf("set: %q", got)
	}
	if got := xdgDir("XDG_CACHE_HOME", ".cache"); got != "/home/someone/.cache/tabfinder" {
		t.Errorf("empty: %q", got)
	}
	os.Unsetenv("XDG_CACHE_HOME")
	if got := xdgDir("XDG_CACHE_HOME", ".cache"); got != "/home/someone/.cache/tabfinder" {
		t.Errorf("unset: %q", got)
	}
}

// U-DSK-02
func TestLoadSaveRoot(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		d := isolate(t)
		if got, _ := d.loadRoot(); got != "" {
			t.Errorf("root = %q", got)
		}
	})
	t.Run("round trip", func(t *testing.T) {
		d := isolate(t)
		config := d.config
		for _, root := range []string{"/home/me/Guitar/Tabs", "/tmp/Äther & \"Quotes\" ü", ""} {
			if err := d.saveRoot(root); err != nil {
				t.Fatal(err)
			}
			if got, _ := d.loadRoot(); got != root {
				t.Errorf("loaded %q, saved %q", got, root)
			}
		}
		d.saveRoot("/x")
		b, _ := os.ReadFile(filepath.Join(config, "config.json"))
		if string(b) != "{\n  \"root\": \"/x\"\n}\n" {
			t.Errorf("config.json = %q", b)
		}
		st, _ := os.Stat(config)
		if !st.IsDir() || st.Mode().Perm()&0o700 != 0o700 {
			t.Errorf("config dir: %v", st.Mode())
		}
	})
	t.Run("corrupt config", func(t *testing.T) {
		d := isolate(t)
		config := d.config
		os.MkdirAll(config, 0o755)
		for _, bad := range []string{"{", "not json", "[1]", `{"root": 5}`, "\x00\x01", ""} {
			os.WriteFile(filepath.Join(config, "config.json"), []byte(bad), 0o644)
			if got, err := d.loadRoot(); got != "" || err == nil || !strings.Contains(err.Error(), "config.json") {
				t.Errorf("%q: root = %q, err = %v", bad, got, err)
			}
		}
	})
	t.Run("config dir can't be made", func(t *testing.T) {
		d := isolate(t)
		config := d.config
		os.MkdirAll(filepath.Dir(config), 0o755)
		os.WriteFile(config, []byte("a file"), 0o644) // where the directory should go
		if err := d.saveRoot("/x"); err == nil {
			t.Error("no error")
		}
	})
}

// U-DSK-04
func TestPickFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Run("kdialog: output trimmed, start folder passed", func(t *testing.T) {
		dir := fakeTools(t, map[string]string{"kdialog": record + `printf '/chosen/Tabs\n'`})
		got, err := pickFolder("/start/here")
		if err != nil || got != "/chosen/Tabs" {
			t.Errorf("got %q, %v", got, err)
		}
		args := calledWith(t, dir, "kdialog")
		if !slices.Equal(args, []string{"--title", "Choose your tab folder", "--getexistingdirectory", "/start/here"}) {
			t.Errorf("args = %q", args)
		}
	})
	t.Run("kdialog cancelled", func(t *testing.T) {
		fakeTools(t, map[string]string{"kdialog": "exit 1"})
		if got, err := pickFolder("/x"); got != "" || err != nil {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("kdialog wins over zenity", func(t *testing.T) {
		dir := fakeTools(t, map[string]string{"kdialog": "printf /k", "zenity": record + "printf /z"})
		if got, _ := pickFolder("/x"); got != "/k" {
			t.Errorf("got %q", got)
		}
		neverCalled(t, dir, "zenity")
	})
	t.Run("a cancelled kdialog doesn't fall through to zenity", func(t *testing.T) {
		dir := fakeTools(t, map[string]string{"kdialog": "exit 1", "zenity": record + "printf /z"})
		if got, err := pickFolder("/x"); got != "" || err != nil {
			t.Errorf("got %q, %v", got, err)
		}
		neverCalled(t, dir, "zenity")
	})
	t.Run("only zenity", func(t *testing.T) {
		dir := fakeTools(t, map[string]string{"zenity": record + `printf '  /zen/Tabs  \n'`})
		got, err := pickFolder("/start/here")
		if err != nil || got != "/zen/Tabs" {
			t.Errorf("got %q, %v", got, err)
		}
		args := calledWith(t, dir, "zenity")
		if !slices.Equal(args, []string{"--file-selection", "--directory", "--title=Choose your tab folder", "--filename=/start/here/"}) {
			t.Errorf("args = %q", args)
		}
	})
	t.Run("a dialog that fails is an error, not a cancel", func(t *testing.T) {
		fakeTools(t, map[string]string{"kdialog": "echo 'cannot connect to X server' >&2; exit 2"})
		if got, err := pickFolder("/x"); got != "" || err == nil || !strings.Contains(err.Error(), "cannot connect") {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("zenity cancelled", func(t *testing.T) {
		fakeTools(t, map[string]string{"zenity": "exit 1"})
		if got, err := pickFolder("/x"); got != "" || err != nil {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("neither", func(t *testing.T) {
		fakeTools(t, nil)
		got, err := pickFolder("/x")
		if got != "" || err == nil || !strings.Contains(err.Error(), "kdialog") || !strings.Contains(err.Error(), "zenity") {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("empty start is the home folder", func(t *testing.T) {
		dir := fakeTools(t, map[string]string{"kdialog": record + "printf /x"})
		pickFolder("")
		if args := calledWith(t, dir, "kdialog"); args[3] != home {
			t.Errorf("start = %q, want %q", args[3], home)
		}
		dir = fakeTools(t, map[string]string{"zenity": record + "printf /x"})
		pickFolder("")
		if args := calledWith(t, dir, "zenity"); args[3] != "--filename="+home+"/" {
			t.Errorf("start = %q", args[3])
		}
	})
}

// U-DSK-05
func TestOpenInTuxGuitar(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "A"), 0o755)
	os.WriteFile(filepath.Join(root, "A", "right.gp5"), []byte("right"), 0o644)
	os.WriteFile(filepath.Join(root, "A", "wrong.gp3"), []byte("wrong"), 0o644)

	t.Run("correctly named: the original is opened in place", func(t *testing.T) {
		d := isolate(t)
		dir := fakeTools(t, map[string]string{"tuxguitar": record})
		err := d.openInTuxGuitar(root, &tab.Song{Path: "A/right.gp5", Format: "gp5", Title: "Right"})
		if err != nil {
			t.Fatal(err)
		}
		if args := calledWith(t, dir, "tuxguitar"); !slices.Equal(args, []string{filepath.Join(root, "A", "right.gp5")}) {
			t.Errorf("args = %q", args)
		}
		if _, err := os.Stat(filepath.Join(d.cache, "open")); err == nil {
			t.Error("a copy was made")
		}
	})
	t.Run("misnamed: a copy with the real extension, overwriting an older one", func(t *testing.T) {
		d := isolate(t)
		cache := d.cache
		dir := fakeTools(t, map[string]string{"tuxguitar": record})
		copyPath := filepath.Join(cache, "open", "Wrong Song.gp5")
		os.MkdirAll(filepath.Dir(copyPath), 0o755)
		os.WriteFile(copyPath, []byte("an older, longer copy of another song"), 0o644)
		err := d.openInTuxGuitar(root, &tab.Song{Path: "A/wrong.gp3", Format: "gp5", Title: "Wrong: Song!"})
		if err != nil {
			t.Fatal(err)
		}
		if args := calledWith(t, dir, "tuxguitar"); !slices.Equal(args, []string{copyPath}) {
			t.Errorf("args = %q, want %q", args, copyPath)
		}
		if b, _ := os.ReadFile(copyPath); string(b) != "wrong" {
			t.Errorf("copy = %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(root, "A", "wrong.gp3")); string(b) != "wrong" {
			t.Errorf("original changed: %q", b)
		}
	})
	t.Run("source missing", func(t *testing.T) {
		d := isolate(t)
		dir := fakeTools(t, map[string]string{"tuxguitar": record})
		err := d.openInTuxGuitar(root, &tab.Song{Path: "A/gone.gp3", Format: "gp5", Title: "Gone"})
		if err == nil {
			t.Fatal("no error")
		}
		neverCalled(t, dir, "tuxguitar")
	})
	t.Run("tuxguitar missing", func(t *testing.T) {
		d := isolate(t)
		fakeTools(t, nil)
		for _, s := range []*tab.Song{
			{Path: "A/right.gp5", Format: "gp5", Title: "Right"},
			{Path: "A/wrong.gp3", Format: "gp5", Title: "Wrong"},
		} {
			if err := d.openInTuxGuitar(root, s); err == nil || err.Error() != "TuxGuitar is not installed" {
				t.Errorf("%s: err = %v", s.Path, err)
			}
		}
	})
	t.Run("the process is reaped", func(t *testing.T) {
		d := isolate(t)
		dir := fakeTools(t, map[string]string{"tuxguitar": `printf '%s' "$$" > "$0.pid"` + "\n"})
		if err := d.openInTuxGuitar(root, &tab.Song{Path: "A/right.gp5", Format: "gp5", Title: "Right"}); err != nil {
			t.Fatal(err)
		}
		var pid int
		for range 200 {
			if b, err := os.ReadFile(filepath.Join(dir, "tuxguitar.pid")); err == nil && len(b) > 0 {
				pid, _ = strconv.Atoi(string(b))
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if pid == 0 {
			t.Fatal("tuxguitar did not run")
		}
		// An exited child nobody waits for stays in /proc as a zombie.
		for range 200 {
			if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); os.IsNotExist(err) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Errorf("process %d is still listed (zombie?)", pid)
	})
}

// U-DSK-06
func TestPrefersDark(t *testing.T) {
	for _, tt := range []struct {
		name, script     string
		wantDark, wantOK bool
	}{
		{"dark", `printf "'prefer-dark'\n"`, true, true},
		{"default", `printf "'default'\n"`, false, true},
		{"light", `printf "'prefer-light'\n"`, false, true},
		{"empty output", `printf ""`, false, false},
		{"blank output", `printf "  \n"`, false, false},
		{"command fails", `exit 1`, false, false},
	} {
		dir := fakeTools(t, map[string]string{"gsettings": record + tt.script})
		dark, ok := prefersDark()
		if dark != tt.wantDark || ok != tt.wantOK {
			t.Errorf("%s: dark %v ok %v, want %v %v", tt.name, dark, ok, tt.wantDark, tt.wantOK)
		}
		if args := calledWith(t, dir, "gsettings"); !slices.Equal(args, []string{"get", "org.gnome.desktop.interface", "color-scheme"}) {
			t.Errorf("%s: args = %q", tt.name, args)
		}
	}
	fakeTools(t, nil)
	if dark, ok := prefersDark(); dark || ok {
		t.Errorf("no gsettings: dark %v ok %v", dark, ok)
	}
}
