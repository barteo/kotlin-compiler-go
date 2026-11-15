package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMissingDependencies compiles three modules, root, lib against
// root, and app against lib alone, and checks app's uses of lib's
// declarations that name root's classes: MISSING_DEPENDENCY_CLASS and
// MISSING_DEPENDENCY_SUPERCLASS, as kotlinc reports them.
func TestMissingDependencies(t *testing.T) {
	std := stdlib()
	if std == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	dir := t.TempDir()
	compile := func(name, src string, cp ...string) (string, error) {
		path := filepath.Join(dir, name+".kt")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, name)
		return out, Compile([]string{path}, Options{OutDir: out, Jobs: 1, ClassPath: append([]string{std}, cp...), ModuleName: name})
	}
	root, err := compile("root", "package r\n\ninterface Missed { fun foo() = 1 }\nclass Some\n")
	if err != nil {
		t.Fatal(err)
	}
	lib, err := compile("lib", `package l

import r.*

interface Mid : Missed
class Impl : Mid { fun own() = 1 }
open class Base {
    fun take(s: Some?) {}
    fun give(): Some? = null
    fun plain() = 1
}
fun same() = 1
`, root)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ src, want string }{
		{"fun f(b: l.Base) = b.plain()", ""},
		{"fun f(b: l.Base) = b.take(null)", "cannot access class 'r.Some'"},
		{"fun f(b: l.Base) = b.give()", "cannot access class 'r.Some'"},
		{"fun f() = l.Impl()", ""}, // only a warning in Kotlin 2.4
		{"fun f(i: l.Impl) = i.own()", "cannot access 'r.Missed' which is a supertype of 'Impl'"},
		{"class C : l.Mid", "cannot access 'r.Missed' which is a supertype of 'C'"},
	}
	for i, tt := range tests {
		_, err := compile("app", "package a\n\n"+tt.src+"\n", lib)
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("%d: %s: %v", i, tt.src, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("%d: %s: got %v, want an error with %q", i, tt.src, err, tt.want)
		}
	}
	// The same function of two modules on one class path is one
	// candidate, not an ambiguity.
	lib2, err := compile("lib2", "package l\n\nfun same() = 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compile("app", "package a\n\nfun f() = l.same()\n", root, lib, lib2); err != nil {
		t.Errorf("same function of two modules: %v", err)
	}
}
