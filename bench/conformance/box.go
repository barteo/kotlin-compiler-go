package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

// The categories of a result. A box test fails in the first of
// unsupported to timeout that applies.
const (
	catPass        = "pass"
	catSkip        = "skip"
	catUnsupported = "parse/unsupported"
	catCheck       = "check error"
	catCrash       = "crash"
	catKc          = "codegen error"
	catVerify      = "VerifyError"
	catLinkage     = "linkage error"
	catException   = "exception"
	catWrong       = "wrong result"
	catTimeout     = "timeout"
)

// A result is the outcome of a test.
type result struct {
	path     string
	category string
	key      string   // what groups it with others in its category
	keys     []string // for parse/unsupported, every feature found
	detail   string
}

// boxConfig is the box mode's configuration.
type boxConfig struct {
	suite   string // compiler/testData/codegen/box
	helpers string // compiler/testData/diagnostics/helpers
	work    string
	jobs    int
	keep    bool
	comp    *compiler
	pool    *pool
}

var boxFunRE = regexp.MustCompile(`(?m)^\s*fun\s+box\s*\(\s*\)`)

// boxSkip returns why the box mode doesn't run t, or "".
func boxSkip(t *test) string {
	if r := commonSkip(t); r != "" {
		return r
	}
	for _, d := range []string{"IGNORE_BACKEND", "IGNORE_BACKEND_K2"} {
		if t.anyOf(d, "JVM", "JVM_IR", "ANY") {
			return d + ": JVM_IR"
		}
	}
	if len(t.modules) > 1 && t.anyOf("IGNORE_BACKEND_K2_MULTI_MODULE", "JVM", "JVM_IR", "ANY") {
		return "IGNORE_BACKEND_K2_MULTI_MODULE: JVM_IR"
	}
	if boxFile(t) == nil {
		return "no box()"
	}
	return ""
}

// boxFile returns the file that declares box(), looking at the last
// module first.
func boxFile(t *test) *srcFile {
	for i := len(t.modules) - 1; i >= 0; i-- {
		for _, f := range t.modules[i].files {
			if boxFunRE.MatchString(f.text) {
				return f
			}
		}
	}
	return nil
}

var (
	packageRE = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)`)
	jvmNameRE = regexp.MustCompile(`@file\s*:\s*JvmName\s*\(\s*"([^"]+)"`)
)

// facadeClass returns the binary name of the class of f's top-level
// functions, as kotlinc names it: by @file:JvmName, or by the file's
// name, made a Java identifier and capitalized, with Kt after it.
func facadeClass(f *srcFile) string {
	name := ""
	if m := jvmNameRE.FindStringSubmatch(f.text); m != nil {
		name = m[1]
	} else {
		base := strings.TrimSuffix(filepath.Base(f.name), ".kt")
		var sb strings.Builder
		for i, r := range base {
			switch {
			case i == 0 && !unicode.IsLetter(r) && r != '_' && r != '$':
				sb.WriteByte('_')
				sb.WriteRune(r)
			case i == 0:
				sb.WriteRune(unicode.ToUpper(r))
			case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$':
				sb.WriteRune(r)
			default:
				sb.WriteByte('_')
			}
		}
		name = sb.String() + "Kt"
	}
	if m := packageRE.FindStringSubmatch(f.text); m != nil {
		return m[1] + "." + name
	}
	return name
}

// prepareBox applies what the test infrastructure does to a box test's
// sources before compiling them.
func prepareBox(t *test) {
	for _, f := range t.files() {
		// The value-class tests are templates for each backend.
		f.text = strings.ReplaceAll(f.text, "OPTIONAL_JVM_INLINE_ANNOTATION", "@JvmInline")
		// The tests in diagnostics/ mark the warnings kotlinc reports.
		f.text, _ = stripMarkup(f.text)
	}
}

// helperFiles returns the helpers in dir (compiler/testData/diagnostics/helpers)
// that the test's directives add, as the test infrastructure's
// AdditionalDiagnosticsSourceFilesProvider and
// CoroutineHelpersSourceFilesProvider do.
func helperFiles(dir string, t *test) ([]*srcFile, error) {
	var names []string
	for _, h := range []struct{ directive, file string }{
		{"CHECK_TYPE", "types/checkType.kt"},
		{"CHECK_TYPE_WITH_EXACT", "types/checkTypeWithExact.kt"},
		{"INFERENCE_HELPERS", "inference/inferenceUtils.kt"},
	} {
		if t.has(h.directive) {
			names = append(names, h.file)
		}
	}
	if t.has("WITH_COROUTINES") {
		names = append(names, "coroutines/CoroutineHelpers.kt")
		if t.has("CHECK_STATE_MACHINE") {
			names = append(names, "coroutines/CoroutineUtil.kt", "coroutines/StateMachineChecker.kt")
		}
		if t.has("CHECK_TAIL_CALL_OPTIMIZATION") {
			names = append(names, "coroutines/TailCallOptimizationChecker.kt")
		}
	}
	var fs []*srcFile
	for _, n := range names {
		src, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		fs = append(fs, &srcFile{name: "helpers/" + n, text: string(src), line: 1})
	}
	return fs, nil
}

// runBox runs the tests and returns their results, in the order of tests.
func runBox(c *boxConfig, tests []*test) []*result {
	results := make([]*result, len(tests))
	var wg sync.WaitGroup
	next := make(chan int)
	var done atomic.Int64
	for range c.jobs {
		wg.Go(func() {
			for i := range next {
				results[i] = c.runOne(i, tests[i])
				progress(done.Add(1), len(tests))
			}
		})
	}
	for i := range tests {
		next <- i
	}
	close(next)
	wg.Wait()
	return results
}

func (c *boxConfig) runOne(i int, t *test) *result {
	r := &result{path: t.path}
	if reason := boxSkip(t); reason != "" {
		r.category, r.key = catSkip, reason
		return r
	}
	prepareBox(t)
	helpers, err := helperFiles(c.helpers, t)
	if err != nil {
		r.category, r.key, r.detail = catKc, "harness error", err.Error()
		return r
	}
	if c.comp.kotlinc == "" {
		if ds := parseFeatures(append(t.files(), helpers...)); len(ds) > 0 {
			r.category = catUnsupported
			r.keys = unsupportedKeys(ds, true)
			r.key = r.keys[0]
			r.detail = fmt.Sprintf("%s:%d: %s", ds[0].file, ds[0].line, ds[0].msg)
			return r
		}
	}
	dir := filepath.Join(c.work, fmt.Sprint(i))
	if !c.keep {
		defer os.RemoveAll(dir)
	}
	withReflect := t.has("WITH_REFLECT")
	cr, err := c.comp.compile(t, dir, helpers, libraries{stdlib: true, reflect: withReflect})
	if err != nil {
		r.category, r.key, r.detail = catKc, "harness error", err.Error()
		return r
	}
	if !cr.ok {
		classifyCompile(r, cr)
		return r
	}
	o := c.pool.run(facadeClass(boxFile(t)), reverse(cr.outDirs), withReflect)
	classifyRun(r, o)
	return r
}

// classifyCompile sets r from a failed compilation.
func classifyCompile(r *result, cr *compileResult) {
	switch {
	case cr.crash != "":
		r.category, r.key = catCrash, cr.crash
	case strings.Contains(cr.other, "runtime error: "):
		// A panic that code generation recovered from.
		r.category, r.key, r.detail = catCrash, otherKey(cr.other), cr.other
	case cr.other != "":
		r.category, r.key, r.detail = catKc, otherKey(cr.other), cr.other
	default:
		if keys := unsupportedKeys(cr.diags, false); len(keys) > 0 {
			r.category, r.keys, r.key = catUnsupported, keys, keys[0]
			for _, d := range cr.diags {
				if d.feature != "" || strings.Contains(d.msg, "not supported") {
					r.detail = fmt.Sprintf("%s:%d: %s", d.file, d.line, d.msg)
					break
				}
			}
			return
		}
		d := cr.diags[0]
		r.category, r.key = catCheck, template(d.msg)
		r.detail = fmt.Sprintf("%s:%d: %s", d.file, d.line, d.msg)
		if strings.HasPrefix(d.file, "helpers/") {
			r.key = "in a helper: " + r.key
		}
	}
}

// otherKey groups the errors kc reports other than diagnostics, mostly
// code generation's: the message without the file, class and method.
func otherKey(msg string) string {
	if _, rest, ok := strings.Cut(msg, ".kt: "); ok {
		msg = rest
	}
	if rest, ok := strings.CutPrefix(msg, "code generation failed: "); ok {
		if _, m, ok := strings.Cut(rest, ": "); ok {
			rest = m
		}
		msg = "code generation: " + rest
	}
	return template(msg)
}

// classifyRun sets r from running box().
func classifyRun(r *result, o runOutcome) {
	field := func(i int) string {
		if i < len(o.detail) {
			return o.detail[i]
		}
		return ""
	}
	switch o.status {
	case "OK":
		r.category = catPass
	case "FAIL":
		r.category, r.key, r.detail = catWrong, truncate(template(field(0)), 60), field(0)
	case "NOBOX":
		r.category, r.key, r.detail = catLinkage, "no static box() in the class kotlinc puts it in", field(0)
	case "VERIFY":
		r.category, r.key = catVerify, field(0)+": "+template(field(1))
		r.detail = field(1) + " at " + field(2)
	case "LINKAGE":
		r.category, r.key = catLinkage, field(0)
		r.detail = field(1) + " at " + field(2)
	case "EXCEPTION":
		r.category, r.key = catException, field(0)
		r.detail = field(1) + " at " + field(2)
	case "TIMEOUT":
		r.category, r.key = catTimeout, "timeout"
	default:
		r.category, r.key, r.detail = catTimeout, "JVM died", strings.Join(o.detail, " ")
	}
}

func reverse(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
