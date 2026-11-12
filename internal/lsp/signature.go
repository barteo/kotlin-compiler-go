package lsp

import (
	"context"
	"unicode/utf16"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// signatureHelp lists the candidates of the call whose arguments the
// caret is in, from a probe check as completion's (ide.SignatureAt).
func (s *Server) signatureHelp(ctx context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.TextDocumentPositionParams
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
	sc, ok := ide.SignatureAt(d, off)
	m := s.ownerModule(d.Path)
	if !ok || m == nil {
		return nil, nil
	}
	snaps, err := m.snapshots(psi.Parse(sc.Path, sc.ProbeText))
	if err != nil {
		return nil, err
	}
	c, err := m.sess.Complete(ctx, snaps, sc.Path, "")
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		s.logMessage(protocol.MessageWarning, "signature help: "+err.Error())
		return nil, nil
	}
	sigs, active := ide.SignatureHelp(sc, c)
	if len(sigs) == 0 {
		return nil, nil
	}
	open := s.openSnapshots()
	help := &protocol.SignatureHelp{ActiveSignature: active, Signatures: []protocol.SignatureInformation{}}
	for _, sig := range sigs {
		si := protocol.SignatureInformation{Label: sig.Label, Parameters: []protocol.ParameterInformation{}}
		for _, pr := range sig.Parameters {
			si.Parameters = append(si.Parameters, protocol.ParameterInformation{Label: [2]int{utf16Len(sig.Label[:pr[0]]), utf16Len(sig.Label[:pr[1]])}})
		}
		a := sig.Active
		si.ActiveParameter = &a
		if sig.Decl != nil {
			snap := open[sig.Decl.Path]
			if snap == nil {
				snap, _ = s.disk.snapshot(sig.Decl.Path)
			}
			if doc := ide.SignatureDoc(snap, sig.Decl.Off); doc != "" {
				si.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc}
			}
		}
		help.Signatures = append(help.Signatures, si)
	}
	help.ActiveParameter = max(sigs[active].Active, 0)
	return help, nil
}

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }
