package syntax

import (
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/types"
)

func TestParseValid(t *testing.T) {
	tests := []struct{ name, src string }{
		{"empty", ""},
		{"fun block", "fun f() { }"},
		{"fun expr", "fun f(x: Int): Int = x * 2"},
		{"brace on next line", "fun f()\n{\n}"},
		{"params trailing comma", "fun f(a: Int, b: String?,) {}"},
		{"top-level props", "val a = 1\nvar b: Long = 2L\nprivate val c = \"x\""},
		{"class", "class P(val x: Int, var y: Double, z: String) {\n init { println(z) }\n fun m() = x\n}"},
		{"class without body", "class E(val v: Int)"},
		{"semicolons", "fun f() { val a = 1; val b = 2; println(a + b) }"},
		{"if else on new line", "fun f(c: Boolean) {\n if (c) println(1)\n else println(2)\n}"},
		{"if expression", "fun f(c: Boolean) = if (c) 1 else 2"},
		{"when subject", "fun f(x: Int) = when (x) {\n 1, 2 -> \"a\"\n in 3..4 -> \"b\"\n !in 0..9 -> \"c\"\n else -> \"d\"\n}"},
		{"definitely non-nullable type", "fun <T> f(x: T & Any, l: List<T & Any>?): (T & Any) -> T = { it }"},
		{"local extension function", "fun f() { fun Int.g() = 1 }"},
		{"anonymous function", "fun f() = fun(x: Int) = x"},
		{"anonymous function on next line", "val f =\n    fun (x: Int): Int { return x }"},
		{"anonymous extension function", "fun f(x: Int) = when (x) {\n 0 -> null\n else -> fun Int.() { }\n}"},
		{"when guard", "fun f(x: Int) = when (x) { 1 if x > 0 -> 1\n else if x < 0 -> 2\n else -> 0 }"},
		{"when no subject", "fun f(x: Int) = when {\n x < 0 -> 1\n else -> 2\n}"},
		{"loops", "fun f() {\n for (i in 0 until 10 step 2) { continue }\n while (true) break\n}"},
		{"ranges", "fun f() { for (i in 10 downTo 1) {}; for (i in 0..<3) {} }"},
		{"safe calls and elvis", "fun f(s: String?) = s?.length ?: -1"},
		{"chain across lines", "fun f(s: String) = s\n  .uppercase()\n  .lowercase()"},
		{"elvis on next line", "fun f(s: String?) = s\n  ?: \"default\""},
		{"logic across lines", "fun f(a: Boolean, b: Boolean) = a\n  && b\n  || !a"},
		{"newlines in parens", "fun f() = g(\n  1,\n  2\n)"},
		{"templates", "fun f(n: Int) = \"n=$n sq=${n * n} esc=\\$n\""},
		{"not null", "fun f(s: String?) = s!!.length"},
		{"postfix inc", "fun f() { var i = 0; i++; i-- }"},
		{"compound assign", "fun f() { var i = 0; i += 1; i -= 1; i *= 2; i /= 2; i %= 2 }"},
		{"return forms", "fun f(x: Int?): Int {\n val y = x ?: return 0\n return y\n}"},
		{"comments", "/* block /* nested */ */ fun f() { // line\n}"},
		{"literals", "val a = 0x1F\nval b = 0b101\nval c = 1_000L\nval d = 1.5e3\nval e = null\nval f = true"},
		{"imports", "import java.util.Random\nimport java.util.*; import java.math.BigInteger as Big\n\nfun f() {}"},
		{"qualified types", "fun f(r: java.util.Random?): java.lang.StringBuilder = java.lang.StringBuilder()"},
		{"generic class", "class Box<out T : Any, in U, V>(val v: T) {\n fun <W : Comparable<W>> m(w: W): Box<T, *, out W>? = null\n}"},
		{"type args", "fun f(m: java.util.HashMap<String, java.util.ArrayList<Int?>>?) {}"},
		{"call type args", "fun f() = g<Int, Box<String>>(1).h<Long>()"},
		{"comparisons", "fun f(a: Int, b: Int) = a < b && b > a"},
		{"function types", "fun f(g: (Int, String?) -> Unit, h: () -> (Int) -> Int, k: ((Int) -> Int)?, n: (x: Int) -> Int) {}"},
		{"function type args", "val m: java.util.HashMap<String, (Int) -> Int>? = null\nfun f() = g<(Int) -> Int>(h)"},
		{"function types with receivers", "fun f(g: String.() -> Unit, h: Int.(String) -> Int, k: (String.() -> Unit)?, m: List<Int>?.(x: Int) -> Unit) {}"},
		{"qualified this", "class A { fun f() = g { this@A.h(this@g) } }"},
		{"lambdas", "val a = { }\nval b = { x: Int, y -> x + y }\nval c = { -> 1 }\nval d = {\n  it\n}\nval e = { _, x -> x }"},
		{"trailing lambdas", "fun f() {\n g { it }\n h(1) { x -> x }\n o.m { }\n k<Int> { 1 }(2)\n}"},
		{"labeled trailing lambda after type arguments", "fun f() = k<Int> sc@{ return@sc 1 }"},
		{"labeled return", "fun f() = g { if (it) return@g 1\n return@g\n}"},
		{"labeled loops", "fun f() {\n outer@ for (i in 0..1) { inner@ while (true) { break@outer; continue@inner } }\n l@ do { continue@l } while (false)\n}"},
		{"labeled lambdas", "fun f() {\n xs.forEach lit@{ return@lit }\n g(1) l@{ }\n val h = l@{ x: Int -> return@l x }\n}"},
		{"delegated properties", "val a by lazy { 1 }\nclass C {\n private var b: Int by\n  d()\n private set\n}\nfun f() { val c: Int by m }"},
		{"object expressions", "fun f() = object { val x = 1 }\nfun g() = h(object : A(1), I {\n override fun m() = 2\n}, 3)\nval r = object\n : Runnable { override fun run() {} }"},
		{"do-while", "fun f() {\n do { val x = 1 } while (x < 1)\n do x++\n while (x < 5)\n}"},
		{"lambda statements", "fun f() = g {\n val x = 1; println(x)\n x\n}"},
		{"conditions before braces", "fun f(a: Int, b: Int) {\n if (a < b) { }\n while (a > b) { }\n when (a) { 1 -> {}; else -> {} }\n}"},
		{"raw strings", "val a = \"\"\"x\n  \"q\" \\n $a ${a + 1}\"\"\"\nval b = \"\"\"\"\"\"\"\""},
		{"strings in templates", "fun f(m: Map<String, Int>) = \"${m[\"a\"]} ${if (m.isEmpty()) \"}\" else \"{\"} ${\"${1}\"}\""},
		{"non-ASCII names", "val π = 3.14\nfun größe(ä: Int) = ä + π"},
		{"negation of is-names", "fun f(s: String) = !s.isEmpty() && !isOk(s)"},
		{"modifier names", "fun f(value: Int, data: Int, open: Int) = value + data + open"},
		{"private constructor", "class A private constructor(val x: Int)\nclass B internal constructor() {}"},
		{"secondary constructors", "class A(val x: Int) {\n constructor(s: String, k: Int = 1) : this(s.length * k) {\n  println(s)\n }\n private constructor() : this(0)\n}\nclass B : E {\n constructor(m: String) : super(m)\n constructor()\n {\n }\n @Inject constructor(x: Int)\n  : this(x.toString())\n}"},
		{"local functions", "fun f(open: Int) {\n fun g(x: Int = 1): Int = x\n fun <T> h(t: T) { println(t) }\n infix fun k(y: Int) = 0\n open + 1\n private fun p() {}\n}"},
		{"class delegation", "class A(b: I, val c: J) : B(), I by b, J by c\nclass C : I by f(1) {\n fun g() {}\n}\nclass D : I by run({ x })"},
		{"identity equality", "fun f(a: Any, b: Any?) = a === b || b !== null"},
		{"callable references", "fun f(x: Any) = listOf(::g, String::length, x::m, this::n, a.b.C::d, Box<*>::get, ::C)"},
		{"class literals", "fun f(x: Any) = listOf(A::class, a.b.C::class.java, Map.Entry<*, *>::class, x::class, this::class.java)"},
		{"annotations", "@file:Suppress(\"X\")\npackage a\n@Deprecated(\"x\", level = DeprecationLevel.HIDDEN)\n@JvmName(\"g\") public fun f(@Ann x: @Ann2 Int): Int {\n  @Suppress(\"UNCHECKED_CAST\")\n  val y = @Ann3 x\n  return y\n}\nclass A @Inject constructor(@param:Named(\"n\") val n: Int) {\n  @Volatile\n  var v = 1\n  @[A B] fun g() {}\n}"},
		{"annotation classes", "annotation class A\n@Target(AnnotationTarget.CLASS) annotation class B(val x: Int, vararg val ys: String)\nclass C { annotation class D }"},
		{"unsigned literals", "val a = 1u + 0xFFu + 1uL + 18446744073709551615u"},
		{"variables assigned later", "fun f(c: Boolean) {\n val x: Int\n var y: String\n if (c) x = 1 else x = 2\n}"},
		{"reified type parameters", "inline fun <reified T : Any, reified U> f(x: Any) = x is T && U::class == T::class"},
		{"inline functions", "inline fun f(noinline g: () -> Unit, crossinline h: () -> Unit) { g(); h() }"},
		{"value classes", "@JvmInline\nvalue class A(val x: Int)\n@kotlin.jvm.JvmInline internal value class B(override val y: String) : I\nfun f(value: Int) = value"},
		{"accessor annotations on their own lines", "class A {\n val a: Int\n  @JvmName(\"n\")\n  get() = 1\n var b = 1\n  @Ann\n  private set\n val c = 2\n  @get:Ann get\n}"},
		{"return types on the next line", "fun f(x: Int)\n : Int = x\nfun <T> g()\n        : List<T> {\n return listOf()\n}\nfun h():\n  Int = 1"},
		{"by on the next line", "object A : I\nby b()\nval x: Int\n by lazy { 1 }"},
		{"bounds on the next line", "class A<E, B\n: C<E>>\nfun <\n T,\n R : Any,\n> f() {}"},
		{"if and parenthesis on different lines", "fun f(c: Boolean) = if\n (c) 1 else 2"},
		{"function type arrow on the next line", "fun f(g: (Int)\n -> Int): (Int, Int)\n  -> Int = h"},
		{"annotated expression bodies", "fun f() =\n @Suppress(\"x\")\n when (x) { else -> 1 }"},
		{"trailing comma in when", "fun f(x: Int) = when (x) {\n 1,\n 2,\n -> 3\n else -> 4\n}"},
		{"lambdas as branch bodies", "fun f(c: Boolean) = if (c) { x -> x } else { y: Int -> y }\nfun g(x: Int) = when (x) {\n 0 -> { a, b -> a }\n else -> { -> 1 }\n}"},
		{"else on the next line in a when branch", "fun f(x: Int?) {\n when (x) {\n  null -> if (c) return\n  else -> g()\n }\n}"},
		{"lambda arrow on the next line", "fun f() = g { x\n -> x }"},
		{"type arguments and lambda on the next line", "fun f() = xs\n .fold<Int, Int>\n { a, b -> a + b }"},
		{"lambda on the next line is trailing", "fun f() {\n g()\n { 1 }\n}"},
		{"callable reference on a nullable type", "val f = String?::plus"},
		{"types of inner classes of generic classes", "fun f(x: Outer<*>.Inner, y: A<Int>.B.C<String>?) = x as Outer<Int>.Inner"},
		{"named function type parameters in type arguments", "val a = ArrayList<(call: Call, id: String) -> Unit>()"},
		{"receiver in parentheses", "fun f(g: (List<*>).() -> Int) {}"},
		{"trailing comma in an index", "fun f() = m[\n 1,\n 2,\n]"},
		{"empty loop body", "fun f() { while (g());\n }"},
		{"annotations on catch parameters and type parameters", "fun <@A T> f() {\n try { } catch (@Suppress(\"x\") e: E) { }\n}"},
		{"annotated primary constructor on the next line", "class A<S>\n@Inject\ninternal constructor(val s: S)"},
		{"typealiases", "typealias A = Int\nprivate typealias F<T> = (T) -> Unit\nactual typealias L = java.util.ArrayList<String>\nclass C {\n typealias N = String\n}"},
		{"setters", "class A {\n var a = 1\n  set(value) { field = value }\n var b = 2\n  private set(v: Int) = g(v)\n var c: Int\n  get() = 1\n  set(x,) {}\n}"},
		{"lateinit and tailrec", "class A { lateinit var a: String }\nlateinit var b: String\nfun f() {\n lateinit var c: String\n tailrec fun g(n: Int): Int = if (n == 0) 0 else g(n - 1)\n}\ntailrec fun h(n: Int) {}"},
		{"local classes", "fun f() {\n class A(val x: Int)\n data class P(val a: Int)\n open class B { fun g() = 1 }\n}"},
		{"expect and actual", "expect fun f(): Int\nactual fun f(): Int = 1\nexpect class A(x: Int) { fun g() }\nactual class A actual constructor(x: Int) {\n actual fun g() {}\n}\nactual class B\n  actual constructor()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, errs := Parse("t.kt", tt.src); len(errs) > 0 {
				t.Fatalf("unexpected error: %v", errs[0])
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct{ src, pos, msg string }{
		{"fun f() { val x }", "1:15", "this variable must either have an explicit type or be initialized"},
		{"fun f() { 1 + }", "1:15", "expected expression"},
		{"fun f() { val a = 1 val b = 2 }", "1:21", "expected newline or ';'"},
		{"val <T> x = 1", "1:1", "type parameters of a property must be used in its receiver type"},
		{"val c = 'a", "1:9", "unterminated character literal"},
		{"val s = \"\"\"abc", "1:9", "unterminated string literal"},
		{"fun f(g: (Int, Int)) {}", "1:20", "expected '->'"},
		{"val x: Box<Int = 1", "1:16", "expected '>'"},
		{"fun f() {}\nimport a.B", "2:1", "imports must come before declarations"},
		{"import a.*.b", "1:11", "expected newline or ';'"},
		{"import a.B as", "1:14", "expected identifier"},
		{"fun ((@receiver:A Int) -> Unit).f() {}", "1:7", "this annotation is not applicable to target 'type usage' and use-site target '@receiver'"},
		{"val s = \"abc", "1:9", "unterminated string literal"},
		{"val c = ''", "1:9", "empty character literal"},
		{"val c = '\\q'", "1:9", "illegal escape"},
		{"val c = '\U0001F600'", "1:9", "too many characters in a character literal"},
		{"fun f() { try { } }", "1:11", "'try' must have at least one 'catch' or 'finally' block"},
		{"vararg fun f() {}", "1:1", "modifier 'vararg' is only allowed on parameters"},
		{"val x by d\n get() = 1", "2:2", "delegated property cannot have accessors with non-default implementations"},
		{"fun <T> f() where U : Any {}", "1:19", "'U' does not refer to a type parameter of this declaration"},
		{"context(Int) fun f() = 1", "1:9", "context parameters must be named"},
		{"context() fun f() = 1", "1:9", "expected a context parameter"},
		{"fun f(g: context() () -> Unit) {}", "1:18", "expected a context parameter type"},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			_, errs := Parse("t.kt", tt.src)
			if len(errs) == 0 {
				t.Fatal("expected an error")
			}
			e := errs[0]
			if e.Pos.String() != tt.pos || !strings.Contains(e.Msg, tt.msg) {
				t.Errorf("got %s: %s, want %s: %s", e.Pos, e.Msg, tt.pos, tt.msg)
			}
		})
	}
}

// TestUnsupportedFeatures checks the feature that each unsupported
// construct is reported as, which bench/survey counts.
func TestUnsupportedFeatures(t *testing.T) {
	tests := []struct{ src, feature string }{
		{"context(c: Int) class C", "context parameters"},
		{"class C {\n  context(c: Int)\n  constructor()\n}", "context parameters"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			_, errs := Parse("t.kt", tt.src)
			if len(errs) == 0 {
				t.Fatal("expected an error")
			}
			if errs[0].Feature != tt.feature {
				t.Errorf("got feature %q (%s), want %q", errs[0].Feature, errs[0].Msg, tt.feature)
			}
		})
	}
}

func TestTokens(t *testing.T) {
	toks, errs := Tokenize("t.kt", "a as? B; !is; !isOk; x === y !== z; A::f; @Ann; loop@ for; return@f; this@A; 'c'; 1f; 2uL; is as")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	var kinds []string
	for _, tk := range toks {
		if tk.Kind == SEMI {
			continue
		}
		kinds = append(kinds, tk.Kind.String()+"("+tk.Text+")")
	}
	got := strings.Join(kinds, " ")
	want := "identifier(a) as?(as?) identifier(B) !is(!is) !(!) identifier(isOk) identifier(x) ===(===) identifier(y) !==(!==) identifier(z) " +
		"identifier(A) ::(::) identifier(f) @(@) identifier(Ann) label(loop) for(for) return(return@f) this(this@A) " +
		"character literal('c') float literal(1f) unsigned literal(2uL) is(is) as(as) end of file()"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestRawString(t *testing.T) {
	toks, errs := Tokenize("t.kt", "\"\"\"a\n\\n$x${y}\"\"\"\"")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	var parts []string
	for _, p := range toks[0].Parts {
		if p.IsExpr {
			parts = append(parts, "{"+p.Src+"}")
		} else {
			parts = append(parts, p.Lit)
		}
	}
	if got, want := strings.Join(parts, "|"), "a\n\\n|{x}|{y}|\""; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if toks[1].Kind != EOF {
		t.Errorf("got %s after the string, want end of file", toks[1].Kind)
	}
}

func TestNewlinesSuppressedInParens(t *testing.T) {
	toks, _ := Tokenize("t.kt", "f(\n1\n)\n{\nx\n}")
	var kinds []string
	for _, tk := range toks {
		kinds = append(kinds, tk.Kind.String())
	}
	got := strings.Join(kinds, " ")
	want := "identifier ( integer literal ) newline { newline identifier newline } end of file"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestImports(t *testing.T) {
	f, errs := Parse("t.kt", "import java.util.Random\nimport java.util.*\nimport java.math.BigInteger as Big")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	want := []Import{
		{Path: "java.util.Random"},
		{Path: "java.util", Star: true},
		{Path: "java.math.BigInteger", Alias: "Big"},
	}
	if len(f.Imports) != len(want) {
		t.Fatalf("got %d imports, want %d", len(f.Imports), len(want))
	}
	for i, w := range want {
		g := *f.Imports[i]
		g.P = Pos{}
		if g != w {
			t.Errorf("import %d: got %+v, want %+v", i, g, w)
		}
	}
}

func TestFacadeName(t *testing.T) {
	for in, want := range map[string]string{
		"a/fizzBuzz.kt": "FizzBuzzKt",
		"Main.kt":       "MainKt",
		"my-file.kt":    "My_fileKt",
		"-Ext.kt":       "_ExtKt",
		"1st.kt":        "_1stKt",
		"_x.kt":         "_xKt",
	} {
		if got := FacadeName(in); got != want {
			t.Errorf("FacadeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenericSyntax(t *testing.T) {
	f, errs := Parse("t.kt", "fun <T : Any> f(b: Box<out T, *>?) {\n g<Int>(1)\n a < b > (c)\n a < b\n}")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	fd := f.Decls[0].(*FunDecl)
	if len(fd.TypeParams) != 1 || fd.TypeParams[0].Name != "T" || len(fd.TypeParams[0].Bounds) != 1 || fd.TypeParams[0].Bounds[0].Name != "Any" {
		t.Errorf("type parameters: %+v", fd.TypeParams)
	}
	pt := fd.Params[0].Type
	if !pt.Nullable || len(pt.Args) != 2 || pt.Args[0].Variance != types.Out || pt.Args[0].Type.Name != "T" || pt.Args[1].Variance != types.Star {
		t.Errorf("parameter type: %+v", pt)
	}
	stmts := fd.Body.Stmts
	if c, ok := stmts[0].(*ExprStmt).X.(*Call); !ok || len(c.TypeArgs) != 1 || c.TypeArgs[0].Type.Name != "Int" {
		t.Errorf("g<Int>(1) is not a call with type arguments")
	}
	// As in Kotlin, a < b > (c) is a call with a type argument.
	if c, ok := stmts[1].(*ExprStmt).X.(*Call); !ok || len(c.TypeArgs) != 1 {
		t.Errorf("a < b > (c) is not a call")
	}
	if b, ok := stmts[2].(*ExprStmt).X.(*Binary); !ok || b.Op != LT {
		t.Errorf("a < b is not a comparison")
	}
}

func TestParseAllRecovers(t *testing.T) {
	src := `@file:JvmName("X")
package a.b

import a.C

context(c: Int) class D(val x: Int)

fun ok() = 1

class K {
  fun f() = x as String
  fun g() = 2
  companion object {
    fun h() {
      try { } finally { }
    }
  }
  val n: Int
    get() = 1
}

interface I {
  fun f()
}

fun ok2() {
  println("${ "}" }")
}
`
	f, errs := ParseAll("t.kt", src)
	var got []string
	for _, e := range errs {
		got = append(got, e.Pos.String()+" "+e.Feature)
	}
	want := []string{"6:1 context parameters"}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("got  %s\nwant %s", strings.Join(got, ", "), strings.Join(want, ", "))
	}
	var names []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *FunDecl:
			names = append(names, d.Name)
		case *ClassDecl:
			names = append(names, d.Name)
			for _, m := range d.Members {
				if fd, ok := m.(*FunDecl); ok {
					names = append(names, d.Name+"."+fd.Name)
				}
			}
		}
	}
	if got, want := strings.Join(names, " "), "ok K K.f K.g I I.f ok2"; got != want {
		t.Errorf("parsed declarations: got %s, want %s", got, want)
	}
}

func TestClassHeaderOnSeveralLines(t *testing.T) {
	f, errs := Parse("t.kt", "class A\nprivate constructor(val x: Int)\nclass B\nfun f() {}\nclass C\n(val y: Int) {\n}")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	a, b, c := f.Decls[0].(*ClassDecl), f.Decls[1].(*ClassDecl), f.Decls[3].(*ClassDecl)
	if len(a.CtorParams) != 1 || len(b.CtorParams) != 0 || len(c.CtorParams) != 1 {
		t.Errorf("constructor parameters: A %d, B %d, C %d", len(a.CtorParams), len(b.CtorParams), len(c.CtorParams))
	}
	f, errs = Parse("t.kt", "class A(val x: Int)\n  : B(x),\n    I")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	if a := f.Decls[0].(*ClassDecl); len(a.Supers) != 2 || a.Supers[0].Call == nil || a.Supers[1].Call != nil {
		t.Errorf("supertypes on the next lines: %+v", a.Supers)
	}
}

func TestPackage(t *testing.T) {
	f, errs := Parse("dir/util.kt", "@file:JvmName(\"Utils\")\n\npackage com.example.util\n\nimport a.B\nfun f() {}")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	if f.Package != "com.example.util" || f.Facade != "com/example/util/Utils" || len(f.Imports) != 1 {
		t.Errorf("package %q, facade %q, %d imports", f.Package, f.Facade, len(f.Imports))
	}
	f, _ = Parse("main.kt", "package a\nfun main() {}")
	if f.Facade != "a/MainKt" {
		t.Errorf("facade %q, want a/MainKt", f.Facade)
	}
}

func TestInheritanceSyntax(t *testing.T) {
	src := `interface Shape : Named, Comparable<Shape> {
  val area: Double
  val label: String get() = "shape"
  fun scale(k: Double): Shape
  fun describe(): String { return label }
}
abstract class Base<T>(val x: T) : java.lang.Object(), Shape {
  abstract fun f(): Int
  open val y: Int = 1
  override final fun scale(k: Double): Shape = this
  var count = 0
    private set
}
open class Sub : Base<Int>(1) {
  override fun f() = super.f() + super<Shape>.hashCode()
  override val area: Double
    get() {
      return 1.0
    }
}`
	f, errs := Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	shape, base, sub := f.Decls[0].(*ClassDecl), f.Decls[1].(*ClassDecl), f.Decls[2].(*ClassDecl)
	if !shape.Interface || len(shape.Supers) != 2 || shape.Supers[1].Type.Args == nil {
		t.Errorf("interface: %+v", shape)
	}
	if fd := shape.Members[2].(*FunDecl); fd.Body != nil || fd.ExprBody != nil {
		t.Errorf("abstract function has a body")
	}
	if pd := shape.Members[1].(*PropDecl); pd.Getter == nil || pd.Getter.ExprBody == nil || pd.Init != nil {
		t.Errorf("getter: %+v", pd)
	}
	if !base.Abstract || base.Supers[0].Call == nil || base.Supers[0].Type.Name != "java.lang.Object" {
		t.Errorf("abstract class: %+v", base)
	}
	if fd := base.Members[2].(*FunDecl); !fd.Override || !fd.Final {
		t.Errorf("override final: %+v", fd.Modifiers)
	}
	if !sub.Open || sub.Supers[0].Call == nil || len(sub.Supers[0].Call.TypeArgs) != 1 {
		t.Errorf("open class: %+v", sub)
	}
	if pd := sub.Members[1].(*PropDecl); !pd.Override || pd.Getter == nil || pd.Getter.Body == nil {
		t.Errorf("override with getter: %+v", pd)
	}
}

func TestTypeChecks(t *testing.T) {
	f, errs := Parse("t.kt", `fun f(x: Any) {
  val a = x is String && x !is Int
  val b = -x as Int * 2
  val c = x
    as? String ?: ""
  when (val y = g(x)) {
    is String, !is Int -> 1
    null -> 2
    else -> 3
  }
}`)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	stmts := f.Decls[0].(*FunDecl).Body.Stmts
	and := stmts[0].(*VarStmt).Init.(*Binary)
	if is, ok := and.X.(*Is); !ok || is.Not || is.Ref.Name != "String" {
		t.Errorf("x is String: %#v", and.X)
	}
	if is, ok := and.Y.(*Is); !ok || !is.Not {
		t.Errorf("x !is Int: %#v", and.Y)
	}
	// as binds tighter than *, and looser than unary minus.
	mul := stmts[1].(*VarStmt).Init.(*Binary)
	if as, ok := mul.X.(*As); !ok || as.Safe {
		t.Errorf("-x as Int * 2: %#v", mul.X)
	} else if _, ok := as.X.(*Unary); !ok {
		t.Errorf("-x as Int: %#v", as.X)
	}
	elvis := stmts[2].(*VarStmt).Init.(*Binary)
	if as, ok := elvis.X.(*As); !ok || !as.Safe {
		t.Errorf("x as? String: %#v", elvis.X)
	}
	w := stmts[3].(*ExprStmt).X.(*When)
	if w.SubjVar != "y" || w.Subject == nil {
		t.Errorf("subject variable: %q", w.SubjVar)
	}
	if c := w.Branches[0].Conds; len(c) != 2 || !c[0].Is || !c[1].NotIs || c[1].Type.Name != "Int" {
		t.Errorf("is conditions: %+v", c)
	}
}

func TestTry(t *testing.T) {
	f, errs := Parse("t.kt", `fun f(): Int {
  val x = try {
    g()
  }
  catch (e: IllegalStateException) { 1 }
  catch (e: Exception) {
    throw RuntimeException("wrapped", e)
  } finally {
    done()
  }
  try { h() } finally { }
  return x ?: throw IllegalArgumentException()
}`)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	stmts := f.Decls[0].(*FunDecl).Body.Stmts
	x := stmts[0].(*VarStmt).Init.(*Try)
	if len(x.Catches) != 2 || x.Catches[1].Type.Name != "Exception" || x.Finally == nil {
		t.Errorf("try: %+v", x)
	}
	if th, ok := x.Catches[1].Body.Stmts[0].(*ExprStmt).X.(*Jump); !ok || th.Kind != THROW {
		t.Errorf("throw in catch: %+v", x.Catches[1].Body.Stmts[0])
	}
	if y := stmts[1].(*ExprStmt).X.(*Try); len(y.Catches) != 0 || y.Finally == nil {
		t.Errorf("try-finally: %+v", y)
	}
}

func TestObjects(t *testing.T) {
	f, errs := Parse("t.kt", `object O : I {
  const val k = 1
}
class A {
  companion object {
    fun make() = A()
  }
  companion object Named
  class B
  interface C
  object D
}`)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	o, a := f.Decls[0].(*ClassDecl), f.Decls[1].(*ClassDecl)
	if !o.Object || o.Name != "O" || len(o.Supers) != 1 {
		t.Errorf("object: %+v", o)
	}
	var names []string
	for _, m := range a.Members {
		if cd, ok := m.(*ClassDecl); ok {
			n := cd.Name
			if cd.Companion {
				n = "companion " + n
			}
			names = append(names, n)
		}
	}
	if got := strings.Join(names, ", "); got != "companion Companion, companion Named, B, C, D" {
		t.Errorf("nested: %s", got)
	}
}

func TestDefaultsAndNamedArgs(t *testing.T) {
	f, errs := Parse("t.kt", "class P(val x: Int = 1, val y: Int = x + 1)\nfun f(a: Int, b: String = \"b\") = g(a, c = 2, d =\n 3) { it }")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	p, fd := f.Decls[0].(*ClassDecl), f.Decls[1].(*FunDecl)
	if p.CtorParams[0].Default == nil || p.CtorParams[1].Default == nil || fd.Params[0].Default != nil || fd.Params[1].Default == nil {
		t.Errorf("defaults: %+v %+v", p.CtorParams, fd.Params)
	}
	c := fd.ExprBody.(*Call)
	if len(c.Args) != 4 || c.Name(0) != "" || c.Name(1) != "c" || c.Name(2) != "d" || c.Name(3) != "" {
		t.Errorf("names: %q", c.Names)
	}
}

func TestVarargAndSpread(t *testing.T) {
	f, errs := Parse("t.kt", "class A(vararg val xs: Int)\nfun f(a: Int, vararg b: String?) = g(1, *a, 2, c = *d)")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	a, fd := f.Decls[0].(*ClassDecl), f.Decls[1].(*FunDecl)
	if !a.CtorParams[0].Param.Vararg || !a.CtorParams[0].IsProp || fd.Params[0].Vararg || !fd.Params[1].Vararg || !fd.Params[1].Type.Nullable {
		t.Errorf("vararg: %+v %+v", a.CtorParams[0], fd.Params)
	}
	c := fd.ExprBody.(*Call)
	_, s1 := c.Args[1].(*Spread)
	_, s3 := c.Args[3].(*Spread)
	if len(c.Args) != 4 || !s1 || !s3 || c.Name(3) != "c" {
		t.Errorf("spread: %+v", c.Args)
	}
	if _, ok := c.Args[1].(*Spread).X.(*Ident); !ok {
		t.Errorf("spread operand: %T", c.Args[1].(*Spread).X)
	}
}

func TestCharAndFloatLiterals(t *testing.T) {
	f, errs := Parse("t.kt", `val xs = listOf('a', '\n', '\'', '\\', '\u0041', 'é', 1f, 2.5F, 1_0.0f, 1e3f)`)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	args := f.Decls[0].(*PropDecl).Init.(*Call).Args
	for i, want := range []rune{'a', '\n', '\'', '\\', 'A', 'é'} {
		if c, ok := args[i].(*CharLit); !ok || c.Val != want {
			t.Errorf("arg %d: %#v, want %q", i, args[i], want)
		}
	}
	for i, want := range []float32{1, 2.5, 10, 1000} {
		if fl, ok := args[6+i].(*FloatLit); !ok || fl.Val != want {
			t.Errorf("arg %d: %#v, want %v", 6+i, args[6+i], want)
		}
	}
}

func TestDestructuring(t *testing.T) {
	f, errs := Parse("t.kt", "fun f() {\n val (a, _, c: Int) = g()\n for ((k, v) in m) { }\n}\ndata class P(val x: Int)\ndata object O")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	stmts := f.Decls[0].(*FunDecl).Body.Stmts
	if d := stmts[0].(*Destructure); strings.Join(d.Names, ",") != "a,_,c" || d.Init == nil {
		t.Errorf("destructure: %+v", d)
	}
	if fr := stmts[1].(*For); strings.Join(fr.Names, ",") != "k,v" || fr.Var != "" {
		t.Errorf("for: %+v", fr)
	}
	if !f.Decls[1].(*ClassDecl).Data || !f.Decls[2].(*ClassDecl).Data || !f.Decls[2].(*ClassDecl).Object {
		t.Errorf("data class and object")
	}
}

func TestDestructuredLambdaParams(t *testing.T) {
	lambda := func(src string) *Lambda {
		t.Helper()
		f, errs := Parse("t.kt", "val f = "+src)
		if len(errs) > 0 {
			t.Fatal(errs[0])
		}
		return f.Decls[0].(*PropDecl).Init.(*Lambda)
	}
	params := func(l *Lambda) string {
		var ps []string
		for _, p := range l.Params {
			s := p.Name
			if p.Names != nil {
				s = "(" + strings.Join(p.Names, ",") + ")"
			}
			if p.Type != nil {
				s += ":" + p.Type.Name
			}
			ps = append(ps, s)
		}
		return strings.Join(ps, " ")
	}
	for src, want := range map[string]string{
		"{ (k, v) -> k }":                 "(k,v)",
		"{ (_, v) -> v }":                 "(_,v)",
		"{ i, (a, b) -> a }":              "i (a,b)",
		"{ (a, b), c -> a }":              "(a,b) c",
		"{ (a, b): Pair<Int, Int> -> a }": "(a,b):Pair",
		"{ (a)\n -> a }":                  "(a)",
		"{ (type, init) -> init }":        "(type,init)",
	} {
		if l := lambda(src); !l.Arrow || params(l) != want {
			t.Errorf("%q: params %q, want %q", src, params(l), want)
		}
	}
	// A body that starts with a parenthesized expression has no parameters.
	for _, src := range []string{"{ (1 + 2) * 3 }", "{ (x) }", "{ (a)(b) }"} {
		if l := lambda(src); l.Arrow || len(l.Params) != 0 || len(l.Body.Stmts) != 1 {
			t.Errorf("%q: arrow %v, %d params, %d statements", src, l.Arrow, len(l.Params), len(l.Body.Stmts))
		}
	}
}

// Annotations on lambdas: Compose's are recorded, also on a trailing
// lambda, before its label.
func TestLambdaAnnotations(t *testing.T) {
	for src, want := range map[string][2]bool{
		"val f = @Composable { }":                              {true, false},
		"val f = @DontMemoize { }":                             {false, true},
		"val f = @Suppress(\"x\") @DontMemoize { }":            {false, true},
		"fun f() { @androidx.compose.runtime.Composable { } }": {true, false},
		"val f = g @DontMemoize { }":                           {false, true},
		"val f = g(1) @Composable @DontMemoize l@{ }":          {true, true},
		"val f = a.g @DontMemoize { }":                         {false, true},
	} {
		f, errs := Parse("t.kt", src)
		if len(errs) > 0 {
			t.Errorf("%q: %v", src, errs[0])
			continue
		}
		var root any
		switch d := f.Decls[0].(type) {
		case *PropDecl:
			root = d.Init
		case *FunDecl:
			root = d.Body
		}
		var l *Lambda
		Inspect(root, func(n any) bool {
			if x, ok := n.(*Lambda); ok && l == nil {
				l = x
			}
			return true
		})
		if l == nil || [2]bool{l.Annotated, l.DontMemoize} != want {
			t.Errorf("%q: lambda %+v, want @Composable %v, @DontMemoize %v", src, l, want[0], want[1])
		}
	}
}

func TestEnums(t *testing.T) {
	f, errs := Parse("t.kt", `enum class Color(val rgb: Int) {
  @Deprecated("x") RED(0xff0000),
  GREEN(0x00ff00), BLUE(0x0000ff),
  ;
  fun hex() = rgb
}
enum class Dir { UP, DOWN }
enum class Empty { ; val x = 1 }
sealed interface Shape`)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	color, dir, empty := f.Decls[0].(*ClassDecl), f.Decls[1].(*ClassDecl), f.Decls[2].(*ClassDecl)
	if !color.Enum || len(color.Entries) != 3 || len(color.Entries[1].Call.Args) != 1 || len(color.Members) != 1 {
		t.Errorf("Color: %d entries, %d members", len(color.Entries), len(color.Members))
	}
	if len(dir.Entries) != 2 || dir.Entries[1].Name != "DOWN" || len(empty.Entries) != 0 || len(empty.Members) != 1 {
		t.Errorf("Dir, Empty: %+v %+v", dir.Entries, empty.Members)
	}
	if !f.Decls[3].(*ClassDecl).Sealed {
		t.Errorf("sealed interface")
	}
}

// TestReceiverType checks how a function type with a receiver is parsed.
func TestReceiverType(t *testing.T) {
	f, errs := Parse("t.kt", "fun f(g: List<Int>?.(Int) -> String, h: (A.() -> B)?) {}")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	ps := f.Decls[0].(*FunDecl).Params
	g := ps[0].Type
	if g.Recv == nil || g.Recv.Name != "List" || !g.Recv.Nullable || len(g.Params) != 1 || g.Ret.Name != "String" {
		t.Errorf("g: %+v", g)
	}
	h := ps[1].Type
	if h.Recv == nil || h.Recv.Name != "A" || !h.Nullable || len(h.Params) != 0 || h.Ret.Name != "B" {
		t.Errorf("h: %+v", h)
	}
}

func TestExtensionDecls(t *testing.T) {
	tests := []struct {
		src, name, recv string
		nullable        bool
	}{
		{"fun String.f() {}", "f", "String", false},
		{"fun <T> List<T>.second(): T = this[1]", "second", "List", false},
		{"fun String?.orEmpty2() = this ?: \"\"", "orEmpty2", "String", true},
		{"fun java.lang.StringBuilder.f() = this", "f", "java.lang.StringBuilder", false},
		{"fun Map<String, List<Int>>?.f(x: Int) = x", "f", "Map", true},
		{"infix fun Int.pow(e: Int): Int = e", "pow", "Int", false},
		{"operator fun Vec.plus(o: Vec) = o", "plus", "Vec", false},
		{"fun (() -> Unit).twice() { this(); this() }", "twice", "", false},
		{"fun ((Int) -> Int)?.f() = 1", "f", "", true},
		{"fun Outer.Nested.f() = 1", "f", "Outer.Nested", false},
		{"fun <T> T.f() {}", "f", "T", false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			f, errs := Parse("t.kt", tt.src)
			if len(errs) > 0 {
				t.Fatal(errs[0])
			}
			d := f.Decls[0].(*FunDecl)
			if d.Name != tt.name || d.Recv == nil || d.Recv.Name != tt.recv || d.Recv.Nullable != tt.nullable {
				t.Errorf("got name %q receiver %+v", d.Name, d.Recv)
			}
		})
	}

	props := []struct {
		src, name, recv string
		tparams         int
	}{
		{"val String.lastChar: Char get() = this[length - 1]", "lastChar", "String", 0},
		{"val String.lastChar: Char\n  get() = this[length - 1]", "lastChar", "String", 0},
		{"val String.n get() = 1", "n", "String", 0},
		{"val <T> List<T>.penultimate: T get() = this[size - 2]", "penultimate", "List", 1},
		{"val Int?.orZero = 1", "orZero", "Int", 0},
	}
	for _, tt := range props {
		t.Run(tt.src, func(t *testing.T) {
			f, errs := Parse("t.kt", tt.src)
			if len(errs) > 0 {
				t.Fatal(errs[0])
			}
			d := f.Decls[0].(*PropDecl)
			if d.Name != tt.name || d.Recv == nil || d.Recv.Name != tt.recv || len(d.TypeParams) != tt.tparams {
				t.Errorf("got name %q receiver %+v, %d type parameters", d.Name, d.Recv, len(d.TypeParams))
			}
		})
	}

	// Not extensions: the receiver scan must leave these alone.
	plain := []string{
		"fun f(x: Int) = x",
		"fun <T : Comparable<T>> max(a: T, b: T) = a",
		"val x: Map<String, Int>? = null",
		"val x = a < b",
		"var x = 1\n  get() = 2",
		"class A { val x get() = 1 }",
	}
	for _, src := range plain {
		f, errs := Parse("t.kt", src)
		if len(errs) > 0 {
			t.Errorf("%s: %v", src, errs[0])
			continue
		}
		switch d := f.Decls[0].(type) {
		case *FunDecl:
			if d.Recv != nil {
				t.Errorf("%s: unexpected receiver %+v", src, d.Recv)
			}
		case *PropDecl:
			if d.Recv != nil {
				t.Errorf("%s: unexpected receiver %+v", src, d.Recv)
			}
		}
	}
}

func TestOperatorInfixModifiers(t *testing.T) {
	f, errs := Parse("t.kt", "class V { operator fun plus(o: V) = o\n infix fun to(o: V) = o }\noperator fun V.minus(o: V) = o")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	ms := f.Decls[0].(*ClassDecl).Members
	if d := ms[0].(*FunDecl); !d.Operator || d.Infix {
		t.Errorf("plus: %+v", d.Modifiers)
	}
	if d := ms[1].(*FunDecl); d.Operator || !d.Infix {
		t.Errorf("to: %+v", d.Modifiers)
	}
	if d := f.Decls[1].(*FunDecl); !d.Operator || d.Recv == nil {
		t.Errorf("minus: %+v", d)
	}
	// Soft keywords are still names.
	if _, errs := Parse("t.kt", "fun f(operator: Int, infix: Int) = operator + infix"); len(errs) > 0 {
		t.Error(errs[0])
	}
}

// TestExpects checks that expect declarations are kept apart from the
// program's declarations.
func TestExpects(t *testing.T) {
	f, errs := Parse("t.kt", "expect fun f(): Int\nactual fun f(): Int = 1\nexpect val x: Int\nexpect class A\nfun g() = 1")
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	if len(f.Expects) != 3 || len(f.Decls) != 2 {
		t.Fatalf("got %d expects and %d declarations, want 3 and 2", len(f.Expects), len(f.Decls))
	}
	if d := f.Decls[0].(*FunDecl); !d.Actual {
		t.Errorf("f is not actual")
	}
}

// TestWhereClauses checks that a where clause's bounds are added to the
// declaration's type parameters, after the ones declared with them.
func TestWhereClauses(t *testing.T) {
	src := "fun <T : A, U> f() where T : B,\n  U : C, T : D {}\n" +
		"class K<T>(x: T) : S() where T : A, T : B { }\n" +
		"val <T> T.p: Int\n  where T : A\n  get() = 1"
	f, errs := Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	names := func(tp *TypeParam) string {
		var out []string
		for _, b := range tp.Bounds {
			out = append(out, b.Name)
		}
		return strings.Join(out, ",")
	}
	fd, cd, pd := f.Decls[0].(*FunDecl), f.Decls[1].(*ClassDecl), f.Decls[2].(*PropDecl)
	for _, c := range []struct {
		tp   *TypeParam
		want string
	}{{fd.TypeParams[0], "A,B,D"}, {fd.TypeParams[1], "C"}, {cd.TypeParams[0], "A,B"}, {pd.TypeParams[0], "A"}} {
		if got := names(c.tp); got != c.want {
			t.Errorf("%s: bounds %s, want %s", c.tp.Name, got, c.want)
		}
	}
	if fd.Body == nil || pd.Getter == nil {
		t.Errorf("lost the body after the where clause")
	}
}

// TestInlineAccessors checks that inline get and inline set are ordinary
// accessors, marked inline.
func TestInlineAccessors(t *testing.T) {
	src := "val Int.s: Boolean\n  inline get() = this > 0\n" +
		"var x: Int\n  @A inline get() = 1\n  inline set(v) {}\n" +
		"val y: Int get() = 2"
	f, errs := Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	s, x, y := f.Decls[0].(*PropDecl), f.Decls[1].(*PropDecl), f.Decls[2].(*PropDecl)
	if !s.InlineGetter || s.Getter == nil || s.Getter.Inline {
		t.Errorf("s: inline getter %v, getter %+v", s.InlineGetter, s.Getter)
	}
	if !x.InlineGetter || !x.InlineSetter || x.Setter == nil {
		t.Errorf("x: inline getter %v, inline setter %v, setter %+v", x.InlineGetter, x.InlineSetter, x.Setter)
	}
	if y.InlineGetter || y.InlineSetter {
		t.Errorf("y is inline")
	}
}

// TestSuspend checks suspend functions and suspend function types.
func TestSuspend(t *testing.T) {
	src := "suspend fun f(g: suspend () -> Unit, h: suspend Int.(String) -> Int, k: (suspend () -> Int)?) {}\n" +
		"class A { override suspend fun m() { suspend fun local() = 1 } }\n" +
		"fun suspend(suspend: Int) = suspend { suspend }"
	f, errs := Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	d := f.Decls[0].(*FunDecl)
	if !d.Suspend {
		t.Errorf("f is not suspend")
	}
	g, h, k := d.Params[0].Type, d.Params[1].Type, d.Params[2].Type
	if !g.Suspend || g.Ret.Name != "Unit" || len(g.Params) != 0 {
		t.Errorf("g: %+v", g)
	}
	if !h.Suspend || h.Recv == nil || h.Recv.Name != "Int" || len(h.Params) != 1 {
		t.Errorf("h: %+v", h)
	}
	if !k.Suspend || !k.Nullable {
		t.Errorf("k: %+v", k)
	}
	m := f.Decls[1].(*ClassDecl).Members[0].(*FunDecl)
	if !m.Suspend || !m.Override {
		t.Errorf("m: %+v", m.Modifiers)
	}
	local := m.Body.Stmts[0].(*LocalFun).Decl
	if !local.Suspend {
		t.Errorf("local: %+v", local.Modifiers)
	}
	if f.Decls[2].(*FunDecl).Suspend {
		t.Errorf("suspend is a name")
	}
}

func TestContextParameters(t *testing.T) {
	src := "context(a: A, _: B<C>)\n@Ann public inline fun <T> T.f(g: context(A) () -> Unit, h: suspend context(A, B) Int.(String) -> T) {}\n" +
		"fun context(x: Int) = context(x) { }"
	f, errs := Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	d := f.Decls[0].(*FunDecl)
	if len(d.Contexts) != 2 || d.Contexts[0].Name != "a" || d.Contexts[1].Name != "_" || d.Contexts[1].Type.Name != "B" {
		t.Errorf("contexts: %+v", d.Contexts)
	}
	if !d.Inline || d.Visibility != Public || len(d.Annotations) != 1 || d.Recv == nil {
		t.Errorf("f: %+v", d.Modifiers)
	}
	g, h := d.Params[0].Type, d.Params[1].Type
	if len(g.Contexts) != 1 || g.Contexts[0].Name != "A" || g.Ret == nil || len(g.Params) != 0 {
		t.Errorf("g: %+v", g)
	}
	if !h.Suspend || len(h.Contexts) != 2 || h.Recv == nil || h.Recv.Name != "Int" || len(h.Params) != 1 {
		t.Errorf("h: %+v", h)
	}
	if d := f.Decls[1].(*FunDecl); d.Name != "context" || len(d.Contexts) != 0 {
		t.Errorf("context is a name: %+v", d)
	}

	// Members, properties, local and anonymous functions.
	src = "class C {\n  context(a: A)\n  fun f() = 1\n  context(a: A) var <T> p: T get() = a; set(v) {}\n}\n" +
		"fun g() {\n  context(a: A) fun h() = 1\n  val k = context(a: A) fun (x: Int) = a\n  context(a)\n}"
	f, errs = Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	c := f.Decls[0].(*ClassDecl)
	if m := c.Members[0].(*FunDecl); len(m.Contexts) != 1 {
		t.Errorf("member: %+v", m.Modifiers)
	}
	if p := c.Members[1].(*PropDecl); len(p.Contexts) != 1 || len(p.TypeParams) != 1 || p.Getter == nil || p.Setter == nil {
		t.Errorf("property: %+v", p)
	}
	body := f.Decls[1].(*FunDecl).Body.Stmts
	if lf, ok := body[0].(*LocalFun); !ok || len(lf.Decl.Contexts) != 1 {
		t.Errorf("local function: %#v", body[0])
	}
	if vs, ok := body[1].(*VarStmt); !ok || len(vs.Init.(*Lambda).AnonCtx) != 1 {
		t.Errorf("anonymous function: %#v", body[1])
	}
	if es, ok := body[2].(*ExprStmt); !ok {
		t.Errorf("context(a) is a call: %#v", body[2])
	} else if _, ok := es.X.(*Call); !ok {
		t.Errorf("context(a) is a call: %#v", es.X)
	}
}

// TestSurveyGaps checks the syntax trees of constructs the parser rejected
// in the wide survey.
func TestSurveyGaps(t *testing.T) {
	src := "class A {\n var size: Int = 0\n  @JvmName(\"count\")\n  get\n  @Ann\n  private set\n}\n" +
		"fun f(x: Outer<Int>.Inner<String>) {\n g()\n { 1 }\n}"
	f, errs := Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	pd := f.Decls[0].(*ClassDecl).Members[0].(*PropDecl)
	if pd.Find("get", "JvmName") == nil || pd.Find("set", "Ann") == nil || pd.SetterVis != Private {
		t.Errorf("accessor modifiers: %+v, setter %v", pd.Annotations, pd.SetterVis)
	}
	fd := f.Decls[1].(*FunDecl)
	if ty := fd.Params[0].Type; ty.Name != "Outer.Inner" || ty.Outer == nil || ty.Outer.Name != "Outer" || len(ty.Outer.Args) != 1 || len(ty.Args) != 1 {
		t.Errorf("inner class type: %+v", ty)
	}
	if len(fd.Body.Stmts) != 1 {
		t.Fatalf("a lambda on the next line is the call's trailing lambda: %d statements", len(fd.Body.Stmts))
	}
	if c, ok := fd.Body.Stmts[0].(*ExprStmt).X.(*Call); !ok || len(c.Args) != 1 {
		t.Errorf("trailing lambda: %#v", fd.Body.Stmts[0])
	}
}

// TestRecoverAfterReceiver checks that a declaration that fails in its
// receiver type is skipped cleanly: the parser restores the dot it
// borrows, and reports the error once.
func TestRecoverAfterReceiver(t *testing.T) {
	src := "@Deprecated(\"x\")\nfun <T> A<T & Any>.f() {}\nprivate val g =\n fun (x: Int) = x\nfun ok() {}\nclass B {\n var v = 1\n  private set(value) { field = value }\n fun ok2() {}\n}"
	f, errs := ParseAll("t.kt", src)
	var features []string
	for _, e := range errs {
		features = append(features, e.Feature)
	}
	if len(features) != 0 {
		t.Errorf("errors: %v", errs)
	}
	if n := len(f.Decls); n < 2 {
		t.Errorf("declarations after the errors: %d", n)
	}
}
