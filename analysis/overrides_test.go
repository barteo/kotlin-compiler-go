package analysis

import (
	"slices"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestOverrides(t *testing.T) {
	cfg := withStdlib(t)
	r, s := analyzed(t, cfg, `interface Shape { fun area(): Double }
abstract class Base : Shape { abstract override fun area(): Double; open fun name() = "base" }
open class Square(val side: Double) : Base() {
    override fun area() = side * side
    override fun name() = "square"
    override fun toString() = name()
}
class Tile : Square(1.0) { override fun area() = 2.0 }
`)
	decls := map[string][]psi.NodeRef{}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if n.NumericKind() == psi.KindFUN {
			decls[n.Name()] = append(decls[n.Name()], n)
		}
		return true
	})
	overrides := func(n psi.NodeRef) []psi.NodeRef {
		t.Helper()
		var out []psi.NodeRef
		for _, el := range symbol(t, r, fact(t, r, n).Symbol).Overrides {
			if el.Snapshot != s.ID() {
				t.Fatalf("override in snapshot %d", el.Snapshot)
			}
			out = append(out, s.Node(el.Node))
		}
		return out
	}
	area, name := decls["area"], decls["name"]
	for _, c := range []struct {
		fun  psi.NodeRef
		want []psi.NodeRef
	}{
		{area[0], nil},
		{area[1], area[:1]},
		{area[2], []psi.NodeRef{area[1], area[0]}},
		{area[3], []psi.NodeRef{area[2], area[1], area[0]}},
		{name[0], nil},
		{name[1], name[:1]},
		{decls["toString"][0], nil}, // Any's is in the library
	} {
		if got := overrides(c.fun); !slices.Equal(got, c.want) {
			t.Errorf("%q overrides %v, want %v", c.fun.Text(), got, c.want)
		}
	}
	// FileFacts walks one file's facts in order.
	var n int
	last := psi.NodeID(0)
	for f := range r.FileFacts(s.ID()) {
		if f.Element.Node <= last {
			t.Fatalf("fact of node %d after %d", f.Element.Node, last)
		}
		last = f.Element.Node
		n++
	}
	if n != len(r.Data().Facts) || !slices.Equal(r.Snapshots(), []psi.SnapshotID{s.ID()}) {
		t.Errorf("FileFacts has %d facts of %d; snapshots %v", n, len(r.Data().Facts), r.Snapshots())
	}
}
