package check

import (
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

func TestParseMetroOptions(t *testing.T) {
	for name, tc := range map[string]struct {
		opts map[string]string
		want metroConfig
		err  bool
	}{
		"defaults": {want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT"}},
		"supported": {opts: map[string]string{"contributes-as-inject": "false", "enable-function-providers": "false", "generate-contribution-providers": "false"},
			want: metroConfig{optional: "DEFAULT"}},
		"hilt implies dagger, javax and jakarta": {opts: map[string]string{"interop-include-hilt-annotations": "true"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT",
				javax: true, jakarta: true, dagger: true, hilt: true, daggerRuntime: true}, err: true},
		"javax alone": {opts: map[string]string{"interop-include-javax-annotations": "true"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT", javax: true}},
		"jakarta alone": {opts: map[string]string{"interop-include-jakarta-annotations": "true"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT", jakarta: true}},
		"dagger": {opts: map[string]string{"interop-include-dagger-annotations": "true"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT",
				javax: true, jakarta: true, dagger: true, daggerRuntime: true}},
		"dagger runtime alone": {opts: map[string]string{"enable-dagger-runtime-interop": "true"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT", daggerRuntime: true}},
		"kclass to class": {opts: map[string]string{"enable-kclass-to-class-interop": "true"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT", kclassToClass: true}, err: true},
		"optional bindings": {opts: map[string]string{"optional-binding-behavior": "DISABLED"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DISABLED"}},
		"required optional bindings": {opts: map[string]string{"optional-binding-behavior": "REQUIRE_OPTIONAL_BINDING"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "REQUIRE_OPTIONAL_BINDING"}, err: true},
		"custom": {opts: map[string]string{"custom-inject": "a/I"},
			want: metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT"}, err: true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseMetroOptions(tc.opts)
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
			if (err != nil) != tc.err {
				t.Errorf("got error %v, want error %v", err, tc.err)
			}
		})
	}
}

func TestUsesMetro(t *testing.T) {
	javax := map[string]string{"interop-include-javax-annotations": "true"}
	hilt := map[string]string{"interop-include-hilt-annotations": "true"}
	for name, tc := range map[string]struct {
		src  string
		opts map[string]string
		want bool
	}{
		"import":                 {src: "import dev.zacsweers.metro.Inject\nclass A", want: true},
		"star import":            {src: "import dev.zacsweers.metro.*\nclass A", want: true},
		"class annotation":       {src: "@dev.zacsweers.metro.Inject class A", want: true},
		"constructor annotation": {src: "class A @dev.zacsweers.metro.Inject constructor()", want: true},
		"member annotation":      {src: "interface G { @dev.zacsweers.metro.Provides fun s(): String = \"\" }", want: true},
		"parameter annotation":   {src: "class A(@dev.zacsweers.metro.Assisted val s: String)", want: true},
		"none":                   {src: "class A"},
		"javax, not enabled":     {src: "import javax.inject.Inject\nclass A @Inject constructor()"},
		"javax":                  {src: "import javax.inject.Inject\nclass A @Inject constructor()", opts: javax, want: true},
		"qualified javax":        {src: "class A @javax.inject.Inject constructor()", opts: javax, want: true},
		"dagger, javax only":     {src: "import dagger.Module\n@Module object M", opts: javax},
		"dagger with hilt":       {src: "import dagger.Module\n@Module object M", opts: hilt, want: true},
		"hilt":                   {src: "import dagger.hilt.InstallIn\nclass A", opts: hilt, want: true},
		"jakarta with hilt":      {src: "import jakarta.inject.Inject\nclass A", opts: hilt, want: true},
		"similar package":        {src: "import daggerx.Module\nclass A", opts: hilt},
		"unqualified annotation": {src: "@Inject class A", opts: hilt},
	} {
		t.Run(name, func(t *testing.T) {
			f, errs := syntax.Parse("a.kt", tc.src+"\n")
			if len(errs) > 0 {
				t.Fatal(errs[0])
			}
			if got := UsesMetro(f, tc.opts); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
