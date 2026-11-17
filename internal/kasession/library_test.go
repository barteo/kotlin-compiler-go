package kasession

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
)

func TestKey(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "a.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := driver.Options{NoJDK: true, ClassPath: []string{jar}, ModuleName: "m"}
	k := Key(base)
	if k == "" || Key(base) != k {
		t.Fatalf("key %q not stable", k)
	}
	for name, o := range map[string]driver.Options{
		"module":     {NoJDK: true, ClassPath: []string{jar}, ModuleName: "n"},
		"opt-in":     {NoJDK: true, ClassPath: []string{jar}, ModuleName: "m", OptIns: []string{"a.B"}},
		"compose":    {NoJDK: true, ClassPath: []string{jar}, ModuleName: "m", Compose: true},
		"keepHidden": {NoJDK: true, ClassPath: []string{jar}, ModuleName: "m", KeepHidden: true},
	} {
		if Key(o) == k {
			t.Errorf("%s: same key", name)
		}
	}
	later := time.Now().Add(time.Hour)
	os.Chtimes(jar, later, later)
	if Key(base) == k {
		t.Error("a changed jar has the same key")
	}
	if Key(driver.Options{NoJDK: true, ClassPath: []string{dir}}) != "" {
		t.Error("a directory on the class path is shared")
	}
}

func TestAttach(t *testing.T) {
	l, err := Acquire(driver.Options{NoJDK: true, ModuleName: "attach"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	type k struct{}
	n := 0
	mk := func() any { n++; return &n }
	if a, b := l.Attach(k{}, mk), l.Attach(k{}, mk); a != b || n != 1 {
		t.Errorf("Attach made %d values", n)
	}
	l2, err := Acquire(driver.Options{NoJDK: true, ModuleName: "attach"})
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Release()
	if l2 != l {
		t.Error("the same options opened two libraries")
	}
}
