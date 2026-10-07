package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"tabfinder/internal/finder"
	"tabfinder/internal/tab"
	"tabfinder/internal/testlib"
)

// session runs serve over the request lines and returns one decoded response per line.
func session(t testing.TB, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var resps []map[string]any
	sc := bufio.NewScanner(&out)
	sc.Buffer(nil, 1<<26)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("response %q: %v", sc.Text(), err)
		}
		resps = append(resps, m)
	}
	if len(resps) != len(lines) {
		t.Fatalf("%d responses for %d requests", len(resps), len(lines))
	}
	return resps
}

func req(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func list(t testing.TB, m map[string]any, key string) []any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("no %q in %v", key, m)
	}
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("%q = %#v, want a list", key, v)
	}
	return l
}

func TestServeRequests(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "index.jsonl")
	testlib.WriteIndex(t, index, testlib.Songs())

	t.Run("load with a missing index", func(t *testing.T) {
		r := session(t, req(map[string]any{"op": "load", "index": filepath.Join(dir, "nope.jsonl")}))
		if got := list(t, r[0], "songs"); len(got) != 0 || len(r[0]) != 1 {
			t.Errorf("response = %v", r[0])
		}
	})
	t.Run("load", func(t *testing.T) {
		r := session(t, req(map[string]any{"op": "load", "index": index}))
		songs := list(t, r[0], "songs")
		if len(songs) != len(testlib.Songs()) {
			t.Errorf("%d songs", len(songs))
		}
		if _, ok := r[0]["warning"]; ok {
			t.Error("warning on load")
		}
		if _, ok := r[0]["unreadable"]; ok {
			t.Error("unreadable on load")
		}
	})
	t.Run("load of a malformed index", func(t *testing.T) {
		bad := filepath.Join(dir, "bad.jsonl")
		os.WriteFile(bad, []byte("{\"path\":\"a\"}\nnope\n"), 0o644)
		r := session(t, req(map[string]any{"op": "load", "index": bad}), req(map[string]any{"op": "search", "query": map[string]any{}}))
		if _, ok := r[0]["error"].(string); !ok {
			t.Errorf("response = %v", r[0])
		}
		// The failed load leaves the library as it was: empty.
		if got := list(t, r[1], "matches"); len(got) != 0 {
			t.Errorf("matches after a failed load = %v", got)
		}
	})
	t.Run("scan of an empty dir", func(t *testing.T) {
		r := session(t, req(map[string]any{"op": "scan", "root": t.TempDir(), "index": filepath.Join(t.TempDir(), "i.jsonl")}))
		if got := list(t, r[0], "songs"); len(got) != 0 {
			t.Errorf("songs = %v", got)
		}
		if _, ok := r[0]["warning"]; ok {
			t.Errorf("warning = %v", r[0]["warning"])
		}
	})
	t.Run("scan counts unreadable files", func(t *testing.T) {
		root := testlib.Tree(t)
		idx := filepath.Join(t.TempDir(), "i.jsonl")
		r := session(t, req(map[string]any{"op": "scan", "root": root, "index": idx}))
		if got := list(t, r[0], "songs"); len(got) != 20 {
			t.Errorf("%d songs", len(got))
		}
		if r[0]["unreadable"] != float64(1) {
			t.Errorf("unreadable = %v", r[0]["unreadable"])
		}
		if _, err := os.Stat(idx); err != nil {
			t.Error("index not written")
		}
		// Sorted by artist, then title, ignoring case (the file at the root has no artist, so it is first).
		songs := list(t, r[0], "songs")
		key := func(i int) string {
			m := songs[i].(map[string]any)
			return strings.ToLower(m["artist"].(string)) + "\x00" + strings.ToLower(m["title"].(string))
		}
		for i := 1; i < len(songs); i++ {
			if key(i-1) > key(i) {
				t.Errorf("song %d (%q) sorts before song %d (%q)", i, key(i), i-1, key(i-1))
			}
		}
		if songs[0].(map[string]any)["path"] != "mystery_tab.gp3" {
			t.Errorf("first = %v", songs[0])
		}
	})
	t.Run("scan with an unreadable subdirectory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		root := testlib.Tree(t)
		locked := filepath.Join(root, "Zzz")
		os.Mkdir(locked, 0o755)
		os.Chmod(locked, 0)
		defer os.Chmod(locked, 0o755)
		r := session(t, req(map[string]any{"op": "scan", "root": root, "index": filepath.Join(t.TempDir(), "i.jsonl")}))
		w, _ := r[0]["warning"].(string)
		if !strings.Contains(w, locked) || len(list(t, r[0], "songs")) == 0 {
			t.Errorf("response = %v", r[0])
		}
	})
	t.Run("scan of a missing root is an error", func(t *testing.T) {
		r := session(t, req(map[string]any{"op": "scan", "root": filepath.Join(dir, "nope"), "index": filepath.Join(t.TempDir(), "i.jsonl")}))
		if e, _ := r[0]["error"].(string); e == "" {
			t.Errorf("response = %v", r[0])
		}
	})
	t.Run("search before any load", func(t *testing.T) {
		r := session(t, req(map[string]any{"op": "search", "query": map[string]any{"artist": "x", "tuning": "y"}}))
		if _, ok := r[0]["error"]; ok {
			t.Fatalf("error: %v", r[0])
		}
		for _, k := range []string{"matches", "artists", "tunings"} {
			if got := list(t, r[0], k); len(got) != 0 {
				t.Errorf("%s = %v", k, got)
			}
		}
		if r[0]["bpmInvalid"] != false {
			t.Errorf("bpmInvalid = %v", r[0]["bpmInvalid"])
		}
	})
	t.Run("search without a query", func(t *testing.T) {
		r := session(t, req(map[string]any{"op": "load", "index": index}), `{"op":"search"}`)
		if got := list(t, r[1], "matches"); len(got) != len(testlib.Songs()) {
			t.Errorf("%d matches", len(got))
		}
	})
	t.Run("malformed lines don't end the session", func(t *testing.T) {
		r := session(t, "not json", `{"op":`, `[1,2]`, `{"op":5}`, `null`, `"str"`, req(map[string]any{"op": "load", "index": index}), `{"op":"search","query":{"strings":"six"}}`, `{"op":"search"}`)
		for i := range 6 {
			if e, _ := r[i]["error"].(string); e == "" {
				t.Errorf("line %d: %v", i, r[i])
			}
		}
		if len(list(t, r[6], "songs")) == 0 {
			t.Error("load after the garbage failed")
		}
	})
	t.Run("unknown and empty op", func(t *testing.T) {
		r := session(t, `{"op":"nope"}`, `{}`, `{"op":""}`, `{"op":"LOAD"}`)
		for i, want := range []string{`unknown op "nope"`, `unknown op ""`, `unknown op ""`, `unknown op "LOAD"`} {
			if r[i]["error"] != want {
				t.Errorf("response %d = %v, want error %q", i, r[i], want)
			}
		}
	})
	t.Run("empty line", func(t *testing.T) {
		// An empty line is bad JSON: it gets an error answer, so requests and responses stay paired.
		r := session(t, ``, `   `, `{"op":"nope"}`)
		if len(r) != 3 || r[0]["error"] == nil || r[1]["error"] == nil {
			t.Errorf("responses = %v", r)
		}
	})
	t.Run("long lines", func(t *testing.T) {
		long := req(map[string]any{"op": "search", "query": map[string]any{"name": strings.Repeat("x", 900_000)}})
		r := session(t, long)
		if _, ok := r[0]["matches"]; !ok {
			t.Errorf("response = %v", r[0])
		}
	})
	t.Run("a line over 1 MB ends the session with an error", func(t *testing.T) {
		long := req(map[string]any{"op": "search", "query": map[string]any{"name": strings.Repeat("x", 1_100_000)}})
		var out bytes.Buffer
		err := serve(strings.NewReader(`{"op":"nope"}`+"\n"+long+"\n"+`{"op":"nope"}`+"\n"), &out)
		if err == nil || !strings.Contains(err.Error(), "too long") {
			t.Errorf("err = %v", err)
		}
		if n := strings.Count(out.String(), "\n"); n != 1 {
			t.Errorf("%d responses before the end", n)
		}
	})
	t.Run("matches index the latest load or scan", func(t *testing.T) {
		other := filepath.Join(dir, "other.jsonl")
		testlib.WriteIndex(t, other, []*tab.Song{{Path: "z", Artist: "Zed", Title: "Only"}})
		r := session(t,
			req(map[string]any{"op": "load", "index": index}),
			req(map[string]any{"op": "search", "query": map[string]any{"artist": "zed"}}),
			req(map[string]any{"op": "load", "index": other}),
			req(map[string]any{"op": "search", "query": map[string]any{"artist": "zed"}}),
			req(map[string]any{"op": "search", "query": map[string]any{"artist": "soilbed quartet"}}),
			req(map[string]any{"op": "scan", "root": testlib.Tree(t), "index": filepath.Join(t.TempDir(), "i.jsonl")}),
			req(map[string]any{"op": "search", "query": map[string]any{"artist": "soilbed quartet"}}),
		)
		if got := list(t, r[1], "matches"); len(got) != 0 {
			t.Errorf("zed in the first library: %v", got)
		}
		if got := list(t, r[3], "matches"); len(got) != 1 || got[0] != float64(0) {
			t.Errorf("zed in the second: %v", got)
		}
		if got := list(t, r[4], "matches"); len(got) != 0 {
			t.Errorf("soilbed quartet in the second: %v", got)
		}
		if got := list(t, r[6], "matches"); len(got) != 3 {
			t.Errorf("soilbed quartet after the scan: %v", got)
		}
		// The indices point into the songs list of that scan.
		songs := list(t, r[5], "songs")
		for _, m := range list(t, r[6], "matches") {
			if a := songs[int(m.(float64))].(map[string]any)["artist"]; a != "Soilbed Quartet" {
				t.Errorf("match %v is %v", m, a)
			}
		}
	})
	t.Run("suggestions follow the query", func(t *testing.T) {
		r := session(t, req(map[string]any{"op": "load", "index": index}),
			req(map[string]any{"op": "search", "query": map[string]any{"artist": "in", "tuning": "drop"}}))
		var artists []string
		for _, a := range list(t, r[1], "artists") {
			artists = append(artists, a.(string))
		}
		// "in" is in "Inkwell Flamingos" (prefix) and in several other artists (substring).
		if len(artists) < 2 || artists[0] != "Inkwell Flamingos" {
			t.Errorf("artists = %q", artists)
		}
		tunings := list(t, r[1], "tunings")
		for _, tu := range tunings {
			if label := tu.(map[string]any)["label"].(string); !strings.Contains(strings.ToLower(label+" "+tu.(map[string]any)["notes"].(string)), "drop") {
				t.Errorf("tuning %v doesn't match the typed text", tu)
			}
		}
		if len(tunings) == 0 {
			t.Error("no tuning suggestions")
		}
	})
}

func hasNull(v any) (string, bool) {
	switch v := v.(type) {
	case nil:
		return "", true
	case map[string]any:
		for k, x := range v {
			if p, ok := hasNull(x); ok {
				return "." + k + p, true
			}
		}
	case []any:
		for i, x := range v {
			if p, ok := hasNull(x); ok {
				return fmt.Sprintf("[%d]%s", i, p), true
			}
		}
	}
	return "", false
}

func TestServeNeverSendsNull(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "index.jsonl")
	// Songs without tracks, tempos, tunings, artist or title: every list in them is empty.
	songs := append(testlib.Songs(), &tab.Song{Path: "bare.gp5"}, &tab.Song{Path: "x/drums.gp5", Tracks: []tab.Track{testlib.Drums}}, &tab.Song{Path: "x/untuned.gp5", Tracks: []tab.Track{{Name: "Vox"}}})
	testlib.WriteIndex(t, index, songs)
	empty := t.TempDir()
	queries := []map[string]any{
		{}, {"artist": "soil"}, {"artist": "nobody"}, {"tuning": "zzz"}, {"bpm": "fast"}, {"bpm": "100-200"}, {"strings": 9}, {"strings": 3}, {"name": "x", "artist": "y", "tuning": "z", "bpm": "1-"},
	}
	var reqs []string
	for _, id := range []string{"missing", "empty", "index"} {
		switch id {
		case "missing":
			reqs = append(reqs, req(map[string]any{"op": "load", "index": filepath.Join(dir, "nope")}))
		case "empty":
			reqs = append(reqs, req(map[string]any{"op": "scan", "root": empty, "index": filepath.Join(dir, "e.jsonl")}))
		case "index":
			reqs = append(reqs, req(map[string]any{"op": "load", "index": index}))
		}
		for _, q := range queries {
			reqs = append(reqs, req(map[string]any{"op": "search", "query": q}))
		}
	}
	reqs = append(reqs, req(map[string]any{"op": "scan", "root": testlib.Tree(t), "index": filepath.Join(dir, "t.jsonl")}), `{"op":"search"}`)
	for i, r := range session(t, reqs...) {
		if p, ok := hasNull(r); ok {
			t.Errorf("response %d (%s) has null at %s: %v", i, reqs[i], p, r)
		}
	}
	// Spot check: the empty songs really are in a load response.
	r := session(t, req(map[string]any{"op": "load", "index": index}))
	var bare map[string]any
	for _, s := range list(t, r[0], "songs") {
		if s.(map[string]any)["path"] == "bare.gp5" {
			bare = s.(map[string]any)
		}
	}
	if bare == nil || len(list(t, bare, "tunings")) != 0 || len(list(t, bare, "bpms")) != 0 {
		t.Errorf("bare song = %v", bare)
	}
}

func TestSongOut(t *testing.T) {
	lib := finder.New([]*tab.Song{
		{Path: "a.gp5", Format: "gp5", Title: "Broken", Error: "unknown file format"},
		{Path: "b.gp5", Format: "gp5", Title: "Partial", Error: "tempo changes incomplete", Tracks: []tab.Track{testlib.EStd6}},
		{Path: "c.gp5", Format: "gp5", Title: "NoTunings"},
		{Path: "d.gp5", Format: "gp5", Title: "Drums", Tracks: []tab.Track{testlib.Drums}},
		{Path: "e.gpx.crdownload", Format: "gp6", Title: "Half: Down?"},
		{Path: "f.gp5", Format: "gp5", Title: "Custom", Tracks: []tab.Track{testlib.Custom6, testlib.DropC6}},
		{Path: "g.gp5", Format: "gp5", Title: "Odd", Error: "x", Tracks: []tab.Track{{Name: "T", Tuning: "Custom"}}},
	})
	got := map[string]songOut{}
	for _, s := range songsOut(lib) {
		got[s.Title] = s
	}
	for title, want := range map[string]bool{"Broken": true, "Partial": false, "NoTunings": false, "Drums": false, "Half: Down?": false, "Odd": false} {
		if got[title].Unreadable != want {
			t.Errorf("%s: unreadable = %v, want %v", title, got[title].Unreadable, want)
		}
	}
	for title, want := range map[string]string{"Broken": "Broken.gp5", "Half: Down?": "Half Down.gpx", "Custom": "Custom.gp5"} {
		if got[title].OpenAs != want {
			t.Errorf("%s: openAs = %q, want %q", title, got[title].OpenAs, want)
		}
	}
	// A custom tuning shows its notes as the label and has no detail; named ones keep the notes as detail.
	var labels, details []string
	for _, tu := range got["Custom"].Tunings {
		labels = append(labels, tu.Label)
		details = append(details, tu.Detail)
	}
	if !slices.Equal(labels, []string{"D G D G B D", "Drop C"}) || !slices.Equal(details, []string{"", "C G C F A D"}) {
		t.Errorf("labels %q details %q", labels, details)
	}
	// Notes missing: the label stays "Custom" and so does no detail.
	if tu := got["Odd"].Tunings[0]; tu.Label != "Custom" || tu.Detail != "" || tu.Notes != "" {
		t.Errorf("odd tuning = %+v", tu)
	}
	// JSON field names, which the Android app decodes.
	b, _ := json.Marshal(got["Custom"].Tunings[1])
	if string(b) != `{"strings":6,"name":"Drop C","notes":"C G C F A D","label":"Drop C","detail":"C G C F A D"}` {
		t.Errorf("tuning JSON = %s", b)
	}
}

// FuzzServe: random lines never panic, and every line is answered with one line.
func FuzzServe(f *testing.F) {
	dir := f.TempDir()
	index := filepath.Join(dir, "index.jsonl")
	testlib.WriteIndex(f, index, testlib.Songs())
	root := testlib.Tree(f)
	f.Add("load", "", "", "", "", 0, `{"op":"search","query":{"artist":1}}`)
	f.Add("search", "so", "drop c", "100-", "x", 6, `{"op":"nope"}`)
	f.Add("search", "\x00", "ä", "-", "", -1, "")
	f.Add("scan", "", "", "", "", 0, "{")
	f.Fuzz(func(t *testing.T, op, artist, tuning, bpm, name string, strings_ int, raw string) {
		// A fuzzed line must not make the server read outside the fixture.
		// encoding/json matches keys case-insensitively and decodes \u escapes, so check both.
		if low := strings.ToLower(raw); strings.Contains(low, "root") || strings.Contains(low, "index") || strings.Contains(low, `\u`) {
			raw = "{}"
		}
		raw = strings.NewReplacer("\n", " ", "\r", " ").Replace(raw)
		lines := []string{
			req(map[string]any{"op": "load", "index": index}),
			req(map[string]any{"op": op, "index": index, "root": root, "query": map[string]any{"artist": artist, "tuning": tuning, "bpm": bpm, "name": name, "strings": strings_}}),
			raw,
			req(map[string]any{"op": "search", "query": map[string]any{"artist": artist}}),
		}
		if op == "scan" {
			lines[1] = req(map[string]any{"op": "scan", "root": root, "index": filepath.Join(dir, "scan.jsonl")})
		}
		var out bytes.Buffer
		if err := serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(out.String(), "\n"); n != len(lines) {
			t.Errorf("%d responses for %d lines:\n%s", n, len(lines), out.String())
		}
		for _, l := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
			if !json.Valid([]byte(l)) {
				t.Errorf("invalid JSON response %q", l)
			}
		}
	})
}

// E-CLI-09: the process the Android app starts.
func TestServeProcess(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "index.jsonl")
	root := testlib.Tree(t)

	cmd := exec.Command(tabscanBin, "-serve")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill() })
	r := bufio.NewReader(stdout)
	ask := func(v any) (map[string]any, time.Duration) {
		t.Helper()
		start := time.Now()
		fmt.Fprintln(stdin, req(v))
		line, err := r.ReadBytes('\n')
		if err != nil {
			t.Fatalf("reading the response: %v (stderr %q)", err, stderr.String())
		}
		d := time.Since(start)
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("response %q: %v", line, err)
		}
		return m, d
	}

	if m, _ := ask(map[string]any{"op": "load", "index": index}); len(list(t, m, "songs")) != 0 {
		t.Errorf("load without an index: %v", m)
	}
	m, _ := ask(map[string]any{"op": "scan", "root": root, "index": index})
	if len(list(t, m, "songs")) != 20 || m["unreadable"] != float64(1) {
		t.Errorf("scan = %v", m)
	}
	if m, _ := ask(map[string]any{"op": "load", "index": index}); len(list(t, m, "songs")) != 20 {
		t.Errorf("load after the scan: %d songs", len(list(t, m, "songs")))
	}

	queries := []map[string]any{{"artist": "so"}, {"tuning": "drop"}, {"name": "bras"}, {"bpm": "100-200"}, {"artist": "in", "strings": 6}}
	var slow []time.Duration
	var total time.Duration
	for i := range 100 {
		m, d := ask(map[string]any{"op": "search", "query": queries[i%len(queries)]})
		if _, ok := m["matches"]; !ok {
			t.Fatalf("search %d = %v", i, m)
		}
		total += d
		if d > 5*time.Millisecond {
			slow = append(slow, d)
		}
	}
	// The budget is per search; allow a few outliers from a busy machine.
	if len(slow) > 5 {
		t.Errorf("%d of 100 searches took over 5 ms: %v (mean %v)", len(slow), slow, total/100)
	}

	// Closing stdin ends the session cleanly.
	stdin.Close()
	done := make(chan error, 1)
	go func() {
		io.Copy(io.Discard, r)
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("exit after closing stdin: %v (stderr %q)", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Error("still running 5 s after stdin closed")
	}
}

// A request the Android app sends after the server died is replayed by the app;
// the server itself must treat a fresh process like a new session.
func TestServeProcessStartsEmpty(t *testing.T) {
	out, _, code := run(t, "", `{"op":"search","query":{"artist":"x"}}`+"\n", "-serve")
	if code != 0 || !strings.Contains(out, `"matches":[]`) {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}
