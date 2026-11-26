package lsp

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type client struct {
	t         *testing.T
	req       *io.PipeWriter
	resp      *bufio.Reader
	id        int
	published struct {
		URI         string
		Version     int
		Diagnostics json.RawMessage
	}
}

func start(t *testing.T, root string) *client {
	t.Helper()
	return startClient(t, root, completionOptions{}, false)
}
func startClient(t *testing.T, root string, opts completionOptions, definitionLinks bool) *client {
	t.Helper()
	return startClientEdits(t, root, opts, definitionLinks, true)
}
func startClientEdits(t *testing.T, root string, opts completionOptions, definitionLinks, versionEdits bool) *client {
	return startWithCapabilities(t, root, object{"textDocument": object{"definition": object{"linkSupport": definitionLinks}, "completion": object{"completionItem": opts}, "signatureHelp": object{"signatureInformation": object{"parameterInformation": object{"labelOffsetSupport": true}}}}, "workspace": object{"workspaceEdit": object{"documentChanges": versionEdits}}})
}

func startWithCapabilities(t *testing.T, root string, caps object) *client {
	t.Helper()
	r, w := io.Pipe()
	rr, ww := io.Pipe()
	done := make(chan error, 1)
	go func() { err := Serve(r, ww); ww.Close(); done <- err }()
	c := &client{t: t, req: w, resp: bufio.NewReader(rr)}
	c.call("initialize", object{"rootUri": uri(root), "capabilities": caps})
	c.notify("initialized", object{})
	t.Cleanup(func() {
		if !t.Failed() {
			c.call("shutdown", nil)
			c.notify("exit", nil)
		}
		w.Close()
		rr.Close()
		if err := <-done; err != nil && !t.Failed() {
			t.Error(err)
		}
	})
	return c
}
func (c *client) write(m object) {
	c.t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err = fmt.Fprintf(c.req, "Content-Length: %d\r\n\r\n%s", len(b), b); err != nil {
		c.t.Fatal(err)
	}
}
func (c *client) notify(m string, p any) { c.write(object{"jsonrpc": "2.0", "method": m, "params": p}) }
func (c *client) call(m string, p any) json.RawMessage {
	c.t.Helper()
	result, err := c.request(m, p)
	if err != nil {
		c.t.Fatalf("%s: %v", m, err)
	}
	return result
}
func (c *client) request(m string, p any) (json.RawMessage, *rpcError) {
	c.t.Helper()
	c.id++
	c.write(object{"jsonrpc": "2.0", "id": c.id, "method": m, "params": p})
	for {
		b, err := frame(c.resp)
		if err != nil {
			c.t.Fatal(err)
		}
		var r struct {
			ID     int
			Error  *rpcError
			Result json.RawMessage
			Method string
			Params json.RawMessage
		}
		if err = json.Unmarshal(b, &r); err != nil {
			c.t.Fatal(err)
		}
		if r.Method == "textDocument/publishDiagnostics" {
			if err := json.Unmarshal(r.Params, &c.published); err != nil {
				c.t.Fatal(err)
			}
		}
		if r.ID != c.id {
			continue
		}
		return r.Result, r.Error
	}
}
func params(path string, pos position) object {
	return object{"textDocument": object{"uri": uri(path)}, "position": pos}
}
func put(t testing.TB, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
func input(t testing.TB, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func checkJSON(t *testing.T, a, b []byte) {
	t.Helper()
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil || !reflect.DeepEqual(x, y) {
		t.Fatalf("got %s\nwant %s", a, b)
	}
}

func TestUpstreamReplay(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	path := filepath.Join(root, "Example.kt")
	put(t, path, string(input(t, "testdata/Example.kt")))
	var ref map[string]json.RawMessage
	if err := json.Unmarshal(input(t, "testdata/upstream-271.2085.0.json"), &ref); err != nil {
		t.Fatal(err)
	}
	c := start(t, root)
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/foldingRange", "textDocument/definition", "textDocument/hover", "textDocument/diagnostic"} {
		got := c.call(method, params(path, position{7, 18}))
		want := bytes.ReplaceAll(ref[method], []byte("file:///workspace/Example.kt"), []byte(uri(path)))
		checkJSON(t, got, want)
	}
	// Compare all signature records, including parameter label ranges in UTF-16.
	// kc also returns a top-level activeParameter for older clients.
	type signatureResult struct {
		ActiveSignature int
		Signatures      []object
	}
	var got, want signatureResult
	if err := json.Unmarshal(c.call("textDocument/signatureHelp", params(path, position{7, 25})), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(ref["textDocument/signatureHelp"], &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("signature help %+v, want %+v", got, want)
	}
}
func TestMultiModuleEditing(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	lib := filepath.Join(root, "lib", "Box.kt")
	app := filepath.Join(root, "app", "App.kt")
	put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"lib","sourceRoots":["lib"]},{"name":"app","sourceRoots":["app"],"dependencies":["lib"]}]}`)
	libSrc := "package lib\nclass Box { fun number(): Int = 1 }\n"
	put(t, lib, libSrc)
	source := "package app\nimport lib.Box\nfun read(box: Box) = box.number()\n"
	put(t, app, source)
	c := start(t, root)
	q := params(app, position{2, 26})
	hover := c.call("textDocument/hover", q)
	if !bytes.Contains(hover, []byte("Int")) {
		t.Fatalf("hover %s", hover)
	}
	c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(lib), "text": libSrc, "version": 1, "languageId": "kotlin"}})
	c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(lib), "version": 2}, "contentChanges": []change{{Text: strings.Replace(libSrc, "Int = 1", "String = \"hi\"", 1)}}})
	if h := c.call("textDocument/hover", q); !bytes.Contains(h, []byte("String")) {
		t.Fatalf("unsaved hover %s", h)
	}
	name := params(lib, position{1, 17})
	name["newName"] = "length"
	if b := c.call("textDocument/rename", name); !bytes.Contains(b, []byte(uri(app))) || !bytes.Contains(b, []byte(`"version":2`)) {
		t.Fatalf("rename %s", b)
	}
	broken := "package app\nimport lib.Box\nfun read(box: Box) { box. }\n"
	c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(app), "version": 1, "text": broken}})
	if b := c.call("textDocument/completion", params(app, position{2, 25})); !bytes.Contains(b, []byte(`"label":"number"`)) {
		t.Fatalf("incomplete member completion %s", b)
	}
}
func TestPositionsAndEditBatch(t *testing.T) {
	s := psi.ParseDocument("x.kt", "😀a\r\nβ\n")
	for _, test := range []struct {
		p   position
		off int
	}{{position{0, 2}, 4}, {position{0, 999}, 5}, {position{1, 1}, 9}, {position{2, 0}, 10}} {
		got, err := offset(s, test.p)
		if err != nil || got != test.off {
			t.Fatalf("offset %v: %d %v", test.p, got, err)
		}
	}
	version := 1
	w := workspace{docs: map[string]document{"x.kt": {s, &version}}}
	err := w.change("x.kt", 2, []change{{Range: &span{position{1, 0}, position{1, 1}}, Text: "z"}, {Range: &span{position{0, 1}, position{0, 2}}, Text: "x"}})
	if err == nil || w.docs["x.kt"].snap != s {
		t.Fatal("malformed batch must be atomic")
	}
}
func TestGradleAndMavenImport(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "settings.gradle.kts"), `include(":core")`)
	put(t, filepath.Join(root, "build.gradle.kts"), "dependencies {\n testImplementation(project(\":core\"))\n}\n")
	p, err := readProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.group(p.Modules[0])) != 1 || len(p.group(p.Modules[1])) != 3 {
		t.Fatalf("source-set scope: %+v", p.Modules)
	}
	root = t.TempDir()
	put(t, filepath.Join(root, "pom.xml"), `<project><groupId>dev</groupId><artifactId>root</artifactId><version>1</version><packaging>pom</packaging><modules><module>core</module><module>app</module></modules></project>`)
	for _, name := range []string{"core", "app"} {
		deps := ""
		if name == "app" {
			deps = `<dependencies><dependency><groupId>dev</groupId><artifactId>core</artifactId><version>1</version></dependency></dependencies>`
		}
		put(t, filepath.Join(root, name, "pom.xml"), `<project><parent><groupId>dev</groupId><artifactId>root</artifactId><version>1</version></parent><artifactId>`+name+`</artifactId>`+deps+`</project>`)
	}
	p, err = readProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Modules) != 4 || len(p.group(p.Modules[2])) != 2 {
		t.Fatalf("Maven project %v", p)
	}
}
func BenchmarkWarmHover(b *testing.B) {
	root := b.TempDir()
	p := filepath.Join(root, "Example.kt")
	put(b, p, string(input(b, "testdata/Example.kt")))
	w := workspace{roots: []string{root}}
	w.reload(context.Background())
	defer w.close()
	i, err := w.check(context.Background(), p)
	if err != nil {
		b.Fatal(err)
	}
	s := w.docs[p].snap
	n := s.NodeAt(strings.LastIndex(s.Source(), "answer"))
	for b.Loop() {
		i.hover(n)
	}
}

func TestEditingOperations(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	path := filepath.Join(root, "Main.kt")
	source := "package sample\ninterface Base { fun value(): Int }\nclass Impl : Base { override fun value(): Int = 7 }\nfun read(x: Impl) = x.value()\n"
	put(t, path, source)
	c := start(t, root)
	// Call, type and override relationships come from the checker.
	q := params(path, position{2, 33})
	prep := c.call("textDocument/prepareCallHierarchy", q)
	var items []object
	if err := json.Unmarshal(prep, &items); err != nil || len(items) != 1 {
		t.Fatalf("prepare call: %s", prep)
	}
	if got := c.call("callHierarchy/incomingCalls", object{"item": items[0]}); !bytes.Contains(got, []byte(`"name":"read"`)) {
		t.Fatalf("calls %s", got)
	}
	prep = c.call("textDocument/prepareTypeHierarchy", params(path, position{1, 11}))
	json.Unmarshal(prep, &items)
	if len(items) != 1 {
		t.Fatalf("prepare type: %s", prep)
	}
	if got := c.call("typeHierarchy/subtypes", object{"item": items[0]}); !bytes.Contains(got, []byte(`"name":"Impl"`)) {
		t.Fatalf("subtypes %s", got)
	}
	if got := c.call("textDocument/implementation", params(path, position{1, 21})); !bytes.Contains(got, []byte(`"line":2`)) {
		t.Fatalf("implementation %s", got)
	}
	q = params(path, position{})
	q["range"] = span{position{}, position{4, 0}}
	if got := c.call("textDocument/semanticTokens/range", q); bytes.Equal(got, []byte(`{"data":[]}`)) {
		t.Fatal("no highlighting")
	}
}
func TestFileMoveAndImportAlias(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src", "old", "Api.kt")
	user := filepath.Join(root, "src", "client", "Main.kt")
	put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"m","sourceRoots":["src"]}]}`)
	put(t, path, "package old\nfun answer() = 42\n")
	put(t, user, "package client\nimport old.answer as result\nfun read() = result()\n")
	c := start(t, root)
	q := params(path, position{1, 4})
	q["newName"] = "number"
	got := c.call("textDocument/rename", q)
	var rename struct {
		DocumentChanges []struct {
			TextDocument folder
			Edits        []edit
		}
	}
	json.Unmarshal(got, &rename)
	if len(rename.DocumentChanges) != 2 {
		t.Fatalf("rename %s", got)
	}
	for _, d := range rename.DocumentChanges {
		if d.TextDocument.URI == uri(user) && (len(d.Edits) != 1 || d.Edits[0].Range.Start.Line != 1) {
			t.Fatalf("alias %s", got)
		}
	}
	got = c.call("workspace/willRenameFiles", object{"files": []fileMove{{uri(path), uri(filepath.Join(root, "src", "next", "Api.kt"))}}})
	if err := json.Unmarshal(got, &rename); err != nil {
		t.Fatal(err)
	}
	after := map[string]string{}
	for _, d := range rename.DocumentChanges {
		p, err := filePath(d.TextDocument.URI)
		if err != nil {
			t.Fatal(err)
		}
		after[p] = applyText(t, string(input(t, p)), d.Edits)
	}
	if !strings.Contains(after[user], "import next.answer as result") || !strings.Contains(after[path], "package next") {
		t.Fatalf("file move %s", got)
	}
}
func TestNativeClasspathAndDiagnostics(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("KOTLIN_HOME", "")
	t.Setenv("JAVA_HOME", "")
	root := t.TempDir()
	path := filepath.Join(root, "app", "Main.kt")
	java := filepath.Join(root, "app", "Greeter.java")
	put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"app","sourceRoots":["app"],"dependencies":["lib"]},{"name":"lib","sourceRoots":["lib"]}]}`)
	put(t, filepath.Join(root, "lib", "Secret.kt"), "package demo\ninternal fun secret() = 1\n")
	put(t, java, "package demo;\npublic class Greeter { public int number() { return 1; } }\n")
	put(t, path, "package demo\nfun use(g: Greeter): Int = g.number() + secret()\n")
	c := startClient(t, root, completionOptions{}, true)
	got := c.call("textDocument/definition", params(path, position{1, 30}))
	target := span{position{1, 34}, position{1, 40}}
	want, _ := json.Marshal([]object{{"targetUri": uri(java), "targetRange": target, "targetSelectionRange": target,
		"originSelectionRange": span{position{1, 29}, position{1, 35}}}})
	checkJSON(t, got, want)
	if got := c.call("textDocument/diagnostic", params(path, position{})); !bytes.Contains(got, []byte("INVISIBLE_REFERENCE")) {
		t.Fatalf("module visibility %s", got)
	}
}
func TestFormattingAndOrganize(t *testing.T) {
	src := "package p\r\nimport z.Z\r\nimport a.A\r\nfun x(){\r\nval s=\"\"\"a\r\n  b\"\"\"\r\n}\r\n"
	s := psi.ParseDocument("x.kt", src)
	edits := organize(context.Background(), s, nil)
	if len(edits) != 1 || edits[0].NewText != "import a.A\r\nimport z.Z" {
		t.Fatal(edits)
	}
	var p parameters
	p.Options.InsertSpaces = true
	p.Options.TabSize = 2
	apply := func(s *psi.Snapshot, es []edit) string {
		v := s.Source()
		for j := len(es) - 1; j >= 0; j-- {
			a, _ := offset(s, es[j].Range.Start)
			b, _ := offset(s, es[j].Range.End)
			v = v[:a] + es[j].NewText + v[b:]
		}
		return v
	}
	formatted := apply(s, format(s, p, false))
	if !strings.Contains(formatted, "\r\n  val s = \"\"\"a\r\n  b\"\"\"") {
		t.Fatalf("formatted %q", formatted)
	}
	if edits := format(psi.ParseDocument("x.kt", formatted), p, false); len(edits) != 0 {
		t.Fatal("formatting is not idempotent", edits)
	}
}

func TestCachedDependencyResolution(t *testing.T) {
	t.Setenv("KC_LSP_OFFLINE", "1")
	cache := t.TempDir()
	t.Setenv("MAVEN_REPO_LOCAL", cache)
	t.Setenv("GRADLE_USER_HOME", t.TempDir())
	for _, v := range []struct{ name, version, deps string }{{"a", "1", `<dependencies><dependency><groupId>org.test</groupId><artifactId>b</artifactId><version>1</version></dependency></dependencies>`}, {"b", "1", ""}, {"b", "2", ""}} {
		base := filepath.Join(cache, "org", "test", v.name, v.version, v.name+"-"+v.version)
		put(t, base+".jar", "")
		put(t, base+".pom", "<project><groupId>org.test</groupId><artifactId>"+v.name+"</artifactId><version>"+v.version+"</version>"+v.deps+"</project>")
	}
	root := t.TempDir()
	put(t, filepath.Join(root, "pom.xml"), `<project><groupId>demo</groupId><artifactId>app</artifactId><version>1</version><dependencyManagement><dependencies><dependency><groupId>org.test</groupId><artifactId>b</artifactId><version>2</version></dependency></dependencies></dependencyManagement><dependencies><dependency><groupId>org.test</groupId><artifactId>a</artifactId><version>1</version></dependency></dependencies></project>`)
	p, err := readProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cp := p.Modules[0].ClassPath; len(cp) != 2 || filepath.Base(cp[1]) != "b-2.jar" {
		t.Fatal(cp)
	}
	root = t.TempDir()
	put(t, filepath.Join(root, "gradle", "libs.versions.toml"), "[versions]\nfixture = \"1\"\n[libraries]\ntest-lib = { module = \"org.test:a\", version.ref = \"fixture\" }\n")
	put(t, filepath.Join(root, "build.gradle.kts"), "plugins { kotlin(\"jvm\") }\ndependencies { implementation(libs.test.lib) }\n")
	p, err = readProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cp := p.Modules[0].ClassPath; len(cp) != 2 || filepath.Base(cp[1]) != "b-1.jar" {
		t.Fatal(cp)
	}
	put(t, filepath.Join(root, "build.gradle.kts"), "plugins { id(\"custom.conventions\") }")
	if _, err := readProject(context.Background(), root); err == nil {
		t.Fatal("custom build imported as complete")
	}
}

func TestActionsAndRecovery(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	path := filepath.Join(root, "Test.kt")
	src := "class Box { fun method() = 42 }\nfun make() = Box()\nfun read() { make(). }\nfun exact(): Int {\n    val n = 42\n    return n\n}\n"
	put(t, path, src)
	c := start(t, root)
	if got := c.call("textDocument/completion", params(path, position{2, 20})); !bytes.Contains(got, []byte(`"label":"method"`)) {
		t.Fatalf("chained receiver %s", got)
	}
	p := params(path, position{})
	p["range"] = span{position{4, 8}, position{4, 9}}
	p["context"] = object{"only": []string{"refactor.inline.variable"}}
	if got := c.call("textDocument/codeAction", p); !bytes.Contains(got, []byte(`"newText":"42"`)) {
		t.Fatalf("inline %s", got)
	}
	src = "fun f(s: String) = s!!.length\n"
	c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "version": 1, "text": src}})
	p["range"] = span{position{0, 0}, position{0, 29}}
	p["context"] = object{"only": []string{"quickfix"}}
	if got := c.call("textDocument/codeAction", p); !bytes.Contains(got, []byte("Remove unnecessary !!")) {
		t.Fatalf("assertion fix %s", got)
	}
}

func TestBinaryDependencyAndAttachedSource(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	lib := filepath.Join(t.TempDir(), "Api.kt")
	source := "package library\nclass Api { fun message(): Int = 42\nfun choose(x: Int) = x\nfun choose(x: String) = x }\n"
	put(t, lib, source)
	classes := filepath.Join(root, "classes")
	if err := driver.Compile([]string{lib}, driver.Options{NoJDK: true, OutDir: classes, ModuleName: "dependency"}); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "sources.jar")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	entry, err := z.Create("library/Api.kt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(entry, source); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "src", "Use.kt")
	put(t, path, "import library.Api\nfun read(x: Api) = x.message()\nfun other(x: Api) = x.choose(1)\n")
	put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"app","sourceRoots":["src"],"classPath":["classes"],"sources":["sources.jar"]}]}`)
	c := start(t, root)
	q := params(path, position{1, 23})
	b := c.call("textDocument/definition", q)
	var l location
	if err := json.Unmarshal(b, &l); err != nil || l.URI == "" {
		t.Fatalf("dependency definition %s", b)
	}
	p, _ := filePath(l.URI)
	if string(input(t, p)) != source || l.Range.Start != (position{1, 16}) {
		t.Fatalf("attachment %s", b)
	}
	if got := c.call("workspace/symbol", object{"query": "message"}); !bytes.Contains(got, []byte(l.URI)) {
		t.Fatalf("library symbols %s", got)
	}
	if got := c.call("textDocument/completion", params(path, position{1, 22})); !bytes.Contains(got, []byte(`"label":"message"`)) {
		t.Fatalf("library completion %s", got)
	}
	b = c.call("textDocument/definition", params(path, position{2, 24}))
	if err := json.Unmarshal(b, &l); err != nil || l.Range.Start != (position{2, 4}) {
		t.Fatalf("overloaded attached definition %s", b)
	}
}

func TestGradleConsumerSeesOnlyExportedDependencies(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	put(t, filepath.Join(root, "settings.gradle.kts"), `include(":app", ":library", ":detail")`)
	put(t, filepath.Join(root, "app", "build.gradle.kts"), `dependencies { implementation(project(":library")) }`)
	build := filepath.Join(root, "library", "build.gradle.kts")
	put(t, build, `dependencies { implementation(project(":detail")) }`)
	put(t, filepath.Join(root, "detail", "src", "main", "kotlin", "Detail.kt"), "package detail\nfun fetch() = 42\n")
	app := filepath.Join(root, "app", "src", "main", "kotlin", "App.kt")
	put(t, app, "package app\nimport detail.fetch\nfun use() = fetch()\n")
	c := start(t, root)
	q := params(app, position{})
	if got := c.call("textDocument/diagnostic", q); !bytes.Contains(got, []byte("INVISIBLE_REFERENCE")) {
		t.Fatalf("leaked implementation dependency %s", got)
	}
	put(t, build, `dependencies { api(project(":detail")) }`)
	c.notify("workspace/didChangeWatchedFiles", object{"changes": []object{{"uri": uri(build), "type": 2}}})
	if got := c.call("textDocument/diagnostic", q); bytes.Contains(got, []byte("INVISIBLE_REFERENCE")) {
		t.Fatalf("api dependency hidden %s", got)
	}
}

func TestOverrideRenameAppliesInConsumers(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	files := map[string]string{"base/Base.kt": "package sample\ninterface Base { fun count(): Int }\n", "left/Left.kt": "package sample\nclass Left : Base { override fun count() = 1 }\nfun left(b: Left) = b.count()\n", "right/Right.kt": "package sample\nclass Right : Base { override fun count() = 2 }\nfun right(b: Base) = b.count()\nfun unrelated(count: String) = count\n"}
	for f, src := range files {
		put(t, filepath.Join(root, f), src)
	}
	put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"base","sourceRoots":["base"]},{"name":"left","sourceRoots":["left"],"dependencies":["base"]},{"name":"right","sourceRoots":["right"],"dependencies":["base"]}]}`)
	c := start(t, root)
	q := params(filepath.Join(root, "left/Left.kt"), position{1, 35})
	q["newName"] = "total"
	got := c.call("textDocument/rename", q)
	var changes struct {
		DocumentChanges []struct {
			TextDocument folder
			Edits        []edit
		}
	}
	if err := json.Unmarshal(got, &changes); err != nil || len(changes.DocumentChanges) != 3 {
		t.Fatalf("family edits %s", got)
	}
	for _, file := range changes.DocumentChanges {
		path, _ := filePath(file.TextDocument.URI)
		src := string(input(t, path))
		snap := psi.ParseDocument(path, src)
		for j := len(file.Edits) - 1; j >= 0; j-- {
			e := file.Edits[j]
			a, _ := offset(snap, e.Range.Start)
			b, _ := offset(snap, e.Range.End)
			src = src[:a] + e.NewText + src[b:]
		}
		put(t, path, src)
	}
	right := filepath.Join(root, "right/Right.kt")
	src := string(input(t, right))
	if !strings.Contains(src, "override fun total()") || !strings.Contains(src, "b.total()") || !strings.Contains(src, "unrelated(count: String) = count") {
		t.Fatal(src)
	}
	c.notify("workspace/didChangeWatchedFiles", object{})
	if got := c.call("textDocument/diagnostic", params(right, position{})); bytes.Contains(got, []byte(`"severity":1`)) {
		t.Fatalf("renamed source %s", got)
	}
}

func TestImportedProjectReplay(t *testing.T) {
	for _, build := range []string{"maven", "gradle"} {
		t.Run(build, func(t *testing.T) { replayImportedProject(t, build) })
	}
}
func replayImportedProject(t *testing.T, build string) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	src := filepath.Join(root, "src", "main", "kotlin")
	paths := []string{filepath.Join(src, "Example.kt"), filepath.Join(src, "Consumer.kt")}
	t.Setenv("KC_LSP_OFFLINE", "1")
	if build == "gradle" {
		paths[0] = filepath.Join(root, "library", "src", "main", "kotlin", "Example.kt")
		for _, file := range []string{"build.gradle.kts", "settings.gradle.kts", "gradle.properties", "library/build.gradle.kts"} {
			put(t, filepath.Join(root, file), string(input(t, filepath.Join("testdata", build, file))))
		}
	} else {
		put(t, filepath.Join(root, "pom.xml"), string(input(t, "testdata/maven/pom.xml")))
	}
	put(t, paths[0], string(input(t, "testdata/Example.kt")))
	put(t, paths[1], string(input(t, "testdata/Consumer.kt")))
	raw := input(t, filepath.Join("testdata", build, "upstream-271.2085.0.json"))
	for _, p := range paths {
		raw = bytes.ReplaceAll(raw, []byte("file:///workspace/"+filepath.Base(p)), []byte(uri(p)))
	}
	var ref map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	c := start(t, root)
	for _, p := range paths {
		c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(p), "version": 1, "languageId": "kotlin", "text": string(input(t, p))}})
	}
	use := params(paths[1], position{2, 25})
	for _, m := range []string{"definition", "hover"} {
		checkJSON(t, c.call("textDocument/"+m, use), ref[m])
	}
	decl := params(paths[0], position{3, 5})
	decl["context"] = object{"includeDeclaration": true}
	normalize := func(b json.RawMessage) []string {
		var ls []location
		if err := json.Unmarshal(b, &ls); err != nil {
			t.Fatal(err)
		}
		var result []string
		for _, l := range ls {
			result = append(result, fmt.Sprint(l))
		}
		slices.Sort(result)
		return result
	}
	if got, want := normalize(c.call("textDocument/references", decl)), normalize(ref["references"]); !slices.Equal(got, want) {
		t.Fatalf("references %v, want %v", got, want)
	}
	// The reference splits some replacements into insert/delete pairs. Apply
	// both edit lists at their advertised versions and compare every file.
	applied := func(b json.RawMessage) map[string]string {
		var we struct {
			DocumentChanges []struct {
				TextDocument struct {
					URI     string
					Version int
				}
				Edits []edit
			}
		}
		if err := json.Unmarshal(b, &we); err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, d := range we.DocumentChanges {
			path, err := filePath(d.TextDocument.URI)
			if err != nil || d.TextDocument.Version != 1 {
				t.Fatalf("unexpected edit target: %+v", d.TextDocument)
			}
			text := string(input(t, path))
			snap := psi.ParseDocument(path, text)
			slices.SortStableFunc(d.Edits, func(a, b edit) int {
				if before(a.Range.Start, b.Range.Start) {
					return 1
				}
				if before(b.Range.Start, a.Range.Start) {
					return -1
				}
				return 0
			})
			for _, e := range d.Edits {
				a, err := offset(snap, e.Range.Start)
				if err != nil {
					t.Fatal(err)
				}
				b, err := offset(snap, e.Range.End)
				if err != nil {
					t.Fatal(err)
				}
				text = text[:a] + e.NewText + text[b:]
			}
			out[path] = text
		}
		return out
	}
	decl["newName"] = "resultOf"
	want := applied(ref["rename"])
	got := applied(c.call("textDocument/rename", decl))
	if len(want) != 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("applied rename %v, want %v", got, want)
	}
}

func TestIncompleteSignatureAndNamedArguments(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	path := filepath.Join(root, "Calls.kt")
	src := "fun choose(count: Int, label: String): String = label\nfun choose(name: String, n: Int): Int = n\nfun read() = choose(1, "
	put(t, path, src)
	c := start(t, root)
	got := c.call("textDocument/signatureHelp", params(path, position{2, 23}))
	var res struct {
		ActiveParameter int
		Signatures      []struct{ Label string }
	}
	if err := json.Unmarshal(got, &res); err != nil || len(res.Signatures) != 2 || res.ActiveParameter != 1 {
		t.Fatalf("incomplete call %s", got)
	}
	text := "fun choose(count: Int, label: String): String = label\nfun read() = choose(label = \"hi\", count = 42)\n"
	c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "version": 1, "text": text}})
	p := params(path, position{0, 13})
	p["newName"] = "amount"
	edits := c.call("textDocument/rename", p)
	if bytes.Count(edits, []byte(`"newText":"amount"`)) != 2 {
		t.Fatalf("named argument rename %s", edits)
	}
	got = c.call("textDocument/semanticTokens/full", params(path, position{}))
	var buf struct{ Data []int }
	json.Unmarshal(got, &buf)
	l, ch := 0, 0
	named := 0
	for j := 0; j < len(buf.Data); j += 5 {
		if buf.Data[j] != 0 {
			ch = 0
		}
		l += buf.Data[j]
		ch += buf.Data[j+1]
		if buf.Data[j+4]&(1<<10) != 0 {
			if l != 1 || !(ch == 20 || ch == 34) {
				t.Fatalf("named argument starts at %d:%d", l, ch)
			}
			named++
		}
	}
	if named != 2 {
		t.Fatalf("named tokens: %d", named)
	}
}

func TestInlayHintsFollowSettingsAndRange(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	t.Setenv("PATH", "")
	root := t.TempDir()
	path := filepath.Join(root, "Hints.kt")
	put(t, path, "val top = 1\nfun twice(count: Int) = count + count\nfun use() { val local = twice(2) }\n")
	c := start(t, root)
	q := params(path, position{})
	q["range"] = span{position{}, position{3, 0}}
	read := func() []struct {
		Position position
		Label    string
	} {
		t.Helper()
		var result []struct {
			Position position
			Label    string
		}
		if err := json.Unmarshal(c.call("textDocument/inlayHint", q), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	initial := read()
	if len(initial) != 4 || initial[1].Position != (position{1, 21}) || initial[1].Label != ": Int" || initial[3].Label != "count:" {
		t.Fatalf("default hints: %+v", initial)
	}
	c.notify("workspace/didChangeConfiguration", object{"settings": object{
		"jetbrains.kotlin.hints":                      object{"settings.types.variable": false, "parameters": false},
		"jetbrains.kotlin.hints.type.function.return": false,
	}})
	remaining := read()
	if len(remaining) != 1 || remaining[0].Position.Line != 0 {
		t.Fatalf("configured hints: %+v", remaining)
	}
	c.notify("workspace/didChangeConfiguration", object{"settings": object{}})
	q["range"] = span{position{2, 28}, position{2, 32}}
	if hits := read(); len(hits) != 1 || hits[0].Label != "count:" {
		t.Fatalf("range hints: %+v", hits)
	}
}

func TestGradleKotlinPluginCatalogAndDefaultStdlib(t *testing.T) {
	t.Setenv("KC_LSP_OFFLINE", "1")
	cache := t.TempDir()
	t.Setenv("MAVEN_REPO_LOCAL", cache)
	t.Setenv("GRADLE_USER_HOME", t.TempDir())
	t.Setenv("KC_LSP_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	put(t, filepath.Join(root, "gradle", "libs.versions.toml"), "[versions]\nkotlin = \"2.4.0\"\n[plugins]\nkotlin-jvm = { id = \"org.jetbrains.kotlin.jvm\", version.ref = \"kotlin\" }")
	put(t, filepath.Join(root, "settings.gradle.kts"), "include(\n  \":app\",\n)\n")
	put(t, filepath.Join(root, "build.gradle.kts"), "plugins { alias(libs.plugins.kotlin.jvm) apply false }")
	put(t, filepath.Join(root, "app", "build.gradle.kts"), "plugins { kotlin(\"jvm\") }")
	base := filepath.Join(cache, "org", "jetbrains", "kotlin", "kotlin-stdlib", "2.4.0", "kotlin-stdlib-2.4.0")
	put(t, base+".jar", "")
	put(t, base+".pom", "<project><groupId>org.jetbrains.kotlin</groupId><artifactId>kotlin-stdlib</artifactId><version>2.4.0</version></project>")
	for _, disabled := range []bool{false, true} {
		if disabled {
			put(t, filepath.Join(root, "gradle.properties"), "kotlin.stdlib.default.dependency=false\n")
		}
		p, err := readProject(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Modules) != 4 || len(p.Modules[0].ClassPath) != 0 {
			t.Fatalf("root: %+v", p.Modules)
		}
		for _, target := range p.Modules[2:] {
			if slices.Contains(target.ClassPath, base+".jar") == disabled {
				t.Fatalf("disabled %v: %s %v", disabled, target.Name, target.ClassPath)
			}
		}
	}
}
func TestCompletionImportsAndAliasedNames(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	t.Setenv("PATH", "")
	root := t.TempDir()
	put(t, filepath.Join(root, "Api.kt"), "package lib\nclass Widget\n")
	path := filepath.Join(root, "Use.kt")
	source := "package app\nfun read() = Wid\n"
	put(t, path, source)
	c := start(t, root)
	q := params(path, position{1, 16})
	var items []struct {
		Label               string
		AdditionalTextEdits []edit
	}
	if err := json.Unmarshal(c.call("textDocument/completion", q), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Label != "Widget" || len(items[0].AdditionalTextEdits) != 1 {
		t.Fatalf("auto-import: %+v", items)
	}
	imp := items[0].AdditionalTextEdits[0]
	s := psi.ParseDocument(path, source)
	a, _ := offset(s, imp.Range.Start)
	b, _ := offset(s, imp.Range.End)
	edited := strings.Replace(source[:a]+imp.NewText+source[b:], "= Wid", "= Widget()", 1)
	c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "text": edited, "version": 1}})
	if got := c.call("textDocument/diagnostic", q); !bytes.Contains(got, []byte(`"items":[]`)) {
		t.Fatalf("after applying completion %s: %s", edited, got)
	}
	alias := "package app\nimport lib.Widget as Gadget\nfun read() = Gad\n"
	c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 2}, "contentChanges": []change{{Text: alias}}})
	got := c.call("textDocument/completion", params(path, position{2, 16}))
	if !bytes.Contains(got, []byte(`"label":"Gadget"`)) || bytes.Contains(got, []byte("additionalTextEdits")) {
		t.Fatalf("aliased completion %s", got)
	}
}
