package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Checks of inline functions' declarations and bodies that kotlinc makes
// and code generation doesn't need: what Kotlin allows of an inline
// function, whether or not kc can copy it.

// inlineBodyChecks checks the body of inline function d, a member of cls
// or not, after the body is checked.
func (c *Checker) inlineBodyChecks(d *syntax.FunDecl, cls *types.ClassSym) {
	c.inlineUsage(d)
	c.publicInline(d, cls)
	if publicAPIInline(d, cls) {
		c.inlineAccess(d, protectedAPI(d, cls))
	}
}

// protectedAPI reports whether public-API inline function d, a member of
// cls or not, is protected, or a member of a protected class: its copies
// are in subclasses, which can use protected members.
func protectedAPI(d *syntax.FunDecl, cls *types.ClassSym) bool {
	if d.Visibility == syntax.Protected {
		return true
	}
	for k := cls; k != nil; k = k.Outer {
		if cd, ok := k.Decl.(*syntax.ClassDecl); ok && cd.Visibility == syntax.Protected {
			return true
		}
	}
	return false
}

// publicAPIInline reports whether inline function d, a member of cls or
// not, is public API, which other modules inline: not private, internal
// unless @PublishedApi, or in a private, internal, local or anonymous
// class.
func publicAPIInline(d *syntax.FunDecl, cls *types.ClassSym) bool {
	if d.Visibility == syntax.Private || d.Visibility == syntax.Internal && d.Find("", "PublishedApi") == nil {
		return false
	}
	return publicAPIClass(cls)
}

// publicAPIClass reports whether the members of cls, or the top-level
// declarations if nil, can be public API.
func publicAPIClass(cls *types.ClassSym) bool {
	for k := cls; k != nil; k = k.Outer {
		if k.Local || k.Anonymous {
			return false
		}
		cd, ok := k.Decl.(*syntax.ClassDecl)
		if !ok {
			return false
		}
		if cd.Visibility == syntax.Private || cd.Visibility == syntax.Internal || cd.Local || cd.Anonymous {
			return false
		}
	}
	return true
}

// localType reports whether t's class is one declared in a block or an
// object expression's.
func localType(t types.Type) bool {
	if t.K != types.Class || t.C == nil {
		return false
	}
	for k := t.C; k != nil; k = k.Outer {
		if k.Local || k.Anonymous {
			return true
		}
		if cd, ok := k.Decl.(*syntax.ClassDecl); ok && (cd.Local || cd.Anonymous) {
			return true
		}
	}
	return false
}

// inlineAccess reports what public-API inline function d's copies in
// other modules couldn't do, as kotlinc does: call a member of a
// superclass through super (SUPER_CALL_FROM_PUBLIC_INLINE), or a
// protected member or constructor of the program's classes
// (PROTECTED_CALL_FROM_PUBLIC_INLINE_ERROR,
// PROTECTED_CONSTRUCTOR_CALL_FROM_PUBLIC_INLINE). The classes it
// declares are its own: their super, and their members through their
// instances, are not reported, nor are protected members when d is
// protected itself (see protectedAPI).
func (c *Checker) inlineAccess(d *syntax.FunDecl, protectedAPI bool) {
	locals := map[string]bool{} // the names of the classes declared in d
	depth := 0                  // the classes declared in d the code is in
	protected := func(p syntax.Pos, recv syntax.Expr, owner *types.ClassSym, m *syntax.Modifiers, what, name string) {
		if protectedAPI || m == nil || m.Visibility != syntax.Protected || owner == nil || !publicAPIClass(owner) {
			return
		}
		if recv == nil && depth > 0 || recv != nil && localType(recv.Type()) {
			return
		}
		c.errorf(p, "protected %s call from public-API inline function is prohibited: '%s'", what, name)
	}
	receiver := func(f syntax.Expr) syntax.Expr {
		if m, ok := f.(*syntax.Member); ok {
			return m.X
		}
		return nil
	}
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.ObjectExpr:
			depth++
			inspectClass(n.Decl, visit)
			depth--
			return false
		case *syntax.LocalClass:
			locals[n.Decl.Name] = true
			depth++
			inspectClass(n.Decl, visit)
			depth--
			return false
		case *syntax.Super:
			if n.Label == "" && depth == 0 || n.Label != "" && !locals[n.Label] {
				c.errorf(n.P, "accessing super members from public-API inline function is prohibited")
			}
		case *syntax.Ident:
			if v := n.Sym; v != nil && v.Kind == types.Prop && n.Member == nil {
				protected(n.P, nil, v.Owner, varModifiers(v), "property", v.Name)
			}
		case *syntax.Member:
			if v := n.Prop; v != nil && v.Kind == types.Prop {
				if _, ok := n.X.(*syntax.Super); !ok {
					protected(n.P, n.X, v.Owner, varModifiers(v), "property", v.Name)
				}
			}
		case *syntax.Call:
			f := n.Sym
			if f == nil || n.Invoke {
				break
			}
			if n.Ctor != nil {
				if f.Decl != nil && depth == 0 && !protectedAPI && publicAPIClass(n.Ctor.Outer) && !localType(types.ClassType(n.Ctor)) {
					if cd, ok := n.Ctor.Decl.(*syntax.ClassDecl); ok && (cd.Visibility == syntax.Protected || ctorVisibility(f) == syntax.Protected) {
						c.errorf(n.P, "protected constructor call from public-API inline function is prohibited: '%s'", cd.Name)
					}
				}
				break
			}
			if fd, ok := f.Decl.(*syntax.FunDecl); ok && fd != d {
				recv := receiver(n.Fun)
				if _, ok := recv.(*syntax.Super); !ok {
					protected(n.P, recv, f.Owner, &fd.Modifiers, "function", f.Name)
				}
			}
		}
		return true
	}
	if d.Body != nil {
		syntax.Inspect(d.Body, visit)
	}
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
}

// inlineParams returns the parameters of inline function d whose
// arguments Kotlin inlines: of a function type, not nullable, noinline
// or vararg.
func inlineParams(d *syntax.FunDecl) map[*types.VarSym]bool {
	var ps map[*types.VarSym]bool
	for _, p := range d.Params {
		if p.Sym == nil || p.Noinline || p.Vararg {
			continue
		}
		if t := p.Sym.Type; t.N || !t.IsFunc() {
			continue
		}
		if ps == nil {
			ps = map[*types.VarSym]bool{}
		}
		ps[p.Sym] = true
	}
	return ps
}

// inlineUsage reports the uses of inline function d's inline parameters
// that aren't calls of them or arguments for inline parameters of other
// inline functions, as kotlinc does (USAGE_IS_NOT_INLINABLE): a lambda
// passed for one isn't a value, so the parameter can't be stored,
// compared, returned or be the receiver of an extension. It also reports
// the calls of those that aren't crossinline where a return in their
// lambdas couldn't leave d (NON_LOCAL_RETURN_NOT_ALLOWED): in a lambda
// that isn't inlined, or is passed for a crossinline parameter, in an
// object expression, a local class or a local function, or passed for a
// crossinline parameter.
func (c *Checker) inlineUsage(d *syntax.FunDecl) {
	ps := inlineParams(d)
	if ps == nil {
		return
	}
	cross := map[*types.VarSym]bool{}
	for _, p := range d.Params {
		if p.Crossinline && p.Sym != nil {
			cross[p.Sym] = true
		}
	}
	param := func(e syntax.Expr) *syntax.Ident {
		if id, ok := e.(*syntax.Ident); ok && id.Sym != nil && ps[id.Sym] && id.Member == nil && id.Call == nil {
			return id
		}
		return nil
	}
	allowed := map[*syntax.Ident]bool{}
	ctx := nlLocal                        // where the code is
	lambdas := map[*syntax.Lambda]nlCtx{} // the lambdas passed as arguments
	nonLocal := func(id *syntax.Ident, at nlCtx) {
		if at == nlForbidden && !cross[id.Sym] {
			c.errorf(id.P, "can't inline '%s' here: it may contain non-local returns", id.Name)
		}
	}
	var visit func(n any) bool
	in := func(at nlCtx, walk func()) {
		saved := ctx
		if at > ctx {
			ctx = at
		}
		walk()
		ctx = saved
	}
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Call:
			if id := param(n.Fun); id != nil && n.Invoke {
				allowed[id] = true // s(x)
				nonLocal(id, ctx)
			}
			if l, ok := n.Fun.(*syntax.Lambda); ok && n.Invoke {
				lambdas[l] = nlForbidden // { ... }()
			}
			if m, ok := n.Fun.(*syntax.Member); ok && m.Name == "invoke" && !m.Safe {
				if id := param(m.X); id != nil && n.Sym != nil && n.Sym.Owner != nil && n.Sym.Owner.Func {
					allowed[id] = true // s.invoke(x)
					nonLocal(id, ctx)
				}
			}
			for k, a := range n.Args {
				kind := inlineArg(n, k)
				at := nlUnknown
				switch {
				case kind == argNotInline:
					at = nlForbidden
				case kind == argInline && argParam(n, k).Crossinline:
					at = nlForbidden
				case kind == argInline:
					at = nlLocal
				}
				if id := param(unconverted(a)); id != nil && kind != argNotInline {
					allowed[id] = true
					nonLocal(id, max(ctx, at))
				}
				if l, ok := a.(*syntax.Lambda); ok && l.Ref == nil {
					lambdas[l] = at
				}
			}
		case *syntax.Lambda:
			if n.Ref != nil {
				break
			}
			at, ok := lambdas[n]
			if !ok {
				at = nlForbidden // a value
			}
			in(at, func() { syntax.Inspect(n.Body, visit) })
			return false
		case *syntax.LocalFun:
			in(nlForbidden, func() {
				for _, p := range n.Decl.Params {
					if p.Default != nil {
						syntax.Inspect(p.Default, visit)
					}
				}
				syntax.Inspect(n.Decl.Body, visit)
			})
			return false
		case *syntax.ObjectExpr:
			in(nlForbidden, func() { inspectClass(n.Decl, visit) })
			return false
		case *syntax.LocalClass:
			in(nlForbidden, func() { inspectClass(n.Decl, visit) })
			return false
		case *syntax.Ident:
			if param(n) != nil && !allowed[n] {
				c.errorf(n.P, "illegal usage of inline-parameter '%s' in '%s': add 'noinline' modifier to the parameter declaration", n.Name, d.Name)
			}
		}
		return true
	}
	for _, p := range d.Params {
		if p.Default != nil {
			syntax.Inspect(p.Default, visit)
		}
	}
	if d.Body != nil {
		syntax.Inspect(d.Body, visit)
	}
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
}

// unconverted returns the function value a suspend conversion converts
// in argument a (see suspendConverted), or a.
func unconverted(a syntax.Expr) syntax.Expr {
	x, ok := a.(*syntax.Call)
	if !ok || len(x.Args) != 1 {
		return a
	}
	m, ok := x.Fun.(*syntax.Member)
	if !ok || m.Name != "let" {
		return a
	}
	ch, ok := m.X.(*syntax.Checked)
	l, ok2 := x.Args[0].(*syntax.Lambda)
	if !ok || !ok2 || len(l.Params) != 1 || l.Params[0].Name != "$f" {
		return a
	}
	return ch.X
}

// nlCtx is whether a return in a lambda inlined in some code can leave
// the inline function the code is in.
type nlCtx int

const (
	nlLocal     nlCtx = iota // it can
	nlUnknown                // kc can't tell
	nlForbidden              // it can't
)

// argKind is what a call does with an argument of a function type.
type argKind int

const (
	argUnknown   argKind = iota // kc can't tell
	argInline                   // an inline parameter of an inline function inlines it
	argNotInline                // it is a value
)

// inlineArg reports whether call x passes its argument k to an inline
// parameter of an inline function, as Kotlin decides it.
func inlineArg(x *syntax.Call, k int) argKind {
	if x.Ctor != nil || x.Invoke {
		return argNotInline
	}
	p := argParam(x, k)
	switch {
	case p == nil:
		if x.Sym == nil {
			return argUnknown
		}
		if d, ok := x.Sym.Decl.(*syntax.FunDecl); ok && x.Sym.Builtin == types.NotBuiltin && !x.SAM && !x.Literal {
			if n := len(d.Params); n > 0 && k >= n && d.Params[n-1].Vararg && x.ArgMap == nil {
				return argNotInline
			}
		}
		return argUnknown
	case !x.Sym.Decl.(*syntax.FunDecl).Inline:
		return argNotInline
	case inlineParams(x.Sym.Decl.(*syntax.FunDecl))[p.Sym]:
		return argInline
	}
	return argNotInline
}

// argParam returns the parameter of the program's function that call x
// passes its argument k to, or nil.
func argParam(x *syntax.Call, k int) *syntax.Param {
	if x.Sym == nil || x.SAM || x.Literal || x.Ctor != nil || x.Invoke {
		return nil
	}
	d, ok := x.Sym.Decl.(*syntax.FunDecl)
	if !ok || x.Sym.Builtin != types.NotBuiltin {
		return nil
	}
	i := k
	switch {
	case x.ArgMap != nil:
		if k >= len(x.ArgMap) {
			return nil
		}
		i = x.ArgMap[k]
	case k < len(x.Names) && x.Names[k] != "":
		i = -1
		for j, p := range d.Params {
			if p.Name == x.Names[k] {
				i = j
			}
		}
	}
	if i < 0 || i >= len(d.Params) {
		return nil
	}
	return d.Params[i]
}

// virtualMember reports whether a member of owner with modifiers m can be
// overridden, which an inline one can't be, as kotlinc requires
// (DECLARATION_CANT_BE_INLINED): open, abstract, a non-private member of
// an interface, or an override that isn't final in a class that isn't.
func virtualMember(m syntax.Modifiers, owner *types.ClassSym) bool {
	if owner == nil || m.Visibility == syntax.Private {
		return false
	}
	return owner.Interface || m.Open || m.Abstract || m.Override && !m.Final && !owner.Final
}

// inlineProp checks an inline property, or one with an inline accessor:
// it can't be virtual, or have a backing field, which a copy of its
// accessors couldn't reach (INLINE_PROPERTY_WITH_BACKING_FIELD).
func (c *Checker) inlineProp(d *syntax.PropDecl, owner *types.ClassSym) {
	if !d.Inline && !d.InlineGetter && !d.InlineSetter {
		return
	}
	if virtualMember(d.Modifiers, owner) {
		p := d.P
		switch {
		case d.Inline:
		case d.InlineGetter && d.Getter != nil:
			p = d.Getter.P
		case d.InlineSetter && d.Setter != nil:
			p = d.Setter.P
		}
		c.errorf(p, "'inline' modifier is not allowed on virtual members: only private or final members can be inlined")
	}
	if d.Init != nil || d.Field != nil {
		c.errorf(d.P, "inline property cannot have a backing field")
	}
}

// inlineFileChecks checks the file's inline functions after all its code
// is checked.
func (c *Checker) inlineFileChecks() {
	c.objectInlineCalls()
	c.inlineCycles()
	c.inlinePropCycles()
	var funs []*syntax.FunDecl
	for _, d := range c.file.Decls {
		if fd, ok := d.(*syntax.FunDecl); ok {
			funs = append(funs, fd)
		}
	}
	for _, cd := range c.classes {
		for _, m := range cd.Members {
			if fd, ok := m.(*syntax.FunDecl); ok {
				funs = append(funs, fd)
			}
		}
	}
	for _, fd := range funs {
		if fd.Inline && fd.Sym != nil && !effectivelyPrivate(fd.Visibility, fd.Sym.Owner) {
			c.privateTypesInlined(fd)
		}
	}
}

// effectivelyPrivate reports whether a declaration with visibility v, a
// member of owner or top-level, is only visible in its file or class:
// private, or in a private or local class.
func effectivelyPrivate(v syntax.Visibility, owner *types.ClassSym) bool {
	if v == syntax.Private {
		return true
	}
	for k := owner; k != nil; k = k.Outer {
		if k.Local || k.Anonymous {
			return true
		}
		if cd, ok := k.Decl.(*syntax.ClassDecl); ok && (cd.Visibility == syntax.Private || cd.Local || cd.Anonymous) {
			return true
		}
	}
	return false
}

// privateTypesInlined reports the calls in non-private inline function d
// of the program's private inline functions whose bodies use a private
// class, which d's copies, compiled in other files, couldn't, as
// kotlinc's backend reports (PRIVATE_TYPE_USED_IN_NON_PRIVATE_INLINE_FUNCTION_ERROR).
func (c *Checker) privateTypesInlined(d *syntax.FunDecl) {
	visit := func(n any) bool {
		if l, ok := n.(*syntax.Lambda); ok && !l.Inline {
			return false
		}
		x, ok := n.(*syntax.Call)
		if !ok || x.Sym == nil {
			return true
		}
		g, ok := x.Sym.Decl.(*syntax.FunDecl)
		if !ok || g == d || !g.Inline || g.Visibility != syntax.Private {
			return true
		}
		if k := usesPrivateClass(g); k != nil {
			c.errorf(x.P, "private type '%s' is used in non-private inline function '%s' through '%s'", k.Name, d.Name, g.Name)
		}
		return true
	}
	if d.Body != nil {
		syntax.Inspect(d.Body, visit)
	}
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
}

// privateClassOf returns the private class t is, or is nested in, if any.
// Its type arguments, erased on the JVM, don't count.
func privateClassOf(t types.Type) *types.ClassSym {
	if t.K != types.Class || t.C == nil {
		return nil
	}
	for k := t.C; k != nil; k = k.Outer {
		if cd, ok := k.Decl.(*syntax.ClassDecl); ok && cd.Visibility == syntax.Private && !cd.Anonymous && !cd.Local {
			return t.C
		}
	}
	return nil
}

// usesPrivateClass returns a private class that the code of g, as its
// copies contain it, uses: the type of a value, a local variable or a
// parameter of a function it calls, or the class whose member it calls.
// Non-inlined lambdas and object expressions, whose classes kotlinc
// copies too, don't count.
func usesPrivateClass(g *syntax.FunDecl) *types.ClassSym {
	var found *types.ClassSym
	use := func(t types.Type) {
		if found == nil {
			found = privateClassOf(t)
		}
	}
	visit := func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			if !n.Inline {
				return false
			}
		case *syntax.ObjectExpr:
			return false
		case *syntax.VarStmt:
			if n.Sym != nil {
				use(n.Sym.Type)
			}
		case *syntax.Call:
			if f := n.Sym; f != nil {
				for _, p := range f.Params {
					use(p.Type)
				}
				for _, p := range f.Contexts {
					use(p.Type)
				}
				if f.Ext {
					use(f.Recv)
				}
				if f.Owner != nil {
					use(types.ClassType(f.Owner))
				}
			}
		}
		if e, ok := n.(syntax.Expr); ok && e.Type().K == types.Class {
			use(e.Type())
		}
		return found == nil
	}
	if g.Body != nil {
		syntax.Inspect(g.Body, visit)
	}
	if g.ExprBody != nil {
		syntax.Inspect(g.ExprBody, visit)
	}
	return found
}

// objectInlineCalls records the calls of the program's inline functions
// in the classes that the file's inline functions declare, object
// expressions' and local classes', as calls in those functions, whose
// copies contain copies of the classes: an inline function that an
// object in it calls is part of an inline call cycle. The checker records
// the calls in the functions' own code.
func (c *Checker) objectInlineCalls() {
	var fun func(d *syntax.FunDecl, outer []*types.FunSym)
	var code func(n any, self *types.FunSym, stack []*types.FunSym)
	code = func(n any, self *types.FunSym, stack []*types.FunSym) {
		var visit func(n any) bool
		class := func(cd *syntax.ClassDecl) {
			for _, m := range cd.Members {
				switch m := m.(type) {
				case *syntax.FunDecl:
					fun(m, stack)
				case *syntax.PropDecl:
					if m.Init != nil {
						code(m.Init, nil, stack)
					}
					fun(m.Getter, stack)
					fun(m.Setter, stack)
				case *syntax.InitBlock:
					code(m.Body, nil, stack)
				}
			}
		}
		visit = func(n any) bool {
			switch n := n.(type) {
			case *syntax.ObjectExpr:
				class(n.Decl)
				return false
			case *syntax.LocalClass:
				class(n.Decl)
				return false
			case *syntax.Call:
				if n.Sym == nil || syntax.SourceInline(n.Sym) == nil {
					break
				}
				for _, f := range stack {
					if f != self {
						c.inlineCalls = append(c.inlineCalls, inlineCall{from: inlineCode{f, -1}, callee: n.Sym, p: n.P})
					}
				}
			}
			return true
		}
		syntax.Inspect(n, visit)
	}
	fun = func(d *syntax.FunDecl, outer []*types.FunSym) {
		if d == nil {
			return
		}
		stack := outer
		if d.Sym != nil && syntax.SourceInline(d.Sym) != nil {
			stack = append(stack[:len(stack):len(stack)], d.Sym)
		}
		if len(stack) == 0 {
			return
		}
		if d.Body != nil {
			code(d.Body, d.Sym, stack)
		}
		if d.ExprBody != nil {
			code(d.ExprBody, d.Sym, stack)
		}
	}
	for _, d := range c.file.Decls {
		if fd, ok := d.(*syntax.FunDecl); ok {
			fun(fd, nil)
		}
	}
	for _, cd := range c.classes {
		if cd.Local || cd.Anonymous {
			continue
		}
		for _, m := range cd.Members {
			if fd, ok := m.(*syntax.FunDecl); ok {
				fun(fd, nil)
			}
		}
	}
}

// inlinePropCycles reports the reads of an inline property in its own
// getter, whose copies would contain themselves (INLINE_CALL_CYCLE).
func (c *Checker) inlinePropCycles() {
	check := func(d *syntax.PropDecl) {
		if d.Sym == nil {
			return
		}
		acc := func(f *syntax.FunDecl, inline bool) {
			if f == nil || !inline && !d.Inline {
				return
			}
			self := func(m *syntax.Member) bool {
				return m.Prop == d.Sym || d.Getter != nil && m.Getter != nil && m.Getter.Decl == d.Getter
			}
			visit := func(n any) bool {
				switch n := n.(type) {
				case *syntax.Ident:
					if n.Sym == d.Sym && n.Member == nil || n.Member != nil && self(n.Member) {
						c.errorf(n.P, "the call of '%s' is a part of an inline call cycle", d.Name)
					}
				case *syntax.Member:
					if self(n) {
						c.errorf(n.P, "the call of '%s' is a part of an inline call cycle", d.Name)
					}
				}
				return true
			}
			if f.Body != nil {
				syntax.Inspect(f.Body, visit)
			}
			if f.ExprBody != nil {
				syntax.Inspect(f.ExprBody, visit)
			}
		}
		acc(d.Getter, d.InlineGetter)
	}
	for _, d := range c.file.Decls {
		if pd, ok := d.(*syntax.PropDecl); ok {
			check(pd)
		}
	}
	for _, cd := range c.classes {
		for _, m := range cd.Members {
			if pd, ok := m.(*syntax.PropDecl); ok {
				check(pd)
			}
		}
	}
}

// inlineDecl checks the declaration of function d, local or not, after
// its parameters are resolved: noinline and crossinline only on
// parameters of function types of inline functions
// (ILLEGAL_INLINE_PARAMETER_MODIFIER), an inline function's inline
// parameters not nullable (NULLABLE_INLINE_PARAMETER) and their default
// values not calls (INVALID_DEFAULT_FUNCTIONAL_PARAMETER_FOR_INLINE), and
// a local function's type parameters not reified, as the other
// functions' aren't checked here.
func (c *Checker) inlineDecl(d *syntax.FunDecl, local bool) {
	if local {
		if d.Inline {
			return // not supported
		}
		for _, tp := range d.TypeParams {
			if tp.Reified {
				c.errorf(tp.P, "only type parameters of inline functions can be reified")
			}
		}
	}
	for _, p := range d.Params {
		if p.Sym == nil || p.Vararg {
			continue
		}
		t := p.Sym.Type
		if p.Noinline != p.Crossinline {
			mod := "noinline"
			if p.Crossinline {
				mod = "crossinline"
			}
			notFunc := t.K == types.Unit || t.K >= types.Bool && t.K <= types.Any ||
				t.K == types.Class && t.C != nil && !t.C.Func && !strings.Contains(t.C.Name, "KFunction") && !strings.Contains(t.C.Name, "KSuspendFunction")
			if !d.Inline || notFunc {
				c.errorf(p.P, "modifier '%s' is allowed only for function parameters of an inline function", mod)
			}
		}
		if !d.Inline || p.Noinline || !t.IsFunc() {
			continue
		}
		if t.N {
			c.errorf(p.P, "inline-parameter '%s' of '%s' must not be nullable. Add 'noinline' modifier to the parameter declaration or make its type not nullable", p.Name, d.Name)
		} else if _, ok := p.Default.(*syntax.Call); ok {
			c.errorf(p.Default.Pos(), "invalid default value for an inline functional parameter: only lambdas, anonymous functions and callable references are supported")
		}
	}
}
