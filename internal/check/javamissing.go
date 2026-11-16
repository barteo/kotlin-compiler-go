package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// javaMissingClass reports a call x whose only candidates of its arity
// are methods of the program's Java sources that kc can't call because
// their signatures name classes that aren't there, and reports whether it
// did. As in K2 (MISSING_DEPENDENCY_CLASS), those are the classes a Java
// source names but kotlinc can't find: one that a file of another name
// declares, or a member type it inherits from two supertypes.
func (c *Checker) javaMissingClass(x *syntax.Call, cands []callee) bool {
	missing := ""
	for _, cl := range cands {
		f := cl.f
		if f.Hidden {
			continue
		}
		if _, ok := mapArgs(f, x); !ok {
			continue
		}
		if !f.Unsupported || f.Why != "" || f.Owner == nil || !c.u.ext.javaSource(f.Owner) {
			return false
		}
		m := c.missingInDesc(f.Desc)
		if m == "" {
			return false
		}
		if missing == "" {
			missing = m
		}
	}
	if missing == "" {
		return false
	}
	c.errorf(x.P, "cannot access class '%s'. Check your module classpath for missing or conflicting dependencies", strings.ReplaceAll(missing, "/", "."))
	return true
}

// missingInDesc returns a class a method descriptor names that isn't on
// the class path, or "".
func (c *Checker) missingInDesc(desc string) string {
	if desc == "" {
		return ""
	}
	for _, d := range append(classfile.DescParams(desc), classfile.RetDesc(desc)) {
		d = strings.TrimLeft(d, "[")
		if name, ok := strings.CutPrefix(d, "L"); ok {
			if name = strings.TrimSuffix(name, ";"); c.u.ext.class(name) == nil {
				return name
			}
		}
	}
	return ""
}
