package tuxguitar

import (
	"strings"
	"testing"

	"tabfinder/internal/tab"
)

func TestFileName(t *testing.T) {
	for _, tt := range []struct {
		song    tab.Song
		name    string
		inPlace bool
	}{
		{tab.Song{Path: "a/Brass Kettle.gp5", Format: "gp5", Title: "Brass Kettle"}, "Brass Kettle.gp5", true},
		{tab.Song{Path: "a/x.gp3", Format: "gp5", Title: "Brass Kettle"}, "Brass Kettle.gp5", false},
		{tab.Song{Path: "a/x.gpx.crdownload", Format: "gp6", Title: "A.B.C (live)"}, "ABC live.gpx", false},
		{tab.Song{Path: "a/x.zip", Format: "gp7", Title: "?!"}, "song.gp", false},
	} {
		if got := FileName(&tt.song); got != tt.name {
			t.Errorf("FileName(%s) = %q, want %q", tt.song.Path, got, tt.name)
		}
		if got := OpensInPlace(&tt.song); got != tt.inPlace {
			t.Errorf("OpensInPlace(%s) = %v", tt.song.Path, got)
		}
	}
}

func TestFileNameMore(t *testing.T) {
	for _, tt := range []struct {
		name    string
		song    tab.Song
		want    string
		inPlace bool
	}{
		{"gp3", tab.Song{Path: "a/x.gp3", Format: "gp3", Title: "T"}, "T.gp3", true},
		{"gp4", tab.Song{Path: "a/x.gp4", Format: "gp4", Title: "T"}, "T.gp4", true},
		{"gp5", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "T"}, "T.gp5", true},
		{"gp6 is gpx", tab.Song{Path: "a/x.gpx", Format: "gp6", Title: "T"}, "T.gpx", true},
		{"gp7 is gp", tab.Song{Path: "a/x.gp", Format: "gp7", Title: "T"}, "T.gp", true},
		{"tg", tab.Song{Path: "a/x.tg", Format: "tg", Title: "T"}, "T.tg", true},
		{"ptb", tab.Song{Path: "a/x.ptb", Format: "ptb", Title: "T"}, "T.ptb", true},
		{"gp5 named gp3", tab.Song{Path: "a/x.gp3", Format: "gp5", Title: "T"}, "T.gp5", false},
		{"gp6 named gpx.crdownload", tab.Song{Path: "a/x.gpx.crdownload", Format: "gp6", Title: "T"}, "T.gpx", false},
		{"gp7 named gp.zip", tab.Song{Path: "a/x.gp.zip", Format: "gp7", Title: "T"}, "T.gp", false},
		{"upper case extension", tab.Song{Path: "a/x.GP5", Format: "gp5", Title: "T"}, "T.gp5", true},
		{"mixed case extension", tab.Song{Path: "a/x.Gpx", Format: "gp6", Title: "T"}, "T.gpx", true},
		{"unknown format uses the path's extension, lower-cased", tab.Song{Path: "a/x.GP5", Title: "T"}, "T.gp5", true},
		{"unknown format, no extension", tab.Song{Path: "a/x", Title: "T"}, "T.", false},
		{"dots in title", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "Mr. A.B. C"}, "Mr AB C.gp5", true},
		{"slashes in title", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "QR/ST"}, "QRST.gp5", true},
		{"emoji dropped", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "Fire 🔥 Song"}, "Fire  Song.gp5", true},
		{"only symbols", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "?!*"}, "song.gp5", true},
		{"only emoji", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "🔥"}, "song.gp5", true},
		{"empty title", tab.Song{Path: "a/x.gp5", Format: "gp5"}, "song.gp5", true},
		{"umlauts and underscores kept", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "Ruf_nach Sonne Äther-ß"}, "Ruf_nach Sonne Äther-ß.gp5", true},
		{"spaces trimmed", tab.Song{Path: "a/x.gp5", Format: "gp5", Title: "  T  "}, "T.gp5", true},
		{"crdownload in place only if the format says so", tab.Song{Path: "a/x.crdownload", Format: "", Title: "T"}, "T.crdownload", true},
	} {
		if got := FileName(&tt.song); got != tt.want {
			t.Errorf("%s: FileName = %q, want %q", tt.name, got, tt.want)
		}
		if got := OpensInPlace(&tt.song); got != tt.inPlace {
			t.Errorf("%s: OpensInPlace = %v, want %v", tt.name, got, tt.inPlace)
		}
	}
	if !strings.HasSuffix(FileName(&tab.Song{Title: "x", Format: "gp7"}), ".gp") {
		t.Error("gp7 name does not end in .gp")
	}
}
