package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// unitAlias reports whether name is a typealias of Unit, typealias
// MyUnit = Unit, which as an expression is the Unit object, as an alias
// of any other object is that object.
func (c *Checker) unitAlias(name string) bool {
	a := c.findClassifier(name).alias
	if a == nil {
		return false
	}
	c.resolveAlias(a)
	return a.Type.K == types.Unit && !a.Type.N
}

// extFuncAnnotated returns t, a function type written with its class
// name and annotated @ExtensionFunctionType, as a function type with a
// receiver, its first parameter: @ExtensionFunctionType Function1<T,
// Unit> is T.() -> Unit.
//
// The annotation, also through a typealias of it, on a type other than a
// function type with a parameter is an error, as kotlinc's
// WRONG_EXTENSION_FUNCTION_TYPE.
func (c *Checker) extFuncAnnotated(ref *syntax.TypeRef, t types.Type) types.Type {
	if len(ref.Annotations) == 0 || t.K == types.Invalid {
		return t
	}
	a := c.findAnnotation(ref.Annotations, "", "ExtensionFunctionType", "kotlin/ExtensionFunctionType")
	switch {
	case a == nil:
	case !t.IsFunc() || t.A.Len() < 2:
		c.errorf(a.P, "'@ExtensionFunctionType' is only allowed on function types with at least one parameter")
	case ref.Ret == nil && !t.IsExtFunc():
		return t.WithReceiver()
	}
	return t
}

// joinIsNarrowed returns the type a when subject of type subj has in a
// branch of is checks that joinIs can't join, is String, is Any, as K2
// has it: the join of the types each check narrows it to, String for a
// String? subject. It reports false if that is no narrower than subj.
func (c *Checker) joinIsNarrowed(subj types.Type, conds []syntax.WhenCond) (types.Type, bool) {
	var t types.Type
	for i, cond := range conds {
		if !cond.Is || cond.Target.K == types.Invalid || cond.Target.HasVars() {
			return types.InvalidType, false
		}
		n := c.isNarrowed(subj, cond.Target)
		if i == 0 {
			t = n
			continue
		}
		j, ok := c.join(t, n)
		if !ok || j.K == types.Invalid {
			return types.InvalidType, false
		}
		t = j
	}
	if t.K == types.Invalid || c.subtype(subj, t) || !c.subtype(t, subj) {
		return types.InvalidType, false
	}
	return t, true
}

// delegationNarrows returns the smart casts of a constructor's parameters
// that its delegation call's arguments leave, this(x!!, ...), in the
// scope of the parameters: the constructor's body has them.
func delegationNarrows(s *scope) []narrow {
	for s.parent != nil {
		s = s.parent
	}
	var out []narrow
	for _, n := range s.narrows {
		if n.recv == nil && slices.Contains(s.vars, n.sym) {
			out = append(out, n)
		}
	}
	return out
}

// objectNamed returns the object that x names as a value, Obj or a
// class's companion, C, or nil.
func objectNamed(x syntax.Expr) *types.ClassSym {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Object
	case *syntax.Member:
		return x.Object
	}
	return nil
}

// objectSym returns the symbol that smart casts of an object's stable
// properties are keyed by: Obj.y and example.Obj.y are the same.
func (c *Checker) objectSym(obj *types.ClassSym) *types.VarSym {
	if v := c.objectSyms[obj]; v != nil {
		return v
	}
	if c.objectSyms == nil {
		c.objectSyms = map[*types.ClassSym]*types.VarSym{}
	}
	v := &types.VarSym{Name: obj.Name, Type: types.ClassType(obj), Kind: types.Global}
	c.objectSyms[obj] = v
	return v
}

// entrySym returns the symbol that smart casts of the stable properties
// of an enum entry are keyed by, or nil if x doesn't name one:
// Message.HELLO.text, and HELLO.text where the entry is imported.
func (c *Checker) entrySym(x syntax.Expr) *types.VarSym {
	var v *types.VarSym
	switch x := x.(type) {
	case *syntax.Ident:
		if x.Member != nil {
			return c.entrySym(x.Member)
		}
		if x.Sym != nil && x.Sym.Kind == types.Static {
			v = x.Sym
		}
	case *syntax.Member:
		v = x.Field
	}
	if v == nil || v.Owner == nil || !isEnumEntry(v.Owner, v.Name) {
		return nil
	}
	key := v.Owner.Name + "." + v.Name
	if e := c.entrySyms[key]; e != nil {
		return e
	}
	if c.entrySyms == nil {
		c.entrySyms = map[string]*types.VarSym{}
	}
	e := &types.VarSym{Name: key, Type: v.Type, Kind: types.Global}
	c.entrySyms[key] = e
	return e
}

// delegateSetter reports whether d is the setter of a delegated property,
// which calls the delegate's setValue: it may return anything.
func delegateSetter(d *syntax.FunDecl) bool {
	if d.Name != "set" || d.Property == "" || d.Body != nil {
		return false
	}
	call, ok := d.ExprBody.(*syntax.Call)
	if !ok {
		return false
	}
	m, ok := call.Fun.(*syntax.Member)
	if !ok || m.Name != "setValue" || len(call.Args) != 3 {
		return false
	}
	switch x := m.X.(type) {
	case *syntax.Ident:
		return strings.Contains(x.Name, "$delegate")
	case *syntax.Member:
		return strings.Contains(x.Name, "$delegate")
	}
	return false
}

// inferredJump returns Nothing if branch b, of type t, ends in a call
// that the branches inferred together, after it was checked, to return
// Nothing, as myRun { return 24 } of inline fun <T> myRun(f: () -> T): T,
// and t otherwise.
func inferredJump(t types.Type, b *syntax.Block) types.Type {
	if t.K == types.Nothing || b == nil || len(b.Stmts) == 0 {
		return t
	}
	if es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt); ok {
		if _, call := es.X.(*syntax.Call); call && es.X.Type().K == types.Nothing && !es.X.Type().N {
			return types.NothingType
		}
	}
	return t
}

// aliasOfString reports whether name is a typealias of kotlin.String,
// whose constructor call S() is String(), a new empty string.
func (c *Checker) aliasOfString(name string) bool {
	a := c.findClassifier(name).alias
	return a != nil && c.stringAlias(a)
}

func (c *Checker) stringAlias(a *types.AliasSym) bool {
	if len(a.TypeParams) > 0 {
		return false
	}
	c.resolveAlias(a)
	return a.Type == types.StringType
}

// qualifiedAlias returns the typealias a.b.S or Class.S names, if any.
func (c *Checker) qualifiedAlias(m *syntax.Member) *types.AliasSym {
	if name := c.qualifiedName(m); name != "" {
		if a := c.lookupAlias(name); a != nil {
			return a
		}
	}
	if cls := c.classRef(m.X); cls != nil && cls.Aliases != nil {
		return cls.Aliases[m.Name]
	}
	return nil
}

// projectedAssignOp returns the type that x op= v on a property of a
// projected receiver works on: the type read, not the type written,
// Nothing for t.v of a Tr<out C>, when it has an op-assign operator,
// t.v += c calling C.plusAssign(c), which assigns nothing.
func (c *Checker) projectedAssignOp(s *syntax.Assign, m *syntax.Member, write types.Type) types.Type {
	read := m.T
	if !never(write) || read.K != types.Class || never(read) {
		return write
	}
	probe := &syntax.Call{Args: []syntax.Expr{s.Value}}
	if !c.hasFun(read.NonNull(), opNames[s.Op], probe) {
		return write
	}
	return read
}

// renamedOperator reports whether operator function f is called as
// operator name through an import that renames it: a - b for import
// a.plus as minus, which isn't an operator minus.
func renamedOperator(f *types.FunSym, name string) bool {
	return f.Operator && f.Builtin == 0 && f.Name != name && f.Name != "<init>"
}

// namesCallee reports whether name names a class or a top-level function
// that a call of the name may be instead of an invoked property.
func (c *Checker) namesCallee(name string) bool {
	return c.findClass(name) != nil || len(c.topFuns(name)) > 0 || len(c.kotlinFuns(name, false)) > 0
}

// inferring reports whether property p's type is being inferred: its
// own, or its delegate's, whose initializer val lazy by lazy { 1 } calls
// the function lazy.
func inferring(p *types.VarSym) bool {
	if p.InProgress {
		return true
	}
	d, ok := p.Decl.(*syntax.PropDecl)
	return ok && d.Field != nil && d.Field.DelegateOf == d && d.Field.Sym != nil && d.Field.Sym.InProgress
}

// useSiteProjected reports whether receiver substitution recv projects an
// invariant type parameter that parameter type t mentions, A<out Any?> or A<*> of an interface A<T>:
// through it, a parameter of type @UnsafeVariance T takes Nothing, as
// for one without the annotation, which only declaration-site variance
// makes relevant (KT-57609).
func useSiteProjected(recv types.Subst, t types.Type) bool {
	for p, a := range recv {
		if p.Var == types.Inv && a.V != types.Inv && mentionsParam(t, p) {
			return true
		}
	}
	return false
}

// varargNonOperator reports whether f, a Java method of a binary
// operator's name, takes a vararg parameter, which makes it no operator:
// x + "" calls the extension plus(s: String), not a Java plus(String...).
func varargNonOperator(f *types.FunSym) bool {
	if len(f.Params) == 0 || !f.Params[len(f.Params)-1].Vararg {
		return false
	}
	switch f.Name {
	case "plus", "minus", "times", "div", "rem", "mod", "rangeTo", "rangeUntil", "contains", "compareTo",
		"plusAssign", "minusAssign", "timesAssign", "divAssign", "remAssign", "modAssign":
		return true
	}
	return false
}
