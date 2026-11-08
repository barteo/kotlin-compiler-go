package analysis

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// runModules returns the programs of testdata/run, each as its files:
// X.kt, or the .kt files of X/ for a directory with a main.kt.
func runModules(t *testing.T) map[string][]string {
	files, _ := filepath.Glob("../testdata/run/*.kt")
	dirs, _ := filepath.Glob("../testdata/run/*/main.kt")
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
	return mods
}

func parseFiles(t *testing.T, paths []string) []*psi.Snapshot {
	t.Helper()
	var snaps []*psi.Snapshot
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		snaps = append(snaps, psi.Parse(filepath.Base(p), string(src)))
	}
	return snaps
}

// TestSessionMatchesAnalyze checks that a session's updates, which share
// one library, find what AnalyzeArena finds with a library of its own, on
// every program of testdata/run.
func TestSessionMatchesAnalyze(t *testing.T) {
	cfg := withJDK(t)
	s := Open(cfg)
	defer s.Close()
	ctx := context.Background()
	n, complete := 0, 0
	for name, paths := range runModules(t) {
		if testing.Short() && n >= 60 {
			break
		}
		n++
		snaps := parseFiles(t, paths)
		want, err := AnalyzeArena(ctx, cfg, snaps)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := s.Update(ctx, snaps)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want.Complete() {
			complete++
			if !got.Complete() {
				t.Errorf("%s: session has diagnostics %v", name, got.Diagnostics())
			} else if !reflect.DeepEqual(got.Data(), want.Data()) {
				t.Errorf("%s: session's facts differ from AnalyzeArena's", name)
			}
			continue
		}
		// With errors, the session reports at least AnalyzeArena's, and
		// lowering errors stop AnalyzeArena before the checker.
		have := map[Diagnostic]bool{}
		for _, d := range got.Diagnostics() {
			have[d] = true
		}
		for _, d := range want.Diagnostics() {
			if !have[d] {
				t.Errorf("%s: session lacks %v", name, d)
			}
		}
	}
	t.Logf("%d programs, %d without errors", n, complete)
	if libraryOpens.Load() > 1 {
		t.Errorf("the session opened %d libraries", libraryOpens.Load())
	}
}

// TestSessionPartial checks that a file with an error loses its facts and
// the others keep theirs.
func TestSessionPartial(t *testing.T) {
	cfg := withStdlib(t)
	s := Open(cfg)
	defer s.Close()
	a := psi.Parse("a.kt", "package p\nfun twice(x: Int) = x * 2\n")
	b := psi.Parse("b.kt", "package p\nfun g(): String = twice(1)\n")
	c := psi.Parse("c.kt", "package p\nfun h() = twice(2) + 1\n")
	res, err := s.Update(context.Background(), []*psi.Snapshot{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if res.Complete() || len(res.Diagnostics()) != 1 || res.Diagnostics()[0].Path != "b.kt" {
		t.Fatalf("diagnostics %v, want one in b.kt", res.Diagnostics())
	}
	files := map[psi.SnapshotID]int{}
	for _, f := range res.Data().Facts {
		files[f.Element.Snapshot]++
	}
	if files[b.ID()] != 0 || files[a.ID()] == 0 || files[c.ID()] == 0 {
		t.Errorf("facts per file a=%d b=%d c=%d, want none in b only", files[a.ID()], files[b.ID()], files[c.ID()])
	}
	// AnalyzeArena stays all or nothing.
	all, err := AnalyzeArena(context.Background(), cfg, []*psi.Snapshot{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Data().Facts) != 0 {
		t.Errorf("AnalyzeArena has %d facts with errors", len(all.Data().Facts))
	}
	// A syntax error in one file: the others are still checked.
	broken := psi.Parse("b.kt", "package p\nfun g(: String = twice(1)\n")
	res, err = s.Update(context.Background(), []*psi.Snapshot{a, broken, c})
	if err != nil {
		t.Fatal(err)
	}
	files = map[psi.SnapshotID]int{}
	for _, f := range res.Data().Facts {
		files[f.Element.Snapshot]++
	}
	if files[broken.ID()] != 0 || files[a.ID()] == 0 || files[c.ID()] == 0 {
		t.Errorf("with a syntax error: facts per file a=%d b=%d c=%d", files[a.ID()], files[broken.ID()], files[c.ID()])
	}
}
