package lsp

import (
	"context"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// The navigation features answer from the modules' last updates (views),
// which may be of older texts than the documents': a position maps to
// the view's text, and a result back to the document's, through the span
// the edits since changed. A position in that span finds nothing until
// the next update.

// views returns the last views of the modules whose sources include
// path, the module that owns it first.
func (s *Server) views(path string) []*ide.View {
	var out []*ide.View
	for _, m := range s.modulesOf(path) {
		m.w.mu.Lock()
		owner := m.w.proj != nil && m.w.proj.ModuleOf(path) == m.m
		m.w.mu.Unlock()
		m.mu.Lock()
		v := m.last
		m.mu.Unlock()
		if v == nil || v.Snapshot(path) == nil {
			continue
		}
		if owner {
			out = append([]*ide.View{v}, out...)
		} else {
			out = append(out, v)
		}
	}
	return out
}

// caret is a request's position, as an offset of its first view's
// snapshot.
type caret struct {
	path  string
	off   int
	views []*ide.View
}

// caretAt resolves a position of an open document; ok is false if no
// view has the document, or the position is in text edited since.
func (s *Server) caretAt(p protocol.TextDocumentPositionParams) (c caret, ok bool, err error) {
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return caret{}, false, err
	}
	off, err := d.Offset(pos(p.Position), enc)
	if err != nil {
		return caret{}, false, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%v", err)
	}
	views := s.views(d.Path)
	if len(views) == 0 {
		return caret{}, false, nil
	}
	if snap := views[0].Snapshot(d.Path); snap != d.Snap {
		if off, ok = ide.NewMapper(snap.Source(), d.Text()).Back(off); !ok {
			return caret{}, false, nil
		}
	}
	return caret{d.Path, off, views}, true, nil
}

// locator converts views' locations to a request's: the open documents'
// texts, or the analyzed ones of closed files.
type locator struct {
	s       *Server
	enc     ide.Encoding
	mappers map[*psi.Snapshot]mapping
}

type mapping struct {
	doc *ide.Document
	uri protocol.DocumentURI
	m   *ide.Mapper // nil if doc is the snapshot's
}

func (s *Server) locator() *locator {
	s.mu.Lock()
	enc := s.enc
	s.mu.Unlock()
	return &locator{s: s, enc: enc, mappers: map[*psi.Snapshot]mapping{}}
}

// rangeOf returns the document and range a location is now at.
func (l *locator) rangeOf(loc ide.Loc) (mapping, psi.Range, bool) {
	mp, ok := l.mappers[loc.Snap]
	if !ok {
		path := loc.Snap.Path()
		l.s.mu.Lock()
		u, open := l.s.paths[path]
		var d *ide.Document
		if open {
			d = l.s.docs[u]
		}
		l.s.mu.Unlock()
		switch {
		case !open:
			mp = mapping{doc: ide.DocumentOf(loc.Snap, 0), uri: fileURI(path)}
		case d.Snap == loc.Snap:
			mp = mapping{doc: d, uri: u}
		default:
			m := ide.NewMapper(loc.Snap.Source(), d.Text())
			mp = mapping{doc: d, uri: u, m: &m}
		}
		l.mappers[loc.Snap] = mp
	}
	r := loc.Range
	if mp.m != nil {
		var ok bool
		if r, ok = mp.m.Range(r); !ok {
			return mapping{}, psi.Range{}, false
		}
	}
	return mp, r, true
}

// locations converts locations, dropping those edited since.
func (l *locator) locations(locs []ide.Loc) []protocol.Location {
	out := []protocol.Location{}
	for _, loc := range locs {
		if mp, r, ok := l.rangeOf(loc); ok {
			out = append(out, protocol.Location{URI: mp.uri, Range: rng(mp.doc, r, l.enc)})
		}
	}
	return out
}

func (s *Server) definition(_ context.Context, r *jsonrpc.Request) (any, error) {
	return s.navigate(r, func(c caret) []ide.Loc { return ide.Definition(c.views[0], c.path, c.off) })
}

func (s *Server) typeDefinition(_ context.Context, r *jsonrpc.Request) (any, error) {
	return s.navigate(r, func(c caret) []ide.Loc { return ide.TypeDefinition(c.views[0], c.path, c.off) })
}

// navigate answers a request for locations found from its position.
func (s *Server) navigate(r *jsonrpc.Request, find func(caret) []ide.Loc) (any, error) {
	var p protocol.TextDocumentPositionParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	c, ok, err := s.caretAt(p)
	if err != nil || !ok {
		return []protocol.Location{}, err
	}
	return s.locator().locations(find(c)), nil
}

func (s *Server) references(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.ReferenceParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	c, ok, err := s.caretAt(p.TextDocumentPositionParams)
	if err != nil || !ok {
		return []protocol.Location{}, err
	}
	return s.locator().locations(ide.References(c.views, c.path, c.off, p.Context.IncludeDeclaration)), nil
}

func (s *Server) documentHighlight(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.TextDocumentPositionParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	out := []protocol.DocumentHighlight{}
	c, ok, err := s.caretAt(p)
	if err != nil || !ok {
		return out, err
	}
	l := s.locator()
	snap := c.views[0].Snapshot(c.path)
	for _, h := range ide.Highlights(c.views[0], c.path, c.off) {
		mp, r, ok := l.rangeOf(ide.Loc{Snap: snap, Range: h.Range})
		if !ok {
			continue
		}
		kind := protocol.HighlightRead
		if h.Kind == ide.Write {
			kind = protocol.HighlightWrite
		}
		out = append(out, protocol.DocumentHighlight{Range: rng(mp.doc, r, l.enc), Kind: kind})
	}
	return out, nil
}
