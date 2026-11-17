package project

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
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

// The Gradle import. Running Gradle takes seconds to minutes, so the
// model is kept in .kc-lsp/model.json under the workspace, with a hash of
// the build's scripts and properties: an import with the same hash reads
// it back instead. A build whose model changes without its scripts
// changing (a dependency's version from an environment variable, a
// plugin's convention) needs GradleOptions.Refresh.

//go:embed kc-lsp.init.gradle
var initScript []byte

// CacheDir is the directory under a workspace's root where kc-lsp keeps
// what it learns of the workspace.
const CacheDir = ".kc-lsp"

// GradleOptions change how ImportGradle runs Gradle.
type GradleOptions struct {
	// Gradle is the command to run; "" for the build's wrapper, gradlew,
	// or gradle if it has none.
	Gradle string
	// Refresh runs Gradle even if the model kept is current.
	Refresh bool
	// DownloadSources has Gradle download the dependencies' source jars,
	// into its cache, where the library's sources are looked for.
	DownloadSources bool
	// Output receives Gradle's output; nil discards it.
	Output io.Writer
}

// gradleRuns counts the Gradle runs, for tests.
var gradleRuns int

// isGradle reports whether root is a Gradle build.
func isGradle(root string) bool {
	for _, f := range []string{"gradlew", "settings.gradle.kts", "settings.gradle", "build.gradle.kts", "build.gradle"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			return true
		}
	}
	return false
}

// cached is the model kept in CacheDir.
type cached struct {
	Key     string    `json:"key"`
	Modules []*Module `json:"modules"`
}

// ImportGradle returns the modules of the Gradle build at root: one for
// each JVM compilation of each project with the Kotlin plugin.
func ImportGradle(ctx context.Context, root string, opts GradleOptions) ([]*Module, error) {
	key, err := buildKey(root)
	if err != nil {
		return nil, err
	}
	cacheFile := filepath.Join(root, CacheDir, "model.json")
	if !opts.Refresh {
		if data, err := os.ReadFile(cacheFile); err == nil {
			var c cached
			if json.Unmarshal(data, &c) == nil && c.Key == key {
				return c.Modules, nil
			}
		}
	}
	raws, err := runGradle(ctx, root, opts)
	if err != nil {
		return nil, err
	}
	mods := convert(raws)
	if data, err := json.MarshalIndent(cached{Key: key, Modules: mods}, "", "  "); err == nil {
		// Keeping it is an optimization: a read-only workspace imports
		// again next time.
		if os.MkdirAll(filepath.Dir(cacheFile), 0o755) == nil {
			os.WriteFile(cacheFile, data, 0o644)
		}
	}
	return mods, nil
}

// rawModule is a compilation as the init script writes it.
type rawModule struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	SourceRoots []string `json:"sourceRoots"`
	JavaRoots   []string `json:"javaRoots"`
	ClassPath   []string `json:"classPath"`
	FriendPaths []string `json:"friendPaths"`
	Associated  []string `json:"associated"`
	Outputs     []string `json:"outputs"`
	ModuleName  *string  `json:"moduleName"`
	OptIns      []string `json:"optIns"`
	JVMDefault  *string  `json:"jvmDefault"`
	FreeArgs    []string `json:"freeArgs"`
	Plugins     []string `json:"plugins"` // the plugin class path's jars' names
	ExplicitAPI *string  `json:"explicitApi"`
	JDKHome     *string  `json:"jdkHome"`
}

// runGradle runs the init script's task in the build at root.
func runGradle(ctx context.Context, root string, opts GradleOptions) ([]rawModule, error) {
	tmp, err := os.MkdirTemp("", "kc-lsp-gradle")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	script := filepath.Join(tmp, "kc-lsp.init.gradle")
	if err := os.WriteFile(script, initScript, 0o644); err != nil {
		return nil, err
	}
	out := filepath.Join(tmp, "model.json")
	gradle := opts.Gradle
	if gradle == "" {
		gradle = filepath.Join(root, "gradlew")
		if _, err := os.Stat(gradle); err != nil {
			gradle = "gradle"
		}
	}
	args := []string{"--quiet", "--no-configuration-cache", "--init-script", script, "-PkcLspOut=" + out}
	if opts.DownloadSources {
		args = append(args, "-PkcLspDownloadSources=true")
	}
	cmd := exec.CommandContext(ctx, gradle, append(args, "kcLspModel")...)
	cmd.Dir = root
	var log bytes.Buffer
	w := io.Writer(&log)
	if opts.Output != nil {
		w = io.MultiWriter(&log, opts.Output)
	}
	cmd.Stdout, cmd.Stderr = w, w
	gradleRuns++
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(log.String())
		if len(msg) > 2000 {
			msg = "..." + msg[len(msg)-2000:]
		}
		return nil, fmt.Errorf("project: gradle import failed: %v\n%s", err, msg)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("project: gradle import wrote no model: %w", err)
	}
	var model struct {
		Modules []rawModule `json:"modules"`
	}
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("project: gradle import: %w", err)
	}
	return model.Modules, nil
}

// convert makes modules of the compilations Gradle reported. A class path
// entry that is another compilation's output, its classes or its jar,
// makes the module depend on that one: its sources are analyzed with the
// module's instead, as they may not be built, or be stale.
func convert(raws []rawModule) []*Module {
	owner := map[string]string{} // output -> module
	for _, r := range raws {
		for _, o := range r.Outputs {
			owner[o] = r.Name
		}
	}
	var mods []*Module
	for _, r := range raws {
		m := &Module{
			Name:        r.Name,
			Kind:        r.Kind,
			SourceRoots: r.SourceRoots,
			JavaRoots:   r.JavaRoots,
			OptIns:      slices.Clone(r.OptIns),
			DependsOn:   slices.Clone(r.Associated),
		}
		for _, jar := range r.Plugins {
			if pl := plugin(jar); pl != "" && !slices.Contains(m.Plugins, pl) {
				m.Plugins = append(m.Plugins, pl)
			}
		}
		if r.ModuleName != nil {
			m.ModuleName = *r.ModuleName
		}
		if r.JDKHome != nil {
			m.JDKHome = *r.JDKHome
		}
		for _, e := range r.ClassPath {
			if o, ok := owner[e]; ok {
				if o != r.Name && !slices.Contains(m.DependsOn, o) {
					m.DependsOn = append(m.DependsOn, o)
				}
				continue
			}
			m.ClassPath = append(m.ClassPath, e)
		}
		for _, e := range r.FriendPaths {
			if _, ok := owner[e]; !ok {
				m.FriendPaths = append(m.FriendPaths, e)
			}
		}
		if r.JVMDefault != nil {
			m.JVMDefault = jvmDefault(*r.JVMDefault)
		}
		if r.ExplicitAPI != nil {
			m.ExplicitAPI = explicitAPI(*r.ExplicitAPI)
		}
		applyFreeArgs(m, r.FreeArgs)
		mods = append(mods, m)
	}
	return mods
}

// applyFreeArgs sets the options of m that compiler arguments give.
func applyFreeArgs(m *Module, args []string) {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "-opt-in="):
			m.OptIns = append(m.OptIns, strings.Split(strings.TrimPrefix(a, "-opt-in="), ",")...)
		case strings.HasPrefix(a, "-jvm-default="):
			m.JVMDefault = jvmDefault(strings.TrimPrefix(a, "-jvm-default="))
		case strings.HasPrefix(a, "-Xjvm-default="):
			switch strings.TrimPrefix(a, "-Xjvm-default=") {
			case "all":
				m.JVMDefault = "no-compatibility"
			case "all-compatibility":
				m.JVMDefault = "enable"
			case "disable":
				m.JVMDefault = "disable"
			}
		case strings.HasPrefix(a, "-Xexplicit-api="):
			m.ExplicitAPI = explicitAPI(strings.TrimPrefix(a, "-Xexplicit-api="))
		}
	}
}

// plugin returns the compiler plugin a jar of a compilation's plugin
// class path is: compose, serialization, parcelize, or for another
// plugin its name, allopen for kotlin-allopen-compiler-plugin-embeddable;
// "" for a jar that isn't a plugin, as the scripting support and the
// standard library the Kotlin plugin always puts there.
func plugin(jar string) string {
	base := strings.TrimSuffix(filepath.Base(jar), ".jar")
	switch {
	case strings.Contains(base, "compose-compiler"):
		return "compose"
	case strings.Contains(base, "serialization-compiler"), strings.HasPrefix(base, "kotlinx-serialization-compiler"):
		return "serialization"
	case strings.Contains(base, "parcelize-compiler"):
		return "parcelize"
	}
	name, _, ok := strings.Cut(base, "-compiler-plugin")
	if !ok {
		return ""
	}
	return strings.TrimPrefix(name, "kotlin-")
}

// jvmDefault returns the -jvm-default value of the Kotlin plugin's
// JvmDefaultMode, NO_COMPATIBILITY, or of the option, no-compatibility.
func jvmDefault(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "_", "-")
	switch s {
	case "enable", "no-compatibility", "disable":
		return s
	}
	return ""
}

// explicitAPI returns the -Xexplicit-api value of the Kotlin plugin's
// ExplicitApiMode, Strict, or of the option.
func explicitAPI(s string) string {
	switch strings.ToLower(s) {
	case "strict":
		return "strict"
	case "warning":
		return "warning"
	case "disabled", "disable":
		return "disable"
	}
	return ""
}

// buildKey returns a hash of what the Gradle model of the build at root
// depends on: its scripts, properties and version catalogs, by path and
// content.
func buildKey(root string) (string, error) {
	h := fnv.New64a()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if path != root && (skipDir(d.Name()) || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".gradle") || strings.HasSuffix(name, ".gradle.kts") || name == "gradle.properties" ||
			strings.HasSuffix(name, ".versions.toml") || name == "gradle-wrapper.properties" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	slices.Sort(paths)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		rel, _ := filepath.Rel(root, p)
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(data))
		h.Write(data)
	}
	return fmt.Sprintf("%016x", h.Sum64()), nil
}
