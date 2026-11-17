package ide

import (
	"fmt"
	"maps"
	"slices"
)

// InspectionLevels are the severities a user set for the hints'
// codes (Hints): 0 hides a code's hints, and a code not in the map keeps
// its own severity.
type InspectionLevels map[string]Severity

// InspectionCodes returns the codes of the hints, sorted.
func InspectionCodes() []string {
	codes := map[string]bool{
		UnusedImport: true, UnusedVariable: true, UnnecessarySafeCall: true, UnnecessaryNotNullAssertion: true,
		UselessCast: true, CanBeVal: true, UnusedParameter: true, UnusedPrivateMember: true,
	}
	maps.Copy(codes, findingCodes)
	maps.Copy(codes, idiomCodes)
	return slices.Sorted(maps.Keys(codes))
}

// ParseInspectionLevels reads the inspections setting: a level, off,
// hint, information or warning, by code. Unknown codes and levels are
// left out, with a warning each.
func ParseInspectionLevels(m map[string]string) (InspectionLevels, []string) {
	if len(m) == 0 {
		return nil, nil
	}
	known := InspectionCodes()
	levels := InspectionLevels{}
	var warnings []string
	for _, code := range slices.Sorted(maps.Keys(m)) {
		if _, ok := slices.BinarySearch(known, code); !ok {
			warnings = append(warnings, fmt.Sprintf("unknown inspection %q", code))
			continue
		}
		switch m[code] {
		case "off":
			levels[code] = 0
		case "hint":
			levels[code] = Hint
		case "information":
			levels[code] = Information
		case "warning":
			levels[code] = Warning
		default:
			warnings = append(warnings, fmt.Sprintf("unknown level %q of inspection %s (off, hint, information or warning)", m[code], code))
		}
	}
	return levels, warnings
}

// Apply returns hints with the levels set: those of codes turned off
// left out, the others with their codes' severities.
func (l InspectionLevels) Apply(hints []Diagnostic) []Diagnostic {
	if len(l) == 0 {
		return hints
	}
	out := hints[:0:0]
	for _, h := range hints {
		sev, ok := l[h.Code]
		switch {
		case !ok:
		case sev == 0:
			continue
		default:
			h.Severity = sev
		}
		out = append(out, h)
	}
	return out
}
