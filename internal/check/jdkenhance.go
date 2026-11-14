package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

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
)

// jdkEnhancements are the predefined enhancements kc applies, by class,
// name and parameter count. Of K2's table, these are the members whose
// types Kotlin doesn't already map: Optional's.
var jdkEnhancements = map[string]map[string]map[int]jdkMember{
	"java/util/Optional": {
		"empty":      {0: {ret: qNPN}},
		"of":         {1: {ret: qNPN, params: [][]jdkQual{{jdkNotNullable}}}},
		"ofNullable": {1: {ret: qNPN, params: [][]jdkQual{{jdkNullable}}}},
		"get":        {0: {ret: []jdkQual{jdkNotNullable}}},
		"ifPresent":  {1: {params: [][]jdkQual{qNPN}}},
	},
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
		}
	}
}
