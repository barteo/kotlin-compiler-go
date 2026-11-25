package lsp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestFormattingReplay(t *testing.T) {
	var cases map[string]struct {
		Text    string
		Options object
		Range   *span
	}
	var reference struct{ Formatting map[string]string }
	fixtureJSON(t, "formatting.json", &cases)
	fixtureJSON(t, "upstream-271.2085.0.json", &reference)
	if len(cases) == 0 || len(reference.Formatting) != len(cases) {
		t.Fatal("incomplete formatting capture")
	}
	root := t.TempDir()
	c := start(t, root)
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name+".kt")
			doc := object{"uri": uri(path)}
			c.notify("textDocument/didOpen", object{"textDocument": object{"uri": uri(path), "version": 1, "text": tc.Text}})
			params := object{"textDocument": doc, "options": tc.Options}
			method := "textDocument/formatting"
			if tc.Range != nil {
				method = "textDocument/rangeFormatting"
				params["range"] = tc.Range
			}
			var edits []edit
			if err := json.Unmarshal(c.call(method, params), &edits); err != nil {
				t.Fatal(err)
			}
			formatted := applyText(t, tc.Text, edits)
			if want, ok := reference.Formatting[name]; !ok || formatted != want {
				t.Fatalf("got:\n%s\nwant:\n%s", formatted, want)
			}
			c.notify("textDocument/didChange", object{"textDocument": object{"uri": uri(path), "version": 2},
				"contentChanges": []change{{Text: formatted}}})
			checkJSON(t, c.call(method, params), []byte("[]"))
		})
	}
}

func TestFormattingClientIndentation(t *testing.T) {
	const src = "fun test(){\r\nval xs=listOf(\r\n/* keep  spacing */ 1,\r\n2, // keep  this\r\n)\r\nxs\r\n.map {n->\r\n\"\"\"a\r\n  raw   b\"\"\"\r\n}\r\n}\r\n"
	for name, unit := range map[string]string{"twoSpaces": "  ", "tabs": "\t"} {
		t.Run(name, func(t *testing.T) {
			var p parameters
			p.Options.TabSize = 2
			p.Options.InsertSpaces = unit != "\t"
			want := strings.Join([]string{"fun test() {", unit + "val xs = listOf(", unit + unit + "/* keep  spacing */ 1,",
				unit + unit + "2, // keep  this", unit + ")", unit + "xs", unit + unit + ".map { n ->",
				unit + unit + unit + "\"\"\"a", "  raw   b\"\"\"", unit + unit + "}", "}", ""}, "\r\n")
			got := applyText(t, src, format(psi.ParseDocument("x.kt", src), p, false))
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
			if edits := format(psi.ParseDocument("x.kt", got), p, false); len(edits) != 0 {
				t.Fatalf("second pass returned %+v", edits)
			}
		})
	}
}

func TestFormattingRangeWithExistingIndent(t *testing.T) {
	const src = "fun f() {\r\n   val n=outer(inner(\r\n1,\r\n),2)\r\n}\r\n"
	s := psi.ParseDocument("x.kt", src)
	var p parameters
	p.Options.TabSize = 2
	p.Options.InsertSpaces = true
	p.Range = span{position{1, 15}, position{4, 0}}
	got := applyText(t, src, format(s, p, true))
	const want = "fun f() {\r\n   val n=outer(\r\n     inner(\r\n       1,\r\n     ), 2\r\n   )\r\n}\r\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	p.Range.End = position{6, 0}
	if edits := format(psi.ParseDocument("x.kt", got), p, true); len(edits) != 0 {
		t.Fatalf("second pass returned %+v", edits)
	}
}

func TestFormattingRangeAfterUnicode(t *testing.T) {
	const src = "fun f() { val s=\"😀\"; val n=1+2; val z=0 }"
	s := psi.ParseDocument("x.kt", src)
	start := strings.Index(src, "1+2")
	end := start + 1
	var p parameters
	p.Range = span{pos(s, start), pos(s, end)}
	got := applyText(t, src, format(s, p, true))
	if want := strings.Replace(src, "n=1+2", "n= 1+2", 1); got != want {
		t.Fatal(got)
	}
}

func TestFormattingInvalidSyntax(t *testing.T) {
	if edits := format(psi.ParseDocument("x.kt", "fun f( {"), parameters{}, false); len(edits) != 0 {
		t.Fatalf("edited invalid source: %+v", edits)
	}
}
