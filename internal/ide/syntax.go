package ide

import (
	"sort"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// The syntax tier: features that need only the document's own tree, and
// so are answered on every change (docs/lsp.md, milestone L0).

// Severity of a diagnostic.
type Severity int

const (
	Error Severity = iota + 1
	Warning
	Information
	Hint
)

// Diagnostic is a problem in a document.
type Diagnostic struct {
	Range    psi.Range
	Severity Severity
	Code     string
	Message  string
}

// SyntaxDiagnostics returns the document's syntax errors, which psi
// reports with kotlinc's messages.
func SyntaxDiagnostics(d *Document) []Diagnostic {
	var out []Diagnostic
	for _, e := range d.Snap.Diagnostics() {
		out = append(out, Diagnostic{Range: e.Range, Severity: Error, Code: e.Code, Message: e.Message})
	}
	return out
}

// SymbolKind is the kind of a declaration in the outline.
type SymbolKind int

const (
	ClassSymbol SymbolKind = iota + 1
	InterfaceSymbol
	EnumSymbol
	ObjectSymbol
	TypeAliasSymbol
	FunctionSymbol
	MethodSymbol
	ConstructorSymbol
	PropertySymbol
	ConstantSymbol
	EnumEntrySymbol
)

// Symbol is a declaration in the outline.
type Symbol struct {
	Name   string
	Detail string // the signature after the name, as written
	Kind   SymbolKind
	// Range is the whole declaration; NameRange is its name, or the
	// keyword of an unnamed one.
	Range, NameRange psi.Range
	Children         []Symbol
}

// Symbols returns the document's declarations as an outline: top-level
// declarations and, nested, the members of classes and objects. Local
// declarations are left out, as in IntelliJ's structure view.
func Symbols(d *Document) []Symbol {
	return symbolList(d.Snap.DeclarationNodes(), false)
}

func symbolList(nodes []psi.NodeRef, inClass bool) []Symbol {
	var out []Symbol
	for _, n := range nodes {
		if s, ok := symbol(n, inClass); ok {
			out = append(out, s)
		}
	}
	return out
}

func symbol(n psi.NodeRef, inClass bool) (Symbol, bool) {
	s := Symbol{Name: n.Name(), Range: n.Range(), NameRange: n.NameIdentifier().Range()}
	switch n.NumericKind() {
	case psi.KindCLASS:
		s.Kind = ClassSymbol
		switch {
		case n.ChildKind(psi.KindINTERFACE_KEYWORD).Valid():
			s.Kind = InterfaceSymbol
		case hasModifier(n, psi.KindENUM_KEYWORD):
			s.Kind = EnumSymbol
		}
		s.Children = classMembers(n)
	case psi.KindOBJECT_DECLARATION:
		s.Kind = ObjectSymbol
		if !n.NameIdentifier().Valid() {
			s.NameRange = n.ChildKind(psi.KindOBJECT_KEYWORD).Range()
		}
		s.Children = classMembers(n)
	case psi.KindTYPEALIAS:
		s.Kind = TypeAliasSymbol
		if t := n.ChildKind(psi.KindTYPE_REFERENCE); t.Valid() {
			s.Detail = "= " + compact(t.Text())
		}
	case psi.KindFUN:
		s.Kind = FunctionSymbol
		if inClass {
			s.Kind = MethodSymbol
		}
		s.Detail = signature(n)
	case psi.KindSECONDARY_CONSTRUCTOR:
		s.Kind = ConstructorSymbol
		s.Name = "constructor"
		s.NameRange = n.ChildKind(psi.KindCONSTRUCTOR_KEYWORD).Range()
		s.Detail = compact(n.ChildKind(psi.KindVALUE_PARAMETER_LIST).Text())
	case psi.KindPROPERTY:
		s.Kind = PropertySymbol
		if hasModifier(n, psi.KindCONST_KEYWORD) {
			s.Kind = ConstantSymbol
		}
		s.Detail = typeDetail(n)
	case psi.KindVALUE_PARAMETER: // a primary constructor's val or var
		s.Kind = PropertySymbol
		s.Detail = typeDetail(n)
	case psi.KindENUM_ENTRY:
		s.Kind = EnumEntrySymbol
	default:
		return Symbol{}, false
	}
	if s.Name == "" {
		return Symbol{}, false // a declaration too broken to name
	}
	return s, true
}

// classMembers returns the properties a class's primary constructor
// declares, then the members of its body.
func classMembers(n psi.NodeRef) []Symbol {
	var nodes []psi.NodeRef
	for _, p := range n.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).Parameters() {
		if p.ChildKind(psi.KindVAL_KEYWORD).Valid() || p.ChildKind(psi.KindVAR_KEYWORD).Valid() {
			nodes = append(nodes, p)
		}
	}
	for c := n.ChildKind(psi.KindCLASS_BODY).FirstChild(); c.Valid(); c = c.NextSibling() {
		nodes = append(nodes, c)
	}
	return symbolList(nodes, true)
}

func hasModifier(n psi.NodeRef, k psi.Kind) bool {
	return n.ChildKind(psi.KindMODIFIER_LIST).ChildKind(k).Valid()
}

// signature returns a function's receiver, parameters and return type as
// written, without its name: "String.(n: Int): Boolean".
func signature(n psi.NodeRef) string {
	var b strings.Builder
	if r := n.ReceiverType(); r.Valid() {
		b.WriteString(compact(r.Text()))
		b.WriteByte('.')
	}
	b.WriteString(compact(n.ChildKind(psi.KindVALUE_PARAMETER_LIST).Text()))
	if t := n.ReturnType(); t.Valid() {
		b.WriteString(": ")
		b.WriteString(compact(t.Text()))
	}
	return b.String()
}

func typeDetail(n psi.NodeRef) string {
	if t := n.TypeReference(); t.Valid() {
		return ": " + compact(t.Text())
	}
	return ""
}

// compact collapses runs of whitespace, comments aside, to one space.
func compact(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// FoldKind is what a folding range holds.
type FoldKind int

const (
	FoldRegion FoldKind = iota
	FoldComment
	FoldImports
)

// Fold is a range of zero-based lines an editor can collapse; StartLine
// stays visible.
type Fold struct {
	StartLine, EndLine int
	Kind               FoldKind
}

// Folds returns the document's folding ranges: the imports, each braced
// block, body or lambda (keeping the closing brace's line visible), and
// block comments and KDoc.
func Folds(d *Document) []Fold {
	var out []Fold
	add := func(start, end int, k FoldKind) {
		if end > start {
			out = append(out, Fold{start, end, k})
		}
	}
	psi.WalkNodes(d.Snap.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindIMPORT_LIST:
			var first, last psi.NodeRef
			for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
				if c.NumericKind() == psi.KindIMPORT_DIRECTIVE {
					if !first.Valid() {
						first = c
					}
					last = c
				}
			}
			if first.Valid() {
				add(d.Line(first.Range().Start), d.Line(last.Range().End), FoldImports)
			}
			return false
		case psi.KindBLOCK_COMMENT, psi.KindKDOC:
			r := n.Range()
			add(d.Line(r.Start), d.Line(r.End), FoldComment)
			return false
		}
		if n.IsLeaf() {
			return false
		}
		if l, r := n.ChildKind(psi.KindLBRACE), lastChildKind(n, psi.KindRBRACE); l.Valid() && r.Valid() {
			add(d.Line(l.Range().Start), d.Line(r.Range().Start)-1, FoldRegion)
		}
		return true
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartLine < out[j].StartLine })
	return out
}

func lastChildKind(n psi.NodeRef, k psi.Kind) psi.NodeRef {
	var last psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == k {
			last = c
		}
	}
	return last
}

// SelectionRanges returns the ranges an editor's "expand selection" walks
// through from offset off, innermost first: the token there, then each
// enclosing element with a larger range, up to the whole file. At the end
// of a token (a caret just after a name) the token is preferred to the
// whitespace after it.
func SelectionRanges(d *Document, off int) []psi.Range {
	n := d.Snap.NodeAt(off)
	if n.IsTrivia() && off == n.Range().Start && off > 0 {
		if p := d.Snap.NodeAt(off - 1); p.Valid() && !p.IsTrivia() {
			n = p
		}
	}
	var out []psi.Range
	for ; n.Valid(); n = n.Parent() {
		r := n.Range()
		if len(out) > 0 && out[len(out)-1] == r {
			continue
		}
		out = append(out, r)
	}
	return out
}

// TokenType classifies a highlighted token.
type TokenType int

// The token types, in the order of TokenTypeNames.
const (
	NamespaceToken TokenType = iota
	TypeToken
	ClassToken
	EnumToken
	InterfaceToken
	TypeParameterToken
	ParameterToken
	VariableToken
	PropertyToken
	EnumMemberToken
	FunctionToken
	MethodToken
	KeywordToken
	CommentToken
	StringToken
	NumberToken
	DecoratorToken
)

// TokenTypeNames are the LSP names of the token types.
var TokenTypeNames = []string{
	"namespace", "type", "class", "enum", "interface", "typeParameter", "parameter", "variable",
	"property", "enumMember", "function", "method", "keyword", "comment", "string", "number", "decorator",
}

// Token modifiers, as bits.
const (
	DeclarationMod = 1 << iota
	ReadonlyMod
	DocumentationMod
)

// TokenModifierNames are the LSP names of the modifier bits, lowest first.
var TokenModifierNames = []string{"declaration", "readonly", "documentation"}

// Token is a highlighted range.
type Token struct {
	Range psi.Range
	Type  TokenType
	Mods  int
}

// LexicalTokens returns the tokens the syntax alone classifies, in order:
// keywords, comments, strings, numbers, annotations and the names of
// declarations. References are left to the semantic tier.
func LexicalTokens(d *Document) []Token {
	var out []Token
	add := func(n psi.NodeRef, t TokenType, mods int) {
		if r := n.Range(); r.End > r.Start {
			out = append(out, Token{r, t, mods})
		}
	}
	psi.WalkNodes(d.Snap.RootNode(), func(n psi.NodeRef) bool {
		k := n.NumericKind()
		if k == psi.KindANNOTATION_ENTRY {
			psi.WalkNodes(n, func(c psi.NodeRef) bool {
				if c.NumericKind() == psi.KindAT || c.NumericKind() == psi.KindIDENTIFIER && inside(c, n.ChildKind(psi.KindCONSTRUCTOR_CALLEE)) {
					add(c, DecoratorToken, 0)
					return false
				}
				if c.NumericKind() == psi.KindVALUE_ARGUMENT_LIST {
					return false // the arguments are walked as usual below
				}
				return true
			})
			if args := n.ChildKind(psi.KindVALUE_ARGUMENT_LIST); args.Valid() {
				psi.WalkNodes(args, func(c psi.NodeRef) bool { return lexical(c, add) })
			}
			return false
		}
		return lexical(n, add)
	})
	return out
}

// lexical classifies a node outside annotations; it returns whether to
// visit its children.
func lexical(n psi.NodeRef, add func(psi.NodeRef, TokenType, int)) bool {
	if !n.IsLeaf() {
		return true
	}
	switch k := n.NumericKind(); k {
	case psi.KindEOL_COMMENT, psi.KindBLOCK_COMMENT, psi.KindSHEBANG_COMMENT:
		add(n, CommentToken, 0)
	case psi.KindKDOC:
		add(n, CommentToken, DocumentationMod)
	case psi.KindOPEN_QUOTE, psi.KindCLOSING_QUOTE, psi.KindREGULAR_STRING_PART, psi.KindCHARACTER_LITERAL, psi.KindESCAPE_SEQUENCE:
		add(n, StringToken, 0)
	case psi.KindINTEGER_LITERAL, psi.KindFLOAT_LITERAL:
		add(n, NumberToken, 0)
	case psi.KindIDENTIFIER:
		if t, mods, ok := declarationName(n); ok {
			add(n, t, mods)
		}
	default:
		if strings.HasSuffix(n.Kind(), "_KEYWORD") {
			add(n, KeywordToken, 0)
		}
	}
	return false
}

func inside(n, outer psi.NodeRef) bool {
	return outer.Valid() && n.ID() >= outer.ID() && n.ID() < outer.SubtreeEnd()
}

// declarationName classifies the name identifier of a declaration.
func declarationName(id psi.NodeRef) (TokenType, int, bool) {
	p := id.Parent()
	if p.NumericKind() == psi.KindREFERENCE_EXPRESSION {
		if inPackageDirective(p) {
			return NamespaceToken, 0, true
		}
		return 0, 0, false
	}
	if p.NameIdentifier() != id {
		return 0, 0, false
	}
	readonly := 0
	if p.ChildKind(psi.KindVAL_KEYWORD).Valid() {
		readonly = ReadonlyMod
	}
	member := p.Parent().NumericKind() == psi.KindCLASS_BODY
	switch p.NumericKind() {
	case psi.KindCLASS:
		switch {
		case p.ChildKind(psi.KindINTERFACE_KEYWORD).Valid():
			return InterfaceToken, DeclarationMod, true
		case hasModifier(p, psi.KindENUM_KEYWORD):
			return EnumToken, DeclarationMod, true
		}
		return ClassToken, DeclarationMod, true
	case psi.KindOBJECT_DECLARATION:
		return ClassToken, DeclarationMod, true
	case psi.KindTYPEALIAS:
		return TypeToken, DeclarationMod, true
	case psi.KindTYPE_PARAMETER:
		return TypeParameterToken, DeclarationMod, true
	case psi.KindFUN:
		if member {
			return MethodToken, DeclarationMod, true
		}
		return FunctionToken, DeclarationMod, true
	case psi.KindPROPERTY:
		if member || topLevel(p) {
			return PropertyToken, DeclarationMod | readonly, true
		}
		return VariableToken, DeclarationMod | readonly, true
	case psi.KindVALUE_PARAMETER:
		if readonly != 0 || p.ChildKind(psi.KindVAR_KEYWORD).Valid() {
			return PropertyToken, DeclarationMod | readonly, true
		}
		return ParameterToken, DeclarationMod, true
	case psi.KindENUM_ENTRY:
		return EnumMemberToken, DeclarationMod | ReadonlyMod, true
	case psi.KindDESTRUCTURING_DECLARATION_ENTRY:
		return VariableToken, DeclarationMod, true
	}
	return 0, 0, false
}

func topLevel(n psi.NodeRef) bool {
	k := n.Parent().NumericKind()
	return k == psi.KindFILE || k == psi.KindBLOCK && n.Parent().Parent().NumericKind() == psi.KindSCRIPT
}

func inPackageDirective(n psi.NodeRef) bool {
	for ; n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindPACKAGE_DIRECTIVE:
			return true
		case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindREFERENCE_EXPRESSION:
			continue
		}
		return false
	}
	return false
}
