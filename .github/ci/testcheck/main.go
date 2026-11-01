// Command testcheck reads the output of go test -json on standard input,
// prints what go test would print without -v, and exits with status 1 if
// a test failed, a package didn't build, a -run pattern matched no tests,
// or a test was skipped for a reason that no -allow pattern matches.
//
// Many tests skip themselves when java, kotlinc, kotlin-stdlib.jar or a
// library's jars aren't installed. On a developer's machine that is
// convenient; in CI it would let a misconfigured runner pass while
// testing almost nothing, so CI lists the skips it expects and fails on
// the others.
//
//	go test -json ./... | go run .github/ci/testcheck/main.go -allow 'needs the standard library'
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// event is one line of go test -json output (go doc test2json).
type event struct {
	Action  string
	Package string
	Test    string
	Output  string
}

// logLine matches a line that t.Log, t.Skip or t.Error writes, and
// captures its message.
var logLine = regexp.MustCompile(`^\s+[\w.-]+\.go:\d+: (.*)$`)

func main() {
	var allow []*regexp.Regexp
	flag.Func("allow", "a `regexp` matching the reason of an expected skip (repeatable)", func(s string) error {
		re, err := regexp.Compile(s)
		allow = append(allow, re)
		return err
	})
	flag.Parse()

	type key struct{ pkg, test string }
	output := map[key]*strings.Builder{}
	buf := func(k key) *strings.Builder {
		b := output[k]
		if b == nil {
			b = new(strings.Builder)
			output[k] = b
		}
		return b
	}
	var failed, unexpected, problems []string
	reasons := map[string]int{}
	passed, skipped := 0, 0

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for in.Scan() {
		var e event
		if err := json.Unmarshal(in.Bytes(), &e); err != nil || e.Action == "" {
			// Not an event: a message from go itself.
			fmt.Println(in.Text())
			continue
		}
		k := key{e.Package, e.Test}
		switch e.Action {
		case "build-output":
			fmt.Print(e.Output)
		case "output":
			if e.Test != "" {
				buf(k).WriteString(e.Output)
				continue
			}
			fmt.Print(e.Output)
			if strings.HasPrefix(e.Output, "ok") && strings.Contains(e.Output, "[no tests to run]") {
				problems = append(problems, e.Package+": a -run pattern matched no tests")
			}
		case "pass":
			if e.Test != "" {
				passed++
			}
			delete(output, k)
		case "fail":
			if e.Test != "" {
				fmt.Print(buf(k).String())
				failed = append(failed, e.Package+" "+e.Test)
			} else {
				problems = append(problems, e.Package+": FAIL")
			}
			delete(output, k)
		case "skip":
			if e.Test == "" { // a package without tests
				continue
			}
			skipped++
			r := reason(buf(k).String())
			reasons[r]++
			if !matches(allow, r) {
				unexpected = append(unexpected, fmt.Sprintf("%s %s: %s", e.Package, e.Test, r))
			}
			delete(output, k)
		}
	}
	if err := in.Err(); err != nil {
		problems = append(problems, "reading go test -json: "+err.Error())
	}

	fmt.Printf("\n%d passed, %d failed, %d skipped\n", passed, len(failed), skipped)
	if len(reasons) > 0 {
		fmt.Println("\nSkips by reason:")
		rs := make([]string, 0, len(reasons))
		for r := range reasons {
			rs = append(rs, r)
		}
		sort.Slice(rs, func(i, j int) bool { return reasons[rs[i]] > reasons[rs[j]] })
		for _, r := range rs {
			mark := "  "
			if !matches(allow, r) {
				mark = "! "
			}
			fmt.Printf("%s%5d  %s\n", mark, reasons[r], r)
		}
	}
	report := func(title string, lines []string) {
		if len(lines) == 0 {
			return
		}
		fmt.Printf("\n%s:\n", title)
		for _, l := range lines {
			fmt.Println("    " + l)
		}
	}
	report("Failed tests", failed)
	report("Unexpected skips (install what they need, or pass -allow if they are expected)", unexpected)
	report("Problems", problems)
	if len(failed)+len(unexpected)+len(problems) > 0 {
		os.Exit(1)
	}
}

// reason returns the message of the last line a test logged, which for a
// skipped test is t.Skip's.
func reason(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if m := logLine.FindStringSubmatch(lines[i]); m != nil {
			return m[1]
		}
	}
	return "(no reason given)"
}

func matches(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
