package lsp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
)

const intentionsFixture = "testdata/intentions/"

type intentionsReference struct {
	Outline json.RawMessage
	Hints   json.RawMessage
	Actions map[string]struct{ Title, Kind, Text, Absent string }
}

func intentionsBaseline(t *testing.T) intentionsReference {
	t.Helper()
	var ref intentionsReference
	if err := json.Unmarshal(input(t, intentionsFixture+"upstream-271.2085.0.json"), &ref); err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestIntentionReplay(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	path := filepath.Join(root, "app", "Editing.kt")
	put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"library","sourceRoots":["library"],"javaRoots":["library"]},{"name":"app","sourceRoots":["app"],"dependencies":["library"]}]}`)
	for _, f := range []string{"Library.kt", "Printer.java"} {
		put(t, filepath.Join(root, "library", f), string(input(t, intentionsFixture+f)))
	}
	ref := intentionsBaseline(t)
	c := start(t, root)
	doc := object{"uri": uri(path), "version": 1, "text": string(input(t, intentionsFixture+"Outline.kt"))}
	c.notify("textDocument/didOpen", object{"textDocument": doc})
	checkJSON(t, c.call("textDocument/documentSymbol", params(path, position{})), ref.Outline)

	var cases map[string]string
	if err := json.Unmarshal(input(t, intentionsFixture+"cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	cases["vararg"] = "package sample\nfun take(vararg count: Int) = 3\nfun result() = take(|2, 3)\n"
	version := 1
	change := func(text string) {
		version++
		c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": version}, "contentChanges": []change{{Text: text}}})
	}
	for name, marked := range cases {
		t.Run(name, func(t *testing.T) {
			src, q := markedQuery(t, path, marked)
			change(src)
			at := q["position"].(position)
			q["range"] = span{at, at}
			q["context"] = object{"only": []string{"refactor"}, "diagnostics": []object{}}
			var actions []struct {
				Title, Kind string
				Edit        struct {
					DocumentChanges []struct {
						TextDocument struct{ Version int }
						Edits        []edit
					}
				}
			}
			if err := json.Unmarshal(c.call("textDocument/codeAction", q), &actions); err != nil {
				t.Fatal(err)
			}
			want, ok := ref.Actions[name]
			if name == "vararg" {
				want.Absent = "Add names to call arguments"
			} else if !ok {
				t.Fatalf("Missing upstream result for %s", name)
			}
			if want.Absent != "" {
				for _, a := range actions {
					if a.Title == want.Absent {
						t.Fatalf("unexpected %s", a.Title)
					}
				}
				return
			}
			for _, a := range actions {
				if a.Title != want.Title {
					continue
				}
				if a.Kind != want.Kind || len(a.Edit.DocumentChanges) != 1 || a.Edit.DocumentChanges[0].TextDocument.Version != version {
					t.Fatalf("action %+v", a)
				}
				result := applyText(t, src, a.Edit.DocumentChanges[0].Edits)
				if result != want.Text {
					t.Fatalf("got %s\nwant %s", result, want.Text)
				}
				change(result)
				var report struct {
					Items []struct {
						Severity int
						Message  string
					}
				}
				if err := json.Unmarshal(c.call("textDocument/diagnostic", q), &report); err != nil {
					t.Fatal(err)
				}
				for _, d := range report.Items {
					if d.Severity == 1 {
						t.Error(d.Message)
					}
				}
				if name == "names" {
					q["position"] = position{3, 13}
					var dest []location
					if err := json.Unmarshal(c.call("textDocument/definition", q), &dest); err != nil || len(dest) != 1 || dest[0].Range.Start.Line != 1 {
						t.Errorf("overload destination %v (%v)", dest, err)
					}
				}
				return
			}
			t.Fatalf("%s missing from %+v", want.Title, actions)
		})
	}
}

func TestTypeHintResolveReplay(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	lib := input(t, intentionsFixture+"Library.kt")
	for _, attached := range []bool{false, true} {
		name := "workspace"
		if attached {
			name = "attached"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "app", "Editing.kt")
			if attached {
				classes := filepath.Join(root, "classes")
				if err := driver.Compile([]string{intentionsFixture + "Library.kt"}, driver.Options{NoJDK: true, OutDir: classes}); err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(classes, "kotlin/kotlin.kotlin_builtins"), "\x00\x00\x00\x03\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x07")
				writeSources(t, filepath.Join(root, "sources.jar"), fstest.MapFS{"library/Library.kt": {Data: lib}})
				put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"app","sourceRoots":["app"],"classPath":["classes"],"sources":["sources.jar"]}]}`)
			} else {
				put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"library","sourceRoots":["library"]},{"name":"app","sourceRoots":["app"],"dependencies":["library"]}]}`)
				put(t, filepath.Join(root, "library", "Library.kt"), string(lib))
			}
			c := startWithCapabilities(t, root, object{"textDocument": object{"inlayHint": object{"resolveSupport": object{"properties": []string{"label.location"}}}}})
			src := "package sample\nimport library.packet\nfun result() = packet()\n"
			c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "text": src, "version": 1}})
			q := params(path, position{})
			q["range"] = span{position{}, position{3, 0}}
			var hints []object
			raw := c.call("textDocument/inlayHint", q)
			if err := json.Unmarshal(raw, &hints); err != nil || len(hints) != 1 {
				t.Fatalf("hints %s", raw)
			}
			var h struct{ Label []hintPart }
			resolved := c.call("inlayHint/resolve", hints[0])
			if err := json.Unmarshal(resolved, &h); err != nil || len(h.Label) != 2 || h.Label[1].Location == nil {
				t.Fatalf("resolve %s", resolved)
			}
			dest := h.Label[1].Location.URI
			p, err := filePath(dest)
			if err != nil || filepath.Base(p) != "Library.kt" || !bytes.Equal(input(t, p), lib) {
				t.Fatalf("destination %s", dest)
			}
			var public object
			json.Unmarshal(resolved, &public)
			delete(public, "data")
			got, _ := json.Marshal([]object{public})
			want := bytes.ReplaceAll(intentionsBaseline(t).Hints, []byte(`"Library.kt"`), []byte(`"`+dest+`"`))
			checkJSON(t, got, want)
			c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 2}, "contentChanges": []change{{Text: strings.Replace(src, "packet()", "1", 1)}}})
			stale := c.call("inlayHint/resolve", hints[0])
			if bytes.Contains(stale, []byte(`"location"`)) || !bytes.Contains(stale, []byte(`"Packet"`)) {
				t.Fatalf("stale resolve %s", stale)
			}
			if attached {
				// The binary still resolves, but both source entries declare its class.
				archive := filepath.Join(root, "sources.jar")
				writeSources(t, archive, fstest.MapFS{
					"library/Library.kt": {Data: lib},
					"library/Other.kt":   {Data: []byte("package library\nclass Packet\n")},
				})
				c.notify("workspace/didChangeWatchedFiles", object{"changes": []object{{"type": 2, "uri": uri(archive)}}})
				c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 3}, "contentChanges": []change{{Text: src}}})
				if err := json.Unmarshal(c.call("textDocument/inlayHint", q), &hints); err != nil || len(hints) != 1 {
					t.Fatalf("ambiguous class hint: %v %v", hints, err)
				}
				result := c.call("inlayHint/resolve", hints[0])
				if bytes.Contains(result, []byte(`"location"`)) || !bytes.Contains(result, []byte(`"Packet"`)) {
					t.Fatalf("ambiguous class resolved to a location: %s", result)
				}
			}
		})
	}
}

// A client may accept structured labels without implementing deferred resolution.
func TestEagerGenericHintLinks(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	path := filepath.Join(root, "Types.kt")
	src := "class Packet\nclass Box<T>(val item: T)\nfun result() = Box(Packet())\n"
	put(t, path, src)
	c := startWithCapabilities(t, root, object{"textDocument": object{"inlayHint": object{}}})
	q := params(path, position{})
	q["range"] = span{position{2, 0}, position{2, 14}}
	var hints []struct {
		Label []hintPart
		Data  json.RawMessage
	}
	raw := c.call("textDocument/inlayHint", q)
	if err := json.Unmarshal(raw, &hints); err != nil || len(hints) != 1 {
		t.Fatalf("hints %s", raw)
	}
	label := hints[0].Label
	if len(label) != 5 || len(hints[0].Data) != 0 {
		t.Fatalf("eager type %s", raw)
	}
	for part, line := range map[int]int{1: 1, 3: 0} {
		dest := label[part].Location
		if dest == nil || dest.URI != uri(path) || dest.Range.Start != (position{line, 6}) {
			t.Errorf("classifier part %+v", label[part])
		}
	}
}
