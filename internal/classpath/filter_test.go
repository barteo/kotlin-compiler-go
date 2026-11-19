package classpath

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

func TestHashJoin(t *testing.T) {
	for _, c := range []struct {
		a   string
		sep byte
		b   string
	}{{"java/util", '/', "Map"}, {"java/util/Map", '$', "Entry"}, {"", '/', "A"}, {"a", '$', ""}} {
		if got, want := HashJoin(Hash(c.a), c.sep, c.b), Hash(c.a+string(c.sep)+c.b); got != want {
			t.Errorf("HashJoin(%q, %q, %q) = %x, want %x", c.a, c.sep, c.b, got, want)
		}
	}
}

func TestNameFilter(t *testing.T) {
	for _, n := range []int{0, 1, 100, 30000} {
		f := newNameFilter(n)
		for i := range n {
			f.add(Hash(fmt.Sprintf("pkg/C%d", i)))
		}
		for i := range n {
			if !f.has(Hash(fmt.Sprintf("pkg/C%d", i))) {
				t.Fatalf("n=%d: filter lost pkg/C%d", n, i)
			}
		}
		// Mostly absent names are ruled out: the point of the filter.
		maybe := 0
		for i := range 10000 {
			if f.has(Hash(fmt.Sprintf("other/D%d", i))) {
				maybe++
			}
		}
		if maybe > 500 {
			t.Errorf("n=%d: %d of 10000 absent names may be there", n, maybe)
		}
	}
}

func TestMayHave(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "demo"), 0o755)
	for _, n := range []string{"A", "A$In"} {
		c := classfile.NewClass(classfile.AccPublic|classfile.AccSuper, "demo/"+n, "java/lang/Object")
		os.WriteFile(filepath.Join(dir, "demo", n+".class"), c.Bytes(), 0o644)
	}
	p, err := Open(jdk(t), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, name := range []string{"demo/A", "demo/A$In", "java/lang/String", "java/util/Map$Entry"} {
		if !p.MayHave(Hash(name)) {
			t.Errorf("MayHave(%s) = false", name)
		}
	}
	// A source added before the filter is built, and one added after.
	p.AddSource("src/B", func() (*classfile.Info, error) { return &classfile.Info{Name: "src/B"}, nil })
	o := p.Overlay()
	o.AddSource("src/C", func() (*classfile.Info, error) { return &classfile.Info{Name: "src/C"}, nil })
	if !o.MayHave(Hash("src/C")) || !o.MayHave(Hash("src/B")) || !o.MayHave(Hash("demo/A")) {
		t.Error("overlay filter misses a class")
	}
	o.AddSource("src/D", func() (*classfile.Info, error) { return &classfile.Info{Name: "src/D"}, nil })
	if !o.MayHave(Hash("src/D")) {
		t.Error("filter misses a source added after it was built")
	}
	for name := range p.classes {
		if !p.MayHave(Hash(name)) {
			t.Fatalf("MayHave(%s) = false", name)
		}
	}
}
