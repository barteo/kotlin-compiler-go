package analysis

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// analyzed analyzes src as a.kt and fails t on an error or a diagnostic.
func analyzed(t *testing.T, cfg Config, src string) (*Result, *psi.Snapshot) {
	t.Helper()
	s := psi.Parse("a.kt", src)
	r, err := AnalyzeArena(context.Background(), cfg, []*psi.Snapshot{s})
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Diagnostics(); len(d) > 0 {
		t.Fatalf("diagnostics: %+v", d)
	}
	return r, s
}

// withStdlib returns a configuration with the standard library, or skips
// t.
func withStdlib(t *testing.T) Config {
	t.Helper()
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skipf("no kotlin-stdlib.jar (set KOTLIN_HOME): %v", err)
	}
	return Config{NoJDK: true, ClassPath: []string{jar}}
}

// at returns the node of kind whose text is text, the nth (from 0).
func at(t *testing.T, s *psi.Snapshot, kind, text string, nth int) psi.NodeRef {
	t.Helper()
	var found psi.NodeRef
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if found.Valid() {
			return false
		}
		if n.Kind() == kind && n.Text() == text {
			if nth == 0 {
				found = n
				return false
			}
			nth--
		}
		return true
	})
	if !found.Valid() {
		t.Fatalf("no %s %q", kind, text)
	}
	return found
}

func fact(t *testing.T, r *Result, n psi.NodeRef) Fact {
	t.Helper()
	f, ok := r.Fact(ElementID{n.ContainingFile().ID(), n.ID()})
	if !ok {
		t.Fatalf("no fact for %s %q", n.Kind(), n.Text())
	}
	return f
}

func symbol(t *testing.T, r *Result, id uint32) Symbol {
	t.Helper()
	s, ok := r.Symbol(id)
	if !ok {
		t.Fatalf("no symbol %d", id)
	}
	return s
}

func TestDemo(t *testing.T) {
	r, s := analyzed(t, Config{NoJDK: true}, "class Demo\nfun use(x: Demo): Demo = x\n")
	fun := fact(t, r, at(t, s, "FUN", "fun use(x: Demo): Demo = x", 0))
	fs := symbol(t, r, fun.Symbol)
	if fs.Kind != "function" || fs.QualifiedName != "use" || fs.Type.Name != "Demo" || fs.Declaration == nil || fs.Declaration.Node != fun.Element.Node {
		t.Errorf("use: %+v", fs)
	}
	param := fact(t, r, at(t, s, "VALUE_PARAMETER", "x: Demo", 0))
	ref := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "x", 0))
	if ref.Symbol == 0 || ref.Symbol != param.Symbol || ref.Role != "reference" {
		t.Errorf("x refers to %d, the parameter is %d", ref.Symbol, param.Symbol)
	}
	if ref.Type == nil || ref.Type.Name != "Demo" || !ref.UsedAsExpression {
		t.Errorf("x: %+v", ref)
	}
	if ps := symbol(t, r, param.Symbol); ps.Kind != "parameter" || ps.Name != "x" {
		t.Errorf("parameter: %+v", ps)
	}
	cls := symbol(t, r, fact(t, r, at(t, s, "CLASS", "class Demo", 0)).Symbol)
	if cls.Kind != "class" || cls.Name != "Demo" || !slices.Contains(cls.Supertypes, "kotlin.Any") {
		t.Errorf("Demo: %+v", cls)
	}
	tr := fact(t, r, at(t, s, "TYPE_REFERENCE", "Demo", 1))
	if tr.Type == nil || tr.Type.Name != "Demo" {
		t.Errorf("return type: %+v", tr.Type)
	}
	d := r.Data()
	if len(d.Facts) == 0 || len(d.Symbols) == 0 || !slices.Contains(d.Capabilities, CapCalls) {
		t.Errorf("data: %d facts, %d symbols, capabilities %v", len(d.Facts), len(d.Symbols), d.Capabilities)
	}
	for i := 1; i < len(d.Facts); i++ {
		if d.Facts[i-1].Element.Node >= d.Facts[i].Element.Node {
			t.Fatalf("facts out of order at %d", i)
		}
	}
	for i, sym := range d.Symbols {
		if sym.ID != uint32(i+1) {
			t.Fatalf("symbol %d has ID %d", i, sym.ID)
		}
	}
}

func TestCalls(t *testing.T) {
	src := `package p

fun g(a: Int, b: String = "") = a

fun f(l: List<Int>) {
    val m = l.map { it * 2 }
    check(m.size > 1)
    g(b = "x", a = 1)
    println(m)
}
`
	r, s := analyzed(t, withStdlib(t), src)
	mapCall := fact(t, r, at(t, s, "CALL_EXPRESSION", "map { it * 2 }", 0))
	if c := mapCall.Call; c == nil || c.Name != "kotlin.collections.map" || !c.Extension || !c.Inline ||
		c.DeclaredReceiver == nil || c.DeclaredReceiver.Name != "kotlin.collections.Iterable" ||
		c.Receiver == nil || c.Receiver.Name != "kotlin.collections.List" || c.TypeParameters != 2 {
		t.Errorf("map: %+v", c)
	}
	qualified := fact(t, r, at(t, s, "DOT_QUALIFIED_EXPRESSION", "l.map { it * 2 }", 0))
	if qualified.Call == nil || qualified.Call.Name != "kotlin.collections.map" {
		t.Errorf("l.map: %+v", qualified.Call)
	}
	callee := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "map", 0))
	if callee.Call == nil || callee.Symbol == 0 || symbol(t, r, callee.Symbol).QualifiedName != "kotlin.collections.map" {
		t.Errorf("map callee: %+v", callee)
	}
	lambda := fact(t, r, at(t, s, "LAMBDA_EXPRESSION", "{ it * 2 }", 0))
	it := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "it", 0))
	if !lambda.ImplicitParameter || len(lambda.LambdaParameters) != 1 || lambda.LambdaParameters[0] != it.Symbol {
		t.Errorf("lambda %+v, it %d", lambda, it.Symbol)
	}
	if f := fact(t, r, at(t, s, "CALL_EXPRESSION", "check(m.size > 1)", 0)); f.Builtin != "kotlin.check" {
		t.Errorf("check: builtin %q", f.Builtin)
	}
	g := fact(t, r, at(t, s, "CALL_EXPRESSION", `g(b = "x", a = 1)`, 0))
	if c := g.Call; c == nil || c.Name != "p.g" || !slices.Equal(c.ArgumentMapping, []int{1, 0}) ||
		!slices.Equal(c.ParameterNames, []string{"a", "b"}) || c.Declaration == nil || g.Builtin != "" {
		t.Errorf("g: %+v", c)
	}
	if c := g.Call; c != nil && c.Declaration != nil {
		decl := fact(t, r, at(t, s, "FUN", `fun g(a: Int, b: String = "") = a`, 0))
		if *c.Declaration != decl.Element {
			t.Errorf("g declared at %v, want %v", *c.Declaration, decl.Element)
		}
	}
	println := fact(t, r, at(t, s, "CALL_EXPRESSION", "println(m)", 0))
	if println.UsedAsExpression || println.Call == nil || println.Call.Name != "kotlin.io.println" {
		t.Errorf("println(m): %+v", println)
	}
}

func TestWhen(t *testing.T) {
	src := `package p

enum class E { A, B, C }
sealed interface S
class X : S
object Y : S

fun f(e: E, s: S, b: Boolean) {
    when (e) {
        E.A -> {}
        else -> {}
    }
    when (e) {
        E.A, E.B, E.C -> {}
        else -> {}
    }
    when (s) {
        is X -> {}
        else -> {}
    }
    val v = when (b) {
        true -> 1
        false -> 2
    }
}
`
	r, s := analyzed(t, withStdlib(t), src)
	tests := []struct {
		text       string
		exhaustive bool
		category   string
		missing    []string
		used       bool
	}{
		{"when (e) {\n        E.A -> {}\n        else -> {}\n    }", false, "enum class", []string{"B", "C"}, false},
		{"when (e) {\n        E.A, E.B, E.C -> {}\n        else -> {}\n    }", true, "enum class", nil, false},
		{"when (s) {\n        is X -> {}\n        else -> {}\n    }", false, "sealed class", []string{"Y"}, false},
		{"when (b) {\n        true -> 1\n        false -> 2\n    }", true, "boolean", nil, true},
	}
	for _, tt := range tests {
		f := fact(t, r, at(t, s, "WHEN", tt.text, 0))
		w := f.When
		if w == nil || w.Exhaustive != tt.exhaustive || w.Category != tt.category || !slices.Equal(w.Missing, tt.missing) || f.UsedAsExpression != tt.used {
			t.Errorf("%s: %+v, used %v", strings.SplitN(tt.text, "\n", 2)[0], w, f.UsedAsExpression)
		}
	}
}

func TestFlow(t *testing.T) {
	src := `package p

class C {
    fun m() = this
}

fun f(n: String?): Int {
    with(C()) { this.m() }
    listOf(1).forEach { if (it > 0) return 1 }
    run lit@{ return@lit }
    if (n != null) return n.length
    return 0
    println()
}
`
	r, s := analyzed(t, withStdlib(t), src)
	n := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "n", 1))
	if !n.FlowNonNull || !n.DefinitelyNonNull || n.Type == nil || !n.Type.Nullable || n.Type.Name != "kotlin.String" {
		t.Errorf("n after the null check: %+v", n)
	}
	if f := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "n", 0)); f.FlowNonNull {
		t.Errorf("n before the null check: %+v", f)
	}
	dead := fact(t, r, at(t, s, "CALL_EXPRESSION", "println()", 0))
	if !dead.Unreachable {
		t.Errorf("println() after return: %+v", dead)
	}
	ret := fact(t, r, at(t, s, "RETURN", "return 1", 0))
	f := at(t, s, "FUN", s.Source()[strings.Index(s.Source(), "fun f"):strings.LastIndex(s.Source(), "}")+1], 0)
	if ret.ReturnTarget == nil || ret.ReturnTarget.Node != f.ID() {
		t.Errorf("return 1 leaves %v, want f %d", ret.ReturnTarget, f.ID())
	}
	lit := fact(t, r, at(t, s, "RETURN", "return@lit", 0))
	lam := at(t, s, "LAMBDA_EXPRESSION", "{ return@lit }", 0)
	if lit.ReturnTarget == nil || lit.ReturnTarget.Node != lam.ID() {
		t.Errorf("return@lit leaves %v, want the lambda %d", lit.ReturnTarget, lam.ID())
	}
	this := fact(t, r, at(t, s, "THIS_EXPRESSION", "this", 1))
	withLambda := at(t, s, "LAMBDA_EXPRESSION", "{ this.m() }", 0)
	if this.ReceiverTarget == nil || this.ReceiverTarget.Node != withLambda.ID() {
		t.Errorf("this in with's lambda: %v, want %d", this.ReceiverTarget, withLambda.ID())
	}
	classThis := fact(t, r, at(t, s, "THIS_EXPRESSION", "this", 0))
	cls := at(t, s, "CLASS", "class C {\n    fun m() = this\n}", 0)
	if classThis.ReceiverTarget == nil || classThis.ReceiverTarget.Node != cls.ID() {
		t.Errorf("this in C: %v, want %d", classThis.ReceiverTarget, cls.ID())
	}
}

func TestDelegates(t *testing.T) {
	src := `package p

class C {
    val a by lazy { 1 }
}

fun f(): Int {
    val b by lazy { 2 }
    return b + C().a
}
`
	r, s := analyzed(t, withStdlib(t), src)
	for _, text := range []string{"val a by lazy { 1 }", "val b by lazy { 2 }"} {
		f := fact(t, r, at(t, s, "PROPERTY", text, 0))
		if f.DelegateType == nil || f.DelegateType.Name != "kotlin.Lazy" || len(f.DelegatedCalls) != 1 ||
			!strings.HasSuffix(f.DelegatedCalls[0].Name, ".getValue") {
			t.Errorf("%s: delegate %+v, calls %+v", text, f.DelegateType, f.DelegatedCalls)
		}
		if sym := symbol(t, r, f.Symbol); sym.Type.Name != "kotlin.Int" {
			t.Errorf("%s: %+v", text, sym)
		}
	}
}

func TestDiagnostics(t *testing.T) {
	s := psi.Parse("a.kt", "fun f() = undefined\n")
	r, err := AnalyzeArena(context.Background(), Config{NoJDK: true}, []*psi.Snapshot{s})
	if err != nil {
		t.Fatal(err)
	}
	d := r.Diagnostics()
	if len(d) != 1 || d[0].Path != "a.kt" || d[0].Line != 1 || d[0].Column != 11 || d[0].Code != "source_error" {
		t.Errorf("diagnostics: %+v", d)
	}
	if data := r.Data(); len(data.Facts) != 0 || len(data.Symbols) != 0 {
		t.Errorf("facts reported with diagnostics: %d", len(data.Facts))
	}
}

func TestDuplicateFacades(t *testing.T) {
	a := psi.Parse("x/a.kt", "fun f() = 1\n")
	b := psi.Parse("y/a.kt", "fun g() = 2\n")
	r, err := AnalyzeArena(context.Background(), Config{NoJDK: true}, []*psi.Snapshot{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Diagnostics(); len(d) != 1 || !strings.Contains(d[0].Message, "duplicate JVM class name AKt") {
		t.Errorf("diagnostics: %+v", d)
	}
}

func TestErrors(t *testing.T) {
	s := psi.Parse("a.kt", "class A\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AnalyzeArena(ctx, Config{NoJDK: true}, []*psi.Snapshot{s}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
	if _, err := Analyze(context.Background(), Config{NoJDK: true, ExplicitAPIMode: "loud"}, []*psi.Snapshot{s}); err == nil {
		t.Error("invalid explicit API mode accepted")
	}
	for _, mode := range []string{"", "disable", "warning", "strict"} {
		if _, err := Analyze(context.Background(), Config{NoJDK: true, ExplicitAPIMode: mode}, []*psi.Snapshot{s}); err != nil {
			t.Errorf("mode %q: %v", mode, err)
		}
	}
	kts := psi.Parse("build.gradle.kts", "val x = 1\n")
	if _, err := AnalyzeArena(context.Background(), Config{NoJDK: true}, []*psi.Snapshot{s, kts}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("script: %v", err)
	}
	if _, err := AnalyzeArena(context.Background(), Config{NoJDK: true}, []*psi.Snapshot{s, s}); err == nil {
		t.Error("a snapshot passed twice accepted")
	}
}

func TestCopies(t *testing.T) {
	r, s := analyzed(t, withStdlib(t), "fun f(l: List<String>) = l.map { it.length }\n")
	n := at(t, s, "CALL_EXPRESSION", "map { it.length }", 0)
	f := fact(t, r, n)
	if f.Call == nil || f.Type == nil || len(f.Type.Arguments) != 1 {
		t.Fatalf("map: %+v", f)
	}
	f.Call.Name = "changed"
	f.Call.ArgumentMapping[0] = 9
	f.Type.Arguments[0].Name = "changed"
	f.Type.Supertypes = append(f.Type.Supertypes[:0], "changed")
	again := fact(t, r, n)
	if again.Call.Name == "changed" || again.Call.ArgumentMapping[0] == 9 || again.Type.Arguments[0].Name == "changed" ||
		slices.Contains(again.Type.Supertypes, "changed") {
		t.Errorf("Fact shares its result's memory: %+v", again)
	}
	d := r.Data()
	d.Facts[0].Role = "changed"
	d.Symbols[0].Name = "changed"
	d.Capabilities[0] = "changed"
	d2 := r.Data()
	if d2.Facts[0].Role == "changed" || d2.Symbols[0].Name == "changed" || d2.Capabilities[0] == "changed" || Capabilities()[0] == "changed" {
		t.Error("Data shares its result's memory")
	}
	sym := symbol(t, r, f.Symbol+1)
	sym.Supertypes = append(sym.Supertypes, "changed")
	if slices.Contains(symbol(t, r, f.Symbol+1).Supertypes, "changed") {
		t.Error("Symbol shares its result's memory")
	}
}

func TestNilResult(t *testing.T) {
	var r *Result
	if _, ok := r.Fact(ElementID{}); ok {
		t.Error("nil result has a fact")
	}
	if _, ok := r.Symbol(1); ok {
		t.Error("nil result has a symbol")
	}
	if d := r.Data(); d.Facts == nil || d.Symbols == nil || len(d.Capabilities) == 0 {
		t.Errorf("nil result's data: %+v", d)
	}
}

func TestAnnotationTypes(t *testing.T) {
	r, s := analyzed(t, withStdlib(t), "annotation class A\nannotation class B\n@B @A fun f() {}\n@Deprecated(\"x\") class K\n")
	for _, c := range []struct {
		text string
		nth  int
		want string
	}{{"A", 0, "A"}, {"B", 0, "B"}, {"Deprecated", 0, "kotlin.Deprecated"}} {
		for _, kind := range []string{"TYPE_REFERENCE", "USER_TYPE"} {
			n := at(t, s, kind, c.text, c.nth)
			f, ok := r.Fact(ElementID{Snapshot: s.ID(), Node: n.ID()})
			if !ok || f.Type == nil || f.Type.Name != c.want {
				t.Errorf("%s %s: %+v %v, want type %s", kind, c.text, f, ok, c.want)
			}
		}
	}
}

func TestElvis(t *testing.T) {
	src := `package p

fun f(l: List<Int>?, s: String?, a: Any?) {
    val x = l ?: emptyList()
    val y = s ?: ""
    val z = a ?: ""
}
`
	r, s := analyzed(t, withStdlib(t), src)
	for _, c := range []struct {
		text string
		want bool
	}{{"l ?: emptyList()", true}, {`s ?: ""`, true}, {`a ?: ""`, false}} {
		f := fact(t, r, at(t, s, "BINARY_EXPRESSION", c.text, 0))
		if f.NonNullLeftSubtypeRight == nil || *f.NonNullLeftSubtypeRight != c.want {
			t.Errorf("%s: NonNullLeftSubtypeRight %v, want %v", c.text, f.NonNullLeftSubtypeRight, c.want)
		}
	}
}
