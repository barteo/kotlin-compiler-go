package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// platformMapped maps the Java classes that kotlinc maps to Kotlin ones,
// as its JavaToKotlinClassMap, to those: Kotlin code should use the
// Kotlin class, and kotlinc warns where it names the Java one
// (PLATFORM_CLASS_MAPPED_TO_KOTLIN).
var platformMapped = map[string]string{
	"java/lang/Object":                "kotlin.Any",
	"java/lang/String":                "kotlin.String",
	"java/lang/CharSequence":          "kotlin.CharSequence",
	"java/lang/Throwable":             "kotlin.Throwable",
	"java/lang/Cloneable":             "kotlin.Cloneable",
	"java/lang/Number":                "kotlin.Number",
	"java/lang/Comparable":            "kotlin.Comparable",
	"java/lang/Enum":                  "kotlin.Enum",
	"java/lang/annotation/Annotation": "kotlin.Annotation",
	"java/lang/Iterable":              "kotlin.collections.Iterable",
	"java/util/Iterator":              "kotlin.collections.Iterator",
	"java/util/Collection":            "kotlin.collections.Collection",
	"java/util/List":                  "kotlin.collections.List",
	"java/util/Set":                   "kotlin.collections.Set",
	"java/util/ListIterator":          "kotlin.collections.ListIterator",
	"java/util/Map":                   "kotlin.collections.Map",
	"java/util/Map$Entry":             "kotlin.collections.Map.Entry",
	"java/lang/Integer":               "kotlin.Int",
	"java/lang/Long":                  "kotlin.Long",
	"java/lang/Short":                 "kotlin.Short",
	"java/lang/Byte":                  "kotlin.Byte",
	"java/lang/Character":             "kotlin.Char",
	"java/lang/Boolean":               "kotlin.Boolean",
	"java/lang/Float":                 "kotlin.Float",
	"java/lang/Double":                "kotlin.Double",
}

// platformClassRef warns where type reference ref, resolved to t, names
// a Java class that is a Kotlin one. kc's Kotlin types are the Java
// classes, so it is the name as written that tells: the Java class's
// qualified name, a name an explicit import gives the class or one of
// its outer classes, or Integer, Character or Object, which only the
// default import of java.lang has. Typealiases to them aren't followed.
func (c *Checker) platformClassRef(ref *syntax.TypeRef, t types.Type) {
	if ref.Known != nil || c.classLitLHS || t.K != types.Class {
		return
	}
	k, ok := platformMapped[t.C.Name]
	if !ok {
		return
	}
	if c.namesPlatformClass(ref.Name, t.C) {
		c.platformClass(ref.P, k)
	}
}

// platformConstruct warns where call x, of a constructor of cls, names a
// Java class that is a Kotlin one, as platformClassRef.
func (c *Checker) platformConstruct(x *syntax.Call, cls *types.ClassSym, p syntax.Pos) {
	k, ok := platformMapped[cls.Name]
	if !ok {
		return
	}
	if name, ok := dotted(x.Fun); ok && c.namesPlatformClass(name, cls) {
		c.platformClass(p, k)
	}
}

// namesPlatformClass reports whether name, as written, names Java class
// cls rather than the Kotlin class it is in kc: see platformClassRef.
func (c *Checker) namesPlatformClass(name string, cls *types.ClassSym) bool {
	fq := strings.ReplaceAll(strings.ReplaceAll(cls.Name, "/", "."), "$", ".")
	first, rest, _ := strings.Cut(name, ".")
	switch {
	case name == fq:
		return true
	case !strings.Contains(name, ".") && (name == "Integer" || name == "Character" || name == "Object") && fq == "java.lang."+name && !c.importsName(first):
		return true
	}
	path, ok := c.explicitImport(first)
	if !ok {
		return false
	}
	if rest != "" {
		path += "." + rest
	}
	return path == fq
}

// explicitImport returns the path of the file's import, not a star
// import, of the given name.
func (c *Checker) explicitImport(name string) (string, bool) {
	for _, imp := range c.file.Imports {
		if imp.Star {
			continue
		}
		as := imp.Alias
		if as == "" {
			as = imp.Path[strings.LastIndexByte(imp.Path, '.')+1:]
		}
		if as == name {
			return imp.Path, true
		}
	}
	return "", false
}

// importsName reports whether the file explicitly imports something of
// the given name.
func (c *Checker) importsName(name string) bool {
	_, ok := c.explicitImport(name)
	return ok
}

// platformClass warns of a use of a Java class at p, that Kotlin class k
// stands for.
func (c *Checker) platformClass(p syntax.Pos, k string) {
	c.warnf(p, "PLATFORM_CLASS_MAPPED_TO_KOTLIN", "this class is not recommended for use in Kotlin. Use '%s' instead", k)
}

// platformClassImports warns of the file's imports of Java classes that
// are Kotlin ones, as kotlinc's PlatformClassMappedToKotlinImportsChecker
// does: in their order, up to the first import of a class's member or
// nested class, or of a class's members, which stops kotlinc's.
func (c *Checker) platformClassImports() {
	for _, imp := range c.file.Imports {
		if imp.Star {
			if c.importOwner(imp.Path) != nil {
				return
			}
			continue
		}
		if i := strings.LastIndexByte(imp.Path, '.'); i < 0 || c.importOwner(imp.Path[:i]) != nil {
			return
		}
		if k, ok := platformMapped[strings.ReplaceAll(imp.Path, ".", "/")]; ok {
			c.platformClass(imp.PathP, k)
		}
	}
}

// jvmFunctionInit reports a lambda, an anonymous function or a callable
// reference init that initializes a declaration of type ref written as
// one of the classes kotlin.jvm.functions.FunctionN, by its qualified
// name or an explicit import: K2 takes that for the class, not the
// function type, which a function value isn't (INITIALIZER_TYPE_MISMATCH).
// kc's function types are those classes, so only the name as written
// tells, here only of a property's or a variable's initializer.
func (c *Checker) jvmFunctionInit(ref *syntax.TypeRef, init syntax.Expr) {
	if ref == nil || ref.Known != nil {
		return
	}
	switch init.(type) {
	case *syntax.Lambda: // and a callable reference, a Lambda with Ref
	default:
		return
	}
	name := ref.Name
	first, rest, _ := strings.Cut(name, ".")
	if path, ok := c.explicitImport(first); ok {
		name = path
		if rest != "" {
			name += "." + rest
		}
	}
	n, ok := strings.CutPrefix(name, "kotlin.jvm.functions.Function")
	if !ok || n == "" || strings.Trim(n, "0123456789") != "" {
		return
	}
	c.errorf(init.Pos(), "type mismatch: a function value isn't a %s, which names the class: use a function type", ref.Name)
}
