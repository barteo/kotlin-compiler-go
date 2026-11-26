package analysis

import (
	"context"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func openLib(t *testing.T, cfg Config) *Library {
	t.Helper()
	lib, err := OpenLibrary(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib
}

// TestLibraryBrokenFile checks that a file with a syntax error has facts
// for the code around the error, and no checker errors on its line.
func TestLibraryBrokenFile(t *testing.T) {
	lib := openLib(t, Config{NoJDK: true})
	src := "class Box(val n: Int)\n" +
		"fun f(b: Box): Box {\n" +
		"    val y = b.n +\n" + // syntax error
		"    val z: Box = b\n" +
		"    return z\n" +
		"}\n"
	s := psi.Parse("a.kt", src)
	if len(s.Diagnostics()) == 0 {
		t.Fatal("no syntax error in the test source")
	}
	r, err := lib.Analyze(context.Background(), []*psi.Snapshot{s}, Options{Warnings: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range r.Diagnostics() {
		if d.Line == 3 && d.Severity == "error" {
			t.Errorf("checker error on the syntax error's line: %+v", d)
		}
	}
	z := fact(t, r, at(t, s, "REFERENCE_EXPRESSION", "z", 0))
	if z.Type == nil || z.Type.Name != "Box" {
		t.Errorf("z: type %+v, want Box", z.Type)
	}
	if sym := symbol(t, r, z.Symbol); sym.Declaration == nil {
		t.Errorf("z: no declaration: %+v", sym)
	}
}

func TestLibraryErrorsAndWarnings(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	src := "fun g() {\n" +
		"    val x = 1 as Int\n" +
		"    undefinedCall(x)\n" +
		"}\n"
	s := psi.Parse("b.kt", src)
	r, err := lib.Analyze(context.Background(), []*psi.Snapshot{s}, Options{Warnings: true})
	if err != nil {
		t.Fatal(err)
	}
	var warn, unresolved bool
	for _, d := range r.Diagnostics() {
		switch {
		case d.Severity == "warning" && d.Name == "INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL" && d.Line == 2:
			warn = true
		case d.Severity == "error" && d.Line == 3 && strings.Contains(d.Message, "undefinedCall"):
			unresolved = true
		}
	}
	if !warn || !unresolved {
		t.Errorf("diagnostics %+v: want INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL on line 2 and an unresolved reference on line 3", r.Diagnostics())
	}
	// Errors don't stop the facts of what the checker resolved.
	if f := fact(t, r, at(t, s, "INTEGER_CONSTANT", "1", 0)); f.Type == nil || f.Type.Name != "kotlin.Int" {
		t.Errorf("1: type %+v", f.Type)
	}
	// Without Warnings, only errors.
	r, err = lib.Analyze(context.Background(), []*psi.Snapshot{psi.Parse("b.kt", src)}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range r.Diagnostics() {
		if d.Severity != "error" {
			t.Errorf("warning without Options.Warnings: %+v", d)
		}
	}
}

// TestLibraryReuse analyzes edits of a module against one library, as a
// language server does.
func TestLibraryReuse(t *testing.T) {
	lib := openLib(t, withStdlib(t))
	for i, src := range []string{
		"fun a() = listOf(1).first()\n",
		"fun a() = listOf(\"s\").first()\n",
		"fun a() = listOf(1.0).first()\n",
	} {
		s := psi.Parse("c.kt", src)
		r, err := lib.Analyze(context.Background(), []*psi.Snapshot{s}, Options{Warnings: true})
		if err != nil {
			t.Fatal(err)
		}
		if d := r.Diagnostics(); len(d) > 0 {
			t.Fatalf("edit %d: %+v", i, d)
		}
		want := []string{"kotlin.Int", "kotlin.String", "kotlin.Double"}[i]
		f := fact(t, r, at(t, s, "CALL_EXPRESSION", "first()", 0))
		if f.Type == nil || f.Type.Name != want {
			t.Errorf("edit %d: first(): %+v, want %s", i, f.Type, want)
		}
	}
}

func TestLibraryJavaSources(t *testing.T) {
	lib := openLib(t, withJDK(t))
	s := psi.Parse("d.kt", "fun use(j: J): Int = j.size()\n")
	r, err := lib.Analyze(context.Background(), []*psi.Snapshot{s}, Options{
		Java: []JavaSource{{Path: "J.java", Text: "public class J { public int size() { return 0; } }\n"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Diagnostics(); len(d) > 0 {
		t.Fatalf("diagnostics: %+v", d)
	}
	f := fact(t, r, at(t, s, "CALL_EXPRESSION", "size()", 0))
	if f.Call == nil || f.Call.Name != "J.size" || !f.Call.FromJava {
		t.Errorf("size(): call %+v", f.Call)
	}
}

// TestLibraryDuplicateFacades checks that a file with another's facade
// class is left out with a diagnostic, and the rest keep their facts.
func TestLibraryDuplicateFacades(t *testing.T) {
	lib := openLib(t, Config{NoJDK: true})
	a := psi.Parse("x/A.kt", "fun a(): Int = 1\n")
	b := psi.Parse("y/A.kt", "fun b(): Int = 2\n")
	c := psi.Parse("C.kt", "fun c(): Int = a()\n")
	r, err := lib.Analyze(context.Background(), []*psi.Snapshot{a, b, c}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Diagnostics(); len(d) != 1 || d[0].Path != "y/A.kt" {
		t.Fatalf("diagnostics: %+v", d)
	}
	if f := fact(t, r, at(t, c, "CALL_EXPRESSION", "a()", 0)); f.Call == nil || f.Call.Declaration == nil {
		t.Errorf("a(): %+v", f.Call)
	}
}
