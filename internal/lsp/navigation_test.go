package lsp

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
)

const navFixture = "testdata/navigation/"

type navigationReference struct {
	Definitions map[string]struct {
		OriginSelectionRange              span
		TargetURI                         string
		TargetRange, TargetSelectionRange span
	}
	Hints map[string]json.RawMessage
}

func navigationBaseline(t *testing.T) navigationReference {
	t.Helper()
	var ref navigationReference
	if err := json.Unmarshal(input(t, navFixture+"upstream-271.2085.0.json"), &ref); err != nil {
		t.Fatal(err)
	}
	return ref
}

func writeSources(t *testing.T, path string, sources fstest.MapFS) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z := zip.NewWriter(file)
	if err := z.AddFS(sources); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOverloadNavigationReplay(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	t.Setenv("PATH", "")
	definitions := navigationBaseline(t).Definitions
	var cases map[string]string
	if err := json.Unmarshal(input(t, navFixture+"cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"workspace-java-links", "attached-locations"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "src", "Caller.kt")
			classes := filepath.Join(root, "classes")
			if err := driver.Compile([]string{navFixture + "Overloads.kt", navFixture + "IntExtensions.kt"}, driver.Options{NoJDK: true, OutDir: classes}); err != nil {
				t.Fatal(err)
			}
			// Enable Kotlin package metadata lookup with an empty versioned
			// built-ins fragment. The fixture uses only compiler intrinsic types.
			put(t, filepath.Join(classes, "kotlin/kotlin.kotlin_builtins"), "\x00\x00\x00\x03\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x07")
			java := input(t, navFixture+"Parser.java")
			sources := fstest.MapFS{
				"nav/Overloads.kt":     {Data: input(t, navFixture+"Overloads.kt")},
				"nav/IntExtensions.kt": {Data: input(t, navFixture+"IntExtensions.kt")},
			}
			links := mode == "workspace-java-links"
			if links {
				put(t, filepath.Join(root, "src", "Parser.java"), string(java))
			} else {
				// A Java library with the fixture's two overloads. CI replays
				// these calls without a Java compiler or installed JDK.
				class := classfile.NewClass(classfile.AccPublic, "nav/Parser", "java/lang/Object")
				for _, desc := range []string{"(I)I", "(Lnav/Parser;)I"} {
					class.AddAbstractMethod(classfile.AccPublic|classfile.AccStatic|classfile.AccNative, "parse", desc)
				}
				put(t, filepath.Join(classes, "nav", "Parser.class"), string(class.Bytes()))
				sources["nav/Parser.java"] = &fstest.MapFile{Data: java}
			}
			writeSources(t, filepath.Join(root, "sources.jar"), sources)
			put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"app","sourceRoots":["src"],"classPath":["classes"],"sources":["sources.jar"]}]}`)
			c := startClient(t, root, completionOptions{}, links)
			version := 1
			c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "text": "", "version": version}})
			for name, marked := range cases {
				t.Run(name, func(t *testing.T) {
					src, q := markedQuery(t, path, marked)
					version++
					c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": version}, "contentChanges": []change{{Text: src + "\n"}}})
					got := c.call("textDocument/definition", q)
					ref := definitions[name]
					var l location
					if links {
						var result []struct {
							OriginSelectionRange, TargetRange, TargetSelectionRange span
							TargetURI                                               string
						}
						if err := json.Unmarshal(got, &result); err != nil || len(result) != 1 {
							t.Fatalf("definition %s", got)
						}
						d := result[0]
						if d.OriginSelectionRange != ref.OriginSelectionRange || d.TargetRange != ref.TargetRange {
							t.Fatalf("link ranges %s", got)
						}
						l = location{d.TargetURI, d.TargetSelectionRange}
					} else if err := json.Unmarshal(got, &l); err != nil {
						t.Fatal(err)
					}
					p, err := filePath(l.URI)
					if err != nil || filepath.Base(p) != ref.TargetURI || l.Range != ref.TargetSelectionRange || string(input(t, p)) != string(input(t, navFixture+ref.TargetURI)) {
						t.Fatalf("target %s", got)
					}
				})
			}
			// A stale attachment also claims this exact compiled signature.
			// Neither location is a unique destination.
			sources["nav/Other.kt"] = &fstest.MapFile{Data: []byte("package nav\nfun parse(value: Int): Int = 0\n")}
			archive := filepath.Join(root, "sources.jar")
			writeSources(t, archive, sources)
			c.notify("workspace/didChangeWatchedFiles", object{"changes": []object{{"uri": uri(archive), "type": 2}}})
			src, q := markedQuery(t, path, cases["kotlinInt"])
			c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": version + 1},
				"contentChanges": []change{{Text: src}}})
			checkJSON(t, c.call("textDocument/definition", q), []byte("null"))
		})
	}
}

func TestLambdaAndChainHintReplay(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	t.Setenv("PATH", "")
	root := t.TempDir()
	path := filepath.Join(root, "Hints.kt")
	src := string(input(t, navFixture+"Hints.kt"))
	put(t, path, src)
	c := start(t, root)
	c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "version": 1, "text": src}})
	ref := navigationBaseline(t).Hints
	q := params(path, position{})
	for _, name := range []string{"default", "chains", "partialEdit", "disabled"} {
		t.Run(name, func(t *testing.T) {
			opts := object{}
			r := span{position{12, 0}, position{strings.Count(src, "\n"), 0}}
			switch name {
			case "chains":
				opts["jetbrains.kotlin.hints.call.chains"] = true
			case "disabled":
				opts["jetbrains.kotlin.hints"] = object{"settings.lambda.return": false, "lambda.receivers.parameters": false}
				c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 3}, "contentChanges": []change{{Text: src}}})
			case "partialEdit":
				before, _, _ := strings.Cut(src, "    42\n")
				line := strings.Count(before, "\n")
				changed := span{position{line, 4}, position{line, 6}}
				c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 2}, "contentChanges": []change{{Range: &changed, Text: "/* 😀 */ 420"}}})
				r = span{position{line - 2, 0}, position{line + 1, 0}}
			}
			c.notify("workspace/didChangeConfiguration", object{"settings": opts})
			q["range"] = r
			checkJSON(t, c.call("textDocument/inlayHint", q), ref[name])
		})
	}
}
