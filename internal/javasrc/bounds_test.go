package javasrc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// boundsSrc has nullability annotations on type parameters' bounds: on
// an Object, a class, an interface and a qualified bound, and on one of
// several bounds.
const boundsSrc = `package p;
import java.lang.annotation.*;
@Target(ElementType.TYPE_USE) @interface NotNull {}
@Target(ElementType.TYPE_USE) @interface Nullable {}
public class K<A, B extends @NotNull Object, C extends @Nullable Number> {
	public static <T extends @NotNull Comparable<T>> T cmp(T a) { return a; }
	public static <T, U extends Object & @NotNull Comparable<U>> U two(U a) { return a; }
	public static <T extends java.lang.@NotNull Object> T qual(T a) { return a; }
	public static <T> T none(T a) { return a; }
}
`

// TestBoundAnnotations checks that Info gives the nullability annotations
// on type parameters' bounds as class files do, by javac.
func TestBoundAnnotations(t *testing.T) {
	f, err := Parse("p/K.java", boundsSrc)
	if err != nil {
		t.Fatal(err)
	}
	src, err := NewSet([]*File{f}).Info("p/K")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"K": "[[] [Lp/NotNull;] [Lp/Nullable;]]", "cmp": "[[Lp/NotNull;]]",
		"two": "[[] [Lp/NotNull;]]", "qual": "[[Lp/NotNull;]]", "none": "[]"}
	check := func(what string, info *classfile.Info) {
		got := map[string]string{"K": fmt.Sprint(info.BoundAnnotations)}
		for _, m := range info.Methods {
			if _, ok := want[m.Name]; ok {
				got[m.Name] = fmt.Sprint(m.BoundAnnotations)
			}
		}
		for k, w := range want {
			if got[k] != w {
				t.Errorf("%s: %s: got %s, want %s", what, k, got[k], w)
			}
		}
	}
	check("source", src)

	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "K.java")
	if err := os.WriteFile(path, []byte(boundsSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(javac, "-d", dir, path).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	b, err := os.ReadFile(filepath.Join(dir, "p", "K.class"))
	if err != nil {
		t.Fatal(err)
	}
	cls, err := classfile.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	check("javac", cls)
}
