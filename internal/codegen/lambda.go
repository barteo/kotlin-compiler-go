package codegen

import (
	"slices"
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Lambdas.
//
// Each lambda's body becomes a private static method of the class being
// generated, named after the function it appears in (main$lambda$0, and
// main$lambda$0$0 for a lambda inside that one), as kotlinc names them.
// The method takes the enclosing object if the lambda uses it, then the
// captured variables, then the parameters of its function type's
// interface (or, when SAM-converted, of the Java interface's method), with
// that method's erased types. An invokedynamic call site creates the
// lambda object through LambdaMetafactory, passing the captured values.
//
// A captured var lives in a one-element array, so the lambda and the
// enclosing function see each other's assignments.

// metafactory is the bootstrap method of lambda call sites.
const metafactory = "(Ljava/lang/invoke/MethodHandles$Lookup;Ljava/lang/String;Ljava/lang/invoke/MethodType;" +
	"Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodHandle;Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/CallSite;"

// lambdaCtx describes the lambda whose method is being generated.
type lambdaCtx struct {
	slots map[*types.VarSym]int // captured variables
	this  int                   // slot of the enclosing object, if it is captured
	out   types.Type            // the method's erased result type
	unit  bool                  // a Unit result is kotlin.Unit, not null
	fun   bool                  // a local function's method: return is a plain return
	class bool                  // the method of a lambda's class: see lambdaClass
}

// frameDesc is the start of the descriptor of the method of a lambda or
// a local function: the enclosing object if it uses this, then the
// captured variables.
func (g *gen) frameDesc(x *syntax.Lambda) string {
	d := ""
	if x.This {
		d += "L" + g.thisClass().Name + ";"
	}
	for _, v := range x.Captures {
		d += storageDesc(v)
	}
	return d
}

// allocFrame allocates the locals of a lambda's or local function's
// method that frameDesc describes, and records them in lc.
func (g *gen) allocFrame(x *syntax.Lambda, lc *lambdaCtx) {
	if x.This {
		lc.this = g.code.AllocLocal("L" + g.thisClass().Name + ";")
	}
	for _, v := range x.Captures {
		lc.slots[v] = g.code.AllocLocal(storageDesc(v))
	}
}

// loadFrame pushes the values that frameDesc describes.
func (g *gen) loadFrame(x *syntax.Lambda) {
	if x.This {
		g.loadThis()
		if g.thisClass().Value {
			g.boxValue(g.thisClass().This())
		}
	}
	for _, v := range x.Captures {
		g.loadStorage(v)
	}
}

// FunctionInterface generates the interface for function types with n
// parameters: one erased invoke method, as in kotlin.jvm.functions.
func FunctionInterface(n int) Output {
	name := types.KcFunctions.Class(n).Name
	c := cf.NewClass(cf.AccPublic|cf.AccInterface|cf.AccAbstract, name, "java/lang/Object")
	c.AddAbstractMethod(cf.AccPublic|cf.AccAbstract, "invoke", "("+strings.Repeat("Ljava/lang/Object;", n)+")Ljava/lang/Object;")
	return Output{Name: name, Bytes: c.Bytes()}
}

// spreadArgs copies the arguments of FunctionN's invoke, the array in
// local args, to the n locals slots, throwing an IllegalArgumentException
// if there aren't n of them, as kotlinc's bridges do.
func spreadArgs(code *cf.Code, args int, slots []int) {
	n := len(slots)
	ok := cf.NewLabel()
	code.Local(cf.ALOAD, args)
	code.Op(cf.ARRAYLENGTH)
	code.Iconst(int32(n))
	code.Jump(cf.IF_ICMPEQ, ok)
	code.New("java/lang/IllegalArgumentException")
	code.Op(cf.DUP)
	code.Sconst("Expected " + strconv.Itoa(n) + " arguments")
	code.Invoke(cf.INVOKESPECIAL, "java/lang/IllegalArgumentException", "<init>", "(Ljava/lang/String;)V")
	code.Op(cf.ATHROW)
	code.Place(ok)
	for i, slot := range slots {
		code.Local(cf.ALOAD, args)
		code.Iconst(int32(i))
		code.Op(cf.AALOAD)
		code.Local(cf.ASTORE, slot)
	}
}

// argArray pushes the arguments of a call of FunctionN's invoke, which
// takes them in an array: a suspend function type's continuation last.
func (g *gen) argArray(x *syntax.Call, f *types.FunSym) {
	n := len(x.Args)
	if f.Suspend {
		n++
	}
	g.code.Iconst(int32(n))
	g.code.ANewArray("java/lang/Object")
	for i, a := range x.Args {
		g.code.Op(cf.DUP)
		g.code.Iconst(int32(i))
		g.argAs(a, f.Params[i].Type)
		g.code.Op(cf.AASTORE)
	}
	if f.Suspend {
		g.code.Op(cf.DUP)
		g.code.Iconst(int32(len(x.Args)))
		g.continuationArg(x)
		g.code.Op(cf.AASTORE)
	}
	g.line(x.Pos()) // the call, after arguments on other lines
}

// varArity reports whether lambda x implements FunctionN, which
// LambdaMetafactory can't: its arity comes from kotlin.jvm.internal.Lambda.
func varArity(x *syntax.Lambda) bool {
	return x.SAM == nil && x.KProp == nil && x.T.K == types.Class && x.T.C.VarArity()
}

// storageDesc is the descriptor of the local holding v: an array for a
// captured var.
func storageDesc(v *types.VarSym) string {
	if v.Boxed() {
		return "[" + desc(v.Type)
	}
	return desc(v.Type)
}

// lamCaptures reports whether the lambda whose method is being generated
// captures v.
func (g *gen) lamCaptures(v *types.VarSym) bool {
	if g.lam == nil {
		return false
	}
	_, ok := g.lam.slots[v]
	return ok
}

// classCaptures reports whether the local or anonymous class whose method
// is being generated captures v, which loadCaptures copied to a local.
func (g *gen) classCaptures(v *types.VarSym) bool {
	if g.owner == nil {
		return false
	}
	cd, ok := g.owner.Decl.(*syntax.ClassDecl)
	return ok && slices.Contains(cd.Captures, v)
}

// loadStorage pushes what holds v: its value, or its array if boxed.
func (g *gen) loadStorage(v *types.VarSym) {
	if v.OuterOf != nil && !g.lamCaptures(v) && !g.classCaptures(v) {
		g.loadStorage(v.OuterOf)
		g.code.FieldInsn(cf.GETFIELD, v.OuterCls.Name, "this$0", desc(v.OuterCls.Outer.This()))
		return
	}
	if p := g.fieldParams[v]; p != nil && g.lam == nil {
		// A primary constructor's parameter in the serialization
		// plugin's synthetic constructor: its property's field.
		g.code.Local(cf.ALOAD, 0)
		g.code.FieldInsn(cf.GETFIELD, p.Owner.Name, p.Name, desc(p.Type))
		return
	}
	slot := v.Slot
	if g.lam != nil {
		if s, ok := g.lam.slots[v]; ok {
			slot = s
		}
	}
	if v.Boxed() {
		g.code.Local(cf.ALOAD, slot)
		return
	}
	g.code.Local(loadOp(v.Type), slot)
}

// loadThis pushes the object whose members are being generated: this, or
// in a lambda, the enclosing object.
func (g *gen) loadThis() {
	switch n := len(g.srcInl); {
	case n > 0 && g.srcInl[n-1].self >= 0:
		g.code.Local(cf.ALOAD, g.srcInl[n-1].self) // the receiver of a copy of an inline member
	case g.lam != nil:
		g.code.Local(cf.ALOAD, g.lam.this)
		if g.owner != nil && g.owner.Value {
			// A value class's lambdas capture its this boxed.
			g.code.Invoke(cf.INVOKEVIRTUAL, g.owner.Name, "unbox-impl", "()"+desc(g.owner.Underlying.Type))
		}
	case g.objThis:
		g.objectRef(g.owner) // in static code: the object's instance
	case g.valueThis:
		g.code.Local(loadOp(g.owner.Underlying.Type), g.valueSelf)
	default:
		g.code.Local(cf.ALOAD, g.self)
	}
}

// inInterface reports whether the code being generated is in an
// interface's class file, where lambdas' and local functions' methods are
// interface methods: not in its DefaultImpls class. A companion's
// initializers are, which the interface's static initializer runs.
func (g *gen) inInterface() bool {
	return g.cls.Access&cf.AccInterface != 0
}

// thisClass returns the class of this: the class of an inline member
// whose copy is being generated, or else the class being generated.
func (g *gen) thisClass() *types.ClassSym {
	if n := len(g.srcInl); n > 0 && g.srcInl[n-1].self >= 0 {
		return g.srcInl[n-1].d.Sym.Owner
	}
	return g.owner
}

// newBox creates the array that holds a captured var of type t.
func (g *gen) newBox(t types.Type) {
	g.code.Iconst(1)
	g.newArray("[" + desc(t))
}

// arrayLoad loads element 0 of a box of type t; the array and the index
// are on the stack.
func (g *gen) arrayLoad(t types.Type) {
	g.elemLoad("[" + desc(t))
}

// arrayStore stores into a box of type t; the array, the index and the
// value are on the stack.
func (g *gen) arrayStore(t types.Type) {
	g.elemStore("[" + desc(t))
}

// lambdaReturn returns value (nil: none) from a lambda's method.
func (g *gen) lambdaReturn(value syntax.Expr) {
	g.lambdaValue(value)
	g.lambdaFinish()
}

// lambdaValue pushes a lambda's result, of its result type: nothing for
// Unit.
func (g *gen) lambdaValue(value syntax.Expr) {
	g.resultAs(value, g.retType)
}

// resultAs pushes a lambda's result, value, as type t: a bare return@label
// returns Unit, which is kotlin.Unit.INSTANCE when the lambda's result
// type isn't Unit.
func (g *gen) resultAs(value syntax.Expr, t types.Type) {
	switch {
	case t.K == types.Unit:
		if value != nil {
			g.exprAs(value, types.UnitType)
		}
	case value == nil:
		g.code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
	default:
		g.exprAs(value, t)
	}
}

// lambdaFinish returns the result that lambdaValue pushed, as the
// interface method's erased result type.
func (g *gen) lambdaFinish() {
	out := g.lam.out
	if g.retType.K == types.Unit {
		switch {
		case out.K == types.Unit:
		case g.lam.unit:
			g.code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
		default:
			g.code.Op(cf.ACONST_NULL)
		}
	} else {
		g.coerce(g.retType, out)
	}
	g.composeLambdaEnd()
	g.code.Op(returnOp(out))
}

// lambda creates the object for a lambda, generating its method: in
// composable code, a remembered one (see skip.go).
func (g *gen) lambda(x *syntax.Lambda) {
	if g.memoized(x) {
		g.memoize(x, func() { g.newLambda(x) })
		return
	}
	g.newLambda(x)
}

// newLambda creates a new object for a lambda, generating its method.
func (g *gen) newLambda(x *syntax.Lambda) {
	if x.Class != "" {
		g.suspendLambda(x)
		return
	}
	if g.publicInline || x.KProp != nil || x.KFun != nil || varArity(x) {
		g.compatSingleton(x)
		g.lambdaClass(x)
		return
	}
	if g.singleton(x) {
		g.singletonLambda(x)
		return
	}
	g.indyLambda(x)
}

// indyLambda creates a new object for a lambda with an invokedynamic call
// site, generating its method.
func (g *gen) indyLambda(x *syntax.Lambda) {
	key := g.cls.Name + "." + g.prefix
	name := g.prefix + "$" + strconv.Itoa(g.shared.count[key])
	g.shared.count[key]++
	wrap := g.composeLambdaWrap(x)
	method, samDesc, implDesc := g.lambdaMethod(x, name)

	g.loadFrame(x)
	site := "(" + g.frameDesc(x) + ")L" + x.T.C.Name + ";"
	p := &g.cls.Pool
	impl := p.Method(g.cls.Name, name, implDesc)
	if g.inInterface() {
		// A lambda in an interface's default method is a method of the
		// interface.
		impl = p.InterfaceMethod(g.cls.Name, name, implDesc)
	}
	bsm := p.Bootstrap(
		p.MethodHandle(cf.RefInvokeStatic, p.Method("java/lang/invoke/LambdaMetafactory", "metafactory", metafactory)),
		p.MethodType(samDesc),
		p.MethodHandle(cf.RefInvokeStatic, impl),
		p.MethodType(samDesc))
	g.code.InvokeDynamic(bsm, method, site)
	wrap()
}

// samMethodDesc returns the JVM descriptor of an interface's method: one of
// the class path has it, one of the program's fun interfaces computes it.
func samMethodDesc(f *types.FunSym) string {
	if f.Desc != "" {
		return f.Desc
	}
	return funDesc(f)
}

// lambdaMethod generates the method of a lambda. It returns the name and
// erased descriptor of the interface method the lambda implements, and the
// descriptor of the generated method.
func (g *gen) lambdaMethod(x *syntax.Lambda, name string) (method, samDesc, implDesc string) {
	var in []types.Type
	var out types.Type
	if x.SAM != nil {
		method, samDesc = x.SAM.JVM(), samMethodDesc(x.SAM)
		if x.SAM.Ext {
			in = append(in, types.Erase(x.SAM.Recv))
		}
		for _, p := range x.SAM.Params {
			in = append(in, types.Erase(p.Type))
		}
		out = types.Erase(x.SAM.Ret)
	} else {
		n := x.T.A.Len() - 1
		if x.Composable {
			n += 1 + x.T.C.Methods["invoke"][0].Changed // the Composer and the change flags
		}
		method = "invoke"
		samDesc = "(" + strings.Repeat("Ljava/lang/Object;", n) + ")Ljava/lang/Object;"
		out = types.AnyType.Nullable()
		for range n {
			in = append(in, out)
		}
	}

	// The enclosing object and the captured variables come first.
	implDesc = "(" + g.frameDesc(x) + samDesc[1:]
	// Kotlin's function types return kotlin.Unit for Unit, as kotlinc's
	// lambdas do; kc's own return null.
	unit := x.SAM == nil && strings.HasPrefix(x.T.C.Name, types.KotlinFunctionPackage)
	lc := &lambdaCtx{slots: map[*types.VarSym]int{}, out: out, unit: unit}
	lg := &gen{file: g.file, cls: g.cls, owner: g.thisClass(), shared: g.shared, prefix: name, lam: lc, noLines: g.noLines,
		publicInline: g.publicInline, reifiedUse: g.reifiedUse, // copies of an inline function copy its lambdas
		lambdaReified: g.reifiedSubst(), inlineSingletons: g.inlineSingletons}
	access := uint16(cf.AccPrivate | cf.AccStatic | cf.AccFinal | cf.AccSynthetic)
	if g.inInterface() {
		access &^= cf.AccFinal // interface methods can't be final
	}
	lg.code = g.cls.NewMethod(access, name, implDesc)
	lg.retType = x.Ret
	lg.allocFrame(x, lc)

	// Parameters arrive erased and are converted to the lambda's
	// parameter types.
	syms := x.ParamSyms()
	slots := make([]int, len(in))
	for i := range in {
		slots[i] = lg.code.AllocLocal(desc(in[i]))
	}
	for i, p := range syms {
		// A value class whose underlying type is Any has its descriptor,
		// but arrives boxed for a type parameter.
		boxed := types.Unboxed(p.Type) && !types.IsValue(in[i])
		// A Unit parameter is never read: a Unit value is nothing on
		// the stack. A Nothing or Nothing? one, as for a star
		// projection's type argument, is null, which needs no cast.
		unit := p.Type.K == types.Unit && !p.Type.N || p.Type.K == types.Null || p.Type.K == types.Nothing
		if desc(in[i]) == desc(p.Type) && !boxed || unit {
			p.Slot = slots[i]
			lg.named(p)
			continue
		}
		lg.code.Local(loadOp(in[i]), slots[i])
		if in[i].IsPrimitive() {
			lg.coerce(in[i], p.Type)
		} else {
			lg.castErased(in[i], p.Type)
		}
		p.Slot = lg.code.AllocLocal(desc(p.Type))
		lg.named(p)
		lg.code.Local(storeOp(p.Type), p.Slot)
	}
	if x.Composable {
		lg.composeLambdaStart(x, slots[len(syms):])
	}
	if x.Ret.K == types.Unit {
		lg.block(x.Body, types.UnitType, false)
		if lg.code.Reachable() {
			lg.line(x.Body.End)
			lg.lambdaReturn(nil)
		}
		if x.Composable {
			lg.composeLambdaSkip()
		}
	} else {
		lg.block(x.Body, x.Ret, true)
		lg.coerce(x.Ret, out)
		lg.composeLambdaEnd()
		lg.code.Op(returnOp(out))
	}
	lg.check()
	return method, samDesc, implDesc
}

// lambdaClass creates the object of a lambda in the code of an inline
// function that isn't private, which other modules copy, as kotlinc
// does: an instance of a class of its own, Owner$fun$1, rather than of an
// invokedynamic call site, whose method other classes can't call. The
// class implements the function type's interface (or the SAM-converted
// one) with a final method; its constructor takes what the lambda
// captures, and keeps it in fields, $x for a local x and this$0 for the
// enclosing object, which the method copies to locals. A lambda that
// captures nothing is the class's INSTANCE. If its code operates on the
// function's reified type parameters, Intrinsics.needClassReification()
// comes first: copiers create copies of the class with the call's type
// arguments.
//
// A property reference's KProperty is also an object of a class of its
// own, as kotlinc's, which extends the standard library's for its kind
// of reference (see kpropSuper) and whose get is the lambda. So is a
// lambda of a function type with more than MaxArity parameters, which
// extends kotlin.jvm.internal.Lambda for its arity, and whose invoke
// takes the arguments in an array.
func (g *gen) lambdaClass(x *syntax.Lambda) {
	var name string
	if x.KFunClass != "" && !g.declaredInCopy(x) {
		name = x.KFunClass
	} else if g.lam != nil && g.lam.class {
		key := g.cls.Name + ".class"
		g.shared.count[key]++
		name = g.cls.Name + "$" + strconv.Itoa(g.shared.count[key])
	} else {
		// A function's default values' lambdas are named after the
		// function, as its body's are.
		base := strings.TrimSuffix(strings.TrimSuffix(g.prefix, "$lambda"), "$default")
		key := g.cls.Name + ".class." + base
		for name == "" || g.file.Named[name] {
			g.shared.count[key]++
			name = g.cls.Name + "$" + base + "$" + strconv.Itoa(g.shared.count[key])
		}
	}
	self := "L" + name + ";"

	var iface, method, samDesc string
	var in []types.Type
	var out types.Type
	super, kprop := "java/lang/Object", x.KProp
	switch {
	case kprop != nil:
		// A property reference's KProperty: see kpropSuper.
		iface, method = x.T.C.Name, "get"
		super = kpropSuper(x)
		n := len(x.ParamSyms())
		samDesc = "(" + strings.Repeat("Ljava/lang/Object;", n) + ")Ljava/lang/Object;"
		out = types.AnyType.Nullable()
		for range n {
			in = append(in, out)
		}
	case x.SAM != nil:
		iface, method, samDesc = x.SAM.Owner.Name, x.SAM.JVM(), samMethodDesc(x.SAM)
		if x.SAM.Ext {
			in = append(in, types.Erase(x.SAM.Recv))
		}
		for _, p := range x.SAM.Params {
			in = append(in, types.Erase(p.Type))
		}
		out = types.Erase(x.SAM.Ret)
	default:
		n := x.T.A.Len() - 1
		if x.Composable {
			n += 1 + x.T.C.Methods["invoke"][0].Changed // the Composer and the change flags
		}
		if x.KFun != nil && x.Suspend {
			n++ // the continuation, which the function doesn't take
		}
		iface, method = x.T.C.Name, "invoke"
		samDesc = "(" + strings.Repeat("Ljava/lang/Object;", n) + ")Ljava/lang/Object;"
		if varArity(x) {
			samDesc, super = types.VarArityDesc, "kotlin/jvm/internal/Lambda"
		}
		if x.KFun != nil {
			// A function reference's KFunction: see kfunInit.
			super = kfunSuper(x)
			if x.T.C.Ensure(); x.T.C.KFunctionN() {
				iface = x.T.C.Interfaces[1].Name
			}
		}
		out = types.AnyType.Nullable()
		for range n {
			in = append(in, out)
		}
	}
	// A reference converted to a fun interface implements it by calling
	// the function, and FunctionAdapter by a FunctionReference to it, as
	// kotlinc's: two of the same function are equal.
	adapted := x.KFun != nil && x.SAM != nil
	access := uint16(cf.AccPublic)
	if adapted {
		access = cf.AccSynthetic
	}
	c := cf.NewClass(access|cf.AccFinal|cf.AccSuper, name, super)
	if kprop == nil {
		c.Interfaces = []string{iface}
	}
	if adapted {
		c.Interfaces = append(c.Interfaces, adapter)
	}
	if x.KFun != nil && x.Suspend {
		c.Interfaces = append(c.Interfaces, "kotlin/coroutines/jvm/internal/SuspendFunction")
	}
	owner := ""
	if x.This {
		owner = "L" + g.thisClass().Name + ";"
		c.AddField(cf.AccFinal|cf.AccSynthetic, "this$0", owner)
	}
	for _, v := range x.Captures {
		c.AddField(cf.AccFinal|cf.AccSynthetic, lambdaField(v), storageDesc(v))
	}
	single := !x.This && len(x.Captures) == 0

	// The constructor takes the captures, in the order of frameDesc.
	ctor := "(" + g.frameDesc(x) + ")V"
	code := c.NewMethod(access&cf.AccPublic, "<init>", ctor)
	code.AllocLocal(self)
	if x.This {
		slot := code.AllocLocal(owner)
		code.Local(cf.ALOAD, 0)
		code.Local(cf.ALOAD, slot)
		code.FieldInsn(cf.PUTFIELD, name, "this$0", owner)
	}
	bound := -1 // the slot of a bound property reference's receiver
	for _, v := range x.Captures {
		slot := code.AllocLocal(storageDesc(v))
		code.Local(cf.ALOAD, 0)
		code.Local(loadOpOf(storageDesc(v)), slot)
		code.FieldInsn(cf.PUTFIELD, name, lambdaField(v), storageDesc(v))
		if (kprop != nil && kprop.Bound || x.KFun != nil) && v == x.Ref.Bound {
			bound = slot
		}
	}
	code.Local(cf.ALOAD, 0)
	switch {
	case kprop != nil:
		g.kpropInit(code, x, super, bound)
	case adapted:
		code.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	case x.KFun != nil:
		g.kfunInit(code, x, super, bound)
	case varArity(x):
		code.Iconst(int32(len(in)))
		code.Invoke(cf.INVOKESPECIAL, super, "<init>", "(I)V")
	default:
		code.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	}
	code.Op(cf.RETURN)
	finishCode(code)
	if single {
		c.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal, "INSTANCE", self)
		code := c.NewMethod(cf.AccStatic, "<clinit>", "()V")
		code.New(name)
		code.Op(cf.DUP)
		code.Invoke(cf.INVOKESPECIAL, name, "<init>", "()V")
		code.FieldInsn(cf.PUTSTATIC, name, "INSTANCE", self)
		code.Op(cf.RETURN)
		finishCode(code)
	}

	// The interface's method: the lambda's body.
	unit := x.SAM == nil && kprop == nil && strings.HasPrefix(iface, types.KotlinFunctionPackage)
	reified := false
	g.classMethod(c, x, method, samDesc, in, out, unit, &reified)
	if kprop != nil && kprop.Set != nil {
		set := kprop.Set
		n := len(set.ParamSyms())
		in := make([]types.Type, n)
		for i := range in {
			in[i] = types.AnyType.Nullable()
		}
		g.classMethod(c, set, "set", "("+strings.Repeat("Ljava/lang/Object;", n)+")V", in, types.UnitType, false, &reified)
	}
	if adapted {
		ag := &gen{file: g.file, cls: c, owner: g.owner, shared: g.shared, noLines: 1}
		ag.funAdapter(c, x, lambdaField)
	}
	h := kmeta.WriteSynthetic()
	if x.KFun != nil {
		h.XI = kmeta.XILambda // as kotlinc's function references
	}
	g.localClass(c, g.cls, g.code.Name(), g.code.Desc(), x.P.Line, g.inlineXI(h, kmeta.XIInlineLambda))
	if !adapted {
		// Public, as kotlinc's: a copier that needn't copy the class
		// creates it where it copies the function.
		c.InnerClasses[len(c.InnerClasses)-1].Access |= cf.AccPublic
		g.cls.InnerClasses[len(g.cls.InnerClasses)-1].Access |= cf.AccPublic
	}
	if reified && g.reifiedUse != nil {
		*g.reifiedUse = true // copies of the enclosing class create copies of this one
	}

	wrap := g.composeLambdaWrap(x)
	if reified {
		g.code.Invoke(cf.INVOKESTATIC, intrinsics, "needClassReification", "()V")
	}
	// The cast to the interface, as kotlinc's, ends the instructions that
	// kotlinc's inliner recognizes as the creation of a default value's
	// lambda.
	if single {
		g.code.FieldInsn(cf.GETSTATIC, name, "INSTANCE", self)
		g.code.Checkcast(iface)
		wrap()
		return
	}
	g.code.New(name)
	g.code.Op(cf.DUP)
	g.loadFrame(x)
	g.code.Invoke(cf.INVOKESPECIAL, name, "<init>", ctor)
	if !x.Composable {
		g.code.Checkcast(iface)
	}
	wrap()
}

// funAdapter writes the FunctionAdapter methods of class c, the object of
// reference x converted to a fun interface: getFunctionDelegate returns a
// new FunctionReference to the function, with the receiver of x::f that
// c holds in the field that field names, and equals and hashCode compare
// them.
func (g *gen) funAdapter(c *cf.Class, x *syntax.Lambda, field func(*types.VarSym) string) {
	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "getFunctionDelegate", "()Lkotlin/Function;")
	c.LastMethod().SetSignature("()Lkotlin/Function<*>;")
	g.code.AllocLocal("L" + c.Name + ";")
	super := kfunSuper(x)
	g.code.New(super)
	g.code.Op(cf.DUP)
	var load func()
	if v := x.Ref.Bound; v != nil {
		load = func() {
			g.code.Local(cf.ALOAD, 0)
			g.code.FieldInsn(cf.GETFIELD, c.Name, field(v), storageDesc(v))
			g.boxReceiver(v.Type)
		}
	}
	g.kfunArgs(g.code, x, super, load)
	g.code.Checkcast("kotlin/Function")
	g.code.Op(cf.ARETURN)
	g.check()
	g.adapterEquality(c, x.SAM.Owner.Name)
}

// lambdaField returns the name of the field of a lambda's class that
// holds captured variable v, as kotlinc names it: $x for a local x, and
// $this_f for the receiver of extension function f, whose local is
// $this$f.
func lambdaField(v *types.VarSym) string {
	return "$" + strings.ReplaceAll(strings.TrimPrefix(v.Name, "$"), "$", "_")
}

// classMethod generates a method of a lambda's class, c: l's body,
// which is the lambda's or a property reference's setter, whose captures
// are among the getter's. It takes in and returns out, erased types, and
// reads the captures from c's fields.
func (g *gen) classMethod(c *cf.Class, l *syntax.Lambda, method, samDesc string, in []types.Type, out types.Type, unit bool, reified *bool) {
	name, self := c.Name, "L"+c.Name+";"
	lc := &lambdaCtx{slots: map[*types.VarSym]int{}, out: out, unit: unit, class: true}
	lg := &gen{file: g.file, cls: c, owner: g.thisClass(), shared: g.shared, prefix: "invoke$lambda", lam: lc, noLines: g.noLines,
		publicInline: true, reifiedUse: reified, lambdaReified: g.reifiedSubst()}
	lg.code = c.NewMethod(cf.AccPublic|cf.AccFinal, method, samDesc)
	lg.retType = l.Ret
	code := lg.code
	code.AllocLocal(self)
	syms := l.ParamSyms()
	args := -1 // FunctionN's invoke takes an array
	if samDesc == types.VarArityDesc && len(in) > types.MaxArity {
		args = code.AllocLocal("[Ljava/lang/Object;")
	}
	slots := make([]int, len(in)) // and a composable's Composer and change flags
	for i := range in {
		slots[i] = code.AllocLocal(desc(in[i]))
	}
	if args >= 0 {
		spreadArgs(code, args, slots)
	}
	if l.This {
		owner := "L" + g.thisClass().Name + ";"
		lc.this = code.AllocLocal(owner)
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, "this$0", owner)
		code.Local(cf.ASTORE, lc.this)
	}
	for _, v := range l.Captures {
		slot := code.AllocLocal(storageDesc(v))
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, lambdaField(v), storageDesc(v))
		code.Local(storeOpOf(storageDesc(v)), slot)
		lc.slots[v] = slot
	}
	for i, p := range syms {
		boxed := types.Unboxed(p.Type) && !types.IsValue(in[i])
		// A Unit parameter is never read: a Unit value is nothing on
		// the stack. A Nothing or Nothing? one, as for a star
		// projection's type argument, is null, which needs no cast.
		unit := p.Type.K == types.Unit && !p.Type.N || p.Type.K == types.Null || p.Type.K == types.Nothing
		if desc(in[i]) == desc(p.Type) && !boxed || unit {
			p.Slot = slots[i]
			lg.named(p)
			continue
		}
		code.Local(loadOp(in[i]), slots[i])
		if in[i].IsPrimitive() {
			lg.coerce(in[i], p.Type)
		} else {
			lg.castErased(in[i], p.Type)
		}
		p.Slot = code.AllocLocal(desc(p.Type))
		lg.named(p)
		code.Local(storeOp(p.Type), p.Slot)
	}
	if l.Composable {
		lg.composeLambdaStart(l, slots[len(syms):])
	}
	if l.KFun != nil && l.Suspend && l.KFun.Sym.Suspend {
		// A reference to a suspend function, whose invoke passes the
		// function its continuation, the last parameter, and returns
		// what it returns, as a suspend function in tail form does.
		lg.sm = &stateMachine{completion: slots[len(slots)-1], ret: types.AnyType.Nullable(), tails: map[*syntax.Call]bool{l.KFun: true}}
		lg.expr(l.KFun)
		lg.check()
		return
	}
	if l.Ret.K == types.Unit {
		lg.block(l.Body, types.UnitType, false)
		if code.Reachable() {
			lg.line(l.Body.End)
			lg.lambdaReturn(nil)
		}
		if l.Composable {
			lg.composeLambdaSkip()
		}
	} else {
		lg.block(l.Body, l.Ret, true)
		lg.coerce(l.Ret, out)
		lg.composeLambdaEnd()
		code.Op(returnOp(out))
	}
	lg.check()
}
