package main

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"tabfinder/internal/tab"
)

type move struct{ src, dst string } // paths relative to the root

type planner struct {
	root        string
	skip        []string
	rules       nameRules
	artistByKey map[string]string   // key(folder) -> artist folder
	albumDirs   map[string][]string // artist folder -> album folders
}

func newPlanner(root string, skip []string, rules nameRules) (*planner, error) {
	p := &planner{root: root, skip: skip, rules: rules, artistByKey: map[string]string{}, albumDirs: map[string][]string{}}
	artists, err := subdirs(root)
	if err != nil {
		return nil, err
	}
	for _, a := range artists {
		p.artistByKey[key(a)] = a
		if p.albumDirs[a], err = subdirs(filepath.Join(root, a)); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

func (p *planner) skipped(rel string) bool {
	for _, d := range p.skip {
		if rel == d || strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// placement is where one tab goes.
type placement struct {
	song  *tab.Song
	dir   string // destination folder, relative to the root ("" = root)
	final string // destination path
}

// plan decides the destination of every tab:
//   - files already at <Artist>/<Album>/file keep their folder;
//   - loose files in an artist folder go to the album named in the file
//     (matched loosely against existing album folders), else stay;
//   - loose files at the root go to the artist named in the file, else stay;
//   - with rename, files are named after the song. Files that would end up
//     with the same name in one folder keep their names and are returned
//     as clashes;
//   - other name collisions get a " (2)" suffix. Nothing is overwritten.
func (p *planner) plan(songs []*tab.Song, rename bool) (places []*placement, clashes [][]string) {
	places, moving := p.candidates(songs)
	p.placeInAlbums(moving)
	names, clashing, clashes := p.names(places, rename)
	return places, append(clashes, p.finalPaths(places, names, clashing)...)
}

// candidate is a loose tab that may move: to this artist's folder, and the album's.
type candidate struct {
	pl            *placement
	artist, album string // album "" for none
}

// candidates places every tab in its current folder and picks the loose ones that may move.
func (p *planner) candidates(songs []*tab.Song) (places []*placement, moving []candidate) {
	for _, s := range songs {
		pl := &placement{song: s, dir: path.Dir(s.Path)}
		if pl.dir == "." {
			pl.dir = ""
		}
		places = append(places, pl)
		parts := strings.Split(s.Path, "/")
		if p.skipped(s.Path) || len(parts) > 2 {
			continue
		}
		metaArtist := ""
		if s.ArtistSource == tab.FromFile && !p.rules.junkArtistName(s.Artist) {
			metaArtist = s.Artist
		}
		var artist string
		if len(parts) == 1 {
			if metaArtist == "" {
				continue
			}
			artist = p.artistByKey[key(metaArtist)]
			if artist == "" {
				artist = strings.ReplaceAll(strings.TrimSpace(metaArtist), "/", "-")
			}
		} else {
			artist = parts[0]
			if other := p.artistByKey[key(metaArtist)]; metaArtist != "" && other != "" && other != artist && !p.skipped(other) {
				artist = other // e.g. a Felix Ferien song filed under Die Äther
			}
		}
		album := ""
		if s.AlbumSource == tab.FromFile {
			album = p.rules.cleanAlbum(s.Album)
		}
		moving = append(moving, candidate{pl, artist, album})
	}
	return places, moving
}

// placeInAlbums sets the folder of each candidate: an existing album folder that matches its
// album, or a new one. New album names are merged by similarity per artist.
func (p *planner) placeInAlbums(moving []candidate) {
	newAlbums := map[string][]string{}
	var artistOrder []string
	for _, c := range moving {
		if c.album != "" && p.existingAlbum(c.artist, c.album) == "" {
			if _, ok := newAlbums[c.artist]; !ok {
				artistOrder = append(artistOrder, c.artist)
			}
			newAlbums[c.artist] = append(newAlbums[c.artist], c.album)
		}
	}
	canonical := map[[2]string]string{} // {artist, album as found} -> folder
	for _, artist := range artistOrder {
		var clusters [][]string
		for _, n := range newAlbums[artist] {
			i := slices.IndexFunc(clusters, func(c []string) bool { return similar(key(n), key(c[0])) })
			if i < 0 {
				clusters = append(clusters, []string{n})
			} else {
				clusters[i] = append(clusters[i], n)
			}
		}
		for _, c := range clusters {
			for _, n := range c {
				canonical[[2]string{artist, n}] = nicest(c)
			}
		}
	}
	for _, c := range moving {
		folder := ""
		if c.album != "" {
			if folder = p.existingAlbum(c.artist, c.album); folder == "" {
				folder = canonical[[2]string{c.artist, c.album}]
			}
		}
		c.pl.dir = path.Join(c.artist, folder)
	}
}

// names picks the file name of every tab: its own, or with rename the song's. Tabs that would
// get the same new name in one folder keep theirs; they are clashing, and listed as clashes.
func (p *planner) names(places []*placement, rename bool) (names map[*placement]string, clashing map[*placement]bool, clashes [][]string) {
	names = map[*placement]string{}
	groups := map[[2]string][]*placement{} // {folder, lower-case name}
	var groupOrder [][2]string
	for _, pl := range places {
		orig := path.Base(pl.song.Path)
		names[pl] = orig
		if rename && !p.skipped(pl.song.Path) {
			if name := p.rules.songName(pl.song); name != "" {
				names[pl] = name + tabExt(orig)
			}
		}
		g := [2]string{pl.dir, strings.ToLower(names[pl])}
		if _, ok := groups[g]; !ok {
			groupOrder = append(groupOrder, g)
		}
		groups[g] = append(groups[g], pl)
	}
	clashing = map[*placement]bool{}
	for _, g := range groupOrder {
		members := groups[g]
		renamed := slices.ContainsFunc(members, func(pl *placement) bool { return names[pl] != path.Base(pl.song.Path) })
		if len(members) > 1 && renamed {
			var paths []string
			for _, pl := range members {
				clashing[pl] = true
				paths = append(paths, path.Join(pl.dir, path.Base(pl.song.Path)))
			}
			clashes = append(clashes, paths)
		}
	}
	return names, clashing, clashes
}

// finalPaths sets every tab's destination. Any existing file blocks a name (as does an
// earlier placement), except the tab's own current path: a new name that's blocked is given
// up (a clash), and a kept one gets a " (2)" suffix.
func (p *planner) finalPaths(places []*placement, names map[*placement]string, clashing map[*placement]bool) (clashes [][]string) {
	taken := map[string]bool{} // lower-case final paths
	blocked := func(rel, own string) bool {
		if taken[strings.ToLower(rel)] {
			return true
		}
		_, err := os.Lstat(filepath.Join(p.root, filepath.FromSlash(rel)))
		return err == nil && rel != own
	}
	for _, pl := range places {
		own := pl.song.Path
		orig := path.Base(own)
		name := orig
		if !clashing[pl] {
			name = names[pl]
		}
		if name != orig && blocked(path.Join(pl.dir, name), own) {
			clashes = append(clashes, []string{path.Join(pl.dir, orig), path.Join(pl.dir, name)})
			name = orig
		}
		final := path.Join(pl.dir, name)
		ext := tabExt(name)
		stem := strings.TrimSuffix(name, ext)
		for n := 2; blocked(final, own); n++ {
			final = path.Join(pl.dir, stem+" ("+strconv.Itoa(n)+")"+ext)
		}
		pl.final = final
		taken[strings.ToLower(final)] = true
	}
	return clashes
}

// existingAlbum returns the artist's album folder matching album, if any.
func (p *planner) existingAlbum(artist, album string) string {
	for _, d := range p.albumDirs[artist] {
		if similar(key(album), key(d)) {
			return d
		}
	}
	return ""
}

func moves(places []*placement) []move {
	var ms []move
	for _, pl := range places {
		if pl.final != pl.song.Path {
			ms = append(ms, move{pl.song.Path, pl.final})
		}
	}
	return ms
}
