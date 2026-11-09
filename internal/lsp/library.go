package lsp

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/libsrc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Navigation into the library's sources. A location in the library is a
// file extracted from its source archive (internal/libsrc) to a cache
// directory, read-only, so that every client opens it as a file; a
// document of it has the syntax features, as a file outside the modules
// does.

// libraries is the server's state of the library's sources.
type libraries struct {
	off bool   // initializationOptions.librarySources false
	dir string // initializationOptions.libraryCache, or ""

	mu      sync.Mutex
	finders map[string]*libsrc.Finder // by class path and JDK
	snaps   map[string]*psi.Snapshot  // the extracted files' parses, by path
}

// maxLibrarySnaps is how many parses of extracted files the server keeps.
const maxLibrarySnaps = 64

// finder returns the finder of the class path of the module that owns
// path.
func (s *Server) finder(path string) *libsrc.Finder {
	if s.lib.off {
		return nil
	}
	var cp []string
	jdk := ""
	found := false
	for _, m := range s.modulesOf(path) {
		m.w.mu.Lock()
		owner := m.w.proj != nil && m.w.proj.ModuleOf(path) == m.m
		m.w.mu.Unlock()
		if owner || !found {
			cp, jdk, found = m.m.ClassPath, m.m.JDKHome, true
		}
		if owner {
			break
		}
	}
	if !found {
		return nil
	}
	key := strings.Join(cp, "\x00") + "\x01" + jdk
	s.lib.mu.Lock()
	defer s.lib.mu.Unlock()
	if s.lib.finders == nil {
		s.lib.finders = map[string]*libsrc.Finder{}
	}
	f := s.lib.finders[key]
	if f == nil {
		f = libsrc.New(cp, jdk)
		s.lib.finders[key] = f
	}
	return f
}

// libraryLoc locates t in the library of the module of path, and returns
// its location in the file extracted for it.
func (s *Server) libraryLoc(path string, t libsrc.Target) (ide.Loc, libsrc.Location, bool) {
	f := s.finder(path)
	if f == nil {
		return ide.Loc{}, libsrc.Location{}, false
	}
	loc, ok := f.Locate(t)
	if !ok {
		return ide.Loc{}, libsrc.Location{}, false
	}
	file, err := s.extract(loc)
	if err != nil {
		s.logMessage(protocol.MessageWarning, "kc-lsp: library sources: "+err.Error())
		return ide.Loc{}, libsrc.Location{}, false
	}
	return ide.Loc{Snap: s.librarySnap(file, loc.Text), Range: loc.Range}, loc, true
}

// librarySnap returns the parse of an extracted file.
func (s *Server) librarySnap(file, text string) *psi.Snapshot {
	s.lib.mu.Lock()
	defer s.lib.mu.Unlock()
	if snap := s.lib.snaps[file]; snap != nil && snap.Source() == text {
		return snap
	}
	if s.lib.snaps == nil || len(s.lib.snaps) >= maxLibrarySnaps {
		s.lib.snaps = map[string]*psi.Snapshot{}
	}
	snap := psi.Parse(file, text)
	s.lib.snaps[file] = snap
	return snap
}

// extract writes loc's file to the cache directory, if it isn't there
// with its text, and returns its path: <dir>/<archive>-<hash>/<entry>,
// where hash is of the archive's path.
func (s *Server) extract(loc libsrc.Location) (string, error) {
	dir := s.lib.dir
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cache, "kc-lsp", "sources")
	}
	h := sha256.Sum256([]byte(loc.Archive))
	base := strings.TrimSuffix(filepath.Base(loc.Archive), filepath.Ext(loc.Archive))
	file := filepath.Join(dir, base+"-"+hex.EncodeToString(h[:6]), filepath.FromSlash(loc.Entry))
	if old, err := os.ReadFile(file); err == nil && string(old) == loc.Text {
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".extract-*")
	if err != nil {
		return "", err
	}
	_, werr := tmp.WriteString(loc.Text)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), 0o444) // the library's: not to edit
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), file)
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return "", werr
	}
	return file, nil
}

// libraryDefinition returns the location in the library of what the
// caret uses, if it isn't in the sources.
func (s *Server) libraryDefinition(c caret, typ bool) []ide.Loc {
	target := ide.LibraryTarget
	if typ {
		target = ide.LibraryTypeTarget
	}
	t, ok := target(c.views[0], c.path, c.off)
	if !ok {
		return nil
	}
	l, _, ok := s.libraryLoc(c.path, t)
	if !ok {
		return nil
	}
	return []ide.Loc{l}
}

// libraryDoc returns the documentation of the library's declaration the
// caret uses, as Markdown, or "".
func (s *Server) libraryDoc(c caret) string {
	t, ok := ide.LibraryTarget(c.views[0], c.path, c.off)
	if !ok {
		return ""
	}
	_, loc, ok := s.libraryLoc(c.path, t)
	if !ok || !loc.Doc.Valid() {
		return ""
	}
	return ide.KDoc(loc.Doc)
}
