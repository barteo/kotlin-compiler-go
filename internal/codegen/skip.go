package codegen

import (
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Skipping.
//
// As kotlinc's Compose plugin with strong skipping, its default, kc
// compiles a restartable composable function, and a composable lambda
// that returns Unit and whose parameters' types aren't known to be
// unstable, to skip its body when it is called with the arguments it got
// last time. Its change flags, $changed, say for each of its tracked
// parameters, the extension receiver, the parameters and then the
// dispatch receiver (a lambda's own object for a lambda), in 3 bits after
// the first, the force bit, what the caller knows of the argument: same as
// last time, different, static (never changing) or uncertain, and with
// the third bit, whether its value is unstable. The function copies them
// to $dirty, and asks the Composer about the uncertain arguments of the
// parameters it uses:
//
//	if ($changed & 0b011<<off) == 0 {
//		$dirty |= if ($composer.changed(p)) 0b010<<off else 0b001<<off
//	}
//
// changed compares a stable argument by equals, another one by identity
// (changedInstance), and one of uncertain stability as the stability bit
// says. Then
//
//	if ($composer.shouldExecute(($dirty & used) != same, $dirty & 1)) {
//		(the default values of the omitted arguments)
//		(the body)
//	} else {
//		$composer.skipToGroupEnd()
//	}
//
// where used has the low bit of each used parameter's state, and same
// is what it is if all are the same. Default values that aren't static,
// such as calls, are computed in a group of their own, between
// startDefaults and endDefaults, which a recomposition that the scope
// starts skips, keeping the values it was called with.
//
// A caller passes the change flags of each argument: static for a
// constant, a remembered composable lambda or other static values, the
// flags of its own parameter for one passed on, and the stability bits of
// a class whose stability is known at run time, from its $stable field
// (see stability.go); a vararg's or an omitted argument's are uncertain.
//
// A lambda in composable code that isn't inlined or composable, or
// annotated @DontMemoize, is remembered, with what it captures as keys,
// so that composables it is passed to can skip:
//
//	val invalid = $composer.changed(capture) || ...
//	val v = $composer.rememberedValue()
//	if (invalid || v === Composer.Empty) {
//		v = (the lambda)
//		$composer.updateRememberedValue(v)
//	}
//
// as kotlinc's intrinsic remember does, which tells from the flags of a
// captured parameter whether it changed.

// Parameter states: the 3 bits of a tracked parameter, after the force
// bit.
const (
	paramSame      = 0b001
	paramDifferent = 0b010
	paramStatic    = 0b011
	paramMask      = 0b111
	paramUnstable  = 0b100 // the stability bit
)

// slotBits returns bits for slot i, in its change flags int.
func slotBits(bits, i int) int32 { return int32(bits) << (1 + 3*(i%10)) }

// tracked are the tracked parameters of a composable function or lambda,
// whose arguments' change flags it gets, in the order of their slots.
type tracked struct {
	params []*types.VarSym // nil for the dispatch receiver, this
	types  []types.Type    // their types: a vararg parameter's elements'
	vararg []bool
	// defaults are the indexes of the parameters with default values in
	// the masks of the omitted arguments, -1 for others.
	defaults []int
	used     []bool // read by the code: only those can change its result
	count    int    // the slots: a lambda's has its own object last
	// nonStatic are set on the parameters whose default values aren't
	// static: their flags don't tell whether a value computed again
	// changed.
	nonStatic []bool
}

// flagScope is code that knows the change flags of tracked parameters:
// the body of a composable function or lambda, of a copy of the program's
// inline composable, whose flags are those its call passes, or of a
// composable lambda inlined where an inline function invokes it, with
// those the invoke passes. The code of an inlined lambda reads the
// parameters of the code around it too, its parent's, as the plugin's
// does, and that of a function with type parameters knows its type
// arguments' stability bits, from the flags of parameters of those types.
type flagScope struct {
	tracked    *tracked // nil if it has none
	typeParams []*types.TypeParamSym
	flags      []int // the locals of the flags: $dirty if dirtyVar, else $changed
	dirtyVar   bool
	parent     *flagScope
}

// scope returns the flag scope of the code being generated, or nil.
func (g *gen) scope() *flagScope {
	if g.compose == nil {
		return nil
	}
	return g.compose.flagScope
}

// inScope switches the code being generated to flag scope s, which
// returns switch back.
func (g *gen) inScope(s *flagScope) func() {
	saved := g.compose
	if saved == nil {
		return func() {}
	}
	c := *saved
	c.flagScope = s
	g.compose = &c
	return func() { g.compose = saved }
}

// funTracked returns the tracked parameters of composable function fd,
// and which of them its code uses.
func funTracked(fd *syntax.FunDecl, member bool) *tracked {
	t := &tracked{}
	add := func(v *types.VarSym, typ types.Type, vararg bool, def int) {
		t.params = append(t.params, v)
		t.types = append(t.types, typ)
		t.vararg = append(t.vararg, vararg)
		t.defaults = append(t.defaults, def)
	}
	if r := fd.RecvSym; r != nil {
		add(r, r.Type, false, -1)
	}
	for j, p := range fd.Params {
		typ, def := p.Sym.Type, -1
		if p.Vararg {
			typ = types.StoredElem(typ)
		}
		if p.Default != nil && !fd.Sym.ComposeDefaults {
			def = j
		}
		add(p.Sym, typ, p.Vararg, def)
	}
	if member {
		add(nil, fd.Sym.Owner.This(), false, -1)
	}
	t.count = len(t.params)
	t.nonStatic = make([]bool, t.count)
	var roots []any
	for _, p := range fd.Params {
		if !fd.Sym.ComposeDefaults {
			roots = append(roots, p.Default)
		}
	}
	t.used = usedParams(t.params, append(roots, fd.Body, fd.ExprBody)...)
	return t
}

// lambdaTracked returns the tracked parameters of composable lambda x:
// its receiver and parameters, and then its own object, which is used.
func lambdaTracked(x *syntax.Lambda) *tracked {
	t := &tracked{}
	for _, p := range x.ParamSyms() {
		t.params = append(t.params, p)
		t.types = append(t.types, p.Type)
		t.vararg = append(t.vararg, false)
		t.defaults = append(t.defaults, -1)
	}
	t.count = len(t.params) + 1
	t.nonStatic = make([]bool, t.count)
	t.used = append(usedParams(t.params, x.Body), true)
	return t
}

// usedParams reports which of params the code in roots uses: nil stands
// for the dispatch receiver, this, which members of the class use too.
func usedParams(params []*types.VarSym, roots ...any) []bool {
	used := make([]bool, len(params))
	index := map[*types.VarSym]int{}
	this := -1
	for i, p := range params {
		if p == nil {
			this = i
		} else {
			index[p] = i
		}
	}
	mark := func(v *types.VarSym) {
		if i, ok := index[v]; ok {
			used[i] = true
		}
	}
	for _, r := range roots {
		syntax.Inspect(r, func(n any) bool {
			switch n := n.(type) {
			case *syntax.Ident:
				mark(n.Sym)
				if this >= 0 && implicitThis(n) {
					used[this] = true
				}
			case *syntax.This:
				if n.Recv != nil {
					mark(n.Recv)
				} else if this >= 0 {
					used[this] = true
				}
			case *syntax.Super:
				if this >= 0 && n.Outer == nil {
					used[this] = true
				}
			case *syntax.Call:
				if f := n.Sym; this >= 0 && f != nil && f.Owner != nil && !f.Owner.Object && !f.Static && !n.Invoke && n.Ctor == nil {
					if _, ok := n.Fun.(*syntax.Ident); ok {
						used[this] = true // a member called on this
					}
				}
			case *syntax.Lambda:
				for _, c := range n.Captures {
					mark(c)
				}
				if n.This && this >= 0 {
					used[this] = true
				}
			case *syntax.CallableRef:
				mark(n.Bound)
			}
			return true
		})
	}
	return used
}

// implicitThis reports whether x reads a member of this.
func implicitThis(x *syntax.Ident) bool {
	if x.Getter != nil || x.Member != nil {
		return true
	}
	v := x.Sym
	return v != nil && v.Kind == types.Prop && !v.Const && !v.StaticField() && v.Owner != nil && !v.Owner.Object
}

// skipPrologue starts the code of a composable function, or lambda, that
// can skip its body: it computes $dirty from the change flags in c, and
// asks the Composer whether to execute the body, and jumps to c.skip if
// not. In a function, the default values of omitted arguments follow,
// with defaults the declarations of its parameters.
func (g *gen) skipPrologue(c *composeFn, t *tracked, params []*syntax.Param) {
	code := g.code
	c.dirtyVar = true
	c.flags = nil
	for i, ch := range c.changed {
		d := code.AllocLocal("I")
		code.NameLocal(d, "$dirty"+suffix(i))
		code.Local(cf.ILOAD, ch)
		code.Local(cf.ISTORE, d)
		c.flags = append(c.flags, d)
	}
	statics := make([]bool, t.count)
	for i := range t.params {
		statics[i] = t.defaults[i] >= 0 && !t.nonStatic[i]
	}
	for i := range t.params {
		if t.vararg[i] || !t.used[i] {
			continue
		}
		g.dirtyParam(c, t, i, statics[i])
	}
	for i := range t.params {
		if t.vararg[i] {
			g.dirtyVararg(c, t, i)
		}
	}
	c.skip = cf.NewLabel()
	if g.shared.opts.ShouldExecute {
		g.loadComposer()
		g.hasDifferences(c, t)
		code.Local(cf.ILOAD, c.flags[0])
		code.Iconst(1)
		code.Op(cf.IAND)
		code.InvokeInterface(composerClass, "shouldExecute", "(ZI)Z")
		code.Jump(cf.IFEQ, c.skip)
	} else {
		// An older runtime's: unless something changed, skip if the
		// Composer is skipping.
		run := cf.NewLabel()
		g.hasDifferences(c, t)
		code.Jump(cf.IFNE, run)
		g.loadComposer()
		code.InvokeInterface(composerClass, "getSkipping", "()Z")
		code.Jump(cf.IFNE, c.skip)
		code.Place(run)
	}
	if params != nil {
		g.skipDefaults(c, t, params, statics)
	}
}

// dirtyParam sets the state of tracked parameter i in $dirty, if its
// caller didn't say: whether the Composer finds the argument changed.
// static is set if its default value is static, which an omitted
// argument then is.
func (g *gen) dirtyParam(c *composeFn, t *tracked, i int, static bool) {
	code := g.code
	d, ch := c.flags[i/10], c.changed[i/10]
	done := cf.NewLabel()
	def := t.defaults[i]
	if def >= 0 && static {
		// if ($default & bit) $dirty |= static
		provided := cf.NewLabel()
		g.defaultBit(c, def)
		code.Jump(cf.IFEQ, provided)
		code.Local(cf.ILOAD, d)
		code.Iconst(slotBits(paramStatic, i))
		code.Op(cf.IOR)
		code.Local(cf.ISTORE, d)
		code.Jump(cf.GOTO, done)
		code.Place(provided)
	}
	code.Local(cf.ILOAD, ch)
	code.Iconst(slotBits(paramStatic, i))
	code.Op(cf.IAND)
	code.Jump(cf.IFNE, done)
	code.Local(cf.ILOAD, d)
	same, or := cf.NewLabel(), cf.NewLabel()
	if def >= 0 && !static {
		// An omitted argument is the same: its default value is
		// computed again.
		g.defaultBit(c, def)
		code.Jump(cf.IFNE, same)
	}
	g.paramChanged(c, t, i, func() { g.loadTracked(t, i) })
	code.Jump(cf.IFEQ, same)
	code.Iconst(slotBits(paramDifferent, i))
	code.Jump(cf.GOTO, or)
	code.Place(same)
	code.Iconst(slotBits(paramSame, i))
	code.Place(or)
	code.Op(cf.IOR)
	code.Local(cf.ISTORE, d)
	code.Place(done)
}

// paramChanged pushes whether the Composer finds the argument of tracked
// parameter i, which load pushes, changed: for a parameter of uncertain
// stability, as its stability bit in $changed says.
func (g *gen) paramChanged(c *composeFn, t *tracked, i int, load func()) {
	typ := t.types[i]
	s := g.stabilityOf(typ)
	if !s.uncertain() {
		g.changed(typ, s, load, changedParam)
		return
	}
	code := g.code
	unstable, done := cf.NewLabel(), cf.NewLabel()
	code.Local(cf.ILOAD, c.changed[i/10])
	code.Iconst(slotBits(paramUnstable, i))
	code.Op(cf.IAND)
	code.Jump(cf.IFNE, unstable)
	g.changed(typ, s, load, changedStable)
	code.Jump(cf.GOTO, done)
	code.Place(unstable)
	g.changed(typ, s, load, changedParam)
	code.Place(done)
}

// dirtyVararg sets the state of vararg parameter i in $dirty from its
// elements: in a movable group whose key is their number, whether the
// Composer finds the number, or an element, changed.
func (g *gen) dirtyVararg(c *composeFn, t *tracked, i int) {
	code := g.code
	d := c.flags[i/10]
	skip := cf.NewLabel()
	if def := t.defaults[i]; def >= 0 {
		g.defaultBit(c, def)
		code.Jump(cf.IFNE, skip)
	}
	arr := t.params[i]
	g.loadComposer()
	code.Iconst(g.siteKey(syntax.Pos{}, "vararg$"+arr.Name, i))
	code.Local(cf.ALOAD, arr.Slot)
	code.Op(cf.ARRAYLENGTH)
	g.box(types.IntType)
	code.InvokeInterface(composerClass, "startMovableGroup", "(ILjava/lang/Object;)V")
	orDifferent := func(changed func()) {
		zero, or := cf.NewLabel(), cf.NewLabel()
		code.Local(cf.ILOAD, d)
		changed()
		code.Jump(cf.IFEQ, zero)
		code.Iconst(slotBits(paramDifferent, i))
		code.Jump(cf.GOTO, or)
		code.Place(zero)
		code.Iconst(0)
		code.Place(or)
		code.Op(cf.IOR)
		code.Local(cf.ISTORE, d)
	}
	orDifferent(func() {
		g.loadComposer()
		code.Local(cf.ALOAD, arr.Slot)
		code.Op(cf.ARRAYLENGTH)
		code.InvokeInterface(composerClass, "changed", "(I)Z")
	})
	ed := desc(types.StoredElem(arr.Type))
	idx := code.AllocLocal("I")
	code.Iconst(0)
	code.Local(cf.ISTORE, idx)
	top, end := cf.NewLabel(), cf.NewLabel()
	code.Place(top)
	code.Local(cf.ILOAD, idx)
	code.Local(cf.ALOAD, arr.Slot)
	code.Op(cf.ARRAYLENGTH)
	code.Jump(cf.IF_ICMPGE, end)
	orDifferent(func() {
		g.paramChanged(c, t, i, func() {
			code.Local(cf.ALOAD, arr.Slot)
			code.Local(cf.ILOAD, idx)
			g.elemLoad("[" + ed)
		})
	})
	code.Iinc(idx, 1)
	code.Jump(cf.GOTO, top)
	code.Place(end)
	code.Retire(idx)
	g.loadComposer()
	code.InvokeInterface(composerClass, "endMovableGroup", "()V")
	// No element changed: the same.
	code.Local(cf.ILOAD, d)
	code.Iconst(slotBits(paramMask, i))
	code.Op(cf.IAND)
	code.Jump(cf.IFNE, skip)
	code.Local(cf.ILOAD, d)
	code.Iconst(slotBits(paramSame, i))
	code.Op(cf.IOR)
	code.Local(cf.ISTORE, d)
	code.Place(skip)
}

// defaultBit pushes the bit of the mask of omitted arguments of the
// function's parameter j: nonzero if the argument was omitted.
func (g *gen) defaultBit(c *composeFn, j int) {
	g.code.Local(cf.ILOAD, c.defaults[j/31])
	g.code.Iconst(int32(1) << (j % 31))
	g.code.Op(cf.IAND)
}

// hasDifferences pushes whether a used tracked parameter's argument
// changed, or the force bit is set: for each change flags int, $dirty
// masked with the low bits of the used parameters' states, and the force
// bit, differs from all same.
func (g *gen) hasDifferences(c *composeFn, t *tracked) {
	code := g.code
	if t.count == 0 {
		code.Local(cf.ILOAD, c.flags[0])
		g.boolOf(cf.IFNE)
		return
	}
	yes, done := cf.NewLabel(), cf.NewLabel()
	n := len(c.flags)
	for k := range n {
		var mask int32
		for i := 10 * k; i < min(10*k+10, t.count); i++ {
			if t.used[i] {
				mask |= slotBits(paramSame, i)
			}
		}
		code.Local(cf.ILOAD, c.flags[k])
		code.Iconst(mask | 1)
		code.Op(cf.IAND)
		if mask == 0 {
			if k < n-1 {
				code.Jump(cf.IFNE, yes)
				continue
			}
			g.boolOf(cf.IFNE)
		} else {
			code.Iconst(mask)
			if k < n-1 {
				code.Jump(cf.IF_ICMPNE, yes)
				continue
			}
			g.boolOf(cf.IF_ICMPNE)
		}
	}
	if n > 1 {
		code.Jump(cf.GOTO, done)
		code.Place(yes)
		code.Iconst(1)
		code.Place(done)
	}
}

// boolOf pushes 1 if jump op would jump, else 0.
func (g *gen) boolOf(op byte) {
	yes, done := cf.NewLabel(), cf.NewLabel()
	g.code.Jump(op, yes)
	g.code.Iconst(0)
	g.code.Jump(cf.GOTO, done)
	g.code.Place(yes)
	g.code.Iconst(1)
	g.code.Place(done)
}

// skipDefaults computes the default values of a skippable function's
// omitted arguments, after shouldExecute: in a defaults group if some
// aren't static, which a recomposition that the restart scope starts
// skips, unless the Composer says they are invalid. A non-static default
// value's state is uncertain.
func (g *gen) skipDefaults(c *composeFn, t *tracked, params []*syntax.Param, statics []bool) {
	code := g.code
	all := true
	any := false
	for i := range t.params {
		if t.defaults[i] >= 0 {
			any = true
			all = all && statics[i]
		}
	}
	if !any {
		return
	}
	assign := func(uncertain bool) {
		for i, p := range t.params {
			def := t.defaults[i]
			if def < 0 || uncertain && statics[i] {
				continue
			}
			skip := cf.NewLabel()
			g.defaultBit(c, def)
			code.Jump(cf.IFEQ, skip)
			if !uncertain {
				g.exprAs(params[def].Default, p.Type)
				code.Local(storeOp(p.Type), p.Slot)
			}
			if !statics[i] {
				d := c.flags[i/10]
				code.Local(cf.ILOAD, d)
				code.Iconst(^slotBits(paramMask, i))
				code.Op(cf.IAND)
				code.Local(cf.ISTORE, d)
			}
			code.Place(skip)
		}
	}
	if all {
		assign(false)
		return
	}
	g.loadComposer()
	code.InvokeInterface(composerClass, "startDefaults", "()V")
	compute, done := cf.NewLabel(), cf.NewLabel()
	code.Local(cf.ILOAD, c.changed[0])
	code.Iconst(1)
	code.Op(cf.IAND)
	code.Jump(cf.IFEQ, compute)
	g.loadComposer()
	code.InvokeInterface(composerClass, "getDefaultsInvalid", "()Z")
	code.Jump(cf.IFNE, compute)
	g.loadComposer()
	code.InvokeInterface(composerClass, "skipToGroupEnd", "()V")
	assign(true)
	code.Jump(cf.GOTO, done)
	code.Place(compute)
	assign(false)
	code.Place(done)
	g.loadComposer()
	code.InvokeInterface(composerClass, "endDefaults", "()V")
}

// skipEpilogue ends the body of a function or lambda whose prologue jumps
// to c.skip: the Composer skips to the end of the group.
func (g *gen) skipEpilogue(c *composeFn) {
	g.code.Place(c.skip)
	g.loadComposer()
	g.code.InvokeInterface(composerClass, "skipToGroupEnd", "()V")
}

// loadTracked pushes tracked parameter i.
func (g *gen) loadTracked(t *tracked, i int) {
	if p := t.params[i]; p != nil {
		g.code.Local(loadOp(p.Type), p.Slot)
	} else {
		g.loadThis()
	}
}

// How changed compares a value: as the plugin's irChanged.
const (
	changedParam  = iota // a parameter's argument: instances unless known stable
	changedStable        // one whose stability bit says stable: by equals
	changedKey           // a remembered lambda's capture: as its stability
)

// changed pushes whether the Composer finds value, of type t and
// stability s, which load pushes, changed since it last compared it at
// this point: by its primitive value, an enum's by ordinal, by equals or
// by identity (changedInstance), as mode says.
func (g *gen) changed(t types.Type, s stability, load func(), mode int) {
	code := g.code
	g.loadComposer()
	load()
	r := types.Repr(t)
	if r.IsPrimitive() {
		d := desc(r)
		if d == "B" || d == "S" || d == "C" || d == "Z" || d == "I" || d == "J" || d == "F" || d == "D" {
			code.InvokeInterface(composerClass, "changed", "("+d+")Z")
			return
		}
	}
	if !t.N && t.K == types.Class && enumClass(t.C) {
		code.Invoke(cf.INVOKEVIRTUAL, "java/lang/Enum", "ordinal", "()I")
		code.InvokeInterface(composerClass, "changed", "(I)Z")
		return
	}
	if r.IsPrimitive() {
		g.box(r)
	}
	fun := t.K == types.Class && (t.C.Func && !t.C.Suspend || t.C.Composable)
	instance := false
	switch mode {
	case changedParam:
		instance = fun || !s.knownStable()
	case changedStable:
		instance = fun
	case changedKey:
		instance = !s.knownStable()
	}
	if instance {
		code.InvokeInterface(composerClass, "changedInstance", "(Ljava/lang/Object;)Z")
	} else {
		code.InvokeInterface(composerClass, "changed", "(Ljava/lang/Object;)Z")
	}
}

// callArg is the argument of a call of a composable for one of its
// tracked parameters.
type callArg struct {
	x       syntax.Expr // nil for this, implicit, or an omitted argument
	this    bool        // the implicit this
	typ     types.Type  // the parameter's type, for an implicit receiver
	omitted bool
	vararg  bool
	args    []syntax.Expr // a vararg parameter's
	elem    types.Type    // and their type
}

// argMeta is what a call knows of an argument of a composable: the
// plugin's CallArgumentMeta.
type argMeta struct {
	stab      stability
	static    bool
	uncertain bool // a vararg's or an omitted argument's
	// slot is the tracked parameter of the code it is, of flag scope
	// scope, or -1.
	slot  int
	scope *flagScope
}

// argMeta returns what the code knows of argument a.
func (g *gen) argMeta(a callArg) argMeta {
	m := argMeta{stab: unstableStab, slot: -1}
	switch {
	case a.omitted:
		m.stab, m.uncertain = stableStab, true
	case a.vararg:
		m.uncertain = true
		if a.args != nil {
			m.stab = g.stabilityOf(a.elem)
		}
	case a.this:
		m.stab = g.stabilityOf(a.typ)
		m.scope, m.slot = g.trackedParam(nil)
	case a.x == nil:
		m.stab = g.stabilityOf(a.typ)
	default:
		g.exprMeta(a.x, &m)
	}
	return m
}

// exprMeta sets m from expression x, an argument: the plugin's
// populateArgumentMeta.
func (g *gen) exprMeta(x syntax.Expr, m *argMeta) {
	in := &stabInfer{g: g, analyzing: map[*types.ClassSym]bool{}}
	m.stab = in.ofExpr(x)
	if g.isStatic(x) {
		m.static = true
		return
	}
	switch x := x.(type) {
	case *syntax.Ident:
		v := x.Sym
		if v == nil || x.Member != nil || x.Getter != nil {
			return
		}
		switch v.Kind {
		case types.Param:
			m.scope, m.slot = g.trackedParam(v)
		case types.Local:
			if vs, ok := v.Decl.(*syntax.VarStmt); ok && !v.Mutable && vs.Init != nil && vs.Delegate == nil {
				g.exprMeta(vs.Init, m)
			}
		}
	case *syntax.This:
		m.scope, m.slot = g.trackedParam(x.Recv)
	case *syntax.As:
		if !x.Safe {
			g.exprMeta(x.X, m)
			m.stab = in.ofExpr(x)
		}
	}
}

// trackedParam returns the flag scope of the code being generated, or
// around it, whose tracked parameter v (nil: this) is, and its slot, or
// -1.
func (g *gen) trackedParam(v *types.VarSym) (*flagScope, int) {
	for s := g.scope(); s != nil; s = s.parent {
		if s.tracked == nil {
			continue
		}
		for i, p := range s.tracked.params {
			if p == v {
				return s, i
			}
		}
	}
	return nil, -1
}

// changedArgs pushes the change flags of the arguments of a call, args by
// the callee's tracked parameters, which a lambda's invoke takes boxed.
func (g *gen) changedArgs(args []callArg, n int, boxed bool) {
	for k := range n {
		var metas []argMeta
		for i := 10 * k; i < min(10*k+10, len(args)); i++ {
			metas = append(metas, g.argMeta(args[i]))
		}
		g.changedInt(metas)
		if boxed {
			g.box(types.IntType)
		}
	}
}

// changedInt pushes the change flags of up to 10 arguments: the plugin's
// buildChangedArgumentForCall.
func (g *gen) changedInt(metas []argMeta) {
	code := g.code
	var bits int32
	var exprs []func()
	params := g.paramBits()
	for i, m := range metas {
		if !m.stab.knownStable() && stableExprOK(m.stab, params) {
			s := m.stab
			exprs = append(exprs, func() {
				g.stableExpr(s, params)
				if i > 0 {
					code.Iconst(int32(3 * i))
					code.Op(cf.ISHL)
				}
			})
		}
		switch {
		case m.uncertain:
		case m.static:
			bits |= slotBits(paramStatic, i)
		case m.slot >= 0:
			exprs = append(exprs, func() {
				code.Iconst(slotBits(paramMask, i))
				g.shiftedFlags(m.scope, m.slot, i)
				code.Op(cf.IAND)
			})
		}
	}
	n := 0
	if bits != 0 || len(exprs) == 0 {
		code.Iconst(bits)
		n++
	}
	for _, e := range exprs {
		e()
		if n++; n > 1 {
			code.Op(cf.IOR)
		}
	}
}

// shiftedFlags pushes the change flags of tracked parameter from of flag
// scope s, moved to the place of slot to's.
func (g *gen) shiftedFlags(s *flagScope, from, to int) {
	code := g.code
	code.Local(cf.ILOAD, s.flags[from/10])
	switch shift := 3 * (to%10 - from%10); {
	case shift > 0:
		code.Iconst(int32(shift))
		code.Op(cf.ISHL)
	case shift < 0:
		code.Iconst(int32(-shift))
		code.Op(cf.ISHR)
	}
}

// paramBits returns how the code pushes the stability bits of the
// argument of a type parameter: those that the caller of the innermost
// function around it with type parameters, as the plugin's
// irTypeParameterStability looks for it, passed for the first tracked
// parameter of that type, if it has one.
func (g *gen) paramBits() paramBits {
	if g.scope() == nil {
		return nil
	}
	return func(p *types.TypeParamSym) func() {
		for s := g.scope(); s != nil; s = s.parent {
			if len(s.typeParams) == 0 || s.tracked == nil {
				continue
			}
			for i, t := range s.tracked.types {
				if (t.K == types.TypeParam || t.K == types.TypeVar) && t.Param() == p && s.tracked.params[i] != nil && !s.tracked.vararg[i] {
					return func() {
						g.code.Iconst(slotBits(paramUnstable, 0))
						g.shiftedFlags(s, i, 0)
						g.code.Op(cf.IAND)
					}
				}
			}
		}
		return nil
	}
}

// isStatic reports whether expression x always has the same value, as
// the plugin's isStatic: a constant, an enum entry, an object, a val
// whose value is static, a remembered lambda, or such.
func (g *gen) isStatic(x syntax.Expr) bool {
	switch x := x.(type) {
	case nil:
		return false
	case *syntax.IntLit, *syntax.DoubleLit, *syntax.FloatLit, *syntax.CharLit, *syntax.BoolLit, *syntax.NullLit:
		return true
	case *syntax.StringLit:
		return len(x.Parts) == 0 || len(x.Parts) == 1 && x.Parts[0].X == nil
	case *syntax.Unary:
		if x.Op == syntax.MINUS && x.Call == nil {
			switch x.X.(type) {
			case *syntax.IntLit, *syntax.DoubleLit, *syntax.FloatLit:
				return true
			}
		}
	case *syntax.Ident:
		if x.Unit || x.Object != nil {
			return true
		}
		v := x.Sym
		if v == nil || x.Member != nil || x.Getter != nil {
			return false
		}
		switch v.Kind {
		case types.Local:
			vs, ok := v.Decl.(*syntax.VarStmt)
			return ok && !v.Mutable && vs.Init != nil && vs.Delegate == nil && g.isStatic(vs.Init)
		case types.Global, types.Prop, types.Static:
			return g.staticProp(v, x.T, v.Kind != types.Prop)
		}
	case *syntax.Member:
		if x.Object != nil {
			return true
		}
		if x.Static != nil && x.Field != nil {
			return x.Field.Const || enumClass(x.Static) && x.T.K == types.Class && x.T.C == x.Static
		}
		if x.Prop != nil && !x.Safe {
			return g.staticProp(x.Prop, x.T, g.isStatic(x.X))
		}
	case *syntax.Binary:
		switch x.Op {
		case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.AND, syntax.OR,
			syntax.EQ, syntax.EQ_EQ_EQ, syntax.LT, syntax.LE, syntax.GT, syntax.GE:
			return x.Call == nil && g.stabilityOf(x.T).knownStable() && g.isStatic(x.X) && g.isStatic(x.Y)
		}
	case *syntax.Call:
		return g.staticCall(x)
	case *syntax.Lambda:
		if x.Inline || x.KProp != nil {
			return false
		}
		if x.Composable {
			return x.Ret.K == types.Unit && !x.Ret.N // remembered or a singleton
		}
		return g.memoized(x) && g.stabilityOf(x.T).knownStable() && g.staticCaptures(x)
	}
	return false
}

// staticProp reports whether reading property p, of type t, is static: a
// const, or a val of a stable type with a default getter, in the file
// being compiled, read from a static receiver.
func (g *gen) staticProp(p *types.VarSym, t types.Type, staticRecv bool) bool {
	if p.Const {
		return true
	}
	if p.Mutable || p.Getter != nil || !staticRecv || !g.stabilityOf(t).knownStable() {
		return false
	}
	pd, ok := p.Decl.(*syntax.PropDecl)
	return ok && g.fileDecls()[pd]
}

// staticCall reports whether call x's value is static: a stable
// constructor's or a known stable function's with static arguments, or
// remember's without keys.
func (g *gen) staticCall(x *syntax.Call) bool {
	all := func() bool {
		for _, a := range x.Args {
			if !g.isStatic(a) {
				return false // a spread array, too
			}
		}
		return true
	}
	if c := x.Ctor; c != nil {
		if c.Value && c.Underlying != nil {
			return g.stabilityOf(c.Underlying.Type).knownStable() && len(x.Args) == 1 && g.isStatic(x.Args[0])
		}
		for _, a := range classAnnotations(c) {
			if a.Name == "androidx/compose/runtime/Immutable" {
				return all()
			}
		}
		return false
	}
	f := x.Sym
	if f == nil {
		return false
	}
	if f.Name == "remember" && f.Facade == composablesKt && len(x.Args) == 1 {
		return g.stabilityOf(x.T).knownStable()
	}
	if _, ok := stableFunctions[funFQName(f)]; ok {
		return all()
	}
	return false
}

// memoized reports whether lambda x, in the code being generated, is
// remembered: in composable code, but in a try or a crossinline lambda,
// as with the plugin, and if it captures no var, and neither it nor the
// function it is in is annotated @DontMemoize.
func (g *gen) memoized(x *syntax.Lambda) bool {
	if g.compose == nil || x.Inline || x.Composable || x.KProp != nil || x.DontMemoize || !g.canRemember() {
		return false
	}
	for _, t := range g.tries {
		if t.source {
			return false
		}
	}
	for _, l := range g.inl {
		if l.l.Crossinline {
			return false
		}
	}
	for _, v := range x.Captures {
		if v.Mutable && v.Kind == types.Local || v.Delegate != nil {
			return false
		}
	}
	return true
}

// canRemember reports whether the innermost function that the code being
// generated is in, as the plugin sees it, remembers its lambdas: a lambda
// inlined there, unless it is annotated @DontMemoize, the copy of the
// program's inline function if that is composable, or the composable
// function or lambda of the method.
func (g *gen) canRemember() bool {
	n, k := len(g.srcInl), len(g.inl)
	switch {
	case k > 0 && (n == 0 || g.srcInl[n-1].inl < k):
		return !g.inl[k-1].l.DontMemoize
	case n > 0:
		return g.srcInl[n-1].d.Sym.Composable
	}
	return !g.compose.dontMemoize
}

// staticCaptures reports whether lambda x captures only static values.
func (g *gen) staticCaptures(x *syntax.Lambda) bool {
	if x.This {
		return false
	}
	for _, v := range x.Captures {
		if !g.staticVar(v) {
			return false
		}
	}
	return true
}

// staticVar reports whether v is a val whose value is static.
func (g *gen) staticVar(v *types.VarSym) bool {
	vs, ok := v.Decl.(*syntax.VarStmt)
	return ok && v.Kind == types.Local && !v.Mutable && vs.Init != nil && vs.Delegate == nil && g.isStatic(vs.Init)
}

// memoize generates the object of lambda x, which create pushes, as a
// value the Composer remembers, created again when a capture changed:
// the plugin's intrinsic remember of it.
func (g *gen) memoize(x *syntax.Lambda, create func()) {
	code := g.code
	n := 0
	key := func(push func() bool) {
		if push() {
			if n++; n > 1 {
				code.Op(cf.IOR)
			}
		}
	}
	if x.This {
		key(func() bool { return g.keyChanged(nil) })
	}
	for _, v := range x.Captures {
		key(func() bool { return g.keyChanged(v) })
	}
	invalid := -1
	if n > 0 {
		invalid = code.AllocLocal("Z")
		code.Local(cf.ISTORE, invalid)
	}
	g.loadComposer()
	code.InvokeInterface(composerClass, "rememberedValue", "()Ljava/lang/Object;")
	v := code.AllocLocal("Ljava/lang/Object;")
	code.Local(cf.ASTORE, v)
	fresh, done := cf.NewLabel(), cf.NewJoin("Ljava/lang/Object;")
	if invalid >= 0 {
		code.Local(cf.ILOAD, invalid)
		code.Jump(cf.IFNE, fresh)
	}
	code.Local(cf.ALOAD, v)
	code.FieldInsn(cf.GETSTATIC, composerClass, "Companion", "Landroidx/compose/runtime/Composer$Companion;")
	code.Invoke(cf.INVOKEVIRTUAL, "androidx/compose/runtime/Composer$Companion", "getEmpty", "()Ljava/lang/Object;")
	use := cf.NewLabel()
	code.Jump(cf.IF_ACMPNE, use)
	code.Place(fresh)
	create()
	code.Local(cf.ASTORE, v)
	g.loadComposer()
	code.Local(cf.ALOAD, v)
	code.InvokeInterface(composerClass, "updateRememberedValue", "(Ljava/lang/Object;)V")
	code.Place(use)
	code.Local(cf.ALOAD, v)
	code.Place(done)
	code.Checkcast(types.ClassName(types.Erase(x.T)))
	code.Retire(v)
	if invalid >= 0 {
		code.Retire(invalid)
	}
}

// keyChanged pushes whether capture v (nil: this) of a remembered lambda
// changed, as the plugin's intrinsic remember asks, and reports whether
// it pushed anything: not for a static value. A parameter's flags tell
// whether it changed if its caller knew, or its prologue asked.
func (g *gen) keyChanged(v *types.VarSym) bool {
	m := argMeta{slot: -1}
	load := func() { g.loadThis() }
	var t types.Type
	if v == nil {
		t = g.thisClass().This()
		m.stab = g.stabilityOf(t)
		m.scope, m.slot = g.trackedParam(nil)
	} else {
		t = v.Type
		x := &syntax.Ident{Name: v.Name, Sym: v}
		x.T = v.Type
		g.exprMeta(x, &m)
		load = func() { g.loadStorage(v) }
	}
	if m.static {
		return false
	}
	code := g.code
	s, i := m.scope, m.slot
	if i < 0 || m.stab.knownUnstable() {
		g.changed(t, m.stab, load, changedKey)
		return true
	}
	d := s.flags[i/10]
	isolated := func(bits int) {
		code.Local(cf.ILOAD, d)
		code.Iconst(slotBits(bits, i))
		code.Op(cf.IAND)
	}
	dirty := s.dirtyVar && !s.tracked.nonStatic[i]
	if dirty && m.stab.knownStable() {
		isolated(paramMask)
		code.Iconst(slotBits(paramDifferent, i))
		g.boolOf(cf.IF_ICMPEQ)
		return true
	}
	yes, no, done := cf.NewLabel(), cf.NewLabel(), cf.NewLabel()
	if dirty {
		// Different, or unstable and changed.
		isolated(paramMask)
		code.Iconst(slotBits(paramDifferent, i))
		code.Jump(cf.IF_ICMPEQ, yes)
		isolated(paramUnstable)
		code.Jump(cf.IFEQ, no)
		g.changed(t, m.stab, load, changedKey)
		code.Jump(cf.IFEQ, no)
	} else {
		// Unstable or uncertain, and changed; or different.
		other := cf.NewLabel()
		isolated(paramMask)
		code.Iconst(slotBits(paramStatic, i))
		code.Op(cf.IXOR)
		code.Iconst(slotBits(paramDifferent, i))
		code.Jump(cf.IF_ICMPLE, other)
		g.changed(t, m.stab, load, changedKey)
		code.Jump(cf.IFNE, yes)
		code.Place(other)
		isolated(paramStatic)
		code.Iconst(slotBits(paramDifferent, i))
		code.Jump(cf.IF_ICMPNE, no)
	}
	code.Place(yes)
	code.Iconst(1)
	code.Jump(cf.GOTO, done)
	code.Place(no)
	code.Iconst(0)
	code.Place(done)
	return true
}
