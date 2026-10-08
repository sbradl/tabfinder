package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tabfinder/internal/finder"
)

func TestServe(t *testing.T) {
	index := filepath.Join(t.TempDir(), "index.jsonl")
	os.WriteFile(index, []byte(finder.IndexHeader+"\n"+`{"path":"s/x.gp3","format":"gp5","artist":"Soilbed Quartet","title":"Brass Kettle","tracks":[{"name":"G","pitches":[36,43,48,53,57,62],"tuning":"Drop C (C G C F A D)"}],"tempos":[{"bar":1,"bpm":190},{"bar":9,"bpm":145}]}
{"path":"a/y.gp5","format":"gp5","artist":"Amber Marsh","title":"First Frost","tracks":[{"name":"G","pitches":[35,40,45,50,55,59],"tuning":"Custom (B E A D G B)"}]}
`), 0o644)
	req := `{"op":"load","index":"` + index + `"}
{"op":"search","query":{"artist":"am"}}
{"op":"search","query":{"bpm":"fast"}}
{"op":"search","query":{"artist":"Amber Marsh","tuning":"zzz"}}
{"op":"nope"}
`
	var out strings.Builder
	if err := serve(strings.NewReader(req), &out); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(strings.NewReader(out.String()))

	var load songsResponse
	lines.Scan()
	json.Unmarshal(lines.Bytes(), &load)
	if len(load.Songs) != 2 || load.Songs[0].Artist != "Amber Marsh" {
		t.Fatalf("load = %s", lines.Text())
	}
	if s := load.Songs[1]; s.OpenAs != "Brass Kettle.gp5" || strings.Join(s.BPMs, ",") != "190,145" || s.Tunings[0].Label != "Drop C" || s.Tunings[0].Detail != "C G C F A D" {
		t.Errorf("song = %+v", s)
	}
	if tu := load.Songs[0].Tunings[0]; tu.Label != "B E A D G B" || tu.Detail != "" {
		t.Errorf("custom tuning = %+v", tu)
	}

	var search searchResponse
	lines.Scan()
	json.Unmarshal(lines.Bytes(), &search)
	if len(search.Matches) != 1 || search.Matches[0] != "a/y.gp5" || len(search.Artists) != 1 || len(search.Tunings) != 1 {
		t.Errorf("search artist am = %s", lines.Text())
	}
	lines.Scan()
	if !strings.Contains(lines.Text(), `"bpmInvalid":true`) {
		t.Errorf("search bpm fast = %s", lines.Text())
	}
	lines.Scan()
	// Lists are empty, never null: the app's decoder rejects null.
	if !strings.Contains(lines.Text(), `"matches":[]`) || !strings.Contains(lines.Text(), `"artists":[]`) || !strings.Contains(lines.Text(), `"tunings":[]`) {
		t.Errorf("empty search = %s", lines.Text())
	}
	lines.Scan()
	if !strings.Contains(lines.Text(), `"error":"unknown op`) {
		t.Errorf("unknown op = %s", lines.Text())
	}
}
