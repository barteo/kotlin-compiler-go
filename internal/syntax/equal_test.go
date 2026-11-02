package syntax

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEqualSelf(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/run/*.kt")
	if len(files) == 0 {
		t.Fatal("no files")
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		a, errs := ParseAll(f, string(src))
		b, _ := ParseAll(f, string(src))
		if a == nil || len(errs) > 0 {
			continue
		}
		if d := Diff(a, b, false); d != "" {
			t.Fatalf("%s: %s", f, d)
		}
	}
}

func TestDiffFinds(t *testing.T) {
	a, _ := Parse("a.kt", "fun f() { val x = 1 + 2 }")
	b, _ := Parse("a.kt", "fun f() { val x = 1 - 2 }")
	c, _ := Parse("a.kt", "fun f() {  val x = 1 + 2 }")
	if d := Diff(a, b, true); !strings.Contains(d, "Op") {
		t.Errorf("op: %q", d)
	}
	if Diff(a, c, true) != "" {
		t.Error("positions not ignored")
	}
	if d := Diff(a, c, false); !strings.Contains(d, "Col") {
		t.Errorf("pos: %q", d)
	}
	x, _ := Parse("a.kt", "fun f() { @A g() }")
	y, _ := Parse("a.kt", "fun f() { @B g() }")
	if d := Diff(x, y, true); !strings.Contains(d, "Annotated") {
		t.Errorf("annotated: %q", d)
	}
}
