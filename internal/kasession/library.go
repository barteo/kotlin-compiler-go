// Package kasession holds what the incremental analyses share: libraries
// opened once and kept across sessions (this file), and the hash that
// tells a body edit from a declaration edit (signature.go). Package
// analysis/ka uses them for its signature model, package analysis for its
// sessions.
package kasession

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
)

// Libraries. Opening the class path is cheap, but reading its classes is
// not: checking reads the ones the sources use, and each check of a fresh
// library reads them again. So a session's updates share one library, and
// so do the sessions of a process (a Gradle daemon running KSP again and
// again, an editor's modules) whose configurations are the same and whose
// jars haven't changed since. A library is shared as long as someone holds
// it, and the last MaxIdle unused ones are kept for the next session.
//
// The checker keeps what it learns of the class path's classes in the
// library and what it learns of the program's in each check's own
// Universe (check.Universe.cache), so checks one after another, or at the
// same time, can share a library. A class path with Java sources, whose
// classes the program's classes are found through, isn't shared.

// MaxIdle is how many libraries nobody holds are kept.
const MaxIdle = 2

// Library is an opened class path and the checker's library over it.
type Library struct {
	Path *classpath.Path
	Lib  *check.Library
	Java *driver.JavaSources // the module's Java sources' classes, or nil

	key  string // "" if the library isn't shared
	refs int    // guarded by libraries.mu

	mu  sync.Mutex
	ext map[any]any // see Attach
}

var libraries struct {
	mu   sync.Mutex
	list []*Library // the libraries sessions can share, most recently used last
}

// Acquire returns a library for opts that the caller must Release: a
// shared one if there is one for opts, else one it opens.
func Acquire(opts driver.Options) (*Library, error) {
	key := Key(opts)
	libraries.mu.Lock()
	if key != "" {
		for i, l := range libraries.list {
			if l.key == key {
				l.refs++
				libraries.list = append(slices.Delete(libraries.list, i, i+1), l)
				libraries.mu.Unlock()
				return l, nil
			}
		}
	}
	libraries.mu.Unlock()
	// Opening reads the jars' indexes: not under the lock. Two sessions
	// opening the same library at once share the first one listed.
	opts.Jobs = max(opts.Jobs, 1)
	cp, lib, err := driver.OpenLibrary(opts)
	if err != nil {
		return nil, err
	}
	l := &Library{Path: cp, Lib: lib, key: key, refs: 1}
	if cp.HasSources() {
		l.key = ""
	}
	if l.key == "" {
		return l, nil
	}
	libraries.mu.Lock()
	defer libraries.mu.Unlock()
	for _, t := range libraries.list {
		if t.key == key {
			t.refs++
			cp.Close()
			return t, nil
		}
	}
	libraries.list = append(libraries.list, l)
	trimLocked()
	return l, nil
}

// AcquireJava returns a library for opts with the classes of the module's
// Java source files, which isn't shared.
func AcquireJava(opts driver.Options, files []*javasrc.File) (*Library, error) {
	opts.Jobs = max(opts.Jobs, 1)
	cp, lib, err := driver.OpenLibrary(opts)
	if err != nil {
		return nil, err
	}
	return &Library{Path: cp, Lib: lib, Java: driver.AddJavaSources(cp, files), refs: 1}, nil
}

// Attach returns the value a user of the library keeps with it under key,
// made by mk the first time. A library's users share what they attach.
func (l *Library) Attach(key any, mk func() any) any {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.ext[key]
	if !ok {
		if l.ext == nil {
			l.ext = map[any]any{}
		}
		v = mk()
		l.ext[key] = v
	}
	return v
}

// Current reports whether l is still the library of opts: its jars haven't
// changed since it was opened. An unshared library, whose class path has a
// directory, is always current: its changes can't be seen.
func (l *Library) Current(opts driver.Options) bool {
	return l.key == "" || l.key == Key(opts)
}

// Retain adds a user to l.
func (l *Library) Retain() {
	libraries.mu.Lock()
	l.refs++
	libraries.mu.Unlock()
}

// Release removes a user from l, closing its class path once it has none
// and isn't kept for other sessions.
func (l *Library) Release() error {
	libraries.mu.Lock()
	defer libraries.mu.Unlock()
	if l.refs--; l.refs > 0 {
		return nil
	}
	if l.key == "" {
		return l.Path.Close()
	}
	return trimLocked()
}

// trimLocked closes the least recently used unused libraries past
// MaxIdle.
func trimLocked() error {
	idle := 0
	for _, l := range libraries.list {
		if l.refs == 0 {
			idle++
		}
	}
	var err error
	libraries.list = slices.DeleteFunc(libraries.list, func(l *Library) bool {
		if l.refs > 0 || idle <= MaxIdle {
			return false
		}
		idle--
		if e := l.Path.Close(); err == nil {
			err = e
		}
		return true
	})
	return err
}

// Listed returns the shared libraries and how many users each has, least
// recently used first; for tests.
func Listed() ([]*Library, []int) {
	libraries.mu.Lock()
	defer libraries.mu.Unlock()
	refs := make([]int, len(libraries.list))
	for i, l := range libraries.list {
		refs[i] = l.refs
	}
	return slices.Clone(libraries.list), refs
}

// Key returns what identifies the library of opts across sessions: the
// options that change what it holds, and the size and modification time of
// the JDK's class image and of each jar. It returns "" if the library
// can't be shared: if the class path has a directory, whose classes can
// change without the directory's own time changing, or a file can't be
// read.
func Key(opts driver.Options) string {
	jdk := opts.JDKHome
	if jdk == "" && !opts.NoJDK {
		jdk, _ = classpath.FindJDK()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "module=%q nojdk=%t friends=%q hidden=%t javatypes=%t optins=%q plugins=%t,%t,%t jvmdefault=%t,%t\n",
		opts.ModuleName, opts.NoJDK, opts.FriendPaths, opts.KeepHidden, opts.JavaTypes, opts.OptIns,
		opts.Parcelize, opts.Serialization, opts.Compose, opts.NoJVMDefault, opts.JVMDefaultCompat)
	stamp := func(path string) bool {
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() {
			return false
		}
		fmt.Fprintf(&b, "%q %d %d\n", path, fi.Size(), fi.ModTime().UnixNano())
		return true
	}
	if jdk != "" && !stamp(filepath.Join(jdk, "lib", "modules")) {
		return ""
	}
	for _, p := range opts.ClassPath {
		if !stamp(p) {
			return ""
		}
	}
	return b.String()
}
