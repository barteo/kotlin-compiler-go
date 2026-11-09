package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Rename finds what renaming a declaration in the sources edits: its
// name, its uses in every view that analyzed its file, and the names
// that stand for it without a fact:
//
//   - the declarations of its override family: what it overrides, and
//     what overrides that, in the sources;
//   - a parameter's name in named arguments of its function's calls;
//   - the last name of import directives that import it;
//   - KDoc links ([name], [Owner.name]) and a parameter's @param tag, in
//     the files edited.
//
// A use written with another name (an import alias, or an operator) is
// not edited. Conflicts with other declarations are not checked.

// RenameError is why a declaration can't be renamed.
type RenameError struct{ Msg string }

func (e *RenameError) Error() string { return e.Msg }

func refuse(msg string) error { return &RenameError{msg} }

// PrepareRename returns the range of the name the caret at off of
// views[0] is on, and the name, if what it names can be renamed.
func PrepareRename(views []*View, path string, off int) (psi.Range, string, error) {
	if len(views) == 0 {
		return psi.Range{}, "", refuse("no analysis of the file yet")
	}
	t, err := renameTarget(views[0], path, off)
	if err != nil {
		return psi.Range{}, "", err
	}
	leaf := caretLeaf(views[0].Snapshot(path), off)
	return leaf.Range(), t.decl.Name(), nil
}

// Rename returns the ranges renaming the declaration the caret at off of
// views[0] is on, or uses, to name replaces, and name as source text, in
// backticks if it needs them. The ranges are sorted and of the views'
// snapshots.
func Rename(views []*View, path string, off int, name string) ([]Loc, string, error) {
	if len(views) == 0 {
		return nil, "", refuse("no analysis of the file yet")
	}
	text, ok := psi.QuoteName(psi.Unquote(name))
	if !ok {
		return nil, "", refuse("'" + name + "' is not a valid name")
	}
	t, err := renameTarget(views[0], path, off)
	if err != nil {
		return nil, "", err
	}
	old := t.decl.Name()
	family, err := views[0].overrideFamily(t.decl)
	if err != nil {
		return nil, "", err
	}
	var out []Loc
	for _, d := range family {
		out = append(out, references(views, d, true)...)
		if d.NumericKind() == psi.KindVALUE_PARAMETER {
			out = append(out, namedArguments(views, d, old)...)
			out = append(out, contractNames(d, old)...)
		}
		if d.NumericKind() == psi.KindTYPE_PARAMETER {
			out = append(out, constraintNames(d, old)...)
		}
	}
	// A class's name in calls of its secondary constructors.
	for c := t.decl.ChildKind(psi.KindCLASS_BODY).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindSECONDARY_CONSTRUCTOR {
			out = append(out, references(views, c, false)...)
		}
	}
	if fq := declQualifiedName(t.decl); fq != "" {
		out = append(out, importNames(views, fq)...)
	}
	out = append(out, labels(views, t.decl, old)...)
	// Only the names written as the old one: not an alias's, or an
	// operator's.
	out = slices.DeleteFunc(out, func(l Loc) bool {
		return psi.Unquote(l.Snap.Source()[l.Range.Start:l.Range.End]) != old
	})
	out = append(out, kdocLinks(t.decl, old, out)...)
	return sortLocs(out), text, nil
}

// renameTarget returns the declaration the caret is on, if it can be
// renamed.
func renameTarget(v *View, path string, off int) (target, error) {
	t, ok := v.at(path, off)
	switch {
	case !ok:
		return target{}, refuse("nothing to rename here")
	case !t.decl.Valid():
		return target{}, refuse("only declarations in the sources can be renamed")
	}
	switch t.decl.NumericKind() {
	case psi.KindSECONDARY_CONSTRUCTOR, psi.KindPRIMARY_CONSTRUCTOR, psi.KindCLASS_INITIALIZER:
		return target{}, refuse("rename the class instead")
	case psi.KindFUNCTION_LITERAL, psi.KindPROPERTY_ACCESSOR:
		return target{}, refuse("nothing to rename here")
	}
	if !t.decl.NameIdentifier().Valid() {
		return target{}, refuse("the declaration has no name")
	}
	if hasModifier(t.decl, psi.KindOPERATOR_KEYWORD) {
		return target{}, refuse("an operator's name is fixed by its uses")
	}
	return t, nil
}

// references returns the uses of declaration d of views[0] in every view
// that analyzed its file, and its name if decl is set.
func references(views []*View, d psi.NodeRef, decl bool) []Loc {
	k, from := keyOf(d), d.ContainingFile()
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
	return out
}

// overrideFamily returns d and, for a member function or property, the
// members in the sources it overrides and that override those. It fails
// if a member of the family overrides one of the library's.
func (v *View) overrideFamily(d psi.NodeRef) ([]psi.NodeRef, error) {
	switch d.NumericKind() {
	case psi.KindFUN:
		return v.functionFamily(d)
	case psi.KindPROPERTY, psi.KindVALUE_PARAMETER:
		return v.propertyFamily(d)
	}
	return []psi.NodeRef{d}, nil
}

func elementOf(n psi.NodeRef) analysis.ElementID {
	return analysis.ElementID{Snapshot: n.ContainingFile().ID(), Node: n.ID()}
}

func (v *View) functionFamily(d psi.NodeRef) ([]psi.NodeRef, error) {
	f, _ := v.fact(d)
	s, ok := v.symbol(f.Symbol)
	if !ok {
		return []psi.NodeRef{d}, nil
	}
	roots := map[analysis.ElementID]bool{elementOf(d): true}
	for _, o := range s.Overrides {
		roots[o] = true
	}
	members := map[analysis.ElementID]bool{}
	for el := range roots {
		members[el] = true
	}
	v.eachSymbol(func(s analysis.Symbol) {
		if s.Declaration == nil {
			return
		}
		for _, o := range s.Overrides {
			if roots[o] {
				members[*s.Declaration] = true
				return
			}
		}
	})
	var out []psi.NodeRef
	for el := range members {
		n := v.node(el)
		if !n.Valid() {
			continue
		}
		if hasModifier(n, psi.KindOVERRIDE_KEYWORD) {
			// An override whose overridden members are all the library's.
			nf, _ := v.fact(n)
			if ns, ok := v.symbol(nf.Symbol); !ok || len(ns.Overrides) == 0 {
				return nil, refuse("'" + d.Name() + "' overrides a library declaration")
			}
		}
		out = append(out, n)
	}
	return out, nil
}

// propertyFamily finds a member property's family by name in the class
// hierarchy, since symbols record only functions' overrides: the roots,
// the highest declarations of the property that aren't overrides, and
// the overrides in their subclasses. A primary constructor's parameter is
// a property if it has val or var.
func (v *View) propertyFamily(d psi.NodeRef) ([]psi.NodeRef, error) {
	cls := memberOwner(d)
	if !cls.Valid() {
		return []psi.NodeRef{d}, nil
	}
	f, _ := v.fact(cls)
	cs, ok := v.symbol(f.Symbol)
	if !ok || cs.QualifiedName == "" {
		return []psi.NodeRef{d}, nil
	}
	supers := map[string][]string{}
	decls := map[string]psi.NodeRef{}
	v.eachSymbol(func(s analysis.Symbol) {
		if s.Kind == "class" && s.Declaration != nil && s.QualifiedName != "" {
			supers[s.QualifiedName] = s.Supertypes
			decls[s.QualifiedName] = v.node(*s.Declaration)
		}
	})
	name := d.Name()
	roots := map[string]bool{}
	library := false
	seen := map[string]bool{}
	var up func(c string) bool // reports whether c or a supertype declares it
	up = func(c string) bool {
		if seen[c] {
			return roots[c]
		}
		seen[c] = true
		p := memberProperty(decls[c], name)
		if p.Valid() && !hasModifier(p, psi.KindOVERRIDE_KEYWORD) {
			roots[c] = true
			return true
		}
		found := false
		for _, s := range supers[c] {
			if decls[s].Valid() && up(s) {
				found = true
			}
		}
		if p.Valid() && !found {
			library = true
		}
		return found || p.Valid()
	}
	up(cs.QualifiedName)
	if library {
		return nil, refuse("'" + name + "' overrides a library declaration")
	}
	// Down from the roots, through the classes in the sources.
	family := map[string]bool{}
	for r := range roots {
		family[r] = true
	}
	for changed := true; changed; {
		changed = false
		for c, ss := range supers {
			if family[c] {
				continue
			}
			for _, s := range ss {
				if family[s] {
					family[c], changed = true, true
					break
				}
			}
		}
	}
	out := []psi.NodeRef{d}
	for c := range family {
		if p := memberProperty(decls[c], name); p.Valid() && p != d {
			out = append(out, p)
		}
	}
	return out, nil
}

// memberOwner returns the class or object d is a member property of: a
// property of its body, or a val or var parameter of its primary
// constructor.
func memberOwner(d psi.NodeRef) psi.NodeRef {
	switch d.NumericKind() {
	case psi.KindPROPERTY:
		if b := d.Parent(); b.NumericKind() == psi.KindCLASS_BODY {
			return b.Parent()
		}
	case psi.KindVALUE_PARAMETER:
		if !d.ChildKind(psi.KindVAL_KEYWORD).Valid() && !d.ChildKind(psi.KindVAR_KEYWORD).Valid() {
			break
		}
		if c := d.Parent().Parent(); c.NumericKind() == psi.KindPRIMARY_CONSTRUCTOR {
			return c.Parent()
		}
	}
	return psi.NodeRef{}
}

// memberProperty returns class cls's member property name.
func memberProperty(cls psi.NodeRef, name string) psi.NodeRef {
	for c := cls.ChildKind(psi.KindCLASS_BODY).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindPROPERTY && c.Name() == name {
			return c
		}
	}
	for c := cls.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).ChildKind(psi.KindVALUE_PARAMETER_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindVALUE_PARAMETER && c.Name() == name && memberOwner(c).Valid() {
			return c
		}
	}
	return psi.NodeRef{}
}

// namedArguments returns the names of named arguments passed to
// parameter p in the calls of its function or constructor.
func namedArguments(views []*View, p psi.NodeRef, name string) []Loc {
	callee := p.Parent().Parent() // VALUE_PARAMETER_LIST's owner
	switch callee.NumericKind() {
	case psi.KindPRIMARY_CONSTRUCTOR:
		callee = callee.Parent()
	case psi.KindFUN, psi.KindSECONDARY_CONSTRUCTOR:
	default:
		return nil
	}
	// A data class's property is a parameter of its copy too.
	copyName := ""
	if callee.NumericKind() == psi.KindCLASS && hasModifier(callee, psi.KindDATA_KEYWORD) && memberOwner(p).Valid() {
		if fq := declQualifiedName(callee); fq != "" {
			copyName = fq + ".copy"
		}
	}
	k, from := keyOf(callee), callee.ContainingFile()
	var out []Loc
	named := func(snap *psi.Snapshot, args psi.NodeRef) {
		for a := args.FirstChild(); a.Valid(); a = a.NextSibling() {
			id := a.ChildKind(psi.KindVALUE_ARGUMENT_NAME).ChildKind(psi.KindREFERENCE_EXPRESSION).ChildKind(psi.KindIDENTIFIER)
			if id.Valid() && psi.Unquote(id.Text()) == name {
				out = append(out, Loc{snap, id.Range()})
			}
		}
	}
	for _, v := range views {
		d, ok := v.find(k, from)
		if !ok {
			continue
		}
		for _, r := range v.uses(d) {
			named(r.snap, callArguments(r.snap.Node(r.node)))
		}
		if copyName == "" {
			continue
		}
		for _, sid := range v.res.Snapshots() {
			snap := v.snaps[sid]
			if snap == nil {
				continue
			}
			for f := range v.res.FileFacts(sid) {
				if f.Call != nil && f.Call.Name == copyName {
					if n := snap.Node(f.Element.Node); n.NumericKind() == psi.KindREFERENCE_EXPRESSION {
						named(snap, callArguments(n))
					}
				}
			}
		}
	}
	return out
}

// labels returns the labels that name declaration d: this@C in class C,
// and @f in function f, which return@f, this@f and the labels of the
// lambdas passed to f's calls use.
func labels(views []*View, d psi.NodeRef, name string) []Loc {
	var scopes []psi.NodeRef
	switch d.NumericKind() {
	case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindPROPERTY:
		// this@p in an extension property's accessors.
		scopes = append(scopes, d)
	case psi.KindFUN:
		scopes = append(scopes, d)
		k, from := keyOf(d), d.ContainingFile()
		for _, v := range views {
			vd, ok := v.find(k, from)
			if !ok {
				continue
			}
			for _, r := range v.uses(vd) {
				c := r.snap.Node(r.node).Parent()
				if c.NumericKind() != psi.KindCALL_EXPRESSION {
					continue
				}
				for a := c.FirstChild(); a.Valid(); a = a.NextSibling() {
					switch a.NumericKind() {
					case psi.KindLAMBDA_ARGUMENT, psi.KindVALUE_ARGUMENT_LIST:
						scopes = append(scopes, a)
					}
				}
			}
		}
	default:
		return nil
	}
	var out []Loc
	for _, sc := range scopes {
		psi.WalkNodes(sc, func(n psi.NodeRef) bool {
			if n.NumericKind() == psi.KindLABEL_QUALIFIER {
				if l := n.ChildKind(psi.KindLABEL); l.Valid() && l.Text() == "@"+name {
					r := l.Range()
					out = append(out, Loc{sc.ContainingFile(), psi.Range{Start: r.Start + 1, End: r.End}})
				}
			}
			return true
		})
	}
	return out
}

// callArguments returns the argument list of the call whose callee is
// use n: a call's, a supertype's constructor call's, or an annotation's.
func callArguments(n psi.NodeRef) psi.NodeRef {
	switch n.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION:
		if c := n.Parent(); c.NumericKind() == psi.KindCALL_EXPRESSION && firstNonTrivia(c) == n {
			return c.ChildKind(psi.KindVALUE_ARGUMENT_LIST)
		}
	case psi.KindUSER_TYPE:
		if c := n.Parent().Parent(); c.NumericKind() == psi.KindCONSTRUCTOR_CALLEE {
			return c.Parent().ChildKind(psi.KindVALUE_ARGUMENT_LIST)
		}
	}
	return psi.NodeRef{}
}

// declQualifiedName returns the fully qualified name of a declaration an
// import can name: a top-level one, or one nested in classes and
// objects; "" for others.
func declQualifiedName(d psi.NodeRef) string {
	var names []string
	for n := d; ; {
		switch n.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindFUN, psi.KindPROPERTY, psi.KindTYPEALIAS:
		default:
			return ""
		}
		names = append(names, n.Name())
		p := n.Parent()
		if p.NumericKind() == psi.KindFILE {
			break
		}
		if p.NumericKind() != psi.KindCLASS_BODY {
			return ""
		}
		n = p.Parent()
		if n.NumericKind() != psi.KindCLASS && n.NumericKind() != psi.KindOBJECT_DECLARATION {
			return ""
		}
	}
	if pkg := packageName(d.ContainingFile()); pkg != "" {
		names = append(names, pkg)
	}
	slices.Reverse(names)
	return strings.Join(names, ".")
}

// importNames returns the last names of the import directives of the
// views' files that import fq.
func importNames(views []*View, fq string) []Loc {
	var out []Loc
	for _, v := range views {
		for _, snap := range v.byPath {
			for c := snap.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
				if c.NumericKind() != psi.KindIMPORT_DIRECTIVE {
					continue
				}
				// import fq, import fq as A, or import fq.* of a class's
				// members.
				names, ids := importPath(c)
				if len(names) > 0 && strings.Join(names, ".") == fq {
					out = append(out, Loc{snap, ids[len(ids)-1].Range()})
				}
			}
		}
	}
	return out
}

// importPath returns the names of an import directive's path, and their
// identifiers.
func importPath(imp psi.NodeRef) ([]string, []psi.NodeRef) {
	var names []string
	var ids []psi.NodeRef
	for c := imp.FirstChild(); c.Valid(); c = c.NextSibling() {
		if k := c.NumericKind(); k != psi.KindDOT_QUALIFIED_EXPRESSION && k != psi.KindREFERENCE_EXPRESSION {
			continue
		}
		psi.WalkNodes(c, func(n psi.NodeRef) bool {
			if n.NumericKind() == psi.KindIDENTIFIER {
				names, ids = append(names, psi.Unquote(n.Text())), append(ids, n)
			}
			return true
		})
	}
	return names, ids
}

// kdocLinks returns the names in KDoc of the declaration's file and the
// files of edits that link to name: [name] and [Owner.name], and a
// parameter's @param tag.
func kdocLinks(d psi.NodeRef, name string, edits []Loc) []Loc {
	files := map[*psi.Snapshot]bool{d.ContainingFile(): true}
	for _, l := range edits {
		files[l.Snap] = true
	}
	// A parameter's links are in its function's KDoc; a property
	// parameter's anywhere, as a property's.
	param := (d.NumericKind() == psi.KindVALUE_PARAMETER || d.NumericKind() == psi.KindTYPE_PARAMETER) && !memberOwner(d).Valid()
	var owner psi.NodeRef
	if param {
		owner = d.Parent().Parent()
		if owner.NumericKind() == psi.KindPRIMARY_CONSTRUCTOR {
			owner = owner.Parent()
		}
	}
	var out []Loc
	for snap := range files {
		psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
			if n.NumericKind() != psi.KindKDOC {
				return true
			}
			if param && n.Parent() != owner {
				return false
			}
			for _, r := range kdocNames(n.Text(), name, param) {
				out = append(out, Loc{snap, psi.Range{Start: n.Range().Start + r.Start, End: n.Range().Start + r.End}})
			}
			return false
		})
	}
	return out
}

// kdocNames returns the ranges in KDoc text of name as a segment of a
// link, and as a @param tag's name if param is set.
func kdocNames(text, name string, param bool) []psi.Range {
	var out []psi.Range
	for i := 0; i < len(text); i++ {
		switch {
		case text[i] == '[':
			j := strings.IndexAny(text[i+1:], "]\n")
			if j < 0 || text[i+1+j] != ']' {
				continue
			}
			start := i + 1
			for _, seg := range strings.Split(text[start:start+j], ".") {
				if psi.Unquote(strings.TrimSpace(seg)) == name && strings.TrimSpace(seg) == seg {
					out = append(out, psi.Range{Start: start, End: start + len(seg)})
				}
				start += len(seg) + 1
			}
			i += j + 1
		case param && strings.HasPrefix(text[i:], "@param"):
			j := i + len("@param")
			k := j
			for k < len(text) && (text[k] == ' ' || text[k] == '\t') {
				k++
			}
			if k > j && strings.HasPrefix(text[k:], name) {
				e := k + len(name)
				if e == len(text) || !isNamePart(text[e]) {
					out = append(out, psi.Range{Start: k, End: e})
				}
			}
			i = k
		}
	}
	return out
}

func isNamePart(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b >= 0x80
}

// contractNames returns the uses of parameter p in its function's
// contract { ... } block, which the analysis leaves without facts.
func contractNames(p psi.NodeRef, name string) []Loc {
	fn := p.Parent().Parent()
	if fn.NumericKind() != psi.KindFUN {
		return nil
	}
	body := fn.ChildKind(psi.KindBLOCK)
	var call psi.NodeRef
	for c := body.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsTrivia() && !c.IsLeaf() {
			call = c
			break
		}
	}
	if call.NumericKind() != psi.KindCALL_EXPRESSION || call.ChildKind(psi.KindREFERENCE_EXPRESSION).Text() != "contract" {
		return nil
	}
	var out []Loc
	psi.WalkNodes(call, func(n psi.NodeRef) bool {
		if n.NumericKind() == psi.KindREFERENCE_EXPRESSION && psi.Unquote(n.Text()) == name {
			out = append(out, Loc{fn.ContainingFile(), n.Range()})
		}
		return true
	})
	return out
}

// constraintNames returns the names of type parameter p in its owner's
// where clause, which the analysis leaves without facts.
func constraintNames(p psi.NodeRef, name string) []Loc {
	owner := p.Parent().Parent()
	var out []Loc
	for c := owner.ChildKind(psi.KindTYPE_CONSTRAINT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() != psi.KindTYPE_CONSTRAINT {
			continue
		}
		if r := c.ChildKind(psi.KindREFERENCE_EXPRESSION); r.Valid() && psi.Unquote(r.Text()) == name {
			out = append(out, Loc{p.ContainingFile(), r.Range()})
		}
	}
	return out
}
