// Package lsp serves Kotlin editor requests using kc's Go frontend.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// Each module represents one compilation, including a separate test source set.
type module struct {
	compilerName string
	// Gradle uses distinct compile and outgoing API variants. A nil export
	// list preserves transitive visibility for explicitly resolved projects.
	exportedModules []string
	exportedPaths   []string

	FriendModules   []string `json:"friendModules,omitempty"`
	Name            string   `json:"name"`
	SourceRoots     []string `json:"sourceRoots"`
	JavaRoots       []string `json:"javaRoots,omitempty"`
	GeneratedRoots  []string `json:"generatedRoots,omitempty"`
	Dependencies    []string `json:"dependencies,omitempty"`
	ClassPath       []string `json:"classPath,omitempty"`
	FriendPaths     []string `json:"friendPaths,omitempty"`
	Sources         []string `json:"sources,omitempty"`
	JDKHome         string   `json:"jdkHome,omitempty"`
	LanguageVersion string   `json:"languageVersion,omitempty"`
	ExplicitAPI     string   `json:"explicitAPI,omitempty"`
}
type project struct {
	Version int       `json:"version"`
	Modules []*module `json:"modules"`
}

func absolute(root, s string) string {
	if filepath.IsAbs(s) {
		return filepath.Clean(s)
	}
	return filepath.Join(root, filepath.FromSlash(s))
}
func (m *module) roots() []string { return slices.Concat(m.SourceRoots, m.JavaRoots, m.GeneratedRoots) }
func readProject(ctx context.Context, root string) (project, error) {
	path := filepath.Join(root, "kc-lsp.json")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return discover(ctx, root)
	}
	if err != nil {
		return project{}, err
	}
	defer f.Close()
	var p project
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return p, fmt.Errorf("unexpected trailing content in %s", path)
	}
	if p.Version != 1 || len(p.Modules) == 0 {
		return p, fmt.Errorf("%s requires version 1 and modules", path)
	}
	for _, m := range p.Modules {
		if m == nil {
			return p, fmt.Errorf("null module in %s", path)
		}
		for _, paths := range [][]string{m.SourceRoots, m.JavaRoots, m.GeneratedRoots, m.ClassPath, m.FriendPaths, m.Sources} {
			for i, s := range paths {
				paths[i] = absolute(root, s)
			}
		}
		if m.JDKHome != "" {
			m.JDKHome = absolute(root, m.JDKHome)
		}
		if len(m.roots()) == 0 {
			return p, fmt.Errorf("module %s has no source roots", m.Name)
		}
		if m.LanguageVersion != "" && m.LanguageVersion != "2.4" {
			return p, fmt.Errorf("module %s requests unsupported Kotlin version %s", m.Name, m.LanguageVersion)
		}
		for _, s := range m.ClassPath {
			if _, err := os.Stat(s); err != nil {
				return p, fmt.Errorf("module %s classpath: %w", m.Name, err)
			}
		}
	}
	return p, p.validate()
}
func (p project) validate() error {
	mods := map[string]*module{}
	for _, m := range p.Modules {
		if m.Name == "" || mods[m.Name] != nil {
			return fmt.Errorf("empty or duplicate module name %q", m.Name)
		}
		mods[m.Name] = m
	}
	visited := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		if visited[name] == 1 {
			return fmt.Errorf("module dependency cycle through %s", name)
		}
		if visited[name] == 2 {
			return nil
		}
		m := mods[name]
		if m == nil {
			return fmt.Errorf("unknown module %q", name)
		}
		visited[name] = 1
		for _, d := range m.Dependencies {
			if err := visit(d); err != nil {
				return err
			}
		}
		visited[name] = 2
		return nil
	}
	for _, m := range p.Modules {
		if err := visit(m.Name); err != nil {
			return err
		}
	}
	return nil
}
func (p project) group(m *module) []*module {
	var list []*module
	seen := map[*module]bool{}
	var add func(*module)
	add = func(x *module) {
		if seen[x] {
			return
		}
		seen[x] = true
		list = append(list, x)
		for _, d := range x.Dependencies {
			for _, dep := range p.Modules {
				if dep.Name == d {
					add(dep)
					break
				}
			}
		}
	}
	add(m)
	return list
}

// visibleModules includes direct dependencies and their exported APIs.
func (p project) visibleModules(m *module) map[*module]bool {
	byName := map[string]*module{}
	for _, dep := range p.Modules {
		byName[dep.Name] = dep
	}
	visible := map[*module]bool{m: true}
	var add func([]string)
	add = func(names []string) {
		for _, name := range names {
			d := byName[name]
			if d == nil || visible[d] {
				continue
			}
			visible[d] = true
			exported := d.exportedModules
			if exported == nil {
				exported = d.Dependencies
			}
			add(exported)
		}
	}
	add(m.Dependencies)
	return visible
}
func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func (p project) owner(path string) *module {
	var m *module
	longest := -1
	for _, candidate := range p.Modules {
		for _, root := range candidate.roots() {
			if len(root) > longest && within(path, root) {
				m = candidate
				longest = len(root)
			}
		}
	}
	return m
}
func filesUnder(root string, visit func(string) error) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains([]string{"build", "target", "out", "node_modules"}, d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".java") {
			return visit(path)
		}
		return nil
	})
}
func (p project) config(m *module) analysis.Config {
	cfg := analysis.Config{ModuleName: m.Name, JDKHome: m.JDKHome, FriendPaths: m.FriendPaths, ExplicitAPIMode: m.ExplicitAPI, Jobs: 1}
	if cfg.JDKHome == "" {
		cfg.JDKHome, _ = classpath.FindJDK()
		cfg.NoJDK = cfg.JDKHome == ""
	}
	visible := p.visibleModules(m)
	for _, dep := range p.group(m) {
		if !visible[dep] {
			continue
		}
		paths := dep.ClassPath
		if dep != m && dep.exportedPaths != nil {
			paths = dep.exportedPaths
		}
		for _, path := range paths {
			if !slices.Contains(cfg.ClassPath, path) {
				cfg.ClassPath = append(cfg.ClassPath, path)
			}
		}
	}
	if !slices.ContainsFunc(cfg.ClassPath, func(p string) bool { return strings.HasPrefix(filepath.Base(p), "kotlin-stdlib") }) {
		if std, err := classpath.FindStdlib(); err == nil {
			cfg.ClassPath = append(cfg.ClassPath, std)
		}
	}
	return cfg
}
