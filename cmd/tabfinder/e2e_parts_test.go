package main

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"tabfinder/internal/testlib"
)

// renderWidget draws w alone on the background, in a window size dp square.
func (h *harness) renderWidget(size int, w layout.Widget) *image.RGBA {
	h.t.Helper()
	px := image.Pt(size*scale, size*scale)
	win, err := headless.NewWindow(px.X, px.Y)
	if err != nil {
		h.t.Skip("no GPU for offscreen rendering:", err)
	}
	defer win.Release()
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: scale, PxPerSp: scale}, Constraints: layout.Constraints{Max: px}, Now: h.now}
	paint.Fill(gtx.Ops, h.u.pal.bg)
	w(gtx)
	if err := win.Frame(&ops); err != nil {
		h.t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: px})
	if err := win.Screenshot(img); err != nil {
		h.t.Fatal(err)
	}
	return img
}

func TestLevelColor(t *testing.T) {
	for name, p := range map[string]palette{"dark": dark, "light": light} {
		if c := p.level(1); c.G <= c.R {
			t.Errorf("%s: level 1 is %s, want green", name, hex(c))
		}
		if c := p.level(10); c.R <= c.G {
			t.Errorf("%s: level 10 is %s, want red", name, hex(c))
		}
		for l := 1; l <= 10; l++ {
			c := p.level(l)
			// WCAG's contrast for graphics.
			if r := contrast(c, p.bg); r < 3 {
				t.Errorf("%s: level %d (%s) has a contrast of %.1f to the background, want 3 or more", name, l, hex(c), r)
			}
			if l == 1 {
				continue
			}
			a := p.level(l - 1)
			if a == c || int(c.G)-int(c.R) > int(a.G)-int(a.R) {
				t.Errorf("%s: level %d (%s) isn't redder than %d (%s)", name, l, hex(c), l-1, hex(a))
			}
		}
	}
}

// E-DSK-34: the meter has five rising bars, two levels each; an odd level lights half a bar.
func TestMeter(t *testing.T) {
	h := libHarness(t)
	for _, tt := range []struct {
		level int
		bars  string // per bar: full, half, empty
	}{
		{1, "h...."}, {2, "f...."}, {5, "ffh.."}, {8, "ffff."}, {9, "ffffh"}, {10, "fffff"},
	} {
		img := h.renderWidget(40, func(gtx C) D { return h.u.meter(gtx, tt.level) })
		lit, dim := h.u.pal.level(tt.level), h.u.pal.outline
		for i, want := range tt.bars {
			b := meterBar(i)
			x := (b.Min.X + b.Max.X) / 2
			bottom, top := px(img, x, b.Max.Y-1), px(img, x, b.Min.Y+1)
			wantBottom, wantTop := dim, dim
			switch want {
			case 'f':
				wantBottom, wantTop = lit, lit
			case 'h':
				wantBottom = lit
			}
			if !near(bottom, wantBottom) || !near(top, wantTop) {
				t.Errorf("level %d, bar %d: bottom %s, top %s; want %s, %s", tt.level, i, hex(bottom), hex(top), hex(wantBottom), hex(wantTop))
			}
		}
	}
	// The bars rise, side by side.
	for i := 1; i < 5; i++ {
		a, b := meterBar(i-1), meterBar(i)
		if b.Dy() <= a.Dy() || b.Max.Y != a.Max.Y || b.Min.X <= a.Max.X {
			t.Errorf("bars %v, %v", a, b)
		}
	}
}

// hasColor reports whether any pixel in r (dp) is c.
func hasColor(img *image.RGBA, r rect, c color.NRGBA) bool {
	for y := r.Min.Y * scale; y < r.Max.Y*scale; y++ {
		for x := r.Min.X * scale; x < r.Max.X*scale; x++ {
			p := img.RGBAAt(x, y)
			if near(color.NRGBA{R: p.R, G: p.G, B: p.B, A: 0xff}, c) {
				return true
			}
		}
	}
	return false
}

// E-DSK-34: a row shows a meter per part, in the colors of their levels; one without parts none.
func TestRowParts(t *testing.T) {
	for _, w := range []int{1000, 360} {
		h := newHarnessWith(t, harnessOpts{index: indexOf(testlib.Songs()), size: image.Pt(w, 900)})
		h.shot(fmt.Sprintf("34-row-parts-%d", w))
		h.u.in.Name = "brass kettle" // drums 7, bass 5, rhythm 8, lead 9
		h.frame()
		row := h.rowRect(0)
		img := h.render()
		for _, l := range []int{5, 7, 8, 9} {
			if !hasColor(img, row, h.u.pal.level(l)) {
				t.Errorf("%d dp: no meter of level %d in the row", w, l)
			}
		}
		h.u.in.Name = "eight string"
		h.frame()
		row = h.rowRect(0)
		img = h.render()
		for l := 1; l <= 10; l++ {
			if hasColor(img, row, h.u.pal.level(l)) {
				t.Errorf("%d dp: a song without parts has a meter of level %d", w, l)
			}
		}
	}
}
