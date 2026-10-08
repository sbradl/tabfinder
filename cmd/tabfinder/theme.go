package main

import (
	_ "embed"
	"image/color"
	"log"
	"math"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/unit"
)

// The Android app's theme (android/app/src/main/java/dev/tabsync/tabfinder/theme), ported: amp and fretboard, graphite
// neutrals with a slight blue bias, brass (fret wire, amp knobs) as the accent.

type palette struct {
	bg, panel, raised, raisedHigh color.NRGBA // background, filter panel/top bar, menus, hover
	fg, fgMuted                   color.NRGBA
	outline, outlineSoft          color.NRGBA
	accent, onAccent              color.NRGBA
	err                           color.NRGBA
	inverseBg, inverseFg          color.NRGBA
	bass, guitar, extended        badgeColors // string-count badges by range
}

type badgeColors struct{ bg, fg color.NRGBA }

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

var dark = palette{
	bg: rgb(0x15181B), panel: rgb(0x1D2125), raised: rgb(0x2C3238), raisedHigh: rgb(0x242930),
	fg: rgb(0xE8E4DC), fgMuted: rgb(0x9BA3AB),
	outline: rgb(0x3A4148), outlineSoft: rgb(0x2C3238),
	accent: rgb(0xD9A441), onAccent: rgb(0x15181B),
	err:       rgb(0xFFB4AB),
	inverseBg: rgb(0xE8E4DC), inverseFg: rgb(0x1A1D20),
	bass:     badgeColors{rgb(0x233240), rgb(0x8FB3D1)},
	guitar:   badgeColors{rgb(0x2C3238), rgb(0xE8E4DC)},
	extended: badgeColors{rgb(0x3B3020), rgb(0xD9A441)},
}

var light = palette{
	bg: rgb(0xF1F3F4), panel: rgb(0xFFFFFF), raised: rgb(0xE4E7EA), raisedHigh: rgb(0xF1F3F4),
	fg: rgb(0x1A1D20), fgMuted: rgb(0x6B7580),
	outline: rgb(0xC3C9CF), outlineSoft: rgb(0xE4E7EA),
	accent: rgb(0x7A5200), onAccent: rgb(0xFFFFFF),
	err:       rgb(0xBA1A1A),
	inverseBg: rgb(0x1D2125), inverseFg: rgb(0xE8E4DC),
	bass:     badgeColors{rgb(0xDCE8F2), rgb(0x2F5878)},
	guitar:   badgeColors{rgb(0xE4E7EA), rgb(0x1A1D20)},
	extended: badgeColors{rgb(0xF6E7C6), rgb(0x7A5200)},
}

func (p palette) badge(strings int) badgeColors {
	switch {
	case strings <= 5:
		return p.bass
	case strings == 6:
		return p.guitar
	default:
		return p.extended
	}
}

// contrast is WCAG's contrast ratio of two colors, 1 to 21.
func contrast(a, b color.NRGBA) float64 {
	lum := func(c color.NRGBA) float64 {
		ch := func(v uint8) float64 {
			f := float64(v) / 255
			if f <= 0.03928 {
				return f / 12.92
			}
			return math.Pow((f+0.055)/1.055, 2.4)
		}
		return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
	}
	la, lb := lum(a), lum(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

//go:embed fonts/barlow_condensed_medium.ttf
var barlowMedium []byte

//go:embed fonts/barlow_condensed_semibold.ttf
var barlowSemiBold []byte

//go:embed fonts/roboto_regular.ttf
var robotoRegular []byte

// Barlow Condensed (amp panel lettering) for titles and labels, Roboto (as on Android) for
// running text, Go Mono for numbers and notes, which line up like tab. All embedded; see newShaper.
var (
	condensed       = font.Font{Typeface: "Barlow Condensed", Weight: font.SemiBold}
	condensedMedium = font.Font{Typeface: "Barlow Condensed", Weight: font.Medium}
	body            = font.Font{Typeface: "Roboto"}
	mono            = font.Font{Typeface: "Go Mono"}
	monoBold        = font.Font{Typeface: "Go Mono", Weight: font.Bold}
)

// The Android type scale.
type style struct {
	font font.Font
	size unit.Sp
}

var (
	titleLarge  = style{condensed, 26}
	titleMedium = style{condensed, 20}
	bodyLarge   = style{body, 16}
	bodyMedium  = style{body, 14}
	labelLarge  = style{condensed, 16}
	labelMedium = style{condensedMedium, 14}
	labelSmall  = style{mono, 12}
)

func newShaper() *text.Shaper {
	faces := gofont.Collection()
	for _, ttf := range [][]byte{barlowMedium, barlowSemiBold, robotoRegular} {
		f, err := opentype.ParseCollection(ttf)
		if err != nil {
			log.Fatal(err)
		}
		faces = append(faces, f...)
	}
	// No system fonts: scanning and loading them cost about 8 MB of heap, and these fonts
	// cover the Latin, Greek and Cyrillic text tab names use.
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))
}
