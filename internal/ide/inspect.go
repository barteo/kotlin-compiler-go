package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Inspections: warnings of code that compiles but is redundant, as
// kotlinc's and IntelliJ's, each with a quick fix (fixes). They are
// judged on a file without diagnostics only, whose facts are complete,
// so that broken code gets no false warnings: by the facts' types for
// the safe calls, assertions and casts that do nothing, and by names for
// what is never used or reassigned.

// Codes of the inspections' warnings.
const (
	UnnecessarySafeCall         = "UNNECESSARY_SAFE_CALL"
	UnnecessaryNotNullAssertion = "UNNECESSARY_NOT_NULL_ASSERTION"
	UselessCast                 = "USELESS_CAST"
	CanBeVal                    = "CAN_BE_VAL"
	UnusedParameter             = "UNUSED_PARAMETER"
	UnusedPrivateMember         = "UNUSED_PRIVATE_MEMBER"
)

// Inspections returns the warnings of path's file in v; none if the file
// has diagnostics.
func Inspections(v *View, path string) []Diagnostic {
	snap := v.byPath[path]
	if snap == nil {
		return nil
	}
	for _, d := range v.res.Diagnostics() {
		if d.Path == path {
			return nil
		}
	}
	var out []Diagnostic
	warn := func(r psi.Range, code, msg string, unnecessary bool) {
		out = append(out, Diagnostic{Range: r, Severity: Warning, Code: code, Message: msg, Unnecessary: unnecessary})
	}
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindSAFE_ACCESS_EXPRESSION:
			if f, ok := v.fact(n); ok && f.FlowNonNull && f.DefinitelyNonNull {
				if op := n.ChildKind(psi.KindSAFE_ACCESS); op.Valid() {
					if t, ok := operandType(v, firstNonTrivia(n)); ok && !t.Nullable && !t.Platform {
						warn(op.Range(), UnnecessarySafeCall, "Unnecessary safe call on a non-null receiver of type "+renderType(t), false)
					}
				}
			}
		case psi.KindPOSTFIX_EXPRESSION:
			op := n.ChildKind(psi.KindOPERATION_REFERENCE)
			if op.FirstChild().NumericKind() == psi.KindEXCLEXCL {
				if t, ok := operandType(v, firstNonTrivia(n)); ok && !t.Nullable && !t.Platform && !t.Parameter {
					warn(op.Range(), UnnecessaryNotNullAssertion, "Unnecessary non-null assertion (!!) on a non-null receiver of type "+renderType(t), false)
				}
			}
		case psi.KindBINARY_WITH_TYPE:
			if uselessCast(v, n) {
				op := n.ChildKind(psi.KindOPERATION_REFERENCE)
				warn(psi.Range{Start: op.Range().Start, End: n.Range().End}, UselessCast, "No cast needed", true)
			}
		case psi.KindPROPERTY:
			if canBeVal(n) {
				warn(n.ChildKind(psi.KindVAR_KEYWORD).Range(), CanBeVal, "Variable '"+n.Name()+"' is never modified and can be declared immutable using 'val'", false)
			}
		case psi.KindFUN:
			for _, p := range unusedParameters(n) {
				warn(nameRange(p), UnusedParameter, "Parameter '"+p.Name()+"' is never used", true)
			}
		}
		return true
	})
	for _, x := range styleFindings(snap, v.ExplicitAPI) {
		out = append(out, x.Diagnostic)
	}
	for _, x := range idiomFindings(v, snap) {
		out = append(out, x.Diagnostic)
	}
	for _, d := range unusedPrivate(snap) {
		what := "Function"
		if d.NumericKind() == psi.KindPROPERTY {
			what = "Property"
		}
		warn(nameRange(d), UnusedPrivateMember, what+" '"+d.Name()+"' is never used", true)
	}
	slices.SortStableFunc(out, func(a, b Diagnostic) int { return a.Range.Start - b.Range.Start })
	return out
}

// operandType returns the type of expression x, as written: not one a
// smart cast gives it, nor a literal's that the expected type chose.
func operandType(v *View, x psi.NodeRef) (analysis.Type, bool) {
	for x.NumericKind() == psi.KindPARENTHESIZED {
		x = lastExpression(x)
	}
	switch x.NumericKind() {
	case psi.KindINTEGER_CONSTANT, psi.KindFLOAT_CONSTANT, psi.KindNULL, psi.KindPREFIX_EXPRESSION, psi.KindLAMBDA_EXPRESSION,
		psi.KindANNOTATED_EXPRESSION, psi.KindLABELED_EXPRESSION, psi.KindIF, psi.KindWHEN, psi.KindTRY, psi.KindOBJECT_LITERAL,
		psi.KindCALLABLE_REFERENCE_EXPRESSION, psi.KindCOLLECTION_LITERAL_EXPRESSION:
		return analysis.Type{}, false
	}
	f, ok := v.fact(x)
	if !ok || f.Type == nil || f.Type.Name == "" || f.FlowNonNull {
		return analysis.Type{}, false
	}
	return *f.Type, true
}

// uselessCast reports whether a cast, x as T, casts x to the type it has.
func uselessCast(v *View, n psi.NodeRef) bool {
	op := n.ChildKind(psi.KindOPERATION_REFERENCE)
	if op.FirstChild().NumericKind() != psi.KindAS_KEYWORD {
		return false
	}
	x := firstNonTrivia(n)
	if inferredCall(x) {
		return false // the cast may be what infers its type arguments
	}
	t, ok := operandType(v, x)
	if !ok || t.Platform || hasParameter(t) {
		return false
	}
	f, ok := v.fact(n)
	if !ok || f.Type == nil || f.ImpossibleCast {
		return false
	}
	return sameType(t, *f.Type)
}

// inferredCall reports whether x is a call, or a qualified one, without
// type arguments.
func inferredCall(x psi.NodeRef) bool {
	for {
		switch x.NumericKind() {
		case psi.KindPARENTHESIZED:
			x = lastExpression(x)
		case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
			x = lastExpression(x)
		case psi.KindCALL_EXPRESSION:
			return !x.ChildKind(psi.KindTYPE_ARGUMENT_LIST).Valid()
		default:
			return false
		}
	}
}

// sameType reports whether a and b are the same type, with their
// arguments and nullability.
func sameType(a, b analysis.Type) bool {
	if a.Name != b.Name || a.Nullable != b.Nullable || a.Platform != b.Platform || a.Projection != b.Projection ||
		a.Parameter || b.Parameter || len(a.Arguments) != len(b.Arguments) {
		return false
	}
	for i := range a.Arguments {
		if !sameType(a.Arguments[i], b.Arguments[i]) {
			return false
		}
	}
	return true
}

// canBeVal reports whether a local var is never assigned after its
// initializer: no assignment, compound assignment, ++ or -- of its name in
// its block.
func canBeVal(n psi.NodeRef) bool {
	if n.Parent().NumericKind() != psi.KindBLOCK || !n.ChildKind(psi.KindVAR_KEYWORD).Valid() ||
		!n.ChildKind(psi.KindEQ).Valid() || n.ChildKind(psi.KindPROPERTY_DELEGATE).Valid() || n.ChildKind(psi.KindMODIFIER_LIST).Valid() {
		return false
	}
	name := n.Name()
	if name == "" || name == "_" {
		return false
	}
	assigned := false
	psi.WalkNodes(n.Parent(), func(m psi.NodeRef) bool {
		if assigned {
			return false
		}
		var target psi.NodeRef
		switch m.NumericKind() {
		case psi.KindBINARY_EXPRESSION:
			switch m.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() {
			case psi.KindEQ, psi.KindPLUSEQ, psi.KindMINUSEQ, psi.KindMULTEQ, psi.KindDIVEQ, psi.KindPERCEQ:
				target = firstNonTrivia(m)
			}
		case psi.KindPOSTFIX_EXPRESSION, psi.KindPREFIX_EXPRESSION:
			switch m.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() {
			case psi.KindPLUSPLUS, psi.KindMINUSMINUS:
				target = lastExpression(m)
				if m.NumericKind() == psi.KindPOSTFIX_EXPRESSION {
					target = firstNonTrivia(m)
				}
			}
		}
		for target.NumericKind() == psi.KindPARENTHESIZED {
			target = lastExpression(target)
		}
		if target.NumericKind() == psi.KindREFERENCE_EXPRESSION && psi.Unquote(target.Text()) == name {
			assigned = true
		}
		return !assigned
	})
	return !assigned
}

// unusedParameters returns the parameters of a private or local function
// that its body and its parameters' defaults never name; none of an
// override, an operator, an open, abstract, external or expect function,
// or a main function.
func unusedParameters(f psi.NodeRef) []psi.NodeRef {
	body := f.Body()
	if !body.Valid() || f.Name() == "main" || f.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindANNOTATION_ENTRY).Valid() {
		return nil
	}
	for _, k := range []psi.Kind{psi.KindOVERRIDE_KEYWORD, psi.KindOPERATOR_KEYWORD, psi.KindOPEN_KEYWORD, psi.KindABSTRACT_KEYWORD,
		psi.KindEXTERNAL_KEYWORD, psi.KindEXPECT_KEYWORD, psi.KindACTUAL_KEYWORD, psi.KindINFIX_KEYWORD} {
		if hasModifier(f, k) {
			return nil
		}
	}
	local := f.Parent().NumericKind() == psi.KindBLOCK
	if !local && !hasModifier(f, psi.KindPRIVATE_KEYWORD) {
		return nil
	}
	var out []psi.NodeRef
	for _, p := range f.Parameters() {
		name := p.Name()
		if name == "" || name == "_" || p.ChildKind(psi.KindMODIFIER_LIST).Valid() {
			continue
		}
		if !namedIn(name, body, p) && !namedInDefaults(name, f, p) {
			out = append(out, p)
		}
	}
	return out
}

// namedIn reports whether an identifier other than except's name writes
// name in n.
func namedIn(name string, n, except psi.NodeRef) bool {
	found := false
	psi.WalkNodes(n, func(m psi.NodeRef) bool {
		if found {
			return false
		}
		if m.NumericKind() == psi.KindIDENTIFIER && m.Parent() != except && psi.Unquote(m.Text()) == name {
			found = true
		}
		return !found
	})
	return found
}

// namedInDefaults reports whether another parameter of f names name in
// its default value.
func namedInDefaults(name string, f, p psi.NodeRef) bool {
	for _, q := range f.Parameters() {
		if q != p && namedIn(name, q, q) {
			return true
		}
	}
	return false
}

// unusedPrivate returns the private functions and properties of snap's
// classes, objects and top level that no other identifier of the file
// names, nor a string literal holds. Annotated ones, overrides, operators and constructors' are left
// out, as are overloads and names a class may serialize.
func unusedPrivate(snap *psi.Snapshot) []psi.NodeRef {
	var cands []psi.NodeRef
	count := map[string]int{}
	var collect func(container psi.NodeRef)
	collect = func(container psi.NodeRef) {
		for d := container.FirstChild(); d.Valid(); d = d.NextSibling() {
			switch d.NumericKind() {
			case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
				collect(d.ChildKind(psi.KindCLASS_BODY))
			case psi.KindFUN, psi.KindPROPERTY:
				count[d.Name()]++
				if privateCandidate(d) {
					cands = append(cands, d)
				}
			}
		}
	}
	collect(snap.RootNode())
	used := map[string]bool{}
	psi.WalkNodes(snap.RootNode(), func(m psi.NodeRef) bool {
		if m.NumericKind() == psi.KindLITERAL_STRING_TEMPLATE_ENTRY {
			used[m.Text()] = true // a name to reflect on
		}
		if m.NumericKind() == psi.KindIDENTIFIER {
			if p := m.Parent(); !(p.NumericKind() == psi.KindFUN || p.NumericKind() == psi.KindPROPERTY) || p.NameIdentifier() != m {
				used[psi.Unquote(m.Text())] = true
			}
		}
		return true
	})
	var out []psi.NodeRef
	for _, d := range cands {
		if name := d.Name(); !used[name] && count[name] == 1 {
			out = append(out, d)
		}
	}
	return out
}

// privateCandidate reports whether d is a private member an unused
// warning may name.
func privateCandidate(d psi.NodeRef) bool {
	if !hasModifier(d, psi.KindPRIVATE_KEYWORD) || d.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindANNOTATION_ENTRY).Valid() {
		return false
	}
	for _, k := range []psi.Kind{psi.KindOVERRIDE_KEYWORD, psi.KindOPERATOR_KEYWORD, psi.KindEXTERNAL_KEYWORD, psi.KindCONST_KEYWORD} {
		if hasModifier(d, k) {
			return false
		}
	}
	switch d.Name() {
	case "", "main", "serialVersionUID", "readResolve", "writeReplace", "readObject", "writeObject":
		return false
	}
	return true
}

// removeCast replaces a useless cast with its operand.
func (f fixer) removeCast() []Fix {
	n := f.enclosing(psi.KindBINARY_WITH_TYPE)
	if !n.Valid() || n.ChildKind(psi.KindOPERATION_REFERENCE).Range().Start != f.d.Range.Start {
		return nil
	}
	return f.fix("Remove useless cast", true, f.edit(n.Range(), firstNonTrivia(n).Text()))
}

// removeMember removes an unused private declaration, with its lines.
func (f fixer) removeMember() []Fix {
	d := f.declaration(psi.KindFUN, psi.KindPROPERTY)
	if !d.Valid() || nameRange(d) != f.d.Range {
		return nil
	}
	what := "function"
	if d.NumericKind() == psi.KindPROPERTY {
		what = "property"
	}
	return f.fix("Remove "+what+" '"+d.Name()+"'", true, f.edit(lineRange(f.snap.Source(), d.Range()), ""))
}

// removeParameter removes an unused parameter, and its arguments of the
// function's calls in the file. There is no fix when the function is
// overloaded or used other than by a call, or when an argument can't be
// told or has an effect.
func (f fixer) removeParameter() []Fix {
	p := f.declaration(psi.KindVALUE_PARAMETER)
	fun := p.Parent().Parent()
	if !p.Valid() || nameRange(p) != f.d.Range || fun.NumericKind() != psi.KindFUN {
		return nil
	}
	params := fun.Parameters()
	idx := slices.Index(params, p)
	name, pname := fun.Name(), p.Name()
	if idx < 0 || name == "" {
		return nil
	}
	edits := []FixEdit{f.edit(listElement(p), "")}
	ok := true
	psi.WalkNodes(f.snap.RootNode(), func(m psi.NodeRef) bool {
		if !ok {
			return false
		}
		switch m.NumericKind() {
		case psi.KindFUN:
			if m != fun && m.Name() == name {
				ok = false // an overload, or another function of the name
			}
		case psi.KindREFERENCE_EXPRESSION:
			if psi.Unquote(m.Text()) != name {
				return true
			}
			call := m.Parent()
			if call.NumericKind() != psi.KindCALL_EXPRESSION || call.FirstChild() != m {
				ok = false
				return false
			}
			e, found := argumentOf(call, idx, pname)
			switch {
			case e == nil:
				ok = false
			case found:
				edits = append(edits, f.edit(listElement(*e), ""))
			}
		}
		return ok
	})
	if !ok {
		return nil
	}
	return []Fix{{Title: "Remove parameter '" + pname + "'", Preferred: true, Edits: edits}}
}

// argumentOf returns the argument of call for the parameter at idx,
// named pname: found false if the call passes none; nil if it can't be
// told or has an effect.
func argumentOf(call psi.NodeRef, idx int, pname string) (*psi.NodeRef, bool) {
	if call.ChildKind(psi.KindLAMBDA_ARGUMENT).Valid() {
		return nil, false
	}
	var args []psi.NodeRef
	for a := call.ChildKind(psi.KindVALUE_ARGUMENT_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() == psi.KindVALUE_ARGUMENT {
			args = append(args, a)
		}
	}
	var hit psi.NodeRef
	for i, a := range args {
		if a.ChildKind(psi.KindMUL).Valid() {
			return nil, false // a spread
		}
		if n := a.ChildKind(psi.KindVALUE_ARGUMENT_NAME); n.Valid() {
			if psi.Unquote(strings.TrimSpace(n.Text())) == pname {
				hit = a
			} else if i <= idx && !hit.Valid() {
				return nil, false // named before: positions can't be told
			}
			continue
		}
		if i == idx {
			hit = a
		}
	}
	if !hit.Valid() {
		return &hit, false
	}
	if hasCall(hit) {
		return nil, false
	}
	return &hit, true
}

// listElement returns the range that removes e from its comma-separated
// list: with the comma after it and the blanks to the next element, or
// else with the comma before it.
func listElement(e psi.NodeRef) psi.Range {
	r := e.Range()
	next := e.NextSibling()
	for next.Valid() && trivia(next) {
		next = next.NextSibling()
	}
	if next.NumericKind() == psi.KindCOMMA {
		after := next.NextSibling()
		for after.Valid() && strings.TrimSpace(after.Text()) == "" {
			after = after.NextSibling()
		}
		if after.Valid() && after.NumericKind() != psi.KindRPAR {
			return psi.Range{Start: r.Start, End: after.Range().Start}
		}
		r.End = next.Range().End
	}
	var prev psi.NodeRef
	for c := e.Parent().FirstChild(); c.Valid() && c != e; c = c.NextSibling() {
		if !trivia(c) {
			prev = c
		}
	}
	if prev.NumericKind() == psi.KindCOMMA {
		r.Start = prev.Range().Start
	}
	return r
}
