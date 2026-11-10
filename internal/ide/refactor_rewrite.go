package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// Rewrite intentions: conversions between equivalent forms of code, by
// its syntax, with the facts only to tell a String concatenation or an
// expression's use. Each is offered where the caret is on the construct
// it rewrites, and only where the result means the same.

// rewrites returns the rewrite intentions at the caret.
func (r refactorer) rewrites() []Fix {
	var out []Fix
	for _, f := range []func() []Fix{r.ifToWhen, r.whenToIf, r.braces, r.invertIf, r.concatToTemplate, r.templateToConcat,
		r.itToParameter, r.parameterToIt, r.joinDeclaration, r.argumentLines} {
		out = append(out, f()...)
	}
	return out
}

// keyword returns the node whose keyword, of kind kw, the caret is on.
func (r refactorer) keyword(kw psi.Kind) psi.NodeRef {
	c := r.caret()
	if c.NumericKind() != kw {
		if p := r.snap.NodeAt(r.d.Range.Start); p.NumericKind() == kw {
			c = p
		} else {
			return psi.NodeRef{}
		}
	}
	return c.Parent()
}

// reindent returns text with indentation add added to each line after
// the first, or, with add empty, up to remove columns of blanks removed
// from each.
func reindent(text, add string, remove int) string {
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if add != "" {
			lines[i] = add + lines[i]
			continue
		}
		k := 0
		for k < remove && k < len(lines[i]) && (lines[i][k] == ' ' || lines[i][k] == '\t') {
			k++
		}
		lines[i] = lines[i][k:]
	}
	return strings.Join(lines, "\n")
}

// ifChain returns the conditions and branches of an if-else-if chain;
// the last branch has no condition if the chain ends with an else.
func ifChain(n psi.NodeRef) (conds, branches []psi.NodeRef, ok bool) {
	for {
		c := lastExpression(n.ChildKind(psi.KindCONDITION))
		th := lastExpression(n.ChildKind(psi.KindTHEN))
		if !c.Valid() || !th.Valid() {
			return nil, nil, false
		}
		conds, branches = append(conds, c), append(branches, th)
		el := n.ChildKind(psi.KindELSE)
		if !el.Valid() {
			return conds, branches, true
		}
		e := lastExpression(el)
		if !e.Valid() {
			return nil, nil, false
		}
		if e.NumericKind() != psi.KindIF {
			return conds, append(branches, e), true
		}
		n = e
	}
}

// ifToWhen converts an if-else chain to a when without a subject.
func (r refactorer) ifToWhen() []Fix {
	n := r.keyword(psi.KindIF_KEYWORD)
	if n.NumericKind() != psi.KindIF || n.Parent().NumericKind() == psi.KindELSE || hasComment(n) {
		return nil
	}
	conds, branches, ok := ifChain(n)
	if !ok || len(branches) < 2 {
		return nil
	}
	if len(branches) == len(conds) && r.used(n) {
		return nil // an if expression without an else doesn't compile
	}
	ind, unit := lineIndent(r.snap.Source(), n.Range().Start), indentUnit(r.snap.Source())
	var b strings.Builder
	b.WriteString("when {\n")
	for i, br := range branches {
		cond := "else"
		if i < len(conds) {
			cond = conds[i].Text()
		}
		b.WriteString(ind + unit + cond + " -> " + reindent(br.Text(), unit, 0) + "\n")
	}
	b.WriteString(ind + "}")
	return r.refactor(RefactorRewrite, "Replace 'if' with 'when'", r.edit(n.Range(), b.String()))
}

// whenToIf converts a when without a subject to an if-else chain.
func (r refactorer) whenToIf() []Fix {
	n := r.keyword(psi.KindWHEN_KEYWORD)
	if n.NumericKind() != psi.KindWHEN || n.ChildKind(psi.KindLPAR).Valid() || hasComment(n) {
		return nil // a subject, or comments
	}
	var entries []psi.NodeRef
	for e := n.FirstChild(); e.Valid(); e = e.NextSibling() {
		if e.NumericKind() == psi.KindWHEN_ENTRY {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	hasElse := entries[len(entries)-1].ChildKind(psi.KindELSE_KEYWORD).Valid()
	if !hasElse && r.used(n) {
		return nil // exhaustive by its types, which an if isn't
	}
	ind := lineIndent(r.snap.Source(), n.Range().Start)
	var b strings.Builder
	for i, e := range entries {
		body := lastExpression(e)
		if !body.Valid() || body.NumericKind() == psi.KindWHEN_CONDITION_EXPRESSION {
			return nil
		}
		if body.NumericKind() == psi.KindIF && i < len(entries)-1 {
			return nil // the next else could go to it
		}
		text := reindent(body.Text(), "", len(indentUnit(r.snap.Source())))
		if e.ChildKind(psi.KindELSE_KEYWORD).Valid() {
			if i == 0 {
				return nil
			}
			b.WriteString("\n" + ind + "else " + text)
			continue
		}
		var conds []string
		for c := e.FirstChild(); c.Valid(); c = c.NextSibling() {
			if c.NumericKind() != psi.KindWHEN_CONDITION_EXPRESSION {
				continue
			}
			x := lastExpression(c)
			if !x.Valid() {
				return nil
			}
			conds = append(conds, x.Text())
		}
		if len(conds) == 0 {
			return nil
		}
		cond := conds[0]
		if len(conds) > 1 {
			for k := range conds {
				conds[k] = "(" + conds[k] + ")"
			}
			cond = strings.Join(conds, " || ")
		}
		if i > 0 {
			b.WriteString("\n" + ind + "else ")
		}
		b.WriteString("if (" + cond + ") " + text)
	}
	return r.refactor(RefactorRewrite, "Replace 'when' with 'if'", r.edit(n.Range(), b.String()))
}

// used reports whether an expression's value is used, as the view says;
// true if it doesn't know.
func (r refactorer) used(x psi.NodeRef) bool {
	f, ok := r.v.fact(x)
	return !ok || f.UsedAsExpression
}

// braces adds braces around the body of the if, else, for, while or do
// whose keyword the caret is on, or removes them from a body of one
// statement.
func (r refactorer) braces() []Fix {
	c := r.caret()
	var body, stmt psi.NodeRef
	switch c.NumericKind() {
	case psi.KindIF_KEYWORD:
		stmt = c.Parent()
		body = stmt.ChildKind(psi.KindTHEN)
	case psi.KindELSE_KEYWORD:
		stmt = c.Parent()
		if stmt.NumericKind() != psi.KindIF {
			return nil
		}
		body = stmt.ChildKind(psi.KindELSE)
		if lastExpression(body).NumericKind() == psi.KindIF {
			return nil // else if
		}
	case psi.KindFOR_KEYWORD, psi.KindWHILE_KEYWORD, psi.KindDO_KEYWORD:
		stmt = c.Parent()
		body = stmt.ChildKind(psi.KindBODY)
	default:
		return nil
	}
	x := lastExpression(body)
	if !x.Valid() || hasComment(body) {
		return nil
	}
	top := stmt
	for top.Parent().NumericKind() == psi.KindELSE {
		top = top.Parent().Parent()
	}
	ind := lineIndent(r.snap.Source(), top.Range().Start)
	if x.NumericKind() != psi.KindBLOCK {
		unit := indentUnit(r.snap.Source())
		text := "{\n" + ind + unit + reindent(x.Text(), unit, 0) + "\n" + ind + "}"
		return r.refactor(RefactorRewrite, "Add braces to '"+c.Text()+"' statement", r.edit(x.Range(), text))
	}
	inner := branch(body)
	if !inner.Valid() {
		return nil
	}
	switch inner.NumericKind() {
	case psi.KindPROPERTY, psi.KindFUN, psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS, psi.KindLAMBDA_EXPRESSION,
		psi.KindDESTRUCTURING_DECLARATION:
		return nil
	case psi.KindIF:
		// if (a) { if (b) x } else y: the else would go to the inner if.
		if c.NumericKind() == psi.KindIF_KEYWORD && stmt.ChildKind(psi.KindELSE).Valid() {
			if conds, branches, ok := ifChain(inner); !ok || len(conds) == len(branches) {
				return nil
			}
		}
	}
	if c.NumericKind() == psi.KindIF_KEYWORD && r.used(stmt) && inner.NumericKind() == psi.KindBINARY_EXPRESSION && assignment(inner) {
		return nil
	}
	return r.refactor(RefactorRewrite, "Remove braces from '"+c.Text()+"' statement", r.edit(x.Range(), reindent(inner.Text(), "", len(indentUnit(r.snap.Source())))))
}

// invertIf negates an if's condition and swaps its branches.
func (r refactorer) invertIf() []Fix {
	n := r.keyword(psi.KindIF_KEYWORD)
	if n.NumericKind() != psi.KindIF || hasComment(n) {
		return nil
	}
	c := lastExpression(n.ChildKind(psi.KindCONDITION))
	th, el := lastExpression(n.ChildKind(psi.KindTHEN)), lastExpression(n.ChildKind(psi.KindELSE))
	if !c.Valid() || !th.Valid() || !el.Valid() || el.NumericKind() == psi.KindIF || th.NumericKind() == psi.KindIF {
		return nil
	}
	return r.refactor(RefactorRewrite, "Invert 'if' condition",
		r.edit(c.Range(), negate(r.v, c)), r.edit(th.Range(), el.Text()), r.edit(el.Range(), th.Text()))
}

// plusChain returns the outermost + of String concatenations that holds
// the caret, flattened to its operands, if its leftmost operand is a
// String: then each + is String.plus, as a template's parts are.
func (r refactorer) plusChain() (psi.NodeRef, []psi.NodeRef) {
	var top psi.NodeRef
	for n := r.caret(); n.Valid(); n = n.Parent() {
		if n.NumericKind() == psi.KindBINARY_EXPRESSION && n.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() == psi.KindPLUS {
			top = n
		} else if top.Valid() && n.NumericKind() != psi.KindPARENTHESIZED {
			break
		}
	}
	if !top.Valid() {
		return psi.NodeRef{}, nil
	}
	if f, ok := r.v.fact(top); !ok || f.Type == nil || f.Type.Name != "kotlin.String" {
		return psi.NodeRef{}, nil
	}
	var ops []psi.NodeRef
	var flatten func(n psi.NodeRef) bool
	flatten = func(n psi.NodeRef) bool {
		if n.NumericKind() == psi.KindBINARY_EXPRESSION && n.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() == psi.KindPLUS {
			return flatten(firstNonTrivia(n)) && func() bool { ops = append(ops, lastExpression(n)); return true }()
		}
		ops = append(ops, n)
		return true
	}
	flatten(top)
	if len(ops) < 2 {
		return psi.NodeRef{}, nil
	}
	if f, ok := r.v.fact(ops[0]); !ok || f.Type == nil || f.Type.Name != "kotlin.String" || f.Type.Nullable {
		return psi.NodeRef{}, nil
	}
	return top, ops
}

// stringContent returns the text between a non-raw string literal's
// quotes.
func stringContent(n psi.NodeRef) (string, bool) {
	t := n.Text()
	if n.NumericKind() != psi.KindSTRING_TEMPLATE || strings.HasPrefix(t, `"""`) || len(t) < 2 {
		return "", false
	}
	return t[1 : len(t)-1], true
}

// concatToTemplate converts a concatenation of Strings to a template.
func (r refactorer) concatToTemplate() []Fix {
	top, ops := r.plusChain()
	if !top.Valid() || hasComment(top) {
		return nil
	}
	var parts []string // template text, an expression's as "\x00name" when a bare name may do
	var lits []bool    // whether a part is a literal's
	for _, op := range ops {
		if s, ok := stringContent(op); ok {
			parts, lits = append(parts, s), append(lits, true)
			continue
		}
		lits = append(lits, false)
		if strings.HasPrefix(op.Text(), `"""`) || strings.Contains(op.Text(), "\n") {
			return nil
		}
		x := op
		for x.NumericKind() == psi.KindPARENTHESIZED && lastExpression(x).Valid() {
			x = lastExpression(x)
		}
		if x.NumericKind() == psi.KindREFERENCE_EXPRESSION && !strings.HasPrefix(x.Text(), "`") || x.NumericKind() == psi.KindTHIS_EXPRESSION && x.Text() == "this" {
			parts = append(parts, "\x00"+x.Text())
		} else {
			parts = append(parts, "${"+x.Text()+"}")
		}
	}
	if !slices.Contains(lits, true) {
		return nil
	}
	var b strings.Builder
	for i, p := range parts {
		if lits[i] {
			// "$" + "x" would join to the template "$x".
			if i+1 < len(parts) && lits[i+1] && parts[i+1] != "" && (isIdentPart(parts[i+1][0]) || parts[i+1][0] == '{') && loneDollar(p) {
				p = p[:len(p)-1] + `\$`
			}
			b.WriteString(p)
			continue
		}
		name, bare := strings.CutPrefix(p, "\x00")
		if !bare {
			b.WriteString(p)
			continue
		}
		next := ""
		if i+1 < len(parts) {
			next = parts[i+1]
		}
		if next != "" && !strings.HasPrefix(next, "\x00") && !strings.HasPrefix(next, "$") && isIdentPart(next[0]) {
			b.WriteString("${" + name + "}")
		} else {
			b.WriteString("$" + name)
		}
	}
	return r.refactor(RefactorRewrite, "Convert concatenation to template", r.edit(top.Range(), `"`+b.String()+`"`))
}

// loneDollar reports whether a literal's text ends with a $ that isn't
// escaped.
func loneDollar(text string) bool {
	if !strings.HasSuffix(text, "$") {
		return false
	}
	k := 0
	for i := len(text) - 2; i >= 0 && text[i] == '\\'; i-- {
		k++
	}
	return k%2 == 0
}

// templateToConcat converts a string template with expressions to a
// concatenation.
func (r refactorer) templateToConcat() []Fix {
	var n psi.NodeRef
	for m := r.caret(); m.Valid(); m = m.Parent() {
		if m.NumericKind() == psi.KindSTRING_TEMPLATE {
			n = m
			break
		}
		if m.NumericKind() == psi.KindLONG_STRING_TEMPLATE_ENTRY {
			return nil // inside an expression of a template
		}
	}
	if _, ok := stringContent(n); !ok || constant(n) {
		return nil
	}
	var parts []string
	var lit strings.Builder
	entries := 0
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, `"`+lit.String()+`"`)
			lit.Reset()
		}
	}
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindOPEN_QUOTE, psi.KindCLOSING_QUOTE:
		case psi.KindLITERAL_STRING_TEMPLATE_ENTRY, psi.KindESCAPE_STRING_TEMPLATE_ENTRY:
			lit.WriteString(c.Text())
		case psi.KindSHORT_STRING_TEMPLATE_ENTRY, psi.KindLONG_STRING_TEMPLATE_ENTRY:
			x := lastExpression(c)
			if !x.Valid() || strings.Contains(x.Text(), "\n") {
				return nil
			}
			flush()
			entries++
			t := x.Text()
			if !simpleOperand(x) {
				t = "(" + t + ")"
			}
			if len(parts) == 0 {
				t += ".toString()"
			}
			parts = append(parts, t)
		default:
			return nil
		}
	}
	flush()
	if entries == 0 {
		return nil
	}
	text := strings.Join(parts, " + ")
	if operand(n) {
		text = "(" + text + ")"
	}
	return r.refactor(RefactorRewrite, "Convert template to concatenation", r.edit(n.Range(), text))
}

// constant reports whether x is in a const val's initializer or an
// annotation's arguments, where a template of constants is a constant
// but String.plus isn't.
func constant(x psi.NodeRef) bool {
	for n := x.Parent(); n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindANNOTATION_ENTRY:
			return true
		case psi.KindPROPERTY:
			return hasModifier(n, psi.KindCONST_KEYWORD)
		case psi.KindBLOCK, psi.KindFUN, psi.KindCLASS_BODY:
			return false
		}
	}
	return false
}

// itToParameter names the implicit parameter of the lambda whose it the
// caret is on.
func (r refactorer) itToParameter() []Fix {
	c := r.caret()
	ref := c.Parent()
	if c.Text() != "it" || ref.NumericKind() != psi.KindREFERENCE_EXPRESSION || nameOnly(ref) {
		return nil
	}
	var lit psi.NodeRef
	for n := ref.Parent(); n.Valid(); n = n.Parent() {
		if n.NumericKind() == psi.KindFUNCTION_LITERAL {
			if !n.ChildKind(psi.KindARROW).Valid() {
				lit = n
			}
			break
		}
	}
	if !lit.Valid() {
		return nil
	}
	fact, ok := r.v.fact(ref)
	if !ok || fact.Type == nil || fact.Type.Name == "" {
		return nil
	}
	name := uniqueName(lit, typeName(*fact.Type))
	lb := lit.ChildKind(psi.KindLBRACE)
	edits := []FixEdit{r.edit(psi.Range{Start: lb.Range().End, End: lb.Range().End}, " "+name+" ->")}
	for _, u := range implicitIts(lit) {
		edits = append(edits, r.edit(u.Range(), name))
	}
	return r.refactor(RefactorRewrite, "Replace 'it' with explicit parameter", edits...)
}

// nameOnly reports whether a reference names a member, a named
// argument or a label rather than a value in scope.
func nameOnly(ref psi.NodeRef) bool {
	if selector(ref) {
		return true
	}
	switch ref.Parent().NumericKind() {
	case psi.KindVALUE_ARGUMENT_NAME, psi.KindUSER_TYPE, psi.KindLABEL_QUALIFIER:
		return true
	}
	return false
}

// implicitIts returns the uses of a lambda's implicit parameter, it: not
// those in a nested lambda with its own.
func implicitIts(lit psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	psi.WalkNodes(lit.ChildKind(psi.KindBLOCK), func(m psi.NodeRef) bool {
		switch m.NumericKind() {
		case psi.KindFUNCTION_LITERAL:
			return m.ChildKind(psi.KindARROW).Valid() // a lambda without parameters has an it of its own
		case psi.KindREFERENCE_EXPRESSION:
			if m.Text() == "it" && !nameOnly(m) {
				out = append(out, m)
			}
		}
		return true
	})
	return out
}

// parameterToIt replaces a lambda's only parameter, untyped, with it.
func (r refactorer) parameterToIt() []Fix {
	c := r.caret()
	p := c.Parent()
	if c.NumericKind() != psi.KindIDENTIFIER || p.NumericKind() != psi.KindVALUE_PARAMETER || p.Parent().NumericKind() != psi.KindVALUE_PARAMETER_LIST {
		return nil
	}
	lit := p.Parent().Parent()
	if lit.NumericKind() != psi.KindFUNCTION_LITERAL || len(lit.Parameters()) != 1 ||
		p.TypeReference().Valid() || p.ChildKind(psi.KindDESTRUCTURING_DECLARATION).Valid() || hasComment(lit.ChildKind(psi.KindVALUE_PARAMETER_LIST)) {
		return nil
	}
	name := p.Name()
	if name == "" || name == "_" || name == "it" {
		return nil
	}
	switch lit.Parent().Parent().NumericKind() {
	case psi.KindTHEN, psi.KindELSE, psi.KindBODY, psi.KindWHEN_ENTRY:
		return nil // without its arrow, a branch's lambda would be a block
	}
	body := lit.ChildKind(psi.KindBLOCK)
	var uses []psi.NodeRef
	clash := false
	psi.WalkNodes(body, func(m psi.NodeRef) bool {
		switch m.NumericKind() {
		case psi.KindFUNCTION_LITERAL:
			if !m.ChildKind(psi.KindARROW).Valid() {
				clash = true // a nested lambda's it would hide the parameter
			}
		case psi.KindPROPERTY, psi.KindVALUE_PARAMETER, psi.KindFUN, psi.KindDESTRUCTURING_DECLARATION_ENTRY:
			if m.Name() == name || m.Name() == "it" {
				clash = true
			}
		case psi.KindREFERENCE_EXPRESSION:
			switch t := psi.Unquote(m.Text()); {
			case t == "it" && !nameOnly(m):
				clash = true // an outer it
			case t == name && !nameOnly(m):
				uses = append(uses, m)
			}
		}
		return !clash
	})
	if clash {
		return nil
	}
	src := r.snap.Source()
	start, end := lit.ChildKind(psi.KindLBRACE).Range().End, lit.ChildKind(psi.KindARROW).Range().End
	repl := ""
	k := end
	for k < len(src) && (src[k] == ' ' || src[k] == '\t') {
		k++
	}
	if k < len(src) && src[k] != '\n' && src[k] != '\r' {
		end, repl = k, " "
	}
	edits := []FixEdit{r.edit(psi.Range{Start: start, End: end}, repl)}
	for _, u := range uses {
		edits = append(edits, r.edit(u.Range(), "it"))
	}
	return r.refactor(RefactorRewrite, "Replace explicit parameter '"+name+"' with 'it'", edits...)
}

// joinDeclaration joins a local variable's declaration without an
// initializer and the assignment that follows it.
func (r refactorer) joinDeclaration() []Fix {
	p := r.header(psi.KindPROPERTY)
	if !p.Valid() || p.Parent().NumericKind() != psi.KindBLOCK || p.ChildKind(psi.KindEQ).Valid() || !p.TypeReference().Valid() ||
		p.ChildKind(psi.KindPROPERTY_DELEGATE).Valid() || hasModifier(p, psi.KindLATEINIT_KEYWORD) {
		return nil
	}
	if p.ChildKind(psi.KindVAR_KEYWORD).Valid() && p.TypeReference().FirstChild().NumericKind() == psi.KindNULLABLE_TYPE {
		return nil // the checker doesn't smart cast a var by its initializer as by an assignment
	}
	next := p.NextSibling()
	for next.Valid() && next.IsTrivia() && !hasComment(next) {
		next = next.NextSibling()
	}
	if next.NumericKind() != psi.KindBINARY_EXPRESSION || next.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() != psi.KindEQ {
		return nil
	}
	target, value := firstNonTrivia(next), lastExpression(next)
	if target.NumericKind() != psi.KindREFERENCE_EXPRESSION || psi.Unquote(target.Text()) != p.Name() || !value.Valid() || hasComment(next) {
		return nil
	}
	src := r.snap.Source()
	if p.Range().End != p.TypeReference().Range().End {
		return nil
	}
	lines := lineRange(src, next.Range())
	return r.refactor(RefactorRewrite, "Join declaration and assignment",
		r.edit(psi.Range{Start: p.Range().End, End: lines.End}, " = "+value.Text()+lineEnding(src, lines)))
}

// lineEnding returns the line break that ends r, the lines a statement
// removed takes, to keep after what precedes it.
func lineEnding(src string, r psi.Range) string {
	if r.End > 0 && src[r.End-1] == '\n' {
		return "\n"
	}
	return ""
}

// argumentLines puts the arguments, or parameters, of the list the caret
// is in on separate lines, or on one line.
func (r refactorer) argumentLines() []Fix {
	var list psi.NodeRef
	for n := r.caret(); n.Valid(); n = n.Parent() {
		k := n.NumericKind()
		if k == psi.KindVALUE_ARGUMENT_LIST || k == psi.KindVALUE_PARAMETER_LIST && n.ChildKind(psi.KindLPAR).Valid() {
			list = n
			break
		}
		if k == psi.KindBLOCK || k == psi.KindCLASS_BODY || k == psi.KindFUNCTION_LITERAL {
			return nil
		}
	}
	if !list.Valid() || hasComment(list) {
		return nil
	}
	var items []psi.NodeRef
	for c := list.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindVALUE_ARGUMENT || c.NumericKind() == psi.KindVALUE_PARAMETER {
			items = append(items, c)
		}
	}
	what := "arguments"
	if list.NumericKind() == psi.KindVALUE_PARAMETER_LIST {
		what = "parameters"
	}
	src := r.snap.Source()
	text := list.Text()
	if !strings.Contains(text, "\n") {
		if len(items) < 2 {
			return nil
		}
		ind := lineIndent(src, list.Range().Start)
		var b strings.Builder
		b.WriteString("(\n")
		for i, it := range items {
			b.WriteString(ind + indentUnit(src) + it.Text())
			if i < len(items)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(ind + ")")
		return r.refactor(RefactorRewrite, "Put "+what+" on separate lines", r.edit(list.Range(), b.String()))
	}
	var texts []string
	for _, it := range items {
		if strings.Contains(it.Text(), "\n") {
			return nil
		}
		texts = append(texts, it.Text())
	}
	return r.refactor(RefactorRewrite, "Put "+what+" on one line", r.edit(list.Range(), "("+strings.Join(texts, ", ")+")"))
}
