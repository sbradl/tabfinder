package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"tabfinder/internal/finder"
)

// serve answers the Android app's requests, one JSON object per line each
// way, so its search runs on the same code as the desktop app:
//
//	{"op":"load","index":"/path/index.jsonl"}               -> {"songs":[...]}
//	{"op":"scan","root":"/tabs","index":"/path/index.jsonl"} -> {"songs":[...],"unreadable":3,"warning":"..."}
//	{"op":"search","query":{"artist":"am","tuning":"drop","strings":0,"name":"","bpm":""}}
//	    -> {"matches":[0,4],"bpmInvalid":false,"artists":["Amber Marsh"],"tunings":[...]}
//
// Matches index the songs of the last load or scan. Failures answer {"error":"..."}.
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
				resp = songsResponse{Songs: songsOut(lib)}
			case "scan":
				songs, err := finder.ScanIndex(req.Root, req.Index)
				if songs == nil && err != nil {
					resp = errorResponse{err.Error()}
					break
				}
				lib = finder.New(songs)
				r := songsResponse{Songs: songsOut(lib)}
				for _, s := range songs {
					if s.Error != "" {
						r.Unreadable++
					}
				}
				if err != nil {
					r.Warning = err.Error()
				}
				resp = r
			case "search":
				res := lib.Search(req.Query)
				resp = searchResponse{
					Matches:    res.Matches,
					BPMInvalid: res.BPMInvalid,
					Artists:    lib.SuggestArtists(req.Query.Artist),
					Tunings:    tuningsOut(finder.SuggestTunings(res.Tunings, req.Query.Tuning)),
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
	Songs      []songOut `json:"songs"`
	Unreadable int       `json:"unreadable,omitempty"`
	Warning    string    `json:"warning,omitempty"` // an unreadable directory cut the scan short
}

type searchResponse struct {
	Matches    []int       `json:"matches"`
	BPMInvalid bool        `json:"bpmInvalid"`
	Artists    []string    `json:"artists"` // suggestions for the artist field
	Tunings    []tuningOut `json:"tunings"` // suggestions for the tuning field, grouped by string count
}

// songOut is a song with what its row shows.
type songOut struct {
	Path       string      `json:"path"`
	Title      string      `json:"title"`
	Artist     string      `json:"artist"`
	Album      string      `json:"album"`
	Tunings    []tuningOut `json:"tunings"`
	BPMs       []string    `json:"bpms"`
	Unreadable bool        `json:"unreadable"` // parsing failed and there's nothing to show
	OpenAs     string      `json:"openAs"`     // the file name to hand TuxGuitar a copy under
}

type tuningOut struct {
	finder.Tuning
	Label  string `json:"label"`
	Detail string `json:"detail"` // the notes, unless they are the label already
}

func songsOut(lib *finder.Library) []songOut {
	out := make([]songOut, len(lib.Entries))
	for i, e := range lib.Entries {
		out[i] = songOut{
			Path: e.Song.Path, Title: e.Song.Title, Artist: e.Song.Artist, Album: e.Song.Album,
			Tunings:    tuningsOut(e.Tunings),
			BPMs:       e.BPMs,
			Unreadable: e.Song.Error != "" && len(e.Tunings) == 0,
			OpenAs:     finder.TuxGuitarName(e.Song),
		}
		if out[i].BPMs == nil {
			out[i].BPMs = []string{}
		}
	}
	return out
}

func tuningsOut(ts []finder.Tuning) []tuningOut {
	out := make([]tuningOut, len(ts))
	for i, t := range ts {
		out[i] = tuningOut{Tuning: t, Label: t.Label()}
		if t.Label() != t.Notes {
			out[i].Detail = t.Notes
		}
	}
	return out
}
