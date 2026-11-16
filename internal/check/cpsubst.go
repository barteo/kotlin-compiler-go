package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// classPathCycles reports the cycles in the hierarchy of the program's
// classes that go through the class path, CYCLIC_INHERITANCE_HIERARCHY:
// a class of the program substitutes the class path's class of its
// name, as K2's symbol providers look in the program's sources first,
// so that a class path class that names it as its supertype now
// inherits from the program's class.
//
//	// MODULE: dependency
//	interface ToSubstitute
//	interface Intermediate : ToSubstitute
//	// MODULE: main(dependency)
//	interface ToSubstitute : Intermediate // a cycle
//
// The supertype references of a class that lead back to it are
// reported. Cycles of the program's classes alone are findCycles'.
func (c *Checker) classPathCycles() {
	for _, cd := range c.classes {
		cls := cd.Sym
		if cls == nil {
			continue
		}
		for _, st := range cd.Supers {
			if st.T.K != types.Class || st.T.C == nil || st.Type == nil {
				continue
			}
			substituted := false
			if c.reachesThroughClassPath(st.T.C, cls, map[*types.ClassSym]bool{}, &substituted) && substituted {
				c.errorf(st.Type.P, "there's a cycle in the inheritance hierarchy for this type")
			}
		}
	}
}

// reachesThroughClassPath reports whether cls is target or inherits from
// it, with the class path's classes' supertypes substituted by the
// program's classes of their names. It sets *substituted if it found
// the way through such a substitution.
func (c *Checker) reachesThroughClassPath(cls, target *types.ClassSym, seen map[*types.ClassSym]bool, substituted *bool) bool {
	if cls == target {
		return true
	}
	if cls == nil || seen[cls] {
		return false
	}
	seen[cls] = true
	external := cls.External && !cls.Source
	if external {
		cls.Ensure()
	}
	supers := append([]*types.ClassSym{cls.Super}, cls.Interfaces...)
	for _, s := range supers {
		if s == nil {
			continue
		}
		sub := false
		if external && s.External {
			if src := c.u.sourceClass(s.Name); src != nil && src != s {
				s, sub = src, true
			}
		}
		found := false
		if c.reachesThroughClassPath(s, target, seen, &found) {
			*substituted = *substituted || sub || found
			return true
		}
	}
	return false
}
