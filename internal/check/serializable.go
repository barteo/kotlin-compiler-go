package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// The kotlinx.serialization compiler plugin's members for classes with a
// custom serializer.
//
// A class annotated @Serializable(with = S::class), or @Serializable(S::class),
// gets from kotlinc's serialization plugin a function serializer() that
// returns S: an object's is its own, and a class's its companion object's,
// which the plugin declares if the class has none. kc declares them too,
// as source, before the file is checked, so that other code, as other
// modules compiled by kotlinc, can call C.serializer(). Classes annotated
// @Serializable without a serializer, whose serializers the plugin
// generates, are left as they are.

// serializableWith adds the serializer() functions of the file's classes
// annotated @Serializable(with = S::class).
func (c *Checker) serializableWith() {
	if !c.importsSerialization() {
		return
	}
	var visit func(decls []syntax.Decl, outer string)
	visit = func(decls []syntax.Decl, outer string) {
		for _, d := range decls {
			cd, ok := d.(*syntax.ClassDecl)
			if !ok {
				continue
			}
			name := cd.Name
			if outer != "" {
				name = outer + "." + cd.Name
			}
			visit(cd.Members, name)
			if s := customSerializer(cd); s != nil && len(cd.TypeParams) == 0 && !cd.Interface && !cd.Annotation {
				addSerializerFun(cd, name, s)
			}
		}
	}
	visit(c.file.Decls, "")
}

// importsSerialization reports whether the file can name kotlinx.serialization's
// Serializable as Serializable.
func (c *Checker) importsSerialization() bool {
	if c.file.Package == "kotlinx.serialization" {
		return true
	}
	for _, imp := range c.file.Imports {
		if imp.Star && imp.Path == "kotlinx.serialization" || !imp.Star && imp.Path == "kotlinx.serialization.Serializable" {
			return true
		}
	}
	return false
}

// customSerializer returns the tokens of the class literal S::class of a
// class's @Serializable(with = S::class) annotation, without ::class, or nil.
func customSerializer(cd *syntax.ClassDecl) []syntax.Token {
	for i := range cd.Annotations {
		a := &cd.Annotations[i]
		if a.Name != "Serializable" && a.Name != "kotlinx.serialization.Serializable" || len(a.Args) != 1 {
			continue
		}
		arg := a.Args[0]
		if arg.Name != "" && arg.Name != "with" {
			continue
		}
		toks := arg.Toks
		n := len(toks)
		if n < 3 || toks[n-2].Kind != syntax.COLON_COLON || toks[n-1].Text != "class" {
			continue
		}
		for _, t := range toks[:n-2] {
			if t.Kind != syntax.IDENT && t.Kind != syntax.DOT {
				return nil
			}
		}
		return toks[:n-2]
	}
	return nil
}

// addSerializerFun declares fun serializer(): KSerializer<C> = S in class
// cd, named name, if it is an object, or else in its companion object.
func addSerializerFun(cd *syntax.ClassDecl, name string, s []syntax.Token) {
	p := cd.P
	var serializer strings.Builder
	for _, t := range s {
		serializer.WriteString(t.Text)
	}
	// S, an object; pluginSerializer makes it S() for a class.
	x := qualifiedExpr(serializer.String(), p)
	f := &syntax.FunDecl{P: p, Name: "serializer", ExprBody: x,
		Ret: &syntax.TypeRef{P: p, Name: "kotlinx.serialization.KSerializer", Args: []*syntax.TypeArg{{P: p, Type: &syntax.TypeRef{P: p, Name: name}}}}}
	f.Final = true
	f.Plugin = true
	if cd.Object {
		cd.Members = append(cd.Members, f)
		return
	}
	for _, m := range cd.Members {
		if comp, ok := m.(*syntax.ClassDecl); ok && comp.Companion {
			comp.Members = append(comp.Members, f)
			return
		}
	}
	comp := &syntax.ClassDecl{P: p, Name: "Companion", Object: true, HasPrimary: true, Members: []syntax.Decl{f}}
	comp.Companion = true
	cd.Members = append(cd.Members, comp)
}

// qualifiedExpr returns the expression a.b.C for a dotted name.
func qualifiedExpr(name string, p syntax.Pos) syntax.Expr {
	parts := strings.Split(name, ".")
	id := &syntax.Ident{Name: parts[0]}
	id.P = p
	var x syntax.Expr = id
	for _, part := range parts[1:] {
		m := &syntax.Member{X: x, Name: part}
		m.P = p
		x = m
	}
	return x
}

// pluginSerializer makes the result of a serializer() that kc declares a
// new instance of its serializer, S(), if S is a class rather than an
// object, as the plugin's.
func (c *Checker) pluginSerializer(d *syntax.FunDecl) {
	if !d.Plugin || d.ExprBody == nil {
		return
	}
	cls := c.classRef(d.ExprBody)
	if cls == nil || cls.Object {
		return
	}
	call := &syntax.Call{Fun: d.ExprBody}
	call.P = d.P
	d.ExprBody = call
}
