package main

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

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

	"tab-sync/internal/finder"
	"tab-sync/internal/tab"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var icClose, icSearch, icFolder, icRefresh = icon(icons.NavigationClose), icon(icons.ActionSearch), icon(icons.FileFolderOpen), icon(icons.NavigationRefresh)

func icon(data []byte) *widget.Icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return ic
}

// The clock and timer the UI uses, replaced in tests so they needn't sleep.
var (
	now       = time.Now
	afterFunc = time.AfterFunc
)

type ui struct {
	redraw func() // asks for a frame, from any goroutine
	pal    palette
	th     *material.Theme
	posts  chan func() // state changes from background work, run on the UI goroutine

	root     string
	lib      *finder.Library
	loaded   bool // the cached index has been read
	scanning bool
	message  string
	msgUntil time.Time

	in                        finder.Query
	artist, name, tuning, bpm field
	list                      widget.List
	rows                      []gesture.Click
	folderBtn, rescanBtn, cta widget.Clickable

	// The search for in over lib, redone when either changes.
	result    finder.Result
	resultFor *finder.Library
	resultIn  finder.Query
}

func newUI(redraw func()) *ui {
	u := &ui{redraw: redraw, pal: dark, posts: make(chan func(), 16), root: loadRoot(), lib: finder.New(nil)}
	if d, ok := prefersDark(); ok && !d {
		u.pal = light
	}
	u.th = material.NewTheme()
	u.th.Shaper = newShaper()
	u.th.Palette = material.Palette{Bg: u.pal.bg, Fg: u.pal.fg, ContrastBg: u.pal.accent, ContrastFg: u.pal.onAccent}
	u.list.Axis = layout.Vertical
	u.artist.label, u.name.label, u.tuning.label, u.bpm.label = "Artist", "Song", "Tuning", "BPM"
	for _, f := range []*field{&u.artist, &u.name, &u.tuning, &u.bpm} {
		f.editor.SingleLine = true
		f.editor.Submit = true
	}
	u.bpm.placeholder = "100-140"
	go func() {
		songs, err := finder.LoadIndex(indexFile)
		u.post(func() {
			u.loaded = true
			if err != nil {
				u.show("Couldn't read the saved scan: " + err.Error())
			}
			u.setSongs(songs)
			if len(songs) == 0 && u.root != "" {
				u.rescan()
			}
		})
	}()
	return u
}

func (u *ui) post(f func()) {
	u.posts <- f
	u.redraw()
}

func (u *ui) drain() {
	for {
		select {
		case f := <-u.posts:
			f()
		default:
			return
		}
	}
}

func (u *ui) setSongs(songs []*tab.Song) {
	u.lib = finder.New(songs)
	// Decoding the index and drawing the new list for the first time (shaping text, caching
	// glyphs) leave garbage twice the size of what's live; Go would keep it as headroom.
	// Hand it back to the system once that first drawing is done.
	afterFunc(2*time.Second, debug.FreeOSMemory)
}

func (u *ui) show(msg string) {
	u.message, u.msgUntil = msg, now().Add(4*time.Second)
}

func (u *ui) rescan() {
	if u.scanning || u.root == "" {
		return
	}
	u.scanning = true
	root := u.root
	go func() {
		start := time.Now()
		songs, err := finder.ScanIndex(root, indexFile)
		u.post(func() {
			u.scanning = false
			if songs == nil && err != nil {
				u.show("Scan failed: " + err.Error())
				return
			}
			u.setSongs(songs)
			unreadable := 0
			for _, s := range songs {
				if s.Error != "" {
					unreadable++
				}
			}
			u.show(fmt.Sprintf("%d tabs, %d unreadable, in %.1f s", len(songs), unreadable, time.Since(start).Seconds()))
		})
	}()
}

func (u *ui) chooseFolder() {
	start := u.root
	go func() {
		dir, err := pickFolder(start)
		u.post(func() {
			switch {
			case err != nil:
				u.show(err.Error())
			case dir != "":
				u.root = dir
				if err := saveRoot(dir); err != nil {
					u.show("Couldn't save the folder: " + err.Error())
				}
				dropIndex()
				u.setSongs(nil)
				u.rescan()
			}
		})
	}()
}

func (u *ui) search() {
	if u.resultFor != u.lib || u.resultIn != u.in {
		u.result, u.resultFor, u.resultIn = u.lib.Search(u.in), u.lib, u.in
	}
}

func (u *ui) layout(gtx C) D {
	u.handleFields(gtx)
	if u.folderBtn.Clicked(gtx) {
		u.chooseFolder()
	}
	if u.rescanBtn.Clicked(gtx) {
		u.rescan()
	}
	u.search()

	paint.Fill(gtx.Ops, u.pal.bg)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.topBar),
		layout.Flexed(1, u.content),
	)
	u.snackbar(gtx)
	return D{Size: gtx.Constraints.Max}
}

func (u *ui) content(gtx C) D {
	if u.root == "" {
		return u.prompt(gtx, "Choose your tab folder", "Pick the folder that holds your Guitar Pro, TuxGuitar and Power Tab files.", "Choose folder", u.chooseFolder)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.filters),
		layout.Rigid(u.progress),
		layout.Flexed(1, func(gtx C) D {
			switch {
			case !u.loaded:
				return D{Size: gtx.Constraints.Max} // reading the saved scan, a moment at most
			case len(u.result.Matches) == 0 && u.scanning:
				return D{Size: gtx.Constraints.Max}
			case len(u.result.Matches) == 0 && u.in.Active():
				return u.prompt(gtx, "No tabs match", "Try fewer filters.", "Clear filters", u.clearFilters)
			case len(u.result.Matches) == 0:
				return u.prompt(gtx, "No tabs found", "Rescan, or choose another folder.", "Rescan", u.rescan)
			}
			return u.songList(gtx)
		}),
	)
}

func (u *ui) clearFilters() {
	for _, f := range []*field{&u.artist, &u.name, &u.tuning, &u.bpm} {
		f.editor.SetText("")
	}
	u.in = finder.Query{}
}

// --- top bar and filters ---

func (u *ui) topBar(gtx C) D {
	return u.panel(gtx, layout.Inset{Left: 16, Right: 8, Top: 10, Bottom: 6}, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return u.text(gtx, titleLarge, u.pal.fg, "TabFinder") }),
					layout.Rigid(func(gtx C) D {
						if u.root == "" {
							return D{}
						}
						return u.text(gtx, labelMedium, u.pal.fgMuted, u.subtitle())
					}),
				)
			}),
			layout.Rigid(func(gtx C) D { return u.iconButton(gtx, &u.folderBtn, icFolder, "Choose tab folder", true) }),
			layout.Rigid(func(gtx C) D {
				return u.iconButton(gtx, &u.rescanBtn, icRefresh, "Rescan", u.root != "" && !u.scanning)
			}),
		)
	})
}

// subtitle is the line under the title: how many tabs, in which folder.
func (u *ui) subtitle() string {
	return strings.ToUpper(fmt.Sprintf("%d tabs in %s", len(u.lib.Entries), filepath.Base(filepath.Clean(u.root))))
}

// counter is "matches / total", padded to the total's width so it never changes size.
func (u *ui) counter() string {
	total := fmt.Sprint(len(u.lib.Entries))
	return fmt.Sprintf("%*d / %s", len(total), len(u.result.Matches), total)
}

func (u *ui) filters(gtx C) D {
	return u.panel(gtx, layout.Inset{Left: 16, Right: 16, Top: 12, Bottom: 12}, func(gtx C) D {
		gap := layout.Spacer{Width: 8}.Layout
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return u.field(gtx, &u.artist, nil, false, u.artistMenu())
					}),
					layout.Rigid(gap),
					layout.Flexed(1.4, func(gtx C) D {
						return u.field(gtx, &u.name, func(gtx C) D { return u.icon(gtx, icSearch, u.pal.fgMuted) }, false, nil)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						var badge layout.Widget
						if u.in.Strings != 0 {
							badge = func(gtx C) D { return u.badge(gtx, u.in.Strings) }
						}
						return u.field(gtx, &u.tuning, badge, false, u.tuningMenu())
					}),
					layout.Rigid(gap),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(150), gtx.Dp(150)
						return u.field(gtx, &u.bpm, nil, u.result.BPMInvalid, nil)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: 16, Right: 8}.Layout(gtx, func(gtx C) D {
							return u.text(gtx, style{monoBold, 16}, u.pal.accent, u.counter())
						})
					}),
				)
			}),
		)
	})
}

func (u *ui) artistMenu() *menu {
	var items []menuItem
	for _, a := range u.lib.SuggestArtists(u.in.Artist) {
		items = append(items, menuItem{text: a})
	}
	return &menu{items: items, pick: func(it menuItem) {
		u.artist.pick(it.text)
		u.in.Artist = it.text
	}}
}

func (u *ui) tuningMenu() *menu {
	var items []menuItem
	for _, t := range finder.SuggestTunings(u.result.Tunings, u.in.Tuning) {
		detail := t.Notes
		if t.Label() == t.Notes {
			detail = "" // a custom tuning, labelled by its notes already
		}
		items = append(items, menuItem{text: t.Label(), detail: detail, section: fmt.Sprintf("%d STRINGS", t.Strings), value: t})
	}
	return &menu{items: items, pick: func(it menuItem) {
		t := it.value.(finder.Tuning)
		u.tuning.pick(t.Label())
		u.in.Tuning, u.in.Strings = t.Label(), t.Strings
	}}
}

// handleFields applies typing, clearing, Enter and Escape before layout.
func (u *ui) handleFields(gtx C) {
	for _, f := range []*field{&u.artist, &u.name, &u.tuning, &u.bpm} {
		// Before the editor, which would take the down arrow to move the caret.
		for {
			ev, ok := gtx.Event(key.Filter{Focus: &f.editor, Name: key.NameEscape}, key.Filter{Focus: &f.editor, Name: key.NameDownArrow})
			if !ok {
				break
			}
			if ev, ok := ev.(key.Event); ok && ev.State == key.Press {
				f.dismissed = ev.Name == key.NameEscape
			}
		}
		changed := false
		for {
			ev, ok := f.editor.Update(gtx)
			if !ok {
				break
			}
			switch ev.(type) {
			case widget.ChangeEvent:
				picked := f.picked
				f.picked = ""
				if picked != "" && f.editor.Text() == picked {
					break
				}
				changed = true
				f.dismissed = false
			case widget.SubmitEvent:
				f.submitted = true
			}
		}
		if f.clear.Clicked(gtx) {
			f.editor.SetText("")
			changed = true
			gtx.Execute(key.FocusCmd{Tag: &f.editor})
		}
		for {
			ev, ok := f.reopen.Update(gtx.Source)
			if !ok {
				break
			}
			if ev.Kind == gesture.KindPress {
				f.dismissed = false
			}
		}
		if !changed {
			continue
		}
		t := f.editor.Text()
		switch f {
		case &u.artist:
			u.in.Artist = t
		case &u.name:
			u.in.Name = t
		case &u.tuning:
			u.in.Tuning, u.in.Strings = t, 0 // typing drops the string count a pick set
		case &u.bpm:
			u.in.BPM = t
		}
	}
}

// --- text field with suggestions ---

type field struct {
	label, placeholder string
	editor             widget.Editor
	picked             string // text set by picking a suggestion; its change event isn't typing
	clear              widget.Clickable
	submitted          bool
	dismissed          bool          // suggestions closed with Escape or a pick, until reopened
	reopen             gesture.Click // a click anywhere on the field, which reopens them
	wasFocused         bool

	menuList  widget.List
	menuItems []gesture.Click
	menuBlock int // tag for the menu's background, so clicks don't reach rows below
}

// pick puts a picked suggestion into the field, caret at the end.
func (f *field) pick(text string) {
	f.picked = text
	f.editor.SetText(text)
	n := f.editor.Len()
	f.editor.SetCaret(n, n)
}

type menuItem struct {
	text, detail, section string
	value                 any
}

type menu struct {
	items []menuItem
	pick  func(menuItem)
}

// field draws an outlined text field like Material's, with a floating label,
// a clear button and, if m is set, a suggestion menu while focused.
func (u *ui) field(gtx C, f *field, leading layout.Widget, isErr bool, m *menu) D {
	focused := gtx.Focused(&f.editor)
	if focused && !f.wasFocused {
		f.dismissed = false
	}
	f.wasFocused = focused
	if m != nil && f.submitted && len(m.items) > 0 && !f.dismissed {
		m.pick(m.items[0])
		f.dismissed = true
	}
	f.submitted = false

	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(56))
	gtx.Constraints = layout.Exact(size)
	empty := f.editor.Len() == 0

	// Outline.
	bc, bw := u.pal.outline, gtx.Dp(1)
	switch {
	case isErr:
		bc, bw = u.pal.err, gtx.Dp(2)
	case focused:
		bc, bw = u.pal.accent, gtx.Dp(2)
	}
	rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(10))
	paint.FillShape(gtx.Ops, bc, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(bw)}.Op())

	// Content.
	layout.Inset{Left: 16, Right: 4}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if leading == nil {
					return D{}
				}
				return layout.Inset{Right: 12}.Layout(gtx, leading)
			}),
			layout.Flexed(1, func(gtx C) D {
				// Full width for clicks, natural height so the flex centers the text.
				gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
				ed := material.Editor(u.th, &f.editor, "")
				ed.Font, ed.TextSize, ed.Color = bodyLarge.font, bodyLarge.size, u.pal.fg
				ed.HintColor, ed.SelectionColor = u.pal.fgMuted, withAlpha(u.pal.accent, 0x60)
				if empty && !focused {
					ed.Hint = f.label
				} else if empty {
					ed.Hint = f.placeholder
				}
				return ed.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if empty {
					return layout.Spacer{Width: 12}.Layout(gtx)
				}
				return f.clear.Layout(gtx, func(gtx C) D {
					pointer.CursorPointer.Add(gtx.Ops)
					return layout.UniformInset(10).Layout(gtx, func(gtx C) D { return u.icon(gtx, icClose, u.pal.fgMuted) })
				})
			}),
		)
	})

	// Clicks anywhere on the field reopen its suggestions, and still reach the editor
	// (caret) and the clear button below.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	f.reopen.Add(gtx.Ops)
	pass.Pop()
	area.Pop()

	// Floating label, cut into the outline.
	if !empty || focused {
		lc := u.pal.fgMuted
		if focused {
			lc = u.pal.accent
		}
		m := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Point{} // measure the label, not the field
		dims := u.text(gtx, style{body, 12}, lc, f.label)
		call := m.Stop()
		pad := gtx.Dp(4)
		off := op.Offset(image.Pt(gtx.Dp(12), -dims.Size.Y/2)).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, u.pal.panel, clip.Rect{Max: image.Pt(dims.Size.X+2*pad, dims.Size.Y)}.Op())
		op.Offset(image.Pt(pad, 0)).Add(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
	}

	if m != nil && focused && !f.dismissed && len(m.items) > 0 {
		rec := op.Record(gtx.Ops)
		op.Offset(image.Pt(0, size.Y+gtx.Dp(4))).Add(gtx.Ops)
		u.menu(gtx, f, size.X, m)
		op.Defer(gtx.Ops, rec.Stop()) // drawn on top of everything
	}
	return D{Size: size}
}

// menu draws the suggestions; sections get a header wherever they change.
func (u *ui) menu(gtx C, f *field, width int, m *menu) {
	type line struct {
		header string
		item   int
	}
	var lines []line
	last := ""
	for i, it := range m.items {
		if it.section != "" && it.section != last {
			lines = append(lines, line{header: it.section, item: -1})
			last = it.section
		}
		lines = append(lines, line{item: i})
	}
	itemH, headH := gtx.Dp(44), gtx.Dp(36)
	h := 0
	for _, l := range lines {
		if l.item < 0 {
			h += headH
		} else {
			h += itemH
		}
	}
	h = min(h+gtx.Dp(8), gtx.Dp(440))
	size := image.Pt(width, h)
	for len(f.menuItems) < len(m.items) {
		f.menuItems = append(f.menuItems, gesture.Click{})
	}
	for i := range m.items {
		for {
			ev, ok := f.menuItems[i].Update(gtx.Source)
			if !ok {
				break
			}
			// On press: the editor may lose focus before a release arrives.
			if ev.Kind == gesture.KindPress {
				m.pick(m.items[i])
				f.dismissed = true
			}
		}
	}

	r := gtx.Dp(10)
	// Shadow, surface, outline.
	for i, a := range []uint8{0x30, 0x20, 0x10} {
		d := gtx.Dp(unit.Dp(2 + 3*i))
		paint.FillShape(gtx.Ops, color.NRGBA{A: a}, clip.UniformRRect(image.Rect(-d/2, d/2, size.X+d/2, size.Y+d), r+d).Op(gtx.Ops))
	}
	shape := clip.UniformRRect(image.Rectangle{Max: size}, r)
	paint.FillShape(gtx.Ops, u.pal.raised, shape.Op(gtx.Ops))
	defer shape.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &f.menuBlock)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &f.menuBlock, Kinds: pointer.Press | pointer.Release}); !ok {
			break
		}
	}

	gtx.Constraints = layout.Exact(size)
	f.menuList.Axis = layout.Vertical
	layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
		return f.menuList.Layout(gtx, len(lines), func(gtx C, i int) D {
			l := lines[i]
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if l.item < 0 {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if i == 0 {
							return D{}
						}
						return u.divider(gtx, u.pal.outline)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: 16, Right: 16, Top: 10, Bottom: 6}.Layout(gtx, func(gtx C) D {
							return u.text(gtx, labelMedium, u.pal.accent, l.header)
						})
					}),
				)
			}
			it, click := m.items[l.item], &f.menuItems[l.item]
			sz := image.Pt(gtx.Constraints.Max.X, itemH)
			if click.Hovered() {
				paint.FillShape(gtx.Ops, u.pal.raisedHigh, clip.Rect{Max: sz}.Op())
			}
			area := clip.Rect{Max: sz}.Push(gtx.Ops)
			click.Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			area.Pop()
			gtx.Constraints = layout.Exact(sz)
			return layout.Inset{Left: 16, Right: 16}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D { return u.text(gtx, labelLarge, u.pal.fg, it.text) }),
					layout.Rigid(func(gtx C) D {
						if it.detail == "" {
							return D{}
						}
						return u.text(gtx, labelSmall, u.pal.fgMuted, it.detail)
					}),
				)
			})
		})
	})
	paint.FillShape(gtx.Ops, u.pal.outline, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
}

// --- song list ---

func (u *ui) songList(gtx C) D {
	for len(u.rows) < len(u.result.Matches) {
		u.rows = append(u.rows, gesture.Click{})
	}
	ls := material.List(u.th, &u.list)
	ls.Indicator.Color, ls.Indicator.HoverColor = u.pal.outline, u.pal.fgMuted
	return ls.Layout(gtx, len(u.result.Matches), func(gtx C, i int) D {
		return u.songRow(gtx, &u.rows[i], u.lib.Entries[u.result.Matches[i]])
	})
}

func (u *ui) songRow(gtx C, click *gesture.Click, e finder.Entry) D {
	for {
		ev, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if ev.Kind == gesture.KindClick {
			if err := openInTuxGuitar(u.root, e.Song); err != nil {
				u.show(err.Error())
			}
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	rec := op.Record(gtx.Ops)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return u.divider(gtx, u.pal.outline) }),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 16, Right: 16, Top: 18, Bottom: 18}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D { return u.text(gtx, titleMedium, u.pal.fg, e.Song.Title) }),
									layout.Rigid(func(gtx C) D {
										return u.text(gtx, bodyMedium, u.pal.fgMuted, rowSubtitle(e.Song))
									}),
								)
							}),
							layout.Rigid(func(gtx C) D { return u.tempo(gtx, e.BPMs) }),
						)
					}),
					layout.Rigid(func(gtx C) D {
						switch {
						case len(e.Tunings) > 0:
							return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D {
								tags := make([]layout.Widget, len(e.Tunings))
								for i, t := range e.Tunings {
									tags[i] = func(gtx C) D { return u.tuningTag(gtx, t) }
								}
								return flow(gtx, gtx.Dp(6), tags)
							})
						case unreadable(e):
							return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D {
								return u.text(gtx, labelSmall, u.pal.err, "Couldn't read this file")
							})
						}
						return D{}
					}),
				)
			})
		}),
	)
	call := rec.Stop()
	if click.Hovered() {
		paint.FillShape(gtx.Ops, u.pal.panel, clip.Rect{Max: dims.Size}.Op())
	}
	call.Add(gtx.Ops)
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	click.Add(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	area.Pop()
	return dims
}

// rowSubtitle is "artist · album", leaving out what's missing.
func rowSubtitle(s *tab.Song) string {
	var parts []string
	for _, p := range []string{s.Artist, s.Album} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

// unreadable: the file couldn't be parsed and there's nothing else to show of it.
func unreadable(e finder.Entry) bool { return len(e.Tunings) == 0 && e.Song.Error != "" }

// tempoParts is the large opening tempo and the small line under it: "BPM", or the
// tempo changes that follow (at most two, then an ellipsis).
func tempoParts(bpms []string) (main, sub string) {
	if len(bpms) == 0 {
		return "", ""
	}
	sub = "BPM"
	if len(bpms) > 1 {
		sub = "→ " + strings.Join(bpms[1:min(len(bpms), 3)], " ")
		if len(bpms) > 3 {
			sub += " …"
		}
	}
	return bpms[0], sub
}

// tempo shows the opening tempo, large, with later tempo changes beneath.
func (u *ui) tempo(gtx C, bpms []string) D {
	if len(bpms) == 0 {
		return D{}
	}
	main, sub := tempoParts(bpms)
	return layout.Inset{Left: 12}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.text(gtx, style{monoBold, 20}, u.pal.fg, main) }),
			layout.Rigid(func(gtx C) D { return u.text(gtx, labelSmall, u.pal.fgMuted, sub) }),
		)
	})
}

// tuningTag draws "[7] Drop A".
func (u *ui) tuningTag(gtx C, t finder.Tuning) D {
	rec := op.Record(gtx.Ops)
	dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return u.badge(gtx, t.Strings) }),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx C) D { return u.text(gtx, labelLarge, u.pal.fg, t.Label()) })
		}),
	)
	call := rec.Stop()
	shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(6))
	paint.FillShape(gtx.Ops, u.pal.bg, shape.Op(gtx.Ops))
	call.Add(gtx.Ops)
	paint.FillShape(gtx.Ops, u.pal.outlineSoft, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	return dims
}

// badge shows a string count, colored by range: bass, guitar, extended.
func (u *ui) badge(gtx C, strings int) D {
	c := u.pal.badge(strings)
	sz := image.Pt(gtx.Dp(26), gtx.Dp(26))
	paint.FillShape(gtx.Ops, c.bg, clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(6)).Op(gtx.Ops))
	gtx.Constraints = layout.Exact(sz)
	layout.Center.Layout(gtx, func(gtx C) D { return u.text(gtx, style{monoBold, 13}, c.fg, fmt.Sprint(strings)) })
	return D{Size: sz}
}

// flow lays out widgets in rows, wrapping when the width runs out.
func flow(gtx C, gap int, ws []layout.Widget) D {
	maxW := gtx.Constraints.Max.X
	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	x, y, rowH, width := 0, 0, 0, 0
	for _, w := range ws {
		rec := op.Record(gtx.Ops)
		d := w(cgtx)
		call := rec.Stop()
		if x > 0 && x+d.Size.X > maxW {
			x, y, rowH = 0, y+rowH+gap, 0
		}
		off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		x += d.Size.X + gap
		rowH = max(rowH, d.Size.Y)
		width = max(width, x-gap)
	}
	return D{Size: image.Pt(width, y+rowH)}
}

// --- small pieces ---

func (u *ui) panel(gtx C, in layout.Inset, w layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	rec := op.Record(gtx.Ops)
	dims := in.Layout(gtx, w)
	call := rec.Stop()
	paint.FillShape(gtx.Ops, u.pal.panel, clip.Rect{Max: dims.Size}.Op())
	call.Add(gtx.Ops)
	return dims
}

func (u *ui) progress(gtx C) D {
	if !u.scanning {
		return D{}
	}
	w, h := gtx.Constraints.Max.X, gtx.Dp(3)
	paint.FillShape(gtx.Ops, u.pal.outline, clip.Rect{Max: image.Pt(w, h)}.Op())
	// A segment sweeping across, a third of the width.
	t := float64(gtx.Now.UnixMilli()%1500) / 1500
	seg := w / 3
	x := int(t*float64(w+seg)) - seg
	paint.FillShape(gtx.Ops, u.pal.accent, clip.Rect{Min: image.Pt(max(x, 0), 0), Max: image.Pt(min(x+seg, w), h)}.Op())
	gtx.Execute(op.InvalidateCmd{})
	return D{Size: image.Pt(w, h)}
}

func (u *ui) prompt(gtx C, title, text, action string, onClick func()) D {
	if u.cta.Clicked(gtx) {
		onClick()
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.text(gtx, titleLarge, u.pal.fg, title) }),
			layout.Rigid(layout.Spacer{Height: 12}.Layout),
			layout.Rigid(func(gtx C) D { return u.text(gtx, bodyLarge, u.pal.fgMuted, text) }),
			layout.Rigid(layout.Spacer{Height: 20}.Layout),
			layout.Rigid(func(gtx C) D {
				b := material.Button(u.th, &u.cta, action)
				b.Font, b.TextSize = labelLarge.font, labelLarge.size
				b.Background, b.Color, b.CornerRadius = u.pal.accent, u.pal.onAccent, 20
				b.Inset = layout.Inset{Left: 24, Right: 24, Top: 10, Bottom: 10}
				return b.Layout(gtx)
			}),
		)
	})
}

func (u *ui) snackbar(gtx C) {
	if u.message == "" || !gtx.Now.Before(u.msgUntil) {
		u.message = ""
		return
	}
	gtx.Execute(op.InvalidateCmd{At: u.msgUntil})
	win := gtx.Constraints.Max
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(win.X-gtx.Dp(32), gtx.Dp(640))
	rec := op.Record(gtx.Ops)
	dims := layout.Inset{Left: 16, Right: 16, Top: 14, Bottom: 14}.Layout(gtx, func(gtx C) D {
		return u.text(gtx, bodyMedium, u.pal.inverseFg, u.message)
	})
	call := rec.Stop()
	// Bottom center of the window.
	off := op.Offset(image.Pt((win.X-dims.Size.X)/2, win.Y-dims.Size.Y-gtx.Dp(16))).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, u.pal.inverseBg, clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(4)).Op(gtx.Ops))
	call.Add(gtx.Ops)
	off.Pop()
}

func (u *ui) iconButton(gtx C, b *widget.Clickable, ic *widget.Icon, desc string, enabled bool) D {
	if !enabled {
		gtx = gtx.Disabled()
	}
	c := u.pal.fg
	if !enabled {
		c = withAlpha(c, 0x60)
	}
	return b.Layout(gtx, func(gtx C) D {
		if enabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return layout.UniformInset(12).Layout(gtx, func(gtx C) D { return u.icon(gtx, ic, c) })
	})
}

func (u *ui) icon(gtx C, ic *widget.Icon, c color.NRGBA) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(24), gtx.Dp(24)))
	return ic.Layout(gtx, c)
}

func (u *ui) divider(gtx C, c color.NRGBA) D {
	sz := image.Pt(gtx.Constraints.Max.X, max(gtx.Dp(1), 1))
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: sz}.Op())
	return D{Size: sz}
}

func (u *ui) text(gtx C, s style, c color.NRGBA, txt string) D {
	l := material.Label(u.th, s.size, txt)
	l.Font, l.Color, l.MaxLines = s.font, c, 1
	return l.Layout(gtx)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}
