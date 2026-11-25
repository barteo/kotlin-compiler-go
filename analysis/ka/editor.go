package ka

import (
	"context"
	"fmt"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// EditorModel holds body-level source facts and the lazy library model from
// the same check. Close it when readers have finished with this revision.
type EditorModel struct {
	*Model
	Facts *analysis.EditorResult
}

// CheckEditor accepts parsed overlays so edits only require reparsing the
// changed document. KSP's signature-only check cache is independent.
func (s *Session) CheckEditor(ctx context.Context, snaps []*psi.Snapshot, sources []Source) (out *EditorModel, err error) {
	shared, err := s.library()
	if err != nil {
		return nil, err
	}
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("editor analysis: %v", v)
		}
		if err != nil {
			shared.release()
		}
	}()
	lib := shared.lib
	var files []*javasrc.File
	m := &Model{shared: shared}
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		text, err := src.text()
		if err != nil {
			return nil, err
		}
		f, err := javasrc.Parse(src.Path, text)
		if err != nil {
			m.Diagnostics = append(m.Diagnostics, Diagnostic{Path: src.Path, Line: 1, Col: 1, Message: err.Error()})
		}
		if f != nil {
			files = append(files, f)
		}
	}
	if len(files) > 0 {
		lib = lib.withJava(files)
	}
	m.lib = lib
	facts, err := analysis.CheckEditor(ctx, s.cfg, snaps, lib.cp, lib.lib, lib.java)
	if err != nil {
		return nil, err
	}
	return &EditorModel{Model: m, Facts: facts}, nil
}
