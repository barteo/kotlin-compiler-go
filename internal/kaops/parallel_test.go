package kaops

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
)

// TestParallelModel checks that an update's model and facts are the same,
// byte for byte as OpKaUpdate and OpKaFacts encode them, whether Update
// builds the files' models on several goroutines and the facts alongside
// them, or one after another. It checks a few small files, and with
// KC_LINT_SRC (and KC_LINT_CP, KC_LINT_OPTINS) the sources TestLintStages
// times.
func TestParallelModel(t *testing.T) {
	var sources []ka.Source
	var cfg analysis.Config
	if src := os.Getenv("KC_LINT_SRC"); src != "" {
		for _, dir := range filepath.SplitList(src) {
			filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() && strings.HasSuffix(p, ".kt") {
					sources = append(sources, ka.Source{Path: p})
				}
				return nil
			})
		}
		cfg = analysis.Config{ModuleName: "bench", Jobs: 4, ClassPath: filepath.SplitList(os.Getenv("KC_LINT_CP"))}
		cfg.OptIns = strings.Fields(os.Getenv("KC_LINT_OPTINS"))
	} else {
		jar, err := classpath.FindStdlib()
		if err != nil {
			t.Skipf("no kotlin-stdlib.jar (set KOTLIN_HOME): %v", err)
		}
		cfg = analysis.Config{ModuleName: "m", Jobs: 4, ClassPath: []string{jar}}
		dir := t.TempDir()
		for i := range 12 {
			p := filepath.Join(dir, fmt.Sprintf("f%d.kt", i))
			src := fmt.Sprintf(`package p%d

annotation class A%d(val n: Int)

@A%d(%d)
open class C%d<T : Comparable<T>>(val x: T) {
    fun max(y: T): T = if (x > y) x else y
    val list: List<T> get() = listOf(x)
}

fun <T : Comparable<T>> f%d(c: C%d<T>, y: T): Int = c.list.map { it.compareTo(y) }.sum() + c.max(y).hashCode()
`, i, i, i, i, i, i, i)
			if err := os.WriteFile(p, []byte(src), 0o666); err != nil {
				t.Fatal(err)
			}
			sources = append(sources, ka.Source{Path: p})
		}
	}
	encode := func(serial bool) []byte {
		defer func(old bool) { ka.SerialModel = old }(ka.SerialModel)
		ka.SerialModel = serial
		s := ka.OpenFacts(cfg)
		defer s.Close()
		m, err := s.Update(context.Background(), sources)
		if err != nil {
			t.Fatal(err)
		}
		defer m.Close()
		if len(m.Diagnostics) > 0 {
			t.Fatalf("%d diagnostics, first: %v", len(m.Diagnostics), m.Diagnostics[0])
		}
		var w wire.Writer
		writeFiles(&w, m)
		out := factsOf(m)
		var b bytes.Buffer
		b.Write(w.Message())
		for i := range out.facts {
			b.Write(encodeFile(out, out.facts[i]))
		}
		return b.Bytes()
	}
	serial := encode(true)
	for range 3 {
		if parallel := encode(false); !bytes.Equal(serial, parallel) {
			t.Fatalf("the model and facts differ: %d bytes serially, %d in parallel", len(serial), len(parallel))
		}
	}
}
