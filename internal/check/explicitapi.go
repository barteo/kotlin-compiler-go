package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// APIMode is how kotlinc's explicit API mode (-Xexplicit-api) or its
// explicit return types mode (-XXexplicit-return-types) reports what it
// requires: not at all, as warnings or as errors.
type APIMode uint8

const (
	APIDisabled APIMode = iota
	APIWarning
	APIStrict
)

// ParseAPIMode returns the mode kotlinc's option value s names: disable,
// warning or strict.
func ParseAPIMode(s string) (APIMode, bool) {
	switch s {
	case "", "disable":
		return APIDisabled, true
	case "warning":
		return APIWarning, true
	case "strict":
		return APIStrict, true
	}
	return APIDisabled, false
}

// checkExplicitAPI reports, as kotlinc's FirExplicitApiDeclarationChecker,
// the declarations of the file's public API (effectively public or
// protected, or @PublishedApi themselves) that don't say their visibility
// in explicit API mode, or that have an inferred type in explicit API
// mode or explicit return types mode.
//
// A visibility isn't required of an override, a primary constructor, an
// accessor, an enum entry or a property of a data or an annotation class;
// a type is required of an override too, but only of a function with an
// expression body or a property. Local declarations and the members of
// local classes and object expressions aren't checked.
func (c *Checker) checkExplicitAPI() {
	if c.u.explicitAPI == APIDisabled && c.u.explicitReturnTypes == APIDisabled {
		return
	}
	for _, d := range c.file.Decls {
		c.apiDecl(d, nil)
	}
}

// apiDecl checks a declaration in class owner, or at the top level.
func (c *Checker) apiDecl(d syntax.Decl, owner *types.ClassSym) {
	var ownerDecl *syntax.ClassDecl
	if owner != nil {
		ownerDecl, _ = owner.Decl.(*syntax.ClassDecl)
	}
	// Properties of data and annotation classes need no visibility.
	propsExempt := ownerDecl != nil && (ownerDecl.Data || ownerDecl.Annotation)
	switch d := d.(type) {
	case *syntax.ClassDecl:
		if d.Synthetic || d.Generated || d.Sym == nil || d.EnumEntry || localClass(d.Sym) {
			return
		}
		if apiVis(classEffVis(d.Sym), &d.Modifiers, owner, true) {
			c.noVisibility(d.P, &d.Modifiers)
		}
		cls := d.Sym
		for _, cp := range d.CtorParams {
			if !cp.IsProp || cp.PropSym == nil || cp.Override || d.Data || d.Annotation {
				continue
			}
			if apiVis(propEffVis(cp.PropSym, cls), &cp.Modifiers, cls, false) {
				c.noVisibility(cp.P, &cp.Modifiers)
			}
		}
		for _, m := range d.Members {
			if sc, ok := m.(*syntax.SecondaryCtor); ok {
				if sc.Serial != nil || d.Enum {
					continue
				}
				if apiVis(memberEffVis(sc.Visibility, cls, false), &sc.Modifiers, cls, false) {
					c.noVisibility(sc.P, &sc.Modifiers)
				}
				continue
			}
			c.apiDecl(m, cls)
		}
	case *syntax.FunDecl:
		if d.Plugin || d.Generated || d.Sym == nil || d.AccessorOf != nil {
			return
		}
		v, _ := funVisibility(d.Sym)
		if !d.Explicit && d.Sym.Inherited == types.InheritInternal {
			v = syntax.Internal
		}
		if !apiVis(memberEffVis(v, owner, false), &d.Modifiers, owner, false) {
			return
		}
		if !d.Override {
			c.noVisibility(d.P, &d.Modifiers)
		}
		if d.Ret == nil && d.ExprBody != nil {
			c.noReturnType(d.P)
		}
	case *syntax.PropDecl:
		if d.Plugin || d.Generated || d.Sym == nil || d.DelegateOf != nil {
			return
		}
		if !apiVis(propEffVis(d.Sym, owner), &d.Modifiers, owner, false) {
			return
		}
		if !d.Override && !propsExempt {
			c.noVisibility(d.P, &d.Modifiers)
		}
		if d.Type == nil {
			c.noReturnType(d.P)
		}
	case *syntax.TypeAliasDecl:
		if d.Sym != nil && apiVis(memberEffVis(d.Visibility, owner, true), &d.Modifiers, owner, true) {
			c.noVisibility(d.P, &d.Modifiers)
		}
	}
}

// apiVis reports whether a declaration of effective visibility ev, with
// modifiers m, in class owner or at the top level, is of the public API:
// effectively public or protected, or, if it is @PublishedApi itself,
// as it would be if it were public. (The members of a @PublishedApi
// class aren't.)
func apiVis(ev effVis, m *syntax.Modifiers, owner *types.ClassSym, forClass bool) bool {
	switch ev.k {
	case evPublic, evProtected, evProtectedBound:
		return true
	}
	if m.Visibility == syntax.Internal && m.Find("", "PublishedApi") != nil {
		ev = memberEffVis(syntax.Public, owner, forClass)
		return ev.k == evPublic || ev.k == evProtected || ev.k == evProtectedBound
	}
	return false
}

// noVisibility reports a declaration at p with modifiers m that doesn't
// say its visibility, in explicit API mode.
func (c *Checker) noVisibility(p syntax.Pos, m *syntax.Modifiers) {
	if m.Explicit {
		return
	}
	switch c.u.explicitAPI {
	case APIStrict:
		c.errorf(p, "visibility must be specified in explicit API mode")
	case APIWarning:
		c.warnf(p, "NO_EXPLICIT_VISIBILITY_IN_API_MODE_WARNING", "visibility must be specified in explicit API mode")
	}
}

// noReturnType reports a declaration at p whose type is inferred, in
// explicit API mode or explicit return types mode.
func (c *Checker) noReturnType(p syntax.Pos) {
	switch max(c.u.explicitAPI, c.u.explicitReturnTypes) {
	case APIStrict:
		c.errorf(p, "return type must be specified in explicit API mode")
	case APIWarning:
		c.warnf(p, "NO_EXPLICIT_RETURN_TYPE_IN_API_MODE_WARNING", "return type must be specified in explicit API mode")
	}
}
