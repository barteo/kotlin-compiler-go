package analysis

import (
	"context"
	"fmt"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// EditorResult owns the facts of one editor revision. The checker no
// longer retains them. Readers share this data without deep-copying each
// symbol, call signature and type tree to build their indexes.
type EditorResult struct {
	Facts       []Fact
	Symbols     []Symbol
	Diagnostics []Diagnostic
}

func editorResult(r *Result) *EditorResult {
	out := &EditorResult{Facts: make([]Fact, 0, len(r.order)), Symbols: r.symbols, Diagnostics: r.diags}
	for _, id := range r.order {
		out.Facts = append(out.Facts, *r.facts[id])
	}
	return out
}

// CheckEditor analyzes the current snapshots using a session's open library.
// Recovering lowering discards malformed statements while preserving the
// provenance of the declarations and references that still resolve.
func CheckEditor(ctx context.Context, cfg Config, snaps []*psi.Snapshot, cp *classpath.Path, lib *check.Library, java *driver.JavaSources) (*EditorResult, error) {
	files := make([]*syntax.File, len(snaps))
	provs := make([]*lower.Provenance, len(snaps))
	res := &Result{facts: map[ElementID]*Fact{}}
	recovered := false
	for i, s := range snaps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var es []*syntax.Error
		files[i], provs[i], es = lower.FileRecovering(s)
		for _, e := range es {
			res.diags = append(res.diags, diagnostic(s.Path(), e))
		}
		recovered = recovered || len(es) > 0 || len(s.Diagnostics()) > 0
	}
	if d := duplicateClasses(files); d != nil {
		res.diags = append(res.diags, *d)
		return editorResult(res), nil
	}
	if java != nil {
		java.SetKotlin(files)
	}
	mode, ok := check.ParseAPIMode(cfg.ExplicitAPIMode)
	if !ok {
		return nil, fmt.Errorf("invalid explicit API mode %q", cfg.ExplicitAPIMode)
	}
	rec := newRecorder()
	es, err := check.CheckOptions(ctx, files, lib, max(1, cfg.Jobs), rec, check.Options{Warnings: true, Recovered: recovered, ExplicitAPI: mode, NotMultiplatform: true})
	if err != nil {
		return nil, err
	}
	for _, e := range es {
		d := diagnostic(e.File, e)
		if e.Name != "" {
			d.Code = e.Name
		}
		d.Warning = e.IsWarning()
		res.diags = append(res.diags, d)
	}
	b := newBuilder(res, snaps, files, provs, rec)
	b.editor = true
	b.cp = cp
	b.build()
	return editorResult(res), nil
}
