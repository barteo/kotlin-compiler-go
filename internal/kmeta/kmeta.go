// Package kmeta decodes Kotlin metadata: the @kotlin.Metadata annotation
// of class files, .kotlin_module files and .kotlin_builtins files. It
// implements the subset of protobuf it needs, following the .proto files in
// the Kotlin repository (core/metadata and core/metadata.jvm). It also
// encodes @kotlin.Metadata and .kotlin_module files, for kc's output.
package kmeta

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// Kinds of @kotlin.Metadata (the k argument).
const (
	KindClassFile      = 1
	KindFileFacade     = 2
	KindSynthetic      = 3
	KindMultiFileClass = 4
	KindMultiFilePart  = 5
)

// messages splits the metadata of a class file into its name resolver and
// the bytes of its Class or Package message.
func messages(m *classfile.KotlinMeta) (names, []byte) {
	b := Bytes(m.D1)
	d := decoder{b}
	types := d.bytes() // a length-delimited StringTableTypes
	return readJVMNames(types, m.D2), d.b
}

// ReadClass decodes the metadata of a class (kind 1).
func ReadClass(m *classfile.KotlinMeta) (c *Class, err error) {
	defer catch(&err)
	if m.K != KindClassFile {
		return nil, fmt.Errorf("kotlin metadata: kind %d is not a class", m.K)
	}
	n, msg := messages(m)
	return scope{names: n}.class(msg), nil
}

// ReadLambda decodes the metadata of a lambda's class (kind 3): its
// function, or nil for a synthetic class without one.
func ReadLambda(m *classfile.KotlinMeta) (f *Function, err error) {
	defer catch(&err)
	if m.K != KindSynthetic {
		return nil, fmt.Errorf("kotlin metadata: kind %d is not a synthetic class", m.K)
	}
	if len(m.D1) == 0 {
		return nil, nil
	}
	n, msg := messages(m)
	return scope{names: n}.function(msg), nil
}

// Package holds the top-level declarations of a file facade or multi-file
// part (kinds 2 and 5), or of a .kotlin_builtins package. Declarations are
// indexed by name when the package is read and decoded on request, so a
// program pays only for the names it uses. A Package is safe for
// concurrent use.
type Package struct {
	scope   scope
	module  string // package_module_name
	funcs   map[string][][]byte
	props   map[string][][]byte
	aliases map[string][][]byte
}

// ReadPackage reads the metadata of a file facade or multi-file part.
func ReadPackage(m *classfile.KotlinMeta) (p *Package, err error) {
	defer catch(&err)
	if m.K != KindFileFacade && m.K != KindMultiFilePart {
		return nil, fmt.Errorf("kotlin metadata: kind %d is not a file facade", m.K)
	}
	n, msg := messages(m)
	return readPackage(scope{names: n}, msg), nil
}

func readPackage(s scope, msg []byte) *Package {
	if t := find(msg, 30); t != nil {
		s.types = readTypeTable(t)
	}
	p := &Package{scope: s, funcs: map[string][][]byte{}, props: map[string][][]byte{}, aliases: map[string][][]byte{}}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		var m map[string][][]byte
		switch f {
		case 3:
			m = p.funcs
		case 4:
			m = p.props
		case 5:
			m = p.aliases
		case 101:
			if w == wireVarint {
				p.module = s.names.str(d.int())
				continue
			}
			d.skip(w)
			continue
		default:
			d.skip(w)
			continue
		}
		b := d.bytes()
		name := s.names.str(nameOf(b))
		m[name] = append(m[name], b)
	}
	return p
}

// nameOf returns field 2, the name of a Function, Property or TypeAlias.
func nameOf(msg []byte) int {
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f == 2 && w == wireVarint {
			return d.int()
		}
		d.skip(w)
	}
	fail("declaration has no name")
	return 0
}

// Names returns the names of the package's functions, properties and type
// aliases.
func (p *Package) Names() (funcs, props, aliases []string) {
	keys := func(m map[string][][]byte) []string {
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		return ks
	}
	return keys(p.funcs), keys(p.props), keys(p.aliases)
}

// Functions decodes the functions with a name.
func (p *Package) Functions(name string) (fs []*Function, err error) {
	defer catch(&err)
	for _, b := range p.funcs[name] {
		fs = append(fs, p.scope.function(b))
	}
	return fs, nil
}

// Properties decodes the properties with a name.
func (p *Package) Properties(name string) (ps []*Property, err error) {
	defer catch(&err)
	for _, b := range p.props[name] {
		ps = append(ps, p.scope.property(b))
	}
	return ps, nil
}

// TypeAliases decodes the type aliases with a name.
func (p *Package) TypeAliases(name string) (as []*TypeAlias, err error) {
	defer catch(&err)
	for _, b := range p.aliases[name] {
		as = append(as, p.scope.typeAlias(b))
	}
	return as, nil
}

// PackageData is all the declarations of a file facade, as WritePackage
// writes them and Package.All decodes them.
type PackageData struct {
	Funcs      []*Function
	Props      []*Property
	Aliases    []*TypeAlias
	ModuleName string // the module, if it isn't "main"
}

// All decodes all the package's declarations, ordered by name.
func (p *Package) All() (*PackageData, error) {
	out := &PackageData{ModuleName: p.module}
	fs, ps, as := p.Names()
	slices.Sort(fs)
	slices.Sort(ps)
	slices.Sort(as)
	for _, n := range fs {
		d, err := p.Functions(n)
		if err != nil {
			return nil, err
		}
		out.Funcs = append(out.Funcs, d...)
	}
	for _, n := range ps {
		d, err := p.Properties(n)
		if err != nil {
			return nil, err
		}
		out.Props = append(out.Props, d...)
	}
	for _, n := range as {
		d, err := p.TypeAliases(n)
		if err != nil {
			return nil, err
		}
		out.Aliases = append(out.Aliases, d...)
	}
	return out, nil
}

// Builtins is a .kotlin_builtins file: the built-in classes of one package,
// such as kotlin.Int and kotlin.collections.List, which have no class
// files.
type Builtins struct {
	Package *Package
	Classes []*Class
}

// ReadBuiltins decodes a .kotlin_builtins file: a binary version, then a
// PackageFragment message.
func ReadBuiltins(data []byte) (b *Builtins, err error) {
	defer catch(&err)
	rest := skipVersion(data, false)
	var strs, qnames, pkg []byte
	var classes [][]byte
	d := decoder{rest}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			strs = d.bytes()
		case 2:
			qnames = d.bytes()
		case 3:
			pkg = d.bytes()
		case 4:
			classes = append(classes, d.bytes())
		default:
			d.skip(w)
		}
	}
	s := scope{names: &tableNames{strs: readStringTable(strs), qnames: readQualifiedNames(qnames)}}
	b = &Builtins{Package: readPackage(s, pkg)}
	for _, c := range classes {
		b.Classes = append(b.Classes, s.class(c))
	}
	return b, nil
}

// skipVersion skips the version that .kotlin_module and .kotlin_builtins
// files start with: a count and that many big-endian ints. Module files
// written by Kotlin 1.4 and later then have an int of flags.
func skipVersion(data []byte, module bool) []byte {
	if len(data) < 4 {
		fail("truncated version")
	}
	n := int(binary.BigEndian.Uint32(data))
	if n < 0 || n > 1024 || len(data) < 4+4*n {
		fail("bad version")
	}
	v := make([]int, n)
	for i := range v {
		v[i] = int(binary.BigEndian.Uint32(data[4+4*i:]))
	}
	data = data[4+4*n:]
	if module && len(v) >= 2 && (v[0] > 1 || v[0] == 1 && v[1] >= 4) {
		if len(data) < 4 {
			fail("truncated flags")
		}
		data = data[4:]
	}
	return data
}

// Part is a class with top-level declarations of a package: a file facade,
// or a part of a multi-file class.
type Part struct {
	Class  string // internal name
	Facade string // the multi-file facade, or "" for a file facade
}

// Module is a .kotlin_module file: which classes hold each package's
// top-level declarations.
type Module struct {
	Packages map[string][]Part // by package name, e.g. kotlin.collections
}

// ReadModule decodes a .kotlin_module file (ModuleMapping.loadModuleMapping).
func ReadModule(data []byte) (m *Module, err error) {
	defer catch(&err)
	m = &Module{Packages: map[string][]Part{}}
	var jvmPackages []string
	var parts [][]byte
	d := decoder{skipVersion(data, true)}
	for d.more() {
		f, w := d.key()
		switch f {
		case 1:
			parts = append(parts, d.bytes())
		case 3:
			jvmPackages = append(jvmPackages, string(d.bytes()))
		default:
			d.skip(w)
		}
	}
	for _, b := range parts {
		var pkg string
		var short, facades, jvmShort []string
		var facadeIDs, jvmFacadeIDs, jvmPkgIDs []int
		d := decoder{b}
		for d.more() {
			f, w := d.key()
			switch f {
			case 1:
				pkg = string(d.bytes())
			case 2:
				short = append(short, string(d.bytes()))
			case 3:
				facadeIDs = d.ints(w, facadeIDs)
			case 4:
				facades = append(facades, string(d.bytes()))
			case 5:
				jvmShort = append(jvmShort, string(d.bytes()))
			case 7:
				jvmFacadeIDs = d.ints(w, jvmFacadeIDs)
			case 6:
				jvmPkgIDs = d.ints(w, jvmPkgIDs)
			default:
				d.skip(w)
			}
		}
		facade := func(ids []int, i int, jvmPkg string) string {
			if i >= len(ids) || ids[i] <= 0 || ids[i] > len(facades) {
				return ""
			}
			return internalName(jvmPkg, facades[ids[i]-1])
		}
		for i, s := range short {
			m.Packages[pkg] = append(m.Packages[pkg], Part{internalName(pkg, s), facade(facadeIDs, i, pkg)})
		}
		for i, s := range jvmShort {
			id := -1
			if i < len(jvmPkgIDs) {
				id = jvmPkgIDs[i]
			} else if len(jvmPkgIDs) > 0 {
				id = jvmPkgIDs[len(jvmPkgIDs)-1]
			}
			if id < 0 || id >= len(jvmPackages) {
				continue
			}
			jp := jvmPackages[id]
			m.Packages[pkg] = append(m.Packages[pkg], Part{internalName(jp, s), facade(jvmFacadeIDs, i, jp)})
		}
	}
	return m, nil
}

// internalName returns the internal name of a class in a package with a
// dotted name.
func internalName(pkg, short string) string {
	if pkg == "" {
		return short
	}
	return strings.ReplaceAll(pkg, ".", "/") + "/" + short
}
