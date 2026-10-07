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
)

// writeSummary writes the markdown report: files without album, songs
// with several tabs (same artist + title, or byte-identical) and the
// renames skipped because of name clashes.
func writeSummary(file, root string, p *planner, places []*placement, ms []move, clashes [][]string, applied bool) error {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	status := "already organized, nothing to move"
	if len(ms) > 0 {
		status = fmt.Sprintf("%d to move/rename (dry run)", len(ms))
		if applied {
			status = fmt.Sprintf("%d moved/renamed", len(ms))
		}
	}
	w("# Tab reorganization summary (%s)", time.Now().Format("2006-01-02"))
	w("")
	w("Root: `%s` · %d tab files scanned · %s", root, len(places), status)
	w("")
	var skip []string
	for _, d := range p.skip {
		skip = append(skip, "`"+d+"`")
	}
	w("Left untouched by request: %s.", strings.Join(skip, ", "))

	inSkipped := func(final string) bool { return p.skipped(final) && strings.Contains(final, "/") }

	// Files without album.
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
	w("")
	w("## Files without album (%d)", total)
	w("")
	w("No usable album in the file's metadata and not inside an album folder.")
	w("")
	for _, a := range artists {
		files := noAlbum[a]
		slices.SortFunc(files, func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) })
		var quoted []string
		for _, f := range files {
			quoted = append(quoted, "`"+f+"`")
		}
		w("- **%s** (%d): %s", a, len(files), strings.Join(quoted, ", "))
	}

	// Renames skipped because several tabs would get the same name.
	w("")
	w("## Not renamed: same song name in one folder (%d)", len(clashes))
	w("")
	w("These tabs would all be named after the same song, so they keep their file names.")
	w("")
	for _, c := range clashes {
		var quoted []string
		for _, f := range c {
			quoted = append(quoted, "`"+f+"`")
		}
		w("- %s", strings.Join(quoted, " · "))
	}

	// Songs with several tabs.
	hashOf := map[*placement]string{}
	for _, pl := range places {
		rel := pl.song.Path
		if applied {
			rel = pl.final
		}
		h, err := md5File(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		hashOf[pl] = h
	}
	parent := map[*placement]*placement{}
	var find func(x *placement) *placement
	find = func(x *placement) *placement {
		for parent[x] != nil && parent[x] != x {
			x = parent[x]
		}
		return x
	}
	union := func(g []*placement) {
		for _, pl := range g[1:] {
			parent[find(pl)] = find(g[0])
		}
	}
	bySong := map[[2]string][]*placement{}
	var songOrder [][2]string
	byHash := map[string][]*placement{}
	var hashOrder []string
	for _, pl := range places {
		if _, ok := byHash[hashOf[pl]]; !ok {
			hashOrder = append(hashOrder, hashOf[pl])
		}
		byHash[hashOf[pl]] = append(byHash[hashOf[pl]], pl)
		k := songKey(pl.song)
		if k == "" || k == "untitled" || k == "track1" {
			continue
		}
		artist := ""
		if i := strings.IndexByte(pl.final, '/'); i > 0 {
			artist = pl.final[:i]
		}
		if pl.song.ArtistSource == "file" && !reJunkArtist.MatchString(pl.song.Artist) && (inSkipped(pl.final) || artist == "") {
			artist = pl.song.Artist
		}
		g := [2]string{key(artist), k}
		if _, ok := bySong[g]; !ok {
			songOrder = append(songOrder, g)
		}
		bySong[g] = append(bySong[g], pl)
	}
	for _, g := range songOrder {
		union(bySong[g])
	}
	for _, h := range hashOrder {
		union(byHash[h])
	}
	groups := map[*placement][]*placement{}
	var groupOrder []*placement
	for _, pl := range places {
		r := find(pl)
		if _, ok := groups[r]; !ok {
			groupOrder = append(groupOrder, r)
		}
		groups[r] = append(groups[r], pl)
	}
	var dups [][]*placement
	identical, files := 0, 0
	for _, r := range groupOrder {
		g := groups[r]
		if len(g) < 2 {
			continue
		}
		dups = append(dups, g)
		files += len(g)
		seen := map[string]bool{}
		for _, pl := range g {
			seen[hashOf[pl]] = true
		}
		if len(seen) < len(g) {
			identical++
		}
	}
	lower := func(pl *placement) string { return strings.ToLower(pl.final) }
	slices.SortStableFunc(dups, func(a, b []*placement) int { return strings.Compare(lower(a[0]), lower(b[0])) })

	w("")
	w("## Duplicate songs (%d songs, %d files)", len(dups), files)
	w("")
	w("Same artist + same song title (or byte-identical). %d groups contain byte-identical files (marked **identical**, safe to delete all but one). Paths are after the reorganization.", identical)
	w("")
	for _, g := range dups {
		slices.SortStableFunc(g, func(a, b *placement) int { return strings.Compare(lower(a), lower(b)) })
		var titles []string
		for _, pl := range g {
			if pl.song.TitleSource == "file" && songKey(pl.song) == songKey(g[0].song) {
				titles = append(titles, pl.song.Title)
			}
		}
		title := g[0].song.Title
		if len(titles) > 0 {
			title = nicest(titles)
		}
		artist := g[0].song.Artist
		if i := strings.IndexByte(g[0].final, '/'); i > 0 {
			artist = g[0].final[:i]
		} else if artist == "" {
			artist = "(root)"
		}
		count := map[string]int{}
		for _, pl := range g {
			count[hashOf[pl]]++
		}
		w("### %s – %s", artist, title)
		w("")
		w("| File | Format | Size | Tracks | Main tuning | Note |")
		w("|---|---|---|---|---|---|")
		for _, pl := range g {
			rel := pl.song.Path
			if applied {
				rel = pl.final
			}
			st, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
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
			w("| `%s` | %s | %d KB | %d | %s | %s |", pl.final, format, st.Size()/1024, len(pl.song.Tracks), mainTuning(pl), note)
		}
		w("")
	}
	return os.WriteFile(file, []byte(b.String()), 0o644)
}

// mainTuning is the most common tuning name among the tab's tracks
// (the first one seen wins ties).
func mainTuning(pl *placement) string {
	count := map[string]int{}
	var order []string
	for _, t := range pl.song.Tracks {
		if t.Tuning == "" {
			continue
		}
		label, _, _ := strings.Cut(t.Tuning, " (")
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
