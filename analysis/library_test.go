package analysis

import (
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestLibraryStub renders stubs of the standard library's and the JDK's
// declarations, which parse without errors and declare what was asked.
func TestLibraryStub(t *testing.T) {
	cfg := withJDK(t)
	s := Open(cfg)
	defer s.Close()
	for _, c := range []struct{ name, path, want string }{
		{"kotlin.Pair", "kotlin/Pair.kt", "class Pair<out A, out B>"},
		{"kotlin.Pair.first", "kotlin/Pair.kt", "val first: A"},
		{"kotlin.collections.listOf", "kotlin/collections/listOf.kt", "fun <T> listOf(vararg elements: T): List<T>"},
		{"kotlin.text.trim", "kotlin/text/trim.kt", "fun String.trim(): String"},
		{"kotlin.io.println", "kotlin/io/println.kt", "fun println(message: Any?)"},
		{"java.lang.StringBuilder", "java/lang/StringBuilder.kt", "fun append("},
		{"kotlin.collections.ArrayDeque", "kotlin/collections/ArrayDeque.kt", "fun addFirst(element: E)"},
	} {
		path, text, ok, err := s.LibraryStub(c.name)
		if err != nil || !ok {
			t.Errorf("%s: ok %v, %v", c.name, ok, err)
			continue
		}
		if path != c.path {
			t.Errorf("%s: path %s, want %s", c.name, path, c.path)
		}
		if !strings.Contains(text, c.want) {
			t.Errorf("%s: no %q in\n%s", c.name, c.want, text)
		}
		if ds := psi.Parse(path, text).Diagnostics(); len(ds) > 0 {
			t.Errorf("%s: %v in\n%s", c.name, ds, text)
		}
	}
	if _, _, ok, _ := s.LibraryStub("no.such.Thing"); ok {
		t.Error("no.such.Thing: a stub")
	}
}
