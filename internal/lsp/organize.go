package lsp

import (
	"context"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type importUsage struct {
	refs              map[string][]psi.NodeRef // indexed by spelling, including aliases
	docNames          map[string]bool
	implicit          map[string]bool // qualified names of implicit calls
	uncertainOperator bool
	source            map[string]bool // known top-level names; true for an operator overload
}

func collectImportUsage(s *psi.Snapshot, i *index) *importUsage {
	u := &importUsage{refs: map[string][]psi.NodeRef{}, docNames: map[string]bool{}, implicit: map[string]bool{}, source: map[string]bool{}}
	for _, d := range i.decls {
		if d.Parent().NumericKind() == psi.KindFILE {
			path := d.Name()
			if pkg := packageName(d.ContainingFile()); pkg != "" {
				path = pkg + "." + path
			}
			u.source[path] = u.source[path] || d.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindOPERATOR_KEYWORD).Valid()
		}
	}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindIMPORT_LIST, psi.KindPACKAGE_DIRECTIVE:
			return false
		case psi.KindKDOC:
			kdocImportNames(n.Text(), u.docNames)
			return false
		case psi.KindIDENTIFIER:
			if i.targets[n] != n && n.Parent().NameIdentifier() == n && !n.Parent().Declaration().Valid() {
				u.refs[n.Name()] = append(u.refs[n.Name()], n)
			}
		case psi.KindBINARY_EXPRESSION, psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION, psi.KindARRAY_ACCESS_EXPRESSION:
			if call := i.facts[n].Call; call != nil {
				u.implicit[call.Name] = true
				if decl := i.node(call.Declaration); decl.Valid() {
					u.implicit[packageName(decl.ContainingFile())+"."+decl.Name()] = true
				}
			} else {
				u.uncertainOperator = true
			}
		case psi.KindFOR, psi.KindDESTRUCTURING_DECLARATION, psi.KindPROPERTY_DELEGATE:
			// Not every convention call has a separately addressable fact.
			u.uncertainOperator = true
		}
		return true
	})
	return u
}

// Keep an unrecognized import. In particular, the project may be missing a
// dependency while the user is editing its build or source files.
func (u *importUsage) needed(i *index, imp psi.NodeRef) bool {
	path := imp.Path()
	name := imp.Alias()
	if name == "" {
		name = short(path)
	}
	if u.docNames[name] || u.implicit[path] {
		return true
	}
	pkg, simple := "", path
	if j := strings.LastIndexByte(path, '.'); j >= 0 {
		pkg, simple = path[:j], path[j+1:]
	}
	for _, ref := range u.refs[name] {
		f := i.facts[ref.Parent()]
		sym := i.symbols[ref]
		if sym.Kind == "local" || sym.Kind == "parameter" {
			continue
		}
		// Type aliases and constructor calls can carry their importable
		// name on the enclosing type or call instead of the identifier.
		fq := f.ImportableName
		for n := ref.Parent().Parent(); n.Valid() && factName(n) == ref && fq == ""; n = n.Parent() {
			fq = i.facts[n].ImportableName
		}
		if q := packageQualifier(ref, pkg); q.Valid() && !(f.Call != nil && f.Call.Extension) {
			// A value can have the same spelling as the package of an
			// extension property (or function).
			if receiver := i.symbols[factName(q)]; receiver.Kind == "" {
				continue
			}
		}
		if fq == "" {
			fq = strings.TrimSuffix(sym.QualifiedName, ".<init>")
		}
		if fq == "" || fq == path {
			return true
		}
	}
	operator, known := u.source[path]
	if !known {
		if p := i.model.Package(pkg); p != nil {
			known = slices.Contains(p.Classes, strings.ReplaceAll(path, ".", "/"))
			for _, d := range p.Decls {
				if d.Name == simple {
					known = true
					operator = operator || d.Flags&ka.FlagOperator != 0
				}
			}
		}
	}
	return !known || operator && (u.uncertainOperator || name == "invoke")
}

func organize(ctx context.Context, s *psi.Snapshot, i *index) []edit {
	list := s.RootNode().ChildKind(psi.KindIMPORT_LIST)
	if !list.ChildKind(psi.KindIMPORT_DIRECTIVE).Valid() || len(s.Diagnostics()) != 0 {
		return nil
	}
	var usage *importUsage
	if i != nil {
		if edits, ok := i.importEdits[s]; ok {
			return edits
		}
		prune := true
		for _, d := range i.model.Facts.Diagnostics {
			if !d.Warning && d.Path == s.Path() {
				prune = false
				break
			}
		}
		if prune {
			usage = collectImportUsage(s, i)
		}
	}
	src := s.Source()
	sep := "\n"
	if strings.Contains(src, "\r\n") {
		sep = "\r\n"
	}
	var edits []edit
	var group, keep []psi.NodeRef
	flush := func() {
		if len(group) == 0 {
			return
		}
		a, b := group[0].Range().Start, group[len(group)-1].Range().End
		// Sort on the imported name so redundant spelling or whitespace
		// doesn't affect ordering. Keep the original text of the first copy.
		slices.SortStableFunc(keep, compareImports)
		keep = slices.CompactFunc(keep, func(a, b psi.NodeRef) bool { return compareImports(a, b) == 0 })
		lines := make([]string, 0, len(keep))
		for _, imp := range keep {
			lines = append(lines, imp.Text())
		}
		text := strings.Join(lines, sep)
		if text == "" && strings.HasPrefix(src[b:], sep) {
			b += len(sep)
		}
		if src[a:b] != text {
			edits = append(edits, edit{span{pos(s, a), pos(s, b)}, text})
		}
		group, keep = nil, nil
	}
	for c := list.FirstChild(); c.Valid(); c = c.NextSibling() {
		if ctx.Err() != nil {
			return nil
		}
		if c.NumericKind() == psi.KindWHITE_SPACE {
			continue
		}
		if !c.Import().Valid() || !plainImportLine(src, c) {
			flush()
			continue
		}
		group = append(group, c)
		if usage == nil || c.IsAllUnder() || usage.needed(i, c) {
			keep = append(keep, c)
		}
	}
	flush()
	if i != nil && ctx.Err() == nil {
		if len(edits) > 0 && !i.checkIntention(ctx, s, edits, i.importBindingsUnchanged) {
			return nil
		}
		// Cursor movement may request code actions repeatedly on the same
		// revision. The workspace replaces this index on dependency edits.
		if i.importEdits == nil {
			i.importEdits = make(map[*psi.Snapshot][]edit)
		}
		i.importEdits[s] = edits
	}
	return edits
}

// Kotlin's default layout puts aliases after ordinary imports.
func compareImports(a, b psi.NodeRef) int {
	aa, ba := a.Alias(), b.Alias()
	if aa == "" && ba != "" {
		return -1
	}
	if aa != "" && ba == "" {
		return 1
	}
	ap, bp := a.Path(), b.Path()
	if a.IsAllUnder() {
		ap += ".*"
	}
	if b.IsAllUnder() {
		bp += ".*"
	}
	if c := strings.Compare(ap, bp); c != 0 {
		return c
	}
	return strings.Compare(aa, ba)
}

// Sort independent runs. Leave commented directives and their adjacent
// documentation in place, including same-line comments outside the PSI node.
func plainImportLine(src string, imp psi.NodeRef) bool {
	r := imp.Range()
	line := strings.LastIndexByte(src[:r.Start], '\n') + 1
	tail, _, _ := strings.Cut(src[r.End:], "\n")
	if strings.TrimSpace(src[line:r.Start]) != "" || strings.TrimSpace(tail) != "" {
		return false
	}
	if strings.Contains(imp.Text(), "/*") || strings.Contains(imp.Text(), "//") || strings.Contains(imp.Text(), "\n") {
		return false
	}
	if line > 0 {
		prev := src[:line-1]
		prev = strings.TrimSpace(prev[strings.LastIndexByte(prev, '\n')+1:])
		if strings.HasPrefix(prev, "//") || strings.HasSuffix(prev, "*/") {
			return false
		}
	}
	return true
}

// An import can select a different overload without introducing a diagnostic.
// Check resolved uses in the unsaved overlay, including calls without a name
// token (operators and indexed access). Edits are confined to import lines,
// so the rest of the document has the same syntax tree at translated offsets.
func (i *index) importBindingsUnchanged(next *index, preview movedDocument) bool {
	path := preview.snap.Path()
	for ref, target := range i.targets {
		if ref.ContainingFile().Path() != path || ref == target || ancestor(ref, psi.KindIMPORT_DIRECTIVE).Valid() {
			continue
		}
		if target.ContainingFile().Path() == path {
			target = preview.token(target)
		}
		if next.targets[preview.token(ref)] != target {
			return false
		}
	}
	for n, f := range i.facts {
		if n.ContainingFile().Path() != path || ancestor(n, psi.KindIMPORT_DIRECTIVE, psi.KindPACKAGE_DIRECTIVE).Valid() {
			continue
		}
		if f.Call == nil && f.Symbol == 0 && f.ImportableName == "" {
			continue
		}
		mapped := ancestor(preview.token(n), n.NumericKind())
		// For example a.b().c() and a.b() begin at the same token.
		for mapped.Valid() && mapped.Range().End-mapped.Range().Start < n.Range().End-n.Range().Start {
			mapped = ancestor(mapped.Parent(), n.NumericKind())
		}
		g := next.facts[mapped]
		if f.ImportableName != g.ImportableName {
			return false
		}
		if f.Symbol != 0 {
			if g.Symbol == 0 {
				return false
			}
			before, after := i.model.Facts.Symbols[f.Symbol-1], next.model.Facts.Symbols[g.Symbol-1]
			if before.Kind != after.Kind || before.QualifiedName != after.QualifiedName {
				return false
			}
		}
		if f.Call != nil && !i.sameCall(next, preview, f.Call, g.Call) {
			return false
		}
	}
	return true
}
