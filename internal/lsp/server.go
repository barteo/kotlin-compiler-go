// Package lsp is kc-lsp's language server: it speaks the Language Server
// Protocol over a jsonrpc.Conn and answers with package ide's features
// (docs/lsp.md).
package lsp

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Version is reported in the initialize response.
const Version = "0.1.0"

// Server is the state of one client connection.
type Server struct {
	conn   *jsonrpc.Conn
	ctx    context.Context // the server's; done at exit
	cancel context.CancelFunc
	work   *activity
	disk   diskCache

	// pubMu orders the diagnostics published.
	pubMu     sync.Mutex
	published atomic.Int64 // diagnostics notifications sent, for tests

	mu           sync.Mutex
	docs         map[protocol.DocumentURI]*ide.Document
	paths        map[string]protocol.DocumentURI // the open documents' URIs by path
	sem          map[protocol.DocumentURI]semantic
	closedShown  map[string]bool // files not open whose diagnostics are shown
	enc          ide.Encoding
	hierarchical bool
	snippets     bool // the client takes snippets in completion items
	initialized  bool
	shutdown     bool
	exitCode     int

	roots        []string // the workspace folders
	ws           []*workspace
	mode         ide.DiagnosticsMode
	noGradle     bool
	initWarnings []string
	canWatch     bool // the client registers file watchers
	watching     bool // and does for kc-lsp
}

// Serve runs a server on r and w until the client sends exit or closes the
// connection, and returns the exit code LSP prescribes: 0 after shutdown,
// 1 otherwise.
func Serve(ctx context.Context, r io.Reader, w io.Writer) (int, error) {
	return newServer(ctx, r, w).run()
}

func newServer(ctx context.Context, r io.Reader, w io.Writer) *Server {
	ctx, cancel := context.WithCancel(ctx)
	return &Server{
		conn: jsonrpc.NewConn(r, w), ctx: ctx, cancel: cancel, work: newActivity(),
		docs: map[protocol.DocumentURI]*ide.Document{}, paths: map[string]protocol.DocumentURI{},
		sem: map[protocol.DocumentURI]semantic{}, closedShown: map[string]bool{}, exitCode: 1,
	}
}

func (s *Server) run() (int, error) {
	defer s.cancel()
	err := s.conn.Serve(s.ctx, s.handle)
	s.cancel()
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	for _, w := range ws {
		w.load.Lock()
		w.stopModules(false)
		w.load.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitCode, err
}

// waitIdle waits until no workspace is loading and no module updating,
// and returns the number of diagnostics notifications sent; for tests.
func (s *Server) waitIdle() int64 {
	s.work.wait()
	return s.published.Load()
}

func (s *Server) handle(ctx context.Context, r *jsonrpc.Request) (any, error) {
	s.mu.Lock()
	initialized, shutdown := s.initialized, s.shutdown
	s.mu.Unlock()
	switch r.Method {
	case "initialize":
		var p protocol.InitializeParams
		if err := r.Unmarshal(&p); err != nil {
			return nil, err
		}
		return s.initialize(&p), nil
	case "exit":
		s.mu.Lock()
		if s.shutdown {
			s.exitCode = 0
		}
		s.mu.Unlock()
		s.cancel()
		return nil, nil
	}
	if !initialized {
		if r.IsNotification() {
			return nil, nil
		}
		return nil, jsonrpc.Errorf(jsonrpc.CodeServerNotInitialized, "%s before initialize", r.Method)
	}
	if shutdown && !r.IsNotification() {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidRequest, "%s after shutdown", r.Method)
	}
	h := handlers[r.Method]
	if h == nil {
		if r.IsNotification() {
			return nil, nil // includes $/ notifications, which may be ignored
		}
		return nil, jsonrpc.Errorf(jsonrpc.CodeMethodNotFound, "%s is not supported", r.Method)
	}
	return h(s, ctx, r)
}

type handler func(s *Server, ctx context.Context, r *jsonrpc.Request) (any, error)

var handlers map[string]handler

func init() {
	handlers = map[string]handler{
		"initialized":                       (*Server).initializedNotification,
		"shutdown":                          (*Server).shutdownRequest,
		"textDocument/didOpen":              (*Server).didOpen,
		"textDocument/didChange":            (*Server).didChange,
		"textDocument/didClose":             (*Server).didClose,
		"textDocument/didSave":              (*Server).didSave,
		"workspace/didChangeWatchedFiles":   (*Server).didChangeWatchedFiles,
		"textDocument/documentSymbol":       (*Server).documentSymbol,
		"textDocument/foldingRange":         (*Server).foldingRange,
		"textDocument/selectionRange":       (*Server).selectionRange,
		"textDocument/semanticTokens/full":  (*Server).semanticTokens,
		"textDocument/definition":           (*Server).definition,
		"textDocument/typeDefinition":       (*Server).typeDefinition,
		"textDocument/references":           (*Server).references,
		"textDocument/documentHighlight":    (*Server).documentHighlight,
		"textDocument/hover":                (*Server).hover,
		"textDocument/implementation":       (*Server).implementation,
		"textDocument/rename":               (*Server).rename,
		"textDocument/prepareRename":        (*Server).prepareRename,
		"textDocument/prepareCallHierarchy": (*Server).prepareCallHierarchy,
		"callHierarchy/incomingCalls":       (*Server).incomingCalls,
		"callHierarchy/outgoingCalls":       (*Server).outgoingCalls,
		"workspace/symbol":                  (*Server).workspaceSymbol,
		"textDocument/inlayHint":            (*Server).inlayHint,
		"textDocument/completion":           (*Server).completion,
		"completionItem/resolve":            (*Server).resolveCompletion,
		"textDocument/signatureHelp":        (*Server).signatureHelp,
	}
}

func (s *Server) initialize(p *protocol.InitializeParams) *protocol.InitializeResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialized = true
	enc := protocol.EncodingUTF16
	if g := p.Capabilities.General; g != nil {
		for _, e := range g.PositionEncodings {
			if e == protocol.EncodingUTF8 {
				enc, s.enc = protocol.EncodingUTF8, ide.UTF8
			}
		}
	}
	if td := p.Capabilities.TextDocument; td != nil && td.DocumentSymbol != nil {
		s.hierarchical = td.DocumentSymbol.HierarchicalDocumentSymbolSupport
	}
	if td := p.Capabilities.TextDocument; td != nil && td.Completion != nil && td.Completion.CompletionItem != nil {
		s.snippets = td.Completion.CompletionItem.SnippetSupport
	}
	if ws := p.Capabilities.Workspace; ws != nil && ws.DidChangeWatchedFiles != nil {
		s.canWatch = ws.DidChangeWatchedFiles.DynamicRegistration
	}
	s.roots = workspaceRoots(p)
	s.parseInitOptions(p.InitializationOptions)
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			PositionEncoding:       enc,
			TextDocumentSync:       &protocol.TextDocumentSyncOptions{OpenClose: true, Change: protocol.SyncIncremental, Save: true},
			DocumentSymbolProvider: true,
			FoldingRangeProvider:   true,
			SelectionRangeProvider: true,
			SemanticTokensProvider: &protocol.SemanticTokensOptions{
				Legend: protocol.SemanticTokensLegend{TokenTypes: ide.TokenTypeNames, TokenModifiers: ide.TokenModifierNames},
				Full:   true,
			},
			DefinitionProvider:        true,
			TypeDefinitionProvider:    true,
			ReferencesProvider:        true,
			DocumentHighlightProvider: true,
			HoverProvider:             true,
			ImplementationProvider:    true,
			RenameProvider:            &protocol.RenameOptions{PrepareProvider: true},
			CallHierarchyProvider:     true,
			WorkspaceSymbolProvider:   true,
			CompletionProvider:        &protocol.CompletionOptions{TriggerCharacters: []string{".", ":"}, ResolveProvider: true},
			SignatureHelpProvider:     &protocol.SignatureHelpOptions{TriggerCharacters: []string{"(", ","}, RetriggerCharacters: []string{"="}},
			InlayHintProvider:         true,
		},
		ServerInfo: &protocol.ServerInfo{Name: "kc-lsp", Version: Version},
	}
}

func (s *Server) initializedNotification(context.Context, *jsonrpc.Request) (any, error) {
	s.mu.Lock()
	roots, warnings, canWatch, loaded := s.roots, s.initWarnings, s.canWatch, s.ws != nil
	s.mu.Unlock()
	if loaded {
		return nil, nil // a second initialized
	}
	for _, w := range warnings {
		s.logMessage(protocol.MessageWarning, w)
	}
	if canWatch && len(roots) > 0 {
		go s.watch()
	}
	s.loadWorkspaces(roots)
	return nil, nil
}

func (s *Server) shutdownRequest(context.Context, *jsonrpc.Request) (any, error) {
	s.mu.Lock()
	s.shutdown = true
	s.mu.Unlock()
	return nil, nil
}

// ---- documents ----

// uriPath returns the file path of a file URI, or the URI itself.
func uriPath(u protocol.DocumentURI) string {
	p, err := url.Parse(string(u))
	if err != nil || p.Scheme != "file" {
		return string(u)
	}
	return filepath.FromSlash(p.Path)
}

func (s *Server) doc(u protocol.DocumentURI) (*ide.Document, ide.Encoding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.docs[u]
	if d == nil {
		return nil, 0, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%s is not open", u)
	}
	return d, s.enc, nil
}

func (s *Server) didOpen(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DidOpenTextDocumentParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	td := p.TextDocument
	d := ide.NewDocument(uriPath(td.URI), td.Version, td.Text)
	// The last update's diagnostics of the file, if it is the same text.
	last, ok := s.ownerResult(d.Path)
	s.mu.Lock()
	s.docs[td.URI] = d
	s.paths[d.Path] = td.URI
	delete(s.closedShown, d.Path)
	if ok && last.snap.Source() == td.Text {
		s.sem[td.URI] = semantic{d.Snap, ide.SemanticDiagnostics(last.diags, s.mode)}
	}
	enc := s.enc
	s.mu.Unlock()
	s.kickPath(d.Path, false)
	s.publish(td.URI, d, enc)
	return nil, nil
}

func (s *Server) didChange(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DidChangeTextDocumentParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	u := p.TextDocument.URI
	d, enc, err := s.doc(u)
	if err != nil {
		return nil, err
	}
	edits := make([]ide.Edit, len(p.ContentChanges))
	for i, c := range p.ContentChanges {
		if c.Range == nil {
			edits[i] = ide.Edit{Whole: true, Text: c.Text}
		} else {
			edits[i] = ide.Edit{From: pos(c.Range.Start), To: pos(c.Range.End), Text: c.Text}
		}
	}
	nd, err := d.Apply(p.TextDocument.Version, edits, enc)
	if err != nil {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%s: %v", u, err)
	}
	s.mu.Lock()
	s.docs[u] = nd
	if sem, ok := s.sem[u]; ok && sem.snap == d.Snap {
		s.sem[u] = semantic{nd.Snap, shiftDiagnostics(sem.diags, d.Text(), nd.Text())}
	}
	s.mu.Unlock()
	s.kickPath(nd.Path, false)
	s.publish(u, nd, enc)
	return nil, nil
}

func (s *Server) didClose(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DidCloseTextDocumentParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	u := p.TextDocument.URI
	s.mu.Lock()
	d := s.docs[u]
	delete(s.docs, u)
	delete(s.sem, u)
	if d != nil {
		delete(s.paths, d.Path)
	}
	s.mu.Unlock()
	s.notifyDiagnostics(protocol.PublishDiagnosticsParams{URI: u, Diagnostics: []protocol.Diagnostic{}}, nil)
	if d != nil {
		// The file on disk is the module's again: its diagnostics are
		// shown as a closed file's.
		s.kickPath(d.Path, false)
	}
	return nil, nil
}

// publish sends an open document's diagnostics: its syntax errors and the
// semantic diagnostics of its snapshot, unless a later version is open.
func (s *Server) publish(u protocol.DocumentURI, d *ide.Document, enc ide.Encoding) {
	diags := []protocol.Diagnostic{}
	for _, e := range ide.SyntaxDiagnostics(d) {
		diags = append(diags, protocolDiagnostic(d, e, enc))
	}
	s.mu.Lock()
	sem := s.sem[u]
	s.mu.Unlock()
	if sem.snap == d.Snap {
		for _, e := range sem.diags {
			diags = append(diags, protocolDiagnostic(d, e, enc))
		}
	}
	v := d.Version
	s.notifyDiagnostics(protocol.PublishDiagnosticsParams{URI: u, Version: &v, Diagnostics: diags}, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.docs[u] == d
	})
}

// notifyDiagnostics sends diagnostics if current reports they are still
// current, or it is nil.
func (s *Server) notifyDiagnostics(p protocol.PublishDiagnosticsParams, current func() bool) {
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	if current != nil && !current() {
		return
	}
	if s.conn.Notify("textDocument/publishDiagnostics", p) == nil {
		s.published.Add(1)
	}
}

func protocolDiagnostic(d *ide.Document, e ide.Diagnostic, enc ide.Encoding) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range:    rng(d, e.Range, enc),
		Severity: int(e.Severity),
		Code:     e.Code,
		Source:   "kc",
		Message:  e.Message,
	}
}

// ---- positions ----

func pos(p protocol.Position) ide.Pos { return ide.Pos{Line: p.Line, Char: p.Character} }

func ppos(p ide.Pos) protocol.Position { return protocol.Position{Line: p.Line, Character: p.Char} }

func rng(d *ide.Document, r psi.Range, enc ide.Encoding) protocol.Range {
	return protocol.Range{Start: ppos(d.Pos(r.Start, enc)), End: ppos(d.Pos(r.End, enc))}
}

// ---- syntax features ----

func (s *Server) documentSymbol(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DocumentSymbolParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	syms := ide.Symbols(d)
	s.mu.Lock()
	hierarchical := s.hierarchical
	s.mu.Unlock()
	if hierarchical {
		return documentSymbols(d, syms, enc), nil
	}
	out := []protocol.SymbolInformation{}
	var flat func([]ide.Symbol, string)
	flat = func(syms []ide.Symbol, container string) {
		for _, sym := range syms {
			out = append(out, protocol.SymbolInformation{
				Name:          sym.Name,
				Kind:          symbolKind(sym.Kind),
				Location:      protocol.Location{URI: p.TextDocument.URI, Range: rng(d, sym.Range, enc)},
				ContainerName: container,
			})
			flat(sym.Children, sym.Name)
		}
	}
	flat(syms, "")
	return out, nil
}

func documentSymbols(d *ide.Document, syms []ide.Symbol, enc ide.Encoding) []protocol.DocumentSymbol {
	out := make([]protocol.DocumentSymbol, 0, len(syms))
	for _, sym := range syms {
		out = append(out, protocol.DocumentSymbol{
			Name:           sym.Name,
			Detail:         sym.Detail,
			Kind:           symbolKind(sym.Kind),
			Range:          rng(d, sym.Range, enc),
			SelectionRange: rng(d, sym.NameRange, enc),
			Children:       documentSymbols(d, sym.Children, enc),
		})
	}
	return out
}

func symbolKind(k ide.SymbolKind) int {
	switch k {
	case ide.ClassSymbol, ide.TypeAliasSymbol:
		return protocol.SymbolClass
	case ide.InterfaceSymbol:
		return protocol.SymbolInterface
	case ide.EnumSymbol:
		return protocol.SymbolEnum
	case ide.ObjectSymbol:
		return protocol.SymbolObject
	case ide.FunctionSymbol:
		return protocol.SymbolFunction
	case ide.MethodSymbol:
		return protocol.SymbolMethod
	case ide.ConstructorSymbol:
		return protocol.SymbolConstructor
	case ide.PropertySymbol:
		return protocol.SymbolProperty
	case ide.ConstantSymbol:
		return protocol.SymbolConstant
	case ide.EnumEntrySymbol:
		return protocol.SymbolEnumMember
	}
	return protocol.SymbolVariable
}

func (s *Server) foldingRange(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.FoldingRangeParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, _, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	out := []protocol.FoldingRange{}
	for _, f := range ide.Folds(d) {
		fr := protocol.FoldingRange{StartLine: f.StartLine, EndLine: f.EndLine}
		switch f.Kind {
		case ide.FoldComment:
			fr.Kind = protocol.FoldingComment
		case ide.FoldImports:
			fr.Kind = protocol.FoldingImports
		}
		out = append(out, fr)
	}
	return out, nil
}

func (s *Server) selectionRange(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.SelectionRangeParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	out := make([]*protocol.SelectionRange, 0, len(p.Positions))
	for _, ps := range p.Positions {
		off, err := d.Offset(pos(ps), enc)
		if err != nil {
			return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%v", err)
		}
		var sel *protocol.SelectionRange
		ranges := ide.SelectionRanges(d, off)
		for i := len(ranges) - 1; i >= 0; i-- {
			sel = &protocol.SelectionRange{Range: rng(d, ranges[i], enc), Parent: sel}
		}
		if sel == nil {
			sel = &protocol.SelectionRange{Range: protocol.Range{Start: ps, End: ps}}
		}
		out = append(out, sel)
	}
	return out, nil
}

func (s *Server) semanticTokens(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.SemanticTokensParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	return &protocol.SemanticTokens{Data: encodeTokens(d, s.tokens(d), enc)}, nil
}

// tokens returns a document's lexical tokens and, from the last view of
// the module that owns it, its references' tokens, mapped to its text.
func (s *Server) tokens(d *ide.Document) []ide.Token {
	toks := ide.LexicalTokens(d)
	views := s.views(d.Path)
	if len(views) == 0 {
		return toks
	}
	v := views[0]
	snap := v.Snapshot(d.Path)
	var m *ide.Mapper
	if snap != d.Snap {
		mp := ide.NewMapper(snap.Source(), d.Text())
		m = &mp
	}
	lexical := make(map[int]bool, len(toks))
	for _, t := range toks {
		lexical[t.Range.Start] = true
	}
	for _, t := range ide.ReferenceTokens(v, d.Path) {
		if m != nil {
			r, ok := m.Range(t.Range)
			if !ok {
				continue
			}
			t.Range = r
		}
		if !lexical[t.Range.Start] {
			toks = append(toks, t)
		}
	}
	return toks
}

// encodeTokens writes tokens in LSP's relative form, splitting a token that
// spans lines (a block comment, a raw string) into one per line.
func encodeTokens(d *ide.Document, toks []ide.Token, enc ide.Encoding) []uint32 {
	sort.SliceStable(toks, func(i, j int) bool { return toks[i].Range.Start < toks[j].Range.Start })
	data := []uint32{}
	prevLine, prevChar := 0, 0
	for _, t := range toks {
		start := t.Range.Start
		for start < t.Range.End {
			line := d.Line(start)
			end := min(t.Range.End, d.LineEnd(line))
			if end > start {
				a, b := d.Pos(start, enc), d.Pos(end, enc)
				dl, dc := a.Line-prevLine, a.Char
				if dl == 0 {
					dc = a.Char - prevChar
				}
				data = append(data, uint32(dl), uint32(dc), uint32(b.Char-a.Char), uint32(t.Type), uint32(t.Mods))
				prevLine, prevChar = a.Line, a.Char
			}
			next, err := d.Offset(ide.Pos{Line: line + 1}, ide.UTF8)
			if err != nil || next <= start {
				break
			}
			start = next
		}
	}
	return data
}
