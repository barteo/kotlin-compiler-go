package check

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Operator conventions.
//
// On other types than the built-in ones, an operator calls a function
// declared operator, a member or an extension, as Kotlin defines it:
//
//	-x, +x, !x        x.unaryMinus(), x.unaryPlus(), x.not()
//	x + y, x - y, ... x.plus(y), x.minus(y), x.times(y), x.div(y), x.rem(y)
//	x < y, ...        x.compareTo(y) < 0, ...
//	x in y, x !in y   y.contains(x), !y.contains(x)
//	x(args)           x.invoke(args)
//	x += y, ...       x.plusAssign(y), or x = x.plus(y)
//	x[i] += y         x.set(i, x.get(i) + y), or x.get(i).plusAssign(y)
//	x++, x--          x = x.inc(), x = x.dec()
//
// Indexing, iteration and destructuring call get, set, iterator, hasNext,
// next and componentN. The checker builds the call, as a Call with
// Operator set whose receiver is the operand, already checked, in a
// Checked node; codegen generates it as any call. Compound assignments
// and ++ are rewritten into statements, with the receiver and indices of
// their target in temporaries, so that each is evaluated once.

// opNames maps an operator to the function it calls.
var opNames = map[syntax.Kind]string{
	syntax.PLUS: "plus", syntax.MINUS: "minus", syntax.STAR: "times", syntax.SLASH: "div", syntax.PERCENT: "rem",
	syntax.PLUS_EQ: "plusAssign", syntax.MINUS_EQ: "minusAssign", syntax.STAR_EQ: "timesAssign",
	syntax.SLASH_EQ: "divAssign", syntax.PERCENT_EQ: "remAssign",
	syntax.INC: "inc", syntax.DEC: "dec",
}

// operatorArity gives the number of parameters an operator function
// takes, by name: -1 for any number, -n for at least n.
var operatorArity = map[string]int{
	"unaryPlus": 0, "unaryMinus": 0, "not": 0, "inc": 0, "dec": 0,
	"iterator": 0, "hasNext": 0, "next": 0,
	"plus": 1, "minus": 1, "times": 1, "div": 1, "rem": 1, "mod": 1,
	"rangeTo": 1, "rangeUntil": 1, "contains": 1, "compareTo": 1, "equals": 1,
	"plusAssign": 1, "minusAssign": 1, "timesAssign": 1, "divAssign": 1, "remAssign": 1,
	"get": -1, "set": -2, "invoke": -0x7fff,
	"getValue": 2, "setValue": 3, "provideDelegate": 2,
}

// checkOperatorDecl checks a function declared operator: its name must be
// one of an operator, and it must take the operator's parameters.
func (c *Checker) checkOperatorDecl(d *syntax.FunDecl, owner *types.ClassSym) {
	n, ok := operatorArity[d.Name]
	if !ok {
		if k, err := strconv.Atoi(strings.TrimPrefix(d.Name, "component")); err == nil && k > 0 && strings.HasPrefix(d.Name, "component") {
			n, ok = 0, true
		}
	}
	switch {
	case !ok:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: illegal function name")
	case d.Name == "mod" || d.Name == "modAssign":
		// % calls rem, and %= remAssign: mod is an operator no longer.
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: illegal function name")
	case owner == nil && d.Recv == nil:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must be a member or an extension function")
	case n >= 0 && len(d.Params) != n && !((d.Name == "inc" || d.Name == "dec") && defaultsOnly(d.Params)):
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must have %s", countParams(n))
	case n < 0 && n != -0x7fff && len(d.Params) < -n:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must have at least %s", countParams(-n))
	case d.Suspend && (d.Name == "equals" || d.Name == "getValue" || d.Name == "setValue" || d.Name == "provideDelegate"):
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must not be suspend")
	case n >= 0 && d.Name != "inc" && d.Name != "dec" && slices.ContainsFunc(d.Params, func(p *syntax.Param) bool { return p.Vararg || p.Default != nil }):
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: should not have varargs or parameters with default values")
	case d.Name == "equals" && d.Recv != nil:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must be a member function")
	case d.Name == "equals" && d.Params[0].Sym != nil && d.Params[0].Sym.Type != types.AnyType.Nullable():
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must override ''equals()'' in Any")
	case (d.Name == "getValue" || d.Name == "setValue" || d.Name == "provideDelegate") && !c.takesKProperty(d.Params[1]):
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: second parameter must be of type KProperty<*> or its supertype")
	case d.Sym.Resolved:
		c.operatorReturn(d, owner)
	}
}

// operatorReturn checks the return type of operator function d, a member
// of owner or not, once it is known.
func (c *Checker) operatorReturn(d *syntax.FunDecl, owner *types.ClassSym) {
	ret := d.Sym.Ret.Flex()
	if ret.K == types.Invalid {
		return
	}
	switch {
	case d.Name == "compareTo" && ret != types.IntType:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must return Int")
	case (d.Name == "contains" || d.Name == "hasNext") && ret != types.BoolType:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must return Boolean")
	case strings.HasSuffix(d.Name, "Assign") && ret != types.UnitType:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must return Unit")
	case d.Name == "inc" || d.Name == "dec":
		// The result is assigned to the receiver.
		recv := d.Sym.Recv
		if d.Recv == nil {
			if owner == nil {
				return
			}
			recv = owner.This()
		}
		if recv.K != types.Invalid && !c.subtype(d.Sym.Ret, recv) {
			c.errorf(d.P, "'operator' modifier is inapplicable on this function: receiver must be a supertype of the return type")
		}
	}
}

// takesKProperty reports whether parameter p, a delegate operator's
// second, can take a KProperty<*>.
func (c *Checker) takesKProperty(p *syntax.Param) bool {
	cls := c.u.ext.class("kotlin/reflect/KProperty")
	if cls == nil || p.Sym == nil || p.Sym.Type.K == types.Invalid {
		return true
	}
	return c.subtype(types.GenericType(cls, []types.Arg{{V: types.Star}}), p.Sym.Type)
}

// defaultsOnly reports whether every parameter has a default value: as
// kotlinc, ++ calls an inc(s: String = "OK"), which other operators with
// no value parameters may not take.
func defaultsOnly(ps []*syntax.Param) bool {
	for _, p := range ps {
		if p.Default == nil || p.Vararg {
			return false
		}
	}
	return true
}

func countParams(n int) string {
	switch n {
	case 0:
		return "no value parameters"
	case 1:
		return "a single value parameter"
	}
	return fmt.Sprintf("%d value parameters", n)
}

// builtinOperand reports whether operators on a value of type t are the
// built-in ones: on numbers, Char, Boolean and String, and on type
// parameters, which compare through Comparable.
func builtinOperand(t types.Type) bool {
	t = t.Flex().NonNull()
	return t.IsNumeric() || t == types.CharType || t == types.BoolType || t == types.StringType ||
		t.IsParam() || t.K == types.Invalid || t.K == types.Nothing || t.K == types.Range
}

// classBound reports whether t is a type parameter bounded by a class
// type, whose operator functions x op= v calls: destination += pair for a
// destination: M, M : MutableMap<in K, in V>.
func classBound(t types.Type) bool {
	t = t.Flex()
	return t.K == types.TypeParam && t.Param().Bound().NonNull().K == types.Class
}

// checked wraps an expression the checker already checked, of type t.
func checked(x syntax.Expr, t types.Type) *syntax.Checked {
	ch := &syntax.Checked{X: x}
	ch.P = x.Pos()
	ch.T = t
	return ch
}

// opCall builds and checks the operator call recv.name(args), where recv
// is checked already.
func (c *Checker) opCall(p syntax.Pos, recv syntax.Expr, name string, args []syntax.Expr, want types.Type) (*syntax.Call, types.Type) {
	m := &syntax.Member{X: recv, Name: name}
	m.P = p
	call := &syntax.Call{Fun: m, Args: args, Operator: true}
	call.P = p
	return call, c.expr(call, want)
}

// opExtension reports whether a receiver of type t has an operator
// extension named name that may take the argument arg: one of a nullable
// receiver's, as Int?.inc() or Long?.compareTo(Long?), or one of a
// built-in type's whose built-in operator doesn't take arg's type, as
// Int.compareTo(Char). The arguments must fit by their types.
func (c *Checker) opExtension(t types.Type, name string, args ...syntax.Expr) bool {
	t = t.Flex()
	x := &syntax.Call{Args: args}
	var ts []types.Type
	for _, cl := range c.extCalleesFor(name, t, x) {
		if !cl.f.Operator || !c.recvTakes(t, cl.f.Recv) || !c.mayTake(cl.f, x) {
			continue
		}
		if ts == nil {
			ts = c.argTypes(x)
		}
		if c.takeTypes([]*types.FunSym{cl.f}, x, ts) {
			return true
		}
	}
	return false
}

// binaryOpName returns the name of the operator function that an
// arithmetic operator or a comparison calls, or "".
func binaryOpName(k syntax.Kind) string {
	switch k {
	case syntax.LT, syntax.GT, syntax.LE, syntax.GE:
		return "compareTo"
	case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT:
		return opNames[k]
	}
	return ""
}

// never reports whether t is Nothing, the type of an expression that
// never completes.
func never(t types.Type) bool {
	return t.K == types.Nothing && !t.N
}

// nullableOperand reports whether t is a nullable type whose operators
// are only extensions: not a platform type, which may be used as not
// null, nor the type of null, nor a String?, whose plus is built in.
func nullableOperand(t types.Type) bool {
	t = t.Flex()
	return types.MayBeNull(t) && !t.P && t.K != types.Null && t.NonNull() != types.StringType && t.K != types.Invalid
}

// binaryOp checks an arithmetic operator or a comparison whose left
// operand, of type xt, isn't of a built-in type.
func (c *Checker) binaryOp(x *syntax.Binary, xt types.Type, want types.Type) types.Type {
	recv := checked(x.X, xt)
	switch x.Op {
	case syntax.LT, syntax.GT, syntax.LE, syntax.GE:
		call, t := c.opCall(x.P, recv, "compareTo", []syntax.Expr{x.Y}, noWant)
		x.Call = call
		if t.K != types.Invalid && t.Flex() != types.IntType {
			c.errorf(x.P, "'compareTo' must return Int, not %s", t)
		}
		return types.BoolType
	}
	call, t := c.opCall(x.P, recv, opNames[x.Op], []syntax.Expr{x.Y}, want)
	x.Call = call
	return t
}

// containsOp checks x in y, or x !in y, for a y that isn't a range written
// in place: y.contains(x).
func (c *Checker) containsOp(x *syntax.Binary) types.Type {
	yt := c.value(x.Y, noWant)
	if yt.K == types.Invalid {
		c.value(x.X, noWant)
		return types.BoolType
	}
	call, t := c.opCall(x.P, checked(x.Y, yt), "contains", []syntax.Expr{x.X}, types.BoolType)
	x.Call = call
	if t.K != types.Invalid && t.Flex() != types.BoolType {
		c.errorf(x.P, "'contains' must return Boolean, not %s", t)
	}
	if f := call.Sym; f != nil && f.Name == "contains" && len(f.Params) == 1 && types.MayBeNull(f.Params[0].Type) && f.Params[0].Type.Flex().NonNull() == types.AnyType {
		// ConcurrentHashMap's contains(Object), which Kotlin calls for in
		// as for its other operators, is containsValue, not containsKey.
		if cls := c.classOf(yt.Flex().NonNull()); cls != nil && c.isSubclassOf(cls, "java/util/concurrent/ConcurrentHashMap") {
			c.errorf(x.P, "method 'contains' from ConcurrentHashMap may have unexpected semantics: it calls 'containsValue' instead of 'containsKey'; use the explicit call or cast the value to kotlin.collections.Map")
		}
	}
	return types.BoolType
}

// unaryOp checks -x, +x or !x on a value of type xt that isn't a number
// or a Boolean.
func (c *Checker) unaryOp(x *syntax.Unary, xt types.Type, want types.Type) types.Type {
	name := map[syntax.Kind]string{syntax.MINUS: "unaryMinus", syntax.PLUS: "unaryPlus", syntax.NOT: "not"}[x.Op]
	call, t := c.opCall(x.P, checked(x.X, xt), name, nil, want)
	x.Call = call
	return t
}

// invokeOp checks x(args) on a value x of type ft that isn't a function:
// x.invoke(args).
func (c *Checker) invokeOp(x *syntax.Call, ft, want types.Type, sink *system) (types.Type, bool) {
	// Built-in types take invoke extensions too: "a"(12) for a
	// String.invoke(i: Int).
	if ft.K == types.Class && ft.C.Func || !c.hasFun(ft, "invoke", x) {
		return types.InvalidType, false
	}
	m := &syntax.Member{X: checked(x.Fun, ft), Name: "invoke"}
	m.P = x.Fun.Pos()
	x.Fun, x.Operator = m, true
	return c.call(x, want, sink), true
}

// temp declares a temporary local holding a checked expression x of type
// t, and returns its statement.
func (c *Checker) temp(p syntax.Pos, what string, x syntax.Expr, t types.Type) *syntax.VarStmt {
	c.temps++
	vs := &syntax.VarStmt{P: p, Name: fmt.Sprintf("<%s%d>", what, c.temps), Init: checked(x, t)}
	c.varStmt(vs)
	return vs
}

// tempRef refers to a temporary.
func tempRef(vs *syntax.VarStmt, p syntax.Pos) *syntax.Ident {
	id := &syntax.Ident{Name: vs.Name}
	id.P = p
	return id
}

// indexCompound checks a compound assignment or ++ of an element, x[i] op=
// v, of a receiver that isn't an array: it reports false for an array
// indexed by an Int, which assigns elements directly, and not for one
// with an operator get(Long) or get(Int, Int). The receiver and indices
// go to temporaries; then, if the element has the assignment operator
// assignOp (plusAssign, taking arg), it is x.get(i).plusAssign(v), and
// otherwise x.set(i, x.get(i) op v). value builds the new value from the
// element read. It returns a function that makes another read of the
// element, to check after the statements.
func (c *Checker) indexCompound(p syntax.Pos, ix *syntax.Index, assignOp string, arg syntax.Expr, value func(read syntax.Expr) syntax.Expr) ([]syntax.Stmt, func() syntax.Expr, bool) {
	xt := c.value(ix.X, noWant)
	ix.X = checked(ix.X, xt)
	if xt.K == types.Invalid {
		return nil, nil, false
	}
	ts := make([]types.Type, len(ix.Args))
	if slices.ContainsFunc(ix.Args, func(a syntax.Expr) bool { _, ok := a.(*syntax.Lambda); return ok }) {
		// A lambda or a reference takes its type from get's parameter,
		// as A[::f] += 1 for an operator get(IFoo) does a fun
		// interface's: x.get(i) is checked first, and the index keeps the
		// type it gave the lambda.
		probe := &syntax.Index{X: checked(ix.X, xt), Args: slices.Clone(ix.Args)}
		probe.P = ix.P
		c.value(probe, noWant)
		for i, a := range ix.Args {
			if l, ok := a.(*syntax.Lambda); ok {
				ts[i] = l.T
				ix.Args[i] = checked(a, ts[i])
			}
		}
	}
	for i, a := range ix.Args {
		if _, done := a.(*syntax.Checked); done {
			continue
		}
		ts[i] = c.value(a, noWant)
		ix.Args[i] = checked(a, ts[i])
	}
	if xt.NonNull().IsArray() && len(ts) == 1 && (ts[0].K == types.Int || ts[0].K == types.Invalid) {
		return nil, nil, false
	}
	recv := c.temp(p, "array", ix.X, xt)
	stmts := []syntax.Stmt{recv}
	var idx []*syntax.VarStmt
	for i, a := range ix.Args {
		vs := c.temp(p, "index", a, ts[i])
		idx = append(idx, vs)
		stmts = append(stmts, vs)
	}
	elem := func() syntax.Expr {
		e := &syntax.Index{X: tempRef(recv, p)}
		e.P = ix.P
		for _, vs := range idx {
			e.Args = append(e.Args, tempRef(vs, p))
		}
		return e
	}
	read := elem()
	et := c.value(read, noWant)
	if et.K == types.Invalid {
		c.unresolvedArg(arg)
		return stmts, elem, true
	}
	if r := c.stub(et.Flex()); r != nil {
		// As kotlinc's BUILDER_INFERENCE_STUB_RECEIVER, an element of a
		// builder's type doesn't fix it here, as it does in get(0) + "K":
		// this[0] += "K" in buildList { add("O") } is an error.
		c.errorf(ix.P, "the type of a receiver hasn't been inferred yet. Specify type argument for generic parameter '%s' explicitly.", et.Flex().Param().Name)
		return stmts, elem, true
	}
	if assignOp != "" && !builtinOperand(et) && c.hasFun(et.NonNull(), assignOp, &syntax.Call{Args: []syntax.Expr{arg}}) {
		// As in Kotlin, x.set(i, x.get(i).plus(v)) may apply too: then
		// the two are ambiguous, and v isn't checked.
		opName := strings.TrimSuffix(assignOp, "Assign")
		probe := &syntax.Call{Args: []syntax.Expr{arg}}
		if c.hasFun(et.NonNull(), opName, probe) && c.setFits(xt, len(idx), et, opName, probe) {
			c.errorf(p, "assignment operators ambiguity: '%s' and '%s' both apply", assignOp, opName)
			c.unresolvedArg(arg)
			return stmts, elem, true
		}
		call, t := c.opCall(p, checked(read, et), assignOp, []syntax.Expr{arg}, noWant)
		c.wantUnit(p, assignOp, t)
		return append(stmts, &syntax.ExprStmt{X: call}), elem, true
	}
	set := &syntax.Assign{P: p, Op: syntax.ASSIGN, Target: elem(), Value: value(checked(read, et))}
	c.assign(set)
	return append(stmts, set), elem, true
}

// opFits reports whether a receiver of type t has an operator function
// name that may take call x and whose result may be assigned to want:
// whether x op= v may also be x = x.op(v), as Kotlin resolves it. It
// decides from the declarations, without checking x's arguments (a
// lambda's body would be checked once per form, which is exponential
// in nested x[i] += { }), and where they can't tell, it is true. So a
// var list: ArrayList<Int>'s list - 2, a List<Int>, doesn't fit.
func (c *Checker) opFits(t types.Type, name string, x *syntax.Call, want types.Type) bool {
	t = t.Flex()
	if !types.MayBeNull(t) {
		if builtinFun(t, name, len(x.Args)) {
			return true
		}
		if cls := c.classOf(t); cls != nil {
			cls.Ensure()
			if cls.Missing {
				return true
			}
			for _, cl := range c.memberCallees(c.accessible(cls, c.javaMethods(cls, name)), t) {
				if c.mayTake(cl.f, x) && c.retFits(substRet(cl.recv, cl.f.Ret), want, cl.tparams) {
					return true
				}
			}
		}
	}
	for _, cl := range c.extCalleesFor(name, t, x) {
		if cl.f.Operator && c.recvTakes(t, cl.f.Recv) && c.mayTake(cl.f, x) && c.retFits(substRet(c.extRecvSubst(t, cl), cl.f.Ret), want, cl.tparams) {
			return true
		}
	}
	return false
}

// substRet applies s, which may be nil, to a declared type.
func substRet(s types.Subst, t types.Type) types.Type {
	if len(s) == 0 {
		return t
	}
	return s.Apply(t)
}

// extRecvSubst returns the type arguments of extension cl's own type
// parameters that its receiver's type gives for a receiver of type rt:
// T of Collection<T>.plus(T): List<T> is Int for a List<Int>.
func (c *Checker) extRecvSubst(rt types.Type, cl callee) types.Subst {
	if len(cl.tparams) == 0 {
		return nil
	}
	own := func(u types.Type) bool { return u.K == types.TypeParam && slices.Contains(cl.tparams, u.Param()) }
	r := cl.f.Recv.NonNull()
	if own(r) {
		return types.Subst{r.Param(): {T: rt.Flex().NonNull()}}
	}
	if r.K != types.Class {
		return nil
	}
	sup, ok := c.asSuper(rt.Flex().NonNull(), r.C)
	if !ok {
		return nil
	}
	actual := classSubst(sup)
	s := types.Subst{}
	for p, a := range classSubst(r) {
		if b, ok := actual[p]; ok && own(a.T) {
			s[a.T.Param()] = b
		}
	}
	return s
}

// retFits reports whether a function's result type ret, which may
// mention its own type parameters tparams, may be assigned to want. With
// them, only the classes and the nullability are compared.
func (c *Checker) retFits(ret, want types.Type, tparams []*types.TypeParamSym) bool {
	if ret.K == types.Invalid || want.K == types.Invalid {
		return true
	}
	own := func(u types.Type) bool { return u.K == types.TypeParam && slices.Contains(tparams, u.Param()) }
	if !ret.Has(own) {
		return c.subtype(ret, want)
	}
	r, w := ret.Flex(), want.Flex()
	if own(r.NonNull()) {
		return true
	}
	if types.MayBeNull(r) && !types.MayBeNull(w) {
		return false
	}
	if r.NonNull().K != types.Class || w.NonNull().K != types.Class {
		return true
	}
	_, ok := c.asSuper(r.NonNull(), w.NonNull().C)
	return ok
}

// setFits reports whether a receiver of type xt has an operator set
// taking n indices whose value parameter an element's opName(arg)
// result fits: whether x[i] op= v may also be x.set(i, x.get(i).op(v)).
func (c *Checker) setFits(xt types.Type, n int, et types.Type, opName string, probe *syntax.Call) bool {
	xt = xt.Flex()
	var params []types.Type
	if !types.MayBeNull(xt) {
		if cls := c.classOf(xt); cls != nil {
			cls.Ensure()
			for _, cl := range c.memberCallees(c.accessible(cls, c.javaMethods(cls, "set")), xt) {
				if len(cl.f.Params) == n+1 {
					params = append(params, substRet(cl.recv, cl.f.Params[n].Type))
				}
			}
		}
	}
	for _, cl := range c.extCalleesFor("set", xt, nil) {
		if cl.f.Operator && len(cl.f.Params) == n+1 && c.recvTakes(xt, cl.f.Recv) {
			p := substRet(c.extRecvSubst(xt, cl), cl.f.Params[n].Type)
			if p.Has(func(u types.Type) bool { return u.K == types.TypeParam && slices.Contains(cl.tparams, u.Param()) }) {
				p = types.InvalidType // unknown: fits
			}
			params = append(params, p)
		}
	}
	for _, p := range params {
		if c.opFits(et, opName, probe, p) {
			return true
		}
	}
	return false
}

// unresolvedArg checks the value v of an operator call that is ambiguous,
// or that has no operator to call, once, as kotlinc checks an unresolved
// call's arguments: the errors in a lambda's body are reported, even in
// nested x[i] += { x[i] += { } }, whose check stays linear.
func (c *Checker) unresolvedArg(v syntax.Expr) {
	if v == nil {
		return
	}
	if _, done := v.(*syntax.Checked); done {
		return
	}
	c.args(&syntax.Call{Args: []syntax.Expr{v}}, nil)
}

func (c *Checker) wantUnit(p syntax.Pos, name string, t types.Type) {
	if t.K != types.Invalid && t.K != types.Unit {
		c.errorf(p, "function '%s' should return Unit", name)
	}
}

// compoundOp checks x op= v whose target x, already checked as an
// assignment target of type tt, isn't of a built-in type. With
// x.plusAssign(v) and no plus, or a val x, it calls plusAssign; with plus
// only, it is x = x.plus(v); with both and a var x, it is ambiguous.
// recv is x read with its smart cast, of type rt, or nil if it has none.
// It reports whether x op= v calls x.plusAssign(v), which leaves x as it
// is.
func (c *Checker) compoundOp(s *syntax.Assign, tt, want types.Type, mutable bool, recv syntax.Expr, rt types.Type) bool {
	assignName, opName := opNames[s.Op], opNames[arithOf(s.Op)]
	if recv == nil {
		recv, rt = checked(s.Target, tt), tt
	}
	probe := &syntax.Call{Args: []syntax.Expr{s.Value}}
	hasAssign := c.hasFun(rt.NonNull(), assignName, probe)
	hasOp := c.hasFun(rt.NonNull(), opName, probe)
	fits := hasAssign && hasOp && mutable && c.opFits(rt.NonNull(), opName, probe, want)
	if fits && rt.Flex().NonNull().IsNumeric() && c.argTypes(probe)[0].K == types.Nothing {
		// a + todo() is ambiguous among Int's plus functions, which all
		// take a Nothing: only a.plusAssign(todo()) applies.
		fits = false
	}
	switch {
	case fits:
		c.errorf(s.P, "assignment operators ambiguity: '%s' and '%s' both apply", assignName, opName)
		c.unresolvedArg(s.Value)
		return false
	case hasAssign:
		call, t := c.opCall(s.P, recv, assignName, []syntax.Expr{s.Value}, noWant)
		c.wantUnit(s.P, assignName, t)
		s.Stmts = []syntax.Stmt{&syntax.ExprStmt{X: call}}
		return true
	case !hasOp:
		c.errorf(s.P, "operator '%s' cannot be applied to %s", s.Op, rt)
		return false
	case !mutable:
		c.errorf(s.Target.Pos(), "val cannot be reassigned")
		return false
	}
	op := arithOf(s.Op)
	s.Stmts, _ = c.reassign(s.P, s.Target, tt, func(read syntax.Expr) syntax.Expr {
		b := &syntax.Binary{Op: op, X: read, Y: s.Value}
		b.P = s.P
		return b
	})
	return false
}

// incDecOp checks x++ or x-- on a target of type t that isn't a number or
// a Char: x = x.inc(), whose read keep may change, as reassign returns it.
func (c *Checker) incDecOp(u *syntax.Unary, t types.Type, keep func(func(syntax.Expr) syntax.Expr) func(syntax.Expr) syntax.Expr) ([]syntax.Stmt, func() syntax.Expr) {
	return c.reassign(u.P, u.X, t, keep(func(read syntax.Expr) syntax.Expr {
		m := &syntax.Member{X: read, Name: opNames[u.Op]}
		m.P = u.P
		call := &syntax.Call{Fun: m, Operator: true}
		call.P = u.P
		return call
	}))
}

// reassign checks target = value(target), for a target already checked
// as an assignment target of type tt. A member target's receiver goes to
// a temporary unless it is this, a local or a name that isn't a variable
// (an object), so that it is evaluated once: a property's getter may have
// effects. So do an array element's array and indices. It returns a
// function that makes another read of the target, to check after the
// statements.
func (c *Checker) reassign(p syntax.Pos, target syntax.Expr, tt types.Type, value func(read syntax.Expr) syntax.Expr) ([]syntax.Stmt, func() syntax.Expr) {
	var stmts []syntax.Stmt
	ref := func() syntax.Expr { return target }
	if id, ok := target.(*syntax.Ident); ok {
		ref = func() syntax.Expr {
			n := &syntax.Ident{Name: id.Name}
			n.P = id.P
			return n
		}
	}
	if m, ok := target.(*syntax.Member); ok {
		stable := false
		switch x := m.X.(type) {
		case *syntax.This:
			stable = true
		case *syntax.Ident:
			_, _, local := c.scope.lookup(x.Name)
			stable = local || !c.isVar(x.Name)
		}
		switch {
		case stable:
			ref = func() syntax.Expr {
				n := &syntax.Member{X: m.X, Name: m.Name}
				n.P = m.P
				return n
			}
		default:
			recv := c.temp(p, "receiver", m.X, m.X.Type())
			stmts = append(stmts, recv)
			ref = func() syntax.Expr {
				n := &syntax.Member{X: tempRef(recv, p), Name: m.Name}
				n.P = m.P
				return n
			}
		}
	}
	if ix, ok := target.(*syntax.Index); ok {
		// An array's element: the array and the indices go to
		// temporaries, and the read and the write are distinct nodes.
		recv := c.temp(p, "array", ix.X, ix.X.Type())
		stmts = append(stmts, recv)
		var idx []*syntax.VarStmt
		for _, a := range ix.Args {
			vs := c.temp(p, "index", a, a.Type())
			idx = append(idx, vs)
			stmts = append(stmts, vs)
		}
		ref = func() syntax.Expr {
			e := &syntax.Index{X: tempRef(recv, p)}
			e.P = ix.P
			for _, vs := range idx {
				e.Args = append(e.Args, tempRef(vs, p))
			}
			return e
		}
	}
	read := ref()
	rt := c.value(read, noWant)
	a := &syntax.Assign{P: p, Op: syntax.ASSIGN, Target: ref(), Value: value(checked(read, rt))}
	c.assign(a)
	return append(stmts, a), ref
}

// opProperty reports whether the receiver recv, of type rt, has an
// extension property name, of a function type that may take args, that
// an operator's function of the name would be called for: a property
// isn't an operator, as kotlinc's NOT_FUNCTION_AS_OPERATOR.
func (c *Checker) opProperty(recv syntax.Expr, rt types.Type, name string, args ...syntax.Expr) bool {
	if rt.K == types.Invalid {
		return false
	}
	if cls := c.classOf(rt.NonNull()); cls != nil && c.hasProp(cls, name) {
		return false
	}
	probe := &syntax.Member{X: checked(recv, rt), Name: name}
	probe.P = recv.Pos()
	n := len(c.errs)
	pt, ok := c.extProp(probe, rt)
	c.errs = c.errs[:n]
	return ok && pt.IsFunc() && c.invokableWith(pt, &syntax.Call{Args: args})
}
