package check

import (
	"maps"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/inline"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Declarations of the Kotlin standard library, read from metadata.
//
// When the standard library is on the class path, kc sees Kotlin
// classes, top-level functions and the built-in collection types as
// kotlinc does, from the @kotlin.Metadata of class files and from the
// .kotlin_builtins and .kotlin_module files:
//
//   - A Kotlin class gets its type parameters, supertypes, constructors,
//     functions and properties from metadata, not from its Java signatures.
//   - The collection types, such as List and MutableList, have no class
//     files. Their members come from .kotlin_builtins; on the JVM they are
//     the java.util interfaces. Java's own types are seen through them: a
//     java.util.List in a Java signature is a platform MutableList.
//   - Top-level functions are found by package: the module file lists the
//     classes that hold each package's declarations. A package is read when
//     a name is first looked up in it, and each declaration is decoded
//     when its name is first used.
//   - Function types are kotlin.jvm.functions.FunctionN.
//
// Inline functions are inlined: their bodies are read from the class files
// and copied into the callers (see internal/inline and
// internal/codegen/inline.go). An inline-only function, which has no
// callable method, can't be called if kc can't copy its body; a public
// one is called instead. A function with reified type parameters must be
// copied, since its own method doesn't know their type arguments.

// defaultPackages are imported into every Kotlin file on the JVM, after
// explicit and star imports. java.lang comes last, in findClass.
var defaultPackages = []string{
	"kotlin", "kotlin.annotation", "kotlin.collections", "kotlin.comparisons", "kotlin.io",
	"kotlin.ranges", "kotlin.sequences", "kotlin.text", "kotlin.jvm",
}

// The built-in collection types and the Java interfaces they are on the JVM.
var collectionJVM = map[string]string{
	"kotlin/collections/Iterable":                "java/lang/Iterable",
	"kotlin/collections/MutableIterable":         "java/lang/Iterable",
	"kotlin/collections/Collection":              "java/util/Collection",
	"kotlin/collections/MutableCollection":       "java/util/Collection",
	"kotlin/collections/List":                    "java/util/List",
	"kotlin/collections/MutableList":             "java/util/List",
	"kotlin/collections/Set":                     "java/util/Set",
	"kotlin/collections/MutableSet":              "java/util/Set",
	"kotlin/collections/Map":                     "java/util/Map",
	"kotlin/collections/MutableMap":              "java/util/Map",
	"kotlin/collections/Map.Entry":               "java/util/Map$Entry",
	"kotlin/collections/MutableMap.MutableEntry": "java/util/Map$Entry",
	"kotlin/collections/Iterator":                "java/util/Iterator",
	"kotlin/collections/MutableIterator":         "java/util/Iterator",
	"kotlin/collections/ListIterator":            "java/util/ListIterator",
	"kotlin/collections/MutableListIterator":     "java/util/ListIterator",
}

// mutableOf maps the Java collection interfaces to the mutable types Java
// signatures are seen as.
var mutableOf = map[string]string{
	"java/lang/Iterable":     "kotlin/collections/MutableIterable",
	"java/util/Collection":   "kotlin/collections/MutableCollection",
	"java/util/List":         "kotlin/collections/MutableList",
	"java/util/Set":          "kotlin/collections/MutableSet",
	"java/util/Map":          "kotlin/collections/MutableMap",
	"java/util/Map$Entry":    "kotlin/collections/MutableMap.MutableEntry",
	"java/util/Iterator":     "kotlin/collections/MutableIterator",
	"java/util/ListIterator": "kotlin/collections/MutableListIterator",
}

// readOnlyOf maps each mutable collection type to its read-only one.
var readOnlyOf = map[string]string{
	"kotlin/collections/MutableIterable":         "kotlin/collections/Iterable",
	"kotlin/collections/MutableCollection":       "kotlin/collections/Collection",
	"kotlin/collections/MutableList":             "kotlin/collections/List",
	"kotlin/collections/MutableSet":              "kotlin/collections/Set",
	"kotlin/collections/MutableMap":              "kotlin/collections/Map",
	"kotlin/collections/MutableMap.MutableEntry": "kotlin/collections/Map.Entry",
	"kotlin/collections/MutableIterator":         "kotlin/collections/Iterator",
	"kotlin/collections/MutableListIterator":     "kotlin/collections/ListIterator",
}

// Kotlin types that are Java classes on the JVM, with the same members.
var javaLangOf = map[string]string{
	"kotlin/Comparable":   "java/lang/Comparable",
	"kotlin/CharSequence": "java/lang/CharSequence",
	"kotlin/Number":       "java/lang/Number",
	"kotlin/Throwable":    "java/lang/Throwable",
	"kotlin/Enum":         "java/lang/Enum",
	"kotlin/Cloneable":    "java/lang/Cloneable",
	"kotlin/Annotation":   "java/lang/annotation/Annotation",
}

// builtinJava are the classes of javaLangOf.
var builtinJava = func() map[string]bool {
	m := map[string]bool{}
	for _, j := range javaLangOf {
		m[j] = true
	}
	return m
}()

// companionJVM maps the companion objects of the built-in types to their
// classes: Int.MAX_VALUE is IntCompanionObject.MAX_VALUE, and
// String.format(...) an extension of String.Companion.
var companionJVM = map[string]string{
	"kotlin/Int.Companion":     "kotlin/jvm/internal/IntCompanionObject",
	"kotlin/Long.Companion":    "kotlin/jvm/internal/LongCompanionObject",
	"kotlin/Short.Companion":   "kotlin/jvm/internal/ShortCompanionObject",
	"kotlin/Byte.Companion":    "kotlin/jvm/internal/ByteCompanionObject",
	"kotlin/Char.Companion":    "kotlin/jvm/internal/CharCompanionObject",
	"kotlin/Float.Companion":   "kotlin/jvm/internal/FloatCompanionObject",
	"kotlin/Double.Companion":  "kotlin/jvm/internal/DoubleCompanionObject",
	"kotlin/Boolean.Companion": "kotlin/jvm/internal/BooleanCompanionObject",
	"kotlin/String.Companion":  "kotlin/jvm/internal/StringCompanionObject",
	"kotlin/Enum.Companion":    "kotlin/jvm/internal/EnumCompanionObject",
}

// builtinCompanions are the classes of companionJVM. They are internal to
// the standard library, but Kotlin code uses them as Int.Companion.
var builtinCompanions = func() map[string]bool {
	m := map[string]bool{}
	for _, jc := range companionJVM {
		m[jc] = true
	}
	return m
}()

// builtinJVMNames are the members of the collection types whose JVM
// methods have other names (BuiltinMethodsWithDifferentJvmName and
// BuiltinSpecialProperties in kotlinc).
var builtinJVMNames = map[string]string{
	"size":     "size",
	"keys":     "keySet",
	"values":   "values",
	"entries":  "entrySet",
	"removeAt": "remove",
}

// mutators are Java methods of the collection interfaces that the
// read-only types don't have.
var mutators = map[string]bool{
	"add": true, "addAll": true, "remove": true, "removeAll": true, "retainAll": true, "removeIf": true,
	"clear": true, "set": true, "replaceAll": true, "sort": true, "put": true, "putAll": true,
	"putIfAbsent": true, "compute": true, "computeIfAbsent": true, "computeIfPresent": true,
	"merge": true, "replace": true, "setValue": true, "addFirst": true, "addLast": true,
	"removeFirst": true, "removeLast": true,
}

// kotlinTable holds what is read from Kotlin metadata. It is part of
// classTable and uses its lock for the symbol maps.
type kotlinTable struct {
	builtinsOnce sync.Once
	builtins     map[string]*kmeta.Class // the built-in classes, by class id
	// probeBuiltins are the classes of built-in types' members that
	// probes list (see builtinMembers), by class id, under mu.
	probeBuiltins map[string]*types.ClassSym
	collections   map[string]*types.ClassSym
	atomics       map[string]*types.ClassSym // kotlin.concurrent.atomics' types: see atomics.go
	raw           map[string]*types.ClassSym // the Java collection interfaces themselves
	kfuncs        map[int]*types.ClassSym    // kotlin.reflect.KFunctionN, by N, and KSuspendFunctionN, by -1-N

	moduleOnce sync.Once
	parts      map[string][]kmeta.Part // by package name

	// pkgs are the packages of parts, by name, made with them. A package's
	// declarations are read on first use (see pkg).
	pkgs map[string]*kpkg
	// refs are the packages lookups have asked pkgRef for, by the name asked.
	refs sync.Map

	defaultOnce sync.Once
	defaultPkgs []*pkgRef // defaultPackages' refs

	platformOnce sync.Once
	platformSet  map[string]bool
}

// kpkg is a Kotlin package's top-level declarations.
type kpkg struct {
	name  string // dotted, kotlin.collections
	once  sync.Once
	parts []*kpart
	// names are the names of the parts' functions, properties and type
	// aliases: a name that isn't one needs no lookup and no cache entry,
	// as the program's own functions' names, which calls look up here too.
	names map[string]bool

	mu   sync.Mutex
	funs map[string][]*types.FunSym
	// entries are the functions by name, converted as lookups need
	// them, and kinds the functions of a kind: see pkgFunsWhere.
	entries map[kindKey][]*kfunEntry
	kinds   map[kindKey][]*types.FunSym
	// internalFuns are the internal functions, by pkgInternalFuns.
	internalFuns map[string][]*types.FunSym
	props        map[string][]*types.FunSym // getters of extension properties
	vars         map[string]*types.VarSym   // properties that aren't extensions; nil if none
	aliases      map[string]*types.ClassSym // nil if the name is no class alias
	// typeAliases are the other typealiases, which stand for types that
	// aren't a class with the alias's own type parameters; nil if none.
	typeAliases map[string]*types.AliasSym
}

type kpart struct {
	meta    *kmeta.Package
	facade  string          // the class to call
	class   string          // the class with the declarations: the facade, or a multifile part
	methods map[string]bool // name+desc of the public methods
	friend  bool            // in a friend path: its internal declarations are visible
	low     map[string]bool // name+desc of the methods marked LowPriorityInOverloadResolution
	// hidden are the name+desc of the methods of functions deprecated
	// with level HIDDEN, as the standard library's old Iterable<T>.maxBy,
	// which returned T?.
	hidden map[string]bool
	// annots are the methods with annotations or a deprecation.
	annots map[string]*classfile.Member
}

// pkgRef is a package that lookups of simple names probe, resolved
// once: a star import or a default import.
type pkgRef struct {
	name string // as asked: kotlin.collections
	path string // internal: kotlin/collections
	hash uint64 // classpath.Hash(path)
	k    *kpkg  // its Kotlin declarations, read on first use; nil if none
}

// pkgRef returns the package of a name, dotted as kotlin.collections. A
// slashed name finds the package's classes but no Kotlin declarations, as
// pkg finds none for it.
func (t *classTable) pkgRef(name string) *pkgRef {
	t = t.root()
	if r, ok := t.refs.Load(name); ok {
		return r.(*pkgRef)
	}
	path := strings.ReplaceAll(name, ".", "/")
	r := &pkgRef{name: name, path: path, hash: classpath.Hash(path)}
	if t.kotlin {
		t.modules()
		r.k = t.pkgs[name]
	}
	actual, _ := t.refs.LoadOrStore(name, r)
	return actual.(*pkgRef)
}

// javaLangHash is classpath.Hash("java/lang").
var javaLangHash = classpath.Hash("java/lang")

// starPkgs returns the packages of c.stars, by their dotted names, in
// order.
func (c *Checker) starPkgs() []*pkgRef {
	for len(c.starRefs) < len(c.stars) {
		c.starRefs = append(c.starRefs, c.u.ext.pkgRef(strings.ReplaceAll(c.stars[len(c.starRefs)], "/", ".")))
	}
	return c.starRefs
}

// defaultRefs returns the packages of defaultPackages.
func (t *classTable) defaultRefs() []*pkgRef {
	t = t.root()
	t.defaultOnce.Do(func() {
		for _, pkg := range defaultPackages {
			t.defaultPkgs = append(t.defaultPkgs, t.pkgRef(pkg))
		}
	})
	return t.defaultPkgs
}

// absent reports whether class certainly returns nil for the internal
// name that hashes to h, a name that isn't one of the classes without a
// class file (see syntheticClass): no class of the class path has it,
// and no class of the program takes its place.
func (t *classTable) absent(h uint64) bool {
	r := t.root()
	return t.cp != nil && r.shadow.Load() == nil && t.sources.Load() == nil && r.sources.Load() == nil && !t.cp.MayHave(h)
}

// classIn is class(qualified(path, name)) for a package path whose hash
// is h: it builds the name only if the class may be there.
func (t *classTable) classIn(path string, h uint64, name string) *types.ClassSym {
	if path != "" && !syntheticClass(path, name) && t.absent(classpath.HashJoin(h, '/', name)) {
		if verifyFilter {
			t.verifyAbsent(qualified(path, name), t.class(qualified(path, name)))
		}
		return nil
	}
	return t.class(qualified(path, name))
}

// nestedNamed is class(outer + "$" + name), building the name only if the
// class may be there.
func (t *classTable) nestedNamed(outer, name string) *types.ClassSym {
	if (outer != "java/util/Map" || name != "Entry") && t.absent(classpath.HashJoin(classpath.Hash(outer), '$', name)) {
		if verifyFilter {
			t.verifyAbsent(outer+"$"+name, t.class(outer+"$"+name))
		}
		return nil
	}
	return t.class(outer + "$" + name)
}

// syntheticClass reports whether class path/name may be one that class
// finds without a class file: a function interface or a mutable
// collection interface.
func syntheticClass(path, name string) bool {
	switch path {
	case "kotlin/jvm/functions", "kotlin":
		return strings.HasPrefix(name, "Function")
	case "kotlin/reflect":
		return strings.HasPrefix(name, "K")
	case "kotlin/coroutines":
		return strings.HasPrefix(name, "Suspend")
	case "java/lang", "java/util":
		return mutableNames[path+"/"+name] // path and name have no other separators here
	}
	return false
}

// maybeSynthetic reports whether class may find a class of an internal
// name without a class file, as syntheticClass does.
func maybeSynthetic(name string) bool {
	i := strings.LastIndexByte(name, '/')
	if i < 0 {
		return false
	}
	return syntheticClass(name[:i], name[i+1:])
}

// mutableNames are mutableOf's keys.
var mutableNames = func() map[string]bool {
	m := map[string]bool{}
	for k := range mutableOf {
		m[k] = true
	}
	return m
}()

// pkgSynthetic reports whether pkgClass may find a class of a package by
// name without a class file: a built-in type, a collection or atomic
// type, or a class standing for a java.lang one.
func pkgSynthetic(path, name string) bool {
	if strings.IndexByte(name, '.') >= 0 || strings.IndexByte(name, '/') >= 0 {
		return true
	}
	if path == "kotlin" && (strings.HasPrefix(name, "Function") || types.ArrayByName(name) != nil) {
		return true
	}
	return syntheticNames[path][name]
}

// syntheticNames are the classes pkgClass finds by id, by package and
// simple name.
var syntheticNames = func() map[string]map[string]bool {
	m := map[string]map[string]bool{}
	for _, ids := range []map[string]string{collectionJVM, atomicJVM, javaLangOf} {
		for id := range ids {
			i := strings.LastIndexByte(id, '/')
			if m[id[:i]] == nil {
				m[id[:i]] = map[string]bool{}
			}
			m[id[:i]][id[i+1:]] = true
		}
	}
	return m
}()

// fieldConstant returns the value of a final static field's ConstantValue,
// as types.VarSym.Constant holds it, or nil.
func fieldConstant(fd classfile.Member) any {
	if fd.Access&classfile.AccStatic == 0 || fd.Access&classfile.AccFinal == 0 {
		return nil
	}
	if i, ok := fd.Constant.(int32); ok {
		return int64(i)
	}
	return fd.Constant
}

// facadeConstant returns the value of the const val name of a file
// facade, whose field it is, also of a multifile class's facade, or nil.
func (t *classTable) facadeConstant(facade, name string) any {
	t = t.root()
	info, err := t.cp.Load(facade)
	if err != nil {
		return nil
	}
	for _, fd := range info.Fields {
		if fd.Name == name {
			return fieldConstant(fd)
		}
	}
	return nil
}

// timed adds the time since start and n declarations to the Kotlin
// metadata counter.
func (t *classTable) timed(start time.Time, n int) {
	t = t.root()
	t.cp.Kotlin.Add(n, time.Since(start))
}

// functionArity returns n for the class pkg+"FunctionN", such as
// kotlin/jvm/functions/Function1 or kotlin/Function1.
func functionArity(name, pkg string) (int, bool) {
	// Without building pkg+"Function": class lookups call this for every
	// name they're asked.
	s, ok := strings.CutPrefix(name, pkg)
	if ok {
		s, ok = strings.CutPrefix(s, "Function")
	}
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0 && n <= types.MaxVarArity && strconv.Itoa(n) == s
}

// kclass returns the class a Kotlin class id names, or nil if it has no
// class kc knows.
func (t *classTable) kclass(id string) *types.ClassSym {
	if _, ok := collectionJVM[id]; ok {
		t.mu.Lock()
		defer t.mu.Unlock()
		return t.collection(id)
	}
	if _, ok := atomicJVM[id]; ok {
		return t.atomic(id)
	}
	if n, ok := functionArity(id, "kotlin/"); ok {
		return t.funcs.Class(n)
	}
	if jl, ok := javaLangOf[id]; ok {
		return t.class(jl)
	}
	if jc, ok := companionJVM[id]; ok {
		return t.class(jc)
	}
	if a := types.ArrayByID(id); a != nil {
		return a
	}
	return t.class(strings.ReplaceAll(id, ".", "$"))
}

// kfunction returns kotlin.reflect.KFunctionN, the type of a reference
// to a function with n parameters, or KSuspendFunctionN, a suspend
// function's. As in Kotlin, it has no class file of its own: it is a
// KFunction<R> and a function type, and its values are
// kotlin.reflect.KFunction objects on the JVM.
func (t *classTable) kfunction(n int, suspend bool) *types.ClassSym {
	t = t.root()
	t.mu.Lock()
	defer t.mu.Unlock()
	key := n
	if suspend {
		key = -1 - n
	}
	c := t.kfuncs[key]
	if c == nil {
		if t.kfuncs == nil {
			t.kfuncs = map[int]*types.ClassSym{}
		}
		c = types.NewExternal("kotlin/reflect/KFunction", t.loadKFunction)
		c.KotlinName = "kotlin/reflect/KFunction" + strconv.Itoa(n)
		if suspend {
			c.KotlinName = "kotlin/reflect/KSuspendFunction" + strconv.Itoa(n)
		}
		t.kfuncs[key] = c
	}
	return c
}

// loadKFunction fills in a KFunctionN<in P1, ..., in Pn, out R> that
// extends KFunction<R> and FunctionN<P1, ..., Pn, R>, or a
// KSuspendFunctionN that extends SuspendFunctionN<P1, ..., Pn, R>.
func (t *classTable) loadKFunction(cls *types.ClassSym) {
	t = t.root()
	n, suspend, _ := kfunctionArity(cls)
	kf := t.class("kotlin/reflect/KFunction")
	if kf == nil {
		cls.Missing = true
		return
	}
	cls.Interface, cls.Abstract = true, true
	cls.Super = t.class("java/lang/Object")
	var args []types.Arg
	for i := range n + 1 {
		p := &types.TypeParamSym{Name: "P" + strconv.Itoa(i+1), Var: types.In, Index: i, Owner: cls, Library: true}
		if i == n {
			p.Name, p.Var = "R", types.Out
		}
		cls.TypeParams = append(cls.TypeParams, p)
		args = append(args, types.Arg{T: types.ParamType(p)})
	}
	fn := t.funcs.Class(n)
	if suspend {
		fn = t.funcs.SuspendClass(n)
	}
	cls.Supers = []types.Type{types.GenericType(kf, args[n:]), types.GenericType(fn, args)}
	cls.Interfaces = []*types.ClassSym{kf, fn}
	cls.Methods = map[string][]*types.FunSym{}
	cls.Statics = map[string][]*types.FunSym{}
	cls.Fields = map[string]*types.VarSym{}
	cls.KProps = map[string]*types.KProp{}
}

// kfunctionArity returns n if cls is kotlin.reflect.KFunctionN or
// KSuspendFunctionN, and whether it is the latter.
func kfunctionArity(cls *types.ClassSym) (n int, suspend, ok bool) {
	if !cls.KFunctionN() {
		return 0, false, false
	}
	if n, ok := functionArity(cls.KotlinName, "kotlin/reflect/KSuspend"); ok {
		return n, true, true
	}
	n, ok = functionArity(cls.KotlinName, "kotlin/reflect/K")
	return n, false, ok
}

// collection returns a built-in collection type. t.mu is held.
func (t *classTable) collection(id string) *types.ClassSym {
	t = t.root()
	c := t.collections[id]
	if c == nil {
		c = types.NewExternal(collectionJVM[id], t.loadCollection)
		c.KotlinName = id
		t.collections[id] = c
		if ro, ok := readOnlyOf[id]; ok {
			c.ReadOnly = t.collection(ro)
			c.ReadOnly.Mutable = c
		}
		for m, ro := range readOnlyOf {
			if ro == id {
				t.collection(m) // which links c to its mutable type
			}
		}
	}
	return c
}

// rawClass returns the Java class of a collection interface, which Kotlin
// code otherwise sees as a built-in type.
func (t *classTable) rawClass(name string) *types.ClassSym {
	t = t.root()
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.raw[name]
	if c == nil {
		c = types.NewExternal(name, t.load)
		t.raw[name] = c
	}
	return c
}

func (t *classTable) builtinClasses() map[string]*kmeta.Class {
	t = t.root()
	t.builtinsOnce.Do(func() {
		start := time.Now()
		t.builtins = map[string]*kmeta.Class{}
		for _, name := range t.cp.Resources(".kotlin_builtins") {
			data, err := t.cp.Resource(name)
			if err != nil {
				continue
			}
			b, err := kmeta.ReadBuiltins(data)
			if err != nil {
				continue
			}
			for _, c := range b.Classes {
				if _, dup := t.builtins[c.Name]; !dup {
					t.builtins[c.Name] = c
				}
			}
		}
		t.timed(start, len(t.builtins))
	})
	return t.builtins
}

// variance converts a declared variance or projection.
func variance(p kmeta.Projection) types.Variance {
	switch p {
	case kmeta.In:
		return types.In
	case kmeta.Out:
		return types.Out
	case kmeta.Star:
		return types.Star
	}
	return types.Inv
}

// tscope maps the ids of the type parameters in scope to their symbols.
type tscope map[int]*types.TypeParamSym

// ktypeParams creates the symbols of declared type parameters and adds them
// to scope. It reports false if a bound uses a type kc doesn't support.
func (t *classTable) ktypeParams(ps []*kmeta.TypeParam, owner any, scope tscope) ([]*types.TypeParamSym, bool) {
	t = t.root()
	syms := make([]*types.TypeParamSym, len(ps))
	for i, p := range ps {
		syms[i] = &types.TypeParamSym{Name: p.Name, Var: variance(p.Variance), Index: i, Owner: owner, Reified: p.Reified, OnlyInput: p.OnlyInput, Library: true}
		scope[p.ID] = syms[i]
	}
	ok := true
	for i, p := range ps {
		for _, b := range p.Bounds {
			bt, bok := t.ktype(b, scope)
			if !bok {
				ok = false
				continue
			}
			syms[i].Bounds = append(syms[i].Bounds, bt)
		}
	}
	return syms, ok
}

// ktype maps a type in Kotlin metadata to a kc type, and reports false
// if kc doesn't support it.
func (t *classTable) ktype(kt *kmeta.Type, scope tscope) (types.Type, bool) {
	t = t.root()
	if kt == nil {
		return types.InvalidType, false
	}
	nullable := func(r types.Type) types.Type {
		switch {
		case kt.Upper != nil: // a flexible type from Java: T!
			return r.Platform()
		case kt.Nullable:
			return r.Nullable()
		}
		return r
	}
	if kt.Class == "" {
		p, ok := scope[kt.Param]
		if !ok {
			return types.InvalidType, false
		}
		r := types.ParamType(p)
		if kt.Flags&kmeta.TypeDefinitelyNonNull != 0 {
			r.D = true
		}
		return nullable(r), true
	}
	var r types.Type
	switch kt.Class {
	case "kotlin/Int":
		r = types.IntType
	case "kotlin/Long":
		r = types.LongType
	case "kotlin/Double":
		r = types.DoubleType
	case "kotlin/Boolean":
		r = types.BoolType
	case "kotlin/Char":
		r = types.CharType
	case "kotlin/Float":
		r = types.FloatType
	case "kotlin/Byte":
		r = types.ByteType
	case "kotlin/Short":
		r = types.ShortType
	case "kotlin/String":
		r = types.StringType
	case "kotlin/Any":
		r = types.AnyType
	case "kotlin/Unit":
		if kt.Nullable {
			// Unit?, as in the checker's nullable.
			if u := t.class("kotlin/Unit"); u != nil {
				return types.ClassType(u).Nullable(), true
			}
			return types.InvalidType, false
		}
		return types.UnitType, true
	case "kotlin/Nothing":
		if kt.Nullable {
			return types.NullType, true
		}
		return types.NothingType, true
	}
	if r.K != types.Invalid {
		if len(kt.Args) > 0 {
			return types.InvalidType, false
		}
		return nullable(r), true
	}
	if kt.Flags&kmeta.TypeSuspend != 0 {
		r, ok := t.suspendType(kt, scope)
		return nullable(r), ok
	}
	cls := t.kclass(kt.Class)
	if kt.Composable {
		// kotlin.FunctionN annotated @Composable: its interface on the
		// JVM is a larger one.
		n, ok := functionArity(kt.Class, "kotlin/")
		if !ok || len(kt.Args) != n+1 {
			return types.InvalidType, false
		}
		cls = t.funcs.ComposableClass(n)
	}
	if cls == nil {
		cls = t.missingClass(kt.Class, len(kt.Args))
	}
	if cls == nil {
		return types.InvalidType, false
	}
	raw := kt.Raw || kt.Upper != nil && kt.Upper.Raw
	if !raw {
		cls = t.arityMismatch(cls, kt)
	}
	if len(kt.Args) == 0 || raw {
		// A non-generic class, or a raw Java type.
		return nullable(types.ClassType(cls)), true
	}
	args := make([]types.Arg, len(kt.Args))
	for i, a := range kt.Args {
		args[i].V = variance(a.Proj)
		if args[i].V == types.Star {
			continue
		}
		at, ok := t.ktype(a.Type, scope)
		if !ok {
			return types.InvalidType, false
		}
		args[i].T = at
	}
	for o := kt.Outer; o != nil; o = o.Outer {
		// An inner class's type has its outer class's arguments after
		// its own, as its type parameters are.
		for _, a := range o.Args {
			arg := types.Arg{V: variance(a.Proj)}
			if arg.V != types.Star {
				at, ok := t.ktype(a.Type, scope)
				if !ok {
					return types.InvalidType, false
				}
				arg.T = at
			}
			args = append(args, arg)
		}
	}
	r = types.GenericType(cls, args)
	if kt.ExtFunc && cls.Func && len(args) > 1 {
		r = r.WithReceiver()
	}
	if kt.Contexts > 0 && cls.Func && len(args) > kt.Contexts {
		r = r.WithContexts(kt.Contexts)
	}
	return nullable(r), true
}

// arityMismatch returns cls, the class kt names, or a stand-in of its name
// if kt gives it another number of type arguments than it has: kt is of a
// class file of another module that has a class of that name, which the
// class path's first one hides (see classpath.Duplicated). K2 finds one
// class of a name too, and a type of it with the other module's arguments
// is a subtype of neither its types nor others: A<Int> isn't A. But a type
// of a class without type parameters is one of its own name with any
// arguments (see subtype1).
func (t *classTable) arityMismatch(cls *types.ClassSym, kt *kmeta.Type) *types.ClassSym {
	if t.cp == nil || cls.Missing || !t.cp.Duplicated(cls.Name) {
		return cls
	}
	n := len(kt.Args)
	for o := kt.Outer; o != nil; o = o.Outer {
		n += len(o.Args)
	}
	a, ok := t.arity(cls.Name, 0)
	if !ok || a.n == n {
		return cls
	}
	key := "mismatch:" + cls.Name + "/" + strconv.Itoa(n)
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.m[key]; ok {
		return c
	}
	c := types.NewExternal(cls.Name, func(*types.ClassSym) {})
	c.Ensure()
	c.Interface, c.ArityOf = a.iface, cls
	for i := range n {
		c.TypeParams = append(c.TypeParams, &types.TypeParamSym{Name: "T" + strconv.Itoa(i), Index: i, Owner: c, Library: true})
	}
	t.m[key] = c
	return c
}

// classArity is a class's number of type parameters, its outer classes'
// included, and whether it is an interface.
type classArity struct {
	n     int
	iface bool
}

// arity returns the arity of the class of the class path named name, as
// load gives it, from its class file alone. arityMismatch can't load the
// class: a type in a class's own signatures can name it, and its loading
// would then wait for itself.
func (t *classTable) arity(name string, depth int) (classArity, bool) {
	if t.cp == nil || depth > 8 {
		return classArity{}, false
	}
	type key struct {
		cp   *classpath.Path
		name string
	}
	k := key{t.cp, name}
	if v, ok := t.arities.Load(k); ok {
		return v.(classArity), true
	}
	info, err := t.cp.Load(name)
	if err != nil {
		return classArity{}, false
	}
	a := classArity{iface: info.Access&classfile.AccInterface != 0}
	outer := ""
	fromKotlin := false
	if t.kotlin && info.Kotlin != nil && info.Kotlin.K == kmeta.KindClassFile {
		if kc, err := kmeta.ReadClass(info.Kotlin); err == nil {
			fromKotlin = true
			a.n = len(kc.TypeParams)
			if kc.IsInner() && info.Inner != nil {
				outer = info.Inner.Outer
			}
		}
	}
	if !fromKotlin {
		a.n = sigArity(info.Sig)
		if info.Inner != nil && info.Inner.Outer != "" && info.Inner.Access&classfile.AccStatic == 0 &&
			info.Access&(classfile.AccInterface|classfile.AccEnum|classfile.AccAnnotation) == 0 {
			if oi, err := t.cp.Load(info.Inner.Outer); err == nil && oi.Access&classfile.AccInterface == 0 {
				outer = info.Inner.Outer
			}
		}
	}
	if outer != "" {
		o, ok := t.arity(outer, depth+1)
		if !ok {
			return classArity{}, false
		}
		a.n += o.n
	}
	t.arities.Store(k, a)
	return a, true
}

// sigArity returns the number of the type parameters a class's generic
// signature declares.
func sigArity(sig string) int {
	p := &sigParser{s: sig, ok: true}
	if p.peek() != '<' {
		return 0
	}
	p.i++
	n := 0
	for p.ok && p.peek() != '>' {
		p.ident(":")
		n++
		for p.ok && p.peek() == ':' {
			p.i++
			if p.isTypeStart() {
				p.skipRef()
			}
		}
	}
	return n
}

// suspendType maps a suspend function type, which metadata writes as the
// function type of its JVM interface: suspend (A) -> R is kotlin.Function2
// <A, Continuation<R>, Any?>.
func (t *classTable) suspendType(kt *kmeta.Type, scope tscope) (types.Type, bool) {
	t = t.root()
	n, ok := functionArity(kt.Class, "kotlin/")
	if !ok || n == 0 || len(kt.Args) != n+1 {
		return types.InvalidType, false
	}
	cont := kt.Args[n-1].Type
	if cont == nil || cont.Class != "kotlin/coroutines/Continuation" || len(cont.Args) != 1 || cont.Args[0].Type == nil && cont.Args[0].Proj != kmeta.Star {
		return types.InvalidType, false
	}
	args := make([]types.Arg, n)
	for i, a := range kt.Args[:n-1] {
		args[i].V = variance(a.Proj)
		if args[i].V == types.Star {
			continue
		}
		at, ok := t.ktype(a.Type, scope)
		if !ok {
			return types.InvalidType, false
		}
		args[i].T = at
	}
	if cont.Args[0].Proj == kmeta.Star {
		args[n-1].V = types.Star // suspend () -> *, as GivenLambda<*, *>
	} else {
		rt, ok := t.ktype(cont.Args[0].Type, scope)
		if !ok {
			return types.InvalidType, false
		}
		args[n-1] = types.Arg{T: rt, V: variance(cont.Args[0].Proj)}
		if args[n-1].V == types.In {
			args[n-1].V = types.Inv
		}
	}
	r := types.GenericType(t.funcs.SuspendClass(n-1), args)
	if kt.ExtFunc && n > 1 {
		r = r.WithReceiver()
	}
	if kt.Contexts > 0 && n > kt.Contexts {
		r = r.WithContexts(kt.Contexts)
	}
	return r, true
}

// isUnsigned reports whether t is an unsigned integer type.
func isUnsigned(t types.Type) bool {
	if !types.IsValue(t) {
		return false
	}
	switch t.C.Name {
	case "kotlin/UInt", "kotlin/ULong", "kotlin/UByte", "kotlin/UShort":
		return true
	}
	return false
}

// markValue marks a class of the class path as a value class if its
// metadata says it is one. Java's classes have no metadata. It returns
// false for a class kotlinc can't see: one whose metadata says it is a
// Kotlin class, k = 1 written out, but has neither its data nor a metadata
// version, as a Java class annotated @Metadata(k = 1) compiles to.
func (t *classTable) markValue(cls *types.ClassSym) bool {
	for _, pkg := range javaPackages {
		if strings.HasPrefix(cls.Name, pkg) {
			return true
		}
	}
	info, err := t.cp.Load(cls.Name)
	if err != nil || info.Kotlin == nil {
		return true
	}
	if m := info.Kotlin; m.KWritten && m.K == kmeta.KindClassFile && len(m.D1) == 0 && len(m.MV) == 0 {
		return false
	}
	kc, err := kmeta.ReadClass(info.Kotlin)
	if err != nil {
		return true
	}
	t.valueOf(cls, kc)
	t.decoded.Store(cls.Name, kc) // for load, which needn't decode it again
	return true
}

// javaPackages are packages without Kotlin classes, whose classes
// markValue needn't read.
var javaPackages = []string{"java/", "javax/", "jdk/", "sun/", "android/", "dalvik/"}

// valueOf marks cls a value class if metadata kc says it is one: its
// values are then held unboxed. Its underlying property's type is in the
// metadata of its property, which kotlinc 2.4 writes instead of the type
// itself, and may be a value class too: Compose's Color is a ULong.
func (t *classTable) valueOf(cls *types.ClassSym, kc *kmeta.Class) {
	if !kc.IsValue() || cls.Value {
		return
	}
	u := kc.Underlying
	for _, p := range kc.Props {
		if u == nil && p.Name == kc.UnderlyingProp {
			u = p.Type
		}
	}
	if u == nil {
		return
	}
	// A generic value class's underlying type may use its type
	// parameters, value class A<T>(val x: T & Any), so they are created
	// here, before the class is loaded, which keeps them.
	scope := tscope{}
	var tps []*types.TypeParamSym
	if len(kc.TypeParams) > 0 && cls.TypeParams == nil {
		tps = t.valueTypeParams(cls, kc, scope)
	}
	if ut, ok := t.ktype(u, scope); ok && ut.K != types.Invalid {
		cls.Value = true
		cls.Underlying = &types.VarSym{Name: kc.UnderlyingProp, Type: ut, Kind: types.Prop, Owner: cls, Resolved: true, Final: true}
		if tps != nil {
			cls.TypeParams = tps
		}
	}
}

// valueTypeParams creates the type parameters of value class kc for
// valueOf, and adds them to scope. Bounds that use the class itself, as
// T : Comparable<A<T>>, are left out: they would load it again while it is
// being created, and the underlying value's type erases to the first
// bound's class, which is another.
func (t *classTable) valueTypeParams(cls *types.ClassSym, kc *kmeta.Class, scope tscope) []*types.TypeParamSym {
	t = t.root()
	syms := make([]*types.TypeParamSym, len(kc.TypeParams))
	for i, p := range kc.TypeParams {
		syms[i] = &types.TypeParamSym{Name: p.Name, Var: variance(p.Variance), Index: i, Owner: cls, Reified: p.Reified, Library: true}
		scope[p.ID] = syms[i]
	}
	for i, p := range kc.TypeParams {
		for _, b := range p.Bounds {
			if mentionsClass(b, kc.Name) {
				continue
			}
			if bt, ok := t.ktype(b, scope); ok {
				syms[i].Bounds = append(syms[i].Bounds, bt)
			}
		}
	}
	return syms
}

// mentionsClass reports whether type kt uses the class named name.
func mentionsClass(kt *kmeta.Type, name string) bool {
	if kt == nil {
		return false
	}
	if kt.Class == name || mentionsClass(kt.Upper, name) {
		return true
	}
	for _, a := range kt.Args {
		if mentionsClass(a.Type, name) {
			return true
		}
	}
	return false
}

// kfun creates the symbol of a Kotlin function. scope holds the type
// parameters of the enclosing class; methods says which JVM methods are
// public (nil: all are).
func (t *classTable) kfun(f *kmeta.Function, owner *types.ClassSym, scope tscope, methods map[string]bool) *types.FunSym {
	t = t.root()
	sym := &types.FunSym{Name: f.Name, Owner: owner, Desc: f.JVM.Desc, Operator: f.IsOperator(), Resolved: true}
	if f.JVM.Name != f.Name {
		sym.JVMName = f.JVM.Name
	}
	scope = maps.Clone(scope)
	if scope == nil {
		scope = tscope{}
	}
	tps, ok := t.ktypeParams(f.TypeParams, sym, scope)
	sym.TypeParams = tps
	sym.Params, ok = t.kparams(f.Params, scope, ok)
	ret, rok := t.ktype(f.Ret, scope)
	sym.Ret = ret
	sym.NoInferRet = kNoInfer(f.Ret)
	sym.MissingAnno = t.missingTypeAnno(f.Ret)
	for _, p := range f.Params {
		if a := t.missingTypeAnno(p.Type); a != "" && sym.ParamMissingAnno == "" {
			sym.ParamMissingAnno = a
		}
	}
	if f.Recv != nil {
		sym.Ext = true
		recv, recvOK := t.ktype(f.Recv, scope)
		sym.Recv = recv
		sym.NoInferRecv = kNoInfer(f.Recv)
		ok = ok && recvOK
	}
	sym.Infix = f.IsInfix()
	sym.Suspend = f.IsSuspend()
	for _, ip := range f.InPlace {
		if ip.Param < 0 || ip.Param >= len(f.Params) {
			continue
		}
		if sym.InPlace == nil {
			sym.InPlace = make([]bool, len(f.Params))
		}
		sym.InPlace[ip.Param] = true
		if ip.Kind == kmeta.ExactlyOnce {
			if sym.Once == nil {
				sym.Once = make([]bool, len(f.Params))
			}
			sym.Once[ip.Param] = true
		}
		if ip.Kind == kmeta.AtLeastOnce {
			if sym.AtLeastOnce == nil {
				sym.AtLeastOnce = make([]bool, len(f.Params))
			}
			sym.AtLeastOnce[ip.Param] = true
		}
		if ip.Kind == kmeta.AtMostOnce {
			if sym.AtMostOnce == nil {
				sym.AtMostOnce = make([]bool, len(f.Params))
			}
			sym.AtMostOnce[ip.Param] = true
		}
	}
	for _, im := range f.Implies {
		// A context parameter's index follows the value parameters'.
		if im.Param < len(f.Params)+len(f.ContextParams) && (im.Param >= 0 || f.Recv != nil) {
			r := -1
			switch im.Returns {
			case kmeta.ReturnsTrue:
				r = 1
			case kmeta.ReturnsFalse:
				r = 0
			case kmeta.ReturnsNotNull:
				r = 2
			case kmeta.ReturnsNull:
				r = 3
			}
			e := types.Effect{Returns: r, Param: im.Param, Holds: im.Holds, Negated: im.Negated}
			if im.Is != nil {
				is, isOK := t.ktype(im.Is, scope)
				if !isOK {
					continue
				}
				e.Is = &is
			}
			sym.Implies = append(sym.Implies, e)
		}
	}
	jvm := f.JVM
	if methods != nil && !hasMethod(methods, jvm.Name+jvm.Desc, f.IsInline()) && jvm.Name == f.Name && ok && rok {
		// A metadata's signature leaves out the name and descriptor that
		// value class parameters give: Compose's MeasurePolicy's
		// measure-3p2s80s(MeasureScope, List, long), which kotlinc
		// computes.
		if name, desc, found := mangledJVM(sym, methods); found {
			sym.JVMName, sym.Desc = name, desc
			jvm = kmeta.JVMSig{Name: name, Desc: desc}
		}
	}
	if f.Contexts > len(f.ContextParams) && sym.Why == "" {
		sym.Why, sym.Feature = "calling a library's functions with context receivers is not supported", "context receivers"
	}
	if len(f.ContextParams) > 0 {
		var cok bool
		sym.Contexts, cok = t.kparams(f.ContextParams, scope, true)
		ok = ok && cok
	}
	ok = ok && rok && f.Contexts == len(f.ContextParams) && sym.Desc != "" && zeroable(sym)
	matchJVM(sym)
	composeShape(sym, owner != nil)
	composeTypes(sym)
	sym.Unsupported = !ok || sym.Unsupported
	sym.Abstract = f.Flags.Modality() == kmeta.Abstract
	if methods != nil && !hasMethod(methods, jvm.Name+jvm.Desc, f.IsInline()) && sym.Why == "" {
		sym.Unsupported = true
		sym.Why = "'" + f.Name + "' is an inline-only function, and kc doesn't inline functions"
		if ok && f.IsInline() {
			// pkgFuns makes a top-level function callable by inlining it.
			sym.Inline = inlineInfo(f, true)
		}
	}
	if f.IsInline() {
		in := inlineInfo(f, false)
		sym.Declared = &types.InlineFlags{Crossinline: in.Crossinline, Noinline: in.Noinline}
	}
	return sym
}

// mangledJVM returns the name and descriptor of the method of f, a
// function of a class-path class other than a value class, as kotlinc
// mangles them for its value class parameters, if methods has it.
func mangledJVM(f *types.FunSym, methods map[string]bool) (string, string, bool) {
	if f.Owner != nil && f.Owner.Value {
		return "", "", false
	}
	probe := *f
	probe.Desc, probe.JVMName = "", ""
	name := probe.JVM()
	if name == f.Name {
		return "", "", false
	}
	var ps []*types.VarSym
	if f.Ext {
		ps = append(ps, &types.VarSym{Type: f.Recv})
	}
	ps = append(ps, f.Params...)
	d, ok := types.MethodDesc(ps, f.Ret)
	if !ok {
		return "", "", false
	}
	if f.Suspend {
		d = types.SuspendDesc(d)
	}
	return name, d, methods[name+d]
}

// inlineInfo describes an inline function, whose class is set later.
func inlineInfo(f *kmeta.Function, private bool) *types.InlineInfo {
	in := &types.InlineInfo{Name: f.JVM.Name, Desc: f.JVM.Desc, Private: private, Suspend: f.IsSuspend()}
	for _, p := range f.Params {
		in.Crossinline = append(in.Crossinline, p.Flags&kmeta.ParamCrossinline != 0)
		in.Noinline = append(in.Noinline, p.Flags&kmeta.ParamNoinline != 0)
	}
	return in
}

// loadInline returns the function that reads and analyzes the bodies of
// a class's inline functions.
type inlineKey struct{ class, name, desc string }

func (t *classTable) loadInline(class string) func(name, desc string) (*inline.Body, error) {
	t = t.root()
	return func(name, desc string) (*inline.Body, error) {
		key := inlineKey{class, name, desc}
		if load, ok := t.inlineBodies.Load(key); ok {
			return load.(func() (*inline.Body, error))()
		}
		load := sync.OnceValues(func() (*inline.Body, error) {
			start := time.Now()
			defer func() { t.cp.Inline.Add(1, time.Since(start)) }()
			methods, err := t.cp.MethodReader(class)
			if err != nil {
				return nil, err
			}
			m, err := methods.ReadMethod(name, desc)
			if err != nil {
				return nil, err
			}
			b, err := inline.Analyze(m, methods.ReadMethod)
			if err != nil {
				return nil, err
			}
			if err := b.LoadTemplates(t.cp.Bytes); err != nil {
				return nil, err
			}
			return b, nil
		})
		actual, _ := t.inlineBodies.LoadOrStore(key, load)
		return actual.(func() (*inline.Body, error))()
	}
}

// kparams creates the symbols of value parameters. It reports whether kc
// supports their types, and ok.
func (t *classTable) kparams(ps []*kmeta.Param, scope tscope, ok bool) ([]*types.VarSym, bool) {
	t = t.root()
	syms := make([]*types.VarSym, len(ps))
	for i, p := range ps {
		kt := p.Type
		if p.Vararg != nil {
			kt = p.Vararg
		}
		pt, pok := t.ktype(kt, scope)
		if p.Vararg != nil {
			if isUnsigned(pt) && !pt.N {
				// A vararg of unsigned integers is an unsigned array:
				// UIntArray for UInt.
				a := t.class(pt.C.Name + "Array")
				pok = pok && a != nil
				pt = types.InvalidType
				if a != nil {
					pt = types.ClassType(a)
				}
			} else {
				pt = types.VarargArray(pt)
			}
		}
		def := p.Flags&kmeta.ParamDeclaresDefault != 0
		// A parameter of a type kc doesn't support can still be left to
		// its default value: its type is then Invalid.
		ok = ok && (pok || def && p.Vararg == nil)
		syms[i] = &types.VarSym{Name: p.Name, Type: pt, Kind: types.Param, Vararg: p.Vararg != nil, Default: def, NoInfer: kNoInfer(kt)}
	}
	return syms, ok
}

// kNoInfer reports whether a type of metadata, or a type in it, is
// annotated @NoInfer, as noInfer does for the program's.
func kNoInfer(t *kmeta.Type) bool {
	if t == nil {
		return false
	}
	if t.NoInfer {
		return true
	}
	for _, a := range t.Args {
		if kNoInfer(a.Type) {
			return true
		}
	}
	return false
}

// jvmParams splits a method descriptor's parameter types, and returns
// its result type.
func jvmParams(d string) ([]string, string) { return classfile.DescParams(d), classfile.RetDesc(d) }

// boxedOnJVM returns t as the type of a JVM value of type jvm: a Kotlin
// Int is a java.lang.Integer where the method overrides a generic one,
// as IntIterator.next() does Iterator<Int>.next(). kc sees it as Int!.
func boxedOnJVM(t types.Type, jvm string) types.Type {
	if t.IsPrimitive() && (jvm[0] == 'L' || jvm[0] == '[') {
		return t.Nullable().Platform()
	}
	return t
}

// matchJVM gives f's parameter and result types the representation its
// JVM descriptor has.
func matchJVM(f *types.FunSym) {
	if f.Desc == "" {
		return
	}
	ps, ret := jvmParams(f.Desc)
	n := len(f.Contexts) + len(f.Params)
	if f.Ext {
		n++
	}
	if o := f.Owner; o != nil && o.Value && f.Name != "<init>" && len(ps) == n+1 {
		ps = ps[1:] // a value class's static -impl method takes the value first
	}
	for i, p := range f.Contexts {
		if i < len(ps) {
			p.Type = boxedOnJVM(p.Type, ps[i])
		}
	}
	ps = ps[min(len(f.Contexts), len(ps)):]
	if f.Ext && len(ps) > 0 {
		f.Recv = boxedOnJVM(f.Recv, ps[0])
		ps = ps[1:]
	}
	for i, p := range f.Params {
		if i < len(ps) && !p.Vararg {
			p.Type = boxedOnJVM(p.Type, ps[i])
		}
	}
	if f.Name != "<init>" && !f.Suspend {
		// A suspend function's result is an Object, whatever its type.
		f.Ret = boxedOnJVM(f.Ret, ret)
	}
}

// composeShape marks f, a function of the class path, composable if its
// descriptor takes a Composer after the receiver and parameters it
// declares, then ints, as Compose's compiler plugin compiles it: the
// change flags of its receivers and parameters, and the masks of the
// omitted arguments if any has a default value. One whose ints aren't
// those is unsupported. member is set on a method, whose object is a
// receiver too.
func composeShape(f *types.FunSym, member bool) {
	if f.Desc == "" {
		return
	}
	ps, _ := jvmParams(f.Desc)
	n := len(f.Contexts) + len(f.Params)
	if f.Ext {
		n++
	}
	if len(ps) <= n || ps[n] != types.ComposerDesc {
		return
	}
	ints := ps[n+1:]
	for _, p := range ints {
		if p != "I" {
			return
		}
	}
	slots := n
	if member {
		slots++
	}
	defaults := false
	for _, p := range f.Params {
		defaults = defaults || p.Default
	}
	f.Composable, f.Changed = true, types.ChangedInts(slots)
	switch {
	case defaults && member && len(ints) == f.Changed:
		// An open member's: see ComposeDefaults.
		f.ComposeDefaults = true
	case defaults:
		f.Defaults = types.DefaultInts(len(f.Params))
	}
	if len(ints) != f.Changed+f.Defaults {
		f.Unsupported, f.Feature = true, "compose"
		f.Why = "'" + f.Name + "' is a composable function whose parameters kc doesn't know"
	}
}

// composeTypes makes f, a function of the class path, unsupported if it
// takes or returns a function type whose interface on the JVM isn't the
// one kc gives it: a composable function type that its metadata doesn't
// mark @Composable, whose interface takes a Composer and ints too.
func composeTypes(f *types.FunSym) {
	if f.Desc == "" {
		return
	}
	ps, ret := jvmParams(f.Desc)
	found := composeType(f.Ret, ret)
	if f.Ext && len(ps) > 0 {
		found = found || composeType(f.Recv, ps[0])
		ps = ps[1:]
	}
	for i, p := range f.Params {
		found = found || i < len(ps) && composeType(p.Type, ps[i])
	}
	if found {
		f.Unsupported, f.Feature = true, "compose"
		f.Why = "'" + f.Name + "' has a function type whose interface kc doesn't know"
	}
}

// composeType reports whether t, a function type, is a composable one,
// whose JVM type jvm is another FunctionN interface.
func composeType(t types.Type, jvm string) bool {
	const fn = "Lkotlin/jvm/functions/Function"
	if t.K != types.Class || !t.C.Func || !strings.HasPrefix(jvm, fn) {
		return false
	}
	d, ok := types.Desc(t)
	return ok && strings.HasPrefix(d, fn) && d != jvm
}

// zeroable reports whether kc can pass the zero value that stands for an
// omitted argument to each parameter of a type it doesn't support.
func zeroable(f *types.FunSym) bool {
	jvm, _ := jvmParams(f.Desc)
	if f.Ext && len(jvm) > 0 {
		jvm = jvm[1:]
	}
	for k, p := range f.Params {
		if p.Type.K == types.Invalid && k >= len(jvm) {
			return false
		}
	}
	return true
}

// kprop creates the accessors of a Kotlin property, or returns nil if kc
// can't use it.
func (t *classTable) kprop(p *kmeta.Property, owner *types.ClassSym, scope tscope, methods map[string]bool, getter, setter func(*kmeta.Property) *kmeta.JVMSig) *types.KProp {
	t = t.root()
	friend := t.friend(owner.Name)
	if !visibleOutside(p.Flags.Visibility(), friend) || !visibleOutside(p.GetterFlags.Visibility(), friend) || p.Recv != nil || p.Contexts > len(p.ContextParams) || len(p.TypeParams) > 0 {
		return nil
	}
	g := getter(p)
	if g == nil || methods != nil && !hasMethod(methods, g.Name+g.Desc, p.GetterFlags>>8&1 != 0) {
		return nil
	}
	pt, ok := t.ktype(p.Type, scope)
	if !ok {
		return nil
	}
	ctx, ok := t.kparams(p.ContextParams, scope, true)
	if !ok {
		return nil
	}
	abstract := p.Flags.Modality() == kmeta.Abstract
	kp := &types.KProp{Getter: &types.FunSym{Name: g.Name, Owner: owner, Ret: pt, Desc: g.Desc, Resolved: true, Abstract: abstract,
		Protected: p.GetterFlags.Visibility() == kmeta.Protected, Contexts: ctx}}
	kp.Getter.Declared = inlineAccessor(p.GetterFlags)
	matchJVM(kp.Getter)
	composeShape(kp.Getter, true)
	if friend && !p.IsVar() && p.Flags.Modality() == kmeta.Final && p.GetterFlags&kmeta.AccessorNotDefault == 0 && p.Flags&kmeta.PropDelegated == 0 {
		kp.Stable = &types.VarSym{Name: p.Name, Type: pt, Kind: types.Prop, Owner: owner, Resolved: true, Final: true}
	}
	if s := setter(p); p.IsVar() && s != nil && visibleOutside(p.SetterFlags.Visibility(), friend) && (methods == nil || hasMethod(methods, s.Name+s.Desc, p.SetterFlags>>8&1 != 0)) {
		kp.Setter = &types.FunSym{Name: s.Name, Owner: owner, Ret: types.UnitType, Desc: s.Desc, Resolved: true, Abstract: abstract,
			Protected: p.SetterFlags.Visibility() == kmeta.Protected, Params: []*types.VarSym{{Name: "value", Type: pt, Kind: types.Param}}, Contexts: ctx,
			Declared: inlineAccessor(p.SetterFlags)}
		matchJVM(kp.Setter)
	}
	return kp
}

// visibleOutside reports whether a class member's visibility lets other
// modules use it: public, or protected, in subclasses, or internal in a
// friend path's class.
func visibleOutside(v kmeta.Visibility, friend bool) bool {
	return v == kmeta.Public || v == kmeta.Protected || friend && v == kmeta.Internal
}

// public reports whether a top-level declaration's or a member
// extension's visibility lets other modules use it: public, or internal in
// a friend path.
func public(v kmeta.Visibility, friend bool) bool {
	return v == kmeta.Public || friend && v == kmeta.Internal
}

// friend reports whether a class is in a friend path, whose internal
// declarations the module sees, or, with the serialization plugin, one of
// kotlinx.serialization's whose internal declarations its code uses: those
// of its package internal, and UnknownFieldException.
func (t *classTable) friend(class string) bool {
	return t.cp != nil && (t.cp.Friend(class) || t.serialization &&
		(strings.HasPrefix(class, "kotlinx/serialization/internal/") || class == "kotlinx/serialization/UnknownFieldException"))
}

// inlineAccessor returns the Declared of a property accessor of the class
// path with flags: set if it is inline. inlineIn sets its Class.
func inlineAccessor(flags kmeta.Flags) *types.InlineFlags {
	if flags>>8&1 == 0 {
		return nil
	}
	return &types.InlineFlags{}
}

// inlineIn sets the Declared.Class of the inline functions and accessors
// among fs, which the metadata of class file class declares.
func inlineIn(class string, fs ...*types.FunSym) {
	for _, f := range fs {
		if f != nil && f.Declared != nil {
			f.Declared.Class = class
		}
	}
}

// memberExtProp returns the getter of an extension property declared in
// a class of the class path, as Duration.Companion's Int.milliseconds,
// or nil. It is a method of the class that takes the extension receiver.
// An inline-only getter is private: its body is copied.
func (t *classTable) memberExtProp(p *kmeta.Property, owner *types.ClassSym, scope tscope, methods map[string]bool) *types.FunSym {
	t = t.root()
	g := p.Getter
	inlineOnly := g != nil && !hasMethod(methods, g.Name+g.Desc, p.GetterFlags>>8&1 != 0) && p.GetterFlags>>8&1 != 0
	friend := t.friend(owner.Name)
	if !visibleOutside(p.Flags.Visibility(), friend) || !visibleOutside(p.GetterFlags.Visibility(), friend) || g == nil ||
		!hasMethod(methods, g.Name+g.Desc, p.GetterFlags>>8&1 != 0) && !inlineOnly || p.Contexts > len(p.ContextParams) {
		return nil
	}
	sym := &types.FunSym{Name: g.Name, Owner: owner, Desc: g.Desc, Ext: true, Resolved: true,
		Abstract: p.Flags.Modality() == kmeta.Abstract, Protected: p.GetterFlags.Visibility() == kmeta.Protected, Declared: inlineAccessor(p.GetterFlags)}
	if inlineOnly {
		sym.Inline = &types.InlineInfo{Name: g.Name, Desc: g.Desc, Private: true, Class: owner.Name, Load: t.loadInline(owner.Name)}
	}
	scope = maps.Clone(scope)
	tps, ok := t.ktypeParams(p.TypeParams, sym, scope)
	recv, rok := t.ktype(p.Recv, scope)
	ret, tok := t.ktype(p.Type, scope)
	ctx, cok := t.kparams(p.ContextParams, scope, true)
	if !ok || !rok || !tok || !cok {
		return nil
	}
	sym.TypeParams, sym.Recv, sym.Ret, sym.Contexts = tps, recv, ret, ctx
	if owner.Value {
		sym.Impl, sym.Facade = true, owner.Name // a static method, as a value class's other members
	}
	matchJVM(sym)
	composeShape(sym, true)
	if s := p.Setter; p.IsVar() && s != nil && visibleOutside(p.SetterFlags.Visibility(), friend) && hasSetter(p, methods) {
		set := &types.FunSym{Name: s.Name, Owner: owner, Desc: s.Desc, Ext: true, Resolved: true, Abstract: sym.Abstract,
			Protected: p.SetterFlags.Visibility() == kmeta.Protected, TypeParams: tps, Recv: recv, Ret: types.UnitType,
			Params: []*types.VarSym{{Name: "value", Type: ret, Kind: types.Param}}, Contexts: ctx, Declared: inlineAccessor(p.SetterFlags)}
		if inlineOnlySetter(p, methods) {
			set.Inline = &types.InlineInfo{Name: s.Name, Desc: s.Desc, Private: true, Class: owner.Name, Load: t.loadInline(owner.Name)}
		}
		if owner.Value {
			set.Impl, set.Facade = true, owner.Name
		}
		matchJVM(set)
		sym.ExtSetter = set
	}
	return sym
}

// companionFields returns the types of a companion's properties that are
// fields of its class, by their field names: its const and @JvmField
// properties, which have no getter. It also returns the fields of its
// properties that have one, as accessorFields does.
func (t *classTable) companionFields(name string) (map[string]*kmeta.Type, map[string]bool) {
	t = t.root()
	info, err := t.cp.Load(name)
	if err != nil || info.Kotlin == nil {
		return nil, nil
	}
	kc, err := kmeta.ReadClass(info.Kotlin)
	if err != nil {
		return nil, nil
	}
	m := map[string]*kmeta.Type{}
	for _, p := range kc.Props {
		if p.Getter == nil && p.Field != nil && p.Recv == nil && len(p.TypeParams) == 0 {
			m[p.Field.Name] = p.Type
		}
	}
	return m, accessorFields(kc)
}

// accessorFields returns the names of the fields of a class's properties
// that have a getter: a lateinit property's field is as visible as its
// setter, but Kotlin code reads and writes it through the accessors.
func accessorFields(kc *kmeta.Class) map[string]bool {
	m := map[string]bool{}
	for _, p := range kc.Props {
		if p.Getter != nil && p.Field != nil {
			m[p.Field.Name] = true
		}
	}
	return m
}

func metaGetter(p *kmeta.Property) *kmeta.JVMSig { return p.Getter }
func metaSetter(p *kmeta.Property) *kmeta.JVMSig { return p.Setter }

// lowPriority returns the name+desc of a class file's methods marked
// @LowPriorityInOverloadResolution, or nil.
func lowPriority(info *classfile.Info) map[string]bool {
	var m map[string]bool
	for _, f := range info.Methods {
		if f.LowPriority {
			if m == nil {
				m = map[string]bool{}
			}
			m[f.Name+f.Desc] = true
		}
	}
	return m
}

// annotatedMethods returns a class file's methods with annotations or a
// deprecation, by name+desc, or nil.
func annotatedMethods(info *classfile.Info) map[string]*classfile.Member {
	var m map[string]*classfile.Member
	for i := range info.Methods {
		f := &info.Methods[i]
		if len(f.Annotations) > 0 || f.Deprecated != nil {
			if m == nil {
				m = map[string]*classfile.Member{}
			}
			m[f.Name+f.Desc] = f
		}
	}
	return m
}

// defaultImplsAnnots adds to annots, the annotated methods of interface
// cls, those of its DefaultImpls class, where an interface compiled with
// -jvm-default=disable has its properties' $annotations methods, if one
// of kc's properties has such a method that annots lacks.
func (t *classTable) defaultImplsAnnots(cls *types.ClassSym, kc *kmeta.Class, annots map[string]*classfile.Member) map[string]*classfile.Member {
	for _, p := range kc.Props {
		if p.Synthetic == nil || annots[p.Synthetic.Name+p.Synthetic.Desc] != nil {
			continue
		}
		info, err := t.cp.Load(cls.Name + "$DefaultImpls")
		if err != nil {
			return annots
		}
		for k, m := range annotatedMethods(info) {
			if annots == nil {
				annots = map[string]*classfile.Member{}
			}
			if annots[k] == nil {
				annots[k] = m
			}
		}
		return annots
	}
	return annots
}

// annotate gives a Kotlin function of the class path its method's
// annotations and @Deprecated, if m isn't nil.
func annotate(f *types.FunSym, m *classfile.Member) {
	if m == nil {
		return
	}
	// kotlinc writes nullability annotations for Java, which Kotlin
	// doesn't see on its own declarations.
	f.Annotations = slices.DeleteFunc(slices.Clone(m.Annotations), func(a string) bool {
		return a == "Lorg/jetbrains/annotations/NotNull;" || a == "Lorg/jetbrains/annotations/Nullable;"
	})
	if len(f.Annotations) == 0 {
		f.Annotations = nil
	}
	if m.Deprecated != nil && !m.Deprecated.Java {
		f.Deprecated = classFileDeprecation(m.Deprecated)
	}
}

// annotateProp gives the getter and setter of a property of the class
// path, which may be nil, their methods' annotations and @Deprecated, and
// the property's, which kotlinc keeps on its $annotations method.
func annotateProp(p *kmeta.Property, annots map[string]*classfile.Member, get, set *types.FunSym) {
	var prop *classfile.Member
	if p.Synthetic != nil {
		prop = annots[p.Synthetic.Name+p.Synthetic.Desc]
	}
	for i, f := range []*types.FunSym{get, set} {
		if f == nil {
			continue
		}
		if sig := []*kmeta.JVMSig{p.Getter, p.Setter}[i]; sig != nil {
			annotate(f, annots[sig.Name+sig.Desc])
		}
		if prop != nil {
			f.PropAnnotations = prop.Annotations
			if prop.Deprecated != nil && !prop.Deprecated.Java {
				f.Deprecated = classFileDeprecation(prop.Deprecated)
			}
		}
	}
}

// aliasRequires reports whether a typealias of the class path may have
// an annotation that its uses must check: @Deprecated, or one that may be
// an opt-in requirement marker. Such an alias of a class is kept as an
// alias, which the checks see, rather than resolved as the class.
func aliasRequires(a *kmeta.TypeAlias) bool {
	for _, an := range a.Annotations {
		if !neverMarker("L" + internalName(an.Class) + ";") {
			return true
		}
	}
	return false
}

// neverMarker reports whether the annotation with descriptor desc is one
// of those that aren't opt-in requirement markers, which the checks then
// needn't load: Kotlin's and Java's own, but kotlin.Deprecated and those
// of the kotlin package that may be markers, and JetBrains' nullability
// annotations.
func neverMarker(desc string) bool {
	switch desc {
	case "Lkotlin/Deprecated;", "Lkotlin/DeprecatedSinceKotlin;":
		return false
	case "Lkotlin/SinceKotlin;", "Lkotlin/WasExperimental;", "Lkotlin/PublishedApi;", "Lkotlin/BuilderInference;",
		"Lkotlin/OverloadResolutionByLambdaReturnType;", "Lkotlin/IgnorableReturnValue;", "Lkotlin/ReplaceWith;":
		return true
	}
	for _, p := range []string{"Lkotlin/jvm/", "Lkotlin/internal/", "Lkotlin/annotation/", "Lkotlin/js/", "Ljava/lang/", "Lorg/jetbrains/annotations/", "Ljavax/annotation/", "Landroidx/annotation/"} {
		if strings.HasPrefix(desc, p) {
			return true
		}
	}
	return false
}

// aliasAnnotations gives a typealias of the class path the annotations
// and @Deprecated its metadata keeps.
func aliasAnnotations(s *types.AliasSym, a *kmeta.TypeAlias) {
	var d *types.Deprecation
	since, notYet := "", false // the level @DeprecatedSinceKotlin gives
	for _, an := range a.Annotations {
		s.Annotations = append(s.Annotations, "L"+internalName(an.Class)+";")
		switch an.Class {
		case "kotlin/Deprecated":
			d = &types.Deprecation{Level: "WARNING"}
			for _, arg := range an.Args {
				switch arg.Name {
				case "message":
					d.Message = arg.Value.Str
				case "level":
					d.Level = arg.Value.Str
				}
			}
		case "kotlin/DeprecatedSinceKotlin":
			for _, arg := range an.Args {
				switch {
				case classfile.LaterThanAPI(arg.Value.Str):
					// A version the API version hasn't reached.
					notYet = notYet || arg.Name == "warningSince"
				case arg.Name == "hiddenSince":
					since = "HIDDEN"
				case arg.Name == "errorSince" && since == "":
					since = "ERROR"
				}
			}
		}
	}
	if notYet {
		d = nil
	}
	if d != nil && (since == "HIDDEN" || since == "ERROR" && d.Level == "WARNING") {
		d.Level = since
	}
	s.Deprecated = d
}

// publicMethods returns the name+desc of a class file's public and
// protected methods.
// hiddenMethods returns the name+desc of a class's methods deprecated
// with level HIDDEN.
func hiddenMethods(info *classfile.Info) map[string]bool {
	var m map[string]bool
	for _, f := range info.Methods {
		if f.Hidden {
			if m == nil {
				m = map[string]bool{}
			}
			m[f.Name+f.Desc] = true
		}
	}
	return m
}

// publicMethods returns the class's public and protected methods: true
// for one Java can call, false for a synthetic one that Kotlin can, as
// kotlinc writes a function annotated @JvmSynthetic or an inline one with
// reified type parameters. A synthetic one annotated @Deprecated(level =
// HIDDEN), which calls can't choose, is left out.
func publicMethods(info *classfile.Info) map[string]bool {
	m := make(map[string]bool, len(info.Methods))
	for _, f := range info.Methods {
		if f.Access&(classfile.AccPublic|classfile.AccProtected) == 0 || f.Hidden && f.Access&classfile.AccSynthetic != 0 {
			continue
		}
		m[f.Name+f.Desc] = f.Access&classfile.AccSynthetic == 0
	}
	return m
}

// hasMethod reports whether methods has a method of signature sig that
// a Kotlin call can call: a synthetic one, unless it is inline, whose
// method doesn't know its reified type parameters and whose body is
// copied.
func hasMethod(methods map[string]bool, sig string, inline bool) bool {
	v, ok := methods[sig]
	return v || ok && !inline
}

// hasSetter reports whether a call can set var extension property p,
// whose methods are these: through its setter's method, or a copy of its
// body, an inline setter's whose method is synthetic or private, as one
// with reified type parameters.
func hasSetter(p *kmeta.Property, methods map[string]bool) bool {
	s := p.Setter
	return hasMethod(methods, s.Name+s.Desc, p.SetterFlags>>8&1 != 0) || inlineOnlySetter(p, methods)
}

// inlineOnlySetter reports whether a set of p copies its inline setter's
// body, as its method can't be called.
func inlineOnlySetter(p *kmeta.Property, methods map[string]bool) bool {
	s := p.Setter
	inline := p.SetterFlags>>8&1 != 0
	return inline && !hasMethod(methods, s.Name+s.Desc, inline)
}

const markerSuffix = "Lkotlin/jvm/internal/DefaultConstructorMarker;)V"

// markerCtor reports whether the class has a public or protected
// synthetic constructor of descriptor d, which takes a marker last.
func markerCtor(info *classfile.Info, d string) bool {
	if !strings.HasSuffix(d, markerSuffix) {
		return false
	}
	for _, f := range info.Methods {
		if f.Name == "<init>" && f.Desc == d && f.Access&(classfile.AccPublic|classfile.AccProtected) != 0 {
			return true
		}
	}
	return false
}

// loadKotlin fills in a Kotlin class from its metadata.
func (t *classTable) loadKotlin(cls *types.ClassSym, info *classfile.Info, kc *kmeta.Class) {
	t = t.root()
	start := time.Now()
	cls.KotlinName = kc.Name
	cls.Interface = info.Access&classfile.AccInterface != 0
	cls.Fun = cls.Interface && kc.Flags&kmeta.ClassFun != 0
	cls.RestrictsSuspension = slices.Contains(info.Invisible, "Lkotlin/coroutines/RestrictsSuspension;")
	cls.JVMDefault = cls.Interface && kc.JVMFlags&kmeta.JVMDefaultMode != 0
	cls.JVMCompat = cls.JVMDefault && kc.JVMFlags&kmeta.JVMCompatibilityMode != 0
	t.valueOf(cls, kc)
	if kc.Flags.Modality() == kmeta.Sealed {
		// A when over it is exhaustive if it covers these.
		cls.Sealed = true
		for _, id := range kc.SealedSubclasses {
			if sub := t.class(internalName(id)); sub != nil {
				cls.Subclasses = append(cls.Subclasses, sub)
			}
		}
	}
	mod := kc.Flags.Modality()
	cls.Abstract = cls.Interface || mod == kmeta.Abstract || mod == kmeta.Sealed
	cls.Final = !cls.Interface && mod == kmeta.Final
	cls.Object = kc.Kind() == kmeta.KindObject || kc.Kind() == kmeta.KindCompanion
	cls.Data = kc.Flags&kmeta.ClassData != 0
	if kc.IsInner() && info.Inner != nil && info.Inner.Outer != "" {
		// An inner class: its constructors take the outer class's
		// instance first, which a subclass's call of one passes, as the
		// program's inner classes' do. It sees the outer class's type
		// parameters, which it has after its own (see the scope below).
		if o := t.class(info.Inner.Outer); o != nil {
			cls.Inner, cls.Outer = true, o
		}
	}
	if kc.Companion != "" {
		// The companion object is a nested class, whose instance is in a
		// static field of this class named after it.
		if comp := t.class(cls.Name + "$" + kc.Companion); comp != nil {
			cls.Companion, comp.CompanionOf = comp, cls
		}
	}
	// A protected nested class is public on the JVM, for subclasses:
	// clikt's AbstractHelpFormatter.RenderedSection.
	friend := t.friend(cls.Name)
	cls.Private = info.Access&classfile.AccPublic == 0 || !visibleOutside(kc.Flags.Visibility(), friend) && !builtinCompanions[cls.Name]
	cls.Internal = info.Access&classfile.AccPublic != 0 && kc.Flags.Visibility() == kmeta.Internal
	scope := tscope{}
	var outerParams []*types.TypeParamSym
	if cls.Inner && cls.Outer != nil {
		// The outer class's, by the ids of its metadata, which the inner
		// class's numbers its own after.
		os, found := t.scopes.Load(cls.Outer)
		if !found {
			cls.Outer.Ensure()
			os, found = t.scopes.Load(cls.Outer)
		}
		if found {
			maps.Copy(scope, os.(tscope))
			outerParams = cls.Outer.TypeParams
		}
	}
	var tps []*types.TypeParamSym
	ok := true
	if early := cls.TypeParams; cls.Value && len(early) == len(kc.TypeParams) && len(early) > 0 {
		// valueOf's, which the underlying value's type uses, with their
		// complete bounds now.
		for i, p := range kc.TypeParams {
			scope[p.ID] = early[i]
		}
		for i, p := range kc.TypeParams {
			var bounds []types.Type
			for _, b := range p.Bounds {
				bt, bok := t.ktype(b, scope)
				if !bok {
					ok = false
					continue
				}
				bounds = append(bounds, bt)
			}
			early[i].Bounds = bounds
		}
		tps = early
	} else {
		tps, ok = t.ktypeParams(kc.TypeParams, cls, scope)
	}
	if len(outerParams) > 0 {
		tps = append(tps[:len(tps):len(tps)], outerParams...)
		cls.OuterParams = len(outerParams)
	}
	cls.TypeParams = tps
	t.scopes.Store(cls, maps.Clone(scope))
	if kc.IsValue() && cls.Underlying == nil && kc.Underlying != nil {
		if ut, uok := t.ktype(kc.Underlying, scope); uok {
			cls.Value = true
			cls.Underlying = &types.VarSym{Name: kc.UnderlyingProp, Type: ut, Kind: types.Prop, Owner: cls, Resolved: true, Final: true}
		}
	}
	if info.Super != "" {
		if cls.Super = t.class(info.Super); cls.Super == nil {
			cls.Super = t.missingClass(info.Super, 0)
		}
	}
	var supers []types.Type
	for _, st := range kc.Supers {
		s, sok := t.ktype(st, scope)
		switch {
		case !sok:
			ok = false
		case s.K == types.Class:
			supers = append(supers, s)
		}
	}
	if ok {
		cls.Supers = supers
		for _, s := range supers {
			if s.C != cls.Super {
				cls.Interfaces = append(cls.Interfaces, s.C)
			}
		}
	} else {
		cls.TypeParams = nil
		for _, name := range info.Interfaces {
			if i := t.class(name); i != nil {
				cls.Interfaces = append(cls.Interfaces, i)
			}
		}
	}
	for _, a := range kc.Aliases {
		if !public(a.Flags.Visibility(), friend) {
			continue
		}
		e := a.Expanded
		if e == nil {
			e = a.Underlying
		}
		s := &types.AliasSym{Name: a.Name, Outer: cls, Resolved: true}
		aliasAnnotations(s, a)
		ascope := tscope{}
		atps, aok := t.ktypeParams(a.TypeParams, s, ascope)
		if at, tok := t.ktype(e, ascope); aok && tok {
			s.TypeParams, s.Type = atps, at
			if cls.Aliases == nil {
				cls.Aliases = map[string]*types.AliasSym{}
			}
			cls.Aliases[a.Name] = s
		}
	}
	cls.Methods = map[string][]*types.FunSym{}
	cls.Statics = map[string][]*types.FunSym{}
	cls.Fields = map[string]*types.VarSym{}
	cls.KProps = map[string]*types.KProp{}
	if kc.Kind() == kmeta.KindEnum {
		// An enum's entries are its static fields marked ACC_ENUM.
		cls.Entries = []string{}
		for _, fd := range info.Fields {
			if fd.Access&classfile.AccEnum != 0 {
				cls.Entries = append(cls.Entries, fd.Name)
				cls.Fields[fd.Name] = &types.VarSym{Name: fd.Name, Type: types.ClassType(cls), Kind: types.Static, Owner: cls, Resolved: true,
					Annotations: fd.Annotations}
			}
		}
		// Its static methods, as for the program's enums.
		t := types.ClassType(cls)
		str := &types.VarSym{Name: "value", Type: types.StringType, Kind: types.Param}
		cls.Statics["valueOf"] = []*types.FunSym{{Name: "valueOf", Owner: cls, Static: true, Params: []*types.VarSym{str}, Ret: t, Resolved: true}}
		cls.Statics["values"] = []*types.FunSym{{Name: "values", Owner: cls, Static: true, Ret: types.ArrayOf(t), Resolved: true}}
	}
	if kc.Companion != "" {
		// The companion's const and @JvmField properties are public static
		// fields of this class, of the types the companion's metadata
		// gives them: ProtoAdapter<ByteString> for ProtoAdapter.BYTES.
		props, accessors := t.companionFields(cls.Name + "$" + kc.Companion)
		for _, fd := range info.Fields {
			if fd.Access&classfile.AccPublic == 0 || fd.Access&classfile.AccStatic == 0 || fd.Access&classfile.AccSynthetic != 0 || fd.Name == kc.Companion || accessors[fd.Name] {
				continue
			}
			ft, ok := t.fieldType(fd.Desc)
			if kt := props[fd.Name]; kt != nil {
				if pt, pok := t.ktype(kt, tscope{}); pok {
					ft, ok = pt, true
				}
			}
			if ok && ft.K != types.Unit {
				cls.Fields[fd.Name] = &types.VarSym{Name: fd.Name, Type: ft, Kind: types.Static, Owner: cls, Resolved: true,
					Constant: fieldConstant(fd), Mutable: fd.Access&classfile.AccFinal == 0}
			}
		}
	}
	if k := kc.Kind(); k == kmeta.KindObject || k == kmeta.KindCompanion && builtinCompanions[cls.Name] {
		// An object's const and @JvmField properties are its public static
		// fields: IntCompanionObject.MAX_VALUE.
		accessors := accessorFields(kc)
		for _, fd := range info.Fields {
			if fd.Access&classfile.AccPublic == 0 || fd.Access&classfile.AccStatic == 0 || fd.Access&classfile.AccSynthetic != 0 || fd.Name == "INSTANCE" || accessors[fd.Name] {
				continue
			}
			if ft, ok := t.fieldType(fd.Desc); ok && ft.K != types.Unit {
				cls.Fields[fd.Name] = &types.VarSym{Name: fd.Name, Type: ft, Kind: types.Static, Owner: cls, Resolved: true,
					Constant: fieldConstant(fd), Mutable: fd.Access&classfile.AccFinal == 0}
			}
		}
	}
	if cls.TypeParams == nil && len(kc.TypeParams) > 0 {
		return // members would refer to the type parameters
	}
	methods := publicMethods(info)
	// The members of an object annotated @JvmStatic are static methods.
	statics := map[string]bool{}
	if cls.Object {
		for _, m := range info.Methods {
			if m.Access&classfile.AccStatic != 0 {
				statics[m.Name+m.Desc] = true
			}
		}
	}
	annots := annotatedMethods(info)
	if cls.Interface {
		annots = t.defaultImplsAnnots(cls, kc, annots)
	}
	hidden := map[string]bool{}
	for _, m := range info.Methods {
		if m.Access&classfile.AccPublic != 0 && m.Access&classfile.AccSynthetic != 0 {
			hidden[m.Name+m.Desc] = true
		}
	}
	invisible := func(key string, flags kmeta.Flags) {
		if cls.InvisibleMembers == nil {
			cls.InvisibleMembers = map[string]bool{}
		}
		cls.InvisibleMembers[key] = flags.Modality() == kmeta.Abstract
	}
	for _, f := range kc.Funcs {
		if f.Flags.Visibility() == kmeta.Internal && !friend && f.Flags.MemberKind() != 1 && f.Recv == nil {
			invisible(f.Name+"/"+strconv.Itoa(len(f.Params)), f.Flags)
		}
		if !visibleOutside(f.Flags.Visibility(), friend) || f.Flags.MemberKind() == 1 /* fake override */ {
			continue
		}
		sym := t.kfun(f, cls, scope, methods)
		annotate(sym, annots[f.JVM.Name+f.JVM.Desc])
		if sym.Unsupported && sym.Inline == nil && hidden[f.JVM.Name+f.JVM.Desc] {
			// @Deprecated(level = HIDDEN): a synthetic method, which
			// calls can't choose but delegation implements.
			sym = t.kfun(f, cls, scope, nil)
			sym.Hidden = true
		}
		inlineIn(cls.Name, sym)
		if cls.Value && f.Flags.MemberKind() != 3 /* synthesized: equals, hashCode */ {
			// A value class's member is a static method of the class,
			// which takes the value first; an inline one is copied, as
			// with kotlinc, so that a lambda passed to it can return from
			// the caller.
			sym.Impl, sym.Facade = true, cls.Name
			if sym.Inline == nil && f.IsInline() && !sym.Unsupported && sym.Builtin == types.NotBuiltin {
				sym.Inline = inlineInfo(f, false)
			}
			if sym.Inline != nil {
				sym.Inline.Load = t.loadInline(cls.Name)
				sym.Unsupported, sym.Why = false, ""
			}
		} else if sym.Inline != nil && !cls.Interface {
			// An inline member with reified type parameters, whose
			// method is synthetic, or an inline-only one, whose method is
			// private: it is copied, as clikt's Context.findObject<T>()
			// and Result.success(value).
			sym.Inline.Load = t.loadInline(cls.Name)
			sym.Inline.Class, sym.Inline.Owner = cls.Name, cls.Name
			sym.Unsupported, sym.Why = false, ""
		} else if f.IsInline() && !cls.Interface && !sym.Unsupported && sym.Builtin == types.NotBuiltin {
			// A public inline member is copied too, as kotlinc does, when
			// its body can be: a lambda passed to it may then suspend the
			// caller, as in kotlinx-io's UnsafeBufferOperations.readFromHead.
			sym.Inline = inlineInfo(f, false)
			sym.Inline.Load = t.loadInline(cls.Name)
			sym.Inline.Class, sym.Inline.Owner = cls.Name, cls.Name
		}
		if sym.Why != "" && !sym.Abstract {
			continue // not callable on the JVM
		}
		sym.Protected = f.Flags.Visibility() == kmeta.Protected
		sym.Final = cls.Final || !cls.Interface && f.Flags.Modality() == kmeta.Final
		sym.Static = statics[f.JVM.Name+f.JVM.Desc]
		if sym.Ext {
			// A member extension: called with the class as its dispatch
			// receiver, as ParameterizedTypeName.Companion's parameterizedBy.
			if cls.ExtMethods == nil {
				cls.ExtMethods = map[string][]*types.FunSym{}
			}
			cls.ExtMethods[f.Name] = append(cls.ExtMethods[f.Name], sym)
			continue
		}
		cls.Methods[f.Name] = append(cls.Methods[f.Name], sym)
	}
	instanceFields := map[string]bool{}
	for _, fd := range info.Fields {
		if fd.Access&classfile.AccPublic != 0 && fd.Access&classfile.AccStatic == 0 {
			instanceFields[fd.Name] = true
		}
	}
	for _, p := range kc.Props {
		if p.Flags.Visibility() == kmeta.Internal && !friend && p.Flags.MemberKind() != 1 && p.Recv == nil {
			invisible("prop "+p.Name, p.Flags)
		}
		if p.Getter == nil && p.Field != nil && p.Recv == nil && public(p.Flags.Visibility(), friend) && instanceFields[p.Field.Name] {
			// A @JvmField property: a public field and no accessors, as
			// Buffer.UnsafeCursor's offset.
			if pt, ok := t.ktype(p.Type, scope); ok {
				if cls.JavaFields == nil {
					cls.JavaFields = map[string]*types.VarSym{}
				}
				cls.JavaFields[p.Name] = &types.VarSym{Name: p.Field.Name, Type: pt, Kind: types.Prop, Owner: cls, Resolved: true, Final: true,
					JvmField: true, Mutable: p.IsVar()}
			}
			continue
		}
		if p.Recv != nil {
			if g := t.memberExtProp(p, cls, scope, methods); g != nil {
				inlineIn(cls.Name, g, g.ExtSetter)
				annotateProp(p, annots, g, g.ExtSetter)
				g.Static = statics[g.JVM()+g.Desc]
				if g.ExtSetter != nil {
					g.ExtSetter.Static = statics[g.ExtSetter.JVM()+g.ExtSetter.Desc]
				}
				if cls.ExtProps == nil {
					cls.ExtProps = map[string][]*types.FunSym{}
				}
				cls.ExtProps[p.Name] = append(cls.ExtProps[p.Name], g)
			}
			continue
		}
		if kp := t.kprop(p, cls, scope, methods, metaGetter, metaSetter); kp != nil {
			inlineIn(cls.Name, kp.Getter, kp.Setter)
			annotateProp(p, annots, kp.Getter, kp.Setter)
			if cls.Value {
				kp.Getter.Impl, kp.Getter.Facade = true, cls.Name
			}
			kp.Getter.Static = statics[kp.Getter.JVM()+kp.Getter.Desc]
			if kp.Setter != nil {
				kp.Setter.Static = statics[kp.Setter.JVM()+kp.Setter.Desc]
			}
			if prev := cls.KProps[p.Name]; prev != nil && !sameContexts(prev.Getter.Contexts, kp.Getter.Contexts) {
				addKAlt(prev, kp) // one with other context parameters
				continue
			}
			cls.KProps[p.Name] = kp
		}
	}
	if kc.Kind() == kmeta.KindAnnotation && len(kc.Ctors) > 0 {
		// An annotation class's constructor has no method: its parameters
		// are the annotation's.
		cls.AnnotationParams, _ = t.kparams(kc.Ctors[0].Params, scope, true)
		if cls.AnnotationParams == nil {
			cls.AnnotationParams = []*types.VarSym{} // marks a Kotlin annotation class
		}
		for _, m := range info.Methods {
			if m.Default != nil {
				if cls.AnnotationDefaults == nil {
					cls.AnnotationDefaults = map[string]*classfile.ElementValue{}
				}
				cls.AnnotationDefaults[m.Name] = m.Default
			}
		}
	}
	if kc.Kind() == kmeta.KindClass {
		// An abstract class's constructors are for its subclasses.
		for _, k := range kc.Ctors {
			internal := !visibleOutside(k.Flags.Visibility(), friend) && k.Flags.Visibility() == kmeta.Internal
			if !visibleOutside(k.Flags.Visibility(), friend) && !internal {
				continue
			}
			d, marker := k.JVM.Desc, false
			if internal && cls.Value {
				// Its constructor-impl, which is public.
			} else if !methods[k.JVM.Name+d] {
				// One that takes a value class, as Compose's
				// SolidColor(Color), is the synthetic one that takes a
				// marker: the metadata names it.
				if !markerCtor(info, d) {
					continue
				}
				d, marker = strings.TrimSuffix(d, markerSuffix)+")V", true
			}
			f := &types.FunSym{Name: "<init>", Owner: cls, Desc: d, Ret: types.ClassType(cls), Resolved: true,
				Protected: k.Flags.Visibility() == kmeta.Protected, Marker: marker}
			var ok bool
			f.Params, ok = t.kparams(k.Params, scope, true)
			f.Unsupported = !ok
			matchJVM(f)
			annotate(f, annots[k.JVM.Name+k.JVM.Desc])
			f.Hidden = f.Deprecated != nil && f.Deprecated.Level == "HIDDEN"
			if internal {
				cls.InternalCtors = append(cls.InternalCtors, f)
				continue
			}
			cls.Ctors = append(cls.Ctors, f)
		}
	}
	if cls.Data {
		componentMarkers(kc, cls, annots)
	}
	t.timed(start, len(kc.Funcs)+len(kc.Props)+len(kc.Ctors))
}

// componentMarkers gives a data class's componentN functions the
// annotations of the properties of its primary constructor they return,
// whose opt-in requirements they have, as K2's.
func componentMarkers(kc *kmeta.Class, cls *types.ClassSym, annots map[string]*classfile.Member) {
	for _, k := range kc.Ctors {
		if k.Flags&kmeta.CtorSecondary != 0 {
			continue
		}
		for i, prm := range k.Params {
			for _, p := range kc.Props {
				if p.Name != prm.Name || p.Synthetic == nil {
					continue
				}
				m := annots[p.Synthetic.Name+p.Synthetic.Desc]
				if m == nil {
					continue
				}
				for _, f := range cls.Methods["component"+strconv.Itoa(i+1)] {
					if len(f.Params) == 0 {
						f.PropAnnotations = m.Annotations
					}
				}
			}
		}
	}
}

// internalName returns the JVM internal name of a class id: nested
// classes separated by '$'.
func internalName(id string) string { return strings.ReplaceAll(id, ".", "$") }

// erasedDesc returns the JVM descriptor of a type in the built-in
// collection types, whose type parameters have no bounds.
func erasedDesc(kt *kmeta.Type) string {
	if kt.Class == "" {
		return "Ljava/lang/Object;"
	}
	if kt.Class == "kotlin/Array" {
		// Array<T> is an array of the erasure of T, with primitives boxed.
		if len(kt.Args) == 1 && kt.Args[0].Type != nil {
			e := *kt.Args[0].Type
			e.Nullable = true
			return "[" + erasedDesc(&e)
		}
		return "[Ljava/lang/Object;"
	}
	if kt.Nullable {
		if box := types.PrimOf(types.IDKind(kt.Class)).Box; box != "" {
			return "L" + box + ";"
		}
	}
	return kmeta.MapClass(kt.Class)
}

// builtinSig returns the JVM name and descriptor of a member of a
// built-in collection type.
func builtinSig(name string, params []*kmeta.Param, ret *kmeta.Type) kmeta.JVMSig {
	if n, ok := builtinJVMNames[name]; ok {
		name = n
	}
	var b strings.Builder
	b.WriteByte('(')
	for _, p := range params {
		b.WriteString(erasedDesc(p.Type))
	}
	b.WriteByte(')')
	b.WriteString(erasedDesc(ret))
	return kmeta.JVMSig{Name: name, Desc: b.String()}
}

// loadCollection fills in a built-in collection type: its members from
// .kotlin_builtins, and the Java interface's other members.
func (t *classTable) loadCollection(cls *types.ClassSym) {
	t = t.root()
	kc := t.builtinClasses()[cls.KotlinName]
	if kc == nil {
		cls.Missing = true
		return
	}
	start := time.Now()
	cls.Interface = true
	cls.Java = t.rawClass(cls.Name)
	cls.Super = t.class("java/lang/Object")
	scope := tscope{}
	cls.TypeParams, _ = t.ktypeParams(kc.TypeParams, cls, scope)
	for _, st := range kc.Supers {
		if s, ok := t.ktype(st, scope); ok && s.K == types.Class {
			cls.Supers = append(cls.Supers, s)
			cls.Interfaces = append(cls.Interfaces, s.C)
		}
	}
	cls.Methods = map[string][]*types.FunSym{}
	cls.KProps = map[string]*types.KProp{}
	hidden := map[string]bool{}
	for _, f := range kc.Funcs {
		sig := builtinSig(f.Name, f.Params, f.Ret)
		f.JVM = sig
		hidden[sig.Name+sig.Desc] = true
		sym := t.kfun(f, cls, scope, nil)
		if n, ok := platformDependent[cls.KotlinName+"."+f.Name]; ok && n == len(f.Params) {
			// Only the JVM's interface declares it: a class's
			// remove(key, value) needn't be marked override, and
			// delegation leaves it to Java's default method.
			sym.Platform = true
		}
		cls.Methods[f.Name] = append(cls.Methods[f.Name], sym)
	}
	for _, p := range kc.Props {
		get := builtinSig(types.GetterName(p.Name), nil, p.Type)
		if n, ok := builtinJVMNames[p.Name]; ok {
			get.Name = n
		}
		hidden[get.Name+get.Desc] = true
		if kp := t.kprop(p, cls, scope, nil, func(*kmeta.Property) *kmeta.JVMSig { return &get }, metaSetter); kp != nil {
			cls.KProps[p.Name] = kp
		}
	}

	// The Java interface's members that Kotlin doesn't declare, such as
	// stream() and forEach(Consumer), with its type parameters replaced by
	// the Kotlin type's. The Java interface is loaded here; its own loading
	// only refers to other classes.
	raw := cls.Java
	raw.Ensure()
	cls.Statics, cls.Fields = raw.Statics, raw.Fields
	sub := types.Subst{}
	if len(raw.TypeParams) == len(cls.TypeParams) {
		for i, p := range raw.TypeParams {
			sub[p] = types.Arg{T: types.ParamType(cls.TypeParams[i])}
		}
	}
	readOnly := cls.ReadOnly == nil
	for name, fs := range raw.Methods {
		if readOnly && mutators[name] {
			continue
		}
		for _, f := range fs {
			if hidden[f.Name+f.Desc] || f.Static {
				continue
			}
			g := *f
			g.Owner, g.Platform = cls, true
			g.Params = make([]*types.VarSym, len(f.Params))
			for i, p := range f.Params {
				q := *p
				q.Type = sub.ApplyIn(p.Type)
				g.Params[i] = &q
			}
			g.Ret = sub.Apply(f.Ret)
			cls.Methods[name] = append(cls.Methods[name], &g)
		}
	}
	t.timed(start, len(kc.Funcs)+len(kc.Props))
}

// platform returns the names and descriptors of the members of the Java
// collection interfaces that Kotlin's collection types don't declare,
// such as forEach(Consumer). One that a Kotlin collection type declares
// isn't, though a subtype gets it from Java: MutableIterator's hasNext(),
// which Iterator declares.
func (t *classTable) platform() map[string]bool {
	t = t.root()
	t.platformOnce.Do(func() {
		t.platformSet = map[string]bool{}
		declared := map[string]bool{}
		for id := range collectionJVM {
			cls := t.kclass(id)
			cls.Ensure()
			for _, fs := range cls.Methods {
				for _, f := range fs {
					if f.Platform {
						t.platformSet[f.Name+f.Desc] = true
					} else {
						declared[f.Name+f.Desc] = true
					}
				}
			}
		}
		for k := range declared {
			delete(t.platformSet, k)
		}
	})
	return t.platformSet
}

// modules reads the .kotlin_module files on the class path.
func (t *classTable) modules() map[string][]kmeta.Part {
	t = t.root()
	t.moduleOnce.Do(func() {
		start := time.Now()
		t.parts = map[string][]kmeta.Part{}
		// Module names repeat across jars (common, or the empty module
		// files of Compose's umbrella artifacts), so every copy is read. A
		// class can still be listed twice, by a jar and its duplicate.
		seen := map[string]bool{}
		for _, name := range t.cp.Resources(".kotlin_module") {
			all, err := t.cp.ResourceAll(name)
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
							t.parts[pkg] = append(t.parts[pkg], part)
						}
					}
				}
			}
		}
		t.pkgs = make(map[string]*kpkg, len(t.parts))
		for pkg := range t.parts {
			t.pkgs[pkg] = &kpkg{name: pkg, funs: map[string][]*types.FunSym{}, props: map[string][]*types.FunSym{}, vars: map[string]*types.VarSym{},
				aliases: map[string]*types.ClassSym{}, typeAliases: map[string]*types.AliasSym{}}
		}
		t.timed(start, 0)
	})
	return t.parts
}

// isKotlinPackage reports whether a package (pkg.name) has top-level
// Kotlin declarations.
func (t *classTable) isKotlinPackage(pkg string) bool {
	t = t.root()
	if !t.kotlin {
		return false
	}
	_, ok := t.modules()[pkg]
	return ok
}

// pkg returns a package's top-level declarations, reading them on first
// use, or nil if it has none.
func (t *classTable) pkg(name string) *kpkg {
	t = t.root()
	if !t.kotlin {
		return nil
	}
	t.modules()
	return t.read(t.pkgs[name])
}

// read reads the declarations of package p, if not read yet, and returns
// it. p may be nil.
func (t *classTable) read(p *kpkg) *kpkg {
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		// A package can have dozens of classes, some large, so they are
		// read in parallel.
		parts := t.parts[p.name]
		read := make([]*kpart, len(parts))
		var wg sync.WaitGroup
		sem := make(chan struct{}, runtime.GOMAXPROCS(0))
		for i, part := range parts {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				info, err := t.cp.Load(part.Class)
				if err != nil || info.Kotlin == nil {
					return
				}
				start := time.Now()
				meta, err := kmeta.ReadPackage(info.Kotlin)
				t.timed(start, 0)
				if err != nil {
					return
				}
				facade := part.Facade
				if facade == "" {
					facade = part.Class
				}
				read[i] = &kpart{meta: meta, facade: facade, class: part.Class, methods: publicMethods(info), friend: t.friend(part.Class), low: lowPriority(info), hidden: hiddenMethods(info), annots: annotatedMethods(info)}
			})
		}
		wg.Wait()
		p.names = map[string]bool{}
		for _, kp := range read {
			if kp != nil {
				p.parts = append(p.parts, kp)
				funcs, props, aliases := kp.meta.Names()
				for _, ns := range [][]string{funcs, props, aliases} {
					for _, n := range ns {
						p.names[n] = true
					}
				}
			}
		}
	})
	return p
}

// pkgFuns returns the public top-level functions of a package with a name.
func (t *classTable) pkgFuns(pkg, name string) []*types.FunSym {
	t = t.root()
	return t.pkgFunsOf(pkg, name, false)
}

// pkgInternalFuns returns the internal top-level functions of a package
// with a name, which code that suppresses INVISIBLE_REFERENCE may call.
func (t *classTable) pkgInternalFuns(pkg, name string) []*types.FunSym {
	t = t.root()
	return t.pkgFunsOf(pkg, name, true)
}

func (t *classTable) pkgFunsOf(pkg, name string, internal bool) []*types.FunSym {
	return t.pkgFunsWhere(pkg, name, internal, allFuns, nil)
}

// funKind selects a package's functions by whether they are extensions.
type funKind uint8

const (
	allFuns funKind = iota
	extFuns
	nonExtFuns
)

// kindKey keys kpkg.kinds.
type kindKey struct {
	name     string
	internal bool
	kind     funKind
}

// kfunEntry is a top-level function of a package, read from its metadata
// and converted to a symbol only when a lookup can use it: a lookup of
// extensions for a receiver needs only those whose receiver takes it, of
// the dozens a name like map or forEach has.
type kfunEntry struct {
	f    *kmeta.Function
	part *kpart
	key  string // the method's name and descriptor
	// recv is an extension's receiver type without its type arguments,
	// which is enough for recvFits; recvOK is unset if it isn't had
	// that cheaply, for a type parameter or a suspend function type.
	recv           types.Type
	recvOK, recvIn bool
	sym            *types.FunSym // once converted
}

// pkgFunsWhere returns the top-level functions of a package with a name,
// as pkgFunsOf, of a kind. If fits isn't nil, it leaves out the
// extensions whose receiver type, without its type arguments, it rejects;
// it must reject only receivers that can't take the call's, as recvFits
// does, so the result is the others' in the same order, without the
// duplicates pkgFunsOf leaves out.
func (t *classTable) pkgFunsWhere(pkg, name string, internal bool, kind funKind, fits func(types.Type) bool) []*types.FunSym {
	return t.pkgFunsWhereIn(t.pkgRef(pkg), name, internal, kind, fits)
}

// pkgFunsWhereIn is pkgFunsWhere of a package resolved once.
func (t *classTable) pkgFunsWhereIn(r *pkgRef, name string, internal bool, kind funKind, fits func(types.Type) bool) []*types.FunSym {
	t = t.root()
	pkg := r.name
	p := t.read(r.k)
	if p == nil || !p.names[name] {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var cache map[string][]*types.FunSym
	var kk kindKey
	switch {
	case fits != nil:
	case kind == allFuns && internal:
		if p.internalFuns == nil {
			p.internalFuns = map[string][]*types.FunSym{}
		}
		cache = p.internalFuns
	case kind == allFuns:
		cache = p.funs
	default:
		kk = kindKey{name, internal, kind}
		if fs, ok := p.kinds[kk]; ok {
			return fs
		}
	}
	if fs, ok := cache[name]; ok {
		return fs
	}
	var fs []*types.FunSym
	for _, e := range t.pkgEntries(p, name, internal) {
		if kind == extFuns && e.f.Recv == nil || kind == nonExtFuns && e.f.Recv != nil {
			continue
		}
		if fits != nil && e.f.Recv != nil {
			if !e.recvIn {
				e.recv, e.recvOK = t.shallowType(e.f.Recv)
				e.recvIn = true
			}
			if e.recvOK && !fits(e.recv) {
				continue
			}
		}
		sym := t.pkgFun(pkg, name, e)
		// As in K2, one function of two modules on the class path, by
		// its package, name and signature, is one candidate.
		if slices.ContainsFunc(fs, func(g *types.FunSym) bool { return sameDeclaration(sym, g) }) {
			continue
		}
		fs = append(fs, sym)
	}
	switch {
	case fits != nil:
	case cache != nil:
		cache[name] = fs
	default:
		if p.kinds == nil {
			p.kinds = map[kindKey][]*types.FunSym{}
		}
		p.kinds[kk] = fs
	}
	return fs
}

// pkgEntries returns the entries of the functions of a package with a
// name, public or internal ones, reading them on first use. p.mu is held.
func (t *classTable) pkgEntries(p *kpkg, name string, internal bool) []*kfunEntry {
	k := kindKey{name: name, internal: internal}
	if es, ok := p.entries[k]; ok {
		return es
	}
	var es []*kfunEntry
	for _, part := range p.parts {
		decls, err := part.meta.Functions(name)
		if err != nil {
			continue
		}
		for _, f := range decls {
			if internal != (f.Flags.Visibility() == kmeta.Internal && !part.friend) {
				continue
			}
			key := f.JVM.Name + f.JVM.Desc
			if !internal && !public(f.Flags.Visibility(), part.friend) || part.hidden[key] {
				continue
			}
			es = append(es, &kfunEntry{f: f, part: part, key: key})
		}
	}
	if p.entries == nil {
		p.entries = map[kindKey][]*kfunEntry{}
	}
	p.entries[k] = es
	return es
}

// pkgFun converts entry e of a package's functions with a name to its
// symbol, on first use. The package's lock is held.
func (t *classTable) pkgFun(pkg, name string, e *kfunEntry) *types.FunSym {
	if e.sym != nil {
		return e.sym
	}
	start := time.Now()
	f, part := e.f, e.part
	sym := t.kfun(f, nil, nil, part.methods)
	sym.Facade = part.facade
	inlineIn(part.class, sym)
	sym.LowPriority = part.low[e.key]
	annotate(sym, part.annots[e.key])
	if sym.Why != "" && typesOK(sym) {
		intrinsic(sym, part.facade, f.JVM)
	}
	switch {
	case sym.Inline != nil && sym.Unsupported:
		// An inline-only function is called by copying its body.
		sym.Unsupported, sym.Why = false, ""
	case sym.Inline != nil:
		sym.Inline = nil // an intrinsic
	case f.IsInline() && !sym.Unsupported && sym.Builtin == types.NotBuiltin:
		// A public inline function too, if its body can be copied.
		// One with reified type parameters must be: its method
		// doesn't know them.
		sym.Inline = inlineInfo(f, hasReified(sym))
	}
	if in := sym.Inline; in != nil {
		in.Class, in.Load = part.class, t.loadInline(part.class)
	}
	if (pkg == "kotlin.reflect" || pkg == "kotlin/reflect") && name == "typeOf" {
		// An intrinsic, as in kotlinc: its body only throws.
		sym.Inline, sym.Unsupported, sym.Why = nil, false, ""
		sym.Builtin = types.TypeOf
	}
	if (pkg == "kotlin.coroutines.intrinsics" || pkg == "kotlin/coroutines/intrinsics") && name == "suspendCoroutineUninterceptedOrReturn" {
		sym.Inline, sym.Unsupported, sym.Why = nil, false, ""
		sym.Builtin = types.SuspendIntrinsic
	}
	if (pkg == "androidx.compose.runtime" || pkg == "androidx/compose/runtime") && name == "key" && sym.Composable {
		// An intrinsic of Compose's plugin: its body doesn't
		// start the movable group.
		sym.Inline, sym.Builtin = nil, types.ComposeKey
	}
	e.sym = sym
	t.timed(start, 1)
	return sym
}

// shallowType is a type of metadata without its type arguments, as ktype
// would have it but for those, or false if it is a type parameter, a
// suspend or @Composable function type, or a class that isn't there.
func (t *classTable) shallowType(kt *kmeta.Type) (types.Type, bool) {
	switch {
	case kt.Class == "" || kt.Flags&kmeta.TypeSuspend != 0 || kt.Composable:
		return types.InvalidType, false
	case len(kt.Args) == 0 && kt.Outer == nil:
		return t.ktype(kt, nil)
	}
	switch kt.Class {
	case "kotlin/Int", "kotlin/Long", "kotlin/Double", "kotlin/Boolean", "kotlin/Char", "kotlin/Float", "kotlin/Byte", "kotlin/Short",
		"kotlin/String", "kotlin/Any", "kotlin/Unit", "kotlin/Nothing":
		return types.InvalidType, false // as ktype has them, which takes no arguments
	}
	cls := t.kclass(kt.Class)
	if cls == nil {
		return types.InvalidType, false
	}
	r := types.ClassType(cls)
	switch {
	case kt.Upper != nil:
		r = r.Platform()
	case kt.Nullable:
		r = r.Nullable()
	}
	return r, true
}

// pkgExtProps returns the getters of a package's public extension
// properties with a name. A getter takes the receiver and has the
// property's type parameters.
func (t *classTable) pkgExtProps(pkg, name string) []*types.FunSym {
	return t.pkgExtPropsIn(t.pkgRef(pkg), name)
}

// pkgExtPropsIn is pkgExtProps of a package resolved once.
func (t *classTable) pkgExtPropsIn(r *pkgRef, name string) []*types.FunSym {
	t = t.root()
	p := t.read(r.k)
	if p == nil || !p.names[name] {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if gs, ok := p.props[name]; ok {
		return gs
	}
	start := time.Now()
	var gs []*types.FunSym
	for _, part := range p.parts {
		decls, err := part.meta.Properties(name)
		if err != nil {
			continue
		}
		for _, pr := range decls {
			g := pr.Getter
			// An inline-only getter is private: its body is copied.
			inlineOnly := g != nil && !hasMethod(part.methods, g.Name+g.Desc, pr.GetterFlags>>8&1 != 0) && pr.GetterFlags>>8&1 != 0
			if pr.Recv == nil || !public(pr.Flags.Visibility(), part.friend) || g == nil || !hasMethod(part.methods, g.Name+g.Desc, pr.GetterFlags>>8&1 != 0) && !inlineOnly || pr.Contexts > len(pr.ContextParams) {
				continue
			}
			sym := &types.FunSym{Name: g.Name, Desc: g.Desc, Facade: part.facade, Ext: true, Resolved: true, Declared: inlineAccessor(pr.GetterFlags)}
			if inlineOnly {
				sym.Inline = &types.InlineInfo{Name: g.Name, Desc: g.Desc, Private: true, Class: part.class, Load: t.loadInline(part.class)}
			}
			scope := tscope{}
			tps, ok := t.ktypeParams(pr.TypeParams, sym, scope)
			sym.TypeParams = tps
			recv, rok := t.ktype(pr.Recv, scope)
			ret, tok := t.ktype(pr.Type, scope)
			ctx, cok := t.kparams(pr.ContextParams, scope, true)
			if !ok || !rok || !tok || !cok {
				continue
			}
			sym.Recv, sym.Ret, sym.Contexts = recv, ret, ctx
			matchJVM(sym)
			composeShape(sym, false) // WindowInsets.isImeVisible
			// A var's setter: (receiver, value) on the same facade.
			if s := pr.Setter; pr.IsVar() && s != nil && public(pr.SetterFlags.Visibility(), part.friend) && hasSetter(pr, part.methods) {
				set := &types.FunSym{Name: s.Name, Desc: s.Desc, Facade: part.facade, Ext: true, Final: true, Resolved: true,
					TypeParams: tps, Recv: recv, Ret: types.UnitType, Params: []*types.VarSym{{Name: "value", Type: ret, Kind: types.Param}}, Contexts: ctx,
					Declared: inlineAccessor(pr.SetterFlags)}
				if inlineOnlySetter(pr, part.methods) {
					set.Inline = &types.InlineInfo{Name: s.Name, Desc: s.Desc, Private: true, Class: part.class, Load: t.loadInline(part.class)}
				}
				matchJVM(set)
				sym.ExtSetter = set
			}
			inlineIn(part.class, sym, sym.ExtSetter)
			annotateProp(pr, part.annots, sym, sym.ExtSetter)
			gs = append(gs, sym)
		}
	}
	p.props[name] = gs
	t.timed(start, len(gs))
	return gs
}

// pkgVar returns a package's public top-level property with a name that
// isn't an extension, or nil.
func (t *classTable) pkgVar(pkg, name string) *types.VarSym {
	return t.pkgVarIn(t.pkgRef(pkg), name)
}

// pkgVarIn is pkgVar of a package resolved once.
func (t *classTable) pkgVarIn(r *pkgRef, name string) *types.VarSym {
	t = t.root()
	pkg := r.name
	p := t.read(r.k)
	if p == nil || !p.names[name] {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.vars[name]; ok {
		return v
	}
	var v, first *types.VarSym // first of those with other context parameters
	for _, part := range p.parts {
		decls, err := part.meta.Properties(name)
		if err != nil {
			continue
		}
		for _, pr := range decls {
			if pr.Recv != nil || !public(pr.Flags.Visibility(), part.friend) || len(pr.TypeParams) > 0 || pr.Contexts > len(pr.ContextParams) {
				continue
			}
			typ, ok := t.ktype(pr.Type, tscope{})
			ctx, cok := t.kparams(pr.ContextParams, tscope{}, true)
			if !ok || !cok {
				continue
			}
			prev := v
			v = &types.VarSym{Name: pr.Name, Type: typ, Kind: types.Global, Mutable: pr.IsVar(), Facade: part.facade,
				Resolved: true, Const: pr.IsConst(), Contexts: ctx,
				InlineGetter: pr.GetterFlags>>8&1 != 0, InlineSetter: pr.IsVar() && pr.SetterFlags>>8&1 != 0}
			if prev != nil && !sameContexts(prev.Contexts, ctx) {
				// One with other context parameters.
				addAlt(prev, v)
				if first == nil {
					first = prev
				}
			}
			if v.Const {
				v.Constant = t.facadeConstant(part.facade, pr.Name)
			}
			if pr.Synthetic != nil {
				if m := part.annots[pr.Synthetic.Name+pr.Synthetic.Desc]; m != nil {
					v.Annotations = m.Annotations
					if m.Deprecated != nil && !m.Deprecated.Java {
						v.Deprecated = classFileDeprecation(m.Deprecated)
					}
				}
			}
			if (pkg == "kotlin.coroutines" || pkg == "kotlin/coroutines") && name == "coroutineContext" {
				v.Suspend = true // an intrinsic: its getter only throws
			}
			switch {
			case pr.Getter != nil:
				v.JVMGetter = pr.Getter.Name
				// A composable getter takes a Composer and an int.
				v.Composable = strings.HasPrefix(pr.Getter.Desc, "("+types.ComposerDesc+"I)")
				if pr.Setter != nil {
					v.JVMSetter = pr.Setter.Name
				}
			case !v.Const && pr.Field != nil:
				v.JvmField = true // only a public field
			}
		}
	}
	if first != nil {
		v = first
	}
	p.vars[name] = v
	return v
}

// typesOK reports whether kc supports the types of a function's
// parameters, receiver and result.
func typesOK(f *types.FunSym) bool {
	if f.Ext && f.Recv.K == types.Invalid {
		return false
	}
	for _, p := range f.Params {
		if p.Type.K == types.Invalid {
			return false
		}
	}
	return f.Ret.K != types.Invalid
}

// intrinsics are inline-only functions whose bodies kc knows. Their
// calls are generated directly, or call another function.
var intrinsics = map[string]struct {
	builtin  types.Builtin
	class    string // NewInstance: the class to create
	redirect string // the facade method to call instead: name and descriptor
}{
	// String.trim() is (this as CharSequence).trim().toString().
	"kotlin/text/StringsKt.trim(Ljava/lang/String;)Ljava/lang/String;": {builtin: types.ThenToString,
		redirect: "trim(Ljava/lang/CharSequence;)Ljava/lang/CharSequence;"},
	"kotlin/text/StringsKt.trimStart(Ljava/lang/String;)Ljava/lang/String;": {builtin: types.ThenToString,
		redirect: "trimStart(Ljava/lang/CharSequence;)Ljava/lang/CharSequence;"},
	"kotlin/text/StringsKt.trimEnd(Ljava/lang/String;)Ljava/lang/String;": {builtin: types.ThenToString,
		redirect: "trimEnd(Ljava/lang/CharSequence;)Ljava/lang/CharSequence;"},
	"kotlin/io/ConsoleKt.println":                                                         {builtin: types.Println},
	"kotlin/io/ConsoleKt.print":                                                           {builtin: types.Print},
	"kotlin/collections/CollectionsKt.mutableListOf()Ljava/util/List;":                    {builtin: types.NewInstance, class: "java/util/ArrayList"},
	"kotlin/collections/CollectionsKt.arrayListOf()Ljava/util/ArrayList;":                 {builtin: types.NewInstance, class: "java/util/ArrayList"},
	"kotlin/collections/CollectionsKt.listOf()Ljava/util/List;":                           {redirect: "emptyList()Ljava/util/List;"},
	"kotlin/collections/MapsKt.mutableMapOf()Ljava/util/Map;":                             {builtin: types.NewInstance, class: "java/util/LinkedHashMap"},
	"kotlin/collections/MapsKt.hashMapOf()Ljava/util/HashMap;":                            {builtin: types.NewInstance, class: "java/util/HashMap"},
	"kotlin/collections/MapsKt.mapOf()Ljava/util/Map;":                                    {redirect: "emptyMap()Ljava/util/Map;"},
	"kotlin/collections/MapsKt.set(Ljava/util/Map;Ljava/lang/Object;Ljava/lang/Object;)V": {builtin: types.MapSet},
	"kotlin/collections/MapsKt.component1(Ljava/util/Map$Entry;)Ljava/lang/Object;":       {builtin: types.EntryKey},
	"kotlin/collections/MapsKt.component2(Ljava/util/Map$Entry;)Ljava/lang/Object;":       {builtin: types.EntryValue},
	"kotlin/collections/SetsKt.mutableSetOf()Ljava/util/Set;":                             {builtin: types.NewInstance, class: "java/util/LinkedHashSet"},
	"kotlin/collections/SetsKt.hashSetOf()Ljava/util/HashSet;":                            {builtin: types.NewInstance, class: "java/util/HashSet"},
	"kotlin/collections/SetsKt.setOf()Ljava/util/Set;":                                    {redirect: "emptySet()Ljava/util/Set;"},
	"kotlin/collections/ArraysKt.toTypedArray(Ljava/util/Collection;)[Ljava/lang/Object;": {builtin: types.ToTypedArray},
}

// intrinsic makes an inline-only function callable if kc knows its body.
func intrinsic(f *types.FunSym, facade string, jvm kmeta.JVMSig) {
	in, ok := intrinsics[facade+"."+jvm.Name+jvm.Desc]
	if !ok {
		in, ok = intrinsics[facade+"."+jvm.Name]
		if !ok || in.builtin != types.Println && in.builtin != types.Print {
			return
		}
	}
	f.Unsupported, f.Why = false, ""
	switch {
	case in.redirect != "":
		i := strings.IndexByte(in.redirect, '(')
		f.JVMName, f.Desc, f.Builtin = in.redirect[:i], in.redirect[i:], in.builtin
	case in.builtin == types.NewInstance:
		f.Builtin, f.Facade = in.builtin, in.class
	default:
		f.Builtin = in.builtin
	}
}

// qualified returns the internal name of class name in package pkg,
// whose parts are separated by slashes: pkg/name, or name in the default
// package.
func qualified(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "/" + name
}

// pkgClass returns the class that a simple name refers to in a package:
// a class, a built-in type or a typealias of a class, or nil.
func (t *classTable) pkgClass(pkg, name string) *types.ClassSym {
	return t.pkgClassIn(t.pkgRef(pkg), name)
}

// pkgClassIn is pkgClass of a package resolved once. It looks the class up
// by name only if the class path may have it.
func (t *classTable) pkgClassIn(r *pkgRef, name string) *types.ClassSym {
	if r.path != "" && !pkgSynthetic(r.path, name) && !syntheticClass(r.path, name) && t.absent(classpath.HashJoin(r.hash, '/', name)) {
		alias := t.pkgClassAlias(r, name)
		if verifyFilter && t.pkgClassFull(r, name) != alias {
			panic("check: the class path filter ruled out " + qualified(r.path, name))
		}
		return alias
	}
	return t.pkgClassFull(r, name)
}

// pkgClassFull is pkgClassIn without the filter.
func (t *classTable) pkgClassFull(r *pkgRef, name string) *types.ClassSym {
	id := qualified(r.path, name)
	if _, ok := collectionJVM[id]; ok {
		return t.kclass(id)
	}
	if _, ok := atomicJVM[id]; ok {
		return t.atomic(id)
	}
	if a := types.ArrayByID(id); a != nil {
		return a
	}
	if n, ok := functionArity(id, "kotlin/"); ok {
		return t.funcs.Class(n) // kotlin.Function1, which no class file declares
	}
	if jl, ok := javaLangOf[id]; ok {
		return t.class(jl)
	}
	if c := t.class(id); c != nil {
		return c
	}
	return t.pkgClassAlias(r, name)
}

// pkgClassAlias returns the class a public typealias of a package named
// name stands for, or nil.
func (t *classTable) pkgClassAlias(r *pkgRef, name string) *types.ClassSym {
	p := t.root().read(r.k)
	if p == nil || !p.names[name] {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.aliases[name]; ok {
		return c
	}
	var found *types.ClassSym
	for _, part := range p.parts {
		as, err := part.meta.TypeAliases(name)
		if err != nil {
			continue
		}
		for _, a := range as {
			if c := t.aliasClass(a); c != nil && public(a.Flags.Visibility(), part.friend) && !aliasRequires(a) {
				found = c
			}
		}
	}
	p.aliases[name] = found
	return found
}

// pkgAlias returns a public typealias of a package on the class path that
// pkgClass doesn't return as a class, such as an alias of a function type
// or of a class with fixed type arguments, or nil.
func (t *classTable) pkgAlias(pkg, name string) *types.AliasSym {
	return t.pkgAliasIn(t.pkgRef(pkg), name)
}

// pkgAliasIn is pkgAlias of a package resolved once.
func (t *classTable) pkgAliasIn(r *pkgRef, name string) *types.AliasSym {
	t = t.root()
	pkg := r.name
	p := t.read(r.k)
	if p == nil || !p.names[name] {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if a, ok := p.typeAliases[name]; ok {
		return a
	}
	var found *types.AliasSym
	for _, part := range p.parts {
		as, err := part.meta.TypeAliases(name)
		if err != nil {
			continue
		}
		for _, a := range as {
			e := a.Expanded
			if e == nil {
				e = a.Underlying
			}
			if !public(a.Flags.Visibility(), part.friend) || t.aliasClass(a) != nil && !aliasRequires(a) || e == nil {
				continue
			}
			s := &types.AliasSym{Name: name, Pkg: pkg, Resolved: true}
			aliasAnnotations(s, a)
			scope := tscope{}
			tps, ok := t.ktypeParams(a.TypeParams, s, scope)
			typ, tok := t.ktype(e, scope)
			if ok && tok {
				s.TypeParams, s.Type = tps, typ
				found = s
			}
		}
	}
	p.typeAliases[name] = found
	return found
}

// aliasClass returns the class a typealias stands for, if it is
// typealias A<P1, ..., Pn> = C<P1, ..., Pn>.
func (t *classTable) aliasClass(a *kmeta.TypeAlias) *types.ClassSym {
	t = t.root()
	e := a.Expanded
	if e == nil || e.Class == "" || e.Nullable || len(e.Args) != len(a.TypeParams) {
		return nil
	}
	for i, arg := range e.Args {
		if arg.Proj != kmeta.Inv || arg.Type == nil || arg.Type.Class != "" || arg.Type.Param != a.TypeParams[i].ID {
			return nil
		}
	}
	return t.kclass(e.Class)
}

// platformDependent are the members of Kotlin's collection types marked
// @PlatformDependent, by type and name: their numbers of parameters.
var platformDependent = map[string]int{
	"kotlin/collections/Map.getOrDefault":  2,
	"kotlin/collections/MutableMap.remove": 2,
}

// verifyAbsent panics if cls, what a full lookup of name found, is a
// class that the class path's name filter ruled out.
func (t *classTable) verifyAbsent(name string, cls *types.ClassSym) {
	if cls != nil {
		panic("check: the class path filter ruled out " + name)
	}
}

// missingTypeAnno returns the name of the class of an annotation of type
// kt, not of its type arguments, that the class path doesn't have, or "".
func (t *classTable) missingTypeAnno(kt *kmeta.Type) string {
	if kt == nil {
		return ""
	}
	for _, an := range kt.Annotations {
		if c := t.missingClass(an.Class, 0); c != nil {
			return missingName(c)
		}
	}
	return ""
}
