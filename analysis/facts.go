package analysis

import (
	"github.com/barteo/kotlin-compiler-go/analysis/internal/facts"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// The hooks analysis/ka builds facts from its own check with.
func init() {
	facts.NewRecorder = func() facts.Recorder { return newRecorder() }
	facts.Build = func(snaps []*psi.Snapshot, files []*syntax.File, provs []*lower.Provenance, rec facts.Recorder, cp *classpath.Path) any {
		res := &Result{facts: map[ElementID]*Fact{}}
		b := newBuilder(res, snaps, files, provs, rec.(*recorder))
		b.cp = cp
		b.build()
		return res
	}
}
