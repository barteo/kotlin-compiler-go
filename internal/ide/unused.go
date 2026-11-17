package ide

import (
	"slices"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// Codes of the hints kc reports, after kotlinc's warnings.
const (
	UnusedImport   = "UNUSED_IMPORT"
	UnusedVariable = "UNUSED_VARIABLE"
)

// Hints returns the file's unused imports and unused local variables, as
// hint diagnostics tagged unnecessary, which editors gray out. Imports
// are judged by the view's facts (UnusedImports), so a file with
// diagnostics has none; variables by their names: a local whose name
// isn't written again in its block.
func Hints(v *View, path string) []Diagnostic {
	snap := v.byPath[path]
	if snap == nil {
		return nil
	}
	var out []Diagnostic
	for _, r := range UnusedImports(v, path) {
		out = append(out, Diagnostic{Range: r, Severity: Hint, Code: UnusedImport, Message: "Unused import directive", Unnecessary: true})
	}
	for _, d := range UnusedVariables(snap) {
		out = append(out, Diagnostic{Range: nameRange(d), Severity: Hint, Code: UnusedVariable, Message: "Variable '" + d.Name() + "' is never used", Unnecessary: true})
	}
	out = append(out, Inspections(v, path)...)
	slices.SortStableFunc(out, func(a, b Diagnostic) int { return a.Range.Start - b.Range.Start })
	return out
}

// UnusedVariables returns the local variables of snap whose names no
// identifier after them in their block writes: no read, write, or
// reference from a string template, a lambda or a local function.
// Destructuring entries and _ are left out.
func UnusedVariables(snap *psi.Snapshot) []psi.NodeRef {
	var out []psi.NodeRef
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		if n.NumericKind() != psi.KindPROPERTY || n.Parent().NumericKind() != psi.KindBLOCK {
			return true
		}
		id := n.NameIdentifier()
		if !id.Valid() {
			return true
		}
		name := psi.Unquote(id.Text())
		if name == "_" {
			return true
		}
		used := false
		psi.WalkNodes(n.Parent(), func(m psi.NodeRef) bool {
			if used || m.Range().End <= id.Range().Start {
				return !used
			}
			if m.NumericKind() == psi.KindIDENTIFIER && m != id && psi.Unquote(m.Text()) == name {
				used = true
			}
			return !used
		})
		if !used {
			out = append(out, n)
		}
		return true
	})
	return out
}
