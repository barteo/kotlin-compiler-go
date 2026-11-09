package lsp

import (
	"context"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Formatting (docs/lsp.md, milestone L5) formats the document's text as
// it is now with ide.Format: whitespace only, and nothing for a document
// with syntax errors.

func formatOptions(o protocol.FormattingOptions) ide.FormatOptions {
	return ide.FormatOptions{TabSize: o.TabSize, UseTabs: !o.InsertSpaces}
}

// textEdits converts edits of d's text; an empty list, not null, for none.
func textEdits(d *ide.Document, edits []ide.TextEdit, enc ide.Encoding) []protocol.TextEdit {
	out := []protocol.TextEdit{}
	for _, e := range edits {
		out = append(out, protocol.TextEdit{Range: rng(d, e.Range, enc), NewText: e.Text})
	}
	return out
}

func (s *Server) formatting(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DocumentFormattingParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	return textEdits(d, ide.Format(d.Snap, formatOptions(p.Options)), enc), nil
}

func (s *Server) rangeFormatting(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DocumentRangeFormattingParams
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
	return textEdits(d, ide.FormatRange(d.Snap, formatOptions(p.Options), psi.Range{Start: from, End: to}), enc), nil
}

func (s *Server) onTypeFormatting(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DocumentOnTypeFormattingParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	off, err := d.Offset(pos(p.Position), enc)
	if err != nil {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%v", err)
	}
	return textEdits(d, ide.OnType(d.Snap, off, p.Ch, formatOptions(p.Options)), enc), nil
}
