package ide

import (
	"fmt"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
)

// The semantic tier's diagnostics: kc's checker's errors, as an editor
// shows them. kc checks what it compiles, and some of its errors are its
// own limits rather than kotlinc's errors: those mustn't look like errors
// in code kotlinc compiles. So an error's severity says how sure it is
// (docs/lsp.md, milestone L1):
//
//   - An error of a construct kc doesn't support is information.
//   - An error that is one of kotlinc's diagnostics (analysis.Diagnostic's
//     Kotlinc) is an error, unless kc reports that diagnostic too often in
//     code kotlinc accepts (lowPrecision).
//   - Any other error is a warning.
//
// A syntax error (kotlinc's SYNTAX) isn't shown: SyntaxDiagnostics shows
// it, from the same parse.
//
// DiagnosticsMode changes that.

// DiagnosticsMode is the kc.diagnostics setting.
type DiagnosticsMode string

const (
	// DiagnosticsDefault shows errors as above.
	DiagnosticsDefault DiagnosticsMode = "default"
	// DiagnosticsCoded shows only the errors that are kotlinc's
	// diagnostics.
	DiagnosticsCoded DiagnosticsMode = "coded"
	// DiagnosticsStrict shows every error as an error, as kc's compiler
	// reports them.
	DiagnosticsStrict DiagnosticsMode = "strict"
)

// lowPrecision are the kotlinc diagnostics that kc reports in more than
// one in a hundred of the conformance tests it reports them in that
// kotlinc accepts (bench/results/lsp-precision.tsv), which are warnings.
var lowPrecision = map[string]bool{
	"MUST_BE_INITIALIZED_OR_BE_ABSTRACT":    true, // 0.930
	"NO_ELSE_IN_WHEN":                       true, // 0.919
	"NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY": true, // 0.981
	"NONE_APPLICABLE":                       true, // 0.879
	"NOTHING_TO_OVERRIDE":                   true, // 0.867
	"RETURN_TYPE_MISMATCH_ON_OVERRIDE":      true, // 0.909
	"TYPE_MISMATCH":                         true, // 0.880
	"UNRESOLVED_IMPORT":                     true, // 0.875
	"UNRESOLVED_LABEL":                      true, // 0.931
	"UNRESOLVED_REFERENCE":                  true, // 0.927
	"UNSAFE_CALL":                           true, // 0.970
	"UPPER_BOUND_VIOLATED":                  true, // 0.917
	"VAL_REASSIGNMENT":                      true, // 0.985
	"VIRTUAL_MEMBER_HIDDEN":                 true, // 0.917
}

// SemanticDiagnostics returns the editor's diagnostics of ds, the
// analysis diagnostics of one file, as mode shows them.
func SemanticDiagnostics(ds []analysis.Diagnostic, mode DiagnosticsMode) []Diagnostic {
	var out []Diagnostic
	for _, d := range ds {
		if d.Kotlinc == "SYNTAX" {
			continue
		}
		sd := Diagnostic{Range: d.Range, Severity: Error, Code: d.Kotlinc, Message: d.Message, Data: d.Data}
		feature, unsupported := strings.CutPrefix(d.Code, "unsupported:")
		switch {
		case mode == DiagnosticsStrict:
			if sd.Code == "" {
				sd.Code = d.Code
			}
		case unsupported:
			if mode == DiagnosticsCoded {
				continue
			}
			sd.Severity = Information
			sd.Code = d.Code
			sd.Message = fmt.Sprintf("kc doesn't support %s, so this file isn't checked further; kotlinc may accept it (%s)", feature, d.Message)
		case d.Kotlinc == "":
			if mode == DiagnosticsCoded {
				continue
			}
			sd.Severity = Warning
			sd.Code = "kc"
		case lowPrecision[d.Kotlinc]:
			sd.Severity = Warning
		}
		out = append(out, sd)
	}
	return out
}

// ParseDiagnosticsMode returns the mode a setting names; the default for
// "" or an unknown one, with ok false for an unknown one.
func ParseDiagnosticsMode(s string) (mode DiagnosticsMode, ok bool) {
	switch m := DiagnosticsMode(s); m {
	case DiagnosticsDefault, DiagnosticsCoded, DiagnosticsStrict:
		return m, true
	case "":
		return DiagnosticsDefault, true
	}
	return DiagnosticsDefault, false
}
