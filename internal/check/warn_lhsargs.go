package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Misplaced type arguments on a callable reference's left side.
//
// kotlinc 2.4 reads the type arguments of a callable reference's left
// side, Outer.Middle.Inner<C, B, A>::foo, as all those of the class and
// its outer classes, the innermost class's first, whichever names they
// are written on. If they are as many as the class's type parameters,
// it takes them so and warns, WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING, of
// the leftmost name, among the class and its outer classes through
// inner classes, whose own arguments don't fit its own type parameters
// (FirTypeResolverImpl.matchQualifierPartsAndClassesForLhs), or of a
// local class with the ..._IN_LOCAL_CLASS_IN_LHS_WARNING variant. Until
// language version 2.5 makes it an error, kc accepts these types as
// innerType and flatInnerType resolve them and warns as kotlinc does.

// lhsPart is a name of a callable reference's left side: its position,
// how many type arguments it has and where they are.
type lhsPart struct {
	name  string
	p     syntax.Pos
	args  int
	argsP syntax.Pos
}

// lhsParts returns the names of ref, the outermost first, or nil if the
// parser didn't record their positions.
func lhsParts(ref *syntax.TypeRef) []lhsPart {
	var segs []*syntax.TypeRef
	for seg := ref; seg != nil; seg = seg.Outer {
		segs = append(segs, seg)
	}
	var parts []lhsPart
	for i := len(segs) - 1; i >= 0; i-- {
		seg := segs[i]
		name := seg.Name
		if seg.Outer != nil {
			if !strings.HasPrefix(name, seg.Outer.Name+".") {
				return nil
			}
			name = name[len(seg.Outer.Name)+1:]
		}
		names := strings.Split(name, ".")
		if len(names) != len(seg.Parts) {
			return nil
		}
		for j, n := range names {
			part := lhsPart{name: n, p: seg.Parts[j]}
			if j == len(names)-1 {
				part.args, part.argsP = len(seg.Args), seg.ArgsP
			}
			parts = append(parts, part)
		}
	}
	return parts
}

// ownTypeParams returns how many type parameters class k declares: not
// its outer class's, nor a local class's captured ones.
func ownTypeParams(k *types.ClassSym) int {
	if k.Inner {
		return len(k.TypeParams) - k.OuterParams
	}
	own, _ := localOwnParams(k)
	return len(own)
}

// lhsArgsWarning warns of the misplaced type arguments of ref, the type
// of class cls on a callable reference's left side that kc resolved as
// t, as kotlinc does.
func (c *Checker) lhsArgsWarning(cls *types.ClassSym, ref *syntax.TypeRef, t types.Type) {
	parts := lhsParts(ref)
	if parts == nil {
		return
	}
	declared, total := 0, 0
	k := cls
	for ; k.Inner && k.Outer != nil; k = k.Outer {
		declared += ownTypeParams(k)
	}
	declared += ownTypeParams(k)
	for _, p := range parts {
		total += p.args
	}
	if total != declared {
		return // kotlinc's error, not its warning
	}
	bad, badK := -1, (*types.ClassSym)(nil)
	k = cls
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i].name != simpleName(k) {
			return
		}
		if parts[i].args != ownTypeParams(k) {
			bad, badK = i, k
		}
		if !k.Inner || k.Outer == nil {
			break
		}
		k = k.Outer
	}
	if bad < 0 {
		return
	}
	p := parts[bad].p
	if parts[bad].args > 0 && parts[bad].argsP.Line > 0 {
		p = parts[bad].argsP
	}
	expected := countArgs(ownTypeParams(badK))
	if localChain(cls) {
		c.warnf(p, "WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING",
			"%s expected for '%s'. Try to use a lambda expression instead. This will become an error in language version 2.5. See https://youtrack.jetbrains.com/issue/KTLC-388.",
			expected, classSymText(badK))
		return
	}
	c.warnf(p, "WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING",
		"%s expected for '%s'. Use the full type '%s' instead. This will become an error in language version 2.5. See https://youtrack.jetbrains.com/issue/KTLC-388.",
		expected, classSymText(badK), innerTypeText(t))
}

// localChain reports whether class k is local or an inner class of a
// local class, which kotlinc counts as local.
func localChain(k *types.ClassSym) bool {
	for ; k.Inner && k.Outer != nil; k = k.Outer {
	}
	return k.Local
}

// classSymText renders class k as kotlinc's messages do: inner class
// Middle<B, Outer(A)> : Any.
func classSymText(k *types.ClassSym) string {
	var b strings.Builder
	if k.Inner {
		b.WriteString("inner ")
	}
	switch {
	case k.Interface:
		b.WriteString("interface ")
	case k.Object:
		b.WriteString("object ")
	default:
		b.WriteString("class ")
	}
	b.WriteString(simpleName(k))
	own := ownTypeParams(k)
	tps := k.TypeParams
	if k.Inner {
		// The type parameters a local outer class captures, which an
		// inner class of it may not have yet.
		top := k
		for ; top.Inner && top.Outer != nil; top = top.Outer {
		}
		if top.Local {
			ownTop, _ := localOwnParams(top)
			for _, p := range top.TypeParams[len(ownTop):] {
				if !slices.Contains(tps, p) {
					tps = append(tps[:len(tps):len(tps)], p)
				}
			}
		}
	}
	if len(tps) > 0 {
		names := make([]string, len(tps))
		for i, p := range tps {
			names[i] = p.Name
			if i >= own {
				names[i] = "Outer(" + p.Name + ")"
			}
		}
		b.WriteString("<" + strings.Join(names, ", ") + ">")
	}
	var supers []string
	if k.Super != nil && k.Super.Name != "java/lang/Object" {
		supers = append(supers, k.Super.DisplayName())
	}
	for _, i := range k.Interfaces {
		supers = append(supers, i.DisplayName())
	}
	if len(supers) == 0 {
		supers = []string{"Any"}
	}
	b.WriteString(" : " + strings.Join(supers, ", "))
	return b.String()
}

// innerTypeText renders t, a type of an inner class, with each class's
// own type arguments on its name: Outer<A>.Middle<B>.Inner<C>.
func innerTypeText(t types.Type) string {
	if t.K != types.Class || t.A == nil {
		return t.String()
	}
	var names []string
	off := 0
	k := t.C
	for {
		own := ownTypeParams(k)
		s := simpleName(k)
		if own > 0 && off+own <= len(t.A.List) {
			args := make([]string, own)
			for i := range args {
				args[i] = t.A.List[off+i].String()
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
		off += own
		names = append(names, s)
		if k.Outer == nil || k.Local {
			break
		}
		k = k.Outer
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	s := strings.Join(names, ".")
	if t.N {
		s += "?"
	}
	return s
}
