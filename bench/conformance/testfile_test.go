package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageSkip(t *testing.T) {
	saved := languageFeatures
	defer func() { languageFeatures = saved }()
	languageFeatures = map[string]bool{
		"ContextParameters":                   true,
		"NestedTypeAliases":                   true,
		"WhenGuards":                          true,
		"ExplicitContextArguments":            false,
		"ForbidParenthesizedLhsInAssignments": false,
	}
	tests := []struct{ directive, skip string }{
		{"", ""},
		{"// LANGUAGE: +ContextParameters\n", ""},
		{"// LANGUAGE: +NestedTypeAliases\n", ""},
		{"// LANGUAGE: +WhenGuards, +NestedTypeAliases\n", ""},
		{"// LANGUAGE: -ForbidParenthesizedLhsInAssignments\n", ""},
		{"// LANGUAGE: +ContextParameters +ExplicitContextArguments\n", "LANGUAGE"},
		{"// LANGUAGE: -ContextParameters\n", "LANGUAGE"},
		{"// LANGUAGE: +UnknownFeature\n", "LANGUAGE"},
		{"// LANGUAGE: -UnknownFeature\n", "LANGUAGE"},
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

func TestReadLanguageFeatures(t *testing.T) {
	const src = `enum class LanguageFeature(
    val sinceVersion: LanguageVersion?,
    val sinceApiVersion: ApiVersion = ApiVersion.KOTLIN_1_0,
) {
    // Kotlin 2.2
    WhenGuards(KOTLIN_2_2, "KT-13626"),
    ExplicitBackingFields(sinceVersion = KOTLIN_2_4, issue = "KT-14663"),
    NameBasedDestructuring(sinceVersion = KOTLIN_2_5, "KT-19627"),
    MultiPlatformProjects(sinceVersion = null, NO_ISSUE_SPECIFIED),
    ForbidParenthesizedLhsInAssignments(null, enabledInProgressiveMode = true, "KT-70507"),
    JvmNullOutSpilledCoroutineLocals(KOTLIN_2_2, ApiVersion.KOTLIN_2_2, "KT-63720"),
    WrapContinuationForTailCallFunctions(KOTLIN_2_4, sinceApiVersion = ApiVersion.KOTLIN_2_5, "KT-74051"),
    CollectionLiterals(sinceVersion = null, issue = "KT-80489") {
        override val testOnly = true
    },
    ;
    AfterTheEntries(KOTLIN_1_0),
}

enum class LanguageVersion(val major: Int, val minor: Int) {
    KOTLIN_2_4(2, 4),
    KOTLIN_2_5(2, 5),
    ;
    companion object {
        val LATEST_STABLE = KOTLIN_2_4
    }
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, languageSettings)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readLanguageFeatures(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"WhenGuards":                           true,
		"ExplicitBackingFields":                true,
		"NameBasedDestructuring":               false,
		"MultiPlatformProjects":                false,
		"ForbidParenthesizedLhsInAssignments":  false,
		"JvmNullOutSpilledCoroutineLocals":     true,
		"WrapContinuationForTailCallFunctions": false,
		"CollectionLiterals":                   false,
	}
	if len(got) != len(want) {
		t.Errorf("got %d features %v, want %d", len(got), got, len(want))
	}
	for f, on := range want {
		if g, ok := got[f]; !ok || g != on {
			t.Errorf("%s: got %v (known %v), want %v", f, g, ok, on)
		}
	}
}
