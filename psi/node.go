package psi

import "strings"

// NodeRef is a reference to one node of a Snapshot. It is comparable;
// the zero value is invalid and every method is safe to call on it.
type NodeRef struct {
	s  *Snapshot
	id NodeID
}

func (n NodeRef) get() *node {
	if n.s == nil || n.id == 0 {
		return nil
	}
	return &n.s.nodes[n.id]
}

func (n NodeRef) rel(id NodeID) NodeRef {
	if id == 0 {
		return NodeRef{}
	}
	return NodeRef{n.s, id}
}

// Valid reports whether n refers to a node.
func (n NodeRef) Valid() bool { return n.s != nil && n.id != 0 }

// ID returns the node's ID, or 0.
func (n NodeRef) ID() NodeID { return n.id }

// NumericKind returns the node's kind, or KindUnknown.
func (n NodeRef) NumericKind() Kind {
	if p := n.get(); p != nil {
		return p.kind
	}
	return KindUnknown
}

// Kind returns the kind's name, or "" for an invalid node.
func (n NodeRef) Kind() string {
	if !n.Valid() {
		return ""
	}
	return n.NumericKind().String()
}

// Text returns the node's source text.
func (n NodeRef) Text() string {
	if p := n.get(); p != nil {
		return n.s.src[p.start:p.end]
	}
	return ""
}

// Range returns the node's byte range.
func (n NodeRef) Range() Range {
	if p := n.get(); p != nil {
		return Range{int(p.start), int(p.end)}
	}
	return Range{}
}

// SubtreeEnd returns the first ID after n's subtree: the subtree is
// exactly the IDs [n.ID(), n.SubtreeEnd()).
func (n NodeRef) SubtreeEnd() NodeID {
	if p := n.get(); p != nil {
		return p.subtreeEnd
	}
	return 0
}

func (n NodeRef) Parent() NodeRef {
	if p := n.get(); p != nil {
		return n.rel(p.parent)
	}
	return NodeRef{}
}

func (n NodeRef) FirstChild() NodeRef {
	if p := n.get(); p != nil {
		return n.rel(p.firstChild)
	}
	return NodeRef{}
}

func (n NodeRef) NextSibling() NodeRef {
	if p := n.get(); p != nil {
		return n.rel(p.nextSibling)
	}
	return NodeRef{}
}

func (n NodeRef) PreviousSibling() NodeRef {
	if p := n.get(); p != nil {
		return n.rel(p.prevSibling)
	}
	return NodeRef{}
}

// ChildKind returns the first child of kind k.
func (n NodeRef) ChildKind(k Kind) NodeRef {
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == k {
			return c
		}
	}
	return NodeRef{}
}

// Child returns the first child whose kind is named name.
func (n NodeRef) Child(name string) NodeRef {
	k := KindForName(name)
	if k == KindUnknown {
		return NodeRef{}
	}
	return n.ChildKind(k)
}

// IsLeaf reports whether n is a token.
func (n NodeRef) IsLeaf() bool {
	p := n.get()
	return p != nil && p.leaf
}

// IsTrivia reports whether n is whitespace or a comment.
func (n NodeRef) IsTrivia() bool {
	p := n.get()
	return p != nil && p.leaf && (p.kind == KindWHITE_SPACE || commentTokens.has(p.kind))
}

// ContainingFile returns the node's snapshot.
func (n NodeRef) ContainingFile() *Snapshot {
	if !n.Valid() {
		return nil
	}
	return n.s
}

// ---- typed views ----

// declarationKinds are the element types whose PSI is a KtDeclaration.
var declarationKinds = newSet(KindCLASS, KindOBJECT_DECLARATION, KindFUN, KindPROPERTY, KindTYPEALIAS,
	KindENUM_ENTRY, KindCLASS_INITIALIZER, KindSCRIPT_INITIALIZER, KindSECONDARY_CONSTRUCTOR,
	KindPRIMARY_CONSTRUCTOR, KindPROPERTY_ACCESSOR, KindVALUE_PARAMETER, KindTYPE_PARAMETER,
	KindDESTRUCTURING_DECLARATION, KindDESTRUCTURING_DECLARATION_ENTRY, KindFUNCTION_LITERAL,
	KindBACKING_FIELD, KindSCRIPT)

// functionKinds are the element types whose PSI is a KtFunction.
var functionKinds = newSet(KindFUN, KindFUNCTION_LITERAL, KindPRIMARY_CONSTRUCTOR, KindSECONDARY_CONSTRUCTOR)

// Declaration returns n if it is a declaration.
func (n NodeRef) Declaration() NodeRef {
	if declarationKinds.has(n.NumericKind()) {
		return n
	}
	return NodeRef{}
}

// Function returns n if it is a function, lambda literal or constructor.
func (n NodeRef) Function() NodeRef {
	if functionKinds.has(n.NumericKind()) {
		return n
	}
	return NodeRef{}
}

// Import returns n if it is an import directive.
func (n NodeRef) Import() NodeRef {
	if n.NumericKind() == KindIMPORT_DIRECTIVE {
		return n
	}
	return NodeRef{}
}

// Element returns n itself; typed views share NodeRef's representation.
func (n NodeRef) Element() NodeRef { return n }

// NameIdentifier returns the IDENTIFIER child.
func (n NodeRef) NameIdentifier() NodeRef { return n.ChildKind(KindIDENTIFIER) }

// Name returns the name identifier's text without backticks, or "". Like
// KtObjectDeclaration.getName, an unnamed companion object is "Companion".
func (n NodeRef) Name() string {
	id := n
	if !n.IsLeaf() {
		id = n.NameIdentifier()
	}
	if id.NumericKind() != KindIDENTIFIER {
		if n.NumericKind() == KindOBJECT_DECLARATION && n.ChildKind(KindMODIFIER_LIST).ChildKind(KindCOMPANION_KEYWORD).Valid() {
			return "Companion"
		}
		return ""
	}
	return unquote(id.Text())
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1]
	}
	return s
}

// TypeReference returns the declared type: the TYPE_REFERENCE after the
// colon of a function or property, else the first TYPE_REFERENCE child.
func (n NodeRef) TypeReference() NodeRef {
	switch n.NumericKind() {
	case KindFUN, KindPROPERTY:
		return n.ReturnType()
	}
	return n.ChildKind(KindTYPE_REFERENCE)
}

// ReturnType returns the TYPE_REFERENCE child after a COLON.
func (n NodeRef) ReturnType() NodeRef {
	after := false
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case KindCOLON:
			after = true
		case KindTYPE_REFERENCE:
			if after {
				return c
			}
		}
	}
	return NodeRef{}
}

// ReceiverType returns a TYPE_REFERENCE child before the parameters and
// before any colon: an extension receiver.
func (n NodeRef) ReceiverType() NodeRef {
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case KindVALUE_PARAMETER_LIST, KindCOLON, KindEQ, KindBLOCK:
			return NodeRef{}
		case KindTYPE_REFERENCE:
			return c
		}
	}
	return NodeRef{}
}

// Body returns the BLOCK child, or the expression after '='.
func (n NodeRef) Body() NodeRef {
	if b := n.ChildKind(KindBLOCK); b.Valid() {
		return b
	}
	after := false
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == KindEQ {
			after = true
		} else if after && !c.IsLeaf() && c.NumericKind() != KindERROR_ELEMENT {
			return c
		}
	}
	return NodeRef{}
}

// Parameters returns the VALUE_PARAMETERs of the VALUE_PARAMETER_LIST child.
func (n NodeRef) Parameters() []NodeRef {
	var out []NodeRef
	for c := n.ChildKind(KindVALUE_PARAMETER_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == KindVALUE_PARAMETER {
			out = append(out, c)
		}
	}
	return out
}

// Path returns the qualified name of an import or package directive,
// without backticks and without a trailing ".*".
func (n NodeRef) Path() string {
	var parts []string
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if k := c.NumericKind(); k == KindDOT_QUALIFIED_EXPRESSION || k == KindREFERENCE_EXPRESSION {
			WalkNodes(c, func(d NodeRef) bool {
				if d.NumericKind() == KindREFERENCE_EXPRESSION {
					parts = append(parts, d.Name())
					return false
				}
				return true
			})
			break
		}
	}
	return strings.Join(parts, ".")
}

// Alias returns an import's alias name, or "".
func (n NodeRef) Alias() string { return n.ChildKind(KindIMPORT_ALIAS).Name() }

// IsAllUnder reports whether an import ends in ".*".
func (n NodeRef) IsAllUnder() bool { return n.ChildKind(KindMUL).Valid() }

// WalkNodes visits root's subtree in preorder, including root and trivia.
// When fn returns false the node's children are skipped.
func WalkNodes(root NodeRef, fn func(NodeRef) bool) {
	if !root.Valid() {
		return
	}
	s := root.s
	end := s.nodes[root.id].subtreeEnd
	for id := root.id; id < end; {
		if fn(NodeRef{s, id}) {
			id++
		} else {
			id = s.nodes[id].subtreeEnd
		}
	}
}
