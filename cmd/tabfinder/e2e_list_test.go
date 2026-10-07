package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"tab-sync/internal/finder"
	"tab-sync/internal/tab"
	"tab-sync/internal/testlib"
)

func opener() map[string]string {
	return map[string]string{"tuxguitar": record, "gsettings": `printf "'prefer-dark'\n"`}
}

// E-DSK-20
func TestMenuClicksNeverOpenASong(t *testing.T) {
	h := newHarnessWith(t, harnessOpts{index: indexOf(testlib.Songs()), tools: opener()})
	fr := h.fieldRect(&h.u.tuning)
	if h.rowRect(0).Empty() {
		t.Fatal("no rows below the menu")
	}
	reopen := func() {
		h.u.clearFilters()
		h.press(key.NameEscape)
		h.clickRect(fr)
		if !h.open(&h.u.tuning) {
			t.Fatal("menu not open")
		}
	}
	reopen()
	// The first line is a section header: clicking it does nothing at all.
	h.click(float32(fr.Min.X+100), float32(fr.Max.Y+4+4+18))
	if h.u.in.Active() {
		t.Errorf("a click on a header picked something: %+v", h.u.in)
	}
	// Walk down the menu: headers, dividers, items. Items pick; nothing opens a song.
	picked := 0
	for y := fr.Max.Y + 8; y < fr.Max.Y+4+440; y += 9 {
		reopen()
		h.click(float32(fr.Min.X+120), float32(y))
		if h.u.in.Tuning != "" {
			picked++
		}
	}
	if picked == 0 {
		t.Error("no click picked an item: the sweep missed the menu")
	}
	// A click on the menu's border and in the gap above its first line.
	reopen()
	h.click(float32(fr.Min.X+5), float32(fr.Max.Y+5))
	reopen()
	h.click(float32(fr.Max.X-2), float32(fr.Max.Y+30))
	neverCalled(t, h.toolDir, "tuxguitar")
	// For contrast: a click at the same place with the menu closed opens the song under it.
	h.u.clearFilters()
	h.press(key.NameEscape)
	if h.open(&h.u.tuning) {
		t.Fatal("menu still open")
	}
	h.click(float32(fr.Min.X+120), 260)
	calledWith(t, h.toolDir, "tuxguitar")
}

// E-DSK-23
func TestRowContent(t *testing.T) {
	songs := testlib.Songs()
	lib := finder.New(songs)
	by := map[string]finder.Entry{}
	for _, e := range lib.Entries {
		by[e.Song.Title] = e
	}
	for title, want := range map[string]string{
		"Brass Kettle": "Soilbed Quartet · Glass Orchard",
		"Eight String": "Merrowgate", // no album: no dot
		"Nine String":  "Nine",
		"Broken Song":  "Broken",
	} {
		if got := rowSubtitle(by[title].Song); got != want {
			t.Errorf("%s: subtitle %q, want %q", title, got, want)
		}
	}
	if got := rowSubtitle(&tab.Song{Artist: "  ", Album: "Album"}); got != "Album" {
		t.Errorf("blank artist: %q", got)
	}
	if got := rowSubtitle(&tab.Song{}); got != "" {
		t.Errorf("nothing: %q", got)
	}
	for _, tt := range []struct {
		bpms      []string
		main, sub string
	}{
		{nil, "", ""},
		{[]string{"190"}, "190", "BPM"},
		{[]string{"190", "145"}, "190", "→ 145"},
		{[]string{"190", "145", "100"}, "190", "→ 145 100"},
		{[]string{"190", "145", "100", "80"}, "190", "→ 145 100 …"},
		{[]string{"120.5"}, "120.5", "BPM"},
	} {
		if m, s := tempoParts(tt.bpms); m != tt.main || s != tt.sub {
			t.Errorf("tempoParts(%v) = %q, %q; want %q, %q", tt.bpms, m, s, tt.main, tt.sub)
		}
	}
	if !unreadable(by["Broken Song"]) {
		t.Error("the unparseable file isn't shown as unreadable")
	}
	for _, title := range []string{"Brass Kettle", "Eight String"} {
		if unreadable(by[title]) {
			t.Errorf("%s is shown as unreadable", title)
		}
	}
	// An error with something read before it is not "couldn't read".
	partial := finder.New([]*tab.Song{{Path: "a.gp5", Error: "tempo changes incomplete", Tracks: []tab.Track{testlib.EStd6}}})
	if unreadable(partial.Entries[0]) {
		t.Error("a partly read file is shown as unreadable")
	}
	// Tuning tags: one per distinct tuning, bigger string counts first.
	var tags []string
	for _, tu := range by["Brass Kettle"].Tunings {
		tags = append(tags, fmt.Sprintf("%d %s", tu.Strings, tu.Label()))
	}
	if !slices.Equal(tags, []string{"6 Drop C", "4 Drop C"}) {
		t.Errorf("tags = %q", tags)
	}
	// Every row draws, scrolled through, in a narrow and a wide window.
	for _, w := range []int{360, 1000, 2000} {
		h := newHarnessWith(t, harnessOpts{index: indexOf(songs), size: image.Pt(w, 900)})
		for range 5 {
			h.frame()
		}
		if got := len(h.u.result.Matches); got != len(songs) {
			t.Errorf("%d dp: %d rows", w, got)
		}
		h.scrollList(3000)
	}
}

func (h *harness) scrollList(dy float32) {
	p := f32.Pt(float32(h.winDp().X/2)*scale, 600*scale)
	h.now = h.now.Add(time.Millisecond)
	h.r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p, Time: h.now.Sub(time.Unix(0, 0))})
	h.frame()
	h.r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: p, Scroll: f32.Pt(0, dy*scale), Time: h.now.Sub(time.Unix(0, 0)) + time.Millisecond})
	h.frame()
	h.frame()
}

func treeHarness(t *testing.T, tools map[string]string) *harness {
	t.Helper()
	root := testlib.Tree(t)
	h := newHarnessWith(t, harnessOpts{root: &root, noIndex: true, tools: tools})
	h.waitFor("the scan", func() bool { return !h.u.scanning && len(h.u.lib.Entries) == 20 })
	return h
}

// E-DSK-24
func TestOpenSong(t *testing.T) {
	t.Run("correctly named: the original", func(t *testing.T) {
		h := treeHarness(t, opener())
		h.typeInto(&h.u.name, "parade")
		h.press(key.NameEscape)
		if got := h.titles(); len(got) != 1 {
			t.Fatalf("titles = %q", got)
		}
		h.clickRect(h.rowRect(0))
		want := filepath.Join(h.u.root, "Soilbed Quartet - Rust Parade.gp3")
		if args := calledWith(t, h.toolDir, "tuxguitar"); !slices.Equal(args, []string{want}) {
			t.Errorf("args = %q, want %q", args, want)
		}
	})
	t.Run("misnamed: a copy under the real name", func(t *testing.T) {
		h := treeHarness(t, opener())
		h.typeInto(&h.u.name, "quartz")
		h.press(key.NameEscape)
		h.clickRect(h.rowRect(0))
		want := filepath.Join(cacheDir, "open", "Quartz.gp") // the file inside is Guitar Pro 7: named after what it really is
		args := calledWith(t, h.toolDir, "tuxguitar")
		if !slices.Equal(args, []string{want}) {
			t.Fatalf("args = %q, want %q", args, want)
		}
		orig, _ := os.ReadFile(filepath.Join(h.u.root, "Gorsewick", "Gravel Hymns", "Quartz.gpx.crdownload"))
		if got, _ := os.ReadFile(want); string(got) != string(orig) || len(orig) == 0 {
			t.Error("the copy differs from the original")
		}
	})
	t.Run("only a click opens, a hover doesn't", func(t *testing.T) {
		h := treeHarness(t, opener())
		h.hoverAt(500, 300)
		h.hoverAt(520, 330)
		neverCalled(t, h.toolDir, "tuxguitar")
	})
}

// E-DSK-25
func TestTuxGuitarMissingSnackbar(t *testing.T) {
	h := treeHarness(t, map[string]string{"gsettings": `printf "'prefer-dark'\n"`})
	h.typeInto(&h.u.name, "parade")
	h.press(key.NameEscape)
	h.clickRect(h.rowRect(0))
	if h.u.message != "TuxGuitar is not installed" {
		t.Fatalf("message = %q", h.u.message)
	}
	h.advance(3900 * time.Millisecond)
	if h.u.message == "" {
		t.Error("the message is gone before 4 s")
	}
	h.advance(200 * time.Millisecond)
	if h.u.message != "" {
		t.Errorf("the message is still there after 4 s: %q", h.u.message)
	}
	// A new message gets its own 4 s.
	h.u.show("again")
	h.advance(2 * time.Second)
	if h.u.message != "again" {
		t.Error("new message gone early")
	}
	h.advance(2100 * time.Millisecond)
	if h.u.message != "" {
		t.Error("new message stays")
	}
}

// E-DSK-26
func TestHoverAndLongList(t *testing.T) {
	h := libHarness(t)
	r0, r1 := h.rowRect(0), h.rowRect(1)
	c0, c1 := center(r0), center(r1)
	h.hoverAt(int(c0.X), int(c0.Y))
	if !h.u.rows[0].Hovered() || h.u.rows[1].Hovered() {
		t.Errorf("hover over row 0: %v %v", h.u.rows[0].Hovered(), h.u.rows[1].Hovered())
	}
	h.hoverAt(int(c1.X), int(c1.Y))
	if h.u.rows[0].Hovered() || !h.u.rows[1].Hovered() {
		t.Errorf("hover over row 1: %v %v", h.u.rows[0].Hovered(), h.u.rows[1].Hovered())
	}

	var songs []*tab.Song
	for i := range 1000 {
		songs = append(songs, &tab.Song{
			Path: fmt.Sprintf("Band %03d/Album/Song %04d.gp5", i%50, i), Format: "gp5", Artist: fmt.Sprintf("Band %03d", i%50), Album: "Album", Title: fmt.Sprintf("Song %04d", i),
			Tracks: []tab.Track{testlib.EStd6, testlib.Bass4}, Tempos: []tab.Tempo{{Bar: 1, BPM: float64(80 + i%100)}, {Bar: 9, BPM: 120}},
		})
	}
	big := libHarness(t, songs...)
	if len(big.u.result.Matches) != 1000 {
		t.Fatalf("%d matches", len(big.u.result.Matches))
	}
	var times []time.Duration
	p := f32.Pt(500*scale, 500*scale)
	big.hoverAt(500, 500)
	for i := 0; i < 60; i++ {
		big.now = big.now.Add(time.Millisecond)
		big.r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: p, Scroll: f32.Pt(0, 97*scale), Time: big.now.Sub(time.Unix(0, 0))})
		start := time.Now()
		big.frame()
		times = append(times, time.Since(start))
	}
	if big.u.list.Position.First < 50 {
		t.Errorf("scrolled only to row %d", big.u.list.Position.First)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	median, worst := times[len(times)/2], times[len(times)-1]
	t.Logf("frames while scrolling 1000 songs: median %v, worst %v", median, worst)
	if median > 16*time.Millisecond {
		t.Errorf("median frame %v: more than a 60 Hz frame (are all rows laid out?)", median)
	}
	// A frame of the long list costs about what one of a short list does: only visible rows are laid out.
	small := libHarness(t, songs[:20]...)
	var smallTimes []time.Duration
	for range 30 {
		start := time.Now()
		small.frame()
		smallTimes = append(smallTimes, time.Since(start))
	}
	sort.Slice(smallTimes, func(i, j int) bool { return smallTimes[i] < smallTimes[j] })
	if m := smallTimes[len(smallTimes)/2]; median > 8*m+2*time.Millisecond {
		t.Errorf("1000 songs: median frame %v vs %v for 20", median, m)
	}
}
