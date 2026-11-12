package analysis

import (
	"context"
	"os"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// FuzzEdits edits a.kt of a two-file module and checks that the session's
// update after the edit, incremental or not, finds what a fresh session
// does, and that neither panics.
func FuzzEdits(f *testing.F) {
	cfg := withStdlib(f)
	const a = "fun twice(x: Int) = x * 2\n\nfun f(n: Int): Int {\n    val y = twice(n)\n    if (y > 2) {\n        return y\n    }\n    return listOf(1, 2).map { it + y }.sum()\n}\n"
	b := psi.Parse("b.kt", "fun useB() = twice(2)\n\nclass BB {\n    fun m(): Int = f(1)\n}\n")
	f.Add(a, 61, 61, "foo(1, ")
	f.Add(a, 61, 61, "val z = ")
	f.Add(a, 61, 61, "y.")
	f.Add(a, 61, 61, "if (")
	f.Add(a, 61, 61, `"abc`)
	f.Add(a, 61, 61, "run {")
	f.Add(a, 72, 76, "")
	f.Add(a, 20, 21, "(")
	f.Add(a, 0, 0, "import ")
	f.Add(a, 120, 125, "?.")
	for _, p := range []string{"../testdata/run/hello.kt", "../testdata/run/stdlibReferences.kt"} {
		if src, err := os.ReadFile(p); err == nil {
			f.Add(string(src), len(src)/2, len(src)/2, "}")
			f.Add(string(src), len(src)/3, len(src)/3+5, "")
		}
	}
	s := Open(cfg)
	defer s.Close()
	ctx := context.Background()
	f.Fuzz(func(t *testing.T, src string, from, to int, text string) {
		if len(src) > 1<<14 || len(text) > 1<<10 {
			return
		}
		// Any offsets edit the source: from in it, and to after it.
		from = int(uint(from) % uint(len(src)+1))
		to = from + int(uint(to)%uint(len(src)-from+1))
		before := psi.Parse("a.kt", src)
		if _, err := s.Update(ctx, []*psi.Snapshot{before, b}); err != nil {
			t.Fatal(err)
		}
		after := psi.Reparse(before, from, to, text)
		snaps := []*psi.Snapshot{after, b}
		got, err := s.Update(ctx, snaps)
		if err != nil {
			t.Fatal(err)
		}
		if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
			t.Errorf("edit [%d, %d) to %q: %s", from, to, text, d)
		}
	})
}
