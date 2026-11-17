package lsp

import (
	"context"
	"fmt"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// codeAction offers the quick fixes of the document's diagnostics in the
// range, the refactorings of the range, and organize imports. The diagnostics are the server's own, as
// last shown with the document's current text, not the client's copy:
// those have the fix data, and a diagnostic whose text was edited since
// was dropped and has no fix.
func (s *Server) codeAction(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.CodeActionParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	from, err := d.Offset(pos(p.Range.Start), enc)
	if err != nil {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%v", err)
	}
	to, err := d.Offset(pos(p.Range.End), enc)
	if err != nil {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%v", err)
	}
	want := func(kind string) bool {
		if len(p.Context.Only) == 0 {
			return true
		}
		for _, k := range p.Context.Only {
			if kind == k || strings.HasPrefix(kind, k+".") {
				return true
			}
		}
		return false
	}
	wantRefactor := len(p.Context.Only) == 0
	for _, k := range p.Context.Only {
		wantRefactor = wantRefactor || k == "refactor" || strings.HasPrefix(k, "refactor.")
	}
	var v *ide.View
	if views := s.views(d.Path); len(views) > 0 {
		v = views[0]
	}
	l := s.locator()
	actions := []protocol.CodeAction{}
	if want(protocol.CodeActionQuickFix) {
		s.mu.Lock()
		sem := s.sem[p.TextDocument.URI]
		s.mu.Unlock()
		if sem.snap == d.Snap {
			env := ide.FixEnv{Diagnostics: sem.diags, Importables: s.importables(d.Path)}
			seen := map[string]bool{}
			for _, e := range sem.diags {
				if e.Range.End < from || e.Range.Start > to {
					continue
				}
				for _, fx := range ide.QuickFixes(v, d, e, env) {
					we, ok := l.fixEdit(fx.Edits)
					if !ok {
						continue
					}
					// The members a class doesn't implement are a
					// diagnostic each, with the same fix of them all.
					k := fmt.Sprint(fx.Title, we)
					if seen[k] {
						continue
					}
					seen[k] = true
					actions = append(actions, protocol.CodeAction{
						Title: fx.Title, Kind: protocol.CodeActionQuickFix, IsPreferred: fx.Preferred,
						Diagnostics: []protocol.Diagnostic{protocolDiagnostic(d, e, enc)}, Edit: we,
					})
				}
			}
		}
	}
	if wantRefactor && v != nil {
		for _, fx := range ide.Refactors(v, d, psi.Range{Start: from, End: to}, ide.FixEnv{}) {
			if !want(fx.Kind) {
				continue
			}
			if we, ok := l.fixEdit(fx.Edits); ok {
				actions = append(actions, protocol.CodeAction{Title: fx.Title, Kind: fx.Kind, Edit: we})
			}
		}
	}
	if want(protocol.CodeActionOrganizeImports) && v != nil {
		if e, ok := ide.OrganizeImports(v, d.Path); ok {
			if we, ok := l.fixEdit([]ide.FixEdit{{Loc: ide.Loc{Snap: v.Snapshot(d.Path), Range: e.Range}, Text: e.Text}}); ok {
				actions = append(actions, protocol.CodeAction{Title: "Organize imports", Kind: protocol.CodeActionOrganizeImports, Edit: we})
			}
		}
	}
	return actions, nil
}

// fixEdit converts a fix's edits of views' or documents' texts to the
// documents'; false if one falls in text edited since.
func (l *locator) fixEdit(edits []ide.FixEdit) (*protocol.WorkspaceEdit, bool) {
	we := &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{}}
	for _, e := range edits {
		mp, r, ok := l.rangeOf(e.Loc)
		if !ok {
			return nil, false
		}
		we.Changes[mp.uri] = append(we.Changes[mp.uri], protocol.TextEdit{Range: rng(mp.doc, r, l.enc), NewText: e.Text})
	}
	return we, true
}

// importables returns the lookup of declarations that path's file may
// import, in its module; nil if no module has the file.
func (s *Server) importables(path string) func(string) []analysis.Importable {
	m := s.ownerModule(path)
	if m == nil {
		return nil
	}
	return func(name string) []analysis.Importable {
		snaps, err := m.snapshots(nil)
		if err != nil {
			return nil
		}
		ims, err := m.sess.Importables(snaps, path, name)
		if err != nil {
			s.logMessage(protocol.MessageWarning, "code action: "+err.Error())
		}
		return ims
	}
}
