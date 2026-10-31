package codegen

import (
	"maps"
	"slices"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The methods that the JVM annotations ask for, for Java callers.

// overload is a method that @JvmOverloads asks for: the function without
// some of its parameters that have default values.
type overload struct {
	desc    string // its descriptor
	omitted int    // the mask of the parameters it omits
}

// overloadsOf returns the overloads of a function annotated @JvmOverloads,
// as kotlinc writes them: one for each parameter with a default value,
// without it and the ones after it that have default values.
func overloadsOf(fd *syntax.FunDecl) []overload {
	if fd.Find("", "JvmOverloads") == nil {
		return nil
	}
	var defaults []int
	for i, p := range fd.Params {
		if p.Default != nil {
			defaults = append(defaults, i)
		}
	}
	f := fd.Sym
	var out []overload
	for n := len(defaults) - 1; n >= 0; n-- {
		mask := 0
		for _, i := range defaults[n:] {
			mask |= 1 << i
		}
		var sb strings.Builder
		sb.WriteByte('(')
		if f.Ext {
			sb.WriteString(desc(f.Recv))
		}
		for i, p := range f.Params {
			if mask&(1<<i) == 0 {
				sb.WriteString(desc(p.Type))
			}
		}
		sb.WriteByte(')')
		r, _ := types.RetDesc(f.Ret)
		sb.WriteString(r)
		d := sb.String()
		if f.Suspend {
			d = types.SuspendDesc(d)
		}
		out = append(out, overload{d, mask})
	}
	return out
}

// ctorOverloads generates the overloads of constructor ctor, declared
// with modifiers m and parameters params, if it is annotated
// @JvmOverloads: each calls the constructor for omitted arguments.
func (g *gen) ctorOverloads(m *syntax.Modifiers, ctor *types.FunSym, params []*syntax.Param) {
	if m.Find("", "JvmOverloads") == nil {
		return
	}
	full := methodDesc(ctor.Params, types.UnitType)
	i := strings.IndexByte(full, ')')
	d := full[:i] + "ILkotlin/jvm/internal/DefaultConstructorMarker;" + full[i:]
	var defaults []int
	for i, p := range params {
		if p.Default != nil {
			defaults = append(defaults, i)
		}
	}
	for n := len(defaults) - 1; n >= 0; n-- {
		mask := 0
		for _, i := range defaults[n:] {
			mask |= 1 << i
		}
		var sb strings.Builder
		sb.WriteByte('(')
		access := uint16(cf.AccPublic)
		for i, p := range ctor.Params {
			if mask&(1<<i) == 0 {
				sb.WriteString(desc(p.Type))
			}
		}
		if hasVararg(taken(ctor.Params, mask)) {
			access |= cf.AccVarargs
		}
		sb.WriteString(")V")
		g.code = g.cls.NewMethod(access, "<init>", sb.String())
		if len(params) == len(ctor.Params) {
			var ps []sigPart
			for i, p := range ctor.Params {
				if mask&(1<<i) == 0 {
					var ref *syntax.TypeRef
					if !params[i].Vararg {
						ref = params[i].Type
					}
					ps = append(ps, typedPart(p.Type, ref))
				}
			}
			if s := methodSig(nil, ps, sigPart{t: types.UnitType}, sb.String(), 0, g.wildcards(m.Annotations, ctor.Owner)); s != "" {
				g.cls.LastMethod().SetSignature(s)
			}
		}
		g.code.Local(cf.ALOAD, g.code.AllocLocal("L"+ctor.Owner.Name+";"))
		for i, p := range ctor.Params {
			pd := desc(p.Type)
			if mask&(1<<i) != 0 {
				g.zero(pd)
				continue
			}
			slot := g.code.AllocLocal(pd)
			g.code.NameLocal(slot, p.Name)
			g.checkParam(slot, p.Type, p.Name)
			g.code.Local(loadOpOf(pd), slot)
		}
		g.code.Iconst(int32(mask))
		g.code.Op(cf.ACONST_NULL)
		g.code.Invoke(cf.INVOKESPECIAL, ctor.Owner.Name, "<init>", d)
		g.code.Op(cf.RETURN)
		g.check()
	}
}

// emptyConstructor generates the constructor without parameters that
// kotlinc adds to a class whose primary constructor gives all of its
// parameters default values, for Java and reflection: it calls the
// constructor with all of them omitted. Without @JvmOverloads, which
// makes one too.
func (g *gen) emptyConstructor(cd *syntax.ClassDecl, params []*syntax.Param) {
	ctor := cd.Ctor
	cls := cd.Sym
	if len(params) == 0 || cd.CtorMods.Find("", "JvmOverloads") != nil || cls.Inner || cls.Local || cls.Anonymous ||
		cls.Value || cls.Entries != nil || cls.Object || ctor.Private || cd.Visibility == syntax.Private || cls.Sealed {
		return
	}
	for _, p := range params {
		if p.Default == nil {
			return
		}
	}
	for _, sc := range cd.Secondaries() {
		if len(sc.Params) == 0 {
			return
		}
	}
	access := uint16(cf.AccPublic)
	if cd.CtorMods.Visibility == syntax.Protected {
		access = cf.AccProtected
	}
	full := methodDesc(ctor.Params, types.UnitType)
	i := strings.IndexByte(full, ')')
	g.code = g.cls.NewMethod(access, "<init>", "()V")
	g.code.Line(cd.P.Line)
	g.code.Local(cf.ALOAD, g.code.AllocLocal("L"+cls.Name+";"))
	for _, p := range ctor.Params {
		g.zero(desc(p.Type))
	}
	g.code.Iconst(int32(1<<len(params) - 1))
	g.code.Op(cf.ACONST_NULL)
	g.code.Invoke(cf.INVOKESPECIAL, cls.Name, "<init>", full[:i]+"ILkotlin/jvm/internal/DefaultConstructorMarker;"+full[i:])
	g.code.Op(cf.RETURN)
	g.check()
}

// overloads generates the overloads of a function annotated @JvmOverloads,
// a method of owner or, if it is nil, a top-level function. Each calls
// name$default with the parameters it omits.
func (g *gen) overloads(fd *syntax.FunDecl, owner *types.ClassSym, access uint16) {
	f := fd.Sym
	access |= synthetic(&fd.Modifiers)
	full := funDesc(f)
	i := strings.IndexByte(full, ')')
	d := full[:i] + "ILjava/lang/Object;" + full[i:]
	if owner != nil {
		d = "(L" + owner.Name + ";" + d[1:]
	}
	for _, o := range overloadsOf(fd) {
		acc := access &^ cf.AccVarargs
		if hasVararg(taken(f.Params, o.omitted)) {
			acc |= cf.AccVarargs
		}
		if owner == nil || !owner.Interface {
			acc |= cf.AccFinal // as kotlinc writes them, also of open functions
		}
		g.code = g.cls.NewMethod(acc, f.JVM(), o.desc)
		g.signParams(g.cls.LastMethod(), f, fd, o.desc, types.InvalidType, taken(f.Params, o.omitted))
		g.cls.Throws(f.Throws...)
		stubAnnotations(g.cls.LastMethod(), fd, f.Ext, taken(f.Params, o.omitted))
		g.code.Line(fd.P.Line)
		if owner != nil {
			g.code.Local(cf.ALOAD, g.code.AllocLocal("L"+owner.Name+";"))
		}
		if f.Ext {
			slot := g.code.AllocLocal(desc(f.Recv))
			g.checkParam(slot, f.Recv, "<this>")
			g.code.Local(loadOp(f.Recv), slot)
		}
		for i, p := range f.Params {
			pd := desc(p.Type)
			if o.omitted&(1<<i) != 0 {
				g.zero(pd)
				continue
			}
			slot := g.code.AllocLocal(pd)
			g.code.NameLocal(slot, p.Name)
			g.checkParam(slot, p.Type, p.Name)
			g.code.Local(loadOpOf(pd), slot)
		}
		if f.Suspend {
			k := g.code.AllocLocal(types.ContinuationDesc)
			g.code.NameLocal(k, "$completion")
			g.code.Local(cf.ALOAD, k)
		}
		g.code.Iconst(int32(o.omitted))
		g.code.Op(cf.ACONST_NULL)
		g.code.Invoke(cf.INVOKESTATIC, g.cls.Name, f.JVM()+"$default", d)
		g.code.Op(returnOpOf(o.desc[strings.IndexByte(o.desc, ')')+1:]))
		g.check()
	}
}

// taken returns the parameters that an overload with omitted parameters
// (a mask) takes.
func taken(params []*types.VarSym, omitted int) []*types.VarSym {
	var out []*types.VarSym
	for i, p := range params {
		if omitted&(1<<i) == 0 {
			out = append(out, p)
		}
	}
	return out
}

// staticStubs generates, in a class with a companion object, the static
// methods of the companion's functions and property accessors annotated
// @JvmStatic, as kotlinc writes them: each calls the function on the
// companion, and so does one for each of its @JvmOverloads overloads.
func (g *gen) staticStubs(cd *syntax.ClassDecl) {
	comp := cd.Sym.Companion
	if comp == nil {
		return
	}
	for _, m := range cd.Members {
		ccd, ok := m.(*syntax.ClassDecl)
		if !ok || ccd.Sym != comp {
			continue
		}
		for _, m := range ccd.Members {
			if pd, ok := m.(*syntax.PropDecl); ok && pd.Sym != nil && pd.Sym.JvmStatic && !pd.Sym.Private {
				g.propStubs(pd.Sym, comp)
				continue
			}
			fd, ok := m.(*syntax.FunDecl)
			if !ok || fd.Find("", "JvmStatic") == nil {
				continue
			}
			g.staticStub(fd, comp, funDesc(fd.Sym), varargs(fd.Sym), fd.Sym.Params)
			for _, o := range overloadsOf(fd) {
				params := taken(fd.Sym.Params, o.omitted)
				g.staticStub(fd, comp, o.desc, hasVararg(params), params)
			}
		}
	}
}

// propStubs generates the static accessors of companion comp's @JvmStatic
// property p, which call its accessors on the companion.
func (g *gen) propStubs(p *types.VarSym, comp *types.ClassSym) {
	t := types.GetterType(p)
	stub := func(name, d string, param types.Type) {
		g.code = g.cls.NewMethod(g.stubAccess(), name, d)
		if param.K == types.Unit {
			g.signAccessor(g.cls.LastMethod(), t, propRef(p), d, nil, comp)
		} else {
			g.signAccessor(g.cls.LastMethod(), param, propRef(p), d, nil, comp)
		}
		g.code.Line(declPos(p.Decl).Line)
		if param.K == types.Unit {
			nullability(methodAdder(g.cls.LastMethod()), t)
		}
		g.objectRef(comp)
		for _, pd := range descParams(d) {
			g.code.Local(loadOpOf(pd), g.code.AllocLocal(pd))
		}
		g.code.Invoke(cf.INVOKEVIRTUAL, comp.Name, name, d)
		g.code.Op(returnOpOf(d[strings.IndexByte(d, ')')+1:]))
		g.check()
	}
	stub(types.GetterJVM(p), "()"+desc(t), types.UnitType)
	if p.Mutable && !p.SetterPrivate {
		stub(types.SetterJVM(p), "("+desc(types.FieldType(p))+")V", types.FieldType(p))
	}
}

// implsClass generates the DefaultImpls class of interface cd, compiled
// without JVM default methods, as kotlinc writes it: the bodies of its
// members, fds, as static methods that take the instance first, with
// their $default methods and lambdas, and methods that call the bodies it
// inherits from other interfaces' DefaultImpls.
func (g *gen) implsClass(cd *syntax.ClassDecl, fds []*syntax.FunDecl) {
	cls, iface := cd.Sym, g.cls
	c := g.implsOf(cls, iface)
	annotations := c.NumMethods() > 0
	g.cls, g.implsThis = c, true
	for _, fd := range fds {
		access := uint16(cf.AccPublic | cf.AccStatic)
		if fd.Visibility == syntax.Private {
			access = cf.AccPrivate | cf.AccStatic
		}
		g.function(fd, access)
		if hasDefaults(fd.Params) {
			g.defaults(fd, cls)
		}
	}
	g.implsThis = false
	self := "L" + cls.Name + ";"
	for _, fw := range cd.Forwarders {
		f := fw.Fun
		fd := funDesc(f) // with a member extension's receiver
		g.code = c.NewMethod(cf.AccPublic|cf.AccStatic, f.JVM(), "("+self+fd[1:])
		g.code.Local(cf.ALOAD, g.code.AllocLocal(self))
		g.code.Checkcast(fw.Of.Name)
		for _, pd := range descParams(fd) {
			g.code.Local(loadOpOf(pd), g.code.AllocLocal(pd))
		}
		g.code.Invoke(cf.INVOKESTATIC, fw.Impls, f.JVM(), "(L"+fw.Of.Name+";"+fd[1:])
		g.code.Op(returnOpOf(fd[strings.IndexByte(fd, ')')+1:]))
		g.check()
	}
	g.cls = iface
	if len(fds) > 0 || len(cd.Forwarders) > 0 || annotations {
		g.shared.impls[cls] = c
	}
}

// implsOf returns the DefaultImpls class of interface cls, whose class is
// iface, begun by an earlier call or new.
func (g *gen) implsOf(cls *types.ClassSym, iface *cf.Class) *cf.Class {
	if c := g.shared.begun[cls]; c != nil {
		return c
	}
	c := cf.NewClass(cf.AccPublic|cf.AccFinal|cf.AccSuper, cls.Name+"$DefaultImpls", "java/lang/Object")
	c.SourceFile = iface.SourceFile
	g.shared.begun[cls] = c
	return c
}

// annotationsHolder returns the class that holds the $annotations methods
// of the properties of class cls, whose class is c: as kotlinc writes
// them, an interface's DefaultImpls, if it has one, else c.
func (g *gen) annotationsHolder(c *cf.Class, cls *types.ClassSym) *cf.Class {
	if cls.HasDefaultImpls() || cls.JVMCompat {
		return g.implsOf(cls, c)
	}
	return c
}

// stubAccess returns the access flags of a @JvmStatic stub in the class
// being generated: final, except in an interface, where the JVM allows no
// final methods.
func (g *gen) stubAccess() uint16 {
	if g.cls.Access&cf.AccInterface != 0 {
		return cf.AccPublic | cf.AccStatic
	}
	return cf.AccPublic | cf.AccStatic | cf.AccFinal
}

// staticStub generates the static method of descriptor d that calls the
// method of companion comp with the same name and descriptor.
func (g *gen) staticStub(fd *syntax.FunDecl, comp *types.ClassSym, d string, varargs bool, params []*types.VarSym) {
	access := g.stubAccess() | synthetic(&fd.Modifiers)
	if varargs {
		access |= cf.AccVarargs
	}
	name := fd.Sym.JVM()
	if fd.Sym.External && d == funDesc(fd.Sym) {
		// The native method, which the companion's calls: see external.
		g.cls.AddAbstractMethod(access|cf.AccNative, name, d)
	} else {
		g.code = g.cls.NewMethod(access, name, d)
	}
	if access&cf.AccSynthetic == 0 {
		g.signParams(g.cls.LastMethod(), fd.Sym, fd, d, types.InvalidType, params)
	}
	stubAnnotations(g.cls.LastMethod(), fd, fd.Sym.Ext, params)
	if fd.Sym.External && d == funDesc(fd.Sym) {
		return
	}
	g.code.Line(fd.P.Line)
	g.objectRef(comp)
	for _, pd := range descParams(d) {
		g.code.Local(loadOpOf(pd), g.code.AllocLocal(pd))
	}
	g.code.Invoke(cf.INVOKEVIRTUAL, comp.Name, name, d)
	g.code.Op(returnOpOf(d[strings.IndexByte(d, ')')+1:]))
	g.check()
}

// defaultImpls generates the DefaultImpls class of an interface compiled
// with -jvm-default=enable or annotated @JvmDefaultWithCompatibility, for
// code compiled to call the bodies of its members there, as kotlinc
// writes it: for each member with a body, its own and those it inherits
// from Kotlin interfaces, a static method that takes the interface first
// and calls the interface's static access$name$jd, which calls the member
// with invokespecial; and for each of its own functions with default
// values, a name$default method that calls the interface's.
func (g *gen) defaultImpls(cd *syntax.ClassDecl, iface *cf.Class) *cf.Class {
	if c := g.shared.impls[cd.Sym]; c != nil && !cd.Sym.JVMCompat {
		// An interface compiled without JVM default methods, or one with
		// only the forwarders of implsClass.
		return c
	}
	if !cd.Interface || !cd.Sym.JVMCompat {
		return nil
	}
	own := ownBodies(cd.Sym)
	var ms []defaultBody
	for _, m := range allBodies(cd.Sym) {
		if !overrides(own, m) {
			ms = append(ms, m)
		}
	}
	defaults := false
	for _, b := range own {
		defaults = defaults || b.decl != nil && hasDefaults(b.decl.Params)
	}
	begun := g.shared.begun[cd.Sym] // with $annotations methods
	if len(ms) == 0 && !defaults && (begun == nil || begun.NumMethods() == 0) {
		return nil
	}
	self := "L" + iface.Name + ";"
	c := g.implsOf(cd.Sym, iface)
	body := func(m defaultBody) {
		d := "(" + self + m.desc[1:]
		ret := m.desc[strings.IndexByte(m.desc, ')')+1:]
		jd := "access$" + m.name + "$jd"
		code := iface.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccSynthetic, jd, d)
		for _, pd := range descParams(d) {
			code.Local(loadOpOf(pd), code.AllocLocal(pd))
		}
		code.InvokeSpecialInterface(iface.Name, m.name, m.desc)
		code.Op(returnOpOf(ret))
		if err := code.Err(); err != nil {
			panic(err)
		}
		code = c.NewMethod(cf.AccPublic|cf.AccStatic|m.synthetic(), m.name, d)
		if m.decl != nil && m.decl.Sym.Owner == cd.Sym && !m.hidden {
			g.signFun(c.LastMethod(), m.decl.Sym, m.decl, d, cd.Sym.This())
		}
		g.code = code
		if m.line > 0 {
			code.Line(m.line)
		}
		// As kotlinc writes them, they are deprecated: code compiled
		// against the interface calls its default methods.
		ref := c.LastMethod()
		ref.Deprecate()
		ref.Annotate(cf.Annotation{Type: "Ljava/lang/Deprecated;"}, true)
		nullability(methodAdder(ref), m.ret)
		nullability(paramAdder(ref, 0), types.ClassType(cd.Sym))
		for i, p := range m.params {
			nullability(paramAdder(ref, i+1), p.Type)
		}
		pds := descParams(d)
		slots := make([]int, len(pds))
		for i, pd := range pds {
			slots[i] = code.AllocLocal(pd)
		}
		code.NameLocal(slots[0], "$this")
		if len(m.params) == len(pds)-1 {
			for i, p := range m.params {
				code.NameLocal(slots[i+1], p.Name)
				g.checkParam(slots[i+1], p.Type, p.Name)
			}
		}
		for i, pd := range pds {
			code.Local(loadOpOf(pd), slots[i])
		}
		code.InvokeStaticInterface(iface.Name, jd, d)
		code.Op(returnOpOf(ret))
		if err := code.Err(); err != nil {
			panic(err)
		}
	}
	nown := 0 // its own bodies, which come first in ms
	for _, m := range own {
		if !m.abstract {
			body(m)
			nown++
		}
		if m.decl != nil && hasDefaults(m.decl.Params) {
			// name$default, which calls the interface's.
			i := strings.IndexByte(m.desc, ')')
			d := "(" + self + m.desc[1:i] + "ILjava/lang/Object;" + m.desc[i:]
			code := c.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccSynthetic, m.name+"$default", d)
			code.Line(m.line)
			for _, pd := range descParams(d) {
				code.Local(loadOpOf(pd), code.AllocLocal(pd))
			}
			code.InvokeStaticInterface(iface.Name, m.name+"$default", d)
			code.Op(returnOpOf(m.desc[i+1:]))
			if err := code.Err(); err != nil {
				panic(err)
			}
		}
	}
	for _, m := range ms[nown:] {
		body(m)
	}
	return c
}

// overrides reports whether one of an interface's own members, own,
// overrides inherited member m with another result type, as String
// build() does Any build(): the interface's DefaultImpls has no method
// for m.
func overrides(own []defaultBody, m defaultBody) bool {
	params := m.desc[:strings.IndexByte(m.desc, ')')]
	for _, o := range own {
		if o.name == m.name && o.desc != m.desc && strings.HasPrefix(o.desc, params+")") {
			return true
		}
	}
	return false
}

// defaultBody is a member of a Kotlin interface compiled with JVM default
// methods: a function or a property accessor, with a body unless abstract.
type defaultBody struct {
	name, desc string
	params     []*types.VarSym
	ret        types.Type
	abstract   bool
	// decl is the program's function, line the line of the program's
	// member, 0 for the class path's.
	decl *syntax.FunDecl
	line int
	// owner is the interface that declares it; hidden is set if it is
	// @Deprecated(level = HIDDEN), which kotlinc makes synthetic.
	owner  *types.ClassSym
	hidden bool
}

func (m defaultBody) synthetic() uint16 {
	if m.hidden {
		return cf.AccSynthetic
	}
	return 0
}

// ownBodies returns the members that interface s declares and that other
// classes see, the program's in declaration order: those with bodies,
// and the abstract ones, which hide the bodies of the members they
// override.
func ownBodies(s *types.ClassSym) []defaultBody {
	var out []defaultBody
	if cd, ok := s.Decl.(*syntax.ClassDecl); ok && !s.External {
		for _, m := range cd.Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				if f := m.Sym; f != nil && m.Visibility != syntax.Private {
					abstract := f.Abstract || m.Body == nil && m.ExprBody == nil
					out = append(out, defaultBody{f.JVM(), funDesc(f), f.Params, f.Ret, abstract, m, m.P.Line, s, f.Hidden})
				}
			case *syntax.PropDecl:
				p := m.Sym
				if p == nil || m.Visibility == syntax.Private {
					continue
				}
				get := defaultBody{name: types.GetterJVM(p), desc: "()" + desc(types.GetterType(p)), ret: types.GetterType(p), abstract: true}
				if a := m.Getter; a != nil && a.Sym != nil && (a.Body != nil || a.ExprBody != nil) {
					f := a.Sym
					get = defaultBody{f.JVM(), funDesc(f), f.Params, f.Ret, false, nil, a.P.Line, s, f.Hidden}
				}
				out = append(out, get)
				if !p.Mutable {
					continue
				}
				set := defaultBody{name: types.SetterJVM(p), desc: "(" + desc(types.FieldType(p)) + ")V", ret: types.UnitType, abstract: true}
				if a := m.Setter; a != nil && a.Sym != nil && (a.Body != nil || a.ExprBody != nil) {
					f := a.Sym
					set = defaultBody{f.JVM(), funDesc(f), f.Params, f.Ret, false, nil, a.P.Line, s, f.Hidden}
				}
				out = append(out, set)
			}
		}
		return out
	}
	s.Ensure()
	add := func(f *types.FunSym) {
		if f != nil && !f.Static && !f.Platform {
			out = append(out, defaultBody{name: f.JVM(), desc: funDesc(f), params: f.Params, ret: f.Ret, abstract: f.Abstract, owner: s, hidden: f.Hidden})
		}
	}
	for _, name := range slices.Sorted(maps.Keys(s.Methods)) {
		for _, f := range s.Methods[name] {
			add(f)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(s.KProps)) {
		add(s.KProps[name].Getter)
		add(s.KProps[name].Setter)
	}
	return out
}

// allBodies returns the members with bodies that Kotlin interface s has
// as default methods, its own first, then those it inherits from other
// such interfaces without overriding them, as kotlinc lists them.
func allBodies(s *types.ClassSym) []defaultBody {
	var out []defaultBody
	seen := map[string]bool{} // by name and descriptor
	visited := map[*types.ClassSym]bool{}
	var visit func(s *types.ClassSym)
	visit = func(s *types.ClassSym) {
		if visited[s] {
			return
		}
		visited[s] = true
		for _, m := range ownBodies(s) {
			if k := m.name + m.desc; !seen[k] {
				seen[k] = true
				if !m.abstract {
					out = append(out, m)
				}
			}
		}
		for _, i := range s.Interfaces {
			if i.JVMDefault {
				visit(i)
			}
		}
	}
	visit(s)
	return out
}

// defaultBridges gives class cd, compiled as with kotlinc's
// -jvm-default=enable, a bridge for each member with a body that it
// inherits from the default method of a Kotlin interface it implements,
// and that neither it nor its superclass implements or inherits: it
// calls the default method with invokespecial, as kotlinc writes them.
func (g *gen) defaultBridges(cd *syntax.ClassDecl, c *cf.Class) {
	cls := cd.Sym
	for _, i := range cls.Interfaces {
		if !i.JVMDefault || cls.Super != nil && cls.Super.IsSubclassOf(i) {
			continue
		}
		for _, m := range allBodies(i) {
			if c.HasMethod(m.name, m.desc) || superImplements(cls.Super, m) || cls.Super != nil && cls.Super.IsSubclassOf(m.owner) {
				continue
			}
			code := c.NewMethod(cf.AccPublic|cf.AccBridge|m.synthetic(), m.name, m.desc)
			ref := c.LastMethod()
			nullability(methodAdder(ref), m.ret)
			for i, p := range m.params {
				nullability(paramAdder(ref, i), p.Type)
			}
			code.Line(cd.P.Line)
			code.AllocLocal("L" + cls.Name + ";")
			pds := descParams(m.desc)
			slots := make([]int, len(pds))
			for j, pd := range pds {
				slots[j] = code.AllocLocal(pd)
				if len(m.params) == len(pds) {
					code.NameLocal(slots[j], m.params[j].Name)
				}
			}
			code.Local(cf.ALOAD, 0)
			for j, pd := range pds {
				code.Local(loadOpOf(pd), slots[j])
			}
			code.InvokeSpecialInterface(i.Name, m.name, m.desc)
			code.Op(returnOpOf(m.desc[strings.IndexByte(m.desc, ')')+1:]))
			if err := code.Err(); err != nil {
				panic(err)
			}
		}
	}
}

// superImplements reports whether class s or a superclass has a method
// that implements interface member m.
func superImplements(s *types.ClassSym, m defaultBody) bool {
	for ; s != nil; s = s.Super {
		s.Ensure()
		for _, fs := range s.Methods {
			for _, f := range fs {
				if !f.Abstract && !f.Static && f.JVM() == m.name && funDesc(f) == m.desc {
					return true
				}
			}
		}
		for _, p := range s.Props {
			if types.GetterJVM(p) == m.name && "()"+desc(types.GetterType(p)) == m.desc {
				return true
			}
		}
		for _, kp := range s.KProps {
			if g := kp.Getter; !g.Abstract && g.JVM() == m.name && funDesc(g) == m.desc {
				return true
			}
		}
	}
	return false
}
