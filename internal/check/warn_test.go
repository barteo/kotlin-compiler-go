package check

import (
	"context"
	"fmt"
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
	ds, err := CheckOptions(context.Background(), []*syntax.File{f}, NewLibrary(cp), 1, nil, Options{Warnings: true})
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
		{"useless cast", `
fun <T> g(): T = TODO()
fun f(s: String, a: Any) {
    s as String
    a as String
    g<String>() as String
    val x = g() as String
    1 as Int
}`, []string{"4:USELESS_CAST", "6:USELESS_CAST"}},
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
	want := "11:UNCHECKED_CAST,15:UNCHECKED_CAST,2:UNCHECKED_CAST,3:UNCHECKED_CAST,4:UNCHECKED_CAST,9:UNCHECKED_CAST"
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
