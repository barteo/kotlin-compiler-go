package driver

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// Libraries keeps the class paths and libraries of a process's compiles,
// for the later compiles of the same configuration: indexing the JDK and
// reading the standard library's classes is most of the work of
// compiling a small program, as a test is. It is set in Options by a
// process that compiles many times, as kc -serve does.
//
// The checker keeps what it learns of the class path's classes in the
// library and what it learns of the program's in each check's own
// Universe, so checks one after another can share a library (as
// analysis/ka's sessions do). Each compile gets its own copy of the
// library's settings.
//
// A configuration is shared only if its class path is the JDK and jars,
// which are identified by their size and modification time: a
// directory's classes can change without the directory's own time
// changing. Compiles with Java sources, which are added to the class
// path, aren't shared either.
type Libraries struct {
	mu   sync.Mutex
	list []*sharedLibrary // most recently used last
	// Max is how many libraries no compile uses are kept; 4 if 0.
	Max int
}

type sharedLibrary struct {
	key  string
	cp   *classpath.Path
	lib  *check.Library
	refs int // the compiles using it, guarded by Libraries.mu
}

// get returns the class path and a library for opts, and a function the
// caller calls once it is done with them. The library is shared, if opts
// allow, with the caller's own copy of the settings; otherwise both are
// opened as for a compile without Libraries, and done closes the class
// path.
func (ls *Libraries) get(opts Options) (cp *classpath.Path, lib *check.Library, done func(), shared bool, err error) {
	key := libraryKey(opts)
	if key == "" {
		cp, err := openClassPath(opts)
		if err != nil {
			return nil, nil, nil, false, err
		}
		return cp, newLibrary(cp, opts), func() { cp.Close() }, false, nil
	}
	ls.mu.Lock()
	defer ls.mu.Unlock()
	var s *sharedLibrary
	for i, t := range ls.list {
		if t.key == key {
			s = t
			ls.list = slices.Delete(ls.list, i, i+1)
			break
		}
	}
	if s == nil {
		if cp, err = openClassPath(opts); err != nil {
			return nil, nil, nil, false, err
		}
		s = &sharedLibrary{key: key, cp: cp, lib: newLibrary(cp, opts)}
	}
	ls.list = append(ls.list, s)
	s.refs++
	ls.trimLocked()
	done = sync.OnceFunc(func() {
		ls.mu.Lock()
		defer ls.mu.Unlock()
		s.refs--
		ls.trimLocked()
	})
	return s.cp, configure(s.lib.Clone(), opts), done, true, nil
}

// trimLocked closes the least recently used libraries no compile uses,
// past Max.
func (ls *Libraries) trimLocked() {
	idle := 0
	for _, s := range ls.list {
		if s.refs == 0 {
			idle++
		}
	}
	ls.list = slices.DeleteFunc(ls.list, func(s *sharedLibrary) bool {
		if s.refs > 0 || idle <= cmp.Or(ls.Max, 4) {
			return false
		}
		idle--
		s.cp.Close()
		return true
	})
}

// Close closes the class paths kept. The compiles using them must be
// done.
func (ls *Libraries) Close() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	for _, s := range ls.list {
		s.cp.Close()
	}
	ls.list = nil
}

// configure sets the settings of lib that each check reads for itself,
// which compiles sharing a library can differ in.
func configure(lib *check.Library, opts Options) *check.Library {
	lib.Module = opts.ModuleName
	lib.OptIns = opts.OptIns
	lib.MetroPlan = nil
	lib.MetroOptions = nil
	return lib
}

// libraryKey returns what identifies the library of opts across
// compiles: its class path, with the size and modification time of the
// JDK's class image and of each jar, and the settings that the library's
// classes are read with, or that checks might compute shared results
// with. It returns "" if the library can't be shared: if the class path
// has a directory or a file that can't be read.
func libraryKey(opts Options) string {
	jdk := opts.JDKHome
	if jdk == "" && !opts.NoJDK {
		jdk, _ = classpath.FindJDK()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "jdk=%q friends=%q hidden=%t javatypes=%t serialization=%t parcelize=%t compose=%t nojvmdefault=%t jvmdefaultcompat=%t\n",
		jdk, opts.FriendPaths, opts.KeepHidden, opts.JavaTypes, opts.Serialization, opts.Parcelize, opts.Compose,
		opts.NoJVMDefault, opts.JVMDefaultCompat)
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
