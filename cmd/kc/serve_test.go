package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestServe compiles through serve: a program, one with an error, bad
// flags, and a compile that fails other than with a diagnostic, after
// which the server exits without reading the next request.
func TestServe(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write("good.kt", "fun f(): Int = 1\n")
	bad := write("bad.kt", "val x: Int = \"s\"\n")
	notDir := write("file", "")
	out := filepath.Join(dir, "out")
	var in strings.Builder
	for _, args := range [][]string{
		{"-no-jdk", "-features", "-d", out, good},
		{"-no-jdk", "-features", "-d", out, good}, // again, on the shared library
		{"-no-jdk", "-features", "-d", out, bad},
		{"-bogus"},
		{"-no-jdk", "-d", filepath.Join(notDir, "out"), good}, // can't write: exits
		{"-no-jdk", "-d", out, good},                          // not read
	} {
		b, _ := json.Marshal(args)
		in.Write(append(b, '\n'))
	}
	var answers strings.Builder
	if err := serve(strings.NewReader(in.String()), &answers); err != nil {
		t.Fatal(err)
	}
	type answer struct {
		Code int
		Out  string
		Exit bool
	}
	var got []answer
	sc := bufio.NewScanner(strings.NewReader(answers.String()))
	for sc.Scan() {
		var a answer
		if err := json.Unmarshal(sc.Bytes(), &a); err != nil {
			t.Fatalf("answer %s: %v", sc.Text(), err)
		}
		got = append(got, a)
	}
	if len(got) != 5 {
		t.Fatalf("%d answers, want 5:\n%s", len(got), answers.String())
	}
	for i, want := range []struct {
		code int
		out  string
		exit bool
	}{
		{0, "", false},
		{0, "", false},
		{1, "bad.kt\t1:14\t\ttype mismatch", false},
		{2, "flag provided but not defined: -bogus", false},
		{1, "not a directory", true},
	} {
		a := got[i]
		if a.Code != want.code || a.Exit != want.exit || !strings.Contains(a.Out, want.out) || want.out == "" && a.Out != "" {
			t.Errorf("answer %d: %+v, want code %d, exit %t, output with %q", i, a, want.code, want.exit, want.out)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "GoodKt.class")); err != nil {
		t.Error(err)
	}
}
