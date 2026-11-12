package psi

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// sameSnapshot fails unless a and b have the same text, tree, diagnostics
// and lazy blocks.
func sameSnapshot(t testing.TB, what string, a, b *Snapshot) {
	t.Helper()
	switch {
	case a.src != b.src:
		t.Fatalf("%s: text differs", what)
	case !reflect.DeepEqual(a.nodes, b.nodes):
		for i := range min(len(a.nodes), len(b.nodes)) {
			if a.nodes[i] != b.nodes[i] {
				t.Fatalf("%s: node %d: %+v, whole parse %+v (%d vs %d nodes)", what, i, a.nodes[i], b.nodes[i], len(a.nodes), len(b.nodes))
			}
		}
		t.Fatalf("%s: %d nodes, whole parse %d", what, len(a.nodes), len(b.nodes))
	case !reflect.DeepEqual(a.diags, b.diags) && (len(a.diags) != 0 || len(b.diags) != 0):
		t.Fatalf("%s: diagnostics %v, whole parse %v", what, a.diags, b.diags)
	case !reflect.DeepEqual(a.lazy, b.lazy) && (len(a.lazy) != 0 || len(b.lazy) != 0):
		t.Fatalf("%s: lazy %v, whole parse %v", what, a.lazy, b.lazy)
	}
}

var snippets = []string{
	"", "x", " ", "\n", "val y = 1\n", "(", ")", "{", "}", "{ }", "\"", "\"$", "\"${", "'", "/*", "*/", "//",
	"\"\"\"", "if (a) b else c", "fun g() {}", "class C", "->", ".", "?.", "<", ">", "when (x) { 1 -> {} }",
	"listOf(1).map { it }", "return", "@A", "`", "\\", "é", "😀",
}

// edit returns a random edit of s, mostly inside one of its lazy blocks.
func edit(r *rand.Rand, s *Snapshot) (from, to int, text string) {
	lo, hi := 0, len(s.src)
	if len(s.lazy) > 0 && r.Intn(8) != 0 {
		n := s.nodes[s.lazy[r.Intn(len(s.lazy))].id]
		lo, hi = int(n.start), int(n.end)
	}
	from = lo + r.Intn(hi-lo+1)
	to = from
	if r.Intn(3) == 0 {
		to = min(hi, from+r.Intn(12))
	}
	text = snippets[r.Intn(len(snippets))]
	return from, to, text
}

// TestReparse checks that Reparse equals a whole parse after chains of
// random edits to every testdata file.
func TestReparse(t *testing.T) {
	for seed := int64(1); seed <= *reparseSeeds; seed++ {
		testReparse(t, seed)
	}
}

var reparseSeeds = flag.Int64("reparse-seeds", 1, "random edit chains per file in TestReparse")

func testReparse(t *testing.T, seed int64) {
	files := corpusFiles(t)
	if testing.Short() {
		files = files[:min(len(files), 40)]
	}
	r := rand.New(rand.NewSource(seed))
	var p Parser
	edits, spliced := 0, 0
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join("..", f))
		if err != nil {
			t.Fatal(err)
		}
		path := f
		if !strings.HasSuffix(f, ".kts") {
			path = filepath.Base(f)
		}
		s := Parse(path, string(src))
		for i := 0; i < 6; i++ {
			from, to, text := edit(r, s)
			got := p.Reparse(s, from, to, text)
			want := Parse(path, got.src)
			sameSnapshot(t, fmt.Sprintf("%s: seed %d edit %d: [%d,%d) %q -> %q in\n%s", f, seed, i, from, to, s.src[from:to], text, got.src[max(0, from-150):min(len(got.src), from+150)]), got, want)
			checkTree(t, got)
			edits++
			if got.Reparsed() {
				spliced++
			}
			s = got
		}
	}
	t.Logf("%d edits, %d spliced", edits, spliced)
	if spliced < edits/3 {
		t.Errorf("only %d of %d edits spliced", spliced, edits)
	}
}

func FuzzReparse(f *testing.F) {
	f.Add("fun f() { val x = 1 }", 12, 13, "y")
	f.Add("class A { fun f() { g { h() } } }", 22, 22, "}")
	f.Add("fun f() { val s = \"${ fun() { x } }\" }", 30, 30, "`")
	f.Add("fun f() {\n  a`b\n}\nfun g() { \"${1}\" }", 12, 12, "\"")
	f.Fuzz(func(t *testing.T, src string, from, to int, text string) {
		if from < 0 || to < from || to > len(src) {
			return
		}
		got := Reparse(Parse("f.kt", src), from, to, text)
		sameSnapshot(t, "reparse", got, Parse("f.kt", got.src))
	})
}
