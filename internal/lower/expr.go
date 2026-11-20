package lower

import (
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// binaryOps maps the operator leaves of binary expressions to kc's
// operator kinds.
var binaryOps = map[psi.Kind]syntax.Kind{
	psi.KindPLUS: syntax.PLUS, psi.KindMINUS: syntax.MINUS, psi.KindMUL: syntax.STAR,
	psi.KindDIV: syntax.SLASH, psi.KindPERC: syntax.PERCENT, psi.KindRANGE: syntax.RANGE,
	psi.KindRANGE_UNTIL: syntax.RANGE_LT, psi.KindEQEQ: syntax.EQ, psi.KindEXCLEQ: syntax.NE,
	psi.KindEQEQEQ: syntax.EQ_EQ_EQ, psi.KindEXCLEQEQ: syntax.NOT_EQ_EQ, psi.KindLT: syntax.LT,
	psi.KindGT: syntax.GT, psi.KindLTEQ: syntax.LE, psi.KindGTEQ: syntax.GE, psi.KindANDAND: syntax.AND,
	psi.KindOROR: syntax.OR, psi.KindELVIS: syntax.ELVIS, psi.KindIN_KEYWORD: syntax.IN,
	psi.KindNOT_IN: syntax.NOT_IN,
}

// assignOps maps the operator leaves of assignments to kc's.
var assignOps = map[psi.Kind]syntax.Kind{
	psi.KindEQ: syntax.ASSIGN, psi.KindPLUSEQ: syntax.PLUS_EQ, psi.KindMINUSEQ: syntax.MINUS_EQ,
	psi.KindMULTEQ: syntax.STAR_EQ, psi.KindDIVEQ: syntax.SLASH_EQ, psi.KindPERCEQ: syntax.PERCENT_EQ,
}

var prefixOps = map[psi.Kind]syntax.Kind{
	psi.KindMINUS: syntax.MINUS, psi.KindPLUS: syntax.PLUS, psi.KindEXCL: syntax.NOT,
	psi.KindPLUSPLUS: syntax.INC, psi.KindMINUSMINUS: syntax.DEC,
}

// opOf returns the operator leaf of an expression's OPERATION_REFERENCE.
func opOf(n psi.NodeRef) psi.NodeRef { return first(child(n, psi.KindOPERATION_REFERENCE)) }

// operands returns the expression children of n.
func operands(n psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	for _, c := range kids(n) {
		if isExpr(c) {
			out = append(out, c)
		}
	}
	return out
}

// expr lowers an expression.
func (l *lowerer) expr(n psi.NodeRef) syntax.Expr {
	if !n.Valid() {
		l.failAt(l.posAt(len(l.src)), "expected an expression")
	}
	x := l.expr1(n)
	if _, ok := l.prov[x]; !ok {
		l.prov[x] = n.ID()
	}
	return x
}

func (l *lowerer) expr1(n psi.NodeRef) syntax.Expr {
	switch n.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION:
		return l.ident(n)
	case psi.KindPARENTHESIZED:
		x := l.expr(exprChild(n))
		syntax.Parenthesized(x)
		return x
	case psi.KindBINARY_EXPRESSION:
		return l.binary(n)
	case psi.KindIS_EXPRESSION:
		ops := operands(n)
		op := opOf(n)
		is := &syntax.Is{X: l.expr(ops[0]), Ref: l.typeRef(child(n, psi.KindTYPE_REFERENCE)),
			Not: op.NumericKind() == psi.KindNOT_IS}
		is.P = l.pos(op)
		return is
	case psi.KindBINARY_WITH_TYPE:
		op := opOf(n)
		if k := op.NumericKind(); k != psi.KindAS_KEYWORD && k != psi.KindAS_SAFE {
			l.fail(op, "unexpected '%s'", op.Text())
		}
		as := &syntax.As{X: l.expr(operands(n)[0]), Ref: l.typeRef(child(n, psi.KindTYPE_REFERENCE)),
			Safe: op.NumericKind() == psi.KindAS_SAFE}
		as.P = l.pos(op)
		return as
	case psi.KindPREFIX_EXPRESSION:
		return l.prefix(n)
	case psi.KindPOSTFIX_EXPRESSION:
		op := opOf(n)
		x := l.expr(exprChild(n))
		switch op.NumericKind() {
		case psi.KindEXCLEXCL:
			nn := &syntax.NotNull{X: x}
			nn.P = l.pos(op)
			return nn
		case psi.KindPLUSPLUS, psi.KindMINUSMINUS:
			u := &syntax.Unary{Op: prefixOps[op.NumericKind()], X: x}
			u.P = l.pos(op)
			return u
		}
		l.unexpected(op)
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
		return l.qualified(n)
	case psi.KindCALL_EXPRESSION:
		return l.call(n)
	case psi.KindARRAY_ACCESS_EXPRESSION:
		ix := &syntax.Index{X: l.expr(exprChild(n))}
		indices := child(n, psi.KindINDICES)
		ix.P = l.pos(indices)
		for _, c := range operands(indices) {
			ix.Args = append(ix.Args, l.expr(c))
		}
		return ix
	case psi.KindINTEGER_CONSTANT:
		l.checkGlued(n)
		t := first(n)
		text := t.Text()
		if syntax.IllegalUnderscore(text) {
			l.errorLater(l.pos(t), "illegal underscore")
		}
		for _, msg := range syntax.IntLiteralErrors(text) {
			l.errorLater(l.pos(t), "%s", msg)
		}
		lit, ok := syntax.IntLiteral(l.pos(t), text, strings.ContainsAny(text, "uU"))
		if !ok {
			l.fail(t, "the value is out of range")
		}
		return lit
	case psi.KindFLOAT_CONSTANT:
		l.checkGlued(n)
		t := first(n)
		if syntax.IllegalUnderscore(t.Text()) {
			l.errorLater(l.pos(t), "illegal underscore")
		}
		lit, msg := syntax.FloatLiteral(l.pos(t), t.Text())
		if msg != "" {
			l.fail(t, "%s", msg)
		}
		return lit
	case psi.KindCHARACTER_CONSTANT:
		l.checkGlued(n)
		r, msg := syntax.CharValue(n.Text())
		if msg != "" {
			l.errorLater(l.pos(n), "%s", msg)
		}
		c := &syntax.CharLit{Val: r}
		c.P = l.pos(n)
		return c
	case psi.KindBOOLEAN_CONSTANT:
		b := &syntax.BoolLit{Val: has(n, psi.KindTRUE_KEYWORD)}
		b.P = l.pos(n)
		return b
	case psi.KindNULL:
		x := &syntax.NullLit{}
		x.P = l.pos(n)
		return x
	case psi.KindSTRING_TEMPLATE:
		l.checkGlued(n)
		return l.stringLit(n)
	case psi.KindTHIS_EXPRESSION:
		th := &syntax.This{Label: l.label(n)}
		th.P = l.pos(n)
		return th
	case psi.KindSUPER_EXPRESSION:
		s := &syntax.Super{Label: l.label(n)}
		s.P = l.pos(n)
		if t := child(n, psi.KindTYPE_REFERENCE); t.Valid() {
			s.Iface = l.typeRef(t)
		}
		return s
	case psi.KindIF:
		return l.ifExpr(n)
	case psi.KindWHEN:
		return l.when(n)
	case psi.KindTRY:
		return l.try(n)
	case psi.KindRETURN, psi.KindBREAK, psi.KindCONTINUE:
		kind := map[psi.Kind]syntax.Kind{psi.KindRETURN: syntax.RETURN, psi.KindBREAK: syntax.BREAK,
			psi.KindCONTINUE: syntax.CONTINUE}[n.NumericKind()]
		j := &syntax.Jump{Kind: kind, Label: l.label(n)}
		j.P = l.pos(n)
		if x := exprChild(n); x.Valid() {
			j.X = l.expr(x)
		}
		return j
	case psi.KindTHROW:
		j := &syntax.Jump{Kind: syntax.THROW, X: l.expr(exprChild(n))}
		j.P = l.pos(n)
		return j
	case psi.KindLAMBDA_EXPRESSION:
		return l.lambda(n)
	case psi.KindLABELED_EXPRESSION:
		n = l.innermostLabel(n)
		name := l.label(n)
		x := exprChild(n)
		switch x.NumericKind() {
		// The lambda's or function's facts are its own node's, which the
		// label wraps.
		case psi.KindLAMBDA_EXPRESSION:
			lam := l.lambda(x)
			lam.Label = name
			l.prov[lam] = x.ID()
			return lam
		case psi.KindFUN:
			f := l.anonFun(x, name)
			l.prov[f] = x.ID()
			return f
		}
		// A label on another expression has no effect.
		return l.expr(x)
	case psi.KindANNOTATED_EXPRESSION:
		if l.hoisted[n.ID()] {
			// A statement's annotations, lowered with the statement.
			return l.expr(exprChild(n))
		}
		as := l.annotationsOf(n)
		x := l.expr(exprChild(n))
		if lam, ok := x.(*syntax.Lambda); ok {
			for _, a := range as {
				syntax.AnnotateLambda(lam, a)
			}
		}
		syntax.Annotate(&l.annotated, x, as)
		return x
	case psi.KindCLASS_LITERAL_EXPRESSION:
		return l.classLit(n)
	case psi.KindCALLABLE_REFERENCE_EXPRESSION:
		return l.callableRef(n)
	case psi.KindOBJECT_LITERAL:
		return l.objectExpr(n)
	case psi.KindCOLLECTION_LITERAL_EXPRESSION:
		fn := &syntax.Ident{Name: "arrayOf"}
		fn.P = l.pos(n)
		c := &syntax.Call{Fun: fn, Literal: true}
		c.P = fn.P
		for _, a := range operands(n) {
			c.Args = append(c.Args, l.expr(a))
		}
		return c
	case psi.KindFUN:
		if has(n, psi.KindIDENTIFIER) {
			l.fail(n, "unexpected fun")
		}
		return l.anonFun(n, "")
	}
	l.fail(n, "unexpected %s", describeNode(n))
	return nil
}

// describeNode names a node for an error message.
func describeNode(n psi.NodeRef) string {
	if n.IsLeaf() {
		return "'" + n.Text() + "'"
	}
	return strings.ToLower(strings.ReplaceAll(n.Kind(), "_", " "))
}

// ident lowers a REFERENCE_EXPRESSION.
func (l *lowerer) ident(n psi.NodeRef) *syntax.Ident {
	t := first(n)
	if !t.IsLeaf() || t.NumericKind() == psi.KindTHIS_KEYWORD || t.NumericKind() == psi.KindSUPER_KEYWORD {
		l.fail(t, "unexpected %s", describeNode(t))
	}
	if p := n.Parent(); t.Text() != "_" || p.NumericKind() != psi.KindCALL_EXPRESSION || !has(p, psi.KindTYPE_ARGUMENT_LIST) {
		l.underscoreRef(t)
	}
	id := &syntax.Ident{Name: ident(t)}
	id.P = l.pos(t)
	return record(l, id, n)
}

// checkGlued reports a literal n written against an identifier, a number
// or a keyword, as kc's lexer does for its parser (see TreeErrors).
func (l *lowerer) checkGlued(n psi.NodeRef) {
	if p := prevLeaf(n); gluedAffix(p) {
		l.errorLater(l.pos(p), gluedLiteralMsg)
	}
	if x := nextLeaf(n); gluedAffix(x) {
		l.errorLater(l.pos(x), gluedLiteralMsg)
	}
}

// innermostLabel returns the innermost of the labels n starts with, as
// a@ b@ while (c), reporting the others, as kotlinc does.
func (l *lowerer) innermostLabel(n psi.NodeRef) psi.NodeRef {
	for x := exprChild(n); x.NumericKind() == psi.KindLABELED_EXPRESSION; x = exprChild(n) {
		l.errorLater(l.pos(child(n, psi.KindLABEL_QUALIFIER)), "multiple labels are forbidden")
		n = x
	}
	return n
}

// newLineBefore reports whether whitespace between n and the node before
// it breaks the line (a comment that spans lines doesn't).
func newLineBefore(n psi.NodeRef) bool {
	for c := n.PreviousSibling(); c.Valid() && c.IsTrivia(); c = c.PreviousSibling() {
		if c.NumericKind() == psi.KindWHITE_SPACE && strings.Contains(c.Text(), "\n") {
			return true
		}
	}
	return false
}

// label returns the label of n's LABEL_QUALIFIER, or "".
func (l *lowerer) label(n psi.NodeRef) string {
	q := child(n, psi.KindLABEL_QUALIFIER)
	if !q.Valid() {
		return ""
	}
	name := ident(child(child(q, psi.KindLABEL), psi.KindIDENTIFIER))
	if n.NumericKind() == psi.KindLABELED_EXPRESSION && strings.Trim(name, "_") == "" {
		// As the lexer, of a label's definition: __@{ }.
		l.errorLater(l.pos(q), "names _, __, ___, ... are reserved in Kotlin")
	}
	return name
}

// annotationsOf lowers the annotations of an ANNOTATED_EXPRESSION.
func (l *lowerer) annotationsOf(n psi.NodeRef) []syntax.Annotation {
	var as []syntax.Annotation
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindANNOTATION_ENTRY:
			as = append(as, l.annotation(c))
		case psi.KindANNOTATION:
			as = append(as, l.bracketAnnotations(c)...)
		}
	}
	return as
}

func (l *lowerer) binary(n psi.NodeRef) syntax.Expr {
	ops := operands(n)
	if len(ops) != 2 {
		l.fail(n, "expected an expression")
	}
	op := opOf(n)
	opPos := l.pos(op)
	if op.NumericKind() == psi.KindIDENTIFIER {
		var k syntax.Kind
		switch op.Text() {
		case "until":
			k = syntax.UNTIL
		case "downTo":
			k = syntax.DOWN_TO
		case "step":
			k = syntax.STEP
		default:
			m := &syntax.Member{X: l.expr(ops[0]), Name: ident(op)}
			m.P = opPos
			if op.Text() == "suspend" && ops[1].NumericKind() == psi.KindFUN {
				l.errorLater(opPos, "modifier form for a non-built-in 'suspend' function is not allowed")
			}
			c := &syntax.Call{Fun: m, Args: []syntax.Expr{l.expr(ops[1])}, Infix: true}
			c.P = opPos
			syntax.LabelLambdas(c)
			return c
		}
		b := &syntax.Binary{Op: k, X: l.expr(ops[0]), Y: l.expr(ops[1])}
		b.P = opPos
		return b
	}
	k, ok := binaryOps[op.NumericKind()]
	if !ok {
		l.fail(op, "unexpected '%s'", op.Text())
	}
	b := &syntax.Binary{Op: k, X: l.expr(ops[0]), Y: l.expr(ops[1])}
	b.P = opPos
	return b
}

func (l *lowerer) prefix(n psi.NodeRef) syntax.Expr {
	op := opOf(n)
	k, ok := prefixOps[op.NumericKind()]
	if !ok {
		l.unexpected(op)
	}
	operand := exprChild(n)
	x := l.expr(operand)
	if lit, ok := x.(*syntax.IntLit); ok && k == syntax.MINUS {
		// -1 is a literal, but not -(1).
		if last := l.lastLeaf(operand); last.NumericKind() == psi.KindINTEGER_LITERAL && !strings.ContainsAny(last.Text(), "uU") &&
			syntax.NegateLiteral(lit, last.Text(), l.pos(op)) {
			l.negated[n.ID()] = lit
			return lit
		}
	}
	u := &syntax.Unary{Op: k, X: x, Prefix: k == syntax.INC || k == syntax.DEC}
	u.P = l.pos(op)
	return u
}

// lastLeaf returns the last non-trivia leaf in n.
func (l *lowerer) lastLeaf(n psi.NodeRef) psi.NodeRef {
	var last psi.NodeRef
	for id := n.ID(); id < n.SubtreeEnd(); id++ {
		if c := l.s.Node(id); c.IsLeaf() && !c.IsTrivia() {
			last = c
		}
	}
	return last
}

// qualified lowers a DOT_QUALIFIED_EXPRESSION or SAFE_ACCESS_EXPRESSION.
func (l *lowerer) qualified(n psi.NodeRef) syntax.Expr {
	ops := operands(n)
	if len(ops) != 2 {
		l.failAt(l.end(n), "expected a name")
	}
	safe := n.NumericKind() == psi.KindSAFE_ACCESS_EXPRESSION
	x := l.expr(ops[0])
	sel := ops[1]
	switch sel.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION:
		return l.memberExpr(x, sel, safe)
	case psi.KindCALL_EXPRESSION:
		callee := exprChild(sel)
		switch callee.NumericKind() {
		case psi.KindREFERENCE_EXPRESSION:
			return l.callOn(l.memberExpr(x, callee, safe), l.typeArgsOf(sel), sel)
		case psi.KindPARENTHESIZED:
			if safe {
				break
			}
			// r.(f)(args) is (f)(r, args).
			c := l.callOn(l.expr(callee), nil, sel)
			c.Args = append([]syntax.Expr{x}, c.Args...)
			c.Recv = true
			if c.Names != nil {
				c.Names = append([]string{""}, c.Names...)
			}
			return c
		}
	}
	l.fail(sel, "expected a name")
	return nil
}

func (l *lowerer) memberExpr(x syntax.Expr, name psi.NodeRef, safe bool) *syntax.Member {
	t := first(name)
	if t.NumericKind() != psi.KindIDENTIFIER {
		l.fail(t, "expected a name")
	}
	l.underscoreRef(t)
	m := &syntax.Member{X: x, Name: ident(t), Safe: safe, Written: true}
	m.P = l.pos(t)
	return record(l, m, name)
}

// typeArgsOf returns the type arguments of a CALL_EXPRESSION, or nil.
func (l *lowerer) typeArgsOf(n psi.NodeRef) []*syntax.TypeArg {
	if ta := child(n, psi.KindTYPE_ARGUMENT_LIST); ta.Valid() {
		return l.typeArgs(ta)
	}
	return nil
}

// call lowers a CALL_EXPRESSION.
func (l *lowerer) call(n psi.NodeRef) syntax.Expr {
	callee := exprChild(n)
	if callee.NumericKind() == psi.KindREFERENCE_EXPRESSION && first(callee).Text() == "suspend" &&
		!has(n, psi.KindTYPE_ARGUMENT_LIST) && !has(n, psi.KindVALUE_ARGUMENT_LIST) {
		// suspend { ... }: a lambda with the suspend modifier.
		la := child(n, psi.KindLAMBDA_ARGUMENT)
		if first(la).NumericKind() == psi.KindLAMBDA_EXPRESSION && !next(la).Valid() && !l.newlineBetween(callee, la) {
			lam := l.lambda(first(la))
			lam.P, lam.SuspendMod = l.pos(callee), true
			return lam
		}
	}
	return l.callOn(l.expr(callee), l.typeArgsOf(n), n)
}

// callOn returns a call of fn with type arguments targs and the value
// and lambda arguments of n.
func (l *lowerer) callOn(fn syntax.Expr, targs []*syntax.TypeArg, n psi.NodeRef) *syntax.Call {
	c := &syntax.Call{Fun: fn, TypeArgs: targs}
	args := child(n, psi.KindVALUE_ARGUMENT_LIST)
	if args.Valid() {
		c.P = l.pos(args)
		for _, a := range kids(args) {
			if a.NumericKind() != psi.KindVALUE_ARGUMENT {
				continue
			}
			v := first(a)
			if v.NumericKind() == psi.KindVALUE_ARGUMENT_NAME {
				for len(c.Names) < len(c.Args) {
					c.Names = append(c.Names, "")
				}
				c.Names = append(c.Names, ident(first(first(v))))
				v = next(next(v))
			}
			if v.NumericKind() == psi.KindMUL {
				s := &syntax.Spread{X: l.expr(next(v))}
				s.P = l.pos(v)
				c.Args = append(c.Args, record(l, s, a))
				continue
			}
			c.Args = append(c.Args, l.expr(v))
		}
	}
	trailing := false
	for _, a := range kids(n) {
		if a.NumericKind() != psi.KindLAMBDA_ARGUMENT {
			continue
		}
		if !args.Valid() && !trailing {
			c.P = l.pos(a)
		}
		if trailing {
			// f() {} {}: kotlinc passes only the first, and reports the
			// others.
			if newLineBefore(a) {
				l.errorLater(l.pos(a), "unexpected trailing lambda on a new line")
			} else {
				l.errorLater(l.pos(a), "only one lambda expression is allowed outside a parenthesized argument list")
			}
			continue
		}
		c.Args = append(c.Args, l.expr(first(a)))
		c.TrailingNL = args.Valid() && newLineBefore(a)
		trailing = true
	}
	if !args.Valid() && !trailing {
		l.failAt(l.end(n), "expected '('")
	}
	if trailing {
		c.PassLast()
	}
	syntax.LabelLambdas(c)
	return record(l, c, n)
}

// dottedName returns the name a.b.C that n spells, and the node of its
// last name.
func dottedName(n psi.NodeRef) (string, psi.NodeRef, bool) {
	switch n.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION:
		if t := first(n); t.NumericKind() == psi.KindIDENTIFIER {
			return ident(t), t, true
		}
	case psi.KindDOT_QUALIFIED_EXPRESSION:
		ops := operands(n)
		if len(ops) == 2 {
			x, _, ok := dottedName(ops[0])
			y, last, ok2 := dottedName(ops[1])
			if ok && ok2 {
				return x + "." + y, last, true
			}
		}
	}
	return "", psi.NodeRef{}, false
}

// genericName returns the type a.b.C<T> that n, before ::, spells, if it
// has type arguments.
func (l *lowerer) genericName(n psi.NodeRef) *syntax.TypeRef {
	prefix := ""
	if n.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION {
		ops := operands(n)
		if len(ops) != 2 {
			return nil
		}
		x, _, ok := dottedName(ops[0])
		if !ok {
			return nil
		}
		prefix, n = x+".", ops[1]
	}
	if n.NumericKind() != psi.KindCALL_EXPRESSION || has(n, psi.KindVALUE_ARGUMENT_LIST) || has(n, psi.KindLAMBDA_ARGUMENT) ||
		!has(n, psi.KindTYPE_ARGUMENT_LIST) {
		return nil
	}
	name, last, ok := dottedName(exprChild(n))
	if !ok || strings.Contains(name, ".") {
		return nil
	}
	return &syntax.TypeRef{P: l.pos(last), Name: prefix + name, Args: l.typeArgsOf(n)}
}

// colonColonLeft returns the left side of a class literal or callable
// reference: the expression, or the type when it has type arguments or
// is nullable.
func (l *lowerer) colonColonLeft(n psi.NodeRef) (syntax.Expr, *syntax.TypeRef) {
	cc := child(n, psi.KindCOLONCOLON)
	left := prev(cc)
	if !left.Valid() {
		return nil, nil
	}
	if left.NumericKind() == psi.KindQUEST {
		x := prev(left)
		name, last, ok := dottedName(x)
		if !ok {
			l.unexpected(left)
		}
		return nil, &syntax.TypeRef{P: l.pos(last), Name: name, Nullable: true}
	}
	if ref := l.genericName(left); ref != nil {
		return nil, ref
	}
	if ref, ok := l.typeChain(left, true); ok && hasTypeArgs(ref) {
		return nil, ref
	}
	return l.expr(left), nil
}

// typeChain returns the type that n, before ::, spells as a chain of
// names with type arguments, possibly in parentheses, as kotlinc reads
// it: Outer<A>.Inner<B> or (Outer<A>).Inner<B>. A dotted name's position
// is its last name's, as kc's parser gives it, but in parentheses, where
// the parser reads a type, its first's.
func (l *lowerer) typeChain(n psi.NodeRef, lastPos bool) (*syntax.TypeRef, bool) {
	switch n.NumericKind() {
	case psi.KindPARENTHESIZED:
		return l.typeChain(exprChild(n), false)
	case psi.KindREFERENCE_EXPRESSION:
		if t := first(n); t.NumericKind() == psi.KindIDENTIFIER {
			return &syntax.TypeRef{P: l.pos(t), Name: ident(t)}, true
		}
	case psi.KindCALL_EXPRESSION:
		if has(n, psi.KindVALUE_ARGUMENT_LIST) || has(n, psi.KindLAMBDA_ARGUMENT) || !has(n, psi.KindTYPE_ARGUMENT_LIST) {
			return nil, false
		}
		ref, ok := l.typeChain(exprChild(n), lastPos)
		if !ok || ref.Args != nil || ref.Outer != nil {
			return nil, false
		}
		ref.Args = l.typeArgsOf(n)
		return ref, true
	case psi.KindDOT_QUALIFIED_EXPRESSION:
		ops := operands(n)
		if len(ops) != 2 {
			return nil, false
		}
		outer, ok := l.typeChain(ops[0], lastPos)
		if !ok {
			return nil, false
		}
		seg, ok := l.typeChain(ops[1], lastPos)
		if !ok || seg.Outer != nil || strings.Contains(seg.Name, ".") {
			return nil, false
		}
		if outer.Args != nil {
			return &syntax.TypeRef{P: outer.P, Name: outer.Name + "." + seg.Name, Outer: outer, Args: seg.Args}, true
		}
		outer.Name += "." + seg.Name
		outer.Args = seg.Args
		if lastPos && outer.Outer == nil {
			outer.P = seg.P
		}
		return outer, true
	}
	return nil, false
}

// hasTypeArgs reports whether a type or one of its outer classes' has type
// arguments.
func hasTypeArgs(ref *syntax.TypeRef) bool {
	for ; ref != nil; ref = ref.Outer {
		if ref.Args != nil {
			return true
		}
	}
	return false
}

func (l *lowerer) classLit(n psi.NodeRef) syntax.Expr {
	cc := child(n, psi.KindCOLONCOLON)
	x, ref := l.colonColonLeft(n)
	if x == nil && ref == nil {
		l.fail(cc, "a class literal needs a class or a value before ::")
	}
	cl := &syntax.ClassLit{X: x, Ref: ref}
	cl.P = l.pos(cc)
	return cl
}

func (l *lowerer) callableRef(n psi.NodeRef) syntax.Expr {
	cc := child(n, psi.KindCOLONCOLON)
	x, ref := l.colonColonLeft(n)
	name := next(cc)
	if name.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		l.failAt(l.end(cc), "expected a name")
	}
	l.underscoreRef(first(name))
	return syntax.CallableRefLambda(l.pos(cc), &syntax.CallableRef{X: x, Type: ref, Name: ident(first(name))})
}

// stringLit lowers a STRING_TEMPLATE. Its expressions are parsed apart
// in kc, so annotations in them aren't recorded and anonymous functions
// in them are numbered from 1.
func (l *lowerer) stringLit(n psi.NodeRef) syntax.Expr {
	s := &syntax.StringLit{Src: n.Text()}
	s.P = l.pos(n)
	if p := child(n, psi.KindSTRING_INTERPOLATION_PREFIX); p.Valid() {
		s.Prefix = len(p.Text())
		s.Src = s.Src[s.Prefix:]
	}
	var sb strings.Builder
	flush := func() {
		if sb.Len() > 0 {
			s.Parts = append(s.Parts, syntax.TmplPart{Lit: sb.String()})
			sb.Reset()
		}
	}
	for _, e := range kids(n) {
		switch e.NumericKind() {
		case psi.KindLITERAL_STRING_TEMPLATE_ENTRY:
			sb.WriteString(e.Text())
		case psi.KindESCAPE_STRING_TEMPLATE_ENTRY:
			seq := e.Text()
			r, ok := syntax.EscapeValue(seq)
			if !ok {
				if strings.HasPrefix(seq, `\u`) {
					l.fail(e, "invalid unicode escape")
				}
				l.fail(e, "illegal escape: %s", seq)
			}
			if strings.HasPrefix(seq, `\u`) {
				sb.WriteRune(r)
			} else {
				sb.WriteByte(byte(r))
			}
		case psi.KindSHORT_STRING_TEMPLATE_ENTRY, psi.KindLONG_STRING_TEMPLATE_ENTRY:
			flush()
			x := exprChild(e)
			if !x.Valid() {
				l.failAt(l.end(first(e)), "expected an expression")
			}
			annotated, anonFuns := l.annotated, l.anonFuns
			inv, sh, dsl := l.suppressInvisible, l.suppressShadowed, l.suppressDSL
			l.annotated, l.anonFuns = nil, 0
			part := l.expr(x)
			l.annotated, l.anonFuns = annotated, anonFuns
			l.suppressInvisible, l.suppressShadowed, l.suppressDSL = inv, sh, dsl
			s.Parts = append(s.Parts, syntax.TmplPart{X: part})
		}
	}
	flush()
	return s
}

// lambda lowers a LAMBDA_EXPRESSION.
func (l *lowerer) lambda(n psi.NodeRef) *syntax.Lambda {
	fl := child(n, psi.KindFUNCTION_LITERAL)
	lbrace := child(fl, psi.KindLBRACE)
	lam := &syntax.Lambda{}
	lam.P = l.pos(lbrace)
	if ps := child(fl, psi.KindVALUE_PARAMETER_LIST); ps.Valid() {
		for _, p := range kids(ps) {
			if p.NumericKind() != psi.KindVALUE_PARAMETER {
				continue
			}
			var lp *syntax.LambdaParam
			if d := child(p, psi.KindDESTRUCTURING_DECLARATION); d.Valid() {
				lp = &syntax.LambdaParam{P: l.pos(d)}
				lp.Names, lp.Types = l.destructuringNames(d)
			} else {
				name := child(p, psi.KindIDENTIFIER)
				lp = &syntax.LambdaParam{P: l.pos(name), Name: ident(name)}
			}
			if t := child(p, psi.KindTYPE_REFERENCE); t.Valid() {
				lp.Type = l.typeRef(t)
			}
			lam.Params = append(lam.Params, record(l, lp, p))
		}
	}
	block := child(fl, psi.KindBLOCK)
	rbrace := child(fl, psi.KindRBRACE)
	if !rbrace.Valid() {
		l.missingBrace(n)
	}
	body := &syntax.Block{}
	if arrow := child(fl, psi.KindARROW); arrow.Valid() {
		lam.Arrow = true
		// The body starts at the token after the arrow, or at the end of
		// the line.
		body.P = l.afterToken(arrow)
	} else {
		body.P = l.pos(block)
	}
	body.Stmts = l.stmts(block)
	body.End = l.pos(rbrace)
	if !rbrace.Valid() {
		body.End = l.end(n)
	}
	lam.Body = record(l, body, block)
	return lam
}

// afterToken returns the position of the token after leaf t, as kc's
// lexer gives it: a line break is a token, at the end of the line's last
// token.
func (l *lowerer) afterToken(t psi.NodeRef) syntax.Pos {
	end := t.Range().End
	nextStart := l.tokenStartFrom(t.ID() + 1)
	if l.newlineIn(end, nextStart) {
		return l.posAt(end)
	}
	return l.posAt(nextStart)
}

// newlineIn reports whether kc's lexer sees a line break between offsets
// from and to, which hold only whitespace and comments.
func (l *lowerer) newlineIn(from, to int) bool {
	return strings.Contains(l.src[from:to], "\n")
}

// destructuringNames lowers the entries of a DESTRUCTURING_DECLARATION.
func (l *lowerer) destructuringNames(d psi.NodeRef) ([]string, []*syntax.TypeRef) {
	names, types, _ := l.destructuringEntries(d)
	return names, types
}

// destructuringEntries lowers the entries of a DESTRUCTURING_DECLARATION
// with their annotations, (@A a, b): nil where there are none (see
// syntax.Destructure.Annotations).
func (l *lowerer) destructuringEntries(d psi.NodeRef) ([]string, []*syntax.TypeRef, [][]syntax.Annotation) {
	l.newDestructuring(d)
	var names []string
	var types []*syntax.TypeRef
	var annots [][]syntax.Annotation
	for _, e := range kids(d) {
		if e.NumericKind() != psi.KindDESTRUCTURING_DECLARATION_ENTRY {
			continue
		}
		if m := child(e, psi.KindMODIFIER_LIST); m.Valid() {
			// Only annotations: (private a) is a parse error in kotlinc.
			for _, c := range kids(m) {
				if k := c.NumericKind(); k != psi.KindANNOTATION_ENTRY && k != psi.KindANNOTATION {
					l.unexpected(m)
				}
			}
			for len(annots) < len(names) {
				annots = append(annots, nil)
			}
			annots = append(annots, l.annotationsOf(m))
		}
		names = append(names, ident(child(e, psi.KindIDENTIFIER)))
		var t *syntax.TypeRef
		if tr := child(e, psi.KindTYPE_REFERENCE); tr.Valid() {
			t = l.typeRef(tr)
		}
		types = append(types, t)
	}
	if len(names) == 0 {
		l.failAt(l.end(child(d, psi.KindLPAR)), "unexpected ')'")
	}
	if annots != nil {
		for len(annots) < len(names) {
			annots = append(annots, nil)
		}
	}
	return names, types, annots
}

// anonFun lowers an anonymous FUN, with label, or one of its own.
func (l *lowerer) anonFun(n psi.NodeRef, label string) *syntax.Lambda {
	lam := &syntax.Lambda{Arrow: true, Anon: true, Label: label}
	lam.P = l.pos(child(n, psi.KindFUN_KEYWORD))
	if label == "" {
		l.anonFuns++
		lam.Label = "fun$" + strconv.Itoa(l.anonFuns)
	}
	if m := child(n, psi.KindMODIFIER_LIST); m.Valid() {
		// context(a: A) fun () = a: an anonymous function with context
		// parameters.
		cs := kids(m)
		if label != "" || len(cs) != 1 || cs[0].NumericKind() != psi.KindCONTEXT_PARAMETER_LIST {
			l.unexpected(m)
		}
		lam.P, lam.AnonCtx = l.pos(m), l.contextParams(cs[0])
	}
	if tps := child(n, psi.KindTYPE_PARAMETER_LIST); tps.Valid() {
		// As kotlinc, the function is kept, without them.
		l.errorLater(l.pos(tps), "type parameters are not allowed for anonymous functions")
		names := map[string]bool{}
		for _, tp := range kids(tps) {
			if tp.NumericKind() == psi.KindTYPE_PARAMETER {
				names[ident(child(tp, psi.KindIDENTIFIER))] = true
			}
		}
		for _, c := range kids(child(n, psi.KindTYPE_CONSTRAINT_LIST)) {
			if ref := child(c, psi.KindREFERENCE_EXPRESSION); c.NumericKind() == psi.KindTYPE_CONSTRAINT && !names[ident(first(ref))] {
				l.errorLater(l.pos(ref), "'%s' does not refer to a type parameter of this declaration", ident(first(ref)))
			}
		}
	}
	ps := child(n, psi.KindVALUE_PARAMETER_LIST)
	if !ps.Valid() {
		l.failAt(l.end(n), "expected '('")
	}
	for c := prev(ps); c.Valid(); c = prev(c) {
		if c.NumericKind() == psi.KindTYPE_REFERENCE {
			lam.AnonRecv = l.typeRef(c)
		}
	}
	for _, p := range kids(ps) {
		if p.NumericKind() != psi.KindVALUE_PARAMETER {
			continue
		}
		m := l.modifiers(child(p, psi.KindMODIFIER_LIST))
		syntax.NoContexts(&m, "value parameters", l)
		name := child(p, psi.KindIDENTIFIER)
		lp := &syntax.LambdaParam{P: l.pos(name), Name: ident(name), Mods: m.Written, Annotations: m.Annotations}
		if t := child(p, psi.KindTYPE_REFERENCE); t.Valid() {
			lp.Type = l.typeRef(t)
		}
		if eq := child(p, psi.KindEQ); eq.Valid() {
			// As kotlinc, the function is kept, without the value.
			l.errorLater(l.pos(exprAfter(p, psi.KindEQ)), "anonymous functions can't have default values")
		}
		lam.Params = append(lam.Params, record(l, lp, p))
	}
	for c := next(ps); c.Valid(); c = next(c) {
		if c.NumericKind() == psi.KindTYPE_REFERENCE {
			lam.AnonRet = l.typeRef(c)
			break
		}
	}
	switch {
	case has(n, psi.KindBLOCK):
		lam.Body = l.block(child(n, psi.KindBLOCK))
		lam.AnonUnit = lam.AnonRet == nil
	case has(n, psi.KindEQ):
		x := l.expr(exprAfter(n, psi.KindEQ))
		lam.Body = &syntax.Block{P: x.Pos(), Stmts: []syntax.Stmt{&syntax.ExprStmt{X: x}}}
		lam.AnonExpr = true
	default:
		l.failAt(l.end(n), "expected the body of an anonymous function")
	}
	syntax.LabelReturns(lam.Body, lam.Label)
	return lam
}

func (l *lowerer) ifExpr(n psi.NodeRef) syntax.Expr {
	x := &syntax.If{Cond: l.expr(exprChild(child(n, psi.KindCONDITION)))}
	x.P = l.pos(n)
	then := child(n, psi.KindTHEN)
	if !then.Valid() {
		l.failAt(l.end(n), "expected an expression")
	}
	x.Then = l.body(then)
	if e := child(n, psi.KindELSE); e.Valid() {
		x.Else = l.body(e)
	} else if kw := child(n, psi.KindELSE_KEYWORD); kw.Valid() {
		l.failAt(l.end(kw), "expected an expression")
	}
	return x
}

// confusingCond lowers x, a condition of w, reporting it as kc's parser
// does if w has a subject and x is one kotlinc reserves (see confusing).
func (l *lowerer) confusingCond(w *syntax.When, x psi.NodeRef) syntax.Expr {
	e := l.expr(x)
	if w.Subject != nil && confusing(x) {
		l.errorLater(l.pos(x), confusingMsg)
	}
	return e
}

func (l *lowerer) when(n psi.NodeRef) syntax.Expr {
	w := &syntax.When{}
	w.P = l.pos(n)
	if has(n, psi.KindLPAR) {
		if dd := child(n, psi.KindDESTRUCTURING_DECLARATION); dd.Valid() && has(dd, psi.KindEQ) {
			// As kotlinc, the initializer is the subject, and the names
			// aren't declared.
			l.errorLater(l.pos(dd), "illegal variable declaration in 'when' subject: destructuring declaration; only 'val' variables with initializers are allowed")
			w.Subject = l.expr(exprAfter(dd, psi.KindEQ))
		} else if pd := child(n, psi.KindPROPERTY); pd.Valid() {
			if m := child(pd, psi.KindMODIFIER_LIST); m.Valid() || has(pd, psi.KindVAR_KEYWORD) {
				if !has(pd, psi.KindEQ) || has(pd, psi.KindPROPERTY_DELEGATE) || has(pd, psi.KindTYPE_REFERENCE) && has(pd, psi.KindDOT) {
					l.fail(pd, "unexpected declaration in 'when' subject")
				}
				// As kotlinc, the variable is declared all the same.
				l.errorLater(l.pos(pd), "illegal variable declaration in 'when' subject; only 'val' variables with initializers are allowed")
			}
			if id := child(pd, psi.KindIDENTIFIER); id.Valid() {
				w.SubjVar = ident(id)
			} else if e := child(pd, psi.KindERROR_ELEMENT); unnamedSubject(pd) {
				l.errorLater(l.pos(e), "expecting property name or receiver type")
			}
			l.prov[WhenSubject{w}] = pd.ID()
			if t := child(pd, psi.KindTYPE_REFERENCE); t.Valid() {
				w.SubjType = l.typeRef(t)
			}
			w.Subject = l.expr(exprAfter(pd, psi.KindEQ))
		} else {
			w.Subject = l.expr(exprChild(n))
		}
	}
	for _, e := range kids(n) {
		if e.NumericKind() != psi.KindWHEN_ENTRY {
			continue
		}
		var br *syntax.WhenBranch
		if !l.recovering {
			br = l.whenEntry(w, e)
		} else if !l.attempt(func() {
			l.syntaxError(e)
			br = l.whenEntry(w, e)
		}) {
			br = l.placeholderEntry(e)
		}
		w.Branches = append(w.Branches, record(l, br, e))
	}
	if !has(n, psi.KindRBRACE) {
		l.missingBrace(n)
	}
	return w
}

// whenEntry lowers WHEN_ENTRY e of when w.
func (l *lowerer) whenEntry(w *syntax.When, e psi.NodeRef) *syntax.WhenBranch {
	br := &syntax.WhenBranch{P: l.pos(e), Else: has(e, psi.KindELSE_KEYWORD)}
	for _, c := range kids(e) {
		var wc syntax.WhenCond
		switch c.NumericKind() {
		case psi.KindWHEN_CONDITION_IN_RANGE:
			wc.In = opOf(c).NumericKind() == psi.KindIN_KEYWORD
			wc.NotIn = !wc.In
			wc.P = l.pos(opOf(c))
			wc.X = l.confusingCond(w, exprChild(c))
		case psi.KindWHEN_CONDITION_IS_PATTERN:
			kw := first(c)
			wc.Is, wc.NotIs = kw.NumericKind() == psi.KindIS_KEYWORD, kw.NumericKind() == psi.KindNOT_IS
			wc.P = l.pos(kw)
			wc.Type = l.typeRef(child(c, psi.KindTYPE_REFERENCE))
		case psi.KindWHEN_CONDITION_EXPRESSION:
			wc.X = l.confusingCond(w, exprChild(c))
		case psi.KindWHEN_ENTRY_GUARD:
			br.Guard = l.expr(exprChild(c))
			continue
		default:
			continue
		}
		br.Conds = append(br.Conds, wc)
	}
	body := exprAfter(e, psi.KindARROW)
	if !body.Valid() {
		l.failAt(l.end(e), "expected an expression")
	}
	br.Body = l.bodyOf(body)
	return br
}

// placeholderEntry returns the branch to keep, when recovering, for
// WHEN_ENTRY e that has a syntax error: its condition and its value have
// an invalid type, so that they report nothing of their own, and the when
// keeps its other branches.
func (l *lowerer) placeholderEntry(e psi.NodeRef) *syntax.WhenBranch {
	p := l.pos(e)
	invalid := func() syntax.Expr {
		x := &syntax.As{X: &syntax.NullLit{}, Ref: &syntax.TypeRef{P: p, Known: &types.InvalidType}}
		x.X.(*syntax.NullLit).P = p
		x.P = p
		return x
	}
	// An else branch is kept as one only last, where it is valid.
	last := true
	for n := next(e); n.Valid(); n = next(n) {
		if n.NumericKind() == psi.KindWHEN_ENTRY {
			last = false
		}
	}
	br := &syntax.WhenBranch{P: p, Else: has(e, psi.KindELSE_KEYWORD) && last}
	if !br.Else {
		br.Conds = []syntax.WhenCond{{X: invalid()}}
	}
	br.Body = &syntax.Block{P: p, Stmts: []syntax.Stmt{&syntax.ExprStmt{X: invalid()}}}
	return br
}

func (l *lowerer) try(n psi.NodeRef) syntax.Expr {
	t := &syntax.Try{Body: l.block(child(n, psi.KindBLOCK))}
	t.P = l.pos(n)
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindCATCH:
			ps := child(c, psi.KindVALUE_PARAMETER_LIST)
			var p psi.NodeRef
			for _, x := range kids(ps) {
				if x.NumericKind() == psi.KindVALUE_PARAMETER {
					p = x
					break
				}
			}
			if !p.Valid() {
				l.failAt(l.end(c), "expected a name")
			}
			ct := &syntax.Catch{P: l.pos(c), Name: ident(child(p, psi.KindIDENTIFIER))}
			if m := child(p, psi.KindMODIFIER_LIST); m.Valid() {
				for _, a := range kids(m) {
					switch a.NumericKind() {
					case psi.KindANNOTATION_ENTRY:
						ct.Annotations = append(ct.Annotations, l.annotation(a))
					case psi.KindANNOTATION:
						ct.Annotations = append(ct.Annotations, l.bracketAnnotations(a)...)
					default:
						if !a.IsLeaf() {
							l.unexpected(a)
						}
						// A catch parameter takes no modifiers, as kotlinc's
						// WRONG_MODIFIER_TARGET; the try is kept.
						l.errorLater(l.pos(a), "modifier '%s' is not applicable to 'value parameter'", a.Text())
					}
				}
			}
			ct.Type = l.typeRef(child(p, psi.KindTYPE_REFERENCE))
			ct.Body = l.block(child(c, psi.KindBLOCK))
			t.Catches = append(t.Catches, record(l, ct, c))
		case psi.KindFINALLY:
			t.Finally = l.block(child(c, psi.KindBLOCK))
		}
	}
	if len(t.Catches) == 0 && t.Finally == nil {
		l.failAt(t.P, "'try' must have at least one 'catch' or 'finally' block")
	}
	return t
}

// newDestructuring reports the destructuring forms of Kotlin 2.4 that kc
// doesn't support: positional destructuring in square brackets, [a, b],
// and name-based destructuring, whose entries are declared with val or
// var or rename a property: (val a, val b = second).
func (l *lowerer) newDestructuring(d psi.NodeRef) {
	if lb := child(d, psi.KindLBRACKET); lb.Valid() {
		l.unsupported(l.pos(lb), "positional destructuring", "positional destructuring with square brackets is not supported")
	}
	for _, e := range kids(d) {
		if e.NumericKind() != psi.KindDESTRUCTURING_DECLARATION_ENTRY {
			continue
		}
		if has(e, psi.KindVAL_KEYWORD) || has(e, psi.KindVAR_KEYWORD) || has(e, psi.KindEQ) {
			l.unsupported(l.pos(e), "name-based destructuring", "name-based destructuring is not supported")
		}
	}
}
