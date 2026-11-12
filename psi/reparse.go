package psi

// Reparse returns the snapshot of old's text with the bytes [from, to)
// replaced by text, for the same path. It equals Parse of the new text.
//
// When the edit lies strictly inside one lazily parsed BLOCK (a
// declaration's body, IntelliJ's chameleon) and the body's braces stay
// balanced, only that body is parsed again and spliced into a copy of
// old's tree, as IntelliJ's BlockSupportImpl does. Otherwise the whole text
// is parsed. It panics unless 0 <= from <= to <= len(old.Source()).
func Reparse(old *Snapshot, from, to int, text string) *Snapshot {
	var p Parser
	return p.Reparse(old, from, to, text)
}

// Reparse is the package-level Reparse with the Parser's buffers.
func (p *Parser) Reparse(old *Snapshot, from, to int, text string) *Snapshot {
	src := old.src[:from] + text + old.src[to:]
	if s := p.reparseBlock(old, from, to, src); s != nil {
		return s
	}
	return p.Parse(old.path, src)
}

// Reparsed reports whether s was made by Reparse splicing a body rather
// than by a whole parse; for tests and benchmarks.
func (s *Snapshot) Reparsed() bool { return s.spliced }

func (p *Parser) reparseBlock(old *Snapshot, from, to int, src string) *Snapshot {
	// The innermost lazy BLOCK whose braces the edit leaves alone. Ancestors
	// come first in preorder, so it is the last that contains the edit.
	li := -1
	for i, l := range old.lazy {
		if n := &old.nodes[l.id]; int(n.start) < from && to < int(n.end) {
			li = i
		}
	}
	if li < 0 {
		return nil
	}
	lb := old.lazy[li]
	blk, ob := lb.id, old.nodes[lb.id]
	delta := int32(len(src) - len(old.src))
	bs, be := ob.start, ob.end+delta

	// The new body must lex as one balanced block that leaves the lexer as
	// it found it: then the rest of the file lexes as before, and what the
	// outer parser's advanceBalancedBlock skipped before, it skips again.
	// The old body must have done the same, or the text after it was lexed
	// in another state.
	var neutral bool
	if p.kinds, p.starts, neutral = tokenizeNeutral(old.src[ob.start:ob.end], p.kinds[:0], p.starts[:0]); !neutral {
		return nil
	}
	p.kinds, p.starts, neutral = tokenizeNeutral(src[bs:be], p.kinds[:0], p.starts[:0])
	kinds, starts := p.kinds, p.starts
	if !neutral || len(kinds) == 0 || kinds[0] != KindLBRACE {
		return nil
	}
	depth := 0
	for i, k := range kinds {
		switch k {
		case KindLBRACE:
			depth++
		case KindRBRACE:
			depth--
			if depth == 0 && i != len(kinds)-1 {
				return nil
			}
		}
	}
	if depth != 0 || kinds[len(kinds)-1] != KindRBRACE {
		return nil
	}
	// A whole parse falls back to a flat tree past maxNesting; the block's
	// ancestors bound the bracket depth around it.
	// Inside a string template the lexer tracks braces for the entry, so the
	// body would not lex as it does in place.
	anc := 0
	for a := ob.parent; a != 0; a = old.nodes[a].parent {
		if old.nodes[a].kind == KindLONG_STRING_TEMPLATE_ENTRY {
			return nil
		}
		anc++
	}
	if anc+nesting(kinds) > maxNesting {
		return nil
	}
	for i := range starts {
		starts[i] += bs
	}

	// Parse the body as treeWriter.reparse does inside a whole parse.
	p.lexed = append(p.lexed[:0], kinds...)
	w := treeWriter{nodes: append(p.nodes[:0], node{}), lexed: p.lexed}
	b := newBuilder(src, kinds, starts, true)
	createForTopLevel(b).parseBlockExpression()
	w.emit(b)
	p.nodes = w.nodes[:0]

	// Splice: nodes before blk keep their IDs, blk's subtree is replaced,
	// and the nodes after it shift by dn IDs and delta bytes.
	oldEnd := ob.subtreeEnd
	m := NodeID(len(w.nodes) - 1)
	dn := m - (oldEnd - blk)
	fix := func(v NodeID) NodeID {
		if v >= oldEnd {
			return v + dn
		}
		return v
	}
	nodes := make([]node, NodeID(len(old.nodes))+dn)
	for id := NodeID(1); id < blk; id++ {
		n := old.nodes[id]
		n.firstChild, n.nextSibling, n.subtreeEnd = fix(n.firstChild), fix(n.nextSibling), fix(n.subtreeEnd)
		if n.end >= ob.end { // an ancestor
			n.end += delta
		}
		nodes[id] = n
	}
	shift := func(v NodeID) NodeID {
		if v == 0 {
			return 0
		}
		return v - 1 + blk
	}
	for j := NodeID(1); j <= m; j++ {
		n := w.nodes[j]
		n.parent, n.firstChild, n.nextSibling, n.prevSibling, n.subtreeEnd =
			shift(n.parent), shift(n.firstChild), shift(n.nextSibling), shift(n.prevSibling), shift(n.subtreeEnd)
		nodes[blk-1+j] = n
	}
	root := &nodes[blk]
	root.parent, root.prevSibling, root.nextSibling = ob.parent, ob.prevSibling, fix(ob.nextSibling)
	for id := oldEnd; id < NodeID(len(old.nodes)); id++ {
		n := old.nodes[id]
		n.parent, n.firstChild, n.nextSibling, n.prevSibling, n.subtreeEnd =
			fix(n.parent), fix(n.firstChild), fix(n.nextSibling), fix(n.prevSibling), fix(n.subtreeEnd)
		n.start += delta
		n.end += delta
		nodes[id+dn] = n
	}

	// Diagnostics are in emission order: those before the body (an
	// enclosing error element's ends after it), the body's, those after.
	dd := int32(len(w.diags)) - (lb.d1 - lb.d0)
	diags := make([]Diagnostic, 0, len(old.diags)+int(dd))
	for _, d := range old.diags[:lb.d0] {
		if d.Range.End >= int(ob.end) {
			d.Range.End += int(delta)
		}
		diags = append(diags, d)
	}
	diags = append(diags, w.diags...)
	for _, d := range old.diags[lb.d1:] {
		d.Range.Start += int(delta)
		d.Range.End += int(delta)
		diags = append(diags, d)
	}

	lazy := make([]lazyBlock, 0, len(old.lazy)+len(w.lazy))
	for _, l := range old.lazy[:li] {
		if old.nodes[l.id].subtreeEnd > blk { // an ancestor
			l.d1 += dd
		}
		lazy = append(lazy, l)
	}
	lazy = append(lazy, lazyBlock{id: blk, d0: lb.d0, d1: lb.d0 + int32(len(w.diags))})
	for _, l := range w.lazy {
		lazy = append(lazy, lazyBlock{id: shift(l.id), d0: l.d0 + lb.d0, d1: l.d1 + lb.d0})
	}
	for _, l := range old.lazy[li+1:] {
		if l.id >= oldEnd {
			lazy = append(lazy, lazyBlock{id: l.id + dn, d0: l.d0 + dd, d1: l.d1 + dd})
		}
	}

	s := newSnapshot(old.path, src, nodes, diags)
	s.lazy = lazy
	s.spliced = true
	return s
}
