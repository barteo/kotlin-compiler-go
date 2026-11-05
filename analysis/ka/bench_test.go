package ka

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// BenchmarkUpdate times Update on the module that KA_BENCH describes:
// SRC_DIR[:SRC_DIR...] and a file listing the class path, one jar per
// line, separated by a comma (as bench/ksp/run.sh takes them). It is
// skipped without KA_BENCH.
func BenchmarkUpdate(b *testing.B) {
	spec := os.Getenv("KA_BENCH")
	if spec == "" {
		b.Skip("KA_BENCH is not set")
	}
	dirs, libs, _ := strings.Cut(spec, ",")
	var sources []Source
	for _, dir := range filepath.SplitList(dirs) {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".kt") {
				sources = append(sources, Source{Path: path})
			}
			return err
		})
	}
	data, err := os.ReadFile(libs)
	if err != nil {
		b.Fatal(err)
	}
	cfg := Config{ModuleName: "main", Jobs: runtime.NumCPU(), ClassPath: strings.Fields(string(data))}
	s := Open(cfg)
	defer s.Close()
	for b.Loop() {
		m, err := s.Update(context.Background(), sources)
		if err != nil {
			b.Fatal(err)
		}
		m.Close()
	}
}
