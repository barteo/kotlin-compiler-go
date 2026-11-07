// Package ka is the Go half of kc's implementation of the Kotlin Analysis
// API (docs/analysis-api.md): a session over a module's sources whose
// snapshots, one per KSP round, answer the queries the JVM half (jvm/kc-aa)
// makes through libkc. A Model reads the class path's classes on demand
// (Model.Class) until it is closed.
//
// A Session holds a module's configuration. Update checks the module's
// sources as they are now and returns a Model, which is immutable and safe
// for concurrent use. A Model so far knows the declarations of the sources
// and their annotations: what the M0.4 vertical slice needs.
package ka

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Config describes a module: its class path, JDK and name, as for package
// analysis.
type Config = analysis.Config

// Session is a module whose sources Update checks. Its updates share the
// class path's classes that they read, as do the sessions of a process
// whose configurations and jars are the same; Close releases them.
type Session struct {
	cfg Config

	mu  sync.Mutex
	lib *shared // the library of the last Update, unless it has Java sources
}

// Close releases the class path the session's updates read. The models
// it returned can still be used until they are closed.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lib == nil {
		return nil
	}
	err := s.lib.release()
	s.lib = nil
	return err
}

// library returns the library for an update, which the caller must
// release.
func (s *Session) library() (*shared, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lib != nil {
		s.lib.retain()
		return s.lib, nil
	}
	l, err := acquire(s.cfg)
	if err != nil {
		return nil, err
	}
	if !l.lib.cp.HasSources() {
		l.retain()
		s.lib = l
	}
	return l, nil
}

// Open returns a session for the module cfg describes.
func Open(cfg Config) *Session { return &Session{cfg: cfg} }

// Source is a source file: its path and, if Text is nil, the file at Path
// is read.
type Source struct {
	Path string
	Text *string
}

// text returns the source's text.
func (src Source) text() (string, error) {
	if src.Text != nil {
		return *src.Text, nil
	}
	b, err := os.ReadFile(src.Path)
	if err != nil {
		return "", fmt.Errorf("ka: %w", err)
	}
	return string(b), nil
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
	// KindJavaField is a Java class's field.
	KindJavaField
	// KindDestructuring is a local destructuring declaration, val (a, b)
	// = e: Params are its entries, local properties, and Annotations
	// the declaration's own.
	KindDestructuring
)

// Visibility is a declaration's visibility. The numbers are part of
// libkc's ABI.
type Visibility uint8

const (
	Public Visibility = iota
	Private
	Protected
	Internal
	// PackagePrivate is a Java declaration's package visibility.
	PackagePrivate
	// Local is a local declaration's.
	Local
)

// Modality is a declaration's modality. The numbers are part of libkc's
// ABI.
type Modality uint8

const (
	ModalityFinal Modality = iota
	ModalityOpen
	ModalityAbstract
	ModalitySealed
)

// Flags are a declaration's modifiers, as a bit set. The bits are part of
// libkc's ABI: append only.
type Flags uint32

const (
	FlagCompanion Flags = 1 << iota
	FlagData
	FlagInner
	FlagValue
	FlagFun // a fun interface
	FlagExpect
	FlagActual
	FlagInline
	FlagSuspend
	FlagOperator
	FlagInfix
	FlagTailrec
	FlagExternal
	FlagConst
	FlagLateinit
	FlagMutable // a var, or a val or var constructor parameter's property
	FlagVararg
	FlagNoinline
	FlagCrossinline
	FlagHasDefault // a value parameter with a default value
	FlagOverride
	FlagReified
	FlagJava    // a declaration of a Java class
	FlagStatic  // a Java class's static member or nested class, or a library Kotlin class's nested class that isn't inner
	FlagPrimary // a class's primary constructor
	// FlagBackingField is set on a property that has a backing field: one
	// with a default accessor or one that uses field, not abstract and not
	// delegated; in a library, one whose metadata names its field.
	FlagBackingField
	// FlagJavaSource is set on a class of the module's Java sources.
	FlagJavaSource
	// FlagEntryBody is set on a source enum entry with a body; its
	// Members are the body's declarations.
	FlagEntryBody
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

	lib       *library // the class path's classes; see Class
	shared    *shared  // lib, which Close releases
	closeOnce sync.Once
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
	// Lookups are the names the file's code looks up in packages and
	// classes, for incremental processing, sorted: what Kotlin's lookup
	// tracker records for it, and more. A file with syntax errors has
	// none.
	Lookups []Lookup
}

// Lookup is a name looked up in a scope: a package, a.b ("" for the
// default package), or a class, a.b.Outer.Inner.
type Lookup struct {
	Scope string
	Kind  LookupKind
	Name  string
}

// LookupKind is the kind of scope a Lookup searched.
type LookupKind uint8

const (
	PackageLookup LookupKind = iota // a package
	ClassLookup                     // a class: its members and nested classes
)

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
	ClassID    string
	Visibility Visibility
	Modality   Modality
	Flags      Flags
	// Variance is a type parameter's declaration-site variance.
	Variance    Variance
	Annotations []Annotation
	// Type is a property's, a value parameter's (a vararg's element
	// type), a function's return type, or a type alias's expansion; nil
	// for the others, and where the checker didn't get to it.
	Type *Type
	// Receiver is an extension's receiver type.
	Receiver *Type
	// Supertypes are a class's direct supertypes, as written (kotlin/Any
	// if none is), and Bounds a type parameter's upper bounds (none for
	// Any?).
	Supertypes []*Type
	Bounds     []*Type
	TypeParams []*Decl
	// Params are a function's or constructor's value parameters.
	Params []*Decl
	// Members are a class's declared members: constructors, properties
	// (those of its primary constructor's val and var parameters too),
	// functions, nested classes and enum entries, in source order with
	// the primary constructor first.
	Members []*Decl
	// Default is the default value of an annotation class's
	// constructor's parameter, if it has one, and a const val's value.
	Default *Value
	// Locals are the local variables, destructuring declarations,
	// functions and classes written directly in a function's or a
	// constructor's body, or in a property's accessors' bodies, in source
	// order: what an Analysis API client lists as its declarations. Their
	// visibility is Local, and a local class has no ClassID.
	Locals []*Decl
	// Module is a library Kotlin class's or top-level callable's module
	// name, from its metadata; "" for "main" and for the others.
	Module string
	// Inheritors are a library sealed class's direct subclasses, as its
	// metadata lists them; nil for the others (a source sealed class's
	// are found from the sources).
	Inheritors []string
}

// Annotation is an annotation in the sources.
type Annotation struct {
	// ClassID is its class's class ID, or "" if the checker didn't
	// resolve it.
	ClassID string
	Node    psi.NodeID // the ANNOTATION_ENTRY
	Target  string     // the use-site target as written: get, field, file...; or ""
	// Implied is set when Target isn't written but Kotlin's default rule
	// chose it: "field", for an annotation of a property that may be on
	// its backing field and not on the property.
	Implied bool
	// Args are its arguments as written, in the order written, as
	// the checker evaluated them; none if it couldn't.
	Args []NamedValue
	// TypeArgs are the type arguments written, @A<String>, as the
	// checker resolved them; nil for a star projection.
	TypeArgs []*Type
}

// Update checks the module's sources and returns their model. Errors in
// the sources don't stop it: they are the Model's Diagnostics.
func (s *Session) Update(ctx context.Context, sources []Source) (*Model, error) {
	tm := NewTimer()
	defer tm.Print(fmt.Sprintf("update files=%d", len(sources)))
	jobs := max(s.cfg.Jobs, 1)
	var javaSources []Source
	sources = slices.DeleteFunc(slices.Clone(sources), func(src Source) bool {
		if strings.HasSuffix(src.Path, ".java") {
			javaSources = append(javaSources, src)
			return true
		}
		return false
	})
	texts := make([]string, len(sources)+len(javaSources))
	readErrs := make([]error, len(texts))
	all := slices.Concat(sources, javaSources)
	parallel(len(all), jobs, func(i int) { texts[i], readErrs[i] = all[i].text() })
	if err := errors.Join(readErrs...); err != nil {
		return nil, err
	}
	tm.Mark("read")
	// The results of the last checks are kept: an update of the same
	// files with the same texts reuses one whole, and the others reuse
	// the parses of the files whose texts are the same (reuse.go).
	paths := make([]string, len(all))
	for i, src := range all {
		paths[i] = src.Path
	}
	var kept *shared // the library of a module without Java sources
	var cache *checkCache
	var key string
	if len(javaSources) == 0 {
		l, err := s.library()
		if err != nil {
			return nil, err
		}
		kept, cache = l, &l.checks
	} else if key = libraryKey(s.cfg); key != "" {
		cache = &javaChecks
	}
	prev, whole := cache.find(key, paths, texts)
	if whole {
		tm.Mark("reused")
		l := kept
		if l == nil {
			l = prev.sl
			l.retain()
		}
		return &Model{lib: l.lib, shared: l, Files: prev.files, Diagnostics: prev.diags}, nil
	}
	if prev == nil && kept == nil {
		// The first update with Java sources: the parses of the update
		// before, without them.
		s.mu.Lock()
		if s.lib != nil {
			prev, _ = s.lib.checks.find("", nil, nil)
		}
		s.mu.Unlock()
	}
	javaTexts := texts[len(sources):]
	texts = texts[:len(sources)]
	snaps := make([]*psi.Snapshot, len(sources))
	parallel(len(sources), jobs, func(i int) {
		if snap := prev.snapshot(sources[i].Path, texts[i]); snap != nil {
			snaps[i] = snap
			return
		}
		snaps[i] = psi.Parse(sources[i].Path, texts[i])
	})
	javaFiles := make([]*javasrc.File, len(javaSources))
	javaErrs := make([]error, len(javaSources))
	parallel(len(javaSources), jobs, func(i int) {
		javaFiles[i], javaErrs[i] = javasrc.Parse(javaSources[i].Path, javaTexts[i])
	})
	tm.Mark("parse")
	var sl *shared
	var java *driver.JavaSources
	var err error
	var javaDiags []Diagnostic
	if len(javaSources) > 0 {
		// A file kc can't parse is left out: its classes aren't the model's.
		// One with malformed members keeps the rest of them.
		var parsed []*javasrc.File
		for i, f := range javaFiles {
			if f != nil {
				parsed = append(parsed, f)
			}
			if javaErrs[i] != nil {
				javaDiags = append(javaDiags, Diagnostic{Path: javaSources[i].Path, Line: 1, Col: 1, Message: javaErrs[i].Error()})
			}
		}
		sl, java, err = acquireJava(s.cfg, parsed)
	} else {
		sl = kept
	}
	if err != nil {
		return nil, err
	}
	lib := sl.lib.lib
	m := &Model{lib: sl.lib, shared: sl, Diagnostics: javaDiags}
	ok := false
	defer func() {
		if !ok {
			m.Close()
		}
	}()
	tm.Mark("open")
	files, provs, broken := lowerAll(snaps, m, jobs)
	if java != nil {
		java.SetKotlin(files)
	}
	tm.Mark("lower")
	// An update whose changed files declare what they did, and whose
	// other files are those of the check before, checks only the changed
	// files and keeps the others' models (reuse.go).
	var only map[*syntax.File]bool
	if len(broken) == 0 {
		if changed := prev.bodyEdits(paths, sources, texts, javaTexts, snaps); changed != nil {
			incrementalUpdates.Add(1)
			only = map[*syntax.File]bool{}
			for _, i := range changed {
				only[files[i]] = true
			}
		}
	}
	rec, checkErrs, err := checkFiles(ctx, files, lib, jobs, only)
	if !rec.bodies.IsZero() {
		tm.MarkAt("signatures", rec.bodies)
	}
	tm.Mark("bodies")
	if _, crashed := err.(checkPanic); crashed && len(broken) > 0 {
		// The checker can't take a file with syntax errors: check the
		// others, and model the broken ones from syntax alone.
		files, provs, _ = lowerAll(snaps, nil, jobs)
		if java != nil {
			java.SetKotlin(files)
		}
		var clean []*syntax.File
		for i, f := range files {
			if !broken[i] {
				clean = append(clean, f)
			}
		}
		rec, checkErrs, err = checkFiles(ctx, clean, lib, jobs, nil)
	}
	if err != nil {
		return nil, err
	}
	if only != nil {
		// The kept diagnostics of the files not checked: the Java
		// sources' are the same, as their texts are, and are kept with
		// them.
		m.Diagnostics = nil
		checkedPaths := map[string]bool{}
		for f := range only {
			checkedPaths[f.Path] = true
		}
		for _, d := range prev.diags {
			if !checkedPaths[d.Path] {
				m.Diagnostics = append(m.Diagnostics, d)
			}
		}
	}
	for _, e := range checkErrs {
		m.Diagnostics = append(m.Diagnostics, diagnostic(e.File, e))
	}
	slices.SortStableFunc(m.Diagnostics, func(a, b Diagnostic) int {
		return cmp.Or(cmp.Compare(index(snaps, a.Path), index(snaps, b.Path)),
			cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	for i, f := range files {
		if only != nil && !only[f] {
			m.Files = append(m.Files, prev.files[i])
			continue
		}
		b := &fileBuilder{f: f, snap: snaps[i], prov: provs[i], annots: rec.annots[f], values: rec.values[f], refs: rec.refs, aliases: rec.aliases, lib: m.lib,
			tparams: map[*types.TypeParamSym]psi.NodeID{}}
		mf := &File{Path: snaps[i].Path(), Package: f.Package, Snapshot: snaps[i], Lookups: rec.fileLookups(f)}
		mf.Annotations = b.annotations(f.Annotations)
		pkg := strings.ReplaceAll(f.Package, ".", "/")
		for _, d := range f.Decls {
			if md := b.decl(d, nil, pkg+"/"); md != nil {
				mf.Decls = append(mf.Decls, md)
			}
		}
		m.Files = append(m.Files, mf)
	}
	tm.Mark("model")
	if cache != nil {
		c := &checked{key: key, paths: paths, texts: slices.Concat(texts, javaTexts), files: m.Files, diags: m.Diagnostics,
			clean: len(broken) == 0,
			snaps: make(map[string]checkedSnap, len(sources))}
		for i, src := range sources {
			c.snaps[src.Path] = checkedSnap{texts[i], snaps[i]}
		}
		if kept == nil {
			// The entry keeps the module's own library.
			sl.retain()
			c.sl = sl
		}
		cache.keep(c)
	}
	ok = true
	return m, nil
}

// lowerAll lowers snaps, adding their errors to m if it isn't nil, and
// reports which have errors.
func lowerAll(snaps []*psi.Snapshot, m *Model, jobs int) ([]*syntax.File, []*lower.Provenance, map[int]bool) {
	files := make([]*syntax.File, len(snaps))
	provs := make([]*lower.Provenance, len(snaps))
	errLists := make([][]*syntax.Error, len(snaps))
	parallel(len(snaps), jobs, func(i int) {
		files[i], provs[i], errLists[i] = lower.File(snaps[i])
	})
	broken := map[int]bool{}
	for i, snap := range snaps {
		errs := errLists[i]
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

// parallel runs fn(i) for i in [0, n) on up to jobs goroutines.
func parallel(n, jobs int, fn func(i int)) {
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(max(jobs, 1), n) {
		wg.Go(func() {
			for i := range next {
				fn(i)
			}
		})
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}

// checkPanic is a checker crash, which only sources with syntax errors are
// expected to cause.
// Its message ends with the stack from where the checker panicked.
type checkPanic struct {
	v     any
	stack string
}

func (p checkPanic) Error() string { return fmt.Sprintf("ka: checker panic: %v\n%s", p.v, p.stack) }

// panicStack returns the frames of the goroutine's stack below the
// panic call, at most 16 of them.
func panicStack() string {
	s := string(debug.Stack())
	if i := strings.Index(s, "\npanic("); i >= 0 {
		s = s[i+1:]
		if j := strings.Index(s, "\n\t"); j >= 0 {
			if k := strings.IndexByte(s[j+2:], '\n'); k >= 0 {
				s = s[j+2+k+1:] // drop panic's own frame
			}
		}
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 32 {
		lines = lines[:32]
	}
	return strings.Join(lines, "\n")
}

func checkFiles(ctx context.Context, files []*syntax.File, lib *check.Library, jobs int, only map[*syntax.File]bool) (rec *recorder, errs []*syntax.Error, err error) {
	rec = &recorder{annots: map[*syntax.File]map[syntax.Pos]*types.ClassSym{},
		values: map[*syntax.File]map[syntax.Pos][]syntax.AnnotationValue{}, refs: map[*syntax.TypeRef]types.Type{},
		aliases: map[*syntax.TypeRef]*types.AliasSym{}, lookups: map[*syntax.File][]Lookup{}}
	defer func() {
		if v := recover(); v != nil {
			err = checkPanic{v, panicStack()}
		}
	}()
	errs, err = check.CheckOptions(ctx, files, lib, jobs, rec, check.Options{Signatures: true, Only: only})
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
	// values are the arguments of the annotations, as the checker
	// evaluated them.
	values map[*syntax.File]map[syntax.Pos][]syntax.AnnotationValue
	mu     sync.Mutex
	refs   map[*syntax.TypeRef]types.Type // the types written, as resolved
	// aliases are the typealiases the types written name.
	aliases map[*syntax.TypeRef]*types.AliasSym
	// bodies is when the checker started the bodies.
	bodies time.Time
	// lookups are the files' lookups, as the checker reports them.
	lookups map[*syntax.File][]Lookup
}

func (r *recorder) Lookup(f *syntax.File, scope string, kind check.LookupKind, name string) {
	k := PackageLookup
	if kind == check.ClassLookup {
		k = ClassLookup
	}
	r.mu.Lock()
	r.lookups[f] = append(r.lookups[f], Lookup{Scope: scope, Kind: k, Name: name})
	r.mu.Unlock()
}

// fileLookups returns f's lookups, sorted and without duplicates.
func (r *recorder) fileLookups(f *syntax.File) []Lookup {
	ls := r.lookups[f]
	slices.SortFunc(ls, func(a, b Lookup) int {
		return cmp.Or(cmp.Compare(a.Scope, b.Scope), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Name, b.Name))
	})
	return slices.Compact(ls)
}

func (r *recorder) Bodies() { r.bodies = time.Now() }

func (r *recorder) Alias(x any, a *types.AliasSym) {
	if ref, ok := x.(*syntax.TypeRef); ok {
		r.mu.Lock()
		r.aliases[ref] = a
		r.mu.Unlock()
	}
}

func (r *recorder) TypeRef(ref *syntax.TypeRef, t types.Type) {
	r.mu.Lock()
	r.refs[ref] = t
	r.mu.Unlock()
}

func (r *recorder) Annotation(f *syntax.File, a *syntax.Annotation, cls *types.ClassSym) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.annots[f]
	if m == nil {
		m = map[syntax.Pos]*types.ClassSym{}
		r.annots[f] = m
	}
	m[a.P] = cls
}
func (r *recorder) AnnotationValues(f *syntax.File, a *syntax.Annotation, values []syntax.AnnotationValue) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.values[f]
	if m == nil {
		m = map[syntax.Pos][]syntax.AnnotationValue{}
		r.values[f] = m
	}
	m[a.P] = values
}

func (*recorder) When(*syntax.When, bool, bool)    {}
func (*recorder) Stmt(syntax.Stmt, bool)           {}
func (*recorder) Unreachable(syntax.Stmt)          {}
func (*recorder) Elvis(*syntax.Binary, bool)       {}
func (*recorder) StmtType(syntax.Expr, types.Type) {}

type fileBuilder struct {
	f      *syntax.File
	snap   *psi.Snapshot
	prov   *lower.Provenance
	annots map[syntax.Pos]*types.ClassSym
	values map[syntax.Pos][]syntax.AnnotationValue
	refs   map[*syntax.TypeRef]types.Type
	// aliases are the typealiases the types written name.
	aliases map[*syntax.TypeRef]*types.AliasSym
	lib     *library // the class path's classes, for defaultAlias
	// tparams are the TYPE_PARAMETER nodes of the file's type
	// parameters, which their types point to.
	tparams map[*types.TypeParamSym]psi.NodeID
	lines   []int // the byte offsets the source's lines start at, once needed
}

func (b *fileBuilder) annotations(as []syntax.Annotation) []Annotation {
	var out []Annotation
	for i := range as {
		if a := &as[i]; !a.Implicit {
			out = append(out, b.annotation(a))
		}
	}
	return out
}

// annotation returns the model of annotation a.
func (b *fileBuilder) annotation(a *syntax.Annotation) Annotation {
	values, ok := b.values[a.P]
	if !ok {
		values = a.Values
	}
	return Annotation{ClassID: classID(b.annotationClass(a)), Node: b.prov.Annotation(a.P), Target: a.Target,
		Args: b.args(a, values), TypeArgs: b.annotationTypeArgs(a)}
}

// annotationTypeArgs returns the models of a's type arguments.
func (b *fileBuilder) annotationTypeArgs(a *syntax.Annotation) []*Type {
	var out []*Type
	for _, ta := range a.TypeArgs {
		out = append(out, b.written(ta.Type))
	}
	return out
}

// annotationClass returns the class of annotation a, or nil.
func (b *fileBuilder) annotationClass(a *syntax.Annotation) *types.ClassSym {
	if a.Class != nil {
		return a.Class
	}
	if a.Source != nil {
		return a.Source
	}
	return b.annots[a.P]
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
