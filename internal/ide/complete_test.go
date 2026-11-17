package ide

import (
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
)

// at returns the context at | in src.
func completionAt(t *testing.T, src string) CompletionContext {
	t.Helper()
	off := strings.Index(src, "|")
	if off < 0 {
		t.Fatalf("no caret in %q", src)
	}
	return CompletionAt(NewDocument("a.kt", 1, src[:off]+src[off+1:]), off)
}

func TestCompletionAt(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		prefix    string
		none      bool
		keywords  []string // nil: no keywords; else a subset
		probe     string   // a substring of the probe text
	}{
		{"statement", "fun f() {\n    pr|\n}", "pr", false, []string{"val", "return", "if"}, "    __kc_probe__\n"},
		{"empty statement", "fun f() {\n    |\n}", "", false, []string{"val", "for"}, ""},
		{"middle of name", "fun f() {\n    pr|intln()\n}", "pr", false, []string{"val"}, "__kc_probe__()"},
		{"member", "fun f(s: String) {\n    s.le|\n}", "le", false, nil, "s.__kc_probe__"},
		{"safe member", "fun f(s: String?) {\n    s?.|\n}", "", false, nil, "s?.__kc_probe__"},
		{"argument", "fun f() {\n    println(|\n}", "", false, []string{"if", "null"}, "println(__kc_probe__)"},
		{"nested argument", "fun f() {\n    g(1, h(x|\n}", "x", false, []string{"null"}, "h(__kc_probe__))"},
		{"closed argument", "fun f() {\n    println(|)\n}", "", false, []string{"null"}, "println(__kc_probe__)\n"},
		{"type", "fun f() {\n    val x: Str|\n}", "Str", false, nil, "val x: __kc_probe__"},
		{"callable reference", "fun f() {\n    val r = String::le|\n}", "le", false, nil, "String::__kc_probe__"},
		{"class body", "class A {\n    ov|\n}", "ov", false, []string{"override", "fun"}, ""},
		{"top level", "package p\n\nfu|\n", "fu", false, []string{"fun", "class"}, ""},
		{"template", "fun f(s: String) = \"${s.le|}\"", "le", false, nil, "${s.__kc_probe__}"},
		{"short template", "fun f(sx: String) = \"$s|\"", "s", false, nil, "\"$__kc_probe__\""},
		{"declaration name", "fun f() {\n    val cou|\n}", "cou", true, nil, ""},
		{"parameter name", "fun f(na|) {}", "na", true, nil, ""},
		{"comment", "// pr|\nfun f() {}", "pr", true, nil, ""},
		{"block comment", "/* pr| */\nfun f() {}", "pr", true, nil, ""},
		{"after block comment", "fun f() {\n    /* c */pr|\n}", "pr", false, []string{"val"}, ""},
		{"kdoc", "/** pr| */\nfun f() {}", "pr", true, nil, ""},
		{"string", "fun f() = \"ab|c\"", "ab", true, nil, ""},
		{"string start", "fun f() = \"|\"", "", true, nil, ""},
		{"number", "fun f() = 12|", "12", true, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cc := completionAt(t, tc.src)
			if cc.Prefix != tc.prefix || cc.None != tc.none {
				t.Fatalf("prefix %q none %v, want %q %v", cc.Prefix, cc.None, tc.prefix, tc.none)
			}
			if tc.none {
				return
			}
			if !strings.Contains(cc.ProbeText, tc.probe) {
				t.Errorf("probe text %q lacks %q", cc.ProbeText, tc.probe)
			}
			if tc.keywords == nil && cc.Keywords != nil {
				t.Errorf("keywords %v, want none", cc.Keywords)
			}
			for _, kw := range tc.keywords {
				if !slices.Contains(cc.Keywords, kw) {
					t.Errorf("no keyword %s in %v", kw, cc.Keywords)
				}
			}
		})
	}
}

func TestCompletionAtImport(t *testing.T) {
	cc := completionAt(t, "package p\n\nimport java.util.con|\n")
	if !cc.Import || cc.Package || cc.Qualifier != "java.util" || cc.Prefix != "con" {
		t.Errorf("%+v", cc)
	}
	cc = completionAt(t, "package com.ex|\n")
	if !cc.Import || !cc.Package || cc.Qualifier != "com" {
		t.Errorf("%+v", cc)
	}
}

func TestComplete(t *testing.T) {
	src := "package p\n\nimport a.B\n\nfun f(count: Int) {\n    co|\n}\n"
	cc := completionAt(t, src)
	intT := &analysis.Type{Name: "kotlin.Int"}
	c := &analysis.Completion{
		Reached:  true,
		Expected: intT,
		Items: []analysis.CompletionItem{
			{Name: "count", Kind: "parameter", Origin: "local", Type: intT, Receiver: -1},
			{Name: "copy", Kind: "function", Origin: "topLevel", Receiver: -1, Type: intT, Call: &analysis.Call{Name: "p.copy", DeclaredReturnType: intT}},
			{Name: "compute", Kind: "function", Origin: "topLevel", Receiver: -1, Call: &analysis.Call{Name: "p.compute",
				Parameters: []analysis.Type{{Name: "kotlin.Function0"}}, ParameterNames: []string{"block"}}},
			{Name: "cold", Kind: "property", Origin: "topLevel", Receiver: -1, Deprecated: true, Type: intT},
			{Name: "other", Kind: "local", Origin: "local", Receiver: -1},
		},
		Importables: []analysis.Importable{{Name: "ConcurrentHashMap", Package: "java.util.concurrent", Class: true}},
	}
	items, incomplete := Complete(cc, c, CompletionOptions{Snippets: true})
	var labels []string
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	// The expected type first, locals first, deprecated last; then
	// keywords; imports.
	want := []string{"count", "copy", "cold", "continue", "compute", "ConcurrentHashMap"}
	if !slices.Equal(labels, want) || incomplete {
		t.Fatalf("labels %v, want %v", labels, want)
	}
	if items[4].Insert != "compute { $0 }" || !items[4].Snippet || items[1].Insert != "copy()" {
		t.Errorf("inserts %q %q", items[4].Insert, items[1].Insert)
	}
	im := items[5].Import
	if im == nil || im.Text != "\nimport java.util.concurrent.ConcurrentHashMap" || src[:im.Range.Start] != "package p\n\nimport a.B" {
		t.Errorf("import edit %+v", im)
	}
	if _, incomplete := Complete(cc, c, CompletionOptions{Limit: 2}); !incomplete {
		t.Error("not incomplete over the limit")
	}
}

func TestCompletionDoc(t *testing.T) {
	d := NewDocument("a.kt", 1, "package p\n\n/** Adds [x]. */\nfun add(x: Int) = x\n")
	off := strings.Index(d.Text(), "/**")
	if got := CompletionDoc(d.Snap, off); got != "```kotlin\nfun add(x: Int)\n```\n\nAdds `x`." {
		t.Errorf("doc %q", got)
	}
}
