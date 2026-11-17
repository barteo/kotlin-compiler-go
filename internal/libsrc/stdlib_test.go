package libsrc

import (
	"strings"
	"testing"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// TestStdlibSources locates declarations of the standard library in its
// sources jar, next to kotlin-stdlib.jar in Kotlin's distribution.
func TestStdlibSources(t *testing.T) {
	stdlib, err := classpath.FindStdlib()
	if err != nil {
		t.Skipf("no Kotlin standard library: %v", err)
	}
	if SourcesOf(stdlib) == "" {
		t.Skipf("no sources jar next to %s", stdlib)
	}
	f := New([]string{stdlib}, "-")
	cases := []struct {
		t      Target
		suffix string // of the entry
		line   string // a substring of the name's line
	}{
		{Target{Kind: "function", Name: "kotlin.collections.listOf", Params: true, Parameters: []string{"T"}, Varargs: []bool{false}}, "collections/CollectionsJVM.kt", "actual fun <T> listOf(element: T)"},
		{Target{Kind: "function", Name: "kotlin.collections.listOf", Params: true, Parameters: []string{"T"}, Varargs: []bool{true}}, "collections/Collections.kt", "fun <T> listOf(vararg elements: T)"},
		{Target{Kind: "function", Name: "kotlin.text.trim", Params: true, Receiver: "String"}, "text/Strings.kt", "String.trim(): String"},
		{Target{Kind: "function", Name: "kotlin.io.println", Params: true, Parameters: []string{"Int"}}, "io/Console.kt", "fun println(message: Int)"},
		{Target{Kind: "constructor", Name: "kotlin.Pair"}, "Tuples.kt", "data class Pair<"},
		{Target{Kind: "function", Name: "kotlin.lazy", Params: true, Parameters: []string{"Function0"}}, "Lazy", "fun <T> lazy(initializer: () -> T)"},
		{Target{Kind: "function", Name: "kotlin.collections.MutableList.add", Params: true, Parameters: []string{"E"}}, "Collections.kt", "fun add(element: E): Boolean"},
		{Target{Kind: "class", Name: "kotlin.collections.List"}, "Collections.kt", "interface List<"},
		{Target{Kind: "property", Name: "kotlin.String.length"}, "String.kt", "length"},
		{Target{Kind: "function", Name: "java.util.mutableListOf", Params: true}, "collections/Collections.kt", "fun <T> mutableListOf(): MutableList<T>"},
	}
	start := time.Now()
	for _, c := range cases {
		loc, ok := f.Locate(c.t)
		if !ok {
			t.Errorf("%s: not found", c.t.Name)
			continue
		}
		ls := strings.LastIndexByte(loc.Text[:loc.Range.Start], '\n') + 1
		le := ls + strings.IndexByte(loc.Text[ls:], '\n')
		if !strings.HasSuffix(loc.Entry, c.suffix) && !strings.Contains(loc.Entry, c.suffix) || !strings.Contains(loc.Text[ls:le], c.line) {
			t.Errorf("%s: %s %q, want %s %q", c.t.Name, loc.Entry, loc.Text[ls:le], c.suffix, c.line)
		}
	}
	t.Logf("%d located in %v", len(cases), time.Since(start))
}
