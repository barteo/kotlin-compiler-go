package analysis

import (
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// A span is the psi nodes [lo, hi) of a snapshot, in preorder: a subtree,
// or the statements of a block from one on.
type span struct{ lo, hi psi.NodeID }

// errorSpans returns the nodes of snap whose facts an error at offset off
// invalidates, as recovering lowering (lower.Recover) and the checker
// leave the code around an error, or false if it is all of the file's.
// The error is in a declaration of the file or of a class body, D:
//
//   - in a statement of D's block body, or an accessor's: the statement,
//     those after it, which it may narrow or declare for, and the block;
//   - at a brace of that body, as a missing return: the block alone;
//   - in the expression after D's '=', or an accessor's: the expression;
//   - in a class's header: the header, but for its body's members;
//   - elsewhere in D: D.
//
// An error between the declarations of a class body, or the file, is its
// node's; in the package or imports, or past the declarations, it is the
// whole file's.
func errorSpans(snap *psi.Snapshot, off int) ([]span, bool) {
	if snap == nil || off < 0 || off >= len(snap.Source()) {
		return nil, false
	}
	leaf := snap.NodeAt(off)
	if !leaf.Valid() {
		return nil, false
	}
	var path []psi.NodeRef // root first
	for n := leaf; n.Valid(); n = n.Parent() {
		path = append(path, n)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	subtree := func(n psi.NodeRef) []span { return []span{{n.ID(), n.SubtreeEnd()}} }
	var d, acc psi.NodeRef // the declaration, and its accessor
	for k := 1; k < len(path); k++ {
		p, c := path[k-1], path[k]
		switch {
		case d.Valid() && (p == d || p == acc):
			owner := p
			switch {
			case lower.InTail(owner, c):
				return tailSpan(owner), true
			case c.NumericKind() == psi.KindBLOCK:
				return bodySpans(d, c, path[k+1:]), true
			case c.NumericKind() == psi.KindPROPERTY_ACCESSOR && p == d:
				acc = c
				continue
			case c.NumericKind() == psi.KindCLASS_BODY && p == d:
				if k+1 < len(path) && isMember(path[k+1]) {
					d, acc = path[k+1], psi.NodeRef{}
					k++
					continue
				}
				if k+1 < len(path) && !path[k+1].IsLeaf() {
					return subtree(path[k+1]), true
				}
				return nil, true // a brace, or a separator
			}
			if b := d.ChildKind(psi.KindCLASS_BODY); b.Valid() {
				return []span{{d.ID(), b.ID() + 1}}, true
			}
			return subtree(d), true
		case !d.Valid() && p.NumericKind() == psi.KindFILE:
			switch {
			case isMember(c):
				d = c
			case c.NumericKind() == psi.KindERROR_ELEMENT:
				return subtree(c), true
			default:
				return nil, false
			}
		}
	}
	if d.Valid() {
		return subtree(d), true
	}
	return nil, false
}

// isMember reports whether n is a declaration kc lowers as a member of a
// file or a class.
func isMember(n psi.NodeRef) bool {
	switch n.NumericKind() {
	case psi.KindFUN, psi.KindPROPERTY, psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS,
		psi.KindCLASS_INITIALIZER, psi.KindSECONDARY_CONSTRUCTOR, psi.KindENUM_ENTRY:
		return true
	}
	return false
}

// bodySpans returns the spans an error in block b, the body of
// declaration d, invalidates; below are the nodes from b's child down to
// the error's.
func bodySpans(d, b psi.NodeRef, below []psi.NodeRef) []span {
	if !b.ChildKind(psi.KindLBRACE).Valid() || !b.ChildKind(psi.KindRBRACE).Valid() || len(below) == 0 {
		return []span{{d.ID(), d.SubtreeEnd()}}
	}
	self := span{b.ID(), b.ID() + 1}
	s := below[0]
	switch s.NumericKind() {
	case psi.KindLBRACE, psi.KindRBRACE:
		return []span{self}
	}
	if s.IsTrivia() {
		// Between statements: from the next one on.
		for s = s.NextSibling(); s.Valid() && s.IsTrivia(); s = s.NextSibling() {
		}
		if !s.Valid() || s.NumericKind() == psi.KindRBRACE {
			return []span{self}
		}
	}
	return []span{self, {s.ID(), b.SubtreeEnd()}}
}

// tailSpan returns the nodes after the '=' of n, a property, function or
// accessor, up to a property's accessors.
func tailSpan(n psi.NodeRef) []span {
	eq := n.ChildKind(psi.KindEQ)
	lo, hi := psi.NodeID(0), psi.NodeID(0)
	for c := eq.NextSibling(); c.Valid() && lower.InTail(n, c); c = c.NextSibling() {
		if lo == 0 {
			lo = c.ID()
		}
		hi = c.SubtreeEnd()
	}
	if lo == 0 {
		return nil
	}
	return []span{{lo, hi}}
}

// dropped reports whether node id is in one of spans.
func dropped(spans []span, id psi.NodeID) bool {
	for _, s := range spans {
		if s.lo <= id && id < s.hi {
			return true
		}
	}
	return false
}
