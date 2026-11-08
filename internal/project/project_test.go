package project

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// write makes the files of a workspace under root.
func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"libs/kotlin-stdlib-2.1.0.jar": "",
		"libs/dep.jar":                 "",
		ConfigFile: `{"modules": [
			{"name": "main", "sourceRoots": ["src/main/kotlin"], "classPath": ["libs/kotlin-stdlib-2.1.0.jar", "libs/dep.jar", "libs/missing.jar"],
			 "optIns": ["kotlin.RequiresOptIn"], "plugins": ["serialization", "allopen"], "moduleName": "app", "jvmDefault": "enable"},
			{"name": "test", "kind": "test", "sourceRoots": ["src/test/kotlin"], "classPath": ["libs/kotlin-stdlib-2.1.0.jar"], "dependsOn": ["main"]}
		]}`,
		"src/main/kotlin/a/A.kt":          "package a\n",
		"src/main/kotlin/a/B.kt":          "package a\n",
		"src/main/kotlin/a/notes.txt":     "",
		"src/main/kotlin/build/Gen.kt":    "package build\n",
		"src/test/kotlin/a/ATest.kt":      "package a\n",
		"src/test/kotlin/a/nested/Wow.kt": "package a.nested\n",
	})
	p, err := Load(context.Background(), root, Options{NoGradle: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Origin != ConfigFile || len(p.Modules) != 2 {
		t.Fatalf("origin %s, %d modules", p.Origin, len(p.Modules))
	}
	main, test := p.Modules[0], p.Modules[1]
	if main.Kind != "main" || test.Kind != "test" {
		t.Errorf("kinds %s %s", main.Kind, test.Kind)
	}
	// The missing jar and the unknown plugin are warnings.
	if len(p.Warnings) != 2 {
		t.Errorf("warnings %q", p.Warnings)
	}
	cfg := main.Config()
	want := []string{filepath.Join(root, "libs/kotlin-stdlib-2.1.0.jar"), filepath.Join(root, "libs/dep.jar")}
	if !slices.Equal(cfg.ClassPath, want) || cfg.ModuleName != "app" || !slices.Equal(cfg.Plugins, []string{"serialization"}) ||
		cfg.JVMDefault != "enable" || !slices.Equal(cfg.OptIns, []string{"kotlin.RequiresOptIn"}) {
		t.Errorf("config %+v", cfg)
	}
	if got := test.Config().ModuleName; got != "main" {
		t.Errorf("default module name %q", got)
	}
	for path, want := range map[string]*Module{
		"src/main/kotlin/a/A.kt":          main,
		"src/main/kotlin":                 main,
		"src/test/kotlin/a/nested/Wow.kt": test,
		"src/main/kotlinx/A.kt":           nil,
		"build.gradle.kts":                nil,
	} {
		if got := p.ModuleOf(filepath.Join(root, path)); got != want {
			t.Errorf("ModuleOf(%s) = %v, want %v", path, got, want)
		}
	}
	names := func(ms []*Module) []string {
		var out []string
		for _, m := range ms {
			out = append(out, m.Name)
		}
		return out
	}
	for path, want := range map[string][]string{
		"src/main/kotlin/a/A.kt":     {"main", "test"},
		"src/test/kotlin/a/ATest.kt": {"test"},
		"elsewhere/X.kt":             nil,
	} {
		if got := names(p.Dependents(filepath.Join(root, path))); !slices.Equal(got, want) {
			t.Errorf("Dependents(%s) = %v, want %v", path, got, want)
		}
	}
	files, err := p.Sources(test)
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	for _, f := range files {
		r, _ := filepath.Rel(root, f)
		rel = append(rel, filepath.ToSlash(r))
	}
	wantFiles := []string{"src/main/kotlin/a/A.kt", "src/main/kotlin/a/B.kt", "src/test/kotlin/a/ATest.kt", "src/test/kotlin/a/nested/Wow.kt"}
	if !slices.Equal(rel, wantFiles) {
		t.Errorf("sources %q, want %q", rel, wantFiles)
	}
}

func TestLoadErrors(t *testing.T) {
	for _, cfg := range []string{
		`{"modules": [{"name": "a"}, {"name": "a"}]}`,
		`{"modules": [{"name": "a", "dependsOn": ["b"]}]}`,
		`{"modules": [{"name": "a", "kind": "benchmark"}]}`,
		`{"modules": [{}]}`,
		`{"modules": `,
	} {
		root := t.TempDir()
		write(t, root, map[string]string{ConfigFile: cfg})
		if _, err := Load(context.Background(), root, Options{NoGradle: true}); err == nil {
			t.Errorf("%s: no error", cfg)
		}
	}
}

func TestLoadImplicit(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{"a.kt": "", "sub/b.kt": "", "build/gen.kt": "", ".git/x.kt": ""})
	p, err := Load(context.Background(), root, Options{NoGradle: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Origin != "implicit" || len(p.Modules) != 1 {
		t.Fatalf("origin %s, %d modules", p.Origin, len(p.Modules))
	}
	files, err := p.Sources(p.Modules[0])
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join(root, "a.kt"), filepath.Join(root, "sub/b.kt")}; !slices.Equal(files, want) {
		t.Errorf("sources %q, want %q", files, want)
	}
}

func TestIsStdlib(t *testing.T) {
	for path, want := range map[string]bool{
		"/k/lib/kotlin-stdlib.jar":       true,
		"/m2/kotlin-stdlib-2.1.0.jar":    true,
		"kotlin-stdlib-jdk8-1.9.0.jar":   false,
		"kotlin-stdlib-common-1.9.0.jar": false,
		"kotlin-stdlib-.jar":             false,
		"kotlin-stdlib-2.1.0-sources":    false,
		"kotlin-reflect.jar":             false,
	} {
		if got := isStdlib(path); got != want {
			t.Errorf("isStdlib(%s) = %t", path, got)
		}
	}
}

// fakeGradle writes a gradlew to root that writes model as the init
// script would, to the file -PkcLspOut names.
func fakeGradle(t *testing.T, root string, model any) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake gradlew is a shell script")
	}
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, map[string]string{
		"model.json": string(data),
		"gradlew": `#!/bin/sh
for a in "$@"; do
  case "$a" in
    -PkcLspOut=*) out="${a#-PkcLspOut=}" ;;
    --init-script) script=1 ;;
  esac
done
[ -n "$script" ] || { echo "no init script" >&2; exit 1; }
cp "$(dirname "$0")/model.json" "$out"
`,
	})
}

func TestImportGradle(t *testing.T) {
	root := t.TempDir()
	str := func(s string) *string { return &s }
	main := filepath.Join(root, "app/build/classes/kotlin/main")
	jar := filepath.Join(root, "app/build/libs/app.jar")
	write(t, root, map[string]string{
		"settings.gradle.kts":                   `include(":app")`,
		"app/build.gradle.kts":                  `plugins { kotlin("jvm") }`,
		"gradle/libs.versions.toml":             "",
		"lib/kotlin-stdlib-2.2.0.jar":           "",
		"lib/junit.jar":                         "",
		"app/build/classes/kotlin/main/A.class": "",
	})
	stdlib, junit := filepath.Join(root, "lib/kotlin-stdlib-2.2.0.jar"), filepath.Join(root, "lib/junit.jar")
	fakeGradle(t, root, map[string]any{"modules": []rawModule{
		{Name: ":app:main", Kind: "main", SourceRoots: []string{filepath.Join(root, "app/src/main/kotlin")}, ClassPath: []string{stdlib},
			Outputs: []string{main, jar}, ModuleName: str("app"), JVMDefault: str("NO_COMPATIBILITY"), ExplicitAPI: str("Strict"),
			FreeArgs: []string{"-opt-in=a.B,c.D", "-Xcontext-parameters"},
			Plugins:  []string{"/m2/kotlin-scripting-jvm-2.2.0.jar", "/m2/kotlin-stdlib-2.2.0.jar", "/m2/kotlin-serialization-compiler-plugin-embeddable-2.2.0.jar"}},
		{Name: ":app:test", Kind: "test", SourceRoots: []string{filepath.Join(root, "app/src/test/kotlin")}, ClassPath: []string{main, stdlib, junit},
			FriendPaths: []string{main}, Associated: []string{":app:main"}, FreeArgs: []string{"-Xjvm-default=all-compatibility"},
			Plugins: []string{"/m2/kotlin-allopen-compiler-plugin-embeddable-2.2.0.jar"}},
	}})
	gradleRuns = 0
	p, err := Load(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Origin != "gradle" || gradleRuns != 1 {
		t.Fatalf("origin %s, %d runs", p.Origin, gradleRuns)
	}
	m, test := p.Module(":app:main"), p.Module(":app:test")
	if m == nil || test == nil {
		t.Fatalf("modules %v", p.Modules)
	}
	want := &Module{Name: ":app:main", Kind: "main", SourceRoots: []string{filepath.Join(root, "app/src/main/kotlin")}, JavaRoots: []string{},
		ClassPath: []string{stdlib}, FriendPaths: []string{}, ModuleName: "app", JVMDefault: "no-compatibility", ExplicitAPI: "strict",
		OptIns: []string{"a.B", "c.D"}, Plugins: []string{"serialization"}}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("main\n got %+v\nwant %+v", m, want)
	}
	// The test module's class path and friend paths lose the main
	// module's classes, whose sources it sees instead.
	if !slices.Equal(test.ClassPath, []string{stdlib, junit}) || len(test.FriendPaths) != 0 ||
		!slices.Equal(test.DependsOn, []string{":app:main"}) || test.JVMDefault != "enable" {
		t.Errorf("test %+v", test)
	}
	// kc doesn't know allopen: a warning, and the plugin is dropped.
	if len(p.Warnings) != 1 || len(test.Plugins) != 0 {
		t.Errorf("warnings %q, test plugins %q", p.Warnings, test.Plugins)
	}

	// Again: the model kept is current.
	if _, err := Load(context.Background(), root, Options{}); err != nil {
		t.Fatal(err)
	}
	if gradleRuns != 1 {
		t.Errorf("%d runs, want the model kept", gradleRuns)
	}
	// A script changes: Gradle runs again; and with Refresh.
	write(t, root, map[string]string{"gradle/libs.versions.toml": "[versions]\n"})
	if _, err := Load(context.Background(), root, Options{}); err != nil {
		t.Fatal(err)
	}
	if gradleRuns != 2 {
		t.Errorf("%d runs after an edit, want 2", gradleRuns)
	}
	if _, err := Load(context.Background(), root, Options{Gradle: GradleOptions{Refresh: true}}); err != nil {
		t.Fatal(err)
	}
	if gradleRuns != 3 {
		t.Errorf("%d runs with Refresh, want 3", gradleRuns)
	}
	// A file that isn't the build's doesn't matter.
	write(t, root, map[string]string{"app/src/main/kotlin/A.kt": "package a\n"})
	if _, err := Load(context.Background(), root, Options{}); err != nil {
		t.Fatal(err)
	}
	if gradleRuns != 3 {
		t.Errorf("%d runs after a source edit, want 3", gradleRuns)
	}
	// kc-lsp.json wins over Gradle.
	write(t, root, map[string]string{ConfigFile: `{"modules": [{"name": "x", "sourceRoots": ["."]}]}`})
	p, err = Load(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Origin != ConfigFile {
		t.Errorf("origin %s", p.Origin)
	}
}

func TestImportGradleFails(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{"gradlew": "#!/bin/sh\necho 'Could not resolve plugin' >&2\nexit 1\n"})
	if runtime.GOOS == "windows" {
		t.Skip("gradlew is a shell script")
	}
	_, err := Load(context.Background(), root, Options{})
	if err == nil || !strings.Contains(err.Error(), "Could not resolve plugin") {
		t.Errorf("error %v, want Gradle's output", err)
	}
}
