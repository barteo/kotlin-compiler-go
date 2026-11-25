package lsp

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type quickFix struct {
	title string
	edits []edit
}

func diagnosticRange(n psi.NodeRef, d analysis.Diagnostic) span {
	switch d.Code {
	case "UNNECESSARY_SAFE_CALL":
		if expr := ancestor(n, psi.KindSAFE_ACCESS_EXPRESSION); expr.Valid() {
			if op := expr.ChildKind(psi.KindSAFE_ACCESS); op.Valid() {
				return rng(op)
			}
		}
	case "USELESS_ELVIS":
		if expr := ancestor(n, psi.KindBINARY_EXPRESSION); expr.Valid() {
			op := expr.ChildKind(psi.KindOPERATION_REFERENCE)
			if op.Text() == "?:" {
				return span{rng(op).Start, rng(expr).End}
			}
		}
	}
	if invalidInner(d.Message) {
		owner := ancestor(n, psi.KindCLASS, psi.KindOBJECT_DECLARATION)
		if token := owner.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindINNER_KEYWORD); token.Valid() {
			return rng(token)
		}
	}
	return rng(n)
}
func invalidInner(msg string) bool {
	return strings.HasPrefix(msg, "modifier 'inner' is not applicable") || strings.HasPrefix(msg, "modifier 'inner' is only applicable")
}
func quickFixes(ctx context.Context, s *psi.Snapshot, i *index, d object) []quickFix {
	r := d["range"].(span)
	off, _ := offset(s, r.Start)
	n := s.NodeAt(off)
	code := d["code"].(string)
	msg := d["message"].(string)
	var out []quickFix
	replace := func(title string, r span, text string) { out = append(out, quickFix{title, []edit{{r, text}}}) }
	switch {
	case code == "UNNECESSARY_NOT_NULL_ASSERTION" && n.Text() == "!!":
		replace("Remove unnecessary !!", rng(n), "")
	case code == "UNNECESSARY_SAFE_CALL" && n.Text() == "?.":
		replace("Replace unnecessary safe call with '.'", rng(n), ".")
	case code == "USELESS_ELVIS" && n.Text() == "?:":
		expr := ancestor(n, psi.KindBINARY_EXPRESSION)
		left := expr.FirstChild()
		if left.Valid() {
			// Retain comments in the discarded expression, including the newline
			// terminating a line comment. The left operand stays byte-for-byte intact.
			start, end := left.Range().End, expr.Range().End
			var comments strings.Builder
			psi.WalkNodes(expr, func(c psi.NodeRef) bool {
				if c.Range().Start >= start && (c.NumericKind() == psi.KindBLOCK_COMMENT || c.NumericKind() == psi.KindEOL_COMMENT) {
					comments.WriteByte(' ')
					comments.WriteString(c.Text())
					if c.NumericKind() == psi.KindEOL_COMMENT {
						if strings.Contains(s.Source(), "\r\n") {
							comments.WriteByte('\r')
						}
						comments.WriteByte('\n')
					}
					return false
				}
				return true
			})
			replace("Remove useless Elvis fallback", span{pos(s, start), pos(s, end)}, comments.String())
		}
	case invalidInner(msg) && n.Text() == "inner":
		end := n.Range().End
		for end < len(s.Source()) && (s.Source()[end] == ' ' || s.Source()[end] == '\t') {
			end++
		}
		replace("Remove 'inner' modifier", span{rng(n).Start, pos(s, end)}, "")
	case strings.HasPrefix(msg, "unresolved reference:") || code == "UNRESOLVED_REFERENCE":
		if n.NumericKind() != psi.KindIDENTIFIER || i.targets[n].Valid() {
			break
		}
		ref := n.Parent()
		if call := ref.Parent(); call.NumericKind() == psi.KindCALL_EXPRESSION && factName(call) == n {
			ref = call
		}
		if parent := ref.Parent(); (parent.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION || parent.NumericKind() == psi.KindSAFE_ACCESS_EXPRESSION) && parent.FirstChild() != ref {
			break
		}
		var names []string
		for _, c := range i.candidates(ctx, n, n.Name(), nil, true) {
			if c.name == n.Name() && c.importPath != "" {
				names = append(names, c.importPath)
			}
		}
		names = append(names, i.model.ImportNames(ctx, n.Name())...)
		slices.Sort(names)
		for _, fq := range slices.Compact(names) {
			conflict := false
			for imp := s.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); imp.Valid(); imp = imp.NextSibling() {
				if imp.Path() == fq || imp.Alias() == n.Name() || imp.Alias() == "" && short(imp.Path()) == n.Name() {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
			if edits := addImports(s, map[string]bool{fq: true}); len(edits) > 0 {
				out = append(out, quickFix{"Import " + fq, edits})
			}
		}
	}
	if d["severity"] == 2 && code != "" {
		out = append(out, suppressionFixes(s, n, code)...)
	}
	return out
}
func suppressionFixes(s *psi.Snapshot, n psi.NodeRef, code string) []quickFix {
	var out []quickFix
	stmt := n
	for stmt.Valid() && stmt.Parent().NumericKind() != psi.KindBLOCK {
		stmt = stmt.Parent()
	}
	for _, scope := range []psi.NodeRef{stmt, ancestor(n, psi.KindFUN), ancestor(n, psi.KindCLASS, psi.KindOBJECT_DECLARATION)} {
		if !scope.Valid() {
			continue
		}
		label := scope.Name()
		if label == "" {
			label = "statement"
		}
		mods := scope.ChildKind(psi.KindMODIFIER_LIST)
		parent := mods
		if !mods.Valid() {
			parent = scope
		}
		var existing psi.NodeRef
		for c := parent.FirstChild(); c.Valid(); c = c.NextSibling() {
			if c.NumericKind() != psi.KindANNOTATION_ENTRY {
				continue
			}
			name := c.ChildKind(psi.KindCONSTRUCTOR_CALLEE).Text()
			if name == "Suppress" || name == "kotlin.Suppress" {
				existing = c
				break
			}
		}
		var e edit
		if existing.Valid() {
			var ok bool
			if e, ok = appendSuppression(existing, code); !ok {
				continue
			}
		} else {
			first := scope.FirstChild()
			for first.Valid() && (first.IsTrivia() || first.NumericKind() == psi.KindKDOC) {
				first = first.NextSibling()
			}
			if !first.Valid() {
				continue
			}
			at := rng(first).Start
			e = edit{span{at, at}, "@Suppress(" + strconv.Quote(code) + ") "}
		}
		fix := quickFix{"Suppress " + code + " for " + label, []edit{e}}
		if !slices.ContainsFunc(out, func(prev quickFix) bool { return slices.Equal(prev.edits, fix.edits) }) {
			out = append(out, fix)
		}
	}
	return out
}

// Keep the existing argument syntax, including named arrays and trailing commas.
func appendSuppression(annotation psi.NodeRef, code string) (edit, bool) {
	list := annotation.ChildKind(psi.KindVALUE_ARGUMENT_LIST)
	close := list.ChildKind(psi.KindRPAR)
	if !close.Valid() {
		return edit{}, false
	}
	for c := list.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() != psi.KindVALUE_ARGUMENT || !c.ChildKind(psi.KindVALUE_ARGUMENT_NAME).Valid() {
			continue
		}
		array := c.ChildKind(psi.KindCOLLECTION_LITERAL_EXPRESSION)
		if !array.Valid() {
			return edit{}, false
		}
		list = array
		close = array.ChildKind(psi.KindRBRACKET)
		break
	}
	if !close.Valid() {
		return edit{}, false
	}
	quoted := strconv.Quote(code)
	last := list.FirstChild()
	for c := list.FirstChild(); c.Valid() && c != close; c = c.NextSibling() {
		if !c.IsTrivia() {
			last = c
		}
		if c.Text() == quoted {
			return edit{}, false
		}
	}
	sep := ", "
	switch last.NumericKind() {
	case psi.KindLPAR, psi.KindLBRACKET:
		sep = ""
	case psi.KindCOMMA:
		sep = " "
	}
	at := rng(close).Start
	return edit{span{at, at}, sep + quoted}, true
}
