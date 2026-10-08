package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/tab"
	"tabfinder/internal/tabfiles"
	"tabfinder/internal/testlib"
)

func TestTSVWriter(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	emit := tsvWriter(w)
	emit(&tab.Song{
		Path: "A/B/c.gp5", Artist: "Art\tist", Album: "Al\nbum", Title: "Ti\r\ntle",
		Tempos: []tab.Tempo{{Bar: 1, BPM: 120}, {Bar: 9, BPM: 90.5}, {Bar: 17, BPM: 120}},
		Tracks: []tab.Track{
			{Name: "Lead", Instrument: "Distortion Guitar", Pitches: testlib.DropC6.Pitches},
			{Name: "Bass", Instrument: "bass", Pitches: testlib.Bass4.Pitches}, // same as the name, ignoring case
			{Name: "Drums", Instrument: "Drums", Drums: true},
			{Name: "Voice"}, // no instrument at all
			{Name: "Keys", Instrument: "Piano\tx"},
		},
		Parts: []difficulty.Part{
			{Role: difficulty.Drums, Score: 2.6},
			{Role: difficulty.Rhythm, Score: 7.5, Tags: []string{"fast", "triplets"}},
		},
	})
	emit(&tab.Song{Path: "x.gp3"}) // nothing known
	w.Flush()
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", lines)
	}
	want := strings.Join([]string{
		"A/B/c.gp5", "Art ist", "Al bum", "Ti  tle", "120, 90.5",
		"Lead [Distortion Guitar]; Bass; Drums; Voice; Keys [Piano x]",
		"Drop C (C G C F A D); E Standard (E A D G); -; -; -",
		"drums 3, rhythm 8", "rhythm: fast, triplets",
	}, "\t")
	if lines[0] != want {
		t.Errorf("row =\n%q\nwant\n%q", lines[0], want)
	}
	if lines[1] != "x.gp3\t\t\t\t\t\t\t\t" {
		t.Errorf("empty row = %q", lines[1])
	}
	for _, l := range lines {
		if n := strings.Count(l, "\t"); n != 8 {
			t.Errorf("%d tabs in %q", n, l)
		}
	}
}

// run runs the built tabscan and returns stdout, stderr and the exit code.
func run(t *testing.T, dir string, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(tabscanBin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
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

// paths lists the first column of TSV output, without the header.
func paths(tsv string) []string {
	var out []string
	for i, l := range strings.Split(strings.TrimSuffix(tsv, "\n"), "\n") {
		if i == 0 || l == "" {
			continue
		}
		out = append(out, strings.SplitN(l, "\t", 2)[0])
	}
	return out
}

func TestCLIGolden(t *testing.T) {
	root := testlib.Tree(t)
	t.Run("tsv", func(t *testing.T) {
		out, errOut, code := run(t, "", "", root)
		if code != 0 {
			t.Errorf("exit %d", code)
		}
		if errOut != "warn: Broken/garbled.gp5: unknown file format\n" {
			t.Errorf("stderr = %q", errOut)
		}
		testlib.Golden(t, "tree.tsv", []byte(out))
		if !strings.HasPrefix(out, "path\tartist\talbum\ttitle\ttempo\tinstruments\ttunings\tdifficulty\ttags\n") {
			t.Errorf("header = %q", strings.SplitN(out, "\n", 2)[0])
		}
	})
	t.Run("json", func(t *testing.T) {
		out, _, code := run(t, "", "", "-json", root)
		if code != 0 {
			t.Errorf("exit %d", code)
		}
		testlib.Golden(t, "tree.jsonl", []byte(out))
		sc := bufio.NewScanner(strings.NewReader(out))
		n := 0
		for sc.Scan() {
			n++
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Fatalf("line %d: %v", n, err)
			}
			for _, k := range []string{"path", "format", "artist", "album", "title", "artistSource", "albumSource", "titleSource"} {
				if _, ok := m[k]; !ok {
					t.Errorf("line %d: no %q: %s", n, k, sc.Text())
				}
			}
		}
		if n != 20 {
			t.Errorf("%d lines, want 20", n)
		}
		// Per-track details and tempos for a song that has them.
		var overload tab.Song
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, `{"path":"Soilbed Quartet/Glass Orchard/Brass Kettle.gp"`) {
				json.Unmarshal([]byte(l), &overload)
			}
		}
		if overload.Format != "gp7" || len(overload.Tracks) != 1 || overload.Tracks[0].Tuning().String() != "Drop C (C G C F A D)" || len(overload.Tempos) != 2 || overload.Tempos[1] != (tab.Tempo{Bar: 91, BPM: 145}) {
			t.Errorf("overload = %+v", overload)
		}
		// HTML isn't escaped, non-ASCII stays as is.
		if !strings.Contains(out, `"artist":"Die Äther"`) {
			t.Error("non-ASCII artist escaped or missing")
		}
	})
}

func TestCLIFilters(t *testing.T) {
	root := testlib.Tree(t)
	const total = 20
	tests := []struct {
		name string
		args []string
		want []string // paths
	}{
		{"name", []string{"-name", "brass"}, []string{"Soilbed Quartet/Brass Kettle.gp", "Soilbed Quartet/Glass Orchard/Brass Kettle.gp"}},
		{"name words in any order", []string{"-name", "daydream enter"}, []string{"Metro Kettle/Tan Album/Enter Daydream (ver 1).gp3", "Metro Kettle/Tan Album/Enter Daydream (ver 2).gp3"}},
		{"name from the file name", []string{"-name", "ver 2"}, []string{"Metro Kettle/Tan Album/Enter Daydream (ver 2).gp3"}},
		{"artist", []string{"-artist", "marrow"}, []string{"Cousins of Marrow/Quiet Morning.gp3", "Cousins of Marrow/Tidebloom.gp3"}},
		{"tuning by name", []string{"-tuning", "drop c"}, []string{
			"Argyle Moth/argyle_moth_gluttonous.gp3", "Soilbed Quartet/Brass Kettle.gp", "Soilbed Quartet/Glass Orchard/Brass Kettle.gp", "Soilbed Quartet - Rust Parade.gp3"}},
		{"tuning by notes", []string{"-tuning", "d g d g b d"}, []string{"Felix Ferien/Endlich Ferien/Klug.gp3"}},
		{"tuning enharmonic", []string{"-tuning", "drop db"}, nil},
		{"tuning b standard", []string{"-tuning", "b standard"}, []string{"Metro Kettle/Tan Album/Enter Daydream (ver 2).gp3"}},
		{"bpm exact", []string{"-bpm", "190"}, []string{"Cousins of Marrow/Quiet Morning.gp3", "Soilbed Quartet/Brass Kettle.gp", "Soilbed Quartet/Glass Orchard/Brass Kettle.gp", "Soilbed Quartet - Rust Parade.gp3"}},
		{"bpm range", []string{"-bpm", "120-125"}, []string{
			"Amber Marsh/Tide of Lanterns/Where Rivers Seem to Rest.gp3", "Gorsewick/Gravel Hymns/Quartz.gpx.crdownload",
			"Metro Kettle/Tan Album/Enter Daydream (ver 1).gp3", "Metro Kettle/Tan Album/Enter Daydream (ver 2).gp3"}},
		{"bpm at least", []string{"-bpm", "190-"}, []string{
			"Cousins of Marrow/Quiet Morning.gp3", "Cousins of Marrow/Tidebloom.gp3", "Soilbed Quartet/Brass Kettle.gp", "Soilbed Quartet/Glass Orchard/Brass Kettle.gp", "Soilbed Quartet - Rust Parade.gp3"}},
		{"bpm at most", []string{"-bpm", "-80"}, []string{"Fit for a Lighthouse/Chart Happens/x.gp3", "Scales/major_scale.gp3"}},
		{"bpm tempo change", []string{"-bpm", "145"}, []string{"Soilbed Quartet/Brass Kettle.gp", "Soilbed Quartet/Glass Orchard/Brass Kettle.gp"}},
		{"combined", []string{"-artist", "soilbed quartet", "-tuning", "drop c", "-bpm", "145", "-name", "brass"}, []string{"Soilbed Quartet/Brass Kettle.gp", "Soilbed Quartet/Glass Orchard/Brass Kettle.gp"}},
		{"combined, none", []string{"-artist", "soilbed quartet", "-bpm", "100"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errOut, code := run(t, "", "", append(slices.Clone(tt.args), root)...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errOut)
			}
			slices.Sort(tt.want)
			if got := paths(out); !slices.Equal(sortedCopy(got), tt.want) && !(len(got) == 0 && len(tt.want) == 0) {
				t.Errorf("paths = %q\nwant    %q", got, tt.want)
			}
			wantLine := strings.Join([]string{strconv.Itoa(len(tt.want)), "of", strconv.Itoa(total), "tabs match"}, " ")
			if !strings.Contains(errOut, wantLine+"\n") {
				t.Errorf("stderr = %q, want it to say %q", errOut, wantLine)
			}
		})
	}
	t.Run("no filter, no count", func(t *testing.T) {
		_, errOut, _ := run(t, "", "", root)
		if strings.Contains(errOut, "tabs match") {
			t.Errorf("stderr = %q", errOut)
		}
	})
	t.Run("json is filtered too", func(t *testing.T) {
		out, _, _ := run(t, "", "", "-json", "-artist", "felix", root)
		if n := strings.Count(out, "\n"); n != 2 {
			t.Errorf("%d lines: %s", n, out)
		}
	})
}

func sortedCopy(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

func TestCLIBadBPM(t *testing.T) {
	root := testlib.Tree(t)
	for _, bpm := range []string{"fast", "140-100", "-", "1-2-3"} {
		out, errOut, code := run(t, "", "", "-bpm", bpm, root)
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "tabscan: invalid bpm") {
			t.Errorf("-bpm %q: exit %d, stdout %q, stderr %q", bpm, code, out, errOut)
		}
	}
}

func TestCLIManyArguments(t *testing.T) {
	root := testlib.Tree(t)
	missing := filepath.Join(root, "nope")
	out, errOut, code := run(t, "", "", filepath.Join(root, "Soilbed Quartet"), missing, filepath.Join(root, "Gorsewick"))
	if code != 1 {
		t.Errorf("exit %d", code)
	}
	// The existing ones are printed, in argument order, under one header.
	want := []string{"Brass Kettle.gp", "Glass Orchard/Brass Kettle.gp", "Gravel Hymns/Quartz.gpx.crdownload"}
	if got := paths(out); !slices.Equal(got, want) {
		t.Errorf("paths = %q, want %q", got, want)
	}
	if strings.Count(out, "path\tartist") != 1 {
		t.Errorf("header repeated: %s", out)
	}
	if !strings.Contains(errOut, "tabscan: ") || !strings.Contains(errOut, "nope") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCLISingleFileAndRoot(t *testing.T) {
	root := testlib.Tree(t)
	file := filepath.Join(root, "Argyle Moth", "argyle_moth_gluttonous.gp3")
	t.Run("file alone: its directory is the root", func(t *testing.T) {
		out, _, code := run(t, "", "", file)
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		row := strings.Split(strings.Split(out, "\n")[1], "\t")
		if row[0] != "argyle_moth_gluttonous.gp3" || row[1] != "" || row[2] != "" || row[3] != "Argyle Moth Gluttonous" { // no artist known, so none is cut off the title
			t.Errorf("row = %q", row)
		}
	})
	t.Run("-root restores the Artist/Album fallback", func(t *testing.T) {
		out, _, _ := run(t, "", "", "-root", root, file)
		row := strings.Split(strings.Split(out, "\n")[1], "\t")
		if row[0] != "Argyle Moth/argyle_moth_gluttonous.gp3" || row[1] != "Argyle Moth" || row[3] != "Gluttonous" {
			t.Errorf("row = %q", row)
		}
	})
	t.Run("-root on a directory", func(t *testing.T) {
		out, _, _ := run(t, "", "", "-root", root, filepath.Join(root, "Gorsewick"))
		if got := paths(out); !slices.Equal(got, []string{"Gorsewick/Gravel Hymns/Quartz.gpx.crdownload"}) {
			t.Errorf("paths = %q", got)
		}
	})
	t.Run("directory without -root is its own root", func(t *testing.T) {
		out, _, _ := run(t, "", "", filepath.Join(root, "Gorsewick"))
		row := strings.Split(strings.Split(out, "\n")[1], "\t")
		if row[0] != "Gravel Hymns/Quartz.gpx.crdownload" {
			t.Errorf("row = %q", row)
		}
	})
}

func TestCLIDefaultsToCurrentDirectory(t *testing.T) {
	root := testlib.Tree(t)
	out, _, code := run(t, filepath.Join(root, "Gorsewick"), "")
	if code != 0 || !slices.Equal(paths(out), []string{"Gravel Hymns/Quartz.gpx.crdownload"}) {
		t.Errorf("exit %d, paths %q", code, paths(out))
	}
	// An empty directory prints just the header.
	out, _, code = run(t, t.TempDir(), "")
	if code != 0 || out != "path\tartist\talbum\ttitle\ttempo\tinstruments\ttunings\tdifficulty\ttags\n" {
		t.Errorf("empty dir: exit %d, stdout %q", code, out)
	}
}

func TestCLIWarnings(t *testing.T) {
	root := t.TempDir()
	testlib.WriteFiles(t, root, map[string][]byte{
		"Art/Alb/ok.gp3":       tabfiles.GP(tabfiles.GPSpec{Version: "3.00", Title: "OK", Tempo: 120, Tracks: []tabfiles.GPTrack{{Name: "G", Strings: tabfiles.StdGuitar}}}),
		"Art/Alb/broken.gp5":   []byte("junk"),
		"Art/Alb/empty.gp4":    {},
		"Art/Alb/in a zip.zip": tabfiles.Zip(map[string][]byte{"readme.txt": []byte("x")}),
	})
	out, errOut, code := run(t, "", "", root)
	if code != 0 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{
		"warn: Art/Alb/broken.gp5: unknown file format\n",
		"warn: Art/Alb/empty.gp4: empty file\n",
		"warn: Art/Alb/in a zip.zip: zip contains no tab file\n",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
	if strings.Contains(errOut, "ok.gp3") {
		t.Errorf("warning for the good file: %s", errOut)
	}
	// The rows are still printed, with the path fallbacks.
	if got := paths(out); !slices.Equal(got, []string{"Art/Alb/broken.gp5", "Art/Alb/empty.gp4", "Art/Alb/in a zip.zip", "Art/Alb/ok.gp3"}) {
		t.Errorf("paths = %q", got)
	}
	if !strings.Contains(out, "Art/Alb/broken.gp5\tArt\tAlb\tBroken\t") {
		t.Errorf("fallbacks missing:\n%s", out)
	}
	// Only matching files warn.
	_, errOut, _ = run(t, "", "", "-name", "ok.gp3", root)
	if strings.Contains(errOut, "warn:") {
		t.Errorf("warned about a filtered-out file: %s", errOut)
	}
}

func TestCLIUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	root := testlib.Tree(t)
	locked := filepath.Join(root, "Zzz")
	os.Mkdir(locked, 0o755)
	os.Chmod(locked, 0)
	defer os.Chmod(locked, 0o755)
	out, errOut, code := run(t, "", "", root)
	if code != 1 || !strings.Contains(errOut, locked) {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	if len(paths(out)) == 0 {
		t.Error("nothing printed before the error")
	}
}
