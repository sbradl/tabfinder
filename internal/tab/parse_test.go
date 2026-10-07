package tab

import (
	"slices"
	"strings"
	"testing"
	"time"

	"tabfinder/internal/tabfiles"
)

func TestParseBytesDispatch(t *testing.T) {
	album := "Alb"
	tests := []struct {
		name    string
		in      []byte
		format  string // wanted format, or ""
		errPart string // wanted error text, or ""
	}{
		{"empty", nil, "", "empty file"},
		{"unknown", []byte("hello world, not a tab"), "", "unknown file format"},
		{"one byte", []byte{1}, "", "unknown file format"},
		{"zip magic", []byte("PK\x03\x04junk"), "", ""}, // a zip error, not "unknown file format"
		{"gpx magic", []byte("BCFZ"), "", "gpx"},
		{"bcfs magic", []byte("BCFS"), "", "gpx"},
		{"ptab magic", []byte("ptab\x01\x00"), "", "Power Tab 1.0"},
		{"gp3", tabfiles.GP3(tabfiles.GP3Spec{Title: "T", Artist: "A", Tempo: 120, Tracks: []tabfiles.GP3Track{{Name: "G", Strings: tabfiles.StdGuitar}}}), "gp3", ""},
		{"tg", tabfiles.TG1("T", "A", "B"), "tg", ""},
		{"ptb", tabfiles.PTB(3, "T", "A", &album), "ptb", ""},
	}
	for _, tt := range tests {
		s, err := parseBytes(tt.in)
		switch {
		case tt.format != "":
			if err != nil || s.Format != tt.format {
				t.Errorf("%s: got %+v, %v; want format %s", tt.name, s, err, tt.format)
			}
		case err == nil:
			t.Errorf("%s: no error, got %+v", tt.name, s)
		case tt.errPart != "" && !strings.Contains(err.Error(), tt.errPart):
			t.Errorf("%s: error %q, want it to contain %q", tt.name, err, tt.errPart)
		}
	}
	// "unknown file format" is only for content nobody claims.
	if _, err := parseBytes([]byte("PK\x03\x04junk")); err == nil || err.Error() == "unknown file format" {
		t.Errorf("corrupt zip error = %v", err)
	}
}

func TestParseBytesIgnoresExtensionAndOffset(t *testing.T) {
	// GUITAR must be within bytes 1-31; the length byte at 0 doesn't count,
	// and a file that merely mentions GUITAR later isn't a GP file.
	late := append(make([]byte, 40), "GUITAR"...)
	late[0] = 0x18
	if _, err := parseBytes(late); err == nil || err.Error() != "unknown file format" {
		t.Errorf("GUITAR at offset 40: err = %v", err)
	}
	short := append([]byte{0x18}, "FICHIER GUITAR PRO v3.00"...) // 25 bytes: too short to sniff
	if _, err := parseBytes(short); err == nil || err.Error() != "unknown file format" {
		t.Errorf("short GP header: err = %v", err)
	}
}

func TestParseGP3Synthetic(t *testing.T) {
	b := tabfiles.GP3(tabfiles.GP3Spec{
		Title: "Brass Kettle", Artist: "Soilbed Quartet", Album: "Glass", Tempo: 190,
		Tracks: []tabfiles.GP3Track{
			{Name: "Guitar", Strings: tabfiles.StdGuitar},
			{Name: "Bass", Strings: []int{43, 38, 33, 28}},
			{Name: "Drums", Drums: true, Strings: []int{0, 0, 0, 0, 0, 0}},
		},
	})
	s, err := parseBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format != "gp3" || s.Title != "Brass Kettle" || s.Artist != "Soilbed Quartet" || s.Album != "Glass" {
		t.Errorf("header = %+v", s)
	}
	if want := []Tempo{{1, 190}}; !slices.Equal(s.Tempos, want) {
		t.Errorf("tempos = %v", s.Tempos)
	}
	want := []Track{
		{Name: "Guitar", Pitches: []int{40, 45, 50, 55, 59, 64}, Tuning: "E Standard (E A D G B E)", Instrument: "Acoustic Grand Piano"},
		{Name: "Bass", Pitches: []int{28, 33, 38, 43}, Tuning: "E Standard (E A D G)", Instrument: "Acoustic Grand Piano"},
		{Name: "Drums", Drums: true, Instrument: "Drums"},
	}
	if len(s.Tracks) != len(want) {
		t.Fatalf("tracks = %+v", s.Tracks)
	}
	for i, w := range want {
		g := s.Tracks[i]
		if g.Name != w.Name || g.Drums != w.Drums || g.Tuning != w.Tuning || !slices.Equal(g.Pitches, w.Pitches) {
			t.Errorf("track %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestParseZip(t *testing.T) {
	gpifXML := tabfiles.GPIF("Nectar", "Argyle Moth", "Rent", [][2]float64{{0, 120}, {8, 150}},
		tabfiles.GPIFTrack{Name: "Lead", Instrument: "Electric Guitar", Kind: "electricGuitar", Pitches: "40 45 50 55 59 64"})
	tgXML := tabfiles.TG2("Song", "Band", "Disc", []int{100, 100, 130},
		[]tabfiles.TG2Channel{{ID: 1, Bank: 0, Program: 30}, {ID: 2, Bank: 128, Program: 0}},
		tabfiles.TG2Track{Name: "Git", Channel: 1, Strings: []int{64, 59, 55, 50, 45, 38}},
		tabfiles.TG2Track{Name: "Dr", Channel: 2})
	inner := tabfiles.TG1("Inner", "Art", "Alb")

	t.Run("gp7", func(t *testing.T) {
		s, err := parseBytes(tabfiles.Zip(map[string][]byte{"Content/score.gpif": gpifXML}))
		if err != nil {
			t.Fatal(err)
		}
		if s.Format != "gp7" || s.Title != "Nectar" || s.Artist != "Argyle Moth" || s.Album != "Rent" {
			t.Errorf("song = %+v", s)
		}
		if want := []Tempo{{1, 120}, {9, 150}}; !slices.Equal(s.Tempos, want) {
			t.Errorf("tempos = %v", s.Tempos)
		}
		if len(s.Tracks) != 1 || s.Tracks[0].Tuning != "E Standard (E A D G B E)" || s.Tracks[0].Instrument != "Electric Guitar" {
			t.Errorf("tracks = %+v", s.Tracks)
		}
	})
	t.Run("tuxguitar2", func(t *testing.T) {
		s, err := parseBytes(tabfiles.Zip(map[string][]byte{"version.txt": []byte("2"), "content.xml": tgXML}))
		if err != nil {
			t.Fatal(err)
		}
		if s.Format != "tg" || s.Title != "Song" || s.Artist != "Band" || s.Album != "Disc" {
			t.Errorf("song = %+v", s)
		}
		if want := []Tempo{{1, 100}, {3, 130}}; !slices.Equal(s.Tempos, want) {
			t.Errorf("tempos = %v", s.Tempos)
		}
		if len(s.Tracks) != 2 || s.Tracks[0].Tuning != "Drop D (D A D G B E)" || !s.Tracks[1].Drums || s.Tracks[1].Tuning != "" {
			t.Errorf("tracks = %+v", s.Tracks)
		}
	})
	t.Run("gpif wins over content.xml", func(t *testing.T) {
		s, err := parseBytes(tabfiles.Zip(map[string][]byte{"content.xml": tgXML, "Content/score.gpif": gpifXML}, "content.xml", "Content/score.gpif"))
		if err != nil || s.Format != "gp7" {
			t.Errorf("got %+v, %v", s, err)
		}
	})
	t.Run("wrapped tab, recursive", func(t *testing.T) {
		for _, name := range []string{"x.tg", "dir/X.TG", "x.tg.crdownload"} {
			s, err := parseBytes(tabfiles.Zip(map[string][]byte{"readme.txt": []byte("hi"), name: inner}))
			if err != nil || s.Title != "Inner" {
				t.Errorf("%s: got %+v, %v", name, s, err)
			}
		}
	})
	t.Run("zip in zip", func(t *testing.T) {
		// A nested archive is only opened when its name looks like a tab:
		// the ".zip" extension itself is skipped, to stop recursing into archives.
		in := tabfiles.Zip(map[string][]byte{"a.tg": inner})
		s, err := parseBytes(tabfiles.Zip(map[string][]byte{"nested.gp5": in}))
		if err != nil || s.Title != "Inner" {
			t.Errorf("zip named .gp5: got %+v, %v", s, err)
		}
		_, err = parseBytes(tabfiles.Zip(map[string][]byte{"nested.zip": in}))
		if err == nil || err.Error() != "zip contains no tab file" {
			t.Errorf("zip named .zip: err = %v", err)
		}
	})
	t.Run("no tab inside", func(t *testing.T) {
		for _, files := range []map[string][]byte{{"readme.txt": []byte("x")}, {"a.png": []byte("x")}} {
			if _, err := parseBytes(tabfiles.Zip(files)); err == nil || err.Error() != "zip contains no tab file" {
				t.Errorf("%v: err = %v", slices.Collect(mapKeys(files)), err)
			}
		}
	})
	t.Run("empty archive has no local header", func(t *testing.T) {
		if _, err := parseBytes(tabfiles.Zip(nil)); err == nil || err.Error() != "unknown file format" {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("corrupt", func(t *testing.T) {
		good := tabfiles.Zip(map[string][]byte{"Content/score.gpif": gpifXML})
		for _, in := range [][]byte{
			[]byte("PK\x03\x04"),
			good[:len(good)/2],
			good[:len(good)-5],
			append([]byte("PK\x03\x04"), make([]byte, 100)...),
			tabfiles.Zip(map[string][]byte{"Content/score.gpif": []byte("<GPIF><Score>")}),
			tabfiles.Zip(map[string][]byte{"content.xml": []byte("<a")}),
			tabfiles.Zip(map[string][]byte{"x.gp5": []byte("garbage")}),
		} {
			if _, err := parseBytes(in); err == nil {
				t.Errorf("no error for %q", in[:min(len(in), 20)])
			}
		}
	})
}

func mapKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func TestParseGPIFEdgeCases(t *testing.T) {
	parse := func(x []byte) *Song {
		t.Helper()
		s, err := parseBytes(tabfiles.Zip(map[string][]byte{"Content/score.gpif": x}))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	t.Run("no tracks, no tempo", func(t *testing.T) {
		s := parse(tabfiles.GPIF("T", "A", "B", nil))
		if len(s.Tracks) != 0 || len(s.Tempos) != 0 || s.Title != "T" {
			t.Errorf("song = %+v", s)
		}
	})
	t.Run("tuning highest first is reversed", func(t *testing.T) {
		s := parse(tabfiles.GPIF("T", "A", "B", nil, tabfiles.GPIFTrack{Name: "G", Instrument: "Guitar", Pitches: "64 59 55 50 45 38"}))
		if got := s.Tracks[0]; got.Tuning != "Drop D (D A D G B E)" || !slices.Equal(got.Pitches, []int{38, 45, 50, 55, 59, 64}) {
			t.Errorf("track = %+v", got)
		}
	})
	t.Run("odd tunings", func(t *testing.T) {
		s := parse(tabfiles.GPIF("T", "A", "B", nil,
			tabfiles.GPIFTrack{Name: "8", Pitches: "30 35 40 45 50 55 59 64"},
			tabfiles.GPIFTrack{Name: "Weird", Pitches: "38 43 50 55 59 62"},
			tabfiles.GPIFTrack{Name: "None"},
		))
		if got := s.Tracks[0].Tuning; !strings.HasPrefix(got, "Custom (") {
			t.Errorf("8 strings = %q", got)
		}
		if got := s.Tracks[1].Tuning; got != "Custom (D G D G B D)" {
			t.Errorf("weird = %q", got)
		}
		if s.Tracks[2].Tuning != "" || s.Tracks[2].Pitches != nil {
			t.Errorf("no tuning = %+v", s.Tracks[2])
		}
	})
	t.Run("drum kit", func(t *testing.T) {
		s := parse(tabfiles.GPIF("T", "A", "B", nil, tabfiles.GPIFTrack{Name: "Kit", Instrument: "Drums", Kind: "drumKit", Pitches: "40 45"}))
		if got := s.Tracks[0]; !got.Drums || got.Instrument != "Drums" || got.Tuning != "" {
			t.Errorf("track = %+v", got)
		}
	})
	t.Run("tempo automations sorted, repeats dropped, junk ignored", func(t *testing.T) {
		s := parse(tabfiles.GPIF("T", "A", "B", [][2]float64{{4, 90}, {0, 120}, {2, 120}, {6, 0}, {8, 120.5}}))
		if want := []Tempo{{1, 120}, {5, 90}, {9, 120.5}}; !slices.Equal(s.Tempos, want) {
			t.Errorf("tempos = %v, want %v", s.Tempos, want)
		}
	})
}

func TestParseTG1(t *testing.T) {
	s, err := parseBytes(tabfiles.TG1("Título", "Die Äther", "Ä"))
	if err != nil || s.Title != "Título" || s.Artist != "Die Äther" || s.Album != "Ä" {
		t.Errorf("got %+v, %v", s, err)
	}
	full := tabfiles.TG1("T", "A", "B")
	for cut := len(tabfiles.TG1("", "", "")) - 1; cut < len(full)-1; cut++ {
		_, err := parseBytes(full[:cut])
		if cut < len(tgMagic) || err == nil {
			continue // too short to be sniffed as TuxGuitar, or complete
		}
		if !strings.Contains(err.Error(), "tg: truncated header") && err.Error() != "unknown file format" {
			t.Errorf("cut %d: err = %v", cut, err)
		}
	}
	// Header only: title, artist and album missing.
	var head []byte
	head = append(head, full[:2*(len(tgMagic+" - 1.5"))+1]...)
	if _, err := parseBytes(head); err == nil || err.Error() != "tg: truncated header" {
		t.Errorf("header only: err = %v", err)
	}
}

func TestParsePTB(t *testing.T) {
	album := "Rent of Summer"
	s, err := parseBytes(tabfiles.PTB(3, "Gluttonous", "Argyle Moth", &album))
	if err != nil || s.Title != "Gluttonous" || s.Artist != "Argyle Moth" || s.Album != album || s.Format != "ptb" {
		t.Errorf("got %+v, %v", s, err)
	}
	s, err = parseBytes(tabfiles.PTB(4, "T", "A", nil))
	if err != nil || s.Album != "" || s.Title != "T" {
		t.Errorf("no release: %+v, %v", s, err)
	}
	if _, err := parseBytes(tabfiles.PTB(2, "T", "A", nil)); err == nil || err.Error() != "Power Tab 1.0 files are not supported" {
		t.Errorf("version 2: err = %v", err)
	}
	bad := "\x01\x02"
	s, err = parseBytes(tabfiles.PTB(3, bad, "A", nil))
	if err != nil || s.Title != "" || s.Artist != "A" {
		t.Errorf("unprintable Title: %+v, %v", s, err)
	}
	full := tabfiles.PTB(3, "Gluttonous", "Argyle Moth", &album)
	for cut := 4; cut < len(full); cut++ {
		if s, err := parseBytes(full[:cut]); err == nil && s == nil {
			t.Errorf("cut %d: nil song without error", cut)
		}
	}
}

func TestReader(t *testing.T) {
	t.Run("need past end", func(t *testing.T) {
		r := &reader{b: []byte{1, 2, 3}}
		if r.need(4) || r.err == nil {
			t.Error("need(4) on 3 bytes succeeded")
		}
		if r.pos != 0 {
			t.Errorf("pos moved to %d", r.pos)
		}
		// The first error sticks and later reads return zero values.
		first := r.err
		if r.u8() != 0 || r.u16() != 0 || r.i32() != 0 || r.bytes(1) != nil || r.err != first {
			t.Error("reads after an error returned data or changed the error")
		}
	})
	t.Run("need exact and negative", func(t *testing.T) {
		r := &reader{b: []byte{1, 2, 3}}
		if !r.need(3) || r.err != nil {
			t.Error("need(3) on 3 bytes failed")
		}
		if r.need(-1) || r.err == nil {
			t.Error("need(-1) succeeded")
		}
	})
	t.Run("reads at end of buffer", func(t *testing.T) {
		for name, read := range map[string]func(*reader) int{
			"u8":  (*reader).u8,
			"u16": (*reader).u16,
			"i32": (*reader).i32,
		} {
			r := &reader{b: []byte{1}}
			r.pos = 1
			if got := read(r); got != 0 || r.err == nil {
				t.Errorf("%s at end = %d, err %v", name, got, r.err)
			}
		}
	})
	t.Run("little endian and sign", func(t *testing.T) {
		r := &reader{b: []byte{0x34, 0x12, 0xFF, 0xFF, 0xFF, 0xFF, 7}}
		if r.u16() != 0x1234 || r.i32() != -1 || r.u8() != 7 || r.err != nil {
			t.Errorf("reads wrong, err %v", r.err)
		}
	})
	t.Run("skip", func(t *testing.T) {
		r := &reader{b: []byte{1, 2, 3}}
		r.skip(2)
		if r.pos != 2 || r.err != nil {
			t.Errorf("pos %d err %v", r.pos, r.err)
		}
		r.skip(2)
		if r.err == nil {
			t.Error("skip past end gave no error")
		}
	})
	t.Run("string length prefixes", func(t *testing.T) {
		huge := []byte{0xFF, 0xFF, 0xFF, 0x7F} // int32 max
		neg := []byte{0xFE, 0xFF, 0xFF, 0xFF}  // -2
		minInt := []byte{0, 0, 0, 0x80}        // int32 min
		for name, f := range map[string]func(*reader) string{
			"intSizeString":     (*reader).intSizeString,
			"intByteSizeString": (*reader).intByteSizeString,
		} {
			for lname, l := range map[string][]byte{"huge": huge, "negative": neg, "min": minInt} {
				r := &reader{b: append(slices.Clone(l), "abc"...)}
				got := f(r) // must not panic
				if got != "" {
					t.Errorf("%s %s = %q, want empty", name, lname, got)
				}
				if name == "intSizeString" && r.err == nil {
					t.Errorf("%s %s: no error", name, lname)
				}
			}
		}
		// byteSizeString: length byte longer than its field is clamped.
		r := &reader{b: []byte{200, 'a', 'b', 'c'}}
		if got := r.byteSizeString(3); got != "abc" || r.err != nil {
			t.Errorf("clamped = %q, err %v", got, r.err)
		}
		// Field cut short: error, empty string, no panic.
		r = &reader{b: []byte{3, 'a'}}
		if got := r.byteSizeString(5); got != "" || r.err == nil {
			t.Errorf("short field = %q, err %v", got, r.err)
		}
		// intByteSizeString with zero length.
		r = &reader{b: []byte{0, 0, 0, 0}}
		if got := r.intByteSizeString(); got != "" || r.err != nil {
			t.Errorf("zero length = %q, err %v", got, r.err)
		}
		// Normal: int32 = size+1, length byte, field.
		r = &reader{b: []byte{4, 0, 0, 0, 2, 'h', 'i', 'x'}}
		if got := r.intByteSizeString(); got != "hi" || r.err != nil || r.pos != 8 {
			t.Errorf("normal = %q, pos %d, err %v", got, r.pos, r.err)
		}
	})
}

func TestDecodeText(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"ascii", []byte("Nectar"), "Nectar"},
		{"utf8", []byte("Die Äther"), "Die Äther"},
		{"latin1", []byte{'D', 'i', 'e', ' ', 0xC4, 't', 'h', 'e', 'r'}, "Die Äther"},
		{"latin1 ö ß", []byte{'S', 0xF6, 'h', 'n', 'e', ' ', 0xDF}, "Söhne ß"},
		{"invalid utf8 falls back to latin1", []byte{0xC4, 0x28}, "Ä("},
		{"lone continuation byte", []byte{'a', 0x80, 'b'}, "a\u0080b"},
		{"trailing nuls", []byte("abc\x00\x00"), "abc"},
		{"trimmed", []byte("  abc \t"), "abc"},
		{"empty", nil, ""},
		{"only nuls", []byte{0, 0}, ""},
	}
	for _, tt := range tests {
		if got := decodeText(tt.in); got != tt.want {
			t.Errorf("%s: decodeText(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

// Every cut of a file must give an error or a song, never a panic, and
// a cut after the header keeps what was read before it.
func TestTruncatedGP3(t *testing.T) {
	full := tabfiles.GP3(tabfiles.GP3Spec{
		Title: "Brass Kettle", Artist: "Soilbed Quartet", Album: "Glass", Tempo: 190,
		Tracks: []tabfiles.GP3Track{{Name: "Guitar", Strings: tabfiles.StdGuitar}, {Name: "Bass", Strings: []int{43, 38, 33, 28}}},
	})
	if _, err := parseBytes(full); err != nil {
		t.Fatal(err)
	}
	trackStart := len(full) - 2*(1+41+4+28+8+16)
	for cut := 1; cut < len(full); cut++ {
		s, err := parseBytes(full[:cut])
		if cut > 31 && err == nil {
			t.Errorf("cut %d of %d: no error", cut, len(full))
		}
		if cut >= trackStart && (s == nil || s.Title != "Brass Kettle" || s.Artist != "Soilbed Quartet" || len(s.Tempos) != 1) {
			t.Errorf("cut %d: lost header data: %+v", cut, s)
		}
	}
}

func TestTruncatedFormats(t *testing.T) {
	album := "Alb"
	gpifXML := tabfiles.GPIF("T", "A", "B", [][2]float64{{0, 120}}, tabfiles.GPIFTrack{Name: "G", Pitches: "40 45 50 55 59 64"})
	inputs := map[string][]byte{
		"gp3": tabfiles.GP3(tabfiles.GP3Spec{Title: "T", Artist: "A", Tempo: 120, Tracks: []tabfiles.GP3Track{{Name: "G", Strings: tabfiles.StdGuitar}}}),
		"tg":  tabfiles.TG1("T", "A", "B"),
		"ptb": tabfiles.PTB(3, "T", "A", &album),
		"gp7": tabfiles.Zip(map[string][]byte{"Content/score.gpif": gpifXML}),
		"tg2": tabfiles.Zip(map[string][]byte{"content.xml": tabfiles.TG2("S", "B", "D", []int{100}, nil, tabfiles.TG2Track{Name: "x", Strings: []int{64}})}),
	}
	for name, full := range inputs {
		for i := 0; i <= 10; i++ {
			cut := len(full) * i / 11
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s cut %d: panic %v", name, cut, r)
					}
				}()
				parseBytes(full[:cut])
			}()
		}
	}
}

func FuzzParseBytes(f *testing.F) {
	album := "Alb"
	f.Add(tabfiles.GP3(tabfiles.GP3Spec{Title: "T", Artist: "A", Tempo: 120, Tracks: []tabfiles.GP3Track{{Name: "G", Strings: tabfiles.StdGuitar}}}))
	f.Add(tabfiles.TG1("T", "A", "B"))
	f.Add(tabfiles.PTB(3, "T", "A", &album))
	f.Add(tabfiles.Zip(map[string][]byte{"Content/score.gpif": tabfiles.GPIF("T", "A", "B", [][2]float64{{0, 120}}, tabfiles.GPIFTrack{Name: "G", Pitches: "40 45 50 55 59 64"})}))
	f.Add(tabfiles.Zip(map[string][]byte{"content.xml": tabfiles.TG2("S", "B", "D", []int{100}, []tabfiles.TG2Channel{{ID: 1, Bank: 0, Program: 1}}, tabfiles.TG2Track{Name: "x", Channel: 1, Strings: []int{64}})}))
	for _, fx := range fixtures() {
		f.Add(fx.data)
	}
	f.Add([]byte("BCFZ\x10\x00\x00\x00abcdefgh"))
	f.Add([]byte("BCFS"))
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte("ptab\x03\x00\x00\x00"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			b = b[:1<<20]
		}
		start := time.Now()
		parseBytes(b)
		if d := time.Since(start); d > time.Second {
			t.Errorf("took %v for %d bytes", d, len(b))
		}
	})
}
