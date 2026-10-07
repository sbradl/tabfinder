package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"tabfinder/internal/tab"
)

// outcome is what a run planned or did, which the summary reports.
type outcome struct {
	root    string
	planner *planner
	places  []*placement
	moves   []move
	clashes [][]string
	applied bool // the moves were made: files are at their final paths
}

// inSkipped reports whether a final path is inside a folder left untouched.
func (o outcome) inSkipped(final string) bool {
	return o.planner.skipped(final) && strings.Contains(final, "/")
}

// report is Markdown being written, line by line.
type report struct{ strings.Builder }

func (r *report) line(format string, a ...any) { fmt.Fprintf(r, format+"\n", a...) }

// quoted is the names as code, joined by sep.
func quoted(names []string, sep string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = "`" + n + "`"
	}
	return strings.Join(q, sep)
}

// writeSummary writes the markdown report: files without album, songs
// with several tabs (same artist + title, or byte-identical) and the
// renames skipped because of name clashes.
func writeSummary(file string, o outcome) error {
	var r report
	o.header(&r)
	o.withoutAlbumSection(&r)
	o.clashSection(&r)
	if err := o.duplicateSection(&r); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(r.String()), 0o644)
}

func (o outcome) header(r *report) {
	status := "already organized, nothing to move"
	if len(o.moves) > 0 {
		status = fmt.Sprintf("%d to move/rename (dry run)", len(o.moves))
		if o.applied {
			status = fmt.Sprintf("%d moved/renamed", len(o.moves))
		}
	}
	r.line("# Tab reorganization summary (%s)", time.Now().Format("2006-01-02"))
	r.line("")
	r.line("Root: `%s` · %d tab files scanned · %s", o.root, len(o.places), status)
	if skip := o.planner.skip; len(skip) > 0 {
		r.line("")
		r.line("Left untouched by request: %s.", quoted(skip, ", "))
	}
}

func (o outcome) withoutAlbumSection(r *report) {
	noAlbum := withoutAlbum(o.places, o.inSkipped)
	total := 0
	var artists []string
	for a, files := range noAlbum {
		total += len(files)
		artists = append(artists, a)
	}
	slices.SortFunc(artists, func(a, b string) int {
		if (a == "(root)") != (b == "(root)") {
			if a == "(root)" {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	r.line("")
	r.line("## Files without album (%d)", total)
	r.line("")
	r.line("No usable album in the file's metadata and not inside an album folder.")
	r.line("")
	for _, a := range artists {
		files := noAlbum[a]
		slices.SortFunc(files, func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) })
		r.line("- **%s** (%d): %s", a, len(files), quoted(files, ", "))
	}
}

// clashSection lists the renames skipped because several tabs would get the same name.
func (o outcome) clashSection(r *report) {
	r.line("")
	r.line("## Not renamed: same song name in one folder (%d)", len(o.clashes))
	r.line("")
	r.line("These tabs would all be named after the same song, so they keep their file names.")
	r.line("")
	for _, c := range o.clashes {
		r.line("- %s", quoted(c, " · "))
	}
}

// duplicateSection lists the songs with several tabs, a table of the tabs for each.
func (o outcome) duplicateSection(r *report) error {
	hashOf, err := hashFiles(o.root, o.places, o.applied)
	if err != nil {
		return err
	}
	dups := duplicates(o.places, hashOf, o.inSkipped, o.planner.rules)
	identical, files := 0, 0
	for _, g := range dups {
		files += len(g)
		if hasIdentical(g, hashOf) {
			identical++
		}
	}
	r.line("")
	r.line("## Duplicate songs (%d songs, %d files)", len(dups), files)
	r.line("")
	r.line("Same artist + same song title (or byte-identical). %d groups contain byte-identical files (marked **identical**, safe to delete all but one). Paths are after the reorganization.", identical)
	r.line("")
	lower := func(pl *placement) string { return strings.ToLower(pl.final) }
	for _, g := range dups {
		slices.SortStableFunc(g, func(a, b *placement) int { return strings.Compare(lower(a), lower(b)) })
		artist, title := o.groupName(g)
		count := map[string]int{}
		for _, pl := range g {
			count[hashOf[pl]]++
		}
		r.line("### %s – %s", artist, title)
		r.line("")
		r.line("| File | Format | Size | Tracks | Main tuning | Note |")
		r.line("|---|---|---|---|---|---|")
		for _, pl := range g {
			st, err := os.Stat(currentPath(o.root, pl, o.applied))
			if err != nil {
				return err
			}
			note := ""
			if count[hashOf[pl]] > 1 {
				note = "**identical**"
			}
			if pl.song.Error != "" {
				note = strings.TrimSpace(note + " ⚠ " + pl.song.Error)
			}
			format := pl.song.Format
			if format == "" {
				format = "?"
			}
			r.line("| `%s` | %s | %d KB | %d | %s | %s |", pl.final, format, st.Size()/1024, len(pl.song.Tracks), mainTuning(pl), note)
		}
		r.line("")
	}
	return nil
}

// hasIdentical reports whether two tabs of g are byte-identical.
func hasIdentical(g []*placement, hashOf map[*placement]string) bool {
	seen := map[string]bool{}
	for _, pl := range g {
		seen[hashOf[pl]] = true
	}
	return len(seen) < len(g)
}

// groupName is the artist and the title a group of tabs of one song goes by: its first tab's
// artist folder, and the nicest of the titles in the files that name this song.
func (o outcome) groupName(g []*placement) (artist, title string) {
	rules := o.planner.rules
	var titles []string
	for _, pl := range g {
		if pl.song.TitleSource == tab.FromFile && rules.songKey(pl.song) == rules.songKey(g[0].song) {
			titles = append(titles, pl.song.Title)
		}
	}
	title = g[0].song.Title
	if len(titles) > 0 {
		title = nicest(titles)
	}
	artist = g[0].song.Artist
	if i := strings.IndexByte(g[0].final, '/'); i > 0 {
		artist = g[0].final[:i]
	} else if artist == "" {
		artist = "(root)"
	}
	return artist, title
}

// mainTuning is the most common tuning name among the tab's tracks
// (the first one seen wins ties).
func mainTuning(pl *placement) string {
	count := map[string]int{}
	var order []string
	for _, t := range pl.song.Tracks {
		if len(t.Pitches) == 0 {
			continue
		}
		label := t.Tuning().Name
		if count[label] == 0 {
			order = append(order, label)
		}
		count[label]++
	}
	best := "?"
	for _, l := range order {
		if best == "?" || count[l] > count[best] {
			best = l
		}
	}
	return best
}

// withoutAlbum lists the file names of the tabs that end up outside an album folder, by
// artist folder ("(root)" for the root), leaving out the folders left untouched.
func withoutAlbum(places []*placement, inSkipped func(final string) bool) map[string][]string {
	noAlbum := map[string][]string{}
	for _, pl := range places {
		parts := strings.Split(pl.final, "/")
		if len(parts) <= 2 && !inSkipped(pl.final) {
			artist := "(root)"
			if len(parts) == 2 {
				artist = parts[0]
			}
			noAlbum[artist] = append(noAlbum[artist], parts[len(parts)-1])
		}
	}
	return noAlbum
}

// currentPath is where pl's file is now: at its final path once the moves are applied.
func currentPath(root string, pl *placement, applied bool) string {
	rel := pl.song.Path
	if applied {
		rel = pl.final
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

func md5File(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
