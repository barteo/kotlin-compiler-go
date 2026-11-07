package check

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

type testRecorder struct {
	mu          sync.Mutex
	whens       map[*syntax.When][2]bool
	used        map[syntax.Stmt]bool
	unreachable []syntax.Stmt
}

func (r *testRecorder) When(x *syntax.When, covered, valueUsed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.whens[x] = [2]bool{covered, valueUsed}
}

func (r *testRecorder) Stmt(s syntax.Stmt, valueUsed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.used[s] = valueUsed
}

func (r *testRecorder) Unreachable(s syntax.Stmt) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unreachable = append(r.unreachable, s)
}

func (r *testRecorder) Annotation(*syntax.File, *syntax.Annotation, *types.ClassSym) {}

func (r *testRecorder) Elvis(*syntax.Binary, bool) {}

func (r *testRecorder) TypeRef(*syntax.TypeRef, types.Type) {}

func (r *testRecorder) StmtType(syntax.Expr, types.Type) {}

func TestRecorder(t *testing.T) {
	src := `
enum class E { A, B }

fun g(i: Int) {}

fun f(e: E, n: Int): Int {
    when (n) {
        1 -> g(1)
    }
    val x = when (e) {
        E.A -> 1
        E.B -> 2
        else -> 3
    }
    return x
    g(x)
}
`
	f, errs := syntax.Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	rec := &testRecorder{whens: map[*syntax.When][2]bool{}, used: map[syntax.Stmt]bool{}}
	cerrs, err := CheckContext(context.Background(), []*syntax.File{f}, NewLibrary(nil), 1, rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(cerrs) > 0 {
		t.Fatal(cerrs[0])
	}
	body := f.Decls[2].(*syntax.FunDecl).Body.Stmts
	stmtWhen := body[0].(*syntax.ExprStmt).X.(*syntax.When)
	exprWhen := body[1].(*syntax.VarStmt).Init.(*syntax.When)
	if got := rec.whens[stmtWhen]; got != [2]bool{false, false} {
		t.Errorf("when (n): covered, used = %v, want false, false", got)
	}
	if got := rec.whens[exprWhen]; got != [2]bool{true, true} {
		t.Errorf("when (e): covered without else, used = %v, want true, true", got)
	}
	if len(rec.unreachable) != 1 || rec.unreachable[0] != body[3] {
		t.Errorf("unreachable = %v, want the g(x) after return", rec.unreachable)
	}
	if used, ok := rec.used[body[0]]; !ok || used {
		t.Errorf("first statement: used %v, reported %v", used, ok)
	}
}

func TestCheckContextCancelled(t *testing.T) {
	f, errs := syntax.Parse("t.kt", "fun f() = 1\n")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CheckContext(ctx, []*syntax.File{f}, NewLibrary(nil), 1, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
