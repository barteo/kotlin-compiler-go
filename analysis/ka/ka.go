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
	"maps"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	// Check identifies the check the model's files and diagnostics come
	// from: an update that reuses a check whole returns a model with the
	// same Check, and the same Files and Diagnostics, as the update that
	// made it. A client that kept what it built from that model may keep
	// using it.
	Check uint64

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
	// The library of the class path, which the module's Java sources, if
	// any, are put over for each check (library.withJava).
	sl, err := s.library()
	if err != nil {
		return nil, err
	}
	all := slices.Concat(sources, javaSources)
	texts, readErrs := sl.read.texts(all, jobs)
	if err := errors.Join(readErrs...); err != nil {
		sl.release()
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
	cache := &sl.checks
	prev, whole := cache.find("", paths, texts)
	if whole {
		tm.Mark("reused")
		lib := sl.lib
		if prev.lib != nil {
			lib = prev.lib
		}
		return &Model{lib: lib, shared: sl, Files: prev.files, Diagnostics: prev.diags, Check: prev.check}, nil
	}
	javaTexts := texts[len(sources):]
	texts = texts[:len(sources)]
	snaps := make([]*psi.Snapshot, len(sources))
	parallel(len(sources), jobs, func(i int) {
		if snap := prev.snapshot(sources[i].Path, texts[i]); snap != nil {
			snaps[i] = snap
			return
		}
		// A KSP run's second update finds the last run's check of its
		// round, whose edited files' texts differ: the check of the
		// first round, just before, has them.
		if snap := cache.snapshot(sources[i].Path, texts[i]); snap != nil {
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
	ml := sl.lib // the Model's library
	var java *driver.JavaSources
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
		ml = sl.lib.withJava(parsed)
		java = ml.java
	}
	lib := ml.lib
	m := &Model{lib: ml, shared: sl, Diagnostics: javaDiags, Check: checkIDs.Add(1)}
	ok := false
	defer func() {
		if !ok {
			m.Close()
		}
	}()
	tm.Mark("open")
	// An update that may check only some files lowers the changed ones
	// first, then the others, those it won't check as declarations only
	// (lower.Declarations).
	files := make([]*syntax.File, len(snaps))
	provs := make([]*lower.Provenance, len(snaps))
	changed := prev.changedFiles(paths, sources, texts)
	broken := map[int]bool{}
	if changed != nil {
		broken = lowerFiles(snaps, files, provs, m, jobs, changed, nil, nil)
	}
	rest := func(i int) bool { return files[i] == nil }
	// An update whose changed files declare what they did, and whose
	// other files are those of the check before, checks only the changed
	// files and keeps the others' models (reuse.go).
	// One whose changed files declare something else checks those
	// files and the ones that look up what they declare, until the
	// models of the files it checks besides the changed ones are as they
	// were, or it would check too many (declEdits, dependents).
	var only map[*syntax.File]bool
	// dirty is the files a partial check checks; declEdit is set if they
	// are declEdits', which may reach more as they're checked.
	var dirty map[int]bool
	// shifted are the changed files that keep their models (bodyreuse.go).
	var shifted shiftedModels
	declEdit := false
	if changed != nil && len(broken) == 0 {
		if edits := prev.bodyEdits(paths, sources, texts, javaTexts, snaps); edits != nil {
			incrementalUpdates.Add(1)
			dirty = map[int]bool{}
			for _, i := range edits {
				// A file whose change no check looks into keeps its model,
				// moved to its new parse (bodyreuse.go).
				if f, diags, ok := prev.shiftedFile(i, snaps[i], texts[i]); ok {
					if shifted == nil {
						shifted = shiftedModels{}
					}
					shifted[i] = shiftedModel{f, diags}
					continue
				}
				dirty[i] = true
			}
			if len(dirty) == 0 {
				shiftedUpdates.Add(int64(len(shifted)))
				m.Files = slices.Clone(prev.files)
				for i, s := range shifted {
					m.Files[i] = s.file
				}
				m.Diagnostics = nil
				for _, d := range prev.diags {
					if !shifted.has(d.Path) {
						m.Diagnostics = append(m.Diagnostics, d)
					}
				}
				for _, s := range shifted {
					m.Diagnostics = append(m.Diagnostics, s.diags...)
				}
				sortDiagnostics(m.Diagnostics, snaps)
				tm.Mark("shifted")
				// The Java sources and what the Kotlin ones declare are
				// the same: so are the classes of the check's library.
				if prev.lib != nil {
					m.lib = prev.lib
				}
				keepCheck(cache, m, paths, slices.Concat(texts, javaTexts), sources, snaps, true, prev.lib)
				ok = true
				return m, nil
			}
		} else if dirty = prev.declEdits(paths, sources, texts, &javaSet{javaTexts, javaFiles}, snaps, files); dirty != nil {
			declUpdates.Add(1)
			declEdit = true
		}
	}
	notDirty := func(i int) bool { return !dirty[i] }
	if dirty == nil {
		notDirty = nil
	}
	// The files a check of only some can't reach are stubs (reachable).
	var reach map[int]bool
	stub := func(i int) bool { return reach != nil && !reach[i] }
	if dirty != nil {
		reach = reachable(snaps, dirty, javaTexts, cache)
	} else {
		lastResolved.Store(int64(len(sources)))
	}
	maps.Copy(broken, lowerFiles(snaps, files, provs, m, jobs, rest, notDirty, stub))
	if java != nil {
		java.SetKotlin(files)
	}
	tm.Mark("lower")
	for {
		if dirty != nil {
			only = map[*syntax.File]bool{}
			for i := range dirty {
				only[files[i]] = true
			}
		}
		if only != nil {
			lastChecked.Store(int64(len(only)))
		} else {
			lastChecked.Store(int64(len(files)))
		}
		tm.Note("checked", int(lastChecked.Load()))
		rec, checkErrs, err := checkFiles(ctx, files, lib, jobs, only, namesIn(snaps, files, only))
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
			rec, checkErrs, err = checkFiles(ctx, clean, lib, jobs, nil, nil)
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
				if !checkedPaths[d.Path] && !shifted.has(d.Path) {
					m.Diagnostics = append(m.Diagnostics, d)
				}
			}
			for _, s := range shifted {
				m.Diagnostics = append(m.Diagnostics, s.diags...)
			}
		}
		for _, e := range checkErrs {
			m.Diagnostics = append(m.Diagnostics, diagnostic(e.File, e))
		}
		sortDiagnostics(m.Diagnostics, snaps)
		for i, f := range files {
			if only != nil && !only[f] {
				if s, ok := shifted[i]; ok {
					m.Files = append(m.Files, s.file)
				} else {
					m.Files = append(m.Files, prev.files[i])
				}
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
		if !declEdit {
			break
		}
		more := prev.dependents(dirty, m.Files, texts, &javaSet{javaTexts, javaFiles})
		tm.Mark("dependents")
		if len(more) == 0 {
			break
		}
		// Check again, as the checker leaves its marks on the files.
		for i := range more {
			dirty[i] = true
		}
		if len(dirty)*maxDirtyShare > len(sources) {
			dirty, only, notDirty = nil, nil, nil
		}
		clear(files)
		reach = nil
		if dirty != nil {
			reach = reachable(snaps, dirty, javaTexts, cache)
		} else {
			lastResolved.Store(int64(len(sources)))
		}
		lowerFiles(snaps, files, provs, nil, jobs, rest, notDirty, stub)
		m.Diagnostics, m.Files = javaDiags, nil
		tm.Mark("lower")
	}
	var javaLib *library
	if java != nil {
		javaLib = ml
	}
	keepCheck(cache, m, paths, slices.Concat(texts, javaTexts), sources, snaps, len(broken) == 0, javaLib)
	tm.Mark("keep")
	ok = true
	return m, nil
}

// keepCheck keeps the check that made model m, of sources with texts
// (the Java sources' last), in cache.
func keepCheck(cache *checkCache, m *Model, paths, texts []string, sources []Source, snaps []*psi.Snapshot, clean bool, lib *library) {
	if cache == nil || noReuse {
		return
	}
	c := &checked{check: m.Check, paths: paths, texts: texts, files: m.Files, diags: m.Diagnostics, clean: clean, lib: lib,
		snaps: make(map[string]checkedSnap, len(sources))}
	for i, src := range sources {
		c.snaps[src.Path] = checkedSnap{texts[i], snaps[i]}
	}
	c.keys = cache.keysOf(m.Files)
	cache.keep(c)
}

// sortDiagnostics sorts diagnostics by their files' order in snaps, then
// by position.
func sortDiagnostics(diags []Diagnostic, snaps []*psi.Snapshot) {
	slices.SortStableFunc(diags, func(a, b Diagnostic) int {
		return cmp.Or(cmp.Compare(index(snaps, a.Path), index(snaps, b.Path)),
			cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
}

// namesIn returns the identifiers in the snapshots of the files in only,
// those of files: the names of what they may need the omitted types of
// (check.Options.Infer).
func namesIn(snaps []*psi.Snapshot, files []*syntax.File, only map[*syntax.File]bool) map[string]bool {
	if only == nil {
		return nil
	}
	names := map[string]bool{}
	for i, f := range files {
		if !only[f] {
			continue
		}
		snap := snaps[i]
		for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
			if n := snap.Node(id); n.NumericKind() == psi.KindIDENTIFIER {
				names[strings.Trim(n.Text(), "`")] = true
			}
		}
	}
	return names
}

// reachable returns, for a check of only the dirty files, the files
// the check can reach: the others are given to it as stubs, their
// packages without declarations (declStub). A top-level declaration is
// reached if a reached text writes a name it declares (its own or a
// class's it nests), and its text is then reached: the dirty files'
// whole texts are, and so are the top-level operators and functions a
// construct calls by convention, which code calls without writing their
// names, and the declarations named in the Java sources, whose members
// may return a Kotlin class no Kotlin file names. So an inferred type
// (val x = f()), a supertype, a typealias or an extension is reached
// through the names that lead to it, and so are the members of the
// classes a reached text names. A reached sealed class reaches the
// classes whose supertypes name it too, which a when on it counts
// though no text need name them. Every name the check looks up is one a
// reached text writes, so the files declaring it are reached. It
// returns nil, every file, if the files reached are more than half.
func reachable(snaps []*psi.Snapshot, dirty map[int]bool, javaTexts []string, cache *checkCache) map[int]bool {
	lastResolved.Store(int64(len(snaps)))
	if dirty == nil || noReach {
		return nil
	}
	type at struct{ file, decl int }
	fns := make([]*fileNames, len(snaps))
	declares := map[string][]at{}
	sealed := map[string]bool{}
	subs := map[string][]at{} // by a name their supertypes write
	set := map[int]bool{}
	reached := map[at]bool{}
	var queue []map[string]bool
	reach := func(a at) {
		set[a.file] = true
		if !reached[a] {
			reached[a] = true
			queue = append(queue, fns[a.file].decls[a.decl].ids)
		}
	}
	for i, snap := range snaps {
		fn := cache.names(snap)
		fns[i] = fn
		for k, d := range fn.decls {
			for _, n := range d.names {
				declares[n] = append(declares[n], at{i, k})
			}
			for _, n := range d.sealed {
				sealed[n] = true
			}
			for n := range d.supers {
				subs[n] = append(subs[n], at{i, k})
			}
		}
	}
	for i, fn := range fns {
		for k, d := range fn.decls {
			if d.operator {
				reach(at{i, k})
			}
		}
		if dirty[i] {
			set[i] = true
			queue = append(queue, fn.all)
		}
	}
	names := map[string]bool{}
	name := func(n string) {
		if names[n] {
			return
		}
		names[n] = true
		for _, a := range declares[n] {
			reach(a)
		}
		if sealed[n] {
			for _, a := range subs[n] {
				reach(a)
			}
		}
	}
	for _, t := range javaTexts {
		javaIdentifiers(t, name)
	}
	for len(queue) > 0 {
		ids := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for n := range ids {
			name(n)
		}
	}
	if len(set)*2 > len(snaps) {
		return nil
	}
	lastResolved.Store(int64(len(set)))
	return set
}

// noReach turns reachable off, for tests and comparisons
// (KC_NO_REACH): a check of only some files resolves every file's
// signatures.
var noReach = os.Getenv("KC_NO_REACH") != ""

// lastResolved is how many files the last check was given whole, not
// as stubs (reachable), for tests.
var lastResolved atomic.Int64

// fileNames are the identifiers a parse writes (reachable): all of them,
// and those of each top-level declaration.
type fileNames struct {
	all   map[string]bool
	decls []declNames
}

type declNames struct {
	names []string        // the names it declares: its own, and its classes'
	ids   map[string]bool // the identifiers it writes
	// sealed are the names of its sealed classes, and supers the
	// identifiers its classes' supertype lists write.
	sealed []string
	supers map[string]bool
	// operator is set for a function whose name is an operator's or a
	// conventional one's (check.IsOperatorName).
	operator bool
}

func snapNames(snap *psi.Snapshot) *fileNames {
	fn := &fileNames{all: map[string]bool{}}
	root := snap.RootNode()
	for n := root.FirstChild(); n.Valid(); n = n.NextSibling() {
		k := n.NumericKind()
		isDecl := k == psi.KindCLASS || k == psi.KindOBJECT_DECLARATION || k == psi.KindFUN || k == psi.KindPROPERTY || k == psi.KindTYPEALIAS
		var d declNames
		if isDecl {
			d.ids = map[string]bool{}
			if name := n.NameIdentifier(); name.Valid() {
				d.names = append(d.names, strings.Trim(name.Text(), "`"))
				d.operator = k == psi.KindFUN && check.IsOperatorName(d.names[0])
			}
		}
		for id := n.ID(); id < n.SubtreeEnd(); id++ {
			m := snap.Node(id)
			switch m.NumericKind() {
			case psi.KindIDENTIFIER:
				t := strings.Trim(m.Text(), "`")
				fn.all[t] = true
				if isDecl {
					d.ids[t] = true
				}
			case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
				if !isDecl {
					break
				}
				name := m.NameIdentifier()
				if name.Valid() && id != n.ID() {
					d.names = append(d.names, strings.Trim(name.Text(), "`"))
				}
				if name.Valid() && m.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindSEALED_KEYWORD).Valid() {
					d.sealed = append(d.sealed, strings.Trim(name.Text(), "`"))
				}
				if st := m.ChildKind(psi.KindSUPER_TYPE_LIST); st.Valid() {
					for sid := st.ID(); sid < st.SubtreeEnd(); sid++ {
						if x := snap.Node(sid); x.NumericKind() == psi.KindIDENTIFIER {
							if d.supers == nil {
								d.supers = map[string]bool{}
							}
							d.supers[strings.Trim(x.Text(), "`")] = true
						}
					}
				}
			}
		}
		if isDecl {
			fn.decls = append(fn.decls, d)
		}
	}
	return fn
}

// javaIdentifiers calls f with each identifier of a Java text.
func javaIdentifiers(text string, f func(string)) {
	start := -1
	for i := 0; i <= len(text); i++ {
		c := byte(0)
		if i < len(text) {
			c = text[i]
		}
		word := c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80 || start >= 0 && c >= '0' && c <= '9'
		switch {
		case word && start < 0:
			start = i
		case !word && start >= 0:
			f(text[start:i])
			start = -1
		}
	}
}

// names returns the identifiers snap's text writes, kept with the parse
// while a check c keeps holds it.
func (c *checkCache) names(snap *psi.Snapshot) *fileNames {
	if c == nil {
		return snapNames(snap)
	}
	c.mu.Lock()
	fn, ok := c.ids[snap]
	c.mu.Unlock()
	if ok {
		return fn
	}
	fn = snapNames(snap)
	c.mu.Lock()
	if c.ids == nil {
		c.ids = map[*psi.Snapshot]*fileNames{}
	}
	c.ids[snap] = fn
	c.mu.Unlock()
	return fn
}

// lowerAll lowers snaps, adding their errors to m if it isn't nil, and
// returns the indexes of those with errors.
func lowerAll(snaps []*psi.Snapshot, m *Model, jobs int) ([]*syntax.File, []*lower.Provenance, map[int]bool) {
	files := make([]*syntax.File, len(snaps))
	provs := make([]*lower.Provenance, len(snaps))
	broken := lowerFiles(snaps, files, provs, m, jobs, nil, nil, nil)
	return files, provs, broken
}

// lowerFiles lowers the snaps for which need is true (every one if need
// is nil) into files and provs, as declarations only those for which
// declsOnly is true (lower.Declarations), adding their errors to m if it
// isn't nil, and returns the indexes of those with errors.
func lowerFiles(snaps []*psi.Snapshot, files []*syntax.File, provs []*lower.Provenance, m *Model, jobs int, need, declsOnly, stub func(i int) bool) map[int]bool {
	var todo []int
	for i := range snaps {
		if need == nil || need(i) {
			todo = append(todo, i)
		}
	}
	errLists := make([][]*syntax.Error, len(snaps))
	parallel(len(todo), jobs, func(k int) {
		i := todo[k]
		if stub != nil && stub(i) {
			files[i], provs[i], errLists[i] = declStub(snaps[i])
		} else if declsOnly != nil && declsOnly(i) {
			files[i], provs[i], errLists[i] = declarations(snaps[i])
		} else {
			files[i], provs[i], errLists[i] = lower.File(snaps[i])
		}
	})
	broken := map[int]bool{}
	for _, i := range todo {
		errs := errLists[i]
		if len(errs) > 0 {
			broken[i] = true
		}
		if m != nil {
			for _, e := range errs {
				m.Diagnostics = append(m.Diagnostics, diagnostic(snaps[i].Path(), e))
			}
		}
	}
	return broken
}

// parallel runs fn(i) for i in [0, n) on up to jobs goroutines, the
// caller's one of them. Each takes the next index from a counter, so
// items that take microseconds (a stat, a kept parse) cost no handoff
// between threads: in a daemon, Go's threads are parked between updates,
// and waking one per item took longer than the items.
func parallel(n, jobs int, fn func(i int)) {
	var next atomic.Int64
	work := func() {
		for {
			i := int(next.Add(1) - 1)
			if i >= n {
				return
			}
			fn(i)
		}
	}
	var wg sync.WaitGroup
	for range min(max(jobs, 1), n) - 1 {
		wg.Go(work)
	}
	work()
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

func checkFiles(ctx context.Context, files []*syntax.File, lib *check.Library, jobs int, only map[*syntax.File]bool, infer map[string]bool) (rec *recorder, errs []*syntax.Error, err error) {
	rec = &recorder{annots: map[*syntax.File]map[syntax.Pos]*types.ClassSym{},
		values: map[*syntax.File]map[syntax.Pos][]syntax.AnnotationValue{}, refs: map[*syntax.TypeRef]types.Type{},
		aliases: map[*syntax.TypeRef]*types.AliasSym{}, lookups: map[*syntax.File][]Lookup{}}
	defer func() {
		if v := recover(); v != nil {
			err = checkPanic{v, panicStack()}
		}
	}()
	errs, err = check.CheckOptions(ctx, files, lib, jobs, rec, check.Options{Signatures: true, Only: only, Infer: infer})
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
