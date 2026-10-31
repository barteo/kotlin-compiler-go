package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// kc compiles multiplatform code for the JVM only: an actual declaration
// is an ordinary one, and the parser leaves expect declarations out of the
// program (syntax.File.Expects). They are only checked against the actual
// ones, by name and kind, here.

// expectKey identifies an expect or actual declaration.
type expectKey struct {
	pkg, kind, name string
}

// declKey returns the key of a top-level declaration.
func declKey(pkg string, d syntax.Decl) (expectKey, syntax.Pos, bool) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		return expectKey{pkg, "function", d.Name}, d.P, true
	case *syntax.PropDecl:
		return expectKey{pkg, "property", d.Name}, d.P, true
	case *syntax.ClassDecl:
		return expectKey{pkg, "class", d.Name}, d.P, true
	case *syntax.TypeAliasDecl:
		// actual typealias A = B is the actual of expect class A.
		return expectKey{pkg, "class", d.Name}, d.P, true
	}
	return expectKey{}, syntax.Pos{}, false
}

// expectActual checks the program's expect declarations: they have no
// bodies, each has an actual declaration, and, if there are any, each
// actual declaration has one.
func expectActual(checkers []*Checker) {
	expects := map[expectKey]bool{}
	actuals := map[expectKey]bool{}
	for _, c := range checkers {
		for _, d := range c.file.Expects {
			c.checkExpect(d)
			if k, _, ok := declKey(c.file.Package, d); ok {
				expects[k] = true
			}
		}
		for _, d := range c.file.Decls {
			if k, _, ok := declKey(c.file.Package, d); ok && actual(d) {
				actuals[k] = true
			}
		}
	}
	if len(expects) == 0 {
		// The expect declarations aren't part of this compilation.
		return
	}
	for _, c := range checkers {
		for _, d := range c.file.Expects {
			if k, p, ok := declKey(c.file.Package, d); ok && !actuals[k] {
				c.errorf(p, "the 'expect' declaration '%s' has no 'actual' declaration", k.name)
			}
		}
		for _, d := range c.file.Decls {
			if k, p, ok := declKey(c.file.Package, d); ok && actual(d) && !expects[k] {
				c.errorf(p, "'%s' has no corresponding 'expect' declaration", k.name)
			}
		}
	}
}

// actual reports whether d is marked actual.
func actual(d syntax.Decl) bool {
	switch d := d.(type) {
	case *syntax.FunDecl:
		return d.Actual
	case *syntax.PropDecl:
		return d.Actual
	case *syntax.ClassDecl:
		return d.Actual
	case *syntax.TypeAliasDecl:
		return d.Actual
	}
	return false
}

// checkExpect reports an expect declaration with a body or an
// initializer, also among an expect class's members.
func (c *Checker) checkExpect(d syntax.Decl) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		if d.Body != nil || d.ExprBody != nil {
			c.errorf(d.P, "'expect' declaration cannot have a body")
		}
	case *syntax.PropDecl:
		switch {
		case d.Init != nil:
			c.errorf(d.P, "'expect' property cannot have an initializer")
		case d.Getter != nil:
			c.errorf(d.P, "'expect' declaration cannot have a body")
		}
	case *syntax.ClassDecl:
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.InitBlock:
				c.errorf(m.P, "'expect' declaration cannot have a body")
			default:
				c.checkExpect(m)
			}
		}
	}
}

// expectMembers reports the expect modifier on the members of a class
// that isn't expect.
func (c *Checker) expectMembers(d *syntax.ClassDecl) {
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			if m.Expect {
				c.errorf(m.ExpectP, "modifier 'expect' is not applicable to 'member function'")
			}
		case *syntax.PropDecl:
			if m.Expect {
				c.errorf(m.ExpectP, "modifier 'expect' is not applicable to 'member property'")
			}
		case *syntax.ClassDecl:
			if m.Expect {
				c.errorf(m.ExpectP, "modifier 'expect' is not applicable to 'nested class'")
			}
		}
	}
}

// inheritDefaults gives the parameters of actual functions and
// constructors the default values of their expect declarations', which
// Kotlin's actual declarations inherit and can't repeat: okio's actual
// ByteString.indexOf(other, fromIndex) has fromIndex = 0 of its expect
// declaration. The values are checked where the actual declarations are.
func inheritDefaults(files []*syntax.File) {
	expects := map[string][]*syntax.Param{}
	var collect func(prefix string, ds []syntax.Decl)
	collect = func(prefix string, ds []syntax.Decl) {
		for _, d := range ds {
			switch d := d.(type) {
			case *syntax.FunDecl:
				expects[expectFunKey(prefix, d.Name, d.Recv, d.Params)] = d.Params
			case *syntax.ClassDecl:
				p := prefix + d.Name + "."
				if d.CtorParams != nil {
					expects[expectFunKey(p, "<init>", nil, ctorParams(d.CtorParams))] = ctorParams(d.CtorParams)
				}
				for _, m := range d.Members {
					if s, ok := m.(*syntax.SecondaryCtor); ok {
						expects[expectFunKey(p, "<init>", nil, s.Params)] = s.Params
					}
				}
				collect(p, d.Members)
			}
		}
	}
	for _, f := range files {
		collect(f.Package+":", f.Expects)
	}
	if len(expects) == 0 {
		return
	}
	inherit := func(key string, ps []*syntax.Param) {
		e := expects[key]
		for i, p := range ps {
			if i < len(e) && p.Default == nil && e[i].Default != nil {
				p.Default = e[i].Default
			}
		}
	}
	var visit func(prefix string, ds []syntax.Decl, actualCls bool)
	visit = func(prefix string, ds []syntax.Decl, actualCls bool) {
		for _, d := range ds {
			switch d := d.(type) {
			case *syntax.FunDecl:
				if d.Actual || actualCls {
					inherit(expectFunKey(prefix, d.Name, d.Recv, d.Params), d.Params)
				}
			case *syntax.SecondaryCtor:
				if d.Actual || actualCls {
					inherit(expectFunKey(prefix, "<init>", nil, d.Params), d.Params)
				}
			case *syntax.ClassDecl:
				p := prefix + d.Name + "."
				act := d.Actual || actualCls
				if act && d.CtorParams != nil {
					ps := ctorParams(d.CtorParams)
					inherit(expectFunKey(p, "<init>", nil, ps), ps)
				}
				visit(p, d.Members, act)
			}
		}
	}
	for _, f := range files {
		visit(f.Package+":", f.Decls, false)
	}
}

func ctorParams(cps []*syntax.CtorParam) []*syntax.Param {
	ps := make([]*syntax.Param, len(cps))
	for i, cp := range cps {
		ps[i] = &cp.Param
	}
	return ps
}

// expectFunKey identifies a function or constructor among expect and actual
// declarations by its name and the types of its receiver and parameters,
// as written.
func expectFunKey(prefix, name string, recv *syntax.TypeRef, ps []*syntax.Param) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(name)
	b.WriteByte('(')
	writeTypeRef(&b, recv)
	for _, p := range ps {
		b.WriteByte(',')
		if p.Vararg {
			b.WriteString("...")
		}
		writeTypeRef(&b, p.Type)
	}
	b.WriteByte(')')
	return b.String()
}

func writeTypeRef(b *strings.Builder, t *syntax.TypeRef) {
	if t == nil {
		return
	}
	if t.Recv != nil {
		writeTypeRef(b, t.Recv)
		b.WriteByte('.')
	}
	if t.Ret != nil {
		b.WriteByte('(')
		for _, p := range t.Params {
			writeTypeRef(b, p)
			b.WriteByte(',')
		}
		b.WriteString(")->")
		writeTypeRef(b, t.Ret)
	}
	// The last part of a qualified name, which the files may import.
	b.WriteString(t.Name[strings.LastIndexByte(t.Name, '.')+1:])
	if t.Args != nil {
		b.WriteByte('<')
		for _, a := range t.Args {
			if a.Type == nil {
				b.WriteByte('*')
			}
			writeTypeRef(b, a.Type)
			b.WriteByte(',')
		}
		b.WriteByte('>')
	}
	if t.Nullable {
		b.WriteByte('?')
	}
}
