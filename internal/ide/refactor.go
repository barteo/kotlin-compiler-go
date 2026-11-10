package ide

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// The code action kinds of refactorings.
const (
	RefactorExtract = "refactor.extract"
	RefactorInline  = "refactor.inline"
	RefactorRewrite = "refactor.rewrite"
)

// Refactors returns the refactorings and intentions that apply at rng of
// doc, a selection or a caret: extract variable, inline variable,
// specify or remove a declaration's type, add names to a call's
// arguments, and convert a function's body between expression and
// block. They read v, the module's view, which must have analyzed doc's
// text; with none, or another text, there are none.
//
// Each is conservative: one that might change what the program means,
// or not compile, is left out rather than offered.
func Refactors(v *View, doc *Document, rng psi.Range, env FixEnv) []Fix {
	if v == nil {
		return nil
	}
	snap := v.Snapshot(doc.Path)
	if snap == nil || snap.Source() != doc.Snap.Source() {
		return nil
	}
	r := refactorer{fixer{v: v, doc: doc, snap: snap, d: Diagnostic{Range: rng}}}
	var out []Fix
	for _, f := range []func() []Fix{r.extract, r.inline, r.specifyType, r.removeType, r.argumentNames, r.toExpressionBody, r.toBlockBody, r.rewrites} {
		out = append(out, f()...)
	}
	return out
}

type refactorer struct{ fixer }

func (r refactorer) refactor(kind, title string, edits ...FixEdit) []Fix {
	return []Fix{{Title: title, Kind: kind, Edits: edits}}
}

// caret returns the token the caret, the range's start, is on.
func (r refactorer) caret() psi.NodeRef { return caretLeaf(r.snap, r.d.Range.Start) }

// header returns the innermost declaration of one of kinds whose header
// holds the caret: from its start to its body, or initializer.
func (r refactorer) header(kinds ...psi.Kind) psi.NodeRef {
	for n := r.caret(); n.Valid(); n = n.Parent() {
		for _, k := range kinds {
			if n.NumericKind() != k {
				continue
			}
			end := n.Range().End
			if b := n.Body(); b.Valid() {
				end = b.Range().Start
			}
			if eq := n.ChildKind(psi.KindEQ); eq.Valid() {
				end = eq.Range().Start
			}
			if r.d.Range.Start <= end {
				return n
			}
			return psi.NodeRef{}
		}
		switch n.NumericKind() {
		case psi.KindBLOCK, psi.KindCLASS_BODY, psi.KindLAMBDA_EXPRESSION:
			return psi.NodeRef{}
		}
	}
	return psi.NodeRef{}
}

// typeText returns t as the file writes it, with the imports it needs.
func (r refactorer) typeText(t analysis.Type) (string, []FixEdit, bool) {
	if !r.nameable(t) {
		return "", nil, false
	}
	sh := r.shortener()
	s := sh.short(qualifiedType(t))
	if !validType(s) {
		return "", nil, false
	}
	return s, sh.imports(), true
}

// nameable reports whether the classes t names can be written: none is
// local, anonymous or of an enum entry's body.
func (r refactorer) nameable(t analysis.Type) bool {
	if t.Projection == "*" {
		return true
	}
	if t.Name == "" || strings.ContainsAny(t.Name, "<>$ ") {
		return false
	}
	if !t.Parameter {
		segs := strings.Split(t.Name, ".")
		upper := false
		for _, s := range segs {
			if s == "" {
				return false
			}
			c := s[0]
			if upper && ('a' <= c && c <= 'z' || strings.ToUpper(s) == s && len(s) > 1 && s != "IO") {
				return false // a facade's local class, or an enum entry's
			}
			upper = upper || 'A' <= c && c <= 'Z'
		}
		r.v.classIndex()
		if el, ok := r.v.classes[t.Name]; ok {
			d := r.v.node(el)
			if len(t.Arguments) > 0 && d.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindINNER_KEYWORD).Valid() {
				return false // Outer<A>.Inner, which the type names Outer.Inner<A>
			}
			for n := d.Parent(); n.Valid(); n = n.Parent() {
				switch n.NumericKind() {
				case psi.KindBLOCK, psi.KindOBJECT_LITERAL, psi.KindENUM_ENTRY:
					return false
				}
			}
		}
	}
	for _, a := range t.Arguments {
		if !r.nameable(a) {
			return false
		}
	}
	return true
}

// declType returns the type of declaration d, from its symbol.
func (r refactorer) declType(d psi.NodeRef) (analysis.Type, bool) {
	f, ok := r.v.fact(d)
	if !ok {
		return analysis.Type{}, false
	}
	s, ok := r.v.symbol(f.Symbol)
	return s.Type, ok && s.Type.Name != ""
}

// specifyType writes the inferred type of a property, or the return type
// of a function with an expression body.
func (r refactorer) specifyType() []Fix {
	d := r.header(psi.KindPROPERTY, psi.KindFUN)
	if !d.Valid() || d.TypeReference().Valid() || !d.ChildKind(psi.KindEQ).Valid() || d.ChildKind(psi.KindPROPERTY_DELEGATE).Valid() {
		return nil
	}
	at := nameRange(d).End
	title := "Specify type explicitly"
	if d.NumericKind() == psi.KindFUN {
		ps := d.ChildKind(psi.KindVALUE_PARAMETER_LIST)
		if !ps.Valid() {
			return nil
		}
		at = ps.Range().End
		title = "Specify return type explicitly"
	} else if !d.NameIdentifier().Valid() {
		return nil
	}
	t, ok := r.declType(d)
	if !ok {
		return nil
	}
	s, imports, ok := r.typeText(t)
	if !ok {
		return nil
	}
	return r.refactor(RefactorRewrite, title, append([]FixEdit{r.edit(psi.Range{Start: at, End: at}, ": "+s)}, imports...)...)
}

// removeType removes a property's declared type when its initializer has
// that type on its own: a literal, a reference, or a call of a callee
// without type parameters.
func (r refactorer) removeType() []Fix {
	d := r.header(psi.KindPROPERTY)
	tr := d.TypeReference()
	init := d.Body()
	if !tr.Valid() || !init.Valid() || !d.NameIdentifier().Valid() {
		return nil
	}
	ft, ok1 := r.v.fact(tr)
	fi, ok2 := r.v.fact(init)
	if !ok1 || !ok2 || ft.Type == nil || fi.Type == nil || !standalone(r.v, init) {
		return nil
	}
	if qualifiedType(*ft.Type) != qualifiedType(*fi.Type) || fi.Type.Platform || fi.FlowNonNull || fi.DefinitelyNonNull {
		return nil
	}
	return r.refactor(RefactorRewrite, "Remove explicit type specification", r.edit(psi.Range{Start: nameRange(d).End, End: tr.Range().End}, ""))
}

// standalone reports whether expression x has its type without an
// expected type: a literal, a string, a reference, or a call of a callee
// without type parameters, or with type arguments written.
func standalone(v *View, x psi.NodeRef) bool {
	if !x.Valid() {
		return false
	}
	switch x.NumericKind() {
	case psi.KindSTRING_TEMPLATE, psi.KindBOOLEAN_CONSTANT, psi.KindCHARACTER_CONSTANT, psi.KindTHIS_EXPRESSION:
		return true
	case psi.KindINTEGER_CONSTANT, psi.KindFLOAT_CONSTANT:
		f, ok := v.fact(x)
		if !ok || f.Type == nil {
			return false
		}
		t := strings.ToLower(x.Text())
		switch f.Type.Name {
		case "kotlin.Int":
			return !strings.HasSuffix(t, "l") && !strings.HasPrefix(t, "-")
		case "kotlin.Long":
			return strings.HasSuffix(t, "l")
		case "kotlin.Double":
			return !strings.HasSuffix(t, "f")
		case "kotlin.Float":
			return strings.HasSuffix(t, "f")
		}
		return false
	case psi.KindREFERENCE_EXPRESSION:
		f, ok := v.fact(x)
		return ok && f.Call == nil && f.Type != nil && !f.Classifier
	case psi.KindPARENTHESIZED:
		for c := x.FirstChild(); c.Valid(); c = c.NextSibling() {
			if !c.IsLeaf() {
				return standalone(v, c)
			}
		}
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
		return standalone(v, lastExpression(x))
	case psi.KindCALL_EXPRESSION:
		if x.ChildKind(psi.KindTYPE_ARGUMENT_LIST).Valid() {
			return true
		}
		f, ok := v.fact(x.ChildKind(psi.KindREFERENCE_EXPRESSION))
		if !ok || f.Call == nil {
			f, ok = v.fact(x)
		}
		if !ok || f.Call == nil || f.Call.TypeParameters > 0 || f.Call.ValueInvocation ||
			f.Call.DeclaredReturnType != nil && hasParameter(*f.Call.DeclaredReturnType) {
			return false
		}
		// A generic class's constructor infers its type arguments; a
		// callee the checker knows by itself has no declared type.
		sym, _ := v.symbol(f.Symbol)
		if sym.Kind != "constructor" {
			return f.Call.DeclaredReturnType != nil
		}
		return f.Call.ReturnType == nil || len(f.Call.ReturnType.Arguments) == 0
	}
	return false
}

// hasParameter reports whether t is or has a type parameter.
func hasParameter(t analysis.Type) bool {
	if t.Parameter {
		return true
	}
	for _, a := range t.Arguments {
		if hasParameter(a) {
			return true
		}
	}
	return false
}

// lastExpression returns the last child of n that isn't a token.
func lastExpression(n psi.NodeRef) psi.NodeRef {
	var last psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsLeaf() && !trivia(c) {
			last = c
		}
	}
	return last
}

// selected returns the expression the range selects, trimmed of blanks,
// or for a caret the expression it is on: a call rather than its callee,
// a qualified expression rather than its selector.
func (r refactorer) selected() psi.NodeRef {
	src := r.snap.Source()
	start, end := r.d.Range.Start, r.d.Range.End
	for start < end && unicode.IsSpace(rune(src[start])) {
		start++
	}
	for end > start && unicode.IsSpace(rune(src[end-1])) {
		end--
	}
	if start < end {
		var x psi.NodeRef
		for n := r.snap.NodeAt(start); n.Valid() && n.Range().Start == start && n.Range().End <= end; n = n.Parent() {
			if n.Range().End == end && expressionKinds[n.NumericKind()] {
				x = n
			}
		}
		return x
	}
	n := r.caret()
	for n.Valid() && !expressionKinds[n.NumericKind()] {
		if n.NumericKind() == psi.KindBLOCK || n.NumericKind() == psi.KindFILE {
			return psi.NodeRef{}
		}
		n = n.Parent()
	}
	for n.Valid() {
		p := n.Parent()
		switch p.NumericKind() {
		case psi.KindCALL_EXPRESSION:
			if p.FirstChild() == n {
				n = p
				continue
			}
		case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION, psi.KindCALLABLE_REFERENCE_EXPRESSION:
			if p.FirstChild() != n {
				n = p
				continue
			}
		}
		break
	}
	return n
}

// statement returns the statement of a block that holds x, if evaluating
// x before it doesn't change what the code means: x isn't in a branch,
// a loop's body or condition, the right of && || ?:, a safe call's
// selector, or a nested declaration or lambda.
func statement(x psi.NodeRef) psi.NodeRef {
	n := x
	for {
		p := n.Parent()
		if !p.Valid() {
			return psi.NodeRef{}
		}
		if p.NumericKind() == psi.KindBLOCK {
			if p.Parent().NumericKind() == psi.KindFILE {
				return psi.NodeRef{} // a script's
			}
			return n
		}
		if crosses(p, n) {
			return psi.NodeRef{}
		}
		n = p
	}
}

// crosses reports whether child c of p may be evaluated other than once,
// or in another scope, when p is.
func crosses(p, c psi.NodeRef) bool {
	switch p.NumericKind() {
	case psi.KindTHEN, psi.KindELSE, psi.KindBODY, psi.KindWHEN_ENTRY, psi.KindFUN, psi.KindPROPERTY_ACCESSOR,
		psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindOBJECT_LITERAL, psi.KindCLASS_BODY, psi.KindANNOTATION_ENTRY,
		psi.KindVALUE_PARAMETER, psi.KindFUNCTION_LITERAL, psi.KindLAMBDA_EXPRESSION, psi.KindSECONDARY_CONSTRUCTOR,
		psi.KindCLASS_INITIALIZER, psi.KindFILE, psi.KindPROPERTY_DELEGATE:
		return true
	case psi.KindCONDITION:
		k := p.Parent().NumericKind()
		return k == psi.KindWHILE || k == psi.KindDO_WHILE
	case psi.KindBINARY_EXPRESSION:
		switch p.ChildKind(psi.KindOPERATION_REFERENCE).Text() {
		case "&&", "||", "?:":
			return firstOperand(p) != c
		}
	case psi.KindSAFE_ACCESS_EXPRESSION:
		return p.FirstChild() != c
	case psi.KindPROPERTY:
		// A member's or a top-level property's initializer.
		return p.Parent().NumericKind() != psi.KindBLOCK
	}
	return false
}

func firstOperand(p psi.NodeRef) psi.NodeRef {
	for c := p.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsLeaf() && !trivia(c) {
			return c
		}
	}
	return psi.NodeRef{}
}

// assignOps are the operators of assignments.
var assignOps = map[string]bool{"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true}

// extract puts the selected expression in a new local variable, declared
// before the statement that holds it.
func (r refactorer) extract() []Fix {
	x := r.selected()
	if !x.Valid() || selector(x) {
		return nil
	}
	if p := x.Parent(); p.NumericKind() == psi.KindCALL_EXPRESSION && p.FirstChild() == x {
		return nil // a callee
	}
	switch x.NumericKind() {
	case psi.KindLAMBDA_EXPRESSION, psi.KindCALLABLE_REFERENCE_EXPRESSION, psi.KindNULL:
		return nil
	}
	f, ok := r.v.fact(x)
	if !ok || f.Type == nil || f.Classifier || f.Role == "type" || f.Type.Name == "kotlin.Unit" || f.Type.Name == "kotlin.Nothing" || f.Type.Name == "" {
		return nil
	}
	switch p := x.Parent(); p.NumericKind() {
	case psi.KindBINARY_EXPRESSION:
		if assignOps[p.ChildKind(psi.KindOPERATION_REFERENCE).Text()] && firstOperand(p) == x {
			return nil
		}
	case psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION:
		if op := p.ChildKind(psi.KindOPERATION_REFERENCE).Text(); op == "++" || op == "--" {
			return nil
		}
	}
	if castSubject(x) {
		return nil
	}
	stmt := statement(x)
	if !stmt.Valid() || stmt == x {
		return nil
	}
	if !standalone(r.v, x) {
		switch x.NumericKind() {
		case psi.KindINTEGER_CONSTANT, psi.KindFLOAT_CONSTANT, psi.KindIF, psi.KindWHEN, psi.KindTRY, psi.KindCALL_EXPRESSION,
			psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION, psi.KindPREFIX_EXPRESSION:
			// Its type may come from where it is: say it.
			if f.Type.Platform {
				return nil
			}
			s, imports, ok := r.typeText(*f.Type)
			if !ok {
				return nil
			}
			return r.extractWith(x, stmt, f, ": "+s, imports)
		}
	}
	return r.extractWith(x, stmt, f, "", nil)
}

// castSubject reports whether x is checked by an is or compared, which
// may smart cast it, or what it is compared with, where a variable of it
// wouldn't be.
func castSubject(x psi.NodeRef) bool {
	switch p := x.Parent(); p.NumericKind() {
	case psi.KindIS_EXPRESSION, psi.KindBINARY_WITH_TYPE:
		return firstOperand(p) == x
	case psi.KindBINARY_EXPRESSION:
		switch p.ChildKind(psi.KindOPERATION_REFERENCE).Text() {
		case "==", "!=", "===", "!==":
			return true
		}
	case psi.KindPOSTFIX_EXPRESSION:
		return p.ChildKind(psi.KindOPERATION_REFERENCE).Text() == "!!"
	}
	return false
}

func (r refactorer) extractWith(x, stmt psi.NodeRef, f analysis.Fact, decl string, imports []FixEdit) []Fix {
	src := r.snap.Source()
	name := uniqueName(stmt, suggestName(x, *f.Type))
	intro := "val " + name + decl + " = " + x.Text() + "\n" + lineIndent(src, stmt.Range().Start)
	var edits []FixEdit
	if x.Range().Start == stmt.Range().Start {
		edits = append(edits, r.edit(x.Range(), intro+name))
	} else {
		at := stmt.Range().Start
		edits = append(edits, r.edit(psi.Range{Start: at, End: at}, intro), r.edit(x.Range(), name))
	}
	return r.refactor(RefactorExtract, "Extract variable", append(edits, imports...)...)
}

// suggestName returns a name for a variable of x's value, of type t:
// of what x calls or reads, else of t.
func suggestName(x psi.NodeRef, t analysis.Type) string {
	n := x
	for {
		switch n.NumericKind() {
		case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
			n = lastExpression(n)
			continue
		case psi.KindCALL_EXPRESSION:
			n = n.FirstChild()
			continue
		case psi.KindPARENTHESIZED:
			n = lastExpression(n)
			continue
		}
		break
	}
	if n.NumericKind() == psi.KindREFERENCE_EXPRESSION {
		s := n.Text()
		for _, p := range []string{"get", "is", "to", "as"} {
			if rest, ok := strings.CutPrefix(s, p); ok && rest != "" && unicode.IsUpper(rune(rest[0])) {
				s = rest
				break
			}
		}
		if s = decapitalize(s); identifier(s) {
			return s
		}
	}
	return typeName(t)
}

// typeName returns a name for a value of type t.
func typeName(t analysis.Type) string {
	simple := simpleName(t.Name)
	switch simple {
	case "Int", "Short", "Byte":
		return "i"
	case "Long":
		return "l"
	case "Double", "Float":
		return "d"
	case "Boolean":
		return "b"
	case "Char":
		return "c"
	case "String", "CharSequence":
		return "s"
	case "List", "MutableList", "ArrayList", "Set", "MutableSet", "HashSet", "Collection", "Iterable", "Array", "Sequence":
		if len(t.Arguments) == 1 && t.Arguments[0].Name != "" && t.Arguments[0].Projection != "*" {
			if s := typeName(t.Arguments[0]) + "s"; identifier(s) {
				return s // not "is", of Ints
			}
		}
	}
	if _, ok := functionArity(t.Name); ok {
		return "function"
	}
	if t.Parameter || simple == "" {
		return "value"
	}
	if s := decapitalize(simple); identifier(s) {
		return s
	}
	return "value"
}

func decapitalize(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	// URL -> url, HTMLParser -> htmlParser
	i := 0
	for i < len(s) && 'A' <= s[i] && s[i] <= 'Z' {
		i++
	}
	if i > 1 && i < len(s) {
		return strings.ToLower(s[:i-1]) + s[i-1:]
	}
	if i > 1 {
		return strings.ToLower(s)
	}
	return string(unicode.ToLower(r)) + s[n:]
}

// identifier reports whether s is a name that needs no backticks.
func identifier(s string) bool {
	if s == "" || hardKeywords[s] {
		return false
	}
	for i, c := range s {
		if !(c == '_' || unicode.IsLetter(c) || i > 0 && unicode.IsDigit(c)) {
			return false
		}
	}
	return true
}

var hardKeywords = map[string]bool{
	"as": true, "break": true, "class": true, "continue": true, "do": true, "else": true, "false": true, "for": true,
	"fun": true, "if": true, "in": true, "interface": true, "is": true, "null": true, "object": true, "package": true,
	"return": true, "super": true, "this": true, "throw": true, "true": true, "try": true, "typealias": true,
	"typeof": true, "val": true, "var": true, "when": true, "while": true,
}

// uniqueName returns base, or base with a number, a name no token of
// the top-level declaration that holds n is but a qualified selector's,
// so that a variable of it shadows nothing and nothing shadows it.
func uniqueName(n psi.NodeRef, base string) string {
	top := n
	for top.Valid() && top.Parent().Valid() && top.Parent().NumericKind() != psi.KindFILE {
		top = top.Parent()
	}
	used := map[string]bool{}
	psi.WalkNodes(top, func(m psi.NodeRef) bool {
		if m.NumericKind() == psi.KindIDENTIFIER && !selector(m.Parent()) {
			used[strings.Trim(m.Text(), "`")] = true
		}
		return true
	})
	name := base
	for i := 1; used[name]; i++ {
		name = base + strconv.Itoa(i)
	}
	return name
}

// selector reports whether n is a reference or a call after a
// qualified expression's dot, or a call's callee there.
func selector(n psi.NodeRef) bool {
	switch n.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION:
		if p := n.Parent(); p.NumericKind() == psi.KindCALL_EXPRESSION && p.FirstChild() == n {
			n = p
		}
	case psi.KindCALL_EXPRESSION:
	default:
		return false
	}
	switch p := n.Parent(); p.NumericKind() {
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION, psi.KindCALLABLE_REFERENCE_EXPRESSION:
		return p.FirstChild() != n
	}
	return false
}

// inline replaces the uses of a local val by its initializer, and
// removes it: when the initializer is a constant or a val's name, or the
// val is used once, in the next statement, where nothing but the
// statement's own evaluation is between.
func (r refactorer) inline() []Fix {
	d := r.inlined()
	if !d.Valid() || d.Parent().NumericKind() != psi.KindBLOCK || !d.ChildKind(psi.KindVAL_KEYWORD).Valid() ||
		d.ChildKind(psi.KindMODIFIER_LIST).Valid() || d.ChildKind(psi.KindPROPERTY_DELEGATE).Valid() || !d.NameIdentifier().Valid() {
		return nil
	}
	init := d.Body()
	if !init.Valid() || init.NumericKind() == psi.KindNULL {
		return nil
	}
	if tr := d.TypeReference(); tr.Valid() {
		ft, ok1 := r.v.fact(tr)
		fi, ok2 := r.v.fact(init)
		if !ok1 || !ok2 || ft.Type == nil || fi.Type == nil || qualifiedType(*ft.Type) != qualifiedType(*fi.Type) || !standalone(r.v, init) {
			return nil
		}
	}
	uses := r.v.uses(d)
	if len(uses) == 0 {
		return nil
	}
	var nodes []psi.NodeRef
	for _, u := range uses {
		if u.snap != r.snap {
			return nil
		}
		nodes = append(nodes, u.snap.Node(u.node))
	}
	if !r.pure(init, nodes) {
		if len(nodes) != 1 || flowEffects(init) {
			return nil
		}
		if f, ok := r.v.fact(init); ok && f.Classifier {
			return nil
		}
		next := d.NextSibling()
		for next.Valid() && (next.IsLeaf() || trivia(next)) {
			if next.NumericKind() == psi.KindSEMICOLON || strings.TrimSpace(next.Text()) == "" || trivia(next) {
				next = next.NextSibling()
				continue
			}
			return nil
		}
		if s := statement(nodes[0]); !s.Valid() || s != next {
			return nil
		}
	}
	text := init.Text()
	edits := []FixEdit{r.edit(lineRange(r.snap.Source(), d.Range()), "")}
	for _, u := range nodes {
		if p := u.Parent(); p.NumericKind() == psi.KindSHORT_STRING_TEMPLATE_ENTRY {
			edits = append(edits, r.edit(p.Range(), "${"+text+"}"))
			continue
		}
		if p := u.Parent(); p.NumericKind() == psi.KindCALL_EXPRESSION && p.FirstChild() == u {
			return nil // a function value's invocation
		}
		t := text
		if needsParens(init, u) {
			t = "(" + text + ")"
		}
		edits = append(edits, r.edit(u.Range(), t))
	}
	return r.refactor(RefactorInline, "Inline variable '"+d.Name()+"'", edits...)
}

// flowEffects reports whether evaluating x may smart cast what is
// evaluated after it: x casts, checks a type, asserts not null, or
// jumps.
func flowEffects(x psi.NodeRef) bool {
	found := false
	psi.WalkNodes(x, func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindBINARY_WITH_TYPE, psi.KindIS_EXPRESSION, psi.KindRETURN, psi.KindTHROW, psi.KindBREAK, psi.KindCONTINUE,
			psi.KindEXCLEXCL, psi.KindWHEN_CONDITION_IS_PATTERN:
			found = true
		}
		return !found
	})
	return found
}

// inlined returns the local property whose name the caret is on, or whose
// use it is on.
func (r refactorer) inlined() psi.NodeRef {
	leaf := r.caret()
	if leaf.NumericKind() != psi.KindIDENTIFIER {
		return psi.NodeRef{}
	}
	p := leaf.Parent()
	if p.NumericKind() == psi.KindPROPERTY && p.NameIdentifier() == leaf {
		return p
	}
	if p.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		return psi.NodeRef{}
	}
	f, ok := r.v.fact(p)
	if !ok || f.Call != nil {
		return psi.NodeRef{}
	}
	s, ok := r.v.symbol(f.Symbol)
	if !ok || s.Kind != "local" || s.Declaration == nil {
		return psi.NodeRef{}
	}
	if d := r.v.node(*s.Declaration); d.NumericKind() == psi.KindPROPERTY && d.ContainingFile() == r.snap {
		return d
	}
	return psi.NodeRef{}
}

// pure reports whether init's value is the same wherever and however
// often it is evaluated at uses: a constant, a string without templates,
// or the name of a val or a parameter that each use sees too.
func (r refactorer) pure(init psi.NodeRef, uses []psi.NodeRef) bool {
	switch init.NumericKind() {
	case psi.KindINTEGER_CONSTANT, psi.KindFLOAT_CONSTANT, psi.KindBOOLEAN_CONSTANT, psi.KindCHARACTER_CONSTANT:
		return true
	case psi.KindSTRING_TEMPLATE:
		for c := init.FirstChild(); c.Valid(); c = c.NextSibling() {
			if k := c.NumericKind(); k == psi.KindSHORT_STRING_TEMPLATE_ENTRY || k == psi.KindLONG_STRING_TEMPLATE_ENTRY {
				return false
			}
		}
		return true
	case psi.KindREFERENCE_EXPRESSION:
		f, ok := r.v.fact(init)
		if !ok || f.Call != nil {
			return false
		}
		s, ok := r.v.symbol(f.Symbol)
		if !ok || s.Declaration == nil || (s.Kind != "local" && s.Kind != "parameter") {
			return false
		}
		d := r.v.node(*s.Declaration)
		if d.NumericKind() == psi.KindPROPERTY && !d.ChildKind(psi.KindVAL_KEYWORD).Valid() {
			return false
		}
		name := init.Text()
		if lookup(init, name) != d {
			return false
		}
		for _, u := range uses {
			if lookup(u, name) != d {
				return false
			}
		}
		return true
	}
	return false
}

// needsParens reports whether init, put in place of use u, needs
// parentheses to keep its meaning.
func needsParens(init, u psi.NodeRef) bool {
	switch init.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION, psi.KindCALL_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindPARENTHESIZED,
		psi.KindSTRING_TEMPLATE, psi.KindINTEGER_CONSTANT, psi.KindFLOAT_CONSTANT, psi.KindBOOLEAN_CONSTANT,
		psi.KindCHARACTER_CONSTANT, psi.KindTHIS_EXPRESSION, psi.KindARRAY_ACCESS_EXPRESSION, psi.KindNULL:
		if !strings.HasPrefix(init.Text(), "-") {
			return false
		}
	}
	switch u.Parent().NumericKind() {
	case psi.KindVALUE_ARGUMENT, psi.KindPARENTHESIZED, psi.KindLONG_STRING_TEMPLATE_ENTRY, psi.KindINDICES, psi.KindCONDITION:
		return false
	case psi.KindPROPERTY, psi.KindRETURN:
		return init.NumericKind() == psi.KindLAMBDA_EXPRESSION && u.Parent().NumericKind() == psi.KindRETURN
	}
	return true
}

// argumentNames names the arguments of the call the caret is in: the one
// the caret is on, and all of them. A Java callee's, a vararg's and a
// function value's can't be named.
func (r refactorer) argumentNames() []Fix {
	var args psi.NodeRef
	for n := r.caret(); n.Valid(); n = n.Parent() {
		if n.NumericKind() == psi.KindVALUE_ARGUMENT_LIST && n.Parent().NumericKind() == psi.KindCALL_EXPRESSION {
			args = n
			break
		}
		if k := n.NumericKind(); k == psi.KindBLOCK || k == psi.KindLAMBDA_EXPRESSION || k == psi.KindCLASS_BODY {
			return nil
		}
	}
	if !args.Valid() {
		return nil
	}
	call := args.Parent()
	f, ok := r.v.fact(call.ChildKind(psi.KindREFERENCE_EXPRESSION))
	if !ok || f.Call == nil {
		f, ok = r.v.fact(call)
	}
	if !ok || f.Call == nil || f.Call.ValueInvocation || f.Call.FromJava {
		return nil
	}
	c := f.Call
	if !calleeIs(c, call) {
		return nil // a call the checker rewrote, as a safe call into a let
	}
	seen := map[string]bool{}
	for _, n := range c.ParameterNames {
		if seen[n] {
			return nil // not the declared names
		}
		seen[n] = true
	}
	var all []FixEdit
	var mine []FixEdit
	mineName := ""
	i := 0
	for a := args.FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() != psi.KindVALUE_ARGUMENT {
			continue
		}
		k := i
		i++
		if a.ChildKind(psi.KindVALUE_ARGUMENT_NAME).Valid() {
			continue
		}
		if k >= len(c.ArgumentMapping) || strings.HasPrefix(strings.TrimSpace(a.Text()), "*") {
			break
		}
		p := c.ArgumentMapping[k]
		if p != k || p >= len(c.ParameterNames) || p < len(c.Varargs) && c.Varargs[p] || !identifier(c.ParameterNames[p]) {
			break
		}
		e := r.edit(psi.Range{Start: a.Range().Start, End: a.Range().Start}, c.ParameterNames[p]+" = ")
		all = append(all, e)
		if a.Range().Start <= r.d.Range.Start && r.d.Range.Start <= a.Range().End {
			mine = []FixEdit{e}
			mineName = c.ParameterNames[p]
		}
	}
	var out []Fix
	if len(mine) > 0 && len(all) > 1 {
		out = append(out, r.refactor(RefactorRewrite, "Add '"+mineName+" =' to argument", mine...)...)
	}
	if len(all) > 0 {
		out = append(out, r.refactor(RefactorRewrite, "Add names to call arguments", all...)...)
	}
	return out
}

// calleeIs reports whether c, a call's fact, is of the function call
// names, or of the constructor of the class it names.
func calleeIs(c *analysis.Call, call psi.NodeRef) bool {
	name := psi.Unquote(strings.TrimSpace(call.ChildKind(psi.KindREFERENCE_EXPRESSION).Text()))
	q := strings.TrimSuffix(c.Name, ".<init>")
	return name != "" && (q == name || strings.HasSuffix(q, "."+name))
}

// function returns the innermost function the caret is in, outside its
// lambdas and nested classes.
func (r refactorer) function() psi.NodeRef {
	for n := r.caret(); n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindFUN:
			return n
		case psi.KindLAMBDA_EXPRESSION, psi.KindCLASS_BODY, psi.KindOBJECT_LITERAL:
			return psi.NodeRef{}
		}
	}
	return psi.NodeRef{}
}

// toExpressionBody converts a function whose block holds only return e,
// or only e of type Unit, to = e.
func (r refactorer) toExpressionBody() []Fix {
	fn := r.function()
	block := fn.ChildKind(psi.KindBLOCK)
	if !block.Valid() {
		return nil
	}
	var stmt psi.NodeRef
	for c := block.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch {
		case c.NumericKind() == psi.KindEOL_COMMENT || c.NumericKind() == psi.KindBLOCK_COMMENT || c.NumericKind() == psi.KindSEMICOLON:
			return nil
		case c.IsLeaf():
		case stmt.Valid():
			return nil
		default:
			stmt = c
		}
	}
	if !stmt.Valid() {
		return nil
	}
	var e psi.NodeRef
	rt := fn.ReturnType()
	if stmt.NumericKind() == psi.KindRETURN {
		if stmt.ChildKind(psi.KindLABEL_QUALIFIER).Valid() || !rt.Valid() {
			return nil
		}
		e = lastExpression(stmt)
	} else {
		if rt.Valid() && rt.Text() != "Unit" || !valueExpression(stmt) || returns(stmt) {
			return nil
		}
		f, ok := r.v.fact(stmt)
		if !ok || f.Type == nil || f.Type.Name != "kotlin.Unit" {
			return nil
		}
		e = stmt
	}
	if !e.Valid() {
		return nil
	}
	return r.refactor(RefactorRewrite, "Convert to expression body", r.edit(block.Range(), "= "+e.Text()))
}

// returns reports whether n holds a return of its function.
func returns(n psi.NodeRef) bool {
	found := false
	psi.WalkNodes(n, func(m psi.NodeRef) bool {
		switch m.NumericKind() {
		case psi.KindRETURN:
			found = found || !m.ChildKind(psi.KindLABEL_QUALIFIER).Valid()
		case psi.KindFUN, psi.KindLAMBDA_EXPRESSION, psi.KindCLASS, psi.KindOBJECT_LITERAL:
			return false
		}
		return !found
	})
	return found
}

// valueExpression reports whether statement s may be an expression body.
func valueExpression(s psi.NodeRef) bool {
	switch s.NumericKind() {
	case psi.KindIF:
		return s.ChildKind(psi.KindELSE).Valid()
	case psi.KindWHEN:
		for e := s.FirstChild(); e.Valid(); e = e.NextSibling() {
			if e.NumericKind() == psi.KindWHEN_ENTRY && e.ChildKind(psi.KindELSE_KEYWORD).Valid() {
				return true
			}
		}
		return false
	case psi.KindBINARY_EXPRESSION:
		return !assignOps[s.ChildKind(psi.KindOPERATION_REFERENCE).Text()]
	}
	return expressionKinds[s.NumericKind()] && s.NumericKind() != psi.KindLAMBDA_EXPRESSION
}

// toBlockBody converts a function with = e to a block, return e or e,
// writing its return type if it was inferred.
func (r refactorer) toBlockBody() []Fix {
	fn := r.function()
	eq := fn.ChildKind(psi.KindEQ)
	e := fn.Body()
	ps := fn.ChildKind(psi.KindVALUE_PARAMETER_LIST)
	if !eq.Valid() || !e.Valid() || !ps.Valid() || e.NumericKind() == psi.KindANNOTATED_EXPRESSION {
		return nil // return @A x would read as a label
	}
	var edits []FixEdit
	unit := false
	if rt := fn.ReturnType(); rt.Valid() {
		unit = rt.Text() == "Unit"
	} else {
		t, ok := r.declType(fn)
		if !ok {
			return nil
		}
		unit = t.Name == "kotlin.Unit"
		if !unit {
			s, imports, ok := r.typeText(t)
			if !ok {
				return nil
			}
			edits = append(edits, r.edit(psi.Range{Start: ps.Range().End, End: ps.Range().End}, ": "+s))
			edits = append(edits, imports...)
		}
	}
	indent := lineIndent(r.snap.Source(), fn.Range().Start)
	ret := "return "
	if unit {
		ret = ""
	}
	body := "{\n" + indent + "    " + ret + e.Text() + "\n" + indent + "}"
	edits = append([]FixEdit{r.edit(psi.Range{Start: eq.Range().Start, End: e.Range().End}, body)}, edits...)
	return r.refactor(RefactorRewrite, "Convert to block body", edits...)
}
