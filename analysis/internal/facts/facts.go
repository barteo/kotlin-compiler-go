// Package facts lets package analysis/ka have package analysis build its
// facts from a check of its own: ka checks a module once, and with
// facts on, analysis's recorder hears the checker too, and analysis's
// builder writes the facts of the checked files (analysis.Result), as
// Analyze does. Package analysis sets the hooks when it is initialized.
package facts

import (
	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Recorder is analysis's recorder: what it hears of the checker.
type Recorder interface {
	check.Recorder
	check.OverrideRecorder
	check.AliasRecorder
}

var (
	// NewRecorder returns a new analysis recorder.
	NewRecorder func() Recorder
	// Build returns the *analysis.Result of the files lowered from
	// snaps, with provenances provs, which a check reported to rec, with
	// cp the class path the check read.
	Build func(snaps []*psi.Snapshot, files []*syntax.File, provs []*lower.Provenance, rec Recorder, cp *classpath.Path) any
)
