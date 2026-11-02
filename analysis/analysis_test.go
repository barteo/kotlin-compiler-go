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

// withJDK returns a configuration with the JDK and the standard library,
// or skips t.
func withJDK(t *testing.T) Config {
	t.Helper()
	cfg := withStdlib(t)
	if _, err := classpath.FindJDK(); err != nil {
		t.Skipf("no JDK (set JAVA_HOME): %v", err)
	}
	cfg.NoJDK = false
	return cfg
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

func TestBuiltinCallNames(t *testing.T) {
	src := `package p

fun f() {
    val a = emptyArray<Int>()
    val b = arrayOf(1)
    val c = intArrayOf(1)
    val d = arrayOfNulls<String>(2)
    val e = Char(65)
    val g = String(charArrayOf('a'))
    val h = IntArray(3)
    val i = Array(2) { it }
}
`
	r, s := analyzed(t, withStdlib(t), src)
	for text, want := range map[string]string{
		"emptyArray<Int>()":        "kotlin.emptyArray",
		"arrayOf(1)":               "kotlin.arrayOf",
		"intArrayOf(1)":            "kotlin.intArrayOf",
		"arrayOfNulls<String>(2)":  "kotlin.arrayOfNulls",
		"Char(65)":                 "kotlin.Char",
		"String(charArrayOf('a'))": "kotlin.text.String",
		"IntArray(3)":              "kotlin.IntArray.<init>",
		"Array(2) { it }":          "kotlin.Array.<init>",
	} {
		f := fact(t, r, at(t, s, "CALL_EXPRESSION", text, 0))
		if f.Call == nil || f.Call.Name != want {
			t.Errorf("%s: call %+v, want name %s", text, f.Call, want)
		}
	}
}

func TestTypeRefFacts(t *testing.T) {
	src := `package p

open class A<T>
class D : A<Void>()
typealias V = java.lang.Void
lateinit var c: () -> Void
val l: List<java.lang.Void?> = emptyList()
fun <T : Comparable<T>> g(vararg xs: String, y: Any): Map<String, Int?> {
    val z = y as List<String>
    val b = y is Set<*>
    val e = emptyList<Int>()
    return emptyMap()
}
`
	r, s := analyzed(t, withJDK(t), src)
	want := map[string]string{
		"A<Void>": "p.A", "Void": "java.lang.Void", "java.lang.Void": "java.lang.Void",
		"java.lang.Void?": "java.lang.Void", "() -> Void": "kotlin.Function0",
		"List<java.lang.Void?>": "kotlin.collections.List", "Comparable<T>": "kotlin.Comparable",
		"Int?": "kotlin.Int", "Set<*>": "kotlin.collections.Set",
	}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if n.NumericKind() != psi.KindTYPE_REFERENCE {
			return true
		}
		f, ok := r.Fact(ElementID{s.ID(), n.ID()})
		if !ok || f.Type == nil {
			t.Errorf("%q at %d: no type", n.Text(), n.Range().Start)
		} else if w, ok := want[n.Text()]; ok && f.Type.Name != w {
			t.Errorf("%q: %s, want %s", n.Text(), f.Type.Name, w)
		}
		return true
	})
}

func TestUnaryCallNames(t *testing.T) {
	src := `package p

fun f(b: Boolean, d: Double, c: Char) {
    var i = 1
    val a = -3
    val l = -3L
    val p = +3
    val n = !b
    val m = -d
    i++
    val q = c + 1
    -(4)
    i = if (b) {
        -1
    } else {
        1
    }
}
`
	r, s := analyzed(t, withStdlib(t), src)
	for _, c := range []struct{ kind, text, want string }{
		{"PREFIX_EXPRESSION", "-3", "kotlin.Int.unaryMinus"},
		{"PREFIX_EXPRESSION", "-3L", "kotlin.Long.unaryMinus"},
		{"PREFIX_EXPRESSION", "+3", "kotlin.Int.unaryPlus"},
		{"PREFIX_EXPRESSION", "!b", "kotlin.Boolean.not"},
		{"PREFIX_EXPRESSION", "-d", "kotlin.Double.unaryMinus"},
		{"POSTFIX_EXPRESSION", "i++", "kotlin.Int.inc"},
		{"PREFIX_EXPRESSION", "-(4)", "kotlin.Int.unaryMinus"},
	} {
		f := fact(t, r, at(t, s, c.kind, c.text, 0))
		if f.Call == nil || f.Call.Name != c.want {
			t.Errorf("%s: call %+v, want %s", c.text, f.Call, c.want)
		}
	}
	if f := fact(t, r, at(t, s, "PREFIX_EXPRESSION", "-3", 0)); !f.UsedAsExpression || f.Type == nil || f.Type.Name != "kotlin.Int" {
		t.Errorf("-3: %+v", f)
	}
	if f := fact(t, r, at(t, s, "PREFIX_EXPRESSION", "-(4)", 0)); f.UsedAsExpression {
		t.Errorf("-(4) is a statement: %+v", f)
	}
	if f := fact(t, r, at(t, s, "PREFIX_EXPRESSION", "-1", 0)); !f.UsedAsExpression {
		t.Errorf("-1 is the if's value: %+v", f)
	}
}

func TestCallAnnotations(t *testing.T) {
	src := `package p

annotation class CheckReturnValue
annotation class Marker

@CheckReturnValue
fun checked() = 1

@CheckReturnValue
class Outer {
    @Marker
    class Inner {
        fun g() = 2
    }
}

fun f() {
    checked()
    Outer.Inner().g()
    listOf(1)
}
`
	r, s := analyzed(t, withStdlib(t), src)
	c := fact(t, r, at(t, s, "CALL_EXPRESSION", "checked()", 0)).Call
	if c == nil || !slices.Equal(c.Annotations, []string{"p.CheckReturnValue"}) || len(c.OwnerAnnotations) != 0 {
		t.Errorf("checked(): %+v", c)
	}
	c = fact(t, r, at(t, s, "CALL_EXPRESSION", "g()", 0)).Call
	if c == nil || len(c.Annotations) != 0 || !slices.Equal(c.OwnerAnnotations, []string{"p.Marker"}) {
		t.Errorf("g(): %+v", c)
	}
	c = fact(t, r, at(t, s, "CALL_EXPRESSION", "listOf(1)", 0)).Call
	if c == nil || len(c.Annotations)+len(c.OwnerAnnotations)+len(c.PackageAnnotations) != 0 {
		t.Errorf("listOf(1): %+v", c)
	}
}

func TestDeprecatedFacts(t *testing.T) {
	src := `package p

const val MESSAGE = "hello"

@Deprecated("old fun") fun old() = 1
@Deprecated("old property") val oldProp = 1
@Deprecated("old class") open class Old
class A { @Deprecated("old method") fun m() = 1 }
@Deprecated(MESSAGE) fun constMsg() = 1
@Deprecated("old " + "message") fun concat() = 1
@Deprecated("error", level = DeprecationLevel.ERROR) fun err() = 1
fun fresh() = 1

class Sub : Old()

fun f(a: A, l: List<Old>) {
    old()
    oldProp
    Old()
    a.m()
    constMsg()
    concat()
    fresh()
    err()
}
`
	r, s := analyzed(t, withStdlib(t), src)
	for _, c := range []struct {
		kind, text string
		nth        int
		want       string
	}{
		{"REFERENCE_EXPRESSION", "old", 0, "old fun"},
		{"REFERENCE_EXPRESSION", "oldProp", 0, "old property"},
		{"REFERENCE_EXPRESSION", "Old", 2, "old class"},
		{"REFERENCE_EXPRESSION", "m", 0, "old method"},
		{"REFERENCE_EXPRESSION", "constMsg", 0, "hello"},
		{"REFERENCE_EXPRESSION", "concat", 0, "old message"},
		{"USER_TYPE", "Old", 0, "old class"},
		{"USER_TYPE", "Old", 1, "old class"},
	} {
		f := fact(t, r, at(t, s, c.kind, c.text, c.nth))
		if f.Deprecated == nil || *f.Deprecated != c.want {
			t.Errorf("%s %s #%d: deprecated %v, want %q", c.kind, c.text, c.nth, f.Deprecated, c.want)
		}
	}
	for _, text := range []string{"fresh", "err"} {
		if f := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", text, 0)); f.Deprecated != nil {
			t.Errorf("%s: deprecated %q", text, *f.Deprecated)
		}
	}
}

func TestLibraryAnnotations(t *testing.T) {
	src := `package p

fun f(d: java.util.Date, r: Runnable, l: List<Int>) {
    d.getYear()
    r.run()
    l.size
}
`
	r, s := analyzed(t, withJDK(t), src)
	n := at(t, s, "REFERENCE_EXPRESSION", "getYear", 0)
	if f := fact(t, r, n); f.Deprecated == nil || *f.Deprecated != "Deprecated in Java" {
		t.Errorf("getYear: deprecated %v", f.Deprecated)
	}
	c := fact(t, r, at(t, s, "CALL_EXPRESSION", "getYear()", 0)).Call
	if c == nil || !slices.Contains(c.Annotations, "java.lang.Deprecated") {
		t.Errorf("getYear(): %+v", c)
	}
	c = fact(t, r, at(t, s, "CALL_EXPRESSION", "run()", 0)).Call
	if c == nil || !slices.Contains(c.OwnerAnnotations, "java.lang.FunctionalInterface") {
		t.Errorf("run(): %+v", c)
	}
}

func TestSuspendOperators(t *testing.T) {
	src := `package p

class C {
    suspend operator fun inc() = this
    suspend operator fun plusAssign(o: Int) {}
    suspend operator fun iterator(): Iterator<Int> = listOf(1).iterator()
}

class D {
    suspend operator fun plus(o: Int) = this
}

suspend fun f() {
    var c = C()
    c++
    c += 1
    var d = D()
    d += 1
    for (x in c) {}
    for (y in 1..2) {}
    println(d)
}
`
	r, s := analyzed(t, withStdlib(t), src)
	for _, want := range []struct{ kind, text, name string }{
		{"POSTFIX_EXPRESSION", "c++", "p.C.inc"},
		{"BINARY_EXPRESSION", "c += 1", "p.C.plusAssign"},
		{"BINARY_EXPRESSION", "d += 1", "p.D.plus"},
	} {
		c := fact(t, r, at(t, s, want.kind, want.text, 0)).Call
		if c == nil || c.Name != want.name || !c.Suspend {
			t.Errorf("%s: %+v", want.text, c)
		}
	}
	if f := fact(t, r, at(t, s, "FOR", "for (x in c) {}", 0)); !f.SuspendOperation {
		t.Errorf("for (x in c): %+v", f)
	}
	if n := at(t, s, "FOR", "for (y in 1..2) {}", 0); func() bool { f, _ := r.Fact(ElementID{n.ContainingFile().ID(), n.ID()}); return f.SuspendOperation }() {
		t.Errorf("for (y in 1..2) suspends")
	}
	// println is an inline-only function kc calls as an intrinsic.
	if c := fact(t, r, at(t, s, "CALL_EXPRESSION", "println(d)", 0)).Call; c == nil || !c.Inline {
		t.Errorf("println(d): %+v", c)
	}
}

func TestReferenceCalls(t *testing.T) {
	src := `package p

class C(val n: Int)

fun g(x: Int) = x

val s = suspend { g(1) }

fun f() {
    listOf(1).map(::C)
    listOf(2).map(::g)
    listOf("a").map(String::length)
}
`
	r, s := analyzed(t, withStdlib(t), src)
	for _, want := range []struct{ text, name string }{{"::C", "p.C.<init>"}, {"::g", "p.g"}} {
		c := fact(t, r, at(t, s, "CALLABLE_REFERENCE_EXPRESSION", want.text, 0)).Call
		if c == nil || c.Name != want.name {
			t.Errorf("%s: %+v", want.text, c)
		}
	}
	if c := fact(t, r, at(t, s, "CALLABLE_REFERENCE_EXPRESSION", "String::length", 0)).Call; c != nil {
		t.Errorf("String::length: %+v", c)
	}
	c := fact(t, r, at(t, s, "CALL_EXPRESSION", "suspend { g(1) }", 0)).Call
	if c == nil || c.Name != "kotlin.suspend" || !c.Inline || !slices.Equal(c.Noinline, []bool{true}) || len(c.Parameters) != 1 || !c.Parameters[0].Suspend {
		t.Errorf("suspend { }: %+v", c)
	}
	if c := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "suspend", 0)).Call; c == nil || c.Name != "kotlin.suspend" {
		t.Errorf("suspend: %+v", c)
	}
}

func TestNullability(t *testing.T) {
	src := `package p

fun <T> f(x: T, y: String, z: String?) {
    x!!
    val a = y ?: "fallback"
    val b = z ?: "fallback"
}

fun <T : Any> g(x: T) = x
`
	r, s := analyzed(t, withStdlib(t), src)
	if ty := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "x", 0)).Type; ty == nil || !ty.Nullable || ty.MarkedNullable {
		t.Errorf("x: T: %+v", ty)
	}
	if ty := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "x", 1)).Type; ty == nil || ty.Nullable {
		t.Errorf("x: T : Any: %+v", ty)
	}
	if f := fact(t, r, at(t, s, "BINARY_EXPRESSION", `y ?: "fallback"`, 0)); !f.Unreachable {
		t.Errorf("y ?: fallback: %+v", f)
	}
	if f := fact(t, r, at(t, s, "BINARY_EXPRESSION", `z ?: "fallback"`, 0)); f.Unreachable {
		t.Errorf("z ?: fallback: %+v", f)
	}
}
