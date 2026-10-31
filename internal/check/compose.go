package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Compose.
//
// With Library.Compose, kc compiles the program's @Composable functions
// as Compose's compiler plugin does, and calls those of the class path,
// which it knows by their descriptors: a composable function takes a
// Composer after its parameters, then ints of change flags and of masks of
// the omitted arguments (types.ComposeDesc). A Unit function's body runs
// in a restart group, whose scope calls the function again to recompose
// it, and skips if its arguments didn't change (see codegen/skip.go).
//
// A composable function type, @Composable (A) -> R, is a function type
// of its own (types.Functions), whose interface takes a Composer and an
// int of change flags after the parameters. A lambda whose expected type
// is one, or that is annotated @Composable, is composable: its body
// calls composables with the Composer it gets.
//
// An inline composable function gets no group: its body, copied into
// the caller, calls composables with the caller's Composer, and a
// composable lambda inlined into it with the Composer it is invoked
// with. Compose's key() { ... } is an intrinsic, as currentComposer is.
//
// A composable function can only be called from another one, or from a
// composable lambda, and lambdas inlined into their code. Conditional
// code that calls composables runs in replace groups of its own, as with
// kotlinc (see composeFlow). What kc doesn't compile yet is reported as
// unsupported, with the feature "compose", for kotlinc to compile, such as
// non-local jumps from the lambdas of inline composables, which would
// leave groups open.

// composePkg is the package of Compose's runtime and its annotations.
const composePkg = "androidx.compose.runtime"

// composableLambdaArity is the largest number of parameters, with the
// receiver, of a composable lambda: kotlinc's plugin fails to compile
// those with more.
const composableLambdaArity = 9

// composeAnno reports whether the annotations as have Compose's annotation
// name on target ("" for the declaration's own), as the file names it.
func (c *Checker) composeAnno(as []syntax.Annotation, target, name string) bool {
	for _, a := range as {
		if a.Target == target && c.pluginAnno(a, composePkg, name) {
			return true
		}
	}
	return false
}

// composeFun marks d's function composable, if it is annotated
// @Composable, and reports what kc doesn't compile of it yet. owner is
// the class it is a member of, or nil.
func (c *Checker) composeFun(d *syntax.FunDecl, owner *types.ClassSym) {
	sym := d.Sym
	if !c.u.compose || !c.composeAnno(d.Annotations, "", "Composable") {
		return
	}
	defaults := false
	for _, p := range d.Params {
		defaults = defaults || p.Default != nil
	}
	sym.Composable = true
	slots := len(d.Params)
	if d.Recv != nil {
		slots++
	}
	if owner != nil {
		slots++ // the object it is called on
	}
	sym.Changed = types.ChangedInts(slots)
	switch {
	case defaults && owner != nil && !sym.Final:
		sym.ComposeDefaults = true
	case defaults:
		sym.Defaults = types.DefaultInts(len(d.Params))
	}
	why := ""
	switch {
	case d.Suspend:
		why = "suspend composable functions"
	case d.Tailrec:
		why = "tailrec composable functions"
	case sym.Static:
		why = "@JvmStatic composable functions"
	case d.Find("", "JvmOverloads") != nil:
		why = "@JvmOverloads composable functions"
	case owner != nil && owner.Value:
		why = "composable functions of value classes"
	case owner != nil && owner.Interface && (d.Body != nil || d.ExprBody != nil):
		why = "composable functions with bodies in interfaces"
	case defaults && len(d.Params) > 31:
		why = "composable functions with default values and more than 31 parameters"
	case c.composeAnno(d.Annotations, "", "ExplicitGroupsComposable"):
		why = "@ExplicitGroupsComposable functions"
	}
	if why != "" {
		c.unsupported(d.P, "compose", "%s are not supported", why)
	}
}

// composeGetter marks the getter of property d, p's, composable, if it is
// annotated @Composable, as get() or with @get:Composable.
func (c *Checker) composeGetter(d *syntax.PropDecl, p *types.VarSym, owner *types.ClassSym) {
	if !c.u.compose || !c.composeAnno(d.Annotations, "get", "Composable") &&
		(d.Getter == nil || !c.composeAnno(d.Getter.Annotations, "", "Composable")) {
		return
	}
	why := ""
	switch {
	case d.Getter == nil || d.Getter.Body == nil && d.Getter.ExprBody == nil:
		why = "composable properties without a getter's body"
	case d.Recv != nil:
		why = "composable extension properties"
	case d.InlineGetter:
		why = "inline composable getters"
	case owner != nil && (owner.Interface || owner.Value):
		why = "composable properties of interfaces and value classes"
	case d.Field != nil:
		why = "delegated composable properties"
	}
	if why != "" {
		c.unsupported(d.P, "compose", "%s are not supported", why)
		return
	}
	g := d.Getter.Sym
	g.Composable, p.Composable = true, true
	slots := 0
	if owner != nil {
		slots++
	}
	g.Changed = types.ChangedInts(slots)
}

// composeCall checks a call at p of a composable function or of the
// getter of a property, named name.
func (c *Checker) composeCall(p syntax.Pos, name string) {
	if !c.u.compose {
		c.unsupported(p, "compose", "'%s' is @Composable: compile with -compose", name)
		return
	}
	switch c.composeScope() {
	case composeRef:
		c.unsupported(p, "compose", "references to composables are not supported yet")
	case composeNone:
		c.errorf(p, "@Composable invocations can only happen from the context of a @Composable function")
	}
}

// What composeScope finds.
const (
	composeNone = iota // code that isn't composable
	composeCode        // a composable function's or lambda's
	composeRef         // a callable reference's body
)

// composeScope reports whether the code being checked is composable: the
// innermost lambda whose body is code of its own is composable, or there
// is none and the function is, or the code is a default value of a
// composable's parameter, which its body computes. A lambda inlined into
// a composable's code is part of it.
func (c *Checker) composeScope() int {
	for i := len(c.lambdas) - 1; i >= c.base; i-- {
		l := c.lambdas[i]
		switch {
		case l.Ref != nil:
			return composeRef
		case l.Composable:
			return composeCode
		case l.Inline && !c.jumps[l].crossinline:
			continue
		}
		return composeNone
	}
	if f := c.fn; f != nil && f.Composable || f == nil && c.defaultsOf != nil && c.defaultsOf.Composable {
		return composeCode
	}
	return composeNone
}

// composeLambda checks composable lambda x, whose body has been checked,
// in the code around it.
func (c *Checker) composeLambda(x *syntax.Lambda) {
	switch {
	case x.Ref != nil:
		c.unsupported(x.P, "compose", "references to composables are not supported yet")
		return
	}
	if !x.Inline {
		c.composeFlow(false, false, x.Body) // an inlined one's is the code around it's
	}
}

// composeFunCall checks a call at p of f, if it is a composable function.
func (c *Checker) composeFunCall(p syntax.Pos, f *types.FunSym) {
	if f != nil && f.Composable {
		c.composeCall(p, f.Name)
	}
}

// composeRead checks x, if it reads a property whose getter is
// composable.
func (c *Checker) composeRead(x syntax.Expr) {
	switch x := x.(type) {
	case *syntax.Ident:
		if x.Sym != nil && x.Sym.Composable || x.Getter != nil && x.Getter.Composable {
			c.composeCall(x.P, x.Name)
		}
	case *syntax.Member:
		if x.Prop != nil && x.Prop.Composable || x.Getter != nil && x.Getter.Composable {
			c.composeCall(x.P, x.Name)
		}
	}
}

// composeKey checks call x of Compose's key(keys...) { ... }, which kc
// compiles as kotlinc's plugin does: the lambda, inlined, runs in a
// movable group, whose key the keys joined are. It takes keys, not an
// array of them, and a lambda.
func (c *Checker) composeKey(x *syntax.Call, cl *callee) {
	n := len(x.Args) - 1
	if n < 0 {
		return // the checker reports the missing lambda
	}
	l, ok := x.Args[n].(*syntax.Lambda)
	switch {
	case !ok || n == 0 || cl.paramIndex(n) != 1:
		c.unsupported(x.P, "compose", "key() is only supported with keys and a lambda")
		return
	case slices.ContainsFunc(x.Args[:n], func(a syntax.Expr) bool { _, ok := a.(*syntax.Spread); return ok }):
		c.unsupported(x.P, "compose", "key() is not supported with a spread array of keys")
		return
	}
	l.Inline = true
	j := c.jumps[l]
	j.callee, j.composable = "key", true
	c.jumps[l] = j
}

// composeFlow marks the code of a composable function or lambda, the
// roots of its body, that runs in groups of its own, as kotlinc's plugin
// does, which keep what the composables in each remember apart from what
// the code around it does:
//
//   - the branches of an if or a when whose branches call composables,
//     with an empty group for a missing else; the right side of &&, ||
//     and ?: that calls composables, and a safe call that does, with an
//     empty group for the code that skips them
//   - a loop, or a call of an inline function that isn't composable,
//     whose code calls composables, and that composable code in the same
//     group follows, or which runs where code may follow that kc doesn't
//     see: in a loop's body, a lambda that an inline function invokes, or
//     at the end of a function without a group of its own (inner)
//   - the body of a loop that calls composables and is continued
//
// Lambdas inlined into the code are part of it; a composable lambda that
// a composable remembers counts as a call. Composable calls in a try are
// an error, as with kotlinc, and kc doesn't compile those in a catch or
// a finally block yet.
//
// The code is a composable lambda's, whose method runs in a group, unless
// fun is set: a function's, which runs in one if restartable. It reports
// whether the code has a return before its end: a function without a
// restart group then runs in a replace group.
func (c *Checker) composeFlow(fun, restartable bool, roots ...any) bool {
	w := &composeWalk{c: c, returns: map[*syntax.Jump]bool{}}
	var last *syntax.Jump
	for _, r := range roots {
		w.calls(r)
		if b, ok := r.(*syntax.Block); ok && b != nil && len(b.Stmts) > 0 {
			if s, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt); ok {
				last, _ = s.X.(*syntax.Jump)
			}
		}
	}
	delete(w.returns, last)
	early := fun && len(w.returns) > 0
	if fun && !restartable && !early {
		w.event() // the caller's code follows, in the same group
	}
	return early
}

// composeWalk walks composable code for composeFlow.
type composeWalk struct {
	c *Checker
	// pending are the groups of loops and calls that composable code
	// after them realizes: the Group fields that it sets.
	pending []*bool
	events  int                   // the composable code walked
	returns map[*syntax.Jump]bool // the returns of the function
}

// event records composable code, which realizes the groups pending.
func (w *composeWalk) event() {
	for _, p := range w.pending {
		*p = true
	}
	w.pending = nil
	w.events++
}

// pend records the group of a loop or call that calls composables, after
// the code before it, which it follows.
func (w *composeWalk) pend(group *bool) {
	w.event()
	w.pending = append(w.pending, group)
}

// owned walks code n, which runs in a group of its own: the groups that
// stay pending in it aren't realized. It reports whether n calls
// composables.
func (w *composeWalk) owned(n any) bool {
	return w.ownedBy(func() bool { return w.calls(n) })
}

// ownedBy is owned for the code that walk walks.
func (w *composeWalk) ownedBy(walk func() bool) bool {
	saved, events := w.pending, w.events
	w.pending = nil
	found := walk()
	w.pending, w.events = saved, events
	return found
}

// inner walks code n, which may run again, or be followed by code that
// isn't walked: the groups pending in it are realized at its end.
func (w *composeWalk) inner(n any) bool {
	from, events := len(w.pending), w.events
	found := w.calls(n)
	if w.events != events {
		from = 0 // an event in n realized those before it
	}
	for _, p := range w.pending[min(from, len(w.pending)):] {
		*p = true
	}
	w.pending = w.pending[:min(from, len(w.pending))]
	return found
}

// loop walks a loop's condition, cond, and body, and marks its groups.
func (w *composeWalk) loop(loop syntax.Stmt, cond syntax.Expr, body *syntax.Block, group, bodyGroup *bool) bool {
	found := w.inner(cond)
	if continued(loop, body) {
		*bodyGroup = w.owned(body)
		found = *bodyGroup || found
	} else {
		found = w.inner(body) || found
	}
	if found {
		w.pend(group)
	}
	return found
}

// calls walks n, marking its groups, and reports whether it calls
// composables.
func (w *composeWalk) calls(n any) bool {
	found := false
	syntax.Inspect(n, func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			found = w.lambda(n) || found
			return false
		case *syntax.LocalFun, *syntax.LocalClass, *syntax.ObjectExpr:
			return false
		case *syntax.If:
			found = w.calls(n.Cond) || found
			then := w.owned(n.Then)
			n.Groups = w.owned(n.Else) || then
		case *syntax.When:
			found = w.calls(n.Subject) || found
			for _, br := range n.Branches {
				for _, wc := range br.Conds {
					found = w.calls(wc.X) || found
				}
				found = w.calls(br.Guard) || found
				if w.owned(br.Body) {
					n.Groups = true
				}
			}
		case *syntax.While:
			found = w.loop(n, n.Cond, n.Body, &n.Group, &n.BodyGroup) || found
			return false
		case *syntax.For:
			found = w.calls(n.Range) || found
			found = w.loop(n, nil, n.Body, &n.Group, &n.BodyGroup) || found
			return false
		case *syntax.Try:
			if w.calls(n.Body) {
				w.c.errorf(n.P, "try catch is not supported around composable function invocations.")
			}
			handlers := w.calls(n.Finally)
			for _, ct := range n.Catches {
				handlers = w.calls(ct.Body) || handlers
			}
			if handlers {
				w.c.unsupported(n.P, "compose", "composable calls in catch and finally blocks are not supported yet")
			}
			return false
		case *syntax.Binary:
			if n.Op != syntax.AND && n.Op != syntax.OR && n.Op != syntax.ELVIS {
				return true
			}
			found = w.calls(n.X) || found
			n.Groups = w.owned(n.Y)
		case *syntax.Member:
			found = w.calls(n.X) || found
			composable := n.Getter != nil && n.Getter.Composable || n.Prop != nil && n.Prop.Composable
			if n.Safe {
				n.Groups = composable
			} else if composable {
				w.event()
				found = true
			}
		case *syntax.Ident:
			if n.Sym != nil && n.Sym.Composable && !currentComposer(n.Sym) || n.Getter != nil && n.Getter.Composable {
				w.event()
				found = true
			}
			return true
		case *syntax.Call:
			found = w.call(n) || found
			return false
		case *syntax.Jump:
			if n.Kind == syntax.RETURN && n.Lambda == nil {
				w.returns[n] = true
			}
			return true
		default:
			return true
		}
		// An if, a when, a binary operator or a member access, whose
		// parts were walked.
		if g := groups(n); g != nil && *g {
			w.event()
			found = true
		}
		return false
	})
	return found
}

// currentComposer reports whether p is Compose's currentComposer, an
// intrinsic that calls no composable.
func currentComposer(p *types.VarSym) bool {
	return p.Name == "currentComposer" && p.Facade == "androidx/compose/runtime/ComposablesKt"
}

// groups returns the Groups field of an expression that has one.
func groups(n any) *bool {
	switch n := n.(type) {
	case *syntax.If:
		return &n.Groups
	case *syntax.When:
		return &n.Groups
	case *syntax.Binary:
		return &n.Groups
	case *syntax.Member:
		return &n.Groups
	}
	return nil
}

// lambda walks lambda x, and reports whether it calls composables where
// it is: a composable lambda that a composable remembers does, and an
// inlined one's code is the code around it's. A lambda inlined into a
// composable runs in a replace group if it returns a value, but for
// key()'s. Other lambdas are code of their own: see composeLambda.
func (w *composeWalk) lambda(x *syntax.Lambda) bool {
	j := w.c.jumps[x]
	switch {
	case remembered(x):
		w.event()
		return true
	case !x.Inline || j.crossinline:
		return false
	case x.Composable && (x.Ret.K != types.Unit || x.Ret.N) && j.callee != "key":
		return w.owned(x.Body)
	}
	return w.inner(x.Body)
}

// call walks call x, after its receiver and arguments, as they run.
func (w *composeWalk) call(x *syntax.Call) bool {
	recv := false
	m, member := x.Fun.(*syntax.Member)
	switch {
	case member:
		recv = w.calls(m.X)
	case x.Invoke:
		recv = w.calls(x.Fun)
	}
	inlined := false // calls in a lambda that an inline function invokes
	args := func() bool {
		found := false
		for _, a := range x.Args {
			calls := w.calls(a)
			if l, ok := a.(*syntax.Lambda); ok && l.Inline && !l.Composable {
				inlined = inlined || calls
			}
			found = found || calls
		}
		return found
	}
	composable := x.Sym != nil && x.Sym.Composable
	if member && m.Safe {
		// The arguments and the call run in the safe call's group.
		m.Groups = w.ownedBy(args) || composable
		if m.Groups {
			w.event()
		}
		return recv || m.Groups
	}
	found := args() || recv
	switch {
	case composable:
		w.event()
		return true
	case inlined:
		w.pend(&x.Group)
	}
	return found
}

// continued reports whether loop, whose body is body, is continued.
func continued(loop syntax.Stmt, body *syntax.Block) bool {
	found := false
	syntax.Inspect(body, func(n any) bool {
		if j, ok := n.(*syntax.Jump); ok && j.Kind == syntax.CONTINUE && j.Loop == loop {
			found = true
		}
		return !found
	})
	return found
}

// composeFunFlow checks the flow of composable function d: see
// composeFlow.
func (c *Checker) composeFunFlow(d *syntax.FunDecl) {
	var roots []any
	for _, p := range d.Params {
		roots = append(roots, p.Default)
	}
	if d.Body != nil {
		roots = append(roots, d.Body)
	}
	restartable := d.Sym.Ret.K == types.Unit && !d.Sym.Ret.N && !d.Inline &&
		!c.composeAnno(d.Annotations, "", "NonRestartableComposable") && !c.composeAnno(d.Annotations, "", "ReadOnlyComposable")
	d.Group = c.composeFlow(true, restartable, append(roots, d.ExprBody)...)
}

// remembered reports whether lambda x is a composable lambda whose object
// a composable remembers, rememberComposableLambda: one that returns
// Unit and isn't inlined, but for a singleton, which a static field
// holds.
func remembered(x *syntax.Lambda) bool {
	return x.Composable && !x.Inline && x.Ret.K == types.Unit && !x.Ret.N && !x.Singleton()
}
