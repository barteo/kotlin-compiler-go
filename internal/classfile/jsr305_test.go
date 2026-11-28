package classfile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
)

// TestJSR305Nonnull reads the when element of JSR-305's @Nonnull on
// methods and parameters, by which kotlinc reads the annotation, and
// leaves their annotations as they are: Member.When and ParamWhen are ""
// for ALWAYS, its default, and Nullability has @Nonnull as @CheckForNull
// for MAYBE and NEVER, and leaves it out for UNKNOWN. The annotation and
// When are stubs that kc compiles.
func TestJSR305Nonnull(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	srcs := []string{
		write("when.kt", `package javax.annotation.meta
enum class When { ALWAYS, UNKNOWN, MAYBE, NEVER }
`),
		write("nonnull.kt", `package javax.annotation
import javax.annotation.meta.When
annotation class Nonnull(val `+"`when`"+`: When = When.ALWAYS)
`),
		write("c.kt", `import javax.annotation.Nonnull
import javax.annotation.meta.When
class C {
    @Nonnull fun plain(): String = ""
    @Nonnull(When.ALWAYS) fun always(): String = ""
    @Nonnull(When.MAYBE) fun maybe(): String = ""
    @Nonnull(When.NEVER) fun never(): String = ""
    @Nonnull(When.UNKNOWN) fun unknown(): String = ""
    fun params(@Nonnull a: String, @Nonnull(When.MAYBE) b: String, @Nonnull(When.UNKNOWN) c: String) {}
}
`),
	}
	out := filepath.Join(dir, "out")
	if err := driver.Compile(srcs, driver.Options{OutDir: out, Jobs: 1}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "C.class"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := classfile.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	const nonnull, checkForNull = "Ljavax/annotation/Nonnull;", "Ljavax/annotation/CheckForNull;"
	// check checks the annotations as of a member or parameter, and
	// when, theirs, for when element want, and the nullability
	// annotation null that Nullability makes of them, "" for none.
	check := func(what string, as []string, when, want, null string) {
		t.Helper()
		if !slices.Contains(as, nonnull) {
			t.Errorf("%s: annotations %q, want @Nonnull among them", what, as)
		}
		if when != want {
			t.Errorf("%s: when %q, want %q", what, when, want)
		}
		got := classfile.Nullability(as, when)
		if has := func(a string) bool { return slices.Contains(got, a) }; null == "" && (has(nonnull) || has(checkForNull)) ||
			null == nonnull && (!has(nonnull) || has(checkForNull)) || null == checkForNull && (has(nonnull) || !has(checkForNull)) {
			t.Errorf("%s: nullability %q, want JSR-305's %q", what, got, null)
		}
	}
	methods := map[string]*classfile.Member{}
	for i := range info.Methods {
		methods[info.Methods[i].Name] = &info.Methods[i]
	}
	for _, tc := range []struct{ name, when, null string }{
		{"plain", "", nonnull},
		{"always", "", nonnull},
		{"maybe", "MAYBE", checkForNull},
		{"never", "NEVER", checkForNull},
		{"unknown", "UNKNOWN", ""},
	} {
		if m := methods[tc.name]; m == nil {
			t.Errorf("no method %s", tc.name)
		} else {
			check(tc.name, m.Annotations, m.When, tc.when, tc.null)
		}
	}
	m := methods["params"]
	if m == nil || len(m.ParamAnnotations) != 3 || len(m.ParamWhen) != 3 {
		t.Fatalf("params: %+v, want 3 parameters' annotations and when elements", m)
	}
	for i, tc := range []struct{ when, null string }{{"", nonnull}, {"MAYBE", checkForNull}, {"UNKNOWN", ""}} {
		check(fmt.Sprintf("params: parameter %d", i), m.ParamAnnotations[i], m.ParamWhen[i], tc.when, tc.null)
	}
}
