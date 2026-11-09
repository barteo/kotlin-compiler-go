package check

import (
	"cmp"
	"slices"
	"strings"
	"sync"
)

// Importable is a declaration of the class path that code names after
// importing it: a top-level class, or a top-level function or property
// that isn't an extension.
type Importable struct {
	Name    string
	Package string // dotted
	Class   bool   // a class or typealias; otherwise a function or property
}

// importables is a classTable's index of Importables.
type importables struct {
	once sync.Once
	all  []Importable
}

// Importables returns the class path's Importables outside the default
// imports, sorted by name and package, for an editor's completion of
// names that need an import. It reads the class path's Kotlin packages'
// metadata on first use, and keeps them.
func (l *Library) Importables() []Importable {
	t := l.t
	t.importables.once.Do(func() {
		if t.cp == nil {
			return
		}
		var out []Importable
		defaults := map[string]bool{"java.lang": true}
		for _, p := range defaultPackages {
			defaults[p] = true
		}
		for _, pkg := range t.cp.Packages() {
			dotted := strings.ReplaceAll(pkg, "/", ".")
			if defaults[dotted] || !importablePackage(pkg) {
				continue
			}
			for _, name := range t.classNames(pkg) {
				if importableName(name) {
					out = append(out, Importable{Name: name, Package: dotted, Class: true})
				}
			}
			if !t.kotlin {
				continue
			}
			kp := t.pkg(dotted)
			if kp == nil {
				continue
			}
			seen := map[string]bool{}
			for _, part := range kp.parts {
				funcs, props, aliases := part.meta.Names()
				for _, name := range aliases {
					if !seen["a "+name] {
						seen["a "+name] = true
						out = append(out, Importable{Name: name, Package: dotted, Class: true})
					}
				}
				for _, name := range funcs {
					fs, err := part.meta.Functions(name)
					if err != nil || seen["f "+name] {
						continue
					}
					for _, f := range fs {
						if f.Recv == nil && public(f.Flags.Visibility(), part.friend) {
							seen["f "+name] = true
							out = append(out, Importable{Name: name, Package: dotted})
							break
						}
					}
				}
				for _, name := range props {
					ps, err := part.meta.Properties(name)
					if err != nil || seen["f "+name] {
						continue
					}
					for _, p := range ps {
						if p.Recv == nil && public(p.Flags.Visibility(), part.friend) {
							seen["f "+name] = true
							out = append(out, Importable{Name: name, Package: dotted})
							break
						}
					}
				}
			}
		}
		slices.SortFunc(out, func(a, b Importable) int {
			return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Package, b.Package), cmp.Compare(btoi(b.Class), btoi(a.Class)))
		})
		t.importables.all = slices.CompactFunc(out, func(a, b Importable) bool { return a == b })
	})
	return t.importables.all
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// importablePackage reports whether code imports from package pkg, by
// internal name: not the JDK's internal packages.
func importablePackage(pkg string) bool {
	for _, prefix := range []string{"sun/", "jdk/", "com/sun/", "META-INF/"} {
		if strings.HasPrefix(pkg+"/", prefix) {
			return false
		}
	}
	return !strings.Contains(pkg+"/", "/internal/") && !strings.Contains(pkg+"/", "/impl/")
}

// importableName reports whether a class's simple name is one code names:
// not a Kotlin file facade's nor a multifile facade part's (classNames
// leaves out those its module lists; some modules don't list theirs) nor a
// synthetic class's.
func importableName(name string) bool {
	if name == "" || strings.HasPrefix(name, "_") || strings.HasSuffix(name, "Kt") || strings.Contains(name, "Kt__") {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}
