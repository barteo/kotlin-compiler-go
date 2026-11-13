package ka

import (
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

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
// key, whose parses an update may reuse, or nil. A nil c keeps none.
func (c *checkCache) find(key string, paths, texts []string) (*checked, bool) {
	if c == nil || noReuse {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var last *checked
	for i := len(c.list) - 1; i >= 0; i-- {
		e := c.list[i]
		if e.key != key {
			continue
		}
		if slices.Equal(e.paths, paths) && slices.Equal(e.texts, texts) {
			c.list = append(slices.Delete(c.list, i, i+1), e)
			return e, true
		}
		if last == nil {
			last = e
		}
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
// declarations checks, if e is a clean check of the same files, without
// Java sources, and the changed files parse without errors: the changed
// files, and those whose lookups in e find what the changed files declare
// before (e's models) or after the edit (files, as lowered), or look in
// the classes they declare (declaredNames.hit), or that declare the same
// (declaredNames.declaredBy), which may conflict. A file's model depends on
// another's declarations only through the names it looks up, which the
// checker records as FIR does, and more; but a model that changes because
// of them, an inferred type, may change others', which dependents checks
// once the files are checked. It returns nil otherwise, and if no text
// changed or more than 1/maxDirtyShare of the files would be checked.
func (e *checked) declEdits(paths []string, sources []Source, texts, javaTexts []string, snaps []*psi.Snapshot, files []*syntax.File) map[int]bool {
	if e == nil || !e.clean || !slices.Equal(e.paths, paths) || len(javaTexts) > 0 || len(e.files) != len(sources) {
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
	}
	if len(dirty) == 0 {
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
func (e *checked) dependents(dirty map[int]bool, files []*File, texts []string) map[int]bool {
	names := newDeclaredNames()
	changed := false
	for i := range dirty {
		if e.snaps[files[i].Path].text != texts[i] || sameDecls(e.files[i], files[i]) {
			continue
		}
		changed = true
		names.addModel(e.files[i])
		names.addModel(files[i])
	}
	if !changed {
		return nil
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

// keep adds e to c, dropping the oldest past maxChecks.
func (c *checkCache) keep(e *checked) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list = append(c.list, e)
	if n := len(c.list) - maxChecks; n > 0 {
		c.list = slices.Delete(c.list, 0, n)
	}
}
