package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Kotlin's collection types on the JVM.
//
// A class of the program that implements Kotlin's read-only collection
// types, List or Map.Entry, implements their Java interfaces on the JVM,
// java.util.List or Map$Entry, which kotlinc completes as Java expects:
//
//   - KMappedMarker marks it, among its interfaces, if it implements one of
//     the types directly.
//   - Members that Kotlin names differently from Java, such as the
//     property size and the method size(), get final bridges to the
//     Kotlin ones.
//   - Members that Java declares with an Object parameter, such as
//     contains(Object), get final bridges that return what kotlinc's do
//     (false, -1 or null) for an argument of another type than the one
//     the class gives the parameter. If the class doesn't override one, it
//     gets a method with that type that calls its superclass's.
//   - Each member of the mutable type that the class doesn't implement,
//     add(E) or Java's mutating default methods such as sort(Comparator),
//     is a stub that throws an UnsupportedOperationException.
//   - A Collection gets toArray() and toArray(T[]), which convert it with
//     the standard library's CollectionToArray.
//
// None of this is written where a superclass, such as the standard
// library's AbstractList, already has the method.

// readOnlyMsg is the message of the UnsupportedOperationException that the
// stubs of a read-only collection throw.
const readOnlyMsg = "Operation is not supported for read-only collection"

// renamedProps are the properties of Kotlin's collection types whose Java
// methods have other names.
var renamedProps = map[string]bool{"size": true, "keys": true, "values": true, "entries": true}

// renamedOrder is the order in which kotlinc writes their methods.
var renamedOrder = []string{"values", "size", "keys", "entries"}

// typeSafe are the members of Kotlin's collection types that Java declares
// with Object parameters, by Kotlin type and name: the indices of those
// parameters, and what the bridge returns for an argument of another
// type.
var typeSafe = map[string]map[string]safeBridge{
	"kotlin/collections/Collection": {"contains": {[]int{0}, falseResult}},
	"kotlin/collections/List":       {"indexOf": {[]int{0}, minusOne}, "lastIndexOf": {[]int{0}, minusOne}},
	"kotlin/collections/Map": {
		"containsKey": {[]int{0}, falseResult}, "containsValue": {[]int{0}, falseResult},
		"get": {[]int{0}, nullResult}, "getOrDefault": {[]int{0}, defaultArg},
	},
	"kotlin/collections/MutableCollection": {"remove": {[]int{0}, falseResult}},
	"kotlin/collections/MutableMap":        {"remove": {[]int{0}, nullResult}},
}

// safeBridge describes a type-safe bridge: the parameters it checks and
// its result for an argument of another type.
type safeBridge struct {
	params []int
	result int
}

// The results of type-safe bridges for arguments of other types.
const (
	falseResult = iota
	minusOne
	nullResult
	defaultArg // the last argument: getOrDefault's default value
)

// The kinds of collMethod.
const (
	collRenamed     = iota // a final bridge to the Kotlin member: size() to getSize()
	collAbstract           // the Kotlin member of an abstract class, which Java names otherwise
	collSpecialized        // a member with the class's parameter type, which calls the superclass's
	collSafeBridge         // a final type-safe bridge
	collStub               // throws an UnsupportedOperationException
	collToArray            // toArray() or toArray(T[])
	collBridge             // a bridge to the Kotlin member: remove(int) to removeAt(int)
)

// collMethod is a method that a class gets for its collection types.
type collMethod struct {
	kind       int
	name, desc string
	// target is the name and descriptor of the method a bridge calls: the
	// Kotlin member, or the superclass's for collSpecialized.
	target string
	safe   safeBridge
	// nullable is set on a type-safe bridge's checked parameters that
	// take null.
	nullable []bool
	// synthetic is set, as kotlinc sets it, on the stub or bridge with
	// Java's erased descriptor of a member that the class also has with
	// its own types: add(Object) beside add(String).
	synthetic bool
	// fun is a stub's member of the mutable type, whose parameters'
	// types and result are params and ret as the class sees them: its
	// generic signature's.
	fun    *types.FunSym
	params []*types.VarSym
	ret    types.Type
}

func (m collMethod) final() bool { return m.kind == collRenamed || m.kind == collSafeBridge }

// argSubst maps the type parameters of t's class to t's arguments.
func argSubst(t types.Type) types.Subst {
	s := types.Subst{}
	if t.C == nil {
		return s
	}
	for i, p := range t.C.TypeParams {
		if i < t.A.Len() {
			s[p] = t.A.Get(i)
		}
	}
	return s
}

// mappedSupers returns the Kotlin collection types among cls's supertypes,
// as cls sees them, nearest first.
func mappedSupers(cls *types.ClassSym) []types.Type {
	var out []types.Type
	seen := map[*types.ClassSym]bool{cls: true}
	queue := []types.Type{cls.This()}
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		c := t.C
		c.Ensure()
		s := argSubst(t)
		for _, st := range c.Supers {
			st = s.Apply(st)
			if st.K != types.Class || st.C == nil || seen[st.C] {
				continue
			}
			seen[st.C] = true
			st.C.Ensure()
			if st.C.Java != nil {
				out = append(out, st)
			}
			queue = append(queue, st)
		}
	}
	return out
}

// directlyMapped reports whether one of cls's own supertypes is a Kotlin
// collection type, which KMappedMarker then marks.
func directlyMapped(cls *types.ClassSym) bool {
	for _, st := range cls.Supers {
		if st.C != nil && st.C.Java != nil && st.C.ReadOnly == nil {
			return true
		}
	}
	return false
}

// jvmMethods returns the methods of a class with code, by name and
// descriptor, and whether each is final: those of the program are
// computed from its symbols, since its own class file may not be
// generated yet. It memoizes the result in the compilation's cache.
func (cc *Cache) jvmMethods(cls *types.ClassSym) map[string]bool {
	if cls == nil {
		return nil
	}
	if m, ok := cc.methods.Load(cls); ok {
		return m.(map[string]bool)
	}
	m := map[string]bool{}
	if cls.External {
		cls.Ensure()
		for nd, acc := range cls.JVMMethods {
			if acc&cf.AccAbstract == 0 {
				m[nd] = acc&cf.AccFinal != 0
			}
		}
	} else {
		for nd, final := range declaredMethods(cls, false) {
			m[nd] = final
		}
		for _, cm := range cc.collectionMethods(cls) {
			if cm.kind != collAbstract {
				m[cm.name+cm.desc] = cm.final()
			}
		}
	}
	cc.methods.Store(cls, m)
	return m
}

// superMethods returns the methods with code of cls's superclasses, and
// whether each is final.
func (cc *Cache) superMethods(cls *types.ClassSym) map[string]bool {
	out := map[string]bool{}
	for s := cls.Super; s != nil; s = s.Super {
		s.Ensure()
		for nd, final := range cc.jvmMethods(s) {
			if _, ok := out[nd]; !ok {
				out[nd] = final
			}
		}
	}
	return out
}

// interfaceMethods returns the methods with code of the program's
// interfaces that cls implements, as ReadOnlyArray's iterator() { ... }:
// a class needs no stub of those.
func interfaceMethods(cls *types.ClassSym) map[string]bool {
	out := map[string]bool{}
	seen := map[*types.ClassSym]bool{}
	var visit func(c *types.ClassSym)
	visit = func(c *types.ClassSym) {
		if c == nil || seen[c] || c.External {
			return
		}
		seen[c] = true
		if c.Interface {
			for nd := range declaredMethods(c, false) {
				out[nd] = true
			}
		}
		visit(c.Super)
		for _, st := range c.Supers {
			visit(st.C)
		}
	}
	visit(cls)
	return out
}

// declaredMethods returns the methods that a class of the program
// declares, by name and descriptor, and whether each is final: its
// functions and accessors, with the bridges of its overrides, and with
// abstract ones if abstract is set.
func declaredMethods(cls *types.ClassSym, abstract bool) map[string]bool {
	m := map[string]bool{}
	for _, fs := range cls.Methods {
		for _, f := range fs {
			if f.Owner != cls || f.Abstract && !abstract || f.Static {
				continue
			}
			m[types.InstanceJVM(f)+funDesc(f)] = f.Final
			for _, b := range f.Bridges {
				m[b] = false
			}
		}
	}
	for _, p := range cls.Props {
		if p.Owner != cls || p.Private || p.Const || p.JvmField || p.Abstract && !abstract {
			continue
		}
		d, ok := types.Desc(types.FieldType(p))
		if !ok {
			continue
		}
		getter := types.GetterJVM(p)
		if cls.Value && p.Getter != nil {
			getter = types.InstanceJVM(p.Getter) // the box's, not the static -impl
		}
		m[getter+"()"+desc(types.GetterType(p))] = p.Final
		if p.Mutable && !p.SetterPrivate {
			m[types.SetterJVM(p)+"("+d+")V"] = p.Final
		}
		for _, b := range p.Bridges {
			m[b] = false
		}
	}
	return m
}

// collectionMethods returns the methods that class cls of the program
// gets for its Kotlin collection types (see the top of this file).
func (cc *Cache) collectionMethods(cls *types.ClassSym) []collMethod {
	if cls.Interface || cls.External {
		return nil
	}
	supers := mappedSupers(cls)
	if len(supers) == 0 {
		return nil
	}
	inherited := cc.superMethods(cls)
	own := declaredMethods(cls, true)
	var out []collMethod
	has := func(nd string) bool {
		if _, ok := own[nd]; ok {
			return true
		}
		_, ok := inherited[nd]
		return ok
	}
	added := map[string]bool{}
	add := func(m collMethod) {
		if added[m.name+m.desc] {
			return
		}
		added[m.name+m.desc] = true
		out = append(out, m)
		own[m.name+m.desc] = m.final()
	}
	inSupers := map[*types.ClassSym]bool{}
	for _, st := range supers {
		inSupers[st.C] = true
	}
	// The collection types a superclass implements: it has their stubs
	// and toArray, as the standard library's AbstractList has List's.
	bySuper := map[*types.ClassSym]bool{}
	if cls.Super != nil {
		for _, st := range mappedSupers(cls.Super) {
			bySuper[st.C] = true
		}
	}

	// Members that Java names otherwise: size() for the property size.
	for _, st := range supers {
		s := argSubst(st)
		for _, name := range renamedOrder {
			kp := st.C.KProps[name]
			if kp == nil {
				continue
			}
			g := kp.Getter
			java := g.JVM() + funDesc(g)
			jd := java[strings.IndexByte(java, '('):]
			if p := cls.Prop(name); p != nil && p.Owner == cls && "()"+desc(types.GetterType(p)) != jd {
				// An override of another type, keys: ImmutableSet<K>: as
				// with kotlinc, a final method with its type, and a bridge
				// with Java's.
				od := "()" + desc(types.GetterType(p))
				add(collMethod{kind: collRenamed, name: g.JVM(), desc: od, target: types.GetterJVM(p) + od})
				if _, ok := own[java]; !ok && !hasInherited(inherited, java) {
					add(collMethod{kind: collBridge, name: g.JVM(), desc: jd, target: g.JVM() + od})
				}
				continue
			}
			d, ok := types.Desc(s.Apply(g.Ret))
			if !ok {
				continue
			}
			kotlin := types.GetterName(name) + "()" + d
			if final, ok := inherited[java]; ok && !final && !has(kotlin) && !added[java] {
				if _, ok := own[java]; !ok {
					// A Java superclass's, as ArrayList's size(): as with
					// kotlinc, the Kotlin member calls it, and Java's is
					// a final bridge to the Kotlin member.
					add(collMethod{kind: collSpecialized, name: types.GetterName(name), desc: "()" + d, target: java})
					add(collMethod{kind: collRenamed, name: g.JVM(), desc: jd, target: kotlin})
					continue
				}
			}
			// A Java superclass's method that isn't final, as AbstractMap's
			// size(), which the class's own property overrides: as with
			// kotlinc, a final bridge to its getter overrides it.
			final, inheritedJava := inherited[java]
			ownProp := inheritedJava && !final && has(kotlin) && cls.Prop(name) != nil && cls.Prop(name).Owner == cls
			if _, ok := own[java]; ok && !isBridge(cls, java) || added[java] || inheritedJava && !ownProp {
				continue
			}
			if !has(kotlin) {
				if !cls.Abstract {
					continue
				}
				add(collMethod{kind: collAbstract, name: types.GetterName(name), desc: "()" + d})
			}
			add(collMethod{kind: collRenamed, name: g.JVM(), desc: jd, target: kotlin})
		}
	}

	// MutableList's removeAt(int), which is Java's remove(int): a final
	// bridge with the class's result type, unboxed, and one with Java's.
	for _, st := range supers {
		if st.C.KotlinName != "kotlin/collections/MutableList" {
			continue
		}
		s := argSubst(st)
		for _, f := range st.C.Methods["removeAt"] {
			java := f.JVM() + funDesc(f)
			kd, ok1 := safeDesc(f.Params, viewRet(s, f.Ret))
			bd, ok2 := safeDesc(f.Params, s.Apply(f.Ret))
			if !ok1 || !ok2 {
				continue
			}
			kotlin := "removeAt" + kd
			if final, ok := inherited[java]; ok && !final && !has(kotlin) {
				// ArrayList's remove(int), which removeAt calls.
				add(collMethod{kind: collSpecialized, name: "removeAt", desc: kd, target: java})
			}
			if !has(kotlin) {
				if !cls.Abstract {
					continue
				}
				add(collMethod{kind: collAbstract, name: "removeAt", desc: kd})
			}
			if final, ok := inherited[f.JVM()+bd]; !ok || !final {
				add(collMethod{kind: collRenamed, name: f.JVM(), desc: bd, target: kotlin})
			}
			if !hasInherited(inherited, java) && !added[java] {
				add(collMethod{kind: collBridge, name: f.JVM(), desc: funDesc(f), target: kotlin})
			}
		}
	}

	// Type-safe bridges of the members Java declares with Object
	// parameters, where the class gives them other types.
	for _, st := range supers {
		safe := typeSafe[st.C.KotlinName]
		if safe == nil {
			continue
		}
		s := argSubst(st)
		for name, sb := range safe {
			if name == "getOrDefault" && !bySuper[st.C] {
				continue // a default method of Java's Map: only a superclass implements it
			}
			for _, f := range st.C.Methods[name] {
				if f.Platform || f.Static || name == "remove" && len(f.Params) != 1 {
					continue
				}
				java := funDesc(f)
				params := make([]*types.VarSym, len(f.Params))
				nullable := make([]bool, len(sb.params))
				for i, p := range f.Params {
					q := *p
					q.Type = s.ApplyIn(p.Type)
					if name == "remove" && q.Type.IsPrimitive() {
						// remove(Integer), not remove(int), which is
						// removeAt's on the JVM.
						q.Type = q.Type.Nullable()
					}
					params[i] = &q
				}
				for i, pi := range sb.params {
					nullable[i] = types.MayBeNull(params[pi].Type)
				}
				view, ok := safeDesc(params, viewRet(s, f.Ret))
				if !ok || view == java {
					continue // generic: the Object parameter is the class's too
				}
				if !has(name + view) {
					switch {
					case hasInherited(inherited, name+java) || name == "getOrDefault":
						add(collMethod{kind: collSpecialized, name: name, desc: view, target: name + java})
					case cls.Abstract:
						add(collMethod{kind: collAbstract, name: name, desc: view})
					}
				}
				// The bridge with Java's descriptor, and if the result's
				// type differs, one with the class's result type.
				descs := []string{java}
				if r := view[strings.IndexByte(view, ')'):]; r != java[strings.IndexByte(java, ')'):] {
					// The class's types, but Object for the checked ones.
					vps := descParams(view)
					for _, pi := range sb.params {
						vps[pi] = "Ljava/lang/Object;"
					}
					descs = append(descs, "("+strings.Join(vps, "")+r)
				}
				for i, d := range descs {
					if final, ok := inherited[name+d]; ok && final || own[name+d] {
						continue
					}
					add(collMethod{kind: collSafeBridge, name: name, desc: d, target: name + view, safe: sb, nullable: nullable,
						synthetic: i == 0 && len(descs) > 1})
				}
			}
		}
	}

	// Stubs of the mutable types' members, for the most specific read-only
	// types (List, but not its Collection) whose mutable ones the class
	// doesn't implement.
	inner := map[*types.ClassSym]bool{}
	for _, st := range supers {
		for _, sup := range mappedSupers(st.C) {
			inner[sup.C] = true
		}
	}
	defaults := interfaceMethods(cls)
	for _, st := range supers {
		m := st.C.Mutable
		if m == nil || inSupers[m] || bySuper[st.C] || inner[st.C] {
			continue
		}
		m.Ensure()
		s := types.Subst{}
		for i, p := range m.TypeParams {
			if i < st.A.Len() {
				s[p] = st.A.Get(i)
			}
		}
		for _, fs := range m.Methods {
			for _, f := range fs {
				if f.Static || f.Platform && !mutating[f.Name] || f.Owner != m {
					continue
				}
				params := make([]*types.VarSym, len(f.Params))
				for i, p := range f.Params {
					q := *p
					q.Type = s.ApplyIn(p.Type)
					params[i] = &q
				}
				erased := funDesc(f)
				view, ok := safeDesc(params, viewRet(s, f.Ret))
				if !ok || f.Platform && f.Name == "remove" {
					view = erased // Java's remove(key, value) only
				} else if f.Name == "remove" && allParams(f) {
					// remove(E) and remove(K, V) are Java's remove(Object)
					// and remove(Object, Object), as contains(E) is.
					view = erased[:strings.IndexByte(erased, ')')] + view[strings.IndexByte(view, ')'):]
				}
				for _, d := range []string{view, erased} {
					if nd := f.JVM() + d; !has(nd) && !defaults[nd] {
						add(collMethod{kind: collStub, name: f.JVM(), desc: d, synthetic: d != view,
							fun: f, params: params, ret: viewRet(s, f.Ret)})
					}
				}
			}
		}
	}

	// toArray, for Java's Collection.
	for _, st := range supers {
		if st.C.KotlinName != "kotlin/collections/Collection" || bySuper[st.C] {
			continue
		}
		for _, d := range []string{"()[Ljava/lang/Object;", "([Ljava/lang/Object;)[Ljava/lang/Object;"} {
			if !has("toArray" + d) {
				add(collMethod{kind: collToArray, name: "toArray", desc: d})
			}
		}
	}
	return out
}

// mutating are the members of Java's collection interfaces that change
// the collection, which Kotlin's read-only types don't have.
var mutating = map[string]bool{
	"add": true, "addAll": true, "remove": true, "removeAll": true, "retainAll": true, "removeIf": true,
	"clear": true, "set": true, "replaceAll": true, "sort": true, "put": true, "putAll": true,
	"putIfAbsent": true, "compute": true, "computeIfAbsent": true, "computeIfPresent": true,
	"merge": true, "replace": true, "setValue": true, "addFirst": true, "addLast": true,
	"removeFirst": true, "removeLast": true,
}

// isBridge reports whether the method nd of cls is a bridge of one of
// its overrides, which a collection member replaces.
func isBridge(cls *types.ClassSym, nd string) bool {
	for _, fs := range cls.Methods {
		for _, f := range fs {
			if f.Owner == cls && contains(f.Bridges, nd) {
				return true
			}
		}
	}
	for _, p := range cls.Props {
		if p.Owner == cls && contains(p.Bridges, nd) {
			return true
		}
	}
	return false
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func hasInherited(inherited map[string]bool, nd string) bool {
	_, ok := inherited[nd]
	return ok
}

// safeDesc returns the descriptor of a member of a collection type with
// these parameters and result, as a class sees it, or false if a type has
// none.
func safeDesc(params []*types.VarSym, ret types.Type) (string, bool) {
	d, ok := types.MethodDesc(params, ret)
	return d, ok
}

// viewRet returns a member's result type as a class sees it: boxed if
// the member declares it with a type parameter, as kotlinc boxes it.
func viewRet(s types.Subst, ret types.Type) types.Type {
	r := s.Apply(ret)
	if ret.IsParam() && r.IsPrimitive() {
		r = r.Nullable()
	}
	return r
}

// allParams reports whether f's parameters' types are type parameters.
func allParams(f *types.FunSym) bool {
	for _, p := range f.Params {
		if !p.Type.IsParam() {
			return false
		}
	}
	return len(f.Params) > 0
}

// collectionMarkers returns the marker interfaces of a class or interface
// that implements Kotlin's collection types directly: KMappedMarker for
// the read-only ones, and one for each mutable one, KMutableList.
func collectionMarkers(cls *types.ClassSym) []string {
	var out []string
	if directlyMapped(cls) {
		out = append(out, "kotlin/jvm/internal/markers/KMappedMarker")
	}
	for _, st := range cls.Supers {
		if c := st.C; c != nil && c.Java != nil && c.ReadOnly != nil {
			name := strings.TrimPrefix(c.KotlinName, "kotlin/collections/")
			name = strings.ReplaceAll(strings.Replace(name, "MutableMap.MutableEntry", "MutableMap$Entry", 1), ".", "$")
			out = append(out, "kotlin/jvm/internal/markers/K"+name)
		}
	}
	return out
}

// collectionMembers writes the methods class cls gets for its Kotlin
// collection types.
func (g *gen) collectionMembers(cls *types.ClassSym) {
	for _, m := range g.cache().collectionMethods(cls) {
		switch m.kind {
		case collAbstract:
			g.cls.AddAbstractMethod(cf.AccPublic|cf.AccAbstract, m.name, m.desc)
		case collRenamed:
			g.collBridge(m, cf.AccPublic|cf.AccFinal|cf.AccBridge)
		case collBridge:
			g.collBridge(m, cf.AccPublic|cf.AccSynthetic|cf.AccBridge)
		case collSpecialized:
			g.collSpecialized(cls, m)
		case collSafeBridge:
			g.collSafeBridge(m)
			g.markSynthetic(m)
		case collStub:
			g.collStub(m)
			g.markSynthetic(m)
		case collToArray:
			g.collToArray(m)
		}
	}
}

// markSynthetic marks the method just written for m synthetic, as a
// bridge, if m.synthetic is set.
func (g *gen) markSynthetic(m collMethod) {
	if m.synthetic {
		g.cls.LastMethod().AddAccess(cf.AccSynthetic | cf.AccBridge)
	}
}

// collMethodStart starts method m and allocates its parameters' locals,
// which it returns with their descriptors.
func (g *gen) collMethodStart(access uint16, name, d string) ([]int, []string) {
	g.code = g.cls.NewMethod(access, name, d)
	g.code.AllocLocal("L" + g.owner.Name + ";")
	ps := descParams(d)
	slots := make([]int, len(ps))
	for i, pd := range ps {
		slots[i] = g.code.AllocLocal(pd)
	}
	return slots, ps
}

// collBridge writes a bridge to the method m.target with the same
// parameters, converting the arguments and the result.
func (g *gen) collBridge(m collMethod, access uint16) {
	slots, ps := g.collMethodStart(access, m.name, m.desc)
	target := m.target[strings.IndexByte(m.target, '('):]
	if access&cf.AccSynthetic == 0 && target == m.desc {
		// keySet() for getKeys(): the Kotlin member's signature.
		if s := g.cls.MethodSignature(m.target[:strings.IndexByte(m.target, '(')], target); s != "" {
			g.cls.LastMethod().SetSignature(s)
		}
	}
	tps := descParams(target)
	g.code.Local(cf.ALOAD, 0)
	for i, pd := range ps {
		g.code.Local(loadOpOf(pd), slots[i])
		g.convertDesc(pd, tps[i])
	}
	g.invokeVirtual(g.owner, m.target[:strings.IndexByte(m.target, '(')], target)
	ret, tret := m.desc[strings.IndexByte(m.desc, ')')+1:], target[strings.IndexByte(target, ')')+1:]
	g.convertDesc(tret, ret)
	g.code.Op(returnOpOf(ret))
	g.check()
}

// collSpecialized writes a member with the class's parameter types that
// calls the superclass's member with Java's, boxing its arguments.
func (g *gen) collSpecialized(cls *types.ClassSym, m collMethod) {
	slots, ps := g.collMethodStart(cf.AccPublic, m.name, m.desc)
	target := m.target[strings.IndexByte(m.target, '('):]
	tps := descParams(target)
	g.code.Local(cf.ALOAD, 0)
	for i, pd := range ps {
		g.code.Local(loadOpOf(pd), slots[i])
		g.convertDesc(pd, tps[i])
	}
	g.invokeSpecial(cls.Super, m.target[:strings.IndexByte(m.target, '(')], target)
	ret, tret := m.desc[strings.IndexByte(m.desc, ')')+1:], target[strings.IndexByte(target, ')')+1:]
	g.convertDesc(tret, ret)
	g.code.Op(returnOpOf(ret))
	g.check()
}

// collSafeBridge writes a final type-safe bridge: for an argument that
// isn't of the type the class gives the parameter, it returns kotlinc's
// result; otherwise it calls the member with that type.
func (g *gen) collSafeBridge(m collMethod) {
	slots, ps := g.collMethodStart(cf.AccPublic|cf.AccFinal|cf.AccBridge, m.name, m.desc)
	target := m.target[strings.IndexByte(m.target, '('):]
	tps := descParams(target)
	ret := m.desc[strings.IndexByte(m.desc, ')')+1:]
	for i, pi := range m.safe.params {
		t := tps[pi]
		box := t
		if len(t) == 1 {
			box = "L" + descBox[t] + ";"
		}
		if box == "Ljava/lang/Object;" {
			continue
		}
		ok := cf.NewLabel()
		g.code.Local(cf.ALOAD, slots[pi])
		if m.nullable[i] {
			g.code.Jump(cf.IFNULL, ok)
			g.code.Local(cf.ALOAD, slots[pi])
		}
		g.code.InstanceOf(strings.TrimSuffix(strings.TrimPrefix(box, "L"), ";"))
		g.code.Jump(cf.IFNE, ok)
		switch m.safe.result {
		case falseResult:
			g.code.Iconst(0)
		case minusOne:
			g.code.Iconst(-1)
		case nullResult:
			g.code.Op(cf.ACONST_NULL)
		case defaultArg:
			last := len(ps) - 1
			g.code.Local(loadOpOf(ps[last]), slots[last])
			g.convertDesc(ps[last], ret)
		}
		g.code.Op(returnOpOf(ret))
		g.code.Place(ok)
	}
	g.code.Local(cf.ALOAD, 0)
	for i, pd := range ps {
		g.code.Local(loadOpOf(pd), slots[i])
		g.convertDesc(pd, tps[i])
	}
	g.invokeVirtual(g.owner, m.name, target)
	g.convertDesc(target[strings.IndexByte(target, ')')+1:], ret)
	g.code.Op(returnOpOf(ret))
	g.check()
}

// collStub writes a stub of a mutable collection's member, which throws.
func (g *gen) collStub(m collMethod) {
	g.collMethodStart(cf.AccPublic, m.name, m.desc)
	if m.fun != nil && !m.synthetic {
		var ps []sigPart
		special := specialParams(m.fun.Name, m.fun.Ret, m.params)
		for i, p := range m.params {
			ps = append(ps, sigPart{t: p.Type, erase: special[i] == erasedParam, anyColl: special[i] == anyCollection})
		}
		if s := methodSig(nil, ps, typedPart(m.ret, nil), m.desc, 0, inParam); s != "" {
			g.cls.LastMethod().SetSignature(s)
		}
	}
	g.code.New("java/lang/UnsupportedOperationException")
	g.code.Op(cf.DUP)
	g.code.Sconst(readOnlyMsg)
	g.code.Invoke(cf.INVOKESPECIAL, "java/lang/UnsupportedOperationException", "<init>", "(Ljava/lang/String;)V")
	g.code.Op(cf.ATHROW)
	g.check()
}

// collToArray writes toArray() or toArray(T[]) of a Collection, as kotlinc
// does: through the standard library's CollectionToArray.
func (g *gen) collToArray(m collMethod) {
	slots, _ := g.collMethodStart(cf.AccPublic, m.name, m.desc)
	if len(slots) == 1 {
		g.cls.LastMethod().SetSignature("<T:Ljava/lang/Object;>([TT;)[TT;") // Java's
	}
	const toArray = "kotlin/jvm/internal/CollectionToArray"
	if len(slots) == 0 {
		g.code.Local(cf.ALOAD, 0)
		g.code.Invoke(cf.INVOKESTATIC, toArray, "toArray", "(Ljava/util/Collection;)[Ljava/lang/Object;")
	} else {
		g.checkParam(slots[0], types.ArrayOf(types.AnyType.Nullable()), "array")
		g.code.Local(cf.ALOAD, 0)
		g.code.Local(cf.ALOAD, slots[0])
		g.code.Invoke(cf.INVOKESTATIC, toArray, "toArray", "(Ljava/util/Collection;[Ljava/lang/Object;)[Ljava/lang/Object;")
	}
	g.code.Op(cf.ARETURN)
	g.check()
}

// collectionBridge reports whether an override's bridge nd of class cls
// isn't written: a collection member replaces it, or a superclass has the
// method as final.
func (cc *Cache) collectionBridge(cls *types.ClassSym, nd string) bool {
	if final, ok := cc.superMethods(cls)[nd]; ok && final {
		return true
	}
	for _, m := range cc.collectionMethods(cls) {
		if m.name+m.desc == nd {
			return true
		}
	}
	return false
}
