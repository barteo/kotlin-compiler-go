package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The program's inline functions. Code generation copies the body of a
// top-level or extension inline function, or a final member of a class,
// into its callers, and inlines there the lambdas passed for its
// function-typed parameters, which it only invokes. In a member's copy,
// this is the call's receiver.

// inlineFun checks an inline function's declaration and decides which of
// its parameters take inlined lambdas: those of a function type, not
// noinline, that its body only invokes.
func (c *Checker) inlineFun(d *syntax.FunDecl, owner *types.ClassSym) {
	if owner != nil && (owner.Interface || owner.Value || !isFinal(d.Modifiers, owner)) {
		return // an ordinary method
	}
	d.Inlinable = make([]bool, len(d.Params))
	for i, p := range d.Params {
		t := p.Sym.Type
		if p.Noinline || p.Vararg || t.N || !t.Flex().IsFunc() {
			continue
		}
		d.Inlinable[i] = onlyInvoked(d, p.Name, p.Crossinline)
	}
	c.inlineDecls = append(c.inlineDecls, d)
}

// forwardInlinable makes a crossinline parameter of the program's inline
// function a function object if the function passes it on to a parameter
// of another inline function, of the call's name, that is one: of the
// program's, or a top-level one of the class path whose body captures it.
// kc doesn't copy a crossinline lambda into a nested lambda, as kotlinc
// does, so clikt's convert(metavar, conversion), which passes its
// conversion to convert(metavarGetter, conversion), whose lambda invokes
// it, takes a function object too, and so does a test's fixture(config),
// which passes it to a library's inline featureFixture(config).
func forwardInlinable(checkers []*Checker) {
	byName := map[string][]*syntax.FunDecl{}
	var all []*syntax.FunDecl
	checkerOf := map[*syntax.FunDecl]*Checker{}
	for _, c := range checkers {
		for _, d := range c.inlineDecls {
			byName[d.Name] = append(byName[d.Name], d)
			all = append(all, d)
			checkerOf[d] = c
		}
	}
	for changed := true; changed; {
		changed = false
		for _, d := range all {
			for i, p := range d.Params {
				if d.Inlinable[i] && p.Crossinline && forwardedToObject(d, p.Name, byName, checkerOf[d]) {
					d.Inlinable[i] = false
					changed = true
				}
			}
		}
	}
}

// forwardedToObject reports whether the body of d, checked by c, passes
// its parameter name to a parameter of an inline function that takes a
// function object: one in byName, or one of the class path's.
func forwardedToObject(d *syntax.FunDecl, name string, byName map[string][]*syntax.FunDecl, c *Checker) bool {
	found := false
	visit := func(n any) bool {
		x, ok := n.(*syntax.Call)
		if !ok || found {
			return !found
		}
		callee := ""
		switch f := x.Fun.(type) {
		case *syntax.Ident:
			callee = f.Name
		case *syntax.Member:
			callee = f.Name
		}
		for k, a := range x.Args {
			if id, ok := a.(*syntax.Ident); !ok || id.Name != name {
				continue
			}
			for _, g := range byName[callee] {
				if g == d {
					continue
				}
				j := k
				switch {
				case k < len(x.Names) && x.Names[k] != "":
					j = slices.IndexFunc(g.Params, func(p *syntax.Param) bool { return p.Name == x.Names[k] })
				case x.Trailing() && k == len(x.Args)-1:
					j = len(g.Params) - 1
				}
				if j >= 0 && j < len(g.Inlinable) && !g.Inlinable[j] && !g.Params[j].Noinline {
					found = true
				}
			}
			if _, ok := x.Fun.(*syntax.Ident); ok && c != nil && len(byName[callee]) == 0 {
				found = found || c.libTakesObject(callee, x, k)
			}
		}
		return !found
	}
	if d.Body != nil {
		syntax.Inspect(d.Body, visit)
	}
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
	return found
}

// libTakesObject reports whether a top-level inline function of the class
// path, named name, that call x may call takes a function object for
// argument k: its body captures the parameter, as a crossinline one in a
// lambda.
func (c *Checker) libTakesObject(name string, x *syntax.Call, k int) bool {
	for _, fs := range c.kotlinFunLevels(name, false) {
		for _, f := range fs {
			if f.Inline == nil {
				continue
			}
			j := k
			switch {
			case k < len(x.Names) && x.Names[k] != "":
				j = slices.IndexFunc(f.Params, func(p *types.VarSym) bool { return p.Name == x.Names[k] })
			case x.Trailing() && k == len(x.Args)-1:
				j = len(f.Params) - 1
			}
			if j < 0 || j >= len(f.Inline.Noinline) || f.Inline.Noinline[j] {
				continue
			}
			b, err := inlineBody(f, false)
			if err == nil && b != nil && !inlinable(f, b, j) {
				return true
			}
		}
		if len(fs) > 0 {
			return false
		}
	}
	return false
}

// onlyInvoked reports whether the body of d uses its parameter name only
// to invoke it: name(args) or, for a function type with a receiver,
// x.name(args), or to pass it to a call, which must be of an inline
// function that inlines it in turn (see forwardedArgs). A crossinline
// parameter used in a lambda, which may not be inlined, an object
// expression or a local function is a function object: kc doesn't copy
// its lambda into theirs, as kotlinc does.
func onlyInvoked(d *syntax.FunDecl, name string, crossinline bool) bool {
	invoked := map[*syntax.Ident]bool{}
	ok := true
	visit := func(n any) bool {
		switch n.(type) {
		case *syntax.Lambda, *syntax.ObjectExpr, *syntax.LocalFun, *syntax.LocalClass:
			if crossinline && mentions(n, name) {
				ok = false
			}
		}
		switch n := n.(type) {
		case *syntax.Call:
			switch f := n.Fun.(type) {
			case *syntax.Ident:
				invoked[f] = f.Name == name
			case *syntax.Member:
				if f.Name == name {
					invoked[nil] = true // the receiver form, x.name()
				}
			}
			for _, a := range n.Args {
				if id, ok := a.(*syntax.Ident); ok && id.Name == name {
					invoked[id] = true // forwarded: f(name)
				}
			}
		case *syntax.Ident:
			if n.Name == name && !invoked[n] {
				ok = false
			}
		case *syntax.CallableRef:
			ok = false
		}
		return ok
	}
	syntax.Inspect(d.Body, visit)
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
	return ok
}

// mentions reports whether name occurs in node as an identifier or the
// name of a member call, also in the members of the object expressions
// and local classes in it.
func mentions(node any, name string) bool {
	found := false
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Ident:
			found = found || n.Name == name
		case *syntax.Member:
			found = found || n.Name == name
		case *syntax.ObjectExpr:
			inspectClass(n.Decl, visit)
		case *syntax.LocalClass:
			inspectClass(n.Decl, visit)
		}
		return !found
	}
	syntax.Inspect(node, visit)
	return found
}

// inspectClass inspects the code of class d, which syntax.Inspect
// doesn't enter: its supertypes' constructor calls and delegates, and its
// members' bodies and initializers.
func inspectClass(d *syntax.ClassDecl, f func(any) bool) {
	for _, st := range d.Supers {
		if st.Call != nil {
			syntax.Inspect(st.Call, f)
		}
		if st.Delegate != nil {
			syntax.Inspect(st.Delegate, f)
		}
	}
	fun := func(fd *syntax.FunDecl) {
		if fd == nil {
			return
		}
		if fd.Body != nil {
			syntax.Inspect(fd.Body, f)
		}
		if fd.ExprBody != nil {
			syntax.Inspect(fd.ExprBody, f)
		}
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			fun(m)
		case *syntax.PropDecl:
			if m.Init != nil {
				syntax.Inspect(m.Init, f)
			}
			fun(m.Getter)
			fun(m.Setter)
		case *syntax.InitBlock:
			syntax.Inspect(m.Body, f)
		case *syntax.ClassDecl:
			inspectClass(m, f)
		}
	}
}

// sourceInlineCall marks the lambdas that a call of the program's inline
// function d inlines, and records whether non-local jumps can leave them.
func (c *Checker) sourceInlineCall(x *syntax.Call, cl *callee, d *syntax.FunDecl) {
	if c.state.fn == cl.f {
		c.errorf(x.P, "inline function '%s' cannot be recursive", d.Name)
		return
	}
	c.recordInlineCall(x, cl)
	for k, a := range x.Args {
		l, ok := a.(*syntax.Lambda)
		i := cl.paramIndex(k)
		if !ok || i >= len(d.Inlinable) || !d.Inlinable[i] {
			continue
		}
		l.Inline = true
		j := c.jumps[l]
		j.callee, j.composable, j.suspends = cl.f.Name, cl.f.Composable, cl.f.Suspend
		j.crossinline = j.crossinline || d.Params[i].Crossinline
		l.Crossinline = j.crossinline
		c.jumps[l] = j
	}
}

// inlineCode is code that a copy of an inline function contains: its
// body (param -1) or the default value of a parameter.
type inlineCode struct {
	f     *types.FunSym
	param int
}

// inlineCall is a call, at p, in code of one of the program's inline
// functions, of another, whose copy contains callee's body and the
// default values of the parameters the call omits.
type inlineCall struct {
	from   inlineCode
	callee *types.FunSym
	omits  []int
	p      syntax.Pos
}

// recordInlineCall records a call x of the program's inline function
// cl.f, if it is in the code of another.
func (c *Checker) recordInlineCall(x *syntax.Call, cl *callee) {
	from := inlineCode{c.state.fn, -1}
	if from.f == nil {
		from = inlineCode{c.state.defaultsOf, c.state.defaultParam}
	}
	if syntax.SourceInline(from.f) == nil {
		return
	}
	used := map[int]bool{}
	for k := range x.Args {
		used[cl.paramIndex(k)] = true
	}
	ic := inlineCall{from: from, callee: cl.f, p: x.P}
	for j, p := range cl.f.Params {
		if !used[j] && p.Default {
			ic.omits = append(ic.omits, j)
		}
	}
	c.inlineCalls = append(c.inlineCalls, ic)
}

// inlineCycles reports the file's calls among inline functions whose
// copies would contain themselves, which code generation would copy
// forever: inline fun foo(x: Int = bar()), inline fun bar(x: Int = foo()).
func (c *Checker) inlineCycles() {
	calls := map[inlineCode][]inlineCode{}
	for _, ic := range c.inlineCalls {
		calls[ic.from] = append(calls[ic.from], ic.targets()...)
	}
	reaches := func(from []inlineCode, to inlineCode) bool {
		seen := map[inlineCode]bool{}
		for work := from; len(work) > 0; {
			n := work[len(work)-1]
			work = work[:len(work)-1]
			if n == to {
				return true
			}
			if !seen[n] {
				seen[n] = true
				work = append(work, calls[n]...)
			}
		}
		return false
	}
	reported := map[syntax.Pos]bool{}
	for _, ic := range c.inlineCalls {
		if !reported[ic.p] && reaches(ic.targets(), ic.from) {
			reported[ic.p] = true
			c.errorf(ic.p, "the call of '%s' is a part of an inline call cycle", ic.callee.Name)
		}
	}
}

// targets returns the code a call's copy contains.
func (ic *inlineCall) targets() []inlineCode {
	out := []inlineCode{{ic.callee, -1}}
	for _, j := range ic.omits {
		out = append(out, inlineCode{ic.callee, j})
	}
	return out
}

// hasReified reports whether f has reified type parameters.
func hasReified(f *types.FunSym) bool {
	for _, tp := range f.TypeParams {
		if tp.Reified {
			return true
		}
	}
	return false
}

// reifiedArgs records the type arguments of a call x of f, which has
// reified type parameters: they must be known when the program runs, so
// not type parameters, which are erased, unless reified in turn.
func (c *Checker) reifiedArgs(x *syntax.Call, f *types.FunSym, sub types.Subst, s *system) {
	x.Reified = c.reifiedTypes(x.P, f, sub, s)
}

// reifiedProp records the type arguments of a read x of an extension
// property whose getter g has reified type parameters, as reifiedArgs.
func (c *Checker) reifiedProp(x *syntax.Member, g *types.FunSym, sub types.Subst, s *system) {
	x.Reified = c.reifiedTypes(x.P, g, sub, s)
}

func (c *Checker) reifiedTypes(pos syntax.Pos, f *types.FunSym, sub types.Subst, s *system) []types.Type {
	ts := make([]types.Type, len(f.TypeParams))
	for i, tp := range f.TypeParams {
		t := s.final(sub.Apply(types.ParamType(tp)))
		ts[i] = t
		if tp.Reified && t.IsParam() && !t.Param().Reified {
			c.errorf(pos, "cannot use '%s' as reified type parameter", t.Param().Name)
		}
	}
	if f.Builtin == types.TypeOf {
		if p := recursiveBound(ts[0], nil); p != nil {
			c.errorf(pos, "non-reified type parameters with recursive bounds are not supported yet: %s", p.Name)
		}
	}
	return ts
}

// recursiveBound returns a type parameter of t, or of the bounds of
// those, whose bounds lead back to a type parameter they are in, as
// T : Comparable<T>: the one whose bound closes the cycle, as kotlinc
// names it. typeOf<T>() builds the KTypeParameter of a type parameter
// with its bounds, which kotlinc doesn't for recursive ones. A reified
// one in t, not in a bound, is replaced by its type argument. outer are
// the type parameters whose bounds t is in.
func recursiveBound(t types.Type, outer []*types.TypeParamSym) *types.TypeParamSym {
	if t.IsParam() {
		p := t.Param()
		if slices.Contains(outer, p) {
			return outer[len(outer)-1]
		}
		if p.Reified && outer == nil {
			return nil
		}
		for _, b := range p.Bounds {
			if r := recursiveBound(b, append(outer, p)); r != nil {
				return r
			}
		}
		return nil
	}
	if t.A != nil {
		for _, a := range t.A.List {
			if r := recursiveBound(a.T, outer); r != nil {
				return r
			}
		}
	}
	return nil
}

// publicInline checks the body of an inline function that is public API,
// public or protected in a public class: its copies are compiled into
// other code, which can't use the private and internal declarations the
// body would, as kotlinc requires. @PublishedApi internal ones it can.
func (c *Checker) publicInline(d *syntax.FunDecl, cls *types.ClassSym) {
	if d.Visibility == syntax.Private || d.Visibility == syntax.Internal && d.Find("", "PublishedApi") == nil {
		return
	}
	for k := cls; k != nil; k = k.Outer {
		if cd, ok := k.Decl.(*syntax.ClassDecl); ok && (cd.Visibility == syntax.Private || cd.Visibility == syntax.Internal) {
			return
		}
	}
	report := func(p syntax.Pos, what, name string, m *syntax.Modifiers) {
		if m == nil || m.Find("", "PublishedApi") != nil {
			return
		}
		if m.Visibility == syntax.Private || m.Visibility == syntax.Internal {
			c.errorf(p, "public-API inline function cannot access non-public-API %s '%s'", what, name)
		}
	}
	visit := func(n any) bool {
		switch n := n.(type) {
		case *syntax.Ident:
			if v := n.Sym; v != nil && (v.Kind == types.Prop || v.Kind == types.Global) {
				report(n.P, "property", v.Name, varModifiers(v))
			}
		case *syntax.Member:
			if v := n.Prop; v != nil && (v.Kind == types.Prop || v.Kind == types.Global) {
				report(n.P, "property", v.Name, varModifiers(v))
			}
		case *syntax.Call:
			if f := n.Sym; f != nil {
				if fd, ok := f.Decl.(*syntax.FunDecl); ok && fd != d {
					report(n.P, "function", f.Name, &fd.Modifiers)
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

// varModifiers returns the modifiers of a property of the program, or nil.
func varModifiers(v *types.VarSym) *syntax.Modifiers {
	switch d := v.Decl.(type) {
	case *syntax.PropDecl:
		return &d.Modifiers
	case *syntax.CtorParam:
		return &d.Modifiers
	}
	return nil
}

// forwardedArgs checks the arguments of call x, of cl, that are inlined
// lambda parameters of the inline function the call is in: the callee
// must inline them too, as Kotlin requires of inline parameters, and
// kc's copies of the function then pass the lambda on.
func (c *Checker) forwardedArgs(x *syntax.Call, cl *callee) {
	d := syntax.SourceInline(c.state.fn)
	if d == nil {
		return
	}
	for k, a := range x.Args {
		id, ok := a.(*syntax.Ident)
		if !ok || id.Sym == nil || id.Sym.Kind != types.Param {
			continue
		}
		j := -1
		for i, p := range d.Params {
			if p.Sym == id.Sym && d.Inlinable[i] {
				j = i
			}
		}
		if j < 0 {
			continue
		}
		i := cl.paramIndex(k)
		if sd := syntax.SourceInline(cl.f); sd != nil {
			if i < len(sd.Inlinable) && sd.Inlinable[i] {
				continue
			}
		} else if b, err := inlineBody(cl.f, cl.omitted(len(x.Args)) > 0); err == nil && b != nil && inlinable(cl.f, b, i) {
			p := b.Params[jvmParam(cl.f, b, i)]
			uncopied := false
			for _, t := range p.InTry {
				uncopied = uncopied || t
			}
			if !uncopied {
				continue
			}
			c.unsupported(id.P, "non-local jump from a try", "passing inline parameter '%s' to '%s' isn't supported: kc doesn't copy its finally block", id.Name, cl.f.Name)
			continue
		}
		c.errorf(id.P, "illegal usage of inline-parameter '%s' in '%s': add 'noinline' modifier to the parameter declaration", id.Name, d.Name)
	}
}
