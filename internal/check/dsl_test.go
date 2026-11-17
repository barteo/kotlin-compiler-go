package check

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// dslMarkerSrc has receivers marked by their class, A, and by their
// type, @M B, so a use meets the markers of both kinds.
const dslMarkerSrc = `
@DslMarker
@Target(AnnotationTarget.CLASS, AnnotationTarget.TYPE)
annotation class M

@M
class A {
    fun a() {}
}

class B {
    fun b() {}
}

fun outer(block: A.() -> Unit) = A().block()
fun inner(block: (@M B).() -> Unit) = B().block()
fun marked(block: (@M A).() -> Unit) = A().block()

fun f() {
    outer {
        inner {
            a()
            b()
            this@outer.a()
        }
    }
    inner {
        marked {
            b()
            a()
        }
    }
}
`

// checkDslMarkers checks dslMarkerSrc with classes of its own and
// returns the lines of its errors.
func checkDslMarkers(t *testing.T, cp *classpath.Path) []string {
	t.Helper()
	f, errs := syntax.Parse("t.kt", dslMarkerSrc)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	ds, err := CheckOptions(context.Background(), []*syntax.File{f}, NewLibrary(cp), 1, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range ds {
		got = append(got, fmt.Sprintf("%d:%s", d.Pos.Line, d.Msg))
	}
	slices.Sort(got)
	return got
}

// A marker class is another symbol in each check of a process: the
// markers of a type are those of the check that reads them.
func TestDslMarkersAcrossChecks(t *testing.T) {
	lib, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open("", []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	want := []string{
		"22:'a' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary",
		"29:'b' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary",
	}
	for i := range 3 {
		if got := checkDslMarkers(t, cp); !slices.Equal(got, want) {
			t.Errorf("check %d: got %q, want %q", i, got, want)
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := checkDslMarkers(t, cp); !slices.Equal(got, want) {
				t.Errorf("concurrent check: got %q, want %q", got, want)
			}
		})
	}
	wg.Wait()
}
