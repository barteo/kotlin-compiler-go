package lower

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestDifferential checks that lowering the psi tree of every test file
// kc parses gives the AST kc's parser gives.
func TestDifferential(t *testing.T) {
	var files []string
	err := filepath.WalkDir("../../testdata", func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".kt") {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test files")
	}
	for _, path := range files {
		t.Run(strings.TrimPrefix(path, "../../testdata/"), func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			src := string(b)
			want, errs := syntax.ParseAll(path, src)
			if len(errs) > 0 {
				t.Skipf("kc rejects the file: %v", errs[0])
			}
			snap := psi.Parse(path, src)
			if ds := snap.Diagnostics(); len(ds) > 0 {
				t.Skipf("psi diagnostics: %v", ds[0])
			}
			got, _, lerrs := File(snap)
			if len(lerrs) > 0 {
				t.Fatalf("lowering failed: %v", lerrs[0])
			}
			if d := syntax.Diff(want, got, !isASCII(src)); d != "" {
				t.Errorf("ASTs differ:\n%s", d)
			}
		})
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
