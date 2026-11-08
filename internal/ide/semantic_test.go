package ide

import (
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestSemanticDiagnostics(t *testing.T) {
	ds := []analysis.Diagnostic{
		{Range: psi.Range{Start: 1, End: 2}, Code: "source_error", Kotlinc: "REDECLARATION", Message: "conflicting declarations: x"},
		{Code: "source_error", Message: "ranges are only supported in 'for' loops and 'in' checks"},
		{Code: "unsupported:context receivers", Message: "context receivers are not supported"},
	}
	lowPrecision["LOW"] = true
	defer delete(lowPrecision, "LOW")
	ds = append(ds, analysis.Diagnostic{Code: "source_error", Kotlinc: "LOW", Message: "low"})
	sev := func(out []Diagnostic) []Severity {
		var s []Severity
		for _, d := range out {
			s = append(s, d.Severity)
		}
		return s
	}
	out := SemanticDiagnostics(ds, DiagnosticsDefault)
	if got, want := sev(out), []Severity{Error, Warning, Information, Warning}; !slices.Equal(got, want) {
		t.Errorf("default: severities %v, want %v", got, want)
	}
	if out[0].Code != "REDECLARATION" || out[0].Range != (psi.Range{Start: 1, End: 2}) || out[1].Code != "kc" ||
		out[2].Code != "unsupported:context receivers" || !strings.Contains(out[2].Message, "kotlinc may accept it") {
		t.Errorf("default: %+v", out)
	}
	if got, want := sev(SemanticDiagnostics(ds, DiagnosticsCoded)), []Severity{Error, Warning}; !slices.Equal(got, want) {
		t.Errorf("coded: severities %v, want %v", got, want)
	}
	if got, want := sev(SemanticDiagnostics(ds, DiagnosticsStrict)), []Severity{Error, Error, Error, Error}; !slices.Equal(got, want) {
		t.Errorf("strict: severities %v, want %v", got, want)
	}
	for s, want := range map[string]bool{"": true, "coded": true, "strict": true, "default": true, "loud": false} {
		if _, ok := ParseDiagnosticsMode(s); ok != want {
			t.Errorf("ParseDiagnosticsMode(%q) ok = %t", s, ok)
		}
	}
}

// TestLowPrecision checks lowPrecision against the measurement: the codes
// of bench/results/lsp-precision.tsv whose precision is below 0.99.
func TestLowPrecision(t *testing.T) {
	data, err := os.ReadFile("../../bench/results/lsp-precision.tsv")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		f := strings.Split(line, "\t")
		if i == 0 || len(f) < 7 || f[0] == "(none)" {
			continue
		}
		p, err := strconv.ParseFloat(f[6], 64)
		if err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		if p < 0.99 {
			want[f[0]] = true
		}
	}
	if !maps.Equal(lowPrecision, want) {
		t.Errorf("lowPrecision = %v, want %v", slices.Sorted(maps.Keys(lowPrecision)), slices.Sorted(maps.Keys(want)))
	}
}
