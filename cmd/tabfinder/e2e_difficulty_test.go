package main

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/finder"
)

// drag presses at from, moves to to in a few steps and lets go; points in dp.
func (h *harness) drag(from, to f32.Point) {
	h.now = h.now.Add(time.Second)
	at := func() time.Duration { h.now = h.now.Add(16 * time.Millisecond); return h.now.Sub(time.Unix(0, 0)) }
	px := func(p f32.Point) f32.Point { return p.Mul(scale) }
	h.r.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: px(from), Time: at()},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: px(from), Time: at()},
	)
	h.frame()
	for i := 1; i <= 5; i++ {
		p := from.Add(to.Sub(from).Mul(float32(i) / 5))
		h.r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: px(p), Time: at()})
		h.frame()
	}
	h.r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: px(to), Time: at()})
	h.frame()
	h.frame()
}

func (h *harness) winArea() rect { return rect{Max: h.winDp()} }

// sliderRect finds a role's slider in the open popup.
func (h *harness) sliderRect(r difficulty.Role) rect {
	h.t.Helper()
	s := h.u.sliders[r]
	got := h.locate(h.winArea(), 8, func() bool { return s.hovered })
	if got.Empty() {
		h.t.Fatalf("%s slider not found", r)
	}
	return got
}

// thumbAt is where a slider's thumb for level v sits, in dp.
func thumbAt(r rect, v int) f32.Point {
	pad := float32(sliderPad)
	return f32.Pt(float32(r.Min.X)+pad+sliderX(v, float32(r.Dx())-2*pad), float32(r.Min.Y+r.Max.Y)/2)
}

func (h *harness) openLevels() {
	h.t.Helper()
	h.clickRect(h.buttonRect(h.filterArea(), &h.u.levelsBtn))
	if !h.u.levelsOpen {
		h.t.Fatal("the difficulty popup didn't open")
	}
}

// E-DSK-31
func TestDifficultyPopup(t *testing.T) {
	h := libHarness(t)
	h.openLevels()
	r := h.sliderRect(difficulty.Rhythm)
	h.drag(thumbAt(r, 1), thumbAt(r, 5))
	h.drag(thumbAt(r, 10), thumbAt(r, 7))
	if h.u.in.Rhythm.Level != "5-7" {
		t.Errorf("rhythm level = %q, want 5-7", h.u.in.Rhythm.Level)
	}
	// The list follows while the popup is open.
	want := []string{"Embrace the Unseen", "Only for the Brave", "Where Rivers Seem to Rest"}
	if got := sorted(h.titles()); !slices.Equal(got, want) {
		t.Errorf("titles = %q, want %q", got, want)
	}
	h.shot("31-difficulty-popup")
	if c := h.u.counter(); c != " 3 / 11" {
		t.Errorf("counter = %q", c)
	}
	h.press(key.NameEscape)
	if h.u.levelsOpen {
		t.Error("Escape didn't close the popup")
	}
	if h.u.in.Rhythm.Level != "5-7" {
		t.Errorf("closing dropped the level: %q", h.u.in.Rhythm.Level)
	}

	// Reopened, it shows the range; a click on the track moves the nearer thumb there.
	h.openLevels()
	r = h.sliderRect(difficulty.Rhythm)
	h.clickRect(rect{Min: thumbAt(r, 9).Round(), Max: thumbAt(r, 9).Round().Add(image.Pt(1, 1))})
	if lo, hi := h.u.level(difficulty.Rhythm); lo != 5 || hi != 9 {
		t.Errorf("after a click on 9: %d–%d, want 5–9", lo, hi)
	}
	h.clickRect(h.buttonRect(h.winArea(), &h.u.levelsDone))
	if h.u.levelsOpen {
		t.Error("Done didn't close the popup")
	}

	// A click beside the popup closes it, and opens no song under it.
	h.openLevels()
	card := h.locate(h.winArea(), 12, func() bool { return h.u.cardHovered })
	if card.Empty() {
		t.Fatal("popup not found")
	}
	h.click(float32(card.Min.X+card.Max.X)/2, float32(card.Max.Y+40)) // over the list
	if h.u.levelsOpen {
		t.Error("a click beside the popup didn't close it")
	}
	if h.u.message != "" {
		t.Errorf("the click reached the list: %q", h.u.message)
	}
	// A click on the popup itself, off its controls, leaves it open.
	h.openLevels()
	h.click(float32(card.Min.X+8), float32(card.Min.Y+8))
	if !h.u.levelsOpen {
		t.Error("a click on the popup closed it")
	}
	// Reset clears every role's range.
	h.u.setLevel(difficulty.Drums, 2, 3)
	h.clickRect(h.buttonRect(h.winArea(), &h.u.levelsReset))
	if h.u.in.Active() {
		t.Errorf("after Reset: %+v", h.u.in)
	}
}

// E-DSK-32
func TestDifficultyChips(t *testing.T) {
	h := libHarness(t)
	h.u.setLevel(difficulty.Rhythm, 5, 7)
	h.u.setLevel(difficulty.Drums, 7, 10)
	h.frame()
	h.shot("32-difficulty-chips")
	if got := h.u.chips(); len(got) != 2 {
		t.Fatalf("chips = %+v", got)
	}
	drums := h.buttonRect(h.filterArea(), h.u.chipClear[difficulty.Drums])
	rhythm := h.buttonRect(h.filterArea(), h.u.chipClear[difficulty.Rhythm])
	levels := h.buttonRect(h.filterArea(), &h.u.levelsBtn)
	if !(levels.Max.X <= drums.Min.X && drums.Max.X <= rhythm.Min.X) || drums.Min.Y < levels.Min.Y || drums.Max.Y > levels.Max.Y {
		t.Errorf("button %v, chips' ✕ %v %v: want them in a line after the button", levels, drums, rhythm)
	}
	h.clickRect(drums)
	if h.u.in.Drums.Level != "" || h.u.in.Rhythm.Level != "5-7" {
		t.Errorf("after the drums' ✕: %+v", h.u.in)
	}
	if got := len(h.titles()); got != 3 {
		t.Errorf("%d titles, want 3", got)
	}
	// "Clear filters" clears the ranges too.
	h.u.setLevel(difficulty.Lead, 10, 10) // none that hard
	h.frame()
	if len(h.titles()) != 0 {
		t.Fatalf("titles = %q", h.titles())
	}
	h.clickRect(h.buttonRect(h.bodyArea(), &h.u.cta))
	if h.u.in.Active() || len(h.u.chips()) != 0 {
		t.Errorf("after clearing the filters: %+v", h.u.in)
	}
}

// E-DSK-33
func TestSortMenu(t *testing.T) {
	h := libHarness(t)
	az := h.titles()
	pick := func(o finder.Sort) {
		t.Helper()
		h.clickRect(h.buttonRect(h.topBarArea(), &h.u.sortBtn))
		if !h.u.sortOpen {
			t.Fatal("the sort menu didn't open")
		}
		h.shot("33-sort-menu")
		i := slices.Index(sorts, o)
		h.clickRect(h.buttonRect(h.winArea(), &h.u.sortItems[i]))
		if h.u.sortOpen {
			t.Error("the sort menu stays open after a pick")
		}
	}
	pick(finder.SortHardest)
	got := h.titles()
	if got[0] != "Brass Kettle" || got[len(got)-1] == "Paper Ride" {
		t.Errorf("hardest first: %q", got)
	}
	if h.u.sortLabel() != "Hardest" {
		t.Errorf("label = %q", h.u.sortLabel())
	}
	pick(finder.SortEasiest)
	if got := h.titles(); got[0] != "Paper Ride" {
		t.Errorf("easiest first: %q", got)
	}
	// Songs without parts come last either way.
	if got := h.titles(); !slices.Contains(got[6:], "Eight String") {
		t.Errorf("unrated songs not last: %q", got)
	}
	pick(finder.SortAZ)
	if got := h.titles(); !slices.Equal(got, az) {
		t.Errorf("A–Z: %q, want %q", got, az)
	}
	if h.u.sortLabel() != "A–Z" {
		t.Errorf("label = %q", h.u.sortLabel())
	}
	// A click beside the open menu closes it, changes nothing and opens no song.
	h.clickRect(h.buttonRect(h.topBarArea(), &h.u.sortBtn))
	h.click(100, 500)
	if h.u.sortOpen || h.u.in.Sort != finder.SortAZ || h.u.message != "" {
		t.Errorf("after a click beside the menu: open %v, sort %q, message %q", h.u.sortOpen, h.u.in.Sort, h.u.message)
	}
}
