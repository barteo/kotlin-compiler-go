package ka

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// BenchmarkUpdate times Update on the module that KA_BENCH describes:
// SRC_DIR[:SRC_DIR...] and a file listing the class path, one jar per
// line, separated by a comma (as bench/ksp/run.sh takes them). It is
// skipped without KA_BENCH.
func BenchmarkUpdate(b *testing.B) {
	s, sources := benchModule(b)
	defer s.Close()
	for b.Loop() {
		m, err := s.Update(context.Background(), sources)
		if err != nil {
			b.Fatal(err)
		}
		m.Close()
	}
}

// BenchmarkUpdateEdit times an Update after an edit of one file's body,
// KA_BENCH_EDIT's (the first file by default): a comment added at its end;
// with KA_BENCH_SIGNATURE set, a function, named after the run; or, with
// KA_BENCH_RETURN set, a function added once whose body returns the run's
// number, as bench/ksp's body edits do.
func BenchmarkUpdateEdit(b *testing.B) {
	s, sources := benchModule(b)
	defer s.Close()
	i := 0
	for j, src := range sources {
		if e := os.Getenv("KA_BENCH_EDIT"); e != "" && strings.HasSuffix(src.Path, e) {
			i = j
		}
	}
	data, err := os.ReadFile(sources[i].Path)
	if err != nil {
		b.Fatal(err)
	}
	n := 0
	for b.Loop() {
		n++
		text := string(data) + "\n// edit " + strconv.Itoa(n) + "\n"
		if os.Getenv("KA_BENCH_RETURN") != "" {
			text = string(data) + "\nfun kcEdit(): Int {\n    return " + strconv.Itoa(n) + "\n}\n"
		}
		if os.Getenv("KA_BENCH_SIGNATURE") != "" {
			text = string(data) + "\nfun kcEdit" + strconv.Itoa(n) + "(): Int { return 1 }\n"
		}
		sources[i].Text = &text
		m, err := s.Update(context.Background(), sources)
		if err != nil {
			b.Fatal(err)
		}
		m.Close()
	}
}

func benchModule(b testing.TB) (*Session, []Source) {
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
	return Open(cfg), sources
}
