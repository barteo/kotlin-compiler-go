package codegen

import (
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Suspend lambdas.
//
// As with kotlinc, a suspend lambda that isn't inlined is an object of a
// class of its own, Owner$fun$1, which is its own continuation: it
// extends SuspendLambda (RestrictedSuspendLambda if its receiver's class
// is annotated @RestrictsSuspension) and implements the function type's
// interface, FunctionN+1. Its fields are what it captures ($x, and this$0
// for the enclosing object), the spill fields of its state machine, and
// label. Its parameters are kept in the spill fields of their locals,
// which they are loaded from when it resumes.
//
//   - invoke(P1, ..., Pn, Continuation) creates a new instance for the
//     call, with the parameters in its fields (through create, with at
//     most one parameter), and runs it: invokeSuspend(Unit).
//   - invokeSuspend(result) is the state machine: the lambda's body, with
//     the captures in locals, whose suspension points save the locals in
//     the instance's fields.
//   - invoke(Object, ..., Object), the interface's method, is a bridge.

const (
	suspendLambda           = "kotlin/coroutines/jvm/internal/SuspendLambda"
	restrictedSuspendLambda = "kotlin/coroutines/jvm/internal/RestrictedSuspendLambda"
)

// suspendLambda creates the object of a suspend lambda, generating its
// class.
func (g *gen) suspendLambda(x *syntax.Lambda) {
	ctor := g.suspendLambdaCtor(x)
	// A copy of the inline function that declares x creates an object of
	// the class the function's file generates, unless the copy reifies.
	gen := !g.declaredInCopy(x) || len(g.reifiedSubst()) > 0
	reified := false
	if x.SAM != nil {
		if gen {
			reified = g.suspendSAM(x)
		}
		ctor = ctor[:strings.Index(ctor, types.ContinuationDesc)] + ")V"
	} else if gen {
		reified = g.suspendLambdaClass(x)
	}
	c := g.code
	if samRefSingleton(x) {
		c.FieldInsn(cf.GETSTATIC, x.Class, "INSTANCE", "L"+x.Class+";")
		return
	}
	if reified {
		// As a lambda's class: its code operates on the inline function's
		// reified type parameters, and copiers create copies of it.
		if g.reifiedUse != nil {
			*g.reifiedUse = true
		}
		c.Invoke(cf.INVOKESTATIC, intrinsics, "needClassReification", "()V")
	}
	c.New(x.Class)
	c.Op(cf.DUP)
	for _, v := range x.Captures {
		g.loadStorage(v)
	}
	if x.This {
		g.loadThis()
		if g.thisClass().Value {
			g.boxValue(g.thisClass().This())
		}
	}
	if x.SAM == nil {
		c.Op(cf.ACONST_NULL)
	}
	c.Invoke(cf.INVOKESPECIAL, x.Class, "<init>", ctor)
}

// samRefSingleton reports whether suspend lambda x is a reference
// converted to a fun interface that captures nothing, whose class has a
// single instance, as kotlinc's.
func samRefSingleton(x *syntax.Lambda) bool {
	return x.SAM != nil && x.KFun != nil && len(x.Captures) == 0 && !x.This
}

// suspendSAM generates the class of a lambda converted to a fun
// interface whose method is suspend, such as FlowCollector, as kotlinc
// does: it implements the interface, with what the lambda captures in
// fields, and its method is a suspend member, with a continuation class
// of its own if it is a state machine. A reference's implements
// FunctionAdapter too, as funAdapter writes it. It reports whether the
// method operates on reified type parameters.
func (g *gen) suspendSAM(x *syntax.Lambda) bool {
	name := x.Class
	self := "L" + name + ";"
	m := x.SAM
	access := uint16(0)
	if g.publicInline {
		access = cf.AccPublic // as lambdaClass's
	}
	c := cf.NewClass(access|cf.AccFinal|cf.AccSuper, name, "java/lang/Object")
	c.Interfaces = []string{m.Owner.Name}
	if x.KFun != nil {
		c.Interfaces = append(c.Interfaces, adapter)
	}
	owner := ""
	if x.This {
		owner = "L" + g.thisClass().Name + ";"
	}
	for _, v := range x.Captures {
		c.AddField(cf.AccFinal|cf.AccSynthetic, "$"+v.Name, storageDesc(v))
	}
	if x.This {
		c.AddField(cf.AccFinal|cf.AccSynthetic, "this$0", owner)
	}

	// The constructor takes the captures.
	ctor := g.suspendLambdaCtor(x)
	ctor = ctor[:strings.Index(ctor, types.ContinuationDesc)] + ")V"
	code := c.NewMethod(access, "<init>", ctor)
	code.AllocLocal(self)
	for _, v := range x.Captures {
		slot := code.AllocLocal(storageDesc(v))
		code.Local(cf.ALOAD, 0)
		code.Local(loadOpOf(storageDesc(v)), slot)
		code.FieldInsn(cf.PUTFIELD, name, "$"+v.Name, storageDesc(v))
	}
	if x.This {
		slot := code.AllocLocal(owner)
		code.Local(cf.ALOAD, 0)
		code.Local(cf.ALOAD, slot)
		code.FieldInsn(cf.PUTFIELD, name, "this$0", owner)
	}
	code.Local(cf.ALOAD, 0)
	code.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	code.Op(cf.RETURN)
	finishCode(code)
	if samRefSingleton(x) {
		c.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal, "INSTANCE", self)
		code := c.NewMethod(cf.AccStatic, "<clinit>", "()V")
		code.New(name)
		code.Op(cf.DUP)
		code.Invoke(cf.INVOKESPECIAL, name, "<init>", "()V")
		code.FieldInsn(cf.PUTSTATIC, name, "INSTANCE", self)
		code.Op(cf.RETURN)
		finishCode(code)
	}

	// The method, with the lambda's parameter types.
	params := x.ParamSyms()
	typed := "("
	for _, p := range params {
		typed += paramDesc(p.Type)
	}
	typed += types.ContinuationDesc + ")" + objectDesc
	var reified bool
	if cont := g.suspendSAMMethod(c, x, typed, "", &reified); cont != "" && g.publicInline && capturesCrossinline(x) {
		g.suspendSAMMethod(c, x, typed, cont, &reified) // as suspendLambdaClass's invokeSuspend
	}

	// The interface's method, if its erasure differs.
	if desc := samMethodDesc(m); typed != desc {
		mg := &gen{file: g.file, cls: c, owner: g.thisClass(), shared: g.shared, noLines: g.noLines}
		mg.code = c.NewMethod(cf.AccPublic|cf.AccBridge|cf.AccSynthetic, m.Name, desc)
		code = mg.code
		code.AllocLocal(self)
		code.Local(cf.ALOAD, 0)
		jvm := descParams(desc)
		for i, p := range params {
			code.Local(loadOpOf(jvm[i]), code.AllocLocal(jvm[i]))
			if jvm[i] == objectDesc && p.Type.K == types.Unit {
				code.Checkcast("kotlin/Unit")
			} else if jvm[i] == objectDesc {
				mg.fromObject(p.Type)
			}
		}
		code.Local(cf.ALOAD, code.AllocLocal(types.ContinuationDesc))
		code.Invoke(cf.INVOKEVIRTUAL, name, m.Name, typed)
		code.Op(cf.ARETURN)
		finishCode(code)
	}
	if x.KFun != nil {
		ag := &gen{file: g.file, cls: c, owner: g.owner, shared: g.shared, noLines: 1}
		ag.funAdapter(c, x, func(v *types.VarSym) string { return "$" + v.Name })
	}
	g.localClass(c, g.cls, g.code.Name(), g.code.Desc(), x.P.Line, g.inlineXI(kmeta.WriteLambda(g.lambdaMeta(x)), kmeta.XIInlineLambda))
	c.InnerClasses[len(c.InnerClasses)-1].Access |= access
	g.cls.InnerClasses[len(g.cls.InnerClasses)-1].Access |= access
	return reified
}

// suspendLambdaBody generates method, invokeSuspend or its copy for
// inliners, of the class of suspend lambda x, whose parameters ps are in
// fields, with state machine sm: the lambda's body, with the parameters,
// the captures and this loaded from their fields into locals. It sets
// reified if the body operates on reified type parameters.
func (g *gen) suspendLambdaBody(c *cf.Class, x *syntax.Lambda, ps []lambdaParam, sm *stateMachine, method string, reified *bool) {
	name := x.Class
	lc := &lambdaCtx{slots: map[*types.VarSym]int{}, out: types.AnyType.Nullable(), unit: true}
	lg := &gen{file: g.file, cls: c, owner: g.thisClass(), shared: g.shared, prefix: "invokeSuspend$lambda", lam: lc, noLines: g.noLines,
		publicInline: g.publicInline, reifiedUse: reified, lambdaReified: g.reifiedSubst()}
	lg.code = c.NewMethod(cf.AccPublic|cf.AccFinal, method, "(Ljava/lang/Object;)Ljava/lang/Object;")
	lg.retType = x.Ret
	code := lg.code
	if !sm.forInline {
		code.TraceLiveness()
	}
	code.AllocLocal("L" + name + ";")
	sm.result = code.AllocLocal(objectDesc)
	code.NameLocal(sm.result, "$result")
	lg.sm = sm
	for _, p := range ps {
		if p.field == "" {
			continue
		}
		d := paramDesc(p.sym.Type) // a Unit parameter holds kotlin.Unit
		if !sm.forInline {
			sm.useField(p.field)
		}
		p.sym.Slot = code.AllocLocal(d)
		if p.sym == x.Recv {
			code.NameLocal(p.sym.Slot, p.sym.Name) // $this$sequence
		} else {
			lg.named(p.sym)
		}
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, p.field, fieldDesc(p.field))
		if fieldKind(d) == 'L' && d != objectDesc {
			code.Checkcast(className(d))
		}
		code.Local(storeOpOf(d), p.sym.Slot)
	}
	for _, v := range x.Captures {
		slot := code.AllocLocal(storageDesc(v))
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, "$"+v.Name, storageDesc(v))
		code.Local(storeOpOf(storageDesc(v)), slot)
		lc.slots[v] = slot
		sm.skip[slot] = true
	}
	if x.This {
		owner := "L" + g.thisClass().Name + ";"
		lc.this = code.AllocLocal(owner)
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, "this$0", owner)
		code.Local(cf.ASTORE, lc.this)
		sm.skip[lc.this] = true
	}
	if !sm.forInline {
		lg.stateMachineEntry(sm)
	}
	if x.Ret.K == types.Unit {
		lg.block(x.Body, types.UnitType, false)
		if code.Reachable() {
			lg.line(x.Body.End)
			lg.lambdaReturn(nil)
		}
	} else {
		lg.block(x.Body, x.Ret, true)
		lg.coerce(x.Ret, lc.out)
		code.Op(cf.ARETURN)
	}
	lg.endSuspend()
	if sm.forInline {
		lg.check()
	}
}

// capturesCrossinline reports whether suspend lambda x captures a
// crossinline parameter of an inline function, which kotlinc's inliner
// inlines into its copy of x's class.
func capturesCrossinline(x *syntax.Lambda) bool {
	for _, v := range x.Captures {
		if p, ok := v.Decl.(*syntax.Param); ok && p.Crossinline {
			return true
		}
	}
	return false
}

// suspendSAMMethod generates the method of the class of lambda x,
// converted to a fun interface whose method is suspend, with descriptor
// typed: the lambda's body, with the captures and this loaded from their
// fields into locals. It returns its continuation class, or "" in tail
// form. Given the continuation class, cont, it generates the method's
// copy for inliners instead, without a state machine, which creates a
// continuation between markers, as kotlinc's does: its inliner makes the
// copy of the method a state machine with a copy of that class. It sets
// reified if the body operates on reified type parameters.
func (g *gen) suspendSAMMethod(c *cf.Class, x *syntax.Lambda, typed, cont string, reified *bool) string {
	forInline := cont != ""
	name := x.Class
	m := x.SAM
	owner := ""
	if x.This {
		owner = "L" + g.thisClass().Name + ";"
	}
	params := x.ParamSyms()
	lc := &lambdaCtx{slots: map[*types.VarSym]int{}, out: types.AnyType.Nullable(), unit: true}
	lg := &gen{file: g.file, cls: c, owner: g.thisClass(), shared: g.shared, prefix: m.Name + "$lambda", lam: lc, noLines: g.noLines,
		publicInline: g.publicInline, reifiedUse: reified, lambdaReified: g.reifiedSubst()}
	method := m.Name
	if forInline {
		method += "$$forInline"
	}
	lg.code = c.NewMethod(cf.AccPublic|cf.AccFinal, method, typed)
	lg.retType = x.Ret
	code := lg.code
	code.AllocLocal("L" + name + ";")
	for _, p := range params {
		p.Slot = code.AllocLocal(paramDesc(p.Type))
		if x.Ref != nil && g.noLines == 0 {
			code.NameLocal(p.Slot, strings.TrimPrefix(p.Name, "$")) // a reference's $p0, p0 as kotlinc's
			continue
		}
		lg.named(p)
	}
	k := code.AllocLocal(types.ContinuationDesc)
	code.NameLocal(k, "$completion")
	if forInline {
		lg.fakeContinuation(cont, "(L"+name+";"+types.ContinuationDesc+")V", k)
	}
	skip := map[int]bool{}
	for _, v := range x.Captures {
		slot := code.AllocLocal(storageDesc(v))
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, "$"+v.Name, storageDesc(v))
		code.Local(storeOpOf(storageDesc(v)), slot)
		lc.slots[v] = slot
		skip[slot] = true
	}
	if x.This {
		lc.this = code.AllocLocal(owner)
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, "this$0", owner)
		code.Local(cf.ASTORE, lc.this)
		skip[lc.this] = true
	}
	fd := &syntax.FunDecl{P: x.P, Name: m.Name, Body: x.Body, Continuation: name + "$" + m.Name + "$1",
		Sym: &types.FunSym{Name: m.Name, Ret: x.Ret, Suspend: true}}
	lg.suspendFun(fd, m.Name, typed, k, &resumeCall{this: true, forInline: forInline, lambda: x, call: func(g *gen) {
		g.code.Invoke(cf.INVOKEVIRTUAL, name, m.Name, typed)
	}})
	if lg.sm.cls != "" {
		for s := range skip {
			lg.sm.skip[s] = true
		}
	}
	if x.Ret.K == types.Unit {
		lg.block(x.Body, types.UnitType, false)
		if code.Reachable() {
			lg.line(x.Body.End)
			lg.lambdaReturn(nil)
		}
	} else {
		lg.block(x.Body, x.Ret, true)
		lg.coerce(x.Ret, lc.out)
		code.Op(cf.ARETURN)
	}
	cls := lg.sm.cls
	lg.endSuspend()
	lg.check()
	return cls
}

// suspendLambdaCtor returns the descriptor of the constructor of a
// suspend lambda's class: it takes the captures, the enclosing object if
// the lambda uses this, and the continuation the lambda is created for.
func (g *gen) suspendLambdaCtor(x *syntax.Lambda) string {
	d := "("
	for _, v := range x.Captures {
		d += storageDesc(v)
	}
	if x.This {
		d += "L" + g.thisClass().Name + ";"
	}
	return d + types.ContinuationDesc + ")V"
}

// lambdaParam is a parameter of a suspend lambda, in the field of its
// kind.
type lambdaParam struct {
	sym   *types.VarSym
	field string
}

func (g *gen) suspendLambdaClass(x *syntax.Lambda) bool {
	name := x.Class
	self := "L" + name + ";"
	super := suspendLambda
	params := x.ParamSyms()
	if x.Recv != nil && restrictedClass(x.Recv.Type) != nil {
		super = restrictedSuspendLambda
	}
	access := uint16(0)
	if g.publicInline {
		access = cf.AccPublic // as lambdaClass's
	}
	c := cf.NewClass(access|cf.AccFinal|cf.AccSuper, name, super)
	arity := len(params) + 1
	c.Interfaces = []string{types.KotlinFunctionPackage + "Function" + strconv.Itoa(arity)}
	spread := arity > types.MaxArity // FunctionN, whose invoke takes an array
	if spread {
		c.Interfaces[0] = types.KotlinFunctionPackage + "FunctionN"
	}
	owner := ""
	if x.This {
		owner = "L" + g.thisClass().Name + ";"
	}

	// invokeSuspend, which tells the fields the class has.
	var ps []lambdaParam
	counts := map[byte]int{}
	used := uses(x.Body)
	for _, p := range params {
		if !used[p] {
			// As kotlinc does, a parameter the body doesn't use isn't
			// kept.
			ps = append(ps, lambdaParam{p, ""})
			continue
		}
		k := fieldKind(paramDesc(p.Type))
		ps = append(ps, lambdaParam{p, string(k) + "$" + strconv.Itoa(counts[k])})
		counts[k]++
	}
	sm := &stateMachine{name: "invokeSuspend", cls: name, lambda: true, cont: 0, completion: -1, line: x.P.Line, skip: map[int]bool{}}
	var reified bool
	g.suspendLambdaBody(c, x, ps, sm, "invokeSuspend", &reified)
	if g.publicInline && capturesCrossinline(x) {
		// As kotlinc's, for its inliner, which copies the class with the
		// caller's lambdas inlined: the body without the state machine,
		// whose suspension points are marked, which the copy makes one.
		sm := &stateMachine{name: "invokeSuspend", lambda: true, forInline: true, line: x.P.Line, skip: map[int]bool{}}
		g.suspendLambdaBody(c, x, ps, sm, "invokeSuspend$$forInline", &reified)
	}

	// The fields.
	for _, f := range sm.fields {
		access := uint16(0)
		for i, p := range ps {
			if p.field == f {
				// As kotlinc's: a parameter's is synthetic, and the
				// receiver's private.
				access = cf.AccSynthetic
				if i == 0 && x.Recv != nil {
					access |= cf.AccPrivate
				}
			}
		}
		c.AddField(access, f, fieldDesc(f))
	}
	c.AddField(0, "label", "I")
	g.debugMetadata(c, sm, "invokeSuspend", name)
	for _, v := range x.Captures {
		c.AddField(cf.AccFinal|cf.AccSynthetic, "$"+v.Name, storageDesc(v))
	}
	if x.This {
		c.AddField(cf.AccFinal|cf.AccSynthetic, "this$0", owner)
	}

	// The constructor.
	ctor := g.suspendLambdaCtor(x)
	code := c.NewMethod(access, "<init>", ctor)
	code.AllocLocal(self)
	slots := map[*types.VarSym]int{}
	for _, v := range x.Captures {
		slots[v] = code.AllocLocal(storageDesc(v))
	}
	this := -1
	if x.This {
		this = code.AllocLocal(owner)
	}
	k := code.AllocLocal(types.ContinuationDesc)
	code.NameLocal(k, "$completion")
	for _, v := range x.Captures {
		code.Local(cf.ALOAD, 0)
		code.Local(loadOpOf(storageDesc(v)), slots[v])
		code.FieldInsn(cf.PUTFIELD, name, "$"+v.Name, storageDesc(v))
	}
	if this >= 0 {
		code.Local(cf.ALOAD, 0)
		code.Local(cf.ALOAD, this)
		code.FieldInsn(cf.PUTFIELD, name, "this$0", owner)
	}
	code.Local(cf.ALOAD, 0)
	code.Iconst(int32(arity))
	code.Local(cf.ALOAD, k)
	code.Invoke(cf.INVOKESPECIAL, super, "<init>", "(I"+types.ContinuationDesc+")V")
	code.Op(cf.RETURN)
	finishCode(code)

	// newInstance pushes a new instance, for continuation k, in code of
	// the class.
	newInstance := func(code *cf.Code, k int) {
		code.New(name)
		code.Op(cf.DUP)
		for _, v := range x.Captures {
			code.Local(cf.ALOAD, 0)
			code.FieldInsn(cf.GETFIELD, name, "$"+v.Name, storageDesc(v))
		}
		if x.This {
			code.Local(cf.ALOAD, 0)
			code.FieldInsn(cf.GETFIELD, name, "this$0", owner)
		}
		code.Local(cf.ALOAD, k)
		code.Invoke(cf.INVOKESPECIAL, name, "<init>", ctor)
	}
	// mg generates code in the class's other methods.
	mg := &gen{file: g.file, cls: c, owner: g.thisClass(), shared: g.shared, noLines: g.noLines}

	// create, and the typed invoke.
	typed := "("
	for _, p := range params {
		typed += paramDesc(p.Type)
	}
	typed += types.ContinuationDesc + ")" + objectDesc
	if len(params) <= 1 {
		createDesc := "("
		for range params {
			createDesc += objectDesc
		}
		createDesc += types.ContinuationDesc + ")" + types.ContinuationDesc
		mg.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "create", createDesc)
		code = mg.code
		code.AllocLocal(self)
		var arg int
		if len(params) == 1 {
			arg = code.AllocLocal(objectDesc)
			code.NameLocal(arg, "value")
		}
		k := code.AllocLocal(types.ContinuationDesc)
		code.NameLocal(k, "$completion")
		newInstance(code, k)
		if len(params) == 1 && ps[0].field != "" {
			inst := code.AllocLocal(self)
			code.Local(cf.ASTORE, inst)
			code.Local(cf.ALOAD, inst)
			code.Local(cf.ALOAD, arg)
			if params[0].Type.K != types.Unit {
				mg.fromObject(params[0].Type)
			}
			code.FieldInsn(cf.PUTFIELD, name, ps[0].field, fieldDesc(ps[0].field))
			code.Local(cf.ALOAD, inst)
		}
		code.Checkcast(continuationName)
		code.Op(cf.ARETURN)
		finishCode(code)

		mg.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "invoke", typed)
		code = mg.code
		code.AllocLocal(self)
		code.Local(cf.ALOAD, 0)
		for _, p := range params {
			d := paramDesc(p.Type)
			code.Local(loadOpOf(d), code.AllocLocal(d))
			if p.Type.K != types.Unit {
				mg.coerce(p.Type, types.AnyType.Nullable())
			}
		}
		code.Local(cf.ALOAD, code.AllocLocal(types.ContinuationDesc))
		code.Invoke(cf.INVOKEVIRTUAL, name, "create", createDesc)
		code.Checkcast(name)
	} else {
		mg.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "invoke", typed)
		code = mg.code
		code.AllocLocal(self)
		args := make([]int, len(params))
		for i, p := range params {
			args[i] = code.AllocLocal(paramDesc(p.Type))
		}
		k := code.AllocLocal(types.ContinuationDesc)
		newInstance(code, k)
		inst := code.AllocLocal(self)
		code.Local(cf.ASTORE, inst)
		for i, p := range params {
			if ps[i].field == "" {
				continue
			}
			code.Local(cf.ALOAD, inst)
			code.Local(loadOpOf(paramDesc(p.Type)), args[i])
			code.FieldInsn(cf.PUTFIELD, name, ps[i].field, fieldDesc(ps[i].field))
		}
		code.Local(cf.ALOAD, inst)
	}
	code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
	code.Invoke(cf.INVOKEVIRTUAL, name, "invokeSuspend", "(Ljava/lang/Object;)Ljava/lang/Object;")
	code.Op(cf.ARETURN)
	finishCode(code)

	// The interface's method.
	bridge := "("
	for range arity {
		bridge += objectDesc
	}
	bridge += ")" + objectDesc
	bridgeAccess := uint16(cf.AccPublic | cf.AccBridge | cf.AccSynthetic)
	if spread {
		bridge = types.VarArityDesc
		bridgeAccess |= cf.AccFinal // as kotlinc's
	}
	mg.code = c.NewMethod(bridgeAccess, "invoke", bridge)
	code = mg.code
	code.AllocLocal(self)
	var array int
	if spread {
		array = code.AllocLocal("[Ljava/lang/Object;")
	}
	args := make([]int, arity)
	for i := range args {
		args[i] = code.AllocLocal(objectDesc)
	}
	if spread {
		spreadArgs(code, array, args)
	}
	code.Local(cf.ALOAD, 0)
	for i, p := range params {
		code.Local(cf.ALOAD, args[i])
		if p.Type.K == types.Unit {
			code.Checkcast("kotlin/Unit")
		} else {
			mg.fromObject(p.Type)
		}
	}
	code.Local(cf.ALOAD, args[len(params)])
	code.Checkcast(continuationName)
	code.Invoke(cf.INVOKEVIRTUAL, name, "invoke", typed)
	code.Op(cf.ARETURN)
	finishCode(code)

	g.localClass(c, g.cls, g.code.Name(), g.code.Desc(), x.P.Line, g.inlineXI(kmeta.WriteLambda(g.lambdaMeta(x)), kmeta.XIInlineLambda))
	c.InnerClasses[len(c.InnerClasses)-1].Access |= access
	g.cls.InnerClasses[len(g.cls.InnerClasses)-1].Access |= access
	return reified
}

// restrictedClass returns the class of t if it or a supertype is
// annotated @RestrictsSuspension.
func restrictedClass(t types.Type) *types.ClassSym {
	t = t.Flex().NonNull()
	if t.K != types.Class {
		return nil
	}
	found := false
	t.C.Supertypes(func(s *types.ClassSym) bool {
		found = s.RestrictsSuspension
		return !found
	})
	if !found {
		return nil
	}
	return t.C
}

// uses returns the variables that code reads or writes, as values or as
// implicit receivers, also in the lambdas and local functions in it.
func uses(n any) map[*types.VarSym]bool {
	out := map[*types.VarSym]bool{}
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Ident:
			if n.Sym != nil {
				out[n.Sym] = true
			}
			if n.Member != nil {
				syntax.Inspect(n.Member, visit)
			}
		case *syntax.This:
			if n.Recv != nil {
				out[n.Recv] = true
			}
		case *syntax.Member:
			if n.DispatchThis != nil && n.DispatchThis.Recv != nil {
				out[n.DispatchThis.Recv] = true // a member extension's
			}
		case *syntax.Binary:
			if n.Call != nil {
				syntax.Inspect(n.Call, visit)
			}
		case *syntax.Index:
			if n.Call != nil {
				syntax.Inspect(n.Call, visit)
			}
		case *syntax.Unary:
			if n.Call != nil {
				syntax.Inspect(n.Call, visit)
			}
		}
		return true
	}
	syntax.Inspect(n, visit)
	return out
}

// finishCode reports an assembly error in a method generated outside g.
func finishCode(code *cf.Code) {
	if err := code.Err(); err != nil {
		panic(err)
	}
}
