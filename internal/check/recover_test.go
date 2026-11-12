package check

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestErrorExpr checks that the ErrorExprs of a recovered lowering are
// checked without errors, theirs or their uses'.
func TestErrorExpr(t *testing.T) {
	src := `
fun f(): Int {
    val x = 1
    foo(1 2)
    val y = )
    return x + y
}

fun g() = 1 + )

fun h(): Int = g() * 2

fun i(s: String): Int {
    return s.lengt +
}
`
	f, _, lerrs := lower.Recover(psi.Parse("t.kt", src))
	if len(lerrs) == 0 {
		t.Fatal("no lowering error")
	}
	errs, err := CheckContext(context.Background(), []*syntax.File{f}, NewLibrary(nil), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) > 0 {
		t.Errorf("errors %v, want none", errs)
	}
}

// cancelRecorder cancels the check once it has seen n statements.
type cancelRecorder struct {
	testRecorder
	n      int
	seen   int
	cancel func()
}

func (r *cancelRecorder) Stmt(s syntax.Stmt, valueUsed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seen++; r.seen == r.n {
		r.cancel()
	}
}

// TestCancelInBody checks that a check cancelled in the middle of a body,
// one inferred up front or checked in parallel, stops there.
func TestCancelInBody(t *testing.T) {
	const n = 2000
	var b strings.Builder
	b.WriteString("fun inferred() = if (true) {\n")
	for i := range n {
		fmt.Fprintf(&b, "    val a%d = %d + 1\n", i, i)
	}
	b.WriteString("    1\n} else 2\n\nfun checked(): Int {\n")
	for i := range n {
		fmt.Fprintf(&b, "    val b%d = %d + 1\n", i, i)
	}
	b.WriteString("    return 1\n}\n")
	for _, at := range []int{10, n + 10} {
		f, errs := syntax.Parse("t.kt", b.String())
		if len(errs) > 0 {
			t.Fatal(errs[0])
		}
		ctx, cancel := context.WithCancel(context.Background())
		rec := &cancelRecorder{n: at, cancel: cancel}
		if _, err := CheckContext(ctx, []*syntax.File{f}, NewLibrary(nil), 1, rec); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled at %d: err = %v, want context.Canceled", at, err)
		}
		if rec.seen > at+1 {
			t.Errorf("cancelled at statement %d: %d statements checked", at, rec.seen)
		}
	}
}
