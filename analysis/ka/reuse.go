package ka

import (
	"slices"
	"sync"
	"sync/atomic"

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
// A module without Java sources keeps its checks in the library its
// updates share (shared.checks). One with Java sources is checked against
// a library of its own, made with those sources; javaChecks keeps such
// checks, with their libraries, by the class path's key (libraryKey).
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

// noReuse turns the caches off, for tests that compare an update with a
// whole check.
var noReuse bool

// maxChecks is how many checks' results a cache keeps.
const maxChecks = 2

// javaChecks are the last checks of modules with Java sources.
var javaChecks checkCache

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
	key          string                 // the class path's key, for javaChecks
	paths, texts []string               // the Kotlin sources', then the Java sources'
	snaps        map[string]checkedSnap // the Kotlin sources' parses, by path
	files        []*File
	diags        []Diagnostic
	sl           *shared // the library the check made, for javaChecks
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

// keep adds e to c, releasing the libraries of the checks it drops.
func (c *checkCache) keep(e *checked) {
	c.mu.Lock()
	c.list = append(c.list, e)
	var dropped []*checked
	if n := len(c.list) - maxChecks; n > 0 {
		dropped = slices.Clone(c.list[:n])
		c.list = slices.Delete(c.list, 0, n)
	}
	c.mu.Unlock()
	for _, d := range dropped {
		if d.sl != nil {
			d.sl.release()
		}
	}
}
