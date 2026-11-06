package check

import (
	"fmt"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// A graph's bindings, and the Impl class kc writes for it.
//
// Each binding the graph's members need, directly or through other
// bindings, becomes a private function of Impl that returns its value;
// a scoped binding's holds it in a kotlin.Lazy. A request for a binding
// as () -> T, Provider<T> or Lazy<T> gets a function that wraps it, and a
// set or map multibinding one that collects its elements.

// mGraph is a graph being resolved.
type mGraph struct {
	m      *metroAnalysis
	c      *Checker
	g      *types.ClassSym
	pos    syntax.Pos
	scopes map[string]bool // the keys of the scope annotations the graph has

	bindings map[string][]*mBinding
	multis   map[string]*mMulti
	// implicit holds the bindings of injected classes, made when first
	// requested, by key.
	implicit map[string]*mBinding

	ctorParams []string // Impl's constructor's parameters
	fields     []string // Impl's properties and functions
	supers     []string // the interfaces Impl implements besides the graph
	overrides  []string
	nfun       int
	wrappers   map[string]string // request key -> the function that returns it
	stack      []*mBinding
	containers map[*types.ClassSym]string // binding containers' instances
	failed     bool
	excluded   map[string]bool
}

// mBinding is how a graph gets the value of a key.
type mBinding struct {
	key    string
	t      types.Type
	what   string // for messages
	scoped bool
	scope  string
	deps   []*mReq
	// expr writes the value's Kotlin from its dependencies'.
	expr     func(args []string) string
	fn       string // the function of Impl that returns it, once written
	priority int64
	multi    *mMulti
	// from is the class that contributes the binding, if any.
	from string
}

// mMulti is a set or map multibinding.
type mMulti struct {
	key      string
	t        types.Type // Set<T> or Map<K, V>
	isMap    bool
	declared bool
	elems    []*mElem
}

// mElem is an element of a multibinding: a binding, the elements of a
// collection binding into a set, or a map entry with its key's Kotlin.
type mElem struct {
	b        *mBinding
	elements bool
	mapKey   string
	mapKeyID string
}

// mReq is a request for a key's value: as is, or wrapped, outermost first,
// in () -> T (f), Provider<T> (p) or Lazy<T> (l). A map's values may be
// wrapped too: vwrap.
type mReq struct {
	key      string
	t        types.Type // the key's type
	q        string
	wrap     []byte
	vwrap    []byte
	full     types.Type // the type requested
	optional bool       // a parameter with a default value
	name     string
}

func (m *metroAnalysis) newGraph(c *Checker, cls *types.ClassSym, pos syntax.Pos) *mGraph {
	return &mGraph{m: m, c: c, g: cls, pos: pos, scopes: map[string]bool{}, bindings: map[string][]*mBinding{},
		multis: map[string]*mMulti{}, implicit: map[string]*mBinding{}, wrappers: map[string]string{},
		containers: map[*types.ClassSym]string{}, excluded: map[string]bool{}}
}

func (gr *mGraph) errorf(format string, args ...any) {
	gr.failed = true
	gr.m.errorf(gr.c, gr.pos, format, args...)
}

func (gr *mGraph) unsupported(format string, args ...any) {
	gr.failed = true
	gr.m.unsupported(gr.c, gr.pos, format, args...)
}

// keyOf returns the key of a type with a qualifier.
func keyOf(t types.Type, q string) (string, bool) {
	s, ok := kotlinTypeSrc(t)
	if !ok {
		return "", false
	}
	if q != "" {
		return s + " @" + q, true
	}
	return s, true
}

// request returns a request for type t with qualifier q.
func (gr *mGraph) request(t types.Type, q string) (*mReq, bool) {
	r := &mReq{full: t, q: q}
	for {
		w := wrapperOf(t)
		if w == 0 || w == 'f' && !gr.m.functionProviders {
			break
		}
		r.wrap = append(r.wrap, w)
		t = t.A.List[len(t.A.List)-1].T
	}
	if isMapType(t) {
		v := t.A.List[1].T
		for {
			w := wrapperOf(v)
			if w == 0 || w == 'f' && !gr.m.functionProviders {
				break
			}
			r.vwrap = append(r.vwrap, w)
			v = v.A.List[len(v.A.List)-1].T
		}
		if len(r.vwrap) > 0 {
			args := slices.Clone(t.A.List)
			args[1] = types.Arg{V: args[1].V, T: v}
			t = types.GenericType(t.C, args)
		}
	}
	r.t = t
	k, ok := keyOf(t, q)
	r.key = k
	return r, ok
}

// wrapperOf returns how type t wraps a value: f for () -> T, p for
// Metro's Provider<T>, l for Lazy<T>, or 0.
func wrapperOf(t types.Type) byte {
	if t.K != types.Class || t.N || t.A == nil {
		return 0
	}
	switch {
	case t.IsFunc() && len(t.A.List) == 1 && !t.C.Suspend && !t.E:
		return 'f'
	case t.C.Name == mProvider:
		return 'p'
	case t.C.KotlinName == "kotlin/Lazy":
		return 'l'
	}
	return 0
}

func isMapType(t types.Type) bool {
	return t.K == types.Class && !t.N && t.A != nil && len(t.A.List) == 2 && t.C.KotlinName == "kotlin/collections/Map"
}

func isSetType(t types.Type) bool {
	return t.K == types.Class && !t.N && t.A != nil && len(t.A.List) == 1 && t.C.KotlinName == "kotlin/collections/Set"
}

// add adds a binding.
func (gr *mGraph) add(b *mBinding) {
	gr.bindings[b.key] = append(gr.bindings[b.key], b)
}

// multi returns the multibinding of a key, which it creates.
func (gr *mGraph) multi(t types.Type, q string) *mMulti {
	key := gr.multiKey(t, q)
	mu := gr.multis[key]
	if mu == nil {
		mu = &mMulti{key: key, t: t, isMap: isMapType(t)}
		gr.multis[key] = mu
	}
	return mu
}

// graph resolves the bindings of graph class d, annotated a, and writes
// its Impl class and companion.
func (m *metroAnalysis) graph(c *Checker, d *syntax.ClassDecl, a *mAnn) {
	cls := d.Sym
	gr := m.newGraph(c, cls, d.P)
	if len(cls.TypeParams) > 0 {
		gr.unsupported("Metro graphs with type parameters are not supported")
		return
	}
	if !d.Interface && !d.Abstract {
		gr.errorf("@DependencyGraph classes must be interfaces or abstract classes")
		return
	}
	var scopeClasses []string
	if s := a.args["scope"]; s.tag == 'c' && s.class != mNothing {
		scopeClasses = append(scopeClasses, s.class)
	}
	scopeClasses = append(scopeClasses, a.args["additionalScopes"].classes()...)
	for _, s := range scopeClasses {
		gr.scopes[singleIn(s)] = true
	}
	for _, s := range m.scopesOf(m.annsOf(cls)) {
		gr.scopes[s] = true
	}
	for _, e := range a.args["excludes"].classes() {
		gr.excluded[e] = true
	}

	// The graph itself.
	if k, ok := keyOf(cls.This(), ""); ok {
		gr.add(&mBinding{key: k, t: cls.This(), what: cls.DisplayName(), expr: func([]string) string { return "this" }})
	}

	// The factory and its parameters.
	var factory *types.ClassSym
	var create *mMember
	for _, n := range cls.Nested {
		if find(m.annsOf(n), mGraphFactory) != nil {
			factory = n
		}
	}
	if factory != nil {
		for _, mm := range m.inherited(factory) {
			if mm.abstract {
				if create != nil {
					gr.errorf("@DependencyGraph.Factory %s must have a single abstract function", factory.DisplayName())
					return
				}
				create = mm
			}
		}
		if create == nil || create.prop {
			gr.errorf("@DependencyGraph.Factory %s must have a single abstract function", factory.DisplayName())
			return
		}
		if !factory.Interface {
			gr.unsupported("Metro graph factories that are abstract classes are not supported")
			return
		}
	}
	var containers []*types.ClassSym
	containerSeen := map[*types.ClassSym]bool{}
	addContainer := func(k *types.ClassSym, instance string) {
		if k == nil || containerSeen[k] || gr.excluded[k.Name] {
			return
		}
		containerSeen[k] = true
		if instance != "" {
			gr.containers[k] = instance
		}
		containers = append(containers, k)
	}
	if create != nil {
		for i, p := range create.params {
			pa := create.paramAnns[i]
			field := fmt.Sprintf("`$p%d`", i)
			ts, ok := kotlinTypeSrc(p.Type)
			if !ok {
				gr.unsupported("kc can't name the type %s of a graph factory parameter", p.Type)
				return
			}
			gr.ctorParams = append(gr.ctorParams, fmt.Sprintf("private val %s: %s", field, ts))
			switch {
			case find(pa, mProvides) != nil:
				k, _ := keyOf(p.Type, m.qualifier(pa))
				gr.add(&mBinding{key: k, t: p.Type, what: "@Provides parameter " + p.Name, expr: func([]string) string { return field }})
			case find(pa, mIncludes) != nil:
				if p.Type.K != types.Class {
					gr.unsupported("kc can't include %s in a graph", p.Type)
					return
				}
				inc := p.Type.C
				k, _ := keyOf(p.Type, "")
				gr.add(&mBinding{key: k, t: p.Type, what: "@Includes parameter " + p.Name, expr: func([]string) string { return field }})
				if find(m.annsOf(inc), mBindingContainer) != nil {
					addContainer(inc, field)
					continue
				}
				gr.includeAccessors(inc, field)
			default:
				gr.errorf("graph factory parameters must be annotated @Provides or @Includes: %s", p.Name)
				return
			}
		}
	}

	for _, k := range a.args["bindingContainers"].classes() {
		addContainer(m.class(k), "")
	}

	// Contributions to the graph's scopes.
	type contrib struct {
		cls *types.ClassSym
		ann *mAnn
	}
	var contribs []contrib
	replaced := map[string]bool{}
	for _, scope := range scopeClasses {
		for _, cc := range m.contributorsOf(scope) {
			for _, ca := range m.annsOf(cc) {
				switch ca.cls {
				case mContributesTo, mContributesBinding, mContributesIntoSet, mContributesIntoMap:
					if ca.args["scope"].class != scope {
						continue
					}
					for _, r := range ca.args["replaces"].classes() {
						replaced[r] = true
					}
					contribs = append(contribs, contrib{cc, ca})
				}
			}
		}
	}
	var supers []*types.ClassSym
	for _, ct := range contribs {
		if replaced[ct.cls.Name] || gr.excluded[ct.cls.Name] {
			continue
		}
		switch ct.ann.cls {
		case mContributesTo:
			if find(m.annsOf(ct.cls), mBindingContainer) != nil {
				addContainer(ct.cls, "")
				continue
			}
			if !slices.Contains(supers, ct.cls) && !cls.IsSubclassOf(ct.cls) {
				supers = append(supers, ct.cls)
			}
		default:
			if err := gr.contributedBinding(ct.cls, ct.ann); err != nil {
				gr.unsupported("%v", err)
				return
			}
		}
	}
	for _, s := range supers {
		ts, ok := kotlinTypeSrc(s.This())
		if !ok || len(s.TypeParams) > 0 {
			gr.unsupported("kc can't implement contributed interface %s", s.DisplayName())
			return
		}
		gr.supers = append(gr.supers, ts)
	}

	// The members of the graph and of the interfaces it implements.
	var accessors []*mMember
	treeSeen := map[string]bool{}
	tree := append([]*types.ClassSym{cls}, supers...)
	for _, t := range tree {
		for _, mm := range m.inherited(t) {
			k := fmt.Sprintf("%s/%v/%d/%v", mm.name, mm.prop, len(mm.params), mm.recv != nil)
			if treeSeen[k] {
				continue
			}
			treeSeen[k] = true
			switch {
			case find(mm.anns, mProvides) != nil:
				gr.provides(mm, "this")
			case find(mm.anns, mBinds) != nil:
				gr.binds(mm, true)
			case find(mm.anns, mMultibinds) != nil:
				gr.multibinds(mm)
				if mm.abstract {
					accessors = append(accessors, mm)
				}
			case mm.abstract:
				accessors = append(accessors, mm)
			}
		}
	}
	// Binding containers, and those they include.
	for i := 0; i < len(containers); i++ {
		k := containers[i]
		for _, inc := range find(m.annsOf(k), mBindingContainer).argClasses("includes") {
			addContainer(m.class(inc), "")
		}
		gr.container(k)
	}
	if gr.failed {
		return
	}
	for _, mm := range accessors {
		gr.accessor(mm)
	}
	if gr.failed {
		return
	}
	gr.write(d, factory, create)
}

func (a *mAnn) argClasses(name string) []string {
	if a == nil {
		return nil
	}
	return a.args[name].classes()
}

// includeAccessors binds what the members of an included graph or
// interface return, from the instance in field.
func (gr *mGraph) includeAccessors(inc *types.ClassSym, field string) {
	for _, mm := range gr.m.inherited(inc) {
		if len(mm.params) > 0 || mm.recv != nil || mm.ret.K == types.Unit || !mm.abstract {
			continue
		}
		k, ok := keyOf(mm.ret, gr.m.qualifier(mm.anns))
		if !ok {
			continue
		}
		get := field + "." + backtick(mm.name)
		if !mm.prop {
			get += "()"
		}
		gr.add(&mBinding{key: k, t: mm.ret, what: inc.DisplayName() + "." + mm.name, expr: func([]string) string { return get }})
	}
}

func backtick(name string) string { return "`" + name + "`" }

// contributorsOf returns the classes that contribute to a scope: the
// program's, and those that the class path's hints name.
func (m *metroAnalysis) contributorsOf(scope string) []*types.ClassSym {
	var out []*types.ClassSym
	for _, c := range m.checkers {
		for _, d := range c.classes {
			if d.Sym == nil || d.Sym.Local || d.Sym.Anonymous {
				continue
			}
			for _, a := range m.annsOf(d.Sym) {
				switch a.cls {
				case mContributesTo, mContributesBinding, mContributesIntoSet, mContributesIntoMap:
					if a.args["scope"].class == scope && !slices.Contains(out, d.Sym) {
						out = append(out, d.Sym)
					}
				}
			}
		}
	}
	sc := m.class(scope)
	cp := m.u.ext.cp
	if sc == nil || cp == nil {
		return out
	}
	hint := strings.ReplaceAll(fqName(sc), ".", "_")
	for _, name := range cp.Package("metro/hints") {
		info, err := cp.Load(name)
		if err != nil {
			continue
		}
		for _, mt := range info.Methods {
			if mt.Name != hint || !strings.HasPrefix(mt.Desc, "(L") || !strings.HasSuffix(mt.Desc, ";)V") {
				continue
			}
			cn := mt.Desc[2 : len(mt.Desc)-3]
			if m.u.sourceClass(cn) != nil {
				continue // the program's, from an earlier compilation
			}
			if cls := m.u.ext.class(cn); cls != nil && !slices.Contains(out, cls) {
				out = append(out, cls)
			}
		}
	}
	return out
}

// contributedBinding adds the bindings of a contributing class's
// @ContributesBinding, @ContributesIntoSet or @ContributesIntoMap a.
func (gr *mGraph) contributedBinding(cls *types.ClassSym, a *mAnn) error {
	m := gr.m
	scope := a.args["scope"].class
	if !isSource(cls) {
		// Its nested contribution interfaces' @Binds functions.
		cf := m.classfile(cls.Name)
		if cf == nil {
			return fmt.Errorf("kc can't read contributing class %s", cls.DisplayName())
		}
		found := false
		for _, n := range cf.Nested {
			nc := m.class(n)
			if nc == nil {
				continue
			}
			ca := find(m.annsOf(nc), mContribution)
			if ca == nil || ca.args["scope"].class != scope {
				continue
			}
			if _, done := gr.containers[nc]; done {
				continue
			}
			found = true
			gr.containers[nc] = ""
			gr.container(nc)
		}
		if !found && find(m.annsOf(cls), mContributesBinding) == a {
			return fmt.Errorf("kc can't find the contribution of %s to its scope: its class files have no MetroContribution interface", cls.DisplayName())
		}
		return nil
	}
	anns := m.annsOf(cls)
	bound, err := m.boundType(cls, a)
	if err != nil {
		return err
	}
	q := m.qualifier(anns)
	if cls.Object {
		return fmt.Errorf("kc doesn't support contributing objects (%s)", cls.DisplayName())
	}
	dep, ok := gr.request(cls.This(), "")
	if !ok {
		return fmt.Errorf("kc can't name %s", cls.DisplayName())
	}
	prio := int64(-1 << 31)
	if p, ok := a.args["priority"]; ok {
		prio = p.num
	}
	alias := func(key string) *mBinding {
		return &mBinding{key: key, t: bound, what: cls.DisplayName() + " as " + bound.String(), deps: []*mReq{dep},
			expr: func(args []string) string { return args[0] }, priority: prio, from: cls.Name}
	}
	switch a.cls {
	case mContributesBinding:
		k, ok := keyOf(bound, q)
		if !ok {
			return fmt.Errorf("kc can't name %s", bound)
		}
		gr.add(alias(k))
	case mContributesIntoSet:
		st := gr.setOf(bound)
		mu := gr.multi(st, q)
		mu.elems = append(mu.elems, &mElem{b: alias("")})
	case mContributesIntoMap:
		key := m.mapKeyAnn(anns)
		if key == nil {
			return fmt.Errorf("@ContributesIntoMap needs a map key annotation on %s", cls.DisplayName())
		}
		return gr.mapElem(m.implicitKey(key, cls), bound, q, alias(""))
	}
	return nil
}

func (gr *mGraph) setOf(t types.Type) types.Type {
	set := gr.c.u.ext.kclass("kotlin/collections/Set")
	return types.GenericType(set, []types.Arg{{V: types.Inv, T: t}})
}

// mapElem adds an entry to the map multibinding of values of type v,
// with qualifier q, under the key of map key annotation key.
func (gr *mGraph) mapElem(key *mAnn, v types.Type, q string, b *mBinding) error {
	m := gr.m
	val, ok := key.args["value"]
	if !ok {
		for _, x := range key.args {
			val = x
		}
	}
	keyCls := m.class(key.cls)
	if keyCls == nil {
		return fmt.Errorf("unknown map key %s", key.cls)
	}
	keyCls.Ensure()
	var kt types.Type
	if ps := keyCls.AnnotationParams; len(ps) == 1 {
		kt = ps[0].Type
	} else if kp := keyCls.KProps; len(kp) == 1 {
		for _, p := range kp {
			kt = p.Getter.Ret
		}
	} else if ms := keyCls.Methods; len(ms) >= 1 {
		for _, fs := range ms {
			for _, f := range fs {
				if f.Abstract && len(f.Params) == 0 {
					kt = f.Ret
				}
			}
		}
	}
	if kt.K == types.Invalid {
		return fmt.Errorf("kc can't read the key type of map key %s", keyCls.DisplayName())
	}
	src, err := m.kotlinValSrc(val)
	if err != nil {
		return err
	}
	mapCls := gr.c.u.ext.kclass("kotlin/collections/Map")
	mt := types.GenericType(mapCls, []types.Arg{{V: types.Inv, T: kt}, {V: types.Inv, T: v}})
	if _, ok := keyOf(mt, q); !ok {
		return fmt.Errorf("kc can't name %s", mt)
	}
	mu := gr.multi(mt, q)
	mu.elems = append(mu.elems, &mElem{b: b, mapKey: src, mapKeyID: val.key()})
	return nil
}

// multiKey returns the key a multibinding of type t is found by: maps
// by key class and value type, sets by element type.
func (gr *mGraph) multiKey(t types.Type, q string) string {
	s, _ := kotlinTypeSrc(t)
	if isMapType(t) {
		kt := t.A.List[0].T
		vt, _ := kotlinTypeSrc(t.A.List[1].T)
		kc := "?"
		if kt.K == types.Class {
			kc = kt.C.Name
		} else {
			kc, _ = kotlinTypeSrc(kt)
		}
		s = "Map<" + kc + "," + vt + ">"
	}
	if isSetType(t) {
		et, _ := kotlinTypeSrc(t.A.List[0].T)
		s = "Set<" + et + ">"
	}
	if q != "" {
		s += " @" + q
	}
	return s
}

// container adds the bindings of a binding container's members.
func (gr *mGraph) container(k *types.ClassSym) {
	m := gr.m
	k.Ensure()
	recv := ""
	switch {
	case gr.containers[k] != "":
		recv = gr.containers[k]
	case k.Object:
		recv = metroClassName(k)
	}
	for _, mm := range m.declared(k) {
		switch {
		case find(mm.anns, mProvides) != nil:
			r := recv
			if r == "" {
				if k.Interface || k.Abstract {
					gr.unsupported("kc can't call @Provides %s of %s: put it in its companion object", mm.name, k.DisplayName())
					return
				}
				r = gr.instance(k)
			}
			gr.provides(mm, r)
		case find(mm.anns, mBinds) != nil:
			gr.binds(mm, false)
		case find(mm.anns, mMultibinds) != nil:
			gr.multibinds(mm)
		}
	}
	if comp := k.Companion; comp != nil && !k.Object {
		for _, mm := range m.declared(comp) {
			if find(mm.anns, mProvides) != nil {
				gr.provides(mm, metroClassName(comp))
			}
		}
	}
}

// instance returns the property of Impl that holds a binding container's
// instance, which Impl creates.
func (gr *mGraph) instance(k *types.ClassSym) string {
	if s, ok := gr.containers[k]; ok && s != "" {
		return s
	}
	name := fmt.Sprintf("`$c%d`", len(gr.containers))
	gr.containers[k] = name
	gr.fields = append(gr.fields, fmt.Sprintf("private val %s: %s = %s()", name, metroClassName(k), metroClassName(k)))
	return name
}

// provides adds the binding of a @Provides member, called on recv.
func (gr *mGraph) provides(mm *mMember, recv string) {
	m := gr.m
	if mm.recv != nil {
		gr.unsupported("kc doesn't support @Provides extensions (%s)", mm.name)
		return
	}
	if len(mm.params) > 0 && mm.prop {
		gr.unsupported("kc can't call @Provides %s", mm.name)
		return
	}
	var deps []*mReq
	for i, p := range mm.params {
		var pa []*mAnn
		if i < len(mm.paramAnns) {
			pa = mm.paramAnns[i]
		}
		r, ok := gr.request(p.Type, m.qualifier(pa))
		if !ok {
			gr.unsupported("kc can't name the type %s of %s's parameter %s", p.Type, mm.name, p.Name)
			return
		}
		r.name = p.Name
		r.optional = i < len(mm.defaults) && mm.defaults[i]
		deps = append(deps, r)
	}
	name := backtick(mm.name)
	call := func(args []string) string {
		target := name
		if recv != "this" {
			target = recv + "." + name
		}
		if mm.prop {
			return target
		}
		return target + "(" + namedArgs(deps, args, isSource(mm.owner) || mm.owner.KotlinName != "") + ")"
	}
	b := &mBinding{t: mm.ret, what: "@Provides " + mm.owner.DisplayName() + "." + mm.name, deps: deps, expr: call}
	gr.scope(b, m.scopesOf(mm.anns))
	gr.bind(b, mm.anns, mm.ret)
}

// namedArgs writes the arguments of a call: by name where an optional
// parameter has no binding, as Kotlin's default value then applies.
func namedArgs(deps []*mReq, args []string, kotlin bool) string {
	skip := false
	for _, a := range args {
		skip = skip || a == ""
	}
	var out []string
	for i, a := range args {
		switch {
		case a == "":
		case skip && kotlin:
			out = append(out, backtick(deps[i].name)+" = "+a)
		default:
			out = append(out, a)
		}
	}
	return strings.Join(out, ", ")
}

// scope makes a binding scoped if it has a scope annotation.
func (gr *mGraph) scope(b *mBinding, scopes []string) {
	if len(scopes) == 0 {
		return
	}
	b.scoped, b.scope = true, scopes[0]
	if !gr.scopes[b.scope] {
		gr.errorf("[Metro/IncompatiblyScopedBindings] %s is scoped (%s), but graph %s doesn't have that scope", b.what, b.scope, gr.g.DisplayName())
	}
}

// bind adds a binding of type t declared with annotations anns: as the
// key of t and its qualifier, or into a multibinding.
func (gr *mGraph) bind(b *mBinding, anns []*mAnn, t types.Type) {
	m := gr.m
	q := m.qualifier(anns)
	switch {
	case find(anns, mIntoSet) != nil:
		st := gr.setOf(t)
		mu := gr.multi(st, q)
		mu.elems = append(mu.elems, &mElem{b: b})
	case find(anns, mElementsIntoSet) != nil:
		if t.K != types.Class || t.A == nil || len(t.A.List) != 1 {
			gr.unsupported("kc can't read the element type of @ElementsIntoSet %s", b.what)
			return
		}
		st := gr.setOf(t.A.List[0].T)
		mu := gr.multi(st, q)
		mu.elems = append(mu.elems, &mElem{b: b, elements: true})
	case find(anns, mIntoMap) != nil:
		key := m.mapKeyAnn(anns)
		if key == nil {
			gr.errorf("@IntoMap %s needs a map key annotation", b.what)
			return
		}
		if err := gr.mapElem(key, t, q, b); err != nil {
			gr.unsupported("%v", err)
		}
	default:
		k, ok := keyOf(t, q)
		if !ok {
			gr.unsupported("kc can't name the type %s of %s", t, b.what)
			return
		}
		b.key = k
		gr.add(b)
	}
}

// binds adds the binding of a @Binds member: its parameter's or
// receiver's value as its type. An abstract one of the graph's is
// implemented by Impl.
func (gr *mGraph) binds(mm *mMember, inGraph bool) {
	m := gr.m
	var from types.Type
	var fromAnns []*mAnn
	switch {
	case mm.recv != nil && len(mm.params) == 0:
		from, fromAnns = *mm.recv, mm.recvAnns
	case mm.recv == nil && len(mm.params) == 1:
		from = mm.params[0].Type
		if len(mm.paramAnns) > 0 {
			fromAnns = mm.paramAnns[0]
		}
	default:
		gr.errorf("@Binds %s must have one parameter or a receiver", mm.name)
		return
	}
	dep, ok := gr.request(from, m.qualifier(fromAnns))
	if !ok {
		gr.unsupported("kc can't name the type %s of @Binds %s", from, mm.name)
		return
	}
	b := &mBinding{t: mm.ret, what: "@Binds " + mm.owner.DisplayName() + "." + mm.name, deps: []*mReq{dep}, expr: func(args []string) string { return args[0] }}
	gr.scope(b, m.scopesOf(mm.anns))
	gr.bind(b, mm.anns, mm.ret)
	if inGraph && mm.abstract {
		ret, ok1 := kotlinTypeSrc(mm.ret)
		ft, ok2 := kotlinTypeSrc(from)
		if !ok1 || !ok2 {
			gr.unsupported("kc can't implement @Binds %s", mm.name)
			return
		}
		switch {
		case mm.prop:
			gr.overrides = append(gr.overrides, fmt.Sprintf("override val %s.%s: %s get() = this", ft, backtick(mm.name), ret))
		case mm.recv != nil:
			gr.overrides = append(gr.overrides, fmt.Sprintf("override fun %s.%s(): %s = this", ft, backtick(mm.name), ret))
		default:
			gr.overrides = append(gr.overrides, fmt.Sprintf("override fun %s(instance: %s): %s = instance", backtick(mm.name), ft, ret))
		}
	}
}

// multibinds declares a multibinding.
func (gr *mGraph) multibinds(mm *mMember) {
	r, ok := gr.request(mm.ret, gr.m.qualifier(mm.anns))
	if !ok || !isMapType(r.t) && !isSetType(r.t) {
		gr.errorf("@Multibinds %s must return a Set or a Map", mm.name)
		return
	}
	mu := gr.multi(r.t, r.q)
	mu.declared = true
}

// accessor implements an abstract member of the graph: a property or a
// function that returns a binding, or a function that injects the members
// of its parameter.
func (gr *mGraph) accessor(mm *mMember) {
	m := gr.m
	if mm.recv != nil {
		gr.unsupported("kc can't implement graph member %s, an extension", mm.name)
		return
	}
	if len(mm.params) == 1 && mm.ret.K == types.Unit && !mm.prop {
		gr.injector(mm)
		return
	}
	if len(mm.params) > 0 {
		gr.unsupported("kc can't implement graph member %s, a function with parameters (graph extension factories are not supported)", mm.name)
		return
	}
	if mm.mutable {
		gr.errorf("graph accessor %s must be a val", mm.name)
		return
	}
	r, ok := gr.request(mm.ret, m.qualifier(mm.anns))
	ret, ok2 := kotlinTypeSrc(mm.ret)
	if !ok || !ok2 {
		gr.unsupported("kc can't name the type %s of graph member %s", mm.ret, mm.name)
		return
	}
	r.name = mm.name
	x := gr.expr(r, mm.owner.DisplayName()+"."+mm.name)
	if x == "" {
		return
	}
	if mm.prop {
		gr.overrides = append(gr.overrides, fmt.Sprintf("override val %s: %s get() = %s", backtick(mm.name), ret, x))
	} else {
		gr.overrides = append(gr.overrides, fmt.Sprintf("override fun %s(): %s = %s", backtick(mm.name), ret, x))
	}
}

// injector implements a graph function that injects the members of its
// parameter.
func (gr *mGraph) injector(mm *mMember) {
	t := mm.params[0].Type
	ts, ok := kotlinTypeSrc(t)
	if !ok || t.K != types.Class {
		gr.unsupported("kc can't inject the members of %s", t)
		return
	}
	stmts, ok := gr.memberInjections(t.C, "target", mm.name)
	if !ok {
		return
	}
	gr.overrides = append(gr.overrides, fmt.Sprintf("override fun %s(target: %s) {\n%s\n}", backtick(mm.name), ts, strings.Join(stmts, "\n")))
}

// memberInjections returns the statements that inject the @Inject
// properties and functions of class cls, and its superclasses', into x.
func (gr *mGraph) memberInjections(cls *types.ClassSym, x, what string) ([]string, bool) {
	m := gr.m
	var chain []*types.ClassSym
	for s := cls; s != nil && s.Name != "java/lang/Object"; s = s.Super {
		chain = append([]*types.ClassSym{s}, chain...)
	}
	var stmts []string
	for _, s := range chain {
		if !isSource(s) {
			if s.External && s.KotlinName != "" && m.hasInjectedMembers(s) {
				gr.unsupported("kc can't inject the members of %s, a class of the class path", s.DisplayName())
				return nil, false
			}
			continue
		}
		c := m.checkerOf[s]
		for _, md := range s.Decl.(*syntax.ClassDecl).Members {
			switch md := md.(type) {
			case *syntax.PropDecl:
				var own []syntax.Annotation
				for _, a := range md.Annotations {
					if !a.Accessor {
						own = append(own, a)
					}
				}
				as := c.fromSource(own, s)
				if find(as, mInject) == nil || md.Sym == nil {
					continue
				}
				r, ok := gr.request(md.Sym.Type, m.qualifier(as))
				if !ok {
					gr.unsupported("kc can't name the type %s of %s", md.Sym.Type, md.Name)
					return nil, false
				}
				v := gr.expr(r, s.DisplayName()+"."+md.Name)
				if v == "" {
					return nil, false
				}
				stmts = append(stmts, fmt.Sprintf("%s.%s = %s", x, backtick(md.Name), v))
			case *syntax.FunDecl:
				var own []syntax.Annotation
				for _, a := range md.Annotations {
					if !a.Accessor {
						own = append(own, a)
					}
				}
				as := c.fromSource(own, s)
				if find(as, mInject) == nil || md.Sym == nil {
					continue
				}
				var args []string
				for i, p := range md.Sym.Params {
					r, ok := gr.request(p.Type, m.qualifier(c.fromSource(md.Params[i].Annotations, s)))
					if !ok {
						gr.unsupported("kc can't name the type %s of %s", p.Type, p.Name)
						return nil, false
					}
					v := gr.expr(r, s.DisplayName()+"."+md.Name)
					if v == "" {
						return nil, false
					}
					args = append(args, v)
				}
				stmts = append(stmts, fmt.Sprintf("%s.%s(%s)", x, backtick(md.Name), strings.Join(args, ", ")))
			}
		}
	}
	return stmts, true
}

// hasInjectedMembers reports whether a class of the class path has
// members annotated @Inject.
func (m *metroAnalysis) hasInjectedMembers(cls *types.ClassSym) bool {
	cf := m.classfile(cls.Name)
	if cf == nil {
		return false
	}
	for _, mt := range cf.Methods {
		if mt.Name == "<init>" {
			continue
		}
		for _, a := range mt.Annots {
			if a.Type == "L"+mInject+";" {
				return true
			}
		}
	}
	return false
}

// resolve returns the binding of a request's key, or nil.
func (gr *mGraph) resolve(r *mReq) *mBinding {
	m := gr.m
	if bs := gr.bindings[r.key]; len(bs) > 0 {
		if len(bs) > 1 {
			best := bs[0]
			tie := false
			for _, b := range bs[1:] {
				switch {
				case b.priority > best.priority:
					best, tie = b, false
				case b.priority == best.priority:
					tie = true
				}
			}
			if tie {
				var whats []string
				for _, b := range bs {
					whats = append(whats, b.what)
				}
				gr.errorf("[Metro/DuplicateBinding] multiple bindings for %s: %s", r.key, strings.Join(whats, ", "))
				return nil
			}
			return best
		}
		return bs[0]
	}
	if mu := gr.multis[gr.multiKey(r.t, r.q)]; mu != nil && (isMapType(r.t) || isSetType(r.t)) {
		return gr.multiBinding(mu)
	}
	if b := gr.implicit[r.key]; b != nil {
		return b
	}
	if r.q != "" || r.t.K != types.Class || r.t.N {
		return nil
	}
	cls := r.t.C
	if anns := m.annsOf(cls); find(anns, mAssistedFactory) != nil {
		b := gr.assistedFactory(r, cls)
		if b != nil {
			gr.implicit[r.key] = b
		}
		return b
	}
	ctor, pa, defs, assisted := m.ctorOf(cls)
	if ctor == nil {
		if cls.Object && find(m.annsOf(cls), mInject) != nil {
			name := metroClassName(cls)
			b := &mBinding{key: r.key, t: r.t, what: cls.DisplayName(), expr: func([]string) string { return name }}
			gr.implicit[r.key] = b
			return b
		}
		return nil
	}
	if assisted {
		gr.errorf("%s is injected with assisted parameters: request its @AssistedFactory instead", cls.DisplayName())
		return nil
	}
	if len(cls.TypeParams) > 0 && (r.t.A == nil || r.t.HasParams()) {
		gr.unsupported("kc can't inject generic class %s", cls.DisplayName())
		return nil
	}
	b, ok := gr.ctorBinding(r, cls, ctor, pa, defs, nil)
	if !ok {
		return nil
	}
	gr.implicit[r.key] = b
	return b
}

// ctorBinding returns the binding that calls an injected constructor.
// Assisted parameters, by their position, take the values of assisted.
func (gr *mGraph) ctorBinding(r *mReq, cls *types.ClassSym, ctor *types.FunSym, pa [][]*mAnn, defs []bool, assisted map[int]string) (*mBinding, bool) {
	m := gr.m
	var deps []*mReq
	subst := func(t types.Type) types.Type {
		if r.t.A != nil && len(cls.TypeParams) > 0 {
			sub := types.Subst{}
			for i, tp := range cls.TypeParams {
				if i < len(r.t.A.List) {
					sub[tp] = r.t.A.List[i]
				}
			}
			return sub.ApplyIn(t)
		}
		return t
	}
	for i, p := range ctor.Params {
		var as []*mAnn
		if i < len(pa) {
			as = pa[i]
		}
		if _, ok := assisted[i]; ok {
			deps = append(deps, nil)
			continue
		}
		dr, ok := gr.request(subst(p.Type), m.qualifier(as))
		if !ok {
			gr.unsupported("kc can't name the type %s of %s's parameter %s", p.Type, cls.DisplayName(), p.Name)
			return nil, false
		}
		dr.name = p.Name
		dr.optional = i < len(defs) && defs[i]
		deps = append(deps, dr)
	}
	ts, ok := kotlinTypeSrc(r.t)
	if !ok {
		gr.unsupported("kc can't name %s", r.t)
		return nil, false
	}
	kotlin := isSource(cls) || cls.KotlinName != ""
	inject, ok := gr.memberInjections(cls, "it", cls.DisplayName())
	if !ok {
		return nil, false
	}
	names := make([]string, len(ctor.Params))
	for i, p := range ctor.Params {
		names[i] = p.Name
	}
	b := &mBinding{key: r.key, t: r.t, what: cls.DisplayName(), from: cls.Name}
	b.deps = slices.DeleteFunc(slices.Clone(deps), func(d *mReq) bool { return d == nil })
	b.expr = func(args []string) string {
		all := make([]string, len(deps))
		fullDeps := make([]*mReq, len(deps))
		j := 0
		for i, d := range deps {
			if d == nil {
				all[i] = assisted[i]
				fullDeps[i] = &mReq{name: names[i]}
				continue
			}
			all[i] = args[j]
			fullDeps[i] = d
			j++
		}
		x := ts + "(" + namedArgs(fullDeps, all, kotlin) + ")"
		if len(inject) > 0 {
			x += ".also {\n" + strings.Join(inject, "\n") + "\n}"
		}
		return x
	}
	gr.scope(b, m.scopesOf(m.annsOf(cls)))
	return b, true
}

// assistedFactory returns the binding of an @AssistedFactory interface:
// an object whose function calls the assisted class's constructor.
func (gr *mGraph) assistedFactory(r *mReq, f *types.ClassSym) *mBinding {
	m := gr.m
	var sam *mMember
	for _, mm := range m.inherited(f) {
		if mm.abstract {
			sam = mm
		}
	}
	if sam == nil || sam.prop || sam.ret.K != types.Class {
		gr.errorf("@AssistedFactory %s needs one abstract function", f.DisplayName())
		return nil
	}
	target := sam.ret.C
	ctor, pa, defs, assisted := m.ctorOf(target)
	if ctor == nil || !assisted && !slices.ContainsFunc(pa, func(as []*mAnn) bool { return find(as, mAssisted) != nil }) {
		gr.errorf("%s, which @AssistedFactory %s creates, has no @AssistedInject constructor", target.DisplayName(), f.DisplayName())
		return nil
	}
	var params []string
	samArgs := map[string]string{}
	for i, p := range sam.params {
		ts, ok := kotlinTypeSrc(p.Type)
		if !ok {
			gr.unsupported("kc can't name %s", p.Type)
			return nil
		}
		name := fmt.Sprintf("`$a%d`", i)
		params = append(params, name+": "+ts)
		samArgs[p.Name] = name
	}
	byPos := map[int]string{}
	for i, p := range ctor.Params {
		var as []*mAnn
		if i < len(pa) {
			as = pa[i]
		}
		if find(as, mAssisted) == nil {
			continue
		}
		arg, ok := samArgs[p.Name]
		if !ok {
			gr.errorf("@AssistedFactory %s has no parameter named %s for %s's assisted parameter", f.DisplayName(), p.Name, target.DisplayName())
			return nil
		}
		byPos[i] = arg
	}
	tr, _ := gr.request(sam.ret, "")
	inner, ok := gr.ctorBinding(tr, target, ctor, pa, defs, byPos)
	if !ok {
		return nil
	}
	if inner.scoped {
		gr.errorf("assisted-injected %s can't be scoped", target.DisplayName())
		return nil
	}
	fs, _ := kotlinTypeSrc(r.t)
	ret, _ := kotlinTypeSrc(sam.ret)
	samName := backtick(sam.name)
	return &mBinding{key: r.key, t: r.t, what: f.DisplayName(), deps: inner.deps, expr: func(args []string) string {
		return fmt.Sprintf("object : %s {\noverride fun %s(%s): %s = %s\n}", fs, samName, strings.Join(params, ", "), ret, inner.expr(args))
	}}
}

// multiBinding returns the binding of a multibinding, which collects
// its elements.
func (gr *mGraph) multiBinding(mu *mMulti) *mBinding {
	b := &mBinding{key: mu.key, t: mu.t, what: "multibinding " + mu.key, multi: mu}
	return b
}

// expr returns the Kotlin expression of a request's value, writing the
// functions it calls, or "" after reporting why it can't.
func (gr *mGraph) expr(r *mReq, site string) string {
	b := gr.resolve(r)
	if b == nil {
		if gr.failed {
			return ""
		}
		if r.optional {
			return ""
		}
		gr.errorf("[Metro/MissingBinding] Cannot find an @Inject constructor or @Provides-annotated function/property for: %s\n    %s is requested at %s", r.key, r.key, site)
		return ""
	}
	if len(r.wrap) == 0 && len(r.vwrap) == 0 && slices.Contains(gr.stack, b) {
		var path []string
		for _, s := range gr.stack[slices.Index(gr.stack, b):] {
			path = append(path, s.what)
		}
		gr.errorf("[Metro/DependencyCycle] found a dependency cycle: %s -> %s", strings.Join(path, " -> "), b.what)
		return ""
	}
	fn := gr.fun(b, r.vwrap)
	if fn == "" {
		return ""
	}
	x := fn + "()"
	if len(r.wrap) == 0 {
		return x
	}
	return gr.wrapped(r, x)
}

// wrapped returns a call of the function that returns value x wrapped as
// request r asks.
func (gr *mGraph) wrapped(r *mReq, x string) string {
	key := fmt.Sprintf("%s|%s|%s", r.key, r.wrap, r.vwrap)
	if fn, ok := gr.wrappers[key]; ok {
		return fn + "()"
	}
	ts, ok := kotlinTypeSrc(r.full)
	if !ok {
		gr.unsupported("kc can't name %s", r.full)
		return ""
	}
	inner := x
	t := r.full
	var layers []byte
	for range r.wrap {
		layers = append(layers, wrapperOf(t))
		t = t.A.List[len(t.A.List)-1].T
	}
	for i := len(layers) - 1; i >= 0; i-- {
		inner = wrapSrc(layers[i], inner, gr.innerType(r.full, i))
	}
	gr.nfun++
	fn := fmt.Sprintf("`$w%d`", gr.nfun)
	gr.wrappers[key] = fn
	gr.fields = append(gr.fields, fmt.Sprintf("private fun %s(): %s = %s", fn, ts, inner))
	return fn + "()"
}

// innerType returns the type that the i-th wrapper of t wraps.
func (gr *mGraph) innerType(t types.Type, i int) string {
	for range i + 1 {
		t = t.A.List[len(t.A.List)-1].T
	}
	s, _ := kotlinTypeSrc(t)
	return s
}

// wrapSrc writes value x, of type t, wrapped by wrapper w.
func wrapSrc(w byte, x, t string) string {
	switch w {
	case 'f':
		return "{ " + x + " }"
	case 'p':
		return "object : dev.zacsweers.metro.Provider<" + t + "> { override fun invoke(): " + t + " = " + x + " }"
	}
	return "kotlin.lazy<" + t + "> { " + x + " }"
}

// fun returns the function of Impl that returns a binding's value,
// writing it and those it calls, or "".
func (gr *mGraph) fun(b *mBinding, vwrap []byte) string {
	if b.multi != nil {
		return gr.multiFun(b.multi, vwrap)
	}
	if b.fn != "" {
		return b.fn
	}
	gr.nfun++
	n := gr.nfun
	b.fn = fmt.Sprintf("`$b%d`", n)
	gr.stack = append(gr.stack, b)
	args := make([]string, len(b.deps))
	for i, d := range b.deps {
		args[i] = gr.expr(d, b.what)
		if args[i] == "" && !d.optional {
			gr.stack = gr.stack[:len(gr.stack)-1]
			return ""
		}
	}
	gr.stack = gr.stack[:len(gr.stack)-1]
	ts, ok := kotlinTypeSrc(b.t)
	if !ok {
		gr.unsupported("kc can't name %s", b.t)
		return ""
	}
	x := b.expr(args)
	if b.scoped {
		holder := fmt.Sprintf("`$s%d`", n)
		gr.fields = append(gr.fields, fmt.Sprintf("private val %s: kotlin.Lazy<%s> = kotlin.lazy<%s> {\n%s\n}", holder, ts, ts, x))
		gr.fields = append(gr.fields, fmt.Sprintf("private fun %s(): %s = %s.value", b.fn, ts, holder))
	} else {
		gr.fields = append(gr.fields, fmt.Sprintf("private fun %s(): %s = %s", b.fn, ts, x))
	}
	return b.fn
}

// multiFun returns the function of Impl that collects a multibinding,
// with its map's values wrapped as vwrap says.
func (gr *mGraph) multiFun(mu *mMulti, vwrap []byte) string {
	key := fmt.Sprintf("%s|multi|%s", mu.key, vwrap)
	if fn, ok := gr.wrappers[key]; ok {
		return fn
	}
	if len(mu.elems) == 0 && !mu.declared {
		gr.errorf("[Metro/EmptyMultibinding] multibinding %s has no elements and is not declared @Multibinds", mu.key)
		return ""
	}
	gr.nfun++
	fn := fmt.Sprintf("`$m%d`", gr.nfun)
	gr.wrappers[key] = fn
	if mu.isMap {
		kt, ok1 := kotlinTypeSrc(mu.t.A.List[0].T)
		v := mu.t.A.List[1].T
		vt, ok2 := kotlinTypeSrc(v)
		if !ok1 || !ok2 {
			gr.unsupported("kc can't name %s", mu.t)
			return ""
		}
		for i := len(vwrap) - 1; i >= 0; i-- {
			vt = wrapTypeSrc(vwrap[i], vt)
		}
		seen := map[string]bool{}
		var entries []string
		for _, e := range mu.elems {
			if seen[e.mapKeyID] {
				gr.errorf("[Metro/DuplicateMapKeys] multibinding %s has more than one entry for key %s", mu.key, e.mapKey)
				return ""
			}
			seen[e.mapKeyID] = true
			x := gr.elemExpr(e.b, vwrap, v)
			if x == "" {
				return ""
			}
			entries = append(entries, fmt.Sprintf("kotlin.Pair<%s, %s>(%s, %s)", kt, vt, e.mapKey, x))
		}
		body := fmt.Sprintf("kotlin.collections.emptyMap<%s, %s>()", kt, vt)
		if len(entries) > 0 {
			body = fmt.Sprintf("kotlin.collections.mapOf<%s, %s>(%s)", kt, vt, strings.Join(entries, ", "))
		}
		gr.fields = append(gr.fields, fmt.Sprintf("private fun %s(): kotlin.collections.Map<%s, %s> = %s", fn, kt, vt, body))
		return fn
	}
	et, ok := kotlinTypeSrc(mu.t.A.List[0].T)
	if !ok {
		gr.unsupported("kc can't name %s", mu.t)
		return ""
	}
	var single, collections []string
	for _, e := range mu.elems {
		x := gr.elemExpr(e.b, nil, mu.t.A.List[0].T)
		if x == "" {
			return ""
		}
		if e.elements {
			collections = append(collections, x)
		} else {
			single = append(single, x)
		}
	}
	body := fmt.Sprintf("kotlin.collections.setOf<%s>(%s)", et, strings.Join(single, ", "))
	for _, x := range collections {
		body += " + " + x
	}
	gr.fields = append(gr.fields, fmt.Sprintf("private fun %s(): kotlin.collections.Set<%s> = %s", fn, et, body))
	return fn
}

func wrapTypeSrc(w byte, t string) string {
	switch w {
	case 'f':
		return "() -> " + t
	case 'p':
		return "dev.zacsweers.metro.Provider<" + t + ">"
	}
	return "kotlin.Lazy<" + t + ">"
}

// elemExpr returns the value of a multibinding's element binding,
// wrapped as vwrap says, as values of type v.
func (gr *mGraph) elemExpr(b *mBinding, vwrap []byte, v types.Type) string {
	if slices.Contains(gr.stack, b) && len(vwrap) == 0 {
		gr.errorf("[Metro/DependencyCycle] found a dependency cycle through %s", b.what)
		return ""
	}
	fn := gr.fun(b, nil)
	if fn == "" {
		return ""
	}
	x := fn + "()"
	vt, _ := kotlinTypeSrc(v)
	for i := len(vwrap) - 1; i >= 0; i-- {
		x = wrapSrc(vwrap[i], x, vt)
		vt = wrapTypeSrc(vwrap[i], vt)
	}
	return x
}

// write writes the graph's Impl class and its companion object.
func (gr *mGraph) write(d *syntax.ClassDecl, factory *types.ClassSym, create *mMember) {
	cls := gr.g
	gname := metroClassName(cls)
	var b strings.Builder
	supers := []string{gname}
	if !cls.Interface {
		supers[0] += "()"
	}
	supers = append(supers, gr.supers...)
	fmt.Fprintf(&b, "class Impl internal constructor(%s) : %s {\n", strings.Join(gr.ctorParams, ", "), strings.Join(supers, ", "))
	for _, f := range gr.fields {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	for _, o := range gr.overrides {
		b.WriteString(o)
		b.WriteByte('\n')
	}
	b.WriteString("}\n")
	members := []string{b.String()}

	var compMembers []string
	var compSuper string
	if create != nil {
		var params, args []string
		for i, p := range create.params {
			ts, _ := kotlinTypeSrc(p.Type)
			params = append(params, fmt.Sprintf("%s: %s", backtick(p.Name), ts))
			args = append(args, backtick(create.params[i].Name))
		}
		ret, _ := kotlinTypeSrc(create.ret)
		compSuper = metroClassName(factory)
		compMembers = append(compMembers, fmt.Sprintf("override fun %s(%s): %s = %s.Impl(%s)", backtick(create.name), strings.Join(params, ", "), ret, gname, strings.Join(args, ", ")))
	} else {
		compMembers = append(compMembers, fmt.Sprintf("operator fun invoke(): %s = %s.Impl()", gname, gname))
	}
	if comp := cls.Companion; comp != nil {
		gr.m.plan.members[comp.Name] = append(gr.m.plan.members[comp.Name], compMembers...)
		if compSuper != "" {
			gr.m.plan.supers[comp.Name] = append(gr.m.plan.supers[comp.Name], compSuper)
		}
	} else {
		head := "companion object"
		if compSuper != "" {
			head += " : " + compSuper
		}
		members = append(members, head+" {\n"+strings.Join(compMembers, "\n")+"\n}\n")
	}
	gr.m.plan.members[cls.Name] = append(gr.m.plan.members[cls.Name], members...)
}
