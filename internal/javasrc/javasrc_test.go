package javasrc

import (
	"archive/zip"
	"fmt"
	"io"

	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// jdkClasses looks up the JDK's classes, for the sources of the JDK's own.
type jdkClasses struct{ cp *classpath.Path }

func (j jdkClasses) Has(name string) bool { return j.cp.Has(name) }

func (j jdkClasses) Supers(name string) ([]string, bool) {
	info, err := j.cp.Load(name)
	if err != nil {
		return nil, false
	}
	supers := info.Interfaces
	if info.Super != "" {
		supers = append([]string{info.Super}, supers...)
	}
	return supers, info.Access&classfile.AccInterface != 0
}

func (j jdkClasses) Constant(class, field string) (any, string) {
	info, err := j.cp.Load(class)
	if err != nil {
		return nil, ""
	}
	for _, f := range info.Fields {
		if f.Name == field {
			return f.Constant, f.Desc
		}
	}
	return nil, ""
}

// TestJDK reads the declarations of some of the JDK's packages from its
// sources, lib/src.zip, and compares each public class with its class
// file: supertypes, signatures, access and the public and protected
// members, with their constant values.
func TestJDK(t *testing.T) {
	jdk, err := classpath.FindJDK()
	if err != nil {
		t.Skip("no JDK")
	}
	z, err := zip.OpenReader(filepath.Join(jdk, "lib", "src.zip"))
	if err != nil {
		t.Skip("the JDK has no lib/src.zip")
	}
	defer z.Close()
	cp, err := classpath.Open(jdk, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	prefixes := []string{"java.base/java/util/", "java.base/java/time/", "java.base/java/io/", "java.sql/", "java.net.http/", "java.xml/"}
	var files []*File
	for _, f := range z.File {
		match := false
		for _, p := range prefixes {
			match = match || strings.HasPrefix(f.Name, p)
		}
		if !match || !strings.HasSuffix(f.Name, ".java") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		src, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		jf, err := Parse(f.Name, string(src))
		if err != nil {
			t.Error(err)
			continue
		}
		files = append(files, jf)
	}
	s := NewSet(files)
	s.Outside = jdkClasses{cp}
	n := 0
	for _, name := range s.Names() {
		want, err := cp.Load(name)
		if err != nil || want.Access&classfile.AccPublic == 0 {
			continue
		}
		got, err := s.Info(name)
		if err != nil {
			t.Fatal(err)
		}
		n++
		if d := compare(got, want); len(d) > 0 {
			t.Errorf("%s:\n\t%s", name, strings.Join(d, "\n\t"))
		}
	}
	if n < 1000 {
		t.Errorf("compared %d classes, want over 1000", n)
	}
}

// compare returns the differences of two classes that matter to a
// compiler that calls them.
func compare(got, want *classfile.Info) []string {
	var d []string
	if got.Super != want.Super {
		d = append(d, fmt.Sprintf("super %s, want %s", got.Super, want.Super))
	}
	if g, w := strings.Join(got.Interfaces, ","), strings.Join(want.Interfaces, ","); g != w {
		d = append(d, fmt.Sprintf("interfaces %s, want %s", g, w))
	}
	if got.Sig != want.Sig {
		d = append(d, fmt.Sprintf("signature %s, want %s", got.Sig, want.Sig))
	}
	const classMask = classfile.AccPublic | classfile.AccFinal | classfile.AccAbstract | classfile.AccInterface | classfile.AccEnum | classfile.AccAnnotation
	if got.Access&classMask != want.Access&classMask {
		d = append(d, fmt.Sprintf("access %#x, want %#x", got.Access&classMask, want.Access&classMask))
	}
	members := func(ms []classfile.Member) map[string]string {
		const mask = classfile.AccPublic | classfile.AccProtected | classfile.AccStatic | classfile.AccFinal | classfile.AccAbstract | classfile.AccVarargs | classfile.AccEnum
		m := map[string]string{}
		for _, x := range ms {
			if x.Access&(classfile.AccPublic|classfile.AccProtected) != 0 && x.Access&(classfile.AccSynthetic|classfile.AccBridge) == 0 {
				m[x.Name+x.Desc] = fmt.Sprintf("%#x %s %v", x.Access&mask, x.Sig, x.Constant)
			}
		}
		return m
	}
	for _, pair := range [][2][]classfile.Member{{got.Fields, want.Fields}, {got.Methods, want.Methods}} {
		g, w := members(pair[0]), members(pair[1])
		var keys []string
		for k := range w {
			keys = append(keys, k)
		}
		for k := range g {
			if _, ok := w[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			if g[k] != w[k] {
				d = append(d, fmt.Sprintf("%s: %q, want %q", k, g[k], w[k]))
			}
		}
	}
	return d
}

func TestConstants(t *testing.T) {
	src := `package p;
import static p.Other.BASE;
public class C extends Other {
    public static final int A = 1 << 4 | 3, B = -A * 2, C = (int) 3.9, D = BASE + 1, E = INHERITED;
    public static final long L = 0xFFFF_FFFFL + 1, M = Long.MIN_VALUE;
    public static final byte BY = (byte) 200;
    public static final char CH = 'a' + 1, UNI = '\uD83D', OCT = '\101';
    public static final String S = "x" + A + 'c' + true + L, T = """
        two
          lines\
         joined
        """, U = A > 10 ? "big" : "small";
    public static final boolean Z = !(A == 19) || S.length() > 0;
    public static final float F = 1.5f / 2;
    public static final double INF = 1.0 / 0.0, NOT = Math.random();
    public static final int DIV0 = 1 / 0, CYCLE = CYCLE2, CYCLE2 = CYCLE;
}
class Other {
    static final int BASE = 41;
    public static final int INHERITED = 7;
}
`
	f, err := Parse("C.java", src)
	if err != nil {
		t.Fatal(err)
	}
	s := NewSet([]*File{f})
	info, err := s.Info("p/C")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"A": int32(19), "B": int32(-38), "C": int32(3), "D": int32(42), "E": int32(7),
		"L": int64(1 << 32), "M": nil, "BY": int32(-56), "CH": int32('b'), "UNI": int32(0xD83D), "OCT": int32('A'),
		"S": "x19ctrue4294967296", "T": "two\n  lines joined\n", "U": "big", "Z": nil, "F": float32(0.75),
		"NOT": nil, "DIV0": nil, "CYCLE": nil, "CYCLE2": nil,
	}
	got := map[string]any{}
	for _, fd := range info.Fields {
		got[fd.Name] = fd.Constant
	}
	for name, w := range want {
		if g, ok := got[name]; !ok || fmt.Sprint(g) != fmt.Sprint(w) {
			t.Errorf("%s = %#v, want %#v", name, g, w)
		}
	}
	// Infinity is a constant, but fmt can't tell +Inf from nil above.
	if f := got["INF"]; f == nil {
		t.Errorf("INF is not a constant")
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"class A { void f( }",
		"class A { int x = 1 }",
		`class A { String s = "unterminated; }`,
		"class A {",
	} {
		if _, err := Parse("A.java", src); err == nil {
			t.Errorf("%q: no error", src)
		}
	}
}
