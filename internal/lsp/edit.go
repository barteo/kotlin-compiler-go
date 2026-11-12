package lsp

import (
	"context"
	"errors"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
)

// The editing features return edits of the documents. They are computed
// on the modules' last views like navigation, but an edit is all or
// nothing: one that falls in text typed since the view fails the whole
// request rather than leave a partial change.

// settle is how long an edit waits for the modules' pending updates.
const settle = 2 * time.Second

// awaitUpdates waits, up to settle, for the pending module updates, so
// an edit sees the text just typed.
func (s *Server) awaitUpdates(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.work.wait()
		close(done)
	}()
	t := time.NewTimer(settle)
	defer t.Stop()
	select {
	case <-done:
	case <-t.C:
	case <-ctx.Done():
	}
}

var errStale = jsonrpc.Errorf(jsonrpc.CodeRequestFailed, "the analysis is out of date; try again")

// workspaceEdit converts edits of views' texts to the documents'.
func (l *locator) workspaceEdit(locs []ide.Loc, text string) (*protocol.WorkspaceEdit, error) {
	we := &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{}}
	for _, loc := range locs {
		mp, r, ok := l.rangeOf(loc)
		if !ok {
			return nil, errStale
		}
		we.Changes[mp.uri] = append(we.Changes[mp.uri], protocol.TextEdit{Range: rng(mp.doc, r, l.enc), NewText: text})
	}
	return we, nil
}

func (s *Server) rename(ctx context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.RenameParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	s.awaitUpdates(ctx)
	c, ok, err := s.caretAt(p.TextDocumentPositionParams)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errStale
	}
	c = s.widen(c)
	locs, text, err := ide.Rename(c.views, c.path, c.off, p.NewName)
	if err != nil {
		return nil, renameError(err)
	}
	return s.locator().workspaceEdit(locs, text)
}

func (s *Server) prepareRename(ctx context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.TextDocumentPositionParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	c, ok, err := s.caretAt(p)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errStale
	}
	rg, name, err := ide.PrepareRename(c.views, c.path, c.off)
	if err != nil {
		return nil, renameError(err)
	}
	l := s.locator()
	mp, rg, ok := l.rangeOf(ide.Loc{Snap: c.views[0].Snapshot(c.path), Range: rg})
	if !ok {
		return nil, errStale
	}
	return protocol.PrepareRenameResult{Range: rng(mp.doc, rg, l.enc), Placeholder: name}, nil
}

// renameError reports a refused rename to the user.
func renameError(err error) error {
	var re *ide.RenameError
	if errors.As(err, &re) {
		return jsonrpc.Errorf(jsonrpc.CodeRequestFailed, "%s", re.Msg)
	}
	return err
}
