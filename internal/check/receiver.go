package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Lambdas with receivers.
//
// A lambda whose expected type is a function type with a receiver, A.() ->
// R, has an implicit receiver: its first parameter, a local named
// $this$label, as kotlinc names it, whose members and extensions can be
// used unqualified, and which this means in its body. Nested lambdas
// capture it as any other parameter.
//
// An unqualified name is looked up in the lambdas' receivers, innermost
// first, after locals and before the enclosing class's members and
// top-level declarations. A name a receiver has is rewritten to a member
// access or call on it: f(x) becomes this.f(x), with this the receiver.

// receiverFor returns the implicit receiver this refers to: the innermost
// lambda's receiver, or with a label, the receiver of the innermost lambda
// with the label. It returns nil if this is the enclosing class's. In the
// supertypes of an object expression or a local class, the arguments of
// its superclass's constructor and its delegates, which the object isn't
// yet, this is the code around it's.
func (c *Checker) receiverFor(x *syntax.This) *implicitRecv {
	if x.Class {
		return nil
	}
	header := c.cls == nil && c.staticCls != nil && c.anon[c.staticCls] != nil
	for i := len(c.recvs) - 1; i >= 0; i-- {
		r := &c.recvs[i]
		switch {
		case x.Recv != nil:
			if r.sym == x.Recv {
				return r
			}
		case x.Label == "" && r.outer && !header:
			return nil // the object's this
		case x.Label == "" || x.Label == r.label:
			return r
		}
	}
	if x.Recv != nil {
		// A context argument: a context parameter.
		for i := range c.ctxs {
			if r := &c.ctxs[i]; r.sym == x.Recv {
				return r
			}
		}
	}
	return nil
}

// thisExpr checks this, or this@label.
func (c *Checker) thisExpr(x *syntax.This) types.Type {
	if r := c.receiverFor(x); r != nil {
		x.Recv = r.sym
		if r.sym != c.noCapture {
			c.capture(r.sym, r.depth)
		}
		t := c.scope.narrowed(r.sym)
		x.Cast = needsCast(r.sym.Type, t)
		return t
	}
	if c.cls == nil {
		if x.Label != "" {
			c.errorf(x.P, "unresolved label: %s", x.Label)
			return types.InvalidType
		}
		c.errorf(x.P, "'this' is not defined in this context")
		return types.InvalidType
	}
	if name := c.cls.DisplayName(); x.Label != "" && x.Label != name[strings.LastIndexByte(name, '.')+1:] {
		c.errorf(x.P, "unresolved label: %s", x.Label)
		return types.InvalidType
	}
	c.captureThis()
	t := c.cls.This()
	if n, ok := c.narrowedThis(); ok {
		x.Cast = needsCast(t, n)
		t = n
	}
	return t
}

// recvThis returns this, referring to an implicit receiver.
func recvThis(r *implicitRecv, p syntax.Pos) *syntax.This {
	this := &syntax.This{Recv: r.sym}
	this.P = p
	return this
}

// recvMember checks a name that isn't a local as a property of an implicit
// receiver: x is this.x for the innermost receiver that has a property x.
// It reports false if none has.
func (c *Checker) recvMember(x *syntax.Ident) (types.Type, bool) {
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if c.recvs[i].outer && c.ownProp(x) {
			return types.InvalidType, false // the object's own
		}
		r := &c.recvs[i]
		m := &syntax.Member{X: recvThis(r, x.P), Name: x.Name}
		m.P = x.P
		n := len(c.errs)
		// The receiver is captured only if it has the property.
		saved := c.noCapture
		c.noCapture = r.sym
		t := c.expr(m, noWant)
		c.noCapture = saved
		if len(c.errs) > n {
			c.errs = c.errs[:n]
			continue
		}
		c.capture(r.sym, r.depth)
		x.Member = m
		return t, true
	}
	return types.InvalidType, false
}

// recvTarget returns the target of an assignment: a name that isn't a
// local but a property of an implicit receiver is that property.
func (c *Checker) recvTarget(x syntax.Expr) syntax.Expr {
	id, ok := x.(*syntax.Ident)
	if !ok || len(c.recvs) == 0 {
		return x
	}
	if _, _, local := c.scope.lookup(id.Name); local {
		return x
	}
	if _, ok := c.recvMember(id); ok && id.Member != nil && (id.Member.Prop != nil || id.Member.Getter != nil) {
		return id.Member
	}
	id.Member = nil
	return x
}

// recvCall rewrites an unqualified call f(args) to a call on the innermost
// implicit receiver that has a function f that takes the arguments: a
// member or an extension. It reports false if none has.
func (c *Checker) recvCall(x *syntax.Call, id *syntax.Ident) bool {
	for i := len(c.recvs) - 1; i >= 0; i-- {
		r := &c.recvs[i]
		if r.outer && c.cls != nil && c.hasFun(c.cls.This(), id.Name, x) {
			return false // the object's own
		}
		if t := c.scope.narrowed(r.sym); !c.hasFun(t, id.Name, x) && !c.hasFunProp(t, id.Name, x) {
			continue
		}
		m := &syntax.Member{X: recvThis(r, id.P), Name: id.Name}
		m.P = id.P
		x.Fun = m
		return true
	}
	return false
}

// hasFun reports whether a receiver of type t has a member or extension
// function with a name that may take a call's arguments, judging by their
// number and names, and whether lambdas go to function-typed parameters.
func (c *Checker) hasFun(t types.Type, name string, x *syntax.Call) bool {
	t = t.Flex()
	fits := func(fs []*types.FunSym) bool {
		for _, f := range fs {
			if c.mayTake(f, x) {
				return true
			}
		}
		return false
	}
	if types.MayBeNull(t) && name == "toString" && len(x.Args) == 0 {
		return true // Any?.toString(), a built-in extension
	}
	if !types.MayBeNull(t) {
		if builtinFun(t, name, len(x.Args)) {
			return true
		}
		if cls := c.classOf(t); cls != nil {
			cls.Ensure()
			if !cls.Missing && fits(c.accessible(c.javaMethods(cls, name))) {
				return true
			}
			if n, _ := c.innerNamed(cls, name); n != nil {
				// An inner class's constructor: Inner() is this.Inner().
				n.Ensure()
				if fits(userCtors(n)) {
					return true
				}
			}
		}
	}
	for _, cl := range c.extCalleesFor(name, t, x) {
		if c.mayTake(cl.f, x) {
			return true
		}
	}
	return false
}

// hasFunProp reports whether a receiver of type t has a property with a
// name, which f(x) invokes: a property of function type, as
// transformAll(it) in a lambda in an extension of clikt's
// OptionWithValues, or of a class with an invoke operator, as fixture()
// in kotlinfixture's factory { }, Generator's fixture.invoke().
func (c *Checker) hasFunProp(t types.Type, name string, x *syntax.Call) bool {
	t = t.Flex()
	if types.MayBeNull(t) {
		return false
	}
	cls := c.classOf(t)
	if cls == nil {
		return false
	}
	if cls.External {
		// A library's: transformEach(ctx, values) in an object
		// expression in an extension of clikt's OptionWithValues.
		if len(c.javaMethods(cls, name)) > 0 || !c.hasProp(cls, name) {
			return false
		}
		m := &syntax.Member{Name: name}
		pt, ok := c.javaProperty(m, t)
		return ok && c.invokableWith(pt, x)
	}
	p := c.userProp(cls, name)
	if p == nil {
		return false
	}
	ensureVar(p)
	return c.invokableWith(p.Type, x)
}

// invokableWith reports whether a value of type t may be invoked with a
// call's arguments: it is of function type and they fit, or its class
// has an invoke function that may take them.
func (c *Checker) invokableWith(t types.Type, x *syntax.Call) bool {
	if t.Flex().IsFunc() {
		return c.invokeFitsType(t, x)
	}
	return c.hasFun(t, "invoke", x)
}

// onReceiver checks a call f(args) that recvCall or the caller rewrote to
// one on an implicit receiver, x.Fun. In Kotlin each level of scope is
// tried in turn, so a receiver's function that the arguments don't fit
// gives way to a top-level one: run(listOf("a")) in a class calls the
// package's run(List<String>), not the standard library's T.run(block),
// which mayTake can't tell from the call. When a top-level function may
// take the arguments too, and there are no lambdas, which are checked
// once, the call is checked first on its own, without the sink, which
// would defer its errors, as in append(run(listOf("a"))). If it has
// errors, it goes back to f(args) (id), and onReceiver reports false.
func (c *Checker) onReceiver(x *syntax.Call, id *syntax.Ident, want types.Type, sink *system) (types.Type, bool) {
	if hasLambda(x.Args) || !c.anyMayTake(c.topFuns(id.Name), x) && !c.anyMayTake(c.kotlinFuns(id.Name, false), x) {
		return c.call(x, want, sink), true
	}
	n := len(c.errs)
	c.call(x, want, nil)
	if len(c.errs) > n {
		c.errs = c.errs[:n]
		x.Fun = id
		return types.InvalidType, false
	}
	return c.call(x, want, sink), true
}

// thisExtension reports whether the enclosing class's this has an
// extension function with a name that may take a call's arguments.
func (c *Checker) thisExtension(name string, x *syntax.Call) bool {
	t := c.cls.This()
	if n, ok := c.narrowedThis(); ok {
		t = n
	}
	for _, cl := range c.extCalleesFor(name, t, x) {
		if c.mayTake(cl.f, x) {
			return true
		}
	}
	return false
}

// lowPriorityGivesWay drops the members marked
// @LowPriorityInOverloadResolution when an extension of rt that isn't
// hidden may take the call's arguments: kotlinc picks them only if nothing
// else fits, so SelectBuilder's deprecated member onTimeout(timeMillis,
// block) calls the extension SelectBuilder.onTimeout, not itself.
func (c *Checker) lowPriorityGivesWay(ms []*types.FunSym, name string, rt types.Type, x *syntax.Call) []*types.FunSym {
	if !slices.ContainsFunc(ms, func(f *types.FunSym) bool { return f.LowPriority }) ||
		!slices.ContainsFunc(c.extCalleesFor(name, rt, x), func(cl callee) bool { return !cl.f.Hidden && c.mayTake(cl.f, x) }) {
		return ms
	}
	return slices.DeleteFunc(slices.Clone(ms), func(f *types.FunSym) bool { return f.LowPriority })
}

// mayTake reports whether f may take a call's arguments: their number and
// names fit its parameters, lambdas go to function-typed ones, and
// arguments whose types are known before the call is checked (this,
// locals, literals) fit parameters that aren't generic. An implicit
// receiver whose functions don't take the arguments gives way to the
// next one, as emit(this) in buildCodeString { } calls the enclosing
// class's emit(CodeWriter), not CodeWriter.emit(String). So does one
// whose function has a context parameter that no implicit value fits.
func (c *Checker) mayTake(f *types.FunSym, x *syntax.Call) bool {
	amap, ok := mapArgs(f, x)
	if !ok {
		return false
	}
	cl := callee{f: f, tparams: f.TypeParams, amap: amap, names: x.Names}
	if _, ok := c.contextArgs(&cl, x); !ok {
		return false
	}
	for i, a := range x.Args {
		p := cl.argType(i, x.Args)
		if l, ok := a.(*syntax.Lambda); ok && !c.lambdaFits(l, p) {
			return false
		}
		if p.K == types.Invalid || p.HasParams() || p.HasVars() || isSpread(a) {
			continue
		}
		if l, ok := a.(*syntax.IntLit); ok && !l.Unsigned && isUnsigned(p.NonNull()) {
			// 0xff000000 isn't a ULong: Compose's Color(Long), not the
			// constructor Color(ULong).
			return false
		}
		if t, ok := c.peekType(a); ok && !c.subtype(t, p) {
			if _, sam := c.samValue(t, p); !sam {
				return false
			}
		}
		if cls := c.peekClass(a); cls != nil && p.K == types.Class && !cls.IsSubclassOf(p.C) {
			return false
		}
	}
	return true
}

// argTypes checks a call's arguments ahead of the call, to choose among
// candidates that mayTake can't tell apart, and returns their types: those
// of the arguments without lambdas, object expressions or jumps, which
// check without errors, and Invalid for the others. The call checks them
// again.
func (c *Checker) argTypes(x *syntax.Call) []types.Type {
	ts := make([]types.Type, len(x.Args))
	for i, a := range x.Args {
		ok := true
		syntax.Inspect(a, func(n any) bool {
			switch n.(type) {
			case *syntax.Lambda, *syntax.ObjectExpr, *syntax.Jump, *syntax.Assign:
				ok = false
			}
			return ok
		})
		if !ok {
			continue
		}
		n := len(c.errs)
		t := c.expr(a, noWant)
		if len(c.errs) > n {
			c.errs = c.errs[:n]
			continue
		}
		ts[i] = t
	}
	return ts
}

// takeTypes reports whether one of fs may take a call's arguments of
// types ts, as argTypes returns them.
func (c *Checker) takeTypes(fs []*types.FunSym, x *syntax.Call, ts []types.Type) bool {
	for _, f := range fs {
		amap, ok := mapArgs(f, x)
		if !ok || f.Unsupported {
			continue
		}
		cl := callee{f: f, amap: amap, names: x.Names}
		fits := true
		for i, t := range ts {
			p := cl.argType(i, x.Args)
			if t.K == types.Invalid || p.K == types.Invalid || p.HasParams() || p.HasVars() {
				continue
			}
			if lit, ok := x.Args[i].(*syntax.IntLit); ok && fitsLit(lit, p) {
				continue // 1 for a Long
			}
			if _, sam := c.samValue(t, p); !sam && !c.subtype(t, p) {
				fits = false
				break
			}
		}
		if fits {
			return true
		}
	}
	return false
}

// peekClass returns the class of the value of a call of a top-level
// function, without checking it, if every function the name may call
// returns that class: listOf(m) is a List, whatever its element type, so
// it can't be a Module. It returns nil for other expressions.
func (c *Checker) peekClass(a syntax.Expr) *types.ClassSym {
	x, ok := a.(*syntax.Call)
	if !ok || hasLambda(x.Args) {
		return nil
	}
	fn, ok := x.Fun.(*syntax.Ident)
	if !ok {
		return nil
	}
	if _, _, ok := c.scope.lookup(fn.Name); ok || c.topVar(fn.Name) != nil || c.imports[fn.Name] != nil || c.pkg.Classes[fn.Name] != nil {
		return nil
	}
	if c.cls != nil && len(c.javaMethods(c.cls, fn.Name)) > 0 {
		return nil // a member of this is called
	}
	for _, r := range c.recvs {
		if cls := c.classOf(c.scope.narrowed(r.sym).NonNull()); cls == nil || len(c.javaMethods(cls, fn.Name)) > 0 {
			return nil // a member of an implicit receiver may be called
		}
	}
	fs := c.topFuns(fn.Name)
	if len(fs) == 0 {
		fs = c.kotlinFuns(fn.Name, false)
	}
	var cls *types.ClassSym
	for _, f := range fs {
		ensureFun(f)
		if !f.Resolved || f.Ret.K != types.Class || cls != nil && f.Ret.C != cls {
			return nil
		}
		cls = f.Ret.C
	}
	return cls
}

// peekType returns the type of a simple argument without checking it:
// this, a local, or a literal. It reports false for other expressions,
// which are checked once, when the call is resolved.
func (c *Checker) peekType(a syntax.Expr) (types.Type, bool) {
	switch a := a.(type) {
	case *syntax.This:
		if a.Label != "" {
			return types.InvalidType, false
		}
		if r := c.receiverFor(a); r != nil {
			return c.scope.narrowed(r.sym), true
		}
		if c.cls != nil {
			if n, ok := c.narrowedThis(); ok {
				return n, true
			}
			return c.cls.This(), true
		}
	case *syntax.StringLit:
		return types.StringType, true
	case *syntax.NullLit:
		return types.NullType, true
	case *syntax.BoolLit:
		return types.BoolType, true
	case *syntax.Ident:
		if _, t, ok := c.scope.lookup(a.Name); ok && t.K != types.Invalid {
			return t, true
		}
	}
	return types.InvalidType, false
}

// builtinRefParams returns the parameter types of a builtin type's builtin
// member function name (see builtinFun), if it has one of the name, for
// a reference to it without an expected type.
func builtinRefParams(t types.Type, name string) ([]types.Type, bool) {
	switch {
	case builtinFun(t, name, 0):
		return nil, true
	case !builtinFun(t, name, 1):
		return nil, false
	case name == "equals":
		return []types.Type{types.AnyType.Nullable()}, true
	case t == types.StringType && name == "plus":
		return []types.Type{types.AnyType.Nullable()}, true
	case t == types.StringType && name == "get", name == "shl" || name == "shr" || name == "ushr":
		return []types.Type{types.IntType}, true
	case t == types.BoolType || types.BitwiseOps[name]:
		return []types.Type{t}, true
	}
	return nil, false // the numbers' operators are overloaded
}

// builtinFun reports whether a builtin type has a builtin member function
// (see builtinMember) with a name that takes n arguments.
func builtinFun(t types.Type, name string, n int) bool {
	switch {
	case t == types.StringType:
		switch name {
		case "uppercase", "lowercase", "isEmpty", "toInt", "toString", "toCharArray":
			return n == 0
		case "get", "plus":
			return n == 1
		case "substring":
			return n == 1 || n == 2
		}
	case t.IsNumeric() || t == types.CharType:
		if _, _, ok := conversion(name); ok {
			return n == 0
		}
		if k, ok := types.OperatorFuns[name]; ok {
			return k == n
		}
		if (t == types.IntType || t == types.LongType) && types.BitwiseOps[name] {
			return n == 0 == (name == "inv")
		}
		if _, ok := types.CharMethods[name]; ok && t == types.CharType && n == 0 {
			return true
		}
	case t == types.BoolType:
		switch name {
		case "toString", "not":
			return n == 0
		case "and", "or", "xor":
			return n == 1
		}
	}
	// Any's members, which a primitive's box has.
	return t.IsPrimitive() && (name == "equals" && n == 1 || (name == "hashCode" || name == "toString") && n == 0)
}

// implicitArg passes the implicit receiver to a call of a function value
// with a receiver that leaves it out: block() for block: A.() -> R, in a
// lambda whose receiver is an A, is block(this). It passes the context
// arguments of a context function type that leaves them out too.
func (c *Checker) implicitArg(x *syntax.Call, ft types.Type) {
	defer c.implicitContexts(x, ft)
	k := ft.Contexts()
	if !ft.IsExtFunc() || len(x.Args) != ft.A.Len()-2-k {
		return
	}
	recv := ft.A.Get(k).T
	for i := len(c.recvs) - 1; i >= 0; i-- {
		r := &c.recvs[i]
		if c.subtype(c.scope.narrowed(r.sym), recv) {
			x.Args = append([]syntax.Expr{recvThis(r, x.P)}, x.Args...)
			return
		}
	}
	if c.cls != nil && c.subtype(c.cls.This(), recv) {
		this := &syntax.This{}
		this.P = x.P
		x.Args = append([]syntax.Expr{this}, x.Args...)
	}
}

// memberExtFirst reports whether x.f() calls a member of x, or a function
// f, rather than a property f in scope of function type with a receiver:
// as in Kotlin, x's members come first, and at the level of a class, its
// member extensions come before its properties. ktor's DelegatingMutableSet
// has a convert: To.() -> From property and a Collection<To>.convert()
// member extension, which elements.convert() calls; clikt calls the
// transformEach property of an option, the implicit receiver, in an
// object expression in a class with one too.
func (c *Checker) memberExtFirst(fn *syntax.Member, x *syntax.Call) bool {
	rt, ok := c.peekType(fn.X)
	if !ok {
		if !simpleExpr(fn.X) {
			return false
		}
		// Checked again when the call is resolved.
		n := len(c.errs)
		rt = c.expr(fn.X, noWant)
		if len(c.errs) > n {
			c.errs = c.errs[:n]
			return false
		}
	}
	rt = rt.Flex()
	if cls := c.classOf(rt.NonNull()); cls != nil && !types.MayBeNull(rt) {
		cls.Ensure()
		if slices.ContainsFunc(c.javaMethods(cls, fn.Name), func(f *types.FunSym) bool { return c.mayTake(f, x) }) {
			return true
		}
		if !cls.External && c.userProp(cls, fn.Name) != nil || cls.External && c.hasProp(cls, fn.Name) {
			return true
		}
	}
	if _, _, local := c.scope.lookup(fn.Name); local || c.cls == nil || c.userProp(c.cls, fn.Name) == nil {
		return false
	}
	return slices.ContainsFunc(memberExtMethods(c.cls, fn.Name), func(f *types.FunSym) bool {
		return c.recvFits(rt, f.Recv) && c.recvTakes(rt, f.Recv) && c.mayTake(f, x)
	})
}

// simpleExpr reports whether an expression may be checked twice: a name, or
// a chain of member accesses and calls without lambdas on one.
func simpleExpr(e syntax.Expr) bool {
	switch e := e.(type) {
	case *syntax.Ident, *syntax.This:
		return true
	case *syntax.Member:
		return simpleExpr(e.X)
	case *syntax.Call:
		return !hasLambda(e.Args) && simpleExpr(e.Fun)
	}
	return false
}

// extFuncVar returns the type of a local or a property of the enclosing
// class with a name, if it is a function type with a receiver: x.f() calls
// it with x as the receiver. In an object expression, the classes around
// it count too: ktor's DelegatingMutableSet calls its convertTo: From.() ->
// To in an iterator object's next().
func (c *Checker) extFuncVar(name string) (types.Type, bool) {
	if _, t, ok := c.scope.lookup(name); ok {
		return t, t.Flex().IsExtFunc()
	}
	prop := func(p *types.VarSym) (types.Type, bool) {
		if p.InProgress {
			// Being inferred, as aliases in val aliases =
			// command.aliases(): x.f() calls a function.
			return types.InvalidType, false
		}
		if d, ok := p.Decl.(*syntax.PropDecl); ok && d.Field != nil && d.Field.Sym != nil && d.Field.Sym.InProgress {
			// So is its delegate: val enabled by lazy { checker.enabled() }.
			return types.InvalidType, false
		}
		ensureVar(p)
		return p.Type, p.Type.Flex().IsExtFunc()
	}
	// The implicit receivers of lambdas and extensions come first: 5.foo()
	// in with(a) { ... } calls a's foo: Int.() -> Int.
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if c.recvs[i].outer {
			continue
		}
		if cls := c.classOf(c.recvs[i].sym.Type); cls != nil && !cls.External {
			if p := c.userProp(cls, name); p != nil {
				return prop(p)
			}
		}
	}
	if c.cls != nil {
		if p := c.userProp(c.cls, name); p != nil {
			return prop(p)
		}
	}
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if !c.recvs[i].outer {
			continue
		}
		if cls := c.classOf(c.recvs[i].sym.Type); cls != nil && !cls.External {
			if p := c.userProp(cls, name); p != nil {
				return prop(p)
			}
		}
	}
	if v := c.topVar(name); v != nil && !v.InProgress {
		ensureVar(v)
		return v.Type, v.Type.Flex().IsExtFunc() // a top-level one: clikt's conversion
	}
	return types.InvalidType, false
}
