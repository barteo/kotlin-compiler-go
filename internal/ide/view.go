package ide

import (
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// A View is a module's semantic update: its result and the snapshots it
// analyzed. The navigation features read views; a view doesn't change,
// and is safe for concurrent use.
//
// A view's offsets are those of its snapshots. A document edited since
// maps them with a Mapper; so do views of one file from two modules
// updated at different times.
type View struct {
	res    *analysis.Result
	snaps  map[psi.SnapshotID]*psi.Snapshot
	byPath map[string]*psi.Snapshot

	classOnce sync.Once
	classes   map[string]analysis.ElementID // source classes by name
	aliases   map[string]psi.NodeRef        // source typealiases by name
	refOnce   sync.Once
	refs      map[analysis.ElementID][]ref // by declaration
}

// A ref is a use of a declaration: the node of its fact, and the range of
// the name it is written with.
type ref struct {
	snap *psi.Snapshot
	node psi.NodeID
	name psi.Range
}

// NewView returns the view of res, an update of snaps.
func NewView(res *analysis.Result, snaps []*psi.Snapshot) *View {
	v := &View{res: res, snaps: make(map[psi.SnapshotID]*psi.Snapshot, len(snaps)), byPath: make(map[string]*psi.Snapshot, len(snaps))}
	for _, s := range snaps {
		v.snaps[s.ID()] = s
		v.byPath[s.Path()] = s
	}
	return v
}

// Result returns the view's result.
func (v *View) Result() *analysis.Result { return v.res }

// Snapshot returns the snapshot of path the view analyzed, or nil.
func (v *View) Snapshot(path string) *psi.Snapshot { return v.byPath[path] }

// Loc is a range of a snapshot's text.
type Loc struct {
	Snap  *psi.Snapshot
	Range psi.Range
}

// node returns the node an element names, if it is of one of the view's
// snapshots.
func (v *View) node(el analysis.ElementID) psi.NodeRef {
	s := v.snaps[el.Snapshot]
	if s == nil || int(el.Node) > s.NodeCount() {
		return psi.NodeRef{}
	}
	return s.Node(el.Node)
}

// fact returns the fact of n.
func (v *View) fact(n psi.NodeRef) (analysis.Fact, bool) {
	if !n.Valid() {
		return analysis.Fact{}, false
	}
	return v.res.Fact(analysis.ElementID{Snapshot: n.ContainingFile().ID(), Node: n.ID()})
}

// symbol returns the symbol with ID id.
func (v *View) symbol(id uint32) (analysis.Symbol, bool) {
	if id == 0 {
		return analysis.Symbol{}, false
	}
	return v.res.Symbol(id)
}

// classIndex builds the view's map of source classes, once.
func (v *View) classIndex() {
	v.classOnce.Do(func() {
		v.classes = map[string]analysis.ElementID{}
		v.eachSymbol(func(s analysis.Symbol) {
			if s.Kind == "class" && s.Declaration != nil && s.QualifiedName != "" {
				v.classes[s.QualifiedName] = *s.Declaration
			}
		})
		v.aliases = map[string]psi.NodeRef{}
		for _, snap := range v.snaps {
			psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
				switch n.NumericKind() {
				case psi.KindTYPEALIAS:
					if fq := declQualifiedName(n); fq != "" {
						v.aliases[fq] = n
					}
					return false
				case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
					// A class no symbol names, as one of a file only
					// declared (analysis.UpdateOptions), which has no facts.
					if fq := declQualifiedName(n); fq != "" {
						if _, ok := v.classes[fq]; !ok {
							v.classes[fq] = analysis.ElementID{Snapshot: snap.ID(), Node: n.ID()}
						}
					}
					return true
				case psi.KindFILE, psi.KindCLASS_BODY:
					return true
				}
				return false
			})
		}
	})
}

// index builds the view's reverse index of uses, once.
func (v *View) index() {
	v.classIndex()
	v.refOnce.Do(func() {
		v.refs = map[analysis.ElementID][]ref{}
		for _, sid := range v.res.Snapshots() {
			snap := v.snaps[sid]
			if snap == nil {
				continue
			}
			for f := range v.res.FileFacts(sid) {
				n := snap.Node(f.Element.Node)
				if d, name, ok := v.use(f, n); ok {
					v.refs[d] = append(v.refs[d], ref{snap, n.ID(), name})
				}
			}
		}
	})
}

// use returns the declaration a fact of node n uses, and the range of
// the name it is written with: a reference's, a type's, or an operator
// call's operator.
func (v *View) use(f analysis.Fact, n psi.NodeRef) (analysis.ElementID, psi.Range, bool) {
	var decl *analysis.ElementID
	name := n.Range()
	switch n.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION:
		if s, ok := v.symbol(f.Symbol); ok && s.Declaration != nil {
			decl = s.Declaration
			// An extension property's inline getter: the property.
			if d := v.node(*decl); d.NumericKind() == psi.KindPROPERTY_ACCESSOR {
				el := analysis.ElementID{Snapshot: decl.Snapshot, Node: d.Parent().ID()}
				decl = &el
			}
			// A class's name that stands for its companion object, C in
			// C.f(), names the class.
			if d := v.node(*decl); d.NumericKind() == psi.KindOBJECT_DECLARATION && hasModifier(d, psi.KindCOMPANION_KEYWORD) &&
				psi.Unquote(n.Text()) != d.Name() {
				if c := d.Parent().Parent(); c.Declaration().Valid() {
					el := analysis.ElementID{Snapshot: c.ContainingFile().ID(), Node: c.ID()}
					decl = &el
				}
			}
		} else if f.Call != nil {
			decl = callDeclaration(f.Call)
			// A fun interface's SAM constructor, F { ... }: the interface.
			if r := f.Call.ReturnType; decl == nil && r != nil && r.SAM && strings.HasSuffix("."+r.Name, "."+psi.Unquote(n.Text())) {
				if el, ok := v.classes[r.Name]; ok {
					decl = &el
				}
			}
		} else if f.Role == "type" && f.Type != nil && f.Type.Parameter {
			decl = v.typeParameterOf(n, *f.Type)
		}
		// A typealias's name, A() for typealias A = C.
		if f.Classifier {
			if a, ok := v.aliasFor(f, psi.Unquote(n.Text()), decl); ok {
				el := analysis.ElementID{Snapshot: a.ContainingFile().ID(), Node: a.ID()}
				decl = &el
			}
		}
	case psi.KindUSER_TYPE:
		if f.Type == nil {
			break
		}
		r := lastChild(n, psi.KindREFERENCE_EXPRESSION)
		if !r.Valid() {
			break
		}
		name = r.Range()
		if a, ok := v.aliases[f.ImportableName]; ok && f.ImportableName != "" {
			el := analysis.ElementID{Snapshot: a.ContainingFile().ID(), Node: a.ID()}
			decl = &el
			break
		}
		if f.Type.Parameter {
			decl = v.typeParameterOf(n, *f.Type)
			break
		}
		if el, ok := v.classes[f.Type.Name]; ok {
			decl = &el
		}
	case psi.KindCALLABLE_REFERENCE_EXPRESSION:
		// ::f, C::f, x::p: the name after the "::".
		r := lastChild(n, psi.KindREFERENCE_EXPRESSION)
		if !r.Valid() || r.Range().Start < n.ChildKind(psi.KindCOLONCOLON).Range().End {
			break
		}
		name = r.Range()
		if s, ok := v.symbol(f.Symbol); ok && s.Declaration != nil {
			decl = s.Declaration
		} else if f.Call != nil {
			decl = f.Call.Declaration
		}
	case psi.KindBINARY_EXPRESSION, psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION:
		op := n.ChildKind(psi.KindOPERATION_REFERENCE)
		if f.Call != nil && op.Valid() {
			decl, name = f.Call.Declaration, op.Range()
		}
	}
	if decl == nil {
		return analysis.ElementID{}, psi.Range{}, false
	}
	return *decl, name, true
}

// aliasFor returns the typealias a classifier reference written name
// stands for, when decl, the class or constructor it resolved to, has
// another name.
func (v *View) aliasFor(f analysis.Fact, name string, decl *analysis.ElementID) (psi.NodeRef, bool) {
	if decl != nil {
		d := v.node(*decl)
		switch d.NumericKind() {
		case psi.KindPRIMARY_CONSTRUCTOR:
			d = d.Parent()
		case psi.KindSECONDARY_CONSTRUCTOR:
			d = d.Parent().Parent()
		}
		if d.Name() == name {
			return psi.NodeRef{}, false
		}
	}
	if a, ok := v.aliases[f.ImportableName]; ok {
		return a, true
	}
	var found psi.NodeRef
	for _, a := range v.aliases {
		if a.Name() == name {
			if found.Valid() {
				return psi.NodeRef{}, false
			}
			found = a
		}
	}
	return found, found.Valid()
}

// callDeclaration returns the declaration a call resolved to, or for a
// member a class delegates, the interface's member.
func callDeclaration(c *analysis.Call) *analysis.ElementID {
	if c.Declaration != nil {
		return c.Declaration
	}
	for _, t := range c.Targets {
		if t.Declaration != nil {
			return t.Declaration
		}
	}
	return nil
}

// typeParameterOf returns the declaration of type parameter t, used at
// node n: of its owner, or else of the nearest declaration around n
// that declares one of its name (a typealias's).
func (v *View) typeParameterOf(n psi.NodeRef, t analysis.Type) *analysis.ElementID {
	var p psi.NodeRef
	if t.ParameterOwner != nil {
		p = typeParameter(v.node(*t.ParameterOwner), t.Name)
	}
	for d := n.Parent(); !p.Valid() && d.Valid(); d = d.Parent() {
		if d.Declaration().Valid() || d.NumericKind() == psi.KindTYPEALIAS {
			p = typeParameter(d, t.Name)
		}
	}
	if !p.Valid() {
		return nil
	}
	el := analysis.ElementID{Snapshot: p.ContainingFile().ID(), Node: p.ID()}
	return &el
}

// typeParameter returns the type parameter of declaration owner named
// name.
func typeParameter(owner psi.NodeRef, name string) psi.NodeRef {
	for c := owner.ChildKind(psi.KindTYPE_PARAMETER_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindTYPE_PARAMETER && c.Name() == name {
			return c
		}
	}
	return psi.NodeRef{}
}

func lastChild(n psi.NodeRef, k psi.Kind) psi.NodeRef {
	var last psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == k {
			last = c
		}
	}
	return last
}

// nameRange returns the range of the name of declaration d: its
// identifier, or the keyword that stands for it ("constructor",
// "object", "init").
func nameRange(d psi.NodeRef) psi.Range {
	if id := d.NameIdentifier(); id.Valid() {
		return id.Range()
	}
	for _, k := range []psi.Kind{psi.KindCONSTRUCTOR_KEYWORD, psi.KindOBJECT_KEYWORD, psi.KindINIT_KEYWORD} {
		if c := d.ChildKind(k); c.Valid() {
			return c.Range()
		}
	}
	r := d.Range()
	return psi.Range{Start: r.Start, End: r.Start}
}

// target is what a caret is on: a declaration, as its node in the view,
// and the use of it the caret is on, if any.
type target struct {
	decl psi.NodeRef
	use  psi.NodeRef // the node of the fact the caret is on
	fact analysis.Fact
}

// caretLeaf returns the token at offset off of snap, preferring a name
// just before off to the whitespace or punctuation after it.
func caretLeaf(snap *psi.Snapshot, off int) psi.NodeRef {
	n := snap.NodeAt(off)
	if !named(n) && n.Parent().NumericKind() != psi.KindOPERATION_REFERENCE && off > 0 {
		if p := snap.NodeAt(off - 1); named(p) {
			return p
		}
	}
	return n
}

// named reports whether leaf n is a name: an identifier, or a keyword
// that names something (this, super).
func named(n psi.NodeRef) bool {
	switch n.NumericKind() {
	case psi.KindIDENTIFIER, psi.KindFIELD_IDENTIFIER, psi.KindTHIS_KEYWORD, psi.KindSUPER_KEYWORD,
		psi.KindCONSTRUCTOR_KEYWORD, psi.KindOBJECT_KEYWORD, psi.KindINIT_KEYWORD:
		return true
	}
	return false
}

// at returns what the caret at offset off of path's snapshot is on.
func (v *View) at(path string, off int) (target, bool) {
	snap := v.byPath[path]
	if snap == nil || off < 0 || off > len(snap.Source()) {
		return target{}, false
	}
	leaf := caretLeaf(snap, off)
	if !leaf.Valid() {
		return target{}, false
	}
	v.classIndex()
	// A declaration's name.
	if p := leaf.Parent(); p.Declaration().Valid() && named(leaf) && nameRange(p) == leaf.Range() {
		f, _ := v.fact(p) // a secondary constructor has none
		return target{decl: p, fact: f}, true
	}
	// A type parameter's name in a where clause.
	if tp := constrained(leaf); tp.Valid() {
		f, _ := v.fact(tp)
		return target{decl: tp, fact: f}, true
	}
	// A use: the nearest node with a fact, if the caret is on a name or
	// an operator.
	if !named(leaf) && leaf.Parent().NumericKind() != psi.KindOPERATION_REFERENCE {
		return target{}, false
	}
	for n := leaf; n.Valid(); n = n.Parent() {
		f, ok := v.fact(n)
		if !ok {
			continue
		}
		t := target{use: n, fact: f}
		if d, name, ok := v.use(f, n); ok && name.Start <= leaf.Range().Start && leaf.Range().End <= name.End {
			t.decl = v.node(d)
		}
		return t, true
	}
	return target{}, false
}

// key is a declaration by its file and range, which names it across the
// views of modules updated at different times.
type key struct {
	path string
	r    psi.Range
}

// keyOf returns the key of declaration d.
func keyOf(d psi.NodeRef) key { return key{d.ContainingFile().Path(), d.Range()} }

// find returns the declaration k names in v, if v analyzed its file: k
// is of snapshot from, mapped to v's snapshot of the file.
func (v *View) find(k key, from *psi.Snapshot) (psi.NodeRef, bool) {
	snap := v.byPath[k.path]
	if snap == nil {
		return psi.NodeRef{}, false
	}
	r := k.r
	if snap != from {
		var ok bool
		if r, ok = NewMapper(from.Source(), snap.Source()).Range(r); !ok {
			return psi.NodeRef{}, false
		}
	}
	for n := snap.NodeAt(r.Start); n.Valid(); n = n.Parent() {
		if n.Range() == r {
			if n.Declaration().Valid() {
				return n, true
			}
		}
		if n.Range().Start < r.Start || n.Range().End > r.End {
			break
		}
	}
	return psi.NodeRef{}, false
}

// uses returns the uses of declaration d of the view.
func (v *View) uses(d psi.NodeRef) []ref {
	v.index()
	return v.refs[analysis.ElementID{Snapshot: d.ContainingFile().ID(), Node: d.ID()}]
}

// A Mapper maps offsets of one text to another that differs from it in
// one span: the common prefix and suffix keep their text.
type Mapper struct {
	prefix         int // the common prefix's length
	oldEnd, newEnd int // where the common suffix starts in each text
}

// NewMapper returns the mapper from text a to text b.
func NewMapper(a, b string) Mapper {
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	q := 0
	for q < len(a)-p && q < len(b)-p && a[len(a)-1-q] == b[len(b)-1-q] {
		q++
	}
	return Mapper{p, len(a) - q, len(b) - q}
}

// Range maps a range of the old text to the new one, or reports false if
// the change touches it.
func (m Mapper) Range(r psi.Range) (psi.Range, bool) {
	switch {
	case r.End <= m.prefix:
		return r, true
	case r.Start >= m.oldEnd:
		d := m.newEnd - m.oldEnd
		return psi.Range{Start: r.Start + d, End: r.End + d}, true
	}
	return psi.Range{}, false
}

// Back maps an offset of the new text to the old one, or reports false
// if it is in the changed span.
func (m Mapper) Back(off int) (int, bool) {
	switch {
	case off <= m.prefix:
		return off, true
	case off >= m.newEnd:
		return off - (m.newEnd - m.oldEnd), true
	}
	return 0, false
}

// sortLocs sorts locations by file and offset, dropping duplicates.
func sortLocs(ls []Loc) []Loc {
	slices.SortFunc(ls, func(a, b Loc) int {
		if a.Snap.Path() != b.Snap.Path() {
			if a.Snap.Path() < b.Snap.Path() {
				return -1
			}
			return 1
		}
		if a.Range.Start != b.Range.Start {
			return a.Range.Start - b.Range.Start
		}
		return a.Range.End - b.Range.End
	})
	return slices.CompactFunc(ls, func(a, b Loc) bool { return a.Snap.Path() == b.Snap.Path() && a.Range == b.Range })
}

// constrained returns the type parameter whose name leaf is in a where
// clause's constraint, T in where T : A; none if leaf isn't one.
func constrained(leaf psi.NodeRef) psi.NodeRef {
	ref := leaf.Parent()
	c := ref.Parent()
	if leaf.NumericKind() != psi.KindIDENTIFIER || ref.NumericKind() != psi.KindREFERENCE_EXPRESSION ||
		c.NumericKind() != psi.KindTYPE_CONSTRAINT || firstNonTrivia(c) != ref {
		return psi.NodeRef{}
	}
	owner := c.Parent().Parent()
	for tp := owner.ChildKind(psi.KindTYPE_PARAMETER_LIST).FirstChild(); tp.Valid(); tp = tp.NextSibling() {
		if tp.NumericKind() == psi.KindTYPE_PARAMETER && tp.Name() == leaf.Text() {
			return tp
		}
	}
	return psi.NodeRef{}
}
