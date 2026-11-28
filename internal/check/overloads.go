package check

import (
	"fmt"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The conflicts between the declarations of one scope, a package or a
// class, that kotlinc reports by their Kotlin signatures, whatever their
// JVM ones: two functions with the same parameter types, whatever they
// return (CONFLICTING_OVERLOADS), a function and a class's constructor,
// a property and a class (REDECLARATION), a package and a class
// (PACKAGE_CONFLICTS_WITH_CLASSIFIER), and two classes with the same JVM
// name (DUPLICATE_CLASS_NAMES).

// overload is a function, or a constructor, of a scope, by what kotlinc
// tells overloads apart by.
type overload struct {
	c       *Checker // the file's
	name    string
	pos     syntax.Pos
	decl    any // what it is reported once for
	fun     *types.FunSym
	tparams []*types.TypeParamSym
	recv    *types.Type // an extension's receiver type
	ctx     []types.Type
	params  []types.Type
	vararg  int // the index of the vararg parameter, or -1
	suspend bool
	ctor    bool
	prop    bool // a property, which contextualShadowing compares too
	// private is set on a private one, which doesn't conflict with those
	// of other files.
	private bool
	// hidden is set on one deprecated with level HIDDEN, which only
	// conflicts with others if one of them can be overridden: open.
	hidden, open bool
	// main is set on a top-level fun main() or main(args: Array<String>),
	// which other files can declare too.
	main bool
	// low is set on one annotated @LowPriorityInOverloadResolution, which
	// doesn't conflict with one that isn't.
	low bool
}

// conflicts reports the conflicting declarations of the program's
// packages and classes.
func conflicts(checkers []*Checker) {
	if len(checkers) == 0 {
		return
	}
	u := checkers[0].u
	if u.conflicted == nil {
		u.conflicted = map[any]bool{}
	}
	reported := map[any]bool{}
	pkgs := map[string][]*Checker{}
	var order []string
	for _, c := range checkers {
		if pkgs[c.file.Package] == nil {
			order = append(order, c.file.Package)
		}
		pkgs[c.file.Package] = append(pkgs[c.file.Package], c)
	}
	for _, name := range order {
		var ovs []overload
		var decls []namedDecl
		for _, c := range pkgs[name] {
			for _, d := range c.file.Decls {
				ovs = c.declOverloads(ovs, d, nil)
				decls = c.namedDecls(decls, d)
			}
		}
		ovs = javaCtorOverloads(ovs, name, reported)
		overloadConflicts(ovs, reported)
		redeclarations(decls, reported)
		for _, c := range pkgs[name] {
			ovs = c.propOverloads(ovs, c.file.Decls, nil)
		}
		contextualShadowing(ovs, false)
		aliasedPropConflicts(ovs)
	}
	for _, c := range checkers {
		for _, d := range c.classes {
			if d.Sym == nil || d.Expect || d.Anonymous || d.Local {
				continue
			}
			var ovs []overload
			var decls []namedDecl
			for _, m := range d.Members {
				ovs = c.declOverloads(ovs, m, d)
				decls = c.namedDecls(decls, m)
			}
			ovs = c.dataOverloads(ovs, d)
			for _, cp := range d.CtorParams {
				if cp.IsProp && !d.Annotation {
					decls = append(decls, namedDecl{c: c, name: cp.Name, pos: cp.P, decl: cp, prop: true, hidden: cp.Hidden()})
				}
			}
			for _, e := range d.Entries {
				decls = append(decls, namedDecl{c: c, name: e.Name, pos: e.P, decl: e, prop: true, entry: true})
				if e.Name == "name" || e.Name == "ordinal" {
					// Enum's properties.
					c.errorf(e.P, "REDECLARATION", "redeclaration: %s", e.Name)
					c.errorf(d.P, "REDECLARATION", "redeclaration: %s", e.Name)
				}
				if e.Name == "entries" {
					c.errorf(e.P, "DECLARATION_OF_ENUM_ENTRY_ENTRIES_ERROR", "conflicting declarations: the enum entry 'entries' and the property 'Enum.entries' (KT-48872)")
				}
			}
			overloadConflicts(ovs, reported)
			redeclarations(decls, reported)
			c.inheritedConflicts(d, reported)
			props := c.propOverloads(nil, d.Members, d)
			contextualShadowing(append(ovs, props...), true)
			aliasedPropConflicts(props)
		}
	}
	classNames(checkers)
}

// declOverloads appends the overloads that declaration d of a package, or
// of class owner, adds to its scope: a function, or a class's or a
// typealias's constructors.
func (c *Checker) declOverloads(out []overload, d syntax.Decl, owner *syntax.ClassDecl) []overload {
	switch d := d.(type) {
	case *syntax.FunDecl:
		if d.Sym == nil || d.Expect {
			return out
		}
		o := funOverload(c, d.Sym, d.P, d)
		o.private = owner == nil && d.Visibility == syntax.Private
		o.hidden = d.Sym.Hidden || d.Hidden()
		o.open = d.Open || d.Abstract || d.Override && !d.Final ||
			owner != nil && owner.Interface && d.Visibility != syntax.Private
		o.low = d.Sym.LowPriority
		o.main = owner == nil && d.Name == "main" && d.Recv == nil && len(o.tparams) == 0 && len(o.ctx) == 0 &&
			d.Sym.Ret.K == types.Unit && (len(o.params) == 0 || len(o.params) == 1 && stringArray(o.params[0]))
		return append(out, o)
	case *syntax.ClassDecl:
		if !ctorScoped(d) {
			return out
		}
		for _, f := range userCtors(d.Sym) {
			pos, mods := d.P, &d.CtorMods
			if sc, ok := f.Decl.(*syntax.SecondaryCtor); ok {
				pos, mods = sc.P, &sc.Modifiers
			}
			o := funOverload(c, f, pos, f)
			o.low = lowPriorityMods(mods)
			o.name, o.tparams, o.ctor = d.Name, d.Sym.TypeParams, true
			o.private = ctorVisibility(f) == syntax.Private
			o.hidden = f.Hidden
			out = append(out, o)
		}
	case *syntax.TypeAliasDecl:
		a := d.Sym
		if a == nil || !a.Resolved || a.Type.K != types.Class {
			return out
		}
		cd, ok := a.Type.C.Decl.(*syntax.ClassDecl)
		if !ok || a.Type.C.External || !ctorScoped(cd) {
			return out
		}
		s := classSubst(a.Type)
		for _, f := range userCtors(a.Type.C) {
			o := funOverload(c, f, d.P, d)
			o.name, o.tparams, o.ctor = d.Name, a.TypeParams, true
			for i, t := range o.params {
				o.params[i] = s.Apply(t)
			}
			o.private = d.Visibility == syntax.Private || ctorVisibility(f) == syntax.Private
			o.hidden = f.Hidden
			out = append(out, o)
		}
	}
	return out
}

// javaCtorOverloads appends the constructors of the Java classes of
// package pkg that its top-level functions ovs are named after: fun J()
// conflicts with a Java class J's J(), which only the function is
// reported for.
func javaCtorOverloads(ovs []overload, pkg string, reported map[any]bool) []overload {
	done := map[string]bool{}
	for _, o := range ovs[:len(ovs):len(ovs)] {
		if o.ctor || o.recv != nil || o.prop || done[o.name] {
			continue
		}
		done[o.name] = true
		name := o.name
		if pkg != "" {
			name = strings.ReplaceAll(pkg, ".", "/") + "/" + o.name
		}
		cls := o.c.u.ext.class(name)
		if cls == nil || !isJavaClass(cls) || cls.Interface || cls.Abstract || cls.Annotation {
			continue
		}
		cls.Ensure()
		for _, f := range cls.Ctors {
			if f.Private {
				continue
			}
			j := funOverload(o.c, f, o.pos, f)
			j.name, j.tparams, j.ctor, j.hidden = o.name, cls.TypeParams, true, f.Hidden
			reported[f] = true // the Java class isn't the program's to report
			ovs = append(ovs, j)
		}
	}
	return ovs
}

// ctorScoped reports whether class d of the program has constructors
// that calls by its name call: it isn't an object, an interface, an
// annotation class, an enum entry's class or an inner class.
func ctorScoped(d *syntax.ClassDecl) bool {
	return d.Sym != nil && !d.Expect && !d.Object && !d.Interface && !d.Annotation && !d.EnumEntry && !d.Inner
}

// dataOverloads appends the members that data class d gets, componentN
// and copy, which its own functions with their signatures conflict with.
func (c *Checker) dataOverloads(out []overload, d *syntax.ClassDecl) []overload {
	if !d.Data || d.Object || len(d.CtorParams) == 0 {
		return out
	}
	var params []types.Type
	for i, cp := range d.CtorParams {
		if !cp.IsProp || cp.Sym == nil {
			return out
		}
		params = append(params, cp.Sym.Type)
		out = append(out, overload{c: c, name: fmt.Sprintf("component%d", i+1), pos: d.P, decl: d, vararg: -1})
	}
	return append(out, overload{c: c, name: "copy", pos: d.P, decl: d, params: params, vararg: -1})
}

// funOverload returns function or constructor f as an overload.
func funOverload(c *Checker, f *types.FunSym, pos syntax.Pos, decl any) overload {
	o := overload{c: c, name: f.Name, pos: pos, decl: decl, fun: f, tparams: f.TypeParams, suspend: f.Suspend, vararg: -1}
	if f.Ext {
		t := f.Recv
		o.recv = &t
	}
	for _, p := range f.Contexts {
		o.ctx = append(o.ctx, p.Type)
	}
	for i, p := range f.Params {
		o.params = append(o.params, p.Type)
		if p.Vararg {
			o.vararg = i
		}
	}
	return o
}

// lowPriorityMods reports whether modifiers m have the annotation
// @LowPriorityInOverloadResolution, as declare reads it for functions.
func lowPriorityMods(m *syntax.Modifiers) bool {
	return m.Find("", "internal.LowPriorityInOverloadResolution") != nil || m.Find("", "LowPriorityInOverloadResolution") != nil
}

// stringArray reports whether t is Array<String>, as main's parameter.
func stringArray(t types.Type) bool {
	s := t.String()
	return s == "Array<String>" || s == "Array<out String>"
}

// sameSignature reports whether overloads a and b take the same
// receiver, context parameters and parameters, with the same type
// parameters, which correspond by position: kotlinc doesn't tell them
// apart by what they return, nor by suspend.
func sameSignature(a, b *overload) bool {
	_, ok := signatureMatch(a, b, true)
	return ok
}

// mutuallySpecific reports whether overloads a and b are each at least
// as specific as the other, which K2 also takes for conflicting
// overloads (FirDeclarationOverloadabilityHelper): a call that one takes
// the other takes too, whatever their type parameters' bounds, as fun
// <A> f(arg: A?) and fun <B> f(arg: B), or fun <T> f(x: Inv<T>) and fun
// <T> f(x: Inv<out T>), if they have as many type parameters.
func mutuallySpecific(a, b *overload) bool {
	if len(a.tparams) != len(b.tparams) || (a.recv == nil) != (b.recv == nil) || len(a.ctx) != len(b.ctx) || len(a.params) != len(b.params) || a.vararg != b.vararg || a.c != b.c && a.c.u != b.c.u {
		return false
	}
	return a.c.overloadMoreSpecific(a, b) && a.c.overloadMoreSpecific(b, a)
}

// overloadMoreSpecific reports whether g takes what f takes: f's
// receiver, context parameters and parameters, with f's type parameters
// kept and g's inferred within their bounds.
func (c *Checker) overloadMoreSpecific(f, g *overload) bool {
	all := slices.Concat(f.ctx, f.params)
	if f.recv != nil {
		all = append(all, *f.recv)
	}
	all = append(all, slices.Concat(g.ctx, g.params)...)
	if g.recv != nil {
		all = append(all, *g.recv)
	}
	if slices.ContainsFunc(all, func(t types.Type) bool { return t.K == types.Invalid }) {
		return false
	}
	sys := newSystem(c)
	sub := sys.fresh(g.tparams, nil, syntax.Pos{})
	for _, p := range g.tparams {
		v := sub[p].T
		for _, b := range v.Param().Bounds {
			sys.sub(v, b)
		}
	}
	for i := range f.ctx {
		sys.sub(f.ctx[i], sub.ApplyIn(g.ctx[i]))
	}
	for i := range f.params {
		sys.sub(f.params[i], sub.ApplyIn(g.params[i]))
	}
	if f.recv != nil {
		sys.sub(*f.recv, sub.ApplyIn(*g.recv))
	}
	return sys.ok
}

// signatureMatch is sameSignature, but of context parameters only if
// ctx, which also returns the substitution of b's type parameters by
// a's.
func signatureMatch(a, b *overload, ctx bool) (types.Subst, bool) {
	if len(a.tparams) != len(b.tparams) || (a.recv == nil) != (b.recv == nil) || ctx && len(a.ctx) != len(b.ctx) ||
		len(a.params) != len(b.params) || a.vararg != b.vararg {
		return nil, false
	}
	s := types.Subst{}
	for i, p := range b.tparams {
		s[p] = types.Arg{T: types.ParamType(a.tparams[i])}
	}
	same := func(x, y types.Type) bool {
		return x.K != types.Invalid && x == s.Apply(y)
	}
	for i, p := range a.tparams {
		// The same bounds, in any order.
		q := b.tparams[i]
		if len(p.Bounds) != len(q.Bounds) {
			return nil, false
		}
		for _, x := range p.Bounds {
			found := false
			for _, y := range q.Bounds {
				found = found || same(x, y)
			}
			if !found {
				return nil, false
			}
		}
	}
	if a.recv != nil && !same(*a.recv, *b.recv) {
		return nil, false
	}
	// The same context parameters' types, in any order: a call passes
	// them by type.
	used := make([]bool, len(b.ctx))
	for _, x := range a.ctx {
		if !ctx {
			break
		}
		found := false
		for j, y := range b.ctx {
			if !used[j] && same(x, y) {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	for i := range a.params {
		if !same(a.params[i], b.params[i]) {
			return nil, false
		}
	}
	return s, true
}

// overloadConflicts reports the overloads of one scope that conflict.
func overloadConflicts(ovs []overload, reported map[any]bool) {
	byName := map[string][]int{}
	for i := range ovs {
		byName[ovs[i].name] = append(byName[ovs[i].name], i)
	}
	for i := range ovs {
		a := &ovs[i]
		for _, j := range byName[a.name] {
			if j <= i {
				continue
			}
			b := &ovs[j]
			switch {
			case a.ctor && b.ctor, // a class's, which resolveSecondaries reports
				a.decl == b.decl,
				a.c != b.c && (a.private && b.private || a.main && b.main),
				(a.hidden || b.hidden) && !a.open && !b.open,
				a.low != b.low,
				!sameSignature(a, b) && !mutuallySpecific(a, b):
				continue
			}
			for _, o := range []*overload{a, b} {
				if a.c != b.c && a.private != b.private && !o.private {
					// A private one of one file, and another file's: kotlinc
					// reports the private one.
					continue
				}
				if o.fun != nil {
					o.c.u.conflicted[o.fun] = true
				}
				if !reported[o.decl] {
					reported[o.decl] = true
					o.c.errorf(o.pos, "CONFLICTING_OVERLOADS", "conflicting overloads: %s", o.name)
				}
			}
		}
	}
}

// namedDecl is a property, or a classifier, of a scope.
type namedDecl struct {
	c       *Checker
	name    string
	pos     syntax.Pos
	decl    any
	prop    bool // a property, or an enum entry; else a class or a typealias
	entry   bool // an enum entry
	private bool
	hidden  bool
}

// namedDecls appends declaration d, if it is a property without a
// receiver, a class or a typealias.
func (c *Checker) namedDecls(out []namedDecl, d syntax.Decl) []namedDecl {
	switch d := d.(type) {
	case *syntax.PropDecl:
		if d.Recv == nil && !d.Expect && d.DelegateOf == nil {
			out = append(out, namedDecl{c: c, name: d.Name, pos: d.P, decl: d, prop: true, private: d.Visibility == syntax.Private, hidden: d.Hidden()})
		}
	case *syntax.ClassDecl:
		if !d.Expect && !d.EnumEntry {
			out = append(out, namedDecl{c: c, name: d.Name, pos: d.P, decl: d, private: d.Visibility == syntax.Private, hidden: d.Hidden()})
		}
	case *syntax.TypeAliasDecl:
		if !d.Expect {
			out = append(out, namedDecl{c: c, name: d.Name, pos: d.P, decl: d, private: d.Visibility == syntax.Private, hidden: d.Hidden()})
		}
	}
	return out
}

// redeclarations reports the properties and classifiers of one scope
// with the same name. Properties with the same name, and classifiers,
// are reported where they are declared.
func redeclarations(decls []namedDecl, reported map[any]bool) {
	byName := map[string][]int{}
	for i := range decls {
		byName[decls[i].name] = append(byName[decls[i].name], i)
	}
	for i := range decls {
		a := &decls[i]
		if !a.prop || a.entry {
			// Two classifiers of a name, or two enum entries: where they
			// are declared, the second is reported, and kotlinc reports
			// the first too.
			for _, j := range byName[a.name] {
				b := &decls[j]
				if j == i || b.prop != a.prop || b.entry != a.entry || a.hidden || b.hidden || a.c != b.c && a.c.file.Package != b.c.file.Package {
					continue
				}
				for _, o := range []*namedDecl{a, b} {
					if !reported[o.decl] && !o.c.hasErrorAt(o.pos, "redeclaration") {
						reported[o.decl] = true
						o.c.errorf(o.pos, "CLASSIFIER_REDECLARATION", "redeclaration: %s", o.name)
					}
				}
			}
			if !a.prop {
				continue
			}
		}
		for _, j := range byName[a.name] {
			b := &decls[j]
			if b.prop || a.hidden || b.hidden || a.c != b.c && (a.private || b.private) {
				continue
			}
			for _, o := range []*namedDecl{a, b} {
				if !reported[o.decl] {
					reported[o.decl] = true
					o.c.errorf(o.pos, "REDECLARATION", "redeclaration: %s", o.name)
				}
			}
		}
		if !a.entry {
			propRedeclarations(a, decls, byName[a.name], reported)
		}
	}
}

// propRedeclarations reports property a where it is declared if it
// conflicts with another of decls (by index is): an enum's entry, or a
// property of its file kc reported where that one is declared (see
// declare and declareClass), or one of another file of the package, where
// a private property conflicts with one that isn't and is reported alone.
func propRedeclarations(a *namedDecl, decls []namedDecl, is []int, reported map[any]bool) {
	for _, j := range is {
		b := &decls[j]
		if b == a || !b.prop || a.hidden || b.hidden {
			continue
		}
		msg := "conflicting declarations"
		switch {
		case b.entry:
			// An enum's entry and a property: both.
			msg = "redeclaration"
			if !reported[b.decl] && !b.c.hasErrorAt(b.pos, msg) {
				reported[b.decl] = true
				b.c.errorf(b.pos, "REDECLARATION", "%s: %s", msg, b.name)
			}
		case a.c == b.c:
			if !b.c.hasErrorAt(b.pos, "conflicting declarations") || !propsConflict(a.decl, b.decl) {
				continue
			}
		case a.private && b.private, !a.private && b.private, !propsConflict(a.decl, b.decl):
			continue
		}
		if !reported[a.decl] && !a.c.hasErrorAt(a.pos, "conflicting declarations") && !a.c.hasErrorAt(a.pos, "redeclaration") {
			reported[a.decl] = true
			a.c.errorf(a.pos, "REDECLARATION", "%s: %s", msg, a.name)
		}
	}
}

// propsConflict reports whether properties (or constructor parameters)
// a and b of a name conflict: their context parameters' types are the
// same, in any order if neither is hidden (see altProp).
func propsConflict(a, b any) bool {
	ctx := func(d any) ([]string, string, bool) {
		pd, ok := d.(*syntax.PropDecl)
		if !ok {
			return nil, "", false
		}
		var tps strings.Builder // the type parameters' bounds
		for _, tp := range pd.TypeParams {
			tps.WriteString("<")
			for _, b := range tp.Bounds {
				typeKey(&tps, b)
				tps.WriteString(",")
			}
		}
		ctx := contextTypes(pd)
		for i, q := range pd.Contexts {
			if q.Type.And != nil { // T & Any isn't T
				var b strings.Builder
				typeKey(&b, q.Type.And)
				ctx[i] += "&" + b.String()
			}
		}
		return ctx, tps.String(), pd.Hidden()
	}
	x, xt, xh := ctx(a)
	y, yt, yh := ctx(b)
	return xt == yt && (slices.Equal(x, y) || !xh && !yh && slices.Equal(sorted(x), sorted(y)))
}

// hasErrorAt reports whether the file has an error at p whose message
// starts with prefix.
func (c *Checker) hasErrorAt(p syntax.Pos, prefix string) bool {
	for _, e := range c.errs {
		if e.Pos == p && strings.HasPrefix(e.Msg, prefix) {
			return true
		}
	}
	return false
}

// programClass is a class of the program, by its Kotlin name.
type programClass struct {
	c      *Checker
	d      *syntax.ClassDecl
	kotlin string // its fully qualified Kotlin name
}

// programClasses returns the program's classes, but local ones and
// object expressions', by their JVM names.
func programClasses(checkers []*Checker) map[string][]programClass {
	out := map[string][]programClass{}
	var walk func(c *Checker, d *syntax.ClassDecl, kotlin string)
	walk = func(c *Checker, d *syntax.ClassDecl, kotlin string) {
		if d.Sym == nil || d.Expect {
			return
		}
		out[d.Sym.Name] = append(out[d.Sym.Name], programClass{c, d, kotlin})
		for _, m := range d.Members {
			if md, ok := m.(*syntax.ClassDecl); ok && !md.EnumEntry {
				walk(c, md, kotlin+"."+md.Name)
			}
		}
	}
	for _, c := range checkers {
		for _, d := range c.file.Decls {
			if d, ok := d.(*syntax.ClassDecl); ok {
				kotlin := d.Name
				if c.file.Package != "" {
					kotlin = c.file.Package + "." + d.Name
				}
				walk(c, d, kotlin)
			}
		}
	}
	return out
}

// classNames reports the classes of the program with the same JVM name,
// as class A { class B } and class `A$B`, and the packages that are named
// as classes: package a.b, and class b of package a or class a.
func classNames(checkers []*Checker) {
	classes := programClasses(checkers)
	kotlin := map[string]bool{}
	for _, cs := range classes {
		for _, pc := range cs {
			kotlin[pc.kotlin] = true
		}
		// Classes with the same Kotlin name are redeclarations.
		for i, pc := range cs[1:] {
			if pc.kotlin != cs[i].kotlin {
				for _, pc := range cs {
					pc.c.errorf(pc.d.P, "DUPLICATE_CLASS_NAMES", "duplicate JVM class name '%s'", pc.d.Sym.Name)
				}
				break
			}
		}
	}
	for _, c := range checkers {
		pkg := c.file.Package
		for n := pkg; n != ""; {
			if kotlin[n] {
				c.errorf(c.file.PackageP, "PACKAGE_CONFLICTS_WITH_CLASSIFIER", "package '%s' conflicts with class '%s'", pkg, n)
				break
			}
			i := strings.LastIndexByte(n, '.')
			if i < 0 {
				break
			}
			n = n[:i]
		}
	}
}

// localClassNames reports the local classes whose JVM names are those of
// other classes of the program, as a class B declared in an init block of
// class A, A$B, and a class B nested in A.
func localClassNames(checkers []*Checker) {
	var classes map[string][]programClass
	for _, c := range checkers {
		for sym := range c.localClasses {
			d, ok := sym.Decl.(*syntax.ClassDecl)
			if !ok || d.Sym != sym {
				continue // an earlier check's
			}
			if classes == nil {
				classes = programClasses(checkers)
			}
			if cs := classes[sym.Name]; len(cs) > 0 {
				c.errorf(d.P, "DUPLICATE_CLASS_NAMES", "duplicate JVM class name '%s'", sym.Name)
				for _, pc := range cs {
					pc.c.errorf(pc.d.P, "DUPLICATE_CLASS_NAMES", "duplicate JVM class name '%s'", sym.Name)
				}
			}
		}
	}
}

// hiddenFinalProp reports whether property d, declared beside prev and
// its alternatives, doesn't conflict with them as kotlinc has it: one of
// them is deprecated with level HIDDEN, and neither can be overridden.
func hiddenFinalProp(prev *types.VarSym, d *syntax.PropDecl) bool {
	if d.Open || d.Abstract || d.Override || len(d.Contexts) > 0 {
		return false
	}
	for q := range altProps(prev) {
		pd, ok := q.Decl.(*syntax.PropDecl)
		if !ok || !q.Final || q.Abstract || len(pd.Contexts) > 0 || !d.Hidden() && !pd.Hidden() {
			return false
		}
	}
	return true
}

// inheritedConflicts reports the functions of class d that conflict with
// those it inherits but doesn't override: as kotlinc has it, one that
// takes the same parameters as an inherited one with other type
// parameters, as fun some(s: String) and an inherited fun <T> some(s:
// String): T. Only the program's classes are looked at, and functions
// whose types don't mention type parameters.
func (c *Checker) inheritedConflicts(d *syntax.ClassDecl, reported map[any]bool) {
	if d.Sym.Super == nil && len(d.Sym.Interfaces) == 0 {
		return
	}
	c.inheritedTypeParamConflicts(d)
	for _, m := range d.Members {
		fd, ok := m.(*syntax.FunDecl)
		if !ok || fd.Sym == nil || fd.Expect || reported[fd] || fd.Sym.Hidden || fd.Hidden() {
			continue
		}
		a := funOverload(c, fd.Sym, fd.P, fd)
		if mentionsParams(&a) {
			continue
		}
		d.Sym.Supertypes(func(s *types.ClassSym) bool {
			if s == d.Sym || s.External || len(s.TypeParams) > 0 {
				return true
			}
			for _, g := range s.Methods[fd.Name] {
				if g.Private || g.Hidden || len(g.TypeParams) == len(fd.Sym.TypeParams) {
					continue
				}
				b := funOverload(c, g, fd.P, g)
				if mentionsParams(&b) || !sameTypes(&a, &b) {
					continue
				}
				reported[fd] = true
				c.u.conflicted[fd.Sym] = true
				c.errorf(fd.P, "CONFLICTING_OVERLOADS", "conflicting overloads: %s", fd.Name)
				return false
			}
			return true
		})
	}
}

// inheritedTypeParamConflicts reports class d if it inherits two
// functions of a name that take the same parameters but as many type
// parameters, which kotlinc reports as CONFLICTING_INHERITED_MEMBERS:
// fun a(arg: Int) of A and fun <T> a(arg: Int) of B in interface C : A,
// B. The class's own functions of the name take their place.
func (c *Checker) inheritedTypeParamConflicts(d *syntax.ClassDecl) {
	type inherited struct {
		g *types.FunSym
		o overload
	}
	byName := map[string][]inherited{}
	var names []string
	d.Sym.Supertypes(func(s *types.ClassSym) bool {
		if s == d.Sym || s.External || len(s.TypeParams) > 0 {
			return true
		}
		for name, gs := range s.Methods {
			if len(d.Sym.Methods[name]) > 0 {
				continue
			}
			for _, g := range gs {
				if g.Private || g.Hidden || g.Static || g.Name == "<init>" {
					continue
				}
				o := funOverload(c, g, d.P, g)
				if mentionsParams(&o) {
					continue
				}
				if byName[name] == nil {
					names = append(names, name)
				}
				byName[name] = append(byName[name], inherited{g, o})
			}
		}
		return true
	})
	slices.Sort(names) // the Methods maps' order isn't
	for _, name := range names {
		gs := byName[name]
	pairs:
		for i := range gs {
			for j := i + 1; j < len(gs); j++ {
				a, b := &gs[i], &gs[j]
				if a.g.Owner == b.g.Owner || len(a.g.TypeParams) == len(b.g.TypeParams) || !sameTypes(&a.o, &b.o) {
					continue
				}
				if !c.hasErrorAt(d.P, classKind(d.Sym)+" inherits conflicting members") {
					c.errorf(d.P, "CONFLICTING_INHERITED_MEMBERS", "%s inherits conflicting members '%s': functions that take the same parameters but not as many type parameters", classKind(d.Sym), name)
				}
				break pairs
			}
		}
	}
}

// mentionsParams reports whether the receiver, context parameters or
// parameters of o mention type parameters.
func mentionsParams(o *overload) bool {
	if o.recv != nil && o.recv.HasParams() {
		return true
	}
	for _, t := range o.ctx {
		if t.HasParams() {
			return true
		}
	}
	for _, t := range o.params {
		if t.HasParams() {
			return true
		}
	}
	return false
}

// sameTypes reports whether overloads a and b take the same receiver,
// context parameters and parameters, whatever their type parameters.
func sameTypes(a, b *overload) bool {
	if (a.recv == nil) != (b.recv == nil) || a.recv != nil && *a.recv != *b.recv ||
		len(a.ctx) != len(b.ctx) || len(a.params) != len(b.params) || a.vararg != b.vararg || a.suspend != b.suspend {
		return false
	}
	for i := range a.ctx {
		if a.ctx[i] != b.ctx[i] || a.ctx[i].K == types.Invalid {
			return false
		}
	}
	for i := range a.params {
		if a.params[i] != b.params[i] || a.params[i].K == types.Invalid {
			return false
		}
	}
	return true
}

// aliasedPropConflicts reports the properties of one scope, ovs, that
// kotlinc's frontend reports as redeclarations but kc's declarations let
// be: those declared beside each other because their context
// parameters' types differ as written (see altProp) but are the same
// types, as through a typealias, context(c: A) val p and context(c:
// AAlias) val p, or with type parameters of other names or bounds, and
// extension properties, which are declared apart. They conflict as
// functions do: by the same signature (sameSignature) or, as in K2,
// each being as specific as the other (mutuallySpecific). Hidden ones
// don't, whose accessors may clash only on the JVM.
func aliasedPropConflicts(ovs []overload) {
	for i := range ovs {
		// A generic property's type parameters, which an extension's
		// overload has already.
		if o := &ovs[i]; o.recv == nil && len(o.tparams) == 0 {
			if d, ok := o.decl.(*syntax.PropDecl); ok && d.Sym != nil && d.Sym.Getter != nil {
				o.tparams = d.Sym.Getter.TypeParams
			}
		}
	}
	for i := range ovs {
		a := &ovs[i]
		ad, ok := a.decl.(*syntax.PropDecl)
		if !ok || !a.prop || ad.Sym == nil || a.hidden || a.recv == nil && ad.Sym.Alt == nil {
			continue
		}
		for j := range ovs {
			b := &ovs[j]
			bd, ok := b.decl.(*syntax.PropDecl)
			if j == i || !ok || !b.prop || b.name != a.name || bd.Sym == nil || b.hidden || (a.recv == nil) != (b.recv == nil) ||
				a.c != b.c && (a.private || b.private) ||
				a.recv == nil && !slices.Contains(slices.Collect(altProps(ad.Sym)), bd.Sym) || !sameSignature(a, b) && !mutuallySpecific(a, b) {
				continue
			}
			// The alt chain runs from the later property to the earlier,
			// so report both of the pair here.
			for _, o := range []*overload{a, b} {
				if !o.c.hasErrorAt(o.pos, "conflicting declarations") {
					o.c.errorf(o.pos, "REDECLARATION", "conflicting declarations: %s", o.name)
				}
			}
			break
		}
	}
}
