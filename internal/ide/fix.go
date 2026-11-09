package ide

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// A Fix is a quick fix of a diagnostic: edits of the document's text,
// and of other files' for a fix that reaches them.
type Fix struct {
	Title     string
	Preferred bool
	Edits     []FixEdit
}

// A FixEdit replaces a range of a snapshot's text.
type FixEdit struct {
	Loc
	Text string
}

// QuickFixes returns the fixes of d, a diagnostic of doc whose range is
// of doc's text. The fixes read doc's syntax around the diagnostic, its
// Data, and v, the module's view (nil if there is none), for what the
// syntax doesn't tell. A fix whose edit can't be computed is left out.
func QuickFixes(v *View, doc *Document, d Diagnostic) []Fix {
	f := fixer{v: v, doc: doc, snap: doc.Snap, d: d}
	switch d.Code {
	case UnusedImport:
		return f.removeImport("Remove unused import")
	case UnusedVariable:
		return f.removeVariable()
	}
	return nil
}

type fixer struct {
	v    *View
	doc  *Document
	snap *psi.Snapshot
	d    Diagnostic
}

func (f fixer) edit(r psi.Range, text string) FixEdit {
	return FixEdit{Loc{f.snap, r}, text}
}

func (f fixer) fix(title string, preferred bool, edits ...FixEdit) []Fix {
	return []Fix{{Title: title, Preferred: preferred, Edits: edits}}
}

// enclosing returns the innermost node of kind k that holds the
// diagnostic's start.
func (f fixer) enclosing(k psi.Kind) psi.NodeRef {
	for n := f.snap.NodeAt(f.d.Range.Start); n.Valid(); n = n.Parent() {
		if n.NumericKind() == k {
			return n
		}
	}
	return psi.NodeRef{}
}

// lineRange extends r to whole lines when nothing but blanks share them
// with r: from the start of its first line through its last line's
// newline. Otherwise it is r.
func lineRange(src string, r psi.Range) psi.Range {
	start := r.Start
	for start > 0 && (src[start-1] == ' ' || src[start-1] == '\t') {
		start--
	}
	end := r.End
	for end < len(src) && (src[end] == ' ' || src[end] == '\t' || src[end] == '\r') {
		end++
	}
	if (start == 0 || src[start-1] == '\n') && (end == len(src) || src[end] == '\n') {
		if end < len(src) {
			end++
		}
		return psi.Range{Start: start, End: end}
	}
	return r
}

// removeImport removes the import directive the diagnostic is on.
func (f fixer) removeImport(title string) []Fix {
	imp := f.enclosing(psi.KindIMPORT_DIRECTIVE)
	if !imp.Valid() {
		return nil
	}
	return f.fix(title, true, f.edit(lineRange(f.snap.Source(), imp.Range()), ""))
}

// removeVariable removes the unused local the diagnostic names; an
// initializer that calls something stays, as a statement.
func (f fixer) removeVariable() []Fix {
	prop := f.enclosing(psi.KindPROPERTY)
	if !prop.Valid() {
		return nil
	}
	var init psi.NodeRef
	eq := false
	for c := prop.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.Text() == "=" {
			eq = true
			continue
		}
		if eq && !trivia(c) {
			init = c
			break
		}
	}
	if init.Valid() && hasCall(init) {
		return f.fix("Remove variable '"+prop.Name()+"', keeping its initializer", true, f.edit(prop.Range(), init.Text()))
	}
	return f.fix("Remove variable '"+prop.Name()+"'", true, f.edit(lineRange(f.snap.Source(), prop.Range()), ""))
}

// trivia reports whether n is whitespace or a comment.
func trivia(n psi.NodeRef) bool {
	return strings.TrimSpace(n.Text()) == "" || n.NumericKind() == psi.KindEOL_COMMENT || n.NumericKind() == psi.KindBLOCK_COMMENT
}

// hasCall reports whether n may have an effect: it holds a call, an
// operator, an assignment or a lambda.
func hasCall(n psi.NodeRef) bool {
	found := false
	psi.WalkNodes(n, func(m psi.NodeRef) bool {
		switch m.NumericKind() {
		case psi.KindCALL_EXPRESSION, psi.KindBINARY_EXPRESSION, psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION,
			psi.KindARRAY_ACCESS_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION, psi.KindOBJECT_LITERAL:
			found = true
		}
		return !found
	})
	return found
}
