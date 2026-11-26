package lsp

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func organizeAction(t *testing.T, c *client, path string) []edit {
	t.Helper()
	q := params(path, position{})
	q["range"] = span{}
	q["context"] = object{"only": []string{"source.organizeImports"}, "diagnostics": []object{}}
	raw := c.call("textDocument/codeAction", q)
	var actions []struct {
		Kind string
		Edit struct {
			DocumentChanges []struct {
				TextDocument struct{ URI string }
				Edits        []edit
			}
		}
	}
	if err := json.Unmarshal(raw, &actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) == 0 {
		return nil
	}
	if len(actions) != 1 || actions[0].Kind != "source.organizeImports" || len(actions[0].Edit.DocumentChanges) != 1 || actions[0].Edit.DocumentChanges[0].TextDocument.URI != uri(path) {
		t.Fatalf("unexpected organize action %s", raw)
	}
	return actions[0].Edit.DocumentChanges[0].Edits
}

func TestOrganizeImportsReplay(t *testing.T) {
	var cases map[string]string
	var reference struct{ Imports map[string]string }
	fixtureJSON(t, "imports.json", &cases)
	fixtureJSON(t, "upstream-271.2085.0.json", &reference)
	if len(cases) == 0 || len(cases) != len(reference.Imports) {
		t.Fatal("incomplete import replay")
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if errors := checkOrganized(t, src, reference.Imports[name]); len(errors) != 0 {
				t.Fatalf("fixture errors: %v", errors)
			}
		})
	}
}

func TestOrganizeImportPolicies(t *testing.T) {
	const docs = "package consumer\n\nimport editinglib.Printer\nimport editinglib.EditingPacket as Packet\n\n/** [caption][Packet.tag], Printer in prose, `[Printer]` and [editinglib.Printer]. */\nfun read() = 1\n"
	const incomplete = "package consumer\n\nimport editinglib.Printer\n\nfun read() = missing()\n"
	const comment = "package consumer\r\n\r\nimport editinglib.EditingPacket // keep comment\r\nimport editinglib.Printer\r\n\r\nfun read() = 1\r\n"
	const duplicate = "package consumer\n\nimport sample.Scope\nimport sample . Scope\n\nfun read(s: Scope) = s.editingMember\n"
	const unknown = "package consumer\n\nimport missing.Unknown\n\nfun read() = 1\n"
	const wild = "package consumer\n\nimport editinglib.*\nimport sample.*\nimport sample.choose\nimport editinglib.Printer\n\nfun read() = choose(1)\n"
	for _, tc := range []struct{ name, src, want string }{
		{"kdoc", docs, strings.Replace(docs, "import editinglib.Printer\n", "", 1)},
		{"incomplete", incomplete, incomplete},
		{"comment", comment, strings.Replace(comment, "import editinglib.Printer\r\n", "", 1)},
		{"duplicates", duplicate, strings.Replace(duplicate, "import sample . Scope\n", "", 1)},
		{"unknown", unknown, unknown},
		{"wildcards", wild, strings.Replace(wild, "import editinglib.Printer\n", "", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) { checkOrganized(t, tc.src, tc.want) })
	}
}

func checkOrganized(t *testing.T, src, want string) []string {
	t.Helper()
	c, path := editingWorkspace(t)
	c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "version": 1, "text": src}})
	// The parameter types and overloads must still navigate to the same
	// source declarations when the edit shifts the caller's line numbers.
	type destination struct {
		TargetURI            string
		TargetSelectionRange span
	}
	definitions := func(text string) []destination {
		snap := psi.ParseDocument(path, text)
		var defs []destination
		for _, use := range []string{"pick(", "choose(", "Scope)", "Scope,"} {
			if at := strings.LastIndex(text, use); at >= 0 {
				raw := c.call("textDocument/definition", params(path, pos(snap, at)))
				if !strings.Contains(string(raw), "Scopes.kt") {
					t.Fatalf("missing definition for %s: %s", use, raw)
				}
				var links []destination
				if err := json.Unmarshal(raw, &links); err != nil {
					t.Fatal(err)
				}
				if len(links) != 1 {
					t.Fatalf("expected a unique definition for %s: %s", use, raw)
				}
				defs = append(defs, links[0])
			}
		}
		return defs
	}
	before := definitions(src)
	issues := func() []string {
		var report struct {
			Items []struct {
				Code, Message string
				Severity      int
			}
		}
		if err := json.Unmarshal(c.call("textDocument/diagnostic", params(path, position{})), &report); err != nil {
			t.Fatal(err)
		}
		var errors []string
		for _, d := range report.Items {
			if d.Severity == 1 {
				errors = append(errors, d.Code+" "+d.Message)
			}
		}
		return errors
	}
	initial := issues()
	edits := organizeAction(t, c, path)
	got := applyText(t, src, edits)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 2}, "contentChanges": []change{{Text: got}}})
	if !slices.Equal(definitions(got), before) {
		t.Fatal("organizing imports changed a source binding")
	}
	for _, issue := range issues() {
		if !slices.Contains(initial, issue) {
			t.Fatalf("organizing imports introduced %s", issue)
		}
	}
	if again := organizeAction(t, c, path); len(again) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
	return initial
}

func TestKDocImportNames(t *testing.T) {
	doc := "/** [Direct] [Caption][Alias.member] [qualified.Type] [Markdown](url)\n" +
		" * ``one ` [Inline] two``\n * ```kotlin\n * [Example]\n * ```\n" +
		" * @throws Failure on error\n */"
	used := map[string]bool{}
	kdocImportNames(doc, used)
	want := map[string]bool{"Direct": true, "Alias": true, "qualified": true, "Failure": true}
	if !maps.Equal(used, want) {
		t.Fatalf("KDoc link roots %v, want %v", used, want)
	}
}
