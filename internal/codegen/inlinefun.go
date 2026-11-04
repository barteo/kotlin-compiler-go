package codegen

import (
	"sync"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The program's inline functions (see syntax.SourceInline) are copied
// from their AST into each caller, as the stdlib's are from their
// bytecode. The arguments go to new locals, which the body's parameters
// then are, and a lambda passed for a parameter the body only invokes is
// generated where the body invokes it. A return in the body jumps to the
// end of the copy, with the result. The function's own method is still
// generated, for callable references and Java. A composable's copy knows
// its parameters' change flags, which the call computes as it would pass
// them to the method, as the plugin's code does after kotlinc inlines it.

// srcInline is a copy of an inline function's body being generated.
type srcInline struct {
	d     *syntax.FunDecl
	end   *cf.Label // where the copy leaves its result
	depth int       // the stack depth where the body starts
	tries int       // len(g.tries) where the body starts
	// lambdas are the inlined lambdas, by the parameter they are passed for.
	lambdas map[*types.VarSym]*syntax.Lambda
	// owners are the copies whose callers' code forwarded lambdas are, by
	// lambda: a copy passes its own inlined lambda parameter on to this
	// one, f(block), and invokes generate it as that copy's caller's.
	owners map[*syntax.Lambda]*srcInline
	// outer are the copies the call is in: a lambda passed to the call
	// is the caller's code, and returns from them.
	outer []*srcInline
	// noLines is the caller's gen.noLines, for the inlined lambdas.
	noLines int
	// reified are the call's type arguments for the function's reified
	// type parameters, which is checks, casts, class literals and arrays
	// in the copy use.
	reified types.Subst
	// self is the local that holds the receiver of an inline member, which
	// this is in the copy, or -1.
	self int
	// defaults is set while the parameters' default values are copied,
	// which may call the function: they aren't its body.
	defaults bool
	// groups is the number of replace groups open around the copy, which
	// its returns don't end, and inl the number of inlined lambdas it is
	// in.
	groups int
	inl    int
	// scope is the flag scope of the caller's code, its lambdas'.
	scope *flagScope
}

// inlining reports whether a copy of d is being generated: an inline
// function can't be copied into itself.
func (g *gen) inlining(d *syntax.FunDecl) bool {
	for _, s := range g.srcInl {
		if s.d == d && !s.defaults {
			return true
		}
	}
	return false
}

// declaredInCopy reports whether lambda x is in an inline function that
// a copy is being generated of, not in the caller's code.
func (g *gen) declaredInCopy(x *syntax.Lambda) bool {
	for _, s := range g.srcInl {
		m := g.shared.lambdasIn[s.d]
		if m == nil {
			m = lambdasIn(s.d)
			if g.shared.lambdasIn == nil {
				g.shared.lambdasIn = map[*syntax.FunDecl]map[*syntax.Lambda]bool{}
			}
			g.shared.lambdasIn[s.d] = m
		}
		if m[x] {
			return true
		}
	}
	return false
}

// lambdasIn returns the lambdas in the declaration of function d.
func lambdasIn(d *syntax.FunDecl) map[*syntax.Lambda]bool {
	m := map[*syntax.Lambda]bool{}
	visit := func(n any) bool {
		if l, ok := n.(*syntax.Lambda); ok {
			m[l] = true
		}
		return true
	}
	for _, p := range d.Params {
		syntax.Inspect(p.Default, visit)
	}
	syntax.Inspect(d.Body, visit)
	syntax.Inspect(d.ExprBody, visit)
	return m
}

// inlineSource generates a call of the program's inline function d: a copy
// of its body. An extension's receiver is evaluated first, and with ?.,
// a null receiver skips the call.
func (g *gen) inlineSource(x *syntax.Call, d *syntax.FunDecl) {
	if g.inlining(d) {
		panic("inline function " + d.Name + " calls itself")
	}
	f := d.Sym
	if f.Owner != nil && f.Ext {
		// A member extension: the extension receiver, then the dispatch
		// receiver, which can't have side effects.
		m := x.Fun.(*syntax.Member)
		rt := m.T
		g.safeCall(m, f.Ret, x.T, func() {
			g.contextsUnder(x, f, m.X.Type().NonNull())
			g.coerce(rt, f.Recv)
			g.dispatch(m, f.Owner)
			g.inlineBody(x, d)
		})
		return
	}
	if f.Owner != nil {
		// A member: its receiver is this in the copy.
		if m, ok := x.Fun.(*syntax.Member); ok {
			g.safeCall(m, f.Ret, x.T, func() {
				g.contextsUnder(x, f, m.X.Type().NonNull())
				g.coerce(m.T.NonNull(), f.Owner.This())
				g.inlineBody(x, d)
			})
			return
		}
		g.contextArgs(x, f)
		g.loadThis()
		g.inlineBody(x, d)
		g.coerce(f.Ret, x.T)
		return
	}
	if !f.Ext {
		g.contextArgs(x, f)
		g.inlineBody(x, d)
		g.coerce(f.Ret, x.T)
		return
	}
	m := x.Fun.(*syntax.Member)
	rt := m.T
	if !m.Safe {
		g.contextArgs(x, f)
	}
	g.safeCall(m, f.Ret, x.T, func() {
		if m.Safe {
			g.contextsUnder(x, f, m.X.Type().NonNull())
		}
		g.coerce(rt, f.Recv)
		g.inlineBody(x, d)
	})
}

// inlineBody copies d's body, for call x whose context arguments and
// receiver, if d is an extension, are on the stack. It leaves the
// result, of d's result type.
func (g *gen) inlineBody(x *syntax.Call, d *syntax.FunDecl) {
	g.lockInline()
	defer g.unlockInline()
	defer g.restoreNames(len(g.shared.renamed))
	f := d.Sym
	if len(g.reifiedArgs(x, f)) > 0 {
		g.nameObjects(d)
	}
	c := g.code
	saved := g.saveSlots(d)
	defer g.restoreSlots(saved)
	self := -1
	switch {
	case f.Ext && f.Owner != nil:
		self = c.AllocLocal("L" + f.Owner.Name + ";")
		c.Local(cf.ASTORE, self)
		g.setSlot(d.RecvSym, c.AllocLocal(desc(f.Recv)))
		c.Local(storeOp(f.Recv), g.slot(d.RecvSym))
	case f.Ext:
		g.setSlot(d.RecvSym, c.AllocLocal(desc(f.Recv)))
		c.Local(storeOp(f.Recv), g.slot(d.RecvSym))
	case f.Owner != nil:
		self = c.AllocLocal("L" + f.Owner.Name + ";")
		c.Local(cf.ASTORE, self)
	}
	for i := len(f.Contexts) - 1; i >= 0; i-- {
		p := f.Contexts[i]
		g.setSlot(p, c.AllocLocal(desc(p.Type)))
		c.Local(storeOp(p.Type), g.slot(p))
	}
	// The body of a member of an inner, local or anonymous class reads
	// what the class captures, its outer instance among them, from self.
	if o := f.Owner; self >= 0 && (o.Anonymous || o.Inner || o.Local) {
		if cd, ok := o.Decl.(*syntax.ClassDecl); ok {
			for _, v := range cd.Captures {
				saved[v] = g.slot(v)
			}
			if _, od := outerLocal(cd); od != nil {
				for _, v := range od.Captures {
					saved[v] = g.slot(v)
				}
			}
			g.loadCaptures(cd, self)
		}
	}
	// A function value converted to a suspend one is inlined as the
	// conversion's lambda, as kotlinc's: see convertedArg.
	convs := map[int]*syntax.Lambda{}
	for i := range f.Params {
		if l, _ := g.forwardedArg(x, i); l != nil {
			continue
		}
		if conv, l, fv := convertedArg(x, d, i); l != nil {
			convs[i] = l
			if g.shared.inlinedConvs == nil {
				g.shared.inlinedConvs = map[*syntax.Call]*types.VarSym{}
			}
			g.shared.inlinedConvs[conv] = fv
			defer delete(g.shared.inlinedConvs, conv)
		}
	}
	// The arguments, in the parameters' order, go to the parameters'
	// locals; an inlined lambda's placeholder is dropped. A composable's
	// copy calls composables with the caller's Composer.
	mask := g.valueArgs(x, f)
	lambdas := map[*types.VarSym]*syntax.Lambda{}
	owners := map[*syntax.Lambda]*srcInline{}
	for i := len(f.Params) - 1; i >= 0; i-- {
		p := f.Params[i]
		if l := inlinedArg(x, i); l != nil {
			c.Op(cf.POP)
			lambdas[p] = l
			continue
		}
		if l, owner := g.forwardedArg(x, i); l != nil {
			c.Op(cf.POP)
			lambdas[p], owners[l] = l, owner
			continue
		}
		if l := convs[i]; l != nil {
			c.Op(cf.POP)
			lambdas[p] = l
			continue
		}
		g.setSlot(p, c.AllocLocal(desc(p.Type)))
		switch {
		case size(p.Type) > 0:
			c.Local(storeOp(p.Type), g.slot(p))
		case p.Type.K == types.Unit && !p.Type.N:
			// A Unit argument is a kotlin.Unit, or null if omitted, which
			// the copy doesn't read.
			c.Op(cf.POP)
		}
	}
	// A composable's copy is a flag scope of its own.
	scope := g.scope()
	if f.Composable && scope != nil {
		defer g.inScope(g.copyScope(x, d, mask))()
	}
	// Omitted arguments are the parameters' default values, which can
	// read the parameters before them, and a member's this.
	defaults := &srcInline{d: d, end: join(f.Ret), depth: c.Depth(), tries: len(g.tries), lambdas: lambdas, owners: owners, outer: g.srcInl,
		reified: g.reifiedArgs(x, f), noLines: g.noLines, self: self, defaults: true, inl: len(g.inl), scope: scope}
	g.srcInl = append(g.srcInl, defaults)
	for i, p := range f.Params {
		if mask.has(len(f.Contexts)+i) && d.Params[i].Default != nil {
			g.exprAs(d.Params[i].Default, p.Type)
			if size(p.Type) > 0 {
				c.Local(storeOp(p.Type), g.slot(p))
			}
		}
	}
	g.srcInl = defaults.outer
	// The values on the stack go to locals meanwhile, and are put back
	// below the result (see spillStack). A try in the body would move
	// them anyway, and a return from it would then leave them behind.
	spilled, ok := g.spillStack()
	if !ok && hasTry(d) {
		panic("an inline function with a try can't be an argument of a constructor call")
	}
	ctx := &srcInline{d: d, end: join(f.Ret), depth: c.Depth(), tries: len(g.tries), lambdas: lambdas, owners: owners, outer: g.srcInl,
		reified: g.reifiedArgs(x, f), noLines: g.noLines, self: self, groups: g.groupDepth(), inl: len(g.inl), scope: scope}
	g.srcInl = append(g.srcInl, ctx)
	g.noLines++
	if f.Composable && d.Group {
		// As the function's method runs its body: see composeStart.
		g.openGroup(g.siteKey(d.P, "body", 0))
	}
	if d.Body != nil {
		g.block(d.Body, types.UnitType, false)
	} else {
		g.exprAs(d.ExprBody, f.Ret)
	}
	g.closeGroups(ctx.groups)
	g.noLines--
	g.srcInl = ctx.outer
	c.Place(ctx.end)
	g.line(x.Pos())
	g.unspill(spilled, retDesc(f.Ret))
}

// copyScope returns the flag scope of the copy of composable inline
// function d for call x, whose omitted arguments' bits are in mask: its
// parameters' change flags, which it computes into new locals.
func (g *gen) copyScope(x *syntax.Call, d *syntax.FunDecl, mask argMask) *flagScope {
	f, c := d.Sym, g.code
	g.changedArgs(g.composeCallArgs(x, f, mask), f.Changed, false)
	flags := make([]int, f.Changed)
	for i := len(flags) - 1; i >= 0; i-- {
		flags[i] = c.AllocLocal("I")
		c.Local(cf.ISTORE, flags[i])
	}
	t := funTracked(d, f.Owner != nil)
	for i := range t.params {
		if def := t.defaults[i]; def >= 0 {
			t.nonStatic[i] = !g.isStatic(d.Params[def].Default)
		}
	}
	return &flagScope{tracked: t, typeParams: f.TypeParams, flags: flags}
}

// inlinesCode reports whether the expressions, outside lambdas that
// aren't inlined, have a try or a loop, Array(n) { ... }'s included, or
// call an inline function: code that starts with the stack in locals (see
// spillStack), which an object that new created and isn't constructed
// yet can't be stored in. A constructor call with such arguments
// evaluates them first, as kotlinc does: Foo(list.sumOf { it }), and
// ktor's AttributeKey(name, typeInfo<T>()), whose typeInfo calls
// typeOfOrNull, a try.
func inlinesCode(xs []syntax.Expr) bool {
	found := false
	inline := func(f *types.FunSym) bool {
		return syntax.SourceInline(f) != nil || f != nil && f.Inline != nil && inlineBody(f, nil) != nil
	}
	visit := func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			return n.Inline
		case *syntax.LocalFun:
			return false
		case *syntax.Try, *syntax.While, *syntax.For:
			found = true
		case *syntax.Call:
			found = found || n.Builtin == types.ArrayInit || inline(n.Sym)
		case *syntax.Member:
			found = found || inline(n.Getter) || inline(n.Setter)
		}
		return !found
	}
	for _, x := range xs {
		if x != nil {
			syntax.Inspect(x, visit)
		}
	}
	return found
}

// hasTry reports whether d's body has a try.
func hasTry(d *syntax.FunDecl) bool {
	found := false
	visit := func(n any) bool {
		if _, ok := n.(*syntax.Try); ok {
			found = true
		}
		return !found
	}
	syntax.Inspect(d.Body, visit)
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
	return found
}

// reifiedArgs returns the substitution of a call's type arguments for the
// reified type parameters of f, in the copy being generated: a type
// argument that is a reified type parameter of the copy the call is in
// is its own argument.
func (g *gen) reifiedArgs(x *syntax.Call, f *types.FunSym) types.Subst {
	if x.Reified == nil {
		return nil
	}
	s := types.Subst{}
	for i, tp := range f.TypeParams {
		if tp.Reified && i < len(x.Reified) {
			s[tp] = types.Arg{T: g.reify(x.Reified[i])}
		}
	}
	return s
}

// reify substitutes the reified type parameters of the copy being
// generated in t.
func (g *gen) reify(t types.Type) types.Type {
	if s := g.reifiedSubst(); s != nil {
		return s.Apply(t)
	}
	return t
}

// reifiedSubst returns the type arguments of the reified type parameters
// of the copy being generated, also in the methods of its lambdas, or nil.
func (g *gen) reifiedSubst() types.Subst {
	if n := len(g.srcInl); n > 0 && g.srcInl[n-1].reified != nil {
		return g.srcInl[n-1].reified
	}
	return g.lambdaReified
}

// inlinedArg returns the lambda that call x passes for parameter i if it
// is inlined.
func inlinedArg(x *syntax.Call, i int) *syntax.Lambda {
	for k, a := range x.Args {
		j := k
		if x.ArgMap != nil {
			j = x.ArgMap[k]
		}
		if l, ok := a.(*syntax.Lambda); ok && j == i && l.Inline {
			return l
		}
	}
	return nil
}

// forwarded returns the inlined lambda that x, an argument in a copy of
// an inline function, is if it is one of the copy's lambda parameters,
// and the copy whose caller's code the lambda is.
func (g *gen) forwarded(x syntax.Expr) (*syntax.Lambda, *srcInline) {
	id, ok := unconverted(x).(*syntax.Ident)
	if !ok || id.Sym == nil || len(g.srcInl) == 0 {
		return nil, nil
	}
	ctx := g.srcInl[len(g.srcInl)-1]
	l := ctx.lambdas[id.Sym]
	if l == nil {
		return nil, nil
	}
	if owner := ctx.owners[l]; owner != nil {
		return l, owner // forwarded to this copy in turn
	}
	return l, ctx
}

// unconverted returns the function value that the suspend conversion x
// converts, f.let { $f -> { $f() } } (see check's suspendConverted), or
// x: an inlined lambda converted to a suspend one for an inline function
// is forwarded to it as it is, as kotlinc does.
func unconverted(x syntax.Expr) syntax.Expr {
	call, ok := x.(*syntax.Call)
	if !ok || len(call.Args) != 1 {
		return x
	}
	m, ok := call.Fun.(*syntax.Member)
	if !ok || m.Name != "let" {
		return x
	}
	ch, ok := m.X.(*syntax.Checked)
	l, ok2 := call.Args[0].(*syntax.Lambda)
	if !ok || !ok2 || len(l.Params) != 1 || l.Params[0].Name != "$f" {
		return x
	}
	return ch.X
}

// conversion returns, for suspend conversion x of a function value that
// isn't nullable, f.let { $f -> { $f() } } (see check's suspendConverted),
// the converted value, the lambda the conversion makes and its $f.
func conversion(x syntax.Expr) (*syntax.Checked, *syntax.Lambda, *types.VarSym) {
	if unconverted(x) == x {
		return nil, nil, nil
	}
	call := x.(*syntax.Call)
	m := call.Fun.(*syntax.Member)
	outer := call.Args[0].(*syntax.Lambda)
	if m.Safe || len(outer.Body.Stmts) != 1 {
		return nil, nil, nil
	}
	s, ok := outer.Body.Stmts[0].(*syntax.ExprStmt)
	if !ok {
		return nil, nil, nil
	}
	l, ok := s.X.(*syntax.Lambda)
	ps := outer.ParamSyms()
	if !ok || len(ps) != 1 || ps[0] == nil {
		return nil, nil, nil
	}
	return m.X.(*syntax.Checked), l, ps[0]
}

// convertedArg returns, if call x of the program's inline function d
// passes for its parameter i a function value converted to a suspend
// one, the conversion, its lambda and its $f: unless the value is one of
// the inlined lambdas being forwarded, the lambda is inlined, as kotlinc
// inlines it, and its invoke of the value is no suspension point. The
// argument is the value, in $f's local.
func convertedArg(x *syntax.Call, d *syntax.FunDecl, i int) (*syntax.Call, *syntax.Lambda, *types.VarSym) {
	if i >= len(d.Inlinable) || !d.Inlinable[i] || d.Params[i].Crossinline {
		return nil, nil, nil
	}
	for k, a := range x.Args {
		j := k
		if x.ArgMap != nil {
			j = x.ArgMap[k]
		}
		if j != i {
			continue
		}
		if _, l, fv := conversion(a); l != nil {
			return a.(*syntax.Call), l, fv
		}
	}
	return nil, nil, nil
}

// forwardedArg returns the lambda that call x passes on for parameter i,
// if it is a forwarded one, and the copy whose caller's code it is.
func (g *gen) forwardedArg(x *syntax.Call, i int) (*syntax.Lambda, *srcInline) {
	for k, a := range x.Args {
		j := k
		if x.ArgMap != nil {
			j = x.ArgMap[k]
		}
		if j == i {
			return g.forwarded(a)
		}
	}
	return nil, nil
}

// srcReturn generates a return from the body of an inline function being
// copied: the value is computed, the finally blocks of the trys it leaves
// run, and it jumps to the copy's end, leaving the value.
func (g *gen) srcReturn(x *syntax.Jump, ctx *srcInline) {
	c := g.code
	ret := ctx.d.Sym.Ret
	switch {
	case ret.K == types.Unit:
		if x.X != nil {
			g.exprAs(x.X, types.UnitType)
		}
	default:
		g.exprAs(x.X, ret)
	}
	if !c.Reachable() {
		return
	}
	tmp := -1
	if size(ret) > 0 {
		tmp = c.AllocLocal(desc(ret))
		c.Local(storeOp(ret), tmp)
	}
	g.leaveTries(ctx.tries)
	c.PopTo(ctx.depth)
	if tmp >= 0 {
		c.Local(loadOp(ret), tmp)
	}
	g.endGroups(ctx.groups)
	c.Jump(cf.GOTO, ctx.end)
}

// inlinedInvoke returns the lambda that an invoke in the body being copied
// invokes, if it is an inlined lambda's parameter.
func (g *gen) inlinedInvoke(x *syntax.Call) (*syntax.Lambda, *srcInline) {
	if len(g.srcInl) == 0 {
		return nil, nil
	}
	id, ok := x.Fun.(*syntax.Ident)
	if !ok || id.Sym == nil {
		return nil, nil
	}
	ctx := g.srcInl[len(g.srcInl)-1]
	return ctx.lambdas[id.Sym], ctx
}

// invokeInlined generates an invoke of an inlined lambda: its arguments go
// to the lambda's parameters, and its body is generated there, as the
// caller's code. It leaves the result, of the invoke's type.
func (g *gen) invokeInlined(x *syntax.Call, l *syntax.Lambda, ctx *srcInline) {
	c := g.code
	scope := c.Scope()
	defer c.EndScope(scope)
	// A composable one knows the change flags of its parameters, which
	// the copy computes as it would pass them to the invoke.
	var flags []int
	if l.Composable && g.compose != nil {
		n := l.T.C.Methods["invoke"][0].Changed
		g.changedArgs(g.invokeArgs(x), n, false)
		flags = make([]int, n)
		for i := n - 1; i >= 0; i-- {
			flags[i] = c.AllocLocal("I")
			c.Local(cf.ISTORE, flags[i])
		}
	}
	params := l.ParamSyms()
	for i, a := range x.Args {
		p := params[i]
		g.exprAs(a, p.Type)
		g.setSlot(p, c.AllocLocal(desc(p.Type)))
		if size(p.Type) > 0 {
			c.Local(storeOp(p.Type), g.slot(p))
		}
	}
	if owner := ctx.owners[l]; owner != nil {
		ctx = owner // a forwarded lambda: the code of that copy's caller
	}
	savedNoLines := g.noLines
	g.noLines = ctx.noLines // the lambda is the caller's code
	defer func() { g.noLines = savedNoLines }()
	for _, p := range params[:len(x.Args)] {
		g.named(p)
	}
	saved := g.srcInl
	g.srcInl = ctx.outer
	// The copy's values on the stack, as s in s += f(i), go to locals
	// meanwhile: see spillStack.
	spilled, _ := g.spillStack()
	lc := &inlineLambda{l: l, end: join(l.Ret), depth: c.Depth(), tries: len(g.tries), direct: true}
	g.inl = append(g.inl, lc)
	back := g.inScope(ctx.scope)
	restore := g.composeInlineStart(lc, -1, flags, ctx.scope) // the copy's Composer is the caller's
	if l.Ret.K == types.Unit {
		g.block(l.Body, types.UnitType, false)
	} else {
		g.block(l.Body, l.Ret, true)
	}
	g.composeInlineEnd(lc)
	restore()
	back()
	g.inl = g.inl[:len(g.inl)-1]
	g.srcInl = saved
	c.Place(lc.end)
	g.unspill(spilled, retDesc(l.Ret))
	g.coerce(l.Ret, x.T)
}

// inlineMu guards the names of the classes of the object expressions in
// the program's inline functions, which are in their shared syntax trees:
// files are generated in parallel, and a copy of a function in one file
// renames them (see nameObjects) while its own file generates its method,
// or another file another copy. It is held while a file generates an
// inline function's method or the outermost copy of one, and taken again
// by the copies and methods within those only in name: one goroutine
// generates a file. The variables' locals aren't in the trees but in each
// file's fileState (see slot), so no lock guards them.
var inlineMu sync.Mutex

func (g *gen) lockInline() {
	if g.shared.inlineDepth == 0 {
		inlineMu.Lock()
	}
	g.shared.inlineDepth++
}

func (g *gen) unlockInline() {
	g.shared.inlineDepth--
	if g.shared.inlineDepth == 0 {
		g.restoreNames(0)
		inlineMu.Unlock()
	}
}

// saveSlots records the locals of d's parameters and of the variables
// declared in its body, which a copy reassigns: a lambda passed to a copy
// may contain another copy.
func (g *gen) saveSlots(d *syntax.FunDecl) map[*types.VarSym]int {
	saved := map[*types.VarSym]int{}
	add := func(v *types.VarSym) {
		if v != nil {
			saved[v] = g.slot(v)
		}
	}
	add(d.RecvSym)
	for _, p := range d.Sym.Contexts {
		add(p)
	}
	for _, p := range d.Sym.Params {
		add(p)
	}
	visit := func(n any) bool {
		switch n := n.(type) {
		case *syntax.VarStmt:
			add(n.Sym)
			add(n.DelegateSym)
		case *syntax.For:
			add(n.Sym)
		case *syntax.Try:
			for _, c := range n.Catches {
				add(c.Sym)
			}
		case *syntax.When:
			add(n.SubjSym)
		case *syntax.Lambda:
			for _, p := range n.ParamSyms() {
				add(p)
			}
			if n.Ref != nil {
				add(n.Ref.Bound)
			}
		}
		return true
	}
	syntax.Inspect(d.Body, visit)
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
	return saved
}

// restoreSlots gives the variables back the locals saveSlots recorded.
func (g *gen) restoreSlots(saved map[*types.VarSym]int) {
	for v, slot := range saved {
		g.setSlot(v, slot)
	}
}

// slot returns the local that holds variable v, a local or a parameter,
// in the code being generated.
//
// The locals are kept by file, not on the VarSyms: those of the program's
// inline functions, and the captures of their object expressions, are
// shared by every file that copies them, and files are generated in
// parallel. One goroutine generates a file, so its map needs no lock.
func (g *gen) slot(v *types.VarSym) int {
	return g.shared.slots[v]
}

// setSlot records that local slot holds variable v: see slot.
func (g *gen) setSlot(v *types.VarSym, slot int) {
	g.shared.slots[v] = slot
}

// Kinds of operations on reified type parameters, as kotlinc numbers them
// in its markers.
const (
	reifiedNewArray = iota
	reifiedAs
	reifiedSafeAs
	reifiedIs
	reifiedJavaClass
	reifiedEnum
	reifiedTypeOf
)

// reifiedParam reports whether t is still a reified type parameter where
// code is generated: in the method of an inline function, which kotlinc's
// inliner and kc's copy into callers with the call's type argument.
func (g *gen) reifiedParam(t types.Type) bool {
	return t.IsParam() && t.Param().Reified && g.shared.opts.Stdlib
}

// memberParam reports whether type parameter t is one of the member
// function of the class being generated whose own code is being
// generated, as an object's inline method's: the method's copiers reify
// it, not those of the class. A lambda's class in it is a template of the
// method's copiers, which its markers make them copy.
func (g *gen) memberParam(t types.Type) bool {
	f, ok := t.Param().Owner.(*types.FunSym)
	return ok && g.lam == nil && f.Owner != nil && f.Owner == g.owner
}

// reifiedMarker writes kotlinc's marker of an operation on reified type
// parameter t, which comes just before the operation's instruction:
// Intrinsics.reifiedOperationMarker(kind, "T").
func (g *gen) reifiedMarker(t types.Type, kind int) {
	if g.reifiedUse != nil && !g.memberParam(t) {
		*g.reifiedUse = true
	}
	name := t.Param().Name
	if t.N {
		name += "?"
	}
	g.code.Iconst(int32(kind))
	g.code.Sconst(name)
	g.code.Invoke(cf.INVOKESTATIC, intrinsics, "reifiedOperationMarker", "(ILjava/lang/String;)V")
}
