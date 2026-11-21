package check

import (
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Classes that the class path's Kotlin declarations name but the class
// path doesn't have, as a library's classes that name those of a module
// it depends on that isn't on the class path: lib's Base.foo(s: Some),
// with Some in rootLib.
//
// As in K2, such a class is still a type, of a class kc knows nothing
// about but its name, rather than making the declaration unusable: a
// class can implement Base by delegation, and code can call foo. Code
// that uses a declaration whose signature names one, or a class whose
// supertypes include one, is an error, MISSING_DEPENDENCY_CLASS or
// MISSING_DEPENDENCY_SUPERCLASS (see missingDependencies).

// missingKey is the key of a missing class in the class table's map,
// which class never looks up: the class path doesn't have its name.
const missingKey = "\x00missing:"

// missingClass returns the class that stands for a class the class
// path doesn't have, by its Kotlin class id, with n type parameters, or
// nil if the class path has the class or there is none. The class is
// Missing, and has no supertypes or members.
func (t *classTable) missingClass(id string, n int) *types.ClassSym {
	return t.missing(id, internalName(id), n)
}

// javaMissingClass returns the class that stands for a class a Java
// class file's descriptor names, by its internal name, that the class
// path doesn't have, or nil. As in K2, which can't tell a nested class
// from a top-level one by a name with a $ whose outer class it lacks,
// that is a top-level class of the name: a/b/C$Some isn't the class
// a/b/C.Some that Kotlin metadata names, so a Kotlin override that
// takes the nested class doesn't implement a Java method that takes it.
func (t *classTable) javaMissingClass(name string) *types.ClassSym {
	return t.missing(name, name, 0)
}

// missing returns the stand-in for the class of Kotlin class id id and
// internal name name, with n type parameters; stand-ins of different
// ids are different classes, even of one internal name.
func (t *classTable) missing(id, name string, n int) *types.ClassSym {
	t = t.root()
	// Object, Any to Kotlin, is a builtin: never a missing dependency,
	// even on a class path without the JDK's classes.
	if t.cp == nil || name == "" || name == "java/lang/Object" || strings.HasPrefix(name, "kotlin/") || t.cp.Has(name) {
		return nil
	}
	if src := t.sources.Load(); src != nil && (*src)(name) != nil {
		return nil
	}
	key := missingKey + id + "/" + strconv.Itoa(n)
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.m[key]; ok {
		return c
	}
	c := types.NewExternal(name, func(c *types.ClassSym) { c.Missing = true })
	c.Ensure()
	c.Decl = missingDep{}
	for i := range n {
		c.TypeParams = append(c.TypeParams, &types.TypeParamSym{Name: "T" + strconv.Itoa(i), Index: i, Owner: c, Library: true})
	}
	t.m[key] = c
	return c
}

// missingName is the name of a missing class as Kotlin writes it:
// a.b.Outer.Inner.
func missingName(cls *types.ClassSym) string {
	return strings.NewReplacer("/", ".", "$", ".").Replace(cls.Name)
}

// missingDep is the Decl of a missing class.
type missingDep struct{}

// isMissingDep reports whether cls stands for a class the class path
// doesn't have.
func isMissingDep(cls *types.ClassSym) bool {
	if cls == nil {
		return false
	}
	_, ok := cls.Decl.(missingDep)
	return ok
}

// missingSet collects missing classes, each once, in the order found.
type missingSet []*types.ClassSym

func (s *missingSet) add(cls *types.ClassSym) {
	for _, c := range *s {
		if c == cls {
			return
		}
	}
	*s = append(*s, cls)
}

// top adds the missing classes t is of: its class, or those of an
// intersection, but not those of its type arguments, as K2's
// considerType. Type parameters and inference variables add none.
func (s *missingSet) top(t types.Type) {
	switch {
	case t.K == types.Class && isMissingDep(t.C):
		s.add(t.C)
	case t.IsIntersection():
		for _, b := range t.Param().Bounds {
			s.top(b)
		}
	}
}

// deep adds the missing classes t names anywhere: its own and its type
// arguments'.
func (s *missingSet) deep(t types.Type) {
	s.top(t)
	if t.K == types.Class && t.A != nil {
		for _, a := range t.A.List {
			if a.V != types.Star {
				s.deep(a.T)
			}
		}
	}
}

// args adds the missing classes of t's type arguments: to arrays, if t
// is an array type, and to other otherwise, as K2 does for receivers
// and parameters.
func args(t types.Type, arrays, other *missingSet) {
	if t.K != types.Class || t.A == nil {
		return
	}
	to := other
	if t.C != nil && t.C.Array {
		to = arrays
	}
	for _, a := range t.A.List {
		if a.V != types.Star {
			to.deep(a.T)
		}
	}
}

// missingSupers returns the missing classes among the supertypes of cls,
// direct or not.
func missingSupers(cls *types.ClassSym) missingSet {
	var out missingSet
	seen := map[*types.ClassSym]bool{}
	var walk func(c *types.ClassSym)
	walk = func(c *types.ClassSym) {
		if c == nil || seen[c] {
			return
		}
		seen[c] = true
		if isMissingDep(c) {
			out.add(c)
			return
		}
		c.Ensure()
		walk(c.Super)
		for _, i := range c.Interfaces {
			walk(i)
		}
		for _, st := range c.Supers {
			if st.K == types.Class {
				walk(st.C)
			}
		}
	}
	cls.Ensure()
	walk(cls.Super)
	for _, i := range cls.Interfaces {
		walk(i)
	}
	for _, st := range cls.Supers {
		if st.K == types.Class {
			walk(st.C)
		}
	}
	return out
}

// classOf returns the class of a type: its own, or nil.
func classOf(t types.Type) *types.ClassSym {
	if t.K == types.Class {
		return t.C
	}
	return nil
}

// classPathChecks runs the checks of what the program uses of the class
// path that depend on which modules are on it: missing classes.
func (c *Checker) classPathChecks() {
	c.missingDependencies()
}

// missingDependencies reports the uses of declarations whose signatures
// name classes the class path doesn't have, MISSING_DEPENDENCY_CLASS,
// and of classes whose supertypes include one,
// MISSING_DEPENDENCY_SUPERCLASS, as K2's FirMissingDependency*Checkers
// do with the language features of Kotlin 2.4: a constructor call's
// class with a missing supertype is only a warning there, and kc
// reports none.
func (c *Checker) missingDependencies() {
	reported := map[syntax.Pos]bool{}
	report := func(p syntax.Pos, s missingSet) {
		if len(s) == 0 || reported[p] {
			return
		}
		reported[p] = true
		names := make([]string, len(s))
		for i, cls := range s {
			names[i] = missingName(cls)
		}
		c.errorf(p, "cannot access class '%s'. Check your module classpath for missing or conflicting dependencies", strings.Join(names, "', '"))
	}
	superclass := func(p syntax.Pos, classes ...*types.ClassSym) {
		if reported[p] {
			return
		}
		for _, cls := range classes {
			if cls == nil || isMissingDep(cls) {
				continue
			}
			if ms := missingSupers(cls); len(ms) > 0 {
				reported[p] = true
				c.errorf(p, "cannot access '%s' which is a supertype of '%s'. Check your module classpath for missing or conflicting dependencies", missingName(ms[0]), missingName(cls))
				return
			}
		}
	}
	// access checks a call or a property's read: its declaration's
	// types, then the expression's type.
	access := func(p syntax.Pos, expr types.Type, ret types.Type, recv *types.Type, params []types.Type) {
		var other, fromExpr missingSet
		fromExpr.deep(expr)
		other.top(ret)
		if recv != nil {
			other.top(*recv)
			args(*recv, &other, &fromExpr)
		}
		for _, pt := range params {
			other.top(pt)
			args(pt, &other, &fromExpr)
		}
		if len(other) > 0 {
			report(p, other)
		} else {
			report(p, fromExpr)
		}
	}
	// getter checks the read of a property of a class through its
	// getter: its types, and the classes of its owner, of x when x is the
	// dispatch receiver, and of its extension receiver.
	getter := func(p syntax.Pos, expr types.Type, g *types.FunSym, x syntax.Expr) {
		var recv *types.Type
		var dispatch, ext *types.ClassSym
		if g.Ext {
			recv, ext = &g.Recv, classOf(g.Recv)
		} else if x != nil {
			dispatch = classOf(x.Type())
		}
		access(p, expr, g.Ret, recv, nil)
		superclass(p, dispatch, g.Owner, ext)
	}
	// inferredAnno reports at p the type annotation class name of an
	// inferred type that the class path doesn't have.
	inferredAnno := func(p syntax.Pos, name string) {
		if reported[p] {
			return
		}
		reported[p] = true
		c.errorf(p, "type annotation class '%s' of the inferred type is inaccessible. Check the module classpath for missing or conflicting dependencies", name[strings.LastIndexByte(name, '.')+1:])
	}
	// inferred checks the expression e whose type a declaration without
	// a declared type has.
	inferred := func(e syntax.Expr) {
		if call, ok := e.(*syntax.Call); ok && call.Sym != nil && call.Sym.MissingAnno != "" && !call.Invoke && call.Ctor == nil {
			p := call.P
			if call.Fun != nil {
				p = call.Fun.Pos()
				if m, ok := call.Fun.(*syntax.Member); ok {
					p = m.P
				}
			}
			inferredAnno(p, call.Sym.MissingAnno)
		}
	}
	var decl func(d syntax.Decl)
	var visit func(n any) bool
	visit = func(n any) bool {
		switch x := n.(type) {
		case *syntax.Call:
			if x.SAM && len(x.Args) == 1 && x.T.K == types.Class {
				// Sam { }: the lambda's parameters are of the types of the
				// interface's method's.
				if m := c.sam(x.T.C); m != nil && m.ParamMissingAnno != "" {
					inferredAnno(x.Args[0].Pos(), m.ParamMissingAnno)
				}
			}
			f := x.Sym
			if f == nil || x.Ctor != nil || x.Invoke || f.Name == "<init>" {
				return true
			}
			p := x.P
			if x.Fun != nil {
				p = x.Fun.Pos()
			}
			var recv *types.Type
			if f.Ext {
				recv = &f.Recv
			}
			var params []types.Type
			for i := range x.Args {
				j := i
				if x.ArgMap != nil && i < len(x.ArgMap) {
					j = x.ArgMap[i]
				}
				if j >= 0 && j < len(f.Params) {
					params = append(params, f.Params[j].Type)
				}
			}
			access(p, x.T, f.Ret, recv, params)
			var dispatch *types.ClassSym
			if m, ok := x.Fun.(*syntax.Member); ok && m.X != nil && !f.Ext {
				dispatch = classOf(m.X.Type())
			}
			var ext *types.ClassSym
			if f.Ext {
				ext = classOf(f.Recv)
			}
			superclass(p, dispatch, f.Owner, ext)
		case *syntax.Member:
			switch {
			case x.Prop != nil:
				access(x.P, x.T, x.Prop.Type, nil, nil)
				var dispatch *types.ClassSym
				if x.X != nil {
					dispatch = classOf(x.X.Type())
				}
				superclass(x.P, dispatch, ownerClass(x.Prop))
			case x.Getter != nil && x.Field == nil && x.Static == nil:
				getter(x.P, x.T, x.Getter, x.X)
			}
		case *syntax.Ident:
			switch {
			case x.Call != nil:
			case x.Getter != nil:
				getter(x.P, x.T, x.Getter, nil)
			case x.Member != nil:
				syntax.Inspect(x.Member, visit)
			case x.Sym != nil:
				access(x.P, x.T, x.Sym.Type, nil, nil)
				superclass(x.P, ownerClass(x.Sym))
			}
		case *syntax.Lambda:
			var s missingSet
			if x.Recv != nil {
				s.top(x.Recv.Type)
			}
			if x.It != nil {
				s.top(x.It.Type)
			}
			for _, lp := range x.Params {
				if lp.Type == nil && lp.Sym != nil {
					s.top(lp.Sym.Type)
				}
			}
			report(x.P, s)
		case *syntax.ObjectExpr:
			decl(x.Decl)
		case *syntax.LocalClass:
			decl(x.Decl)
		case *syntax.LocalFun:
			decl(x.Decl)
		}
		return true
	}
	inspect := func(x any) {
		if x != nil {
			syntax.Inspect(x, visit)
		}
	}
	fun := func(f *syntax.FunDecl) {
		if f == nil {
			return
		}
		for _, tp := range f.TypeParams {
			typeParamBounds(tp, superclass)
		}
		for _, p := range f.Params {
			if p.Default != nil {
				inspect(p.Default)
			}
		}
		if f.Body != nil {
			inspect(f.Body)
		}
		if f.ExprBody != nil {
			inspect(f.ExprBody)
		}
	}
	decl = func(d syntax.Decl) {
		switch x := d.(type) {
		case *syntax.FunDecl:
			if x.Ret == nil && x.ExprBody != nil {
				inferred(x.ExprBody)
			}
			fun(x)
		case *syntax.PropDecl:
			for _, tp := range x.TypeParams {
				typeParamBounds(tp, superclass)
			}
			if x.Init != nil {
				if x.Type == nil {
					inferred(x.Init)
				}
				inspect(x.Init)
			}
			if g := x.Getter; g != nil && x.Type == nil && g.Ret == nil && g.ExprBody != nil {
				inferred(g.ExprBody)
			}
			fun(x.Getter)
			fun(x.Setter)
		case *syntax.InitBlock:
			if x.Body != nil {
				inspect(x.Body)
			}
		case *syntax.ClassDecl:
			if x.Sym != nil && !x.Anonymous {
				superclass(x.P, x.Sym)
			}
			for _, tp := range x.TypeParams {
				typeParamBounds(tp, superclass)
			}
			for _, cp := range x.CtorParams {
				if x.Data && cp.IsProp && cp.Sym != nil {
					superclass(cp.P, classOf(cp.Sym.Type))
				}
				if cp.Default != nil {
					inspect(cp.Default)
				}
			}
			for _, st := range x.Supers {
				if st.Call != nil {
					inspect(st.Call)
				}
				if st.Delegate != nil {
					inspect(st.Delegate)
				}
			}
			for _, m := range x.Members {
				decl(m)
			}
		}
	}
	for _, d := range c.file.Decls {
		decl(d)
	}
}

// typeParamBounds checks the classes of a type parameter's bounds.
func typeParamBounds(tp *syntax.TypeParam, check func(syntax.Pos, ...*types.ClassSym)) {
	if tp.Sym == nil {
		return
	}
	for _, b := range tp.Sym.Bounds {
		check(tp.P, classOf(b))
	}
}

// ownerClass returns the class a property is a member of, or nil.
func ownerClass(v *types.VarSym) *types.ClassSym {
	if v.Kind != types.Prop {
		return nil
	}
	return v.Owner
}
