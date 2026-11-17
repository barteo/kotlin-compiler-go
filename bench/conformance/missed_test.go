package main

import (
	"strings"
	"testing"
)

func TestMissCause(t *testing.T) {
	e := mark{"a.kt", expected{"X", 10}}
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
		{"nearby, marked", diagTest{kc: []diag{{file: "a.kt", line: 11}}, marks: []mark{{"a.kt", expected{"Y", 11}}}}, catRejectsInvalid, causeMissingCheck},
	}
	for _, tt := range tests {
		if got := tt.dt.missCause(tt.category, e); got != tt.want {
			t.Errorf("%s: missCause = %s, want %s", tt.name, got, tt.want)
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
	if err := compareMissed("base", []byte(base), cats, rs, now); err != nil {
		t.Errorf("no regression: %v", err)
	}

	now = append(now, "t1.kt\tt1.kt:7\tW\tnearby")
	err := compareMissed("base", []byte(base), cats, rs, now)
	if err == nil || !strings.Contains(err.Error(), "1 expected errors") {
		t.Errorf("one regression: got %v", err)
	}
}
