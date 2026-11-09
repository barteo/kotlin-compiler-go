package project

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// The Maven import. The modules, their sources and the Kotlin plugin's
// options are read from the poms; the class paths are Maven's, from the
// dependency plugin's build-classpath goal, run once for the main
// sources' scope and once for the tests'. As Gradle's, the model is kept
// in CacheDir, with a hash of the poms.

// MavenOptions change how ImportMaven runs Maven.
type MavenOptions struct {
	// Mvn is the command to run; "" for the build's wrapper, mvnw, or
	// mvn if it has none.
	Mvn string
	// Refresh runs Maven even if the model kept is current.
	Refresh bool
	// DownloadSources has Maven download the dependencies' source jars,
	// for navigation into them.
	DownloadSources bool
	// Output receives Maven's output; nil discards it.
	Output io.Writer
}

// mavenRuns counts the Maven runs, for tests.
var mavenRuns int

// isMaven reports whether root is a Maven build.
func isMaven(root string) bool {
	_, err := os.Stat(filepath.Join(root, "pom.xml"))
	return err == nil
}

// pom is what the import reads of a pom.xml.
type pom struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Packaging  string `xml:"packaging"`
	Parent     struct {
		GroupID string `xml:"groupId"`
		Version string `xml:"version"`
	} `xml:"parent"`
	Modules    []string   `xml:"modules>module"`
	Properties properties `xml:"properties"`
	Build      struct {
		SourceDirectory     string        `xml:"sourceDirectory"`
		TestSourceDirectory string        `xml:"testSourceDirectory"`
		Plugins             []mavenPlugin `xml:"plugins>plugin"`
		Management          []mavenPlugin `xml:"pluginManagement>plugins>plugin"`
	} `xml:"build"`
}

type mavenPlugin struct {
	ArtifactID    string      `xml:"artifactId"`
	Configuration kotlinConf  `xml:"configuration"`
	Executions    []execution `xml:"executions>execution"`
}

type execution struct {
	Goals         []string   `xml:"goals>goal"`
	Configuration kotlinConf `xml:"configuration"`
}

// kotlinConf is the kotlin-maven-plugin's configuration.
type kotlinConf struct {
	SourceDirs      []string `xml:"sourceDirs>sourceDir"`
	ModuleName      string   `xml:"moduleName"`
	CompilerPlugins []string `xml:"compilerPlugins>plugin"`
	Args            []string `xml:"args>arg"`
}

// properties are a pom's <properties>, by name.
type properties map[string]string

func (p *properties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*p = properties{}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var v string
			if err := d.DecodeElement(&v, &t); err != nil {
				return err
			}
			(*p)[t.Name.Local] = strings.TrimSpace(v)
		case xml.EndElement:
			return nil
		}
	}
}

// mavenProject is a module of the build, its pom read with what it
// inherits from its aggregator's.
type mavenProject struct {
	dir        string
	pom        *pom
	props      properties
	kotlin     *mavenPlugin // the kotlin-maven-plugin, or nil
	groupID    string
	version    string
	management map[string]*mavenPlugin
}

// readPoms reads the pom of dir and of its modules', depth first.
func readPoms(dir string, parent *mavenProject, out *[]*mavenProject, seen map[string]bool) error {
	if seen[dir] {
		return nil
	}
	seen[dir] = true
	data, err := os.ReadFile(filepath.Join(dir, "pom.xml"))
	if err != nil {
		return err
	}
	var p pom
	if err := xml.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("project: %s: %w", filepath.Join(dir, "pom.xml"), err)
	}
	mp := &mavenProject{dir: dir, pom: &p, props: properties{}, management: map[string]*mavenPlugin{}}
	if parent != nil {
		for k, v := range parent.props {
			mp.props[k] = v
		}
		for k, v := range parent.management {
			mp.management[k] = v
		}
		mp.kotlin = parent.kotlin // build plugins are inherited
		mp.groupID, mp.version = parent.groupID, parent.version
	}
	for k, v := range p.Properties {
		mp.props[k] = v
	}
	mp.groupID = first(p.GroupID, p.Parent.GroupID, mp.groupID)
	mp.version = first(p.Version, p.Parent.Version, mp.version)
	for i := range p.Build.Management {
		pl := &p.Build.Management[i]
		mp.management[pl.ArtifactID] = pl
	}
	for i := range p.Build.Plugins {
		if pl := &p.Build.Plugins[i]; pl.ArtifactID == "kotlin-maven-plugin" {
			mp.kotlin = merge(pl, mp.management[pl.ArtifactID])
		}
	}
	*out = append(*out, mp)
	for _, m := range p.Modules {
		sub := filepath.Join(dir, filepath.FromSlash(mp.interpolate(m)))
		if strings.HasSuffix(sub, ".xml") {
			sub = filepath.Dir(sub)
		}
		if err := readPoms(sub, mp, out, seen); err != nil {
			return err
		}
	}
	return nil
}

// merge returns pl with what it doesn't configure taken from its
// management's declaration, def.
func merge(pl, def *mavenPlugin) *mavenPlugin {
	if def == nil {
		return pl
	}
	m := *pl
	c, d := &m.Configuration, def.Configuration
	if len(c.SourceDirs) == 0 {
		c.SourceDirs = d.SourceDirs
	}
	if c.ModuleName == "" {
		c.ModuleName = d.ModuleName
	}
	if len(c.CompilerPlugins) == 0 {
		c.CompilerPlugins = d.CompilerPlugins
	}
	if len(c.Args) == 0 {
		c.Args = d.Args
	}
	if len(m.Executions) == 0 {
		m.Executions = def.Executions
	}
	return &m
}

func first(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

// interpolate replaces the ${...} of s that the import knows: the
// project's base directory, coordinates, and properties.
func (mp *mavenProject) interpolate(s string) string {
	for range 10 { // properties of properties, not forever
		i := strings.Index(s, "${")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], "}")
		if j < 0 {
			return s
		}
		name := s[i+2 : i+j]
		var v string
		switch name {
		case "basedir", "project.basedir":
			v = mp.dir
		case "project.artifactId":
			v = mp.pom.ArtifactID
		case "project.groupId":
			v = mp.groupID
		case "project.version":
			v = mp.version
		case "project.build.directory":
			v = filepath.Join(mp.dir, "target")
		default:
			var ok bool
			if v, ok = mp.props[name]; !ok {
				return s
			}
		}
		s = s[:i] + v + s[i+j+1:]
	}
	return s
}

// path returns a pom's path, made absolute against the project's
// directory.
func (mp *mavenProject) path(s string) string {
	s = filepath.FromSlash(mp.interpolate(strings.TrimSpace(s)))
	if !filepath.IsAbs(s) {
		s = filepath.Join(mp.dir, s)
	}
	return filepath.Clean(s)
}

// roots returns the Kotlin and Java source roots of the project's main
// sources, or its tests'.
func (mp *mavenProject) roots(test bool) (kotlin, java []string) {
	scope, goal, dir := "main", "compile", mp.pom.Build.SourceDirectory
	if test {
		scope, goal, dir = "test", "test-compile", mp.pom.Build.TestSourceDirectory
	}
	if dir == "" {
		dir = "src/" + scope + "/java"
	}
	java = []string{mp.path(dir)}
	if mp.kotlin != nil {
		for _, e := range mp.kotlin.Executions {
			if slices.Contains(e.Goals, goal) {
				for _, d := range e.Configuration.SourceDirs {
					kotlin = append(kotlin, mp.path(d))
				}
			}
		}
		if len(kotlin) == 0 && !test {
			for _, d := range mp.kotlin.Configuration.SourceDirs {
				kotlin = append(kotlin, mp.path(d))
			}
		}
	}
	if len(kotlin) == 0 {
		kotlin = []string{mp.path("src/" + scope + "/kotlin"), java[0]}
	}
	return slices.Compact(kotlin), java
}

// ImportMaven returns the modules of the Maven build at root: a main and
// a test module of each project that isn't an aggregator.
func ImportMaven(ctx context.Context, root string, opts MavenOptions) ([]*Module, error) {
	var projects []*mavenProject
	if err := readPoms(root, nil, &projects, map[string]bool{}); err != nil {
		return nil, err
	}
	key, err := mavenKey(projects)
	if err != nil {
		return nil, err
	}
	cacheFile := filepath.Join(root, CacheDir, "maven.json")
	if !opts.Refresh {
		if data, err := os.ReadFile(cacheFile); err == nil {
			var c cached
			if json.Unmarshal(data, &c) == nil && c.Key == key {
				return c.Modules, nil
			}
		}
	}
	paths := map[string]map[string][]string{} // scope -> project dir -> class path
	for _, scope := range []string{"compile", "test"} {
		file := "target/kc-lsp-" + scope + ".classpath"
		if err := runMaven(ctx, root, opts, "-Dmdep.outputFile="+file, "-Dmdep.includeScope="+scope, "dependency:build-classpath"); err != nil {
			return nil, err
		}
		paths[scope] = map[string][]string{}
		for _, mp := range projects {
			f := filepath.Join(mp.dir, filepath.FromSlash(file))
			data, err := os.ReadFile(f)
			if err != nil {
				continue // an aggregator's, or a module Maven skipped
			}
			os.Remove(f)
			for _, e := range strings.Split(strings.TrimSpace(string(data)), string(os.PathListSeparator)) {
				if e != "" {
					paths[scope][mp.dir] = append(paths[scope][mp.dir], e)
				}
			}
		}
	}
	if opts.DownloadSources {
		// Into the local repository, next to the jars, where the
		// library's sources are looked for; a failure loses nothing else.
		runMaven(ctx, root, opts, "-DfailOnMissingClassifierArtifact=false", "dependency:sources")
	}
	mods := mavenModules(projects, paths)
	if data, err := json.MarshalIndent(cached{Key: key, Modules: mods}, "", "  "); err == nil {
		if os.MkdirAll(filepath.Dir(cacheFile), 0o755) == nil {
			os.WriteFile(cacheFile, data, 0o644)
		}
	}
	return mods, nil
}

// mavenModules makes the modules of the projects, with the class paths
// Maven gave, by scope and project. A class path entry that is another
// project's, its classes or its jar in the build or the local repository,
// makes the module depend on that project's main module instead.
func mavenModules(projects []*mavenProject, paths map[string]map[string][]string) []*Module {
	name := func(mp *mavenProject, kind string) string { return mp.pom.ArtifactID + ":" + kind }
	owner := func(e string) *mavenProject {
		for _, mp := range projects {
			if mp.pom.Packaging == "pom" {
				continue
			}
			if within(filepath.Join(mp.dir, "target"), e) {
				return mp
			}
			art := mp.pom.ArtifactID + "-" + mp.version
			if mp.version != "" && strings.HasPrefix(filepath.Base(e), art) &&
				filepath.Base(filepath.Dir(e)) == mp.version && filepath.Base(filepath.Dir(filepath.Dir(e))) == mp.pom.ArtifactID {
				return mp
			}
		}
		return nil
	}
	var mods []*Module
	for _, mp := range projects {
		if mp.pom.Packaging == "pom" || mp.pom.ArtifactID == "" {
			continue
		}
		for _, kind := range []string{"main", "test"} {
			kotlin, java := mp.roots(kind == "test")
			m := &Module{Name: name(mp, kind), Kind: kind, SourceRoots: kotlin, JavaRoots: java, ModuleName: mp.pom.ArtifactID}
			scope := "compile"
			if kind == "test" {
				scope = "test"
				m.DependsOn = []string{name(mp, "main")}
			}
			for _, e := range paths[scope][mp.dir] {
				if o := owner(e); o != nil {
					if dep := name(o, "main"); o != mp && !slices.Contains(m.DependsOn, dep) {
						m.DependsOn = append(m.DependsOn, dep)
					}
					continue
				}
				m.ClassPath = append(m.ClassPath, e)
			}
			if k := mp.kotlin; k != nil {
				c := k.Configuration
				if c.ModuleName != "" {
					m.ModuleName = mp.interpolate(c.ModuleName)
				}
				for _, pl := range c.CompilerPlugins {
					if pl := mavenCompilerPlugin(pl); !slices.Contains(m.Plugins, pl) {
						m.Plugins = append(m.Plugins, pl)
					}
				}
				var args []string
				for _, a := range c.Args {
					args = append(args, mp.interpolate(strings.TrimSpace(a)))
				}
				applyFreeArgs(m, args)
			}
			mods = append(mods, m)
		}
	}
	return mods
}

// mavenCompilerPlugin returns the plugin kc names a kotlin-maven-plugin
// compiler plugin by: serialization for kotlinx-serialization.
func mavenCompilerPlugin(name string) string {
	switch name {
	case "kotlinx-serialization":
		return "serialization"
	}
	return name
}

// runMaven runs Maven in the build at root with args.
func runMaven(ctx context.Context, root string, opts MavenOptions, args ...string) error {
	mvn := opts.Mvn
	if mvn == "" {
		mvn = filepath.Join(root, "mvnw")
		if _, err := os.Stat(mvn); err != nil {
			mvn = "mvn"
		}
	}
	cmd := exec.CommandContext(ctx, mvn, append([]string{"--batch-mode", "--quiet"}, args...)...)
	cmd.Dir = root
	var log bytes.Buffer
	w := io.Writer(&log)
	if opts.Output != nil {
		w = io.MultiWriter(&log, opts.Output)
	}
	cmd.Stdout, cmd.Stderr = w, w
	mavenRuns++
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(log.String())
		if len(msg) > 2000 {
			msg = "..." + msg[len(msg)-2000:]
		}
		return fmt.Errorf("project: maven import failed: %v\n%s", err, msg)
	}
	return nil
}

// mavenKey returns a hash of the build's poms, by path and content.
func mavenKey(projects []*mavenProject) (string, error) {
	h := fnv.New64a()
	for _, mp := range projects {
		data, err := os.ReadFile(filepath.Join(mp.dir, "pom.xml"))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", mp.dir, len(data))
		h.Write(data)
	}
	return fmt.Sprintf("%016x", h.Sum64()), nil
}
