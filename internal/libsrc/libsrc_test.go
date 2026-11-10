package libsrc

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	text := `@file:JvmName("X")
package a.b

import c.d

@Deprecated("no, never") public inline fun <T> List<T>.first(): T = get(0)
public fun (() -> Unit).run2(): Unit = this()
public expect fun <T> listOf(element: T): List<T>
val <T> List<T>.lastIndex: Int get() = size - 1
public const val MAX = 1
public data class Pair<out A, out B>(val first: A, val second: B)
fun interface Runner { fun run() }
public typealias Num = Int
private object Single
var ` + "`weird name`" + ` = 1
    fun nested() {}
// fun commented() {}
`
	pkg, names := scan(text)
	if pkg != "a.b" {
		t.Errorf("package %q", pkg)
	}
	want := "first run2 listOf lastIndex MAX Pair Runner Num Single weird name"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("names %q, want %q", got, want)
	}
}

func writeJar(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, text := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(text))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestLocate(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "lib-1.0.jar")
	writeJar(t, jar, map[string]string{"x/Y.class": ""})
	writeJar(t, filepath.Join(dir, "lib-1.0-sources.jar"), map[string]string{
		"common/Shapes.kt": `package shapes

/** A shape. */
expect interface Shape {
    /** Its area. */
    fun area(): Double
}

/** Makes one. */
expect fun make(side: Double): Shape
`,
		"jvm/ShapesJvm.kt": `package shapes

actual interface Shape {
    actual fun area(): Double
    companion object {
        fun unit(): Shape = make(1.0)
    }
}

actual fun make(side: Double): Shape = TODO()
fun make(side: Double, name: String): Shape = TODO()
fun String.shout(): String = uppercase()
fun CharSequence.shout(): String = toString()
class Box(val item: Int) {
    class Inner
}
`,
		"java/util/Things.java": `package java.util;

public class Things<E> {
    private int count;
    public Things() {}
    public boolean add(E e) { return true; }
    public void add(int index, E e) {}
    public static class Entry {}
}
`,
	})
	f := New([]string{jar, filepath.Join(dir, "missing.jar")}, "-")
	if len(f.Archives()) != 1 {
		t.Fatalf("archives %q", f.Archives())
	}
	cases := []struct {
		t         Target
		entry, at string // the entry, and the line of the name
		doc       string
	}{
		{Target{Kind: "class", Name: "shapes.Shape"}, "jvm/ShapesJvm.kt", "actual interface Shape {", "A shape."},
		{Target{Kind: "function", Name: "shapes.Shape.area"}, "jvm/ShapesJvm.kt", "    actual fun area(): Double", "Its area."},
		{Target{Kind: "function", Name: "shapes.make", Params: true, Parameters: []string{"Double"}, Varargs: []bool{false}}, "jvm/ShapesJvm.kt", "actual fun make(side: Double): Shape = TODO()", "Makes one."},
		{Target{Kind: "function", Name: "shapes.make", Params: true, Parameters: []string{"Double", "String"}}, "jvm/ShapesJvm.kt", "fun make(side: Double, name: String): Shape = TODO()", ""},
		{Target{Kind: "function", Name: "shapes.shout", Receiver: "CharSequence"}, "jvm/ShapesJvm.kt", "fun CharSequence.shout(): String = toString()", ""},
		{Target{Kind: "function", Name: "shapes.Shape.unit"}, "jvm/ShapesJvm.kt", "        fun unit(): Shape = make(1.0)", ""},
		{Target{Kind: "property", Name: "shapes.Box.item"}, "jvm/ShapesJvm.kt", "class Box(val item: Int) {", ""},
		{Target{Kind: "constructor", Name: "shapes.Box"}, "jvm/ShapesJvm.kt", "class Box(val item: Int) {", ""},
		{Target{Kind: "class", Name: "shapes.Box.Inner"}, "jvm/ShapesJvm.kt", "    class Inner", ""},
		{Target{Kind: "function", Name: "wrong.pkg.shout"}, "jvm/ShapesJvm.kt", "fun String.shout(): String = uppercase()", ""},
		{Target{Kind: "class", Name: "java.util.Things", Binary: "java/util/Things"}, "java/util/Things.java", "public class Things<E> {", ""},
		{Target{Kind: "function", Name: "java.util.Things.add", Binary: "java/util/Things", Params: true, Parameters: []string{"int", "E"}}, "java/util/Things.java", "    public void add(int index, E e) {}", ""},
		{Target{Kind: "property", Name: "java.util.Things.count"}, "java/util/Things.java", "    private int count;", ""},
		{Target{Kind: "class", Name: "java.util.Things.Entry", Binary: "java/util/Things$Entry"}, "java/util/Things.java", "    public static class Entry {}", ""},
	}
	for _, c := range cases {
		loc, ok := f.Locate(c.t)
		if !ok {
			t.Errorf("%v: not found", c.t)
			continue
		}
		start := strings.LastIndexByte(loc.Text[:loc.Range.Start], '\n') + 1
		end := start + strings.IndexByte(loc.Text[start:], '\n')
		if loc.Entry != c.entry || loc.Text[start:end] != c.at {
			t.Errorf("%v: %s %q, want %s %q", c.t, loc.Entry, loc.Text[start:end], c.entry, c.at)
		}
		if name := c.t.Name[strings.LastIndexByte(c.t.Name, '.')+1:]; loc.Text[loc.Range.Start:loc.Range.End] != name {
			t.Errorf("%v: range %q", c.t, loc.Text[loc.Range.Start:loc.Range.End])
		}
		doc := ""
		if loc.Doc.Valid() {
			doc = loc.Doc.ChildKind(0).Text()
			for ch := loc.Doc.FirstChild(); ch.Valid(); ch = ch.NextSibling() {
				if s := ch.Text(); strings.HasPrefix(s, "/**") {
					doc = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "/**"), "*/"))
				}
			}
		}
		if doc != c.doc {
			t.Errorf("%v: doc %q, want %q", c.t, doc, c.doc)
		}
	}
	if _, ok := f.Locate(Target{Kind: "class", Name: "shapes.Missing"}); ok {
		t.Error("found a missing class")
	}
}

func TestSourcesOfGradle(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "files-2.1", "com.example", "lib", "1.0")
	os.MkdirAll(filepath.Join(v, "aaa"), 0o755)
	os.MkdirAll(filepath.Join(v, "bbb"), 0o755)
	jar := filepath.Join(v, "aaa", "lib-1.0.jar")
	os.WriteFile(jar, nil, 0o644)
	src := filepath.Join(v, "bbb", "lib-1.0-sources.jar")
	os.WriteFile(src, nil, 0o644)
	if got := SourcesOf(jar); got != src {
		t.Errorf("SourcesOf = %q, want %q", got, src)
	}
}

// TestLocalEntries checks that entries outside the archive's root are
// not indexed, so their names never reach a file system path.
func TestLocalEntries(t *testing.T) {
	for name, want := range map[string]bool{
		"a/B.kt": true, "B.kt": true, "a/./B.kt": true,
		"../B.kt": false, "a/../../B.kt": false, "/etc/B.kt": false, `a\..\..\B.kt`: false, "": false,
	} {
		if got := LocalEntry(name); got != want {
			t.Errorf("LocalEntry(%q) = %v, want %v", name, got, want)
		}
	}
	dir := t.TempDir()
	jar := filepath.Join(dir, "lib-1.0.jar")
	writeJar(t, jar, map[string]string{"x/Y.class": ""})
	writeJar(t, filepath.Join(dir, "lib-1.0-sources.jar"), map[string]string{
		"../../Evil.kt":   "package evil\n\nclass Evil\n",
		"../../Evil.java": "package evil;\n\npublic class Evil {}\n",
	})
	f := New([]string{jar}, "-")
	for _, tg := range []Target{{Kind: "class", Name: "evil.Evil"}, {Kind: "class", Name: "evil.Evil", Binary: "evil/Evil"}} {
		if loc, ok := f.Locate(tg); ok {
			t.Errorf("%v: found in %q", tg, loc.Entry)
		}
	}
}
