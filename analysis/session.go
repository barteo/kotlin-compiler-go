package analysis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/kasession"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// A Session analyzes the versions of one module's sources, as an editor
// asks for them: each Update analyzes the sources as they are now.
//
// A session's updates share the library the class path's classes are
// read into, as do the sessions of a process whose configurations and
// jars are the same (internal/kasession); Close releases it.
//
// Unlike AnalyzeArena, an update with errors still has facts: those of
// every file without errors (Result.Complete).
type Session struct {
	cfg Config

	mu     sync.Mutex
	lib    *kasession.Library // nil until the first Update
	closed bool
}

// Open returns a session for the module cfg describes.
func Open(cfg Config) *Session { return &Session{cfg: cfg} }

// libraryOpens counts the libraries sessions open, for tests.
var libraryOpens atomic.Int64

// Update analyzes snapshots as the module's sources. It returns an error if
// ctx is done, the configuration is invalid, the class path can't be
// opened, or a snapshot is a script (ErrUnsupported). Errors in the
// sources are the Result's Diagnostics. Updates may run at the same time.
func (s *Session) Update(ctx context.Context, snapshots []*psi.Snapshot) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validate(s.cfg, snapshots); err != nil {
		return nil, err
	}
	return run(ctx, s.cfg, snapshots, true, s.library)
}

// library returns the session's library, opening it on first use or
// again when its jars changed.
func (s *Session) library() (*classpath.Path, *check.Library, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, nil, nil, errors.New("analysis: session closed")
	}
	opts := libraryOptions(s.cfg)
	if s.lib == nil || !s.lib.Current(opts) {
		l, err := kasession.Acquire(opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("analysis: %w", err)
		}
		libraryOpens.Add(1)
		if s.lib != nil {
			s.lib.Release()
		}
		s.lib = l
	}
	l := s.lib
	l.Retain()
	return l.Path, l.Lib, func() { l.Release() }, nil
}

// Close releases the session's library. Results stay usable.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.lib == nil {
		return nil
	}
	err := s.lib.Release()
	s.lib = nil
	return err
}
