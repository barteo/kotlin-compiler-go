package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// A test is one test file of the Kotlin repository: its directives, and
// its sources split into modules and files by the // MODULE: and // FILE:
// markers.
type test struct {
	path       string // relative to the suite's directory
	directives map[string][]string
	modules    []*module
}

type module struct {
	name      string
	deps      []string
	friends   []string
	dependsOn []string
	files     []*srcFile
}

type srcFile struct {
	name string // relative, as // FILE: names it
	text string
	// line is the line of the test file that the file's first line is,
	// counting from 1.
	line int
}

// directiveRE matches a directive line: // NAME or // NAME: value, also
// in the older // !NAME form.
var directiveRE = regexp.MustCompile(`^//\s*!?([A-Z][A-Z0-9_]+)(?:\s*:\s*(.*?))?\s*$`)

// moduleRE matches a module's specification: its name, then its
// dependencies, friends and dependsOn modules in parentheses.
var moduleRE = regexp.MustCompile(`^([\w.-]+)\s*((?:\([^)]*\))*)`)

// readTest reads and splits the test file abs.
func readTest(abs, rel string) (*test, error) {
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	t := &test{path: rel, directives: map[string][]string{}}
	var mod *module
	var file *srcFile
	var sb strings.Builder
	defaultName := filepath.Base(abs)
	flush := func() {
		if file != nil {
			file.text = sb.String()
		}
		sb.Reset()
	}
	newModule := func(name string) {
		flush()
		mod = &module{name: name}
		t.modules = append(t.modules, mod)
		file = &srcFile{name: defaultName}
		mod.files = append(mod.files, file)
	}
	lines := strings.SplitAfter(string(src), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := directiveRE.FindStringSubmatch(trimmed)
		switch {
		case m != nil && m[1] == "MODULE":
			spec := moduleRE.FindStringSubmatch(m[2])
			if spec == nil {
				t.directives[m[1]] = append(t.directives[m[1]], m[2])
				continue
			}
			newModule(spec[1])
			groups := strings.Split(strings.TrimSuffix(strings.TrimPrefix(spec[2], "("), ")"), ")(")
			lists := []*[]string{&mod.deps, &mod.friends, &mod.dependsOn}
			for j, g := range groups {
				if j < len(lists) {
					*lists[j] = splitList(g)
				}
			}
			mod.files[0].line = i + 2
			t.directives["MODULE"] = append(t.directives["MODULE"], m[2])
			continue
		case m != nil && m[1] == "FILE":
			if mod == nil {
				newModule("main")
			}
			flush()
			file = &srcFile{name: m[2], line: i + 2}
			mod.files = append(mod.files, file)
			continue
		case m != nil:
			t.directives[m[1]] = append(t.directives[m[1]], m[2])
		}
		if mod == nil {
			newModule("main")
			file.line = i + 1
		}
		sb.WriteString(line)
	}
	flush()
	// Drop the files that hold only directives and comments, as the text
	// before the first // FILE: usually does.
	var mods []*module
	for _, m := range t.modules {
		m.files = filterFiles(m.files, func(f *srcFile) bool { return !onlyComments(f.text) })
		if len(m.files) > 0 {
			mods = append(mods, m)
		}
	}
	t.modules = mods
	return t, nil
}

func filterFiles(fs []*srcFile, keep func(*srcFile) bool) []*srcFile {
	var out []*srcFile
	for _, f := range fs {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}

// onlyComments reports whether src holds only blank lines and // comments.
func onlyComments(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "//") {
			return false
		}
	}
	return true
}

// splitList splits a directive's value, a list separated by commas or
// spaces.
func splitList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}

// has reports whether the test has the directive name.
func (t *test) has(name string) bool {
	_, ok := t.directives[name]
	return ok
}

// values returns the values of all of the directive's occurrences.
func (t *test) values(name string) []string {
	var vs []string
	for _, v := range t.directives[name] {
		vs = append(vs, splitList(v)...)
	}
	return vs
}

// anyOf reports whether a value of the directive name is one of vs.
func (t *test) anyOf(name string, vs ...string) bool {
	for _, v := range t.values(name) {
		for _, w := range vs {
			if v == w {
				return true
			}
		}
	}
	return false
}

// files returns all of the test's files.
func (t *test) files() []*srcFile {
	var fs []*srcFile
	for _, m := range t.modules {
		fs = append(fs, m.files...)
	}
	return fs
}

// commonSkip returns why the harnesses don't run a test, or "": it is for
// another platform, needs kotlinc's options or Java sources, or has
// modules that aren't plain dependencies.
func commonSkip(t *test) string {
	if vs := t.values("TARGET_BACKEND"); len(vs) > 0 && !t.anyOf("TARGET_BACKEND", "JVM", "JVM_IR") {
		return "TARGET_BACKEND: " + strings.Join(vs, ", ")
	}
	if t.anyOf("DONT_TARGET_EXACT_BACKEND", "JVM", "JVM_IR") {
		return "DONT_TARGET_EXACT_BACKEND: JVM_IR"
	}
	if vs := t.values("TARGET_PLATFORM"); len(vs) > 0 && !slicesContainsFold(vs, "JVM") {
		return "TARGET_PLATFORM: " + strings.Join(vs, ", ")
	}
	for _, f := range t.values("LANGUAGE") {
		if !defaultLanguage(f) {
			return "LANGUAGE"
		}
	}
	for _, d := range []string{"API_VERSION", "LANGUAGE_VERSION", "FREE_COMPILER_ARGS", "ASSERTIONS_MODE", "ALLOW_KOTLIN_PACKAGE"} {
		if t.has(d) {
			return d
		}
	}
	if t.anyOf("JVM_DEFAULT_MODE", "enable") {
		return "JVM_DEFAULT_MODE: enable"
	}
	if t.anyOf("LAMBDAS", "CLASS") {
		return "LAMBDAS: CLASS" // -Xlambdas=class, which kc doesn't have
	}
	for _, f := range t.files() {
		if !strings.HasSuffix(f.name, ".kt") {
			if strings.HasSuffix(f.name, ".java") {
				return "Java sources"
			}
			return "other sources (" + filepath.Ext(f.name) + ")"
		}
	}
	for _, m := range t.modules {
		if len(m.friends) > 0 || len(m.dependsOn) > 0 {
			return "modules with friends or dependsOn"
		}
	}
	return ""
}

// languageFeatures says, for each of kotlinc's language features, whether
// it is on by default, as kotlinc's LanguageFeature at the checkout's tag
// says (readLanguageFeatures). The Kotlin kc compiles is that version's.
var languageFeatures map[string]bool

// defaultLanguage reports whether a value of the LANGUAGE directive changes
// nothing: it turns on a feature on by default, or turns off one off by
// default.
func defaultLanguage(v string) bool {
	if len(v) < 2 {
		return false
	}
	on, known := languageFeatures[v[1:]]
	switch v[0] {
	case '+':
		return on
	case '-':
		return known && !on
	}
	return false
}

// languageSettings is the file of the checkout that declares kotlinc's
// language features and versions.
const languageSettings = "core/language.version-settings/src/org/jetbrains/kotlin/config/LanguageVersionSettings.kt"

var (
	latestStableRE = regexp.MustCompile(`\bval LATEST_STABLE = KOTLIN_(\d+)_(\d+)\b`)
	// featureRE matches an entry of enum class LanguageFeature, which gives
	// first the language version that turns it on, or null.
	featureRE = regexp.MustCompile(`^    ([A-Z]\w*)\(\s*(?:sinceVersion\s*=\s*)?(?:null|KOTLIN_(\d+)_(\d+))\b`)
	// apiVersionRE matches the API version an entry also needs.
	apiVersionRE = regexp.MustCompile(`\bApiVersion\.KOTLIN_(\d+)_(\d+)\b`)
)

// readLanguageFeatures reads, from the checkout of JetBrains/kotlin in
// dir, which language features kotlinc has on by default: those since a
// language version, and API version if they give one, up to the latest
// stable one, which kotlinc compiles by default.
func readLanguageFeatures(dir string) (map[string]bool, error) {
	path := filepath.Join(dir, languageSettings)
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := latestStableRE.FindSubmatch(src)
	if m == nil {
		return nil, fmt.Errorf("%s: no LanguageVersion.LATEST_STABLE", path)
	}
	latest := version(string(m[1]), string(m[2]))
	features := map[string]bool{}
	in := false
	sc := bufio.NewScanner(bytes.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		if !in {
			in = strings.HasPrefix(line, "enum class LanguageFeature(")
			continue
		}
		if strings.TrimSpace(line) == ";" {
			break
		}
		m := featureRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		on := m[2] != "" && version(m[2], m[3]) <= latest
		if a := apiVersionRE.FindStringSubmatch(line); a != nil && version(a[1], a[2]) > latest {
			on = false
		}
		features[m[1]] = on
	}
	if len(features) == 0 {
		return nil, fmt.Errorf("%s: no entries of enum class LanguageFeature", path)
	}
	return features, nil
}

// version orders the version major.minor.
func version(major, minor string) int {
	a, _ := strconv.Atoi(major)
	b, _ := strconv.Atoi(minor)
	return a*1000 + b
}

func slicesContainsFold(vs []string, s string) bool {
	for _, v := range vs {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
