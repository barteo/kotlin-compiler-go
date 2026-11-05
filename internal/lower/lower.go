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
}

// File lowers a psi snapshot to kc's AST, as syntax.ParseAll parses the
// same source: after an error in a declaration, it goes on with the next
// one, so it returns the first error of each declaration that fails.
func File(s *psi.Snapshot) (*syntax.File, *Provenance, []*syntax.Error) {
	l := &lowerer{s: s, src: s.Source(), path: s.Path(), prov: map[any]psi.NodeID{}, annots: map[syntax.Pos]psi.NodeID{}, negated: map[psi.NodeID]*syntax.IntLit{}, hoisted: map[psi.NodeID]bool{}}
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
func (l *lowerer) syntaxError(n psi.NodeRef) {
	lo, hi := n.ID(), n.SubtreeEnd()
	i := sort.Search(len(l.errorIDs), func(i int) bool { return l.errorIDs[i] >= lo })
	if i == len(l.errorIDs) || l.errorIDs[i] >= hi {
		return
	}
	e := l.s.Node(l.errorIDs[i])
	msg := "syntax error"
	r := e.Range()
	for _, d := range l.s.Diagnostics() {
		if d.Range.Start >= r.Start && d.Range.End <= r.End {
			msg = d.Message
			break
		}
	}
	l.failAt(l.posAt(r.Start), "%s", msg)
}

// decl lowers one declaration with lower. An error is recorded, and
// lowering goes on after the declaration.
func (l *lowerer) decl(n psi.NodeRef, lower func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			if k := len(l.errs); k == 0 || *l.errs[k-1] != *l.err {
				l.errs = append(l.errs, l.err)
			}
		}
	}()
	l.syntaxError(n)
	lower()
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
