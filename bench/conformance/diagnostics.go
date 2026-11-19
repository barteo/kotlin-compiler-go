package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// The categories of a diagnostics test's result, besides skip, crash and
// parse/unsupported, which the box mode's are.
const (
	catRejectsInvalid = "rejects invalid"
	catAcceptsValid   = "accepts valid"
	catAcceptsInvalid = "accepts invalid"
	catRejectsValid   = "rejects valid"
)

// The suites of compiler/testData/diagnostics that are for the JVM, or
// any platform.
var diagSuites = []string{"tests/", "testsWithStdLib/", "testsWithJvmBackend/", "testsWithAnyBackend/"}

// The files that declare the diagnostics, with their severities,
// relative to the checkout.
var diagDefs = []string{
	"compiler/fir/checkers/gen/org/jetbrains/kotlin/fir/analysis/diagnostics/FirErrors.kt",
	"compiler/fir/checkers/checkers.jvm/gen/org/jetbrains/kotlin/fir/analysis/diagnostics/jvm/FirJvmErrors.kt",
	"compiler/fir/raw-fir/raw-fir.common/gen/org/jetbrains/kotlin/fir/builder/FirSyntaxErrors.kt",
	"compiler/ir/backend.jvm/src/org/jetbrains/kotlin/backend/jvm/JvmBackendErrors.kt",
	"compiler/ir/backend.common/src/org/jetbrains/kotlin/backend/common/CommonBackendErrors.kt",
}

// variantRE matches the files that are variants of another test's, for
// the Analysis API, the latest language version and so on.
var variantRE = regexp.MustCompile(`\.(ll|latestLV|disabled|reversed|partialBody)\.kt$`)

// isDiagTest reports whether the file at rel, relative to
// compiler/testData/diagnostics, is a diagnostics test for the JVM.
func isDiagTest(rel string) bool {
	if variantRE.MatchString(rel) {
		return false
	}
	for _, s := range diagSuites {
		if strings.HasPrefix(rel, s) {
			return true
		}
	}
	return false
}

// diagSkip returns why the diagnostics mode doesn't run t, or "".
func diagSkip(t *test) string {
	if r := commonSkip(t); r != "" {
		return r
	}
	for _, d := range []string{"IGNORE_FIR_DIAGNOSTICS", "LANGUAGE_FEATURE_TOGGLED", "JDK_KIND"} {
		if t.has(d) {
			return d
		}
	}
	if strings.HasPrefix(t.path, "tests/multiplatform/") {
		return "multiplatform"
	}
	// Directories whose test classes give kotlinc options.
	for _, d := range []struct{ prefix, reason string }{
		{"tests/crv", "return value checker (crv/, crvFull/)"},
		{"tests/testsWithExplicitApi/", "explicit API mode"},
		{"tests/testsWithExplicitReturnTypes/", "explicit API mode"},
		{"tests/headerMode/", "header mode"},
		{"tests/strictEquals/", "strict equality"},
	} {
		if strings.HasPrefix(t.path, d.prefix) {
			return d.reason
		}
	}
	return ""
}

// Severities of diagnostics. A deprecation is a warning or an error
// depending on the language version; the markup names it NAME_ERROR or
// NAME_WARNING.
const (
	sevError       = "error"
	sevWarning     = "warning"
	sevDeprecation = "deprecation"
)

var (
	factoryRE  = regexp.MustCompile(`KtDiagnosticFactory(ForDeprecation)?\d*\(\s*"([A-Z0-9_]+)",\s*(\w+)`)
	delegateRE = regexp.MustCompile(`val ([A-Z0-9_]+) by (error|warning|deprecationError|info)\d*`)
)

// readSeverities reads the diagnostics' severities from their
// declarations in the files of diagDefs.
func readSeverities(kotlin string) (map[string]string, error) {
	sev := map[string]string{}
	for _, def := range diagDefs {
		src, err := os.ReadFile(filepath.Join(kotlin, def))
		if err != nil {
			return nil, err
		}
		for _, m := range factoryRE.FindAllStringSubmatch(string(src), -1) {
			switch {
			case m[1] != "":
				// The markup names a deprecation by its severity.
				sev[m[2]] = sevDeprecation
				sev[m[2]+"_ERROR"] = sevError
				sev[m[2]+"_WARNING"] = sevWarning
			case m[3] == "ERROR":
				sev[m[2]] = sevError
			case m[3] == "WARNING":
				sev[m[2]] = sevWarning
			}
		}
		for _, m := range delegateRE.FindAllStringSubmatch(string(src), -1) {
			switch m[2] {
			case "error":
				sev[m[1]] = sevError
			case "warning":
				sev[m[1]] = sevWarning
			case "deprecationError":
				sev[m[1]] = sevDeprecation
				sev[m[1]+"_ERROR"] = sevError
				sev[m[1]+"_WARNING"] = sevWarning
			}
		}
	}
	return sev, nil
}

// diagConfig is the diagnostics mode's configuration.
type diagConfig struct {
	kotlin string // the checkout
	suite  string // compiler/testData/diagnostics
	work   string
	jobs   int
	keep   bool
	comp   *compiler
	sev    map[string]string
	// strayErrs is the file to write kc's errors on lines without an
	// expected diagnostic to, or "".
	strayErrs string
	// marks is the file to write each expected diagnostic to, with
	// whether kc hit its line, or "".
	marks string
	// strayWarns is the file to write kc's warnings on lines that don't
	// expect them to, or "".
	strayWarns string
	// missed are the expected errors kc didn't report on their lines,
	// as missed-errors TSV lines (missedLines), once the tests have run.
	missed []string
}

// missedLines returns the expected errors of the tests kc checked that
// it didn't report on their lines, as TSV lines: the test's path, file:line,
// the diagnostic's name and the cause (missCause), sorted.
func missedLines(rs []*result, dts []*diagTest) []string {
	var ls []string
	for i, dt := range dts {
		if dt == nil || !marksChecked(rs[i].category) {
			continue
		}
		for _, e := range dt.errors {
			if !dt.hit(e) {
				ls = append(ls, fmt.Sprintf("%s\t%s:%d\t%s\t%s", rs[i].path, e.file, e.line, e.name, dt.missCause(rs[i].category, e)))
			}
		}
	}
	slices.Sort(ls)
	return ls
}

// marksChecked reports whether a test in category has its marks checked
// against kc's errors: whether kc type-checked it.
func marksChecked(category string) bool {
	switch category {
	case catRejectsInvalid, catAcceptsInvalid, catAcceptsValid, catRejectsValid:
		return true
	}
	return false
}

// missedHeader is the first line of a missed-errors TSV file.
const missedHeader = "path\tfile:line\tname\tcause"

// compareMissed prints the expected errors kc reports on their lines in
// the baseline but misses now, and how many it reports now that it
// missed. The baseline is a missed-errors TSV file, read from path, and
// cats are the baseline's categories of the tests (its results TSV); a
// test whose marks weren't checked then, or aren't now, is left out. It
// returns an error if an error is missed that wasn't.
func compareMissed(path string, data []byte, cats map[string]string, rs []*result, missed []string) error {
	key := func(line string) (string, string) {
		f := strings.SplitN(line, "\t", 4)
		if len(f) < 3 {
			return "", ""
		}
		return f[0], f[0] + "\t" + f[1] + "\t" + f[2]
	}
	old := map[string]int{}
	for _, line := range strings.Split(string(data), "\n")[1:] {
		if _, k := key(line); k != "" {
			old[k]++
		}
	}
	now := map[string]int{}
	for _, line := range missed {
		_, k := key(line)
		now[k]++
	}
	checked := map[string]bool{}
	for _, r := range rs {
		if marksChecked(r.category) && marksChecked(cats[r.path]) {
			checked[r.path] = true
		}
	}
	var worse []string
	better := 0
	for k, n := range now {
		p, _ := key(k)
		if checked[p] && n > old[k] {
			worse = append(worse, strings.ReplaceAll(k, "\t", " "))
		}
	}
	for k, n := range old {
		p, _ := key(k)
		if checked[p] && n > now[k] {
			better += n - now[k]
		}
	}
	slices.Sort(worse)
	fmt.Printf("missed errors %s: %d newly missed, %d newly reported\n", path, len(worse), better)
	for _, s := range worse {
		fmt.Println("  newly missed:", s)
	}
	if len(worse) > 0 {
		return fmt.Errorf("%d expected errors are no longer reported on their lines (see %s)", len(worse), path)
	}
	return nil
}

// mark is an expected diagnostic of a test, in a file.
type mark struct {
	file string
	expected
}

// A diagTest is a test's expectations, and what kc did.
type diagTest struct {
	rejects  bool   // kotlinc rejects it
	pipeline string // its RUN_PIPELINE_TILL
	marks    []mark // its expected diagnostics
	errors   []mark // those that are errors
	kc       []diag // the errors kc reported, unless it crashed or found an unsupported feature
	// kcWarns are the warnings kc reported, and off the names of the
	// diagnostics the test's DIAGNOSTICS directive switches off, which
	// it doesn't mark. Warnings don't decide the test's category.
	kcWarns []diag
	off     map[string]bool
	// parseErrs is whether kc's parser rejects the test's code, and tried
	// the files of the modules kc compiled or tried to: what missCause
	// needs.
	parseErrs bool
	tried     map[string]bool
}

// The causes of an expected error that kc doesn't report on its line.
const (
	causeAccepted     = "accepted"            // kc compiled the test
	causeParse        = "syntax-error"        // kc's parser rejected it, and checked only what it recovered
	causeModule       = "module-not-compiled" // it is in a module after one that failed
	causeNearby       = "nearby"              // kc reported an error within cascadeLines of it, on a line without a mark
	causeMissingCheck = "missing-check"       // none of those
	cascadeLines      = 3
)

// causes are the causes in the order the report lists them.
var causes = []string{causeAccepted, causeParse, causeModule, causeNearby, causeMissingCheck}

// missCause returns why kc didn't report the expected error e on its
// line, given the test's category: the first of the causes that applies.
func (dt *diagTest) missCause(category string, e mark) string {
	switch {
	case category == catAcceptsInvalid:
		return causeAccepted
	case dt.parseErrs:
		return causeParse
	case dt.tried != nil && !dt.tried[e.file]:
		return causeModule
	}
	// An error on a line the markup expects another one on is most
	// likely that one.
	for _, d := range dt.stray() {
		if d.file == e.file && d.line >= e.line-cascadeLines && d.line <= e.line+cascadeLines {
			return causeNearby
		}
	}
	return causeMissingCheck
}

// runDiagnostics runs the tests and writes the report.
func runDiagnostics(c *diagConfig, tests []*test, tag string) ([]*result, string, error) {
	sev, err := readSeverities(c.kotlin)
	if err != nil {
		return nil, "", err
	}
	c.sev = sev
	results := make([]*result, len(tests))
	dts := make([]*diagTest, len(tests))
	var wg sync.WaitGroup
	var done atomic.Int64
	next := make(chan int)
	for range c.jobs {
		wg.Go(func() {
			for i := range next {
				results[i], dts[i] = c.runOne(i, tests[i])
				progress(done.Add(1), len(tests))
			}
		})
	}
	for i := range tests {
		next <- i
	}
	close(next)
	wg.Wait()
	if c.strayErrs != "" {
		if err := writeStray(c.strayErrs, results, dts); err != nil {
			return nil, "", err
		}
	}
	if c.marks != "" {
		if err := writeMarks(c.marks, c.sev, results, dts); err != nil {
			return nil, "", err
		}
	}
	if c.strayWarns != "" {
		if err := writeStrayWarnings(c.strayWarns, results, dts); err != nil {
			return nil, "", err
		}
	}
	c.missed = missedLines(results, dts)
	return results, reportDiagnostics(c, results, dts, tag), nil
}

// writeMarks writes each expected diagnostic of the tests kc compiled as
// TSV: the test's path, its category, file:line, the diagnostic's name,
// its severity, and hit if kc reported an error on its line, or miss,
// followed for a missed error by its cause (missCause).
func writeMarks(name string, sev map[string]string, rs []*result, dts []*diagTest) error {
	var sb strings.Builder
	for i, dt := range dts {
		if dt == nil || rs[i].category == catUnsupported || rs[i].category == catCrash {
			continue
		}
		for _, m := range dt.marks {
			hit := "miss"
			if sev[m.name] == sevWarning && dt.warnHit(m) || sev[m.name] != sevWarning && dt.hit(m) {
				hit = "hit"
			}
			s := sev[m.name]
			if s == "" {
				s = "-"
			}
			if hit == "miss" && s == sevError {
				hit += "\t" + dt.missCause(rs[i].category, m)
			}
			fmt.Fprintf(&sb, "%s\t%s\t%s:%d\t%s\t%s\t%s\n", rs[i].path, rs[i].category, m.file, m.line, m.name, s, hit)
		}
	}
	return os.WriteFile(name, []byte(sb.String()), 0o644)
}

// writeStray writes kc's errors on lines without an expected diagnostic
// as TSV: the test's path, its category, file:line and the message.
func writeStray(name string, rs []*result, dts []*diagTest) error {
	var sb strings.Builder
	for i, dt := range dts {
		if dt == nil {
			continue
		}
		for _, d := range dt.stray() {
			msg := strings.NewReplacer("\t", " ", "\n", " ").Replace(d.msg)
			fmt.Fprintf(&sb, "%s\t%s\t%s:%d\t%s\n", rs[i].path, rs[i].category, d.file, d.line, msg)
		}
	}
	return os.WriteFile(name, []byte(sb.String()), 0o644)
}

// stray returns kc's errors on lines without an expected diagnostic.
func (dt *diagTest) stray() []diag {
	var ds []diag
	for _, d := range dt.kc {
		if !slices.ContainsFunc(dt.marks, func(m mark) bool { return m.file == d.file && m.line == d.line }) {
			ds = append(ds, d)
		}
	}
	return ds
}

func (c *diagConfig) runOne(i int, t *test) (*result, *diagTest) {
	r := &result{path: t.path}
	if reason := diagSkip(t); reason != "" {
		r.category, r.key = catSkip, reason
		return r, nil
	}
	dt := &diagTest{}
	if vs := t.values("RUN_PIPELINE_TILL"); len(vs) > 0 {
		dt.pipeline = vs[0]
	}
	stdlib := t.has("WITH_STDLIB") || strings.HasPrefix(t.path, "testsWithStdLib/")
	for _, f := range t.files() {
		if !stdlib && strings.HasSuffix(f.name, ".kt") {
			f.text = nonNullAnyCalls(f.text)
		}
		var es []expected
		f.text, es = stripMarkup(f.text)
		for _, e := range es {
			m := mark{f.name, e}
			dt.marks = append(dt.marks, m)
			if c.sev[e.name] == sevError {
				dt.errors = append(dt.errors, m)
			}
		}
	}
	// A test runs to the backend when its errors are the JVM backend's,
	// so it is invalid if it stops earlier or marks an error. The
	// CHECK_TYPE helpers declare names kotlinc reports as errors (_), and
	// a test that adds them stops in the frontend: only its marks count.
	dt.rejects = len(dt.errors) > 0
	if !t.has("CHECK_TYPE") && !t.has("CHECK_TYPE_WITH_EXACT") && (dt.pipeline == "FRONTEND" || dt.pipeline == "FIR2IR") {
		dt.rejects = true
	}
	helpers, err := helperFiles(filepath.Join(c.suite, "helpers"), t)
	if err != nil {
		r.category, r.key, r.detail = catKc, "harness error", err.Error()
		return r, nil
	}
	// The unsupported features the parser finds, with recovery. A parse
	// error that isn't about a feature is kc rejecting the test.
	var parseErrs []diag
	if c.comp.kotlinc == "" {
		var features []diag
		for _, d := range parseFeatures(append(t.files(), helpers...)) {
			if d.feature != "" {
				features = append(features, d)
			} else {
				parseErrs = append(parseErrs, d)
			}
		}
		if len(features) == 0 && !dt.rejects && len(parseErrs) > 0 {
			// Valid code kc's parser doesn't parse.
			features = parseErrs
		}
		if len(features) > 0 {
			r.category, r.keys = catUnsupported, unsupportedKeys(features, true)
			r.key = r.keys[0]
			r.detail = fmt.Sprintf("%s:%d: %s", features[0].file, features[0].line, features[0].msg)
			return r, dt
		}
	}
	dir := filepath.Join(c.work, fmt.Sprint(i))
	if !c.keep {
		defer os.RemoveAll(dir)
	}
	libs := libraries{
		// Without WITH_STDLIB, the test infrastructure gives kotlinc a
		// minimal standard library, which isn't published: the built-in
		// declarations, run, let and a few more. kotlinc can't compile
		// without one, and kc lacks some of the built-ins (List,
		// KProperty) without one, so both get the standard library. A
		// test can then resolve what it expects to be unresolved; the
		// self-check counts those. Not Any?.hashCode() or toString(),
		// which nonNullAnyCalls takes out of the test's calls.
		stdlib:  true,
		reflect: t.has("WITH_REFLECT"),
		optIns:  t.values("OPT_IN"),
	}
	if strings.Contains(t.path, "/contracts/") {
		// As the contracts tests' test classes do.
		libs.optIns = append(libs.optIns, "kotlin.contracts.ExperimentalContracts")
	}
	cr, err := c.comp.compile(t, dir, helpers, libs)
	if err != nil {
		r.category, r.key, r.detail = catKc, "harness error", err.Error()
		return r, nil
	}
	dt.kcWarns, dt.off = cr.warns, switchedOff(t, dt.marks)
	dt.parseErrs, dt.tried = len(parseErrs) > 0, cr.tried
	rejected := false
	switch {
	case cr.crash != "":
		r.category, r.key = catCrash, cr.crash
		return r, dt
	case strings.Contains(cr.other, "runtime error: "):
		r.category, r.key, r.detail = catCrash, otherKey(cr.other), cr.other
		return r, dt
	case c.comp.kotlinc != "":
		rejected = !cr.ok
		r.detail = cr.other
	case !cr.ok && cr.other == "":
		if keys := unsupportedKeys(cr.diags, false); len(keys) > 0 {
			r.category, r.keys, r.key = catUnsupported, keys, keys[0]
			return r, dt
		}
		dt.kc = dropOffBackendErrors(t, cr.diags)
		rejected = len(dt.kc) > 0
	}
	// A code generation error isn't a diagnostic: kc's checker accepted
	// the test.
	switch {
	case dt.rejects && rejected:
		r.category, r.key = catRejectsInvalid, "on an expected line"
		if !dt.hitAny() && c.comp.kotlinc == "" {
			r.key = "elsewhere"
		}
	case !dt.rejects && !rejected:
		r.category = catAcceptsValid
	case dt.rejects:
		r.category = catAcceptsInvalid
		r.keys = dt.errorNames()
		if len(r.keys) == 0 {
			// Its errors are deprecations, which are errors in this
			// language version, or unmarked.
			for _, m := range dt.marks {
				if c.sev[m.name] == sevDeprecation && !slices.Contains(r.keys, m.name) {
					r.keys = append(r.keys, m.name)
				}
			}
		}
		if len(r.keys) == 0 {
			r.keys = []string{"(no error marked)"}
		}
		r.key = r.keys[0]
		if len(dt.errors) > 0 {
			e := dt.errors[0]
			r.detail = fmt.Sprintf("%s:%d: %s", e.file, e.line, e.name)
		}
	default:
		r.category = catRejectsValid
		if c.comp.kotlinc != "" {
			r.key = template(r.detail)
			break
		}
		d := dt.kc[0]
		r.key = template(d.msg)
		r.detail = fmt.Sprintf("%s:%d: %s", d.file, d.line, d.msg)
	}
	return r, dt
}

// backendMessages are the beginnings of kc's messages for the JVM
// backend's errors, which kc reports in its checker.
var backendMessages = map[string]string{
	"CONFLICTING_JVM_DECLARATIONS":           "platform declaration clash",
	"CONFLICTING_INHERITED_JVM_DECLARATIONS": "inherited platform declarations clash",
	"ACCIDENTAL_OVERRIDE":                    "accidental override",
}

// dropOffBackendErrors returns kc's errors without those of the JVM
// backend that the test's DIAGNOSTICS directive switches off. The test
// infrastructure doesn't report them, though kotlinc does.
func dropOffBackendErrors(t *test, ds []diag) []diag {
	var off []string
	for _, v := range t.values("DIAGNOSTICS") {
		if p, ok := backendMessages[strings.TrimPrefix(v, "-")]; ok && strings.HasPrefix(v, "-") {
			off = append(off, p)
		}
	}
	if len(off) == 0 {
		return ds
	}
	return slices.DeleteFunc(slices.Clone(ds), func(d diag) bool {
		return slices.ContainsFunc(off, func(p string) bool { return strings.HasPrefix(d.msg, p) })
	})
}

// hit reports whether kc reported an error on the line of e.
func (dt *diagTest) hit(e mark) bool {
	for _, d := range dt.kc {
		if d.file == e.file && d.line == e.line {
			return true
		}
	}
	return false
}

// hitAny reports whether kc reported an error on the line of any
// expected error, or of any expected diagnostic if none is an error.
func (dt *diagTest) hitAny() bool {
	ms := dt.errors
	if len(ms) == 0 {
		ms = dt.marks
	}
	return slices.ContainsFunc(ms, dt.hit)
}

// errorNames returns the names of the test's expected errors, without
// duplicates.
func (dt *diagTest) errorNames() []string {
	var names []string
	for _, e := range dt.errors {
		if !slices.Contains(names, e.name) {
			names = append(names, e.name)
		}
	}
	return names
}

// reportDiagnostics writes the diagnostics mode's report.
func reportDiagnostics(c *diagConfig, rs []*result, dts []*diagTest, tag string) string {
	var sb strings.Builder
	cats := byCategory(rs)
	run := len(rs) - cats[catSkip]
	compilerName := "kc"
	if c.comp.kotlinc != "" {
		compilerName = "kotlinc (the harness's self-check)"
	}
	fmt.Fprintf(&sb, "# Kotlin's diagnostics tests on %s\n\n", compilerName)
	fmt.Fprintf(&sb, "The diagnostics tests of JetBrains/kotlin %s for the JVM (`compiler/testData/diagnostics/`: %s), each compiled by %s. A test's markup marks the diagnostics kotlinc reports, and its `RUN_PIPELINE_TILL` says whether kotlinc stops with errors in the frontend (`FRONTEND`, `FIR2IR`) or goes on to the backend (`BACKEND`). `bench/kotlin-tests.sh diagnostics` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.\n\n",
		tag, strings.Join(diagSuites, ", "), compilerName)
	fmt.Fprintf(&sb, "Of %d test files, %d are skipped and %d run.\n\n", len(rs), cats[catSkip], run)
	fmt.Fprintf(&sb, "| Result | Tests | Share |\n|:---|---:|---:|\n")
	for _, cat := range []string{catRejectsInvalid, catAcceptsValid, catAcceptsInvalid, catRejectsValid, catUnsupported, catCrash} {
		fmt.Fprintf(&sb, "| %s | %d | %s |\n", cat, cats[cat], pct(cats[cat], run))
	}
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "*Rejects invalid* and *accepts valid* agree with kotlinc. *Accepts invalid* is code kotlinc rejects that kc compiles; *rejects valid*, code kotlinc compiles that kc's checker rejects. Both are bugs. A *parse/unsupported* test uses a feature kc doesn't support, which says nothing about its checks.\n\n")
	reject := 0
	for _, r := range rs {
		if r.category == catRejectsInvalid && r.key == "on an expected line" {
			reject++
		}
	}
	fmt.Fprintf(&sb, "Of the %d tests both reject, kc reports an error on the line of an expected one in %d.\n\n", cats[catRejectsInvalid], reject)

	// The expectations' consistency: the pipeline and the markup.
	frontendNoError, backendError, unknown := 0, 0, map[string]int{}
	for _, dt := range dts {
		if dt == nil {
			continue
		}
		switch {
		case dt.pipeline == "FRONTEND" && len(dt.errors) == 0:
			frontendNoError++
		case dt.pipeline == "BACKEND" && len(dt.errors) > 0:
			backendError++
		}
		for _, m := range dt.marks {
			if c.sev[m.name] == "" {
				unknown[m.name]++
			}
		}
	}
	fmt.Fprintf(&sb, "A test is invalid if it stops before the backend or marks an error: one that runs to it (`BACKEND`) can mark the JVM backend's errors, such as `CONFLICTING_JVM_DECLARATIONS`. Of the tests run, %d stop at `FRONTEND` but mark no error: most add the `CHECK_TYPE` helpers, whose names (`_`) kotlinc reports outside the test infrastructure, and count as valid; the others leave their errors out with the `DIAGNOSTICS` directive. %d run to the backend and mark one. %d marked names have no declared severity (debug information, for one), and count as neither.\n\n",
		frontendNoError, backendError, len(unknown))

	if len(unknown) > 0 {
		var names []string
		for n := range unknown {
			names = append(names, n)
		}
		slices.SortFunc(names, func(a, b string) int {
			if d := unknown[b] - unknown[a]; d != 0 {
				return d
			}
			return strings.Compare(a, b)
		})
		for i, n := range names {
			names[i] = fmt.Sprintf("`%s` (%d)", n, unknown[n])
		}
		fmt.Fprintf(&sb, "The names without a severity: %s.\n\n", strings.Join(names, ", "))
	}

	fmt.Fprintf(&sb, "## Skipped\n\nTests for other platforms, that need language features off by default or compiler options, or modules other than plain dependencies, or whose FIR diagnostics aren't checked.\n\n")
	reportKeys(&sb, rs, catSkip, "Reason", 30, false)

	fmt.Fprintf(&sb, "## Accepts invalid\n\nTests kotlinc rejects that kc compiles, by the errors their markup expects. A test counts for each; *Only error* counts the tests it is the only expected error of, which implementing it alone would fix.\n\n")
	reportKeysSole(&sb, rs, catAcceptsInvalid, "Expected error", 60)

	fmt.Fprintf(&sb, "## Rejects valid\n\nTests kotlinc compiles that kc's checker rejects, by kc's first error, with names replaced by _.\n\n")
	reportKeys(&sb, rs, catRejectsValid, "First error", 40, false)

	fmt.Fprintf(&sb, "## Errors by diagnostic\n\nFor each expected error of the tests kc checked (not unsupported), whether kc reported an error on its line. Sorted by the errors kc missed.\n\n")
	type recall struct {
		name           string
		expected, hits int
	}
	byName := map[string]*recall{}
	extra := map[string]int{}
	missed := map[string]map[string]int{} // by cause, by name
	for i, dt := range dts {
		if dt == nil || rs[i].category == catUnsupported || rs[i].category == catCrash || rs[i].category == catKc {
			continue
		}
		for _, e := range dt.errors {
			rc := byName[e.name]
			if rc == nil {
				rc = &recall{name: e.name}
				byName[e.name] = rc
			}
			rc.expected++
			if dt.hit(e) {
				rc.hits++
			} else {
				cause := dt.missCause(rs[i].category, e)
				if missed[cause] == nil {
					missed[cause] = map[string]int{}
				}
				missed[cause][e.name]++
			}
		}
		// kc's errors on lines without an expected diagnostic.
		for _, d := range dt.stray() {
			extra[template(d.msg)]++
		}
	}
	var rl []*recall
	totalExp, totalHit := 0, 0
	for _, rc := range byName {
		rl = append(rl, rc)
		totalExp += rc.expected
		totalHit += rc.hits
	}
	slices.SortFunc(rl, func(a, b *recall) int {
		if d := (b.expected - b.hits) - (a.expected - a.hits); d != 0 {
			return d
		}
		return strings.Compare(a.name, b.name)
	})
	fmt.Fprintf(&sb, "kc reports %d of %d expected errors (%s) on their lines.\n\n", totalHit, totalExp, pct(totalHit, totalExp))
	fmt.Fprintf(&sb, "| Diagnostic | Expected | Reported | Missed |\n|:---|---:|---:|---:|\n")
	for i, rc := range rl {
		if i == 60 {
			fmt.Fprintf(&sb, "| *%d more* | | | |\n", len(rl)-i)
			break
		}
		fmt.Fprintf(&sb, "| `%s` | %d | %d | %d |\n", rc.name, rc.expected, rc.hits, rc.expected-rc.hits)
	}
	fmt.Fprintln(&sb)
	reportMissCauses(&sb, missed, totalExp-totalHit)

	fmt.Fprintf(&sb, "## Errors kotlinc doesn't report\n\nkc's errors on lines where the markup expects no diagnostic, in the tests it checked, with names replaced by _. In a test kotlinc compiles, one is a false error; in one it rejects, it can also be the same error on another line, or one kotlinc doesn't report after a first.\n\n")
	type kc struct {
		key string
		n   int
	}
	var el []kc
	for k, n := range extra {
		el = append(el, kc{k, n})
	}
	slices.SortFunc(el, func(a, b kc) int {
		if a.n != b.n {
			return b.n - a.n
		}
		return strings.Compare(a.key, b.key)
	})
	fmt.Fprintf(&sb, "| Error | Count |\n|:---|---:|\n")
	for i, e := range el {
		if i == 30 {
			fmt.Fprintf(&sb, "| *%d more* | |\n", len(el)-i)
			break
		}
		fmt.Fprintf(&sb, "| %s | %d |\n", cell(e.key), e.n)
	}
	fmt.Fprintln(&sb)

	fmt.Fprintf(&sb, "## Unsupported features\n\nBy the features the parser (with error recovery) and the checker found. A test counts for each.\n\n")
	reportKeys(&sb, rs, catUnsupported, "Feature", 30, true)
	fmt.Fprintf(&sb, "### Greedy order\n\nAt each step, the feature that unblocks the most tests given the ones before it.\n\n")
	reportGreedy(&sb, rs, 20)

	if cats[catCrash] > 0 {
		fmt.Fprintf(&sb, "## Crashes\n\n")
		reportKeys(&sb, rs, catCrash, "Panic", 1000, false)
	}

	reportWarnings(&sb, c, rs, dts)
	return sb.String()
}

// reportMissCauses writes the expected errors kc missed by their cause
// (missCause), each with the diagnostics it misses most.
func reportMissCauses(sb *strings.Builder, missed map[string]map[string]int, total int) {
	fmt.Fprintf(sb, "### Missed errors by cause\n\n")
	fmt.Fprintf(sb, "Why kc reports no error on the line of an expected one: *%s*, it compiled the test; *%s*, its parser rejected the code, and kc checked it as recovered from kotlinc's grammar, without the statements and declarations it couldn't lower; *%s*, the error is in a module after one that failed, which isn't compiled; *%s*, kc reported an error within %d lines of it on a line the markup expects none on (most likely the error, at another position); *%s*, none of those.\n\n",
		causeAccepted, causeParse, causeModule, causeNearby, cascadeLines, causeMissingCheck)
	fmt.Fprintf(sb, "| Cause | Missed | Share | Most missed |\n|:---|---:|---:|:---|\n")
	for _, cause := range causes {
		byName := missed[cause]
		n := 0
		type named struct {
			name string
			n    int
		}
		var ns []named
		for name, k := range byName {
			n += k
			ns = append(ns, named{name, k})
		}
		slices.SortFunc(ns, func(a, b named) int {
			if a.n != b.n {
				return b.n - a.n
			}
			return strings.Compare(a.name, b.name)
		})
		var top []string
		for i, x := range ns {
			if i == 5 {
				break
			}
			top = append(top, fmt.Sprintf("`%s` %d", x.name, x.n))
		}
		fmt.Fprintf(sb, "| %s | %d | %s | %s |\n", cause, n, pct(n, total), strings.Join(top, ", "))
	}
	fmt.Fprintln(sb)
}

// reportKeysSole writes reportKeys's table with the Only column named
// for errors.
func reportKeysSole(sb *strings.Builder, rs []*result, category, keyName string, limit int) {
	var t strings.Builder
	reportKeys(&t, rs, category, keyName, limit, true)
	sb.WriteString(strings.Replace(t.String(), "| Only blocker |", "| Only error |", 1))
}
