// Package project finds what a workspace's Kotlin modules are, for the
// language server: their sources, class paths and compiler options. A
// workspace describes them in a kc-lsp.json at its root; else, if it is a
// Gradle build, they are imported from Gradle (gradle.go); else the
// workspace is one module of all its sources, with the standard library
// and the JDK.
package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// ConfigFile is the name of the file at a workspace's root that
// describes its modules.
const ConfigFile = "kc-lsp.json"

// A Module is a set of sources compiled together, as a Gradle source set:
// a project's main sources, or its tests.
type Module struct {
	// Name identifies the module in the project: :app:main.
	Name string `json:"name"`
	// Kind is "main" or "test".
	Kind string `json:"kind,omitempty"`
	// SourceRoots are the directories of the module's Kotlin sources,
	// JavaRoots of its Java sources.
	SourceRoots []string `json:"sourceRoots"`
	JavaRoots   []string `json:"javaRoots,omitempty"`
	// ClassPath are the jars and class directories the module compiles
	// against, FriendPaths those whose internal declarations it sees.
	ClassPath   []string `json:"classPath,omitempty"`
	FriendPaths []string `json:"friendPaths,omitempty"`
	// DependsOn are the modules whose sources the module's see, as a
	// test module sees its main module's: they are analyzed with it.
	DependsOn []string `json:"dependsOn,omitempty"`
	// JDKHome is the JDK the module compiles against; "" for the one kc
	// finds.
	JDKHome string `json:"jdkHome,omitempty"`
	// ModuleName is kotlinc's -module-name.
	ModuleName string `json:"moduleName,omitempty"`
	// OptIns, Plugins, JVMDefault and ExplicitAPI are as in
	// analysis.Config.
	OptIns      []string `json:"optIns,omitempty"`
	Plugins     []string `json:"plugins,omitempty"`
	JVMDefault  string   `json:"jvmDefault,omitempty"`
	ExplicitAPI string   `json:"explicitApi,omitempty"`
}

// A Project is a workspace's modules.
type Project struct {
	// Root is the workspace's directory.
	Root string
	// Origin says where the modules come from: ConfigFile, "gradle" or
	// "implicit".
	Origin  string
	Modules []*Module
	// Warnings are what was wrong but didn't stop loading: class path
	// entries that don't exist, unknown plugins.
	Warnings []string
}

// Options change how Load finds a project.
type Options struct {
	// NoGradle stops Load from running Gradle.
	NoGradle bool
	// Gradle are the options of a Gradle import.
	Gradle GradleOptions
}

// Load returns the project of the workspace at root.
func Load(ctx context.Context, root string, opts Options) (*Project, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	p := &Project{Root: root}
	data, err := os.ReadFile(filepath.Join(root, ConfigFile))
	switch {
	case err == nil:
		p.Origin = ConfigFile
		var file struct {
			Modules []*Module `json:"modules"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("%s: %w", ConfigFile, err)
		}
		p.Modules = file.Modules
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	case !opts.NoGradle && isGradle(root):
		p.Origin = "gradle"
		mods, err := ImportGradle(ctx, root, opts.Gradle)
		if err != nil {
			return nil, err
		}
		p.Modules = mods
	default:
		p.Origin = "implicit"
		p.Modules = []*Module{{Name: "main", Kind: "main", SourceRoots: []string{"."}}}
	}
	if err := p.resolve(); err != nil {
		return nil, err
	}
	return p, nil
}

// resolve checks the modules, makes their paths absolute, and adds the
// standard library to the class paths that lack one.
func (p *Project) resolve() error {
	names := map[string]bool{}
	for _, m := range p.Modules {
		if m == nil || m.Name == "" {
			return errors.New("project: a module has no name")
		}
		if names[m.Name] {
			return fmt.Errorf("project: two modules named %s", m.Name)
		}
		names[m.Name] = true
	}
	stdlib, stdlibErr := classpath.FindStdlib()
	for _, m := range p.Modules {
		switch m.Kind {
		case "":
			m.Kind = "main"
		case "main", "test":
		default:
			return fmt.Errorf("project: module %s: kind %q", m.Name, m.Kind)
		}
		for _, d := range m.DependsOn {
			if !names[d] {
				return fmt.Errorf("project: module %s depends on unknown module %s", m.Name, d)
			}
		}
		abs := func(paths []string) []string {
			out := make([]string, 0, len(paths))
			for _, path := range paths {
				if !filepath.IsAbs(path) {
					path = filepath.Join(p.Root, path)
				}
				out = append(out, filepath.Clean(path))
			}
			return out
		}
		m.SourceRoots = abs(m.SourceRoots)
		m.JavaRoots = abs(m.JavaRoots)
		m.FriendPaths = abs(m.FriendPaths)
		if m.JDKHome != "" {
			m.JDKHome = abs([]string{m.JDKHome})[0]
		}
		var cp []string
		for _, e := range abs(m.ClassPath) {
			if _, err := os.Stat(e); err != nil {
				// A build's directory that isn't made yet, as its
				// resources' or another module's classes, is no news.
				if strings.HasSuffix(e, ".jar") {
					p.Warnings = append(p.Warnings, fmt.Sprintf("module %s: class path entry %s doesn't exist", m.Name, e))
				}
				continue
			}
			cp = append(cp, e)
		}
		m.ClassPath = cp
		if !slices.ContainsFunc(m.ClassPath, isStdlib) {
			if stdlibErr != nil {
				p.Warnings = append(p.Warnings, fmt.Sprintf("module %s: no Kotlin standard library: %v", m.Name, stdlibErr))
			} else {
				m.ClassPath = append(m.ClassPath, stdlib)
			}
		}
		for _, pl := range m.Plugins {
			switch pl {
			case "compose", "serialization", "parcelize":
			default:
				p.Warnings = append(p.Warnings, fmt.Sprintf("module %s: kc doesn't support compiler plugin %s", m.Name, pl))
			}
		}
		m.Plugins = slices.DeleteFunc(slices.Clone(m.Plugins), func(pl string) bool {
			return pl != "compose" && pl != "serialization" && pl != "parcelize"
		})
	}
	return nil
}

// isStdlib reports whether a class path entry is the Kotlin standard
// library's jar: kotlin-stdlib.jar or kotlin-stdlib-2.1.0.jar, not
// kotlin-stdlib-jdk8 or kotlin-stdlib-common.
func isStdlib(path string) bool {
	base := filepath.Base(path)
	rest, ok := strings.CutPrefix(base, "kotlin-stdlib")
	if !ok || !strings.HasSuffix(rest, ".jar") {
		return false
	}
	rest = strings.TrimSuffix(rest, ".jar")
	return rest == "" || len(rest) > 1 && rest[0] == '-' && rest[1] >= '0' && rest[1] <= '9'
}

// Module returns the module named name, or nil.
func (p *Project) Module(name string) *Module {
	for _, m := range p.Modules {
		if m.Name == name {
			return m
		}
	}
	return nil
}

// ModuleOf returns the module whose source root holds path, the one with
// the longest root if several do; nil if none does.
func (p *Project) ModuleOf(path string) *Module {
	var best *Module
	n := -1
	for _, m := range p.Modules {
		for _, r := range m.SourceRoots {
			if within(r, path) && len(r) > n {
				best, n = m, len(r)
			}
		}
	}
	return best
}

// within reports whether path is dir or inside it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Roots returns the source roots of m and of the modules it depends on,
// directly or not: the sources analyzed with it.
func (p *Project) Roots(m *Module) []string {
	var roots []string
	seen := map[string]bool{}
	var add func(m *Module)
	add = func(m *Module) {
		if m == nil || seen[m.Name] {
			return
		}
		seen[m.Name] = true
		roots = append(roots, m.SourceRoots...)
		for _, d := range m.DependsOn {
			add(p.Module(d))
		}
	}
	add(m)
	return roots
}

// Sources returns the Kotlin source files of m and of the modules it
// depends on, sorted. A file under the source roots of a module nested
// in m's root isn't m's.
func (p *Project) Sources(m *Module) ([]string, error) {
	var files []string
	seen := map[string]bool{}
	for _, r := range p.Roots(m) {
		owner := p.ownerOfRoot(r)
		err := filepath.WalkDir(r, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				if path != r && skipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".kt") || seen[path] {
				return nil
			}
			if p.ModuleOf(path) != owner {
				return nil // a nested module's
			}
			seen[path] = true
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(files)
	return files, nil
}

// ownerOfRoot returns the module whose source root r is.
func (p *Project) ownerOfRoot(r string) *Module {
	for _, m := range p.Modules {
		if slices.Contains(m.SourceRoots, r) {
			return m
		}
	}
	return nil
}

// skipDir reports whether a directory under a source root holds no
// sources: the build's, the VCS's and the tools'.
func skipDir(name string) bool {
	switch name {
	case "build", "out", "node_modules", ".git", ".gradle", ".idea", ".kc-lsp":
		return true
	}
	return false
}

// Config returns the analysis configuration of m.
func (m *Module) Config() analysis.Config {
	name := m.ModuleName
	if name == "" {
		name = "main"
	}
	return analysis.Config{
		JDKHome:         m.JDKHome,
		ModuleName:      name,
		ClassPath:       slices.Clone(m.ClassPath),
		FriendPaths:     slices.Clone(m.FriendPaths),
		OptIns:          slices.Clone(m.OptIns),
		Plugins:         slices.Clone(m.Plugins),
		JVMDefault:      m.JVMDefault,
		ExplicitAPIMode: m.ExplicitAPI,
	}
}
