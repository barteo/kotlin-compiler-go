package kaops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/analysis/ka"
)

// The stages of a linter's analysis, timed on real sources:
//
//	KC_LINT_SRC=dir1:dir2 KC_LINT_CP=a.jar:b.jar go test ./internal/kaops -run TestLintStages -v
//
// It prints the time of each stage that OpKaOpen with facts, OpKaUpdate
// and OpKaFacts run, as detekt on kc calls them: the update, which parses,
// checks and builds the facts, the split of the facts by file, and
// encoding every file's facts, one by one and on all cores.
// KC_LINT_OPTINS adds -opt-ins. It is skipped without KC_LINT_SRC.
func TestLintStages(t *testing.T) {
	src := os.Getenv("KC_LINT_SRC")
	if src == "" {
		t.Skip("KC_LINT_SRC is not set")
	}
	var sources []ka.Source
	for _, dir := range filepath.SplitList(src) {
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".kt") {
				sources = append(sources, ka.Source{Path: p})
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
		s := ka.OpenFacts(cfg)
		m, err := s.Update(context.Background(), sources)
		if err != nil {
			t.Fatal(err)
		}
		lap("update: parse, check, facts")
		if len(m.Diagnostics) > 0 {
			t.Fatalf("%d diagnostics, first: %v", len(m.Diagnostics), m.Diagnostics[0])
		}
		out := factsOf(m)
		lap("split facts by file")
		var bytes, nfacts int
		for i := range out.facts {
			b := encodeFile(out, out.facts[i])
			bytes += len(b)
			nfacts += len(out.facts[i])
		}
		lap("encode all files, serially")
		var wg sync.WaitGroup
		next := make(chan int)
		for range cfg.Jobs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range next {
					encodeFile(out, out.facts[i])
				}
			}()
		}
		for i := range out.facts {
			next <- i
		}
		close(next)
		wg.Wait()
		lap("encode all files, on 4 goroutines")
		if round == 0 {
			t.Logf("%d files, %d facts, %d symbols, %d bytes of facts", len(out.facts), nfacts, len(out.symbols), bytes)
		}
		dropFacts(m)
		m.Close()
		s.Close()
	}
}
