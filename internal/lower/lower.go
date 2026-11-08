// Package lower builds kc's AST directly from a psi tree, as the token
// parser in internal/syntax would build it from the same source, and
// records which psi node each AST node came from.
//
// The two front ends must agree: lower_test.go checks that, for every
// file the token parser accepts, lowering its psi tree gives the same AST,
// positions included. The desugarings the parser performs while parsing
// are shared through internal/syntax/build.go.
package lower

import (
	"fmt"
	"sort"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Provenance maps AST nodes, as pointers, to the psi nodes they were
// lowered from. Nodes that lowering or the checker synthesize, with no
// psi node of their own, have no entry.
type Provenance struct {
	m map[any]psi.NodeID
	// annots are the ANNOTATION_ENTRYs of annotations, by position:
	// syntax.Annotation is a value the AST copies. The checker parses an
	// annotation's arguments from tokens, so the CALL_EXPRESSIONs in them,
	// which may be nested annotations, are here too, by the position of
	// their '(' as a parsed call's.
	annots map[syntax.Pos]psi.NodeID
	// negated are the PREFIX_EXPRESSIONs folded into negative literals:
	// -1 is a literal, which the AST keeps for the INTEGER_CONSTANT.
	negated map[psi.NodeID]*syntax.IntLit
}

// Negated calls fn for each PREFIX_EXPRESSION lowered to a negative
// literal and the literal, in no particular order.
func (p *Provenance) Negated(fn func(prefix psi.NodeID, lit *syntax.IntLit)) {
	if p == nil {
		return
	}
	for id, lit := range p.negated {
		fn(id, lit)
	}
}

// Annotation returns the ANNOTATION_ENTRY of the annotation at p, the
// CALL_EXPRESSION of a nested annotation whose '(' is at p, or 0.
func (p *Provenance) Annotation(at syntax.Pos) psi.NodeID {
	if p == nil {
		return 0
	}
	return p.annots[at]
}

// Node returns the psi node AST node n was lowered from, or 0.
func (p *Provenance) Node(n any) psi.NodeID {
	if p == nil {
		return 0
	}
	return p.m[n]
}

// Len returns the number of AST nodes with a psi node.
func (p *Provenance) Len() int {
	if p == nil {
		return 0
	}
	return len(p.m)
}

// Each calls fn for each AST node and its psi node, in no particular
// order.
func (p *Provenance) Each(fn func(n any, id psi.NodeID)) {
	if p == nil {
		return
	}
	for n, id := range p.m {
		fn(n, id)
	}
}

// WhenSubject is the provenance key of the PROPERTY a when declares
// its subject with, when (val x = e): the AST keeps only its name.
type WhenSubject struct{ When *syntax.When }

type bailout struct{}

type lowerer struct {
	s     *psi.Snapshot
	src   string
	path  string
	lines []int // the offsets at which lines start
	prov  map[any]psi.NodeID
	// annots are the annotation entries, by position.
	annots map[syntax.Pos]psi.NodeID
	// negated are the prefix expressions folded into negative literals.
	negated map[psi.NodeID]*syntax.IntLit

	err   *syntax.Error
	errs  []*syntax.Error
	later []*syntax.Error

	// errorIDs are the ERROR_ELEMENT nodes, in order.
	errorIDs []psi.NodeID

	annotated map[any][]syntax.Annotation
	anonFuns  int

	// hoisted are the ANNOTATED_EXPRESSIONs whose annotations were
	// lowered as their statement's.
	hoisted map[psi.NodeID]bool

	// carry is the companion object name detachName left as a modifier
	// of the next member.
	carry psi.NodeRef

	suppressInvisible, suppressShadowed, suppressDSL bool

	// noTrailing is set while lowering where a brace after an expression
	// isn't a trailing lambda for kc: a delegate in a class header.
	noTrailing bool

	// signatures is set by Signatures: block bodies are lowered empty.
	signatures bool

	// recover is set by Recover and Signatures: an error in a statement
	// of a block, or in the expression after '=' of a property, function
	// or accessor, fails only that statement or expression, which is
	// lowered as an ErrorExpr.
	recover bool
}

// File lowers a psi snapshot to kc's AST, as syntax.ParseAll parses the
// same source: after an error in a declaration, it goes on with the next
// one, so it returns the first error of each declaration that fails.
func File(s *psi.Snapshot) (*syntax.File, *Provenance, []*syntax.Error) {
	return lowerFile(s, false, false)
}

// Recover lowers a snapshot as File does, but recovers from errors in
// statements and expressions too: a statement of a block in braces that
// fails, or the expression after '=' of a property, function or
// accessor, is lowered as a syntax.ErrorExpr, with its error returned,
// and the declaration is kept. A block that psi left without its closing
// brace still fails its declaration: what follows it is lost to it.
func Recover(s *psi.Snapshot) (*syntax.File, *Provenance, []*syntax.Error) {
	return lowerFile(s, false, true)
}

// Signatures lowers a snapshot as File does, but for the block bodies of
// functions, property accessors, class initializers and secondary
// constructors, which are lowered empty: what a check of other files
// that only declares this one (check.Options.Only) needs of it. A body
// keeps its contract, and an inline function's body is lowered whole:
// the checker reads those of the files it doesn't check too. It recovers
// as Recover does, and the errors in the bodies it doesn't lower aren't
// found.
func Signatures(s *psi.Snapshot) (*syntax.File, *Provenance, []*syntax.Error) {
	return lowerFile(s, true, true)
}

func lowerFile(s *psi.Snapshot, signatures, recover bool) (*syntax.File, *Provenance, []*syntax.Error) {
	l := &lowerer{signatures: signatures, recover: recover, s: s, src: s.Source(), path: s.Path(), prov: map[any]psi.NodeID{}, annots: map[syntax.Pos]psi.NodeID{}, negated: map[psi.NodeID]*syntax.IntLit{}, hoisted: map[psi.NodeID]bool{}}
	l.lines = append(l.lines, 0)
	for i := 0; i < len(l.src); i++ {
		if l.src[i] == '\n' {
			l.lines = append(l.lines, i+1)
		}
	}
	for id := psi.NodeID(1); int(id) <= s.NodeCount(); id++ {
		if s.Node(id).NumericKind() == psi.KindERROR_ELEMENT {
			l.errorIDs = append(l.errorIDs, id)
		}
	}
	f := l.file(s.RootNode())
	if len(l.errs) == 0 && !signatures {
		// Every error element is under a declaration that reports it;
		// this keeps a file with one from ever lowering cleanly.
		for _, id := range l.errorIDs {
			if !l.recoverable(s.Node(id)) {
				l.decl(s.RootNode(), func() { l.failSyntax(s.Node(id)) })
				break
			}
		}
	}
	f.Errors = l.later
	f.SuppressInvisible = l.suppressInvisible
	f.SuppressShadowed = l.suppressShadowed
	f.SuppressDSL = l.suppressDSL
	f.Annotated = l.annotated
	syntax.FinishFile(f, l.path)
	return f, &Provenance{l.prov, l.annots, l.negated}, l.errs
}

// record notes that AST node x comes from n, and returns x.
func record[T any](l *lowerer, x T, n psi.NodeRef) T {
	l.prov[x] = n.ID()
	return x
}

// ---- positions ----

// posAt returns the position of byte offset off: lines count '\n's and
// columns bytes.
func (l *lowerer) posAt(off int) syntax.Pos {
	i := sort.Search(len(l.lines), func(i int) bool { return l.lines[i] > off }) - 1
	return syntax.Pos{Line: i + 1, Col: off - l.lines[i] + 1}
}

// pos returns the position of n's first token: its first leaf that isn't
// trivia, or, for a node without one, the first after it.
func (l *lowerer) pos(n psi.NodeRef) syntax.Pos {
	return l.posAt(l.tokenStart(n))
}

func (l *lowerer) tokenStart(n psi.NodeRef) int { return l.tokenStartFrom(n.ID()) }

// tokenStartFrom returns the offset of the first token at or after node
// id in preorder, or the end of the source.
func (l *lowerer) tokenStartFrom(from psi.NodeID) int {
	for id := from; int(id) <= l.s.NodeCount(); id++ {
		c := l.s.Node(id)
		if c.IsLeaf() && !c.IsTrivia() {
			return c.Range().Start
		}
	}
	return len(l.src)
}

// labeledKeywords are the expressions whose keyword kc scans together
// with the label after it.
var labeledKeywords = map[psi.Kind]bool{
	psi.KindTHIS_EXPRESSION: true, psi.KindSUPER_EXPRESSION: true, psi.KindRETURN: true,
	psi.KindBREAK: true, psi.KindCONTINUE: true,
}

// lastTok returns the position of n's last token: its last leaf that
// isn't trivia or a semicolon. kc scans a string template as one token,
// and a keyword with its label, this@L, as another, so a leaf inside
// one stands for the whole.
func (l *lowerer) lastTok(n psi.NodeRef) syntax.Pos {
	for id := n.SubtreeEnd() - 1; id >= n.ID(); id-- {
		c := l.s.Node(id)
		if !c.IsLeaf() || c.IsTrivia() || c.Range().Start == c.Range().End || c.NumericKind() == psi.KindSEMICOLON {
			continue
		}
		for p := c.Parent(); p.Valid() && p.ID() >= n.ID(); p = p.Parent() {
			switch p.NumericKind() {
			case psi.KindSTRING_TEMPLATE:
				c = p
			case psi.KindLABEL_QUALIFIER:
				// this@L, return@L: kc scans the keyword and its label
				// as one token.
				if q := p.Parent(); labeledKeywords[q.NumericKind()] && q.ID() >= n.ID() {
					c = q
				}
			}
		}
		return l.pos(c)
	}
	return l.pos(n)
}

// end returns the position just after n's text.
func (l *lowerer) end(n psi.NodeRef) syntax.Pos { return l.posAt(n.Range().End) }

// ---- errors ----

func (l *lowerer) failAt(pos syntax.Pos, format string, args ...any) {
	l.err = &syntax.Error{File: l.path, Pos: pos, Msg: fmt.Sprintf(format, args...)}
	panic(bailout{})
}

func (l *lowerer) fail(n psi.NodeRef, format string, args ...any) {
	l.failAt(l.pos(n), format, args...)
}

func (l *lowerer) unsupported(pos syntax.Pos, feature, format string, args ...any) {
	l.err = &syntax.Error{File: l.path, Pos: pos, Msg: fmt.Sprintf(format, args...), Feature: feature}
	panic(bailout{})
}

func (l *lowerer) errorLater(pos syntax.Pos, format string, args ...any) {
	l.later = append(l.later, &syntax.Error{File: l.path, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// unexpected fails on node n, which kc's grammar has no place for.
func (l *lowerer) unexpected(n psi.NodeRef) {
	if !n.Valid() {
		l.failAt(l.posAt(len(l.src)), "unexpected end of file")
	}
	l.fail(n, "unexpected %s", strings.ToLower(n.Kind()))
}

// syntaxError fails if n contains a psi syntax error, with its message.
// Errors inside a class body under n don't count: each member of a body
// is lowered with decl, which reports them, so the class keeps its other
// members.
func (l *lowerer) syntaxError(n psi.NodeRef) {
	lo, hi := n.ID(), n.SubtreeEnd()
	i := sort.Search(len(l.errorIDs), func(i int) bool { return l.errorIDs[i] >= lo })
	for ; i < len(l.errorIDs) && l.errorIDs[i] < hi; i++ {
		e := l.s.Node(l.errorIDs[i])
		if l.inBody(e, n) || l.recoverable(e) || l.recovers(e, n) {
			continue
		}
		l.failSyntax(e)
	}
}

// recoverable reports whether the psi error element e leaves its
// declaration whole: a value parameter written without its type, which
// paramType gives an invalid type and reports, an object declaration
// without its name, which class names as FIR does and reports, or a
// property type's unclosed type argument list, which prop keeps as far as
// it parsed, as FIR does, and reports.
func (l *lowerer) recoverable(e psi.NodeRef) bool {
	p := e.Parent()
	if unclosedPropType(e) {
		return true
	}
	if p.NumericKind() == psi.KindOBJECT_DECLARATION && !first(e).Valid() && !has(p, psi.KindIDENTIFIER) {
		return true
	}
	return p.NumericKind() == psi.KindVALUE_PARAMETER && !has(p, psi.KindTYPE_REFERENCE) &&
		has(p, psi.KindIDENTIFIER) && p.Parent().NumericKind() == psi.KindVALUE_PARAMETER_LIST &&
		(p.Parent().Parent().NumericKind() == psi.KindPRIMARY_CONSTRUCTOR ||
			p.Parent().Parent().NumericKind() == psi.KindFUN ||
			p.Parent().Parent().NumericKind() == psi.KindSECONDARY_CONSTRUCTOR)
}

// unclosedPropType reports whether the psi error element e is a property
// type's unclosed type argument list, Bar<out P in val x: Bar<out P!!>, or
// the rest of the property after that type, which psi leaves in one error
// element.
func unclosedPropType(e psi.NodeRef) bool {
	p := e.Parent()
	if p.NumericKind() == psi.KindPROPERTY {
		t := prev(e)
		return t.NumericKind() == psi.KindTYPE_REFERENCE && unclosedTypeArgs(t).Valid()
	}
	if p.NumericKind() != psi.KindTYPE_ARGUMENT_LIST || first(e).Valid() || next(e).Valid() {
		return false
	}
	for n := p.Parent(); ; n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindUSER_TYPE, psi.KindNULLABLE_TYPE, psi.KindTYPE_PROJECTION, psi.KindTYPE_ARGUMENT_LIST:
		case psi.KindTYPE_REFERENCE:
			if n.Parent().NumericKind() == psi.KindPROPERTY && prev(n).NumericKind() == psi.KindCOLON {
				return true
			}
		default:
			return false
		}
	}
}

// unclosedTypeArgs returns the error element that ends an unclosed type
// argument list in type reference t, or an invalid NodeRef.
func unclosedTypeArgs(t psi.NodeRef) psi.NodeRef {
	var found psi.NodeRef
	var walk func(n psi.NodeRef)
	walk = func(n psi.NodeRef) {
		for _, c := range kids(n) {
			if found.Valid() {
				return
			}
			if c.NumericKind() == psi.KindERROR_ELEMENT && n.NumericKind() == psi.KindTYPE_ARGUMENT_LIST && unclosedPropType(c) {
				found = c
				return
			}
			walk(c)
		}
	}
	walk(t)
	return found
}

// inBody reports whether e is inside a CLASS_BODY strictly under n.
func (l *lowerer) inBody(e, n psi.NodeRef) bool {
	for p := e.Parent(); p.Valid() && p.ID() > n.ID(); p = p.Parent() {
		if p.NumericKind() == psi.KindCLASS_BODY {
			return true
		}
	}
	return false
}

// failSyntax fails on the psi error element e, with its message.
func (l *lowerer) failSyntax(e psi.NodeRef) {
	pos, msg := l.syntaxMessage(e)
	l.err = &syntax.Error{File: l.path, Pos: pos, Msg: msg, Code: "SYNTAX"}
	panic(bailout{})
}

// syntaxMessage returns the position and message of the psi error
// element e.
func (l *lowerer) syntaxMessage(e psi.NodeRef) (syntax.Pos, string) {
	msg := "syntax error"
	r := e.Range()
	for _, d := range l.s.Diagnostics() {
		if d.Range.Start >= r.Start && d.Range.End <= r.End {
			msg = d.Message
			break
		}
	}
	return l.posAt(r.Start), msg
}

// recovers reports whether, with Recover, the psi error element e under
// n is left to a statement or expression that recovers from it: e is in
// a statement of a block in braces under n, or after the '=' of a
// property, function or accessor under n, or n itself.
func (l *lowerer) recovers(e, n psi.NodeRef) bool {
	if !l.recover {
		return false
	}
	for p := e; p.Valid() && p.ID() > n.ID(); p = p.Parent() {
		if par := p.Parent(); recoveringBlock(par) || InTail(par, p) {
			return true
		}
	}
	return false
}

// recoveringBlock reports whether n is a BLOCK whose statements are each
// lowered with blockStmt: one in braces, or a lambda's, closed.
func recoveringBlock(n psi.NodeRef) bool {
	if n.NumericKind() != psi.KindBLOCK {
		return false
	}
	if has(n, psi.KindLBRACE) {
		return has(n, psi.KindRBRACE)
	}
	p := n.Parent()
	return p.NumericKind() == psi.KindFUNCTION_LITERAL && has(p, psi.KindRBRACE)
}

// InTail reports whether c, a child of n, follows the '=' of n, a
// property, function or accessor, up to a property's accessors: the
// expression Recover lowers as an ErrorExpr if it fails.
func InTail(n, c psi.NodeRef) bool {
	switch n.NumericKind() {
	case psi.KindPROPERTY, psi.KindFUN, psi.KindPROPERTY_ACCESSOR:
	default:
		return false
	}
	eq := child(n, psi.KindEQ)
	if !eq.Valid() || c.ID() <= eq.ID() {
		return false
	}
	for s := eq.NextSibling(); s.Valid(); s = s.NextSibling() {
		switch s.NumericKind() {
		case psi.KindPROPERTY_ACCESSOR, psi.KindBACKING_FIELD:
			return false
		}
		if s.ID() == c.ID() {
			return true
		}
	}
	return false
}

// tailExpr lowers the expression after the '=' of n, a property,
// function or accessor. With Recover, an expression that fails, or that
// psi syntax errors follow, is recorded as an error and lowered as an
// ErrorExpr.
func (l *lowerer) tailExpr(n psi.NodeRef) syntax.Expr {
	x := exprAfter(n, psi.KindEQ)
	if !l.recover {
		return l.expr(x)
	}
	var out syntax.Expr
	eq := child(n, psi.KindEQ)
	r, at := eq.Range(), l.pos(eq)
	for c := eq.NextSibling(); c.Valid() && InTail(n, c); c = c.NextSibling() {
		if !c.IsTrivia() {
			r.End, at = c.Range().End, l.lastTok(c)
		}
	}
	if l.recoveredIn(r, at, func() {
		for c := eq.NextSibling(); c.Valid() && InTail(n, c); c = c.NextSibling() {
			l.syntaxError(c)
		}
		out = l.expr(x)
	}) {
		return out
	}
	e := &syntax.ErrorExpr{}
	if x.Valid() {
		e.P = l.pos(x)
	} else {
		e.P = l.afterToken(eq)
	}
	return e
}

// decl lowers one declaration with lower. An error is recorded, and
// lowering goes on after the declaration.
func (l *lowerer) decl(n psi.NodeRef, lower func()) {
	l.recovered(func() {
		l.syntaxError(n)
		lower()
	})
}

// recoveredIn is recovered for lowering the code at r. An error found
// outside r, as an expression missing at the end of the file, is put at
// at, so that the error's diagnostic, and the facts it invalidates, stay
// the code's.
func (l *lowerer) recoveredIn(r psi.Range, at syntax.Pos, lower func()) bool {
	return l.recovered(func() {
		defer func() {
			if v := recover(); v != nil {
				if _, ok := v.(bailout); ok {
					if off := l.offset(l.err.Pos); off < r.Start || off >= r.End {
						e := *l.err
						e.Pos = at
						l.err = &e
					}
				}
				panic(v)
			}
		}()
		lower()
	})
}

// offset returns the offset of pos.
func (l *lowerer) offset(pos syntax.Pos) int {
	if pos.Line < 1 || pos.Line > len(l.lines) {
		return len(l.src)
	}
	return l.lines[pos.Line-1] + pos.Col - 1
}

// recovered runs lower, and reports whether it succeeded: when it fails,
// its error is recorded, unless it repeats the last one.
func (l *lowerer) recovered(lower func()) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, b := r.(bailout); !b {
				panic(r)
			}
			if k := len(l.errs); k == 0 || *l.errs[k-1] != *l.err {
				l.errs = append(l.errs, l.err)
			}
			ok = false
		}
	}()
	lower()
	return true
}

// ---- tree helpers ----

// kids returns n's children that aren't trivia.
func kids(n psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsTrivia() {
			out = append(out, c)
		}
	}
	return out
}

// first returns n's first child that isn't trivia.
func first(n psi.NodeRef) psi.NodeRef {
	c := n.FirstChild()
	for c.Valid() && c.IsTrivia() {
		c = c.NextSibling()
	}
	return c
}

// next returns the first sibling after n that isn't trivia.
func next(n psi.NodeRef) psi.NodeRef {
	c := n.NextSibling()
	for c.Valid() && c.IsTrivia() {
		c = c.NextSibling()
	}
	return c
}

// prev returns the first sibling before n that isn't trivia.
func prev(n psi.NodeRef) psi.NodeRef {
	c := n.PreviousSibling()
	for c.Valid() && c.IsTrivia() {
		c = c.PreviousSibling()
	}
	return c
}

// child returns n's first child of kind k.
func child(n psi.NodeRef, k psi.Kind) psi.NodeRef { return n.ChildKind(k) }

// has reports whether n has a child of kind k.
func has(n psi.NodeRef, k psi.Kind) bool { return n.ChildKind(k).Valid() }

// childAfter returns the first child of n after the first child of kind
// after that isn't trivia.
func childAfter(n psi.NodeRef, after psi.Kind) psi.NodeRef {
	c := n.ChildKind(after)
	if !c.Valid() {
		return psi.NodeRef{}
	}
	return next(c)
}

// ident returns the text of an IDENTIFIER leaf without backticks.
func ident(n psi.NodeRef) string {
	s := n.Text()
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1]
	}
	return s
}

// newlineBetween reports whether there is a line break between nodes a
// and b, siblings or not.
func (l *lowerer) newlineBetween(a, b psi.NodeRef) bool {
	return strings.Contains(l.src[a.Range().End:l.tokenStart(b)], "\n")
}

// isExpr reports whether n is an expression node.
func isExpr(n psi.NodeRef) bool { return exprKinds[n.NumericKind()] }

var exprKinds = func() map[psi.Kind]bool {
	m := map[psi.Kind]bool{}
	for _, k := range []psi.Kind{
		psi.KindREFERENCE_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION,
		psi.KindCALL_EXPRESSION, psi.KindBINARY_EXPRESSION, psi.KindBINARY_WITH_TYPE, psi.KindIS_EXPRESSION,
		psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION, psi.KindPARENTHESIZED, psi.KindINTEGER_CONSTANT,
		psi.KindFLOAT_CONSTANT, psi.KindCHARACTER_CONSTANT, psi.KindBOOLEAN_CONSTANT, psi.KindNULL,
		psi.KindSTRING_TEMPLATE, psi.KindTHIS_EXPRESSION, psi.KindSUPER_EXPRESSION, psi.KindIF, psi.KindWHEN,
		psi.KindTRY, psi.KindFOR, psi.KindWHILE, psi.KindDO_WHILE, psi.KindRETURN, psi.KindBREAK,
		psi.KindCONTINUE, psi.KindTHROW, psi.KindLAMBDA_EXPRESSION, psi.KindANNOTATED_EXPRESSION,
		psi.KindLABELED_EXPRESSION, psi.KindARRAY_ACCESS_EXPRESSION, psi.KindCLASS_LITERAL_EXPRESSION,
		psi.KindCALLABLE_REFERENCE_EXPRESSION, psi.KindOBJECT_LITERAL, psi.KindCOLLECTION_LITERAL_EXPRESSION,
		psi.KindFUN, psi.KindBLOCK, psi.KindENUM_ENTRY_SUPERCLASS_REFERENCE_EXPRESSION,
	} {
		m[k] = true
	}
	return m
}()

// exprChild returns n's first child that is an expression.
func exprChild(n psi.NodeRef) psi.NodeRef {
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if isExpr(c) {
			return c
		}
	}
	return psi.NodeRef{}
}

// exprAfter returns the first expression child of n after its first
// child of kind k.
func exprAfter(n psi.NodeRef, k psi.Kind) psi.NodeRef {
	c := n.ChildKind(k)
	if !c.Valid() {
		return psi.NodeRef{}
	}
	for c = c.NextSibling(); c.Valid(); c = c.NextSibling() {
		if isExpr(c) {
			return c
		}
	}
	return psi.NodeRef{}
}
