package ka

import (
	"context"
	"strings"
	"testing"
)

// The package's tests check every file's keys against its names
// (declaredNames.affects).
func init() {
	verifyKeys = true
	verifyReach = true
}

func TestIntersects(t *testing.T) {
	many := make([]uint64, 100)
	for i := range many {
		many[i] = uint64(i * 2)
	}
	for _, c := range []struct {
		a, b []uint64
		want bool
	}{
		{nil, nil, false},
		{nil, []uint64{1}, false},
		{[]uint64{1}, []uint64{1}, true},
		{[]uint64{1, 3, 5}, []uint64{2, 4, 6}, false},
		{[]uint64{1, 3, 6}, []uint64{2, 4, 6}, true},
		{[]uint64{7}, many, false},   // searched
		{[]uint64{98}, many, true},   // searched
		{many, []uint64{199}, false}, // searched, either way round
		{many[:40], many[30:], true}, // merged
		{many[:30], many[30:], false},
	} {
		if got := intersects(c.a, c.b); got != c.want {
			t.Errorf("intersects(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// TestFileKeys checks that a file's keys find each single name and class
// that hit and declaredBy find, and only those.
func TestFileKeys(t *testing.T) {
	texts := []string{
		"package p\nopen class A { open fun f() = 1; class N { val n = 0 } }\nfun top() = 1\nval tv = 2\n",
		"package p\nclass B : A() { override fun f() = 2 }\nenum class E { X, Y }\n",
		"package q\nimport p.*\nfun viaImport() = A().f() + A.N().n\nfun e() = E.X\n",
		"fun inDefault() = 0\n",
	}
	s := Open(withStdlib(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources(texts...))
	if err != nil {
		t.Fatal(err)
	}
	var probes []*declaredNames
	probe := func(add func(n *declaredNames)) {
		n := newDeclaredNames()
		add(n)
		n.seal()
		probes = append(probes, n)
	}
	for _, f := range m.Files {
		for _, l := range f.Lookups {
			probe(func(n *declaredNames) { n.add(l.Scope, l.Name) })
			probe(func(n *declaredNames) { n.classes[l.Scope] = true })
		}
		var walk func(scope string, ds []*Decl)
		walk = func(scope string, ds []*Decl) {
			for _, d := range ds {
				probe(func(n *declaredNames) { n.add(scope, d.Name) })
				if d.ClassID != "" {
					walk(strings.ReplaceAll(d.ClassID, "/", "."), d.Members)
				}
			}
		}
		walk(f.Package, f.Decls)
	}
	probe(func(n *declaredNames) { n.add("p", "nowhere") })
	if len(probes) < 20 {
		t.Fatalf("only %d probes", len(probes))
	}
	for _, f := range m.Files {
		k := newFileKeys(f)
		for _, n := range probes {
			if got, want := n.hitKeys(k), n.hit(f.Lookups); got != want {
				t.Errorf("%s: hitKeys %v, hit %v for %v %v", f.Path, got, want, n.names, n.classes)
			}
			if got, want := n.declaredByKeys(k), n.declaredBy(f); got != want {
				t.Errorf("%s: declaredByKeys %v, declaredBy %v for %v", f.Path, got, want, n.names)
			}
		}
	}
}
