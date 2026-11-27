package analysis

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// storeCorpus analyzes the programs of testdata/run, each on its own,
// and calls fn with each result.
func storeCorpus(t *testing.T, fn func(name string, r *Result, s *psi.Snapshot)) {
	t.Helper()
	cfg := withStdlib(t)
	lib, err := OpenLibrary(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	paths, err := filepath.Glob("../testdata/run/*.kt")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no programs: %v", err)
	}
	if testing.Short() {
		paths = paths[:min(len(paths), 40)]
	}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := psi.Parse(filepath.Base(p), string(src))
		r, err := lib.Analyze(context.Background(), []*psi.Snapshot{s}, Options{})
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		fn(strings.TrimSuffix(filepath.Base(p), ".kt"), r, s)
	}
}

// TestStoreRoundTrip checks that storing a fact and reading it back gives
// the same fact, nil slices apart from empty ones.
func TestStoreRoundTrip(t *testing.T) {
	n := 0
	storeCorpus(t, func(name string, r *Result, s *psi.Snapshot) {
		facts := r.Data().Facts
		ptrs := make([]*Fact, len(facts))
		for i := range facts {
			ptrs[i] = &facts[i]
		}
		ff := newStore(nil).freezeFile(s.ID(), ptrs)
		for i := range facts {
			if got := ff.fact(i); !reflect.DeepEqual(got, facts[i]) {
				t.Fatalf("%s: stored %+v\nread back %+v", name, facts[i], got)
			}
		}
		n += len(facts)
	})
	if n == 0 {
		t.Error("no facts")
	}
}

// TestStoreSharing checks that a Result stores each distinct type and
// signature once, and a call and its callee's reference as one record.
func TestStoreSharing(t *testing.T) {
	r, s := analyzed(t, withStdlib(t), "fun f(a: List<String>, b: List<String>) { println(a.size); println(b.size) }\n")
	ff := &r.files[0]
	get := func(kind, text string, nth int) *storedFact {
		t.Helper()
		i, ok := ff.find(at(t, s, kind, text, nth).ID())
		if !ok {
			t.Fatalf("no fact of %s %q", kind, text)
		}
		return &ff.facts[i]
	}
	a, b := get("REFERENCE_EXPRESSION", "a", 0), get("REFERENCE_EXPRESSION", "b", 0)
	if a.typ == nil || a.typ != b.typ {
		t.Errorf("List<String> stored twice: %p, %p", a.typ, b.typ)
	}
	call1, call2 := get("CALL_EXPRESSION", "println(a.size)", 0), get("CALL_EXPRESSION", "println(b.size)", 0)
	if call1.call == nil || call1.call != call2.call {
		t.Errorf("println(Int) stored twice: %p, %p", call1.call, call2.call)
	}
	if callee := get("REFERENCE_EXPRESSION", "println", 0); callee.call != call1.call {
		t.Errorf("callee's call isn't its call's: %p, %p", callee.call, call1.call)
	}
}

// TestFacts checks that Facts visits the facts Fact returns, in node
// order, without copying them.
func TestFacts(t *testing.T) {
	storeCorpus(t, func(name string, r *Result, s *psi.Snapshot) {
		var want []Fact
		for _, f := range r.Data().Facts {
			if f.Element.Snapshot == s.ID() {
				want = append(want, f)
			}
		}
		i := 0
		r.Facts(s.ID(), func(f *Fact) bool {
			if i >= len(want) {
				t.Fatalf("%s: more facts than Data's %d", name, len(want))
			}
			if got := copyFact(*f); !reflect.DeepEqual(got, want[i]) {
				t.Fatalf("%s: fact %d\n got %+v\nwant %+v", name, i, got, want[i])
			}
			i++
			return true
		})
		if i != len(want) {
			t.Fatalf("%s: %d facts, want %d", name, i, len(want))
		}
		if len(want) > 1 {
			n := 0
			r.Facts(s.ID(), func(*Fact) bool { n++; return false })
			if n != 1 {
				t.Errorf("%s: %d facts visited after false", name, n)
			}
		}
		// The fact, its calls and their buffers, however many facts: a few,
		// more with -race, where an allocation a fact would be hundreds.
		if allocs := testing.AllocsPerRun(5, func() { r.Facts(s.ID(), func(*Fact) bool { return true }) }); allocs > 16 {
			t.Errorf("%s: Facts makes %v allocations over %d facts", name, allocs, len(want))
		}
	})
	var nilResult *Result
	nilResult.Facts(1, func(*Fact) bool { t.Error("a nil Result's fact"); return true })
}
