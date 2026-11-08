package kasession

import (
	"hash/fnv"
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
	h := fnv.New64a()
	var buf []byte
	// depth is the nodes' depths, which, written with each node, makes
	// the tree's shape part of the hash.
	depth := make([]uint16, s.NodeCount()+1)
	for id, end := psi.NodeID(1), psi.NodeID(s.NodeCount()+1); id < end; {
		n := s.Node(id)
		if p := n.Parent(); p.Valid() {
			depth[id] = depth[p.ID()] + 1
		}
		if n.NumericKind() == psi.KindBLOCK && isBodyOwner(n.Parent().NumericKind()) {
			buf = strconv.AppendUint(buf[:0], uint64(depth[id]), 10)
			buf = append(buf, "{}"...)
			h.Write(buf)
			id = n.SubtreeEnd()
			continue
		}
		switch {
		case n.IsTrivia():
		case n.IsLeaf():
			buf = strconv.AppendUint(buf[:0], uint64(depth[id]), 10)
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(n.NumericKind()), 10)
			buf = append(buf, ':')
			buf = append(buf, n.Text()...)
			buf = append(buf, 0)
			h.Write(buf)
		default:
			buf = strconv.AppendUint(buf[:0], uint64(depth[id]), 10)
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
