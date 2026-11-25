package lsp

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// Format gaps between tokens in the original snapshot. Treat strings and
// KDoc as single tokens so their internal whitespace never becomes an edit.
func format(s *psi.Snapshot, p parameters, limited bool) []edit {
	edits := []edit{}
	if len(s.Diagnostics()) != 0 {
		return edits
	}
	start, end := 0, len(s.Source())
	if limited {
		var startErr, endErr error
		start, startErr = offset(s, p.Range.Start)
		end, endErr = offset(s, p.Range.End)
		if startErr != nil || endErr != nil || start >= end {
			return edits
		}
	}
	size := p.Options.TabSize
	if size < 1 || size > 16 {
		size = 4
	}
	unit := "\t"
	if p.Options.InsertSpaces {
		unit = strings.Repeat(" ", size)
	}

	// A receiver or opening delimiter can be on a line outside a range
	// selection. Keep its actual indent unless we emit an edit for that gap.
	// Key by source offset since formatting can split an original line.
	indents := map[int]string{}
	var scopes []string // effective line indent at each opening delimiter
	var prev psi.NodeRef
	lineIndent := ""
	src := s.Source()
	lineBreak := "\n"
	if strings.Contains(src, "\r\n") {
		lineBreak = "\r\n"
	}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		k := n.NumericKind()
		if !n.IsLeaf() && k != psi.KindKDOC && k != psi.KindSTRING_TEMPLATE {
			return true
		}
		if k == psi.KindWHITE_SPACE || n.Range().Start == n.Range().End {
			return false
		}

		a, b := prev.Range().End, n.Range().Start
		gap := src[a:b]
		newline := strings.LastIndexByte(gap, '\n')
		lineStart := newline >= 0 || !prev.Valid()
		// Edit only the indentation after the last newline. The newline
		// itself can be outside the requested range.
		if lineStart {
			a += newline + 1
			gap = src[a:b]
			lineIndent = gap
		}
		list := parameterOrArgumentList(n.Parent())
		closeScope := k == psi.KindRBRACE || k == psi.KindRPAR && list
		if a >= start && b <= end && a < end && strings.Trim(gap, " \t") == "" {
			wrap := !lineStart && (multilineListEdge(s, prev, psi.KindLPAR) || multilineListEdge(s, n, psi.KindRPAR))
			want := gap
			if lineStart || wrap {
				indent := ""
				if len(scopes) > 0 {
					indent = scopes[len(scopes)-1]
					if !closeScope {
						indent += unit
					}
				}
				if selector(n) {
					indent = indents[n.Parent().Range().Start] + unit
				} else if prev.NumericKind() == psi.KindEQ {
					indent = indents[prev.Range().Start] + unit
				}
				lineIndent = indent
				want = indent
				if wrap {
					want = lineBreak + indent
				}
			} else {
				want = inlineSpacing(prev, n, gap)
			}
			if want != gap {
				edits = append(edits, edit{span{pos(s, a), pos(s, b)}, want})
			}
		}

		indents[b] = lineIndent
		if closeScope && len(scopes) > 0 {
			scopes = scopes[:len(scopes)-1]
		}
		if k == psi.KindLBRACE || k == psi.KindLPAR && list {
			scopes = append(scopes, lineIndent)
		}
		// A raw string or comment can span lines. Its final line may
		// also contain the opening delimiter of a following construct.
		if i := strings.LastIndexByte(n.Text(), '\n'); i >= 0 {
			last := n.Text()[i+1:]
			lineIndent = last[:len(last)-len(strings.TrimLeft(last, " \t"))]
		}
		prev = n
		return false
	})
	return edits
}

func parameterOrArgumentList(n psi.NodeRef) bool {
	return n.NumericKind() == psi.KindVALUE_PARAMETER_LIST || n.NumericKind() == psi.KindVALUE_ARGUMENT_LIST
}

func multilineListEdge(s *psi.Snapshot, token psi.NodeRef, kind psi.Kind) bool {
	if token.NumericKind() != kind || !parameterOrArgumentList(token.Parent()) {
		return false
	}
	r := token.Parent().Range()
	return pos(s, r.Start).Line != pos(s, r.End).Line
}

func selector(n psi.NodeRef) bool {
	switch n.Parent().NumericKind() {
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
		return n.NumericKind() == psi.KindDOT || n.NumericKind() == psi.KindSAFE_ACCESS
	}
	return false
}

func inlineSpacing(prev, next psi.NodeRef, gap string) string {
	left, right := prev.NumericKind(), next.NumericKind()
	switch {
	case prev.IsTrivia() || next.IsTrivia() || left == psi.KindKDOC || right == psi.KindKDOC:
		return gap
	case selector(prev) || selector(next):
		return ""
	case right == psi.KindCOMMA || right == psi.KindCOLON || right == psi.KindRPAR || left == psi.KindLPAR:
		return ""
	case right == psi.KindLPAR && next.Parent().NumericKind() == psi.KindVALUE_ARGUMENT_LIST:
		return ""
	case left == psi.KindLBRACE && right == psi.KindRBRACE:
		return ""
	case right == psi.KindLBRACE || left == psi.KindLBRACE && prev.Parent().NumericKind() == psi.KindFUNCTION_LITERAL ||
		right == psi.KindRBRACE && next.Parent().NumericKind() == psi.KindFUNCTION_LITERAL:
		return " "
	case left == psi.KindCOMMA || left == psi.KindCOLON || left == psi.KindEQ || right == psi.KindEQ ||
		left == psi.KindARROW || right == psi.KindARROW:
		return " "
	}
	for _, n := range [...]psi.NodeRef{prev, next} {
		op := n.Parent()
		if op.NumericKind() == psi.KindOPERATION_REFERENCE && op.Parent().NumericKind() == psi.KindBINARY_EXPRESSION {
			if n.NumericKind() == psi.KindRANGE || n.NumericKind() == psi.KindRANGE_UNTIL {
				return ""
			}
			return " "
		}
	}
	return gap
}
