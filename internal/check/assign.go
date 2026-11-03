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
	// inited are the tracked properties with an initializer or a
	// delegate, which only reads are checked for.
	inited map[*types.VarSym]bool
	// delegated are those of inited with a delegate, whose reads by the
	// class's interface delegation kotlinc doesn't report.
	delegated map[*types.VarSym]bool
	inSupers  bool // walking the class's superclass call and delegates
	// libOnce counts the lambdas called once by functions of the class
	// path that the code is in: kotlinc doesn't report the properties
	// they read, as it does for its own functions'.
	libOnce int
	// rets are the states at the returns from the secondary constructor
	// walked, if one is.
	rets []*daState
	// comp is the companion object of the enum class whose constructor
	// code is walked, which isn't initialized before its entries are.
	comp *types.ClassSym
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
func (a *assignment) inPlace(x *syntax.Call, l *syntax.Lambda, s *daState) {
	o := &daOnce{l: l}
	a.once = append(a.once, o)
	lib := x.Sym.Decl == nil
	if lib {
		a.libOnce++
	}
	a.block(l.Body, s)
	if lib {
		a.libOnce--
	}
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
// It also checks that a property with an initializer isn't read before
// the initializer runs: by the superclass's constructor call, a delegate,
// or the code before it.
func (c *Checker) classAssignment(d *syntax.ClassDecl) {
	a := c.newAssignment(d.Sym)
	var props []*syntax.PropDecl
	for _, m := range d.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && pd.Sym != nil && pd.Sym.Deferred && pd.Recv == nil {
			props = append(props, pd)
			a.depth[pd.Sym] = 0
		}
	}
	a.trackInits(d.Members)
	s := newDAState()
	switch {
	case len(d.Entries) > 0 && d.Sym.Companion != nil:
		a.enumEntries(d)
	case d.EnumEntry && d.Sym.Super != nil && d.Sym.Super.Companion != nil:
		a.comp = d.Sym.Super.Companion // the entry's code runs first too
	}
	a.supers(d, s)
	a.inits(d.Members, s)
	a.comp = nil
	missing := map[*syntax.PropDecl]bool{}
	check := func(end *daState) {
		for _, pd := range props {
			if !end.def[pd.Sym] {
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
			a.rets = []*daState{}
			a.block(sc.Body, end)
			// A constructor that always throws needn't assign.
			if end := join(append(a.rets, end)...); !end.dead {
				check(end)
			}
			a.rets = nil
		}
	} else {
		// Code after an initializer or an init block that never ends
		// doesn't run, as kotlinc reports.
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

// fileAssignment checks that a file's top-level properties aren't read
// before their initializers run, in order, in the file's class.
func (c *Checker) fileAssignment(decls []syntax.Decl) {
	a := c.newAssignment(nil)
	a.trackInits(decls)
	if len(a.inited) > 0 {
		a.inits(decls, newDAState())
	}
}

// trackInits tracks the properties among decls whose initializer or
// delegate sets their field: they are assigned once it has run.
func (a *assignment) trackInits(decls []syntax.Decl) {
	for _, m := range decls {
		pd, ok := m.(*syntax.PropDecl)
		if !ok || pd.Recv != nil {
			continue
		}
		v := pd.Sym
		switch {
		case pd.DelegateOf != nil:
			v = pd.DelegateOf.Sym
			if a.delegated == nil {
				a.delegated = map[*types.VarSym]bool{}
			}
			a.delegated[v] = true
		case pd.Init == nil || pd.Getter != nil || pd.Field != nil:
			continue
		}
		if v == nil || v.Const || v.Deferred || pd.Const {
			continue
		}
		if a.inited == nil {
			a.inited = map[*types.VarSym]bool{}
		}
		a.inited[v] = true
		a.depth[v] = 0
	}
}

// inits walks the property initializers and init blocks among decls, in
// order.
func (a *assignment) inits(decls []syntax.Decl, s *daState) {
	for _, m := range decls {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Init == nil && s.dead && m.Sym != nil {
				// Declared after the code stops: kotlinc doesn't ask
				// that it be initialized.
				s.def[m.Sym] = true
			}
			if m.Init == nil || m.Getter != nil {
				continue
			}
			a.expr(m.Init, s)
			v := m.Sym
			if m.DelegateOf != nil {
				v = m.DelegateOf.Sym
			}
			if v != nil && a.inited[v] {
				s.def[v], s.maybe[v] = true, true
			}
		case *syntax.InitBlock:
			a.block(m.Body, s)
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
	if (v.Kind == types.Prop || v.Kind == types.Global) && a.lambdas > 0 {
		return // a lambda may run once the property is assigned
	}
	if a.inSupers && a.delegated[v] || a.libOnce > 0 && a.inited[v] {
		return
	}
	a.reported[v] = true
	a.c.errorf(p, "variable '%s' must be initialized", v.Name)
}

// assign records an assignment of v.
func (a *assignment) assign(v *types.VarSym, p syntax.Pos, s *daState) {
	if v == nil || !a.tracked(v) {
		return
	}
	if a.inited[v] {
		return // assigned by its initializer, as other checks report
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
	if !s.dead {
		s.def[v] = true
	}
	s.maybe[v] = true
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
		} else if v != nil && a.tracked(v) {
			s.def[v], s.maybe[v] = true, true // a do-while body's, as doLoop tracks
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
	// The body's variables are in scope in the condition, which a
	// continue can reach before they are declared.
	var vars []*types.VarSym
	if st.Body != nil {
		for _, b := range st.Body.Stmts {
			if vs, ok := b.(*syntax.VarStmt); ok && vs.Sym != nil && !vs.Sym.Deferred && vs.Delegate == nil {
				vars = append(vars, vs.Sym)
				a.depth[vs.Sym] = a.lambdas
			}
		}
	}
	pass := func(in *daState) (end *daState, l *daLoop) {
		for _, v := range vars {
			delete(in.def, v)
		}
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
		if a.comp != nil && (x.Object == a.comp || x.Sym != nil && x.Sym.Kind == types.Prop && x.Sym.Owner == a.comp && !x.Sym.Const) {
			a.companion(x.P)
		}
		if x.Sym == nil && x.Member != nil {
			a.expr(x.Member, s) // a member of an outer class or an object
			break
		}
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
		if x.Prop != nil && x.Prop.Const {
			break // inlined
		}
		if a.comp != nil && x.Object == a.comp {
			a.companion(x.P)
		}
		if x.Static != nil || x.Object != nil {
			break
		}
		if _, this := x.X.(*syntax.This); this && x.Prop != nil {
			a.read(x.Prop, x.P, s)
			break
		}
		if a.cls != nil && a.cls.Object && x.Prop != nil && a.inited[x.Prop] && namesObject(x.X) == a.cls {
			a.read(x.Prop, x.P, s) // Obj.x in Obj's own constructor code
			break
		}
		a.expr(x.X, s)
	case *syntax.Index:
		a.expr(x.X, s)
		for _, arg := range x.Args {
			a.expr(arg, s)
		}
	case *syntax.Call:
		if _, ok := x.Fun.(*syntax.Ident); ok && a.comp != nil && x.Sym != nil && x.Sym.Owner == a.comp {
			a.companion(x.P)
		}
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
		as := s
		if m, ok := x.Fun.(*syntax.Member); ok && m.Safe {
			as = s.clone() // the arguments don't run on null
		}
		for k, arg := range x.Args {
			if l, ok := arg.(*syntax.Lambda); ok && calledOnce(x, k) {
				a.inPlace(x, l, as)
				continue
			}
			a.expr(arg, as)
		}
		if as != s {
			s.mergeMaybe(as)
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
	case *syntax.ObjectExpr:
		a.objectExpr(x.Decl, s)
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
		end := join(outs...)
		if x.Finally != nil {
			// The finally block also runs when the body or a catch ends
			// early, by a jump or an exception: what they assign may not
			// be assigned in it.
			fs := entry.clone()
			for _, o := range outs {
				fs.mergeMaybe(o)
			}
			a.block(x.Finally, fs)
			a.quiet++
			a.block(x.Finally, end)
			a.quiet--
		}
		*s = *end
	case *syntax.Jump:
		a.expr(x.X, s)
		if x.Kind == syntax.RETURN && x.Lambda == nil && a.rets != nil && a.lambdas == 0 {
			a.rets = append(a.rets, s.clone())
		}
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

// namesObject returns the object that x names, if it does.
func namesObject(x syntax.Expr) *types.ClassSym {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Object
	case *syntax.Member:
		return x.Object
	}
	return nil
}

// objectExpr walks the code an object expression runs where it is: its
// superclass's constructor call and delegates, its property initializers
// and init blocks. Its functions run later.
func (a *assignment) objectExpr(d *syntax.ClassDecl, s *daState) {
	if d == nil {
		return
	}
	a.supers(d, s)
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
}

// supers walks a class's superclass constructor call and the objects its
// interfaces are delegated to, which run before its own code.
func (a *assignment) supers(d *syntax.ClassDecl, s *daState) {
	call := d.SuperCall
	for _, st := range d.Supers {
		if call == nil && st.Call != nil {
			call = st.Call
		}
		a.inSupers = true
		a.expr(st.Delegate, s)
		a.inSupers = false
	}
	if call != nil {
		for _, arg := range call.Args {
			a.expr(arg, s)
		}
	}
}

// enumEntries sets up the walk of an enum class's constructor code, which
// its companion object isn't initialized for, and walks the arguments of
// its entries without a body, which run first. The code of an entry with a
// body is walked with its class.
func (a *assignment) enumEntries(d *syntax.ClassDecl) {
	a.comp = d.Sym.Companion
	s := newDAState()
	for _, e := range d.Entries {
		if e.Call != nil && e.Body == nil {
			for _, arg := range e.Call.Args {
				a.expr(arg, s)
			}
		}
	}
}

// companion reports an access of the enum's companion object in its
// constructor code.
func (a *assignment) companion(p syntax.Pos) {
	if a.quiet > 0 || a.lambdas > 0 || a.libOnce > 0 {
		return
	}
	a.c.errorf(p, "companion object of enum class '%s' is uninitialized here", a.comp.Outer.Name)
}

// getterField reports whether property d's getter uses its backing field.
// In the expression body of the getter of a property declared without a
// type or a value, field isn't the backing field, which has no type yet:
// kotlinc resolves it as another name, as a property field of the class.
func getterField(d *syntax.PropDecl) bool {
	g := d.Getter
	if g != nil && d.Type == nil && d.Init == nil && g.ExprBody != nil {
		return false
	}
	return usesField(g)
}
