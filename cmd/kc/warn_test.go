package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
