package codegen

import (
	"fmt"
	"slices"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

var boxClass = types.Boxes

func (g *gen) box(t types.Type) {
	c := boxClass[t.K]
	g.code.Invoke(cf.INVOKESTATIC, c, "valueOf", "("+desc(t.NonNull())+")L"+c+";")
}

var unboxNames = map[types.Kind]string{
	types.Int: "intValue", types.Long: "longValue", types.Double: "doubleValue", types.Bool: "booleanValue",
	types.Char: "charValue", types.Float: "floatValue", types.Byte: "byteValue", types.Short: "shortValue",
}

func (g *gen) unbox(t types.Type) {
	nn := t.NonNull()
	g.code.Invoke(cf.INVOKEVIRTUAL, boxClass[t.K], unboxNames[t.K], "()"+desc(nn))
}

// coerce converts the value on the stack from type from to type to.
// The checker guarantees from is assignable to to, a smart cast of it,
// or a numeric type that widens to it.
func (g *gen) coerce(from, to types.Type) {
	if from.K == types.Nothing && !from.N {
		// A call that returns Nothing never returns: on the JVM it has a
		// result, a java.lang.Void, but the code after it is unreachable.
		if g.code.Reachable() {
			g.nothingValue(false)
		}
		return
	}
	if from.K == types.Null && to.K != types.Null && size(to) > 0 && g.code.Reachable() && g.code.StackTop().Tag() == cf.VObject {
		// A generic call's result inferred as Nothing?: the erased value,
		// null, needs to's class.
		if d := desc(to); d != objectDesc && !to.IsPrimitive() {
			g.code.Checkcast(className(d))
		}
		return
	}
	if to.K == types.Null && g.code.Reachable() {
		// A Nothing? value, null, as a java.lang.Void: a parameter of
		// type Nothing? is one.
		if top := g.code.StackTop(); top.Tag() == cf.VObject && top.Class() != "java/lang/Void" {
			g.code.Checkcast("java/lang/Void")
		}
	}
	if from == to || from.K == types.Null || from.K == types.Invalid {
		return
	}
	if from.K == types.Unit && size(to) > 0 {
		// Unit as a value, as a generic call's argument: kotlin.Unit.
		if g.shared.opts.Stdlib {
			g.code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
		} else {
			g.code.Op(cf.ACONST_NULL)
		}
		return
	}
	if to.K == types.Unit {
		g.pop(from)
		return
	}
	if to.K == types.Nothing {
		// A generic call inferred to return Nothing, as run { throw e }:
		// its erased result is never produced.
		g.pop(from)
		g.nothingValue(false)
		return
	}
	if types.Unboxed(from) || types.Unboxed(to) {
		g.coerceValue(from, to)
		return
	}
	if from.IsParam() {
		// The value has from's erasure, which may be wider than to.
		if to.IsParam() {
			to = types.Erase(to)
		}
		g.castErased(types.Erase(from), to)
		return
	}
	if from.IsArray() && to.IsArray() && from.HasParams() {
		// An Array<T> is an array of T's erasure: Object[] for Array<Int>.
		if d := desc(to); d != desc(from) {
			g.code.Checkcast(d)
		}
		return
	}
	fp, tp := from.IsPrimitive(), to.IsPrimitive()
	switch {
	case fp && tp:
		g.convert(from, to)
	case fp:
		g.box(from)
	case tp:
		if boxClass[from.K] != "" {
			g.unbox(from)
			g.coerce(from.NonNull(), to) // Int! to Long
		}
	}
}

// castErased converts a value read as the erasure of a type parameter to
// type to: a cast to its class, then unboxing for primitives.
func (g *gen) castErased(erased, to types.Type) {
	if to.K == types.Nothing || to.K == types.Null || to.K == types.Any {
		return
	}
	if types.IsValue(to) {
		g.castBox(erased, to)
		if types.Unboxed(to) {
			g.unboxValue(to)
		}
		return
	}
	ref := to.Nullable()
	if to.IsPrimitive() && desc(ref) != desc(erased) {
		// As kotlinc: a Long read as an Int? is unboxed by Number.intValue.
		switch to.K {
		case types.Bool, types.Char:
			g.code.Checkcast(boxClass[to.K])
			g.unbox(to)
		default:
			d := desc(to)
			if desc(erased) != "Ljava/lang/Number;" {
				g.code.Checkcast("java/lang/Number")
			}
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/Number", numberValue[d], "()"+d)
		}
		return
	}
	if d := desc(ref); d != desc(erased) && g.code.Reachable() {
		top := g.code.StackTop()
		at := g.code.Pos()
		g.code.Checkcast(className(d))
		g.erasedCast = erasedCast{g.code, at, top, erased}
	}
	if to.IsPrimitive() {
		g.unbox(to)
	}
}

// erasedCast is a checkcast that castErased emitted at offset at of
// code, of a value of type top on the stack, the erasure erased.
type erasedCast struct {
	code   *cf.Code
	at     int
	top    cf.VType
	erased types.Type
}

// uncastErased removes the checkcast of a generic call's result or
// another erased value if it is the last instruction: kotlinc casts the
// value only where it is used. It returns the erasure the value is left
// as, or false.
func (g *gen) uncastErased() (types.Type, bool) {
	e := g.erasedCast
	if e.code != g.code || !g.code.Uncast(e.at, e.top) {
		return types.Type{}, false
	}
	g.erasedCast = erasedCast{}
	return e.erased, true
}

// exprAs generates x and converts it to type t.
func (g *gen) exprAs(x syntax.Expr, t types.Type) {
	if x.Type().P && checked(t) {
		// A platform value used as a non-null type is checked in each
		// branch, where it has its own text, as with kotlinc.
		switch x := x.(type) {
		case *syntax.If:
			g.ifValue(x, t)
			return
		case *syntax.When:
			g.when(x, t, true)
			return
		}
	}
	g.expr(x)
	g.checkPlatform(x, t)
	g.coerce(x.Type(), t)
}

// coerceRecv is coerce for the receiver of an extension: an extension of
// Unit, Unit.f(), takes kotlin.Unit, which a Unit expression doesn't leave.
func (g *gen) coerceRecv(from, to types.Type) {
	if from.K == types.Unit && to.K == types.Unit && !to.N {
		g.code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
		return
	}
	g.coerce(from, to)
}

// ifValue generates an if expression, leaving its value as type t.
func (g *gen) ifValue(x *syntax.If, t types.Type) {
	els, end := cf.NewLabel(), join(t)
	g.cond(x.Cond, els, false)
	g.grouped(x.Groups, g.siteKey(x.P, "if", 0), func() { g.block(x.Then, t, true) })
	g.code.Jump(cf.GOTO, end)
	g.code.Place(els)
	g.grouped(x.Groups, g.siteKey(x.P, "if", 1), func() { g.block(x.Else, t, true) })
	g.code.Place(end)
}

// expr generates x, leaving a value of type x.Type() on the stack
// (nothing for Unit and Nothing).
func (g *gen) expr(x syntax.Expr) {
	switch x := x.(type) {
	case *syntax.Ident:
		g.ident(x)
	case *syntax.This:
		if x.Recv != nil {
			if x.Recv.Type.K == types.Unit && !x.Recv.Type.N && !x.Cast {
				return // a Unit value is nothing on the stack
			}
			// A lambda's receiver, its first parameter.
			g.loadStorage(x.Recv)
			if x.Cast {
				g.smartCast(x.Recv.Type, x.T)
			} else {
				g.coerce(x.Recv.Type, x.T)
			}
			return
		}
		g.loadThis()
		if x.Cast {
			g.smartCast(g.owner.This(), x.T)
		}
	case *syntax.Is:
		g.expr(x.X)
		g.instanceOf(x.X.Type(), x.Target)
		if x.Not {
			g.code.Iconst(1)
			g.code.Op(cf.IXOR)
		}
	case *syntax.As:
		g.as(x)
	case *syntax.Try:
		g.try(x, x.T, true)
	case *syntax.Lambda:
		g.refReceiver(x)
		if x.Inline {
			// Inlined where the function invokes it: the argument is unused.
			g.code.Op(cf.ACONST_NULL)
			return
		}
		g.lambda(x)
	case *syntax.IntLit:
		if r := types.Repr(x.T); r.K == types.Long {
			g.code.Lconst(x.Val) // also a ULong's bits
		} else {
			g.code.Iconst(int32(x.Val))
		}
	case *syntax.DoubleLit:
		g.code.Dconst(x.Val)
	case *syntax.FloatLit:
		g.code.Fconst(x.Val)
	case *syntax.CharLit:
		g.code.Iconst(int32(x.Val))
	case *syntax.BoolLit:
		if x.Val {
			g.code.Iconst(1)
		} else {
			g.code.Iconst(0)
		}
	case *syntax.NullLit:
		g.code.Op(cf.ACONST_NULL)
	case *syntax.StringLit:
		g.stringLit(x)
	case *syntax.Checked:
		if x.SAM != nil {
			g.samWrap(x)
			break
		}
		g.expr(x.X)
		if from := x.X.Type(); from.K == types.TypeParam && x.T.K != types.TypeParam {
			g.coerce(from, x.T) // T : Int as an Int: see builtinBound
		}
	case *syntax.Unary:
		if x.Call != nil {
			g.call(x.Call) // an operator function
			return
		}
		switch x.Op {
		case syntax.MINUS:
			g.exprAs(x.X, x.T)
			g.code.Op(negOp(x.T))
		case syntax.PLUS:
			g.exprAs(x.X, x.T)
		case syntax.INC, syntax.DEC:
			if n := len(x.Stmts); n > 0 {
				// x = x.inc(), then the value.
				g.stmts(x.Stmts[:n-1])
				g.expr(x.Stmts[n-1].(*syntax.ExprStmt).X)
				break
			}
			g.incDecValue(x)
		default:
			g.boolValue(x)
		}
	case *syntax.Binary:
		g.binary(x)
	case *syntax.Member:
		g.member(x)
	case *syntax.ClassLit:
		g.classLit(x)
	case *syntax.Call:
		g.call(x)
	case *syntax.Index:
		g.call(x.Call)
	case *syntax.NotNull:
		g.notNull(x)
	case *syntax.If:
		g.ifValue(x, x.T)
	case *syntax.When:
		g.when(x, x.T, true)
	case *syntax.Jump:
		g.jump(x)
	case *syntax.PropRef:
		g.propRef(x)
	case *syntax.ObjectExpr:
		g.objectExpr(x)
	default:
		panic(fmt.Sprintf("unsupported expression %T", x))
	}
}

func (g *gen) ident(x *syntax.Ident) {
	if x.Unit {
		return // a Unit value is nothing on the stack
	}
	if x.Call != nil {
		g.call(x.Call) // a property with context parameters: getP()
		return
	}
	if l, _ := g.forwarded(x); l != nil {
		// An inlined lambda passed on to an inline function, which
		// inlines it: the argument is unused.
		g.code.Op(cf.ACONST_NULL)
		return
	}
	if x.Object != nil {
		g.objectRef(x.Object)
		return
	}
	if x.Member != nil {
		// A member of this's smart-cast type.
		g.member(x.Member)
		return
	}
	if f := x.Getter; f != nil {
		// A property inherited from a Java or Kotlin class.
		g.loadThis()
		g.invokeJava(f, g.thisClass().This())
		g.coerce(types.JVMRet(f), x.T)
		return
	}
	sym := x.Sym
	if sym.Delegate != nil {
		// A local delegated property: its delegate's getValue.
		get := sym.Decl.(*syntax.VarStmt).Get
		g.expr(get)
		g.coerce(get.T, x.T)
		return
	}
	switch sym.Kind {
	case types.Local, types.Param:
		if sym.Type.K == types.Unit && !sym.Type.N && !sym.Boxed() {
			break // a Unit value is nothing on the stack: it of a Flow<Unit>'s collect { }
		}
		g.loadStorage(sym)
		if sym.Boxed() {
			g.code.Iconst(0)
			g.arrayLoad(sym.Type)
		}
		g.lateinit(sym)
	case types.Global:
		g.global(sym)
	case types.Static:
		if sym.Constant != nil {
			// A Java constant, which kotlinc inlines as javac does.
			g.pushConst(sym.Constant, sym.Type)
			break
		}
		g.code.FieldInsn(cf.GETSTATIC, sym.Owner.Name, sym.Name, desc(sym.Type))
	case types.BackingField:
		g.backingField(sym.Prop, true)
	case types.Prop:
		if sym.Const {
			g.constProp(sym)
			break
		}
		if sym.StaticField() && g.directGet(sym) {
			g.code.FieldInsn(cf.GETSTATIC, sym.ConstOwner().Name, sym.Name, desc(types.FieldType(sym)))
			g.lateinit(sym)
			g.coerce(types.FieldType(sym), sym.Type)
			break
		}
		g.loadThis()
		if sym.Owner.Value && types.Unboxed(g.owner.This()) && g.owner == sym.Owner {
			g.valueGet(sym)
		} else if g.directGet(sym) {
			g.code.FieldInsn(cf.GETFIELD, sym.Owner.Name, sym.Name, desc(types.FieldType(sym)))
			g.lateinit(sym)
			g.coerce(types.FieldType(sym), sym.Type)
		} else {
			g.getProp(g.thisClass(), sym)
		}
	}
	if x.Cast {
		g.smartCast(sym.Type, x.T)
		return
	}
	g.coerce(sym.Type, x.T) // smart cast from T? to T
}

// smartCast converts a value read as type from to the type a smart cast
// gives it, of another class: a cast, and unboxing for a primitive.
func (g *gen) smartCast(from, to types.Type) {
	if types.Unboxed(to) {
		if !types.Unboxed(from) {
			g.castBox(from, to)
			g.unboxValue(to)
		}
		return
	}
	if to.IsPrimitive() {
		if from.IsPrimitive() {
			g.coerce(from, to)
			return
		}
		if box := boxClass[to.K]; desc(from) != "L"+box+";" {
			g.code.Checkcast(box)
		}
		g.unbox(to)
		return
	}
	if d := desc(to.Nullable()); d != desc(from) {
		g.code.Checkcast(className(d))
	}
}

// instanceClass returns the class that values of type t, not a type
// parameter, are instances of on the JVM: a box class for primitives.
func instanceClass(t types.Type) string {
	if c := boxClass[t.K]; c != "" {
		return c
	}
	if types.IsValue(t) || t.K == types.Unit {
		return types.ClassName(t)
	}
	return className(desc(t.Nullable()))
}

// instanceOf replaces the value on the stack, of type from, with whether
// it is an instance of t, 1 or 0. For a nullable t, null is one.
func (g *gen) instanceOf(from, t types.Type) {
	t = g.reify(t)
	switch {
	case from.IsPrimitive():
		g.box(from)
	case types.Unboxed(from):
		g.boxValue(from)
	case from.K == types.Unit:
		g.coerce(from, types.AnyType) // kotlin.Unit
	}
	cls := instanceClass(t)
	if g.reifiedParam(t) {
		// The copies check the type argument, null included.
		g.reifiedMarker(t, reifiedIs)
		g.code.InstanceOf(cls)
		return
	}
	if !types.MayBeNull(t) {
		g.isInstance(t, cls)
		return
	}
	notNull, end := cf.NewLabel(), cf.NewLabel()
	g.code.Op(cf.DUP)
	g.code.Jump(cf.IFNONNULL, notNull)
	g.code.Op(cf.POP)
	g.code.Iconst(1)
	g.code.Jump(cf.GOTO, end)
	g.code.Place(notNull)
	g.isInstance(t, cls)
	g.code.Place(end)
}

// mutableIntrinsics are the names TypeIntrinsics' functions give the
// mutable collection types: isMutableList and asMutableList check that a
// List isn't a read-only Kotlin one, which is a KMappedMarker but not a
// KMutableList.
var mutableIntrinsics = map[string]string{
	"kotlin/collections/MutableIterable":         "MutableIterable",
	"kotlin/collections/MutableCollection":       "MutableCollection",
	"kotlin/collections/MutableList":             "MutableList",
	"kotlin/collections/MutableSet":              "MutableSet",
	"kotlin/collections/MutableMap":              "MutableMap",
	"kotlin/collections/MutableMap.MutableEntry": "MutableMapEntry",
	"kotlin/collections/MutableIterator":         "MutableIterator",
	"kotlin/collections/MutableListIterator":     "MutableListIterator",
}

// mutableIntrinsic returns TypeIntrinsics' name for t if it is a
// mutable collection type, or "".
func mutableIntrinsic(t types.Type) string {
	if t.K != types.Class || t.C.ReadOnly == nil {
		return ""
	}
	return mutableIntrinsics[t.C.KotlinName]
}

// isInstance replaces the object on the stack with whether it is an
// instance of t, whose JVM class is cls.
func (g *gen) isInstance(t types.Type, cls string) {
	if m := mutableIntrinsic(t); m != "" {
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/TypeIntrinsics", "is"+m, "(Ljava/lang/Object;)Z")
		return
	}
	if n, ok := functionArity(t); ok {
		g.code.Iconst(int32(n))
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/TypeIntrinsics", "isFunctionOfArity", "(Ljava/lang/Object;I)Z")
		return
	}
	if t.IsSuspendFunc() {
		// As with kotlinc, a SuspendFunction whose arity counts the
		// continuation.
		no, end := cf.NewLabel(), cf.NewLabel()
		g.code.Op(cf.DUP)
		g.code.InstanceOf("kotlin/coroutines/jvm/internal/SuspendFunction")
		g.code.Jump(cf.IFEQ, no)
		g.code.Iconst(int32(t.A.Len()))
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/TypeIntrinsics", "isFunctionOfArity", "(Ljava/lang/Object;I)Z")
		g.code.Jump(cf.GOTO, end)
		g.code.Place(no)
		g.code.Op(cf.POP)
		g.code.Iconst(0)
		g.code.Place(end)
		return
	}
	g.code.InstanceOf(cls)
}

// functionArity returns the number of parameters of a function type of
// the standard library's. As with kotlinc, TypeIntrinsics checks and casts
// to it, comparing a FunctionBase's arity too.
func functionArity(t types.Type) (int, bool) {
	if t.K != types.Class || !t.C.Func || t.C.Suspend || !strings.HasPrefix(t.C.Name, types.KotlinFunctionPackage) {
		return 0, false
	}
	return len(t.C.TypeParams) - 1, true
}

// as generates x as T and x as? T. A cast to a non-null type throws a
// NullPointerException for null, as with kotlinc. A cast to a type
// parameter is unchecked: it casts to the parameter's erasure.
func (g *gen) as(x *syntax.As) {
	from, to := x.X.Type(), g.reify(x.Target)
	switch {
	case x.Target.IsParam() && to.IsPrimitive():
		// In a copy of an inline function, where values of type T are
		// boxed, as in seed as T with T Int.
		to = to.Nullable()
	case x.Target.IsParam() && types.Unboxed(to):
		to = types.Boxed(to) // a value class's, boxed too
	}
	g.expr(x.X)
	if from.K == types.Unit {
		g.coerce(from, types.AnyType) // kotlin.Unit
		from = types.AnyType
	}
	if from.IsPrimitive() {
		if from == to {
			g.coerce(from, x.T) // boxed for as?
			return
		}
		g.box(from)
		from = from.Nullable()
	}
	if types.Unboxed(from) {
		if from == to {
			g.coerce(from, x.T)
			return
		}
		g.boxValue(from)
		from = types.AnyType
		if types.MayBeNull(x.X.Type()) {
			from = from.Nullable()
		}
	}
	if g.reifiedParam(to) && x.Safe && g.lam != nil && g.lam.class {
		// In a lambda's class, which copiers copy with the type argument
		// in place of the marked operations' classes: an instanceof and
		// a cast, as as? to a class is.
		ok, end := cf.NewLabel(), join(x.T)
		nn := to
		nn.N = false
		g.code.Op(cf.DUP)
		g.reifiedMarker(nn, reifiedIs)
		g.code.InstanceOf(className(desc(to)))
		g.code.Jump(cf.IFNE, ok)
		g.code.Op(cf.POP)
		g.code.Op(cf.ACONST_NULL)
		g.code.Jump(cf.GOTO, end)
		g.code.Place(ok)
		g.reifiedMarker(nn, reifiedAs)
		g.code.Checkcast(className(desc(to)))
		g.code.Place(end)
		return
	}
	if g.reifiedParam(to) {
		// The copies cast to the type argument, safely for as?.
		kind := reifiedAs
		if x.Safe {
			kind = reifiedSafeAs
		}
		g.reifiedMarker(to, kind)
		g.code.Checkcast(className(desc(to)))
		return
	}
	if to.IsParam() && (!x.Safe || desc(to) == objectDesc) {
		if d := desc(to); d != desc(from) {
			g.code.Checkcast(className(d))
		}
		return
	}
	cls := instanceClass(to) // as? T checks T's erasure, as kotlinc
	if x.Safe {
		ok, end := cf.NewLabel(), join(x.T)
		g.code.Op(cf.DUP)
		g.isInstance(to, cls)
		g.code.Jump(cf.IFNE, ok)
		g.code.Op(cf.POP)
		g.code.Op(cf.ACONST_NULL)
		g.code.Jump(cf.GOTO, end)
		g.code.Place(ok)
		g.code.Checkcast(cls)
		g.code.Place(end)
		if types.Unboxed(x.T) {
			g.unboxValue(x.T)
		}
		return
	}
	if types.MayBeNull(from) && !types.MayBeNull(to) {
		ok := cf.NewLabel()
		g.code.Op(cf.DUP)
		g.code.Jump(cf.IFNONNULL, ok)
		g.code.New("java/lang/NullPointerException")
		g.code.Op(cf.DUP)
		g.code.Sconst("null cannot be cast to non-null type " + kotlinName(to))
		g.code.Invoke(cf.INVOKESPECIAL, "java/lang/NullPointerException", "<init>", "(Ljava/lang/String;)V")
		g.code.Op(cf.ATHROW)
		g.code.Place(ok)
	}
	if m := mutableIntrinsic(to); m != "" {
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/TypeIntrinsics", "as"+m, "(Ljava/lang/Object;)L"+cls+";")
	} else if n, ok := functionArity(to); ok {
		g.code.Iconst(int32(n))
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/TypeIntrinsics", "beforeCheckcastToFunctionOfArity", "(Ljava/lang/Object;I)Ljava/lang/Object;")
		g.code.Checkcast(cls)
	} else if to.K != types.Any && cls != className(desc(from)) {
		g.code.Checkcast(cls)
	}
	switch {
	case to.IsPrimitive():
		g.unbox(to)
	case types.Unboxed(to):
		g.unboxValue(to)
	case x.T.K == types.Unit:
		g.code.Op(cf.POP) // x as Unit, which has no value
	}
}

// kotlinName returns a type's name as kotlinc writes it in messages:
// kotlin.Int, java.util.ArrayList<kotlin.String>.
func kotlinName(t types.Type) string {
	var s string
	switch t.K {
	case types.Int, types.Long, types.Double, types.Char, types.Float, types.Byte, types.Short,
		types.String, types.Any, types.Unit, types.Nothing:
		s = "kotlin." + t.NonNull().String()
	case types.Bool:
		s = "kotlin.Boolean"
	case types.Class:
		name := t.C.KotlinName
		switch {
		case t.IsSuspendFunc():
			name = fmt.Sprintf("kotlin/coroutines/SuspendFunction%d", t.A.Len()-1)
		case t.IsFunc():
			name = fmt.Sprintf("kotlin/Function%d", t.A.Len()-1)
		case name == "":
			name = t.C.Name
		}
		s = strings.NewReplacer("/", ".", "$", ".").Replace(name)
		if t.IsExtFunc() {
			s = "@[ExtensionFunctionType] " + s
		}
		if t.A != nil {
			args := make([]string, len(t.A.List))
			for i, a := range t.A.List {
				switch a.V {
				case types.Star:
					args[i] = "*"
				case types.In:
					args[i] = "in " + kotlinName(a.T)
				case types.Out:
					args[i] = "out " + kotlinName(a.T)
				default:
					args[i] = kotlinName(a.T)
				}
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
	default:
		s = t.NonNull().String()
	}
	if types.MayBeNull(t) {
		s += "?"
	}
	return s
}

// global reads a top-level property: its field, or its getter.
func (g *gen) global(p *types.VarSym) {
	ft := types.FieldType(p)
	d := desc(ft)
	switch {
	case p.Suspend:
		g.coroutineContext()
	case p.Getter != nil:
		g.composeArgs(p.Getter, 0, nil)
		g.callStatic(p.Getter, p.Facade, p.Getter.JVM(), composeGetterDesc(p, "()"+d))
	case p.Const:
		g.constProp(p)
	case p.Lateinit && !p.Private && !p.JvmField:
		g.code.Invoke(cf.INVOKESTATIC, p.Facade, types.GetterJVM(p), "()"+d) // which checks it
	case g.globalField(p):
		g.code.FieldInsn(cf.GETSTATIC, p.FieldClass(), p.Name, d)
		g.lateinit(p)
	case p.Private:
		g.shared.access[p] |= 1
		g.code.Invoke(cf.INVOKESTATIC, p.Facade, privateGetter(p), "()"+d)
	case p.Composable && p.Facade == composablesKt && p.Name == "currentComposer":
		// An intrinsic: its getter only throws.
		g.loadComposer()
	case p.Composable:
		// A composable getter of the class path.
		g.loadComposer()
		g.code.Iconst(0)
		g.code.Invoke(cf.INVOKESTATIC, p.Facade, types.GetterJVM(p), composeGetterDesc(p, "()"+d))
	default:
		g.code.Invoke(cf.INVOKESTATIC, p.Facade, types.GetterJVM(p), "()"+d)
	}
	g.coerce(ft, p.Type)
}

// globalField reports whether code reads and writes top-level property
// p's field rather than call its accessors: in its file facade, as
// kotlinc does, but for a public inline function's body, whose copies in
// other classes call the accessors of a property that isn't private, or
// a const's or @JvmField property's, which have none.
func (g *gen) globalField(p *types.VarSym) bool {
	if p.Const || p.JvmField {
		return true
	}
	if g.cls.Name != p.FieldClass() || g.publicInline && !p.Private {
		return false
	}
	g.forCopies(p, 3)
	return true
}

// forCopies registers the accessors of private property p, getter (1)
// and setter (2), that copies of the inline function being generated
// call in other classes, where they can't use its field: kotlinc's
// access$getX$p. An object expression's member's copies are in copies of
// its class, with its fields.
func (g *gen) forCopies(p *types.VarSym, bits uint8) {
	if !g.publicInline || !p.Private || g.owner != nil && g.owner.Anonymous && p.Owner == g.owner {
		return
	}
	if !p.Mutable {
		bits &= 1
	}
	g.shared.access[p] |= bits
}

// printDesc returns the descriptor of the overload of StringBuilder.append,
// String.valueOf or PrintStream.println that takes a primitive of type t:
// an int for a Byte or Short, which have none of their own.
func printDesc(t types.Type) string {
	if t.K == types.Byte || t.K == types.Short {
		return "I"
	}
	return desc(t)
}

// appendDesc returns the StringBuilder.append overload for type t.
func appendDesc(t types.Type) string {
	switch {
	case t.IsPrimitive():
		return "(" + printDesc(t) + ")Ljava/lang/StringBuilder;"
	case t.K == types.String:
		return "(Ljava/lang/String;)Ljava/lang/StringBuilder;"
	}
	return "(Ljava/lang/Object;)Ljava/lang/StringBuilder;"
}

func (g *gen) newStringBuilder() {
	g.code.New("java/lang/StringBuilder")
	g.code.Op(cf.DUP)
	g.code.Invoke(cf.INVOKESPECIAL, "java/lang/StringBuilder", "<init>", "()V")
}

func (g *gen) appendExpr(x syntax.Expr) {
	g.expr(x)
	t := x.Type()
	if t.K == types.Unit {
		g.coerce(t, types.AnyType) // kotlin.Unit, its toString
		t = types.AnyType
	}
	if types.Unboxed(t) {
		g.boxValue(t) // its toString
		t = types.AnyType.Nullable()
	}
	g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(t))
}

func (g *gen) stringLit(x *syntax.StringLit) {
	if len(x.Parts) == 0 {
		g.code.Sconst("")
		return
	}
	if len(x.Parts) == 1 && x.Parts[0].X == nil {
		g.code.Sconst(x.Parts[0].Lit)
		return
	}
	g.newStringBuilder()
	for _, p := range x.Parts {
		if p.X == nil {
			g.code.Sconst(p.Lit)
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", "(Ljava/lang/String;)Ljava/lang/StringBuilder;")
			continue
		}
		g.appendExpr(p.X)
	}
	g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
}

// stringValueOf converts x to a String as string templates do.
func (g *gen) stringValueOf(x syntax.Expr) {
	g.expr(x)
	t := x.Type()
	if t.K == types.Unit {
		g.coerce(t, types.AnyType)
		t = types.AnyType
	}
	if types.Unboxed(t) {
		g.boxValue(t)
		t = types.AnyType.Nullable()
	}
	switch {
	case t.IsPrimitive():
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/String", "valueOf", "("+printDesc(t)+")Ljava/lang/String;")
	default:
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;")
	}
}

// join returns the label where branches meet that each leave a value of
// type t on the stack. For a reference type, the frame there needs t: the
// branches' own types may differ.
func join(t types.Type) *cf.Label {
	if size(t) == 1 && loadOp(t) == cf.ALOAD {
		return cf.NewJoin(desc(t))
	}
	return cf.NewLabel()
}

// boolValue materializes a condition as 0 or 1.
func (g *gen) boolValue(x syntax.Expr) {
	f, end := cf.NewLabel(), cf.NewLabel()
	g.cond(x, f, false)
	g.code.Iconst(1)
	g.code.Jump(cf.GOTO, end)
	g.code.Place(f)
	g.code.Iconst(0)
	g.code.Place(end)
}

func (g *gen) binary(x *syntax.Binary) {
	if x.Call != nil && x.Op != syntax.IN && x.Op != syntax.NOT_IN && !isComparison(x.Op) {
		g.call(x.Call) // an operator function: plus, rangeTo, ...
		return
	}
	switch x.Op {
	case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT:
		if x.Call != nil {
			g.call(x.Call) // an operator function
			return
		}
		if x.OpType.K == types.Nothing {
			// An operand never completes: the code after it is dead.
			g.expr(x.X)
			g.expr(x.Y)
			return
		}
		if x.OpType == types.StringType {
			g.newStringBuilder()
			g.appendExpr(x.X)
			g.appendExpr(x.Y)
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
			return
		}
		g.exprAs(x.X, x.OpType)
		g.exprAs(x.Y, x.OpType)
		g.code.Op(arithOp(x.OpType, x.Op))
	case syntax.ELVIS:
		g.elvis(x)
	default:
		g.boolValue(x)
	}
}

func isComparison(k syntax.Kind) bool {
	return k == syntax.LT || k == syntax.GT || k == syntax.LE || k == syntax.GE
}

func (g *gen) elvis(x *syntax.Binary) {
	xt, res := x.X.Type(), x.T
	g.expr(x.X)
	if !types.MayBeNull(xt) && !nullableRef(xt) {
		g.coerce(xt, res) // never null: the right side is not evaluated
		return
	}
	// The right side runs in a replace group if the checker says so, and
	// the code that skips it in an empty one.
	right := func() {
		g.grouped(x.Groups, g.siteKey(x.P, "elvis", 0), func() { g.exprAs(x.Y, res) })
	}
	if xt.K == types.Null {
		g.code.Op(cf.POP)
		right()
		return
	}
	nonNull, end := cf.NewLabel(), join(res)
	g.code.Op(cf.DUP)
	g.code.Jump(cf.IFNONNULL, nonNull)
	g.code.Op(cf.POP)
	right()
	g.code.Jump(cf.GOTO, end)
	g.code.Place(nonNull)
	g.grouped(x.Groups, g.siteKey(x.P, "elvis", 1), func() {})
	switch {
	case res.IsPrimitive() && xt.K == types.TypeParam:
		g.coerce(xt.NonNull(), res) // x ?: 0 for T : Int?
	case res.IsPrimitive():
		g.unbox(xt)
	case types.Unboxed(res) && !types.Unboxed(xt):
		g.unboxValue(res.NonNull())
	}
	g.code.Place(end)
}

// nullableRef reports whether a value of type t that isn't null to
// Kotlin is a reference all the same, which kotlinc checks with ?: and
// == null: reflection, Java serialization and Gson can leave it null.
func nullableRef(t types.Type) bool {
	return !t.IsPrimitive() && !types.Unboxed(t) && t.K != types.Nothing && t.K != types.Unit && t.K != types.Null
}

func (g *gen) notNull(x *syntax.NotNull) {
	xt := x.X.Type()
	g.expr(x.X)
	if !types.MayBeNull(xt) {
		return
	}
	if xt.K == types.Null {
		g.code.Op(cf.POP)
		g.throwNPE()
		return
	}
	// As kotlinc, an erased value is checked before its cast, to the
	// non-null type: f()!! with T Int? unboxes by Number.intValue.
	erased, uncast := g.uncastErased()
	g.code.Op(cf.DUP)
	if g.shared.opts.Stdlib {
		g.code.Invoke(cf.INVOKESTATIC, intrinsics, "checkNotNull", "(Ljava/lang/Object;)V")
	} else {
		ok := cf.NewLabel()
		g.code.Jump(cf.IFNONNULL, ok)
		g.throwNPE()
		g.code.Place(ok)
	}
	switch {
	case uncast:
		g.castErased(erased, x.T)
	case x.T.IsPrimitive():
		g.unbox(xt)
	case types.Unboxed(x.T) && !types.Unboxed(xt):
		g.unboxValue(x.T)
	}
}

func (g *gen) throwNPE() {
	g.code.New("java/lang/NullPointerException")
	g.code.Op(cf.DUP)
	g.code.Invoke(cf.INVOKESPECIAL, "java/lang/NullPointerException", "<init>", "()V")
	g.code.Op(cf.ATHROW)
}

// safeCall generates member access or call m on its receiver. With a safe
// call on a nullable receiver, a null receiver yields null instead of
// calling body, and each runs in a replace group if the checker says so.
// body consumes the receiver and pushes a value of type memberType.
func (g *gen) safeCall(m *syntax.Member, memberType, result types.Type, body func()) {
	recv, safe := m.X, m.Safe
	rt := recv.Type()
	g.expr(recv)
	if !safe || !types.MayBeNull(rt) {
		switch nn := rt.NonNull(); {
		case rt.P && nn.IsPrimitive():
			g.unbox(rt) // a boxed platform value used as a primitive
		case rt.P && types.Unboxed(nn) && !types.Unboxed(rt):
			g.castBox(rt, nn)
			g.unboxValue(nn)
		}
		body()
		g.coerce(memberType, result)
		return
	}
	if rt.K == types.Null {
		g.code.Op(cf.POP)
		if result.K != types.Unit || result.N {
			g.code.Op(cf.ACONST_NULL)
		}
		return
	}
	// The call of a suspension point on the receiver, whose stack was
	// spilled before it: the values go back below the result on both
	// paths, after the join.
	var p *suspendPoint
	if sm := g.sm; sm != nil && len(sm.points) > 0 {
		top := sm.points[len(sm.points)-1]
		if m, ok := callFun(top.call).(*syntax.Member); ok && m.X == recv && len(top.stack) > 0 {
			p = top
			p.late = true
		}
	}
	isNull, end := cf.NewLabel(), join(result)
	g.code.Op(cf.DUP)
	g.code.Jump(cf.IFNULL, isNull)
	switch nn := rt.NonNull(); {
	case nn.IsPrimitive():
		g.unbox(rt)
	case types.Unboxed(nn) && !types.Unboxed(rt):
		g.unboxValue(nn)
	}
	g.grouped(m.Groups, g.siteKey(m.P, "safe", 0), func() {
		body()
		if m.Nothing {
			g.coerce(memberType, types.NothingType) // never returns
			return
		}
		g.coerce(memberType, result)
	})
	g.code.Jump(cf.GOTO, end)
	g.code.Place(isNull)
	g.code.Op(cf.POP)
	g.grouped(m.Groups, g.siteKey(m.P, "safe", 1), func() {})
	if result.K != types.Unit || result.N {
		g.code.Op(cf.ACONST_NULL) // a Unit? is null too
	}
	g.code.Place(end)
	if p != nil {
		g.unspillBelow(p.stack, result)
	}
}

// callFun returns the function expression of a call, or nil.
func callFun(x *syntax.Call) syntax.Expr {
	if x == nil {
		return nil
	}
	return x.Fun
}

// unspillBelow puts the values spilled to locals back on the stack, below
// the value of type t on top of it.
func (g *gen) unspillBelow(stack []int, t types.Type) {
	c := g.code
	if t.K == types.Unit || !c.Reachable() {
		c.Unspill(stack)
		return
	}
	d := desc(t)
	tmp := c.AllocLocal(d)
	c.Local(storeOpOf(d), tmp)
	c.Unspill(stack)
	c.Local(loadOpOf(d), tmp)
	c.Retire(tmp)
}

// boundReceiver casts a receiver of a type parameter's type, on the
// stack, to the class of member f if f is of another bound than the
// first, which the receiver erases to: Variant's serializedName of an E
// where E : Enum<E>, E : Variant. It returns the receiver's type.
func (g *gen) boundReceiver(rt types.Type, f *types.FunSym) types.Type {
	if !rt.IsParam() || f.Owner == nil {
		return rt
	}
	if e := types.Erase(rt); e.K == types.Class && !e.C.IsSubclassOf(f.Owner) && f.Owner.Name != "java/lang/Object" {
		g.code.Checkcast(f.Owner.Name)
		return types.ClassType(f.Owner)
	}
	return rt
}

// invokeJava calls the instance method f, of Java or of the program, on a
// receiver of type rt already on the stack, below the arguments. As with
// kotlinc, the method is referenced through the receiver's static type.
func (g *gen) invokeJava(f *types.FunSym, rt types.Type) {
	if rt.IsParam() {
		rt = types.Erase(rt)
	}
	d := funDesc(f)
	if f.Protected && !f.Static && g.protectedAccess(f.Owner, f.JVM(), d) {
		return
	}
	if rt.K == types.Class && rt.C.Interface {
		g.code.InvokeInterface(rt.C.Name, f.JVM(), d)
		return
	}
	owner := "java/lang/Object"
	if rt.K == types.Class {
		owner = rt.C.Name
	}
	g.code.Invoke(cf.INVOKEVIRTUAL, owner, f.JVM(), d)
}

func (g *gen) member(x *syntax.Member) {
	if x.Call != nil {
		g.call(x.Call) // a property with context parameters: x.getP()
		return
	}
	if cl, ok := x.X.(*syntax.ClassLit); ok && cl.Java {
		g.classLit(cl) // X::class.java
		return
	}
	if x.Object != nil {
		g.objectRef(x.Object)
		return
	}
	if s, ok := x.X.(*syntax.Super); ok {
		// super.p calls the supertype's getter directly.
		g.superThis(s)
		if x.Getter != nil {
			g.invokeSuperBody(s, x.Getter.JVM(), x.Getter.Desc)
			g.coerce(types.JVMRet(x.Getter), x.T)
			return
		}
		g.invokeSuperBody(s, types.GetterJVM(x.Prop), "()"+desc(types.GetterType(x.Prop)))
		g.coerce(types.GetterType(x.Prop), x.T)
		return
	}
	switch {
	case x.Entries:
		// A Java enum's entries, as kotlinc's EntriesMappings make them.
		g.code.Invoke(cf.INVOKESTATIC, x.Getter.Owner.Name, x.Getter.Name, x.Getter.Desc)
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/enums/EnumEntriesKt", "enumEntries", "([Ljava/lang/Enum;)Lkotlin/enums/EnumEntries;")
		return
	case x.Getter != nil && x.Getter.Static:
		// A static getter: E.entries.
		g.code.Invoke(cf.INVOKESTATIC, x.Getter.Owner.Name, x.Getter.Name, x.Getter.Desc)
		g.coerce(types.JVMRet(x.Getter), x.T)
		return
	case x.Field != nil:
		f := x.Field
		if x.EvalX {
			g.expr(x.X)
			g.pop(x.X.Type())
		}
		if f.Constant != nil {
			g.pushConst(f.Constant, f.Type) // a Java constant, inlined
		} else {
			g.code.FieldInsn(cf.GETSTATIC, x.Static.Name, f.Name, desc(f.Type))
		}
		g.coerce(f.Type, x.T)
		return
	case x.Getter != nil && x.Getter.Ext && syntax.SourceInline(x.Getter) != nil:
		// An extension property with reified type parameters: a copy of
		// its getter, as of a call of an inline function.
		m := *x
		m.T = x.X.Type()
		if x.Safe {
			m.T = m.T.NonNull()
		}
		call := &syntax.Call{Fun: &m, Sym: x.Getter, Reified: x.Reified}
		call.P, call.T = x.P, x.T
		g.inlineSource(call, syntax.SourceInline(x.Getter))
		return
	case x.Getter != nil && x.Getter.Ext && x.Getter.Owner != nil:
		// An extension property declared in a class: its getter is a
		// method of this, or of the object in scope.
		f := x.Getter
		rt := x.X.Type()
		if x.Safe {
			rt = rt.NonNull()
		} else {
			g.dispatch(x, f.Owner)
		}
		g.safeCall(x, types.JVMRet(f), x.T, func() {
			if x.Safe {
				g.dispatch(x, f.Owner)
				g.swapUnder(rt)
			}
			g.coerceRecv(rt, f.Recv)
			if b := inlineBody(f, 0); b != nil {
				g.inline(b, nil) // inline-only, of the class path
				return
			}
			g.composeArgs(f, 0, nil)
			g.callMethod(f, f.Owner, f.JVM(), funDesc(f))
		})
		return
	case x.Getter != nil && x.Getter.Ext:
		// An extension property: its getter is a static method. Without
		// ?., the receiver may be null if the getter takes null.
		f := x.Getter
		rt := x.X.Type()
		if x.Safe {
			rt = rt.NonNull()
		}
		g.safeCall(x, types.JVMRet(f), x.T, func() {
			g.coerceRecv(rt, f.Recv)
			if inlineBody(f, 0) == nil {
				g.composeArgs(f, 0, nil) // a composable getter's Composer
			}
			g.invokeFun(nil, f, 0) // copied if inline-only
		})
		return
	case x.Getter != nil && x.Getter.Builtin == types.ArraySize:
		g.safeCall(x, types.IntType, x.T, func() { g.code.Op(cf.ARRAYLENGTH) })
		return
	case x.Getter != nil:
		rt := x.X.Type().NonNull()
		g.safeCall(x, types.JVMRet(x.Getter), x.T, func() {
			if x.Getter.Impl {
				g.invokeFun(nil, x.Getter, 0)
				return
			}
			g.boxReceiver(rt)
			rt := g.boundReceiver(rt, x.Getter)
			g.composeArgs(x.Getter, 0, nil)
			g.invokeJava(x.Getter, rt)
		})
		return
	}
	if x.Builtin == types.RefName {
		if l, ok := x.X.(*syntax.Lambda); ok {
			// The name of a reference whose receiver has effects, which
			// are kept.
			g.refReceiver(l)
			g.code.Sconst(l.RefName())
			return
		}
		g.expr(x.X) // ::f.name: the constant
		return
	}
	if x.Builtin == types.IsInitialized {
		// this::x.isInitialized: whether the field is assigned.
		p := x.Prop
		if p.Kind == types.Global {
			g.code.FieldInsn(cf.GETSTATIC, p.FieldClass(), p.Name, desc(types.FieldType(p)))
		} else {
			g.expr(x.X)
			g.getField(p)
		}
		yes, done := cf.NewLabel(), cf.NewLabel()
		g.code.Jump(cf.IFNONNULL, yes)
		g.code.Iconst(0)
		g.code.Jump(cf.GOTO, done)
		g.code.Place(yes)
		g.code.Iconst(1)
		g.code.Place(done)
		return
	}
	if x.Builtin == types.StrLength {
		g.safeCall(x, types.IntType, x.T, func() {
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/String", "length", "()I")
		})
		return
	}
	if x.Builtin == types.CharCode {
		g.safeCall(x, types.IntType, x.T, func() {}) // a Char is its code on the JVM
		return
	}
	p := x.Prop
	if p.Kind == types.Global {
		// A top-level property by its qualified name: a.b.x.
		g.global(p)
		g.coerce(p.Type, x.T)
		return
	}
	_, this := x.X.(*syntax.This)
	direct := this && g.directGet(p)
	cls := recvClass(x.X.Type())
	result := x.T
	if x.Cast {
		result = p.Type // then smart cast, below
	}
	g.safeCall(x, p.Type, result, func() {
		if p.Owner != nil && p.Owner.Value && types.Unboxed(x.X.Type().NonNull()) {
			g.valueGet(p)
			return
		}
		switch {
		case direct:
			g.getField(p)
			g.lateinit(p)
		case x.X.Type().NonNull().IsParam() && p.Owner != nil && cls != nil && !cls.IsSubclassOf(p.Owner):
			// A property of another bound than the first.
			g.code.Checkcast(p.Owner.Name)
			g.getProp(p.Owner, p)
		default:
			g.getProp(cls, p)
		}
	})
	if x.Cast {
		g.smartCast(p.Type, x.T)
	}
}

// dispatch loads the dispatch receiver of a member extension: the object
// it is declared in, or this.
func (g *gen) dispatch(m *syntax.Member, owner *types.ClassSym) {
	switch {
	case m.DispatchThis != nil:
		g.expr(m.DispatchThis)
		if t := m.DispatchThis.Type(); types.Unboxed(t) && !owner.Value {
			g.boxValue(t) // a value class's value, for an interface's member
		}
	case m.Dispatch != nil:
		g.objectRef(m.Dispatch)
	default:
		g.loadThis()
		if owner != g.owner {
			g.boxThis()
		}
	}
}

// invokeSpecial calls a supertype's method directly, as super.f() does.
func (g *gen) invokeSpecial(cls *types.ClassSym, name, d string) {
	switch {
	case g.superAccess(cls, name, d):
	case cls.Interface:
		g.code.InvokeSpecialInterface(cls.Name, name, d)
	default:
		g.code.Invoke(cf.INVOKESPECIAL, cls.Name, name, d)
	}
}

// superThis pushes the receiver of a member of super s: this, or the
// outer class's instance for super@Outer.
func (g *gen) superThis(s *syntax.Super) {
	if s.Outer != nil {
		g.expr(s.Outer)
		return
	}
	g.loadThis()
	g.boxThis()
}

// boxThis boxes this, which loadThis pushed, if it is a value class's
// value, as in the class's static methods: its supertypes' members take
// the box.
func (g *gen) boxThis() {
	if o := g.owner; o != nil && o.Value && types.Unboxed(o.This()) {
		if top := g.code.StackTop(); top.Tag() != cf.VObject || top.Class() != o.Name {
			g.boxValue(o.This())
		}
	}
}

// invokeSuper calls method name of super s's supertype, with descriptor
// d, on the receiver superThis pushed below the arguments: for
// super@Outer, through the outer class's accessor, as kotlinc does.
func (g *gen) invokeSuper(s *syntax.Super, name, d string) {
	if s.Outer == nil {
		g.invokeSpecial(s.Cls, name, d)
		return
	}
	host := s.Outer.Recv.Type.C
	m := privateMethod{class: host.Name, name: name, desc: d, super: s.Cls.Name, iface: s.Cls.Interface}
	g.shared.methods[m] = true
	g.code.Invoke(cf.INVOKESTATIC, host.Name, m.accessor(), "(L"+host.Name+";"+d[1:])
}

// invokeSuperBody calls the body of a property accessor of super s's
// supertype as invokeSuper does, or, of an interface compiled without JVM
// default methods, its DefaultImpls class's static method, which takes
// the receiver first.
func (g *gen) invokeSuperBody(s *syntax.Super, name, d string) {
	if s.Cls.HasDefaultImpls() {
		g.code.Invoke(cf.INVOKESTATIC, s.Cls.Name+"$DefaultImpls", name, "(L"+s.Cls.Name+";"+d[1:])
		return
	}
	g.invokeSuper(s, name, d)
}

func (g *gen) args(x *syntax.Call, f *types.FunSym) {
	for i, a := range x.Args {
		g.argAs(a, paramType(f, i))
	}
	g.continuationArg(x)
	g.line(x.Pos()) // the call, after arguments on other lines
}

// paramType returns the type of the argument that a call of f passes for
// parameter j: a box for a Result that f takes boxed (see BoxedParams).
func paramType(f *types.FunSym, j int) types.Type {
	t := f.Params[j].Type
	if slices.Contains(f.BoxedParams, j) {
		return types.Boxed(t)
	}
	return t
}

// superCall reports whether x calls a supertype's member, super.f().
func superCall(x *syntax.Call) bool {
	m, ok := x.Fun.(*syntax.Member)
	if !ok {
		return false
	}
	_, ok = m.X.(*syntax.Super)
	return ok
}

// argAs pushes argument a of a parameter of type t: a Unit one is a
// kotlin.Unit on the JVM.
func (g *gen) argAs(a syntax.Expr, t types.Type) {
	g.exprAs(a, t)
	if t.K == types.Unit && !t.N {
		g.coerce(types.UnitType, types.AnyType)
	}
}

// passedArg returns argument i of call x: a named argument of a vararg
// parameter passes its array, as a spread one does.
func passedArg(x *syntax.Call, i int, vararg bool) syntax.Expr {
	a := x.Args[i]
	if _, spread := a.(*syntax.Spread); vararg && !spread && x.Name(i) != "" {
		sp := &syntax.Spread{X: a}
		sp.P, sp.T = a.Pos(), a.Type()
		return sp
	}
	return a
}

// callArgs pushes the arguments of a call of f in the order of its
// parameters: those for a vararg parameter in an array, and zeros for
// omitted ones, whose bits it sets in the returned mask. A composable
// function takes the mask itself, after its Composer: see composeArgs.
func (g *gen) callArgs(x *syntax.Call, f *types.FunSym) int {
	if df := composeDefaultsOf(f); df != nil && !superCall(x) {
		g.composeDefaultsArgs(x, f, df)
		return composeDefaultsMask
	}
	mask := g.valueArgs(x, f)
	if f.Composable {
		g.composeArgs(f, mask, g.composeCallArgs(x, f, mask))
		return 0
	}
	return mask
}

// valueArgs pushes the arguments of a call of f that its parameters
// declare, as callArgs does, and returns the mask of the omitted ones.
func (g *gen) valueArgs(x *syntax.Call, f *types.FunSym) int {
	if x.ArgMap == nil {
		g.args(x, f)
		return 0
	}
	if o := f.DefaultsOf; o != nil && omits(x, f) && !f.Impl {
		// The $default of the function whose default values f inherits
		// takes its parameters: T's erasure, a box, for f's IC.
		f = o
	}
	jvm := descParams(funDesc(f))
	if f.Impl {
		jvm = jvm[1:] // a class-path value class's member takes the value first
	}
	jvm = jvm[len(f.Contexts):]
	if f.Ext {
		jvm = jvm[1:]
	}
	// Arguments are evaluated in the order they are written. Named ones
	// out of the parameters' order go to locals first, and so does the
	// array of a vararg parameter's arguments, where they are written:
	// f(1, 2, b = x, a = y) evaluates 1 and 2 before x.
	stored := map[int]int{} // argument: local
	varargs := -1           // the array's local
	if !inOrder(x.ArgMap) {
		for i, a := range x.Args {
			p := f.Params[x.ArgMap[i]]
			if p.Vararg {
				if varargs < 0 {
					var args []syntax.Expr
					for k := range x.Args {
						if x.ArgMap[k] == x.ArgMap[i] {
							args = append(args, passedArg(x, k, true))
						}
					}
					d, elem := varargDesc(x, p, jvm[x.ArgMap[i]])
					g.varargs(args, d, elem)
					varargs = g.code.AllocLocal(d)
					g.code.Local(cf.ASTORE, varargs)
				}
				continue
			}
			g.argAs(a, paramType(f, x.ArgMap[i]))
			stored[i] = g.code.AllocLocal(paramDesc(p.Type))
			g.code.Local(storeOp(p.Type), stored[i])
		}
	}
	mask := 0
	for j, p := range f.Params {
		var args []syntax.Expr
		arg := -1
		for i := range x.Args {
			if x.ArgMap[i] == j {
				args = append(args, passedArg(x, i, p.Vararg))
				arg = i
			}
		}
		switch {
		case p.Vararg && len(args) > 0 && varargs >= 0:
			g.code.Local(cf.ALOAD, varargs)
			g.code.Retire(varargs)
		case p.Vararg && (len(args) > 0 || !p.Default):
			d, elem := varargDesc(x, p, jvm[j])
			g.varargs(args, d, elem)
		case len(args) == 0:
			// Omitted, or a vararg parameter with a default value that
			// gets no arguments.
			g.zero(jvm[j])
			mask |= 1 << (len(f.Contexts) + j) // as kotlinc's, the contexts count
		default:
			if slot, ok := stored[arg]; ok {
				g.code.Local(loadOp(p.Type), slot)
				g.code.Retire(slot)
			} else {
				g.argAs(args[0], paramType(f, j))
			}
		}
	}
	g.continuationArg(x)
	g.line(x.Pos())
	return mask
}

// omits reports whether call x leaves out an argument of f's.
func omits(x *syntax.Call, f *types.FunSym) bool {
	for j := range f.Params {
		if !slices.Contains(x.ArgMap, j) {
			return true
		}
	}
	return false
}

// varargDesc returns the descriptor of the array that the arguments of
// vararg parameter p are passed in, and its elements' type: an array of
// the call's type arguments, as with kotlinc, or of the erasure of p's.
func varargDesc(x *syntax.Call, p *types.VarSym, jvm string) (string, types.Type) {
	if t := x.Vararg; t.IsArray() && !t.HasVars() && !t.HasParams() {
		switch e := types.StoredElem(t); e.K {
		case types.Nothing, types.Null, types.Unit, types.Invalid:
		default:
			return desc(t), e
		}
	}
	if !p.Type.IsArray() {
		return jvm, types.VarargElem(p.Type) // an unsigned array's, unboxed
	}
	return jvm, types.StoredElem(p.Type)
}

// inOrder reports whether arguments go to parameters in increasing order.
func inOrder(amap []int) bool {
	for i := 1; i < len(amap); i++ {
		if amap[i] < amap[i-1] {
			return false
		}
	}
	return true
}

// zero pushes the zero value of a JVM type, for an omitted argument.
func (g *gen) zero(d string) {
	switch d[0] {
	case 'I', 'Z', 'C', 'B', 'S':
		g.code.Iconst(0)
	case 'J':
		g.code.Lconst(0)
	case 'F':
		g.code.Fconst(0)
	case 'D':
		g.code.Dconst(0)
	default:
		g.code.Op(cf.ACONST_NULL)
	}
}

// descParams splits a method descriptor's parameter types.
func descParams(d string) []string {
	var out []string
	for i := 1; d[i] != ')'; {
		j := i
		for d[j] == '[' {
			j++
		}
		if d[j] == 'L' {
			j += strings.IndexByte(d[j:], ';')
		}
		out = append(out, d[i:j+1])
		i = j + 1
	}
	return out
}

// invokeStatic calls a static method of owner: f itself, or with omitted
// arguments (a mask other than 0), its $default variant, which takes the
// mask and an unused marker after the arguments. A member function's
// $default takes the receiver first.
func (g *gen) invokeStatic(owner string, f *types.FunSym, mask int, iface bool) {
	name, d := f.JVM(), funDesc(f)
	if mask != 0 {
		g.code.Iconst(int32(mask))
		g.code.Op(cf.ACONST_NULL)
		i := strings.IndexByte(d, ')')
		d = d[:i] + "ILjava/lang/Object;" + d[i:]
		name += "$default"
	}
	switch {
	case iface:
		g.code.InvokeStaticInterface(owner, name, d)
	case mask == 0:
		g.callStatic(f, owner, name, d)
	default:
		g.code.Invoke(cf.INVOKESTATIC, owner, name, d)
	}
}

// invokeMember calls a Java method or a member of a Kotlin class on a
// receiver of type rt, below the arguments. With omitted arguments, it
// calls the static $default variant.
func (g *gen) invokeMember(f *types.FunSym, rt types.Type, mask int) {
	if mask == 0 {
		g.invokeJava(f, rt)
		return
	}
	g.invokeDefault(f, mask)
}

// invokeDefault calls the static name$default method of a method, which
// takes the receiver first, then the arguments, the mask of the omitted
// ones and an unused marker: a value class's takes the value. An override
// calls the one of the method whose default values it inherits.
func (g *gen) invokeDefault(f *types.FunSym, mask int) {
	if o := f.DefaultsOf; o != nil {
		// The overridden function's $default, whose result may be of a
		// class the override's narrows, as T's erasure.
		ret := func(f *types.FunSym) string {
			d := funDesc(f)
			return d[strings.IndexByte(d, ')')+1:]
		}
		switch want := ret(f); {
		case want == ret(o):
		case (want[0] == 'L' || want[0] == '[') && !types.Unboxed(types.JVMRet(f)):
			defer g.code.Checkcast(className(want))
		default:
			// A primitive or a value class's value, which T's boxes.
			defer g.coerce(types.Erase(types.JVMRet(o)), types.JVMRet(f))
		}
		f = o
	}
	if f.ComposeDefaults {
		g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name+"$ComposeDefaultImpls", f.JVM()+"$default", composeDefaultsDesc(f))
		return
	}
	g.code.Iconst(int32(mask))
	g.code.Op(cf.ACONST_NULL)
	fd := funDesc(f)
	i := strings.IndexByte(fd, ')')
	self := "L" + f.Owner.Name + ";"
	if f.Owner.Value {
		self = desc(f.Owner.Underlying.Type)
	}
	d := "(" + self + fd[1:i] + "ILjava/lang/Object;" + fd[i:]
	switch {
	case f.Owner.HasDefaultImpls():
		// A Kotlin interface compiled without default methods: the
		// method is in its DefaultImpls class.
		g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name+"$DefaultImpls", f.JVM()+"$default", d)
	case f.Owner.Interface:
		g.code.InvokeStaticInterface(f.Owner.Name, f.JVM()+"$default", d)
	default:
		g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name, f.JVM()+"$default", d)
	}
}

func (g *gen) call(x *syntax.Call) {
	if x.Group {
		// An inline function's call whose lambda calls composables, in
		// a replace group: see composeFlow.
		g.grouped(true, g.siteKey(x.P, "call", 0), func() { g.ungroupedCall(x) })
		return
	}
	g.ungroupedCall(x)
}

// ungroupedCall generates call x, but for the group of an inline
// function's call.
func (g *gen) ungroupedCall(x *syntax.Call) {
	if s, ok := constTrim(x, 0); ok {
		g.code.Sconst(s) // as kotlinc's, a constant
		return
	}
	if t := g.tail; x.Tail && t != nil && x.Sym == t.fd.Sym && len(g.inl) == 0 && len(g.srcInl) == 0 && g.lam == nil {
		g.tailCall(x, t)
		return
	}
	if x.Builtin == types.SuspendIntrinsic {
		g.suspendIntrinsic(x)
		return
	}
	if syntax.SourceInline(x.Sym) == nil {
		// A copy of the program's inline function has the suspension
		// points of its body.
		if p := g.suspendStart(x, x.Sym); p != nil {
			defer g.suspendEnd(p)
		}
	}
	switch x.Builtin {
	case types.Require, types.Check, types.ErrorCall, types.Todo, types.RequireNotNull, types.CheckNotNull:
		g.assertion(x)
		return
	case types.ArrayNew, types.ArrayInit, types.ArrayLiteral:
		g.arrayFactory(x)
		return
	case types.EnumValues, types.EnumValueOf:
		g.enumIntrinsic(x)
		return
	case types.TypeOf:
		g.typeOf(g.reify(x.Reified[0]))
		return
	case types.ComposeKey:
		g.composeKey(x)
		return
	case types.ToChar, types.NewString:
		if _, ok := x.Fun.(*syntax.Ident); ok {
			if x.Builtin == types.ToChar { // Char(code)
				g.exprAs(x.Args[0], types.IntType)
				g.code.Op(cf.I2C)
				return
			}
			g.code.New("java/lang/String") // String(chars) or String()
			g.code.Op(cf.DUP)
			if len(x.Args) == 0 {
				g.code.Invoke(cf.INVOKESPECIAL, "java/lang/String", "<init>", "()V")
				return
			}
			g.exprAs(x.Args[0], types.ArrayOf(types.CharType))
			g.code.Invoke(cf.INVOKESPECIAL, "java/lang/String", "<init>", "([C)V")
			return
		}
	}
	if d := syntax.SourceInline(x.Sym); d != nil {
		g.inlineSource(x, d) // the program's inline function
		return
	}
	if m, ok := x.Fun.(*syntax.Member); ok {
		if s, ok := m.X.(*syntax.Super); ok {
			f := x.Sym
			g.superThis(s)
			g.contextArgs(x, f)
			g.callArgs(x, f)
			if s.Cls.HasDefaultImpls() && !f.Abstract {
				// The body in the interface's DefaultImpls class.
				g.code.Invoke(cf.INVOKESTATIC, s.Cls.Name+"$DefaultImpls", f.JVM(), "(L"+s.Cls.Name+";"+funDesc(f)[1:])
			} else {
				g.invokeSuper(s, f.JVM(), funDesc(f))
			}
			g.coerce(types.JVMRet(f), x.T)
			return
		}
	}
	switch {
	case x.Builtin == types.Println || x.Builtin == types.Print:
		// As kotlinc's inlined println, an argument that suspends comes
		// first, so that System.out isn't saved with the locals.
		late := g.sm != nil && hasSuspension(x.Args)
		out := func(t types.Type) {
			if !late {
				return
			}
			g.code.FieldInsn(cf.GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
			if size(t) == 2 {
				g.code.Op(cf.DUP_X2)
				g.code.Op(cf.POP)
			} else {
				g.code.Op(cf.SWAP)
			}
		}
		if !late {
			g.code.FieldInsn(cf.GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
		}
		name := "println"
		if x.Builtin == types.Print {
			name = "print"
		}
		if len(x.Args) == 0 {
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/io/PrintStream", name, "()V")
			return
		}
		if d := x.Sym.Desc; d != "" {
			// kotlin.io.println(message): its body calls the PrintStream
			// method with the same parameter.
			g.args(x, x.Sym)
			out(x.Sym.Params[0].Type)
			if d == "(B)V" || d == "(S)V" {
				d = "(I)V"
			}
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/io/PrintStream", name, d)
			return
		}
		a := x.Args[0]
		g.expr(a)
		t := a.Type()
		if types.Unboxed(t) {
			g.boxValue(t)
			t = types.AnyType.Nullable()
		}
		out(t)
		var d string
		switch {
		case t.IsPrimitive():
			d = printDesc(t)
		case t.K == types.String:
			d = "Ljava/lang/String;"
		default:
			d = "Ljava/lang/Object;"
		}
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/io/PrintStream", name, "("+d+")V")

	case x.SAM:
		g.expr(x.Args[0])

	case x.Builtin == types.NewInstance:
		// mutableListOf() and the like: a constructor.
		g.code.New(x.Sym.Facade)
		g.code.Op(cf.DUP)
		g.code.Invoke(cf.INVOKESPECIAL, x.Sym.Facade, "<init>", "()V")

	case x.Sym != nil && x.Sym.Ext && x.Sym.Owner != nil:
		// An extension declared in a class: a method of this, or of the
		// object in scope, which takes the extension receiver first.
		f := x.Sym
		m := x.Fun.(*syntax.Member)
		rt := m.T
		if !m.Safe {
			g.dispatch(m, f.Owner)
			g.contextArgs(x, f)
		}
		g.safeCall(m, types.JVMRet(f), x.T, func() {
			if m.Safe {
				// This goes below the receiver, unboxed if primitive.
				g.dispatch(m, f.Owner)
				g.swapUnder(m.X.Type().NonNull())
				g.contextsUnder(x, f, m.X.Type().NonNull())
			}
			g.coerceRecv(rt, f.Recv)
			if mask := g.callArgs(x, f); mask != 0 {
				g.invokeDefault(f, mask)
			} else {
				g.callMethod(f, f.Owner, f.JVM(), funDesc(f))
			}
			g.returned(f)
		})

	case x.Sym != nil && x.Sym.Ext && localOf(x.Sym) != nil:
		// A local extension function, x.g(): x is its first argument.
		g.callLocal(x, localOf(x.Sym))

	case x.Sym != nil && x.Sym.Ext:
		f := x.Sym
		m := x.Fun.(*syntax.Member)
		rt := m.T
		if !m.Safe {
			g.contextArgs(x, f)
		}
		g.safeCall(m, types.JVMRet(f), x.T, func() {
			if m.Safe {
				g.contextsUnder(x, f, m.X.Type().NonNull())
			}
			g.coerceRecv(rt, f.Recv)
			mask := g.callArgs(x, f)
			switch f.Builtin {
			case types.MapSet:
				// map[key] = value is map.put(key, value).
				g.code.InvokeInterface("java/util/Map", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;")
				g.code.Op(cf.POP)
				return
			case types.EntryKey:
				g.code.InvokeInterface("java/util/Map$Entry", "getKey", "()Ljava/lang/Object;")
				return
			case types.EntryValue:
				g.code.InvokeInterface("java/util/Map$Entry", "getValue", "()Ljava/lang/Object;")
				return
			case types.ToTypedArray:
				g.toTypedArray(x.T)
				return
			case types.ThenToString:
				g.invokeStatic(f.Facade, f, mask, false)
				g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/Object", "toString", "()Ljava/lang/String;")
				return
			}
			g.invokeFun(x, f, mask)
			g.returned(f)
		})

	case x.Invoke:
		if l, ctx := g.inlinedInvoke(x); l != nil {
			g.invokeInlined(x, l, ctx)
			return
		}
		f := x.Sym
		g.expr(x.Fun)
		if f.Owner.VarArity() {
			g.argArray(x, f)
			g.code.InvokeInterface(f.Owner.Name, f.Name, f.Desc)
			g.coerce(types.JVMRet(f), x.T)
			return
		}
		g.args(x, f)
		g.composeArgs(f, 0, g.invokeArgs(x))
		g.code.InvokeInterface(f.Owner.Name, f.Name, funDesc(f))
		g.coerce(types.JVMRet(f), x.T)

	case x.Builtin == types.ArrayGet || x.Builtin == types.ArraySet || x.Builtin == types.ArrayClone:
		m := x.Fun.(*syntax.Member)
		rt := m.T.NonNull()
		g.safeCall(m, arrayMemberType(x.Builtin, rt), x.T, func() { g.arrayMember(x, rt) })

	case x.Builtin != types.NotBuiltin:
		m := x.Fun.(*syntax.Member)
		rt := m.T // receiver type, non-null
		ret := builtinRet(x)
		g.safeCall(m, ret, x.T, func() { g.builtinMember(x, rt) })

	case x.Ctor != nil && x.Ctor.Annotation:
		g.annotationNew(x)

	case x.Ctor != nil && x.Ctor.Value:
		g.valueCtorCall(x)

	case x.Ctor != nil:
		d := ctorJVMDesc(x.Sym)
		var mask int
		if g.sm != nil && hasSuspension(x.Args) || hasHandlers(x.Args) {
			// The object under construction can't be stored while the
			// arguments suspend, or in the locals that a try's handler
			// needs the stack spilled to: they are evaluated first, as
			// kotlinc does.
			depth := g.code.Depth()
			g.outerArg(x)
			mask = g.callArgs(x, x.Sym)
			args, _ := g.code.SpillAbove(depth)
			g.code.New(x.Ctor.Name)
			g.code.Op(cf.DUP)
			g.code.Unspill(args)
		} else {
			g.code.New(x.Ctor.Name)
			g.code.Op(cf.DUP)
			g.outerArg(x)
			mask = g.callArgs(x, x.Sym)
		}
		if mask != 0 {
			// The constructor for omitted arguments takes a mask and a marker.
			g.code.Iconst(int32(mask))
			g.code.Op(cf.ACONST_NULL)
			i := strings.IndexByte(d, ')')
			d = d[:i] + "ILkotlin/jvm/internal/DefaultConstructorMarker;" + d[i:]
		} else if g.viaMarker(x.Sym) {
			g.useMarker(x.Sym)
			d = g.marker(d)
		}
		g.code.Invoke(cf.INVOKESPECIAL, x.Ctor.Name, "<init>", d)

	case localOf(x.Sym) != nil:
		g.callLocal(x, localOf(x.Sym))

	case x.Sym.Static:
		f := x.Sym
		owner := f.Owner // an object's @JvmStatic member
		if m, ok := x.Fun.(*syntax.Member); ok && m.Static != nil {
			owner = m.Static
		}
		g.contextArgs(x, f)
		g.invokeStatic(owner.Name, f, g.callArgs(x, f), owner.Interface)
		g.coerce(types.JVMRet(f), x.T)

	case x.Sym.Owner != nil && x.Sym.Owner.External:
		f := x.Sym
		m, ok := x.Fun.(*syntax.Member)
		if !ok {
			// An inherited method, called on the implicit this.
			g.loadThis()
			g.boxReceiver(g.thisClass().This())
			g.contextArgs(x, f)
			if f.Inline != nil {
				g.invokeFun(x, f, g.callArgs(x, f)) // copied: a reified one
			} else {
				g.invokeMember(f, g.thisClass().This(), g.callArgs(x, f))
			}
			g.coerce(types.JVMRet(f), x.T)
			return
		}
		g.safeCall(m, types.JVMRet(f), x.T, func() {
			if f.Impl || f.Inline != nil {
				// A class-path value class's member: a static method; or
				// an inline member with reified type parameters, copied.
				g.contextArgs(x, f)
				g.invokeFun(x, f, g.callArgs(x, f))
				return
			}
			g.boxReceiver(m.T)
			rt := g.boundReceiver(m.T, f)
			g.contextArgs(x, f)
			g.invokeMember(f, rt, g.callArgs(x, f))
		})

	case x.Sym.Owner != nil:
		f := x.Sym
		d := "(" + leadDesc(f) + sigDesc(f)[1:]
		invoke := func(cls *types.ClassSym, rt types.Type) {
			if f.Owner.Value {
				// A value class's member: a static method that takes the
				// value first, or with omitted arguments its $default.
				g.contextArgs(x, f)
				if mask := g.callArgs(x, f); mask != 0 {
					g.invokeDefault(f, mask)
					return
				}
				g.code.Invoke(cf.INVOKESTATIC, f.Owner.Name, f.JVM(), "("+desc(f.Owner.Underlying.Type)+d[1:])
				return
			}
			g.boxReceiver(rt)
			if f.Owner.VarArity() {
				g.argArray(x, f)
				g.code.InvokeInterface(f.Owner.Name, f.Name, f.Desc)
				return
			}
			g.contextArgs(x, f)
			mask := g.callArgs(x, f)
			switch {
			case f.Owner.Func && f.Suspend:
				// A suspend function type's invoke, of FunctionN+1: it
				// takes the continuation as an Object too.
				g.code.InvokeInterface(f.Owner.Name, f.Name, "("+strings.Repeat(objectDesc, len(f.Params)+1)+")"+objectDesc)
			case f.Owner.Func && f.Composable:
				// A composable function type's invoke, which takes the
				// Composer and the change flags as Objects too.
				g.code.InvokeInterface(f.Owner.Name, f.Name, f.Desc)
			case f.Owner.Func:
				g.code.InvokeInterface(f.Owner.Name, f.Name, d)
			case mask != 0:
				g.invokeDefault(f, mask)
			default:
				g.callMethod(f, cls, f.JVM(), d)
			}
		}
		if m, ok := x.Fun.(*syntax.Member); ok {
			cls := recvClass(m.T)
			g.safeCall(m, types.JVMRet(f), x.T, func() { invoke(cls, m.T.NonNull()) })
			return
		}
		g.loadThis() // implicit this
		invoke(g.thisClass(), g.thisClass().This())
		g.coerce(types.JVMRet(f), x.T)

	default:
		f := x.Sym
		g.contextArgs(x, f)
		g.invokeFun(x, f, g.callArgs(x, f))
		g.returned(f)
		g.coerce(types.JVMRet(f), x.T)
	}
}

// contextArgs pushes the context arguments of call x of f, which takes
// them first.
func (g *gen) contextArgs(x *syntax.Call, f *types.FunSym) {
	for i, a := range x.Ctx {
		g.argAs(a, f.Contexts[i].Type)
	}
}

// contextsUnder pushes the context arguments of call x of f below its
// receiver on the stack, of type t: that of a safe call, evaluated first.
func (g *gen) contextsUnder(x *syntax.Call, f *types.FunSym, t types.Type) {
	if len(x.Ctx) == 0 {
		return
	}
	slot := g.code.AllocLocal(desc(t))
	g.code.Local(storeOp(t), slot)
	g.contextArgs(x, f)
	g.code.Local(loadOp(t), slot)
	g.code.Retire(slot)
}

// returned handles the result of a call of a function that returns
// Nothing, which never returns: on the JVM it has a result, but the code
// after it is unreachable.
func (g *gen) returned(f *types.FunSym) {
	if f.Ret.K == types.Nothing && !strings.HasSuffix(funDesc(f), ")V") && g.code.Reachable() {
		g.nothingValue(true)
	}
}

// nothingValue ends the code after a call whose value is Nothing, which
// on the JVM can return, as a generic function's that unsafely casts its
// value: as kotlinc's, it throws a KotlinNothingValueException, after
// popping the value if pop is set.
func (g *gen) nothingValue(pop bool) {
	if pop {
		g.code.Op(cf.POP)
	}
	if !g.shared.opts.Stdlib {
		g.code.Op(cf.ACONST_NULL)
		g.code.Op(cf.ATHROW)
		return
	}
	const exc = "kotlin/KotlinNothingValueException"
	g.code.New(exc)
	g.code.Op(cf.DUP)
	g.code.Invoke(cf.INVOKESPECIAL, exc, "<init>", "()V")
	g.code.Op(cf.ATHROW)
}

// builtinRet returns the type of the value a builtin member call leaves.
func builtinRet(x *syntax.Call) types.Type {
	switch x.Builtin {
	case types.StrIsEmpty:
		return types.BoolType
	case types.ToInt:
		return types.IntType
	case types.ToLong:
		return types.LongType
	case types.ToDouble:
		return types.DoubleType
	case types.ToFloat:
		return types.FloatType
	case types.ToByte:
		return types.ByteType
	case types.ToShort:
		return types.ShortType
	case types.ToChar, types.StrGet:
		return types.CharType
	case types.StrToCharArray:
		return types.ArrayOf(types.CharType)
	case types.CharMethod:
		if m := types.CharMethods[x.Fun.(*syntax.Member).Name]; strings.HasPrefix(m, "is") {
			return types.BoolType
		}
		return types.CharType
	case types.Bitwise:
		return x.Fun.(*syntax.Member).T
	case types.OperatorFun:
		_, res, _ := types.OperatorFunType(x.Fun.(*syntax.Member).Name, x.Fun.(*syntax.Member).T, operatorArg(x))
		return res
	}
	return types.StringType
}

// operatorArg returns the type of the argument of a number's or Char's
// operator function called by name, Invalid for a unary one.
func operatorArg(x *syntax.Call) types.Type {
	if len(x.Args) == 0 {
		return types.InvalidType
	}
	return x.Args[0].Type().Flex()
}

// builtinMember emits a call on a receiver of type rt already on the stack.
func (g *gen) builtinMember(x *syntax.Call, rt types.Type) {
	const str = "java/lang/String"
	switch x.Builtin {
	case types.StrUppercase, types.StrLowercase:
		name := "toUpperCase"
		if x.Builtin == types.StrLowercase {
			name = "toLowerCase"
		}
		// Kotlin's uppercase()/lowercase() are locale-independent.
		g.code.FieldInsn(cf.GETSTATIC, "java/util/Locale", "ROOT", "Ljava/util/Locale;")
		g.code.Invoke(cf.INVOKEVIRTUAL, str, name, "(Ljava/util/Locale;)Ljava/lang/String;")
	case types.StrIsEmpty:
		g.code.Invoke(cf.INVOKEVIRTUAL, str, "isEmpty", "()Z")
	case types.StrSubstring:
		for _, a := range x.Args {
			g.exprAs(a, types.IntType)
		}
		if len(x.Args) == 1 {
			g.code.Invoke(cf.INVOKEVIRTUAL, str, "substring", "(I)Ljava/lang/String;")
		} else {
			g.code.Invoke(cf.INVOKEVIRTUAL, str, "substring", "(II)Ljava/lang/String;")
		}
	case types.ToString:
		switch {
		case types.MayBeNull(rt):
			// Any?.toString(): "null" for null, and a value class's
			// toString for its value.
			if types.Unboxed(rt) {
				g.boxValue(rt)
			}
			g.code.Invoke(cf.INVOKESTATIC, str, "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;")
		case rt != types.StringType:
			g.code.Invoke(cf.INVOKESTATIC, str, "valueOf", "("+printDesc(rt)+")Ljava/lang/String;")
		}
	case types.ToInt, types.ToLong, types.ToDouble, types.ToFloat, types.ToByte, types.ToShort, types.ToChar:
		if rt == types.StringType {
			g.code.Invoke(cf.INVOKESTATIC, "java/lang/Integer", "parseInt", "(Ljava/lang/String;)I")
			return
		}
		g.convert(rt, builtinRet(x))
	case types.StrGet:
		g.exprAs(x.Args[0], types.IntType)
		g.code.Invoke(cf.INVOKEVIRTUAL, str, "charAt", "(I)C")
	case types.StrToCharArray:
		g.code.Invoke(cf.INVOKEVIRTUAL, str, "toCharArray", "()[C")
	case types.StrPlus:
		g.newStringBuilder()
		g.code.Op(cf.SWAP)
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
		g.appendExpr(x.Args[0])
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
	case types.CharMethod:
		m := types.CharMethods[x.Fun.(*syntax.Member).Name]
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/Character", m, "(C)"+desc(builtinRet(x)))
	case types.Bitwise:
		g.bitwise(x.Fun.(*syntax.Member).Name, rt, x.Args)
	case types.OperatorFun:
		g.operatorFun(x.Fun.(*syntax.Member).Name, rt, x)
	}
}

// negOp returns the instruction that negates a value of type t, which is
// Int, Long, Float or Double.
func negOp(t types.Type) byte {
	switch t.K {
	case types.Long:
		return cf.LNEG
	case types.Double:
		return cf.DNEG
	case types.Float:
		return cf.FNEG
	}
	return cf.INEG
}

// operatorFun emits a number's or Char's operator function name, called
// by name, on the receiver of type rt on the stack, as the operator:
// 1.plus(2) is 1 + 2, b.inc() is b + 1 as a Byte, and a.compareTo(b)
// is Double.compare(a, b) for Doubles, as kotlinc writes it.
func (g *gen) operatorFun(name string, rt types.Type, x *syntax.Call) {
	op, res, _ := types.OperatorFunType(name, rt, operatorArg(x))
	g.convert(rt, op)
	switch name {
	case "unaryPlus":
	case "unaryMinus":
		g.code.Op(negOp(op))
	case "inc", "dec":
		switch op.K {
		case types.Long:
			g.code.Lconst(1)
		case types.Float:
			g.code.Fconst(1)
		case types.Double:
			g.code.Dconst(1)
		default:
			g.code.Iconst(1)
		}
		k := syntax.PLUS
		if name == "dec" {
			k = syntax.MINUS
		}
		g.code.Op(arithOp(op, k))
	case "compareTo":
		g.exprAs(x.Args[0], op)
		switch op.K {
		case types.Long:
			g.code.Op(cf.LCMP)
		case types.Float:
			g.code.Invoke(cf.INVOKESTATIC, "java/lang/Float", "compare", "(FF)I")
		case types.Double:
			g.code.Invoke(cf.INVOKESTATIC, "java/lang/Double", "compare", "(DD)I")
		default:
			g.code.Invoke(cf.INVOKESTATIC, "java/lang/Integer", "compare", "(II)I")
		}
		return
	default:
		g.exprAs(x.Args[0], op)
		g.code.Op(arithOp(op, map[string]syntax.Kind{
			"plus": syntax.PLUS, "minus": syntax.MINUS, "times": syntax.STAR, "div": syntax.SLASH, "rem": syntax.PERCENT,
		}[name]))
	}
	g.convert(op, res)
}

// bitwiseOps are the instructions of the bitwise functions, for an Int
// and a Long receiver.
var bitwiseOps = map[string][2]byte{
	"and": {cf.IAND, cf.LAND}, "or": {cf.IOR, cf.LOR}, "xor": {cf.IXOR, cf.LXOR},
	"shl": {cf.ISHL, cf.LSHL}, "shr": {cf.ISHR, cf.LSHR}, "ushr": {cf.IUSHR, cf.LUSHR},
	"inv": {cf.IXOR, cf.LXOR}, "not": {cf.IXOR},
}

// bitwise emits Int's or Long's bitwise function name on the receiver on
// the stack: inv() is xor with -1, as kotlinc writes it, and Boolean's
// not() xor with 1.
func (g *gen) bitwise(name string, rt types.Type, args []syntax.Expr) {
	long := rt.K == types.Long
	switch {
	case name == "not":
		g.code.Iconst(1)
	case name == "inv" && long:
		g.code.Lconst(-1)
	case name == "inv":
		g.code.Iconst(-1)
	case name == "shl" || name == "shr" || name == "ushr":
		g.exprAs(args[0], types.IntType)
	default:
		g.exprAs(args[0], rt)
	}
	op := bitwiseOps[name][0]
	if long {
		op = bitwiseOps[name][1]
	}
	g.code.Op(op)
}

// convert converts a primitive on the stack from type from to type to, as
// Kotlin's conversion functions do: Long.toInt() keeps the low 32 bits,
// Double.toInt() rounds toward zero and saturates, and a conversion to
// Byte, Short or Char then keeps the low bits of the Int.
func (g *gen) convert(from, to types.Type) {
	f, t := stackKind(from), stackKind(to)
	if op, ok := convOps[[2]byte{f, t}]; ok {
		g.code.Op(op)
	}
	switch {
	case to.K == types.Byte && from.K != types.Byte:
		g.code.Op(cf.I2B)
	case to.K == types.Short && from.K != types.Short && from.K != types.Byte:
		g.code.Op(cf.I2S)
	case to.K == types.Char && from.K != types.Char:
		g.code.Op(cf.I2C)
	}
}

// stackKind returns the kind of value a primitive is on the JVM's operand
// stack: I, J, F or D.
func stackKind(t types.Type) byte {
	switch t.K {
	case types.Long:
		return 'J'
	case types.Float:
		return 'F'
	case types.Double:
		return 'D'
	}
	return 'I'
}

var convOps = map[[2]byte]byte{
	{'I', 'J'}: cf.I2L, {'I', 'F'}: cf.I2F, {'I', 'D'}: cf.I2D,
	{'J', 'I'}: cf.L2I, {'J', 'F'}: cf.L2F, {'J', 'D'}: cf.L2D,
	{'F', 'I'}: cf.F2I, {'F', 'J'}: cf.F2L, {'F', 'D'}: cf.F2D,
	{'D', 'I'}: cf.D2I, {'D', 'J'}: cf.D2L, {'D', 'F'}: cf.D2F,
}

// whenBody generates the body of a when's branch: the subject, if it is a
// temporary, is dead there.
func (g *gen) whenBody(subj *types.VarSym, b *syntax.Block, t types.Type, value bool) {
	if subj == nil || g.sm == nil {
		g.block(b, t, value)
		return
	}
	if name, _ := g.code.Named(subj.Slot); name == "" && !g.code.Retired(subj.Slot) {
		g.code.Retire(subj.Slot)
		defer g.code.Unretire(subj.Slot)
	}
	g.block(b, t, value)
}

func (g *gen) when(x *syntax.When, t types.Type, value bool) {
	var subj *types.VarSym
	if x.Subject != nil {
		subj = x.SubjSym
		subj.Slot = g.code.AllocLocal(desc(subj.Type))
		g.named(subj)
		g.exprAs(x.Subject, subj.Type)
		g.code.Local(storeOp(subj.Type), subj.Slot)
	}
	end := cf.NewLabel()
	if value {
		end = join(t)
	}
	// Each branch runs in a replace group if the checker says so, and no
	// branch matching in an empty one.
	body := func(i int, b *syntax.Block) {
		g.grouped(x.Groups, g.siteKey(x.P, "when", i), func() { g.whenBody(subj, b, t, value) })
	}
	matched := false
	for i, br := range x.Branches {
		if br.Else && br.Guard == nil {
			body(i, br.Body)
			matched = true
			break
		}
		next, bodyStart := cf.NewLabel(), cf.NewLabel()
		for j := range br.Conds {
			c := &br.Conds[j]
			last := j == len(br.Conds)-1
			// Jump to the body if any condition holds; to the next branch
			// if the last one fails.
			target, jumpIf := bodyStart, true
			if last {
				target, jumpIf = next, false
			}
			switch {
			case c.Is || c.NotIs:
				g.code.Local(loadOp(subj.Type), subj.Slot)
				g.instanceOf(subj.Type, c.Target)
				if c.Is == jumpIf {
					g.code.Jump(cf.IFNE, target)
				} else {
					g.code.Jump(cf.IFEQ, target)
				}
			case (c.In || c.NotIn) && c.Call != nil:
				// X.contains(subject)
				g.exprAs(c.Call, types.BoolType)
				if c.In == jumpIf {
					g.code.Jump(cf.IFNE, target)
				} else {
					g.code.Jump(cf.IFEQ, target)
				}
			case c.In || c.NotIn:
				r := c.X.(*syntax.Binary)
				g.inRange(nil, func() { g.code.Local(loadOp(subj.Type), subj.Slot) }, subj.Type, r, c.In, target, jumpIf)
			case subj != nil:
				load := func() { g.code.Local(loadOp(subj.Type), subj.Slot) }
				lt := subj.Type
				if c.SubjType.K != types.Invalid {
					lt = c.SubjType
					load = func() {
						g.code.Local(loadOp(subj.Type), subj.Slot)
						g.castErased(subj.Type, lt)
					}
				}
				g.equals(load, lt, c.X, c.OpType, syntax.EQ, target, jumpIf)
			default:
				g.cond(c.X, target, jumpIf)
			}
		}
		g.code.Place(bodyStart)
		if br.Guard != nil {
			g.cond(br.Guard, next, false)
		}
		body(i, br.Body)
		g.code.Jump(cf.GOTO, end)
		g.code.Place(next)
	}
	if !matched && x.NoMatch == "" {
		g.grouped(x.Groups, g.siteKey(x.P, "when", len(x.Branches)), func() {})
	}
	if x.NoMatch != "" {
		// An exhaustive when without else: no branch matching is a bug.
		g.code.New(x.NoMatch)
		g.code.Op(cf.DUP)
		g.code.Invoke(cf.INVOKESPECIAL, x.NoMatch, "<init>", "()V")
		g.code.Op(cf.ATHROW)
	}
	g.code.Place(end)
}

func (g *gen) jump(x *syntax.Jump) {
	switch x.Kind {
	case syntax.THROW:
		g.expr(x.X)
		g.code.Op(cf.ATHROW)
	case syntax.RETURN:
		if x.Lambda != nil && x.Lambda.Inline {
			g.inlineReturn(x)
			return
		}
		if n := len(g.srcInl); n > 0 && x.Lambda == nil {
			g.srcReturn(x, g.srcInl[n-1]) // from a copy of an inline function
			return
		}
		// The value is computed before the finally blocks of the tries
		// the return leaves run.
		t := g.retType
		inLambda := g.lam != nil && !g.lam.fun
		if inLambda {
			g.lambdaValue(x.X) // return@label
		} else if x.X != nil {
			g.exprAs(x.X, t)
		}
		if g.hasFinally(0) && g.code.Reachable() {
			tmp := -1
			if size(t) > 0 && (x.X != nil || !inLambda) {
				tmp = g.code.AllocLocal(desc(t))
				g.code.Local(storeOp(t), tmp)
			}
			g.leaveTries(0)
			if tmp >= 0 {
				g.code.Local(loadOp(t), tmp)
				g.code.Retire(tmp)
			}
		}
		if inLambda {
			g.lambdaFinish()
			return
		}
		if g.valueCtor {
			g.loadThis() // a value class's constructor returns the value
			t = g.owner.Underlying.Type
		}
		g.ret(t)
	case syntax.BREAK, syntax.CONTINUE:
		i := len(g.loops) - 1
		for i > 0 && g.loops[i].node != x.Loop {
			i--
		}
		loop := g.loops[i]
		g.leaveTries(g.loopTries(i))
		if g.code.Reachable() && g.code.Depth() < loop.depth {
			panic("a break or continue leaves an inlined function whose code spilled the stack")
		}
		g.code.PopTo(loop.depth) // values an inlined lambda's code left
		g.endGroups(loop.groups)
		if x.Kind == syntax.BREAK {
			g.code.Jump(cf.GOTO, loop.brk)
		} else {
			g.code.Jump(cf.GOTO, loop.cont)
		}
	}
}

// tailCall generates a tailrec function's call of itself in tail
// position, as kotlinc does: the arguments, in the order of the
// parameters and with zeros for omitted ones, are stored in the
// parameters, a new receiver of an extension function in its receiver,
// the omitted ones get their default values, which can read the new
// values of the parameters before them, and the code jumps back to the
// start of the body.
func (g *gen) tailCall(x *syntax.Call, t *tailrec) {
	f, fd := x.Sym, t.fd
	m, recv := x.Fun.(*syntax.Member)
	if recv {
		if _, this := m.X.(*syntax.This); this || fd.RecvSym == nil {
			recv = false
		}
	}
	g.contextArgs(x, f)
	if recv {
		g.exprAs(m.X, f.Recv)
	}
	mask := g.callArgs(x, f)
	for i := len(f.Params) - 1; i >= 0; i-- {
		p := f.Params[i]
		g.code.Local(storeOp(p.Type), p.Slot)
	}
	if recv {
		g.code.Local(storeOp(fd.RecvSym.Type), fd.RecvSym.Slot)
	}
	for i := len(f.Contexts) - 1; i >= 0; i-- {
		p := f.Contexts[i]
		g.code.Local(storeOp(p.Type), p.Slot)
	}
	for i, p := range f.Params {
		if mask&(1<<i) != 0 && fd.Params[i].Default != nil {
			g.exprAs(fd.Params[i].Default, p.Type)
			g.code.Local(storeOp(p.Type), p.Slot)
		}
	}
	g.code.Jump(cf.GOTO, t.start)
}
