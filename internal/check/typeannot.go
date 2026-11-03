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
	typeSite      = annotSite{siteType, types.TargetType, "type usage"}
	typeParamSite = annotSite{siteTypeParam, types.TargetTypeParameter, "type parameter"}
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
		c.checkAnnotations(typeUseAnnotations(t.ref.Annotations), t.owner, typeSite, false)
	}
	c.typeAnnots = nil
	c.state, c.lookupCls = saved, savedCls
}

// signatureTypeAnnotations checks the annotations of the types refs of
// a declaration's signature in class owner, and those in them, there
// rather than where they're first resolved, as from another class
// inferring the declaration's type.
func (c *Checker) signatureTypeAnnotations(owner *types.ClassSym, refs ...*syntax.TypeRef) {
	for _, ref := range refs {
		if ref == nil || ref.Known != nil {
			continue
		}
		if len(ref.Annotations) > 0 {
			if _, done := c.u.typeAnnotsDone.LoadOrStore(ref, true); !done {
				c.checkAnnotations(typeUseAnnotations(ref.Annotations), owner, typeSite, false)
			}
		}
		for _, a := range ref.Args {
			c.signatureTypeAnnotations(owner, a.Type)
		}
		c.signatureTypeAnnotations(owner, ref.Params...)
		c.signatureTypeAnnotations(owner, ref.Contexts...)
		c.signatureTypeAnnotations(owner, ref.Ret, ref.Recv, ref.Outer, ref.And)
	}
}

// paramTypeAnnotations checks those of parameters ps' types.
func (c *Checker) paramTypeAnnotations(owner *types.ClassSym, ps []*syntax.Param) {
	for _, p := range ps {
		c.signatureTypeAnnotations(owner, p.Type)
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
		c.signatureTypeAnnotations(owner, tp.Bounds...)
	}
}

// localTypeParamAnnotations checks those of a local declaration's.
func (c *Checker) localTypeParamAnnotations(tps []*syntax.TypeParam) {
	for _, tp := range tps {
		c.localAnnotations(tp.Annotations, typeParamSite)
	}
}
