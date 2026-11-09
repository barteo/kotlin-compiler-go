package ide

import (
	"hash/fnv"
	"sort"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// FormatOptions are a formatting request's options.
type FormatOptions struct {
	TabSize int  // the spaces of an indent; 4 if 0
	UseTabs bool // indent with tabs, one per level
}

func (o FormatOptions) unit() string {
	if o.UseTabs {
		return "\t"
	}
	if o.TabSize <= 0 {
		return "    "
	}
	return strings.Repeat(" ", o.TabSize)
}

// Format returns the edits that format snap: whitespace only, close to
// IntelliJ's Kotlin official style (docs/lsp.md). Each edit replaces one
// run of whitespace between two tokens, which stay as they are, as do
// comments, string templates and KDoc. Lines are never joined or split:
// a line break stays one, with its indent recomputed and blank lines
// limited, and a space between tokens on one line follows the rules of
// space. A file with syntax errors, or whose formatting would change its
// tokens, gets no edits.
func Format(snap *psi.Snapshot, opts FormatOptions) []TextEdit {
	edits, err := format(snap, opts)
	if err != "" {
		return nil
	}
	return edits
}

// FormatRange returns the edits of Format that touch r.
func FormatRange(snap *psi.Snapshot, opts FormatOptions, r psi.Range) []TextEdit {
	var out []TextEdit
	for _, e := range Format(snap, opts) {
		if e.Range.End >= r.Start && e.Range.Start <= r.End {
			out = append(out, e)
		}
	}
	return out
}

// format returns Format's edits, or why there are none.
func format(snap *psi.Snapshot, opts FormatOptions) ([]TextEdit, string) {
	if len(snap.Diagnostics()) > 0 {
		return nil, "syntax errors"
	}
	f := newFormatter(snap, opts)
	edits := f.edits()
	if len(edits) == 0 {
		return nil, ""
	}
	out := applyEdits(snap.Source(), edits)
	if tokenHash(psi.Parse(snap.Path(), out)) != tokenHash(snap) {
		return nil, "formatting changes the tokens"
	}
	return edits, ""
}

// applyEdits applies sorted, disjoint edits to src.
func applyEdits(src string, edits []TextEdit) string {
	var b strings.Builder
	at := 0
	for _, e := range edits {
		b.WriteString(src[at:e.Range.Start])
		b.WriteString(e.Text)
		at = e.Range.End
	}
	b.WriteString(src[at:])
	return b.String()
}

// tokenHash hashes snap's tree without its whitespace: the kinds of its
// nodes in preorder and the text of its leaves.
func tokenHash(snap *psi.Snapshot) uint64 {
	h := fnv.New64a()
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		if n.NumericKind() == psi.KindWHITE_SPACE {
			return false
		}
		h.Write([]byte(n.Kind()))
		if n.IsLeaf() {
			h.Write([]byte{0})
			h.Write([]byte(n.Text()))
		}
		h.Write([]byte{1})
		return true
	})
	return h.Sum64()
}

// A token is a leaf other than whitespace, or a node formatted as one: a
// string template or KDoc.
type token struct {
	node  psi.NodeRef
	kind  psi.Kind
	start int
	end   int
}

type formatter struct {
	src    string
	unit   string
	toks   []token
	lines  []int          // offsets of the lines' starts
	indent map[int]string // a line's new indent, by line
}

func newFormatter(snap *psi.Snapshot, opts FormatOptions) *formatter {
	f := &formatter{src: snap.Source(), unit: opts.unit(), indent: map[int]string{}}
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		k := n.NumericKind()
		switch {
		case k == psi.KindWHITE_SPACE:
			return false
		case k == psi.KindSTRING_TEMPLATE || k == psi.KindKDOC || n.IsLeaf():
			if r := n.Range(); r.End > r.Start {
				f.toks = append(f.toks, token{n, k, r.Start, r.End})
			}
			return false
		}
		return true
	})
	f.lines = lineStarts(f.src)
	return f
}

// line returns the index of the line of off.
func (f *formatter) line(off int) int {
	return sort.Search(len(f.lines), func(i int) bool { return f.lines[i] > off }) - 1
}

// lineIndent returns the indent of a line: its new one if formatting set
// it, else the blanks it starts with.
func (f *formatter) lineIndent(l int) string {
	if s, ok := f.indent[l]; ok {
		return s
	}
	s := f.src[f.lines[l]:]
	return s[:len(s)-len(strings.TrimLeft(s, " \t"))]
}

func (f *formatter) edits() []TextEdit {
	var out []TextEdit
	add := func(start, end int, text string) {
		if f.src[start:end] != text {
			out = append(out, TextEdit{psi.Range{Start: start, End: end}, text})
		}
	}
	if len(f.toks) == 0 {
		return nil
	}
	add(0, f.toks[0].start, "")
	f.indent[f.line(f.toks[0].start)] = ""
	for i := 1; i < len(f.toks); i++ {
		l, r := f.toks[i-1], f.toks[i]
		gap := f.src[l.end:r.start]
		if strings.TrimSpace(gap) != "" {
			continue // not whitespace: a token formatting doesn't know
		}
		n := strings.Count(gap, "\n")
		if n == 0 {
			add(l.end, r.start, f.space(l, r, gap))
			continue
		}
		n = min(n, f.maxBreaks(l, r))
		ind := f.indentOf(r)
		f.indent[f.line(r.start)] = ind
		add(l.end, r.start, strings.Repeat("\n", n)+ind)
	}
	last := f.toks[len(f.toks)-1]
	add(last.end, len(f.src), "\n")
	return out
}

// maxBreaks returns the most line breaks before r: blank lines are
// limited to two between declarations and one in code, and dropped
// after a block's '{' and before a '}'.
func (f *formatter) maxBreaks(l, r token) int {
	if r.kind == psi.KindRBRACE {
		return 1
	}
	if l.kind == psi.KindLBRACE {
		switch l.node.Parent().NumericKind() {
		case psi.KindBLOCK, psi.KindFUNCTION_LITERAL, psi.KindWHEN:
			return 1
		}
	}
	for n := r.node.Parent(); n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindFILE, psi.KindCLASS_BODY:
			return 3
		case psi.KindBLOCK, psi.KindFUNCTION_LITERAL, psi.KindWHEN, psi.KindVALUE_ARGUMENT_LIST, psi.KindVALUE_PARAMETER_LIST:
			return 2
		}
	}
	return 2
}

// codeStart returns the start of n's first token that isn't a comment.
func (f *formatter) codeStart(n psi.NodeRef) int {
	start := n.Range().Start
	i := sort.Search(len(f.toks), func(i int) bool { return f.toks[i].start >= start })
	for ; i < len(f.toks) && f.toks[i].start < n.Range().End; i++ {
		if !comment(f.toks[i].kind) {
			return f.toks[i].start
		}
	}
	return start
}

func comment(k psi.Kind) bool {
	switch k {
	case psi.KindEOL_COMMENT, psi.KindBLOCK_COMMENT, psi.KindKDOC, psi.KindSHEBANG_COMMENT:
		return true
	}
	return false
}

// indentOf returns the indent of token t that starts a line: one more
// than the line where its innermost enclosing node that starts on an
// earlier line does, a continuation; or the same, for a closing bracket,
// an else or catch, a declaration's keyword after its annotations, and
// the nodes that don't indent (structuralIndent).
func (f *formatter) indentOf(t token) string {
	tl := f.line(t.start)
	for c, a := t.node, t.node.Parent(); a.Valid(); c, a = a, a.Parent() {
		al := f.line(f.codeStart(a))
		if al >= tl {
			continue
		}
		if o := owner(a); o.Valid() {
			al = min(al, f.line(f.codeStart(o)))
		}
		base := f.lineIndent(al)
		if sameIndent(a, c, t) {
			return base
		}
		return base + f.unit
	}
	return ""
}

// owner returns what a braced body belongs to, whose first line its
// contents indent from, as IntelliJ does: a class, function, if or call,
// which may start lines before the '{'. Nothing for other nodes.
func owner(n psi.NodeRef) psi.NodeRef {
	switch n.NumericKind() {
	case psi.KindBLOCK:
		if n.FirstChild().NumericKind() != psi.KindLBRACE {
			return psi.NodeRef{}
		}
	case psi.KindCLASS_BODY, psi.KindFUNCTION_LITERAL:
	default:
		return psi.NodeRef{}
	}
	o := n.Parent()
	for {
		switch o.NumericKind() {
		case psi.KindTHEN, psi.KindELSE, psi.KindBODY, psi.KindLAMBDA_EXPRESSION, psi.KindLAMBDA_ARGUMENT, psi.KindANNOTATED_EXPRESSION, psi.KindLABELED_EXPRESSION:
			o = o.Parent()
			continue
		case psi.KindVALUE_ARGUMENT:
			return psi.NodeRef{} // a lambda in parentheses: its own line
		}
		return o
	}
}

// declarationKinds are the nodes whose annotations and modifiers may
// be on lines of their own, at the declaration's indent.
var declarationKinds = map[psi.Kind]bool{
	psi.KindCLASS: true, psi.KindOBJECT_DECLARATION: true, psi.KindFUN: true, psi.KindPROPERTY: true,
	psi.KindSECONDARY_CONSTRUCTOR: true, psi.KindTYPEALIAS: true, psi.KindENUM_ENTRY: true,
	psi.KindVALUE_PARAMETER: true, psi.KindCLASS_INITIALIZER: true, psi.KindPROPERTY_ACCESSOR: true,
	psi.KindTYPE_PARAMETER: true, psi.KindPRIMARY_CONSTRUCTOR: true,
}

// sameIndent reports whether a line starting with t, in child c of a,
// has the indent of a's first line.
func sameIndent(a, c psi.NodeRef, t token) bool {
	switch a.NumericKind() {
	case psi.KindFILE, psi.KindIMPORT_LIST, psi.KindPACKAGE_DIRECTIVE, psi.KindMODIFIER_LIST, psi.KindSCRIPT, psi.KindFILE_ANNOTATION_LIST:
		return true
	case psi.KindBLOCK:
		if p := a.Parent().NumericKind(); p == psi.KindFUNCTION_LITERAL || p == psi.KindSCRIPT {
			return true // a lambda's statements: the lambda indents them
		}
	case psi.KindIF:
		if c.NumericKind() == psi.KindELSE_KEYWORD {
			return true
		}
	case psi.KindTRY:
		if k := c.NumericKind(); k == psi.KindCATCH || k == psi.KindFINALLY {
			return true
		}
	case psi.KindDO_WHILE:
		if c.NumericKind() == psi.KindWHILE_KEYWORD {
			return true
		}
	}
	if c == t.node {
		switch t.kind {
		case psi.KindRBRACE, psi.KindRPAR, psi.KindRBRACKET, psi.KindGT:
			return true
		}
	}
	if declarationKinds[a.NumericKind()] || a.NumericKind() == psi.KindANNOTATED_EXPRESSION {
		for s := a.FirstChild(); s.Valid() && s != c; s = s.NextSibling() {
			switch k := s.NumericKind(); {
			case k == psi.KindMODIFIER_LIST, k == psi.KindANNOTATION_ENTRY, k == psi.KindANNOTATION, k == psi.KindWHITE_SPACE, comment(k):
			default:
				return false
			}
		}
		return true
	}
	return false
}

// space returns the whitespace between tokens l and r on one line, gap
// as it is: none inside brackets, before a comma and around a dot, one
// around binary operators, arrows and braces, and so on. Where no rule
// applies, gap stays.
func (f *formatter) space(l, r token, gap string) string {
	lk, rk := l.kind, r.kind
	lp, rp := l.node.Parent(), r.node.Parent()
	switch {
	case comment(lk) || comment(rk):
		return gap
	case rk == psi.KindCOMMA || rk == psi.KindSEMICOLON:
		return ""
	case lk == psi.KindCOMMA:
		if closing(rk) {
			return ""
		}
		return " "
	case lk == psi.KindDOT || lk == psi.KindSAFE_ACCESS:
		return ""
	case rk == psi.KindDOT || rk == psi.KindSAFE_ACCESS:
		if lk == psi.KindINTEGER_LITERAL || lk == psi.KindFLOAT_LITERAL {
			return gap
		}
		return ""
	case lk == psi.KindCOLONCOLON:
		return ""
	case rk == psi.KindCOLONCOLON:
		if wordEnd(lk) {
			return ""
		}
		return gap
	case lk == psi.KindLPAR || lk == psi.KindLBRACKET:
		return ""
	case rk == psi.KindRPAR || rk == psi.KindRBRACKET:
		return ""
	case lk == psi.KindLBRACE && rk == psi.KindRBRACE:
		return gap
	case lk == psi.KindLBRACE || rk == psi.KindRBRACE:
		return " "
	case rk == psi.KindLBRACE:
		if lk == psi.KindAT {
			return gap
		}
		return " "
	case lk == psi.KindARROW || rk == psi.KindARROW:
		return " "
	case rk == psi.KindQUEST && rp.NumericKind() == psi.KindNULLABLE_TYPE, rk == psi.KindEXCLEXCL:
		return ""
	case prefixOp(l):
		if strings.ContainsAny(f.src[r.start:r.start+1], "+-!&|=<>.?:*/%") {
			return gap // --x would lex as a decrement
		}
		return ""
	case postfixOp(r):
		return ""
	case binaryOp(l) || binaryOp(r):
		op := l
		if binaryOp(r) {
			op = r
		}
		if t := f.src[op.start:op.end]; t == ".." || t == "..<" {
			return ""
		}
		return " "
	case lk == psi.KindEQ || rk == psi.KindEQ:
		eq := lp
		if rk == psi.KindEQ {
			eq = rp
		}
		switch eq.NumericKind() {
		case psi.KindPROPERTY, psi.KindFUN, psi.KindVALUE_PARAMETER, psi.KindVALUE_ARGUMENT, psi.KindTYPEALIAS, psi.KindPROPERTY_ACCESSOR, psi.KindDESTRUCTURING_DECLARATION:
			return " "
		}
	case rk == psi.KindCOLON || lk == psi.KindCOLON:
		colon := lp
		if rk == psi.KindCOLON {
			colon = rp
		}
		switch colon.NumericKind() {
		case psi.KindVALUE_PARAMETER, psi.KindPROPERTY, psi.KindFUN, psi.KindDESTRUCTURING_DECLARATION_ENTRY, psi.KindPROPERTY_ACCESSOR:
			if rk == psi.KindCOLON {
				return ""
			}
			return " "
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindSECONDARY_CONSTRUCTOR, psi.KindTYPE_PARAMETER, psi.KindTYPE_CONSTRAINT, psi.KindENUM_ENTRY:
			return " "
		}
	case rk == psi.KindLPAR:
		switch lk {
		case psi.KindIF_KEYWORD, psi.KindWHEN_KEYWORD, psi.KindFOR_KEYWORD, psi.KindWHILE_KEYWORD, psi.KindCATCH_KEYWORD:
			return " "
		}
		// A call's or declaration's parameters; not a function type's,
		// where @A () -> R would become an annotation's arguments.
		if k := rp.NumericKind(); k == psi.KindVALUE_ARGUMENT_LIST || k == psi.KindVALUE_PARAMETER_LIST && rp.Parent().NumericKind() != psi.KindFUNCTION_TYPE {
			switch lk {
			case psi.KindIDENTIFIER, psi.KindGT, psi.KindRPAR, psi.KindRBRACKET, psi.KindTHIS_KEYWORD, psi.KindSUPER_KEYWORD, psi.KindCONSTRUCTOR_KEYWORD:
				return ""
			}
		}
	case rk == psi.KindLBRACKET && rp.NumericKind() == psi.KindINDICES:
		return ""
	case rk == psi.KindLT && lk == psi.KindIDENTIFIER && typeList(rp):
		return ""
	case lk == psi.KindLT && typeList(lp), rk == psi.KindGT && typeList(rp):
		return ""
	case lk == psi.KindRBRACE:
		switch rk {
		case psi.KindELSE_KEYWORD, psi.KindCATCH_KEYWORD, psi.KindFINALLY_KEYWORD, psi.KindWHILE_KEYWORD:
			return " "
		}
	}
	return gap
}

func closing(k psi.Kind) bool {
	return k == psi.KindRPAR || k == psi.KindRBRACKET || k == psi.KindGT || k == psi.KindRBRACE
}

// wordEnd reports whether a token of kind k ends an expression or type
// that :: may follow without a space.
func wordEnd(k psi.Kind) bool {
	switch k {
	case psi.KindIDENTIFIER, psi.KindRPAR, psi.KindGT, psi.KindRBRACKET, psi.KindQUEST, psi.KindTHIS_KEYWORD, psi.KindSUPER_KEYWORD:
		return true
	}
	return false
}

func typeList(n psi.NodeRef) bool {
	k := n.NumericKind()
	return k == psi.KindTYPE_ARGUMENT_LIST || k == psi.KindTYPE_PARAMETER_LIST
}

// operation returns the expression whose operator t is, if it is one.
func operation(t token) psi.NodeRef {
	p := t.node.Parent()
	if p.NumericKind() != psi.KindOPERATION_REFERENCE {
		return psi.NodeRef{}
	}
	return p.Parent()
}

func prefixOp(t token) bool { return operation(t).NumericKind() == psi.KindPREFIX_EXPRESSION }

func postfixOp(t token) bool { return operation(t).NumericKind() == psi.KindPOSTFIX_EXPRESSION }

func binaryOp(t token) bool {
	switch operation(t).NumericKind() {
	case psi.KindBINARY_EXPRESSION, psi.KindBINARY_WITH_TYPE, psi.KindIS_EXPRESSION:
		return true
	}
	return false
}
