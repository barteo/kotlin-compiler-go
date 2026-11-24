package classfile_test

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
)

// TestOptInAnnotations reads the opt-in requirements that kc writes:
// @RequiresOptIn's level and @SubclassOptInRequired's markers.
func TestOptInAnnotations(t *testing.T) {
	lib, _ := classpath.FindStdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.kt")
	os.WriteFile(src, []byte(`
@RequiresOptIn
annotation class Err

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class Warn

@SubclassOptInRequired(Err::class)
open class Base

@Deprecated("", level = DeprecationLevel.HIDDEN)
class Gone
`), 0o644)
	out := filepath.Join(dir, "out")
	if err := driver.Compile([]string{src}, driver.Options{OutDir: out, Jobs: 1, ClassPath: []string{lib}}); err != nil {
		t.Fatal(err)
	}
	parse := func(name string) *classfile.Info {
		b, err := os.ReadFile(filepath.Join(out, name+".class"))
		if err != nil {
			t.Fatal(err)
		}
		info, err := classfile.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	if got := parse("Err").OptInLevel; got != "ERROR" {
		t.Errorf("Err: OptInLevel = %q, want ERROR", got)
	}
	if got := parse("Warn").OptInLevel; got != "WARNING" {
		t.Errorf("Warn: OptInLevel = %q, want WARNING", got)
	}
	if got := parse("Base").SubclassOptIn; !slices.Equal(got, []string{"Err"}) {
		t.Errorf("Base: SubclassOptIn = %q, want [Err]", got)
	}
	if got := parse("Gone").Deprecated; got == nil || got.Level != "HIDDEN" {
		t.Errorf("Gone: Deprecated = %+v, want level HIDDEN", got)
	}
}

// TestDeprecatedSinceKotlin reads @DeprecatedSinceKotlin(errorSince) of
// the standard library's String.subSequence(start, end) as level ERROR.
func TestDeprecatedSinceKotlin(t *testing.T) {
	lib, _ := classpath.FindStdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	z, err := zip.OpenReader(lib)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	f, err := z.Open("kotlin/text/StringsKt__StringsKt.class")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	info, err := classfile.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range info.Methods {
		if m.Name == "subSequence" && m.Desc == "(Ljava/lang/String;II)Ljava/lang/CharSequence;" {
			if m.Deprecated == nil || m.Deprecated.Level != "ERROR" {
				t.Errorf("subSequence: Deprecated = %+v, want level ERROR", m.Deprecated)
			}
			return
		}
	}
	t.Fatal("String.subSequence(start, end) not found")
}
