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
	name    string // the diagnostic's name, if kc knows it: a warning's
}

// compileResult is what compiling a test's modules came to.
type compileResult struct {
	ok      bool
	diags   []diag
	warns   []diag // kc's warnings (see warnings.go)
	crash   string // kc's panic: its message and where
	other   string // another failure: the last line kc wrote
	outDirs []string
	// tried are the names of the files of the modules it compiled, or
	// tried to: the modules after one that failed aren't compiled.
	tried map[string]bool
}

// compiler compiles the test's modules into dir, each against the ones
// before it, with kc, or with kotlinc for the harness's self-check.
type compiler struct {
	kc      string   // the kc binary
	kotlinc string   // the kotlinc command, used instead of kc if set
	libs    []string // the standard library and kotlin-test
	reflect string
	javac   string // compiles the Java sources of a module others depend on
	timeout time.Duration
	// servers, if set, run kc's compiles, instead of a process each.
	servers *kcPool
	// annotations is the JetBrains annotations jar, which the test
	// infrastructure puts on every module's class path, if set.
	annotations string
	// checkerSets is set to enable kotlinc's extra and experimental
	// checkers as the diagnostics tests' infrastructure does: see
	// checkerFlags.
	checkerSets bool
}

// libraries says which of the libraries a test compiles against.
type libraries struct {
	stdlib  bool // the standard library and kotlin-test
	reflect bool
	optIns  []string // the opt-ins the test infrastructure gives it
}

// compile writes the test's files and extra into dir and compiles them.
// extra are files added to every module, such as helpers: the test
// infrastructure adds them to each module the directives apply to, and
// those at the top of a test apply to all.
func (c *compiler) compile(t *test, dir string, extra []*srcFile, libs libraries) (*compileResult, error) {
	res := &compileResult{tried: map[string]bool{}}
	byPath := map[string]string{}
	for mi, m := range t.modules {
		src := filepath.Join(dir, fmt.Sprintf("m%d", mi), "src")
		out := filepath.Join(dir, fmt.Sprintf("m%d", mi), "out")
		files := append(m.files[:len(m.files):len(m.files)], extra...)
		var paths, javas []string
		kotlin := false
		for _, f := range files {
			res.tried[f.name] = true
			p := filepath.Join(src, filepath.FromSlash(f.name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(p, []byte(f.text), 0o644); err != nil {
				return nil, err
			}
			paths = append(paths, p)
			byPath[p] = f.name
			if isJava(f) {
				javas = append(javas, p)
			} else {
				kotlin = true
			}
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		var cp []string
		if c.annotations != "" {
			cp = append(cp, c.annotations)
		}
		if libs.stdlib {
			cp = append(cp, c.libs...)
		}
		if libs.reflect {
			cp = append(cp, c.reflect)
		}
		deps, friends := moduleDeps(t.modules[:mi], res.outDirs, m)
		cp = append(cp, deps...)
		args := []string{"-d", out, "-module-name", m.name}
		if len(cp) > 0 {
			args = append(args, "-cp", strings.Join(cp, string(os.PathListSeparator)))
		}
		name := c.kc
		friendFlag := "-friend-paths"
		if c.kotlinc != "" {
			name = c.kotlinc
			friendFlag = "-Xfriend-paths"
			args = append(args, "-no-stdlib", "-no-reflect", "-nowarn")
		} else {
			args = append(args, "-features", "-j", "1")
			if c.checkerSets {
				args = append(args, checkerFlags(t, m)...)
			}
		}
		if len(friends) > 0 {
			args = append(args, friendFlag+"="+strings.Join(friends, ","))
		}
		for _, o := range libs.optIns {
			args = append(args, "-opt-in="+o)
		}
		if mode := jvmDefaultMode(t, m); mode != "" {
			args = append(args, "-jvm-default="+mode)
		}
		args = append(args, explicitAPIFlags(t, m)...)
		if target := moduleDirective(t, m, "JVM_TARGET"); target != "" {
			args = append(args, "-jvm-target", target)
		}
		args = append(args, paths...)
		// The Java sources of a module the next ones depend on are
		// compiled with javac, after the Kotlin ones, as the test
		// infrastructure does; those of the last module are only read.
		later := mi < len(t.modules)-1
		if !kotlin {
			res.outDirs = append(res.outDirs, out)
			if later {
				if err := c.compileJava(out, cp, javas); err != nil {
					return nil, err
				}
			}
			continue
		}
		output, failed, timedOut := c.run(name, args)
		res.outDirs = append(res.outDirs, out)
		// What the compiler said, without the directory's name, for
		// comparing two runs kept with -keep.
		kept := "test: " + t.path + "\n" + strings.ReplaceAll(output, dir, "")
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("m%d", mi), "compiler.out"), []byte(kept), 0o644); err != nil {
			return nil, err
		}
		if c.kotlinc == "" {
			res.warns = append(res.warns, kcWarnings(output, byPath)...)
		}
		if !failed {
			if later && len(javas) > 0 {
				if err := c.compileJava(out, append(cp, out), javas); err != nil {
					return nil, err
				}
			}
			continue
		}
		if timedOut {
			res.crash = "timed out"
			return res, nil
		}
		if c.kotlinc != "" {
			if res.other = kotlincError(output); res.other == "" {
				continue
			}
			return res, nil
		}
		parseKcOutput(output, byPath, res)
		return res, nil
	}
	res.ok = true
	return res, nil
}

// run runs the compiler command name with args, on a server if there
// are servers, and returns its output and whether it failed or timed
// out.
func (c *compiler) run(name string, args []string) (out string, failed, timedOut bool) {
	if c.servers != nil && c.kotlinc == "" {
		code, out, timedOut := c.servers.compile(args, c.timeout)
		return out, code != 0, timedOut
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	err := cmd.Run()
	return stderr.String(), err != nil, ctx.Err() != nil
}

// compileJava compiles the Java sources javas against the class path cp
// into out.
func (c *compiler) compileJava(out string, cp, javas []string) error {
	args := []string{"-d", out, "-nowarn", "-proc:none"}
	if len(cp) > 0 {
		args = append(args, "-cp", strings.Join(cp, string(os.PathListSeparator)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	b, err := exec.CommandContext(ctx, c.javac, append(args, javas...)...).CombinedOutput()
	if m := packageInModule.FindSubmatch(b); err != nil && m != nil {
		// A source of a package of the JDK's modules, as a library's
		// java.util.Date, which javac compiles only as a patch of the
		// module, from its source root.
		if root := sourceRoot(javas); root != "" {
			args = append(args, "--patch-module", string(m[1])+"="+root)
			b, err = exec.CommandContext(ctx, c.javac, append(args, javas...)...).CombinedOutput()
		}
	}
	if err != nil {
		return fmt.Errorf("javac: %v\n%s", err, b)
	}
	return nil
}

// packageInModule matches javac's error for a source of a package of a
// module of the JDK.
var packageInModule = regexp.MustCompile(`package exists in another module: ([\w.]+)`)

// sourceRoot returns the directory that the Java sources javas are in by
// their packages, or "" if they aren't all in one.
func sourceRoot(javas []string) string {
	root := ""
	for _, j := range javas {
		b, err := os.ReadFile(j)
		if err != nil {
			return ""
		}
		dir := filepath.Dir(j)
		if m := javaPackage.FindSubmatch(b); m != nil {
			pkg := filepath.FromSlash(strings.ReplaceAll(string(m[1]), ".", "/"))
			d, ok := strings.CutSuffix(dir, string(filepath.Separator)+pkg)
			if !ok {
				return ""
			}
			dir = d
		}
		if root != "" && root != dir {
			return ""
		}
		root = dir
	}
	return root
}

// javaPackage matches a Java source's package declaration.
var javaPackage = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)\s*;`)

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
		if d, warning, ok := parseKcLine(line, byPath); ok && !warning {
			res.diags = append(res.diags, d)
		}
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
		if isJava(f) {
			continue
		}
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

// moduleDeps returns the class path of m's output directories of the
// modules before it, outs[i] being mods[i]'s, and those of them that
// are its friends. As the test infrastructure's, the class path holds
// only the modules m's // MODULE: line names, its dependencies and then
// its friends, in that order: of two modules that declare one class,
// the one named first wins. It isn't transitive: a module that m's
// dependency depends on isn't on m's class path unless m names it too,
// and a module that names none gets none.
func moduleDeps(mods []*module, outs []string, m *module) (cp, friends []string) {
	dir := func(name string) string {
		for i, mod := range mods {
			if mod.name == name && i < len(outs) {
				return outs[i]
			}
		}
		return ""
	}
	for _, d := range m.deps {
		if o := dir(d); o != "" && !slices.Contains(cp, o) {
			cp = append(cp, o)
		}
	}
	for _, f := range m.friends {
		if o := dir(f); o != "" {
			if !slices.Contains(cp, o) {
				cp = append(cp, o)
			}
			friends = append(friends, o)
		}
	}
	return cp, friends
}

// checkerFlags returns kc's flags for the warnings of kotlinc's checkers
// that its command line enables only on request, as the diagnostics
// tests' infrastructure enables them for module m: the extra checkers
// (-Wextra, as UNREACHABLE_CODE) for WITH_EXTRA_CHECKERS and the tests of
// controlFlowAnalysis/deadCode and extraCheckers, whose markup expects
// their warnings without it, and the experimental ones (as
// REDUNDANT_INTERPOLATION_PREFIX) for WITH_EXPERIMENTAL_CHECKERS and the
// tests of extraCheckers, which expect EMPTY_RANGE.
func checkerFlags(t *test, m *module) []string {
	var flags []string
	has := func(d string) bool {
		_, inModule := m.directives[d]
		_, inTest := t.global[d]
		return inModule || inTest
	}
	if has("WITH_EXTRA_CHECKERS") || strings.HasPrefix(t.path, "tests/controlFlowAnalysis/deadCode/") ||
		strings.HasPrefix(t.path, "tests/extraCheckers/") {
		flags = append(flags, "-Wextra")
	}
	if has("WITH_EXPERIMENTAL_CHECKERS") || strings.HasPrefix(t.path, "tests/extraCheckers/") {
		flags = append(flags, "-Xuse-fir-experimental-checkers")
	}
	return flags
}

// explicitAPIFlags returns the flags of the explicit API mode
// (-Xexplicit-api) and the explicit return types mode
// (-XXexplicit-return-types) that module m is compiled with, as kotlinc's
// options for both compilers: the EXPLICIT_API_MODE directive's, or strict
// for the tests of testsWithExplicitApi and testsWithExplicitReturnTypes,
// whose test classes set them.
func explicitAPIFlags(t *test, m *module) []string {
	api := strings.ToLower(moduleDirective(t, m, "EXPLICIT_API_MODE"))
	if strings.HasPrefix(t.path, "tests/testsWithExplicitApi/") {
		api = "strict"
	}
	var flags []string
	if api != "" {
		flags = append(flags, "-Xexplicit-api="+api)
	}
	if strings.HasPrefix(t.path, "tests/testsWithExplicitReturnTypes/") {
		flags = append(flags, "-XXexplicit-return-types=strict")
	}
	return flags
}

// jvmDefaultMode returns the -jvm-default mode the JVM_DEFAULT_MODE
// directive asks for in module m: its own, or the test's before its
// first module, or "" for the default.
func jvmDefaultMode(t *test, m *module) string {
	return moduleDirective(t, m, "JVM_DEFAULT_MODE")
}

// moduleDirective returns the value of directive name in module m: its
// own, or the test's before its first module, or "" if neither has it.
// JVM_TARGET, the -jvm-target a module is compiled for, is one.
func moduleDirective(t *test, m *module, name string) string {
	for _, ds := range []map[string][]string{m.directives, t.global} {
		if vs := ds[name]; len(vs) > 0 {
			return strings.TrimSpace(vs[len(vs)-1])
		}
	}
	return ""
}
