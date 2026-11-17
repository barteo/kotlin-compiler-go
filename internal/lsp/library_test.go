package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
)

// TestExtractLocalOnly checks that a library entry is extracted under
// the cache directory, and that one naming a path outside it is refused.
func TestExtractLocalOnly(t *testing.T) {
	dir := t.TempDir()
	s := &Server{}
	s.lib.dir = filepath.Join(dir, "cache")
	file, err := s.extract(libsrc.Location{Archive: "/m2/lib-sources.jar", Entry: "a/B.kt", Text: "package a\n"})
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(s.lib.dir, file); err != nil || !filepath.IsLocal(rel) {
		t.Fatalf("extracted to %s, outside %s", file, s.lib.dir)
	}
	for _, entry := range []string{"../../Evil.kt", "/tmp/Evil.kt", `..\..\Evil.kt`} {
		if file, err := s.extract(libsrc.Location{Archive: "/m2/lib-sources.jar", Entry: entry, Text: "x"}); err == nil {
			t.Errorf("%q extracted to %s", entry, file)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "Evil.kt")); err == nil {
		t.Error("Evil.kt written outside the cache")
	}
}
