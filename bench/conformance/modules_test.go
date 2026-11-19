package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func readTestString(t *testing.T, src string) *test {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.kt")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	test, err := readTest(path, "t.kt")
	if err != nil {
		t.Fatal(err)
	}
	return test
}

// A module's class path has the outputs of the modules it names as
// dependencies or friends, not transitively, and its friends are also
// friend paths.
func TestModuleDeps(t *testing.T) {
	test := readTestString(t, `// MODULE: a
// FILE: a.kt
class A
// MODULE: b(a)
// FILE: b.kt
class B
// MODULE: c(b)(a)
// FILE: c.kt
class C
// MODULE: d
// FILE: d.kt
class D
`)
	if len(test.modules) != 4 {
		t.Fatalf("%d modules, want 4", len(test.modules))
	}
	outs := []string{"oa", "ob", "oc", "od"}
	tests := []struct {
		cp, friends []string
	}{
		{nil, nil},
		{[]string{"oa"}, nil},
		{[]string{"ob", "oa"}, []string{"oa"}},
		{nil, nil},
	}
	for i, tt := range tests {
		m := test.modules[i]
		cp, friends := moduleDeps(test.modules[:i], outs, m)
		if !slices.Equal(cp, tt.cp) || !slices.Equal(friends, tt.friends) {
			t.Errorf("module %s: class path %v, friends %v; want %v, %v", m.name, cp, friends, tt.cp, tt.friends)
		}
	}
	if s := commonSkip(test); s != "" {
		t.Errorf("a test with friends skips for %q", s)
	}
}

// JVM_DEFAULT_MODE applies to its module, or, before the first module,
// to all of them.
func TestJVMDefaultMode(t *testing.T) {
	test := readTestString(t, `// JVM_DEFAULT_MODE: no-compatibility
// MODULE: lib
// JVM_DEFAULT_MODE: disable
// FILE: 1.kt
interface Foo
// MODULE: main(lib)
// FILE: main.kt
fun box() = "OK"
`)
	if got := jvmDefaultMode(test, test.modules[0]); got != "disable" {
		t.Errorf("lib: %q, want disable", got)
	}
	if got := jvmDefaultMode(test, test.modules[1]); got != "no-compatibility" {
		t.Errorf("main: %q, want no-compatibility", got)
	}
	if s := commonSkip(test); s != "" {
		t.Errorf("a test with JVM_DEFAULT_MODE skips for %q", s)
	}
}

// JVM_TARGET applies to its module, as wrongInlineTarget's library is
// compiled for JVM 11 and its main module for 1.8.
func TestJVMTarget(t *testing.T) {
	test := readTestString(t, `// MODULE: library
// JVM_TARGET: 11
// FILE: a.kt
inline fun f() {}
// MODULE: main(library)
// JVM_TARGET: 1.8
// FILE: source.kt
fun g() = f()
`)
	if got := moduleDirective(test, test.modules[0], "JVM_TARGET"); got != "11" {
		t.Errorf("library: %q, want 11", got)
	}
	if got := moduleDirective(test, test.modules[1], "JVM_TARGET"); got != "1.8" {
		t.Errorf("main: %q, want 1.8", got)
	}
}
