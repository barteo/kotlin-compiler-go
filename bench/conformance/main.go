// Command conformance runs the Kotlin compiler's own tests against kc,
// and ranks the ways kc fails them.
//
//	go run ./bench/conformance -mode box -kotlin dir -tag v2.4.21 -kc bin/kc \
//	    -stdlib kotlin-stdlib.jar -test kotlin-test.jar -reflect kotlin-reflect.jar \
//	    -o bench/results/box.md -tsv bench/results/box.tsv
//
// dir is a checkout of JetBrains/kotlin (bench/kotlin-tests.sh makes a
// sparse one). The box mode compiles each test of
// compiler/testData/codegen/box with kc, split into its modules and
// files and with the helpers its directives add, as the test
// infrastructure does, and runs its box(), which must return "OK", in a
// fresh class loader of a long-lived JVM (BoxRunner.java). The
// diagnostics mode compiles the JVM tests of compiler/testData/diagnostics
// without their markup, and compares what kc rejects with what kotlinc
// does: its errors, and the lines of the errors the markup marks. Both
// sort the results into categories, rank the keys of each, and write one
// line per test to a TSV file, which -baseline compares a later run with.
//
// -kotlinc runs the tests (a -sample of them: it is slow) through the
// same steps with kotlinc instead of kc, which checks the harness
// itself: they must all pass.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

func main() {
	mode := flag.String("mode", "box", "box or diagnostics")
	kotlin := flag.String("kotlin", "", "checkout of JetBrains/kotlin")
	tag := flag.String("tag", "", "the checkout's tag, for the report")
	kc := flag.String("kc", "bin/kc", "the kc binary")
	kotlinc := flag.String("kotlinc", "", "compile with this kotlinc command instead of kc, to check the harness")
	stdlib := flag.String("stdlib", "", "kotlin-stdlib.jar")
	reflectJar := flag.String("reflect", "", "kotlin-reflect.jar")
	kotlinTest := flag.String("test", "", "kotlin-test.jar, which the box tests' kotlin.test functions are in")
	java := flag.String("java", "java", "the java command")
	javac := flag.String("javac", "javac", "the javac command")
	out := flag.String("o", "", "the Markdown report to write")
	tsv := flag.String("tsv", "", "the TSV file of results to write")
	baseline := flag.String("baseline", "", "a TSV file of an earlier run to compare with")
	filter := flag.String("filter", "", "only the tests whose path, relative to the suite, starts with this")
	sample := flag.Int("sample", 0, "run this many tests, chosen at random among those not skipped")
	seed := flag.Uint64("seed", 1, "the seed of -sample")
	jobs := flag.Int("j", runtime.NumCPU(), "tests compiled at a time")
	jvms := flag.Int("jvms", max(2, runtime.NumCPU()/2), "JVMs running tests")
	timeout := flag.Duration("timeout", 10*time.Second, "how long a test's box() may run")
	work := flag.String("work", filepath.Join(os.TempDir(), "kc-conformance"), "directory for the compiled tests")
	keep := flag.Bool("keep", false, "keep the compiled tests in -work")
	flag.Parse()
	if *kotlin == "" || *stdlib == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *kotlinc == "" {
		abs, err := filepath.Abs(*kc)
		if err != nil {
			log.Fatal(err)
		}
		*kc = abs
	}
	libs := []string{*stdlib}
	if *kotlinTest != "" {
		libs = append(libs, *kotlinTest)
	}
	comp := &compiler{kc: *kc, kotlinc: *kotlinc, libs: libs, reflect: *reflectJar, timeout: 2 * time.Minute}
	if err := os.RemoveAll(*work); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*work, 0o755); err != nil {
		log.Fatal(err)
	}
	start := time.Now()

	var suite string
	var skip func(*test) string
	switch *mode {
	case "box":
		suite = filepath.Join(*kotlin, "compiler/testData/codegen/box")
		skip = boxSkip
	case "diagnostics":
		suite = filepath.Join(*kotlin, "compiler/testData/diagnostics")
		skip = diagSkip
	default:
		log.Fatalf("unknown -mode %s", *mode)
	}
	tests, err := findTests(suite, *mode, *filter)
	if err != nil {
		log.Fatal(err)
	}
	if *sample > 0 {
		var run []*test
		for _, t := range tests {
			if skip(t) == "" {
				run = append(run, t)
			}
		}
		r := rand.New(rand.NewPCG(*seed, 0))
		r.Shuffle(len(run), func(i, j int) { run[i], run[j] = run[j], run[i] })
		tests = run[:min(*sample, len(run))]
		slices.SortFunc(tests, func(a, b *test) int { return strings.Compare(a.path, b.path) })
	}
	fmt.Fprintf(os.Stderr, "%d tests\n", len(tests))

	var rs []*result
	var report string
	good := func(cat string) bool { return cat == catPass }
	switch *mode {
	case "box":
		classes := filepath.Join(*work, "runner")
		src := filepath.Join(filepath.Dir(must(os.Executable())), "BoxRunner.java")
		if _, err := os.Stat(src); err != nil {
			src = "bench/conformance/BoxRunner.java"
		}
		if err := buildRunner(*javac, src, classes); err != nil {
			log.Fatal(err)
		}
		p := newPool(&runnerConfig{java: *java, classes: classes, libs: libs, reflect: *reflectJar, timeout: *timeout}, *jvms)
		c := &boxConfig{
			suite:   suite,
			helpers: filepath.Join(*kotlin, "compiler/testData/diagnostics/helpers"),
			work:    *work,
			jobs:    *jobs,
			keep:    *keep,
			comp:    comp,
			pool:    p,
		}
		rs = runBox(c, tests)
		p.close()
		report = reportBox(rs, *tag, *kotlinc != "")
	case "diagnostics":
		c := &diagConfig{kotlin: *kotlin, suite: suite, work: *work, jobs: *jobs, keep: *keep, comp: comp}
		if rs, report, err = runDiagnostics(c, tests, *tag); err != nil {
			log.Fatal(err)
		}
		good = func(cat string) bool { return cat == catRejectsInvalid || cat == catAcceptsValid }
	}
	elapsed := time.Since(start).Round(time.Second)
	fmt.Fprintf(os.Stderr, "%d tests in %v\n", len(rs), elapsed)
	for cat, n := range byCategory(rs) {
		fmt.Fprintf(os.Stderr, "  %-22s %d\n", cat, n)
	}
	if *out != "" {
		if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	if *tsv != "" {
		if err := writeTSV(*tsv, rs); err != nil {
			log.Fatal(err)
		}
	}
	if *baseline != "" {
		if err := compareBaseline(*baseline, rs, good); err != nil {
			log.Fatal(err)
		}
	}
}

// findTests reads the suite's test files under filter.
func findTests(suite, mode, filter string) ([]*test, error) {
	var tests []*test
	err := filepath.WalkDir(suite, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(suite, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() || !strings.HasSuffix(p, ".kt") || !strings.HasPrefix(rel, filter) {
			return nil
		}
		if mode == "diagnostics" && !isDiagTest(rel) {
			return nil
		}
		t, err := readTest(p, rel)
		if err != nil {
			return err
		}
		tests = append(tests, t)
		return nil
	})
	return tests, err
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
}

// reportBox writes the box mode's report.
func reportBox(rs []*result, tag string, kotlinc bool) string {
	var sb strings.Builder
	cats := byCategory(rs)
	run := len(rs) - cats[catSkip]
	compilerName := "kc"
	if kotlinc {
		compilerName = "kotlinc (the harness's self-check)"
	}
	fmt.Fprintf(&sb, "# Kotlin's box tests on %s\n\n", compilerName)
	fmt.Fprintf(&sb, "The box tests of JetBrains/kotlin %s (`compiler/testData/codegen/box`), each compiled by %s and run: its `box()` must return `\"OK\"`. `bench/kotlin-tests.sh box` writes this file, and [docs/conformance.md](../../docs/conformance.md) explains it.\n\n", tag, compilerName)
	fmt.Fprintf(&sb, "Of %d test files, %d are skipped and %d run. %d pass (%s).\n\n", len(rs), cats[catSkip], run, cats[catPass], pct(cats[catPass], run))
	fmt.Fprintf(&sb, "| Result | Tests | Share |\n|:---|---:|---:|\n")
	order := []string{catPass, catUnsupported, catCheck, catCrash, catKc, catVerify, catLinkage, catException, catWrong, catTimeout}
	for _, c := range order {
		fmt.Fprintf(&sb, "| %s | %d | %s |\n", c, cats[c], pct(cats[c], run))
	}
	fmt.Fprintln(&sb)

	fmt.Fprintf(&sb, "## Skipped\n\nTests for other backends, that kotlinc's JVM backend ignores, or that need what the harness doesn't do: language features off by default, compiler options, Java sources, or modules other than plain dependencies.\n\n")
	reportKeys(&sb, rs, catSkip, "Reason", 30, false)

	fmt.Fprintf(&sb, "## By directory\n\n*Bad bytecode* is VerifyError and linkage errors; *wrong* is a wrong result, an exception or a timeout.\n\n")
	type dirCount struct {
		dir string
		m   map[string]int
		run int
	}
	dirs := map[string]*dirCount{}
	for _, r := range rs {
		if r.category == catSkip {
			continue
		}
		d := dirs[topDir(r.path)]
		if d == nil {
			d = &dirCount{dir: topDir(r.path), m: map[string]int{}}
			dirs[d.dir] = d
		}
		d.m[r.category]++
		d.run++
	}
	var dl []*dirCount
	for _, d := range dirs {
		dl = append(dl, d)
	}
	slices.SortFunc(dl, func(a, b *dirCount) int {
		if a.run != b.run {
			return b.run - a.run
		}
		return strings.Compare(a.dir, b.dir)
	})
	fmt.Fprintf(&sb, "| Directory | Run | Pass | Unsupported | Check error | Crash | Bad bytecode | Wrong |\n|:---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, d := range dl {
		fmt.Fprintf(&sb, "| %s | %d | %d | %d | %d | %d | %d | %d |\n", d.dir, d.run, d.m[catPass], d.m[catUnsupported], d.m[catCheck],
			d.m[catCrash]+d.m[catKc], d.m[catVerify]+d.m[catLinkage], d.m[catWrong]+d.m[catException]+d.m[catTimeout])
	}
	fmt.Fprintln(&sb)

	sections := []struct{ cat, title, key, intro string }{
		{catUnsupported, "Unsupported features", "Feature", "Tests that use a feature kc doesn't support, or syntax its parser doesn't, by the features the parser (with error recovery) and the checker found. A test counts for each; *Only blocker* counts the tests it is the only one found in."},
		{catCheck, "Check errors", "First error", "Tests kotlinc compiles that kc's checker rejects, by its first error, with names replaced by _."},
		{catCrash, "Crashes", "Panic", "kc panicked, ran out of stack or timed out, or code generation recovered from a panic. The panics name kc's innermost function."},
		{catKc, "Code generation errors", "Error", "kc checked the test but failed to generate its code."},
		{catVerify, "VerifyError", "Error", "The JVM rejected a class kc wrote."},
		{catLinkage, "Linkage errors", "Error", "A class kc wrote refers to a member or class that doesn't exist, or doesn't fit."},
		{catException, "Exceptions", "Exception", "box() threw."},
		{catWrong, "Wrong results", "Result", "box() returned something other than `\"OK\"`, with numbers replaced by n."},
		{catTimeout, "Timeouts", "", ""},
	}
	for _, s := range sections {
		if cats[s.cat] == 0 {
			continue
		}
		fmt.Fprintf(&sb, "## %s\n\n", s.title)
		if s.intro != "" {
			fmt.Fprintf(&sb, "%s\n\n", s.intro)
		}
		limit := 40
		if s.cat == catCrash {
			limit = 1000
		}
		reportKeys(&sb, rs, s.cat, orDefault(s.key, "Key"), limit, s.cat == catUnsupported)
		if s.cat == catUnsupported {
			fmt.Fprintf(&sb, "### Greedy order\n\nAt each step, the feature that unblocks the most tests given the ones before it. A test is unblocked when every feature found in it is; it can still fail later.\n\n")
			reportGreedy(&sb, rs, 30)
		}
	}
	return sb.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
