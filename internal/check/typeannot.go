package check

// This file checks the annotations of types, @A T, and of type
// parameters, <@A T>: whether they may be written there, whether they
// repeat, and their arguments. kc writes neither in class files.
//
// A type is resolved where it's written, and often more than once, so
// resolveType notes its annotations, and the file's checks of bodies
// check them last, once, in the class around the type.

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

var (
	typeSite      = annotSite{siteType, types.TargetType, "type usage", false, false, false}
	typeBoundSite = annotSite{siteType, types.TargetType, "type usage", true, false, false}
	typeParamSite = annotSite{siteTypeParam, types.TargetTypeParameter, "type parameter", false, false, false}
)

// typeAnnot is an annotated type, with the class it's written in.
type typeAnnot struct {
	ref   *syntax.TypeRef
	owner *types.ClassSym
}

// noteTypeAnnotations records the annotations of type ref, the first
// time it's resolved, for typeAnnotations.
func (c *Checker) noteTypeAnnotations(ref *syntax.TypeRef) {
	if len(ref.Annotations) == 0 {
		return
	}
	if _, seen := c.u.typeAnnots.LoadOrStore(ref, true); seen {
		return
	}
	c.typeAnnots = append(c.typeAnnots, typeAnnot{ref, c.scopeClass()})
}

// typeAnnotations checks the annotations of the types noted by
// noteTypeAnnotations.
func (c *Checker) typeAnnotations() {
	saved, savedCls := c.state, c.lookupCls
	for i := 0; i < len(c.typeAnnots); i++ {
		t := c.typeAnnots[i]
		if _, done := c.u.typeAnnotsDone.LoadOrStore(t.ref, true); done {
			continue
		}
		c.state = state{}
		c.checkTypeUseAnnotations(t.ref.Annotations, t.owner, typeSite)
	}
	c.typeAnnots = nil
	c.state, c.lookupCls = saved, savedCls
}

// signatureTypeAnnotations checks the annotations of the types refs of
// a declaration's signature in class owner, and those in them, there
// rather than where they're first resolved, as from another class
// inferring the declaration's type.
func (c *Checker) signatureTypeAnnotations(owner *types.ClassSym, refs ...*syntax.TypeRef) {
	c.typeAnnotationsAt(owner, typeSite, refs...)
}

// typeAnnotationsAt is signatureTypeAnnotations for the types at site:
// typeSite, or typeBoundSite for a type parameter's bounds.
func (c *Checker) typeAnnotationsAt(owner *types.ClassSym, site annotSite, refs ...*syntax.TypeRef) {
	for _, ref := range refs {
		if ref == nil || ref.Known != nil {
			continue
		}
		if len(ref.Annotations) > 0 {
			if _, done := c.u.typeAnnotsDone.LoadOrStore(ref, true); !done {
				c.checkTypeUseAnnotations(ref.Annotations, owner, site)
			}
		}
		for _, a := range ref.Args {
			c.typeAnnotationsAt(owner, site, a.Type)
		}
		c.typeAnnotationsAt(owner, site, ref.Params...)
		c.typeAnnotationsAt(owner, site, ref.Contexts...)
		c.typeAnnotationsAt(owner, site, ref.Ret, ref.Recv, ref.Outer, ref.And)
	}
}

// paramTypeAnnotations checks those of parameters ps' types.
func (c *Checker) paramTypeAnnotations(owner *types.ClassSym, ps []*syntax.Param) {
	for _, p := range ps {
		c.signatureTypeAnnotations(owner, p.Type)
	}
}

// checkTypeUseAnnotations checks the annotations as of a type, those
// typeUseAnnotations keeps, which get the classes they resolve to, for
// the type's metadata.
func (c *Checker) checkTypeUseAnnotations(as []syntax.Annotation, owner *types.ClassSym, site annotSite) {
	kept := typeUseAnnotations(as)
	c.checkAnnotations(kept, owner, site, false)
	j := 0
	for i := range as {
		if j < len(kept) && as[i].P == kept[j].P {
			as[i].Class = kept[j].Class
			j++
		}
	}
}

// typeUseAnnotations returns as without the annotations of other
// libraries that the parser reads as flags of a type, as Compose's
// @Composable, which the class path may not have.
func typeUseAnnotations(as []syntax.Annotation) []syntax.Annotation {
	var out []syntax.Annotation
	for _, a := range as {
		if a.Name == "Composable" || strings.HasSuffix(a.Name, ".Composable") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// typeParamAnnotations checks the annotations of type parameters tps of
// a declaration in class owner.
func (c *Checker) typeParamAnnotations(tps []*syntax.TypeParam, owner *types.ClassSym) {
	for _, tp := range tps {
		c.annotations(tp.Annotations, owner, typeParamSite)
		c.typeAnnotationsAt(owner, typeBoundSite, tp.Bounds...)
	}
}

// localTypeParamAnnotations checks those of a local declaration's.
func (c *Checker) localTypeParamAnnotations(tps []*syntax.TypeParam) {
	for _, tp := range tps {
		c.localAnnotations(tp.Annotations, typeParamSite)
	}
}

// localHeader keys Checker.done for a local class whose header's
// annotations localHeaderAnnotations checked.
type localHeader struct{ *syntax.ClassDecl }

// localHeaderAnnotations checks the annotations of local class d's type
// parameters, their bounds and its supertypes in the scope around it, as
// its own: @A("$x") for a local variable x isn't a constant.
func (c *Checker) localHeaderAnnotations(d *syntax.ClassDecl) {
	c.done[localHeader{d}] = true
	c.localTypeParamAnnotations(d.TypeParams)
	for _, tp := range d.TypeParams {
		c.localTypeAnnotations(typeBoundSite, tp.Bounds...)
	}
	for _, s := range d.Supers {
		c.localTypeAnnotations(typeSite, s.Type)
	}
}

// localTypeAnnotations is typeAnnotationsAt for the types of a local
// declaration, in the current scope.
func (c *Checker) localTypeAnnotations(site annotSite, refs ...*syntax.TypeRef) {
	for _, ref := range refs {
		if ref == nil || ref.Known != nil {
			continue
		}
		if len(ref.Annotations) > 0 {
			if _, done := c.u.typeAnnotsDone.LoadOrStore(ref, true); !done {
				c.localAnnotations(typeUseAnnotations(ref.Annotations), site)
			}
		}
		for _, a := range ref.Args {
			c.localTypeAnnotations(site, a.Type)
		}
		c.localTypeAnnotations(site, ref.Params...)
		c.localTypeAnnotations(site, ref.Contexts...)
		c.localTypeAnnotations(site, ref.Ret, ref.Recv, ref.Outer, ref.And)
	}
}

// aliasRepeatRef keys Checker.done for a reference to a typealias whose
// repeated annotations are reported: types are resolved more than once.
type aliasRepeatRef struct{ *syntax.TypeRef }

// aliasRepeatedAnnotations reports the annotations of ref, a reference to
// typealias a, that its expansion already has: @A AInt for typealias
// AInt = @A Int, or Gen<@A Int> for typealias Gen<T> = List<@A T>.
func (c *Checker) aliasRepeatedAnnotations(ref *syntax.TypeRef, a *types.AliasSym) {
	d, ok := a.Decl.(*syntax.TypeAliasDecl)
	if !ok || c.done[aliasRepeatRef{ref}] {
		return
	}
	c.done[aliasRepeatRef{ref}] = true
	if len(ref.Annotations) > 0 {
		have := c.aliasAnnotations(a, map[*types.AliasSym]bool{})
		for _, an := range ref.Annotations {
			if cls := c.lookupClass(an.Name); cls != nil && have[cls] && !repeatable(cls) {
				c.errorf(an.P, "REPEATED_ANNOTATION", "this annotation is not repeatable")
			}
		}
	}
	for i, arg := range ref.Args {
		if arg.Type == nil || len(arg.Type.Annotations) == 0 || i >= len(a.TypeParams) {
			continue
		}
		have := map[*types.ClassSym]bool{}
		c.paramAnnotations(d.Type, a.TypeParams[i].Name, have)
		for _, an := range arg.Type.Annotations {
			if cls := c.lookupClass(an.Name); cls != nil && have[cls] && !repeatable(cls) {
				c.errorf(an.P, "REPEATED_ANNOTATION", "this annotation is not repeatable")
			}
		}
	}
}

// aliasAnnotations returns the classes of the annotations on the type
// typealias a expands to, through the aliases it stands for.
func (c *Checker) aliasAnnotations(a *types.AliasSym, seen map[*types.AliasSym]bool) map[*types.ClassSym]bool {
	have := map[*types.ClassSym]bool{}
	d, ok := a.Decl.(*syntax.TypeAliasDecl)
	if !ok || seen[a] || d.Type == nil {
		return have
	}
	seen[a] = true
	for _, an := range d.Type.Annotations {
		if cls := c.lookupClass(an.Name); cls != nil {
			have[cls] = true
		}
	}
	if next := c.lookupAlias(d.Type.Name); next != nil && d.Type.Ret == nil {
		for cls := range c.aliasAnnotations(next, seen) {
			have[cls] = true
		}
	}
	return have
}

// paramAnnotations adds to have the classes of the annotations on the
// uses of type parameter name in ref.
func (c *Checker) paramAnnotations(ref *syntax.TypeRef, name string, have map[*types.ClassSym]bool) {
	if ref == nil {
		return
	}
	if ref.Name == name && ref.Args == nil && ref.Ret == nil {
		for _, an := range ref.Annotations {
			if cls := c.lookupClass(an.Name); cls != nil {
				have[cls] = true
			}
		}
	}
	for _, a := range ref.Args {
		c.paramAnnotations(a.Type, name, have)
	}
	for _, p := range ref.Params {
		c.paramAnnotations(p, name, have)
	}
	c.paramAnnotations(ref.Ret, name, have)
	c.paramAnnotations(ref.Recv, name, have)
}

// repeatable reports whether annotation class cls may be repeated.
func repeatable(cls *types.ClassSym) bool {
	if cls.Repeatable {
		return true
	}
	for _, m := range cls.Annotations {
		if m.Name == "kotlin/annotation/Repeatable" || m.Name == "java/lang/annotation/Repeatable" {
			return true
		}
	}
	return false
}
