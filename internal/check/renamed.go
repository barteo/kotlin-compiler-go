package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// renamedAway reports whether the declarations named name in package pkg,
// dotted or with slashes, are imported only under another name, which
// hides their simple name below the explicit imports: import a.A as ER.
func (c *Checker) renamedAway(pkg, name string) bool {
	if len(c.renamed) == 0 {
		return false
	}
	if pkg == "" {
		return c.renamed[name]
	}
	return c.renamed[strings.ReplaceAll(pkg, "/", ".")+"."+name]
}

// funsOf returns package p's functions named name, unless an import
// renames them away.
func (c *Checker) funsOf(p *Package, name string) []*types.FunSym {
	if c.renamedAway(p.Name, name) {
		return nil
	}
	return p.Funs[name]
}
