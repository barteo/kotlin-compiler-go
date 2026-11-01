package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Annotation instances.
//
// An instantiation of an annotation class, A(x), creates an instance of a
// class that implements the annotation interface, as kotlinc writes it:
// Outer$annotationImpl$A$0, one for each annotation class a file
// instantiates, named after the class or facade of its first
// instantiation. It keeps the values in fields, a KClass as a Class, and
// implements equals, hashCode and toString as java.lang.annotation.
// Annotation specifies them, and annotationType.

// annotationNew creates an instance of annotation class x.Ctor.
func (g *gen) annotationNew(x *syntax.Call) {
	impl := g.annotationImpl(x.Ctor, x.P.Line)
	d := methodDesc(x.Sym.Params, types.UnitType)
	g.code.New(impl)
	g.code.Op(cf.DUP)
	if mask := g.callArgs(x, x.Sym); mask.any() {
		g.pushMask(mask, len(x.Sym.Params))
		d = defaultsDesc(d, len(x.Sym.Params), ctorMarker)
	}
	g.code.Invoke(cf.INVOKESPECIAL, impl, "<init>", d)
	g.code.Checkcast(x.Ctor.Name)
}

// annotationImpl returns the class that implements annotation class ann,
// which it writes the first time, with the line of that instantiation.
func (g *gen) annotationImpl(ann *types.ClassSym, line int) string {
	if name, ok := g.shared.annImpls[ann]; ok {
		return name
	}
	outer := g.cls.Name
	if g.owner != nil {
		outer = g.owner.Name
	}
	name := outer + "$annotationImpl$" + ann.Name[strings.LastIndexAny(ann.Name, "/$")+1:] + "$0"
	if g.shared.annImpls == nil {
		g.shared.annImpls = map[*types.ClassSym]string{}
	}
	g.shared.annImpls[ann] = name
	ig := &gen{file: g.file, shared: g.shared, noLines: 1}
	ig.annotationClassImpl(name, outer, ann, line)
	return name
}

// annotationClassImpl writes class name, which implements annotation
// class ann.
func (g *gen) annotationClassImpl(name, outer string, ann *types.ClassSym, line int) {
	params := ann.AnnotationParams
	var defaults []*syntax.CtorParam // the source's, with the default values
	if cd, ok := ann.Decl.(*syntax.ClassDecl); ok {
		defaults = cd.CtorParams
	}
	c := cf.NewClass(cf.AccPublic|cf.AccFinal|cf.AccSuper|cf.AccSynthetic, name, "java/lang/Object")
	c.Interfaces = []string{ann.Name}
	c.SourceFile = g.file.Path[strings.LastIndexByte(g.file.Path, '/')+1:]
	c.Line = line
	g.cls = c
	self := "L" + name + ";"
	jvm := func(p *types.VarSym) string { return annotationJVM(p.Type) }
	for _, p := range params {
		c.AddField(cf.AccPrivate|cf.AccFinal|cf.AccSynthetic, p.Name, jvm(p))
	}

	// The constructor stores the values, a KClass's Class.
	// The parameters' slots: the symbols are the annotation class's, which
	// other files' code generation shares.
	slots := make([]int, len(params))
	d := methodDesc(params, types.UnitType)
	g.code = c.NewMethod(cf.AccPublic, "<init>", d)
	g.code.AllocLocal(self)
	for i, p := range params {
		slots[i] = g.code.AllocLocal(desc(p.Type))
	}
	g.code.Local(cf.ALOAD, 0)
	g.code.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	for i, p := range params {
		g.code.Local(cf.ALOAD, 0)
		g.code.Local(loadOp(p.Type), slots[i])
		g.javaClasses(p.Type)
		g.code.FieldInsn(cf.PUTFIELD, name, p.Name, jvm(p))
	}
	g.code.Op(cf.RETURN)
	g.check()

	// The constructor for omitted arguments computes their default
	// values.
	if hasDefault(params) {
		g.code = c.NewMethod(cf.AccPublic|cf.AccSynthetic, "<init>", defaultsDesc(d, len(params), ctorMarker))
		g.code.AllocLocal(self)
		for i, p := range params {
			slots[i] = g.code.AllocLocal(desc(p.Type))
		}
		masks := g.allocMasks(len(params), ctorMarker)
		for j, p := range params {
			if !p.Default || j >= len(defaults) || defaults[j].Default == nil {
				continue
			}
			skip := cf.NewLabel()
			g.jumpIfPassed(masks, j, skip)
			g.exprAs(defaults[j].Default, p.Type)
			g.code.Local(storeOp(p.Type), slots[j])
			g.code.Place(skip)
		}
		g.code.Local(cf.ALOAD, 0)
		for i, p := range params {
			g.code.Local(loadOp(p.Type), slots[i])
		}
		g.code.Invoke(cf.INVOKESPECIAL, name, "<init>", d)
		g.code.Op(cf.RETURN)
		g.check()
	}

	field := func(p *types.VarSym) {
		g.code.Local(cf.ALOAD, 0)
		g.code.FieldInsn(cf.GETFIELD, name, p.Name, jvm(p))
	}
	for _, p := range params {
		g.code = c.NewMethod(cf.AccPublic|cf.AccFinal|cf.AccSynthetic, p.Name, "()"+jvm(p))
		g.code.AllocLocal(self)
		field(p)
		g.code.Op(returnOp(p.Type))
		g.check()
	}

	// equals compares the values of the annotation's methods, on both
	// sides, so that an instance equals the JVM's of the same annotation.
	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "equals", "(Ljava/lang/Object;)Z")
	g.code.AllocLocal(self)
	g.code.AllocLocal("Ljava/lang/Object;")
	other := g.code.AllocLocal("L" + ann.Name + ";")
	isAnn := cf.NewLabel()
	g.code.Local(cf.ALOAD, 1)
	g.code.InstanceOf(ann.Name)
	g.code.Jump(cf.IFNE, isAnn)
	g.code.Iconst(0)
	g.code.Op(cf.IRETURN)
	g.code.Place(isAnn)
	g.code.Local(cf.ALOAD, 1)
	g.code.Checkcast(ann.Name)
	g.code.Local(cf.ASTORE, other)
	for _, p := range params {
		next := cf.NewLabel()
		g.code.Local(cf.ALOAD, 0)
		g.code.Checkcast(ann.Name)
		g.code.InvokeInterface(ann.Name, p.Name, "()"+jvm(p))
		g.code.Local(cf.ALOAD, other)
		g.code.InvokeInterface(ann.Name, p.Name, "()"+jvm(p))
		if t := types.Repr(p.Type); t.IsArray() {
			g.code.Invoke(cf.INVOKESTATIC, "java/util/Arrays", "equals", "("+arraysParam(t)+arraysParam(t)+")Z")
			g.code.Jump(cf.IFNE, next)
		} else {
			g.jumpIfEqual(p.Type, next)
		}
		g.code.Iconst(0)
		g.code.Op(cf.IRETURN)
		g.code.Place(next)
	}
	g.code.Iconst(1)
	g.code.Op(cf.IRETURN)
	g.check()

	// hashCode sums (127 * name.hashCode()) ^ the value's hash code.
	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "hashCode", "()I")
	g.code.AllocLocal(self)
	g.code.Iconst(0)
	for _, p := range params {
		g.code.Iconst(127 * javaHash(p.Name))
		g.hashOf(func() { field(p) }, p.Type)
		g.code.Op(cf.IXOR)
		g.code.Op(cf.IADD)
	}
	g.code.Op(cf.IRETURN)
	g.check()

	// toString: @a.b.A(x=1, y=[2, 3])
	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "toString", "()Ljava/lang/String;")
	g.code.AllocLocal(self)
	g.newStringBuilder()
	appendString := func() {
		g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.StringType))
	}
	qualified := strings.NewReplacer("/", ".", "$", ".").Replace(ann.Name)
	if ann.KotlinName != "" {
		qualified = strings.NewReplacer("/", ".").Replace(ann.KotlinName)
	}
	for i, p := range params {
		sep := ", "
		if i == 0 {
			sep = "@" + qualified + "("
		}
		g.code.Sconst(sep + p.Name + "=")
		appendString()
		field(p)
		switch t := p.Type; {
		case types.Unboxed(t):
			g.boxValue(t) // its toString
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(types.AnyType))
		case t.IsArray():
			g.code.Invoke(cf.INVOKESTATIC, "java/util/Arrays", "toString", "("+arraysParam(t)+")Ljava/lang/String;")
			appendString()
		default:
			g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "append", appendDesc(t))
		}
	}
	if len(params) == 0 {
		g.code.Sconst("@" + qualified + "(")
		appendString()
	}
	g.code.Sconst(")")
	appendString()
	g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/StringBuilder", "toString", "()Ljava/lang/String;")
	g.code.Op(cf.ARETURN)
	g.check()

	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal|cf.AccSynthetic, "annotationType", "()Ljava/lang/Class;")
	g.code.AllocLocal(self)
	g.code.LdcClass(ann.Name)
	g.code.Op(cf.ARETURN)
	g.check()

	e := cf.InnerClassEntry{Inner: name, Access: cf.AccPublic | cf.AccStatic | cf.AccFinal}
	c.InnerClasses = append(c.InnerClasses, e)
	c.Enclosing = &cf.EnclosingMethod{Class: outer}
	g.annotateMeta(c, kmeta.WriteSynthetic())
	g.shared.classes[name] = c
	g.shared.later = append(g.shared.later, name)
}

// javaClasses converts a value of an annotation's parameter type t on
// the stack to the type the annotation's method returns: a KClass to its
// Class, and an array of KClasses to an array of Classes.
func (g *gen) javaClasses(t types.Type) {
	switch d := desc(t); d {
	case "Lkotlin/reflect/KClass;":
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/JvmClassMappingKt", "getJavaClass", "(Lkotlin/reflect/KClass;)Ljava/lang/Class;")
	case "[Lkotlin/reflect/KClass;":
		// for (i in src.indices) dst[i] = src[i].java
		src := g.code.AllocLocal(d)
		dst := g.code.AllocLocal("[Ljava/lang/Class;")
		i := g.code.AllocLocal("I")
		g.code.Local(cf.ASTORE, src)
		g.code.Local(cf.ALOAD, src)
		g.code.Op(cf.ARRAYLENGTH)
		g.code.ANewArray("java/lang/Class")
		g.code.Local(cf.ASTORE, dst)
		g.code.Iconst(0)
		g.code.Local(cf.ISTORE, i)
		loop, end := cf.NewLabel(), cf.NewLabel()
		g.code.Place(loop)
		g.code.Local(cf.ILOAD, i)
		g.code.Local(cf.ALOAD, src)
		g.code.Op(cf.ARRAYLENGTH)
		g.code.Jump(cf.IF_ICMPGE, end)
		g.code.Local(cf.ALOAD, dst)
		g.code.Local(cf.ILOAD, i)
		g.code.Local(cf.ALOAD, src)
		g.code.Local(cf.ILOAD, i)
		g.code.Op(cf.AALOAD)
		g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/JvmClassMappingKt", "getJavaClass", "(Lkotlin/reflect/KClass;)Ljava/lang/Class;")
		g.code.Op(cf.AASTORE)
		g.code.Iinc(i, 1)
		g.code.Jump(cf.GOTO, loop)
		g.code.Place(end)
		g.code.Local(cf.ALOAD, dst)
	}
}

// hasDefault reports whether one of params declares a default value.
func hasDefault(params []*types.VarSym) bool {
	for _, p := range params {
		if p.Default {
			return true
		}
	}
	return false
}
