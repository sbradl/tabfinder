package tab

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

func parseFile(p string) (*Song, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return parseBytes(b)
}

// parseBytes sniffs the format from the content, not the extension:
// renamed files (.gp3 that is really GP5, .crdownload, .zip wrappers) are common.
func parseBytes(b []byte) (*Song, error) {
	switch {
	case len(b) == 0:
		return nil, errors.New("empty file")
	case bytes.HasPrefix(b, []byte("PK\x03\x04")):
		return parseZip(b)
	case bytes.HasPrefix(b, []byte("BCFZ")), bytes.HasPrefix(b, []byte("BCFS")):
		return parseGPX(b)
	case bytes.HasPrefix(b, []byte("ptab")):
		return parsePTB(b)
	case len(b) > 31 && bytes.Contains(b[1:31], []byte("GUITAR")):
		return parseGP(b)
	case isTuxGuitar(b):
		return parseTG(b)
	}
	return nil, errors.New("unknown file format")
}

func parseZip(b []byte) (*Song, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name == "Content/score.gpif" {
			data, err := readZipFile(f)
			if err != nil {
				return nil, err
			}
			return parseGPIF(data, FormatGP7)
		}
	}
	for _, f := range zr.File {
		if f.Name == "content.xml" {
			data, err := readZipFile(f)
			if err != nil {
				return nil, err
			}
			return parseTG2(data)
		}
	}
	// Plain zip archive wrapping a single tab (e.g. downloads from tab sites).
	for _, f := range zr.File {
		if ext := strings.ToLower(path.Ext(f.Name)); tabExts[ext] && ext != ".zip" {
			data, err := readZipFile(f)
			if err != nil {
				return nil, err
			}
			return parseBytes(data)
		}
	}
	return nil, errors.New("zip contains no tab file")
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if n := f.UncompressedSize64; n > 0 && n <= maxGPXSize { // read in one go when the size is plausible
		buf := bytes.NewBuffer(make([]byte, 0, n+1))
		_, err := buf.ReadFrom(rc)
		return buf.Bytes(), err
	}
	return io.ReadAll(rc)
}

// reader is a little-endian cursor that records the first out-of-bounds read
// instead of panicking; callers check r.err once after a block of reads.
type reader struct {
	b   []byte
	pos int
	err error
}

func (r *reader) need(n int) bool {
	if r.err != nil {
		return false
	}
	if n < 0 || r.pos+n > len(r.b) {
		r.err = fmt.Errorf("truncated or corrupt data at offset %d", r.pos)
		return false
	}
	return true
}

func (r *reader) skip(n int) {
	if r.need(n) {
		r.pos += n
	}
}

func (r *reader) u8() int {
	if !r.need(1) {
		return 0
	}
	r.pos++
	return int(r.b[r.pos-1])
}

func (r *reader) u16() int {
	if !r.need(2) {
		return 0
	}
	r.pos += 2
	return int(binary.LittleEndian.Uint16(r.b[r.pos-2:]))
}

func (r *reader) i32() int {
	if !r.need(4) {
		return 0
	}
	r.pos += 4
	return int(int32(binary.LittleEndian.Uint32(r.b[r.pos-4:])))
}

func (r *reader) bytes(n int) []byte {
	if !r.need(n) {
		return nil
	}
	r.pos += n
	return r.b[r.pos-n : r.pos]
}

// byteSizeString: 1 length byte followed by a fixed-size field of size bytes.
func (r *reader) byteSizeString(size int) string {
	n := r.u8()
	field := r.bytes(size)
	return decodeText(field[:min(n, len(field))])
}

// intByteSizeString: int32 (field size + 1), length byte, field.
func (r *reader) intByteSizeString() string {
	n := r.i32()
	if n <= 0 {
		return ""
	}
	return r.byteSizeString(n - 1)
}

// intSizeString: int32 length followed by that many bytes.
func (r *reader) intSizeString() string {
	return decodeText(r.bytes(r.i32()))
}

// decodeText handles the mix of UTF-8 and Windows-1252/Latin-1 found in tab files.
func decodeText(b []byte) string {
	b = bytes.TrimRight(b, "\x00")
	if utf8.Valid(b) {
		return strings.TrimSpace(string(b))
	}
	rs := make([]rune, len(b))
	for i, c := range b {
		rs[i] = rune(c)
	}
	return strings.TrimSpace(string(rs))
}
