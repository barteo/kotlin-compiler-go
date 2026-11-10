package ka

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Reuse. A KSP run in a Gradle daemon checks the same module again and
// again, mostly with the same texts: a run with no edits, or KSP's later
// rounds when the earlier ones generated the same files as last run.
// The results of the last maxChecks checks are kept: an update of the
// same files with the same texts gets the earlier model's files and
// diagnostics back, and an update whose texts differ parses again only
// the files whose texts changed.
//
// A module keeps its checks in the library its updates share
// (shared.checks). One with Java sources is checked against a library of
// its own, made with those sources over the shared one, which the check
// keeps (library.withJava).
//
// An update that only changes bodies, whose changed files declare what
// they did (signatureHash), with the other files and the Java texts the
// same, checks only the changed files (check.Options.Only) and keeps the
// others' models and diagnostics: what a file declares depends on the
// other files' declarations, not on their bodies. Every other update is
// checked again whole: the checker resolves a file's declarations
// against the whole module's (an inferred type, an overload, a supertype
// may come from any file), and a module's declarations aren't kept from
// one check to the next. A model's files and declarations aren't changed
// once Update returns, so models can share them.

// A KSP run's checks alternate between file sets, the sources and then
// the sources with what the processors generated, and an edit's update
// finds the last run's check of the same set (find). The second round's
// update of an edit that changes declarations then finds the edited files
// changed, though the first round's check, just before, checked them with
// the same texts. Such an update takes their models from that check
// instead of checking them again, when what the two checks' files differ
// by can't change them (borrow): a file's model depends on another's
// declarations only through the names it looks up, as declEdits has it.

// incrementalUpdates counts the updates that check only some files, for
// tests.
var incrementalUpdates atomic.Int64

// declUpdates counts the updates that check some files after changes to
// what files declare (declEdits), for tests.
var declUpdates atomic.Int64

// lastChecked is how many files the last check of an update checked, for
// tests.
var lastChecked atomic.Int64

// noReuse turns the caches off, for tests that compare an update with a
// whole check.
var noReuse bool

// maxChecks is how many checks' results a cache keeps.
const maxChecks = 2

// checkIDs numbers checks, for Model.Check.
var checkIDs atomic.Uint64

// checkCache is a list of checks' results, most recently used last.
type checkCache struct {
	mu   sync.Mutex
	list []*checked
	// ids are the identifiers of the parses the checks hold (reachable).
	ids map[*psi.Snapshot]*fileNames
	// keys are the keys of the models the checks hold (keysOf).
	keys map[*File]*fileKeys
	// reach are reachable's indexes of the last updates' files, the most
	// recent first (reachIndex).
	reach []*reachIndex
}

// checked is the result of checking a module's sources.
type checked struct {
	check        uint64                 // the Model.Check of its models
	key          string                 // "", or what else must match for find
	paths, texts []string               // the Kotlin sources', then the Java sources'
	snaps        map[string]checkedSnap // the Kotlin sources' parses, by path
	files        []*File
	keys         []*fileKeys // files' (keysOf)
	diags        []Diagnostic
	lib          *library // the library with the Java sources the check made, or nil
	// clean is set if no file had syntax errors, so that the check's
	// files can be kept by an update that checks only some (bodyEdits).
	clean bool
}

type checkedSnap struct {
	text string
	snap *psi.Snapshot
}

// find returns the check with key of paths with texts that c keeps, and
// whether it is of the same texts; or, if none is, the last check with
// key of the same paths, which an update may check only some files
// against (changedFiles); or else the last check with key, whose parses
// an update may reuse; or nil. A nil c keeps none.
//
// The checks of a KSP run alternate between file sets: the sources, then
// the sources with what the processors generated. An edit's update finds
// the last run's check of the same round, not the round before it.
func (c *checkCache) find(key string, paths, texts []string) (*checked, bool) {
	if c == nil || noReuse {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var last, same *checked
	for i := len(c.list) - 1; i >= 0; i-- {
		e := c.list[i]
		if e.key != key {
			continue
		}
		if slices.Equal(e.paths, paths) {
			if slices.Equal(e.texts, texts) {
				c.list = append(slices.Delete(c.list, i, i+1), e)
				return e, true
			}
			if same == nil {
				same = e
			}
		}
		if last == nil {
			last = e
		}
	}
	if same != nil {
		return same, false
	}
	return last, false
}

// snapshot returns e's parse of the file at path if its text was text.
func (e *checked) snapshot(path, text string) *psi.Snapshot {
	if e == nil {
		return nil
	}
	if p, ok := e.snaps[path]; ok && p.text == text {
		return p.snap
	}
	return nil
}

// snapshot returns the parse of the file at path with text of any check
// c keeps, or nil.
func (c *checkCache) snapshot(path, text string) *psi.Snapshot {
	if c == nil || noReuse {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.list) - 1; i >= 0; i-- {
		if snap := c.list[i].snapshot(path, text); snap != nil {
			return snap
		}
	}
	return nil
}

// changedFiles returns a test of which sources' texts differ from e's, if
// e is a clean check of the same files, whose update may then check only
// some of them; nil otherwise.
func (e *checked) changedFiles(paths []string, sources []Source, texts []string) func(i int) bool {
	if e == nil || !e.clean || !slices.Equal(e.paths, paths) {
		return nil
	}
	for _, src := range sources {
		if _, ok := e.snaps[src.Path]; !ok {
			return nil
		}
	}
	return func(i int) bool { return e.snaps[sources[i].Path].text != texts[i] }
}

// bodyEdits returns the indexes of the Kotlin sources whose texts differ
// from e's, if e is a clean check of the same files, with the same Java
// texts, and the changed files declare what they did in e
// (signatureHash): an update that only changes bodies. It returns nil
// otherwise, and if no text changed.
func (e *checked) bodyEdits(paths []string, sources []Source, texts, javaTexts []string, snaps []*psi.Snapshot) []int {
	if e == nil || !e.clean || !slices.Equal(e.paths, paths) || !slices.Equal(e.texts[len(sources):], javaTexts) {
		return nil
	}
	var changed []int
	for i, src := range sources {
		p, ok := e.snaps[src.Path]
		if !ok {
			return nil
		}
		if p.text == texts[i] {
			continue
		}
		if len(snaps[i].Diagnostics()) > 0 || signatureHash(p.snap) != signatureHash(snaps[i]) {
			return nil
		}
		changed = append(changed, i)
	}
	return changed
}

// maxDirtyShare is the share of a module's files above which an update
// that changes declarations checks the whole module instead (declEdits):
// checking the rest too costs little more.
const maxDirtyShare = 2

// declEdits returns the indexes of the files an update that changes
// declarations checks, if e is a clean check of the same files, with the
// same Java texts, and the changed files parse without errors: the changed
// files, and those whose lookups in e find what the changed files declare
// before (e's models) or after the edit (files, as lowered), or look in
// the classes they declare (declaredNames.hit), or that declare the same
// (declaredNames.declaredBy), which may conflict. A file's model depends on
// another's declarations only through the names it looks up, which the
// checker records as FIR does, and more; but a model that changes because
// of them, an inferred type, may change others', which dependents checks
// once the files are checked. The Java sources whose texts name a class
// the changed files declare, or their facade class, have their classes
// counted as the changed files' (addJava): their signatures may change
// with them. It returns nil otherwise, and if no text changed or more than
// 1/maxDirtyShare of the files would be checked.
func (e *checked) declEdits(paths []string, sources []Source, texts []string, java *javaSet, snaps []*psi.Snapshot, files []*syntax.File) map[int]bool {
	if e == nil || !e.clean || !slices.Equal(e.paths, paths) || len(e.files) != len(sources) ||
		!slices.Equal(e.texts[len(sources):], java.texts) {
		return nil
	}
	names := newDeclaredNames()
	dirty := map[int]bool{}
	for i, src := range sources {
		p, ok := e.snaps[src.Path]
		if !ok {
			return nil
		}
		if p.text == texts[i] {
			continue
		}
		if len(snaps[i].Diagnostics()) > 0 {
			return nil
		}
		dirty[i] = true
		// Only the declarations whose texts changed: one whose text didn't
		// may still change with them, which dependents finds in its model.
		top := changedDecls(p.snap, snaps[i])
		names.addModelOf(e.files[i], top)
		names.addSyntaxOf(files[i], top)
		if !names.addFacade(src.Path, e.snaps[src.Path].text, texts[i], java) {
			return nil
		}
	}
	if len(dirty) == 0 {
		return nil
	}
	if !names.addJava(java) {
		return nil
	}
	names.seal()
	e.addLookingUp(dirty, names)
	if len(dirty)*maxDirtyShare > len(sources) {
		return nil
	}
	return dirty
}

// dependents returns the files that look up what the files in dirty
// declare in the top-level declarations whose models in files, as checked
// again, differ from e's: those files' models may depend on theirs
// (declEdits). Models are compared but for psi node IDs, which an edit
// moves, and which no other file's model holds.
func (e *checked) dependents(dirty map[int]bool, files []*File, texts []string, java *javaSet) map[int]bool {
	names := newDeclaredNames()
	changed := false
	for i := range dirty {
		top, any := changedModels(e.files[i], files[i])
		if !any {
			continue
		}
		changed = true
		names.addModelOf(e.files[i], top)
		names.addModelOf(files[i], top)
		if !names.addFacade(files[i].Path, e.snaps[files[i].Path].text, texts[i], java) {
			return allFiles(len(files))
		}
	}
	if !changed {
		return nil
	}
	if !names.addJava(java) {
		return allFiles(len(files))
	}
	names.seal()
	more := map[int]bool{}
	for j, f := range e.files {
		if !dirty[j] && names.affects(f, e.keys[j]) {
			more[j] = true
		}
	}
	return more
}

// borrowedUpdates counts the updates that took the models of their dirty
// files from another check (borrow), for tests.
var borrowedUpdates atomic.Int64

// noBorrow turns borrow off, for tests and comparisons (KC_NO_BORROW).
var noBorrow = os.Getenv("KC_NO_BORROW") != ""

// borrow returns the models and diagnostics of the dirty files of an
// update whose check before is prev (declEdits), taken from another check
// c keeps, which checked them with the same parses: a KSP run's first
// round, when prev is the last run's second (find). It reports false,
// and the update checks them, unless:
//   - the other check is clean, of Kotlin sources only, has every dirty
//     file with the same parse and reports nothing in it, and has no file
//     the update doesn't;
//   - each Kotlin file the update has that the other check doesn't, or
//     has with another text, prev has with the same text: what it
//     declares is prev's model of it; and kc parses each of the update's
//     Java sources, which prev has too (declEdits);
//   - no dirty file looks up or declares a name those files declare
//     (declaredNames.affects), nor writes, as a word, the name of a
//     package they make, one none of the other check's files is in or
//     inside and the class path doesn't have (cp). A file that names a
//     package missing from a check reports it there, which the rule
//     above leaves out; this is for the rest;
//   - with Java sources, no Java text names a class or facade of the
//     dirty files whose models differ from prev's (addJava): the classes
//     of prev's library, which the update takes as the shifted one does,
//     see the Kotlin classes as they were (JavaSources.SetKotlin).
func (c *checkCache) borrow(prev *checked, dirty map[int]bool, sources []Source, texts []string, java *javaSet, snaps []*psi.Snapshot, cp *classpath.Path) (map[int]*File, []Diagnostic, bool) {
	if c == nil || noReuse || noBorrow || prev == nil || len(dirty) == 0 || len(java.texts) > 0 && prev.lib == nil {
		return nil, nil, false
	}
	c.mu.Lock()
	list := slices.Clone(c.list)
	c.mu.Unlock()
	for i := len(list) - 1; i >= 0; i-- {
		o := list[i]
		if o == prev || !o.clean || o.key != prev.key || len(o.texts) != len(o.files) {
			continue
		}
		files, diags, ok := o.lend(prev, dirty, sources, texts, java, snaps, cp)
		if !ok {
			continue
		}
		if len(java.texts) > 0 && !prev.javaBlind(dirty, files, texts, java) {
			return nil, nil, false
		}
		return files, diags, true
	}
	return nil, nil, false
}

// lend is borrow from o, a clean check of Kotlin sources only.
func (o *checked) lend(prev *checked, dirty map[int]bool, sources []Source, texts []string, java *javaSet, snaps []*psi.Snapshot, cp *classpath.Path) (map[int]*File, []Diagnostic, bool) {
	at := make(map[string]int, len(o.paths))
	for k, p := range o.paths {
		at[p] = k
	}
	inUpdate := make(map[string]bool, len(sources))
	for _, src := range sources {
		inUpdate[src.Path] = true
	}
	for _, p := range o.paths {
		if !inUpdate[p] {
			return nil, nil, false
		}
	}
	files := map[int]*File{}
	keys := map[int]*fileKeys{}
	for i := range dirty {
		k, ok := at[sources[i].Path]
		if !ok || o.texts[k] != texts[i] || o.snaps[sources[i].Path].snap != snaps[i] {
			return nil, nil, false
		}
		files[i] = o.files[k]
		if k < len(o.keys) && o.keys[k] != nil {
			keys[i] = o.keys[k]
		} else {
			keys[i] = newFileKeys(o.files[k])
		}
	}
	for _, d := range o.diags {
		for i := range dirty {
			if d.Path == sources[i].Path {
				return nil, nil, false
			}
		}
	}
	// What the update's files differ by from o's.
	names := newDeclaredNames()
	// The packages o's files are in, and those these are inside.
	pkgs := map[string]bool{}
	for _, f := range o.files {
		for p := f.Package; p != ""; p = parentPackage(p) {
			pkgs[p] = true
		}
	}
	newPkgs := map[string]bool{}
	addPackage := func(pkg string) {
		for p := pkg; p != "" && !pkgs[p]; p = parentPackage(p) {
			pkgs[p] = true
			if cp != nil {
				if _, ok := cp.Classes(strings.ReplaceAll(p, ".", "/")); ok {
					continue
				}
			}
			newPkgs[p[strings.LastIndexByte(p, '.')+1:]] = true
		}
	}
	prevAt := make(map[string]int, len(prev.paths))
	for k, p := range prev.paths {
		prevAt[p] = k
	}
	for j, src := range sources {
		if k, ok := at[src.Path]; ok && o.texts[k] == texts[j] {
			continue
		}
		if dirty[j] {
			return nil, nil, false
		}
		k, ok := prevAt[src.Path]
		if !ok || k >= len(prev.files) || prev.texts[k] != texts[j] {
			return nil, nil, false
		}
		f := prev.files[k]
		names.addModel(f)
		addPackage(f.Package)
	}
	for _, f := range java.files {
		if f == nil {
			return nil, nil, false
		}
		var add func(scope string, cs []*javasrc.Class)
		add = func(scope string, cs []*javasrc.Class) {
			for _, c := range cs {
				names.add(scope, c.Name)
				cls := c.Name
				if scope != "" {
					cls = scope + "." + c.Name
				}
				names.classes[cls] = true
				add(cls, c.Nested)
			}
		}
		add(f.Package, f.Classes)
		addPackage(f.Package)
	}
	names.seal()
	for i, f := range files {
		if names.affects(f, keys[i]) || len(newPkgs) > 0 && namesAny(texts[i], newPkgs) {
			return nil, nil, false
		}
	}
	return files, nil, true
}

// parentPackage returns the package pkg is in, "" for a top-level one.
func parentPackage(pkg string) string {
	if i := strings.LastIndexByte(pkg, '.'); i >= 0 {
		return pkg[:i]
	}
	return ""
}

// javaBlind reports whether no Java source names a class or facade of the
// dirty files whose models in files differ from e's (borrow).
func (e *checked) javaBlind(dirty map[int]bool, files map[int]*File, texts []string, java *javaSet) bool {
	names := newDeclaredNames()
	for i := range dirty {
		top, any := changedModels(e.files[i], files[i])
		if !any {
			continue
		}
		names.addModelOf(e.files[i], top)
		names.addModelOf(files[i], top)
		if !names.addFacade(files[i].Path, e.snaps[files[i].Path].text, texts[i], java) {
			return false
		}
	}
	simple := map[string]bool{}
	for cls := range names.classes {
		simple[cls[strings.LastIndexByte(cls, '.')+1:]] = true
	}
	for _, f := range names.facades {
		simple[f] = true
	}
	if len(simple) == 0 {
		return true
	}
	for _, text := range java.texts {
		if namesAny(text, simple) {
			return false
		}
	}
	return true
}

// addLookingUp adds to dirty the files whose lookups in e hit names, and
// those that declare one of them, whose declarations may conflict.
func (e *checked) addLookingUp(dirty map[int]bool, names *declaredNames) {
	for j, f := range e.files {
		if !dirty[j] && names.affects(f, e.keys[j]) {
			dirty[j] = true
		}
	}
}

// changedDecls returns a test of which top-level declarations, by name,
// differ between parses old and new of a file (declHashes), or nil if
// anything else does, the package, imports or file annotations: then any
// declaration may.
func changedDecls(old, new *psi.Snapshot) func(name string) bool {
	or, od := declHashes(old)
	nr, nd := declHashes(new)
	if or != nr {
		return nil
	}
	return func(name string) bool {
		o, inOld := od[name]
		n, inNew := nd[name]
		// A name neither parse shows, one the model makes up, may be anything.
		return !inOld && !inNew || !slices.Equal(o, n)
	}
}

// changedModels returns a test of which top-level declarations, by name,
// differ between a and b, models of a file (sameNodes), nil if the package
// or the file's annotations do, and whether any does.
func changedModels(a, b *File) (func(name string) bool, bool) {
	if a.Package != b.Package || !sameNodes(a.Annotations, b.Annotations) {
		return nil, true
	}
	group := func(ds []*Decl) map[string][]*Decl {
		g := map[string][]*Decl{}
		for _, d := range ds {
			g[d.Name] = append(g[d.Name], d)
		}
		return g
	}
	ga, gb := group(a.Decls), group(b.Decls)
	changed := map[string]bool{}
	for name, ds := range ga {
		if !sameNodes(ds, gb[name]) {
			changed[name] = true
		}
	}
	for name := range gb {
		if _, ok := ga[name]; !ok {
			changed[name] = true
		}
	}
	return func(name string) bool { return changed[name] }, len(changed) > 0
}

// sameNodes reports whether a and b, parts of models, are deeply equal, as
// reflect.DeepEqual, but for the psi node IDs and snapshots they hold.
func sameNodes(a, b any) bool {
	return equalNodes(reflect.ValueOf(a), reflect.ValueOf(b), map[[3]uintptr]bool{})
}

var snapshotType = reflect.TypeFor[*psi.Snapshot]()

func equalNodes(a, b reflect.Value, seen map[[3]uintptr]bool) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() {
		return false
	}
	if a.Type() == nodeIDType || a.Type() == snapshotType {
		return true
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		if a.Kind() != reflect.Slice || a.Len() > 0 {
			if a.UnsafePointer() == b.UnsafePointer() && (a.Kind() != reflect.Slice || a.Len() == b.Len()) {
				return true
			}
			// A cycle is equal if it is equal on the way round.
			k := [3]uintptr{uintptr(a.UnsafePointer()), uintptr(b.UnsafePointer()), uintptr(a.Kind())}
			if seen[k] {
				return true
			}
			seen[k] = true
		}
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		return equalNodes(a.Elem(), b.Elem(), seen)
	case reflect.Struct:
		for i := range a.NumField() {
			if !equalNodes(a.Field(i), b.Field(i), seen) {
				return false
			}
		}
		return true
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return false
		}
		for i := range a.Len() {
			if !equalNodes(a.Index(i), b.Index(i), seen) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.Len() != b.Len() {
			return false
		}
		for it := a.MapRange(); it.Next(); {
			v := b.MapIndex(it.Key())
			if !v.IsValid() || !equalNodes(it.Value(), v, seen) {
				return false
			}
		}
		return true
	case reflect.Func:
		return a.IsNil() && b.IsNil()
	case reflect.Bool:
		return a.Bool() == b.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() == b.Uint()
	case reflect.Float32, reflect.Float64:
		return a.Float() == b.Float() || a.Float() != a.Float() && b.Float() != b.Float()
	case reflect.Complex64, reflect.Complex128:
		return a.Complex() == b.Complex()
	case reflect.String:
		return a.String() == b.String()
	case reflect.Chan, reflect.UnsafePointer:
		return a.Pointer() == b.Pointer()
	}
	panic("ka: equalNodes: unexpected kind " + a.Kind().String())
}

// declaredNames are the names files declare, as lookups name them: each
// declaration's name in its package or class, and the classes, by their
// dotted names, whose members a lookup of any name in them may find.
type declaredNames struct {
	names   map[[2]string]bool // scope, name
	classes map[string]bool
	facades []string // the changed files' facade classes' simple names, with Java sources (addFacade)
	// nameKeys and classKeys are the names' and classes' keys (seal).
	nameKeys, classKeys []uint64
}

func newDeclaredNames() *declaredNames {
	return &declaredNames{names: map[[2]string]bool{}, classes: map[string]bool{}}
}

// hit reports whether one of ls looks up a declared name or in a declared class.
func (n *declaredNames) hit(ls []Lookup) bool {
	for _, l := range ls {
		if n.classes[l.Scope] || n.names[[2]string{l.Scope, l.Name}] {
			return true
		}
	}
	return false
}

func (n *declaredNames) add(scope, name string) {
	if name != "" && !strings.HasPrefix(name, "<") {
		n.names[[2]string{scope, name}] = true
	}
}

// declaredBy reports whether f, a model, declares one of the names.
func (n *declaredNames) declaredBy(f *File) bool {
	var walk func(scope string, ds []*Decl) bool
	walk = func(scope string, ds []*Decl) bool {
		for _, d := range ds {
			if n.names[[2]string{scope, d.Name}] {
				return true
			}
			if d.ClassID != "" && walk(strings.ReplaceAll(d.ClassID, "/", "."), d.Members) {
				return true
			}
		}
		return false
	}
	return walk(f.Package, f.Decls)
}

// addModel adds the declarations of f, a model.
func (n *declaredNames) addModel(f *File) { n.addModelOf(f, nil) }

// addModelOf adds the declarations of f, a model, whose top-level
// declarations' names top reports true for, all if top is nil.
func (n *declaredNames) addModelOf(f *File, top func(name string) bool) {
	var walk func(scope string, ds []*Decl)
	walk = func(scope string, ds []*Decl) {
		for _, d := range ds {
			n.add(scope, d.Name)
			if d.ClassID != "" {
				cls := strings.ReplaceAll(d.ClassID, "/", ".")
				n.classes[cls] = true
				walk(cls, d.Members)
			}
		}
	}
	var ds []*Decl
	for _, d := range f.Decls {
		if top == nil || top(d.Name) {
			ds = append(ds, d)
		}
	}
	walk(f.Package, ds)
}

// addSyntax adds the declarations of f, as lowered.
func (n *declaredNames) addSyntax(f *syntax.File) { n.addSyntaxOf(f, nil) }

// addSyntaxOf adds the declarations of f, as lowered, whose top-level
// declarations' names top reports true for, all if top is nil.
func (n *declaredNames) addSyntaxOf(f *syntax.File, top func(name string) bool) {
	var walk func(scope string, ds []syntax.Decl)
	walk = func(scope string, ds []syntax.Decl) {
		for _, d := range ds {
			if top != nil && scope == f.Package && !top(syntaxName(d)) {
				continue
			}
			switch d := d.(type) {
			case *syntax.FunDecl:
				n.add(scope, d.Name)
			case *syntax.PropDecl:
				n.add(scope, d.Name)
			case *syntax.TypeAliasDecl:
				n.add(scope, d.Name)
			case *syntax.ClassDecl:
				n.add(scope, d.Name)
				cls := d.Name
				if scope != "" {
					cls = scope + "." + d.Name
				}
				n.classes[cls] = true
				for _, p := range d.CtorParams {
					n.add(cls, p.Name)
				}
				for _, en := range d.Entries {
					n.add(cls, en.Name)
				}
				walk(cls, d.Members)
			}
		}
	}
	walk(f.Package, f.Decls)
}

// syntaxName returns the name d declares, "" if none.
func syntaxName(d syntax.Decl) string {
	switch d := d.(type) {
	case *syntax.FunDecl:
		return d.Name
	case *syntax.PropDecl:
		return d.Name
	case *syntax.TypeAliasDecl:
		return d.Name
	case *syntax.ClassDecl:
		return d.Name
	}
	return ""
}

// javaSet holds a module's Java sources: their texts, and their parses,
// nil for one kc can't parse.
type javaSet struct {
	texts []string
	files []*javasrc.File
}

// allFiles returns every index below n: what dependents returns when it
// can't tell which files a change reaches.
func allFiles(n int) map[int]bool {
	all := make(map[int]bool, n)
	for i := range n {
		all[i] = true
	}
	return all
}

// addFacade adds the facade class of the Kotlin source at path, whose text
// was old and is now text, if there are Java sources, which may name it.
// It reports false if either text may rename the facade (@JvmName,
// @JvmMultifileClass), which a Java source may then name unseen.
func (n *declaredNames) addFacade(path, old, text string, java *javaSet) bool {
	if java == nil || len(java.texts) == 0 {
		return true
	}
	for _, t := range []string{old, text} {
		if strings.Contains(t, "JvmName") || strings.Contains(t, "JvmMultifileClass") {
			return false
		}
	}
	base := strings.TrimSuffix(filepath.Base(path), ".kt")
	if base == "" {
		return false
	}
	n.facades = append(n.facades, strings.ToUpper(base[:1])+base[1:]+"Kt")
	return true
}

// addJava adds, as classes the changed files declare, the classes of the
// Java sources whose texts name one of those classes or facades, by its
// simple name: those Java classes' signatures may change with them, which
// a Kotlin file using them doesn't look up. It reports false if a Java
// source kc can't parse names one, whose classes it doesn't know.
func (n *declaredNames) addJava(java *javaSet) bool {
	if java == nil || len(java.texts) == 0 {
		return true
	}
	simple := map[string]bool{}
	for cls := range n.classes {
		simple[cls[strings.LastIndexByte(cls, '.')+1:]] = true
	}
	for _, f := range n.facades {
		simple[f] = true
	}
	var add func(scope string, cs []*javasrc.Class)
	add = func(scope string, cs []*javasrc.Class) {
		for _, c := range cs {
			cls := c.Name
			if scope != "" {
				cls = scope + "." + c.Name
			}
			n.classes[cls] = true
			add(cls, c.Nested)
		}
	}
	for i, text := range java.texts {
		if !namesAny(text, simple) {
			continue
		}
		f := java.files[i]
		if f == nil {
			return false
		}
		add(f.Package, f.Classes)
	}
	return true
}

// namesAny reports whether text has one of names as a Java identifier.
func namesAny(text string, names map[string]bool) bool {
	isID := func(b byte) bool {
		return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
	}
	for i := 0; i < len(text); {
		if !isID(text[i]) {
			i++
			continue
		}
		j := i
		for j < len(text) && isID(text[j]) {
			j++
		}
		if names[text[i:j]] {
			return true
		}
		i = j
	}
	return false
}

// keep adds e to c, dropping the oldest past maxChecks.
func (c *checkCache) keep(e *checked) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list = append(c.list, e)
	if n := len(c.list) - maxChecks; n > 0 {
		c.list = slices.Delete(c.list, 0, n)
	}
	for snap := range c.ids {
		if !slices.ContainsFunc(c.list, func(e *checked) bool { s, ok := e.snaps[snap.Path()]; return ok && s.snap == snap }) {
			delete(c.ids, snap)
		}
	}
	if c.keys != nil {
		held := map[*File]bool{}
		for _, e := range c.list {
			for _, f := range e.files {
				held[f] = true
			}
		}
		for f := range c.keys {
			if !held[f] {
				delete(c.keys, f)
			}
		}
	}
}
