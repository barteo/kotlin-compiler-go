package lower

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TreeErrors returns the errors kotlinc reports for syntax its grammar
// accepts but its raw FIR builder rejects, found in the tree of s:
//
//   - an assignment where an expression is expected, as in val a = b = c
//     (ASSIGNMENT_IN_EXPRESSION_CONTEXT);
//   - a loop or a declaration where an expression is expected, as in
//     val a = while (c) {} (EXPRESSION_EXPECTED);
//   - a condition of a when with a subject that is a comparison, an in,
//     is, && or || not in parentheses (CONFUSING_BRANCH_CONDITION_ERROR).
//
// kc's parser rejects the first two, and stops at them; the driver reports
// these for a file kc's parser rejects, which psi parses with recovery.
func TreeErrors(s *psi.Snapshot) []*syntax.Error {
	l := &lowerer{s: s, src: s.Source(), path: s.Path()}
	l.lines = append(l.lines, 0)
	for i := 0; i < len(l.src); i++ {
		if l.src[i] == '\n' {
			l.lines = append(l.lines, i+1)
		}
	}
	var errs []*syntax.Error
	report := func(n psi.NodeRef, msg string) {
		errs = append(errs, &syntax.Error{File: l.path, Pos: l.pos(n), Msg: msg, Name: syntax.ErrorName(msg)})
	}
	for id := psi.NodeID(1); int(id) <= s.NodeCount(); id++ {
		n := s.Node(id)
		switch n.NumericKind() {
		case psi.KindBINARY_EXPRESSION:
			if _, ok := assignOps[operation(n)]; ok && inExpressionContext(n) {
				report(n, "assignments are not expressions, and only expressions are allowed in this context")
			}
		case psi.KindWHILE, psi.KindDO_WHILE, psi.KindFOR:
			if inExpressionContext(n) {
				report(n, "expected an expression, found a loop")
			}
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindPROPERTY, psi.KindTYPEALIAS:
			if inExpressionContext(n) {
				report(n, "expected an expression, found a declaration")
			}
		case psi.KindSTRING_TEMPLATE, psi.KindCHARACTER_CONSTANT, psi.KindFLOAT_CONSTANT, psi.KindINTEGER_CONSTANT:
			if p := prevLeaf(n); gluedAffix(p) {
				report(p, gluedLiteralMsg)
			}
			if x := nextLeaf(n); gluedAffix(x) {
				report(x, gluedLiteralMsg)
			}
		case psi.KindWHEN:
			if !has(n, psi.KindLPAR) {
				continue // no subject
			}
			for _, e := range kids(n) {
				if e.NumericKind() != psi.KindWHEN_ENTRY {
					continue
				}
				for _, c := range kids(e) {
					switch c.NumericKind() {
					case psi.KindWHEN_CONDITION_EXPRESSION, psi.KindWHEN_CONDITION_IN_RANGE:
						if x := lastKid(c); x.Valid() && confusing(x) {
							report(x, confusingMsg)
						}
					}
				}
			}
		}
	}
	return errs
}

// gluedLiteralMsg is kotlinc's error for a literal written against an
// identifier, a number or a keyword, as a foo"bar" or 1in a.
const gluedLiteralMsg = "unsupported [literal prefixes and suffixes]"

// gluedAffix reports whether the leaf l, next to a literal, may not touch
// it: an identifier, a number or a keyword. Whitespace, comments and any
// other token separate the two.
func gluedAffix(l psi.NodeRef) bool {
	if !l.Valid() {
		return false
	}
	k := l.NumericKind()
	return k == psi.KindIDENTIFIER || k == psi.KindINTEGER_LITERAL || k == psi.KindFLOAT_LITERAL || psi.IsKeyword(k)
}

// prevLeaf returns the leaf just before n in the file, or an invalid
// NodeRef. Nodes are in preorder, so the nodes before n are its
// ancestors, which aren't leaves, and what ends before it.
func prevLeaf(n psi.NodeRef) psi.NodeRef {
	s := n.ContainingFile()
	for id := n.ID() - 1; id > 0; id-- {
		if c := s.Node(id); c.IsLeaf() {
			return c
		}
	}
	return psi.NodeRef{}
}

// nextLeaf returns the leaf just after n in the file, or an invalid
// NodeRef.
func nextLeaf(n psi.NodeRef) psi.NodeRef {
	s := n.ContainingFile()
	for id := n.SubtreeEnd(); int(id) <= s.NodeCount(); id++ {
		if c := s.Node(id); c.IsLeaf() {
			return c
		}
	}
	return psi.NodeRef{}
}

// confusingOps are the binary operators of a when condition with a subject
// that kotlinc reserves: x == y there reads as subject == (x == y).
const confusingMsg = "confusing branch condition: wrap it in parentheses, or use a guard (if)"

var confusingOps = map[psi.Kind]bool{
	psi.KindIN_KEYWORD: true, psi.KindNOT_IN: true,
	psi.KindLT: true, psi.KindGT: true, psi.KindLTEQ: true, psi.KindGTEQ: true,
	psi.KindEQEQ: true, psi.KindEXCLEQ: true, psi.KindEQEQEQ: true, psi.KindEXCLEQEQ: true,
	psi.KindANDAND: true, psi.KindOROR: true,
}

func confusing(x psi.NodeRef) bool {
	switch x.NumericKind() {
	case psi.KindIS_EXPRESSION:
		return true
	case psi.KindBINARY_EXPRESSION:
		return confusingOps[operation(x)]
	}
	return false
}

// operation returns the kind of the operator token of a binary
// expression, or 0.
func operation(n psi.NodeRef) psi.Kind {
	if op := child(n, psi.KindOPERATION_REFERENCE); op.Valid() {
		if t := first(op); t.Valid() {
			return t.NumericKind()
		}
	}
	return 0
}

// lastKid returns n's last child that isn't trivia.
func lastKid(n psi.NodeRef) psi.NodeRef {
	ks := kids(n)
	if len(ks) == 0 {
		return psi.NodeRef{}
	}
	return ks[len(ks)-1]
}

// expressionContexts are the parents whose children that are expressions
// must be expressions, not statements: the initializer of a property, an
// operand, an argument, a returned value, a function's expression body.
var expressionContexts = map[psi.Kind]bool{
	psi.KindPROPERTY: true, psi.KindFUN: true, psi.KindBINARY_EXPRESSION: true,
	psi.KindBINARY_WITH_TYPE: true, psi.KindIS_EXPRESSION: true,
	psi.KindPREFIX_EXPRESSION: true, psi.KindPOSTFIX_EXPRESSION: true,
	psi.KindPARENTHESIZED: true, psi.KindVALUE_ARGUMENT: true,
	psi.KindRETURN: true, psi.KindTHROW: true, psi.KindCONDITION: true,
	psi.KindWHEN_CONDITION_EXPRESSION: true, psi.KindWHEN_CONDITION_IN_RANGE: true,
	psi.KindLONG_STRING_TEMPLATE_ENTRY: true, psi.KindINDICES: true,
	psi.KindCOLLECTION_LITERAL_EXPRESSION: true, psi.KindPROPERTY_DELEGATE: true,
	psi.KindDELEGATED_SUPER_TYPE_ENTRY: true,
}

// inExpressionContext reports whether n, past the annotations and labels
// on it, is where kotlinc expects an expression.
func inExpressionContext(n psi.NodeRef) bool {
	p := n.Parent()
	for k := p.NumericKind(); k == psi.KindANNOTATED_EXPRESSION || k == psi.KindLABELED_EXPRESSION; k = p.NumericKind() {
		p = p.Parent()
	}
	switch p.NumericKind() {
	case psi.KindPROPERTY, psi.KindFUN:
		// Only as the initializer or the expression body, after '='.
		return prevIsEq(p, n)
	}
	return expressionContexts[p.NumericKind()]
}

// prevIsEq reports whether the child of p that n is in follows '='.
func prevIsEq(p, n psi.NodeRef) bool {
	for n.Parent() != p {
		n = n.Parent()
	}
	q := prev(n)
	return q.Valid() && q.NumericKind() == psi.KindEQ
}

// redundantPrefixes returns the positions of the string literals of the
// file whose interpolation prefix is redundant, as
// syntax.StringLit.RedundantPrefix says, read from the tree: those of
// the statements and declarations that recovery leaves out too, which
// kotlinc's experimental REDUNDANT_INTERPOLATION_PREFIX is reported of
// as of the others. A literal is one whose entries are all text and
// escapes; with any other, a syntax error among them, kotlinc's is a
// concatenation, of which only a single $ is redundant.
//
// A literal inside a syntax error has no expression in kotlinc's tree,
// and one under an annotation that may suppress the warning is left
// out: the checker knows only the annotations of what is kept.
func (l *lowerer) redundantPrefixes() []syntax.Pos {
	var ps []syntax.Pos
	for id := psi.NodeID(1); int(id) <= l.s.NodeCount(); id++ {
		n := l.s.Node(id)
		if n.NumericKind() != psi.KindSTRING_TEMPLATE {
			continue
		}
		p := child(n, psi.KindSTRING_INTERPOLATION_PREFIX)
		if !p.Valid() {
			continue
		}
		prefix := len(p.Text())
		entries := false
		for _, e := range kids(n) {
			switch e.NumericKind() {
			case psi.KindSTRING_INTERPOLATION_PREFIX, psi.KindOPEN_QUOTE, psi.KindCLOSING_QUOTE,
				psi.KindLITERAL_STRING_TEMPLATE_ENTRY, psi.KindESCAPE_STRING_TEMPLATE_ENTRY:
			default:
				entries = true
			}
		}
		if syntax.RedundantPrefixOf(prefix, entries, n.Text()[prefix:]) && !prefixWarningOff(n) {
			ps = append(ps, l.pos(n))
		}
	}
	return ps
}

// prefixWarningOff reports whether string template n is inside a syntax
// error, or under an annotation that may suppress its
// REDUNDANT_INTERPOLATION_PREFIX warning: a @Suppress of that name or of
// all warnings.
func prefixWarningOff(n psi.NodeRef) bool {
	off := func(a psi.NodeRef) bool {
		t := strings.ToLower(a.Text())
		return strings.Contains(t, "suppress") && (strings.Contains(t, "redundant_interpolation_prefix") || strings.Contains(t, `"warnings"`))
	}
	for a := n.Parent(); a.Valid(); a = a.Parent() {
		switch a.NumericKind() {
		case psi.KindERROR_ELEMENT:
			return true
		case psi.KindANNOTATED_EXPRESSION:
			if off(a) {
				return true
			}
		}
		for _, k := range []psi.Kind{psi.KindMODIFIER_LIST, psi.KindFILE_ANNOTATION_LIST} {
			if m := child(a, k); m.Valid() && off(m) {
				return true
			}
		}
	}
	return false
}
