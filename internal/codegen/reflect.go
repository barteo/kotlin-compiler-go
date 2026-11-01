package codegen

import (
	"strconv"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// classLit pushes a class literal: the java.lang.Class of X::class.java,
// and for X::class, its KClass from the standard library.
func (g *gen) classLit(x *syntax.ClassLit) {
	t := g.reify(x.Class)
	switch {
	case !x.Bound && g.reifiedParam(t):
		// T::class of a reified T: its copies' type argument's class.
		g.reifiedMarker(t, reifiedJavaClass)
		g.code.LdcClass(types.ClassName(types.Erase(t)))
	case x.Class.IsParam() && !x.Bound && t.IsPrimitive():
		// A reified type parameter's class is a box, as with kotlinc:
		// T::class for Int is Integer's.
		g.code.LdcClass(boxClass[t.K])
	case x.Bound && t.IsPrimitive():
		// A primitive's class is the static one, int.class, as with kotlinc.
		g.expr(x.X)
		g.pop(x.X.Type())
		g.code.FieldInsn(cf.GETSTATIC, boxClass[t.K], "TYPE", "Ljava/lang/Class;")
	case x.Bound:
		// The run-time class of a value.
		g.expr(x.X)
		if xt := x.X.Type(); types.Unboxed(xt) {
			g.boxValue(xt)
		} else {
			g.coerce(xt, t)
		}
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/Object", "getClass", "()Ljava/lang/Class;")
	case t.IsPrimitive():
		// Int::class.java is int.class.
		g.code.FieldInsn(cf.GETSTATIC, boxClass[t.K], "TYPE", "Ljava/lang/Class;")
	case t.K == types.Unit:
		g.code.LdcClass("kotlin/Unit")
	case t.K == types.Nothing:
		g.code.LdcClass("java/lang/Void")
	default:
		g.code.LdcClass(types.ClassName(types.Erase(t)))
	}
	if !x.Java {
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/Reflection", "getOrCreateKotlinClass",
			"(Ljava/lang/Class;)Lkotlin/reflect/KClass;")
	}
}

// typeOf pushes the KType of t, for typeOf<T>(), as kotlinc builds it:
// Reflection.typeOf (or nullableTypeOf) of t's class and the projections
// of its type arguments, and mutableCollectionType around a mutable
// collection's. In the body of an inline function, typeOf<T>() of its
// reified T is kotlinc's marker and a null that copies replace; another
// type with T in it throws. A type parameter that isn't reified is a
// KTypeParameter, as typeParam builds it.
func (g *gen) typeOf(t types.Type) { g.ktype(t, false) }

// ktype pushes the KType of t, a type argument of typeOf or, if bound,
// the bound of a type parameter, whose reified type parameters are
// KTypeParameters too.
func (g *gen) ktype(t types.Type, bound bool) {
	const refl, proj = "kotlin/jvm/internal/Reflection", "kotlin/reflect/KTypeProjection"
	if !bound && g.reifiedParam(t) {
		g.reifiedMarker(t, reifiedTypeOf)
		g.code.Op(cf.ACONST_NULL)
		return
	}
	if t.IsParam() && t.Param().Reified && !bound {
		const exc = "java/lang/UnsupportedOperationException"
		g.code.New(exc)
		g.code.Op(cf.DUP)
		g.code.Sconst("This function has a reified type parameter and thus can only be inlined at compilation time, not called directly.")
		g.code.Invoke(cf.INVOKESPECIAL, exc, "<init>", "(Ljava/lang/String;)V")
		g.code.Op(cf.ATHROW)
		return
	}
	if t.IsParam() {
		// T? is nullable, and T & Any, as with kotlinc, T.
		g.typeParam(t.Param())
		name := "typeOf"
		if t.N {
			name = "nullableTypeOf"
		}
		g.code.Invoke(cf.INVOKESTATIC, refl, name, "(Lkotlin/reflect/KClassifier;)Lkotlin/reflect/KType;")
		return
	}
	nn := t.NonNull()
	switch {
	case nn.IsPrimitive() && t.N:
		g.code.LdcClass(boxClass[nn.K]) // Int? is Integer's
	case nn.IsPrimitive():
		g.code.FieldInsn(cf.GETSTATIC, boxClass[nn.K], "TYPE", "Ljava/lang/Class;")
	case nn.K == types.Unit:
		g.code.LdcClass("kotlin/Unit")
	case nn.K == types.Nothing:
		g.code.LdcClass("java/lang/Void")
	case nn.K == types.Any:
		g.code.LdcClass("java/lang/Object")
	default:
		g.code.LdcClass(types.ClassName(types.Erase(nn)))
	}
	var args []types.Arg
	if nn.K == types.Class && nn.A != nil {
		args = nn.A.List
	}
	for _, a := range args {
		g.code.FieldInsn(cf.GETSTATIC, proj, "Companion", "L"+proj+"$Companion;")
		if a.V == types.Star {
			g.code.Invoke(cf.INVOKEVIRTUAL, proj+"$Companion", "getSTAR", "()L"+proj+";")
			continue
		}
		g.ktype(a.T, bound)
		name := map[types.Variance]string{types.Inv: "invariant", types.Out: "covariant", types.In: "contravariant"}[a.V]
		g.code.Invoke(cf.INVOKEVIRTUAL, proj+"$Companion", name, "(Lkotlin/reflect/KType;)L"+proj+";")
	}
	name := "typeOf"
	if types.MayBeNull(t) {
		name = "nullableTypeOf"
	}
	if len(args) > 2 {
		// The projections, in an array.
		n := len(args)
		arr := g.code.AllocLocal("[L" + proj + ";")
		g.code.Iconst(int32(n))
		g.code.ANewArray(proj)
		g.code.Local(cf.ASTORE, arr)
		for i := n - 1; i >= 0; i-- {
			tmp := g.code.AllocLocal("L" + proj + ";")
			g.code.Local(cf.ASTORE, tmp)
			g.code.Local(cf.ALOAD, arr)
			g.code.Iconst(int32(i))
			g.code.Local(cf.ALOAD, tmp)
			g.code.Op(cf.AASTORE)
		}
		g.code.Local(cf.ALOAD, arr)
		g.code.Invoke(cf.INVOKESTATIC, refl, name, "(Ljava/lang/Class;[L"+proj+";)Lkotlin/reflect/KType;")
	} else {
		d := "(Ljava/lang/Class;"
		for range args {
			d += "L" + proj + ";"
		}
		g.code.Invoke(cf.INVOKESTATIC, refl, name, d+")Lkotlin/reflect/KType;")
	}
	if nn.K == types.Class && nn.C.ReadOnly != nil {
		g.code.Invoke(cf.INVOKESTATIC, refl, "mutableCollectionType", "(Lkotlin/reflect/KType;)Lkotlin/reflect/KType;")
	}
}

// typeParam pushes the KTypeParameter of p, as kotlinc builds it for
// typeOf: Reflection.typeParameter of p's container, name, variance and
// whether it is reified, then setUpperBounds with its bounds, Any? if it
// has none. Bounds can't refer back to p: the checker rejects that.
func (g *gen) typeParam(p *types.TypeParamSym) {
	const refl = "kotlin/jvm/internal/Reflection"
	g.typeParamOwner(p)
	g.code.Sconst(p.Name)
	v := map[types.Variance]string{types.Inv: "INVARIANT", types.In: "IN", types.Out: "OUT"}[p.Var]
	g.code.FieldInsn(cf.GETSTATIC, "kotlin/reflect/KVariance", v, "Lkotlin/reflect/KVariance;")
	reified := int32(0)
	if p.Reified {
		reified = 1
	}
	g.code.Iconst(reified)
	g.code.Invoke(cf.INVOKESTATIC, refl, "typeParameter",
		"(Ljava/lang/Object;Ljava/lang/String;Lkotlin/reflect/KVariance;Z)Lkotlin/reflect/KTypeParameter;")
	g.code.Op(cf.DUP)
	switch bs := p.Bounds; len(bs) {
	case 0:
		g.code.LdcClass("java/lang/Object")
		g.code.Invoke(cf.INVOKESTATIC, refl, "nullableTypeOf", "(Ljava/lang/Class;)Lkotlin/reflect/KType;")
	case 1:
		g.ktype(bs[0], true)
	default:
		g.code.Iconst(int32(len(bs)))
		g.code.ANewArray("kotlin/reflect/KType")
		for i, b := range bs {
			g.code.Op(cf.DUP)
			g.code.Iconst(int32(i))
			g.ktype(b, true)
			g.code.Op(cf.AASTORE)
		}
		g.code.Invoke(cf.INVOKESTATIC, refl, "setUpperBounds", "(Lkotlin/reflect/KTypeParameter;[Lkotlin/reflect/KType;)V")
		return
	}
	g.code.Invoke(cf.INVOKESTATIC, refl, "setUpperBounds", "(Lkotlin/reflect/KTypeParameter;Lkotlin/reflect/KType;)V")
}

// typeParamOwner pushes the container of type parameter p, as kotlinc:
// its class's KClass, or a reference to its function or extension
// property, as ::f and ::p are without their receivers.
func (g *gen) typeParamOwner(p *types.TypeParamSym) {
	f, ok := p.Owner.(*types.FunSym)
	if !ok {
		g.code.LdcClass(p.Owner.(*types.ClassSym).Name)
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/Reflection", "getOrCreateKotlinClass",
			"(Ljava/lang/Class;)Lkotlin/reflect/KClass;")
		return
	}
	recvs := int32(len(f.Params))
	if f.Owner != nil {
		recvs++
	}
	if f.Ext {
		recvs++
	}
	if fd, ok := f.Decl.(*syntax.FunDecl); ok && fd.AccessorOf != nil {
		// An extension property's: PropertyReference1Impl, or 2 for a
		// member's, Mutable for a var's.
		owner, sig, flags := g.kfunOf(f)
		cls := "kotlin/jvm/internal/PropertyReference" + strconv.Itoa(int(recvs)) + "Impl"
		if f.ExtSetter != nil {
			cls = "kotlin/jvm/internal/MutablePropertyReference" + strconv.Itoa(int(recvs)) + "Impl"
		}
		g.code.New(cls)
		g.code.Op(cf.DUP)
		g.code.LdcClass(owner)
		g.code.Sconst(fd.AccessorOf.Name)
		g.code.Sconst(sig)
		g.code.Iconst(flags)
		g.code.Invoke(cf.INVOKESPECIAL, cls, "<init>", "(Ljava/lang/Class;Ljava/lang/String;Ljava/lang/String;I)V")
		return
	}
	owner, name, sig, flags := "", f.Name, "", int32(0)
	if lf := localOf(f); lf != nil {
		// A local function's own method, unlike ::f's.
		owner, name, sig, flags = lf.Owner, lf.Method, lf.Method+g.localDesc(lf), 1
	} else {
		owner, sig, flags = g.kfunOf(f)
	}
	const impl = "kotlin/jvm/internal/FunctionReferenceImpl"
	g.code.New(impl)
	g.code.Op(cf.DUP)
	g.code.Iconst(recvs)
	g.code.LdcClass(owner)
	g.code.Sconst(name)
	g.code.Sconst(sig)
	g.code.Iconst(flags)
	g.code.Invoke(cf.INVOKESPECIAL, impl, "<init>", "(ILjava/lang/Class;Ljava/lang/String;Ljava/lang/String;I)V")
}

// refReceiver evaluates the receiver of a bound callable reference into
// the local its lambda reads it from.
func (g *gen) refReceiver(l *syntax.Lambda) {
	r := l.Ref
	if r == nil || r.Bound == nil {
		return
	}
	v := r.Bound
	g.setSlot(v, g.code.AllocLocal(desc(v.Type)))
	g.exprAs(r.BoundExpr, v.Type)
	g.code.Local(storeOp(v.Type), g.slot(v))
}
