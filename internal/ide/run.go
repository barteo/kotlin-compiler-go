package ide

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// MainFunc is an entry point of a program: a main function the JVM runs,
// of class Class, the binary name the java command takes.
type MainFunc struct {
	Decl  psi.Range // the function's declaration
	Name  psi.Range // its name
	Class string
}

// MainFunctions returns snap's entry points, by their syntax: top-level
// main functions with no parameters or an array of strings, and
// @JvmStatic ones with an array of strings in a top-level object or a
// top-level class's companion.
func MainFunctions(snap *psi.Snapshot) []MainFunc {
	pkg := packageName(snap)
	qualify := func(name string) string {
		if pkg == "" {
			return name
		}
		return pkg + "." + name
	}
	var out []MainFunc
	add := func(f psi.NodeRef, class string) {
		out = append(out, MainFunc{Decl: f.Range(), Name: f.NameIdentifier().Range(), Class: qualify(class)})
	}
	root := snap.RootNode()
	for d := root.FirstChild(); d.Valid(); d = d.NextSibling() {
		switch d.NumericKind() {
		case psi.KindFUN:
			if isMain(d, true) {
				add(d, facadeName(snap))
			}
		case psi.KindOBJECT_DECLARATION:
			for _, f := range staticMains(d) {
				add(f, d.Name())
			}
		case psi.KindCLASS:
			for c := d.ChildKind(psi.KindCLASS_BODY).FirstChild(); c.Valid(); c = c.NextSibling() {
				if c.NumericKind() == psi.KindOBJECT_DECLARATION && hasModifier(c, psi.KindCOMPANION_KEYWORD) {
					for _, f := range staticMains(c) {
						add(f, d.Name())
					}
				}
			}
		}
	}
	return out
}

// staticMains returns the @JvmStatic main functions of an object.
func staticMains(o psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	for f := o.ChildKind(psi.KindCLASS_BODY).FirstChild(); f.Valid(); f = f.NextSibling() {
		if f.NumericKind() == psi.KindFUN && isMain(f, false) && annotated(f, "JvmStatic") {
			out = append(out, f)
		}
	}
	return out
}

// isMain reports whether f is a main function by its syntax: named main,
// public, with no receiver nor type parameters, a Unit result, and an
// Array<String> or vararg String parameter, or none if top is set.
func isMain(f psi.NodeRef, top bool) bool {
	if f.Name() != "main" || f.ReceiverType().Valid() || f.ChildKind(psi.KindTYPE_PARAMETER_LIST).Valid() || !f.Body().Valid() {
		return false
	}
	for _, k := range []psi.Kind{psi.KindPRIVATE_KEYWORD, psi.KindPROTECTED_KEYWORD, psi.KindINTERNAL_KEYWORD, psi.KindABSTRACT_KEYWORD} {
		if hasModifier(f, k) {
			return false
		}
	}
	if r := f.ReturnType(); r.Valid() && compact(r.Text()) != "Unit" {
		return false
	}
	ps := f.Parameters()
	switch len(ps) {
	case 0:
		return top
	case 1:
		t := compact(ps[0].TypeReference().Text())
		if hasModifier(ps[0], psi.KindVARARG_KEYWORD) {
			return t == "String"
		}
		return t == "Array<String>" || t == "Array<out String>"
	}
	return false
}

// annotated reports whether d carries an annotation of simple name name.
func annotated(d psi.NodeRef, name string) bool {
	for a := d.ChildKind(psi.KindMODIFIER_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() == psi.KindANNOTATION_ENTRY && annotationName(a) == name {
			return true
		}
	}
	return false
}

// annotationName returns an annotation entry's simple name.
func annotationName(a psi.NodeRef) string {
	t := a.ChildKind(psi.KindCONSTRUCTOR_CALLEE).Text()
	if i := strings.LastIndexByte(t, '.'); i >= 0 {
		t = t[i+1:]
	}
	return strings.TrimSpace(t)
}

// facadeName returns the simple name of the class of snap's top-level
// declarations: @file:JvmName's, or its file name's.
func facadeName(snap *psi.Snapshot) string {
	for a := snap.RootNode().ChildKind(psi.KindFILE_ANNOTATION_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() != psi.KindANNOTATION_ENTRY || annotationName(a) != "JvmName" {
			continue
		}
		arg := a.ChildKind(psi.KindVALUE_ARGUMENT_LIST).ChildKind(psi.KindVALUE_ARGUMENT)
		if s := arg.ChildKind(psi.KindSTRING_TEMPLATE); s.Valid() && !strings.ContainsAny(s.Text(), "$\\") {
			return strings.Trim(s.Text(), `"`)
		}
	}
	return syntax.FacadeName(snap.Path())
}
