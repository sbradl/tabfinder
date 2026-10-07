// tabscan walks guitar tab files and prints artist, album, song title,
// tempo(s), instruments and tunings, optionally filtered by search criteria.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"tabfinder/internal/tab"
)

func main() {
	jsonOut := flag.Bool("json", false, "emit one JSON object per file (with per-track details) instead of TSV")
	serveApp := flag.Bool("serve", false, "answer the TabFinder app's requests on stdin/stdout instead (JSON lines, see serve.go)")
	root := flag.String("root", "", "base directory for the Artist/Album fallback (default: the directory argument, or a file argument's parent)")
	var f tab.Filter
	flag.StringVar(&f.Name, "name", "", "only songs whose title or file name contains all these words")
	flag.StringVar(&f.Artist, "artist", "", "only songs whose artist contains this text")
	flag.StringVar(&f.Tuning, "tuning", "", `only songs with a track in this tuning, by name or notes: "drop c", "eb standard", "D A D G A D"`)
	bpm := flag.String("bpm", "", `only songs using a tempo in this range: "120", "100-140", "180-" or "-90"`)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: tabscan [-json] [-serve] [-root dir] [-name words] [-artist text] [-tuning t] [-bpm range] [dir|file ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *serveApp {
		if err := serve(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "tabscan:", err)
			os.Exit(1)
		}
		return
	}
	if *bpm != "" {
		var err error
		if f.BPMMin, f.BPMMax, err = tab.ParseBPMRange(*bpm); err != nil {
			fmt.Fprintln(os.Stderr, "tabscan:", err)
			os.Exit(2)
		}
		f.BPMSet = true
	}
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	emit := tsvWriter(out)
	if *jsonOut {
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		emit = func(s *tab.Song) { enc.Encode(s) }
	} else {
		fmt.Fprintln(out, "path\tartist\talbum\ttitle\ttempo\tinstruments\ttunings")
	}

	failed, scanned, matched := false, 0, 0
	for _, arg := range args {
		err := tab.Walk(arg, *root, func(s *tab.Song) {
			scanned++
			if !f.Matches(s) {
				return
			}
			matched++
			if s.Error != "" {
				fmt.Fprintf(os.Stderr, "warn: %s: %s\n", s.Path, s.Error)
			}
			emit(s)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "tabscan:", err)
			failed = true
		}
	}
	if f.Active() {
		fmt.Fprintf(os.Stderr, "%d of %d tabs match\n", matched, scanned)
	}
	if failed {
		out.Flush()
		os.Exit(1)
	}
}

func tsvWriter(w *bufio.Writer) func(*tab.Song) {
	clean := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	return func(s *tab.Song) {
		var instr, tun []string
		for _, t := range s.Tracks {
			name := t.Name
			if t.Instrument != "" && !strings.EqualFold(t.Instrument, t.Name) {
				name += " [" + t.Instrument + "]"
			}
			instr = append(instr, name)
			if t.Tuning == "" {
				tun = append(tun, "-")
			} else {
				tun = append(tun, t.Tuning)
			}
		}
		fields := []string{s.Path, s.Artist, s.Album, s.Title, s.TempoSummary(), strings.Join(instr, "; "), strings.Join(tun, "; ")}
		for i := range fields {
			fields[i] = clean.Replace(fields[i])
		}
		fmt.Fprintln(w, strings.Join(fields, "\t"))
	}
}
