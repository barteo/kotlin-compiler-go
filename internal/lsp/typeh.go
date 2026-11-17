package lsp

import (
	"context"
	"encoding/json"
	"maps"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// typeData is a type hierarchy item's data: its class's qualified name,
// and the source file whose module the hierarchy was asked in, which
// finds a library class again.
type typeData struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s *Server) prepareTypeHierarchy(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.TextDocumentPositionParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	c, ok, err := s.caretAt(p)
	if err != nil || !ok {
		return nil, err
	}
	it, ok := ide.PrepareTypeHierarchy(s.withAll(c.views), c.path, c.off, s.libraryClass(c.path))
	if !ok {
		return nil, nil
	}
	item, ok := s.locator().typeItem(it, c.path)
	if !ok {
		return nil, nil
	}
	return []protocol.TypeHierarchyItem{item}, nil
}

func (s *Server) supertypes(_ context.Context, r *jsonrpc.Request) (any, error) {
	return s.hierarchy(r, func(d psi.NodeRef, data typeData, c caret) []ide.TypeItem {
		return ide.Supertypes(s.withAll(c.views), d, s.libraryClass(data.Path))
	})
}

func (s *Server) subtypes(_ context.Context, r *jsonrpc.Request) (any, error) {
	return s.hierarchy(r, func(_ psi.NodeRef, data typeData, c caret) []ide.TypeItem {
		// A class's subtypes are in its module and the modules that
		// depend on it, a library class's in any: all modules'.
		return ide.Subtypes(s.withAll(c.views), data.Name)
	})
}

// allViews returns the last views of all modules.
func (s *Server) allViews() []*ide.View {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	var out []*ide.View
	for _, w := range ws {
		w.mu.Lock()
		mods := make([]*module, 0, len(w.mods))
		for _, name := range slices.Sorted(maps.Keys(w.mods)) {
			mods = append(mods, w.mods[name])
		}
		w.mu.Unlock()
		for _, m := range mods {
			m.mu.Lock()
			if m.last != nil {
				out = append(out, m.last)
			}
			m.mu.Unlock()
		}
	}
	return out
}

// withAll returns views, then the other modules' last views.
func (s *Server) withAll(views []*ide.View) []*ide.View {
	out := slices.Clone(views)
	for _, v := range s.allViews() {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// hierarchy answers a type hierarchy request for an item's supertypes or
// subtypes: find gets the item's class, of the sources or the library,
// its data, and the caret at its name, or of the module it was asked in
// for a library class.
func (s *Server) hierarchy(r *jsonrpc.Request, find func(psi.NodeRef, typeData, caret) []ide.TypeItem) (any, error) {
	var p protocol.TypeHierarchyParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	var data typeData
	if b, err := json.Marshal(p.Item.Data); err != nil || json.Unmarshal(b, &data) != nil || data.Name == "" {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "type hierarchy item without kc's data")
	}
	var d psi.NodeRef
	c, ok, err := s.caretIn(protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: p.Item.URI}, Position: p.Item.SelectionRange.Start})
	if err != nil {
		return nil, err
	}
	if ok {
		c = s.widen(c)
		d = ide.TypeDecl(c.views[0], c.path, c.off)
	} else {
		c = caret{path: data.Path, views: s.views(data.Path)}
		if lib := s.libraryClass(data.Path); lib != nil {
			d, _ = lib(libsrc.Target{Kind: "class", Name: data.Name})
		}
	}
	out := []protocol.TypeHierarchyItem{}
	if !d.Valid() {
		return out, nil
	}
	l := s.locator()
	for _, it := range find(d, data, c) {
		if item, ok := l.typeItem(it, data.Path); ok {
			out = append(out, item)
		}
	}
	return out, nil
}

// libraryClass finds a class of the library of path's module, as
// libraryDecl does; nil if neither sources nor stubs are on.
func (s *Server) libraryClass(path string) ide.LibraryClass {
	f := s.libraryDecl(path)
	if f == nil {
		return nil
	}
	return f
}

// typeItem converts an item, asked about in path's module, unless its
// declaration was edited since.
func (l *locator) typeItem(it ide.TypeItem, path string) (protocol.TypeHierarchyItem, bool) {
	ci, ok := l.callItem(it.CallItem)
	if !ok {
		return protocol.TypeHierarchyItem{}, false
	}
	return protocol.TypeHierarchyItem{
		Name: ci.Name, Kind: ci.Kind, Detail: ci.Detail, URI: ci.URI, Range: ci.Range, SelectionRange: ci.SelectionRange,
		Data: typeData{Name: it.Qualified, Path: path},
	}, true
}
