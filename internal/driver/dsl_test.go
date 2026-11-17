package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDslMarkerLibrary compiles a program against a library's DSL marker
// more than once in one process: each compilation reads the marker class
// anew, and reports the same use of an outer receiver.
func TestDslMarkerLibrary(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	tmp := t.TempDir()
	write := func(name, src string) string {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	libOut := filepath.Join(tmp, "lib")
	libSrc := write("lib.kt", `package dsl

@DslMarker
@Target(AnnotationTarget.CLASS, AnnotationTarget.TYPE)
annotation class M

@M
class A {
    fun a() {}
}

class B {
    fun b() {}
}

fun outer(block: A.() -> Unit) = A().block()
`)
	if err := Compile([]string{libSrc}, Options{OutDir: libOut, Jobs: 1, ClassPath: []string{lib}}); err != nil {
		t.Fatalf("compile library: %v", err)
	}
	cp := []string{lib, libOut}
	legal := write("legal.kt", `import dsl.*

fun inner(block: (@M B).() -> Unit) = B().block()

fun f() {
    outer {
        inner {
            b()
            this@outer.a()
        }
    }
}
`)
	illegal := write("illegal.kt", `import dsl.*

fun inner(block: (@M B).() -> Unit) = B().block()

fun f() {
    outer {
        inner {
            a()
        }
    }
}
`)
	const want = "illegal.kt:8:13: 'a' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary"
	for i := range 3 {
		if err := Compile([]string{legal}, Options{OutDir: filepath.Join(tmp, "legal"), Jobs: 1, ClassPath: cp}); err != nil {
			t.Errorf("compilation %d: %v", i, err)
		}
		err := Compile([]string{illegal}, Options{OutDir: filepath.Join(tmp, "illegal"), Jobs: 1, ClassPath: cp})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("compilation %d: got %v, want %s", i, err, want)
		}
	}
}
