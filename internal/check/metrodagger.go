package check

import (
	"fmt"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Dagger's factories. A graph that kotlinc compiles with Metro reads a
// class of another module only through what Metro or, with
// enable-dagger-runtime-interop, Dagger generated for it: its _Factory, a
// Dagger module's _<Name>Factory per @Provides function, an
// @AssistedFactory's _Impl. kc writes Dagger's, as Kotlin, so that such a
// graph can use the modules kc compiles; it writes no Metro factories,
// whose metadata is Metro's own.
//
// The factories are what Dagger 2.60 writes, but in Kotlin: their static
// functions are a companion object's, or an object's, @JvmStatic
// functions, as Anvil's are, which Metro reads too.

// daggerFactories reports whether kc writes Dagger's factories for the
// module's injected classes and modules: with Metro's
// enable-dagger-runtime-interop and Dagger's runtime on the class path.
func (m *metroAnalysis) daggerFactories() bool {
	return m.cfg.daggerRuntime && m.class("dagger/internal/Factory") != nil
}

// writeDaggerFactories adds the files of the module's Dagger factories to
// the plan. A class it can't write one for has none, and a graph that
// kotlinc compiles reports it as unprocessed.
func (m *metroAnalysis) writeDaggerFactories() {
	if !m.daggerFactories() {
		return
	}
	for _, c := range m.checkers {
		for _, d := range c.classes {
			cls := d.Sym
			if cls == nil || cls.Local || cls.Anonymous {
				continue
			}
			vis, ok := daggerVisibility(cls)
			if !ok {
				continue
			}
			anns := m.annsOf(cls)
			m.daggerInjectFactory(d, vis)
			m.daggerMembersInjector(c, d, vis)
			if find(anns, mAssistedFactory) != nil {
				m.daggerAssistedImpl(cls, vis)
			}
			if a := find(anns, mBindingContainer); a != nil && a.orig == "dagger/Module" || find(anns, "dagger/Module") != nil {
				m.daggerModuleFactories(cls, vis)
			}
		}
	}
}

// daggerVisibility returns the modifier of the factories of a class,
// internal for an internal class, or false if code of its package can't
// see the class.
func daggerVisibility(cls *types.ClassSym) (string, bool) {
	vis := ""
	for c := cls; c != nil; c = c.Outer {
		switch {
		case c.Internal:
			vis = "internal "
		case c.Private:
			return "", false
		}
	}
	return vis, true
}

// daggerName returns the package of a class, as Kotlin writes it, and
// the name of its generated class with suffix: Outer_Inner_Factory.
func daggerName(cls *types.ClassSym, suffix string) (pkg, name string) {
	top := cls
	for top.Outer != nil {
		top = top.Outer
	}
	if i := strings.LastIndexByte(top.Name, '/'); i >= 0 {
		pkg = strings.ReplaceAll(top.Name[:i], "/", ".")
	}
	simple := strings.TrimPrefix(fqName(cls), pkg)
	simple = strings.TrimPrefix(simple, ".")
	return pkg, strings.ReplaceAll(simple, ".", "_") + suffix
}

// addDaggerFile adds the file of a generated class of package pkg, but
// not if the program or the class path already has a class of its name,
// as one written by hand.
func (m *metroAnalysis) addDaggerFile(pkg, name, src string) {
	internal := name
	if pkg != "" {
		internal = strings.ReplaceAll(pkg, ".", "/") + "/" + name
	}
	if m.class(internal) != nil {
		return
	}
	var b strings.Builder
	path := "metro/dagger/" + name + ".kt"
	if pkg != "" {
		fmt.Fprintf(&b, "package %s\n\n", pkgSrc(pkg))
		path = "metro/dagger/" + strings.ReplaceAll(pkg, ".", "/") + "/" + name + ".kt"
	}
	b.WriteString(src)
	m.plan.Files = append(m.plan.Files, MetroFile{Path: path, Src: b.String()})
}

// daggerTypeParams returns the declaration and the arguments of the type
// parameters of a generated class or function, <T : B> and <T>, or false
// if kc can't write them.
func daggerTypeParams(tps []*types.TypeParamSym) (decl, args string, ok bool) {
	if len(tps) == 0 {
		return "", "", true
	}
	var ds, as []string
	for _, tp := range tps {
		d := backtick(tp.Name)
		switch len(tp.Bounds) {
		case 0:
		case 1:
			s, ok := kotlinTypeSrc(tp.Bounds[0])
			if !ok {
				return "", "", false
			}
			d += " : " + s
		default:
			return "", "", false
		}
		ds = append(ds, d)
		as = append(as, backtick(tp.Name))
	}
	return "<" + strings.Join(ds, ", ") + ">", "<" + strings.Join(as, ", ") + ">", true
}

// daggerParam is a parameter of an injected constructor or a @Provides
// function: its name, declared type and, but for an assisted one, the
// type its provider provides and the expression that converts the
// provider, prov, to the declared type.
type daggerParam struct {
	name, typ string
	assisted  bool
	key       string
	conv      func(prov string) string
}

// daggerParams returns the parameters of a function, or false if kc
// can't write them; assisted are those to pass to the factory's get.
func (m *metroAnalysis) daggerParams(ps []*types.VarSym, defs []bool, assisted func(i int) bool) ([]daggerParam, bool) {
	var out []daggerParam
	for i, p := range ps {
		if i < len(defs) && defs[i] || isVarargParam(p) {
			return nil, false
		}
		ts, ok := kotlinTypeSrc(p.Type)
		if !ok {
			return nil, false
		}
		dp := daggerParam{name: backtick(p.Name), typ: ts}
		if assisted != nil && assisted(i) {
			dp.assisted = true
			out = append(out, dp)
			continue
		}
		t := p.Type
		var layers []byte
		var inner []string
		for {
			w := m.wrapperOf(t)
			if w == 0 || w == 'f' && !m.functionProviders {
				break
			}
			layers = append(layers, w)
			t = t.A.List[len(t.A.List)-1].T
			s, ok := kotlinTypeSrc(t)
			if !ok {
				return nil, false
			}
			inner = append(inner, s)
		}
		if dp.key, ok = kotlinTypeSrc(t); !ok {
			return nil, false
		}
		dp.conv = func(prov string) string {
			x := prov + ".get()"
			for i := len(layers) - 1; i >= 0; i-- {
				x = wrapSrc(layers[i], x, inner[i])
			}
			return x
		}
		out = append(out, dp)
	}
	return out, true
}

// isVarargParam reports whether a parameter of the program is vararg.
func isVarargParam(p *types.VarSym) bool {
	switch d := p.Decl.(type) {
	case *syntax.Param:
		return d.Vararg
	case *syntax.CtorParam:
		return d.Param.Vararg || d.Modifiers.Vararg
	}
	return false
}

// daggerProviders writes a factory's constructor parameters, its fields
// of the providers of ps, and the arguments that pass them.
func daggerProviders(ps []daggerParam) (fields, params, args []string) {
	for _, p := range ps {
		if p.assisted {
			continue
		}
		f := providerField(p.name)
		fields = append(fields, fmt.Sprintf("private val %s: dagger.internal.Provider<%s>", f, p.key))
		params = append(params, fmt.Sprintf("%s: dagger.internal.Provider<%s>", f, p.key))
		args = append(args, f)
	}
	return
}

func providerField(name string) string {
	return backtick(strings.Trim(name, "`") + "Provider")
}

// daggerInjectFactory writes the _Factory of an injected class: a
// dagger.internal.Factory whose create takes its parameters' providers
// and newInstance its parameters, or, for an assisted-injected class, a
// class whose get takes the assisted parameters.
func (m *metroAnalysis) daggerInjectFactory(d *syntax.ClassDecl, vis string) {
	cls := d.Sym
	ctor, pa, defs, assisted := m.ctorOf(cls)
	if ctor == nil || ctor.Private || ctor.Protected || cls.Abstract || cls.Inner || cls.Object {
		return
	}
	// Metro can't use the factory of a class with injected members, as
	// Dagger writes it, which injects them too.
	if members, ok := m.injectedChain(cls); !ok || len(members) > 0 {
		return
	}
	tpDecl, tpArgs, ok := daggerTypeParams(cls.TypeParams)
	if !ok {
		return
	}
	ps, ok := m.daggerParams(ctor.Params, defs, func(i int) bool {
		return assisted && i < len(pa) && find(pa[i], mAssisted) != nil
	})
	if !ok {
		return
	}
	ts, ok := kotlinTypeSrc(cls.This())
	if !ok {
		return
	}
	pkg, name := daggerName(cls, "_Factory")
	fields, cparams, cargs := daggerProviders(ps)
	var getArgs, getParams, newParams, newArgs []string
	for _, p := range ps {
		newParams = append(newParams, p.name+": "+p.typ)
		newArgs = append(newArgs, p.name)
		if p.assisted {
			getParams = append(getParams, p.name+": "+p.typ)
			getArgs = append(getArgs, p.name)
		} else {
			getArgs = append(getArgs, p.conv(providerField(p.name)))
		}
	}
	self := backtick(name) + tpArgs
	var b strings.Builder
	fmt.Fprintf(&b, "%sclass %s%s private constructor(%s)", vis, backtick(name), tpDecl, strings.Join(fields, ", "))
	get := "override fun get(): " + ts
	if assisted {
		get = "fun get(" + strings.Join(getParams, ", ") + "): " + ts
	} else {
		fmt.Fprintf(&b, " : dagger.internal.Factory<%s>", ts)
	}
	fmt.Fprintf(&b, " {\n%s = newInstance(%s)\n\n", get, strings.Join(getArgs, ", "))
	fmt.Fprintf(&b, "companion object {\n@kotlin.jvm.JvmStatic\nfun %s create(%s): %s = %s(%s)\n\n", tpDecl, strings.Join(cparams, ", "), self, backtick(name), strings.Join(cargs, ", "))
	fmt.Fprintf(&b, "@kotlin.jvm.JvmStatic\nfun %s newInstance(%s): %s = %s(%s)\n}\n}\n", tpDecl, strings.Join(newParams, ", "), ts, ts, strings.Join(newArgs, ", "))
	m.addDaggerFile(pkg, name, b.String())
}

// daggerAssistedImpl writes the _Impl of an @AssistedFactory, which
// calls the get of its assisted-injected class's _Factory.
func (m *metroAnalysis) daggerAssistedImpl(f *types.ClassSym, vis string) {
	if len(f.TypeParams) > 0 || !f.Abstract || f.Inner {
		return
	}
	var sam *mMember
	for _, mm := range m.inherited(f) {
		if mm.abstract {
			if sam != nil {
				return
			}
			sam = mm
		}
	}
	if sam == nil || sam.prop || sam.recv != nil || sam.ret.K != types.Class || sam.ret.N {
		return
	}
	target := sam.ret.C
	ctor, pa, _, assisted := m.ctorOf(target)
	if ctor == nil || !assisted || len(target.TypeParams) > 0 || !isSource(target) {
		return
	}
	byPos, err := m.assistedArgs(f, sam, target, ctor, pa)
	if err != nil {
		return
	}
	fs, ok := kotlinTypeSrc(f.This())
	ret, ok2 := kotlinTypeSrc(sam.ret)
	if !ok || !ok2 {
		return
	}
	var params, args []string
	for i, p := range sam.params {
		ts, ok := kotlinTypeSrc(p.Type)
		if !ok {
			return
		}
		params = append(params, fmt.Sprintf("`$a%d`: %s", i, ts))
	}
	for i := range ctor.Params {
		if j, ok := byPos[i]; ok {
			args = append(args, fmt.Sprintf("`$a%d`", j))
		}
	}
	tpkg, tname := daggerName(target, "_Factory")
	delegate := backtick(tname)
	if tpkg != "" {
		delegate = tpkg + "." + delegate
	}
	pkg, name := daggerName(f, "_Impl")
	super := fs
	if !f.Interface {
		super += "()"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%sclass %s internal constructor(private val delegateFactory: %s) : %s {\n", vis, backtick(name), delegate, super)
	fmt.Fprintf(&b, "override fun %s(%s): %s = delegateFactory.get(%s)\n\n", backtick(sam.name), strings.Join(params, ", "), ret, strings.Join(args, ", "))
	fmt.Fprintf(&b, "companion object {\n@kotlin.jvm.JvmStatic\nfun create(delegateFactory: %s): javax.inject.Provider<%s> = dagger.internal.InstanceFactory.create(%s(delegateFactory))\n\n", delegate, fs, backtick(name))
	fmt.Fprintf(&b, "@kotlin.jvm.JvmStatic\nfun createFactoryProvider(delegateFactory: %s): dagger.internal.Provider<%s> = dagger.internal.InstanceFactory.create(%s(delegateFactory))\n}\n}\n", delegate, fs, backtick(name))
	m.addDaggerFile(pkg, name, b.String())
}

// daggerModuleFactories writes the _<Name>Factory of each @Provides
// function of a Dagger module, an object's or an instance's. Metro reads
// neither a companion object's nor properties' as Dagger's.
func (m *metroAnalysis) daggerModuleFactories(mod *types.ClassSym, vis string) {
	if len(mod.TypeParams) > 0 || mod.Inner || mod.CompanionOf != nil {
		return
	}
	object := mod.Object
	if !object && mod.Abstract {
		return
	}
	modSrc := metroClassName(mod)
	members := m.declared(mod)
	count := map[string]int{}
	for _, mm := range members {
		count[mm.name]++
	}
	for _, mm := range members {
		if find(mm.anns, mProvides) == nil || mm.prop || mm.recv != nil || mm.abstract || count[mm.name] > 1 {
			continue
		}
		f := mm.fn
		if f == nil || f.Private || f.Protected || len(f.TypeParams) > 0 {
			continue
		}
		ps, ok := m.daggerParams(mm.params, mm.defaults, nil)
		if !ok {
			continue
		}
		ret, ok := kotlinTypeSrc(mm.ret)
		if !ok {
			continue
		}
		cap := strings.ToUpper(mm.name[:1]) + mm.name[1:]
		pkg, name := daggerName(mod, "_"+cap+"Factory")
		fn := backtick(mm.name)
		fields, cparams, cargs := daggerProviders(ps)
		var getArgs, sparams, sargs []string
		if !object {
			fields = append([]string{"private val module: " + modSrc}, fields...)
			cparams = append([]string{"module: " + modSrc}, cparams...)
			cargs = append([]string{"module"}, cargs...)
			getArgs = append(getArgs, "module")
			sparams = append(sparams, "instance: "+modSrc)
		}
		for _, p := range ps {
			getArgs = append(getArgs, p.conv(providerField(p.name)))
			sparams = append(sparams, p.name+": "+p.typ)
			sargs = append(sargs, p.name)
		}
		recv := modSrc
		if !object {
			recv = "instance"
		}
		call := fmt.Sprintf("%s.%s(%s)", recv, fn, strings.Join(sargs, ", "))
		var b strings.Builder
		if len(fields) == 0 {
			fmt.Fprintf(&b, "%sobject %s : dagger.internal.Factory<%s> {\noverride fun get(): %s = %s()\n\n", vis, backtick(name), ret, ret, fn)
			fmt.Fprintf(&b, "@kotlin.jvm.JvmStatic\nfun create(): %s = this\n\n", backtick(name))
			fmt.Fprintf(&b, "@kotlin.jvm.JvmStatic\nfun %s(): %s = %s\n}\n", fn, ret, call)
		} else {
			fmt.Fprintf(&b, "%sclass %s private constructor(%s) : dagger.internal.Factory<%s> {\n", vis, backtick(name), strings.Join(fields, ", "), ret)
			fmt.Fprintf(&b, "override fun get(): %s = %s(%s)\n\n", ret, fn, strings.Join(getArgs, ", "))
			fmt.Fprintf(&b, "companion object {\n@kotlin.jvm.JvmStatic\nfun create(%s): %s = %s(%s)\n\n", strings.Join(cparams, ", "), backtick(name), backtick(name), strings.Join(cargs, ", "))
			fmt.Fprintf(&b, "@kotlin.jvm.JvmStatic\nfun %s(%s): %s = %s\n}\n}\n", fn, strings.Join(sparams, ", "), ret, call)
		}
		m.addDaggerFile(pkg, name, b.String())
	}
}

// assistedArgs matches the assisted parameters of an assisted-injected
// class's constructor with the parameters of its @AssistedFactory's
// function sam: by position in the constructor, the position in sam.
// Dagger's @Assisted parameters match by type and identifier, its value;
// Metro's by name.
func (m *metroAnalysis) assistedArgs(f *types.ClassSym, sam *mMember, target *types.ClassSym, ctor *types.FunSym, pa [][]*mAnn) (map[int]int, error) {
	byName := map[string]int{}
	daggerArgs := map[string][]int{}
	for i, p := range sam.params {
		ts, ok := kotlinTypeSrc(p.Type)
		if !ok {
			return nil, unsupportedErr{fmt.Sprintf("kc can't name %s", p.Type)}
		}
		byName[p.Name] = i
		var as []*mAnn
		if i < len(sam.paramAnns) {
			as = sam.paramAnns[i]
		}
		id := ts + "|" + find(as, mAssisted).str("value")
		daggerArgs[id] = append(daggerArgs[id], i)
	}
	byPos := map[int]int{}
	for i, p := range ctor.Params {
		var as []*mAnn
		if i < len(pa) {
			as = pa[i]
		}
		a := find(as, mAssisted)
		if a == nil {
			continue
		}
		if a.orig == "dagger/assisted/Assisted" {
			ts, _ := kotlinTypeSrc(p.Type)
			id := a.str("value")
			switch args := daggerArgs[ts+"|"+id]; len(args) {
			case 1:
				byPos[i] = args[0]
				continue
			case 0:
				return nil, fmt.Errorf("@AssistedFactory %s has no parameter of type %s with identifier %q for %s's assisted parameter %s", f.DisplayName(), ts, id, target.DisplayName(), p.Name)
			default:
				return nil, fmt.Errorf("@AssistedFactory %s has more than one parameter of type %s with identifier %q", f.DisplayName(), ts, id)
			}
		}
		j, ok := byName[p.Name]
		if !ok {
			return nil, fmt.Errorf("@AssistedFactory %s has no parameter named %s for %s's assisted parameter", f.DisplayName(), p.Name, target.DisplayName())
		}
		byPos[i] = j
	}
	return byPos, nil
}

// unsupportedErr is an error of a feature kc doesn't support.
type unsupportedErr struct{ msg string }

func (e unsupportedErr) Error() string { return e.msg }

// daggerMember is an injected member of a class of the program: a
// property, whose value its inject function sets, or a function, which
// its inject function calls.
type daggerMember struct {
	owner  *types.ClassSym
	name   string
	fn     bool
	params []daggerParam
	// quals are the qualifier annotations of a property, on its inject
	// function, as Dagger writes them, or of a function's parameters.
	qual   string
	pquals []string
}

// daggerMembers returns the injected members of a class of the program,
// or false if kc can't write their inject functions.
func (m *metroAnalysis) daggerMembers(cls *types.ClassSym) ([]daggerMember, bool) {
	d, ok := cls.Decl.(*syntax.ClassDecl)
	if !ok || cls.External {
		return nil, true
	}
	c := m.checkerOf[cls]
	if c == nil {
		return nil, false
	}
	own := func(anns []syntax.Annotation) []*mAnn {
		var out []syntax.Annotation
		for _, a := range anns {
			if !a.Accessor {
				out = append(out, a)
			}
		}
		return c.fromSource(out, cls)
	}
	qualSrc := func(as []*mAnn) (string, bool) {
		q := m.qualifierAnn(as)
		if q == nil {
			return "", true
		}
		s, err := m.kotlinAnnSrc(q)
		return s, err == nil
	}
	var out []daggerMember
	for _, md := range d.Members {
		switch md := md.(type) {
		case *syntax.PropDecl:
			as := own(md.Annotations)
			if find(as, mInject) == nil {
				continue
			}
			v := md.Sym
			if v == nil || !v.Mutable || md.Recv != nil || md.Visibility == syntax.Private || md.Visibility == syntax.Protected || md.SetterVis == syntax.Private || md.SetterVis == syntax.Protected {
				return nil, false
			}
			ps, ok := m.daggerParams([]*types.VarSym{{Name: v.Name, Type: v.Type}}, nil, nil)
			if !ok {
				return nil, false
			}
			q, ok := qualSrc(as)
			if !ok {
				return nil, false
			}
			out = append(out, daggerMember{owner: cls, name: v.Name, params: ps, qual: q})
		case *syntax.FunDecl:
			as := own(md.Annotations)
			if find(as, mInject) == nil {
				continue
			}
			f := md.Sym
			if f == nil || f.Private || f.Protected || md.Recv != nil || len(f.TypeParams) > 0 {
				return nil, false
			}
			ps, ok := m.daggerParams(f.Params, nil, nil)
			if !ok {
				return nil, false
			}
			dm := daggerMember{owner: cls, name: f.Name, fn: true, params: ps}
			for i := range f.Params {
				q, ok := qualSrc(c.fromSource(md.Params[i].Annotations, cls))
				if !ok {
					return nil, false
				}
				dm.pquals = append(dm.pquals, q)
			}
			out = append(out, dm)
		}
	}
	return out, true
}

// injectedChain returns the injected members of a class and its
// superclasses, the outermost superclass's first, and whether the class
// has any: false, false if a superclass of the class path has some, whose
// inject functions kc doesn't know.
func (m *metroAnalysis) injectedChain(cls *types.ClassSym) (members []daggerMember, ok bool) {
	var chain []*types.ClassSym
	for s := cls; s != nil && s.Name != "java/lang/Object"; s = s.Super {
		chain = append([]*types.ClassSym{s}, chain...)
	}
	for _, s := range chain {
		if !isSource(s) {
			if m.hasInjectedMembers(s) {
				return nil, false
			}
			continue
		}
		ms, ok := m.daggerMembers(s)
		if !ok || len(ms) > 0 && s != cls && len(s.TypeParams) > 0 {
			return nil, false
		}
		members = append(members, ms...)
	}
	return members, true
}

// daggerMembersInjector writes the _MembersInjector of a class with
// injected members: its static inject function per member, and a
// dagger.MembersInjector whose create takes the providers of its and
// its superclasses' members.
//
// Without it, a graph that kotlinc compiles would take the class for one
// without injected members, and inject none: one kc can't write is
// reported.
func (m *metroAnalysis) daggerMembersInjector(c *Checker, d *syntax.ClassDecl, vis string) {
	cls := d.Sym
	own, ok := m.daggerMembers(cls)
	if ok && len(own) == 0 {
		return
	}
	all, chainOK := m.injectedChain(cls)
	ts, tsOK := kotlinTypeSrc(cls.This())
	tdecl, targs, tpOK := daggerTypeParams(cls.TypeParams)
	if !ok || !chainOK || !tsOK || !tpOK || cls.Inner {
		m.unsupported(c, d.P, "kc can't write the Dagger members injector of %s, which a graph that kotlinc compiles needs to inject its members, with enable-dagger-runtime-interop; its injected members must be public or internal properties with setters, or functions without type parameters, and its superclasses' those of the program's non-generic classes", cls.DisplayName())
		return
	}
	pkg, name := daggerName(cls, "_MembersInjector")
	used := map[string]bool{}
	var fields, cparams, cargs, stmts []string
	for _, dm := range all {
		injector := ""
		if dm.owner != cls {
			ipkg, iname := daggerName(dm.owner, "_MembersInjector")
			injector = backtick(iname) + "."
			if ipkg != "" {
				injector = pkgSrc(ipkg) + "." + injector
			}
		}
		args := []string{"instance"}
		for _, p := range dm.params {
			f := providerField(p.name)
			for n := 2; used[f]; n++ {
				f = providerField(fmt.Sprintf("%s%d", strings.Trim(p.name, "`"), n))
			}
			used[f] = true
			fields = append(fields, fmt.Sprintf("private val %s: dagger.internal.Provider<%s>", f, p.key))
			cparams = append(cparams, fmt.Sprintf("%s: dagger.internal.Provider<%s>", f, p.key))
			cargs = append(cargs, f)
			args = append(args, p.conv(f))
		}
		stmts = append(stmts, fmt.Sprintf("%s%s(%s)", injector, injectFunName(dm.name), strings.Join(args, ", ")))
	}
	var b strings.Builder
	tfun := ""
	if tdecl != "" {
		tfun = tdecl + " "
	}
	fmt.Fprintf(&b, "%sclass %s%s private constructor(%s) : dagger.MembersInjector<%s> {\n", vis, backtick(name), tdecl, strings.Join(fields, ", "), ts)
	fmt.Fprintf(&b, "override fun injectMembers(instance: %s) {\n%s\n}\n\n", ts, strings.Join(stmts, "\n"))
	fmt.Fprintf(&b, "companion object {\n@kotlin.jvm.JvmStatic\nfun %screate(%s): dagger.MembersInjector<%s> = %s%s(%s)\n", tfun, strings.Join(cparams, ", "), ts, backtick(name), targs, strings.Join(cargs, ", "))
	for _, dm := range own {
		params := []string{"instance: " + ts}
		var args []string
		for i, p := range dm.params {
			q := ""
			if dm.fn && dm.pquals[i] != "" {
				q = dm.pquals[i] + " "
			}
			params = append(params, q+p.name+": "+p.typ)
			args = append(args, p.name)
		}
		b.WriteString("\n@kotlin.jvm.JvmStatic\n")
		if dm.fn {
			fmt.Fprintf(&b, "fun %s%s(%s) {\ninstance.%s(%s)\n}\n", tfun, injectFunName(dm.name), strings.Join(params, ", "), backtick(dm.name), strings.Join(args, ", "))
			continue
		}
		fmt.Fprintf(&b, "@dagger.internal.InjectedFieldSignature(%q)\n", fqName(cls)+"."+dm.name)
		if dm.qual != "" {
			b.WriteString(dm.qual + "\n")
		}
		fmt.Fprintf(&b, "fun %s%s(%s) {\ninstance.%s = %s\n}\n", tfun, injectFunName(dm.name), strings.Join(params, ", "), backtick(dm.name), args[0])
	}
	b.WriteString("}\n}\n")
	m.addDaggerFile(pkg, name, b.String())
}

// injectFunName is the name of a member's inject function.
func injectFunName(member string) string {
	return backtick("inject" + strings.ToUpper(member[:1]) + member[1:])
}

// pkgSrc returns a package name as source, its keywords backticked.
func pkgSrc(pkg string) string {
	parts := strings.Split(pkg, ".")
	for i, p := range parts {
		if kotlinKeywords[p] {
			parts[i] = "`" + p + "`"
		}
	}
	return strings.Join(parts, ".")
}
