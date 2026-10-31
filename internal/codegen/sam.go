package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// SAM wrappers.
//
// A function value passed for a Kotlin fun interface, as handler in
// collect(handler) for a FlowCollector, is wrapped in an instance of a
// class that implements the interface by invoking the function, as kotlinc
// writes it: Facade$sam$pkg_Iface$0, one for each interface a file
// converts to, named after the file's facade. It also implements
// FunctionAdapter, by which two wrappers of the same function are equal.

// samWrap converts function value x.X to fun interface x.T.
func (g *gen) samWrap(x *syntax.Checked) {
	iface := x.T.NonNull().C
	fn := desc(x.X.Type())
	name := g.samWrapper(iface, x.SAM, fn)
	if types.MayBeNull(x.X.Type()) {
		// A null function value is a null interface.
		tmp := g.code.AllocLocal(fn)
		g.expr(x.X)
		g.code.Local(cf.ASTORE, tmp)
		isNull, end := cf.NewLabel(), cf.NewLabel()
		g.code.Local(cf.ALOAD, tmp)
		g.code.Jump(cf.IFNULL, isNull)
		g.code.New(name)
		g.code.Op(cf.DUP)
		g.code.Local(cf.ALOAD, tmp)
		g.code.Invoke(cf.INVOKESPECIAL, name, "<init>", "("+fn+")V")
		g.code.Jump(cf.GOTO, end)
		g.code.Place(isNull)
		g.code.Op(cf.ACONST_NULL)
		g.code.Place(end)
		g.code.Checkcast(iface.Name)
		return
	}
	g.code.New(name)
	g.code.Op(cf.DUP)
	g.expr(x.X)
	g.code.Invoke(cf.INVOKESPECIAL, name, "<init>", "("+fn+")V")
	g.code.Checkcast(iface.Name)
}

// samWrapper returns the class that wraps a function value, whose class
// has descriptor fn, as interface iface, whose method is m. It writes the
// class the first time.
func (g *gen) samWrapper(iface *types.ClassSym, m *types.FunSym, fn string) string {
	key := samKey{iface, g.publicInline}
	if name, ok := g.shared.samImpls[key]; ok {
		return name
	}
	outer := g.file.Class()
	qualified := strings.NewReplacer("/", "_", "$", "_").Replace(iface.Name)
	if iface.KotlinName != "" {
		qualified = strings.NewReplacer("/", "_", ".", "_").Replace(iface.KotlinName)
	}
	name := outer + "$sam$" + qualified + "$0"
	if g.publicInline {
		// Public, for other modules' copies of the function: kotlinc's
		// Facade$sam$i$pkg_Iface$0.
		name = outer + "$sam$i$" + qualified + "$0"
	}
	if g.shared.samImpls == nil {
		g.shared.samImpls = map[samKey]string{}
	}
	g.shared.samImpls[key] = name
	sg := &gen{file: g.file, shared: g.shared, noLines: 1, publicInline: g.publicInline}
	sg.samWrapperClass(name, outer, iface, m, fn)
	return name
}

// samKey identifies a file's SAM wrapper class: there is one for each
// interface, and another for the bodies of inline functions.
type samKey struct {
	iface  *types.ClassSym
	inline bool
}

const adapter = "kotlin/jvm/internal/FunctionAdapter"

// descBoxes are the boxes of primitive descriptors, and the methods that
// unbox them.
var descBoxes = map[byte][2]string{
	'I': {"java/lang/Integer", "intValue"}, 'J': {"java/lang/Long", "longValue"},
	'Z': {"java/lang/Boolean", "booleanValue"}, 'C': {"java/lang/Character", "charValue"},
	'B': {"java/lang/Byte", "byteValue"}, 'S': {"java/lang/Short", "shortValue"},
	'F': {"java/lang/Float", "floatValue"}, 'D': {"java/lang/Double", "doubleValue"},
}

// samWrapperClass writes class name, which implements iface's method m
// by invoking a function of class descriptor fn.
func (g *gen) samWrapperClass(name, outer string, iface *types.ClassSym, m *types.FunSym, fn string) {
	access := uint16(0)
	if g.publicInline {
		access = cf.AccPublic
	}
	c := cf.NewClass(access|cf.AccFinal|cf.AccSuper|cf.AccSynthetic, name, "java/lang/Object")
	c.Interfaces = []string{iface.Name, adapter}
	c.SourceFile = g.file.Path[strings.LastIndexByte(g.file.Path, '/')+1:]
	g.cls = c
	self := "L" + name + ";"
	fnClass := fn[1 : len(fn)-1]
	c.AddField(cf.AccPrivate|cf.AccFinal|cf.AccSynthetic, "function", fn)
	field := func() {
		g.code.Local(cf.ALOAD, 0)
		g.code.FieldInsn(cf.GETFIELD, name, "function", fn)
	}

	g.code = c.NewMethod(access, "<init>", "("+fn+")V")
	g.code.AllocLocal(self)
	g.code.AllocLocal(fn)
	g.code.Local(cf.ALOAD, 1)
	g.code.Sconst("function")
	g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/Intrinsics", "checkNotNullParameter", "(Ljava/lang/Object;Ljava/lang/String;)V")
	g.code.Local(cf.ALOAD, 0)
	g.code.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	g.code.Local(cf.ALOAD, 0)
	g.code.Local(cf.ALOAD, 1)
	g.code.FieldInsn(cf.PUTFIELD, name, "function", fn)
	g.code.Op(cf.RETURN)
	g.check()

	// The interface's method passes its arguments, boxed, to invoke.
	d := samMethodDesc(m)
	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal|cf.AccSynthetic, m.JVM(), d)
	g.code.AllocLocal(self)
	ps := descParams(d)
	slots := make([]int, len(ps))
	for i, p := range ps {
		slots[i] = g.code.AllocLocal(p)
	}
	field()
	var kts []types.Type // the Kotlin types of the parameters
	if m.Ext {
		kts = append(kts, m.Recv)
	}
	for _, p := range m.Params {
		kts = append(kts, p.Type)
	}
	if len(kts) != len(ps) {
		kts = nil
	}
	for i, p := range ps {
		g.code.Local(loadOpOf(p), slots[i])
		if i < len(kts) && types.Unboxed(kts[i]) {
			g.boxValue(kts[i]) // a value class's value, which invoke takes boxed
		} else if b, ok := descBoxes[p[0]]; ok && len(p) == 1 {
			g.code.Invoke(cf.INVOKESTATIC, b[0], "valueOf", "("+p+")L"+b[0]+";")
		}
	}
	g.code.InvokeInterface(fnClass, "invoke", "("+strings.Repeat(objectDesc, len(ps))+")"+objectDesc)
	switch r := d[strings.IndexByte(d, ')')+1:]; {
	case types.Unboxed(types.JVMRet(m)) && r != "V":
		g.castBox(types.AnyType.Nullable(), types.JVMRet(m))
		g.unboxValue(types.JVMRet(m))
	case r == "V":
		g.code.Op(cf.POP)
	case len(r) == 1:
		b := descBoxes[r[0]]
		g.code.Checkcast(b[0])
		g.code.Invoke(cf.INVOKEVIRTUAL, b[0], b[1], "()"+r)
	case r != objectDesc:
		g.code.Checkcast(className(r))
	}
	g.code.Op(returnOpOf(d[strings.IndexByte(d, ')')+1:]))
	g.check()

	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "getFunctionDelegate", "()Lkotlin/Function;")
	g.code.AllocLocal(self)
	field()
	g.code.Checkcast("kotlin/Function")
	g.code.Op(cf.ARETURN)
	g.check()

	g.adapterEquality(c, iface.Name)

	c.InnerClasses = append(c.InnerClasses, cf.InnerClassEntry{Inner: name, Access: access | cf.AccStatic | cf.AccFinal})
	c.Enclosing = &cf.EnclosingMethod{Class: outer}
	h := kmeta.WriteSynthetic()
	h.XI = kmeta.XILambda
	g.annotateMeta(c, g.inlineXI(h, kmeta.XIInlineLambda))
	g.shared.classes[name] = c
	g.shared.later = append(g.shared.later, name)
}

// adapterEquality writes the equals and hashCode of class c, which
// implements interface iface and FunctionAdapter, as kotlinc's: two
// objects of the interface are equal if their functions are.
func (g *gen) adapterEquality(c *cf.Class, iface string) {
	self := "L" + c.Name + ";"
	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "equals", "(Ljava/lang/Object;)Z")
	g.code.AllocLocal(self)
	g.code.AllocLocal(objectDesc)
	no := cf.NewLabel()
	end := cf.NewLabel()
	g.code.Local(cf.ALOAD, 1)
	g.code.InstanceOf(iface)
	g.code.Jump(cf.IFEQ, no)
	g.code.Local(cf.ALOAD, 1)
	g.code.InstanceOf(adapter)
	g.code.Jump(cf.IFEQ, no)
	g.code.Local(cf.ALOAD, 0)
	g.code.Checkcast(adapter)
	g.code.InvokeInterface(adapter, "getFunctionDelegate", "()Lkotlin/Function;")
	g.code.Local(cf.ALOAD, 1)
	g.code.Checkcast(adapter)
	g.code.InvokeInterface(adapter, "getFunctionDelegate", "()Lkotlin/Function;")
	g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/Intrinsics", "areEqual", "(Ljava/lang/Object;Ljava/lang/Object;)Z")
	g.code.Jump(cf.GOTO, end)
	g.code.Place(no)
	g.code.Iconst(0)
	g.code.Place(end)
	g.code.Op(cf.IRETURN)
	g.check()

	g.code = c.NewMethod(cf.AccPublic|cf.AccFinal, "hashCode", "()I")
	g.code.AllocLocal(self)
	g.code.Local(cf.ALOAD, 0)
	g.code.Checkcast(adapter)
	g.code.InvokeInterface(adapter, "getFunctionDelegate", "()Lkotlin/Function;")
	g.code.Invoke(cf.INVOKEVIRTUAL, "java/lang/Object", "hashCode", "()I")
	g.code.Op(cf.IRETURN)
	g.check()
}
