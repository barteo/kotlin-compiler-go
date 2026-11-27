package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// normalized returns r's diagnostics and facts as text that doesn't
// depend on how the symbols are numbered: each symbol ID a fact names
// is replaced by the symbol it names.
func normalized(t *testing.T, r *Result) (diags, facts []string) {
	t.Helper()
	for _, d := range r.Diagnostics() {
		diags = append(diags, fmt.Sprintf("%s:%d:%d %s %s %s", filepath.Base(d.Path), d.Line, d.Column, d.Severity, d.Name, d.Message))
	}
	slices.Sort(diags)
	sym := func(id uint32) string {
		if id == 0 {
			return ""
		}
		s, ok := r.Symbol(id)
		if !ok {
			t.Fatalf("fact names symbol %d, which the result doesn't have", id)
		}
		s.ID = 0
		b, _ := json.Marshal(s)
		return string(b)
	}
	for _, f := range r.Data().Facts {
		s := sym(f.Symbol)
		var ps []string
		for _, p := range f.LambdaParameters {
			ps = append(ps, sym(p))
		}
		f.Symbol, f.LambdaParameters = 0, nil
		b, _ := json.Marshal(f)
		facts = append(facts, fmt.Sprintf("%s symbol=%s params=%v", b, s, ps))
	}
	slices.Sort(facts)
	return diags, facts
}

// sameAsAnalyze checks that a session's result for snaps is the result
// Library.Analyze has for them.
func sameAsAnalyze(t *testing.T, lib *Library, got *Result, snaps []*psi.Snapshot, opts Options, step string) {
	t.Helper()
	want, err := lib.Analyze(context.Background(), snaps, opts)
	if err != nil {
		t.Fatalf("%s: Analyze: %v", step, err)
	}
	gd, gf := normalized(t, got)
	wd, wf := normalized(t, want)
	if !slices.Equal(gd, wd) {
		t.Errorf("%s: diagnostics differ\n got  %q\n want %q", step, gd, wd)
	}
	if !slices.Equal(gf, wf) {
		missing, extra := diff(wf, gf), diff(gf, wf)
		t.Errorf("%s: facts differ: %d missing, %d extra\n missing %.600q\n extra   %.600q", step, len(missing), len(extra), missing, extra)
	}
}

// diff returns the strings of a that b doesn't have.
func diff(a, b []string) []string {
	var out []string
	for _, s := range a {
		if _, ok := slices.BinarySearch(b, s); !ok {
			out = append(out, s)
		}
	}
	return out
}

// module is a module's sources being edited.
type module struct {
	paths []string
	snaps map[string]*psi.Snapshot
}

func newModule(files map[string]string) *module {
	m := &module{snaps: map[string]*psi.Snapshot{}}
	for p, src := range files {
		m.paths = append(m.paths, p)
		m.snaps[p] = psi.Parse(p, src)
	}
	slices.Sort(m.paths)
	return m
}

func (m *module) set(path, src string) {
	if _, ok := m.snaps[path]; !ok {
		m.paths = append(m.paths, path)
		slices.Sort(m.paths)
	}
	m.snaps[path] = psi.Parse(path, src)
}

func (m *module) remove(path string) {
	m.paths = slices.DeleteFunc(m.paths, func(p string) bool { return p == path })
	delete(m.snaps, path)
}

func (m *module) replace(t *testing.T, path, old, new string) {
	t.Helper()
	src := m.snaps[path].Source()
	if !strings.Contains(src, old) {
		t.Fatalf("%s has no %q", path, old)
	}
	m.set(path, strings.Replace(src, old, new, 1))
}

func (m *module) list() []*psi.Snapshot {
	out := make([]*psi.Snapshot, len(m.paths))
	for i, p := range m.paths {
		out[i] = m.snaps[p]
	}
	return out
}

const (
	sessA = `@file:OptIn(kotlin.contracts.ExperimentalContracts::class)
package p

import kotlin.contracts.*

class Box(val n: Int) {
    fun twice(): Int = n * 2
}

fun isStr(x: Any?): Boolean {
    contract { returns(true) implies (x is String) }
    return x is String
}

inline fun once(f: () -> Unit) {
    f()
}

fun helper(b: Box): Int {
    val unused = 1
    return b.twice()
}
`
	sessB = `package p

fun use(o: Any?): Int {
    val b = Box(1)
    if (isStr(o)) return o.length + helper(b)
    once { println("x") }
    return b.n
}
`
	sessC = `package q

import p.Box

fun other(): Box {
    val x = 1 as Int
    return Box(x)
}
`
	sessD = `package q

fun alone(s: String): Int {
    var y = s.length
    y += 1
    return y
}
`
)

// TestSessionMatchesAnalyze checks that each update of a session, after
// body edits, declaration edits, syntax errors and files added and
// removed, has the result Library.Analyze has, and that the updates that
// can be are incremental.
func TestSessionMatchesAnalyze(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	ctx := context.Background()
	opts := Options{Warnings: true, ExtraWarnings: true}
	m := newModule(map[string]string{"a.kt": sessA, "b.kt": sessB, "c.kt": sessC, "d.kt": sessD})
	s := lib.Session()
	defer s.Close()
	steps := []struct {
		name        string
		edit        func()
		incremental bool
	}{
		{"first", func() {}, false},
		{"unchanged", func() {}, true},
		{"body edit", func() { m.replace(t, "b.kt", "return b.n", "return b.n + 1") }, true},
		{"body error", func() { m.replace(t, "d.kt", "y += 1", "y += undefinedName") }, true},
		{"body warning", func() { m.replace(t, "d.kt", "var y = s.length", "var y = s.length as Int") }, true},
		{"body syntax error", func() { m.replace(t, "d.kt", "y += undefinedName", "y += (1 +") }, true},
		{"fix syntax error", func() { m.replace(t, "d.kt", "y += (1 +", "y += 2") }, true},
		{"declaration edit used elsewhere", func() { m.replace(t, "a.kt", "fun helper(b: Box): Int", "fun helper(b: Box, k: Int = 0): Int") }, true},
		{"declaration edit breaking a caller", func() { m.replace(t, "a.kt", "fun helper(b: Box, k: Int = 0): Int", "fun helper(b: Box, k: Int): Int") }, true},
		// Box's members are its declaration's: every file that uses Box is
		// checked, most of the module's, so the update is whole.
		{"member removed", func() { m.replace(t, "a.kt", "    fun twice(): Int = n * 2\n", "") }, false},
		{"member back", func() {
			m.replace(t, "a.kt", "class Box(val n: Int) {\n", "class Box(val n: Int) {\n    fun twice(): Int = n * 2\n")
		}, false},
		{"contract edit", func() {
			m.replace(t, "a.kt", "returns(true) implies (x is String)", "returns(false) implies (x is String)")
		}, true},
		{"inline body edit", func() {
			m.replace(t, "a.kt", "inline fun once(f: () -> Unit) {\n    f()", "inline fun once(f: () -> Unit) {\n    f()\n    f()")
		}, true},
		{"file added", func() { m.set("e.kt", "package q\n\nfun more(): Int = alone(\"x\") + other().n\n") }, true},
		{"file removed", func() { m.remove("e.kt") }, true},
		// Lowering leaves out other: whoever looked up a name c.kt
		// declares is checked, before and after.
		{"declaration syntax error", func() { m.replace(t, "c.kt", "fun other(): Box {", "fun other(: Box {") }, true},
		{"fix declaration syntax error", func() { m.replace(t, "c.kt", "fun other(: Box {", "fun other(): Box {") }, true},
		{"after whole", func() { m.replace(t, "b.kt", "return b.n + 1", "return b.n + 2") }, true},
	}
	for _, st := range steps {
		st.edit()
		before := s.Stats()
		snaps := m.list()
		res, err := s.Update(ctx, snaps, opts)
		if err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		after := s.Stats()
		if inc := after.Incremental > before.Incremental; inc != st.incremental {
			t.Errorf("%s: incremental %v, want %v", st.name, inc, st.incremental)
		}
		sameAsAnalyze(t, lib, res, snaps, opts, st.name)
	}
}

// TestSessionDeclared checks that files only declared have no facts and
// no diagnostics, and that the others' are those Library.Analyze has
// with the same Options.Declared.
func TestSessionDeclared(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	ctx := context.Background()
	m := newModule(map[string]string{"a.kt": sessA, "b.kt": sessB, "c.kt": sessC})
	opts := Options{Warnings: true, ExtraWarnings: true, Declared: map[string]bool{"a.kt": true}}
	// Compacted after every update, which keeps the classes too.
	defer func(n int) { compactSlack = n }(compactSlack)
	compactSlack = -1 << 30
	s := lib.Session()
	defer s.Close()
	for i, edit := range []func(){
		func() {},
		func() { m.replace(t, "a.kt", "return b.twice()", "return b.twice() + undefinedInDeclared") },
		func() { m.replace(t, "a.kt", "fun helper(b: Box): Int", "fun helper(b: Box, k: Int): Int") },
		func() { m.replace(t, "b.kt", "return b.n", "return b.n + 3") },
		func() { m.set("a.kt", m.snaps["a.kt"].Source()+"\nclass Unused\n") },
	} {
		edit()
		snaps := m.list()
		res, err := s.Update(ctx, snaps, opts)
		if err != nil {
			t.Fatal(err)
		}
		// The symbols have the classes of the declared files, as those
		// of the files checked.
		classes := map[string]bool{}
		for id := uint32(1); ; id++ {
			sym, ok := res.Symbol(id)
			if !ok {
				break
			}
			if sym.Kind == "class" && sym.Declaration != nil && sym.Declaration.Snapshot == m.snaps["a.kt"].ID() {
				classes[sym.QualifiedName] = true
			}
		}
		if !classes["p.Box"] || i == 4 && !classes["p.Unused"] {
			t.Errorf("step %d: classes of the declared file %v", i, classes)
		}
		for _, d := range res.Diagnostics() {
			if d.Path == "a.kt" {
				t.Errorf("step %d: diagnostic of a declared file: %+v", i, d)
			}
		}
		if n := len(slices.Collect(func(yield func(Fact) bool) {
			for _, f := range res.Data().Facts {
				if f.Element.Snapshot == m.snaps["a.kt"].ID() && !yield(f) {
					return
				}
			}
		})); n > 0 {
			t.Errorf("step %d: %d facts of a declared file", i, n)
		}
		sameAsAnalyze(t, lib, res, snaps, opts, fmt.Sprintf("step %d", i))
	}
	if st := s.Stats(); st.Incremental < 2 {
		t.Errorf("%d incremental updates of 5, want at least 2", st.Incremental)
	}
}

// TestSessionCompletionNotKept checks that a completion doesn't change
// what the next update starts from.
func TestSessionCompletionNotKept(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	ctx := context.Background()
	m := newModule(map[string]string{"a.kt": sessA, "b.kt": sessB})
	s := lib.Session()
	defer s.Close()
	opts := Options{Warnings: true}
	if _, err := s.Update(ctx, m.list(), opts); err != nil {
		t.Fatal(err)
	}
	probe := m.snaps["b.kt"].Source()
	probe = strings.Replace(probe, "return b.n", "return b."+Placeholder, 1)
	snaps := m.list()
	snaps[1] = psi.Parse("b.kt", probe)
	r, err := s.Update(ctx, snaps, Options{Complete: &Complete{Path: "b.kt"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Completion() == nil || len(r.Completion().Items) == 0 {
		t.Fatalf("no completion items")
	}
	m.replace(t, "b.kt", "return b.n", "return b.n + 1")
	res, err := s.Update(ctx, m.list(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if st := s.Stats(); st.Updates != 2 || st.Incremental != 1 {
		t.Errorf("stats %+v, want 2 updates, 1 incremental", st)
	}
	sameAsAnalyze(t, lib, res, m.list(), opts, "after completion")
}

// TestSessionRandomEdits makes random edits to a generated module, KC_SESSION_DIR
// or testdata/session, and checks each update against Library.Analyze.
func TestSessionRandomEdits(t *testing.T) {
	dir := os.Getenv("KC_SESSION_DIR")
	if dir == "" {
		t.Skip("KC_SESSION_DIR not set: a directory of Kotlin sources, as bench/gen writes")
	}
	lib := openLib(t, withJDK(t))
	ctx := context.Background()
	files := map[string]string{}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.kt"))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		files[p] = string(b)
	}
	m := newModule(files)
	s := lib.Session()
	defer s.Close()
	opts := Options{Warnings: true, ExtraWarnings: true}
	reasons := map[string]int{}
	traceWhole = func(r string) { reasons[r]++ }
	defer func() { traceWhole = nil }()
	seed := int64(1)
	if v := os.Getenv("KC_SESSION_SEED"); v != "" {
		fmt.Sscan(v, &seed)
	}
	rng := rand.New(rand.NewSource(seed))
	n := 40
	if testing.Short() {
		n = 10
	}
	for i := 0; i < n; i++ {
		p := m.paths[rng.Intn(len(m.paths))]
		src := m.snaps[p].Source()
		var step string
		switch k := rng.Intn(5); k {
		case 0, 1: // a statement in a body
			at := strings.Index(src, "{\n")
			if at < 0 {
				continue
			}
			stmt := []string{" val r%d = %d;", " val r%d = undefined%d;", " val r%d = (1 + ;", " println(%d.toString() + %d)"}[rng.Intn(4)]
			src = src[:at+1] + fmt.Sprintf(stmt, i, i) + src[at+1:]
			step = fmt.Sprintf("%d body edit of %s", i, filepath.Base(p))
		case 2: // a new top-level function
			src += fmt.Sprintf("\nfun added%d(x: Int): Int = x + %d\n", i, i)
			step = fmt.Sprintf("%d function added to %s", i, filepath.Base(p))
		case 3: // back to the original
			src = files[p]
			step = fmt.Sprintf("%d %s restored", i, filepath.Base(p))
		case 4: // a parameter added to the first function
			at := strings.Index(src, "fun ")
			open := strings.Index(src[max(at, 0):], "(")
			if at < 0 || open < 0 {
				continue
			}
			src = src[:at+open+1] + fmt.Sprintf("p%d: Int = 0, ", i) + src[at+open+1:]
			step = fmt.Sprintf("%d parameter added in %s", i, filepath.Base(p))
		}
		m.set(p, src)
		snaps := m.list()
		res, err := s.Update(ctx, snaps, opts)
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		sameAsAnalyze(t, lib, res, snaps, opts, step)
		if t.Failed() {
			return
		}
	}
	st := s.Stats()
	t.Logf("%d updates, %d incremental; whole: %v", st.Updates, st.Incremental, reasons)
	if st.Incremental*2 < st.Updates {
		t.Errorf("only %d of %d updates incremental", st.Incremental, st.Updates)
	}
}
