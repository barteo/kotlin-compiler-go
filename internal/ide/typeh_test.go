package ide

import (
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestTypeHierarchy(t *testing.T) {
	const a = `package p

sealed interface Shape
class Circle(val r: Double) : Shape
object Origin : Shape
open class Base
class Derived : Base(), Shape, Iterable<Int> {
    override fun iterator() = listOf(1).iterator()
}
`
	v := testView(t, "a.kt", a)
	names := func(items []TypeItem) string {
		var out []string
		for _, it := range items {
			out = append(out, it.Qualified)
		}
		return strings.Join(out, " ")
	}
	it, ok := PrepareTypeHierarchy([]*View{v}, "a.kt", caret(t, a, "class Ci|rcle"), nil)
	if !ok || it.Qualified != "p.Circle" || it.Name != "Circle" {
		t.Fatalf("prepare: %+v %v", it, ok)
	}
	if _, ok := PrepareTypeHierarchy([]*View{v}, "a.kt", caret(t, a, "val |r"), nil); ok {
		t.Errorf("prepare on a property")
	}
	if it, ok := PrepareTypeHierarchy([]*View{v}, "a.kt", caret(t, a, "Base|()"), nil); !ok || it.Qualified != "p.Base" {
		t.Errorf("prepare on a supertype: %+v %v", it, ok)
	}
	d := TypeDecl(v, "a.kt", caret(t, a, "class |Derived"))
	if got := names(Supertypes([]*View{v}, d, nil)); got != "p.Base p.Shape" {
		t.Errorf("supertypes of Derived: %q", got)
	}
	if got := names(Subtypes([]*View{v}, "p.Shape")); got != "p.Circle p.Origin p.Derived" {
		t.Errorf("subtypes of Shape: %q", got)
	}
	if got := names(Subtypes([]*View{v}, "kotlin.collections.Iterable")); got != "p.Derived" {
		t.Errorf("subtypes of Iterable: %q", got)
	}

	// A library class's supertypes resolve by its file's syntax.
	lib := psi.Parse("lib/Lib.kt", `package lib

import other.Thing as T

abstract class Lib<E> : AbstractList<E>(), T, Local, java.io.Serializable
`)
	// A class of its own package comes before a default import's.
	var asked []string
	find := func(tg libsrc.Target) (psi.NodeRef, bool) {
		asked = append(asked, tg.Name)
		return psi.NodeRef{}, false
	}
	Supertypes(nil, lib.RootNode().ChildKind(psi.KindCLASS), find)
	if got := strings.Join(asked, " "); got != "lib.AbstractList lib.Local kotlin.collections.AbstractList other.Thing lib.Local java.io.Serializable" {
		t.Errorf("library supertypes asked: %q", got)
	}
}
