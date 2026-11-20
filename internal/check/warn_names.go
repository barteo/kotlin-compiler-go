package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// This file reports kotlinc's DANGEROUS_CHARACTERS (FirJvmNamesChecker):
// a declaration's name, in backticks, with a character that file names
// can't have on Windows, or that expands an environment variable there,
// ?*"|%. A name with one the JVM doesn't allow is an error instead.

// dangerousChars are the characters DANGEROUS_CHARACTERS reports.
const dangerousChars = `?*"|%`

// dangerousName reports name, of a declaration at pos, if it has one of
// dangerousChars.
func (c *Checker) dangerousName(name string, pos syntax.Pos) {
	if !strings.ContainsAny(name, dangerousChars) || syntax.InvalidNameChars(name) {
		return
	}
	var found []byte
	for i := range len(dangerousChars) {
		if strings.IndexByte(name, dangerousChars[i]) >= 0 {
			found = append(found, dangerousChars[i])
		}
	}
	c.warnf(pos, "DANGEROUS_CHARACTERS", "name contains characters which can cause problems on Windows: %s", found)
}

// dangerousNames reports the names of the file's package and
// declarations, local ones too, with dangerous characters.
func (c *Checker) dangerousNames() {
	if strings.ContainsAny(c.file.Package, dangerousChars) {
		for _, seg := range strings.Split(c.file.Package, ".") {
			c.dangerousName(seg, c.file.PackageP)
		}
	}
	typeParams := func(tps []*syntax.TypeParam) {
		for _, tp := range tps {
			c.dangerousName(tp.Name, tp.P)
		}
	}
	params := func(ps []*syntax.Param) {
		for _, p := range ps {
			c.dangerousName(p.Name, p.P)
		}
	}
	var decl func(d syntax.Decl)
	fun := func(d *syntax.FunDecl) {
		if d == nil || d.Generated {
			return
		}
		c.dangerousName(d.Name, d.P)
		typeParams(d.TypeParams)
		params(d.Params)
	}
	class := func(d *syntax.ClassDecl) {
		if d == nil || d.Generated || d.Synthetic {
			return
		}
		if !d.Anonymous && !d.EnumEntry {
			c.dangerousName(d.Name, d.P)
		}
		typeParams(d.TypeParams)
		for _, cp := range d.CtorParams {
			c.dangerousName(cp.Name, cp.P)
		}
		for _, e := range d.Entries {
			c.dangerousName(e.Name, e.P)
		}
		for _, m := range d.Members {
			decl(m)
		}
	}
	decl = func(d syntax.Decl) {
		switch d := d.(type) {
		case *syntax.FunDecl:
			fun(d)
		case *syntax.PropDecl:
			if d.Generated || d.Plugin || d.DelegateOf != nil {
				return
			}
			c.dangerousName(d.Name, d.P)
			typeParams(d.TypeParams)
			fun(d.Setter)
		case *syntax.TypeAliasDecl:
			c.dangerousName(d.Name, d.P)
			typeParams(d.TypeParams)
		case *syntax.SecondaryCtor:
			params(d.Params)
		case *syntax.ClassDecl:
			class(d)
		}
	}
	for _, d := range c.file.Decls {
		decl(d)
	}
	walkCode(c.file.Decls, func(n any) bool {
		switch n := n.(type) {
		case *syntax.LocalFun:
			fun(n.Decl)
		case *syntax.LocalClass:
			class(n.Decl)
		case *syntax.ObjectExpr:
			class(n.Decl)
		case *syntax.VarStmt:
			c.dangerousName(n.Name, n.P)
		case *syntax.Destructure:
			for _, s := range n.Stmts {
				c.dangerousName(s.Name, s.P)
			}
		case *syntax.For:
			if n.Names == nil {
				c.dangerousName(n.Var, n.P)
			}
		case *syntax.Try:
			for _, k := range n.Catches {
				c.dangerousName(k.Name, k.P)
			}
		case *syntax.Lambda:
			if n.Ref == nil && n.KProp == nil && n.KFun == nil {
				for _, p := range n.Params {
					c.dangerousName(p.Name, p.P)
				}
			}
		}
		return true
	})
}
