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
