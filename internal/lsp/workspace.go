package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/internal/project"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// The semantic tier (docs/lsp.md, milestone L1). Each workspace folder is a
// project (package project), and each of its modules has an
// analysis.Session and a goroutine that updates it: an edit, open, close
// or file change kicks the modules whose sources hold the file, and a
// module updates 50ms after its last kick, from the open documents'
// snapshots and the other files' on disk. A kick while an update runs
// cancels it, unless the module has never finished one (which would
// start again from nothing).
//
// An update's diagnostics of a file are shown with the document version
// they were computed from. An edit shows the last ones moved by the edit
// (shiftDiagnostics) until the next update replaces them; a diagnostic the
// edit touches is dropped. A file is shown the diagnostics of the module
// it belongs to, not of the modules that depend on it; a file that isn't
// open is shown its diagnostics too, and a file in no module only its
// syntax errors.

// debounce is how long a module waits after a kick before it updates.
var debounce = 50 * time.Millisecond

// initOptions are kc-lsp's initializationOptions.
type initOptions struct {
	// Diagnostics is the kc.diagnostics setting: default, coded or strict.
	Diagnostics string `json:"diagnostics"`
	// Gradle false stops kc-lsp from running Gradle.
	Gradle *bool `json:"gradle"`
}

// workspace is a workspace folder's project and its modules.
type workspace struct {
	s    *Server
	root string

	load sync.Mutex // held while loading

	mu   sync.Mutex
	proj *project.Project
	mods map[string]*module // by name
}

// module is a module's session and its updates.
type module struct {
	w    *workspace
	m    *project.Module
	sess *analysis.Session
	kick chan struct{}
	stop context.CancelFunc
	done chan struct{}

	mu      sync.Mutex
	dirty   bool     // kicked and not yet updated; counts in Server.work
	sources []string // nil until listed
	results map[string]fileResult
	last    *ide.View // the last update's, for navigation
	failed  string    // the last update's error, logged once
}

// fileResult is the last update's diagnostics of a file, and the snapshot
// they are of.
type fileResult struct {
	snap  *psi.Snapshot
	diags []analysis.Diagnostic
	view  *ide.View // the update's, for the hints
}

// activity counts the work under way: workspaces loading and modules
// kicked, for tests.
type activity struct {
	mu   sync.Mutex
	cond *sync.Cond
	n    int
}

func newActivity() *activity {
	a := &activity{}
	a.cond = sync.NewCond(&a.mu)
	return a
}

func (a *activity) add() {
	a.mu.Lock()
	a.n++
	a.mu.Unlock()
}

func (a *activity) done() {
	a.mu.Lock()
	a.n--
	if a.n == 0 {
		a.cond.Broadcast()
	}
	a.mu.Unlock()
}

func (a *activity) wait() {
	a.mu.Lock()
	for a.n > 0 {
		a.cond.Wait()
	}
	a.mu.Unlock()
}

// fileURI returns the file URI of a path.
func fileURI(path string) protocol.DocumentURI {
	return protocol.DocumentURI((&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String())
}

// workspaceRoots returns the directories of the workspace folders, or of
// the root URI.
func workspaceRoots(p *protocol.InitializeParams) []string {
	var roots []string
	add := func(u protocol.DocumentURI) {
		if strings.HasPrefix(string(u), "file:") {
			if r := uriPath(u); !slices.Contains(roots, r) {
				roots = append(roots, r)
			}
		}
	}
	for _, f := range p.WorkspaceFolders {
		add(f.URI)
	}
	if len(roots) == 0 && p.RootURI != "" {
		add(p.RootURI)
	}
	return roots
}

// loadWorkspaces loads the projects of the workspace folders, in the
// background.
func (s *Server) loadWorkspaces(roots []string) {
	for _, r := range roots {
		w := &workspace{s: s, root: r, mods: map[string]*module{}}
		s.mu.Lock()
		s.ws = append(s.ws, w)
		s.mu.Unlock()
		s.work.add()
		go func() {
			defer s.work.done()
			w.reload()
		}()
	}
}

// reload loads w's project again and starts its modules, stopping the old
// ones.
func (w *workspace) reload() {
	w.load.Lock()
	defer w.load.Unlock()
	w.stopModules(true)
	s := w.s
	opts := project.Options{NoGradle: s.noGradle}
	p, err := project.Load(s.ctx, w.root, opts)
	if err != nil {
		if s.ctx.Err() == nil {
			s.showMessage(protocol.MessageError, fmt.Sprintf("kc-lsp: %s: %v; only syntax errors are shown", w.root, err))
		}
		return
	}
	for _, warning := range p.Warnings {
		s.logMessage(protocol.MessageWarning, "kc-lsp: "+warning)
	}
	s.logMessage(protocol.MessageInfo, fmt.Sprintf("kc-lsp: %s: %d modules (%s)", w.root, len(p.Modules), p.Origin))
	w.mu.Lock()
	w.proj = p
	for _, pm := range p.Modules {
		ctx, stop := context.WithCancel(s.ctx)
		cfg := pm.Config()
		cfg.Jobs = runtime.GOMAXPROCS(0) // an update lowers and checks files in parallel
		m := &module{w: w, m: pm, sess: analysis.Open(cfg), kick: make(chan struct{}, 1), stop: stop,
			done: make(chan struct{}), results: map[string]fileResult{}}
		w.mods[pm.Name] = m
		go m.loop(ctx)
		m.kickUpdate()
	}
	w.mu.Unlock()
}

// stopModules stops w's modules and, if clear is set, the diagnostics they
// showed.
func (w *workspace) stopModules(clear bool) {
	w.mu.Lock()
	mods := w.mods
	w.mods = map[string]*module{}
	w.proj = nil
	w.mu.Unlock()
	for _, m := range mods {
		m.stop()
		<-m.done
		m.sess.Close()
		m.mu.Lock()
		if m.dirty {
			m.dirty = false
			w.s.work.done()
		}
		m.mu.Unlock()
	}
	if clear {
		w.s.clearShown(w.root)
	}
}

// modulesOf returns the modules whose sources include path, across the
// workspaces.
func (s *Server) modulesOf(path string) []*module {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	var out []*module
	for _, w := range ws {
		w.mu.Lock()
		if w.proj != nil {
			for _, pm := range w.proj.Dependents(path) {
				out = append(out, w.mods[pm.Name])
			}
		}
		w.mu.Unlock()
	}
	return out
}

// ownerResult returns the last diagnostics of path from the module it
// belongs to.
func (s *Server) ownerResult(path string) (fileResult, bool) {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	for _, w := range ws {
		w.mu.Lock()
		var m *module
		if w.proj != nil {
			if pm := w.proj.ModuleOf(path); pm != nil {
				m = w.mods[pm.Name]
			}
		}
		w.mu.Unlock()
		if m != nil {
			m.mu.Lock()
			r, ok := m.results[path]
			m.mu.Unlock()
			return r, ok
		}
	}
	return fileResult{}, false
}

// kickPath kicks the modules whose sources include path; listAgain lists
// their sources again first, for a file created or deleted.
func (s *Server) kickPath(path string, listAgain bool) {
	for _, m := range s.modulesOf(path) {
		if listAgain {
			m.mu.Lock()
			m.sources = nil
			m.mu.Unlock()
		}
		m.kickUpdate()
	}
}

// kickUpdate asks m to update.
func (m *module) kickUpdate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirty {
		m.dirty = true
		m.w.s.work.add()
	}
	select {
	case m.kick <- struct{}{}:
	default:
	}
}

// loop updates m when it is kicked, until ctx is done.
func (m *module) loop(ctx context.Context) {
	defer close(m.done)
	completed := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.kick:
		}
		t := time.NewTimer(debounce)
	debouncing:
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-m.kick:
				t.Reset(debounce)
			case <-t.C:
				break debouncing
			}
		}
		runCtx, cancel := context.WithCancel(ctx)
		type outcome struct {
			res   *analysis.Result
			snaps []*psi.Snapshot
			err   error
		}
		out := make(chan outcome, 1)
		go func() {
			res, snaps, err := m.update(runCtx)
			out <- outcome{res, snaps, err}
		}()
		again := false
	running:
		for {
			select {
			case o := <-out:
				switch {
				case o.err == nil:
					completed = true
					m.show(o.res, o.snaps)
				case runCtx.Err() == nil:
					m.fail(o.err)
				}
				break running
			case <-m.kick:
				again = true
				if completed {
					cancel()
				}
			case <-ctx.Done():
				cancel()
				<-out
				return
			}
		}
		cancel()
		m.mu.Lock()
		if again {
			// Go round again: the kick was taken.
			select {
			case m.kick <- struct{}{}:
			default:
			}
		} else {
			select {
			case <-m.kick:
				m.kick <- struct{}{} // kicked since: still dirty
			default:
				m.dirty = false
				m.w.s.work.done()
			}
		}
		m.mu.Unlock()
	}
}

// update analyzes m's sources: the open documents' snapshots and the
// other files' on disk.
func (m *module) update(ctx context.Context) (*analysis.Result, []*psi.Snapshot, error) {
	paths, err := m.listSources()
	if err != nil {
		return nil, nil, err
	}
	s := m.w.s
	open := s.openSnapshots()
	snaps := make([]*psi.Snapshot, 0, len(paths))
	for _, p := range paths {
		if snap := open[p]; snap != nil {
			snaps = append(snaps, snap)
		} else if snap, err := s.disk.snapshot(p); err == nil {
			snaps = append(snaps, snap)
		}
	}
	res, err := m.sess.Update(ctx, snaps)
	return res, snaps, err
}

// listSources returns m's source files, listing them if they aren't.
func (m *module) listSources() ([]string, error) {
	m.mu.Lock()
	paths := m.sources
	m.mu.Unlock()
	if paths != nil {
		return paths, nil
	}
	m.w.mu.Lock()
	p := m.w.proj
	m.w.mu.Unlock()
	if p == nil {
		return nil, context.Canceled
	}
	paths, err := p.Sources(m.m)
	if err != nil {
		return nil, err
	}
	if paths == nil {
		paths = []string{}
	}
	m.mu.Lock()
	m.sources = paths
	m.mu.Unlock()
	return paths, nil
}

// fail logs an update's error, once until it changes.
func (m *module) fail(err error) {
	m.mu.Lock()
	msg := err.Error()
	same := msg == m.failed
	m.failed = msg
	m.mu.Unlock()
	if !same {
		m.w.s.logMessage(protocol.MessageError, fmt.Sprintf("kc-lsp: module %s: %v", m.m.Name, err))
	}
}

// show shows the diagnostics of an update of snaps of the files m owns.
func (m *module) show(res *analysis.Result, snaps []*psi.Snapshot) {
	byPath := map[string][]analysis.Diagnostic{}
	for _, d := range res.Diagnostics() {
		byPath[d.Path] = append(byPath[d.Path], d)
	}
	m.w.mu.Lock()
	p := m.w.proj
	m.w.mu.Unlock()
	if p == nil {
		return
	}
	m.mu.Lock()
	m.failed = ""
	m.last = ide.NewView(res, snaps)
	var mine []*psi.Snapshot
	for _, snap := range snaps {
		if p.ModuleOf(snap.Path()) == m.m {
			m.results[snap.Path()] = fileResult{snap, byPath[snap.Path()], m.last}
			mine = append(mine, snap)
		}
	}
	v := m.last
	m.mu.Unlock()
	for _, snap := range mine {
		m.w.s.showSemantic(snap, byPath[snap.Path()], v)
	}
}

// ---- documents ----

// semantic are the semantic diagnostics shown with an open document, and
// the snapshot whose offsets their ranges are.
type semantic struct {
	snap  *psi.Snapshot
	diags []ide.Diagnostic
}

// semanticOf returns an open document's semantic diagnostics: ds as mode
// shows them, and v's hints of path, unless mode is coded.
func semanticOf(ds []analysis.Diagnostic, mode ide.DiagnosticsMode, v *ide.View, path string) []ide.Diagnostic {
	out := ide.SemanticDiagnostics(ds, mode)
	if v != nil && mode != ide.DiagnosticsCoded {
		out = append(out, ide.Hints(v, path)...)
	}
	return out
}

// openSnapshots returns the open documents' snapshots by path.
func (s *Server) openSnapshots() map[string]*psi.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]*psi.Snapshot, len(s.docs))
	for _, d := range s.docs {
		out[d.Path] = d.Snap
	}
	return out
}

// showSemantic shows ds, an update's diagnostics of snap, and for an
// open document the hints of v, the update's view.
func (s *Server) showSemantic(snap *psi.Snapshot, ds []analysis.Diagnostic, v *ide.View) {
	path := snap.Path()
	s.mu.Lock()
	mode := s.mode
	if u, open := s.paths[path]; open {
		d := s.docs[u]
		if d.Snap != snap {
			s.mu.Unlock()
			return // edited since: the next update shows it
		}
		s.mu.Unlock()
		sem := semanticOf(ds, mode, v, path)
		s.mu.Lock()
		if s.docs[u] != d {
			s.mu.Unlock()
			return
		}
		s.sem[u] = semantic{snap, sem}
		enc := s.enc
		s.mu.Unlock()
		s.publish(u, d, enc)
		return
	}
	shown := s.closedShown[path]
	if len(ds) == 0 && !shown {
		s.mu.Unlock()
		return
	}
	if len(ds) == 0 {
		delete(s.closedShown, path)
	} else {
		s.closedShown[path] = true
	}
	enc := s.enc
	s.mu.Unlock()
	d := ide.DocumentOf(snap, 0)
	diags := []protocol.Diagnostic{}
	for _, e := range ide.SemanticDiagnostics(ds, mode) {
		diags = append(diags, protocolDiagnostic(d, e, enc))
	}
	s.notifyDiagnostics(protocol.PublishDiagnosticsParams{URI: fileURI(path), Diagnostics: diags}, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		_, open := s.paths[path]
		return !open // opened since: the document's diagnostics win
	})
}

// clearShown clears the semantic diagnostics shown of files under root.
func (s *Server) clearShown(root string) {
	s.mu.Lock()
	var closed []string
	for p := range s.closedShown {
		if under(root, p) {
			closed = append(closed, p)
			delete(s.closedShown, p)
		}
	}
	var open []*ide.Document
	for u, d := range s.docs {
		if _, ok := s.sem[u]; ok && under(root, d.Path) {
			delete(s.sem, u)
			open = append(open, d)
		}
	}
	enc := s.enc
	s.mu.Unlock()
	for _, p := range closed {
		s.notifyDiagnostics(protocol.PublishDiagnosticsParams{URI: fileURI(p), Diagnostics: []protocol.Diagnostic{}}, nil)
	}
	for _, d := range open {
		s.publish(s.uriOf(d.Path), d, enc)
	}
}

// uriOf returns the URI of an open document's path.
func (s *Server) uriOf(path string) protocol.DocumentURI {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paths[path]
}

// under reports whether path is in the directory root.
func under(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// shiftDiagnostics moves diagnostics of text a to text b: one before the
// change keeps its range, one after it moves with it, and one the change
// touches is dropped.
func shiftDiagnostics(ds []ide.Diagnostic, a, b string) []ide.Diagnostic {
	m := ide.NewMapper(a, b)
	var out []ide.Diagnostic
	for _, d := range ds {
		if r, ok := m.Range(d.Range); ok {
			d.Range = r
			out = append(out, d)
		}
	}
	return out
}

// ---- files on disk ----

// diskCache keeps the snapshots of files on disk, by path, as long as
// their size and time don't change.
type diskCache struct {
	mu    sync.Mutex
	files map[string]diskFile
}

type diskFile struct {
	size int64
	mod  time.Time
	snap *psi.Snapshot
}

func (c *diskCache) snapshot(path string) (*psi.Snapshot, error) {
	fi, err := os.Stat(path)
	if err != nil {
		c.mu.Lock()
		delete(c.files, path)
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Lock()
	f, ok := c.files[path]
	c.mu.Unlock()
	if ok && f.size == fi.Size() && f.mod.Equal(fi.ModTime()) {
		return f.snap, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	snap := psi.Parse(path, string(data))
	c.mu.Lock()
	if c.files == nil {
		c.files = map[string]diskFile{}
	}
	c.files[path] = diskFile{fi.Size(), fi.ModTime(), snap}
	c.mu.Unlock()
	return snap, nil
}

func (c *diskCache) forget(path string) {
	c.mu.Lock()
	delete(c.files, path)
	c.mu.Unlock()
}

// ---- notifications ----

// isProjectFile reports whether a change to path can change a project's
// modules.
func isProjectFile(path string) bool {
	name := filepath.Base(path)
	return name == project.ConfigFile || strings.HasSuffix(name, ".gradle") || strings.HasSuffix(name, ".gradle.kts") ||
		name == "gradle.properties" || strings.HasSuffix(name, ".versions.toml")
}

// reloadFor reloads the workspaces whose project path can change.
func (s *Server) reloadFor(path string) {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	for _, w := range ws {
		if under(w.root, path) {
			s.work.add()
			go func() {
				defer s.work.done()
				w.reload()
			}()
		}
	}
}

func (s *Server) didSave(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DidSaveTextDocumentParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	s.mu.Lock()
	watching := s.watching
	s.mu.Unlock()
	if path := uriPath(p.TextDocument.URI); !watching && isProjectFile(path) {
		s.reloadFor(path)
	}
	return nil, nil
}

func (s *Server) didChangeWatchedFiles(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.DidChangeWatchedFilesParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	reloaded := map[string]bool{}
	for _, c := range p.Changes {
		path := uriPath(c.URI)
		switch {
		case isProjectFile(path):
			if !reloaded[path] {
				reloaded[path] = true
				s.reloadFor(path)
			}
		case strings.HasSuffix(path, ".kt"):
			s.disk.forget(path) // a rewrite may keep its size and time
			s.mu.Lock()
			_, open := s.paths[path]
			shown := s.closedShown[path]
			if c.Type == protocol.FileDeleted {
				delete(s.closedShown, path)
			}
			s.mu.Unlock()
			if c.Type == protocol.FileDeleted && shown && !open {
				s.notifyDiagnostics(protocol.PublishDiagnosticsParams{URI: fileURI(path), Diagnostics: []protocol.Diagnostic{}}, nil)
			}
			if !open || c.Type != protocol.FileChanged {
				s.kickPath(path, c.Type != protocol.FileChanged)
			}
		}
	}
	return nil, nil
}

// watch asks the client to tell the server of changes to sources and
// build files.
func (s *Server) watch() {
	globs := []string{"**/*.kt", "**/" + project.ConfigFile, "**/*.gradle", "**/*.gradle.kts", "**/gradle.properties", "**/*.versions.toml"}
	var ws []protocol.FileSystemWatcher
	for _, g := range globs {
		ws = append(ws, protocol.FileSystemWatcher{GlobPattern: g})
	}
	params := protocol.RegistrationParams{Registrations: []protocol.Registration{{
		ID: "kc-lsp-watch", Method: "workspace/didChangeWatchedFiles",
		RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{Watchers: ws},
	}}}
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	if err := s.conn.Call(ctx, "client/registerCapability", params, nil); err != nil {
		s.logMessage(protocol.MessageWarning, fmt.Sprintf("kc-lsp: can't watch files: %v", err))
		return
	}
	s.mu.Lock()
	s.watching = true
	s.mu.Unlock()
}

func (s *Server) logMessage(typ int, msg string) {
	_ = s.conn.Notify("window/logMessage", protocol.LogMessageParams{Type: typ, Message: msg})
}

func (s *Server) showMessage(typ int, msg string) {
	_ = s.conn.Notify("window/showMessage", protocol.LogMessageParams{Type: typ, Message: msg})
}

// parseInitOptions reads the initializationOptions.
func (s *Server) parseInitOptions(raw json.RawMessage) {
	var o initOptions
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &o); err != nil {
			s.initWarnings = append(s.initWarnings, fmt.Sprintf("kc-lsp: initializationOptions: %v", err))
		}
	}
	mode, ok := ide.ParseDiagnosticsMode(o.Diagnostics)
	if !ok {
		s.initWarnings = append(s.initWarnings, fmt.Sprintf("kc-lsp: unknown diagnostics setting %q; using default", o.Diagnostics))
	}
	s.mode = mode
	s.noGradle = o.Gradle != nil && !*o.Gradle
}
