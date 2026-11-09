package kasession

import (
	"hash/fnv"
	"slices"
	"strconv"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// SignatureHash returns a hash of what s declares: its tokens but for
// whitespace, comments and the block bodies of functions, accessors,
// initializers and secondary constructors, which a check of the
// declarations (check.Options.Signatures) doesn't need. A block body
// doesn't make a declaration's type: a function with one and no written
// return type returns Unit. Two parses with the same hash declare the
// same, so the other files of a module don't need checking again when
// only bodies change (an incremental update).
func SignatureHash(s *psi.Snapshot) uint64 {
	return hashNodes(s, 1, psi.NodeID(s.NodeCount()+1))
}

// DeclarationHash returns SignatureHash's hash of the subtree of n: two
// parses of a declaration with the same hash declare the same.
func DeclarationHash(n psi.NodeRef) uint64 {
	return hashNodes(n.ContainingFile(), n.ID(), n.SubtreeEnd())
}

// hashNodes hashes the nodes of s from id to end, a run of subtrees.
func hashNodes(s *psi.Snapshot, id, end psi.NodeID) uint64 {
	h := fnv.New64a()
	var buf []byte
	// depth is the nodes' depths below the first's parent, which, written
	// with each node, makes the tree's shape part of the hash.
	depth := make([]uint16, int(end-id)+1)
	first := id
	for id < end {
		n := s.Node(id)
		if p := n.Parent(); p.Valid() && p.ID() >= first {
			depth[id-first] = depth[p.ID()-first] + 1
		}
		d := uint64(depth[id-first])
		if n.NumericKind() == psi.KindBLOCK && isBodyOwner(n.Parent().NumericKind()) {
			buf = strconv.AppendUint(buf[:0], d, 10)
			buf = append(buf, "{}"...)
			h.Write(buf)
			id = n.SubtreeEnd()
			continue
		}
		switch {
		case n.IsTrivia():
		case n.IsLeaf():
			buf = strconv.AppendUint(buf[:0], d, 10)
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(n.NumericKind()), 10)
			buf = append(buf, ':')
			buf = append(buf, n.Text()...)
			buf = append(buf, 0)
			h.Write(buf)
		default:
			buf = strconv.AppendUint(buf[:0], d, 10)
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(n.NumericKind()), 10)
			buf = append(buf, '(')
			h.Write(buf)
		}
		id++
	}
	return h.Sum64()
}

func isBodyOwner(k psi.Kind) bool {
	switch k {
	case psi.KindFUN, psi.KindPROPERTY_ACCESSOR, psi.KindCLASS_INITIALIZER, psi.KindSECONDARY_CONSTRUCTOR:
		return true
	}
	return false
}

// SignatureNodes returns the nodes SignatureHash hashes, in order: the
// nodes but for trivia and the insides of block bodies, whose blocks it
// lists. Two parses with the same hash have as many, of the same kinds,
// and the i-th of one declares what the i-th of the other does: an
// incremental update maps what refers to the declarations of an edited
// file's last parse to its new one by them.
func SignatureNodes(s *psi.Snapshot) []psi.NodeID {
	var ids []psi.NodeID
	for id, end := psi.NodeID(1), psi.NodeID(s.NodeCount()+1); id < end; {
		n := s.Node(id)
		if n.NumericKind() == psi.KindBLOCK && isBodyOwner(n.Parent().NumericKind()) {
			ids = append(ids, id)
			id = n.SubtreeEnd()
			continue
		}
		if !n.IsTrivia() {
			ids = append(ids, id)
		}
		id++
	}
	return ids
}

// AlignSignatureNodes returns the SignatureNodes of old and cur that
// correspond, in order: those of the longest common prefix and suffix
// of the two lists, nodes of the same kinds and, for leaves, texts.
// What a declaration edit changed is between them, and corresponds to
// nothing. Two parses with the same SignatureHash correspond whole.
func AlignSignatureNodes(old, cur *psi.Snapshot) (oldIDs, curIDs []psi.NodeID) {
	a, b := SignatureNodes(old), SignatureNodes(cur)
	same := func(i, j int) bool {
		x, y := old.Node(a[i]), cur.Node(b[j])
		if x.NumericKind() != y.NumericKind() {
			return false
		}
		return !x.IsLeaf() || x.Text() == y.Text()
	}
	p := 0
	for p < len(a) && p < len(b) && same(p, p) {
		p++
	}
	q := 0
	for q < len(a)-p && q < len(b)-p && same(len(a)-1-q, len(b)-1-q) {
		q++
	}
	oldIDs = append(slices.Clip(a[:p]), a[len(a)-q:]...)
	curIDs = append(slices.Clip(b[:p]), b[len(b)-q:]...)
	return oldIDs, curIDs
}
