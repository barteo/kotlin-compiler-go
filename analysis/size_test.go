package analysis

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// genModule returns the snapshots of bench/gen's module of n files, or
// skips b.
func genModule(b *testing.B, n int) []*psi.Snapshot {
	b.Helper()
	dir := b.TempDir()
	cmd := exec.Command("go", "run", "../bench/gen", "-n", strconv.Itoa(n), "-o", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Skipf("bench/gen: %v\n%s", err, out)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		b.Fatal(err)
	}
	var snaps []*psi.Snapshot
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".kt") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			b.Fatal(err)
		}
		snaps = append(snaps, psi.Parse(e.Name(), string(src)))
	}
	return snaps
}

func heapAlloc() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// BenchmarkResultSize reports how much heap a Result of bench/gen's
// module of 200 files keeps, as the heap's growth after GC.
func BenchmarkResultSize(b *testing.B) {
	snaps := genModule(b, 200)
	cfg := withStdlib(b)
	cfg.Jobs = runtime.GOMAXPROCS(0)
	lib, err := OpenLibrary(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer lib.Close()
	// Once to fill the library's caches, which aren't the Result's.
	if _, err := lib.Analyze(context.Background(), snaps, Options{}); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	var size, facts float64
	for b.Loop() {
		before := heapAlloc()
		r, err := lib.Analyze(context.Background(), snaps, Options{})
		if err != nil {
			b.Fatal(err)
		}
		size = float64(heapAlloc()) - float64(before)
		facts = float64(len(r.Data().Facts))
		runtime.KeepAlive(r)
	}
	b.ReportMetric(size/1e6, "MB/result")
	b.ReportMetric(size/facts, "B/fact")
	b.ReportMetric(facts, "facts")
}

// BenchmarkFact reports the cost of Result.Fact, over each fact of
// bench/gen's module of 20 files.
func BenchmarkFact(b *testing.B) {
	snaps := genModule(b, 20)
	cfg := withStdlib(b)
	r, err := Analyze(context.Background(), cfg, snaps)
	if err != nil {
		b.Fatal(err)
	}
	var ids []ElementID
	for _, f := range r.Data().Facts {
		ids = append(ids, f.Element)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		for _, id := range ids {
			if _, ok := r.Fact(id); !ok {
				b.Fatal("no fact")
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(ids)), "ns/fact")
}

// BenchmarkFacts reports the cost of Result.Facts, over each fact of
// bench/gen's module of 20 files, against BenchmarkFact's.
func BenchmarkFacts(b *testing.B) {
	snaps := genModule(b, 20)
	r, err := Analyze(context.Background(), withStdlib(b), snaps)
	if err != nil {
		b.Fatal(err)
	}
	n := 0
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		n = 0
		for _, s := range snaps {
			r.Facts(s.ID(), func(f *Fact) bool {
				n++
				return true
			})
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/fact")
}
