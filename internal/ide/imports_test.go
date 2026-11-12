package ide

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestOrganizeImports(t *testing.T) {
	lib := `package lib

class Box(val x: Int)
class Other
object Tools { fun twice(x: Int) = 2 * x }
fun helper() = 1
val answer = 42
operator fun Box.plus(o: Box) = Box(x + o.x)
fun Box.ext() = x
`
	for _, tc := range []struct{ src, want string }{
		{`package p

import lib.Other
import kotlin.math.abs
import lib.Box
import lib.helper

fun f() = abs(helper()) + Box(1).x
`, `package p

import lib.Box
import lib.helper
import kotlin.math.abs

fun f() = abs(helper()) + Box(1).x
`},
		{`package p

import lib.Other
import lib.answer

fun f() = 1
`, `package p

fun f() = 1
`},
		{`package p

import lib.answer
import lib.Box as B
import lib.plus
import lib.ext
import lib.Tools.twice
import kotlin.math.max

/** Uses [Other]. */
fun f(): Int {
    val l = max(1, 2)
    return answer + (B(1) + B(2)).ext() + twice(l)
}
`, `package p

import lib.Tools.twice
import lib.answer
import lib.ext
import lib.plus
import kotlin.math.max
import lib.Box as B

/** Uses [Other]. */
fun f(): Int {
    val l = max(1, 2)
    return answer + (B(1) + B(2)).ext() + twice(l)
}
`},
		{`package p

import lib.*

fun f() = 1
`, `package p

fun f() = 1
`},
		{`package p

import lib.*

fun f() = helper()
`, ``},
	} {
		v := testView(t, "lib.kt", lib, "a.kt", tc.src)
		e, ok := OrganizeImports(v, "a.kt")
		got := tc.src
		if ok {
			got = tc.src[:e.Range.Start] + e.Text + tc.src[e.Range.End:]
		} else if tc.want != "" {
			t.Errorf("no edit for\n%s", tc.src)
			continue
		}
		if tc.want == "" {
			tc.want = tc.src
		}
		if got != tc.want {
			t.Errorf("organized\n%s\ngot\n%s\nwant\n%s", tc.src, got, tc.want)
		}
	}
}

func TestImportEdit(t *testing.T) {
	for _, tc := range []struct{ src, fq, want string }{
		{"package p\n\nimport a.B\nimport c.D\n\nfun f() = 1\n", "b.C", "package p\n\nimport a.B\nimport b.C\nimport c.D\n\nfun f() = 1\n"},
		{"package p\n\nimport a.B\nimport kotlin.math.abs\n", "java.io.File", "package p\n\nimport a.B\nimport java.io.File\nimport kotlin.math.abs\n"},
		{"package p\n\nimport c.D\nimport a.B\n", "b.C", "package p\n\nimport c.D\nimport a.B\nimport b.C\n"},
		{"package p\n\nfun f() = 1\n", "b.C", "package p\n\nimport b.C\n\nfun f() = 1\n"},
		{"fun f() = 1\n", "b.C", "import b.C\n\nfun f() = 1\n"},
	} {
		e := ImportEdit(psi.Parse("a.kt", tc.src), tc.fq)
		if got := tc.src[:e.Range.Start] + e.Text + tc.src[e.Range.End:]; got != tc.want {
			t.Errorf("importing %s into\n%s\ngot\n%s", tc.fq, tc.src, got)
		}
	}
}

// TestOrganizeImportsScan organizes the imports of every program of
// testdata/run, with an unused import added to each file, and checks
// that the added import is the one removed and the program still checks
// without errors.
func TestOrganizeImportsScan(t *testing.T) {
	cfg := stdlib(t)
	if _, err := classpath.FindJDK(); err != nil {
		t.Skipf("no JDK (set JAVA_HOME): %v", err)
	}
	cfg.NoJDK = false
	files, _ := filepath.Glob("../../testdata/run/*.kt")
	dirs, _ := filepath.Glob("../../testdata/run/*/main.kt")
	mods := map[string][]string{}
	for _, f := range files {
		mods[strings.TrimSuffix(filepath.Base(f), ".kt")] = []string{f}
	}
	for _, d := range dirs {
		kts, _ := filepath.Glob(filepath.Join(filepath.Dir(d), "*.kt"))
		mods[filepath.Base(filepath.Dir(d))] = kts
	}
	names := make([]string, 0, len(mods))
	for n := range mods {
		names = append(names, n)
	}
	slices.Sort(names)
	s := analysis.Open(cfg)
	defer s.Close()
	check := analysis.Open(cfg)
	defer check.Close()
	const extra = "java.util.zip.Adler32"
	var organized, removed int
	for k, name := range names {
		if testing.Short() && k%10 != 0 {
			continue
		}
		var snaps []*psi.Snapshot
		for _, p := range mods[name] {
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			snap := psi.Parse(filepath.Base(p), string(src))
			e := ImportEdit(snap, extra)
			snaps = append(snaps, psi.Parse(snap.Path(), snap.Source()[:e.Range.Start]+e.Text+snap.Source()[e.Range.End:]))
		}
		res, err := s.Update(context.Background(), snaps)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Diagnostics()) > 0 {
			continue
		}
		v := NewView(res, snaps)
		var after []*psi.Snapshot
		for _, sn := range snaps {
			src := sn.Source()
			unused := UnusedImports(v, sn.Path())
			removed += len(unused)
			if !slices.ContainsFunc(unused, func(r psi.Range) bool { return strings.Contains(src[r.Start:r.End], extra) }) {
				t.Errorf("%s/%s: the import of %s isn't reported unused", name, sn.Path(), extra)
			}
			if e, ok := OrganizeImports(v, sn.Path()); ok {
				organized++
				src = src[:e.Range.Start] + e.Text + src[e.Range.End:]
				if strings.Contains(src, extra) {
					t.Errorf("%s/%s: the unused import of %s is kept", name, sn.Path(), extra)
				}
			}
			after = append(after, psi.Parse(sn.Path(), src))
		}
		res, err = check.Update(context.Background(), after)
		if err != nil {
			t.Fatal(err)
		}
		if ds := res.Diagnostics(); len(ds) > 0 {
			t.Errorf("%s: organized imports break it: %s:%d:%d %s", name, ds[0].Path, ds[0].Line, ds[0].Column, ds[0].Message)
		}
	}
	t.Logf("%d files organized, %d imports removed", organized, removed)
}
