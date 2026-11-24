package ka

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestFactsLikeAnalyze checks that an OpenFacts session's facts are
// package analysis's: those Analyze makes of the same sources.
func TestFactsLikeAnalyze(t *testing.T) {
	dir := t.TempDir()
	srcs := map[string]string{
		"a.kt": `package p

sealed interface Shape
class Circle(val r: Int) : Shape
object Empty : Shape

fun area(s: Shape?): Int {
    if (s == null) return 0
    val n = when (s) {
        is Circle -> s.r * s.r
        Empty -> 0
    }
    listOf(1, 2).forEach { println(it + n) }
    return n
}
`,
		"b.kt": `package p

class Box<T>(var value: T) {
    fun <R> map(f: (T) -> R): Box<R> = Box(f(value))
}

fun use(b: Box<String>): Int = b.map { it.length }.value ?: 1
`,
	}
	var sources []Source
	var snaps []*psi.Snapshot
	for _, name := range []string{"a.kt", "b.kt"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(srcs[name]), 0o666); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, Source{Path: p})
		snaps = append(snaps, psi.Parse(p, srcs[name]))
	}
	cfg := withStdlib(t)
	cfg.ModuleName, cfg.Jobs = "m", 2
	want, err := analysis.Analyze(context.Background(), cfg, snaps)
	if err != nil {
		t.Fatal(err)
	}
	if d := want.Diagnostics(); len(d) > 0 {
		t.Fatalf("Analyze: %v", d)
	}
	s := OpenFacts(cfg)
	defer s.Close()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if len(m.Diagnostics) > 0 {
		t.Fatalf("Update: %v", m.Diagnostics)
	}
	if m.Facts == nil {
		t.Fatal("no facts")
	}
	// The snapshots differ, so compare the facts by file index and node.
	norm := func(d analysis.Data, files []*psi.Snapshot) string {
		b, _ := json.Marshal(d)
		out := string(b)
		for i, s := range files {
			out = strings.ReplaceAll(out, fmt.Sprintf(`"snapshot":%d,`, s.ID()), fmt.Sprintf(`"snapshot":f%d,`, i))
		}
		return out
	}
	var got []*psi.Snapshot
	for _, f := range m.Files {
		got = append(got, f.Snapshot)
	}
	if g, w := norm(m.Facts.Data(), got), norm(want.Data(), snaps); g != w {
		i := 0
		for i < len(g) && i < len(w) && g[i] == w[i] {
			i++
		}
		from := max(i-300, 0)
		t.Errorf("facts differ at %d:\n got …%.900s\nwant …%.900s", i, g[from:], w[from:])
	}

	plain := Open(cfg)
	defer plain.Close()
	pm, err := plain.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	defer pm.Close()
	if pm.Facts != nil {
		t.Error("facts without OpenFacts")
	}
}
