package ide

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Completion (docs/lsp.md, milestone L4). CompletionAt reads from the
// document alone what the caret is at: the name typed, a comment or a
// string, an import, and the keywords that may go there. It also gives
// the document with analysis.ProbeName in place of the name at the caret,
// whose check (analysis.Session.Complete) says what the code may name
// there; Complete ranks that and the keywords into items.

// CompletionContext is what an editor's caret is at, for completion.
type CompletionContext struct {
	Path string
	// Prefix is the name typed before the caret, and Replace the whole
	// name at the caret, which an item replaces.
	Prefix  string
	Replace psi.Range
	// None is set in a comment, a string or a number, or on the name of
	// a declaration: nothing is completed there.
	None bool
	// Import is set in an import or package directive (Package): the
	// caret's name follows Qualifier, the dotted name before it.
	Import    bool
	Package   bool
	Qualifier string
	// Keywords are the keywords that may go at the caret.
	Keywords []string
	// Call is set when ( or < follows the name: an item inserts no
	// parentheses.
	Call bool
	// ArgName is set at the name of a named argument: only parameters'
	// names fit.
	ArgName bool
	// ProbeText is the document with analysis.ProbeName in place of the
	// name at the caret, and a ) for each ( the line leaves open.
	ProbeText string

	snap  *psi.Snapshot // the document's
	probe psi.Range     // the probe's, and the )s added, in ProbeText
}

// The keywords that may start an expression, a statement, a member of a
// class, and a top-level declaration.
var (
	expressionKeywords = []string{"false", "if", "null", "object", "return", "super", "this", "throw", "true", "try", "when"}
	statementKeywords  = append([]string{"break", "class", "continue", "do", "for", "fun", "interface", "val", "var", "while"}, expressionKeywords...)
	memberKeywords     = []string{"abstract", "annotation", "class", "companion", "const", "constructor", "data", "enum", "fun",
		"infix", "init", "inline", "inner", "interface", "internal", "lateinit", "object", "open", "operator", "override",
		"private", "protected", "public", "sealed", "suspend", "val", "value", "var"}
	topKeywords = []string{"abstract", "annotation", "class", "const", "data", "enum", "fun", "infix", "inline", "interface",
		"internal", "object", "open", "operator", "private", "public", "sealed", "suspend", "typealias", "val", "value", "var"}
)

func isNameRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// CompletionAt returns what the caret at offset off of d is at.
func CompletionAt(d *Document, off int) CompletionContext {
	text := d.Text()
	off = max(0, min(off, len(text)))
	start, end := off, off
	for start > 0 {
		r, n := utf8.DecodeLastRuneInString(text[:start])
		if !isNameRune(r) {
			break
		}
		start -= n
	}
	for end < len(text) {
		r, n := utf8.DecodeRuneInString(text[end:])
		if !isNameRune(r) {
			break
		}
		end += n
	}
	cc := CompletionContext{Path: d.Path, Prefix: text[start:off], Replace: psi.Range{Start: start, End: end}, snap: d.Snap}
	if r, _ := utf8.DecodeRuneInString(cc.Prefix); cc.Prefix != "" && unicode.IsDigit(r) || inCommentOrString(d.Snap, start) {
		cc.None = true
		return cc
	}
	lineStart := strings.LastIndexByte(text[:start], '\n') + 1
	line := strings.TrimLeft(text[lineStart:start], " \t")
	for _, kw := range []string{"import", "package"} {
		if rest, ok := strings.CutPrefix(line, kw); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
			cc.Import, cc.Package = true, kw == "package"
			cc.Qualifier = strings.TrimSuffix(strings.Join(strings.Fields(rest), ""), ".")
			return cc
		}
	}
	rest := text[end:]
	lineEnd := strings.IndexByte(rest, '\n')
	if lineEnd < 0 {
		lineEnd = len(rest)
	}
	after := strings.TrimLeft(rest[:lineEnd], " \t")
	cc.Call = strings.HasPrefix(after, "(") || strings.HasPrefix(rest, "<")
	if before := strings.TrimRight(text[:start], " \t\r\n"); (strings.HasSuffix(before, "(") || strings.HasSuffix(before, ",")) &&
		strings.HasPrefix(after, "=") && !strings.HasPrefix(after, "==") {
		// name| = value: the probe as a positional argument, with the
		// value after it, gives the call's candidates.
		cc.ArgName = true
		eq := end + strings.IndexByte(rest, '=')
		text = text[:eq] + "," + text[eq+1:]
		rest = text[end:]
	}
	closers := strings.Repeat(")", max(0, strings.Count(text[lineStart:start], "(")-strings.Count(text[lineStart:start], ")")-strings.Count(rest[:lineEnd], ")")))
	cc.ProbeText = text[:start] + analysis.ProbeName + closers + text[end:]
	cc.probe = psi.Range{Start: start, End: start + len(analysis.ProbeName) + len(closers)}
	cc.Keywords, cc.None = keywordsAt(parse(d.Path, cc.ProbeText), start)
	if cc.ArgName {
		cc.Keywords = nil
	}
	return cc
}

// inCommentOrString reports whether a name starting at off would be in a
// comment, a string's text or a character literal.
func inCommentOrString(snap *psi.Snapshot, off int) bool {
	if off == 0 {
		return false
	}
	leaf := snap.NodeAt(off - 1)
	switch leaf.NumericKind() {
	case psi.KindEOL_COMMENT, psi.KindSHEBANG_COMMENT, psi.KindREGULAR_STRING_PART, psi.KindOPEN_QUOTE, psi.KindESCAPE_SEQUENCE, psi.KindCHARACTER_LITERAL:
		return true
	case psi.KindBLOCK_COMMENT:
		return leaf.Range().End > off || !strings.HasSuffix(leaf.Text(), "*/")
	}
	for n := leaf; n.Valid(); n = n.Parent() {
		if n.NumericKind() == psi.KindKDOC {
			return n.Range().End > off || !strings.HasSuffix(n.Text(), "*/")
		}
	}
	return false
}

// keywordsAt returns the keywords that may go at the probe at off of
// snap, or none if the probe names a new declaration.
func keywordsAt(snap *psi.Snapshot, off int) (kws []string, none bool) {
	id := snap.NodeAt(off)
	if id.NumericKind() != psi.KindIDENTIFIER {
		return nil, false
	}
	p := id.Parent()
	switch p.NumericKind() {
	case psi.KindPROPERTY, psi.KindFUN, psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindVALUE_PARAMETER,
		psi.KindTYPE_PARAMETER, psi.KindTYPEALIAS, psi.KindDESTRUCTURING_DECLARATION_ENTRY, psi.KindENUM_ENTRY:
		return nil, true
	case psi.KindREFERENCE_EXPRESSION:
		e := p
		pp := e.Parent()
		switch pp.NumericKind() {
		case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
			if firstNonTrivia(pp) != e {
				return nil, false
			}
		case psi.KindCALLABLE_REFERENCE_EXPRESSION, psi.KindUSER_TYPE, psi.KindSHORT_STRING_TEMPLATE_ENTRY:
			return nil, false
		case psi.KindCALL_EXPRESSION:
			e, pp = pp, pp.Parent()
		}
		if pp.NumericKind() == psi.KindBLOCK || pp.NumericKind() == psi.KindFILE {
			return statementKeywords, false
		}
		return expressionKeywords, false
	case psi.KindUSER_TYPE:
		return nil, false
	}
	for n := p; n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindBLOCK:
			return statementKeywords, false
		case psi.KindCLASS_BODY:
			return memberKeywords, false
		case psi.KindVALUE_ARGUMENT_LIST, psi.KindVALUE_PARAMETER_LIST, psi.KindTYPE_REFERENCE:
			return nil, false
		}
	}
	return topKeywords, false
}

// CompletionOptions are an editor's completion settings.
type CompletionOptions struct {
	// Snippets is set when the editor takes snippets: a function's item
	// puts the caret between its parentheses.
	Snippets bool
	// Limit caps the items; 0 for 200.
	Limit int
}

// CompletionItem is one item an editor lists.
type CompletionItem struct {
	Label string
	// Kind is an analysis.CompletionItem's Kind, "method" for a member
	// function, "keyword" or "namedArgument".
	Kind   string
	Detail string
	// Insert replaces CompletionContext.Replace; Snippet is set when it
	// is a snippet. Filter is the text an editor filters by, if not
	// Label, and Sort the text it sorts by.
	Insert  string
	Snippet bool
	Filter  string
	Sort    string
	// Import is set on a name that needs an import: the edit that adds
	// it.
	Import *TextEdit
	// Deprecated is set on a deprecated declaration.
	Deprecated bool
	// Decl is where the declaration starts, in the document Complete's
	// context is of or another source: CompletionDoc documents it.
	Decl *DeclAt
}

// DeclAt is the start of a declaration in the file at Path.
type DeclAt struct {
	Path string
	Off  int
}

// TextEdit replaces Range of a document by Text.
type TextEdit struct {
	Range psi.Range
	Text  string
}

// candidate is an item before ranking.
type candidate struct {
	item     CompletionItem
	group    int  // the scope it comes from, nearest first
	expected bool // its type is the expected one
	score    int  // how well it matches the prefix
}

// Complete returns the items for the caret of cc, from c, what the code
// may name there (nil if it isn't known), best first, and whether there
// were more than opts.Limit.
func Complete(cc CompletionContext, c *analysis.Completion, opts CompletionOptions) (items []CompletionItem, incomplete bool) {
	if cc.None || cc.Import {
		return nil, false
	}
	var cands []candidate
	seen := map[string]bool{}
	// The expected type, if it says something: not Any's nor Unit's.
	var wanted *analysis.Type
	if c != nil {
		wanted = c.Expected
		if wanted == nil && len(c.Calls) > 0 {
			wanted = argumentType(c.Calls[0])
		}
	}
	if wanted != nil && (wanted.Name == "kotlin.Any" || wanted.Name == "kotlin.Unit" || wanted.Parameter) {
		wanted = nil
	}
	add := func(it CompletionItem, group int, typ *analysis.Type) {
		score, ok := completionScore(cmp.Or(it.Filter, it.Label), cc.Prefix)
		if !ok {
			return
		}
		key := it.Label + "\x00" + it.Detail
		if it.Kind == "namedArgument" {
			key = it.Label
		}
		if seen[key] {
			return
		}
		seen[key] = true
		exp := wanted != nil && typ != nil && !classifierKinds[it.Kind] && (typ.Name == wanted.Name && (!typ.Nullable || wanted.Nullable) || slices.Contains(typ.Supertypes, wanted.Name))
		cands = append(cands, candidate{item: it, group: group, expected: exp, score: score})
	}
	typePos := c != nil && c.TypePosition
	member := c != nil && (c.Receiver != nil || c.Static != "" || c.Package != "")
	callable := c != nil && c.Callable
	if c != nil && !member && !typePos && !callable && len(c.Calls) > 0 {
		cs := c.Calls[0]
		if cs.Name == "" {
			for _, cand := range cs.Candidates {
				if !cand.StableNames {
					continue
				}
				for i, name := range cand.ParameterNames {
					if name == "" || slices.Contains(cs.Names, name) {
						continue
					}
					insert := name + " = "
					if cc.ArgName {
						insert = name
					}
					add(CompletionItem{Label: name + " =", Kind: "namedArgument", Detail: renderType(cand.Parameters[i]), Insert: insert, Filter: name}, 0, nil)
				}
			}
		}
	}
	if c != nil && !cc.ArgName {
		for _, ci := range c.Items {
			add(cc.item(ci, c, opts, typePos || callable), group(ci), ci.Type)
		}
		for _, im := range c.Importables {
			fq := qualifiedName(im.Package, im.Name)
			kind := "function"
			if im.Class {
				kind = "class"
			}
			add(CompletionItem{Label: im.Name, Kind: kind, Detail: fq, Insert: im.Name, Import: cc.importEdit(fq)}, 7, nil)
		}
	}
	// A keyword typed in part goes before what extensions and imports
	// bring in; with nothing typed, after them.
	keywordGroup := 6
	if cc.Prefix != "" {
		keywordGroup = 3
	}
	if !member && !typePos && !callable && !cc.ArgName {
		for _, kw := range cc.Keywords {
			var typ *analysis.Type
			switch kw {
			case "true", "false":
				typ = &analysis.Type{Name: "kotlin.Boolean"}
			case "null":
				if wanted != nil && wanted.Nullable {
					typ = wanted
				}
			}
			add(CompletionItem{Label: kw, Kind: "keyword", Insert: kw}, keywordGroup, typ)
		}
	}
	slices.SortStableFunc(cands, func(a, b candidate) int {
		if a.group == 0 || b.group == 0 {
			// Named arguments first, in their parameters' order.
			return cmp.Compare(a.group, b.group)
		}
		return cmp.Or(cmp.Compare(matchTier(a.score), matchTier(b.score)), cmp.Compare(btoi(!a.expected), btoi(!b.expected)), cmp.Compare(a.group, b.group), cmp.Compare(b.score, a.score),
			cmp.Compare(btoi(a.item.Deprecated), btoi(b.item.Deprecated)), cmp.Compare(btoi(a.item.Decl == nil), btoi(b.item.Decl == nil)),
			cmp.Compare(btoi(cc.Prefix != "")*len(a.item.Label), btoi(cc.Prefix != "")*len(b.item.Label)),
			strings.Compare(strings.ToLower(a.item.Label), strings.ToLower(b.item.Label)), strings.Compare(a.item.Label, b.item.Label),
			strings.Compare(a.item.Detail, b.item.Detail))
	})
	limit := cmp.Or(opts.Limit, 200)
	if len(cands) > limit {
		cands, incomplete = cands[:limit], true
	}
	for i, cd := range cands {
		it := cd.item
		it.Sort = sortKey(i)
		items = append(items, it)
	}
	return items, incomplete
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// classifierKinds are the kinds of items that name a type, not a value
// of it.
var classifierKinds = map[string]bool{"class": true, "interface": true, "enum": true, "annotation": true, "typealias": true, "typeParameter": true}

// argumentType returns the type of the parameter the probe is an
// argument for, if every candidate of cs has the same one.
func argumentType(cs analysis.CompletionCall) *analysis.Type {
	var out *analysis.Type
	for _, cand := range cs.Candidates {
		i := cs.Argument
		if cs.Name != "" {
			i = slices.Index(cand.ParameterNames, cs.Name)
		} else if n := len(cand.Parameters); i >= n && n > 0 && n <= len(cand.Varargs) && cand.Varargs[n-1] {
			i = n - 1
		}
		if i < 0 || i >= len(cand.Parameters) {
			return nil
		}
		t := &cand.Parameters[i]
		if out != nil && (out.Name != t.Name || out.Nullable != t.Nullable) {
			return nil
		}
		out = t
	}
	return out
}

// completionScore is matchScore, but a prefix as typed beats one in
// another case: of "MAX", MAX_VALUE beats max.
func completionScore(name, prefix string) (int, bool) {
	switch {
	case prefix == "":
		return 0, true
	case name == prefix:
		return 6, true
	case strings.HasPrefix(name, prefix):
		return 5, true
	}
	return matchScore(name, prefix)
}

// matchTier ranks a completionScore: prefixes, then humps, then other
// subsequences, before the scope an item comes from.
func matchTier(score int) int {
	switch {
	case score == 0 || score >= 3:
		return 0
	case score == 2:
		return 1
	}
	return 2
}

// sortKey is the sort text of the i'th item: fixed-width, so editors keep
// the order.
func sortKey(i int) string {
	const digits = "0123456789"
	b := []byte("0000")
	for j := 3; j >= 0 && i > 0; j-- {
		b[j] = digits[i%10]
		i /= 10
	}
	return string(b)
}

func qualifiedName(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// group ranks where ci comes from: locals, then the innermost receiver's
// members, other receivers', extensions, and the top-level names.
func group(ci analysis.CompletionItem) int {
	switch ci.Origin {
	case "local", "typeParameter":
		return 1
	case "member":
		if ci.Receiver > 0 {
			return 3
		}
		return 2
	case "extension":
		return 4
	}
	return 5
}

// item converts ci; bare is set where a function or class is named
// without a call: in a type or a callable reference.
func (cc CompletionContext) item(ci analysis.CompletionItem, c *analysis.Completion, opts CompletionOptions, bare bool) CompletionItem {
	it := CompletionItem{Label: ci.Name, Kind: ci.Kind, Insert: ci.Name, Deprecated: ci.Deprecated}
	if ci.Kind == "function" && (ci.Origin == "member" || ci.Origin == "static") {
		it.Kind = "method"
	}
	switch {
	case ci.Kind == "function" && ci.Call != nil:
		it.Detail = renderCall(*ci.Call)
	case ci.Kind == "property" && ci.Call != nil && ci.Call.DeclaredReturnType != nil:
		it.Detail = renderType(*ci.Call.DeclaredReturnType)
	case ci.Type != nil && (ci.Kind == "local" || ci.Kind == "parameter" || ci.Kind == "property"):
		it.Detail = renderType(*ci.Type)
	case ci.QualifiedName != "":
		it.Detail = ci.QualifiedName
	}
	if ci.Type != nil && (ci.Kind == "property") && it.Detail == "" {
		it.Detail = renderType(*ci.Type)
	}
	call := ci.Kind == "function" || ci.Call != nil && ci.Kind == "class" && !bare
	if call && ci.Call != nil && !bare && !cc.Call {
		params := ci.Call.Parameters
		switch {
		case !opts.Snippets && len(params) == 0:
			it.Insert += "()"
		case !opts.Snippets:
			it.Insert += "("
		case len(params) == 0:
			it.Insert += "()"
			it.Snippet = true
		case len(params) == 1 && isFunctionType(params[0]):
			it.Insert += " { $0 }"
			it.Snippet = true
		default:
			it.Insert += "($0)"
			it.Snippet = true
		}
	}
	if ci.Declaration != nil {
		if n := c.Node(*ci.Declaration); n.Valid() {
			path, off := n.ContainingFile().Path(), n.Range().Start
			if path == cc.Path && off >= cc.probe.End {
				off -= (cc.probe.End - cc.probe.Start) - (cc.Replace.End - cc.Replace.Start)
			}
			it.Decl = &DeclAt{Path: path, Off: off}
		}
	}
	return it
}

// isFunctionType reports whether t is a function type, which a lambda
// passes.
func isFunctionType(t analysis.Type) bool {
	_, ok := functionArity(t.Name)
	return ok
}

// importEdit returns the edit that imports fq into cc's document: after
// its last import, or its package directive.
func (cc CompletionContext) importEdit(fq string) *TextEdit {
	e := ImportEdit(cc.snap, fq)
	return &e
}

// CompletionDoc documents the declaration at offset off of snap, an
// item's Decl: its header and KDoc, as Markdown; "" if there is none.
func CompletionDoc(snap *psi.Snapshot, off int) string {
	if snap == nil || off < 0 || off >= len(snap.Source()) {
		return ""
	}
	var d psi.NodeRef
	for n := snap.NodeAt(off); n.Valid() && n.Range().Start == off; n = n.Parent() {
		if n.Declaration().Valid() {
			d = n
		}
	}
	if !d.Valid() {
		return ""
	}
	parts := []string{"```kotlin\n" + header(d, nil) + "\n```"}
	if doc := KDoc(d); doc != "" {
		parts = append(parts, doc)
	}
	return strings.Join(parts, "\n\n")
}

// CompleteImport returns the items of an import or package directive at
// cc, from members, what may follow its qualifier
// (analysis.Session.PackageMembers): a package directive only names
// packages.
func CompleteImport(cc CompletionContext, members []analysis.CompletionItem) []CompletionItem {
	var cands []candidate
	for _, m := range members {
		if cc.Package && m.Kind != "package" {
			continue
		}
		score, ok := matchScore(m.Name, cc.Prefix)
		if !ok {
			continue
		}
		g := 1
		if m.Kind == "package" {
			g = 0
		}
		cands = append(cands, candidate{item: CompletionItem{Label: m.Name, Kind: m.Kind, Detail: m.QualifiedName, Insert: m.Name}, group: g, score: score})
	}
	slices.SortFunc(cands, func(a, b candidate) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.group, b.group), strings.Compare(a.item.Label, b.item.Label))
	})
	var items []CompletionItem
	for i, cd := range cands {
		if i == 200 {
			break
		}
		cd.item.Sort = sortKey(i)
		items = append(items, cd.item)
	}
	return items
}
