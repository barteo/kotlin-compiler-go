package check

import (
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Object expressions.
//
// An object expression, object : A(x), I { ... }, is an instance of an
// anonymous class, which the checker declares and checks where it meets
// the expression, through the phases that the file's classes go through
// before. Its members' code is checked in a frame of its own on the
// lambda stack, as a local function's, so that the locals of the
// enclosing code that it uses are recorded as captures. The enclosing
// class's this is a capture too, a local that this@Outer and the outer
// class's members used without a receiver refer to; it and the enclosing
// lambdas' receivers are implicit receivers that come after the object's
// own members. The class's constructor takes the captures, and keeps them
// in fields, as kotlinc does.

// anonCtx is the code around an object expression, which its members'
// code sees.
type anonCtx struct {
	scope      *scope
	lambdas    []*syntax.Lambda // ending with the object's frame
	lrets      []*lambdaResult
	recvs      []implicitRecv
	ctxs       []implicitRecv // the context parameters around: see context.go
	tparams    []*types.TypeParamSym
	localCalls *[]localCall
	staticCls  *types.ClassSym // the class whose static scope the code is in, outside a class
}

// inAnon makes the state of code in class cls, if it is an object
// expression's, see the code around the object.
func (c *Checker) inAnon(cls *types.ClassSym) {
	ctx := c.anon[cls]
	if ctx == nil {
		return
	}
	s := c.scope
	for s.parent != nil {
		s = s.parent
	}
	s.parent = ctx.scope
	n := len(ctx.lambdas)
	for t := c.scope; t != ctx.scope; t = t.parent {
		t.depth = n
	}
	c.lambdas, c.lrets = ctx.lambdas, ctx.lrets
	c.base, c.thisBase = n, n
	own := slices.Clone(c.recvs)
	for i := range own {
		// A member's extension receiver is the member's own parameter,
		// inside the object's frame.
		own[i].depth = max(own[i].depth, n)
	}
	c.recvs = append(ctx.recvs[:len(ctx.recvs):len(ctx.recvs)], own...)
	ownCtx := slices.Clone(c.ctxs)
	for i := range ownCtx {
		ownCtx[i].depth = max(ownCtx[i].depth, n)
		ownCtx[i].at += len(ctx.recvs) // nearer than the receivers around
	}
	c.ctxs = append(ctx.ctxs[:len(ctx.ctxs):len(ctx.ctxs)], ownCtx...)
	c.tparams = append(c.tparams[:len(c.tparams):len(c.tparams)], ctx.tparams...)
	c.localCalls = ctx.localCalls
}

// ownProp reports whether the object expression, inner class or local
// class whose members are being checked has a property, its own or
// inherited, which comes before an outer receiver's.
func (c *Checker) ownProp(x *syntax.Ident) bool {
	if c.cls == nil {
		return false
	}
	this := &syntax.This{Class: true} // not a lambda's receiver
	this.P = x.P
	m := &syntax.Member{X: this, Name: x.Name}
	m.P = x.P
	n := len(c.errs)
	c.expr(m, noWant)
	ok := len(c.errs) == n
	c.errs = c.errs[:n]
	return ok
}

// objectExpr checks an object expression. Its type is its class's.
func (c *Checker) objectExpr(x *syntax.ObjectExpr) types.Type {
	d := x.Decl
	for _, m := range d.Members {
		if sc, ok := m.(*syntax.SecondaryCtor); ok {
			c.errorf(sc.P, "constructors are not allowed for objects")
			return types.InvalidType
		}
	}
	sym := c.anonClass(d, "object expression", "object expressions", func() string { return c.anonName() }, func(sym *types.ClassSym, _ *syntax.Lambda) {
		sym.Anonymous, sym.Final = true, true
	})
	if sym == nil {
		return types.InvalidType
	}
	return sym.This()
}

// localClass checks a class declared in a block, as an object
// expression's class with a name, which is in scope after its
// declaration, and in its own code, where its constructions are recorded
// too: a lambda in it that constructs it captures what it captures.
func (c *Checker) localClass(s *syntax.LocalClass) {
	d := s.Decl
	depth := len(c.lambdas)
	scope := c.scope
	c.anonClass(d, "local class", "local classes", func() string { return c.localName(d.Name) }, func(sym *types.ClassSym, frame *syntax.Lambda) {
		sym.Local = true
		scope.classes = append(scope.classes, sym)
		if c.localClasses == nil {
			c.localClasses = map[*types.ClassSym]localClassFrame{}
		}
		c.localClasses[sym] = localClassFrame{frame, depth}
	})
}

// localClassFrame is a local class's capture frame, and the depth of the
// lambdas its declaration is in.
type localClassFrame struct {
	frame *syntax.Lambda
	depth int
}

// localConstructed records a construction of local class cls, which
// passes what the class captures: the lambdas and local functions
// between the class's declaration and the call capture it too.
func (c *Checker) localConstructed(cls *types.ClassSym) {
	lf, ok := c.localClasses[cls]
	if !ok {
		return
	}
	chain := slices.Clone(c.lambdas[min(lf.depth, len(c.lambdas)):])
	c.addLocalCall(localCall{lf.frame, chain})
}

// anonClass checks the class of an object expression or of a local class,
// d, named by name, in the code it is in, and returns its symbol, which
// setup marks, with its capture frame, before the class is checked; or
// nil if kc doesn't support it. what and whats name the class in errors.
func (c *Checker) anonClass(d *syntax.ClassDecl, what, whats string, name func() string, setup func(*types.ClassSym, *syntax.Lambda)) *types.ClassSym {
	if !c.localMembers(d, what) {
		return nil
	}
	if c.fn != nil && c.fn.Decl != nil && syntax.SourceInline(c.fn) != nil && d.Local {
		c.errorf(d.P, "local classes are not yet supported in inline functions")
		return nil
	}
	if c.cls != nil && c.cls.Anonymous {
		if od, ok := c.cls.Decl.(*syntax.ClassDecl); ok && od.EnclosingFun != nil {
			if fd := syntax.SourceInline(od.EnclosingFun); fd != nil && reified(fd) {
				// Copies of the function copy the classes of its own
				// object expressions only.
				c.unsupported(d.P, what+" in inline function", "%s in object expressions in inline functions with reified type parameters are not supported", whats)
				return nil
			}
		}
	}
	if c.localCalls == nil {
		// Shared with the class's code, and kept after it, to propagate
		// what it captures to the lambdas in it that construct it.
		c.localCalls = new([]localCall)
	}
	saved, savedClasses, savedLookup := c.state, c.classes, c.lookupCls
	frame := &syntax.Lambda{}
	frame.P = d.P
	depth := len(c.lambdas)
	ctx := &anonCtx{scope: c.scope, lambdas: append(c.lambdas[:depth:depth], frame),
		lrets: append(c.lrets[:len(c.lrets):len(c.lrets)], &lambdaResult{}), tparams: c.tparams, localCalls: c.localCalls}
	if c.cls == nil {
		ctx.staticCls = c.staticCls
	}
	for _, r := range c.recvs {
		r.outer = true
		ctx.recvs = append(ctx.recvs, r)
	}
	ctx.ctxs = slices.Clone(c.ctxs)
	var outer *types.VarSym
	if c.cls != nil {
		// The enclosing class's this: inside the receivers outside an
		// inner or anonymous enclosing class, which come first, and
		// outside the lambdas'.
		outer = &types.VarSym{Name: "this$0", Kind: types.Local, Type: c.cls.This()}
		name := c.cls.DisplayName()
		r := implicitRecv{sym: outer, label: name[strings.LastIndexByte(name, '.')+1:], depth: depth, outer: true}
		n := 0
		if cc := c.anon[c.cls]; cc != nil {
			n = min(len(cc.recvs), len(ctx.recvs))
		}
		ctx.recvs = slices.Insert(ctx.recvs, n, r)
		for i := range ctx.ctxs {
			if ctx.ctxs[i].at >= n {
				ctx.ctxs[i].at++
			}
		}
	}

	sym := c.declareClass(d, name(), c.cls)
	setup(sym, frame)
	d.EnclosingCls, d.EnclosingFun = c.file.Class(), c.fn
	if c.cls != nil {
		d.EnclosingCls = c.cls.Name
		if od, ok := c.cls.Decl.(*syntax.ClassDecl); ok && c.fn == nil && c.state.init && !c.cls.Object && od.Ctor != nil {
			d.EnclosingInit = od
		}
	}
	if c.anon == nil {
		c.anon = map[*types.ClassSym]*anonCtx{}
	}
	c.anon[sym] = ctx
	// The phases of the file's classes, for this one and its inner classes.
	c.classes = []*syntax.ClassDecl{d}
	for i := 0; i < len(c.classes); i++ {
		for _, m := range c.classes[i].Members {
			if m, ok := m.(*syntax.ClassDecl); ok {
				c.localInner(m)
				c.classes = append(c.classes, m)
			}
		}
	}
	c.resolveSupertypes()
	c.checkSupers()
	for _, d := range c.classes {
		c.lookupCls = d.Sym
		c.resolveClass(d)
	}
	c.lookupCls = nil
	c.inheritDefaults()
	c.delegations()
	c.defaultImpls()
	for _, d := range c.classes {
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				ensureFun(m.Sym)
			case *syntax.PropDecl:
				ensureVar(m.Sym)
			}
		}
	}
	// Their annotations, which checkBodies resolves for the other classes.
	c.declAnnotations(d, nil)
	for _, d := range c.classes[1:] {
		c.declAnnotations(d, d.Sym.Outer)
	}
	for _, d := range c.classes {
		c.checkClass(d)
	}
	c.state, c.classes, c.lookupCls = saved, savedClasses, savedLookup
	c.propagateCaptures()

	for _, v := range frame.Captures {
		if v == outer {
			d.OuterThis = outer
			c.captureThis() // the enclosing code's this
		}
	}
	d.Captures = frame.Captures
	n := len(c.file.Anon)
	c.onUndo(func() { c.file.Anon = c.file.Anon[:n] })
	c.file.Anon = append(c.file.Anon, d)
	return sym
}

// localMembers reports whether the classes declared in d, a local class
// or an object expression's class (what), and in those, are inner
// classes, the only ones Kotlin allows there, or reports an error.
func (c *Checker) localMembers(d *syntax.ClassDecl, what string) bool {
	ok := true
	for _, m := range d.Members {
		m, isCls := m.(*syntax.ClassDecl)
		if !isCls {
			continue
		}
		switch {
		case m.Companion:
			c.errorf(m.P, "modifier 'companion' is not applicable inside '%s'", what)
		case m.Object:
			c.errorf(m.P, "named object '%s' cannot be local. Try to use an anonymous object instead", m.Name)
		case m.Enum:
			c.errorf(m.P, "modifier 'enum' is not applicable to 'local class'")
		case m.Interface:
			c.errorf(m.P, "'Interface' is prohibited here")
		case !m.Inner:
			c.errorf(m.P, "'Class' is prohibited here")
		default:
			ok = c.localMembers(m, what) && ok
			continue
		}
		ok = false
	}
	return ok
}

// localInner makes the code of inner class d, of a local class or an
// object expression's class, or of an inner class of one, see the code
// around that class as the outer class's code does. What the class
// captures for d's code d reads from it, through this$0, as kotlinc's
// does. The outer class's own receivers are this$0's fields.
func (c *Checker) localInner(d *syntax.ClassDecl) {
	outer := d.Sym.Outer
	oc := c.anon[outer]
	depth := len(oc.lambdas)
	ctx := &anonCtx{scope: oc.scope, lambdas: oc.lambdas, lrets: oc.lrets, tparams: oc.tparams, localCalls: oc.localCalls}
	for _, r := range oc.recvs {
		if r.depth >= depth {
			v := &types.VarSym{Name: r.sym.Name, Kind: types.Local, Type: r.sym.Type, OuterOf: d.OuterThis, OuterCls: outer}
			r = implicitRecv{sym: v, label: r.label, depth: depth, outer: true}
		}
		ctx.recvs = append(ctx.recvs, r)
	}
	own := c.anon[d.Sym].recvs
	this := own[len(own)-1]
	this.depth = depth
	ctx.recvs = append(ctx.recvs, this)
	c.anon[d.Sym] = ctx
}

// localName names a local class as kotlinc does: after the class or the
// facade, and the function, it is in.
func (c *Checker) localName(simple string) string {
	name := c.outerName()
	if c.lambdaClass != "" {
		name = c.lambdaClass
	} else if fn := c.fnPart(); fn != "" {
		name += "$" + fn
	}
	return name + "$" + simple
}

// outerName returns the class that local and anonymous classes are named
// after: the class the code is in, a constructor's default values', or
// the file's facade.
func (c *Checker) outerName() string {
	switch {
	case c.cls != nil:
		return c.cls.Name
	case c.defaultsOf != nil && c.defaultsOf.Name == "<init>" && c.defaultsOf.Owner != nil:
		return c.defaultsOf.Owner.Name
	}
	return c.file.Class()
}

// fnPart returns the name of the function that the code is in, as
// kotlinc names local and anonymous classes after it: a local function's
// after the functions it is in, an accessor's its property's, and a
// default value's its function's.
func (c *Checker) fnPart() string {
	switch {
	case c.fnName != "":
		return c.fnName
	case c.fn != nil && c.fn.Name != "" && !strings.HasPrefix(c.fn.Name, "<"):
		if d, ok := c.fn.Decl.(*syntax.FunDecl); ok && d.Property != "" {
			return d.Property
		}
		return c.fn.Name
	case c.fn == nil && c.defaultsOf != nil && !strings.HasPrefix(c.defaultsOf.Name, "<"):
		return c.defaultsOf.Name
	}
	return ""
}

// anonName names an object expression's class as kotlinc does: after the
// class or the facade, the function, and the variable or property, it is
// in, and numbered from 1.
func (c *Checker) anonName() string {
	name := c.outerName()
	if c.lambdaClass != "" {
		// In a suspend lambda, after its class.
		name = c.lambdaClass
	} else if fn := c.fnPart(); fn != "" {
		name += "$" + fn
	}
	if c.declName != "" {
		name += "$" + c.declName
	}
	if c.anonCount == nil {
		c.anonCount = map[string]int{}
	}
	base := name
	c.anonCount[base]++
	name += "$" + strconv.Itoa(c.anonCount[base])
	if c.file.Named == nil {
		c.file.Named = map[string]bool{}
	}
	c.onUndo(func() { c.anonCount[base]--; delete(c.file.Named, name) })
	c.file.Named[name] = true
	return name
}

// approx returns the type that a declaration whose value is an object
// expression has: its supertype, or Any, as Kotlin approximates it for
// declarations visible outside.
func approx(t types.Type) types.Type {
	n := t.NonNull()
	if n.K != types.Class || !n.C.Anonymous {
		return t
	}
	a := types.AnyType
	if len(n.C.Supers) > 0 {
		a = n.C.Supers[0]
	}
	if types.MayBeNull(t) {
		a = a.Nullable()
	}
	return a
}

// keepsAnon reports whether a declaration with modifiers m, in class cls
// or at the top level if it is nil, has an object expression's own type,
// as in Kotlin, where only the code that sees it uses it: a private one
// that isn't inline, or one of a local class or an object expression.
func keepsAnon(m *syntax.Modifiers, cls *types.ClassSym) bool {
	return m.Visibility == syntax.Private && !m.Inline || cls != nil && (cls.Local || cls.Anonymous)
}

// reified reports whether function d has a reified type parameter.
func reified(d *syntax.FunDecl) bool {
	for _, tp := range d.TypeParams {
		if tp.Reified {
			return true
		}
	}
	return false
}
