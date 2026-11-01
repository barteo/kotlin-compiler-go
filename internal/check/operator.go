package check

import (
	"fmt"
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
	case owner == nil && d.Recv == nil:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must be a member or an extension function")
	case n >= 0 && len(d.Params) != n && !((d.Name == "inc" || d.Name == "dec") && defaultsOnly(d.Params)):
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must have %s", countParams(n))
	case n < 0 && n != -0x7fff && len(d.Params) < -n:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must have at least %s", countParams(-n))
	case !d.Sym.Resolved:
	case d.Name == "compareTo" && d.Sym.Ret.Flex() != types.IntType:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must return Int")
	case d.Name == "contains" && d.Sym.Ret.Flex() != types.BoolType:
		c.errorf(d.P, "'operator' modifier is inapplicable on this function: must return Boolean")
	}
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
	if ft.K == types.Class && ft.C.Func || ft.K == types.Nothing || !c.hasFun(ft, "invoke", x) {
		return types.InvalidType, false
	}
	m := &syntax.Member{X: checked(x.Fun, ft), Name: "invoke"}
	m.P = x.Fun.Pos()
	// A trial's check, as a builder lambda's with its variables unfixed,
	// checks the call afresh: x's type is the trial's.
	fun, op := x.Fun, x.Operator
	c.onUndo(func() { x.Fun, x.Operator = fun, op })
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
	for i, a := range ix.Args {
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
		return stmts, elem, true
	}
	if assignOp != "" && !builtinOperand(et) && c.hasFun(et.NonNull(), assignOp, &syntax.Call{Args: []syntax.Expr{arg}}) {
		call, t := c.opCall(p, checked(read, et), assignOp, []syntax.Expr{arg}, noWant)
		c.wantUnit(p, assignOp, t)
		return append(stmts, &syntax.ExprStmt{X: call}), elem, true
	}
	set := &syntax.Assign{P: p, Op: syntax.ASSIGN, Target: elem(), Value: value(checked(read, et))}
	c.assign(set)
	return append(stmts, set), elem, true
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
func (c *Checker) compoundOp(s *syntax.Assign, tt types.Type, mutable bool) {
	assignName, opName := opNames[s.Op], opNames[arithOf(s.Op)]
	probe := &syntax.Call{Args: []syntax.Expr{s.Value}}
	hasAssign := c.hasFun(tt.NonNull(), assignName, probe)
	hasOp := c.hasFun(tt.NonNull(), opName, probe)
	switch {
	case hasAssign && hasOp && mutable:
		c.errorf(s.P, "assignment operators ambiguity: '%s' and '%s' both apply", assignName, opName)
		return
	case hasAssign:
		call, t := c.opCall(s.P, checked(s.Target, tt), assignName, []syntax.Expr{s.Value}, noWant)
		c.wantUnit(s.P, assignName, t)
		s.Stmts = []syntax.Stmt{&syntax.ExprStmt{X: call}}
		return
	case !hasOp:
		c.errorf(s.P, "operator '%s' cannot be applied to %s", s.Op, tt)
		return
	case !mutable:
		c.errorf(s.Target.Pos(), "val cannot be reassigned")
		return
	}
	op := arithOf(s.Op)
	s.Stmts, _ = c.reassign(s.P, s.Target, tt, func(read syntax.Expr) syntax.Expr {
		b := &syntax.Binary{Op: op, X: read, Y: s.Value}
		b.P = s.P
		return b
	})
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
