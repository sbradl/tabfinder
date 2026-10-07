package main

import (
	"image"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

// U-DSK-08
func TestPaletteBadge(t *testing.T) {
	for _, p := range []struct {
		name string
		pal  palette
	}{{"dark", dark}, {"light", light}} {
		for n, want := range map[int]badgeColors{
			0: p.pal.bass, 1: p.pal.bass, 4: p.pal.bass, 5: p.pal.bass,
			6: p.pal.guitar,
			7: p.pal.extended, 8: p.pal.extended, 9: p.pal.extended, 12: p.pal.extended,
		} {
			if got := p.pal.badge(n); got != want {
				t.Errorf("%s: badge(%d) = %+v, want %+v", p.name, n, got, want)
			}
		}
		if p.pal.bass == p.pal.guitar || p.pal.guitar == p.pal.extended || p.pal.bass == p.pal.extended {
			t.Errorf("%s: two badge ranges share colors", p.name)
		}
	}
	if dark.bg == light.bg {
		t.Error("dark and light share a background")
	}
}

// U-DSK-09
func TestFlow(t *testing.T) {
	box := func(w, h int) layout.Widget {
		return func(gtx C) D { return D{Size: image.Pt(w, h)} }
	}
	run := func(maxW, gap int, ws ...layout.Widget) D {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(maxW, 1000)}}
		return flow(gtx, gap, ws)
	}
	tests := []struct {
		name string
		maxW int
		gap  int
		ws   []layout.Widget
		want image.Point
	}{
		{"none", 250, 10, nil, image.Pt(0, 0)},
		{"one", 250, 10, []layout.Widget{box(100, 20)}, image.Pt(100, 20)},
		{"fits one row, gap between", 250, 10, []layout.Widget{box(100, 20), box(100, 20)}, image.Pt(210, 20)},
		{"third wraps", 250, 10, []layout.Widget{box(100, 20), box(100, 20), box(100, 20)}, image.Pt(210, 50)},
		{"exactly the width fits", 210, 10, []layout.Widget{box(100, 20), box(100, 20)}, image.Pt(210, 20)},
		{"one pixel short wraps", 209, 10, []layout.Widget{box(100, 20), box(100, 20)}, image.Pt(100, 50)},
		{"row height is the tallest", 250, 10, []layout.Widget{box(100, 20), box(100, 35), box(100, 20)}, image.Pt(210, 35+10+20)},
		{"too wide item alone, then the next wraps", 250, 10, []layout.Widget{box(300, 20), box(50, 20)}, image.Pt(300, 50)},
		{"too wide item after others wraps first", 250, 10, []layout.Widget{box(50, 20), box(300, 20)}, image.Pt(300, 50)},
		{"gap zero", 200, 0, []layout.Widget{box(100, 10), box(100, 10), box(100, 10)}, image.Pt(200, 20)},
		{"many rows", 100, 5, []layout.Widget{box(60, 10), box(60, 10), box(60, 10), box(60, 10)}, image.Pt(60, 4*10+3*5)},
	}
	for _, tt := range tests {
		if got := run(tt.maxW, tt.gap, tt.ws...).Size; got != tt.want {
			t.Errorf("%s: size %v, want %v", tt.name, got, tt.want)
		}
	}
}

// U-DSK-10
func TestShaperFonts(t *testing.T) {
	shaper := newShaper()
	// The width of a string tells the face: each one has its own metrics, and an
	// unknown typeface falls back to the first loaded face.
	width := func(f font.Font, s string) fixed.Int26_6 {
		shaper.LayoutString(text.Parameters{Font: f, PxPerEm: fixed.I(32), MaxWidth: 1 << 20, MinWidth: 0, Truncator: "…"}, s)
		var w fixed.Int26_6
		for {
			g, ok := shaper.NextGlyph()
			if !ok {
				break
			}
			w += g.Advance
		}
		return w
	}
	const sample = "Tuning Drop C 190 BPM"
	faces := map[string]font.Font{
		"Barlow Condensed Medium":   condensedMedium,
		"Barlow Condensed SemiBold": condensed,
		"Roboto":                    body,
		"Go Mono":                   mono,
		"Go Mono Bold":              monoBold,
	}
	widths := map[string]fixed.Int26_6{}
	for name, f := range faces {
		widths[name] = width(f, sample)
		if widths[name] == 0 {
			t.Errorf("%s: shaped nothing", name)
		}
	}
	fallback := width(font.Font{Typeface: "No Such Typeface"}, sample)
	for name, w := range widths {
		if w == fallback && name != "Go Mono" && name != "Go Mono Bold" {
			t.Errorf("%s has the width of the fallback face: not found by name", name)
		}
	}
	if !(widths["Barlow Condensed Medium"] < widths["Roboto"]) {
		t.Errorf("condensed (%v) not narrower than Roboto (%v)", widths["Barlow Condensed Medium"], widths["Roboto"])
	}
	if widths["Barlow Condensed Medium"] == widths["Barlow Condensed SemiBold"] {
		t.Error("Medium and SemiBold are the same face")
	}
	// Go Mono is monospaced: every character is as wide as every other.
	if width(mono, "iiiiii") != width(mono, "WWWWWW") || width(monoBold, "iiiiii") != width(monoBold, "WWWWWW") {
		t.Error("Go Mono is not monospaced")
	}
	if width(body, "iiiiii") == width(body, "WWWWWW") {
		t.Error("Roboto is monospaced?")
	}
	// Non-Latin and accented text comes from the embedded fonts too.
	if width(body, "Äther öß") == 0 || width(condensed, "Äther öß") == 0 {
		t.Error("umlauts not shaped")
	}
}
