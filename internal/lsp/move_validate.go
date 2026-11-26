package lsp

import (
	"context"
	"maps"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type moveDelta struct{ start, end, size int }
type movedDocument struct {
	snap   *psi.Snapshot
	deltas []moveDelta
}

// Translate an identifier through package/import edits. A token inside a
// replaced qualifier no longer has a corresponding source position.
func (d movedDocument) token(n psi.NodeRef) psi.NodeRef {
	at := n.Range().Start
	shift := 0
	for _, e := range d.deltas {
		if at < e.start {
			break
		}
		if at < e.end {
			return psi.NodeRef{}
		}
		shift += e.size - (e.end - e.start)
	}
	return d.snap.NodeAt(at + shift)
}

func previewDocument(snap *psi.Snapshot, dest string, edits []edit) (movedDocument, error) {
	var text strings.Builder
	var deltas []moveDelta
	end := 0
	for _, e := range edits {
		a, err := offset(snap, e.Range.Start)
		if err != nil {
			return movedDocument{}, err
		}
		b, err := offset(snap, e.Range.End)
		if err != nil {
			return movedDocument{}, err
		}
		text.WriteString(snap.Source()[end:a])
		text.WriteString(e.NewText)
		end = b
		deltas = append(deltas, moveDelta{a, b, len(e.NewText)})
	}
	text.WriteString(snap.Source()[end:])
	return movedDocument{psi.ParseDocument(dest, text.String()), deltas}, nil
}

// Operator syntax uses a function without spelling its name. The operator
// token (or opening bracket) uniquely identifies the call across import edits.
func operatorTargets(idx *index) map[psi.NodeRef]psi.NodeRef {
	targets := map[psi.NodeRef]psi.NodeRef{}
	for n, f := range idx.facts {
		if f.Call == nil || f.Call.Declaration == nil {
			continue
		}
		var token psi.NodeRef
		switch n.NumericKind() {
		case psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION, psi.KindBINARY_EXPRESSION:
			token = n.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild()
		case psi.KindARRAY_ACCESS_EXPRESSION:
			token = n.ChildKind(psi.KindINDICES).ChildKind(psi.KindLBRACKET)
		}
		if target := idx.node(f.Call.Declaration).NameIdentifier(); token.Valid() && target.Valid() {
			targets[token] = target
		}
	}
	return targets
}

// Check the entire proposed edit in memory. A syntactically valid import can
// silently redirect another use to a different declaration with the same name.
func (m *packageMove) validate(ctx context.Context, w *workspace, paths []string, changes map[string][]edit) error {
	preview := workspace{p: w.p, docs: maps.Clone(w.docs), java: w.java, sessions: map[*module]*ka.Session{}, checked: map[*module]*index{}, owners: map[string]*module{}, visible: map[*module]map[*module]bool{}}
	defer preview.close()
	revisions := map[*psi.Snapshot]movedDocument{}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		dest, moving := m.paths[p]
		if !moving {
			dest = p
		}
		if !moving && len(changes[p]) == 0 {
			continue
		}
		d := w.docs[p]
		r, err := previewDocument(d.snap, dest, changes[p])
		if err != nil {
			return err
		}
		if len(r.snap.Diagnostics()) != 0 {
			return moveConflict("Move produces invalid Kotlin in %s", dest)
		}
		revisions[d.snap] = r
		delete(preview.docs, p)
		preview.docs[dest] = document{r.snap, d.version}
	}
	translated := func(n psi.NodeRef) psi.NodeRef {
		if r, ok := revisions[n.ContainingFile()]; ok {
			return r.token(n)
		}
		return n
	}
	checked := map[*index]bool{}
	for _, p := range paths {
		old := w.checked[w.module(p)]
		if checked[old] {
			continue
		}
		checked[old] = true
		dest := p
		if next, ok := m.paths[p]; ok {
			dest = next
		}
		current, err := preview.check(ctx, dest)
		if err != nil {
			return err
		}
		for ref, target := range old.targets {
			if ref == target || ancestor(ref, psi.KindPACKAGE_DIRECTIVE, psi.KindIMPORT_DIRECTIVE).Valid() {
				continue
			}
			if got := translated(ref); got.Valid() && current.targets[got] != translated(target) {
				return moveConflict("Move changes the binding of %s in %s", ref.Text(), ref.ContainingFile().Path())
			}
		}
		operators := operatorTargets(current)
		for ref, target := range operatorTargets(old) {
			if operators[translated(ref)] != translated(target) {
				return moveConflict("Move changes the binding of %s in %s", ref.Text(), ref.ContainingFile().Path())
			}
		}
		for ref, sym := range old.symbols {
			if old.targets[ref].Valid() || sym.QualifiedName == "" || ancestor(ref, psi.KindIMPORT_DIRECTIVE, psi.KindPACKAGE_DIRECTIVE).Valid() {
				continue
			}
			got := current.symbols[translated(ref)]
			if got.QualifiedName != sym.QualifiedName || got.Kind != sym.Kind || current.targets[translated(ref)].Valid() {
				return moveConflict("Move changes the binding of %s in %s", ref.Text(), ref.ContainingFile().Path())
			}
		}
		type problem struct{ path, code, message string }
		existing := map[problem]int{}
		for _, d := range old.model.Facts.Diagnostics {
			if d.Warning {
				continue
			}
			path := d.Path
			if dest, ok := m.paths[path]; ok {
				path = dest
			}
			existing[problem{path, d.Code, d.Message}]++
		}
		for _, d := range current.model.Facts.Diagnostics {
			if d.Warning {
				continue
			}
			key := problem{d.Path, d.Code, d.Message}
			if existing[key] == 0 {
				return moveConflict("Move introduces an error in %s: %s", d.Path, d.Message)
			}
			existing[key]--
		}
	}
	return nil
}
