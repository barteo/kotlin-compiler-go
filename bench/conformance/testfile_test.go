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
		{"// LANGUAGE: +NestedTypeAliases\n", ""},
		{"// LANGUAGE: +NestedTypeAliases +LocalTypeAliases\n", "LANGUAGE"},
		{"// LANGUAGE: +ExplicitBackingFields\n", ""},
		{"// LANGUAGE: -ExplicitBackingFields\n", "LANGUAGE"},
		{"// LANGUAGE: +BreakContinueInInlineLambdas\n", ""},
		{"// LANGUAGE: +WhenGuards +BreakContinueInInlineLambdas\n", ""},
		{"// LANGUAGE: +MultiDollarInterpolation\n", ""},
		{"// LANGUAGE: +DataFlowBasedExhaustiveness\n", ""},
		{"// LANGUAGE: +AllowReifiedTypeInCatchClause\n", ""},
		{"// LANGUAGE: +AllowReturnInExpressionBodyWithExplicitType\n", ""},
		{"// LANGUAGE: +ProperIeee754Comparisons\n", ""},
		{"// LANGUAGE: -ProperIeee754Comparisons\n", "LANGUAGE"},
		{"// LANGUAGE: +JvmNullOutSpilledCoroutineLocals\n", ""},
		{"// LANGUAGE: +PCLAEnhancementsIn21\n", ""},
		{"// LANGUAGE: +ResolveTopLevelLambdasAsSyntheticCallArgument\n", ""},
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
