package ide

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Hover describes what the caret at off of path is on, as Markdown: a
// declaration in the sources by its header, KDoc and container; a
// library function by its call; a variable that a smart cast makes
// non-null with a note. rng is the token hovered.
func Hover(v *View, path string, off int) (md string, rng psi.Range, ok bool) {
	t, ok := v.at(path, off)
	if !ok {
		return "", psi.Range{}, false
	}
	rng = caretLeaf(v.byPath[path], off).Range()
	var sig, doc, where string
	switch {
	case t.decl.Valid():
		d := t.decl
		var sym *analysis.Symbol
		if f, ok := v.fact(d); ok {
			if s, ok := v.symbol(f.Symbol); ok {
				sym = &s
			}
		}
		sig, doc, where = header(d, sym), KDoc(d), container(d)
	case len(t.fact.Accessors) > 0:
		sig = renderProperty(t.use.Text(), t.fact.Accessors)
	case t.fact.Call != nil:
		sig = renderCall(*t.fact.Call)
	case t.use.NumericKind() != psi.KindREFERENCE_EXPRESSION:
		// An operator without a call, on primitives.
	default:
		if s, ok := v.symbol(t.fact.Symbol); ok {
			sig = renderSymbol(s)
		} else if t.fact.Type != nil && t.fact.Role == "reference" {
			sig = t.use.Text() + ": " + renderType(*t.fact.Type) // an intrinsic: String.length
		} else if t.fact.Type != nil {
			sig = renderType(*t.fact.Type)
		}
	}
	if sig == "" {
		return "", psi.Range{}, false
	}
	parts := []string{"```kotlin\n" + sig + "\n```"}
	if t.use.Valid() && t.fact.FlowNonNull && t.fact.Type != nil {
		nn := *t.fact.Type
		nn.Nullable, nn.MarkedNullable, nn.Platform = false, false, false
		parts = append(parts, "Smart cast to `"+renderType(nn)+"`")
	}
	for _, p := range []string{doc, where} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "\n\n"), rng, true
}

// container names what declaration d is a member of: its class or
// object, or its file's package; "" for a local declaration, a
// parameter of a function; a type parameter's, its declaration.
func container(d psi.NodeRef) string {
	if d.NumericKind() == psi.KindTYPE_PARAMETER {
		if name := d.Parent().Parent().Name(); name != "" {
			return "type parameter of `" + name + "`"
		}
		return ""
	}
	for p := d.Parent(); p.Valid(); p = p.Parent() {
		switch p.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
			if name := p.Name(); name != "" {
				return "member of `" + name + "`"
			}
			return ""
		case psi.KindFUN, psi.KindSECONDARY_CONSTRUCTOR, psi.KindFUNCTION_LITERAL, psi.KindPROPERTY_ACCESSOR,
			psi.KindCLASS_INITIALIZER, psi.KindBLOCK:
			return ""
		}
	}
	for c := d.ContainingFile().RootNode().FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindPACKAGE_DIRECTIVE {
			if name := strings.TrimSpace(strings.TrimPrefix(compact(c.Text()), "package")); name != "" {
				return "package `" + name + "`"
			}
		}
	}
	return ""
}
