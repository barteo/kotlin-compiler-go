package codegen

import (
	"strconv"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Delegated properties (see check/delegated.go) are generated from what
// the parser and the checker make of them: a field x$delegate, accessors
// that call its getValue and setValue, and the KProperty objects these
// take, which kotlinc keeps in a static array, $$delegatedProperties.

const kpropsDesc = "[Lkotlin/reflect/KProperty;"

// propRef pushes a delegated property's KProperty, or null if its
// delegate doesn't use it.
func (g *gen) propRef(x *syntax.PropRef) {
	if x.Index < 0 {
		g.code.Op(cf.ACONST_NULL)
		return
	}
	g.code.FieldInsn(cf.GETSTATIC, x.Owner, "$$delegatedProperties", kpropsDesc)
	g.code.Iconst(int32(x.Index))
	g.code.Op(cf.AALOAD)
}

// kprops declares the class's $$delegatedProperties, and creates it in
// the <clinit> being generated, as kotlinc does: a class's properties'
// KProperty objects are property references with a receiver, the
// others without one, and those of extension properties take one more.
func (g *gen) kprops(refs []*syntax.PropRef) {
	if len(refs) == 0 {
		return
	}
	acc := uint16(cf.AccStatic | cf.AccFinal | cf.AccSynthetic)
	if g.inInterface() {
		acc |= cf.AccPublic // an interface's fields are public
	}
	g.cls.AddField(acc, "$$delegatedProperties", kpropsDesc)
	c := g.code
	c.Iconst(int32(len(refs)))
	c.ANewArray("kotlin/reflect/KProperty")
	for i, r := range refs {
		impl := "kotlin/jvm/internal/PropertyReference"
		if r.Mutable {
			impl = "kotlin/jvm/internal/MutablePropertyReference"
		}
		flags := int32(1) // top level or local: no receiver
		recvs := 0
		if r.Recv {
			recvs++
			flags = 0
		}
		if r.Ext {
			recvs++
		}
		impl += strconv.Itoa(recvs) + "Impl"
		c.Op(cf.DUP)
		c.Iconst(int32(i))
		c.New(impl)
		c.Op(cf.DUP)
		c.LdcClass(r.Owner)
		c.Sconst(r.Name)
		c.Sconst(r.Sig)
		c.Iconst(flags)
		c.Invoke(cf.INVOKESPECIAL, impl, "<init>", "(Ljava/lang/Class;Ljava/lang/String;Ljava/lang/String;I)V")
		c.Op(cf.AASTORE)
	}
	c.FieldInsn(cf.PUTSTATIC, refs[0].Owner, "$$delegatedProperties", kpropsDesc)
}

// isDelegateField reports whether a property is a delegated property's
// field, x$delegate, which has no accessors.
func isDelegateField(decl any) bool {
	pd, ok := decl.(*syntax.PropDecl)
	return ok && pd.DelegateOf != nil
}

// kpropSuper returns the superclass of the class of a property
// reference's KProperty (see lambdaClass): kotlin.jvm.internal's
// PropertyReference1Impl for C::p, PropertyReference0Impl for x::p and
// ::p, and their Mutable versions for a var's.
func kpropSuper(x *syntax.Lambda) string {
	impl := "kotlin/jvm/internal/PropertyReference"
	if x.KProp.Set != nil {
		impl = "kotlin/jvm/internal/MutablePropertyReference"
	}
	return impl + strconv.Itoa(len(x.ParamSyms())) + "Impl"
}

// kpropInit calls the constructor of a property reference's superclass,
// with this pushed: it takes the receiver of x::p, in slot bound, then
// the class that declares the property, or its file's facade, its name
// and its getter's JVM name and descriptor, and flags, 1 for a top-level
// property.
func (g *gen) kpropInit(c *cf.Code, x *syntax.Lambda, super string, bound int) {
	k := x.KProp
	var owner, sig string
	flags := int32(1)
	switch {
	case k.Getter != nil && k.Getter.Owner != nil:
		owner, sig, flags = refOwner(x, k.Getter.Owner), k.Getter.JVM()+funDesc(k.Getter), 0
	case k.Getter != nil:
		owner, sig = k.Getter.Facade, k.Getter.JVM()+funDesc(k.Getter)
	case k.Prop.Kind == types.Global:
		owner, sig = k.Prop.Facade, types.GetterJVM(k.Prop)+"()"+desc(types.GetterType(k.Prop))
	default:
		owner, sig, flags = refOwner(x, k.Prop.Owner), types.GetterJVM(k.Prop)+"()"+desc(types.GetterType(k.Prop)), 0
	}
	d := "(Ljava/lang/Class;Ljava/lang/String;Ljava/lang/String;I)V"
	if k.Bound {
		g.loadBound(c, x, bound)
		d = "(Ljava/lang/Object;" + d[1:]
	}
	c.LdcClass(owner)
	c.Sconst(x.Ref.Name)
	c.Sconst(sig)
	c.Iconst(flags)
	c.Invoke(cf.INVOKESPECIAL, super, "<init>", d)
}

// loadBound loads the receiver of reference x::p, in slot bound, boxed if
// it is a primitive or a value class's.
func (g *gen) loadBound(c *cf.Code, x *syntax.Lambda, bound int) {
	v := x.Ref.Bound
	c.Local(loadOpOf(storageDesc(v)), bound)
	code := g.code
	g.code = c
	g.boxReceiver(v.Type)
	g.code = code
}

// kfunInit calls the constructor of a function reference's superclass,
// FunctionReferenceImpl, with this pushed: see kfunArgs. The receiver of
// x::f is in slot bound.
func (g *gen) kfunInit(c *cf.Code, x *syntax.Lambda, super string, bound int) {
	var load func()
	if bound >= 0 {
		load = func() { g.loadBound(c, x, bound) }
	}
	g.kfunArgs(c, x, super, load)
}

// kfunArgs calls the constructor of FunctionReferenceImpl, or super, its
// subclass, for reference x: it takes the function's arity, the receiver
// of x::f, which load pushes, then the class that declares the function,
// or its file's facade, its name and its method's JVM name and
// descriptor, and flags, 1 for a top-level function. A constructor's name
// is <init>.
func (g *gen) kfunArgs(c *cf.Code, x *syntax.Lambda, super string, load func()) {
	f := x.KFun.Sym
	if x.KFun.SAM {
		// ::KRunnable: FunInterfaceConstructorReference takes the
		// interface, by which the references to it are equal.
		c.LdcClass(f.Owner.Name)
		c.Invoke(cf.INVOKESPECIAL, super, "<init>", "(Ljava/lang/Class;)V")
		return
	}
	owner, sig, flags := g.kfunOf(f)
	if f.Owner != nil && localOf(f) == nil {
		owner = refOwner(x, f.Owner)
	}
	name := x.Ref.Name
	if x.KFun.Ctor != nil {
		owner, name, sig, flags = x.KFun.Ctor.Name, "<init>", "<init>"+ctorJVMDesc(f), 0
	}
	d := "(ILjava/lang/Class;Ljava/lang/String;Ljava/lang/String;I)V"
	arity := x.T.A.Len() - 1
	if x.SAM != nil {
		arity = len(x.SAM.Params) // a fun interface's: see funAdapter
	}
	if x.Suspend {
		arity++ // the continuation
	}
	c.Iconst(int32(arity))
	if load != nil {
		load()
		d = "(ILjava/lang/Object;" + d[2:]
	}
	c.LdcClass(owner)
	c.Sconst(name)
	c.Sconst(sig)
	c.Iconst(flags | x.KFunFlags)
	c.Invoke(cf.INVOKESPECIAL, super, "<init>", d)
}

// kfunSuper returns the superclass of reference x's FunctionReference:
// FunctionReferenceImpl, AdaptedFunctionReference if it is adapted, or
// FunInterfaceConstructorReference for a fun interface's constructor,
// ::KRunnable, as kotlinc's.
func kfunSuper(x *syntax.Lambda) string {
	switch {
	case x.KFun.SAM:
		return "kotlin/jvm/internal/FunInterfaceConstructorReference"
	case x.KFunAdapted:
		return "kotlin/jvm/internal/AdaptedFunctionReference"
	}
	return "kotlin/jvm/internal/FunctionReferenceImpl"
}

// refOwner returns the class that reference x names as the owner of its
// member of class declaring, as kotlinc does: the class of the receiver
// it is made on, whose member it is, maybe as a fake override with the
// supertype's type arguments, rather than the supertype that declares it.
func refOwner(x *syntax.Lambda, declaring *types.ClassSym) string {
	var t types.Type
	switch ps := x.ParamSyms(); {
	case x.Ref.Bound != nil:
		t = x.Ref.Bound.Type
	case x.Ref.X != nil && len(ps) > 0:
		t = ps[0].Type // A::f's receiver
	}
	if t = t.NonNull(); t.K == types.Class && t.C != declaring && t.C.IsSubclassOf(declaring) {
		return t.C.Name
	}
	return declaring.Name
}

// kfunOf returns what a FunctionReferenceImpl of f takes: the class that
// declares f, or its file's facade, its method's JVM name and
// descriptor, and flags, 1 for a top-level function.
func (g *gen) kfunOf(f *types.FunSym) (owner, sig string, flags int32) {
	switch lf := localOf(f); {
	case lf != nil:
		// A local function, which has no owner to kotlin.reflect: its
		// method's name, as kotlinc's.
		return "kotlin/jvm/internal/Intrinsics$Kotlin", lf.Method + g.localDesc(lf), 0
	case f.Owner != nil:
		return f.Owner.Name, f.JVM() + funDesc(f), 0
	}
	return f.Facade, f.JVM() + funDesc(f), 1
}
