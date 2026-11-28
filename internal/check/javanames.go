package check

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// kotlinParams names the parameters of Kotlin's collection members by the
// kind of collection (see kotlinCollectionKinds) and the JVM name and
// descriptor of the Java method that overrides them.
var kotlinParams = map[byte]map[string][]string{
	'c': {
		"contains(Ljava/lang/Object;)Z":        {"element"},
		"add(Ljava/lang/Object;)Z":             {"element"},
		"remove(Ljava/lang/Object;)Z":          {"element"},
		"containsAll(Ljava/util/Collection;)Z": {"elements"},
		"addAll(Ljava/util/Collection;)Z":      {"elements"},
		"removeAll(Ljava/util/Collection;)Z":   {"elements"},
		"retainAll(Ljava/util/Collection;)Z":   {"elements"},
	},
	'l': {
		"get(I)Ljava/lang/Object;":                   {"index"},
		"indexOf(Ljava/lang/Object;)I":               {"element"},
		"lastIndexOf(Ljava/lang/Object;)I":           {"element"},
		"add(ILjava/lang/Object;)V":                  {"index", "element"},
		"addAll(ILjava/util/Collection;)Z":           {"index", "elements"},
		"set(ILjava/lang/Object;)Ljava/lang/Object;": {"index", "element"},
		"listIterator(I)Ljava/util/ListIterator;":    {"index"},
		"subList(II)Ljava/util/List;":                {"fromIndex", "toIndex"},
	},
	'm': {
		"containsKey(Ljava/lang/Object;)Z":                                     {"key"},
		"containsValue(Ljava/lang/Object;)Z":                                   {"value"},
		"get(Ljava/lang/Object;)Ljava/lang/Object;":                            {"key"},
		"getOrDefault(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;": {"key", "defaultValue"},
		"put(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;":          {"key", "value"},
		"remove(Ljava/lang/Object;)Ljava/lang/Object;":                         {"key"},
		"remove(Ljava/lang/Object;Ljava/lang/Object;)Z":                        {"key", "value"},
		"putAll(Ljava/util/Map;)V":                                             {"from"},
	},
}

// builtinMethodParams names the parameters of the members of the Java
// classes that are Kotlin's built-in ones, by class and method name, that
// Kotlin declares otherwise than the collections' (see kotlinParams):
// other of Any's equals(other).
var builtinMethodParams = map[string][]string{
	"java/lang/Object.equals":            {"other"},
	"java/lang/Comparable.compareTo":     {"other"},
	"java/lang/CharSequence.charAt":      {"index"}, // get(index)
	"java/lang/CharSequence.subSequence": {"startIndex", "endIndex"},
}

// builtinParamNames gives the members of builtinMethodParams of class
// cls, being loaded, Kotlin's parameter names, which calls may name and
// overrides should keep.
func builtinParamNames(cls *types.ClassSym) {
	for name, fs := range cls.Methods {
		names := builtinMethodParams[cls.Name+"."+name]
		if names == nil {
			continue
		}
		for _, f := range fs {
			if len(f.Params) == len(names) && !f.Static {
				for i, p := range f.Params {
					p.Name = names[i]
				}
			}
		}
	}
}

// kotlinParamNames gives the members of Java class cls, being loaded,
// that override Kotlin collection members the names of those members'
// parameters, which calls may name, as K2 does: a HashMap's
// put(key = k, value = v).
func kotlinParamNames(cls *types.ClassSym) {
	if _, kind := kotlinCollectionKinds[cls.Name]; kotlinMapped[cls.Name] || !kind && len(cls.Supers) == 0 {
		return
	}
	var kinds []byte
	for k := range collectionArgs(cls) {
		kinds = append(kinds, k)
	}
	for _, k := range kinds {
		for name, fs := range cls.Methods {
			for i, f := range fs {
				names := kotlinParams[k][name+f.Desc]
				if names == nil || len(names) != len(f.Params) || f.Static {
					continue
				}
				g := *f
				g.Params = make([]*types.VarSym, len(f.Params))
				for j, p := range f.Params {
					q := *p
					q.Name = names[j]
					g.Params[j] = &q
				}
				fs[i] = &g
			}
		}
	}
}

// javaNamedArgs reports the named arguments of call x of Java function f
// (NAMED_ARGUMENTS_NOT_ALLOWED): Java has no stable parameter names, and
// kc names a Java method's parameters p, but for those Kotlin names (see
// kotlinParamNames).
func (c *Checker) javaNamedArgs(x *syntax.Call, f *types.FunSym) {
	if f.Owner == nil || !isJavaClass(f.Owner) || !slices.ContainsFunc(f.Params, func(p *types.VarSym) bool { return p.Name == "p" }) {
		return
	}
	for i, a := range x.Args {
		if x.Name(i) != "" {
			c.errorf(a.Pos(), "NAMED_ARGUMENTS_NOT_ALLOWED", "named arguments are not allowed for non-Kotlin functions")
			return
		}
	}
}
