package ide

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestRenameScan renames declarations of every program of testdata/run,
// a sample of each kind per program, and checks that the result still
// checks without errors. The renames known to break a program, from
// uses the analysis has no facts for, are listed in
// testdata/rename-budget.txt; the list may only shrink.
func TestRenameScan(t *testing.T) {
	budget := map[string]bool{}
	if b, err := os.ReadFile("testdata/rename-budget.txt"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				budget[l] = true
			}
		}
	}
	failing := map[string]bool{}
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
	var renamed, refused, failed int
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
			snaps = append(snaps, psi.Parse(filepath.Base(p), string(src)))
		}
		res, err := s.Update(context.Background(), snaps)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Diagnostics()) > 0 {
			continue
		}
		v := NewView(res, snaps)
		for _, d := range renameSample(snaps) {
			path := d.ContainingFile().Path()
			locs, text, err := Rename([]*View{v}, path, nameRange(d).Start, d.Name()+"R9")
			if err != nil {
				refused++
				continue
			}
			renamed++
			edited := applyRename(locs, text)
			var after []*psi.Snapshot
			for _, sn := range snaps {
				src, ok := edited[sn.Path()]
				if !ok {
					src = sn.Source()
				}
				after = append(after, psi.Parse(sn.Path(), src))
			}
			res, err := check.Update(context.Background(), after)
			if err != nil {
				t.Fatal(err)
			}
			line, col, _ := d.ContainingFile().Position(d.Range().Start)
			id := fmt.Sprintf("%s %s:%d:%d %s", name, path, line, col, d.Name())
			if ds := res.Diagnostics(); len(ds) > 0 {
				failed++
				failing[id] = true
				if budget[id] {
					continue
				}
				var msgs []string
				for _, e := range ds[:min(3, len(ds))] {
					msgs = append(msgs, fmt.Sprintf("%s:%d:%d %s", e.Path, e.Line, e.Column, e.Message))
				}
				t.Errorf("%s: renaming %s %s at %s:%d:%d: %s", name, d.Kind(), d.Name(), path, line, col, strings.Join(msgs, "; "))
			} else if budget[id] && !testing.Short() {
				t.Errorf("%s now renames cleanly: remove it from testdata/rename-budget.txt", id)
			}
		}
	}
	t.Logf("%d renamed, %d refused, %d failed (%d budgeted)", renamed, refused, failed, len(budget))
	if os.Getenv("KC_RENAME_BUDGET") != "" {
		ids := make([]string, 0, len(failing))
		for id := range failing {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		os.WriteFile("testdata/rename-budget.txt", []byte("# Renames TestRenameScan knows to break a program: module path:line:col name.\n"+strings.Join(ids, "\n")+"\n"), 0o644)
	}
}

// renameSample returns declarations to rename: of each kind, the first
// and every fifth after it, per program.
func renameSample(snaps []*psi.Snapshot) []psi.NodeRef {
	seen := map[psi.Kind]int{}
	var out []psi.NodeRef
	for _, s := range snaps {
		psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
			switch n.NumericKind() {
			case psi.KindFUN, psi.KindPROPERTY, psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindVALUE_PARAMETER,
				psi.KindTYPE_PARAMETER, psi.KindTYPEALIAS, psi.KindDESTRUCTURING_DECLARATION_ENTRY:
				if n.NameIdentifier().Valid() && n.Name() != "main" {
					if seen[n.NumericKind()]%5 == 0 {
						out = append(out, n)
					}
					seen[n.NumericKind()]++
				}
			}
			return true
		})
	}
	return out
}
