package ide

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestRefactors applies a refactoring at each case's marker, /*|*/ for a
// caret or /*[*/ and /*]*/ around a selection, and checks the result:
// that it is want, checks without diagnostics, and compiles with kc. A
// case without want checks that the refactoring isn't offered.
func TestRefactors(t *testing.T) {
	cfg := stdlib(t)
	if _, err := classpath.FindJDK(); err != nil {
		t.Skipf("no JDK (set JAVA_HOME): %v", err)
	}
	jar := cfg.ClassPath[0]
	for _, tc := range []struct {
		name, title, src, want string
	}{
		{"specify type", "Specify type explicitly", `package p

fun f(): Int {
    val /*|*/x = listOf(lib.Box(1))
    return x.size
}
`, `package p

import lib.Box

fun f(): Int {
    val x: List<Box> = listOf(lib.Box(1))
    return x.size
}
`},
		{"specify return type", "Specify return type explicitly", `package p

fun /*|*/f(s: String) = s.length > 2
`, `package p

fun f(s: String): Boolean = s.length > 2
`},
		{"specify type, not in the initializer", "Specify type explicitly", `package p

val x = /*|*/listOf(1)
`, ""},
		{"remove type", "Remove explicit type specification", `package p

val /*|*/x: String = "a"
`, `package p

val x = "a"
`},
		{"remove type, inferred from it", "Remove explicit type specification", `package p

val /*|*/x: List<String> = emptyList()
`, ""},
		{"remove type, a supertype", "Remove explicit type specification", `package p

val /*|*/x: Any = "a"
`, ""},
		{"extract", "Extract variable", `package p

fun f(s: String): Int {
    println(s.trim()./*|*/length + 1)
    return 0
}
`, `package p

fun f(s: String): Int {
    val length = s.trim().length
    println(length + 1)
    return 0
}
`},
		{"extract selection", "Extract variable", `package p

fun f(a: Int, b: Int): Int {
    return /*[*/a * b/*]*/ + 1
}
`, `package p

fun f(a: Int, b: Int): Int {
    val i = a * b
    return i + 1
}
`},
		{"extract unique name", "Extract variable", `package p

fun f(s: String): Int {
    val length = 3
    return s./*|*/length + length
}
`, `package p

fun f(s: String): Int {
    val length = 3
    val length1 = s.length
    return length1 + length
}
`},
		{"extract a typed literal", "Extract variable", `package p

fun f(): Long {
    val y: Long = 1 + /*|*/2
    return y
}
`, `package p

fun f(): Long {
    val l: Long = 2
    val y: Long = 1 + l
    return y
}
`},
		{"extract in a branch", "Extract variable", `package p

fun f(s: String?): Int {
    if (s != null) return s./*|*/length
    return 0
}
`, ""},
		{"extract after &&", "Extract variable", `package p

fun f(s: String?): Boolean = s != null && s./*|*/length > 0
`, ""},
		{"extract in a lambda", "Extract variable", `package p

fun f(l: List<String>): List<Int> = l.map {
    it./*|*/length * 2
}
`, `package p

fun f(l: List<String>): List<Int> = l.map {
    val length = it.length
    length * 2
}
`},
		{"inline", "Inline variable 'n'", `package p

fun f(a: Int): Int {
    val /*|*/n = a + 1
    return n * 2
}
`, `package p

fun f(a: Int): Int {
    return (a + 1) * 2
}
`},
		{"inline from a use", "Inline variable 'n'", `package p

fun f(a: Int): Int {
    val n = 5
    println("$n")
    return /*|*/n + n
}
`, `package p

fun f(a: Int): Int {
    println("${5}")
    return 5 + 5
}
`},
		{"inline, two uses of a call", "Inline variable 'n'", `package p

fun f(a: String): Int {
    val /*|*/n = a.trim().length
    return n + n
}
`, ""},
		{"inline, not the next statement", "Inline variable 'n'", `package p

fun f(a: MutableList<Int>): Int {
    val /*|*/n = a.size
    a.add(1)
    return n
}
`, ""},
		{"inline a parameter's name", "Inline variable 'b'", `package p

fun f(a: Int): Int {
    val /*|*/b = a
    val g = { a: Int -> a + b }
    return g(1) + b
}
`, ""},
		{"argument names", "Add names to call arguments", `package p

fun g(a: Int, b: String, c: Boolean = false) = a
fun f() = g(1, /*|*/"x", true)
`, `package p

fun g(a: Int, b: String, c: Boolean = false) = a
fun f() = g(a = 1, b = "x", c = true)
`},
		{"argument name", "Add 'b =' to argument", `package p

fun g(a: Int, b: String, c: Boolean = false) = a
fun f() = g(1, /*|*/"x", true)
`, `package p

fun g(a: Int, b: String, c: Boolean = false) = a
fun f() = g(1, b = "x", true)
`},
		{"argument names, vararg", "Add names to call arguments", `package p

fun g(a: Int, vararg b: String) = a
fun f() = g(1, /*|*/"x", "y")
`, `package p

fun g(a: Int, vararg b: String) = a
fun f() = g(a = 1, "x", "y")
`},
		{"to expression body", "Convert to expression body", `package p

fun /*|*/f(a: Int): Int {
    return a + 1
}
`, `package p

fun f(a: Int): Int = a + 1
`},
		{"to expression body, Unit", "Convert to expression body", `package p

fun /*|*/f(a: Int) {
    println(a)
}
`, `package p

fun f(a: Int) = println(a)
`},
		{"to expression body, inferred", "Convert to expression body", `package p

fun /*|*/f(a: Int) {
    if (a > 0) println(a)
}
`, ""},
		{"to block body", "Convert to block body", `package p

fun /*|*/f(a: Int) = a + 1
`, `package p

fun f(a: Int): Int {
    return a + 1
}
`},
		{"to block body, Unit", "Convert to block body", `package p

class C {
    fun /*|*/f(a: Int) = println(a)
}
`, `package p

class C {
    fun f(a: Int) {
        println(a)
    }
}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := applyRefactor(t, cfg, tc.title, tc.src)
			if tc.want == "" {
				if ok {
					t.Fatalf("offered %q:\n%s", tc.title, got)
				}
				return
			}
			if !ok {
				t.Fatalf("no refactoring %q: %s", tc.title, got)
			}
			if got != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", got, tc.want)
			}
			checkCompiles(t, cfg, jar, got)
		})
	}
}

// checkCompiles checks that src, as a.kt beside quickFixLib, has no
// diagnostics and compiles with kc.
func checkCompiles(t *testing.T, cfg analysis.Config, jar, src string) {
	t.Helper()
	s := analysis.Open(cfg)
	defer s.Close()
	snaps := []*psi.Snapshot{psi.Parse("lib.kt", quickFixLib), psi.Parse("a.kt", src)}
	res, err := s.Update(context.Background(), snaps)
	if err != nil {
		t.Fatal(err)
	}
	if ds := res.Diagnostics(); len(ds) > 0 {
		t.Fatalf("diagnostics after the refactoring: %v", ds)
	}
	dir := t.TempDir()
	var paths []string
	for _, sn := range snaps {
		p := filepath.Join(dir, sn.Path())
		if err := os.WriteFile(p, []byte(sn.Source()), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	if err := driver.Compile(paths, driver.Options{OutDir: filepath.Join(dir, "out"), Jobs: 1, ClassPath: []string{jar}}); err != nil {
		t.Fatalf("kc: %v", err)
	}
}

// applyRefactor analyzes src, without its markers, as a.kt beside
// quickFixLib and returns it with the refactoring titled title at the
// markers applied; false and the titles offered if there is none.
func applyRefactor(t *testing.T, cfg analysis.Config, title, marked string) (string, bool) {
	t.Helper()
	var rng psi.Range
	src := marked
	if i := strings.Index(src, "/*|*/"); i >= 0 {
		src = src[:i] + src[i+5:]
		rng = psi.Range{Start: i, End: i}
	} else {
		i := strings.Index(src, "/*[*/")
		src = src[:i] + src[i+5:]
		j := strings.Index(src, "/*]*/")
		src = src[:j] + src[j+5:]
		rng = psi.Range{Start: i, End: j}
	}
	s := analysis.Open(cfg)
	defer s.Close()
	snaps := []*psi.Snapshot{psi.Parse("lib.kt", quickFixLib), psi.Parse("a.kt", src)}
	res, err := s.Update(context.Background(), snaps)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range res.Diagnostics() {
		t.Fatalf("diagnostic before the refactoring: %v", d)
	}
	v := NewView(res, snaps)
	var titles []string
	for _, fx := range Refactors(v, NewDocument("a.kt", 1, src), rng, FixEnv{}) {
		titles = append(titles, fx.Title)
		if fx.Title == title {
			return applyFixEdits(t, src, fx.Edits), true
		}
	}
	return strings.Join(titles, ", "), false
}

// applyFixEdits applies edits of a.kt to src, from the end; of insertions
// at one offset, the first ends up first.
func applyFixEdits(t *testing.T, src string, edits []FixEdit) string {
	t.Helper()
	edits = slices.Clone(edits)
	for _, e := range edits {
		if e.Snap.Path() != "a.kt" {
			t.Fatalf("edit of %s", e.Snap.Path())
		}
	}
	slices.SortStableFunc(edits, func(a, b FixEdit) int { return b.Range.Start - a.Range.Start })
	for _, e := range edits {
		src = src[:e.Range.Start] + e.Text + src[e.Range.End:]
	}
	return src
}
