package check

import (
	"context"
	"fmt"
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

// UsesMetro reports whether a file imports declarations of Metro's
// runtime or names its annotations qualified, which the plugin then
// reads.
func UsesMetro(f *syntax.File) bool {
	for _, imp := range f.Imports {
		if imp.Path == metroPkg || strings.HasPrefix(imp.Path, metroPkg+".") || strings.HasPrefix(imp.Path, "dev.zacsweers.metrox.") {
			return true
		}
	}
	found := false
	var visit func(ds []syntax.Decl)
	visit = func(ds []syntax.Decl) {
		for _, d := range ds {
			if cd, ok := d.(*syntax.ClassDecl); ok {
				for _, a := range cd.Annotations {
					if strings.HasPrefix(a.Name, metroPkg+".") {
						found = true
					}
				}
				visit(cd.Members)
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
	u, checkers, err := signatures(context.Background(), files, lib, nil)
	if err != nil {
		return nil, []*syntax.Error{{Msg: err.Error()}}
	}
	broken := false
	for _, c := range checkers {
		c.allAnnotations()
		broken = broken || len(c.errs) > 0
	}
	m := newMetroAnalysis(u, checkers)
	if err := m.options(lib.MetroOptions); err != nil {
		return nil, []*syntax.Error{{Msg: "kc: " + err.Error(), Feature: "metro"}}
	}
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

// options reads the plugin's options.
func (m *metroAnalysis) options(opts map[string]string) error {
	for key, v := range opts {
		def, known := metroOptions[key]
		switch {
		case key == "contributes-as-inject":
			m.contributesAsInject = v != "false"
		case key == "enable-function-providers":
			m.functionProviders = v != "false"
		case key == "optional-binding-behavior":
			if v != "DEFAULT" && v != "" {
				return fmt.Errorf("Metro option %s=%s is not supported", key, v)
			}
		case known:
			if v != def && v != "" {
				return fmt.Errorf("Metro option %s=%s is not supported", key, v)
			}
		case strings.HasPrefix(key, "custom-") && v != "":
			return fmt.Errorf("Metro option %s (custom annotations) is not supported", key)
		}
	}
	return nil
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
	inspect := func(node any) {
		syntax.Inspect(node, func(n any) bool {
			if x, ok := n.(*syntax.Call); ok && len(x.TypeArgs) == 1 && len(x.Args) == 0 {
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
