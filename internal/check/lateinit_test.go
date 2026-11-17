package check

import (
	"context"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestIsInitializedValue checks that ::x.isInitialized of a top-level
// lateinit property, which has no receiver once checked, may be a local
// variable's value: a value a val is narrowed by.
func TestIsInitializedValue(t *testing.T) {
	src := `package p
private lateinit var top: String
class A {
    lateinit var open: String
    fun f(): Boolean {
        val a = ::top.isInitialized
        val b = ::open.isInitialized
        val c = this::open.isInitialized
        return a && b && c
    }
}
`
	f, _, lerrs := lower.Recover(psi.Parse("t.kt", src))
	if len(lerrs) > 0 {
		t.Fatal(lerrs)
	}
	errs, err := CheckContext(context.Background(), []*syntax.File{f}, NewLibrary(nil), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range errs {
		t.Error(e)
	}
}
