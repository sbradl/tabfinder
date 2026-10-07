package main

import (
	"fmt"
	"image"
	"image/color"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"tabfinder/internal/tab"
	"tabfinder/internal/testlib"
)

// render draws the current state to an image, or skips the test when there is
// no GPU (the headless renderer needs EGL).
func (h *harness) render() *image.RGBA {
	h.t.Helper()
	w, err := headless.NewWindow(h.size.X, h.size.Y)
	if err != nil {
		h.t.Skip("no GPU for offscreen rendering:", err)
	}
	defer w.Release()
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: scale, PxPerSp: scale}, Constraints: layout.Exact(h.size), Source: h.r.Source(), Now: h.now}
	h.u.layout(gtx)
	if err := w.Frame(&ops); err != nil {
		h.t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: h.size})
	if err := w.Screenshot(img); err != nil {
		h.t.Fatal(err)
	}
	return img
}

// px is the pixel at a point given in dp.
func px(img *image.RGBA, x, y int) color.NRGBA {
	c := img.RGBAAt(x*scale, y*scale)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// edge is the pixel on a field's top outline (1 dp strokes sit just above the field's top).
func edge(img *image.RGBA, x, top int) color.NRGBA {
	c := img.RGBAAt(x*scale, top*scale-1)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= 4 && d(a.G, b.G) <= 4 && d(a.B, b.B) <= 4
}

func hex(c color.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

func (h *harness) wantPixel(img *image.RGBA, what string, x, y int, want color.NRGBA) {
	h.t.Helper()
	if got := px(img, x, y); !near(got, want) {
		h.t.Errorf("%s at (%d,%d) dp: %s, want %s", what, x, y, hex(got), hex(want))
	}
}

// E-DSK-27
func TestColorScheme(t *testing.T) {
	for _, tt := range []struct {
		name  string
		tools map[string]string
		want  palette
	}{
		{"dark", map[string]string{"gsettings": `printf "'prefer-dark'\n"`}, dark},
		{"light", map[string]string{"gsettings": `printf "'default'\n"`}, light},
		{"prefer-light", map[string]string{"gsettings": `printf "'prefer-light'\n"`}, light},
		// Not readable: the dark palette, the app's own default (the plan said light here).
		{"no gsettings", map[string]string{}, dark},
		{"gsettings fails", map[string]string{"gsettings": "exit 1"}, dark},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarnessWith(t, harnessOpts{tools: tt.tools})
			if h.u.pal != tt.want {
				t.Fatalf("palette = background %s, want %s", hex(h.u.pal.bg), hex(tt.want.bg))
			}
			img := h.render()
			h.wantPixel(img, "background", 5, 890, tt.want.bg)
			h.wantPixel(img, "top bar", 5, 5, tt.want.panel)
			h.wantPixel(img, "filter panel", 5, 150, tt.want.panel)
			// Focus shows in the accent color.
			h.clickField(&h.u.artist)
			fr := h.fieldRect(&h.u.artist)
			h.wantPixel(h.render(), "focused outline", (fr.Min.X+fr.Max.X)/2, fr.Min.Y, tt.want.accent)
			if tt.want == light && near(light.bg, dark.bg) {
				t.Error("palettes are the same")
			}
		})
	}
}

// E-DSK-12 (the outline), E-DSK-28
func TestFieldOutlinesAndLabels(t *testing.T) {
	h := libHarness(t)
	bpm := h.fieldRect(&h.u.bpm)
	top := func(r rect) (int, int) { return r.Min.X + r.Dx()*3/4, r.Min.Y }
	img := h.render()
	x, y := top(bpm)
	wantEdge := func(img *image.RGBA, what string, want color.NRGBA) {
		t.Helper()
		if got := edge(img, x, y); !near(got, want) {
			t.Errorf("%s: %s, want %s", what, hex(got), hex(want))
		}
	}
	wantEdge(img, "bpm outline", h.u.pal.outline)

	h.typeInto(&h.u.bpm, "fast")
	h.press(key.NameEscape)
	// Invalid: red outline (even while focused), and the text stays.
	h.clickRect(h.fieldRect(&h.u.name))
	img = h.render()
	wantEdge(img, "invalid bpm outline", h.u.pal.err)
	h.u.bpm.editor.SetText("100-120")
	h.u.in.BPM = "100-120"
	h.frame()
	wantEdge(h.render(), "valid bpm outline", h.u.pal.outline)

	// Floating label: inside the empty, unfocused field...
	art := h.fieldRect(&h.u.artist)
	ax, ay := art.Min.X+24, art.Min.Y
	h.u.name.editor.SetText("")
	h.u.in.Name = ""
	h.press(key.NameEscape)
	h.hoverAt(500, 500)
	img = h.render()
	if got := edge(img, ax, ay); !near(got, h.u.pal.outline) {
		t.Errorf("outline of an empty field (the label is inside): %s", hex(got))
	}
	if !hasInk(img, art, h.u.pal.panel) {
		t.Error("no label drawn inside the empty field")
	}
	// ...and on the outline once it has text or focus, cutting the outline without touching the text row.
	h.typeInto(&h.u.artist, "Soil")
	h.press(key.NameEscape)
	h.clickRect(h.fieldRect(&h.u.name)) // focus moves away; the artist field is filled
	img = h.render()
	cut := 0
	for dx := 14; dx < 40; dx++ { // the label is "Artist" at 12 sp, starting 12 dp in
		if !near(edge(img, art.Min.X+dx, ay), h.u.pal.outline) {
			cut++
		}
	}
	if cut < 15 {
		t.Errorf("the label doesn't interrupt the outline of a filled field (%d of 26 px differ)", cut)
	}
	// The text row is not painted over by the label's panel-colored backing: text is there.
	textRow := rect{Min: image.Pt(art.Min.X+16, art.Min.Y+20), Max: image.Pt(art.Min.X+120, art.Max.Y-14)}
	if !hasInk(img, textRow, h.u.pal.panel) {
		t.Error("the typed text isn't visible")
	}
	// Focused and empty: the placeholder (BPM's "100-140") shows, the label is on the outline.
	h.u.bpm.editor.SetText("")
	h.u.in.BPM = ""
	h.clickRect(h.fieldRect(&h.u.bpm))
	img = h.render()
	if !hasInk(img, rect{Min: image.Pt(bpm.Min.X+16, bpm.Min.Y+20), Max: image.Pt(bpm.Max.X-30, bpm.Max.Y-14)}, h.u.pal.panel) {
		t.Error("the placeholder isn't drawn in the focused empty field")
	}
}

// hasInk reports whether any pixel in r (dp) is not the background color.
func hasInk(img *image.RGBA, r rect, bg color.NRGBA) bool {
	for y := r.Min.Y * scale; y < r.Max.Y*scale; y++ {
		for x := r.Min.X * scale; x < r.Max.X*scale; x++ {
			c := img.RGBAAt(x, y)
			if !near(color.NRGBA{R: c.R, G: c.G, B: c.B}, bg) {
				return true
			}
		}
	}
	return false
}

// brightest is the largest channel value in r (dp).
func brightest(img *image.RGBA, r rect) uint8 {
	var m uint8
	for y := r.Min.Y * scale; y < r.Max.Y*scale; y++ {
		for x := r.Min.X * scale; x < r.Max.X*scale; x++ {
			c := img.RGBAAt(x, y)
			m = max(m, c.R, c.G, c.B)
		}
	}
	return m
}

// E-DSK-20 (drawn above), E-DSK-26 (highlight), E-DSK-06 (disabled button)
func TestPixelsMenuHoverAndDisabled(t *testing.T) {
	h := libHarness(t)
	row0 := h.rowRect(0)
	// Hover highlights the row with the panel color.
	probe := image.Pt(row0.Min.X+500, row0.Max.Y-8)
	h.hoverAt(500, row0.Min.Y+40)
	h.wantPixel(h.render(), "hovered row", probe.X, probe.Y, h.u.pal.panel)
	h.hoverAt(500, 880)
	h.wantPixel(h.render(), "row without hover", probe.X, probe.Y, h.u.pal.bg)

	// The open menu is drawn over the rows, not under them.
	fr := h.fieldRect(&h.u.tuning)
	h.clickRect(fr)
	if !h.open(&h.u.tuning) {
		t.Fatal("menu not open")
	}
	h.hoverAt(500, 880)
	over := image.Pt(fr.Min.X+fr.Dx()/2, 300)
	if over.Y < row0.Min.Y {
		t.Fatal("test point isn't over a row")
	}
	h.wantPixel(h.render(), "menu over the list", over.X, over.Y, h.u.pal.raised)
	h.press(key.NameEscape)
	h.hoverAt(500, 880)
	h.wantPixel(h.render(), "same point, menu closed", over.X, over.Y, h.u.pal.bg)

	// A disabled button is dimmer: the rescan button without a folder, against the folder button.
	g := newHarnessWith(t, harnessOpts{noConfig: true})
	folder := g.buttonRect(g.topBarArea(), &g.u.folderBtn)
	rescan := folder.Add(image.Pt(folder.Dx(), 0))
	img := g.render()
	if f, r := brightest(img, folder), brightest(img, rescan); !(r < f) {
		t.Errorf("disabled rescan icon (%d) isn't dimmer than the folder icon (%d)", r, f)
	}
	// With a folder it is as bright.
	root := "/tabs/Tabs"
	g = newHarnessWith(t, harnessOpts{root: &root})
	folder = g.buttonRect(g.topBarArea(), &g.u.folderBtn)
	rescan = folder.Add(image.Pt(folder.Dx(), 0))
	img = g.render()
	if f, r := brightest(img, folder), brightest(img, rescan); r != f {
		t.Errorf("enabled rescan icon (%d) differs from the folder icon (%d)", r, f)
	}
}

// E-DSK-29
func TestWindowSizes(t *testing.T) {
	for _, w := range []int{360, 600, 1000, 2000} {
		t.Run(fmt.Sprint(w, "dp"), func(t *testing.T) {
			h := newHarnessWith(t, harnessOpts{index: indexOf(testlib.Songs()), size: image.Pt(w, 900)})
			win := rect{Max: h.winDp()}
			var rs []rect
			for _, f := range []*field{&h.u.artist, &h.u.name, &h.u.tuning, &h.u.bpm} {
				r := h.fieldRect(f)
				if !r.In(win) {
					t.Errorf("%s field %v outside the window %v", f.label, r, win)
				}
				if r.Dx() < 60 || r.Dy() != 56 {
					t.Errorf("%s field is %dx%d dp", f.label, r.Dx(), r.Dy())
				}
				for i, o := range rs {
					if r.Overlaps(o) {
						t.Errorf("%s field %v overlaps %v", f.label, r, rs[i])
					}
				}
				rs = append(rs, r)
			}
			// Artist and song share a row, tuning and bpm the next one.
			if rs[0].Min.Y != rs[1].Min.Y || rs[2].Min.Y != rs[3].Min.Y || rs[2].Min.Y <= rs[0].Max.Y {
				t.Errorf("rows: %v", rs)
			}
			// The top bar buttons stay on screen, and the list still takes clicks.
			folder := h.buttonRect(h.topBarArea(), &h.u.folderBtn)
			if !folder.In(win) {
				t.Errorf("folder button %v outside the window", folder)
			}
			row := h.rowRect(0)
			if row.Empty() || !row.In(win) || row.Min.Y < rs[3].Max.Y {
				t.Errorf("row 0 %v, fields end at %d", row, rs[3].Max.Y)
			}
			// The menu of the narrowest field opens inside the window.
			h.clickField(&h.u.tuning)
			m := h.menuItemRect(&h.u.tuning, 0)
			if !m.In(win) {
				t.Errorf("menu item %v outside the window", m)
			}
		})
	}
}

// E-DSK-30
func TestMemoryAfterLoading(t *testing.T) {
	var songs []*tab.Song
	for i := range 2000 {
		songs = append(songs, &tab.Song{
			Path: fmt.Sprintf("Band %03d/Album %02d/Song %04d with a longer name.gp5", i%100, i%7, i), Format: "gp5",
			Artist: fmt.Sprintf("Band %03d", i%100), Album: fmt.Sprintf("Album %02d", i%7), Title: fmt.Sprintf("Song %04d with a longer name", i),
			ArtistSource: "file", AlbumSource: "file", TitleSource: "file",
			Tracks: []tab.Track{testlib.EStd6, testlib.DropC6, testlib.Bass4, testlib.Drums},
			Tempos: []tab.Tempo{{Bar: 1, BPM: float64(80 + i%100)}, {Bar: 17, BPM: 120}, {Bar: 33, BPM: 90}},
		})
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	h := libHarness(t, songs...)
	for i := 0; i < 30; i++ { // draw the list as a user scrolling would
		h.scrollList(200)
	}
	// The app asks to hand memory back two seconds after loading; the harness holds that timer.
	if !hasDelay(h.timers, 2*time.Second) {
		t.Errorf("no memory release timer, timers: %v", h.timers)
	}
	h.advance(3 * time.Second)
	debug.FreeOSMemory()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	t.Logf("live heap %.1f MB (before loading: %.1f MB), heap in use %.1f MB", float64(after.HeapAlloc)/1e6, float64(before.HeapAlloc)/1e6, float64(after.HeapInuse)/1e6)
	if after.HeapAlloc > 30<<20 {
		t.Errorf("live heap %.1f MB after loading 2000 songs, want under 30 MB", float64(after.HeapAlloc)/1e6)
	}
	runtime.KeepAlive(h)
}

func hasDelay(ds []time.Duration, d time.Duration) bool {
	for _, x := range ds {
		if x == d {
			return true
		}
	}
	return false
}
