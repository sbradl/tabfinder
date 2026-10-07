package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"tab-sync/internal/tab"
)

func TestKey(t *testing.T) {
	for in, want := range map[string]string{
		"":                        "",
		"The Tan Album":           "tanalbum",
		"the tan album":           "tanalbum",
		"Die Äther":               "äther",
		"Theatre of Rain":         "theatreofrain", // "The" needs a space after it
		"The":                     "the",
		"  The  ":                 "the",
		"2008 - Rent of Summer":   "rentofsummer",
		"Rent of Summer (2004)":   "rentofsummer",
		"Rent of Summer [2004]":   "rentofsummer",
		"Rent of Summer (Deluxe)": "rentofsummerdeluxe", // only years are dropped
		"Rock & Roll!":            "rockroll",
		"1984":                    "1984",
		"Cannons N' Petals":       "cannonsnpetals",
		"QR/ST":                   "qrst",
		"Ünïcödé":                 "ünïcödé",
		"The Die Gently":          "diegently", // one article only
	} {
		if got := key(in); got != want {
			t.Errorf("key(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanAlbumMore(t *testing.T) {
	for in, want := range map[string]string{
		"":                               "",
		"Rent of Summer":                 "Rent of Summer",
		"  Spaced   Out ":                "Spaced Out",
		"2008 - Rent of Summer (Deluxe)": "Rent of Summer",
		"Rent of Summer (2004)":          "Rent of Summer",
		"Album...":                       "Album",
		"QR/ST Live":                     "QR-ST Live",
		"Single":                         "",
		"single":                         "",
		"EP":                             "",
		"S/T":                            "",
		"s/t":                            "",
		"Self-Titled":                    "",
		"self titled":                    "",
		"Unknown":                        "",
		"Unbekannt":                      "",
		"Untitled":                       "",
		"-":                              "",
		"???":                            "",
		"(Single - 2021)":                "",
		"(anything at all)":              "",
		"Rhythm Star World Tour":         "",
		"Tabbed by Someone":              "",
		"Not Released Yet":               "",
		"dreizehn":                       "13",
		"BEAST":                          "Beast",
		"Ep Micro":                       "Micro",
		"Weaver(Teal)":                   "Weaver (Teal Album)",
		"Helsinki Syndrom":               "Helsinki Syndrome",
		"Runter mit den Pantoffeln Unsichtbarer!": "Runter mit den Pantoffeln, Unsichtbarer!",
	} {
		if got := cleanAlbum(in); got != want {
			t.Errorf("cleanAlbum(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSimilar(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want bool
	}{
		{"", "", true},
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"dawn", "down", false}, // short keys must be equal
		{"abcde", "abcdf", false},
		{"prayforvillains", "prayforvillians", true},
		{"moonbreeder", "moonbreder", true},
		{"moonbreeder", "followtheweaver", false},
		{"", "abcdefgh", false},
		{"abcdefgh", "", false},
		{"abcdef", "abcdeg", false}, // 5 of 6 = 0.83
		{"abcdefghij", "abcdefghix", true},
		{"ätherzzzzz", "ätherzzzzy", true}, // counted in runes, not bytes
	} {
		if got := similar(tt.a, tt.b); got != tt.want {
			t.Errorf("similar(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
		if got := similar(tt.b, tt.a); got != tt.want {
			t.Errorf("similar(%q, %q) = %v, want %v (not symmetric)", tt.b, tt.a, got, tt.want)
		}
	}
}

func TestNicestMore(t *testing.T) {
	for _, tt := range []struct {
		in   []string
		want string
	}{
		{[]string{"only"}, "only"},
		{[]string{"foo", "foo", "Foo"}, "foo"}, // most frequent first
		{[]string{"a b", "A B"}, "A B"},        // then most capitalized words
		{[]string{"Abcd", "Abc"}, "Abc"},       // then shortest
		{[]string{"Abc", "Xyz"}, "Abc"},        // then the first
		{[]string{"Xyz", "Abc"}, "Xyz"},
		{[]string{"äther", "Äther"}, "Äther"},
		{[]string{"Ab", "Ab", "Cd", "Cd", "Ef"}, "Ab"},
		{[]string{"x y z", "X y z", "X Y z"}, "X Y z"},
	} {
		if got := nicest(tt.in); got != tt.want {
			t.Errorf("nicest(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFilenameKey(t *testing.T) {
	for _, tt := range []struct {
		path, artist, want string
	}{
		{"A/Argyle Moth - Nectar (ver 2).gp5", "Argyle Moth", "nectar"},
		{"Argyle Moth/argyle_moth_nectar.gp5", "Argyle Moth", "nectar"},
		{"Slate Ravens/slate_ravens_the_archivist.gp5", "Slate Ravens", "archivist"}, // article after the artist
		{"Slate Ravens/Slate Ravens - Die Gently.gp5", "Slate Ravens", "gently"},     // key() drops a leading article, "Die" included
		{"X/Song_drop_c.gp5", "", "song"},
		{"X/Song 7string.gp5", "", "song"},
		{"X/Song_withbass.gp5", "", "song"},
		{"X/Song (live).gp5", "", "song"},
		{"X/Song v3.gp5", "", "song"},
		{"X/Song - 779.gp5", "", "song"},
		{"X/Song.gpx.crdownload", "", "song"},
		{"X/Song.gp5.zip", "", "song"},
		{"X/www-tablatures-tk @ Song.gp4", "", "song"},
		{"Song.gp5", "", "song"},
		{"X/The Song.gp5", "", "song"},
		{"X/Song Of Songs.gp5", "", "songofsongs"},
		{"Nectar/Nectar.gp5", "Nectar", "nectar"}, // the key is never cut down to nothing
	} {
		if got := filenameKey(&tab.Song{Path: tt.path, Artist: tt.artist}); got != tt.want {
			t.Errorf("filenameKey(%q, artist %q) = %q, want %q", tt.path, tt.artist, got, tt.want)
		}
	}
}

func TestSongKey(t *testing.T) {
	for _, tt := range []struct {
		name                        string
		path, title, titleSource, a string
		want                        string
	}{
		{"title from the path: the file name", "X/nectar_ver2.gp5", "Nectar Ver2", "path", "", "nectar"},
		{"title agrees: parentheses dropped", "X/nectar.gp5", "Nectar (live)", "file", "", "nectar"},
		{"title contradicts the file name", "Eisenmond/Eisenmond - Heimatland.gp5", "Eisenmond", "file", "Eisenmond", "heimatland"},
		{"empty title", "X/nectar.gp5", "", "file", "", "nectar"},
		{"title only a parenthesis", "X/nectar.gp5", "(Part 2)", "file", "", "nectar"},
		{"same title, different spelling", "X/Nectar.gp5", "NECTAR!", "file", "", "nectar"},
		{"title longer than the file name", "X/nec.gp5", "Nectar", "file", "", "nectar"},
	} {
		s := &tab.Song{Path: tt.path, Title: tt.title, TitleSource: tt.titleSource, Artist: tt.a}
		if got := songKey(s); got != tt.want {
			t.Errorf("%s: songKey = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSongNameMore(t *testing.T) {
	for _, tt := range []struct {
		name                                   string
		path, artist, title, titleSource, want string
	}{
		{"unsafe characters", "A/B/qr_st_live_now.gp5", "A", "QR/ST: Live? *Now* <1> |2|", "file", "QR-ST - Live Now 1 2"},
		{"title contradicts the file name: the file name wins", "A/B/x.gp5", "A", "Something Else", "file", "X"},
		{"slash and colon", "A/B/qr_st_live.gp5", "A", "QR/ST Live: Now", "file", "QR-ST Live - Now"},
		{"quotes dropped", "A/B/x_y.gp5", "A", `X "Y"`, "file", "X Y"},
		{"all lower case gets title case", "A/B/my song.gp5", "A", "my song", "file", "My Song"},
		{"trailing dots and spaces", "A/B/nectar.gp5", "A", "Nectar ...", "file", "Nectar"},
		{"version tag in the title", "A/B/nectar.gp5", "A", "Nectar (ver 3)", "file", "Nectar"},
		{"junk bracket in the title", "A/B/nectar.gp5", "A", "Nectar [by me@example.com]", "file", "Nectar"},
		{"artist in the title", "A/B/nectar.gp5", "A", "A - Nectar", "file", "Nectar"},
		{"another artist in the title stays", "A/B/b_nectar.gp5", "A", "B - Nectar", "file", "B - Nectar"},
		{"tuning suffix", "A/B/nectar_drop_c.gp5", "A", "", "path", "Nectar"},
		{"7string", "A/B/nectar_7string.gp5", "A", "", "path", "Nectar"},
		{"leading apostrophe trimmed", "A/B/'nectar'.gp5", "A", "", "path", "Nectar"},
		{"junk title", "A/B/nectar.gp5", "A", "Unknown", "file", "Nectar"},
		{"umlauts", "Die Äther/x/ruf_nach_sonne.gp4", "Die Äther", "Ruf nach Sonne", "file", "Ruf nach Sonne"},
	} {
		s := &tab.Song{Path: tt.path, Artist: tt.artist, Title: tt.title, TitleSource: tt.titleSource}
		if got := songName(s); got != tt.want {
			t.Errorf("%s: songName(%q) = %q, want %q", tt.name, tt.path, got, tt.want)
		}
	}
}

func TestTitleCase(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"hello world":           "Hello World",
		"  multiple   spaces  ": "Multiple Spaces",
		"äther öl":              "Äther Öl",
		"ALREADY upper":         "ALREADY Upper",
		"x":                     "X",
	} {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTabExtMore(t *testing.T) {
	for in, want := range map[string]string{
		"a.gp5":               ".gp5",
		"a.GP5":               ".GP5",
		"x.gp3.zip":           ".gp3.zip",
		"x.GP3.ZIP":           ".GP3.ZIP",
		"x.gpx.crdownload":    ".gpx.crdownload",
		"x.zip":               ".zip",
		"x.crdownload":        ".crdownload",
		"x.txt.zip":           ".zip",
		"x.pdf.crdownload":    ".crdownload",
		"x.zip.zip":           ".zip.zip",
		"noext":               "",
		"a.b.gp5":             ".gp5",
		"Mr. X - Song.gp5":    ".gp5",
		"Mr. X - Song.gp.zip": ".gp.zip",
	} {
		if got := tabExt(in); got != want {
			t.Errorf("tabExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	dir := t.TempDir() // the shell runs here, so a quoting bug can only touch this folder
	cases := []string{
		"plain", "with space", "it's", "'", "''", `a"b`, "$HOME", "`id`", "$(id)", "a;b", "a&b", "a|b", "a\nb", "a\tb", "a\\b", "*", "?", "~", "!", "#x",
		"Die Äther/Polka ist anders", "emoji 🔥", "", "a'b'c", `'; touch PWNED; '`, "-n", "--", "%s", "\\n",
	}
	for _, in := range cases {
		q := shellQuote(in)
		if q[0] != '\'' || q[len(q)-1] != '\'' {
			t.Errorf("shellQuote(%q) = %s: not quoted", in, q)
		}
		// The shell must read it back as exactly the input.
		cmd := exec.Command("sh", "-c", "printf '%s' "+q)
		cmd.Dir = dir
		out, err := cmd.Output()
		if _, statErr := os.Stat(filepath.Join(dir, "PWNED")); statErr == nil {
			t.Fatalf("shellQuote(%q) = %s let the shell run the input", in, q)
		}
		if err != nil || string(out) != in {
			t.Errorf("sh read shellQuote(%q) = %s back as %q (%v)", in, q, out, err)
		}
	}
}

func TestSplitList(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{",", nil},
		{" , , ", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
		{"a,b,", []string{"a", "b"}},
		{",a,,b", []string{"a", "b"}},
		{"a b,c d", []string{"a b", "c d"}},
		{"Fit for a Lighthouse/Chart Happens, Rock&Pop", []string{"Fit for a Lighthouse/Chart Happens", "Rock&Pop"}},
	} {
		if got := splitList(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("splitList(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
