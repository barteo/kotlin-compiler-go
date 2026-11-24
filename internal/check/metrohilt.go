package check

import (
	"slices"
	"strings"
)

// Hilt, as Metro's interop-include-hilt-annotations reads it: a class
// annotated @InstallIn(component) and Dagger's @Module or Hilt's
// @EntryPoint contributes to the scope the component maps to, as a
// binding container or a graph supertype, like @ContributesTo(scope);
// so do the modules and entry points of Hilt's compiled @AggregatedDeps
// markers.

const (
	hInstallIn      = "dagger/hilt/InstallIn"
	hEntryPoint     = "dagger/hilt/EntryPoint"
	hDefineComp     = "dagger/hilt/DefineComponent"
	hAggregatedDeps = "dagger/hilt/processor/internal/aggregateddeps/AggregatedDeps"
	hAggregatedPkg  = "hilt_aggregated_deps"
)

// hiltScopes are the scopes of Hilt's built-in components, as Metro's
// HiltBuiltInComponents.
var hiltScopes = map[string]string{
	"dagger/hilt/components/SingletonComponent":                "javax/inject/Singleton",
	"dagger/hilt/android/components/ActivityRetainedComponent": "dagger/hilt/android/scopes/ActivityRetainedScoped",
	"dagger/hilt/android/components/ActivityComponent":         "dagger/hilt/android/scopes/ActivityScoped",
	"dagger/hilt/android/components/ViewModelComponent":        "dagger/hilt/android/scopes/ViewModelScoped",
	"dagger/hilt/android/components/FragmentComponent":         "dagger/hilt/android/scopes/FragmentScoped",
	"dagger/hilt/android/components/ServiceComponent":          "dagger/hilt/android/scopes/ServiceScoped",
	"dagger/hilt/android/components/ViewComponent":             "dagger/hilt/android/scopes/ViewScoped",
	"dagger/hilt/android/components/ViewWithFragmentComponent": "dagger/hilt/android/scopes/ViewScoped",
}

// hiltDep is a compiled @AggregatedDeps marker: the classes it names.
type hiltDep struct {
	components, modules, entryPoints []string
}

// hiltScope returns the scope a Hilt component maps to: a built-in's,
// or a @DefineComponent's scope annotation; "" if none, and then what is
// installed in it contributes nothing, as in Metro.
func (m *metroAnalysis) hiltScope(component string) string {
	if s, ok := hiltScopes[component]; ok {
		return s
	}
	if s, ok := m.hiltComponentScopes[component]; ok {
		return s
	}
	m.hiltComponentScopes[component] = ""
	s := ""
	if cls := m.class(component); cls != nil {
		for _, a := range m.annsOf(cls) {
			if a.cls != hDefineComp && m.meta(a.cls).scope {
				s = a.cls
				break
			}
		}
	}
	m.hiltComponentScopes[component] = s
	return s
}

// hiltContributions returns the @ContributesTo(scope) annotations implied
// for the class name by its annotations as (@InstallIn with @Module or
// @EntryPoint) and, for a class of the class path, by the @AggregatedDeps
// markers naming it. Their orig marks them as Hilt's; they're never
// written as source.
func (m *metroAnalysis) hiltContributions(name string, as []*mAnn, source bool) []*mAnn {
	if !m.cfg.hilt {
		return nil
	}
	var scopes []string
	add := func(components []string) {
		for _, comp := range components {
			if s := m.hiltScope(comp); s != "" && !slices.Contains(scopes, s) {
				scopes = append(scopes, s)
			}
		}
	}
	if in := find(as, hInstallIn); in != nil {
		mod := find(as, mBindingContainer)
		if (mod != nil && mod.orig == "dagger/Module") || find(as, hEntryPoint) != nil {
			add(in.argClasses("value"))
		}
	}
	if !source {
		for _, d := range m.hiltAggregatedDeps() {
			if slices.Contains(d.modules, name) || slices.Contains(d.entryPoints, name) {
				add(d.components)
			}
		}
	}
	var out []*mAnn
	for _, s := range scopes {
		out = append(out, &mAnn{cls: mContributesTo, orig: hInstallIn, args: map[string]mVal{"scope": {tag: 'c', class: s}}})
	}
	return out
}

// hiltAggregatedDeps returns the class path's @AggregatedDeps markers,
// but tests' (@TestInstallIn and @UninstallModules, which Metro
// doesn't support).
func (m *metroAnalysis) hiltAggregatedDeps() []hiltDep {
	if m.hiltDeps != nil {
		return *m.hiltDeps
	}
	deps := []hiltDep{}
	m.hiltDeps = &deps
	cp := m.u.ext.cp
	if !m.cfg.hilt || cp == nil {
		return deps
	}
	strs := func(v mVal) []string {
		var out []string
		for _, e := range v.elems {
			if e.tag == 's' {
				out = append(out, e.str)
			}
		}
		if v.tag == 's' {
			out = append(out, v.str)
		}
		return out
	}
	names := func(v mVal) []string {
		var out []string
		for _, s := range strs(v) {
			if n := m.canonicalClass(s); n != "" {
				out = append(out, n)
			}
		}
		return out
	}
	for _, name := range cp.Package(hAggregatedPkg) {
		cf := m.classfile(name)
		if cf == nil {
			continue
		}
		for _, ca := range cf.Class {
			if descClass(ca.Type) != hAggregatedDeps {
				continue
			}
			a := fromClassfile(ca)
			if slices.ContainsFunc(strs(a.args["test"]), func(t string) bool { return t != "" }) {
				continue
			}
			deps = append(deps, hiltDep{components: names(a.args["components"]), modules: names(a.args["modules"]), entryPoints: names(a.args["entryPoints"])})
		}
	}
	return deps
}

// canonicalClass returns the internal name of the class with a
// canonical name (a.b.Outer.Inner → a/b/Outer$Inner), trying the longest
// package first; "" if there's none.
func (m *metroAnalysis) canonicalClass(s string) string {
	parts := strings.Split(s, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		name := strings.Join(parts[i:], "$")
		if i > 0 {
			name = strings.Join(parts[:i], "/") + "/" + name
		}
		if m.class(name) != nil {
			return name
		}
	}
	return ""
}

// hiltContributorsOf returns the class path's classes the
// @AggregatedDeps markers name for a scope.
func (m *metroAnalysis) hiltContributorsOf(scope string) []string {
	var out []string
	for _, d := range m.hiltAggregatedDeps() {
		match := false
		for _, comp := range d.components {
			if m.hiltScope(comp) == scope {
				match = true
			}
		}
		if !match {
			continue
		}
		for _, n := range append(slices.Clone(d.modules), d.entryPoints...) {
			if !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}
