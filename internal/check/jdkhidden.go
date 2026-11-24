package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// jdkHiddenMembers are the members of JDK classes mapped to Kotlin
// classes that K2 hides (JvmBuiltInsSignatures' hidden signatures), by
// class and name+descriptor: an extension of the same name is called.
var jdkHiddenMembers = map[string]map[string]bool{
	"java/lang/Enum": {"describeConstable()Ljava/util/Optional;": true},
}

// hideJDK hides f, a member of the JDK class named cls, if K2 does.
func hideJDK(cls string, f *types.FunSym) {
	if jdkHiddenMembers[cls][f.Name+f.Desc] {
		f.Hidden = true
	}
}

// deprecatedOverrideOfHidden returns the message of K2's deprecation of
// f, or "". K2 hides the JDK members of the collection interfaces Kotlin
// maps that its signature lists don't name in the Kotlin types (JDK 21's
// List.getFirst() and getLast(), JDK 11's Collection.toArray(IntFunction))
// but lets super calls and overrides of them be called, deprecated
// (JvmMappedScope's HIDDEN_IN_DECLARING_CLASS_ONLY): a call of
// ArrayList's getFirst() or of a Kotlin override of it is deprecated, and
// so is its synthetic property, first. For a List before JDK 21, K2 adds
// getFirst() and getLast() for the same deprecation of what will
// override them (KT-65440), so the rule goes by name.
func (c *Checker) deprecatedOverrideOfHidden(f *types.FunSym) string {
	if f == nil || f.Owner == nil || f.Static || f.Ext {
		return ""
	}
	switch {
	case (f.Name == "getFirst" || f.Name == "getLast") && len(f.Params) == 0 && len(f.TypeParams) == 0:
		if !c.isSubclassOf(f.Owner, "java/util/List") && !c.isKotlinSubclassOf(f.Owner, "kotlin/collections/List") {
			return ""
		}
		return "This declaration will be renamed in a future version of Kotlin. Please consider using the '" +
			strings.ToLower(f.Name[3:4]) + f.Name[4:] + "()' stdlib extension if the collection supports fast random access."
	case f.Name == "toArray" && len(f.Params) == 1:
		t := f.Params[0].Type.NonNull()
		if t.K != types.Class || t.C.Name != "java/util/function/IntFunction" || !c.isSubclassOf(f.Owner, "java/util/Collection") && !c.isKotlinSubclassOf(f.Owner, "kotlin/collections/Collection") {
			return ""
		}
		return "This declaration is redundant in Kotlin and might be removed soon."
	}
	return ""
}

// jdkOverrideDepr is deprecatedOverrideOfHidden's deprecation of f.
func (c *Checker) jdkOverrideDepr(f *types.FunSym) depr {
	if msg := c.deprecatedOverrideOfHidden(f); msg != "" {
		return depr{level: "WARNING", msg: msg}
	}
	return depr{}
}

// isKotlinSubclassOf reports whether cls is a subclass of the built-in
// Kotlin class with the id name, as a class of the program that
// implements kotlin.collections.List.
func (c *Checker) isKotlinSubclassOf(cls *types.ClassSym, name string) bool {
	d := c.u.ext.kclass(name)
	return d != nil && cls.IsSubclassOf(d)
}

// hiddenJDKMember reports whether f is a member that
// deprecatedOverrideOfHidden deprecates in the mapped collection
// interface that declares it, rather than an override: K2 hides it there
// but for super calls, so that list.getFirst() of a List is unresolved.
func (c *Checker) hiddenJDKMember(f *types.FunSym) bool {
	if c.deprecatedOverrideOfHidden(f) == "" {
		return false
	}
	switch f.Owner.Name {
	case "java/util/List", "java/util/Collection":
		return true
	}
	switch f.Owner.KotlinName {
	case "kotlin/collections/List", "kotlin/collections/MutableList", "kotlin/collections/Collection", "kotlin/collections/MutableCollection":
		return true
	}
	return false
}
