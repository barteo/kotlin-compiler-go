package main

import (
	"os"
	"reflect"
	"testing"
)

func TestStripMarkup(t *testing.T) {
	tests := []struct {
		src, want string
		es        []expected
	}{
		{"val x = 1\n", "val x = 1\n", nil},
		{"<!NO_TAIL_CALLS_FOUND!>tailrec<!> fun f()", "tailrec fun f()", []expected{{"NO_TAIL_CALLS_FOUND", 1}}},
		{"a\nreturn <!TYPE_MISMATCH(\"String; Nothing\")!>\"OK\"<!>", "a\nreturn \"OK\"", []expected{{"TYPE_MISMATCH", 2}}},
		{"<!A, B(\"x)!>\")!>y<!>", "y", []expected{{"A", 1}, {"B", 1}}},
		{"<!A!>f(<!B!>x<!>)<!>", "f(x)", []expected{{"A", 1}, {"B", 1}}},
		{"<!A!>multi\nline<!> <!C{LT}!>z<!>", "multi\nline z", []expected{{"A", 1}, {"C", 2}}},
		{"if (a <!b) {}", "if (a <!b) {}", nil},
		{"x <!= y", "x <!= y", nil},
		{"<!DEBUG_INFO_EXPRESSION_TYPE(\"kotlin.Function1<kotlin.Int, kotlin.Unit>\")!>f<!>", "f",
			[]expected{{"DEBUG_INFO_EXPRESSION_TYPE", 1}}},
	}
	for _, tt := range tests {
		got, es := stripMarkup(tt.src)
		if got != tt.want {
			t.Errorf("stripMarkup(%q) = %q, want %q", tt.src, got, tt.want)
		}
		if !reflect.DeepEqual(es, tt.es) {
			t.Errorf("stripMarkup(%q) diagnostics = %v, want %v", tt.src, es, tt.es)
		}
	}
}

func TestReadTest(t *testing.T) {
	src := `// WITH_STDLIB
// MODULE: lib
// FILE: a.kt
package a
fun f() = 1

// MODULE: main(lib)
// FILE: b.kt
fun box() = "OK"
`
	path := t.TempDir() + "/t.kt"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tt, err := readTest(path, "t.kt")
	if err != nil {
		t.Fatal(err)
	}
	if !tt.has("WITH_STDLIB") || len(tt.modules) != 2 {
		t.Fatalf("directives %v, %d modules", tt.directives, len(tt.modules))
	}
	lib, main := tt.modules[0], tt.modules[1]
	if lib.name != "lib" || len(lib.files) != 1 || lib.files[0].name != "a.kt" || lib.files[0].line != 4 {
		t.Errorf("lib = %+v, files %+v", lib, lib.files[0])
	}
	if main.name != "main" || !reflect.DeepEqual(main.deps, []string{"lib"}) || main.files[0].name != "b.kt" {
		t.Errorf("main = %+v", main)
	}
	if got := facadeClass(lib.files[0]); got != "a.AKt" {
		t.Errorf("facade of a.kt = %s", got)
	}
	if got := facadeClass(&srcFile{name: "1.kt"}); got != "_1Kt" {
		t.Errorf("facade of 1.kt = %s", got)
	}
}

func TestNonNullAnyCalls(t *testing.T) {
	src := "fun f(s: String?) {\n    s<!UNSAFE_CALL!>.<!>hashCode()\n    s?.hashCode()\n    s<!UNSAFE_CALL, X!>.<!>toString()\n}"
	want := "fun f(s: String?) {\n    s<!UNSAFE_CALL!>.<!>kcNonNull_hashCode()\n    s?.hashCode()\n    s<!UNSAFE_CALL, X!>.<!>kcNonNull_toString()\n}" +
		"\nprivate fun Any.kcNonNull_hashCode(): Int = hashCode()\n" +
		"\nprivate fun Any.kcNonNull_toString(): String = toString()\n"
	if got := nonNullAnyCalls(src); got != want {
		t.Errorf("nonNullAnyCalls(%q) = %q, want %q", src, got, want)
	}
	src = "fun f() {\n    <!SMARTCAST_IMPOSSIBLE!>i<!>.hashCode()\n}"
	want = "fun f() {\n    <!SMARTCAST_IMPOSSIBLE!>i<!>.kcNonNull_hashCode()\n}" +
		"\nprivate fun Any.kcNonNull_hashCode(): Int = hashCode()\n"
	if got := nonNullAnyCalls(src); got != want {
		t.Errorf("nonNullAnyCalls(%q) = %q, want %q", src, got, want)
	}
	if src := "fun f() = super.hashCode()"; nonNullAnyCalls(src) != src {
		t.Errorf("nonNullAnyCalls(%q) changed an unmarked call", src)
	}
}
