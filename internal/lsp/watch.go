package lsp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
)

// Files created and deleted when the client doesn't watch them, and
// re-import. A client that registers file watchers tells the server of
// them (didChangeWatchedFiles); without, the server lists a module's
// sources again when a file it doesn't list is opened, saved or closed,
// and scans the modules' roots every scanEvery for the others.

// scanEvery is how often the server lists the sources again when the
// client doesn't watch them.
const scanEvery = 5 * time.Second

// reimportCommand reloads the projects, running Gradle even if the model
// kept is current.
const reimportCommand = "kc.reimport"

// relistUnlisted lists again the sources of the modules whose roots hold
// path, a Kotlin file, if they don't list it.
func (s *Server) relistUnlisted(path string) {
	if !strings.HasSuffix(path, ".kt") {
		return
	}
	for _, m := range s.modulesOf(path) {
		m.mu.Lock()
		unlisted := m.sources != nil && !slices.Contains(m.sources, path)
		if unlisted {
			m.sources = nil
		}
		m.mu.Unlock()
		if unlisted {
			m.kickUpdate()
		}
	}
}

// scanLoop scans the sources every interval while the client doesn't
// watch them, until the server stops.
func (s *Server) scanLoop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
		}
		s.mu.Lock()
		watching := s.watching
		s.mu.Unlock()
		if !watching {
			s.scanSources()
		}
	}
}

// scanSources lists the modules' sources again, kicks the modules whose
// list changed, and drops the snapshots of files on disk no module lists.
func (s *Server) scanSources() {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	listed := map[string]bool{}
	for _, w := range ws {
		w.mu.Lock()
		p := w.proj
		var mods []*module
		for _, m := range w.mods {
			mods = append(mods, m)
		}
		w.mu.Unlock()
		if p == nil {
			continue
		}
		for _, m := range mods {
			paths, err := p.Sources(m.m)
			if err != nil {
				continue
			}
			if paths == nil {
				paths = []string{}
			}
			for _, path := range paths {
				listed[path] = true
			}
			m.mu.Lock()
			changed := m.sources != nil && !slices.Equal(m.sources, paths)
			if changed {
				m.sources = paths
			}
			m.mu.Unlock()
			if changed {
				m.kickUpdate()
			}
		}
	}
	s.disk.keep(listed)
}

// keep drops the snapshots of the files not in paths.
func (c *diskCache) keep(paths map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for p := range c.files {
		if !paths[p] {
			delete(c.files, p)
		}
	}
}

// reloadAll reloads every workspace's project, refreshing Gradle's model
// if refresh is set.
func (s *Server) reloadAll(refresh bool) {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	for _, w := range ws {
		s.work.add()
		go func() {
			defer s.work.done()
			w.reload(refresh)
		}()
	}
}

func (s *Server) executeCommand(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p struct {
		Command   string            `json:"command"`
		Arguments []json.RawMessage `json:"arguments,omitempty"`
	}
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	switch p.Command {
	case reimportCommand:
		s.reloadAll(true)
		return nil, nil
	}
	return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "unknown command %q", p.Command)
}
