package ka

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// annotType writes t with its arguments and the annotations of each type
// in it, as libType does.
func annotType(t *Type) string {
	if t == nil {
		return "*"
	}
	var sb strings.Builder
	for _, a := range t.Annotations {
		sb.WriteString("@" + a.ClassID[strings.LastIndexByte(a.ClassID, '/')+1:] + " ")
	}
	sb.WriteString(t.ClassID + t.Name)
	if len(t.Args) > 0 {
		sb.WriteString("<")
		for i, a := range t.Args {
			if i > 0 {
				sb.WriteString(", ")
			}
			switch a.Variance {
			case In:
				sb.WriteString("in ")
			case Out:
				sb.WriteString("out ")
			}
			sb.WriteString(annotType(a.Type))
		}
		sb.WriteString(">")
	}
	return sb.String()
}

// TestJavaTypeAnnotations checks that the type annotations of a Java class
// file are on the types of its members and type parameters' bounds, where
// their paths lead, as stock KSP on FIR shows them.
func TestJavaTypeAnnotations(t *testing.T) {
	cfg := withJDK(t)
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("no javac")
	}
	dir := t.TempDir()
	files := map[string]string{
		"T.java": "package j;\nimport java.lang.annotation.*;\n@Target({ElementType.TYPE_USE, ElementType.FIELD}) public @interface T {}\n",
		"R.java": "package j;\nimport java.lang.annotation.*;\n@Retention(RetentionPolicy.RUNTIME) @Target(ElementType.TYPE_USE) public @interface R {}\n",
		"Foo.java": `package j;
import java.util.*;
public class Foo<X extends @T Number, Y extends @T Comparable<Y>> {
    public @T int a;
    public @R @T String s;
    public List<@T String> list;
    public Map<String, List<@R Integer>> nested;
    public String @T [] arr;
    public @T String [] el;
    public List<? extends @T Number> wild;
    public List<@T ? extends Number> wildSelf;
    public @T String m(@T int x, List<@T String> y, @T String... z) { return null; }
    public <Z extends @R Object> void g() {}
}
`,
	}
	var srcs []string
	for name, text := range files {
		p := filepath.Join(dir, "src", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		srcs = append(srcs, p)
	}
	classes := filepath.Join(dir, "classes")
	if out, err := exec.Command("javac", append([]string{"-d", classes}, srcs...)...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	cfg.ClassPath = append(cfg.ClassPath, classes)
	kotlin := "package k\nval foo = j.Foo<Int, String>()\n"
	m, err := Open(cfg).Update(context.Background(), []Source{{Path: "k/a.kt", Text: &kotlin}})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	d := m.Class("j/Foo")
	if d == nil {
		t.Fatal("j/Foo: nil")
	}
	var sb strings.Builder
	for _, tp := range d.TypeParams {
		for _, b := range tp.Bounds {
			sb.WriteString(tp.Name + " : " + annotType(b) + "\n")
		}
	}
	for _, md := range d.Members {
		switch md.Kind {
		case KindJavaField:
			sb.WriteString(md.Name + ": " + annotType(md.Type) + "\n")
		case KindFunction:
			sb.WriteString(md.Name)
			for _, tp := range md.TypeParams {
				for _, b := range tp.Bounds {
					sb.WriteString(" <" + tp.Name + " : " + annotType(b) + ">")
				}
			}
			sb.WriteString("(")
			for i, p := range md.Params {
				if i > 0 {
					sb.WriteString(", ")
				}
				if p.Flags&FlagVararg != 0 {
					sb.WriteString("vararg ")
				}
				sb.WriteString(annotType(p.Type))
			}
			sb.WriteString("): " + annotType(md.Type) + "\n")
		}
	}
	want := `X : @T kotlin/Number
Y : @T kotlin/Comparable<Y>
a: @T kotlin/Int
s: @R @T kotlin/String
list: kotlin/collections/MutableList<@T kotlin/String>
nested: kotlin/collections/MutableMap<kotlin/String, kotlin/collections/MutableList<@R kotlin/Int>>
arr: @T kotlin/Array<kotlin/String>
el: kotlin/Array<@T kotlin/String>
wild: kotlin/collections/MutableList<out @T kotlin/Number>
wildSelf: kotlin/collections/MutableList<out kotlin/Number>
m(@T kotlin/Int, kotlin/collections/MutableList<@T kotlin/String>, vararg @T kotlin/String): @T kotlin/String
g <Z : @R kotlin/Any>(): kotlin/Unit
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
