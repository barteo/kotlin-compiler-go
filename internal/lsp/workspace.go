package lsp

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type span struct {
	Start position `json:"start"`
	End   position `json:"end"`
}
type location struct {
	URI   string `json:"uri"`
	Range span   `json:"range"`
}
type edit struct {
	Range   span   `json:"range"`
	NewText string `json:"newText"`
}
type object = map[string]any

func filePath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid file URI %q", uri)
	}
	p := u.Path
	if u.Host != "" && u.Host != "localhost" {
		if runtime.GOOS != "windows" {
			return "", fmt.Errorf("unsupported remote file URI")
		}
		p = "//" + u.Host + p
	}
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("file URI needs an absolute path")
	}
	return filepath.Clean(p), nil
}
func uri(path string) string {
	p := filepath.ToSlash(path)
	u := url.URL{Scheme: "file"}
	if runtime.GOOS == "windows" && strings.HasPrefix(p, "//") {
		u.Host, p, _ = strings.Cut(p[2:], "/")
		p = "/" + p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u.Path = p
	return u.String()
}
func pos(s *psi.Snapshot, off int) position {
	l, c, _ := s.Position(off)
	return position{l - 1, c - 1}
}
func rng(n psi.NodeRef) span {
	r := n.Range()
	s := n.ContainingFile()
	return span{pos(s, r.Start), pos(s, r.End)}
}
func loc(n psi.NodeRef) location                      { return location{uri(n.ContainingFile().Path()), rng(n)} }
func offset(s *psi.Snapshot, p position) (int, error) { return s.ByteOffset(p.Line+1, p.Character+1) }

type document struct {
	snap    *psi.Snapshot
	version *int
}
type change struct {
	Range *span  `json:"range"`
	Text  string `json:"text"`
}
type workspace struct {
	attached  map[string][]*psi.Snapshot
	sourceDir string
	roots     []string
	p         project
	docs      map[string]document
	java      map[string]string
	sessions  map[*module]*ka.Session
	checked   map[*module]*index
	owners    map[string]*module
	visible   map[*module]map[*module]bool
	issues    []string
}

func (w *workspace) clearModels() {
	for _, i := range w.checked {
		i.model.Close()
	}
	w.checked = map[*module]*index{}
}
func (w *workspace) close() {
	w.clearModels()
	for _, s := range w.sessions {
		s.Close()
	}
	w.sessions = map[*module]*ka.Session{}
}
func (w *workspace) reload(ctx context.Context) {
	w.close()
	w.attached = nil
	w.p = project{}
	w.issues = nil
	w.owners = map[string]*module{}
	w.visible = map[*module]map[*module]bool{}
	old := w.docs
	w.docs = map[string]document{}
	w.java = map[string]string{}
	for _, root := range w.roots {
		p, err := readProject(ctx, root)
		if err != nil {
			w.issues = append(w.issues, err.Error()+"; provide a resolved kc-lsp.json")
			continue
		}
		// Module names are local to a workspace folder.
		for _, m := range p.Modules {
			m.compilerName = m.Name
			m.Name = root + "#" + m.Name
			for i := range m.exportedModules {
				m.exportedModules[i] = root + "#" + m.exportedModules[i]
			}
			for i := range m.FriendModules {
				m.FriendModules[i] = root + "#" + m.FriendModules[i]
			}
			for i := range m.Dependencies {
				m.Dependencies[i] = root + "#" + m.Dependencies[i]
			}
		}
		w.p.Modules = append(w.p.Modules, p.Modules...)
		for _, m := range p.Modules {
			for _, r := range m.roots() {
				err := filesUnder(r, func(path string) error {
					if d, ok := old[path]; ok && d.version != nil {
						if strings.HasSuffix(path, ".java") {
							w.java[path] = d.snap.Source()
						}
						w.docs[path] = d
						return nil
					}
					if _, ok := w.docs[path]; ok {
						return nil
					}
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.HasSuffix(path, ".java") {
						w.java[path] = string(b)
						return nil
					}
					if d, ok := old[path]; ok && d.snap.Source() == string(b) {
						w.docs[path] = d
					} else {
						w.docs[path] = document{snap: psi.ParseDocument(path, string(b))}
					}
					return nil
				})
				if err != nil {
					w.issues = append(w.issues, err.Error())
				}
			}
		}
	}
	for path, d := range old {
		if d.version != nil {
			if strings.HasSuffix(path, ".java") {
				w.java[path] = d.snap.Source()
			}
			w.docs[path] = d
		}
	}
}
func (w *workspace) set(path string, text string, v int) {
	if strings.HasSuffix(path, ".java") {
		w.java[path] = text
	}
	w.docs[path] = document{psi.ParseDocument(path, text), &v}
	w.invalidate(path)
}
func (w *workspace) invalidate(path string) {
	owner := w.p.owner(path)
	for m, i := range w.checked {
		if m == owner || m == w.owners[path] || slices.Contains(w.p.group(m), owner) {
			i.model.Close()
			delete(w.checked, m)
		}
	}
}
func (w *workspace) change(path string, v int, changes []change) error {
	d, ok := w.docs[path]
	if !ok || d.version == nil {
		return fmt.Errorf("change to closed document")
	}
	if v <= *d.version {
		return nil
	}
	s := d.snap
	for _, c := range changes {
		text := c.Text
		if c.Range != nil {
			a, err := offset(s, c.Range.Start)
			if err != nil {
				return err
			}
			b, err := offset(s, c.Range.End)
			if err != nil {
				return err
			}
			if b < a {
				return fmt.Errorf("reversed change range")
			}
			text = s.Source()[:a] + text + s.Source()[b:]
		}
		s = psi.ParseDocument(path, text)
	}
	if strings.HasSuffix(path, ".java") {
		w.java[path] = s.Source()
	}
	w.docs[path] = document{s, &v}
	w.invalidate(path)
	return nil
}
func (w *workspace) module(path string) *module {
	if m := w.owners[path]; m != nil {
		return m
	}
	m := w.p.owner(path)
	if m == nil {
		m = &module{Name: filepath.Base(path), SourceRoots: []string{path}}
	}
	w.owners[path] = m
	return m
}
func (w *workspace) check(ctx context.Context, path string) (*index, error) {
	m := w.module(path)
	if i := w.checked[m]; i != nil {
		return i, nil
	}
	i, err := w.buildIndex(ctx, path, nil)
	if err == nil {
		w.checked[m] = i
	}
	return i, err
}

func (w *workspace) buildIndex(ctx context.Context, path string, overlay *psi.Snapshot) (*index, error) {
	m := w.module(path)
	group := w.p.group(m)
	includes := func(p string) bool { return p == path || slices.Contains(group, w.p.owner(p)) }
	var snaps []*psi.Snapshot
	for _, p := range slices.Sorted(maps.Keys(w.docs)) {
		if includes(p) && strings.HasSuffix(p, ".kt") {
			snap := w.docs[p].snap
			if overlay != nil && p == path {
				snap = overlay
			}
			snaps = append(snaps, snap)
		}
	}
	var java []ka.Source
	for _, p := range slices.Sorted(maps.Keys(w.java)) {
		if includes(p) {
			text := w.java[p]
			java = append(java, ka.Source{Path: p, Text: &text})
		}
	}
	session := w.sessions[m]
	if session == nil {
		cfg := w.p.config(m)
		cfg.ModuleName = m.compilerName
		if cfg.ModuleName == "" {
			cfg.ModuleName = m.Name
		}
		session = ka.Open(cfg)
		w.sessions[m] = session
	}
	model, err := session.CheckEditor(ctx, snaps, java)
	if err != nil {
		return nil, err
	}
	i := newIndex(model, snaps)
	i.workspace = w
	return i, nil
}
