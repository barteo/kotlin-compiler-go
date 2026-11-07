package ka

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// stripSnapshots returns m's files without their snapshots, which a
// whole check parses again.
func stripSnapshots(m *Model) []File {
	var out []File
	for _, f := range m.Files {
		c := *f
		c.Snapshot = nil
		out = append(out, c)
	}
	return out
}

// wholeCheck returns the model of sources from a check without the
// caches.
func wholeCheck(t testing.TB, s *Session, sources []Source) *Model {
	t.Helper()
	noReuse = true
	defer func() { noReuse = false }()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// sameModel reports how incremental model got differs from want.
func sameModel(got, want *Model) string {
	g, w := stripSnapshots(got), stripSnapshots(want)
	if len(g) != len(w) {
		return fmt.Sprintf("%d files, want %d", len(g), len(w))
	}
	for i := range g {
		if !reflect.DeepEqual(g[i], w[i]) {
			var gs, ws strings.Builder
			dump(&gs, got.Files[i].Snapshot, g[i].Decls, "")
			dump(&ws, want.Files[i].Snapshot, w[i].Decls, "")
			return fmt.Sprintf("file %s differs:\n%s\nwant:\n%s", g[i].Path, gs.String(), ws.String())
		}
	}
	if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
		return fmt.Sprintf("diagnostics %v, want %v", got.Diagnostics, want.Diagnostics)
	}
	return ""
}

func sources(texts ...string) []Source {
	var out []Source
	for i := range texts {
		out = append(out, Source{Path: fmt.Sprintf("f%d.kt", i), Text: &texts[i]})
	}
	return out
}

// An update that changes only bodies checks only the changed files and
// gives what a whole check does.
func TestIncremental(t *testing.T) {
	a := `package p
class A(val n: Int) {
    fun twice() = B().scale(n) * 2
    fun body(): Int {
        val x = n + 1
        return x
    }
    val lazyB by lazy { B() }
}
fun top() = A(1).twice()
fun unit() { println("a") }
fun len(x: Any?) = if (isStr(x)) x.length else 0
`
	b := `package p
import kotlin.contracts.contract
@OptIn(kotlin.contracts.ExperimentalContracts::class)
fun isStr(x: Any?): Boolean {
    contract { returns(true) implies (x is String) }
    return x is String
}
open class B {
    fun scale(n: Int) = n * 3
    fun use(): Long { return top().toLong() }
    val a get() = A(2)
    init { println("b") }
    constructor() { println("c") }
}
@Deprecated("x") fun err(): Int { return "s" }
`
	edits := []struct{ name, file, from, to string }{
		{"body", "a", "val x = n + 1", "val x = n + 2\n        val y = listOf(x).map { it * 2 }"},
		{"comment", "a", "class A", "/* c */ class A"},
		{"unit", "a", `println("a")`, `println("aa"); class Local { fun f() = 1 }`},
		{"accessor and init", "b", `init { println("b") }`, `init { println("b"); println(1) }`},
		{"error body", "b", `return "s"`, `return "t" + 1`},
		{"ctor", "b", `println("c")`, `val q = 1; println(q)`},
	}
	cfg := withStdlib(t)
	for _, e := range edits {
		t.Run(e.name, func(t *testing.T) {
			s := Open(cfg)
			defer s.Close()
			if _, err := s.Update(context.Background(), sources(a, b)); err != nil {
				t.Fatal(err)
			}
			a2, b2 := a, b
			if e.file == "a" {
				a2 = strings.Replace(a, e.from, e.to, 1)
			} else {
				b2 = strings.Replace(b, e.from, e.to, 1)
			}
			incrementalUpdates.Store(0)
			got, err := s.Update(context.Background(), sources(a2, b2))
			if err != nil {
				t.Fatal(err)
			}
			if incrementalUpdates.Load() != 1 {
				t.Errorf("not checked incrementally")
			}
			if d := sameModel(got, wholeCheck(t, s, sources(a2, b2))); d != "" {
				t.Error(d)
			}
		})
	}
}

// An update that changes what a file declares is checked whole.
func TestIncrementalSignature(t *testing.T) {
	a := "package p\nfun f() = 1\nfun g(): Int { return 1 }\n"
	b := "package p\nfun h() = f()\n"
	for _, a2 := range []string{
		"package p\nfun f() = 1L\nfun g(): Int { return 1 }\n",
		"package p\nfun f() = 1\nfun g(): Long { return 1 }\n",
		"package p\nfun f() = 1\nfun g(): Int { return 1 }\nfun k() {}\n",
		"package p\nfun f() = 1\nfun g(): Int { kotlin.contracts.contract { returns() }; return 1 }\n",
	} {
		s := Open(withStdlib(t))
		if _, err := s.Update(context.Background(), sources(a, b)); err != nil {
			t.Fatal(err)
		}
		incrementalUpdates.Store(0)
		got, err := s.Update(context.Background(), sources(a2, b))
		if err != nil {
			t.Fatal(err)
		}
		if incrementalUpdates.Load() != 0 {
			t.Errorf("%q checked incrementally", a2)
		}
		if d := sameModel(got, wholeCheck(t, s, sources(a2, b))); d != "" {
			t.Error(d)
		}
		s.Close()
	}
}

// bodyEdit returns text with a statement added to its first function's
// block body, or "" if it has none.
func bodyEdit(path, text string) string {
	snap := psi.Parse(path, text)
	for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
		n := snap.Node(id)
		if n.NumericKind() == psi.KindBLOCK && n.Parent().NumericKind() == psi.KindFUN {
			at := n.Range().Start + 1
			return text[:at] + "\n val kcEdit = listOf(1).map { it + 1 }\n" + text[at:]
		}
	}
	return ""
}

// TestIncrementalModule edits each function body of the module that
// KA_BENCH describes (BenchmarkUpdate), one file at a time, and compares
// each incremental update with a whole check.
func TestIncrementalModule(t *testing.T) {
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
		text := bodyEdit(srcs[i].Path, texts[i])
		if text == "" {
			continue
		}
		edit := slices.Clone(srcs)
		edit[i].Text = &text
		incrementalUpdates.Store(0)
		got, err := s.Update(context.Background(), edit)
		if err != nil {
			t.Fatal(err)
		}
		if incrementalUpdates.Load() != 1 {
			t.Errorf("%s: not checked incrementally", srcs[i].Path)
			continue
		}
		if d := sameModel(got, wholeCheck(t, s, edit)); d != "" {
			t.Errorf("%s: %s", srcs[i].Path, d)
		}
		edited++
		// Back to the module as it was, for the next edit.
		if _, err := s.Update(context.Background(), srcs); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d files edited", edited)
}

// An update that changes what a file declares checks it and the files
// that look up what it declares, and gives what a whole check does.
func TestIncrementalDecl(t *testing.T) {
	base := []string{
		"package p\nopen class A { open fun f() = 1 }\nfun top() = 1\nval tv = 2\n",
		"package p\nclass B : A() { override fun f() = 2 }\n",
		"package p\nfun useTop() = top()\nfun inferred() = tv\n",
		"package q\nimport p.*\nfun viaImport() = A().f()\n",
		"package r\nfun alone(): Int { return 3 }\n",
		"package r\nfun alone2() = alone()\n",
		"package s\nfun more() = 4\n",
		"package s\nfun more2() = more() + 1\n",
		"package p\nfun chain() = inferred()\n",
		"package t\nclass T { fun g() = 5 }\n",
		"package u\nfun solo() = 1\nclass Solo\n",
		"package u\nfun other() = 2\n",
	}
	edits := []struct {
		name     string
		file     int
		from, to string
		partial  bool
	}{
		{"return type", 0, "fun top() = 1", "fun top() = 1L", true},
		{"new function", 0, "val tv = 2\n", "val tv = 2\nfun added() = 3\n", true},
		{"member type", 0, "open fun f() = 1", "open fun f() = 1L", true},
		{"new member", 0, "open fun f() = 1 }", "open fun f() = 1\n fun g() = 0 }", true},
		{"property type", 0, "val tv = 2", "val tv = \"\"", true},
		{"shadowing", 6, "fun more() = 4", "fun more() = 4\nfun alone() = \"\"", true},
		{"removed", 4, "fun alone(): Int { return 3 }", "fun alone2x() = 1", true},
		{"supertype", 9, "class T {", "class T : p.A() {", true},
		{"class removed", 0, "open class A { open fun f() = 1 }", "", true},
		{"conflicting function", 11, "fun other() = 2", "fun other() = 2\nfun solo() = 3", true},
		{"conflicting class", 11, "fun other() = 2", "fun other() = 2\nclass Solo", true},
	}
	cfg := withStdlib(t)
	for _, e := range edits {
		t.Run(e.name, func(t *testing.T) {
			s := Open(cfg)
			defer s.Close()
			if _, err := s.Update(context.Background(), sources(base...)); err != nil {
				t.Fatal(err)
			}
			texts := slices.Clone(base)
			texts[e.file] = strings.Replace(texts[e.file], e.from, e.to, 1)
			declUpdates.Store(0)
			got, err := s.Update(context.Background(), sources(texts...))
			if err != nil {
				t.Fatal(err)
			}
			if partial := declUpdates.Load() == 1; partial != e.partial {
				t.Errorf("checked partially: %v, want %v", partial, e.partial)
			}
			if d := sameModel(got, wholeCheck(t, s, sources(texts...))); d != "" {
				t.Error(d)
			}
			// And back.
			declUpdates.Store(0)
			got, err = s.Update(context.Background(), sources(base...))
			if err != nil {
				t.Fatal(err)
			}
			if d := sameModel(got, wholeCheck(t, s, sources(base...))); d != "" {
				t.Errorf("back: %s", d)
			}
		})
	}
}

// signatureEdits returns edits of text that change what it declares: a
// function added at its end, and its first top-level callable's and first
// top-level class's names changed, if it has them.
func signatureEdits(path, text string) []string {
	edits := []string{text + "\nfun kcEdit(): Int { return 1 }\n"}
	snap := psi.Parse(path, text)
	renamed := map[bool]bool{}
	for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
		n := snap.Node(id)
		k := n.NumericKind()
		class := k == psi.KindCLASS || k == psi.KindOBJECT_DECLARATION
		if !class && k != psi.KindFUN && k != psi.KindPROPERTY || n.Parent().Kind() != "FILE" || renamed[class] {
			continue
		}
		if name := n.NameIdentifier(); name.Valid() {
			at := name.Range().End
			edits = append(edits, text[:at]+"KcEdit"+text[at:])
			renamed[class] = true
		}
	}
	return edits
}

// TestIncrementalModuleDecl makes signatureEdits to each file of the
// module that KA_BENCH describes, one at a time, and compares each update
// with a whole check. KA_BENCH_FILES limits it to that many files.
func TestIncrementalModuleDecl(t *testing.T) {
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
	limit := len(srcs)
	if n, err := strconv.Atoi(os.Getenv("KA_BENCH_FILES")); err == nil {
		limit = min(n, limit)
	}
	edits, partial := 0, 0
	check.LateInferences.Store(0)
	for i := range srcs[:limit] {
		for _, text := range signatureEdits(srcs[i].Path, texts[i]) {
			edit := slices.Clone(srcs)
			edit[i].Text = &text
			declUpdates.Store(0)
			got, err := s.Update(context.Background(), edit)
			if err != nil {
				t.Fatal(err)
			}
			partial += int(declUpdates.Load())
			want := wholeCheck(t, s, edit)
			if d := sameModel(got, want); d != "" {
				t.Errorf("%s: %s", srcs[i].Path, d)
			}
			got.Close()
			want.Close()
			edits++
			back, err := s.Update(context.Background(), srcs)
			if err != nil {
				t.Fatal(err)
			}
			back.Close()
		}
	}
	t.Logf("%d edits, %d checked partially, %d late inferences", edits, partial, check.LateInferences.Load())
}
