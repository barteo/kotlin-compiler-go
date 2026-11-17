package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// A TypeItem is a class, interface or object in a type hierarchy, of the
// sources or of the library's sources or stubs.
type TypeItem struct {
	CallItem
	Qualified string // its fully qualified name
}

// LibraryClass returns the node of a class of the library, by its fully
// qualified name: in its sources, or its stub; nil if there are none.
type LibraryClass func(libsrc.Target) (psi.NodeRef, bool)

// PrepareTypeHierarchy returns the item of the class the caret at off of
// path is on, or names: one of the sources of views, the first of which
// has path, or one of the library's that lib finds.
func PrepareTypeHierarchy(views []*View, path string, off int, lib LibraryClass) (TypeItem, bool) {
	v := views[0]
	t, ok := v.at(path, off)
	if !ok {
		return TypeItem{}, false
	}
	if t.decl.Valid() {
		return typeItem(classOf(t.decl))
	}
	if !t.use.Valid() {
		return TypeItem{}, false
	}
	var name string
	if target, ok := libraryTarget(v, t.fact, t.use); ok {
		if target.Kind != "class" && target.Kind != "constructor" {
			return TypeItem{}, false
		}
		name = target.Name
	} else if t.fact.Type != nil && !t.fact.Type.Parameter && t.use.NumericKind() != psi.KindCALL_EXPRESSION {
		name = t.fact.Type.Name
	}
	if name == "" {
		return TypeItem{}, false
	}
	// Another module's class: its view has it.
	if d := findClass(views, name); d.Valid() {
		return typeItem(d)
	}
	if lib == nil {
		return TypeItem{}, false
	}
	return libraryItem(name, lib)
}

// findClass returns the source class called qualified that one of views
// has.
func findClass(views []*View, qualified string) psi.NodeRef {
	for _, v := range views {
		if v == nil {
			continue
		}
		v.classIndex()
		if el, ok := v.classes[qualified]; ok {
			if d := v.node(el); d.Valid() {
				return d
			}
		}
	}
	return psi.NodeRef{}
}

// TypeDecl returns the class whose name starts at off of path.
func TypeDecl(v *View, path string, off int) psi.NodeRef {
	return classOf(callTarget(v, path, off))
}

// classOf returns the class a declaration is, or its constructor's.
func classOf(d psi.NodeRef) psi.NodeRef {
	switch d.NumericKind() {
	case psi.KindPRIMARY_CONSTRUCTOR:
		return d.Parent()
	case psi.KindSECONDARY_CONSTRUCTOR:
		return d.Parent().Parent()
	case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
		return d
	}
	return psi.NodeRef{}
}

func typeItem(d psi.NodeRef) (TypeItem, bool) {
	if !d.Valid() {
		return TypeItem{}, false
	}
	q := declQualifiedName(d)
	it, ok := callItem(d)
	if !ok || q == "" {
		return TypeItem{}, false
	}
	return TypeItem{it, q}, true
}

func libraryItem(name string, lib LibraryClass) (TypeItem, bool) {
	d, ok := lib(libsrc.Target{Kind: "class", Name: name})
	if !ok || (d.NumericKind() != psi.KindCLASS && d.NumericKind() != psi.KindOBJECT_DECLARATION) {
		return TypeItem{}, false
	}
	it, ok := typeItem(d)
	if ok {
		it.Qualified = name // the stub's or the sources' file may nest it otherwise
	}
	return it, ok
}

// Supertypes returns the direct supertypes of class d, of the sources of
// views or of the library: kotlin.Any for a class that names none. A
// source class's are as its module's facts resolve them; a library
// class's by its file's imports and package, and the default imports.
func Supertypes(views []*View, d psi.NodeRef, lib LibraryClass) []TypeItem {
	exists := func(q string) bool {
		if findClass(views, q).Valid() {
			return true
		}
		if lib == nil {
			return false
		}
		_, ok := lib(libsrc.Target{Kind: "class", Name: q})
		return ok
	}
	names := superNames(views, d, exists)
	if len(names) == 0 && declQualifiedName(d) != "kotlin.Any" && !d.ChildKind(psi.KindINTERFACE_KEYWORD).Valid() {
		names = []string{"kotlin.Any"}
	}
	var out []TypeItem
	for _, q := range names {
		if c := findClass(views, q); c.Valid() {
			if it, ok := typeItem(c); ok {
				out = append(out, it)
			}
			continue
		}
		if lib != nil {
			if it, ok := libraryItem(q, lib); ok {
				out = append(out, it)
			}
		}
	}
	return out
}

// superNames returns the qualified names of the types d's supertype list
// names, as the first of views with facts of them resolves them, else by
// the syntax, with exists telling which candidate classes there are.
func superNames(views []*View, d psi.NodeRef, exists func(string) bool) []string {
	var out []string
	for e := d.ChildKind(psi.KindSUPER_TYPE_LIST).FirstChild(); e.Valid(); e = e.NextSibling() {
		var tr psi.NodeRef
		switch e.NumericKind() {
		case psi.KindSUPER_TYPE_ENTRY, psi.KindDELEGATED_SUPER_TYPE_ENTRY:
			tr = e.ChildKind(psi.KindTYPE_REFERENCE)
		case psi.KindSUPER_TYPE_CALL_ENTRY:
			tr = e.ChildKind(psi.KindCONSTRUCTOR_CALLEE).ChildKind(psi.KindTYPE_REFERENCE)
		default:
			continue
		}
		if !tr.Valid() {
			continue
		}
		if q := factTypeName(views, tr); q != "" {
			out = append(out, q)
		} else if q := resolveTypeName(d.ContainingFile(), typeName0(tr), exists); q != "" {
			out = append(out, q)
		}
	}
	return out
}

// factTypeName returns the class type reference tr names, by the first
// of views with a fact of it.
func factTypeName(views []*View, tr psi.NodeRef) string {
	for _, v := range views {
		if v == nil || v.Snapshot(tr.ContainingFile().Path()) != tr.ContainingFile() {
			continue
		}
		if f, ok := v.fact(tr); ok && f.Type != nil && f.Type.Name != "" && !f.Type.Parameter {
			return f.Type.Name
		}
	}
	return ""
}

// typeName0 returns the dotted name a type reference writes, without
// type arguments and nullability.
func typeName0(tr psi.NodeRef) string {
	var b strings.Builder
	depth := 0
	for _, c := range tr.Text() {
		switch {
		case c == '<':
			depth++
		case c == '>':
			depth--
		case depth == 0 && (c == '.' || c == '_' || c == '$' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c > 127):
			b.WriteRune(c)
		}
	}
	return strings.Trim(b.String(), ".")
}

// resolveTypeName resolves a type's name as file snap writes it, by the
// syntax alone: an import of its first segment, else its package's class
// if exists says there is one, else a class the default imports are
// known to have, else its package's class; "" if the name is empty.
// exists may be nil.
func resolveTypeName(snap *psi.Snapshot, name string, exists func(string) bool) string {
	if name == "" {
		return ""
	}
	first, rest, _ := strings.Cut(name, ".")
	for _, i := range imports(snap) {
		if !i.star && (i.alias == first || i.alias == "" && i.local() == first) {
			if rest != "" {
				return i.path + "." + rest
			}
			return i.path
		}
	}
	if first[0] >= 'a' && first[0] <= 'z' {
		return name // already qualified
	}
	local := name
	if pkg := packageName(snap); pkg != "" {
		local = pkg + "." + name
	}
	if exists != nil && exists(local) {
		return local
	}
	for _, p := range defaultOrder {
		if slices.Contains(knownDefault[p], first) {
			return p + "." + name
		}
	}
	return local
}

// The default imports' packages in the order a name resolves in them,
// and the classes the stdlib declares there that library classes extend.
var (
	defaultOrder = []string{"kotlin", "kotlin.collections", "kotlin.ranges", "kotlin.sequences", "kotlin.text", "kotlin.annotation", "java.lang"}
	knownDefault = map[string][]string{
		"kotlin": {"Any", "Annotation", "CharSequence", "Comparable", "Enum", "Number", "Throwable", "Exception", "Error",
			"RuntimeException", "IllegalArgumentException", "IllegalStateException", "Function", "Lazy", "Result"},
		"kotlin.collections": {"Iterable", "Collection", "List", "Set", "Map", "MutableIterable", "MutableCollection",
			"MutableList", "MutableSet", "MutableMap", "Iterator", "MutableIterator", "ListIterator", "MutableListIterator",
			"AbstractList", "AbstractCollection", "AbstractSet", "AbstractMap", "AbstractMutableList",
			"AbstractMutableCollection", "AbstractMutableSet", "AbstractMutableMap", "RandomAccess"},
		"kotlin.ranges":     {"ClosedRange", "OpenEndRange", "IntProgression", "LongProgression", "CharProgression", "ClosedFloatingPointRange"},
		"kotlin.sequences":  {"Sequence"},
		"kotlin.text":       {"Appendable", "StringBuilder"},
		"kotlin.annotation": {"Retention", "Target", "MustBeDocumented", "Repeatable"},
		"java.lang":         {"Object", "Cloneable", "Runnable", "AutoCloseable", "Iterable"},
	}
)

// Subtypes returns the classes of views' sources that name the class
// called qualified in their supertype lists.
func Subtypes(views []*View, qualified string) []TypeItem {
	var out []TypeItem
	seen := map[key]bool{}
	for _, v := range views {
		v.eachSymbol(func(s analysis.Symbol) {
			if s.Kind != "class" || s.Declaration == nil || !slices.Contains(s.Supertypes, qualified) {
				return
			}
			d := v.node(*s.Declaration)
			if !d.Valid() || seen[keyOf(d)] || !slices.Contains(superNames([]*View{v}, d, nil), qualified) {
				return
			}
			if it, ok := typeItem(d); ok {
				seen[keyOf(d)] = true
				out = append(out, it)
			}
		})
	}
	slices.SortFunc(out, func(a, b TypeItem) int {
		if c := strings.Compare(a.Snap.Path(), b.Snap.Path()); c != 0 {
			return c
		}
		return a.Range.Start - b.Range.Start
	})
	return out
}
