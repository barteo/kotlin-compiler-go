package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The JVM signature clashes of a class's members with its inherited ones,
// and with the methods and fields that kotlinc adds to it: a getter's
// with a function's, getX(), an erasure's with a bridge's, foo$default's,
// an object's INSTANCE field's.

// jvmMember is a method, or a field, that a class or a file facade has
// on the JVM.
type jvmMember struct {
	sig     string // its JVM name and descriptor, or "field " and its name and descriptor
	pos     syntax.Pos
	fun     *types.FunSym // the function, or the accessor of an extension or a delegated property
	prop    *types.VarSym // the property that it is an accessor of, if it isn't an extension
	extProp string        // the name of the extension property that it is an accessor of
	setter  bool          // it is a property's setter
	static  bool
	private bool
	synth   bool // a method that kotlinc adds: foo$default, a @JvmOverloads overload, values()
}

// accessor reports whether m is a property's getter or setter.
func (m *jvmMember) accessor() bool { return m.prop != nil || m.extProp != "" }

// field reports whether m is a field.
func (m *jvmMember) field() bool { return strings.HasPrefix(m.sig, "field ") }

// objectFinals are the final methods of java.lang.Object, which no class
// or file facade may declare.
var objectFinals = map[string]bool{
	"notify()V": true, "notifyAll()V": true, "wait()V": true, "wait(J)V": true, "wait(JI)V": true,
	"getClass()Ljava/lang/Class;": true,
}

// classMembers returns the JVM methods, and the fields that may clash, of
// class d: its functions, its properties' accessors, delegation's methods
// and the methods and fields kotlinc adds.
func (c *Checker) classMembers(d *syntax.ClassDecl) []jvmMember {
	cls := d.Sym
	var out []jvmMember
	for _, cp := range d.CtorParams {
		if cp.PropSym != nil {
			out = c.propMembers(out, cp.PropSym, cp.Visibility == syntax.Private, cp.P, cp.P, cp.P)
		}
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			if m.Sym != nil && !m.Expect {
				out = c.funMembers(out, m, cls)
			}
		case *syntax.PropDecl:
			if m.Sym == nil || m.Expect {
				continue
			}
			if m.Recv == nil {
				gp, sp := m.P, m.P
				if m.Getter != nil {
					gp = m.Getter.P
				}
				if m.Setter != nil {
					sp = m.Setter.P
				}
				out = c.propMembers(out, m.Sym, m.Visibility == syntax.Private, m.P, gp, sp)
			} else if m.Getter != nil && m.Getter.Sym != nil {
				out = extPropMembers(out, m)
			}
		}
	}
	for _, dl := range d.Delegated {
		f := dl.Sym
		if f == nil {
			continue
		}
		desc := jvmDesc(f)
		if desc == "" {
			continue
		}
		m := jvmMember{sig: types.InstanceJVM(f) + desc, pos: d.P, fun: f, extProp: dl.ExtProp}
		if dl.ExtProp == "" {
			for _, p := range cls.Props {
				if p.Decl == dl || p.Getter == f || p.Setter == f {
					m.prop, m.setter = p, p.Setter == f
					break
				}
			}
		} else {
			m.setter = f.Name != types.GetterName(dl.ExtProp)
		}
		out = append(out, m)
	}
	if d.Data {
		// copy$default, of the copy that kotlinc generates: all its
		// parameters have default values.
		for _, f := range cls.Methods["copy"] {
			if f.Decl == nil && len(f.Params) > 0 {
				out = append(out, defaultsMember(f, cls, false, d.P))
			}
		}
	}
	if d.Enum {
		e := "L" + cls.Name + ";"
		out = append(out,
			jvmMember{sig: "values()[" + e, pos: d.P, static: true, synth: true},
			jvmMember{sig: "valueOf(Ljava/lang/String;)" + e, pos: d.P, static: true, synth: true})
	}
	if d.Object && !d.Anonymous && cls.CompanionOf == nil {
		out = append(out, jvmMember{sig: "field INSTANCE:L" + cls.Name + ";", pos: d.P, static: true, synth: true})
	}
	return out
}

// propMembers appends the accessors of property p, which private says is
// declared private, and its field if it is public and named INSTANCE.
func (c *Checker) propMembers(out []jvmMember, p *types.VarSym, private bool, pos, getPos, setPos syntax.Pos) []jvmMember {
	ensureVar(p)
	if p.Composable || p.Type.K == types.Invalid {
		return out
	}
	if p.Name == "INSTANCE" && !private && (p.JvmField || p.Lateinit && !p.SetterPrivate) {
		if fd, ok := types.Desc(types.FieldType(p)); ok {
			out = append(out, jvmMember{sig: "field INSTANCE:" + fd, pos: pos, prop: p})
		}
	}
	if p.Private || p.JvmField || p.Const {
		return out
	}
	ctx, ok := types.ParamsDesc(p.Contexts)
	if !ok {
		return out
	}
	ctx = ctx[:len(ctx)-1]
	if gd, ok := types.Desc(types.GetterType(p)); ok {
		out = append(out, jvmMember{sig: types.GetterJVM(p) + ctx + ")" + gd, pos: getPos, prop: p, private: private, static: p.JvmStatic && p.Owner != nil && p.Owner.CompanionOf == nil})
	}
	if p.Mutable && !p.SetterPrivate {
		if fd, ok := types.ParamDesc(types.FieldType(p)); ok {
			out = append(out, jvmMember{sig: types.SetterJVM(p) + ctx + fd + ")V", pos: setPos, prop: p, setter: true, private: private,
				static: p.JvmStatic && p.Owner != nil && p.Owner.CompanionOf == nil})
		}
	}
	return out
}

// extPropMembers appends the accessors of extension property d.
func extPropMembers(out []jvmMember, d *syntax.PropDecl) []jvmMember {
	private := d.Visibility == syntax.Private
	get := d.Getter.Sym
	if desc := jvmDesc(get); desc != "" {
		out = append(out, jvmMember{sig: types.InstanceJVM(get) + desc, pos: d.Getter.P, fun: get, extProp: d.Name, private: private})
	}
	if set := get.ExtSetter; set != nil {
		pos := d.P
		if d.Setter != nil {
			pos = d.Setter.P
		}
		if desc := jvmDesc(set); desc != "" {
			out = append(out, jvmMember{sig: types.InstanceJVM(set) + desc, pos: pos, fun: set, extProp: d.Name, setter: true,
				private: private || d.SetterVis == syntax.Private})
		}
	}
	return out
}

// funMembers appends function d of class cls (nil in a file facade), and
// the methods kotlinc adds for it: name$default, which takes the values
// of its parameters, a mask of those omitted and a marker, and the
// overloads of @JvmOverloads.
func (c *Checker) funMembers(out []jvmMember, d *syntax.FunDecl, cls *types.ClassSym) []jvmMember {
	f := d.Sym
	ensureFun(f)
	if f.Composable {
		return out
	}
	desc := jvmDesc(f)
	if desc == "" {
		return out
	}
	static := cls == nil || f.Static
	out = append(out, jvmMember{sig: types.InstanceJVM(f) + desc, pos: d.P, fun: f, static: static, private: d.Visibility == syntax.Private})
	var defaults []int
	for i, p := range d.Params {
		if p.Default != nil {
			defaults = append(defaults, i)
		}
	}
	if len(defaults) == 0 || cls != nil && (cls.Interface || cls.Value) || d.Override {
		return out
	}
	out = append(out, defaultsMember(f, cls, static, d.P))
	if d.Find("", "JvmOverloads") == nil {
		return out
	}
	// Without each parameter with a default value and the ones after it
	// that have one.
	for n := len(defaults) - 1; n >= 0; n-- {
		g := *f
		g.Params = nil
		for i, p := range f.Params {
			if !containsInt(defaults[n:], i) {
				g.Params = append(g.Params, p)
			}
		}
		if desc := jvmDesc(&g); desc != "" {
			out = append(out, jvmMember{sig: types.InstanceJVM(f) + desc, pos: d.P, static: static, synth: true})
		}
	}
	return out
}

func containsInt(s []int, n int) bool {
	for _, x := range s {
		if x == n {
			return true
		}
	}
	return false
}

// defaultsMember returns the static method name$default of function f of
// class cls: it takes the instance first, unless f is static, then f's
// parameters, the masks of those omitted and a marker.
func defaultsMember(f *types.FunSym, cls *types.ClassSym, static bool, pos syntax.Pos) jvmMember {
	desc := jvmDesc(f)
	i := strings.IndexByte(desc, ')')
	if desc == "" || i < 0 {
		return jvmMember{}
	}
	var sb strings.Builder
	sb.WriteByte('(')
	if !static && cls != nil {
		sb.WriteString("L" + cls.Name + ";")
	}
	sb.WriteString(desc[1:i])
	sb.WriteString(strings.Repeat("I", types.MaskInts(types.DefaultBits(f))))
	sb.WriteString("Ljava/lang/Object;")
	sb.WriteString(desc[i:])
	return jvmMember{sig: types.InstanceJVM(f) + "$default" + sb.String(), pos: pos, static: true, synth: true}
}

// sameMember reports whether a and b, seen from a receiver of type this,
// are the same Kotlin member: of which one overrides the other, or both
// one of a third.
func (c *Checker) sameMember(this types.Type, a, b *jvmMember) bool {
	switch {
	case a.prop != nil && b.prop != nil:
		return a.prop.Name == b.prop.Name && a.setter == b.setter &&
			c.sameContextsIn(a.prop.Contexts, c.memberSubst(this, a.prop.Owner), b.prop.Contexts, c.memberSubst(this, b.prop.Owner))
	case a.prop != nil || b.prop != nil || a.fun == nil || b.fun == nil:
		return false
	case a.extProp != "" || b.extProp != "":
		return a.extProp == b.extProp && a.setter == b.setter && c.sameParams(this, a.fun, b.fun)
	}
	return a.fun.Name == b.fun.Name && c.sameParams(this, a.fun, b.fun)
}

// inheritedMembers returns the JVM methods that class cls inherits from
// its supertypes declared in the program, but not by value classes: their
// instance methods, of each one's own erasure, as its bridges have them.
func (c *Checker) inheritedMembers(cls *types.ClassSym) []jvmMember {
	var out []jvmMember
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls || s.External {
			return true
		}
		sd, ok := s.Decl.(*syntax.ClassDecl)
		if !ok || s.Value || sd.Expect {
			return true
		}
		for _, m := range c.classMembers(sd) {
			if !m.static && !m.private && !m.field() && (m.fun != nil || m.prop != nil) {
				out = append(out, m)
			}
		}
		return true
	})
	return out
}

// classClashes reports the JVM signature clashes of class d: its members'
// with each other's, with those it inherits that they don't override, and
// those of members it inherits from different supertypes.
func (c *Checker) classClashes(d *syntax.ClassDecl) {
	cls := d.Sym
	if cls == nil || d.Expect || d.Annotation {
		return
	}
	c.ctorClashes(d)
	if cls.Value {
		return
	}
	own := c.classMembers(d)
	c.sameContainer(own)
	if c.bigArityFuns(cls) {
		c.errorf(d.P, "inherited platform declarations clash: class '%s' implements several function types with more than 22 parameters", cls.DisplayName())
	}
	inherited := c.inheritedMembers(cls)
	this := cls.This()
	ownSigs := map[string]bool{}
	for i := range own {
		m := &own[i]
		ownSigs[m.sig] = true
		if m.private || m.field() {
			continue
		}
		if !cls.Interface && !m.static && objectFinals[m.sig] {
			c.errorf(m.pos, "accidental override: the following declarations have the same JVM signature (%s)", m.sig)
			continue
		}
		for j := range inherited {
			g := &inherited[j]
			if g.sig == m.sig && !c.sameMember(this, m, g) {
				c.errorf(m.pos, "accidental override: the following declarations have the same JVM signature (%s)", m.sig)
				break
			}
		}
	}
	if comp := cls.Companion; comp != nil {
		c.jvmStaticClashes(comp, inherited, this)
	}
	// Members inherited from different supertypes, which the class
	// doesn't declare one of.
	for i := range inherited {
		g := &inherited[i]
		if ownSigs[g.sig] {
			continue
		}
		for j := i + 1; j < len(inherited); j++ {
			h := &inherited[j]
			if h.sig == g.sig && !(g.fun != nil && g.fun == h.fun || g.prop != nil && g.prop == h.prop && g.setter == h.setter) && !c.sameMember(this, g, h) {
				c.errorf(d.P, "inherited platform declarations clash: the following declarations have the same JVM signature (%s)", g.sig)
				return
			}
		}
	}
}

// jvmStaticClashes reports the @JvmStatic functions of companion object
// comp whose static methods in its outer class, of type this, have the
// JVM signature of an instance method the outer class inherits.
func (c *Checker) jvmStaticClashes(comp *types.ClassSym, inherited []jvmMember, this types.Type) {
	cd, ok := comp.Decl.(*syntax.ClassDecl)
	if !ok {
		return
	}
	for _, m := range cd.Members {
		fd, ok := m.(*syntax.FunDecl)
		if !ok || fd.Sym == nil || fd.Find("", "JvmStatic") == nil || fd.Visibility == syntax.Private {
			continue
		}
		desc := jvmDesc(fd.Sym)
		if desc == "" {
			continue
		}
		sig := types.InstanceJVM(fd.Sym) + desc
		for _, g := range inherited {
			if g.sig == sig {
				c.errorf(fd.P, "accidental override: the following declarations have the same JVM signature (%s)", sig)
				break
			}
		}
	}
}

// sameContainer reports the members of one class or file facade that
// have the same JVM signature.
func (c *Checker) sameContainer(ms []jvmMember) {
	seen := map[string]*jvmMember{}
	for i := range ms {
		m := &ms[i]
		if m.sig == "" {
			continue
		}
		first := seen[m.sig]
		if first == nil {
			seen[m.sig] = m
			continue
		}
		if m.fun != nil && first.fun != nil && c.u.conflicted[m.fun] && c.u.conflicted[first.fun] {
			continue // conflicting overloads, reported as such
		}
		// At both, if one is a method that kotlinc adds: at the function
		// it adds it for, or at the class.
		sig := strings.TrimPrefix(m.sig, "field ")
		if (m.synth || first.synth) && first.pos != m.pos {
			c.errorf(first.pos, "platform declaration clash: the following declarations have the same JVM signature (%s)", sig)
		}
		c.errorf(m.pos, "platform declaration clash: the following declarations have the same JVM signature (%s)", sig)
	}
}

// bigArityFuns reports whether class cls implements function types of
// more than one arity above 22: on the JVM, they are all FunctionN.
func (c *Checker) bigArityFuns(cls *types.ClassSym) bool {
	n := 0
	for _, s := range cls.Interfaces {
		if s.Func && !s.Suspend && !s.Composable && s.Name == "kotlin/jvm/functions/FunctionN" {
			n++
		}
	}
	return n > 1
}

// ctorClashes reports the constructors of class d that have the same JVM
// descriptor but not the same parameter types: one that takes a value
// class's instance takes its underlying value, and List<A> is List<B>.
func (c *Checker) ctorClashes(d *syntax.ClassDecl) {
	seen := map[string]*types.FunSym{}
	add := func(f *types.FunSym, pos syntax.Pos) {
		if f == nil {
			return
		}
		desc, ok := types.ParamsDesc(f.Params)
		if !ok {
			return
		}
		if g := seen[desc]; g != nil && len(g.Params) == len(f.Params) {
			for i, p := range f.Params {
				if !c.sameType(p.Type, g.Params[i].Type) {
					c.errorf(pos, "platform declaration clash: the following constructors have the same JVM signature (<init>%sV)", desc)
					return
				}
			}
			return
		}
		seen[desc] = f
	}
	if d.HasPrimary {
		add(d.Ctor, d.P)
	}
	for _, sc := range d.Secondaries() {
		add(sc.Sym, sc.P)
	}
}

// jvmDesc returns f's method descriptor, as memberDesc does, or "" if any
// of its receiver, context parameters or parameters has no JVM type, as
// one whose class is unresolved: memberDesc then leaves the receiver or
// contexts out, and a descriptor without them could clash with another.
func jvmDesc(f *types.FunSym) string {
	if f.Desc == "" {
		if _, ok := types.ParamsDesc(f.Contexts); !ok {
			return ""
		}
		if f.Ext {
			if _, ok := types.ParamDesc(f.Recv); !ok {
				return ""
			}
		}
	}
	return memberDesc(f)
}
