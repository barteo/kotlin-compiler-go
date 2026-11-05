// Package ka is the Go half of kc's implementation of the Kotlin Analysis
// API (docs/analysis-api.md): a session over a module's sources whose
// snapshots, one per KSP round, answer the queries the JVM half (jvm/kc-aa)
// makes through libkc.
//
// A Session holds a module's configuration. Update checks the module's
// sources as they are now and returns a Model, which is immutable and safe
// for concurrent use. A Model so far knows the declarations of the sources
// and their annotations: what the M0.4 vertical slice needs.
package ka

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Config describes a module: its class path, JDK and name, as for package
// analysis.
type Config = analysis.Config

// Session is a module whose sources Update checks.
type Session struct {
	cfg Config
}

// Open returns a session for the module cfg describes.
func Open(cfg Config) *Session { return &Session{cfg: cfg} }

// Source is a source file: its path and, if Text is nil, the file at Path
// is read.
type Source struct {
	Path string
	Text *string
}

// Kind is the kind of a declaration.
type Kind uint8

// The kinds of declarations. The numbers are part of libkc's ABI: append
// only.
const (
	KindClass Kind = iota + 1
	KindInterface
	KindObject
	KindEnumClass
	KindAnnotationClass
	KindEnumEntry
	KindFunction
	KindProperty
	KindTypeAlias
	KindConstructor
	KindValueParameter
	KindTypeParameter
)

// Visibility is a declaration's visibility. The numbers are part of
// libkc's ABI.
type Visibility uint8

const (
	Public Visibility = iota
	Private
	Protected
	Internal
)

// Model is the declarations of a module's sources as they were when Update
// made it.
type Model struct {
	Files []*File
	// Diagnostics are the errors in the sources, in file order. A Model
	// has the facts of sources with errors too: a declaration with a
	// syntax error is left out (a class keeps its other members), and an
	// unresolved annotation has no class ID.
	Diagnostics []Diagnostic
}

// Diagnostic is an error in the sources.
type Diagnostic struct {
	Path      string
	Line, Col int
	Message   string
	// Feature names the Kotlin feature kc doesn't support, if that is
	// the error.
	Feature string
}

// File is a source file.
type File struct {
	Path    string
	Package string // dotted; "" for the default package
	// Snapshot is the file's parse; nodes are its psi node IDs.
	Snapshot    *psi.Snapshot
	Annotations []Annotation // @file: annotations
	Decls       []*Decl      // top-level declarations, in source order
}

// Decl is a declaration in the sources.
type Decl struct {
	Kind Kind
	Name string
	// Node is the psi node that declares it: a CLASS, OBJECT_DECLARATION,
	// FUN, PROPERTY, TYPEALIAS, PRIMARY_CONSTRUCTOR, SECONDARY_CONSTRUCTOR,
	// VALUE_PARAMETER, TYPE_PARAMETER or ENUM_ENTRY.
	Node psi.NodeID
	// ClassID is a class's class ID, as Kotlin's ClassId.asString():
	// a/b/Outer.Inner.
	ClassID     string
	Visibility  Visibility
	Companion   bool
	Data        bool
	Annotations []Annotation
	TypeParams  []*Decl
	// Params are a function's or constructor's value parameters.
	Params []*Decl
	// Members are a class's declared members: constructors, properties
	// (those of its primary constructor's val and var parameters too),
	// functions, nested classes and enum entries, in source order with
	// the primary constructor first.
	Members []*Decl
}

// Annotation is an annotation in the sources.
type Annotation struct {
	// ClassID is its class's class ID, or "" if the checker didn't
	// resolve it.
	ClassID string
	Node    psi.NodeID // the ANNOTATION_ENTRY
	Target  string     // the use-site target as written: get, field, file...; or ""
}

// Update checks the module's sources and returns their model. Errors in
// the sources don't stop it: they are the Model's Diagnostics.
func (s *Session) Update(ctx context.Context, sources []Source) (*Model, error) {
	snaps := make([]*psi.Snapshot, len(sources))
	for i, src := range sources {
		var text string
		if src.Text != nil {
			text = *src.Text
		} else {
			b, err := os.ReadFile(src.Path)
			if err != nil {
				return nil, fmt.Errorf("ka: %w", err)
			}
			text = string(b)
		}
		snaps[i] = psi.Parse(src.Path, text)
	}
	jobs := max(s.cfg.Jobs, 1)
	cp, lib, err := driver.OpenLibrary(driver.Options{
		JDKHome: s.cfg.JDKHome, NoJDK: s.cfg.NoJDK, ClassPath: s.cfg.ClassPath,
		ModuleName: s.cfg.ModuleName, Jobs: jobs,
	})
	if err != nil {
		return nil, fmt.Errorf("ka: %w", err)
	}
	defer cp.Close()

	m := &Model{}
	files, provs, broken := lowerAll(snaps, m)
	rec, checkErrs, err := checkFiles(ctx, files, lib, jobs)
	if _, crashed := err.(checkPanic); crashed && len(broken) > 0 {
		// The checker can't take a file with syntax errors: check the
		// others, and model the broken ones from syntax alone.
		files, provs, _ = lowerAll(snaps, nil)
		var clean []*syntax.File
		for i, f := range files {
			if !broken[i] {
				clean = append(clean, f)
			}
		}
		rec, checkErrs, err = checkFiles(ctx, clean, lib, jobs)
	}
	if err != nil {
		return nil, err
	}
	for _, e := range checkErrs {
		m.Diagnostics = append(m.Diagnostics, diagnostic(e.File, e))
	}
	slices.SortStableFunc(m.Diagnostics, func(a, b Diagnostic) int {
		return cmp.Or(cmp.Compare(index(snaps, a.Path), index(snaps, b.Path)),
			cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	for i, f := range files {
		b := &fileBuilder{f: f, snap: snaps[i], prov: provs[i], annots: rec.annots[f]}
		mf := &File{Path: snaps[i].Path(), Package: f.Package, Snapshot: snaps[i]}
		mf.Annotations = b.annotations(f.Annotations)
		pkg := strings.ReplaceAll(f.Package, ".", "/")
		for _, d := range f.Decls {
			if md := b.decl(d, pkg+"/"); md != nil {
				mf.Decls = append(mf.Decls, md)
			}
		}
		m.Files = append(m.Files, mf)
	}
	return m, nil
}

// lowerAll lowers snaps, adding their errors to m if it isn't nil, and
// reports which have errors.
func lowerAll(snaps []*psi.Snapshot, m *Model) ([]*syntax.File, []*lower.Provenance, map[int]bool) {
	files := make([]*syntax.File, len(snaps))
	provs := make([]*lower.Provenance, len(snaps))
	broken := map[int]bool{}
	for i, snap := range snaps {
		var errs []*syntax.Error
		files[i], provs[i], errs = lower.File(snap)
		if len(errs) > 0 {
			broken[i] = true
		}
		if m != nil {
			for _, e := range errs {
				m.Diagnostics = append(m.Diagnostics, diagnostic(snap.Path(), e))
			}
		}
	}
	return files, provs, broken
}

// checkPanic is a checker crash, which only sources with syntax errors are
// expected to cause.
type checkPanic struct{ v any }

func (p checkPanic) Error() string { return fmt.Sprintf("ka: checker panic: %v", p.v) }

func checkFiles(ctx context.Context, files []*syntax.File, lib *check.Library, jobs int) (rec *recorder, errs []*syntax.Error, err error) {
	rec = &recorder{annots: map[*syntax.File]map[syntax.Pos]*types.ClassSym{}}
	defer func() {
		if v := recover(); v != nil {
			err = checkPanic{v}
		}
	}()
	errs, err = check.CheckContext(ctx, files, lib, jobs, rec)
	return rec, errs, err
}

func index(snaps []*psi.Snapshot, path string) int {
	return slices.IndexFunc(snaps, func(s *psi.Snapshot) bool { return s.Path() == path })
}

func diagnostic(path string, e *syntax.Error) Diagnostic {
	if e.File != "" {
		path = e.File
	}
	return Diagnostic{Path: path, Line: e.Pos.Line, Col: e.Pos.Col, Message: e.Msg, Feature: e.Feature}
}

// recorder keeps the classes the checker resolves annotations to; the
// other reports aren't needed yet.
type recorder struct {
	annots map[*syntax.File]map[syntax.Pos]*types.ClassSym
}

func (r *recorder) Annotation(f *syntax.File, a *syntax.Annotation, cls *types.ClassSym) {
	m := r.annots[f]
	if m == nil {
		m = map[syntax.Pos]*types.ClassSym{}
		r.annots[f] = m
	}
	m[a.P] = cls
}
func (*recorder) When(*syntax.When, bool, bool)       {}
func (*recorder) Stmt(syntax.Stmt, bool)              {}
func (*recorder) Unreachable(syntax.Stmt)             {}
func (*recorder) Elvis(*syntax.Binary, bool)          {}
func (*recorder) TypeRef(*syntax.TypeRef, types.Type) {}
func (*recorder) StmtType(syntax.Expr, types.Type)    {}

type fileBuilder struct {
	f      *syntax.File
	snap   *psi.Snapshot
	prov   *lower.Provenance
	annots map[syntax.Pos]*types.ClassSym
}

func (b *fileBuilder) annotations(as []syntax.Annotation) []Annotation {
	var out []Annotation
	for i := range as {
		a := &as[i]
		if a.Implicit {
			continue
		}
		cls := a.Class
		if cls == nil {
			cls = a.Source
		}
		if cls == nil {
			cls = b.annots[a.P]
		}
		out = append(out, Annotation{ClassID: classID(cls), Node: b.prov.Annotation(a.P), Target: a.Target})
	}
	return out
}

// classID returns the class ID of c, or "" for nil.
func classID(c *types.ClassSym) string {
	if c == nil {
		return ""
	}
	if c.KotlinName != "" {
		return c.KotlinName
	}
	pkg, rel := "", c.Name
	if i := strings.LastIndexByte(c.Name, '/'); i >= 0 {
		pkg, rel = c.Name[:i+1], c.Name[i+1:]
	}
	return pkg + strings.ReplaceAll(rel, "$", ".")
}

func visibility(v syntax.Visibility) Visibility {
	switch v {
	case syntax.Private:
		return Private
	case syntax.Protected:
		return Protected
	case syntax.Internal:
		return Internal
	}
	return Public
}

// decl returns the model of declaration d, whose class ID, if it is a
// class, starts with prefix: a/b/ or a/b/Outer.; nil for a declaration the
// model doesn't have yet.
func (b *fileBuilder) decl(d syntax.Decl, prefix string) *Decl {
	switch d := d.(type) {
	case *syntax.ClassDecl:
		return b.class(d, prefix)
	case *syntax.FunDecl:
		md := &Decl{Kind: KindFunction, Name: d.Name, Node: b.prov.Node(d),
			Visibility: visibility(d.Visibility), Annotations: b.annotations(d.Annotations)}
		md.TypeParams = b.typeParams(d.TypeParams)
		md.Params = b.params(d.Params)
		return md
	case *syntax.PropDecl:
		md := &Decl{Kind: KindProperty, Name: d.Name, Node: b.prov.Node(d),
			Visibility: visibility(d.Visibility), Annotations: b.annotations(d.Annotations)}
		md.TypeParams = b.typeParams(d.TypeParams)
		return md
	case *syntax.TypeAliasDecl:
		md := &Decl{Kind: KindTypeAlias, Name: d.Name, Node: b.prov.Node(d),
			ClassID: prefix + d.Name, Visibility: visibility(d.Visibility),
			Annotations: b.annotations(d.Annotations)}
		md.TypeParams = b.typeParams(d.TypeParams)
		return md
	case *syntax.SecondaryCtor:
		md := &Decl{Kind: KindConstructor, Name: "<init>", Node: b.prov.Node(d),
			Visibility: visibility(d.Visibility), Annotations: b.annotations(d.Annotations)}
		md.Params = b.params(d.Params)
		return md
	}
	return nil
}

func (b *fileBuilder) class(d *syntax.ClassDecl, prefix string) *Decl {
	md := &Decl{Kind: KindClass, Name: d.Name, Node: b.prov.Node(d), ClassID: prefix + d.Name,
		Visibility: visibility(d.Visibility), Companion: d.Companion, Data: d.Data,
		Annotations: b.annotations(d.Annotations)}
	switch {
	case d.Annotation:
		md.Kind = KindAnnotationClass
	case d.Interface:
		md.Kind = KindInterface
	case d.Object:
		md.Kind = KindObject
	case d.Enum:
		md.Kind = KindEnumClass
	}
	md.TypeParams = b.typeParams(d.TypeParams)
	if !d.Interface && !d.Object {
		// The PRIMARY_CONSTRUCTOR, if written.
		pc := b.snap.Node(md.Node).ChildKind(psi.KindPRIMARY_CONSTRUCTOR).ID()
		ctor := &Decl{Kind: KindConstructor, Name: "<init>", Node: pc,
			Visibility: visibility(d.CtorMods.Visibility), Annotations: b.annotations(d.CtorMods.Annotations)}
		for _, p := range d.CtorParams {
			ctor.Params = append(ctor.Params, &Decl{Kind: KindValueParameter, Name: p.Name,
				Node: b.prov.Node(p), Annotations: b.annotations(p.Modifiers.Annotations)})
		}
		md.Members = append(md.Members, ctor)
		for _, p := range d.CtorParams {
			if p.IsProp {
				md.Members = append(md.Members, &Decl{Kind: KindProperty, Name: p.Name,
					Node: b.prov.Node(p), Visibility: visibility(p.Visibility),
					Annotations: b.annotations(p.Modifiers.Annotations)})
			}
		}
	}
	for _, e := range d.Entries {
		md.Members = append(md.Members, &Decl{Kind: KindEnumEntry, Name: e.Name, Node: b.prov.Node(e),
			ClassID: md.ClassID + "." + e.Name, Annotations: b.annotations(e.Annotations)})
	}
	for _, m := range d.Members {
		if c, ok := m.(*syntax.ClassDecl); ok && c.EnumEntry {
			continue // the body of an enum entry, which is among Entries
		}
		if mm := b.decl(m, md.ClassID+"."); mm != nil {
			md.Members = append(md.Members, mm)
		}
	}
	return md
}

func (b *fileBuilder) typeParams(tps []*syntax.TypeParam) []*Decl {
	var out []*Decl
	for _, tp := range tps {
		out = append(out, &Decl{Kind: KindTypeParameter, Name: tp.Name, Node: b.prov.Node(tp),
			Annotations: b.annotations(tp.Annotations)})
	}
	return out
}

func (b *fileBuilder) params(ps []*syntax.Param) []*Decl {
	var out []*Decl
	for _, p := range ps {
		out = append(out, &Decl{Kind: KindValueParameter, Name: p.Name, Node: b.prov.Node(p),
			Annotations: b.annotations(p.Annotations)})
	}
	return out
}
