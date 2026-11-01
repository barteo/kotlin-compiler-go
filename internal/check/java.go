package check

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// classTable turns class files into symbols. Each class gets one ClassSym
// when it is first referenced; its members are read when first used.
// Body checking runs in parallel, so this is safe for concurrent use.
type classTable struct {
	cp *classpath.Path // nil: no class path
	mu sync.Mutex
	m  map[string]*types.ClassSym

	// kotlin is set when the Kotlin standard library is on the class path:
	// Kotlin declarations are then read from metadata (see kotlin.go).
	kotlin bool
	// serialization is Library.Serialization: see friend.
	serialization bool
	funcs         *types.Functions // the interfaces of function types
	kotlinTable
	// sources finds the program's classes by internal name, which the
	// class path's classes of Java sources name. Checking sets it once
	// the program's classes are declared.
	sources atomic.Pointer[func(string) *types.ClassSym]

	// Caches of results that depend only on loaded classes, which checking
	// asks for again and again: supertypes with their type arguments, and
	// the methods of a class by name.
	supers  sync.Map // superKey -> superResult
	methods sync.Map // methodsKey -> []*types.FunSym
	// defaults has a sync.Once for each method whose inherited default
	// values inheritExtDefaults looked for.
	defaults     sync.Map // *types.FunSym -> *sync.Once
	inlineBodies sync.Map // inlineKey -> func() (*inline.Body, error)
}

type superKey struct {
	t      types.Type
	target *types.ClassSym
}

type superResult struct {
	t  types.Type
	ok bool
}

type methodsKey struct {
	cls  *types.ClassSym
	name string
}

func newClassTable(cp *classpath.Path) *classTable {
	t := &classTable{cp: cp, m: map[string]*types.ClassSym{}, funcs: types.KcFunctions}
	if cp != nil && cp.HasKotlin() {
		t.kotlin = true
		t.funcs = types.KotlinFunctions
		t.collections = map[string]*types.ClassSym{}
		t.raw = map[string]*types.ClassSym{}
		t.pkgs = map[string]*kpkg{}
	}
	return t
}

// class returns the class with the given internal name, or nil if it is
// not on the class path. With the Kotlin standard library, the Java
// collection interfaces are the mutable collection types, and the
// function interfaces are function types.
func (t *classTable) class(name string) *types.ClassSym {
	if t.kotlin {
		if id, ok := mutableOf[name]; ok {
			return t.kclass(id)
		}
		if n, ok := functionArity(name, types.KotlinFunctionPackage); ok {
			return t.funcs.Class(n)
		}
		if n, ok := functionArity(name, "kotlin/reflect/K"); ok {
			return t.kfunction(n, false)
		}
		if n, ok := functionArity(name, "kotlin/reflect/KSuspend"); ok {
			return t.kfunction(n, true)
		}
		if n, ok := functionArity(name, "kotlin/coroutines/Suspend"); ok {
			return t.funcs.SuspendClass(n) // kotlin.coroutines.SuspendFunction1, suspend (P1) -> R
		}
	}
	if t.cp == nil || !t.cp.Has(name) {
		if src := t.sources.Load(); src != nil {
			return (*src)(name)
		}
		return nil
	}
	t.mu.Lock()
	c, ok := t.m[name]
	t.mu.Unlock()
	if ok {
		return c
	}
	// Whether it is a value class is known before anyone sees it: a
	// type's descriptor depends on it, as a field's of Compose's Color,
	// whose members may never be loaded.
	c = types.NewExternal(name, t.load)
	t.markValue(c)
	t.mu.Lock()
	defer t.mu.Unlock()
	if old, ok := t.m[name]; ok {
		return old
	}
	t.m[name] = c
	return c
}

func (t *classTable) hasPackage(pkg string) bool { return t.cp != nil && t.cp.HasPackage(pkg) }

// load fills in a class's supertypes and public members.
func (t *classTable) load(cls *types.ClassSym) {
	info, err := t.cp.Load(cls.Name)
	if err != nil {
		cls.Missing = true
		return
	}
	cls.JVMMethods = make(map[string]uint16, len(info.Methods))
	for _, m := range info.Methods {
		cls.JVMMethods[m.Name+m.Desc] = m.Access
	}
	cls.StabilityInferred = info.StabilityInferred
	for _, a := range slices.Concat(info.Invisible, info.Visible) {
		if strings.HasPrefix(a, "L") && strings.HasSuffix(a, ";") {
			if c := t.class(a[1 : len(a)-1]); c != nil {
				cls.Annotations = append(cls.Annotations, c)
			}
		}
	}
	if info.Access&classfile.AccAnnotation != 0 {
		cls.Retention = types.RetentionOf(info.Retention)
		if cls.Retention == types.NoRetention {
			cls.Retention = types.RetentionBinary // Java's default, CLASS
		}
		if info.Repeatable != "" {
			cls.Container = t.class(info.Repeatable)
		}
		if info.KotlinTargets != nil {
			cls.Targets = types.KotlinTargets(info.KotlinTargets)
		} else {
			cls.Targets = types.JavaTargets(info.JavaTargets)
		}
	}
	if t.kotlin && info.Kotlin != nil && info.Kotlin.K == kmeta.KindClassFile {
		if kc, err := kmeta.ReadClass(info.Kotlin); err == nil {
			t.loadKotlin(cls, info, kc)
			return
		}
	}
	cls.Interface = info.Access&classfile.AccInterface != 0
	cls.Abstract = info.Access&(classfile.AccAbstract|classfile.AccInterface) != 0
	cls.Final = info.Access&classfile.AccFinal != 0
	cls.Private = info.Access&classfile.AccPublic == 0
	if info.Super != "" {
		cls.Super = t.class(info.Super)
	}
	for _, name := range info.Interfaces {
		if i := t.class(name); i != nil {
			cls.Interfaces = append(cls.Interfaces, i)
		}
	}
	if info.Sig != "" {
		t.classSignature(cls, info.Sig)
	}
	cls.Methods = map[string][]*types.FunSym{}
	cls.Statics = map[string][]*types.FunSym{}
	cls.Fields = map[string]*types.VarSym{}
	for _, m := range info.Methods {
		// Protected members are for subclasses, which can be Kotlin classes.
		if m.Access&(classfile.AccPublic|classfile.AccProtected) == 0 || m.Access&(classfile.AccSynthetic|classfile.AccBridge) != 0 {
			continue
		}
		f := &types.FunSym{Name: m.Name, Owner: cls, Desc: m.Desc, Static: m.Access&classfile.AccStatic != 0, Operator: true, Resolved: true,
			Protected: m.Access&classfile.AccPublic == 0, Final: m.Access&classfile.AccFinal != 0 || cls.Final}
		// A varargs method's last parameter is parsed as its element type.
		desc, sig := m.Desc, m.Sig
		varargs := m.Access&classfile.AccVarargs != 0
		vok := true
		if varargs {
			desc, vok = varargElem(desc)
			if sig != "" {
				var sok bool
				sig, sok = varargElem(sig)
				vok = vok && sok
			}
		}
		ok := t.signature(f, desc)
		if sig != "" {
			ok = t.methodSignature(f, cls, sig, len(f.Params))
		}
		for _, p := range f.Params {
			p.Type = javaParam(p.Type)
		}
		if varargs && len(f.Params) > 0 {
			p := f.Params[len(f.Params)-1]
			p.Vararg, p.Type = true, types.VarargArray(p.Type)
		}
		f.Unsupported = !ok || !vok
		f.Abstract = m.Access&classfile.AccAbstract != 0
		if m.Name == "toString" && len(f.Params) == 0 && kotlinMapped[cls.Name] {
			// Kotlin sees these classes as Kotlin types, whose toString is
			// Any's: a String, not a platform String!.
			f.Ret = types.StringType
		}
		switch {
		case m.Name == "<clinit>":
		case m.Name == "<init>":
			// An abstract class's constructors are for its subclasses.
			f.Ret = types.ClassType(cls)
			cls.Ctors = append(cls.Ctors, f)
			if names := builtinParams[cls.Name+m.Desc]; len(names) == len(f.Params) {
				for i, p := range f.Params {
					p.Name = names[i]
				}
			}
		case f.Static:
			cls.Statics[m.Name] = append(cls.Statics[m.Name], f)
		default:
			cls.Methods[m.Name] = append(cls.Methods[m.Name], f)
		}
	}
	if cls.Name == "java/lang/Number" {
		// Kotlin's Number converts with toInt() and the others, its
		// intValue() methods on the JVM.
		for _, conv := range numberConversions {
			for _, f := range cls.Methods[conv[1]] {
				k := *f
				k.Name, k.JVMName = conv[0], conv[1]
				cls.Methods[conv[0]] = append(cls.Methods[conv[0]], &k)
			}
		}
	}
	for _, fd := range info.Fields {
		if fd.Access&classfile.AccStatic == 0 && fd.Access&(classfile.AccPublic|classfile.AccProtected) != 0 && fd.Access&classfile.AccSynthetic == 0 {
			// An instance field: a property to Kotlin, as AbstractList's
			// modCount is in its subclasses.
			if ft, ok := t.fieldType(fd.Desc); ok && ft.K != types.Unit {
				if fd.Sig != "" {
					if st, sok := t.fieldSignature(fd.Sig, cls.TypeParams); sok {
						ft = st
					}
				}
				if cls.JavaFields == nil {
					cls.JavaFields = map[string]*types.VarSym{}
				}
				cls.JavaFields[fd.Name] = &types.VarSym{Name: fd.Name, Type: ft.Platform(), Kind: types.Prop, Owner: cls, Resolved: true, Final: true,
					JvmField: true, Mutable: fd.Access&classfile.AccFinal == 0, Protected: fd.Access&classfile.AccProtected != 0}
			}
			continue
		}
		if fd.Access&classfile.AccPublic == 0 || fd.Access&classfile.AccStatic == 0 {
			continue
		}
		ft, ok := t.fieldType(fd.Desc)
		if fd.Sig != "" {
			ft, ok = t.fieldSignature(fd.Sig, nil)
		}
		if ok && ft.K != types.Unit {
			cls.Fields[fd.Name] = &types.VarSym{Name: fd.Name, Type: ft, Kind: types.Static, Owner: cls, Resolved: true,
				Constant: fieldConstant(fd), Mutable: fd.Access&classfile.AccFinal == 0}
		}
		if fd.Access&classfile.AccEnum != 0 {
			cls.Entries = append(cls.Entries, fd.Name)
		}
	}
}

// kotlinMapped are the Java classes that are Kotlin types, with Kotlin's
// members.
var kotlinMapped = func() map[string]bool {
	m := map[string]bool{"java/lang/Object": true}
	for _, jc := range javaLangOf {
		m[jc] = true
	}
	return m
}()

// fieldType maps a JVM field descriptor to a Kotlin type. Reference types
// are platform types, as Kotlin sees Java declarations.
func (t *classTable) fieldType(d string) (types.Type, bool) {
	switch d {
	case "I":
		return types.IntType, true
	case "J":
		return types.LongType, true
	case "D":
		return types.DoubleType, true
	case "Z":
		return types.BoolType, true
	case "C":
		return types.CharType, true
	case "F":
		return types.FloatType, true
	case "B":
		return types.ByteType, true
	case "S":
		return types.ShortType, true
	case "V":
		return types.UnitType, true
	}
	if elem, ok := strings.CutPrefix(d, "["); ok {
		et, ok := t.fieldType(elem)
		if !ok || et.K == types.Unit {
			return types.InvalidType, false
		}
		return types.ArrayOf(et).Platform(), true
	}
	name, ok := strings.CutPrefix(d, "L")
	if !ok {
		return types.InvalidType, false
	}
	switch name = strings.TrimSuffix(name, ";"); name {
	case "java/lang/String":
		return types.StringType.Platform(), true
	case "java/lang/Object":
		return types.AnyType.Platform(), true
	}
	for k, box := range boxes {
		if name == box {
			return types.Type{K: k}.Nullable().Platform(), true
		}
	}
	if c := t.class(name); c != nil {
		return types.ClassType(c).Platform(), true
	}
	return types.InvalidType, false
}

// signature fills in f's parameters and return type from a method
// descriptor and reports whether kc supports all of them.
func (t *classTable) signature(f *types.FunSym, d string) bool {
	ok := true
	i := 1
	for d[i] != ')' {
		j := i
		for d[j] == '[' {
			j++
		}
		if d[j] == 'L' {
			j = i + strings.IndexByte(d[i:], ';')
		}
		pt, pok := t.fieldType(d[i : j+1])
		ok = ok && pok
		f.Params = append(f.Params, &types.VarSym{Name: "p", Type: pt, Kind: types.Param})
		i = j + 1
	}
	rt, rok := t.fieldType(d[i+1:])
	f.Ret = rt
	return ok && rok
}

// javaParam returns the type of a Java method's parameter as Kotlin sees
// it: an array parameter, T[], also takes arrays of subtypes of T.
func javaParam(t types.Type) types.Type {
	if t.K != types.Class || t.C != types.ArrayClass || t.A.Get(0).V != types.Inv {
		return t
	}
	return types.GenericType(t.C, []types.Arg{{V: types.Out, T: t.A.Get(0).T}}).Platform()
}

// boxes are the box classes of the primitive types.
var boxes = types.Boxes

// Kotlin type names never refer to java.lang classes.
var kotlinTypes = map[string]bool{
	"Int": true, "Long": true, "Double": true, "Boolean": true, "String": true,
	"Char": true, "Float": true, "Byte": true, "Short": true,
	"Unit": true, "Nothing": true, "Any": true,
}

// resolveImports checks the file's imports. Explicit imports bind a simple
// name to a class, or to Kotlin top-level functions; star imports add a
// package to search.
func (c *Checker) resolveImports() {
	c.imports = map[string]*types.ClassSym{}
	c.aliasImports = map[string]*types.AliasSym{}
	c.funImports = map[string][]*types.FunSym{}
	c.memberImports = map[string]memberImport{}
	c.userFuns = map[string][]*types.FunSym{}
	c.userVars = map[string]*types.VarSym{}
	c.userExtProps = map[string][]*types.FunSym{}
	ext := c.u.ext
	// The file's package on the class path, as a library's Java classes
	// in it, or its classes in the default package: searched before the
	// star imports.
	if name := strings.ReplaceAll(c.file.Package, ".", "/"); ext.hasPackage(name) {
		c.stars = append(c.stars, name)
	}
	starred := map[string]bool{}
	var memberStars []*syntax.Import
	for _, imp := range c.file.Imports {
		name := strings.ReplaceAll(imp.Path, ".", "/")
		if imp.Star {
			if starred[imp.Path] {
				continue // imported twice: its members aren't overloads of themselves
			}
			starred[imp.Path] = true
			user := c.u.Pkgs[imp.Path]
			if user != nil && imp.Path != "" {
				c.userStars = append(c.userStars, user)
			}
			if ext.hasPackage(name) || ext.isKotlinPackage(imp.Path) {
				c.stars = append(c.stars, name)
			} else if owner := c.importOwner(imp.Path); user == nil && owner != nil && owner.Object && !owner.External {
				c.errorf(imp.P, "cannot import on demand from object '%s'", owner.DisplayName())
			} else if user == nil && owner != nil {
				memberStars = append(memberStars, imp)
			} else if user == nil && !optionalExpectationPackages[imp.Path] {
				c.errorf(imp.P, "unresolved import: %s.*", imp.Path)
			}
			continue
		}
		alias := imp.Alias
		if alias == "" {
			alias = imp.Path[strings.LastIndexByte(imp.Path, '.')+1:]
		}
		pkg, simple := "", imp.Path
		if i := strings.LastIndexByte(imp.Path, '.'); i > 0 {
			pkg, simple = imp.Path[:i], imp.Path[i+1:]
		}
		// The program's own declarations come first. Declarations in the
		// default package can't be imported.
		if user := c.u.Pkgs[pkg]; user != nil && pkg != "" {
			found := false
			if cls, ok := user.Classes[simple]; ok {
				c.imports[alias], found = cls, true
			}
			if a, ok := user.Aliases[simple]; ok {
				c.aliasImports[alias], found = a, true
			}
			if fs := user.Funs[simple]; len(fs) > 0 {
				// Functions of one name imported from several packages
				// are overloads: clikt's options.convert and
				// arguments.convert.
				for _, f := range fs {
					if !slices.Contains(c.userFuns[alias], f) {
						c.userFuns[alias] = append(c.userFuns[alias], f)
					}
				}
				found = true
			}
			if v, ok := user.Vars[simple]; ok {
				c.userVars[alias], found = v, true
			}
			if gs := user.ExtProps[simple]; len(gs) > 0 {
				for _, g := range gs {
					if !slices.Contains(c.userExtProps[alias], g) {
						c.userExtProps[alias] = append(c.userExtProps[alias], g)
					}
				}
				found = true
			}
			if found {
				continue
			}
		}
		cls := ext.class(name)
		if cls == nil && ext.kotlin && pkg != "" {
			cls = ext.pkgClass(pkg, simple)
		}
		fs := ext.pkgFuns(pkg, simple)
		if cls != nil {
			c.imports[alias] = cls
		}
		var ta *types.AliasSym
		if cls == nil && ext.kotlin && pkg != "" {
			if ta = ext.pkgAlias(pkg, simple); ta != nil {
				c.aliasImports[alias] = ta
			}
		}
		// Functions and extension properties of one name imported from
		// several packages are overloads, as the program's own above.
		for _, f := range fs {
			if !slices.Contains(c.funImports[alias], f) {
				c.funImports[alias] = append(c.funImports[alias], f)
			}
		}
		// A library's top-level property, or extension properties.
		var v *types.VarSym
		var gs []*types.FunSym
		if ext.kotlin && pkg != "" {
			if v = ext.pkgVar(pkg, simple); v != nil {
				c.userVars[alias] = v
			}
			gs = ext.pkgExtProps(pkg, simple)
			for _, g := range gs {
				if !slices.Contains(c.userExtProps[alias], g) {
					c.userExtProps[alias] = append(c.userExtProps[alias], g)
				}
			}
		}
		if cls != nil || ta != nil || len(fs) > 0 || v != nil || len(gs) > 0 {
			continue
		}
		// A.b imports a member of a class or an object, or a nested class.
		if owner := c.importOwner(pkg); owner != nil {
			if n := c.nestedOf(owner, simple); n != nil {
				c.imports[alias] = n
			} else {
				c.memberImports[alias] = memberImport{owner: owner, name: simple, p: imp.P, path: imp.Path}
			}
			continue
		}
		if optionalExpectations[imp.Path] {
			continue // an annotation the JVM has no class of
		}
		if pkg == "kotlin" && kotlinTypes[simple] && alias == simple {
			continue // import kotlin.String: a built-in type, imported by default
		}
		c.errorf(imp.P, "unresolved import: %s", imp.Path)
	}
	// import java.nio.file.StandardOpenOption.* imports a Java class's
	// statics, enum entries too, after the explicit imports, and import
	// kotlinx.coroutines.CoroutineStart.* a program's enum's entries.
	for _, imp := range memberStars {
		owner := c.importOwner(imp.Path)
		owner.Ensure()
		c.starOwners = append(c.starOwners, owner)
		add := func(name string) {
			if _, ok := c.memberImports[name]; !ok {
				c.memberImports[name] = memberImport{owner: owner, name: name, p: imp.P, path: imp.Path + "." + name, star: true}
			}
		}
		if !owner.External {
			for _, name := range owner.Entries {
				add(name)
			}
			continue
		}
		for name := range owner.Fields {
			add(name)
		}
		for name := range owner.Statics {
			add(name)
		}
	}
}

// builtinClasses are the simple names of the classes built into the
// compiler, rather than declared in the standard library, that kotlinc
// resolves before the classes of a file's star imports.
var builtinClasses = map[string]bool{
	"Any": true, "Nothing": true, "Unit": true, "Boolean": true, "Char": true, "Byte": true,
	"Short": true, "Int": true, "Long": true, "Float": true, "Double": true, "String": true,
	"CharSequence": true, "Comparable": true, "Number": true, "Enum": true, "Annotation": true,
	"Throwable": true, "Cloneable": true, "Function": true, "Deprecated": true,
	"DeprecationLevel": true, "ReplaceWith": true, "Suppress": true, "PublishedApi": true,
	"SinceKotlin": true, "DslMarker": true, "ExtensionFunctionType": true, "ParameterName": true,
	"UnsafeVariance": true, "Iterable": true, "MutableIterable": true, "Collection": true,
	"MutableCollection": true, "List": true, "MutableList": true, "Set": true, "MutableSet": true,
	"Map": true, "MutableMap": true, "Iterator": true, "MutableIterator": true,
	"ListIterator": true, "MutableListIterator": true, "Target": true, "Retention": true,
	"Repeatable": true, "MustBeDocumented": true, "AnnotationTarget": true,
	"AnnotationRetention": true, "IntRange": true, "LongRange": true, "CharRange": true,
}

// optionalExpectationPackages are the packages of optionalExpectations,
// which common code can star-import although the JVM has no classes in
// them.
var optionalExpectationPackages = map[string]bool{
	"kotlin.js": true, "kotlin.native": true, "kotlin.native.concurrent": true,
}

// optionalExpectations are the standard library's annotations marked
// @OptionalExpectation that the JVM has no actual class of. As kotlinc
// does, common code can import and use them, and the JVM drops them.
var optionalExpectations = map[string]bool{
	"kotlin.js.JsName":                         true,
	"kotlin.js.JsExport":                       true,
	"kotlin.native.CName":                      true,
	"kotlin.native.ObjCName":                   true,
	"kotlin.native.HiddenFromObjC":             true,
	"kotlin.native.ShouldRefineInSwift":        true,
	"kotlin.native.RefinesInSwift":             true,
	"kotlin.native.concurrent.SharedImmutable": true,
	"kotlin.native.concurrent.ThreadLocal":     true,
}

// importOwner resolves the qualified name of a class in an import, a.b.C
// or a.b.C.Nested, or returns nil.
func (c *Checker) importOwner(name string) *types.ClassSym {
	if name == "" {
		return nil
	}
	parts := strings.Split(name, ".")
	if cls := c.userClassPath(parts); cls != nil {
		return cls
	}
	ext := c.u.ext
	// On the JVM, a nested class's name is its outer class's, $ and its
	// own.
	for i := len(parts) - 1; i >= 0; i-- {
		n := strings.Join(parts[i:], "$")
		if i > 0 {
			n = strings.Join(parts[:i], "/") + "/" + n
		}
		if cls := ext.class(n); cls != nil {
			return cls
		}
	}
	if ext.kotlin {
		for i := len(parts) - 1; i >= 1; i-- {
			cls := ext.pkgClass(strings.Join(parts[:i], "."), parts[i])
			for _, n := range parts[i+1:] {
				if cls == nil {
					break
				}
				cls = c.nestedOf(cls, n)
			}
			if cls != nil {
				return cls
			}
		}
	}
	return nil
}

// nestedOf returns the class nested in cls named name, or nil.
func (c *Checker) nestedOf(cls *types.ClassSym, name string) *types.ClassSym {
	if !cls.External {
		return cls.Nested[name]
	}
	return c.u.ext.class(cls.Name + "$" + name)
}

// findClass resolves a simple class name in Kotlin's order: explicit
// imports, classes in the program, star imports, Kotlin's default imports,
// then java.lang.
func (c *Checker) findClass(name string) *types.ClassSym {
	k := c.findClassifier(name)
	if k.alias != nil {
		return c.aliasClass(k.alias)
	}
	return k.cls
}

// classifier is what a simple name in a type names: a class, or one of
// the program's typealiases.
type classifier struct {
	cls   *types.ClassSym
	alias *types.AliasSym
}

// findClassifier resolves a simple name of a class or a typealias, in
// Kotlin's order: nested in the enclosing classes, explicit imports, the
// file's package, star imports, then the default imports.
func (c *Checker) findClassifier(name string) classifier {
	if c.scope != nil {
		if k := c.scope.findClass(name); k != nil {
			return classifier{cls: k} // a local class
		}
	}
	if k := c.nestedClassifier(name); k != (classifier{}) {
		return k
	}
	if k, ok := c.classCache[name]; ok {
		return k
	}
	k := c.findClassifier1(name)
	c.classCache[name] = k
	return k
}

func (c *Checker) findClassifier1(name string) classifier {
	if cls, ok := c.imports[name]; ok {
		return classifier{cls: cls}
	}
	if a, ok := c.aliasImports[name]; ok {
		return classifier{alias: a}
	}
	if cls, ok := c.pkg.Classes[name]; ok {
		return classifier{cls: cls}
	}
	if a, ok := c.pkg.Aliases[name]; ok {
		return classifier{alias: a}
	}
	for _, p := range c.userStars {
		if cls, ok := p.Classes[name]; ok {
			return classifier{cls: cls}
		}
		if a, ok := p.Aliases[name]; ok {
			return classifier{alias: a}
		}
	}
	for _, owner := range c.starOwners {
		if cls := c.nestedOf(owner, name); cls != nil {
			return classifier{cls: cls}
		}
	}
	if cls := c.extClass(name); cls != nil {
		return classifier{cls: cls}
	}
	return classifier{alias: c.extAlias(name)}
}

// extAlias resolves a simple name of a typealias on the class path that
// isn't one of a class, in the star imports, then the default imports.
func (c *Checker) extAlias(name string) *types.AliasSym {
	ext := c.u.ext
	if kotlinTypes[name] || !ext.kotlin {
		return nil
	}
	for _, pkg := range c.stars {
		if a := ext.pkgAlias(strings.ReplaceAll(pkg, "/", "."), name); a != nil {
			return a
		}
	}
	for _, pkg := range defaultPackages {
		if a := ext.pkgAlias(pkg, name); a != nil {
			return a
		}
	}
	return nil
}

// extClass resolves a simple name of a class on the class path.
func (c *Checker) extClass(name string) *types.ClassSym {
	if kotlinTypes[name] {
		return nil
	}
	ext := c.u.ext
	// A built-in class, as kotlin.Array, comes before the star imports:
	// import java.lang.reflect.* doesn't hide it.
	if cls := types.ArrayByName(name); cls != nil {
		return cls
	}
	if builtinClasses[name] && ext.kotlin {
		for _, pkg := range defaultPackages {
			if cls := ext.pkgClass(pkg, name); cls != nil {
				return cls
			}
		}
	}
	for _, pkg := range c.stars {
		if ext.kotlin {
			if cls := ext.pkgClass(strings.ReplaceAll(pkg, "/", "."), name); cls != nil {
				return cls
			}
		} else if cls := ext.class(qualified(pkg, name)); cls != nil {
			return cls
		}
	}
	if ext.kotlin {
		for _, pkg := range defaultPackages {
			if cls := ext.pkgClass(pkg, name); cls != nil {
				return cls
			}
		}
	}
	return ext.class("java/lang/" + name)
}

// kotlinFuns returns the Kotlin top-level functions a simple name can
// call, in Kotlin's order: explicit imports, star imports, then default
// imports. ext selects extension functions, or the others.
func (c *Checker) kotlinFuns(name string, ext bool) []*types.FunSym {
	k := funKey{name, ext}
	if fs, ok := c.funCache[k]; ok {
		return fs
	}
	fs := c.kotlinFuns1(name, ext)
	c.funCache[k] = fs
	return fs
}

func (c *Checker) kotlinFuns1(name string, ext bool) []*types.FunSym {
	for _, fs := range c.kotlinFunLevels(name, ext) {
		if len(fs) > 0 {
			return fs
		}
	}
	return nil
}

// kotlinFunLevels returns the class path's functions with a name, by the
// level of the imports that bring them, highest first: explicit imports,
// star imports and the default imports.
func (c *Checker) kotlinFunLevels(name string, ext bool) [][]*types.FunSym {
	keep := func(fs []*types.FunSym) []*types.FunSym {
		var out []*types.FunSym
		for _, f := range fs {
			if f.Ext == ext {
				out = append(out, f)
			}
		}
		return out
	}
	levels := [][]*types.FunSym{keep(c.funImports[name])}
	ext1 := c.u.ext
	if !ext1.kotlin {
		return levels
	}
	var stars, defaults []*types.FunSym
	for _, pkg := range c.stars {
		stars = append(stars, keep(ext1.pkgFuns(strings.ReplaceAll(pkg, "/", "."), name))...)
		if c.file.SuppressInvisible {
			// ktor's recoverStackTrace(e, cont) of kotlinx.coroutines.internal.*
			// under @Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE").
			stars = append(stars, keep(ext1.pkgInternalFuns(strings.ReplaceAll(pkg, "/", "."), name))...)
		}
	}
	for _, pkg := range defaultPackages {
		defaults = append(defaults, keep(ext1.pkgFuns(pkg, name))...)
	}
	return append(levels, stars, defaults)
}

// extCallees returns the extension functions with a name that a receiver
// of type rt may call. The program's come in the order of topFuns, and
// before the class path's default and star imports: the first of these
// scopes that has an extension for the receiver hides the others.
func (c *Checker) extCallees(name string, rt types.Type) []callee {
	return c.extCalleesFor(name, rt, nil)
}

// calleeFuns returns the functions of cs.
func calleeFuns(cs []callee) []*types.FunSym {
	fs := make([]*types.FunSym, len(cs))
	for i, cl := range cs {
		fs[i] = cl.f
	}
	return fs
}

// extCalleesFor is extCallees for call x: as in Kotlin, a level of scope
// whose extensions can't take x's arguments gives way to the next one,
// as wire's internal C.use(close, block) to the standard library's
// T.use(block). If no level's can, the first level's are returned, for
// their errors.
func (c *Checker) extCalleesFor(name string, rt types.Type, x *syntax.Call) []callee {
	var unfit []callee
	fits := func(fs []*types.FunSym) []callee {
		var out []callee
		for _, f := range fs {
			if f.Ext && c.recvFits(rt, f.Recv) {
				out = append(out, callee{f: f, tparams: f.TypeParams, ext: rt})
			}
		}
		if slices.ContainsFunc(out, func(cl callee) bool { return c.hiddenFun(cl.f) }) {
			// Other files' private extensions give way to the visible
			// ones: two files' private Modifier.preview().
			all := out
			out = slices.DeleteFunc(slices.Clone(out), func(cl callee) bool { return c.hiddenFun(cl.f) })
			if len(out) == 0 {
				if unfit == nil {
					unfit = all
				}
				return nil
			}
		}
		if x != nil && len(out) > 0 && !slices.ContainsFunc(out, func(cl callee) bool { return c.recvTakes(rt, cl.f.Recv) && c.mayTake(cl.f, x) }) {
			if unfit == nil {
				unfit = out
			}
			return nil
		}
		return out
	}
	// typedFits is fits for the extensions declared in classes, which
	// give way to the top-level ones when the arguments' types don't fit
	// either: a class's Prefs.get(key, gson: Gson) to an imported
	// Prefs.get(key, default: T) for get(key, 0).
	var ts []types.Type
	typedFits := func(fs []*types.FunSym) []callee {
		out := fits(fs)
		if x == nil || len(out) == 0 {
			return out
		}
		if ts == nil {
			ts = c.argTypes(x)
		}
		if !c.takeTypes(calleeFuns(out), x, ts) {
			if unfit == nil {
				unfit = out
			}
			return nil
		}
		return out
	}
	if name == "iterator" && rt.NonNull().IsArray() && c.u.ext.kotlin {
		// An array's iterator(), a member to Kotlin: a static function of
		// the standard library on the JVM.
		if f := c.arrayIterator(rt.NonNull()); f != nil {
			return []callee{{f: f, tparams: f.TypeParams, ext: rt}}
		}
	}
	// Local extension functions in scope come first.
	if c.scope != nil {
		if out := fits(c.localExts(name)); len(out) > 0 {
			return out
		}
	}
	if c.cls != nil {
		// Extensions declared in the enclosing class come first: their
		// dispatch receiver is the innermost implicit receiver after the
		// lambdas'.
		if out := typedFits(memberExtMethods(c.cls, name)); len(out) > 0 {
			c.captureThis()
			for j := range out {
				out[j].recv = c.memberSubst(c.cls.This(), out[j].f.Owner) // a superclass's C
			}
			return out
		}
		// Then those declared in the objects in scope: companions.
		for _, obj := range c.scopeObjects() {
			if out := typedFits(memberExtMethods(obj, name)); len(out) > 0 {
				return dispatchOn(out, obj)
			}
		}
	}
	// Those declared in the classes of lambdas' implicit receivers, the
	// innermost first, with that receiver as their dispatch receiver:
	// SelectBuilder's SelectClause1<Q>.invoke in select { d.onAwait { } }.
	for i := len(c.recvs) - 1; i >= 0; i-- {
		r := &c.recvs[i]
		rt := c.scope.narrowed(r.sym).NonNull()
		if cls := c.classOf(rt); cls != nil && (cls.External || c.u.memberExts) {
			if out := fits(memberExtMethods(cls, name)); len(out) > 0 {
				for j := range out {
					out[j].dispatchRecv = r
					out[j].recv = c.memberSubst(rt, out[j].f.Owner) // Builder<R>'s R
				}
				return out
			}
		}
	}
	if mi, ok := c.memberImports[name]; ok && mi.owner.Object {
		if out := fits(memberExtMethods(mi.owner, mi.name)); len(out) > 0 {
			return dispatchOn(out, mi.owner)
		}
	}
	if out := fits(c.userFuns[name]); len(out) > 0 {
		return out
	}
	// Explicitly imported ones come before the package's, if one fits.
	if out := fits(c.funImports[name]); len(out) > 0 {
		return out
	}
	var fs []*types.FunSym
	for _, p := range c.userStars {
		fs = append(fs, p.Funs[name]...)
	}
	if out := fits(c.pkg.Funs[name]); len(out) > 0 {
		// They give way to the star-imported ones if only those take the
		// arguments' types: kotlinx.coroutines.time's
		// Flow<T>.debounce(Duration) to kotlinx.coroutines.flow's
		// Flow<T>.debounce(Long).
		if stars := fits(fs); x != nil && len(stars) > 0 {
			if ts == nil {
				ts = c.argTypes(x)
			}
			if !c.takeTypes(calleeFuns(out), x, ts) && c.takeTypes(calleeFuns(stars), x, ts) {
				return stars
			}
		}
		return out
	}
	if out := fits(fs); len(out) > 0 {
		return out
	}
	// The first level of imports with an extension for the receiver.
	k := funKey{name, true}
	levels, ok := c.levelCache[k]
	if !ok {
		levels = c.kotlinFunLevels(name, true)
		c.levelCache[k] = levels
	}
	for _, fs := range levels {
		if out := fits(fs); len(out) > 0 {
			return out
		}
	}
	return unfit
}

// dispatchOn makes obj the dispatch receiver of member extensions.
func dispatchOn(cands []callee, obj *types.ClassSym) []callee {
	for i := range cands {
		cands[i].dispatch = obj
	}
	return cands
}

// recvFits reports whether an extension declared for receivers of type r
// may be called on a receiver of type rt, judging by its class: inference
// checks type arguments and bounds.
func (c *Checker) recvFits(rt, r types.Type) bool {
	switch {
	case r.K == types.Invalid:
		return false
	case types.MayBeNull(rt) && !rt.P && !types.MayBeNull(r):
		return false
	case r.IsParam():
		return true
	case rt.K == types.Null:
		return true // null fits a nullable receiver: null.f() for Id?.f()
	}
	rn := r.NonNull()
	switch rn.K {
	case types.Any:
		return true
	case types.Class:
		_, ok := c.asSuper(rt, rn.C)
		return ok
	}
	return rt.NonNull().K == rn.K
}

// recvTakes reports whether an extension whose receiver type is r, which
// recvFits rt, takes a receiver of type rt: a receiver type with type
// arguments but no type parameters must be a supertype, so a List<T> has
// no List<Module>.plus, and a type parameter's bound must be one, so
// ktor's T.use for T : Closeable? gives way to the standard library's
// for a kotlinx.io Source, which is only AutoCloseable.
func (c *Checker) recvTakes(rt, r types.Type) bool {
	if r.IsParam() {
		b := r.Param().Bound()
		if b.K != types.Class || b.HasParams() || b.HasVars() || rt.HasVars() {
			return true
		}
		return c.subtype(rt.NonNull(), b.NonNull())
	}
	rn := r.NonNull()
	if rn.K != types.Class || rn.A == nil || rn.HasParams() || rt.HasVars() {
		return true
	}
	return c.subtype(rt.NonNull(), rn)
}

// extProp checks x.name as an extension property of a receiver of type rt.
// The program's extension properties come first, in the order of
// extCallees, then the class path's.
func (c *Checker) extProp(x *syntax.Member, rt types.Type) (types.Type, bool) {
	var best *types.FunSym
	var bestSub types.Subst // a superclass's type parameters, for a member's
	sub := func(g *types.FunSym) types.Subst { return nil }
	fits := func(gs []*types.FunSym) {
		for _, g := range gs {
			ensureFun(g)
			s := sub(g)
			if c.recvFits(rt, s.Apply(g.Recv)) && (best == nil || c.subtype(s.Apply(g.Recv), bestSub.Apply(best.Recv))) {
				best, bestSub = g, s
			}
		}
	}
	// Those declared in the class the code is in come first, then those of
	// the objects in scope, and imported ones of objects.
	var dispatch *types.ClassSym
	if c.cls != nil {
		sub = func(g *types.FunSym) types.Subst { return c.memberSubst(c.cls.This(), g.Owner) }
		if fits(memberExtProps(c.cls, x.Name)); best != nil {
			c.captureThis()
		}
		sub = func(g *types.FunSym) types.Subst { return nil }
		for _, obj := range c.scopeObjects() {
			if best == nil {
				fits(memberExtProps(obj, x.Name))
				dispatch = obj
			}
		}
	}
	// Then those declared in the classes of the implicit receivers, the
	// innermost first, with that receiver as their dispatch receiver:
	// vico's MeasuringContext's Dp.pixels in with(context) { }.
	var dispatchRecv *implicitRecv
	for i := len(c.recvs) - 1; i >= 0 && best == nil; i-- {
		r := &c.recvs[i]
		t := c.scope.narrowed(r.sym).NonNull()
		if cls := c.classOf(t); cls != nil && (cls.External || c.u.memberExts) {
			sub = func(g *types.FunSym) types.Subst { return c.memberSubst(t, g.Owner) }
			if fits(memberExtProps(cls, x.Name)); best != nil {
				dispatch, dispatchRecv = nil, r
			}
			sub = func(g *types.FunSym) types.Subst { return nil }
		}
	}
	if mi, ok := c.memberImports[x.Name]; ok && mi.owner.Object && best == nil {
		fits(memberExtProps(mi.owner, mi.name))
		dispatch = mi.owner
	}
	if best == nil {
		dispatch = nil
		fits(c.userExtProps[x.Name])
	}
	if best == nil {
		fits(c.pkg.ExtProps[x.Name])
	}
	for _, p := range c.userStars {
		if best == nil {
			fits(p.ExtProps[x.Name])
		}
	}
	if best == nil && c.u.ext.kotlin {
		pkgs := make([]string, 0, len(c.stars)+len(defaultPackages))
		for _, s := range c.stars {
			pkgs = append(pkgs, strings.ReplaceAll(s, "/", "."))
		}
		for _, pkg := range append(pkgs, defaultPackages...) {
			fits(c.u.ext.pkgExtProps(pkg, x.Name))
		}
	}
	if best == nil {
		return types.InvalidType, false
	}
	x.Getter, x.Dispatch = best, dispatch
	if dispatchRecv != nil {
		th := recvThis(dispatchRecv, x.P)
		c.value(th, noWant)
		x.DispatchThis = th
	}
	if len(best.TypeParams) == 0 {
		if rt.HasVars() {
			// A builder's receiver, whose variables the receiver type
			// constrains: see stubInference.
			c.subtype(rt, bestSub.Apply(best.Recv))
		}
		return bestSub.Apply(best.Ret), true
	}
	sys := newSystem(c)
	vars := sys.fresh(best.TypeParams, bestSub, x.P)
	sys.sub(rt, vars.ApplyIn(best.Recv))
	sys.finish()
	if hasReified(best) {
		c.reifiedProp(x, best, vars, sys)
	}
	return sys.final(vars.Apply(best.Ret)), true
}

// lookupClass resolves a simple or qualified class name.
func (c *Checker) lookupClass(name string) *types.ClassSym {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		parts := strings.Split(name, ".")
		if cls := c.dottedClass(parts); cls != nil {
			return cls
		}
		if len(parts) == 2 && parts[1] == "Companion" && kotlinTypes[parts[0]] {
			if obj := c.builtinCompanion(parts[0]); obj != nil {
				return obj // Int.Companion, as an extension's receiver
			}
		}
		if a := c.lookupAlias(name); a != nil {
			return c.aliasClass(a) // a.b.Alias(x), Outer.Alias.member
		}
		if first := c.findClass(parts[0]); first != nil && first.External {
			// A nested type of a Kotlin collection type: Map.Entry.
			if first.KotlinName != "" {
				if _, ok := collectionJVM[first.KotlinName+"."+strings.Join(parts[1:], ".")]; ok {
					return c.u.ext.kclass(first.KotlinName + "." + strings.Join(parts[1:], "."))
				}
			}
			// A nested class of a class on the class path: Thread.State.
			if cls := c.u.ext.class(first.Name + "$" + strings.Join(parts[1:], "$")); cls != nil {
				return cls
			}
		}
		if cls := c.u.ext.class(strings.ReplaceAll(name, ".", "/")); cls != nil {
			return cls
		}
		// A nested class of a class path class by its qualified name:
		// java.util.AbstractMap.SimpleEntry.
		for j := len(parts) - 1; j >= 2; j-- {
			if cls := c.u.ext.class(strings.Join(parts[:j], "/") + "$" + strings.Join(parts[j:], "$")); cls != nil {
				return cls
			}
		}
		if !c.u.ext.kotlin {
			return nil
		}
		return c.u.ext.pkgClass(name[:i], name[i+1:])
	}
	return c.findClass(name)
}

// isVar reports whether name refers to a variable, without inferring its type.
func (c *Checker) isVar(name string) bool {
	if _, _, ok := c.scope.lookup(name); ok {
		return true
	}
	if c.cls != nil && c.userProp(c.cls, name) != nil {
		return true
	}
	return c.topVar(name) != nil || c.valueImport(name)
}

// userClass returns a class of the program by its package and name.
func (c *Checker) userClass(pkg, name string) *types.ClassSym {
	if p := c.u.Pkgs[pkg]; p != nil && pkg != "" {
		return p.Classes[name]
	}
	return nil
}

// packageRef returns the package of the program that x names, as a.b in
// a.b.f(), or nil.
func (c *Checker) packageRef(x syntax.Expr) *Package {
	root := x
	for m, ok := root.(*syntax.Member); ok; m, ok = root.(*syntax.Member) {
		root = m.X
	}
	if id, ok := root.(*syntax.Ident); !ok || !c.u.prefixes[id.Name] {
		return nil
	}
	name := c.qualifiedName(x)
	if name == "" || !strings.Contains(name, ".") && c.findClass(name) != nil {
		return nil
	}
	if p := c.u.Pkgs[name]; p != nil && name != "" {
		return p
	}
	return nil
}

// qualifiedName returns the dotted name that x spells, a.b.c, if it is a
// chain of names whose first isn't a variable, or "".
func (c *Checker) qualifiedName(x syntax.Expr) string {
	switch x := x.(type) {
	case *syntax.Ident:
		if c.isVar(x.Name) {
			return ""
		}
		return x.Name
	case *syntax.Member:
		if x.Safe {
			return ""
		}
		if q := c.qualifiedName(x.X); q != "" {
			return q + "." + x.Name
		}
	}
	return ""
}

// libraryPackageFuns returns the top-level functions of a library's
// package that a qualified name a.b.f names, if a.b is that package rather
// than a value or a class.
func (c *Checker) libraryPackageFuns(fn *syntax.Member) []*types.FunSym {
	ext := c.u.ext
	if !ext.kotlin {
		return nil
	}
	name := c.qualifiedName(fn.X)
	if name == "" || !ext.isKotlinPackage(name) {
		return nil
	}
	if root, _, _ := strings.Cut(name, "."); c.findClass(root) != nil || c.u.Pkgs[root] != nil {
		return nil
	}
	fs := ext.pkgFuns(name, fn.Name)
	if c.file.SuppressInvisible {
		// kotlin.coroutines.jvm.internal.probeCoroutineCreated(completion)
		// under @Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE").
		fs = append(slices.Clip(fs), ext.pkgInternalFuns(name, fn.Name)...)
	}
	return nonExt(fs)
}

// libraryPackageVar returns the top-level property of a library's package
// that a qualified name a.b.x names, as libraryPackageFuns its functions.
func (c *Checker) libraryPackageVar(x *syntax.Member) *types.VarSym {
	ext := c.u.ext
	if !ext.kotlin {
		return nil
	}
	name := c.qualifiedName(x.X)
	if name == "" || !ext.isKotlinPackage(name) {
		return nil
	}
	if root, _, _ := strings.Cut(name, "."); c.findClass(root) != nil || c.u.Pkgs[root] != nil {
		return nil
	}
	return ext.pkgVar(name, x.Name)
}

// classRef returns the class that x names, if x is a class name rather
// than a value: Math in Math.max(a, b), or java.util.Random.
func (c *Checker) classRef(x syntax.Expr) *types.ClassSym {
	switch x := x.(type) {
	case *syntax.Ident:
		if c.isVar(x.Name) {
			return nil
		}
		return c.findClass(x.Name)
	case *syntax.Member:
		var parts []string
		var e syntax.Expr = x
		for {
			if m, ok := e.(*syntax.Member); ok && !m.Safe {
				parts = append(parts, m.Name)
				e = m.X
				continue
			}
			break
		}
		id, ok := e.(*syntax.Ident)
		if !ok || c.isVar(id.Name) {
			return nil
		}
		path := id.Name
		names := []string{id.Name}
		for i := len(parts) - 1; i >= 0; i-- {
			path += "/" + parts[i]
			names = append(names, parts[i])
		}
		if cls := c.dottedClass(names); cls != nil {
			return cls
		}
		if a := c.lookupAlias(strings.Join(names, ".")); a != nil {
			return c.aliasClass(a) // a.b.Alias, Outer.Alias
		}
		if first := c.findClass(id.Name); first != nil && first.External {
			// A nested class of a class on the class path: Thread.State.
			name := first.Name
			for i := len(parts) - 1; i >= 0; i-- {
				name += "$" + parts[i]
			}
			if cls := c.u.ext.class(name); cls != nil {
				return cls
			}
		}
		if c.u.isPackage(strings.ReplaceAll(path, "/", ".")) {
			return nil
		}
		if cls := c.u.ext.class(path); cls != nil {
			return cls
		}
		if pkg := strings.Join(names[:len(names)-1], "."); c.u.ext.kotlin {
			// A library's typealias of a class: kotlin.NoSuchElementException.
			if cls := c.u.ext.pkgClass(pkg, names[len(names)-1]); cls != nil {
				return cls
			}
		}
		// java.util.AbstractMap.SimpleEntry: a nested class by its
		// qualified name.
		for j := len(names) - 1; j >= 2; j-- {
			if cls := c.u.ext.class(strings.Join(names[:j], "/") + "$" + strings.Join(names[j:], "$")); cls != nil {
				return cls
			}
		}
	}
	return nil
}

// usable loads a Java class and reports whether kc can use it.
func (c *Checker) usable(cls *types.ClassSym, p syntax.Pos) bool {
	if !cls.External {
		return true
	}
	cls.Ensure()
	switch {
	case cls.Missing:
		c.errorf(p, "cannot read class %s", cls.Name)
	case cls.Private && !(cls.Internal && c.file.SuppressInvisible):
		// SafeCollector : ContinuationImpl under @Suppress("INVISIBLE_REFERENCE").
		c.errorf(p, "class %s is not public", cls.DisplayName())
	default:
		return true
	}
	return false
}

// boundWith returns the first bound for which has holds of a type
// parameter t with several bounds, whose members are those of all of
// them: x.copy() for T : Throwable, T : Copyable<T>. Otherwise, it
// returns t.
func (c *Checker) boundWith(t types.Type, has func(*types.ClassSym) bool) types.Type {
	if t.K != types.TypeParam && t.K != types.TypeVar || len(t.Param().Bounds) < 2 {
		return t
	}
	for _, b := range t.Param().Bounds {
		if cls := c.classOf(b.NonNull()); cls != nil && has(cls) {
			return b.NonNull()
		}
	}
	return t
}

// classOf returns the class whose members a receiver of type t has.
func (c *Checker) classOf(t types.Type) *types.ClassSym {
	switch t.K {
	case types.Class:
		return t.C
	case types.Any:
		return c.u.ext.class("java/lang/Object")
	case types.Unit:
		return c.u.ext.class("kotlin/Unit") // the object's
	case types.TypeParam, types.TypeVar:
		return c.classOf(t.Param().Bound().NonNull())
	}
	return nil
}

// jvmClass returns the Java class of values of type t, boxed for
// primitives, for subtype checks against Java classes.
func (c *Checker) jvmClass(t types.Type) *types.ClassSym {
	switch t.K {
	case types.String:
		return c.u.ext.class("java/lang/String")
	case types.Class:
		return t.C
	}
	if box, ok := boxes[t.K]; ok {
		return c.u.ext.class(box)
	}
	return nil
}

// javaMethods collects the instance methods named name of cls and its
// supertypes. An override hides the methods it overrides.
func (c *Checker) javaMethods(cls *types.ClassSym, name string) []*types.FunSym {
	k := methodsKey{cls, name}
	if ms, ok := c.u.ext.methods.Load(k); ok {
		return ms.([]*types.FunSym)
	}
	ms := javaMethods(cls, name)
	if len(ms) == 0 && (cls.Interface || cls.Array) && anyMember[name] {
		// A Kotlin interface's supertypes, as its metadata gives them, end
		// without Any: a function value's f.equals(g) is Object's. So do
		// an array's: a.toString().
		if object := c.u.ext.class("java/lang/Object"); object != nil {
			ms = javaMethods(object, name)
		}
	}
	for _, f := range ms {
		c.inheritExtDefaults(f)
	}
	ms = c.intersectionDefaults(cls, ms)
	c.u.ext.methods.Store(k, ms)
	return ms
}

// intersectionDefaults replaces each method of ms without default values
// that also implements, in cls, an interface's method with default values
// with a copy that takes them, as Kotlin's intersection override does: B's
// f of class B : A(), I, with A's f(x: String) and I's f(x: String = "1").
// Callers leave arguments out through the interface method's name$default
// method (see DefaultsOf), as kotlinc does.
func (c *Checker) intersectionDefaults(cls *types.ClassSym, ms []*types.FunSym) []*types.FunSym {
	var out []*types.FunSym
	for i, f := range ms {
		if f.Static || len(f.Params) == 0 || hasDefaults(f) {
			continue
		}
		var g *types.FunSym
		this := cls.This()
		cls.Supertypes(func(s *types.ClassSym) bool {
			if s == f.Owner || !s.Interface {
				return true
			}
			for _, h := range s.Methods[f.Name] {
				if !h.Static && !h.Private && hasDefaults(h) && c.sameParams(this, f, h) {
					g = h
					return false
				}
			}
			return true
		})
		if g == nil {
			continue
		}
		if out == nil {
			out = slices.Clone(ms)
		}
		fo := *f
		fo.Params = make([]*types.VarSym, len(f.Params))
		for j, p := range f.Params {
			q := *p
			q.Default = q.Default || g.Params[j].Default
			fo.Params[j] = &q
		}
		fo.DefaultsOf = g
		if g.DefaultsOf != nil {
			fo.DefaultsOf = g.DefaultsOf
		}
		if !f.Resolved && f.Infer != nil {
			// The result's type, when f's is inferred.
			fo.Infer = func() {
				ensureFun(f)
				fo.Ret, fo.Resolved = f.Ret, true
			}
		}
		out[i] = &fo
	}
	if out == nil {
		return ms
	}
	return out
}

// inheritExtDefaults gives a method of a class-path class that overrides a
// function with default values those values, as in Kotlin: an override's
// metadata declares none, and callers leave arguments out through the
// overridden function's name$default method, as for the program's
// overrides (see DefaultsOf).
func (c *Checker) inheritExtDefaults(f *types.FunSym) {
	if f.Decl != nil || f.Owner == nil || !f.Owner.External || f.Static || f.Name == "<init>" || len(f.Params) == 0 {
		return
	}
	once, _ := c.u.ext.defaults.LoadOrStore(f, new(sync.Once))
	once.(*sync.Once).Do(func() {
		if declaresDefaults(f) {
			return
		}
		for _, g := range c.overridden(f.Owner, f) {
			c.inheritExtDefaults(g)
			if g.Decl == nil && declaresDefaults(g) {
				for i, p := range f.Params {
					p.Default = g.Params[i].Default
				}
				f.DefaultsOf = g
				return
			}
		}
	})
}

// declaresDefaults reports whether a class-path function declares
// default values, rather than inheriting them.
func declaresDefaults(f *types.FunSym) bool {
	if f.DefaultsOf != nil {
		return false
	}
	for _, p := range f.Params {
		if p.Default {
			return true
		}
	}
	return false
}

func javaMethods(cls *types.ClassSym, name string) []*types.FunSym {
	var out []*types.FunSym
	seen := map[string]bool{}
	cls.Supertypes(func(s *types.ClassSym) bool {
		var own []string
		for _, f := range s.Methods[name] {
			params := paramKey(f)
			if seen[params] || javaRemoveAt(f) {
				continue
			}
			// The program's overloads in one class are distinct even when
			// their erasures are the same, as f(List<A>) and f(List<B>)
			// renamed by @JvmName. A class file's are distinct by
			// descriptor, but for covariant return bridges.
			if f.Decl == nil {
				seen[params] = true
			} else {
				own = append(own, params)
			}
			out = append(out, f)
		}
		for _, p := range own {
			seen[p] = true
		}
		return true
	})
	return out
}

// javaRemoveAt reports whether f is a Java List's remove(int), which
// Kotlin, which sees every Java List as a MutableList, calls removeAt:
// remove(x) on a list of Ints removes the element x.
func javaRemoveAt(f *types.FunSym) bool {
	return f.Name == "remove" && f.Owner != nil && f.Owner.External && f.Owner.KotlinName == "" &&
		len(f.Params) == 1 && f.Params[0].Type.Flex() == types.IntType && strings.HasPrefix(f.Desc, "(I)") &&
		f.Owner.IsSubclassOf(listInterface(f.Owner))
}

// listInterface returns the java.util.List class among cls's supertypes,
// or nil.
func listInterface(cls *types.ClassSym) *types.ClassSym {
	var l *types.ClassSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s.Name == "java/util/List" {
			l = s
		}
		return l == nil
	})
	return l
}

// javaStatics collects the static methods named name of cls and its
// superclasses.
func javaStatics(cls *types.ClassSym, name string) []*types.FunSym {
	var out []*types.FunSym
	hidden := map[string]bool{} // the parameters of a subclass's
	for s := cls; s != nil; s = s.Super {
		s.Ensure()
		for _, f := range s.Statics[name] {
			// A subclass's static method hides its superclass's of the
			// same parameters: GoogleApiAvailability.getInstance().
			params, _, _ := strings.Cut(f.Desc, ")")
			if f.Desc != "" && hidden[params] {
				continue
			}
			hidden[params] = true
			out = append(out, f)
		}
	}
	return out
}

func javaField(cls *types.ClassSym, name string) *types.VarSym {
	var f *types.VarSym
	cls.Supertypes(func(s *types.ClassSym) bool {
		f = s.Fields[name]
		return f == nil
	})
	return f
}

func (c *Checker) isCharSequence(cls *types.ClassSym) bool {
	return c.isSubclassOf(cls, "java/lang/CharSequence")
}

// charSequenceGet returns Kotlin's CharSequence.get(index), which is
// charAt(index) on the JVM.
func (c *Checker) charSequenceGet() *types.FunSym {
	return &types.FunSym{Name: "get", JVMName: "charAt", Owner: c.u.ext.class(charSequence), Desc: "(I)C", Operator: true, Resolved: true,
		Params: []*types.VarSym{{Name: "index", Type: types.IntType, Kind: types.Param}}, Ret: types.CharType}
}

func (c *Checker) isSubclassOf(cls *types.ClassSym, name string) bool {
	d := c.u.ext.class(name)
	return d != nil && cls.IsSubclassOf(d)
}

// hasSize reports whether Kotlin sees size() of cls as the property size,
// as it does for collections and maps.
func (c *Checker) hasSize(cls *types.ClassSym) bool {
	return c.isSubclassOf(cls, "java/util/Collection") || c.isSubclassOf(cls, "java/util/Map")
}

// dedupe drops methods that a method earlier in the list overrides:
// those with the same parameter types once the receiver's type
// arguments are substituted, such as Comparable<BigInteger>.compareTo(T)
// after BigInteger.compareTo(BigInteger).
func (c *Checker) dedupe(cands []callee) []callee {
	generic := func(cl callee) bool {
		if len(cl.tparams) > 0 {
			return true
		}
		for _, p := range cl.f.Params {
			if p.Type.HasParams() {
				return true
			}
		}
		return false
	}
	any := false
	for _, cl := range cands {
		any = any || generic(cl)
	}
	if !any {
		return cands // erased descriptors already tell them apart
	}
	out := cands[:0:0]
outer:
	for _, cl := range cands {
		for _, prev := range out {
			if len(prev.f.Params) != len(cl.f.Params) || len(prev.tparams) != len(cl.tparams) || !generic(prev) && !generic(cl) {
				continue
			}
			same := true
			for i, p := range cl.f.Params {
				if !c.sameType(prev.recv.ApplyIn(prev.f.Params[i].Type), cl.recv.ApplyIn(p.Type)) {
					same = false
					break
				}
			}
			if same {
				continue outer
			}
		}
		out = append(out, cl)
	}
	return out
}

// companionInvokes reports whether an invoke operator of companion
// object comp, a member or an extension, may take x's arguments.
func (c *Checker) companionInvokes(comp *types.ClassSym, x *syntax.Call) bool {
	if c.anyMayTake(c.javaMethods(comp, "invoke"), x) {
		return true
	}
	for _, cl := range c.extCalleesFor("invoke", types.ClassType(comp), x) {
		if cl.f.Operator && c.mayTake(cl.f, x) {
			return true
		}
	}
	return false
}

// construct checks a constructor call of a user or Java class.
func (c *Checker) construct(x *syntax.Call, cls *types.ClassSym, p syntax.Pos, want types.Type, sink *system) types.Type {
	if cls.Array {
		return c.arrayConstruct(x, cls, want, sink)
	}
	if cls.Object {
		c.errorf(p, "expression '%s' of type '%s' cannot be invoked as a function", cls.DisplayName(), cls.DisplayName())
		c.args(x, nil)
		return types.InvalidType
	}
	if cls.Entries != nil && !cls.External {
		c.errorf(p, "enum types cannot be instantiated")
		c.args(x, nil)
		return types.InvalidType
	}
	if cls.Annotation {
		return c.annotationConstruct(x, cls, p, want, sink)
	}
	if cls.Inner && x.Outer == nil {
		outer, ot := c.implicitOuter(cls.Outer, p)
		if outer == nil {
			c.errorf(p, "constructor of inner class %s can be called only with receiver of containing class", cls.DisplayName())
			c.args(x, nil)
			return types.InvalidType
		}
		return c.innerConstruct(x, cls, outer, ot, p, want, sink)
	}
	if cls.Fun && len(x.Args) == 1 && c.sam(cls) != nil {
		return c.samConstructor(x, cls, want, sink) // a fun interface of the program
	}
	if !cls.External {
		ctors := c.visibleCtors(cls)
		if comp := cls.Companion; comp != nil && !c.anyMayTake(ctors, x) && c.companionInvokes(comp, x) {
			// The companion's invoke operator, as kotlinc calls it when
			// no constructor it sees takes the arguments: A(x) is
			// A.Companion.invoke(x), a member or an extension, also for
			// an interface's.
			m := &syntax.Member{X: objectIdent(comp, p), Name: "invoke"}
			m.P = p
			x.Fun = m
			return c.call(x, want, sink)
		}
		if cls.Abstract {
			kind := "an abstract class"
			if cls.Interface {
				kind = "an interface"
			}
			c.errorf(p, "cannot create an instance of %s: %s", kind, cls.DisplayName())
			c.args(x, nil)
			return types.InvalidType
		}
		// Its arguments may have a try: codegen evaluates them before
		// the object is created, as kotlinc does.
		x.Ctor = cls
		var cands []callee
		for _, f := range ctors {
			cands = append(cands, callee{f: f, tparams: cls.TypeParams[:len(cls.TypeParams)-cls.OuterParams], recv: c.outerSubst})
		}
		switch {
		case len(cands) == 0 && len(userCtors(cls)) > 0:
			c.errorf(p, "cannot access 'constructor': it is private in '%s'", cls.DisplayName())
			c.args(x, nil)
			return types.InvalidType
		case len(cands) == 0:
			c.errorf(p, "cannot create an instance of %s", cls.DisplayName())
			c.args(x, nil)
			return types.InvalidType
		}
		if cls.Local {
			defer c.localConstructed(cls)
		}
		return c.resolve(x, cands, want, sink)
	}
	if !c.usable(cls, p) {
		c.args(x, nil)
		return types.InvalidType
	}
	if len(x.Args) == 1 && c.sam(cls) != nil && (cls.Fun || cls.KotlinName == "") {
		return c.samConstructor(x, cls, want, sink)
	}
	var cands []callee
	var ctors []*types.FunSym
	if !cls.Abstract {
		for _, f := range cls.Ctors {
			if !f.Protected {
				ctors = append(ctors, f)
				cands = append(cands, callee{f: f, tparams: append(cls.TypeParams[:len(cls.TypeParams):len(cls.TypeParams)], f.TypeParams...)})
			}
		}
	}
	if comp := cls.Companion; comp != nil && !c.anyMayTake(ctors, x) && c.companionInvokes(comp, x) {
		// A library's companion invoke operator: TypeVariableName("T").
		m := &syntax.Member{X: objectIdent(comp, p), Name: "invoke"}
		m.P = p
		x.Fun = m
		return c.call(x, want, sink)
	}
	if len(cands) == 0 {
		c.errorf(p, "cannot create an instance of %s", cls.DisplayName())
		c.args(x, nil)
		return types.InvalidType
	}
	x.Ctor = cls
	return c.resolve(x, cands, want, sink)
}

// samConstructor checks Runnable { ... }: a lambda converted to a Java
// interface named explicitly, or Runnable(f), a function value converted
// as one passed for the interface is (see samValue).
func (c *Checker) samConstructor(x *syntax.Call, cls *types.ClassSym, want types.Type, sink *system) types.Type {
	x.SAM = true
	t := cls.This()
	f := &types.FunSym{Name: cls.DisplayName(), Owner: cls, Ret: t, Resolved: true,
		Params: []*types.VarSym{{Name: "function", Type: t, Kind: types.Param}}}
	rt := c.resolve(x, []callee{{f: f, tparams: cls.TypeParams}}, want, sink)
	if _, ok := x.Args[0].(*syntax.Lambda); !ok && rt.K != types.Invalid {
		// It takes the method's function type, not the interface.
		if ch, ok := x.Args[0].(*syntax.Checked); !ok || ch.SAM == nil {
			_, ft := c.samTarget(rt)
			c.errorf(x.Args[0].Pos(), "type mismatch: inferred type is %s but %s was expected", x.Args[0].Type(), ft)
			return types.InvalidType
		}
	}
	return rt
}

// samCtorRef returns the constructor that ::KRunnable names, for fun
// interface cls: a function of the type of its method, whose call is
// KRunnable(function).
func (c *Checker) samCtorRef(cls *types.ClassSym) *types.FunSym {
	if !cls.Fun || c.sam(cls) == nil {
		return nil
	}
	t := cls.This()
	_, ft := c.samTarget(t)
	return &types.FunSym{Name: cls.DisplayName(), Owner: cls, Ret: t, Resolved: true, TypeParams: cls.TypeParams,
		Params: []*types.VarSym{{Name: "function", Type: ft, Kind: types.Param}}}
}

// staticCall checks a call such as Math.max(a, b).
func (c *Checker) staticCall(x *syntax.Call, fn *syntax.Member, cls *types.ClassSym, want types.Type, sink *system) types.Type {
	fn.Static = cls
	fn.T = types.ClassType(cls)
	if !c.usable(cls, fn.X.Pos()) {
		c.args(x, nil)
		return types.InvalidType
	}
	ms := javaStatics(cls, fn.Name)
	if len(ms) == 0 && c.hasStatic(cls, fn.Name) {
		// A static field's value, invoked: E.ENTRY(x) with an invoke
		// operator.
		fn.Static, fn.T = nil, types.Type{}
		return c.invokeValue(x, c.member(fn), want, sink)
	}
	if len(ms) == 0 {
		if cls.External && len(c.javaMethods(cls, fn.Name)) > 0 {
			c.errorf(fn.P, "'%s' is not a static method of %s", fn.Name, cls.DisplayName())
		} else {
			c.errorf(fn.P, "unresolved reference: %s", fn.Name)
		}
		c.args(x, nil)
		return types.InvalidType
	}
	return c.resolve(x, methodCallees(ms, nil), want, sink)
}

// javaCall checks a method call on a Java object.
func (c *Checker) javaCall(x *syntax.Call, fn *syntax.Member, rt types.Type, want types.Type, sink *system) types.Type {
	cls := c.classOf(rt)
	if cls == nil || !c.usable(cls, fn.P) {
		if cls == nil {
			c.errorf(fn.P, "unresolved reference: %s", fn.Name)
		}
		c.args(x, nil)
		return types.InvalidType
	}
	if fn.Name == "length" && c.isCharSequence(cls) || fn.Name == "size" && c.hasSize(cls) {
		c.errorf(fn.P, "expression '%s' of type Int cannot be invoked as a function", fn.Name)
		c.args(x, nil)
		return types.InvalidType
	}
	ms := c.lowPriorityGivesWay(c.accessible(c.javaMethods(cls, fn.Name)), fn.Name, rt, x)
	if len(ms) == 0 && fn.Name == "get" && c.isCharSequence(cls) {
		ms = []*types.FunSym{c.charSequenceGet()} // s[i]: charAt(i)
	}
	if len(ms) == 0 {
		if cands := c.extCalleesFor(fn.Name, rt, x); len(cands) > 0 {
			return c.resolve(x, cands, want, sink)
		}
		c.errorf(fn.P, "unresolved reference: %s", fn.Name)
		c.args(x, nil)
		return types.InvalidType
	}
	if c.allPlatform(ms) {
		// The JDK's own collection members come after extensions:
		// xs.forEach { } is Iterable.forEach's, if the lambda fits.
		var fit []callee
		for _, cl := range c.extCallees(fn.Name, rt) {
			if c.mayTake(cl.f, x) {
				fit = append(fit, cl)
			}
		}
		if len(fit) > 0 {
			return c.resolve(x, fit, want, sink)
		}
	}
	cands := c.memberCallees(ms, rt)
	// Members that don't apply give way to extensions, as in Kotlin:
	// Taggable's tag(type) to the extension tag<T>(), BigDecimal's
	// plus(MathContext) to the operator BigDecimal.plus(BigDecimal).
	if exts := c.extCalleesFor(fn.Name, rt, x); len(exts) > 0 {
		fit := false
		for _, f := range ms {
			fit = fit || c.mayTake(f, x)
		}
		if !fit {
			return c.resolve(x, exts, want, sink)
		}
		if !x.Operator && hasLambda(x.Args) {
			// The arguments' types, without the lambdas, decide: a
			// LazyListScope's items(list) { } is the extension's, not
			// the member items(count: Int) { }.
			ts := c.argTypes(x)
			gs := make([]*types.FunSym, len(exts))
			for i, cl := range exts {
				gs[i] = cl.f
			}
			if !c.takeTypes(ms, x, ts) && c.takeTypes(gs, x, ts) {
				return c.resolve(x, exts, want, sink)
			}
		}
		if x.Operator || !hasLambda(x.Args) {
			// Resolving checks the arguments; lambdas are checked once.
			n := len(c.errs)
			if t := c.resolve(x, cands, want, sink); len(c.errs) == n {
				return t
			}
			c.errs = c.errs[:n]
			return c.resolve(x, exts, want, sink)
		}
	}
	t := c.resolve(x, cands, want, sink)
	if fn.Name == "get" && len(x.Args) == 1 && t.K != types.Invalid && c.isSubclassOf(cls, "java/util/Map") {
		t = t.Nullable() // Kotlin's Map.get returns V?
	}
	return t
}

// allPlatform reports whether methods are all members that only the Java
// collection interfaces declare, or a Java class's overrides of them: as
// ArrayList.forEach(Consumer) overrides Iterable's.
func (c *Checker) allPlatform(ms []*types.FunSym) bool {
	if !c.u.ext.kotlin {
		return false
	}
	for _, f := range ms {
		if !f.Platform && !(f.Owner != nil && f.Owner.External && c.u.ext.platform()[f.Name+f.Desc]) {
			return false
		}
	}
	return true
}

// staticField checks an access such as Integer.MAX_VALUE.
func (c *Checker) staticField(x *syntax.Member, cls *types.ClassSym) types.Type {
	x.Static = cls
	if !c.usable(cls, x.X.Pos()) {
		return types.InvalidType
	}
	if x.Name == "entries" && cls.Entries != nil && c.u.ext.kotlin {
		return c.enumEntries(x, cls)
	}
	f := c.staticFieldOf(cls, x.Name)
	if f == nil {
		c.errorf(x.P, "unresolved reference: %s", x.Name)
		return types.InvalidType
	}
	x.Field = f
	return f.Type
}

// javaProperty checks x.name on a Java object, which Kotlin maps to
// getName() (or isName() for names starting with "is") and setName(v).
func (c *Checker) javaProperty(x *syntax.Member, rt types.Type) (types.Type, bool) {
	cls := c.classOf(rt)
	if cls == nil {
		return types.InvalidType, false
	}
	if !c.usable(cls, x.P) {
		return types.InvalidType, true
	}
	var kp *types.KProp
	cls.Supertypes(func(s *types.ClassSym) bool {
		kp = s.KProps[x.Name]
		return kp == nil
	})
	if kp = c.kpropAlt(kp); kp != nil {
		x.Getter, x.Setter = kp.Getter, kp.Setter
		return c.memberSubst(rt, kp.Getter.Owner).Apply(kp.Getter.Ret), true
	}
	// Kotlin decapitalizes the name of getTypeName(): typeName. A
	// capitalized name, as TypeName, names a class, not the getter.
	if r, _ := utf8.DecodeRuneInString(x.Name); unicode.IsUpper(r) {
		return types.InvalidType, false
	}
	getterName := types.GetterName(x.Name)
	special := false
	switch {
	case x.Name == "length" && c.isCharSequence(cls):
		getterName, special = "length", true // Kotlin's CharSequence.length property
	case x.Name == "size" && c.hasSize(cls):
		getterName, special = "size", true // Kotlin's Collection.size and Map.size
	case (x.Name == "name" || x.Name == "ordinal") && c.isSubclassOf(cls, "java/lang/Enum"):
		getterName, special = x.Name, true // Kotlin's Enum.name and Enum.ordinal
	case cls.External && cls.Retention != types.NoRetention:
		getterName, special = x.Name, true // a Java annotation's element: name()
	}
	setterName := types.SetterName(x.Name)
	// Not those of an earlier check of x, an extension property's: a
	// member setTextColor isn't the setter of TextView.textColor.
	x.Getter, x.Setter = nil, nil
	for _, f := range c.accessible(c.javaMethods(cls, getterName)) {
		// Only Java methods are seen as properties, and the program's
		// overrides of them.
		if c.javaAccessor(f) && len(f.Params) == 0 && !f.Unsupported && f.Ret.K != types.Unit &&
			(getterName != x.Name || f.Ret.Flex() == types.BoolType || special) {
			x.Getter = f
		}
	}
	if g := acronymAccessor("get", x.Name); x.Getter == nil && !special && g != "" {
		for _, f := range c.accessible(c.javaMethods(cls, g)) {
			if f.Owner.External && len(f.Params) == 0 && !f.Unsupported && f.Ret.K != types.Unit {
				x.Getter, setterName = f, acronymAccessor("set", x.Name)
			}
		}
	}
	if x.Getter == nil {
		return types.InvalidType, false
	}
	sub := c.memberSubst(rt, x.Getter.Owner)
	// A setter may return a value, which an assignment drops: Android's
	// Intent.setFlags returns the Intent.
	for _, f := range c.accessible(c.javaMethods(cls, setterName)) {
		if c.javaAccessor(f) && len(f.Params) == 1 && !f.Unsupported && sameJavaType(f, x.Getter) {
			x.Setter = f
		}
	}
	return sub.Apply(x.Getter.Ret), true
}

// sameJavaType reports whether setter's parameter is of getter's type,
// in Java's terms when both are Java methods: their descriptors.
func sameJavaType(setter, getter *types.FunSym) bool {
	if setter.Desc != "" && getter.Desc != "" && setter.Decl == nil && getter.Decl == nil {
		_, ret, _ := strings.Cut(getter.Desc, ")")
		return strings.HasPrefix(setter.Desc, "("+ret+")")
	}
	return setter.Params[0].Type.NonNull() == getter.Ret.NonNull()
}

// builtinParams has the names of the parameters of the constructors of
// the Java classes that are Kotlin's built-in classes, which Kotlin
// declares: Throwable(message = "x").
var builtinParams = map[string][]string{
	"java/lang/Throwable(Ljava/lang/String;)V":                      {"message"},
	"java/lang/Throwable(Ljava/lang/Throwable;)V":                   {"cause"},
	"java/lang/Throwable(Ljava/lang/String;Ljava/lang/Throwable;)V": {"message", "cause"},
}

// javaAccessor reports whether method f can be a Java property's getter
// or setter: a Java method, or a Kotlin override of one, as ChipButton's
// setEnabled(Boolean), which View.isEnabled's assignments call.
func (c *Checker) javaAccessor(f *types.FunSym) bool {
	if f.Owner.External {
		return true
	}
	if f.Decl == nil {
		return false
	}
	for _, g := range c.overridden(f.Owner, f) {
		if c.javaAccessor(g) {
			return true
		}
	}
	return false
}

// acronymAccessor returns the name of the Java accessor, with prefix get
// or set, of property p if p starts with an acronym, which Kotlin
// decapitalizes as a whole: getFD() is fd, getURLConnection()
// urlConnection. It returns "" for other names.
func acronymAccessor(prefix, p string) string {
	i := strings.IndexFunc(p, unicode.IsUpper)
	if i < 0 {
		i = len(p)
	}
	if i < 2 {
		return ""
	}
	return prefix + strings.ToUpper(p[:i]) + p[i:]
}

// numberConversions are kotlin.Number's functions and the methods of
// java.lang.Number that they are.
var numberConversions = [][2]string{
	{"toByte", "byteValue"}, {"toShort", "shortValue"}, {"toInt", "intValue"},
	{"toLong", "longValue"}, {"toFloat", "floatValue"}, {"toDouble", "doubleValue"},
}

// arrayIterator returns the function that iterator() of an array of type
// t is, as kotlinc compiles it: ArrayIteratorKt.iterator(T[]) for an
// Array<T>, giving an Iterator<T>, and ArrayIteratorsKt.iterator(int[])
// and the others for primitive arrays, giving an IntIterator.
func (c *Checker) arrayIterator(t types.Type) *types.FunSym {
	f := &types.FunSym{Name: "iterator", Ext: true, Operator: true, Final: true, Resolved: true}
	if t.C == types.ArrayClass {
		it := c.u.ext.class("java/util/Iterator")
		if it == nil {
			return nil
		}
		tp := &types.TypeParamSym{Name: "T", Owner: f}
		pt := types.ParamType(tp)
		f.TypeParams = []*types.TypeParamSym{tp}
		f.Recv = types.GenericType(types.ArrayClass, []types.Arg{{V: types.Out, T: pt}})
		f.Ret = types.GenericType(it, []types.Arg{{T: pt}})
		f.Facade, f.Desc = "kotlin/jvm/internal/ArrayIteratorKt", "([Ljava/lang/Object;)Ljava/util/Iterator;"
		return f
	}
	name := "kotlin/collections/" + strings.TrimPrefix(t.C.KotlinName, "kotlin/")
	it := c.u.ext.class(strings.TrimSuffix(name, "Array") + "Iterator")
	if it == nil {
		return nil
	}
	f.Recv, f.Ret = types.ClassType(t.C), types.ClassType(it)
	f.Facade, f.Desc = "kotlin/jvm/internal/ArrayIteratorsKt", "("+t.C.Name+")L"+it.Name+";"
	return f
}
