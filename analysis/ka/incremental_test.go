package ka

import (
	"context"
	"encoding/json"
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
			return fmt.Sprintf("file %s differs:\n%s\nwant:\n%s\n%s", g[i].Path, gs.String(), ws.String(), firstDiff(g[i], w[i]))
		}
	}
	if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
		return fmt.Sprintf("diagnostics %v, want %v", got.Diagnostics, want.Diagnostics)
	}
	return ""
}

// firstDiff returns the lines around the first line of a and b's JSON
// that differs.
func firstDiff(a, b any) string {
	ja, err1 := json.MarshalIndent(a, "", " ")
	jb, err2 := json.MarshalIndent(b, "", " ")
	if err1 != nil || err2 != nil {
		return fmt.Sprint(err1, err2)
	}
	la, lb := strings.Split(string(ja), "\n"), strings.Split(string(jb), "\n")
	for i := 0; i < min(len(la), len(lb)); i++ {
		if la[i] != lb[i] {
			from := max(0, i-15)
			return fmt.Sprintf("at line %d:\n%s\n<<< got %s\n>>> want %s", i, strings.Join(la[from:i], "\n"), la[i], lb[i])
		}
	}
	return fmt.Sprintf("%d lines, want %d", len(la), len(lb))
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

// KSP's rounds alternate between the sources and the sources with what
// the processors generated: a body edit's update in each round is checked
// against the last run's check of the same files, not the round before.
func TestIncrementalRounds(t *testing.T) {
	a := "package p\nclass A { fun f(): Int { return 1 } }\n"
	b := "package p\nclass B(val a: A)\n"
	gen := "package p\nclass BAdapter { fun make(a: A) = B(a) }\n"
	a2 := "package p\nclass A { fun f(): Int { return 2 } }\n"
	s := Open(withStdlib(t))
	defer s.Close()
	for _, srcs := range [][]Source{sources(a, b), sources(a, b, gen)} {
		if _, err := s.Update(context.Background(), srcs); err != nil {
			t.Fatal(err)
		}
	}
	for _, srcs := range [][]Source{sources(a2, b), sources(a2, b, gen)} {
		incrementalUpdates.Store(0)
		got, err := s.Update(context.Background(), srcs)
		if err != nil {
			t.Fatal(err)
		}
		if incrementalUpdates.Load() != 1 {
			t.Errorf("%d files: not checked incrementally", len(srcs))
		}
		if d := sameModel(got, wholeCheck(t, s, srcs)); d != "" {
			t.Error(d)
		}
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
		"package v\nclass V { fun m() = 1 }\nfun useExt() = V().ext()\nfun useExtVal() = V().extVal\n",
		"package v\nfun other3() = 0\n",
		"package w\nfun useErr(x: Missing) = x.wext()\n",
		"package w\nfun other4() = 0\n",
		"package x\nfun base() = 1\nval derived = base()\nclass Holder { val h = derived }\nfun unrelated() = 0\n",
		"package x\nfun useDerived() = derived\nfun useHolder() = Holder().h\n",
		"package x\nfun useUnrelated() = unrelated()\n",
		"package y\ntypealias Id = Int\nclass Box(val id: Id)\nclass Other\n",
		"package y\nfun useBox(b: Box) = b.id\nfun useOther() = Other()\n",
		// Files a check of only one reaches through names alone: an
		// inferred type of another file, and an operator no file names.
		"package z\nclass Deep { fun deep() = 1 }\n",
		"package z\nfun makeDeep() = Deep()\n",
		"package z\nfun useDeep() = makeDeep().deep()\nfun zz() = 0\n",
		"package o\nclass Vec(val x: Int)\n",
		"package o\noperator fun Vec.plus(o: Vec) = Vec(x + o.x)\n",
		"package o\nfun sum(a: Vec, b: Vec) = a + b\nfun oo() = 0\n",
		// Only Near is reached in its file: Far's isn't resolved.
		"package m2\nclass Near { fun near() = farValue() }\nclass Unused(val f: Far)\n",
		"package m2\nfun farValue() = 1L\n",
		"package m2\nclass Far { fun far() = 2 }\n",
		"package m2\nfun useNear() = Near().near()\nfun mm() = 0\n",
		// A sealed class's subclass no file names, which a when counts.
		"package sl\nsealed class S\nclass A1 : S()\nfun pick(s: S) = when (s) { is A1 -> 1 }\nfun sl0() = 0\n",
		"package sl\nclass B1 : S()\n",
		// A package only a file the check doesn't reach declares.
		"package qq\nfun qq1() = 1\n",
		"package pp\nimport qq.*\nfun pp0() = 0\n",
		// Two facades of one name. The checker reports no clash, so this
		// only shows that declStub, which never stubs them, stays right.
		"@file:JvmName(\"Facade\")\npackage jj\nfun jj1() = 1\n",
		"@file:JvmName(\"Facade\")\npackage jj\nfun jj0() = 0\n",
	}
	// checked, if not 0, is how many files the update's last check checks:
	// the changed one, those that look up what changed in it, and those
	// whose models change with them, and no more.
	edits := []struct {
		name     string
		file     int
		from, to string
		partial  bool
		checked  int
	}{
		{"return type", 0, "fun top() = 1", "fun top() = 1L", true, 0},
		{"new function", 0, "val tv = 2\n", "val tv = 2\nfun added() = 3\n", true, 0},
		{"member type", 0, "open fun f() = 1", "open fun f() = 1L", true, 0},
		{"new member", 0, "open fun f() = 1 }", "open fun f() = 1\n fun g() = 0 }", true, 0},
		{"property type", 0, "val tv = 2", "val tv = \"\"", true, 0},
		{"shadowing", 6, "fun more() = 4", "fun more() = 4\nfun alone() = \"\"", true, 0},
		{"removed", 4, "fun alone(): Int { return 3 }", "fun alone2x() = 1", true, 0},
		{"supertype", 9, "class T {", "class T : p.A() {", true, 0},
		{"class removed", 0, "open class A { open fun f() = 1 }", "", true, 0},
		{"conflicting function", 11, "fun other() = 2", "fun other() = 2\nfun solo() = 3", true, 0},
		{"conflicting class", 11, "fun other() = 2", "fun other() = 2\nclass Solo", true, 0},
		{"new extension", 13, "fun other3() = 0", "fun other3() = 0\nfun V.ext() = 2", true, 0},
		{"extension on an error type", 15, "fun other4() = 0", "fun other4() = 0\nfun Any.wext() = 1", true, 0},
		{"new extension property", 13, "fun other3() = 0", "fun other3() = 0\nval V.extVal get() = \"\"", true, 0},
		// Declarations whose texts don't change, but whose models do with
		// what they use in their file.
		{"inferred from the same file", 16, "fun base() = 1", "fun base() = 1L", true, 2},
		{"alias in the same file", 19, "typealias Id = Int", "typealias Id = Long", true, 2},
		// Every node after the edit moves: the other declarations' models
		// are the same.
		{"declarations moved", 16, "fun base() = 1", "fun first() = 0\nfun base() = 1", true, 1},
		{"unused function", 16, "fun unrelated() = 0", "fun unrelated() = 0\nfun last() = 0", true, 1},
		{"unrelated change", 16, "fun unrelated() = 0", "fun unrelated() = 0L", true, 2},
		{"import", 3, "import p.*", "import p.*\nimport r.alone", true, 1},
		{"reached by an inferred type", 23, "fun zz() = 0", "fun zz() = 0L", true, 1},
		{"reached by an operator", 26, "fun oo() = 0", "fun oo() = 0L", true, 1},
		{"reached by a declaration", 30, "fun mm() = 0", "fun mm() = 0L", true, 1},
		{"sealed subclass", 31, "fun sl0() = 0", "fun sl0() = 0L", true, 1},
		{"package of a stub", 34, "fun pp0() = 0", "fun pp0() = 0L", true, 1},
		{"facade name", 36, "fun jj0() = 0", "fun jj0() = 0L", true, 1},
	}
	// resolved is, for some edits, how many files the update's last
	// check resolves the signatures of (reachable).
	resolved := map[string]int64{
		"reached by an inferred type": 5, // its own three, o's operator and Vec
		"reached by an operator":      3,
		"reached by a declaration":    5, // its own, Near's, farValue's, o's operator and Vec
		"sealed subclass":             4, // its own, B1's, o's operator and Vec
		"package of a stub":           3,
		"unused function":             3, // and o's operator and Vec
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
			if n := lastChecked.Load(); e.checked != 0 && n != int64(e.checked) {
				t.Errorf("checked %d files, want %d", n, e.checked)
			}
			if n, want := lastResolved.Load(), resolved[e.name]; want != 0 && n != want {
				t.Errorf("resolved %d files, want %d", n, want)
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

// An update that checks only some files changes the reachable index the
// update before kept for the same files: its entries stay those of a new
// index (verifyReach), and its models a whole check's. The edits of
// TestIncrementalDecl are made one after another, on one session.
func TestReachIndexKept(t *testing.T) {
	base := []string{
		"package p\nopen class A { open fun f() = 1 }\nfun top() = 1\nval tv = 2\n",
		"package p\nclass B : A() { override fun f() = 2 }\n",
		"package p\nfun useTop() = top()\nfun inferred() = tv\n",
		"package sl\nsealed class S\nclass A1 : S()\nfun pick(s: S) = when (s) { is A1 -> 1 }\nfun sl0() = 0\n",
		"package sl\nclass B1 : S()\n",
		"package o\nclass Vec(val x: Int)\n",
		"package o\noperator fun Vec.plus(o: Vec) = Vec(x + o.x)\n",
		"package o\nfun sum(a: Vec, b: Vec) = a + b\nfun oo() = 0\n",
		"package u\nfun solo() = 1\nclass Solo\n",
		"package u\nfun other() = 2\n",
	}
	edits := []struct {
		file     int
		from, to string
	}{
		{0, "fun top() = 1", "fun top() = 1L"},
		{0, "val tv = 2\n", "val tv = 2\nfun added() = 3\n"},
		{4, "class B1 : S()", "class B1 : S()\nclass C1 : S()"},
		{3, "sealed class S", "sealed class S\nsealed class S2"},
		{4, "class C1 : S()", "class C1 : S2()"},
		{3, "sealed class S2", "open class S2"},
		{7, "fun oo() = 0", "fun oo() = 0L"},
		{6, "operator fun Vec.plus", "operator fun Vec.minus(o: Vec) = this\noperator fun Vec.plus"},
		{9, "fun other() = 2", "fun other() = 2\nfun solo() = 3"},
		{0, "open class A { open fun f() = 1 }", ""},
		{8, "class Solo", "class Solo2"},
	}
	s := Open(withStdlib(t))
	defer s.Close()
	if _, err := s.Update(context.Background(), sources(base...)); err != nil {
		t.Fatal(err)
	}
	texts := slices.Clone(base)
	before := reachIndexPatched.Load()
	for i, e := range edits {
		texts[e.file] = strings.Replace(texts[e.file], e.from, e.to, 1)
		got, err := s.Update(context.Background(), sources(texts...))
		if err != nil {
			t.Fatal(err)
		}
		if d := sameModel(got, wholeCheck(t, s, sources(texts...))); d != "" {
			t.Errorf("edit %d: %s", i, d)
		}
	}
	if reachIndexPatched.Load() == before {
		t.Error("no update changed a kept index")
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
// with a whole check. KA_BENCH_FILES limits it to that many files, and
// KA_BENCH_FILE to the files whose paths contain it.
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
	edits, partial, resolved := 0, 0, 0
	check.LateInferences.Store(0)
	only := os.Getenv("KA_BENCH_FILE")
	for i := range srcs[:limit] {
		if !strings.Contains(srcs[i].Path, only) {
			continue
		}
		for _, text := range signatureEdits(srcs[i].Path, texts[i]) {
			edit := slices.Clone(srcs)
			edit[i].Text = &text
			declUpdates.Store(0)
			got, err := s.Update(context.Background(), edit)
			if err != nil {
				t.Fatal(err)
			}
			partial += int(declUpdates.Load())
			resolved += int(lastResolved.Load())
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
			if os.Getenv("KA_BENCH_BACK") != "" {
				want := wholeCheck(t, s, srcs)
				if d := sameModel(back, want); d != "" {
					t.Errorf("%s, undone: %s", srcs[i].Path, d)
				}
				want.Close()
			}
			back.Close()
		}
	}
	t.Logf("%d edits, %d checked partially, %d late inferences, %.1f of %d files resolved per edit",
		edits, partial, check.LateInferences.Load(), float64(resolved)/float64(max(edits, 1)), len(srcs))
}

// A body edit of a module with Java sources, whose texts are the same,
// checks only the changed file, and gives what a whole check does.
func TestIncrementalJava(t *testing.T) {
	cfg := withJDK(t)
	cfg.ModuleName = "incrementalJava"
	java := `package q;
public class J extends p.Base {
    public int n;
    public java.util.List<String> names() { return null; }
    public p.Base base() { return this; }
}
`
	a := `package p
open class Base { open fun id() = 1 }
class A { fun f(): Int { val j = q.J(); return j.n + j.id() } }
`
	b := `package p
fun g(j: q.J): Int { return j.names().size }
fun h() = q.J().base().id()
`
	srcs := func(a, b string) []Source {
		return []Source{{Path: "a.kt", Text: &a}, {Path: "b.kt", Text: &b}, {Path: "J.java", Text: &java}}
	}
	s := Open(cfg)
	defer s.Close()
	m1, err := s.Update(context.Background(), srcs(a, b))
	if err != nil {
		t.Fatal(err)
	}
	defer m1.Close()
	if len(m1.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", m1.Diagnostics)
	}
	b2 := strings.Replace(b, "return j.names().size", "return j.names().size + 1", 1)
	incrementalUpdates.Store(0)
	shiftedUpdates.Store(0)
	got, err := s.Update(context.Background(), srcs(a, b2))
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	if incrementalUpdates.Load() != 1 {
		t.Errorf("not checked incrementally")
	}
	if shiftedUpdates.Load() != 1 {
		t.Errorf("b.kt not shifted")
	}
	if got.Class("q/J") == nil {
		t.Error("Class(q/J) = nil")
	}
	want := wholeCheck(t, s, srcs(a, b2))
	defer want.Close()
	if d := sameModel(got, want); d != "" {
		t.Error(d)
	}
	if g, w := got.Class("q/J"), want.Class("q/J"); !reflect.DeepEqual(g, w) {
		t.Errorf("Class(q/J) = %s, want %s", firstDiff(g, w), "")
	}
}

// TestIncrementalJavaDecl makes signature edits to a module with a Java
// source that extends, and returns, a Kotlin class, and compares each
// update with a whole check.
func TestIncrementalJavaDecl(t *testing.T) {
	cfg := withJDK(t)
	cfg.ModuleName = "incrementalJavaDecl"
	java := `package q;
public class J extends p.Base {
    public int n;
    public p.Base base() { return this; }
    public int k() { return p.UtilKt.util(); }
}
`
	java2 := `package q;
public class K {
    public p.Box box() { return null; }
}
`
	base := []string{
		"package p\nopen class Base { open fun id() = 1 }\n",
		"package p\nfun h() = q.J().base().id()\nfun i() = q.J().id()\nfun n() = q.J().n\n",
		"package p\nfun util() = 3\n",
		"package r\nfun alone() = 1\n",
		"package r\nfun alone2() = alone()\n",
		"package s\nfun more() = 4\n",
		"package s\nfun more2() = more() + 1\n",
		"package t\nfun solo() = 5\n",
		"package p\nclass Box { fun size() = 1 }\n",
		"package u\nfun b() = q.K().box()\n",
		"package u\nfun other() = 0\n",
		"package u\nfun other2() = 0\n",
		// Only K.java names Box.
		"package u\nfun viaJava() = q.K().box().size()\nfun zz() = 0\n",
	}
	edits := []struct {
		name     string
		file     int
		from, to string
		partial  bool
	}{
		{"returned class removed", 8, "class Box {", "class Box2 {", true},
		{"reached by a Java source", 12, "fun zz() = 0", "fun zz() = 0L", true},
		{"inherited member type", 0, "open fun id() = 1", "open fun id() = 1L", true},
		{"new member", 0, "open fun id() = 1 }", "open fun id() = 1\n fun other() = 0 }", true},
		{"class removed", 0, "open class Base { open fun id() = 1 }", "", true},
		{"facade function", 2, "fun util() = 3", "fun util() = 3L", true},
		{"unrelated", 3, "fun alone() = 1", "fun alone() = 1L", true},
		{"jvm name", 2, "package p\n", "@file:JvmName(\"U\")\npackage p\n", false},
	}
	srcs := func(texts []string) []Source {
		out := sources(texts...)
		return append(out, Source{Path: "J.java", Text: &java}, Source{Path: "K.java", Text: &java2})
	}
	for _, e := range edits {
		t.Run(e.name, func(t *testing.T) {
			s := Open(cfg)
			defer s.Close()
			m, err := s.Update(context.Background(), srcs(base))
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Diagnostics) > 0 {
				t.Fatalf("diagnostics: %v", m.Diagnostics)
			}
			texts := slices.Clone(base)
			texts[e.file] = strings.Replace(texts[e.file], e.from, e.to, 1)
			declUpdates.Store(0)
			got, err := s.Update(context.Background(), srcs(texts))
			if err != nil {
				t.Fatal(err)
			}
			if partial := declUpdates.Load() == 1; partial != e.partial {
				t.Errorf("checked partially: %v, want %v", partial, e.partial)
			}
			if d := sameModel(got, wholeCheck(t, s, srcs(texts))); d != "" {
				t.Error(d)
			}
			got, err = s.Update(context.Background(), srcs(base))
			if err != nil {
				t.Fatal(err)
			}
			if d := sameModel(got, wholeCheck(t, s, srcs(base))); d != "" {
				t.Errorf("back: %s", d)
			}
		})
	}
}

// A KSP run's second round, after an edit that changes a declaration,
// takes the edited files' models from the first round's check when what
// the generated files declare can't change them (checkCache.borrow), and
// gives what a whole check does either way.
func TestIncrementalBorrow(t *testing.T) {
	a := "package p\nclass A { val n = 1 }\nfun edit(): Int = 1\n"
	b := "package p\nclass B(val a: A)\nfun helper(x: Any) = 1\n"
	gen := "package p\nclass BAdapter { fun make(a: A) = B(a) }\n"
	java := "package q;\npublic class J { public int n; }\n"
	cases := []struct {
		name   string
		s, g   []string // the sources, and the generated files
		g2     []string // the generated files after the edit, if others
		java   bool
		file   int
		from   string
		to     string
		borrow bool
		gj     string // a generated Java file, if any
	}{
		{"signature", []string{a, b}, []string{gen}, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", true, ""},
		{"new declaration", []string{a, b}, []string{gen}, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Int = 1\nfun added() = 2", true, ""},
		{"looked up, borrowed too", []string{a, b + "fun useEdit() = edit()\n"}, []string{gen}, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Long = 1L", true, ""},
		{"looks up what a generated file declares",
			[]string{a + "fun pick() = helper(1)\n", b}, []string{gen + "fun helper(x: Int) = 1L\n"}, nil, false,
			0, "fun edit(): Int = 1", "fun edit(): Any = 2", false, ""},
		{"a generated file looks it up", []string{a, b}, []string{gen + "fun g() = edit()\n"}, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Long = 1L", false, ""},
		{"a generated file changed", []string{a, b}, []string{gen}, []string{gen + "class More\n"}, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", false, ""},
		{"names a new package", []string{a + "val label = \"gen\"\n", b}, []string{"package p.gen\nclass G\n"}, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", false, ""},
		{"other package", []string{a, b}, []string{"package p.gen\nclass G(val a: p.A)\n"}, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", true, ""},
		{"java", []string{a, b}, []string{gen}, nil, true, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", false, ""},
		{"generated java", []string{a, b}, nil, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", true,
			"package p;\npublic class AFactory { public A make() { return new A(); } }\n"},
		{"java names the facade", []string{a, b}, nil, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Long = 1L", false,
			"package p;\npublic class Edits { public Object e() { return F0Kt.edit(); } }\n"},
		{"java declares what it looks up", []string{strings.Replace(a, "package p\n", "package p\nimport r.*\n", 1) + "fun make() = Rc()\n", b, "package r\nclass Rc\n"}, nil, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", false,
			"package p;\npublic class Rc { }\n"},
		{"names a new java package", []string{a + "val label = \"aggregated\"\n", b}, nil, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", false,
			"package deps.aggregated.codegen;\npublic class D { }\n"},
		{"names a java package the class path has", []string{a + "val label = \"util\"\n", b}, nil, nil, false, 0, "fun edit(): Int = 1", "fun edit(): Any = 2", true,
			"package java.util.gen;\npublic class D { }\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := withStdlib(t)
			if c.java || c.gj != "" {
				cfg = withJDK(t)
			}
			s := Open(cfg)
			defer s.Close()
			gj := c.gj
			srcs := func(texts ...string) []Source {
				out := sources(texts...)
				if c.java {
					out = append(out, Source{Path: "J.java", Text: &java})
				}
				return out
			}
			// A round with the generated files: the Java one too.
			withGen := func(texts ...string) []Source {
				out := srcs(texts...)
				if gj != "" {
					out = append(out, Source{Path: "gen/G.java", Text: &gj})
				}
				return out
			}
			update := func(srcs []Source) *Model {
				m, err := s.Update(context.Background(), srcs)
				if err != nil {
					t.Fatal(err)
				}
				return m
			}
			// Files no edit reaches, so that the updates check only some
			// (maxDirtyShare).
			base := append(slices.Clone(c.s), "package r\nfun r1() = 1\n", "package r\nfun r2() = 2\n", "package r\nfun r3() = 3\n")
			// A run before the edit: its two rounds.
			update(srcs(base...))
			update(withGen(slices.Concat(base, c.g)...))
			edited := slices.Clone(base)
			edited[c.file] = strings.Replace(edited[c.file], c.from, c.to, 1)
			if edited[c.file] == base[c.file] {
				t.Fatal("no edit")
			}
			g := c.g
			if c.g2 != nil {
				g = c.g2
			}
			for round, texts := range [][]string{edited, slices.Concat(edited, g)} {
				in := srcs
				if round == 1 {
					in = withGen
				}
				borrowedUpdates.Store(0)
				declUpdates.Store(0)
				got := update(in(texts...))
				if declUpdates.Load() != 1 {
					t.Errorf("round %d: not checked partially", round+1)
				}
				if borrowed, want := borrowedUpdates.Load() == 1, round == 1 && c.borrow && !noBorrow; borrowed != want {
					t.Errorf("round %d: borrowed %v, want %v", round+1, borrowed, want)
				}
				if d := sameModel(got, wholeCheck(t, s, in(texts...))); d != "" {
					t.Errorf("round %d: %s", round+1, d)
				}
			}
		})
	}
}
