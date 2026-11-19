package main

import (
	"os"
	"path/filepath"
	"slices"
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
		{"// LANGUAGE: +AvoidWrongOptimizationOfTypeOperatorsOnValueClasses\n", ""},
		{"// LANGUAGE: +RefinedVarargConversionRulesForCallableReferences\n", ""},
		{"// LANGUAGE: +AllowAccessToProtectedFieldFromSuperCompanion\n", ""},
		{"// LANGUAGE: +NullableNothingInReifiedPosition\n", ""},
		{"// LANGUAGE: +PrioritizedEnumEntries\n", ""},
		{"// LANGUAGE: -ForbidParenthesizedLhsInAssignments\n", ""},
		{"// LANGUAGE: +ForbidParenthesizedLhsInAssignments\n", "LANGUAGE"},
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

// The code before a module's first // FILE: starts that file; comments
// alone are no file.
func TestCodeBeforeFirstFile(t *testing.T) {
	tests := []struct {
		src   string
		names []string
		first string // the first file's text
		line  int    // and its line
	}{
		{"// RUN_PIPELINE_TILL: BACKEND\nimport java.util.*;\n// FILE: A.java\nclass A {}\n// FILE: b.kt\nfun b() {}\n",
			[]string{"A.java", "b.kt"}, "// RUN_PIPELINE_TILL: BACKEND\nimport java.util.*;\n\nclass A {}\n", 1},
		{"// RUN_PIPELINE_TILL: BACKEND\n\n// FILE: A.java\nclass A {}\n",
			[]string{"A.java"}, "class A {}\n", 4},
		{"// MODULE: lib\nclass L\n// MODULE: main(lib)\n// FILE: m.kt\nfun m() {}\n",
			[]string{"t.kt", "m.kt"}, "class L\n", 2},
	}
	dir := t.TempDir()
	for i, tt := range tests {
		path := filepath.Join(dir, "t.kt")
		if err := os.WriteFile(path, []byte(tt.src), 0o644); err != nil {
			t.Fatal(err)
		}
		test, err := readTest(path, "t.kt")
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, f := range test.files() {
			names = append(names, f.name)
		}
		if !slices.Equal(names, tt.names) {
			t.Errorf("%d: files %v, want %v", i, names, tt.names)
			continue
		}
		if f := test.files()[0]; f.text != tt.first || f.line != tt.line {
			t.Errorf("%d: first file %q at line %d, want %q at %d", i, f.text, f.line, tt.first, tt.line)
		}
	}
}
