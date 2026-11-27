package analysis

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// A Session analyzes the versions of one module's sources against a
// Library, as an editor asks for them: each Update analyzes the sources
// as they are now, as Library.Analyze does, and has the same Result. An
// update that only changes bodies checks only the changed files, and one
// that changes declarations also the files that use them
// (incremental.go); the others keep their facts and diagnostics from the
// last update.
//
// Updates may run at the same time; each starts from the last update
// that finished. Close the session before its Library.
type Session struct {
	lib *Library

	mu     sync.Mutex
	last   *state // what the next update may start from, or nil
	closed bool

	updates, incremental atomic.Int64
}

// Session returns a new session of the library's module.
func (l *Library) Session() *Session { return &Session{lib: l} }

// SessionStats counts a session's updates.
type SessionStats struct {
	// Updates are the updates that finished, but completions; of them,
	// Incremental checked only some files.
	Updates, Incremental int64
}

// Stats returns the session's counters.
func (s *Session) Stats() SessionStats {
	return SessionStats{Updates: s.updates.Load(), Incremental: s.incremental.Load()}
}

// Update analyzes snapshots as the Kotlin sources of the session's
// module, as Library.Analyze does. An update that asks for a completion
// (Options.Complete) doesn't change what the next update starts from.
func (s *Session) Update(ctx context.Context, snapshots []*psi.Snapshot, opts Options) (res *Result, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, snap := range snapshots {
		if snap == nil {
			return nil, errors.New("analysis: nil snapshot")
		}
		if strings.HasSuffix(snap.Path(), ".kts") {
			return nil, fmt.Errorf("%w: script %s", ErrUnsupported, snap.Path())
		}
	}
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("analysis: internal error: %v\n%s", r, debug.Stack())
		}
	}()
	if opts.Complete != nil {
		return s.lib.analyze(ctx, snapshots, opts)
	}
	s.mu.Lock()
	last, closed := s.last, s.closed
	s.mu.Unlock()
	if closed {
		return nil, errors.New("analysis: session closed")
	}
	if last != nil {
		res, next, ok, err := s.lib.incremental(ctx, snapshots, opts, last)
		if err != nil {
			return nil, err
		}
		if ok {
			s.keep(next)
			s.incremental.Add(1)
			return res, nil
		}
	}
	res, next, err := s.lib.run(ctx, snapshots, opts, true)
	if err != nil {
		return nil, err
	}
	s.keep(next)
	return res, nil
}

// keep makes st what the next update starts from.
func (s *Session) keep(st *state) {
	s.mu.Lock()
	if !s.closed {
		s.last = st
	}
	s.mu.Unlock()
	s.updates.Add(1)
}

// Close forgets what the next update would start from; later updates
// fail. Results stay usable.
func (s *Session) Close() error {
	s.mu.Lock()
	s.closed, s.last = true, nil
	s.mu.Unlock()
	return nil
}
