package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"tabfinder/internal/testlib"
)

func reorg(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(tabreorgBin, args...)
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configHome)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		code = ee.ExitCode()
	default:
		t.Fatal(err)
	}
	return out.String(), errb.String(), code
}

// snapshot maps every file under root (relative path) to its MD5.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		m[filepath.ToSlash(rel)] = fmt.Sprintf("%x", md5.Sum(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func hashes(m map[string]string) []string {
	var out []string
	for _, h := range m {
		out = append(out, h)
	}
	slices.Sort(out)
	return out
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var reDate = regexp.MustCompile(`\(\d{4}-\d\d-\d\d\)`)

// normalize makes output independent of the temp dir and the day.
func normalize(s, root string) string {
	s = strings.ReplaceAll(s, root, "<root>")
	return reDate.ReplaceAllString(s, "(<date>)")
}

// plannedMoves reads "src\t->\tdst" lines from tabreorg's stdout.
func plannedMoves(stdout string) map[string]string {
	m := map[string]string{}
	for _, l := range strings.Split(stdout, "\n") {
		if parts := strings.Split(l, "\t->\t"); len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func logs(t *testing.T, root string) (moveLogs, undoScripts []string) {
	t.Helper()
	moveLogs, _ = filepath.Glob(root + "-moves-*.tsv")
	undoScripts, _ = filepath.Glob(root + "-undo-*.sh")
	return
}

// E-REO-01
func TestReorgDryRun(t *testing.T) {
	root := testlib.Tree(t)
	before := snapshot(t, root)
	out, errOut, code := reorg(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Error("a dry run changed the tree")
	}
	if l, u := logs(t, root); len(l)+len(u) != 0 {
		t.Errorf("dry run wrote a log or undo script: %v %v", l, u)
	}
	// Next to the root there is only the summary.
	entries, _ := os.ReadDir(filepath.Dir(root))
	if len(entries) != 2 {
		t.Errorf("files next to the root: %v", entries)
	}
	summary := root + "-reorg-summary.md"
	b := mustRead(t, summary)
	testlib.Golden(t, "summary_dryrun.md", []byte(normalize(string(b), root)))
	testlib.Golden(t, "stdout_dryrun.txt", []byte(normalize(out, root)))

	wantErr := fmt.Sprintf("\n10 moves/renames planned, 1 name clashes; summary: %s\n", summary)
	if errOut != wantErr {
		t.Errorf("stderr = %q\nwant      %q", errOut, wantErr)
	}
	if !strings.Contains(string(b), "10 to move/rename (dry run)") {
		t.Error("summary doesn't say it was a dry run")
	}
}

// E-REO-02, E-REO-03, E-REO-06
func TestReorgApplyUndoAndIdempotence(t *testing.T) {
	root := testlib.Tree(t)
	before := snapshot(t, root)
	dry, _, _ := reorg(t, root)
	plan := plannedMoves(dry)
	if len(plan) != 10 {
		t.Fatalf("plan = %v", plan)
	}

	out, errOut, code := reorg(t, "-apply", root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if got := plannedMoves(out); !mapsEqual(got, plan) {
		t.Errorf("applied moves differ from the dry run:\n%v\n%v", got, plan)
	}
	after := snapshot(t, root)
	for src, dst := range plan {
		if _, ok := after[src]; ok {
			t.Errorf("%s is still there", src)
		}
		if after[dst] != before[src] {
			t.Errorf("%s did not arrive at %s with its content", src, dst)
		}
	}
	// Everything that isn't in the plan stays put and unchanged.
	for p, h := range before {
		if _, moved := plan[p]; !moved && after[p] != h {
			t.Errorf("%s changed", p)
		}
	}
	if len(after) != len(before) || !slices.Equal(hashes(after), hashes(before)) {
		t.Errorf("file count or content set changed: %d -> %d files", len(before), len(after))
	}
	if !strings.Contains(errOut, "10 moves/renames done, 1 name clashes; summary: ") {
		t.Errorf("stderr = %q", errOut)
	}
	logFiles, undoScripts := logs(t, root)
	if len(logFiles) != 1 || len(undoScripts) != 1 {
		t.Fatalf("log %v, undo %v", logFiles, undoScripts)
	}
	for _, f := range []string{logFiles[0], undoScripts[0]} {
		if !strings.Contains(errOut, f) {
			t.Errorf("stderr doesn't name %s: %q", f, errOut)
		}
	}
	logged := map[string]string{}
	for _, l := range strings.Split(strings.TrimSuffix(string(mustRead(t, logFiles[0])), "\n"), "\n") {
		parts := strings.Split(l, "\t")
		if len(parts) != 2 {
			t.Fatalf("log line %q", l)
		}
		logged[parts[0]] = parts[1]
	}
	if !mapsEqual(logged, plan) {
		t.Errorf("log lists %v\nplan %v", logged, plan)
	}
	if st, _ := os.Stat(undoScripts[0]); st.Mode().Perm()&0o100 == 0 {
		t.Errorf("undo script is not executable: %v", st.Mode())
	}
	sb := mustRead(t, root+"-reorg-summary.md")
	testlib.Golden(t, "summary_applied.md", []byte(normalize(string(sb), root)))
	if !strings.Contains(string(sb), "10 moved/renamed") || strings.Contains(string(sb), "dry run") {
		t.Error("summary doesn't say the moves were applied")
	}
	// The summary describes the tree as it is now.
	if !strings.Contains(string(sb), "Soilbed Quartet/Glass Orchard/Brass Kettle (2).gp") {
		t.Error("summary doesn't mention the moved copy")
	}
	stamp := regexp.MustCompile(`-moves-\d{8}-\d{6}\.tsv`)
	undoScript := stamp.ReplaceAllString(string(mustRead(t, undoScripts[0])), "-moves-<stamp>.tsv")
	testlib.Golden(t, "undo.sh", []byte(normalize(undoScript, root)))

	// A second run on the result has nothing to do.
	out, errOut, code = reorg(t, "-apply", root)
	if code != 0 || len(plannedMoves(out)) != 0 {
		t.Errorf("second apply: exit %d, moves %v", code, plannedMoves(out))
	}
	if !strings.Contains(errOut, "0 moves/renames done") {
		t.Errorf("stderr = %q", errOut)
	}
	if l, u := logs(t, root); len(l) != 1 || len(u) != 1 {
		t.Errorf("second apply wrote more logs: %v %v", l, u)
	}
	if !mapsEqual(snapshot(t, root), after) {
		t.Error("second apply changed the tree")
	}
	if !strings.Contains(string(mustRead(t, root+"-reorg-summary.md")), "already organized, nothing to move") {
		t.Error("summary of the second run doesn't say so")
	}

	// The undo script puts every file back. (Folders it made stay, empty.)
	if o, err := exec.Command("sh", undoScripts[0]).CombinedOutput(); err != nil {
		t.Fatalf("undo script: %v\n%s", err, o)
	}
	if restored := snapshot(t, root); !mapsEqual(restored, before) {
		t.Errorf("after the undo script the tree differs:\nnow    %v\nbefore %v", keys(restored), keys(before))
	}
}

// E-REO-04
func TestReorgNoRename(t *testing.T) {
	root := testlib.Tree(t)
	out, _, code := reorg(t, "-rename=false", root)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	plan := plannedMoves(out)
	for src, dst := range plan {
		if path.Base(src) != path.Base(dst) && !strings.HasSuffix(dst, " (2).gp") {
			t.Errorf("%s -> %s renames the file", src, dst)
		}
	}
	want := map[string]string{
		"Amber Marsh/amber_marsh_dawn_of_the_copper_giant.gp3": "Amber Marsh/Dawn of the Copper Giant/amber_marsh_dawn_of_the_copper_giant.gp3",
		"Cousins of Marrow/Quiet Morning.gp3":                  "Cousins of Marrow/Moonbreder/Quiet Morning.gp3",
		"Cousins of Marrow/Tidebloom.gp3":                      "Cousins of Marrow/Moonbreder/Tidebloom.gp3",
		"Die Äther/Felix Ferien - Weniger.gp3":                 "Felix Ferien/Endlich Ferien/Felix Ferien - Weniger.gp3",
		"Soilbed Quartet/Brass Kettle.gp":                      "Soilbed Quartet/Glass Orchard/Brass Kettle (2).gp",
		"Soilbed Quartet - Rust Parade.gp3":                    "Soilbed Quartet/Glass Orchard/Soilbed Quartet - Rust Parade.gp3",
	}
	if !mapsEqual(plan, want) {
		t.Errorf("plan:\n%v\nwant:\n%v", plan, want)
	}
	if strings.Contains(out, "Not renamed") {
		t.Errorf("clashes reported with -rename=false:\n%s", out)
	}
	reorg(t, "-rename=false", "-apply", root)
	after := snapshot(t, root)
	for src, dst := range want {
		if _, ok := after[dst]; !ok {
			t.Errorf("%s did not arrive", dst)
		}
		if _, ok := after[src]; ok {
			t.Errorf("%s is still there", src)
		}
	}
}

// E-REO-05
func TestReorgSkip(t *testing.T) {
	t.Run("default skip list", func(t *testing.T) {
		root := testlib.Tree(t)
		out, _, _ := reorg(t, root)
		for src := range plannedMoves(out) {
			if strings.HasPrefix(src, "Scales/") || strings.HasPrefix(src, "Fit for a Lighthouse/Chart Happens/") {
				t.Errorf("%s is in a skipped folder", src)
			}
		}
		sb := mustRead(t, root+"-reorg-summary.md")
		if !strings.Contains(string(sb), "Left untouched by request: `RS World Tour`, `Rock&Pop`, `Chrismas`, `Guitar`, `Scales`") {
			t.Errorf("summary doesn't list the default folders:\n%s", sb)
		}
	})
	t.Run("custom list replaces the default", func(t *testing.T) {
		root := testlib.Tree(t)
		out, _, _ := reorg(t, "-skip", "Amber Marsh, Die Äther ,,", root)
		plan := plannedMoves(out)
		for src := range plan {
			if strings.HasPrefix(src, "Amber Marsh/") || strings.HasPrefix(src, "Die Äther/") {
				t.Errorf("%s is in a skipped folder", src)
			}
		}
		// Scales is no longer skipped: "major_scale" gets renamed.
		if plan["Scales/major_scale.gp3"] != "Scales/Major Scale.gp3" {
			t.Errorf("Scales/major_scale.gp3 -> %q", plan["Scales/major_scale.gp3"])
		}
		if _, ok := plan["Soilbed Quartet - Rust Parade.gp3"]; !ok {
			t.Error("unrelated moves are gone")
		}
		if sb := mustRead(t, root+"-reorg-summary.md"); !strings.Contains(string(sb), "Left untouched by request: `Amber Marsh`, `Die Äther`.") {
			t.Errorf("summary:\n%s", sb)
		}
	})
	t.Run("-config replaces the config file", func(t *testing.T) {
		root := testlib.Tree(t)
		cfgFile := filepath.Join(t.TempDir(), "other.json")
		if err := os.WriteFile(cfgFile, []byte(`{"skip":["Amber Marsh"]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		out, _, code := reorg(t, "-config", cfgFile, root)
		plan := plannedMoves(out)
		if code != 0 || plan["Scales/major_scale.gp3"] != "Scales/Major Scale.gp3" {
			t.Errorf("exit %d, Scales/major_scale.gp3 -> %q", code, plan["Scales/major_scale.gp3"])
		}
		if sb := mustRead(t, root+"-reorg-summary.md"); !strings.Contains(string(sb), "Left untouched by request: `Amber Marsh`.") {
			t.Errorf("summary:\n%s", sb)
		}
	})
	t.Run("no config file: nothing skipped", func(t *testing.T) {
		root := testlib.Tree(t)
		cmd := exec.Command(tabreorgBin, root)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+t.TempDir())
		out, err := cmd.Output()
		if err != nil || plannedMoves(string(out))["Scales/major_scale.gp3"] == "" {
			t.Errorf("err = %v; Scales is still skipped", err)
		}
	})
	t.Run("bad config file", func(t *testing.T) {
		root := testlib.Tree(t)
		_, errOut, code := reorg(t, "-config", filepath.Join(t.TempDir(), "missing.json"), root)
		if code != 1 || !strings.Contains(errOut, "tabreorg:") {
			t.Errorf("exit %d: %s", code, errOut)
		}
	})
	t.Run("empty list skips nothing", func(t *testing.T) {
		root := testlib.Tree(t)
		out, _, _ := reorg(t, "-skip", "", root)
		if plannedMoves(out)["Scales/major_scale.gp3"] == "" {
			t.Error("Scales is still skipped")
		}
	})
}

func TestReorgSummaryFlag(t *testing.T) {
	root := testlib.Tree(t)
	custom := filepath.Join(t.TempDir(), "my summary.md")
	_, errOut, code := reorg(t, "-summary", custom, root)
	if code != 0 || !strings.Contains(errOut, "summary: "+custom) {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(root + "-reorg-summary.md"); err == nil {
		t.Error("default summary written too")
	}
}

// E-REO-07
func TestReorgArguments(t *testing.T) {
	root := testlib.Tree(t)
	for _, args := range [][]string{{}, {root, root}, {"-apply"}, {"-bogus", root}} {
		out, errOut, code := reorg(t, args...)
		if code != 2 || out != "" || !strings.Contains(errOut, "usage: tabreorg") {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
	missing := filepath.Join(t.TempDir(), "nope")
	out, errOut, code := reorg(t, "-apply", missing)
	if code != 1 || out != "" || !strings.HasPrefix(errOut, "tabreorg: ") {
		t.Errorf("missing root: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if matches, _ := filepath.Glob(missing + "*"); len(matches) != 0 {
		t.Errorf("files written for a missing root: %v", matches)
	}
	// A file is not a root.
	if _, _, code := reorg(t, filepath.Join(root, "mystery_tab.gp3")); code != 1 {
		t.Errorf("file as root: exit %d", code)
	}
}

func TestReorgUnreadableFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	root := testlib.Tree(t)
	locked := filepath.Join(root, "Zzz")
	os.Mkdir(locked, 0o755)
	snap := func() map[string]string {
		os.Chmod(locked, 0o755)
		defer os.Chmod(locked, 0)
		return snapshot(t, root)
	}
	os.Chmod(locked, 0)
	defer os.Chmod(locked, 0o755)
	before := snap()
	_, errOut, code := reorg(t, "-apply", root)
	if code != 1 || !strings.Contains(errOut, locked) {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	// Nothing moves when the scan failed.
	if after := snap(); !mapsEqual(before, after) {
		t.Error("files moved although the scan failed")
	}
	if l, u := logs(t, root); len(l)+len(u) != 0 {
		t.Errorf("log written: %v %v", l, u)
	}
}

func TestApplyMovesRefusesToOverwrite(t *testing.T) {
	root := t.TempDir()
	testlib.WriteFiles(t, root, map[string][]byte{"a/x.tg": []byte("x"), "a/y.tg": []byte("y"), "b/z.tg": []byte("z"), "b/w.tg": []byte("w")})
	err := applyMoves(root, []move{{"a/x.tg", "c/x.tg"}, {"a/y.tg", "b/z.tg"}, {"b/w.tg", "c/w.tg"}})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("err = %v", err)
	}
	if string(mustRead(t, filepath.Join(root, "b/z.tg"))) != "z" {
		t.Error("existing file overwritten")
	}
	// The first move was done; the one that would overwrite and those after it were not.
	snap := snapshot(t, root)
	if _, ok := snap["c/x.tg"]; !ok {
		t.Error("first move not done")
	}
	if _, ok := snap["a/y.tg"]; !ok {
		t.Error("refused file moved")
	}
	if _, ok := snap["b/w.tg"]; !ok {
		t.Error("later move done")
	}
	// The log covers what was done.
	logFiles, undoScripts := logs(t, root)
	if len(logFiles) != 1 || len(undoScripts) != 1 {
		t.Fatalf("logs %v undo %v", logFiles, undoScripts)
	}
	if lb := string(mustRead(t, logFiles[0])); lb != "a/x.tg\tc/x.tg\n" {
		t.Errorf("log = %q", lb)
	}
	if err := applyMoves(root, nil); err != nil {
		t.Errorf("no moves: %v", err)
	}
}

func TestUndoScriptQuoting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "My Tabs's")
	testlib.WriteFiles(t, root, map[string][]byte{
		"A B/it's $HOME `x`.tg": []byte("1"),
		"Die Äther/ä\"ö.tg":     []byte("2"),
		"loose \\n.tg":          []byte("3"),
	})
	before := snapshot(t, root)
	ms := []move{{"A B/it's $HOME `x`.tg", "New Dir/it's $HOME `x`.tg"}, {"Die Äther/ä\"ö.tg", "Neu/x y/ä\"ö.tg"}, {"loose \\n.tg", "Neu/loose \\n.tg"}}
	if err := applyMoves(root, ms); err != nil {
		t.Fatal(err)
	}
	if mapsEqual(snapshot(t, root), before) {
		t.Fatal("nothing moved")
	}
	_, undo := logs(t, root)
	if len(undo) != 1 {
		t.Fatalf("undo scripts %v", undo)
	}
	if o, err := exec.Command("sh", undo[0]).CombinedOutput(); err != nil {
		t.Fatalf("undo: %v\n%s", err, o)
	}
	if after := snapshot(t, root); !mapsEqual(after, before) {
		t.Errorf("after undo: %v", keys(after))
	}
}

// U-REO-06
func TestSummaryCountsMatch(t *testing.T) {
	root := testlib.Tree(t)
	reorg(t, root)
	s := string(mustRead(t, root+"-reorg-summary.md"))
	section := func(heading string) string {
		i := strings.Index(s, "## "+heading)
		if i < 0 {
			t.Fatalf("no section %q", heading)
		}
		rest := s[i+3:]
		if j := strings.Index(rest, "\n## "); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	// "Files without album (N)": N file names are listed.
	noAlbum := section("Files without album")
	var n int
	fmt.Sscanf(strings.TrimPrefix(noAlbum, "Files without album ("), "%d)", &n)
	listed := 0
	for _, l := range strings.Split(noAlbum, "\n") {
		if strings.HasPrefix(l, "- **") {
			listed += strings.Count(l, "`") / 2
		}
	}
	if n != listed || n != 3 {
		t.Errorf("files without album: header says %d, %d listed", n, listed)
	}
	// "Not renamed (N)": N bullets.
	clashes := section("Not renamed")
	var c int
	fmt.Sscanf(strings.TrimPrefix(clashes, "Not renamed: same song name in one folder ("), "%d)", &c)
	if bullets := strings.Count(clashes, "\n- "); c != bullets || c != 1 {
		t.Errorf("clashes: header says %d, %d listed", c, bullets)
	}
	// "Duplicate songs (S songs, F files)": S headings, F table rows.
	dups := section("Duplicate songs")
	var songs, files int
	fmt.Sscanf(strings.TrimPrefix(dups, "Duplicate songs ("), "%d songs, %d files)", &songs, &files)
	rows := 0
	for _, l := range strings.Split(dups, "\n") {
		if strings.HasPrefix(l, "| `") {
			rows++
		}
	}
	if h := strings.Count(dups, "\n### "); songs != h || files != rows || songs != 2 || files != 4 {
		t.Errorf("duplicates: header says %d songs, %d files; %d headings, %d rows", songs, files, h, rows)
	}
	if !strings.Contains(s, "1 groups contain byte-identical files") {
		t.Error("identical group count missing")
	}
}

// U-REO-06: duplicates by MD5 and by song, and the problem of an unreadable one.
func TestSummaryDuplicates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Tabs")
	same := tg("Nectar", "Argyle Moth", "Rent of Summer")
	testlib.WriteFiles(t, root, map[string][]byte{
		// Byte-identical, named differently: grouped by MD5.
		"Argyle Moth/Rent of Summer/one.tg": same,
		"Argyle Moth/Rent of Summer/two.tg": same,
		// The same song twice, different content: grouped by artist and title.
		"Soilbed Quartet/Glass Orchard/Brass Kettle (ver 1).tg": tg("Brass Kettle", "Soilbed Quartet", "Glass Orchard"),
		"Soilbed Quartet/Glass Orchard/Brass Kettle (ver 2).tg": tg("Brass Kettle", "Soilbed Quartet", "Glass Orchard (2005)"),
		// Unparseable copies: grouped by MD5, with the problem noted.
		"Broken/a.gp5": []byte("junk"),
		"Broken/b.gp5": []byte("junk"),
		// Not duplicates.
		"Gorsewick/Gravel Hymns/Quartz.tg":    tg("Quartz", "Gorsewick", "Gravel Hymns"),
		"Gorsewick/Gravel Hymns/Amberwood.tg": tg("Amberwood", "Gorsewick", "Gravel Hymns"),
	})
	if _, _, code := reorg(t, root); code != 0 {
		t.Fatalf("exit %d", code)
	}
	s := string(mustRead(t, root+"-reorg-summary.md"))
	// Paths in the summary are the ones after the reorganization (renamed after their song).
	for _, want := range []string{
		"## Duplicate songs (3 songs, 6 files)",
		"2 groups contain byte-identical files",
		"### Argyle Moth – Nectar",
		"### Soilbed Quartet – Brass Kettle",
		"### Broken – A",
		"`Argyle Moth/Rent of Summer/One.tg` | tg | 0 KB | 0 | ? | **identical** |",
		"`Broken/A.gp5` | ? | 0 KB | 0 | ? | **identical** ⚠ unknown file format |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("summary lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "Quartz") || strings.Contains(s, "Amberwood") {
		t.Error("distinct songs listed as duplicates")
	}
	// The Brass Kettle group isn't marked identical.
	group := s[strings.Index(s, "### Soilbed Quartet – Brass Kettle"):]
	if end := strings.Index(group[1:], "###"); end >= 0 {
		group = group[:1+end]
	}
	if !strings.Contains(group, "Brass Kettle (ver 1).tg") || strings.Contains(group, "identical") {
		t.Errorf("Brass Kettle group:\n%s", group)
	}
}

// With no folders to leave alone, the summary doesn't say it left nothing alone.
func TestSummaryWithoutSkip(t *testing.T) {
	root := testlib.Tree(t)
	if _, stderr, code := reorg(t, "-skip=", root); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	s := string(mustRead(t, root+"-reorg-summary.md"))
	if strings.Contains(s, "Left untouched") {
		t.Errorf("summary mentions skipped folders:\n%s", s[:min(len(s), 300)])
	}
	if !strings.Contains(s, "tab files scanned") || !strings.Contains(s, "## Files without album") {
		t.Errorf("summary incomplete:\n%s", s[:min(len(s), 300)])
	}
}
