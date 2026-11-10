package ka

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// reachAt is a top-level declaration: its file's index and its own in the
// file's names (fileNames.decls).
type reachAt struct{ file, decl int }

// reachIndex is reachable's index of what a module's files declare: the
// declarations that declare each name, the sealed classes' names, and the
// declarations whose classes' supertype lists write each name. An update
// that checks only some files builds it over all the files, for each
// check, though between two updates most files are the same parses: a
// daemon's edit changes one. So the check cache keeps the indexes of the
// last updates, by their files' paths (a KSP run's rounds alternate
// between two lists, the second with the generated files), and an update
// over the same paths takes its index and changes only the entries of the
// files whose parses differ (their fileNames, which the cache keeps by
// parse).
type reachIndex struct {
	paths    []string
	fns      []*fileNames
	declares map[string][]reachAt
	sealed   map[string]int // how many declarations declare a sealed class of the name
	subs     map[string][]reachAt
}

// keptReachIndexes is how many indexes the check cache keeps: a KSP
// run's updates, and a few more.
const keptReachIndexes = 4

func newReachIndex(paths []string) *reachIndex {
	return &reachIndex{paths: paths, fns: make([]*fileNames, len(paths)),
		declares: map[string][]reachAt{}, sealed: map[string]int{}, subs: map[string][]reachAt{}}
}

// set makes fns the files' names, changing the entries of the files whose
// names differ from those the index has.
func (x *reachIndex) set(fns []*fileNames) {
	for i, fn := range fns {
		if x.fns[i] == fn {
			continue
		}
		if x.fns[i] != nil {
			x.remove(i)
			reachIndexPatched.Add(1)
		}
		x.add(i, fn)
	}
}

func (x *reachIndex) add(i int, fn *fileNames) {
	x.fns[i] = fn
	for k, d := range fn.decls {
		for _, n := range d.names {
			x.declares[n] = append(x.declares[n], reachAt{i, k})
		}
		for _, n := range d.sealed {
			x.sealed[n]++
		}
		for n := range d.supers {
			x.subs[n] = append(x.subs[n], reachAt{i, k})
		}
	}
}

func (x *reachIndex) remove(i int) {
	fn := x.fns[i]
	x.fns[i] = nil
	for _, d := range fn.decls {
		for _, n := range d.names {
			dropFile(x.declares, n, i)
		}
		for _, n := range d.sealed {
			if x.sealed[n]--; x.sealed[n] == 0 {
				delete(x.sealed, n)
			}
		}
		for n := range d.supers {
			dropFile(x.subs, n, i)
		}
	}
}

// dropFile removes file i's declarations from m[n].
func dropFile(m map[string][]reachAt, n string, i int) {
	as, ok := m[n]
	if !ok {
		return
	}
	as = slices.DeleteFunc(as, func(a reachAt) bool { return a.file == i })
	if len(as) == 0 {
		delete(m, n)
	} else {
		m[n] = as
	}
}

// reachIndex returns the index of the files of snaps, whose names are fns:
// one the cache kept for the same paths, changed to fns, or a new one.
// The caller owns it until it gives it back (putReachIndex).
func (c *checkCache) reachIndex(snaps []*psi.Snapshot, fns []*fileNames) *reachIndex {
	paths := make([]string, len(snaps))
	for i, s := range snaps {
		paths[i] = s.Path()
	}
	var x *reachIndex
	if c != nil && !noReachIndex {
		c.mu.Lock()
		for j, k := range c.reach {
			if slices.Equal(k.paths, paths) {
				x = k
				c.reach = slices.Delete(c.reach, j, j+1)
				break
			}
		}
		c.mu.Unlock()
	}
	if x == nil {
		x = newReachIndex(paths)
	}
	x.set(fns)
	if verifyReach {
		fresh := newReachIndex(paths)
		fresh.set(fns)
		if err := x.diff(fresh); err != nil {
			panic("ka: kept reachable index differs: " + err.Error())
		}
	}
	return x
}

// putReachIndex keeps x for the updates after, the most recent first.
func (c *checkCache) putReachIndex(x *reachIndex) {
	if c == nil || noReachIndex {
		return
	}
	c.mu.Lock()
	c.reach = slices.Insert(c.reach, 0, x)
	if len(c.reach) > keptReachIndexes {
		c.reach = c.reach[:keptReachIndexes]
	}
	c.mu.Unlock()
}

// diff describes how x's entries differ from y's, as sets, or is nil.
func (x *reachIndex) diff(y *reachIndex) error {
	same := func(a, b []reachAt) bool {
		cmp := func(p, q reachAt) int {
			if p.file != q.file {
				return p.file - q.file
			}
			return p.decl - q.decl
		}
		a, b = slices.Clone(a), slices.Clone(b)
		slices.SortFunc(a, cmp)
		slices.SortFunc(b, cmp)
		return slices.Equal(a, b)
	}
	if !maps.EqualFunc(x.declares, y.declares, same) {
		return fmt.Errorf("declares")
	}
	if !maps.Equal(x.sealed, y.sealed) {
		return fmt.Errorf("sealed")
	}
	if !maps.EqualFunc(x.subs, y.subs, same) {
		return fmt.Errorf("subs")
	}
	return nil
}

// reachIndexPatched counts the files whose entries a kept index changed,
// for tests.
var reachIndexPatched atomic.Int64

// verifyReach makes reachIndex check a kept index against a new one, for
// tests.
var verifyReach bool

// noReachIndex turns the kept indexes off, for comparisons
// (KC_NO_REACH_INDEX=1): reachable builds its index for each update.
var noReachIndex = os.Getenv("KC_NO_REACH_INDEX") != ""
