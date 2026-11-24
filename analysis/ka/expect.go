package ka

import (
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// expectKey identifies a top-level expect or actual declaration: an actual
// typealias is the actual of an expect class, and a function or property
// is one of its name's overloads by sig.
type expectKey struct {
	pkg, name string
	kind      byte
	sig       string
}

// declKey returns the key of top-level declaration d, its position and
// whether it is an actual declaration.
func declKey(pkg string, d syntax.Decl) (k expectKey, p syntax.Pos, actual bool) {
	switch d := d.(type) {
	case *syntax.FunDecl:
		return expectKey{pkg, d.Name, 'f', signature(d.TypeParams, d.Recv, d.Params)}, d.P, d.Actual
	case *syntax.PropDecl:
		return expectKey{pkg, d.Name, 'p', signature(d.TypeParams, d.Recv, nil)}, d.P, d.Actual
	case *syntax.ClassDecl:
		return expectKey{pkg, d.Name, 'c', ""}, d.P, d.Actual
	case *syntax.TypeAliasDecl:
		return expectKey{pkg, d.Name, 'c', ""}, d.P, d.Actual
	}
	return expectKey{}, syntax.Pos{}, false
}

// signature returns what tells the overloads of a name apart, as the
// compiler matches an expect declaration with its actual one: the number
// of type parameters and the types of the receiver and the parameters, as
// written. A type parameter is its index, and a qualified name its last
// part, which the other file may import.
func signature(tps []*syntax.TypeParam, recv *syntax.TypeRef, ps []*syntax.Param) string {
	var b strings.Builder
	var write func(t *syntax.TypeRef)
	write = func(t *syntax.TypeRef) {
		if t == nil {
			return
		}
		if t.Recv != nil {
			write(t.Recv)
			b.WriteByte('.')
		}
		if t.Ret != nil {
			b.WriteByte('(')
			for _, p := range t.Params {
				write(p)
				b.WriteByte(',')
			}
			b.WriteString(")->")
			write(t.Ret)
		}
		name := t.Name[strings.LastIndexByte(t.Name, '.')+1:]
		if i := slices.IndexFunc(tps, func(tp *syntax.TypeParam) bool { return tp.Name == t.Name }); i >= 0 {
			name = "#" + strconv.Itoa(i)
		}
		b.WriteString(name)
		if t.Args != nil {
			b.WriteByte('<')
			for _, a := range t.Args {
				if a.Type == nil {
					b.WriteByte('*')
				}
				write(a.Type)
				b.WriteByte(',')
			}
			b.WriteByte('>')
		}
		if t.Nullable {
			b.WriteByte('?')
		}
	}
	b.WriteString(strconv.Itoa(len(tps)))
	b.WriteByte('(')
	write(recv)
	for _, p := range ps {
		b.WriteByte(',')
		if p.Vararg {
			b.WriteString("...")
		}
		write(p.Type)
	}
	b.WriteByte(')')
	return b.String()
}

// expects is what keepExpects found in the files of one lowering.
type expects struct {
	// paired is set if an expect declaration has an actual one in the
	// sources: the other view of the files has declarations that this
	// one leaves out.
	paired bool
	// other are the declarations that the second view has in place of
	// the first's.
	other map[syntax.Decl]bool
	// defaults are the default values of the parameters of the expect
	// declarations left out. The checker gives them to the actual
	// declarations' parameters, which don't declare them.
	defaults map[syntax.Expr]bool
}

// keepExpects makes expect declarations among files declarations of the
// program, in the place they are written. The lowering leaves every expect
// declaration out (syntax.File.Expects), as the actual one stands for it.
//
// One without an actual declaration in the sources is the only
// declaration of its name: a symbol processor may be what generates the
// actual one, as Room does for the object a database's @ConstructedBy
// names.
//
// One with an actual declaration is a declaration next to it, which the
// checker can't have under one name. So the files have two views. The
// first has the actual declaration, and the second, lowered and checked
// apart for the model of the declarations it has instead, the expect one.
// An expect class whose actual declaration is a typealias is in both, as
// the class is what the name resolves to, and the typealias in the second
// only.
func keepExpects(files []*syntax.File, second bool) *expects {
	ex := &expects{other: map[syntax.Decl]bool{}, defaults: map[syntax.Expr]bool{}}
	some := false
	for _, f := range files {
		some = some || f != nil && len(f.Expects) > 0
	}
	if !some {
		return ex
	}
	actuals := map[expectKey]syntax.Decl{}
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, d := range f.Decls {
			k, _, actual := declKey(f.Package, d)
			if _, dup := actuals[k]; actual && !dup {
				actuals[k] = d
			}
		}
	}
	drop := map[syntax.Decl]bool{}
	for _, f := range files {
		if f == nil {
			continue
		}
		var rest []syntax.Decl
		for _, d := range f.Expects {
			k, p, _ := declKey(f.Package, d)
			if a, ok := actuals[k]; ok {
				ex.paired = true
				_, alias := a.(*syntax.TypeAliasDecl)
				switch {
				case alias && second:
					ex.other[a] = true
				case alias:
					drop[a] = true
				case second:
					drop[a] = true
					ex.other[d] = true
				default:
					rest = append(rest, d)
					continue
				}
			}
			i := slices.IndexFunc(f.Decls, func(o syntax.Decl) bool {
				_, q, _ := declKey("", o)
				return q.Line > p.Line || q.Line == p.Line && q.Col > p.Col
			})
			if i < 0 {
				i = len(f.Decls)
			}
			f.Decls = slices.Insert(f.Decls, i, d)
			switch d := d.(type) {
			case *syntax.ClassDecl:
				expectMembers(d)
			case *syntax.PropDecl:
				expectAccessors(d)
			}
		}
		f.Expects = rest
		for _, d := range rest {
			ex.addDefaults(d)
		}
	}
	if len(drop) > 0 {
		for _, f := range files {
			if f == nil {
				continue
			}
			f.Decls = slices.DeleteFunc(f.Decls, func(d syntax.Decl) bool {
				if p, ok := d.(*syntax.PropDecl); ok && p.DelegateOf != nil && drop[p.DelegateOf] {
					return true
				}
				return drop[d]
			})
		}
	}
	return ex
}

// addDefaults adds the default values of the parameters of expect
// declaration d, of its constructors and of its members to ex.defaults.
func (ex *expects) addDefaults(d syntax.Decl) {
	add := func(ps []*syntax.Param) {
		for _, p := range ps {
			if p.Default != nil {
				ex.defaults[p.Default] = true
			}
		}
	}
	switch d := d.(type) {
	case *syntax.FunDecl:
		add(d.Params)
	case *syntax.SecondaryCtor:
		add(d.Params)
	case *syntax.ClassDecl:
		for _, p := range d.CtorParams {
			if p.Default != nil {
				ex.defaults[p.Default] = true
			}
		}
		for _, m := range d.Members {
			ex.addDefaults(m)
		}
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
			expectAccessors(m)
		case *syntax.SecondaryCtor:
			m.Expect = true
		case *syntax.ClassDecl:
			m.Expect = true
			expectMembers(m)
		}
	}
}

// expectAccessors gives expect extension property d the accessors it
// declares without bodies: an extension property has no field, so the
// checker resolves the types of one with a getter only.
func expectAccessors(d *syntax.PropDecl) {
	if d.Recv == nil || d.Getter != nil {
		return
	}
	d.Getter = &syntax.FunDecl{P: d.P, Name: "get", Property: d.Name, Ret: d.Type}
	if d.Mutable && d.Setter == nil {
		d.Setter = &syntax.FunDecl{P: d.P, Name: "set", Property: d.Name, Params: []*syntax.Param{{P: d.P, Name: "value"}}}
	}
}
