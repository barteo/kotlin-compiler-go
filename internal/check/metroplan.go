package check

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Metro's annotations, by internal name.
const (
	mInject             = "dev/zacsweers/metro/Inject"
	mAssistedInject     = "dev/zacsweers/metro/AssistedInject"
	mAssisted           = "dev/zacsweers/metro/Assisted"
	mAssistedFactory    = "dev/zacsweers/metro/AssistedFactory"
	mProvides           = "dev/zacsweers/metro/Provides"
	mBinds              = "dev/zacsweers/metro/Binds"
	mIncludes           = "dev/zacsweers/metro/Includes"
	mIntoSet            = "dev/zacsweers/metro/IntoSet"
	mIntoMap            = "dev/zacsweers/metro/IntoMap"
	mElementsIntoSet    = "dev/zacsweers/metro/ElementsIntoSet"
	mMultibinds         = "dev/zacsweers/metro/Multibinds"
	mBindingContainer   = "dev/zacsweers/metro/BindingContainer"
	mDependencyGraph    = "dev/zacsweers/metro/DependencyGraph"
	mGraphFactory       = "dev/zacsweers/metro/DependencyGraph$Factory"
	mGraphExtension     = "dev/zacsweers/metro/GraphExtension"
	mContributesTo      = "dev/zacsweers/metro/ContributesTo"
	mContributesBinding = "dev/zacsweers/metro/ContributesBinding"
	mContributesIntoSet = "dev/zacsweers/metro/ContributesIntoSet"
	mContributesIntoMap = "dev/zacsweers/metro/ContributesIntoMap"
	mContribution       = "dev/zacsweers/metro/internal/MetroContribution"
	mQualifier          = "dev/zacsweers/metro/Qualifier"
	mScope              = "dev/zacsweers/metro/Scope"
	mMapKey             = "dev/zacsweers/metro/MapKey"
	mSingleIn           = "dev/zacsweers/metro/SingleIn"
	mProvider           = "dev/zacsweers/metro/Provider"
	mNothing            = "kotlin/Nothing"
)

// metroAnalysis computes the plan of PlanMetro.
type metroAnalysis struct {
	u         *Universe
	checkers  []*Checker
	checkerOf map[*types.ClassSym]*Checker
	plan      *MetroPlan
	errs      []*syntax.Error

	// contributesAsInject and functionProviders are Metro's options of
	// those names.
	contributesAsInject, functionProviders bool

	classfiles map[string]*classfile.Annotated
	classAnns  map[*types.ClassSym][]*mAnn
	metas      map[string]mMeta
}

// mMeta is what an annotation class is to Metro.
type mMeta struct {
	qualifier, scope, mapKey bool
	implicitClassKey         bool
}

func newMetroAnalysis(u *Universe, checkers []*Checker) *metroAnalysis {
	m := &metroAnalysis{u: u, checkers: checkers, checkerOf: map[*types.ClassSym]*Checker{},
		plan:                &MetroPlan{members: map[string][]string{}, supers: map[string][]string{}, calls: map[string]map[syntax.Pos]string{}},
		contributesAsInject: true, functionProviders: true,
		classfiles: map[string]*classfile.Annotated{}, classAnns: map[*types.ClassSym][]*mAnn{}, metas: map[string]mMeta{}}
	for _, c := range checkers {
		for _, d := range c.classes {
			if d.Sym != nil {
				m.checkerOf[d.Sym] = c
			}
		}
	}
	return m
}

func (m *metroAnalysis) errorf(c *Checker, p syntax.Pos, format string, args ...any) {
	m.errs = append(m.errs, &syntax.Error{File: c.file.Path, Pos: p, Msg: fmt.Sprintf(format, args...)})
}

func (m *metroAnalysis) unsupported(c *Checker, p syntax.Pos, format string, args ...any) {
	m.errs = append(m.errs, &syntax.Error{File: c.file.Path, Pos: p, Msg: fmt.Sprintf(format, args...), Feature: "metro"})
}

func (m *metroAnalysis) run() {
	for _, c := range m.checkers {
		for _, d := range c.classes {
			if d.Sym == nil || d.Sym.Local || d.Sym.Anonymous {
				continue
			}
			m.contributions(c, d)
		}
	}
	for _, c := range m.checkers {
		for _, d := range c.classes {
			if d.Sym == nil || d.Sym.Local || d.Sym.Anonymous {
				continue
			}
			for _, a := range m.annsOf(d.Sym) {
				switch a.cls {
				case mDependencyGraph:
					m.graph(c, d, a)
				case mGraphExtension:
					m.unsupported(c, d.P, "Metro graph extensions (@GraphExtension) are not supported")
				}
			}
		}
	}
	for _, c := range m.checkers {
		m.graphCalls(c)
	}
}

// class returns the class with an internal name, the program's or the
// class path's, or nil.
func (m *metroAnalysis) class(name string) *types.ClassSym {
	if cls := m.u.sourceClass(name); cls != nil {
		return cls
	}
	return m.u.ext.class(name)
}

func isSource(cls *types.ClassSym) bool {
	_, ok := cls.Decl.(*syntax.ClassDecl)
	return ok && !cls.External
}

// classfile returns the annotations of a class file of the class path.
func (m *metroAnalysis) classfile(name string) *classfile.Annotated {
	if a, ok := m.classfiles[name]; ok {
		return a
	}
	var a *classfile.Annotated
	if cp := m.u.ext.cp; cp != nil && cp.Has(name) {
		if b, err := cp.Bytes(name); err == nil {
			a, _ = classfile.ParseAnnotated(b)
		}
	}
	m.classfiles[name] = a
	return a
}

// annsOf returns a class's annotations.
func (m *metroAnalysis) annsOf(cls *types.ClassSym) []*mAnn {
	if as, ok := m.classAnns[cls]; ok {
		return as
	}
	var as []*mAnn
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok && !cls.External {
		if c := m.checkerOf[cls]; c != nil {
			as = c.fromSource(d.Annotations, cls)
		}
	} else if cf := m.classfile(cls.Name); cf != nil {
		for _, a := range cf.Class {
			as = append(as, fromClassfile(a))
		}
		as = expandContainers(as)
	}
	m.classAnns[cls] = as
	return as
}

func find(as []*mAnn, name string) *mAnn {
	for _, a := range as {
		if a.cls == name {
			return a
		}
	}
	return nil
}

// meta returns what an annotation class is to Metro.
func (m *metroAnalysis) meta(name string) mMeta {
	if v, ok := m.metas[name]; ok {
		return v
	}
	var v mMeta
	m.metas[name] = v
	if cls := m.class(name); cls != nil {
		for _, a := range m.annsOf(cls) {
			switch a.cls {
			case mQualifier, "javax/inject/Qualifier", "jakarta/inject/Qualifier":
				v.qualifier = true
			case mScope, "javax/inject/Scope", "jakarta/inject/Scope":
				v.scope = true
			case mMapKey:
				v.mapKey = true
				if ik, ok := a.args["implicitClassKey"]; ok && ik.num != 0 {
					v.implicitClassKey = true
				}
				if uw, ok := a.args["unwrapValue"]; ok && uw.num == 0 {
					v.mapKey = false // a key that is the annotation itself
				}
			}
		}
	}
	m.metas[name] = v
	return v
}

// qualifier returns the key of the qualifier among annotations, or "".
func (m *metroAnalysis) qualifier(as []*mAnn) string {
	for _, a := range as {
		if m.meta(a.cls).qualifier {
			return a.key()
		}
	}
	return ""
}

func (m *metroAnalysis) qualifierAnn(as []*mAnn) *mAnn {
	for _, a := range as {
		if m.meta(a.cls).qualifier {
			return a
		}
	}
	return nil
}

// scopesOf returns the keys of the scope annotations among annotations.
func (m *metroAnalysis) scopesOf(as []*mAnn) []string {
	var out []string
	for _, a := range as {
		if m.meta(a.cls).scope {
			out = append(out, a.key())
		}
	}
	return out
}

func (m *metroAnalysis) mapKeyAnn(as []*mAnn) *mAnn {
	for _, a := range as {
		if m.meta(a.cls).mapKey {
			return a
		}
	}
	return nil
}

// singleIn returns the key of @SingleIn(scope::class).
func singleIn(scope string) string {
	return (&mAnn{cls: mSingleIn, args: map[string]mVal{"scope": {tag: 'c', class: scope}}}).key()
}

// mMember is a function or a property of a class, as Metro sees it.
type mMember struct {
	name      string
	owner     *types.ClassSym
	prop      bool
	mutable   bool
	ret       types.Type
	params    []*types.VarSym
	recv      *types.Type // an extension's receiver
	abstract  bool
	anns      []*mAnn
	paramAnns [][]*mAnn // the parameters' annotations
	recvAnns  []*mAnn
	defaults  []bool // which parameters declare default values
}

// declared returns the members a class declares.
func (m *metroAnalysis) declared(cls *types.ClassSym) []*mMember {
	if cls == nil {
		return nil
	}
	cls.Ensure()
	var out []*mMember
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok && !cls.External {
		c := m.checkerOf[cls]
		if c == nil {
			return nil
		}
		for _, md := range d.Members {
			switch md := md.(type) {
			case *syntax.FunDecl:
				f := md.Sym
				if f == nil || f.Static {
					continue
				}
				mm := &mMember{name: f.Name, owner: cls, ret: f.Ret, params: f.Params, abstract: f.Abstract, anns: c.fromSource(md.Annotations, cls)}
				if md.Recv != nil && md.RecvSym != nil {
					t := md.RecvSym.Type
					mm.recv = &t
					mm.recvAnns = c.fromSource(md.RecvAnnotations, cls)
				}
				for _, p := range md.Params {
					mm.paramAnns = append(mm.paramAnns, c.fromSource(p.Annotations, cls))
					mm.defaults = append(mm.defaults, p.Default != nil)
				}
				out = append(out, mm)
			case *syntax.PropDecl:
				v := md.Sym
				if v == nil {
					continue
				}
				mm := &mMember{name: v.Name, owner: cls, prop: true, mutable: v.Mutable, ret: v.Type, abstract: v.Abstract, anns: c.fromSource(md.Annotations, cls)}
				if md.Getter != nil {
					mm.anns = append(mm.anns, c.fromSource(md.Getter.Annotations, cls)...)
				}
				if md.Recv != nil {
					t := c.metroResolve(md.Recv, cls)
					mm.recv = &t
					mm.recvAnns = c.fromSource(md.RecvAnnotations, cls)
				}
				out = append(out, mm)
			}
		}
		return out
	}
	cf := m.classfile(cls.Name)
	methodAnns := func(names ...string) (*classfile.AnnotatedMethod, []*mAnn) {
		var first *classfile.AnnotatedMethod
		var as []*mAnn
		for _, owner := range []*classfile.Annotated{cf, m.classfile(cls.Name + "$DefaultImpls")} {
			if owner == nil {
				continue
			}
			for i := range owner.Methods {
				am := &owner.Methods[i]
				for _, n := range names {
					if am.Name+am.Desc == n || strings.HasSuffix(n, "$annotations") && am.Name == n {
						if first == nil && !strings.HasSuffix(n, "$annotations") {
							first = am
						}
						for _, a := range am.Annots {
							as = append(as, fromClassfile(a))
						}
					}
				}
			}
		}
		return first, expandContainers(as)
	}
	for _, name := range slices.Sorted(maps.Keys(cls.Methods)) {
		for _, f := range cls.Methods[name] {
			if f.Static || f.Owner != cls {
				continue
			}
			am, as := methodAnns(f.JVM() + types.FunDesc(f))
			mm := &mMember{name: f.Name, owner: cls, ret: f.Ret, params: f.Params, abstract: f.Abstract, anns: as}
			off := 0
			if f.Ext {
				t := f.Recv
				mm.recv = &t
				off = 1
				if am != nil && len(am.Params) > 0 {
					mm.recvAnns = convertAll(am.Params[0])
				}
			}
			for i, p := range f.Params {
				var pa []*mAnn
				if am != nil && i+off < len(am.Params) {
					pa = convertAll(am.Params[i+off])
				}
				mm.paramAnns = append(mm.paramAnns, pa)
				mm.defaults = append(mm.defaults, p.Default)
			}
			out = append(out, mm)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cls.KProps)) {
		kp := cls.KProps[name]
		g := kp.Getter
		if g == nil || g.Owner != cls {
			continue
		}
		_, as := methodAnns(g.JVM()+types.FunDesc(g), g.JVM()+"$annotations")
		mm := &mMember{name: name, owner: cls, prop: true, mutable: kp.Setter != nil, ret: g.Ret, abstract: g.Abstract, anns: as}
		if g.Ext {
			t := g.Recv
			mm.recv = &t
		}
		out = append(out, mm)
	}
	return out
}

func convertAll(as []classfile.Annotation) []*mAnn {
	var out []*mAnn
	for _, a := range as {
		out = append(out, fromClassfile(a))
	}
	return expandContainers(out)
}

// metroResolve resolves a type in the scope of class cls.
func (c *Checker) metroResolve(ref *syntax.TypeRef, cls *types.ClassSym) types.Type {
	saved := c.lookupCls
	c.lookupCls = cls
	defer func() { c.lookupCls = saved }()
	return c.resolveType(ref)
}

// inherited returns the members of a class and its supertypes, the
// first of each name and kind; a member is abstract only if every one
// of that name is.
func (m *metroAnalysis) inherited(cls *types.ClassSym) []*mMember {
	var out []*mMember
	index := map[string]*mMember{}
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s.Name == "java/lang/Object" || s.KotlinName == "kotlin/Any" {
			return true
		}
		for _, mm := range m.declared(s) {
			k := fmt.Sprintf("%s/%v/%d/%v", mm.name, mm.prop, len(mm.params), mm.recv != nil)
			if prev, ok := index[k]; ok {
				prev.abstract = prev.abstract && mm.abstract
				if len(prev.anns) == 0 {
					prev.anns = mm.anns
				}
				continue
			}
			cp := *mm
			index[k] = &cp
			out = append(out, &cp)
		}
		return true
	})
	return out
}

// ctorOf returns the constructor Metro injects of a class, its
// parameters' annotations and whether it is an assisted one, or nil if
// the class isn't injected.
func (m *metroAnalysis) ctorOf(cls *types.ClassSym) (*types.FunSym, [][]*mAnn, []bool, bool) {
	anns := m.annsOf(cls)
	classInject := find(anns, mInject) != nil || find(anns, "javax/inject/Inject") != nil
	assisted := find(anns, mAssistedInject) != nil
	implicit := m.contributesAsInject && (find(anns, mContributesBinding) != nil || find(anns, mContributesIntoSet) != nil || find(anns, mContributesIntoMap) != nil)
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok && !cls.External {
		c := m.checkerOf[cls]
		paramsOf := func(ps []*syntax.Param) ([][]*mAnn, []bool) {
			var pa [][]*mAnn
			var defs []bool
			for _, p := range ps {
				pa = append(pa, c.fromSource(p.Annotations, cls))
				defs = append(defs, p.Default != nil)
			}
			return pa, defs
		}
		ctorAnns := c.fromSource(d.CtorMods.Annotations, cls)
		if d.HasPrimary && (classInject || assisted || implicit || find(ctorAnns, mInject) != nil || find(ctorAnns, mAssistedInject) != nil) {
			var pa [][]*mAnn
			var defs []bool
			for _, p := range d.CtorParams {
				pa = append(pa, c.fromSource(p.Modifiers.Annotations, cls))
				defs = append(defs, p.Default != nil)
			}
			return d.Sym.Ctor, pa, defs, assisted || find(ctorAnns, mAssistedInject) != nil
		}
		for _, md := range d.Members {
			if sc, ok := md.(*syntax.SecondaryCtor); ok && sc.Sym != nil {
				as := c.fromSource(sc.Annotations, cls)
				if find(as, mInject) != nil || find(as, mAssistedInject) != nil {
					pa, defs := paramsOf(sc.Params)
					return sc.Sym, pa, defs, find(as, mAssistedInject) != nil
				}
			}
		}
		return nil, nil, nil, false
	}
	cls.Ensure()
	cf := m.classfile(cls.Name)
	if cf == nil {
		return nil, nil, nil, false
	}
	var pick *types.FunSym
	var pickAnns [][]*mAnn
	var pickAssisted bool
	for _, f := range cls.Ctors {
		desc := types.FunDesc(f)
		for i := range cf.Methods {
			am := &cf.Methods[i]
			if am.Name != "<init>" || am.Desc != desc {
				continue
			}
			as := convertAll(am.Annots)
			explicit := find(as, mInject) != nil || find(as, mAssistedInject) != nil || find(as, "javax/inject/Inject") != nil
			if explicit || (classInject || assisted || implicit) && (pick == nil || len(cls.Ctors) == 1) {
				pick = f
				pickAnns = nil
				for j := range f.Params {
					var pa []*mAnn
					if j < len(am.Params) {
						pa = convertAll(am.Params[j])
					}
					pickAnns = append(pickAnns, pa)
				}
				pickAssisted = assisted || find(as, mAssistedInject) != nil
				if explicit {
					var defs []bool
					for _, p := range f.Params {
						defs = append(defs, p.Default)
					}
					return pick, pickAnns, defs, pickAssisted
				}
			}
		}
	}
	if pick == nil {
		return nil, nil, nil, false
	}
	var defs []bool
	for _, p := range pick.Params {
		defs = append(defs, p.Default)
	}
	return pick, pickAnns, defs, pickAssisted
}

// contributions writes what a class contributing to a scope adds to the
// module: for each scope a nested interface of @Binds functions that bind
// it, for a graph to find, and the hint that finds the class.
func (m *metroAnalysis) contributions(c *Checker, d *syntax.ClassDecl) {
	cls := d.Sym
	anns := m.annsOf(cls)
	scopes := map[string]bool{}
	byScope := map[string][]string{}
	var order []string
	for _, a := range anns {
		switch a.cls {
		case mContributesTo, mContributesBinding, mContributesIntoSet, mContributesIntoMap:
		default:
			continue
		}
		scope := a.args["scope"].class
		if scope == "" {
			continue
		}
		if !scopes[scope] {
			scopes[scope] = true
			order = append(order, scope)
		}
		if a.cls == mContributesTo {
			continue
		}
		fn, err := m.bindsFor(c, cls, a, anns)
		if err != nil {
			m.unsupported(c, d.P, "%v", err)
			continue
		}
		byScope[scope] = append(byScope[scope], fn)
	}
	for _, scope := range order {
		sc := m.class(scope)
		if sc == nil {
			m.errorf(c, d.P, "unresolved Metro scope %s", scope)
			continue
		}
		if fns := byScope[scope]; len(fns) > 0 {
			var b strings.Builder
			fmt.Fprintf(&b, "@dev.zacsweers.metro.internal.MetroContribution(scope = %s::class)\n@dev.zacsweers.metro.BindingContainer\n", metroClassName(sc))
			fmt.Fprintf(&b, "interface %s {\n%s\n}\n", contributionName(sc), strings.Join(fns, "\n"))
			m.plan.members[cls.Name] = append(m.plan.members[cls.Name], b.String())
		}
		m.hint(cls, sc)
	}
}

// contributionName is the name of a contributing class's nested
// interface for a scope, MetroContributionTo and the scope's qualified
// name.
func contributionName(scope *types.ClassSym) string {
	parts := strings.Split(fqName(scope), ".")
	for i, p := range parts {
		if i == len(parts)-1 {
			parts[i] = p
		} else {
			parts[i] = strings.ToLower(p[:1]) + p[1:]
		}
	}
	s := strings.Join(parts, "")
	return "MetroContributionTo" + strings.ToUpper(s[:1]) + s[1:]
}

// hint writes the hint of a class's contribution to a scope: a function
// in package metro.hints, named after the scope, that takes the class.
func (m *metroAnalysis) hint(cls, scope *types.ClassSym) {
	fn := strings.ReplaceAll(fqName(scope), ".", "_")
	var name strings.Builder
	for _, p := range strings.Split(fqName(cls), ".") {
		name.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	name.WriteString(strings.ToUpper(fn[:1]) + fn[1:])
	facade := name.String() + "Kt"
	src := fmt.Sprintf("@file:JvmName(%q)\npackage metro.hints\n\n@kotlin.Deprecated(\"This synthesized declaration should not be used directly\", level = kotlin.DeprecationLevel.HIDDEN)\nfun %s(contributed: %s) {}\n",
		facade, fn, metroClassName(cls))
	m.plan.Files = append(m.plan.Files, MetroFile{Path: "metro/hints/" + facade + ".kt", Src: src})
}

// bindsFor writes the @Binds function of a class's @ContributesBinding,
// @ContributesIntoSet or @ContributesIntoMap annotation a.
func (m *metroAnalysis) bindsFor(c *Checker, cls *types.ClassSym, a *mAnn, anns []*mAnn) (string, error) {
	bound, err := m.boundType(cls, a)
	if err != nil {
		return "", err
	}
	ts, ok := kotlinTypeSrc(bound)
	if !ok {
		return "", fmt.Errorf("kc can't name the bound type %s of %s", bound, cls.DisplayName())
	}
	var lines []string
	lines = append(lines, "@dev.zacsweers.metro.Binds")
	prefix := "bindsAs"
	switch a.cls {
	case mContributesIntoSet:
		lines = append(lines, "@dev.zacsweers.metro.IntoSet")
		prefix = "bindIntoSetAs"
	case mContributesIntoMap:
		lines = append(lines, "@dev.zacsweers.metro.IntoMap")
		prefix = "bindIntoMapAs"
		key := m.mapKeyAnn(anns)
		if key == nil {
			return "", fmt.Errorf("@ContributesIntoMap needs a map key annotation on %s", cls.DisplayName())
		}
		key = m.implicitKey(key, cls)
		s, err := m.kotlinAnnSrc(key)
		if err != nil {
			return "", err
		}
		lines = append(lines, s)
	}
	if q := m.qualifierAnn(anns); q != nil {
		s, err := m.kotlinAnnSrc(q)
		if err != nil {
			return "", err
		}
		lines = append(lines, s)
	}
	name := prefix + strings.ReplaceAll(strings.ReplaceAll(bound.C.DisplayName(), ".", ""), "`", "")
	lines = append(lines, fmt.Sprintf("fun %s(instance: %s): %s = throw java.lang.IllegalStateException(\"Never called\")", name, metroClassName(cls), ts))
	return strings.Join(lines, "\n"), nil
}

// implicitKey returns a map key annotation with its value, the class, if
// it is an implicit class key without one.
func (m *metroAnalysis) implicitKey(key *mAnn, cls *types.ClassSym) *mAnn {
	if !m.meta(key.cls).implicitClassKey {
		return key
	}
	if v, ok := key.args["value"]; ok && v.class != mNothing {
		return key
	}
	k := &mAnn{cls: key.cls, args: map[string]mVal{"value": {tag: 'c', class: cls.Name}}}
	return k
}

// boundType returns the type a contribution annotation binds a class as.
func (m *metroAnalysis) boundType(cls *types.ClassSym, a *mAnn) (types.Type, error) {
	if b, ok := a.args["binding"]; ok && b.ann != nil && len(b.ann.typeArgs) == 1 && b.ann.typeArgs[0].K != types.Nothing {
		t := b.ann.typeArgs[0]
		if t.K != types.Class {
			return t, fmt.Errorf("kc can't bind %s as %s", cls.DisplayName(), t)
		}
		return t, nil
	}
	var supers []types.Type
	for _, s := range cls.Supers {
		if s.K == types.Class && s.C.Name != "java/lang/Object" && s.C.KotlinName != "kotlin/Any" {
			supers = append(supers, s)
		}
	}
	if len(supers) == 0 {
		if cls.Super != nil && cls.Super.Name != "java/lang/Object" {
			supers = append(supers, types.ClassType(cls.Super))
		}
		for _, i := range cls.Interfaces {
			supers = append(supers, types.ClassType(i))
		}
	}
	if len(supers) != 1 {
		return types.InvalidType, fmt.Errorf("%s has %d supertypes: its contribution needs binding = binding<T>() to name the bound type", cls.DisplayName(), len(supers))
	}
	return supers[0], nil
}

// graphCalls notes the calls of createGraph and createGraphFactory in a
// file, which metroInsert replaces.
func (m *metroAnalysis) graphCalls(c *Checker) {
	type site struct {
		x   *syntax.Call
		cls *types.ClassSym
	}
	var sites []site
	metroCallsIn(c.file, func(x *syntax.Call, outer *syntax.ClassDecl) {
		var scope *types.ClassSym
		if outer != nil {
			scope = outer.Sym
		}
		sites = append(sites, site{x, scope})
	})
	for _, s := range sites {
		fn := ""
		switch f := s.x.Fun.(type) {
		case *syntax.Ident:
			fn = f.Name
			if !c.importsMetroFun(fn) {
				continue
			}
		case *syntax.Member:
			fn = f.Name
			if q := qualifiedName(f.X); q != metroPkg {
				continue
			}
		}
		ref := s.x.TypeArgs[0].Type
		if ref == nil {
			continue
		}
		t := c.metroResolve(ref, s.cls)
		if t.K != types.Class {
			continue
		}
		cls := t.C
		var src string
		if fn == "createGraph" {
			src = metroGraphCall(cls, false)
		} else {
			src = m.factoryCall(cls)
		}
		if src == "" {
			m.errorf(c, s.x.P, "%s is not a Metro graph factory", cls.DisplayName())
			continue
		}
		calls := m.plan.calls[c.file.Path]
		if calls == nil {
			calls = map[syntax.Pos]string{}
			m.plan.calls[c.file.Path] = calls
		}
		calls[s.x.P] = src
	}
}

// factoryCall returns the Kotlin expression for createGraphFactory of
// factory class f.
func (m *metroAnalysis) factoryCall(f *types.ClassSym) string {
	g := f.Outer
	if g == nil {
		return ""
	}
	if isSource(g) {
		return metroGraphCall(f, true)
	}
	g.Ensure()
	if comp := g.Companion; comp != nil {
		if comp.IsSubclassOf(f) {
			return metroGraphCall(f, true)
		}
		if len(comp.Methods["factory"]) > 0 {
			return metroClassName(g) + ".Companion.factory()"
		}
	}
	if cp := m.u.ext.cp; cp != nil && cp.Has(f.Name+"$Impl") {
		return "kotlin.run { " + metroClassName(f) + ".Impl }"
	}
	return metroGraphCall(f, true)
}

// importsMetroFun reports whether the file can call Metro's function
// name by its simple name.
func (c *Checker) importsMetroFun(name string) bool {
	if c.file.Package == metroPkg {
		return true
	}
	for _, imp := range c.file.Imports {
		if imp.Star && imp.Path == metroPkg || !imp.Star && imp.Path == metroPkg+"."+name && (imp.Alias == "" || imp.Alias == name) {
			return true
		}
	}
	return false
}

// metroCallsIn is metroCalls, with the class each call is in, or nil.
func metroCallsIn(file *syntax.File, f func(*syntax.Call, *syntax.ClassDecl)) {
	var visit func(ds []syntax.Decl, outer *syntax.ClassDecl)
	visit = func(ds []syntax.Decl, outer *syntax.ClassDecl) {
		var own []syntax.Decl
		for _, d := range ds {
			if cd, ok := d.(*syntax.ClassDecl); ok {
				visit(cd.Members, cd)
				shallow := *cd
				shallow.Members = nil
				own = append(own, &shallow)
				continue
			}
			own = append(own, d)
		}
		metroCalls(&syntax.File{Decls: own}, func(x *syntax.Call) { f(x, outer) })
	}
	visit(file.Decls, nil)
}
