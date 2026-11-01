package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageSkip(t *testing.T) {
	tests := []struct{ directive, skip string }{
		{"", ""},
		{"// LANGUAGE: +ContextParameters\n", ""},
		{"// LANGUAGE: +ContextParameters +ExplicitContextArguments\n", "LANGUAGE"},
		{"// LANGUAGE: -ContextParameters\n", "LANGUAGE"},
		{"// LANGUAGE: +NestedTypeAliases\n", "LANGUAGE"},
	}
	dir := t.TempDir()
	for i, tt := range tests {
		path := filepath.Join(dir, "t.kt")
		if err := os.WriteFile(path, []byte(tt.directive+"fun box() = \"OK\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		test, err := readTest(path, "t.kt")
		if err != nil {
			t.Fatal(err)
		}
		if got := commonSkip(test); got != tt.skip {
			t.Errorf("%d: %q skips for %q, want %q", i, tt.directive, got, tt.skip)
		}
	}
}
