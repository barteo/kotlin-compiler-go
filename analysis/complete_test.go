package analysis

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// complete analyzes src, with Placeholder where | is, and returns the
// completion's items' names, with a mark for the importable ones.
func complete(t *testing.T, lib *Library, src string, match func(string) bool, importable bool) (*Completion, []string) {
	t.Helper()
	src = strings.Replace(src, "|", Placeholder, 1)
	s := psi.Parse("c.kt", src)
	r, err := lib.Analyze(context.Background(), []*psi.Snapshot{s}, Options{Complete: &Complete{Path: "c.kt", Match: match, Importable: importable}})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range r.Diagnostics() {
		if strings.Contains(d.Message, Placeholder) {
			t.Errorf("placeholder's diagnostic: %+v", d)
		}
	}
	c := r.Completion()
	if c == nil {
		t.Fatal("no completion")
	}
	var names []string
	for _, it := range c.Items {
		n := fmt.Sprintf("%s:%s:%d", it.Kind, it.Name, it.Level)
		if it.Import != "" {
			n += " import " + it.Import
		}
		names = append(names, n)
	}
	return c, names
}

func TestCompleteScope(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	src := "package p\n\nclass Box(val size: Int) {\n    fun grow(by: Int) = Box(size + by)\n    fun use(limit: Int) {\n        val local = 1\n        |\n        val later = 2\n    }\n}\n\nfun topLevel() = 1\n"
	c, names := complete(t, lib, src, nil, false)
	if !c.Found {
		t.Fatal("placeholder not reached")
	}
	for _, want := range []string{"local:local:0", "parameter:limit:0", "property:size:1", "function:grow:1", "function:topLevel:2", "class:Box:2", "function:println:2", "class:String:2"} {
		if !slices.Contains(names, want) {
			t.Errorf("no %s among %d items", want, len(names))
		}
	}
	if slices.Contains(names, "local:later:0") {
		t.Error("a local declared after the caret")
	}
}

func TestCompleteMembers(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	src := "fun f(xs: List<String>, s: String?) {\n    xs.|\n}\n"
	c, names := complete(t, lib, src, nil, false)
	if c.Receiver == nil || c.Receiver.Name != "kotlin.collections.List" {
		t.Fatalf("receiver %+v", c.Receiver)
	}
	for _, want := range []string{"property:size:1", "function:get:1", "function:map:1", "function:first:1", "function:isEmpty:1"} {
		if !slices.Contains(names, want) {
			t.Errorf("no %s among %v", want, len(names))
		}
	}
	for _, it := range c.Items {
		if it.Name == "get" && (it.Type == nil || it.Type.Name != "kotlin.String") {
			t.Errorf("get's type %+v, want String", it.Type)
		}
	}
	_, names = complete(t, lib, "fun f(s: String?) {\n    s?.|\n}\n", func(n string) bool { return strings.HasPrefix(n, "len") || strings.HasPrefix(n, "upper") }, false)
	if !slices.Contains(names, "property:length:1") || !slices.Contains(names, "function:uppercase:1") {
		t.Errorf("string members: %v", names)
	}
}

func TestCompleteImportable(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	_, names := complete(t, lib, "fun f() {\n    measureTime|\n}\n", func(n string) bool { return strings.HasPrefix(n, "measureTime") }, true)
	found := slices.ContainsFunc(names, func(n string) bool { return strings.Contains(n, "import kotlin.time.measureTime") })
	if !found {
		t.Errorf("no importable measureTime: %v", names)
	}
}
