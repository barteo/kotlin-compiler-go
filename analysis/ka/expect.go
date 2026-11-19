package ka

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// expectKey identifies a top-level expect or actual declaration: an actual
// typealias is the actual of an expect class.
type expectKey struct {
	pkg, name string
	kind      byte
}

// declKey returns the key of top-level declaration d, its position and
// whether it is an actual declaration.
func declKey(pkg string, d syntax.Decl) (k expectKey, p syntax.Pos, actual bool) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		return expectKey{pkg, d.Name, 'f'}, d.P, d.Actual
	case *syntax.PropDecl:
		return expectKey{pkg, d.Name, 'p'}, d.P, d.Actual
	case *syntax.ClassDecl:
		return expectKey{pkg, d.Name, 'c'}, d.P, d.Actual
	case *syntax.TypeAliasDecl:
		return expectKey{pkg, d.Name, 'c'}, d.P, d.Actual
	}
	return expectKey{}, syntax.Pos{}, false
}

// keepExpects makes the expect declarations that have no actual one among
// files declarations of the program, in the place they are written. The
// lowering leaves every expect declaration out (syntax.File.Expects), as
// the actual one stands for it. One without an actual declaration in the
// sources is the only declaration of its name: a symbol processor may be
// what generates the actual one, as Room does for the object a database's
// @ConstructedBy names.
func keepExpects(files []*syntax.File) {
	some := false
	for _, f := range files {
		some = some || f != nil && len(f.Expects) > 0
	}
	if !some {
		return
	}
	actuals := map[expectKey]bool{}
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, d := range f.Decls {
			if k, _, actual := declKey(f.Package, d); actual {
				actuals[k] = true
			}
		}
	}
	after := func(d syntax.Decl, p syntax.Pos) bool {
		_, q, _ := declKey("", d)
		return q.Line > p.Line || q.Line == p.Line && q.Col > p.Col
	}
	for _, f := range files {
		if f == nil {
			continue
		}
		var rest []syntax.Decl
		for _, d := range f.Expects {
			k, p, _ := declKey(f.Package, d)
			if actuals[k] {
				rest = append(rest, d)
				continue
			}
			i := slices.IndexFunc(f.Decls, func(o syntax.Decl) bool { return after(o, p) })
			if i < 0 {
				i = len(f.Decls)
			}
			f.Decls = slices.Insert(f.Decls, i, d)
			if c, ok := d.(*syntax.ClassDecl); ok {
				expectMembers(c)
			}
		}
		f.Expects = rest
	}
}

// expectMembers marks the members of expect class d, its constructors and
// the members of its nested classes expect, as they are without the
// modifier.
func expectMembers(d *syntax.ClassDecl) {
	d.CtorMods.Expect = true
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			m.Expect = true
		case *syntax.PropDecl:
			m.Expect = true
		case *syntax.SecondaryCtor:
			m.Expect = true
		case *syntax.ClassDecl:
			m.Expect = true
			expectMembers(m)
		}
	}
}
