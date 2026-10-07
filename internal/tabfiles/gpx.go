package tabfiles

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
	"strings"
)

// BCFSFile is a file inside a Guitar Pro 6 container.
type BCFSFile struct {
	Name string
	Data []byte
}

const bcfsSector = 0x1000

// BCFS builds the uncompressed Guitar Pro 6 container ("BCFS" and 4 KiB sectors):
// sector 0 is a header, then one sector per file entry (int32 2, name, size and the
// numbers of the data sectors), then the files' data sectors.
func BCFS(files ...BCFSFile) []byte {
	var entries, data bytes.Buffer
	first := 1 + len(files) // sector number of the first data sector
	next := first
	for _, f := range files {
		entry := make([]byte, bcfsSector)
		binary.LittleEndian.PutUint32(entry, 2)
		copy(entry[4:4+127], f.Name)
		binary.LittleEndian.PutUint32(entry[0x8C:], uint32(len(f.Data)))
		sectors := (len(f.Data) + bcfsSector - 1) / bcfsSector
		if 0x94+4*sectors > bcfsSector {
			panic("tabfiles.BCFS: file too big")
		}
		for i := range sectors {
			binary.LittleEndian.PutUint32(entry[0x94+4*i:], uint32(next+i))
		}
		next += sectors
		entries.Write(entry)
		data.Write(f.Data)
		data.Write(make([]byte, sectors*bcfsSector-len(f.Data)))
	}
	// Data of a file that starts with the int32 2 would pass for an entry.
	for off := 0; off < data.Len(); off += bcfsSector {
		if binary.LittleEndian.Uint32(data.Bytes()[off:]) == 2 {
			panic("tabfiles.BCFS: data sector looks like a file entry")
		}
	}
	var out bytes.Buffer
	out.WriteString("BCFS")
	out.Write(make([]byte, bcfsSector)) // sector 0
	out.Write(entries.Bytes())
	out.Write(data.Bytes())
	return out.Bytes()
}

// GPX builds a Guitar Pro 6 file around a score.gpif: the container compressed with
// BCFZ, which is what GP6 writes. With compress false the plain "BCFS" container is
// written instead (GP6 uses that one for scores it couldn't shrink).
func GPX(gpif []byte, compress bool) []byte {
	container := BCFS(
		BCFSFile{"misc.xml", []byte(`<?xml version="1.0"?><Misc><Version>6.1.4</Version></Misc>`)},
		BCFSFile{"score.gpif", gpif},
	)
	if !compress {
		return container
	}
	var out bytes.Buffer
	out.WriteString("BCFZ")
	binary.Write(&out, binary.LittleEndian, uint32(len(container)))
	out.Write(bcfzCompress(container))
	return out.Bytes()
}

type bitWriter struct {
	buf  []byte
	nbit int
}

func (w *bitWriter) bit(v int) {
	if w.nbit%8 == 0 {
		w.buf = append(w.buf, 0)
	}
	if v != 0 {
		w.buf[len(w.buf)-1] |= 1 << (7 - w.nbit%8)
	}
	w.nbit++
}

// bits writes the n low bits of v, most significant first.
func (w *bitWriter) bits(v, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bit(v >> i & 1)
	}
}

// bitsReversed writes the n low bits of v, least significant first.
func (w *bitWriter) bitsReversed(v, n int) {
	for i := range n {
		w.bit(v >> i & 1)
	}
}

// bcfzCompress is the LZ77 variant of BCFZ: a flag bit, then either 0 + up to 3
// literal bytes, or 1 + the width of the numbers + offset + length of a copy.
func bcfzCompress(in []byte) []byte {
	const window, minMatch, maxLen = 4095, 5, 2000
	w := &bitWriter{}
	var lit []byte
	flush := func() {
		for len(lit) > 0 {
			n := min(len(lit), 3)
			w.bit(0)
			w.bitsReversed(n, 2)
			for _, c := range lit[:n] {
				w.bits(int(c), 8)
			}
			lit = lit[n:]
		}
	}
	for pos := 0; pos < len(in); {
		bestLen, bestOff := 0, 0
		for off := 1; off <= min(pos, window); off++ {
			n := 0
			// The decoder copies at most `off` bytes per reference.
			for n < off && n < maxLen && pos+n < len(in) && in[pos+n-off] == in[pos+n] {
				n++
			}
			if n > bestLen {
				bestLen, bestOff = n, off
			}
			if bestLen == maxLen {
				break
			}
		}
		if bestLen < minMatch {
			lit = append(lit, in[pos])
			pos++
			continue
		}
		flush()
		width := bits.Len(uint(max(bestOff, bestLen)))
		w.bit(1)
		w.bits(width, 4)
		w.bitsReversed(bestOff, width)
		w.bitsReversed(bestLen, width)
		pos += bestLen
	}
	flush()
	return w.buf
}

// GPIF6Track is a track of a Guitar Pro 6 score.
type GPIF6Track struct {
	Name       string
	Instrument string // the instrument reference, e.g. "e-gtr-clean"
	Program    int    // General MIDI program
	Channel    int    // primary MIDI channel, 0-based; 9 is the drum channel
	Percussion bool
	Pitches    string // space-separated MIDI notes, lowest string first
}

// GPIF6 builds a score.gpif the way Guitar Pro 6 writes it; tempos are {bar (0-based), bpm} pairs.
func GPIF6(title, artist, album string, tempos [][2]float64, tracks ...GPIF6Track) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?><GPIF><GPVersion>6.1.4</GPVersion><Score><Title><![CDATA[%s]]></Title><Artist><![CDATA[%s]]></Artist><Album><![CDATA[%s]]></Album></Score><MasterTrack><Automations>`, title, artist, album)
	for _, t := range tempos {
		fmt.Fprintf(&b, `<Automation><Type>Tempo</Type><Linear>false</Linear><Bar>%d</Bar><Position>0</Position><Visible>true</Visible><Value>%g 2</Value></Automation>`, int(t[0]), t[1])
	}
	b.WriteString(`</Automations></MasterTrack><Tracks>`)
	for i, t := range tracks {
		table := "GeneralMidi"
		if t.Percussion {
			table = "Percussion"
		}
		fmt.Fprintf(&b, `<Track id="%d"><Name><![CDATA[%s]]></Name><Instrument ref="%s"/><GeneralMidi table="%s"><Program>%d</Program><PrimaryChannel>%d</PrimaryChannel><SecondaryChannel>%d</SecondaryChannel></GeneralMidi>`,
			i, t.Name, t.Instrument, table, t.Program, t.Channel, t.Channel+1)
		if t.Pitches != "" {
			fmt.Fprintf(&b, `<Properties><Property name="Tuning"><Pitches>%s</Pitches><Flat/></Property></Properties>`, t.Pitches)
		}
		b.WriteString(`</Track>`)
	}
	b.WriteString(`</Tracks><MasterBars/><Bars/></GPIF>`)
	return []byte(b.String())
}
