package psi

import (
	"fmt"
	"sort"
	"sync/atomic"
	"unicode/utf8"
)

// Range is a half-open range of UTF-8 byte offsets.
type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Diagnostic is a syntax error found while parsing.
type Diagnostic struct {
	Range   Range  `json:"range"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NodeID identifies a node within one Snapshot. IDs run 1..NodeCount in
// preorder; 0 means no node.
type NodeID uint32

// SnapshotID identifies a Snapshot within the process.
type SnapshotID uint64

var snapshotIDs atomic.Uint64

// Snapshot is an immutable, lossless syntax tree of one source file.
type Snapshot struct {
	id    SnapshotID
	path  string
	src   string
	nodes []node // nodes[0] is unused
	diags []Diagnostic

	lineStarts []int32
	// utf16 holds checkpoints for byte to UTF-16 conversion: utf16[i] is
	// the UTF-16 length of src[:cpOff[i]]. Both are nil for ASCII sources.
	cpOff []int32
	utf16 []int32
}

const utf16Stride = 512

func newSnapshot(path, src string, nodes []node, diags []Diagnostic) *Snapshot {
	s := &Snapshot{id: SnapshotID(snapshotIDs.Add(1)), path: path, src: src, nodes: nodes, diags: diags}
	lines := 1
	ascii := true
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '\n' {
			lines++
		} else if c >= utf8.RuneSelf {
			ascii = false
		}
	}
	s.lineStarts = make([]int32, 1, lines)
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			s.lineStarts = append(s.lineStarts, int32(i+1))
		}
	}
	if !ascii {
		units := 0
		next := 0
		for i, r := range src {
			if i >= next {
				s.cpOff = append(s.cpOff, int32(i))
				s.utf16 = append(s.utf16, int32(units))
				next = i + utf16Stride
			}
			units += utf16Len(r)
		}
	}
	return s
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// ID returns the snapshot's process-unique identity.
func (s *Snapshot) ID() SnapshotID { return s.id }

// Path returns the path given to Parse.
func (s *Snapshot) Path() string { return s.path }

// Source returns the parsed text.
func (s *Snapshot) Source() string { return s.src }

// Diagnostics returns the syntax errors, one per ERROR_ELEMENT node, in
// preorder.
func (s *Snapshot) Diagnostics() []Diagnostic { return s.diags }

// NodeCount returns the number of nodes; IDs run 1..NodeCount.
func (s *Snapshot) NodeCount() int { return len(s.nodes) - 1 }

// RootNode returns the FILE node.
func (s *Snapshot) RootNode() NodeRef { return s.Node(1) }

// Node returns the node with the given ID, or the zero NodeRef.
func (s *Snapshot) Node(id NodeID) NodeRef {
	if s == nil || id == 0 || int(id) >= len(s.nodes) {
		return NodeRef{}
	}
	return NodeRef{s, id}
}

// NodeAt returns the leaf containing byte offset off; at the end of the
// source it returns the last leaf.
func (s *Snapshot) NodeAt(off int) NodeRef {
	if s == nil || len(s.nodes) < 2 || off < 0 || off > len(s.src) {
		return NodeRef{}
	}
	id := NodeID(1)
	for {
		n := &s.nodes[id]
		if n.leaf {
			return NodeRef{s, id}
		}
		var pick, last NodeID
		for c := n.firstChild; c != 0; c = s.nodes[c].nextSibling {
			cn := &s.nodes[c]
			if cn.start == cn.end {
				continue
			}
			last = c
			if int(cn.start) <= off && off < int(cn.end) {
				pick = c
				break
			}
		}
		if pick == 0 {
			if off == len(s.src) && last != 0 {
				pick = last
			} else {
				return NodeRef{s, id}
			}
		}
		id = pick
	}
}

// DeclarationNodes returns the file's top-level declarations, as
// KtFile.getDeclarations does.
func (s *Snapshot) DeclarationNodes() []NodeRef {
	var out []NodeRef
	root := s.RootNode()
	for c := root.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == KindSCRIPT {
			if b := c.ChildKind(KindBLOCK); b.Valid() {
				for d := b.FirstChild(); d.Valid(); d = d.NextSibling() {
					if declarationKinds.has(d.NumericKind()) {
						out = append(out, d)
					}
				}
			}
			continue
		}
		if declarationKinds.has(c.NumericKind()) {
			out = append(out, c)
		}
	}
	return out
}

// checkOffset accepts the offsets at which decoding the source rune by rune
// (as a range loop does, one rune per invalid byte) can stop.
func (s *Snapshot) checkOffset(off int) error {
	if off < 0 || off > len(s.src) {
		return fmt.Errorf("invalid UTF-8 boundary: %d", off)
	}
	if off == len(s.src) || utf8.RuneStart(s.src[off]) {
		return nil
	}
	// A continuation byte is inside a rune only when the nearest preceding
	// rune start decodes through it; otherwise it decodes on its own.
	for p := off - 1; p >= 0 && p >= off-utf8.UTFMax; p-- {
		if utf8.RuneStart(s.src[p]) {
			if _, size := utf8.DecodeRuneInString(s.src[p:]); p+size > off {
				return fmt.Errorf("invalid UTF-8 boundary: %d", off)
			}
			return nil
		}
	}
	return nil
}

// UTF16Offset converts a byte offset to a UTF-16 offset.
func (s *Snapshot) UTF16Offset(off int) (int, error) {
	if err := s.checkOffset(off); err != nil {
		return 0, err
	}
	return s.utf16At(off), nil
}

func (s *Snapshot) utf16At(off int) int {
	if s.cpOff == nil {
		return off
	}
	i := sort.Search(len(s.cpOff), func(i int) bool { return int(s.cpOff[i]) > off }) - 1
	pos, units := int(s.cpOff[i]), int(s.utf16[i])
	for _, r := range s.src[pos:off] {
		units += utf16Len(r)
	}
	return units
}

// Position returns the 1-based line and UTF-16 column of a byte offset.
func (s *Snapshot) Position(off int) (line, column int, err error) {
	_, line, column, err = s.OffsetPosition(off)
	return
}

// OffsetPosition returns the UTF-16 offset and the 1-based line and
// UTF-16 column of a byte offset.
func (s *Snapshot) OffsetPosition(off int) (utf16Off, line, column int, err error) {
	if err = s.checkOffset(off); err != nil {
		return 0, 0, 0, err
	}
	l := sort.Search(len(s.lineStarts), func(i int) bool { return int(s.lineStarts[i]) > off }) - 1
	u := s.utf16At(off)
	return u, l + 1, u - s.utf16At(int(s.lineStarts[l])) + 1, nil
}
