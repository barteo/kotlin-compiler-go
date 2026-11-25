package lsp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func fixtureJSON(t *testing.T, name string, v any) {
	t.Helper()
	if err := json.Unmarshal(input(t, filepath.Join("testdata", "daily", name)), v); err != nil {
		t.Fatal(err)
	}
}
func dailyWorkspace(t *testing.T) (*client, string) {
	t.Helper()
	t.Setenv("PATH", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("KOTLIN_HOME", "")
	root := t.TempDir()
	if err := driver.Compile([]string{"testdata/daily/Library.kt"}, driver.Options{NoJDK: true, OutDir: filepath.Join(root, "classes"), ModuleName: "editinglib"}); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "kc-lsp.json"), `{"version":1,"modules":[{"name":"library","sourceRoots":["library"]},{"name":"app","sourceRoots":["src"],"dependencies":["library"],"classPath":["classes"]}]}`)
	put(t, filepath.Join(root, "library", "Scopes.kt"), string(input(t, "testdata/daily/Scopes.kt")))
	return start(t, root), filepath.Join(root, "src", "Daily.kt")
}
func applyText(t *testing.T, text string, edits []edit) string {
	t.Helper()
	snap := psi.ParseDocument("edits.kt", text)
	edits = slices.Clone(edits)
	slices.SortStableFunc(edits, func(a, b edit) int {
		if before(a.Range.Start, b.Range.Start) {
			return 1
		}
		if before(b.Range.Start, a.Range.Start) {
			return -1
		}
		return 0
	})
	for _, e := range edits {
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
	return text
}
func markedQuery(t *testing.T, path, marked string) (string, object) {
	t.Helper()
	a, b, ok := strings.Cut(marked, "|")
	if !ok {
		t.Fatal("missing cursor")
	}
	src := a + b
	return src, params(path, pos(psi.ParseDocument(path, src), len(a)))
}
func TestDailyEditingReplay(t *testing.T) {
	c, path := dailyWorkspace(t)
	var cases map[string]string
	var ref map[string]json.RawMessage
	fixtureJSON(t, "cases.json", &cases)
	fixtureJSON(t, "upstream-271.2085.0.json", &ref)
	version := 0
	change := func(src string) {
		version++
		doc := object{"uri": uri(path), "version": version}
		if version == 1 {
			doc["text"] = src
			c.notify("textDocument/didOpen", object{"textDocument": doc})
			return
		}
		c.notify("textDocument/didChange", object{"textDocument": doc, "contentChanges": []change{{Text: src}}})
	}
	for _, name := range []string{"completion", "signature", "assertion", "safeCall", "elvis", "inner", "import"} {
		t.Run(name, func(t *testing.T) {
			src, q := markedQuery(t, path, cases[name])
			change(src)
			var want struct {
				Order       []string
				Text        string
				After       json.RawMessage
				Diagnostics []struct {
					Range    span
					Severity int
				}
				Signatures      []object
				ActiveSignature int
			}
			if err := json.Unmarshal(ref[name], &want); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "completion":
				var items []struct {
					Label    string
					TextEdit edit
				}
				got := c.call("textDocument/completion", q)
				json.Unmarshal(got, &items)
				var names []string
				for _, item := range items {
					if slices.Contains(want.Order, item.Label) {
						names = append(names, item.Label)
					}
					if item.Label == "dailyLocal" && applyText(t, src, []edit{item.TextEdit}) != want.Text {
						t.Errorf("completion edit %+v", item)
					}
				}
				if !slices.Equal(names, want.Order) {
					t.Errorf("order %v want %v", names, want.Order)
				}
			case "signature":
				var res struct {
					Signatures      []object
					ActiveSignature int
				}
				got := c.call("textDocument/signatureHelp", q)
				json.Unmarshal(got, &res)
				a, _ := json.Marshal(res.Signatures)
				b, _ := json.Marshal(want.Signatures)
				checkJSON(t, a, b)
				if res.ActiveSignature != want.ActiveSignature {
					t.Fatal(got)
				}
			default:
				ds := c.call("textDocument/diagnostic", q)
				var report struct {
					Items []struct {
						Range    span
						Severity int
						Code     string
					}
				}
				json.Unmarshal(ds, &report)
				var pull struct{ Items json.RawMessage }
				json.Unmarshal(ds, &pull)
				if c.published.Version != version {
					t.Fatalf("push version %d, want %d", c.published.Version, version)
				}
				checkJSON(t, c.published.Diagnostics, pull.Items)
				target := want.Diagnostics[0]
				idx := slices.IndexFunc(report.Items, func(d struct {
					Range    span
					Severity int
					Code     string
				}) bool {
					return d.Range == target.Range && d.Severity == target.Severity
				})
				if idx < 0 {
					t.Errorf("expected diagnostic at %+v, got %s", target, ds)
				}
				at := q["position"].(position)
				q["range"] = span{at, at}
				q["context"] = object{"only": []string{"quickfix"}}
				got := c.call("textDocument/codeAction", q)
				var actions []struct {
					Title       string
					Diagnostics []struct {
						Range    span
						Severity int
					}
					Edit struct {
						DocumentChanges []struct {
							Edits        []edit
							TextDocument struct{ Version int }
						}
					}
				}
				json.Unmarshal(got, &actions)
				for _, a := range actions {
					if len(a.Edit.DocumentChanges) != 1 {
						continue
					}
					d := a.Edit.DocumentChanges[0]
					if applyText(t, src, d.Edits) == want.Text {
						if d.TextDocument.Version != version || !slices.Contains(a.Diagnostics, target) {
							t.Fatalf("missing diagnostic/version: %s", got)
						}
						change(want.Text)
						after := c.call("textDocument/diagnostic", q)
						json.Unmarshal(after, &pull)
						checkJSON(t, pull.Items, want.After)
						checkJSON(t, c.published.Diagnostics, want.After)
						if c.published.Version != version {
							t.Fatalf("clear push version %d want %d", c.published.Version, version)
						}
						return
					}
				}
				t.Errorf("expected %q from actions %s", want.Text, got)
			}
		})
	}
}

func TestCompletionEditsAndResolve(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		root := t.TempDir()
		path := filepath.Join(root, "Use.kt")
		opts := completionOptions{SnippetSupport: advanced, InsertReplaceSupport: advanced}
		if advanced {
			opts.ResolveSupport.Properties = []string{"documentation", "detail"}
			opts.DocumentationFormat = []string{"markdown"}
		}
		c := startClient(t, root, opts)
		src, q := markedQuery(t, path, "/** First overload. */\r\nfun sample(x: Int) = x\r\n/** Second overload. */\r\nfun sample(x: String) = x\r\nfun use() { val s = \"😀\"; sam|old }\r\n")
		c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "text": src, "version": 1}})
		var items []object
		if err := json.Unmarshal(c.call("textDocument/completion", q), &items); err != nil {
			t.Fatal(err)
		}
		var selected []object
		for _, item := range items {
			if item["label"] != "sample" {
				continue
			}
			selected = append(selected, item)
			e := item["textEdit"].(map[string]any)
			if advanced {
				if e["newText"] != "sample($0)" || e["range"] != nil || e["insert"] == nil || item["insertTextFormat"] != float64(2) {
					t.Fatal(e)
				}
			} else if e["newText"] != "sample()" || e["range"] == nil {
				t.Fatal(e)
			}
			raw, _ := json.Marshal(e)
			var ranges struct{ Range, Insert, Replace span }
			json.Unmarshal(raw, &ranges)
			r := ranges.Range
			if advanced {
				r = ranges.Replace
				if ranges.Insert.End != q["position"].(position) {
					t.Fatal(ranges)
				}
			}
			if got := applyText(t, src, []edit{{r, "sample(1)"}}); !strings.Contains(got, "😀\"; sample(1) }\r\n") {
				t.Fatal(got)
			}
			res := c.call("completionItem/resolve", item)
			var resolved struct {
				Detail        string
				Documentation struct{ Kind string }
			}
			if err := json.Unmarshal(res, &resolved); err != nil {
				t.Fatal(err)
			}
			if resolved.Detail == "" || advanced && (item["detail"] != nil || resolved.Documentation.Kind != "markdown") {
				t.Fatalf("detail resolution %s", res)
			}
			doc := "First overload."
			if strings.Contains(resolved.Detail, "String") {
				doc = "Second overload."
			}
			if !bytes.Contains(res, []byte(doc)) {
				t.Fatalf("resolve %s want %s", res, doc)
			}
		}
		if len(selected) != 2 {
			t.Fatalf("overloads %v", items)
		}
		c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 2}, "contentChanges": []change{{Text: strings.ReplaceAll(src, "overload.", "changed.")}}})
		for _, item := range selected {
			if got := c.call("completionItem/resolve", item); bytes.Contains(got, []byte("documentation")) {
				t.Fatalf("stale resolve %s", got)
			}
		}
	}
}

func TestCompiledPartialSignatures(t *testing.T) {
	c, path := dailyWorkspace(t)
	for j, tc := range []struct {
		call, first string
		active      int
	}{
		{"p.printValue(\"x\", |", "value: String", 1},
		{"p.printValue(4, 7, |", "vararg rest", 1},
		{"p.printValue(count = 2, value = |\"x\")", "value: String", 0},
		{"p.printValue(p.printValue(\"x\", |1), 2)", "value: String", 1},
		{"DailyPacket(|", "tag: Int", 0},
	} {
		src, q := markedQuery(t, path, "package sample\nimport editinglib.*\nfun f(p: Printer) = "+tc.call)
		if j == 0 {
			c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "text": src, "version": 1}})
		} else {
			c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": j + 1}, "contentChanges": []change{{Text: src}}})
		}
		var res struct {
			Signatures []struct {
				Label           string
				ActiveParameter int
			}
			ActiveSignature int
		}
		b := c.call("textDocument/signatureHelp", q)
		json.Unmarshal(b, &res)
		if len(res.Signatures) == 0 || !strings.Contains(res.Signatures[res.ActiveSignature].Label, tc.first) || res.Signatures[res.ActiveSignature].ActiveParameter != tc.active {
			t.Errorf("%s: %s", tc.call, b)
		}
	}
}

func TestQuickFixScopesAndComments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Fix.kt")
	src := "class Box {\n    @Suppress(\"UNUSED_PARAMETER\") fun length(s: String) {\n        val n = s!!.length\n    }\n}"
	put(t, path, src)
	c := start(t, root)
	q := params(path, position{2, 17})
	q["range"] = span{position{2, 17}, position{2, 19}}
	q["context"] = object{"only": []string{"quickfix"}}
	b := c.call("textDocument/codeAction", q)
	var fixes []struct {
		Title string
		Edit  struct{ DocumentChanges []struct{ Edits []edit } }
	}
	json.Unmarshal(b, &fixes)
	count := 0
	for _, fix := range fixes {
		if !strings.HasPrefix(fix.Title, "Suppress UNNECESSARY_NOT_NULL_ASSERTION") {
			continue
		}
		count++
		text := applyText(t, src, fix.Edit.DocumentChanges[0].Edits)
		if strings.Contains(fix.Title, "for length") && !strings.Contains(text, `@Suppress("UNUSED_PARAMETER", "UNNECESSARY_NOT_NULL_ASSERTION")`) {
			t.Error(text)
		}
		c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "version": count, "text": text}})
		if ds := c.call("textDocument/diagnostic", q); bytes.Contains(ds, []byte(`"code":"UNNECESSARY_NOT_NULL_ASSERTION"`)) {
			t.Errorf("suppression %s: %s", text, ds)
		}
	}
	if count != 3 {
		t.Fatalf("scopes %s", b)
	}
	marked := "fun f(s: String) = s /* left */ |?: /* explanation */ error(\"unused\")"
	text, at := markedQuery(t, path, marked)
	c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 10}, "contentChanges": []change{{Text: text}}})
	p := at["position"].(position)
	at["range"] = span{p, p}
	at["context"] = q["context"]
	b = c.call("textDocument/codeAction", at)
	json.Unmarshal(b, &fixes)
	for _, f := range fixes {
		if strings.Contains(f.Title, "Elvis") {
			after := applyText(t, text, f.Edit.DocumentChanges[0].Edits)
			if !strings.Contains(after, "= s /* left */ /* explanation */") || strings.Contains(after, "error(") {
				t.Error(after)
			}
			return
		}
	}
	t.Fatalf("missing Elvis fix %s", b)
}

func TestCompletionScopesAndSafeReceiver(t *testing.T) {
	c, path := dailyWorkspace(t)
	source := "package sample\nclass Row(val heading: Int)\nfun Row.visibleExt()=heading\n"
	for j, tc := range []struct{ use, has, lacks string }{
		{"fun use(row: Row?) { row?.| }", "heading", "use"},
		{"fun Row.use() { visible| }", "visibleExt", "dailyLocal"},
		{"fun Row.use() { hea| }", "heading", "inScope"},
		{"fun use() { val marked = 1; run { val marked = \"inner\"; mar| } }", "marked", "heading"},
	} {
		src, q := markedQuery(t, path, source+tc.use)
		if j == 0 {
			c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "version": 1, "text": src}})
		} else {
			c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": j + 1}, "contentChanges": []change{{Text: src}}})
		}
		var items []struct{ Label string }
		raw := c.call("textDocument/completion", q)
		json.Unmarshal(raw, &items)
		count := 0
		for _, item := range items {
			if item.Label == tc.has {
				count++
			}
			if item.Label == tc.lacks {
				t.Errorf("unexpected %s for %s", item.Label, tc.use)
			}
		}
		if count != 1 {
			t.Errorf("%s yielded %d copies of %q: %s", tc.use, count, tc.has, raw)
		}
	}
}

func TestMergeExistingSuppression(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`@Suppress() class C`, `@Suppress("NEW") class C`},
		{`@Suppress("OLD",) class C`, `@Suppress("OLD", "NEW") class C`},
		{`@Suppress(names = ["OLD"]) class C`, `@Suppress(names = ["OLD", "NEW"]) class C`},
		{`@Suppress(names = ["OLD", "NEW"]) class C`, ""},
	} {
		snap := psi.ParseDocument("C.kt", tc.input)
		entry := snap.DeclarationNodes()[0].ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindANNOTATION_ENTRY)
		e, ok := appendSuppression(entry, "NEW")
		if !ok && tc.want == "" {
			continue
		}
		if !ok || applyText(t, tc.input, []edit{e}) != tc.want {
			t.Errorf("merge %s: %+v", tc.input, e)
		}
	}
}
