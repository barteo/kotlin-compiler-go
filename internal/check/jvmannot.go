package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The JVM annotations of kotlin.jvm have rules of their own, which
// kotlinc's JVM checkers enforce: where @JvmOverloads, @JvmStatic,
// @Synchronized, @JvmSynthetic and @Strictfp can be written, which names
// @JvmName gives, and which packages @JvmPackageName does. jvmAnnotation
// checks those of one annotation by where it is written, and
// jvmMemberAnnotations those that depend on the declaration's class.

// jvmAnnotation checks annotation a, of class cls, written on site.
func (c *Checker) jvmAnnotation(a *syntax.Annotation, cls *types.ClassSym, site annotSite) {
	if !strings.HasPrefix(cls.Name, "kotlin/jvm/") {
		return
	}
	switch strings.TrimPrefix(cls.Name, "kotlin/jvm/") {
	case "JvmName":
		if s, ok := a.Str("name"); ok && !validJvmName(s) {
			c.errorf(a.P, "illegal JVM name")
		}
		if site == localFunSite {
			c.errorf(a.P, inapplicableJvmName)
		}
	case "JvmPackageName":
		if s, ok := a.Str("name"); ok && site == fileSite {
			c.jvmPackageName(a, s)
		}
	case "Strictfp":
		if site.kind == siteClass || site == objectExprSite {
			c.errorf(a.P, "'@Strictfp' annotation on classes is unsupported yet")
		}
	case "JvmOverloads":
		if site == localFunSite {
			c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on local declarations")
		}
	case "JvmExposeBoxed":
		if name, pos, named := exposedName(a); named {
			if site.kind == siteClass || site.kind == siteCtor {
				c.errorf(pos, "'@JvmExposeBoxed' with name is applicable only to functions, getters and setters")
			}
			if name != "" && !validJvmName(name) {
				c.errorf(pos, "illegal JVM name")
			}
		}
		if site == localFunSite {
			c.errorf(a.P, "'@JvmExposeBoxed' cannot expose local functions")
		}
	case "JvmStatic":
		if site == localFunSite {
			c.errorf(a.P, "only members in named objects and companion objects can be annotated with '@JvmStatic'")
		}
	}
}

// validJvmName reports whether s can name a method or a class on the
// JVM: it isn't empty, and has none of the characters the class file
// format reserves.
func validJvmName(s string) bool {
	return s != "" && !strings.ContainsAny(s, ".;[/<>")
}

// jvmPackageName checks name, the value of @file:JvmPackageName a.
func (c *Checker) jvmPackageName(a *syntax.Annotation, name string) {
	switch {
	case name == "":
		c.errorf(a.P, "'@JvmPackageName' annotation value cannot be empty")
		return
	case !validPackageName(name):
		c.errorf(a.P, "'@JvmPackageName' annotation value must be a valid dot-qualified name of a package")
		return
	}
	if c.file != nil {
		for _, d := range c.file.Decls {
			if _, ok := d.(*syntax.ClassDecl); ok {
				// kotlinc's "is not supported for files with class
				// declarations", reworded: kc's "not supported" are
				// features it lacks.
				c.errorf(a.P, "'@JvmPackageName' annotation cannot be used in files with class declarations")
				return
			}
		}
	}
}

// validPackageName reports whether s is a dot-qualified name of Java
// identifiers.
func validPackageName(s string) bool {
	for _, part := range strings.Split(s, ".") {
		if part == "" {
			return false
		}
		for i, r := range part {
			letter := r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 0x7f
			if !letter && (i == 0 || r < '0' || r > '9') {
				return false
			}
		}
	}
	return true
}

// inapplicableJvmName is the message of INAPPLICABLE_JVM_NAME.
const inapplicableJvmName = "'@JvmName' annotation is not applicable to this declaration"

// renamesVirtual reports whether @JvmName can't rename a function of
// owner with modifiers m (INAPPLICABLE_JVM_NAME): an override, or one
// that can be overridden, whose overrides would keep the JVM name it
// overrides.
func renamesVirtual(m syntax.Modifiers, owner *types.ClassSym) bool {
	return owner != nil && m.Override || overridable(m, owner)
}

// overridable reports whether a member of owner with modifiers m can be
// overridden, as a property's accessors, which kotlinc checks for
// INAPPLICABLE_JVM_NAME, can be: they aren't overrides themselves.
func overridable(m syntax.Modifiers, owner *types.ClassSym) bool {
	if owner == nil || m.Visibility == syntax.Private || owner.Final && !owner.Interface {
		return false
	}
	return owner.Interface || m.Open || m.Abstract || m.Override && !m.Final
}

// jvmMemberAnnotations checks the JVM annotations of declaration d, in
// class owner (nil at top level), that depend on what owner is.
func (c *Checker) jvmMemberAnnotations(d syntax.Decl, owner *types.ClassSym) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		c.overloads(&d.Modifiers, owner)
		c.synchronized(&d.Modifiers, owner, "")
		if a := d.Find("", "JvmName"); a != nil && d.AccessorOf == nil && renamesVirtual(d.Modifiers, owner) {
			c.errorf(a.P, inapplicableJvmName)
		}
	case *syntax.PropDecl:
		if overridable(d.Modifiers, owner) {
			for _, a := range []*syntax.Annotation{d.Find("get", "JvmName"), d.Find("set", "JvmName")} {
				if a != nil {
					c.errorf(a.P, inapplicableJvmName)
				}
			}
			for _, acc := range []*syntax.FunDecl{d.Getter, d.Setter} {
				if acc == nil {
					continue
				}
				if a := acc.Find("", "JvmName"); a != nil {
					c.errorf(a.P, inapplicableJvmName)
				}
			}
		}
		for _, acc := range []*syntax.FunDecl{d.Getter, d.Setter} {
			if acc != nil {
				c.synchronized(&acc.Modifiers, owner, "")
			}
		}
		c.synchronized(&d.Modifiers, owner, "get")
		c.synchronized(&d.Modifiers, owner, "set")
		c.jvmFieldApplicable(d, owner)
		if d.Field != nil {
			if a := d.Find("delegate", "JvmSynthetic"); a != nil {
				c.errorf(a.P, "'@JvmSynthetic' annotation cannot be used on delegated properties")
			}
		}
	case *syntax.ClassDecl:
		if a := d.CtorMods.Find("", "JvmOverloads"); a != nil && d.Annotation {
			c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on constructors of annotation classes")
		}
		for _, cp := range d.CtorParams {
			if a := cp.Modifiers.Find("", "JvmStatic"); a != nil && cp.IsProp {
				c.errorf(a.P, "only members in named objects and companion objects can be annotated with '@JvmStatic'")
			}
			if a := cp.Modifiers.Find("", "JvmField"); a != nil && cp.IsProp && cp.PropSym != nil {
				switch {
				case cp.Visibility == syntax.Private:
					c.errorf(a.P, "JvmField has no effect on a private property")
				case cp.Open || cp.Override:
					c.errorf(a.P, "JvmField cannot be applied to a property that overrides some other property or can be overridden")
				case c.valueClassType(cp.PropSym.Type):
					c.errorf(a.P, "JvmField cannot be applied to a property of an inline class type")
				}
			}
		}
	}
}

// jvmFieldApplicable checks where property d, in class owner (nil at top
// level), is annotated @JvmField, which needs a field it can make public:
// one of a final property with default accessors, not delegated, const or
// lateinit, of a type that isn't a value class.
func (c *Checker) jvmFieldApplicable(d *syntax.PropDecl, owner *types.ClassSym) {
	a := d.Find("", "JvmField")
	if a == nil {
		if a = d.Find("delegate", "JvmField"); a == nil {
			return
		}
	}
	inapplicable := func(why string) { c.errorf(a.P, "JvmField %s", why) }
	switch {
	case d.Field != nil:
		inapplicable("cannot be applied to a delegated property")
	case d.Abstract || owner != nil && owner.Interface:
		c.errorf(a.P, "this annotation is not applicable to target 'member property without backing field'")
	case d.Sym != nil && !d.Sym.HasField():
		c.errorf(a.P, "this annotation is not applicable to target 'member property without backing field'")
	case d.Const:
		inapplicable("cannot be applied to a const property")
	case d.Lateinit:
		inapplicable("cannot be applied to a lateinit property")
	case d.Visibility == syntax.Private:
		inapplicable("has no effect on a private property")
	case d.Open || d.Override:
		inapplicable("cannot be applied to a property that overrides some other property or can be overridden")
	case d.Getter != nil || d.Setter != nil || d.AccessorP.Line != 0:
		inapplicable("cannot be applied to a property with a custom accessor")
	case owner != nil && owner.CompanionOf != nil && owner.CompanionOf.Annotation:
		inapplicable("cannot be applied to a property of a companion object of an annotation class")
	case owner != nil && owner.CompanionOf != nil && owner.CompanionOf.Interface && !c.interfaceCompanionFields(owner):
		inapplicable("could be applied only if all interface companion properties are 'public final val' with '@JvmField' annotation")
	case d.Sym != nil && c.valueClassType(d.Sym.Type):
		inapplicable("cannot be applied to a property of an inline class type")
	}
}

// interfaceCompanionFields reports whether every property of companion,
// an interface's companion object, is a public final val with @JvmField,
// as kotlinc requires of any of them to be a field.
func (c *Checker) interfaceCompanionFields(companion *types.ClassSym) bool {
	cd, ok := companion.Decl.(*syntax.ClassDecl)
	if !ok {
		return true
	}
	for _, m := range cd.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && (pd.Mutable || pd.Visibility != syntax.Public || pd.Open || pd.Find("", "JvmField") == nil) {
			return false
		}
	}
	return true
}

// valueClassType reports whether t is a value class's type, which a
// @JvmField can't expose: its field holds the underlying value.
func (c *Checker) valueClassType(t types.Type) bool {
	t = t.Flex()
	return t.K == types.Class && t.C != nil && !t.N && ensured(t.C).Value
}

// overloads checks the @JvmOverloads of a member function of owner with
// modifiers m.
func (c *Checker) overloads(m *syntax.Modifiers, owner *types.ClassSym) {
	a := m.Find("", "JvmOverloads")
	if a == nil || owner == nil {
		return
	}
	switch {
	case owner.Interface:
		c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on interface methods")
	case m.Abstract:
		c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on abstract methods")
	case localClass(owner):
		c.errorf(a.P, "'@JvmOverloads' annotation cannot be used on local declarations")
	}
}

// synchronized checks the @Synchronized, with use-site target target, of
// a member of owner with modifiers m.
func (c *Checker) synchronized(m *syntax.Modifiers, owner *types.ClassSym, target string) {
	a := m.Find(target, "Synchronized")
	if a == nil || owner == nil {
		return
	}
	switch {
	case owner.Interface:
		c.errorf(a.P, "'@Synchronized' annotation cannot be used on interface members")
	case m.Abstract:
		c.errorf(a.P, "'@Synchronized' annotation cannot be used on abstract functions")
	}
}

// projectedAnnotationParams reports the parameters of annotation class d
// whose array types have projected type arguments, as Array<out A>.
func (c *Checker) projectedAnnotationParams(d *syntax.ClassDecl) {
	for _, cp := range d.CtorParams {
		t := cp.Type
		if t == nil || t.Name != "Array" && t.Name != "kotlin.Array" {
			continue
		}
		for _, arg := range t.Args {
			if arg.Variance == types.Out || arg.Variance == types.In {
				c.errorf(t.P, "projections are not allowed in types of annotation members")
				break
			}
		}
	}
}

// multifileSynthetic reports the parts of a multi-file class that aren't
// annotated @file:JvmSynthetic when another part is, as kotlinc's
// NOT_ALL_MULTIFILE_CLASS_PARTS_ARE_JVM_SYNTHETIC: the facade is either
// synthetic or not.
func multifileSynthetic(checkers []*Checker) {
	synthetic := map[string]bool{}
	for _, c := range checkers {
		if f := c.file; f.Part != "" && fileAnnotation(f, "JvmSynthetic") != nil {
			synthetic[f.Facade] = true
		}
	}
	for _, c := range checkers {
		f := c.file
		if f.Part == "" || !synthetic[f.Facade] || fileAnnotation(f, "JvmSynthetic") != nil {
			continue
		}
		if a := fileAnnotation(f, "JvmMultifileClass"); a != nil {
			c.errorf(a.P, "not all parts of the multi-file class are annotated @JvmSynthetic")
		}
	}
}

// fileAnnotation returns file f's annotation of simple name name, or nil.
func fileAnnotation(f *syntax.File, name string) *syntax.Annotation {
	for i := range f.Annotations {
		if a := &f.Annotations[i]; a.Is(name) {
			return a
		}
	}
	return nil
}

// exposeBoxed checks @JvmExposeBoxed on the declarations among ds, in
// class owner (nil at top level), as kotlinc's FirJvmExposeBoxedChecker
// does: a private declaration, or one in a private class if outer, can't
// have it, as Java can't call it; nor can an interface, or an open,
// abstract, synthetic, suspend or reified function; and a function whose
// boxed form would clash with it needs a name of its own.
func (c *Checker) exposeBoxed(ds []syntax.Decl, owner *types.ClassSym, outer bool) {
	report := func(as []syntax.Annotation, private bool, target string) {
		if !private {
			return
		}
		for _, a := range as {
			if a.Is("JvmExposeBoxed") && a.Target == target {
				c.errorf(a.P, "'@JvmExposeBoxed' cannot be applied to a private declaration")
			}
		}
	}
	for _, d := range ds {
		switch d := d.(type) {
		case *syntax.FunDecl:
			report(d.Annotations, outer || d.Visibility == syntax.Private, "")
			if a := d.Find("", "JvmExposeBoxed"); a != nil && d.Sym != nil {
				f := exposedFun{name: d.Name, owner: owner, mods: &d.Modifiers, ret: d.Sym.Ret, contexts: d.Sym.Contexts,
					suspend: d.Suspend, reified: slices.ContainsFunc(d.TypeParams, func(tp *syntax.TypeParam) bool { return tp.Reified })}
				if d.Sym.Ext {
					f.recv = d.Sym.Recv
				}
				for _, p := range d.Sym.Params {
					f.params = append(f.params, p.Type)
				}
				c.exposeBoxedFun(a, f)
			}
		case *syntax.PropDecl:
			private := outer || d.Visibility == syntax.Private
			getter := false
			for i, ms := range d.AccessorMods {
				if !d.AccessorSets[i] && slices.ContainsFunc(ms, func(m syntax.Mod) bool { return m.Name == "private" }) {
					getter = true
				}
			}
			report(d.Annotations, private || getter, "get")
			report(d.Annotations, private || d.SetterVis == syntax.Private, "set")
			if d.Sym == nil {
				continue
			}
			for _, target := range [...]string{"get", "set"} {
				a := d.Find(target, "JvmExposeBoxed")
				if a == nil {
					continue
				}
				f := exposedFun{target: target, owner: owner, mods: &d.Modifiers, ret: d.Sym.Type, contexts: d.Sym.Contexts,
					reified: slices.ContainsFunc(d.TypeParams, func(tp *syntax.TypeParam) bool { return tp.Reified })}
				if d.Recv != nil && d.Recv.Known != nil {
					f.recv = *d.Recv.Known
				}
				if target == "set" {
					f.params, f.ret = []types.Type{d.Sym.Type}, types.UnitType
				}
				c.exposeBoxedFun(a, f)
			}
		case *syntax.ClassDecl:
			private := outer || d.Visibility == syntax.Private
			report(d.Annotations, private, "")
			report(d.CtorMods.Annotations, private || d.CtorMods.Visibility == syntax.Private, "")
			for _, sc := range d.Secondaries() {
				report(sc.Annotations, private || sc.Visibility == syntax.Private, "")
			}
			if a := d.Find("", "JvmExposeBoxed"); a != nil && d.Interface {
				c.errorf(a.P, exposeBoxedOpen)
			}
			c.exposeBoxed(d.Members, d.Sym, private)
		}
	}
}

const exposeBoxedOpen = "'@JvmExposeBoxed' cannot expose functions which are open or abstract, or member of an interface"

// exposedFun is what kotlinc's rules for @JvmExposeBoxed read of a
// function, or of a property's accessor.
type exposedFun struct {
	name     string // a function's
	target   string // an accessor's: get or set
	owner    *types.ClassSym
	mods     *syntax.Modifiers
	recv     types.Type
	contexts []*types.VarSym
	params   []types.Type
	ret      types.Type
	suspend  bool
	reified  bool // has a reified type parameter
}

// overloadable reports whether f's boxed form can have f's name: some of
// its receivers or parameters are of a value class, so that the two
// differ in parameter types.
func (f *exposedFun) overloadable() bool {
	if types.IsValue(f.recv) || f.owner != nil && f.owner.Value {
		return true
	}
	for _, p := range f.contexts {
		if types.IsValue(p.Type) {
			return true
		}
	}
	return slices.ContainsFunc(f.params, types.IsValue)
}

// exposeBoxedFun checks annotation a, @JvmExposeBoxed, on f.
func (c *Checker) exposeBoxedFun(a *syntax.Annotation, f exposedFun) {
	name, pos, named := exposedName(a)
	overloadable := f.overloadable()
	if !named && !overloadable && types.IsValue(f.ret) && f.owner == nil {
		// The boxed form would differ only in its result type.
		c.errorf(a.P, "this declaration requires a name in the '@JvmExposeBoxed' annotation")
	}
	if f.mods.Find(f.target, "JvmSynthetic") != nil {
		c.errorf(a.P, "'@JvmExposeBoxed' cannot expose synthetic functions")
	}
	if overridable(*f.mods, f.owner) {
		c.errorf(a.P, exposeBoxedOpen)
	}
	if named && !overloadable {
		if jn := f.mods.Find(f.target, "JvmName"); jn != nil {
			if s, ok := jn.Str("name"); ok && s == name {
				c.errorf(pos, "the name in '@JvmExposeBoxed' cannot coincide with the one in '@JvmName'")
			}
		}
	}
	if f.reified {
		c.errorf(a.P, "'@JvmExposeBoxed' cannot expose inline functions with reified type parameters")
	}
	if f.suspend {
		c.errorf(a.P, "suspend functions cannot be exposed by '@JvmExposeBoxed'")
	}
	if named && f.target == "" && name == f.name {
		c.errorf(pos, "the name in '@JvmExposeBoxed' cannot coincide with that of the declaration")
	}
}

// exposedName returns the name annotation a, @JvmExposeBoxed, gives, a
// constant string, and where it is written; named is set if a has one.
func exposedName(a *syntax.Annotation) (name string, pos syntax.Pos, named bool) {
	arg := a.Arg(0, "jvmName")
	if arg == nil || len(arg.Toks) == 0 {
		return "", syntax.Pos{}, false
	}
	pos = arg.Toks[0].Pos
	if s, ok := a.Str("jvmName"); ok {
		return s, pos, true
	}
	for _, v := range a.Values {
		if v.Name == "jvmName" && len(v.Elems) == 1 && v.Elems[0].X != nil {
			s, _ := constString(v.Elems[0].X, 0)
			return s, pos, true
		}
	}
	return "", pos, true
}
