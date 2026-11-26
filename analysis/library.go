package analysis

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Library is a module's class path, opened once for many analyses, as a
// language server analyzes a module again after each edit. The checker
// keeps what it reads of the class path's classes in the Library, so a
// later analysis doesn't read them again; analyses may run at the same
// time. Close releases it.
type Library struct {
	cfg   Config
	cp    *classpath.Path
	lib   *check.Library
	major uint16 // the class file version of cfg.JVMTarget
}

// OpenLibrary opens the class path of the module cfg describes.
func OpenLibrary(cfg Config) (*Library, error) {
	switch cfg.ExplicitAPIMode {
	case "", "disable", "warning", "strict":
	default:
		return nil, fmt.Errorf("analysis: invalid explicit API mode %q", cfg.ExplicitAPIMode)
	}
	major, ok := cf.TargetMajor(cfg.JVMTarget)
	if !ok {
		return nil, fmt.Errorf("analysis: unsupported JVM target %q: 1.8, or 9 to 25", cfg.JVMTarget)
	}
	cp, lib, err := driver.OpenLibrary(driverOptions(cfg))
	if err != nil {
		return nil, fmt.Errorf("analysis: %w", err)
	}
	return &Library{cfg: cfg, cp: cp, lib: lib, major: major}, nil
}

// Close releases the class path.
func (l *Library) Close() error { return l.cp.Close() }

// Config returns the configuration the library was opened with.
func (l *Library) Config() Config { return l.cfg }

// HasClass reports whether the class path has the class with the JVM
// internal name name, as java/lang/String.
func (l *Library) HasClass(name string) bool { return l.cp.Has(name) }

// JavaSource is a Java source file of the module, whose declarations the
// Kotlin sources see.
type JavaSource struct {
	Path, Text string
}

// Options configure Library.Analyze.
type Options struct {
	// Java are the module's Java sources.
	Java []JavaSource
	// Warnings adds the checker's warnings to the diagnostics.
	Warnings bool
	// ExtraWarnings adds, with Warnings, those of kotlinc's extra
	// checkers, which an IDE shows and kotlinc only with -Wextra: unused
	// variables and parameters, redundant modifiers, unreachable code.
	ExtraWarnings bool
	// Complete asks what the code could name at a completion's caret:
	// see Result.Completion.
	Complete *Complete
	// Declared are the paths of snapshots that are only declared: the
	// sources of modules this one depends on, which their own analyses
	// check. Their declarations are this module's to use, as a library's
	// are; they aren't checked, and have no facts and no diagnostics.
	Declared map[string]bool
}

// Analyze analyzes snapshots as the Kotlin sources of the library's
// module. Unlike AnalyzeArena it goes on where the sources have errors,
// as an editor needs:
//
//   - A file with syntax errors is checked as far as lowering recovers
//     it, leaving out the statements with errors, and has facts for what
//     is left. Its syntax errors are not among the diagnostics: they are
//     the snapshot's own (psi.Snapshot.Diagnostics). The checker's errors
//     on a line with a syntax error are left out too, as kotlinc leaves
//     them out.
//   - A construct kc doesn't support is a diagnostic with Code
//     "unsupported:<feature>", and the rest of the sources keep their
//     facts.
//   - A panic in the checker is an error, not a crash.
func (l *Library) Analyze(ctx context.Context, snapshots []*psi.Snapshot, opts Options) (res *Result, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, s := range snapshots {
		if s == nil {
			return nil, errors.New("analysis: nil snapshot")
		}
		if strings.HasSuffix(s.Path(), ".kts") {
			return nil, fmt.Errorf("%w: script %s", ErrUnsupported, s.Path())
		}
	}
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("analysis: internal error: %v\n%s", r, debug.Stack())
		}
	}()
	return l.analyze(ctx, snapshots, opts)
}

func (l *Library) analyze(ctx context.Context, snaps []*psi.Snapshot, opts Options) (*Result, error) {
	res, _, err := l.run(ctx, snaps, opts, false)
	return res, err
}

// lowered is a snapshot lowered for checking.
type lowered struct {
	file   *syntax.File
	prov   *lower.Provenance
	errs   []*syntax.Error
	broken bool         // lowered recovering from syntax errors
	lines  map[int]bool // lines with syntax errors
	// lost is set if lowering may have left out a declaration, rather
	// than statements of a closed body: the other files were checked
	// without it.
	lost bool
}

// lowerSnapshot lowers s, recovering from its syntax errors as kotlinc
// does.
func lowerSnapshot(s *psi.Snapshot) lowered {
	var lw lowered
	for _, d := range s.Diagnostics() {
		if line, _, err := s.Position(d.Range.Start); err == nil {
			if lw.lines == nil {
				lw.lines = map[int]bool{}
			}
			lw.lines[line] = true
		}
	}
	var errs []*syntax.Error
	if len(s.Diagnostics()) == 0 {
		lw.file, lw.prov, errs = lowerSafely(s, false)
		if len(errs) == 0 && lw.file != nil {
			return lw
		}
	}
	lw.broken = true
	var rerrs []*syntax.Error
	lw.file, lw.prov, rerrs = lowerSafely(s, true)
	if len(s.Diagnostics()) == 0 {
		// Errors psi's grammar doesn't find, as kotlinc's FIR builder
		// reports them, and constructs kc doesn't lower.
		lw.errs = errs
	} else {
		// Lowering's errors repeat psi's syntax errors, but for
		// constructs kc doesn't support away from them.
		for _, e := range rerrs {
			if e.Feature != "" && !lw.lines[e.Pos.Line] {
				lw.errs = append(lw.errs, e)
			}
		}
	}
	for _, e := range lw.errs {
		if lw.lines == nil {
			lw.lines = map[int]bool{}
		}
		lw.lines[e.Pos.Line] = true
	}
	lw.lost = lw.file == nil || lostDeclarations(s, lw.file)
	return lw
}

// lostDeclarations reports whether f, lowered from s recovering from
// errors, leaves out a declaration s has: a class, object, function,
// property, typealias or enum entry of the file or of a class body, or
// a property of a primary constructor.
func lostDeclarations(s *psi.Snapshot, f *syntax.File) bool {
	have := map[string]int{}
	var decls func(ds []syntax.Decl)
	decls = func(ds []syntax.Decl) {
		for _, d := range ds {
			switch d := d.(type) {
			case *syntax.FunDecl:
				have[d.Name]++
			case *syntax.PropDecl:
				have[d.Name]++
			case *syntax.TypeAliasDecl:
				have[d.Name]++
			case *syntax.ClassDecl:
				have[d.Name]++
				for _, p := range d.CtorParams {
					if p.IsProp {
						have[p.Name]++
					}
				}
				for _, e := range d.Entries {
					have[e.Name]++
				}
				decls(d.Members)
			}
		}
	}
	decls(f.Decls)
	want := map[string]int{}
	var walk func(n psi.NodeRef)
	walk = func(n psi.NodeRef) {
		for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
			switch c.NumericKind() {
			case psi.KindFUN, psi.KindPROPERTY, psi.KindTYPEALIAS, psi.KindENUM_ENTRY:
				want[c.Name()]++
			case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
				want[c.Name()]++
				for _, p := range c.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).Parameters() {
					if p.ChildKind(psi.KindVAL_KEYWORD).Valid() || p.ChildKind(psi.KindVAR_KEYWORD).Valid() {
						want[p.Name()]++
					}
				}
				walk(c.ChildKind(psi.KindCLASS_BODY))
			}
		}
	}
	walk(s.RootNode())
	for name, n := range want {
		if name != "" && have[name] < n {
			return true
		}
	}
	return false
}

// diagKind is what a file's diagnostic is, for the filters of the
// module's diagnostics (finish).
type diagKind uint8

const (
	diagLower   diagKind = iota // lowering's
	diagError                   // the checker's
	diagBackend                 // the checker's, of kotlinc's JVM backend
	diagWarning                 // the checker's
)

// fileDiag is a diagnostic of one file.
type fileDiag struct {
	Diagnostic
	kind diagKind
}

// checkerDiag returns the fileDiag of the checker's error e.
func checkerDiag(path string, e *syntax.Error) fileDiag {
	k := diagError
	switch {
	case e.IsWarning():
		k = diagWarning
	case check.IsBackendError(e):
		k = diagBackend
	}
	return fileDiag{diagnostic(path, e), k}
}

// finish returns the module's diagnostics: extra, then each file's, as
// kotlinc reports them: the JVM backend's errors only if there are no
// others, and of the warnings of a module with syntax errors only those
// check.RecoveredWarning says.
func finish(extra []Diagnostic, files [][]fileDiag, broken bool) []Diagnostic {
	others := false
	for _, ds := range files {
		for _, d := range ds {
			others = others || d.kind == diagError
		}
	}
	out := slices.Clone(extra)
	for _, ds := range files {
		for _, d := range ds {
			if d.kind == diagBackend && others || d.kind == diagWarning && broken && !check.RecoveredWarning(d.Name) {
				continue
			}
			out = append(out, d.Diagnostic)
		}
	}
	return out
}

// run analyzes snaps; with keep, it also returns the state a Session's
// incremental update starts from, or nil if one can't.
func (l *Library) run(ctx context.Context, snaps []*psi.Snapshot, opts Options, keep bool) (*Result, *state, error) {
	jobs := max(l.cfg.Jobs, 1)
	n := len(snaps)
	low := make([]lowered, n)
	parallel(n, jobs, func(i int) { low[i] = lowerSnapshot(snaps[i]) })
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	declOnly := declaredOf(snaps, opts.Declared)
	isDeclared := func(i int) bool { return declOnly != nil && declOnly[i] }
	stateful := keep && opts.Complete == nil

	res := &Result{}
	var (
		kept    []int // the indexes of the snapshots checked
		files   []*syntax.File
		provs   []*lower.Provenance
		extra   []Diagnostic
		broken  bool
		perFile = make([][]fileDiag, n)
		dropped bool
	)
	for i, lw := range low {
		if !isDeclared(i) {
			for _, e := range lw.errs {
				perFile[i] = append(perFile[i], fileDiag{diagnostic(snaps[i].Path(), e), diagLower})
			}
		}
		if lw.file == nil {
			dropped = true
			continue
		}
		broken = broken || lw.broken && !isDeclared(i)
		kept = append(kept, i)
		files = append(files, lw.file)
		provs = append(provs, lw.prov)
	}
	// A file whose facade class another file already has is left out, as
	// the checker assumes there are no duplicates; the others go on.
	for {
		d := duplicateClasses(files)
		if d == nil {
			break
		}
		extra = append(extra, *d)
		dropped = true
		j := slices.IndexFunc(files, func(f *syntax.File) bool { return f.Path == d.Path })
		if j < 0 {
			break
		}
		kept = slices.Delete(kept, j, j+1)
		files = slices.Delete(files, j, j+1)
		provs = slices.Delete(provs, j, j+1)
	}
	keptSnaps := make([]*psi.Snapshot, len(kept))
	for j, i := range kept {
		keptSnaps[j] = snaps[i]
	}

	cp, lib, javaDiags := l.withJava(opts.Java, files)
	extra = append(extra, javaDiags...)

	rec := newRecorder()
	var r check.Recorder = rec
	if stateful {
		r = lookupRecorder{rec}
	}
	mode, _ := check.ParseAPIMode(l.cfg.ExplicitAPIMode)
	var probe *check.Probe
	if cp := opts.Complete; cp != nil {
		for _, f := range files {
			if f.Path == cp.Path {
				probe = &check.Probe{File: f, Name: Placeholder, Match: cp.Match, Importable: cp.Importable}
			}
		}
	}
	copts := check.Options{
		ExplicitAPI: mode, Warnings: opts.Warnings, ExtraWarnings: opts.Warnings && opts.ExtraWarnings, Raw: true, JVMTarget: l.major, Probe: probe,
	}
	switch {
	case probe != nil:
		// A completion needs the bodies of its file only: the other files'
		// omitted types are inferred as it needs them.
		copts.Only = map[*syntax.File]bool{probe.File: true}
	case declOnly != nil:
		copts.Only = map[*syntax.File]bool{}
		for j, i := range kept {
			if !isDeclared(i) {
				copts.Only[files[j]] = true
			}
		}
	}
	errs, err := check.CheckOptions(ctx, files, lib, jobs, r, copts)
	if err != nil {
		return nil, nil, err
	}
	index := make(map[string]int, n)
	for i, s := range snaps {
		index[s.Path()] = i
	}
	whole := true // every error is one file's
	for _, e := range errs {
		i, ok := index[e.File]
		if !ok {
			whole = false
			d := checkerDiag(e.File, e)
			if !(d.kind == diagWarning && broken && !check.RecoveredWarning(d.Name)) {
				extra = append(extra, d.Diagnostic)
			}
			continue
		}
		if isDeclared(i) || !e.IsWarning() && low[i].lines[e.Pos.Line] {
			continue
		}
		perFile[i] = append(perFile[i], checkerDiag(snaps[i].Path(), e))
	}
	b := newBuilder(res, keptSnaps, files, provs, rec)
	b.cp = cp
	b.probe = probe
	if declOnly != nil {
		b.broken = make([]bool, len(kept))
		for j, i := range kept {
			b.broken[j] = isDeclared(i)
		}
	}
	b.build()
	res.diags = finish(extra, perFile, broken)
	if probe != nil {
		res.diags = slices.DeleteFunc(res.diags, placeholderDiagnostic)
	}
	if !stateful {
		return res, nil, nil
	}
	switch {
	case !whole:
		return res, nil, wholeNext("an error of no file")
	case dropped:
		return res, nil, wholeNext("a file left out")
	}
	st := newState(res, snaps, b.syms, opts)
	if st == nil {
		return res, nil, nil
	}
	st.diags, st.broken, st.lost, st.declared, st.extra = perFile, make([]bool, n), make([]bool, n), declOnly, extra
	for i := range low {
		st.broken[i] = low[i].broken && !isDeclared(i)
		st.lost[i] = low[i].lost
	}
	parallel(n, jobs, func(i int) {
		st.lookups[i] = rec.fileLookups(files[i])
		st.decls[i] = declared(snaps[i])
	})
	return res, st, nil
}

// wholeNext reports why an update leaves no state for the next to start
// from to traceWhole.
func wholeNext(reason string) error {
	if traceWhole != nil {
		traceWhole("no state: " + reason)
	}
	return nil
}

// withJava returns the class path and library to check files against:
// the library's, with java's classes if there are any, and java's
// errors.
func (l *Library) withJava(java []JavaSource, files []*syntax.File) (*classpath.Path, *check.Library, []Diagnostic) {
	if len(java) == 0 {
		return l.cp, l.lib, nil
	}
	var diags []Diagnostic
	var parsed []*javasrc.File
	for _, src := range java {
		f, err := javasrc.Parse(src.Path, src.Text)
		if f != nil {
			parsed = append(parsed, f)
		}
		if err != nil {
			diags = append(diags, Diagnostic{Path: src.Path, Line: 1, Column: 1, Code: "source_error", Severity: "error", Message: err.Error()})
		}
	}
	js := driver.AddJavaSources(l.cp, parsed)
	js.SetKotlin(files)
	return js.Path, l.lib.WithSources(js.Path), diags
}

// declaredOf returns which snapshots paths says are only declared, or
// nil if none is.
func declaredOf(snaps []*psi.Snapshot, paths map[string]bool) []bool {
	if len(paths) == 0 {
		return nil
	}
	out := make([]bool, len(snaps))
	any := false
	for i, s := range snaps {
		out[i] = paths[s.Path()]
		any = any || out[i]
	}
	if !any {
		return nil
	}
	return out
}

// lowerSafely lowers s, recovering if recovering is set. A file lowering
// has no case for is left out (nil), as the driver leaves it out.
func lowerSafely(s *psi.Snapshot, recovering bool) (f *syntax.File, prov *lower.Provenance, errs []*syntax.Error) {
	defer func() {
		if recover() != nil {
			f, prov, errs = nil, nil, nil
		}
	}()
	if recovering {
		return lower.FileRecovering(s)
	}
	return lower.File(s)
}
