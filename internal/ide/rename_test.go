package ide

import (
	"slices"
	"strings"
	"testing"
)

const renameA = `package p

/** A box of [item]; see [Box.get]. */
open class Box<T>(open val item: T) {
    /** Returns [item]. */
    open fun get(): T = item
}

class IntBox(override val item: Int) : Box<Int>(item = item) {
    override fun get() = item
}

/**
 * Makes a box.
 * @param size the size
 */
fun make(size: Int, other: Int = 0): Box<Int> = IntBox(size + other)

fun use(): Int {
    val local = make(size = 1, other = 2).get()
    val b = make(1)
    return local + b.item + Box("x").item.length
}
`

const renameB = `package q

import p.make
import p.Box as B
import p.IntBox

fun more() = make(size = 3).get() + B(1).item + IntBox(2).item
`

// applyRename returns files' texts with the edits applied.
func applyRename(locs []Loc, text string) map[string]string {
	out := map[string]string{}
	slices.Reverse(locs) // sorted: last first per file
	for _, l := range locs {
		src, ok := out[l.Snap.Path()]
		if !ok {
			src = l.Snap.Source()
		}
		out[l.Snap.Path()] = src[:l.Range.Start] + text + src[l.Range.End:]
	}
	return out
}

func TestRename(t *testing.T) {
	v := testView(t, "a.kt", renameA, "b.kt", renameB)
	views := []*View{v}
	for _, tc := range []struct {
		path, at, name string
		want           []string // per file, substrings of the result
	}{
		{"a.kt", "val loc|al", "total", []string{"a.kt", "val total = make", "return total + b.item"}},
		{"a.kt", "fun ma|ke", "build", []string{"a.kt", "fun build(", "= build(size = 1", "val b = build(1)", "b.kt", "import p.build", "= build(size = 3)"}},
		{"a.kt", "make(si|ze: Int", "count", []string{"a.kt", "fun make(count: Int", "IntBox(count + other)", "make(count = 1, other = 2)", "@param count the size", "b.kt", "make(count = 3)"}},
		{"a.kt", "open fun g|et", "fetch", []string{"a.kt", "open fun fetch()", "override fun fetch()", "[Box.fetch]", "make(size = 1, other = 2).fetch()", "b.kt", ".fetch() + B(1)"}},
		{"a.kt", "override val it|em", "value", []string{"a.kt", "open val value: T", "override val value: Int", "Box<Int>(value = value)", "Returns [value]", "b.value", "b.kt", "B(1).value", "IntBox(2).value"}},
		{"a.kt", "class B|ox", "Crate", []string{"a.kt", "open class Crate<T>", "/** A box of [item]; see [Crate.get]. */", ": Crate<Int>(item", "): Crate<Int> =", "Crate(\"x\")", "b.kt", "import p.Crate as B", "B(1)"}},
		{"a.kt", "val b| = make", "in", []string{"a.kt", "val `in` = make(1)", "`in`.item"}},
	} {
		off := caret(t, map[string]string{"a.kt": renameA, "b.kt": renameB}[tc.path], tc.at)
		locs, text, err := Rename(views, tc.path, off, tc.name)
		if err != nil {
			t.Errorf("%s: %v", tc.at, err)
			continue
		}
		got := applyRename(locs, text)
		file := ""
		for _, w := range tc.want {
			if strings.HasSuffix(w, ".kt") && !strings.ContainsAny(w, " (") {
				file = w
				if _, ok := got[file]; !ok {
					t.Errorf("%s: %s not edited", tc.at, file)
				}
				continue
			}
			if !strings.Contains(got[file], w) {
				t.Errorf("%s: %s has no %q:\n%s", tc.at, file, w, got[file])
			}
		}
	}
}

func TestRenameRefused(t *testing.T) {
	src := `package p

class C {
    override fun toString() = "c"
    operator fun plus(o: C) = this
}

fun f(c: C) = c.toString().length + listOf(1).size
`
	v := testView(t, "a.kt", src)
	for _, at := range []string{"fun toStr|ing", "fun pl|us", "listO|f", ".si|ze"} {
		if _, _, err := Rename([]*View{v}, "a.kt", caret(t, src, at), "x"); err == nil {
			t.Errorf("%s: renamed", at)
		}
	}
	if _, _, err := Rename([]*View{v}, "a.kt", caret(t, src, "fun f|(c"), "a.b"); err == nil {
		t.Error("renamed to a.b")
	}
	r, name, err := PrepareRename([]*View{v}, "a.kt", caret(t, src, "fun f|(c"))
	if err != nil || name != "f" || src[r.Start:r.End] != "f" {
		t.Errorf("prepare: %v %q %v", r, name, err)
	}
}
