package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// A Move is a source file moved to the directory of another package.
type Move struct {
	Path    string // the file's path before the move
	Package string // its package after
}

// MoveEdits returns the edits that keep the sources compiling when files
// move to other packages, as of views, all the modules' views:
//
//   - a moved file's package directive names its new package;
//   - imports of a moved file's top-level declarations, and their fully
//     qualified uses, name the new package;
//   - a file that used one without an import (from its own package, or
//     by a star import) imports it;
//   - a moved file imports the declarations of its old package it uses,
//     and drops the imports its new package makes needless.
//
// A file whose package is already the new one is left alone. The edits
// are of the views' snapshots, at most one per range.
func MoveEdits(views []*View, moves []Move) []FixEdit {
	m := &mover{canon: map[string]*psi.Snapshot{}, newPkg: map[string]string{}, adds: map[string][]string{}, deleted: map[string][]psi.Range{}, seen: map[moveKey]bool{}}
	for _, v := range views {
		if v == nil {
			continue
		}
		m.views = append(m.views, v)
		for p, s := range v.byPath {
			if _, ok := m.canon[p]; !ok {
				m.canon[p] = s
			}
		}
	}
	for _, mv := range moves {
		if s := m.canon[mv.Path]; s != nil && packageName(s) != mv.Package {
			m.newPkg[mv.Path] = mv.Package
		}
	}
	// The moved declarations, by their old qualified names.
	moved := map[string]string{} // old fq → new package
	for _, path := range slices.Sorted(mapKeys(m.newPkg)) {
		snap := m.canon[path]
		m.packageEdit(snap, m.newPkg[path])
		old := packageName(snap)
		for _, d := range topLevelDecls(snap) {
			moved[qualify(old, d.Name())] = m.newPkg[path]
			for _, l := range references(m.views, d, false) {
				if l.Snap.Path() != path {
					m.use(l, old, m.newPkg[path], d.Name())
				}
			}
		}
	}
	// What the moved files use of the files that stay.
	for _, path := range slices.Sorted(mapKeys(m.newPkg)) {
		m.movedUses(path)
	}
	for _, path := range slices.Sorted(mapKeys(m.canon)) {
		m.importEdits(m.canon[path], moved)
	}
	for _, path := range slices.Sorted(mapKeys(m.adds)) {
		m.addImports(m.canon[path], m.adds[path])
	}
	return m.edits
}

type mover struct {
	views   []*View
	canon   map[string]*psi.Snapshot // each file's snapshot the edits are of
	newPkg  map[string]string        // the moved files' new packages
	adds    map[string][]string      // the imports to add, by file
	deleted map[string][]psi.Range   // the imports deleted, by file
	edits   []FixEdit
	seen    map[moveKey]bool
}

type moveKey struct {
	path string
	r    psi.Range
}

func mapKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func qualify(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// pkgOf returns the package path's file is in after the moves.
func (m *mover) pkgOf(path string) string {
	if p, ok := m.newPkg[path]; ok {
		return p
	}
	if s := m.canon[path]; s != nil {
		return packageName(s)
	}
	return ""
}

// edit adds an edit of loc, mapped to the file's canonical snapshot,
// unless one of the range is there.
func (m *mover) edit(loc Loc, text string) {
	snap := m.canon[loc.Snap.Path()]
	if snap == nil {
		return
	}
	if snap != loc.Snap {
		r, ok := NewMapper(loc.Snap.Source(), snap.Source()).Range(loc.Range)
		if !ok {
			return
		}
		loc = Loc{snap, r}
	}
	k := moveKey{snap.Path(), loc.Range}
	if m.seen[k] {
		return
	}
	m.seen[k] = true
	m.edits = append(m.edits, FixEdit{loc, text})
}

// topLevelDecls returns the named top-level declarations of snap.
func topLevelDecls(snap *psi.Snapshot) []psi.NodeRef {
	var out []psi.NodeRef
	for c := snap.RootNode().FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindFUN, psi.KindPROPERTY, psi.KindTYPEALIAS:
			if c.NameIdentifier().Valid() && !hasModifier(c, psi.KindPRIVATE_KEYWORD) {
				out = append(out, c)
			}
		}
	}
	return out
}

// packageEdit renames snap's package to pkg.
func (m *mover) packageEdit(snap *psi.Snapshot, pkg string) {
	dir := snap.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE)
	var first, last psi.NodeRef
	for n := dir.FirstChild(); n.Valid(); n = n.NextSibling() {
		if !n.IsTrivia() && n.NumericKind() != psi.KindPACKAGE_KEYWORD {
			if !first.Valid() {
				first = n
			}
			last = n
		}
	}
	switch {
	case first.Valid() && pkg == "":
		r := dir.Range()
		for src := snap.Source(); r.End < len(src) && src[r.End] == '\n'; {
			r.End++
		}
		m.edit(Loc{snap, r}, "")
	case first.Valid():
		m.edit(Loc{snap, psi.Range{Start: first.Range().Start, End: last.Range().End}}, quotePath(pkg))
	case pkg != "":
		at := 0
		if dir.Valid() {
			at = dir.Range().Start
		}
		m.edit(Loc{snap, psi.Range{Start: at, End: at}}, "package "+quotePath(pkg)+"\n\n")
	}
}

// inHeader reports whether loc is in its file's package directive or
// imports.
func inHeader(loc Loc) bool {
	for n := loc.Snap.NodeAt(loc.Range.Start); n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindIMPORT_LIST, psi.KindPACKAGE_DIRECTIVE:
			return true
		}
	}
	return false
}

// use keeps the use at loc of a top-level declaration called name,
// moving from package from to package to, resolving: a qualified one
// names the new package; an unqualified one an import doesn't cover
// gets one.
func (m *mover) use(loc Loc, from, to, name string) {
	if from == to || inHeader(loc) {
		return
	}
	src := loc.Snap.Source()
	if from != "" && strings.HasSuffix(src[:loc.Range.Start], from+".") {
		q := loc.Range.Start - len(from) - 1
		text := ""
		if to != "" {
			text = quotePath(to) + "."
		}
		m.edit(Loc{loc.Snap, psi.Range{Start: q, End: loc.Range.Start}}, text)
		return
	}
	path := loc.Snap.Path()
	if to == "" || to == m.pkgOf(path) {
		return
	}
	snap := m.canon[path]
	if snap == nil {
		return
	}
	for _, i := range imports(snap) {
		if !i.star && i.path == qualify(from, name) || i.star && i.path == to {
			return // rewritten by importEdits, or already in scope
		}
	}
	fq := qualify(to, name)
	if !slices.Contains(m.adds[path], fq) {
		m.adds[path] = append(m.adds[path], fq)
	}
}

// movedUses keeps the uses in moved file path of the top-level
// declarations of files that stay.
func (m *mover) movedUses(path string) {
	for _, v := range m.views {
		snap := v.byPath[path]
		if snap == nil {
			continue
		}
		psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
			switch n.NumericKind() {
			case psi.KindIMPORT_LIST, psi.KindPACKAGE_DIRECTIVE:
				return false
			case psi.KindREFERENCE_EXPRESSION:
			default:
				return true
			}
			f, ok := v.fact(n)
			if !ok {
				return true
			}
			el, name, ok := v.use(f, n)
			if !ok {
				return true
			}
			d := v.node(el)
			if c := classOf(d); c.Valid() {
				d = c // a constructor's class
			}
			if !d.Valid() || d.Parent().NumericKind() != psi.KindFILE {
				return true
			}
			dp := d.ContainingFile().Path()
			if _, ok := m.newPkg[dp]; ok || dp == path {
				return true // moved too, or its own
			}
			// A use of a declaration of another package, but by its
			// qualified name: an import, unless one covers it.
			pkg := packageName(d.ContainingFile())
			if pkg != "" && pkg != m.newPkg[path] && !strings.HasSuffix(snap.Source()[:name.Start], pkg+".") {
				m.needImport(path, pkg, d.Name())
			}
			return true
		})
	}
}

// needImport adds an import of pkg's declaration called name to path's
// file, unless an import covers it.
func (m *mover) needImport(path, pkg, name string) {
	snap := m.canon[path]
	fq := qualify(pkg, name)
	for _, i := range imports(snap) {
		if !i.star && i.path == fq || i.star && i.path == pkg {
			return
		}
	}
	if !slices.Contains(m.adds[path], fq) {
		m.adds[path] = append(m.adds[path], fq)
	}
}

// importEdits rewrites snap's imports of moved declarations, and drops a
// moved file's imports of its new package's declarations.
func (m *mover) importEdits(snap *psi.Snapshot, moved map[string]string) {
	pkg := m.pkgOf(snap.Path())
	_, isMoved := m.newPkg[snap.Path()]
	for _, i := range imports(snap) {
		if i.star {
			if m.emptied(i.path) {
				m.deleteImport(snap, i)
			}
			continue
		}
		for fq, to := range moved {
			if i.path != fq && !strings.HasPrefix(i.path, fq+".") {
				continue
			}
			from := parent(fq)
			np := qualify(to, strings.TrimPrefix(strings.TrimPrefix(i.path, from), "."))
			switch q, ok := qualifierOf(i.node, from); {
			case i.alias == "" && parent(np) == pkg:
				m.deleteImport(snap, i) // of its own package now
			case ok && to == "":
				q.End++ // the dot
				m.edit(Loc{snap, q}, "")
			case ok:
				m.edit(Loc{snap, q}, quotePath(to))
			case from == "":
				if _, ids := importPath(i.node); len(ids) > 0 {
					at := ids[0].Range().Start
					m.edit(Loc{snap, psi.Range{Start: at, End: at}}, quotePath(to)+".")
				}
			}
		}
		if isMoved && i.alias == "" && parent(i.path) == pkg {
			if _, ok := moved[i.path]; !ok {
				m.deleteImport(snap, i)
			}
		}
	}
}

// emptied reports whether the moves leave no source file in package pkg,
// which one moved from.
func (m *mover) emptied(pkg string) bool {
	left := false
	for p := range m.newPkg {
		if packageName(m.canon[p]) == pkg {
			left = true
		}
	}
	if !left {
		return false
	}
	for p := range m.canon {
		if m.pkgOf(p) == pkg {
			return false
		}
	}
	return true
}

// qualifierOf returns the range of the leading names of import imp that
// spell pkg.
func qualifierOf(imp psi.NodeRef, pkg string) (psi.Range, bool) {
	names, ids := importPath(imp)
	n := len(strings.Split(pkg, "."))
	if pkg == "" || len(names) <= n || strings.Join(names[:n], ".") != pkg {
		return psi.Range{}, false
	}
	return psi.Range{Start: ids[0].Range().Start, End: ids[n-1].Range().End}, true
}

// deleteImport removes import i's line.
func (m *mover) deleteImport(snap *psi.Snapshot, i importDirective) {
	r := i.node.Range()
	src := snap.Source()
	if nl := strings.LastIndexByte(src[:r.Start], '\n'); nl >= 0 {
		r.Start = nl
	} else if r.End < len(src) && src[r.End] == '\n' {
		r.End++
	}
	m.edit(Loc{snap, r}, "")
	m.deleted[snap.Path()] = append(m.deleted[snap.Path()], i.node.Range())
}

// addImports adds imports of fqs to snap, as one edit per place.
func (m *mover) addImports(snap *psi.Snapshot, fqs []string) {
	slices.Sort(fqs)
	type group struct {
		r     psi.Range
		texts []string
	}
	var groups []group
	for _, fq := range fqs {
		e := ImportEdit(snap, fq)
		// Not at the start of an import deleteImport removes: after it.
		for _, d := range m.deleted[snap.Path()] {
			if e.Range.Start == d.Start {
				e = TextEdit{Range: psi.Range{Start: d.End, End: d.End}, Text: "\n" + strings.TrimSuffix(e.Text, "\n")}
			}
		}
		if k := len(groups) - 1; k >= 0 && groups[k].r == e.Range {
			groups[k].texts = append(groups[k].texts, e.Text)
		} else {
			groups = append(groups, group{e.Range, []string{e.Text}})
		}
	}
	for _, g := range groups {
		first := g.texts[0]
		lead := first[:len(first)-len(strings.TrimLeft(first, "\n"))]
		trail := first[len(strings.TrimRight(first, "\n")):]
		var body []string
		for _, t := range g.texts {
			body = append(body, strings.Trim(t, "\n"))
		}
		m.edit(Loc{snap, g.r}, lead+strings.Join(body, "\n")+trail)
	}
}

// PackageOf returns the package snap's directive names; "" for the
// default package.
func PackageOf(snap *psi.Snapshot) string { return packageName(snap) }

// PackageText returns the package directive of a new file in pkg, and
// the blank line after it.
func PackageText(pkg string) string { return "package " + quotePath(pkg) + "\n\n" }
