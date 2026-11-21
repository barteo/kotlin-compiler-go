package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// inPlaceWarnings reports, of the file's functions whose contracts say
// they call parameters in place, callsInPlace(f, kind), the warnings of
// kotlinc's FirCallsEffectAnalyzer:
//
//   - LEAKED_IN_PLACE_LAMBDA, on the effect and on each use of f that
//     lets it escape: an initializer of a variable, an assigned value, a
//     returned value, an argument for a parameter that isn't called in
//     place (neither by a contract of its function nor as an inline
//     function's inline parameter), and any call of f, or argument, in a
//     local function, an object, a local class or a lambda that isn't
//     called in place;
//   - WRONG_INVOCATION_KIND, on the effect of a parameter that doesn't
//     leak, when the number of its calls on the paths that return
//     normally doesn't fit kind.
//
// kc judges a function only where it can count as kotlinc does: a
// parameter used in a crossinline lambda, a callable reference, a safe
// call or a context argument, or called in a loop that also jumps, in a
// lambda that jumps or in a when without else that isn't exhaustive, is
// left alone, and so is a function that never returns normally, which
// kotlinc reports as not calling f at all.
func (c *Checker) inPlaceWarnings() {
	var decls func(ds []syntax.Decl)
	decls = func(ds []syntax.Decl) {
		for _, d := range ds {
			switch d := d.(type) {
			case *syntax.FunDecl:
				if len(d.Calls) > 0 && d.Body != nil && !d.Generated {
					c.inPlaceFun(d)
				}
			case *syntax.ClassDecl:
				decls(d.Members)
			}
		}
	}
	decls(c.file.Decls)
}

// inPlaceFun reports the warnings of inPlaceWarnings of function d.
func (c *Checker) inPlaceFun(d *syntax.FunDecl) {
	effects := map[*types.VarSym]syntax.CallsEffect{}
	seen := map[string]int{}
	for _, e := range d.Calls {
		seen[e.Param]++
	}
	for _, e := range d.Calls {
		if seen[e.Param] > 1 {
			continue
		}
		for _, p := range d.Params {
			if p.Name == e.Param && p.Sym != nil && !p.Vararg && p.Sym.Type.IsFunc() && !p.Sym.Type.N {
				effects[p.Sym] = e
			}
		}
	}
	if len(effects) == 0 {
		return
	}
	s := &inPlaceScan{params: effects, leaks: map[*types.VarSym][]syntax.Pos{}, unsure: map[*types.VarSym]bool{}, args: map[*syntax.Lambda]lambdaArg{}}
	s.block(d.Body, true)
	for _, p := range d.Params {
		e, ok := effects[p.Sym]
		if !ok || s.unsure[p.Sym] {
			continue
		}
		if uses := s.leaks[p.Sym]; len(uses) > 0 {
			c.warnf(e.P, "LEAKED_IN_PLACE_LAMBDA", "leaked in-place lambda '%s: %s'", p.Name, p.Sym.Type)
			for _, u := range uses {
				c.warnf(u, "LEAKED_IN_PLACE_LAMBDA", "leaked in-place lambda '%s: %s'", p.Name, p.Sym.Type)
			}
			continue
		}
		required, ok := invocationRanges[e.Kind]
		if !ok {
			continue
		}
		found, ok := countInvocations(d.Body, p.Sym)
		if !ok || found.contains(required) {
			continue
		}
		c.warnf(e.P, "WRONG_INVOCATION_KIND", "wrong invocation kind '%s' for '%s: %s' specified, the actual invocation kind is '%s'", e.Kind, p.Name, p.Sym.Type, found.coerced())
	}
}

// inPlaceScan finds the uses of the parameters called in place that
// leak them, as FirCallsEffectAnalyzer's findNonInPlaceUsesOf.
type inPlaceScan struct {
	params map[*types.VarSym]syntax.CallsEffect
	leaks  map[*types.VarSym][]syntax.Pos
	// unsure marks the parameters used where kc can't tell what kotlinc
	// counts.
	unsure map[*types.VarSym]bool
	// args are the lambdas passed as arguments: whether each runs in
	// place, by its parameter, or is crossinline.
	args map[*syntax.Lambda]lambdaArg
}

type lambdaArg struct{ inPlace, crossinline bool }

// param returns the parameter that e reads, if one of s's.
func (s *inPlaceScan) param(e syntax.Expr) *syntax.Ident {
	if e == nil {
		return nil
	}
	id, ok := unchecked(e).(*syntax.Ident)
	if !ok || id.Sym == nil || id.Member != nil || id.Call != nil {
		return nil
	}
	if _, ok := s.params[id.Sym]; !ok {
		return nil
	}
	return id
}

// mark records a leak of the parameter that e reads, if any.
func (s *inPlaceScan) mark(e syntax.Expr) {
	if id := s.param(e); id != nil {
		s.leaks[id.Sym] = append(s.leaks[id.Sym], id.P)
	}
}

// doubt marks the parameters node mentions as ones kc can't judge.
func (s *inPlaceScan) doubt(node any) {
	syntax.Inspect(node, func(n any) bool {
		if id, ok := n.(*syntax.Ident); ok && id.Sym != nil {
			if _, ok := s.params[id.Sym]; ok {
				s.unsure[id.Sym] = true
			}
		}
		return true
	})
}

func (s *inPlaceScan) block(b *syntax.Block, valid bool) {
	if b != nil {
		s.node(b, valid)
	}
}

// node scans node, code that runs in place if valid.
func (s *inPlaceScan) node(node any, valid bool) {
	syntax.Inspect(node, func(n any) bool {
		switch n := n.(type) {
		case *syntax.VarStmt:
			s.mark(n.Init)
			s.mark(n.Delegate)
		case *syntax.Assign:
			if n.Op == syntax.ASSIGN {
				s.mark(n.Value)
			}
		case *syntax.Jump:
			if n.Kind == syntax.RETURN {
				s.mark(n.X)
			}
		case *syntax.Call:
			s.call(n, valid)
		case *syntax.Lambda:
			a, isArg := s.args[n]
			switch {
			case n.Ref != nil:
				s.doubt(n)
			case isArg:
				// A crossinline lambda doesn't run in place.
				s.block(n.Body, valid && a.inPlace && !a.crossinline)
			default:
				s.block(n.Body, false)
			}
			return false
		case *syntax.LocalFun:
			for _, p := range n.Decl.Params {
				if p.Default != nil {
					s.node(p.Default, false)
				}
			}
			s.block(n.Decl.Body, false)
			if n.Decl.ExprBody != nil {
				s.node(n.Decl.ExprBody, false)
			}
			return false
		case *syntax.ObjectExpr:
			s.class(n.Decl)
			return false
		case *syntax.LocalClass:
			s.class(n.Decl)
			return false
		}
		return true
	})
}

// class scans local class or object d, whose code doesn't run in place;
// its properties' initializers and delegates leak what they read.
func (s *inPlaceScan) class(d *syntax.ClassDecl) {
	for _, m := range d.Members {
		if p, ok := m.(*syntax.PropDecl); ok {
			s.mark(p.Init)
		}
	}
	inspectClass(d, func(n any) bool {
		s.node(n, false)
		return false
	})
}

// call scans the receiver and the arguments of call x.
func (s *inPlaceScan) call(x *syntax.Call, valid bool) {
	for k, a := range x.Args {
		if l, ok := a.(*syntax.Lambda); ok {
			_, in := argRange(x, k)
			i := argIndex(x, k)
			s.args[l] = lambdaArg{in, i >= 0 && isInlineFun(x.Sym) && crossinlineParam(x.Sym, i)}
		}
	}
	if l, ok := x.Fun.(*syntax.Lambda); ok && x.Invoke {
		s.doubt(l)
	}
	for _, a := range x.Ctx {
		if s.param(a) != nil {
			s.doubt(a)
		}
	}
	// The receiver: invoke's is called exactly once.
	switch f := x.Fun.(type) {
	case *syntax.Member:
		if s.param(f.X) == nil {
			break
		}
		switch {
		case f.Safe || x.Sym == nil:
			s.doubt(f.X)
		case f.Name == "invoke" && x.Sym.Owner != nil && x.Sym.Owner.Func:
			if !valid {
				s.mark(f.X)
			}
		case x.Sym.Ext && x.Sym.Recv.IsFunc():
			// Its contract may call its receiver in place, which kc
			// doesn't read.
			s.doubt(f.X)
		default:
			s.mark(f.X)
		}
	default:
		if x.Invoke && s.param(x.Fun) != nil && !valid {
			s.mark(x.Fun)
		}
	}
	for k, a := range x.Args {
		if s.param(a) == nil {
			continue
		}
		if x.Sym == nil && x.Ctor == nil {
			s.doubt(a)
			continue
		}
		if _, ok := argRange(x, k); !ok || !valid {
			s.mark(a)
		}
	}
}

// argIndex returns the index of the parameter of x.Sym that call x
// passes its argument k to, or -1.
func argIndex(x *syntax.Call, k int) int {
	f := x.Sym
	if f == nil || x.SAM || x.Literal || x.Ctor != nil || x.Invoke {
		return -1
	}
	i := k
	switch {
	case x.ArgMap != nil:
		if k >= len(x.ArgMap) {
			return -1
		}
		i = x.ArgMap[k]
	case k < len(x.Names) && x.Names[k] != "":
		i = -1
		for j, p := range f.Params {
			if p.Name == x.Names[k] {
				i = j
			}
		}
	}
	if i < 0 || i >= len(f.Params) {
		return -1
	}
	return i
}

// argRange returns how many times call x calls its argument k in place:
// by its function's contract, or any number of times as an inline
// function's inline parameter. ok is false if it doesn't call it in place.
func argRange(x *syntax.Call, k int) (r occurrences, ok bool) {
	i := argIndex(x, k)
	if i < 0 {
		return occurrences{}, false
	}
	f := x.Sym
	at := func(bs []bool) bool { return i < len(bs) && bs[i] }
	switch {
	case at(f.Once):
		return occurrences{1, 1}, true
	case at(f.AtLeastOnce):
		return occurrences{1, many}, true
	case at(f.AtMostOnce):
		return occurrences{0, 1}, true
	case at(f.InPlace):
		return occurrences{0, many}, true
	case isInlineFun(f) && inlineParam(f, i) && !crossinlineParam(f, i):
		return occurrences{0, many}, true
	}
	return occurrences{}, false
}

// crossinlineParam reports whether parameter i of inline function f is
// crossinline.
func crossinlineParam(f *types.FunSym, i int) bool {
	if d, ok := f.Decl.(*syntax.FunDecl); ok {
		return i < len(d.Params) && d.Params[i].Crossinline
	}
	return f.Declared != nil && i < len(f.Declared.Crossinline) && f.Declared.Crossinline[i]
}

// many is the upper end of occurrences that may be more than one.
const many = 2

// occurrences is a range of the number of times something happens, as
// kotlinc's EventOccurrencesRange: from lo to hi, each 0, 1 or 2 for
// more than once.
type occurrences struct{ lo, hi int8 }

var invocationRanges = map[string]occurrences{
	"AT_MOST_ONCE":  {0, 1},
	"EXACTLY_ONCE":  {1, 1},
	"AT_LEAST_ONCE": {1, many},
	"UNKNOWN":       {0, many},
}

func (o occurrences) plus(p occurrences) occurrences {
	return occurrences{min(o.lo+p.lo, many), min(o.hi+p.hi, many)}
}

func (o occurrences) join(p occurrences) occurrences {
	return occurrences{min(o.lo, p.lo), max(o.hi, p.hi)}
}

// contains reports whether r contains o.
func (o occurrences) contains(r occurrences) bool {
	return r.lo <= o.lo && o.hi <= r.hi
}

// coerced names o as an InvocationKind, as kotlinc's message does.
func (o occurrences) coerced() string {
	switch o {
	case occurrences{1, 1}:
		return "EXACTLY_ONCE"
	case occurrences{0, 0}, occurrences{0, 1}:
		return "AT_MOST_ONCE"
	case occurrences{1, many}, occurrences{2, many}:
		return "AT_LEAST_ONCE"
	}
	return "UNKNOWN"
}

// countInvocations returns how many times body calls p on the paths that
// return normally, and whether kc can tell.
func countInvocations(body *syntax.Block, p *types.VarSym) (occurrences, bool) {
	k := &invocationCounter{p: p}
	end := k.stmts(body.Stmts, flowCount{live: true})
	if k.unsure {
		return occurrences{}, false
	}
	exits := k.exits
	if end.live {
		exits = append(exits, end.o)
	}
	if len(exits) == 0 {
		return occurrences{}, false
	}
	r := exits[0]
	for _, e := range exits[1:] {
		r = r.join(e)
	}
	return r, true
}

// flowCount is the number of calls on the paths that reach a point of the
// code, if any do.
type flowCount struct {
	live bool
	o    occurrences
}

func (f flowCount) join(g flowCount) flowCount {
	switch {
	case !f.live:
		return g
	case !g.live:
		return f
	}
	return flowCount{true, f.o.join(g.o)}
}

func (f flowCount) plus(o occurrences) flowCount {
	if !f.live {
		return f
	}
	return flowCount{true, f.o.plus(o)}
}

// invocationCounter counts the calls of p along the structure of a
// function's body.
type invocationCounter struct {
	p      *types.VarSym
	unsure bool
	// exits are the counts at the returns from the function.
	exits []occurrences
	// tries are the try blocks around the code: the counts at the returns
	// and throws in each, which go through its finally block.
	tries []*tryCount
}

type tryCount struct {
	returns []occurrences
	// hi is the most calls on a path that may throw in the try.
	hi int8
}

// mentions reports whether node reads p.
func (k *invocationCounter) mentions(node any) bool {
	found := false
	syntax.Inspect(node, func(n any) bool {
		if id, ok := n.(*syntax.Ident); ok && id.Sym == k.p {
			found = true
		}
		return !found
	})
	return found
}

// isP reports whether e reads p.
func (k *invocationCounter) isP(e syntax.Expr) bool {
	id, ok := unchecked(e).(*syntax.Ident)
	return ok && id.Sym == k.p && id.Member == nil && id.Call == nil
}

func (k *invocationCounter) stmts(ss []syntax.Stmt, f flowCount) flowCount {
	for _, s := range ss {
		if !f.live || k.unsure {
			return f
		}
		f = k.node(s, f)
	}
	return f
}

func (k *invocationCounter) block(b *syntax.Block, f flowCount) flowCount {
	if b == nil {
		return f
	}
	return k.stmts(b.Stmts, f)
}

// ret records a return from the function at count o.
func (k *invocationCounter) ret(o occurrences) {
	if n := len(k.tries); n > 0 {
		t := k.tries[n-1]
		t.returns = append(t.returns, o)
		return
	}
	k.exits = append(k.exits, o)
}

// throws records a path that may throw at count o: in a try, its catch
// blocks may run.
func (k *invocationCounter) throws(o occurrences) {
	if n := len(k.tries); n > 0 {
		k.tries[n-1].hi = max(k.tries[n-1].hi, o.hi)
	}
}

// node returns the count after statement or expression n, reached at f.
func (k *invocationCounter) node(n any, f flowCount) flowCount {
	if !f.live || k.unsure {
		return f
	}
	k.throws(f.o)
	if !k.mentions(n) {
		k.plainReturns(n, f.o)
		f.live = completes(n)
		return f
	}
	switch n := n.(type) {
	case *syntax.Block:
		return k.block(n, f)
	case *syntax.ExprStmt:
		return k.node(n.X, f)
	case *syntax.VarStmt:
		if n.Delegate != nil {
			break
		}
		return k.node(n.Init, f)
	case *syntax.Assign:
		if n.Stmts != nil {
			break
		}
		f = k.node(n.Target, f)
		return k.node(n.Value, f)
	case *syntax.Checked:
		return k.node(n.X, f)
	case *syntax.Ident:
		return f
	case *syntax.If:
		f = k.node(n.Cond, f)
		t := k.block(n.Then, f)
		e := f
		if n.Else != nil {
			e = k.block(n.Else, f)
		}
		return t.join(e)
	case *syntax.When:
		return k.when(n, f)
	case *syntax.Binary:
		return k.binary(n, f)
	case *syntax.Call:
		return k.call(n, f)
	case *syntax.Jump:
		f = k.node(n.X, f)
		switch {
		case n.Kind == syntax.RETURN && n.Lambda == nil && n.Label == "":
			k.ret(f.o)
		case n.Kind == syntax.THROW:
			k.throws(f.o)
		default:
			k.unsure = true
		}
		return flowCount{}
	case *syntax.While:
		return k.loop(n, n.Cond, n.Body, n.Do, f)
	case *syntax.For:
		if n.IterVar != nil || n.HasNextX != nil || k.mentions(n.Range) {
			break
		}
		return k.loop(n, nil, n.Body, false, f)
	case *syntax.Try:
		return k.try(n, f)
	}
	k.unsure = true
	return f
}

// plainReturns records the returns from the function in n, code that
// doesn't read p, all at count o.
func (k *invocationCounter) plainReturns(n any, o occurrences) {
	syntax.Inspect(n, func(m any) bool {
		switch m := m.(type) {
		case *syntax.LocalFun, *syntax.ObjectExpr, *syntax.LocalClass:
			return false
		case *syntax.Lambda:
			return m.Inline && !m.Crossinline && m.Ref == nil
		case *syntax.Jump:
			if m.Kind == syntax.RETURN && m.Lambda == nil {
				if m.Label != "" {
					k.unsure = true
				}
				k.ret(o)
			}
		case syntax.Expr:
			if t := m.Type(); t.K == types.Nothing && !t.N {
				k.throws(o)
			}
		}
		return true
	})
}

// completes reports whether n, code that doesn't read p, may complete
// normally.
func completes(n any) bool {
	switch n := n.(type) {
	case nil:
		return true
	case *syntax.Block:
		if n == nil {
			return true
		}
		for _, s := range n.Stmts {
			if !completes(s) {
				return false
			}
		}
		return true
	case *syntax.ExprStmt:
		return completes(n.X)
	case *syntax.VarStmt:
		return n.Init == nil || completes(n.Init)
	case *syntax.Assign:
		return completes(n.Value)
	case *syntax.While:
		if n.Infinite {
			return false
		}
		return !isTrue(unchecked(n.Cond)) || hasBreak(n.Body, n)
	case *syntax.For:
		return true
	case *syntax.If:
		if !completes(n.Cond) {
			return false
		}
		return n.Else == nil || completes(n.Then) || completes(n.Else)
	case *syntax.Try:
		ok := completes(n.Body)
		for _, c := range n.Catches {
			ok = ok || completes(c.Body)
		}
		return ok && (n.Finally == nil || completes(n.Finally))
	case syntax.Expr:
		t := n.Type()
		return !(t.K == types.Nothing && !t.N)
	}
	return true
}

// hasBreak reports whether body has a break out of loop.
func hasBreak(body *syntax.Block, loop syntax.Stmt) bool {
	found := false
	syntax.Inspect(body, func(n any) bool {
		if j, ok := n.(*syntax.Jump); ok && j.Kind == syntax.BREAK && j.Loop == loop {
			found = true
		}
		return !found
	})
	return found
}

// hasJump reports whether n has a return, break or continue, or a lambda
// that isn't inlined: code whose paths kc doesn't follow.
func hasJump(n any) bool {
	found := false
	syntax.Inspect(n, func(m any) bool {
		switch m := m.(type) {
		case *syntax.Jump:
			found = found || m.Kind != syntax.THROW
		case *syntax.Lambda:
			return false
		}
		return !found
	})
	return found
}

// loop counts loop n, while (cond) body or do body while (cond), or a
// for loop without cond, whose range doesn't read p.
func (k *invocationCounter) loop(n syntax.Stmt, cond syntax.Expr, body *syntax.Block, do bool, f flowCount) flowCount {
	if hasJump(body) || hasJump(cond) || cond != nil && isTrue(unchecked(cond)) {
		k.unsure = true
		return f
	}
	// One run of the body and the condition after it, from no calls.
	saved := k.exits
	one := k.node(cond, k.block(body, flowCount{live: true}))
	if do {
		one = k.block(body, flowCount{live: true})
		one = k.node(cond, one)
	}
	k.exits = saved
	if !one.live || k.unsure {
		k.unsure = true
		return f
	}
	if do {
		// The body runs at least once.
		f = f.plus(one.o)
	} else {
		f = k.node(cond, f)
	}
	if one.o.hi > 0 {
		f.o.hi = many
	}
	k.throws(f.o)
	return f
}

// when counts when x: only one whose conditions don't read p, and with
// an else branch or exhaustive.
func (k *invocationCounter) when(x *syntax.When, f flowCount) flowCount {
	f = k.node(x.Subject, f)
	hasElse := false
	for _, br := range x.Branches {
		hasElse = hasElse || br.Else
		if k.mentions(br.Guard) {
			k.unsure = true
			return f
		}
		for _, c := range br.Conds {
			if k.mentions(c.X) || c.Call != nil && k.mentions(c.Call) {
				k.unsure = true
				return f
			}
		}
	}
	if !hasElse && x.NoMatch == "" {
		k.unsure = true
		return f
	}
	out := flowCount{}
	for _, br := range x.Branches {
		out = out.join(k.block(br.Body, f))
	}
	return out
}

func (k *invocationCounter) binary(x *syntax.Binary, f flowCount) flowCount {
	if x.Call != nil {
		k.unsure = true
		return f
	}
	f = k.node(x.X, f)
	switch x.Op {
	case syntax.AND, syntax.OR, syntax.ELVIS:
		return f.join(k.node(x.Y, f))
	}
	return k.node(x.Y, f)
}

// call counts call x: its receiver, its arguments, then its calls of p.
func (k *invocationCounter) call(x *syntax.Call, f flowCount) flowCount {
	if len(x.Ctx) > 0 && k.mentions(x.Ctx) || x.Value != nil || x.Sym == nil && x.Ctor == nil {
		k.unsure = true
		return f
	}
	var calls []occurrences
	switch fn := x.Fun.(type) {
	case *syntax.Member:
		if fn.Safe {
			k.unsure = true
			return f
		}
		if k.isP(fn.X) {
			if fn.Name != "invoke" || x.Sym.Owner == nil || !x.Sym.Owner.Func {
				k.unsure = true
				return f
			}
			calls = append(calls, occurrences{1, 1})
		} else {
			f = k.node(fn.X, f)
		}
	case *syntax.Ident:
		if x.Invoke && k.isP(fn) {
			calls = append(calls, occurrences{1, 1})
		} else if fn.Sym == k.p {
			k.unsure = true
			return f
		}
	default:
		f = k.node(fn, f)
	}
	for i, a := range x.Args {
		switch {
		case k.isP(a):
			r, ok := argRange(x, i)
			if !ok {
				k.unsure = true
				return f
			}
			calls = append(calls, r)
		case isLambdaArg(a) && k.mentions(a):
			l := a.(*syntax.Lambda)
			r, ok := argRange(x, i)
			j := argIndex(x, i)
			if !ok || isInlineFun(x.Sym) && crossinlineParam(x.Sym, j) || hasJump(l.Body) {
				k.unsure = true
				return f
			}
			saved := k.tries
			k.tries = nil
			in := k.block(l.Body, flowCount{live: true})
			k.tries = saved
			if !in.live {
				k.unsure = true
				return f
			}
			calls = append(calls, times(in.o, r))
			k.throws(f.o.plus(times(in.o, r)))
		default:
			f = k.node(a, f)
		}
		if k.unsure || !f.live {
			return f
		}
	}
	for _, c := range calls {
		f = f.plus(c)
	}
	if t := x.Type(); t.K == types.Nothing && !t.N {
		k.throws(f.o)
		return flowCount{}
	}
	return f
}

func isLambdaArg(a syntax.Expr) bool {
	l, ok := a.(*syntax.Lambda)
	return ok && l.Ref == nil
}

// times returns the calls of a lambda's body, o, when the lambda runs r
// times.
func times(o, r occurrences) occurrences {
	if o.hi == 0 {
		return o
	}
	switch r {
	case occurrences{1, 1}:
		return o
	case occurrences{0, 1}:
		return occurrences{0, o.hi}
	case occurrences{1, many}:
		return occurrences{o.lo, many}
	}
	return occurrences{0, many}
}

// try counts try x: its catch blocks may start at any count reached in
// its body, and its finally block runs on the paths that leave it,
// adding its calls to those that complete or return.
func (k *invocationCounter) try(x *syntax.Try, f flowCount) flowCount {
	var fin occurrences
	if x.Finally != nil {
		if hasJump(x.Finally) {
			k.unsure = true
			return f
		}
		saved := k.tries
		k.tries = nil
		ff := k.block(x.Finally, flowCount{live: true})
		k.tries = saved
		if !ff.live {
			k.unsure = true
			return f
		}
		fin = ff.o
	}
	t := &tryCount{hi: f.o.hi}
	k.tries = append(k.tries, t)
	body := k.block(x.Body, f)
	if body.live {
		t.hi = max(t.hi, body.o.hi)
	}
	out := body
	entry := flowCount{true, occurrences{f.o.lo, t.hi}}
	for _, c := range x.Catches {
		out = out.join(k.block(c.Body, entry))
	}
	k.tries = k.tries[:len(k.tries)-1]
	if k.unsure {
		return f
	}
	// What the catch blocks don't catch leaves through the finally block.
	k.throws(occurrences{0, t.hi}.plus(fin))
	for _, r := range t.returns {
		k.ret(r.plus(fin))
	}
	return out.plus(fin)
}
