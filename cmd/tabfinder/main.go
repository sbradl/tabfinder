// tabfinder is the desktop app: filter your tabs by artist, song, tuning and
// tempo, and open them in TuxGuitar. It looks like the Android app (android/),
// built with Gio for a fast start.
package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
)

func main() {
	useDesktopCursor()
	go func() {
		w := new(app.Window)
		w.Option(app.Title("TabFinder"), app.Size(unit.Dp(1000), unit.Dp(900)))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	u := newUI(w.Invalidate)
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			u.drain()
			gtx := app.NewContext(&ops, e)
			u.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
