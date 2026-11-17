package ide

import (
	"regexp"
	"slices"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
)

// expand returns a snippet's text with its placeholders' defaults.
func expand(s string) string {
	s = regexp.MustCompile(`\$\{\d+:([^}]*)\}`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`\$\{\d+\}|\$\d+`).ReplaceAllString(s, "")
	return regexp.MustCompile(`\\([\\$}])`).ReplaceAllString(s, "$1")
}

func TestPostfixTemplates(t *testing.T) {
	boolean := &analysis.Type{Name: "kotlin.Boolean"}
	list := &analysis.Type{Name: "kotlin.collections.List", Supertypes: []string{"kotlin.collections.Iterable"},
		Arguments: []analysis.Type{{Name: "kotlin.String"}}}
	nullable := &analysis.Type{Name: "kotlin.String", Nullable: true}
	for _, tc := range []struct {
		name, src string
		recv      *analysis.Type
		label     string
		want      string // the source after the item, "" if it isn't offered
	}{
		{"if", "fun f(a: Int) {\n    a > 1|\n}", boolean, "if", ""}, // the dot's receiver is 1
		{"if paren", "fun f(a: Int) {\n    (a > 1).i|\n}", boolean, "if", "fun f(a: Int) {\n    if (a > 1) {\n    \n}\n}"},
		{"not", "fun f(a: Int) = g((a > 1).no|)", boolean, "not", "fun f(a: Int) = g(!(a > 1))"},
		{"not simple", "fun f(b: Boolean) = g(b.|)", boolean, "not", "fun f(b: Boolean) = g(!b)"},
		{"if not a statement", "fun f(b: Boolean) = g(b.|)", boolean, "if", ""},
		{"for", "fun f(names: List<String>) {\n    names.f|\n}", list, "for", "fun f(names: List<String>) {\n    for (name in names) {\n    \n}\n}"},
		{"for not iterable", "fun f(b: Boolean) {\n    b.f|\n}", boolean, "for", ""},
		{"nn", "fun f(s: String?) {\n    s.nn|\n}", nullable, "nn", "fun f(s: String?) {\n    if (s != null) {\n    \n}\n}"},
		{"nn non-null", "fun f(b: Boolean) {\n    b.nn|\n}", boolean, "nn", ""},
		{"val", "fun f(names: List<String>) {\n    names.size.v|\n}", &analysis.Type{Name: "kotlin.Int"}, "val", "fun f(names: List<String>) {\n    val size = names.size\n}"},
		{"let template string", "fun f(s: String) {\n    \"$s\".l|\n}", nullable, "let", "fun f(s: String) {\n    \"$s\".let {  }\n}"},
		{"multi-line", "fun f(names: List<String>) {\n    names\n        .|\n}", list, "for", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cc := completionAt(t, tc.src)
			c := &analysis.Completion{Reached: true, Receiver: tc.recv}
			items, _ := Complete(cc, c, CompletionOptions{Snippets: true})
			i := slices.IndexFunc(items, func(it CompletionItem) bool { return it.Kind == "snippet" && it.Label == tc.label })
			if tc.want == "" {
				if i >= 0 {
					t.Fatalf("offered: %+v", items[i])
				}
				return
			}
			if i < 0 {
				t.Fatalf("not offered among %v", items)
			}
			it := items[i]
			src := cc.snap.Source()
			got := src[:it.Edit.Start] + expand(it.Insert) + src[it.Edit.End:]
			if got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
	// Without snippets, no templates.
	cc := completionAt(t, "fun f(b: Boolean) {\n    b.|\n}")
	items, _ := Complete(cc, &analysis.Completion{Reached: true, Receiver: boolean}, CompletionOptions{})
	if slices.ContainsFunc(items, func(it CompletionItem) bool { return it.Kind == "snippet" }) {
		t.Error("templates without snippets")
	}
}

func TestLiveTemplates(t *testing.T) {
	labels := func(src string) []string {
		cc := completionAt(t, src)
		items, _ := Complete(cc, &analysis.Completion{Reached: true}, CompletionOptions{Snippets: true})
		var out []string
		for _, it := range items {
			if it.Kind == "snippet" {
				out = append(out, it.Label)
			}
		}
		return out
	}
	if got := labels("fun f() {\n    so|\n}"); !slices.Equal(got, []string{"sout"}) {
		t.Errorf("statement: %v", got)
	}
	if got := labels("ma|"); !slices.Equal(got, []string{"main", "maina"}) {
		t.Errorf("top level: %v", got)
	}
	if got := labels("class C {\n    ma|\n}"); len(got) != 0 {
		t.Errorf("class body: %v", got)
	}
	if got := labels("fun f() {\n    |\n}"); len(got) != 0 {
		t.Errorf("nothing typed: %v", got)
	}
	if got := labels("fun f() = g(so|)"); len(got) != 0 {
		t.Errorf("argument: %v", got)
	}
}
