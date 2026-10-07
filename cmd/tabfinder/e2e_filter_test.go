package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"

	"tabfinder/internal/tab"
	"tabfinder/internal/testlib"
)

// indexOf is songs as the cached index file has them.
func indexOf(songs []*tab.Song) *string {
	var buf bytes.Buffer
	for _, s := range songs {
		b, _ := json.Marshal(s)
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return str(buf.String())
}

// libHarness starts the app on a cached index of the shared fixture library.
func libHarness(t *testing.T, songs ...*tab.Song) *harness {
	t.Helper()
	if len(songs) == 0 {
		songs = testlib.Songs()
	}
	return newHarnessWith(t, harnessOpts{index: indexOf(songs)})
}

func menuTexts(m *menu) []string {
	var out []string
	for _, it := range m.items {
		out = append(out, it.text)
	}
	return out
}

func menuSections(m *menu) []string {
	var out []string
	for _, it := range m.items {
		if it.section != "" && (len(out) == 0 || out[len(out)-1] != it.section) {
			out = append(out, it.section)
		}
	}
	return out
}

// open reports whether a field's suggestion menu is showing.
func (h *harness) open(f *field) bool {
	return h.r.Source().Focused(&f.editor) && !f.dismissed
}

// textWidth shapes s with the app's fonts, for comparing what is drawn.
func textWidth(st style, s string) fixed.Int26_6 {
	shaper := newShaper()
	shaper.LayoutString(text.Parameters{Font: st.font, PxPerEm: fixed.I(int(st.size) * 2), MaxWidth: 1 << 20}, s)
	var w fixed.Int26_6
	for {
		g, ok := shaper.NextGlyph()
		if !ok {
			return w
		}
		w += g.Advance
	}
}

// E-DSK-10
func TestFilterEachFieldAlone(t *testing.T) {
	for _, tt := range []struct {
		name  string
		field func(*ui) *field
		text  string
		want  []string
	}{
		{"artist", func(u *ui) *field { return &u.artist }, "inkwell flamingos", []string{"Paper Ride", "Embrace the Unseen", "Only for the Brave"}},
		{"artist case", func(u *ui) *field { return &u.artist }, "SOILBED QUARTET", []string{"Brass Kettle"}},
		{"song", func(u *ui) *field { return &u.name }, "eight", []string{"Eight String"}},
		{"song words", func(u *ui) *field { return &u.name }, "brave only", []string{"Only for the Brave"}},
		{"tuning", func(u *ui) *field { return &u.tuning }, "drop c", []string{"Paper Ride", "Brass Kettle"}},
		{"tuning by notes", func(u *ui) *field { return &u.tuning }, "d g d g b d", []string{"Where Rivers Seem to Rest"}},
		{"bpm", func(u *ui) *field { return &u.bpm }, "190", []string{"Brass Kettle"}},
		{"bpm range", func(u *ui) *field { return &u.bpm }, "100-120", []string{"Paper Ride", "Eight String", testlib.LongTitle, "Only for the Brave"}},
		{"bpm open", func(u *ui) *field { return &u.bpm }, "150-", []string{"Brass Kettle", "Only for the Brave", "Ruf nach Sonne"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := libHarness(t)
			h.typeInto(tt.field(h.u), tt.text)
			h.press(key.NameEscape)
			got := h.titles()
			slices.Sort(got)
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("titles = %q, want %q", got, want)
			}
			if c := h.u.counter(); !strings.HasPrefix(c, fmt.Sprintf("%2d / 11", len(want))) {
				t.Errorf("counter = %q", c)
			}
		})
	}
}

// The counter is padded so it is as wide for 0 as for the total.
func TestCounterWidth(t *testing.T) {
	h := libHarness(t)
	st := style{monoBold, 16}
	full := h.u.counter()
	if full != "11 / 11" {
		t.Fatalf("counter = %q", full)
	}
	for _, name := range []string{"zzzz", "eight", "inkwell flamingos", ""} {
		h.u.name.editor.SetText(name)
		h.frame()
		h.u.in.Name = name
		h.frame()
		c := h.u.counter()
		if len(c) != len(full) {
			t.Errorf("%q: counter %q is a different length than %q", name, c, full)
		}
		if textWidth(st, c) != textWidth(st, full) {
			t.Errorf("%q: counter %q is drawn a different width than %q", name, c, full)
		}
		if !strings.HasSuffix(c, " / 11") {
			t.Errorf("counter = %q", c)
		}
	}
	h.u.in.Name = "zzzz"
	h.frame()
	if c := h.u.counter(); c != " 0 / 11" {
		t.Errorf("counter for no matches = %q (right aligned numbers)", c)
	}
}

// E-DSK-11
func TestFilterAllFieldsAndClear(t *testing.T) {
	h := libHarness(t)
	h.typeInto(&h.u.artist, "inkwell flamingos")
	h.press(key.NameEscape)
	h.typeInto(&h.u.name, "paper")
	h.typeInto(&h.u.tuning, "drop c") // a tuning filter needs the whole name; "drop" only suggests
	h.press(key.NameEscape)
	h.typeInto(&h.u.bpm, "100-150")
	if got := h.titles(); len(got) != 1 || got[0] != "Paper Ride" {
		t.Fatalf("all four: %q", got)
	}
	// ✕ on the song field: that filter goes, the list widens, the field has the focus.
	h.clickRect(h.buttonRect(h.filterArea(), &h.u.name.clear))
	if _, name, _, _ := h.fields(); name != "" || h.u.in.Name != "" {
		t.Errorf("song field %q, query %q", name, h.u.in.Name)
	}
	if !h.r.Source().Focused(&h.u.name.editor) {
		t.Error("the cleared field isn't focused")
	}
	if got := h.titles(); len(got) != 1 { // Paper Ride is the only Drop C song of Inkwell Flamingos in that range
		t.Errorf("after clearing the song: %q", got)
	}
	if h.u.in.Artist != "inkwell flamingos" || h.u.in.Tuning != "drop c" || h.u.in.BPM != "100-150" {
		t.Errorf("other filters changed: %+v", h.u.in)
	}
	// Each of the others too.
	for i, f := range []*field{&h.u.bpm, &h.u.tuning, &h.u.artist} {
		h.clickRect(h.buttonRect(h.filterArea(), &f.clear))
		want := []int{1, 3, 11}[i] // artist and tuning; the artist; everything
		if f.editor.Text() != "" || !h.r.Source().Focused(&f.editor) || len(h.u.result.Matches) != want {
			t.Errorf("clearing %s: text %q focused %v, %d matches, want %d", f.label, f.editor.Text(), h.r.Source().Focused(&f.editor), len(h.u.result.Matches), want)
		}
	}
	if h.u.in.Active() || len(h.u.result.Matches) != 11 {
		t.Errorf("everything cleared: %+v, %d matches", h.u.in, len(h.u.result.Matches))
	}
}

// E-DSK-12
func TestInvalidBPM(t *testing.T) {
	h := libHarness(t)
	h.typeInto(&h.u.bpm, "fast")
	if !h.u.result.BPMInvalid || len(h.u.result.Matches) != 11 {
		t.Errorf("fast: invalid %v, %d matches (the filter is ignored)", h.u.result.BPMInvalid, len(h.u.result.Matches))
	}
	for _, v := range []string{"100-", "1-2-3", "140-100", "100-120"} {
		h.u.bpm.editor.SetText(v)
		h.u.in.BPM = v
		h.frame()
		invalid := v == "1-2-3" || v == "140-100"
		if h.u.result.BPMInvalid != invalid {
			t.Errorf("%q: invalid = %v", v, h.u.result.BPMInvalid)
		}
		if invalid && len(h.u.result.Matches) != 11 || v == "100-120" && len(h.u.result.Matches) != 4 {
			t.Errorf("%q: %d matches", v, len(h.u.result.Matches))
		}
	}
	// The error clears together with the text.
	h.clickRect(h.buttonRect(h.filterArea(), &h.u.bpm.clear))
	if h.u.result.BPMInvalid {
		t.Error("still invalid after clearing")
	}
}

// E-DSK-13
func TestNoMatchesAndClearFilters(t *testing.T) {
	h := libHarness(t)
	h.typeInto(&h.u.artist, "inkwell flamingos")
	h.press(key.NameEscape)
	h.typeInto(&h.u.name, "zzzz")
	h.typeInto(&h.u.tuning, "drop")
	h.press(key.NameEscape)
	h.typeInto(&h.u.bpm, "100")
	if len(h.u.result.Matches) != 0 {
		t.Fatalf("%d matches", len(h.u.result.Matches))
	}
	h.clickRect(h.buttonRect(h.bodyArea(), &h.u.cta)) // "Clear filters"
	h.frame()
	a, n, tu, b := h.fields()
	if a+n+tu+b != "" || h.u.in.Active() || len(h.u.result.Matches) != 11 {
		t.Errorf("fields %q %q %q %q, query %+v, %d matches", a, n, tu, b, h.u.in, len(h.u.result.Matches))
	}
	// A picked tuning's string count is cleared too.
	h.u.in.Strings = 6
	h.u.in.Name = "zzzz"
	h.frame()
	h.clickRect(h.buttonRect(h.bodyArea(), &h.u.cta))
	if h.u.in.Strings != 0 || h.u.in.Active() {
		t.Errorf("query = %+v", h.u.in)
	}
}

// E-DSK-14
func TestArtistSuggestions(t *testing.T) {
	h := libHarness(t)
	h.clickField(&h.u.artist)
	if !h.open(&h.u.artist) {
		t.Fatal("focusing the field doesn't open the menu")
	}
	all := menuTexts(h.u.artistMenu())
	if !slices.Equal(all, h.u.lib.Artists) || len(all) != 9 {
		t.Errorf("all artists = %q", all)
	}
	n := 0
	for _, a := range all {
		if strings.EqualFold(a, "inkwell flamingos") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d spellings of Inkwell Flamingos", n)
	}
	h.typ("in")
	got := menuTexts(h.u.artistMenu())
	if len(got) < 2 || got[0] != "Inkwell Flamingos" {
		t.Fatalf("suggestions for 'in' = %q", got)
	}
	seenSubstring := false
	for _, a := range got {
		prefix := strings.HasPrefix(strings.ToLower(a), "in")
		if prefix && seenSubstring {
			t.Errorf("a prefix match after substring matches: %q", got)
		}
		if !prefix {
			seenSubstring = true
		}
		if !strings.Contains(strings.ToLower(a), "in") {
			t.Errorf("%q doesn't contain what was typed", a)
		}
	}
	// An exact match, in any case, isn't suggested.
	h.typ(" flames")
	if got := menuTexts(h.u.artistMenu()); slices.ContainsFunc(got, func(a string) bool { return strings.EqualFold(a, "inkwell flamingos") }) {
		t.Errorf("exact match suggested: %q", got)
	}
}

// E-DSK-15
func TestPickArtistSuggestion(t *testing.T) {
	h := libHarness(t)
	h.clickField(&h.u.artist)
	first := menuTexts(h.u.artistMenu())[0]
	h.clickRect(h.menuItemRect(&h.u.artist, 0))
	if h.u.in.Artist != first || h.u.artist.editor.Text() != first {
		t.Fatalf("query %q, field %q, want %q", h.u.in.Artist, h.u.artist.editor.Text(), first)
	}
	if h.u.artist.dismissed != true {
		t.Error("the menu is still open after a pick")
	}
	if got := len(h.u.result.Matches); got != 1 {
		t.Errorf("%d matches for %s", got, first)
	}
	// The field's change event from the pick isn't typing: it must not reopen the menu.
	for range 5 {
		h.frame()
	}
	if !h.u.artist.dismissed || h.u.artist.picked != "" {
		t.Errorf("dismissed %v picked %q after the pick", h.u.artist.dismissed, h.u.artist.picked)
	}
	// Typing again does.
	h.typ("x")
	if h.u.artist.dismissed {
		t.Error("typing doesn't reopen the menu")
	}
}

// E-DSK-16
func TestSuggestionKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    func(*ui) *field
	}{{"artist", func(u *ui) *field { return &u.artist }}, {"tuning", func(u *ui) *field { return &u.tuning }}} {
		t.Run(tc.name, func(t *testing.T) {
			h := libHarness(t)
			f := tc.f(h.u)
			h.clickField(f)
			if !h.open(f) {
				t.Fatal("focus doesn't open the menu")
			}
			h.press(key.NameEscape)
			if h.open(f) {
				t.Error("Escape doesn't close")
			}
			h.clickField(f)
			if !h.open(f) {
				t.Error("clicking the focused field doesn't reopen")
			}
			h.press(key.NameEscape)
			h.press(key.NameDownArrow)
			if !h.open(f) {
				t.Error("the down arrow doesn't reopen")
			}
			// Enter picks the first suggestion.
			var want string
			if f == &h.u.artist {
				want = menuTexts(h.u.artistMenu())[0]
			} else {
				want = h.u.tuningMenu().items[0].text
			}
			h.press(key.NameReturn)
			if f.editor.Text() != want {
				t.Errorf("Enter: field %q, want %q", f.editor.Text(), want)
			}
			if h.open(f) {
				t.Error("the menu stays open after Enter")
			}
			// Enter with the menu closed does nothing more.
			before := h.u.in
			h.press(key.NameReturn)
			if h.u.in != before {
				t.Errorf("a second Enter changed the query: %+v -> %+v", before, h.u.in)
			}
		})
	}
	t.Run("Enter with nothing to pick", func(t *testing.T) {
		h := libHarness(t)
		h.typeInto(&h.u.artist, "zzzz")
		h.press(key.NameReturn)
		if h.u.artist.editor.Text() != "zzzz" || h.u.in.Artist != "zzzz" {
			t.Errorf("field %q query %q", h.u.artist.editor.Text(), h.u.in.Artist)
		}
	})
}

// E-DSK-17
func TestTuningSuggestions(t *testing.T) {
	h := libHarness(t)
	h.clickField(&h.u.tuning)
	m := h.u.tuningMenu()
	if got, want := menuSections(m), []string{"6 STRINGS", "7 STRINGS", "8 STRINGS", "9 STRINGS", "4 STRINGS", "5 STRINGS"}; !slices.Equal(got, want) {
		t.Errorf("sections = %q, want %q", got, want)
	}
	// Grouped: each section's items are together.
	seen := map[string]bool{}
	last := ""
	for _, it := range m.items {
		if it.section != last {
			if seen[it.section] {
				t.Errorf("section %s appears twice", it.section)
			}
			seen[it.section] = true
			last = it.section
		}
	}
	byText := map[string]menuItem{}
	for _, it := range m.items {
		byText[it.section+"/"+it.text] = it
	}
	if it := byText["6 STRINGS/Drop C"]; it.detail != "C G C F A D" {
		t.Errorf("Drop C detail = %q", it.detail)
	}
	if it, ok := byText["6 STRINGS/D G D G B D"]; !ok || it.detail != "" {
		t.Errorf("a custom tuning is labelled by its notes, with no detail: %+v (found %v)", it, ok)
	}
	if it, ok := byText["8 STRINGS/F# B E A D G B E"]; !ok || it.detail != "" {
		t.Errorf("8 string custom: %+v", it)
	}
	// Typing filters by label or notes.
	h.typ("drop")
	for _, it := range h.u.tuningMenu().items {
		if !strings.Contains(strings.ToLower(it.text+" "+it.detail), "drop") {
			t.Errorf("%+v doesn't match 'drop'", it)
		}
	}
	if got := len(h.u.tuningMenu().items); got != 3 { // Drop D 6, Drop C 6, Drop C 4
		t.Errorf("%d suggestions for drop", got)
	}
	for range 4 {
		h.press(key.NameDeleteBackward)
	}
	h.typ("e a d")
	got := menuTexts(h.u.tuningMenu())
	if len(got) < 3 {
		t.Errorf("by notes: %q", got)
	}
	for _, it := range h.u.tuningMenu().items {
		if !strings.Contains(strings.ToLower(it.text+" "+it.detail), "e a d") {
			t.Errorf("%+v doesn't match 'e a d'", it)
		}
	}
}

// E-DSK-18
func TestPickTuning(t *testing.T) {
	h := libHarness(t)
	h.clickField(&h.u.tuning)
	idx := slices.IndexFunc(h.u.tuningMenu().items, func(it menuItem) bool { return it.text == "Drop C" && it.section == "4 STRINGS" })
	if idx < 0 {
		t.Fatal("no 4 string Drop C")
	}
	// Scroll it into view if it's below the fold, then click it.
	h.scrollMenu(&h.u.tuning, 2000)
	h.clickRect(h.menuItemRect(&h.u.tuning, idx))
	if h.u.in.Tuning != "Drop C" || h.u.in.Strings != 4 {
		t.Fatalf("query = %+v", h.u.in)
	}
	if got := h.titles(); !slices.Equal(got, []string{"Brass Kettle"}) { // the 6 string Drop C of Paper Ride doesn't count
		t.Errorf("titles = %q", got)
	}
	if h.u.tuning.editor.Text() != "Drop C" || !h.u.tuning.dismissed {
		t.Errorf("field %q dismissed %v", h.u.tuning.editor.Text(), h.u.tuning.dismissed)
	}
	// Typing in the field removes the badge and the string-count filter.
	h.clickField(&h.u.tuning)
	h.typ("x")
	if h.u.in.Strings != 0 || h.u.in.Tuning != "Drop Cx" {
		t.Errorf("after typing: %+v", h.u.in)
	}
	h.press(key.NameDeleteBackward)
	if h.u.in.Strings != 0 || h.u.in.Tuning != "Drop C" {
		t.Errorf("after deleting: %+v", h.u.in)
	}
	if got := len(h.u.result.Matches); got != 2 { // any string count again
		t.Errorf("%d matches", got)
	}
}

// scrollMenu scrolls a field's open menu by dy dp.
func (h *harness) scrollMenu(f *field, dy float32) {
	fr := h.fieldRect(f)
	p := f32.Pt(float32(fr.Min.X+40)*scale, float32(fr.Max.Y+60)*scale)
	h.now = h.now.Add(time.Millisecond)
	h.r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p, Time: h.now.Sub(time.Unix(0, 0))})
	h.frame()
	h.r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: p, Scroll: f32.Pt(0, dy*scale), Time: h.now.Sub(time.Unix(0, 0)) + time.Millisecond})
	h.frame()
	h.frame()
}

// E-DSK-19
func TestArtistTrimsTuningSuggestions(t *testing.T) {
	h := libHarness(t)
	h.clickField(&h.u.tuning)
	all := len(h.u.tuningMenu().items)
	h.press(key.NameEscape)
	h.clickField(&h.u.artist)
	h.press(key.NameReturn) // the first artist, Amber Marsh
	if h.u.in.Artist != "Amber Marsh" {
		t.Fatalf("artist = %q", h.u.in.Artist)
	}
	h.clickField(&h.u.tuning)
	m := h.u.tuningMenu()
	got := menuTexts(m)
	slices.Sort(got)
	if want := []string{"D G D G B D", "E Standard"}; !slices.Equal(got, want) {
		t.Errorf("suggestions for Amber Marsh = %q (of %d before), want %q", got, all, want)
	}
	if len(m.items) >= all {
		t.Errorf("not trimmed: %d of %d", len(m.items), all)
	}
}

// E-DSK-21
func TestManySuggestionsScrollAndCap(t *testing.T) {
	var songs []*tab.Song
	for i := range 200 {
		songs = append(songs, &tab.Song{Path: fmt.Sprintf("b/%03d.gp5", i), Format: "gp5", Artist: fmt.Sprintf("Band %03d", i), Title: fmt.Sprintf("Song %03d", i)})
	}
	h := libHarness(t, songs...)
	h.clickField(&h.u.artist)
	m := h.u.artistMenu()
	if len(m.items) != 60 {
		t.Fatalf("%d suggestions, want 60", len(m.items))
	}
	fr := h.fieldRect(&h.u.artist)
	// The menu is at most 440 dp high: item 0 is on screen, and below the cap nothing is of the menu.
	first := h.menuItemRect(&h.u.artist, 0)
	if first.Min.Y < fr.Max.Y || first.Min.Y > fr.Max.Y+20 {
		t.Errorf("first item at %v, field at %v", first, fr)
	}
	capBottom := fr.Max.Y + 4 + 440
	anyHovered := func() bool {
		for i := range m.items {
			if h.u.artist.menuItems[i].Hovered() {
				return true
			}
		}
		return false
	}
	h.hoverAt(fr.Min.X+40, capBottom-6)
	if !anyHovered() {
		t.Error("the menu doesn't reach its height cap")
	}
	h.hoverAt(fr.Min.X+40, capBottom+14)
	if anyHovered() {
		t.Error("the menu is taller than its cap")
	}
	// The wheel scrolls it.
	if h.u.artist.menuList.Position.First != 0 {
		t.Fatal("not at the top")
	}
	h.scrollMenu(&h.u.artist, 300)
	if h.u.artist.menuList.Position.First == 0 {
		t.Error("the wheel doesn't scroll the menu")
	}
	// ...and doesn't scroll the song list below it.
	if h.u.list.Position.First != 0 {
		t.Errorf("the song list scrolled to %d", h.u.list.Position.First)
	}
	// Past the 60th is never reachable: the cap holds with a longer typed prefix too.
	h.typ("band")
	if got := len(h.u.artistMenu().items); got != 60 {
		t.Errorf("%d suggestions for 'band'", got)
	}
	h.typ(" 19")
	if got := len(h.u.artistMenu().items); got != 10 {
		t.Errorf("%d suggestions for 'band 19'", got)
	}
}

// E-DSK-22
func TestRetypeAfterPick(t *testing.T) {
	t.Run("artist", func(t *testing.T) {
		h := libHarness(t)
		h.clickField(&h.u.artist)
		h.press(key.NameReturn)
		picked := h.u.in.Artist
		h.press(key.NameDeleteBackward)
		if h.u.in.Artist != picked[:len(picked)-1] {
			t.Errorf("after deleting: %q", h.u.in.Artist)
		}
		h.typ(picked[len(picked)-1:])
		if h.u.in.Artist != picked || h.u.artist.editor.Text() != picked {
			t.Errorf("after retyping: query %q field %q", h.u.in.Artist, h.u.artist.editor.Text())
		}
		if len(h.u.result.Matches) != 1 {
			t.Errorf("%d matches", len(h.u.result.Matches))
		}
	})
	t.Run("tuning", func(t *testing.T) {
		h := libHarness(t)
		h.clickField(&h.u.tuning)
		h.press(key.NameReturn)
		picked := h.u.in.Tuning
		if h.u.in.Strings == 0 {
			t.Fatal("no string count after the pick")
		}
		h.press(key.NameDeleteBackward)
		h.typ(picked[len(picked)-1:])
		if h.u.in.Tuning != picked || h.u.in.Strings != 0 {
			t.Errorf("query %+v, want tuning %q and no string count", h.u.in, picked)
		}
	})
}
