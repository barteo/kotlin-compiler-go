package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Definite assignment: a local or a class property declared without a
// value (types.VarSym.Deferred) must be assigned before it is read, and a
// val only once. The analysis walks a checked body as its code runs:
// branches are walked apart and joined, a loop's body twice, so that a val
// assigned in it is assigned again, and code after a jump is dead. A
// variable is definitely assigned after a join if it is on every path
// that reaches it, and maybe assigned if it is on any.

// daState is what is known at one point of the code.
type daState struct {
	def, maybe map[*types.VarSym]bool
	dead       bool // no path reaches this point
}

func newDAState() *daState {
	return &daState{def: map[*types.VarSym]bool{}, maybe: map[*types.VarSym]bool{}}
}

func (s *daState) clone() *daState {
	c := &daState{def: make(map[*types.VarSym]bool, len(s.def)), maybe: make(map[*types.VarSym]bool, len(s.maybe)), dead: s.dead}
	for v := range s.def {
		c.def[v] = true
	}
	for v := range s.maybe {
		c.maybe[v] = true
	}
	return c
}

// mergeMaybe adds what o maybe assigns to s.
func (s *daState) mergeMaybe(o *daState) {
	for v := range o.maybe {
		s.maybe[v] = true
	}
}

// join returns the state where the paths of states meet.
func join(states ...*daState) *daState {
	var out *daState
	for _, s := range states {
		switch {
		case s.dead:
		case out == nil:
			out = s.clone()
		default:
			for v := range out.def {
				if !s.def[v] {
					delete(out.def, v)
				}
			}
			out.mergeMaybe(s)
		}
	}
	if out == nil {
		out = newDAState()
		out.dead = true
		for _, s := range states {
			out.mergeMaybe(s)
		}
	}
	return out
}

// assignment checks the definite assignment of a function body or of a
// class's constructor code.
type assignment struct {
	c        *Checker
	cls      *types.ClassSym        // the class whose properties are tracked, if any
	depth    map[*types.VarSym]int  // tracked variables, by the lambda depth they are declared at
	lambdas  int                    // the lambdas and local functions the code is in
	quiet    int                    // >0 while a loop's body is walked the first time
	reported map[*types.VarSym]bool // variables reported as read before assignment
	loops    []*daLoop              // the loops the code is in, innermost last
	once     []*daOnce              // the lambdas called once that the code is in
}

// daOnce records the states at the returns from a lambda that its
// function calls exactly once.
type daOnce struct {
	l       *syntax.Lambda
	returns []*daState
}

// calledOnce reports whether call x calls the lambda it takes as its
// argument k exactly once, as the callee's contract says.
func calledOnce(x *syntax.Call, k int) bool {
	f := x.Sym
	if f == nil || f.Once == nil {
		return false
	}
	i := k
	if x.ArgMap != nil {
		i = x.ArgMap[k]
	}
	return i < len(f.Once) && f.Once[i]
}

// inPlace walks a lambda that the call it is passed to calls exactly
// once as if its body were the call's code: what it assigns is assigned
// after the call.
func (a *assignment) inPlace(l *syntax.Lambda, s *daState) {
	o := &daOnce{l: l}
	a.once = append(a.once, o)
	a.block(l.Body, s)
	a.once = a.once[:len(a.once)-1]
	*s = *join(append(o.returns, s)...)
}

// daLoop records the states at a loop's breaks and continues.
type daLoop struct {
	node          syntax.Stmt
	breaks, conts []*daState
}

func (c *Checker) newAssignment(cls *types.ClassSym) *assignment {
	return &assignment{c: c, cls: cls, depth: map[*types.VarSym]int{}, reported: map[*types.VarSym]bool{}}
}

// funAssignment checks a function's body.
func (c *Checker) funAssignment(d *syntax.FunDecl) {
	a := c.newAssignment(nil)
	s := newDAState()
	if d.Body != nil {
		a.block(d.Body, s)
	} else if d.ExprBody != nil {
		a.expr(d.ExprBody, s)
	}
}

// classAssignment checks that a class's properties declared without a
// value are assigned by its constructor code: the property initializers
// and init blocks, in order, then the body of each secondary constructor
// that calls the superclass's, if the class has no primary constructor.
func (c *Checker) classAssignment(d *syntax.ClassDecl) {
	a := c.newAssignment(d.Sym)
	var props []*syntax.PropDecl
	for _, m := range d.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && pd.Sym != nil && pd.Sym.Deferred && pd.Recv == nil {
			props = append(props, pd)
			a.depth[pd.Sym] = 0
		}
	}
	s := newDAState()
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Init != nil && m.Getter == nil {
				a.expr(m.Init, s)
			}
		case *syntax.InitBlock:
			a.block(m.Body, s)
		}
	}
	missing := map[*syntax.PropDecl]bool{}
	check := func(end *daState) {
		for _, pd := range props {
			if !end.dead && !end.def[pd.Sym] {
				missing[pd] = true
			}
		}
	}
	secs := d.Secondaries()
	if !d.HasPrimary {
		for _, sc := range secs {
			if sc.ToThis || sc.Body == nil {
				continue
			}
			end := s.clone()
			a.block(sc.Body, end)
			check(end)
		}
	} else {
		check(s)
		for _, sc := range secs {
			// After the primary constructor: a val it assigns is assigned
			// again.
			if sc.Body != nil {
				a.block(sc.Body, s.clone())
			}
		}
	}
	for _, pd := range props {
		if missing[pd] {
			c.errorf(pd.P, "property must be initialized or be abstract")
		}
	}
}

// tracked reports whether v is a variable whose assignment is checked.
func (a *assignment) tracked(v *types.VarSym) bool {
	_, ok := a.depth[v]
	return ok
}

// read checks a read of v.
func (a *assignment) read(v *types.VarSym, p syntax.Pos, s *daState) {
	if v == nil || !a.tracked(v) || s.def[v] || s.dead || a.quiet > 0 || a.reported[v] {
		return
	}
	if v.Kind == types.Prop && a.lambdas > 0 {
		return // a lambda may run once the property is assigned
	}
	a.reported[v] = true
	a.c.errorf(p, "variable '%s' must be initialized", v.Name)
}

// assign records an assignment of v.
func (a *assignment) assign(v *types.VarSym, p syntax.Pos, s *daState) {
	if v == nil || !a.tracked(v) {
		return
	}
	switch {
	case a.quiet > 0 || s.dead:
	case !v.Mutable && a.lambdas > a.depth[v]:
		a.c.errorf(p, "captured values cannot be initialized because of possible reassignments")
	case !v.Mutable && s.maybe[v]:
		a.c.errorf(p, "val cannot be reassigned")
	}
	if !v.Mutable && v.Kind == types.Local {
		for _, o := range a.once {
			if !o.l.Inline && slices.Contains(o.l.Captures, v) {
				v.InitInLambda = true
			}
		}
	}
	s.def[v], s.maybe[v] = true, true
}

func (a *assignment) block(b *syntax.Block, s *daState) {
	if b == nil {
		return
	}
	for _, st := range b.Stmts {
		a.stmt(st, s)
	}
}

func (a *assignment) stmt(st syntax.Stmt, s *daState) {
	switch st := st.(type) {
	case *syntax.Block:
		a.block(st, s)
	case *syntax.ExprStmt:
		a.expr(st.X, s)
	case *syntax.VarStmt:
		if st.Init != nil {
			a.expr(st.Init, s)
		}
		if st.Delegate != nil {
			a.expr(st.Delegate, s)
		}
		if v := st.Sym; v != nil && v.Deferred {
			a.depth[v] = a.lambdas
			delete(s.def, v)
			delete(s.maybe, v)
		}
	case *syntax.Destructure:
		for _, vs := range st.Stmts {
			a.stmt(vs, s)
		}
	case *syntax.Assign:
		a.assignStmt(st, s)
	case *syntax.LocalFun:
		a.lambdas++
		fs := s.clone()
		if st.Decl.Body != nil {
			a.block(st.Decl.Body, fs)
		} else if st.Decl.ExprBody != nil {
			a.expr(st.Decl.ExprBody, fs)
		}
		a.lambdas--
		s.mergeMaybe(fs)
	case *syntax.While:
		if st.Do {
			a.doLoop(st, s)
			break
		}
		a.expr(st.Cond, s)
		a.loop(st, st.Body, s, isTrue(st.Cond))
	case *syntax.For:
		a.expr(st.Range, s)
		a.loop(st, st.Body, s, false)
	}
}

func isTrue(x syntax.Expr) bool {
	lit, ok := x.(*syntax.BoolLit)
	return ok && lit.Val
}

// loop walks a loop's body, which may run any number of times: twice, so
// that the second sees what the first assigns. After the loop, what it
// assigns is maybe assigned, unless the loop only ends by breaking.
func (a *assignment) loop(node syntax.Stmt, body *syntax.Block, s *daState, forever bool) {
	first := s.clone()
	a.quiet++
	a.loops = append(a.loops, &daLoop{node: node})
	a.block(body, first)
	a.loops = a.loops[:len(a.loops)-1]
	a.quiet--
	second := s.clone()
	second.mergeMaybe(first)
	l := &daLoop{node: node}
	a.loops = append(a.loops, l)
	a.block(body, second)
	a.loops = a.loops[:len(a.loops)-1]
	breaks := l.breaks
	if forever {
		*s = *join(breaks...)
		return
	}
	s.mergeMaybe(second)
	for _, b := range breaks {
		s.mergeMaybe(b)
	}
}

// doLoop walks a do-while loop, whose body runs at least once: what it
// assigns before the condition is assigned after the loop.
func (a *assignment) doLoop(st *syntax.While, s *daState) {
	pass := func(in *daState) (end *daState, l *daLoop) {
		l = &daLoop{node: st}
		a.loops = append(a.loops, l)
		a.block(st.Body, in)
		a.loops = a.loops[:len(a.loops)-1]
		end = join(append([]*daState{in}, l.conts...)...)
		a.expr(st.Cond, end)
		return end, l
	}
	if lit, ok := st.Cond.(*syntax.BoolLit); ok && !lit.Val {
		// do { } while (false) runs its body once.
		end, l := pass(s.clone())
		*s = *join(append([]*daState{end}, l.breaks...)...)
		return
	}
	a.quiet++
	first, _ := pass(s.clone())
	a.quiet--
	second := s.clone()
	second.mergeMaybe(first)
	end, l := pass(second)
	if isTrue(st.Cond) {
		*s = *join(l.breaks...)
		return
	}
	*s = *join(append([]*daState{end}, l.breaks...)...)
}

func (a *assignment) assignStmt(st *syntax.Assign, s *daState) {
	if st.Stmts != nil {
		for _, x := range st.Stmts {
			a.stmt(x, s)
		}
		return
	}
	var v *types.VarSym
	switch t := st.Target.(type) {
	case *syntax.Ident:
		v = t.Sym
	case *syntax.Member:
		if _, this := t.X.(*syntax.This); this {
			v = t.Prop
		} else {
			a.expr(t.X, s)
		}
	case *syntax.Index:
		a.expr(t.X, s)
		for _, x := range t.Args {
			a.expr(x, s)
		}
	}
	if st.Op != syntax.ASSIGN {
		a.read(v, st.P, s)
	}
	a.expr(st.Value, s)
	a.assign(v, st.Target.Pos(), s)
}

func (a *assignment) expr(x syntax.Expr, s *daState) {
	switch x := x.(type) {
	case nil:
		return
	case *syntax.Ident:
		a.read(x.Sym, x.P, s)
	case *syntax.StringLit:
		for _, p := range x.Parts {
			a.expr(p.X, s)
		}
	case *syntax.Unary:
		if x.Stmts != nil {
			for _, st := range x.Stmts {
				a.stmt(st, s)
			}
			break
		}
		a.expr(x.X, s)
		if id, ok := x.X.(*syntax.Ident); ok && (x.Op == syntax.INC || x.Op == syntax.DEC) {
			a.assign(id.Sym, id.P, s)
		}
	case *syntax.Binary:
		a.expr(x.X, s)
		switch x.Op {
		case syntax.AND, syntax.OR, syntax.ELVIS:
			// The right side may not run.
			y := s.clone()
			a.expr(x.Y, y)
			s.mergeMaybe(y)
		default:
			a.expr(x.Y, s)
		}
	case *syntax.ClassLit:
		if x.Bound {
			a.expr(x.X, s)
		}
	case *syntax.Checked:
		a.expr(x.X, s)
	case *syntax.Member:
		if x.Static != nil || x.Object != nil {
			break
		}
		if _, this := x.X.(*syntax.This); this && x.Prop != nil {
			a.read(x.Prop, x.P, s)
			break
		}
		a.expr(x.X, s)
	case *syntax.Index:
		a.expr(x.X, s)
		for _, arg := range x.Args {
			a.expr(arg, s)
		}
	case *syntax.Call:
		switch f := x.Fun.(type) {
		case *syntax.Member:
			a.expr(f, s)
		case *syntax.Ident:
			if x.Invoke {
				a.expr(f, s)
			}
		default:
			a.expr(f, s)
		}
		for k, arg := range x.Args {
			if l, ok := arg.(*syntax.Lambda); ok && calledOnce(x, k) {
				a.inPlace(l, s)
				continue
			}
			a.expr(arg, s)
		}
	case *syntax.Lambda:
		if x.Ref != nil {
			a.expr(x.Ref.BoundExpr, s)
		}
		a.lambdas++
		ls := s.clone()
		a.block(x.Body, ls)
		a.lambdas--
		s.mergeMaybe(ls)
	case *syntax.NotNull:
		a.expr(x.X, s)
	case *syntax.Spread:
		a.expr(x.X, s)
	case *syntax.If:
		a.expr(x.Cond, s)
		then, els := s.clone(), s.clone()
		a.block(x.Then, then)
		a.block(x.Else, els)
		*s = *join(then, els)
	case *syntax.When:
		a.when(x, s)
	case *syntax.Is:
		a.expr(x.X, s)
	case *syntax.As:
		a.expr(x.X, s)
	case *syntax.Try:
		entry := s.clone()
		body := s.clone()
		a.block(x.Body, body)
		outs := []*daState{body}
		for _, c := range x.Catches {
			cs := entry.clone()
			cs.mergeMaybe(body)
			a.block(c.Body, cs)
			outs = append(outs, cs)
		}
		*s = *join(outs...)
		if x.Finally != nil {
			a.block(x.Finally, s)
		}
	case *syntax.Jump:
		a.expr(x.X, s)
		for _, l := range a.once {
			if x.Kind == syntax.RETURN && x.Lambda == l.l {
				l.returns = append(l.returns, s.clone())
			}
		}
		for _, l := range a.loops {
			switch {
			case l.node != x.Loop:
			case x.Kind == syntax.BREAK:
				l.breaks = append(l.breaks, s.clone())
			case x.Kind == syntax.CONTINUE:
				l.conts = append(l.conts, s.clone())
			}
		}
		s.dead = true
	}
	if t := x.Type(); t.K == types.Nothing && !s.dead {
		s.dead = true // a call that never returns
	}
}

func (a *assignment) when(x *syntax.When, s *daState) {
	a.expr(x.Subject, s)
	var outs []*daState
	els := false
	for _, b := range x.Branches {
		for _, c := range b.Conds {
			a.expr(c.X, s)
		}
		body := s.clone()
		a.block(b.Body, body)
		outs = append(outs, body)
		els = els || b.Else
	}
	if !els && x.NoMatch == "" {
		outs = append(outs, s.clone()) // no branch matched
	}
	*s = *join(outs...)
}
