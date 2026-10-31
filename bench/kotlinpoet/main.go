// Command kotlinpoet writes bench/results/tests-kotlinpoet.md from the runs
// of bench/kotlinpoet.sh: two runs of kotlinpoet's Gradle build, one with
// kotlinc's build of the library (the control) and one where kc compiles
// it and kotlinc compiles everything else against kc's classes, and the
// control build's tests run on kotlinc's classes and then on kc's.
//
//	go run ./bench/kotlinpoet -control dir -kc dir -control-tsv kotlinc.tsv -kc-tsv kc.tsv -o tests-kotlinpoet.md
//
// Each directory is a copy of kotlinpoet's sources that Gradle built, with
// its log in build.log; the report lists each module's tests from Gradle's
// JUnit XML results and each task's outcome from the log. Each .tsv file
// has a line per test, as RunTests.java writes it: class, method, PASS,
// FAIL or SKIP, and a failure's exception and message. It exits with
// status 1 if a task or a test that succeeds in the control run doesn't
// with kc's classes.
package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// modules are the test results the report reads: Gradle module and the
// directory of its JUnit XML results.
var modules = []struct{ name, dir string }{
	{":kotlinpoet", "kotlinpoet/build/test-results/jvmTest"},
	{":interop:kotlin-metadata", "interop/kotlin-metadata/build/test-results/test"},
	{":interop:ksp:test-processor", "interop/ksp/test-processor/build/test-results/test"},
}

// tasks are the tasks whose outcomes the report lists.
var tasks = []string{
	":kotlinpoet:compileKotlinJvm", ":kotlinpoet:compileTestKotlinJvm", ":kotlinpoet:compileJvmTestJava",
	":kotlinpoet:jvmTest", ":kotlinpoet:jvmApiCheck",
	":interop:kotlin-metadata:compileKotlin", ":interop:kotlin-metadata:compileTestKotlin", ":interop:kotlin-metadata:test",
	":interop:ksp:compileKotlin",
	":interop:ksp:test-processor:compileKotlin", ":interop:ksp:test-processor:compileTestKotlin", ":interop:ksp:test-processor:test",
}

// pkg is the prefix left out of test class names.
const pkg = "com.squareup.kotlinpoet."

type result struct {
	class, test, status, exc, msg string
}

// hash is the identity hash code in the names of parameterized tests,
// which differs between runs: [com.squareup.kotlinpoet.FunSpec$Builder@7aba206b].
var hash = regexp.MustCompile(`@[0-9a-f]+\]`)

type suite struct {
	Cases []struct {
		Class   string    `xml:"classname,attr"`
		Name    string    `xml:"name,attr"`
		Failure *struct{} `xml:"failure"`
		Error   *struct{} `xml:"error"`
		Skipped *struct{} `xml:"skipped"`
	} `xml:"testcase"`
}

// xmlResults returns the tests of Gradle's JUnit XML results in dir, by
// name.
func xmlResults(dir string) map[string]result {
	out := map[string]result{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		var s suite
		if err := xml.Unmarshal(data, &s); err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		for _, c := range s.Cases {
			status := "PASS"
			switch {
			case c.Failure != nil || c.Error != nil:
				status = "FAIL"
			case c.Skipped != nil:
				status = "SKIP"
			}
			r := result{class: c.Class, test: hash.ReplaceAllString(c.Name, "]"), status: status}
			out[r.class+"."+r.test] = r
		}
	}
	return out
}

// tsvResults returns the tests of a file RunTests.java wrote, by name.
func tsvResults(path string) map[string]result {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	out := map[string]result{}
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		fs := strings.Split(sc.Text(), "\t")
		for len(fs) < 5 {
			fs = append(fs, "")
		}
		r := result{fs[0], hash.ReplaceAllString(fs[1], "]"), fs[2], fs[3], fs[4]}
		out[r.class+"."+r.test] = r
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
	return out
}

var taskLine = regexp.MustCompile(`^> Task (\S+)(?: (.*))?$`)

// outcomes returns each task's outcome in a Gradle log: "ran", or what
// Gradle says (FAILED, UP-TO-DATE, NO-SOURCE, SKIPPED).
func outcomes(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	out := map[string]string{}
	for l := range strings.SplitSeq(string(data), "\n") {
		if m := taskLine.FindStringSubmatch(l); m != nil {
			o := m[2]
			if o == "" {
				o = "ran"
			}
			out[m[1]] = o
		}
	}
	return out
}

func count(rs map[string]result, status string) int {
	n := 0
	for _, r := range rs {
		if r.status == status {
			n++
		}
	}
	return n
}

// regressions returns the tests that pass in ctl but not in got,
// including those that got lacks.
func regressions(ctl, got map[string]result) []string {
	var out []string
	for k, r := range ctl {
		if g, ok := got[k]; r.status == "PASS" && (!ok || g.status != "PASS") {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func main() {
	control := flag.String("control", "", "the control build's directory")
	kc := flag.String("kc", "", "the directory of the build with kc's library")
	controlTSV := flag.String("control-tsv", "", "the control build's tests on kotlinc's classes")
	kcTSV := flag.String("kc-tsv", "", "the control build's tests on kc's classes")
	header := flag.String("header", "", "a line about the versions, for the report")
	out := flag.String("o", "", "the report to write")
	flag.Parse()
	if *control == "" || *kc == "" || *controlTSV == "" || *kcTSV == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# kotlinpoet on kc's library\n\n")
	fmt.Fprintf(&b, "Written by `bench/kotlinpoet.sh`. %s\n\n", *header)
	bad := false

	// The Gradle builds.
	fmt.Fprintf(&b, "## kotlinpoet's Gradle build\n\n")
	fmt.Fprintf(&b, "In the kc build, kc compiles `:kotlinpoet`'s JVM library, and kotlinc compiles its tests ")
	fmt.Fprintf(&b, "and the interop modules against kc's classes, reading their Kotlin metadata.\n\n")
	co, ko := outcomes(filepath.Join(*control, "build.log")), outcomes(filepath.Join(*kc, "build.log"))
	fmt.Fprintf(&b, "| Task | kotlinc (control) | kc |\n|:---|:---|:---|\n")
	for _, t := range tasks {
		c, k := co[t], ko[t]
		if c == "" && k == "" {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", t, c, k)
		if k == "FAILED" && c != "FAILED" {
			bad = true
		}
	}
	fmt.Fprintf(&b, "\n| Tests | Build | Passed | Failed | Skipped |\n|:---|:---|---:|---:|---:|\n")
	var gradleRegressions []string
	for _, m := range modules {
		cr, kr := xmlResults(filepath.Join(*control, m.dir)), xmlResults(filepath.Join(*kc, m.dir))
		for _, r := range []struct {
			name string
			rs   map[string]result
		}{{"kotlinc (control)", cr}, {"kc", kr}} {
			fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %d |\n", m.name, r.name, count(r.rs, "PASS"), count(r.rs, "FAIL"), count(r.rs, "SKIP"))
		}
		for _, k := range regressions(cr, kr) {
			gradleRegressions = append(gradleRegressions, k+": "+kr[k].status)
		}
	}
	if len(gradleRegressions) > 0 {
		bad = true
		fmt.Fprintf(&b, "\nTests that pass in the control build only:\n\n")
		for _, r := range gradleRegressions {
			fmt.Fprintf(&b, "- `%s`\n", r)
		}
	}

	// The control build's tests on each build's classes.
	ctl, got := tsvResults(*controlTSV), tsvResults(*kcTSV)
	fmt.Fprintf(&b, "\n## kotlinc's tests on kc's classes\n\n")
	fmt.Fprintf(&b, "The control build's JVM tests, compiled by kotlinc against kotlinc's build of the library, run twice with JUnit 4 and `java -Xverify:all`: on kotlinc's library classes, and with kc's in their place. This needs no metadata from kc, only binary compatibility.\n\n")
	b.WriteString("| Library classes | Passed | Failed | Skipped |\n|:---|---:|---:|---:|\n")
	fmt.Fprintf(&b, "| kotlinc (control) | %d | %d | %d |\n", count(ctl, "PASS"), count(ctl, "FAIL"), count(ctl, "SKIP"))
	fmt.Fprintf(&b, "| kc | %d | %d | %d |\n\n", count(got, "PASS"), count(got, "FAIL"), count(got, "SKIP"))
	regs := regressions(ctl, got)
	if len(regs) == 0 {
		b.WriteString("Every test that passes on kotlinc's classes passes on kc's.\n")
	} else {
		bad = true
		fmt.Fprintf(&b, "%d tests pass on kotlinc's classes but not on kc's:\n\n", len(regs))
		for _, k := range regs {
			g := got[k]
			fmt.Fprintf(&b, "- `%s`: %s %s\n", k, g.exc, strings.TrimSpace(g.msg))
		}
	}

	// The failures of each run, grouped by exception.
	for _, run := range []struct {
		name string
		rs   map[string]result
	}{{"kotlinc's", ctl}, {"kc's", got}} {
		groups := map[string][]string{}
		for k, r := range run.rs {
			if r.status == "FAIL" {
				groups[r.exc] = append(groups[r.exc], k)
			}
		}
		if len(groups) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### Failures on %s classes\n\n", run.name)
		var excs []string
		for e := range groups {
			excs = append(excs, e)
		}
		sort.Strings(excs)
		for _, e := range excs {
			ks := groups[e]
			sort.Strings(ks)
			fmt.Fprintf(&b, "- `%s` (%d): %s\n", e, len(ks), strings.Join(ks, ", "))
		}
	}

	// Test classes.
	type tally struct{ pass, fail, skip int }
	classes := map[string]*[2]tally{}
	for i, rs := range []map[string]result{ctl, got} {
		for _, r := range rs {
			t := classes[r.class]
			if t == nil {
				t = &[2]tally{}
				classes[r.class] = t
			}
			switch r.status {
			case "PASS":
				t[i].pass++
			case "FAIL":
				t[i].fail++
			case "SKIP":
				t[i].skip++
			}
		}
	}
	var names []string
	for n := range classes {
		names = append(names, n)
	}
	sort.Strings(names)
	b.WriteString("\n### Test classes\n\nPassed, failed and skipped tests.\n\n| Class | kotlinc | kc |\n|:---|:---|:---|\n")
	for _, n := range names {
		t := classes[n]
		fmt.Fprintf(&b, "| `%s` | %d / %d / %d | %d / %d / %d |\n", strings.TrimPrefix(n, pkg),
			t[0].pass, t[0].fail, t[0].skip, t[1].pass, t[1].fail, t[1].skip)
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	if bad {
		os.Exit(1)
	}
}
