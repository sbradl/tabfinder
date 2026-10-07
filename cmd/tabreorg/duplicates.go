package main

import (
	"slices"
	"strings"

	"tabfinder/internal/tab"
)

// placeholderTitles are the song keys of titles that name no song.
var placeholderTitles = map[string]bool{"": true, "untitled": true, "track1": true}

// hashFiles hashes the file of every tab where it is now.
func hashFiles(root string, places []*placement, applied bool) (map[*placement]string, error) {
	hashOf := map[*placement]string{}
	for _, pl := range places {
		h, err := md5File(currentPath(root, pl, applied))
		if err != nil {
			return nil, err
		}
		hashOf[pl] = h
	}
	return hashOf, nil
}

// duplicates groups the tabs of one song: the same artist and song key, or byte-identical
// files (by hashOf), transitively. Groups of one are left out; the groups are sorted by their
// first tab's final path, the tabs in a group as they come in places.
func duplicates(places []*placement, hashOf map[*placement]string, inSkipped func(final string) bool, rules nameRules) [][]*placement {
	parent := map[*placement]*placement{}
	find := func(x *placement) *placement {
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
	bySong := map[[2]string][]*placement{} // {key(artist), song key}
	var songOrder [][2]string
	byHash := map[string][]*placement{}
	var hashOrder []string
	for _, pl := range places {
		if _, ok := byHash[hashOf[pl]]; !ok {
			hashOrder = append(hashOrder, hashOf[pl])
		}
		byHash[hashOf[pl]] = append(byHash[hashOf[pl]], pl)
		k := rules.songKey(pl.song)
		if placeholderTitles[k] {
			continue // not a song's name: tabs called that aren't the same song
		}
		g := [2]string{key(songArtist(pl, inSkipped, rules)), k}
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
	for _, r := range groupOrder {
		if g := groups[r]; len(g) > 1 {
			dups = append(dups, g)
		}
	}
	lower := func(pl *placement) string { return strings.ToLower(pl.final) }
	slices.SortStableFunc(dups, func(a, b []*placement) int { return strings.Compare(lower(a[0]), lower(b[0])) })
	return dups
}

// songArtist is the artist a tab is by, for telling songs apart: its artist folder after the
// moves, or the artist in the file for a tab at the root or in a folder left untouched.
func songArtist(pl *placement, inSkipped func(final string) bool, rules nameRules) string {
	artist := ""
	if i := strings.IndexByte(pl.final, '/'); i > 0 {
		artist = pl.final[:i]
	}
	if pl.song.ArtistSource == tab.FromFile && !rules.junkArtistName(pl.song.Artist) && (inSkipped(pl.final) || artist == "") {
		artist = pl.song.Artist
	}
	return artist
}
