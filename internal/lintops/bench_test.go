package lintops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// The stages of the lint operations, timed on real sources:
//
//	KC_LINT_SRC=dir1:dir2 KC_LINT_CP=a.jar:b.jar go test ./internal/lintops -run TestLintStages -v
//
// It prints the time of each stage that OpLintAnalyze and OpLintFile run,
// as detekt on kc calls them: parsing, analysis.Analyze, Result.Data and
// the split by file, and encoding every file's facts, one by one and on
// all cores. It is skipped without KC_LINT_SRC.
func TestLintStages(t *testing.T) {
	src := os.Getenv("KC_LINT_SRC")
	if src == "" {
		t.Skip("KC_LINT_SRC is not set")
	}
	var paths []string
	for _, dir := range filepath.SplitList(src) {
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".kt") {
				paths = append(paths, p)
			}
			return nil
		})
	}
	cfg := analysis.Config{ModuleName: "bench", Jobs: 4, ClassPath: filepath.SplitList(os.Getenv("KC_LINT_CP"))}
	cfg.OptIns = strings.Fields(os.Getenv("KC_LINT_OPTINS"))
	for round := range 3 {
		stage := time.Now()
		lap := func(name string) {
			t.Logf("round %d: %-34s %6.1f ms", round, name, float64(time.Since(stage).Microseconds())/1000)
			stage = time.Now()
		}
		snaps := make([]*psi.Snapshot, len(paths))
		parallel(len(paths), cfg.Jobs, func(i int) {
			b, _ := os.ReadFile(paths[i])
			snaps[i] = psi.Parse(paths[i], string(b))
		})
		lap("parse")
		res, err := analysis.Analyze(context.Background(), cfg, snaps)
		if err != nil {
			t.Fatal(err)
		}
		lap("analysis.Analyze")
		if d := res.Diagnostics(); len(d) > 0 {
			t.Fatalf("%d diagnostics, first: %v", len(d), d[0])
		}
		if os.Getenv("KC_LINT_COPY") != "" {
			res.Data()
			lap("Result.Data (deep copy, as before)")
		}
		facts, symbols := res.SharedData()
		lap("Result.SharedData")
		out := &result{facts: make([][]*analysis.Fact, len(snaps)), symbols: symbols, files: map[psi.SnapshotID]int{}}
		for i, s := range snaps {
			out.files[s.ID()] = i
		}
		for _, f := range facts {
			if i, ok := out.files[f.Element.Snapshot]; ok {
				out.facts[i] = append(out.facts[i], f)
			}
		}
		lap("split facts by file")
		var bytes, nfacts int
		for i := range snaps {
			b := encodeFile(out, out.facts[i])
			bytes += len(b)
			nfacts += len(out.facts[i])
		}
		lap("encode all files, serially")
		parallel(len(snaps), cfg.Jobs, func(i int) { encodeFile(out, out.facts[i]) })
		lap("encode all files, on 4 goroutines")
		if round == 0 {
			t.Logf("%d files, %d facts, %d symbols, %d bytes of facts", len(snaps), nfacts, len(symbols), bytes)
		}
	}
}
