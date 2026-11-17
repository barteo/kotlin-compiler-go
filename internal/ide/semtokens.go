package ide

import (
	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// ReferenceTokens returns the tokens of the names that path's snapshot
// in the view uses, classified by what they name: a declaration in the
// sources as LexicalTokens classifies its name (without the declaration
// modifier), a library symbol by its kind. Offsets are the view's.
func ReferenceTokens(v *View, path string) []Token {
	snap := v.byPath[path]
	if snap == nil {
		return nil
	}
	v.classIndex()
	var out []Token
	for f := range v.res.FileFacts(snap.ID()) {
		n := snap.Node(f.Element.Node)
		var name psi.NodeRef
		switch n.NumericKind() {
		case psi.KindREFERENCE_EXPRESSION:
			name = n.ChildKind(psi.KindIDENTIFIER)
		case psi.KindUSER_TYPE:
			name = lastChild(n, psi.KindREFERENCE_EXPRESSION).ChildKind(psi.KindIDENTIFIER)
		}
		if !name.Valid() {
			continue
		}
		if t, mods, ok := v.classify(f, n); ok {
			out = append(out, Token{name.Range(), t, mods})
		}
	}
	return out
}

// classify returns the token type of the name of reference or type n,
// with fact f.
func (v *View) classify(f analysis.Fact, n psi.NodeRef) (TokenType, int, bool) {
	if f.Package != "" {
		return NamespaceToken, 0, true
	}
	if d, _, ok := v.use(f, n); ok {
		if dn := v.node(d); dn.Valid() {
			if dn.NumericKind() == psi.KindSECONDARY_CONSTRUCTOR || dn.NumericKind() == psi.KindPRIMARY_CONSTRUCTOR {
				return ClassToken, 0, true
			}
			if t, mods, ok := declarationName(dn.NameIdentifier()); ok {
				if n.NumericKind() == psi.KindREFERENCE_EXPRESSION && (t == ClassToken || t == EnumToken || t == InterfaceToken) && f.Call != nil {
					return ClassToken, 0, true // a constructor call
				}
				return t, mods &^ DeclarationMod, true
			}
		}
	}
	if n.NumericKind() == psi.KindUSER_TYPE {
		if f.Type == nil {
			return 0, 0, false
		}
		if f.Type.Parameter {
			return TypeParameterToken, 0, true
		}
		return ClassToken, 0, true
	}
	s, ok := v.symbol(f.Symbol)
	if !ok {
		switch {
		case f.Call != nil:
			return FunctionToken, 0, true
		case f.Role == "reference" && f.Type != nil && !f.Classifier:
			return PropertyToken, 0, true // an intrinsic: String.length
		}
		return 0, 0, false
	}
	switch s.Kind {
	case "class", "constructor":
		return ClassToken, 0, true
	case "function":
		if len(f.Accessors) > 0 { // a Java getter used as a property
			return PropertyToken, readonly(len(f.Accessors) == 1), true
		}
		if s.Owner != "" {
			return MethodToken, 0, true
		}
		return FunctionToken, 0, true
	case "property":
		return PropertyToken, readonly(len(f.Accessors) == 1), true
	case "local":
		return VariableToken, 0, true
	case "parameter":
		return ParameterToken, 0, true
	}
	return 0, 0, false
}

func readonly(b bool) int {
	if b {
		return ReadonlyMod
	}
	return 0
}

// InlayHintKind is what an inlay hint shows.
type InlayHintKind int

const (
	TypeHint InlayHintKind = iota + 1
	ParameterHint
)

// InlayHint is a label shown at an offset: ": Type" after a variable's
// name, or "name:" before an argument.
type InlayHint struct {
	Offset int
	Label  string
	Kind   InlayHintKind
}

// InlayHints returns the hints of path's snapshot in the view within r:
// the inferred types of variables and lambda parameters whose types
// aren't written, unless the initializer is a literal or a constructor
// call that shows it; and the names of the parameters that literal
// positional arguments are passed to.
func InlayHints(v *View, path string, r psi.Range) []InlayHint {
	snap := v.byPath[path]
	if snap == nil {
		return nil
	}
	var out []InlayHint
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		nr := n.Range()
		if nr.End < r.Start || nr.Start > r.End {
			return false
		}
		switch n.NumericKind() {
		case psi.KindPROPERTY, psi.KindDESTRUCTURING_DECLARATION_ENTRY, psi.KindVALUE_PARAMETER:
			if h, ok := v.typeHint(n); ok && h.Offset >= r.Start && h.Offset <= r.End {
				out = append(out, h)
			}
		case psi.KindVALUE_ARGUMENT_LIST:
			for _, h := range v.parameterHints(n) {
				if h.Offset >= r.Start && h.Offset <= r.End {
					out = append(out, h)
				}
			}
		}
		return true
	})
	return out
}

// typeHint returns the hint of variable or lambda parameter d's type.
func (v *View) typeHint(d psi.NodeRef) (InlayHint, bool) {
	id := d.NameIdentifier()
	if !id.Valid() || d.TypeReference().Valid() {
		return InlayHint{}, false
	}
	switch d.NumericKind() {
	case psi.KindVALUE_PARAMETER:
		if d.Parent().Parent().NumericKind() != psi.KindFUNCTION_LITERAL {
			return InlayHint{}, false // a function's, whose type is written
		}
	case psi.KindPROPERTY:
		if d.ChildKind(psi.KindPROPERTY_DELEGATE).Valid() {
			break
		}
		if obvious(v, initializer(d)) {
			return InlayHint{}, false
		}
	}
	f, ok := v.fact(d)
	if !ok {
		return InlayHint{}, false
	}
	s, ok := v.symbol(f.Symbol)
	if !ok || s.Type.Name == "" {
		return InlayHint{}, false
	}
	return InlayHint{Offset: id.Range().End, Label: ": " + renderType(s.Type), Kind: TypeHint}, true
}

// initializer returns the expression after a property's =.
func initializer(d psi.NodeRef) psi.NodeRef {
	eq := false
	for c := d.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindEQ {
			eq = true
		} else if eq && !c.IsTrivia() {
			return c
		}
	}
	return psi.NodeRef{}
}

// obvious reports whether expression e shows its type: a literal, or a
// constructor call.
func obvious(v *View, e psi.NodeRef) bool {
	if literal(e) {
		return true
	}
	if e.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION {
		e = lastChild(e, psi.KindCALL_EXPRESSION)
	}
	if e.NumericKind() != psi.KindCALL_EXPRESSION {
		return false
	}
	callee := e.ChildKind(psi.KindREFERENCE_EXPRESSION)
	f, ok := v.fact(callee)
	if !ok || f.Call == nil {
		return false
	}
	return simpleName(f.Call.Name) == "<init>"
}

// literal reports whether expression e is a literal: a number, string,
// character, boolean or null, or a negated number.
func literal(e psi.NodeRef) bool {
	switch e.NumericKind() {
	case psi.KindINTEGER_CONSTANT, psi.KindFLOAT_CONSTANT, psi.KindBOOLEAN_CONSTANT, psi.KindCHARACTER_CONSTANT,
		psi.KindNULL, psi.KindSTRING_TEMPLATE:
		return true
	case psi.KindPREFIX_EXPRESSION:
		op := e.ChildKind(psi.KindOPERATION_REFERENCE)
		return op.Valid() && op.Text() == "-" && literal(lastNonTrivia(e))
	}
	return false
}

// parameterHints returns the hints of the parameters that a call's
// literal positional arguments, of argument list args, are passed to.
func (v *View) parameterHints(args psi.NodeRef) []InlayHint {
	call := args.Parent()
	if call.NumericKind() != psi.KindCALL_EXPRESSION {
		return nil // an annotation's or a supertype call's
	}
	f, ok := v.fact(call.ChildKind(psi.KindREFERENCE_EXPRESSION))
	if !ok || f.Call == nil {
		f, ok = v.fact(call)
	}
	if !ok || f.Call == nil || f.Call.ValueInvocation || f.Call.FromJava && !f.Call.StableNames {
		return nil
	}
	c := f.Call
	var out []InlayHint
	seen := map[int]bool{}
	i := 0
	for a := args.FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() != psi.KindVALUE_ARGUMENT {
			continue
		}
		k := i
		i++
		if k >= len(c.ArgumentMapping) || a.ChildKind(psi.KindVALUE_ARGUMENT_NAME).Valid() {
			continue
		}
		p := c.ArgumentMapping[k]
		if p < 0 || p >= len(c.ParameterNames) || c.ParameterNames[p] == "" || seen[p] {
			continue
		}
		seen[p] = true
		if e := lastNonTrivia(a); literal(e) {
			out = append(out, InlayHint{Offset: a.Range().Start, Label: c.ParameterNames[p] + ":", Kind: ParameterHint})
		}
	}
	return out
}
