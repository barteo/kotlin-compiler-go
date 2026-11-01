package codegen

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Compose.
//
// A composable function's method takes a Composer after its parameters,
// $composer, then ints of change flags, $changed, and, if a parameter has
// a default value, ints of masks of the omitted arguments, $default (see
// check/compose.go). Its calls of composable functions pass $composer,
// the change flags of the arguments, which say what the caller knows of
// them, and the masks. A function that returns Unit, but for a
// @NonRestartableComposable or @ReadOnlyComposable one, runs in a restart
// group, and skips its body if its arguments are the same as last time
// (see skip.go):
//
//	$composer = $composer.startRestartGroup(key)
//	(if the body runs: the default values of the omitted arguments)
//	(the body; returns go to the end)
//	$composer.endRestartGroup()?.updateScope { c, _ ->
//		f(arguments, c, updateChangedFlags($changed | 1), $default)
//	}
//
// The scope's lambda recomposes the group by calling the function again,
// with the force flag set. As kotlinc's, it is a private static method,
// f$lambda$N, that takes the arguments, then the Composer and the int of
// the Function2 that LambdaMetafactory makes of it.
//
// A composable lambda's method takes the Composer and the change flags
// after its parameters, as Objects, as its function type's interface's
// invoke does. Compose's runtime runs one that returns Unit in a restart
// group of its own: a composable remembers the lambda's object in a
// ComposableLambda, which it updates with the lambda's new object when
// it recomposes,
//
//	rememberComposableLambda(key, tracked, lambda, $composer, flags)
//
// and other code wraps it in one, composableLambdaInstance(key, tracked,
// lambda), tracked if it captures anything, as kotlinc's; one that
// captures nothing is created once, in the file's ComposableSingletons
// class (see singletons.go). A lambda that returns Unit skips its body as
// a function does; the body of one that returns a value runs in a
// replace group, whose key tells lambdas apart that a call site calls in
// turn.
//
// An inline composable's method has no group, and its copies call
// composables with the caller's $composer, and pass on the change flags
// that the call computes for its parameters. A composable lambda inlined
// where such a body invokes it takes the Composer and the change flags
// that the invoke passes from the stack, and calls composables with that
// Composer, passing on its parameters' flags (see skip.go's flagScope);
// one that returns a value still runs in a replace group. As
// with kotlinc, currentComposer is $composer, and key(k1, k2) { ... } is
//
//	$composer.startMovableGroup(key, $composer.joinKey(k1, k2))
//	(the lambda's body)
//	$composer.endMovableGroup()
//
// The code that the checker marks (see check/compose.go's composeFlow),
// branches, loops and calls of inline functions that call composables,
// and the body of a function without a restart group that returns early,
// runs in a replace group:
//
//	$composer.startReplaceGroup(key)
//	(the code)
//	$composer.endReplaceGroup()
//
// composeFn.groups are the groups open where the code being generated
// is. A return, break or continue ends those it leaves, which were opened
// since its target started, as an inlined lambda's return does.

// composerClass is the class of $composer.
const composerClass = "androidx/compose/runtime/Composer"

// composablesKt is the facade of the runtime's composables, and of its
// intrinsics: currentComposer, whose getter only throws, and key().
const composablesKt = "androidx/compose/runtime/ComposablesKt"

// composeFn holds the locals of the composable function whose method is
// being generated.
type composeFn struct {
	composer int   // $composer
	changed  []int // $changed, $changed1, ...
	defaults []int // $default, $default1, ...
	this     bool  // a method, whose this is in local 0
	// end is set in a function with a restart group: the epilogue that
	// returns go to.
	end *cf.Label
	// restart generates the method of the lambda that recomposes the
	// group, after the function's.
	restart func()
	// groups are the replace groups open where the code being generated
	// is, outermost first: the locals of the Composers that end them.
	groups []int
	// Skipping: see skip.go. flagScope says which parameters' change
	// flags the code being generated knows, and skip is where the
	// prologue jumps when the function skips its body.
	*flagScope
	skip *cf.Label
	// dontMemoize is set in a lambda annotated @DontMemoize: its code
	// doesn't remember its lambdas.
	dontMemoize bool
	// impl is set in a name$default method of a ComposeDefaultImpls
	// class: its descriptor.
	impl string
}

// composeParams allocates the locals of the parameters of composable
// function f's method that follow its own. this is set on a method.
func (g *gen) composeParams(f *types.FunSym, this bool) *composeFn {
	c := &composeFn{this: this}
	c.composer = g.code.AllocLocal(types.ComposerDesc)
	g.code.NameLocal(c.composer, "$composer")
	for i := range f.Changed {
		c.changed = append(c.changed, g.code.AllocLocal("I"))
		g.code.NameLocal(c.changed[i], "$changed"+suffix(i))
	}
	for i := range f.Defaults {
		c.defaults = append(c.defaults, g.code.AllocLocal("I"))
		g.code.NameLocal(c.defaults[i], "$default"+suffix(i))
	}
	c.flagScope = &flagScope{flags: c.changed}
	return c
}

// suffix returns what ends the name of the i-th of the locals that share
// a name: nothing for the first, then 1, 2 and so on.
func suffix(i int) string {
	if i == 0 {
		return ""
	}
	return strconv.Itoa(i)
}

// restartable reports whether composable function fd runs in a restart
// group: if it returns Unit, as kotlinc's, but for a
// @NonRestartableComposable or @ReadOnlyComposable one, or an inline one,
// whose body is the caller's code.
func restartable(fd *syntax.FunDecl) bool {
	if fd.Sym.Ret.K != types.Unit || fd.Sym.Ret.N || fd.Inline {
		return false
	}
	for _, a := range fd.Annotations {
		if a.Target == "" && (composeAnno(a, "NonRestartableComposable") || composeAnno(a, "ReadOnlyComposable")) {
			return false
		}
	}
	return true
}

// composeAnno reports whether annotation a is Compose's runtime's
// annotation name. The checker found the ones the function has.
func composeAnno(a syntax.Annotation, name string) bool {
	return a.Name == name || a.Name == "androidx.compose.runtime."+name
}

// groupKey returns the key of the group of composable function fd: a
// hash of where it is, which tells it apart from the other groups.
func (g *gen) groupKey(fd *syntax.FunDecl) int32 {
	h := fnv.New32a()
	h.Write([]byte(filepath.Base(g.file.Path) + ":" + strconv.Itoa(fd.P.Line) + ":" + strconv.Itoa(fd.P.Col) + ":" + fd.Name))
	return int32(h.Sum32())
}

// composeStart starts the body of composable function fd: its restart
// group, if it has one, and, if it can skip its body, the prologue that
// decides (see skip.go); the default values of the omitted arguments,
// and the replace group of a body that needs one without a restart group
// (see syntax.FunDecl.Group), which returns end.
func (g *gen) composeStart(fd *syntax.FunDecl) {
	g.composeBegin(fd, funTracked(fd, fd.Sym.Owner != nil), g.groupKey(fd), !fd.Sym.ComposeDefaults)
	if fd.Group && g.compose.end == nil {
		g.openGroup(g.siteKey(fd.P, "body", 0))
	}
}

// composeBegin starts the body of composable function fd, whose tracked
// parameters are t, as composeStart does, with group key key, computing
// the default values of omitted arguments if defaults is set.
func (g *gen) composeBegin(fd *syntax.FunDecl, t *tracked, key int32, defaults bool) {
	c := g.compose
	for i := range t.params {
		if def := t.defaults[i]; def >= 0 {
			t.nonStatic[i] = !g.isStatic(fd.Params[def].Default)
		}
	}
	c.tracked, c.typeParams = t, fd.Sym.TypeParams
	if restartable(fd) {
		c.end = cf.NewLabel()
		g.code.Local(cf.ALOAD, c.composer)
		g.code.Iconst(key)
		g.code.InvokeInterface(composerClass, "startRestartGroup", "(I)"+types.ComposerDesc)
		g.code.Local(cf.ASTORE, c.composer)
		if !hasComposeAnno(fd, "NonSkippableComposable") {
			g.skipPrologue(c, t, fd.Params)
			return
		}
	}
	for j, p := range fd.Params {
		if p.Default == nil || !defaults {
			continue
		}
		skip := cf.NewLabel()
		g.code.Local(cf.ILOAD, c.defaults[j/31])
		g.code.Iconst(int32(1) << (j % 31))
		g.code.Op(cf.IAND)
		g.code.Jump(cf.IFEQ, skip)
		g.exprAs(p.Default, p.Sym.Type)
		g.code.Local(storeOp(p.Sym.Type), p.Sym.Slot)
		g.code.Place(skip)
	}
}

// hasComposeAnno reports whether fd is annotated with Compose's runtime's
// annotation name.
func hasComposeAnno(fd *syntax.FunDecl, name string) bool {
	for _, a := range fd.Annotations {
		if a.Target == "" && composeAnno(a, name) {
			return true
		}
	}
	return false
}

// composeEnd ends the method of composable function fd, if it has a
// restart group, which the body's returns jumped to the end of: the group
// ends, and its scope gets the lambda that recomposes it.
func (g *gen) composeEnd(fd *syntax.FunDecl) {
	c := g.compose
	if c == nil || c.end == nil {
		return
	}
	if c.skip != nil {
		g.code.Jump(cf.GOTO, c.end)
		g.skipEpilogue(c)
	}
	g.code.Place(c.end)
	if !g.code.Reachable() {
		return // the body never returns
	}
	g.code.Local(cf.ALOAD, c.composer)
	g.code.InvokeInterface(composerClass, "endRestartGroup", "()Landroidx/compose/runtime/ScopeUpdateScope;")
	scope := g.code.AllocLocal("Landroidx/compose/runtime/ScopeUpdateScope;")
	g.code.Local(cf.ASTORE, scope)
	done := cf.NewLabel()
	g.code.Local(cf.ALOAD, scope)
	g.code.Jump(cf.IFNULL, done)
	g.code.Local(cf.ALOAD, scope)
	g.restartLambda(fd)
	g.code.InvokeInterface("androidx/compose/runtime/ScopeUpdateScope", "updateScope", "(Lkotlin/jvm/functions/Function2;)V")
	g.code.Place(done)
	g.code.Op(cf.RETURN)
}

// restartLambda creates the lambda that recomposes the restart group of
// composable function fd, passing it the function's arguments, and
// generates its method after fd's.
func (g *gen) restartLambda(fd *syntax.FunDecl) {
	c, f := g.compose, fd.Sym
	captures := ""
	if c.impl != "" {
		// A name$default method's arguments: its parameters.
		ps := descParams(c.impl)
		for i, slot := 0, 0; i < len(ps)-1-len(c.changed)-len(c.defaults); i++ {
			g.code.Local(loadOpOf(ps[i]), slot)
			captures += ps[i]
			slot += descSize(ps[i])
		}
	} else {
		if c.this {
			g.code.Local(cf.ALOAD, 0)
			captures += "L" + g.cls.Name + ";"
		}
		if r := fd.RecvSym; r != nil {
			g.code.Local(loadOp(r.Type), r.Slot)
			captures += paramDesc(r.Type)
		}
		for _, p := range f.Params {
			g.code.Local(loadOp(p.Type), p.Slot)
			captures += paramDesc(p.Type)
		}
	}
	for _, slot := range append(c.changed[:len(c.changed):len(c.changed)], c.defaults...) {
		g.code.Local(cf.ILOAD, slot)
		captures += "I"
	}
	key := g.cls.Name + "." + g.prefix
	name := g.prefix + "$" + strconv.Itoa(g.shared.count[key])
	g.shared.count[key]++
	implDesc := "(" + captures + types.ComposerDesc + "I)Lkotlin/Unit;"
	const samDesc = "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"
	p := &g.cls.Pool
	bsm := p.Bootstrap(
		p.MethodHandle(cf.RefInvokeStatic, p.Method("java/lang/invoke/LambdaMetafactory", "metafactory", metafactory)),
		p.MethodType(samDesc),
		p.MethodHandle(cf.RefInvokeStatic, p.Method(g.cls.Name, name, implDesc)),
		p.MethodType("("+types.ComposerDesc+"Ljava/lang/Integer;)Lkotlin/Unit;"))
	g.code.InvokeDynamic(bsm, "invoke", "("+captures+")Lkotlin/jvm/functions/Function2;")
	c.restart = func() { g.restartMethod(fd, name, implDesc) }
}

// restartMethod generates the method of the lambda that recomposes the
// restart group of composable function fd: it calls the function with the
// arguments it captured and the Composer it gets, with the force flag
// set in the change flags.
func (g *gen) restartMethod(fd *syntax.FunDecl, name, implDesc string) {
	f := fd.Sym
	g.code = g.cls.NewMethod(cf.AccPrivate|cf.AccStatic|cf.AccFinal|cf.AccSynthetic, name, implDesc)
	ps := descParams(implDesc)
	slots := make([]int, len(ps))
	for i, d := range ps {
		slots[i] = g.code.AllocLocal(d)
	}
	n := len(ps) - 2 - len(g.compose.changed) - len(g.compose.defaults) // the receivers and parameters
	for i := range n {
		g.code.Local(loadOpOf(ps[i]), slots[i])
	}
	g.code.Local(cf.ALOAD, slots[len(ps)-2])
	for i := range g.compose.changed {
		g.code.Local(cf.ILOAD, slots[n+i])
		if i == 0 {
			g.code.Iconst(1)
			g.code.Op(cf.IOR)
		}
		g.code.Invoke(cf.INVOKESTATIC, "androidx/compose/runtime/RecomposeScopeImplKt", "updateChangedFlags", "(I)I")
	}
	for i := range g.compose.defaults {
		g.code.Local(cf.ILOAD, slots[n+len(g.compose.changed)+i])
	}
	switch {
	case g.compose.impl != "":
		g.code.Invoke(cf.INVOKESTATIC, g.cls.Name, f.JVM()+"$default", g.compose.impl)
	case g.compose.this:
		g.callMethod(f, f.Owner, f.JVM(), funDesc(f))
	default:
		g.code.Invoke(cf.INVOKESTATIC, g.cls.Name, f.JVM(), funDesc(f))
	}
	g.code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
	g.code.Op(cf.ARETURN)
	g.check()
}

// composableLambdaKt is the class of the runtime's functions that wrap
// composable lambdas.
const composableLambdaKt = "androidx/compose/runtime/internal/ComposableLambdaKt"

// composeLambdaWrap starts wrapping composable lambda x, if it returns
// Unit, in a ComposableLambda, before its object is created: it returns
// the function that ends it, after.
func (g *gen) composeLambdaWrap(x *syntax.Lambda) func() {
	if !x.Composable || x.Ret.K != types.Unit || x.Ret.N {
		return func() {}
	}
	g.code.Iconst(g.lambdaKey(x))
	// Tracked, as kotlinc's, if it captures anything: a new object of it
	// then recomposes the code that invoked it.
	if len(x.Captures) > 0 || x.This {
		g.code.Iconst(1)
	} else {
		g.code.Iconst(0)
	}
	return func() {
		if g.compose != nil {
			g.loadComposer()
			g.code.Iconst(slotBits(paramStatic, 0) | slotBits(paramStatic, 1)) // the key and tracked
			g.code.Invoke(cf.INVOKESTATIC, composableLambdaKt, "rememberComposableLambda",
				"(IZLjava/lang/Object;"+types.ComposerDesc+"I)Landroidx/compose/runtime/internal/ComposableLambda;")
		} else {
			g.code.Invoke(cf.INVOKESTATIC, composableLambdaKt, "composableLambdaInstance",
				"(IZLjava/lang/Object;)Landroidx/compose/runtime/internal/ComposableLambda;")
		}
		g.code.Checkcast(x.T.C.Name)
	}
}

// lambdaKey returns the key of composable lambda x's group: a hash of
// where it is.
func (g *gen) lambdaKey(x *syntax.Lambda) int32 {
	h := fnv.New32a()
	h.Write([]byte(filepath.Base(g.file.Path) + ":" + strconv.Itoa(x.P.Line) + ":" + strconv.Itoa(x.P.Col) + ":lambda"))
	return int32(h.Sum32())
}

// composeLambdaStart starts the body of composable lambda x's method,
// whose locals in slots hold the Composer and the change flags: its
// $composer and $changed, and the prologue of one that can skip its body
// (see skip.go), which returns Unit and has no parameter of a type known
// to be unstable, or else the replace group of one that returns a value.
func (g *gen) composeLambdaStart(x *syntax.Lambda, slots []int) {
	c := &composeFn{composer: g.code.AllocLocal(types.ComposerDesc), dontMemoize: x.DontMemoize}
	g.code.Local(cf.ALOAD, slots[0])
	g.code.Checkcast(composerClass)
	g.code.Local(cf.ASTORE, c.composer)
	g.code.NameLocal(c.composer, "$composer")
	for i, s := range slots[1:] {
		ch := g.code.AllocLocal("I")
		g.code.Local(cf.ALOAD, s)
		g.code.Checkcast("java/lang/Number")
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/Number", "intValue", "()I")
		g.code.Local(cf.ISTORE, ch)
		g.code.NameLocal(ch, "$changed"+suffix(i))
		c.changed = append(c.changed, ch)
	}
	t := lambdaTracked(x)
	c.flagScope = &flagScope{tracked: t, flags: c.changed}
	g.compose = c
	if x.Ret.K != types.Unit || x.Ret.N {
		g.openGroup(g.lambdaKey(x))
		return
	}
	for _, typ := range t.types {
		if g.stabilityOf(typ).knownUnstable() {
			return
		}
	}
	g.skipPrologue(c, t, nil)
}

// composeLambdaSkip ends the method of a composable lambda that can skip
// its body, after the body's return: the Composer skips the group, and
// the lambda returns Unit.
func (g *gen) composeLambdaSkip() {
	if c := g.compose; c != nil && c.skip != nil {
		g.skipEpilogue(c)
		g.lambdaReturn(nil)
	}
}

// composeLambdaEnd ends the groups open in the composable lambda whose
// method is being generated, before it returns.
func (g *gen) composeLambdaEnd() {
	g.endGroups(0)
}

// composeInvoke takes the Composer and the change flags that an inline
// function passes when it invokes composable lambda l, which is inlined
// there, from the stack, and returns the locals that hold them.
func (g *gen) composeInvoke(l *syntax.Lambda) (composer int, flags []int) {
	c := g.code
	flags = make([]int, l.T.C.Methods["invoke"][0].Changed)
	for i := len(flags) - 1; i >= 0; i-- {
		c.Checkcast("java/lang/Number")
		c.Invoke(cf.INVOKEVIRTUAL, "java/lang/Number", "intValue", "()I")
		flags[i] = c.AllocLocal("I")
		c.Local(cf.ISTORE, flags[i])
	}
	composer = c.AllocLocal(types.ComposerDesc)
	c.Checkcast(composerClass)
	c.Local(cf.ASTORE, composer)
	return composer, flags
}

// composeInlineStart starts the body of lambda ctx.l, inlined where it is
// invoked, recording the groups open there, which its returns don't end.
// A composable one's code calls composables with the Composer in local
// composer, or with the code around it's if composer is -1, and knows its
// parameters' change flags, in locals flags, as well as those of the code
// around it, outer (see flagScope); one that returns a value runs in a
// replace group, as its method's body would. It returns the function
// that restores the code around it's.
func (g *gen) composeInlineStart(ctx *inlineLambda, composer int, flags []int, outer *flagScope) func() {
	l, saved := ctx.l, g.compose
	ctx.groups = g.groupDepth()
	if !l.Composable || saved == nil && composer < 0 {
		return func() {}
	}
	// The code around it's, but for its Composer and its parameters: a
	// return still ends the function's restart group.
	c := composeFn{composer: composer}
	if saved != nil {
		c = *saved
		if composer >= 0 {
			c.composer = composer
		}
	}
	if l.Crossinline {
		outer = nil // its parameters only, as the plugin's
	}
	c.flagScope = &flagScope{tracked: lambdaTracked(l), flags: flags, parent: outer}
	g.compose = &c
	if l.Ret.K != types.Unit || l.Ret.N {
		g.openGroup(g.lambdaKey(l))
	}
	return func() { g.compose = saved }
}

// composeInlineEnd ends the groups opened in inlined lambda ctx.l, where
// its body completes, before its result leaves it.
func (g *gen) composeInlineEnd(ctx *inlineLambda) {
	g.closeGroups(ctx.groups)
}

// composeKey generates call x of Compose's key(keys...) { ... }, as
// kotlinc's plugin does: the lambda runs in a movable group, whose key is
// the keys joined, and leaves its result.
func (g *gen) composeKey(x *syntax.Call) {
	c := g.code
	keys, l := x.Args[:len(x.Args)-1], x.Args[len(x.Args)-1].(*syntax.Lambda)
	g.loadComposer()
	c.Iconst(g.lambdaKey(l))
	for range keys[1:] {
		g.loadComposer()
	}
	for i, k := range keys {
		g.exprAs(k, types.AnyType.Nullable())
		if i > 0 {
			c.InvokeInterface(composerClass, "joinKey", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;")
		}
	}
	c.InvokeInterface(composerClass, "startMovableGroup", "(ILjava/lang/Object;)V")
	ctx := &inlineLambda{l: l, end: join(l.Ret), depth: c.Depth(), tries: len(g.tries), direct: true, groups: g.groupDepth()}
	g.inl = append(g.inl, ctx)
	if l.Ret.K == types.Unit {
		g.block(l.Body, types.UnitType, false)
	} else {
		g.block(l.Body, l.Ret, true)
	}
	g.closeGroups(ctx.groups)
	g.inl = g.inl[:len(g.inl)-1]
	c.Place(ctx.end)
	if c.Reachable() {
		g.loadComposer()
		c.InvokeInterface(composerClass, "endMovableGroup", "()V")
	}
	g.coerce(l.Ret, x.T)
}

// composeArgs pushes the arguments of a call of composable function f
// that follow its own: the Composer, the change flags of args, its
// arguments by tracked parameter (all uncertain if nil: see skip.go), and
// the masks of the omitted arguments, whose bits are in mask. The invoke
// of a composable function type takes the change flags as Integers.
func (g *gen) composeArgs(f *types.FunSym, mask argMask, args []callArg) {
	if !f.Composable {
		return
	}
	g.loadComposer()
	boxed := f.Owner != nil && f.Owner.Func
	if args != nil {
		g.changedArgs(args, f.Changed, boxed)
	} else {
		for range f.Changed {
			g.code.Iconst(0)
			if boxed {
				g.box(types.IntType)
			}
		}
	}
	for i := range f.Defaults {
		g.code.Iconst(mask.bits31(i))
	}
}

// callArgs returns the arguments of call x of composable function f by
// its tracked parameters: the extension receiver, the parameters, whose
// omitted ones' bits are in mask, and the dispatch receiver.
func (g *gen) composeCallArgs(x *syntax.Call, f *types.FunSym, mask argMask) []callArg {
	var args []callArg
	m, member := x.Fun.(*syntax.Member)
	recv := func(t types.Type) callArg {
		if member {
			return callArg{x: m.X, typ: t}
		}
		return callArg{this: !f.Ext, typ: t}
	}
	if f.Ext {
		args = append(args, recv(f.Recv))
	}
	for j, p := range f.Params {
		var as []syntax.Expr
		for i := range x.Args {
			if x.ArgMap == nil && i == j || x.ArgMap != nil && x.ArgMap[i] == j {
				as = append(as, passedArg(x, i, p.Vararg))
			}
		}
		switch {
		case mask.has(j):
			args = append(args, callArg{omitted: true})
		case p.Vararg:
			args = append(args, callArg{vararg: true, args: as, elem: types.StoredElem(p.Type)})
		case len(as) == 1:
			args = append(args, callArg{x: as[0]})
		default:
			args = append(args, callArg{omitted: true})
		}
	}
	switch {
	case f.Owner == nil || f.Static:
	case f.Ext:
		// A member extension's object, which the code doesn't name.
		args = append(args, callArg{typ: f.Owner.This()})
	default:
		args = append(args, recv(f.Owner.This()))
	}
	return args
}

// siteKey returns the key of a group of the code at p: a hash of where it
// is, of kind, what code it is, and of i, which of its groups it is.
func (g *gen) siteKey(p syntax.Pos, kind string, i int) int32 {
	h := fnv.New32a()
	h.Write([]byte(filepath.Base(g.file.Path) + ":" + strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col) + ":" + kind + strconv.Itoa(i)))
	return int32(h.Sum32())
}

// openGroup starts a replace group with key, in the code of a composable.
func (g *gen) openGroup(key int32) {
	g.loadComposer()
	g.code.Iconst(key)
	g.code.InvokeInterface(composerClass, "startReplaceGroup", "(I)V")
	g.compose.groups = append(g.compose.groups, g.compose.composer)
}

// groupDepth returns the number of replace groups open where the code
// being generated is.
func (g *gen) groupDepth() int {
	if g.compose == nil {
		return 0
	}
	return len(g.compose.groups)
}

// endGroups ends the replace groups open but the first from, innermost
// first, where a jump leaves them.
func (g *gen) endGroups(from int) {
	if g.compose == nil {
		return
	}
	for i := len(g.compose.groups) - 1; i >= from; i-- {
		g.code.Local(cf.ALOAD, g.compose.groups[i])
		g.code.InvokeInterface(composerClass, "endReplaceGroup", "()V")
	}
}

// closeGroups ends the replace groups open but the first from, where the
// code reaches their end.
func (g *gen) closeGroups(from int) {
	g.endGroups(from)
	if g.compose != nil {
		g.compose.groups = g.compose.groups[:from]
	}
}

// grouped generates body in a replace group with key, if on, as composeFlow
// says.
func (g *gen) grouped(on bool, key int32, body func()) {
	if !on {
		body()
		return
	}
	from := g.groupDepth()
	g.openGroup(key)
	body()
	g.closeGroups(from)
}

// loadComposer pushes the Composer of the composable function being
// generated.
func (g *gen) loadComposer() {
	if g.compose == nil {
		panic("a composable call outside a composable function")
	}
	g.code.Local(cf.ALOAD, g.compose.composer)
}

// composeGetterDesc returns the descriptor of the getter of property p,
// whose descriptor is d if it isn't composable.
func composeGetterDesc(p *types.VarSym, d string) string {
	if !p.Composable {
		return d
	}
	return types.ComposeDesc(d, 1)
}

// stableField adds the $stable field that kotlinc's Compose plugin gives
// a public or internal class that isn't local, inner, a companion, an
// enum, an annotation, an interface or a value class, with its
// @StabilityInferred annotation: see stability.go. The field holds the
// class's stability bits: 0 for a stable class, which it has from a
// stability marker or is inferred, and else 8 or the value of the $stable
// fields it depends on, which the static initializer assigns (see
// stableInit). The annotation's parameters are the mask of the type
// parameters its stability depends on, and the bit after them if it is
// known to be stable.
func (g *gen) stableField(c *cf.Class, cd *syntax.ClassDecl) {
	cls := cd.Sym
	if !g.shared.opts.Compose || cls.Interface || cls.Annotation || cls.Entries != nil || cd.EnumEntry || cls.CompanionOf != nil ||
		cls.Local || cls.Anonymous || cls.Inner || cls.Value || cd.Visibility == syntax.Private || cd.Visibility == syntax.Protected {
		return
	}
	c.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal, "$stable", "I")
	c.LastField().Annotate(cf.Annotation{Type: "Lkotlin/jvm/JvmField;"}, false)
	in := &stabInfer{g: g, analyzing: map[*types.ClassSym]bool{}}
	if in.marked(cls) {
		return
	}
	s := in.ofType(cls.This(), nil).normalize()
	mask, external := 0, false
	if n := len(cls.TypeParams); n > 0 {
		s.each(func(e stability) {
			if e.kind != stabParam {
				return
			}
			if i := slices.Index(cls.TypeParams, e.param); i >= 0 {
				mask |= 1 << i
			} else {
				external = true
			}
		})
		if s.knownStable() && n < 32 {
			mask |= 1 << n
		}
	} else if s.knownStable() {
		mask = 1
	}
	c.Annotate(cf.Annotation{Type: "Landroidx/compose/runtime/internal/StabilityInferred;", Values: []cf.ElementPair{
		{Name: "parameters", Value: cf.ElementValue{Tag: 'I', Const: int64(mask)}}}}, false)
	// A type parameter's argument is stable, as far as the class's own
	// $stable says: the callers that know it add its bits.
	s = s.withoutParams()
	switch {
	case external || !stableExprOK(s, nil):
		g.shared.stableInits[cd] = func() { g.code.Iconst(8) }
	case hasRuntime(s):
		g.shared.stableInits[cd] = func() { g.stableExpr(s, nil) }
	}
}

// stableInit assigns the $stable field of class cd in its static
// initializer, if its value isn't 0, and reports whether it did.
func (g *gen) stableInit(cd *syntax.ClassDecl) bool {
	init := g.shared.stableInits[cd]
	if init == nil {
		return false
	}
	init()
	g.code.FieldInsn(cf.PUTSTATIC, cd.Sym.Name, "$stable", "I")
	return true
}

// invokeArgs returns the arguments of call x of a value of a function
// type by the tracked parameters of its invoke: the arguments, then the
// value, a lambda's own object.
func (g *gen) invokeArgs(x *syntax.Call) []callArg {
	var args []callArg
	for _, a := range x.Args {
		args = append(args, callArg{x: a})
	}
	return append(args, callArg{x: x.Fun})
}

// Open composables' default values.
//
// An open composable member with default values (see
// types.FunSym.ComposeDefaults) takes no masks: as kotlinc's plugin does,
// kc generates a static name$default method in its class's
// ComposeDefaultImpls class, a composable function of the member's
// parameters, then the object and the extension receiver, whose body
// computes the omitted arguments and calls the member, and calls of the
// member and its overrides call it, with their masks.

// composeDefaultsMask is what callArgs returns for a call of name$default,
// whose arguments it pushed, for invokeDefault to call it: not empty.
var composeDefaultsMask = argMask{1}

// composeDefaultsOf returns the function whose name$default method a call
// of f calls, or nil: f, or the function whose default values it
// inherits.
func composeDefaultsOf(f *types.FunSym) *types.FunSym {
	if f.DefaultsOf != nil {
		f = f.DefaultsOf
	}
	if f.ComposeDefaults {
		return f
	}
	return nil
}

// composeDefaultsDesc returns the descriptor of f's name$default method:
// f's parameters, its class, its extension receiver, the Composer, the
// change flags and the masks.
func composeDefaultsDesc(f *types.FunSym) string {
	d := funDesc(f)
	ps := descParams(d)
	recv := ""
	if f.Ext {
		recv, ps = ps[0], ps[1:]
	}
	n := len(f.Params)
	return "(" + strings.Join(ps[:n], "") + "L" + f.Owner.Name + ";" + recv + types.ComposerDesc +
		strings.Repeat("I", f.Changed+types.DefaultInts(n)) + d[strings.IndexByte(d, ')'):]
}

// composeDefaultsArgs pushes the arguments of call x of f, whose
// name$default method df's is, below which the object and f's extension
// receiver are: the arguments, the object, the receiver, the Composer,
// the change flags and the masks.
func (g *gen) composeDefaultsArgs(x *syntax.Call, f, df *types.FunSym) {
	ps := descParams(composeDefaultsDesc(df))
	n := len(f.Params)
	recv := -1
	if f.Ext {
		recv = g.code.AllocLocal(ps[n+1])
		g.code.Local(storeOpOf(ps[n+1]), recv)
	}
	self := g.code.AllocLocal(ps[n])
	g.code.Local(cf.ASTORE, self)
	mask := g.valueArgs(x, f)
	g.code.Local(cf.ALOAD, self)
	g.code.Retire(self)
	if recv >= 0 {
		g.code.Local(loadOpOf(ps[n+1]), recv)
		g.code.Retire(recv)
	}
	// Its tracked parameters: the parameters, the object, the receiver.
	args := g.composeCallArgs(x, f, mask)
	if f.Ext {
		args = append(args[1:], args[0])
	}
	g.loadComposer()
	g.changedArgs(args, f.Changed, false)
	for i := range types.DefaultInts(n) {
		g.code.Iconst(mask.bits31(i))
	}
}

// composeImpls returns the ComposeDefaultImpls class of the class being
// generated, begun by an earlier call or new.
func (g *gen) composeImpls() *cf.Class {
	name := g.cls.Name + "$ComposeDefaultImpls"
	if c := g.shared.classes[name]; c != nil {
		return c
	}
	c := cf.NewClass(cf.AccPublic|cf.AccFinal|cf.AccSuper, name, "java/lang/Object")
	c.SourceFile = g.cls.SourceFile
	g.shared.classes[name] = c
	return c
}

// composeDefaults generates the name$default method of fd, an open
// composable member with default values.
func (g *gen) composeDefaults(fd *syntax.FunDecl) {
	f := fd.Sym
	cls, code, self, prefix := g.cls, g.code, g.self, g.prefix
	params := f.Params
	if fd.RecvSym != nil {
		params = append(params[:len(params):len(params)], fd.RecvSym)
	}
	slots := make([]int, len(params))
	for i, p := range params {
		slots[i] = p.Slot
	}
	defer func() {
		g.cls, g.code, g.self, g.prefix = cls, code, self, prefix
		for i, p := range params {
			p.Slot = slots[i]
		}
	}()
	d := composeDefaultsDesc(f)
	g.cls = g.composeImpls()
	g.code = g.cls.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccFinal, f.JVM()+"$default", d)
	g.cls.LastMethod().Annotate(cf.Annotation{Type: "Landroidx/compose/runtime/Composable;"}, false)
	g.prefix = f.Name + "_default$lambda"
	g.loops, g.tries = nil, nil
	for _, p := range f.Params {
		p.Slot = g.code.AllocLocal(paramDesc(p.Type))
		g.named(p)
	}
	g.self = g.code.AllocLocal("L" + f.Owner.Name + ";")
	g.code.NameLocal(g.self, "$this")
	if r := fd.RecvSym; r != nil {
		r.Slot = g.code.AllocLocal(paramDesc(r.Type))
	}
	ds := *f
	ds.Defaults, ds.ComposeDefaults = types.DefaultInts(len(f.Params)), false
	outer := g.compose
	defer func() { g.compose = outer }()
	g.compose = g.composeParams(&ds, false)
	g.compose.impl = d
	for i, p := range f.Params {
		if fd.Params[i].Default == nil {
			g.checkParam(p.Slot, p.Type, p.Name)
		}
	}
	g.checkParam(g.self, f.Owner.This(), "$this$")
	if r := fd.RecvSym; r != nil {
		g.checkParam(r.Slot, r.Type, "$this$")
	}
	// Its tracked parameters: the parameters, the object, the receiver,
	// all used.
	t := &tracked{}
	for j, p := range fd.Params {
		def := -1
		if p.Default != nil {
			def = j
		}
		typ := p.Sym.Type
		if p.Vararg {
			typ = types.StoredElem(typ)
		}
		t.params = append(t.params, p.Sym)
		t.types = append(t.types, typ)
		t.vararg = append(t.vararg, p.Vararg)
		t.defaults = append(t.defaults, def)
	}
	t.params = append(t.params, nil)
	t.types = append(t.types, f.Owner.This())
	t.vararg = append(t.vararg, false)
	t.defaults = append(t.defaults, -1)
	if r := fd.RecvSym; r != nil {
		t.params = append(t.params, r)
		t.types = append(t.types, r.Type)
		t.vararg = append(t.vararg, false)
		t.defaults = append(t.defaults, -1)
	}
	t.count = len(t.params)
	t.nonStatic = make([]bool, t.count)
	t.used = make([]bool, t.count)
	for i := range t.used {
		t.used[i] = true
	}
	g.composeBegin(fd, t, g.siteKey(fd.P, "default", 0), true)
	// The call: the object, the receiver, the arguments.
	g.loadThis()
	var args []callArg
	if r := fd.RecvSym; r != nil {
		g.code.Local(loadOp(r.Type), r.Slot)
		args = append(args, callArg{x: paramIdent(r)})
	}
	for _, p := range f.Params {
		g.code.Local(loadOp(p.Type), p.Slot)
		args = append(args, callArg{x: paramIdent(p)})
	}
	args = append(args, callArg{this: true, typ: f.Owner.This()})
	g.loadComposer()
	g.changedArgs(args, f.Changed, false)
	g.callMethod(f, f.Owner, f.JVM(), funDesc(f))
	if g.compose.end == nil {
		g.ret(types.JVMRet(f))
	}
	g.composeEnd(fd)
	g.check()
	if c := g.compose; c.restart != nil {
		c.restart()
	}
}

// composeDefaultsStub generates the method that kotlinc's plugin keeps,
// hidden, for code compiled when open composables took masks: the
// function's method that takes them too, and calls name$default.
func (g *gen) composeDefaultsStub(fd *syntax.FunDecl) {
	f := fd.Sym
	d := funDesc(f)
	i := strings.IndexByte(d, ')')
	n := types.DefaultInts(len(f.Params))
	stub := d[:i] + strings.Repeat("I", n) + d[i:]
	code := g.cls.NewMethod(cf.AccPublic|cf.AccSynthetic, f.JVM(), stub)
	m := g.cls.LastMethod()
	m.Deprecate()
	m.Annotate(cf.Annotation{Type: "Lkotlin/Deprecated;", Values: []cf.ElementPair{
		{Name: "message", Value: cf.ElementValue{Tag: 's', Const: "Binary compatibility stub for default parameters"}},
		{Name: "level", Value: cf.ElementValue{Tag: 'e', Enum: [2]string{"Lkotlin/DeprecationLevel;", "HIDDEN"}}},
	}}, true)
	m.Annotate(cf.Annotation{Type: "Landroidx/compose/runtime/Composable;"}, false)
	self := code.AllocLocal("L" + g.cls.Name + ";")
	ps := descParams(stub)
	slots := make([]int, len(ps))
	for j, p := range ps {
		slots[j] = code.AllocLocal(p)
	}
	recv := 0
	if f.Ext {
		recv = 1
	}
	for j := recv; j < recv+len(f.Params); j++ {
		code.Local(loadOpOf(ps[j]), slots[j])
	}
	code.Local(cf.ALOAD, self)
	if f.Ext {
		code.Local(loadOpOf(ps[0]), slots[0])
	}
	for j := recv + len(f.Params); j < len(ps); j++ {
		code.Local(loadOpOf(ps[j]), slots[j])
	}
	code.Invoke(cf.INVOKESTATIC, g.cls.Name+"$ComposeDefaultImpls", f.JVM()+"$default", composeDefaultsDesc(f))
	code.Op(returnOpOf(d[i+1:]))
	if err := code.Err(); err != nil {
		panic(fmt.Errorf("%s.%s: %w", g.cls.Name, f.JVM(), err))
	}
}

// paramIdent returns a reference to parameter p, for what the code knows
// of it as an argument.
func paramIdent(p *types.VarSym) *syntax.Ident {
	x := &syntax.Ident{Name: p.Name, Sym: p}
	x.T = p.Type
	return x
}

// descSize returns the number of locals a value of descriptor d takes.
func descSize(d string) int {
	if d == "J" || d == "D" {
		return 2
	}
	return 1
}
