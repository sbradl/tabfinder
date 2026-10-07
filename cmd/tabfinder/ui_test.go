package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// harness drives the real UI offscreen: synthetic clicks and typing, and
// optional screenshots (set TABFINDER_SHOTS to a directory to get them).
type harness struct {
	t     *testing.T
	u     *ui
	r     input.Router
	ops   op.Ops
	size  image.Point
	now   time.Time
	shots string

	toolDir string          // where the fake programs live, if any
	timers  []time.Duration // delays of timers the app started (afterFunc)
}

const scale = 2 // px per dp, close to the 1.7 desktop

func newHarness(t *testing.T) *harness {
	return newHarnessWith(t, harnessOpts{})
}

// harnessOpts sets up what the app finds when it starts. The zero value is the
// sample index with the folder /tabs/Tabs set.
type harnessOpts struct {
	index    *string           // content of the cached index file; nil: sampleIndex
	noIndex  bool              // no cached index at all
	root     *string           // folder in config.json; nil: /tabs/Tabs
	noConfig bool              // no config.json
	legacy   string            // content of a settings.properties
	tools    map[string]string // fake programs, the only ones on PATH (see fakeTools); nil: just a gsettings that says dark
	size     image.Point       // window size in dp; zero: 1000 x 900
	waitLoad bool              // false: don't wait for the index to be loaded
}

func str(s string) *string { return &s }

func newHarnessWith(t *testing.T, o harnessOpts) *harness {
	configDir, cacheDir := isolate(t)
	os.MkdirAll(cacheDir, 0o755)
	switch {
	case o.noIndex:
	case o.index != nil:
		os.WriteFile(indexFile, []byte(*o.index), 0o644)
	default:
		os.WriteFile(indexFile, []byte(sampleIndex), 0o644)
	}
	if !o.noConfig {
		root := "/tabs/Tabs"
		if o.root != nil {
			root = *o.root
		}
		if err := saveRoot(root); err != nil {
			t.Fatal(err)
		}
	}
	if o.legacy != "" {
		os.MkdirAll(configDir, 0o755)
		os.WriteFile(filepath.Join(configDir, "settings.properties"), []byte(o.legacy), 0o644)
	}
	if o.tools == nil {
		o.tools = map[string]string{"gsettings": `printf "'prefer-dark'\n"`}
	}
	dir := fakeTools(t, o.tools)
	if o.size == (image.Point{}) {
		o.size = image.Pt(1000, 900)
	}
	h := &harness{t: t, toolDir: dir, size: image.Pt(o.size.X*scale, o.size.Y*scale), now: time.Unix(0, 0), shots: os.Getenv("TABFINDER_SHOTS")}
	// A clock the test owns: the snackbar expires when the harness's time passes, not the wall clock's.
	oldNow, oldAfter := now, afterFunc
	now = func() time.Time { return h.now }
	afterFunc = func(d time.Duration, f func()) *time.Timer {
		h.timers = append(h.timers, d)
		return time.NewTimer(time.Hour)
	}
	t.Cleanup(func() { now, afterFunc = oldNow, oldAfter })
	h.u = newUI(func() {})
	if !o.waitLoad {
		for i := 0; i < 100 && !h.u.loaded; i++ {
			time.Sleep(10 * time.Millisecond)
			h.frame()
		}
		if !h.u.loaded {
			t.Fatal("index not loaded")
		}
	}
	h.frame()
	return h
}

func (h *harness) frame() {
	h.u.drain()
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Constraints: layout.Exact(h.size),
		Source:      h.r.Source(),
		Now:         h.now,
	}
	h.u.layout(gtx)
	h.r.Frame(&h.ops)
	h.now = h.now.Add(16 * time.Millisecond)
}

// click clicks at a point given in dp.
func (h *harness) click(x, y float32) {
	p := f32.Pt(x*scale, y*scale)
	h.now = h.now.Add(time.Second) // a second apart, so not a double click
	at := h.now.Sub(time.Unix(0, 0))
	h.r.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p, Time: at},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: p, Time: at},
	)
	h.frame()
	h.r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: p, Time: at + 50*time.Millisecond})
	h.frame()
	h.frame()
}

// typ types text into the focused field, one character per frame, the way
// app.Window does: replace the selection, then move the caret after the text.
func (h *harness) typ(s string) {
	for _, c := range s {
		sel := h.r.EditorState().Selection.Range
		start := min(sel.Start, sel.End)
		after := key.Range{Start: start + 1, End: start + 1}
		h.r.Queue(key.EditEvent{Range: sel, Text: string(c)}, key.SelectionEvent(after))
		h.frame()
	}
	h.frame()
}

func (h *harness) press(name key.Name) {
	h.r.Queue(key.Event{Name: name, State: key.Press}, key.Event{Name: name, State: key.Release})
	h.frame()
	h.frame()
}

func (h *harness) shot(name string) {
	if h.shots == "" {
		return
	}
	w, err := headless.NewWindow(h.size.X, h.size.Y)
	if err != nil {
		h.t.Fatal(err)
	}
	defer w.Release()
	// Lay out once more into fresh ops for the renderer.
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
	f, _ := os.Create(filepath.Join(h.shots, name+".png"))
	defer f.Close()
	png.Encode(f, img)
}

const sampleIndex = `{"path":"Soilbed Quartet/Brass Kettle.gp5","format":"gp5","artist":"Soilbed Quartet","title":"Brass Kettle","tracks":[{"name":"G","pitches":[36,43,48,53,57,62],"tuning":"Drop C (C G C F A D)"},{"name":"B","pitches":[24,31,36,41],"tuning":"Drop C (C G C F)"}],"tempos":[{"bar":1,"bpm":190},{"bar":91,"bpm":145}]}
{"path":"Amber Marsh/First Frost.gp5","format":"gp5","artist":"Amber Marsh","title":"First Frost","tracks":[{"name":"G","pitches":[38,45,50,55,59,64],"tuning":"D Standard (D G C F A D)"},{"name":"G2","pitches":[35,40,45,50,55,59],"tuning":"Custom (B E A D G B)"}],"tempos":[{"bar":1,"bpm":189}]}
{"path":"Amber Marsh/Raise Your Lanterns.gp5","format":"gp5","artist":"Amber Marsh","title":"Raise Your Lanterns","tracks":[{"name":"G","pitches":[33,40,45,50,54,59],"tuning":"Drop A (A E A D F# B)"}],"tempos":[{"bar":1,"bpm":110}]}
{"path":"Inkwell Flamingos/Mirage.gp5","format":"gp5","artist":"Inkwell Flamingos","title":"Mirage","tracks":[{"name":"G","pitches":[35,40,45,50,55,59,64],"tuning":"B Standard (B E A D G B E)"}],"tempos":[{"bar":1,"bpm":120}]}
`

// Field centers in dp at the 1000 dp wide test window.
var (
	artistAt = f32.Pt(217, 104)
	songAt   = f32.Pt(700, 104)
	tuningAt = f32.Pt(384, 168)
	bpmAt    = f32.Pt(836, 168)
)

func TestInputs(t *testing.T) {
	h := newHarness(t)
	h.shot("01-start")

	h.click(artistAt.X, artistAt.Y)
	if !h.r.Source().Focused(&h.u.artist.editor) {
		t.Fatal("clicking the artist field doesn't focus it")
	}
	h.shot("02-artist-focused")

	h.typ("am")
	if h.u.in.Artist != "am" || h.u.artist.editor.Text() != "am" {
		t.Errorf("typed am: field %q, query %q", h.u.artist.editor.Text(), h.u.in.Artist)
	}
	h.shot("03-artist-typed")

	h.press(key.NameReturn)
	if h.u.in.Artist != "Amber Marsh" {
		t.Errorf("Enter picks the first suggestion: query %q", h.u.in.Artist)
	}
	if got := len(h.u.result.Matches); got != 2 {
		t.Errorf("Amber Marsh: %d matches", got)
	}
	h.shot("04-artist-picked")

	h.click(tuningAt.X, tuningAt.Y)
	h.shot("05-tuning-menu")
	// The first suggestion sits below the field: header, then items.
	h.click(tuningAt.X, tuningAt.Y+28+4+36+22)
	if h.u.in.Tuning == "" || h.u.in.Strings == 0 {
		t.Errorf("clicking a tuning suggestion: query %+v", h.u.in)
	}
	if h.u.tuning.dismissed != true {
		t.Error("picking a tuning leaves the menu open")
	}
	h.shot("06-tuning-picked")

	// Retyping the picked text after deleting a character still counts.
	h.click(artistAt.X, artistAt.Y)
	h.press(key.NameDeleteBackward)
	h.typ("h")
	if h.u.in.Artist != "Amber Marsh" {
		t.Errorf("retyped artist: query %q", h.u.in.Artist)
	}
	h.press(key.NameEscape)

	h.click(bpmAt.X, bpmAt.Y)
	h.typ("100-150")
	if h.u.in.BPM != "100-150" {
		t.Errorf("bpm %q", h.u.in.BPM)
	}
	h.shot("07-bpm")

	h.click(songAt.X, songAt.Y)
	h.typ("ki")
	h.press(key.NameEscape)
	h.shot("08-song")
	if h.u.in.Name != "ki" {
		t.Errorf("song %q", h.u.in.Name)
	}

	// Nothing matches now: the prompt's button clears every field.
	if len(h.u.result.Matches) != 0 {
		t.Fatalf("expected no matches, got %d", len(h.u.result.Matches))
	}
	h.click(500, 594) // "Clear filters", below the centered prompt text
	h.frame()
	if h.u.in.Active() || h.u.artist.editor.Text() != "" || len(h.u.result.Matches) != 4 {
		t.Errorf("after Clear filters: query %+v, artist field %q, %d matches", h.u.in, h.u.artist.editor.Text(), len(h.u.result.Matches))
	}
	h.shot("09-cleared")
}

// A focused field's suggestions come back after Escape or a pick without
// leaving the field: by clicking it again, or with the down arrow.
func TestReopenSuggestions(t *testing.T) {
	h := newHarness(t)
	open := func() bool { return h.r.Source().Focused(&h.u.artist.editor) && !h.u.artist.dismissed }

	h.click(artistAt.X, artistAt.Y)
	if !open() {
		t.Fatal("focusing the field doesn't open its suggestions")
	}
	h.press(key.NameEscape)
	if open() {
		t.Fatal("Escape doesn't close the suggestions")
	}
	h.click(artistAt.X, artistAt.Y)
	if !open() {
		t.Error("clicking the focused field doesn't reopen its suggestions")
	}
	h.press(key.NameEscape)
	h.press(key.NameDownArrow)
	if !open() {
		t.Error("the down arrow doesn't reopen the suggestions")
	}
}
