package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"

	"tabfinder/internal/difficulty"
	"tabfinder/internal/finder"
	"tabfinder/internal/rows"
)

// The difficulty of the parts: a popup with a range of levels per role, chips that sum
// the ranges up under the fields, and the order of the list.

var icTune, icSort, icCheck = icon(icons.ImageTune), icon(icons.ContentSort), icon(icons.NavigationCheck)

// levels is the state of the difficulty popup and the chips.
type levels struct {
	levelsOpen              bool
	levelsBtn               widget.Clickable
	levelsDone, levelsReset widget.Clickable
	sliders                 map[difficulty.Role]*rangeSlider
	chipClear               map[difficulty.Role]*widget.Clickable
	scrim, card, levelsKey  int // tags: the dimmed window, the popup, its keys
	cardHover               gesture.Hover
	cardHovered             bool
	sortOpen                bool
	sortBtn                 widget.Clickable
	sortItems               [3]widget.Clickable // in the order of sorts
	sortScrim, sortBlock    int
}

func newLevels() levels {
	l := levels{sliders: map[difficulty.Role]*rangeSlider{}, chipClear: map[difficulty.Role]*widget.Clickable{}}
	for _, r := range difficulty.Roles {
		l.sliders[r], l.chipClear[r] = new(rangeSlider), new(widget.Clickable)
	}
	return l
}

var sorts = []finder.Sort{finder.SortAZ, finder.SortEasiest, finder.SortHardest}

// sortName is a sort order in the menu; sortLabel on its button.
func sortName(o finder.Sort) string {
	switch o {
	case finder.SortEasiest:
		return "Easiest first"
	case finder.SortHardest:
		return "Hardest first"
	}
	return "A–Z"
}

func (u *ui) sortLabel() string {
	switch u.in.Sort {
	case finder.SortEasiest:
		return "Easiest"
	case finder.SortHardest:
		return "Hardest"
	}
	return "A–Z"
}

func roleName(r difficulty.Role) string {
	switch r {
	case difficulty.Drums:
		return "Drums"
	case difficulty.Bass:
		return "Bass"
	case difficulty.Rhythm:
		return "Rhythm guitar"
	}
	return "Lead guitar"
}

// --- range slider ---

// thumb is one end of a range slider.
type thumb int

const (
	noThumb thumb = iota
	lowThumb
	highThumb
)

// sliderPad is the room at the slider's ends, in dp, so the thumbs at 1 and 10 fit.
const sliderPad = 12

// sliderValue is the level at x along a track w wide: 1 at the start, 10 at the end.
func sliderValue(x, w float32) int {
	v := 1 + int(math.Round(float64(9*x/w)))
	return min(max(v, 1), 10)
}

// sliderX is where level v sits along a track w wide.
func sliderX(v int, w float32) float32 { return float32(v-1) * w / 9 }

// grab is the thumb a press on level v takes: the nearer one, the low one halfway; none
// when both are on v, until the drag shows which way.
func grab(lo, hi, v int) thumb {
	switch {
	case lo == hi && v == lo:
		return noThumb
	case v <= lo:
		return lowThumb
	case v >= hi:
		return highThumb
	case v-lo <= hi-v:
		return lowThumb
	}
	return highThumb
}

// moveThumb moves thumb t to v, not past the other one. Without one it takes the one on
// the side of v.
func moveThumb(lo, hi int, t thumb, v int) (int, int, thumb) {
	if t == noThumb {
		switch {
		case v < lo:
			t = lowThumb
		case v > hi:
			t = highThumb
		default:
			return lo, hi, t
		}
	}
	if t == lowThumb {
		lo = min(v, hi)
	} else {
		hi = max(v, lo)
	}
	return lo, hi, t
}

type rangeSlider struct {
	drag    gesture.Drag
	hover   gesture.Hover
	hovered bool
	held    thumb
	width   int // px, as last drawn
}

// --- input ---

// updateLevels applies the clicks and drags on the popup, the chips and the sort menu.
func (u *ui) updateLevels(gtx C) {
	if u.levelsBtn.Clicked(gtx) {
		u.levelsOpen = true
	}
	for _, r := range difficulty.Roles {
		if u.chipClear[r].Clicked(gtx) {
			u.clearLevel(r)
		}
	}
	if u.levelsDone.Clicked(gtx) {
		u.levelsOpen = false
	}
	if u.levelsReset.Clicked(gtx) {
		for _, r := range difficulty.Roles {
			u.clearLevel(r)
		}
	}
	u.cardHovered = u.cardHover.Update(gtx.Source)
	// The scrim takes the presses beside the popup; the popup those on it, off its controls.
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &u.scrim, Kinds: pointer.Press}, key.FocusFilter{Target: &u.levelsKey}, key.Filter{Focus: &u.levelsKey, Name: key.NameEscape})
		if !ok {
			break
		}
		switch ev := ev.(type) {
		case pointer.Event:
			u.levelsOpen = false
		case key.Event:
			if ev.State == key.Press {
				u.levelsOpen = false
			}
		}
	}
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &u.card, Kinds: pointer.Press}); !ok {
			break
		}
	}
	pad := float32(gtx.Dp(sliderPad))
	for _, r := range difficulty.Roles {
		s := u.sliders[r]
		s.hovered = s.hover.Update(gtx.Source)
		for {
			ev, ok := s.drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
			if !ok {
				break
			}
			v := sliderValue(ev.Position.X-pad, float32(s.width)-2*pad)
			lo, hi := u.level(r)
			switch ev.Kind {
			case pointer.Press:
				s.held = grab(lo, hi, v)
				fallthrough
			case pointer.Drag:
				lo, hi, s.held = moveThumb(lo, hi, s.held, v)
				u.setLevel(r, lo, hi)
			default:
				s.held = noThumb
			}
		}
	}

	if u.sortBtn.Clicked(gtx) {
		u.sortOpen = !u.sortOpen
	}
	for i := range u.sortItems {
		if u.sortItems[i].Clicked(gtx) {
			u.setSort(sorts[i])
			u.sortOpen = false
		}
	}
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &u.sortScrim, Kinds: pointer.Press}); !ok {
			break
		}
		u.sortOpen = false
	}
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &u.sortBlock, Kinds: pointer.Press}); !ok {
			break
		}
	}
}

// --- the line under the fields ---

// levelsLine is the "Difficulty…" button and a chip per role with a range of levels.
func (u *ui) levelsLine(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return u.outlinedButton(gtx, &u.levelsBtn, icTune, "Difficulty…")
		}),
		layout.Flexed(1, func(gtx C) D {
			chips := u.chips()
			if len(chips) == 0 {
				return D{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}
			return layout.Inset{Left: 8}.Layout(gtx, func(gtx C) D {
				ws := make([]layout.Widget, len(chips))
				for i, c := range chips {
					ws[i] = func(gtx C) D { return u.chip(gtx, c) }
				}
				return flow(gtx, gtx.Dp(6), ws)
			})
		}),
	)
}

// chip is a role's icon, its range and a ✕ that drops it.
func (u *ui) chip(gtx C, c chip) D {
	h := gtx.Dp(32)
	rec := op.Record(gtx.Ops)
	dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 10, Right: 6}.Layout(gtx, func(gtx C) D { return u.roleIcon(gtx, c.role, 18, u.pal.fg) })
		}),
		layout.Rigid(func(gtx C) D { return u.text(gtx, labelLarge, u.pal.fg, c.text) }),
		layout.Rigid(func(gtx C) D {
			return u.chipClear[c.role].Layout(gtx, func(gtx C) D {
				pointer.CursorPointer.Add(gtx.Ops)
				gtx.Constraints = layout.Exact(image.Pt(h, h))
				return layout.Center.Layout(gtx, func(gtx C) D {
					gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(16), gtx.Dp(16)))
					return icClose.Layout(gtx, u.pal.fgMuted)
				})
			})
		}),
	)
	call := rec.Stop()
	shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, h/2)
	paint.FillShape(gtx.Ops, u.pal.raisedHigh, shape.Op(gtx.Ops))
	call.Add(gtx.Ops)
	paint.FillShape(gtx.Ops, u.pal.outline, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	return dims
}

// outlinedButton is an icon and a label in a rounded outline, 36 dp high.
func (u *ui) outlinedButton(gtx C, b *widget.Clickable, ic *widget.Icon, label string) D {
	return b.Layout(gtx, func(gtx C) D {
		pointer.CursorPointer.Add(gtx.Ops)
		gtx.Constraints.Min = image.Pt(0, gtx.Dp(36))
		rec := op.Record(gtx.Ops)
		dims := layout.Inset{Left: 12, Right: 16}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(18), gtx.Dp(18)))
					return ic.Layout(gtx, u.pal.accent)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx C) D { gtx.Constraints.Min.Y = 0; return u.text(gtx, labelLarge, u.pal.fg, label) }),
			)
		})
		call := rec.Stop()
		shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, dims.Size.Y/2)
		if b.Hovered() {
			paint.FillShape(gtx.Ops, u.pal.raisedHigh, shape.Op(gtx.Ops))
		}
		call.Add(gtx.Ops)
		paint.FillShape(gtx.Ops, u.pal.outline, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
		return dims
	})
}

// --- the popup ---

// levelsPopup dims the window and shows a slider per role over it.
func (u *ui) levelsPopup(gtx C) {
	if !u.levelsOpen {
		return
	}
	win := gtx.Constraints.Max
	area := clip.Rect{Max: win}.Push(gtx.Ops)
	paint.Fill(gtx.Ops, color.NRGBA{A: 0x99})
	event.Op(gtx.Ops, &u.scrim)
	area.Pop()

	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(win.X-gtx.Dp(32), gtx.Dp(520))
	rec := op.Record(gtx.Ops)
	dims := layout.UniformInset(24).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return u.text(gtx, titleLarge, u.pal.fg, "Difficulty") }),
			layout.Rigid(func(gtx C) D { return u.text(gtx, bodyMedium, u.pal.fgMuted, "Levels 1 to 10, per part") }),
			layout.Rigid(layout.Spacer{Height: 16}.Layout),
		}
		for _, r := range difficulty.Roles {
			children = append(children, layout.Rigid(func(gtx C) D { return u.levelRow(gtx, r) }))
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 16}.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return u.textButton(gtx, &u.levelsReset, "Reset", false) }),
					layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
					layout.Rigid(func(gtx C) D { return u.textButton(gtx, &u.levelsDone, "Done", true) }),
				)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := rec.Stop()

	defer op.Offset(image.Pt((win.X-dims.Size.X)/2, max((win.Y-dims.Size.Y)/2, 0))).Push(gtx.Ops).Pop()
	shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(16))
	paint.FillShape(gtx.Ops, u.pal.panel, shape.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, u.pal.outline, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	// The card takes the clicks beside its controls, so they don't reach the scrim, and the keys.
	cardArea := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &u.card)
	event.Op(gtx.Ops, &u.levelsKey)
	u.cardHover.Add(gtx.Ops)
	if !gtx.Focused(&u.levelsKey) {
		gtx.Execute(key.FocusCmd{Tag: &u.levelsKey})
	}
	call.Add(gtx.Ops)
	cardArea.Pop()
}

// levelRow is a role's icon and name, its slider and its range.
func (u *ui) levelRow(gtx C, r difficulty.Role) D {
	lo, hi := u.level(r)
	return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.roleIcon(gtx, r, 24, u.pal.fg) }),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(120), gtx.Dp(120)
				return layout.Inset{Left: 12}.Layout(gtx, func(gtx C) D { return u.text(gtx, labelLarge, u.pal.fg, roleName(r)) })
			}),
			layout.Flexed(1, func(gtx C) D { return u.slider(gtx, u.sliders[r], lo, hi) }),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Dp(56)
				c := u.pal.fgMuted
				if lo > 1 || hi < 10 {
					c = u.pal.accent
				}
				return layout.E.Layout(gtx, func(gtx C) D { return u.text(gtx, style{monoBold, 15}, c, rows.LevelChip(lo, hi)) })
			}),
		)
	})
}

// slider draws a track with the range lo to hi on it, and takes drags and clicks.
func (u *ui) slider(gtx C, s *rangeSlider, lo, hi int) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(40))
	s.width = size.X
	pad := float32(gtx.Dp(sliderPad))
	w := float32(size.X) - 2*pad
	x := func(v int) float32 { return pad + sliderX(v, w) }
	cy := float32(size.Y) / 2
	th := float32(gtx.Dp(4))
	track := func(from, to float32, c color.NRGBA) {
		r := image.Rect(int(from), int(cy-th/2), int(to), int(cy+th/2))
		paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, int(th/2)).Op(gtx.Ops))
	}
	track(pad, pad+w, u.pal.outline)
	for v := 1; v <= 10; v++ { // a tick per level
		r := float32(gtx.Dp(2))
		paint.FillShape(gtx.Ops, u.pal.fgMuted, clip.Ellipse{Min: f32.Pt(x(v)-r, cy-r).Round(), Max: f32.Pt(x(v)+r, cy+r).Round()}.Op(gtx.Ops))
	}
	track(x(lo), x(hi), u.pal.accent)
	for _, v := range []int{lo, hi} {
		r := float32(gtx.Dp(9))
		paint.FillShape(gtx.Ops, u.pal.accent, clip.Ellipse{Min: f32.Pt(x(v)-r, cy-r).Round(), Max: f32.Pt(x(v)+r, cy+r).Round()}.Op(gtx.Ops))
	}
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	s.drag.Add(gtx.Ops)
	s.hover.Add(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	area.Pop()
	return D{Size: size}
}

// textButton is a flat button, or a filled one for the main action.
func (u *ui) textButton(gtx C, b *widget.Clickable, label string, filled bool) D {
	if filled {
		bt := material.Button(u.th, b, label)
		bt.Font, bt.TextSize = labelLarge.font, labelLarge.size
		bt.Background, bt.Color, bt.CornerRadius = u.pal.accent, u.pal.onAccent, 20
		bt.Inset = layout.Inset{Left: 24, Right: 24, Top: 10, Bottom: 10}
		return bt.Layout(gtx)
	}
	return b.Layout(gtx, func(gtx C) D {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Inset{Left: 16, Right: 16, Top: 10, Bottom: 10}.Layout(gtx, func(gtx C) D {
			return u.text(gtx, labelLarge, u.pal.accent, label)
		})
	})
}

// --- sort menu ---

// sortButton is the sort order's button in the top bar, and its menu while open.
func (u *ui) sortButton(gtx C) D {
	dims := u.outlinedButton(gtx, &u.sortBtn, icSort, u.sortLabel())
	if !u.sortOpen {
		return dims
	}
	rec := op.Record(gtx.Ops)
	// An invisible scrim over everything: a click beside the menu closes it.
	big := gtx.Dp(10000)
	area := clip.Rect{Min: image.Pt(-big, -big), Max: image.Pt(big, big)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &u.sortScrim)
	area.Pop()
	width, itemH := gtx.Dp(200), gtx.Dp(44)
	size := image.Pt(width, itemH*len(sorts)+gtx.Dp(8))
	op.Offset(image.Pt(dims.Size.X-width, dims.Size.Y+gtx.Dp(4))).Add(gtx.Ops)
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(10))
	paint.FillShape(gtx.Ops, u.pal.raised, shape.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, u.pal.outline, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	block := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &u.sortBlock)
	block.Pop()
	op.Offset(image.Pt(0, gtx.Dp(4))).Add(gtx.Ops)
	for i, o := range sorts {
		it := &u.sortItems[i]
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(width, itemH))
		it.Layout(mgtx, func(gtx C) D {
			if it.Hovered() {
				paint.FillShape(gtx.Ops, u.pal.raisedHigh, clip.Rect{Max: gtx.Constraints.Max}.Op())
			}
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Inset{Left: 16, Right: 16}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D { gtx.Constraints.Min.Y = 0; return u.text(gtx, labelLarge, u.pal.fg, sortName(o)) }),
					layout.Rigid(func(gtx C) D {
						if u.in.Sort != o {
							return D{}
						}
						return u.icon(gtx, icCheck, u.pal.accent)
					}),
				)
			})
		})
		op.Offset(image.Pt(0, itemH)).Add(gtx.Ops)
	}
	op.Defer(gtx.Ops, rec.Stop())
	return dims
}

// --- role icons ---

// roleIcon draws a role's instrument, size dp square: a drum with sticks, a bass clef,
// a guitar, and a guitar with a spark for the lead.
func (u *ui) roleIcon(gtx C, r difficulty.Role, size unit.Dp, c color.NRGBA) D {
	s := float32(gtx.Dp(size))
	f := s / 24 // drawn on a 24 grid
	pt := func(x, y float32) f32.Point { return f32.Pt(x*f, y*f) }
	stroke := func(w float32, build func(p *clip.Path)) {
		var p clip.Path
		p.Begin(gtx.Ops)
		build(&p)
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: w * f}.Op())
	}
	fill := func(build func(p *clip.Path)) {
		var p clip.Path
		p.Begin(gtx.Ops)
		build(&p)
		p.Close()
		paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	}
	dot := func(x, y, r float32) {
		paint.FillShape(gtx.Ops, c, clip.Ellipse{Min: pt(x-r, y-r).Round(), Max: pt(x+r, y+r).Round()}.Op(gtx.Ops))
	}
	guitar := func() {
		dot(8, 17, 5.5)
		dot(12, 12.5, 4)
		stroke(2.4, func(p *clip.Path) { p.MoveTo(pt(11, 13)); p.LineTo(pt(19, 5)) })
		fill(func(p *clip.Path) {
			p.MoveTo(pt(18, 3.5))
			p.LineTo(pt(21, 1))
			p.LineTo(pt(23, 3))
			p.LineTo(pt(20.5, 6))
		})
	}
	switch r {
	case difficulty.Drums:
		fill(func(p *clip.Path) { // the shell
			p.MoveTo(pt(3, 12))
			p.LineTo(pt(21, 12))
			p.LineTo(pt(21, 19))
			p.CubeTo(pt(21, 23), pt(3, 23), pt(3, 19))
		})
		stroke(1.6, func(p *clip.Path) { // the head
			p.MoveTo(pt(3, 11))
			p.CubeTo(pt(3, 7), pt(21, 7), pt(21, 11))
			p.CubeTo(pt(21, 14), pt(3, 14), pt(3, 11))
		})
		stroke(1.8, func(p *clip.Path) { p.MoveTo(pt(4, 2)); p.LineTo(pt(11, 9)) })
		stroke(1.8, func(p *clip.Path) { p.MoveTo(pt(20, 2)); p.LineTo(pt(13, 9)) })
	case difficulty.Bass:
		dot(6, 9, 2.6)
		stroke(2.4, func(p *clip.Path) {
			p.MoveTo(pt(4.5, 8))
			p.CubeTo(pt(5, 2), pt(17, 2), pt(17, 9))
			p.CubeTo(pt(17, 15), pt(11, 19), pt(4, 21))
		})
		dot(21, 6.5, 1.6)
		dot(21, 12.5, 1.6)
	case difficulty.Rhythm:
		guitar()
	default:
		guitar()
		stroke(1.6, func(p *clip.Path) { // the spark
			p.MoveTo(pt(2, 5))
			p.LineTo(pt(8, 5))
			p.MoveTo(pt(5, 2))
			p.LineTo(pt(5, 8))
		})
	}
	return D{Size: image.Pt(int(s), int(s))}
}
