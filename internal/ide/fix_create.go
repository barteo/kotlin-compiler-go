package ide

import (
	"strings"
	"unicode"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Quick fixes that create what an unresolved name names, from its use: a
// function from a call, with parameters of its arguments' types and the
// result type the call's place expects; a property or local variable of
// the type a plain name's place expects; a class from a constructor call
// or a type. The bodies and initializers are TODO()s.

const unresolvedType = "unresolved or unsupported type: "

const todoBody = `TODO("Not yet implemented")`

// createFromUsage returns the fixes that create name, which a reference
// at the diagnostic doesn't resolve.
func (f fixer) createFromUsage(name string) []Fix {
	vs := f.viewSnap()
	if vs == nil || !identifier(name) {
		return nil
	}
	ref := vs.NodeAt(f.d.Range.Start).Parent()
	if ref.NumericKind() != psi.KindREFERENCE_EXPRESSION || psi.Unquote(ref.Text()) != name {
		return nil
	}
	pos, call := ref, psi.NodeRef{}
	if p := ref.Parent(); p.NumericKind() == psi.KindCALL_EXPRESSION && firstNonTrivia(p) == ref {
		pos, call = p, p
	}
	// The class of an explicit receiver, if the name is a selector.
	var owner psi.NodeRef
	var ownerSnap *psi.Snapshot
	outer := pos // the expression whose value is the name's
	switch q := pos.Parent(); q.NumericKind() {
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
		recv := firstNonTrivia(q)
		if recv == pos {
			return nil // a qualifier: a package, a class or an object
		}
		owner, ownerSnap = f.receiverClass(recv, q.NumericKind() == psi.KindSAFE_ACCESS_EXPRESSION)
		if !owner.Valid() {
			return nil
		}
		outer = q
	case psi.KindUSER_TYPE, psi.KindCALLABLE_REFERENCE_EXPRESSION, psi.KindVALUE_ARGUMENT_NAME, psi.KindIMPORT_DIRECTIVE,
		psi.KindPACKAGE_DIRECTIVE, psi.KindLABEL_QUALIFIER:
		return nil
	}
	ret, known := f.expectedType(outer)
	var out []Fix
	if call.Valid() {
		if call.ChildKind(psi.KindTYPE_ARGUMENT_LIST).Valid() {
			return nil
		}
		target := f.snap
		if owner.Valid() {
			target = ownerSnap
		}
		sh := fixer{snap: target}.shortener()
		params, ok := f.callParameters(call, sh)
		if !ok {
			return nil
		}
		if known {
			retText := ret
			if retText != "" {
				retText = sh.short(retText)
			}
			out = append(out, f.createFunction(name, params, retText, owner, ownerSnap, pos, sh)...)
		}
		if !owner.Valid() && unicode.IsUpper(rune(name[0])) && (!known || ret == "" || simpleName(ret) == name) {
			out = append(out, f.createConstructed(name, params, sh)...)
		}
		return out
	}
	if !known || ret == "" || unicode.IsUpper(rune(name[0])) || assigned(outer) {
		return nil
	}
	if owner.Valid() {
		sh := fixer{snap: ownerSnap}.shortener()
		decl := "val " + name + ": " + sh.short(ret) + " = TODO()"
		e, ok := memberInsert(owner, decl)
		if !ok {
			return nil
		}
		edits := append([]FixEdit{{Loc{ownerSnap, e.Range}, e.Text}}, sh.imports()...)
		return []Fix{{Title: "Create property '" + owner.Name() + "." + name + "'", Edits: edits}}
	}
	sh := f.shortener()
	typ := sh.short(ret)
	if st := statement(pos); st.Valid() {
		at := st.Range().Start
		text := "val " + name + ": " + typ + " = TODO()\n" + lineIndent(f.snap.Source(), at)
		out = append(out, Fix{Title: "Create local variable '" + name + "'", Edits: append([]FixEdit{f.edit(psi.Range{Start: at, End: at}, text)}, sh.imports()...)})
	}
	if top := topDeclaration(pos); top.Valid() {
		at := top.Range().End
		text := "\n\nval " + name + ": " + typ + " = TODO()"
		out = append(out, Fix{Title: "Create property '" + name + "'", Edits: append([]FixEdit{f.edit(psi.Range{Start: at, End: at}, text)}, sh.imports()...)})
	}
	return out
}

// assigned reports whether x is the target of an assignment.
func assigned(x psi.NodeRef) bool {
	p := x.Parent()
	return p.NumericKind() == psi.KindBINARY_EXPRESSION && assignment(p) && firstNonTrivia(p) == x
}

// receiverClass returns the class of the sources of a receiver's type,
// non-null unless safe.
func (f fixer) receiverClass(recv psi.NodeRef, safe bool) (psi.NodeRef, *psi.Snapshot) {
	if fact, ok := f.v.fact(recv); ok && fact.Classifier {
		return psi.NodeRef{}, nil
	}
	t, ok := f.exprType(recv)
	if !ok || t.Nullable && !safe {
		return psi.NodeRef{}, nil
	}
	f.v.classIndex()
	el, ok := f.v.classes[t.Name]
	if !ok {
		return psi.NodeRef{}, nil
	}
	n := f.v.node(el)
	if n.NumericKind() != psi.KindCLASS && n.NumericKind() != psi.KindOBJECT_DECLARATION || !n.ChildKind(psi.KindCLASS_BODY).Valid() {
		return psi.NodeRef{}, nil
	}
	return n, f.v.snaps[el.Snapshot]
}

// topDeclaration returns the top-level declaration that holds n.
func topDeclaration(n psi.NodeRef) psi.NodeRef {
	for ; n.Valid(); n = n.Parent() {
		if n.Parent().NumericKind() == psi.KindFILE {
			switch n.NumericKind() {
			case psi.KindFUN, psi.KindPROPERTY, psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS:
				return n
			}
			return psi.NodeRef{}
		}
	}
	return psi.NodeRef{}
}

// expectedType returns the qualified type x's place expects of it: a
// property's declared type, a function's declared result type for its
// expression body or a return, a parameter's type for an argument of a
// resolved call, Boolean for a condition, the assigned variable's type,
// or "" for a statement whose value isn't used. known is false where
// the place doesn't say.
func (f fixer) expectedType(x psi.NodeRef) (string, bool) {
	p := x.Parent()
	switch p.NumericKind() {
	case psi.KindPROPERTY:
		if tr := p.TypeReference(); tr.Valid() && lastExpression(p) == x && p.ChildKind(psi.KindEQ).Valid() {
			return f.declaredType(tr)
		}
	case psi.KindRETURN:
		if p.ChildKind(psi.KindLABEL_QUALIFIER).Valid() {
			return "", false
		}
		for fn := p.Parent(); fn.Valid(); fn = fn.Parent() {
			switch fn.NumericKind() {
			case psi.KindFUNCTION_LITERAL, psi.KindPROPERTY_ACCESSOR:
				return "", false
			case psi.KindFUN:
				if tr := fn.TypeReference(); tr.Valid() {
					return f.declaredType(tr)
				}
				if fn.Body().NumericKind() == psi.KindBLOCK {
					return "", true
				}
				return "", false
			}
		}
	case psi.KindFUN:
		if tr := p.TypeReference(); tr.Valid() && p.Body() == x {
			return f.declaredType(tr)
		}
	case psi.KindBLOCK:
		switch p.Parent().NumericKind() {
		case psi.KindFUNCTION_LITERAL:
			if lastExpression(p) == x {
				return "", false // the lambda's result
			}
			return "", true
		case psi.KindFUN, psi.KindBODY, psi.KindTHEN, psi.KindELSE, psi.KindTRY, psi.KindFINALLY, psi.KindCATCH, psi.KindCLASS_INITIALIZER,
			psi.KindSECONDARY_CONSTRUCTOR:
			if p.Parent().NumericKind() == psi.KindTHEN || p.Parent().NumericKind() == psi.KindELSE {
				if f.used(p.Parent().Parent()) && lastExpression(p) == x {
					return "", false
				}
			}
			return "", true
		}
	case psi.KindCONDITION:
		return "kotlin.Boolean", true
	case psi.KindVALUE_ARGUMENT:
		list, call := p.Parent(), p.Parent().Parent()
		if list.NumericKind() != psi.KindVALUE_ARGUMENT_LIST || call.NumericKind() != psi.KindCALL_EXPRESSION {
			return "", false
		}
		fact, ok := f.v.fact(call)
		if !ok || fact.Call == nil {
			return "", false
		}
		i := 0
		for a := list.FirstChild(); a.Valid() && a != p; a = a.NextSibling() {
			if a.NumericKind() == psi.KindVALUE_ARGUMENT {
				i++
			}
		}
		c := fact.Call
		if i >= len(c.ArgumentMapping) || c.ArgumentMapping[i] < 0 || c.ArgumentMapping[i] >= len(c.Parameters) {
			return "", false
		}
		t := c.Parameters[c.ArgumentMapping[i]]
		if hasParameter(t) || t.Name == "" {
			return "", false
		}
		return qualifiedType(t), true
	case psi.KindBINARY_EXPRESSION:
		if p.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() == psi.KindEQ && lastExpression(p) == x {
			if fact, ok := f.v.fact(firstNonTrivia(p)); ok && fact.Type != nil && fact.Type.Name != "" && !hasParameter(*fact.Type) {
				return qualifiedType(*fact.Type), true
			}
		}
	}
	return "", false
}

// used reports whether expression x's value is used, as far as the view
// knows.
func (f fixer) used(x psi.NodeRef) bool {
	fact, ok := f.v.fact(x)
	return !ok || fact.UsedAsExpression
}

// declaredType returns a declared type's qualified text, by its fact if
// it has one, or as written; "" for Unit.
func (f fixer) declaredType(tr psi.NodeRef) (string, bool) {
	t := compact(tr.Text())
	if t == "Unit" || t == "kotlin.Unit" {
		return "", true
	}
	if strings.ContainsAny(t, "@") {
		return "", false
	}
	return t, true
}

// createParameter is a parameter of a function or class created from a
// call.
type createParameter struct{ name, typ string }

// callParameters returns the parameters a call's arguments give a
// function created for it: named as the named arguments or as
// suggestName would name a variable of the argument, unique, of the
// arguments' types written for sh's file. ok is false when an argument
// has no type that can be written, or is spread.
func (f fixer) callParameters(call psi.NodeRef, sh *shortener) ([]createParameter, bool) {
	var out []createParameter
	seen := map[string]int{}
	add := func(x psi.NodeRef, named string) bool {
		t, ok := f.exprType(x)
		if !ok || t.Name == "kotlin.Nothing" || hasParameter(t) {
			return false
		}
		name := named
		if name == "" {
			name = suggestName(x, t)
		}
		if seen[name]++; seen[name] > 1 {
			name += string(rune('0' + seen[name]))
		}
		out = append(out, createParameter{name, sh.short(qualifiedType(t))})
		return true
	}
	for a := call.ChildKind(psi.KindVALUE_ARGUMENT_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() != psi.KindVALUE_ARGUMENT {
			continue
		}
		if a.ChildKind(psi.KindMUL).Valid() {
			return nil, false
		}
		named := ""
		if n := a.ChildKind(psi.KindVALUE_ARGUMENT_NAME); n.Valid() {
			named = psi.Unquote(strings.TrimSpace(n.Text()))
		}
		if !add(lastExpression(a), named) {
			return nil, false
		}
	}
	if l := call.ChildKind(psi.KindLAMBDA_ARGUMENT); l.Valid() {
		if !add(lastExpression(l), "") {
			return nil, false
		}
	}
	return out, true
}

// parameterList writes parameters as a declaration's, with val before
// each if property.
func parameterList(params []createParameter, property bool) string {
	var b strings.Builder
	for i, p := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		if property {
			b.WriteString("val ")
		}
		b.WriteString(p.name + ": " + p.typ)
	}
	return b.String()
}

// createFunction returns the fixes that create a function for a call:
// top-level after the declaration that holds the call; a member of the
// class the call is in, without a receiver; or a member of owner, the
// class of the call's explicit receiver.
func (f fixer) createFunction(name string, params []createParameter, ret string, owner psi.NodeRef, ownerSnap *psi.Snapshot, pos psi.NodeRef, sh *shortener) []Fix {
	sig := name + "(" + parameterList(params, false) + ")"
	if ret != "" {
		sig += ": " + ret
	}
	decl := "fun " + sig + " {\n    " + todoBody + "\n}"
	if owner.Valid() {
		e, ok := memberInsert(owner, decl)
		if !ok {
			return nil
		}
		edits := append([]FixEdit{{Loc{ownerSnap, e.Range}, e.Text}}, sh.imports()...)
		return []Fix{{Title: "Create member function '" + owner.Name() + "." + name + "'", Edits: edits}}
	}
	var out []Fix
	if top := topDeclaration(pos); top.Valid() {
		at := top.Range().End
		edits := append([]FixEdit{f.edit(psi.Range{Start: at, End: at}, "\n\n"+reunit(decl, indentUnit(f.snap.Source())))}, sh.imports()...)
		out = append(out, Fix{Title: "Create function '" + name + "'", Edits: edits})
	}
	if cls := enclosingClass(pos); cls.Valid() {
		if e, ok := memberInsert(cls, "private "+decl); ok {
			edits := append([]FixEdit{f.edit(e.Range, e.Text)}, sh.imports()...)
			out = append(out, Fix{Title: "Create member function '" + cls.Name() + "." + name + "'", Edits: edits})
		}
	}
	return out
}

// enclosingClass returns the named class or object whose member holds n,
// if n isn't in a lambda, a local function or a nested class's member.
func enclosingClass(n psi.NodeRef) psi.NodeRef {
	for p := n.Parent(); p.Valid(); p = p.Parent() {
		switch p.NumericKind() {
		case psi.KindCLASS_BODY:
			c := p.Parent()
			if (c.NumericKind() == psi.KindCLASS || c.NumericKind() == psi.KindOBJECT_DECLARATION) && c.NameIdentifier().Valid() &&
				!c.ChildKind(psi.KindINTERFACE_KEYWORD).Valid() {
				return c
			}
			return psi.NodeRef{}
		case psi.KindFUNCTION_LITERAL, psi.KindOBJECT_LITERAL, psi.KindFILE:
			return psi.NodeRef{}
		case psi.KindFUN:
			if p.Parent().NumericKind() != psi.KindCLASS_BODY {
				return psi.NodeRef{} // a top-level or local function
			}
		}
	}
	return psi.NodeRef{}
}

// memberInsert returns the edit that adds decl, of lines indented from
// four spaces, as the last member of cls.
func memberInsert(cls psi.NodeRef, decl string) (TextEdit, bool) {
	body := cls.ChildKind(psi.KindCLASS_BODY)
	rb := body.ChildKind(psi.KindRBRACE)
	if !rb.Valid() {
		return TextEdit{}, false
	}
	src := cls.ContainingFile().Source()
	classInd := lineIndent(src, cls.Range().Start)
	unit := indentUnit(src)
	ind := classInd + unit
	lines := strings.Split(reunit(decl, unit), "\n")
	for i := range lines {
		lines[i] = ind + lines[i]
	}
	text := strings.Join(lines, "\n") + "\n"
	lineStart := strings.LastIndexByte(src[:rb.Range().Start], '\n') + 1
	if strings.TrimSpace(src[lineStart:rb.Range().Start]) == "" && lineStart > body.Range().Start {
		if !emptyBody(body) {
			text = "\n" + text
		}
		return TextEdit{Range: psi.Range{Start: lineStart, End: lineStart}, Text: text}, true
	}
	at := rb.Range().Start
	for at > body.Range().Start+1 && (src[at-1] == ' ' || src[at-1] == '\t') {
		at--
	}
	return TextEdit{Range: psi.Range{Start: at, End: rb.Range().Start}, Text: "\n" + text + classInd}, true
}

// createConstructed returns the fix that creates a class for a
// constructor call, at the end of the file, with properties of the
// call's arguments.
func (f fixer) createConstructed(name string, params []createParameter, sh *shortener) []Fix {
	decl := "class " + name
	if len(params) > 0 {
		decl += "(" + parameterList(params, true) + ")"
	}
	return []Fix{{Title: "Create class '" + name + "'", Edits: append([]FixEdit{f.atEnd(decl)}, sh.imports()...)}}
}

// atEnd returns the edit that adds decl at the end of the file, after a
// blank line.
func (f fixer) atEnd(decl string) FixEdit {
	src := f.snap.Source()
	end := len(src)
	text := "\n" + decl + "\n"
	switch {
	case src == "":
		text = decl + "\n"
	case !strings.HasSuffix(src, "\n"):
		text = "\n\n" + decl + "\n"
	case strings.HasSuffix(src, "\n\n"):
		text = decl + "\n"
	}
	return f.edit(psi.Range{Start: end, End: end}, text)
}

// createClass returns the fix that creates the class an unresolved type
// names, at the end of the file: open for a superclass, an interface for
// a supertype without a constructor call.
func (f fixer) createClass(name string) []Fix {
	if !identifier(name) || !unicode.IsUpper(rune(name[0])) {
		return nil
	}
	ref := f.snap.NodeAt(f.d.Range.Start).Parent()
	ut := ref.Parent()
	if ref.NumericKind() != psi.KindREFERENCE_EXPRESSION || ut.NumericKind() != psi.KindUSER_TYPE || ut.ChildKind(psi.KindTYPE_ARGUMENT_LIST).Valid() {
		return nil
	}
	kind, what := "class ", "class"
	tr := ut.Parent()
	switch p := tr.Parent(); p.NumericKind() {
	case psi.KindSUPER_TYPE_ENTRY:
		kind, what = "interface ", "interface"
	case psi.KindCONSTRUCTOR_CALLEE:
		entry := p.Parent()
		if entry.NumericKind() != psi.KindSUPER_TYPE_CALL_ENTRY || len(argumentsOf(entry)) > 0 {
			return nil // an annotation, or a superclass with arguments
		}
		kind = "open class "
	case psi.KindDELEGATED_SUPER_TYPE_ENTRY:
		kind, what = "interface ", "interface"
	}
	return []Fix{{Title: "Create " + what + " '" + name + "'", Edits: []FixEdit{f.atEnd(kind + name)}}}
}

// argumentsOf returns the value arguments of a call or a superclass's
// constructor call.
func argumentsOf(n psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	for a := n.ChildKind(psi.KindVALUE_ARGUMENT_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() == psi.KindVALUE_ARGUMENT {
			out = append(out, a)
		}
	}
	return out
}

// exprType returns the type of expression x: its fact's, or, as the
// facts of a function stop at its first error, a literal's, or the
// declared type of the variable or parameter a name names in its scope.
func (f fixer) exprType(x psi.NodeRef) (analysis.Type, bool) {
	if fact, ok := f.v.fact(x); ok && fact.Type != nil && fact.Type.Name != "" {
		t := *fact.Type
		if fact.FlowNonNull {
			t.Nullable = false
		}
		return t, true
	}
	text := x.Text()
	switch x.NumericKind() {
	case psi.KindINTEGER_CONSTANT:
		if strings.HasSuffix(text, "L") {
			return analysis.Type{Name: "kotlin.Long"}, true
		}
		return analysis.Type{Name: "kotlin.Int"}, true
	case psi.KindFLOAT_CONSTANT:
		if strings.HasSuffix(text, "f") || strings.HasSuffix(text, "F") {
			return analysis.Type{Name: "kotlin.Float"}, true
		}
		return analysis.Type{Name: "kotlin.Double"}, true
	case psi.KindBOOLEAN_CONSTANT:
		return analysis.Type{Name: "kotlin.Boolean"}, true
	case psi.KindCHARACTER_CONSTANT:
		return analysis.Type{Name: "kotlin.Char"}, true
	case psi.KindSTRING_TEMPLATE:
		return analysis.Type{Name: "kotlin.String"}, true
	case psi.KindREFERENCE_EXPRESSION:
		if d := scopeDeclaration(x); d.Valid() {
			if fact, ok := f.v.fact(d); ok {
				if s, ok := f.v.symbol(fact.Symbol); ok && s.Type.Name != "" {
					return s.Type, true
				}
			}
		}
	}
	return analysis.Type{}, false
}

// scopeDeclaration returns the local variable, parameter or property a
// name names, looked up by its text in the scopes around it: blocks'
// variables before it, functions' and lambdas' parameters, a for loop's
// variable, a class's properties, the file's.
func scopeDeclaration(ref psi.NodeRef) psi.NodeRef {
	name := psi.Unquote(ref.Text())
	named := func(d psi.NodeRef) bool { return d.Valid() && d.Name() == name }
	for n := ref.Parent(); n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindBLOCK, psi.KindFILE, psi.KindCLASS_BODY:
			var found psi.NodeRef
			for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
				if n.NumericKind() == psi.KindBLOCK && c.Range().Start >= ref.Range().Start {
					break
				}
				if c.NumericKind() == psi.KindPROPERTY && named(c) {
					found = c
				}
			}
			if found.Valid() {
				return found
			}
		case psi.KindFUN, psi.KindSECONDARY_CONSTRUCTOR:
			for _, p := range n.Parameters() {
				if named(p) {
					return p
				}
			}
		case psi.KindFUNCTION_LITERAL:
			for p := n.ChildKind(psi.KindVALUE_PARAMETER_LIST).FirstChild(); p.Valid(); p = p.NextSibling() {
				if p.NumericKind() == psi.KindVALUE_PARAMETER && named(p) {
					return p
				}
			}
		case psi.KindFOR:
			if p := n.ChildKind(psi.KindVALUE_PARAMETER); named(p) {
				return p
			}
		case psi.KindCLASS:
			for _, p := range n.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).Parameters() {
				if named(p) {
					return p
				}
			}
		}
	}
	return psi.NodeRef{}
}
