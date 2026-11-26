package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSplitArgFile(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"-d out\n-cp a.jar:b.jar\r\nx.kt", []string{"-d", "out", "-cp", "a.jar:b.jar", "x.kt"}},
		{`"/a b/c.kt" 'it''s' "q\"uote" "back\\slash" ""`, []string{"/a b/c.kt", "its", `q"uote`, `back\slash`, ""}},
	} {
		got, err := splitArgFile(tc.in)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("splitArgFile(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	if _, err := splitArgFile(`"open`); err == nil {
		t.Error("an unterminated quote isn't an error")
	}
}

// TestArgFile compiles with the arguments in a file.
func TestArgFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a b.kt")
	if err := os.WriteFile(src, []byte("fun main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	args := filepath.Join(dir, "kc.args")
	if err := os.WriteFile(args, []byte("-no-jdk\n-d "+`"`+out+`"`+"\n"+`"`+src+`"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	if code, err := compile([]string{"@" + args}, &stderr, nil); code != 0 || err != nil {
		t.Fatalf("exit %d, %v: %s", code, err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "A_bKt.class")); err != nil {
		entries, _ := os.ReadDir(out)
		t.Fatalf("%v; out has %v", err, entries)
	}
}
