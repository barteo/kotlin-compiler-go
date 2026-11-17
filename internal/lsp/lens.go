package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
)

// Running programs: a Run and a Debug lens over each main function, whose
// commands, kc.run and kc.debug, answer how to build and run it, as a
// RunConfig: the client runs that, kc-lsp doesn't. kc/mainClass gives the
// same for a document's first main function.

const (
	runCommand         = "kc.run"
	debugCommand       = "kc.debug"
	clearCachesCommand = "kc.clearCaches"
	restartCommand     = "kc.restart"
)

// debugPort is the port a debugged program's JVM listens on.
const debugPort = 5005

// runArgs are a run lens's command's argument.
type runArgs struct {
	URI       protocol.DocumentURI `json:"uri"`
	MainClass string               `json:"mainClass"`
}

// RunConfig says how to run a main function: build the module's sources
// with kc into OutputDir (Build), then run the class (Run), both in
// WorkingDirectory.
type RunConfig struct {
	MainClass        string   `json:"mainClass"`
	Module           string   `json:"module"`
	WorkingDirectory string   `json:"workingDirectory"`
	OutputDir        string   `json:"outputDir"`
	ClassPath        []string `json:"classPath"`
	JDKHome          string   `json:"jdkHome,omitempty"`
	Build            []string `json:"build"`
	Run              []string `json:"run"`
	// DebugPort is set on a debug run: the JVM waits for a debugger
	// there.
	DebugPort int `json:"debugPort,omitempty"`
}

func (s *Server) codeLens(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.CodeLensParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	out := []protocol.CodeLens{}
	if s.ownerModule(d.Path) == nil {
		return out, nil
	}
	for _, m := range ide.MainFunctions(d.Snap) {
		rg := rng(d, m.Name, enc)
		args := runArgs{URI: p.TextDocument.URI, MainClass: m.Class}
		out = append(out,
			protocol.CodeLens{Range: rg, Command: &protocol.Command{Title: "Run", Command: runCommand, Arguments: []any{args}}},
			protocol.CodeLens{Range: rg, Command: &protocol.Command{Title: "Debug", Command: debugCommand, Arguments: []any{args}}})
	}
	return out, nil
}

// mainClass answers kc/mainClass: the run configuration of a document's
// first main function, or null.
func (s *Server) mainClass(_ context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.CodeLensParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, _, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	mains := ide.MainFunctions(d.Snap)
	if len(mains) == 0 {
		return nil, nil
	}
	c, ok := s.runConfig(d.Path, mains[0].Class, false)
	if !ok {
		return nil, nil
	}
	return c, nil
}

// runCmd answers kc.run and kc.debug.
func (s *Server) runCmd(args []json.RawMessage, debug bool) (any, error) {
	var a runArgs
	if len(args) != 1 || json.Unmarshal(args[0], &a) != nil || a.MainClass == "" {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "want {uri, mainClass}")
	}
	c, ok := s.runConfig(uriPath(a.URI), a.MainClass, debug)
	if !ok {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%s is in no module", a.URI)
	}
	return c, nil
}

// runConfig returns how to run class, of path's module.
func (s *Server) runConfig(path, class string, debug bool) (RunConfig, bool) {
	m := s.ownerModule(path)
	if m == nil {
		return RunConfig{}, false
	}
	pm := m.m
	m.w.mu.Lock()
	root := m.w.root
	roots := slices.Clone(pm.SourceRoots)
	if m.w.proj != nil {
		for _, dep := range pm.DependsOn {
			if dm := m.w.proj.Module(dep); dm != nil {
				roots = append(roots, dm.SourceRoots...)
			}
		}
	}
	m.w.mu.Unlock()
	out := filepath.Join(root, "build", "kc-run", safeName(pm.Name))
	sep := string(os.PathListSeparator)
	build := []string{"kc", "-d", out}
	if len(pm.ClassPath) > 0 {
		build = append(build, "-cp", strings.Join(pm.ClassPath, sep))
	}
	if len(pm.FriendPaths) > 0 {
		build = append(build, "-friend-paths", strings.Join(pm.FriendPaths, sep))
	}
	if pm.JDKHome != "" {
		build = append(build, "-jdk-home", pm.JDKHome)
	}
	if pm.ModuleName != "" {
		build = append(build, "-module-name", pm.ModuleName)
	}
	if pm.JVMDefault != "" {
		build = append(build, "-jvm-default", pm.JVMDefault)
	}
	for _, o := range pm.OptIns {
		build = append(build, "-opt-in", o)
	}
	for _, pl := range pm.Plugins {
		build = append(build, "-"+pl)
	}
	build = append(build, roots...)
	java := "java"
	if pm.JDKHome != "" {
		java = filepath.Join(pm.JDKHome, "bin", "java")
	}
	run := []string{java}
	c := RunConfig{MainClass: class, Module: pm.Name, WorkingDirectory: root, OutputDir: out,
		ClassPath: append([]string{out}, pm.ClassPath...), JDKHome: pm.JDKHome, Build: build}
	if debug {
		c.DebugPort = debugPort
		run = append(run, "-agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address=localhost:5005")
	}
	c.Run = append(run, "-cp", strings.Join(c.ClassPath, sep), class)
	return c, true
}

// safeName returns a module's name as a directory's: :app:main is
// app_main.
func safeName(name string) string {
	b := []byte(name)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.') {
			b[i] = '_'
		}
	}
	if s := strings.Trim(string(b), "_."); s != "" {
		return s
	}
	return "module"
}

// clearCaches deletes the library's extracted sources and stubs and the
// parses of files on disk, then reloads the projects, refreshing their
// build tools' models.
func (s *Server) clearCaches() error {
	dir, err := s.cacheDir()
	if err == nil {
		err = os.RemoveAll(dir)
	}
	s.lib.mu.Lock()
	s.lib.finders, s.lib.snaps = nil, nil
	s.lib.mu.Unlock()
	s.disk.mu.Lock()
	s.disk.files = nil
	s.disk.mu.Unlock()
	s.reloadAll(true)
	return err
}
