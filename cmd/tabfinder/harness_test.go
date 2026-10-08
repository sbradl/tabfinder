package main

import (
	"image"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/widget"
)

// --- finding elements by layout ---
//
// The layout doesn't expose where things are, and a Gio widget can't ask for its
// own position. So tests find elements the way a user's pointer would: moving
// it around until the element reports being hovered, then measuring how far it
// reaches in each direction. Rectangles are in dp.

type rect = image.Rectangle

func center(r rect) f32.Point {
	return f32.Pt(float32(r.Min.X+r.Max.X)/2, float32(r.Min.Y+r.Max.Y)/2)
}

// hoverAt moves the pointer to a point in dp.
func (h *harness) hoverAt(x, y int) {
	h.now = h.now.Add(time.Millisecond)
	h.r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(x*scale), float32(y*scale)), Time: h.now.Sub(time.Unix(0, 0))})
	h.frame()
}

// locate finds the rectangle over which hovered() is true, searching within area
// in coarse steps and then refining the edges. It returns an empty rectangle if
// the element can't be found.
func (h *harness) locate(area rect, step int, hovered func() bool) rect {
	h.t.Helper()
	probe := func(x, y int) bool { h.hoverAt(x, y); return hovered() }
	found := image.Pt(-1, -1)
scan:
	for y := area.Min.Y; y < area.Max.Y; y += step {
		for x := area.Min.X; x < area.Max.X; x += step {
			if probe(x, y) {
				found = image.Pt(x, y)
				break scan
			}
		}
	}
	if found.X < 0 {
		h.hoverAt(-10, -10)
		return rect{}
	}
	win := h.winDp().Sub(image.Pt(1, 1))
	// edge: the farthest point from `from` towards `to` along one axis where the element is still hovered.
	edge := func(from, to int, at func(int) (int, int)) int {
		lo, hi := from, to // lo is inside
		if x, y := at(hi); probe(x, y) {
			return hi
		}
		for abs(hi-lo) > 1 {
			mid := (lo + hi) / 2
			if x, y := at(mid); probe(x, y) {
				lo = mid
			} else {
				hi = mid
			}
		}
		return lo
	}
	r := rect{
		Min: image.Pt(
			edge(found.X, 0, func(x int) (int, int) { return x, found.Y }),
			edge(found.Y, 0, func(y int) (int, int) { return found.X, y }),
		),
		Max: image.Pt(
			edge(found.X, win.X, func(x int) (int, int) { return x, found.Y }),
			edge(found.Y, win.Y, func(y int) (int, int) { return found.X, y }),
		),
	}
	r.Max = r.Max.Add(image.Pt(1, 1))
	h.hoverAt(-10, -10)
	return r
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// winDp is the window size in dp.
func (h *harness) winDp() image.Point { return image.Pt(h.size.X/scale, h.size.Y/scale) }

// The window's regions, in dp, to search for the elements.
func (h *harness) topBarArea() rect { return image.Rect(0, 0, h.winDp().X, 80) }
func (h *harness) filterArea() rect { return image.Rect(0, 80, h.winDp().X, 320) }
func (h *harness) bodyArea() rect   { return image.Rect(0, 80, h.winDp().X, h.winDp().Y) }

func (h *harness) fieldRect(f *field) rect {
	h.t.Helper()
	r := h.locate(h.filterArea(), 12, f.reopen.Hovered)
	if r.Empty() {
		h.t.Fatalf("field %q not found", f.label)
	}
	return r
}

func (h *harness) buttonRect(area rect, b *widget.Clickable) rect {
	h.t.Helper()
	r := h.locate(area, 10, b.Hovered)
	if r.Empty() {
		h.t.Fatal("button not found")
	}
	return r
}

// rowRect finds the i'th visible song row, scanning down a column.
func (h *harness) rowRect(i int) rect {
	h.t.Helper()
	if len(h.u.result.Matches) <= i {
		h.t.Fatalf("row %d: only %d rows", i, len(h.u.result.Matches))
	}
	r := h.locate(image.Rect(h.winDp().X/2, 120, h.winDp().X/2+1, h.winDp().Y), 6, h.u.rowClick(i).Hovered)
	return r
}

// menuItemRect finds the i'th suggestion of an open menu.
func (h *harness) menuItemRect(f *field, i int) rect {
	h.t.Helper()
	fr := h.fieldRect(f)
	r := h.locate(image.Rect(fr.Min.X+20, fr.Max.Y, fr.Min.X+21, 900), 6, func() bool { return f.menuItems[i].Hovered() })
	if r.Empty() {
		h.t.Fatalf("menu item %d of %q not found", i, f.label)
	}
	return r
}

// --- interacting ---

func (h *harness) clickRect(r rect) {
	c := center(r)
	h.click(c.X, c.Y)
}

// clickField clicks a field's middle, which focuses it.
func (h *harness) clickField(f *field) {
	h.t.Helper()
	r := h.fieldRect(f)
	h.click(float32(r.Min.X+r.Dx()/2), float32(r.Min.Y+r.Dy()/2))
}

// waitFor runs frames until cond holds (background work finishing), or fails.
func (h *harness) waitFor(what string, cond func() bool) {
	h.t.Helper()
	for i := 0; i < 500; i++ {
		h.frame()
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("timed out waiting for %s", what)
}

func (h *harness) advance(d time.Duration) {
	h.now = h.now.Add(d)
	h.frame()
}

func (h *harness) typeInto(f *field, text string) {
	h.t.Helper()
	h.clickField(f)
	h.typ(text)
}

// fieldText returns what the four fields show.
func (h *harness) fields() (artist, name, tuning, bpm string) {
	return h.u.artist.editor.Text(), h.u.name.editor.Text(), h.u.tuning.editor.Text(), h.u.bpm.editor.Text()
}

func (h *harness) titles() []string {
	var out []string
	for _, i := range h.u.result.Matches {
		out = append(out, h.u.lib.Entries[i].Song.Title)
	}
	return out
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
