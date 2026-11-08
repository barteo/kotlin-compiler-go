// Package ide answers editor questions about Kotlin code: the features of
// kc-lsp (docs/lsp.md), as functions over immutable documents that return
// plain Go values. Package internal/lsp translates them to the protocol.
//
// Ranges are UTF-8 byte offsets (psi.Range). A Document converts between
// offsets and editor positions in UTF-16 or UTF-8 units.
package ide

import (
	"fmt"
	"sort"
	"sync"
	"unicode/utf8"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// Encoding is the unit an editor position's character counts.
type Encoding int

const (
	UTF16 Encoding = iota // LSP's default
	UTF8
)

// Pos is a zero-based line and character, counted in an Encoding.
type Pos struct {
	Line, Char int
}

// Edit replaces the text between From and To, or the whole document when
// Whole is set.
type Edit struct {
	Whole    bool
	From, To Pos
	Text     string
}

// Document is one version of an open file and its syntax tree. It is
// immutable and safe for concurrent use.
type Document struct {
	Path    string
	Version int
	Snap    *psi.Snapshot

	lines []int // byte offset of each line's start
}

// NewDocument parses text as the given version of the file at path.
func NewDocument(path string, version int, text string) *Document {
	return &Document{Path: path, Version: version, Snap: parse(path, text), lines: lineStarts(text)}
}

// parsers keeps psi.Parsers, whose reused buffers make a reparse about 40%
// faster (docs/psi.md, psi-reused).
var parsers = sync.Pool{New: func() any { return new(psi.Parser) }}

func parse(path, text string) *psi.Snapshot {
	p := parsers.Get().(*psi.Parser)
	defer parsers.Put(p)
	return p.Parse(path, text)
}

// Text returns the document's text.
func (d *Document) Text() string { return d.Snap.Source() }

func lineStarts(text string) []int {
	ls := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			ls = append(ls, i+1)
		}
	}
	return ls
}

// Apply returns the next version with the edits applied in order, each to
// the result of the ones before it, as LSP's didChange does.
func (d *Document) Apply(version int, edits []Edit, enc Encoding) (*Document, error) {
	p := parsers.Get().(*psi.Parser)
	defer parsers.Put(p)
	snap, lines := d.Snap, d.lines
	for i, e := range edits {
		if e.Whole {
			snap = p.Parse(d.Path, e.Text)
		} else {
			text := snap.Source()
			from, err := offset(text, lines, e.From, enc)
			if err != nil {
				return nil, fmt.Errorf("edit %d: %w", i, err)
			}
			to, err := offset(text, lines, e.To, enc)
			if err != nil {
				return nil, fmt.Errorf("edit %d: %w", i, err)
			}
			if to < from {
				return nil, fmt.Errorf("edit %d: range ends before it starts", i)
			}
			snap = p.Reparse(snap, from, to, e.Text) // only the edited body, when it can
		}
		lines = lineStarts(snap.Source())
	}
	return &Document{Path: d.Path, Version: version, Snap: snap, lines: lines}, nil
}

// Offset converts a position to a byte offset. As LSP asks, a character
// past the end of its line means the line's end; a line past the end of the
// document is an error.
func (d *Document) Offset(p Pos, enc Encoding) (int, error) {
	return offset(d.Text(), d.lines, p, enc)
}

func offset(text string, lines []int, p Pos, enc Encoding) (int, error) {
	if p.Line < 0 || p.Char < 0 {
		return 0, fmt.Errorf("invalid position %d:%d", p.Line, p.Char)
	}
	if p.Line >= len(lines) {
		if p.Line == len(lines) && p.Char == 0 {
			return len(text), nil // just past a final newline
		}
		return 0, fmt.Errorf("position %d:%d is past the end of the document (%d lines)", p.Line, p.Char, len(lines))
	}
	start := lines[p.Line]
	end := len(text)
	if p.Line+1 < len(lines) {
		end = lines[p.Line+1] - 1 // the newline
		if end > start && text[end-1] == '\r' {
			end--
		}
	}
	if enc == UTF8 {
		off := start + p.Char
		if off > end {
			off = end
		}
		for off > start && off < len(text) && !utf8.RuneStart(text[off]) {
			off-- // inside a rune: its start
		}
		return off, nil
	}
	units := 0
	for i, r := range text[start:end] {
		if units+utf16Len(r) > p.Char {
			return start + i, nil // at p.Char, or inside a surrogate pair: its rune's start
		}
		units += utf16Len(r)
	}
	return end, nil
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// Pos converts a byte offset to a position. An offset inside a rune counts
// as that rune's start.
func (d *Document) Pos(off int, enc Encoding) Pos {
	text := d.Text()
	if off < 0 {
		off = 0
	}
	if off > len(text) {
		off = len(text)
	}
	line := sort.SearchInts(d.lines, off+1) - 1
	start := d.lines[line]
	if enc == UTF8 {
		return Pos{line, off - start}
	}
	units := 0
	for i := start; i < off; {
		r, size := utf8.DecodeRuneInString(text[i:])
		if i+size > off {
			break
		}
		units += utf16Len(r)
		i += size
	}
	return Pos{line, units}
}

// Line returns the zero-based line of a byte offset.
func (d *Document) Line(off int) int {
	return sort.SearchInts(d.lines, off+1) - 1
}

// LineEnd returns the byte offset of the end of a zero-based line, before
// its line terminator.
func (d *Document) LineEnd(line int) int {
	text := d.Text()
	if line+1 >= len(d.lines) {
		return len(text)
	}
	end := d.lines[line+1] - 1
	if end > d.lines[line] && text[end-1] == '\r' {
		end--
	}
	return end
}
