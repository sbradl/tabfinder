package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"tabfinder/internal/finder"
	"tabfinder/internal/rows"
)

// serve answers the Android app's requests, one JSON object per line each
// way, so its search runs on the same code as the desktop app:
//
//	{"op":"load","index":"/path/index.jsonl"}               -> {"songs":[...]}
//	{"op":"scan","root":"/tabs","index":"/path/index.jsonl"} -> {"songs":[...],"unreadable":3,"summary":"948 tabs, 3 unreadable","warning":"..."}
//	{"op":"search","query":{"artist":"am","tuning":"drop","strings":0,"name":"","bpm":""}}
//	    -> {"matches":["Amber Marsh/Dusk.gp5"],"bpmInvalid":false,"artists":["Amber Marsh"],"tunings":[...]}
//
// Songs are rows (internal/rows), what the app shows. Matches are the paths of the matching songs of the last load or scan, in list order, so
// the app needn't hold the same list as this process. Failures answer {"error":"..."}.
func serve(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(nil, 1<<20)
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	lib := finder.New(nil)
	for sc.Scan() {
		var req struct {
			Op    string       `json:"op"`
			Index string       `json:"index"`
			Root  string       `json:"root"`
			Query finder.Query `json:"query"`
		}
		var resp any
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			resp = errorResponse{err.Error()}
		} else {
			switch req.Op {
			case "load":
				songs, err := finder.LoadIndex(req.Index)
				if err != nil {
					resp = errorResponse{err.Error()}
					break
				}
				lib = finder.New(songs)
				resp = songsResponse{Songs: rows.All(lib)}
			case "scan":
				songs, err := finder.ScanIndex(req.Root, req.Index)
				if songs == nil && err != nil {
					resp = errorResponse{err.Error()}
					break
				}
				lib = finder.New(songs)
				r := songsResponse{Songs: rows.All(lib), Unreadable: rows.Unreadable(songs), Summary: rows.ScanSummary(songs)}
				if err != nil {
					r.Warning = err.Error()
				}
				resp = r
			case "search":
				res := lib.Search(req.Query)
				resp = searchResponse{
					Matches:    matchedPaths(lib, res.Matches),
					BPMInvalid: res.BPMInvalid,
					Artists:    lib.SuggestArtists(req.Query.Artist),
					Tunings:    rows.Tunings(finder.SuggestTunings(res.Tunings, req.Query.Tuning)),
				}
			default:
				resp = errorResponse{fmt.Sprintf("unknown op %q", req.Op)}
			}
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

type errorResponse struct {
	Error string `json:"error"`
}

type songsResponse struct {
	Songs      []rows.Song `json:"songs"`
	Unreadable int         `json:"unreadable,omitempty"`
	Summary    string      `json:"summary,omitempty"` // of a scan, for the message after it
	Warning    string      `json:"warning,omitempty"` // an unreadable directory cut the scan short
}

type searchResponse struct {
	Matches    []string      `json:"matches"` // paths of the matching songs
	BPMInvalid bool          `json:"bpmInvalid"`
	Artists    []string      `json:"artists"` // suggestions for the artist field
	Tunings    []rows.Tuning `json:"tunings"` // suggestions for the tuning field, grouped by string count
}

func matchedPaths(lib *finder.Library, idx []int) []string {
	out := make([]string, len(idx))
	for i, m := range idx {
		out[i] = lib.Entries[m].Song.Path
	}
	return out
}
