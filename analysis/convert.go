package analysis

import (
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// primitives are the names of kc's built-in kinds of types.
var primitives = [...]string{
	types.Unit:    "kotlin.Unit",
	types.Nothing: "kotlin.Nothing",
	types.Null:    "kotlin.Nothing",
	types.Bool:    "kotlin.Boolean",
	types.Int:     "kotlin.Int",
	types.Long:    "kotlin.Long",
	types.Double:  "kotlin.Double",
	types.Char:    "kotlin.Char",
	types.Float:   "kotlin.Float",
	types.Byte:    "kotlin.Byte",
	types.Short:   "kotlin.Short",
	types.String:  "kotlin.String",
	types.Any:     "kotlin.Any",
}

// primitiveSupers are the supertypes of the built-in kinds.
var primitiveSupers = map[types.Kind][]string{
	types.Int:    {"kotlin.Number", "kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Long:   {"kotlin.Number", "kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Double: {"kotlin.Number", "kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Float:  {"kotlin.Number", "kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Byte:   {"kotlin.Number", "kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Short:  {"kotlin.Number", "kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Char:   {"kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Bool:   {"kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.String: {"kotlin.CharSequence", "kotlin.Comparable", "java.io.Serializable", "kotlin.Any"},
	types.Unit:   {"kotlin.Any"},
}

// kotlinNames are the Kotlin classes that Java classes are, where kc
// doesn't map them itself.
var kotlinNames = map[string]string{
	"java.lang.Object":                "kotlin.Any",
	"java.lang.Enum":                  "kotlin.Enum",
	"java.lang.Comparable":            "kotlin.Comparable",
	"java.lang.CharSequence":          "kotlin.CharSequence",
	"java.lang.Number":                "kotlin.Number",
	"java.lang.Throwable":             "kotlin.Throwable",
	"java.lang.annotation.Annotation": "kotlin.Annotation",
	"java.lang.Cloneable":             "kotlin.Cloneable",
}

// classFQ returns the fully qualified name of class c: its Kotlin class
// id or JVM binary name, dotted, as Kotlin names the Java classes it maps
// to its own.
func classFQ(c *types.ClassSym) string {
	if c == nil {
		return ""
	}
	if c.KotlinName != "" {
		return strings.ReplaceAll(c.KotlinName, "/", ".")
	}
	n := javaFQ(c)
	if k, ok := kotlinNames[n]; ok {
		return k
	}
	return n
}

// javaFQ returns the JVM binary name of c, dotted.
func javaFQ(c *types.ClassSym) string {
	return strings.NewReplacer("/", ".", "$", ".").Replace(c.Name)
}

// facadePackage returns the dotted package of a facade class's internal
// name, a/b/FooKt.
func facadePackage(facade string) string {
	i := strings.LastIndexByte(facade, '/')
	if i < 0 {
		return ""
	}
	return strings.ReplaceAll(facade[:i], "/", ".")
}

func qualify(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// funFQ returns a function's fully qualified name: its class's name and
// its own for a member, its package's and its own for a top-level one,
// and its class's and <init> for a constructor.
func funFQ(f *types.FunSym) string {
	switch {
	case f == nil:
		return ""
	case f.Owner != nil:
		return classFQ(f.Owner) + "." + f.Name
	case f.Facade != "":
		return qualify(facadePackage(f.Facade), f.Name)
	}
	return f.Name
}

// varFQ returns the fully qualified name of a property; "" for a local
// or a parameter.
func varFQ(v *types.VarSym) string {
	switch v.Kind {
	case types.Global:
		return qualify(facadePackage(v.Facade), v.Name)
	case types.Prop, types.Static:
		if v.Owner != nil {
			return classFQ(v.Owner) + "." + v.Name
		}
	}
	return ""
}

// supertypes returns the fully qualified names of c's supertypes, c
// excluded, with the Java classes of Kotlin's mapped types.
func supertypes(c *types.ClassSym) []string {
	var out []string
	seen := map[string]bool{}
	add := func(n string) {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	c.Supertypes(func(s *types.ClassSym) bool {
		if s != c {
			add(classFQ(s))
			// Java's name too, as rules match either.
			if j := javaFQ(s); s.KotlinName == "" && j != "java.lang.Object" {
				add(j)
			}
		}
		if s.Java != nil {
			add(javaFQ(s.Java))
		}
		for _, t := range s.Supers {
			if t.K == types.Class && t.C != c {
				add(classFQ(t.C))
			}
		}
		return true
	})
	if c.KotlinName != "kotlin/Any" {
		add("kotlin.Any")
	}
	return out
}

// converter converts kc's types, resolving declarations through decl.
type converter struct {
	decl func(any) *ElementID
	// supers caches classes' supertypes.
	supers map[*types.ClassSym][]string
}

func (cv *converter) supertypesOf(c *types.ClassSym) []string {
	if s, ok := cv.supers[c]; ok {
		return s
	}
	s := supertypes(c)
	cv.supers[c] = s
	return s
}

// typ converts t; nil for an invalid type or one only checking sees.
func (cv *converter) typ(t types.Type) *Type {
	if t.K == types.Invalid || t.K == types.TypeVar {
		return nil
	}
	v := cv.value(t, 0)
	return &v
}

func (cv *converter) value(t types.Type, depth int) Type {
	var out Type
	switch t.K {
	case types.Class:
		c := t.C
		out.Name = classFQ(c)
		out.SAM = c.Fun || (kotlinNames[javaFQ(c)] == "" && check.JavaSAM(c))
		out.Suspend = c.Suspend
		if depth == 0 {
			out.Supertypes = cv.supertypesOf(c)
		}
		if t.IsFunc() {
			out.Name = functionName(t)
		}
		if t.A != nil && depth < 8 {
			for _, a := range t.A.List {
				var arg Type
				if a.V == types.Star {
					arg = Type{Name: "*", Projection: "*"}
				} else {
					arg = cv.value(a.T, depth+1)
					arg.Projection = a.V.String()
				}
				out.Arguments = append(out.Arguments, arg)
			}
		}
	case types.Range:
		switch t.RangeElem().K {
		case types.Long:
			out.Name = "kotlin.ranges.LongRange"
		case types.Char:
			out.Name = "kotlin.ranges.CharRange"
		default:
			out.Name = "kotlin.ranges.IntRange"
		}
	case types.TypeParam, types.TypeVar:
		p := t.Param()
		out.Name = p.Name
		out.Parameter = true
		if cv.decl != nil {
			switch o := p.Owner.(type) {
			case *types.ClassSym:
				out.ParameterOwner = cv.decl(o.Decl)
			case *types.FunSym:
				out.ParameterOwner = cv.decl(o.Decl)
			}
		}
	default:
		if int(t.K) < len(primitives) {
			out.Name = primitives[t.K]
		}
		if depth == 0 {
			out.Supertypes = primitiveSupers[t.K]
		}
	}
	switch {
	case t.K == types.Null:
		out.Nullable, out.MarkedNullable = true, true
	case t.P:
		out.Nullable, out.Platform = true, true
	case t.N:
		out.Nullable, out.MarkedNullable = true, true
	case types.MayBeNull(t):
		// T, whose bound is nullable, holds null as Kotlin's
		// TypeUtils.isNullableType says.
		out.Nullable = true
	}
	return out
}

// functionName returns the name of a function type's interface as
// Kotlin names it, by its number of parameters, the receiver's included:
// kotlin.Function1, or kotlin.coroutines.SuspendFunction1 for a suspend
// one. On the JVM a suspend one is a Function of one more.
func functionName(t types.Type) string {
	n := strconv.Itoa(len(t.A.List) - 1)
	if t.C.Suspend {
		return "kotlin.coroutines.SuspendFunction" + n
	}
	return "kotlin.Function" + n
}
