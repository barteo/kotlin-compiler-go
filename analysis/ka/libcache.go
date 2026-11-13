package ka

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
)

// Libraries. Opening the class path is cheap, but reading its classes is
// not: checking reads the ones the sources use, and each check of a fresh
// library reads them again. So a Session's Updates share one library, and
// so do the sessions of a process (a Gradle daemon running KSP again and
// again) whose configurations are the same and whose jars haven't changed
// since. A library is shared as long as a session or model uses it, and
// the last maxIdle unused ones are kept for the next session.
//
// The checker keeps what it learns of the class path's classes in the
// library and what it learns of the program's in each check's own
// Universe (check.Universe.cache), so checks one after another, or at the
// same time, can share a library. A module's Java sources, whose classes
// the program's classes are found through, are put over it for each
// check (library.withJava).

// maxIdle is how many libraries no session or model uses are kept.
const maxIdle = 2

// shared is a library with the number of sessions and models using it.
type shared struct {
	key  string // "" if the library isn't shared across sessions
	lib  *library
	refs int // guarded by libraries.mu

	checks checkCache // the last checks against lib; see reuse.go
}

var libraries struct {
	mu   sync.Mutex
	list []*shared // the libraries sessions can share, most recently used last
}

// acquire returns a library for cfg that the caller must release: a
// shared one if there is one for cfg, else one it opens.
func acquire(cfg Config) (*shared, error) {
	key := libraryKey(cfg)
	libraries.mu.Lock()
	if key != "" {
		for i, s := range libraries.list {
			if s.key == key {
				s.refs++
				libraries.list = append(slices.Delete(libraries.list, i, i+1), s)
				libraries.mu.Unlock()
				return s, nil
			}
		}
	}
	libraries.mu.Unlock()
	// Opening reads the jars' indexes: not under the lock. Two sessions
	// opening the same library at once share the first one listed.
	cp, lib, err := driver.OpenLibrary(driver.Options{
		JDKHome: cfg.JDKHome, NoJDK: cfg.NoJDK, ClassPath: cfg.ClassPath, FriendPaths: cfg.FriendPaths, KeepHidden: true, JavaTypes: true,
		ModuleName: cfg.ModuleName, Jobs: max(cfg.Jobs, 1),
	})
	if err != nil {
		return nil, fmt.Errorf("ka: %w", err)
	}
	s := &shared{key: key, refs: 1, lib: &library{cp: cp, lib: lib, decls: map[string]*Decl{}, params: map[string]map[string]string{}}}
	if cp.HasSources() {
		s.key = ""
	}
	if s.key == "" {
		return s, nil
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
	libraries.list = append(libraries.list, s)
	trimLocked()
	return s, nil
}

// retain adds a user to s.
func (s *shared) retain() {
	libraries.mu.Lock()
	s.refs++
	libraries.mu.Unlock()
}

// release removes a user from s, closing its class path once it has none
// and isn't kept for other sessions.
func (s *shared) release() error {
	libraries.mu.Lock()
	defer libraries.mu.Unlock()
	if s.refs--; s.refs > 0 {
		return nil
	}
	if s.key == "" {
		return s.lib.cp.Close()
	}
	return trimLocked()
}

// trimLocked closes the least recently used unused libraries past
// maxIdle.
func trimLocked() error {
	idle := 0
	for _, s := range libraries.list {
		if s.refs == 0 {
			idle++
		}
	}
	var err error
	libraries.list = slices.DeleteFunc(libraries.list, func(s *shared) bool {
		if s.refs > 0 || idle <= maxIdle {
			return false
		}
		idle--
		if e := s.lib.cp.Close(); err == nil {
			err = e
		}
		return true
	})
	return err
}

// libraryKey returns what identifies the library of cfg across sessions:
// its configuration and the size and modification time of the JDK's
// class image and of each jar. It returns "" if the library can't be
// shared: if the class path has a directory, whose classes can change
// without the directory's own time changing, or a file can't be read.
func libraryKey(cfg Config) string {
	jdk := cfg.JDKHome
	if jdk == "" && !cfg.NoJDK {
		jdk, _ = classpath.FindJDK()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "module=%q nojdk=%t friends=%q\n", cfg.ModuleName, cfg.NoJDK, cfg.FriendPaths)
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
	for _, p := range cfg.ClassPath {
		if !stamp(p) {
			return ""
		}
	}
	return b.String()
}
