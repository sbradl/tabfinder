package tab

import (
	"bytes"
	"errors"
)

// A small XML reader for the note data of a score.gpif. encoding/xml is too slow for it:
// a Guitar Pro 7 file has hundreds of kilobytes of notes, and reading them with it took
// longer than reading the rest of a library. GPIF is plain XML (no namespaces, no DTD),
// written by Guitar Pro, so a reader of elements, attributes, text and CDATA will do.

// xnode is an element: its name, raw attributes, the text directly in it and its children.
// All are slices of the document.
type xnode struct {
	name, attrs, text []byte
	kids              []xnode
}

// kid is the first child of that name; the zero xnode if there's none.
func (n *xnode) kid(name string) *xnode {
	for i := range n.kids {
		if string(n.kids[i].name) == name {
			return &n.kids[i]
		}
	}
	return &xnode{}
}

func (n *xnode) has(name string) bool { return n.kid(name).name != nil }

// textOf is the trimmed text of the first child of that name.
func (n *xnode) textOf(name string) string { return string(bytes.TrimSpace(n.kid(name).text)) }

// intOf is the text of the first child of that name as a number, -1 if missing or not one.
func (n *xnode) intOf(name string) int { return natoi(bytes.TrimSpace(n.kid(name).text)) }

// natoi is a non-negative decimal number, -1 for anything else.
func natoi[T string | []byte](s T) int {
	if len(s) == 0 || len(s) > 9 {
		return -1
	}
	v := 0
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return -1
		}
		v = v*10 + int(c-'0')
	}
	return v
}

// attr is an attribute's raw value, "" if missing.
func (n *xnode) attr(name string) string {
	a := n.attrs
	for {
		i := bytes.Index(a, []byte(name+`="`))
		if i < 0 {
			return ""
		}
		if i == 0 || a[i-1] == ' ' || a[i-1] == '\t' || a[i-1] == '\n' || a[i-1] == '\r' {
			v := a[i+len(name)+2:]
			if j := bytes.IndexByte(v, '"'); j >= 0 {
				return string(v[:j])
			}
			return ""
		}
		a = a[i+1:]
	}
}

func (n *xnode) intAttr(name string) int { return natoi(n.attr(name)) }

// xreader reads elements from a document.
type xreader struct {
	b   []byte
	pos int
	// The children of the elements being read are collected on stack, then moved to arena,
	// which holds the kids of the element given to a children callback and is reused after.
	stack, arena []xnode
}

var errXML = errors.New("gpif: malformed XML")

// tag is the next markup: a start tag (end false), an end tag, or a self-closing start tag.
// Text before it, with CDATA unwrapped, is added to text.
func (x *xreader) tag(text *[]byte) (name, attrs []byte, end, selfClosing bool, err error) {
	for {
		i := bytes.IndexByte(x.b[x.pos:], '<')
		if i < 0 {
			return nil, nil, false, false, errXML
		}
		if s := x.b[x.pos : x.pos+i]; text != nil && len(bytes.TrimSpace(s)) > 0 { // not the indentation between tags
			if len(*text) == 0 && bytes.IndexByte(s, '&') < 0 {
				*text = s // as it is in the document: no copy
			} else {
				*text = appendText((*text)[:len(*text):len(*text)], s)
			}
		}
		x.pos += i
		rest := x.b[x.pos:]
		switch {
		case bytes.HasPrefix(rest, []byte("<![CDATA[")):
			j := bytes.Index(rest, []byte("]]>"))
			if j < 0 {
				return nil, nil, false, false, errXML
			}
			if text != nil {
				if len(*text) == 0 {
					*text = rest[9:j]
				} else {
					*text = append((*text)[:len(*text):len(*text)], rest[9:j]...)
				}
			}
			x.pos += j + 3
			continue
		case bytes.HasPrefix(rest, []byte("<!--")):
			j := bytes.Index(rest, []byte("-->"))
			if j < 0 {
				return nil, nil, false, false, errXML
			}
			x.pos += j + 3
			continue
		case bytes.HasPrefix(rest, []byte("<?")), bytes.HasPrefix(rest, []byte("<!")):
			j := bytes.IndexByte(rest, '>')
			if j < 0 {
				return nil, nil, false, false, errXML
			}
			x.pos += j + 1
			continue
		}
		j := bytes.IndexByte(rest, '>')
		if j < 0 {
			return nil, nil, false, false, errXML
		}
		inner := rest[1:j]
		x.pos += j + 1
		if len(inner) > 0 && inner[0] == '/' {
			return bytes.TrimSpace(inner[1:]), nil, true, false, nil
		}
		if len(inner) > 0 && inner[len(inner)-1] == '/' {
			inner, selfClosing = inner[:len(inner)-1], true
		}
		name, attrs = inner, nil
		if k := bytes.IndexAny(inner, " \t\r\n"); k >= 0 {
			name, attrs = inner[:k], inner[k:]
		}
		return name, attrs, false, selfClosing, nil
	}
}

// appendText adds text, unescaping the five XML entities.
func appendText(dst, s []byte) []byte {
	if bytes.IndexByte(s, '&') < 0 {
		return append(dst, s...)
	}
	for len(s) > 0 {
		i := bytes.IndexByte(s, '&')
		if i < 0 {
			return append(dst, s...)
		}
		dst = append(dst, s[:i]...)
		s = s[i:]
		replaced := false
		for _, e := range [...]struct{ from, to string }{{"&amp;", "&"}, {"&lt;", "<"}, {"&gt;", ">"}, {"&quot;", `"`}, {"&apos;", "'"}} {
			if bytes.HasPrefix(s, []byte(e.from)) {
				dst, s, replaced = append(dst, e.to...), s[len(e.from):], true
				break
			}
		}
		if !replaced {
			dst, s = append(dst, '&'), s[1:]
		}
	}
	return dst
}

// element reads the rest of an element whose start tag was just read.
func (x *xreader) element(name, attrs []byte, selfClosing bool, depth int) (xnode, error) {
	n := xnode{name: name, attrs: attrs}
	if selfClosing {
		return n, nil
	}
	if depth > 32 {
		return n, errXML
	}
	mark := len(x.stack)
	defer func() { x.stack = x.stack[:mark] }()
	for {
		kname, kattrs, end, kself, err := x.tag(&n.text)
		if err != nil {
			return n, err
		}
		if end {
			if !bytes.Equal(kname, name) {
				return n, errXML
			}
			if kids := x.stack[mark:]; len(kids) > 0 {
				start := len(x.arena)
				x.arena = append(x.arena, kids...)
				n.kids = x.arena[start:len(x.arena):len(x.arena)]
			}
			return n, nil
		}
		kid, err := x.element(kname, kattrs, kself, depth+1)
		if err != nil {
			return n, err
		}
		x.stack = append(x.stack, kid)
	}
}

// children calls fn with each child element of the element whose start tag was just read.
// The element is only valid during the call.
func (x *xreader) children(name []byte, fn func(*xnode)) error {
	for {
		kname, kattrs, end, kself, err := x.tag(nil)
		if err != nil {
			return err
		}
		if end {
			if !bytes.Equal(kname, name) {
				return errXML
			}
			return nil
		}
		kid, err := x.element(kname, kattrs, kself, 1)
		if err != nil {
			return err
		}
		fn(&kid)
		x.arena = x.arena[:0]
	}
}

// readGPIFNotes reads the note data from the top-level elements in b, the document
// after <Tracks>, up to the end of the root element.
func readGPIFNotes(b []byte, doc *gpifDoc) error {
	x := &xreader{b: b}
	for {
		name, _, end, selfClosing, err := x.tag(nil)
		if err != nil || end { // the end of <GPIF>
			return err
		}
		if selfClosing {
			continue
		}
		var fn func(*xnode)
		switch string(name) {
		case "MasterBars":
			fn = func(n *xnode) { doc.MasterBars = append(doc.MasterBars, readMasterBar(n)) }
		case "Bars":
			fn = func(n *xnode) {
				doc.Bars = append(doc.Bars, gpifBar{ID: n.intAttr("id"), Voices: n.textOf("Voices")})
			}
		case "Voices":
			fn = func(n *xnode) {
				doc.Voices = append(doc.Voices, gpifVoice{ID: n.intAttr("id"), Beats: n.textOf("Beats")})
			}
		case "Beats":
			fn = func(n *xnode) { doc.Beats = append(doc.Beats, readBeat(n)) }
		case "Notes":
			fn = func(n *xnode) { doc.Notes = append(doc.Notes, readNote(n)) }
		case "Rhythms":
			fn = func(n *xnode) { doc.Rhythms = append(doc.Rhythms, readRhythm(n)) }
		default: // skipped whole: no other top-level element has one of its name inside
			end := []byte("</" + string(name) + ">")
			i := bytes.Index(x.b[x.pos:], end)
			if i < 0 {
				return errXML
			}
			x.pos += i + len(end)
			continue
		}
		if err := x.children(name, fn); err != nil {
			return err
		}
	}
}

func readMasterBar(n *xnode) gpifMasterBar {
	mb := gpifMasterBar{
		Time:             n.textOf("Time"),
		Bars:             n.textOf("Bars"),
		AlternateEndings: n.textOf("AlternateEndings"),
		Section:          n.kid("Section").textOf("Text"),
	}
	r := n.kid("Repeat")
	mb.Repeat.Start = r.attr("start") == "true"
	mb.Repeat.End = r.attr("end") == "true"
	mb.Repeat.Count = r.intAttr("count")
	return mb
}

func readProps(n *xnode) []gpifNoteProp {
	ps := n.kid("Properties")
	out := make([]gpifNoteProp, 0, len(ps.kids))
	for i := range ps.kids {
		p := &ps.kids[i]
		if string(p.name) != "Property" {
			continue
		}
		gp := gpifNoteProp{
			Name:      p.attr("name"),
			String:    p.intOf("String"),
			Fret:      p.intOf("Fret"),
			Number:    p.intOf("Number"),
			Element:   p.intOf("Element"),
			Variation: p.intOf("Variation"),
			Enable:    p.has("Enable"),
			Flags:     max(p.intOf("Flags"), 0),
			HType:     p.textOf("HType"),
		}
		out = append(out, gp)
	}
	return out
}

func readBeat(n *xnode) gpifBeat {
	b := gpifBeat{
		ID:         n.intAttr("id"),
		Notes:      n.textOf("Notes"),
		Tremolo:    n.textOf("Tremolo"),
		GraceNotes: n.textOf("GraceNotes"),
		Properties: readProps(n),
	}
	b.Rhythm.Ref = n.kid("Rhythm").intAttr("ref")
	return b
}

func readNote(n *xnode) gpifNote {
	gn := gpifNote{
		ID:         n.intAttr("id"),
		Vibrato:    n.textOf("Vibrato"),
		AntiAccent: n.textOf("AntiAccent"),
		Trill:      n.textOf("Trill"),
		Properties: readProps(n),
	}
	gn.Tie.Destination = n.kid("Tie").attr("destination") == "true"
	gn.Accent = max(n.intOf("Accent"), 0)
	return gn
}

func readRhythm(n *xnode) gpifRhythm {
	r := gpifRhythm{ID: n.intAttr("id"), NoteValue: n.textOf("NoteValue")}
	r.Dot.Count = n.kid("AugmentationDot").intAttr("count")
	t := n.kid("PrimaryTuplet")
	r.Tuplet.Num, r.Tuplet.Den = t.intAttr("num"), t.intAttr("den")
	return r
}
