package ka

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
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

// JavaMethodDescriptor resolves a parsed Java method against the checked
// document's classpath, including other Java sources in the module.
func (m *EditorModel) JavaMethodDescriptor(class *javasrc.Class, method *javasrc.Method) string {
	java := m.lib.java
	if java == nil {
		java = driver.AddJavaSources(m.lib.cp, nil)
	}
	return java.MethodDescriptor(class, method)
}

// ImportNames returns public Kotlin classifiers and non-extension callables
// available under a simple name on this editor model's compile classpath.
func (m *EditorModel) ImportNames(ctx context.Context, name string) []string {
	lib := m.lib
	if lib == nil {
		return nil
	}
	var names []string
	for _, id := range lib.cp.ClassesNamed(name) {
		if ctx.Err() != nil {
			return nil
		}
		if d := m.Class(id); d != nil && d.Visibility == Public && d.Flags&FlagJava == 0 {
			names = append(names, strings.ReplaceAll(id, "/", "."))
		}
	}
	for pkg := range lib.parts() {
		if ctx.Err() != nil {
			return nil
		}
		p := m.Package(pkg)
		if p == nil {
			continue
		}
		for _, d := range p.Decls {
			if d.Name == name && d.Visibility == Public && d.Receiver == nil {
				if pkg != "" {
					names = append(names, pkg+"."+name)
				}
				break
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}
