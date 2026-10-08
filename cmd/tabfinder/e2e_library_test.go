package main

import (
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"tabfinder/internal/finder"
	"tabfinder/internal/tabfiles"
	"tabfinder/internal/testlib"
)

// chooser is a fake kdialog that "picks" dir.
func chooser(dir string) map[string]string {
	return map[string]string{"kdialog": record + `printf '` + dir + `\n'`, "gsettings": `printf "'prefer-dark'\n"`}
}

// blockedTree is a folder whose scan waits until release() is called: one of its
// tab files is a named pipe, which can't be read until something writes to it.
func blockedTree(t *testing.T) (root string, release func()) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "Tabs")
	testlib.WriteFiles(t, root, map[string][]byte{
		"Soilbed Quartet/Brass Kettle.tg": tabfiles.TG1("Brass Kettle", "Soilbed Quartet", "Glass Orchard"),
	})
	pipe := filepath.Join(root, "Soilbed Quartet", "wait.tg")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip("no named pipes here:", err)
	}
	released := false
	release = func() {
		if released {
			return
		}
		released = true
		f, err := os.OpenFile(pipe, os.O_WRONLY, 0)
		if err == nil {
			f.Write(tabfiles.TG1("Waiting", "Soilbed Quartet", ""))
			f.Close()
		}
	}
	t.Cleanup(release)
	return root, release
}

var reScanned = regexp.MustCompile(`^(\d+) tabs, (\d+) unreadable, in \d+\.\d s$`)

// E-DSK-01
func TestFirstRunChoosesFolder(t *testing.T) {
	root := testlib.Tree(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	h := newHarnessWith(t, harnessOpts{noConfig: true, tools: chooser(root)})
	oldIndex := h.u.dirs.index()
	if h.u.root != "" {
		t.Fatalf("root = %q with no config", h.u.root)
	}
	// The prompt is shown: its button is on screen, and the list isn't.
	cta := h.buttonRect(h.bodyArea(), &h.u.cta)
	// The folder button works; rescan is disabled without a folder.
	folder := h.buttonRect(h.topBarArea(), &h.u.folderBtn)
	rescan := folder.Add(image.Pt(folder.Dx(), 0))
	// (A disabled button still reports hovering; what matters is that clicking does nothing.)
	h.clickRect(rescan)
	if h.u.scanning {
		t.Error("rescan without a folder started a scan")
	}

	h.clickRect(cta) // "Choose folder"
	args := calledWith(t, h.toolDir, "kdialog")
	if args[2] != "--getexistingdirectory" || args[3] != home {
		t.Errorf("kdialog args = %q (the start folder is the home folder)", args)
	}
	h.waitFor("the scan", func() bool { return h.u.root == root && !h.u.scanning && len(h.u.lib.Entries) == 20 })
	if got, _ := h.u.dirs.loadRoot(); got != root {
		t.Errorf("saved root = %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(h.u.dirs.config, "config.json")); !strings.Contains(string(b), root) {
		t.Errorf("config.json = %s", b)
	}
	// The old index is gone, the new scan is cached.
	if b, _ := os.ReadFile(oldIndex); strings.Contains(string(b), "Soilbed Quartet/Brass Kettle.gp5") || strings.Count(string(b), "\n") != 21 { // the header, 20 songs
		t.Errorf("index has %d lines, or still the old content", strings.Count(string(b), "\n"))
	}
	m := reScanned.FindStringSubmatch(h.u.message)
	if m == nil || m[1] != "20" || m[2] != "1" {
		t.Errorf("message = %q", h.u.message)
	}
	if got := h.u.subtitle(); got != "20 TABS IN TABS" {
		t.Errorf("subtitle = %q", got)
	}
	if len(h.u.result.Matches) != 20 {
		t.Errorf("%d matches", len(h.u.result.Matches))
	}
	h.rowRect(0) // the list is on screen
}

// E-DSK-02
func TestCancelFolderDialog(t *testing.T) {
	h := newHarnessWith(t, harnessOpts{noConfig: true, tools: map[string]string{"kdialog": record + "exit 1", "gsettings": `printf "'prefer-dark'\n"`}})
	before := len(h.u.lib.Entries)
	h.clickRect(h.buttonRect(h.bodyArea(), &h.u.cta))
	calledWith(t, h.toolDir, "kdialog")
	for range 20 {
		h.frame()
		time.Sleep(5 * time.Millisecond)
	}
	if h.u.root != "" || h.u.scanning || h.u.message != "" || len(h.u.lib.Entries) != before {
		t.Errorf("state changed: root %q scanning %v message %q entries %d", h.u.root, h.u.scanning, h.u.message, len(h.u.lib.Entries))
	}
	if _, err := os.Stat(filepath.Join(h.u.dirs.config, "config.json")); err == nil {
		t.Error("config.json written")
	}
	if _, err := os.Stat(h.u.dirs.index()); err != nil {
		t.Error("index deleted")
	}
}

// E-DSK-03
func TestNoDialogProgram(t *testing.T) {
	h := newHarnessWith(t, harnessOpts{noConfig: true, tools: map[string]string{"gsettings": `printf "'prefer-dark'\n"`}})
	h.clickRect(h.buttonRect(h.bodyArea(), &h.u.cta))
	h.waitFor("the message", func() bool { return h.u.message != "" })
	if !containsAll(h.u.message, "install", "kdialog", "zenity") {
		t.Errorf("message = %q", h.u.message)
	}
	if h.u.root != "" || h.u.scanning {
		t.Error("state changed")
	}
}

// E-DSK-05
func TestCachedIndexShownWithoutScanning(t *testing.T) {
	h := newHarnessWith(t, harnessOpts{root: str("/home/me/Guitar/Tabs")})
	if h.u.scanning {
		t.Error("scanning although there is a cached index")
	}
	if len(h.u.lib.Entries) != 4 || len(h.u.result.Matches) != 4 {
		t.Errorf("%d entries, %d matches", len(h.u.lib.Entries), len(h.u.result.Matches))
	}
	if got := h.u.subtitle(); got != "4 TABS IN TABS" {
		t.Errorf("subtitle = %q", got)
	}
	if got := h.u.counter(); got != "4 / 4" {
		t.Errorf("counter = %q", got)
	}
	h.rowRect(0)
	// The folder's name, not its path; trailing slashes don't matter.
	h.u.root = "/home/me/Gitarre/"
	if got := h.u.subtitle(); got != "4 TABS IN GITARRE" {
		t.Errorf("subtitle = %q", got)
	}
}

// E-DSK-06
func TestAutomaticScanWithProgress(t *testing.T) {
	root, release := blockedTree(t)
	h := newHarnessWith(t, harnessOpts{root: &root, noIndex: true, waitLoad: true})
	for i := 0; i < 300 && !h.u.loaded; i++ {
		h.frame()
		time.Sleep(5 * time.Millisecond)
	}
	if !h.u.loaded || !h.u.scanning {
		t.Fatalf("loaded %v scanning %v: no automatic scan", h.u.loaded, h.u.scanning)
	}
	// While it runs: the rescan button is disabled, and frames keep being requested for the progress bar.
	folder := h.buttonRect(h.topBarArea(), &h.u.folderBtn)
	rescan := folder.Add(image.Pt(folder.Dx(), 0))
	h.clickRect(rescan) // disabled: no effect
	if !h.u.scanning {
		t.Fatal("scan ended")
	}
	for range 5 {
		h.advance(300 * time.Millisecond)
	}
	if !h.u.scanning {
		t.Fatal("scan ended early")
	}
	if len(h.u.result.Matches) != 0 {
		t.Error("songs listed before the scan is done")
	}
	release()
	h.waitFor("the scan to finish", func() bool { return !h.u.scanning })
	h.waitFor("the list", func() bool { return len(h.u.result.Matches) == 2 })
	h.rowRect(0)
}

// E-DSK-07
func TestEmptyFolder(t *testing.T) {
	root := t.TempDir()
	h := newHarnessWith(t, harnessOpts{root: &root, noIndex: true})
	h.waitFor("the scan", func() bool { return !h.u.scanning && h.u.message != "" })
	if !strings.HasPrefix(h.u.message, "0 tabs, 0 unreadable") {
		t.Errorf("message = %q", h.u.message)
	}
	if len(h.u.result.Matches) != 0 || h.u.in.Active() {
		t.Fatal("expected the empty prompt")
	}
	// "No tabs found" offers Rescan: a tab added meanwhile shows up.
	testlib.WriteFiles(t, root, map[string][]byte{"New/new_song.tg": tabfiles.TG1("New Song", "New", "")})
	h.clickRect(h.buttonRect(h.bodyArea(), &h.u.cta))
	h.waitFor("the rescan", func() bool { return len(h.u.result.Matches) == 1 })
	if got := h.titles(); len(got) != 1 || got[0] != "New Song" {
		t.Errorf("titles = %q", got)
	}
}

// E-DSK-08
func TestCorruptIndex(t *testing.T) {
	root, release := blockedTree(t) // keeps the automatic rescan from replacing the message at once
	h := newHarnessWith(t, harnessOpts{root: &root, index: str(finder.IndexHeader + "\n{\"path\":\"a\"}\nthis is not json\n")})
	if !strings.HasPrefix(h.u.message, "Couldn't read the saved scan") {
		t.Errorf("message = %q", h.u.message)
	}
	// The app is usable: fields take input, the scan carries on.
	h.typeInto(&h.u.name, "bras")
	if h.u.in.Name != "bras" {
		t.Errorf("typed: %q", h.u.in.Name)
	}
	release()
	h.waitFor("the scan", func() bool { return !h.u.scanning && len(h.u.lib.Entries) == 2 })
	if got := h.titles(); len(got) != 1 || got[0] != "Brass Kettle" {
		t.Errorf("titles = %q", got)
	}
}

// E-DSK-09
func TestRescanButton(t *testing.T) {
	root := testlib.Tree(t)
	h := newHarnessWith(t, harnessOpts{root: &root})
	if len(h.u.lib.Entries) != 4 {
		t.Fatalf("%d entries from the cached index", len(h.u.lib.Entries))
	}
	h.clickRect(h.buttonRect(h.topBarArea(), &h.u.rescanBtn))
	if !h.u.scanning {
		t.Error("clicking Rescan didn't start a scan")
	}
	h.waitFor("the scan", func() bool { return !h.u.scanning && len(h.u.lib.Entries) == 20 })
	if got := h.u.counter(); got != "20 / 20" {
		t.Errorf("counter = %q", got)
	}
	if m := reScanned.FindStringSubmatch(h.u.message); m == nil || m[1] != "20" || m[2] != "1" {
		t.Errorf("snackbar = %q", h.u.message)
	}
	if got := h.u.subtitle(); got != "20 TABS IN TABS" {
		t.Errorf("subtitle = %q", got)
	}
	// The scan is cached for the next start.
	if b, _ := os.ReadFile(h.u.dirs.index()); strings.Count(string(b), "\n") != 21 { // the header, 20 songs
		t.Errorf("index has %d lines", strings.Count(string(b), "\n"))
	}
	// A scan of a folder that is gone says so and keeps the list.
	h.u.root = filepath.Join(root, "gone")
	h.u.rescan()
	h.waitFor("the failure", func() bool { return !h.u.scanning })
	if !strings.HasPrefix(h.u.message, "Scan failed: ") || len(h.u.lib.Entries) != 20 {
		t.Errorf("message %q, %d entries", h.u.message, len(h.u.lib.Entries))
	}
}

// A config.json that can't be read is said, not taken for "no folder chosen" in silence.
func TestCorruptConfigIsReported(t *testing.T) {
	h := newHarnessWith(t, harnessOpts{noConfig: true, config: "{not json"})
	h.frame()
	if h.u.root != "" || !strings.HasPrefix(h.u.message, "Couldn't read the settings: ") || !strings.Contains(h.u.message, "config.json") {
		t.Errorf("root %q, message %q", h.u.root, h.u.message)
	}
}

// Choosing another folder drops the old songs' row state with them.
func TestChooseAnotherFolder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Other")
	testlib.WriteFiles(t, root, map[string][]byte{"Zed Quill/Only One.tg": tabfiles.TG1("Only One", "Zed Quill", "")})
	h := newHarnessWith(t, harnessOpts{tools: chooser(root), waitLoad: true})
	h.frame()
	if len(h.u.rows) == 0 {
		t.Fatal("no rows drawn for the old folder")
	}
	h.clickRect(h.buttonRect(h.topBarArea(), &h.u.folderBtn))
	h.waitFor("the new folder's scan", func() bool { return h.u.root == root && !h.u.scanning && len(h.u.lib.Entries) == 1 })
	h.frame()
	for path := range h.u.rows {
		if path != "Zed Quill/Only One.tg" {
			t.Errorf("row state kept for %s of the old folder", path)
		}
	}
}
