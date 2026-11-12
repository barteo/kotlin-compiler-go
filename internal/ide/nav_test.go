package ide

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// stdlib returns a configuration with the standard library, or skips t.
func stdlib(t testing.TB) analysis.Config {
	t.Helper()
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skipf("no kotlin-stdlib.jar (set KOTLIN_HOME): %v", err)
	}
	return analysis.Config{NoJDK: true, ClassPath: []string{jar}}
}

// testView returns the view of an update of files, path and text by
// pairs.
func testView(t *testing.T, files ...string) *View {
	t.Helper()
	var snaps []*psi.Snapshot
	for i := 0; i < len(files); i += 2 {
		snaps = append(snaps, psi.Parse(files[i], files[i+1]))
	}
	s := analysis.Open(stdlib(t))
	t.Cleanup(func() { s.Close() })
	res, err := s.Update(context.Background(), snaps)
	if err != nil {
		t.Fatal(err)
	}
	if d := res.Diagnostics(); len(d) > 0 {
		t.Fatalf("diagnostics: %v", d)
	}
	return NewView(res, snaps)
}

// caret returns the offset of the '|' in needle, which must occur once
// in text with it removed.
func caret(t *testing.T, text, needle string) int {
	t.Helper()
	i := strings.Index(needle, "|")
	plain := needle[:i] + needle[i+1:]
	j := strings.Index(text, plain)
	if j < 0 || strings.Count(text, plain) != 1 {
		t.Fatalf("%q occurs %d times", plain, strings.Count(text, plain))
	}
	return j + i
}

// locs formats locations as file:line:col "text".
func locs(ls []Loc) []string {
	var out []string
	for _, l := range ls {
		line, col, _ := l.Snap.Position(l.Range.Start)
		out = append(out, fmt.Sprintf("%s:%d:%d %s", l.Snap.Path(), line, col, l.Snap.Source()[l.Range.Start:l.Range.End]))
	}
	return out
}

const navA = `package p

open class Box<T>(val item: T) {
    open fun get(): T = item
    operator fun plus(other: Box<T>) = this
}

class IntBox(i: Int) : Box<Int>(i) {
    override fun get() = 2
}

fun use(b: Box<String>?, n: Int): String {
    val x = b?.get()
    var y: List<Int> = listOf(1, n)
    y = y + 2
    y += 3
    if (b != null) println(b.item)
    return x ?: "" + IntBox(3).get() + (IntBox(1) + IntBox(2)).item
}
`

const navB = `package p

fun other(b: Box<Int>) = b.item + b.get()
`

func TestNavigation(t *testing.T) {
	v := testView(t, "a.kt", navA, "b.kt", navB)
	src := map[string]string{"a.kt": navA, "b.kt": navB}
	for _, c := range []struct {
		name, path, at string
		def, typ       []string
		refs           []string // with the declaration
	}{
		{
			name: "property from another file", path: "b.kt", at: "b.it|em",
			def:  []string{"a.kt:3:23 item"},
			typ:  nil, // Box<Int>'s: kotlin.Int
			refs: []string{"a.kt:3:23 item", "a.kt:4:25 item", "a.kt:17:30 item", "a.kt:18:64 item", "b.kt:3:28 item"},
		},
		{
			name: "class", path: "a.kt", at: "class B|ox",
			def:  []string{"a.kt:3:12 Box"},
			typ:  []string{"a.kt:3:12 Box"},
			refs: []string{"a.kt:3:12 Box", "a.kt:5:30 Box", "a.kt:8:24 Box", "a.kt:12:12 Box", "b.kt:3:14 Box"},
		},
		{
			name: "override", path: "a.kt", at: "(3).g|et()",
			def:  []string{"a.kt:9:18 get"},
			typ:  nil, // kotlin.Int is the library's
			refs: []string{"a.kt:9:18 get", "a.kt:18:32 get"},
		},
		{
			name: "operator", path: "a.kt", at: "IntBox(1) |+ IntBox(2)",
			def:  []string{"a.kt:5:18 plus"},
			typ:  []string{"a.kt:3:12 Box"},
			refs: []string{"a.kt:5:18 plus", "a.kt:18:51 +"},
		},
		{
			name: "local", path: "a.kt", at: "y = |y + 2",
			def:  []string{"a.kt:14:9 y"},
			refs: []string{"a.kt:14:9 y", "a.kt:15:5 y", "a.kt:15:9 y", "a.kt:16:5 y"},
		},
		{
			name: "type parameter", path: "a.kt", at: "get(): |T",
			def:  []string{"a.kt:3:16 T"},
			typ:  []string{"a.kt:3:16 T"},
			refs: []string{"a.kt:3:16 T", "a.kt:3:29 T", "a.kt:4:21 T", "a.kt:5:34 T"},
		},
		{
			name: "library", path: "a.kt", at: "print|ln",
		},
	} {
		off := caret(t, src[c.path], c.at)
		if got := locs(Definition(v, c.path, off)); !slices.Equal(got, c.def) {
			t.Errorf("%s: definition %q, want %q", c.name, got, c.def)
		}
		if got := locs(TypeDefinition(v, c.path, off)); !slices.Equal(got, c.typ) {
			t.Errorf("%s: type definition %q, want %q", c.name, got, c.typ)
		}
		if got := locs(References([]*View{v}, c.path, off, true)); !slices.Equal(got, c.refs) {
			t.Errorf("%s: references %q, want %q", c.name, got, c.refs)
		}
	}
	var hs []string
	for _, h := range Highlights(v, "a.kt", caret(t, navA, "var |y")) {
		line, col, _ := v.Snapshot("a.kt").Position(h.Range.Start)
		hs = append(hs, fmt.Sprintf("%d:%d %d", line, col, h.Kind))
	}
	if want := []string{"14:9 2", "15:5 2", "15:9 1", "16:5 2"}; !slices.Equal(hs, want) {
		t.Errorf("highlights %q, want %q", hs, want)
	}
}

func TestMapper(t *testing.T) {
	m := NewMapper("abc def ghi", "abc XYZW ghi")
	for _, c := range []struct {
		r    psi.Range
		want psi.Range
		ok   bool
	}{
		{psi.Range{Start: 0, End: 3}, psi.Range{Start: 0, End: 3}, true},
		{psi.Range{Start: 8, End: 11}, psi.Range{Start: 9, End: 12}, true},
		{psi.Range{Start: 4, End: 7}, psi.Range{}, false},
	} {
		if got, ok := m.Range(c.r); got != c.want || ok != c.ok {
			t.Errorf("Range(%v) = %v, %v; want %v, %v", c.r, got, ok, c.want, c.ok)
		}
	}
	if off, ok := m.Back(10); off != 9 || !ok {
		t.Errorf("Back(10) = %d, %v", off, ok)
	}
	if _, ok := m.Back(5); ok {
		t.Error("Back(5) mapped into the change")
	}
}
