package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func formatted(t *testing.T, src string, opts FormatOptions) string {
	t.Helper()
	snap := psi.Parse("a.kt", src)
	edits, why := format(snap, opts)
	if why != "" {
		t.Fatalf("no format of\n%s\n%s", src, why)
	}
	return applyEdits(src, edits)
}

func TestFormat(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		// Indentation of blocks, bodies, when and lambdas.
		{"class A {\nfun f() {\nval x = 1\n}\n}\n", "class A {\n    fun f() {\n        val x = 1\n    }\n}\n"},
		{"fun f(x: Int) = when (x) {\n  1 -> \"a\"\n        else -> {\n  \"b\"\n }\n}\n", "fun f(x: Int) = when (x) {\n    1 -> \"a\"\n    else -> {\n        \"b\"\n    }\n}\n"},
		{"fun f() {\nlistOf(1).map { x ->\nx + 1\n}\n}\n", "fun f() {\n    listOf(1).map { x ->\n        x + 1\n    }\n}\n"},
		// Continuations: chained calls, arguments, parameters, bodies, operators.
		{"val x = listOf(1)\n.map { it }\n.filter { it > 0 }\n", "val x = listOf(1)\n    .map { it }\n    .filter { it > 0 }\n"},
		{"fun f(\na: Int,\nb: Int,\n) = g(\na,\nb\n)\n", "fun f(\n    a: Int,\n    b: Int,\n) = g(\n    a,\n    b\n)\n"},
		{"fun f() =\n1 +\n2\n", "fun f() =\n    1 +\n        2\n"},
		{"val y: Int\nget() = 1\n", "val y: Int\n    get() = 1\n"},
		{"class A(val x: Int)\n: B()\n", "class A(val x: Int)\n    : B()\n"},
		// Same indent: else, catch, annotations, closing brackets, comments.
		{"fun f(b: Boolean) =\nif (b)\n1\nelse\n2\n", "fun f(b: Boolean) =\n    if (b)\n        1\n    else\n        2\n"},
		{"fun f() {\ntry {\ng()\n}\ncatch (e: Exception) {\n}\n}\n", "fun f() {\n    try {\n        g()\n    }\n    catch (e: Exception) {\n    }\n}\n"},
		{"class A {\n@Deprecated(\"x\")\n@Suppress(\"y\")\nfun f() = 1\n}\n", "class A {\n    @Deprecated(\"x\")\n    @Suppress(\"y\")\n    fun f() = 1\n}\n"},
		{"fun f(x: Any): String {\n@Suppress(\"UNCHECKED_CAST\")\nreturn x as String\n}\n", "fun f(x: Any): String {\n    @Suppress(\"UNCHECKED_CAST\")\n    return x as String\n}\n"},
		{"class A(\nx: Int,\n) : B(), C {\nval y = 1\n}\n", "class A(\n    x: Int,\n) : B(), C {\n    val y = 1\n}\n"},
		{"fun f(a: Boolean, b: Boolean) {\nif (a &&\nb) {\ng()\n}\n}\n", "fun f(a: Boolean, b: Boolean) {\n    if (a &&\n        b) {\n        g()\n    }\n}\n"},
		{"val x = foo(1,\n2) {\nit\n}\n", "val x = foo(1,\n    2) {\n    it\n}\n"},
		{"class A {\n/** Doc. */\nfun f() = 1\n// trailing\n}\n", "class A {\n    /** Doc. */\n    fun f() = 1\n    // trailing\n}\n"},
		// Spaces.
		{"fun  f(a:Int,b :Int):Int=a+b\n", "fun  f(a: Int, b: Int): Int = a + b\n"},
		{"class A<T>:B<T>(),C {}\n", "class A<T> : B<T>(), C {}\n"},
		{"val x = listOf(1,2) .map{it*2}.filter {it>0}\n", "val x = listOf(1, 2).map { it * 2 }.filter { it > 0 }\n"},
		{"fun f(s: String ?) = s !!.length\n", "fun f(s: String?) = s!!.length\n"},
		{"val r = 1 .. 10\nval n = - 1\nval b = ! true\n", "val r = 1..10\nval n = -1\nval b = !true\n"},
		{"val f = String :: length\nval g = ::println\n", "val f = String::length\nval g = ::println\n"},
		{"fun f(a: IntArray) = if(a [0] > 0) a else null\n", "fun f(a: IntArray) = if (a[0] > 0) a else null\n"},
		{"fun f() {\n    try {\n    }catch(e: Exception) {\n    }finally{\n    }\n}\n", "fun f() {\n    try {\n    } catch (e: Exception) {\n    } finally {\n    }\n}\n"},
		{"val s = \"${ 1+2 }  x\"   // keep  this\n", "val s = \"${ 1+2 }  x\"   // keep  this\n"},
		// Lines: blank lines, trailing whitespace, the end of the file.
		{"\n\nclass A {\n\n\n\n\n    val x = 1   \n\n}\n\n\n", "class A {\n\n\n    val x = 1\n}\n"},
		{"fun f() {\n\n    val x = 1\n\n\n    val y = 2\n}", "fun f() {\n    val x = 1\n\n    val y = 2\n}\n"},
	} {
		if got := formatted(t, tc.src, FormatOptions{}); got != tc.want {
			t.Errorf("formatted\n%s\ngot\n%s\nwant\n%s", tc.src, got, tc.want)
		}
	}
	if got := formatted(t, "class A {\nfun f() = 1\n}\n", FormatOptions{TabSize: 2}); got != "class A {\n  fun f() = 1\n}\n" {
		t.Errorf("tab size 2: got\n%s", got)
	}
	if got := formatted(t, "class A {\nfun f() = 1\n}\n", FormatOptions{UseTabs: true}); got != "class A {\n\tfun f() = 1\n}\n" {
		t.Errorf("tabs: got\n%s", got)
	}
	if e := Format(psi.Parse("a.kt", "fun f( {\n"), FormatOptions{}); e != nil {
		t.Errorf("edits of a file with syntax errors: %v", e)
	}
}

// corpus returns the paths of the .kt files under testdata, the psi
// corpus (psi/testdata/corpus.sum).
func corpus(t testing.TB) []string {
	var paths []string
	err := filepath.WalkDir("../../testdata", func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".kt") {
			paths = append(paths, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// TestFormatCorpus formats every corpus file: the formatter keeps the
// tokens (format checks it), and formatting the result changes nothing.
func TestFormatCorpus(t *testing.T) {
	var files, changed, skipped int
	for _, p := range corpus(t) {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		snap := psi.Parse(filepath.Base(p), string(src))
		if len(snap.Diagnostics()) > 0 {
			skipped++
			continue
		}
		files++
		edits, why := format(snap, FormatOptions{})
		if why != "" {
			t.Errorf("%s: %s", p, why)
			continue
		}
		if len(edits) == 0 {
			continue
		}
		changed++
		out := applyEdits(string(src), edits)
		if again, why := format(psi.Parse(snap.Path(), out), FormatOptions{}); len(again) > 0 || why != "" {
			e := again[0]
			t.Errorf("%s: formatting isn't idempotent: %q at %d (%s)", p, out[max(0, e.Range.Start-30):e.Range.End], e.Range.Start, e.Text)
		}
	}
	t.Logf("%d files formatted, %d changed, %d with syntax errors skipped", files, changed, skipped)
}
