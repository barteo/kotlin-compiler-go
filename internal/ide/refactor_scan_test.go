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

// TestRefactorScan applies refactorings throughout the programs of
// testdata/run: inline at every local val, extract variable at a sample
// of calls and qualified expressions, and specify type, remove type and
// convert a body at a sample of declarations. Each result must still
// check without errors. The ones known to break a program are listed in
// testdata/refactor-budget.txt; the list may only shrink.
func TestRefactorScan(t *testing.T) {
	budget := map[string]bool{}
	if b, err := os.ReadFile("testdata/refactor-budget.txt"); err == nil {
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
	applied := map[string]int{}
	var failed int
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
		for _, sn := range snaps {
			doc := NewDocument(sn.Path(), 1, sn.Source())
			for _, at := range refactorSample(sn) {
				for _, fx := range Refactors(v, doc, psi.Range{Start: at.off, End: max(at.off, at.end)}, FixEnv{}) {
					if fx.Kind != at.kind || at.kind == RefactorRewrite && strings.HasPrefix(fx.Title, "Add '") {
						continue
					}
					kind, _, _ := strings.Cut(fx.Title, " '")
					applied[kind]++
					var after []*psi.Snapshot
					for _, o := range snaps {
						src := o.Source()
						if o == sn {
							src = applyScanEdits(src, fx.Edits)
						}
						after = append(after, psi.Parse(o.Path(), src))
					}
					res, err := check.Update(context.Background(), after)
					if err != nil {
						t.Fatal(err)
					}
					line, col, _ := sn.Position(at.off)
					id := fmt.Sprintf("%s %s:%d:%d %s", name, sn.Path(), line, col, fx.Title)
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
						t.Errorf("%s: %s", id, strings.Join(msgs, "; "))
					} else if budget[id] && !testing.Short() {
						t.Errorf("%s now applies cleanly: remove it from testdata/refactor-budget.txt", id)
					}
				}
			}
		}
	}
	t.Logf("applied %v, %d failed (%d budgeted)", applied, failed, len(budget))
	if os.Getenv("KC_REFACTOR_BUDGET") != "" {
		ids := make([]string, 0, len(failing))
		for id := range failing {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		os.WriteFile("testdata/refactor-budget.txt", []byte("# Refactorings TestRefactorScan knows to break a program: module path:line:col title.\n"+strings.Join(ids, "\n")+"\n"), 0o644)
	}
}

type scanPoint struct {
	off, end int // a caret, or a selection
	kind     string
}

// refactorSample returns where to refactor in snap, and which kind:
// every local val's name to inline; every seventh call or qualified
// expression, selected, to extract; every third declaration's name, to
// rewrite; and the constructs the rewrite intentions convert: half the
// statement keywords, each it, lambda parameter, string template, String
// concatenation and local declared without a value, and a fifth of the
// argument lists.
func refactorSample(snap *psi.Snapshot) []scanPoint {
	var out []scanPoint
	exprs, decls, rewrites := 0, 0, 0
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindPROPERTY:
			if id := n.NameIdentifier(); id.Valid() {
				if n.Parent().NumericKind() == psi.KindBLOCK && n.ChildKind(psi.KindVAL_KEYWORD).Valid() {
					out = append(out, scanPoint{id.Range().Start, 0, RefactorInline})
				}
				if decls%3 == 0 {
					out = append(out, scanPoint{id.Range().Start, 0, RefactorRewrite})
				}
				decls++
			}
		case psi.KindFUN:
			if id := n.NameIdentifier(); id.Valid() {
				if decls%3 == 0 {
					out = append(out, scanPoint{id.Range().Start, 0, RefactorRewrite})
				}
				decls++
			}
		case psi.KindIF_KEYWORD, psi.KindELSE_KEYWORD, psi.KindWHEN_KEYWORD, psi.KindFOR_KEYWORD, psi.KindWHILE_KEYWORD, psi.KindDO_KEYWORD:
			if rewrites%2 == 0 {
				out = append(out, scanPoint{n.Range().Start, 0, RefactorRewrite})
			}
			rewrites++
		case psi.KindREFERENCE_EXPRESSION:
			if n.Text() == "it" {
				out = append(out, scanPoint{n.Range().Start, 0, RefactorRewrite})
			}
		case psi.KindVALUE_PARAMETER:
			if id := n.NameIdentifier(); id.Valid() && n.Parent().Parent().NumericKind() == psi.KindFUNCTION_LITERAL {
				out = append(out, scanPoint{id.Range().Start, 0, RefactorRewrite})
			}
		case psi.KindSTRING_TEMPLATE:
			out = append(out, scanPoint{n.Range().Start + 1, 0, RefactorRewrite})
		case psi.KindVALUE_ARGUMENT_LIST:
			if rewrites%5 == 0 {
				out = append(out, scanPoint{n.Range().Start + 1, 0, RefactorRewrite})
			}
			rewrites++
		}
		switch n.NumericKind() {
		case psi.KindBINARY_EXPRESSION:
			if n.ChildKind(psi.KindOPERATION_REFERENCE).FirstChild().NumericKind() == psi.KindPLUS && n.Parent().NumericKind() != psi.KindBINARY_EXPRESSION {
				out = append(out, scanPoint{n.ChildKind(psi.KindOPERATION_REFERENCE).Range().Start, 0, RefactorRewrite})
			}
		case psi.KindPROPERTY:
			if id := n.NameIdentifier(); id.Valid() && n.Parent().NumericKind() == psi.KindBLOCK && !n.ChildKind(psi.KindEQ).Valid() {
				out = append(out, scanPoint{id.Range().Start, 0, RefactorRewrite})
			}
		case psi.KindCALL_EXPRESSION, psi.KindDOT_QUALIFIED_EXPRESSION:
			if exprs%7 == 0 {
				out = append(out, scanPoint{n.Range().Start, n.Range().End, RefactorExtract})
			}
			exprs++
		}
		return true
	})
	slices.SortStableFunc(out, func(a, b scanPoint) int { return a.off - b.off })
	return slices.Compact(out)
}

// applyScanEdits applies edits to src, from the end.
func applyScanEdits(src string, edits []FixEdit) string {
	edits = slices.Clone(edits)
	slices.SortStableFunc(edits, func(a, b FixEdit) int { return b.Range.Start - a.Range.Start })
	for _, e := range edits {
		src = src[:e.Range.Start] + e.Text + src[e.Range.End:]
	}
	return src
}
