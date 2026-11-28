package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// TestWarnings compiles a program with a warning: it is printed and the
// compile succeeds, fails with -Werror, and is silent with -nowarn.
func TestWarnings(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "w.kt")
	if err := os.WriteFile(src, []byte("fun f(s: String) = s!!.length\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	const warning = "w.kt:1:21: warning: unnecessary non-null assertion"
	for _, tc := range []struct {
		flags []string
		code  int
		want  []string // substrings of the output, in order; none if empty
	}{
		{nil, 0, []string{warning}},
		{[]string{"-Werror"}, 1, []string{warning, "kc: warnings found and -Werror specified"}},
		{[]string{"-nowarn"}, 0, nil},
		{[]string{"-nowarn", "-Werror"}, 0, nil},
		{[]string{"-features"}, 0, []string{"w.kt\t1:21\twarning\tUNNECESSARY_NOT_NULL_ASSERTION\t\tunnecessary"}},
	} {
		var stderr strings.Builder
		args := append(append([]string{"-no-jdk", "-d", out}, tc.flags...), src)
		code, err := compile(args, &stderr, nil)
		if err != nil {
			t.Fatalf("%v: %v", tc.flags, err)
		}
		got := stderr.String()
		if code != tc.code {
			t.Errorf("%v: exit %d, want %d; output:\n%s", tc.flags, code, tc.code, got)
		}
		if len(tc.want) == 0 && got != "" {
			t.Errorf("%v: output %q, want none", tc.flags, got)
		}
		rest := got
		for _, w := range tc.want {
			i := strings.Index(rest, w)
			if i < 0 {
				t.Errorf("%v: output lacks %q:\n%s", tc.flags, w, got)
				break
			}
			rest = rest[i+len(w):]
		}
	}
}

// TestWarningsExtra compiles a program with unreachable code, which, as
// kotlinc, kc warns of only with -Wextra: -Werror fails the compile only
// then.
func TestWarningsExtra(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "w.kt")
	if err := os.WriteFile(src, []byte("fun f(): Int {\n    return 1\n    println()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	for _, tc := range []struct {
		flags []string
		code  int
		want  string
	}{
		{[]string{"-Werror"}, 0, ""},
		{[]string{"-Wextra"}, 0, "w.kt:3:5: warning: unreachable code"},
		{[]string{"-Wextra", "-Werror"}, 1, "w.kt:3:5: warning: unreachable code"},
	} {
		var stderr strings.Builder
		args := append(append([]string{"-no-jdk", "-d", out}, tc.flags...), src)
		code, err := compile(args, &stderr, nil)
		if err != nil {
			t.Fatalf("%v: %v", tc.flags, err)
		}
		got := stderr.String()
		if code != tc.code {
			t.Errorf("%v: exit %d, want %d; output:\n%s", tc.flags, code, tc.code, got)
		}
		if tc.want == "" && got != "" || !strings.Contains(got, tc.want) {
			t.Errorf("%v: output %q, want %q", tc.flags, got, tc.want)
		}
	}
}

// TestWarningsSyntaxErrors compiles a program with syntax errors: of its
// warnings, kc reports those that the statements left out for their
// errors can't be the reason for, and the redundant interpolation
// prefixes of the statements left out too, each once.
func TestWarningsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "w.kt")
	const program = `annotation class A(val s: String)

@A($"x")
class C {
    open fun f() {}
}
fun g(x: Int) {
    $"${}"
    $"kept"
    $$"${}"
    $$"kept"
    val unused = 1
    @Suppress("REDUNDANT_INTERPOLATION_PREFIX") $"s"
    val y = x!!
}
@Suppress("warnings")
fun h() { $"${}" }
fun <T : Int> k() { println($"a" + ) }
`
	if err := os.WriteFile(src, []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}
	// The files are checked only with a class path.
	if _, err := classpath.FindJDK(); err != nil {
		t.Skip(err)
	}
	var stderr strings.Builder
	code, err := compile([]string{"-features", "-Wextra", "-Xuse-fir-experimental-checkers", "-d", filepath.Join(dir, "out"), src}, &stderr, nil)
	if code != 1 || err == nil {
		t.Errorf("exit %d, error %v, want 1 and the syntax errors", code, err)
	}
	var got []string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if f := strings.Split(line, "\t"); len(f) > 3 && f[2] == "warning" {
			got = append(got, f[1]+" "+f[3])
		}
	}
	want := []string{
		"3:4 REDUNDANT_INTERPOLATION_PREFIX",
		"5:5 NON_FINAL_MEMBER_IN_FINAL_CLASS",
		"8:5 REDUNDANT_INTERPOLATION_PREFIX", // of a statement left out
		"9:5 REDUNDANT_INTERPOLATION_PREFIX",
		"11:5 REDUNDANT_INTERPOLATION_PREFIX",
		"18:10 FINAL_UPPER_BOUND",
		"18:29 REDUNDANT_INTERPOLATION_PREFIX",
	}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("warnings:\ngot  %v\nwant %v\noutput:\n%s", got, want, stderr.String())
	}
}

// TestWarningsLostCode compiles programs with syntax errors whose
// recovery leaves statements or declarations out: a statement left out
// of a body filters the warnings in that body only, but for those judged
// by literals alone, and a declaration left out filters them in the
// whole module.
func TestWarningsLostCode(t *testing.T) {
	if _, err := classpath.FindJDK(); err != nil {
		t.Skip(err) // the files are checked only with a class path
	}
	for _, tc := range []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name: "statement",
			files: []string{`fun g(x: Any, s: String) {
    s?.length
    x is Any
    1 is Int
    val y = 1 +
}
fun h(x: Any, s: String) {
    s?.length
    x is Any
}
`},
			want: []string{
				"a.kt:4:5 USELESS_IS_CHECK",      // judged by the literal alone
				"a.kt:8:8 UNNECESSARY_SAFE_CALL", // in a body that lost nothing
				"a.kt:9:5 USELESS_IS_CHECK",
			},
		},
		{
			name: "declaration",
			files: []string{`fun g(s: String) {
    s?.length
    1 is Int
}
fun broken( {
}
`},
			want: []string{"a.kt:3:5 USELESS_IS_CHECK"},
		},
		{
			name:  "other file, statement",
			files: []string{"fun g() {\n    val y = 1 +\n}\n", "fun h(s: String) {\n    s?.length\n}\n"},
			want:  []string{"b.kt:2:8 UNNECESSARY_SAFE_CALL"},
		},
		{
			name:  "other file, declaration",
			files: []string{"fun broken( {\n}\n", "fun h(s: String) {\n    s?.length\n}\n"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var srcs []string
			for i, f := range tc.files {
				src := filepath.Join(dir, string(rune('a'+i))+".kt")
				if err := os.WriteFile(src, []byte(f), 0o644); err != nil {
					t.Fatal(err)
				}
				srcs = append(srcs, src)
			}
			var stderr strings.Builder
			code, _ := compile(append([]string{"-features", "-d", filepath.Join(dir, "out")}, srcs...), &stderr, nil)
			if code != 1 {
				t.Errorf("exit %d, want 1 for the syntax errors", code)
			}
			var got []string
			for _, line := range strings.Split(stderr.String(), "\n") {
				if f := strings.Split(line, "\t"); len(f) > 3 && f[2] == "warning" {
					got = append(got, filepath.Base(f[0])+":"+f[1]+" "+f[3])
				}
			}
			if strings.Join(got, ", ") != strings.Join(tc.want, ", ") {
				t.Errorf("warnings:\ngot  %v\nwant %v\noutput:\n%s", got, tc.want, stderr.String())
			}
		})
	}
}
