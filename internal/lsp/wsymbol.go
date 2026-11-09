package lsp

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
)

// The library's declarations in workspace/symbol: after the sources',
// the top-level declarations of a module's class path whose names match
// the query. A client that resolves a symbol's range gets them with only
// a URI, and their locations, which extract a file of the library's
// sources or render a stub, from workspaceSymbol/resolve; another gets a
// few, located at once.

const (
	maxLibrarySymbols      = 100
	maxEagerLibrarySymbols = 20
	minLibraryQuery        = 2 // a shorter query matches too much to list
)

// librarySymbol is a library declaration a workspace symbol names, and
// its data for workspaceSymbol/resolve.
type librarySymbol struct {
	Simple  string `json:"simple"`
	Package string `json:"package"`
	Class   bool   `json:"class"`
	Path    string `json:"path"` // a source file of the module whose library has it
	score   int
}

func (l librarySymbol) kind() int {
	if l.Class {
		return protocol.SymbolClass
	}
	return protocol.SymbolFunction
}

func (l librarySymbol) name() string {
	if l.Package == "" {
		return l.Simple
	}
	return l.Package + "." + l.Simple
}

// librarySymbols returns the library's declarations whose names match
// query, best first, at most limit; of the library of the first module
// with sources analyzed.
func (s *Server) librarySymbols(query string, limit int) []librarySymbol {
	if len(query) < minLibraryQuery || s.lib.off && s.lib.noStubs {
		return nil // too short, or nothing could locate them
	}
	var path string
	for _, v := range s.allViews() {
		if ps := v.Paths(); len(ps) > 0 {
			path = ps[0]
			break
		}
	}
	m := s.ownerModule(path)
	if m == nil {
		return nil
	}
	var out []librarySymbol
	seen := map[analysis.Importable]bool{}
	err := m.sess.LibraryImportables(func(im analysis.Importable) bool {
		if score, ok := ide.MatchScore(im.Name, query); ok && !seen[im] {
			seen[im] = true
			out = append(out, librarySymbol{Simple: im.Name, Package: im.Package, Class: im.Class, Path: path, score: score})
		}
		return true
	})
	if err != nil {
		s.logMessage(protocol.MessageWarning, "workspace symbols: "+err.Error())
	}
	slices.SortFunc(out, func(a, b librarySymbol) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(len(a.Simple), len(b.Simple)),
			cmp.Compare(a.Simple, b.Simple), cmp.Compare(a.Package, b.Package))
	})
	return out[:min(len(out), limit)]
}

// resolveLibrarySymbol locates a library symbol.
func (s *Server) resolveLibrarySymbol(l librarySymbol) (protocol.Location, bool) {
	kinds := []string{"function", "property"}
	if l.Class {
		kinds = []string{"class"}
	}
	for _, k := range kinds {
		if loc, _, ok := s.libraryLoc(l.Path, libsrc.Target{Kind: k, Name: l.name()}); ok {
			if locs := s.locator().locations([]ide.Loc{loc}); len(locs) == 1 {
				return locs[0], true
			}
		}
	}
	return protocol.Location{}, false
}

func (s *Server) resolveWorkspaceSymbol(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.WorkspaceSymbol
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	var l librarySymbol
	if b, err := json.Marshal(p.Data); err != nil || json.Unmarshal(b, &l) != nil || l.Simple == "" || strings.TrimSpace(l.Path) == "" {
		return p, nil // a source symbol: located already
	}
	if loc, ok := s.resolveLibrarySymbol(l); ok {
		p.Location = loc
	}
	return p, nil
}
