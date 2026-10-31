package codegen

import (
	"path/filepath"
	"slices"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Values of value classes are their underlying values where their type is
// known (see types.Unboxed), and instances of the class, boxes, where it
// isn't: as an Any, an interface or a type argument. These convert
// between the two, as kotlinc does, with the class's box-impl and
// unbox-impl methods.

// boxValue boxes the unboxed value of value class type t on the stack. A
// nullable value that is null stays null.
func (g *gen) boxValue(t types.Type) {
	cls := t.C.Name
	box := func() {
		g.code.Invoke(cf.INVOKESTATIC, cls, "box-impl", "("+desc(types.Repr(t.NonNull()))+")L"+cls+";")
	}
	if !t.N {
		box()
		return
	}
	nonNull, end := cf.NewLabel(), cf.NewJoin("L"+cls+";")
	g.code.Op(cf.DUP)
	g.code.Jump(cf.IFNONNULL, nonNull)
	g.code.Op(cf.POP)
	g.code.Op(cf.ACONST_NULL)
	g.code.Jump(cf.GOTO, end)
	g.code.Place(nonNull)
	box()
	g.code.Place(end)
}

// unboxValue unboxes the instance of the value class of type t on the
// stack, already cast to the class, to its underlying value. A nullable
// one that is null stays null.
func (g *gen) unboxValue(t types.Type) {
	cls := t.C.Name
	r := types.Repr(t.NonNull())
	unbox := func() {
		g.code.Invoke(cf.INVOKEVIRTUAL, cls, "unbox-impl", "()"+desc(r))
	}
	if !t.N {
		unbox()
		return
	}
	nonNull, end := cf.NewLabel(), join(t)
	g.code.Op(cf.DUP)
	g.code.Jump(cf.IFNONNULL, nonNull)
	g.code.Op(cf.POP)
	g.code.Op(cf.ACONST_NULL)
	g.code.Jump(cf.GOTO, end)
	g.code.Place(nonNull)
	unbox()
	g.code.Place(end)
}

// coerceValue converts the value on the stack from type from to type to
// when either is an unboxed value class type: it boxes, or casts the box
// and unboxes.
func (g *gen) coerceValue(from, to types.Type) {
	switch fu, tu := types.Unboxed(from), types.Unboxed(to); {
	case fu && tu:
		// The same class: its underlying values.
	case fu:
		g.boxValue(from)
	default:
		g.castBox(from, to)
		g.unboxValue(to)
	}
}

// castBox casts the value on the stack, of type from, to the class of
// value class type to, unless from is already that class.
func (g *gen) castBox(from, to types.Type) {
	if from.IsParam() {
		from = types.Erase(from)
	}
	if d := types.BoxedDesc(to); desc(from) != d || types.Unboxed(from) {
		g.code.Checkcast(to.C.Name)
	}
}

// valueClass generates a value class as kotlinc does. Its instances, the
// boxes, have the underlying value in a field. Its members are static
// methods that take the underlying value first, as this; those that
// override an interface's or Any's member also have an instance method
// that calls them on the box's value. The constructor is the static
// constructor-impl, which runs the init blocks and returns the value.
func (g *gen) valueClass(cd *syntax.ClassDecl) *cf.Class {
	cls := cd.Sym
	u := cls.Underlying
	ud := desc(u.Type)
	self := "L" + cls.Name + ";"
	c := cf.NewClass(cf.AccPublic|cf.AccFinal|cf.AccSuper, cls.Name, "java/lang/Object")
	// As for other classes, the Java interfaces of Kotlin's collection
	// types come first, then their marker, then the other interfaces.
	for _, i := range cls.Interfaces {
		if i.Java != nil {
			c.Interfaces = append(c.Interfaces, i.Name)
		}
	}
	c.Interfaces = append(c.Interfaces, collectionMarkers(cls)...)
	for _, i := range cls.Interfaces {
		if i.Java == nil {
			c.Interfaces = append(c.Interfaces, i.Name)
		}
	}
	c.Signature = classSignature(cls, c.Super, c.Interfaces)
	c.SourceFile = filepath.Base(g.file.Path)
	g.cls, g.owner = c, cls
	c.AddField(cf.AccPrivate|cf.AccFinal, u.Name, ud)
	nullability(c.LastField().Annotate, u.Type)

	// The box's value: the underlying property's getter, and unbox-impl.
	acc := uint16(cf.AccPublic)
	if u.Final {
		acc |= cf.AccFinal
	}
	getField := func() {
		g.code.Local(cf.ALOAD, 0)
		g.code.FieldInsn(cf.GETFIELD, cls.Name, u.Name, ud)
	}
	// As with kotlinc, the methods that box and unbox are synthetic, and a
	// non-public underlying property has no getter.
	names := []string{types.GetterJVM(u), "unbox-impl"}
	if !underlyingGetter(cls) {
		names, acc = names[1:], cf.AccPublic|cf.AccFinal|cf.AccSynthetic
	}
	for _, name := range names {
		g.code = c.NewMethod(acc, name, "()"+ud)
		if acc&cf.AccSynthetic == 0 {
			nullability(methodAdder(c.LastMethod()), u.Type)
		}
		g.code.AllocLocal(self)
		getField()
		g.code.Op(returnOp(u.Type))
		g.check()
		acc = cf.AccPublic | cf.AccFinal | cf.AccSynthetic
	}
	for _, b := range u.Bridges {
		g.bridge(b, types.GetterJVM(u), nil, u.Type, false, nil, nil)
	}

	// Members, as static methods, and the instance methods of overrides.
	declared := map[string]bool{}
	var fds []*syntax.FunDecl
	for _, m := range cd.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			fds = append(fds, m)
		case *syntax.PropDecl:
			if m.Getter == nil {
				continue
			}
			m.Getter.Override = m.Getter.Override || m.Override
			fds = append(fds, m.Getter)
			if st := m.Setter; st != nil && st.Sym != nil {
				st.Override = st.Override || m.Override
				fds = append(fds, st)
			}
		}
	}
	for _, fd := range fds {
		f := fd.Sym
		g.valueMember(fd)
		if hasDefaults(fd.Params) {
			g.defaults(fd, cls)
		}
		if fd.Override {
			declared[types.InstanceJVM(f)+funDesc(f)] = true
			g.valueOverride(f)
		}
	}

	// The members with bodies of the interfaces it implements that it
	// doesn't override: an instance method that calls the interface's,
	// and a static one, for callers with the value, that calls it on the
	// box. Those of the program's interfaces are default methods, those of
	// Kotlin interfaces on the class path in DefaultImpls. The members of
	// an interface it implements by delegation to its value are, as
	// kotlinc does, static methods that call them on the value, and the
	// instance methods that call those.
	for _, dl := range cd.Delegated {
		g.delegated(dl)
		declared[types.InstanceJVM(dl.Sym)+funDesc(dl.Sym)] = true
		if dl.Super == nil {
			g.valueInherited(dl.Sym)
		}
	}
	cls.Supertypes(func(s *types.ClassSym) bool {
		sd, ok := s.Decl.(*syntax.ClassDecl)
		if s == cls || !s.Interface || s.External || !ok {
			return true
		}
		for _, m := range sd.Members {
			fd, ok := m.(*syntax.FunDecl)
			if !ok || fd.Recv != nil || fd.Sym == nil || fd.Sym.Abstract || declared[fd.Sym.JVM()+funDesc(fd.Sym)] {
				continue
			}
			f := fd.Sym
			declared[f.JVM()+funDesc(f)] = true
			g.code = c.NewMethod(cf.AccPublic, f.JVM(), funDesc(f))
			g.code.AllocLocal(self)
			g.code.Local(cf.ALOAD, 0)
			for _, p := range f.Params {
				g.code.Local(loadOp(p.Type), g.code.AllocLocal(desc(p.Type)))
			}
			g.code.InvokeSpecialInterface(s.Name, f.JVM(), funDesc(f))
			g.code.Op(returnOp(f.Ret))
			g.check()
			g.valueInherited(f)
		}
		return true
	})

	g.collectionMembers(cls)

	// toString, hashCode and equals, unless declared, as their -impl
	// methods and the instance methods that call them.
	if !declared["toString()Ljava/lang/String;"] {
		g.valueMethod(cf.AccPublic|cf.AccStatic, "toString-impl", "("+ud+")Ljava/lang/String;", func() {
			g.newStringBuilder()
			g.code.Sconst(cls.DisplayName() + "(" + u.Name + "=")
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
			g.code.Local(loadOp(u.Type), 0)
			if types.Unboxed(u.Type) {
				g.boxValue(u.Type) // a value class's, its toString
				g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.AnyType))
			} else {
				g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(u.Type))
			}
			g.code.Sconst(")")
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
			g.code.Op(cf.ARETURN)
		})
		g.valueBridge("toString", "()Ljava/lang/String;", &types.FunSym{JVMName: "toString-impl", Ret: types.StringType})
	}
	if !declared["hashCode()I"] {
		g.valueMethod(cf.AccPublic|cf.AccStatic, "hashCode-impl", "("+ud+")I", func() {
			if types.Unboxed(u.Type) {
				// A value class's, its hashCode.
				g.code.Local(loadOp(u.Type), 0)
				g.code.Invoke(cf.INVOKESTATIC, u.Type.C.Name, "hashCode-impl", "("+ud+")I")
			} else {
				g.hashOf(func() { g.code.Local(loadOp(u.Type), 0) }, u.Type)
			}
			g.code.Op(cf.IRETURN)
		})
		g.valueBridge("hashCode", "()I", &types.FunSym{JVMName: "hashCode-impl", Ret: types.IntType})
	}
	if !declared["equals(Ljava/lang/Object;)Z"] {
		g.valueMethod(cf.AccPublic|cf.AccStatic, "equals-impl", "("+ud+"Ljava/lang/Object;)Z", func() {
			other := 1
			if size(u.Type) == 2 {
				other = 2
			}
			no, yes := cf.NewLabel(), cf.NewLabel()
			g.code.Local(cf.ALOAD, other)
			g.code.InstanceOf(cls.Name)
			g.code.Jump(cf.IFEQ, no)
			g.code.Local(loadOp(u.Type), 0)
			g.code.Local(cf.ALOAD, other)
			g.code.Checkcast(cls.Name)
			g.code.Invoke(cf.INVOKEVIRTUAL, cls.Name, "unbox-impl", "()"+ud)
			g.jumpIfEqual(u.Type, yes)
			g.code.Place(no)
			g.code.Iconst(0)
			g.code.Op(cf.IRETURN)
			g.code.Place(yes)
			g.code.Iconst(1)
			g.code.Op(cf.IRETURN)
		}, "Ljava/lang/Object;")
		eq := &types.FunSym{JVMName: "equals-impl", Ret: types.BoolType,
			Params: []*types.VarSym{{Name: "other", Type: types.AnyType.Nullable(), Kind: types.Param}}}
		g.valueBridge("equals", "(Ljava/lang/Object;)Z", eq)
	}

	// The box's constructor, box-impl and equals-impl0.
	g.code = c.NewMethod(cf.AccPrivate|cf.AccSynthetic, "<init>", "("+ud+")V")
	g.code.AllocLocal(self)
	g.code.AllocLocal(ud)
	g.code.Local(cf.ALOAD, 0)
	g.code.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	g.code.Local(cf.ALOAD, 0)
	g.code.Local(loadOp(u.Type), 1)
	g.code.FieldInsn(cf.PUTFIELD, cls.Name, u.Name, ud)
	g.code.Op(cf.RETURN)
	g.check()
	g.valueMethod(cf.AccPublic|cf.AccStatic|cf.AccFinal|cf.AccSynthetic, "box-impl", "("+ud+")"+self, func() {
		g.code.New(cls.Name)
		g.code.Op(cf.DUP)
		g.code.Local(loadOp(u.Type), 0)
		g.code.Invoke(cf.INVOKESPECIAL, cls.Name, "<init>", "("+ud+")V")
		g.code.Op(cf.ARETURN)
	})
	g.valueMethod(cf.AccPublic|cf.AccStatic|cf.AccFinal, "equals-impl0", "("+ud+ud+")Z", func() {
		yes := cf.NewLabel()
		g.code.Local(loadOp(u.Type), 0)
		g.code.Local(loadOp(u.Type), size(u.Type))
		g.jumpIfEqual(u.Type, yes)
		g.code.Iconst(0)
		g.code.Op(cf.IRETURN)
		g.code.Place(yes)
		g.code.Iconst(1)
		g.code.Op(cf.IRETURN)
	}, ud)

	// constructor-impl runs the init blocks on the value, its parameter.
	// It is private if the constructor is: other classes call it through
	// access$constructor-impl.
	cp := cd.CtorParams[0]
	acc = cf.AccPublic
	if cls.Ctor.Private {
		acc = cf.AccPrivate
	}
	g.code = c.NewMethod(acc|cf.AccStatic, "constructor-impl", "("+ud+")"+ud)
	if !cls.Ctor.Private {
		nullability(methodAdder(c.LastMethod()), u.Type)
		nullability(paramAdder(c.LastMethod(), 0), u.Type)
	}
	g.prefix = "_init_$lambda"
	g.retType = types.UnitType
	g.loops, g.tries = nil, nil
	cp.Sym.Slot = g.code.AllocLocal(ud)
	g.named(cp.Sym)
	if !cd.Sym.Ctor.Private {
		g.checkParam(cp.Sym.Slot, u.Type, cp.Name)
	}
	g.valueThis = true
	for _, m := range cd.Members {
		if ib, ok := m.(*syntax.InitBlock); ok {
			g.block(ib.Body, types.UnitType, false)
		}
	}
	g.valueThis = false
	g.code.Local(loadOp(u.Type), 0)
	g.code.Op(returnOp(u.Type))
	g.check()
	if cp.Default != nil {
		// constructor-impl$default, for a call that omits the value,
		// computes its default value.
		g.code = c.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccSynthetic, "constructor-impl$default", "("+ud+"ILkotlin/jvm/internal/DefaultConstructorMarker;)"+ud)
		g.prefix = "constructor_impl$default$lambda"
		g.code.AllocLocal(ud)
		mask := g.code.AllocLocal("I")
		g.code.AllocLocal("Lkotlin/jvm/internal/DefaultConstructorMarker;")
		skip := cf.NewLabel()
		g.code.Local(cf.ILOAD, mask)
		g.code.Iconst(1)
		g.code.Op(cf.IAND)
		g.code.Jump(cf.IFEQ, skip)
		g.exprAs(cp.Default, u.Type)
		g.code.Local(storeOp(u.Type), 0)
		g.code.Place(skip)
		g.code.Local(loadOp(u.Type), 0)
		g.code.Invoke(cf.INVOKESTATIC, cls.Name, "constructor-impl", "("+ud+")"+ud)
		g.code.Op(returnOp(u.Type))
		g.check()
	}
	for _, sc := range cd.Secondaries() {
		g.valueSecondary(cd, sc)
	}
	g.staticInit(cd)
	return c
}

// valueCtorCall calls a value class's constructor, which returns the
// value: its static constructor-impl, or constructor-impl$default if
// arguments are omitted. A private secondary constructor is called
// through its accessor from other classes.
func (g *gen) valueCtorCall(x *syntax.Call) {
	f := x.Sym
	mask := g.callArgs(x, f)
	u := desc(x.Ctor.Underlying.Type)
	d := "(" + u + ")" + u
	switch {
	case x.Ctor.External && strings.HasSuffix(f.Desc, ")"+u):
		d = f.Desc // a library's other constructor: UByteArray(size)
	case !x.Ctor.External && f != x.Ctor.Ctor:
		d = methodDesc(f.Params, x.Ctor.This()) // a secondary constructor
	}
	if mask != 0 {
		// Arguments omitted: their default values.
		g.code.Iconst(int32(mask))
		g.code.Op(cf.ACONST_NULL)
		i := strings.IndexByte(d, ')')
		g.code.Invoke(cf.INVOKESTATIC, x.Ctor.Name, "constructor-impl$default", d[:i]+"ILkotlin/jvm/internal/DefaultConstructorMarker;"+d[i:])
		return
	}
	g.callStatic(f, x.Ctor.Name, "constructor-impl", d)
}

// valueSecondary generates a secondary constructor of a value class, as
// kotlinc does: a static constructor-impl that takes its parameters,
// calls the constructor it delegates to for the value, runs its body with
// the value as this, and returns the value.
func (g *gen) valueSecondary(cd *syntax.ClassDecl, sc *syntax.SecondaryCtor) {
	cls := cd.Sym
	f := sc.Sym
	u := cls.Underlying.Type
	d := methodDesc(f.Params, cls.This())
	access := uint16(cf.AccPublic | cf.AccStatic)
	if f.Private {
		access = cf.AccPrivate | cf.AccStatic
	}
	if hasVararg(f.Params) {
		access |= cf.AccVarargs
	}
	g.code = g.cls.NewMethod(access, "constructor-impl", d)
	m := g.cls.LastMethod()
	writeAnnotations(methodAdder(m), sc.Annotations, all)
	if !f.Private {
		nullability(methodAdder(m), u)
	}
	for i, p := range f.Params {
		writeAnnotations(paramAdder(m, i), sc.Params[i].Annotations, all)
		if !f.Private {
			nullability(paramAdder(m, i), p.Type)
		}
	}
	g.prefix = "_init_$lambda"
	g.retType = types.UnitType
	g.loops, g.tries = nil, nil
	for _, p := range f.Params {
		p.Slot = g.code.AllocLocal(paramDesc(p.Type))
		g.named(p)
	}
	g.line(sc.P)
	if !f.Private {
		g.checkParams(nil, f.Params)
	}
	g.valueCtorCall(sc.Delegation)
	self := g.code.AllocLocal(desc(u))
	g.code.Local(storeOp(u), self)
	if sc.Body != nil {
		g.valueThis, g.valueSelf, g.valueCtor = true, self, true
		g.block(sc.Body, types.UnitType, false)
		g.valueThis, g.valueSelf, g.valueCtor = false, 0, false
	}
	if g.code.Reachable() {
		if sc.Body != nil {
			g.line(sc.Body.End)
		}
		g.code.Local(loadOp(u), self)
		g.code.Op(returnOp(u))
	}
	g.check()
	if hasDefaults(sc.Params) {
		// constructor-impl$default computes the omitted arguments.
		i := strings.IndexByte(d, ')')
		access := uint16(cf.AccPublic | cf.AccStatic | cf.AccSynthetic)
		if f.Private {
			access &^= cf.AccPublic
		}
		g.code = g.cls.NewMethod(access, "constructor-impl$default", d[:i]+"ILkotlin/jvm/internal/DefaultConstructorMarker;"+d[i:])
		g.prefix = "constructor_impl$default$lambda"
		g.fillDefaults(sc.Params, f.Params)
		for _, p := range f.Params {
			g.code.Local(loadOp(p.Type), p.Slot)
		}
		g.code.Invoke(cf.INVOKESTATIC, cls.Name, "constructor-impl", d)
		g.code.Op(returnOp(u))
		g.check()
	}
}

// valueMethod generates a static method of a value class whose code body
// generates, with the underlying value in local 0 and the locals of more
// parameters.
func (g *gen) valueMethod(access uint16, name, d string, body func(), more ...string) {
	g.code = g.cls.NewMethod(access, name, d)
	g.code.AllocLocal(desc(g.owner.Underlying.Type))
	for _, l := range more {
		g.code.AllocLocal(l)
	}
	body()
	g.check()
}

// valueInherited generates the static method of a value class for a
// member f that it inherits: it calls f on the box.
func (g *gen) valueInherited(f *types.FunSym) {
	cls := g.owner
	ud := desc(cls.Underlying.Type)
	fd := funDesc(f)
	name := f.JVM()
	if name == f.Name {
		name += "-impl" // unless mangled
	}
	g.code = g.cls.NewMethod(cf.AccPublic|cf.AccStatic, name, "("+ud+fd[1:])
	g.code.AllocLocal(ud)
	g.code.Local(loadOp(cls.Underlying.Type), 0)
	g.boxValue(cls.This())
	for _, p := range f.Params {
		g.code.Local(loadOp(p.Type), g.code.AllocLocal(desc(p.Type)))
	}
	g.code.Invoke(cf.INVOKEVIRTUAL, cls.Name, f.JVM(), fd)
	g.code.Op(returnOp(f.Ret))
	g.check()
}

// valueMember generates a member function of a value class, or a
// property's getter: a static method that takes the underlying value as
// this, first.
func (g *gen) valueMember(fd *syntax.FunDecl) {
	f := fd.Sym
	access := uint16(cf.AccPublic | cf.AccStatic)
	if !fd.Override {
		access |= cf.AccFinal
	}
	if hasVararg(f.Params) {
		access |= cf.AccVarargs
	}
	ud := desc(g.owner.Underlying.Type)
	d := "(" + ud + funDesc(f)[1:]
	g.code = g.cls.NewMethod(access, f.JVM(), d)
	m := g.cls.LastMethod()
	g.code.AllocLocal(ud)
	if f.Suspend {
		g.code.NameLocal(0, "arg0") // as kotlinc names it
		// The continuation calls it with the value it saved.
		owner, name := g.cls.Name, f.JVM()
		g.resume = &resumeCall{call: func(g *gen) { g.code.Invoke(cf.INVOKESTATIC, owner, name, d) }}
	}
	g.valueThis = true
	defer func() { g.valueThis = false }()
	g.functionBody(fd)
	add := methodAdder(m)
	writeAnnotations(add, fd.Annotations, all)
	nullability(add, f.Ret)
	for i, p := range f.Params {
		if i < len(fd.Params) {
			writeAnnotations(paramAdder(m, i+1), fd.Params[i].Annotations, all)
		}
		nullability(paramAdder(m, i+1), p.Type)
	}
}

// valueOverride generates the instance method of member f of a value
// class that overrides an interface's, which its callers call, and its
// bridges: they call f's static method on the box's value.
func (g *gen) valueOverride(f *types.FunSym) {
	name := types.InstanceJVM(f)
	g.valueBridge(name, funDesc(f), f)
	params := f.Params
	if f.Ext {
		// A member extension's receiver comes first.
		params = append([]*types.VarSym{{Name: "<this>", Type: f.Recv, Kind: types.Param}}, params...)
	}
	for _, b := range f.Bridges {
		if !collectionBridge(g.owner, b) { // a collection's own bridge, as remove(int)
			g.bridge(b, name, params, f.Ret, slices.Contains(f.BoxingBridges, b), f.BridgeOf[b], f)
		}
	}
}

// valueBridge generates an instance method of a value class, named name
// with descriptor d, that calls the static method of f on the box's
// value.
func (g *gen) valueBridge(name, d string, f *types.FunSym) {
	cls := g.owner
	u := cls.Underlying
	ud := desc(u.Type)
	g.code = g.cls.NewMethod(cf.AccPublic, name, d)
	g.code.AllocLocal("L" + cls.Name + ";")
	g.code.Local(cf.ALOAD, 0)
	g.code.FieldInsn(cf.GETFIELD, cls.Name, u.Name, ud)
	fd := funDesc(f)
	from, to := descParams(d), descParams(fd)
	for i, pd := range from {
		slot := g.code.AllocLocal(pd)
		g.code.Local(loadOpOf(pd), slot)
		g.convertDesc(pd, to[i])
	}
	g.code.Invoke(cf.INVOKESTATIC, cls.Name, f.JVM(), "("+ud+fd[1:])
	ret := d[strings.IndexByte(d, ')')+1:]
	g.convertDesc(fd[strings.IndexByte(fd, ')')+1:], ret)
	g.code.Op(returnOpOf(ret))
	g.check()
}

// underlyingGetter reports whether the box of value class cls of the
// program has a getter of its underlying property: as with kotlinc, if
// the property is public.
func underlyingGetter(cls *types.ClassSym) bool {
	cp, ok := cls.Underlying.Decl.(*syntax.CtorParam)
	return !ok || cp.Visibility == syntax.Public
}

// valueGet reads property p of a value class from the unboxed value on
// the stack: the value itself for the underlying property, or the static
// method of the property's getter.
func (g *gen) valueGet(p *types.VarSym) {
	cls := p.Owner
	if p == cls.Underlying {
		return
	}
	g.code.Invoke(cf.INVOKESTATIC, cls.Name, types.GetterJVM(p), "("+desc(cls.Underlying.Type)+")"+desc(p.Type))
}

// boxReceiver boxes the receiver on the stack, of type rt, if it is an
// unboxed value class value whose member being called is inherited: a
// method of Any or of an interface, called on the box; or a primitive,
// whose Any members are called on its box. A Unit receiver, which has no
// value, is the kotlin.Unit object.
func (g *gen) boxReceiver(rt types.Type) {
	switch {
	case types.Unboxed(rt):
		g.boxValue(rt)
	case rt.IsPrimitive():
		g.box(rt) // Any's members of a number: 1.equals(x)
	case rt.K == types.Unit:
		g.unitValue()
	}
}
