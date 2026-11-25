package lsp

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
func buildFile(root, base string) string {
	for _, s := range []string{base + ".kts", base} {
		p := filepath.Join(root, s)
		if exists(p) {
			return p
		}
	}
	return ""
}
func discover(ctx context.Context, root string) (project, error) {
	if exists(filepath.Join(root, "pom.xml")) {
		return importMaven(ctx, root)
	}
	if buildFile(root, "build.gradle") != "" || buildFile(root, "settings.gradle") != "" {
		return importGradle(ctx, root)
	}
	return project{Version: 1, Modules: []*module{{Name: "main", SourceRoots: []string{root}}}}, nil
}
func sourceSets(root, name string) (*module, *module) {
	makeSet := func(set string) *module {
		base := filepath.Join(root, "src", set)
		return &module{Name: name + "." + set, SourceRoots: []string{filepath.Join(base, "kotlin")}, JavaRoots: []string{filepath.Join(base, "java")}}
	}
	main, test := makeSet("main"), makeSet("test")
	test.Dependencies = []string{main.Name}
	test.FriendModules = []string{main.Name}
	return main, test
}

type coordinate struct {
	Group    string `xml:"groupId"`
	Artifact string `xml:"artifactId"`
	Version  string `xml:"version"`
}

func (c coordinate) key() string { return c.Group + ":" + c.Artifact }

var coordinatePart = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]*$`)

func (c coordinate) valid() bool {
	for _, s := range []string{c.Group, c.Artifact, c.Version} {
		if !coordinatePart.MatchString(s) || strings.Contains(s, "..") {
			return false
		}
	}
	return true
}

type dependency struct {
	coordinate
	Scope      string       `xml:"scope"`
	Optional   bool         `xml:"optional"`
	Type       string       `xml:"type"`
	Classifier string       `xml:"classifier"`
	Exclusions []coordinate `xml:"exclusions>exclusion"`
}
type props map[string]string

func (p *props) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*p = props{}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch e := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			var v string
			if err := d.DecodeElement(&v, &e); err != nil {
				return err
			}
			(*p)[e.Name.Local] = strings.TrimSpace(v)
		}
	}
}

type pom struct {
	coordinate
	Parent struct {
		coordinate
		Relative *string `xml:"relativePath"`
	} `xml:"parent"`
	Packaging string       `xml:"packaging"`
	Modules   []string     `xml:"modules>module"`
	Props     props        `xml:"properties"`
	Deps      []dependency `xml:"dependencies>dependency"`
	Managed   []dependency `xml:"dependencyManagement>dependencies>dependency"`
	Build     struct {
		Source  string `xml:"sourceDirectory"`
		Test    string `xml:"testSourceDirectory"`
		Plugins []struct {
			Artifact   string `xml:"artifactId"`
			Executions []struct {
				Goals  []string `xml:"goals>goal"`
				Config struct {
					Dirs []string `xml:"sourceDirs>sourceDir"`
				} `xml:"configuration"`
			} `xml:"executions>execution"`
		} `xml:"plugins>plugin"`
	} `xml:"build"`
}
type resolver struct {
	ctx           context.Context
	client        *http.Client
	cache, remote string
	offline       bool
	poms          map[string]*pom
	loading       map[string]bool
	modules       map[string]*module
	repo, gradle  string
}

func newResolver(ctx context.Context) *resolver {
	home, _ := os.UserHomeDir()
	repo := os.Getenv("MAVEN_REPO_LOCAL")
	if repo == "" {
		repo = filepath.Join(home, ".m2", "repository")
	}
	gradle := os.Getenv("GRADLE_USER_HOME")
	if gradle == "" {
		gradle = filepath.Join(home, ".gradle")
	}
	cache, err := os.UserCacheDir()
	if err == nil {
		cache = filepath.Join(cache, "kc-lsp", "maven")
	}
	if dir := os.Getenv("KC_LSP_CACHE_DIR"); dir != "" {
		cache = filepath.Join(dir, "maven")
	}
	return &resolver{ctx: ctx, client: artifactClient, cache: cache, remote: os.Getenv("KC_LSP_MAVEN_REPOSITORY"), offline: os.Getenv("KC_LSP_OFFLINE") == "1", poms: map[string]*pom{}, loading: map[string]bool{}, modules: map[string]*module{}, repo: repo, gradle: gradle}
}
func (r *resolver) artifact(c coordinate, ext string) (string, error) {
	return r.classifiedArtifact(c, ext, "")
}
func (r *resolver) classifiedArtifact(c coordinate, ext, classifier string) (string, error) {
	if !c.valid() {
		return "", fmt.Errorf("unresolved dependency %s:%s", c.key(), c.Version)
	}
	file := c.Artifact + "-" + c.Version + classifier + "." + ext
	p := filepath.Join(r.repo, strings.ReplaceAll(c.Group, ".", "/"), c.Artifact, c.Version, file)
	if exists(p) {
		return p, nil
	}
	base := filepath.Join(r.gradle, "caches", "modules-2", "files-2.1", c.Group, c.Artifact, c.Version)
	dirs, _ := os.ReadDir(base)
	for _, d := range dirs {
		if p := filepath.Join(base, d.Name(), file); exists(p) {
			return p, nil
		}
	}
	return r.download(filepath.ToSlash(filepath.Join(strings.ReplaceAll(c.Group, ".", "/"), c.Artifact, c.Version, file)))
}

var variable = regexp.MustCompile(`\$\{([^}]+)\}`)

func expand(s string, p props) string {
	for range 16 {
		next := variable.ReplaceAllStringFunc(s, func(key string) string {
			if v, ok := p[key[2:len(key)-1]]; ok {
				return v
			}
			return key
		})
		if next == s {
			break
		}
		s = next
	}
	return s
}
func (r *resolver) load(path string) (*pom, error) {
	path = filepath.Clean(path)
	if p := r.poms[path]; p != nil {
		return p, nil
	}
	if r.loading[path] {
		return nil, fmt.Errorf("POM inheritance cycle at %s", path)
	}
	r.loading[path] = true
	defer delete(r.loading, path)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p pom
	if err = xml.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if p.Props == nil {
		p.Props = props{}
	}
	if p.Parent.Artifact != "" {
		rel := "../pom.xml"
		if p.Parent.Relative != nil {
			rel = *p.Parent.Relative
		}
		pp := absolute(filepath.Dir(path), rel)
		if rel == "" || !exists(pp) {
			pp, err = r.artifact(p.Parent.coordinate, "pom")
			if err != nil {
				return nil, err
			}
		}
		parent, err := r.load(pp)
		if err != nil {
			return nil, err
		}
		for k, v := range parent.Props {
			if _, ok := p.Props[k]; !ok {
				p.Props[k] = v
			}
		}
		if p.Group == "" {
			p.Group = parent.Group
		}
		if p.Version == "" {
			p.Version = parent.Version
		}
		p.Managed = slices.Concat(parent.Managed, p.Managed)
		overridden := map[string]bool{}
		for _, d := range p.Deps {
			overridden[d.key()] = true
		}
		var inherited []dependency
		for _, d := range parent.Deps {
			if !overridden[d.key()] {
				inherited = append(inherited, d)
			}
		}
		p.Deps = slices.Concat(p.Deps, inherited)
	}
	p.Props["project.basedir"] = filepath.Dir(path)
	p.Props["basedir"] = filepath.Dir(path)
	for k, v := range map[string]string{"groupId": p.Group, "artifactId": p.Artifact, "version": p.Version} {
		p.Props["project."+k] = v
		p.Props["pom."+k] = v
	}
	managed := map[string]string{}
	var effective []dependency
	for _, d := range p.Managed {
		d.Group = expand(d.Group, p.Props)
		d.Artifact = expand(d.Artifact, p.Props)
		d.Version = expand(d.Version, p.Props)
		if d.Scope == "import" && d.Type == "pom" {
			pp, err := r.artifact(d.coordinate, "pom")
			if err != nil {
				return nil, err
			}
			bom, err := r.load(pp)
			if err != nil {
				return nil, err
			}
			effective = append(effective, bom.Managed...)
		} else {
			effective = append(effective, d)
		}
	}
	p.Managed = effective
	for _, d := range effective {
		managed[d.key()] = d.Version
	}
	for j := range p.Deps {
		d := &p.Deps[j]
		d.Group = expand(d.Group, p.Props)
		d.Artifact = expand(d.Artifact, p.Props)
		d.Version = expand(d.Version, p.Props)
		if d.Version == "" {
			d.Version = managed[d.key()]
		}
	}
	r.poms[path] = &p
	return &p, nil
}
func (r *resolver) resolve(m *module, deps []dependency, managed []dependency) error {
	pinned := map[string]string{}
	for _, dep := range managed {
		pinned[dep.key()] = dep.Version
	}
	type item struct {
		d    dependency
		skip []coordinate
	}
	var q []item
	for _, d := range deps {
		q = append(q, item{d: d})
	}
	seen := map[string]bool{}
	for len(q) > 0 {
		it := q[0]
		q = q[1:]
		d := it.d
		if seen[d.key()] || slices.ContainsFunc(it.skip, func(c coordinate) bool {
			return (c.Group == "*" || c.Group == d.Group) && (c.Artifact == "*" || c.Artifact == d.Artifact)
		}) {
			continue
		}
		seen[d.key()] = true
		if local := r.modules[d.key()+":"+d.Version]; local != nil {
			m.Dependencies = append(m.Dependencies, local.Name)
			continue
		}
		if d.Type != "" && d.Type != "jar" || d.Classifier != "" {
			return fmt.Errorf("unsupported dependency type or classifier for %s", d.key())
		}
		jar, err := r.artifact(d.coordinate, "jar")
		if err != nil {
			return err
		}
		m.ClassPath = append(m.ClassPath, jar)
		// A missing source attachment is optional. Preserve the binary
		// dependency even when its publisher only distributes class files.
		if src, err := r.classifiedArtifact(d.coordinate, "jar", "-sources"); err == nil {
			m.Sources = append(m.Sources, src)
		}
		if err := r.ctx.Err(); err != nil {
			return err
		}
		pp, err := r.artifact(d.coordinate, "pom")
		if err != nil {
			return err
		}
		p, err := r.load(pp)
		if err != nil {
			return err
		}
		for _, child := range p.Deps {
			if !child.Optional && (child.Scope == "" || child.Scope == "compile") {
				if version := pinned[child.key()]; version != "" {
					child.Version = version
				}
				q = append(q, item{child, slices.Concat(it.skip, d.Exclusions)})
			}
		}
	}
	return nil
}
func importMaven(ctx context.Context, root string) (project, error) {
	r := newResolver(ctx)
	if r.remote == "" {
		r.remote = centralRepository
	}
	p := project{Version: 1}
	type entry struct {
		main, test *module
		p          *pom
	}
	var entries []entry
	visited := map[string]bool{}
	var walk func(string) error
	walk = func(dir string) error {
		if visited[dir] {
			return fmt.Errorf("duplicate Maven module %s", dir)
		}
		visited[dir] = true
		model, err := r.load(filepath.Join(dir, "pom.xml"))
		if err != nil {
			return err
		}
		if model.Packaging != "pom" {
			main, test := sourceSets(dir, model.key())
			for _, x := range []struct {
				m    *module
				path string
			}{{main, model.Build.Source}, {test, model.Build.Test}} {
				if x.path != "" {
					x.m.SourceRoots = append(x.m.SourceRoots, absolute(dir, expand(x.path, model.Props)))
				}
			}
			for _, plugin := range model.Build.Plugins {
				if plugin.Artifact == "kotlin-maven-plugin" {
					for _, exec := range plugin.Executions {
						m := main
						if slices.Contains(exec.Goals, "test-compile") {
							m = test
						}
						for _, path := range exec.Config.Dirs {
							m.SourceRoots = append(m.SourceRoots, absolute(dir, expand(path, model.Props)))
						}
					}
				}
			}
			entries = append(entries, entry{main, test, model})
			r.modules[model.key()+":"+expand(model.Version, model.Props)] = main
			p.Modules = append(p.Modules, main, test)
		}
		for _, child := range model.Modules {
			if err := walk(absolute(dir, child)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return p, err
	}
	for _, e := range entries {
		var main, test []dependency
		for _, d := range e.p.Deps {
			switch d.Scope {
			case "test", "runtime":
				test = append(test, d)
			case "", "compile", "provided":
				main = append(main, d)
			default:
				return p, fmt.Errorf("unsupported Maven scope %s", d.Scope)
			}
		}
		if err := r.resolve(e.main, main, e.p.Managed); err != nil {
			return p, err
		}
		if err := r.resolve(e.test, test, e.p.Managed); err != nil {
			return p, err
		}
	}
	return p, p.validate()
}

var quoted = regexp.MustCompile(`["']([^"']+)["']`)
var gradlePlugin = regexp.MustCompile(`(?:id\s*\(?|kotlin\s*\()\s*["']([^"']+)["']`)
var depLine = regexp.MustCompile(`^(api|implementation|compileOnlyApi|compileOnly|testImplementation|testCompileOnly)\b(.*)$`)

type versionCatalog struct {
	libraries map[string]string
	plugins   map[string]string
}

var pluginAlias = regexp.MustCompile(`alias\(\s*(libs\.plugins\.[[:alnum:]_.]+)\s*\)`)
var stdlibDisabled = regexp.MustCompile(`(?m)^\s*kotlin\.stdlib\.default\.dependency\s*=\s*false\s*$`)
var dependencyManagementBlock = regexp.MustCompile(`\bdependencyResolutionManagement\s*\{`)
var pluginVersion = regexp.MustCompile(`^\s*\)?\s*version\s*\(?\s*["']([^"']+)["']`)

func catalog(root string) versionCatalog {
	b, _ := os.ReadFile(filepath.Join(root, "gradle", "libs.versions.toml"))
	section := ""
	versions := map[string]string{}
	cat := versionCatalog{map[string]string{}, map[string]string{}}
	for _, line := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if strings.HasPrefix(s, "[") {
			section = s
			continue
		}
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if section == "[versions]" {
			versions[k] = strings.Trim(v, "\"'")
		}
		if section != "[libraries]" && section != "[plugins]" {
			continue
		}
		name := "libs." + strings.NewReplacer("-", ".", "_", ".").Replace(k)
		if !strings.HasPrefix(v, "{") {
			if section == "[libraries]" {
				cat.libraries[name] = strings.Trim(v, "\"'")
			}
			continue
		}
		fields := map[string]string{}
		for _, pair := range strings.Split(strings.Trim(v, "{} "), ",") {
			a, b, _ := strings.Cut(pair, "=")
			fields[strings.TrimSpace(a)] = strings.Trim(strings.TrimSpace(b), "\"'")
		}
		mod := fields["module"]
		if mod == "" {
			mod = fields["group"] + ":" + fields["name"]
		}
		version := fields["version"]
		if version == "" {
			version = versions[fields["version.ref"]]
		}
		if section == "[plugins]" {
			cat.plugins[strings.Replace(name, "libs.", "libs.plugins.", 1)] = fields["id"] + ":" + version
		} else {
			cat.libraries[name] = mod + ":" + version
		}
	}
	return cat
}
func importGradle(ctx context.Context, root string) (project, error) {
	p := project{Version: 1}
	r := newResolver(ctx)
	cat := catalog(root)
	repository := r.remote
	kotlinVersion := ""
	props, _ := os.ReadFile(filepath.Join(root, "gradle.properties"))
	defaultStdlib := !stdlibDisabled.Match(props)
	names := []string{":"}
	if f := buildFile(root, "settings.gradle"); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return p, err
		}
		// Repositories in dependencyResolutionManagement apply to all projects.
		management := dependencyManagementBlock.FindIndex(b)
		if repository == "" && management != nil && strings.Contains(string(b[management[1]:]), "mavenCentral()") {
			repository = centralRepository
		}
		settings := strings.Split(string(b), "\n")
		for line := 0; line < len(settings); line++ {
			content := func(s string) string {
				return strings.TrimSpace(strings.SplitN(s, "//", 2)[0])
			}
			s := content(settings[line])
			if strings.Contains(s, "projectDir") || strings.HasPrefix(s, "includeBuild") {
				return p, fmt.Errorf("dynamic Gradle project layout in %s", f)
			}
			if !strings.HasPrefix(s, "include(") && !strings.HasPrefix(s, "include ") {
				continue
			}
			args := strings.TrimSpace(strings.TrimPrefix(s, "include"))
			if strings.HasPrefix(args, "(") {
				for !strings.Contains(args, ")") && line+1 < len(settings) {
					line++
					args += " " + content(settings[line])
				}
				if !strings.HasSuffix(args, ")") {
					return p, fmt.Errorf("unsupported project includes in %s", f)
				}
				args = args[1 : len(args)-1]
			}
			for _, arg := range strings.Split(strings.TrimSuffix(strings.TrimSpace(args), ","), ",") {
				arg = strings.TrimSpace(arg)
				if len(arg) < 3 || (arg[0] != '\'' && arg[0] != '"') || arg[0] != arg[len(arg)-1] {
					return p, fmt.Errorf("unresolved Gradle project %q in %s", arg, f)
				}
				name := strings.TrimPrefix(arg[1:len(arg)-1], ":")
				for _, component := range strings.Split(name, ":") {
					if !coordinatePart.MatchString(component) {
						return p, fmt.Errorf("unsupported Gradle project name %q in %s", name, f)
					}
				}
				names = append(names, ":"+name)
			}
		}
	}
	for _, name := range names {
		dir := filepath.Join(root, strings.ReplaceAll(strings.Trim(name, ":"), ":", "/"))
		m, t := sourceSets(dir, name)
		p.Modules = append(p.Modules, m, t)
		file := buildFile(dir, "build.gradle")
		if file == "" {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return p, err
		}
		// Interpret only literal dependency declarations. Unknown expressions in
		// the dependency block require a resolved model from the workspace.
		r.remote = repository
		inDeps := false
		usesKotlin := false
		version := kotlinVersion
		var mainDeps, testDeps, apiDeps []dependency
		m.exportedModules = []string{}
		m.exportedPaths = []string{}
		for _, raw := range strings.Split(string(b), "\n") {
			s := strings.TrimSpace(strings.SplitN(raw, "//", 2)[0])
			if r.remote == "" && strings.Contains(s, "mavenCentral()") {
				r.remote = centralRepository
			}
			if strings.HasPrefix(s, "dependencies") {
				_, tail, ok := strings.Cut(s, "{")
				if !ok {
					return p, fmt.Errorf("unsupported dependencies block in %s", file)
				}
				inDeps = true
				s = strings.TrimSpace(tail)
			}
			if !inDeps {
				// Expand literal version-catalog aliases into the same representation
				// used by a direct Kotlin JVM plugin declaration.
				s = pluginAlias.ReplaceAllStringFunc(s, func(alias string) string {
					key := pluginAlias.FindStringSubmatch(alias)[1]
					id, v, _ := strings.Cut(cat.plugins[key], ":")
					if id == "" {
						return alias
					}
					return `id("` + id + `") version "` + v + `"`
				})
				for _, loc := range gradlePlugin.FindAllStringSubmatchIndex(s, -1) {
					id := s[loc[2]:loc[3]]
					if !slices.Contains([]string{"org.jetbrains.kotlin.jvm", "jvm", "java", "java-library", "application"}, id) {
						return p, fmt.Errorf("Gradle plugin %s needs a resolved project model", id)
					}
					if id == "jvm" || id == "org.jetbrains.kotlin.jvm" {
						tail := s[loc[1]:]
						if v := pluginVersion.FindStringSubmatch(tail); len(v) > 0 {
							version = v[1]
							if name == ":" {
								kotlinVersion = version
							}
						}
						usesKotlin = !strings.Contains(tail, "apply false")
					}
				}
				if strings.Contains(s, "alias(libs.plugins.") || strings.HasPrefix(s, "if ") || strings.HasPrefix(s, "if(") || strings.Contains(s, "dependencies.") || strings.Contains(s, "sourceSets") || strings.HasPrefix(s, "apply(") || strings.HasPrefix(s, "apply from") || strings.HasPrefix(s, "subprojects") || strings.HasPrefix(s, "allprojects") {
					return p, fmt.Errorf("custom Gradle build logic in %s", file)
				}
				continue
			}
			end := strings.HasSuffix(s, "}")
			if end {
				s = strings.TrimSpace(strings.TrimSuffix(s, "}"))
				inDeps = false
			}
			if s == "" {
				continue
			}
			match := depLine.FindStringSubmatch(s)
			if match == nil {
				return p, fmt.Errorf("unresolved Gradle expression %q in %s", s, file)
			}
			target := m
			list := &mainDeps
			if strings.HasPrefix(match[1], "test") {
				target = t
				list = &testDeps
			}
			arg := strings.TrimSpace(match[2])
			if strings.HasPrefix(arg, "(") && strings.HasSuffix(arg, ")") {
				arg = strings.TrimSpace(arg[1 : len(arg)-1])
			}
			if strings.HasPrefix(arg, "project(") {
				paths := quoted.FindStringSubmatch(arg)
				if len(paths) != 2 || strings.TrimSpace(arg) != "project("+paths[0]+")" {
					return p, fmt.Errorf("unresolved project dependency %q", arg)
				}
				target.Dependencies = append(target.Dependencies, paths[1]+".main")
				if match[1] == "api" || match[1] == "compileOnlyApi" {
					m.exportedModules = append(m.exportedModules, paths[1]+".main")
				}
				continue
			}
			gav := strings.Trim(arg, "\"'")
			if arg == `kotlin("stdlib")` && version != "" {
				gav = "org.jetbrains.kotlin:kotlin-stdlib:" + version
			}
			if strings.ContainsAny(gav, "$()") {
				return p, fmt.Errorf("dynamic Gradle dependency %s", arg)
			}
			if strings.HasPrefix(gav, "libs.") {
				gav = cat.libraries[gav]
			}
			parts := strings.Split(gav, ":")
			if len(parts) != 3 {
				return p, fmt.Errorf("unresolved Gradle dependency %q", arg)
			}
			dep := dependency{coordinate: coordinate{parts[0], parts[1], parts[2]}}
			*list = append(*list, dep)
			if match[1] == "api" || match[1] == "compileOnlyApi" {
				apiDeps = append(apiDeps, dep)
			}
		}
		if defaultStdlib && usesKotlin && version != "" && !slices.ContainsFunc(mainDeps, func(d dependency) bool { return d.Group == "org.jetbrains.kotlin" && d.Artifact == "kotlin-stdlib" }) {
			stdlib := dependency{coordinate: coordinate{"org.jetbrains.kotlin", "kotlin-stdlib", version}}
			mainDeps = append(mainDeps, stdlib)
			apiDeps = append(apiDeps, stdlib)
		}
		if err := r.resolve(m, mainDeps, nil); err != nil {
			return p, err
		}
		// The test compilation inherits its own main implementation dependencies.
		t.Dependencies = append(t.Dependencies, m.Dependencies...)
		t.ClassPath = append(t.ClassPath, m.ClassPath...)
		api := &module{}
		if err := r.resolve(api, apiDeps, nil); err != nil {
			return p, err
		}
		m.exportedPaths = append(m.exportedPaths, api.ClassPath...)
		if err := r.resolve(t, testDeps, nil); err != nil {
			return p, err
		}
	}
	return p, p.validate()
}
