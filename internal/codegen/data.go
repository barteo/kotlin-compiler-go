package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// dataMembers generates the members of a data class or data object that
// it doesn't declare, as kotlinc writes them: componentN and copy (with
// copy$default), and toString, hashCode and equals over the properties of
// the primary constructor.
func (g *gen) dataMembers(cd *syntax.ClassDecl) {
	cls := cd.Sym
	declared := func(name string, params int) bool {
		for _, m := range cd.Members {
			if fd, ok := m.(*syntax.FunDecl); ok && fd.Name == name && len(fd.Params) == params {
				return true
			}
		}
		return false
	}
	self := "L" + cls.Name + ";"
	name := cls.Name[strings.LastIndexAny(cls.Name, "/$")+1:]
	var access uint16 // componentN's, with its property's visibility
	newMethod := func(name, d string) {
		if access == 0 {
			access = cf.AccPublic
		}
		if name != "toString" && name != "hashCode" && name != "equals" {
			access |= cf.AccFinal // componentN and copy
		}
		g.code = g.cls.NewMethod(access, name, d)
		access = 0
		g.code.AllocLocal(self)
		if name == "toString" {
			nullability(methodAdder(g.cls.LastMethod()), types.StringType)
		}
	}
	if cd.Object {
		if !declared("toString", 0) {
			newMethod("toString", "()Ljava/lang/String;")
			g.code.Sconst(name)
			g.code.Op(cf.ARETURN)
			g.check()
		}
		if !declared("hashCode", 0) {
			newMethod("hashCode", "()I")
			g.code.Iconst(javaHash(name))
			g.code.Op(cf.IRETURN)
			g.check()
		}
		if !declared("equals", 1) {
			newMethod("equals", "(Ljava/lang/Object;)Z")
			g.code.AllocLocal("Ljava/lang/Object;")
			g.code.Local(cf.ALOAD, 1)
			g.code.InstanceOf(cls.Name)
			g.code.Op(cf.IRETURN)
			g.check()
		}
		return
	}
	var props []*types.VarSym
	for _, cp := range cd.CtorParams {
		props = append(props, cp.PropSym)
	}
	field := func(obj int, p *types.VarSym) {
		g.code.Local(cf.ALOAD, obj)
		g.code.FieldInsn(cf.GETFIELD, cls.Name, p.Name, desc(types.FieldType(p)))
	}
	for i, p := range props {
		f := cls.Methods[componentName(i)][0]
		if f.Decl != nil {
			continue // declared by the class
		}
		if f.Private {
			access = cf.AccPrivate
		}
		newMethod(f.JVM(), "()"+desc(p.Type))
		g.signAccessor(g.cls.LastMethod(), p.Type, propRef(p), "()"+desc(p.Type), cd.CtorParams[i].Modifiers.Annotations, cls)
		nullability(methodAdder(g.cls.LastMethod()), p.Type)
		field(0, p)
		g.coerce(types.FieldType(p), p.Type) // a Unit one's returns void
		g.code.Op(returnOp(p.Type))
		g.check()
	}
	if f := synthesized(cls, "copy"); f != nil {
		ctor := methodDesc(cd.Ctor.Params, types.UnitType)
		d := methodDesc(f.Params, f.Ret)
		newMethod(f.JVM(), d)
		m := g.cls.LastMethod()
		var ps []sigPart
		for i, p := range f.Params {
			ps = append(ps, typedPart(p.Type, propRef(props[i])))
		}
		if s := methodSig(nil, ps, typedPart(f.Ret, nil), d, 0, g.wildcards(nil, cls)); s != "" {
			m.SetSignature(s)
		}
		nullability(methodAdder(m), f.Ret)
		for i, p := range f.Params {
			p.Slot = g.code.AllocLocal(paramDesc(p.Type))
			g.named(p)
			nullability(paramAdder(m, i), p.Type)
		}
		g.checkParams(nil, f.Params)
		g.code.New(cls.Name)
		g.code.Op(cf.DUP)
		for _, p := range f.Params {
			g.code.Local(loadOp(p.Type), p.Slot)
		}
		g.code.Invoke(cf.INVOKESPECIAL, cls.Name, "<init>", ctor)
		g.code.Op(cf.ARETURN)
		g.check()

		// copy$default: an omitted argument is the property's value.
		i := strings.IndexByte(d, ')')
		g.code = g.cls.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccSynthetic, f.JVM()+"$default", "("+self+d[1:i]+"ILjava/lang/Object;"+d[i:])
		g.code.AllocLocal(self)
		for _, p := range f.Params {
			p.Slot = g.code.AllocLocal(paramDesc(p.Type))
			g.named(p)
		}
		mask := g.code.AllocLocal("I")
		g.code.AllocLocal("Ljava/lang/Object;")
		for j, p := range f.Params {
			skip := cf.NewLabel()
			g.code.Local(cf.ILOAD, mask)
			g.code.Iconst(int32(1) << j)
			g.code.Op(cf.IAND)
			g.code.Jump(cf.IFEQ, skip)
			field(0, props[j])
			g.code.Local(storeOp(p.Type), p.Slot)
			g.code.Place(skip)
		}
		g.code.Local(cf.ALOAD, 0)
		for _, p := range f.Params {
			g.code.Local(loadOp(p.Type), p.Slot)
		}
		g.code.Invoke(cf.INVOKEVIRTUAL, cls.Name, f.JVM(), d)
		g.code.Op(cf.ARETURN)
		g.check()
	}
	if !declared("toString", 0) {
		// Point(x=1, y=2)
		newMethod("toString", "()Ljava/lang/String;")
		g.newStringBuilder()
		for i, p := range props {
			sep := ", "
			if i == 0 {
				sep = name + "("
			}
			g.code.Sconst(sep + p.Name + "=")
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
			field(0, p)
			if types.Unboxed(p.Type) {
				g.boxValue(p.Type) // its toString
				g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.AnyType))
				continue
			}
			if p.Type.IsArray() {
				// An array's elements, as kotlinc writes them.
				g.code.Invoke(cf.INVOKESTATIC, "java/util/Arrays", "toString", "("+arraysParam(p.Type)+")Ljava/lang/String;")
				g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
				continue
			}
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.FieldType(p)))
		}
		g.code.Sconst(")")
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
		g.code.Op(cf.ARETURN)
		g.check()
	}
	if !declared("hashCode", 0) {
		// result = h(p1); result = result * 31 + h(p2); ...
		newMethod("hashCode", "()I")
		result := g.code.AllocLocal("I")
		for i, p := range props {
			if i > 0 {
				g.code.Local(cf.ILOAD, result)
				g.code.Iconst(31)
				g.code.Op(cf.IMUL)
			}
			g.hashOf(func() { field(0, p) }, types.FieldType(p))
			if i > 0 {
				g.code.Op(cf.IADD)
			}
			g.code.Local(cf.ISTORE, result)
		}
		g.code.Local(cf.ILOAD, result)
		g.code.Op(cf.IRETURN)
		g.check()
	}
	if !declared("equals", 1) {
		newMethod("equals", "(Ljava/lang/Object;)Z")
		g.code.AllocLocal("Ljava/lang/Object;")
		other := g.code.AllocLocal(self)
		notSame, isInstance := cf.NewLabel(), cf.NewLabel()
		returnBool := func(b int32) {
			g.code.Iconst(b)
			g.code.Op(cf.IRETURN)
		}
		g.code.Local(cf.ALOAD, 0)
		g.code.Local(cf.ALOAD, 1)
		g.code.Jump(cf.IF_ACMPNE, notSame)
		returnBool(1)
		g.code.Place(notSame)
		g.code.Local(cf.ALOAD, 1)
		g.code.InstanceOf(cls.Name)
		g.code.Jump(cf.IFNE, isInstance)
		returnBool(0)
		g.code.Place(isInstance)
		g.code.Local(cf.ALOAD, 1)
		g.code.Checkcast(cls.Name)
		g.code.Local(cf.ASTORE, other)
		for _, p := range props {
			next := cf.NewLabel()
			field(0, p)
			field(other, p)
			g.jumpIfEqual(types.FieldType(p), next)
			returnBool(0)
			g.code.Place(next)
		}
		returnBool(1)
		g.check()
	}
}

// jumpIfEqual compares the two values of type t on the stack, as data
// classes do, and jumps to label if they are equal: floating-point values
// with compare, references with equals.
func (g *gen) jumpIfEqual(t types.Type, label *cf.Label) {
	t = types.Repr(t) // a value class's underlying values
	switch {
	case t.IsIntLike():
		g.code.Jump(cf.IF_ICMPEQ, label)
	case t.IsPrimitive() && t.K == types.Float:
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/Float", "compare", "(FF)I")
		g.code.Jump(cf.IFEQ, label)
	case t.IsPrimitive() && t.K == types.Long:
		g.code.Op(cf.LCMP)
		g.code.Jump(cf.IFEQ, label)
	case t.IsPrimitive() && t.K == types.Double:
		g.code.Invoke(cf.INVOKESTATIC, "java/lang/Double", "compare", "(DD)I")
		g.code.Jump(cf.IFEQ, label)
	default:
		g.code.Invoke(cf.INVOKESTATIC, "java/util/Objects", "equals", "(Ljava/lang/Object;Ljava/lang/Object;)Z")
		g.code.Jump(cf.IFNE, label)
	}
}

// hashOf pushes the hash code of the value that load pushes, of type t:
// with the box class's static hashCode for primitives, and 0 for null.
func (g *gen) hashOf(load func(), t types.Type) {
	t = types.Repr(t) // a value class's underlying value's
	if t.IsArray() {
		load()
		g.code.Invoke(cf.INVOKESTATIC, "java/util/Arrays", "hashCode", "("+arraysParam(t)+")I")
		return
	}
	if t.IsPrimitive() {
		load()
		g.code.Invoke(cf.INVOKESTATIC, boxClass[t.K], "hashCode", "("+desc(t)+")I")
		return
	}
	owner := "java/lang/Object"
	if d := desc(t); strings.HasPrefix(d, "L") {
		owner = d[1 : len(d)-1]
	}
	call := func() {
		load()
		if owner == "java/lang/Object" || t.K == types.Class && t.C.Interface {
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/Object", "hashCode", "()I")
		} else {
			g.code.Invoke(cf.INVOKEVIRTUAL, owner, "hashCode", "()I")
		}
	}
	if !types.MayBeNull(t) {
		call()
		return
	}
	isNull, end := cf.NewLabel(), cf.NewLabel()
	load()
	g.code.Jump(cf.IFNULL, isNull)
	call()
	g.code.Jump(cf.GOTO, end)
	g.code.Place(isNull)
	g.code.Iconst(0)
	g.code.Place(end)
}

// arraysParam returns the parameter type of the java.util.Arrays methods
// that take an array of type t: the array itself for a primitive array,
// Object[] for the others.
func arraysParam(t types.Type) string {
	if d := desc(t); d[1] != 'L' && d[1] != '[' {
		return d
	}
	return "[Ljava/lang/Object;"
}

func componentName(i int) string { return "component" + itoa(i+1) }

// synthesized returns the method name of class cls that the compiler
// synthesizes, or nil if the class declares the only ones.
func synthesized(cls *types.ClassSym, name string) *types.FunSym {
	for _, f := range cls.Methods[name] {
		if f.Decl == nil {
			return f
		}
	}
	return nil
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// javaHash returns a string's hash code as Java computes it.
func javaHash(s string) int32 {
	var h int32
	for _, r := range []rune(s) {
		if r >= 0x10000 {
			// A UTF-16 surrogate pair.
			r -= 0x10000
			h = 31*h + int32(0xD800+(r>>10))
			h = 31*h + int32(0xDC00+(r&0x3FF))
			continue
		}
		h = 31*h + int32(r)
	}
	return h
}
