package ka

import (
	"context"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// shiftA has bodies before and after declarations with annotations,
// values, type parameters and a diagnostic (an unresolved type), which
// a shifted model moves.
const shiftA = `package p

annotation class Tag(val name: String, val n: Int = 0)

class A<T>(val n: Int) {
    fun plain(): Int {
        return n + 1
    }
    fun withLambda(): Int {
        return listOf(n).map { it * 2 }.sum()
    }
    fun withLocal(): Int {
        val x = n + 1
        return x
    }
    var acc: Int = 0
        get() { return field + 1 }
    init {
        println("init")
    }
    constructor(s: String) : this(s.length) {
        println(s)
    }
    @Tag("after", n = 3) fun <U : T> after(u: U): List<U> { return listOf(u) }
    val x = object { fun f(): Int { return 1 } }
}

fun top(): Int { return A<Int>(1).plain() }

@Tag(name = "q") val bad: Unresolved = TODO()

fun <R> last(r: R): R {
    return r
}
`

const shiftB = `package p
class B(val a: A<String>) {
    fun use(): Int { return a.plain() + top() }
}
`

// An update that changes a body no check looks into keeps the file's
// model, moved, and gives what a whole check does; one that changes a
// body that declares locals, an accessor's, or one in an expression is
// checked, and gives the same.
func TestShiftedBodyEdits(t *testing.T) {
	edits := []struct {
		name, from, to string
		shifted        bool
	}{
		{"statement", "return n + 1", "println(1)\n        return n + 1", true},
		{"literal", "return n + 1", "return n + 12345", true},
		{"comment", "return n + 1", "// a comment\n\n\n        return n + 1", true},
		{"lambda", "it * 2", "it * 2 + it", true},
		{"delete", "        println(\"init\")\n", "", true},
		{"ctor", "println(s)", "println(s); println(s.length)", true},
		{"last", "return r", "println(r)\n    return r", true},
		{"same line", "return listOf(u)", "return listOf(u, u)", true},
		{"top", "A<Int>(1).plain()", "A<Int>(2).plain() + 1", true},
		{"gain local", "return n + 1", "val y = 2\n        return n + y", false},
		{"edit local", "val x = n + 1", "val x = n + 2", false},
		{"lose local", "val x = n + 1\n        return x", "return n", false},
		{"local fun", "return n + 1", "fun g() = 1\n        return g()", false},
		{"accessor", "return field + 1", "return 2", false},
		{"object expression", "return 1 }", "return 2 }", false},
		{"two bodies", "return n + 1", "return n + 2", false}, // with a second change below
	}
	cfg := withStdlib(t)
	for _, e := range edits {
		t.Run(e.name, func(t *testing.T) {
			s := Open(cfg)
			defer s.Close()
			if _, err := s.Update(context.Background(), sources(shiftA, shiftB)); err != nil {
				t.Fatal(err)
			}
			a2 := strings.Replace(shiftA, e.from, e.to, 1)
			if a2 == shiftA {
				t.Fatalf("%q not found", e.from)
			}
			if e.name == "two bodies" {
				a2 = strings.Replace(a2, "return r", "return r ", 1)
			}
			shiftedUpdates.Store(0)
			got, err := s.Update(context.Background(), sources(a2, shiftB))
			if err != nil {
				t.Fatal(err)
			}
			if n := shiftedUpdates.Load(); (n == 1) != e.shifted {
				t.Errorf("shifted %d files, want shifted %v", n, e.shifted)
			}
			want := wholeCheck(t, s, sources(a2, shiftB))
			if d := sameModel(got, want); d != "" {
				t.Error(d)
			}
			if len(want.Diagnostics) == 0 {
				t.Error("no diagnostics to move")
			}
		})
	}
}

// Every body of shiftA edited in turn, one after another in one session,
// as a daemon's runs are: each update gives what a whole check does.
func TestShiftedBodySweep(t *testing.T) {
	s := Open(withStdlib(t))
	defer s.Close()
	text := shiftA
	if _, err := s.Update(context.Background(), sources(text, shiftB)); err != nil {
		t.Fatal(err)
	}
	shifted := 0
	for k := 0; ; k++ {
		snap := psi.Parse("f0.kt", text)
		var blocks []psi.NodeRef
		for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
			if n := snap.Node(id); n.NumericKind() == psi.KindBLOCK && bodyOwner(n.Parent().NumericKind()) {
				blocks = append(blocks, n)
			}
		}
		if k >= len(blocks) {
			break
		}
		for _, ins := range []string{"\n println(1)\n", " /* c */ ", "\n\n"} {
			at := blocks[k].Range().Start + 1
			text = text[:at] + ins + text[at:]
			shiftedUpdates.Store(0)
			got, err := s.Update(context.Background(), sources(text, shiftB))
			if err != nil {
				t.Fatal(err)
			}
			shifted += int(shiftedUpdates.Load())
			if d := sameModel(got, wholeCheck(t, s, sources(text, shiftB))); d != "" {
				t.Fatalf("body %d, %q: %s", k, ins, d)
			}
			snap = psi.Parse("f0.kt", text)
			for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
				if n := snap.Node(id); n.NumericKind() == psi.KindBLOCK && n.Range().Start == blocks[k].Range().Start {
					blocks[k] = n
					break
				}
			}
		}
	}
	if shifted == 0 {
		t.Error("no update shifted a file")
	}
	t.Logf("%d updates shifted a file", shifted)
}

// TestRemapNodes checks that remapNodes reaches every psi.NodeID of a
// model: each one of a copy is the original's mapped.
func TestRemapNodes(t *testing.T) {
	s := Open(withStdlib(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources(shiftA, shiftB))
	if err != nil {
		t.Fatal(err)
	}
	f := m.Files[0]
	c, ok := remapNodes(f, func(n psi.NodeID) (psi.NodeID, bool) { return n + 1000000, true })
	if !ok {
		t.Fatal("remap failed")
	}
	var orig, copied []psi.NodeID
	collectNodes(reflect.ValueOf(f), &orig, map[uintptr]bool{})
	collectNodes(reflect.ValueOf(c), &copied, map[uintptr]bool{})
	if len(orig) == 0 || len(orig) != len(copied) {
		t.Fatalf("%d nodes, copy %d", len(orig), len(copied))
	}
	for i := range orig {
		if orig[i] != 0 && copied[i] != orig[i]+1000000 {
			t.Fatalf("node %d: %d, copy %d", i, orig[i], copied[i])
		}
	}
	if c.Snapshot != f.Snapshot || !slices.Equal(c.Lookups, f.Lookups) {
		t.Error("copy's snapshot or lookups differ")
	}
}

func collectNodes(v reflect.Value, out *[]psi.NodeID, seen map[uintptr]bool) {
	if v.Type() == nodeIDType {
		*out = append(*out, psi.NodeID(v.Uint()))
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || v.Type().Elem().PkgPath() == nodeIDType.PkgPath() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		collectNodes(v.Elem(), out, seen)
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			collectNodes(v.Index(i), out, seen)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			collectNodes(v.Field(i), out, seen)
		}
	case reflect.Map, reflect.Interface:
		panic("collectNodes: " + v.Type().String())
	}
}

// TestShiftedModule edits each function body of the module that
// KA_BENCH describes with a statement that declares nothing, one file
// at a time: each update keeps the file's model, moved, and gives what
// a whole check does.
func TestShiftedModule(t *testing.T) {
	if os.Getenv("KA_BENCH") == "" {
		t.Skip("KA_BENCH is not set")
	}
	s, srcs := benchModule(t)
	defer s.Close()
	texts := make([]string, len(srcs))
	for i := range srcs {
		data, err := os.ReadFile(srcs[i].Path)
		if err != nil {
			t.Fatal(err)
		}
		texts[i] = string(data)
		srcs[i].Text = &texts[i]
	}
	if _, err := s.Update(context.Background(), srcs); err != nil {
		t.Fatal(err)
	}
	edited := 0
	for i := range srcs {
		text := texts[i]
		snap := psi.Parse(srcs[i].Path, text)
		at := -1
		for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
			if n := snap.Node(id); n.NumericKind() == psi.KindBLOCK && n.Parent().NumericKind() == psi.KindFUN {
				at = n.Range().Start + 1
				break
			}
		}
		if at < 0 {
			continue
		}
		text = text[:at] + "\n println(1)\n" + text[at:]
		edit := slices.Clone(srcs)
		edit[i].Text = &text
		shiftedUpdates.Store(0)
		got, err := s.Update(context.Background(), edit)
		if err != nil {
			t.Fatal(err)
		}
		if shiftedUpdates.Load() != 1 {
			t.Logf("%s: not shifted", srcs[i].Path)
		}
		if d := sameModel(got, wholeCheck(t, s, edit)); d != "" {
			t.Errorf("%s: %s", srcs[i].Path, d)
		}
		edited++
		if _, err := s.Update(context.Background(), srcs); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d files edited", edited)
}
