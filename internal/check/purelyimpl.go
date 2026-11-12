package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// How Kotlin sees the JDK's collection classes, beyond mapping their
// interfaces to its own collection types:
//
//   - A Java method that overrides one of the collection types' members
//     with an erased parameter, contains(Object) for contains(E), is seen
//     with the Kotlin member's signature, on every Java class (kotlinc's
//     BuiltinMethodsWithSpecialGenericSignature): a HashMap<String, Int>
//     has no containsKey(1), as a Vector<String> has no contains(1).
//   - The classes of purelyImplemented implement their Kotlin collection
//     type purely: their type arguments aren't platform types
//     (FakePureImplementationsProvider). An ArrayList<String> is a
//     MutableList<String>, not a MutableList<String!>, and its members
//     that override MutableList's take and return what MutableList's do:
//     ArrayList<String>().add(null) is an error.

// purelyImplemented maps the JDK classes that implement a Kotlin
// collection type purely to that type.
var purelyImplemented = map[string]string{
	"java/util/ArrayList":                        "kotlin/collections/MutableList",
	"java/util/LinkedList":                       "kotlin/collections/MutableList",
	"java/util/HashSet":                          "kotlin/collections/MutableSet",
	"java/util/TreeSet":                          "kotlin/collections/MutableSet",
	"java/util/LinkedHashSet":                    "kotlin/collections/MutableSet",
	"java/util/HashMap":                          "kotlin/collections/MutableMap",
	"java/util/TreeMap":                          "kotlin/collections/MutableMap",
	"java/util/LinkedHashMap":                    "kotlin/collections/MutableMap",
	"java/util/concurrent/ConcurrentHashMap":     "kotlin/collections/MutableMap",
	"java/util/concurrent/ConcurrentSkipListMap": "kotlin/collections/MutableMap",
}

// erasedParamMembers are the Java collection members, by name and
// descriptor, whose Object or Collection<?> parameters Kotlin's
// collection types declare with their type parameters.
var erasedParamMembers = map[string]bool{
	"contains(Ljava/lang/Object;)Z":                                        true,
	"remove(Ljava/lang/Object;)Z":                                          true,
	"containsAll(Ljava/util/Collection;)Z":                                 true,
	"removeAll(Ljava/util/Collection;)Z":                                   true,
	"retainAll(Ljava/util/Collection;)Z":                                   true,
	"indexOf(Ljava/lang/Object;)I":                                         true,
	"lastIndexOf(Ljava/lang/Object;)I":                                     true,
	"containsKey(Ljava/lang/Object;)Z":                                     true,
	"containsValue(Ljava/lang/Object;)Z":                                   true,
	"get(Ljava/lang/Object;)Ljava/lang/Object;":                            true,
	"remove(Ljava/lang/Object;)Ljava/lang/Object;":                         true,
	"remove(Ljava/lang/Object;Ljava/lang/Object;)Z":                        true,
	"getOrDefault(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;": true,
}

// erasedParamNames are the names of erasedParamMembers.
var erasedParamNames = func() map[string]bool {
	m := map[string]bool{}
	for k := range erasedParamMembers {
		m[k[:strings.IndexByte(k, '(')]] = true
	}
	return m
}()

// pureSupers returns the supertypes of a purely implemented class cls,
// with type parameters ps, from its signature's: its Kotlin collection
// type with its own type parameters as the arguments, first, so that it
// is the collection type that cls is seen as, in place of the Java
// interface's platform type; and a purely implemented superclass, as
// LinkedHashSet's HashSet, with them too.
func (t *classTable) pureSupers(cls *types.ClassSym, ps []*types.TypeParamSym, supers []types.Type) []types.Type {
	id, ok := purelyImplemented[cls.Name]
	if !ok || !t.kotlin {
		return supers
	}
	want := 1
	if id == "kotlin/collections/MutableMap" {
		want = 2
	}
	pure := t.kclass(id)
	if pure == nil || len(ps) != want {
		return supers
	}
	args := make([]types.Arg, len(ps))
	for i, p := range ps {
		args[i] = types.Arg{T: types.ParamType(p)}
	}
	out := []types.Type{types.GenericType(pure, args)}
	for _, st := range supers {
		switch {
		case st.C == pure:
			continue
		case purelyImplemented[st.C.Name] != "" && st.A != nil && st.A.Len() == len(ps):
			st = types.GenericType(st.C, args)
		}
		out = append(out, st)
	}
	return out
}

// kotlinView returns Java method f as Kotlin sees it, when f overrides a
// member of Kotlin's collection types that Kotlin sees with its own
// signature (see erasedParamMembers and purelyImplemented): a copy of f
// with the member's parameter types, and for a purely implemented class
// its result type, in f's class's type parameters. Otherwise it returns
// f. The copy is the same method on the JVM.
func (c *Checker) kotlinView(f *types.FunSym) *types.FunSym {
	if !c.u.ext.kotlin || f.Owner == nil || !f.Owner.External || f.Owner.KotlinName != "" || f.Static || f.Decl != nil ||
		f.Name == "<init>" || len(f.TypeParams) > 0 || f.Desc == "" {
		return f
	}
	if purelyImplemented[f.Owner.Name] == "" && !erasedParamNames[f.Name] {
		return f
	}
	if v, ok := c.u.ext.views.Load(f); ok {
		return v.(*types.FunSym)
	}
	v, _ := c.u.ext.views.LoadOrStore(f, c.kotlinView1(f))
	return v.(*types.FunSym)
}

func (c *Checker) kotlinView1(f *types.FunSym) *types.FunSym {
	owner := f.Owner
	pure := purelyImplemented[owner.Name] != ""
	if !pure && !erasedParamMembers[f.Name+f.Desc] {
		return f
	}
	// The member f overrides: one Kotlin declares, rather than one only
	// the Java interface does, and a mutable type's, as MutableIterable's
	// iterator(), rather than a read-only type's.
	var g *types.FunSym
	rank := func(h *types.FunSym) int {
		r := 0
		if !h.Platform {
			r += 2
		}
		if h.Owner.ReadOnly != nil {
			r++
		}
		return r
	}
	owner.Supertypes(func(s *types.ClassSym) bool {
		if _, ok := collectionJVM[s.KotlinName]; !ok {
			return true
		}
		for _, fs := range s.Methods {
			for _, h := range fs {
				if h.Static || h.JVM() != f.Name || h.Desc != f.Desc || len(h.TypeParams) > 0 || len(h.Params) != len(f.Params) {
					continue
				}
				if g == nil || rank(h) > rank(g) {
					g = h
				}
			}
		}
		return true
	})
	if g == nil {
		return f
	}
	st, ok := c.asSuper(owner.This(), g.Owner)
	if !ok || st.A == nil || slices.ContainsFunc(st.A.List, func(a types.Arg) bool { return a.V != types.Inv }) {
		return f // a raw supertype
	}
	sub := classSubst(st)
	// A member only the Java interface declares has its platform types:
	// as kotlinc sees it through a purely implemented class, its type
	// parameters are the Kotlin type's.
	own := func(t types.Type) types.Type {
		if g.Platform && pure && t.P && t.K == types.TypeParam && t.Param().Owner == g.Owner {
			return types.ParamType(t.Param())
		}
		return t
	}
	v := *f
	v.Params = make([]*types.VarSym, len(f.Params))
	for i, p := range f.Params {
		q := *p
		q.Type = sub.Apply(own(g.Params[i].Type))
		v.Params[i] = &q
	}
	if pure {
		v.Ret = sub.Apply(own(g.Ret))
	}
	return &v
}
