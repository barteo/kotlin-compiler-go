package lsp

import (
	"context"
	"fmt"
	"maps"
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
	return name, len(s.Diagnostics()) == 0
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
func (s *server) fileMoves(ctx context.Context, moves []fileMove) (any, error) {
	if len(moves) != 1 {
		return nil, rpcError{-32803, "Move one Kotlin file at a time"}
	}
	path, err := filePath(moves[0].OldURI)
	if err != nil {
		return nil, err
	}
	dest, err := filePath(moves[0].NewURI)
	if err != nil {
		return nil, err
	}
	d, ok := s.w.docs[path]
	if !ok || !strings.HasSuffix(dest, ".kt") {
		return nil, nil
	}
	owner := s.w.module(path)
	old, ok := filePackage(path, owner)
	newPkg, newOK := filePackage(dest, owner)
	if !ok || !newOK || s.w.p.owner(dest) != owner {
		return nil, rpcError{-32803, "Move must stay inside one module"}
	}
	pkg := d.snap.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE)
	if pkg.Path() != old || old == newPkg {
		return nil, nil
	}
	if old == "" || newPkg == "" {
		return nil, rpcError{-32803, "Moving into or out of the default package requires explicit package edits"}
	}
	if len(s.w.java) > 0 {
		return nil, rpcError{-32803, "Move needs Java reference coverage"}
	}
	moved := map[string]string{}
	for _, decl := range d.snap.DeclarationNodes() {
		if decl.Name() != "" {
			moved[old+"."+decl.Name()] = newPkg + "." + decl.Name()
		}
	}
	changes := map[string][]edit{}
	changes[uri(path)] = []edit{{rng(pkg), "package " + newPkg}}
	for _, f := range slices.Sorted(maps.Keys(s.w.docs)) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		snap := s.w.docs[f].snap
		root := snap.RootNode()
		if len(snap.Diagnostics()) > 0 {
			return nil, rpcError{-32803, "Resolve syntax errors before moving a file"}
		}
		if root.ChildKind(psi.KindPACKAGE_DIRECTIVE).Path() == newPkg {
			for _, decl := range snap.DeclarationNodes() {
				if _, conflict := moved[old+"."+decl.Name()]; conflict {
					return nil, rpcError{-32803, "Declaration name already present in destination package"}
				}
			}
		}
		idx, err := s.w.check(ctx, f)
		if err != nil {
			return nil, err
		}
		var edits []edit
		imports := map[string]bool{}
		explicit := map[string]bool{}
		for c := root.ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
			name := c.Path()
			if name == old && c.IsAllUnder() {
				return nil, rpcError{-32803, "Resolve the wildcard import before moving its declarations"}
			}
			for source, dest := range moved {
				if name != source && !strings.HasPrefix(name, source+".") {
					continue
				}
				explicit[source] = true
				for node := c.FirstChild(); node.Valid(); node = node.NextSibling() {
					if node.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION {
						edits = append(edits, edit{rng(node), dest + strings.TrimPrefix(name, source)})
						break
					}
				}
			}
		}
		for ref, target := range idx.targets {
			if ref.ContainingFile() != snap || ref == target || ancestor(ref, psi.KindIMPORT_DIRECTIVE).Valid() {
				continue
			}
			top := target.Parent()
			for top.Valid() && top.Parent().NumericKind() != psi.KindFILE {
				top = top.Parent()
			}
			if !top.Valid() || top.Name() == "" {
				continue
			}
			if top.ContainingFile() == d.snap && f != path {
				src := old + "." + top.Name()
				fq := moved[src]
				q := ancestor(ref, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindUSER_TYPE)
				for q.Valid() && strings.HasPrefix(q.Text(), old+".") && q.Range().End >= ref.Range().End {
					if q.Text() == src {
						edits = append(edits, edit{rng(q), fq})
						break
					}
					q = q.Parent()
				}
				if q.Valid() && q.Text() == src {
					continue
				}
				if strings.Contains(snap.Source(), src) && !explicit[src] {
					return nil, rpcError{-32803, "Move requires an unsupported qualified reference edit"}
				}
				if !explicit[src] && root.ChildKind(psi.KindPACKAGE_DIRECTIVE).Path() != newPkg {
					imports[fq] = true
				}
			} else if f == path && top.ContainingFile() != d.snap && top.ContainingFile().RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path() == old {
				imports[old+"."+top.Name()] = true
			}
		}
		ed := append(edits, addImports(snap, imports)...)
		if len(ed) > 0 {
			changes[uri(f)] = append(changes[uri(f)], ed...)
		}
	}
	for u, edits := range changes {
		slices.SortFunc(edits, func(a, b edit) int {
			if before(a.Range.Start, b.Range.Start) {
				return -1
			}
			if before(b.Range.Start, a.Range.Start) {
				return 1
			}
			return strings.Compare(a.NewText, b.NewText)
		})
		edits = slices.Compact(edits)
		for j := 1; j < len(edits); j++ {
			if before(edits[j].Range.Start, edits[j-1].Range.End) {
				return nil, fmt.Errorf("overlapping package and reference edits in %s", u)
			}
		}
		changes[u] = edits
	}
	return s.workspaceEdits(changes), nil
}
