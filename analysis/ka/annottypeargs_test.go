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

// abbrevString writes t with its typealiases as written.
func abbrevString(t *Type) string {
	if t == nil {
		return "*"
	}
	if t.Abbrev != nil {
		t = t.Abbrev
	}
	s := t.ClassID
	if t.Kind == TypeParameter {
		s = t.Name
	}
	if len(t.Args) > 0 {
		s += "<"
		for i, a := range t.Args {
			if i > 0 {
				s += ", "
			}
			if a.Variance == Out {
				s += "out "
			}
			s += abbrevString(a.Type)
		}
		s += ">"
	}
	return s
}

// The typealiases an alias's right side writes stay in its expansion.
func TestRightSideAbbrevs(t *testing.T) {
	m := update(t, `package p
typealias A23 = (Any) -> Any
typealias A77<Q> = List<Q>
typealias A73<Q> = Map<String, Q>
typealias A43<Q> = A77<Q>
typealias A47<Q> = A43<Q>
typealias A41<Z, Q> = (A43<Z>) -> A47<Q>
fun <T : A41<A23, out A77<A73<Int>>>> bar(x: T) {}
`)
	var got string
	for _, d := range m.Files[0].Decls {
		if d.Name == "bar" {
			b := d.TypeParams[0].Bounds[0]
			got = abbrevString(b.Abbrev) + " = " + b.ClassID + "<"
			for i, a := range b.Args {
				if i > 0 {
					got += ", "
				}
				if a.Variance == Out {
					got += "out "
				}
				got += abbrevString(a.Type)
			}
			got += ">"
		}
	}
	want := "p/A41<p/A23, out p/A77<p/A73<kotlin/Int>>> = kotlin/Function1<p/A43<p/A23>, p/A47<out p/A77<p/A73<kotlin/Int>>>>"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
