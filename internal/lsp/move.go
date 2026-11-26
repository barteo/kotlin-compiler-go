package lsp

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

type fileMove struct{ OldURI, NewURI string }

func filePackage(path string, m *module) (string, bool) {
	best := ""
	for _, r := range m.roots() {
		if within(path, r) && len(r) > len(best) {
			best = r
		}
	}
	if best == "" {
		return "", false
	}
	rel, err := filepath.Rel(best, filepath.Dir(path))
	if err != nil {
		return "", false
	}
	if rel == "." {
		return "", true
	}
	name := strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
	s := psi.ParseDocument("package.kt", "package "+name)
	pkg := s.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE)
	return name, len(s.Diagnostics()) == 0 && pkg.Path() == name && pkg.Text() == "package "+name
}
func importOffset(s *psi.Snapshot) (position, string) {
	if list := s.RootNode().ChildKind(psi.KindIMPORT_LIST); list.Valid() && list.Range().End > list.Range().Start {
		return rng(list).End, "\n"
	}
	if pkg := s.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE); pkg.Valid() && pkg.Path() != "" {
		return rng(pkg).End, "\n\n"
	}
	for _, kind := range []psi.Kind{psi.KindFILE_ANNOTATION_LIST, psi.KindSHEBANG_COMMENT} {
		if n := s.RootNode().ChildKind(kind); n.Valid() {
			return rng(n).End, "\n\n"
		}
	}
	return position{}, ""
}
func addImports(s *psi.Snapshot, paths map[string]bool) []edit {
	for c := s.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsAllUnder() && c.Alias() == "" {
			delete(paths, c.Path())
		}
	}
	if len(paths) == 0 {
		return nil
	}
	at, lead := importOffset(s)
	sep := "\n"
	if strings.Contains(s.Source(), "\r\n") {
		sep = "\r\n"
		lead = strings.ReplaceAll(lead, "\n", sep)
	}
	text := lead + "import " + strings.Join(slices.Sorted(maps.Keys(paths)), sep+"import ")
	if at == (position{}) {
		text += sep + sep
	} else if end, err := offset(s, at); err == nil {
		rest := s.Source()[end:]
		if strings.HasPrefix(rest, sep) && !strings.HasPrefix(rest, sep+sep) && strings.TrimSpace(rest) != "" {
			text += sep
		}
	}
	return []edit{{span{at, at}, text}}
}
func (s *server) workspaceEdits(changes map[string][]edit) object {
	if !s.versionEdits {
		return object{"changes": changes}
	}
	edits := []object{}
	for _, u := range slices.Sorted(maps.Keys(changes)) {
		path, _ := filePath(u)
		edits = append(edits, object{"textDocument": object{"uri": u, "version": s.w.docs[path].version}, "edits": changes[u]})
	}
	return object{"documentChanges": edits}
}

// Expand each folder against the current documents, including open buffers that
// have never been saved. Identical parent and child events describe one move.
func (w *workspace) movePaths(moves []fileMove, preflight bool) (map[string]string, error) {
	paths := map[string]string{}
	destinations := map[string]string{}
	files := map[string]bool{}
	for path := range w.docs {
		files[path] = true
	}
	for path := range w.java {
		files[path] = true
	}
	for _, m := range moves {
		old, err := filePath(m.OldURI)
		if err != nil {
			return nil, err
		}
		dest, err := filePath(m.NewURI)
		if err != nil {
			return nil, err
		}
		if old == dest {
			continue
		}
		matched := false
		for path := range files {
			if !within(path, old) {
				continue
			}
			matched = true
			rel, err := filepath.Rel(old, path)
			if err != nil {
				return nil, err
			}
			to := filepath.Join(dest, rel)
			if prior, ok := paths[path]; ok && prior != to {
				return nil, moveConflict("Conflicting destinations for %s", path)
			}
			if prior, ok := destinations[to]; ok && prior != path {
				return nil, moveConflict("Several files would move to %s", to)
			}
			paths[path], destinations[to] = to, path
		}
		if !matched || !preflight {
			continue
		}
		if prior, ok := destinations[dest]; ok && prior != old {
			return nil, moveConflict("Several paths would move to %s", dest)
		}
		destinations[dest] = old
		if within(dest, old) {
			return nil, moveConflict("Destination is inside %s", old)
		}
		if _, err := os.Lstat(dest); !os.IsNotExist(err) {
			if err != nil {
				return nil, err
			}
			return nil, moveConflict("Destination already exists: %s", dest)
		}
		for path := range files {
			if within(path, dest) {
				return nil, moveConflict("Destination already contains %s", path)
			}
		}
		for _, mod := range w.p.Modules {
			for _, root := range mod.roots() {
				if within(root, old) {
					return nil, moveConflict("Cannot move source root %s", root)
				}
			}
		}
	}
	return paths, nil
}

func moveConflict(format string, args ...any) error {
	return rpcError{-32803, fmt.Sprintf(format, args...)}
}

func (s *server) didMoveFiles(ctx context.Context, moves []fileMove) error {
	paths, err := s.w.movePaths(moves, false)
	if err != nil {
		return err
	}
	// The client has performed the rename. Reload closed files from disk and
	// carry open overlays to their new URIs, unless the client reopened them.
	overlays := map[string]document{}
	for old, dest := range paths {
		d := s.w.docs[old]
		delete(s.w.docs, old)
		if d.version != nil {
			overlays[dest] = document{psi.ParseDocument(dest, d.snap.Source()), d.version}
			s.notify("textDocument/publishDiagnostics", object{"uri": uri(old), "diagnostics": []object{}})
		}
	}
	for dest, overlay := range overlays {
		if s.w.docs[dest].version == nil {
			s.w.docs[dest] = overlay
		}
	}
	s.w.reload(context.WithoutCancel(ctx))
	s.reportIssues()
	s.publish(ctx)
	return nil
}

type packageMove struct {
	owner *module
	// Only documents whose declared package follows the old directory layout.
	packages map[*psi.Snapshot]string
	paths    map[string]string
}

func (s *server) fileMoves(ctx context.Context, moves []fileMove) (any, error) {
	paths, err := s.w.movePaths(moves, true)
	if err != nil {
		return nil, err
	}
	move := packageMove{paths: paths, packages: map[*psi.Snapshot]string{}}
	for path, dest := range paths {
		if !strings.HasSuffix(path, ".kt") {
			continue
		}
		if !strings.HasSuffix(dest, ".kt") {
			return nil, moveConflict("Destination must be a Kotlin file: %s", dest)
		}
		owner := s.w.p.owner(path)
		if owner == nil || s.w.p.owner(dest) != owner || move.owner != nil && move.owner != owner {
			return nil, moveConflict("Move must stay inside one module")
		}
		move.owner = owner
		if filepath.Dir(path) == filepath.Dir(dest) {
			continue
		}
		snap := s.w.docs[path].snap
		old, ok := filePackage(path, owner)
		if !ok || packageName(snap) != old {
			continue
		}
		next, valid := filePackage(dest, owner)
		if !valid {
			return nil, moveConflict("Invalid package directory for %s", dest)
		}
		if old == next {
			continue
		}
		if old == "" || next == "" {
			return nil, moveConflict("Move must stay within named packages")
		}
		move.packages[snap] = next
	}
	if len(move.packages) == 0 {
		return nil, nil
	}
	affected := map[*module]bool{}
	for _, m := range s.w.p.Modules {
		affected[m] = s.w.p.visibleModules(m)[move.owner]
	}
	for path := range s.w.java {
		if affected[s.w.p.owner(path)] {
			return nil, moveConflict("Move needs Java reference coverage in %s", path)
		}
	}
	changes := map[string][]edit{}
	var documents []string
	scopes := map[*index]*moveScope{}
	for _, path := range slices.Sorted(maps.Keys(s.w.docs)) {
		if !affected[s.w.p.owner(path)] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		snap := s.w.docs[path].snap
		if len(snap.Diagnostics()) != 0 {
			return nil, moveConflict("Resolve syntax errors in %s before moving files", path)
		}
		idx, err := s.w.check(ctx, path)
		if err != nil {
			return nil, err
		}
		scope := scopes[idx]
		if scope == nil {
			scope, err = move.scope(idx, snap)
			if err != nil {
				return nil, err
			}
			scopes[idx] = scope
		}
		edits, err := move.documentEdits(snap, scope)
		if err != nil {
			return nil, err
		}
		if len(edits) > 0 {
			changes[path] = edits
		}
		documents = append(documents, path)
	}
	if err := move.validate(ctx, &s.w, documents, changes); err != nil {
		return nil, err
	}
	result := map[string][]edit{}
	for path, edits := range changes {
		result[uri(path)] = edits
	}
	return s.workspaceEdits(result), nil
}

func packageName(s *psi.Snapshot) string {
	return s.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
}

func (m *packageMove) finalPackage(s *psi.Snapshot) string {
	if pkg, ok := m.packages[s]; ok {
		return pkg
	}
	return packageName(s)
}

// A path can name a family of overloads. An explicit import can follow the
// family only if every declaration it names moves to the same package.
type packageDestination struct {
	name  string
	split bool
}

func recordDestination(destinations map[string]packageDestination, old, next string) {
	d, exists := destinations[old]
	destinations[old] = packageDestination{next, exists && (d.split || d.name != next)}
}

// Return the package qualifier immediately to the left of this resolved
// identifier. In old.f() the selector is the call; in old.C<T> it is a user type.
func packageQualifier(ref psi.NodeRef, pkg string) psi.NodeRef {
	node := ref.Parent()
	if node.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		return psi.NodeRef{}
	}
	parent := node.Parent()
	var qualifier psi.NodeRef
	if parent.NumericKind() == psi.KindUSER_TYPE {
		qualifier = parent.ChildKind(psi.KindUSER_TYPE)
	} else {
		if parent.NumericKind() == psi.KindCALL_EXPRESSION {
			node = parent
			parent = node.Parent()
		}
		if parent.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION && parent.FirstChild() != node {
			qualifier = parent.FirstChild()
		}
	}
	if qualifier.Valid() && dottedPath(qualifier) == pkg {
		return qualifier
	}
	return psi.NodeRef{}
}

// Recognize a plain dotted name without mistaking a receiver expression for a package.
func dottedPath(n psi.NodeRef) string {
	var path strings.Builder
	valid := true
	psi.WalkNodes(n, func(c psi.NodeRef) bool {
		if !c.IsLeaf() {
			return true
		}
		switch c.NumericKind() {
		case psi.KindIDENTIFIER:
			path.WriteString(c.Name())
		case psi.KindDOT:
			path.WriteByte('.')
		default:
			if !c.IsTrivia() {
				valid = false
			}
		}
		return false
	})
	if !valid {
		return ""
	}
	return path.String()
}

type moveReference struct {
	token, declaration psi.NodeRef
}

type moveScope struct {
	names, stars map[string]packageDestination
	refs         map[*psi.Snapshot][]moveReference
}

// Build these tables once per compilation. References are grouped by source
// file so a folder rename visits each resolved use only once.
func (m *packageMove) scope(idx *index, snap *psi.Snapshot) (*moveScope, error) {
	names, stars := map[string]packageDestination{}, map[string]packageDestination{}
	finalNames := map[string]string{}
	for _, d := range idx.decls {
		if d.Parent().NumericKind() != psi.KindFILE || !idx.moduleVisible(d, snap.RootNode()) {
			continue
		}
		s := d.ContainingFile()
		old, next := packageName(s), m.finalPackage(s)
		name := d.Name()
		recordDestination(names, old+"."+name, next)
		recordDestination(stars, old, next)
		final := next + "." + name
		if other, ok := finalNames[final]; ok && other != old {
			return nil, moveConflict("Declaration already present at %s", final)
		}
		finalNames[final] = old
	}
	refs := map[*psi.Snapshot][]moveReference{}
	for ref, target := range idx.targets {
		d := target.Parent()
		if ref != target && d.Parent().NumericKind() == psi.KindFILE && !ancestor(ref, psi.KindIMPORT_DIRECTIVE, psi.KindPACKAGE_DIRECTIVE).Valid() {
			s := ref.ContainingFile()
			refs[s] = append(refs[s], moveReference{ref, d})
		}
	}
	for ref, target := range operatorTargets(idx) {
		d := target.Parent()
		if d.Parent().NumericKind() == psi.KindFILE {
			s := ref.ContainingFile()
			refs[s] = append(refs[s], moveReference{ref, d})
		}
	}
	// Binary declarations may still rely on an old wildcard after all of
	// this module's source declarations leave that package.
	for ref, sym := range idx.symbols {
		if idx.targets[ref].Valid() {
			continue
		}
		if j := strings.LastIndexByte(sym.QualifiedName, '.'); j >= 0 {
			pkg := sym.QualifiedName[:j]
			if _, ok := stars[pkg]; ok {
				recordDestination(stars, pkg, pkg)
			}
		}
	}
	return &moveScope{names, stars, refs}, nil
}

func importExpression(d psi.NodeRef) psi.NodeRef {
	for n := d.FirstChild(); n.Valid(); n = n.NextSibling() {
		if n.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION || n.NumericKind() == psi.KindREFERENCE_EXPRESSION {
			return n
		}
	}
	return psi.NodeRef{}
}

func (m *packageMove) documentEdits(snap *psi.Snapshot, scope *moveScope) ([]edit, error) {
	old, next := packageName(snap), m.finalPackage(snap)
	var edits []edit
	if old != next {
		pkg := snap.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE)
		edits = append(edits, edit{rng(importExpression(pkg)), next})
	}
	type importedName struct{ path, spelling string }
	explicit, wildcard := map[importedName]bool{}, map[string]bool{}
	for c := snap.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.Import().Valid() {
			continue
		}
		path := c.Path()
		expr := importExpression(c)
		if c.IsAllUnder() {
			if d, ok := scope.stars[path]; ok && !d.split && path != d.name {
				edits = append(edits, edit{rng(expr), d.name})
				wildcard[d.name] = true
				continue
			}
			wildcard[path] = true
		} else {
			spelling := c.Alias()
			if spelling == "" {
				spelling = short(path)
			}
			explicit[importedName{path, spelling}] = true
		}
		// Replace the package prefix of an imported top-level declaration.
		// Nested/member suffixes and the alias keep their original spelling.
		for q := expr; q.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION; q = q.FirstChild() {
			d, ok := scope.names[dottedPath(q)]
			if !ok {
				continue
			}
			if d.split {
				return nil, moveConflict("Import %s would be split across packages", path)
			}
			prefix := q.FirstChild()
			if d.name != dottedPath(prefix) {
				edits = append(edits, edit{rng(prefix), d.name})
			}
			break
		}
	}
	imports := map[string]bool{}
	for _, use := range scope.refs[snap] {
		ref, d := use.token, use.declaration
		from, to := packageName(d.ContainingFile()), m.finalPackage(d.ContainingFile())
		if from == to && old == next {
			continue
		}
		if !d.ReceiverType().Valid() {
			if q := packageQualifier(ref, from); q.Valid() {
				if from != to {
					edits = append(edits, edit{rng(q), to})
				}
				continue
			}
		}
		spelling := ref.Name()
		if ref.NumericKind() != psi.KindIDENTIFIER {
			spelling = d.Name()
		}
		if next == to || explicit[importedName{from + "." + d.Name(), spelling}] || wildcard[to] {
			continue
		}
		if to == "" {
			return nil, moveConflict("Cannot import %s from the default package", d.Name())
		}
		imports[to+"."+d.NameIdentifier().Text()] = true
	}
	edits = append(edits, addImports(snap, imports)...)
	slices.SortFunc(edits, func(a, b edit) int {
		if before(a.Range.Start, b.Range.Start) {
			return -1
		}
		if before(b.Range.Start, a.Range.Start) {
			return 1
		}
		if before(a.Range.End, b.Range.End) {
			return -1
		}
		if before(b.Range.End, a.Range.End) {
			return 1
		}
		return strings.Compare(a.NewText, b.NewText)
	})
	edits = slices.Compact(edits)
	for j := 1; j < len(edits); j++ {
		if before(edits[j].Range.Start, edits[j-1].Range.End) {
			return nil, moveConflict("Overlapping reference edits in %s", snap.Path())
		}
	}
	return edits, nil
}
