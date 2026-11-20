package check

import (
	"maps"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// inheritedFunGroups returns the methods that class d inherits from more
// than one supertype and doesn't override itself, grouped by signature:
// each group holds the members of the supertypes that no other member of
// the group overrides, as Kotlin's intersection override of them has.
// Only the program's methods, of Kotlin or Java sources, are grouped, and
// not those of an interface that d delegates to, whose members delegation
// implements. With jdk, it returns instead the groups whose only members
// from the class path are the JDK's Java methods and that have an
// implementation of the program's (see checkJDKImpls).
func (c *Checker) inheritedFunGroups(d *syntax.ClassDecl, jdk bool) [][]*types.FunSym {
	cls := d.Sym
	this := cls.This()
	var delegated []*types.ClassSym
	for _, st := range d.Supers {
		if st.Delegate != nil && st.T.K == types.Class {
			delegated = append(delegated, st.T.C)
		}
	}
	var groups [][]*types.FunSym
	external := map[int]bool{}
	notJDK := map[int]bool{} // has a member from the class path other than a JDK Java method
	add := func(g *types.FunSym, ext, jdkMember bool) {
		for i, gr := range groups {
			if gr[0].Name == g.Name && c.sameParams(this, gr[0], g) {
				if !slices.Contains(gr, g) {
					groups[i] = append(gr, g)
				}
				external[i] = external[i] || ext
				notJDK[i] = notJDK[i] || ext && !jdkMember
				return
			}
		}
		external[len(groups)] = ext
		notJDK[len(groups)] = ext && !jdkMember
		groups = append(groups, []*types.FunSym{g})
	}
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls || s.Name == "java/lang/Object" {
			return true
		}
		for name, fs := range s.Methods {
			if anyMember[name] {
				continue
			}
			for _, g := range fs {
				if g.Static || g.Private || g.Ext {
					continue
				}
				_, ok := g.Decl.(*syntax.FunDecl)
				// And a method of the program's Java sources.
				java := s.External && isJavaClass(s) && c.u.ext.javaSource(s)
				viaDelegate := slices.ContainsFunc(delegated, func(i *types.ClassSym) bool { return i.IsSubclassOf(s) })
				// A built-in collection type's members are its Java class's.
				add(g, !ok && !java || s.External && !java || viaDelegate, (isJavaClass(s) || s.Java != nil) && !java && !viaDelegate)
			}
		}
		return true
	})
	var out [][]*types.FunSym
	for i, gr := range groups {
		if jdk {
			if !external[i] || notJDK[i] || !slices.ContainsFunc(gr, func(g *types.FunSym) bool { return !g.Abstract && !g.Owner.External }) {
				continue
			}
			if slices.ContainsFunc(gr, func(g *types.FunSym) bool { return g.Owner.External && renamedJDK(g) }) {
				continue
			}
		} else if external[i] {
			continue
		}
		if len(gr) < 2 || !sameJVMParams(gr, jdk) {
			continue
		}
		// The class's own override of them.
		if slices.ContainsFunc(cls.Methods[gr[0].Name], func(f *types.FunSym) bool { return !f.Static && c.sameParams(this, f, gr[0]) }) {
			continue
		}
		gr = slices.DeleteFunc(slices.Clone(gr), func(g *types.FunSym) bool {
			return slices.ContainsFunc(gr, func(h *types.FunSym) bool { return h.Owner != g.Owner && h.Owner.IsSubclassOf(g.Owner) })
		})
		if len(gr) < 2 {
			continue
		}
		// A supertype that inherits them all has the intersection: it is
		// checked there.
		if slices.ContainsFunc(directSupers(cls), func(s *types.ClassSym) bool {
			return !slices.ContainsFunc(gr, func(g *types.FunSym) bool { return !s.IsSubclassOf(g.Owner) })
		}) {
			continue
		}
		out = append(out, gr)
	}
	return out
}

// sameJVMParams reports whether the members gr of a group, if it has a
// Java method, all take the same parameters on the JVM: a Java method
// that takes an Integer and a Kotlin function that takes an Int, an int,
// are overloads, not one member, as in K2. With jdk, a built-in collection
// type's method is a Java method.
func sameJVMParams(gr []*types.FunSym, jdk bool) bool {
	if !slices.ContainsFunc(gr, func(g *types.FunSym) bool { return isJavaClass(g.Owner) || jdk && g.Owner.Java != nil }) {
		return true
	}
	params := func(g *types.FunSym) string {
		d := jvmDesc(g)
		if jdk {
			// The JDK's generic methods take erased types, Map's
			// replace(K, V) an Object for a String: only an int and
			// an Integer differ.
			return primitiveShape(d)
		}
		return d[:strings.IndexByte(d, ')')+1]
	}
	for _, g := range gr {
		if jvmDesc(g) == "" || params(g) != params(gr[0]) {
			return false
		}
	}
	return true
}

// renamedJDK reports whether JDK method g is a member of another name, or
// a property, in Kotlin: Collection's size() is the property size, and a
// function size() of an interface isn't one member with it. As in
// kotlinc's mapped scopes (BuiltinSpecialProperties and
// BuiltinMethodsWithDifferentJvmName), and the built-in types' members.
func renamedJDK(g *types.FunSym) bool {
	switch g.Name {
	case "size", "keySet", "values", "entrySet", "length", "charAt", "name", "ordinal",
		"intValue", "longValue", "shortValue", "byteValue", "floatValue", "doubleValue", "charValue", "booleanValue",
		"getMessage", "getCause", "getLocalizedMessage", "getStackTrace":
		return true
	case "remove":
		return primitiveShape(jvmDesc(g)) == "I" // List's removeAt
	}
	return false
}

// primitiveShape returns the parameters of method descriptor d as their
// primitive types' letters, and L for any reference type: (ILjava/lang/Object;[I)V
// is "ILL".
func primitiveShape(d string) string {
	var b strings.Builder
	for i := 1; i < len(d) && d[i] != ')'; i++ {
		switch d[i] {
		case '[':
			for d[i] == '[' {
				i++
			}
			if d[i] == 'L' {
				i += strings.IndexByte(d[i:], ';')
			}
			b.WriteByte('L')
		case 'L':
			i += strings.IndexByte(d[i:], ';')
			b.WriteByte('L')
		default:
			b.WriteByte(d[i])
		}
	}
	return b.String()
}

// checkInheritedFuns checks the methods class d inherits from more than
// one supertype: that it overrides those it inherits more than one
// implementation of, or an implementation from an interface and an
// abstract member of another, and that one of their return types is a
// subtype of the others. It returns the names of those it reported.
// A class that mixes suspend and non-suspend function types, which
// checkMixedSuspendSupertypes reported, isn't reported again for their
// invoke methods.
func (c *Checker) checkInheritedFuns(d *syntax.ClassDecl, mixed bool) map[string]bool {
	cls := d.Sym
	this := cls.This()
	reported := map[string]bool{}
	for _, gr := range c.inheritedFunGroups(d, false) {
		name := gr[0].Name
		c.supertypeParamNameWarnings(d, gr)
		if !mixed && slices.ContainsFunc(gr, func(g *types.FunSym) bool { return g.Suspend != gr[0].Suspend }) {
			c.errorf(d.P, "%s inherits conflicting members '%s': a suspend function and a function that isn't", classKind(cls), name)
		}
		var impls, classImpls, classMembers int
		for _, g := range gr {
			if !g.Owner.Interface {
				classMembers++
			}
			if !g.Abstract {
				impls++
				if !g.Owner.Interface {
					classImpls++
				}
			}
		}
		switch {
		case classImpls > 0 && impls > 1:
			c.errorf(d.P, "%s must override '%s' because it inherits many implementations of it", classKind(cls), name)
			reported[name] = true
		case classImpls == 0 && impls > 0 && classMembers == 0:
			c.errorf(d.P, "%s must override '%s' because it inherits multiple interface methods of it", classKind(cls), name)
			reported[name] = true
		}
		if classMembers > 0 && classImpls == 0 {
			impls = 0 // an abstract member of a class isn't implemented by an interface's
		}
		for i := range gr[0].Params {
			n := 0
			for _, g := range gr {
				c.inheritExtDefaults(g)
				// Only a direct supertype's, as multipleDefaults counts.
				if declaresDefault(g, i) && directSupertype(cls, g.Owner) {
					n++
				}
			}
			if n > 1 {
				c.errorf(d.P, "more than one inherited member declares a default value for parameter %d of '%s': it must be overridden explicitly", i+1, name)
				break
			}
		}
		if impls == 1 {
			q := gr[slices.IndexFunc(gr, func(g *types.FunSym) bool { return !g.Abstract })]
			for _, o := range gr {
				if !o.Abstract || inheritsVisibility(q.Decl) {
					continue
				}
				if cmp, ok := compareVisibility(memberVisibility(o), memberVisibility(q)); ok && cmp > 0 {
					c.errorf(d.P, "cannot weaken access privilege '%s' for '%s' in '%s'", visibilityName(memberVisibility(q)), name, q.Owner.DisplayName())
				}
			}
		}
		// The return type of the one implementation they inherit is a
		// subtype of the others; of many, or none, one of them is.
		rets := make([]types.Type, len(gr))
		impl := -1
		for i, g := range gr {
			ensureFun(g)
			rets[i] = c.overrideSubst(this, gr[0], g).Apply(c.inheritedRetNullability(g))
			if !g.Abstract && impls == 1 {
				impl = i
			}
		}
		subOfAll := func(r types.Type) bool {
			return !slices.ContainsFunc(rets, func(o types.Type) bool { return !c.subtype(r, o) })
		}
		if impl >= 0 && !subOfAll(rets[impl]) || impl < 0 && !slices.ContainsFunc(rets, subOfAll) {
			c.errorf(d.P, "return types of inherited members '%s' are incompatible", name)
		}
	}
	c.checkJDKImpls(d, reported)
	return reported
}

// checkJDKImpls checks that class d overrides the methods it inherits an
// implementation of from the program and another from the JDK's Java
// classes, the default replace(K, V) of a java.util.Map too: class ZS :
// AbstractMutableMap<String, String>(), IS, for an IS with a
// replace(String, String) that has a body. Unlike checkInheritedFuns, it
// checks nothing else of them. A Kotlin function of an Int and a Java
// method of an Integer aren't one member (see sameJVMParams).
func (c *Checker) checkJDKImpls(d *syntax.ClassDecl, reported map[string]bool) {
	cls := d.Sym
	for _, gr := range c.inheritedFunGroups(d, true) {
		name := gr[0].Name
		if reported[name] {
			continue
		}
		impls, viaClass := 0, false
		for _, g := range gr {
			if g.Abstract {
				continue
			}
			impls++
			// A JDK interface's default method that a superclass inherits
			// is the superclass's implementation.
			viaClass = viaClass || !g.Owner.Interface || cls.Super != nil && cls.Super.IsSubclassOf(g.Owner)
		}
		if impls < 2 {
			continue
		}
		if viaClass {
			c.errorf(d.P, "%s must override '%s' because it inherits many implementations of it", classKind(cls), name)
		} else {
			c.errorf(d.P, "%s must override '%s' because it inherits multiple interface methods of it", classKind(cls), name)
		}
		reported[name] = true
	}
}

// classKind describes a class in a message: class 'A', interface 'I', or
// object.
func classKind(cls *types.ClassSym) string {
	switch {
	case cls.Anonymous:
		return "object"
	case cls.Interface:
		return "interface '" + cls.DisplayName() + "'"
	case cls.Object:
		return "object '" + cls.DisplayName() + "'"
	}
	return "class '" + cls.DisplayName() + "'"
}

// directSupers returns a class's direct supertypes: its superclass and
// interfaces.
func directSupers(cls *types.ClassSym) []*types.ClassSym {
	out := slices.Clone(cls.Interfaces)
	if cls.Super != nil {
		out = append(out, cls.Super)
	}
	return out
}

// delegatedIfaces returns the interfaces class d delegates to.
func delegatedIfaces(d *syntax.ClassDecl) []*types.ClassSym {
	var out []*types.ClassSym
	for _, st := range d.Supers {
		if st.Delegate != nil && st.T.K == types.Class {
			out = append(out, st.T.C)
		}
	}
	return out
}

// inheritedPropGroups returns the properties that class d inherits from
// more than one supertype and doesn't override itself, by name, as
// inheritedFunGroups does methods: those of the program, without context
// parameters.
func (c *Checker) inheritedPropGroups(d *syntax.ClassDecl) map[string][]superProp {
	cls := d.Sym
	delegated := delegatedIfaces(d)
	names := map[string]bool{}
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s != cls && !s.External {
			for _, p := range s.Props {
				names[p.Name] = true
			}
		}
		return true
	})
	out := map[string][]superProp{}
	for name := range names {
		if cls.Prop(name) != nil {
			continue
		}
		gr := c.superProps(cls, name)
		if len(gr) < 2 || slices.ContainsFunc(gr, func(o superProp) bool {
			return o.owner.External || len(o.ctx) > 0 || o.vis == syntax.Private || slices.ContainsFunc(delegated, func(i *types.ClassSym) bool { return i.IsSubclassOf(o.owner) })
		}) {
			continue
		}
		gr = slices.DeleteFunc(slices.Clone(gr), func(o superProp) bool {
			return slices.ContainsFunc(gr, func(h superProp) bool { return h.owner != o.owner && h.owner.IsSubclassOf(o.owner) })
		})
		if len(gr) < 2 || slices.ContainsFunc(directSupers(cls), func(s *types.ClassSym) bool {
			return !slices.ContainsFunc(gr, func(o superProp) bool { return !s.IsSubclassOf(o.owner) })
		}) {
			continue
		}
		out[name] = gr
	}
	return out
}

// checkInheritedProps checks the properties class d inherits from more
// than one supertype, as checkInheritedFuns does methods: that it
// overrides those it inherits more than one implementation of, that the
// one implementation it inherits is a var if one of them is, and is as
// visible as the others, and, in an abstract class, whose implementing
// property inheritedPropImpl doesn't check, that their types match. It
// adds the names of those it reported to reported.
func (c *Checker) checkInheritedProps(d *syntax.ClassDecl, reported map[string]bool) {
	cls := d.Sym
	this := cls.This()
	groups := c.inheritedPropGroups(d)
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		gr := groups[name]
		var impls, classImpls, classMembers, impl int
		for i, o := range gr {
			if !o.owner.Interface {
				classMembers++
			}
			if !o.abstract {
				impls++
				impl = i
				if !o.owner.Interface {
					classImpls++
				}
			}
		}
		switch {
		case classImpls > 0 && impls > 1:
			c.errorf(d.P, "%s must override '%s' because it inherits many implementations of it", classKind(cls), name)
			reported[name] = true
			continue
		case classImpls == 0 && impls > 0 && classMembers == 0:
			c.errorf(d.P, "%s must override '%s' because it inherits multiple interface methods of it", classKind(cls), name)
			reported[name] = true
			continue
		}
		if classMembers > 0 && classImpls == 0 {
			impls = 0 // an abstract member of a class isn't implemented by an interface's
		}
		// A weaker setter is only warned about (KT-66046).
		if impls == 1 {
			q := gr[impl]
			for _, o := range gr {
				if !o.abstract || q.inherits {
					continue
				}
				if cmp, ok := compareVisibility(o.vis, q.vis); ok && cmp > 0 {
					c.errorf(d.P, "cannot weaken access privilege '%s' for '%s' in '%s'", visibilityName(q.vis), name, q.owner.DisplayName())
				} else if o.mutable && q.mutable && !q.inheritsSetter && !o.inheritsSetter {
					if cmp, ok := compareVisibility(o.setterVis, q.setterVis); ok && cmp > 0 {
						c.warnf(d.P, "CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING", "cannot weaken access privilege '%s' for '%s' in '%s'", visibilityName(q.setterVis), name, q.owner.DisplayName())
					}
				}
			}
		}
		if !cls.Abstract || d.Enum {
			continue // inheritedPropImpl checks the implementation
		}
		ts := make([]types.Type, len(gr))
		for i, o := range gr {
			ts[i] = c.memberSubst(this, o.owner).Apply(o.t)
			if ts[i].K == types.Invalid {
				ts = nil
				break
			}
		}
		if ts == nil {
			continue
		}
		subOfAll := func(t types.Type) bool {
			return !slices.ContainsFunc(ts, func(o types.Type) bool { return !c.subtype(t, o) })
		}
		chosen := -1
		if impls == 1 {
			if subOfAll(ts[impl]) {
				chosen = impl
			}
		} else {
			chosen = slices.IndexFunc(ts, subOfAll)
		}
		if impls == 1 && !gr[impl].mutable && slices.ContainsFunc(gr, func(o superProp) bool { return o.mutable }) {
			c.errorf(d.P, "var '%s' is implemented by an inherited val of %s", name, gr[impl].owner.DisplayName())
			continue
		}
		mutable := slices.ContainsFunc(gr, func(o superProp) bool { return o.mutable })
		switch {
		case chosen < 0 && mutable:
			c.errorf(d.P, "the types of the inherited var '%s' don't match", name)
		case chosen < 0:
			c.errorf(d.P, "the types of the inherited property '%s' don't match", name)
		default:
			for i, o := range gr {
				if o.mutable && !c.sameType(ts[i], ts[chosen]) {
					c.errorf(d.P, "the types of the inherited var '%s' don't match", name)
					break
				}
			}
		}
	}
}

// checkDataOverrides checks the members that data class d gets: its
// componentN may override only abstract members of its supertypes, and
// its copy none, whose default values it can't redeclare.
func (c *Checker) checkDataOverrides(d *syntax.ClassDecl) {
	cls := d.Sym
	conflict, copies := false, false
	for _, name := range slices.Sorted(maps.Keys(cls.Methods)) {
		component := strings.HasPrefix(name, "component")
		if !component && name != "copy" {
			continue
		}
		for _, f := range cls.Methods[name] {
			if f.Decl != nil || f.Owner != cls {
				continue // declared by the class
			}
			for _, g := range c.overridden(cls, f) {
				conflict = conflict || component && !g.Abstract
				copies = copies || !component
			}
		}
	}
	if conflict {
		c.errorf(d.P, "data class '%s' overrides an implementation of a componentN function it generates", cls.DisplayName())
	}
	if copies {
		c.errorf(d.P, "the copy function of data class '%s' overrides a function of a supertype, whose default values it can't declare", cls.DisplayName())
	}
}

// checkDelegatedMembers checks the members that class d's delegation
// implements against the members of its supertypes they override: their
// types must have one that is a subtype of the others, which the
// delegated member's must be too, a delegated val can't implement a var,
// and only one of the direct supertypes may declare a default value.
func (c *Checker) checkDelegatedMembers(d *syntax.ClassDecl) {
	c.delegationHidesWarnings(d)
	cls := d.Sym
	this := cls.This()
	// fits returns how type t, the delegated member's, fits the others:
	// whether one of all of them is a subtype of the rest, and whether t
	// is.
	fits := func(t types.Type, ts []types.Type) (common, sub bool) {
		all := append([]types.Type{t}, ts...)
		subOfAll := func(r types.Type) bool {
			return !slices.ContainsFunc(all, func(o types.Type) bool { return !c.subtype(r, o) })
		}
		return slices.ContainsFunc(all, subOfAll), subOfAll(t)
	}
	reported := map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(cls.Methods)) {
		for _, f := range cls.Methods[name] {
			if _, ok := f.Decl.(*syntax.Delegated); !ok || f.Ext || reported[name] {
				continue
			}
			over := c.overridden(cls, f)
			for i := range f.Params {
				n := 0
				for _, o := range over {
					c.inheritExtDefaults(o)
					if declaresDefault(o, i) && directSupertype(cls, o.Owner) {
						n++
					}
				}
				if n > 1 {
					c.errorf(d.P, "more than one inherited member declares a default value for parameter %d of '%s': it must be overridden explicitly", i+1, name)
					break
				}
			}
			var ts []types.Type
			for _, o := range over {
				ensureFun(o)
				if o.Owner.External {
					ts = nil
					break
				}
				ts = append(ts, c.overrideSubst(this, f, o).Apply(o.Ret))
			}
			if len(ts) == 0 || slices.ContainsFunc(ts, func(t types.Type) bool { return t.K == types.Invalid }) {
				continue
			}
			switch common, sub := fits(f.Ret, ts); {
			case !common:
				c.errorf(d.P, "return types of inherited members '%s' are incompatible", name)
				reported[name] = true
			case !sub:
				c.errorf(d.P, "the return type of '%s', which delegation implements, isn't a subtype of the return types of the members it overrides", name)
				reported[name] = true
			}
		}
	}
	for _, p := range cls.Props {
		if _, ok := p.Decl.(*syntax.Delegated); !ok || reported["prop "+p.Name] {
			continue
		}
		var ts []types.Type
		mutable := false
		for _, o := range c.overriddenProps(cls, p) {
			if o.owner.External {
				ts = nil
				break
			}
			ts = append(ts, c.memberSubst(this, o.owner).Apply(o.t))
			mutable = mutable || o.mutable
		}
		if len(ts) == 0 || slices.ContainsFunc(ts, func(t types.Type) bool { return t.K == types.Invalid }) {
			continue
		}
		reported["prop "+p.Name] = true
		if mutable && !p.Mutable {
			c.errorf(d.P, "var '%s' is overridden by a val that delegation implements", p.Name)
			continue
		}
		switch common, sub := fits(p.Type, ts); {
		case !common:
			c.errorf(d.P, "the types of the inherited property '%s' don't match", p.Name)
		case !sub:
			c.errorf(d.P, "the type of property '%s', which delegation implements, isn't a subtype of the types of the properties it overrides", p.Name)
		}
	}
}

// inheritedRetNullability returns the result type of function g: as in
// Kotlin, a Java method's of a platform type is nullable if a Java
// method it overrides is annotated @Nullable there, JUnknownImpl's foo()
// of JNullBase's @Nullable foo().
func (c *Checker) inheritedRetNullability(g *types.FunSym) types.Type {
	r := g.Ret
	if !r.P || g.Static || g.Owner == nil || !isJavaClass(g.Owner) {
		return r
	}
	g.Owner.Supertypes(func(s *types.ClassSym) bool {
		if s == g.Owner || !isJavaClass(s) {
			return true
		}
		for _, h := range s.Methods[g.Name] {
			if h.Static || h.Private || len(h.Params) != len(g.Params) || jvmDesc(h) != jvmDesc(g) {
				continue
			}
			ensureFun(h)
			if !h.Ret.P && h.Ret.N && h.Ret.K != types.Invalid {
				r.P = false
				return false
			}
		}
		return true
	})
	return r
}
