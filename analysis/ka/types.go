package ka

import (
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TypeKind is the kind of a Type. The numbers are part of libkc's ABI:
// append only.
type TypeKind uint8

const (
	// TypeClass is a class type, function types included: their
	// interface's, kotlin/Function2, with the receiver and context
	// parameters, if any, first among its arguments.
	TypeClass TypeKind = iota + 1
	// TypeParameter is a type parameter's type, T.
	TypeParameter
	// TypeError is a type the checker didn't resolve. Name is its
	// presentable text: the type as written, if it was.
	TypeError
)

// Variance is a type argument's projection, or a type parameter's
// declaration-site variance. The numbers are part of libkc's ABI.
type Variance uint8

const (
	Invariant Variance = iota
	In
	Out
	Star // *: a TypeArg with no Type
)

// Type is a type in the model.
type Type struct {
	Kind TypeKind
	// ClassID is a class type's class, as Kotlin's ClassId.asString():
	// kotlin/collections/List.
	ClassID string
	Args    []TypeArg
	// Nullable is set on a type marked nullable, T?.
	Nullable bool
	// Flexible is set on a platform type from Java, T!: its lower bound
	// is the type without Nullable, its upper bound the nullable one.
	Flexible bool
	// DefinitelyNonNull is set on a type parameter's type T & Any.
	DefinitelyNonNull bool
	// Param is a TypeParameter's TYPE_PARAMETER node, in the file of the
	// declaration the type belongs to; 0 if the parameter isn't in the
	// sources. Name is the parameter's name.
	Param psi.NodeID
	Name  string
	// Extension is set on a function type with a receiver, A.(B) -> R,
	// and Contexts is its number of context parameters, which come first.
	Extension bool
	Contexts  int
}

// TypeArg is a type argument.
type TypeArg struct {
	Variance Variance
	Type     *Type // nil for Star
}

// primitives are the class IDs of kc's built-in kinds of types.
var primitives = [...]string{
	types.Unit:    "kotlin/Unit",
	types.Nothing: "kotlin/Nothing",
	types.Null:    "kotlin/Nothing",
	types.Bool:    "kotlin/Boolean",
	types.Int:     "kotlin/Int",
	types.Long:    "kotlin/Long",
	types.Double:  "kotlin/Double",
	types.Char:    "kotlin/Char",
	types.Float:   "kotlin/Float",
	types.Byte:    "kotlin/Byte",
	types.Short:   "kotlin/Short",
	types.String:  "kotlin/String",
	types.Any:     "kotlin/Any",
}

// kotlinClasses are the Kotlin classes that Java classes are, where kc
// doesn't map them itself.
var kotlinClasses = map[string]string{
	"java/lang/Object":                "kotlin/Any",
	"java/lang/Enum":                  "kotlin/Enum",
	"java/lang/Comparable":            "kotlin/Comparable",
	"java/lang/CharSequence":          "kotlin/CharSequence",
	"java/lang/Number":                "kotlin/Number",
	"java/lang/Throwable":             "kotlin/Throwable",
	"java/lang/annotation/Annotation": "kotlin/Annotation",
	"java/lang/Cloneable":             "kotlin/Cloneable",
}

// variance returns v as the model's Variance.
func variance(v types.Variance) Variance {
	switch v {
	case types.In:
		return In
	case types.Out:
		return Out
	case types.Star:
		return Star
	}
	return Invariant
}

// typ returns the model of type t, which ref, if not nil, wrote: an
// error type's text is ref's. It returns nil for a type the model can't
// have: an inference variable that leaked.
func (b *fileBuilder) typ(t types.Type, ref *syntax.TypeRef) *Type {
	return b.typeAt(t, ref, 0)
}

func (b *fileBuilder) typeAt(t types.Type, ref *syntax.TypeRef, depth int) *Type {
	if depth > 16 {
		return &Type{Kind: TypeError, Name: "<too deep>"}
	}
	out := &Type{}
	switch t.K {
	case types.Invalid, types.TypeVar:
		out.Kind = TypeError
		out.Name = b.refText(ref)
		return out
	case types.Class:
		out.Kind = TypeClass
		if t.IsFunc() {
			n := strconv.Itoa(len(t.A.List) - 1)
			out.ClassID = "kotlin/Function" + n
			if t.C.Suspend {
				out.ClassID = "kotlin/coroutines/SuspendFunction" + n
			}
			out.Extension, out.Contexts = t.E, int(t.X)
		} else {
			out.ClassID = classID(t.C)
		}
		if t.A != nil {
			var argRefs []*syntax.TypeArg
			if ref != nil && ref.Name != "" && len(ref.Args) == len(t.A.List) {
				argRefs = ref.Args
			}
			for i, a := range t.A.List {
				ta := TypeArg{Variance: variance(a.V)}
				if a.V != types.Star {
					var r *syntax.TypeRef
					if argRefs != nil {
						r = argRefs[i].Type
					}
					ta.Type = b.typeAt(a.T, r, depth+1)
				}
				out.Args = append(out.Args, ta)
			}
		}
	case types.Range:
		out.Kind = TypeClass
		switch t.RangeElem().K {
		case types.Long:
			out.ClassID = "kotlin/ranges/LongRange"
		case types.Char:
			out.ClassID = "kotlin/ranges/CharRange"
		default:
			out.ClassID = "kotlin/ranges/IntRange"
		}
	case types.TypeParam:
		p := t.Param()
		if p.Intersection {
			out.Kind = TypeError // TODO(M4): KaIntersectionType
			out.Name = t.String()
			return out
		}
		out.Kind = TypeParameter
		out.Name = p.Name
		for q := p; q != nil && out.Param == 0; q = q.Origin {
			out.Param = b.tparams[q]
		}
		out.DefinitelyNonNull = t.D
	default:
		if int(t.K) >= len(primitives) || primitives[t.K] == "" {
			out.Kind = TypeError
			out.Name = t.String()
			return out
		}
		out.Kind = TypeClass
		out.ClassID = primitives[t.K]
	}
	switch {
	case t.K == types.Null || t.N:
		out.Nullable = true
	case t.P:
		out.Flexible = true
	}
	return out
}

// refText returns the text of type ref as written, or "" for nil.
func (b *fileBuilder) refText(ref *syntax.TypeRef) string {
	if ref == nil {
		return ""
	}
	if id := b.prov.Node(ref); id != 0 {
		return b.snap.Node(id).Text()
	}
	return ref.Name
}

// classID returns the class ID of c, or "" for nil.
func classID(c *types.ClassSym) string {
	if c == nil {
		return ""
	}
	if c.KotlinName != "" {
		return c.KotlinName
	}
	return internalClassID(c.Name)
}

// internalClassID returns the class ID of the class with JVM internal name
// name: a/b/Outer.Inner for a/b/Outer$Inner.
func internalClassID(name string) string {
	if k, ok := kotlinClasses[name]; ok {
		return k
	}
	pkg, rel := "", name
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		pkg, rel = name[:i+1], name[i+1:]
	}
	return pkg + strings.ReplaceAll(rel, "$", ".")
}
