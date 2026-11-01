package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Exposed types.
//
// A declaration may not expose a type less visible than itself in its
// signature: a public function can't return an internal class, nor a
// public class extend a private one. As kotlinc does, the checker
// compares effective visibilities: a declaration's own visibility bounded
// by those of the classes it is in, so that a public member of an
// internal class is effectively internal. They form a lattice, Kotlin's
// EffectiveVisibility, in which protected members of unrelated classes
// are incomparable, as are protected and internal ones: then the type is
// exposed too.
//
// Local classes and the members of local classes and object expressions
// aren't checked, and a type of an object expression in a signature is
// its supertype, to which Kotlin approximates it. The class path's
// classes are public, but the internal ones of a friend path (not the
// built-in types' companions, as Int.Companion, which Kotlin sees as
// public).

// effKind is a kind of effective visibility, Kotlin's but Unknown and
// Java's package-private.
type effKind uint8

const (
	evPublic effKind = iota
	evProtected
	evProtectedBound // protected in unrelated classes
	evInternal
	evInternalProtected
	evInternalProtectedBound
	evPrivateInFile
	evPrivateInClass
	evLocal
)

var effNames = [...]string{"public", "protected", "protected (in different classes)", "internal", "internal & protected",
	"internal & protected (in different classes)", "private-in-file", "private-in-class", "local"}

// effVis is an effective visibility. cls is the class whose subclasses
// see a protected one.
type effVis struct {
	k   effKind
	cls *types.ClassSym
}

func (v effVis) String() string { return effNames[v.k] }

// permissiveness is how visible one effective visibility is relative to
// another.
type permissiveness uint8

const (
	permLess permissiveness = iota
	permSame
	permMore
	permUnknown
)

// relation returns how visible v is relative to w.
func (v effVis) relation(w effVis) permissiveness {
	private := w.k == evPrivateInClass || w.k == evLocal
	switch v.k {
	case evPrivateInClass, evLocal:
		if private {
			return permSame
		}
		return permLess
	case evPrivateInFile:
		switch {
		case w.k == evPrivateInFile:
			return permSame
		case private:
			return permMore
		}
		return permLess
	case evPublic:
		if w.k == evPublic {
			return permSame
		}
		return permMore
	case evInternal:
		switch w.k {
		case evPublic:
			return permLess
		case evInternal:
			return permSame
		case evProtected, evProtectedBound:
			return permUnknown
		}
		return permMore
	case evProtected:
		switch w.k {
		case evPublic:
			return permLess
		case evProtected:
			return containerRelation(v.cls, w.cls)
		case evInternalProtected:
			// Protected is never less visible than internal & protected.
			if r := containerRelation(v.cls, w.cls); r == permSame || r == permMore {
				return permMore
			}
			return permUnknown
		case evInternal:
			return permUnknown
		}
		return permMore
	case evProtectedBound:
		switch w.k {
		case evPublic, evProtected:
			return permLess
		case evProtectedBound:
			return permSame
		case evInternal, evInternalProtected:
			return permUnknown
		}
		return permMore
	case evInternalProtected:
		switch w.k {
		case evPublic, evInternal:
			return permLess
		case evInternalProtected:
			return containerRelation(v.cls, w.cls)
		case evProtected:
			// Internal & protected is never more visible than protected.
			if r := containerRelation(v.cls, w.cls); r == permSame || r == permLess {
				return permLess
			}
			return permUnknown
		case evProtectedBound:
			return permUnknown
		}
		return permMore
	case evInternalProtectedBound:
		switch w.k {
		case evInternalProtectedBound:
			return permSame
		case evPrivateInFile, evPrivateInClass, evLocal:
			return permMore
		}
		return permLess
	}
	return permUnknown
}

// lowerBound returns the most visible effective visibility that is no
// more visible than v and w.
func (v effVis) lowerBound(w effVis) effVis {
	lower := w.k == evPrivateInClass || w.k == evPrivateInFile || w.k == evLocal || w.k == evInternalProtectedBound
	switch v.k {
	case evInternal:
		switch w.k {
		case evPublic:
			return v
		case evProtected:
			return effVis{evInternalProtected, w.cls}
		case evProtectedBound:
			return effVis{k: evInternalProtectedBound}
		}
		return w
	case evProtected:
		switch {
		case w.k == evPublic:
			return v
		case lower || w.k == evProtectedBound:
			return w
		case w.k == evInternal:
			return effVis{evInternalProtected, v.cls}
		case w.k == evInternalProtected:
			if v.relation(w) == permMore {
				return w
			}
			return effVis{k: evInternalProtectedBound}
		}
		switch v.relation(w) {
		case permSame, permLess:
			return v
		case permMore:
			return w
		}
		return effVis{k: evProtectedBound}
	case evProtectedBound:
		switch {
		case w.k == evPublic || w.k == evProtected:
			return v
		case lower || w.k == evProtectedBound:
			return w
		}
		return effVis{k: evInternalProtectedBound}
	case evInternalProtected:
		switch {
		case w.k == evPublic || w.k == evInternal:
			return v
		case lower:
			return w
		case w.k == evProtectedBound:
			return effVis{k: evInternalProtectedBound}
		}
		switch v.relation(w) {
		case permSame, permLess:
			return v
		case permMore:
			return w
		}
		return effVis{k: evInternalProtectedBound}
	}
	switch v.relation(w) {
	case permSame, permLess:
		return v
	case permMore:
		return w
	}
	return effVis{k: evPrivateInClass}
}

// containerRelation compares the protected visibilities in classes a and
// b: a subclass's is less visible.
func containerRelation(a, b *types.ClassSym) permissiveness {
	switch {
	case a == nil || b == nil:
		return permUnknown
	case a == b:
		return permSame
	case a.IsSubclassOf(b):
		return permLess
	case b.IsSubclassOf(a):
		return permMore
	}
	return permUnknown
}

// ownEffVis returns the effective visibility that visibility v gives a
// declaration in class owner, or at the top level if owner is nil,
// before the owner's is taken into account. forClass is set for classes
// and typealiases, which are private to their file at the top level.
// @PublishedApi doesn't make an internal declaration public here: only
// inline functions' checks see it so.
func ownEffVis(v syntax.Visibility, owner *types.ClassSym, forClass bool) effVis {
	switch v {
	case syntax.Private:
		if owner == nil && forClass {
			return effVis{k: evPrivateInFile}
		}
		return effVis{k: evPrivateInClass}
	case syntax.Protected:
		return effVis{evProtected, owner}
	case syntax.Internal:
		return effVis{k: evInternal}
	}
	return effVis{k: evPublic}
}

// memberEffVis returns the effective visibility of a declaration of
// visibility v in class owner, or at the top level.
func memberEffVis(v syntax.Visibility, owner *types.ClassSym, forClass bool) effVis {
	ev := ownEffVis(v, owner, forClass)
	if owner != nil {
		ev = ev.lowerBound(classEffVis(owner))
	}
	return ev
}

// overridesInternal reports whether property p of class cls, an override
// declared without a visibility, overrides an internal property, whose
// visibility it then has.
func overridesInternal(p *types.VarSym, cls *types.ClassSym) bool {
	var m *syntax.Modifiers
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		m = &d.Modifiers
	case *syntax.CtorParam:
		m = &d.Modifiers
	}
	if cls == nil || m == nil || !m.Override || m.Explicit {
		return false
	}
	found := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		switch {
		case s == cls:
		case s.External:
			kp := s.KProps[p.Name]
			found = kp != nil && kp.Getter.Module != ""
		default:
			if sp := s.Prop(p.Name); sp != nil && !sp.Private {
				v, _ := declVisibility(sp.Decl)
				found = v == syntax.Internal || sp.Module != ""
			}
		}
		return !found
	})
	return found
}

// classEffVis returns the effective visibility of a class.
func classEffVis(cls *types.ClassSym) effVis {
	switch {
	case cls.External:
		if cls.Internal && !builtinCompanions[cls.Name] {
			return effVis{k: evInternal}
		}
		return effVis{k: evPublic}
	case cls.Local || cls.Anonymous:
		return effVis{k: evLocal}
	}
	d, ok := cls.Decl.(*syntax.ClassDecl)
	if !ok {
		return effVis{k: evPublic}
	}
	return memberEffVis(d.Visibility, cls.Outer, true)
}

// declaredVisibility returns the visibility a class is declared with.
func declaredVisibility(cls *types.ClassSym) syntax.Visibility {
	if cls.External {
		if cls.Internal && !builtinCompanions[cls.Name] {
			return syntax.Internal
		}
		return syntax.Public
	}
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok {
		return d.Visibility
	}
	return syntax.Public
}

// exposure is a class that makes a type less visible than a declaration.
type exposure struct {
	cls *types.ClassSym
	vis effVis
	arg bool // the class is a type argument's
}

// exposedIn returns the class that makes type t less visible than base,
// or nil. Type parameters don't expose anything: their bounds are checked
// where they are declared.
func exposedIn(t types.Type, base effVis, seen map[types.Type]bool) *exposure {
	if t.K != types.Class || seen[t] {
		return nil
	}
	seen[t] = true
	cls := t.C
	if cls.Anonymous {
		return exposedIn(approx(t), base, seen)
	}
	if localClass(cls) {
		return nil
	}
	vis := classEffVis(cls)
	if r := vis.relation(base); r == permLess || r == permUnknown {
		return &exposure{cls, vis, len(seen) > 1}
	}
	if t.A.Len() == 0 {
		return nil
	}
	if cls.External {
		cls.Ensure()
	}
	for i, a := range t.A.List {
		if a.V != types.Star {
			if e := exposedIn(a.T, base, seen); e != nil {
				return e
			}
			continue
		}
		if i >= len(cls.TypeParams) {
			continue
		}
		for _, b := range cls.TypeParams[i].Bounds {
			if e := exposedIn(b, base, seen); e != nil {
				return e
			}
		}
	}
	return nil
}

// localClass reports whether a class is local or in a local class or an
// object expression.
func localClass(cls *types.ClassSym) bool {
	for k := cls; k != nil; k = k.Outer {
		if k.Local || k.Anonymous {
			return true
		}
	}
	return false
}

// exposedScope is the annotations of the declarations a check is in,
// innermost last, whose @Suppress may name the error: kotlinc reports it
// suppressed as a warning.
type exposedScope [][]syntax.Annotation

func (s exposedScope) with(anns []syntax.Annotation) exposedScope {
	return append(s[:len(s):len(s)], anns)
}

// checkExposure reports the types that the file's declarations expose.
// It runs once their omitted types are inferred.
func (c *Checker) checkExposure() {
	s := exposedScope{c.file.Annotations}
	for _, d := range c.file.Decls {
		c.exposedDecl(d, nil, s)
	}
}

// exposedDecl checks a declaration in class owner, or at the top level.
func (c *Checker) exposedDecl(d syntax.Decl, owner *types.ClassSym, s exposedScope) {
	switch d := d.(type) {
	case *syntax.ClassDecl:
		if !d.Synthetic && d.Sym != nil {
			c.exposedClass(d, s.with(d.Annotations))
		}
	case *syntax.FunDecl:
		if !d.Plugin && d.Sym != nil && d.AccessorOf == nil {
			c.exposedFun(d, owner, s.with(d.Annotations))
		}
	case *syntax.PropDecl:
		if !d.Plugin && d.Sym != nil {
			c.exposedProp(d, owner, s.with(d.Annotations))
		}
	case *syntax.TypeAliasDecl:
		if d.Sym == nil || !d.Sym.Resolved {
			return
		}
		s = s.with(d.Annotations)
		vis := memberEffVis(d.Visibility, owner, true)
		c.exposedBounds(d.TypeParams, vis, s)
		c.exposed(d.Sym.Type, vis, d.P, "EXPOSED_TYPEALIAS_EXPANDED_TYPE", s)
	}
}

func (c *Checker) exposedClass(d *syntax.ClassDecl, s exposedScope) {
	cls := d.Sym
	if localClass(cls) {
		return
	}
	vis := classEffVis(cls)
	for _, st := range d.Supers {
		if st.T.K != types.Class || st.Type == nil {
			continue
		}
		sc := st.T.C
		if sc.External {
			sc.Ensure()
		}
		// A class may implement a less visible interface.
		if sc.Interface != d.Interface {
			continue
		}
		if d.Interface {
			c.exposed(st.T, vis, st.Type.P, "EXPOSED_SUPER_INTERFACE", s)
		} else {
			c.exposed(st.T, vis, st.Type.P, "EXPOSED_SUPER_CLASS", s)
		}
	}
	c.exposedBounds(d.TypeParams, vis, s)
	ctor := func(m *syntax.Modifiers) effVis {
		if cls.Sealed || d.Enum {
			return effVis{k: evPrivateInClass}
		}
		return memberEffVis(m.Visibility, cls, false)
	}
	if len(d.CtorParams) > 0 {
		cv := ctor(&d.CtorMods)
		cs := s.with(d.CtorMods.Annotations)
		for _, cp := range d.CtorParams {
			if cp.Sym == nil {
				continue
			}
			ps := cs.with(cp.Modifiers.Annotations)
			if c.exposed(cp.Sym.Type, cv, paramPos(cp.P, cp.Modifiers.Annotations), "EXPOSED_PARAMETER_TYPE", ps) || !cp.IsProp || cp.PropSym == nil {
				continue
			}
			c.exposed(cp.PropSym.Type, propEffVis(cp.PropSym, cls), cp.P, "EXPOSED_PROPERTY_TYPE_IN_CONSTRUCTOR_ERROR", ps)
		}
	}
	for _, m := range d.Members {
		if sc, ok := m.(*syntax.SecondaryCtor); ok {
			if sc.Serial == nil {
				c.exposedParams(sc.Params, ctor(&sc.Modifiers), s.with(sc.Annotations))
			}
			continue
		}
		c.exposedDecl(m, cls, s)
	}
}

func (c *Checker) exposedFun(d *syntax.FunDecl, owner *types.ClassSym, s exposedScope) {
	sym := d.Sym
	v, _ := funVisibility(sym)
	if !d.Explicit && sym.Inherited == types.InheritInternal {
		v = syntax.Internal
	}
	vis := memberEffVis(v, owner, false)
	if sym.Resolved || sym.Infer == nil {
		c.exposed(sym.Ret, vis, d.P, "EXPOSED_FUNCTION_RETURN_TYPE", s)
	}
	c.exposedParams(d.Params, vis, s)
	c.exposedParams(d.Contexts, vis, s)
	if d.Recv != nil {
		c.exposed(sym.Recv, vis, d.Recv.P, "EXPOSED_RECEIVER_TYPE", s)
	}
	c.exposedBounds(d.TypeParams, vis, s)
}

func (c *Checker) exposedProp(d *syntax.PropDecl, owner *types.ClassSym, s exposedScope) {
	sym := d.Sym
	vis := propEffVis(sym, owner)
	if sym.Resolved || sym.Infer == nil {
		c.exposed(sym.Type, vis, d.P, "EXPOSED_PROPERTY_TYPE", s)
	}
	if d.Recv != nil && sym.Getter != nil {
		c.exposed(sym.Getter.Recv, vis, d.Recv.P, "EXPOSED_RECEIVER_TYPE", s)
	}
	c.exposedBounds(d.TypeParams, vis, s)
	c.exposedParams(d.Contexts, vis, s)
}

// propEffVis returns the effective visibility of property p of class
// owner, or at the top level.
func propEffVis(p *types.VarSym, owner *types.ClassSym) effVis {
	v, _ := propVisibility(p)
	if v == syntax.Public && overridesInternal(p, owner) {
		v = syntax.Internal
	}
	return memberEffVis(v, owner, false)
}

func (c *Checker) exposedParams(ps []*syntax.Param, vis effVis, s exposedScope) {
	for _, p := range ps {
		if p.Sym != nil {
			c.exposed(p.Sym.Type, vis, paramPos(p.P, p.Annotations), "EXPOSED_PARAMETER_TYPE", s.with(p.Annotations))
		}
	}
}

// paramPos returns where a parameter at p with annotations anns starts,
// where kotlinc reports its type.
func paramPos(p syntax.Pos, anns []syntax.Annotation) syntax.Pos {
	if len(anns) > 0 && !anns[0].Implicit {
		return anns[0].P
	}
	return p
}

func (c *Checker) exposedBounds(tps []*syntax.TypeParam, vis effVis, s exposedScope) {
	for _, tp := range tps {
		if tp.Sym == nil {
			continue
		}
		for i, b := range tp.Sym.Bounds {
			p := tp.P
			if i < len(tp.Bounds) {
				p = tp.Bounds[i].P
			}
			c.exposed(b, vis, p, "EXPOSED_TYPE_PARAMETER_BOUND", s)
		}
	}
}

// exposedMessages are the declarations and the types their errors name,
// by kotlinc's diagnostics.
var exposedMessages = map[string][2]string{
	"EXPOSED_SUPER_INTERFACE":                    {"sub-interface", "supertype"},
	"EXPOSED_SUPER_CLASS":                        {"subclass", "supertype"},
	"EXPOSED_TYPE_PARAMETER_BOUND":               {"generic", "parameter bound type"},
	"EXPOSED_FUNCTION_RETURN_TYPE":               {"function", "return type"},
	"EXPOSED_PARAMETER_TYPE":                     {"function", "parameter type"},
	"EXPOSED_RECEIVER_TYPE":                      {"member", "receiver type"},
	"EXPOSED_PROPERTY_TYPE":                      {"property", "type"},
	"EXPOSED_PROPERTY_TYPE_IN_CONSTRUCTOR_ERROR": {"property", "type"},
	"EXPOSED_TYPEALIAS_EXPANDED_TYPE":            {"typealias", "expanded type"},
}

// exposed reports at p that a declaration of effective visibility vis
// exposes its type t, if t is less visible, unless the declarations of s
// suppress diag, kotlinc's name of the error. It reports whether t is
// exposed.
func (c *Checker) exposed(t types.Type, vis effVis, p syntax.Pos, diag string, s exposedScope) bool {
	if vis.k == evLocal {
		return false
	}
	e := exposedIn(t, vis, map[types.Type]bool{})
	if e == nil {
		return false
	}
	for _, anns := range s {
		if suppresses(anns, diag) {
			return true
		}
	}
	// As kotlinc, name the class that the type's is less visible for:
	// the one it is in, if it is declared more visible.
	cls, rel := e.cls, ""
	low := declaredVisibility(cls)
	for k := cls.Outer; k != nil && visRank(low) != effRank(e.vis); k = k.Outer {
		if kv := declaredVisibility(k); visRank(kv) < visRank(low) {
			cls, low = k, kv
		}
	}
	if e.arg {
		rel = " argument"
	}
	if cls != e.cls {
		rel += " containing declaration"
	}
	m := exposedMessages[diag]
	c.errorf(p, "'%s' %s exposes its '%s' %s%s '%s'", vis, m[0], e.vis, m[1], rel, cls.DisplayName())
	return true
}

// suppresses reports whether annotations anns have a @Suppress of
// diagnostic name, on the declaration, its file or, for a parameter, the
// parameter: not on another use-site target, as @property: on a
// constructor's property.
func suppresses(anns []syntax.Annotation, name string) bool {
	for i := range anns {
		a := &anns[i]
		if !a.Is("Suppress") || a.Target != "" && a.Target != "file" && a.Target != "param" {
			continue
		}
		for _, arg := range a.Args {
			for _, t := range arg.Toks {
				for _, part := range t.Parts {
					if part.Lit == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// visRank orders visibilities, protected and internal alike.
func visRank(v syntax.Visibility) int {
	switch v {
	case syntax.Private:
		return 0
	case syntax.Public:
		return 2
	}
	return 1
}

// effRank is visRank of the visibility an effective visibility stands
// for.
func effRank(v effVis) int {
	switch v.k {
	case evPublic:
		return 2
	case evProtected, evProtectedBound, evInternal:
		return 1
	}
	return 0
}
