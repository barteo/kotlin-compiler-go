package lower

import (
	"fmt"
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

// TestSignatures checks that Signatures lowers block bodies empty but for
// contracts and inline functions', and the rest as File does.
func TestSignatures(t *testing.T) {
	src := `package p

import kotlin.contracts.*

class A(val n: Int) {
    init { println(n) }
    constructor() : this(1) { println("c") }
    val p: Int get() { return n }
    fun f(): Int { val x = n; return x }
    fun g() = n + 1
}

fun c(x: Any?): Boolean {
    contract { returns(true) implies (x is String) }
    return x is String
}

inline fun i(f: () -> Unit) { f(); f() }
`
	s := psi.Parse("a.kt", src)
	whole, _, errs := File(s)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	sig, prov, errs := Signatures(s)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	stmts := map[string]int{}
	for _, d := range sig.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			stmts[d.Name] = len(d.Body.Stmts)
		case *syntax.ClassDecl:
			for _, m := range d.Members {
				switch m := m.(type) {
				case *syntax.FunDecl:
					if m.Body != nil {
						stmts[m.Name] = len(m.Body.Stmts)
					} else {
						stmts[m.Name] = -1 // an expression body
					}
				case *syntax.InitBlock:
					stmts["init"] = len(m.Body.Stmts)
				case *syntax.SecondaryCtor:
					stmts["constructor"] = len(m.Body.Stmts)
					if m.Delegation == nil {
						t.Error("constructor: delegation dropped")
					}
				case *syntax.PropDecl:
					stmts["get"] = len(m.Getter.Body.Stmts)
				}
			}
		}
	}
	want := map[string]int{"init": 0, "constructor": 0, "get": 0, "f": 0, "g": -1, "c": 1, "i": 2}
	if fmt.Sprint(stmts) != fmt.Sprint(want) {
		t.Errorf("statements %v, want %v", stmts, want)
	}
	if len(sig.Decls) != len(whole.Decls) {
		t.Errorf("%d declarations, want %d", len(sig.Decls), len(whole.Decls))
	}
	for _, d := range sig.Decls {
		if prov.Node(d) == 0 {
			t.Errorf("%T: no psi node", d)
		}
	}
}
