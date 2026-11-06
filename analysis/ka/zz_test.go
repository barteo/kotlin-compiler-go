package ka

import (
	"strings"
	"testing"
)

func TestZZ(t *testing.T) {
	m := update(t, "annotation class Anno\nclass Foo {\n  fun functionFoo(@Anno p1: Int) {\n    val (@Anno a, b) = Pair<Int, Int>(17, 42)\n  }\n}\n")
	var sb strings.Builder
	dump(&sb, m.Files[0].Snapshot, m.Files[0].Decls, "")
	t.Log(sb.String())
	for _, d := range m.Diagnostics {
		t.Logf("diag %v", d)
	}
}
