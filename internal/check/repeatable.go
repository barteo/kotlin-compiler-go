package check

// This file checks the containers of repeatable annotation classes: the
// class that java.lang.annotation.Repeatable (or kotlin.jvm.JvmRepeatable)
// names holds the repeated annotations in its value, as Java requires,
// and a class @kotlin.annotation.Repeatable makes kotlinc generate a
// nested class Container for.

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// repeatableContainer checks the container of annotation class d, whose
// annotations are checked.
func (c *Checker) repeatableContainer(d *syntax.ClassDecl) {
	cls := d.Sym
	javaRepeatable := false
	for i := range d.Annotations {
		a := &d.Annotations[i]
		if a.Target != "" || a.Class == nil || strings.ReplaceAll(a.Class.Name, "/", ".") != "java.lang.annotation.Repeatable" {
			continue
		}
		javaRepeatable = true
		if len(a.Values) != 1 || len(a.Values[0].Elems) != 1 {
			continue
		}
		lit, ok := a.Values[0].Elems[0].X.(*syntax.ClassLit)
		if !ok || lit.Class.K != types.Class {
			continue
		}
		if msg := c.containerError(cls, lit.Class.C); msg != "" {
			c.errorf(a.P, "%s", msg)
		}
	}
	if javaRepeatable {
		return
	}
	for _, m := range d.Members {
		if n, ok := m.(*syntax.ClassDecl); ok && n.Name == "Container" && kotlinRepeatable(d) {
			c.errorf(d.Find("", "Repeatable").P, "repeatable annotation cannot have a nested class named 'Container'. This name is reserved for auto-generated container class")
			return
		}
	}
}

// kotlinRepeatable reports whether d is annotated @kotlin.annotation.Repeatable.
func kotlinRepeatable(d *syntax.ClassDecl) bool {
	a := d.Find("", "Repeatable")
	return a != nil && (a.Name == "Repeatable" || a.Name == "kotlin.annotation.Repeatable")
}

// containerError returns kotlinc's error for k as the container of the
// repeatable annotation class cls, or "".
func (c *Checker) containerError(cls, k *types.ClassSym) string {
	k.Ensure()
	if !k.Annotation || !k.TargetsKnown || k == cls {
		return "" // a Java container, or the class itself
	}
	var value *types.VarSym
	for _, p := range k.AnnotationParams {
		if p.Name == "value" {
			value = p
		}
	}
	if value == nil || !value.Type.IsArray() {
		return c.containerValueError(cls, k)
	}
	if e := types.ArrayElem(value.Type); e.K != types.Class || e.C != cls {
		return c.containerValueError(cls, k)
	}
	for _, p := range k.AnnotationParams {
		if p != value && !p.Default {
			return "container annotation " + k.DisplayName() + " does not have a default value for '" + p.Name + "'"
		}
	}
	if k.Retention < cls.Retention {
		return "container annotation " + k.DisplayName() + " has a shorter retention than the repeatable annotation " + cls.DisplayName()
	}
	if !containerTargets(k.Targets, cls.Targets) {
		return "target set of container annotation " + k.DisplayName() + " must be a subset of the target set of contained annotation class " + cls.DisplayName()
	}
	return ""
}

func (c *Checker) containerValueError(cls, k *types.ClassSym) string {
	return "container annotation " + k.DisplayName() + " must have a 'value' property of type 'Array<" + cls.DisplayName() + ">'"
}

// containerTargets reports whether a container annotation class with
// targets ct may hold annotations with targets at: each of its targets
// must be one of theirs, as Java's rules have them, where a class's
// annotations apply to an annotation class too and a type's to classes
// and type parameters.
func containerTargets(ct, at types.Targets) bool {
	for t := types.Targets(1); t != 0 && t <= ct; t <<= 1 {
		if ct&t == 0 || at&t != 0 {
			continue
		}
		switch {
		case t == types.TargetAnnotationClass && at&types.TargetClass != 0:
		case (t == types.TargetAnnotationClass || t == types.TargetClass || t == types.TargetTypeParameter) && at&types.TargetType != 0:
		default:
			return false
		}
	}
	return true
}
