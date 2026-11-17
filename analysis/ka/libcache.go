package ka

import (
	"fmt"
	"os"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/kasession"
)

// Libraries are kasession's, shared with the sessions of package analysis
// that use the same options. What ka keeps with one, its models of the
// class path's classes and its last checks, is attached to it: shared. A
// module's Java sources, whose classes the program's classes are found
// through, are put over it for each check (library.withJava).

// shared is what ka keeps with a library.
type shared struct {
	l   *kasession.Library
	lib *library

	checks checkCache // the last checks against lib; see reuse.go
}

// attachment is ka's key for its shared in a kasession.Library.
type attachment struct{}

func attach(l *kasession.Library) *shared {
	return l.Attach(attachment{}, func() any {
		return &shared{l: l, lib: &library{cp: l.Path, lib: l.Lib, decls: map[string]*Decl{}, params: map[string]map[string]string{}}}
	}).(*shared)
}

// libraryOptions returns the options ka opens cfg's library with.
func libraryOptions(cfg Config) driver.Options {
	return driver.Options{
		JDKHome: cfg.JDKHome, NoJDK: cfg.NoJDK, ClassPath: cfg.ClassPath, FriendPaths: cfg.FriendPaths, KeepHidden: true, JavaTypes: true,
		ModuleName: cfg.ModuleName, Jobs: cfg.Jobs,
	}
}

// acquire returns a library for cfg that the caller must release: a
// shared one if there is one for cfg, else one it opens.
func acquire(cfg Config) (*shared, error) {
	cfg.ClassPath = existing(cfg.ClassPath)
	cfg.FriendPaths = existing(cfg.FriendPaths)
	l, err := kasession.Acquire(libraryOptions(cfg))
	if err != nil {
		return nil, fmt.Errorf("ka: %w", err)
	}
	return attach(l), nil
}

// existing returns the paths that exist. A build lists class path entries
// that need not exist (an output directory nothing is written to), and
// kotlinc and stock KSP leave those out.
func existing(paths []string) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// retain adds a user to s.
func (s *shared) retain() { s.l.Retain() }

// release removes a user from s.
func (s *shared) release() error { return s.l.Release() }

// libraryKey returns what identifies the library of cfg across sessions,
// or "" if it can't be shared (kasession.Key).
func libraryKey(cfg Config) string { return kasession.Key(libraryOptions(cfg)) }
