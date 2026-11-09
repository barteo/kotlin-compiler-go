package check

import (
	"cmp"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
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
	Default bool   // in a default import's package: visible unimported
}

// importables is a classTable's index of Importables.
type importables struct {
	once   sync.Once
	all    []Importable
	public sync.Map // internal name -> bool
}

// Accessible reports whether code outside im's package may name im: false
// for a class whose class file isn't public, as a Kotlin private class's
// or a Java package-private one's. It reads the class file's header once.
func (l *Library) Accessible(im Importable) bool {
	t := l.t
	if !im.Class || t.cp == nil {
		return true
	}
	name := strings.ReplaceAll(im.Package, ".", "/") + "/" + im.Name
	if im.Package == "" {
		name = im.Name
	}
	if v, ok := t.importables.public.Load(name); ok {
		return v.(bool)
	}
	ok := true
	if t.cp.Has(name) && !t.cp.IsSource(name) {
		if data, err := t.cp.Bytes(name); err == nil {
			access, err := classfile.AccessFlags(data)
			ok = err == nil && access&classfile.AccPublic != 0
		}
	}
	t.importables.public.Store(name, ok)
	return ok
}

// Importables returns the class path's Importables, sorted by name and package, for an editor's completion of
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
			if !importablePackage(pkg) {
				continue
			}
			def := defaults[dotted]
			for _, name := range t.classNames(pkg) {
				if importableName(name) {
					out = append(out, Importable{Name: name, Package: dotted, Class: true, Default: def})
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
						out = append(out, Importable{Name: name, Package: dotted, Class: true, Default: def})
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
							out = append(out, Importable{Name: name, Package: dotted, Default: def})
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
							out = append(out, Importable{Name: name, Package: dotted, Default: def})
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
