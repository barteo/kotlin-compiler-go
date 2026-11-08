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
		have := map[Diagnostic]bool{}
		for _, d := range got.Diagnostics() {
			have[d] = true
		}
		for _, d := range want.Diagnostics() {
			if !have[d] {
				t.Errorf("%s: session lacks %v", name, d)
			}
		}
	}
	t.Logf("%d programs, %d without errors", n, complete)
	if libraryOpens.Load() > 1 {
		t.Errorf("the session opened %d libraries", libraryOpens.Load())
	}
}

// TestSessionPartial checks that a file with an error loses its facts and
// the others keep theirs.
func TestSessionPartial(t *testing.T) {
	cfg := withStdlib(t)
	s := Open(cfg)
	defer s.Close()
	a := psi.Parse("a.kt", "package p\nfun twice(x: Int) = x * 2\n")
	b := psi.Parse("b.kt", "package p\nfun g(): String = twice(1)\n")
	c := psi.Parse("c.kt", "package p\nfun h() = twice(2) + 1\n")
	res, err := s.Update(context.Background(), []*psi.Snapshot{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if res.Complete() || len(res.Diagnostics()) != 1 || res.Diagnostics()[0].Path != "b.kt" {
		t.Fatalf("diagnostics %v, want one in b.kt", res.Diagnostics())
	}
	files := map[psi.SnapshotID]int{}
	for _, f := range res.Data().Facts {
		files[f.Element.Snapshot]++
	}
	if files[b.ID()] != 0 || files[a.ID()] == 0 || files[c.ID()] == 0 {
		t.Errorf("facts per file a=%d b=%d c=%d, want none in b only", files[a.ID()], files[b.ID()], files[c.ID()])
	}
	// AnalyzeArena stays all or nothing.
	all, err := AnalyzeArena(context.Background(), cfg, []*psi.Snapshot{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Data().Facts) != 0 {
		t.Errorf("AnalyzeArena has %d facts with errors", len(all.Data().Facts))
	}
	// A syntax error in one file: the others are still checked.
	broken := psi.Parse("b.kt", "package p\nfun g(: String = twice(1)\n")
	res, err = s.Update(context.Background(), []*psi.Snapshot{a, broken, c})
	if err != nil {
		t.Fatal(err)
	}
	files = map[psi.SnapshotID]int{}
	for _, f := range res.Data().Facts {
		files[f.Element.Snapshot]++
	}
	if files[broken.ID()] != 0 || files[a.ID()] == 0 || files[c.ID()] == 0 {
		t.Errorf("with a syntax error: facts per file a=%d b=%d c=%d", files[a.ID()], files[broken.ID()], files[c.ID()])
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

// An update that changes what a file declares is checked whole.
func TestSessionIncrementalSignature(t *testing.T) {
	a := "package p\nfun f() = 1\nfun g(): Int { return 1 }\n"
	b := "package p\nfun h() = f()\n"
	cfg := withStdlib(t)
	for _, a2 := range []string{
		"package p\nfun f() = 1L\nfun g(): Int { return 1 }\n",
		"package p\nfun f() = 1\nfun g(): Long { return 1 }\n",
		"package p\nfun f() = 1\nfun g(): Int { return 1 }\nfun k() {}\n",
	} {
		s := Open(cfg)
		bs := psi.Parse("b.kt", b)
		if _, err := s.Update(context.Background(), []*psi.Snapshot{psi.Parse("a.kt", a), bs}); err != nil {
			t.Fatal(err)
		}
		incrementalUpdates.Store(0)
		snaps := []*psi.Snapshot{psi.Parse("a.kt", a2), bs}
		got, err := s.Update(context.Background(), snaps)
		if err != nil {
			t.Fatal(err)
		}
		if incrementalUpdates.Load() != 0 {
			t.Errorf("%q checked incrementally", a2)
		}
		if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
			t.Error(d)
		}
		s.Close()
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
