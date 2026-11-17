package ide

import (
	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// The navigation features answer from views. Each takes a caret, an
// offset of the view's snapshot of path, and returns locations of the
// views' snapshots. Only declarations in the sources are found: the
// library's are reached from L6, with its sources.

// Definition returns the name of the declaration the caret at off is on,
// or uses.
func Definition(v *View, path string, off int) []Loc {
	t, ok := v.at(path, off)
	if !ok || !t.decl.Valid() {
		return nil
	}
	return []Loc{{t.decl.ContainingFile(), nameRange(t.decl)}}
}

// TypeDefinition returns the name of the class of the type of what the
// caret at off is on: an expression's, or a variable's or function's
// declared one.
func TypeDefinition(v *View, path string, off int) []Loc {
	t, ok := v.at(path, off)
	if !ok {
		return nil
	}
	var typ *analysis.Type
	switch {
	case t.fact.Call != nil && t.fact.Call.ReturnType != nil && t.use.Valid():
		typ = t.fact.Call.ReturnType // a callee's: its call's
	case t.fact.Type != nil && t.use.Valid():
		typ = t.fact.Type // a type's, an expression's or a reference's
	case t.decl.Valid():
		if f, ok := v.fact(t.decl); ok {
			if s, ok := v.symbol(f.Symbol); ok {
				typ = &s.Type
			}
		}
	}
	if typ == nil {
		return nil
	}
	var d psi.NodeRef
	switch {
	case typ.Parameter && typ.ParameterOwner != nil:
		d = typeParameter(v.node(*typ.ParameterOwner), typ.Name)
	default:
		v.classIndex()
		if el, ok := v.classes[typ.Name]; ok {
			d = v.node(el)
		}
	}
	if !d.Valid() {
		return nil
	}
	return []Loc{{d.ContainingFile(), nameRange(d)}}
}

// References returns the uses of the declaration the caret at off of
// views[0] is on, or uses, in every view that analyzed its file, and its
// name if decl is set.
func References(views []*View, path string, off int, decl bool) []Loc {
	if len(views) == 0 {
		return nil
	}
	t, ok := views[0].at(path, off)
	if !ok || !t.decl.Valid() {
		return nil
	}
	k, from := keyOf(t.decl), t.decl.ContainingFile()
	var out []Loc
	for _, v := range views {
		d, ok := v.find(k, from)
		if !ok {
			continue
		}
		for _, r := range v.uses(d) {
			out = append(out, Loc{r.snap, r.name})
		}
		if decl {
			out = append(out, Loc{d.ContainingFile(), nameRange(d)})
		}
	}
	return sortLocs(out)
}

// HighlightKind says whether a highlighted name is read or written.
type HighlightKind int

const (
	Read HighlightKind = iota + 1
	Write
)

// Highlight is a name of the declaration under the caret in its file.
type Highlight struct {
	Range psi.Range
	Kind  HighlightKind
}

// Highlights returns the names of the declaration the caret at off is
// on, or uses, in path's snapshot: its uses and its declaration, which is
// a write, as an assignment is.
func Highlights(v *View, path string, off int) []Highlight {
	t, ok := v.at(path, off)
	if !ok || !t.decl.Valid() {
		return nil
	}
	snap := v.Snapshot(path)
	var out []Highlight
	if t.decl.ContainingFile() == snap {
		out = append(out, Highlight{nameRange(t.decl), Write})
	}
	for _, r := range v.uses(t.decl) {
		if r.snap != snap {
			continue
		}
		k := Read
		if written(snap.Node(r.node)) {
			k = Write
		}
		out = append(out, Highlight{r.name, k})
	}
	return out
}

// written reports whether reference n is assigned: the left of '=' or of
// a compound assignment, or the operand of '++' or '--'.
func written(n psi.NodeRef) bool {
	if n.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		return false
	}
	x := n
	for p := x.Parent(); p.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION || p.NumericKind() == psi.KindSAFE_ACCESS_EXPRESSION; p = x.Parent() {
		if lastNonTrivia(p) != x {
			break
		}
		x = p
	}
	p := x.Parent()
	op := p.ChildKind(psi.KindOPERATION_REFERENCE)
	switch p.NumericKind() {
	case psi.KindBINARY_EXPRESSION:
		if firstNonTrivia(p) != x {
			return false
		}
		switch op.Text() {
		case "=", "+=", "-=", "*=", "/=", "%=":
			return true
		}
	case psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION:
		return op.Text() == "++" || op.Text() == "--"
	}
	return false
}

func firstNonTrivia(n psi.NodeRef) psi.NodeRef {
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsTrivia() {
			return c
		}
	}
	return psi.NodeRef{}
}

func lastNonTrivia(n psi.NodeRef) psi.NodeRef {
	var last psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsTrivia() {
			last = c
		}
	}
	return last
}
