package check

import (
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
// compared, returned or be the receiver of an extension.
func (c *Checker) inlineUsage(d *syntax.FunDecl) {
	ps := inlineParams(d)
	if ps == nil {
		return
	}
	param := func(e syntax.Expr) *syntax.Ident {
		if id, ok := e.(*syntax.Ident); ok && id.Sym != nil && ps[id.Sym] && id.Member == nil && id.Call == nil {
			return id
		}
		return nil
	}
	allowed := map[*syntax.Ident]bool{}
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Call:
			if id := param(n.Fun); id != nil && n.Invoke {
				allowed[id] = true // s(x)
			}
			if m, ok := n.Fun.(*syntax.Member); ok && m.Name == "invoke" && !m.Safe {
				if id := param(m.X); id != nil && n.Sym != nil && n.Sym.Owner != nil && n.Sym.Owner.Func {
					allowed[id] = true // s.invoke(x)
				}
			}
			for k, a := range n.Args {
				if id := param(a); id != nil && inlineArg(n, k) != argNotInline {
					allowed[id] = true
				}
			}
		case *syntax.ObjectExpr:
			inspectClass(n.Decl, visit)
		case *syntax.LocalClass:
			inspectClass(n.Decl, visit)
		case *syntax.Ident:
			if param(n) != nil && !allowed[n] {
				c.errorf(n.P, "illegal usage of inline-parameter '%s' in '%s': add 'noinline' modifier to the parameter declaration", n.Name, d.Name)
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
	switch {
	case x.Ctor != nil || x.Invoke:
		return argNotInline
	case x.Sym == nil || x.SAM || x.Literal:
		return argUnknown
	}
	d, ok := x.Sym.Decl.(*syntax.FunDecl)
	if !ok || x.Sym.Builtin != types.NotBuiltin {
		return argUnknown
	}
	i := k
	switch {
	case x.ArgMap != nil:
		if k >= len(x.ArgMap) {
			return argUnknown
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
		if n := len(d.Params); n > 0 && i >= n && d.Params[n-1].Vararg {
			return argNotInline
		}
		return argUnknown
	}
	if !d.Inline {
		return argNotInline
	}
	if inlineParams(d)[d.Params[i].Sym] {
		return argInline
	}
	return argNotInline
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
	c.inlineCycles()
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
