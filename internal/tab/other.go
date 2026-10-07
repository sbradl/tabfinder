package tab

import (
	"encoding/binary"
	"encoding/xml"
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"
)

// TuxGuitar 1.x (.tg) and Power Tab (.ptb): only the song header is read;
// their track/tuning data sits behind the full note data.

const tgMagic = "TuxGuitar File Format"

func tgString(b []byte, pos *int) (string, bool) {
	if *pos >= len(b) {
		return "", false
	}
	n := int(b[*pos])
	*pos++
	if *pos+2*n > len(b) {
		return "", false
	}
	u := make([]uint16, n)
	for i := range u {
		u[i] = binary.BigEndian.Uint16(b[*pos+2*i:])
	}
	*pos += 2 * n
	return strings.TrimSpace(string(utf16.Decode(u))), true
}

func isTuxGuitar(b []byte) bool {
	pos := 0
	s, ok := tgString(b, &pos)
	return ok && strings.HasPrefix(s, tgMagic)
}

func parseTG(b []byte) (*Song, error) {
	pos := 0
	tgString(b, &pos) // format header
	s := &Song{Format: "tg"}
	var ok1, ok2, ok3 bool
	s.Title, ok1 = tgString(b, &pos)
	s.Artist, ok2 = tgString(b, &pos)
	s.Album, ok3 = tgString(b, &pos)
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("tg: truncated header")
	}
	return s, nil
}

func parsePTB(b []byte) (*Song, error) {
	r := &reader{b: b, pos: 4}
	if version := r.u16(); version < 3 {
		return nil, errors.New("Power Tab 1.0 files are not supported")
	}
	s := &Song{Format: "ptb"}
	if fileType := r.u16(); fileType != 0 {
		return s, nil // lesson file, no song header
	}
	r.skip(1) // content type
	s.Title = mfcString(r)
	s.Artist = mfcString(r)
	if releaseType := r.u8(); releaseType == 0 { // public audio release
		r.skip(1) // release type
		s.Album = mfcString(r)
	}
	if r.err != nil {
		return nil, r.err
	}
	for _, f := range []*string{&s.Title, &s.Artist, &s.Album} {
		if strings.IndexFunc(*f, func(c rune) bool { return !unicode.IsPrint(c) }) >= 0 {
			*f = ""
		}
	}
	return s, nil
}

// mfcString reads an MFC CString: u8 length, escalating to u16/u32 via 0xFF markers.
func mfcString(r *reader) string {
	n := r.u8()
	if n == 0xFF {
		n = r.u16()
		if n == 0xFFFE {
			r.err = errors.New("ptb: unicode strings not supported")
			return ""
		}
		if n == 0xFFFF {
			n = r.i32()
		}
	}
	return decodeText(r.bytes(n))
}

type tg2Doc struct {
	Song struct {
		Name     string `xml:"name"`
		Artist   string `xml:"artist"`
		Album    string `xml:"album"`
		Channels []struct {
			ID      int `xml:"id"`
			Bank    int `xml:"bank"`
			Program int `xml:"program"`
		} `xml:"TGChannel"`
		MeasureHeaders []struct {
			Tempo int `xml:"tempo"`
		} `xml:"TGMeasureHeader"`
		Tracks []struct {
			Name      string `xml:"name"`
			ChannelID int    `xml:"channelId"`
			Strings   []int  `xml:"TGString"`
		} `xml:"TGTrack"`
	} `xml:"TGSong"`
}

// parseTG2 reads TuxGuitar 2 files: a zip with version.txt and content.xml.
func parseTG2(data []byte) (*Song, error) {
	var doc tg2Doc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	s := &Song{
		Format: "tg",
		Title:  strings.TrimSpace(doc.Song.Name),
		Artist: strings.TrimSpace(doc.Song.Artist),
		Album:  strings.TrimSpace(doc.Song.Album),
	}
	for i, mh := range doc.Song.MeasureHeaders {
		if mh.Tempo > 0 {
			s.addTempo(i+1, float64(mh.Tempo))
		}
	}
	for _, tt := range doc.Song.Tracks {
		t := Track{Name: strings.TrimSpace(tt.Name)}
		for _, c := range doc.Song.Channels {
			if c.ID == tt.ChannelID {
				t.Drums = c.Bank == 128 // TuxGuitar's percussion bank
				t.Instrument = gmInstrument(c.Program)
			}
		}
		if t.Drums {
			t.Instrument = "Drums"
		} else {
			t.Pitches = slices.Clone(tt.Strings) // stored highest string first
			slices.Reverse(t.Pitches)
			t.Tuning = tuningName(t.Pitches)
		}
		s.Tracks = append(s.Tracks, t)
	}
	return s, nil
}
