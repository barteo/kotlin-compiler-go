package main

// The -repos mode surveys many repositories, each pinned to a release
// tag in a manifest (bench/survey/repos.txt), and ranks the features
// kc doesn't support by the files they block across all of them. The
// ranking counts the library code, the "main" source sets; the tests are
// reported apart.
//
// As a second cross-check, the checker runs on each repository's library
// files together, with only the standard library on the class path, and
// the unsupported features it reports are compared with the detector's.
// It runs in a child process (-check-features), so that a panic on
// unfamiliar code is a finding for that repository, not the end of the
// run.

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// project is a repository of the manifest.
type project struct {
	name, url, tag string
	dir            string   // the clone
	args           []string // its source sets, label:dir[,dir...]
	sets           []*sourceSet
	err            error
	checkerErr     string   // why the checker pass failed, if it did
	crashes        []*crash // the files the checker panics on
}

// main returns the project's library set: the one labeled main, or the
// first.
func (p *project) main() *sourceSet {
	for _, s := range p.sets {
		if s.label == "main" {
			return s
		}
	}
	return p.sets[0]
}

// tests returns the project's other sets.
func (p *project) tests() []*sourceSet {
	var out []*sourceSet
	for _, s := range p.sets {
		if s != p.main() {
			out = append(out, s)
		}
	}
	return out
}

// readManifest reads the repositories that the manifest lists, with
// their clones in cache.
func readManifest(path, cache string) ([]*project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ps []*project
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			return nil, fmt.Errorf("%s:%d: want name url tag label:dir[,dir...] ...", path, n+1)
		}
		ps = append(ps, &project{name: f[0], url: f[1], tag: f[2], args: f[3:],
			dir: filepath.Join(cache, f[0]+"-"+f[2])})
	}
	return ps, nil
}

// wide surveys the repositories of the manifest and writes the report to
// out, and each repository's to dir/name.md if dir is set.
func wide(manifest, cache, cp, out, dir string, inlineOnly map[string]string) {
	ps, err := readManifest(manifest, cache)
	if err != nil {
		log.Fatal(err)
	}
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, p := range ps {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			p.sets, p.err = loadSets(p.dir, p.dir, p.args, inlineOnly)
		})
	}
	wg.Wait()
	for _, p := range ps {
		if p.err != nil {
			log.Fatalf("%s: %v", p.name, p.err)
		}
	}
	// The checker passes each use every CPU.
	for _, p := range ps {
		checkerPass(p, cp)
	}

	var sb strings.Builder
	reportWide(&sb, ps, len(inlineOnly))
	if out == "" {
		fmt.Print(sb.String())
	} else if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, p := range ps {
		var sb strings.Builder
		report(&sb, p.name+" "+p.tag, p.sets, len(inlineOnly))
		reportChecker(&sb, p)
		if err := os.WriteFile(filepath.Join(dir, p.name+".md"), []byte(sb.String()), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

// checkerPass runs the checker on p's library files in a child process,
// and records the unsupported features it reports in the files. If the
// checker panics, it checks each half of the files apart, down to the
// files it panics on, which it records in p.crashes.
func checkerPass(p *project, cp string) {
	byPath := map[string]*file{}
	var paths []string
	for _, f := range p.main().files {
		byPath[f.abs] = f
		paths = append(paths, f.abs)
	}
	var run func(paths []string)
	run = func(paths []string) {
		if p.checkerErr != "" {
			return
		}
		out, c, err := checkFiles(paths, cp)
		switch {
		case err != nil:
			p.checkerErr = err.Error()
			return
		case c != nil && len(paths) == 1:
			c.file = byPath[paths[0]]
			p.crashes = append(p.crashes, c)
			return
		case c != nil:
			run(paths[:len(paths)/2])
			run(paths[len(paths)/2:])
			return
		}
		for _, line := range strings.Split(out, "\n") {
			path, feature, ok := strings.Cut(line, "\t")
			if !ok {
				continue
			}
			feature, _, _ = strings.Cut(feature, "\t")
			if f := byPath[path]; f != nil {
				if f.checker == nil {
					f.checker = map[string]int{}
				}
				f.checker[feature]++
			}
		}
	}
	run(paths)
}

// crash is a panic of the checker.
type crash struct {
	file  *file
	msg   string // the panic's message
	where string // the innermost function of kc's it was in, and the line
}

// checkFiles runs a child process that checks the files together, and
// returns what it printed, or the panic it ended with, or an error if it
// failed otherwise.
func checkFiles(paths []string, cp string) (string, *crash, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command(exe, "-check-features", "-cp", cp)
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil, nil
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "panic: ") {
			continue
		}
		c := &crash{msg: strings.TrimSuffix(strings.TrimPrefix(line, "panic: "), " [recovered, repanicked]")}
		// The first of kc's frames: the function, then its file and line.
		for j := i; j+1 < len(lines); j++ {
			if strings.HasPrefix(lines[j], "github.com/barteo/kotlin-compiler-go/internal/") {
				fn, _, _ := strings.Cut(strings.TrimPrefix(lines[j], "github.com/barteo/kotlin-compiler-go/internal/"), "(0x")
				if i := strings.LastIndexByte(fn, '('); i > 0 && strings.HasSuffix(fn, ")") {
					fn = fn[:i] // the arguments
				}
				loc, _, _ := strings.Cut(strings.TrimSpace(lines[j+1]), " ")
				c.where = fn + " (" + filepath.Base(loc) + ")"
				break
			}
		}
		return "", c, nil
	}
	// It failed otherwise: the last line it wrote says why.
	if last := lines[len(lines)-1]; last != "" {
		return "", nil, fmt.Errorf("%s", last)
	}
	return "", nil, err
}

// printCheckerFeatures checks the files whose paths are on stdin
// together, against the class path cpFlag, and prints each unsupported
// feature the checker reports: path, feature, position and message,
// separated by tabs.
func printCheckerFeatures(cpFlag string) {
	jdk, err := classpath.FindJDK()
	if err != nil {
		log.Fatal(err)
	}
	var entries []string
	if cpFlag != "" {
		entries = strings.Split(cpFlag, string(os.PathListSeparator))
	}
	cp, err := classpath.Open(jdk, entries)
	if err != nil {
		log.Fatal(err)
	}
	defer cp.Close()
	var files []*syntax.File
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		path := sc.Text()
		if path == "" {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		if pf, _ := syntax.ParseAll(path, string(src)); pf != nil {
			files = append(files, pf)
		}
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for _, e := range check.CheckWith(files, check.NewLibrary(cp), runtime.NumCPU()) {
		if e.Feature != "" {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.File, e.Feature, e.Pos, e.Msg)
		}
	}
}

// doneFeatures returns the features the milestones added.
func doneFeatures() map[string]bool {
	done := map[string]bool{}
	for _, m := range milestones {
		for _, f := range m.features {
			done[f] = true
		}
	}
	return done
}

// blockers returns the features of f that aren't done, sorted.
func blockers(f *file, done map[string]bool) []string {
	var out []string
	for feat := range f.features {
		if !done[feat] {
			out = append(out, feat)
		}
	}
	sort.Strings(out)
	return out
}

// repoFiles is a set of files of one repository.
type repoFiles struct {
	p     *project
	files []*file
}

func reportWide(sb *strings.Builder, ps []*project, nInline int) {
	done := doneFeatures()
	var lib, tests []repoFiles
	nLib, nTests := 0, 0
	for _, p := range ps {
		lib = append(lib, repoFiles{p, p.main().files})
		nLib += len(p.main().files)
		var ts []*file
		for _, s := range p.tests() {
			ts = append(ts, s.files...)
		}
		tests = append(tests, repoFiles{p, ts})
		nTests += len(ts)
	}

	fmt.Fprintf(sb, "# Survey of %d Kotlin libraries\n\n", len(ps))
	fmt.Fprintf(sb, "Written by `bench/survey -repos` (see `bench/survey.sh`). For each repository that `bench/survey/repos.txt` lists, at its pinned release tag, a detector that reads kc's tokens lists the features each file uses that kc doesn't support. A file is *blocked* by each of them. The ranking counts the library code that the JVM compilation includes (the `main` sets, %d files); the tests (%d files) are counted apart. The detector only sees syntax and the names of the %d inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile. Each repository's own report, with every file's blockers, is in `bench/results/survey/`.\n\n", nLib, nTests, nInline)

	// The repositories.
	fmt.Fprintf(sb, "## Repositories\n\n| Repository | Tag | Files | Lines | Unblocked | Median blockers left | Test files | Tests unblocked |\n|:---|:---|---:|---:|---:|---:|---:|---:|\n")
	var tf, tl, tu, ttf, ttu int
	for i, p := range ps {
		lines := 0
		for _, f := range lib[i].files {
			lines += f.lines
		}
		u, ut := unblocked(lib[i].files, done), unblocked(tests[i].files, done)
		fmt.Fprintf(sb, "| %s | %s | %d | %d | %d | %s | %d | %d |\n", p.name, p.tag, len(lib[i].files), lines, u, median(lib[i].files, done), len(tests[i].files), ut)
		tf, tl, tu, ttf, ttu = tf+len(lib[i].files), tl+lines, tu+u, ttf+len(tests[i].files), ttu+ut
	}
	fmt.Fprintf(sb, "| **all** | | %d | %d | %d | %s | %d | %d |\n\n", tf, tl, tu, median(pool(lib), done), ttf, ttu)

	fmt.Fprintf(sb, "## Library code\n\n")
	reportFeatures(sb, lib, done)
	reportGreedy(sb, lib, done)
	fmt.Fprintf(sb, "### Top blockers by repository\n\nThe features that block the most of each repository's library files, with the number of files.\n\n| Repository | Blocked | Top blockers |\n|:---|---:|:---|\n")
	for _, r := range lib {
		cs := countFeatures(r.files, done)
		var top []string
		for i, c := range cs {
			if i == 5 {
				break
			}
			top = append(top, fmt.Sprintf("%s (%d)", c.feature, c.files))
		}
		fmt.Fprintf(sb, "| %s | %d | %s |\n", r.p.name, len(r.files)-unblocked(r.files, done), strings.Join(top, ", "))
	}

	fmt.Fprintf(sb, "\n## Tests\n\n")
	reportFeatures(sb, tests, done)
	reportGreedy(sb, tests, done)

	reportParserCheck(sb, ps)
	reportCheckerCheck(sb, ps, done)
}

// pool returns the files of all the repositories.
func pool(rs []repoFiles) []*file {
	var out []*file
	for _, r := range rs {
		out = append(out, r.files...)
	}
	return out
}

// featureCount counts the files a feature blocks.
type featureCount struct {
	feature            string
	files, only, total int
	repos              int
	share              float64 // the mean, over the repositories, of the share of their files it blocks
}

// countFeatures counts the features of files that aren't done, by the
// files they block, most first.
func countFeatures(files []*file, done map[string]bool) []*featureCount {
	counts := map[string]*featureCount{}
	for _, f := range files {
		left := blockers(f, done)
		for _, feat := range left {
			c := counts[feat]
			if c == nil {
				c = &featureCount{feature: feat}
				counts[feat] = c
			}
			c.files++
			c.total += f.features[feat]
			if len(left) == 1 {
				c.only++
			}
		}
	}
	var cs []*featureCount
	for _, c := range counts {
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].files != cs[j].files {
			return cs[i].files > cs[j].files
		}
		return cs[i].feature < cs[j].feature
	})
	return cs
}

func reportFeatures(sb *strings.Builder, rs []repoFiles, done map[string]bool) {
	all := pool(rs)
	cs := countFeatures(all, done)
	for _, c := range cs {
		for _, r := range rs {
			n := 0
			for _, f := range r.files {
				if f.features[c.feature] > 0 {
					n++
				}
			}
			if n > 0 {
				c.repos++
				c.share += float64(n) / float64(len(r.files))
			}
		}
		c.share /= float64(len(rs))
	}
	fmt.Fprintf(sb, "### Features left, by files blocked\n\n%d of %d files use no feature that kc doesn't support. *Repositories* is the number of repositories with a file the feature blocks, and *mean share* the mean, over all %d repositories, of the share of each one's files that it blocks, so that a large repository doesn't decide the order alone. *Only blocker* counts the files it is the last blocker of.\n\n", unblocked(all, done), len(all), len(rs))
	fmt.Fprintf(sb, "| Feature | Files | Repositories | Mean share | Only blocker | Uses |\n|:---|---:|---:|---:|---:|---:|\n")
	for _, c := range cs {
		fmt.Fprintf(sb, "| %s | %d | %d | %.1f%% | %d | %d |\n", c.feature, c.files, c.repos, 100*c.share, c.only, c.total)
	}
	fmt.Fprintln(sb)
}

// reportGreedy writes the greedy order of the features left over the
// pooled files: at each step the feature that unblocks the most files
// given the ones before it, or, if none unblocks one, the one that
// blocks the most files.
func reportGreedy(sb *strings.Builder, rs []repoFiles, done map[string]bool) {
	all := pool(rs)
	cs := countFeatures(all, done)
	fmt.Fprintf(sb, "### Greedy order of the features left\n\nAt each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files. *Repositories gaining* counts the repositories with a file it unblocks.\n\n")
	fmt.Fprintf(sb, "| Step | Feature | Files unblocked | Repositories gaining |\n|---:|:---|---:|---:|\n")
	added := map[string]bool{}
	for f := range done {
		added[f] = true
	}
	before := make([]int, len(rs))
	for i, r := range rs {
		before[i] = unblocked(r.files, added)
	}
	for step := 1; step <= len(cs); step++ {
		best, bestGain, bestFiles := "", -1, -1
		for _, c := range cs {
			if added[c.feature] {
				continue
			}
			added[c.feature] = true
			gain := unblocked(all, added)
			delete(added, c.feature)
			if gain > bestGain || gain == bestGain && c.files > bestFiles {
				best, bestGain, bestFiles = c.feature, gain, c.files
			}
		}
		added[best] = true
		gaining := 0
		for i, r := range rs {
			if n := unblocked(r.files, added); n > before[i] {
				gaining++
				before[i] = n
			}
		}
		fmt.Fprintf(sb, "| %d | %s | %d | %d |\n", step, best, bestGain, gaining)
	}
	fmt.Fprintln(sb)
}

// reportParserCheck writes the parser cross-check over all the files:
// the errors that name no feature the detector found in the file, by
// message.
func reportParserCheck(sb *strings.Builder, ps []*project) {
	type group struct {
		msg, feature string
		files        map[string]bool
		repos        map[string]bool
		first        string
	}
	groups := map[string]*group{}
	nErrs, nMissed := 0, 0
	for _, p := range ps {
		for _, s := range p.sets {
			for _, f := range s.files {
				var errs []*syntax.Error
				for _, e := range f.lexErrs {
					errs = append(errs, &syntax.Error{Pos: e.Pos, Msg: "lexer: " + e.Msg})
				}
				errs = append(errs, f.parseErrs...)
				for _, e := range errs {
					nErrs++
					if e.Feature != "" && f.detected[e.Feature] > 0 {
						continue
					}
					nMissed++
					key := e.Feature + "\x00" + e.Msg
					g := groups[key]
					if g == nil {
						g = &group{msg: e.Msg, feature: e.Feature, files: map[string]bool{}, repos: map[string]bool{},
							first: fmt.Sprintf("%s/%s:%s", p.name, f.path, e.Pos)}
						groups[key] = g
					}
					g.files[p.name+"/"+f.path] = true
					g.repos[p.name] = true
				}
			}
		}
	}
	var gs []*group
	for _, g := range groups {
		gs = append(gs, g)
	}
	sort.Slice(gs, func(i, j int) bool {
		if len(gs[i].files) != len(gs[j].files) {
			return len(gs[i].files) > len(gs[j].files)
		}
		if gs[i].feature != gs[j].feature {
			return gs[i].feature < gs[j].feature
		}
		return gs[i].msg < gs[j].msg
	})
	fmt.Fprintf(sb, "## Cross-check with the parser\n\nThe parser, recovering after each declaration that fails, reported %d errors in all the files, library and tests. %d of them name a feature that the detector also found in the file.", nErrs, nErrs-nMissed)
	if len(gs) == 0 {
		fmt.Fprintf(sb, "\n\n")
		return
	}
	fmt.Fprintf(sb, " These %d weren't predicted, by message: an error with a feature is one the detector missed there, and one without is either a gap in the detector or a construct the parser rejects without naming it.\n\n", nMissed)
	fmt.Fprintf(sb, "| Error | Feature | Files | Repositories | First |\n|:---|:---|---:|---:|:---|\n")
	for _, g := range gs {
		fmt.Fprintf(sb, "| %s | %s | %d | %d | %s |\n", mdEscape(g.msg), g.feature, len(g.files), len(g.repos), g.first)
	}
	fmt.Fprintln(sb)
}

// reportCheckerCheck writes the checker cross-check: the unsupported
// features the checker reports in the library files, and whether the
// detector found them.
func reportCheckerCheck(sb *strings.Builder, ps []*project, done map[string]bool) {
	type count struct {
		feature  string
		files    int
		detected int
		repos    map[string]bool
		first    string
	}
	counts := map[string]*count{}
	for _, p := range ps {
		files := p.main().files
		sorted := append([]*file(nil), files...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })
		for _, f := range sorted {
			for feat := range f.checker {
				if done[feat] {
					continue
				}
				c := counts[feat]
				if c == nil {
					c = &count{feature: feat, repos: map[string]bool{}, first: p.name + "/" + f.path}
					counts[feat] = c
				}
				c.files++
				c.repos[p.name] = true
				if f.detected[feat] > 0 {
					c.detected++
				}
			}
		}
	}
	var cs []*count
	for _, c := range counts {
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].files != cs[j].files {
			return cs[i].files > cs[j].files
		}
		return cs[i].feature < cs[j].feature
	})
	fmt.Fprintf(sb, "## Cross-check with the checker\n\nkc's checker, run on each repository's library files together with only the standard library and `kotlin-reflect` on the class path, reports these unsupported features. Without the repositories' dependencies it can't resolve many names, and it skips the declarations the parser rejects, so it misses features; but where it reports one, the code has it. *Detected* counts the files where the detector found the feature too. A feature the detector misses isn't in the ranking above.\n\n")
	fmt.Fprintf(sb, "| Feature | Files | Detected | Repositories | First |\n|:---|---:|---:|---:|:---|\n")
	for _, c := range cs {
		fmt.Fprintf(sb, "| %s | %d | %d | %d | %s |\n", c.feature, c.files, c.detected, len(c.repos), c.first)
	}
	var failed []string
	for _, p := range ps {
		if p.checkerErr != "" {
			failed = append(failed, fmt.Sprintf("- %s: %s", p.name, mdEscape(p.checkerErr)))
		}
	}
	if len(failed) > 0 {
		fmt.Fprintf(sb, "\nThe checker failed on %d repositories, whose features are left out of this table:\n\n%s\n", len(failed), strings.Join(failed, "\n"))
	}

	// The checker's panics, by where they happen.
	type panicGroup struct {
		where, msg string
		files      int
		repos      map[string]bool
		first      string
	}
	groups := map[string]*panicGroup{}
	nCrashes := 0
	for _, p := range ps {
		for _, c := range p.crashes {
			nCrashes++
			key := c.where + "\x00" + c.msg
			g := groups[key]
			if g == nil {
				g = &panicGroup{where: c.where, msg: c.msg, repos: map[string]bool{}, first: p.name + "/" + c.file.path}
				groups[key] = g
			}
			g.files++
			g.repos[p.name] = true
		}
	}
	if nCrashes > 0 {
		var gs []*panicGroup
		for _, g := range groups {
			gs = append(gs, g)
		}
		sort.Slice(gs, func(i, j int) bool {
			if gs[i].files != gs[j].files {
				return gs[i].files > gs[j].files
			}
			return gs[i].where+gs[i].msg < gs[j].where+gs[j].msg
		})
		fmt.Fprintf(sb, "\nThe checker panics on %d files, which are left out of the table: when it panics on a set of files, the survey checks each half apart, down to the files it panics on. By where in kc:\n\n", nCrashes)
		fmt.Fprintf(sb, "| Where | Panic | Files | Repositories | First |\n|:---|:---|---:|---:|:---|\n")
		for _, g := range gs {
			fmt.Fprintf(sb, "| `%s` | %s | %d | %d | %s |\n", g.where, mdEscape(g.msg), g.files, len(g.repos), g.first)
		}
	}
	fmt.Fprintln(sb)
}

// reportChecker writes the unsupported features the checker reports in
// p's library files, for p's own report.
func reportChecker(sb *strings.Builder, p *project) {
	fmt.Fprintf(sb, "### Cross-check with the checker\n\n")
	if p.checkerErr != "" {
		fmt.Fprintf(sb, "The checker failed on the library files: %s\n", mdEscape(p.checkerErr))
		return
	}
	done := doneFeatures()
	files := append([]*file(nil), p.main().files...)
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	var rows []string
	for _, f := range files {
		var missed []string
		for feat := range f.checker {
			if !done[feat] && f.detected[feat] == 0 {
				missed = append(missed, feat)
			}
		}
		sort.Strings(missed)
		if len(missed) > 0 {
			rows = append(rows, fmt.Sprintf("| %s | %s |", f.path, strings.Join(missed, ", ")))
		}
	}
	fmt.Fprintf(sb, "The checker, run on the library files together, reports unsupported features that the detector didn't find in %d files.\n", len(rows))
	if len(rows) > 0 {
		fmt.Fprintf(sb, "\n| File | Features |\n|:---|:---|\n%s\n", strings.Join(rows, "\n"))
	}
	if len(p.crashes) > 0 {
		fmt.Fprintf(sb, "\nIt panics on %d files, which are left out:\n\n| File | Where | Panic |\n|:---|:---|:---|\n", len(p.crashes))
		cs := append([]*crash(nil), p.crashes...)
		sort.Slice(cs, func(i, j int) bool { return cs[i].file.path < cs[j].file.path })
		for _, c := range cs {
			fmt.Fprintf(sb, "| %s | `%s` | %s |\n", c.file.path, c.where, mdEscape(c.msg))
		}
	}
	fmt.Fprintln(sb)
}

// mdEscape escapes the characters of s that a Markdown table cell can't
// hold as they are.
func mdEscape(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ", "`", "'").Replace(s)
}
