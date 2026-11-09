package lsp

import (
	"context"
	"encoding/json"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
)

func (s *Server) prepareCallHierarchy(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.TextDocumentPositionParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	c, ok, err := s.caretAt(p)
	if err != nil || !ok {
		return nil, err
	}
	it, ok := ide.PrepareCallHierarchy(c.views[0], c.path, c.off)
	var data *callData
	if !ok {
		var t libsrc.Target
		if it, t, ok = ide.PrepareLibraryCallHierarchy(c.views[0], c.path, c.off, s.libraryClass(c.path)); !ok {
			return nil, nil
		}
		data = &callData{Target: t, Path: c.path}
	}
	item, ok := s.locator().callItem(it)
	if !ok {
		return nil, nil
	}
	if data != nil {
		item.Data = data
	}
	return []protocol.CallHierarchyItem{item}, nil
}

// callData is the data of a library declaration's call hierarchy item:
// its target, and the source file whose module it was asked in.
type callData struct {
	Target libsrc.Target `json:"target"`
	Path   string        `json:"path"`
}

// libraryCalls returns the incoming calls in all modules' sources of the
// library's declaration an item names; false if it names none.
func (s *Server) libraryCalls(item protocol.CallHierarchyItem) ([]ide.CallerCall, bool) {
	var d callData
	if b, err := json.Marshal(item.Data); err != nil || item.Data == nil || json.Unmarshal(b, &d) != nil || d.Target.Name == "" {
		return nil, false
	}
	return ide.LibraryIncomingCalls(s.allViews(), d.Target), true
}

func (s *Server) incomingCalls(_ context.Context, r *jsonrpc.Request) (any, error) {
	return s.calls(r, func(c caret) []ide.CallerCall { c = s.widen(c); return ide.IncomingCalls(c.views, c.path, c.off) }, func(l *locator, from ide.CallerCall, item protocol.CallHierarchyItem, rs []protocol.Range) any {
		return protocol.CallHierarchyIncomingCall{From: item, FromRanges: rs}
	})
}

func (s *Server) outgoingCalls(_ context.Context, r *jsonrpc.Request) (any, error) {
	return s.calls(r, func(c caret) []ide.CallerCall {
		return ide.OutgoingCalls(c.views[0], c.path, c.off, s.libraryDecl(c.path))
	}, func(l *locator, to ide.CallerCall, item protocol.CallHierarchyItem, rs []protocol.Range) any {
		return protocol.CallHierarchyOutgoingCall{To: item, FromRanges: rs}
	})
}

// calls answers a call hierarchy request for an item's calls, found at
// the item's name. Incoming calls' ranges are in the caller's file, the
// item of each group; outgoing calls' in the item's asked about.
func (s *Server) calls(r *jsonrpc.Request, find func(caret) []ide.CallerCall, wrap func(*locator, ide.CallerCall, protocol.CallHierarchyItem, []protocol.Range) any) (any, error) {
	var p protocol.CallHierarchyCallsParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	incoming := r.Method == "callHierarchy/incomingCalls"
	if calls, ok := s.libraryCalls(p.Item); ok {
		// A library declaration's: its calls in the sources; it makes
		// none the sources tell of.
		out := []any{}
		if !incoming {
			return out, nil
		}
		l := s.locator()
		for _, cc := range calls {
			item, ok := l.callItem(cc.Item)
			if !ok {
				continue
			}
			var rs []protocol.Range
			for _, rg := range cc.Ranges {
				if mp, rg, ok := l.rangeOf(ide.Loc{Snap: cc.Item.Snap, Range: rg}); ok {
					rs = append(rs, rng(mp.doc, rg, l.enc))
				}
			}
			if len(rs) > 0 {
				out = append(out, wrap(l, cc, item, rs))
			}
		}
		return out, nil
	}
	c, ok, err := s.caretIn(protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: p.Item.URI}, Position: p.Item.SelectionRange.Start})
	if err != nil || !ok {
		return []any{}, err
	}
	out := []any{}
	l := s.locator()
	for _, cc := range find(c) {
		item, ok := l.callItem(cc.Item)
		if !ok {
			continue
		}
		snap := cc.Item.Snap
		if !incoming {
			snap = c.views[0].Snapshot(c.path)
		}
		var rs []protocol.Range
		for _, rg := range cc.Ranges {
			if mp, rg, ok := l.rangeOf(ide.Loc{Snap: snap, Range: rg}); ok {
				rs = append(rs, rng(mp.doc, rg, l.enc))
			}
		}
		if len(rs) > 0 {
			out = append(out, wrap(l, cc, item, rs))
		}
	}
	return out, nil
}

// caretIn is caretAt for a position of any file a module analyzed, open
// or not: an item's.
func (s *Server) caretIn(p protocol.TextDocumentPositionParams) (caret, bool, error) {
	s.mu.Lock()
	_, open := s.docs[p.TextDocument.URI]
	enc := s.enc
	s.mu.Unlock()
	if open {
		return s.caretAt(p)
	}
	path := uriPath(p.TextDocument.URI)
	views := s.views(path)
	if len(views) == 0 {
		return caret{}, false, nil
	}
	off, err := ide.DocumentOf(views[0].Snapshot(path), 0).Offset(pos(p.Position), enc)
	if err != nil {
		return caret{}, false, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%v", err)
	}
	return caret{path, off, views}, true, nil
}

// callItem converts an item, unless its declaration was edited since.
func (l *locator) callItem(it ide.CallItem) (protocol.CallHierarchyItem, bool) {
	mp, whole, ok := l.rangeOf(ide.Loc{Snap: it.Snap, Range: it.Range})
	if !ok {
		return protocol.CallHierarchyItem{}, false
	}
	_, name, ok := l.rangeOf(ide.Loc{Snap: it.Snap, Range: it.NameRange})
	if !ok {
		return protocol.CallHierarchyItem{}, false
	}
	return protocol.CallHierarchyItem{
		Name: it.Name, Kind: symbolKind(it.Kind), Detail: it.Detail, URI: mp.uri,
		Range: rng(mp.doc, whole, l.enc), SelectionRange: rng(mp.doc, name, l.enc),
	}, true
}
