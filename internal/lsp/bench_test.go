package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
)

// The semantic tier's benchmarks (docs/lsp.md, Benchmarks), through the
// server over pipes. The workspace is KC_LSP_BENCH's, a directory with a
// kc-lsp.json (or a Gradle model cached), and the file edited
// KC_LSP_BENCH_FILE, a path under it; or else a generated module of 200
// files, with the standard library.

// benchClient is a server in process and a client of it.
type benchClient struct {
	srv    *Server
	client *jsonrpc.Conn
	done   chan struct{}
	close  func()
}

func startServer(b *testing.B, root string) *benchClient {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	c := &benchClient{srv: newServer(context.Background(), sr, sw), client: jsonrpc.NewConn(cr, cw), done: make(chan struct{})}
	go func() {
		c.srv.run()
		sw.Close()
		close(c.done)
	}()
	go c.client.Serve(context.Background(), func(context.Context, *jsonrpc.Request) (any, error) { return nil, nil })
	c.close = func() {
		c.call("shutdown")
		c.client.Notify("exit", nil)
		<-c.done
		cw.Close()
	}
	params := map[string]any{"processId": nil, "rootUri": fileURI(root), "capabilities": map[string]any{},
		"initializationOptions": map[string]any{"gradle": false}}
	if err := c.client.Call(context.Background(), "initialize", params, nil); err != nil {
		b.Fatal(err)
	}
	c.client.Notify("initialized", struct{}{})
	return c
}

func (c *benchClient) call(method string) {
	c.client.Call(context.Background(), method, nil, nil)
}

// settle waits until the server has handled what was sent and its
// updates are done.
func (c *benchClient) settle() {
	c.call("kc/sync")
	c.srv.waitIdle()
}

// benchWorkspace returns the workspace's root and the file to edit, and
// the offset in it of a body edit's place: the start of the first line of
// the last function body.
func benchWorkspace(b *testing.B) (root, file string, at int) {
	root = os.Getenv("KC_LSP_BENCH")
	if root != "" {
		file = os.Getenv("KC_LSP_BENCH_FILE")
		if file == "" {
			b.Fatal("KC_LSP_BENCH_FILE is not set")
		}
	} else {
		if _, err := classpath.FindStdlib(); err != nil {
			b.Skipf("no Kotlin standard library: %v", err)
		}
		root = b.TempDir()
		generate(b, root, 200, false)
		file = filepath.Join(root, "src", "gen", "File100.kt")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		b.Fatal(err)
	}
	at = bodyLine(string(data))
	if at < 0 {
		b.Fatalf("%s: no function body with a line to edit", file)
	}
	return root, file, at
}

// bodyLine returns the offset of the start of the line after the last
// "{" that ends a line beginning with "fun" or "    fun", or -1.
func bodyLine(text string) int {
	at := -1
	off := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasSuffix(t, "{") && (strings.HasPrefix(t, "fun ") || strings.Contains(t, " fun ")) {
			at = off + len(line)
		}
		off += len(line)
	}
	if at >= len(text) {
		return -1
	}
	return at
}

// generate writes a module of n files to root: classes that call each
// other's members, with lambdas, string templates and when. With split,
// the first half is a module lib, in src/lib, and the second a module
// app, in src/app, that depends on it and whose last file calls lib's
// first.
func generate(b *testing.B, root string, n int, split bool) {
	write := func(name, text string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	write("kc-lsp.json", `{"modules": [{"name": "gen", "sourceRoots": ["src"]}]}`)
	if split {
		write("kc-lsp.json", `{"modules": [{"name": "lib", "sourceRoots": ["src/lib"]},
  {"name": "app", "sourceRoots": ["src/app"], "dependsOn": ["lib"]}]}`)
	}
	for i := range n {
		next := (i + 1) % n
		dir := "gen"
		if split && i < n/2 {
			dir, next = "lib", (i+1)%(n/2)
		} else if split {
			dir = "app"
		}
		write(fmt.Sprintf("src/%s/File%d.kt", dir, i), fmt.Sprintf(`package gen

data class Item%[1]d(val id: Int, val name: String, val tags: List<String> = emptyList())

interface Source%[1]d {
    fun items(): List<Item%[1]d>
}

class Repo%[1]d(private val source: Source%[1]d) {
    private val cache = mutableMapOf<Int, Item%[1]d>()

    fun find(id: Int): Item%[1]d? = cache[id] ?: source.items().firstOrNull { it.id == id }?.also { cache[id] = it }

    fun names(prefix: String): List<String> =
        source.items().filter { it.name.startsWith(prefix) }.sortedBy { it.id }.map { "${it.id}: ${it.name}" }

    fun describe(id: Int): String {
        val item = find(id) ?: return "none"
        val kind = when {
            item.tags.isEmpty() -> "plain"
            item.tags.size == 1 -> "tagged ${item.tags.first()}"
            else -> "tagged ${item.tags.size} times"
        }
        return "${item.name} ($kind)"
    }

    fun next(): Int = Repo%[2]d(object : Source%[2]d {
        override fun items() = listOf(Item%[2]d(1, "a"))
    }).count() + 1

    fun count(): Int = source.items().size
}

fun total%[1]d(items: List<Item%[1]d>): Int {
    var sum = 0
    for (item in items) {
        sum += item.id * 2
    }
    return sum + items.sumOf { it.name.length }
}
`, i, next))
	}
}

// BenchmarkColdOpen times opening a file in a new server to its first
// semantic diagnostics: loading the project, opening the library, and
// checking the module whole.
func BenchmarkColdOpen(b *testing.B) {
	root, file, _ := benchWorkspace(b)
	text, _ := os.ReadFile(file)
	for b.Loop() {
		c := startServer(b, root)
		c.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: fileURI(file), LanguageID: "kotlin", Version: 1, Text: string(text)}})
		c.settle()
		b.StopTimer()
		c.close()
		b.StartTimer()
	}
}

// BenchmarkBodyEdit times an edit inside a function body to its semantic
// diagnostics, without the debounce: the median and 95th percentile of
// the edits.
func BenchmarkBodyEdit(b *testing.B) {
	root, file, at := benchWorkspace(b)
	text, _ := os.ReadFile(file)
	defer func(d time.Duration) { debounce = d }(debounce)
	debounce = 0
	c := startServer(b, root)
	defer c.close()
	u := fileURI(file)
	c.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, LanguageID: "kotlin", Version: 1, Text: string(text)}})
	c.settle()
	pos := func(off int) protocol.Position {
		line := strings.Count(string(text[:off]), "\n")
		return protocol.Position{Line: line, Character: 0}
	}
	p := pos(at)
	var times []time.Duration
	v := 1
	for b.Loop() {
		// Insert a statement, then remove it: both are body edits.
		for _, change := range []protocol.TextDocumentContentChangeEvent{
			{Range: &protocol.Range{Start: p, End: p}, Text: "        val unused = 1 + 2\n"},
			{Range: &protocol.Range{Start: p, End: protocol.Position{Line: p.Line + 1}}, Text: ""},
		} {
			v++
			start := time.Now()
			c.client.Notify("textDocument/didChange", protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{URI: u, Version: v}, ContentChanges: []protocol.TextDocumentContentChangeEvent{change}})
			c.settle()
			times = append(times, time.Since(start))
		}
	}
	slices.Sort(times)
	b.ReportMetric(float64(times[len(times)/2].Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(times[len(times)*95/100].Microseconds())/1000, "p95-ms")
}

// BenchmarkModuleBodyEdit times a body edit of a dependency's file to
// the semantic diagnostics of it and its dependent's, without the
// debounce, on a generated workspace of two modules (generate's split):
// the dependent only declares the dependency's files.
func BenchmarkModuleBodyEdit(b *testing.B) {
	if _, err := classpath.FindStdlib(); err != nil {
		b.Skipf("no Kotlin standard library: %v", err)
	}
	root := b.TempDir()
	generate(b, root, 200, true)
	file := filepath.Join(root, "src", "lib", "File50.kt")
	other := filepath.Join(root, "src", "app", "File150.kt")
	text, _ := os.ReadFile(file)
	otherText, _ := os.ReadFile(other)
	defer func(d time.Duration) { debounce = d }(debounce)
	debounce = 0
	c := startServer(b, root)
	defer c.close()
	u := fileURI(file)
	for _, d := range []struct {
		path string
		text []byte
	}{{other, otherText}, {file, text}} {
		c.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: fileURI(d.path), LanguageID: "kotlin", Version: 1, Text: string(d.text)}})
	}
	c.settle()
	at := bodyLine(string(text))
	p := protocol.Position{Line: strings.Count(string(text[:at]), "\n")}
	var times []time.Duration
	v := 1
	for b.Loop() {
		for _, change := range []protocol.TextDocumentContentChangeEvent{
			{Range: &protocol.Range{Start: p, End: p}, Text: "        val unused = 1 + 2\n"},
			{Range: &protocol.Range{Start: p, End: protocol.Position{Line: p.Line + 1}}, Text: ""},
		} {
			v++
			start := time.Now()
			c.client.Notify("textDocument/didChange", protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{URI: u, Version: v}, ContentChanges: []protocol.TextDocumentContentChangeEvent{change}})
			c.settle()
			times = append(times, time.Since(start))
		}
	}
	slices.Sort(times)
	b.ReportMetric(float64(times[len(times)/2].Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(times[len(times)*95/100].Microseconds())/1000, "p95-ms")
}

// BenchmarkDeclarationEdit times a signature edit to its semantic
// diagnostics, without the debounce: a parameter with a default added to
// a member function that another file calls, then removed. On another
// workspace (KC_LSP_BENCH) it edits the last "fun name(" of the file.
func BenchmarkDeclarationEdit(b *testing.B) {
	root, file, _ := benchWorkspace(b)
	data, _ := os.ReadFile(file)
	text := string(data)
	defer func(d time.Duration) { debounce = d }(debounce)
	debounce = 0
	off := strings.LastIndex(text, "fun count(")
	if off < 0 {
		off = strings.LastIndex(text, "fun ")
		if off < 0 {
			b.Fatalf("%s: no function", file)
		}
		off += strings.IndexByte(text[off:], '(')
	} else {
		off += len("fun count")
	}
	off++ // after '('
	if off >= len(text) || text[off] == '(' {
		b.Fatalf("%s: no parameter list", file)
	}
	insert := "kcBench: Int = 0"
	if text[off] != ')' {
		insert += ", "
	}
	c := startServer(b, root)
	defer c.close()
	u := fileURI(file)
	c.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, LanguageID: "kotlin", Version: 1, Text: text}})
	c.settle()
	line := strings.Count(text[:off], "\n")
	p := protocol.Position{Line: line, Character: off - (strings.LastIndex(text[:off], "\n") + 1)} // ASCII
	end := protocol.Position{Line: line, Character: p.Character + len(insert)}
	var times []time.Duration
	v := 1
	for b.Loop() {
		for _, change := range []protocol.TextDocumentContentChangeEvent{
			{Range: &protocol.Range{Start: p, End: p}, Text: insert},
			{Range: &protocol.Range{Start: p, End: end}, Text: ""},
		} {
			v++
			start := time.Now()
			c.client.Notify("textDocument/didChange", protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{URI: u, Version: v}, ContentChanges: []protocol.TextDocumentContentChangeEvent{change}})
			c.settle()
			times = append(times, time.Since(start))
		}
	}
	slices.Sort(times)
	b.ReportMetric(float64(times[len(times)/2].Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(times[len(times)*95/100].Microseconds())/1000, "p95-ms")
}

// BenchmarkNavigation times hover, definition, references and rename on
// a callee in the file (docs/lsp.md, L5: references and rename under
// 300 ms), and formatting the file: the median and 95th percentile of the requests on
// an updated module, and first-ms, the first request after an update,
// which builds the view's index of uses.
func BenchmarkNavigation(b *testing.B) {
	root, file, at := benchWorkspace(b)
	data, _ := os.ReadFile(file)
	text := string(data)
	off := strings.LastIndex(text, "source.items().size")
	if off >= 0 {
		off += len("source.")
	} else {
		// The first name on the edited line.
		off = at + strings.IndexFunc(text[at:], func(r rune) bool { return r != ' ' && r != '\t' })
	}
	line := strings.Count(text[:off], "\n")
	char := off - (strings.LastIndex(text[:off], "\n") + 1) // ASCII
	u := fileURI(file)
	at2 := protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}, Position: protocol.Position{Line: line, Character: char}}
	for _, c := range []struct {
		name, method string
		params       any
	}{
		{"hover", "textDocument/hover", at2},
		{"definition", "textDocument/definition", at2},
		{"references", "textDocument/references", protocol.ReferenceParams{TextDocumentPositionParams: at2, Context: protocol.ReferenceContext{IncludeDeclaration: true}}},
		{"rename", "textDocument/rename", protocol.RenameParams{TextDocumentPositionParams: at2, NewName: "renamed"}},
		{"formatting", "textDocument/formatting", protocol.DocumentFormattingParams{TextDocument: at2.TextDocument, Options: protocol.FormattingOptions{TabSize: 4, InsertSpaces: true}}},
	} {
		b.Run(c.name, func(b *testing.B) {
			s := startServer(b, root)
			defer s.close()
			s.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{URI: u, LanguageID: "kotlin", Version: 1, Text: text}})
			s.settle()
			call := func() time.Duration {
				start := time.Now()
				var res any
				if err := s.client.Call(context.Background(), c.method, c.params, &res); err != nil {
					b.Fatal(err)
				}
				if res == nil {
					b.Fatalf("%s found nothing", c.method)
				}
				return time.Since(start)
			}
			first := call()
			var times []time.Duration
			for b.Loop() {
				times = append(times, call())
			}
			slices.Sort(times)
			b.ReportMetric(float64(first.Microseconds())/1000, "first-ms")
			b.ReportMetric(float64(times[len(times)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(times[len(times)*95/100].Microseconds())/1000, "p95-ms")
		})
	}
}

// BenchmarkCompletion times completion in a function body (docs/lsp.md,
// L4: p95 under 100 ms): after a dot, at a statement's start with a
// prefix, which also lists the class path's importables, and in a type.
func BenchmarkCompletion(b *testing.B) {
	root, file, at := benchWorkspace(b)
	data, _ := os.ReadFile(file)
	text := string(data)
	u := fileURI(file)
	for _, c := range []struct{ name, insert string }{
		{"dot", "    items."},
		{"statement", "    su"},
		{"type", "    val x: Ma"},
	} {
		b.Run(c.name, func(b *testing.B) {
			s := startServer(b, root)
			defer s.close()
			edited := text[:at] + c.insert + "\n" + text[at:]
			s.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{URI: u, LanguageID: "kotlin", Version: 1, Text: edited}})
			s.settle()
			end := at + len(c.insert)
			pos := protocol.Position{Line: strings.Count(edited[:end], "\n"), Character: len(c.insert)} // ASCII
			params := protocol.CompletionParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u}, Position: pos}}
			call := func() time.Duration {
				start := time.Now()
				var res protocol.CompletionList
				if err := s.client.Call(context.Background(), "textDocument/completion", params, &res); err != nil {
					b.Fatal(err)
				}
				if os.Getenv("KC_LSP_BENCH") == "" && len(res.Items) == 0 {
					b.Fatal("no completion items")
				}
				return time.Since(start)
			}
			first := call()
			var times []time.Duration
			for b.Loop() {
				times = append(times, call())
			}
			slices.Sort(times)
			b.ReportMetric(float64(first.Microseconds())/1000, "first-ms")
			b.ReportMetric(float64(times[len(times)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(times[len(times)*95/100].Microseconds())/1000, "p95-ms")
		})
	}
}

// BenchmarkWorkspaceSymbol times workspace/symbol, whose results are the
// sources' and then the library's: a query few names of the sources
// match, which the class path's importables fill, and a source class's.
func BenchmarkWorkspaceSymbol(b *testing.B) {
	root, file, _ := benchWorkspace(b)
	data, _ := os.ReadFile(file)
	u := fileURI(file)
	for _, q := range []string{"HashMap", "Item"} {
		b.Run(q, func(b *testing.B) {
			s := startServer(b, root)
			defer s.close()
			s.client.Notify("textDocument/didOpen", protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{URI: u, LanguageID: "kotlin", Version: 1, Text: string(data)}})
			s.settle()
			call := func() time.Duration {
				start := time.Now()
				var res []json.RawMessage
				if err := s.client.Call(context.Background(), "workspace/symbol", protocol.WorkspaceSymbolParams{Query: q}, &res); err != nil {
					b.Fatal(err)
				}
				if len(res) == 0 {
					b.Fatalf("no symbols for %q", q)
				}
				return time.Since(start)
			}
			first := call()
			var times []time.Duration
			for b.Loop() {
				times = append(times, call())
			}
			slices.Sort(times)
			b.ReportMetric(float64(first.Microseconds())/1000, "first-ms")
			b.ReportMetric(float64(times[len(times)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(times[len(times)*95/100].Microseconds())/1000, "p95-ms")
		})
	}
}
