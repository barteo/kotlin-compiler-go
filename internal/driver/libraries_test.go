package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// TestLibrariesShare checks which compiles Libraries gives the same class
// path: those of the same jars and settings, but not of a directory.
func TestLibrariesShare(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	ls := &Libraries{Max: 2}
	defer ls.Close()
	get := func(opts Options) (*classpath.Path, bool) {
		t.Helper()
		cp, l, done, shared, err := ls.get(opts)
		if err != nil {
			t.Fatal(err)
		}
		if l.Module != opts.ModuleName {
			t.Errorf("library of module %q, want %q", l.Module, opts.ModuleName)
		}
		done()
		return cp, shared
	}
	jars := Options{ClassPath: []string{lib}, ModuleName: "a"}
	first, shared := get(jars)
	if !shared {
		t.Fatal("a class path of jars isn't shared")
	}
	other := jars
	other.ModuleName = "b" // each compile's own
	if again, _ := get(other); again != first {
		t.Error("compiles of other modules don't share the class path")
	}
	compose := jars
	compose.Compose = true
	if c, _ := get(compose); c == first {
		t.Error("compiles with and without -compose share the class path")
	}
	dir := jars
	dir.ClassPath = append(dir.ClassPath, t.TempDir())
	if _, shared := get(dir); shared {
		t.Error("a class path with a directory is shared")
	}
	// Max is 2: a third configuration evicts the least recently used
	// that no compile uses.
	parcelize := jars
	parcelize.Parcelize = true
	get(parcelize)
	if again, _ := get(jars); again == first {
		t.Error("the least recently used class path wasn't evicted")
	}
	cp, _, done, _, err := ls.get(compose)
	if err != nil {
		t.Fatal(err)
	}
	noDefault := jars
	noDefault.NoJVMDefault = true
	get(jars)
	get(parcelize)
	get(noDefault) // three idle: one is evicted, but not compose's
	if again, _ := get(compose); again != cp {
		t.Error("a class path in use was evicted")
	}
	done()
}

// TestLibrariesPrograms compiles programs that declare the same classes
// differently against one shared library, as kc -serve does for test
// after test: each must see its own classes, and compile as it does
// alone.
func TestLibrariesPrograms(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	progs := []struct{ src, err string }{
		{"class A { fun f() = 1 }\nval x: Int = A().f()\nval l = listOf(1).map { it + 1 }\n", ""},
		{"class A { fun f() = \"s\" }\nval x: String = A().f()\nval l = listOf(\"a\").map { it.length }\n", ""},
		{"class A { fun f() = \"s\" }\nval x: Int = A().f()\n", "type mismatch"},
		{"open class A\nclass B : A()\nval a: A = B()\nval m = mapOf(1 to B())\n", ""},
	}
	compile := func(src string, libs *Libraries) (map[string][]byte, error) {
		dir := t.TempDir()
		path := filepath.Join(dir, "a.kt")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "out")
		err := Compile([]string{path}, Options{OutDir: out, Jobs: 1, ClassPath: []string{lib}, Libraries: libs})
		classes := map[string][]byte{}
		filepath.WalkDir(out, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				b, _ := os.ReadFile(p)
				rel, _ := filepath.Rel(out, p)
				classes[rel] = b
			}
			return nil
		})
		return classes, err
	}
	ls := &Libraries{}
	defer ls.Close()
	for round := range 2 {
		for i, p := range progs {
			alone, aloneErr := compile(p.src, nil)
			shared, sharedErr := compile(p.src, ls)
			if (sharedErr == nil) != (p.err == "") || p.err != "" && !strings.Contains(sharedErr.Error(), p.err) {
				t.Fatalf("round %d, program %d: error %v, want %q", round, i, sharedErr, p.err)
			}
			if (aloneErr == nil) != (sharedErr == nil) {
				t.Fatalf("round %d, program %d: alone: %v; shared: %v", round, i, aloneErr, sharedErr)
			}
			if len(alone) != len(shared) {
				t.Errorf("round %d, program %d: %d files alone, %d shared", round, i, len(alone), len(shared))
			}
			for name, b := range alone {
				if !bytes.Equal(b, shared[name]) {
					t.Errorf("round %d, program %d: %s differs from the one compiled alone", round, i, name)
				}
			}
		}
	}
}
