package ka

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
)

// Package is a package of the class path: Model.Package.
type Package struct {
	Name string // dotted; "" for the root package
	// Classes are the class IDs of its top-level classes, Kotlin's (the
	// built-in ones too) and Java's, sorted; not Kotlin's file facades and
	// synthetic classes.
	Classes []string
	// Decls are its top-level Kotlin functions, properties and type
	// aliases, by kind, then by name. A function's or property's ClassID
	// is the class ID of the class that declares it on the JVM: its file
	// facade, or its multi-file class's part (kotlin/collections/
	// CollectionsKt__CollectionsKt), as FIR's JvmPackagePartSource.classId
	// has it. A type alias's is its own.
	Decls []*Decl
}

// Package returns the model of package pkg (dotted: kotlin.collections)
// of the class path; nil if neither it nor a package in it has a class.
// The sources' declarations aren't in it: they are the Model's Files.
func (m *Model) Package(pkg string) *Package {
	l := m.lib
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.pkgs == nil {
		l.pkgs = map[string]*Package{}
	}
	p, ok := l.pkgs[pkg]
	l.mu.Unlock()
	if ok {
		return p
	}
	p = l.pkg(pkg)
	l.mu.Lock()
	defer l.mu.Unlock()
	if old, ok := l.pkgs[pkg]; ok {
		return old
	}
	l.pkgs[pkg] = p
	return p
}

func (l *library) pkg(pkg string) *Package {
	names, ok := l.cp.Classes(strings.ReplaceAll(pkg, ".", "/"))
	if !ok {
		return nil
	}
	p := &Package{Name: pkg}
	for _, name := range names {
		if l.cp.IsSource(name) {
			continue
		}
		info, err := l.cp.Load(name)
		if err != nil || info.Access&classfile.AccSynthetic != 0 {
			continue
		}
		if info.Kotlin != nil && info.Kotlin.K != kmeta.KindClassFile {
			continue // a file facade, a multi-file class or its part, a lambda's class
		}
		p.Classes = append(p.Classes, internalClassID(name))
	}
	// The built-in classes, as kotlin/collections/List, have no class files.
	if pkg != "" {
		dir := strings.ReplaceAll(pkg, ".", "/")
		if data, err := l.cp.Resource(dir + "/" + pkg[strings.LastIndexByte(pkg, '.')+1:] + ".kotlin_builtins"); err == nil {
			if bs, err := kmeta.ReadBuiltins(data); err == nil {
				for _, c := range bs.Classes {
					if !strings.Contains(c.Name[len(dir)+1:], ".") {
						p.Classes = append(p.Classes, c.Name)
					}
				}
			}
		}
		slices.Sort(p.Classes)
		p.Classes = slices.Compact(p.Classes)
	}
	var funcs, props, aliases []*Decl
	for _, part := range l.parts()[pkg] {
		info, err := l.cp.Load(part.Class)
		if err != nil || info.Kotlin == nil {
			continue
		}
		meta, err := kmeta.ReadPackage(info.Kotlin)
		if err != nil {
			continue
		}
		bySig := map[string]*classfile.MemberDetails{}
		consts := map[string]any{}
		if data, err := l.cp.Bytes(part.Class); err == nil {
			if det, err := classfile.ParseDetails(data); err == nil {
				_, bySig = methods(det)
			}
			if info, err := classfile.Parse(data); err == nil {
				consts = fieldConstants(info)
			}
		}
		owner := internalClassID(part.Class)
		fs, ps, as := meta.Names()
		for _, n := range fs {
			all, _ := meta.Functions(n)
			for _, f := range all {
				if f.Flags.MemberKind() == 0 {
					d := l.kfunction(f, kscope{}, bySig)
					d.ClassID = owner
					d.Module = meta.ModuleName()
					funcs = append(funcs, d)
				}
			}
		}
		for _, n := range ps {
			all, _ := meta.Properties(n)
			for _, pr := range all {
				if pr.Flags.MemberKind() == 0 {
					d := l.kproperty(pr, kscope{}, bySig, consts)
					d.ClassID = owner
					d.Module = meta.ModuleName()
					props = append(props, d)
				}
			}
		}
		prefix := strings.ReplaceAll(pkg, ".", "/") + "/"
		for _, n := range as {
			all, _ := meta.TypeAliases(n)
			for _, a := range all {
				aliases = append(aliases, l.ktypeAlias(a, prefix))
			}
		}
	}
	for _, ds := range [][]*Decl{funcs, props, aliases} {
		slices.SortStableFunc(ds, func(a, b *Decl) int { return strings.Compare(a.Name, b.Name) })
		p.Decls = append(p.Decls, ds...)
	}
	return p
}

// ktypeAlias returns the model of typealias a of the package whose class
// IDs start with prefix (a/b/).
func (l *library) ktypeAlias(a *kmeta.TypeAlias, prefix string) *Decl {
	d := &Decl{Kind: KindTypeAlias, Name: a.Name, ClassID: prefix + a.Name,
		Visibility: kvisibility(a.Flags.Visibility())}
	scope := kscope{}
	d.TypeParams = l.ktypeParams(a.TypeParams, scope)
	if a.Expanded != nil {
		d.Type = l.ktype(a.Expanded, scope, 0)
	}
	if a.Underlying != nil {
		u := l.ktype(a.Underlying, scope, 0)
		switch {
		case d.Type == nil:
			d.Type = u
		case d.Type.Abbrev == nil && u.Abbrev != nil:
			// typealias A = B, where B is a typealias: the expansion is
			// written as B.
			d.Type.Abbrev = u.Abbrev
		}
	}
	return d
}

// parts returns the classes that hold each package's top-level Kotlin
// declarations, from the class path's .kotlin_module files, by package
// (dotted).
func (l *library) parts() map[string][]kmeta.Part {
	l.partsOnce.Do(func() {
		l.partsByPkg = map[string][]kmeta.Part{}
		// Module names repeat across jars, so every copy is read; a part
		// two jars list is kept once.
		seen := map[string]bool{}
		for _, name := range l.cp.Resources(".kotlin_module") {
			all, err := l.cp.ResourceAll(name)
			if err != nil {
				continue
			}
			for _, data := range all {
				m, err := kmeta.ReadModule(data)
				if err != nil {
					continue
				}
				for pkg, parts := range m.Packages {
					for _, part := range parts {
						if !seen[part.Class] {
							seen[part.Class] = true
							l.partsByPkg[pkg] = append(l.partsByPkg[pkg], part)
						}
					}
				}
			}
		}
		for _, parts := range l.partsByPkg {
			slices.SortFunc(parts, func(a, b kmeta.Part) int { return strings.Compare(a.Class, b.Class) })
		}
	})
	return l.partsByPkg
}

// defaultAliases returns the typealiases of the packages that files
// import by default that stand for a class with their own type
// parameters, as typealias ArrayList<E> = java.util.ArrayList<E>, by
// name.
func (l *library) defaultAliases() map[string]*Decl {
	l.defaultOnce.Do(func() {
		l.defaults = map[string]*Decl{}
		m := &Model{lib: l}
		for _, pkg := range defaultImports {
			p := m.Package(pkg)
			if p == nil {
				continue
			}
			for _, d := range p.Decls {
				if _, dup := l.defaults[d.Name]; classAlias(d) && !dup {
					l.defaults[d.Name] = d
				}
			}
		}
	})
	return l.defaults
}

// classAlias reports whether d is a public typealias that the checker
// resolves to the class it expands to: one of a class with its type
// parameters as its arguments, as typealias L<T> = java.util.List<T>.
func classAlias(d *Decl) bool {
	if d.Kind != KindTypeAlias || d.Visibility != Public || d.Type == nil || d.Type.Kind != TypeClass ||
		d.Type.Nullable || len(d.Type.Args) != len(d.TypeParams) {
		return false
	}
	for i, a := range d.Type.Args {
		if a.Variance != Invariant || a.Type == nil || a.Type.Kind != TypeParameter || a.Type.Name != d.TypeParams[i].Name {
			return false
		}
	}
	return true
}

// packageClassAlias returns the class-path typealias name of package pkg
// if classAlias holds for it, or nil.
func (l *library) packageClassAlias(pkg, name string) *Decl {
	p := (&Model{lib: l}).Package(pkg)
	if p == nil {
		return nil
	}
	for _, d := range p.Decls {
		if d.Name == name && classAlias(d) {
			return d
		}
	}
	return nil
}
