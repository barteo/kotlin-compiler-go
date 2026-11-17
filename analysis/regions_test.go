package analysis

import (
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestErrorSpans checks which code an error at each place invalidates.
func TestErrorSpans(t *testing.T) {
	src := `package p

class A(val n: Int) : B {
    val p = 1 + 2
    val q: Int
        get() { val g1 = 1; return g1 }
    fun f() {
        val s1 = 1
        val s2 = 2
        if (s1 > 0) { val s3 = 3 }
        val s4 = 4
    }
    fun e() = n * 2
}

fun top() = 3
`
	all := []string{"val p = 1 + 2", "1 + 2", "val g1 = 1", "return g1", "val s1 = 1", "val s2 = 2", "val s3 = 3", "val s4 = 4", "n * 2", ": B", "fun top() = 3", "3\n"}
	tests := []struct {
		at   string // the error is at its first occurrence
		gone []string
	}{
		{"val s2", []string{"val s2 = 2", "val s3 = 3", "val s4 = 4"}},
		{"val s3", []string{"val s3 = 3", "val s4 = 4"}},
		{"}\n    fun e", nil}, // a missing return
		{"n * 2", []string{"n * 2"}},
		{"return g1", []string{"return g1"}},
		{"B {", []string{": B"}},
		{"1 + 2", []string{"1 + 2"}},
		{"3\n", []string{"3\n"}}, // top's expression, but not top
		{"top", []string{"fun top() = 3", "3\n"}},
	}
	snap := psi.Parse("a.kt", src)
	for _, tt := range tests {
		off := strings.Index(src, tt.at)
		spans, r := errorSpans(snap, off)
		if r == reachFile {
			t.Errorf("at %q: the whole file", tt.at)
			continue
		}
		for _, s := range all {
			n := nodeOf(t, snap, s)
			want := false
			for _, g := range tt.gone {
				want = want || g == s
			}
			if got := dropped(spans, n.ID()); got != want {
				t.Errorf("at %q: %q dropped %v, want %v", tt.at, s, got, want)
			}
		}
	}
	if _, r := errorSpans(snap, strings.Index(src, "package")); r != reachFile {
		t.Error("in the package directive: not the whole file")
	}
}

// nodeOf returns the outermost node of snap whose text is s's first
// occurrence.
func nodeOf(t *testing.T, snap *psi.Snapshot, s string) psi.NodeRef {
	t.Helper()
	start := strings.Index(snap.Source(), s)
	r := psi.Range{Start: start, End: start + len(s)}
	var found psi.NodeRef
	for n := snap.NodeAt(start); n.Valid(); n = n.Parent() {
		if n.Range() == r {
			found = n
		}
	}
	if !found.Valid() {
		// ": B" is a COLON and a supertype list: its first node will do.
		found = snap.NodeAt(start)
	}
	return found
}
