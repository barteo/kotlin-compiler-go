package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// A diag is an error kc reported.
type diag struct {
	file    string // as the test names it
	line    int
	feature string // the unsupported feature, or ""
	msg     string
}

// compileResult is what compiling a test's modules came to.
type compileResult struct {
	ok      bool
	diags   []diag
	crash   string // kc's panic: its message and where
	other   string // another failure: the last line kc wrote
	outDirs []string
}

// compiler compiles the test's modules into dir, each against the ones
// before it, with kc, or with kotlinc for the harness's self-check.
type compiler struct {
	kc      string   // the kc binary
	kotlinc string   // the kotlinc command, used instead of kc if set
	libs    []string // the standard library and kotlin-test
	reflect string
	timeout time.Duration
}

// libraries says which of the libraries a test compiles against.
type libraries struct {
	stdlib  bool // the standard library and kotlin-test
	reflect bool
	optIns  []string // for kotlinc: the opt-ins the test infrastructure gives it
}

// compile writes the test's files and extra into dir and compiles them.
// extra are files added to every module, such as helpers: the test
// infrastructure adds them to each module the directives apply to, and
// those at the top of a test apply to all.
func (c *compiler) compile(t *test, dir string, extra []*srcFile, libs libraries) (*compileResult, error) {
	res := &compileResult{}
	byPath := map[string]string{}
	for mi, m := range t.modules {
		src := filepath.Join(dir, fmt.Sprintf("m%d", mi), "src")
		out := filepath.Join(dir, fmt.Sprintf("m%d", mi), "out")
		files := append(m.files[:len(m.files):len(m.files)], extra...)
		var paths []string
		for _, f := range files {
			p := filepath.Join(src, filepath.FromSlash(f.name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(p, []byte(f.text), 0o644); err != nil {
				return nil, err
			}
			paths = append(paths, p)
			byPath[p] = f.name
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		var cp []string
		if libs.stdlib {
			cp = slices.Clone(c.libs)
		}
		if libs.reflect {
			cp = append(cp, c.reflect)
		}
		cp = append(cp, res.outDirs...)
		args := []string{"-d", out, "-module-name", m.name}
		if len(cp) > 0 {
			args = append(args, "-cp", strings.Join(cp, string(os.PathListSeparator)))
		}
		name := c.kc
		if c.kotlinc != "" {
			name = c.kotlinc
			args = append(args, "-no-stdlib", "-no-reflect", "-nowarn")
			for _, o := range libs.optIns {
				args = append(args, "-opt-in="+o)
			}
		} else {
			args = append(args, "-features", "-j", "1")
		}
		if t.anyOf("JVM_DEFAULT_MODE", "disable") {
			args = append(args, "-jvm-default=disable")
		}
		args = append(args, paths...)
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		cmd := exec.CommandContext(ctx, name, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		cmd.Stdout = &stderr
		err := cmd.Run()
		timedOut := ctx.Err() != nil
		cancel()
		res.outDirs = append(res.outDirs, out)
		if err == nil {
			continue
		}
		if timedOut {
			res.crash = "timed out"
			return res, nil
		}
		if c.kotlinc != "" {
			if res.other = kotlincError(stderr.String()); res.other == "" {
				continue
			}
			return res, nil
		}
		parseKcOutput(stderr.String(), byPath, res)
		return res, nil
	}
	res.ok = true
	return res, nil
}

// parseKcOutput reads the diagnostics kc -features printed, or its
// panic.
func parseKcOutput(out string, byPath map[string]string, res *compileResult) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "panic: ") || strings.HasPrefix(line, "fatal error: ") {
			res.crash = panicWhere(lines[i:])
			return
		}
	}
	for _, line := range lines {
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 {
			continue
		}
		var l, col int
		fmt.Sscanf(f[1], "%d:%d", &l, &col)
		name := byPath[f[0]]
		if name == "" {
			name = filepath.Base(f[0])
		}
		res.diags = append(res.diags, diag{file: name, line: l, feature: f[2], msg: f[3]})
	}
	if len(res.diags) == 0 {
		res.other = lines[len(lines)-1]
	}
}

// panicWhere returns a panic's message and the innermost function of
// kc's it was in, with the line, from the panic's output.
func panicWhere(lines []string) string {
	msg := strings.TrimPrefix(strings.TrimPrefix(lines[0], "panic: "), "fatal error: ")
	msg = strings.TrimSuffix(msg, " [recovered, repanicked]")
	msg = strings.TrimSuffix(msg, " [recovered]")
	for j := 0; j+1 < len(lines); j++ {
		if strings.HasPrefix(lines[j], "github.com/barteo/kotlin-compiler-go/internal/") {
			fn, _, _ := strings.Cut(strings.TrimPrefix(lines[j], "github.com/barteo/kotlin-compiler-go/internal/"), "(0x")
			if i := strings.LastIndexByte(fn, '('); i > 0 && strings.HasSuffix(fn, ")") {
				fn = fn[:i] // the arguments
			}
			loc, _, _ := strings.Cut(strings.TrimSpace(lines[j+1]), " ")
			return msg + " in " + fn + " (" + filepath.Base(loc) + ")"
		}
	}
	return msg
}

// kotlincError returns the first error kotlinc reported, or "" if all
// are the test infrastructure's: the CHECK_TYPE helpers, and the tests
// that call them, use names (_) that kotlinc rejects outside it.
func kotlincError(out string) string {
	ignored := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "error:") {
			if strings.Contains(line, "/src/helpers/") || strings.Contains(line, "error: names _, __, ___") {
				ignored = true
				continue
			}
			return line
		}
	}
	if ignored {
		return ""
	}
	return strings.TrimSpace(out)
}

// progress prints how many tests are done, every 500.
func progress(done int64, total int) {
	if done%500 == 0 || done == int64(total) {
		fmt.Fprintf(os.Stderr, "%d/%d\n", done, total)
	}
}

// parseFeatures parses the files with kc's parser, with error recovery,
// and returns the errors: the unsupported features, as the parser stops
// at the first in kc itself, and syntax kc doesn't parse.
func parseFeatures(files []*srcFile) []diag {
	var ds []diag
	for _, f := range files {
		for _, e := range parseAll(f) {
			ds = append(ds, diag{file: f.name, line: e.Pos.Line, feature: e.Feature, msg: e.Msg})
		}
	}
	return ds
}

// parseAll returns the errors of syntax.ParseAll, or none if it panics,
// as it can on an error outside the declarations it recovers in. kc
// itself then reports the error.
func parseAll(f *srcFile) (errs []*syntax.Error) {
	defer func() {
		if recover() != nil {
			errs = nil
		}
	}()
	_, errs = syntax.ParseAll(f.name, f.text)
	return errs
}

// unsupportedKeys returns the keys of the unsupported features among ds,
// and of the parse errors that aren't about a feature, without
// duplicates. A checker error that says something isn't supported but
// names no feature counts as one, keyed by its message.
func unsupportedKeys(ds []diag, parse bool) []string {
	var keys []string
	seen := map[string]bool{}
	for _, d := range ds {
		k := d.feature
		switch {
		case k != "":
		case parse:
			k = "parse error: " + template(d.msg)
		case strings.Contains(d.msg, "not supported"):
			k = "checker: " + template(d.msg)
		default:
			continue
		}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

var templates = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`'[^']*'`), "'_'"},
	{regexp.MustCompile(`"[^"]*"`), `"_"`},
	{regexp.MustCompile(`: [^ ]+$`), ": _"},
	{regexp.MustCompile(`(class file|in class) \S+`), "$1 _"},
	{regexp.MustCompile(`takes? \(.*\)$`), "take (_)"},
	{regexp.MustCompile(`type is .* but .* was expected`), "type is _ but _ was expected"},
	{regexp.MustCompile(`of type \S+`), "of type _"},
	{regexp.MustCompile(`takes \d+ arguments`), "takes n arguments"},
	{regexp.MustCompile(`the \d+ candidates`), "the n candidates"},
	{regexp.MustCompile(`\d+ candidates`), "n candidates"},
	{regexp.MustCompile(`\d+`), "n"},
}

// template replaces the names, types and counts in a message with _ or
// n, so that messages about different names group together. (As
// bench/survey's, which this copies, with numbers too.)
func template(msg string) string {
	for _, t := range templates {
		msg = t.re.ReplaceAllString(msg, t.repl)
	}
	return msg
}
