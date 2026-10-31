package codegen

import (
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Arrays: the array classes' members, their constructors and factories,
// vararg arrays and the spread operator, and loops over arrays.

// primArray describes the JVM array of a primitive element type, by the
// element's descriptor.
type primArray struct {
	atype       byte // NEWARRAY's operand
	load, store byte
}

var primArrays = map[byte]primArray{
	'I': {cf.TInt, cf.IALOAD, cf.IASTORE},
	'J': {cf.TLong, cf.LALOAD, cf.LASTORE},
	'D': {cf.TDouble, cf.DALOAD, cf.DASTORE},
	'Z': {cf.TBoolean, cf.BALOAD, cf.BASTORE},
	'C': {cf.TChar, cf.CALOAD, cf.CASTORE},
	'F': {cf.TFloat, cf.FALOAD, cf.FASTORE},
	'B': {cf.TByte, cf.BALOAD, cf.BASTORE},
	'S': {cf.TShort, cf.SALOAD, cf.SASTORE},
}

// className returns the class a descriptor names, as instructions take
// it: an internal name, or an array's descriptor.
func className(d string) string {
	if d[0] == 'L' {
		return d[1 : len(d)-1]
	}
	return d
}

// newArray creates an array with descriptor d; the length is on the stack.
// newArrayOf creates an array of descriptor d whose elements have type
// elem: one of a reified type parameter is marked for its copies.
func (g *gen) newArrayOf(d string, elem types.Type) {
	if g.reifiedParam(elem) && d[1] != '[' {
		g.reifiedMarker(elem, reifiedNewArray)
	}
	g.newArray(d)
}

func (g *gen) newArray(d string) {
	comp := d[1:]
	if pa, ok := primArrays[comp[0]]; ok {
		g.code.NewArray(pa.atype)
		return
	}
	g.code.ANewArray(className(comp))
}

// elemLoad and elemStore load and store an element of an array with
// descriptor d; the array and the index, and for a store the value, are
// on the stack.
func (g *gen) elemLoad(d string) {
	if pa, ok := primArrays[d[1]]; ok {
		g.code.Op(pa.load)
		return
	}
	g.code.Op(cf.AALOAD)
}

func (g *gen) elemStore(d string) {
	if pa, ok := primArrays[d[1]]; ok {
		g.code.Op(pa.store)
		return
	}
	g.code.Op(cf.AASTORE)
}

// arrayMemberType returns the type of the value an array member leaves
// on the stack, for an array of type rt.
func arrayMemberType(b types.Builtin, rt types.Type) types.Type {
	switch b {
	case types.ArrayGet:
		return types.StoredElem(rt)
	case types.ArraySet:
		return types.UnitType
	}
	return rt
}

// arrayMember emits a member of an array of type rt, which is on the
// stack: get, set or clone.
func (g *gen) arrayMember(x *syntax.Call, rt types.Type) {
	d := desc(rt)
	switch x.Builtin {
	case types.ArrayGet:
		g.exprAs(x.Args[0], types.IntType)
		g.elemLoad(d)
	case types.ArraySet:
		g.exprAs(x.Args[0], types.IntType)
		g.exprAs(x.Args[1], types.StoredElem(rt))
		g.elemStore(d)
	case types.ArrayClone:
		g.code.Invoke(cf.INVOKEVIRTUAL, d, "clone", "()Ljava/lang/Object;")
		g.code.Checkcast(d)
	}
}

// arrayFactory generates a call that creates an array of type x.T:
// IntArray(n), Array(n) { init }, arrayOf(...) and the like.
func (g *gen) arrayFactory(x *syntax.Call) {
	t := g.reify(x.T.NonNull())
	d := desc(t)
	switch x.Builtin {
	case types.ArrayLiteral:
		g.varargs(x.Args, d, types.StoredElem(t))
	case types.ArrayNew:
		g.exprAs(x.Args[0], types.IntType)
		g.newArrayOf(d, types.ArrayElem(t))
	case types.ArrayInit:
		// The init function is called with each index in turn.
		ft := x.Sym.Params[1].Type
		g.exprAs(x.Args[0], types.IntType)
		n := g.code.AllocLocal("I")
		g.code.Local(cf.ISTORE, n)
		g.exprAs(x.Args[1], ft)
		fn := g.code.AllocLocal(desc(ft))
		g.code.Local(cf.ASTORE, fn)
		g.code.Local(cf.ILOAD, n)
		g.newArrayOf(d, types.ArrayElem(t))
		arr := g.code.AllocLocal(d)
		g.code.Local(cf.ASTORE, arr)
		g.indexLoop(n, func(i int) {
			g.code.Local(cf.ALOAD, arr)
			g.code.Local(cf.ILOAD, i)
			g.code.Local(cf.ALOAD, fn)
			g.code.Local(cf.ILOAD, i)
			g.box(types.IntType)
			g.code.InvokeInterface(ft.C.Name, "invoke", "(Ljava/lang/Object;)Ljava/lang/Object;")
			g.castErased(types.AnyType.Nullable(), types.StoredElem(t))
			g.elemStore(d)
		}, nil)
		g.code.Local(cf.ALOAD, arr)
	}
}

// enumIntrinsic generates enumValues<T>() as T.values() and
// enumValueOf<T>(name) as T.valueOf(name), with T known in the copy of an
// inline function. In the function's own body, T isn't known, and the
// call throws, as with kotlinc.
func (g *gen) enumIntrinsic(x *syntax.Call) {
	e := g.reify(x.T.NonNull())
	if x.Builtin == types.EnumValues {
		e = types.ArrayElem(e)
	}
	if g.reifiedParam(e) {
		// kotlinc's placeholders, which copies replace, exactly: an empty
		// array of enums for values, and Enum.valueOf(null, name), with
		// the name in a local.
		if x.Builtin == types.EnumValues {
			g.reifiedMarker(e, reifiedEnum)
			g.code.Iconst(0)
			g.code.ANewArray("java/lang/Enum")
			return
		}
		g.exprAs(x.Args[0], types.StringType)
		name := g.code.AllocLocal("Ljava/lang/String;")
		g.code.Local(cf.ASTORE, name)
		g.reifiedMarker(e, reifiedEnum)
		g.code.Op(cf.ACONST_NULL)
		g.code.Local(cf.ALOAD, name)
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/Enum", "valueOf", "(Ljava/lang/Class;Ljava/lang/String;)Ljava/lang/Enum;")
		return
	}
	if e.K != types.Class {
		const exc = "java/lang/UnsupportedOperationException"
		g.code.New(exc)
		g.code.Op(cf.DUP)
		g.code.Sconst("This function has a reified type parameter and thus can only be inlined at compilation time, not called directly.")
		g.code.Invoke(cf.INVOKESPECIAL, exc, "<init>", "(Ljava/lang/String;)V")
		g.code.Op(cf.ATHROW)
		return
	}
	cls := e.C.Name
	if x.Builtin == types.EnumValues {
		g.code.Invoke(cf.INVOKESTATIC, cls, "values", "()[L"+cls+";")
		return
	}
	g.exprAs(x.Args[0], types.StringType)
	g.code.Invoke(cf.INVOKESTATIC, cls, "valueOf", "(Ljava/lang/String;)L"+cls+";")
}

// indexLoop generates a loop of index i from 0 to the int in local n,
// exclusive, with body. If labels isn't nil, the body may break and
// continue: labels receives the loop's labels.
func (g *gen) indexLoop(n int, body func(i int), labels func(brk, cont *cf.Label)) {
	i := g.code.AllocLocal("I")
	g.code.Iconst(0)
	g.code.Local(cf.ISTORE, i)
	top, cont, exit := cf.NewLabel(), cf.NewLabel(), cf.NewLabel()
	g.code.Place(top)
	g.code.Local(cf.ILOAD, i)
	g.code.Local(cf.ILOAD, n)
	g.code.Jump(cf.IF_ICMPGE, exit)
	if labels != nil {
		labels(exit, cont)
	}
	body(i)
	g.code.Place(cont)
	g.code.Iinc(i, 1)
	g.code.Jump(cf.GOTO, top)
	g.code.Place(exit)
}

// forArray generates a loop over an array's elements: by index, as
// kotlinc does.
func (g *gen) forArray(s *syntax.For) {
	rt := s.Range.Type().NonNull()
	d := desc(rt)
	g.expr(s.Range)
	arr := g.code.AllocLocal(d)
	g.code.Local(cf.ASTORE, arr)
	n := g.code.AllocLocal("I")
	g.code.Local(cf.ALOAD, arr)
	g.code.Op(cf.ARRAYLENGTH)
	g.code.Local(cf.ISTORE, n)
	s.Sym.Slot = g.code.AllocLocal(desc(s.Sym.Type))
	g.named(s.Sym)
	g.indexLoop(n, func(i int) {
		g.line(s.P)
		g.code.Local(cf.ALOAD, arr)
		g.code.Local(cf.ILOAD, i)
		g.elemLoad(d)
		g.coerce(types.StoredElem(rt), s.Sym.Type)
		g.code.Local(storeOp(s.Sym.Type), s.Sym.Slot)
		g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	}, func(brk, cont *cf.Label) {
		g.pushLoop(s, brk, cont)
	})
}

// forString generates a loop over a String's chars: by index, with
// charAt, as kotlinc does.
func (g *gen) forString(s *syntax.For) {
	g.expr(s.Range)
	str := g.code.AllocLocal("Ljava/lang/String;")
	g.code.Local(cf.ASTORE, str)
	n := g.code.AllocLocal("I")
	g.code.Local(cf.ALOAD, str)
	g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/String", "length", "()I")
	g.code.Local(cf.ISTORE, n)
	s.Sym.Slot = g.code.AllocLocal(desc(s.Sym.Type))
	g.named(s.Sym)
	g.indexLoop(n, func(i int) {
		g.line(s.P)
		g.code.Local(cf.ALOAD, str)
		g.code.Local(cf.ILOAD, i)
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/String", "charAt", "(I)C")
		g.code.Local(storeOp(s.Sym.Type), s.Sym.Slot)
		g.loopBody(func() { g.block(s.Body, types.UnitType, false) })
	}, func(brk, cont *cf.Label) {
		g.pushLoop(s, brk, cont)
	})
}

// indexLvalue reads and writes an element of an array, a[i], for
// compound assignment: the array and the index are pushed once.
func (g *gen) indexLvalue(x *syntax.Index) lvalue {
	rt := x.X.Type().NonNull()
	d, elem, t := desc(rt), types.StoredElem(rt), x.Call.T
	return lvalue{t: t, recv: 2,
		prepare: func() {
			g.expr(x.X)
			g.exprAs(x.Args[0], types.IntType)
		},
		load: func() {
			g.elemLoad(d)
			g.coerce(elem, t)
		},
		store: func() {
			g.coerce(t, elem)
			g.elemStore(d)
		}}
}

// varargs creates the array with descriptor d for a vararg parameter
// from the arguments passed to it, whose elements have type elem. A
// spread argument passes the elements of an array: a lone one is copied,
// since the callee may change its array, and several parts are copied
// into a new array of their total length.
func (g *gen) varargs(args []syntax.Expr, d string, elem types.Type) {
	spreads := 0
	for _, a := range args {
		if _, ok := a.(*syntax.Spread); ok {
			spreads++
		}
	}
	switch {
	case spreads == 0:
		g.code.Iconst(int32(len(args)))
		g.newArrayOf(d, elem)
		for k, a := range args {
			g.code.Op(cf.DUP)
			g.code.Iconst(int32(k))
			g.exprAs(a, elem)
			g.elemStore(d)
		}
		return
	case len(args) == 1:
		x := args[0].(*syntax.Spread).X
		sd := desc(x.Type().NonNull())
		g.expr(x)
		g.code.Invoke(cf.INVOKEVIRTUAL, sd, "clone", "()Ljava/lang/Object;")
		g.code.Checkcast(sd)
		return
	}
	// The parts are evaluated in order, into locals, before the array of
	// their total length is created.
	slots := make([]int, len(args))
	for k, a := range args {
		if s, ok := a.(*syntax.Spread); ok {
			sd := desc(s.X.Type().NonNull())
			g.expr(s.X)
			slots[k] = g.code.AllocLocal(sd)
			g.code.Local(cf.ASTORE, slots[k])
			continue
		}
		g.exprAs(a, elem)
		slots[k] = g.code.AllocLocal(desc(elem))
		g.code.Local(storeOp(elem), slots[k])
	}
	g.code.Iconst(int32(len(args) - spreads))
	for k, a := range args {
		if _, ok := a.(*syntax.Spread); ok {
			g.code.Local(cf.ALOAD, slots[k])
			g.code.Op(cf.ARRAYLENGTH)
			g.code.Op(cf.IADD)
		}
	}
	g.newArrayOf(d, elem)
	arr := g.code.AllocLocal(d)
	g.code.Local(cf.ASTORE, arr)
	pos := g.code.AllocLocal("I")
	g.code.Iconst(0)
	g.code.Local(cf.ISTORE, pos)
	for k, a := range args {
		if _, ok := a.(*syntax.Spread); !ok {
			g.code.Local(cf.ALOAD, arr)
			g.code.Local(cf.ILOAD, pos)
			g.code.Local(loadOp(elem), slots[k])
			g.elemStore(d)
			g.code.Iinc(pos, 1)
			continue
		}
		// System.arraycopy(part, 0, arr, pos, part.length)
		g.code.Local(cf.ALOAD, slots[k])
		g.code.Iconst(0)
		g.code.Local(cf.ALOAD, arr)
		g.code.Local(cf.ILOAD, pos)
		g.code.Local(cf.ALOAD, slots[k])
		g.code.Op(cf.ARRAYLENGTH)
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/System", "arraycopy", "(Ljava/lang/Object;ILjava/lang/Object;II)V")
		g.code.Local(cf.ILOAD, pos)
		g.code.Local(cf.ALOAD, slots[k])
		g.code.Op(cf.ARRAYLENGTH)
		g.code.Op(cf.IADD)
		g.code.Local(cf.ISTORE, pos)
	}
	g.code.Local(cf.ALOAD, arr)
}

// toTypedArray converts the collection on the stack to an array of type
// t, as Collection<T>.toTypedArray() does: toArray(new T[0]).
func (g *gen) toTypedArray(t types.Type) {
	t = g.reify(t)
	d := desc(t.NonNull())
	g.code.Iconst(0)
	g.newArrayOf(d, types.ArrayElem(t))
	g.code.InvokeInterface("java/util/Collection", "toArray", "([Ljava/lang/Object;)[Ljava/lang/Object;")
	g.code.Checkcast(d)
}
