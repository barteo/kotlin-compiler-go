package check

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The Metro dependency injection compiler plugin (-metro).
//
// Metro reads annotations: @DependencyGraph on an interface asks for an
// implementation of it whose members return what the graph binds, and
// classes say how to build them (@Inject constructors, @Provides and
// @Binds members of binding containers, @Contributes* aggregation into a
// scope). kc compiles a module that uses Metro twice over:
//
//   - PlanMetro declares the program and resolves its signatures, as
//     checking does before it checks bodies, then reads the annotations
//     and resolves each graph's bindings (metroplan.go). It writes, as
//     Kotlin, what the plugin adds: a graph's Impl class and its
//     companion object, a contributing class's nested
//     MetroContributionTo... interface of @Binds functions, and the
//     metro.hints functions that downstream modules find the module's
//     contributions by.
//   - The files, parsed again, are then checked with the plan
//     (Library.MetroPlan): declare adds the members to their classes,
//     and replaces the calls of createGraph and createGraphFactory with
//     the graph's companion (metroInsert).
//
// The code kc writes has the shape of Metro's where other compilations
// meet it: graphs' companions, the hints and the contribution interfaces,
// which graphs compiled by kotlinc with Metro read, as kc reads theirs.
// Inside a graph, bindings are private functions of its Impl class that
// call constructors and providers directly, with scoped bindings held in
// kotlin.Lazy instances.

// MetroPlan is what the plugin adds to a module, computed by PlanMetro.
type MetroPlan struct {
	// members are the Kotlin members to add to classes, by internal name.
	members map[string][]string
	// supers are supertypes to add to classes, by internal name.
	supers map[string][]string
	// calls replace calls of createGraph and createGraphFactory: by file
	// path and position, the Kotlin expression that replaces each.
	calls map[string]map[syntax.Pos]string
	// Files are source files to compile with the module: the hints.
	Files []MetroFile
}

// MetroFile is a source file of the plan.
type MetroFile struct {
	Path, Src string
}

const metroPkg = "dev.zacsweers.metro"

// UsesMetro reports whether a file uses Metro's annotations, or the
// annotations of other frameworks that Metro's options opts make it read
// (javax.inject's, Dagger's, Hilt's): whether it imports declarations of
// their packages or names their annotations qualified. The plugin reads
// nothing in a file that doesn't.
func UsesMetro(f *syntax.File, opts map[string]string) bool {
	cfg, _ := parseMetroOptions(opts)
	pkgs := cfg.packages()
	in := func(name string) bool {
		for _, p := range pkgs {
			if name == p || strings.HasPrefix(name, p+".") {
				return true
			}
		}
		return false
	}
	for _, imp := range f.Imports {
		if in(imp.Path) {
			return true
		}
	}
	found := false
	anns := func(as []syntax.Annotation) {
		for _, a := range as {
			if strings.Contains(a.Name, ".") && in(a.Name) {
				found = true
			}
		}
	}
	params := func(ps []*syntax.Param) {
		for _, p := range ps {
			anns(p.Annotations)
		}
	}
	var visit func(ds []syntax.Decl)
	visit = func(ds []syntax.Decl) {
		for _, d := range ds {
			switch d := d.(type) {
			case *syntax.ClassDecl:
				anns(d.Annotations)
				anns(d.CtorMods.Annotations)
				for _, p := range d.CtorParams {
					anns(p.Modifiers.Annotations)
					anns(p.Param.Annotations)
				}
				visit(d.Members)
			case *syntax.FunDecl:
				anns(d.Annotations)
				params(d.Params)
			case *syntax.PropDecl:
				anns(d.Annotations)
			case *syntax.SecondaryCtor:
				anns(d.Annotations)
				params(d.Params)
			}
		}
	}
	visit(f.Decls)
	return found
}

// PlanMetro returns what the Metro plugin adds to the program of files,
// which the files, parsed again, are then checked with. Its diagnostics
// are the plugin's.
func PlanMetro(files []*syntax.File, lib *Library) (*MetroPlan, []*syntax.Error) {
	u, checkers, err := signatures(context.Background(), files, lib, nil, Options{})
	if err != nil {
		return nil, []*syntax.Error{{Msg: err.Error()}}
	}
	broken := false
	for _, c := range checkers {
		c.allAnnotations()
		broken = broken || len(c.errs) > 0
	}
	cfg, err := parseMetroOptions(lib.MetroOptions)
	if err != nil {
		return nil, []*syntax.Error{{Msg: "kc: " + err.Error(), Feature: "metro"}}
	}
	m := newMetroAnalysis(u, checkers, cfg)
	m.run()
	if broken {
		// The program's own errors, which checking reports, may be why.
		m.errs = nil
	}
	return m.plan, m.errs
}

// metroOptions are the Metro plugin's options that change the code it
// writes, with their defaults. kc refuses the others but those that only
// tune Metro's own code generation, reports or diagnostics.
var metroOptions = map[string]string{
	"contributes-as-inject":                           "true",
	"enable-function-providers":                       "true",
	"optional-binding-behavior":                       "DEFAULT",
	"generate-contribution-providers":                 "false",
	"enable-top-level-function-injection":             "false",
	"generate-assisted-factories":                     "false",
	"enable-dagger-runtime-interop":                   "false",
	"enable-guice-runtime-interop":                    "false",
	"enable-dagger-anvil-interop":                     "false",
	"enable-kclass-to-class-interop":                  "false",
	"enable-suspend-providers":                        "false",
	"interop-include-javax-annotations":               "false",
	"interop-include-jakarta-annotations":             "false",
	"interop-include-dagger-annotations":              "false",
	"interop-include-kotlin-inject-annotations":       "false",
	"interop-include-anvil-annotations":               "false",
	"interop-include-kotlin-inject-anvil-annotations": "false",
	"interop-include-hilt-annotations":                "false",
	"interop-include-guice-annotations":               "false",
	"enable-circuit-codegen":                          "false",
}

// metroSupported are the options of metroOptions that kc reads; it
// refuses the others' values but their defaults.
var metroSupported = map[string]bool{
	"contributes-as-inject":               true,
	"enable-function-providers":           true,
	"interop-include-javax-annotations":   true,
	"interop-include-jakarta-annotations": true,
	"interop-include-dagger-annotations":  true,
	"enable-dagger-runtime-interop":       true,
}

// metroOptionalBehaviors are the values of optional-binding-behavior kc
// supports.
var metroOptionalBehaviors = map[string]bool{"DEFAULT": true, "DISABLED": true}

// metroConfig is what the plugin's options ask for.
type metroConfig struct {
	contributesAsInject, functionProviders bool
	// javax, jakarta, dagger and hilt are set if Metro reads those
	// frameworks' annotations as its own; as Metro's, Hilt's imply
	// Dagger's, and Dagger's javax.inject's and Jakarta's.
	javax, jakarta, dagger, hilt bool
	// daggerRuntime is set if Dagger's Lazy and Provider are injected;
	// kclassToClass if a class map key may be a java.lang.Class.
	daggerRuntime, kclassToClass bool
	// optional is optional-binding-behavior: DEFAULT, DISABLED or
	// REQUIRE_OPTIONAL_BINDING.
	optional string
}

// packages are the packages of the annotations Metro reads.
func (cfg metroConfig) packages() []string {
	pkgs := []string{metroPkg, "dev.zacsweers.metrox"}
	if cfg.javax {
		pkgs = append(pkgs, "javax.inject")
	}
	if cfg.jakarta {
		pkgs = append(pkgs, "jakarta.inject")
	}
	if cfg.dagger {
		pkgs = append(pkgs, "dagger")
	}
	return pkgs
}

// aliases maps the annotations of other frameworks that Metro reads as
// its own, by internal name, to Metro's. Only annotations that say what a
// declaration is to Metro are aliased: those that are part of a binding's
// key, as javax.inject.Named or Singleton, stay what they are, as Metro
// keeps them.
func (cfg metroConfig) aliases() map[string]string {
	a := map[string]string{}
	if cfg.javax {
		a["javax/inject/Inject"] = mInject
	}
	if cfg.jakarta {
		a["jakarta/inject/Inject"] = mInject
	}
	if cfg.dagger {
		// Dagger's map keys (ClassKey, StringKey, ...) and Reusable are
		// keys of their own, as in Metro.
		for k, v := range map[string]string{
			"dagger/Module":                        mBindingContainer,
			"dagger/Binds":                         mBinds,
			"dagger/Provides":                      mProvides,
			"dagger/BindsInstance":                 mProvides,
			"dagger/MapKey":                        mMapKey,
			"dagger/multibindings/IntoSet":         mIntoSet,
			"dagger/multibindings/IntoMap":         mIntoMap,
			"dagger/multibindings/ElementsIntoSet": mElementsIntoSet,
			"dagger/multibindings/Multibinds":      mMultibinds,
			"dagger/assisted/Assisted":             mAssisted,
			"dagger/assisted/AssistedInject":       mAssistedInject,
			"dagger/assisted/AssistedFactory":      mAssistedFactory,
		} {
			a[k] = v
		}
	}
	return a
}

// MetroOptionsError returns an error if kc can't compile as Metro does
// with the plugin's options opts.
func MetroOptionsError(opts map[string]string) error {
	_, err := parseMetroOptions(opts)
	return err
}

// parseMetroOptions reads the plugin's options. The error, if not nil,
// names an option whose value kc doesn't support; the config is still
// what the options ask for.
func parseMetroOptions(opts map[string]string) (metroConfig, error) {
	cfg := metroConfig{contributesAsInject: true, functionProviders: true, optional: "DEFAULT"}
	keys := make([]string, 0, len(opts))
	for key := range opts {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var err error
	for _, key := range keys {
		v := opts[key]
		on := v == "true"
		switch key {
		case "contributes-as-inject":
			cfg.contributesAsInject = v != "false"
		case "enable-function-providers":
			cfg.functionProviders = v != "false"
		case "optional-binding-behavior":
			if v != "" {
				cfg.optional = v
			}
		case "interop-include-javax-annotations":
			cfg.javax = cfg.javax || on
		case "interop-include-jakarta-annotations":
			cfg.jakarta = cfg.jakarta || on
		case "interop-include-dagger-annotations":
			cfg.dagger = cfg.dagger || on
		case "interop-include-hilt-annotations":
			cfg.hilt = cfg.hilt || on
		case "enable-dagger-runtime-interop":
			cfg.daggerRuntime = cfg.daggerRuntime || on
		case "enable-kclass-to-class-interop":
			cfg.kclassToClass = on
		}
		def, known := metroOptions[key]
		switch {
		case err != nil:
		case key == "optional-binding-behavior":
			if v != "" && !metroOptionalBehaviors[v] {
				err = fmt.Errorf("Metro option %s=%s is not supported", key, v)
			}
		case known && !metroSupported[key] && v != def && v != "":
			err = fmt.Errorf("Metro option %s=%s is not supported", key, v)
		case strings.HasPrefix(key, "custom-") && v != "":
			err = fmt.Errorf("Metro option %s (custom annotations) is not supported", key)
		}
	}
	if cfg.hilt {
		cfg.dagger = true
	}
	if cfg.dagger {
		cfg.javax, cfg.jakarta, cfg.daggerRuntime = true, true, true
	}
	return cfg, err
}

// allAnnotations resolves the annotations of the file and its
// declarations.
func (c *Checker) allAnnotations() {
	c.annotations(c.file.Annotations, nil, fileSite)
	for _, d := range c.file.Decls {
		c.declAnnotations(d, nil)
	}
	for _, d := range c.classes {
		if d.Sym != nil && d.Sym.Outer == nil {
			continue // a top-level class, checked above
		}
		c.declAnnotations(d, d.Sym.Outer)
	}
}

// metroInsert adds the plan's members to the file's classes, and replaces
// its calls of createGraph and createGraphFactory.
func (c *Checker) metroInsert() {
	plan := c.u.metro
	if plan == nil {
		return
	}
	prefix := ""
	if c.file.Package != "" {
		prefix = strings.ReplaceAll(c.file.Package, ".", "/") + "/"
	}
	var visit func(decls []syntax.Decl, outer string)
	visit = func(decls []syntax.Decl, outer string) {
		for _, d := range decls {
			cd, ok := d.(*syntax.ClassDecl)
			if !ok {
				continue
			}
			name := outer + cd.Name
			visit(cd.Members, name+"$")
			for _, src := range plan.supers[name] {
				ref, err := parseType(c.file.Path, cd.P, src)
				if err != nil {
					c.errorf(cd.P, "metro: %v", err)
					continue
				}
				cd.Supers = append(cd.Supers, &syntax.SuperType{Type: ref})
			}
			if srcs := plan.members[name]; len(srcs) > 0 {
				members, err := parseMembers(c.file.Path, cd.P, strings.Join(srcs, "\n"))
				if err != nil {
					c.errorf(cd.P, "metro: %v\n%s", err, strings.Join(srcs, "\n"))
					continue
				}
				cd.Members = append(cd.Members, members...)
			}
		}
	}
	visit(c.file.Decls, prefix)
	if calls := plan.calls[c.file.Path]; len(calls) > 0 {
		metroCalls(c.file, func(x *syntax.Call) {
			src, ok := calls[x.P]
			if !ok {
				return
			}
			r, err := parseExpr(c.file.Path, x.P, src)
			if err != nil {
				c.errorf(x.P, "metro: %v", err)
				return
			}
			*x = *r
		})
	}
}

// parseType parses a type, written in Kotlin, at position p.
func parseType(path string, p syntax.Pos, src string) (*syntax.TypeRef, error) {
	members, err := parseMembers(path, p, "val x: "+src+"\n")
	if err != nil {
		return nil, err
	}
	return members[0].(*syntax.PropDecl).Type, nil
}

// parseExpr parses a call, written in Kotlin, at position p.
func parseExpr(path string, p syntax.Pos, src string) (*syntax.Call, error) {
	members, err := parseMembers(path, p, "val x = "+src+"\n")
	if err != nil {
		return nil, err
	}
	call, ok := members[0].(*syntax.PropDecl).Init.(*syntax.Call)
	if !ok {
		return nil, fmt.Errorf("not a call: %s", src)
	}
	return call, nil
}

// metroCalls calls f for each call in the file's code that may be one of
// createGraph or createGraphFactory: a call by a simple or qualified name
// of them with one type argument.
func metroCalls(file *syntax.File, f func(*syntax.Call)) {
	allCalls(file, func(x *syntax.Call) {
		if len(x.TypeArgs) != 1 || len(x.Args) != 0 {
			return
		}
		name := ""
		switch fn := x.Fun.(type) {
		case *syntax.Ident:
			name = fn.Name
		case *syntax.Member:
			name = fn.Name
		}
		if name == "createGraph" || name == "createGraphFactory" {
			f(x)
		}
	})
}

// metroIntrinsics reports the calls of Metro's createGraph functions that
// kc has not replaced: their bodies only throw, as Metro's plugin replaces
// each call.
func (c *Checker) metroIntrinsics() {
	if c.u.metro == nil {
		return
	}
	allCalls(c.file, func(x *syntax.Call) {
		if x.Sym != nil && x.Sym.Facade == "dev/zacsweers/metro/CreateGraphKt" {
			c.unsupported(x.P, "metro", "kc can't compile this call of %s", x.Sym.Name)
		}
	})
}

// allCalls calls f for each call in the file's code.
func allCalls(file *syntax.File, f func(*syntax.Call)) {
	inspect := func(node any) {
		syntax.Inspect(node, func(n any) bool {
			if x, ok := n.(*syntax.Call); ok {
				f(x)
			}
			return true
		})
	}
	var decls func(ds []syntax.Decl)
	fun := func(d *syntax.FunDecl) {
		if d == nil {
			return
		}
		for _, p := range d.Params {
			inspect(p.Default)
		}
		inspect(d.Body)
		inspect(d.ExprBody)
	}
	decls = func(ds []syntax.Decl) {
		for _, d := range ds {
			switch d := d.(type) {
			case *syntax.FunDecl:
				fun(d)
			case *syntax.PropDecl:
				inspect(d.Init)
				fun(d.Getter)
				fun(d.Setter)
			case *syntax.InitBlock:
				inspect(d.Body)
			case *syntax.SecondaryCtor:
				inspect(d.Body)
			case *syntax.ClassDecl:
				for _, p := range d.CtorParams {
					inspect(p.Default)
				}
				for _, s := range d.Supers {
					inspect(s.Call)
					inspect(s.Delegate)
				}
				decls(d.Members)
			}
		}
	}
	decls(file.Decls)
}

// metroGraphCall returns the Kotlin expression that replaces a call of
// createGraph (factory false) or createGraphFactory with type argument
// cls, or "" if cls isn't a graph or a graph's factory.
func metroGraphCall(cls *types.ClassSym, factory bool) string {
	if !factory {
		return fqName(cls) + ".Companion.invoke()"
	}
	g := cls.Outer
	if g == nil {
		return ""
	}
	return "kotlin.run { " + fqName(g) + ".Companion }"
}

// kotlinTypeSrc writes type t as Kotlin, with qualified names, which
// reads the same in any file; ok is false if a class it names has no
// name there, as a local class.
func kotlinTypeSrc(t types.Type) (string, bool) {
	var s string
	switch t.K {
	case types.Unit:
		s = "kotlin.Unit"
	case types.Nothing, types.Null:
		s = "kotlin.Nothing"
	case types.Bool:
		s = "kotlin.Boolean"
	case types.Int:
		s = "kotlin.Int"
	case types.Long:
		s = "kotlin.Long"
	case types.Double:
		s = "kotlin.Double"
	case types.Char:
		s = "kotlin.Char"
	case types.Float:
		s = "kotlin.Float"
	case types.Byte:
		s = "kotlin.Byte"
	case types.Short:
		s = "kotlin.Short"
	case types.String:
		s = "kotlin.String"
	case types.Any:
		s = "kotlin.Any"
	case types.Class:
		if t.C.Local || t.C.Anonymous {
			return "", false
		}
		if t.IsFunc() {
			args := t.A.List
			var ps []string
			for _, a := range args[:len(args)-1] {
				p, ok := kotlinTypeSrc(a.T)
				if !ok {
					return "", false
				}
				ps = append(ps, p)
			}
			r, ok := kotlinTypeSrc(args[len(args)-1].T)
			if !ok {
				return "", false
			}
			s = "(" + strings.Join(ps, ", ") + ") -> " + r
			if t.C.Suspend {
				s = "suspend " + s
			}
			if t.N {
				s = "(" + s + ")?"
			}
			return s, true
		}
		s = metroClassName(t.C)
		if t.A != nil {
			var args []string
			for _, a := range t.A.List {
				if a.V == types.Star {
					args = append(args, "*")
					continue
				}
				as, ok := kotlinTypeSrc(a.T)
				if !ok {
					return "", false
				}
				switch a.V {
				case types.Out:
					as = "out " + as
				case types.In:
					as = "in " + as
				}
				args = append(args, as)
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
	case types.TypeParam, types.TypeVar:
		s = t.Param().Name
	default:
		return "", false
	}
	if t.N {
		s += "?"
	}
	return s, true
}

// metroClassName is the qualified name of a class, with Kotlin's names for
// the built-in classes that are Java's on the JVM.
func metroClassName(cls *types.ClassSym) string {
	parts := strings.Split(fqName(cls), ".")
	for i, p := range parts {
		if kotlinKeywords[p] {
			parts[i] = "`" + p + "`"
		}
	}
	return strings.Join(parts, ".")
}

var kotlinKeywords = map[string]bool{"as": true, "break": true, "class": true, "continue": true, "do": true, "else": true, "false": true,
	"for": true, "fun": true, "if": true, "in": true, "interface": true, "is": true, "null": true, "object": true, "package": true,
	"return": true, "super": true, "this": true, "throw": true, "true": true, "try": true, "typealias": true, "typeof": true,
	"val": true, "var": true, "when": true, "while": true}
