package check

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// scopeRec is a Recorder that keeps the scope at the probe.
type scopeRec struct {
	testRecorder
	view *ScopeView
}

func (r *scopeRec) Scope(v *ScopeView) { r.view = v }

var (
	stdlibOnce sync.Once
	stdlibLib  *Library
)

// stdlibLibrary returns a Library of the standard library, shared by the
// tests, or skips the test.
func stdlibLibrary(t *testing.T) *Library {
	t.Helper()
	stdlibOnce.Do(func() {
		stdlib, err := classpath.FindStdlib()
		if err != nil {
			return
		}
		jdk, err := classpath.FindJDK()
		if err != nil {
			return
		}
		cp, err := classpath.Open(jdk, []string{stdlib})
		if err != nil {
			return
		}
		stdlibLib = NewLibrary(cp)
	})
	if stdlibLib == nil {
		t.Skip("no standard library")
	}
	return stdlibLib
}

// probeView checks src, whose | is the probe, and returns the scope there.
func probeView(t *testing.T, src string) *ScopeView {
	t.Helper()
	src = strings.Replace(src, "|", ProbeName, 1)
	f, errs := syntax.Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	rec := &scopeRec{testRecorder: testRecorder{whens: map[*syntax.When][2]bool{}, used: map[syntax.Stmt]bool{}}}
	_, err := CheckContext(context.Background(), []*syntax.File{f}, stdlibLibrary(t), 1, rec)
	if !errors.Is(err, ErrProbed) {
		t.Fatalf("err = %v, want ErrProbed", err)
	}
	if rec.view == nil {
		t.Fatal("no scope recorded")
	}
	return rec.view
}

// itemNames returns the names of v's items of origin o, in order,
// without repeats.
func itemNames(v *ScopeView, o Origin) []string {
	var out []string
	for _, it := range v.Items {
		if it.Origin == o && !slices.Contains(out, it.Name) {
			out = append(out, it.Name)
		}
	}
	return out
}

// has reports whether v has an item named name of origin o.
func has(v *ScopeView, o Origin, name string) bool {
	return slices.Contains(itemNames(v, o), name)
}

func TestProbeLocals(t *testing.T) {
	v := probeView(t, `
fun f(a: Int) {
    val b = 1
    run {
        val a = "s"
        |
    }
    val c = 2
}
`)
	if got := itemNames(v, Local); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("locals = %v, want [a b]: innermost first, a once, not c", got)
	}
	if v.Items[0].Type != types.StringType {
		t.Errorf("a: %v, want the inner a's String", v.Items[0].Type)
	}
	if !has(v, TopLevel, "println") || !has(v, TopLevel, "f") || !has(v, Classifier, "String") {
		t.Errorf("missing top-level println, f or class String: %v", itemNames(v, TopLevel))
	}
}

func TestProbeMembers(t *testing.T) {
	v := probeView(t, `
class A(val x: Int) { fun f() = 1 }
fun g(a: A?) {
    if (a != null) {
        a.|
    }
}
`)
	if v.Receiver == nil || v.Receiver.String() != "A" {
		t.Fatalf("receiver = %v, want the smart-cast A", v.Receiver)
	}
	for _, name := range []string{"x", "f", "toString"} {
		if !has(v, Member, name) {
			t.Errorf("no member %s: %v", name, itemNames(v, Member))
		}
	}
	for _, name := range []string{"getClass", "notify", "wait"} {
		if has(v, Member, name) {
			t.Errorf("member %s, which Kotlin's Any hasn't", name)
		}
	}
	if !has(v, Extension, "let") || !has(v, Extension, "apply") {
		t.Errorf("no let or apply: %v", itemNames(v, Extension))
	}
	for _, name := range []string{"ifBlank", "coerceAtLeast", "use", "hashCode"} {
		if has(v, Extension, name) {
			t.Errorf("extension %s, which doesn't take an A, or a member hides", name)
		}
	}
}

func TestProbeBuiltinMembers(t *testing.T) {
	v := probeView(t, `
fun g(s: String?) {
    s?.|
}
`)
	if !v.Safe || !has(v, Member, "length") || !has(v, Member, "hashCode") || has(v, Member, "charAt") {
		t.Errorf("safe %v, members %v: want length and hashCode, Kotlin's, not charAt", v.Safe, itemNames(v, Member))
	}
	if !has(v, Extension, "trim") || !has(v, Extension, "coerceAtLeast") {
		t.Errorf("no extensions trim or coerceAtLeast for a String")
	}
}

func TestProbeReceivers(t *testing.T) {
	v := probeView(t, `
class Outer {
    val o = 1
    fun f() = buildString {
        |
    }
}
`)
	if len(v.Receivers) < 2 || v.Receivers[0].String() != "StringBuilder" || v.Receivers[1].String() != "Outer" {
		t.Fatalf("receivers = %v, want StringBuilder, then Outer", v.Receivers)
	}
	var appendRecv, oRecv = -2, -2
	for _, it := range v.Items {
		switch it.Name {
		case "append":
			appendRecv = it.Receiver
		case "o":
			oRecv = it.Receiver
		}
	}
	if appendRecv != 0 || oRecv != 1 {
		t.Errorf("append of receiver %d, o of %d: want 0 and 1", appendRecv, oRecv)
	}
}

func TestProbeExpected(t *testing.T) {
	v := probeView(t, `
fun g() {
    val s: String = |
}
`)
	if v.Expected != types.StringType {
		t.Errorf("expected %v, want String", v.Expected)
	}
}

func TestProbeCalls(t *testing.T) {
	v := probeView(t, `
class A {
    fun f(n: Int, label: String = "") = 1
    fun f(b: Boolean) = 2
    fun h(list: List<String>) {
        list.forEach { f(1, label = |) }
    }
}
`)
	if len(v.Calls) != 2 {
		t.Fatalf("calls = %+v, want f's and forEach's", v.Calls)
	}
	f := v.Calls[0]
	if f.Arg != 1 || f.Name != "label" || !slices.Equal(f.Names, []string{"label"}) || len(f.Funs) != 2 {
		t.Errorf("f(1, label = probe): %+v", f)
	}
	if v.Expected != types.StringType {
		t.Errorf("expected %v, want label's String", v.Expected)
	}
	if fe := v.Calls[1]; fe.Arg != 0 || len(fe.Funs) == 0 || fe.Funs[0].Name != "forEach" {
		t.Errorf("forEach { }: %+v", fe)
	}
}

func TestProbeType(t *testing.T) {
	v := probeView(t, `
class Box<T> {
    fun <R> g(): R {
        val x: |
        TODO()
    }
}
`)
	if !v.TypePosition {
		t.Error("not a type position")
	}
	for _, name := range []string{"Box", "String", "List", "Int"} {
		if !has(v, Classifier, name) {
			t.Errorf("no class %s", name)
		}
	}
	if got := itemNames(v, TypeParameter); !slices.Equal(got, []string{"R", "T"}) {
		t.Errorf("type parameters = %v, want [R T]", got)
	}
	if has(v, TopLevel, "println") || has(v, Classifier, "CollectionsKt") {
		t.Error("a function or a file facade in a type position")
	}
	if !has(v, PackageName, "kotlin") || !has(v, PackageName, "java") {
		t.Errorf("packages = %v", itemNames(v, PackageName))
	}
}

func TestProbeQualified(t *testing.T) {
	v := probeView(t, `
enum class E { A, B; companion object { fun of() = A } }
fun g() {
    E.|
}
`)
	if v.StaticOf == nil || !has(v, Static, "A") || !has(v, Static, "B") || !has(v, Member, "of") {
		t.Errorf("E.: static of %v, items %v %v", v.StaticOf, itemNames(v, Static), itemNames(v, Member))
	}
	v = probeView(t, `
fun g() {
    kotlin.collections.|
}
`)
	if v.Package != "kotlin.collections" || !has(v, Static, "listOf") || !has(v, Static, "ArrayList") || !has(v, PackageName, "builders") {
		t.Errorf("kotlin.collections.: package %q, items %v", v.Package, itemNames(v, Static))
	}
	v = probeView(t, `
class A { fun f() = 1 }
fun g() = listOf(A()).map(A::|)
`)
	if !v.Callable || !has(v, Member, "f") {
		t.Errorf("A::: callable %v, members %v", v.Callable, itemNames(v, Member))
	}
}

func TestImportables(t *testing.T) {
	start := time.Now()
	all := stdlibLibrary(t).Importables()
	t.Logf("%d importables in %v", len(all), time.Since(start))
	find := func(name, pkg string, class bool) bool {
		return slices.Contains(all, Importable{Name: name, Package: pkg, Class: class})
	}
	for _, want := range []Importable{
		{"sqrt", "kotlin.math", false},
		{"PI", "kotlin.math", false},
		{"Random", "kotlin.random", true},
		{"ArrayList", "java.util", true},
		{"Duration", "kotlin.time", true},
	} {
		if !find(want.Name, want.Package, want.Class) {
			t.Errorf("no %+v", want)
		}
	}
	for _, it := range all {
		switch {
		case it.Package == "kotlin" || it.Package == "kotlin.collections" || it.Package == "java.lang":
			t.Errorf("%+v, of a default import", it)
		case strings.HasSuffix(it.Name, "Kt") && it.Class:
			t.Errorf("%+v, a file facade", it)
		case strings.HasPrefix(it.Package, "sun."):
			t.Errorf("%+v, of the JDK's internals", it)
		}
	}
	if !slices.IsSortedFunc(all, func(a, b Importable) int { return strings.Compare(a.Name, b.Name) }) {
		t.Error("not sorted by name")
	}
}
