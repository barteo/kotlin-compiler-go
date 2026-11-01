package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Stability.
//
// Compose's compiler plugin infers whether the values of a type are
// stable: whether an instance's equals tells when the composition reads
// something else. A composable compares its stable arguments with the
// ones it was last called with by equals, and others by identity, and
// skips its body if none changed (see compose.go). kc infers stability
// as the plugin's StabilityInferencer does, whose rules and quirks this
// file follows:
//
//   - Unit, the primitive types, String and function types are stable, a
//     nullable type is as its non-null type, and a value class type as
//     its underlying type, if the class has no stability marker
//   - a class is stable if it, or a supertype, is annotated with a
//     stability marker, @Stable or @Immutable, or is an enum or an object
//   - the program's classes in the file being compiled, and its private
//     ones, are stable if final and their properties with fields are vals
//     of stable types, and their superclass's are; a class's own type
//     parameters make it as stable as their arguments
//   - other classes of the program and of the class path have stability
//     known at run time, from their static fields $stable, which the
//     plugin, and kc, give them: 0 for a stable class, 8 (the unstable
//     bit) for others; a Java class, or a Kotlin one without
//     @StabilityInferred, is unstable, and an interface unknown
//
// A class's @StabilityInferred(parameters) tells the plugin which of its
// type parameters its stability depends on, and with the bit after them
// whether it is stable.

// stabilityKind is the kind of a stability.
type stabilityKind uint8

const (
	stable       stabilityKind = iota // known stable
	unstable                          // known unstable
	stabRuntime                       // the class's $stable says
	stabUnknown                       // an interface's, or an open class's: unknown
	stabParam                         // a type parameter's argument's
	stabCombined                      // all of elems
)

// stability is the stability of a type, as the plugin's Stability.
type stability struct {
	kind  stabilityKind
	cls   *types.ClassSym     // stabRuntime, stabUnknown
	param *types.TypeParamSym // stabParam
	elems []stability         // stabCombined
}

var (
	stableStab   = stability{kind: stable}
	unstableStab = stability{kind: unstable}
)

// plus combines s and o, as the plugin's Stability.plus.
func (s stability) plus(o stability) stability {
	switch {
	case o.kind == stable:
		return s
	case o.kind == unstable:
		return o
	case s.kind == stable:
		return o
	case s.kind == unstable:
		return s
	}
	return stability{kind: stabCombined, elems: []stability{s, o}}
}

func (s stability) knownStable() bool {
	switch s.kind {
	case stable:
		return true
	case stabCombined:
		for _, e := range s.elems {
			if !e.knownStable() {
				return false
			}
		}
		return true
	}
	return false
}

func (s stability) knownUnstable() bool {
	switch s.kind {
	case unstable:
		return true
	case stabCombined:
		for _, e := range s.elems {
			if e.knownUnstable() {
				return true
			}
		}
	}
	return false
}

func (s stability) uncertain() bool {
	switch s.kind {
	case stabRuntime, stabUnknown, stabParam:
		return true
	case stabCombined:
		for _, e := range s.elems {
			if e.uncertain() {
				return true
			}
		}
	}
	return false
}

// normalize flattens a combined stability: unstable if a part is, else
// its runtime and parameter parts, without unknown ones.
func (s stability) normalize() stability {
	if s.kind != stabCombined {
		return s
	}
	var parts []stability
	seen := map[*types.TypeParamSym]bool{}
	stack := []stability{s}
	for len(stack) > 0 {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch e.kind {
		case stabCombined:
			stack = append(stack, e.elems...)
		case unstable:
			return unstableStab
		case stabParam:
			if !seen[e.param] {
				seen[e.param] = true
				parts = append(parts, e)
			}
		case stabRuntime:
			parts = append(parts, e)
		}
	}
	return stability{kind: stabCombined, elems: parts}
}

// each calls f with s's parts.
func (s stability) each(f func(stability)) {
	if s.kind == stabCombined {
		for _, e := range s.elems {
			e.each(f)
		}
		return
	}
	f(s)
}

// stableTypes are the classes that the plugin knows are stable if the
// type arguments in their masks are.
var stableTypes = map[string]int{
	"kotlin/Pair":                                        3,
	"kotlin/Triple":                                      7,
	"kotlin/Comparator":                                  1,
	"java/util/Comparator":                               1,
	"kotlin/Result":                                      1,
	"kotlin/ranges/ClosedRange":                          1,
	"kotlin/ranges/ClosedFloatingPointRange":             1,
	"com/google/common/collect/ImmutableList":            1,
	"com/google/common/collect/ImmutableEnumMap":         3,
	"com/google/common/collect/ImmutableMap":             3,
	"com/google/common/collect/ImmutableEnumSet":         1,
	"com/google/common/collect/ImmutableSet":             1,
	"kotlinx/collections/immutable/ImmutableCollection":  1,
	"kotlinx/collections/immutable/ImmutableList":        1,
	"kotlinx/collections/immutable/ImmutableSet":         1,
	"kotlinx/collections/immutable/ImmutableMap":         3,
	"kotlinx/collections/immutable/PersistentCollection": 1,
	"kotlinx/collections/immutable/PersistentList":       1,
	"kotlinx/collections/immutable/PersistentSet":        1,
	"kotlinx/collections/immutable/PersistentMap":        3,
	"dagger/Lazy": 1,
	"kotlin/coroutines/EmptyCoroutineContext": 0,
	"java/math/BigInteger":                    0,
	"java/math/BigDecimal":                    0,
	"java/util/Locale":                        0,
}

// stableFunctions are the functions whose results the plugin knows are
// stable if the type arguments in their masks are, by package and name.
var stableFunctions = map[string]int{
	"kotlin.collections.emptyList":                   0,
	"kotlin.collections.listOf":                      1,
	"kotlin.collections.listOfNotNull":               1,
	"kotlin.collections.mapOf":                       3,
	"kotlin.collections.emptyMap":                    0,
	"kotlin.collections.setOf":                       1,
	"kotlin.collections.emptySet":                    0,
	"kotlin.to":                                      3,
	"kotlinx.collections.immutable.immutableListOf":  1,
	"kotlinx.collections.immutable.immutableSetOf":   1,
	"kotlinx.collections.immutable.immutableMapOf":   3,
	"kotlinx.collections.immutable.persistentListOf": 1,
	"kotlinx.collections.immutable.persistentSetOf":  1,
	"kotlinx.collections.immutable.persistentMapOf":  3,
}

// stabInfer infers stabilities of the types in the code of a file.
type stabInfer struct {
	g         *gen
	analyzing map[*types.ClassSym]bool
}

// stabilityOf returns the stability of values of type t in the file being
// compiled.
func (g *gen) stabilityOf(t types.Type) stability {
	if s, ok := g.shared.stabilities[t]; ok {
		return s
	}
	s := (&stabInfer{g: g, analyzing: map[*types.ClassSym]bool{}}).ofType(t, nil)
	g.shared.stabilities[t] = s
	return s
}

// ofType returns the stability of type t, whose type parameters subst
// replaces (a nil type for a star projection).
func (in *stabInfer) ofType(t types.Type, subst map[*types.TypeParamSym]*types.Arg) stability {
	switch t.K {
	case types.Invalid:
		return unstableStab
	case types.Unit, types.Bool, types.Int, types.Long, types.Double, types.Char, types.Float, types.Byte, types.Short, types.String:
		return stableStab
	case types.Any, types.Nothing, types.Null:
		return unstableStab // classes of the standard library without @StabilityInferred
	case types.Range:
		return unstableStab // IntRange and the others
	case types.TypeParam, types.TypeVar:
		p := t.Param()
		if a, ok := subst[p]; ok {
			delete(subst, p) // as the plugin, once
			defer func() { subst[p] = a }()
			if a == nil {
				return unstableStab
			}
			return in.ofType(a.T, subst)
		}
		return stability{kind: stabParam, param: p}
	}
	c := t.C
	if c.Func && !c.Suspend || c.Composable {
		return stableStab
	}
	if types.IsValue(t) {
		if in.marked(c) {
			return stableStab
		}
		return in.ofType(c.Underlying.Type, subst)
	}
	if n := t.A.Len(); n > 0 && n == len(c.TypeParams) {
		s := make(map[*types.TypeParamSym]*types.Arg, len(subst)+n)
		for k, v := range subst {
			s[k] = v
		}
		for i, p := range c.TypeParams {
			a := t.A.Get(i)
			if a.V != types.Star && (a.T.K == types.TypeParam || a.T.K == types.TypeVar) && a.T.Param() == p {
				continue
			}
			if a.V == types.Star {
				s[p] = nil
			} else {
				s[p] = &a
			}
		}
		subst = s
	}
	return in.ofClass(c, subst)
}

// ofClass returns the stability of class c's instances.
func (in *stabInfer) ofClass(c *types.ClassSym, subst map[*types.TypeParamSym]*types.Arg) stability {
	c.Ensure()
	if in.analyzing[c] {
		return unstableStab
	}
	switch {
	case in.markedDescendant(c, map[*types.ClassSym]bool{}):
		return stableStab
	case enumClass(c) || c.Object:
		return stableStab
	case protobuf(c):
		return stableStab
	}
	in.analyzing[c] = true
	defer delete(in.analyzing, c)
	if mask, ok := stableTypes[c.Name]; ok {
		m := mask
		return in.withParams(stableStab, &m, c, subst)
	}
	cd, _ := c.Decl.(*syntax.ClassDecl)
	switch {
	case c.Array || !c.External && cd == nil:
		return unstableStab // the standard library's arrays and such
	case c.External && c.KotlinName == "":
		return unstableStab // a Java class
	case c.Interface:
		return stability{kind: stabUnknown, cls: c}
	case c.External && c.StabilityInferred < 0:
		return unstableStab
	case in.forcedRuntime(c, cd):
		return in.withParams(stability{kind: stabRuntime, cls: c}, nil, c, subst)
	case c.External || cd == nil:
		m := c.StabilityInferred
		return in.withParams(stability{kind: stabRuntime, cls: c}, &m, c, subst)
	}
	s := stability{kind: stabUnknown, cls: c}
	if declFinal(cd) {
		s = stableStab
	}
	for _, p := range c.Props {
		if p.Owner != c || !p.HasField() || p.Const {
			continue
		}
		if p.Mutable {
			return unstableStab
		}
		s = s.plus(in.ofType(p.Type, subst))
	}
	if sup := c.Super; sup != nil && sup.Name != "java/lang/Object" {
		if ss := in.ofClass(sup, subst); ss.kind != stabUnknown {
			s = s.plus(ss)
		}
	}
	return s
}

// withParams adds to base the stabilities of the arguments of class c's
// type parameters in mask, or of all if mask is nil.
func (in *stabInfer) withParams(base stability, mask *int, c *types.ClassSym, subst map[*types.TypeParamSym]*types.Arg) stability {
	if mask != nil && *mask == 0 || len(c.TypeParams) == 0 {
		return base
	}
	var elems []stability
	for i, p := range c.TypeParams {
		if i >= 32 || mask != nil && *mask&(1<<i) == 0 {
			continue
		}
		if a, ok := subst[p]; ok {
			if a == nil {
				elems = append(elems, unstableStab)
			} else {
				elems = append(elems, in.ofType(a.T, subst))
			}
		} else {
			elems = append(elems, stability{kind: stabParam, param: p})
		}
	}
	return base.plus(stability{kind: stabCombined, elems: elems})
}

// forcedRuntime reports whether the stability of class c, whose
// declaration in the program is cd, if any, is known only at run time
// in the file being compiled: if it is public or internal, and declared
// in another file, or on the class path.
func (in *stabInfer) forcedRuntime(c *types.ClassSym, cd *syntax.ClassDecl) bool {
	if cd == nil {
		return c.External && !c.Private
	}
	if cd.Visibility == syntax.Private || c.Local || c.Anonymous {
		return false
	}
	return !in.g.inFile(c)
}

// inFile reports whether class c is declared in the file being compiled.
func (g *gen) inFile(c *types.ClassSym) bool {
	return g.fileDecls()[c]
}

// fileDecls returns the classes and the properties declared in the file
// being compiled, outside of functions.
func (g *gen) fileDecls() map[any]bool {
	if g.shared.fileDecls == nil {
		m := map[any]bool{}
		var add func(ds []syntax.Decl)
		add = func(ds []syntax.Decl) {
			for _, d := range ds {
				switch d := d.(type) {
				case *syntax.ClassDecl:
					m[d.Sym] = true
					add(d.Members)
				case *syntax.PropDecl:
					m[d] = true
				}
			}
		}
		add(g.file.Decls)
		g.shared.fileDecls = m
	}
	return g.shared.fileDecls
}

// declFinal reports whether the class that cd declares can't be
// subclassed.
func declFinal(cd *syntax.ClassDecl) bool {
	return !cd.Open && !cd.Abstract && !cd.Sealed && !cd.Interface
}

// enumClass reports whether c is an enum class or the class of an entry
// of one.
func enumClass(c *types.ClassSym) bool {
	if c.Entries != nil {
		return true
	}
	for s := c.Super; s != nil; s = s.Super {
		if s.Name == "java/lang/Enum" {
			return true
		}
		if s.Entries != nil {
			return true
		}
	}
	return false
}

// protobuf reports whether c is a protocol buffer's message class, which
// the plugin knows are stable.
func protobuf(c *types.ClassSym) bool {
	if !c.Final || c.Super == nil {
		return false
	}
	n := c.Super.Name
	return n == "com/google/protobuf/GeneratedMessageLite" || n == "com/google/protobuf/GeneratedMessage"
}

// markedDescendant reports whether class c, or one of its supertypes but
// Any, has a stability marker.
func (in *stabInfer) markedDescendant(c *types.ClassSym, seen map[*types.ClassSym]bool) bool {
	if c == nil || seen[c] {
		return false
	}
	seen[c] = true
	c.Ensure()
	if in.marked(c) {
		return true
	}
	if c.Super != nil && c.Super.Name != "java/lang/Object" && in.markedDescendant(c.Super, seen) {
		return true
	}
	for _, i := range c.Interfaces {
		if in.markedDescendant(i, seen) {
			return true
		}
	}
	return false
}

// marked reports whether class c is annotated with a stability marker: an
// annotation whose class is annotated @StableMarker, as @Stable and
// @Immutable are, or errorprone's @Immutable.
func (in *stabInfer) marked(c *types.ClassSym) bool {
	for _, a := range classAnnotations(c) {
		if a.Name == "com/google/errorprone/annotations/Immutable" {
			return true
		}
		a.Ensure()
		for _, m := range classAnnotations(a) {
			if m.Name == "androidx/compose/runtime/StableMarker" {
				return true
			}
		}
	}
	return false
}

// classAnnotations returns the classes of class c's annotations that its
// class file keeps.
func classAnnotations(c *types.ClassSym) []*types.ClassSym {
	cd, ok := c.Decl.(*syntax.ClassDecl)
	if !ok {
		return c.Annotations
	}
	var as []*types.ClassSym
	for _, a := range cd.Annotations {
		if a.Class != nil && a.Target == "" {
			as = append(as, a.Class)
		}
	}
	return as
}

// ofExpr returns the stability of the value of expression x, as the
// plugin's: of its type, unless a literal's or a known stable function's,
// or a val's whose value is.
func (in *stabInfer) ofExpr(x syntax.Expr) stability {
	s := in.g.stabilityOf(x.Type())
	if s.knownStable() {
		return s
	}
	switch x := x.(type) {
	case *syntax.IntLit, *syntax.DoubleLit, *syntax.FloatLit, *syntax.CharLit, *syntax.BoolLit, *syntax.NullLit:
		return stableStab
	case *syntax.StringLit:
		if len(x.Parts) <= 1 {
			return stableStab
		}
	case *syntax.Call:
		if mask, ok := stableFunctions[funFQName(x.Sym)]; ok {
			if mask == 0 {
				return stableStab
			}
			var elems []stability
			for i := 0; i < x.T.A.Len() && i < 32; i++ {
				if mask&(1<<i) != 0 {
					elems = append(elems, in.g.stabilityOf(x.T.A.Get(i).T))
				}
			}
			return stability{kind: stabCombined, elems: elems}
		}
	case *syntax.Ident:
		if v := x.Sym; v != nil && v.Kind == types.Local && !v.Mutable {
			if vs, ok := v.Decl.(*syntax.VarStmt); ok && vs.Init != nil && vs.Delegate == nil {
				return in.ofExpr(vs.Init)
			}
		}
	}
	return s
}

// funFQName returns the package and name of top-level function f, dotted.
func funFQName(f *types.FunSym) string {
	if f == nil || f.Facade == "" {
		return ""
	}
	pkg := f.Facade[:max(strings.LastIndexByte(f.Facade, '/'), 0)]
	return strings.ReplaceAll(pkg, "/", ".") + "." + f.Name
}

// paramBits returns the function that pushes the stability bits of the
// argument of a type parameter, or nil if the code doesn't know them.
type paramBits func(*types.TypeParamSym) func()

// stableExpr pushes the value of stability s's bits, the plugin's
// irStableExpression, and reports whether it did: 0 for stable, the
// $stable fields of the classes it depends on, ored, and param's for a
// type parameter's. It pushes nothing for an unknown or unstable one.
func (g *gen) stableExpr(s stability, param paramBits) bool {
	if !stableExprOK(s, param) {
		return false
	}
	n := 0
	var push func(s stability)
	push = func(s stability) {
		switch s.kind {
		case stabRuntime:
			g.code.FieldInsn(cf.GETSTATIC, s.cls.Name, "$stable", "I")
		case stabParam:
			param(s.param)()
		case stabCombined:
			for _, e := range s.elems {
				push(e)
			}
			return
		default:
			return // 0, which kotlinc's code leaves out
		}
		n++
		if n > 1 {
			g.code.Op(cf.IOR)
		}
	}
	push(s)
	if n == 0 {
		g.code.Iconst(0)
	}
	return true
}

// stableExprOK reports whether stableExpr can push s.
func stableExprOK(s stability, param paramBits) bool {
	switch s.kind {
	case stable, stabRuntime:
		return true
	case stabParam:
		return param != nil && param(s.param) != nil
	case stabCombined:
		for _, e := range s.elems {
			if !stableExprOK(e, param) {
				return false
			}
		}
		return true
	}
	return false
}

// withoutParams returns s with its type parameters' parts stable.
func (s stability) withoutParams() stability {
	switch s.kind {
	case stabParam:
		return stableStab
	case stabCombined:
		elems := make([]stability, len(s.elems))
		for i, e := range s.elems {
			elems[i] = e.withoutParams()
		}
		return stability{kind: stabCombined, elems: elems}
	}
	return s
}

// hasRuntime reports whether s depends on a class's $stable field.
func hasRuntime(s stability) bool {
	found := false
	s.each(func(e stability) { found = found || e.kind == stabRuntime })
	return found
}
