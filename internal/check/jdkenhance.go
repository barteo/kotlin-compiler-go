package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Qualifiers of K2's predefined enhancements of JDK members
// (predefinedEnhancementInfo.kt), which kotlinc applies whatever the
// JDK's annotations.
type jdkQual uint8

const (
	jdkKeep        jdkQual = iota // as read: a platform type
	jdkNullable                   // T?
	jdkNotPlatform                // T: its nullability is the substitution's
	jdkNotNullable                // T & Any
)

// jdkMember is a member's enhancement: the qualifiers of its return
// type and of each parameter's type, each in preorder, the type first
// and then its arguments.
type jdkMember struct {
	ret    []jdkQual
	params [][]jdkQual
}

var (
	qNP  = []jdkQual{jdkNotPlatform}
	qNPN = []jdkQual{jdkNotPlatform, jdkNotNullable}
	qN   = []jdkQual{jdkNullable}
	qNN  = []jdkQual{jdkNotNullable}
	qNP2 = []jdkQual{jdkNotPlatform, jdkNotPlatform}
	qNP3 = []jdkQual{jdkNotPlatform, jdkNotPlatform, jdkNotPlatform}
	qNP4 = []jdkQual{jdkNotPlatform, jdkNotPlatform, jdkNotPlatform, jdkNotPlatform}
)

// jdkEnhancements are K2's predefined enhancements of JDK members, by
// class, name and parameter count, which tell its entries apart. Those
// that K2 reports as errors only from a language version on, List's
// addFirst() and the others, are errors here.
var jdkEnhancements = map[string]map[string]map[int]jdkMember{
	"java/util/Iterator": {
		"forEachRemaining": {1: {params: [][]jdkQual{qNP2}}},
	},
	"java/lang/Iterable": {
		"spliterator": {0: {ret: qNP2}},
	},
	"java/util/Collection": {
		"removeIf":       {1: {params: [][]jdkQual{qNP2}}},
		"stream":         {0: {ret: qNP2}},
		"parallelStream": {0: {ret: qNP2}},
	},
	"java/util/List": {
		"replaceAll":  {1: {params: [][]jdkQual{qNP2}}},
		"addFirst":    {1: {params: [][]jdkQual{qNP}}},
		"addLast":     {1: {params: [][]jdkQual{qNP}}},
		"removeFirst": {0: {ret: qNP}},
		"removeLast":  {0: {ret: qNP}},
	},
	"java/util/LinkedList": {
		"addFirst":    {1: {params: [][]jdkQual{qNP}}},
		"addLast":     {1: {params: [][]jdkQual{qNP}}},
		"removeFirst": {0: {ret: qNP}},
		"removeLast":  {0: {ret: qNP}},
	},
	"java/util/LinkedHashSet": {
		"addFirst":    {1: {params: [][]jdkQual{qNP}}},
		"addLast":     {1: {params: [][]jdkQual{qNP}}},
		"removeFirst": {0: {ret: qNP}},
		"removeLast":  {0: {ret: qNP}},
		"getFirst":    {0: {ret: qNP}},
		"getLast":     {0: {ret: qNP}},
	},
	"java/util/Map": {
		"forEach":     {1: {params: [][]jdkQual{qNP3}}},
		"putIfAbsent": {2: {ret: qN, params: [][]jdkQual{qNP, qNP}}},
		"replace": {
			2: {ret: qN, params: [][]jdkQual{qNP, qNP}},
			3: {params: [][]jdkQual{qNP, qNP, qNP}},
		},
		"replaceAll":       {1: {params: [][]jdkQual{qNP4}}},
		"compute":          {2: {ret: qN, params: [][]jdkQual{qNP, {jdkNotPlatform, jdkNotPlatform, jdkNullable, jdkNullable}}}},
		"computeIfAbsent":  {2: {ret: qNP, params: [][]jdkQual{qNP, qNP3}}},
		"computeIfPresent": {2: {ret: qN, params: [][]jdkQual{qNP, {jdkNotPlatform, jdkNotPlatform, jdkNotNullable, jdkNullable}}}},
		"merge":            {3: {ret: qN, params: [][]jdkQual{qNP, qNN, {jdkNotPlatform, jdkNotNullable, jdkNotNullable, jdkNullable}}}},
	},
	"java/util/LinkedHashMap": {
		"putFirst": {2: {ret: qN, params: [][]jdkQual{qNP, qNP}}},
		"putLast":  {2: {ret: qN, params: [][]jdkQual{qNP, qNP}}},
	},
	"java/util/Optional": {
		"empty":      {0: {ret: qNPN}},
		"of":         {1: {ret: qNPN, params: [][]jdkQual{qNN}}},
		"ofNullable": {1: {ret: qNPN, params: [][]jdkQual{qN}}},
		"get":        {0: {ret: qNN}},
		"ifPresent":  {1: {params: [][]jdkQual{qNPN}}},
	},
	"java/lang/ref/Reference": {
		"get": {0: {ret: qN}},
	},
	"java/util/function/Predicate":   {"test": {1: {params: [][]jdkQual{qNP}}}},
	"java/util/function/BiPredicate": {"test": {2: {params: [][]jdkQual{qNP, qNP}}}},
	"java/util/function/Consumer":    {"accept": {1: {params: [][]jdkQual{qNP}}}},
	"java/util/function/BiConsumer":  {"accept": {2: {params: [][]jdkQual{qNP, qNP}}}},
	"java/util/function/Function":    {"apply": {1: {ret: qNP, params: [][]jdkQual{qNP}}}},
	"java/util/function/BiFunction":  {"apply": {2: {ret: qNP, params: [][]jdkQual{qNP, qNP}}}},
	"java/util/function/Supplier":    {"get": {0: {ret: qNP}}},
}

// enhanceJDK applies the predefined enhancement of f, a member of the
// class named cls, if it has one.
func enhanceJDK(cls string, f *types.FunSym) {
	e, ok := jdkEnhancements[cls][f.Name][len(f.Params)]
	if !ok {
		return
	}
	if e.ret != nil {
		f.Ret, _ = applyJDKQuals(f.Ret, e.ret)
	}
	for i, q := range e.params {
		if q != nil {
			p := *f.Params[i]
			p.Type, _ = applyJDKQuals(p.Type, q)
			f.Params[i] = &p
		}
	}
}

// applyJDKQuals applies qs, in preorder, to t and its arguments, and
// returns the qualifiers left over.
func applyJDKQuals(t types.Type, qs []jdkQual) (types.Type, []jdkQual) {
	if len(qs) == 0 {
		return t, qs
	}
	switch q := qs[0]; {
	case q == jdkNullable:
		t = t.Nullable()
	case q == jdkNotPlatform && t.P:
		t.N, t.P = false, false
	case q == jdkNotNullable:
		t = t.NonNull()
		if t.IsParam() {
			t.D = true
		}
	}
	qs = qs[1:]
	if t.K != types.Class || t.A == nil || len(qs) == 0 {
		return t, qs
	}
	args := append([]types.Arg(nil), t.A.List...)
	for i := range args {
		if args[i].V == types.Star {
			qs = qs[1:]
			if len(qs) == 0 {
				break
			}
			continue
		}
		args[i].T, qs = applyJDKQuals(args[i].T, qs)
		if len(qs) == 0 {
			break
		}
	}
	t.A = types.MakeArgs(args)
	return t, qs
}

// Kotlin's collection classes, which Java's are mapped to, by the kind
// of their members' mapped signatures.
var kotlinCollectionKinds = map[string]byte{
	"kotlin/collections/Collection": 'c', "kotlin/collections/MutableCollection": 'c',
	"kotlin/collections/List": 'l', "kotlin/collections/MutableList": 'l',
	"kotlin/collections/Set": 'c', "kotlin/collections/MutableSet": 'c',
	"kotlin/collections/Map": 'm', "kotlin/collections/MutableMap": 'm',
	"java/util/Collection": 'c', "java/util/List": 'l', "java/util/Set": 'c', "java/util/Map": 'm',
}

// collectionArgs returns the instances of Kotlin's collection classes
// that Java class cls, being loaded, is a subclass of, in terms of its
// own type parameters.
func collectionArgs(cls *types.ClassSym) map[byte]types.Type {
	out := map[byte]types.Type{}
	seen := map[*types.ClassSym]bool{}
	queue := []types.Type{cls.This()}
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		if t.K != types.Class || seen[t.C] {
			continue
		}
		seen[t.C] = true
		if k, ok := kotlinCollectionKinds[t.C.Name]; ok {
			if _, dup := out[k]; !dup && t.A != nil {
				out[k] = t
				if k == 'l' {
					out['c'] = t
				}
			}
			continue
		}
		if t.C != cls {
			t.C.Ensure() // not cls, whose members are being loaded
		}
		supers := t.C.Supers
		if supers == nil {
			if t.C.Super != nil {
				supers = append(supers, types.ClassType(t.C.Super))
			}
			for _, i := range t.C.Interfaces {
				supers = append(supers, types.ClassType(i))
			}
		}
		s := classSubst(t)
		if t.C == cls {
			// cls's arguments are its own parameters, so its supertypes
			// stand as written. Substituting would ask for the variance
			// of cls's parameters where a supertype's argument is cls's
			// own type (Node<T> implements Comparable<Node<T>>), which
			// loads cls and never returns.
			s = nil
		}
		for _, st := range supers {
			if s != nil {
				st = s.Apply(st)
			}
			queue = append(queue, st)
		}
	}
	return out
}

// mapCollectionMembers gives the members of Java class cls that Kotlin
// collections declare their Kotlin signatures, as K2's JvmMappedScope
// does: ArrayList<String>'s contains(Object) is a contains(String!), and
// HashMap<String, Int>'s get(Object) a get(String!).
func mapCollectionMembers(cls *types.ClassSym) {
	if _, kind := kotlinCollectionKinds[cls.Name]; kotlinMapped[cls.Name] || !kind && len(cls.Supers) == 0 {
		return
	}
	need := false
	for _, n := range []string{"contains", "containsAll", "remove", "removeAll", "retainAll", "indexOf", "lastIndexOf", "containsKey", "containsValue", "get", "getOrDefault"} {
		if len(cls.Methods[n]) > 0 {
			need = true
			break
		}
	}
	if !need {
		return
	}
	cs := collectionArgs(cls)
	arg := func(k byte, i int) (types.Type, bool) {
		t, ok := cs[k]
		if !ok || t.A.Len() <= i || t.A.Get(i).V != types.Inv {
			return types.Type{}, false
		}
		return t.A.Get(i).T, true
	}
	retype := func(name, desc string, ps ...types.Type) {
		for i, f := range cls.Methods[name] {
			if f.Desc != desc || len(f.Params) != len(ps) {
				continue
			}
			g := *f
			g.Params = make([]*types.VarSym, len(f.Params))
			for j, p := range f.Params {
				q := *p
				q.Type = ps[j]
				g.Params[j] = &q
			}
			cls.Methods[name][i] = &g
		}
	}
	if e, ok := arg('c', 0); ok {
		retype("contains", "(Ljava/lang/Object;)Z", e)
		retype("remove", "(Ljava/lang/Object;)Z", e)
		for _, n := range []string{"containsAll", "removeAll", "retainAll"} {
			for _, f := range cls.Methods[n] {
				if f.Desc == "(Ljava/util/Collection;)Z" && len(f.Params) == 1 && f.Params[0].Type.K == types.Class {
					ct := f.Params[0].Type
					ct.A = types.MakeArgs([]types.Arg{{V: types.Out, T: e}})
					retype(n, f.Desc, ct)
					break
				}
			}
		}
	}
	if e, ok := arg('l', 0); ok {
		retype("indexOf", "(Ljava/lang/Object;)I", e)
		retype("lastIndexOf", "(Ljava/lang/Object;)I", e)
	}
	if k, ok := arg('m', 0); ok {
		retype("containsKey", "(Ljava/lang/Object;)Z", k)
		retype("get", "(Ljava/lang/Object;)Ljava/lang/Object;", k)
		retype("remove", "(Ljava/lang/Object;)Ljava/lang/Object;", k)
		if v, ok := arg('m', 1); ok {
			retype("containsValue", "(Ljava/lang/Object;)Z", v)
			retype("remove", "(Ljava/lang/Object;Ljava/lang/Object;)Z", k, v)
		}
	}
}

// pureImplementations are the JDK classes that K2 sees as purely
// implementing a Kotlin collection type (FakePureImplementationsProvider),
// by the Java interface it is on the JVM.
var pureImplementations = map[string]string{
	"java/util/ArrayList": "java/util/List", "java/util/LinkedList": "java/util/List",
	"java/util/HashSet": "java/util/Set", "java/util/TreeSet": "java/util/Set", "java/util/LinkedHashSet": "java/util/Set",
	"java/util/HashMap": "java/util/Map", "java/util/TreeMap": "java/util/Map", "java/util/LinkedHashMap": "java/util/Map",
	"java/util/concurrent/ConcurrentHashMap": "java/util/Map", "java/util/concurrent/ConcurrentSkipListMap": "java/util/Map",
}

// purelyImplement makes the members of cls, one of pureImplementations,
// that override its Kotlin collection type's take its type parameters as
// they are, not as platform types, as the Kotlin type's members do:
// ArrayList<String>'s add(E) takes a String, not a String!. Members only
// Java declares, as TreeMap's ceilingKey(K), keep platform types, and so
// do results, of which Kotlin's are sometimes nullable: Map's get. A
// member that overrides one with a predefined enhancement has it too:
// HashMap's compute() takes a function that may return null, as Map's.
func purelyImplement(cls *types.ClassSym) {
	iface, ok := pureImplementations[cls.Name]
	if !ok {
		return
	}
	var ifaces []*types.ClassSym
	seen := map[*types.ClassSym]bool{}
	var walk func(c *types.ClassSym)
	walk = func(c *types.ClassSym) {
		if c == nil || seen[c] {
			return
		}
		seen[c] = true
		if c != cls {
			c.Ensure() // not cls, whose members are being loaded
		}
		if c.Name == iface || c.Name == "java/util/Collection" && iface != "java/util/Map" {
			ifaces = append(ifaces, c)
		}
		for _, t := range c.Supers {
			if t.K == types.Class {
				walk(t.C)
			}
		}
		if len(c.Supers) == 0 {
			walk(c.Super)
			for _, i := range c.Interfaces {
				walk(i)
			}
		}
	}
	walk(cls)
	own := map[*types.TypeParamSym]bool{}
	for _, p := range cls.TypeParams {
		own[p] = true
	}
	for name, fs := range cls.Methods {
		for i, f := range fs {
			if f.Static || !slices.ContainsFunc(ifaces, func(c *types.ClassSym) bool {
				return slices.ContainsFunc(c.Methods[name], func(g *types.FunSym) bool { return !g.Static && len(g.Params) == len(f.Params) })
			}) {
				continue
			}
			g := *f
			g.Params = slices.Clone(f.Params)
			enhanceJDK(iface, &g)
			for j, p := range g.Params {
				q := *p
				q.Type = notPlatformParams(p.Type, own)
				g.Params[j] = &q
			}
			fs[i] = &g
		}
	}
}

// notPlatformParams returns t with the occurrences of the type parameters
// ps, in it and its arguments, not platform types.
func notPlatformParams(t types.Type, ps map[*types.TypeParamSym]bool) types.Type {
	if t.IsParam() && ps[t.Param()] && t.P {
		t.N, t.P = false, false
		return t
	}
	if t.K != types.Class || t.A == nil {
		return t
	}
	args := slices.Clone(t.A.List)
	for i, a := range args {
		if a.V != types.Star {
			args[i].T = notPlatformParams(a.T, ps)
		}
	}
	t.A = types.MakeArgs(args)
	return t
}
