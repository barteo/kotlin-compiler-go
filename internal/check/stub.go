package check

import (
	"cmp"
	"fmt"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Stub renders the class path's declaration named qualified, a top-level
// class (a.b.C) or the top-level functions and properties of a name
// (a.b.f), as Kotlin source for an editor to show when the library has no
// sources: their signatures, without bodies, default values or
// annotations, and with types by their simple names. A class's members
// are those kc reads; a name's functions and properties are its
// package's public ones. It returns the file's path, a/b/C.kt, and its
// text; qualified may name a member, a.b.C.m, whose class is rendered.
func (l *Library) Stub(qualified string) (file, text string, ok bool) {
	t := l.t
	parts := strings.Split(qualified, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		pkg, name := strings.Join(parts[:i], "."), parts[i]
		var b stubWriter
		b.header(qualified, pkg)
		id := qualified2(pkg, name)
		cls := l.Class(id)
		if cls != nil {
			cls.Ensure()
		}
		if kc := t.builtinClasses()[id]; t.kotlin && kc != nil && (cls == nil || cls.Missing) {
			b.builtin(t, kc, name) // String, Int, Any: no class file
		} else if cls != nil {
			if cls.Missing {
				continue
			}
			b.class(cls, name, "")
		} else if t.kotlin {
			funs := t.pkgFuns(pkg, name)
			props := t.pkgExtProps(pkg, name)
			v := t.pkgVar(pkg, name)
			if len(funs) == 0 && len(props) == 0 && v == nil {
				continue
			}
			if v != nil {
				b.variable(v, "")
			}
			for _, g := range props {
				b.extProperty(g, "")
			}
			for _, f := range funs {
				b.fun(f, "")
			}
		} else {
			continue
		}
		path := name + ".kt"
		if pkg != "" {
			path = strings.ReplaceAll(pkg, ".", "/") + "/" + path
		}
		return path, b.String(), true
	}
	return "", "", false
}

func qualified2(pkg, name string) string {
	return qualified(strings.ReplaceAll(pkg, ".", "/"), name)
}

type stubWriter struct{ strings.Builder }

func (b *stubWriter) header(qualified, pkg string) {
	fmt.Fprintf(b, "// %s: a stub from the class path's metadata; no sources were found.\n", qualified)
	if pkg != "" {
		fmt.Fprintf(b, "\npackage %s\n", pkg)
	}
	b.WriteString("\n")
}

// class writes cls, declared as name, indented by indent.
func (b *stubWriter) class(cls *types.ClassSym, name, indent string) {
	var mods []string
	if cls.Private {
		mods = append(mods, "internal")
	}
	kw := "class"
	switch {
	case cls.Annotation:
		kw = "annotation class"
	case cls.Entries != nil:
		kw = "enum class"
	case cls.Interface:
		kw = "interface"
		if cls.Fun {
			kw = "fun interface"
		}
	case cls.Object:
		kw = "object"
		if cls.CompanionOf != nil {
			kw = "companion object"
		}
	default:
		if cls.Sealed {
			mods = append(mods, "sealed")
		} else if cls.Abstract {
			mods = append(mods, "abstract")
		} else if !cls.Final {
			mods = append(mods, "open")
		}
		if cls.Value {
			mods = append(mods, "value")
		}
	}
	b.WriteString(indent)
	for _, m := range mods {
		b.WriteString(m + " ")
	}
	b.WriteString(kw + " " + name + typeParams(cls.TypeParams))
	var supers []string
	if len(cls.Supers) > 0 {
		for _, s := range cls.Supers {
			supers = append(supers, stubType(s))
		}
	} else {
		if s := cls.Super; s != nil && !isRoot(s) && !cls.Interface && cls.Entries == nil {
			supers = append(supers, s.DisplayName())
		}
		for _, s := range cls.Interfaces {
			if s.Name != "java/lang/annotation/Annotation" {
				supers = append(supers, s.DisplayName())
			}
		}
	}
	if len(supers) > 0 {
		b.WriteString(" : " + strings.Join(supers, ", "))
	}
	b.WriteString(" {\n")
	in := indent + "    "
	if len(cls.Entries) > 0 {
		b.WriteString(in + strings.Join(cls.Entries, ", ") + ";\n\n")
	}
	ctors := cls.Ctors
	if len(ctors) == 0 && cls.Ctor != nil {
		ctors = []*types.FunSym{cls.Ctor}
	}
	if !cls.Object && !cls.Interface {
		for _, c := range ctors {
			if !c.Hidden {
				b.WriteString(in + "constructor(" + params(c.Params) + ")\n")
			}
		}
	}
	b.members(cls, in)
	var statics []func()
	if len(cls.Fields) > 0 || len(cls.Statics) > 0 {
		statics = append(statics, func() {
			for _, name := range sortedKeys(cls.Fields) {
				b.variable(cls.Fields[name], in+"    ")
			}
			for _, name := range sortedKeys(cls.Statics) {
				for _, f := range cls.Statics[name] {
					b.fun(f, in+"    ")
				}
			}
		})
	}
	if c := cls.Companion; c != nil {
		c.Ensure()
		statics = append(statics, func() { b.members(c, in+"    ") })
	}
	if len(statics) > 0 {
		b.WriteString("\n" + in + "companion object {\n")
		for _, f := range statics {
			f()
		}
		b.WriteString(in + "}\n")
	}
	b.WriteString(indent + "}\n")
}

// builtin writes kc, a class of the standard library's builtins, which
// has no class file, declared as name: its supertypes and its public
// properties and functions.
func (b *stubWriter) builtin(t *classTable, kc *kmeta.Class, name string) {
	scope := tscope{}
	tps, _ := t.ktypeParams(kc.TypeParams, nil, scope)
	var mods []string
	kw := "class"
	switch kc.Kind() {
	case kmeta.KindInterface:
		kw = "interface"
	case kmeta.KindObject, kmeta.KindCompanion:
		kw = "object"
	case kmeta.KindEnum:
		kw = "enum class"
	case kmeta.KindAnnotation:
		kw = "annotation class"
	default:
		switch kc.Flags.Modality() {
		case kmeta.Open:
			mods = append(mods, "open")
		case kmeta.Abstract:
			mods = append(mods, "abstract")
		case kmeta.Sealed:
			mods = append(mods, "sealed")
		}
	}
	for _, m := range mods {
		b.WriteString(m + " ")
	}
	b.WriteString(kw + " " + name + typeParams(tps))
	var supers []string
	for _, st := range kc.Supers {
		if typ, ok := t.ktype(st, scope); ok && !(typ.K == types.Class && typ.C != nil && isRoot(typ.C)) {
			supers = append(supers, stubType(typ))
		}
	}
	if len(supers) > 0 {
		b.WriteString(" : " + strings.Join(supers, ", "))
	}
	b.WriteString(" {\n")
	const in = "    "
	for _, p := range kc.Props {
		if p.Flags.Visibility() != kmeta.Public {
			continue
		}
		if typ, ok := t.ktype(p.Type, scope); ok {
			b.variable(&types.VarSym{Name: p.Name, Type: typ}, in)
		}
	}
	for _, f := range kc.Funcs {
		if f.Flags.Visibility() != kmeta.Public {
			continue
		}
		if sym := t.kfun(f, nil, scope, nil); !sym.Unsupported {
			b.fun(sym, in)
		}
	}
	b.WriteString("}\n")
}

// members writes cls's properties and methods.
func (b *stubWriter) members(cls *types.ClassSym, in string) {
	seen := map[string]bool{}
	for _, p := range cls.Props {
		seen[p.Name] = true
	}
	var props []*types.VarSym
	props = append(props, cls.Props...)
	for _, name := range sortedKeys(cls.KProps) {
		if p := cls.KProps[name]; !seen[name] && p.Getter != nil && !p.Getter.Hidden {
			props = append(props, &types.VarSym{Name: name, Type: p.Getter.Ret, Mutable: p.Setter != nil})
			seen[name] = true
		}
	}
	slices.SortStableFunc(props, func(a, b *types.VarSym) int { return cmp.Compare(a.Name, b.Name) })
	for _, p := range props {
		b.variable(p, in)
	}
	for _, name := range sortedKeys(cls.ExtProps) {
		for _, g := range cls.ExtProps[name] {
			b.extProperty(g, in)
		}
	}
	for _, name := range sortedKeys(cls.Methods) {
		for _, f := range cls.Methods[name] {
			b.fun(f, in)
		}
	}
	for _, name := range sortedKeys(cls.ExtMethods) {
		for _, f := range cls.ExtMethods[name] {
			b.fun(f, in)
		}
	}
}

func (b *stubWriter) variable(v *types.VarSym, in string) {
	kw := "val"
	if v.Mutable {
		kw = "var"
	}
	fmt.Fprintf(b, "%s%s %s: %s\n", in, kw, quote(v.Name), stubType(v.Type))
}

// extProperty writes an extension property, of its getter g.
func (b *stubWriter) extProperty(g *types.FunSym, in string) {
	if g.Hidden {
		return
	}
	kw := "val"
	if g.ExtSetter != nil {
		kw = "var"
	}
	recv := ""
	if g.Ext {
		recv = stubType(g.Recv) + "."
	}
	tp := typeParams(g.TypeParams)
	if tp != "" {
		tp += " "
	}
	fmt.Fprintf(b, "%s%s %s%s%s: %s\n", in, kw, tp, recv, quote(g.Name), stubType(g.Ret))
}

func (b *stubWriter) fun(f *types.FunSym, in string) {
	if f.Hidden || f.Name == "<init>" || f.Name == "<clinit>" {
		return
	}
	var mods []string
	if f.Abstract && f.Owner != nil && !f.Owner.Interface {
		mods = append(mods, "abstract")
	}
	if f.Inline != nil {
		mods = append(mods, "inline")
	}
	if f.Operator && (f.Owner == nil || f.Owner.KotlinName != "") { // any Java method may be
		mods = append(mods, "operator")
	}
	if f.Infix {
		mods = append(mods, "infix")
	}
	if f.Suspend {
		mods = append(mods, "suspend")
	}
	b.WriteString(in)
	for _, m := range mods {
		b.WriteString(m + " ")
	}
	b.WriteString("fun ")
	if tp := typeParams(f.TypeParams); tp != "" {
		b.WriteString(tp + " ")
	}
	if f.Ext {
		b.WriteString(stubType(f.Recv) + ".")
	}
	b.WriteString(quote(f.Name) + "(" + params(f.Params) + ")")
	if f.Ret.K != types.Unit && f.Ret.K != types.Invalid {
		b.WriteString(": " + stubType(f.Ret))
	}
	b.WriteString("\n")
}

func params(ps []*types.VarSym) string {
	var out []string
	names := map[string]bool{}
	unique := true
	for _, p := range ps {
		unique = unique && !names[p.Name]
		names[p.Name] = true
	}
	for i, p := range ps {
		name := p.Name
		if !unique { // a Java method's, without its parameters' names
			name = fmt.Sprintf("p%d", i)
		}
		typ := p.Type
		if p.Vararg && typ.K == types.Class && typ.C != nil && typ.C.Array {
			if typ.A != nil && len(typ.A.List) == 1 {
				typ = typ.A.List[0].T
			} else {
				typ = typ.C.Elem
			}
		}
		s := quote(name) + ": " + stubType(typ)
		if p.Vararg {
			s = "vararg " + s
		}
		if p.Default {
			s += " /* = ... */"
		}
		out = append(out, s)
	}
	return strings.Join(out, ", ")
}

// typeParams renders type parameters, <out T : B>, or "".
func typeParams(ps []*types.TypeParamSym) string {
	if len(ps) == 0 {
		return ""
	}
	var out []string
	for _, p := range ps {
		s := p.Name
		switch p.Var {
		case types.Out:
			s = "out " + s
		case types.In:
			s = "in " + s
		}
		if p.Reified {
			s = "reified " + s
		}
		if len(p.Bounds) == 1 {
			s += " : " + stubType(p.Bounds[0])
		}
		out = append(out, s)
	}
	return "<" + strings.Join(out, ", ") + ">"
}

// stubType renders t with its classes' simple names, without platform
// types' "!".
func stubType(t types.Type) string {
	if t.K == types.Invalid {
		return "Any?"
	}
	return strings.ReplaceAll(t.Format(nil), "!", "")
}

func isRoot(c *types.ClassSym) bool {
	return c.Name == "java/lang/Object" || c.KotlinName == "kotlin/Any"
}

func quote(name string) string {
	if name == "" || strings.ContainsAny(name, " -.$<>") || keywords[name] {
		return "`" + name + "`"
	}
	return name
}

var keywords = map[string]bool{
	"as": true, "break": true, "class": true, "continue": true, "do": true, "else": true, "false": true, "for": true,
	"fun": true, "if": true, "in": true, "interface": true, "is": true, "null": true, "object": true, "package": true,
	"return": true, "super": true, "this": true, "throw": true, "true": true, "try": true, "typealias": true,
	"typeof": true, "val": true, "var": true, "when": true, "while": true,
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
