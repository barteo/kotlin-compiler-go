package ka

import (
	"hash"
	"hash/fnv"
	"strconv"

	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// signatureHash returns a hash of what s declares: its tokens but for
// whitespace, comments and the block bodies of functions, accessors,
// initializers and secondary constructors, which a check of the
// declarations (check.Options.Signatures) doesn't need, but for a leading
// contract { } call (lower.Contract), whose effects callers see. A block body
// doesn't make a declaration's type: a function with one and no written
// return type returns Unit. Two parses with the same hash declare the
// same, so the other files of a module don't need checking again when
// only bodies change (an incremental update, reuse.go).
func signatureHash(s *psi.Snapshot) uint64 {
	h := fnv.New64a()
	hashSignature(h, s, 1, psi.NodeID(s.NodeCount()+1), make([]uint16, s.NodeCount()+1))
	return h.Sum64()
}

// declHashes returns signatureHash's hashes of the parts of s: of each
// top-level declaration, by name, in order, and of the rest (the package
// directive, the imports, the file's annotations, and any declaration
// without a name). A declaration's hash doesn't depend on where it is.
func declHashes(s *psi.Snapshot) (rest uint64, decls map[string][]uint64) {
	depth := make([]uint16, s.NodeCount()+1)
	decls = map[string][]uint64{}
	hr := fnv.New64a()
	root := s.Node(1)
	for n := root.FirstChild(); n.Valid(); n = n.NextSibling() {
		if n.IsTrivia() {
			continue
		}
		depth[n.ID()] = 1
		name := ""
		switch n.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindFUN, psi.KindPROPERTY, psi.KindTYPEALIAS:
			name = n.Name()
		}
		if name == "" {
			hashSignature(hr, s, n.ID(), n.SubtreeEnd(), depth)
			continue
		}
		h := fnv.New64a()
		hashSignature(h, s, n.ID(), n.SubtreeEnd(), depth)
		decls[name] = append(decls[name], h.Sum64())
	}
	return hr.Sum64(), decls
}

// hashSignature writes to h the nodes of s from id to end, as
// signatureHash does: depth holds the depths of id's ancestors.
func hashSignature(h hash.Hash64, s *psi.Snapshot, id, end psi.NodeID, depth []uint16) {
	var buf []byte
	for id < end {
		n := s.Node(id)
		if p := n.Parent(); p.Valid() && id != 1 {
			depth[id] = depth[p.ID()] + 1
		}
		if n.NumericKind() == psi.KindBLOCK && isBodyOwner(n.Parent().NumericKind()) {
			buf = strconv.AppendUint(buf[:0], uint64(depth[id]), 10)
			buf = append(buf, "{}"...)
			h.Write(buf)
			if c := lower.Contract(n); c.Valid() {
				for cid, end := c.ID(), c.SubtreeEnd(); cid < end; cid++ {
					if l := s.Node(cid); l.IsLeaf() && !l.IsTrivia() {
						h.Write(append(append(buf[:0], l.Text()...), 0))
					}
				}
			}
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
}

func isBodyOwner(k psi.Kind) bool {
	switch k {
	case psi.KindFUN, psi.KindPROPERTY_ACCESSOR, psi.KindCLASS_INITIALIZER, psi.KindSECONDARY_CONSTRUCTOR:
		return true
	}
	return false
}
