package main

import (
	"fmt"
	"image"
	"image/color"
	"runtime/debug"
	"sync"
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

	"tabfinder/internal/finder"
	"tabfinder/internal/rows"
	"tabfinder/internal/tab"
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

// clock is the time the app goes by, and its timers; tests use their own, so they needn't sleep.
type clock struct {
	now       func() time.Time
	afterFunc func(time.Duration, func()) *time.Timer
}

var systemClock = clock{time.Now, time.AfterFunc}

// ui is the window's widgets over a session. Each frame first applies the input since the
// last one (update), then draws (layout), which changes nothing.
type ui struct {
	*session
	dirs   dirs
	clock  clock
	redraw func() // asks for a frame, from any goroutine
	pal    palette
	th     *material.Theme
	posts  chan func()    // state changes from background work, run on the UI goroutine
	opens  sync.WaitGroup // songs being opened in TuxGuitar, which may copy a file first

	artist, name, tuning, bpm field
	list                      widget.List
	rows                      map[string]*gesture.Click // by song path, so a row's hover and press stay with its song
	folderBtn, rescanBtn, cta widget.Clickable
	window                    int // tag for every press in the window, see leaveFields
	levels
}

func newUI(redraw func(), d dirs, c clock) *ui {
	root, rootErr := d.loadRoot()
	u := &ui{session: newSession(root, c.now), dirs: d, clock: c, redraw: redraw, pal: dark, posts: make(chan func(), 16), rows: map[string]*gesture.Click{}, levels: newLevels()}
	if dark, ok := prefersDark(); ok && !dark {
		u.pal = light
	}
	u.th = material.NewTheme()
	u.th.Shaper = newShaper()
	u.th.Palette = material.Palette{Bg: u.pal.bg, Fg: u.pal.fg, ContrastBg: u.pal.accent, ContrastFg: u.pal.onAccent}
	u.list.Axis = layout.Vertical
	u.artist.label, u.name.label, u.tuning.label, u.bpm.label = "Artist", "Song", "Tuning", "BPM"
	for _, f := range u.fields() {
		f.editor.SingleLine = true
		f.editor.Submit = true
	}
	u.bpm.placeholder = "100-140"
	if rootErr != nil {
		u.show("Couldn't read the settings: " + rootErr.Error())
	}
	u.artist.set = func(t string) { u.in.Artist = t }
	u.name.set = func(t string) { u.in.Name = t }
	u.tuning.set = u.typeTuning
	u.bpm.set = func(t string) { u.in.BPM = t }
	go func() {
		songs, err := finder.LoadIndex(u.dirs.index())
		u.post(func() {
			scan := u.indexLoaded(songs, err)
			u.freeMemorySoon()
			if scan {
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

// freeMemorySoon is for after new songs: decoding them and drawing the new list for the
// first time (shaping text, caching glyphs) leave garbage twice the size of what's live; Go
// would keep it as headroom. Hand it back to the system once that first drawing is done.
func (u *ui) freeMemorySoon() { u.clock.afterFunc(2*time.Second, debug.FreeOSMemory) }

func (u *ui) fields() []*field { return []*field{&u.artist, &u.name, &u.tuning, &u.bpm} }

func (u *ui) rescan() {
	root, ok := u.startScan()
	if !ok {
		return
	}
	go func() {
		start := time.Now()
		songs, err := finder.ScanIndex(root, u.dirs.index())
		u.post(func() {
			u.scanDone(songs, err, time.Since(start))
			u.freeMemorySoon()
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
				u.folderChosen(dir)
				clear(u.rows) // rows of the old folder's songs
				if err := u.dirs.saveRoot(dir); err != nil {
					u.show("Couldn't save the folder: " + err.Error())
				}
				u.dirs.dropIndex()
				u.rescan()
			}
		})
	}()
}

// update applies the input since the last frame, which acted on what that frame drew.
func (u *ui) update(gtx C) {
	if u.cta.Clicked(gtx) {
		if p, _ := u.placeholder(); p != nil {
			p.do()
		}
	}
	u.updateLevels(gtx)
	u.handleRows(gtx)
	for _, f := range u.fields() {
		u.handleField(gtx, f)
	}
	u.leaveFields(gtx)
	if u.folderBtn.Clicked(gtx) {
		u.chooseFolder()
	}
	if u.rescanBtn.Clicked(gtx) {
		u.rescan()
	}
	u.expireMessage(gtx.Now)
	u.search()
	u.artist.menu, u.tuning.menu = u.artistMenu(), u.tuningMenu()
	if u.applyEnter() {
		u.search()
		u.artist.menu, u.tuning.menu = u.artistMenu(), u.tuningMenu()
	}
}

func (u *ui) layout(gtx C) D {
	u.update(gtx)
	paint.Fill(gtx.Ops, u.pal.bg)
	// Under everything, so it sees every press but those on menus, which are drawn last.
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &u.window)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.topBar),
		layout.Flexed(1, u.content),
	)
	u.levelsPopup(gtx)
	u.snackbar(gtx)
	return D{Size: gtx.Constraints.Max}
}

// leaveFields closes the suggestions of the fields beside a press, and leaves the focused
// field if the press is on none. Presses on the suggestions don't get here.
func (u *ui) leaveFields(gtx C) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &u.window, Kinds: pointer.Press}); !ok {
			return
		}
		onField := false
		for _, f := range u.fields() {
			if f.reopen.Hovered() {
				onField = true
			} else {
				f.dismissed = true
			}
		}
		if !onField {
			gtx.Execute(key.FocusCmd{})
		}
	}
}

// prompt is a message with a button, shown instead of the song list.
type prompt struct {
	title, text, action string
	do                  func()
}

// placeholder is what shows instead of the song list: a prompt, or a blank space while the
// songs are on their way. Neither (nil, false) means there's a list to show.
func (u *ui) placeholder() (p *prompt, blank bool) {
	none := len(u.result.Matches) == 0
	switch {
	case u.root == "":
		return &prompt{"Choose your tab folder", "Pick the folder that holds your Guitar Pro, TuxGuitar and Power Tab files.", "Choose folder", u.chooseFolder}, false
	case !u.loaded:
		return nil, true // reading the saved scan, a moment at most
	case none && u.scanning:
		return nil, true
	case none && u.in.Active():
		return &prompt{"No tabs match", "Try fewer filters.", "Clear filters", u.clearFilters}, false
	case none:
		return &prompt{"No tabs found", "Rescan, or choose another folder.", "Rescan", u.rescan}, false
	}
	return nil, false
}

func (u *ui) content(gtx C) D {
	p, blank := u.placeholder()
	if u.root == "" {
		return u.prompt(gtx, p)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.filters),
		layout.Rigid(u.progress),
		layout.Flexed(1, func(gtx C) D {
			switch {
			case blank:
				return D{Size: gtx.Constraints.Max}
			case p != nil:
				return u.prompt(gtx, p)
			}
			return u.songList(gtx)
		}),
	)
}

func (u *ui) clearFilters() {
	for _, f := range u.fields() {
		f.editor.SetText("")
	}
	u.clearQuery()
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
			layout.Rigid(func(gtx C) D {
				if u.root == "" {
					return D{}
				}
				return layout.Inset{Right: 8}.Layout(gtx, u.sortButton)
			}),
			layout.Rigid(func(gtx C) D { return u.iconButton(gtx, &u.folderBtn, icFolder, "Choose tab folder", true) }),
			layout.Rigid(func(gtx C) D {
				return u.iconButton(gtx, &u.rescanBtn, icRefresh, "Rescan", u.root != "" && !u.scanning)
			}),
		)
	})
}

// subtitle is the line under the title: how many tabs, in which folder.
func (u *ui) subtitle() string { return rows.LibraryLine(len(u.lib.Entries), u.root) }

// counter is "matches / total".
func (u *ui) counter() string { return rows.Counter(len(u.result.Matches), len(u.lib.Entries)) }

func (u *ui) filters(gtx C) D {
	return u.panel(gtx, layout.Inset{Left: 16, Right: 16, Top: 12, Bottom: 12}, func(gtx C) D {
		gap := layout.Spacer{Width: 8}.Layout
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return u.field(gtx, &u.artist, nil, false)
					}),
					layout.Rigid(gap),
					layout.Flexed(1.4, func(gtx C) D {
						return u.field(gtx, &u.name, func(gtx C) D { return u.icon(gtx, icSearch, u.pal.fgMuted) }, false)
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
						return u.field(gtx, &u.tuning, badge, false)
					}),
					layout.Rigid(gap),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(150), gtx.Dp(150)
						return u.field(gtx, &u.bpm, nil, u.result.BPMInvalid)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: 16, Right: 8}.Layout(gtx, func(gtx C) D {
							return u.text(gtx, style{monoBold, 16}, u.pal.accent, u.counter())
						})
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(u.levelsLine),
		)
	})
}

func (u *ui) artistMenu() *menu {
	var items []menuItem
	for _, a := range u.artistSuggestions() {
		items = append(items, menuItem{text: a})
	}
	return &menu{items: items, pick: func(it menuItem) {
		u.artist.pick(it.text)
		u.in.Artist = it.text
	}}
}

func (u *ui) tuningMenu() *menu {
	var items []menuItem
	for _, t := range u.tuningSuggestions() {
		items = append(items, menuItem{text: t.Label, detail: t.Detail, section: rows.Section(t.Strings), tuning: t})
	}
	return &menu{items: items, pick: func(it menuItem) {
		u.tuning.pick(it.tuning.Label)
		u.pickTuning(it.tuning)
	}}
}

// handleField applies the typing, clearing, keys and clicks on a field since the last frame,
// and a pick by pointer from the suggestions that frame showed.
func (u *ui) handleField(gtx C, f *field) {
	focused := gtx.Focused(&f.editor)
	if focused && !f.wasFocused {
		f.dismissed = false
	}
	f.wasFocused = focused
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
	// A pick's change event came in above, or the editor's layout took it the frame it was made.
	f.picked = ""
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
	if changed {
		f.set(f.editor.Text())
	}
	// The menu's background takes the clicks that would reach the rows below it.
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &f.menuBlock, Kinds: pointer.Press | pointer.Release}); !ok {
			break
		}
	}
	if f.menu == nil {
		return
	}
	for i := range min(len(f.menu.items), len(f.menuItems)) {
		for {
			ev, ok := f.menuItems[i].Update(gtx.Source)
			if !ok {
				break
			}
			// On press: the editor may lose focus before a release arrives.
			if ev.Kind == gesture.KindPress {
				f.menu.pick(f.menu.items[i])
				f.dismissed = true
			}
		}
	}
}

// applyEnter picks the first suggestion of a field where Enter was pressed: of what's typed
// now, so update calls it with the menus up to date. It reports whether anything was picked.
func (u *ui) applyEnter() bool {
	picked := false
	for _, f := range u.fields() {
		if f.submitted && f.menu != nil && len(f.menu.items) > 0 && !f.dismissed {
			f.menu.pick(f.menu.items[0])
			f.dismissed = true
			picked = true
		}
		f.submitted = false
	}
	return picked
}

// handleRows opens the songs whose rows were clicked: rows of the last frame's result, which
// is for the songs of then (resultFor); new songs may have come in since.
func (u *ui) handleRows(gtx C) {
	if u.resultFor == nil {
		return
	}
	for _, m := range u.result.Matches {
		s := u.resultFor.Entries[m].Song
		click := u.rows[s.Path]
		if click == nil {
			continue // never drawn
		}
		for {
			ev, ok := click.Update(gtx.Source)
			if !ok {
				break
			}
			if ev.Kind == gesture.KindClick {
				u.open(s)
			}
		}
	}
}

// open opens s in TuxGuitar, off the UI goroutine: a misnamed file is copied first.
func (u *ui) open(s *tab.Song) {
	root := u.root
	u.opens.Go(func() {
		if err := u.dirs.openInTuxGuitar(root, s); err != nil {
			u.post(func() { u.show(err.Error()) })
		}
	})
}

// rowClick is the click state of the row of the i-th match.
func (u *ui) rowClick(i int) *gesture.Click {
	path := u.lib.Entries[u.result.Matches[i]].Song.Path
	c := u.rows[path]
	if c == nil {
		c = new(gesture.Click)
		u.rows[path] = c
	}
	return c
}

// --- text field with suggestions ---

type field struct {
	label, placeholder string
	set                func(text string) // puts what's typed into the query
	editor             widget.Editor
	picked             string // text set by picking a suggestion; its change event isn't typing
	clear              widget.Clickable
	submitted          bool          // Enter was pressed
	dismissed          bool          // suggestions closed with Escape or a pick, until reopened
	reopen             gesture.Click // a click anywhere on the field, which reopens them
	wasFocused         bool
	menu               *menu // the suggestions; nil for none

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
	tuning                rows.Tuning // the suggestion, in the tuning field's menu
}

type menu struct {
	items []menuItem
	pick  func(menuItem)
}

// field draws an outlined text field like Material's, with a floating label,
// a clear button and, if it has any, its suggestions while focused.
func (u *ui) field(gtx C, f *field, leading layout.Widget, isErr bool) D {
	focused := gtx.Focused(&f.editor)

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

	if m := f.menu; m != nil && focused && !f.dismissed && len(m.items) > 0 {
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
	ls := material.List(u.th, &u.list)
	ls.Indicator.Color, ls.Indicator.HoverColor = u.pal.outline, u.pal.fgMuted
	return ls.Layout(gtx, len(u.result.Matches), func(gtx C, i int) D {
		return u.songRow(gtx, u.rowClick(i), u.songRows[u.result.Matches[i]])
	})
}

func (u *ui) songRow(gtx C, click *gesture.Click, r rows.Song) D {
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
									layout.Rigid(func(gtx C) D { return u.text(gtx, titleMedium, u.pal.fg, r.Title) }),
									layout.Rigid(func(gtx C) D { return u.text(gtx, bodyMedium, u.pal.fgMuted, r.Subtitle) }),
								)
							}),
							layout.Rigid(func(gtx C) D { return u.tempo(gtx, r.Tempo, r.TempoDetail) }),
						)
					}),
					layout.Rigid(func(gtx C) D {
						switch {
						case len(r.Tunings) > 0 || len(r.Parts) > 0:
							return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D { return u.tagsAndParts(gtx, r) })
						case r.Unreadable:
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

// tagsAndParts is the row's tuning tags, and its parts at the right; on a line of their
// own below the tags when both don't fit on one.
func (u *ui) tagsAndParts(gtx C, r rows.Song) D {
	gtx.Constraints.Min = image.Point{}
	width := gtx.Constraints.Max.X
	rec := op.Record(gtx.Ops)
	tags := make([]layout.Widget, len(r.Tunings))
	for i, t := range r.Tunings {
		tags[i] = func(gtx C) D { return u.tuningTag(gtx, t) }
	}
	td := flow(gtx, gtx.Dp(6), tags)
	tagsCall := rec.Stop()
	rec = op.Record(gtx.Ops)
	pd := u.parts(gtx, r.Parts)
	partsCall := rec.Stop()

	tagsCall.Add(gtx.Ops)
	var at image.Point
	switch {
	case len(r.Parts) == 0:
		return td
	case len(r.Tunings) == 0:
	case td.Size.X+gtx.Dp(24)+pd.Size.X <= width:
		at = image.Pt(width-pd.Size.X, (td.Size.Y-pd.Size.Y)/2)
	default:
		at = image.Pt(0, td.Size.Y+gtx.Dp(10))
	}
	off := op.Offset(at).Push(gtx.Ops)
	partsCall.Add(gtx.Ops)
	off.Pop()
	return D{Size: image.Pt(max(td.Size.X, at.X+pd.Size.X), max(td.Size.Y, at.Y+pd.Size.Y))}
}

// tempo shows the opening tempo, large, with later tempo changes beneath.
func (u *ui) tempo(gtx C, main, sub string) D {
	if main == "" {
		return D{}
	}
	return layout.Inset{Left: 12}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.text(gtx, style{monoBold, 20}, u.pal.fg, main) }),
			layout.Rigid(func(gtx C) D { return u.text(gtx, labelSmall, u.pal.fgMuted, sub) }),
		)
	})
}

// tuningTag draws "[7] Drop A".
func (u *ui) tuningTag(gtx C, t rows.Tuning) D {
	rec := op.Record(gtx.Ops)
	dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return u.badge(gtx, t.Strings) }),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx C) D { return u.text(gtx, labelLarge, u.pal.fg, t.Label) })
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

func (u *ui) prompt(gtx C, p *prompt) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.text(gtx, titleLarge, u.pal.fg, p.title) }),
			layout.Rigid(layout.Spacer{Height: 12}.Layout),
			layout.Rigid(func(gtx C) D { return u.text(gtx, bodyLarge, u.pal.fgMuted, p.text) }),
			layout.Rigid(layout.Spacer{Height: 20}.Layout),
			layout.Rigid(func(gtx C) D {
				b := material.Button(u.th, &u.cta, p.action)
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
