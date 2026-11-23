package check

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// checkWarnings checks src against the standard library and returns its
// diagnostics, one "line:NAME" per warning and "line:error" per error,
// sorted.
func checkWarnings(t *testing.T, src string) []string {
	t.Helper()
	return checkWarningsWith(t, src, Options{Warnings: true})
}

// checkWarningsWith is checkWarnings with the options opts.
func checkWarningsWith(t *testing.T, src string, opts Options) []string {
	t.Helper()
	lib, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	jdk, err := classpath.FindJDK()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open(jdk, []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	f, errs := syntax.Parse("t.kt", src)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	ds, err := CheckOptions(context.Background(), []*syntax.File{f}, NewLibrary(cp), 1, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range ds {
		if d.IsWarning() {
			got = append(got, fmt.Sprintf("%d:%s", d.Pos.Line, d.Name))
		} else {
			got = append(got, fmt.Sprintf("%d:error", d.Pos.Line))
		}
	}
	sort.Strings(got)
	return got
}

func TestWarnings(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"not-null assertion", `
fun f(s: String, n: String?) {
    s!!.length
    n!!.length
}`, []string{"3:UNNECESSARY_NOT_NULL_ASSERTION"}},
		{"bounds in a typealias's expansion: errors but in a constructor call", `
class NumColl<T : Collection<Number>>
class Num<T : Number>
typealias NL<T2> = NumColl<List<T2>>
typealias NA<T> = Num<T>
fun f(x: NL<Any>) {}
val direct = NA<Any>()
val nested = NL<Any>()`, []string{"6:error", "7:error", "7:error", "8:UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION_DEPRECATION_WARNING"}},
		{"blank trimMargin prefix in an annotation", `
annotation class Ann(val s: String)
@Ann("".trimMargin(" ")) val a = 1`, []string{"3:TRIM_MARGIN_BLANK_PREFIX", "3:error"}},
		{"JvmOverloads beside IntroducedAt", `
@file:OptIn(ExperimentalVersionOverloading::class)
class C {
    @JvmOverloads
    fun foo(a: Int = 1, @IntroducedAt("1") b: Int = 2) { }
    @kotlin.jvm.JvmOverloads fun bar(a: Int = 1, @IntroducedAt("1") b: Int = 2) { }
}
@JvmOverloads fun top(a: Int = 1, b: Int = 2) { }`, []string{"4:CONFLICT_VERSION_AND_JVM_OVERLOADS_ANNOTATION", "6:CONFLICT_VERSION_AND_JVM_OVERLOADS_ANNOTATION"}},
		{"safe call", `
fun f(s: String, n: String?) {
    s?.length
    n?.length
}`, []string{"3:UNNECESSARY_SAFE_CALL"}},
		{"senseless comparison", `
fun f(s: String, n: String?): Boolean {
    if (s == null) return false
    if (null !== s) return true
    return n != null
}`, []string{"3:SENSELESS_COMPARISON", "4:SENSELESS_COMPARISON"}},
		{"useless elvis", `
fun f(s: String, n: String?) = (s ?: "") + (n ?: "")
`, []string{"2:USELESS_ELVIS"}},
		{"override by inline", `
interface I {
    fun foo()
    fun bar(): Int
}
class C : I {
    override inline fun foo() {}
    @Suppress("OVERRIDE_BY_INLINE")
    override inline fun bar() = 1
}
open class O : I {
    final override inline fun foo() {}
    override fun bar() = 2
    private inline fun p() {}
}`, []string{"12:NOTHING_TO_INLINE", "12:OVERRIDE_BY_INLINE", "14:NOTHING_TO_INLINE", "7:NOTHING_TO_INLINE", "7:OVERRIDE_BY_INLINE", "9:NOTHING_TO_INLINE"}},
		{"nothing to inline", `
typealias Action = () -> Unit
inline fun plain(x: Int) = x
inline fun withLambda(f: () -> Unit) = f()
inline fun withAlias(f: Action) = f()
inline fun withNoinline(noinline f: () -> Unit) = f
inline fun <reified T> reified(x: Any) = x is T
inline suspend fun susp() {}
inline fun withVararg(vararg fs: () -> Unit) {}
inline fun <T : () -> Unit> bounded(t: T) = t()
inline fun (() -> Unit).receiver() = this()
inline fun withExtType(f: Int.() -> Unit) = 1.f()
@Suppress("NOTHING_TO_INLINE")
inline fun suppressed() {}
inline val prop: Int get() = 1
inline val <reified T> T.ext: Int get() = 1
`, []string{"10:NOTHING_TO_INLINE", "11:NOTHING_TO_INLINE", "3:NOTHING_TO_INLINE", "6:NOTHING_TO_INLINE", "9:NOTHING_TO_INLINE"}},
		{"final upper bound", `
class F
open class O
enum class E { A }
class G<T>
class GO<out T>
fun <T : F> f1() {}
fun <T : O> f2() {}
fun <T : String?> f3() {}
fun <T : E> f4() {}
fun <T : G<String>> f5() {}
fun <T : GO<String>> f6() {}
fun <T> f7() where T : Int {}
interface I { fun <T : Int> m() }
class Impl : I { override fun <T : Int> m() {} }
fun <@Suppress("FINAL_UPPER_BOUND") T : F> f8() {}
class C<T : Unit>
`, []string{"12:FINAL_UPPER_BOUND", "13:FINAL_UPPER_BOUND", "14:FINAL_UPPER_BOUND", "17:FINAL_UPPER_BOUND", "7:FINAL_UPPER_BOUND"}},
		{"redundant projection", `
class GO<out T>
class GI<in T>
class G<T>
typealias TL<T> = List<T>
fun f1(a: List<out String>) {}
fun f2(b: GO<out Int>) {}
fun f3(c: GI<in Int>) {}
fun f4(d: G<out Int>) {}
fun f5(e: TL<out Int>) {}
fun f6(g: MutableList<out Int>) {}
`, []string{"10:REDUNDANT_PROJECTION", "6:REDUNDANT_PROJECTION", "7:REDUNDANT_PROJECTION", "8:REDUNDANT_PROJECTION"}},
		{"platform class mapped to kotlin", `
import java.lang.Integer
fun f1(x: java.lang.Integer) {}
fun f2(x: Integer) {}
fun f3(x: List<Int>, y: Comparable<Int>, z: Any) {}
fun f4(x: java.util.List<Int>) {}
fun f5() = java.lang.Object()
fun f6() = Any()
val v = java.lang.String::class
fun f7(x: Any) = x is java.lang.Number
`, []string{"10:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "2:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "3:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "4:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "6:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "7:PLATFORM_CLASS_MAPPED_TO_KOTLIN"}},
		{"opt-in annotation class used as an annotation", `
@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class Exp
@Exp
annotation class EAnno
@EAnno fun f1() {}
fun f2(@EAnno p: String) {}
@Exp @EAnno fun f3() {}
@OptIn(Exp::class) @EAnno class C
fun f4() {
    @EAnno fun local() {}
}
val v = @EAnno fun() {}
`, []string{"11:OPT_IN_USAGE", "13:OPT_IN_USAGE", "6:OPT_IN_USAGE", "7:OPT_IN_USAGE"}},
		{"platform class type argument", `
fun <T> id(t: T) = t
fun f1() = java.lang.Integer::class.java
fun f2() = Integer::class.javaObjectType
fun f3() = id(java.lang.Long::class.java)
fun f4() = listOf(java.lang.Integer::class)
fun f5() = Int::class.java
fun f6() = java.lang.Character::class.java.name
`, []string{"3:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "4:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "5:PLATFORM_CLASS_MAPPED_TO_KOTLIN", "8:PLATFORM_CLASS_MAPPED_TO_KOTLIN"}},
		{"no tail calls found", `
tailrec fun t1(n: Int): Int = if (n == 0) 0 else t1(n - 1)
tailrec fun t2(n: Int): Int = 1
tailrec fun t3(n: Int): Int = 1 + t3(n - 1)
tailrec fun t5(n: Int) { while (n > 0) t5(n - 1) }
class K { tailrec fun Int.t6(): Int = (this - 1).t6() }
@Suppress("x")
tailrec
fun t7(n: Int) {}
`, []string{"3:NO_TAIL_CALLS_FOUND", "4:NON_TAIL_RECURSIVE_CALL", "4:NO_TAIL_CALLS_FOUND", "5:NON_TAIL_RECURSIVE_CALL", "5:NO_TAIL_CALLS_FOUND", "8:NO_TAIL_CALLS_FOUND"}},
		{"useless cast", `
fun <T> g(): T = TODO()
fun f(s: String, a: Any) {
    s as String
    a as String
    g<String>() as String
    val x = g() as String
    1 as Int
}`, []string{"4:USELESS_CAST", "8:INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL"}},
		{"generics and platform types", `
fun <T> f(t: T, v: T, u: List<T>) {
    t?.hashCode()
    if (v == null) return
    t!!
    System.getProperty("a")!!.length
    System.getProperty("a")?.length
    u.first()!!
}`, nil},
		{"after a smart cast", `
fun f(n: String?) {
    if (n != null) n!!.length
}`, []string{"3:UNNECESSARY_NOT_NULL_ASSERTION"}},
		{"deprecation", `
@Deprecated("use g") fun old() {}
@Deprecated("no", level = DeprecationLevel.ERROR) fun gone() {}
fun f() {
    old()
    gone()
}`, []string{"5:DEPRECATION", "6:error"}},
		{"suppressed warnings", `
@Deprecated("use g") fun old() {}
@Suppress("DEPRECATION") fun f() { old() }
fun g(s: String) {
    @Suppress("UNNECESSARY_NOT_NULL_ASSERTION") val n = s!!
    @Suppress("warnings") val m = s?.length
    s!!
}
@Suppress("warnings") fun h(@Suppress("DEPRECATION") p: Unit = old()) = 1 as Int
`, []string{"7:UNNECESSARY_NOT_NULL_ASSERTION"}},
		{"error suppression", `
@Suppress("TYPE_MISMATCH")
fun f(): Int = ""
`, []string{"2:ERROR_SUPPRESSION"}},
		{"silent error suppression", `
@Deprecated("no", level = DeprecationLevel.ERROR) fun gone() {}
@Suppress("type_mismatch")
fun f(): Int = ""
@Suppress("DEPRECATION_ERROR")
fun g() = gone()
interface I {
    @Suppress("INAPPLICABLE_JVM_NAME")
    @JvmName("h")
    fun h()
}
`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkWarnings(t, tc.src)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("got\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(tc.want, "\n\t"))
			}
		})
	}
}

func TestWarningsElvisNothing(t *testing.T) {
	got := checkWarnings(t, `fun f(): String = TODO() ?: ""
fun g(s: String?) = run { s } ?: ""
`)
	if strings.Join(got, ",") != "1:USELESS_ELVIS" {
		t.Errorf("got %v, want 1:USELESS_ELVIS", got)
	}
}

func TestWarningsUncheckedCast(t *testing.T) {
	got := checkWarnings(t, `fun <T> f(a: Any, l: List<*>, c: Collection<String>, s: String, m: Any, n: Any, o: Any, p: Any, q: Any): T {
    a as List<String>
    l as List<Int>
    m as? Map<String, Int>
    n as List<*>
    o as List<Any?>
    c as List<String>
    s as Comparable<String>
    p as Array<String>
    q as (Int) -> String
    return null as T
}
inline fun <reified R> g(a: Any) = a as R
fun <T> h(a: Any, t: T?) {
    a as T
    t as T
}
`)
	want := "10:UNCHECKED_CAST,11:UNCHECKED_CAST,15:UNCHECKED_CAST,16:UNCHECKED_CAST,2:UNCHECKED_CAST,3:UNCHECKED_CAST,4:UNCHECKED_CAST,9:UNCHECKED_CAST"
	if strings.Join(got, ",") != want {
		t.Errorf("got %v, want %s", got, want)
	}
}

func TestWarningsFileSuppress(t *testing.T) {
	got := checkWarnings(t, `@file:Suppress("UNNECESSARY_SAFE_CALL")
fun f(s: String) = s?.length
`)
	if len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

// TestWarningsExperimental checks the warnings of kotlinc's experimental
// checkers, which are reported only on request.
func TestWarningsExperimental(t *testing.T) {
	src := `
fun f(x: Int) {
    println($$"plain")
    println($"x")
    println($"x$x")
    println($$"$$x")
    println($$"a$b")
    println($$"a${'$'}")
    println($$"a$` + "`b`" + `")
    println($$"$%")
    println("$%")
    println($$"""
        raw
    """)
}`
	if got := checkWarnings(t, src); len(got) != 0 {
		t.Errorf("without the option: %v, want none", got)
	}
	got := checkWarningsWith(t, src, Options{Warnings: true, ExperimentalWarnings: true})
	want := []string{"10:REDUNDANT_INTERPOLATION_PREFIX", "12:REDUNDANT_INTERPOLATION_PREFIX", "3:REDUNDANT_INTERPOLATION_PREFIX", "4:REDUNDANT_INTERPOLATION_PREFIX", "5:REDUNDANT_INTERPOLATION_PREFIX"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestWarningsCasts checks the warnings of type checks and casts whose
// outcome the types decide against those kotlinc reports of
// testdata/casts.kt.
func TestWarningsCasts(t *testing.T) {
	checkReference(t, "testdata/casts.kt", Options{Warnings: true})
}

// TestWarningsContractConds checks the null comparisons and type checks
// of contracts' conditions against those kotlinc reports of
// testdata/contractconds.kt.
func TestWarningsContractConds(t *testing.T) {
	checkReference(t, "testdata/contractconds.kt", Options{Warnings: true})
}

// TestWarningsIdentity checks the warnings of identity comparisons of
// primitives against those kotlinc reports of testdata/identity.kt.
func TestWarningsIdentity(t *testing.T) {
	checkReference(t, "testdata/identity.kt", Options{Warnings: true})
}

// TestWarningsAtomic checks the warnings of compare-and-set calls on
// atomic references of primitives and value classes against those of
// testdata/atomic.kt.
func TestWarningsAtomic(t *testing.T) {
	checkReference(t, "testdata/atomic.kt", Options{Warnings: true})
}

// TestWarningsTailrec checks the warnings of tailrec functions' calls of
// themselves against those of testdata/tailrec.kt.
func TestWarningsTailrec(t *testing.T) {
	checkReference(t, "testdata/tailrec.kt", Options{Warnings: true})
}

// TestWarningsOverrideDeprecation checks the warnings of overrides of
// deprecated members against those of testdata/overridedepr.kt.
func TestWarningsOverrideDeprecation(t *testing.T) {
	checkReference(t, "testdata/overridedepr.kt", Options{Warnings: true})
}

// TestWarningsDeadClass checks the warnings of the code of classes that
// their initialization can't run against those kotlinc reports with
// -Wextra of testdata/deadclass.kt, and that there are none without it.
func TestWarningsDeadClass(t *testing.T) {
	checkReference(t, "testdata/deadclass.kt", Options{Warnings: true, ExtraWarnings: true})
	b, err := os.ReadFile("testdata/deadclass.kt")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range checkWarningsWith(t, string(b), Options{Warnings: true}) {
		if strings.HasSuffix(g, ":UNREACHABLE_CODE") {
			t.Errorf("without ExtraWarnings: %s", g)
		}
	}
}

// TestWarningsInlineClass checks the warnings of the deprecated inline
// classes against those of testdata/inlineclass.kt.
func TestWarningsInlineClass(t *testing.T) {
	checkReference(t, "testdata/inlineclass.kt", Options{Warnings: true})
}

// TestWarningsMisplacedConstraints checks the warnings of type
// parameters' bounds declared both with them and in a where clause
// against those of testdata/misplaced.kt.
func TestWarningsMisplacedConstraints(t *testing.T) {
	checkReference(t, "testdata/misplaced.kt", Options{Warnings: true})
}

// TestWarningsReflection checks the warnings of uses of kotlin.reflect
// that need kotlin-reflect, which the tests' class path hasn't, against
// those of testdata/reflect.kt.
func TestWarningsReflection(t *testing.T) {
	checkReference(t, "testdata/reflect.kt", Options{Warnings: true})
}

// TestWarningsConversions checks the warnings of redundant conversions
// and of == of arrays against those of testdata/conversions.kt.
func TestWarningsConversions(t *testing.T) {
	checkReference(t, "testdata/conversions.kt", Options{Warnings: true})
}

// TestWarningsEmptyRange checks the warnings of empty ranges against
// those kotlinc reports with -Xuse-fir-experimental-checkers of
// testdata/emptyrange.kt, and that there are none without it, even with
// -Wextra.
func TestWarningsEmptyRange(t *testing.T) {
	checkReference(t, "testdata/emptyrange.kt", Options{Warnings: true, ExperimentalWarnings: true})
	b, err := os.ReadFile("testdata/emptyrange.kt")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range checkWarningsWith(t, string(b), Options{Warnings: true, ExtraWarnings: true}) {
		if strings.HasSuffix(g, ":EMPTY_RANGE") {
			t.Errorf("without ExperimentalWarnings: %s", g)
		}
	}
}

// TestWarningsUnused checks the warnings of unused expressions against
// those kotlinc reports of testdata/unused.kt.
func TestWarningsUnused(t *testing.T) {
	checkReference(t, "testdata/unused.kt", Options{Warnings: true})
}

// TestWarningsUnusedVar checks the warnings of unused local variables
// and assignments against those kotlinc reports with -Wextra of
// testdata/unusedvar.kt.
func TestWarningsUnusedVar(t *testing.T) {
	checkReference(t, "testdata/unusedvar.kt", Options{Warnings: true, ExtraWarnings: true})
}

// TestWarningsShadow checks the warnings of extensions that members
// shadow against kotlinc's, which testdata/shadow.kt records.
// TestWarningsErrorSuppression checks the warnings of the error names in
// the @Suppress annotations of testdata/suppress.kt against kotlinc's.
func TestWarningsErrorSuppression(t *testing.T) {
	checkReference(t, "testdata/suppress.kt", Options{Warnings: true})
}

// TestWarningsDivision checks the warnings of division by zero against
// kotlinc's of testdata/divide.kt.
func TestWarningsDivision(t *testing.T) {
	checkReference(t, "testdata/divide.kt", Options{Warnings: true})
}

// TestWarningsRedundantElse checks the warnings of redundant else
// branches against kotlinc's of testdata/when.kt.
func TestWarningsRedundantElse(t *testing.T) {
	checkReference(t, "testdata/when.kt", Options{Warnings: true})
}

// TestWarningsEnumComparison checks the warnings of comparisons of
// nullable enums against kotlinc's of testdata/enums.kt.
func TestWarningsEnumComparison(t *testing.T) {
	checkReference(t, "testdata/enums.kt", Options{Warnings: true})
}

func TestWarningsShadow(t *testing.T) {
	checkReference(t, "testdata/shadow.kt", Options{Warnings: true})
}

// TestWarningsUnreachable checks the warnings of unreachable code against
// those kotlinc reports with -Wextra of testdata/unreachable.kt, and that
// there are none without it.
func TestWarningsUnreachable(t *testing.T) {
	checkReference(t, "testdata/unreachable.kt", Options{Warnings: true, ExtraWarnings: true})
	b, err := os.ReadFile("testdata/unreachable.kt")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range checkWarningsWith(t, string(b), Options{Warnings: true}) {
		if strings.HasSuffix(g, ":UNREACHABLE_CODE") {
			t.Errorf("without ExtraWarnings: %s", g)
		}
	}
}

// checkReference checks the warnings of file, with options opts, against
// those named by the comment that ends each of its lines: each name once,
// however many times kotlinc reports it on the line.
func checkReference(t *testing.T, file string, opts Options) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for i, line := range strings.Split(string(b), "\n") {
		if _, names, ok := strings.Cut(line, " // "); ok && !strings.HasPrefix(line, "//") {
			for _, n := range strings.Fields(names) {
				want = append(want, fmt.Sprintf("%d:%s", i+1, n))
			}
		}
	}
	sort.Strings(want)
	// A line may have a warning more than once, at several columns.
	if got := slices.Compact(checkWarningsWith(t, string(b), opts)); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s: got  %v\nwant %v", file, got, want)
	}
}

// checkReferenceFiles is checkReference of file, the last of files, all
// checked together, of which the warnings of the others aren't judged.
func checkReferenceFiles(t *testing.T, opts Options, files ...string) {
	t.Helper()
	lib, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	jdk, err := classpath.FindJDK()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open(jdk, []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	var fs []*syntax.File
	var want []string
	for i, name := range files {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, errs := syntax.Parse(name, string(b))
		if len(errs) > 0 {
			t.Fatal(errs[0])
		}
		fs = append(fs, f)
		if i < len(files)-1 {
			continue
		}
		for j, line := range strings.Split(string(b), "\n") {
			if _, names, ok := strings.Cut(line, " // "); ok && !strings.HasPrefix(line, "//") {
				for _, n := range strings.Fields(names) {
					want = append(want, fmt.Sprintf("%d:%s", j+1, n))
				}
			}
		}
	}
	ds, err := CheckOptions(context.Background(), fs, NewLibrary(cp), 1, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	last := files[len(files)-1]
	for _, d := range ds {
		switch {
		case d.File != last:
		case d.IsWarning():
			got = append(got, fmt.Sprintf("%d:%s", d.Pos.Line, d.Name))
		default:
			got = append(got, fmt.Sprintf("%d:error", d.Pos.Line))
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if got = slices.Compact(got); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s: got  %v\nwant %v", last, got, want)
	}
}

// TestWarningsBackingField checks the warnings of explicit backing
// fields of their properties' types against kotlinc's of
// testdata/backingfield.kt.
func TestWarningsBackingField(t *testing.T) {
	checkReference(t, "testdata/backingfield.kt", Options{Warnings: true})
}

// TestWarningsInferredInvisible checks the warnings of inferred types
// that name a class the code can't see against kotlinc's of
// testdata/inferredvis/b.kt.
func TestWarningsInferredInvisible(t *testing.T) {
	checkReferenceFiles(t, Options{Warnings: true}, "testdata/inferredvis/a.kt", "testdata/inferredvis/b.kt")
}

func TestWarningsNullOperators(t *testing.T) {
	checkReference(t, "testdata/nullops.kt", Options{Warnings: true})
}

// TestWarningsDeprecation checks the DEPRECATION warnings of
// testdata/deprecation.kt.
func TestWarningsDeprecation(t *testing.T) {
	checkReference(t, "testdata/deprecation.kt", Options{Warnings: true})
}

// TestWarningsNullCast checks the elvis, cast, safe call and null
// condition warnings of testdata/nullcast.kt.
// TestWarningsNullSmartCasts checks the null checks and casts that the
// smart casts of is Any, of safe calls and casts, and of a value that is
// always null decide, and casts of platform types.
// TestWarningsAliasBounds checks the deprecation warnings of bounds in
// typealias expansions against kotlinc's of testdata/aliasbounds.kt.
func TestWarningsAliasBounds(t *testing.T) {
	checkReference(t, "testdata/aliasbounds.kt", Options{Warnings: true})
}

// TestWarningsEmptyIntersection checks the warnings of type arguments
// inferred as empty intersections against kotlinc's of
// testdata/emptyinter.kt.
func TestWarningsEmptyIntersection(t *testing.T) {
	checkReference(t, "testdata/emptyinter.kt", Options{Warnings: true})
}

func TestWarningsNullSmartCasts(t *testing.T) {
	checkReference(t, "testdata/nullsmart.kt", Options{Warnings: true})
}

func TestWarningsNullCast(t *testing.T) {
	checkReference(t, "testdata/nullcast.kt", Options{Warnings: true})
}

// TestWarningsModality checks the warnings of redundant modality
// modifiers against those kotlinc reports with -Wextra of
// testdata/modality.kt, and that there are none without it.
func TestWarningsModality(t *testing.T) {
	checkReference(t, "testdata/modality.kt", Options{Warnings: true, ExtraWarnings: true})
	b, err := os.ReadFile("testdata/modality.kt")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range checkWarningsWith(t, string(b), Options{Warnings: true}) {
		if strings.HasSuffix(g, ":REDUNDANT_MODALITY_MODIFIER") {
			t.Errorf("without ExtraWarnings: %s", g)
		}
	}
}

// TestWarningsParamName checks the PARAMETER_NAME_CHANGED_ON_OVERRIDE
// warnings of testdata/paramname.kt.
func TestWarningsParamName(t *testing.T) {
	checkReference(t, "testdata/paramname.kt", Options{Warnings: true})
}

// TestWarningsJavaAnnotations checks the DEPRECATED_JAVA_ANNOTATION
// warnings of testdata/javaannotations.kt.
func TestWarningsJavaAnnotations(t *testing.T) {
	checkReference(t, "testdata/javaannotations.kt", Options{Warnings: true})
}

// TestWarningsJDKList checks the deprecations of the JDK 21 members of
// the collection interfaces that K2 hides, by testdata/jdklist.kt, with a
// JDK that has them.
func TestWarningsJDKList(t *testing.T) {
	jdk, err := classpath.FindJDK()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open(jdk, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cp.Has("java/util/SequencedCollection") {
		t.Skip("the JDK is older than 21")
	}
	checkReference(t, "testdata/jdklist.kt", Options{Warnings: true})
}

// TestWarningsJavaField checks PROPERTY_HIDES_JAVA_FIELD by
// testdata/javafield.kt.
func TestWarningsJavaField(t *testing.T) {
	checkReference(t, "testdata/javafield.kt", Options{Warnings: true})
}

// TestWarningsSupertypeParamNames checks
// DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES by
// testdata/supernames.kt.
func TestWarningsSupertypeParamNames(t *testing.T) {
	checkReference(t, "testdata/supernames.kt", Options{Warnings: true})
}

// TestWarningsDelegationHides checks
// DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE by
// testdata/delegationhides.kt.
func TestWarningsDelegationHides(t *testing.T) {
	checkReference(t, "testdata/delegationhides.kt", Options{Warnings: true})
}

// TestWarningsModifiers checks the warnings of modifiers that have no
// effect or are redundant against kotlinc's of testdata/modifiers.kt.
func TestWarningsModifiers(t *testing.T) {
	checkReference(t, "testdata/modifiers.kt", Options{Warnings: true})
}

// TestWarningsDuplicateConditions checks the warnings of conditions that
// a when's branches repeat against kotlinc's of testdata/whendup.kt.
func TestWarningsDuplicateConditions(t *testing.T) {
	checkReference(t, "testdata/whendup.kt", Options{Warnings: true})
}

// TestWarningsRedundantSpread checks the warnings of spread operators in
// named arguments against kotlinc's of testdata/spread.kt.
func TestWarningsRedundantSpread(t *testing.T) {
	checkReference(t, "testdata/spread.kt", Options{Warnings: true})
}

// TestWarningsRedundantNullable checks REDUNDANT_NULLABLE against
// testdata/nullable.kt.
func TestWarningsRedundantNullable(t *testing.T) {
	checkReference(t, "testdata/nullable.kt", Options{Warnings: true})
}

// TestWarningsExprBodyReturn checks the returns of functions with an
// expression body against kotlinc's warnings of testdata/exprret.kt.
func TestWarningsExprBodyReturn(t *testing.T) {
	checkReference(t, "testdata/exprret.kt", Options{Warnings: true})
}

// TestWarningsContextualShadowing checks CONTEXTUAL_OVERLOAD_SHADOWED
// against kotlinc's warnings of testdata/ctxshadow.kt.
func TestWarningsContextualShadowing(t *testing.T) {
	checkReference(t, "testdata/ctxshadow.kt", Options{Warnings: true})
}

// TestWarningsRedundantVisibility checks REDUNDANT_VISIBILITY_MODIFIER,
// with -Wextra, against kotlinc's warnings of testdata/visibility.kt.
func TestWarningsRedundantVisibility(t *testing.T) {
	checkReference(t, "testdata/visibility.kt", Options{Warnings: true, ExtraWarnings: true})
}

// TestWarningsEqualityNotApplicable checks the comparisons of smart-cast
// values against kotlinc's warnings of testdata/equality.kt.
func TestWarningsEqualityNotApplicable(t *testing.T) {
	checkReference(t, "testdata/equality.kt", Options{Warnings: true})
}

// TestWarningsWeakerSetter checks CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING
// against kotlinc's warnings of testdata/weaken.kt.
func TestWarningsWeakerSetter(t *testing.T) {
	checkReference(t, "testdata/weaken.kt", Options{Warnings: true})
}

func TestWarningsSynchronizedOnInline(t *testing.T) {
	checkReference(t, "testdata/synchronized.kt", Options{Warnings: true})
}

func TestWarningsJavaClassOnCompanion(t *testing.T) {
	checkReference(t, "testdata/javaclass.kt", Options{Warnings: true})
}

func TestWarningsUselessCallOnNotNull(t *testing.T) {
	checkReference(t, "testdata/uselesscall.kt", Options{Warnings: true, ExtraWarnings: true})
}

func TestWarningsDangerousCharacters(t *testing.T) {
	checkReference(t, "testdata/dangerous.kt", Options{Warnings: true})
}

// TestWarningsInPlace checks the LEAKED_IN_PLACE_LAMBDA and
// WRONG_INVOCATION_KIND warnings of testdata/inplace.kt.
func TestWarningsInPlace(t *testing.T) {
	checkReference(t, "testdata/inplace.kt", Options{Warnings: true})
}

// TestWarningsLHSArgs checks the WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
// warnings, and their local classes' variant, of testdata/lhsargs.kt.
func TestWarningsLHSArgs(t *testing.T) {
	checkReference(t, "testdata/lhsargs.kt", Options{Warnings: true})
}

// TestWarningsJavaTarget checks the ANNOTATION_TARGETS_ONLY_IN_JAVA
// warnings of testdata/javatarget.kt.
func TestWarningsJavaTarget(t *testing.T) {
	checkReference(t, "testdata/javatarget.kt", Options{Warnings: true})
}

// TestWarningsOptInOverride checks the OPT_IN_OVERRIDE warnings of
// testdata/optinoverride.kt.
func TestWarningsOptInOverride(t *testing.T) {
	checkReference(t, "testdata/optinoverride.kt", Options{Warnings: true})
}

// TestWarningsUnusedParam checks the UNUSED_ANONYMOUS_PARAMETER warnings
// of testdata/unusedparam.kt.
func TestWarningsUnusedParam(t *testing.T) {
	checkReference(t, "testdata/unusedparam.kt", Options{Warnings: true, ExtraWarnings: true})
}

// TestWarningsLateinit checks the UNNECESSARY_LATEINIT warnings of
// testdata/lateinit.kt.
func TestWarningsLateinit(t *testing.T) {
	checkReference(t, "testdata/lateinit.kt", Options{Warnings: true})
}

// TestWarningsDslMarkerTarget checks the
// DSL_MARKER_APPLIED_TO_WRONG_TARGET warnings of testdata/dslmarker.kt.
func TestWarningsDslMarkerTarget(t *testing.T) {
	checkReference(t, "testdata/dslmarker.kt", Options{Warnings: true})
}

// TestWarningsAnyOverride checks the POTENTIALLY_NON_REPORTED_ANNOTATION
// warnings of testdata/anyoverride.kt.
func TestWarningsAnyOverride(t *testing.T) {
	checkReference(t, "testdata/anyoverride.kt", Options{Warnings: true})
}

// TestWarningsMultipleDefaults checks the
// MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING
// warnings of testdata/multipledefaults.kt.
func TestWarningsMultipleDefaults(t *testing.T) {
	checkReference(t, "testdata/multipledefaults.kt", Options{Warnings: true})
}

// TestWarningsVarargNothing checks the
// ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING warnings of
// testdata/varargnothing.kt.
func TestWarningsVarargNothing(t *testing.T) {
	checkReference(t, "testdata/varargnothing.kt", Options{Warnings: true})
}

// TestWarningsOrderDependent checks the
// IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT warnings of
// testdata/orderdependent.kt.
func TestWarningsOrderDependent(t *testing.T) {
	checkReference(t, "testdata/orderdependent.kt", Options{Warnings: true})
}

// TestWarningsOptInArguments checks the warnings of @OptIn annotations
// that opt in to nothing, and of subclasses of classes whose
// @SubclassOptInRequired marker has level WARNING, against kotlinc's of
// testdata/optin_args.kt.
func TestWarningsOptInArguments(t *testing.T) {
	checkReference(t, "testdata/optin_args.kt", Options{Warnings: true})
}

// TestWarningsRedundantRepeatable checks the warnings of Kotlin's
// @Repeatable beside Java's against kotlinc's of testdata/repeatable.kt.
func TestWarningsRedundantRepeatable(t *testing.T) {
	checkReference(t, "testdata/repeatable.kt", Options{Warnings: true})
}

// TestWarningsDslMarkerPropagation checks the warnings of DSL markers on
// function types with several receivers and context parameters against
// kotlinc's of testdata/dsl_many.kt.
func TestWarningsDslMarkerPropagation(t *testing.T) {
	checkReference(t, "testdata/dsl_many.kt", Options{Warnings: true})
}

// TestWarningsModifierTarget checks the warnings of open on an interface
// and of external on an enum entry or a constructor's property against
// kotlinc's of testdata/modtarget.kt.
func TestWarningsModifierTarget(t *testing.T) {
	checkReference(t, "testdata/modtarget.kt", Options{Warnings: true})
}

// TestWarningsUselessOverloads checks the warnings of @JvmOverloads that
// adds no overloads, and of @param: on a constructor parameter that is
// no property, against kotlinc's of testdata/jvmoverloads.kt.
func TestWarningsUselessOverloads(t *testing.T) {
	checkReference(t, "testdata/jvmoverloads.kt", Options{Warnings: true})
}

// TestWarningsUselessExposeBoxed checks the warnings of @JvmExposeBoxed
// on declarations without value classes against kotlinc's of
// testdata/exposeboxed.kt.
func TestWarningsUselessExposeBoxed(t *testing.T) {
	checkReference(t, "testdata/exposeboxed.kt", Options{Warnings: true})
}

// TestWarningsTrimMargin checks the warnings of trimMargin with a blank
// prefix against kotlinc's of testdata/trimmargin.kt.
func TestWarningsTrimMargin(t *testing.T) {
	checkReference(t, "testdata/trimmargin.kt", Options{Warnings: true})
}

// TestWarningsOperatorRenamed checks the warnings of operator functions
// imported under another operator's name against kotlinc's of
// testdata/oprename.kt.
func TestWarningsOperatorRenamed(t *testing.T) {
	checkReference(t, "testdata/oprename.kt", Options{Warnings: true})
}

// TestWarningsSuperQualifier checks the warnings of type arguments in an
// outer class's super qualifier against kotlinc's of
// testdata/superqualifier.kt.
func TestWarningsSuperQualifier(t *testing.T) {
	checkReference(t, "testdata/superqualifier.kt", Options{Warnings: true})
}

// TestWarningsIncompatibleTargets checks the warnings of Java's @Target
// that lacks a counterpart of Kotlin's against kotlinc's of
// testdata/javatargets.kt.
func TestWarningsIncompatibleTargets(t *testing.T) {
	checkReference(t, "testdata/javatargets.kt", Options{Warnings: true})
}

// TestWarningsContextualShadowingGeneric checks
// CONTEXTUAL_OVERLOAD_SHADOWED of overloads with type parameters against
// kotlinc's warnings of testdata/ctxshadow_generic.kt.
func TestWarningsContextualShadowingGeneric(t *testing.T) {
	checkReference(t, "testdata/ctxshadow_generic.kt", Options{Warnings: true})
}

// TestWarningsAliasExpansionDeprecated checks the warnings of typealiases
// whose expansions use deprecated classes, in types, bounds, class
// literals and constructor calls, against kotlinc's of
// testdata/alias_deprecated.kt.
func TestWarningsAliasExpansionDeprecated(t *testing.T) {
	checkReference(t, "testdata/alias_deprecated.kt", Options{Warnings: true})
}

// TestWarningsContextParameterNames checks the warnings of context
// parameters named otherwise in an override, or in two supertypes,
// against kotlinc's of testdata/ctxnames.kt.
func TestWarningsContextParameterNames(t *testing.T) {
	checkReference(t, "testdata/ctxnames.kt", Options{Warnings: true})
}

// TestWarningsUnsignedVararg checks the warnings of vararg parameters of
// unsigned types, whose arrays need opt-in, against kotlinc's of
// testdata/unsigned_vararg.kt.
func TestWarningsUnsignedVararg(t *testing.T) {
	checkReference(t, "testdata/unsigned_vararg.kt", Options{Warnings: true})
}

// TestWarningsRedundantTypes checks the warnings of a Unit return type, a
// setter parameter's type and a template of one String against those
// kotlinc reports with -Wextra of testdata/redundant_types.kt, and that
// there are none without it.
func TestWarningsRedundantTypes(t *testing.T) {
	checkReference(t, "testdata/redundant_types.kt", Options{Warnings: true, ExtraWarnings: true})
	b, err := os.ReadFile("testdata/redundant_types.kt")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range checkWarningsWith(t, string(b), Options{Warnings: true}) {
		if strings.Contains(g, ":REDUNDANT_") {
			t.Errorf("without ExtraWarnings: %s", g)
		}
	}
}

// TestWarningsSmall checks small warnings — versions newer than the API,
// clashing labels, annotations on the same line as a block-level
// expression, null when-conditions of a narrowed subject — against
// kotlinc's of testdata/smallwarn.kt.
func TestWarningsSmall(t *testing.T) {
	checkReference(t, "testdata/smallwarn.kt", Options{Warnings: true})
}
