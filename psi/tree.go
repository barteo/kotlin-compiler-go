package psi

import "strings"

// Tree building: a port of PsiBuilderImpl.balanceWhiteSpaces and of the
// parts of its tree construction psi needs, plus the edge binders from
// KotlinWhitespaceAndCommentsBinders.kt (see THIRD_PARTY_NOTICES.md).

// edgePosition is WhitespacesAndCommentsBinder.getEdgePosition for the
// trivia tokens [lo, hi) of b.
func (b *builder) edgePosition(binder binderID, lo, hi int) int {
	size := hi - lo
	if size == 0 {
		return 0
	}
	typ := func(i int) Kind { return b.types[lo+i] }
	text := func(i int) string { return b.src[b.starts[lo+i]:b.starts[lo+i+1]] }
	switch binder {
	case defaultLeftBinder:
		return size
	case defaultRightBinder, doNotBindAnything, precedingAllBinder:
		return 0
	case trailingAllBinder:
		return size
	case precedingDocCommentsBinder:
		for i := size - 1; i >= 0; i-- {
			if typ(i) == kDocComment {
				return i
			}
		}
		return size
	case precedingCommentsBinder:
		for i := size - 1; i >= 0; i-- {
			if typ(i) == kDocComment {
				return i
			}
		}
		result := size
	loop:
		for i := size - 1; i >= 0; i-- {
			switch t := typ(i); {
			case t == KindWHITE_SPACE:
				if strings.Count(text(i), "\n") > 1 {
					break loop
				}
			case commentTokens.has(t):
				if i == 0 || typ(i-1) == KindWHITE_SPACE && strings.Contains(text(i-1), "\n") {
					result = i
				}
			default:
				break loop
			}
		}
		return result
	case trailingCommentsBinder:
		result := 0
	tloop:
		for i := 0; i < size; i++ {
			switch typ(i) {
			case KindWHITE_SPACE:
				if strings.Contains(text(i), "\n") {
					break tloop
				}
			case KindEOL_COMMENT, KindBLOCK_COMMENT:
				result = i + 1
			default:
				break tloop
			}
		}
		return result
	case precedingAllCommentsBinder:
		if typ(0) == KindWHITE_SPACE {
			return 1
		}
		return 0
	case trailingAllCommentsBinder:
		if typ(size-1) == KindWHITE_SPACE {
			return size - 1
		}
		return size
	case bindFirstShebangWithWhitespaceOnly:
		if typ(0) == KindSHEBANG_COMMENT {
			if size > 1 && typ(1) == KindWHITE_SPACE {
				return 2
			}
			return 1
		}
		return 0
	}
	return 0
}

// binderOf returns the edge binder of production entry p.
func (b *builder) binderOf(p prod) binderID {
	if p.kind == prodStart {
		return b.markers[p.m].left
	}
	return b.markers[p.m].right
}

// balanceWhiteSpaces is PsiBuilderImpl.balanceWhiteSpaces: it moves every
// production edge across adjacent trivia as its binder decides.
func (b *builder) balanceWhiteSpaces() {
	last := 0
	for i := 1; i < len(b.prods)-1; i++ {
		item := &b.prods[i]
		prev := int(b.prods[i-1].lex)
		wsStart := max(int(item.lex), last)
		for wsStart > prev && isWhitespaceOrComment(b.types[wsStart-1]) {
			wsStart--
		}
		wsEnd := wsStart
		for wsEnd < b.n && isWhitespaceOrComment(b.types[wsEnd]) {
			wsEnd++
		}
		if wsStart != wsEnd {
			item.lex = int32(wsStart + b.edgePosition(b.binderOf(*item), wsStart, wsEnd))
		} else if int(item.lex) < wsStart {
			item.lex = int32(wsStart)
		}
		last = int(item.lex)
	}
}

// node is one arena entry. IDs are indexes; 0 means none.
type node struct {
	kind        Kind
	leaf        bool
	start, end  int32
	parent      NodeID
	firstChild  NodeID
	nextSibling NodeID
	prevSibling NodeID
	subtreeEnd  NodeID
}

// treeWriter appends nodes to an arena in preorder.
type treeWriter struct {
	nodes []node // nodes[0] is unused
	stack []NodeID
	last  []NodeID // last child of each open node
	diags []Diagnostic
	lexed []Kind // token kinds as lexed, before any remapping
}

func (w *treeWriter) add(k Kind, start, end int32) NodeID {
	id := NodeID(len(w.nodes))
	n := node{kind: k, start: start, end: end}
	if d := len(w.stack); d > 0 {
		n.parent = w.stack[d-1]
		if prev := w.last[d-1]; prev != 0 {
			w.nodes[prev].nextSibling = id
			n.prevSibling = prev
		} else {
			w.nodes[n.parent].firstChild = id
		}
		w.last[d-1] = id
	}
	w.nodes = append(w.nodes, n)
	return id
}

func (w *treeWriter) open(k Kind, start int32) NodeID {
	id := w.add(k, start, start)
	w.stack = append(w.stack, id)
	w.last = append(w.last, 0)
	return id
}

func (w *treeWriter) close(end int32) {
	d := len(w.stack) - 1
	id := w.stack[d]
	w.nodes[id].end = end
	w.nodes[id].subtreeEnd = NodeID(len(w.nodes))
	w.stack = w.stack[:d]
	w.last = w.last[:d]
}

func (w *treeWriter) leaf(k Kind, start, end int32) {
	id := w.add(k, start, end)
	w.nodes[id].leaf = true
	w.nodes[id].subtreeEnd = id + 1
}

// leaves adds the tokens [from, to) of b as leaves, skipping empty ones
// as PsiBuilderImpl.insertLeaves does.
func (w *treeWriter) leaves(b *builder, from, to int) {
	for i := from; i < to; i++ {
		if s, e := b.starts[i], b.starts[i+1]; s < e {
			w.leaf(b.types[i], s, e)
		}
	}
}

func (w *treeWriter) syntaxError(start, end int32, msg string) {
	w.diags = append(w.diags, Diagnostic{Range: Range{Start: int(start), End: int(end)}, Code: "syntax_error", Message: msg})
}

// emit balances b's production list and appends its tree.
func (w *treeWriter) emit(b *builder) {
	b.balanceWhiteSpaces()
	prods := b.prods
	tok := 0
	lastErr := -1  // PsiBuilderImpl.bind drops repeated error items at one token
	var pend []int // diagnostics of the open ERROR_ELEMENT markers
	for i := 0; i < len(prods); i++ {
		p := prods[i]
		md := &b.markers[p.m]
		at := int(p.lex)
		switch p.kind {
		case prodStart:
			w.leaves(b, tok, at)
			tok = at
			if md.collapsed {
				j := i + 1
				for prods[j].m != p.m || prods[j].kind != prodDone {
					j++
				}
				end := int(prods[j].lex)
				if md.lazy {
					w.reparse(b, at, end, md.typ)
				} else {
					w.leaf(md.typ, b.starts[at], b.starts[end])
				}
				tok, i = end, j
				continue
			}
			k := md.typ
			if i == 0 && k == KindKT_FILE {
				k = KindFILE
			}
			w.open(k, b.starts[at])
			if k == KindERROR_ELEMENT {
				pend = append(pend, len(w.diags))
				w.syntaxError(b.starts[at], b.starts[at], md.msg)
			}
		case prodDone:
			if i == len(prods)-1 {
				at = b.n // the root keeps every token
			}
			w.leaves(b, tok, at)
			tok = at
			w.close(b.starts[at])
			if md.typ == KindERROR_ELEMENT {
				d := pend[len(pend)-1]
				pend = pend[:len(pend)-1]
				w.diags[d].Range.End = int(b.starts[at])
			}
		case prodError:
			w.leaves(b, tok, at)
			tok = at
			if at == lastErr {
				continue
			}
			lastErr = at
			s := b.starts[at]
			w.open(KindERROR_ELEMENT, s)
			w.close(s)
			w.syntaxError(s, s, md.msg)
		}
	}
}

// reparse expands a lazily collapsed BLOCK or LAMBDA_EXPRESSION over the
// tokens [lo, hi), as IntelliJ does when the chameleon is first opened:
// a fresh builder parses just those tokens, as lexed.
func (w *treeWriter) reparse(b *builder, lo, hi int, k Kind) {
	copy(b.types[lo:hi], w.lexed[b.base0+lo:b.base0+hi])
	sb := newBuilder(b.src, b.types[lo:hi], b.starts[lo:hi+1], true)
	sb.base0 = b.base0 + lo
	p := createForTopLevel(sb)
	if k == KindBLOCK {
		p.parseBlockExpression()
	} else {
		p.parseLambdaExpression()
	}
	w.emit(sb)
}
