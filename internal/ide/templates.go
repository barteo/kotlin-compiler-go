package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Completion templates, as IntelliJ's: postfix templates after expr.,
// which rewrite expr (expr.if gives if (expr) { }), and live templates
// at the start of a statement or declaration (main, fun0, sout, iter).
// They are snippets, offered only to editors that take them, after the
// semantic items.

// template is a live template: its name, description and snippet.
type template struct{ name, detail, body string }

// Live templates by position: of statements, and of declarations at the
// top level or in a class.
var (
	statementTemplates = []template{
		{"sout", "println()", "println($0)"},
		{"serr", "System.err.println()", "System.err.println($0)"},
		{"iter", "for (item in collection)", "for (${1:item} in ${2:collection}) {\n    $0\n}"},
		{"fori", "for (i in 0 until n)", "for (${1:i} in 0 until ${2:n}) {\n    $0\n}"},
		{"ifn", "if (value == null)", "if (${1:value} == null) {\n    $0\n}"},
		{"inn", "if (value != null)", "if (${1:value} != null) {\n    $0\n}"},
		{"todo", "TODO()", "TODO(\"$0\")"},
	}
	functionTemplates = []template{
		{"fun0", "function, no parameters", "fun ${1:name}() {\n    $0\n}"},
		{"fun1", "function, one parameter", "fun ${1:name}(${2:p}: ${3:Any}) {\n    $0\n}"},
		{"fun2", "function, two parameters", "fun ${1:name}(${2:p1}: ${3:Any}, ${4:p2}: ${5:Any}) {\n    $0\n}"},
	}
	declarationTemplates = []template{
		{"exfun", "extension function", "fun ${1:Any}.${2:name}(${3}) {\n    $0\n}"},
		{"exval", "extension property", "val ${1:Any}.${2:name}: ${3:Int}\n    get() = $0"},
		{"singleton", "object", "object ${1:Name} {\n    $0\n}"},
		{"interface", "interface", "interface ${1:Name} {\n    $0\n}"},
		{"enum", "enum class", "enum class ${1:Name} {\n    $0\n}"},
		{"data", "data class", "data class ${1:Name}(${2:val p: Any})"},
	}
	topTemplates = []template{
		{"main", "fun main()", "fun main() {\n    $0\n}"},
		{"maina", "fun main(args)", "fun main(args: Array<String>) {\n    $0\n}"},
	}
)

// templates returns the templates that fit cc's caret: postfix ones after
// a value's dot, live ones where a statement or a declaration starts and
// a name is typed.
func (cc CompletionContext) templates(c *analysis.Completion, member, typePos bool) []CompletionItem {
	if typePos {
		return nil
	}
	if member {
		return cc.postfix(c)
	}
	if cc.Prefix == "" {
		return nil
	}
	var ts []template
	switch {
	case slices.Equal(cc.Keywords, statementKeywords):
		ts = append(slices.Clone(statementTemplates), functionTemplates...)
	case slices.Equal(cc.Keywords, memberKeywords):
		ts = append(slices.Clone(functionTemplates), declarationTemplates...)
	case slices.Equal(cc.Keywords, topKeywords):
		ts = append(append(slices.Clone(topTemplates), functionTemplates...), declarationTemplates...)
	}
	items := make([]CompletionItem, 0, len(ts))
	for _, t := range ts {
		items = append(items, CompletionItem{Label: t.name, Kind: "snippet", Detail: t.detail, Insert: t.body, Snippet: true})
	}
	return items
}

// postfixAt returns the receiver of a probe at off of snap that is the
// selector of expr.probe, and whether expr.probe is a statement of a
// block.
func postfixAt(snap *psi.Snapshot, off int) (psi.NodeRef, bool) {
	id := snap.NodeAt(off)
	ref := id.Parent()
	if id.NumericKind() != psi.KindIDENTIFIER || ref.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		return psi.NodeRef{}, false
	}
	q := ref.Parent()
	if q.NumericKind() != psi.KindDOT_QUALIFIED_EXPRESSION || lastExpression(q) != ref {
		return psi.NodeRef{}, false
	}
	recv := firstNonTrivia(q)
	if !recv.Valid() || recv == ref {
		return psi.NodeRef{}, false
	}
	p := q.Parent()
	return recv, p.NumericKind() == psi.KindBLOCK && p.Parent().NumericKind() != psi.KindFILE
}

// postfix returns the postfix templates for the receiver of cc's caret,
// by its type: c.Receiver.
func (cc CompletionContext) postfix(c *analysis.Completion) []CompletionItem {
	if !cc.recv.Valid() || c == nil || c.Receiver == nil || c.Safe || c.Static != "" || c.Package != "" {
		return nil
	}
	text := cc.snap.Source()
	start := cc.recv.Range().Start
	if start >= cc.Replace.Start || strings.ContainsAny(text[start:cc.Replace.End], "\r\n") {
		return nil // an editor's edit is of one line
	}
	t := *c.Receiver
	expr := strings.TrimSpace(text[start:cc.recv.Range().End])
	x := escapeSnippet(expr)
	px := x
	if !simpleOperand(cc.recv) {
		px = "(" + x + ")"
	}
	// bare is expr without its parentheses, where it stands alone.
	bare := x
	if cc.recv.NumericKind() == psi.KindPARENTHESIZED {
		if in := lastExpression(cc.recv); in.Valid() {
			bare = escapeSnippet(strings.TrimSpace(in.Text()))
		}
	}
	type pf struct {
		name, detail, body string
		ok                 bool
	}
	boolean := t.Name == "kotlin.Boolean" && !t.Nullable
	nullable := t.Nullable || t.Platform
	stmt := cc.recvStmt
	name := func() string {
		base := suggestName(cc.recv, t)
		if cc.recv.NumericKind() == psi.KindREFERENCE_EXPRESSION {
			base = typeName(t) // not the name it already has
		}
		return uniqueName(cc.recv, base)
	}
	var item string
	if stmt && iterable(t) && !t.Nullable {
		item = uniqueName(cc.recv, elementName(cc.recv, t))
	}
	all := []pf{
		{"if", "if (expr) {}", "if (" + bare + ") {\n    $0\n}", stmt && boolean},
		{"else", "if (!expr) {}", "if (!" + px + ") {\n    $0\n}", stmt && boolean},
		{"not", "!expr", "!" + px + "$0", boolean},
		{"for", "for (item in expr) {}", "for (${1:" + item + "} in " + bare + ") {\n    $0\n}", item != ""},
		{"nn", "if (expr != null) {}", "if (" + bare + " != null) {\n    $0\n}", stmt && nullable},
		{"notnull", "if (expr != null) {}", "if (" + bare + " != null) {\n    $0\n}", stmt && nullable},
		{"null", "if (expr == null) {}", "if (" + bare + " == null) {\n    $0\n}", stmt && nullable},
		{"val", "val name = expr", "", stmt},
		{"var", "var name = expr", "", stmt},
		{"return", "return expr", "return " + bare + "$0", stmt},
		{"when", "when (expr) {}", "when (" + bare + ") {\n    $0\n}", stmt},
		{"let", "expr.let {}", x + ".let { $0 }", true},
		{"also", "expr.also {}", x + ".also { $0 }", true},
		{"print", "println(expr)", "println(" + bare + ")$0", true},
		{"par", "(expr)", "(" + x + ")$0", true},
	}
	edit := psi.Range{Start: start, End: cc.Replace.End}
	prefix := text[start:cc.Replace.Start]
	var items []CompletionItem
	for _, p := range all {
		if !p.ok {
			continue
		}
		body := p.body
		if p.name == "val" || p.name == "var" {
			body = p.name + " ${1:" + name() + "} = " + bare + "$0"
		}
		items = append(items, CompletionItem{Label: p.name, Kind: "snippet", Detail: p.detail, Insert: body, Snippet: true,
			Filter: prefix + p.name, Edit: &edit})
	}
	return items
}

// iterable reports whether a for loop takes a value of type t.
func iterable(t analysis.Type) bool {
	switch t.Name {
	case "kotlin.Array", "kotlin.IntArray", "kotlin.LongArray", "kotlin.ShortArray", "kotlin.ByteArray", "kotlin.CharArray",
		"kotlin.DoubleArray", "kotlin.FloatArray", "kotlin.BooleanArray", "kotlin.String", "kotlin.CharSequence",
		"kotlin.collections.Iterable", "kotlin.collections.Map", "kotlin.collections.MutableMap",
		"kotlin.collections.Iterator", "kotlin.sequences.Sequence":
		return true
	}
	for _, s := range t.Supertypes {
		switch s {
		case "kotlin.collections.Iterable", "kotlin.collections.Map", "kotlin.sequences.Sequence", "kotlin.CharSequence", "kotlin.collections.Iterator":
			return true
		}
	}
	return false
}

// elementName returns a name for the elements of x, of type t: a plural
// name's singular, or the element type's name.
func elementName(x psi.NodeRef, t analysis.Type) string {
	if len(t.Arguments) == 1 && t.Arguments[0].Name != "" && t.Arguments[0].Projection != "*" {
		if n := suggestName(x, t); len(n) > 1 && strings.HasSuffix(n, "s") && !strings.HasSuffix(n, "ss") {
			return strings.TrimSuffix(n, "s")
		}
		return typeName(t.Arguments[0])
	}
	if t.Name == "kotlin.String" || t.Name == "kotlin.CharSequence" || t.Name == "kotlin.CharArray" {
		return "c"
	}
	return "item"
}

// simpleOperand reports whether x needs no parentheses as a prefix
// operator's operand.
func simpleOperand(x psi.NodeRef) bool {
	switch x.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION, psi.KindCALL_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION,
		psi.KindPARENTHESIZED, psi.KindTHIS_EXPRESSION, psi.KindBOOLEAN_CONSTANT, psi.KindARRAY_ACCESS_EXPRESSION:
		return true
	}
	return false
}

// escapeSnippet escapes text for a snippet's literal text.
func escapeSnippet(s string) string {
	return strings.NewReplacer(`\`, `\\`, `$`, `\$`, `}`, `\}`).Replace(s)
}
