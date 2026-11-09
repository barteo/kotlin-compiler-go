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

// TestReferencesScan checks, on every program of testdata/run, that the
// references of each declaration in the sources are those a scan of the
// module's facts finds, and that the definition of each of them is the
// declaration, and that hover describes both.
func TestReferencesScan(t *testing.T) {
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
	if len(mods) == 0 {
		t.Fatal("no programs in testdata/run")
	}
	names := make([]string, 0, len(mods))
	for n := range mods {
		names = append(names, n)
	}
	slices.Sort(names)
	s := analysis.Open(cfg)
	defer s.Close()
	var programs, decls, refs, failed int
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
		programs++
		v := NewView(res, snaps)
		want := scanReferences(res, snaps)
		var errs []string
		for d, uses := range want {
			decls++
			refs += len(uses)
			dn := v.node(d)
			name := nameRange(dn)
			path := dn.ContainingFile().Path()
			if name.Start == name.End {
				continue
			}
			got := locs(References([]*View{v}, path, name.Start, false))
			if !slices.Equal(got, locs(sortLocs(uses))) {
				errs = append(errs, fmt.Sprintf("references of %s:\n got %q\nwant %q", locs([]Loc{{dn.ContainingFile(), name}}), got, locs(sortLocs(uses))))
			}
			if _, _, ok := Hover(v, path, name.Start); !ok {
				errs = append(errs, fmt.Sprintf("no hover on %q", locs([]Loc{{dn.ContainingFile(), name}})))
			}
			for _, u := range uses {
				if _, _, ok := Hover(v, u.Snap.Path(), u.Range.Start); !ok {
					errs = append(errs, fmt.Sprintf("no hover at %q", locs([]Loc{u})))
				}
				def := Definition(v, u.Snap.Path(), u.Range.Start)
				if len(def) != 1 || def[0].Snap != dn.ContainingFile() || def[0].Range != name {
					errs = append(errs, fmt.Sprintf("definition at %q is %q, want %q", locs([]Loc{u}), locs(def), locs([]Loc{{dn.ContainingFile(), name}})))
				}
			}
		}
		if len(errs) > 0 {
			failed++
			slices.Sort(errs)
			t.Errorf("%s: %d mismatches, first:\n%s", name, len(errs), strings.Join(errs[:min(3, len(errs))], "\n"))
		}
	}
	t.Logf("%d programs, %d declarations, %d references, %d failed", programs, decls, refs, failed)
}

// scanReferences returns the uses of each declaration in the sources the
// facts of res record: references whose symbol or call it declares,
// types of its class, and operators that call it.
func scanReferences(res *analysis.Result, snaps []*psi.Snapshot) map[analysis.ElementID][]Loc {
	d := res.Data()
	syms := map[uint32]analysis.Symbol{}
	classes := map[string]analysis.ElementID{}
	for _, s := range d.Symbols {
		syms[s.ID] = s
		if s.Kind == "class" && s.Declaration != nil {
			classes[s.QualifiedName] = *s.Declaration
		}
	}
	byID := map[psi.SnapshotID]*psi.Snapshot{}
	for _, s := range snaps {
		byID[s.ID()] = s
	}
	out := map[analysis.ElementID][]Loc{}
	for _, f := range d.Facts {
		snap := byID[f.Element.Snapshot]
		n := snap.Node(f.Element.Node)
		if f.Role == "declaration" {
			if _, ok := out[f.Element]; !ok {
				out[f.Element] = nil
			}
			continue
		}
		var decl *analysis.ElementID
		r := n.Range()
		switch n.NumericKind() {
		case psi.KindREFERENCE_EXPRESSION:
			decl = syms[f.Symbol].Declaration
			if decl == nil && f.Call != nil {
				decl = f.Call.Declaration
			}
		case psi.KindUSER_TYPE:
			var name psi.NodeRef
			for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
				if c.NumericKind() == psi.KindREFERENCE_EXPRESSION {
					name = c
				}
			}
			if f.Type == nil || !name.Valid() {
				break
			}
			r = name.Range()
			if f.Type.Parameter {
				if f.Type.ParameterOwner != nil {
					owner := byID[f.Type.ParameterOwner.Snapshot].Node(f.Type.ParameterOwner.Node)
					if p := typeParameter(owner, f.Type.Name); p.Valid() {
						decl = &analysis.ElementID{Snapshot: p.ContainingFile().ID(), Node: p.ID()}
					}
				}
			} else if el, ok := classes[f.Type.Name]; ok {
				decl = &el
			}
		case psi.KindBINARY_EXPRESSION, psi.KindPREFIX_EXPRESSION, psi.KindPOSTFIX_EXPRESSION:
			if op := n.ChildKind(psi.KindOPERATION_REFERENCE); op.Valid() && f.Call != nil {
				decl, r = f.Call.Declaration, op.Range()
			}
		}
		if decl != nil {
			out[*decl] = append(out[*decl], Loc{snap, r})
		}
	}
	return out
}
