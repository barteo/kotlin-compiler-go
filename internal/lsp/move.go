package lsp

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Moving files: before the client renames .kt files or folders, it asks
// workspace/willRenameFiles for the edits that keep the sources
// compiling; a file whose package directive matches its directory's,
// under a module's source root, moves to the package of its new
// directory (ide.MoveEdits). A new empty .kt file under a source root
// gets its directory's package directive, through workspace/applyEdit.

func (s *Server) willRenameFiles(ctx context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.RenameFilesParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	s.awaitUpdates(ctx)
	views := s.allViews()
	snaps := map[string]*psi.Snapshot{}
	for _, v := range views {
		for _, path := range v.Paths() {
			if _, ok := snaps[path]; !ok {
				snaps[path] = v.Snapshot(path)
			}
		}
	}
	paths := slices.Sorted(mapKeys(snaps))
	var moves []ide.Move
	for _, f := range p.Files {
		from, to := uriPath(f.OldURI), uriPath(f.NewURI)
		for _, path := range paths {
			var dest string
			switch {
			case path == from:
				dest = to
			case strings.HasPrefix(path, from+string(filepath.Separator)):
				dest = to + path[len(from):]
			default:
				continue
			}
			if !strings.HasSuffix(dest, ".kt") {
				continue
			}
			old, ok := s.dirPackage(path)
			if !ok || ide.PackageOf(snaps[path]) != old {
				continue
			}
			if pkg, ok := s.dirPackage(dest); ok {
				moves = append(moves, ide.Move{Path: path, Package: pkg})
			}
		}
	}
	if len(moves) == 0 {
		return nil, nil
	}
	we, ok := s.locator().fixEdit(ide.MoveEdits(views, moves))
	if !ok {
		return nil, errStale
	}
	return we, nil
}

func (s *Server) didCreateFiles(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.CreateFilesParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	s.mu.Lock()
	can := s.canApplyEdit
	s.mu.Unlock()
	if !can {
		return nil, nil
	}
	we := protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{}}
	for _, f := range p.Files {
		path := uriPath(f.URI)
		if !strings.HasSuffix(path, ".kt") {
			continue
		}
		if b, err := os.ReadFile(path); err != nil || strings.TrimSpace(string(b)) != "" {
			continue
		}
		if pkg, ok := s.dirPackage(path); ok && pkg != "" {
			we.Changes[f.URI] = []protocol.TextEdit{{NewText: ide.PackageText(pkg)}}
		}
	}
	if len(we.Changes) == 0 {
		return nil, nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()
		var res protocol.ApplyWorkspaceEditResult
		if err := s.conn.Call(ctx, "workspace/applyEdit", protocol.ApplyWorkspaceEditParams{Label: "Add the package directive", Edit: we}, &res); err != nil {
			s.logMessage(protocol.MessageWarning, "kc-lsp: can't add the package directive: "+err.Error())
		}
	}()
	return nil, nil
}

// dirPackage returns the package of the directory of path, by its
// directory under the source root of its module: false if no module's
// root holds it, or a directory's name isn't a name.
func (s *Server) dirPackage(path string) (string, bool) {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	for _, w := range ws {
		w.mu.Lock()
		var root string
		if w.proj != nil {
			if m := w.proj.ModuleOf(path); m != nil {
				for _, r := range m.SourceRoots {
					if rel, err := filepath.Rel(r, path); err == nil && !strings.HasPrefix(rel, "..") && len(r) > len(root) {
						root = r
					}
				}
			}
		}
		w.mu.Unlock()
		if root == "" {
			continue
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return "", false
		}
		if rel == "." {
			return "", true
		}
		parts := strings.Split(rel, string(filepath.Separator))
		for _, p := range parts {
			if _, ok := psi.QuoteName(p); !ok || strings.Contains(p, ".") {
				return "", false
			}
		}
		return strings.Join(parts, "."), true
	}
	return "", false
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
