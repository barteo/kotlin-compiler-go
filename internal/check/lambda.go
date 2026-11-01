package check

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Lambdas and function types.
//
// A lambda is checked against an expected function type when there is
// one: it gives the parameter types (and the implicit `it`) and the
// result type. A lambda passed to a generic function is checked only
// after the call's other arguments, once the type variables in its
// parameter types are fixed; its result then constrains the rest.
//
// A lambda passed to a Java method can implement a Java interface with a
// single abstract method (SAM conversion), such as Runnable or
// Comparator. It is then checked against that method's signature.

// lambdaWant is what a lambda is checked against.
type lambdaWant struct {
	params     []types.Type // expected parameter types, if known
	known      bool         // the expected type is a function type
	recv       bool         // with a receiver, the first of params after the contexts
	ctx        int          // the number of context parameters, the first of params
	suspend    bool         // the expected type is a suspend function type
	composable bool         // the expected type is a composable function type
	ret        types.Type   // expected result type, or noWant
	check      bool         // the body must have type ret; otherwise ret is only a hint
	recover    bool         // checking after an error: unknown parameters are not reported
	prop       bool         // a callable reference must name a property: see kpropRef
	kprop      bool         // a callable reference to a property may be a KProperty object
}

// lambdaResult collects the values a lambda's labeled returns give.
type lambdaResult struct {
	label string
	want  types.Type // noWant if unknown
	check bool       // the values must have type want
	types []types.Type
	bare  bool // a return@label without a value: the result is Unit
	// postponed are the values whose type arguments only the lambda's
	// result can give: return@flatMap emptyList(), with the state where
	// they are.
	postponed []postponedRet
	// tail is the last expression of a lambda that returns Unit, checked
	// for stubInference, and tailType its type: see stubTail.
	tail     syntax.Expr
	tailType types.Type
}

type postponedRet struct {
	x  syntax.Expr
	at state
}

// useArity records that the file uses function types with n parameters,
// whose interface kc writes without the standard library.
func (c *Checker) useArity(n int) {
	if n <= types.MaxArity {
		c.file.Arities |= 1 << n
	}
}

// arityOK reports whether function types with n parameters can be
// compiled, reporting at p if not: those with more than MaxArity are
// the standard library's FunctionN.
func (c *Checker) arityOK(p syntax.Pos, n int, what string) bool {
	switch {
	case n > types.MaxVarArity:
		c.errorf(p, "%s with more than %d parameters are not supported", what, types.MaxVarArity)
	case n > types.MaxArity && c.u.ext.funcs == types.KcFunctions:
		c.errorf(p, "%s with more than %d parameters need the Kotlin standard library", what, types.MaxArity)
	default:
		return true
	}
	return false
}

// funcParts returns the parameter and result types of a function type:
// the types a lambda of that type takes. Through a projection, a
// parameter is Kotlin's captured type, which reads as its bound: a lambda
// passed to a CompletableFuture<*>'s handle takes an Any?.
func funcParts(t types.Type) ([]types.Type, types.Type, bool) {
	if t.P {
		t = t.Flex()
	}
	if !t.IsFunc() {
		return nil, types.Type{}, false
	}
	n := t.A.Len() - 1
	ps := make([]types.Type, n)
	for i := range n {
		switch a := t.A.Get(i); a.V {
		case types.Star:
			ps[i] = types.AnyType.Nullable()
		default:
			ps[i] = a.T
		}
	}
	ret := t.A.Get(n).T
	if v := t.A.Get(n).V; v == types.In || v == types.Star {
		ret = types.AnyType.Nullable()
	}
	return ps, ret, true
}

// lambda checks a lambda whose expected type, if any, has no variables
// but stubInference's: as in Kotlin, a builder's p = { f(it) } for a
// (T) -> Unit p gives T f's parameter type.
func (c *Checker) lambda(x *syntax.Lambda, want types.Type) types.Type {
	if wantsKProp(x, want) {
		return c.kpropRef(x)
	}
	proper := !want.HasVars() || c.onlyStubs(want)
	if x.Ref != nil && proper {
		if k, ok := c.kfunWant(want); ok {
			return c.kfunRef(x, want, k)
		}
	}
	w := lambdaWant{ret: noWant, kprop: c.u.ext.kotlin}
	if ps, r, ok := funcParts(want); ok && proper {
		w = lambdaWant{params: ps, known: true, recv: want.Flex().E, ctx: want.Flex().Contexts(), suspend: isSuspendFunc(want), composable: isComposableFunc(want), ret: r, check: true}
	}
	if x.Anon && !c.anonWant(x, &w) {
		return types.InvalidType
	}
	ps, r, ok := c.checkLambda(x, w)
	if !ok {
		return types.InvalidType // already reported
	}
	if x.KProp != nil {
		return c.kpropType(x, ps, r)
	}
	t := c.lambdaType(x, ps, r)
	if x.Ref != nil && c.u.ext.kotlin && (want.K == types.Invalid || want.NonNull().K == types.Any) && t.K == types.Class && len(ps) <= types.MaxArity {
		// Without an expected function type, a reference to a function
		// is a KFunctionN, a kotlin.reflect object, as with kotlinc:
		// ::f as T, or val r = ::f. A suspend function's is a
		// KSuspendFunctionN.
		if call := kfunCall(x); call != nil {
			name := "kotlin/reflect/KFunction"
			if call.Sym.Suspend {
				name = "kotlin/reflect/KSuspendFunction"
			}
			if k := c.u.ext.class(name + strconv.Itoa(len(ps))); k != nil && !k.Missing {
				c.setKFun(x, call)
				return types.GenericType(k, t.A.List)
			}
		}
	}
	c.funRef(x, t, len(ps))
	return t
}

// funRef makes reference x, of function type t with n parameters, a
// FunctionReference too, as kotlinc's, which equals the references to the
// same function, as an AdaptedFunctionReference if it is adapted.
func (c *Checker) funRef(x *syntax.Lambda, t types.Type, n int) {
	if x.Ref != nil && x.KFun == nil && c.u.ext.kotlin && !x.Inline && x.SAM == nil && !x.Composable && t.K == types.Class && n <= types.MaxArity {
		if call := kfunCall(x); call != nil && hasJVMOwner(call) {
			c.setKFun(x, call)
			x.KFunAdapted, x.KFunFlags = refAdaptation(x, call)
		}
	}
}

// samRef makes reference l, converted to a Kotlin fun interface, a
// FunctionReference too, which its object wraps, as kotlinc's: two
// objects of the interface that refer to the same function are equal. A
// Java interface's is a lambda. A suspend method's keeps the class of a
// suspend lambda converted to the interface, whose method may be a state
// machine.
func (c *Checker) samRef(l *syntax.Lambda) {
	if l.Ref == nil || l.KFun != nil || !l.SAM.Owner.Fun || !c.u.ext.kotlin || l.Inline || l.Composable {
		return
	}
	if call := kfunCall(l); call != nil && hasJVMOwner(call) {
		if l.SAM.Suspend {
			l.KFun = call
			c.onUndo(func() { l.KFun = nil })
		} else {
			c.setKFun(l, call)
		}
		l.KFunAdapted, l.KFunFlags = refAdaptation(l, call)
	}
}

// hasJVMOwner reports whether the function that call calls has a class
// for a FunctionReference to name: not an intrinsic as arrayOf.
func hasJVMOwner(call *syntax.Call) bool {
	f := call.Sym
	_, local := f.Decl.(*syntax.LocalFun)
	return f.Builtin == 0 && (call.Ctor != nil || f.Owner != nil || f.Facade != "" || local)
}

// refAdaptation reports whether reference x, which makes call, is
// adapted to its function type, and its flags: see Lambda.KFunAdapted.
func refAdaptation(x *syntax.Lambda, call *syntax.Call) (bool, int32) {
	f := call.Sym
	var flags int32
	adapted := false
	if i := slices.IndexFunc(f.Params, func(p *types.VarSym) bool { return p.Vararg }); i >= 0 {
		if !slices.ContainsFunc(call.Args, isSpread) {
			flags |= 2
		} else if len(call.Args) < len(f.Params) {
			adapted = true // defaults after the vararg
		}
	} else if len(call.Args) < len(f.Params) {
		adapted = true // defaults
	}
	if x.Suspend && !f.Suspend {
		flags |= 4 // converted to a suspend function type
	}
	if x.Ret.K == types.Unit && (f.Ret.K != types.Unit || call.Ctor != nil) {
		flags |= 8 // the result, a constructor's object too, ignored
	}
	return adapted || flags != 0, flags
}

// lambdaType returns the function type of a lambda.
func (c *Checker) lambdaType(x *syntax.Lambda, ps []types.Type, r types.Type) types.Type {
	x.Ret = r
	if n := len(ps); x.Suspend && n+1 > types.MaxVarArity {
		c.errorf(x.P, "suspend lambdas with more than %d parameters are not supported", types.MaxVarArity-1)
		return types.InvalidType
	} else if !c.arityOK(x.P, n, "lambdas") {
		return types.InvalidType
	}
	var t types.Type
	switch {
	case x.Suspend:
		t = c.u.ext.funcs.SuspendType(ps, r)
	case x.Composable && len(ps) > composableLambdaArity:
		// As kotlinc's plugin, which fails to compile them.
		c.unsupported(x.P, "compose", "composable lambdas with more than %d parameters are not supported", composableLambdaArity)
		return types.InvalidType
	case x.Composable:
		t = c.u.ext.funcs.ComposableType(ps, r)
		c.composeLambda(x)
	default:
		c.useArity(len(ps))
		t = c.u.ext.funcs.Type(ps, r)
	}
	if x.Recv != nil {
		t = t.WithReceiver()
	}
	if len(x.Ctx) > 0 {
		t = t.WithContexts(len(x.Ctx))
	}
	return t
}

// isSuspendFunc reports whether t is a suspend function type, possibly
// nullable or a platform type.
func isSuspendFunc(t types.Type) bool { return t.Flex().IsSuspendFunc() }

// isComposableFunc reports whether t is a composable function type,
// possibly nullable or a platform type.
func isComposableFunc(t types.Type) bool { return t.Flex().IsComposableFunc() }

func paramsString(ps []types.Type) string {
	switch len(ps) {
	case 0:
		return "no parameters"
	case 1:
		return "1 parameter of type " + ps[0].String()
	}
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.String()
	}
	return strconv.Itoa(len(ps)) + " parameters of types " + strings.Join(names, ", ")
}

// checkLambda declares a lambda's parameters and checks its body. It
// returns the parameter types and the result type, and false if the
// parameters don't match the expected ones.
func (c *Checker) checkLambda(x *syntax.Lambda, w lambdaWant) ([]types.Type, types.Type, bool) {
	if x.Ref != nil {
		return c.checkRef(x, w)
	}
	return c.checkLambda1(x, w)
}

func (c *Checker) checkLambda1(x *syntax.Lambda, w lambdaWant) ([]types.Type, types.Type, bool) {
	var syms []*types.VarSym
	// A receiver checked before keeps its local, which the this the check
	// wrote for its members' calls refers to: see stubInference.
	recv := x.Recv
	x.It, x.Recv = nil, nil
	x.Suspend = w.suspend || x.SuspendMod
	x.Composable = c.u.compose && (w.composable || x.Annotated)
	if w.composable && !c.u.compose {
		c.unsupported(x.P, "compose", "composable lambdas need -compose")
	}
	ok := true
	params := w.params
	// Context parameters checked before keep their locals too.
	ctx := x.Ctx
	x.Ctx = nil
	reported := false
	if w.ctx > 0 && len(params) >= w.ctx {
		for i, t := range params[:w.ctx] {
			s := &types.VarSym{Name: "$context" + strconv.Itoa(i), Type: t, Kind: types.Param}
			if reservedContext(t) && !reported {
				if i < len(x.AnonCtx) && x.AnonCtx[i].Type != nil {
					c.reservedContextError(x.AnonCtx[i].P)
				} else {
					c.reservedContextError(x.P)
				}
				reported = true
			}
			if i < len(x.AnonCtx) {
				// An anonymous function's, which it names.
				p := x.AnonCtx[i]
				if p.Name != "_" {
					s.Name = p.Name
				}
				s.Decl = p
				if p.Type == nil {
					// A lambda the checker made, which only names them.
				} else if pt := c.resolveType(p.Type); !c.subtype(t, pt) {
					c.errorf(p.P, "expected context parameter of type %s", t)
					ok = false
				}
			}
			if i < len(ctx) {
				*ctx[i], s = *s, ctx[i]
			}
			if i < len(x.AnonCtx) {
				x.AnonCtx[i].Sym = s
			}
			x.Ctx = append(x.Ctx, s)
		}
		params = params[w.ctx:]
	}
	if w.recv && len(params) > 0 && !anonRecvParam(x, w) {
		// The receiver is the first parameter, and this in the body:
		// without a label, kotlinc names it <this>.
		name := "<this>"
		if x.Label != "" {
			name = "$this$" + x.Label
		}
		x.Recv = &types.VarSym{Name: name, Type: params[0], Kind: types.Param}
		if recv != nil && recv.Name == name {
			*recv, x.Recv = *x.Recv, recv
		}
		params = params[1:]
	}
	c.destructureParams(x)
	switch {
	case x.Arrow:
		if w.known && len(x.Params) != len(params) {
			c.errorf(x.P, "expected %s, but %d were declared", paramsString(params), len(x.Params))
			ok = false
		}
		for i, p := range x.Params {
			t := types.InvalidType
			switch {
			case p.Type != nil:
				t = c.resolveType(p.Type)
				if w.known && i < len(params) && !c.subtype(params[i], t) {
					c.errorf(p.P, "expected parameter of type %s", params[i])
					ok = false
				}
			case w.known && i < len(params):
				t = params[i]
			case !w.recover && ok:
				c.errorf(p.P, "cannot infer a type for this parameter. Please specify it explicitly.")
			}
			p.Sym = &types.VarSym{Name: p.Name, Type: t, Kind: types.Param, Decl: p}
			syms = append(syms, p.Sym)
		}
	case w.recover:
		x.It = &types.VarSym{Name: "it", Type: types.InvalidType, Kind: types.Param}
	case w.known && len(params) == 1:
		x.It = &types.VarSym{Name: "it", Type: params[0], Kind: types.Param}
	case w.known && len(params) > 1:
		c.errorf(x.P, "expected %s, but 0 were declared", paramsString(params))
		x.It = &types.VarSym{Name: "it", Type: types.InvalidType, Kind: types.Param}
		ok = false
	}
	if x.It != nil {
		syms = append(syms, x.It)
	}

	if x.Suspend && !x.Inline && x.Class == "" {
		// A suspend lambda is an object of a class of its own, and so is
		// a reference passed for a suspend function type: results::add
		// for onEach's action.
		x.Class = c.anonName()
		c.onUndo(func() { x.Class = "" })
	}
	saved := c.state
	if x.Class != "" {
		c.lambdaClass, c.declName = x.Class, ""
	}
	c.lambdas = append(c.lambdas[:len(c.lambdas):len(c.lambdas)], x)
	c.scope = &scope{parent: saved.scope, depth: len(c.lambdas)}
	for _, s := range syms {
		if s.Name != "_" && !c.scope.declare(s) {
			c.errorf(s.Decl.(*syntax.LambdaParam).P, "conflicting declarations: %s", s.Name)
		}
	}
	for _, p := range x.AnonCtx {
		if p.Name != "_" && p.Sym != nil && !c.scope.declare(p.Sym) {
			c.errorf(p.P, "conflicting declarations: %s", p.Name)
		}
	}
	c.pushContexts(x.Ctx, x)
	if x.Recv != nil {
		c.recvs = append(c.recvs[:len(c.recvs):len(c.recvs)], implicitRecv{sym: x.Recv, label: x.Label, depth: len(c.lambdas), owner: x})
		syms = append([]*types.VarSym{x.Recv}, syms...)
	}
	syms = append(x.Ctx[:len(x.Ctx):len(x.Ctx)], syms...)
	if !x.Once {
		// As in a loop, the vars it assigns lose their narrowing at its
		// start: it may run again after it assigns them.
		c.reset(c.watchedVars(x.Body))
	}
	if !x.Inline {
		// An inlined lambda's body is part of the enclosing function: return,
		// break and continue can leave it, and a try in it is in the
		// arguments of a constructor call that the lambda is.
		c.retAllowed, c.loops, c.ctorArgs = false, nil, false
	}
	c.allowRange = false
	lr := &lambdaResult{label: x.Label, want: w.ret, check: w.check}
	c.lrets = append(c.lrets[:len(c.lrets):len(c.lrets)], lr)
	unit := w.ret.K == types.Unit
	var bt types.Type
	if unit {
		if len(c.stubs) > 0 {
			lr.tail = tailExpr(x.Body)
		}
		bt = c.unitBody(x.Body)
		c.stubTail(lr)
	} else {
		bt = c.lambdaBody(x.Body, w.ret)
	}
	if len(lr.postponed) > 0 {
		want := bt
		for _, t := range lr.types {
			if j, ok := c.join(want, t); ok {
				want = j
			}
		}
		if want.K == types.Nothing {
			want = noWant
		}
		for _, p := range lr.postponed {
			c.state = p.at
			if tail := c.tailSink; tail != nil && bt.HasVars() && tail.owns(bt) {
				// The call the lambda ends with is inferred with the call
				// it is passed to (see checkTail), and so is this value.
				c.sink = tail
				tail.sub(c.value(p.x, noWant), bt)
				continue
			}
			lr.types = append(lr.types, c.value(p.x, want))
		}
	}
	c.state = saved
	if lr.bare && !(w.check && w.ret.K != types.Invalid) {
		// As kotlinc infers it, the last expression's value is dropped.
		unit = true
	}
	if !unit && !(w.check && w.ret.K != types.Invalid) {
		for _, t := range lr.types {
			j, ok := c.join(bt, t)
			if !ok {
				c.errorf(x.P, "type mismatch: lambda results have incompatible types %s and %s", bt, t)
			}
			bt = j
		}
	}

	ps := make([]types.Type, len(syms))
	for i, s := range syms {
		ps[i] = s.Type
	}
	switch {
	case unit:
		return ps, types.UnitType, ok
	case w.ret.K != types.Invalid && w.check:
		c.assignable(bt, w.ret, resultPos(x))
		return ps, w.ret, ok
	}
	return ps, bt, ok
}

// tailExpr returns the expression b ends with, if it is one whose value
// exprStmt drops as is: not an if, when, try or increment.
func tailExpr(b *syntax.Block) syntax.Expr {
	if n := len(b.Stmts); n > 0 {
		if es, ok := b.Stmts[n-1].(*syntax.ExprStmt); ok {
			switch x := es.X.(type) {
			case *syntax.If, *syntax.When, *syntax.Try:
			case *syntax.Unary:
				if x.Op != syntax.INC && x.Op != syntax.DEC {
					return x
				}
			default:
				return x
			}
		}
	}
	return nil
}

// stubTail records, as Kotlin does, that the value a lambda returning
// Unit ends with, of a stub's type, is a Unit, if that agrees with what
// the run of stubInference's recorded so far: build { getTypeVariable() }
// gives PTV Unit, and buildList { add(1); removeAt(0) } E Int.
func (c *Checker) stubTail(lr *lambdaResult) {
	t := lr.tailType
	if t.K == types.Invalid {
		return
	}
	var r *stubRun
	t.Has(func(u types.Type) bool {
		if u.K == types.TypeVar && c.stubs[u.Param()] != nil {
			r = c.stubs[u.Param()]
			return true
		}
		return false
	})
	if r != nil {
		r.add(t, types.UnitType)
	}
}

// lambdaBody checks a lambda's body. Its value is the last statement's,
// or Unit if that is not an expression with a value.
func (c *Checker) lambdaBody(b *syntax.Block, want types.Type) types.Type {
	valueUsed := false
	if n := len(b.Stmts); n > 0 {
		if es, ok := b.Stmts[n-1].(*syntax.ExprStmt); ok {
			valueUsed = true
			switch e := es.X.(type) {
			case *syntax.If:
				valueUsed = hasFinalElse(e)
			case *syntax.When:
				valueUsed = len(e.Branches) > 0 && e.Branches[len(e.Branches)-1].Else
				if !valueUsed && e.Subject != nil && want.K != types.Unit {
					// An exhaustive when over an enum, a sealed class or
					// a Boolean is a value too.
					e.Soft, valueUsed = true, true
				}
			case *syntax.Unary:
				// { x++ } is the old value, unless the lambda returns Unit.
				valueUsed = e.Op != syntax.INC && e.Op != syntax.DEC || want.K != types.Unit
			}
		}
	}
	t := c.block(b, want, valueUsed)
	if !valueUsed && t.K != types.Nothing {
		return types.UnitType
	}
	return t
}

// unitBody checks the body of a lambda that returns Unit. Its last
// statement, an if with an else whose branches end in values, is a
// value that the lambda drops, whose branches are inferred together:
// if (c) listOf(x) else emptyList().
func (c *Checker) unitBody(b *syntax.Block) types.Type {
	if n := len(b.Stmts); n > 0 {
		if es, ok := b.Stmts[n-1].(*syntax.ExprStmt); ok {
			if e, ok := es.X.(*syntax.If); ok && valueBranches(e) {
				if t := c.block(b, noWant, true); t.K == types.Nothing {
					return t
				}
				return types.UnitType
			}
		}
	}
	return c.block(b, noWant, false)
}

// valueBranches reports whether each branch of an if ends in a value:
// an expression other than an assignment, a when, or an if without an
// else.
func valueBranches(e *syntax.If) bool {
	for _, b := range []*syntax.Block{e.Then, e.Else} {
		if b == nil || len(b.Stmts) == 0 {
			return false
		}
		es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt)
		if !ok {
			return false
		}
		switch x := es.X.(type) {
		case *syntax.When:
			return false
		case *syntax.If:
			if x.Else == nil || !valueBranches(x) {
				return false
			}
		}
	}
	return true
}

// hasFinalElse reports whether an if, and each else if after it, has an
// else: if (a) x else if (b) y is a statement, not a value.
func hasFinalElse(e *syntax.If) bool {
	for e.Else != nil {
		if len(e.Else.Stmts) != 1 {
			return true
		}
		es, ok := e.Else.Stmts[0].(*syntax.ExprStmt)
		if !ok {
			return true
		}
		if e, ok = es.X.(*syntax.If); !ok {
			return true
		}
	}
	return false
}

// resultPos is where a lambda's result is reported: its last expression.
func resultPos(x *syntax.Lambda) syntax.Pos {
	if n := len(x.Body.Stmts); n > 0 {
		if es, ok := x.Body.Stmts[n-1].(*syntax.ExprStmt); ok {
			return es.X.Pos()
		}
	}
	return x.P
}

// labeledReturn checks return@label. It reports false if the label names
// the enclosing function, so that this is a plain return. A return from an
// outer lambda or the function can leave only inlined lambdas.
func (c *Checker) labeledReturn(x *syntax.Jump) bool {
	i := len(c.lambdas) - 1
	for i >= c.base && c.lambdas[i].Label != x.Label {
		i--
	}
	if i < c.base {
		i = -1 // not in the function being checked
	}
	fail := func() bool {
		if x.X != nil {
			c.value(x.X, noWant)
		}
		return true
	}
	switch {
	case i < 0 && c.fn != nil && c.fn.Name == x.Label:
		if !c.leave(c.base, x) {
			return fail()
		}
		return false
	case i < 0:
		c.errorf(x.P, "unresolved label: %s", x.Label)
		return fail()
	case !c.leave(i+1, x):
		return fail()
	}
	lr := c.lrets[i]
	x.Lambda = c.lambdas[i]
	switch {
	case lr.want.K == types.Unit:
		if x.X != nil {
			// A builder's variable is Unit then: return@build materialize().
			if t := c.value(x.X, noWant); t.K != types.Invalid && !c.subtype(t, types.UnitType) {
				c.errorf(x.X.Pos(), "type mismatch: inferred type is %s but Unit was expected", t)
			}
		}
	case x.X == nil:
		if lr.check && lr.want.K != types.Invalid {
			c.errorf(x.P, "this lambda must return a value of type %s", lr.want)
		}
		lr.types = append(lr.types, types.UnitType)
		lr.bare = true
	default:
		n := len(c.errs)
		t := c.value(x.X, lr.want)
		switch {
		case lr.check && lr.want.K != types.Invalid:
			c.assignable(t, lr.want, x.X.Pos())
		case len(c.errs) > n && simpleExpr(x.X):
			// As kotlinc postpones its type variables, the value is
			// checked against the lambda's other results.
			c.errs = c.errs[:n]
			lr.postponed = append(lr.postponed, postponedRet{x.X, c.state})
		default:
			lr.types = append(lr.types, t)
		}
	}
	return true
}

// inlineJumps says whether a non-local return, break or continue can leave
// an inlined lambda: not if the function takes it as crossinline, or
// invokes it in a try, whose finally block kc would have to copy, or is
// composable, whose groups it would leave open.
type inlineJumps struct {
	callee      string
	crossinline bool
	suspends    bool // the callee is a suspend function
	inTry       bool
	composable  bool
}

// leave checks a jump that leaves the lambdas c.lambdas[from:]: only
// inlined ones, that allow it, can be left. It reports the error.
func (c *Checker) leave(from int, x *syntax.Jump) bool {
	for i := len(c.lambdas) - 1; i >= from; i-- {
		l := c.lambdas[i]
		what := x.Kind.String()
		if x.Label != "" {
			what += "@" + x.Label
		}
		j := c.jumps[l]
		switch {
		case !l.Inline && x.Label != "":
			c.errorf(x.P, "'%s' is not allowed here: it would leave the innermost lambda", what)
		case !l.Inline:
			c.errorf(x.P, "'%s' is not allowed here", what)
		case j.crossinline:
			c.errorf(x.P, "'%s' is not allowed here: '%s' takes the lambda as crossinline", what, j.callee)
		case j.inTry:
			c.unsupported(x.P, "non-local jump from a try", "a non-local %s from '%s' isn't supported: kc doesn't copy its finally block", x.Kind, j.callee)
		case j.composable:
			c.unsupported(x.P, "compose", "non-local jumps from the lambdas of inline composables are not supported yet")
		default:
			continue
		}
		return false
	}
	return true
}

// capture records that the lambdas inside scope depth use v. An inlined
// lambda's body is part of the enclosing function: it captures nothing.
func (c *Checker) capture(v *types.VarSym, depth int) {
	if depth >= len(c.lambdas) {
		return
	}
	for _, l := range c.lambdas[depth:] {
		switch {
		case !l.Inline && !slices.Contains(l.Captures, v):
			c.addCapture(l, v)
		case l.Inline && c.jumps[l].suspends:
			// A suspend inline function may save the locals in the
			// continuation before it invokes the lambda, as
			// suspendCoroutine does in suspendCoroutineUninterceptedOrReturn's:
			// a var the lambda uses is shared, as kotlinc's Ref, which
			// it keeps when the continuation saves it, is.
			v.Captured = true
		}
	}
}

// captureThis records that the enclosing lambdas use this.
func (c *Checker) captureThis() {
	for _, l := range c.lambdas[min(c.thisBase, len(c.lambdas)):] {
		if !l.Inline {
			c.setThis(l)
		}
	}
}

// addCapture records that lambda or frame l captures v.
func (c *Checker) addCapture(l *syntax.Lambda, v *types.VarSym) {
	if c.trial != nil {
		n, was := len(l.Captures), v.Captured
		c.onUndo(func() { l.Captures, v.Captured = l.Captures[:n], was })
	}
	v.Captured = true
	l.Captures = append(l.Captures, v)
}

// setThis records that lambda or frame l captures this.
func (c *Checker) setThis(l *syntax.Lambda) {
	if !l.This {
		c.onUndo(func() { l.This = false })
	}
	l.This = true
}

// invoke checks a call of a value of function type, f(x).
func (c *Checker) invoke(x *syntax.Call, want types.Type, sink *system) types.Type {
	return c.invokeValue(x, c.value(x.Fun, noWant), want, sink)
}

// invokeValue checks a call of x.Fun, already checked to have type ft.
func (c *Checker) invokeValue(x *syntax.Call, ft, want types.Type, sink *system) types.Type {
	if ft.K == types.Invalid {
		c.args(x, nil)
		return ft
	}
	if !ft.IsFunc() {
		if !types.MayBeNull(ft) || ft.P {
			if t, ok := c.invokeOp(x, ft.NonNull(), want, sink); ok {
				return t
			}
		}
		if id, ok := x.Fun.(*syntax.Ident); ok {
			c.errorf(x.Fun.Pos(), "expression '%s' of type %s cannot be invoked as a function", id.Name, ft)
		} else {
			c.errorf(x.Fun.Pos(), "expression of type %s cannot be invoked as a function", ft)
		}
		c.args(x, nil)
		return types.InvalidType
	}
	if types.MayBeNull(ft) && !ft.P {
		c.errorf(x.Fun.Pos(), "reference has a nullable type '%s', use explicit '?.invoke()' to make a function-like call instead", ft)
		c.args(x, nil)
		return types.InvalidType
	}
	rt := ft.NonNull()
	if x.Recv && !rt.IsExtFunc() {
		c.errorf(x.P, "no receiver can be passed to a value of type %s", ft)
		c.args(x, nil)
		return types.InvalidType
	}
	c.implicitArg(x, rt.Flex())
	x.Invoke = true
	c.useArity(rt.A.Len() - 1)
	if len(x.TypeArgs) > 0 {
		c.errorf(x.TypeArgs[0].P, "no type arguments expected for 'invoke'")
		x.TypeArgs = nil
	}
	return c.resolve(x, methodCallees(rt.C.Methods["invoke"], func(*types.FunSym) types.Subst { return classSubst(rt) }), want, sink)
}

// sam returns the single abstract method of an interface of the class
// path or a fun interface of the program, or nil if cls is not one.
func (c *Checker) sam(cls *types.ClassSym) *types.FunSym {
	if !cls.External && !cls.Fun {
		return nil
	}
	cls.Ensure()
	if !cls.Interface || cls.Missing {
		return nil
	}
	return cls.SAM(findSAM)
}

// The methods of Object that interfaces may redeclare as abstract.
var objectMethods = map[string]bool{"equals(Ljava/lang/Object;)": true, "hashCode()": true, "toString()": true}

func findSAM(cls *types.ClassSym) *types.FunSym {
	ms := abstractMethods(cls)
	if len(ms) != 1 {
		return nil
	}
	if f := ms[0]; !f.Unsupported && len(f.TypeParams) == 0 && len(f.Params) <= types.MaxArity {
		return f
	}
	return nil
}

// abstractMethods returns the abstract methods of an interface and its
// superinterfaces, but those of Object and those a subinterface
// implements.
func abstractMethods(cls *types.ClassSym) []*types.FunSym {
	var out []*types.FunSym
	seen := map[string]bool{}
	cls.Supertypes(func(s *types.ClassSym) bool {
		// And the member extensions of the class path's: Compose's
		// MeasurePolicy's MeasureScope.measure.
		for _, methods := range []map[string][]*types.FunSym{s.Methods, s.ExtMethods} {
			for name, fs := range methods {
				for _, f := range fs {
					// A method of the class path by its parameter types, one
					// of the program, which has no descriptor yet, by their
					// number.
					key := name + f.Desc[:strings.IndexByte(f.Desc, ')')+1]
					if f.Desc == "" {
						key = fmt.Sprintf("%s/%d", name, len(f.Params))
					}
					if seen[key] {
						continue // overridden in a subinterface
					}
					seen[key] = true
					if f.Abstract && !objectMethods[key] {
						out = append(out, f)
					}
				}
			}
		}
		return true
	})
	return out
}

// checkFunInterface reports what kotlinc reports of a fun interface: it
// must have one abstract method, without type parameters or default
// values, and no abstract properties.
func (c *Checker) checkFunInterface(d *syntax.ClassDecl) {
	props := false
	d.Sym.Supertypes(func(s *types.ClassSym) bool {
		for _, p := range s.Props {
			props = props || p.Abstract
		}
		return true
	})
	if props {
		c.errorf(d.P, "fun interfaces cannot have abstract properties")
	}
	ms := abstractMethods(d.Sym)
	if len(ms) != 1 {
		c.errorf(d.P, "fun interfaces must have exactly one abstract method")
		return
	}
	f := ms[0]
	if len(f.TypeParams) > 0 {
		c.errorf(d.P, "single abstract member cannot declare type parameters")
	}
	for _, p := range f.Params {
		if p.Default {
			c.errorf(d.P, "single abstract member cannot declare default values")
			break
		}
	}
}

// samTarget returns the abstract method of an interface type that a
// lambda passed for it converts to, and the function type the lambda has:
// a Java interface with a single abstract method, or a fun interface. As
// in Kotlin, other Kotlin interfaces aren't converted, as mockk's Answer,
// for which answers { ... } takes the function type overload.
func (c *Checker) samTarget(p types.Type) (*types.FunSym, types.Type) {
	t := p.NonNull()
	if t.K != types.Class {
		return nil, types.InvalidType
	}
	m := c.sam(t.C)
	if m == nil || !t.C.Fun && !(t.C.External && t.C.KotlinName == "") {
		return nil, types.InvalidType
	}
	sub := c.memberSubst(t, m.Owner)
	ps := make([]types.Type, len(m.Params))
	for i, q := range m.Params {
		ps[i] = lambdaParam(sub, q.Type)
	}
	if m.Ext {
		// A member extension: the lambda's receiver is its receiver.
		ps = append([]types.Type{lambdaParam(sub, m.Recv)}, ps...)
	}
	var ctx []types.Type // and its context parameters, its own
	for _, q := range m.Contexts {
		ctx = append(ctx, lambdaParam(sub, q.Type))
	}
	ps = append(ctx, ps...)
	var ft types.Type
	if m.Suspend {
		ft = c.u.ext.funcs.SuspendType(ps, sub.Apply(m.Ret))
	} else {
		ft = c.u.ext.funcs.Type(ps, sub.Apply(m.Ret))
	}
	if m.Ext {
		ft = ft.WithReceiver()
	}
	return m, ft.WithContexts(len(ctx))
}

// lambdaParam returns the type of a lambda's parameter for a parameter
// of type t of a method that sub substitutes in. Through a projection
// for t, the lambda takes Kotlin's captured type, which reads as the
// projection's type or, for a star, t's bound: a Consumer<*>'s lambda
// takes an Any?, and a Consumer<out CharSequence>'s a CharSequence.
func lambdaParam(sub types.Subst, t types.Type) types.Type {
	if t.IsParam() {
		if a, ok := sub[t.Param()]; ok && (a.V == types.Star || a.V == types.Out) {
			return sub.Apply(t)
		}
	}
	return sub.ApplyIn(t)
}

// samValue reports whether a function value of type t, not a lambda,
// converts to p, a Kotlin fun interface or a Java interface, as kotlinc
// converts one passed for it: collect(handler), for handler: suspend (T)
// -> Unit, passes a FlowCollector<T>. A null function value is a null
// interface, where p takes null. It returns the interface's method.
func (c *Checker) samValue(t, p types.Type) (*types.FunSym, bool) {
	pt := p.NonNull()
	if !samInterface(pt) || types.MayBeNull(t) && !types.AcceptsNull(p) || c.subtype(t, p) {
		return nil, false
	}
	m, ft := c.samTarget(pt)
	if m == nil {
		return nil, false
	}
	vt, ok := c.funcView(t, len(m.Params))
	if !ok {
		return nil, false
	}
	if m.Suspend && vt.NonNull().IsFunc() && !vt.NonNull().C.Suspend {
		ft = c.unsuspended(ft) // converted to the suspend type too: see samConverted
	}
	return m, m != nil && !m.Composable && !m.Ext && c.subtype(t.NonNull(), ft)
}

// samConverted converts checked function value a to fun interface p, of
// method m (see samValue): a value of a function type that isn't suspend,
// for a suspend method, after a suspend conversion (see suspendConverted).
func (c *Checker) samConverted(a syntax.Expr, m *types.FunSym, p types.Type) syntax.Expr {
	a = c.asFunc(a, m)
	if m.Suspend && !isSuspendFunc(a.Type()) {
		_, ft := c.samTarget(p)
		if types.MayBeNull(a.Type()) {
			ft = ft.Nullable()
		}
		a = c.suspendConverted(a, ft)
	}
	ch := checked(a, p.NonNull())
	ch.SAM = m
	return ch
}

// funcView returns the function type that t is, or that a class type t
// or a type parameter's bound implements, with n parameters if n >= 0:
// an object : () -> Unit is a () -> Unit, and so is a T : () -> Unit,
// (Int) -> Unit. It reports false if t is neither.
func (c *Checker) funcView(t types.Type, n int) (types.Type, bool) {
	t = t.Flex()
	if t.IsFunc() {
		return t, true
	}
	if t.NonNull().K == types.TypeParam {
		for _, b := range t.NonNull().Param().Bounds {
			if s, ok := c.funcView(b, n); ok && (n < 0 || s.NonNull().A.Len() == n+1) {
				if types.MayBeNull(t) {
					s = s.Nullable()
				}
				return s, true
			}
		}
		return t, false
	}
	if t.NonNull().K != types.Class {
		return t, false
	}
	for _, s := range c.supertypes(t.NonNull()) {
		if s.IsFunc() && (n < 0 || s.A.Len() == n+1) {
			if types.MayBeNull(t) {
				s = s.Nullable()
			}
			return s, true
		}
	}
	return t, false
}

// asFunc returns checked value a as the function type its class
// implements (see funcView), which a SAM conversion wraps.
func (c *Checker) asFunc(a syntax.Expr, m *types.FunSym) syntax.Expr {
	if t := a.Type(); !t.Flex().IsFunc() {
		if ft, ok := c.funcView(t, len(m.Params)); ok {
			return checked(a, ft)
		}
	}
	return a
}

// funInterface reports whether t is a Kotlin fun interface type.
func funInterface(t types.Type) bool {
	if t.K != types.Class {
		return false
	}
	t.C.Ensure()
	return t.C.Fun
}

// samInterface reports whether a function value converts to an interface
// of type t: a Kotlin fun interface, or a Java interface.
func samInterface(t types.Type) bool {
	return funInterface(t) || t.K == types.Class && t.C.External && t.C.Interface && t.C.KotlinName == ""
}

// samArgs converts the function values a call passes for fun interfaces
// (see samValue).
func (c *Checker) samArgs(x *syntax.Call, ts []types.Type, param func(int) types.Type) {
	for i, a := range x.Args {
		if _, ok := a.(*syntax.Lambda); ok || i >= len(ts) {
			continue
		}
		if m, ok := c.samValue(ts[i], param(i)); ok {
			x.Args[i] = c.samConverted(a, m, param(i))
		}
	}
}

// argValue checks argument *arg for a parameter of type p of f (nil: no
// expected type). Lambdas may be SAM-converted (see samTarget): one
// converted to a fun interface whose method is composable is replaced by
// its object (see composableSAM). Other function values may be converted
// to suspend function types (see suspendConverted).
func (c *Checker) argValue(arg *syntax.Expr, p types.Type, f *types.FunSym) types.Type {
	a := *arg
	l, ok := a.(*syntax.Lambda)
	switch {
	case !ok:
		t := c.value(a, p)
		if _, ok := c.suspendConstraint(t, p, a); ok && f != nil {
			*arg = c.suspendConverted(a, p)
			return p
		}
		return t
	case f == nil:
		c.checkLambda(l, lambdaWant{ret: noWant, recover: true})
		return types.InvalidType
	default:
		if m, ft := c.samTarget(p); m != nil {
			if m.Composable {
				obj := composableSAM(l, m, ft, p)
				if obj == nil {
					c.unsupported(l.P, "compose", "lambdas converted to fun interfaces with composable functions are not supported")
					return types.InvalidType
				}
				*arg = obj
				return c.value(obj, p)
			}
			ps, r, _ := funcParts(ft)
			_, r, _ = c.checkLambda(l, lambdaWant{params: ps, known: true, recv: ft.E, ctx: ft.Contexts(), suspend: m.Suspend, ret: r, check: true})
			l.SAM, l.Ret, l.T = m, r, p
			c.samRef(l)
			return p
		}
	}
	return c.value(a, p)
}

// composableSAM returns the object expression that lambda l is, passed
// for a parameter of fun interface type p, whose method m, of function
// type ft, is composable, as kotlinc's plugin compiles it: an object whose
// composable override of m has the lambda's body,
//
//	object : P { @Composable override fun m(params) { body } }
//
// or nil if kc doesn't compile it so: if m returns a value, or the body
// names this or returns from the lambda, which the object would change.
func composableSAM(l *syntax.Lambda, m *types.FunSym, ft, p types.Type) *syntax.ObjectExpr {
	ps, r, _ := funcParts(ft)
	if r.K != types.Unit || r.N || ft.E {
		return nil
	}
	names := []string{}
	switch {
	case !l.Arrow && len(ps) == 1:
		names = append(names, "it")
	case len(l.Params) != len(ps):
		return nil
	default:
		for _, lp := range l.Params {
			if lp.Names != nil {
				return nil // destructured
			}
			names = append(names, lp.Name)
		}
	}
	ok := true
	syntax.Inspect(l.Body, func(n any) bool {
		switch n := n.(type) {
		case *syntax.This:
			ok = ok && n.Label != "" && n.Label != l.Label
		case *syntax.Jump:
			ok = ok && (n.Kind != syntax.RETURN || n.Label != "" && n.Label != l.Label)
		}
		return ok
	})
	if !ok {
		return nil
	}
	known := func(t types.Type) *syntax.TypeRef { return &syntax.TypeRef{P: l.P, Known: &t} }
	fd := &syntax.FunDecl{P: l.P, Name: m.Name, Body: l.Body,
		Annotations: []syntax.Annotation{{P: l.P, Name: "androidx.compose.runtime.Composable"}}}
	fd.Override = true
	for i, name := range names {
		fd.Params = append(fd.Params, &syntax.Param{P: l.P, Name: name, Type: known(ps[i])})
	}
	cd := &syntax.ClassDecl{P: l.P, Anonymous: true, HasPrimary: true,
		Supers: []*syntax.SuperType{{Type: known(p.NonNull())}}, Members: []syntax.Decl{fd}}
	x := &syntax.ObjectExpr{Decl: cd}
	x.P = l.P
	return x
}

// lambdaFits reports whether a lambda can be passed for a parameter of
// type p, judging by its number of parameters.
func (c *Checker) lambdaFits(l *syntax.Lambda, p types.Type) bool {
	t := p.NonNull()
	n := 0
	switch {
	case t.K == types.TypeVar || t.K == types.TypeParam || t.K == types.Any || wantsKProp(l, t):
		return true // list += { }, MutableCollection<in T>.plusAssign(element: T)
	case t.IsFunc():
		n = t.A.Len() - 1 - t.Contexts() // nor are its contexts
		if t.E {
			n-- // the receiver isn't declared
		}
	default:
		m, _ := c.samTarget(t)
		if m == nil {
			return false
		}
		n = len(m.Params)
	}
	switch {
	case l.Ref != nil:
		return true // a reference takes the parameters it is given
	case l.Arrow:
		return len(l.Params) == n
	}
	return n <= 1
}

func (c *Checker) lambdasFit(cd *candidate, x *syntax.Call) bool {
	for i, a := range x.Args {
		if l, ok := a.(*syntax.Lambda); ok {
			if p := cd.sys.apply(cd.params[i]); !c.lambdaFits(l, p) || !c.lambdaMayBe(cd, l, p) {
				return false
			}
		}
	}
	return true
}

// lambdaMayBe reports whether lambda l, for a parameter of type p of
// candidate cd, can be of a function type that p allows, as kotlinc
// decides: f(TypeToken<String>(), { }) doesn't call f(type: TypeToken<C>,
// value: C), whose C is a String, and f<C>(TypeToken<C>(), { }) in code
// with a type parameter C doesn't either. Of a variable, it tries the
// known upper bounds with the function types of the arities l may have,
// suspend or not.
func (c *Checker) lambdaMayBe(cd *candidate, l *syntax.Lambda, p types.Type) bool {
	v := p.NonNull()
	if l.Ref != nil {
		return true
	}
	if v.K == types.TypeParam {
		// A type parameter of the code around, not the callee's.
		q := v.Param()
		return slices.Contains(cd.tparams, q) || cd.f.Owner != nil && slices.Contains(cd.f.Owner.TypeParams, q)
	}
	s := cd.sys
	if v.K != types.TypeVar || s.byVar[v.Param()] == nil {
		return true
	}
	tv := s.byVar[v.Param()]
	arities := []int{len(l.Params), len(l.Params) + 1} // and a receiver
	if !l.Arrow && !l.Anon {
		arities = []int{0, 1, 2} // it, and a receiver
	}
	for _, u := range tv.upper {
		if u = s.apply(u); u.HasVars() || u.K == types.Invalid {
			continue
		}
		fits := false
		for _, n := range arities {
			ps := make([]types.Type, n)
			for i := range ps {
				ps[i] = types.AnyType.Nullable()
			}
			// Of a function type, the subtype of all those of its arity.
			if c.subtype(c.u.ext.funcs.Type(ps, types.NothingType), u) || c.subtype(c.u.ext.funcs.SuspendType(ps, types.NothingType), u) {
				fits = true
				break
			}
		}
		if !fits {
			return false
		}
	}
	return true
}

// pendingLambda is a lambda argument for a parameter of type p.
type pendingLambda struct {
	l *syntax.Lambda
	p types.Type
	// scope is the call's, whose smart casts the lambda sees when it is
	// checked with the call it is an argument of: x's in x?.let { { f(x) } }.
	scope *scope
}

// inputs returns the parameter types a lambda will be checked with.
func (c *Checker) inputs(s *system, pl pendingLambda) []types.Type {
	pt := s.apply(pl.p)
	if !pt.Flex().IsFunc() {
		if m, ft := c.samTarget(pt); m != nil {
			pt = ft
		}
	}
	ps, _, _ := funcParts(pt)
	return ps
}

// endsInLambda reports whether lambda l returns a lambda it ends with.
func endsInLambda(l *syntax.Lambda) bool {
	if l.Body == nil || len(l.Body.Stmts) == 0 {
		return false
	}
	es, ok := l.Body.Stmts[len(l.Body.Stmts)-1].(*syntax.ExprStmt)
	if !ok {
		return false
	}
	_, ok = es.X.(*syntax.Lambda)
	return ok
}

// lambdaResultOpen reports whether a pending lambda's result is a variable
// without a known upper bound.
func (c *Checker) lambdaResultOpen(s *system, pl pendingLambda) bool {
	pt := s.apply(pl.p)
	if !pt.Flex().IsFunc() {
		if m, ft := c.samTarget(pt); m != nil {
			pt = ft
		}
	}
	_, r, ok := funcParts(pt)
	r = s.apply(r)
	return ok && r.K == types.TypeVar && s.upperHint(r).K == types.Invalid
}

// runLambdas checks the lambdas pending in s. If readyOnly, it leaves
// those whose parameter types are not known yet.
func (c *Checker) runLambdas(s *system, readyOnly bool) {
	var rest []pendingLambda
	for _, pl := range s.lambdas {
		if tv := s.unfixedVar(s.apply(pl.p)); tv != nil && slices.ContainsFunc(append(tv.lower[:len(tv.lower):len(tv.lower)], tv.upper...), s.proper) && !ownLambda(s, tv, pl.l) {
			// A lambda passed for a variable takes its type once known:
			// list += { a, b -> } for MutableCollection<in T>'s T.
			s.fix(tv)
		}
		ready := true
		if readyOnly {
			for _, p := range c.inputs(s, pl) {
				ready = ready && s.known(p)
			}
			// A lambda it returns needs the result's expected type, which
			// the call it is an argument of may give: onCancellation?.let
			// { { cause, _, _ -> } } passed for a function type.
			ready = ready && !(endsInLambda(pl.l) && c.lambdaResultOpen(s, pl))
		}
		if !ready {
			rest = append(rest, pl)
			continue
		}
		saved := c.scope
		c.scope = pl.scope
		c.postponedLambda(s, pl.l, pl.p)
		c.scope = saved
	}
	s.lambdas = rest
}

// ownLambda reports whether lambda l, passed for variable tv, has the type
// that tv's proper bounds would give it: l takes no parameters, and the
// bounds are lower bounds of () -> X. tv is then above the lambdas' own
// types, () -> Number & Comparable<*> for select({ 1 }, { 1f }), not the
// first's.
func ownLambda(s *system, tv *tvar, l *syntax.Lambda) bool {
	if len(l.Params) > 0 || slices.ContainsFunc(tv.upper, s.proper) {
		return false
	}
	for _, t := range tv.lower {
		if t = s.apply(t); !t.HasVars() && !noInputs(t) {
			return false
		}
	}
	return true
}

// noInputs reports whether t is a function type without parameters or a
// receiver: () -> X, the type a lambda without parameters has on its own.
func noInputs(t types.Type) bool {
	return t.IsFunc() && !t.E && !t.IsSuspendFunc() && t.A.Len() == 1
}

// stubInference infers the variables of the parameter types of lambda l,
// its receiver's among them, that no proper type bounds, by checking l
// with them unfixed, as Kotlin's builder inference does: where the body
// uses a value of their types as another type, or another type's value
// as one of them, the subtyping (see subtype1 and system.sub) constrains
// the variable instead of failing. build { yield(x) } gives Buildee<T>'s
// T the type of x, consume(materialize()) the type consume takes, val b:
// Buildee<X> = this X, and suspendCancellableCoroutine { cont ->
// cont.resume(x) } the type of x. The generic calls in l whose variables
// they relate to are inferred with them (see joinRun), and a candidate
// whose constraints contradict the ones recorded so far isn't applicable
// (see stubRun.add). A member of a value of one of their types fixes the
// variable to what those constraints give (see fixedStub), and the value
// a lambda returning Unit ends with makes it Unit (see stubTail). The
// check's errors and the types it gives the tree are discarded, and what
// it changes outside l undone (see trial): l is checked again once the
// variables are fixed.
func (c *Checker) stubInference(s *system, l *syntax.Lambda, params []types.Type, ft types.Type) {
	group := []stubLambda{{l, params, ft, c.scope}}
	vars := c.stubVars(s, params, nil)
	if !stubbable(s, group[0]) || len(vars) == 0 {
		return
	}
	// The call's other lambdas whose parameter types have the variables
	// are checked with them unfixed too, as in Kotlin, before any is
	// fixed: parallelBuild({ consumeBase(it) }, { consumeDerived(it) })
	// gives PTV both upper bounds.
	for _, pl := range s.lambdas {
		pft := s.apply(pl.p)
		if !pft.Flex().IsFunc() {
			if m, f := c.samTarget(pft); m != nil {
				pft = f
			}
		}
		ps, _, ok := funcParts(pft)
		if pl.l == l || !ok || !slices.ContainsFunc(ps, func(p types.Type) bool {
			return s.apply(p).Has(func(u types.Type) bool { return u.K == types.TypeVar && slices.Contains(vars, u.Param()) })
		}) {
			continue
		}
		sl := stubLambda{pl.l, ps, pft, pl.scope}
		if !stubbable(s, sl) {
			return
		}
		group = append(group, sl)
		vars = c.stubVars(s, ps, vars)
	}
	// As in Kotlin, the lambdas' other variables are stubs too, whatever
	// else bounds them: emit2(get()) in a Foo<K1, K2>.() -> Unit, where
	// emit1(x) bounded K1, gives K2 K1.
	for _, sl := range group {
		vars = c.boundVars(s, sl.params, vars)
	}
	if c.stubs == nil {
		c.stubs = map[*types.TypeParamSym]*stubRun{}
	}
	r := &stubRun{s: s, vars: vars}
	for _, v := range vars {
		c.stubs[v] = r
	}
	clean := true
	for _, sl := range group {
		if clean = c.stubCheck(s, sl, r); !clean {
			break
		}
	}
	for _, v := range append(vars, r.extra...) {
		delete(c.stubs, v)
	}
	if !clean {
		// Code that doesn't check with the variables unfixed, as a
		// member of a value of one of them that nothing fixes yet,
		// which Kotlin rejects: they stay uninferred.
		return
	}
	if useful, _, ok := r.solve(); ok {
		for _, k := range useful {
			s.sub(k[0], k[1])
		}
	}
}

// stubLambda is a lambda that stubInference checks, for a function type ft
// with parameter types params, in scope.
type stubLambda struct {
	l      *syntax.Lambda
	params []types.Type
	ft     types.Type
	scope  *scope
}

// stubbable reports whether stubInference can check sl.
func stubbable(s *system, sl stubLambda) bool {
	// As in Kotlin, a receiver whose type is a variable has no members to
	// call: generate { consume(it) } for T.() -> Unit.
	k := sl.ft.Flex().Contexts()
	return sl.l.Ref == nil && !(sl.ft.Flex().E && len(sl.params) > k && s.isVar(sl.params[k]))
}

// stubVars returns vars with the variables of s in params that nothing
// bounds by a proper type added: only other variables, as T of
// selectBuildee<T>(build { yield(x) }, build {}) bounds each build's PTV.
func (c *Checker) stubVars(s *system, params []types.Type, vars []*types.TypeParamSym) []*types.TypeParamSym {
	for _, p := range params {
		s.apply(p).Has(func(u types.Type) bool {
			tv := s.unfixedVar(u.NonNull())
			if tv != nil && !slices.ContainsFunc(tv.lower, s.proper) && !slices.ContainsFunc(tv.upper, s.proper) && len(tv.lits) == 0 && c.stubs[tv.v] == nil && !slices.Contains(vars, tv.v) {
				vars = append(vars, tv.v)
			}
			return false
		})
	}
	return vars
}

// boundVars returns vars with the unfixed variables of s in params that
// aren't stubs already.
func (c *Checker) boundVars(s *system, params []types.Type, vars []*types.TypeParamSym) []*types.TypeParamSym {
	for _, p := range params {
		s.apply(p).Has(func(u types.Type) bool {
			if tv := s.unfixedVar(u.NonNull()); tv != nil && c.stubs[tv.v] == nil && !slices.Contains(vars, tv.v) {
				vars = append(vars, tv.v)
			}
			return false
		})
	}
	return vars
}

// stubCheck checks sl for stubInference, and reports whether it checks
// without errors. They and the types the check gives the tree are
// discarded, and what it changes outside sl's lambda undone. A result type
// with r's variables takes the lambda's: (A<T, T>) -> A<T, T> gives T from
// { it.convert() }.
func (c *Checker) stubCheck(s *system, sl stubLambda, r *stubRun) bool {
	ps := make([]types.Type, len(sl.params))
	for i, p := range sl.params {
		ps[i] = s.apply(p)
	}
	n, checked, saved := len(c.errs), c.checked, c.scope
	c.checked, c.scope = maps.Clone(checked), sl.scope
	ft := sl.ft
	w := lambdaWant{params: ps, known: true, recv: ft.Flex().E, ctx: ft.Flex().Contexts(), suspend: isSuspendFunc(ft), composable: isComposableFunc(ft), ret: noWant}
	_, ret, _ := funcParts(ft)
	if ret = s.apply(ret); !ret.HasVars() {
		w.ret, w.check = ret, true
	}
	t := &trial{classes: map[*types.ClassSym]bool{}, outer: c.trial}
	c.trial = t
	_, bt, _ := c.checkLambda(sl.l, w)
	c.endTrial(t)
	clean := len(c.errs) == n
	if clean && bt.K != types.Invalid && bt.K != types.Unit && ret.Has(func(u types.Type) bool { return u.K == types.TypeVar && c.stubs[u.Param()] == r }) {
		r.cons = append(r.cons, [2]types.Type{bt, ret})
	}
	c.errs, c.checked, c.scope = c.errs[:n], checked, saved
	return clean
}

// stubRun is a check of stubInference's: the system of the call whose
// lambda it checks, the variables it infers, and the constraints recorded
// on them.
type stubRun struct {
	s    *system
	vars []*types.TypeParamSym
	cons [][2]types.Type
	// joined are the systems of the calls in the lambda that joined the
	// run (see join), whose variables are stubs too.
	joined []*system
	extra  []*types.TypeParamSym
}

// joinRun returns the run that s, a call's system about to complete in a
// check of stubInference's, joins, if any: one whose stubs a variable of s
// is related to, by its bounds or by the run's constraints. As in Kotlin,
// such a call isn't inferred on its own, but with the builder's
// variables: f(DifferentType()) for f<E>(Buildee<out E>, E) on a
// Buildee<PTV> that yield(TargetType()) bounds gives E their common
// supertype, not PTV an upper bound DifferentType.
func (c *Checker) joinRun(s *system) *stubRun {
	if len(c.stubs) == 0 {
		return nil
	}
	var r *stubRun
	isStub := func(u types.Type) bool {
		if u.K == types.TypeVar && s.byVar[u.Param()] == nil && c.stubs[u.Param()] != nil {
			r = c.stubs[u.Param()]
			return true
		}
		return false
	}
	ours := func(u types.Type) bool { return u.K == types.TypeVar && s.isVar(u) }
	for _, tv := range s.vars {
		if tv.fixed {
			continue
		}
		for _, b := range append(tv.lower[:len(tv.lower):len(tv.lower)], tv.upper...) {
			if s.apply(b).Has(isStub) {
				return r
			}
		}
	}
	for _, run := range c.stubs {
		for _, k := range run.cons {
			if k[0].Has(ours) || k[1].Has(ours) {
				return run
			}
		}
	}
	return nil
}

// join adds s, a call's system, to r: its unfixed variables are stubs
// until r ends, which solve fixes with r's.
func (r *stubRun) join(s *system) {
	r.joined = append(r.joined, s)
	for _, tv := range s.vars {
		if !tv.fixed {
			s.c.stubs[tv.v] = r
			r.extra = append(r.extra, tv.v)
		}
	}
}

// add records t <: u in r, and reports whether it agrees with the
// constraints recorded so far and the variables' declared bounds: as in
// Kotlin, a candidate that would make them disagree isn't applicable.
// consume(getTypeVariable()) after setTypeVariable(TargetType()) calls
// consume(Any), not consume(DifferentType).
func (r *stubRun) add(t, u types.Type) bool {
	r.cons = append(r.cons, [2]types.Type{t, u})
	trial, _ := r.fixAll()
	if !trial.ok || slices.ContainsFunc(trial.errs[len(r.s.errs):], func(e inferError) bool { return !strings.HasPrefix(e.msg, "cannot infer") }) {
		r.cons = r.cons[:len(r.cons)-1]
		return false
	}
	return true
}

// solve returns the constraints of r that say something, and a copy of
// r's system with them added and r's variables fixed, if they agree.
func (r *stubRun) solve() ([][2]types.Type, *system, bool) {
	trial, useful := r.fixAll()
	if !trial.ok || len(trial.errs) > len(r.s.errs) {
		return nil, nil, false
	}
	return r.project(trial, useful), trial, true
}

// fixAll returns a copy of r's system, with the joined calls' systems, the
// constraints of r that say something added, and r's variables fixed, and
// those constraints.
func (r *stubRun) fixAll() (*system, [][2]types.Type) {
	// In the copy, the stubs are its own variables: the subtyping it does
	// records nothing.
	c := r.s.c
	saved := c.stubs
	c.stubs = nil
	defer func() { c.stubs = saved }()
	trial := r.s.clone()
	for _, j := range r.joined {
		j = j.clone()
		j.fixups, j.errs, j.lambdas = nil, nil, nil
		trial.merge(j)
	}
	// A joined variable's bound that is a variable of r's bounds that one
	// too, as sub relates two variables of a system: shareTypeInfo(42,
	// materialize())'s T <: FT is FT's too, which T's literal then gives.
	for _, v := range r.extra {
		tv, t := trial.byVar[v], types.VarType(v)
		for _, l := range tv.lower {
			if w := trial.unfixedVar(l); w != nil && w != tv && !l.N {
				trial.addUpper(w, t)
			}
		}
		for _, u := range tv.upper {
			if w := trial.unfixedVar(u); w != nil && w != tv {
				trial.addLower(w, t)
			}
		}
	}
	// The constraints must agree: println(get(0)), which every println
	// takes, gives get's E none. Those with the variables of other calls'
	// systems, finished, and trivial ones, say nothing. Nothing says
	// something only of a variable that nothing else does: yield(throw e)
	// alone gives Buildee<Nothing>.
	var useful, nothing [][2]types.Type
	for _, k := range r.cons {
		switch {
		case (k[1].K == types.Any && k[1].N) || !trial.owns(k[0]) || !trial.owns(k[1]):
		case k[0].K == types.Nothing:
			nothing = append(nothing, k)
		default:
			useful = append(useful, k)
		}
	}
	for _, k := range nothing {
		v := k[1].Param()
		has := func(u types.Type) bool { return u.K == types.TypeVar && u.Param() == v }
		if k[1].K == types.TypeVar && !slices.ContainsFunc(useful, func(u [2]types.Type) bool {
			return u[0].Has(has) || u[1].Has(has)
		}) {
			useful = append(useful, k)
		}
	}
	for _, k := range useful {
		trial.sub(k[0], k[1])
	}
	for _, v := range r.vars {
		trial.fixVarsIn(types.VarType(v))
	}
	for _, v := range r.extra {
		if tv := trial.byVar[v]; !tv.fixed && trial.fixable(tv) {
			trial.fixVarsIn(types.VarType(v))
		}
	}
	return trial, useful
}

// project returns the constraints useful, which trial solves, on r's own
// variables. Those with the joined calls' variables say of r's what those
// variables' values do: FT <: T, for shareTypeInfo's T fixed to Int,
// FT <: Int.
func (r *stubRun) project(trial *system, useful [][2]types.Type) [][2]types.Type {
	vals := types.Subst{}
	for _, v := range r.extra {
		if tv := trial.byVar[v]; tv.fixed && trial.vals[v].T.K != types.Invalid {
			vals[v] = trial.vals[v]
		}
	}
	// So do their bounds: value of Buildee<EFT>.value, on a Buildee<FT>,
	// gives FT EFT's value.
	ks := useful
	for _, j := range r.joined {
		for _, tv := range j.vars {
			val, ok := vals[tv.v]
			if !ok {
				continue
			}
			for _, l := range tv.lower {
				ks = append(ks, [2]types.Type{j.apply(l), val.T})
			}
			for _, u := range tv.upper {
				ks = append(ks, [2]types.Type{val.T, j.apply(u)})
			}
		}
	}
	var own [][2]types.Type
	for _, k := range ks {
		a, b := vals.Apply(k[0]), vals.Apply(k[1])
		if r.s.owns(a) && r.s.owns(b) && (a.HasVars() || b.HasVars()) && !(b.K == types.Any && b.N) {
			own = append(own, [2]types.Type{a, b})
		}
	}
	return own
}

// fixedStub returns t, a receiver's type, as what the constraints recorded
// so far fix it to, if it is a variable that stubInference is inferring:
// as in Kotlin, a receiver's members need its type fixed.
// setProducer { Target() } then setConsumer { it.consume() } calls
// Target's consume. The variable stays fixed to it for the rest of the
// check. Otherwise, it returns t.
func (c *Checker) fixedStub(t types.Type) types.Type {
	if c.stub(t.Flex()) == nil {
		return t
	}
	v := types.VarType(t.Flex().Param())
	r := c.stubs[v.Param()]
	_, trial, ok := r.solve()
	if !ok {
		return t
	}
	val := trial.apply(v)
	if val.HasVars() || val.K == types.Invalid {
		return t
	}
	r.cons = append(r.cons, [2]types.Type{val, v}, [2]types.Type{v, val})
	if t.N {
		val = val.Nullable()
	}
	return val
}

// trial is a check of a lambda that stubInference throws away: what it
// changes outside the lambda, its classes' names and the file's list of
// them, the captures it adds to the lambdas around and in it, and the
// declarations of its classes inferred, is undone, so that the check
// that counts declares and names them again, as if it were the first.
// The classes keep their symbols, which the constraints the trial
// records may mention: declareClass declares them again.
type trial struct {
	undo    []func()
	classes map[*types.ClassSym]bool // the classes the check declares
	outer   *trial                   // the one the lambda is in, if any
}

// endTrial ends trial t, undoing what its check changed, last first.
func (c *Checker) endTrial(t *trial) {
	c.trial = t.outer
	for i := len(t.undo) - 1; i >= 0; i-- {
		t.undo[i]()
	}
	if len(t.classes) > 0 && c.redeclared == nil {
		c.redeclared = map[*types.ClassSym]bool{}
	}
	for cls := range t.classes {
		c.redeclared[cls] = true
		// Their members are declared afresh.
		for _, m := range cls.Decl.(*syntax.ClassDecl).Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				m.Sym = nil
			case *syntax.PropDecl:
				m.Sym = nil
			}
		}
	}
}

// onUndo records f, which undoes a change, if a trial is running.
func (c *Checker) onUndo(f func()) {
	if c.trial != nil {
		c.trial.undo = append(c.trial.undo, f)
	}
}

// addLocalCall records lc, a call of a local function or a construction
// of a local class, and propagates the captures.
func (c *Checker) addLocalCall(lc localCall) {
	if c.localCalls == nil {
		c.localCalls = new([]localCall)
	}
	p := c.localCalls
	n := len(*p)
	c.onUndo(func() { *p = (*p)[:n] })
	*p = append(*p, lc)
	c.propagateCaptures()
}

// trialOf returns the trial that declared class cls, or the class cls is
// in, if any. Checking a declaration during inference belongs to it: the
// declaration is checked once, and what that changes is undone only with
// its class.
func (c *Checker) trialOf(cls *types.ClassSym) *trial {
	for t := c.trial; t != nil; t = t.outer {
		for k := cls; k != nil; k = k.Outer {
			if t.classes[k] {
				return t
			}
		}
	}
	return nil
}

// stub returns the run of stubInference's that infers t, if it is a
// variable.
func (c *Checker) stub(t types.Type) *stubRun {
	if len(c.stubs) == 0 || t.K != types.TypeVar {
		return nil
	}
	return c.stubs[t.Param()]
}

// onlyStubs reports whether the variables t mentions are all ones that
// stubInference is inferring.
func (c *Checker) onlyStubs(t types.Type) bool {
	return len(c.stubs) > 0 && !t.Has(func(u types.Type) bool { return u.K == types.TypeVar && c.stubs[u.Param()] == nil })
}

// declaredIn returns the names that a lambda declares: its parameters,
// and the locals, loop variables and catch parameters in its body.
func declaredIn(l *syntax.Lambda) map[string]bool {
	names := map[string]bool{}
	syntax.Inspect(l, func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			names["it"] = true
			for _, p := range n.Params {
				names[p.Name] = true
				for _, q := range p.Names {
					names[q] = true
				}
			}
		case *syntax.VarStmt:
			names[n.Name] = true
		case *syntax.Destructure:
			for _, q := range n.Names {
				names[q] = true
			}
		case *syntax.For:
			names[n.Var] = true
			for _, q := range n.Names {
				names[q] = true
			}
		case *syntax.Catch:
			names[n.Name] = true
		}
		return true
	})
	return names
}

// postponedLambda checks a lambda argument of a call being inferred, for
// a parameter of type p. The variables in its parameter types are fixed
// first; its result then constrains the variables in the result type.
func (c *Checker) postponedLambda(s *system, l *syntax.Lambda, p types.Type) {
	pt := s.apply(p)
	if wantsKProp(l, pt) {
		// Its type doesn't depend on the parameter's.
		lt := c.kpropRef(l)
		l.T = lt
		switch {
		case lt.K == types.Invalid:
		case pt.HasVars():
			s.sub(lt, pt)
		default:
			c.assignable(lt, pt, l.P)
		}
		return
	}
	ft := pt
	var sam *types.FunSym
	if !pt.Flex().IsFunc() {
		if m, f := c.samTarget(pt); m != nil {
			sam, ft = m, f
		}
	}
	params, ret, ok := funcParts(ft)
	if !ok {
		// A type variable or Any: the lambda has its own type.
		lt := c.lambda(l, noWant)
		l.T = lt
		if pt.HasVars() {
			s.sub(lt, pt)
		} else {
			c.assignable(lt, pt, l.P)
		}
		return
	}
	outerBounds(s, params)
	if k := ft.Flex().Contexts(); l.Ref != nil && len(params) > k && s.apply(params[k]).HasVars() {
		// An unbound reference's receiver is its first parameter's
		// bound: T of check(Int::toString, ...) for a (T) -> R is Int.
		if rt, ok := c.refRecvType(l.Ref); ok {
			trial := s.clone()
			if trial.sub(params[k], rt); trial.ok {
				s.sub(params[k], rt)
			}
		}
	}
	// As in Kotlin, the parameter types whose variables are known are
	// fixed first, and only the others are inferred from the body: T of
	// transformLatest's FlowCollector<R>.(T) from its receiver.
	for _, q := range params {
		if s.known(q) && s.apply(q).HasVars() {
			s.fixVarsIn(q)
		}
	}
	c.stubInference(s, l, params, ft)
	for _, q := range params {
		s.fixVarsIn(q)
	}
	for i := range params {
		params[i] = s.final(params[i])
	}
	w := lambdaWant{params: params, known: true, recv: ft.Flex().E, ctx: ft.Flex().Contexts(), suspend: isSuspendFunc(ft), composable: isComposableFunc(ft), ret: noWant}
	r := s.apply(ret)
	switch {
	case !r.HasVars():
		w.ret, w.check = r, true
	case r.K == types.TypeVar:
		w.ret = s.upperHint(r)
		if w.ret.K == types.Invalid && endsInLambda(l) {
			// The lambda it returns takes its parameter types from the
			// result's function-typed bound, once known.
			s.fixFuncInputs(r)
			w.ret = s.upperHint(r)
		}
	}
	var bt types.Type
	if pre, ok := c.checked[l]; ok {
		// Checked to choose the overload, with these parameter types.
		bt = pre
		if w.check && w.ret.K != types.Unit {
			c.assignable(bt, w.ret, resultPos(l))
			bt = w.ret
		}
	} else {
		bt = c.checkTail(s, l, w, r)
	}
	if r.HasVars() && bt.K != types.Invalid && bt.K != types.Unit {
		s.sub(bt, r)
	} else if r.HasVars() && bt.K == types.Unit && r.K == types.TypeVar {
		s.sub(bt, r) // a lambda ending in a statement gives R := Unit
	}
	s.fixups = append(s.fixups, func(s *system) {
		rr := s.final(ret)
		if r.HasVars() && rr.K != types.Unit && rr.K != types.Invalid {
			bt := s.final(bt) // the call it ends with may have been inferred with s
			c.assignable(bt, captured(bt, ret, rr), resultPos(l))
		}
		if sam != nil {
			l.SAM, l.Ret, l.T = sam, rr, s.final(pt)
			c.samRef(l)
			return
		}
		l.T = c.lambdaType(l, params, rr)
		c.funRef(l, l.T, len(params))
	})
}

// outerBounds gives each variable in a lambda's parameter types that has
// no proper bounds, but is another variable of s, with a declared bound,
// that bound, as Kotlin fixes that variable before the lambda: T : Any of
// Box<T : Any>(build { yield(x) }) makes build's Buildee<PTV> a
// Buildee<Any>, whatever x is.
func outerBounds(s *system, params []types.Type) {
	for _, p := range params {
		s.apply(p).Has(func(u types.Type) bool {
			tv := s.unfixedVar(u.NonNull())
			if tv == nil || slices.ContainsFunc(tv.lower, s.proper) || slices.ContainsFunc(tv.upper, s.proper) {
				return false
			}
			for _, l := range tv.lower {
				w := s.unfixedVar(l)
				if w == nil || w == tv || !slices.Contains(tv.upper, l) || slices.ContainsFunc(w.lower, s.proper) || slices.ContainsFunc(w.upper, s.proper) {
					continue
				}
				for _, b := range w.v.Bounds {
					if b = s.apply(b); !b.HasVars() && !(b.K == types.Any && b.N) {
						s.sub(b, l)
						return false
					}
				}
			}
			return false
		})
	}
}

// checkTail checks lambda l, whose result type r has variables of s.
// Without an expected result type, a generic call it ends with that can't
// be inferred on its own is inferred with s, as Kotlin infers a lambda's
// result with the call it is passed to: xs + ys.flatMap { y -> all(y) },
// whose all<T> gets its T from xs's elements, through flatMap's R.
func (c *Checker) checkTail(s *system, l *syntax.Lambda, w lambdaWant, r types.Type) types.Type {
	if !r.HasVars() || w.ret.K != types.Invalid || l.Ref == nil && !endsWithCall(l.Body) {
		_, bt, _ := c.checkLambda(l, w)
		return bt
	}
	tail := newSystem(c)
	savedBlock, savedSink := c.tailBlock, c.tailSink
	c.tailBlock, c.tailSink = l.Body, tail
	if l.Ref != nil {
		// A reference's body, the call it makes, is built by checkRef.
		c.tailBlock = refTail
	}
	_, bt, _ := c.checkLambda(l, w)
	c.tailBlock, c.tailSink = savedBlock, savedSink
	if len(tail.vars) == 0 && len(tail.fixups) == 0 {
		return bt
	}
	if len(tail.lambdas) == 0 && tail.selfInferable(bt) {
		tail.finish()
		return tail.final(bt)
	}
	s.merge(tail)
	return bt
}

// endsWithCall reports whether a block's last statement is a call that
// takes no lambda, whose own inference, a builder's among them, is left
// to it: build { set(x) } infers build's type argument from its lambda.
func endsWithCall(b *syntax.Block) bool {
	if n := len(b.Stmts); n > 0 {
		if es, ok := b.Stmts[n-1].(*syntax.ExprStmt); ok {
			call, ok := es.X.(*syntax.Call)
			return ok && !hasLambda(call.Args)
		}
	}
	return false
}

// implicitRecv is the receiver of a lambda with a receiver: this in its
// body, whose members can be used unqualified. depth is the depth of the
// lambda's scope, for captures.
type implicitRecv struct {
	sym   *types.VarSym
	label string
	depth int
	// outer is set on the receivers around an object expression, in its
	// members: the enclosing class's this and lambdas' receivers, which
	// come after the object's own members.
	outer bool
	// class is set on the enclosing class's this among those, which,
	// unlike a lambda's receiver, may use members private to this.
	class bool
	// at is set on a context parameter, in state.ctxs: the number of
	// receivers declared before it, which it is nearer than. outer is set
	// on those around an object expression or a local class too.
	at int
	// owner is the function or lambda whose receiver or context parameter
	// this is: a function's context parameters are as near as its
	// receiver, though at is its index.
	owner any
}

// destructureParams names a lambda's destructured parameters, { (a, b) ->
// ... }, and declares their components at the start of its body: the
// lambda is { p -> val (a, b) = p; ... }, as a for loop over pairs is. It
// does it once, though a lambda can be checked more than once.
func (c *Checker) destructureParams(x *syntax.Lambda) {
	var decls []syntax.Stmt
	for _, p := range x.Params {
		if p.Names == nil || p.Name != "" {
			continue
		}
		c.temps++
		p.Name = fmt.Sprintf("<destructured%d>", c.temps)
		id := &syntax.Ident{Name: p.Name}
		id.P = p.P
		decls = append(decls, &syntax.Destructure{P: p.P, Names: p.Names, Types: p.Types, Init: id})
	}
	if decls != nil {
		x.Body.Stmts = append(decls, x.Body.Stmts...)
	}
}

// anonWant gives an anonymous function's types to what is expected of it:
// its receiver's and parameters', which it declares, where the context
// doesn't give them, and its result type, declared or Unit for a block
// body, which the body must have.
func (c *Checker) anonWant(x *syntax.Lambda, w *lambdaWant) bool {
	if !w.known {
		var ps []types.Type
		for _, p := range x.AnonCtx {
			ps = append(ps, c.resolveType(p.Type))
		}
		if x.AnonRecv != nil {
			ps = append(ps, c.resolveType(x.AnonRecv))
		}
		for _, p := range x.Params {
			if p.Type == nil {
				c.errorf(p.P, "the parameters of an anonymous function need types")
				return false
			}
			ps = append(ps, c.resolveType(p.Type))
		}
		*w = lambdaWant{params: ps, known: true, recv: x.AnonRecv != nil, ctx: len(x.AnonCtx), ret: noWant}
	} else if x.AnonRecv != nil != w.recv && !anonRecvParam(x, *w) {
		c.errorf(x.P, "the anonymous function's receiver doesn't match the expected function type")
		return false
	} else if len(x.AnonCtx) != w.ctx {
		c.errorf(x.P, "the anonymous function's context parameters don't match the expected function type")
		return false
	}
	switch {
	case x.AnonRet != nil:
		w.ret, w.check = c.resolveType(x.AnonRet), true
	case x.AnonUnit:
		w.ret, w.check = types.UnitType, true
	}
	return true
}

// anonRecvParam reports whether anonymous function x takes the receiver of
// the function type w expects as its first parameter, as in Kotlin:
// fun(s) = s.length for a String.() -> Int.
func anonRecvParam(x *syntax.Lambda, w lambdaWant) bool {
	return x.Anon && w.recv && x.AnonRecv == nil && len(x.Params) == len(w.params)
}
