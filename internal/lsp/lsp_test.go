package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/internal/project"
)

var update = flag.Bool("update", false, "rewrite the want sections of testdata/*.txtar")

// TestScripts runs each testdata/*.txtar against a server in this process,
// over pipes, and compares what the script prints with its want section.
//
// An archive holds source files, in which /*|name*/ marks a position (the
// marker is removed before the file is opened), a "script" section of
// commands, and a "want" section. Commands:
//
// An archive with a kc-lsp.json section is a workspace: its files are
// written to a directory, which is the root URI, and the server analyzes
// them as the modules kc-lsp.json describes (with the standard library:
// the test is skipped without one); a section named _name isn't a file. In
// a command's params, ROOT is the directory's URI.
//
//	initialize [params]       send initialize (default params: hierarchical document symbols)
//	capabilities              print the initialize result
//	open F                    open file F (version 1) and wait for its diagnostics
//	change F FROM TO "text"   replace FROM..TO (@marker or line:char) and wait for diagnostics
//	replace F SECTION         replace F's whole text with another section's
//	close F                   close F
//	wait                      wait until the server's semantic updates are done and their diagnostics received
//	diagnostics F             print F's last published diagnostics
//	symbols F                 print textDocument/documentSymbol
//	folding F                 print textDocument/foldingRange
//	selection F POS...        print textDocument/selectionRange
//	tokens F                  print textDocument/semanticTokens/full, decoded
//	definition F POS          print textDocument/definition's locations
//	typedef F POS             print textDocument/typeDefinition's locations
//	references F POS [decl]   print textDocument/references' locations, with the declaration if decl
//	implementation F POS      print textDocument/implementation's locations
//	rename F POS NAME         print textDocument/rename's edits per file, and apply those of open documents
//	prepare F POS             print textDocument/prepareRename's range and placeholder
//	move FROM TO              print workspace/willRenameFiles' edits per file, and apply those of open documents
//	applied                   wait for the server's workspace/applyEdit requests and print them
//	calls F POS               print the call hierarchy item at POS and its incoming (<-) and outgoing (->) calls
//	highlight F POS           print textDocument/documentHighlight
//	hover F POS               print textDocument/hover's range and Markdown
//	wsymbol [QUERY]           print workspace/symbol
//	inlay F                   print textDocument/inlayHint over the whole document
//	lens F                    print textDocument/codeLens
//	run F [N] [debug]         print the run configuration of F's Nth run lens (kc.run or kc.debug), paths outside ROOT as base names
//	request METHOD [params]   print any request's result or error
//	notify METHOD params      send any notification
//	write F SECTION|-         write a workspace file on disk with a section's text, or delete it
//	scan                      list the modules' sources again, as the server does every few seconds unwatched
//	shutdown, exit            end the session; exit prints the server's exit code
func TestScripts(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.txtar")
	if len(files) == 0 {
		t.Fatal("no testdata/*.txtar")
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".txtar"), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			a := parseArchive(string(data))
			got := runScript(t, a)
			if *update {
				if err := os.WriteFile(file, []byte(a.withWant(got)), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if want := a.section("want"); got != want {
				t.Errorf("output differs (go test ./internal/lsp -update to accept):\n%s", diffLines(want, got))
			}
		})
	}
}

type archive struct {
	sections []section
}

type section struct{ name, body string }

func parseArchive(s string) *archive {
	a := &archive{}
	re := regexp.MustCompile(`(?m)^-- (.+?) --$\n?`)
	locs := re.FindAllStringSubmatchIndex(s, -1)
	for i, l := range locs {
		end := len(s)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		a.sections = append(a.sections, section{s[l[2]:l[3]], s[l[1]:end]})
	}
	return a
}

func (a *archive) section(name string) string {
	for _, s := range a.sections {
		if s.name == name {
			return s.body
		}
	}
	return ""
}

func (a *archive) withWant(want string) string {
	var b strings.Builder
	for _, s := range a.sections {
		if s.name == "want" {
			continue
		}
		fmt.Fprintf(&b, "-- %s --\n%s", s.name, s.body)
	}
	fmt.Fprintf(&b, "-- want --\n%s", want)
	return b.String()
}

var marker = regexp.MustCompile(`/\*\|(\w+)\*/`)

// stripMarkers removes /*|name*/ markers and returns their offsets.
func stripMarkers(text string) (string, map[string]int) {
	marks := map[string]int{}
	var b strings.Builder
	last := 0
	for _, m := range marker.FindAllStringSubmatchIndex(text, -1) {
		b.WriteString(text[last:m[0]])
		marks[text[m[2]:m[3]]] = b.Len()
		last = m[1]
	}
	b.WriteString(text[last:])
	return b.String(), marks
}

type session struct {
	t      *testing.T
	a      *archive
	srv    *Server
	client *jsonrpc.Conn
	out    strings.Builder
	exit   chan int
	root   string // the workspace's directory, or ""

	enc      ide.Encoding
	initRes  json.RawMessage
	versions map[string]int
	texts    map[string]string
	marks    map[string]map[string]int

	mu       sync.Mutex
	diags    map[protocol.DocumentURI]protocol.PublishDiagnosticsParams
	received int64 // diagnostics notifications
	note     chan struct{}
	applied  []protocol.ApplyWorkspaceEditParams // the server's workspace/applyEdit requests
}

func (s *session) uri(name string) protocol.DocumentURI {
	if s.root != "" {
		return fileURI(filepath.Join(s.root, name))
	}
	return protocol.DocumentURI("file:///work/" + name)
}

// fileOf returns the file name of a URI of the session.
func (s *session) fileOf(u protocol.DocumentURI) string {
	if s.root != "" {
		if rel, err := filepath.Rel(s.root, uriPath(u)); err == nil {
			// An extracted library file's directory is named by a hash
			// of its archive's path.
			return libHash.ReplaceAllString(filepath.ToSlash(rel), "$1")
		}
	}
	return strings.TrimPrefix(string(u), "file:///work/")
}

var libHash = regexp.MustCompile(`^(\.libcache/[^/]*)-[0-9a-f]{12}`)

// textOf returns the text of file f: its open document's, or the
// workspace's on disk.
func (s *session) textOf(f string) string {
	if t, ok := s.texts[f]; ok {
		return t
	}
	if s.root != "" {
		if b, err := os.ReadFile(filepath.Join(s.root, f)); err == nil {
			return string(b)
		}
		if strings.HasPrefix(f, ".libcache/") {
			dir, rest, _ := strings.Cut(strings.TrimPrefix(f, ".libcache/"), "/")
			if m, _ := filepath.Glob(filepath.Join(s.root, ".libcache", dir+"-*", rest)); len(m) > 0 {
				b, _ := os.ReadFile(m[0])
				return string(b)
			}
		}
	}
	return ""
}

func runScript(t *testing.T, a *archive) string {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	s := &session{
		t: t, a: a, client: jsonrpc.NewConn(cr, cw), exit: make(chan int, 1),
		versions: map[string]int{}, texts: map[string]string{}, marks: map[string]map[string]int{},
		diags: map[protocol.DocumentURI]protocol.PublishDiagnosticsParams{}, note: make(chan struct{}, 1),
	}
	if a.section(project.ConfigFile) != "" {
		if _, err := classpath.FindStdlib(); err != nil {
			t.Skipf("no Kotlin standard library: %v", err)
		}
		s.root = t.TempDir()
		for _, sec := range a.sections {
			if sec.name == "script" || sec.name == "want" || strings.HasPrefix(sec.name, "_") {
				continue // _name is text for commands
			}
			text, _ := stripMarkers(sec.body)
			path := filepath.Join(s.root, sec.name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	s.srv = newServer(context.Background(), sr, sw)
	s.srv.scanEvery = 0 // the scan command scans
	// Navigation into the library's sources is off but for the scripts
	// that need them, "library" in a section _needs, and into stubs of
	// its metadata but for those that need "stubs".
	s.srv.lib.off, s.srv.lib.noStubs = true, true
	if strings.Contains(a.section("_needs"), "stubs") {
		s.srv.lib.noStubs = false
		s.srv.lib.dir = filepath.Join(s.root, ".libcache")
	}
	if strings.Contains(a.section("_needs"), "library") {
		stdlib, err := classpath.FindStdlib()
		if err != nil || libsrc.SourcesOf(stdlib) == "" {
			t.Skip("no sources of the Kotlin standard library")
		}
		s.srv.lib.off, s.srv.lib.noStubs = false, false
		s.srv.lib.dir = filepath.Join(s.root, ".libcache")
	}
	go func() {
		code, err := s.srv.run()
		if err != nil {
			t.Errorf("server: %v", err)
		}
		s.exit <- code
		sw.Close()
	}()
	go s.client.Serve(context.Background(), func(_ context.Context, r *jsonrpc.Request) (any, error) {
		if r.Method == "textDocument/publishDiagnostics" {
			var p protocol.PublishDiagnosticsParams
			if err := r.Unmarshal(&p); err == nil {
				s.mu.Lock()
				s.diags[p.URI] = p
				s.received++
				s.mu.Unlock()
				select {
				case s.note <- struct{}{}:
				default:
				}
			}
		}
		if r.Method == "workspace/applyEdit" {
			var p protocol.ApplyWorkspaceEditParams
			if err := r.Unmarshal(&p); err != nil {
				return nil, err
			}
			s.mu.Lock()
			s.applied = append(s.applied, p)
			s.mu.Unlock()
			return protocol.ApplyWorkspaceEditResult{Applied: true}, nil
		}
		return nil, nil
	})
	defer cw.Close()
	for _, line := range strings.Split(a.section("script"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fmt.Fprintf(&s.out, "> %s\n", line)
		s.run(line)
	}
	return s.out.String()
}

// params returns a command's JSON params, ROOT replaced.
func (s *session) params(text string) json.RawMessage {
	return json.RawMessage(strings.ReplaceAll(text, "ROOT", string(fileURI(s.root))))
}

func (s *session) call(method string, params, result any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.client.Call(ctx, method, params, result)
}

func (s *session) print(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		s.t.Fatal(err)
	}
	s.out.Write(b)
	s.out.WriteByte('\n')
}

// printResult prints a raw result normalized (sorted keys, indented), or
// the error.
func (s *session) printResult(raw json.RawMessage, err error) {
	if err != nil {
		var e *jsonrpc.Error
		if errors.As(err, &e) {
			fmt.Fprintf(&s.out, "error %d: %s\n", e.Code, e.Message)
			return
		}
		s.t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		s.t.Fatal(err)
	}
	s.print(v)
}

// waitDiagnostics waits for diagnostics of the given version of a file.
func (s *session) waitDiagnostics(name string, version int) {
	deadline := time.After(10 * time.Second)
	for {
		s.mu.Lock()
		p, ok := s.diags[s.uri(name)]
		s.mu.Unlock()
		if ok && (p.Version == nil || *p.Version >= version) {
			return
		}
		select {
		case <-s.note:
		case <-deadline:
			s.t.Fatalf("no diagnostics for %s version %d", name, version)
		}
	}
}

// position resolves @marker or line:char in file name's current text.
func (s *session) position(name, arg string) protocol.Position {
	if m, ok := strings.CutPrefix(arg, "@"); ok {
		off, ok := s.marks[name][m]
		if !ok {
			s.t.Fatalf("no marker %s in %s", m, name)
		}
		p := ide.NewDocument(name, 0, s.texts[name]).Pos(off, s.enc)
		return protocol.Position{Line: p.Line, Character: p.Char}
	}
	l, c, ok := strings.Cut(arg, ":")
	line, err1 := strconv.Atoi(l)
	char, err2 := strconv.Atoi(c)
	if !ok || err1 != nil || err2 != nil {
		s.t.Fatalf("bad position %q", arg)
	}
	return protocol.Position{Line: line, Character: char}
}

func (s *session) run(line string) {
	t := s.t
	cmd, rest, _ := strings.Cut(line, " ")
	args := strings.Fields(rest)
	file := func() string {
		if len(args) == 0 {
			t.Fatalf("%s: no file", cmd)
		}
		return args[0]
	}
	doc := func() protocol.TextDocumentIdentifier { return protocol.TextDocumentIdentifier{URI: s.uri(file())} }
	switch cmd {
	case "initialize":
		params := json.RawMessage(`{"processId":null,"capabilities":{"textDocument":{"documentSymbol":{"hierarchicalDocumentSymbolSupport":true}}}}`)
		if s.root != "" {
			params = json.RawMessage(fmt.Sprintf(`{"processId":null,"rootUri":%q,"capabilities":{}}`, fileURI(s.root)))
		}
		if rest != "" {
			params = s.params(rest)
		}
		if err := s.call("initialize", params, &s.initRes); err != nil {
			t.Fatal(err)
		}
		var r protocol.InitializeResult
		json.Unmarshal(s.initRes, &r)
		if r.Capabilities.PositionEncoding == protocol.EncodingUTF8 {
			s.enc = ide.UTF8
		}
		s.client.Notify("initialized", struct{}{})
	case "capabilities":
		s.printResult(s.initRes, nil)
	case "open":
		name := file()
		text, marks := stripMarkers(s.a.section(name))
		s.texts[name], s.marks[name], s.versions[name] = text, marks, 1
		s.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: s.uri(name), LanguageID: "kotlin", Version: 1, Text: text}})
		s.waitDiagnostics(name, 1)
	case "change":
		name := file()
		if len(args) < 4 {
			t.Fatalf("change: want F FROM TO \"text\"")
		}
		from, to := s.position(name, args[1]), s.position(name, args[2])
		q := strings.IndexByte(rest, '"')
		if q < 0 {
			t.Fatalf("change: no quoted text")
		}
		text, err := strconv.Unquote(strings.TrimSpace(rest[q:]))
		if err != nil {
			t.Fatalf("change: text: %v", err)
		}
		s.edit(name, []protocol.TextDocumentContentChangeEvent{{Range: &protocol.Range{Start: from, End: to}, Text: text}})
	case "replace":
		name := file()
		text, marks := stripMarkers(s.a.section(args[1]))
		s.marks[name] = marks
		s.edit(name, []protocol.TextDocumentContentChangeEvent{{Text: text}})
	case "close":
		s.mu.Lock()
		delete(s.diags, s.uri(file()))
		s.mu.Unlock()
		s.client.Notify("textDocument/didClose", protocol.DidCloseTextDocumentParams{TextDocument: doc()})
		s.waitDiagnostics(file(), 0) // the empty list
	case "wait":
		// The server handles messages in order: a request's answer means
		// it has handled the notifications before it.
		s.call("kc/sync", nil, nil)
		n := s.srv.waitIdle()
		deadline := time.After(10 * time.Second)
		for {
			s.mu.Lock()
			received := s.received
			s.mu.Unlock()
			if received >= n {
				break
			}
			select {
			case <-s.note:
			case <-deadline:
				t.Fatalf("wait: %d diagnostics notifications received of %d", received, n)
			}
		}
	case "diagnostics":
		s.mu.Lock()
		p := s.diags[s.uri(file())]
		s.mu.Unlock()
		s.print(p.Diagnostics)
	case "symbols":
		var raw json.RawMessage
		err := s.call("textDocument/documentSymbol", protocol.DocumentSymbolParams{TextDocument: doc()}, &raw)
		s.printResult(raw, err)
	case "folding":
		var raw json.RawMessage
		err := s.call("textDocument/foldingRange", protocol.FoldingRangeParams{TextDocument: doc()}, &raw)
		s.printResult(raw, err)
	case "selection":
		name := file()
		var ps []protocol.Position
		for _, a := range args[1:] {
			ps = append(ps, s.position(name, a))
		}
		var res []*protocol.SelectionRange
		if err := s.call("textDocument/selectionRange", protocol.SelectionRangeParams{TextDocument: doc(), Positions: ps}, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		d := ide.NewDocument(name, 0, s.texts[name])
		for i, r := range res {
			fmt.Fprintf(&s.out, "%s:\n", args[1+i])
			for ; r != nil; r = r.Parent {
				fmt.Fprintf(&s.out, "  %s %s\n", fmtRange(r.Range), excerpt(d, r.Range, s.enc))
			}
		}
	case "definition", "typedef", "references", "implementation":
		name := file()
		if len(args) < 2 {
			t.Fatalf("%s: no position", cmd)
		}
		at := protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}
		var params any = at
		method := map[string]string{"definition": "textDocument/definition", "typedef": "textDocument/typeDefinition", "references": "textDocument/references", "implementation": "textDocument/implementation"}[cmd]
		if cmd == "references" {
			params = protocol.ReferenceParams{TextDocumentPositionParams: at, Context: protocol.ReferenceContext{IncludeDeclaration: len(args) > 2 && args[2] == "decl"}}
		}
		var res []protocol.Location
		if err := s.call(method, params, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(res) == 0 {
			fmt.Fprintln(&s.out, "(none)")
		}
		for _, l := range res {
			f := s.fileOf(l.URI)
			fmt.Fprintf(&s.out, "%s %s %s\n", f, fmtRange(l.Range), excerpt(ide.NewDocument(f, 0, s.textOf(f)), l.Range, s.enc))
		}
	case "rename":
		// rename F POS NAME prints the edits per file, and applies those
		// of open documents.
		name := file()
		if len(args) < 3 {
			t.Fatalf("rename: want F POS NAME")
		}
		var we *protocol.WorkspaceEdit
		p := protocol.RenameParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}, NewName: args[2]}
		if err := s.call("textDocument/rename", p, &we); err != nil {
			s.printResult(nil, err)
			return
		}
		s.printEdit(we, "")
		s.applyEdit(we)
	case "move":
		// move FROM TO prints workspace/willRenameFiles' edits per file
		// for renaming a file or folder, and applies those of open
		// documents.
		if len(args) != 2 {
			t.Fatalf("move: want FROM TO")
		}
		var we *protocol.WorkspaceEdit
		p := protocol.RenameFilesParams{Files: []protocol.FileRename{{OldURI: s.uri(args[0]), NewURI: s.uri(args[1])}}}
		if err := s.call("workspace/willRenameFiles", p, &we); err != nil {
			s.printResult(nil, err)
			return
		}
		if we == nil {
			fmt.Fprintln(&s.out, "(none)")
			return
		}
		s.printEdit(we, "")
		s.applyEdit(we)
	case "applied":
		// applied waits for the server's workspace/applyEdit requests and
		// prints their edits.
		deadline := time.Now().Add(5 * time.Second)
		for {
			s.mu.Lock()
			got := s.applied
			s.applied = nil
			s.mu.Unlock()
			if len(got) > 0 {
				for _, p := range got {
					fmt.Fprintf(&s.out, "%s\n", p.Label)
					s.printEdit(&p.Edit, "  ")
				}
				break
			}
			if time.Now().After(deadline) {
				fmt.Fprintln(&s.out, "(none)")
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	case "calls":
		// calls F POS prints the call hierarchy item at POS, and its
		// incoming and outgoing calls.
		name := file()
		if len(args) < 2 {
			t.Fatalf("calls: want F POS")
		}
		var items []protocol.CallHierarchyItem
		if err := s.call("textDocument/prepareCallHierarchy", protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}, &items); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(items) == 0 {
			fmt.Fprintln(&s.out, "(none)")
			return
		}
		itemText := func(it protocol.CallHierarchyItem) string {
			f := s.fileOf(it.URI)
			return fmt.Sprintf("%s [%d] (%s) %s %s", it.Name, it.Kind, it.Detail, f, fmtRange(it.SelectionRange))
		}
		ranges := func(f string, rs []protocol.Range) string {
			d := ide.NewDocument(f, 0, s.textOf(f))
			var parts []string
			for _, r := range rs {
				parts = append(parts, fmtRange(r)+" "+excerpt(d, r, s.enc))
			}
			return strings.Join(parts, ", ")
		}
		it := items[0]
		fmt.Fprintf(&s.out, "%s\n", itemText(it))
		var in []protocol.CallHierarchyIncomingCall
		if err := s.call("callHierarchy/incomingCalls", protocol.CallHierarchyCallsParams{Item: it}, &in); err != nil {
			s.printResult(nil, err)
			return
		}
		for _, c := range in {
			fmt.Fprintf(&s.out, "  <- %s: %s\n", itemText(c.From), ranges(s.fileOf(c.From.URI), c.FromRanges))
		}
		var out []protocol.CallHierarchyOutgoingCall
		if err := s.call("callHierarchy/outgoingCalls", protocol.CallHierarchyCallsParams{Item: it}, &out); err != nil {
			s.printResult(nil, err)
			return
		}
		for _, c := range out {
			fmt.Fprintf(&s.out, "  -> %s: %s\n", itemText(c.To), ranges(s.fileOf(it.URI), c.FromRanges))
		}
	case "actions":
		// actions F FROM [TO] [only=KIND] [apply=N] prints the code
		// actions of a range, each with its edits; apply=N applies the
		// Nth's edits to the open documents.
		name := file()
		if len(args) < 2 {
			t.Fatalf("actions: want F FROM [TO]")
		}
		from := s.position(name, args[1])
		to := from
		var only []string
		apply := 0
		for _, a := range args[2:] {
			switch {
			case strings.HasPrefix(a, "only="):
				only = append(only, strings.TrimPrefix(a, "only="))
			case strings.HasPrefix(a, "apply="):
				apply, _ = strconv.Atoi(strings.TrimPrefix(a, "apply="))
			default:
				to = s.position(name, a)
			}
		}
		var res []protocol.CodeAction
		p := protocol.CodeActionParams{TextDocument: doc(), Range: protocol.Range{Start: from, End: to}, Context: protocol.CodeActionContext{Only: only}}
		if err := s.call("textDocument/codeAction", p, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(res) == 0 {
			fmt.Fprintln(&s.out, "(none)")
		}
		for i, a := range res {
			pref := ""
			if a.IsPreferred {
				pref = " preferred"
			}
			var codes []string
			for _, d := range a.Diagnostics {
				codes = append(codes, d.Code)
			}
			fmt.Fprintf(&s.out, "%d. %s [%s%s] %s\n", i+1, a.Title, a.Kind, pref, strings.Join(codes, ","))
			if a.Edit != nil {
				s.printEdit(a.Edit, "  ")
			}
		}
		if apply > 0 && apply <= len(res) && res[apply-1].Edit != nil {
			s.applyEdit(res[apply-1].Edit)
		}
	case "format", "ontype":
		// format F [FROM TO] and ontype F POS CH print the formatting
		// edits of the document, a range or a typed character (\n for a
		// newline), then the text with them applied, which the open
		// document becomes.
		name := file()
		opts := protocol.FormattingOptions{TabSize: 4, InsertSpaces: true}
		var res []protocol.TextEdit
		var err error
		switch {
		case cmd == "ontype" && len(args) == 3:
			ch := strings.ReplaceAll(args[2], `\n`, "\n")
			err = s.call("textDocument/onTypeFormatting", protocol.DocumentOnTypeFormattingParams{TextDocument: doc(), Position: s.position(name, args[1]), Ch: ch, Options: opts}, &res)
		case cmd == "ontype":
			t.Fatalf("ontype: want F POS CH")
		case len(args) == 3:
			err = s.call("textDocument/rangeFormatting", protocol.DocumentRangeFormattingParams{TextDocument: doc(), Range: protocol.Range{Start: s.position(name, args[1]), End: s.position(name, args[2])}, Options: opts}, &res)
		default:
			err = s.call("textDocument/formatting", protocol.DocumentFormattingParams{TextDocument: doc(), Options: opts}, &res)
		}
		if err != nil {
			s.printResult(nil, err)
			return
		}
		we := &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{s.uri(name): res}}
		s.printEdit(we, "")
		s.applyEdit(we)
		fmt.Fprintf(&s.out, "%s", s.texts[name])
	case "types":
		// types F POS prints the type hierarchy item at POS, its
		// supertypes, three levels deep, and its subtypes, two.
		name := file()
		if len(args) < 2 {
			t.Fatalf("types: want F POS")
		}
		var items []protocol.TypeHierarchyItem
		if err := s.call("textDocument/prepareTypeHierarchy", protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}, &items); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(items) == 0 {
			fmt.Fprintln(&s.out, "(none)")
			return
		}
		itemText := func(it protocol.TypeHierarchyItem) string {
			return fmt.Sprintf("%s [%d] (%s) %s %s", it.Name, it.Kind, it.Detail, s.fileOf(it.URI), fmtRange(it.SelectionRange))
		}
		fmt.Fprintf(&s.out, "%s\n", itemText(items[0]))
		var walk func(method, mark string, it protocol.TypeHierarchyItem, depth int)
		walk = func(method, mark string, it protocol.TypeHierarchyItem, depth int) {
			var res []protocol.TypeHierarchyItem
			if err := s.call(method, protocol.TypeHierarchyParams{Item: it}, &res); err != nil {
				s.printResult(nil, err)
				return
			}
			for _, r := range res {
				fmt.Fprintf(&s.out, "%s%s %s\n", strings.Repeat("  ", depth), mark, itemText(r))
				if depth < 3 && !(mark == "v" && depth == 2) {
					walk(method, mark, r, depth+1)
				}
			}
		}
		walk("typeHierarchy/supertypes", "^", items[0], 1)
		walk("typeHierarchy/subtypes", "v", items[0], 1)
	case "prepare":
		name := file()
		if len(args) < 2 {
			t.Fatalf("prepare: want F POS")
		}
		var res protocol.PrepareRenameResult
		if err := s.call("textDocument/prepareRename", protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		fmt.Fprintf(&s.out, "%s %s %q\n", fmtRange(res.Range), excerpt(ide.NewDocument(name, 0, s.texts[name]), res.Range, s.enc), res.Placeholder)
	case "inlay":
		name := file()
		d := ide.NewDocument(name, 0, s.texts[name])
		end := d.Pos(len(s.texts[name]), s.enc)
		var res []protocol.InlayHint
		if err := s.call("textDocument/inlayHint", protocol.InlayHintParams{TextDocument: doc(), Range: protocol.Range{End: protocol.Position{Line: end.Line, Character: end.Char}}}, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(res) == 0 {
			fmt.Fprintln(&s.out, "(none)")
		}
		for _, h := range res {
			kind := map[int]string{protocol.InlayHintType: "type", protocol.InlayHintParameter: "parameter"}[h.Kind]
			fmt.Fprintf(&s.out, "%d:%d %s %q\n", h.Position.Line, h.Position.Character, kind, h.Label)
		}
	case "lens":
		name := file()
		var lenses []protocol.CodeLens
		if err := s.call("textDocument/codeLens", protocol.CodeLensParams{TextDocument: doc()}, &lenses); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(lenses) == 0 {
			fmt.Fprintln(&s.out, "(none)")
		}
		d := ide.NewDocument(name, 0, s.textOf(name))
		for _, l := range lenses {
			args, _ := json.Marshal(l.Command.Arguments)
			fmt.Fprintf(&s.out, "%s %s %s %s %s\n", fmtRange(l.Range), excerpt(d, l.Range, s.enc), l.Command.Title, l.Command.Command,
				strings.ReplaceAll(string(args), string(s.uri(name)), name))
		}
	case "run":
		file()
		n, debug := 0, false
		for _, a := range args[1:] {
			if a == "debug" {
				debug = true
			} else {
				n, _ = strconv.Atoi(a)
			}
		}
		var lenses []protocol.CodeLens
		if err := s.call("textDocument/codeLens", protocol.CodeLensParams{TextDocument: doc()}, &lenses); err != nil {
			s.printResult(nil, err)
			return
		}
		var runs []protocol.CodeLens
		for _, l := range lenses {
			if l.Command.Command == "kc.run" && !debug || l.Command.Command == "kc.debug" && debug {
				runs = append(runs, l)
			}
		}
		if n >= len(runs) {
			fmt.Fprintln(&s.out, "(none)")
			return
		}
		var c RunConfig
		if err := s.call("workspace/executeCommand", map[string]any{"command": runs[n].Command.Command, "arguments": runs[n].Command.Arguments}, &c); err != nil {
			s.printResult(nil, err)
			return
		}
		short := func(list []string) string {
			var out []string
			for _, a := range list {
				var parts []string
				for _, p := range filepath.SplitList(a) {
					if rel, err := filepath.Rel(s.root, p); err == nil && !strings.HasPrefix(rel, "..") && filepath.IsAbs(p) {
						p = filepath.ToSlash(rel)
					} else if filepath.IsAbs(p) {
						p = filepath.Base(p)
					}
					parts = append(parts, p)
				}
				out = append(out, strings.Join(parts, ":"))
			}
			return strings.Join(out, " ")
		}
		fmt.Fprintf(&s.out, "main %s module %s out %s port %d\nbuild %s\nrun %s\n", c.MainClass, c.Module, short([]string{c.OutputDir}), c.DebugPort, short(c.Build), short(c.Run))
	case "wsymbol":
		q := ""
		if len(args) > 0 {
			q = args[0]
		}
		var res []json.RawMessage
		if err := s.call("workspace/symbol", protocol.WorkspaceSymbolParams{Query: q}, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(res) == 0 {
			fmt.Fprintln(&s.out, "(none)")
		}
		for _, raw := range res {
			// A symbol with only a URI is resolved, and marked "resolved".
			var si protocol.SymbolInformation
			var ws struct {
				Location struct {
					Range *protocol.Range `json:"range"`
				} `json:"location"`
			}
			json.Unmarshal(raw, &si)
			json.Unmarshal(raw, &ws)
			mark := ""
			if ws.Location.Range == nil {
				var resolved json.RawMessage
				if err := s.call("workspaceSymbol/resolve", raw, &resolved); err != nil {
					s.printResult(nil, err)
					continue
				}
				si = protocol.SymbolInformation{}
				json.Unmarshal(resolved, &si)
				mark = " resolved"
			}
			f := s.fileOf(si.Location.URI)
			fmt.Fprintf(&s.out, "%s %d %q in %q %s %s%s\n", si.Name, si.Kind, si.ContainerName, f, fmtRange(si.Location.Range), excerpt(ide.NewDocument(f, 0, s.textOf(f)), si.Location.Range, s.enc), mark)
		}
	case "highlight":
		name := file()
		if len(args) < 2 {
			t.Fatalf("%s: no position", cmd)
		}
		var res []protocol.DocumentHighlight
		if err := s.call("textDocument/documentHighlight", protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(res) == 0 {
			fmt.Fprintln(&s.out, "(none)")
		}
		d := ide.NewDocument(name, 0, s.texts[name])
		for _, h := range res {
			kind := map[int]string{protocol.HighlightText: "text", protocol.HighlightRead: "read", protocol.HighlightWrite: "write"}[h.Kind]
			fmt.Fprintf(&s.out, "%s %s %s\n", fmtRange(h.Range), kind, excerpt(d, h.Range, s.enc))
		}
	case "hover":
		name := file()
		if len(args) < 2 {
			t.Fatalf("%s: no position", cmd)
		}
		var res *protocol.Hover
		if err := s.call("textDocument/hover", protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if res == nil {
			fmt.Fprintln(&s.out, "(none)")
			return
		}
		if res.Range != nil {
			d := ide.NewDocument(name, 0, s.texts[name])
			fmt.Fprintf(&s.out, "%s %s\n", fmtRange(*res.Range), excerpt(d, *res.Range, s.enc))
		}
		fmt.Fprintf(&s.out, "%s\n", strings.TrimRight(res.Contents.Value, "\n"))
	case "signature":
		// signature F POS: the candidates, the active one marked with *,
		// with the active parameter in [brackets], and the first line of
		// their documentation.
		name := file()
		if len(args) < 2 {
			t.Fatalf("%s: no position", cmd)
		}
		var res *protocol.SignatureHelp
		params := protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}
		if err := s.call("textDocument/signatureHelp", params, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if res == nil {
			fmt.Fprintln(&s.out, "(none)")
			return
		}
		for i, sig := range res.Signatures {
			label := []rune(sig.Label) // ASCII in the tests: UTF-16 units are runes
			if sig.ActiveParameter != nil && *sig.ActiveParameter >= 0 && *sig.ActiveParameter < len(sig.Parameters) {
				pr := sig.Parameters[*sig.ActiveParameter].Label
				label = []rune(string(label[:pr[0]]) + "[" + string(label[pr[0]:pr[1]]) + "]" + string(label[pr[1]:]))
			}
			mark := " "
			if i == res.ActiveSignature {
				mark = "*"
			}
			line := mark + " " + string(label)
			if sig.Documentation != nil {
				first, _, _ := strings.Cut(sig.Documentation.Value, "\n")
				line += "  // " + first
			}
			fmt.Fprintln(&s.out, line)
		}
	case "complete":
		// complete F POS [N [doc]]: the first N items (default 12), as
		// label, kind and detail, then what an item inserts if it isn't
		// its label, and the import it adds; with doc, the first item's
		// documentation, from completionItem/resolve.
		name := file()
		if len(args) < 2 {
			t.Fatalf("%s: no position", cmd)
		}
		n := 12
		if len(args) > 2 {
			n, _ = strconv.Atoi(args[2])
		}
		var res protocol.CompletionList
		params := protocol.CompletionParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: doc(), Position: s.position(name, args[1])}}
		if err := s.call("textDocument/completion", params, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		if len(res.Items) == 0 {
			fmt.Fprintln(&s.out, "(none)")
		}
		if res.IsIncomplete {
			fmt.Fprintln(&s.out, "(incomplete)")
		}
		kinds := map[int]string{}
		for k, v := range completionKinds {
			kinds[v] = k
		}
		kinds[protocol.CompletionVariable], kinds[protocol.CompletionClass], kinds[protocol.CompletionInterface] = "variable", "class", "interface"
		kinds[protocol.CompletionSnippet] = "snippet"
		for i, it := range res.Items {
			if i == n {
				fmt.Fprintf(&s.out, "... %d more\n", len(res.Items)-n)
				break
			}
			line := it.Label + " " + kinds[it.Kind]
			if it.Detail != "" {
				line += " " + it.Detail
			}
			if it.TextEdit != nil && it.TextEdit.NewText != it.Label {
				line += fmt.Sprintf(" => %q", it.TextEdit.NewText)
			}
			if it.Kind == protocol.CompletionSnippet && it.FilterText != "" {
				// A postfix template's: its edit's range.
				line += " @" + fmtRange(it.TextEdit.Range)
			}
			for _, e := range it.AdditionalTextEdits {
				line += fmt.Sprintf(" +%s %q", fmtRange(e.Range), e.NewText)
			}
			if len(it.Tags) > 0 {
				line += " (deprecated)"
			}
			if len(it.Data) > 0 && i == 0 && len(args) > 3 && args[3] == "doc" {
				var resolved protocol.CompletionItem
				if err := s.call("completionItem/resolve", it, &resolved); err == nil && resolved.Documentation != nil {
					line += "\n    " + strings.ReplaceAll(resolved.Documentation.Value, "\n", "\n    ")
				}
			}
			fmt.Fprintln(&s.out, line)
		}
	case "tokens":
		name := file()
		var res protocol.SemanticTokens
		if err := s.call("textDocument/semanticTokens/full", protocol.SemanticTokensParams{TextDocument: doc()}, &res); err != nil {
			s.printResult(nil, err)
			return
		}
		s.printTokens(name, res.Data)
	case "request":
		params := json.RawMessage("null")
		if len(args) > 1 {
			params = s.params(strings.TrimSpace(strings.TrimPrefix(rest, args[0])))
		}
		var raw json.RawMessage
		err := s.call(args[0], params, &raw)
		s.printResult(raw, err)
	case "notify":
		if len(args) < 2 {
			t.Fatalf("notify: want METHOD params")
		}
		s.client.Notify(args[0], s.params(strings.TrimSpace(strings.TrimPrefix(rest, args[0]))))
	case "scan":
		s.srv.scanSources()
	case "write":
		if s.root == "" || len(args) != 2 {
			t.Fatalf("write: want F SECTION in a workspace")
		}
		path := filepath.Join(s.root, file())
		if args[1] == "-" {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			return
		}
		text, _ := stripMarkers(s.a.section(args[1]))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	case "shutdown":
		if err := s.call("shutdown", nil, nil); err != nil {
			t.Fatal(err)
		}
	case "exit":
		s.client.Notify("exit", nil)
		select {
		case code := <-s.exit:
			fmt.Fprintf(&s.out, "exit code %d\n", code)
		case <-time.After(10 * time.Second):
			t.Fatal("server did not exit")
		}
	default:
		t.Fatalf("unknown command %q", cmd)
	}
}

func (s *session) edit(name string, changes []protocol.TextDocumentContentChangeEvent) {
	s.versions[name]++
	v := s.versions[name]
	s.client.Notify("textDocument/didChange", protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{URI: s.uri(name), Version: v},
		ContentChanges: changes,
	})
	s.waitDiagnostics(name, v)
	// Keep the harness's copy of the text in step, for markers and
	// excerpts; markers after an edited range move with the text.
	d := ide.NewDocument(name, 0, s.texts[name])
	for _, c := range changes {
		e := ide.Edit{Whole: true, Text: c.Text}
		if c.Range != nil {
			e = ide.Edit{From: pos(c.Range.Start), To: pos(c.Range.End), Text: c.Text}
			from, err1 := d.Offset(e.From, s.enc)
			to, err2 := d.Offset(e.To, s.enc)
			if err1 == nil && err2 == nil {
				for m, off := range s.marks[name] {
					switch {
					case off >= to:
						s.marks[name][m] = off + len(c.Text) - (to - from)
					case off > from:
						s.marks[name][m] = from
					}
				}
			}
		}
		nd, err := d.Apply(v, []ide.Edit{e}, s.enc)
		if err != nil {
			s.t.Fatal(err)
		}
		d = nd
	}
	s.texts[name] = d.Text()
}

func posLess(a, b protocol.Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
}

func fmtRange(r protocol.Range) string {
	return fmt.Sprintf("%d:%d-%d:%d", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
}

func excerpt(d *ide.Document, r protocol.Range, enc ide.Encoding) string {
	a, _ := d.Offset(pos(r.Start), enc)
	b, _ := d.Offset(pos(r.End), enc)
	text := d.Text()[a:b]
	if len(text) > 40 {
		text = text[:20] + "…" + text[len(text)-17:]
	}
	return strconv.Quote(text)
}

func (s *session) printTokens(name string, data []uint32) {
	d := ide.NewDocument(name, 0, s.texts[name])
	line, char := 0, 0
	for i := 0; i+5 <= len(data); i += 5 {
		if data[i] > 0 {
			char = 0
		}
		line += int(data[i])
		char += int(data[i+1])
		start := protocol.Position{Line: line, Character: char}
		end := protocol.Position{Line: line, Character: char + int(data[i+2])}
		var mods []string
		for b, m := range ide.TokenModifierNames {
			if data[i+4]&(1<<b) != 0 {
				mods = append(mods, m)
			}
		}
		fmt.Fprintf(&s.out, "%d:%d %s", line, char, ide.TokenTypeNames[data[i+3]])
		if len(mods) > 0 {
			fmt.Fprintf(&s.out, " [%s]", strings.Join(mods, " "))
		}
		fmt.Fprintf(&s.out, " %s\n", excerpt(d, protocol.Range{Start: start, End: end}, s.enc))
	}
}

// diffLines shows the first difference between two outputs with context.
func diffLines(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	i := 0
	for i < len(w) && i < len(g) && w[i] == g[i] {
		i++
	}
	lo := max(0, i-3)
	var b strings.Builder
	fmt.Fprintf(&b, "first difference at line %d\n", i+1)
	for j := lo; j < i; j++ {
		fmt.Fprintf(&b, "  %s\n", w[j])
	}
	for j := i; j < min(len(w), i+5); j++ {
		fmt.Fprintf(&b, "- %s\n", w[j])
	}
	for j := i; j < min(len(g), i+5); j++ {
		fmt.Fprintf(&b, "+ %s\n", g[j])
	}
	return b.String()
}

// printEdit prints a workspace edit's edits, by file and position.
func (s *session) printEdit(we *protocol.WorkspaceEdit, indent string) {
	var files []string
	for u := range we.Changes {
		files = append(files, s.fileOf(u))
	}
	sort.Strings(files)
	for _, f := range files {
		es := slices.Clone(we.Changes[s.uri(f)])
		sort.Slice(es, func(i, j int) bool { return posLess(es[i].Range.Start, es[j].Range.Start) })
		d := ide.NewDocument(f, 0, s.textOf(f))
		fmt.Fprintf(&s.out, "%s%s:\n", indent, f)
		for _, e := range es {
			fmt.Fprintf(&s.out, "%s  %s %s -> %q\n", indent, fmtRange(e.Range), excerpt(d, e.Range, s.enc), e.NewText)
		}
	}
}

// applyEdit applies a workspace edit's edits of open documents, last
// first per file.
func (s *session) applyEdit(we *protocol.WorkspaceEdit) {
	var files []string
	for u := range we.Changes {
		files = append(files, s.fileOf(u))
	}
	sort.Strings(files)
	for _, f := range files {
		if _, open := s.texts[f]; !open {
			continue
		}
		es := slices.Clone(we.Changes[s.uri(f)])
		sort.Slice(es, func(i, j int) bool { return posLess(es[i].Range.Start, es[j].Range.Start) })
		changes := make([]protocol.TextDocumentContentChangeEvent, len(es))
		for i, e := range es {
			r := e.Range
			changes[len(es)-1-i] = protocol.TextDocumentContentChangeEvent{Range: &r, Text: e.NewText}
		}
		s.edit(f, changes)
	}
}
