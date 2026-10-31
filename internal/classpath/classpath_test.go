package classpath

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

func jdk(t *testing.T) string {
	t.Helper()
	home, err := FindJDK()
	if err != nil {
		t.Skip(err)
	}
	return home
}

func TestJDKImage(t *testing.T) {
	p, err := Open(jdk(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	t.Logf("indexed %d classes in %v", p.Len(), p.IndexTime)
	if p.Len() < 20000 {
		t.Errorf("indexed %d classes, want at least 20000", p.Len())
	}
	if !p.HasPackage("java/util") || p.HasPackage("java/nope") {
		t.Error("HasPackage is wrong")
	}
	if !p.Has("java/lang/StringBuilder") || p.Has("java/lang/Nope") {
		t.Error("Has is wrong")
	}

	info, err := p.Load("java/lang/StringBuilder")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "java/lang/StringBuilder" || info.Super != "java/lang/AbstractStringBuilder" {
		t.Errorf("got class %s extends %s", info.Name, info.Super)
	}
	found := false
	for _, m := range info.Methods {
		if m.Name == "append" && m.Desc == "(Ljava/lang/String;)Ljava/lang/StringBuilder;" {
			found = m.Access&classfile.AccPublic != 0
		}
	}
	if !found {
		t.Error("public StringBuilder.append(String) not found")
	}
	if info, err := p.Load("java/util/ArrayList"); err != nil || !strings.HasPrefix(info.Sig, "<E:") {
		t.Errorf("ArrayList should be generic: %v %v", info, err)
	}
	if info, err := p.Load("java/lang/CharSequence"); err != nil || info.Access&classfile.AccInterface == 0 {
		t.Errorf("CharSequence should be an interface: %v", err)
	}
}

// TestConcurrentLoad is meant to run under -race: the checker loads
// classes from many goroutines.
func TestConcurrentLoad(t *testing.T) {
	p, err := Open(jdk(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	names := []string{"java/lang/String", "java/lang/Math", "java/util/Random", "java/math/BigInteger"}
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			if _, err := p.Load(names[i%len(names)]); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n, _ := p.Loaded(); n != len(names) {
		t.Errorf("loaded %d, want %d unique classes", n, len(names))
	}
}

type countingSource struct {
	data  []byte
	reads atomic.Int64
}

func (s *countingSource) read(*entry) ([]byte, error) {
	s.reads.Add(1)
	return append([]byte(nil), s.data...), nil
}

func (s *countingSource) close() error { return nil }

func TestConcurrentDependencyCache(t *testing.T) {
	c := classfile.NewClass(classfile.AccPublic, "demo/A", "java/lang/Object")
	src := &countingSource{data: c.Bytes()}
	p := &Path{classes: map[string]entry{"demo/A": {src: src}}}
	readers := make([]*classfile.MethodReader, 32)
	var wg sync.WaitGroup
	for i := range readers {
		wg.Go(func() {
			if _, err := p.Bytes("demo/A"); err != nil {
				t.Error(err)
			}
			if _, err := p.Load("demo/A"); err != nil {
				t.Error(err)
			}
			var err error
			readers[i], err = p.MethodReader("demo/A")
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := src.reads.Load(); n != 1 {
		t.Errorf("read dependency %d times, want 1", n)
	}
	for _, r := range readers {
		if r == nil || r != readers[0] {
			t.Fatal("method reader was not shared")
		}
	}
}

// TestJarAndDir builds a class with the classfile writer and reads it back
// from a jar and from a directory.
func TestJarAndDir(t *testing.T) {
	c := classfile.NewClass(classfile.AccPublic|classfile.AccSuper, "demo/Greeter", "java/lang/Object")
	c.AddField(classfile.AccPublic|classfile.AccStatic, "COUNT", "I")
	code := c.NewMethod(classfile.AccPublic|classfile.AccStatic, "hello", "()V")
	code.Op(classfile.RETURN)
	data := c.Bytes()

	tmp := t.TempDir()
	jarPath := filepath.Join(tmp, "demo.jar")
	f, err := os.Create(jarPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{"demo/Greeter.class", "META-INF/versions/9/demo/Greeter.class"} {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	// Kotlin metadata files are indexed as resources.
	for _, name := range []string{"META-INF/demo.kotlin_module", "demo/demo.kotlin_builtins", "META-INF/x/y.kotlin_module", "demo/notes.txt"} {
		w, _ := zw.Create(name)
		w.Write([]byte(name))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	classDir := filepath.Join(tmp, "classes")
	os.MkdirAll(filepath.Join(classDir, "demo"), 0o755)
	os.WriteFile(filepath.Join(classDir, "demo", "Other.class"), data, 0o644)
	shadow := classfile.NewClass(classfile.AccPublic, "demo/Greeter", "java/lang/Object")
	shadow.AddField(classfile.AccPublic, "SHADOW", "I")
	os.WriteFile(filepath.Join(classDir, "demo", "Greeter.class"), shadow.Bytes(), 0o644)
	os.MkdirAll(filepath.Join(classDir, "META-INF"), 0o755)
	os.WriteFile(filepath.Join(classDir, "META-INF", "other.kotlin_module"), []byte("other"), 0o644)
	os.WriteFile(filepath.Join(classDir, "META-INF", "demo.kotlin_module"), []byte("shadowed"), 0o644)

	p, err := Open("", []string{jarPath, classDir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Len() != 2 {
		t.Errorf("indexed %d classes, want 2", p.Len())
	}
	for _, name := range []string{"demo/Greeter", "demo/Other"} {
		info, err := p.Load(name)
		if err != nil {
			t.Fatal(err)
		}
		if info.Name != "demo/Greeter" || len(info.Fields) != 1 || len(info.Methods) != 1 ||
			info.Methods[0].Name != "hello" || info.Fields[0].Desc != "I" {
			t.Errorf("%s: unexpected contents %+v", name, info)
		}
	}

	if got, want := p.Resources(".kotlin_module"), []string{"META-INF/demo.kotlin_module", "META-INF/other.kotlin_module"}; !slices.Equal(got, want) {
		t.Errorf("module files %v, want %v", got, want)
	}
	if got := p.Resources(".kotlin_builtins"); !slices.Equal(got, []string{"demo/demo.kotlin_builtins"}) {
		t.Errorf("builtins files %v", got)
	}
	for name, want := range map[string]string{"META-INF/demo.kotlin_module": "META-INF/demo.kotlin_module", "META-INF/other.kotlin_module": "other"} {
		if got, err := p.Resource(name); err != nil || string(got) != want {
			t.Errorf("%s: %q, %v; want %q", name, got, err, want)
		}
	}
	// Every copy of a module file is kept, in class path order.
	if all, err := p.ResourceAll("META-INF/demo.kotlin_module"); err != nil || len(all) != 2 ||
		string(all[0]) != "META-INF/demo.kotlin_module" || string(all[1]) != "shadowed" {
		t.Errorf("all copies of demo.kotlin_module: %q, %v", all, err)
	}
	if p.HasKotlin() {
		t.Error("HasKotlin without kotlin/kotlin.kotlin_builtins")
	}
	// A second compilation has its own cache, and reversed class path
	// precedence must produce the directory's definition, not the jar's.
	reversed, err := Open("", []string{classDir, jarPath})
	if err != nil {
		t.Fatal(err)
	}
	defer reversed.Close()
	info, err := reversed.Load("demo/Greeter")
	if err != nil || len(info.Fields) != 1 || info.Fields[0].Name != "SHADOW" {
		t.Fatalf("cache ignored class path precedence: %+v, %v", info, err)
	}
}

// A source file's class hides a class file of the same name, and has no
// bytes.
func TestAddSource(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "demo"), 0o755)
	c := classfile.NewClass(classfile.AccPublic|classfile.AccSuper, "demo/A", "java/lang/Object")
	os.WriteFile(filepath.Join(dir, "demo", "A.class"), c.Bytes(), 0o644)
	p, err := Open("", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	// A cached binary must not hide a subsequently registered source.
	if _, err := p.Load("demo/A"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.MethodReader("demo/A"); err != nil {
		t.Fatal(err)
	}
	if p.HasSources() {
		t.Error("HasSources before AddSource")
	}
	for _, name := range []string{"demo/A", "src/B"} {
		p.AddSource(name, func() (*classfile.Info, error) { return &classfile.Info{Name: name, Super: "java/lang/Object"}, nil })
	}
	if !p.HasSources() || !p.Has("src/B") || !p.HasPackage("src") || !p.IsSource("demo/A") {
		t.Error("source classes not indexed")
	}
	info, err := p.Load("demo/A")
	if err != nil || info.Name != "demo/A" || len(info.Methods) != 0 {
		t.Errorf("Load(demo/A) = %+v, %v; want the source's", info, err)
	}
	if _, err := p.Bytes("src/B"); err == nil {
		t.Error("Bytes of a source class: no error")
	}
	if _, err := p.MethodReader("demo/A"); err == nil {
		t.Error("MethodReader of a source class: no error")
	}
}
