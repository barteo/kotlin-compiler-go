package check

import (
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// A Probe asks a check what the code could name where an identifier
// named Name is in File, for an editor's completion: the editor writes
// the identifier where the caret is. When the check reaches the
// identifier, as a name, a member a.Name or a call of either, it keeps
// what is in scope there; Items lists it once the check is done.
//
// A probe is set on Options.Probe for one check, and its Items is called
// after the check returns. Items looks things up as the check does, and
// may read more of the class path; it mustn't run at the same time as
// another check of the same Library's classes of the program, which a
// probe's check doesn't share.
type Probe struct {
	File *syntax.File
	Name string
	// Match reports whether a name is worth listing, as one that starts
	// as the identifier typed before the caret does; nil lists all.
	Match func(name string) bool
	// Importable also lists the class path's and the program's
	// top-level functions and properties that an import would bring
	// into scope, and the extensions of a receiver's type.
	Importable bool

	mu   sync.Mutex
	hit  *probeHit
	file *Checker // the file's checker, for a probe the check doesn't reach
}

// probeHit is what a check kept where it reached a probe's identifier.
type probeHit struct {
	c    *Checker
	st   state
	want types.Type
	// recv is the type of the receiver of a.Name, if hasRecv; static the
	// class whose static scope a qualifier names, C.Name; pkg the
	// program's package that names, p.Name.
	recv    types.Type
	hasRecv bool
	static  *types.ClassSym
	pkg     *Package
}

// ProbeKind is what a ProbeItem is.
type ProbeKind uint8

const (
	ProbeVar   ProbeKind = iota // a local, parameter or property
	ProbeFun                    // a function
	ProbeClass                  // a class, interface or object; a call of it constructs it
)

// ProbeItem is a declaration the code can name at a probe.
type ProbeItem struct {
	Kind  ProbeKind
	Name  string
	Var   *types.VarSym
	Fun   *types.FunSym
	Class *types.ClassSym
	// Type is a variable's type, smart casts applied, or a function's
	// result type, with the receiver's type arguments for a member's.
	Type types.Type
	// Params are a function's parameters' types, as Type.
	Params []types.Type
	// Level is how near it is: 0 for the locals, 1 for the members and
	// extensions of the receivers, explicit or implicit, 2 for the
	// top-level declarations in scope, 3 for those an import adds.
	Level int
	// Implicit is set on a member or extension of an implicit receiver,
	// Ext on an extension.
	Implicit, Ext bool
	// Import is the qualified name an import directive needs for the
	// item to be in scope; "" if it is.
	Import string
	// Ctors are a class's constructors the code can call.
	Ctors []*types.FunSym
}

// Found reports whether the check reached the probe's identifier.
func (p *Probe) Found() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hit != nil
}

// Receiver returns the type of the explicit receiver of a member at the
// probe, a.Name.
func (p *Probe) Receiver() (types.Type, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.hit == nil {
		return types.InvalidType, false
	}
	return p.hit.recv, p.hit.hasRecv
}

// Expected returns the type the code expects of the expression at the
// probe, or Invalid.
func (p *Probe) Expected() types.Type {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.hit == nil {
		return types.InvalidType
	}
	return p.hit.want
}

// probeAt keeps the state at x if x is the probe's identifier: a name,
// a member or a call of one.
func (c *Checker) probeAt(x syntax.Expr, want types.Type) {
	p := c.u.probe
	var qual syntax.Expr
	safe := false
	switch x := x.(type) {
	case *syntax.Ident:
		if x.Name != p.Name {
			return
		}
	case *syntax.Member:
		if x.Name != p.Name {
			return
		}
		qual, safe = x.X, x.Safe
	case *syntax.Call:
		switch f := x.Fun.(type) {
		case *syntax.Ident:
			if f.Name != p.Name {
				return
			}
		case *syntax.Member:
			if f.Name != p.Name {
				return
			}
			qual, safe = f.X, f.Safe
		default:
			return
		}
	default:
		return
	}
	h := &probeHit{c: c, st: c.state, want: want}
	// Later statements declare more in the scopes: keep them as they are.
	h.st.scope = copyScopes(c.scope)
	if qual != nil {
		c.probeQualifier(h, qual, safe)
	}
	p.mu.Lock()
	p.hit = h // the last check of it wins, as of a lambda checked again
	p.mu.Unlock()
}

// probeQualifier sets what the qualifier of a member at a probe is.
func (c *Checker) probeQualifier(h *probeHit, x syntax.Expr, safe bool) {
	if pkg := c.packageRef(x); pkg != nil {
		h.pkg = pkg
		return
	}
	if cls := c.classRef(x); cls != nil && !cls.Object {
		h.static = cls
		return
	}
	t := x.Type()
	if t.K == types.Invalid {
		t = c.expr(x, noWant)
	}
	if safe {
		t = t.NonNull()
	}
	h.recv, h.hasRecv = t, true
}

// copyScopes copies a chain of scopes as they are now: the declarations
// made later aren't in the copy.
func copyScopes(s *scope) *scope {
	if s == nil {
		return nil
	}
	cp := *s
	cp.vars = slices.Clip(s.vars)
	cp.funs = slices.Clip(s.funs)
	cp.classes = slices.Clip(s.classes)
	cp.narrows = slices.Clip(s.narrows)
	cp.parent = copyScopes(s.parent)
	return &cp
}

// Items returns what the code can name at the probe, nearest first. If
// the check didn't reach it, as in a type, they are the file's
// top-level declarations in scope.
func (p *Probe) Items() []ProbeItem {
	p.mu.Lock()
	h := p.hit
	p.mu.Unlock()
	if h == nil {
		if p.file == nil {
			return nil
		}
		h = &probeHit{c: p.file, st: state{}, want: types.InvalidType}
	}
	c := h.c
	saved := c.state
	c.state = h.st
	defer func() { c.state = saved }()
	pi := &probeItems{c: c, p: p, seen: map[string]bool{}}
	switch {
	case h.pkg != nil:
		pi.packageMembers(h.pkg)
	case h.static != nil:
		pi.statics(h.static)
	case h.hasRecv:
		pi.members(h.recv, false, 1)
		pi.extensions(h.recv, false, 1)
	default:
		pi.locals()
		pi.receivers()
		pi.topLevel()
	}
	if p.Importable {
		pi.importable(h)
	}
	return pi.out
}

type probeItems struct {
	c    *Checker
	p    *Probe
	out  []ProbeItem
	seen map[string]bool
	// names are the names of the top-level declarations in scope.
	names []string
}

func (pi *probeItems) match(name string) bool {
	if name == "" || name == pi.p.Name || strings.ContainsAny(name, "<$") {
		return false
	}
	return pi.p.Match == nil || pi.p.Match(name)
}

// add adds it unless an item before it has its key: a nearer one hides
// it.
func (pi *probeItems) add(it ProbeItem) {
	var key string
	switch it.Kind {
	case ProbeVar:
		key = "v " + it.Name
	case ProbeFun:
		key = "f " + it.Name + "(" + paramKey(it.Fun)
		if it.Fun.Ext {
			key += " ext " + it.Fun.Recv.String()
		}
	case ProbeClass:
		key = "c " + it.Name
	}
	if pi.seen[key] {
		return
	}
	pi.seen[key] = true
	pi.out = append(pi.out, it)
}

func (pi *probeItems) addFun(f *types.FunSym, sub types.Subst, level int, implicit bool, imp string) {
	if f.Hidden || f.Name == "<init>" || !pi.match(f.Name) {
		return
	}
	ensureFun(f)
	it := ProbeItem{Kind: ProbeFun, Name: f.Name, Fun: f, Type: sub.Apply(f.Ret), Level: level, Implicit: implicit, Ext: f.Ext, Import: imp}
	for _, prm := range f.Params {
		it.Params = append(it.Params, sub.ApplyIn(prm.Type))
	}
	pi.add(it)
}

func (pi *probeItems) addVar(v *types.VarSym, t types.Type, level int, implicit bool, imp string) {
	if !pi.match(v.Name) {
		return
	}
	ensureVar(v)
	if t.K == types.Invalid {
		t = v.Type
	}
	pi.add(ProbeItem{Kind: ProbeVar, Name: v.Name, Var: v, Type: t, Level: level, Implicit: implicit, Import: imp})
}

func (pi *probeItems) addClass(cls *types.ClassSym, name string, level int, imp string) {
	if cls == nil || cls.Missing || cls.Anonymous || !pi.match(name) {
		return
	}
	it := ProbeItem{Kind: ProbeClass, Name: name, Class: cls, Type: types.ClassType(cls), Level: level, Import: imp}
	if !cls.Object && !cls.Interface && cls.KotlinName != "kotlin/Nothing" && !pi.builtin(cls) && constructible(cls) {
		for _, f := range pi.c.visibleCtors(cls) {
			if !f.Private || !cls.External {
				ensureFun(f)
				it.Ctors = append(it.Ctors, f)
			}
		}
	}
	pi.add(it)
}

// builtin reports whether cls is one of builtinMembers' classes, which
// have no constructors.
func (pi *probeItems) builtin(cls *types.ClassSym) bool {
	return cls.KotlinName != "" && cls.Decl == nil && !cls.External && cls.Methods != nil && len(cls.Ctors) == 0 && cls.Ctor == nil
}

// locals adds the locals, parameters, local functions and classes in
// scope, innermost first.
func (pi *probeItems) locals() {
	c := pi.c
	for sc := c.scope; sc != nil; sc = sc.parent {
		for i := len(sc.vars) - 1; i >= 0; i-- {
			v := sc.vars[i]
			if v.Kind == types.BackingField && !pi.match("field") {
				continue
			}
			pi.addVar(v, c.scope.narrowed(v), 0, false, "")
		}
		for _, f := range sc.funs {
			pi.addFun(f, nil, 0, false, "")
		}
		for _, k := range sc.classes {
			if d, ok := k.Decl.(*syntax.ClassDecl); ok {
				pi.addClass(k, d.Name, 0, "")
			}
		}
	}
}

// receivers adds the members and extensions of the implicit receivers:
// lambdas' receivers, innermost first, this, the outer classes' this of
// an inner class, and the companion objects of the classes around.
func (pi *probeItems) receivers() {
	c := pi.c
	for i := len(c.recvs) - 1; i >= 0; i-- {
		t := c.recvs[i].sym.Type
		pi.members(t, true, 1)
		pi.extensions(t, true, 1)
	}
	for k := c.cls; k != nil; k = k.Outer {
		if k == c.cls || k.Object {
			pi.members(k.This(), true, 1)
			pi.extensions(k.This(), true, 1)
		}
		if k.Companion != nil {
			pi.members(types.ClassType(k.Companion), true, 1)
		}
		if !k.Inner && !k.Anonymous {
			// A nested class doesn't see its outer class's members, but
			// for its companion's and those of the objects around.
			for o := k.Outer; o != nil; o = o.Outer {
				if o.Companion != nil {
					pi.members(types.ClassType(o.Companion), true, 1)
				}
				if o.Object {
					pi.members(o.This(), true, 1)
				}
			}
			break
		}
	}
}

// members adds the members of a value of type t that the code can see.
func (pi *probeItems) members(t types.Type, implicit bool, level int) {
	c := pi.c
	cls := c.probeClass(t)
	if cls == nil {
		return
	}
	object := false
	walk := func(s *types.ClassSym) bool {
		if s.External {
			s.Ensure()
		}
		if s.Name == "java/lang/Object" {
			return true // Kotlin sees it as kotlin.Any, without getClass or wait
		}
		object = object || s.KotlinName == "kotlin/Any"
		sub := c.memberSubst(t, s)
		for _, name := range slices.Sorted(maps.Keys(s.Methods)) {
			for _, f := range s.Methods[name] {
				if f.Static || !c.funVisible(f) || notInherited(f, cls) {
					continue
				}
				pi.addFun(f, sub, level, implicit, "")
			}
		}
		for _, v := range s.Props {
			if v.Kind == types.Static || !c.propVisible(v) || propNotInherited(v, cls) {
				continue
			}
			pi.addVar(v, sub.Apply(v.Type), level, implicit, "")
		}
		for _, name := range slices.Sorted(maps.Keys(s.KProps)) {
			kp := s.KProps[name]
			if kp.Getter == nil || !c.funVisible(kp.Getter) {
				continue
			}
			v := &types.VarSym{Name: name, Kind: types.Prop, Owner: s, Type: kp.Getter.Ret, Mutable: kp.Setter != nil, Decl: kp.Getter.Decl}
			pi.addVar(v, sub.Apply(kp.Getter.Ret), level, implicit, "")
		}
		for _, name := range slices.Sorted(maps.Keys(s.JavaFields)) {
			if f := s.JavaFields[name]; c.propVisible(f) {
				pi.addVar(f, sub.Apply(f.Type), level, implicit, "")
			}
		}
		return true
	}
	cls.Supertypes(walk)
	if !object {
		// Every class's supertype kotlin.Any, which a source class's
		// supertypes leave out, and which stands for a Java class's
		// java.lang.Object.
		if any := c.u.ext.builtinMembers("kotlin/Any"); any != nil {
			walk(any)
		}
	}
}

// probeClass returns the class whose members a value of type t has: the
// built-in types' from their metadata.
func (c *Checker) probeClass(t types.Type) *types.ClassSym {
	t = t.NonNull()
	switch {
	case t.K == types.String:
		return c.u.ext.builtinMembers("kotlin/String")
	case types.IsPrimitiveKind(t.K):
		return c.u.ext.builtinMembers("kotlin/" + t.K.Name())
	case t.K == types.Any:
		return c.u.ext.builtinMembers("kotlin/Any")
	case t.K == types.Class && t.C.Array && t.C.KotlinName != "":
		return c.u.ext.builtinMembers(t.C.KotlinName)
	}
	return c.classOf(t)
}

// builtinMembers returns a class with the members that the built-in
// class id declares, as kotlin/String, read from the built-ins'
// metadata, for completion; nil if there is none.
func (t *classTable) builtinMembers(id string) *types.ClassSym {
	t = t.root()
	t.mu.Lock()
	if cls, ok := t.probeBuiltins[id]; ok {
		t.mu.Unlock()
		return cls
	}
	t.mu.Unlock()
	kc := t.builtinClasses()[id]
	if kc == nil {
		return nil
	}
	cls := &types.ClassSym{Name: id, KotlinName: id, Methods: map[string][]*types.FunSym{}}
	scope := tscope{}
	cls.TypeParams, _ = t.ktypeParams(kc.TypeParams, cls, scope)
	for _, f := range kc.Funcs {
		g := *f
		g.JVM = builtinSig(f.Name, f.Params, f.Ret)
		if sym := t.kfun(&g, cls, scope, nil); sym != nil {
			cls.Methods[f.Name] = append(cls.Methods[f.Name], sym)
		}
	}
	for _, p := range kc.Props {
		if pt, ok := t.ktype(p.Type, scope); ok {
			cls.Props = append(cls.Props, &types.VarSym{Name: p.Name, Kind: types.Prop, Owner: cls, Type: pt, Mutable: p.IsVar(), Resolved: true})
		}
	}
	if id != "kotlin/Any" {
		if any := t.builtinMembers("kotlin/Any"); any != nil {
			cls.Supers = []types.Type{types.ClassType(any)}
			cls.Interfaces = []*types.ClassSym{any}
		}
	}
	t.mu.Lock()
	if t.probeBuiltins == nil {
		t.probeBuiltins = map[string]*types.ClassSym{}
	}
	t.probeBuiltins[id] = cls
	t.mu.Unlock()
	return cls
}

// extensions adds the extension functions and properties in scope that
// take a receiver of type t.
func (pi *probeItems) extensions(t types.Type, implicit bool, level int) {
	c := pi.c
	for _, name := range pi.scopeNames() {
		if !pi.match(name) {
			continue
		}
		for _, cl := range c.extCallees(name, t) {
			if !cl.f.Ext {
				continue
			}
			ensureFun(cl.f)
			if c.recvBounded(t, cl.f.Recv) {
				pi.addFun(cl.f, cl.recv, level, implicit, "")
			}
		}
		for _, g := range pi.extProps(name) {
			if !g.Hidden && c.recvFits(t, g.Recv) && c.recvBounded(t, g.Recv) {
				ensureFun(g)
				n := len(pi.out)
				pi.addVar(&types.VarSym{Name: name, Kind: types.Prop, Type: g.Ret, Decl: g.Decl, Facade: g.Facade}, g.Ret, level, implicit, "")
				if len(pi.out) > n {
					pi.out[n].Ext = true
				}
			}
		}
	}
}

// recvBounded reports whether a receiver of type rt is within the bounds
// of an extension's receiver type r if that is a type parameter, which
// recvFits leaves to a call's inference: a Box isn't an Appendable, the
// bound of T.append's T.
func (c *Checker) recvBounded(rt, r types.Type) bool {
	rn := rt.NonNull()
	if !r.IsParam() || rn.IsParam() {
		return true
	}
	for _, b := range r.Param().Bounds {
		if b = b.NonNull(); b.K == types.Class && !c.isSubclass(rn, b.C) {
			return false
		}
	}
	return true
}

// extProps returns the getters of the extension properties in scope
// named name.
func (pi *probeItems) extProps(name string) []*types.FunSym {
	c := pi.c
	gs := slices.Concat(c.userExtProps[name], c.pkg.ExtProps[name])
	for _, p := range c.userStars {
		gs = append(gs, p.ExtProps[name]...)
	}
	for _, r := range pi.libraryRefs() {
		gs = append(gs, c.u.ext.pkgExtPropsIn(r, name)...)
	}
	return gs
}

// libraryRefs returns the class path's packages in scope: the file's,
// the star-imported and the default ones.
func (pi *probeItems) libraryRefs() []*pkgRef {
	c := pi.c
	refs := []*pkgRef{c.u.ext.pkgRef(c.pkg.Name)}
	refs = append(refs, c.starPkgs()...)
	return append(refs, c.u.ext.defaultRefs()...)
}

// scopeNames returns the names of the top-level functions and
// properties in scope, and of the classes of the file's package and the
// imported ones, sorted.
func (pi *probeItems) scopeNames() []string {
	if pi.names != nil {
		return pi.names
	}
	c := pi.c
	set := map[string]bool{}
	addKeys := func(m map[string]bool) {
		for k := range m {
			set[k] = true
		}
	}
	for k := range c.pkg.Funs {
		set[k] = true
	}
	for k := range c.pkg.Vars {
		set[k] = true
	}
	for k := range c.pkg.ExtProps {
		set[k] = true
	}
	for k := range c.pkg.Classes {
		set[k] = true
	}
	for k := range c.fileVars {
		set[k] = true
	}
	for k := range c.userFuns {
		set[k] = true
	}
	for k := range c.userVars {
		set[k] = true
	}
	for k := range c.userExtProps {
		set[k] = true
	}
	for k := range c.funImports {
		set[k] = true
	}
	for k := range c.memberImports {
		set[k] = true
	}
	for k := range c.imports {
		set[k] = true
	}
	for k := range c.aliasImports {
		set[k] = true
	}
	for _, p := range c.userStars {
		for k := range p.Funs {
			set[k] = true
		}
		for k := range p.Vars {
			set[k] = true
		}
		for k := range p.Classes {
			set[k] = true
		}
	}
	for _, r := range pi.libraryRefs() {
		if k := c.u.ext.read(r.k); k != nil {
			addKeys(k.names)
		}
	}
	pi.names = slices.Sorted(maps.Keys(set))
	if pi.names == nil {
		pi.names = []string{}
	}
	return pi.names
}

// topLevel adds the top-level functions, properties and classes in
// scope, by the names in scope and the classes of the packages in scope.
func (pi *probeItems) topLevel() {
	c := pi.c
	names := slices.Clone(pi.scopeNames())
	for _, r := range pi.libraryRefs() {
		names = append(names, pi.classNames(r.path)...)
	}
	names = append(names, pi.classNames("java/lang")...)
	for id := range c.u.ext.builtinClasses() {
		if i := strings.LastIndexByte(id, '/'); i >= 0 && slices.Contains(defaultPackages, strings.ReplaceAll(id[:i], "/", ".")) {
			names = append(names, id[i+1:])
		}
	}
	slices.Sort(names)
	names = slices.Compact(names)
	for _, name := range names {
		if !pi.match(name) {
			continue
		}
		if v := c.topVar(name); v != nil {
			pi.addVar(v, c.scope.narrowed(v), 2, false, "")
		}
		fs := c.topFuns(name)
		if len(fs) == 0 {
			// The class path's, imported, of the file's package, star
			// imported or imported by default: the first level that has
			// one.
			for _, level := range c.funLevels(name, false, nil) {
				if len(level) > 0 {
					fs = level
					break
				}
			}
		}
		for _, f := range fs {
			if !f.Ext && c.funVisible(f) {
				pi.addFun(f, nil, 2, false, "")
			}
		}
		if cls := c.findClass(name); cls != nil && c.classVisible(cls) {
			pi.addClass(cls, name, 2, "")
		} else if k := types.IDKind("kotlin/" + name); k != types.Invalid || name == "String" || name == "Any" {
			// A built-in type, which has no class: Int, String.
			pi.addClass(c.u.ext.builtinMembers("kotlin/"+name), name, 2, "")
		}
	}
}

// classNames returns the simple names of the top-level classes of the
// class path's package path, but for Kotlin's file facades.
func (pi *probeItems) classNames(path string) []string {
	cp := pi.c.u.ext.cp
	if cp == nil {
		return nil
	}
	ids, _ := cp.Classes(path)
	var out []string
	for _, id := range ids {
		name := id[strings.LastIndexByte(id, '/')+1:]
		if !strings.HasSuffix(name, "Kt") && name != "package-info" && name != "module-info" {
			out = append(out, name)
		}
	}
	return out
}

// statics adds what a class's name qualifies: its nested classes, its
// companion's members, its enum entries and a Java class's statics.
func (pi *probeItems) statics(cls *types.ClassSym) {
	c := pi.c
	if cls.External {
		cls.Ensure()
	}
	for _, name := range slices.Sorted(maps.Keys(cls.Nested)) {
		pi.addClass(cls.Nested[name], name, 1, "")
	}
	if cls.Companion != nil {
		pi.members(types.ClassType(cls.Companion), false, 1)
	}
	for _, name := range slices.Sorted(maps.Keys(cls.Statics)) {
		for _, f := range cls.Statics[name] {
			if c.funVisible(f) {
				pi.addFun(f, nil, 1, false, "")
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cls.Fields)) {
		if f := cls.Fields[name]; c.propVisible(f) {
			pi.addVar(f, f.Type, 1, false, "")
		}
	}
	for _, e := range cls.Entries {
		if pi.match(e) {
			pi.add(ProbeItem{Kind: ProbeVar, Name: e, Var: &types.VarSym{Name: e, Kind: types.Static, Owner: cls, Type: types.ClassType(cls)}, Type: types.ClassType(cls), Level: 1})
		}
	}
}

// packageMembers adds the top-level declarations of a package of the
// program, p.Name.
func (pi *probeItems) packageMembers(p *Package) {
	for _, name := range slices.Sorted(maps.Keys(p.Classes)) {
		pi.addClass(p.Classes[name], name, 1, "")
	}
	for _, name := range slices.Sorted(maps.Keys(p.Funs)) {
		for _, f := range p.Funs[name] {
			if !f.Ext && pi.c.funVisible(f) {
				pi.addFun(f, nil, 1, false, "")
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(p.Vars)) {
		pi.addVar(p.Vars[name], types.InvalidType, 1, false, "")
	}
}

// importable adds the top-level functions and properties of the
// packages not in scope, and the extensions there of the receiver's
// type, with the imports they need.
func (pi *probeItems) importable(h *probeHit) {
	if h.pkg != nil || h.static != nil {
		return
	}
	c := pi.c
	inScope := map[string]bool{c.pkg.Name: true}
	for _, r := range pi.libraryRefs() {
		inScope[r.name] = true
	}
	for _, p := range c.userStars {
		inScope[p.Name] = true
	}
	// The program's packages.
	for _, pname := range slices.Sorted(maps.Keys(c.u.Pkgs)) {
		if inScope[pname] {
			continue
		}
		p := c.u.Pkgs[pname]
		for _, name := range slices.Sorted(maps.Keys(p.Funs)) {
			if !pi.match(name) {
				continue
			}
			for _, f := range p.Funs[name] {
				if f.Private || !c.funVisible(f) {
					continue
				}
				if f.Ext && !(h.hasRecv && c.recvFits(h.recv, f.Recv) && c.recvBounded(h.recv, f.Recv)) {
					continue
				}
				if !f.Ext && h.hasRecv {
					continue
				}
				pi.addFun(f, nil, 3, false, qualify(pname, name))
			}
		}
		if !h.hasRecv {
			for _, name := range slices.Sorted(maps.Keys(p.Vars)) {
				if v := p.Vars[name]; pi.match(name) && c.propVisible(v) {
					pi.addVar(v, types.InvalidType, 3, false, qualify(pname, name))
				}
			}
		}
	}
	// The class path's Kotlin packages.
	t := c.u.ext.root()
	if !t.kotlin {
		return
	}
	t.modules()
	for _, pname := range slices.Sorted(maps.Keys(t.pkgs)) {
		if inScope[pname] {
			continue
		}
		k := t.read(t.pkgs[pname])
		if k == nil {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(k.names)) {
			if !pi.match(name) {
				continue
			}
			var fits func(types.Type) bool
			kind := nonExtFuns
			if h.hasRecv {
				kind, fits = extFuns, func(r types.Type) bool { return c.recvFits(h.recv, r) && c.recvBounded(h.recv, r) }
			}
			for _, f := range t.pkgFunsWhere(pname, name, false, kind, fits) {
				if c.funVisible(f) {
					pi.addFun(f, nil, 3, false, qualify(pname, name))
				}
			}
			if !h.hasRecv {
				if v := t.pkgVar(pname, name); v != nil {
					pi.addVar(v, types.InvalidType, 3, false, qualify(pname, name))
				}
			}
		}
	}
}

func qualify(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}
