package check

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Context parameters.
//
// A function with context parameters, context(a: A, _: B) fun f(), takes
// them implicitly: a call passes, for each, the nearest implicit value of
// its type, as an argument before the extension receiver. The implicit
// values are, innermost first, the context parameters and receivers of
// the lambdas and functions the call is in, then the class's this. Its
// FunSym has them in Contexts, apart from Params, as Recv is, and the
// call the chosen values in Ctx, as expressions this that name them. On
// the JVM, the function takes them first, before its receiver.
//
// In the function's body, a named context parameter is a local, and each
// is an implicit value for the calls in it, but not an implicit
// receiver: its members aren't in scope. A lambda whose expected type is
// a context function type, context(A) B.(C) -> R, which is the function
// type (A, B, C) -> R with its first parameter marked as a context (see
// types.Type.X), has its context parameters as implicit values the same
// way, in Lambda.Ctx. A call of a value of a context function type that
// leaves its contexts out, f(c) for f: context(A) (C) -> R, passes them as
// its first arguments.

// declareContexts resolves the types of a function's context parameters
// and declares them.
func (c *Checker) declareContexts(d *syntax.FunDecl, sym *types.FunSym) {
	sym.Contexts = c.contextSyms(d.Contexts)
	if d.Inline {
		c.noinlineContexts(d.Contexts)
	}
}

// noinlineContexts reports the context parameters of an inline function or
// accessor, ps, of function types that aren't noinline: as with kotlinc,
// they aren't inlined.
func (c *Checker) noinlineContexts(ps []*syntax.Param) {
	for _, p := range ps {
		if t := p.Sym.Type; !p.Noinline && !t.N && t.Flex().IsFunc() {
			c.errorf(p.P, "context parameter '%s: %s' must be 'noinline': inlining context parameters is not yet supported", p.Name, t)
		}
	}
}

// reservedContext reports whether t, a context parameter's type, is
// CoroutineContext, which Kotlin reserves for suspend functions' own.
func reservedContext(t types.Type) bool {
	return t.K == types.Class && t.C.Name == "kotlin/coroutines/CoroutineContext"
}

// reservedContextError reports a context parameter at p of type t, or a
// lambda or function type with one, that reservedContext rejects.
func (c *Checker) reservedContextError(p syntax.Pos) {
	c.errorf(p, "context parameter of type 'kotlin.coroutines.CoroutineContext' is reserved for future use")
}

// contextSyms resolves the types of context parameters ps and returns
// their symbols.
func (c *Checker) contextSyms(ps []*syntax.Param) []*types.VarSym {
	var out []*types.VarSym
	seen := map[string]bool{}
	for _, p := range ps {
		if p.Name != "_" {
			if seen[p.Name] {
				c.errorf(p.P, "conflicting declarations: %s", p.Name)
			}
			seen[p.Name] = true
		}
		name := p.Name
		if name == "_" {
			// As kotlinc names its local: $context-Logger.
			name = "$context-" + p.Type.Name[strings.LastIndexByte(p.Type.Name, '.')+1:]
		}
		p.Sym = &types.VarSym{Name: name, Type: c.resolveType(p.Type), Kind: types.Param, Decl: p, NoInfer: noInfer(p.Type)}
		if reservedContext(p.Sym.Type) {
			c.reservedContextError(p.P)
		}
		out = append(out, p.Sym)
	}
	return out
}

// noInfer reports whether t or a type in it is annotated @NoInfer.
func noInfer(t *syntax.TypeRef) bool {
	if t == nil {
		return false
	}
	if t.NoInfer {
		return true
	}
	for _, a := range t.Args {
		if noInfer(a.Type) {
			return true
		}
	}
	return false
}

// propContexts declares the context parameters of property d, of symbol
// sym, which its accessors take, and declare in their bodies. A property
// with them has no field: it needs accessors unless abstract.
func (c *Checker) propContexts(d *syntax.PropDecl, sym *types.VarSym) {
	if len(d.Contexts) == 0 {
		return
	}
	switch {
	case d.Recv != nil, d.Init != nil:
		// Reported as an extension property's, or as an initializer
		// without a field.
	case !sym.Abstract && (d.Getter == nil || d.Mutable && d.Setter == nil):
		c.errorf(d.P, "a property with context parameters must have accessors or be abstract")
	}
	sym.Contexts = c.contextSyms(d.Contexts)
	if d.Inline || d.InlineGetter || d.InlineSetter {
		c.noinlineContexts(d.Contexts)
	}
	for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
		if a != nil && a.Sym != nil {
			a.Contexts = d.Contexts
			a.Sym.Contexts = sym.Contexts
		}
	}
}

// funContexts returns the context parameters in the body of a function,
// as implicit values.
func funContexts(d *syntax.FunDecl) []implicitRecv {
	var out []implicitRecv
	for _, p := range d.Contexts {
		out = append(out, implicitRecv{sym: p.Sym, owner: d})
	}
	return out
}

// declareNamedContexts declares a function's named context parameters in
// scope s.
func declareNamedContexts(s *scope, d *syntax.FunDecl) {
	for _, p := range d.Contexts {
		if p.Name != "_" {
			s.declare(p.Sym)
		}
	}
}

// pushContexts makes the context parameters of a lambda or a local
// function, owner, implicit values in its body, nearer than the receivers
// around it.
func (c *Checker) pushContexts(syms []*types.VarSym, owner any) {
	if len(syms) == 0 {
		return
	}
	c.ctxs = c.ctxs[:len(c.ctxs):len(c.ctxs)]
	for _, s := range syms {
		c.ctxs = append(c.ctxs, implicitRecv{sym: s, depth: len(c.lambdas), at: len(c.recvs), owner: owner})
	}
}

// contextValue returns the nearest implicit value that fits, as an
// expression at p, and its type: a context parameter or a receiver of the
// lambdas and functions around, or the class's this. It reports whether
// another of the same level fits too, the same function's or lambda's
// context parameters and receiver, which kotlinc reports as ambiguous.
func (c *Checker) contextValue(p syntax.Pos, fits func(types.Type) bool) (x syntax.Expr, t types.Type, ok, ambiguous bool) {
	i, j := len(c.recvs)-1, len(c.ctxs)-1
	var found *implicitRecv
	var level [2]int // a receiver's lambda depth and index, as its context parameters' depth and at
	for i >= 0 || j >= 0 {
		var r *implicitRecv
		var lv [2]int
		if j >= 0 && (i < 0 || c.ctxs[j].at > i) {
			r = &c.ctxs[j]
			lv = [2]int{r.depth, r.at}
			j--
		} else {
			r = &c.recvs[i]
			lv = [2]int{r.depth, i}
			i--
		}
		switch {
		case found == nil:
			if rt := c.scope.narrowed(r.sym); fits(rt) {
				found, level, t = r, lv, rt
			}
		case lv != level:
			return recvThis(found, p), t, true, false
		case fits(c.scope.narrowed(r.sym)):
			return recvThis(found, p), t, true, true
		}
	}
	if found != nil {
		return recvThis(found, p), t, true, false
	}
	if c.cls != nil {
		t := c.cls.This()
		if n, ok := c.narrowedThis(); ok {
			t = n
		}
		if fits(t) {
			this := &syntax.This{}
			this.P = p
			return this, t, true, false
		}
	}
	return nil, types.InvalidType, false, false
}

// contextArg is a context argument a call of a callee passes: an implicit
// value, and its type. If ambiguous, another of the same level fits too
// (see contextValue). If noInfer, its parameter's type is @NoInfer: it
// doesn't constrain the call's type arguments, which choose it again.
type contextArg struct {
	x                  syntax.Expr
	t                  types.Type
	ambiguous, noInfer bool
}

// contextArgs chooses the context arguments of call x of callee cl, the
// nearest implicit value for each of its context parameters, of a type
// that fits the parameter's with the call's type arguments, or some. It
// reports false if one has none.
func (c *Checker) contextArgs(cl *callee, x *syntax.Call) ([]contextArg, bool) {
	ctx := cl.f.Contexts
	if len(ctx) == 0 {
		return nil, true
	}
	sys := newSystem(c)
	var sub types.Subst
	explicit := false
	if n := len(x.TypeArgs); n > 0 && n == len(cl.tparams) {
		explicit = true
		// Explicit ones, contextOf<A>(), whose errors resolve reports.
		e := len(c.errs)
		sub = maps.Clone(cl.recv)
		if sub == nil {
			sub = types.Subst{}
		}
		for j, tp := range cl.tparams {
			if a := x.TypeArgs[j]; a.Type != nil {
				sub[tp] = types.Arg{T: c.resolveType(a.Type)}
			}
		}
		c.errs = c.errs[:e]
	} else {
		sub = sys.fresh(cl.tparams, cl.recv, x.P)
	}
	out := make([]contextArg, len(ctx))
	for k, cp := range ctx {
		want := sub.ApplyIn(cp.Type)
		v, t, ok, amb := c.contextValue(x.P, func(t types.Type) bool {
			trial := sys.clone()
			trial.sub(t, want)
			return trial.ok
		})
		if !ok {
			return nil, false
		}
		out[k] = contextArg{v, t, amb, cp.NoInfer && !explicit && cp.Type.HasParams()}
	}
	return out, true
}

// ambiguousContext reports a context argument at p for a parameter of type
// t that two implicit values of the same level fit.
func (c *Checker) ambiguousContext(p syntax.Pos, t types.Type) {
	c.errorf(p, "multiple potential context arguments for '%s' in scope", t)
}

// noContextArg reports a call at p of f, none of whose candidates found
// the context arguments.
func (c *Checker) noContextArg(p syntax.Pos, f *types.FunSym) {
	for _, cp := range f.Contexts {
		c.errorf(p, "no context argument for '%s: %s' found", cp.Name, cp.Type)
		return
	}
}

// setContexts records the context arguments of call x, of callee cl. If
// sys, the call's inference, those of generic parameters that contextArgs
// found ambiguous, before the type arguments were inferred, or whose
// types are @NoInfer, are chosen again with them, sub's solution.
func (c *Checker) setContexts(x *syntax.Call, cl *callee, sys *system, sub types.Subst) {
	x.Ctx = nil
	for k, a := range cl.ctx {
		pt := cl.f.Contexts[k].Type
		switch {
		case !a.ambiguous && !a.noInfer:
		case sys != nil && pt.HasParams():
			// The system may be solved outside the lambda the call is in,
			// as one of an outer call's: choose among the values here.
			recvs, ctxs, sc := c.recvs, c.ctxs, c.scope
			sys.fixups = append(sys.fixups, func(s *system) {
				want := s.final(sub.ApplyIn(pt))
				savedRecvs, savedCtxs, savedScope := c.recvs, c.ctxs, c.scope
				c.recvs, c.ctxs, c.scope = recvs, ctxs, sc
				defer func() { c.recvs, c.ctxs, c.scope = savedRecvs, savedCtxs, savedScope }()
				v, _, ok, amb := c.contextValue(x.P, func(t types.Type) bool { return c.subtype(t, want) })
				switch {
				case !ok && !a.ambiguous:
					v = a.x
					c.errorf(x.P, "no context argument for '%s: %s' found", cl.f.Contexts[k].Name, want)
				case !ok:
					v, amb = a.x, true
				}
				if amb {
					c.ambiguousContext(x.P, pt)
				}
				c.value(v, noWant)
				x.Ctx[k] = v
				c.dslContext(x, k, v, cl.f)
			})
		case a.ambiguous:
			c.ambiguousContext(x.P, pt)
		}
		c.value(a.x, noWant)
		x.Ctx = append(x.Ctx, a.x)
		c.dslContext(x, k, a.x, cl.f)
	}
}

// refContexts reports the declaration with context parameters that the
// body of callable reference x, at p, uses, which Kotlin rejects, and
// true if there is one.
func (c *Checker) refContexts(x *syntax.Lambda, p syntax.Pos) bool {
	var name string
	switch b := x.Body.Stmts[0].(*syntax.ExprStmt).X.(type) {
	case *syntax.Call:
		if b.Sym != nil && !b.Invoke && len(b.Sym.Contexts) > 0 {
			name = b.Sym.Name
		}
	case *syntax.Ident:
		if b.Call != nil {
			name = b.Name
		}
	case *syntax.Member:
		if b.Call != nil {
			name = b.Name
		}
	}
	if name == "" {
		return false
	}
	c.errorf(p, "callable reference to '%s' is unsupported because it has context parameters", name)
	return true
}

// implicitContexts passes the contexts of a call of a value of context
// function type ft that leaves them out: block() for block: context(A)
// () -> R, in a lambda whose receiver is an A, is block(this).
func (c *Checker) implicitContexts(x *syntax.Call, ft types.Type) {
	k := ft.Contexts()
	if k == 0 || len(x.Args) != ft.A.Len()-1-k {
		return
	}
	ctx := make([]syntax.Expr, k)
	for i := range k {
		want := ft.A.Get(i).T
		v, _, ok, amb := c.contextValue(x.P, func(t types.Type) bool { return c.subtype(t, want) })
		if !ok {
			c.errorf(x.P, "no context argument for '%s' found", want)
			return
		}
		if amb {
			c.ambiguousContext(x.P, want)
		}
		ctx[i] = v
		if id, ok := x.Fun.(*syntax.Ident); ok {
			c.dslUse(dslKey{x, i}, v.(*syntax.This).Recv, id.Name, id.P, func() bool { return len(x.Args) > i && x.Args[i] == v })
		}
	}
	x.Args = append(ctx, x.Args...)
}

// Properties with context parameters.
//
// A property with context parameters has no field, and its accessors take
// them first, as a function does: a read of it is a call of its getter,
// and an assignment a call of its setter, which pass the context
// arguments. The checker records the call on the Ident or Member read or
// assigned, as its Call, which is evaluated instead. A compound assignment
// is a read and an assignment, x.p = x.p + v, with x in a temporary.

// contextAccessor returns the accessor of the property with context
// parameters that x reads, its getter, or assigns if set, its setter, and
// the member access x is or stands for; nil if x isn't one.
func contextAccessor(x syntax.Expr, set bool) (*types.FunSym, *syntax.Member) {
	var m *syntax.Member
	switch x := x.(type) {
	case *syntax.Ident:
		switch {
		case x.Member != nil:
			m = x.Member
		case x.Sym != nil && len(x.Sym.Contexts) > 0:
			return accessor(x.Sym, set), nil
		}
	case *syntax.Member:
		m = x
	}
	switch {
	case m == nil:
	case m.Prop != nil && len(m.Prop.Contexts) > 0:
		return accessor(m.Prop, set), m
	case m.Getter != nil && len(m.Getter.Contexts) > 0:
		// An extension property's, or a class path's.
		switch {
		case set && m.Setter != nil:
			return m.Setter, m
		case set:
			return m.Getter.ExtSetter, m
		}
		return m.Getter, m
	}
	return nil, nil
}

// accessor returns property p's getter, or its setter if set: an abstract
// property's, which it doesn't declare, made for the call.
func accessor(p *types.VarSym, set bool) *types.FunSym {
	ensureVar(p)
	switch {
	case set && p.Setter != nil:
		return p.Setter
	case set && p.Mutable:
		return &types.FunSym{Name: types.SetterName(p.Name), JVMName: p.JVMSetter, Owner: p.Owner, Facade: p.Facade, Abstract: p.Abstract, Module: p.Module,
			Contexts: p.Contexts, Params: []*types.VarSym{{Name: "value", Type: p.Type, Kind: types.Param, Resolved: true}}, Ret: types.UnitType, Resolved: true}
	case set:
		return nil
	case p.Getter != nil:
		return p.Getter
	}
	return &types.FunSym{Name: types.GetterName(p.Name), JVMName: p.JVMGetter, Owner: p.Owner, Facade: p.Facade, Abstract: p.Abstract, Module: p.Module,
		Contexts: p.Contexts, Ret: p.Type, Resolved: true}
}

// contextAccess returns the call of accessor f that x, a read or an
// assignment of a property with context parameters, is, unchecked, with
// its callee; m is the member access x is or stands for, or nil.
func (c *Checker) contextAccess(x syntax.Expr, f *types.FunSym, m *syntax.Member) (*syntax.Call, []callee) {
	call := &syntax.Call{}
	call.P = x.Pos()
	cl := callee{f: f, tparams: f.TypeParams}
	if c.accessed == nil {
		c.accessed = map[*syntax.Call]string{}
	}
	switch x := x.(type) {
	case *syntax.Ident:
		c.accessed[call] = x.Name
	case *syntax.Member:
		c.accessed[call] = x.Name
	}
	switch {
	case m != nil:
		rt := m.X.Type()
		if m.Safe {
			rt = rt.NonNull()
		}
		fn := &syntax.Member{X: m.X, Name: f.Name, Safe: m.Safe, Dispatch: m.Dispatch, DispatchThis: m.DispatchThis}
		fn.P, fn.T = m.P, rt
		call.Fun = fn
		switch {
		case f.Ext && f.Owner != nil:
			// A member extension's dispatch receiver: an object, an
			// implicit receiver or this.
			cl.ext, cl.dispatch = rt, m.Dispatch
			switch {
			case m.DispatchThis != nil:
				cl.recv = c.memberSubst(m.DispatchThis.Type(), f.Owner)
			case m.Dispatch != nil:
				cl.recv = c.memberSubst(types.ClassType(m.Dispatch), f.Owner)
			case c.cls != nil:
				cl.recv = c.memberSubst(c.cls.This(), f.Owner)
			}
		case f.Ext:
			cl.ext = rt
		default:
			cl.recv = c.memberSubst(rt, f.Owner)
		}
	case f.Owner != nil:
		// A member of this.
		this := &syntax.This{Class: true}
		this.P = x.Pos()
		rt := c.value(this, noWant)
		fn := &syntax.Member{X: this, Name: f.Name}
		fn.P, fn.T = x.Pos(), rt
		call.Fun = fn
		cl.recv = c.memberSubst(rt, f.Owner)
	default:
		id := &syntax.Ident{Name: f.Name}
		id.P = x.Pos()
		call.Fun = id
	}
	return call, []callee{cl}
}

// altProp reports whether property d can be declared beside prev, the
// property of its name and owner declared before, and those of the name
// declared beside it (see VarSym.Alt): if the sets of their context
// parameters' types differ, as written, or, if one is hidden, their
// order, and so their accessors' JVM descriptors.
func altProp(prev *types.VarSym, d *syntax.PropDecl) bool {
	ctx := contextTypes(d)
	for q := range altProps(prev) {
		var qctx []string
		hidden := d.Hidden()
		if pd, ok := q.Decl.(*syntax.PropDecl); ok {
			// Not a constructor's, which has none.
			qctx, hidden = contextTypes(pd), hidden || pd.Hidden()
		}
		if slices.Equal(qctx, ctx) || !hidden && slices.Equal(sorted(qctx), sorted(ctx)) {
			return false
		}
	}
	return true
}

// contextTypes returns the context parameters' types of d, as written.
func contextTypes(d *syntax.PropDecl) []string {
	var out []string
	for _, p := range d.Contexts {
		var b strings.Builder
		typeKey(&b, p.Type)
		out = append(out, b.String())
	}
	return out
}

// sorted returns a sorted copy of s.
func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

// typeKey writes type t as written, for comparisons.
func typeKey(b *strings.Builder, t *syntax.TypeRef) {
	if t.Suspend {
		b.WriteString("suspend ")
	}
	if t.Contexts != nil {
		b.WriteString("context(")
		for _, c := range t.Contexts {
			typeKey(b, c)
			b.WriteByte(',')
		}
		b.WriteString(") ")
	}
	if t.Recv != nil {
		typeKey(b, t.Recv)
		b.WriteByte('.')
	}
	if t.Ret != nil {
		b.WriteByte('(')
		for _, p := range t.Params {
			typeKey(b, p)
			b.WriteByte(',')
		}
		b.WriteString(")->")
		typeKey(b, t.Ret)
	}
	b.WriteString(t.Name)
	if t.Args != nil {
		b.WriteByte('<')
		for _, a := range t.Args {
			b.WriteString(strconv.Itoa(int(a.Variance)))
			if a.Type != nil {
				typeKey(b, a.Type)
			}
			b.WriteByte(',')
		}
		b.WriteByte('>')
	}
	if t.Nullable {
		b.WriteByte('?')
	}
}

// addAlt adds p to the properties of its name that differ from q's in
// their context parameters (see VarSym.Alt).
func addAlt(q, p *types.VarSym) {
	if q.Alt == nil {
		q.Alt = q
	}
	p.Alt, q.Alt = q.Alt, p
}

// altProps yields p and the properties of its name beside it.
func altProps(p *types.VarSym) func(func(*types.VarSym) bool) {
	return func(yield func(*types.VarSym) bool) {
		for q := p; yield(q) && q.Alt != nil && q.Alt != p; {
			q = q.Alt
		}
	}
}

// kpropAlts yields kp and the class path's properties of its name beside
// it (see KProp.Alt).
func kpropAlts(kp *types.KProp) func(func(*types.KProp) bool) {
	return func(yield func(*types.KProp) bool) {
		for q := kp; yield(q) && q.Alt != nil && q.Alt != kp; {
			q = q.Alt
		}
	}
}

// addKAlt is addAlt for the class path's properties.
func addKAlt(q, p *types.KProp) {
	if q.Alt == nil {
		q.Alt = q
	}
	p.Alt, q.Alt = q.Alt, p
}

// sameContexts reports whether two lists of context parameters have the
// same types.
func sameContexts(a, b []*types.VarSym) bool {
	return slices.EqualFunc(a, b, func(p, q *types.VarSym) bool { return p.Type == q.Type })
}

// propAlt returns the property of p's name that a use here refers to: the
// first of p and those beside it (see VarSym.Alt) that applies, or p. One
// applies if it is visible here and the context arguments it needs are
// found. If more than one does, kotlinc reports an ambiguity, as
// contextRead and contextWrite do.
func (c *Checker) propAlt(p *types.VarSym) *types.VarSym {
	if p == nil || p.Alt == nil {
		return p
	}
	for q := range altProps(p) {
		if c.propApplies(q) {
			return q
		}
	}
	return p
}

// kpropAlt is propAlt for a class path's property.
func (c *Checker) kpropAlt(kp *types.KProp) *types.KProp {
	if kp == nil || kp.Alt == nil {
		return kp
	}
	for q := range kpropAlts(kp) {
		if c.contextsFound(q.Getter) {
			return q
		}
	}
	return kp
}

// contextKProp reports whether a use here of the class path's property
// name of cls refers to one with context parameters (see kpropAlt).
func (c *Checker) contextKProp(cls *types.ClassSym, name string) bool {
	kp := c.kpropAlt(ensured(cls).KProps[name])
	return kp != nil && len(kp.Getter.Contexts) > 0
}

// propApplies reports whether a use here can refer to property p: if it
// is visible, which kc otherwise doesn't check, and its context arguments
// are found.
func (c *Checker) propApplies(p *types.VarSym) bool {
	if d, ok := p.Decl.(*syntax.PropDecl); ok && d.Visibility == syntax.Private && p.Owner != nil && !c.inside(p.Owner) {
		return false
	}
	return c.contextsFound(accessor(p, false))
}

// contextsFound reports whether the context arguments of a call of f here
// are found.
func (c *Checker) contextsFound(f *types.FunSym) bool {
	if len(f.Contexts) == 0 {
		return true
	}
	n := len(c.errs)
	_, ok := c.contextArgs(&callee{f: f, tparams: f.TypeParams}, &syntax.Call{})
	c.errs = c.errs[:n]
	return ok
}

// altAmbiguity reports use x of a property, and true, if another of its
// name applies too: see propAlt.
func (c *Checker) altAmbiguity(x syntax.Expr) bool {
	var prop *types.VarSym
	var m *syntax.Member
	switch x := x.(type) {
	case *syntax.Ident:
		prop, m = x.Sym, x.Member
	case *syntax.Member:
		m = x
	}
	ambiguous := false
	if m != nil {
		prop = m.Prop
		if g := m.Getter; prop == nil && g != nil && g.Owner != nil {
			if kp := g.Owner.KProps[m.Name]; kp != nil && kp.Alt != nil {
				for q := range kpropAlts(kp) {
					ambiguous = ambiguous || q.Getter != g && c.contextsFound(q.Getter)
				}
			}
		}
	}
	if prop != nil && prop.Alt != nil {
		for q := range altProps(prop) {
			ambiguous = ambiguous || q != prop && c.propApplies(q)
		}
	}
	if ambiguous {
		var name string
		if m != nil {
			name = m.Name
		}
		if prop != nil {
			name = prop.Name
		}
		c.errorf(x.Pos(), "overload resolution ambiguity: %s", name)
	}
	return ambiguous
}

// contextRead checks read x, of type t, of a property with context
// parameters as the call of its getter, and returns its type; that of
// another, t.
func (c *Checker) contextRead(x syntax.Expr, t, want types.Type) types.Type {
	if t.K == types.Invalid {
		return t
	}
	if c.altAmbiguity(x) {
		return types.InvalidType
	}
	f, m := contextAccessor(x, false)
	if f == nil {
		return t
	}
	if f.Owner != nil && m == nil && (c.cls == nil || !c.cls.IsSubclassOf(f.Owner)) {
		c.unsupported(x.Pos(), "context parameters", "reading an outer class's property with context parameters is not supported")
		return t
	}
	call, cands := c.contextAccess(x, f, m)
	t = c.resolve(call, cands, want, nil)
	call.SetType(t)
	setAccess(x, call)
	return t
}

// contextWrite checks assignment s of a property with context parameters,
// as the call of its setter, or a compound one as an assignment of the
// value read and v. It reports false for the assignment of another.
func (c *Checker) contextWrite(s *syntax.Assign) bool {
	if c.altAmbiguity(s.Target) {
		return true
	}
	f, m := contextAccessor(s.Target, true)
	if f == nil {
		return false
	}
	if s.Op != syntax.ASSIGN {
		// x.p op= v: val r = x; r.p = r.p op v.
		target, read := func() syntax.Expr { return s.Target }, func() syntax.Expr { id := *s.Target.(*syntax.Ident); id.Call = nil; return &id }
		if m != nil {
			recv := c.temp(s.P, "receiver", m.X, m.X.Type())
			s.Stmts = append(s.Stmts, recv)
			member := func() syntax.Expr {
				r := &syntax.Member{X: tempRef(recv, m.P), Name: m.Name}
				r.P = m.P
				return r
			}
			target, read = member, member
		}
		value := &syntax.Binary{Op: arithOf(s.Op), X: read(), Y: s.Value}
		value.P = s.P
		set := &syntax.Assign{P: s.P, Op: syntax.ASSIGN, Target: target(), Value: value}
		c.assign(set)
		s.Stmts = append(s.Stmts, set)
		return true
	}
	if f.Owner != nil && m == nil && (c.cls == nil || !c.cls.IsSubclassOf(f.Owner)) {
		c.unsupported(s.P, "context parameters", "assigning an outer class's property with context parameters is not supported")
		return true
	}
	call, cands := c.contextAccess(s.Target, f, m)
	call.Args = []syntax.Expr{s.Value}
	call.SetType(c.resolve(call, cands, noWant, nil))
	setAccess(s.Target, call)
	return true
}

// setAccess records the call of an accessor on x, a read or an assignment
// of a property with context parameters.
func setAccess(x syntax.Expr, call *syntax.Call) {
	switch x := x.(type) {
	case *syntax.Ident:
		x.Call = call
	case *syntax.Member:
		x.Call = call
	}
}

// shadowedUse is a call or a property read, named at p, whose implicit
// receiver a context parameter shadows, if still reports that it resolves
// as it did then.
type shadowedUse struct {
	name  string
	p     syntax.Pos
	still func() bool
}

// receiverShadowed records call x of callee cl if an implicit receiver it
// passes, as its dispatch or extension receiver, is shadowed by a context
// parameter: one declared as near as the receiver or nearer, whose value
// it could have been passed. Context parameters aren't implicit
// receivers, so kotlinc asks for the receiver to be explicit. The error
// waits for the end of checking: it must not make the calls that try
// candidates, and drop those with errors, try another, and x may be
// checked again, in a lambda, and resolve otherwise.
func (c *Checker) receiverShadowed(x *syntax.Call, cl *callee) {
	delete(c.shadowed, x)
	f := cl.f
	if len(c.ctxs) == 0 || f.Static || f.Name == "<init>" {
		return
	}
	var p syntax.Pos
	switch fn := x.Fun.(type) {
	case *syntax.Member:
		if !c.memberShadowed(fn, f.Ext, f.Recv, f.Owner) {
			return
		}
		p = fn.P
	case *syntax.Ident:
		if f.Ext || f.Owner == nil || c.cls == nil || !c.cls.IsSubclassOf(f.Owner) || !c.shadows(-1, c.instanceOf(f.Owner)) {
			return
		}
		p = fn.P
	default:
		return
	}
	c.shadowedAt(x, shadowedUse{f.Name, p, func() bool { return x.Sym == f }})
}

// propShadowed records x, a property read on an implicit receiver, if a
// context parameter shadows the receiver, as receiverShadowed does.
func (c *Checker) propShadowed(x *syntax.Ident) {
	delete(c.shadowed, x)
	m := x.Member
	if len(c.ctxs) == 0 || m == nil {
		return
	}
	switch {
	case m.Prop != nil && m.Prop.Owner != nil:
		if !c.memberShadowed(m, false, types.Type{}, m.Prop.Owner) {
			return
		}
	case m.Getter != nil && !m.Getter.Static:
		if !c.memberShadowed(m, m.Getter.Ext, m.Getter.Recv, m.Getter.Owner) {
			return
		}
	default:
		return
	}
	c.shadowedAt(x, shadowedUse{x.Name, x.P, func() bool { return x.Member == m }})
}

// memberShadowed reports whether a context parameter shadows an implicit
// receiver of m, a member of class owner or, if ext, an extension with
// receiver type recv: its receiver, or a member extension's dispatch
// receiver.
func (c *Checker) memberShadowed(m *syntax.Member, ext bool, recv types.Type, owner *types.ClassSym) bool {
	if th, _ := m.X.(*syntax.This); th != nil && th.Implicit {
		i := c.recvIndex(th.Recv)
		if ext && c.shadows(i, func(t types.Type) bool { return c.fitsRecv(t, recv) }) || !ext && owner != nil && c.shadows(i, c.instanceOf(owner)) {
			return true
		}
	}
	if !ext || owner == nil || m.Dispatch != nil {
		return false
	}
	i := -1
	if m.DispatchThis != nil {
		i = c.recvIndex(m.DispatchThis.Recv)
	} else if c.cls == nil || !c.cls.IsSubclassOf(owner) {
		return false
	}
	return c.shadows(i, c.instanceOf(owner))
}

// instanceOf returns whether a value of a type is an instance of cls.
func (c *Checker) instanceOf(cls *types.ClassSym) func(types.Type) bool {
	return func(t types.Type) bool {
		k := c.classOf(t)
		return k != nil && k.IsSubclassOf(cls)
	}
}

// shadowedAt records u, the use x, for shadowedErrors.
func (c *Checker) shadowedAt(x syntax.Expr, u shadowedUse) {
	if c.shadowed == nil {
		c.shadowed = map[syntax.Expr]shadowedUse{}
	}
	c.shadowed[x] = u
}

// shadowedErrors reports the uses that receiverShadowed and propShadowed
// recorded which still resolve as they did then.
func (c *Checker) shadowedErrors() {
	if c.file.SuppressShadowed {
		return
	}
	for _, u := range c.shadowed {
		if u.still() {
			c.errorf(u.p, "'%s' uses an implicit receiver shadowed by a context parameter: make the receiver explicit", u.name)
		}
	}
}

// recvIndex returns the index in c.recvs of the implicit receiver held by
// v, or -2.
func (c *Checker) recvIndex(v *types.VarSym) int {
	for i := range c.recvs {
		if c.recvs[i].sym == v {
			return i
		}
	}
	return -2
}

// shadows reports whether a context parameter as near as implicit
// receiver i or nearer, or than the class's this if i is -1, has a type
// that fits.
func (c *Checker) shadows(i int, fits func(types.Type) bool) bool {
	if i < -1 {
		return false
	}
	for _, k := range c.ctxs {
		near := !k.outer
		if i >= 0 {
			near = k.at > i || k.at == i && k.owner != nil && k.owner == c.recvs[i].owner
		}
		if near && fits(c.scope.narrowed(k.sym)) {
			return true
		}
	}
	return false
}

// fitsRecv reports whether a value of type t could be the receiver of an
// extension whose receiver type is rt: of a subclass of its class if rt
// has type parameters.
func (c *Checker) fitsRecv(t, rt types.Type) bool {
	if rt.HasParams() {
		cls, rc := c.classOf(t), c.classOf(rt)
		return cls != nil && rc != nil && cls.IsSubclassOf(rc)
	}
	return c.subtype(t, rt)
}
