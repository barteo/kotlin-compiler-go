package ka

import (
	"slices"
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
	// sources. Name is the parameter's name. On a local class's type, a
	// TypeClass with a local ClassID (its package's and its name), Param
	// is the class's declaration node.
	Param psi.NodeID
	Name  string
	// Extension is set on a function type with a receiver, A.(B) -> R,
	// and Contexts is its number of context parameters, which come first.
	Extension bool
	Contexts  int
	// Delegated is set on a class's supertype that it implements by
	// delegation, I by x.
	Delegated bool
	// Abbrev is the typealias type the type was written as, if it was:
	// a TypeClass whose ClassID is the typealias's, with the typealias's
	// type arguments and the type's nullability. The type is the
	// typealias's expansion.
	Abbrev *Type
	// Annotations are the type's annotations as written, @A T; they are
	// its Abbrev's too, which doesn't repeat them.
	Annotations []Annotation
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
		} else if c := t.C; c.Local && !c.Anonymous {
			// A local class's type: its ID is local, its package's and
			// its name, and Param is its declaration's node.
			d, _ := c.Decl.(*syntax.ClassDecl)
			if d == nil || b.prov.Node(d) == 0 {
				out.Kind, out.Name = TypeError, b.refText(ref)
				if out.Name == "" && d != nil {
					out.Name = d.Name
				}
				return out
			}
			out.ClassID = b.f.Package
			if out.ClassID != "" {
				out.ClassID = strings.ReplaceAll(out.ClassID, ".", "/") + "/"
			}
			out.ClassID += d.Name
			out.Param = b.prov.Node(d)
		} else {
			out.ClassID = classID(t.C)
		}
		if t.A != nil {
			// The arguments' type references: an inner class's own,
			// then its outer classes', Outer<A>.Inner<B>; a function
			// type's context parameters, receiver, parameters and
			// return type.
			var argRefs []*syntax.TypeRef
			switch {
			case ref == nil || b.aliases[ref] != nil:
			case ref.Name != "":
				for r := ref; r != nil; r = r.Outer {
					for _, a := range r.Args {
						argRefs = append(argRefs, a.Type)
					}
				}
			case ref.Ret != nil:
				argRefs = append(argRefs, ref.Contexts...)
				if ref.Recv != nil {
					argRefs = append(argRefs, ref.Recv)
				}
				argRefs = append(argRefs, ref.Params...)
				argRefs = append(argRefs, ref.Ret)
			}
			if len(argRefs) != len(t.A.List) {
				argRefs = nil
			}
			for i, a := range t.A.List {
				ta := TypeArg{Variance: variance(a.V)}
				if a.V != types.Star {
					var r *syntax.TypeRef
					if argRefs != nil {
						r = argRefs[i]
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
	case t.P: // platform types are also N
		out.Flexible = true
	case t.K == types.Null || t.N:
		out.Nullable = true
	}
	if ref != nil {
		out.Annotations = b.annotations(ref.Annotations)
		if a := b.aliases[ref]; a != nil {
			out.Abbrev = b.abbrev(a, ref, out.Nullable, depth)
			substAbbrevs(out, a.Type, a.TypeParams, out.Abbrev.Args, 0)
			b.rightAbbrevs(out, a, depth)
		} else if out.Kind == TypeClass {
			out.Abbrev = b.defaultAlias(out, ref)
		}
	}
	return out
}

// defaultImports are the packages a Kotlin/JVM file imports by default
// that have typealiases.
var defaultImports = []string{"kotlin", "kotlin.collections", "kotlin.text", "kotlin.io", "kotlin.jvm",
	"kotlin.annotation", "kotlin.ranges", "kotlin.sequences", "kotlin.comparisons"}

// defaultAlias returns the typealias type that ref, which names class
// type t, writes, if it names one of the typealiases of a class that the
// checker resolves to the class: ArrayList<Int>, which is
// kotlin.collections.ArrayList<Int>; nil if it doesn't.
func (b *fileBuilder) defaultAlias(t *Type, ref *syntax.TypeRef) *Type {
	if b.lib == nil || ref.Name == "" {
		return nil
	}
	name, qualified := ref.Name, strings.Contains(ref.Name, ".")
	if qualified {
		name = ref.Name[strings.LastIndexByte(ref.Name, '.')+1:]
	}
	if a := b.classPathAlias(t, ref.Name, name, qualified); a != nil {
		return &Type{Kind: TypeClass, ClassID: a.ClassID, Args: slices.Clone(t.Args), Nullable: t.Nullable}
	}
	a := b.lib.defaultAliases()[name]
	if a == nil || a.Type.ClassID != t.ClassID || len(t.Args) != len(a.TypeParams) {
		return nil
	}
	if qualified {
		if strings.ReplaceAll(a.ClassID, "/", ".") != ref.Name {
			return nil
		}
	} else {
		for _, imp := range b.f.Imports {
			switch {
			case imp.Alias == name || imp.Alias == "" && !imp.Star && strings.HasSuffix("."+imp.Path, "."+name):
				// An explicit import names it: the typealias itself, or
				// the class.
				if strings.ReplaceAll(a.ClassID, "/", ".") != imp.Path {
					return nil
				}
			case imp.Star && !slices.Contains(defaultImports, imp.Path) &&
				b.lib.cp.Has(strings.ReplaceAll(imp.Path, ".", "/")+"/"+name):
				return nil // import java.util.*: the class, which star imports win
			}
		}
	}
	return &Type{Kind: TypeClass, ClassID: a.ClassID, Args: slices.Clone(t.Args), Nullable: t.Nullable}
}

// classPathAlias returns the class-path typealias of class type t that
// written, whose last name is name, names other than through the default
// imports, as the checker finds it: a qualified name's package, then an
// explicit import, the file's package and the star imports.
func (b *fileBuilder) classPathAlias(t *Type, written, name string, qualified bool) *Decl {
	match := func(pkg, name string) *Decl {
		if a := b.lib.packageClassAlias(pkg, name); a != nil && a.Type.ClassID == t.ClassID && len(t.Args) == len(a.TypeParams) {
			return a
		}
		return nil
	}
	split := func(path string) (string, string) {
		if i := strings.LastIndexByte(path, '.'); i >= 0 {
			return path[:i], path[i+1:]
		}
		return "", path
	}
	if qualified {
		pkg, _ := split(written)
		return match(pkg, name)
	}
	for _, imp := range b.f.Imports {
		if imp.Star {
			continue
		}
		pkg, last := split(imp.Path)
		if imp.Alias == name || imp.Alias == "" && last == name {
			return match(pkg, last)
		}
	}
	if a := match(b.f.Package, name); a != nil {
		return a
	}
	for _, imp := range b.f.Imports {
		if imp.Star && !slices.Contains(defaultImports, imp.Path) {
			if a := match(imp.Path, name); a != nil {
				return a
			}
		}
	}
	return nil
}

// abbrev returns the typealias type that ref writes, a type of typealias
// a.
func (b *fileBuilder) abbrev(a *types.AliasSym, ref *syntax.TypeRef, nullable bool, depth int) *Type {
	out := &Type{Kind: TypeClass, ClassID: aliasID(a), Nullable: nullable}
	for _, r := range ref.Args {
		ta := TypeArg{Variance: variance(r.Variance)}
		if r.Variance != types.Star {
			t, ok := b.refs[r.Type]
			if !ok {
				t = types.InvalidType
			}
			ta.Type = b.typeAt(t, r.Type, depth+1)
		}
		out.Args = append(out.Args, ta)
	}
	return out
}

// substAbbrevs puts the typealias types written as the typealias's type
// arguments args into expansion t, where its right side at, whose
// typealias's type parameters are tps, has the type parameters: L<S> is
// List<String as S> for typealias L<T> = List<T>.
func substAbbrevs(t *Type, at types.Type, tps []*types.TypeParamSym, args []TypeArg, depth int) {
	if t == nil || depth > 16 {
		return
	}
	for i, a := range t.Args {
		if a.Type == nil || at.A == nil || len(at.A.List) != len(t.Args) {
			continue
		}
		ra := at.A.List[i].T
		if ra.K != types.TypeParam {
			substAbbrevs(a.Type, ra, tps, args, depth+1)
			continue
		}
		j := slices.Index(tps, ra.Param())
		if j < 0 || j >= len(args) || args[j].Type == nil || args[j].Type.Abbrev == nil {
			continue
		}
		cp := *a.Type
		cp.Abbrev = &Type{}
		*cp.Abbrev = *args[j].Type.Abbrev
		cp.Abbrev.Nullable = cp.Nullable
		t.Args[i].Type = &cp
	}
}

// rightAbbrevs puts the typealias types that source typealias a's right
// side writes into t, its expansion: Alias43<String> for
// typealias L<T> = (Alias43<T>) -> T. The right side's own typealias,
// typealias M = Alias43<Int>, isn't: t's Abbrev is M.
func (b *fileBuilder) rightAbbrevs(t *Type, a *types.AliasSym, depth int) {
	d, ok := a.Decl.(*syntax.TypeAliasDecl)
	if !ok || d.Type == nil || depth > 8 {
		return
	}
	rt, ok := b.refs[d.Type]
	if !ok {
		return
	}
	names := make([]string, len(a.TypeParams))
	for i, p := range a.TypeParams {
		names[i] = p.Name
	}
	innerAbbrevs(t, b.typeAt(rt, d.Type, depth+1), names, t.Abbrev.Args, 0)
}

// innerAbbrevs puts the Abbrevs of the type arguments of r, the right
// side of a typealias whose type parameters are named names, into those
// of t, its expansion with type arguments args.
func innerAbbrevs(t, r *Type, names []string, args []TypeArg, depth int) {
	if t == nil || r == nil || len(t.Args) != len(r.Args) || depth > 16 {
		return
	}
	for i, ra := range r.Args {
		rt, tt := ra.Type, t.Args[i].Type
		if rt == nil || tt == nil || rt.Kind == TypeParameter && slices.Contains(names, rt.Name) {
			continue // the type argument's own, which substAbbrevs puts
		}
		cp := *tt
		cp.Args = slices.Clone(tt.Args)
		if cp.Abbrev == nil && rt.Abbrev != nil {
			cp.Abbrev = substParams(rt.Abbrev, names, args, 0)
			cp.Abbrev.Nullable = cp.Nullable
		}
		innerAbbrevs(&cp, rt, names, args, depth+1)
		t.Args[i].Type = &cp
	}
}

// substParams returns a copy of t with the type parameters named names
// replaced by args.
func substParams(t *Type, names []string, args []TypeArg, depth int) *Type {
	if t == nil || depth > 16 {
		return t
	}
	if j := slices.Index(names, t.Name); t.Kind == TypeParameter && j >= 0 && j < len(args) && args[j].Type != nil {
		cp := *args[j].Type
		cp.Nullable = cp.Nullable || t.Nullable
		return &cp
	}
	cp := *t
	cp.Args = nil
	for _, a := range t.Args {
		if a.Type != nil && a.Type.Kind == TypeParameter {
			if j := slices.Index(names, a.Type.Name); j >= 0 && j < len(args) {
				v := a.Variance
				if v == Invariant {
					v = args[j].Variance
				}
				var at *Type
				if args[j].Type == nil {
					v = Star
				} else {
					at = substParams(a.Type, names, args, depth+1)
				}
				cp.Args = append(cp.Args, TypeArg{Variance: v, Type: at})
				continue
			}
		}
		cp.Args = append(cp.Args, TypeArg{Variance: a.Variance, Type: substParams(a.Type, names, args, depth+1)})
	}
	cp.Abbrev = substParams(t.Abbrev, names, args, depth+1)
	return &cp
}

// aliasID returns the class ID of typealias a.
func aliasID(a *types.AliasSym) string {
	if a.Outer != nil {
		return classID(a.Outer) + "." + a.Name
	}
	return strings.ReplaceAll(a.Pkg, ".", "/") + "/" + a.Name
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
	if !c.External {
		// A source class's simple name may hold a $: `A$B`.
		if o := c.Outer; o != nil && strings.HasPrefix(c.Name, o.Name+"$") {
			return classID(o) + "." + c.Name[len(o.Name)+1:]
		}
		if c.Outer == nil && !c.Local && !c.Anonymous {
			return c.Name
		}
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
