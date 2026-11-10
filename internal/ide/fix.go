package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// A Fix is a quick fix of a diagnostic, or a refactoring: edits of the
// document's text, and of other files' for a fix that reaches them.
type Fix struct {
	Title     string
	Preferred bool
	Edits     []FixEdit
	// Kind is a refactoring's code action kind, RefactorExtract,
	// RefactorInline or RefactorRewrite; "" for a quick fix.
	Kind string
}

// A FixEdit replaces a range of a snapshot's text.
type FixEdit struct {
	Loc
	Text string
}

// QuickFixes returns the fixes of d, a diagnostic of doc whose range is
// of doc's text. The fixes read doc's syntax around the diagnostic, its
// Data, env, and v, the module's view (nil if there is none), for what
// the syntax doesn't tell. A fix whose edit can't be computed is left
// out.
func QuickFixes(v *View, doc *Document, d Diagnostic, env FixEnv) []Fix {
	return fixer{v: v, doc: doc, snap: doc.Snap, d: d}.fixes(env)
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

// FixEnv is what fixes read beyond the diagnostic and the view.
type FixEnv struct {
	// Importables returns the declarations named name that the file may
	// import: not in its package or a default import's. Nil for none.
	Importables func(name string) []analysis.Importable
	// Diagnostics are all the document's diagnostics, of which the
	// members a class doesn't implement are merged into one fix.
	Diagnostics []Diagnostic
}

// Codes of the diagnostics of members a class doesn't implement.
var notImplemented = map[string]bool{
	"ABSTRACT_MEMBER_NOT_IMPLEMENTED":               true,
	"ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED":         true,
	"ABSTRACT_MEMBER_NOT_IMPLEMENTED_BY_ENUM_ENTRY": true,
}

// fixes returns the fixes of f.d by its code.
func (f fixer) fixes(env FixEnv) []Fix {
	switch d := f.d; {
	case d.Code == UnusedImport:
		return f.removeImport("Remove unused import")
	case d.Code == UnusedVariable:
		return f.removeVariable()
	case d.Code == "UNRESOLVED_REFERENCE":
		return f.importFixes(env)
	case d.Code == "UNRESOLVED_IMPORT":
		return f.removeImport("Remove unresolved import")
	case d.Code == "UNSAFE_CALL":
		return f.safeCall()
	case d.Code == "VAL_REASSIGNMENT":
		return f.valToVar()
	case notImplemented[d.Code]:
		var stubs []string
		for _, e := range env.Diagnostics {
			if notImplemented[e.Code] && e.Range == d.Range && len(e.Data) > 0 && !slices.Contains(stubs, e.Data[0]) {
				stubs = append(stubs, e.Data[0])
			}
		}
		if len(d.Data) > 0 && !slices.Contains(stubs, d.Data[0]) {
			stubs = append(stubs, d.Data[0])
		}
		return f.implement("Implement members", stubs)
	case d.Code == "MANY_IMPL_MEMBER_NOT_IMPLEMENTED" && len(d.Data) > 0:
		return f.implement("Override '"+stubName(d.Data[0])+"'", d.Data[:1])
	case d.Code == "NO_ELSE_IN_WHEN":
		return f.whenBranches()
	case d.Code == "TYPE_MISMATCH" && len(d.Data) == 2:
		return append(f.changeType(), f.notNull()...)
	case d.Code == "VIRTUAL_MEMBER_HIDDEN":
		return f.fix("Add 'override' modifier", true, f.edit(psi.Range{Start: d.Range.Start, End: d.Range.Start}, "override "))
	case d.Code == "NOTHING_TO_OVERRIDE":
		return f.removeOverride()
	case d.Code == "MUST_BE_INITIALIZED_OR_BE_ABSTRACT":
		return f.initialize()
	case d.Code == "RETURN_TYPE_MISMATCH_ON_OVERRIDE" && len(d.Data) > 0:
		return f.overrideReturnType()
	case d.Code == UnnecessarySafeCall:
		return f.fix("Replace with dot call", true, f.edit(d.Range, "."))
	case d.Code == UnnecessaryNotNullAssertion:
		return f.fix("Remove unnecessary non-null assertion (!!)", true, f.edit(d.Range, ""))
	case d.Code == UselessCast:
		return f.removeCast()
	case d.Code == CanBeVal:
		return f.fix("Change to 'val'", true, f.edit(d.Range, "val"))
	case d.Code == UnusedParameter:
		return f.removeParameter()
	case d.Code == UnusedPrivateMember:
		return f.removeMember()
	case findingCodes[d.Code]:
		api := ""
		if f.v != nil {
			api = f.v.ExplicitAPI
		}
		return f.findingFix(styleFindings(f.snap, api))
	}
	return nil
}

// declaration returns the innermost declaration of one of kinds that
// holds the diagnostic's start.
func (f fixer) declaration(kinds ...psi.Kind) psi.NodeRef {
	for n := f.snap.NodeAt(f.d.Range.Start); n.Valid(); n = n.Parent() {
		if slices.Contains(kinds, n.NumericKind()) {
			return n
		}
	}
	return psi.NodeRef{}
}

// importFixes imports the unresolved name, one fix per candidate: a
// class in a type, a class or function as a callee, anything else as a
// plain name. A qualified name's selector, a member, has none.
func (f fixer) importFixes(env FixEnv) []Fix {
	if env.Importables == nil || len(f.d.Data) == 0 {
		return nil
	}
	name := f.d.Data[0]
	id := f.snap.NodeAt(f.d.Range.Start)
	ref := id.Parent()
	if ref.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		return nil
	}
	pos := ref
	if p := ref.Parent(); p.NumericKind() == psi.KindCALL_EXPRESSION && p.FirstChild() == ref {
		pos = p
	}
	classes := false
	switch p := pos.Parent(); p.NumericKind() {
	case psi.KindUSER_TYPE:
		if p.ChildKind(psi.KindREFERENCE_EXPRESSION) != ref || p.ChildKind(psi.KindUSER_TYPE).Valid() {
			return nil // a qualified type's selector
		}
		classes = true
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION, psi.KindCALLABLE_REFERENCE_EXPRESSION:
		if p.FirstChild() != pos {
			return nil // a selector: a member or an extension, which isn't looked up by name here
		}
	}
	cands := env.Importables(name)
	var out []Fix
	for _, c := range cands {
		if classes && !c.Class {
			continue
		}
		fq := c.Package + "." + c.Name
		e := ImportEdit(f.snap, fq)
		out = append(out, Fix{Title: "Import '" + fq + "'", Edits: []FixEdit{{Loc{f.snap, e.Range}, e.Text}}})
	}
	slices.SortFunc(out, func(a, b Fix) int { return strings.Compare(a.Title, b.Title) })
	if len(out) == 1 {
		out[0].Preferred = true
	}
	return out
}

// safeCall replaces the '.' of the unsafe call with '?.' or '!!.'.
func (f fixer) safeCall() []Fix {
	n := f.snap.NodeAt(f.d.Range.Start).Parent()
	if n.Parent().NumericKind() == psi.KindCALL_EXPRESSION {
		n = n.Parent()
	}
	q := n.Parent()
	if q.NumericKind() != psi.KindDOT_QUALIFIED_EXPRESSION || q.FirstChild() == n {
		return nil
	}
	dot := q.ChildKind(psi.KindDOT)
	if !dot.Valid() {
		return nil
	}
	return []Fix{
		{Title: "Replace with safe (?.) call", Preferred: true, Edits: []FixEdit{f.edit(dot.Range(), "?.")}},
		{Title: "Add non-null asserted (!!.) call", Edits: []FixEdit{f.edit(dot.Range(), "!!.")}},
	}
}

// valToVar changes the reassigned val's declaration to a var: a
// property, a local, or a constructor's property parameter. A function
// with an error has no facts, so the declaration is found by the syntax:
// a name in its scopes, or the member of this or of a variable declared
// with a class type of the module.
func (f fixer) valToVar() []Fix {
	ref := f.snap.NodeAt(f.d.Range.Start).Parent()
	if ref.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		return nil
	}
	name := ref.Name()
	snap, decl := f.snap, psi.NodeRef{}
	if q := ref.Parent(); q.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION && q.FirstChild() != ref {
		switch recv := q.FirstChild(); recv.NumericKind() {
		case psi.KindTHIS_EXPRESSION:
			if recv.ChildKind(psi.KindLABEL_QUALIFIER).Valid() {
				return nil
			}
			for c := q.Parent(); c.Valid(); c = c.Parent() {
				if k := c.NumericKind(); k == psi.KindCLASS || k == psi.KindOBJECT_DECLARATION {
					decl = member(c, name)
					break
				} else if k == psi.KindFUNCTION_LITERAL || k == psi.KindFUN && c.ReceiverType().Valid() {
					return nil
				}
			}
		case psi.KindREFERENCE_EXPRESSION:
			v := lookup(recv, recv.Name())
			ut := v.TypeReference().ChildKind(psi.KindUSER_TYPE)
			if !ut.Valid() || ut.ChildKind(psi.KindUSER_TYPE).Valid() || ut.ChildKind(psi.KindTYPE_ARGUMENT_LIST).Valid() {
				return nil
			}
			var cls psi.NodeRef
			snap, cls = f.class(ut.ChildKind(psi.KindREFERENCE_EXPRESSION).Name())
			decl = member(cls, name)
		}
	} else {
		decl = lookup(ref, name)
	}
	kw := decl.ChildKind(psi.KindVAL_KEYWORD)
	if !kw.Valid() || decl.NumericKind() == psi.KindVALUE_PARAMETER && decl.Parent().Parent().NumericKind() != psi.KindPRIMARY_CONSTRUCTOR {
		return nil
	}
	return f.fix("Change '"+name+"' to var", true, FixEdit{Loc{snap, kw.Range()}, "var"})
}

// lookup returns the declaration that name, used at from, names by the
// scopes around from: a local before it, a parameter, a member of an
// enclosing class, or a top-level property of the file; or nothing.
func lookup(from psi.NodeRef, name string) psi.NodeRef {
	for c := from.Parent(); c.Valid(); c = c.Parent() {
		switch c.NumericKind() {
		case psi.KindBLOCK:
			var found psi.NodeRef
			for k := c.FirstChild(); k.Valid() && k.Range().End <= from.Range().Start; k = k.NextSibling() {
				if k.NumericKind() == psi.KindPROPERTY && k.Name() == name {
					found = k
				}
			}
			if found.Valid() {
				return found
			}
		case psi.KindFUN, psi.KindSECONDARY_CONSTRUCTOR, psi.KindFUNCTION_LITERAL, psi.KindFOR, psi.KindCATCH, psi.KindPROPERTY_ACCESSOR:
			ps := c.Parameters()
			if p := c.ChildKind(psi.KindVALUE_PARAMETER); p.Valid() {
				ps = append(ps, p) // a for loop's or a catch's
			}
			for _, p := range ps {
				if p.Name() == name && p.Range().End <= from.Range().Start {
					return p
				}
			}
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
			if m := member(c, name); m.Valid() {
				return m
			}
		case psi.KindFILE:
			for k := c.FirstChild(); k.Valid(); k = k.NextSibling() {
				if k.NumericKind() == psi.KindPROPERTY && k.Name() == name {
					return k
				}
			}
		}
	}
	return psi.NodeRef{}
}

// member returns the property of class cls named name: one of its body,
// or a property parameter of its primary constructor.
func member(cls psi.NodeRef, name string) psi.NodeRef {
	for k := cls.ChildKind(psi.KindCLASS_BODY).FirstChild(); k.Valid(); k = k.NextSibling() {
		if k.NumericKind() == psi.KindPROPERTY && k.Name() == name {
			return k
		}
	}
	for _, p := range cls.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).Parameters() {
		if p.Name() == name && (p.ChildKind(psi.KindVAL_KEYWORD).Valid() || p.ChildKind(psi.KindVAR_KEYWORD).Valid()) {
			return p
		}
	}
	return psi.NodeRef{}
}

// class returns the top-level class named name of the document's file,
// or else of the view's other files, with its snapshot.
func (f fixer) class(name string) (*psi.Snapshot, psi.NodeRef) {
	snaps := []*psi.Snapshot{f.snap}
	if f.v != nil {
		var others []*psi.Snapshot
		for _, s := range f.v.byPath {
			if s.Path() != f.doc.Path {
				others = append(others, s)
			}
		}
		slices.SortFunc(others, func(a, b *psi.Snapshot) int { return strings.Compare(a.Path(), b.Path()) })
		snaps = append(snaps, others...)
	}
	for _, s := range snaps {
		for k := s.RootNode().FirstChild(); k.Valid(); k = k.NextSibling() {
			if k.NumericKind() == psi.KindCLASS && k.Name() == name {
				return s, k
			}
		}
	}
	return nil, psi.NodeRef{}
}

// stubName returns the name of the member a stub declares.
func stubName(stub string) string {
	s := stub
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = s[:i]
	} else if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if i := strings.LastIndexAny(s, " ."); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// implement adds overrides of stubs, the checker's signatures of the
// members to implement, to the class, object or enum entry the
// diagnostic is on, whose bodies are TODO().
func (f fixer) implement(title string, stubs []string) []Fix {
	if len(stubs) == 0 {
		return nil
	}
	decl := f.declaration(psi.KindENUM_ENTRY, psi.KindCLASS, psi.KindOBJECT_DECLARATION)
	if !decl.Valid() {
		return nil
	}
	sh := f.shortener()
	var members []string
	for _, s := range stubs {
		sig := sh.short(s)
		switch {
		case strings.HasPrefix(sig, "val ") || strings.HasPrefix(sig, "var "):
			m := "override " + sig + "\n    get() = TODO(\"Not yet implemented\")"
			if strings.HasPrefix(sig, "var ") {
				m += "\n    set(value) {}"
			}
			members = append(members, m)
		default:
			members = append(members, "override "+sig+" {\n    TODO(\"Not yet implemented\")\n}")
		}
	}
	indent := lineIndent(f.snap.Source(), decl.Range().Start)
	var edits []FixEdit
	if body := decl.ChildKind(psi.KindCLASS_BODY); body.Valid() {
		e, ok := f.insertInto(body, members, indent, "\n")
		if !ok {
			return nil
		}
		edits = append(edits, e)
	} else {
		end := lastSignificant(decl, psi.Kind(0))
		if !end.Valid() {
			return nil
		}
		at := end.Range().End
		if decl.NumericKind() == psi.KindENUM_ENTRY {
			// Before the entry's comma or semicolon.
			for c := decl.FirstChild(); c.Valid(); c = c.NextSibling() {
				if k := c.NumericKind(); k == psi.KindCOMMA || k == psi.KindSEMICOLON {
					at = lastBefore(decl, c).Range().End
					break
				}
			}
		}
		edits = append(edits, f.edit(psi.Range{Start: at, End: at}, " {\n"+indentLines(members, indent+"    ", "\n")+"\n"+indent+"}"))
	}
	return f.fix(title, true, append(edits, sh.imports()...)...)
}

// insertInto inserts items, lines of text, at the end of a block's
// contents, before its '}', each indented one level from indent and
// separated by sep plus a newline.
func (f fixer) insertInto(block psi.NodeRef, items []string, indent, sep string) (FixEdit, bool) {
	lb, rb := block.ChildKind(psi.KindLBRACE), lastChild(block, psi.KindRBRACE)
	if !lb.Valid() || !rb.Valid() {
		return FixEdit{}, false
	}
	text := indentLines(items, indent+"    ", sep)
	last := lastSignificant(block, psi.KindRBRACE)
	if !last.Valid() || last == lb {
		return f.edit(psi.Range{Start: lb.Range().End, End: rb.Range().Start}, "\n"+text+"\n"+indent), true
	}
	gap := f.snap.Source()[last.Range().End:rb.Range().Start]
	if strings.Contains(gap, "\n") {
		return f.edit(psi.Range{Start: last.Range().End, End: last.Range().End}, sep+"\n"+text), true
	}
	return f.edit(psi.Range{Start: last.Range().End, End: rb.Range().Start}, sep+"\n"+text+"\n"+indent), true
}

// lastSignificant returns n's last child that isn't trivia, before the
// last child of kind stop if stop isn't 0.
func lastSignificant(n psi.NodeRef, stop psi.Kind) psi.NodeRef {
	var last psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if stop != 0 && c.NumericKind() == stop {
			break
		}
		if !trivia(c) {
			last = c
		}
	}
	return last
}

// lastBefore returns n's last non-trivia child before child c.
func lastBefore(n, c psi.NodeRef) psi.NodeRef {
	var last psi.NodeRef
	for k := n.FirstChild(); k.Valid() && k != c; k = k.NextSibling() {
		if !trivia(k) {
			last = k
		}
	}
	return last
}

// lineIndent returns the blanks that start the line of off.
func lineIndent(src string, off int) string {
	start := strings.LastIndexByte(src[:off], '\n') + 1
	end := start
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	return src[start:end]
}

// indentLines prefixes each line of items with indent, joining the items
// with sep and a newline.
func indentLines(items []string, indent, sep string) string {
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString(sep + "\n")
		}
		for j, l := range strings.Split(it, "\n") {
			if j > 0 {
				b.WriteString("\n")
			}
			b.WriteString(indent + l)
		}
	}
	return b.String()
}

// whenBranches adds the cases the when misses, and an else branch.
func (f fixer) whenBranches() []Fix {
	w := f.enclosing(psi.KindWHEN)
	if !w.Valid() {
		return nil
	}
	indent := lineIndent(f.snap.Source(), w.Range().Start)
	if e := w.ChildKind(psi.KindWHEN_ENTRY); e.Valid() && strings.HasSuffix(f.snap.Source()[:e.Range().Start], "\n"+lineIndent(f.snap.Source(), e.Range().Start)) {
		ei := lineIndent(f.snap.Source(), e.Range().Start)
		if len(ei) >= 4 {
			indent = ei[:len(ei)-4]
		}
	}
	var out []Fix
	if len(f.d.Data) > 0 {
		sh := f.shortener()
		var cases []string
		for _, c := range f.d.Data {
			cases = append(cases, sh.short(c)+" -> TODO()")
		}
		if e, ok := f.insertInto(w, cases, indent, ""); ok {
			out = append(out, f.fix("Add remaining branches", true, append([]FixEdit{e}, sh.imports()...)...)...)
		}
	}
	if e, ok := f.insertInto(w, []string{"else -> TODO()"}, indent, ""); ok {
		out = append(out, f.fix("Add else branch", false, e)...)
	}
	return out
}

// mismatched returns the expression of a type mismatch. The
// diagnostic is on the token the checker positions the expression at: its
// first, or the operator, the '(' of a call, the '[' of an index; so the
// expression is the outermost of the expressions around that token.
func (f fixer) mismatched() psi.NodeRef {
	n := f.snap.NodeAt(f.d.Range.Start)
	for n.Valid() && !expressionKinds[n.NumericKind()] {
		switch n.NumericKind() {
		case psi.KindOPERATION_REFERENCE, psi.KindVALUE_ARGUMENT_LIST, psi.KindINDICES, psi.KindDOT, psi.KindSAFE_ACCESS:
		default:
			if !n.IsLeaf() {
				return psi.NodeRef{}
			}
		}
		n = n.Parent()
	}
	for n.Valid() && expressionKinds[n.Parent().NumericKind()] {
		n = n.Parent()
	}
	return n
}

// expressionKinds are kinds of expressions that may hold a mismatch.
var expressionKinds = map[psi.Kind]bool{
	psi.KindREFERENCE_EXPRESSION: true, psi.KindCALL_EXPRESSION: true, psi.KindDOT_QUALIFIED_EXPRESSION: true,
	psi.KindSAFE_ACCESS_EXPRESSION: true, psi.KindBINARY_EXPRESSION: true, psi.KindBINARY_WITH_TYPE: true,
	psi.KindPREFIX_EXPRESSION: true, psi.KindPOSTFIX_EXPRESSION: true, psi.KindARRAY_ACCESS_EXPRESSION: true,
	psi.KindPARENTHESIZED: true, psi.KindSTRING_TEMPLATE: true, psi.KindINTEGER_CONSTANT: true,
	psi.KindFLOAT_CONSTANT: true, psi.KindBOOLEAN_CONSTANT: true, psi.KindCHARACTER_CONSTANT: true,
	psi.KindNULL: true, psi.KindIF: true, psi.KindWHEN: true, psi.KindTRY: true, psi.KindLAMBDA_EXPRESSION: true,
	psi.KindOBJECT_LITERAL: true, psi.KindTHIS_EXPRESSION: true, psi.KindCALLABLE_REFERENCE_EXPRESSION: true,
	psi.KindIS_EXPRESSION: true, psi.KindCLASS_LITERAL_EXPRESSION: true,
}

// changeType changes the declared type of the property, or the return
// type of the function, whose initializer, body or returned value is
// the mismatched expression, to the expression's type.
func (f fixer) changeType() []Fix {
	x := f.mismatched()
	if !x.Valid() {
		return nil
	}
	var decl psi.NodeRef
	switch p := x.Parent(); p.NumericKind() {
	case psi.KindPROPERTY, psi.KindFUN:
		if p.Body() == x {
			decl = p
		}
	case psi.KindRETURN:
		if p.ChildKind(psi.KindLABEL_QUALIFIER).Valid() {
			return nil
		}
		for d := p.Parent(); d.Valid(); d = d.Parent() {
			if k := d.NumericKind(); k == psi.KindFUN {
				decl = d
				break
			} else if k == psi.KindLAMBDA_EXPRESSION || k == psi.KindPROPERTY_ACCESSOR || k == psi.KindCLASS_BODY {
				return nil
			}
		}
	}
	tr := decl.TypeReference()
	if !tr.Valid() {
		return nil
	}
	sh := f.shortener()
	t := sh.short(f.d.Data[0])
	if !validType(t) {
		return nil
	}
	what := "type of '" + decl.Name() + "'"
	if decl.NumericKind() == psi.KindFUN {
		what = "return type of '" + decl.Name() + "'"
	}
	return f.fix("Change "+what+" to '"+t+"'", false, append([]FixEdit{f.edit(tr.Range(), t)}, sh.imports()...)...)
}

// validType reports whether t parses as a type.
func validType(t string) bool {
	if strings.ContainsAny(t, "&!{}") {
		return false
	}
	snap := psi.Parse("t.kt", "val x: "+t+"\n")
	if len(snap.Diagnostics()) > 0 {
		return false
	}
	p := snap.RootNode().ChildKind(psi.KindPROPERTY)
	return p.TypeReference().Text() == t
}

// notNull adds !! to the mismatched expression when it is the expected
// type made nullable.
func (f fixer) notNull() []Fix {
	actual, expected := f.d.Data[0], f.d.Data[1]
	if actual != expected+"?" {
		return nil
	}
	x := f.mismatched()
	if !x.Valid() {
		return nil
	}
	switch x.NumericKind() {
	case psi.KindBINARY_EXPRESSION, psi.KindBINARY_WITH_TYPE, psi.KindIS_EXPRESSION, psi.KindPREFIX_EXPRESSION,
		psi.KindIF, psi.KindWHEN, psi.KindTRY, psi.KindLAMBDA_EXPRESSION:
		return f.fix("Add non-null asserted (!!) call", false,
			f.edit(psi.Range{Start: x.Range().Start, End: x.Range().Start}, "("),
			f.edit(psi.Range{Start: x.Range().End, End: x.Range().End}, ")!!"))
	}
	return f.fix("Add non-null asserted (!!) call", false, f.edit(psi.Range{Start: x.Range().End, End: x.Range().End}, "!!"))
}

// removeOverride removes the override modifier the diagnostic's
// declaration has.
func (f fixer) removeOverride() []Fix {
	d := f.declaration(psi.KindFUN, psi.KindPROPERTY)
	ov := d.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindOVERRIDE_KEYWORD)
	if !ov.Valid() {
		return nil
	}
	r := ov.Range()
	src := f.snap.Source()
	for r.End < len(src) && (src[r.End] == ' ' || src[r.End] == '\t') {
		r.End++
	}
	return f.fix("Remove 'override' modifier", true, f.edit(r, ""))
}

// initialize adds an initializer to the property, its type's zero value
// or TODO(), or makes a var of a class type lateinit.
func (f fixer) initialize() []Fix {
	p := f.declaration(psi.KindPROPERTY)
	tr := p.TypeReference()
	if !tr.Valid() || p.Body().Valid() {
		return nil
	}
	t := tr.Text()
	init := "TODO()"
	switch {
	case strings.HasSuffix(t, "?"):
		init = "null"
	case t == "Int" || t == "Long" || t == "Short" || t == "Byte" || t == "kotlin.Int":
		init = "0"
	case t == "Double":
		init = "0.0"
	case t == "Float":
		init = "0f"
	case t == "Boolean":
		init = "false"
	case t == "String":
		init = `""`
	case t == "Char":
		init = "' '"
	}
	end := tr.Range().End
	out := f.fix("Add initializer", true, f.edit(psi.Range{Start: end, End: end}, " = "+init))
	if kw := p.ChildKind(psi.KindVAR_KEYWORD); kw.Valid() && !strings.HasSuffix(t, "?") && !primitive[t] {
		out = append(out, f.fix("Add 'lateinit' modifier", false, f.edit(psi.Range{Start: kw.Range().Start, End: kw.Range().Start}, "lateinit "))...)
	}
	return out
}

// primitive are the types lateinit doesn't take.
var primitive = map[string]bool{"Int": true, "Long": true, "Short": true, "Byte": true, "Double": true, "Float": true, "Boolean": true, "Char": true}

// overrideReturnType changes the return type of the override to the
// overridden member's.
func (f fixer) overrideReturnType() []Fix {
	d := f.declaration(psi.KindFUN, psi.KindPROPERTY)
	tr := d.TypeReference()
	if !tr.Valid() {
		return nil
	}
	sh := f.shortener()
	t := sh.short(f.d.Data[0])
	if !validType(t) {
		return nil
	}
	return f.fix("Change return type to '"+t+"'", true, append([]FixEdit{f.edit(tr.Range(), t)}, sh.imports()...)...)
}

// defaultPackages are the packages Kotlin/JVM files import by default.
var defaultPackages = map[string]bool{
	"kotlin": true, "kotlin.annotation": true, "kotlin.collections": true, "kotlin.comparisons": true,
	"kotlin.io": true, "kotlin.ranges": true, "kotlin.sequences": true, "kotlin.text": true,
	"kotlin.jvm": true, "java.lang": true,
}

// kotlinShadowed are classes of java.lang whose simple names name
// another class of kotlin's default imports.
var kotlinShadowed = map[string]bool{"Deprecated": true}

// A shortener shortens the qualified names of a type, or of a stub, as
// the checker writes them, to the names the file sees, collecting the
// imports that need adding.
type shortener struct {
	snap    *psi.Snapshot
	pkg     string
	visible map[string]string // simple name -> qualified name imported
	stars   map[string]bool
	added   []string
}

func (f fixer) shortener() *shortener {
	sh := &shortener{snap: f.snap, pkg: f.snap.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path(), visible: map[string]string{}, stars: map[string]bool{}}
	for _, i := range imports(f.snap) {
		switch {
		case i.star:
			sh.stars[i.path] = true
		case i.alias == "":
			sh.visible[i.local()] = i.path
		}
	}
	return sh
}

// short rewrites every qualified name in s: the package, the segments
// before the first capitalized one, is dropped if the class is visible
// or can be imported; the class is imported if need be.
func (sh *shortener) short(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if !isIdentStart(c) {
			b.WriteByte(c)
			i++
			continue
		}
		j := i
		for j < len(s) && (isIdentPart(s[j]) || s[j] == '.' && j+1 < len(s) && isIdentStart(s[j+1])) {
			j++
		}
		b.WriteString(sh.name(s[i:j]))
		i = j
	}
	return b.String()
}

func isIdentStart(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
func isIdentPart(c byte) bool  { return isIdentStart(c) || '0' <= c && c <= '9' }

// name shortens one dotted name.
func (sh *shortener) name(q string) string {
	segs := strings.Split(q, ".")
	k := 0
	for k < len(segs) && segs[k] != "" && segs[k][0] >= 'a' && segs[k][0] <= 'z' {
		k++
	}
	if k == 0 || k == len(segs) {
		return q // a simple or unqualified name, or no class
	}
	pkg, cls := strings.Join(segs[:k], "."), segs[k]
	fq := pkg + "." + cls
	short := strings.Join(segs[k:], ".")
	if pkg == "java.lang" && kotlinShadowed[cls] {
		return q // the default import of kotlin's comes first
	}
	switch imported, ok := sh.visible[cls]; {
	case ok && imported == fq, !ok && (pkg == sh.pkg || defaultPackages[pkg] || sh.stars[pkg]):
		return short
	case ok:
		return q // another class of the name is imported
	}
	sh.visible[cls] = fq
	sh.added = append(sh.added, fq)
	return short
}

// imports returns the edits that import the names short added.
func (sh *shortener) imports() []FixEdit {
	var out []FixEdit
	for _, fq := range sh.added {
		e := ImportEdit(sh.snap, fq)
		out = append(out, FixEdit{Loc{sh.snap, e.Range}, e.Text})
	}
	return out
}
