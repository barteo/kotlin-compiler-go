package lower

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// TestNamelessObject checks that an object declaration without its name
// is kept, named as FIR names it, with the error reported in the file.
func TestNamelessObject(t *testing.T) {
	src := "class Outer {\n    @Anno\n    object : Any {\n    }\n    fun f() {}\n}\n"
	f, _, errs := File(psi.Parse("A.kt", src))
	if len(errs) > 0 {
		t.Fatalf("lowering failed: %v", errs[0])
	}
	if len(f.Errors) != 1 || f.Errors[0].Msg != "Name expected" || f.Errors[0].Pos.String() != "3:11" {
		t.Errorf("got errors %v, want Name expected at 3:11", f.Errors)
	}
	var names []string
	for _, m := range f.Decls[0].(*syntax.ClassDecl).Members {
		switch m := m.(type) {
		case *syntax.ClassDecl:
			names = append(names, m.Name)
		case *syntax.FunDecl:
			names = append(names, m.Name)
		}
	}
	if got, want := strings.Join(names, " "), NoNameProvided+" f"; got != want {
		t.Errorf("got members %q, want %q", got, want)
	}
}

// TestUnclosedPropertyType checks that a property whose type's argument
// list isn't closed is kept with the type as far as it parsed, as FIR
// keeps it, with the error reported in the file.
func TestUnclosedPropertyType(t *testing.T) {
	src := "class Foo<P> {\n    val a: Bar<out P!!> = TODO()\n    val b: Int = 1\n}\n"
	f, _, errs := File(psi.Parse("A.kt", src))
	if len(errs) > 0 {
		t.Fatalf("lowering failed: %v", errs[0])
	}
	if len(f.Errors) != 1 || f.Errors[0].Msg != "Expecting a '>'" {
		t.Errorf("got errors %v, want Expecting a '>'", f.Errors)
	}
	var got []string
	for _, m := range f.Decls[0].(*syntax.ClassDecl).Members {
		if p, ok := m.(*syntax.PropDecl); ok {
			got = append(got, fmt.Sprintf("%s %s %v", p.Name, p.Type.Name, len(p.Type.Args)))
		}
	}
	if got, want := strings.Join(got, ", "), "a Bar 1, b Int 0"; got != want {
		t.Errorf("got properties %q, want %q", got, want)
	}
}

// TestOutOfRangeFloats checks that a floating-point literal out of range
// is infinity or zero, as kc's parser and kotlinc read it, not an error.
func TestOutOfRangeFloats(t *testing.T) {
	src := "val a = 1e400\nval b = 1e-400\nval c = 1e40f\nval d = 1_0.5F\n"
	want, errs := syntax.ParseAll("A.kt", src)
	if len(errs) > 0 {
		t.Fatalf("kc's parser rejects the file: %v", errs[0])
	}
	got, _, lerrs := File(psi.Parse("A.kt", src))
	if len(lerrs) > 0 {
		t.Fatalf("lowering failed: %v", lerrs[0])
	}
	if d := syntax.Diff(want, got, false); d != "" {
		t.Errorf("ASTs differ:\n%s", d)
	}
}

// TestRedundantPrefixes checks the string literals with a redundant
// interpolation prefix that recovery reads from the tree of a file with
// syntax errors: those of statements it leaves out too, but not those
// inside a syntax error or under an annotation that may suppress the
// warning.
func TestRedundantPrefixes(t *testing.T) {
	src := "fun f() {\n" +
		"    $\"${}\"\n" + // 2: an error in a template entry
		"    $\"a\"\n" + // 3
		"    $$\"${}\"\n" + // 4: text that looks like an entry
		"    $$\"a\"\n" + // 5
		"    $$\"$${}\"\n" + // 6: an entry, with an error
		"    $$\"\\$a\"\n" + // 7: an escaped $ looks like one too
		"    \"a\"\n" +
		"    @Suppress(\"REDUNDANT_INTERPOLATION_PREFIX\") $\"a\"\n" +
		"    @Suppress(\"UNUSED_VARIABLE\") $\"a\"\n" + // 10
		"}\n" +
		"@Suppress(\"warnings\") fun g() { $\"a\" }\n" +
		"val x = $\"a\" +\n" // 13: left out with its declaration
	f, _, errs := FileRecovering(psi.Parse("a.kt", src))
	if len(errs) == 0 {
		t.Fatal("no syntax errors")
	}
	var got []int
	for _, p := range f.RedundantPrefixes {
		got = append(got, p.Line)
	}
	if want := []int{2, 3, 5, 10, 13}; !slices.Equal(got, want) {
		t.Errorf("lines of redundant prefixes: got %v, want %v", got, want)
	}
	if f, _, errs := File(psi.Parse("a.kt", "val x = $\"a\"\n")); len(errs) > 0 || f.RedundantPrefixes != nil {
		t.Errorf("a file without syntax errors: errors %v, prefixes %v", errs, f.RedundantPrefixes)
	}
}

// TestSignatures checks which block bodies Signatures lowers empty: not
// an inline function's or accessor's, one that starts with a contract,
// or an expression body.
func TestSignatures(t *testing.T) {
	src := `class A {
    init { println(1) }
    constructor(x: Int) { println(x) }
    val p: Int get() { return 1 }
    val q: Int inline get() { return 2 }
    fun f(): Int { return 3 }
    inline fun g(): Int { return 4 }
    fun h() = 5
}
fun c(x: Any?): Boolean {
    contract { returns(true) implies (x != null) }
    return x != null
}
fun broken() { val y = (1 + ; }
`
	f, _, errs := Signatures(psi.Parse("A.kt", src))
	if len(errs) > 0 {
		t.Fatalf("lowering failed: %v", errs[0])
	}
	stmts := func(b *syntax.Block) int {
		if b == nil {
			return -1
		}
		return len(b.Stmts)
	}
	var got []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *syntax.ClassDecl:
			for _, m := range d.Members {
				switch m := m.(type) {
				case *syntax.InitBlock:
					got = append(got, fmt.Sprintf("init %d", stmts(m.Body)))
				case *syntax.SecondaryCtor:
					got = append(got, fmt.Sprintf("constructor %d", stmts(m.Body)))
				case *syntax.PropDecl:
					got = append(got, fmt.Sprintf("%s %d", m.Name, stmts(m.Getter.Body)))
				case *syntax.FunDecl:
					got = append(got, fmt.Sprintf("%s %d %v", m.Name, stmts(m.Body), m.ExprBody != nil))
				}
			}
		case *syntax.FunDecl:
			got = append(got, fmt.Sprintf("%s %d", d.Name, stmts(d.Body)))
		}
	}
	want := "init 0, constructor 0, p 0, q 1, f 0 false, g 1 false, h -1 true, c 2, broken 0"
	if got := strings.Join(got, ", "); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestLost checks what FileRecovering records of what it leaves out of a
// file: declarations, or the bodies of the outermost declarations that
// lost only statements nothing else sees, by their lines.
func TestLost(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		decls     bool
		bodies    string // the lines of each span, "from-to", space separated
	}{
		{"statement", "fun f() {\n    val x = 1 +\n}\n", false, "1-3"},
		{"member", "class C {\n    fun g() {\n        1 +\n    }\n}\n", false, "2-4"},
		{"local fun", "fun f() {\n    fun g() {\n        1 +\n    }\n}\n", false, "1-5"},
		{"init block", "class C {\n    val a = 1\n    init {\n        1 +\n    }\n}\n", false, "1-6"},
		{"accessor", "val p: Int\n    get() {\n        val x = 1 +\n    }\n", false, "1-4"},
		{"inline", "inline fun f() {\n    1 +\n}\n", true, ""},
		{"contract", "fun f() {\n    contract { }\n    1 +\n}\n", true, ""},
		{"expression body", "fun f() = run {\n    1 +\n}\n", true, ""},
		{"initializer", "val p by lazy {\n    1 +\n}\n", true, ""},
		{"declaration", "fun f( {\n}\nfun g() {}\n", true, ""},
		{"import", "import a.\nfun f() {}\n", true, ""},
		{"stray brace", "fun f() {}\n}\n", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _, errs := FileRecovering(psi.Parse("A.kt", tc.src))
			if len(errs) == 0 && f.Lost != nil {
				t.Errorf("lost %+v without an error", *f.Lost)
			}
			var lost syntax.Lost
			if f.Lost != nil {
				lost = *f.Lost
			}
			var bodies []string
			for _, s := range lost.Bodies {
				bodies = append(bodies, fmt.Sprintf("%d-%d", s.From.Line, s.To.Line))
			}
			if lost.Decls != tc.decls || strings.Join(bodies, " ") != tc.bodies {
				t.Errorf("got Decls %v, bodies %q (errors %v); want %v, %q", lost.Decls, strings.Join(bodies, " "), errs, tc.decls, tc.bodies)
			}
		})
	}
}
