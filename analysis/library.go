package analysis

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/check"
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
	cfg Config
	cp  *classpath.Path
	lib *check.Library
}

// OpenLibrary opens the class path of the module cfg describes.
func OpenLibrary(cfg Config) (*Library, error) {
	switch cfg.ExplicitAPIMode {
	case "", "disable", "warning", "strict":
	default:
		return nil, fmt.Errorf("analysis: invalid explicit API mode %q", cfg.ExplicitAPIMode)
	}
	cp, lib, err := driver.OpenLibrary(driver.Options{
		JDKHome: cfg.JDKHome, NoJDK: cfg.NoJDK, ClassPath: cfg.ClassPath, FriendPaths: cfg.FriendPaths,
		ModuleName: cfg.ModuleName, Jobs: max(cfg.Jobs, 1),
	})
	if err != nil {
		return nil, fmt.Errorf("analysis: %w", err)
	}
	return &Library{cfg: cfg, cp: cp, lib: lib}, nil
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
	jobs := max(l.cfg.Jobs, 1)
	n := len(snaps)
	type lowered struct {
		file   *syntax.File
		prov   *lower.Provenance
		errs   []*syntax.Error
		broken bool
		lines  map[int]bool // lines with syntax errors
	}
	low := make([]lowered, n)
	parallel(n, jobs, func(i int) {
		s := snaps[i]
		lw := &low[i]
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
				return
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
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	res := &Result{facts: map[ElementID]*Fact{}}
	var (
		keptSnaps []*psi.Snapshot
		files     []*syntax.File
		provs     []*lower.Provenance
		broken    bool
		lines     = map[string]map[int]bool{}
	)
	for i, lw := range low {
		for _, e := range lw.errs {
			res.diags = append(res.diags, diagnostic(snaps[i].Path(), e))
		}
		if lw.lines != nil {
			lines[snaps[i].Path()] = lw.lines
		}
		if lw.file == nil {
			continue
		}
		broken = broken || lw.broken
		keptSnaps = append(keptSnaps, snaps[i])
		files = append(files, lw.file)
		provs = append(provs, lw.prov)
	}
	if d := duplicateClasses(files); d != nil {
		res.diags = append(res.diags, *d)
		return res, nil
	}

	cp, lib := l.cp, l.lib
	var java *driver.JavaSources
	if len(opts.Java) > 0 {
		var parsed []*javasrc.File
		for _, src := range opts.Java {
			f, err := javasrc.Parse(src.Path, src.Text)
			if f != nil {
				parsed = append(parsed, f)
			}
			if err != nil {
				res.diags = append(res.diags, Diagnostic{Path: src.Path, Line: 1, Column: 1, Code: "source_error", Severity: "error", Message: err.Error()})
			}
		}
		java = driver.AddJavaSources(l.cp, parsed)
		java.SetKotlin(files)
		cp, lib = java.Path, l.lib.WithSources(java.Path)
	}

	rec := newRecorder()
	mode, _ := check.ParseAPIMode(l.cfg.ExplicitAPIMode)
	errs, err := check.CheckOptions(ctx, files, lib, jobs, rec, check.Options{
		ExplicitAPI: mode, Warnings: opts.Warnings, Recovered: broken,
	})
	if err != nil {
		return nil, err
	}
	for _, e := range errs {
		if !e.IsWarning() && lines[e.File][e.Pos.Line] {
			continue
		}
		res.diags = append(res.diags, diagnostic(e.File, e))
	}
	b := newBuilder(res, keptSnaps, files, provs, rec)
	b.cp = cp
	b.build()
	return res, nil
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
