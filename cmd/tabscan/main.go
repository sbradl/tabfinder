// tabscan walks guitar tab files and prints artist, album, song title,
// tempo(s), instruments, tunings and how hard each part is, optionally filtered by
// search criteria.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/finder"
	"tabfinder/internal/tab"
)

func main() {
	jsonOut := flag.Bool("json", false, "emit one JSON object per file (with per-track details and parts) instead of TSV")
	serveApp := flag.Bool("serve", false, "answer the TabFinder app's requests on stdin/stdout instead (JSON lines, see serve.go)")
	root := flag.String("root", "", "base directory for the Artist/Album fallback (default: the directory argument, or a file argument's parent)")
	var q finder.Query
	flag.StringVar(&q.Name, "name", "", "only songs whose title or file name contains all these words")
	flag.StringVar(&q.Artist, "artist", "", "only songs whose artist contains this text")
	flag.StringVar(&q.Tuning, "tuning", "", `only songs with a track in this tuning, by name or notes: "drop c", "eb standard", "D A D G A D"`)
	flag.StringVar(&q.BPM, "bpm", "", `only songs using a tempo in this range: "120", "100-140", "180-" or "-90"`)
	flag.StringVar(&q.Drums.Level, "drums", "", `only songs whose drums are this hard, 1 to 10: "3", "2-4", "7-" or "-3"`)
	flag.StringVar(&q.Bass.Level, "bass", "", "only songs whose bass is this hard, like -drums")
	flag.StringVar(&q.Rhythm.Level, "rhythm", "", "only songs whose rhythm guitar is this hard, like -drums")
	flag.StringVar(&q.Lead.Level, "lead", "", "only songs whose lead guitar is this hard, like -drums")
	flag.Func("tag", `only songs with a part tagged so, as role:tag: "rhythm:triplets" (repeatable: all of them)`, func(v string) error {
		return addTag(&q, v)
	})
	order := flag.String("sort", "", `order of the songs: "easiest" or "hardest" first (by the parts searched for); default: as found`)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: tabscan [-json] [-serve] [-root dir] [-name words] [-artist text] [-tuning t] [-bpm range]\n"+
			"               [-drums|-bass|-rhythm|-lead levels] [-tag role:tag] [-sort easiest|hardest] [dir|file ...]\n")
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
	f, err := filterOf(&q, *order)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tabscan:", err)
		os.Exit(2)
	}
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	failed := false // a directory couldn't be read, or a song not written
	emit := tsvWriter(out)
	if *jsonOut {
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		emit = func(s *tab.Song) {
			if err := enc.Encode(s); err != nil {
				fmt.Fprintf(os.Stderr, "tabscan: %s: %v\n", s.Path, err)
				failed = true
			}
		}
	} else {
		fmt.Fprintln(out, "path\tartist\talbum\ttitle\ttempo\tinstruments\ttunings\tdifficulty\ttags")
	}

	scanned := 0
	var matched []*tab.Song
	for _, arg := range args {
		err := tab.Walk(arg, *root, func(s *tab.Song) {
			scanned++
			if !f.Matches(s) {
				return
			}
			if s.Error != "" {
				fmt.Fprintf(os.Stderr, "warn: %s: %s\n", s.Path, s.Error)
			}
			if q.Sort == finder.SortAZ {
				emit(s) // as found
			}
			matched = append(matched, s)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "tabscan:", err)
			failed = true
		}
	}
	if q.Sort != finder.SortAZ {
		lib := finder.New(matched)
		for _, i := range lib.Search(q).Matches {
			emit(lib.Entries[i].Song)
		}
	}
	if f.Active() {
		fmt.Fprintf(os.Stderr, "%d of %d tabs match\n", len(matched), scanned)
	}
	if failed {
		out.Flush()
		os.Exit(1)
	}
}

// filterOf checks the query and the order asked for, and turns the query into criteria.
func filterOf(q *finder.Query, order string) (finder.Filter, error) {
	switch order {
	case "", "az":
	case "easiest":
		q.Sort = finder.SortEasiest
	case "hardest":
		q.Sort = finder.SortHardest
	default:
		return finder.Filter{}, fmt.Errorf("invalid sort %q: easiest or hardest", order)
	}
	if q.BPM != "" {
		if _, err := finder.ParseBPMRange(q.BPM); err != nil {
			return finder.Filter{}, err
		}
	}
	for r := range q.LevelInvalid() {
		_, err := finder.ParseLevelRange(q.Part(r).Level)
		return finder.Filter{}, fmt.Errorf("-%s: %w", r, err)
	}
	f, _ := q.Filter()
	return f, nil
}

// addTag adds a "role:tag" to the query.
func addTag(q *finder.Query, v string) error {
	role, tag, ok := strings.Cut(v, ":")
	if !ok || strings.TrimSpace(tag) == "" {
		return errors.New(`want role:tag, like "rhythm:triplets"`)
	}
	parts := map[difficulty.Role]*finder.PartQuery{
		difficulty.Drums: &q.Drums, difficulty.Bass: &q.Bass, difficulty.Rhythm: &q.Rhythm, difficulty.Lead: &q.Lead,
	}
	pq, ok := parts[difficulty.Role(strings.TrimSpace(role))]
	if !ok {
		return fmt.Errorf("no role %q: drums, bass, rhythm or lead", role)
	}
	if pq.Tags != "" {
		pq.Tags += ","
	}
	pq.Tags += strings.TrimSpace(tag)
	return nil
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
			if tu := t.Tuning().String(); tu == "" {
				tun = append(tun, "-")
			} else {
				tun = append(tun, tu)
			}
		}
		var levels, tags []string
		for _, p := range s.Parts {
			levels = append(levels, fmt.Sprintf("%s %d", p.Role, p.Level()))
			if len(p.Tags) > 0 {
				tags = append(tags, string(p.Role)+": "+strings.Join(p.Tags, ", "))
			}
		}
		fields := []string{s.Path, s.Artist, s.Album, s.Title, s.TempoSummary(), strings.Join(instr, "; "), strings.Join(tun, "; "),
			strings.Join(levels, ", "), strings.Join(tags, "; ")}
		for i := range fields {
			fields[i] = clean.Replace(fields[i])
		}
		fmt.Fprintln(w, strings.Join(fields, "\t"))
	}
}
