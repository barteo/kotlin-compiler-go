package lower

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type benchFile struct{ path, src string }

// benchCorpus reads every .kt file under $KC_BENCH_CORPUS, or under
// testdata when it is unset, that both frontends accept, so each benchmark
// parses the same valid inputs.
func benchCorpus(b *testing.B) ([]benchFile, int64) {
	b.Helper()
	root := os.Getenv("KC_BENCH_CORPUS")
	if root == "" {
		root = "../../testdata"
	}
	var files []benchFile
	var size int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		if _, errs := syntax.Parse(path, src); len(errs) > 0 {
			return nil
		}
		if _, _, errs := File(psi.Parse(path, src)); len(errs) > 0 {
			return nil
		}
		files = append(files, benchFile{path, src})
		size += int64(len(src))
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
	if len(files) == 0 {
		b.Fatalf("no .kt files both frontends accept under %s", root)
	}
	return files, size
}

// BenchmarkFrontends compares the two paths the driver's parser can take
// (internal/driver/driver.go): kc's own parser, and psi.Parse followed by
// lowering. Each iteration parses the whole corpus.
func BenchmarkFrontends(b *testing.B) {
	files, size := benchCorpus(b)
	b.Logf("%d files, %d bytes", len(files), size)
	run := func(name string, parse func(benchFile)) {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(size)
			b.ReportAllocs()
			for b.Loop() {
				for _, f := range files {
					parse(f)
				}
			}
		})
	}
	run("syntax", func(f benchFile) { syntax.Parse(f.path, f.src) })
	run("psi", func(f benchFile) { psi.Parse(f.path, f.src) })
	// A reused psi.Parser keeps its construction buffers between files, as
	// a single-threaded caller such as a linter worker can.
	var parser psi.Parser
	run("psi-reused", func(f benchFile) { parser.Parse(f.path, f.src) })
	run("psi+lower", func(f benchFile) { File(psi.Parse(f.path, f.src)) })
}
