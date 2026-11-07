package ka

import "testing"

func TestAnnotationTypeArgs(t *testing.T) {
	m := update(t, `package p
annotation class A<T, U>
@A<String, List<Int>> class C
`)
	for _, d := range m.Files[0].Decls {
		if d.Name != "C" {
			continue
		}
		a := d.Annotations[0]
		if len(a.TypeArgs) != 2 {
			t.Fatalf("type args %v", a.TypeArgs)
		}
		for _, ta := range a.TypeArgs {
			t.Logf("%+v", ta)
		}
		if a.TypeArgs[0].ClassID != "kotlin/String" || a.TypeArgs[1].ClassID != "kotlin/collections/List" {
			t.Errorf("type args %+v %+v", a.TypeArgs[0], a.TypeArgs[1])
		}
	}
}

// A class path annotation's type arguments are resolved too (an error
// for a class that has none, which the checker leaves to the compiler).
func TestLibraryAnnotationTypeArgs(t *testing.T) {
	m := update(t, `package p
@Deprecated<String>("x") class C
`)
	a := m.Files[0].Decls[0].Annotations[0]
	if len(a.TypeArgs) != 1 || a.TypeArgs[0].ClassID != "kotlin/String" {
		t.Errorf("type args %v", a.TypeArgs)
	}
}
