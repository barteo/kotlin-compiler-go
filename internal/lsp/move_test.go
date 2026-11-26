package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

type moveFixture struct {
	Moves map[string]string
	Files map[string]string
}

const moveModel = `{"version":1,"modules":[{"name":"app","sourceRoots":["src/main/kotlin"],"dependencies":["lib"]},{"name":"lib","sourceRoots":["library/src/main/kotlin"]}]}`

func moveParams(root string, moves map[string]string) object {
	var files []fileMove
	for old, dest := range moves {
		files = append(files, fileMove{uri(filepath.Join(root, old)), uri(filepath.Join(root, dest))})
	}
	return object{"files": files}
}

func moveEdits(t *testing.T, raw json.RawMessage, versioned bool) map[string][]edit {
	t.Helper()
	var result struct {
		Changes         map[string][]edit
		DocumentChanges []struct {
			TextDocument folder
			Edits        []edit
		}
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if versioned {
		if result.Changes != nil || result.DocumentChanges == nil {
			t.Fatalf("expected documentChanges: %s", raw)
		}
		result.Changes = map[string][]edit{}
		for _, doc := range result.DocumentChanges {
			result.Changes[doc.TextDocument.URI] = doc.Edits
		}
	} else if result.Changes == nil || result.DocumentChanges != nil {
		t.Fatalf("expected changes: %s", raw)
	}
	return result.Changes
}

func TestPackageMoveReplay(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	t.Setenv("PATH", "")
	var cases map[string]moveFixture
	var ref struct{ Cases map[string]map[string]string }
	if err := json.Unmarshal(input(t, "testdata/moves/cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(input(t, "testdata/moves/upstream-271.2085.0.json"), &ref); err != nil {
		t.Fatal(err)
	}
	for name, tc := range cases {
		t.Run(name+"/changes", func(t *testing.T) { replayMove(t, name, tc, ref.Cases[name], false) })
		t.Run(name+"/documentChanges", func(t *testing.T) { replayMove(t, name, tc, ref.Cases[name], true) })
	}
}

func replayMove(t *testing.T, name string, tc moveFixture, expected map[string]string, versioned bool) {
	t.Helper()
	root := t.TempDir()
	put(t, filepath.Join(root, "kc-lsp.json"), moveModel)
	for p, text := range tc.Files {
		put(t, filepath.Join(root, p), text)
	}
	c := startClientEdits(t, root, completionOptions{}, false, versioned)
	params := moveParams(root, tc.Moves)
	got := moveEdits(t, c.call("workspace/willRenameFiles", params), versioned)
	for path, text := range tc.Files {
		p := filepath.Join(root, path)
		after := applyText(t, text, got[uri(p)])
		if want := expected[path]; after != want {
			t.Errorf("%s: got %q, want %q", path, after, want)
		}
		delete(got, uri(p))
		put(t, p, after)
	}
	if len(got) != 0 {
		t.Fatal("edits outside fixture", got)
	}
	for old, dest := range tc.Moves {
		if err := os.Rename(filepath.Join(root, old), filepath.Join(root, dest)); err != nil {
			t.Fatal(err)
		}
	}
	c.notify("workspace/didRenameFiles", params)
	path, use, decl := "src/main/kotlin/client/Folder.kt", "result()", "library/src/main/kotlin/fresh/nested/Child.kt"
	if name == "batch" {
		path, use, decl = "src/main/kotlin/client/Batch.kt", "first()", "library/src/main/kotlin/dest/One.kt"
	}
	checkMoveDefinition(t, c, filepath.Join(root, path), string(input(t, filepath.Join(root, path))), use, filepath.Join(root, decl))
	for old := range tc.Files {
		current := old
		for from, to := range tc.Moves {
			if within(old, from) {
				rel, err := filepath.Rel(from, old)
				if err != nil {
					t.Fatal(err)
				}
				current = filepath.Join(to, rel)
				break
			}
		}
		checkMoveDiagnostics(t, c, filepath.Join(root, current))
	}
}

func checkMoveDiagnostics(t *testing.T, c *client, path string) {
	t.Helper()
	var report struct {
		Items []struct {
			Severity int
			Message  string
		}
	}
	if err := json.Unmarshal(c.call("textDocument/diagnostic", params(path, position{})), &report); err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Items {
		if d.Severity == 1 {
			t.Errorf("%s: %s", path, d.Message)
		}
	}
}

func checkMoveDefinition(t *testing.T, c *client, path, text, use, target string) {
	t.Helper()
	at := strings.LastIndex(text, use)
	if at < 0 {
		t.Fatal("missing usage", use)
	}
	q := params(path, pos(psi.ParseDocument(path, text), at))
	var locs []location
	raw := c.call("textDocument/definition", q)
	if err := json.Unmarshal(raw, &locs); err != nil || len(locs) != 1 || locs[0].URI != uri(target) {
		t.Fatalf("%s %s: %s", path, use, raw)
	}
}

func TestPartialPackageMove(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "kc-lsp.json"), strings.Replace(moveModel, `"modules":[`, `"modules":[{"name":"other","javaRoots":["other"]},`, 1))
	put(t, filepath.Join(root, "other", "Unrelated.java"), "class Unrelated {}")
	lib := "library/src/main/kotlin/"
	a, b, keep := lib+"old/A.kt", lib+"old/B.kt", lib+"old/Keep.kt"
	use := "src/main/kotlin/client/Main.kt"
	files := map[string]string{
		a:    "package old\n\nfun a() = b() + keep()\n",
		b:    "package old\n\nfun b() = 2\n",
		keep: "package old\n\nfun keep() = 1\n",
		use:  "package client\n\nimport old.*\n\nfun read() = a() + keep()\n",
	}
	for p, text := range files {
		put(t, filepath.Join(root, p), text)
	}
	c := start(t, root)
	moves := map[string]string{a: lib + "fresh/A.kt", b: lib + "fresh/B.kt"}
	got := moveEdits(t, c.call("workspace/willRenameFiles", moveParams(root, moves)), true)
	after := func(p string) string { return applyText(t, files[p], got[uri(filepath.Join(root, p))]) }
	if !strings.Contains(after(a), "import old.keep") || strings.Contains(after(a), "import fresh.b") || !strings.Contains(after(use), "import old.*") || !strings.Contains(after(use), "import fresh.a") {
		t.Fatalf("%s\n%s", after(a), after(use))
	}
}

func TestMoveOpenBuffers(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "kc-lsp.json"), moveModel)
	base := "library/src/main/kotlin/"
	old := filepath.Join(root, base, "old")
	dest := filepath.Join(root, base, "fresh")
	a, b := filepath.Join(old, "A.kt"), filepath.Join(old, "nested", "B.kt")
	put(t, a, "package old\nfun saved() = 0\n")
	c := start(t, root)
	open := func(p, text string, version int) {
		c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(p), "version": version, "text": text}})
	}
	src := map[string]string{a: "package old\nfun unsaved() = 42\n", b: "package old.nested\nimport old.unsaved\nfun use() = unsaved()\n"}
	for p, text := range src {
		open(p, text, 8)
	}
	req := object{"files": []fileMove{{uri(old), uri(dest)}, {uri(b), uri(filepath.Join(dest, "nested", "B.kt"))}}}
	raw := c.call("workspace/willRenameFiles", req)
	var result struct {
		DocumentChanges []struct {
			TextDocument struct {
				URI     string
				Version int
			}
			Edits []edit
		}
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.DocumentChanges) != 2 {
		t.Fatalf("edit %s", raw)
	}
	for _, change := range result.DocumentChanges {
		if change.TextDocument.Version != 8 {
			t.Fatalf("unversioned open buffer: %s", raw)
		}
		p, _ := filePath(change.TextDocument.URI)
		src[p] = applyText(t, src[p], change.Edits)
		c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(p), "version": 9}, "contentChanges": []object{{"text": src[p]}}})
	}
	if err := os.Rename(old, dest); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didRenameFiles", req)
	checkMoveDefinition(t, c, filepath.Join(dest, "nested", "B.kt"), src[b], "unsaved()", filepath.Join(dest, "A.kt"))
	checkMoveDiagnostics(t, c, filepath.Join(dest, "nested", "B.kt"))
	if _, err := c.request("textDocument/documentSymbol", params(a, position{})); err == nil {
		t.Fatal("old URI still loaded")
	}
	// Some clients open the new URI before sending didRenameFiles.
	from, to := filepath.Join(dest, "A.kt"), filepath.Join(dest, "Renamed.kt")
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	open(to, "package fresh\nfun newer() = 3\n", 1)
	c.notify("workspace/didRenameFiles", object{"files": []fileMove{{uri(from), uri(to)}}})
	if raw := c.call("textDocument/documentSymbol", params(to, position{})); !strings.Contains(string(raw), "newer") {
		t.Fatalf("lost reopened buffer: %s", raw)
	}
}

func TestPackageMoveConflicts(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "kc-lsp.json"), moveModel)
	lib := "library/src/main/kotlin/"
	a, b := lib+"old/A.kt", lib+"old/B.kt"
	put(t, filepath.Join(root, a), "package old\nfun answer()=1\n")
	put(t, filepath.Join(root, b), "package old\nfun another()=2\n")
	put(t, filepath.Join(root, lib, "taken", "C.kt"), "package taken\nfun answer()=3\n")
	c := start(t, root)
	for _, moves := range []map[string]string{
		{a: lib + "new/A.kt", b: lib + "new/A.kt"},
		{a: b},
		{lib + "old": lib + "new", lib + "taken": lib + "new"},
		{lib + "old": lib + "new", a: lib + "elsewhere/A.kt"},
		{a: lib + "taken/A.kt", b: lib + "taken/B.kt"},
		{a: "src/main/kotlin/other/A.kt"},
	} {
		if result, err := c.request("workspace/willRenameFiles", moveParams(root, moves)); err == nil || err.Code != -32803 || len(result) != 0 {
			t.Fatalf("expected atomic conflict for %v: %s %v", moves, result, err)
		}
	}
	if raw := c.call("workspace/willRenameFiles", moveParams(root, map[string]string{a: lib + "old/Renamed.kt"})); string(raw) != "null" {
		t.Fatalf("same package: %s", raw)
	}
	if got := string(c.call("textDocument/documentSymbol", params(filepath.Join(root, a), position{}))); !strings.Contains(got, "answer") {
		t.Fatal(got)
	}
	put(t, filepath.Join(root, "src/main/kotlin/client/User.java"), "package client; class User { int n = old.AKt.answer(); }")
	c.notify("workspace/didChangeWatchedFiles", object{})
	if _, err := c.request("workspace/willRenameFiles", moveParams(root, map[string]string{a: lib + "fresh/A.kt"})); err == nil || err.Code != -32803 {
		t.Fatalf("move with a Java consumer: %v", err)
	}
}

func TestPackageMoveExtensionUses(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "kc-lsp.json"), moveModel)
	lib := "library/src/main/kotlin/"
	put(t, filepath.Join(root, lib, "old", "Box.kt"), "package old\nclass Box\n")
	put(t, filepath.Join(root, lib, "old", "Ext.kt"), "package old\nfun Box.value() = 1\noperator fun Box.plus(other: Box) = other\n")
	use := filepath.Join(root, "src/main/kotlin/client/Use.kt")
	src := "package client\n\nimport old.*\n\nfun use(old: Box) = (old + old).value() + old.value()\n"
	put(t, use, src)
	c := start(t, root)
	got := moveEdits(t, c.call("workspace/willRenameFiles", moveParams(root, map[string]string{lib + "old/Ext.kt": lib + "fresh/Ext.kt"})), true)
	want := strings.Replace(src, "import old.*", "import old.*\nimport fresh.plus\nimport fresh.value", 1)
	if after := applyText(t, src, got[uri(use)]); after != want {
		t.Fatalf("got %q, want %q", after, want)
	}
}

func TestMoveRegistrations(t *testing.T) {
	ops := capabilities()["workspace"].(object)["fileOperations"].(object)
	for _, method := range []string{"willRename", "didRename"} {
		var matches []string
		for _, f := range ops[method].(object)["filters"].([]object) {
			matches = append(matches, f["pattern"].(object)["matches"].(string))
		}
		if !slices.Contains(matches, "file") || !slices.Contains(matches, "folder") {
			t.Fatal(method, matches)
		}
	}
}

func TestMovedQualifiedUsesAndDeclaredPackage(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "kc-lsp.json"), moveModel)
	lib := "library/src/main/kotlin/"
	files := map[string]string{
		lib + "old/Api.kt":                    "package old\nclass Box\nfun create(): Box = Box()\n",
		lib + "old/custom/Explicit.kt":        "package special\nfun custom() = old.create()\n",
		"src/main/kotlin/client/Qualified.kt": "package client\nfun run(): old.Box = old.create()\n",
		"src/main/kotlin/client/Star.kt":      "package client\nimport old.*\nfun runStar()=create()\n",
	}
	for p, text := range files {
		put(t, filepath.Join(root, p), text)
	}
	c := start(t, root)
	raw := c.call("workspace/willRenameFiles", moveParams(root, map[string]string{lib + "old": lib + "fresh"}))
	edits := moveEdits(t, raw, true)
	for p, src := range files {
		want := strings.ReplaceAll(src, "old.", "fresh.")
		want = strings.ReplaceAll(want, "package old\n", "package fresh\n")
		if got := applyText(t, src, edits[uri(filepath.Join(root, p))]); got != want {
			t.Errorf("%s: got %q want %q", p, got, want)
		}
	}
}

func TestPackageMoveBindingConflict(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "kc-lsp.json"), moveModel)
	lib := "library/src/main/kotlin/"
	put(t, filepath.Join(root, lib, "old", "A.kt"), "package old\nfun read() = value()\n")
	put(t, filepath.Join(root, lib, "old", "Value.kt"), "package old\nfun value() = 1\n")
	put(t, filepath.Join(root, lib, "fresh", "Value.kt"), "package fresh\nfun value() = 2\n")
	c := start(t, root)
	// The explicit import that kc generates can preserve the old binding.
	got := moveEdits(t, c.call("workspace/willRenameFiles", moveParams(root, map[string]string{lib + "old/A.kt": lib + "fresh/A.kt"})), true)
	a := filepath.Join(root, lib, "old", "A.kt")
	if !strings.Contains(applyText(t, string(input(t, a)), got[uri(a)]), "import old.value") {
		t.Fatal(got)
	}
	// An overload family explicitly imported by a consumer cannot be
	// retargeted by a single replacement when only half the family moves.
	over := lib + "old/More.kt"
	put(t, filepath.Join(root, over), "package old\nfun value(x: Int) = x\n")
	put(t, filepath.Join(root, "src/main/kotlin/Use.kt"), "package client\nimport old.value\nfun use()=value(1) + value()\n")
	c.notify("workspace/didChangeWatchedFiles", object{})
	result, err := c.request("workspace/willRenameFiles", moveParams(root, map[string]string{over: lib + "other/More.kt"}))
	if err == nil || err.Code != -32803 || len(result) != 0 {
		t.Fatal("split overload import", string(result), err)
	}
}
