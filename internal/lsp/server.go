package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type folder struct {
	URI string `json:"uri"`
}
type parameters struct {
	Settings              object     `json:"settings"`
	InitializationOptions object     `json:"initializationOptions"`
	Files                 []fileMove `json:"files"`
	TextDocument          struct {
		URI, Text string
		Version   int
	} `json:"textDocument"`
	RootURI          string   `json:"rootUri"`
	WorkspaceFolders []folder `json:"workspaceFolders"`
	Position         position `json:"position"`
	Range            span     `json:"range"`
	NewName, Query   string
	Options          struct {
		TabSize      int
		InsertSpaces bool
	}
	Item struct {
		URI            string
		SelectionRange span
	}
	Event   struct{ Added, Removed []folder }
	Context struct {
		IncludeDeclaration bool
		Only               []string
	}
	ContentChanges []change `json:"contentChanges"`
	Capabilities   struct {
		TextDocument struct {
			Definition    struct{ LinkSupport bool }
			Completion    struct{ CompletionItem completionOptions }
			SignatureHelp struct {
				SignatureInformation struct {
					ParameterInformation struct{ LabelOffsetSupport bool }
				}
			}
		}
		Workspace struct {
			InlayHint             struct{ RefreshSupport bool }
			WorkspaceEdit         struct{ DocumentChanges bool }
			DidChangeWatchedFiles struct{ DynamicRegistration bool }
		}
	}
}
type server struct {
	w                                              workspace
	out                                            io.Writer
	started, shutdown, versionEdits, registerWatch bool
	loaded, refreshHints, labelOffsets             bool
	definitionLinks                                bool
	completion                                     completionOptions
	hints                                          hintSettings
	hintRequest                                    uint64
	latest                                         atomic.Uint64
	revision                                       uint64
	writeErr                                       error
}
type job struct {
	m        message
	ctx      context.Context
	cancel   context.CancelFunc
	revision uint64
}

func frame(in *bufio.Reader) ([]byte, error) {
	size := -1
	total := 0
	for {
		b, err := in.ReadSlice('\n')
		if err != nil {
			return nil, err
		}
		total += len(b)
		if total > 16384 {
			return nil, errors.New("LSP headers too large")
		}
		line := strings.TrimSpace(string(b))
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, errors.New("malformed LSP header")
		}
		if strings.EqualFold(k, "Content-Length") {
			if size >= 0 {
				return nil, errors.New("duplicate Content-Length")
			}
			size, err = strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, err
			}
		}
	}
	if size < 0 || size > 64<<20 {
		return nil, errors.New("invalid LSP Content-Length")
	}
	b := make([]byte, size)
	_, err := io.ReadFull(in, b)
	return b, err
}
func (s *server) send(o object) {
	if s.writeErr != nil {
		return
	}
	o["jsonrpc"] = "2.0"
	b, err := json.Marshal(o)
	if err == nil {
		_, err = fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n", len(b))
	}
	if err == nil {
		_, err = s.out.Write(b)
	}
	s.writeErr = err
}
func (s *server) notify(method string, p any) { s.send(object{"method": method, "params": p}) }

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e rpcError) Error() string { return e.Message }
func changesWorld(m string) bool {
	switch m {
	case "textDocument/didChange", "textDocument/didOpen", "textDocument/didClose", "textDocument/didSave", "workspace/didChangeWatchedFiles", "workspace/didChangeWorkspaceFolders":
		return true
	}
	return false
}

// Serve serializes workspace revisions and writes. The reader can cancel a
// running query before it reaches the head of the request queue.
func Serve(in io.Reader, out io.Writer) error {
	s := &server{out: out}
	defer func() {
		s.w.close()
		if s.w.sourceDir != "" {
			os.RemoveAll(s.w.sourceDir)
		}
	}()
	jobs := make(chan job, 128)
	done := make(chan struct{})
	defer close(done)
	var pending sync.Map
	// Cancel the preceding notification's background check as soon as a
	// newer revision arrives. Applying the edits themselves stays ordered.
	var diagnosticCancel context.CancelFunc
	end := make(chan error, 1)
	go func() {
		defer close(jobs)
		r := bufio.NewReader(in)
		var rev uint64
		for {
			b, err := frame(r)
			if err != nil {
				end <- err
				return
			}
			var m message
			if json.Unmarshal(b, &m) != nil {
				m = message{Method: "!parse", ID: json.RawMessage("null")}
			}
			if m.Method == "$/cancelRequest" {
				var p struct{ ID json.RawMessage }
				if json.Unmarshal(m.Params, &p) == nil {
					if c, ok := pending.Load(string(p.ID)); ok {
						c.(context.CancelFunc)()
					}
				}
				continue
			}
			if m.Method == "" {
				continue
			}
			ctx, cancel := context.WithCancel(context.Background())
			if changesWorld(m.Method) {
				rev = s.latest.Add(1)
				if diagnosticCancel != nil {
					diagnosticCancel()
				}
				diagnosticCancel = cancel
			}
			if len(m.ID) > 0 {
				pending.Store(string(m.ID), cancel)
			}
			select {
			case jobs <- job{m, ctx, cancel, rev}:
			case <-done:
				cancel()
				return
			}
			if m.Method == "exit" {
				return
			}
		}
	}()
	for j := range jobs {
		m := j.m
		if m.Method == "exit" {
			j.cancel()
			if !s.shutdown {
				return errors.New("exit before shutdown")
			}
			return s.writeErr
		}
		s.revision = j.revision
		result, err := s.handle(j.ctx, m)
		if j.ctx.Err() != nil && len(m.ID) > 0 {
			err = rpcError{-32800, "Request cancelled"}
		}
		if len(m.ID) > 0 {
			pending.Delete(string(m.ID))
			if err != nil {
				e := rpcError{-32603, err.Error()}
				errors.As(err, &e)
				s.send(object{"id": m.ID, "error": e})
			} else {
				s.send(object{"id": m.ID, "result": result})
			}
		} else if err != nil {
			s.notify("window/logMessage", object{"type": 2, "message": err.Error()})
		}
		j.cancel()
		if s.writeErr != nil {
			return s.writeErr
		}
	}
	err := <-end
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
func capabilities() object {
	return object{
		"positionEncoding": "utf-16", "textDocumentSync": object{"openClose": true, "change": 2, "save": true},
		"workspace":              object{"workspaceFolders": object{"supported": true, "changeNotifications": true}, "fileOperations": object{"willRename": object{"filters": []object{{"scheme": "file", "pattern": object{"glob": "**/*.kt", "matches": "file"}}}}}},
		"documentSymbolProvider": true, "workspaceSymbolProvider": true, "foldingRangeProvider": true, "hoverProvider": true,
		"definitionProvider": true, "typeDefinitionProvider": true, "referencesProvider": true, "documentHighlightProvider": true, "completionProvider": object{"resolveProvider": true, "triggerCharacters": []string{".", "?"}},
		"signatureHelpProvider": object{"triggerCharacters": []string{"(", ","}}, "renameProvider": object{"prepareProvider": true},
		"semanticTokensProvider": object{"legend": object{"tokenTypes": tokenTypes, "tokenModifiers": tokenModifiers}, "full": true, "range": true},
		"diagnosticProvider":     object{"identifier": "kc", "interFileDependencies": true, "workspaceDiagnostics": false},
		"implementationProvider": true, "callHierarchyProvider": true, "typeHierarchyProvider": true, "inlayHintProvider": true,
		"codeActionProvider":         object{"codeActionKinds": []string{"quickfix", "refactor.extract.variable", "refactor.inline.variable", "source.organizeImports"}},
		"documentFormattingProvider": true, "documentRangeFormattingProvider": true,
	}
}
func (s *server) handle(ctx context.Context, m message) (result any, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("request %s failed: %v", m.Method, v)
		}
	}()
	if m.Method == "!parse" {
		return nil, rpcError{-32700, "Invalid JSON"}
	}
	var p parameters
	if len(m.Params) > 0 && !bytes.Equal(m.Params, []byte("null")) {
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, rpcError{-32602, err.Error()}
		}
	}
	if m.Method == "initialize" {
		if s.started {
			return nil, rpcError{-32600, "Already initialized"}
		}
		s.started = true
		s.hints = readHintSettings(p.InitializationOptions)
		s.completion = p.Capabilities.TextDocument.Completion.CompletionItem
		s.definitionLinks = p.Capabilities.TextDocument.Definition.LinkSupport
		s.labelOffsets = p.Capabilities.TextDocument.SignatureHelp.SignatureInformation.ParameterInformation.LabelOffsetSupport
		s.refreshHints = p.Capabilities.Workspace.InlayHint.RefreshSupport
		s.versionEdits = p.Capabilities.Workspace.WorkspaceEdit.DocumentChanges
		s.registerWatch = p.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration
		folders := p.WorkspaceFolders
		if len(folders) == 0 && p.RootURI != "" {
			folders = []folder{{p.RootURI}}
		}
		for _, f := range folders {
			root, err := filePath(f.URI)
			if err != nil {
				return nil, err
			}
			s.w.roots = append(s.w.roots, root)
		}
		return object{"capabilities": capabilities(), "serverInfo": object{"name": "kc-lsp"}}, nil
	}
	if !s.started {
		return nil, rpcError{-32002, "Server not initialized"}
	}
	if s.shutdown {
		return nil, rpcError{-32600, "Server is shutting down"}
	}
	if err := ctx.Err(); err != nil && !changesWorld(m.Method) {
		return nil, err
	}
	// Send the initialize response before walking source roots or fetching artifacts.
	if !s.loaded && m.Method != "shutdown" {
		s.w.reload(context.WithoutCancel(ctx))
		s.loaded = true
	}
	switch m.Method {
	case "shutdown":
		s.shutdown = true
		return nil, nil
	case "initialized":
		s.reportIssues()
		if s.registerWatch {
			s.send(object{"id": "kc-watch", "method": "client/registerCapability", "params": object{"registrations": []object{{"id": "kc-files", "method": "workspace/didChangeWatchedFiles", "registerOptions": object{"watchers": []object{{"globPattern": "**/*.{kt,java,kts,gradle,toml,jar,class}"}, {"globPattern": "**/{pom.xml,kc-lsp.json}"}}}}}}})
		}
		return nil, nil
	case "workspace/didChangeConfiguration":
		settings := readHintSettings(p.Settings)
		if settings != s.hints && s.refreshHints {
			s.send(object{"id": "kc-hints-" + strconv.FormatUint(s.hintRequest, 10), "method": "workspace/inlayHint/refresh"})
			s.hintRequest++
		}
		s.hints = settings
		return nil, nil
	case "$/setTrace":
		return nil, nil
	case "workspace/didChangeWorkspaceFolders":
		for _, f := range p.Event.Removed {
			path, _ := filePath(f.URI)
			s.w.roots = slices.DeleteFunc(s.w.roots, func(r string) bool { return r == path })
		}
		for _, f := range p.Event.Added {
			path, err := filePath(f.URI)
			if err != nil {
				return nil, err
			}
			s.w.roots = append(s.w.roots, path)
		}
		fallthrough
	case "textDocument/didSave":
		if path, err := filePath(p.TextDocument.URI); err == nil && (strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".java")) {
			if d := s.w.docs[path]; d.version != nil {
				s.publish(ctx)
				return nil, nil
			}
		}
		fallthrough
	case "workspace/didChangeWatchedFiles":
		s.w.reload(context.WithoutCancel(ctx))
		s.reportIssues()
		s.publish(ctx)
		return nil, nil
	case "workspace/willRenameFiles":
		return s.fileMoves(ctx, p.Files)
	case "completionItem/resolve":
		var item object
		if err := json.Unmarshal(m.Params, &item); err != nil || item == nil {
			return nil, rpcError{-32602, "Invalid completion item"}
		}
		var payload struct{ Data completionData }
		if err := json.Unmarshal(m.Params, &payload); err != nil {
			return nil, rpcError{-32602, "Invalid completion data"}
		}
		data := payload.Data
		path, _ := filePath(data.URI)
		if d := s.w.docs[path]; d.snap != nil && d.snap.ID() == data.Snapshot {
			n := d.snap.Node(data.Node)
			if n.Declaration().Valid() {
				if _, hasDetail := item["detail"]; !hasDetail {
					item["detail"] = summary(n)
				}
				if doc := kdoc(n); doc != "" {
					format := "plaintext"
					if len(s.completion.DocumentationFormat) > 0 && s.completion.DocumentationFormat[0] == "markdown" {
						format = "markdown"
					}
					item["documentation"] = object{"kind": format, "value": doc}
				}
			}
		}
		return item, nil
	case "workspace/symbol":
		out := []object{}
		for _, path := range slices.Sorted(maps.Keys(s.w.docs)) {
			d := s.w.docs[path]
			psi.WalkNodes(d.snap.RootNode(), func(n psi.NodeRef) bool {
				if n.Declaration().Valid() && n.NameIdentifier().Valid() && strings.Contains(strings.ToLower(n.Name()), strings.ToLower(p.Query)) {
					out = append(out, object{"name": n.Name(), "kind": symbolKind(n), "location": loc(n.NameIdentifier())})
				}
				return n.NumericKind() != psi.KindBLOCK && n.NumericKind() != psi.KindVALUE_PARAMETER_LIST
			})
		}
		libs, err := s.w.librarySymbols(ctx, p.Query)
		return append(out, libs...), err
	}
	u := p.TextDocument.URI
	if u == "" {
		u = p.Item.URI
	}
	if u == "" {
		return nil, rpcError{-32601, "Unknown method " + m.Method}
	}
	path, err := filePath(u)
	if err != nil {
		return nil, rpcError{-32602, err.Error()}
	}
	switch m.Method {
	case "textDocument/didOpen":
		s.w.set(path, p.TextDocument.Text, p.TextDocument.Version)
		s.publish(ctx)
		return nil, nil
	case "textDocument/didChange":
		if err := s.w.change(path, p.TextDocument.Version, p.ContentChanges); err != nil {
			return nil, err
		}
		s.publish(ctx)
		return nil, nil
	case "textDocument/didClose":
		delete(s.w.java, path)
		delete(s.w.docs, path)
		s.w.invalidate(path)
		if s.w.p.owner(path) != nil {
			if b, err := os.ReadFile(path); err == nil {
				if strings.HasSuffix(path, ".java") {
					s.w.java[path] = string(b)
				} else {
					s.w.docs[path] = document{snap: psi.ParseDocument(path, string(b))}
				}
			}
		}
		s.notify("textDocument/publishDiagnostics", object{"uri": u, "diagnostics": []object{}})
		s.publish(ctx)
		return nil, nil
	}
	d, ok := s.w.docs[path]
	if !ok {
		return nil, rpcError{-32602, "Unknown document"}
	}
	switch m.Method {
	case "textDocument/documentSymbol":
		return outline(d.snap), nil
	case "textDocument/foldingRange":
		return folding(d.snap), nil
	case "textDocument/formatting", "textDocument/rangeFormatting":
		return format(d.snap, p, m.Method == "textDocument/rangeFormatting"), nil
	}
	i, err := s.w.check(ctx, path)
	if err != nil {
		return nil, err
	}
	if p.Item.URI != "" {
		p.Position = p.Item.SelectionRange.Start
	}
	off, err := offset(d.snap, p.Position)
	if err != nil {
		return nil, rpcError{-32602, err.Error()}
	}
	n := d.snap.NodeAt(off)
	if n.NumericKind() != psi.KindIDENTIFIER && off > 0 {
		prev := d.snap.NodeAt(off - 1)
		if prev.NumericKind() == psi.KindIDENTIFIER {
			n = prev
		}
	}
	target := i.targets[n]
	switch m.Method {
	case "textDocument/diagnostic":
		return object{"kind": "full", "items": diagnostics(d.snap, i)}, nil
	case "textDocument/hover":
		return i.hover(n), nil
	case "textDocument/typeDefinition":
		sym := i.symbols[n]
		name := sym.Type.Name
		if f := i.facts[n.Parent()]; f.Type != nil {
			name = f.Type.Name
		}
		for _, d := range i.decls {
			ds := i.symbols[d.NameIdentifier()]
			if ds.Kind == "class" && ds.QualifiedName == name {
				return []location{loc(d.NameIdentifier())}, nil
			}
		}
		if name != "" {
			return resolveLibrary(ctx, &s.w, path, i, analysis.Symbol{ID: 1, Kind: "class", Name: strings.ReplaceAll(name, ".", "/"), QualifiedName: name}, nil), nil
		}
		return nil, nil
	case "textDocument/definition":
		var dest *location
		if target.Valid() {
			l := loc(target)
			if !s.definitionLinks {
				return []location{l}, nil
			}
			dest = &l
		} else {
			dest = libraryDefinition(ctx, &s.w, path, i, n)
		}
		if dest == nil || !s.definitionLinks {
			return dest, nil
		}
		return []object{{"originSelectionRange": rng(n), "targetUri": dest.URI,
			"targetRange": dest.Range, "targetSelectionRange": dest.Range}}, nil
	case "textDocument/completion":
		return i.completion(ctx, d.snap, off, s.completion), nil
	case "textDocument/signatureHelp":
		return i.signature(ctx, d.snap.NodeAt(off), off, s.labelOffsets), nil
	case "textDocument/prepareRename":
		if target.Valid() {
			return rng(n), nil
		}
		return nil, nil
	case "textDocument/references", "textDocument/rename", "textDocument/documentHighlight":
		return s.references(ctx, m.Method, path, n, target, p)
	case "textDocument/semanticTokens/full", "textDocument/semanticTokens/range":
		return tokens(d.snap, i, p.Range, strings.HasSuffix(m.Method, "/range")), nil
	case "textDocument/inlayHint":
		return hints(d.snap, i, p.Range, s.hints), nil
	case "textDocument/codeAction":
		out := actions(ctx, d.snap, i, p)
		for _, a := range out {
			we := a["edit"].(object)
			a["edit"] = s.workspaceEdits(we["changes"].(map[string][]edit))
		}
		return out, nil
	case "textDocument/implementation", "textDocument/prepareCallHierarchy", "textDocument/prepareTypeHierarchy", "typeHierarchy/supertypes", "typeHierarchy/subtypes", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
		return s.hierarchy(ctx, m.Method, i, target)
	}
	return nil, rpcError{-32601, "Unknown method " + m.Method}
}
func (s *server) reportIssues() {
	for _, issue := range s.w.issues {
		s.notify("window/showMessage", object{"type": 2, "message": issue})
	}
}
func (s *server) publish(ctx context.Context) {
	if s.latest.Load() != s.revision {
		return
	}
	for _, p := range slices.Sorted(maps.Keys(s.w.docs)) {
		d := s.w.docs[p]
		if d.version == nil || strings.HasSuffix(p, ".java") {
			continue
		}
		i, err := s.w.check(ctx, p)
		if s.latest.Load() != s.revision {
			return
		}
		if err != nil {
			s.notify("window/logMessage", object{"type": 2, "message": err.Error()})
		}
		s.notify("textDocument/publishDiagnostics", object{"uri": uri(p), "version": *d.version, "diagnostics": diagnostics(d.snap, i)})
	}
}
func diagnostics(s *psi.Snapshot, i *index) []object {
	out := []object{}
	for _, d := range s.Diagnostics() {
		out = append(out, object{"range": span{pos(s, d.Range.Start), pos(s, d.Range.End)}, "severity": 1, "source": "kc", "code": d.Code, "message": d.Message})
	}
	if i == nil {
		return out
	}
	for _, d := range i.model.Facts.Diagnostics {
		if d.Path != s.Path() {
			continue
		}
		start, err := s.ByteOffset(max(1, d.Line), 1)
		if err != nil {
			continue
		}
		off := min(len(s.Source()), start+max(0, d.Column-1))
		n := s.NodeAt(off)
		r := diagnosticRange(n, d)
		if slices.ContainsFunc(out, func(o object) bool { return o["range"].(span).Start == r.Start && o["message"] == d.Message }) {
			continue
		}
		severity := 1
		if d.Warning {
			severity = 2
		}
		out = append(out, object{"range": r, "severity": severity, "source": "kc", "code": d.Code, "message": d.Message})
	}
	for ref, target := range i.targets {
		if ref.ContainingFile() == s && ref != target && !i.moduleVisible(target.Parent(), ref) {
			out = append(out, object{"range": rng(ref), "severity": 1, "code": "INVISIBLE_REFERENCE", "source": "kc", "message": "Declaration " + target.Name() + " is not visible in this module"})
		}
	}
	slices.SortStableFunc(out, func(a, b object) int {
		ra, rb := a["range"].(span), b["range"].(span)
		if before(ra.Start, rb.Start) {
			return -1
		}
		if before(rb.Start, ra.Start) {
			return 1
		}
		return strings.Compare(a["message"].(string), b["message"].(string))
	})
	return out
}
