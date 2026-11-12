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

// quickFixLib is a second file of the quick fix cases' module.
const quickFixLib = `package lib

class Box(val x: Int)
interface Shape {
    val area: Double
    var name: String
    fun scale(by: Double, vararg tags: String): Shape?
}
abstract class Base {
    abstract fun id(): Int
}
`

// TestQuickFixes applies a fix of a diagnostic of each case's a.kt and
// checks the result: that it is want, checks without diagnostics, and
// compiles with kc.
func TestQuickFixes(t *testing.T) {
	cfg := stdlib(t)
	if _, err := classpath.FindJDK(); err != nil {
		t.Skipf("no JDK (set JAVA_HOME): %v", err)
	}
	jar := cfg.ClassPath[0]
	for _, tc := range []struct {
		name, code, fix, src, want string
	}{
		{"import class", "UNRESOLVED_REFERENCE", "Import 'lib.Box'", `package p

fun f(): Int = Box(1).x
`, `package p

import lib.Box

fun f(): Int = Box(1).x
`},
		{"import function", "UNRESOLVED_REFERENCE", "Import 'kotlin.math.abs'", `package p

import lib.Box

fun f(b: Box) = abs(b.x)
`, `package p

import lib.Box
import kotlin.math.abs

fun f(b: Box) = abs(b.x)
`},
		{"unresolved import", "UNRESOLVED_IMPORT", "Remove unresolved import", `package p

import lib.Box
import lib.Missing

fun f() = Box(1)
`, `package p

import lib.Box

fun f() = Box(1)
`},
		{"safe call", "UNSAFE_CALL", "Replace with safe (?.) call", `package p

fun f(s: String?) = s.length
`, `package p

fun f(s: String?) = s?.length
`},
		{"asserted call", "UNSAFE_CALL", "Add non-null asserted (!!.) call", `package p

fun f(s: String?): String = s.trim()
`, `package p

fun f(s: String?): String = s!!.trim()
`},
		{"val to var", "VAL_REASSIGNMENT", "Change 'n' to var", `package p

fun f(): Int {
    val n = 1
    n = 2
    return n
}
`, `package p

fun f(): Int {
    var n = 1
    n = 2
    return n
}
`},
		{"val to var, constructor property", "VAL_REASSIGNMENT", "Change 'n' to var", `package p

class C(val n: Int)

fun f(c: C) {
    c.n = 2
}
`, `package p

class C(var n: Int)

fun f(c: C) {
    c.n = 2
}
`},
		{"implement members", "ABSTRACT_MEMBER_NOT_IMPLEMENTED", "Implement members", `package p

import lib.Shape

class Square : Shape {
    val side = 2.0
}
`, `package p

import lib.Shape

class Square : Shape {
    val side = 2.0

    override fun scale(by: Double, vararg tags: String): Shape? {
        TODO("Not yet implemented")
    }

    override val area: Double
        get() = TODO("Not yet implemented")

    override var name: String
        get() = TODO("Not yet implemented")
        set(value) {}
}
`},
		{"implement members, no body, imports", "ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED", "Implement members", `package p

object Impl : lib.Base()
`, `package p

object Impl : lib.Base() {
    override fun id(): Int {
        TODO("Not yet implemented")
    }
}
`},
		{"implement members, empty body", "ABSTRACT_MEMBER_NOT_IMPLEMENTED", "Implement members", `package p

interface I {
    fun f(x: List<String>): Map<String, Int>
}

fun g(): I {
    val o = object : I {}
    return o
}
`, `package p

interface I {
    fun f(x: List<String>): Map<String, Int>
}

fun g(): I {
    val o = object : I {
        override fun f(x: List<String>): Map<String, Int> {
            TODO("Not yet implemented")
        }
    }
    return o
}
`},
		{"implement members, enum entry", "ABSTRACT_MEMBER_NOT_IMPLEMENTED_BY_ENUM_ENTRY", "Implement members", `package p

enum class Op {
    PLUS {
        override fun apply(a: Int, b: Int) = a + b
    },
    TIMES;

    abstract fun apply(a: Int, b: Int): Int
}
`, `package p

enum class Op {
    PLUS {
        override fun apply(a: Int, b: Int) = a + b
    },
    TIMES {
        override fun apply(a: Int, b: Int): Int {
            TODO("Not yet implemented")
        }
    };

    abstract fun apply(a: Int, b: Int): Int
}
`},
		{"override many implementations", "MANY_IMPL_MEMBER_NOT_IMPLEMENTED", "Override 'f'", `package p

interface A { fun f(): Int }
interface B { fun f(): Int }
class AImpl : A { override fun f() = 1 }
class BImpl : B { override fun f() = 2 }
class C(a: A, b: B) : A by a, B by b
`, `package p

interface A { fun f(): Int }
interface B { fun f(): Int }
class AImpl : A { override fun f() = 1 }
class BImpl : B { override fun f() = 2 }
class C(a: A, b: B) : A by a, B by b {
    override fun f(): Int {
        TODO("Not yet implemented")
    }
}
`},
		{"when branches", "NO_ELSE_IN_WHEN", "Add remaining branches", `package p

enum class Color { RED, GREEN, BLUE }

fun f(c: Color): Int = when (c) {
    Color.RED -> 1
}
`, `package p

enum class Color { RED, GREEN, BLUE }

fun f(c: Color): Int = when (c) {
    Color.RED -> 1
    Color.GREEN -> TODO()
    Color.BLUE -> TODO()
}
`},
		{"when branches, sealed, one line", "NO_ELSE_IN_WHEN", "Add remaining branches", `package p

sealed class S
object O : S()
class K(val n: Int) : S()

fun f(s: S): Int = when (s) { is K -> s.n }
`, `package p

sealed class S
object O : S()
class K(val n: Int) : S()

fun f(s: S): Int = when (s) { is K -> s.n
    O -> TODO()
}
`},
		{"when else", "NO_ELSE_IN_WHEN", "Add else branch", `package p

fun f(b: Boolean): Int {
    return when (b) {
        true -> 1
    }
}
`, `package p

fun f(b: Boolean): Int {
    return when (b) {
        true -> 1
        else -> TODO()
    }
}
`},
		{"change property type", "TYPE_MISMATCH", "Change type of 'n' to 'String'", `package p

val n: Int = "one"
`, `package p

val n: String = "one"
`},
		{"change return type", "TYPE_MISMATCH", "Change return type of 'f' to 'List<String>'", `package p

fun f(): Int {
    return listOf("a")
}
`, `package p

fun f(): List<String> {
    return listOf("a")
}
`},
		{"change type, import", "TYPE_MISMATCH", "Change return type of 'f' to 'Box'", `package p

fun f(): Int = lib.Box(1)
`, `package p

import lib.Box

fun f(): Box = lib.Box(1)
`},
		{"not null", "TYPE_MISMATCH", "Add non-null asserted (!!) call", `package p

fun f(m: Map<String, Int>): Int = m["a"]
`, `package p

fun f(m: Map<String, Int>): Int = m["a"]!!
`},
		{"not null, parenthesized", "TYPE_MISMATCH", "Add non-null asserted (!!) call", `package p

fun f(a: String?, b: String?): String = a ?: b
`, `package p

fun f(a: String?, b: String?): String = (a ?: b)!!
`},
		{"add override", "VIRTUAL_MEMBER_HIDDEN", "Add 'override' modifier", `package p

open class A { open fun f() = 1 }
class B : A() { fun f() = 2 }
`, `package p

open class A { open fun f() = 1 }
class B : A() { override fun f() = 2 }
`},
		{"remove override", "NOTHING_TO_OVERRIDE", "Remove 'override' modifier", `package p

class A {
    override fun g() = 1
}
`, `package p

class A {
    fun g() = 1
}
`},
		{"add initializer", "MUST_BE_INITIALIZED_OR_BE_ABSTRACT", "Add initializer", `package p

class A {
    val n: Int
}
`, `package p

class A {
    val n: Int = 0
}
`},
		{"add initializer, nullable", "MUST_BE_INITIALIZED_OR_BE_ABSTRACT", "Add initializer", `package p

class A {
    var s: String?
}
`, `package p

class A {
    var s: String? = null
}
`},
		{"lateinit", "MUST_BE_INITIALIZED_OR_BE_ABSTRACT", "Add 'lateinit' modifier", `package p

class A {
    var s: String
}
`, `package p

class A {
    lateinit var s: String
}
`},
		{"override return type", "RETURN_TYPE_MISMATCH_ON_OVERRIDE", "Change return type to 'Int'", `package p

open class A { open fun f(): Int = 1 }
class B : A() { override fun f(): String = "" }
`, `package p

open class A { open fun f(): Int = 1 }
class B : A() { override fun f(): Int = "" }
`},
		{"unused import", UnusedImport, "Remove unused import", `package p

import lib.Box
import kotlin.math.abs

fun f() = abs(-1)
`, `package p

import kotlin.math.abs

fun f() = abs(-1)
`},
		{"unused variable", UnusedVariable, "Remove variable 'x'", `package p

fun f(): Int {
    val x = 1
    return 2
}
`, `package p

fun f(): Int {
    return 2
}
`},
		{"unused variable, initializer kept", UnusedVariable, "Remove variable 'x', keeping its initializer", `package p

fun f(l: MutableList<Int>) {
    val x = l.add(1)
}
`, `package p

fun f(l: MutableList<Int>) {
    l.add(1)
}
`},
		{"import type", "source_error", "Import 'lib.Box'", "package p\n\nfun f(b: Box) = b.x\n", "package p\n\nimport lib.Box\n\nfun f(b: Box) = b.x\n"},
		{"create class from type", "source_error", "Create class 'Thing'", "package p\n\nfun f(t: Thing) = t\n", "package p\n\nfun f(t: Thing) = t\n\nclass Thing\n"},
		{"create interface", "source_error", "Create interface 'Named'", "package p\n\nclass A : Named\n", "package p\n\nclass A : Named\n\ninterface Named\n"},
		{"create superclass", "source_error", "Create class 'Base2'", "package p\n\nclass A : Base2()\n", "package p\n\nclass A : Base2()\n\nopen class Base2\n"},
		{"create function", "UNRESOLVED_REFERENCE", "Create function 'g'", "package p\n\nfun f(s: String): Int {\n    return g(1, s)\n}\n", "package p\n\nfun f(s: String): Int {\n    return g(1, s)\n}\n\nfun g(i: Int, s: String): Int {\n    TODO(\"Not yet implemented\")\n}\n"},
		{"create unit function", "UNRESOLVED_REFERENCE", "Create function 'log'", "package p\n\nfun f(b: lib.Box) {\n    log(b, 2L)\n}\n", "package p\n\nimport lib.Box\n\nfun f(b: lib.Box) {\n    log(b, 2L)\n}\n\nfun log(b: Box, l: Long) {\n    TODO(\"Not yet implemented\")\n}\n"},
		{"create member function", "UNRESOLVED_REFERENCE", "Create member function 'A.g'", "package p\n\nclass A {\n    fun f(): Boolean = g(\"x\")\n}\n", "package p\n\nclass A {\n    fun f(): Boolean = g(\"x\")\n\n    private fun g(s: String): Boolean {\n        TODO(\"Not yet implemented\")\n    }\n}\n"},
		{"create function on receiver", "UNRESOLVED_REFERENCE", "Create member function 'A.size'", "package p\n\nclass A {\n}\n\nfun f(a: A): Int = a.size()\n", "package p\n\nclass A {\n    fun size(): Int {\n        TODO(\"Not yet implemented\")\n    }\n}\n\nfun f(a: A): Int = a.size()\n"},
		{"create class from call", "UNRESOLVED_REFERENCE", "Create class 'Point'", "package p\n\nfun f() = Point(1, 2.0)\n", "package p\n\nfun f() = Point(1, 2.0)\n\nclass Point(val i: Int, val d: Double)\n"},
		{"create local variable", "UNRESOLVED_REFERENCE", "Create local variable 'limit'", "package p\n\nfun f(): Int {\n    val x = 1\n    return limit\n}\n", "package p\n\nfun f(): Int {\n    val x = 1\n    val limit: Int = TODO()\n    return limit\n}\n"},
		{"create property", "UNRESOLVED_REFERENCE", "Create property 'enabled'", "package p\n\nfun f(): Int = if (enabled) 1 else 0\n", "package p\n\nfun f(): Int = if (enabled) 1 else 0\n\nval enabled: Boolean = TODO()\n"},
		{"add return", "NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY", "Add 'return' before the expression", "package p\n\nfun f(x: Int): Int {\n    println(x)\n    x + 1\n}\n", "package p\n\nfun f(x: Int): Int {\n    println(x)\n    return x + 1\n}\n"},
		{"initialize variable", "UNINITIALIZED_VARIABLE", "Initialize with default value", "package p\n\nfun f(): Int {\n    var x: Int\n    return x\n}\n", "package p\n\nfun f(): Int {\n    var x: Int = 0\n    return x\n}\n"},
		{"make class open", "FINAL_SUPERTYPE", "Make 'A' open", "package p\n\nclass A\n\nclass B : A()\n", "package p\n\nopen class A\n\nclass B : A()\n"},
		{"make member open", "OVERRIDING_FINAL_MEMBER", "Make 'A.f' open", "package p\n\nopen class A {\n    fun f() = 1\n}\n\nclass B : A() {\n    override fun f() = 2\n}\n", "package p\n\nopen class A {\n    open fun f() = 1\n}\n\nclass B : A() {\n    override fun f() = 2\n}\n"},
		{"make final override open", "OVERRIDING_FINAL_MEMBER", "Make 'B.id' open", "package p\n\nopen class B : lib.Base() {\n    final override fun id() = 1\n}\n\nclass C : B() {\n    override fun id() = 2\n}\n", "package p\n\nopen class B : lib.Base() {\n    override fun id() = 1\n}\n\nclass C : B() {\n    override fun id() = 2\n}\n"},
		{"add inline", "REIFIED_TYPE_PARAMETER_NO_INLINE", "Add 'inline' modifier", "package p\n\nfun <reified T> f(x: Any) = x is T\n", "package p\n\ninline fun <reified T> f(x: Any) = x is T\n"},
		{"make reified", "TYPE_PARAMETER_AS_REIFIED", "Make 'U' reified", "package p\n\ninline fun <reified T> f(x: Any) = x is T\n\nfun <U> g(x: Any) = f<U>(x)\n", "package p\n\ninline fun <reified T> f(x: Any) = x is T\n\ninline fun <reified U> g(x: Any) = f<U>(x)\n"},
		{"remove jvmstatic", "JVM_STATIC_NOT_IN_OBJECT_OR_COMPANION", "Remove '@JvmStatic' annotation", "package p\n\nclass A {\n    @JvmStatic fun f() = 1\n}\n", "package p\n\nclass A {\n    fun f() = 1\n}\n"},
		{"remove wrong target", "WRONG_ANNOTATION_TARGET", "Remove '@JvmStatic' annotation", "package p\n\n@JvmStatic\nclass A\n", "package p\n\nclass A\n"},
		{"remove repeated", "REPEATED_ANNOTATION", "Remove '@Deprecated' annotation", "package p\n\n@Deprecated(\"a\") @Deprecated(\"b\")\nfun f() = 1\n", "package p\n\n@Deprecated(\"a\") \nfun f() = 1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := applyQuickFix(t, cfg, tc.code, tc.fix, tc.src)
			if got != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", got, tc.want)
			}
			if tc.name == "override return type" {
				return // the body still doesn't match; the fix only fixes the override
			}
			s := analysis.Open(cfg)
			defer s.Close()
			snaps := []*psi.Snapshot{psi.Parse("lib.kt", quickFixLib), psi.Parse("a.kt", got)}
			res, err := s.Update(context.Background(), snaps)
			if err != nil {
				t.Fatal(err)
			}
			if ds := res.Diagnostics(); len(ds) > 0 {
				t.Fatalf("diagnostics after the fix: %v", ds)
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
		})
	}
}

// applyQuickFix analyzes src as a.kt beside quickFixLib, finds its
// diagnostic of code, and returns src with the fix titled title applied.
func applyQuickFix(t *testing.T, cfg analysis.Config, code, title, src string) string {
	t.Helper()
	s := analysis.Open(cfg)
	defer s.Close()
	snaps := []*psi.Snapshot{psi.Parse("lib.kt", quickFixLib), psi.Parse("a.kt", src)}
	res, err := s.Update(context.Background(), snaps)
	if err != nil {
		t.Fatal(err)
	}
	var own []analysis.Diagnostic
	for _, d := range res.Diagnostics() {
		if d.Path == "a.kt" {
			own = append(own, d)
		}
	}
	v := NewView(res, snaps)
	ds := append(SemanticDiagnostics(own, DiagnosticsStrict), Hints(v, "a.kt")...)
	doc := NewDocument("a.kt", 1, src)
	env := FixEnv{Diagnostics: ds, Importables: func(name string) []analysis.Importable {
		ims, err := s.Importables(snaps, "a.kt", name)
		if err != nil {
			t.Fatal(err)
		}
		return ims
	}}
	var titles []string
	for _, d := range ds {
		if d.Code != code {
			continue
		}
		for _, fx := range QuickFixes(v, doc, d, env) {
			titles = append(titles, fx.Title)
			if fx.Title != title {
				continue
			}
			edits := slices.Clone(fx.Edits)
			for _, e := range edits {
				if e.Snap.Path() != "a.kt" {
					t.Fatalf("edit of %s", e.Snap.Path())
				}
			}
			// Apply from the end; of insertions at one offset, the
			// first ends up first.
			slices.SortStableFunc(edits, func(a, b FixEdit) int { return b.Range.Start - a.Range.Start })
			out := src
			for _, e := range edits {
				out = out[:e.Range.Start] + e.Text + out[e.Range.End:]
			}
			return out
		}
	}
	var codes []string
	for _, d := range ds {
		codes = append(codes, d.Code+" "+strings.Join(d.Data, ","))
	}
	t.Fatalf("no fix %q of %s: fixes %q, diagnostics %q", title, code, titles, codes)
	return ""
}
