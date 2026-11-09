package check

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Completion: an editor asks what the code may name at its caret. The
// editor replaces the name being typed there, or nothing, by ProbeName,
// as IntelliJ inserts its dummy identifier, and checks the file with a
// ScopeRecorder: where the checker meets the probe, as a name, a member,
// a callable reference or a type, it tells the recorder what is in scope
// there, a ScopeView, and stops checking, with ErrProbed.
//
// The view lists by name what the code can name, without choosing among
// overloads: the editor filters them by what was typed.

// ProbeName is the name an editor puts at its caret: one no code has.
const ProbeName = "__kc_probe__"

// ErrProbed is CheckOptions's error once it has reported the probe.
var ErrProbed = errors.New("check: stopped at the completion probe")

// A ScopeRecorder is a Recorder that is told what is in scope at the
// probe, once checking reaches it.
type ScopeRecorder interface {
	Scope(v *ScopeView)
}

// probeStop is the panic that stops checking at the probe.
type probeStop struct{}

// Origin says where an Item comes from.
type Origin uint8

const (
	Local         Origin = iota // a local variable, parameter, function or class
	Member                      // a member of a receiver
	Extension                   // an extension for a receiver
	TopLevel                    // a top-level function or property in scope
	Classifier                  // a class or typealias in scope
	TypeParameter               // a type parameter in scope
	PackageName                 // a package, after a package's name
	Static                      // a static member, enum entry or nested class, after a class's name
)

// Item is one thing the probe may name.
type Item struct {
	Name   string
	Origin Origin
	// One of these is set.
	Var   *types.VarSym
	Fun   *types.FunSym // a function, or the getter of a property
	Class *types.ClassSym
	Alias *types.AliasSym
	Param *types.TypeParamSym
	// Package is a package's dotted name, for PackageName.
	Package string
	// Prop is set when Fun is a property's getter: a Kotlin class's from
	// metadata, an extension property's, or a Java getter, which Kotlin
	// reads as a property named Name.
	Prop bool
	// Type is the value's type, smart casts and the receiver's type
	// arguments applied, or a function's return type.
	Type types.Type
	// Subst maps the type parameters of a member's class to the
	// receiver's type arguments.
	Subst types.Subst
	// Receiver is the index in ScopeView.Receivers of the implicit
	// receiver whose member or extension it is, or -1.
	Receiver int
	// Deprecated is set on a library declaration marked deprecated.
	Deprecated bool
}

// CallSite is a call the probe is an argument of.
type CallSite struct {
	// Funs are the functions and constructors the call may call, by its
	// name.
	Funs []*types.FunSym
	// Arg is the index of the argument the probe is in, and Name the
	// argument's name, if it is a named one.
	Arg  int
	Name string
	// Names are the names of the call's named arguments.
	Names []string
}

// ScopeView is what is in scope at the probe.
type ScopeView struct {
	// Expected is the type expected of the probe's value, or InvalidType.
	Expected types.Type
	// TypePosition is set when the probe is a type: Items are then only
	// classifiers, type parameters and packages.
	TypePosition bool
	// Callable is set on a callable reference: x::probe or ::probe.
	Callable bool
	// Receiver is the explicit receiver's type: x in x.probe; nil for
	// none. Safe is set for x?.probe.
	Receiver *types.Type
	Safe     bool
	// StaticOf is set when the explicit receiver is a class's name: its
	// static members, enum entries, nested classes and companion's
	// members are Items.
	StaticOf *types.ClassSym
	// Package is set when the explicit receiver is a package's name.
	Package string
	// Receivers are the implicit receivers, innermost first: lambdas'
	// receivers, then this, the outer classes' this, and the objects in
	// scope.
	Receivers []types.Type
	Items     []Item
	// Calls are the calls whose argument the probe is in, innermost
	// first.
	Calls []CallSite
}

// isProbe reports whether x is the probe, or a call or callable
// reference of it.
func isProbe(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Name == ProbeName
	case *syntax.Member:
		return x.Name == ProbeName
	case *syntax.Call:
		return !x.Invoke && isProbe(x.Fun)
	case *syntax.Lambda:
		return x.Ref != nil && x.Ref.Name == ProbeName
	}
	return false
}

// probe reports the scope at x, the probe, and stops checking.
func (c *Checker) probe(x syntax.Expr, want types.Type) {
	v := &ScopeView{Expected: want}
	if call, ok := x.(*syntax.Call); ok {
		x = call.Fun
	}
	switch x := x.(type) {
	case *syntax.Ident:
		c.scopeItems(v, "")
	case *syntax.Member:
		v.Safe = x.Safe
		c.qualifiedItems(v, x.X)
	case *syntax.Lambda:
		v.Callable = true
		switch {
		case x.Ref.Type != nil:
			t := c.resolveType(x.Ref.Type)
			v.Receiver = &t
			c.memberItems(v, t, -1, "")
			c.extensionItems(v, t, -1, c.visibleNames(), "")
		case x.Ref.X != nil:
			if cls := c.classRef(x.Ref.X); cls != nil {
				t := cls.This()
				v.Receiver = &t
				c.memberItems(v, t, -1, "")
				c.extensionItems(v, t, -1, c.visibleNames(), "")
			} else {
				c.qualifiedItems(v, x.Ref.X)
			}
		default:
			c.scopeItems(v, "")
		}
	}
	c.callSites(v, x.Pos())
	c.report(v)
}

// probeType reports the scope at ref, a type named the probe, or
// qualified by a name: a.b.probe.
func (c *Checker) probeType(ref *syntax.TypeRef) {
	v := &ScopeView{Expected: types.InvalidType, TypePosition: true}
	if q, ok := strings.CutSuffix(ref.Name, "."+ProbeName); ok {
		if k := c.findClassifier(q); k.cls != nil {
			v.StaticOf = k.cls
			c.nestedItems(v, k.cls, "")
		} else {
			v.Package = q
			c.packageItems(v, q, true, "")
		}
	} else {
		for sc := c.scope; sc != nil; sc = sc.parent {
			for _, k := range sc.classes {
				c.add(v, Item{Name: k.Decl.(*syntax.ClassDecl).Name, Origin: Local, Class: k, Receiver: -1})
			}
		}
		c.typeParamItems(v)
		c.classifierItems(v, "")
		c.rootPackages(v)
	}
	c.report(v)
}

// report gives v to the recorder and stops checking.
func (c *Checker) report(v *ScopeView) {
	c.u.scopeRec.Scope(v)
	panic(probeStop{})
}

// add adds item it to v.
func (c *Checker) add(v *ScopeView, it Item) {
	if it.Name == "" || strings.ContainsAny(it.Name, "<$") || it.Name == "_" || it.Name == ProbeName {
		return
	}
	if it.Fun != nil {
		if it.Fun.Hidden {
			return
		}
		it.Deprecated = it.Fun.Deprecated != nil
	} else if it.Class != nil {
		it.Deprecated = it.Class.Deprecated != nil
	}
	v.Items = append(v.Items, it)
}

// scopeItems adds what a simple name may name at the probe; only those
// named only, if it isn't "".
func (c *Checker) scopeItems(v *ScopeView, only string) {
	shadowed := map[string]bool{}
	for sc := c.scope; sc != nil; sc = sc.parent {
		for i := len(sc.vars) - 1; i >= 0; i-- {
			l := sc.vars[i]
			if shadowed[l.Name] || only != "" && l.Name != only {
				continue
			}
			shadowed[l.Name] = true
			c.add(v, Item{Name: l.Name, Origin: Local, Var: l, Type: sc.narrowed(l), Receiver: -1})
		}
		for _, f := range sc.funs {
			if only == "" || f.Name == only {
				c.add(v, Item{Name: f.Name, Origin: Local, Fun: f, Type: f.Ret, Receiver: -1})
			}
		}
		for _, k := range sc.classes {
			if name := k.Decl.(*syntax.ClassDecl).Name; only == "" || name == only {
				c.add(v, Item{Name: name, Origin: Local, Class: k, Receiver: -1})
			}
		}
	}
	v.Receivers = c.implicitReceivers()
	names := c.visibleNames()
	for i, rt := range v.Receivers {
		c.memberItems(v, rt, i, only)
	}
	for i, rt := range v.Receivers {
		c.extensionItems(v, rt, i, names, only)
	}
	c.topLevelItems(v, names, only)
	if only == "" {
		c.typeParamItems(v)
		c.classifierItems(v, "")
		c.rootPackages(v)
	} else {
		c.classifierItems(v, only)
	}
}

// implicitReceivers returns the types of the implicit receivers at the
// code, innermost first, smart casts applied.
func (c *Checker) implicitReceivers() []types.Type {
	var out []types.Type
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if c.scope != nil {
			out = append(out, c.scope.narrowed(c.recvs[i].sym))
		} else {
			out = append(out, c.recvs[i].sym.Type)
		}
	}
	inner := true
	for _, k := range c.enclosingClasses() {
		if !inner {
			break
		}
		switch {
		case k == c.cls && c.this != nil:
			t, _ := c.narrowedThis()
			out = append(out, t)
		case k == c.cls:
			out = append(out, k.This())
		case k.Object:
			// scopeObjects has it.
		default:
			out = append(out, k.This())
		}
		inner = k.Inner || k.Object && k != c.cls || k.Anonymous
	}
	for _, obj := range c.scopeObjects() {
		out = append(out, types.ClassType(obj))
	}
	return out
}

// receiverClass returns the class whose members a receiver of type t has:
// that of a built-in type's values too.
func (c *Checker) receiverClass(t types.Type) *types.ClassSym {
	t = t.Flex().NonNull()
	if cls := c.classOf(t); cls != nil {
		return cls
	}
	if t.K == types.String || t.IsPrimitive() {
		return c.jvmClass(t)
	}
	return nil
}

// memberItems adds the members of a receiver of type rt, the recv'th
// implicit one or -1, named only if it isn't "".
func (c *Checker) memberItems(v *ScopeView, rt types.Type, recv int, only string) {
	if id := builtinID(rt.Flex().NonNull()); id != "" && c.u.ext.kotlin {
		// Kotlin's own members, not its Java class's: no String.charAt.
		items := c.u.ext.builtinItems(id)
		if id != "kotlin/Any" {
			for _, it := range c.u.ext.builtinItems("kotlin/Any") {
				if !slices.ContainsFunc(items, func(m Item) bool { return m.Name == it.Name }) {
					items = append(slices.Clip(items), it)
				}
			}
		}
		for _, it := range items {
			if only == "" || it.Name == only {
				it.Receiver = recv
				c.add(v, it)
			}
		}
		return
	}
	cls := c.receiverClass(rt)
	if cls == nil {
		return
	}
	rt = rt.Flex().NonNull()
	seen := map[string]bool{}
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s.Name == "java/lang/Object" {
			// Kotlin's Any has only these.
			for _, name := range []string{"equals", "hashCode", "toString"} {
				for _, f := range s.Methods[name] {
					if k := "f " + name + " " + paramKey(f); !seen[k] && !f.Static && (only == "" || only == name) {
						seen[k] = true
						c.add(v, Item{Name: name, Origin: Member, Fun: f, Type: f.Ret, Receiver: recv})
					}
				}
			}
			return true
		}
		sub := c.memberSubst(rt, s)
		prop := func(name string, it Item) {
			if only != "" && name != only || seen["p "+name] {
				return
			}
			seen["p "+name] = true
			it.Name, it.Origin, it.Receiver, it.Subst = name, Member, recv, sub
			c.add(v, it)
		}
		for _, p := range s.Props {
			if p.InProgress || !c.propVisible(p) {
				continue
			}
			ensureVar(p)
			prop(p.Name, Item{Var: p, Type: sub.Apply(p.Type)})
		}
		for _, name := range slices.Sorted(maps.Keys(s.KProps)) {
			if g := s.KProps[name].Getter; g != nil && c.funVisible(g) {
				prop(name, Item{Fun: g, Prop: true, Type: sub.Apply(g.Ret)})
			}
		}
		for _, name := range slices.Sorted(maps.Keys(s.JavaFields)) {
			if f := s.JavaFields[name]; c.visibleField(f) {
				prop(name, Item{Var: f, Type: sub.Apply(f.Type)})
			}
		}
		for _, name := range slices.Sorted(maps.Keys(s.Methods)) {
			for _, f := range s.Methods[name] {
				if f.Static || f.Name == "<init>" || f.InProgress || !c.funVisible(f) {
					continue
				}
				ensureFun(f)
				if s.External && s.KotlinName == "" {
					if p, ok := getterProp(f); ok {
						prop(p, Item{Fun: f, Prop: true, Type: sub.Apply(f.Ret)})
					}
				}
				if only != "" && f.Name != only {
					continue
				}
				k := "f " + f.Name + " " + paramKey(f)
				if seen[k] {
					continue
				}
				seen[k] = true
				c.add(v, Item{Name: f.Name, Origin: Member, Fun: f, Type: sub.Apply(f.Ret), Subst: sub, Receiver: recv})
			}
		}
		return true
	})
}

// builtinID returns the class id of a built-in type whose values'
// members are Kotlin's, not its Java class's, or "".
func builtinID(t types.Type) string {
	switch t.K {
	case types.String:
		return "kotlin/String"
	case types.Int:
		return "kotlin/Int"
	case types.Long:
		return "kotlin/Long"
	case types.Short:
		return "kotlin/Short"
	case types.Byte:
		return "kotlin/Byte"
	case types.Char:
		return "kotlin/Char"
	case types.Bool:
		return "kotlin/Boolean"
	case types.Float:
		return "kotlin/Float"
	case types.Double:
		return "kotlin/Double"
	case types.Any:
		return "kotlin/Any"
	}
	return ""
}

// builtinItems returns the members of the built-in class id, as Items,
// read from the standard library's builtins once.
func (t *classTable) builtinItems(id string) []Item {
	t.builtinMembersMu.Lock()
	defer t.builtinMembersMu.Unlock()
	if items, ok := t.builtinMembers[id]; ok {
		return items
	}
	var items []Item
	if kc := t.builtinClasses()[id]; kc != nil {
		for _, p := range kc.Props {
			pt, ok := t.ktype(p.Type, tscope{})
			if !ok || p.Flags.Visibility() != kmeta.Public {
				continue
			}
			items = append(items, Item{Name: p.Name, Origin: Member, Var: &types.VarSym{Name: p.Name, Type: pt, Kind: types.Prop}, Type: pt})
		}
		for _, f := range kc.Funcs {
			if f.Flags.Visibility() != kmeta.Public {
				continue
			}
			sym := t.kfun(f, nil, tscope{}, nil)
			if sym.Unsupported {
				continue
			}
			items = append(items, Item{Name: f.Name, Origin: Member, Fun: sym, Type: sym.Ret})
		}
	}
	if t.builtinMembers == nil {
		t.builtinMembers = map[string][]Item{}
	}
	t.builtinMembers[id] = items
	return items
}

// getterProp returns the name of the property Kotlin reads through a
// Java getter: x of getX(), isX of isX().
func getterProp(f *types.FunSym) (string, bool) {
	if len(f.Params) > 0 || len(f.TypeParams) > 0 || f.Ret.K == types.Unit {
		return "", false
	}
	for _, prefix := range []string{"get", "is"} {
		rest, ok := strings.CutPrefix(f.Name, prefix)
		if !ok || rest == "" || rest[0] < 'A' || rest[0] > 'Z' {
			continue
		}
		if prefix == "is" {
			if f.Ret.Flex().NonNull() != types.BoolType {
				return "", false
			}
			return f.Name, true
		}
		return decapitalize(rest), true
	}
	return "", false
}

// decapitalize lowers a getter's name's first letters as Kotlin does:
// url of getURL, name of getName.
func decapitalize(s string) string {
	n := 0
	for n < len(s) && s[n] >= 'A' && s[n] <= 'Z' {
		n++
	}
	switch {
	case n == len(s):
		return strings.ToLower(s)
	case n > 1:
		n-- // URLConnection: urlConnection
	}
	return strings.ToLower(s[:n]) + s[n:]
}

// names are the names of top-level and extension functions and
// properties the code can see, sorted, by the scopes that may declare
// them.
type names struct {
	funs, props []string
}

// visibleNames returns the names topLevelItems and extensionItems look
// up: those of the program's packages and imports, and of the class
// path's star and default imports.
func (c *Checker) visibleNames() names {
	funs, props := map[string]bool{}, map[string]bool{}
	addKeys := func(set map[string]bool, keys ...[]string) {
		for _, ks := range keys {
			for _, k := range ks {
				set[k] = true
			}
		}
	}
	addKeys(funs, slices.Collect(maps.Keys(c.userFuns)), slices.Collect(maps.Keys(c.funImports)), slices.Collect(maps.Keys(c.pkg.Funs)), slices.Collect(maps.Keys(builtins)))
	addKeys(props, slices.Collect(maps.Keys(c.userVars)), slices.Collect(maps.Keys(c.userExtProps)), slices.Collect(maps.Keys(c.pkg.Vars)),
		slices.Collect(maps.Keys(c.pkg.ExtProps)), slices.Collect(maps.Keys(c.fileVars)))
	for name := range c.memberImports {
		funs[name], props[name] = true, true
	}
	for _, p := range c.userStars {
		addKeys(funs, slices.Collect(maps.Keys(p.Funs)))
		addKeys(props, slices.Collect(maps.Keys(p.Vars)), slices.Collect(maps.Keys(p.ExtProps)))
	}
	for _, k := range c.enclosingClasses() {
		addKeys(funs, slices.Collect(maps.Keys(k.ExtMethods)))
		addKeys(props, slices.Collect(maps.Keys(k.ExtProps)))
	}
	if ext := c.u.ext; ext.kotlin {
		pkgs := slices.Clone(defaultPackages)
		for _, s := range c.stars {
			pkgs = append(pkgs, strings.ReplaceAll(s, "/", "."))
		}
		for _, pkg := range pkgs {
			if p := ext.pkg(pkg); p != nil {
				for n := range p.names {
					funs[n], props[n] = true, true
				}
			}
		}
	}
	return names{slices.Sorted(maps.Keys(funs)), slices.Sorted(maps.Keys(props))}
}

// extensionItems adds the extension functions and properties a receiver
// of type rt may call, the recv'th implicit one or -1, named only if it
// isn't "".
func (c *Checker) extensionItems(v *ScopeView, rt types.Type, recv int, ns names, only string) {
	rt = rt.Flex()
	if rt.K == types.Invalid {
		return
	}
	// A member hides an extension of its name and parameters.
	members := map[string]bool{}
	for _, it := range v.Items {
		if it.Origin == Member && it.Receiver == recv {
			if it.Fun != nil && !it.Prop {
				members["f "+it.Name+" "+paramTypes(it.Fun)] = true
			} else {
				members["p "+it.Name] = true
			}
		}
	}
	for _, name := range ns.funs {
		if only != "" && name != only {
			continue
		}
		for _, cl := range c.extCallees(name, rt) {
			if !c.hiddenFun(cl.f) && c.extFits(rt, cl.f.Recv) && !members["f "+name+" "+paramTypes(cl.f)] {
				ensureFun(cl.f)
				c.add(v, Item{Name: name, Origin: Extension, Fun: cl.f, Type: cl.f.Ret, Receiver: recv})
			}
		}
	}
	for _, name := range ns.props {
		if only != "" && name != only {
			continue
		}
		if g := c.extPropFor(name, rt); g != nil && !members["p "+name] {
			c.add(v, Item{Name: name, Origin: Extension, Fun: g, Prop: true, Type: g.Ret, Receiver: recv})
		}
	}
}

// extFits reports whether an extension whose receiver type is r, which
// recvFits rt, may take a receiver of type rt, as far as a list of
// completions needs to tell: a type parameter's bounds must be its
// supertypes, whatever their type arguments, so a String has
// T.coerceAtLeast for T : Comparable<T>, but a class that isn't
// Comparable hasn't.
func (c *Checker) extFits(rt, r types.Type) bool {
	if !r.IsParam() {
		return c.recvTakes(rt, r)
	}
	rn := rt.Flex().NonNull()
	if rn.IsParam() || rn.K == types.TypeVar {
		return true
	}
	for _, b := range r.Param().Bounds {
		switch bn := b.Flex().NonNull(); bn.K {
		case types.Class:
			if _, ok := c.asSuper(rn, bn.C); !ok {
				return false
			}
		case types.Any, types.TypeParam, types.TypeVar, types.Invalid:
		default:
			if bn.K != rn.K {
				return false
			}
		}
	}
	return true
}

// paramTypes describes f's parameters, for an extension's that a
// member's hide: as far as a list of completions needs, their number.
func paramTypes(f *types.FunSym) string {
	return strconv.Itoa(len(f.Params))
}

// extPropFor returns the getter of the extension property named name that
// a receiver of type rt reads, by extProp's scopes, or nil.
func (c *Checker) extPropFor(name string, rt types.Type) *types.FunSym {
	var best *types.FunSym
	fits := func(gs []*types.FunSym) {
		for _, g := range gs {
			if best != nil {
				return
			}
			if g.InProgress {
				continue
			}
			ensureFun(g)
			if c.recvFits(rt, g.Recv) && c.extFits(rt, g.Recv) && c.funVisible(g) {
				best = g
			}
		}
	}
	for _, k := range c.enclosingClasses() {
		fits(memberExtProps(k, name))
	}
	fits(c.userExtProps[name])
	fits(c.pkg.ExtProps[name])
	for _, p := range c.userStars {
		fits(p.ExtProps[name])
	}
	if ext := c.u.ext; ext.kotlin {
		for _, s := range c.stars {
			fits(ext.pkgExtProps(strings.ReplaceAll(s, "/", "."), name))
		}
		for _, pkg := range defaultPackages {
			fits(ext.pkgExtProps(pkg, name))
		}
	}
	return best
}

// topLevelItems adds the top-level functions and properties a simple name
// may name, named only if it isn't "".
func (c *Checker) topLevelItems(v *ScopeView, ns names, only string) {
	seenVar := map[*types.VarSym]bool{}
	for _, it := range v.Items {
		if it.Var != nil {
			seenVar[it.Var] = true
		}
	}
	for _, name := range ns.funs {
		if only != "" && name != only {
			continue
		}
		var fs []*types.FunSym
		if f := builtins[name]; f != nil && c.u.ext.cp == nil {
			fs = append(fs, f)
		}
		fs = append(fs, c.topFuns(name)...)
		fs = append(fs, c.kotlinFuns(name, false)...)
		if mi, ok := c.memberImports[name]; ok {
			mi.owner.Ensure()
			fs = append(fs, mi.owner.Statics[mi.name]...)
			if mi.owner.Object {
				fs = append(fs, mi.owner.Methods[mi.name]...)
			}
		}
		for _, f := range fs {
			if f.Ext || c.hiddenFun(f) || f.InProgress {
				continue
			}
			ensureFun(f)
			c.add(v, Item{Name: name, Origin: TopLevel, Fun: f, Type: f.Ret, Receiver: -1})
		}
	}
	for _, name := range ns.props {
		if only != "" && name != only {
			continue
		}
		if l, _ := c.scope.find(name); l != nil {
			continue // a local hides it
		}
		sym := c.topVar(name)
		if sym == nil {
			if mi, ok := c.memberImports[name]; ok {
				mi.owner.Ensure()
				sym = mi.owner.Fields[mi.name]
			}
		}
		if sym == nil || seenVar[sym] || sym.InProgress {
			continue
		}
		ensureVar(sym)
		c.add(v, Item{Name: name, Origin: TopLevel, Var: sym, Type: sym.Type, Receiver: -1})
	}
}

// typeParamItems adds the type parameters in scope.
func (c *Checker) typeParamItems(v *ScopeView) {
	seen := map[string]bool{}
	ps := slices.Clone(c.tparams)
	for k := c.lookupCls; k != nil; k = k.Outer {
		ps = append(ps, k.TypeParams...)
	}
	for _, p := range ps {
		if !seen[p.Name] {
			seen[p.Name] = true
			c.add(v, Item{Name: p.Name, Origin: TypeParameter, Param: p, Receiver: -1})
		}
	}
}

// classifierItems adds the classes and typealiases a simple name may
// name, named only if it isn't "": as findClassifier finds them.
func (c *Checker) classifierItems(v *ScopeView, only string) {
	cands := map[string]bool{}
	addKeys := func(ks []string) {
		for _, k := range ks {
			if only == "" || k == only {
				cands[k] = true
			}
		}
	}
	for _, k := range c.enclosingClasses() {
		addKeys(slices.Collect(maps.Keys(k.Nested)))
		addKeys(slices.Collect(maps.Keys(k.Aliases)))
	}
	addKeys(slices.Collect(maps.Keys(c.imports)))
	addKeys(slices.Collect(maps.Keys(c.aliasImports)))
	addKeys(slices.Collect(maps.Keys(c.pkg.Classes)))
	addKeys(slices.Collect(maps.Keys(c.pkg.Aliases)))
	for _, p := range c.userStars {
		addKeys(slices.Collect(maps.Keys(p.Classes)))
		addKeys(slices.Collect(maps.Keys(p.Aliases)))
	}
	if only != "" {
		cands[only] = true
	} else if ext := c.u.ext; ext.cp != nil {
		for name := range kotlinTypes {
			cands[name] = true
		}
		for _, id := range ext.builtinIDs() {
			if pkg, name := splitID(id); slices.Contains(defaultPackages, pkg) {
				cands[name] = true
			}
		}
		pkgs := slices.Clone(c.stars)
		for _, p := range defaultPackages {
			pkgs = append(pkgs, strings.ReplaceAll(p, ".", "/"))
		}
		pkgs = append(pkgs, "java/lang")
		for _, pkg := range pkgs {
			addKeys(ext.classNames(pkg))
			if p := ext.pkg(strings.ReplaceAll(pkg, "/", ".")); p != nil {
				for _, kp := range p.parts {
					_, _, aliases := kp.meta.Names()
					addKeys(aliases)
				}
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cands)) {
		if kotlinTypes[name] {
			if t, ok := kotlinTypeItem(name); ok {
				c.add(v, Item{Name: name, Origin: Classifier, Type: t, Receiver: -1})
			}
			continue
		}
		k := c.findClassifier(name)
		switch {
		case k.cls != nil && c.canName(k.cls):
			c.add(v, Item{Name: name, Origin: Classifier, Class: k.cls, Receiver: -1})
		case k.alias != nil:
			c.add(v, Item{Name: name, Origin: Classifier, Alias: k.alias, Receiver: -1})
		}
	}
}

// kotlinTypeItem returns the type a built-in type's name names.
func kotlinTypeItem(name string) (types.Type, bool) {
	switch name {
	case "Int":
		return types.IntType, true
	case "Long":
		return types.LongType, true
	case "Short":
		return types.ShortType, true
	case "Byte":
		return types.ByteType, true
	case "Char":
		return types.CharType, true
	case "Boolean":
		return types.BoolType, true
	case "Float":
		return types.FloatType, true
	case "Double":
		return types.DoubleType, true
	case "String":
		return types.StringType, true
	case "Unit":
		return types.UnitType, true
	case "Any":
		return types.AnyType, true
	case "Nothing":
		return types.NothingType, true
	}
	return types.Type{}, false
}

// canName reports whether the code may name cls: not another file's
// private class, nor a class path's class that isn't public.
func (c *Checker) canName(cls *types.ClassSym) bool {
	if c.privateElsewhere(classifier{cls: cls}) || !c.classVisible(cls) {
		return false
	}
	if cls.External {
		cls.Ensure()
		return !cls.Private && !cls.Missing
	}
	return true
}

// rootPackages adds the top-level packages: kotlin, java.
func (c *Checker) rootPackages(v *ScopeView) {
	seen := map[string]bool{}
	for name := range c.u.Pkgs {
		if root, _, _ := strings.Cut(name, "."); root != "" {
			seen[root] = true
		}
	}
	if ext := c.u.ext; ext.cp != nil {
		for _, p := range ext.cp.Packages() {
			if root, _, _ := strings.Cut(p, "/"); root != "" {
				seen[root] = true
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(seen)) {
		c.add(v, Item{Name: name, Origin: PackageName, Package: name, Receiver: -1})
	}
}

// qualifiedItems adds what x.probe may name: x a class, a package or a
// value.
func (c *Checker) qualifiedItems(v *ScopeView, x syntax.Expr) {
	if cls := c.classRef(x); cls != nil {
		v.StaticOf = cls
		c.staticItems(v, cls, "")
		return
	}
	if name := dottedName(x); name != "" && !c.isValueName(x) {
		if c.u.isPackage(name) || c.u.ext.hasPackage(strings.ReplaceAll(name, ".", "/")) {
			v.Package = name
			c.packageItems(v, name, false, "")
			return
		}
	}
	t := c.value(x, noWant)
	if t.K == types.Invalid {
		return
	}
	v.Receiver = &t
	c.memberItems(v, t, -1, "")
	c.extensionItems(v, t.NonNull(), -1, c.visibleNames(), "")
}

// isValueName reports whether x starts with a name of a value: a local,
// or a property in scope.
func (c *Checker) isValueName(x syntax.Expr) bool {
	for {
		switch y := x.(type) {
		case *syntax.Member:
			x = y.X
			continue
		case *syntax.Ident:
			return c.isVar(y.Name)
		}
		return false
	}
}

// dottedName returns the dotted name x is, a.b.c, or "".
func dottedName(x syntax.Expr) string {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Name
	case *syntax.Member:
		if q := dottedName(x.X); q != "" && !x.Safe {
			return q + "." + x.Name
		}
	}
	return ""
}

// staticItems adds what C.probe names, C the class cls: its static
// members, its enum entries, its nested classes, and its companion's or
// its own members, if it is an object; named only if it isn't "".
func (c *Checker) staticItems(v *ScopeView, cls *types.ClassSym, only string) {
	cls.Ensure()
	for _, name := range cls.Entries {
		if only == "" || name == only {
			c.add(v, Item{Name: name, Origin: Static, Var: cls.Fields[name], Type: types.ClassType(cls), Receiver: -1})
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cls.Statics)) {
		for _, f := range cls.Statics[name] {
			if (only == "" || name == only) && c.funVisible(f) {
				c.add(v, Item{Name: name, Origin: Static, Fun: f, Type: f.Ret, Receiver: -1})
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cls.Fields)) {
		f := cls.Fields[name]
		if (only == "" || name == only) && !slices.Contains(cls.Entries, name) && c.visibleField(f) {
			c.add(v, Item{Name: name, Origin: Static, Var: f, Type: f.Type, Receiver: -1})
		}
	}
	c.nestedItems(v, cls, only)
	if obj := objectOf(cls); obj != nil {
		c.memberItems(v, types.ClassType(obj), -1, only)
		c.extensionItems(v, types.ClassType(obj), -1, c.visibleNames(), only)
	}
}

// nestedItems adds cls's nested classes, named only if it isn't "".
func (c *Checker) nestedItems(v *ScopeView, cls *types.ClassSym, only string) {
	for _, name := range slices.Sorted(maps.Keys(cls.Nested)) {
		if only == "" || name == only {
			c.add(v, Item{Name: name, Origin: Static, Class: cls.Nested[name], Receiver: -1})
		}
	}
	if cls.External && c.u.ext.cp != nil {
		pkg, simple := "", cls.Name
		if i := strings.LastIndexByte(cls.Name, '/'); i >= 0 {
			pkg, simple = cls.Name[:i], cls.Name[i+1:]
		}
		for _, n := range c.u.ext.cp.Nested(pkg, simple) {
			name := n[strings.LastIndexByte(n, '$')+1:]
			if only != "" && name != only || name == "" || name[0] >= '0' && name[0] <= '9' || name == "DefaultImpls" || name == "WhenMappings" {
				continue
			}
			if k := c.u.ext.class(n); k != nil && c.canName(k) {
				c.add(v, Item{Name: name, Origin: Static, Class: k, Receiver: -1})
			}
		}
	}
}

// packageItems adds what pkg.probe names: pkg's subpackages and classes,
// and, unless types only, its functions and properties.
func (c *Checker) packageItems(v *ScopeView, pkg string, typesOnly bool, only string) {
	subs := map[string]bool{}
	for name := range c.u.Pkgs {
		if rest, ok := strings.CutPrefix(name, pkg+"."); ok {
			subs[strings.SplitN(rest, ".", 2)[0]] = true
		}
	}
	ext := c.u.ext
	internal := strings.ReplaceAll(pkg, ".", "/")
	if ext.cp != nil {
		for _, p := range ext.cp.Packages() {
			if rest, ok := strings.CutPrefix(p, internal+"/"); ok {
				subs[strings.SplitN(rest, "/", 2)[0]] = true
			}
		}
	}
	for _, s := range slices.Sorted(maps.Keys(subs)) {
		if only == "" || s == only {
			c.add(v, Item{Name: s, Origin: PackageName, Package: pkg + "." + s, Receiver: -1})
		}
	}
	if p := c.u.Pkgs[pkg]; p != nil {
		for _, name := range slices.Sorted(maps.Keys(p.Classes)) {
			if k := p.Classes[name]; (only == "" || name == only) && c.canName(k) {
				c.add(v, Item{Name: name, Origin: Static, Class: k, Receiver: -1})
			}
		}
		if !typesOnly {
			for _, name := range slices.Sorted(maps.Keys(p.Funs)) {
				for _, f := range p.Funs[name] {
					if (only == "" || name == only) && !f.Ext && c.funVisible(f) {
						ensureFun(f)
						c.add(v, Item{Name: name, Origin: Static, Fun: f, Type: f.Ret, Receiver: -1})
					}
				}
			}
			for _, name := range slices.Sorted(maps.Keys(p.Vars)) {
				if only == "" || name == only {
					sym := p.Vars[name]
					ensureVar(sym)
					c.add(v, Item{Name: name, Origin: Static, Var: sym, Type: sym.Type, Receiver: -1})
				}
			}
		}
	}
	if ext.cp == nil {
		return
	}
	for _, name := range ext.classNames(internal) {
		if only != "" && name != only {
			continue
		}
		if k := ext.class(internal + "/" + name); k != nil && c.canName(k) {
			c.add(v, Item{Name: name, Origin: Static, Class: k, Receiver: -1})
		}
	}
	if !ext.kotlin {
		return
	}
	for _, id := range ext.builtinIDs() {
		if p, name := splitID(id); p == pkg && (only == "" || name == only) {
			if k := c.findQualified(pkg, name); k != nil {
				c.add(v, Item{Name: name, Origin: Static, Class: k, Receiver: -1})
			}
		}
	}
	if kp := ext.pkg(pkg); kp != nil {
		for _, name := range slices.Sorted(maps.Keys(kp.names)) {
			if only != "" && name != only {
				continue
			}
			if k := ext.pkgClass(pkg, name); k != nil {
				c.add(v, Item{Name: name, Origin: Static, Class: k, Receiver: -1})
			} else if a := ext.pkgAlias(pkg, name); a != nil {
				c.add(v, Item{Name: name, Origin: Static, Alias: a, Receiver: -1})
			}
			if typesOnly {
				continue
			}
			for _, f := range ext.pkgFuns(pkg, name) {
				if !f.Ext {
					c.add(v, Item{Name: name, Origin: Static, Fun: f, Type: f.Ret, Receiver: -1})
				}
			}
			if sym := ext.pkgVar(pkg, name); sym != nil {
				c.add(v, Item{Name: name, Origin: Static, Var: sym, Type: sym.Type, Receiver: -1})
			}
		}
	}
}

// builtinIDs returns the class ids of the built-in classes, kotlin/Int
// and kotlin/collections/List, which have no class files of their own,
// sorted.
func (t *classTable) builtinIDs() []string {
	if t.cp == nil || !t.kotlin {
		return nil
	}
	return slices.Sorted(maps.Keys(t.builtinClasses()))
}

// splitID splits a class id into its dotted package and its simple name;
// "" for a nested class's.
func splitID(id string) (pkg, name string) {
	i := strings.LastIndexByte(id, '/')
	if i < 0 || strings.Contains(id[i+1:], ".") {
		return "", ""
	}
	return strings.ReplaceAll(id[:i], "/", "."), id[i+1:]
}

// findQualified returns the class pkg.name, a built-in one too, or nil.
func (c *Checker) findQualified(pkg, name string) *types.ClassSym {
	if _, ok := kotlinTypeItem(name); ok && pkg == "kotlin" {
		return nil // a built-in type with no class: classifierItems has it
	}
	return c.u.ext.kclass(strings.ReplaceAll(pkg, ".", "/") + "/" + name)
}

// classNames returns the simple names of the classes of a package of the
// class path, by internal name, that code may name: not Kotlin's file
// facades nor its synthetic classes.
func (t *classTable) classNames(pkg string) []string {
	if t.cp == nil {
		return nil
	}
	all, _ := t.cp.Classes(pkg)
	facades := map[string]bool{}
	if t.kotlin {
		for _, part := range t.modules()[strings.ReplaceAll(pkg, "/", ".")] {
			facades[part.Class] = true
			facades[part.Facade] = true
		}
	}
	var out []string
	for _, name := range all {
		simple := name[strings.LastIndexByte(name, '/')+1:]
		if facades[name] || simple == "" || strings.ContainsAny(simple, "-") {
			continue
		}
		out = append(out, simple)
	}
	return out
}

// callSites adds to v the calls the probe, at p, is an argument of,
// innermost first.
func (c *Checker) callSites(v *ScopeView, p syntax.Pos) {
	for i := len(c.probeCalls) - 1; i >= 0; i-- {
		x := c.probeCalls[i]
		if isProbe(x) || x.Literal {
			continue
		}
		arg := -1
		for j, a := range x.Args {
			if a == nil {
				continue
			}
			if ap := a.Pos(); ap.Line < p.Line || ap.Line == p.Line && ap.Col <= p.Col {
				arg = j
			}
		}
		if arg < 0 {
			if len(x.Args) > 0 {
				continue // the probe is in the callee
			}
			arg = 0
		}
		site := CallSite{Arg: arg, Funs: c.callCandidates(x)}
		for j, n := range x.Names {
			if n == "" {
				continue
			}
			site.Names = append(site.Names, n)
			if j == arg {
				site.Name = n
			}
		}
		if len(site.Funs) > 0 {
			v.Calls = append(v.Calls, site)
		}
	}
}

// callCandidates returns the functions and constructors call x may call,
// by its name.
func (c *Checker) callCandidates(x *syntax.Call) []*types.FunSym {
	tmp := &ScopeView{}
	switch fn := x.Fun.(type) {
	case *syntax.Ident:
		c.scopeItems(tmp, fn.Name)
	case *syntax.Member:
		if cls := c.classRef(fn.X); cls != nil {
			c.staticItems(tmp, cls, fn.Name)
		} else if t := fn.X.Type(); t.K != types.Invalid {
			c.memberItems(tmp, t, -1, fn.Name)
			c.extensionItems(tmp, t.NonNull(), -1, c.visibleNames(), fn.Name)
		}
	default:
		return nil
	}
	var out []*types.FunSym
	for _, it := range tmp.Items {
		switch {
		case it.Fun != nil && !it.Prop:
			out = append(out, it.Fun)
		case it.Class != nil:
			out = append(out, constructors(it.Class)...)
		}
	}
	return out
}

// constructors returns the public constructors of cls.
func constructors(cls *types.ClassSym) []*types.FunSym {
	cls.Ensure()
	if cls.Abstract || cls.Object || cls.Interface {
		return nil
	}
	var out []*types.FunSym
	if cls.Ctor != nil {
		out = append(out, cls.Ctor)
	}
	for _, f := range cls.Ctors {
		if f != cls.Ctor && !f.Private {
			out = append(out, f)
		}
	}
	return out
}
