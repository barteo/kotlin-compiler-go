package ka

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

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

// incrementalUpdates counts the updates that check only some files, for
// tests.
var incrementalUpdates atomic.Int64

// declUpdates counts the updates that check some files after changes to
// what files declare (declEdits), for tests.
var declUpdates atomic.Int64

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
}

// checked is the result of checking a module's sources.
type checked struct {
	check        uint64                 // the Model.Check of its models
	key          string                 // "", or what else must match for find
	paths, texts []string               // the Kotlin sources', then the Java sources'
	snaps        map[string]checkedSnap // the Kotlin sources' parses, by path
	files        []*File
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
		names.addModel(e.files[i])
		names.addSyntax(files[i])
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
	e.addLookingUp(dirty, names)
	if len(dirty)*maxDirtyShare > len(sources) {
		return nil
	}
	return dirty
}

// dependents returns the files that look up what the files in dirty whose
// texts are e's declare, if their models in files, as checked again,
// differ from e's: those files' models may depend on theirs (declEdits).
func (e *checked) dependents(dirty map[int]bool, files []*File, texts []string, java *javaSet) map[int]bool {
	names := newDeclaredNames()
	changed := false
	for i := range dirty {
		if e.snaps[files[i].Path].text != texts[i] || sameDecls(e.files[i], files[i]) {
			continue
		}
		changed = true
		names.addModel(e.files[i])
		names.addModel(files[i])
		if !names.addFacade(files[i].Path, texts[i], texts[i], java) {
			return allFiles(len(files))
		}
	}
	if !changed {
		return nil
	}
	if !names.addJava(java) {
		return allFiles(len(files))
	}
	more := map[int]bool{}
	for j, f := range e.files {
		if !dirty[j] && (names.hit(f.Lookups) || names.declaredBy(f)) {
			more[j] = true
		}
	}
	return more
}

// addLookingUp adds to dirty the files whose lookups in e hit names, and
// those that declare one of them, whose declarations may conflict.
func (e *checked) addLookingUp(dirty map[int]bool, names *declaredNames) {
	for j, f := range e.files {
		if !dirty[j] && (names.hit(f.Lookups) || names.declaredBy(f)) {
			dirty[j] = true
		}
	}
}

// sameDecls reports whether a and b, models of the same text, declare the
// same: their annotations and declarations are equal.
func sameDecls(a, b *File) bool {
	return a.Package == b.Package && reflect.DeepEqual(a.Annotations, b.Annotations) && reflect.DeepEqual(a.Decls, b.Decls)
}

// declaredNames are the names files declare, as lookups name them: each
// declaration's name in its package or class, and the classes, by their
// dotted names, whose members a lookup of any name in them may find.
type declaredNames struct {
	names   map[[2]string]bool // scope, name
	classes map[string]bool
	facades []string // the changed files' facade classes' simple names, with Java sources (addFacade)
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
func (n *declaredNames) addModel(f *File) {
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
	walk(f.Package, f.Decls)
}

// addSyntax adds the declarations of f, as lowered.
func (n *declaredNames) addSyntax(f *syntax.File) {
	var walk func(scope string, ds []syntax.Decl)
	walk = func(scope string, ds []syntax.Decl) {
		for _, d := range ds {
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
}
