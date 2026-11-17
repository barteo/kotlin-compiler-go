package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// A CallItem is a declaration in a call hierarchy: a function,
// constructor, property, or class (for its constructors and
// initializers).
type CallItem struct {
	Name   string
	Kind   SymbolKind
	Detail string // the container: a class's or function's name, or the package
	Snap   *psi.Snapshot
	// Range is the whole declaration; NameRange its name, or keyword.
	Range, NameRange psi.Range
}

// A CallerCall groups the calls one declaration makes: of the item asked
// about, from Item (incoming), or of Item, from the item asked about
// (outgoing). Ranges are the callee names in the caller's file.
type CallerCall struct {
	Item   CallItem
	Ranges []psi.Range
}

// PrepareCallHierarchy returns the item of the declaration the caret is
// on, or that a reference at the caret names.
func PrepareCallHierarchy(v *View, path string, off int) (CallItem, bool) {
	t, ok := v.at(path, off)
	if !ok || !t.decl.Valid() {
		return CallItem{}, false
	}
	return callItem(t.decl)
}

// callItem returns the item of declaration d: a primary constructor or
// an initializer block is its class's.
func callItem(d psi.NodeRef) (CallItem, bool) {
	switch d.NumericKind() {
	case psi.KindPRIMARY_CONSTRUCTOR:
		d = d.Parent()
	case psi.KindCLASS_INITIALIZER:
		d = d.Parent().Parent()
	}
	inClass := d.Parent().NumericKind() == psi.KindCLASS_BODY
	s, ok := symbol(d, inClass)
	switch {
	case d.NumericKind() == psi.KindVALUE_PARAMETER && !ok:
		return CallItem{}, false
	case !ok:
		return CallItem{}, false
	}
	return CallItem{Name: s.Name, Kind: s.Kind, Detail: containerName(d), Snap: d.ContainingFile(), Range: d.Range(), NameRange: s.NameRange}, true
}

// callTarget returns the declaration of item it, found again at its
// name in views[0].
func callTarget(v *View, path string, off int) psi.NodeRef {
	snap := v.Snapshot(path)
	if snap == nil {
		return psi.NodeRef{}
	}
	for n := caretLeaf(snap, off); n.Valid(); n = n.Parent() {
		if d := n.Declaration(); d.Valid() && nameRange(d).Start == off {
			return d
		}
	}
	return psi.NodeRef{}
}

// IncomingCalls returns the calls of the declaration whose name starts
// at off of path, and of the members of its override family, grouped by
// the declaration they are made in. A class's calls are those of its
// constructors.
func IncomingCalls(views []*View, path string, off int) []CallerCall {
	if len(views) == 0 {
		return nil
	}
	d := callTarget(views[0], path, off)
	if !d.Valid() {
		return nil
	}
	targets, err := views[0].overrideFamily(d)
	if err != nil {
		targets = []psi.NodeRef{d}
	}
	isClass := d.NumericKind() == psi.KindCLASS || d.NumericKind() == psi.KindOBJECT_DECLARATION
	if isClass {
		for c := d.ChildKind(psi.KindCLASS_BODY).FirstChild(); c.Valid(); c = c.NextSibling() {
			if c.NumericKind() == psi.KindSECONDARY_CONSTRUCTOR {
				targets = append(targets, c)
			}
		}
	}
	type group struct {
		item   CallItem
		ranges []psi.Range
	}
	groups := map[key]*group{}
	var order []key
	seen := map[Loc]bool{}
	for _, t := range targets {
		k, from := keyOf(t), t.ContainingFile()
		for _, v := range views {
			vt, ok := v.find(k, from)
			if !ok {
				continue
			}
			for _, r := range v.uses(vt) {
				n := r.snap.Node(r.node)
				f, _ := v.fact(n)
				if isClass && f.Call == nil {
					continue // a type, not a constructor call
				}
				if n.Parent().NumericKind() == psi.KindIMPORT_DIRECTIVE || seen[Loc{r.snap, r.name}] {
					continue
				}
				caller, ok := callItem(n.EnclosingDeclaration())
				if !ok {
					continue
				}
				seen[Loc{r.snap, r.name}] = true
				ck := key{caller.Snap.Path(), caller.NameRange}
				g, ok := groups[ck]
				if !ok {
					g = &group{item: caller}
					groups[ck] = g
					order = append(order, ck)
				}
				g.ranges = append(g.ranges, r.name)
			}
		}
	}
	out := make([]CallerCall, 0, len(order))
	for _, k := range order {
		g := groups[k]
		slices.SortFunc(g.ranges, func(a, b psi.Range) int { return a.Start - b.Start })
		out = append(out, CallerCall{g.item, g.ranges})
	}
	sortCalls(out)
	return out
}

// OutgoingCalls returns the calls the declaration whose name starts at
// off of path makes, in its code outside nested named declarations,
// grouped by the function, constructor or class called.
//
// lib, if it isn't nil, finds the declaration of a callee of the library,
// in its sources; the calls of the others are left out.
func OutgoingCalls(v *View, path string, off int, lib func(libsrc.Target) (psi.NodeRef, bool)) []CallerCall {
	d := callTarget(v, path, off)
	if !d.Valid() {
		return nil
	}
	type group struct {
		item   CallItem
		ranges []psi.Range
	}
	groups := map[analysis.ElementID]*group{}
	var order []analysis.ElementID
	psi.WalkNodes(d, func(n psi.NodeRef) bool {
		if n != d && isCaller(n) {
			return false // a nested function, class or member: its own item
		}
		f, ok := v.fact(n)
		if !ok || f.Role == "declaration" {
			return true
		}
		el, name, ok := v.use(f, n)
		var t psi.NodeRef
		if ok {
			t = v.node(el)
		} else if lt, isLib := libraryTarget(v, f, n); isLib && lib != nil && f.Call != nil && n.NumericKind() == psi.KindREFERENCE_EXPRESSION &&
			(lt.Kind == "function" || lt.Kind == "constructor") {
			if t, ok = lib(lt); !ok {
				return true
			}
			el, name = analysis.ElementID{Snapshot: t.ContainingFile().ID(), Node: t.ID()}, n.Range()
		} else {
			return true
		}
		switch t.NumericKind() {
		case psi.KindFUN, psi.KindSECONDARY_CONSTRUCTOR, psi.KindPRIMARY_CONSTRUCTOR:
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
			if f.Call == nil {
				return true
			}
		default:
			return true
		}
		item, ok := callItem(t)
		if !ok {
			return true
		}
		g, ok := groups[el]
		if !ok {
			g = &group{item: item}
			groups[el] = g
			order = append(order, el)
		}
		g.ranges = append(g.ranges, name)
		return true
	})
	out := make([]CallerCall, 0, len(order))
	for _, el := range order {
		out = append(out, CallerCall{groups[el].item, groups[el].ranges})
	}
	sortCalls(out)
	return out
}

func sortCalls(cs []CallerCall) {
	slices.SortStableFunc(cs, func(a, b CallerCall) int {
		if a.Item.Snap.Path() != b.Item.Snap.Path() {
			if a.Item.Snap.Path() < b.Item.Snap.Path() {
				return -1
			}
			return 1
		}
		return a.Item.NameRange.Start - b.Item.NameRange.Start
	})
}

// isCaller reports whether n is a declaration EnclosingDeclaration
// returns, other than an initializer block, which is its class's.
func isCaller(n psi.NodeRef) bool {
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsTrivia() {
			e := c.EnclosingDeclaration()
			return e == n && n.NumericKind() != psi.KindCLASS_INITIALIZER
		}
	}
	return false
}

// containerName returns the name of the class, object or function d is
// declared in, or of its file's package.
func containerName(d psi.NodeRef) string {
	for p := d.Parent(); p.Valid(); p = p.Parent() {
		switch p.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindFUN:
			if n := p.Name(); n != "" {
				return n
			}
		case psi.KindFILE:
			if pd := p.ChildKind(psi.KindPACKAGE_DIRECTIVE); pd.Valid() {
				return strings.TrimSpace(strings.TrimPrefix(compact(pd.Text()), "package"))
			}
		}
	}
	return ""
}

// PrepareLibraryCallHierarchy returns the item of the library's function,
// constructor or class the caret at off of path uses, in the file lib
// finds it in, and the target that names it.
func PrepareLibraryCallHierarchy(v *View, path string, off int, lib LibraryClass) (CallItem, libsrc.Target, bool) {
	t, ok := LibraryTarget(v, path, off)
	if !ok || lib == nil || t.Kind != "function" && t.Kind != "constructor" && t.Kind != "class" {
		return CallItem{}, libsrc.Target{}, false
	}
	d, ok := lib(t)
	if !ok {
		return CallItem{}, libsrc.Target{}, false
	}
	if t.Kind == "constructor" {
		d = classOf(d) // its class's item, as a source class's
	}
	it, ok := callItem(d)
	return it, t, ok
}

// LibraryIncomingCalls returns the calls in views' sources of the
// library's declaration t: of a function, those of its overload; of a
// class or constructor, its constructors'. They are grouped by the
// declaration they are made in.
func LibraryIncomingCalls(views []*View, t libsrc.Target) []CallerCall {
	type group struct {
		item   CallItem
		ranges []psi.Range
	}
	groups := map[key]*group{}
	var order []key
	seen := map[key]bool{}
	for _, v := range views {
		for _, path := range v.Paths() {
			psi.WalkNodes(v.Snapshot(path).RootNode(), func(n psi.NodeRef) bool {
				switch n.NumericKind() {
				case psi.KindIMPORT_LIST, psi.KindPACKAGE_DIRECTIVE:
					return false
				case psi.KindREFERENCE_EXPRESSION:
				default:
					return true
				}
				f, ok := v.fact(n)
				if !ok || f.Call == nil {
					return true
				}
				u, ok := libraryTarget(v, f, n)
				if !ok || !sameTarget(t, u) || seen[key{path, n.Range()}] {
					return true
				}
				seen[key{path, n.Range()}] = true
				caller, ok := callItem(n.EnclosingDeclaration())
				if !ok {
					return true
				}
				ck := key{caller.Snap.Path(), caller.NameRange}
				g, ok := groups[ck]
				if !ok {
					g = &group{item: caller}
					groups[ck] = g
					order = append(order, ck)
				}
				g.ranges = append(g.ranges, n.Range())
				return true
			})
		}
	}
	out := make([]CallerCall, 0, len(order))
	for _, k := range order {
		g := groups[k]
		slices.SortFunc(g.ranges, func(a, b psi.Range) int { return a.Start - b.Start })
		out = append(out, CallerCall{g.item, g.ranges})
	}
	sortCalls(out)
	return out
}

// sameTarget reports whether a use's target u names the declaration t
// does: a class and its constructors are one; a function's parameters
// and receiver must match where both are known.
func sameTarget(t, u libsrc.Target) bool {
	kind := func(k string) string {
		if k == "class" {
			return "constructor"
		}
		return k
	}
	if t.Name != u.Name || kind(t.Kind) != kind(u.Kind) {
		return false
	}
	if t.Kind == "function" && t.Params && u.Params {
		return slices.Equal(t.Parameters, u.Parameters) && slices.Equal(t.Varargs, u.Varargs) && t.Receiver == u.Receiver
	}
	return true
}
