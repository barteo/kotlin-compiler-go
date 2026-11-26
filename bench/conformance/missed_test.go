package main

import (
	"strings"
	"testing"
)

func TestMissCause(t *testing.T) {
	e := mark{"a.kt", expected{name: "X", line: 10}}
	tests := []struct {
		name     string
		dt       diagTest
		category string
		want     string
	}{
		{"accepted", diagTest{}, catAcceptsInvalid, causeAccepted},
		{"parse", diagTest{parseErrs: true, kc: []diag{{file: "a.kt", line: 1}}}, catRejectsInvalid, causeParse},
		{"module", diagTest{tried: map[string]bool{"b.kt": true}, kc: []diag{{file: "b.kt", line: 1}}}, catRejectsInvalid, causeModule},
		{"nearby", diagTest{kc: []diag{{file: "a.kt", line: 12}}}, catRejectsInvalid, causeNearby},
		{"other file", diagTest{kc: []diag{{file: "b.kt", line: 10}}}, catRejectsInvalid, causeMissingCheck},
		{"far", diagTest{kc: []diag{{file: "a.kt", line: 20}}}, catRejectsInvalid, causeMissingCheck},
		{"nearby, marked", diagTest{kc: []diag{{file: "a.kt", line: 11}}, marks: []mark{{"a.kt", expected{name: "Y", line: 11}}}}, catRejectsInvalid, causeMissingCheck},
	}
	for _, tt := range tests {
		if got := tt.dt.missCause(tt.category, e); got != tt.want {
			t.Errorf("%s: missCause = %s, want %s", tt.name, got, tt.want)
		}
	}
	// Names only the full standard library declares, in a test without
	// WITH_STDLIB.
	only := map[string]bool{"println": true, "rangeTo": true}
	for _, tt := range []struct {
		dt   diagTest
		e    expected
		want string
	}{
		{diagTest{stdlibOnly: only}, expected{name: "UNRESOLVED_REFERENCE", line: 10, text: "println"}, causeEnvironment},
		{diagTest{stdlibOnly: only}, expected{name: "UNRESOLVED_REFERENCE", line: 10, text: ".."}, causeEnvironment},
		{diagTest{stdlibOnly: only}, expected{name: "UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH", line: 10, text: "1u"}, causeEnvironment},
		{diagTest{stdlibOnly: only}, expected{name: "UNRESOLVED_REFERENCE", line: 10, text: "foo"}, causeMissingCheck},
		{diagTest{stdlibOnly: only}, expected{name: "ARGUMENT_TYPE_MISMATCH", line: 10, text: "println"}, causeMissingCheck},
		{diagTest{}, expected{name: "UNRESOLVED_REFERENCE", line: 10, text: "println"}, causeMissingCheck},
	} {
		if got := tt.dt.missCause(catRejectsInvalid, mark{"a.kt", tt.e}); got != tt.want {
			t.Errorf("%+v: missCause = %s, want %s", tt.e, got, tt.want)
		}
	}
}

func TestCompareMissed(t *testing.T) {
	base := missedHeader + "\n" +
		"t1.kt\tt1.kt:3\tX\tnearby\n" +
		"t1.kt\tt1.kt:5\tY\tmissing-check\n" +
		"t1.kt\tt1.kt:5\tY\tmissing-check\n"
	cats := map[string]string{"t1.kt": catRejectsInvalid, "t2.kt": catUnsupported}
	rs := []*result{{path: "t1.kt", category: catRejectsInvalid}, {path: "t2.kt", category: catRejectsInvalid}}

	// One of the two Y errors is reported now, X is still missed (with
	// another cause), and t2's errors weren't checked in the baseline.
	now := []string{"t1.kt\tt1.kt:3\tX\tmissing-check", "t1.kt\tt1.kt:5\tY\tmissing-check", "t2.kt\tt2.kt:1\tZ\tnearby"}
	if err := compareMissed("errors", "base", []byte(base), cats, rs, now); err != nil {
		t.Errorf("no regression: %v", err)
	}

	now = append(now, "t1.kt\tt1.kt:7\tW\tnearby")
	err := compareMissed("errors", "base", []byte(base), cats, rs, now)
	if err == nil || !strings.Contains(err.Error(), "1 expected errors") {
		t.Errorf("one regression: got %v", err)
	}
	err = compareMissed("warnings", "base", []byte(base), cats, rs, now)
	if err == nil || !strings.Contains(err.Error(), "1 expected warnings") {
		t.Errorf("one regression in warnings: got %v", err)
	}
}

func TestMissedWarningLines(t *testing.T) {
	sev := map[string]string{"W": sevWarning, "E": sevError}
	w := func(file string, line int) mark { return mark{file, expected{name: "W", line: line}} }
	rs := []*result{
		{path: "hit.kt", category: catAcceptsValid},
		{path: "broken.kt", category: catRejectsInvalid},
		{path: "modules.kt", category: catRejectsInvalid},
		{path: "crash.kt", category: catCrash},
	}
	dts := []*diagTest{
		// One of two warnings on their lines, an error mark, and a
		// warning of another name on the missed one's line.
		{marks: []mark{w("a.kt", 1), w("a.kt", 2), {"a.kt", expected{name: "E", line: 3}}},
			kcWarns: []diag{{file: "a.kt", line: 1, name: "W"}, {file: "a.kt", line: 2, name: "V"}}},
		{marks: []mark{w("b.kt", 4)}, parseErrs: true},
		{marks: []mark{w("c.kt", 5), w("d.kt", 6)}, tried: map[string]bool{"c.kt": true}},
		{marks: []mark{w("e.kt", 7)}},
	}
	want := []string{
		"broken.kt\tb.kt:4\tW\t" + causeParse,
		"hit.kt\ta.kt:2\tW\t" + causeMissingCheck,
		"modules.kt\tc.kt:5\tW\t" + causeMissingCheck,
		"modules.kt\td.kt:6\tW\t" + causeModule,
	}
	if got := missedWarningLines(sev, rs, dts); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("missedWarningLines:\ngot  %q\nwant %q", got, want)
	}
}
