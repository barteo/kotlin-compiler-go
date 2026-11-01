package check

import (
	"fmt"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

func (c *Checker) push() { c.scope = &scope{parent: c.scope, depth: c.scope.depth} }
func (c *Checker) pop()  { c.scope = c.scope.parent }

// block checks a block. If valueUsed, the last statement is its value and
// is checked against want.
func (c *Checker) block(b *syntax.Block, want types.Type, valueUsed bool) types.Type {
	exit := c.exitWatch
	c.exitWatch = nil
	c.push()
	t := c.stmts(b, want, valueUsed)
	if exit != nil {
		exit.record(c, t)
	}
	c.blockScope = c.scope
	c.pop()
	return t
}

// stmts checks a block's statements in the current scope.
func (c *Checker) stmts(b *syntax.Block, want types.Type, valueUsed bool) types.Type {
	t := types.UnitType
	nothing := false
	for i, s := range b.Stmts {
		last := i == len(b.Stmts)-1
		c.site = varSite{b, i}
		if last && b == c.tailBlock {
			c.sink, c.tailBlock = c.tailSink, nil
		}
		st := c.stmt(s, valueUsed && last, want)
		if st.K == types.Nothing || stmtJumps(s) {
			nothing = true
		}
		if last {
			t = st
		}
		c.narrowAfter(s)
	}
	if nothing {
		t = types.NothingType
	}
	b.T = t
	return t
}

// stmtJumps reports whether a statement always jumps, though its type
// isn't Nothing: what it always evaluates is, as foo(x, return y),
// val a = Array(n, return "OK") and for (i in 1u..return "OK").
func stmtJumps(s syntax.Stmt) bool {
	switch s := s.(type) {
	case *syntax.ExprStmt:
		return jumps(s.X)
	case *syntax.VarStmt:
		return s.Delegate == nil && jumps(s.Init)
	case *syntax.Destructure:
		return jumps(s.Init)
	case *syntax.Assign:
		switch t := s.Target.(type) {
		case *syntax.Member:
			if jumps(t.X) {
				return true
			}
		case *syntax.Index:
			if jumps(t.X) || !hasSafeCall(t.X) && anyJumps(t.Args) {
				return true
			}
		}
		return jumps(s.Value)
	case *syntax.For:
		return jumps(s.Range)
	case *syntax.While:
		return !s.Do && jumps(s.Cond)
	}
	return false
}

// jumps reports whether evaluating e always jumps: e, or an expression
// that evaluating it always evaluates, is of type Nothing, as return y
// in foo(x, return y), or it is a lambda that the call it is passed to
// calls exactly once, whose body jumps out of it, as { return a.y } in
// C().apply { return a.y }. The expressions it always evaluates are
// those that alwaysCasts walks.
func jumps(e syntax.Expr) bool {
	if e == nil {
		return false
	}
	if t := e.Type(); t.K == types.Nothing && !t.N {
		return true
	}
	switch e := e.(type) {
	case *syntax.Call:
		if e.Invoke {
			if jumps(e.Fun) {
				return true
			}
		} else if m, ok := e.Fun.(*syntax.Member); ok {
			if jumps(m.X) {
				return true
			}
			if hasSafeCall(m) {
				return false
			}
		}
		for _, a := range e.Args {
			if l, ok := a.(*syntax.Lambda); ok {
				if l.Once && l.Body != nil && l.Body.T.K == types.Nothing && !returnsFrom(l) {
					return true
				}
				continue
			}
			if jumps(a) {
				return true
			}
		}
	case *syntax.Member:
		return jumps(e.X)
	case *syntax.Index:
		return jumps(e.X) || !hasSafeCall(e.X) && anyJumps(e.Args)
	case *syntax.Binary:
		return jumps(e.X) || e.Op != syntax.AND && e.Op != syntax.OR && e.Op != syntax.ELVIS && jumps(e.Y)
	case *syntax.Unary:
		return jumps(e.X)
	case *syntax.As:
		return jumps(e.X)
	case *syntax.Is:
		return jumps(e.X)
	case *syntax.NotNull:
		return jumps(e.X)
	case *syntax.If:
		return jumps(e.Cond)
	case *syntax.When:
		return jumps(e.Subject)
	case *syntax.StringLit:
		for _, p := range e.Parts {
			if jumps(p.X) {
				return true
			}
		}
	}
	return false
}

func anyJumps(es []syntax.Expr) bool {
	for _, e := range es {
		if jumps(e) {
			return true
		}
	}
	return false
}

// returnsFrom reports whether lambda l has a return from it, return@l,
// whose type is Nothing but which completes the call l is passed to.
func returnsFrom(l *syntax.Lambda) bool {
	found := false
	inspect(l.Body, func(n any) bool {
		if j, ok := n.(*syntax.Jump); ok && j.Kind == syntax.RETURN && j.Lambda == l {
			found = true
		}
		return !found
	})
	return found
}

// narrowAfter applies smart casts that hold after a statement such as
// `if (x == null) return` or `x as T`.
func (c *Checker) narrowAfter(s syntax.Stmt) {
	cast := func(as *syntax.As) {
		if k, ok := c.castNarrow(as); ok {
			c.scope.narrows = append(c.scope.narrows, k)
		}
	}
	switch s := s.(type) {
	case *syntax.VarStmt:
		if s.Init != nil {
			alwaysCasts(s.Init, cast)
		}
	case *syntax.Assign:
		alwaysCasts(s.Value, cast)
	}
	es, ok := s.(*syntax.ExprStmt)
	if !ok {
		return
	}
	alwaysCasts(es.X, cast)
	if _, ok := es.X.(*syntax.As); ok {
		return
	}
	if call, ok := es.X.(*syntax.Call); ok {
		c.scope.narrows = append(c.scope.narrows, c.contractNarrows(call, -1)...)
	}
	if call, ok := es.X.(*syntax.Call); ok && len(call.Args) > 0 {
		// The contracts of require(cond) and check(cond): cond holds
		// after them; of requireNotNull(x) and checkNotNull(x): x isn't
		// null.
		switch call.Builtin {
		case types.Require, types.Check:
			whenTrue, _ := c.narrowing(call.Args[0])
			c.scope.narrows = append(c.scope.narrows, whenTrue...)
		case types.RequireNotNull, types.CheckNotNull:
			if k, ok := c.narrowKeyOf(call.Args[0]); ok && k.t.K != types.Invalid {
				k.t = k.t.NonNull()
				c.scope.narrows = append(c.scope.narrows, k)
			}
		}
		return
	}
	n, ok := es.X.(*syntax.If)
	if !ok {
		return
	}
	thenJumps := n.Then.T.K == types.Nothing
	elseJumps := n.Else != nil && n.Else.T.K == types.Nothing
	whenTrue, whenFalse := c.narrowing(n.Cond)
	switch {
	case thenJumps && !elseJumps:
		c.scope.narrows = append(c.scope.narrows, whenFalse...)
	case elseJumps && !thenJumps:
		c.scope.narrows = append(c.scope.narrows, whenTrue...)
	}
}

// castNarrow returns the smart cast that x as T makes, after it.
func (c *Checker) castNarrow(as *syntax.As) (narrow, bool) {
	k, ok := c.narrowKeyOf(as.X)
	if !ok || as.Target.K == types.Invalid {
		return k, false
	}
	cc, tc := c.classOf(k.t.NonNull()), c.classOf(as.Target.NonNull())
	if cc != nil && tc != nil && cc != tc && cc.IsSubclassOf(tc) {
		// A cast up to a supertype, as (ages as Map<Any?, Any?>),
		// which Kotlin intersects with x's type: x keeps its type,
		// non-null if the cast's is.
		if types.MayBeNull(as.Target) {
			return k, false
		}
		k.t = k.t.NonNull()
	} else {
		t := as.Target
		if t.K == types.Class && t.A != nil && allStar(t) {
			// As after x is C<*> (see checkTarget), x has the arguments
			// that follow from its type: a Buildee<PTV> cast to a
			// DerivedBuildee<*> is a DerivedBuildee<PTV>.
			if d := c.bareType(t.C, as.Ref, k.t); c.argsInBounds(d) {
				t = d
			}
		}
		k.t = c.isNarrowed(k.t, t)
	}
	return k, true
}

// argScope returns the scope that argument i of a call, a lambda, is
// checked in: with the smart casts of the arguments before it, as
// onClosed = { onClosedSelectOnSend(element, select) } after element =
// element as E.
func (c *Checker) argScope(args []syntax.Expr, i int) *scope {
	var ns []narrow
	for _, a := range args[:i] {
		alwaysCasts(a, func(as *syntax.As) {
			if k, ok := c.castNarrow(as); ok {
				ns = append(ns, k)
			}
		})
	}
	if ns == nil {
		return c.scope
	}
	return &scope{parent: c.scope, narrows: ns, depth: c.scope.depth}
}

// alwaysCasts calls f on the casts x as T, not x as? T, that evaluating e
// always evaluates, after which x has type T, as in
// assertThat((x as T).title): those in receivers, arguments and operands,
// but not in lambdas, in branches, after a safe call or in the right
// operand of &&, || and ?:.
func alwaysCasts(e syntax.Expr, f func(*syntax.As)) {
	switch e := e.(type) {
	case *syntax.As:
		alwaysCasts(e.X, f)
		if !e.Safe {
			f(e)
		}
	case *syntax.Call:
		if e.Invoke {
			alwaysCasts(e.Fun, f)
		} else if m, ok := e.Fun.(*syntax.Member); ok {
			alwaysCasts(m.X, f)
			if hasSafeCall(m) {
				return
			}
		}
		for _, a := range e.Args {
			alwaysCasts(a, f)
		}
	case *syntax.Member:
		alwaysCasts(e.X, f)
	case *syntax.Index:
		alwaysCasts(e.X, f)
		if !hasSafeCall(e.X) {
			for _, a := range e.Args {
				alwaysCasts(a, f)
			}
		}
	case *syntax.Binary:
		alwaysCasts(e.X, f)
		if e.Op != syntax.AND && e.Op != syntax.OR && e.Op != syntax.ELVIS {
			alwaysCasts(e.Y, f)
		}
	case *syntax.Unary:
		alwaysCasts(e.X, f)
	case *syntax.NotNull:
		alwaysCasts(e.X, f)
	case *syntax.StringLit:
		for _, p := range e.Parts {
			if p.X != nil {
				alwaysCasts(p.X, f)
			}
		}
	}
}

// hasSafeCall reports whether a chain of member accesses, calls and
// indexing has a safe call, after which the rest of it may not run.
func hasSafeCall(e syntax.Expr) bool {
	switch e := e.(type) {
	case *syntax.Member:
		return e.Safe || hasSafeCall(e.X)
	case *syntax.Call:
		return !e.Invoke && hasSafeCall(e.Fun)
	case *syntax.Index:
		return hasSafeCall(e.X)
	}
	return false
}

func (c *Checker) stmt(s syntax.Stmt, valueUsed bool, want types.Type) types.Type {
	switch s := s.(type) {
	case *syntax.ExprStmt:
		if valueUsed {
			return c.value(s.X, want)
		}
		return c.exprStmt(s.X)
	case *syntax.VarStmt:
		c.varStmt(s)
	case *syntax.Destructure:
		c.destructure(s)
	case *syntax.Assign:
		c.assign(s)
	case *syntax.LocalFun:
		c.localFun(s)
	case *syntax.LocalClass:
		c.localClass(s)
	case *syntax.While:
		loop := &loopCtx{node: s, label: s.Label, lambdas: len(c.lambdas)}
		// The vars the loop assigns lose their narrowing in it and after
		// it.
		vars := c.watchedVars(s.Cond, s.Body)
		if s.Do {
			// The condition sees the body's declarations.
			c.push()
			c.reset(vars)
			c.loops = append(c.loops, loop)
			s.Body.T = c.stmts(s.Body, noWant, false)
			c.loops = c.loops[:len(c.loops)-1]
			c.value(s.Cond, types.BoolType)
			c.wantBool(s.Cond)
			c.pop()
		} else {
			c.push()
			c.reset(vars)
			c.value(s.Cond, types.BoolType)
			c.wantBool(s.Cond)
			whenTrue, _ := c.narrowing(s.Cond)
			c.loops = append(c.loops, loop)
			c.push()
			c.scope.narrows = append(c.scope.narrows, whenTrue...)
			c.block(s.Body, noWant, false)
			c.pop()
			c.loops = c.loops[:len(c.loops)-1]
			c.pop()
		}
		c.reset(vars)
		if lit, ok := s.Cond.(*syntax.BoolLit); ok && lit.Val && !loop.hasBreak {
			s.Infinite = true
			return types.NothingType
		}
		if s.Do && s.Body.T.K == types.Nothing && !loop.hasBreak && !loop.hasContinue {
			return types.NothingType // the body runs once, and jumps
		}
	case *syntax.For:
		if len(s.Names) > 0 && s.Var == "" {
			// for ((k, v) in m) is for (e in m) { val (k, v) = e }.
			c.temps++
			s.Var = fmt.Sprintf("<element%d>", c.temps)
			id := &syntax.Ident{Name: s.Var}
			id.P = s.P
			d := &syntax.Destructure{P: s.P, Names: s.Names, Types: s.Types, Init: id}
			s.Body.Stmts = append([]syntax.Stmt{d}, s.Body.Stmts...)
		}
		elem := types.IntType
		if t := c.rangeExpr(s.Range); t.K != types.Range {
			elem = c.forIterator(s, t)
		} else {
			elem = t.RangeElem()
		}
		if s.Type != nil {
			// for (x: T in xs) is for (e in xs) { val x: T = e } unless T
			// is the element's type.
			if t := c.resolveType(s.Type); t != elem && t.K != types.Invalid {
				c.temps++
				id := &syntax.Ident{Name: fmt.Sprintf("<element%d>", c.temps)}
				id.P = s.P
				v := &syntax.VarStmt{P: s.P, Name: s.Var, Type: s.Type, Init: id}
				s.Body.Stmts = append([]syntax.Stmt{v}, s.Body.Stmts...)
				s.Var = id.Name
			}
			s.Type = nil
		}
		vars := c.watchedVars(s.Body)
		c.push()
		c.reset(vars)
		s.Sym = &types.VarSym{Name: s.Var, Type: elem, Kind: types.Local}
		c.scope.declare(s.Sym)
		c.loops = append(c.loops, &loopCtx{node: s, label: s.Label, lambdas: len(c.lambdas)})
		c.block(s.Body, noWant, false)
		c.loops = c.loops[:len(c.loops)-1]
		c.pop()
		c.reset(vars)
	case *syntax.Block:
		return c.block(s, want, valueUsed)
	}
	return types.UnitType
}

// exprStmt checks an expression whose value is discarded.
func (c *Checker) exprStmt(x syntax.Expr) types.Type {
	var t types.Type
	switch x := x.(type) {
	case *syntax.If:
		t = c.ifExpr(x, noWant, false)
	case *syntax.When:
		t = c.whenExpr(x, noWant, false)
	case *syntax.Try:
		t = c.tryExpr(x, noWant, false)
	case *syntax.Unary:
		if x.Op == syntax.INC || x.Op == syntax.DEC {
			t = c.incDec(x, false)
			break
		}
		t = c.expr(x, noWant)
	default:
		t = c.expr(x, noWant)
	}
	if n := len(c.lrets); n > 0 && c.lrets[n-1].tail == x {
		c.lrets[n-1].tailType = t
	}
	if t.K == types.Range {
		c.errorf(x.Pos(), "ranges are only supported in 'for' loops and 'in' checks")
	}
	if t.K == types.Nothing {
		return t
	}
	return types.UnitType
}

func (c *Checker) varStmt(s *syntax.VarStmt) {
	saved := c.declName
	c.declName = s.Name
	defer func() { c.declName = saved }()
	if s.Delegate != nil {
		c.localDelegate(s)
		return
	}
	var t types.Type
	narrowed := types.InvalidType
	switch {
	case s.Init == nil:
		t = c.resolveType(s.Type)
	case s.Type != nil:
		t = c.resolveType(s.Type)
		c.assignable(c.value(s.Init, t), t, s.Init.Pos())
	default:
		t = c.value(s.Init, noWant)
		if d, ok := c.smartCastFrom(s.Init, t); ok && s.Mutable {
			// As kotlinc's, var x = y, for a y smart cast, is of y's
			// declared type, and smart cast as y is.
			t, narrowed = d, t
		}
	}
	s.Sym = &types.VarSym{Name: s.Name, Type: t, Kind: types.Local, Mutable: s.Mutable, Decl: s, Deferred: s.Init == nil && !s.Lateinit}
	if s.Lateinit {
		c.lateinitChecks(s.LateinitP, s.Mutable, s.Init != nil, false, t)
		s.Sym.Lateinit = true
	}
	if s.Mutable && c.site.b != nil && c.site.b.Stmts[c.site.i] == syntax.Stmt(s) {
		c.varSites[s.Sym] = c.site
	}
	if s.Sym.Deferred {
		c.deferred++
	}
	if !s.Mutable && s.Init != nil && types.MayBeNull(t) {
		c.implyNonNull(s.Sym, s.Init)
	}
	if !s.Mutable && s.Init != nil && t.Flex() == types.BoolType {
		c.condVal(s.Sym, s.Init)
	}
	if !c.scope.declare(s.Sym) {
		c.errorf(s.P, "conflicting declarations: %s", s.Name)
	}
	if narrowed.K != types.Invalid {
		c.assignedVar(s.Sym, narrowed)
	}
}

// smartCastFrom returns the declared type of x, a local variable or a
// parameter smart cast to type t.
func (c *Checker) smartCastFrom(x syntax.Expr, t types.Type) (types.Type, bool) {
	id, ok := x.(*syntax.Ident)
	if !ok || id.Sym == nil || id.Member != nil || id.Sym.Delegate != nil || id.Sym.Kind != types.Local && id.Sym.Kind != types.Param {
		return types.InvalidType, false
	}
	d := id.Sym.Type
	if d == t || t.K == types.Invalid || !c.subtype(t, d) || c.subtype(d, t) {
		return types.InvalidType, false
	}
	return d, true
}

// implyNonNull records the smart casts that v, a val of value x, not
// being null implies: x's receivers of safe calls aren't null either,
// when they are vals.
func (c *Checker) implyNonNull(v *types.VarSym, x syntax.Expr) {
	var ns []narrow
	for _, k := range c.safeReceivers(x) {
		if !k.sym.Mutable && (k.recv == nil || !k.recv.Mutable) {
			ns = append(ns, k)
		}
	}
	if len(ns) == 0 {
		return
	}
	if c.implied == nil {
		c.implied = map[*types.VarSym][]narrow{}
	}
	c.implied[v] = ns
}

// condVal records the smart casts that v, a Boolean val of value x,
// being true or false implies: x's, of vals.
func (c *Checker) condVal(v *types.VarSym, x syntax.Expr) {
	whenTrue, whenFalse := c.narrowing(x)
	stable := func(ns []narrow) []narrow {
		var out []narrow
		for _, k := range ns {
			if !k.sym.Mutable && (k.recv == nil || !k.recv.Mutable) {
				out = append(out, k)
			}
		}
		return out
	}
	whenTrue, whenFalse = stable(whenTrue), stable(whenFalse)
	if len(whenTrue) == 0 && len(whenFalse) == 0 {
		return
	}
	if c.conds == nil {
		c.conds = map[*types.VarSym][2][]narrow{}
	}
	c.conds[v] = [2][]narrow{whenTrue, whenFalse}
}

// safeAssign checks a?.b = v, or a?.b op= v, which assigns only if a
// isn't null: val t = a; if (t != null) t.b = v.
func (c *Checker) safeAssign(s *syntax.Assign, m *syntax.Member) {
	c.push()
	defer c.pop()
	recv := c.temp(s.P, "receiver", m.X, c.value(m.X, noWant))
	target := &syntax.Member{X: tempRef(recv, m.P), Name: m.Name}
	target.P = m.P
	null := &syntax.NullLit{}
	null.P = m.P
	cond := &syntax.Binary{Op: syntax.NE, X: tempRef(recv, m.P), Y: null}
	cond.P = m.P
	set := &syntax.Assign{P: s.P, Op: s.Op, Target: target, Value: s.Value}
	ifs := &syntax.If{Cond: cond, Then: &syntax.Block{P: s.P, Stmts: []syntax.Stmt{set}}}
	ifs.P = s.P
	es := &syntax.ExprStmt{X: ifs}
	c.stmt(es, false, noWant)
	s.Stmts = []syntax.Stmt{recv, es}
}

// destructure checks val (a, b) = e, as val t = e; val a =
// t.component1(); val b = t.component2().
func (c *Checker) destructure(s *syntax.Destructure) {
	if s.Stmts == nil {
		c.temps++
		tmp := fmt.Sprintf("<destructured%d>", c.temps)
		s.Stmts = []*syntax.VarStmt{{P: s.P, Name: tmp, Init: s.Init}}
		for i, name := range s.Names {
			if name == "_" {
				continue
			}
			id := &syntax.Ident{Name: tmp}
			id.P = s.P
			m := &syntax.Member{X: id, Name: fmt.Sprintf("component%d", i+1)}
			m.P = s.P
			call := &syntax.Call{Fun: m}
			call.P = s.P
			vs := &syntax.VarStmt{P: s.P, Name: name, Mutable: s.Mutable, Init: call}
			if i < len(s.Types) {
				vs.Type = s.Types[i]
			}
			s.Stmts = append(s.Stmts, vs)
		}
	}
	for i, vs := range s.Stmts {
		c.varStmt(vs)
		if call, ok := vs.Init.(*syntax.Call); ok && i > 0 && call.Sym != nil && !call.Sym.Operator {
			c.errorf(s.P, "'operator' modifier is required on '%s'", call.Sym.Name)
		}
	}
}

// assignsDeferred reports whether a val declared without a value can be
// assigned here, the first time: a local anywhere, a property of the
// class in its constructor's code. Definite assignment checks that it is
// assigned only once.
func (c *Checker) assignsDeferred(v *types.VarSym) bool {
	switch {
	case !v.Deferred:
		return false
	case v.Kind == types.Local:
		return true
	}
	return v.Kind == types.Prop && c.state.init && v.Owner == c.state.cls
}

// lvalue resolves an assignment target and returns its declared type.
func (c *Checker) lvalue(x syntax.Expr) (types.Type, bool) {
	switch x := x.(type) {
	case *syntax.Ident:
		sym, _, _, ok := c.lookupVar(x.Name)
		if !ok {
			c.errorf(x.P, "unresolved reference: %s", x.Name)
			return types.InvalidType, false
		}
		c.checkTopVar(sym, x.P)
		x.Sym = sym
		x.T = sym.Type
		if !sym.Mutable && !c.assignsDeferred(sym) {
			c.errorf(x.P, "val cannot be reassigned")
			return sym.Type, false
		}
		if c.propVisible(sym) && !c.setterVisible(sym) {
			c.invisibleSetter(x.P, sym)
			return sym.Type, false
		}
		return sym.Type, true
	case *syntax.Index:
		// Compound assignment of an element: a[i] += x, a[i]++.
		t := c.index(x, "get", x.Args, noWant)
		if f := x.Call.Sym; f != nil && f.Builtin != types.ArrayGet {
			c.unsupported(x.P, "compound indexed assignment", "compound assignment of an indexed element is not supported")
			return types.InvalidType, false
		}
		return t, t.K != types.Invalid
	case *syntax.Member:
		if x.Safe {
			c.unsupported(x.P, "safe-call assignment", "safe-call assignment is not supported")
			return types.InvalidType, false
		}
		t := c.member(x)
		x.T = t
		c.dslTarget(x)
		if x.Getter != nil && x.Setter == nil {
			x.Setter = x.Getter.ExtSetter // a var extension property's
		}
		switch {
		case x.Field != nil && x.Field.Constant == nil && x.Field.Mutable:
			return t, true
		case x.Field != nil:
			c.errorf(x.P, "val cannot be reassigned")
			return t, false
		case x.Getter != nil && x.Setter == nil:
			c.errorf(x.P, "val cannot be reassigned")
			return t, false
		case x.Getter != nil && !x.Setter.Ext && x.Setter.Owner != nil && x.X.Type().K == types.Class:
			// A Java setter's parameter: a List, where the getter
			// gives a MutableList.
			return c.memberSubst(x.X.Type().NonNull(), x.Setter.Owner).Apply(x.Setter.Params[0].Type), true
		case x.Getter != nil:
			return t.Flex(), true
		}
		if x.Prop == nil {
			if t.K != types.Invalid {
				c.errorf(x.P, "variable expected")
			}
			return types.InvalidType, false
		}
		if _, this := x.X.(*syntax.This); !x.Prop.Mutable && !(this && c.assignsDeferred(x.Prop)) {
			c.errorf(x.P, "val cannot be reassigned")
			return t, false
		}
		if c.propVisible(x.Prop) && !c.setterVisible(x.Prop) {
			c.invisibleSetter(x.P, x.Prop)
			return t, false
		}
		// Through a projected receiver, the type that can be written
		// differs from the one read: Nothing for Box<out T>.
		if rt := x.X.Type(); rt.K == types.Class && rt.A != nil {
			t = c.memberSubst(rt.NonNull(), x.Prop.Owner).ApplyIn(x.Prop.Type)
		}
		return t, true
	}
	c.errorf(x.Pos(), "variable expected")
	return types.InvalidType, false
}

// forIterator checks a loop over a collection, or anything else with an
// iterator() method, and returns the element type.
func (c *Checker) forIterator(s *syntax.For, t types.Type) types.Type {
	if t.K == types.Invalid {
		return t
	}
	if types.MayBeNull(t) && !t.P {
		c.errorf(s.Range.Pos(), "not nullable value required to call an 'iterator()' method on for-loop range")
		return types.InvalidType
	}
	if t.NonNull().IsArray() {
		return types.ArrayElem(t.NonNull()) // an indexed loop
	}
	if t.NonNull() == types.StringType {
		return types.CharType // an indexed loop over its chars
	}
	iter, it := c.operatorMember(t.NonNull(), "iterator")
	if iter == nil && c.isMap(t) {
		// A map is iterated through its entries: Map.iterator() is an
		// inline-only extension.
		m := &syntax.Member{X: s.Range, Name: "entries"}
		m.P = s.Range.Pos()
		if et := c.expr(m, noWant); et.K != types.Invalid {
			s.Range = m
			iter, it = c.operatorMember(et.NonNull(), "iterator")
		}
	}
	if iter == nil && c.isIterator(t) {
		// An iterator is its own: Iterator<T>.iterator() is an
		// inline-only extension that returns it. Iter stays nil.
		it = t.NonNull()
	} else if iter == nil {
		c.errorf(s.Range.Pos(), "for-loop range must have an 'iterator()' method")
		return types.InvalidType
	}
	it = it.Flex()
	hasNext, ht := c.operatorMember(it, "hasNext")
	next, elem := c.operatorMember(it, "next")
	if hasNext == nil || next == nil || ht.Flex() != types.BoolType {
		c.errorf(s.Range.Pos(), "the iterator of the for-loop range has no 'hasNext()' or 'next()' method")
		return types.InvalidType
	}
	s.Iter, s.HasNext, s.Next, s.IterT = iter, hasNext, next, it
	for _, f := range []*types.FunSym{iter, hasNext, next} {
		if f != nil {
			c.suspendCall(s.Range.Pos(), f)
		}
	}
	return elem
}

// isIterator reports whether t is a Kotlin Iterator, with the standard
// library: a Java Iterator too, as a Stream's iterator().
func (c *Checker) isIterator(t types.Type) bool {
	if !c.u.ext.kotlin {
		return false
	}
	it := c.u.ext.kclass("kotlin/collections/Iterator")
	if it == nil {
		return false
	}
	_, ok := c.asSuper(t.NonNull(), it)
	return ok
}

// isMap reports whether t is a Kotlin Map, with the standard library.
func (c *Checker) isMap(t types.Type) bool {
	if !c.u.ext.kotlin || t.NonNull().K != types.Class {
		return false
	}
	m := c.u.ext.kclass("kotlin/collections/Map")
	if m == nil {
		return false
	}
	_, ok := c.asSuper(t.NonNull(), m)
	return ok
}

// operatorMember finds an operator method without parameters, such as
// iterator(), of a receiver of type rt, or a top-level extension, and
// returns its result type.
func (c *Checker) operatorMember(rt types.Type, name string) (*types.FunSym, types.Type) {
	if cls := c.classOf(rt); cls != nil {
		cls.Ensure()
		for _, f := range c.javaMethods(cls, name) {
			if len(f.Params) == 0 && len(f.TypeParams) == 0 && !f.Unsupported && f.Operator {
				return f, c.memberSubst(rt, f.Owner).Apply(f.Ret)
			}
		}
	}
	// A top-level extension, not generic: operator fun R.iterator().
	for _, cl := range c.extCallees(name, rt) {
		f := cl.f
		if f.Owner == nil && len(f.Params) == 0 && len(f.TypeParams) == 0 && !f.Unsupported && f.Operator && f.Inline == nil {
			ensureFun(f)
			return f, f.Ret
		}
	}
	return nil, types.InvalidType
}

// thisJavaTarget rewrites an assignment's target x, a name, to this.x if
// it names a Java property of the class the code is in, inherited from a
// Java class, isDaemon = true in a Thread's subclass, or an extension
// property of the class.
func (c *Checker) thisJavaTarget(x syntax.Expr) syntax.Expr {
	id, ok := x.(*syntax.Ident)
	if !ok || c.cls == nil {
		return x
	}
	if _, _, _, found := c.lookupVar(id.Name); found {
		return x
	}
	this := &syntax.This{}
	this.P = id.P
	m := &syntax.Member{X: this, Name: id.Name}
	m.P = id.P
	probe := *m
	if _, found := c.javaProperty(&probe, c.cls.This()); found && probe.Setter != nil {
		return m
	}
	t := c.cls.This()
	if n, ok := c.narrowedThis(); ok {
		t = n
	}
	if _, found := c.extProp(&syntax.Member{Name: id.Name}, t); found {
		return m
	}
	return x
}

// objectTarget rewrites an assignment's target x, a name that is no
// variable or property of this, to Obj.x if it names a property of an
// object in scope, such as a companion's.
func (c *Checker) objectTarget(x syntax.Expr) syntax.Expr {
	id, ok := x.(*syntax.Ident)
	if !ok || !c.objectHasProp(id.Name) {
		return x
	}
	if _, _, _, found := c.lookupVar(id.Name); found {
		return x
	}
	if _, ok := c.objectMember(id); ok {
		return id.Member
	}
	return x
}

func (c *Checker) assign(s *syntax.Assign) {
	if m, ok := s.Target.(*syntax.Member); ok && m.Safe {
		c.safeAssign(s, m)
		return
	}
	if ix, ok := s.Target.(*syntax.Index); ok && s.Op == syntax.ASSIGN {
		args := append(ix.Args[:len(ix.Args):len(ix.Args)], s.Value)
		c.index(ix, "set", args, noWant)
		return
	}
	if ix, ok := s.Target.(*syntax.Index); ok {
		// An element of a list or a map: x.set(i, x.get(i) op v).
		op := arithOf(s.Op)
		value := func(read syntax.Expr) syntax.Expr {
			b := &syntax.Binary{Op: op, X: read, Y: s.Value}
			b.P = s.P
			return b
		}
		if stmts, _, ok := c.indexCompound(s.P, ix, opNames[s.Op], s.Value, value); ok {
			s.Stmts = stmts
			return
		}
	}
	s.Target = c.objectTarget(c.thisJavaTarget(c.importTarget(c.recvTarget(s.Target))))
	n := len(c.errs)
	var tt types.Type
	var mutable bool
	switch s.Target.(type) {
	case *syntax.Ident, *syntax.Index, *syntax.Member:
		if c.namesObject(s.Target) && s.Op != syntax.ASSIGN {
			tt = c.expr(s.Target, noWant) // Obj += v: Obj.plusAssign(v)
			break
		}
		tt, mutable = c.lvalue(s.Target)
	default:
		if s.Op == syntax.ASSIGN {
			tt, mutable = c.lvalue(s.Target) // reports it
			break
		}
		// x += v on a value that isn't a variable calls x.plusAssign(v):
		// ktor's this += bytes in an extension of its Digest.
		tt = c.expr(s.Target, noWant)
	}
	if mutable && c.contextWrite(s) {
		return
	}
	var v *types.VarSym // a local var, whose smart casts the assignment changes
	if id, ok := s.Target.(*syntax.Ident); ok && id.Sym != nil && isVar(id.Sym) {
		v = id.Sym
	}
	if s.Op == syntax.ASSIGN {
		vt := c.value(s.Value, tt)
		c.assignable(vt, tt, s.Value.Pos())
		if v != nil {
			c.assignedVar(v, vt)
			c.assignedNested(v, vt)
		}
		return
	}
	if v != nil {
		defer c.reset([]*types.VarSym{v})
		c.assignedNested(v, v.Type)
	}
	if !builtinOperand(tt) || classBound(tt) {
		// x += v may call x.plusAssign(v), also on a val or a var whose
		// setter the code can't see.
		if !mutable && len(c.errs) == n+1 && (c.errs[n].Msg == "val cannot be reassigned" || strings.HasPrefix(c.errs[n].Msg, invisibleSetterMsg)) {
			c.errs = c.errs[:n]
		}
		c.compoundOp(s, tt, mutable)
		return
	}
	s.OpType = tt
	vt := c.value(s.Value, noWant).Flex()
	if tt.K == types.Invalid || vt.K == types.Invalid {
		return
	}
	if s.Op == syntax.PLUS_EQ && tt.Flex() == types.StringType {
		s.OpType = types.StringType
		return
	}
	// x op= v is x = x op v, whose result must have x's type.
	op, rt, ok := arith(arithOf(s.Op), tt.Flex(), vt)
	switch {
	case !ok:
		c.errorf(s.P, "operator '%s' cannot be applied to %s and %s", s.Op, tt, vt)
	case rt != tt.Flex():
		c.errorf(s.Value.Pos(), "type mismatch: inferred type is %s but %s was expected", rt, tt)
	default:
		s.OpType = op
	}
}

// namesObject reports whether x is a name that isn't a variable but an
// object's, or a class's with a companion object.
func (c *Checker) namesObject(x syntax.Expr) bool {
	id, ok := x.(*syntax.Ident)
	if !ok {
		return false
	}
	if _, _, _, found := c.lookupVar(id.Name); found {
		return false
	}
	cls := c.lookupClass(id.Name)
	return cls != nil && objectOf(cls) != nil
}

// arithOf returns the arithmetic operator of a compound assignment.
func arithOf(k syntax.Kind) syntax.Kind {
	switch k {
	case syntax.PLUS_EQ:
		return syntax.PLUS
	case syntax.MINUS_EQ:
		return syntax.MINUS
	case syntax.STAR_EQ:
		return syntax.STAR
	case syntax.SLASH_EQ:
		return syntax.SLASH
	}
	return syntax.PERCENT
}

// incDec checks x++, x--, ++x or --x. It returns the type of x if it is
// a number or a Char, whose value x++ can be, and Unit otherwise, unless
// it is a value: then x = x.inc() and x.set(i, x.get(i).inc()) end with
// the value, an ExprStmt. As kotlinc does, x++ keeps the old value in a
// temporary, <unary>, read before the store, and ++x reads x again
// after it.
func (c *Checker) incDec(u *syntax.Unary, value bool) types.Type {
	u.T = types.UnitType
	var old *syntax.VarStmt
	keep := func(inc func(syntax.Expr) syntax.Expr) func(syntax.Expr) syntax.Expr {
		if !value || u.Prefix {
			return inc
		}
		return func(read syntax.Expr) syntax.Expr {
			old = c.temp(u.P, "unary", read, read.Type())
			r := tempRef(old, u.P)
			return inc(checked(r, c.value(r, noWant)))
		}
	}
	result := func(stmts []syntax.Stmt, reread func() syntax.Expr) types.Type {
		u.Stmts = stmts
		if !value {
			return types.UnitType
		}
		var x syntax.Expr
		switch n := len(stmts); {
		case old != nil:
			u.Stmts = append(stmts[:n-1:n-1], old, stmts[n-1])
			x = tempRef(old, u.P)
		case u.Prefix:
			x = reread()
		default:
			return types.InvalidType // the element is invalid
		}
		t := c.value(x, noWant)
		u.Stmts = append(u.Stmts, &syntax.ExprStmt{X: checked(x, t)})
		return t
	}
	if ix, ok := u.X.(*syntax.Index); ok {
		// An element of a list or a map: x.set(i, x.get(i).inc()).
		inc := func(read syntax.Expr) syntax.Expr {
			if builtinIncDec(read.Type()) {
				one := &syntax.IntLit{Val: 1}
				one.P = u.P
				b := &syntax.Binary{Op: arithOf(map[syntax.Kind]syntax.Kind{syntax.INC: syntax.PLUS_EQ, syntax.DEC: syntax.MINUS_EQ}[u.Op]), X: read, Y: one}
				b.P = u.P
				conv := map[types.Type]string{types.ByteType: "toByte", types.ShortType: "toShort"}[read.Type().Flex().NonNull()]
				if conv == "" {
					return b
				}
				// Byte.inc() is a Byte: (x + 1).toByte().
				m := &syntax.Member{X: b, Name: conv}
				m.P = u.P
				call := &syntax.Call{Fun: m}
				call.P = u.P
				return call
			}
			m := &syntax.Member{X: read, Name: opNames[u.Op]}
			m.P = u.P
			call := &syntax.Call{Fun: m, Operator: true}
			call.P = u.P
			return call
		}
		if stmts, reread, ok := c.indexCompound(u.P, ix, "", nil, keep(inc)); ok {
			return result(stmts, reread)
		}
	}
	u.X = c.objectTarget(c.thisJavaTarget(c.importTarget(c.recvTarget(u.X))))
	t, _ := c.lvalue(u.X)
	if f, _ := contextAccessor(u.X, false); !builtinIncDec(t) || f != nil {
		// Also a property with context parameters', whose accessors
		// take them: x = x.inc().
		return result(c.incDecOp(u, t, keep))
	}
	if t.K != types.Invalid && !t.Flex().IsNumeric() && t.Flex() != types.CharType {
		c.errorf(u.P, "operator '%s' cannot be applied to %s", u.Op, t)
		return types.InvalidType
	}
	return t
}

// builtinIncDec reports whether x++ on a value of type t is built in, as
// the operators on it are, but not on a String or a Boolean, whose
// inc() is an extension function.
func builtinIncDec(t types.Type) bool {
	nt := t.Flex().NonNull()
	return builtinOperand(t) && nt != types.StringType && nt != types.BoolType
}

// isInitialized checks this::x.isInitialized, ::x.isInitialized or
// obj::x.isInitialized for a lateinit property x, whose field it reads:
// the receiver becomes the member's, or none for a top-level property.
func (c *Checker) isInitialized(x *syntax.Member, r *syntax.CallableRef) types.Type {
	var p *types.VarSym
	var recv syntax.Expr
	if r.X == nil {
		id := &syntax.Ident{Name: r.Name}
		id.P = x.P
		c.value(id, noWant)
		p = id.Sym
		if p != nil && p.Kind == types.Prop {
			th := &syntax.This{}
			th.P = x.P
			c.value(th, noWant)
			recv = th
		}
	} else {
		m := &syntax.Member{X: r.X, Name: r.Name}
		m.P = x.P
		c.member(m)
		p, recv = m.Prop, m.X
	}
	if p == nil || !p.Lateinit || p.Kind != types.Prop && p.Kind != types.Global {
		c.errorf(x.P, "this declaration can only be called on a reference to a lateinit property")
		return types.InvalidType
	}
	// Where its backing field is: in its class, or for a top-level one
	// in its file.
	inside := p.Kind == types.Global && p.FieldClass() == c.file.Class()
	for k := c.cls; k != nil && !inside; k = k.Outer {
		inside = k == p.Owner
	}
	if !inside {
		c.errorf(x.P, "backing field of 'var %s: %s' is not accessible at this point", p.Name, p.Type)
		return types.InvalidType
	}
	x.X, x.Prop, x.Builtin = recv, p, types.IsInitialized
	return types.BoolType
}
