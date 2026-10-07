package tab

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTuning(t *testing.T) {
	tu := TuningOf([]int{36, 43, 48, 53, 57, 62})
	if tu.Name != "Drop C" || tu.Strings() != 6 || tu.Notes() != "C G C F A D" || tu.String() != "Drop C (C G C F A D)" {
		t.Errorf("drop C = %+v, %q", tu, tu)
	}
	if tu := TuningOf(nil); tu.Name != "" || tu.Strings() != 0 || tu.Notes() != "" || tu.String() != "" {
		t.Errorf("no strings = %+v, %q", tu, tu)
	}
	if tu := TuningOf([]int{38, 43, 50, 55, 59, 62}); tu.Name != Custom {
		t.Errorf("open G = %+v", tu)
	}
}

// The tuning is written for people reading tabscan -json, and ignored when an index is read.
func TestTrackJSON(t *testing.T) {
	b, err := json.Marshal(Track{Name: "G", Pitches: []int{40, 45, 50, 55, 59, 64}})
	if err != nil || !strings.Contains(string(b), `"tuning":"E Standard (E A D G B E)"`) {
		t.Fatalf("json = %s, %v", b, err)
	}
	if b, _ := json.Marshal(Track{Name: "D", Drums: true}); strings.Contains(string(b), "tuning") {
		t.Errorf("drums json = %s", b)
	}
	var back Track
	if err := json.Unmarshal([]byte(`{"name":"G","pitches":[36,43,48,53,57,62],"tuning":"stale text"}`), &back); err != nil || back.Tuning().Name != "Drop C" {
		t.Errorf("decoded %+v, %v", back, err)
	}
}
