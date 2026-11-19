package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// parseKcLine reads a diagnostic that kc -features printed: path,
// line:col, severity, name, unsupported feature and message, separated by
// tabs. It returns the diagnostic, whether it is a warning, and whether
// the line is one.
func parseKcLine(line string, byPath map[string]string) (d diag, warning, ok bool) {
	f := strings.SplitN(line, "\t", 6)
	if len(f) != 6 || f[2] != "error" && f[2] != "warning" {
		return diag{}, false, false
	}
	var l, col int
	fmt.Sscanf(f[1], "%d:%d", &l, &col)
	file := byPath[f[0]]
	if file == "" {
		file = filepath.Base(f[0])
	}
	return diag{file: file, line: l, name: f[3], feature: f[4], msg: f[5]}, f[2] == "warning", true
}

// kcWarnings returns the warnings that kc -features printed.
func kcWarnings(out string, byPath map[string]string) []diag {
	var ws []diag
	for _, line := range strings.Split(out, "\n") {
		if d, warning, ok := parseKcLine(line, byPath); ok && warning {
			ws = append(ws, d)
		}
	}
	return ws
}

// uncheckedWarnings are the warnings kotlinc reports by default that the
// diagnostics tests' markup leaves out unless a test asks for them: with
// WITH_EXTRA_CHECKERS, a DIAGNOSTICS directive's +NAME, or a mark of the
// warning. The markup of the tests that ask for none never expects them,
// though kotlinc reports them there.
var uncheckedWarnings = []string{"UNUSED_EXPRESSION", "UNUSED_LAMBDA_EXPRESSION"}

// switchedOff returns the names of the diagnostics that test t, with
// marks ms, switches off: those its DIAGNOSTICS directive does, -NAME,
// and the uncheckedWarnings it doesn't ask for.
func switchedOff(t *test, ms []mark) map[string]bool {
	off := map[string]bool{}
	on := map[string]bool{}
	for _, v := range t.values("DIAGNOSTICS") {
		for _, f := range strings.Fields(v) {
			if n, ok := strings.CutPrefix(f, "-"); ok {
				off[n] = true
			} else if n, ok := strings.CutPrefix(f, "+"); ok {
				on[n] = true
			}
		}
	}
	if _, ok := t.directives["WITH_EXTRA_CHECKERS"]; !ok {
		for _, n := range uncheckedWarnings {
			if !on[n] && !slices.ContainsFunc(ms, func(m mark) bool { return m.name == n }) {
				off[n] = true
			}
		}
	}
	return off
}

// warnHit reports whether kc reported the expected warning e on its line.
func (dt *diagTest) warnHit(e mark) bool {
	return slices.ContainsFunc(dt.kcWarns, func(d diag) bool { return d.file == e.file && d.line == e.line && d.name == e.name })
}

// strayWarnings returns kc's warnings on lines that don't expect them,
// but those the test switches off.
func (dt *diagTest) strayWarnings() []diag {
	var ds []diag
	for _, d := range dt.kcWarns {
		if dt.off[d.name] || dt.off["warnings"] {
			continue
		}
		if !slices.ContainsFunc(dt.marks, func(m mark) bool { return m.file == d.file && m.line == d.line && m.name == d.name }) {
			ds = append(ds, d)
		}
	}
	return ds
}

// compiled reports whether kc compiled test i to its diagnostics: it
// didn't crash or find an unsupported feature.
func compiled(r *result, dt *diagTest) bool {
	return dt != nil && r.category != catUnsupported && r.category != catCrash && r.category != catSkip && r.category != catKc
}

// writeStrayWarnings writes kc's warnings on lines that don't expect
// them as TSV: the test's path, its category, file:line, the warning's
// name and its message.
func writeStrayWarnings(name string, rs []*result, dts []*diagTest) error {
	var sb strings.Builder
	for i, dt := range dts {
		if !compiled(rs[i], dt) {
			continue
		}
		for _, d := range dt.strayWarnings() {
			msg := strings.NewReplacer("\t", " ", "\n", " ").Replace(d.msg)
			fmt.Fprintf(&sb, "%s\t%s\t%s:%d\t%s\t%s\n", rs[i].path, rs[i].category, d.file, d.line, d.name, msg)
		}
	}
	return os.WriteFile(name, []byte(sb.String()), 0o644)
}

// reportWarnings writes the report's section on warnings: for each
// warning kc reports, how many of those the tests expect it reports on
// their lines, and how many it reports elsewhere; then the warnings the
// tests expect that kc doesn't report.
func reportWarnings(sb *strings.Builder, c *diagConfig, rs []*result, dts []*diagTest) {
	type counts struct{ expected, hit, stray int }
	byName := map[string]*counts{}
	get := func(n string) *counts {
		if byName[n] == nil {
			byName[n] = &counts{}
		}
		return byName[n]
	}
	reported := map[string]bool{}
	var expected, hit, stray int
	for i, dt := range dts {
		if !compiled(rs[i], dt) {
			continue
		}
		for _, m := range dt.marks {
			if c.sev[m.name] != sevWarning {
				continue
			}
			expected++
			get(m.name).expected++
			if dt.warnHit(m) {
				hit++
				get(m.name).hit++
			}
		}
		for _, d := range dt.kcWarns {
			reported[d.name] = true
		}
		for _, d := range dt.strayWarnings() {
			stray++
			get(d.name).stray++
		}
	}
	fmt.Fprintf(sb, "## Warnings\n\n")
	fmt.Fprintf(sb, "Of the tests kc compiled, warnings don't decide the category: those are about errors. kc reports %d of the %d warnings the tests expect (%s) on their lines, and %d warnings on lines that don't expect them, but for those a test's `DIAGNOSTICS` directive switches off.\n\n",
		hit, expected, pct(hit, expected), stray)
	var names []string
	for n := range reported {
		names = append(names, n)
	}
	for n, k := range byName {
		if k.hit > 0 && !reported[n] {
			names = append(names, n)
		}
	}
	slices.SortFunc(names, func(a, b string) int {
		if d := get(b).expected - get(a).expected; d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	if len(names) > 0 {
		fmt.Fprintf(sb, "| Warning kc reports | Expected | On their lines | Elsewhere |\n|:---|---:|---:|---:|\n")
		for _, n := range names {
			k := get(n)
			fmt.Fprintf(sb, "| `%s` | %d | %d | %d |\n", n, k.expected, k.hit, k.stray)
		}
		fmt.Fprintln(sb)
	}
	type kc struct {
		key string
		n   int
	}
	var backlog []kc
	for n, k := range byName {
		if !reported[n] && k.expected > 0 {
			backlog = append(backlog, kc{n, k.expected})
		}
	}
	slices.SortFunc(backlog, func(a, b kc) int {
		if a.n != b.n {
			return b.n - a.n
		}
		return strings.Compare(a.key, b.key)
	})
	fmt.Fprintf(sb, "The warnings the tests expect that kc doesn't report, as a backlog.\n\n")
	fmt.Fprintf(sb, "| Warning | Count |\n|:---|---:|\n")
	for i, w := range backlog {
		if i == 30 {
			fmt.Fprintf(sb, "| *%d more* | |\n", len(backlog)-i)
			break
		}
		fmt.Fprintf(sb, "| `%s` | %d |\n", w.key, w.n)
	}
	fmt.Fprintln(sb)
}
