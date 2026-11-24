package check

import (
	"fmt"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// lessVisibleInline checks inline function d, a member of cls or not,
// that is neither private nor in a private or local class, for the types
// less visible than it that its copies would use, as kotlinc's
// ForbidExposingLessVisibleTypesInInline, on by default in Kotlin 2.4:
//
//   - a type of its own signature, or one its body names in a check, a
//     cast, a class literal or a local's declared type, and an object it
//     uses as a value (LESS_VISIBLE_TYPE_ACCESS_IN_INLINE_ERROR);
//   - a type in the signature of a declaration it uses: the return,
//     parameter and receiver types of a function or a constructor, the
//     type of a property or of one of its parameters, and the class of a
//     member (LESS_VISIBLE_TYPE_IN_INLINE_ACCESSED_SIGNATURE_ERROR);
//   - a member of a private class, or of its companion object, it uses
//     (PRIVATE_CLASS_MEMBER_FROM_INLINE).
//
// A type is less visible if its class is local, or private, or nested in
// a private class, or, for a public-API inline function, internal. The
// classes declared in d itself, which its copies have, aren't. A
// public-API inline function's uses are checked by publicInline
// (NON_PUBLIC_CALL_FROM_PUBLIC_INLINE), which kotlinc reports instead of
// the last two.
func (c *Checker) lessVisibleInline(d *syntax.FunDecl, cls *types.ClassSym) {
	if d.Sym == nil || effectivelyPrivate(d.Visibility, cls) {
		return
	}
	for k := cls; k != nil; k = k.Outer {
		if k.Local || k.Anonymous {
			return
		}
	}
	publicAPI := publicAPIInline(d, cls)
	level := "internal"
	if publicAPI {
		level = "public"
	}
	own := map[*types.ClassSym]bool{} // the classes d declares
	inspectInline(d, func(n any) bool {
		switch n := n.(type) {
		case *syntax.ObjectExpr:
			if n.Decl != nil && n.Decl.Sym != nil {
				own[n.Decl.Sym] = true
			}
		case *syntax.LocalClass:
			if n.Decl != nil && n.Decl.Sym != nil {
				own[n.Decl.Sym] = true
			}
		}
		return true
	})
	reported := map[syntax.Pos]map[string]bool{}
	report := func(p syntax.Pos, format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if reported[p][msg] {
			return
		}
		if reported[p] == nil {
			reported[p] = map[string]bool{}
		}
		reported[p][msg] = true
		c.errorf(p, "%s", msg)
	}
	// lessVisible returns how type t, or a type among its arguments, is
	// less visible than d, and that type, or "".
	var lessVisible func(t types.Type) (string, types.Type)
	lessVisible = func(t types.Type) (string, types.Type) {
		if t.K != types.Class || t.C == nil {
			return "", t
		}
		if why := c.classLessVisible(t.C, publicAPI, own); why != "" {
			return why, t
		}
		if t.A != nil {
			for _, a := range t.A.List {
				if a.V == types.Star {
					continue
				}
				if why, u := lessVisible(a.T); why != "" {
					return why, u
				}
			}
		}
		return "", t
	}
	access := func(p syntax.Pos, t types.Type) {
		if why, u := lessVisible(t); why != "" {
			report(p, "'%s' type '%s' is accessed from '%s' inline declaration", why, u, level)
		}
	}
	// signature checks the use at p of a declaration named what, with the
	// types ts in its signature.
	signature := func(p syntax.Pos, what string, ts ...types.Type) {
		if publicAPI {
			return
		}
		for _, t := range ts {
			if why, u := lessVisible(t); why != "" {
				report(p, "the signature of '%s' contains '%s' type '%s', but is accessed from '%s' inline declaration", what, why, u, level)
			}
		}
	}
	privateMember := func(p syntax.Pos, owner *types.ClassSym, what string) {
		if publicAPI || owner == nil || !privateClassMember(owner) {
			return
		}
		report(p, "non-private inline function '%s' cannot access members of private class '%s'", funText(d.Sym), what)
	}
	ownerType := func(owner *types.ClassSym, static bool) []types.Type {
		if owner == nil || static {
			return nil
		}
		return []types.Type{types.ClassType(owner)}
	}
	// usesVar checks a use at p of variable v.
	usesVar := func(p syntax.Pos, v *types.VarSym) {
		switch v.Kind {
		case types.Param:
			if !isParamOf(v, d) {
				return // a lambda's, which is d's own
			}
			signature(p, v.Name, v.Type)
		case types.Global, types.Prop:
			if v.Owner != nil && c.localMember(v.Owner) {
				return
			}
			kind := "val"
			if v.Mutable {
				kind = "var"
			}
			signature(p, kind+" "+v.Name+": "+v.Type.String(), append([]types.Type{v.Type}, ownerType(v.Owner, false)...)...)
			privateMember(p, v.Owner, kind+" "+v.Name+": "+v.Type.String())
		}
	}
	// usesObject checks a use at p of object k as a value; recv is set on
	// the receiver of a member access.
	usesObject := func(p syntax.Pos, k *types.ClassSym, recv bool) {
		if !recv {
			access(p, types.ClassType(k))
		}
		if k.Outer != nil && k.Outer.Companion == k {
			name := k.DisplayName()
			name = name[strings.LastIndexByte(name, '.')+1:]
			privateMember(p, k.Outer, "companion object "+name+" : Any")
		}
	}
	receivers := map[syntax.Expr]bool{}
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Member:
			receivers[n.X] = true
			if n.Object != nil {
				usesObject(n.P, n.Object, receivers[n])
			} else if v := n.Prop; v != nil {
				usesVar(n.P, v)
			}
		case *syntax.Ident:
			switch {
			case n.Object != nil:
				usesObject(n.P, n.Object, receivers[n])
			case n.Member != nil:
				visit(n.Member)
			case n.Sym != nil:
				usesVar(n.P, n.Sym)
			}
		case *syntax.Call:
			f := n.Sym
			if f == nil || n.Invoke {
				break
			}
			p := n.P
			if fn := n.Fun; fn != nil {
				p = fn.Pos()
			}
			if k := n.Ctor; k != nil {
				if c.localMember(k) || own[k] {
					break
				}
				what := "constructor(): " + k.DisplayName()
				ts := append([]types.Type{types.ClassType(k)}, paramTypes(f)...)
				signature(p, what, ts...)
				privateMember(p, k, what)
				break
			}
			if fd, ok := f.Decl.(*syntax.FunDecl); ok && fd == d {
				break
			}
			if f.Owner != nil && c.localMember(f.Owner) {
				break
			}
			if _, ok := f.Decl.(*syntax.FunDecl); !ok {
				break // the class path's, which can't use the program's classes
			}
			what := funText(f)
			ts := append([]types.Type{f.Ret}, paramTypes(f)...)
			if f.Ext && f.Recv.K != types.Invalid {
				ts = append(ts, f.Recv)
			}
			ts = append(ts, ownerType(f.Owner, f.Static)...)
			signature(p, what, ts...)
			privateMember(p, f.Owner, what)
		case *syntax.Is:
			if n.Ref != nil {
				access(n.Ref.P, n.Target)
			}
		case *syntax.As:
			if n.Ref != nil {
				access(n.Ref.P, n.Target)
			}
		case *syntax.ClassLit:
			if !n.Bound && n.Ref != nil {
				access(n.Ref.P, n.Class)
			}
		case *syntax.VarStmt:
			if n.Type != nil && n.Sym != nil {
				access(n.Type.P, n.Sym.Type)
			}
		case *syntax.Lambda:
			for _, lp := range n.Params {
				if lp.Type != nil && lp.Sym != nil {
					access(lp.Type.P, lp.Sym.Type)
				}
			}
		}
		return true
	}
	// Its own signature.
	if d.Recv != nil && d.Sym.Ext {
		access(d.Recv.P, d.Sym.Recv)
	}
	for i, p := range d.Params {
		if p.Type != nil && i < len(d.Sym.Params) {
			access(p.Type.P, d.Sym.Params[i].Type)
		}
	}
	if d.Ret != nil {
		access(d.Ret.P, d.Sym.Ret)
	}
	// The default values of noinline parameters aren't inlined.
	for _, p := range d.Params {
		if p.Default != nil && !p.Noinline {
			syntax.Inspect(p.Default, visit)
		}
	}
	if d.Body != nil {
		syntax.Inspect(d.Body, visit)
	}
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
}

// classLessVisible returns how class k, or the class it is nested in, is
// less visible than an inline function, public API or not, whose own
// classes are own: "local", "private-in-file", "private-in-class" or
// "internal", or "".
func (c *Checker) classLessVisible(k *types.ClassSym, publicAPI bool, own map[*types.ClassSym]bool) string {
	why := ""
	for ; k != nil; k = k.Outer {
		if own[k] {
			return ""
		}
		cd, ok := k.Decl.(*syntax.ClassDecl)
		if !ok {
			return why // the class path's
		}
		switch {
		case k.Local || k.Anonymous || cd.Local || cd.Anonymous:
			return "local"
		case cd.Visibility == syntax.Private:
			if k.Outer == nil {
				why = "private-in-file"
			} else {
				why = "private-in-class"
			}
		case why == "" && publicAPI && cd.Visibility == syntax.Internal && cd.Find("", "PublishedApi") == nil:
			why = "internal"
		}
	}
	return why
}

// localMember reports whether the members of class k are local: it is a
// local class or an object expression's, or nested in one.
func (c *Checker) localMember(k *types.ClassSym) bool {
	return localType(types.ClassType(k))
}

// privateClassMember reports whether a member of class owner is a member
// of a private class, as kotlinc's PRIVATE_CLASS_MEMBER_FROM_INLINE sees
// it: owner, or the class whose companion object it is, is private.
func privateClassMember(owner *types.ClassSym) bool {
	k := owner
	for k.Outer != nil && k.Outer.Companion == k {
		k = k.Outer
	}
	cd, ok := k.Decl.(*syntax.ClassDecl)
	return ok && cd.Visibility == syntax.Private && !cd.Local && !cd.Anonymous
}

// isParamOf reports whether v is one of function d's parameters.
func isParamOf(v *types.VarSym, d *syntax.FunDecl) bool {
	for _, p := range d.Params {
		if p.Sym == v {
			return true
		}
	}
	return d.RecvSym == v
}

// paramTypes returns the types of function f's parameters and context
// parameters.
func paramTypes(f *types.FunSym) []types.Type {
	var ts []types.Type
	for _, p := range f.Params {
		ts = append(ts, p.Type)
	}
	for _, p := range f.Contexts {
		ts = append(ts, p.Type)
	}
	return ts
}

// funText renders f as kotlinc's messages do: fun name(a: A): R.
func funText(f *types.FunSym) string {
	var b strings.Builder
	b.WriteString("fun ")
	if f.Ext && f.Recv.K != types.Invalid {
		b.WriteString(f.Recv.String() + ".")
	}
	b.WriteString(f.Name + "(")
	for i, p := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name + ": " + p.Type.String())
	}
	b.WriteString("): " + f.Ret.String())
	return b.String()
}
