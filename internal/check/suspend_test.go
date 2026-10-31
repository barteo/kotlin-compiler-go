package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// TestStdlibSuspend checks that the standard library's suspend functions
// and suspend function types resolve, with their JVM descriptors.
func TestStdlibSuspend(t *testing.T) {
	lib, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	jdk, err := classpath.FindJDK()
	if err != nil {
		t.Skip(err)
	}
	entries := []string{lib}
	src := `
import kotlin.coroutines.*

fun seq() = sequence<Int> { yield(1) }

fun start(block: suspend () -> Int, c: Continuation<Int>) { block.startCoroutine(c) }
`
	cp, err := classpath.Open(jdk, entries)
	if err != nil {
		t.Fatal(err)
	}
	f, errs := syntax.Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	if errs := Check([]*syntax.File{f}, cp, 1); len(errs) > 0 {
		t.Fatal(errs[0])
	}
	calls := map[string]*syntax.Call{}
	for _, d := range f.Decls {
		fd := d.(*syntax.FunDecl)
		visit := func(n any) bool {
			if x, ok := n.(*syntax.Call); ok && x.Sym != nil {
				calls[x.Sym.Name] = x
			}
			return true
		}
		syntax.Inspect(fd.Body, visit)
		syntax.Inspect(fd.ExprBody, visit)
	}
	tests := []struct {
		name, desc, param string
		suspend           bool
	}{
		{"sequence", "(Lkotlin/jvm/functions/Function2;)Lkotlin/sequences/Sequence;", "suspend SequenceScope<T>.() -> Unit", false},
		{"yield", "(Ljava/lang/Object;Lkotlin/coroutines/Continuation;)Ljava/lang/Object;", "T", true},
		{"startCoroutine", "(Lkotlin/jvm/functions/Function1;Lkotlin/coroutines/Continuation;)V", "Continuation<T>", false},
	}
	for _, tt := range tests {
		x := calls[tt.name]
		if x == nil {
			t.Errorf("no call of %s", tt.name)
			continue
		}
		f := x.Sym
		if d := types.FunDesc(f); d != tt.desc || f.Suspend != tt.suspend || f.Unsupported {
			t.Errorf("%s: desc %s suspend %v unsupported %v", tt.name, d, f.Suspend, f.Unsupported)
		}
		if p := f.Params[0].Type.String(); p != tt.param {
			t.Errorf("%s: parameter type %s, want %s", tt.name, p, tt.param)
		}
	}
	if x := calls["startCoroutine"]; x != nil {
		if r := x.Sym.Recv; !r.IsSuspendFunc() || r.String() != "suspend () -> T" {
			t.Errorf("startCoroutine: receiver %s", r)
		}
	}
}

// TestKotlinxSuspend checks that kotlinx.coroutines' suspend functions
// resolve, if its jar is next to the standard library's.
func TestKotlinxSuspend(t *testing.T) {
	lib, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	kx := filepath.Join(filepath.Dir(lib), "kotlinx-coroutines-core-jvm.jar")
	if _, err := os.Stat(kx); err != nil {
		t.Skip(err)
	}
	jdk, err := classpath.FindJDK()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open(jdk, []string{lib, kx})
	if err != nil {
		t.Fatal(err)
	}
	f, errs := syntax.Parse("t.kt", "import kotlinx.coroutines.*\nsuspend fun f() = delay(1)\n")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	if errs := Check([]*syntax.File{f}, cp, 1); len(errs) > 0 {
		t.Fatal(errs[0])
	}
	x := f.Decls[0].(*syntax.FunDecl).ExprBody.(*syntax.Call)
	if d := types.FunDesc(x.Sym); !x.Sym.Suspend || d != "(JLkotlin/coroutines/Continuation;)Ljava/lang/Object;" {
		t.Errorf("delay: suspend %v desc %s", x.Sym.Suspend, d)
	}
}
