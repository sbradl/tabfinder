package finder

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"tab-sync/internal/tabfiles"
	"tab-sync/internal/testlib"
)

func writeIndexText(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "index.jsonl")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadIndex(t *testing.T) {
	t.Run("missing file is no error", func(t *testing.T) {
		songs, err := LoadIndex(filepath.Join(t.TempDir(), "nope", "index.jsonl"))
		if songs != nil || err != nil {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("empty file", func(t *testing.T) {
		songs, err := LoadIndex(writeIndexText(t, ""))
		if len(songs) != 0 || err != nil {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("blank lines skipped", func(t *testing.T) {
		songs, err := LoadIndex(writeIndexText(t, "\n  \n{\"path\":\"a\"}\n\t\n\n{\"path\":\"b\"}\n   \n"))
		if err != nil || len(songs) != 2 || songs[0].Path != "a" || songs[1].Path != "b" {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("no trailing newline, CRLF", func(t *testing.T) {
		songs, err := LoadIndex(writeIndexText(t, "{\"path\":\"a\"}\r\n{\"path\":\"b\"}"))
		if err != nil || len(songs) != 2 {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("malformed line", func(t *testing.T) {
		for _, text := range []string{"{\"path\":\"a\"}\nnot json\n", "{\"path\":\n", "[1,2]\n", "{\"path\":5}\n", "{\"path\":\"a\"}\n{\"path\":\"b\"\n"} {
			songs, err := LoadIndex(writeIndexText(t, text))
			if err == nil || songs != nil {
				t.Errorf("%q: songs = %v, err = %v", text, songs, err)
			}
		}
	})
	t.Run("unknown fields ignored", func(t *testing.T) {
		songs, err := LoadIndex(writeIndexText(t, `{"path":"a","title":"T","future":{"x":[1,2]},"tracks":[{"name":"G","newField":1}]}`+"\n"))
		if err != nil || len(songs) != 1 || songs[0].Title != "T" || songs[0].Tracks[0].Name != "G" {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("long lines", func(t *testing.T) {
		for _, size := range []int{70 << 10, 2 << 20, 15 << 20} {
			title := strings.Repeat("x", size)
			songs, err := LoadIndex(writeIndexText(t, `{"path":"a","title":"`+title+`"}`+"\n"+`{"path":"b"}`+"\n"))
			if err != nil || len(songs) != 2 || len(songs[0].Title) != size {
				t.Errorf("%d bytes: %d songs, err = %v", size, len(songs), err)
			}
		}
	})
	t.Run("line over 16 MB", func(t *testing.T) {
		title := strings.Repeat("x", 17<<20)
		songs, err := LoadIndex(writeIndexText(t, `{"path":"a","title":"`+title+`"}`+"\n"))
		if err == nil || songs != nil {
			t.Errorf("songs = %d, err = %v", len(songs), err)
		}
	})
	t.Run("not readable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		p := writeIndexText(t, "{}\n")
		os.Chmod(p, 0)
		songs, err := LoadIndex(p)
		if err == nil || songs != nil {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("a directory", func(t *testing.T) {
		songs, err := LoadIndex(t.TempDir())
		if err == nil || songs != nil {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
	})
	t.Run("round trip of the shared fixture", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "i.jsonl")
		want := testlib.Songs()
		testlib.WriteIndex(t, p, want)
		got, err := LoadIndex(p)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("err = %v; songs differ", err)
		}
	})
}

func scanTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for path, data := range map[string][]byte{
		"Die Äther/Polka ist anders/Ruf & Sonne <live>.tg": tabfiles.TG1("Ruf & Sonne <live>", "Die Äther", "Polka ist anders"),
		"Soilbed Quartet/Brass Kettle.tg":                  tabfiles.TG1("Brass Kettle", "Soilbed Quartet", ""),
		"Soilbed Quartet/broken.gp5":                       []byte("junk"),
		"loose.tg":                                         tabfiles.TG1("", "", ""),
	} {
		p := filepath.Join(root, path)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScanIndex(t *testing.T) {
	t.Run("writes the index and round-trips", func(t *testing.T) {
		root, index := scanTree(t), filepath.Join(t.TempDir(), "index.jsonl")
		songs, err := ScanIndex(root, index)
		if err != nil || len(songs) != 4 {
			t.Fatalf("songs = %d, err = %v", len(songs), err)
		}
		loaded, err := LoadIndex(index)
		if err != nil || !reflect.DeepEqual(loaded, songs) {
			t.Errorf("loaded index differs from scan (err %v)", err)
		}
		if _, err := os.Stat(index + ".tmp"); !os.IsNotExist(err) {
			t.Errorf("temp file left behind: %v", err)
		}
		// Not HTML-escaped, so the file stays readable and the app's text search sees "&" and "<".
		raw, _ := os.ReadFile(index)
		if !strings.Contains(string(raw), "Ruf & Sonne <live>") || !strings.Contains(string(raw), "Die Äther") {
			t.Errorf("index escaped or mangled:\n%s", raw)
		}
		if n := strings.Count(string(raw), "\n"); n != 4 {
			t.Errorf("%d lines", n)
		}
	})
	t.Run("creates the parent directory", func(t *testing.T) {
		index := filepath.Join(t.TempDir(), "a", "b", "c", "index.jsonl")
		if _, err := ScanIndex(scanTree(t), index); err != nil {
			t.Fatal(err)
		}
		if st, err := os.Stat(filepath.Dir(index)); err != nil || !st.IsDir() {
			t.Errorf("dir: %v, %v", st, err)
		}
	})
	t.Run("replaces the old index", func(t *testing.T) {
		index := writeIndexText(t, "{\"path\":\"old\"}\n")
		songs, err := ScanIndex(scanTree(t), index)
		if err != nil {
			t.Fatal(err)
		}
		loaded, _ := LoadIndex(index)
		if len(loaded) != len(songs) || loaded[0].Path == "old" {
			t.Errorf("loaded = %v", loaded)
		}
	})
	t.Run("empty folder writes an empty index", func(t *testing.T) {
		index := filepath.Join(t.TempDir(), "i.jsonl")
		songs, err := ScanIndex(t.TempDir(), index)
		if err != nil || len(songs) != 0 {
			t.Fatalf("songs = %v, err = %v", songs, err)
		}
		if st, err := os.Stat(index); err != nil || st.Size() != 0 {
			t.Errorf("index: %v, %v", st, err)
		}
	})
	t.Run("missing root: error, nothing written, old index kept", func(t *testing.T) {
		index := writeIndexText(t, "{\"path\":\"old\"}\n")
		songs, err := ScanIndex(filepath.Join(t.TempDir(), "nope"), index)
		if err == nil || songs != nil {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
		if raw, _ := os.ReadFile(index); string(raw) != "{\"path\":\"old\"}\n" {
			t.Errorf("old index changed: %q", raw)
		}
		if _, err := os.Stat(index + ".tmp"); !os.IsNotExist(err) {
			t.Error("temp file written")
		}
	})
	t.Run("unreadable root dir with no songs: error and no file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		root := t.TempDir()
		os.Chmod(root, 0)
		defer os.Chmod(root, 0o755)
		index := filepath.Join(t.TempDir(), "i.jsonl")
		songs, err := ScanIndex(root, index)
		if err == nil || songs != nil {
			t.Errorf("songs = %v, err = %v", songs, err)
		}
		if _, err := os.Stat(index); !os.IsNotExist(err) {
			t.Error("index written")
		}
	})
	t.Run("partial scan: songs, file and error", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		root := scanTree(t)
		locked := filepath.Join(root, "zzz-locked")
		os.Mkdir(locked, 0o755)
		os.Chmod(locked, 0)
		defer os.Chmod(locked, 0o755)
		index := filepath.Join(t.TempDir(), "i.jsonl")
		songs, err := ScanIndex(root, index)
		if err == nil || !strings.Contains(err.Error(), locked) || len(songs) == 0 {
			t.Fatalf("songs = %d, err = %v", len(songs), err)
		}
		if loaded, lerr := LoadIndex(index); lerr != nil || len(loaded) != len(songs) {
			t.Errorf("index has %d songs (err %v), scan %d", len(loaded), lerr, len(songs))
		}
	})
	t.Run("write failure keeps the old index", func(t *testing.T) {
		dir := t.TempDir()
		index := filepath.Join(dir, "index.jsonl")
		os.WriteFile(index, []byte("{\"path\":\"old\"}\n"), 0o644)
		os.Mkdir(index+".tmp", 0o755) // the temp file can't be created
		songs, err := ScanIndex(scanTree(t), index)
		if err == nil || len(songs) == 0 {
			t.Errorf("songs = %d, err = %v", len(songs), err)
		}
		if raw, _ := os.ReadFile(index); string(raw) != "{\"path\":\"old\"}\n" {
			t.Errorf("old index changed: %q", raw)
		}
	})
	t.Run("parent is a file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "file")
		os.WriteFile(parent, nil, 0o644)
		songs, err := ScanIndex(scanTree(t), filepath.Join(parent, "index.jsonl"))
		if err == nil || len(songs) == 0 {
			t.Errorf("songs = %d, err = %v", len(songs), err)
		}
	})
}
