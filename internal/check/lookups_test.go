package check

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

type lookupTestRecorder struct {
	testRecorder
	mu      sync.Mutex
	lookups map[string][]string // by file: "scope|kind|name"
}

func (r *lookupTestRecorder) Lookup(f *syntax.File, scope string, kind LookupKind, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := "P"
	if kind == ClassLookup {
		k = "C"
	}
	r.lookups[f.Path] = append(r.lookups[f.Path], scope+"|"+k+"|"+name)
}

func TestLookups(t *testing.T) {
	srcs := map[string]string{
		"a.kt": `package a

open class Base { fun inherited() {} }

class A : Base() {
    fun m(): Res = Res()
    companion object { fun make() = A() }
    class Nested
}

class Res

fun top(): Int = 1

typealias Alias = Res
`,
		"b.kt": `package b

import a.A
import a.*

class B : a.Base() {
    val r: Alias? = null
    fun use(local: Int): Int {
        val v = A.make()
        v.m()
        v.inherited()
        val n: A.Nested? = null
        return top() + local
    }
}
`,
	}
	var files []*syntax.File
	for _, name := range []string{"a.kt", "b.kt"} {
		f, errs := syntax.Parse(name, srcs[name])
		if len(errs) > 0 {
			t.Fatal(errs[0])
		}
		files = append(files, f)
	}
	rec := &lookupTestRecorder{testRecorder: testRecorder{whens: map[*syntax.When][2]bool{}, used: map[syntax.Stmt]bool{}},
		lookups: map[string][]string{}}
	cerrs, err := CheckContext(context.Background(), files, NewLibrary(nil), 2, rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(cerrs) > 0 {
		t.Fatal(cerrs[0])
	}
	got := rec.lookups["b.kt"]
	for _, want := range []string{
		"a|P|A",      // import a.A
		"a|P|Base",   // the supertype
		"a|P|Alias",  // a typealias by the star import
		"a.A|C|make", // A.make(): the companion's member, by its class
		"a.A.Companion|C|make",
		"a.A|C|m",   // v.m()
		"a.Res|C|m", // no: see below
	} {
		if want == "a.Res|C|m" {
			continue
		}
		if !slices.Contains(got, want) {
			t.Errorf("b.kt: no lookup %s in\n%s", want, strings.Join(got, "\n"))
		}
	}
	for _, want := range []string{
		"a.A|C|inherited",    // v.inherited(), in the receiver's class
		"a.Base|C|inherited", // and in the class that declares it
		"a.A|C|Nested",       // A.Nested
		"a|P|top",            // top() by the star import
		"b|P|top",            // and in the file's package, searched first
		"a|P|Res",            // the result type of m()
	} {
		if !slices.Contains(got, want) {
			t.Errorf("b.kt: no lookup %s in\n%s", want, strings.Join(got, "\n"))
		}
	}
	for _, l := range got {
		if strings.HasSuffix(l, "|local") || strings.HasSuffix(l, "|v") || strings.HasSuffix(l, "|n") {
			t.Errorf("b.kt: a local looked up: %s", l)
		}
	}
	seen := map[string]bool{}
	for _, l := range got {
		if seen[l] {
			t.Errorf("b.kt: %s reported twice", l)
		}
		seen[l] = true
	}
}
