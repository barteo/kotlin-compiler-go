package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// runModules returns the programs of testdata/run, each as its files:
// X.kt, or the .kt files of X/ for a directory with a main.kt.
func runModules(t *testing.T) map[string][]string {
	files, _ := filepath.Glob("../testdata/run/*.kt")
	dirs, _ := filepath.Glob("../testdata/run/*/main.kt")
	mods := map[string][]string{}
	for _, f := range files {
		mods[strings.TrimSuffix(filepath.Base(f), ".kt")] = []string{f}
	}
	for _, d := range dirs {
		kts, _ := filepath.Glob(filepath.Join(filepath.Dir(d), "*.kt"))
		mods[filepath.Base(filepath.Dir(d))] = kts
	}
	if len(mods) == 0 {
		t.Fatal("no programs in testdata/run")
	}
	return mods
}

func parseFiles(t *testing.T, paths []string) []*psi.Snapshot {
	t.Helper()
	var snaps []*psi.Snapshot
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		snaps = append(snaps, psi.Parse(filepath.Base(p), string(src)))
	}
	return snaps
}

// TestSessionMatchesAnalyze checks that a session's updates, which share
// one library, find what AnalyzeArena finds with a library of its own, on
// every program of testdata/run.
func TestSessionMatchesAnalyze(t *testing.T) {
	cfg := withJDK(t)
	s := Open(cfg)
	defer s.Close()
	ctx := context.Background()
	opens := libraryOpens.Load()
	n, complete := 0, 0
	for name, paths := range runModules(t) {
		if testing.Short() && n >= 60 {
			break
		}
		n++
		snaps := parseFiles(t, paths)
		want, err := AnalyzeArena(ctx, cfg, snaps)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := s.Update(ctx, snaps)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want.Complete() {
			complete++
			if !got.Complete() {
				t.Errorf("%s: session has diagnostics %v", name, got.Diagnostics())
			} else if !reflect.DeepEqual(got.Data(), want.Data()) {
				t.Errorf("%s: session's facts differ from AnalyzeArena's", name)
			}
			continue
		}
		// With errors, the session reports at least AnalyzeArena's, and
		// lowering errors stop AnalyzeArena before the checker.
		have := map[string]bool{}
		for _, d := range got.Diagnostics() {
			have[fmt.Sprint(d)] = true
		}
		for _, d := range want.Diagnostics() {
			if !have[fmt.Sprint(d)] {
				t.Errorf("%s: session lacks %v", name, d)
			}
		}
	}
	t.Logf("%d programs, %d without errors", n, complete)
	if d := libraryOpens.Load() - opens; d > 1 {
		t.Errorf("the session opened %d libraries", d)
	}
}

// TestSessionPartial checks that an error loses the facts of the code it
// is in, and the rest of its file and the other files keep theirs.
func TestSessionPartial(t *testing.T) {
	cfg := withStdlib(t)
	s := Open(cfg)
	defer s.Close()
	a := psi.Parse("a.kt", "package p\nfun twice(x: Int) = x * 2\n")
	b := psi.Parse("b.kt", "package p\nfun g(): String = twice(1)\nfun k() = twice(3)\n")
	c := psi.Parse("c.kt", "package p\nfun h() = twice(2) + 1\n")
	res, err := s.Update(context.Background(), []*psi.Snapshot{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if res.Complete() || len(res.Diagnostics()) != 1 || res.Diagnostics()[0].Path != "b.kt" {
		t.Fatalf("diagnostics %v, want one in b.kt", res.Diagnostics())
	}
	// g's expression body is dropped, not g or k.
	checkFacts(t, res, b, "twice(1)", "fun g", "twice(3)")
	files := map[psi.SnapshotID]int{}
	for _, f := range res.Data().Facts {
		files[f.Element.Snapshot]++
	}
	if files[a.ID()] == 0 || files[c.ID()] == 0 {
		t.Errorf("facts per file a=%d c=%d, want some", files[a.ID()], files[c.ID()])
	}
	// AnalyzeArena stays all or nothing.
	all, err := AnalyzeArena(context.Background(), cfg, []*psi.Snapshot{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Data().Facts) != 0 {
		t.Errorf("AnalyzeArena has %d facts with errors", len(all.Data().Facts))
	}
	// A syntax error in g's signature: g is dropped, k is kept, and the
	// other files are still checked.
	broken := psi.Parse("b.kt", "package p\nfun g(): = twice(1)\nfun k() = twice(3)\n")
	res, err = s.Update(context.Background(), []*psi.Snapshot{a, broken, c})
	if err != nil {
		t.Fatal(err)
	}
	checkFacts(t, res, broken, "fun g(): = twice(1)", "", "twice(3)")
	files = map[psi.SnapshotID]int{}
	for _, f := range res.Data().Facts {
		files[f.Element.Snapshot]++
	}
	if files[a.ID()] == 0 || files[c.ID()] == 0 {
		t.Errorf("with a syntax error: facts per file a=%d c=%d", files[a.ID()], files[c.ID()])
	}
}

// checkFacts checks that res has no fact of snap's code in the text gone,
// and has facts of the text at kept, and of a node that starts with name,
// unless it is empty.
func checkFacts(t *testing.T, res *Result, snap *psi.Snapshot, gone, name, kept string) {
	t.Helper()
	src := snap.Source()
	g := strings.Index(src, gone)
	goneR := psi.Range{Start: g, End: g + len(gone)}
	k := strings.Index(src, kept)
	keptR := psi.Range{Start: k, End: k + len(kept)}
	var sawKept, sawName bool
	for _, f := range res.Data().Facts {
		if f.Element.Snapshot != snap.ID() {
			continue
		}
		r := snap.Node(f.Element.Node).Range()
		if r.Start >= goneR.Start && r.End <= goneR.End {
			t.Errorf("%s: a fact of %q, in %q", snap.Path(), src[r.Start:r.End], gone)
		}
		if r.Start >= keptR.Start && r.End <= keptR.End {
			sawKept = true
		}
		if name != "" && strings.HasPrefix(src[r.Start:r.End], name) {
			sawName = true
		}
	}
	if !sawKept {
		t.Errorf("%s: no fact in %q", snap.Path(), kept)
	}
	if name != "" && !sawName {
		t.Errorf("%s: no fact of %q...", snap.Path(), name)
	}
}

// normalized returns res's facts, each as JSON with its symbols written
// out instead of numbered, and its diagnostics, sorted: what two results
// say, however they number their symbols and order their diagnostics.
func normalized(t *testing.T, res *Result) (facts, diags []string) {
	t.Helper()
	d := res.Data()
	syms := map[uint32]string{}
	for _, s := range d.Symbols {
		id := s.ID
		s.ID = 0
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		syms[id] = string(b)
	}
	for _, f := range d.Facts {
		sym, lambda := syms[f.Symbol], []string{}
		for _, p := range f.LambdaParameters {
			lambda = append(lambda, syms[p])
		}
		f.Symbol, f.LambdaParameters = 0, nil
		b, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		facts = append(facts, fmt.Sprintf("%s sym=%s lambda=%q", b, sym, lambda))
	}
	for _, x := range res.Diagnostics() {
		diags = append(diags, fmt.Sprint(x))
	}
	slices.Sort(diags)
	return facts, diags
}

// sameResult returns how got differs from want, or "".
func sameResult(t *testing.T, got, want *Result) string {
	t.Helper()
	gf, gd := normalized(t, got)
	wf, wd := normalized(t, want)
	if !slices.Equal(gd, wd) {
		return fmt.Sprintf("diagnostics %q, want %q", gd, wd)
	}
	if len(gf) != len(wf) {
		return fmt.Sprintf("%d facts, want %d", len(gf), len(wf))
	}
	for i := range gf {
		if gf[i] != wf[i] {
			return fmt.Sprintf("fact %d:\n got %s\nwant %s", i, gf[i], wf[i])
		}
	}
	return ""
}

// wholeUpdate returns the update of snaps by a fresh session.
func wholeUpdate(t *testing.T, cfg Config, snaps []*psi.Snapshot) *Result {
	t.Helper()
	s := Open(cfg)
	defer s.Close()
	res, err := s.Update(context.Background(), snaps)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestSessionIncremental checks that an update that only changes bodies
// checks only the changed files and finds what a whole check does.
func TestSessionIncremental(t *testing.T) {
	a := `package p

class A(val n: Int) {
    fun f(): Int {
        val x = n + 1
        return x
    }
    val p: Int get() { return n }
}

fun top(a: A): String {
    println("a")
    return a.f().toString()
}
`
	b := `package p

class B : Comparable<B> {
    init { println("b") }
    constructor(s: String) { println("c") }
    override fun compareTo(other: B): Int { return 0 }
    fun <T> g(t: T): T { return t }
}

fun useA(a: A): Int { return a.f() + a.p + top(a).length }
@Deprecated("x") fun err(): Int { return 1 }
`
	edits := []struct{ name, file, from, to string }{
		{"body", "a", "val x = n + 1", "val x = n + 2\n        val y = listOf(x).map { it * 2 }"},
		{"comment", "a", "class A", "/* c */ class A"},
		{"unit", "a", `println("a")`, `println("aa"); class Local { fun f() = 1 }`},
		{"accessor and init", "b", `init { println("b") }`, `init { println("b"); println(1) }`},
		{"error body", "b", `return 1 }`, `return "t" }`},
		{"ctor", "b", `println("c")`, `val q = 1; println(q)`},
		{"uses", "b", `return a.f() + a.p`, `val k = A(2); return k.f() + a.p + B("").g(1)`},
	}
	cfg := withJDK(t)
	ctx := context.Background()
	for _, e := range edits {
		t.Run(e.name, func(t *testing.T) {
			s := Open(cfg)
			defer s.Close()
			first := []*psi.Snapshot{psi.Parse("a.kt", a), psi.Parse("b.kt", b)}
			res, err := s.Update(ctx, first)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Complete() {
				t.Fatalf("diagnostics %v", res.Diagnostics())
			}
			snaps := slices.Clone(first)
			if e.file == "a" {
				snaps[0] = psi.Parse("a.kt", strings.Replace(a, e.from, e.to, 1))
			} else {
				snaps[1] = psi.Parse("b.kt", strings.Replace(b, e.from, e.to, 1))
			}
			incrementalUpdates.Store(0)
			got, err := s.Update(ctx, snaps)
			if err != nil {
				t.Fatal(err)
			}
			if incrementalUpdates.Load() != 1 {
				t.Errorf("not checked incrementally")
			}
			if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
				t.Error(d)
			}
			// And back, from an update that may have had errors.
			incrementalUpdates.Store(0)
			back, err := s.Update(ctx, first)
			if err != nil {
				t.Fatal(err)
			}
			if incrementalUpdates.Load() != 1 {
				t.Errorf("back: not checked incrementally")
			}
			if d := sameResult(t, back, res); d != "" {
				t.Errorf("back: %s", d)
			}
		})
	}
}

// TestSessionIncrementalMany makes many body edits to one session, in
// turn in each file, so that symbols are kept, added and compacted, and
// compares each update with a whole one.
func TestSessionIncrementalMany(t *testing.T) {
	defer func(n int) { compactSlack = n }(compactSlack)
	compactSlack = 4
	texts := []string{
		"package p\nclass A(val n: Int) { fun f(): Int { return n } }\nfun top(a: A): Int { return a.f() }\n",
		"package p\nfun useA(a: A): Int { return top(a) + a.n }\nfun more(): List<Int> { return listOf(1) }\n",
	}
	cfg := withJDK(t)
	ctx := context.Background()
	s := Open(cfg)
	defer s.Close()
	snaps := []*psi.Snapshot{psi.Parse("a.kt", texts[0]), psi.Parse("b.kt", texts[1])}
	if _, err := s.Update(ctx, snaps); err != nil {
		t.Fatal(err)
	}
	compactions.Store(0)
	incrementalUpdates.Store(0)
	const edits = 12
	for k := range edits {
		i := k % 2
		text := strings.Replace(texts[i], "{ return ", fmt.Sprintf("{ val v%d = listOf(%d).map { x -> x + 1 }.size; return ", k, k), 1)
		snaps = slices.Clone(snaps)
		snaps[i] = psi.Parse(snaps[i].Path(), text)
		got, err := s.Update(ctx, snaps)
		if err != nil {
			t.Fatal(err)
		}
		if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
			t.Fatalf("edit %d: %s", k, d)
		}
		for _, f := range got.Data().Facts {
			for _, id := range append([]uint32{f.Symbol}, f.LambdaParameters...) {
				if sym, ok := got.Symbol(id); id != 0 && (!ok || sym.ID != id) {
					t.Fatalf("edit %d: fact %v names symbol %d: %v", k, f.Element, id, sym)
				}
			}
		}
	}
	if n := incrementalUpdates.Load(); n != edits {
		t.Errorf("%d of %d edits checked incrementally", n, edits)
	}
	if compactions.Load() == 0 {
		t.Error("symbols never compacted")
	}
}

// An incremental update reads the contracts and inline functions' bodies
// of the files it doesn't check.
func TestSessionIncrementalContracts(t *testing.T) {
	b := `@file:OptIn(kotlin.contracts.ExperimentalContracts::class)
package p

import kotlin.contracts.*

fun isStr(x: Any?): Boolean {
    contract { returns(true) implies (x is String) }
    return x is String
}

inline fun once(f: () -> Unit) {
    contract { callsInPlace(f, InvocationKind.EXACTLY_ONCE) }
    f()
}

inline fun forward(crossinline f: () -> Unit): Runnable = Runnable { f() }

inline fun later(crossinline f: () -> Unit) {
    Runnable { f() }.run()
}

inline fun each(xs: List<Int>, crossinline f: (Int) -> Unit) {
    for (x in xs) later { f(x) }
}
`
	a := "package p\n\nfun use(o: Any?): Int {\n    return 0\n}\n"
	a2 := `package p

fun use(o: Any?): Int {
    val v: Int
    once { v = 1 }
    forward { println(v) }.run()
    later { println(v) }
    each(listOf(1)) { if (it > 0) return it }
    if (isStr(o)) return o.length + v
    return v
}
`
	cfg := withJDK(t)
	ctx := context.Background()
	s := Open(cfg)
	defer s.Close()
	bs := psi.Parse("b.kt", b)
	if res, err := s.Update(ctx, []*psi.Snapshot{psi.Parse("a.kt", a), bs}); err != nil {
		t.Fatal(err)
	} else if !res.Complete() {
		t.Fatalf("diagnostics %v", res.Diagnostics())
	}
	snaps := []*psi.Snapshot{psi.Parse("a.kt", a2), bs}
	incrementalUpdates.Store(0)
	got, err := s.Update(ctx, snaps)
	if err != nil {
		t.Fatal(err)
	}
	if incrementalUpdates.Load() != 1 {
		t.Error("not checked incrementally")
	}
	// The return is an error, whose message depends on later's body.
	if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
		t.Error(d)
	}
}

// TestSessionIncrementalDeclarations edits what files declare, and
// compares each update, checked with the files it affects, with a whole
// one; incremental says whether few enough files are affected that the
// update isn't checked whole.
func TestSessionIncrementalDeclarations(t *testing.T) {
	files := map[string]string{
		"a.kt": `package p

open class Base(val n: Int) {
    open fun f(x: Int): Int = x + n
    fun g(): Int { return n }
}

sealed interface Shape
class Circle(val r: Double) : Shape
object Empty : Shape

fun top(a: Base): String = a.f(1).toString()
fun inferred() = 1
typealias Num = Int
fun Base.ext(): Int = n
infix fun Int.plus2(o: Int): Int = this + o
`,
		"b.kt": `package p

class Derived : Base(1) {
    override fun f(x: Int): Int = x * 2
}

fun useBase(b: Base): Int = b.f(2) + b.g() + top(b).length + inferred() + b.ext() + (1 plus2 2)
fun area(s: Shape): Double = when (s) {
    is Circle -> s.r
    Empty -> 0.0
}
val num: Num = 1
`,
		"c.kt": `package p

fun unrelated(): Int { return 3 }
`,
		"d.kt": `package q

fun other(): String { return "d" }
`,
		"e.kt": `package q

fun more(): String { return other() }
`,
	}
	paths := []string{"a.kt", "b.kt", "c.kt", "d.kt", "e.kt"}
	for i := range 3 {
		p := fmt.Sprintf("r%d.kt", i)
		files[p] = fmt.Sprintf("package r\n\nfun r%d(): Int { return %d }\n", i, i)
		paths = append(paths, p)
	}
	type edit struct{ path, from, to string }
	cases := []struct {
		name        string
		edits       []edit
		incremental bool
	}{
		{"parameter added", []edit{{"a.kt", "open fun f(x: Int)", "open fun f(x: Int, y: Int = 0)"}}, true},
		{"parameter renamed", []edit{{"a.kt", "fun top(a: Base)", "fun top(b: Base)"}}, true},
		{"return type", []edit{{"a.kt", "fun g(): Int { return n }", "fun g(): Long { return n.toLong() }"}}, true},
		{"inferred type", []edit{{"a.kt", "fun inferred() = 1", `fun inferred() = "1"`}}, true},
		{"declaration added", []edit{{"c.kt", "fun unrelated()", "fun added() = 2\nfun unrelated()"}}, true},
		{"overload added", []edit{{"c.kt", "fun unrelated()", "fun top(a: Derived): String = \"d\"\nfun unrelated()"}}, true},
		{"overload removed", []edit{{"a.kt", "fun top(a: Base): String = a.f(1).toString()", ""}}, true},
		{"member added", []edit{{"a.kt", "fun g(): Int { return n }", "fun g(): Int { return n }\n    fun ext(): Int = 0"}}, true},
		{"subclass added", []edit{{"c.kt", "fun unrelated()", "class Square(val s: Double) : Shape\nfun unrelated()"}}, true},
		{"supertype removed", []edit{{"a.kt", "object Empty : Shape", "object Empty"}}, true},
		{"typealias", []edit{{"a.kt", "typealias Num = Int", "typealias Num = Long"}}, true},
		{"infix dropped", []edit{{"a.kt", "infix fun Int.plus2", "fun Int.plus2"}}, true},
		{"conflict", []edit{{"c.kt", "fun unrelated()", "fun inferred() = 2\nfun unrelated()"}}, true},
		{"moved", []edit{{"a.kt", "fun inferred() = 1\n", ""}, {"c.kt", "fun unrelated()", "fun inferred() = 1\nfun unrelated()"}}, true},
		{"other package", []edit{{"d.kt", "fun other(): String", "fun other(x: Int = 1): String"}}, true},
		{"package", []edit{{"d.kt", "package q", "package r"}}, true},
		{"class", []edit{{"a.kt", "open class Base(val n: Int)", "open class Base(val n: Long)"}}, true},
		{"most files", []edit{{"d.kt", "package q", "package r"}, {"r0.kt", "fun r0()", "fun other(): String = \"\"\nfun r0()"}, {"r1.kt", "package r", "package q"}, {"r2.kt", "fun r2()", "fun r2(x: Int = 0)"}}, false},
	}
	cfg := withStdlib(t)
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Open(cfg)
			defer s.Close()
			var first []*psi.Snapshot
			for _, p := range paths {
				first = append(first, psi.Parse(p, files[p]))
			}
			res, err := s.Update(ctx, first)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Complete() {
				t.Fatalf("diagnostics %v", res.Diagnostics())
			}
			snaps := slices.Clone(first)
			for _, e := range tc.edits {
				i := slices.Index(paths, e.path)
				text := snaps[i].Source()
				if !strings.Contains(text, e.from) {
					t.Fatalf("%s has no %q", e.path, e.from)
				}
				snaps[i] = psi.Parse(e.path, strings.Replace(text, e.from, e.to, 1))
			}
			for _, step := range []struct {
				name  string
				snaps []*psi.Snapshot
			}{{"edit", snaps}, {"back", first}} {
				incrementalUpdates.Store(0)
				got, err := s.Update(ctx, step.snaps)
				if err != nil {
					t.Fatal(err)
				}
				if inc := incrementalUpdates.Load() == 1; inc != tc.incremental {
					t.Errorf("%s: checked incrementally: %v", step.name, inc)
				}
				if d := sameResult(t, got, wholeUpdate(t, cfg, step.snaps)); d != "" {
					t.Errorf("%s: %s", step.name, d)
				}
			}
		})
	}
	// Files added and removed.
	s := Open(cfg)
	defer s.Close()
	var all []*psi.Snapshot
	for _, p := range paths {
		all = append(all, psi.Parse(p, files[p]))
	}
	for _, snaps := range [][]*psi.Snapshot{all, all[:4], all, append(slices.Clone(all), psi.Parse("f.kt", "package q\nfun other2() = other()\n")), all} {
		got, err := s.Update(ctx, snaps)
		if err != nil {
			t.Fatal(err)
		}
		if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
			t.Errorf("%d files: %s", len(snaps), d)
		}
	}
}

// bodyEdit returns snap's text with a statement added to the block body
// of its nth function, or "" if it has none.
func bodyEdit(snap *psi.Snapshot, nth int) string {
	text := snap.Source()
	for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
		n := snap.Node(id)
		if n.NumericKind() == psi.KindBLOCK && n.Parent().NumericKind() == psi.KindFUN {
			if nth > 0 {
				nth--
				continue
			}
			at := n.Range().Start + 1
			return text[:at] + "\n val kcEdit = listOf(1).map { it + 1 }\n" + text[at:]
		}
	}
	return ""
}

// TestSessionIncrementalRun edits a body of each file of each program of
// testdata/run, but for the one-file programs with -short, and compares
// the incremental update with a whole one.
func TestSessionIncrementalRun(t *testing.T) {
	cfg := withJDK(t)
	ctx := context.Background()
	edits, incremental := 0, 0
	for name, paths := range runModules(t) {
		if len(paths) < 2 && testing.Short() {
			continue
		}
		s := Open(cfg)
		first := parseFiles(t, paths)
		if _, err := s.Update(ctx, first); err != nil {
			t.Fatal(err)
		}
		for i := range first {
			for nth := range 2 {
				text := bodyEdit(first[i], nth)
				if text == "" {
					break
				}
				snaps := slices.Clone(first)
				snaps[i] = psi.Parse(first[i].Path(), text)
				incrementalUpdates.Store(0)
				got, err := s.Update(ctx, snaps)
				if err != nil {
					t.Fatal(err)
				}
				edits++
				if incrementalUpdates.Load() == 1 {
					incremental++
				}
				if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
					t.Errorf("%s: %s, function %d: %s", name, first[i].Path(), nth, d)
				}
			}
		}
		s.Close()
	}
	t.Logf("%d edits, %d incremental", edits, incremental)
	if incremental < edits*9/10 {
		t.Errorf("only %d of %d edits checked incrementally", incremental, edits)
	}
}

// TestFactIDs checks that factIDs and symbolIDs see every element a fact
// or symbol refers to.
func TestFactIDs(t *testing.T) {
	cfg := withJDK(t)
	var count func(v reflect.Value) int
	elem := reflect.TypeFor[ElementID]()
	count = func(v reflect.Value) int {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				return 0
			}
			return count(v.Elem())
		case reflect.Slice:
			n := 0
			for i := range v.Len() {
				n += count(v.Index(i))
			}
			return n
		case reflect.Struct:
			if v.Type() == elem {
				return 1
			}
			n := 0
			for i := range v.NumField() {
				n += count(v.Field(i))
			}
			return n
		}
		return 0
	}
	checked := 0
	for _, paths := range runModules(t) {
		if checked > 40 {
			break
		}
		res, err := AnalyzeArena(context.Background(), cfg, parseFiles(t, paths))
		if err != nil {
			t.Fatal(err)
		}
		checked++
		d := res.Data()
		for _, f := range d.Facts {
			n := 0
			factIDs(&f, func(*ElementID) { n++ })
			if want := count(reflect.ValueOf(f)) - 1; n != want { // but its Element
				t.Fatalf("factIDs sees %d elements of %+v, want %d", n, f, want)
			}
		}
		for _, s := range d.Symbols {
			n := 0
			symbolIDs(&s, func(*ElementID) { n++ })
			if want := count(reflect.ValueOf(s)); n != want {
				t.Fatalf("symbolIDs sees %d elements of %+v, want %d", n, s, want)
			}
		}
	}
}

// TestDiagnosticRange checks the codes and ranges of errors.
func TestDiagnosticRange(t *testing.T) {
	cfg := withStdlib(t)
	src := "package p\n\nfun f(): Int {\n    val s: String = 1\n    val i: Int = \"a $s\"\n    return nope + 1\n}\n"
	snap := psi.Parse("a.kt", src)
	res, err := AnalyzeArena(context.Background(), cfg, []*psi.Snapshot{snap})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range res.Diagnostics() {
		got[d.Kotlinc] += src[d.Range.Start:d.Range.End] + ";"
		if d.Code != "source_error" {
			t.Errorf("%v: code %s", d, d.Code)
		}
	}
	want := map[string]string{"TYPE_MISMATCH": `1;"a $s";`, "UNRESOLVED_REFERENCE": "nope;"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("code: underlined text %q, want %q (diagnostics %v)", got, want, res.Diagnostics())
	}
}

// declarationEdit returns snap's text with what it declares changed: a
// parameter with a default added to its first function with parameters'
// parentheses (kind 0), or a top-level function added (kind 1); "" if
// it has no such function.
func declarationEdit(snap *psi.Snapshot, kind int) string {
	text := snap.Source()
	if kind == 1 {
		return text + "\nfun kcAdded(): Int = 1\n"
	}
	for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
		n := snap.Node(id)
		if n.NumericKind() != psi.KindFUN {
			continue
		}
		l := n.ChildKind(psi.KindVALUE_PARAMETER_LIST)
		if !l.Valid() {
			continue
		}
		at := l.Range().Start + 1
		sep := ""
		if len(n.Parameters()) > 0 {
			sep = ", "
		}
		return text[:at] + "kcAdded: Int = 0" + sep + text[at:]
	}
	return ""
}

// TestSessionIncrementalDeclarationScan changes what each file of each
// program of testdata/run declares, but for the one-file programs with
// -short, and compares the update, and the update back, with whole
// ones.
func TestSessionIncrementalDeclarationScan(t *testing.T) {
	cfg := withJDK(t)
	ctx := context.Background()
	edits, incremental := 0, 0
	for name, paths := range runModules(t) {
		if len(paths) < 2 && testing.Short() {
			continue
		}
		s := Open(cfg)
		first := parseFiles(t, paths)
		if _, err := s.Update(ctx, first); err != nil {
			t.Fatal(err)
		}
		for i := range first {
			for kind := range 2 {
				text := declarationEdit(first[i], kind)
				if text == "" {
					continue
				}
				snaps := slices.Clone(first)
				snaps[i] = psi.Parse(first[i].Path(), text)
				incrementalUpdates.Store(0)
				got, err := s.Update(ctx, snaps)
				if err != nil {
					t.Fatal(err)
				}
				if len(first) > 1 {
					edits++
					if incrementalUpdates.Load() == 1 {
						incremental++
					}
				}
				if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
					t.Errorf("%s: %s, edit %d: %s", name, first[i].Path(), kind, d)
				}
				// And back.
				if got, err = s.Update(ctx, first); err != nil {
					t.Fatal(err)
				}
				if d := sameResult(t, got, wholeUpdate(t, cfg, first)); d != "" {
					t.Errorf("%s: %s, edit %d back: %s", name, first[i].Path(), kind, d)
				}
			}
		}
		s.Close()
	}
	// One file is always checked whole, and a module of two often is.
	t.Logf("%d edits of modules of several files, %d incremental", edits, incremental)
}
