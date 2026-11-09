package ide

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Navigation into the library: what a use names there, for
// internal/libsrc to locate in the library's sources.

// LibraryTarget returns the declaration of the library that the caret at
// off uses: a function, constructor, property or class not declared in
// the sources.
func LibraryTarget(v *View, path string, off int) (libsrc.Target, bool) {
	t, ok := v.at(path, off)
	if !ok || t.decl.Valid() || !t.use.Valid() {
		return libsrc.Target{}, false
	}
	return libraryTarget(v, t.fact, t.use)
}

// LibraryTypeTarget returns the class of the library of the type of what
// the caret at off is on, as TypeDefinition's.
func LibraryTypeTarget(v *View, path string, off int) (libsrc.Target, bool) {
	t, ok := v.at(path, off)
	if !ok {
		return libsrc.Target{}, false
	}
	var typ *analysis.Type
	switch {
	case t.fact.Call != nil && t.fact.Call.ReturnType != nil && t.use.Valid():
		typ = t.fact.Call.ReturnType
	case t.fact.Type != nil && t.use.Valid():
		typ = t.fact.Type
	case t.decl.Valid():
		if f, ok := v.fact(t.decl); ok {
			if s, ok := v.symbol(f.Symbol); ok {
				typ = &s.Type
			}
		}
	}
	if typ == nil || typ.Parameter || typ.Name == "" {
		return libsrc.Target{}, false
	}
	v.classIndex()
	if _, ok := v.classes[typ.Name]; ok {
		return libsrc.Target{}, false // the sources'
	}
	return libsrc.Target{Kind: "class", Name: typ.Name}, true
}

func libraryTarget(v *View, f analysis.Fact, n psi.NodeRef) (libsrc.Target, bool) {
	if s, ok := v.symbol(f.Symbol); ok {
		if s.Declaration != nil {
			return libsrc.Target{}, false
		}
		var t libsrc.Target
		switch s.Kind {
		case "class":
			t = libsrc.Target{Kind: "class", Name: s.QualifiedName, Binary: s.Name}
			if t.Name == "" {
				t.Name = strings.NewReplacer("/", ".", "$", ".").Replace(s.Name)
			}
		case "function", "property", "constructor":
			t = libsrc.Target{Kind: s.Kind, Name: strings.TrimSuffix(s.QualifiedName, ".<init>"), Binary: s.Owner}
		default:
			return libsrc.Target{}, false
		}
		withCall(&t, f.Call)
		return t, t.Name != ""
	}
	if c := f.Call; c != nil {
		if c.Declaration != nil || c.Name == "" {
			return libsrc.Target{}, false
		}
		t := libsrc.Target{Kind: "function", Name: c.Name}
		if name, ok := strings.CutSuffix(c.Name, ".<init>"); ok {
			t.Kind, t.Name = "constructor", name
		}
		withCall(&t, c)
		return t, true
	}
	if f.Type == nil || f.Type.Parameter {
		return libsrc.Target{}, false
	}
	v.classIndex()
	switch n.NumericKind() {
	case psi.KindUSER_TYPE:
		if _, ok := v.classes[f.Type.Name]; ok {
			return libsrc.Target{}, false
		}
		return libsrc.Target{Kind: "class", Name: f.Type.Name}, true
	case psi.KindREFERENCE_EXPRESSION:
		if f.Classifier {
			return libsrc.Target{Kind: "class", Name: f.Type.Name}, true
		}
		// A property no symbol names, x.length: a member of the
		// receiver's class.
		p := n.Parent()
		if k := p.NumericKind(); (k == psi.KindDOT_QUALIFIED_EXPRESSION || k == psi.KindSAFE_ACCESS_EXPRESSION) && p.FirstChild() != n {
			for r := p.FirstChild(); r.Valid(); r = r.NextSibling() {
				if r.IsTrivia() || r.IsLeaf() {
					continue
				}
				rf, ok := v.fact(r)
				if !ok || rf.Type == nil || rf.Type.Parameter {
					break
				}
				return libsrc.Target{Kind: "property", Name: rf.Type.Name + "." + psi.Unquote(n.Text())}, true
			}
		}
	}
	return libsrc.Target{}, false
}

// withCall adds a call's parameters and receiver to t.
func withCall(t *libsrc.Target, c *analysis.Call) {
	if c == nil {
		return
	}
	t.Params = true
	for i, p := range c.Parameters {
		t.Parameters = append(t.Parameters, simpleName(p.Name))
		t.Varargs = append(t.Varargs, i < len(c.Varargs) && c.Varargs[i])
	}
	if r := c.DeclaredReceiver; r != nil && c.Extension {
		t.Receiver = simpleName(r.Name)
	}
}
