package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Idiom inspections: code that compiles and does what it says, but that
// Kotlin writes shorter, judged by the facts' types, each with a fix
// that rewrites it: as styleFindings, but on a file the view checked.

// Codes of the idiom inspections.
const (
	UselessElvis            = "USELESS_ELVIS"
	RedundantStringTemplate = "REDUNDANT_STRING_TEMPLATE"
	ReplaceGetOrSet         = "REPLACE_GET_OR_SET"
	ReplaceSizeZeroCheck    = "REPLACE_SIZE_ZERO_CHECK_WITH_IS_EMPTY"
	ReplaceSizeCheck        = "REPLACE_SIZE_CHECK_WITH_IS_NOT_EMPTY"
	ReplaceNegatedIsEmpty   = "REPLACE_NEGATED_IS_EMPTY"
	RedundantIf             = "REDUNDANT_IF"
	RedundantQualifierName  = "REMOVE_REDUNDANT_QUALIFIER_NAME"
)

var idiomCodes = map[string]bool{
	UselessElvis: true, RedundantStringTemplate: true, ReplaceGetOrSet: true, ReplaceSizeZeroCheck: true,
	ReplaceSizeCheck: true, ReplaceNegatedIsEmpty: true, RedundantIf: true, RedundantQualifierName: true,
}

// negated reports whether x is the operand of a !, in parentheses or
// not.
func negated(x psi.NodeRef) bool {
	p := x.Parent()
	for p.NumericKind() == psi.KindPARENTHESIZED {
		p = p.Parent()
	}
	return p.NumericKind() == psi.KindPREFIX_EXPRESSION && p.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() == psi.KindEXCL
}

// idiomFindings returns the idiom findings of snap, a file of v.
func idiomFindings(v *View, snap *psi.Snapshot) []finding {
	var out []finding
	add := func(r psi.Range, sev Severity, unnecessary bool, code, msg, title string, edit psi.Range, text string) {
		out = append(out, finding{Diagnostic{Range: r, Severity: sev, Code: code, Message: msg, Unnecessary: unnecessary}, title, edit, text})
	}
	info := func(n psi.NodeRef, code, msg, title, text string) {
		add(n.Range(), Information, false, code, msg, title, n.Range(), text)
	}
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindBINARY_EXPRESSION:
			op := n.ChildKind(psi.KindOPERATION_REFERENCE)
			left, right := firstNonTrivia(n), lastExpression(n)
			switch op.FirstChild().NumericKind() {
			case psi.KindELVIS:
				if t, ok := operandType(v, left); ok && !t.Nullable && !t.Platform && !t.Parameter && !genericCall(v, left) {
					add(psi.Range{Start: op.Range().Start, End: n.Range().End}, Warning, true, UselessElvis,
						"Elvis operator (?:) always returns the left operand of non-nullable type "+renderType(t),
						"Remove useless elvis operator", psi.Range{Start: left.Range().End, End: n.Range().End}, "")
				}
			case psi.KindEQEQ, psi.KindEXCLEQ, psi.KindGT:
				if recv, ok := sizeCheck(v, left, right); ok {
					if op.FirstChild().NumericKind() == psi.KindEQEQ {
						info(n, ReplaceSizeZeroCheck, "Size check can be replaced with 'isEmpty()'", "Replace size check with 'isEmpty()'", recv.Text()+".isEmpty()")
					} else {
						info(n, ReplaceSizeCheck, "Size check can be replaced with 'isNotEmpty()'", "Replace size check with 'isNotEmpty()'", recv.Text()+".isNotEmpty()")
					}
				}
			}
		case psi.KindPREFIX_EXPRESSION:
			if n.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() != psi.KindEXCL || negated(n) {
				break // in !!x.isEmpty(), the fix would leave !x.isNotEmpty()
			}
			if x := lastExpression(n); x.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION {
				call := lastExpression(x)
				name := call.ChildKind(psi.KindREFERENCE_EXPRESSION).Text()
				if call.NumericKind() == psi.KindCALL_EXPRESSION && (name == "isEmpty" || name == "isNotEmpty") &&
					compact(call.ChildKind(psi.KindVALUE_ARGUMENT_LIST).Text()) == "()" && !call.ChildKind(psi.KindLAMBDA_ARGUMENT).Valid() &&
					emptiable(v, firstNonTrivia(x)) {
					other := map[string]string{"isEmpty": "isNotEmpty", "isNotEmpty": "isEmpty"}[name]
					info(n, ReplaceNegatedIsEmpty, "Replace negated '"+name+"' with '"+other+"'", "Replace with '"+other+"()'",
						firstNonTrivia(x).Text()+"."+other+"()")
				}
			}
		case psi.KindSTRING_TEMPLATE:
			if x, ok := templateOfString(v, n); ok {
				add(n.Range(), Hint, true, RedundantStringTemplate, "Redundant string template", "Remove redundant string template", n.Range(), x.Text())
			}
		case psi.KindDOT_QUALIFIED_EXPRESSION:
			if text, set, ok := indexing(v, n); ok {
				what := "get"
				if set {
					what = "set"
				}
				info(n, ReplaceGetOrSet, "Explicit '"+what+"' call can be replaced with indexing", "Replace '"+what+"' call with indexing operator", text)
			}
			if redundantQualifiedCall(v, snap, n) {
				recv := firstNonTrivia(n)
				q := psi.Range{Start: recv.Range().Start, End: lastExpression(n).Range().Start}
				add(q, Hint, true, RedundantQualifierName, "Redundant qualifier name", "Remove redundant qualifier name", q, "")
			}
		case psi.KindUSER_TYPE:
			if q, ok := redundantQualifiedType(v, snap, n); ok {
				add(q, Hint, true, RedundantQualifierName, "Redundant qualifier name", "Remove redundant qualifier name", q, "")
			}
		case psi.KindIF:
			if text, ok := redundantIf(v, n); ok {
				info(n.ChildKind(psi.KindIF_KEYWORD), RedundantIf, "Redundant 'if' statement", "Remove redundant 'if' statement", "")
				out[len(out)-1].Edit, out[len(out)-1].Text = n.Range(), text
			}
		}
		return true
	})
	return out
}

// sizeCheck returns x of x.size == 0, x.size != 0, x.size > 0 or
// x.length's, when x is a collection, a map, an array or a char
// sequence, whose size or length it is.
func sizeCheck(v *View, left, right psi.NodeRef) (psi.NodeRef, bool) {
	if right.NumericKind() != psi.KindINTEGER_CONSTANT || right.Text() != "0" || left.NumericKind() != psi.KindDOT_QUALIFIED_EXPRESSION {
		return psi.NodeRef{}, false
	}
	recv, sel := firstNonTrivia(left), lastExpression(left)
	if sel.NumericKind() != psi.KindREFERENCE_EXPRESSION {
		return psi.NodeRef{}, false
	}
	t, ok := v.fact(recv)
	if !ok || t.Type == nil || t.Type.Nullable && !t.FlowNonNull {
		return psi.NodeRef{}, false
	}
	switch sel.Text() {
	case "size":
		if isA(*t.Type, "kotlin.collections.Collection", "kotlin.collections.Map") || arrayTypes[t.Type.Name] {
			return recv, true
		}
	case "length":
		if isA(*t.Type, "kotlin.CharSequence") {
			return recv, true
		}
	}
	return psi.NodeRef{}, false
}

// arrayTypes are the array classes, which have isEmpty and isNotEmpty.
var arrayTypes = map[string]bool{
	"kotlin.Array": true, "kotlin.IntArray": true, "kotlin.LongArray": true, "kotlin.ShortArray": true, "kotlin.ByteArray": true,
	"kotlin.CharArray": true, "kotlin.BooleanArray": true, "kotlin.FloatArray": true, "kotlin.DoubleArray": true,
}

// isA reports whether t is one of classes or a subtype of one.
func isA(t analysis.Type, classes ...string) bool {
	for _, c := range classes {
		if t.Name == c || slices.Contains(t.Supertypes, c) {
			return true
		}
	}
	return false
}

// emptiable reports whether x has isEmpty and isNotEmpty: a non-null
// collection, map, array or char sequence.
func emptiable(v *View, x psi.NodeRef) bool {
	f, ok := v.fact(x)
	if !ok || f.Type == nil || f.Type.Nullable && !f.FlowNonNull {
		return false
	}
	return isA(*f.Type, "kotlin.collections.Collection", "kotlin.collections.Map", "kotlin.CharSequence") || arrayTypes[f.Type.Name]
}

// templateOfString returns x of "$x" or "${x}", a template of nothing but
// a non-null string x that can stand for it.
func templateOfString(v *View, n psi.NodeRef) (psi.NodeRef, bool) {
	if strings.HasPrefix(n.Text(), `"""`) {
		return psi.NodeRef{}, false
	}
	var entry psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindOPEN_QUOTE, psi.KindCLOSING_QUOTE:
		case psi.KindSHORT_STRING_TEMPLATE_ENTRY, psi.KindLONG_STRING_TEMPLATE_ENTRY:
			if entry.Valid() {
				return psi.NodeRef{}, false
			}
			entry = c
		default:
			return psi.NodeRef{}, false
		}
	}
	x := lastExpression(entry)
	switch x.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindCALL_EXPRESSION, psi.KindTHIS_EXPRESSION, psi.KindARRAY_ACCESS_EXPRESSION:
	default:
		return psi.NodeRef{}, false
	}
	t, ok := operandType(v, x)
	return x, ok && t.Name == "kotlin.String" && !t.Nullable && !t.Platform
}

// indexing returns the indexing expression, a[i] or a[i] = v, of a call
// a.get(i) or a statement a.set(i, v) of an operator.
func indexing(v *View, n psi.NodeRef) (string, bool, bool) {
	recv, call := firstNonTrivia(n), lastExpression(n)
	if call.NumericKind() != psi.KindCALL_EXPRESSION || call.ChildKind(psi.KindTYPE_ARGUMENT_LIST).Valid() || call.ChildKind(psi.KindLAMBDA_ARGUMENT).Valid() {
		return "", false, false
	}
	switch recv.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindCALL_EXPRESSION, psi.KindTHIS_EXPRESSION,
		psi.KindARRAY_ACCESS_EXPRESSION, psi.KindPARENTHESIZED:
	default:
		return "", false, false
	}
	callee := call.ChildKind(psi.KindREFERENCE_EXPRESSION)
	name := callee.Text()
	if name != "get" && name != "set" {
		return "", false, false
	}
	var args []string
	for a := call.ChildKind(psi.KindVALUE_ARGUMENT_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() != psi.KindVALUE_ARGUMENT {
			continue
		}
		if a.ChildKind(psi.KindVALUE_ARGUMENT_NAME).Valid() || a.ChildKind(psi.KindMUL).Valid() {
			return "", false, false
		}
		args = append(args, strings.TrimSpace(a.Text()))
	}
	f, ok := v.fact(call)
	if !ok || f.Call == nil {
		f, ok = v.fact(callee)
	}
	if !ok || f.Call == nil || !strings.HasSuffix(f.Call.Name, "."+name) {
		return "", false, false
	}
	if !f.Call.Operator && !f.Call.FromJava {
		return "", false, false
	}
	if f.Call.FromJava {
		// A Java method of the name is an operator too, but indexing
		// reads well on a collection only, not on a reflected field.
		rf, ok := v.fact(recv)
		if !ok || rf.Type == nil || !isA(*rf.Type, "kotlin.collections.Collection", "kotlin.collections.Map", "java.util.Collection", "java.util.Map") {
			return "", false, false
		}
	}
	if name == "get" {
		return recv.Text() + "[" + strings.Join(args, ", ") + "]", false, len(args) > 0
	}
	if len(args) < 2 {
		return "", false, false
	}
	if nf, ok := v.fact(n); !ok || nf.UsedAsExpression || n.Parent().NumericKind() != psi.KindBLOCK {
		return "", false, false
	}
	return recv.Text() + "[" + strings.Join(args[:len(args)-1], ", ") + "] = " + args[len(args)-1], true, true
}

// redundantIf returns what if (c) true else false, or if (c) return true
// else return false, and their negations, say: c or !c, returned or not.
func redundantIf(v *View, n psi.NodeRef) (string, bool) {
	c := lastExpression(n.ChildKind(psi.KindCONDITION))
	th, el := branch(n.ChildKind(psi.KindTHEN)), branch(n.ChildKind(psi.KindELSE))
	if !c.Valid() || !th.Valid() || !el.Valid() || operand(n) {
		return "", false
	}
	if f, ok := v.fact(c); !ok || f.Type == nil || f.Type.Name != "kotlin.Boolean" || f.Type.Nullable {
		return "", false
	}
	value := func(x psi.NodeRef) (string, bool) {
		ret := false
		if x.NumericKind() == psi.KindRETURN {
			if x.ChildKind(psi.KindLABEL_QUALIFIER).Valid() {
				return "", false
			}
			ret, x = true, lastExpression(x)
		}
		if x.NumericKind() != psi.KindBOOLEAN_CONSTANT {
			return "", false
		}
		if ret {
			return "return " + x.Text(), true
		}
		return x.Text(), true
	}
	a, ok1 := value(th)
	b, ok2 := value(el)
	var text string
	switch {
	case !ok1 || !ok2:
		return "", false
	case a == "true" && b == "false", a == "return true" && b == "return false":
		text = c.Text()
	case a == "false" && b == "true", a == "return false" && b == "return true":
		text = negate(v, c)
	default:
		return "", false
	}
	if strings.HasPrefix(a, "return") {
		return "return " + text, true
	}
	return text, true
}

// branch returns the expression of an if's branch: its only statement if
// it is a block.
func branch(b psi.NodeRef) psi.NodeRef {
	x := lastExpression(b)
	if x.NumericKind() != psi.KindBLOCK {
		return x
	}
	var only psi.NodeRef
	for s := x.FirstChild(); s.Valid(); s = s.NextSibling() {
		if s.IsLeaf() || trivia(s) {
			if s.NumericKind() == psi.KindEOL_COMMENT || s.NumericKind() == psi.KindBLOCK_COMMENT {
				return psi.NodeRef{}
			}
			continue
		}
		if only.Valid() {
			return psi.NodeRef{}
		}
		only = s
	}
	return only
}

// operand reports whether n is an operator's operand, where another
// expression in its place may need parentheses.
func operand(n psi.NodeRef) bool {
	switch n.Parent().NumericKind() {
	case psi.KindBINARY_EXPRESSION, psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION,
		psi.KindSAFE_ACCESS_EXPRESSION, psi.KindBINARY_WITH_TYPE, psi.KindIS_EXPRESSION, psi.KindPARENTHESIZED:
		return true
	}
	return false
}

// genericCall reports whether x is a call, or a qualified one, of a
// generic function without type arguments, whose result type the
// expected type may have inferred.
func genericCall(v *View, x psi.NodeRef) bool {
	for {
		switch x.NumericKind() {
		case psi.KindPARENTHESIZED, psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
			x = lastExpression(x)
		case psi.KindCALL_EXPRESSION:
			if x.ChildKind(psi.KindTYPE_ARGUMENT_LIST).Valid() {
				return false
			}
			f, ok := v.fact(x)
			return !ok || f.Call == nil || f.Call.TypeParameters > 0
		default:
			return false
		}
	}
}

// negate returns Boolean expression x negated: with the opposite
// operator of a comparison, an is or an in, without the ! of a negation,
// or else with a !. Orderings of floating point numbers are kept and
// negated, as NaN compares false either way.
func negate(v *View, x psi.NodeRef) string {
	for x.NumericKind() == psi.KindPARENTHESIZED && lastExpression(x).Valid() {
		x = lastExpression(x)
	}
	switch x.NumericKind() {
	case psi.KindBOOLEAN_CONSTANT:
		if x.Text() == "true" {
			return "false"
		}
		return "true"
	case psi.KindPREFIX_EXPRESSION:
		if x.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() == psi.KindEXCL {
			inner := lastExpression(x)
			for inner.NumericKind() == psi.KindPARENTHESIZED && lastExpression(inner).Valid() {
				inner = lastExpression(inner)
			}
			return inner.Text()
		}
	case psi.KindIS_EXPRESSION:
		if op := x.ChildKind(psi.KindOPERATION_REFERENCE); op.Valid() {
			switch op.FirstChild().NumericKind() {
			case psi.KindIS_KEYWORD:
				return replaceRange(x, op.Range(), "!is")
			case psi.KindNOT_IS:
				return replaceRange(x, op.Range(), "is")
			}
		}
	case psi.KindBINARY_EXPRESSION:
		op := x.ChildKind(psi.KindOPERATION_REFERENCE)
		opposite := map[psi.Kind]string{
			psi.KindEQEQ: "!=", psi.KindEXCLEQ: "==", psi.KindEQEQEQ: "!==", psi.KindEXCLEQEQ: "===",
			psi.KindIN_KEYWORD: "!in", psi.KindNOT_IN: "in",
		}
		k := op.FirstChild().NumericKind()
		if s, ok := opposite[k]; ok {
			return replaceRange(x, op.Range(), s)
		}
		ordering := map[psi.Kind]string{psi.KindLT: ">=", psi.KindGT: "<=", psi.KindLTEQ: ">", psi.KindGTEQ: "<"}
		if s, ok := ordering[k]; ok && v != nil && !floating(v, firstNonTrivia(x)) && !floating(v, lastExpression(x)) {
			return replaceRange(x, op.Range(), s)
		}
	}
	if simpleOperand(x) {
		return "!" + x.Text()
	}
	return "!(" + x.Text() + ")"
}

// floating reports whether x may be a floating point number: its type is
// Double or Float, or unknown.
func floating(v *View, x psi.NodeRef) bool {
	f, ok := v.fact(x)
	if !ok || f.Type == nil {
		return true
	}
	switch f.Type.Name {
	case "kotlin.Int", "kotlin.Long", "kotlin.Short", "kotlin.Byte", "kotlin.Char", "kotlin.String",
		"kotlin.UInt", "kotlin.ULong", "kotlin.UShort", "kotlin.UByte":
		return false
	}
	return true
}

// replaceRange returns n's text with r replaced by text.
func replaceRange(n psi.NodeRef, r psi.Range, text string) string {
	s, start := n.Text(), n.Range().Start
	return s[:r.Start-start] + text + s[r.End-start:]
}

// redundantQualifiedType returns the range of the package qualifier of a
// type that names a top-level class its simple name names too, with the
// dot after it.
func redundantQualifiedType(v *View, snap *psi.Snapshot, n psi.NodeRef) (psi.Range, bool) {
	q := n.ChildKind(psi.KindUSER_TYPE)
	if !q.Valid() || strings.ContainsAny(q.Text(), "<?( \t\n/") {
		return psi.Range{}, false
	}
	name := n.ChildKind(psi.KindREFERENCE_EXPRESSION)
	if !name.Valid() || n.Parent().NumericKind() == psi.KindUSER_TYPE {
		return psi.Range{}, false
	}
	if !redundantQualifier(v, snap, q.Text(), name.Text()) {
		return psi.Range{}, false
	}
	return psi.Range{Start: q.Range().Start, End: name.Range().Start}, true
}

// redundantQualifiedCall reports whether n, pkg.C(...) or pkg.C, calls or
// names a top-level class C of package pkg that C names too.
func redundantQualifiedCall(v *View, snap *psi.Snapshot, n psi.NodeRef) bool {
	recv, sel := firstNonTrivia(n), lastExpression(n)
	f, ok := v.fact(recv)
	if !ok || f.Package == "" || compact(recv.Text()) != f.Package {
		return false
	}
	name := sel
	if sel.NumericKind() == psi.KindCALL_EXPRESSION {
		name = sel.ChildKind(psi.KindREFERENCE_EXPRESSION)
	}
	if name.NumericKind() != psi.KindREFERENCE_EXPRESSION || name.Text() == "" || name.Text()[0] < 'A' || name.Text()[0] > 'Z' {
		return false
	}
	if sf, ok := v.fact(name); !ok || !(sf.Classifier || sf.Call != nil && sf.Call.Name == f.Package+"."+name.Text()+".<init>") {
		if cf, ok := v.fact(sel); !ok || cf.Call == nil || cf.Call.Name != f.Package+"."+name.Text()+".<init>" {
			return false
		}
	}
	if !redundantQualifier(v, snap, f.Package, name.Text()) {
		return false
	}
	return true
}

// redundantQualifier reports whether class pkg.cls, top-level, is what
// cls names in snap: imported, of snap's package or of the default
// imports, and no other class of the sources, nor a type parameter of
// snap, has that simple name.
func redundantQualifier(v *View, snap *psi.Snapshot, pkg, cls string) bool {
	if pkg == "" || pkg == "java.lang" || pkg[0] < 'a' || pkg[0] > 'z' {
		return false
	}
	for _, s := range strings.Split(pkg, ".") {
		if s == "" || s[0] < 'a' || s[0] > 'z' {
			return false // a class qualifies the name
		}
	}
	fq := pkg + "." + cls
	sh := fixer{snap: snap}.shortener()
	if imported, ok := sh.visible[cls]; ok && imported != fq ||
		!ok && pkg != sh.pkg && !defaultPackages[pkg] {
		return false
	}
	v.classIndex()
	for name := range v.classes {
		if name != fq && (name == cls || strings.HasSuffix(name, "."+cls)) {
			return false
		}
	}
	for name := range v.aliases {
		if name == cls || strings.HasSuffix(name, "."+cls) {
			return false
		}
	}
	shadowed := false
	psi.WalkNodes(snap.RootNode(), func(m psi.NodeRef) bool {
		if m.NumericKind() == psi.KindTYPE_PARAMETER && m.Name() == cls {
			shadowed = true
		}
		return !shadowed
	})
	return !shadowed
}
