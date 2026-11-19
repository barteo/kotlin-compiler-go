package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// Classifiers of star imports.
//
// As in K2, the classifiers that a file's own star imports give a name
// are of one level, after the explicit imports and the file's package,
// and two distinct ones are ambiguous: import a.*, import b.* that both
// have a class X make the type X an error. Where the name is a qualifier
// of a value, Some.foo, the one of them that is a value, an object or a
// class with a companion, is meant, and only two such are ambiguous.

// starClassifiers returns the distinct classifiers that the file's own
// star imports give a simple name, if they are the nearest that do:
// nothing local, nested, explicitly imported or of the file's package
// has the name.
func (c *Checker) starClassifiers(name string) []classifier {
	if kotlinTypes[name] || builtinClasses[name] || strings.Contains(name, ".") {
		return nil
	}
	if c.scope != nil && c.scope.findClass(name) != nil {
		return nil
	}
	if _, ok := c.imports[name]; ok {
		return nil
	}
	if _, ok := c.aliasImports[name]; ok {
		return nil
	}
	if c.nestedClassifier(name) != (classifier{}) {
		return nil
	}
	ks, ok := c.starCache[name]
	if !ok {
		ks = c.starClassifiers1(name)
		c.starCache[name] = ks
	}
	return ks
}

func (c *Checker) starClassifiers1(name string) []classifier {
	if cls, ok := c.pkg.Classes[name]; ok && !c.privateElsewhere(classifier{cls: cls}) {
		return nil
	}
	if a, ok := c.pkg.Aliases[name]; ok && !c.privateElsewhere(classifier{alias: a}) {
		return nil
	}
	ext := c.u.ext
	own := strings.ReplaceAll(c.file.Package, ".", "/")
	if ext.hasPackage(own) {
		if ext.kotlin && ext.pkgClass(c.file.Package, name) != nil || ext.class(qualified(own, name)) != nil {
			return nil
		}
	}
	var out []classifier
	var targets []any // what each of out is, a typealias's class
	add := func(k classifier) {
		if !c.starVisible(k) {
			return
		}
		var t any = k.cls
		if k.alias != nil {
			t = k.alias
			if cls := c.aliasClass(k.alias); cls != nil {
				t = cls
			}
		}
		if !slices.Contains(targets, t) {
			out, targets = append(out, k), append(targets, t)
		}
	}
	for _, p := range c.userStars {
		if cls, ok := p.Classes[name]; ok {
			add(classifier{cls: cls})
		}
		if a, ok := p.Aliases[name]; ok {
			add(classifier{alias: a})
		}
	}
	for _, owner := range c.starOwners {
		if cls := c.nestedOf(owner, name); cls != nil {
			add(classifier{cls: cls})
		}
	}
	for i, r := range c.starPkgs() {
		if c.stars[i] == own {
			continue // the file's package, of the level above
		}
		if ext.kotlin {
			if cls := ext.pkgClassIn(r, name); cls != nil {
				add(classifier{cls: cls})
			} else if a := ext.pkgAliasIn(r, name); a != nil {
				add(classifier{alias: a})
			}
		} else if cls := ext.classIn(r.path, r.hash, name); cls != nil {
			add(classifier{cls: cls})
		}
	}
	return out
}

// starAmbiguity reports a simple name of a type that two of the file's
// star imports give, and true if it did.
func (c *Checker) starAmbiguity(name string, p syntax.Pos) bool {
	if c.starFallback {
		return false
	}
	if ks := c.starClassifiers(name); len(ks) > 1 {
		c.errorf(p, "overload resolution ambiguity: %s is a classifier of more than one star import", name)
		return true
	}
	return false
}

// starQualifierAmbiguity reports a simple name that qualifies a member,
// Some.foo, and that star imports give two classifiers of: kotlinc
// takes the one that is a value, and reports two, or none if no default
// import has the name either. It reports true if it reported.
func (c *Checker) starQualifierAmbiguity(id *syntax.Ident) bool {
	if c.isVar(id.Name) {
		return false
	}
	ks := c.starClassifiers(id.Name)
	if len(ks) < 2 {
		return false
	}
	values := 0
	for _, k := range ks {
		cls := k.cls
		if k.alias != nil {
			cls = c.aliasClass(k.alias)
		}
		if cls != nil && objectOf(cls) != nil {
			values++
		}
	}
	switch {
	case values > 1:
		c.errorf(id.P, "overload resolution ambiguity: %s is a classifier of more than one star import", id.Name)
		return true
	case values == 0 && !c.defaultClassifier(id.Name):
		c.errorf(id.P, "none of the classifiers named %s that star imports give is a value", id.Name)
		return true
	}
	return false
}

// defaultClassifier reports whether a default import gives a classifier
// of name.
func (c *Checker) defaultClassifier(name string) bool {
	ext := c.u.ext
	if ext.kotlin {
		for _, r := range ext.defaultRefs() {
			if ext.pkgClassIn(r, name) != nil || ext.pkgAliasIn(r, name) != nil {
				return true
			}
		}
	}
	return ext.classIn("java/lang", javaLangHash, name) != nil
}

// starVisible reports whether a star import gives classifier k: not one
// the file can't see, such as a package-private Java class of another
// package or a private nested class.
func (c *Checker) starVisible(k classifier) bool {
	if k.alias != nil || c.privateElsewhere(k) {
		return !c.privateElsewhere(k)
	}
	cls := k.cls
	if !cls.External {
		return c.classVisible(cls)
	}
	cls.Ensure()
	return !cls.Missing && (!cls.Private || cls.PackagePrivate && c.samePackage(cls) || cls.Internal && c.file.SuppressInvisible)
}
