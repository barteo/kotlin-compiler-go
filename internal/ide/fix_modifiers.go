package ide

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// Quick fixes that add or remove a modifier, an annotation, a return or
// an initializer for the checker's errors that ask for one.

// viewSnap returns the view's snapshot of the fixer's document, whose
// facts the view has, if the document is unchanged since.
func (f fixer) viewSnap() *psi.Snapshot {
	if f.v == nil {
		return nil
	}
	s := f.v.byPath[f.snap.Path()]
	if s == nil || s.Source() != f.snap.Source() {
		return nil
	}
	return s
}

// addReturn returns the last statement of a function with a block body
// that lacks a return, when it is an expression of the declared type.
func (f fixer) addReturn() []Fix {
	vs := f.viewSnap()
	if vs == nil {
		return nil
	}
	fn := vs.NodeAt(f.d.Range.Start)
	for fn.Valid() && fn.NumericKind() != psi.KindFUN {
		fn = fn.Parent()
	}
	body, tr := fn.Body(), fn.TypeReference()
	if body.NumericKind() != psi.KindBLOCK || !tr.Valid() {
		return nil
	}
	last := lastExpression(body)
	switch last.NumericKind() {
	case psi.KindPROPERTY, psi.KindFUN, psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS, psi.KindWHILE, psi.KindFOR,
		psi.KindDO_WHILE, psi.KindRETURN, psi.KindTHROW, psi.KindBREAK, psi.KindCONTINUE:
		return nil
	case psi.KindBINARY_EXPRESSION:
		if assignment(last) {
			return nil
		}
	}
	fact, ok := f.v.fact(last)
	if !ok || fact.Type == nil || fact.Type.Name == "" {
		return nil
	}
	want, got := compact(tr.Text()), renderType(*fact.Type)
	if got != want && got+"?" != want {
		return nil
	}
	at := psi.Range{Start: last.Range().Start, End: last.Range().Start}
	return f.fix("Add 'return' before the expression", true, f.edit(at, "return "))
}

// assignment reports whether a binary expression is an assignment, plain
// or compound.
func assignment(n psi.NodeRef) bool {
	switch n.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() {
	case psi.KindEQ, psi.KindPLUSEQ, psi.KindMINUSEQ, psi.KindMULTEQ, psi.KindDIVEQ, psi.KindPERCEQ:
		return true
	}
	return false
}

// defaultValue returns the default a variable of declared type t starts
// with: zero, false, an empty string, null; "" if t has none.
func defaultValue(t string) string {
	switch t = compact(t); {
	case strings.HasSuffix(t, "?"):
		return "null"
	case t == "Int" || t == "Long" || t == "Short" || t == "Byte" || t == "kotlin.Int":
		return "0"
	case t == "Double":
		return "0.0"
	case t == "Float":
		return "0f"
	case t == "Boolean":
		return "false"
	case t == "String":
		return `""`
	case t == "Char":
		return "' '"
	}
	return ""
}

// initializeVariable initializes the local variable a use reads before
// it is assigned with its type's default, when it has a default and the
// variable is a var or never assigned.
func (f fixer) initializeVariable() []Fix {
	if len(f.d.Data) == 0 {
		return nil
	}
	name := f.d.Data[0]
	use := f.snap.NodeAt(f.d.Range.Start)
	for b := use; b.Valid(); b = b.Parent() {
		if b.NumericKind() != psi.KindBLOCK {
			continue
		}
		for p := b.FirstChild(); p.Valid() && p.Range().Start < use.Range().Start; p = p.NextSibling() {
			if p.NumericKind() != psi.KindPROPERTY || p.Name() != name {
				continue
			}
			tr := p.TypeReference()
			if !tr.Valid() || p.ChildKind(psi.KindEQ).Valid() || p.ChildKind(psi.KindPROPERTY_DELEGATE).Valid() {
				return nil
			}
			init := defaultValue(tr.Text())
			if init == "" || p.ChildKind(psi.KindVAL_KEYWORD).Valid() && assigns(b, name) {
				return nil
			}
			end := tr.Range().End
			return f.fix("Initialize with default value", true, f.edit(psi.Range{Start: end, End: end}, " = "+init))
		}
	}
	return nil
}

// assigns reports whether n holds an assignment to name.
func assigns(n psi.NodeRef, name string) bool {
	found := false
	psi.WalkNodes(n, func(m psi.NodeRef) bool {
		if m.NumericKind() == psi.KindBINARY_EXPRESSION && assignment(m) {
			if l := firstNonTrivia(m); l.NumericKind() == psi.KindREFERENCE_EXPRESSION && psi.Unquote(l.Text()) == name {
				found = true
			}
		}
		return !found
	})
	return found
}

// sourceDecl returns the declaration in the sources a reference names, by
// its fact, with the snapshot it is in.
func (f fixer) sourceDecl(ref psi.NodeRef) (psi.NodeRef, *psi.Snapshot) {
	fact, ok := f.v.fact(ref)
	if !ok {
		return psi.NodeRef{}, nil
	}
	s, ok := f.v.symbol(fact.Symbol)
	if !ok || s.Declaration == nil {
		return psi.NodeRef{}, nil
	}
	n := f.v.node(*s.Declaration)
	if !n.Valid() {
		return psi.NodeRef{}, nil
	}
	return n, f.v.snaps[s.Declaration.Snapshot]
}

// sourceClass returns the class of the sources of simple name name, if
// one only has it.
func (f fixer) sourceClass(name string) (psi.NodeRef, *psi.Snapshot) {
	f.v.classIndex()
	var found []string
	for fq := range f.v.classes {
		if fq == name || strings.HasSuffix(fq, "."+name) {
			found = append(found, fq)
		}
	}
	if len(found) != 1 {
		return psi.NodeRef{}, nil
	}
	el := f.v.classes[found[0]]
	return f.v.node(el), f.v.snaps[el.Snapshot]
}

// makeSupertypeOpen makes the final class of the sources a class
// inherits from open.
func (f fixer) makeSupertypeOpen() []Fix {
	vs := f.viewSnap()
	if vs == nil {
		return nil
	}
	ref := vs.NodeAt(f.d.Range.Start)
	for ref.Valid() && ref.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		ref = ref.Parent()
	}
	cls, snap := f.sourceDecl(ref)
	if !cls.Valid() {
		cls, snap = f.sourceClass(ref.Text())
	}
	if cls.NumericKind() != psi.KindCLASS || snap == nil {
		return nil
	}
	for _, k := range []psi.Kind{psi.KindDATA_KEYWORD, psi.KindENUM_KEYWORD, psi.KindVALUE_KEYWORD, psi.KindINLINE_KEYWORD, psi.KindANNOTATION_KEYWORD} {
		if hasModifier(cls, k) {
			return nil
		}
	}
	e, ok := openEdit(cls, cls.ChildKind(psi.KindCLASS_KEYWORD))
	if !ok {
		return nil
	}
	return f.fix("Make '"+cls.Name()+"' open", true, FixEdit{Loc{snap, e.Range}, e.Text})
}

// openEdit returns the edit that makes declaration d, whose keyword is
// kw, open: replacing its final modifier, or adding open before kw.
func openEdit(d, kw psi.NodeRef) (TextEdit, bool) {
	if !kw.Valid() {
		return TextEdit{}, false
	}
	if fin := d.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindFINAL_KEYWORD); fin.Valid() {
		if hasModifier(d, psi.KindOVERRIDE_KEYWORD) {
			src := d.ContainingFile().Source()
			return TextEdit{Range: withSpaceBefore(src, fin.Range())}, true // an override is open
		}
		return TextEdit{Range: fin.Range(), Text: "open"}, true
	}
	at := kw.Range().Start
	return TextEdit{Range: psi.Range{Start: at, End: at}, Text: "open "}, true
}

// makeMemberOpen makes the final member of a class of the sources that a
// member overrides open: OVERRIDING_FINAL_MEMBER's data are the member's
// and the class's names.
func (f fixer) makeMemberOpen() []Fix {
	if f.v == nil || len(f.d.Data) < 2 {
		return nil
	}
	name, owner := f.d.Data[0], f.d.Data[1]
	cls, snap := f.sourceClass(owner)
	if cls.NumericKind() != psi.KindCLASS || snap == nil {
		return nil
	}
	var member, kw psi.NodeRef
	for c := cls.ChildKind(psi.KindCLASS_BODY).FirstChild(); c.Valid(); c = c.NextSibling() {
		if (c.NumericKind() == psi.KindFUN || c.NumericKind() == psi.KindPROPERTY) && c.Name() == name {
			if member.Valid() {
				return nil // overloads
			}
			member = c
		}
	}
	for _, p := range cls.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).Parameters() {
		if p.Name() == name && (p.ChildKind(psi.KindVAL_KEYWORD).Valid() || p.ChildKind(psi.KindVAR_KEYWORD).Valid()) {
			if member.Valid() {
				return nil
			}
			member = p
		}
	}
	for _, k := range []psi.Kind{psi.KindFUN_KEYWORD, psi.KindVAL_KEYWORD, psi.KindVAR_KEYWORD} {
		if kw = member.ChildKind(k); kw.Valid() {
			break
		}
	}
	e, ok := openEdit(member, kw)
	if !ok {
		return nil
	}
	return f.fix("Make '"+owner+"."+name+"' open", true, FixEdit{Loc{snap, e.Range}, e.Text})
}

// inlinable reports whether function fn may be made inline: it isn't
// open, abstract or an override, nor an interface's member.
func inlinable(fn psi.NodeRef) bool {
	for _, k := range []psi.Kind{psi.KindOPEN_KEYWORD, psi.KindABSTRACT_KEYWORD, psi.KindOVERRIDE_KEYWORD, psi.KindEXTERNAL_KEYWORD} {
		if hasModifier(fn, k) {
			return false
		}
	}
	if body := fn.Parent(); body.NumericKind() == psi.KindCLASS_BODY {
		return !body.Parent().ChildKind(psi.KindINTERFACE_KEYWORD).Valid()
	}
	return fn.Body().Valid()
}

// addInline makes the function whose reified type parameter is reported
// inline.
func (f fixer) addInline() []Fix {
	tp := f.declaration(psi.KindTYPE_PARAMETER)
	fn := tp.Parent().Parent()
	if fn.NumericKind() != psi.KindFUN || !inlinable(fn) || hasModifier(fn, psi.KindINLINE_KEYWORD) {
		return nil
	}
	at := fn.ChildKind(psi.KindFUN_KEYWORD).Range().Start
	return f.fix("Add 'inline' modifier", true, f.edit(psi.Range{Start: at, End: at}, "inline "))
}

// makeReified makes the type parameter of the enclosing function that a
// call passes as a reified type argument reified, and the function
// inline if it isn't.
func (f fixer) makeReified() []Fix {
	if len(f.d.Data) == 0 {
		return nil
	}
	name := f.d.Data[0]
	for fn := f.snap.NodeAt(f.d.Range.Start); fn.Valid(); fn = fn.Parent() {
		switch fn.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindPROPERTY:
			for tp := fn.ChildKind(psi.KindTYPE_PARAMETER_LIST).FirstChild(); tp.Valid(); tp = tp.NextSibling() {
				if tp.NumericKind() == psi.KindTYPE_PARAMETER && tp.Name() == name {
					return nil // a class's or a property's, which can't be reified
				}
			}
			continue
		case psi.KindFUN:
		default:
			continue
		}
		for tp := fn.ChildKind(psi.KindTYPE_PARAMETER_LIST).FirstChild(); tp.Valid(); tp = tp.NextSibling() {
			if tp.NumericKind() != psi.KindTYPE_PARAMETER || tp.Name() != name {
				continue
			}
			if hasModifier(tp, psi.KindREIFIED_KEYWORD) || !inlinable(fn) {
				return nil
			}
			at := tp.NameIdentifier().Range().Start
			edits := []FixEdit{f.edit(psi.Range{Start: at, End: at}, "reified ")}
			if !hasModifier(fn, psi.KindINLINE_KEYWORD) {
				kw := fn.ChildKind(psi.KindFUN_KEYWORD).Range().Start
				edits = append([]FixEdit{f.edit(psi.Range{Start: kw, End: kw}, "inline ")}, edits...)
			}
			return f.fix("Make '"+name+"' reified", true, edits...)
		}
	}
	return nil
}

// removeAnnotation removes the annotation reported: the entry the
// report starts at, or the declaration's entry of name if the report is
// on the declaration.
func (f fixer) removeAnnotation(name string) []Fix {
	var entry psi.NodeRef
	for n := f.snap.NodeAt(f.d.Range.Start); n.Valid() && !entry.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindANNOTATION_ENTRY:
			entry = n
		case psi.KindFUN, psi.KindPROPERTY, psi.KindCLASS, psi.KindOBJECT_DECLARATION:
			if name == "" {
				return nil
			}
			for a := n.ChildKind(psi.KindMODIFIER_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
				if a.NumericKind() == psi.KindANNOTATION_ENTRY && annotationName(a) == name {
					entry = a
				}
			}
			if !entry.Valid() {
				return nil
			}
		}
	}
	if !entry.Valid() || entry.Parent().NumericKind() == psi.KindANNOTATION {
		return nil // one of @target:[A B]
	}
	src := f.snap.Source()
	r := lineRange(src, entry.Range())
	if r == entry.Range() {
		for r.End < len(src) && (src[r.End] == ' ' || src[r.End] == '\t') {
			r.End++
		}
	}
	return f.fix("Remove '@"+annotationName(entry)+"' annotation", true, f.edit(r, ""))
}
