package ka

import (
	"cmp"
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// library answers a Model's questions about the classes of the class path:
// Model.Class. Its models are built on demand and kept.
type library struct {
	cp   *classpath.Path
	lib  *check.Library
	java *driver.JavaSources // the module's Java sources' classes, or nil

	mu     sync.Mutex
	decls  map[string]*Decl             // by class ID; nil: no such class
	params map[string]map[string]string // annotation classes' parameters' types, see paramTypes
	pkgs   map[string]*Package          // by name; nil: no such package

	partsOnce  sync.Once
	partsByPkg map[string][]kmeta.Part // see parts

	defaultOnce sync.Once
	defaults    map[string]*Decl // see defaultAliases
}

// maxNesting bounds how deep Class models nested classes.
const maxNesting = 8

// Class returns the model of the class-path class with class ID id
// (a/b/Outer.Inner), its nested classes included; nil if the class path
// has no such class. Kotlin's built-in classes that are Java classes on
// the JVM, as kotlin/String and kotlin/collections/List, are modeled
// without members: kc doesn't read kotlin_builtins.
func (m *Model) Class(id string) *Decl {
	l := m.lib
	if l == nil {
		return nil
	}
	l.mu.Lock()
	d, ok := l.decls[id]
	l.mu.Unlock()
	if ok {
		return d
	}
	d = l.class(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	if old, ok := l.decls[id]; ok {
		return old
	}
	l.decls[id] = d
	return d
}

// Close releases the class path the Model reads library classes from.
func (m *Model) Close() error {
	if m.shared == nil {
		return nil
	}
	var err error
	m.closeOnce.Do(func() { err = m.shared.release() })
	return err
}

// internalName returns the JVM internal name of the class with class ID
// id.
func internalName(id string) string {
	pkg, rel := "", id
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		pkg, rel = id[:i+1], id[i+1:]
	}
	return pkg + strings.ReplaceAll(rel, ".", "$")
}

// simpleName returns the last name of class ID id.
func simpleName(id string) string {
	return id[strings.LastIndexAny(id, "/.")+1:]
}

func (l *library) class(id string) *Decl {
	if id == "kotlin/Any" {
		// The root of every class's supertypes, JDK or not.
		return &Decl{Kind: KindClass, Name: "Any", ClassID: id, Modality: ModalityOpen, Members: anyMembers()}
	}
	if d := l.kotlinClass(id, internalName(id), nil, 0); d != nil {
		return d
	}
	cls := l.lib.Class(id)
	if cls == nil {
		if jid, ok := javaPrimitiveClasses[id]; ok {
			cls = l.lib.Class(jid)
		}
	}
	if cls == nil {
		return nil
	}
	if cls.Decl != nil {
		return nil // a class of the program
	}
	cls.Ensure()
	if cls.Missing {
		return nil
	}
	if cls.KotlinName == "" && classID(cls) == id && l.cp.Has(cls.Name) && !strings.HasPrefix(id, "kotlin/") {
		return l.javaClass(cls, 0)
	}
	// A built-in class: a function type's interface, an array, or a
	// Java class that Kotlin maps.
	d := &Decl{Kind: KindClass, Name: simpleName(id), ClassID: id, Modality: ModalityFinal}
	if cls.Interface {
		d.Kind, d.Modality = KindInterface, ModalityAbstract
	} else if !cls.Final {
		d.Modality = ModalityOpen
	}
	// Its supertypes are its Java class's, which kc maps to Kotlin's, as
	// kotlin/String : kotlin/Comparable<kotlin/String>.
	b := &fileBuilder{}
	d.TypeParams = l.javaTypeParams(b, cls.TypeParams)
	for i, tp := range d.TypeParams {
		tp.Bounds = slices.DeleteFunc(tp.Bounds, func(t *Type) bool { return t.ClassID == "kotlin/Any" })
		for _, t := range tp.Bounds {
			nonNull(t)
		}
		tp.Variance = builtinVariance(id, i, len(d.TypeParams))
	}
	if isFunctionClass(id) {
		// kotlin/Function<R>, which kc's function interfaces leave out.
		r := d.TypeParams[len(d.TypeParams)-1].Name
		d.Supertypes = []*Type{{Kind: TypeClass, ClassID: "kotlin/Function",
			Args: []TypeArg{{Type: &Type{Kind: TypeParameter, Name: r}}}}}
		return d
	}
	// The Kotlin ones, and java/io/Serializable, which Kotlin adds to the
	// classes it maps; not the JDK's others, as java/lang/constant/Constable.
	d.Supertypes = slices.DeleteFunc(javaSupertypes(b, cls, d.Kind, nonNull), func(t *Type) bool {
		return !strings.HasPrefix(t.ClassID, "kotlin/") && t.ClassID != "java/io/Serializable"
	})
	if len(d.Supertypes) == 0 {
		d.Supertypes = []*Type{{Kind: TypeClass, ClassID: "kotlin/Any"}}
	}
	return d
}

// isFunctionClass reports whether id is a function type's class:
// kotlin/FunctionN or kotlin/coroutines/SuspendFunctionN.
func isFunctionClass(id string) bool {
	n, ok := strings.CutPrefix(id, "kotlin/Function")
	if !ok {
		n, ok = strings.CutPrefix(id, "kotlin/coroutines/SuspendFunction")
	}
	if !ok || n == "" {
		return false
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// javaPrimitiveClasses maps the Kotlin classes that kc treats as types,
// not classes, to the Java classes whose supertypes they have.
var javaPrimitiveClasses = map[string]string{
	"kotlin/String":  "java/lang/String",
	"kotlin/Int":     "java/lang/Integer",
	"kotlin/Long":    "java/lang/Long",
	"kotlin/Short":   "java/lang/Short",
	"kotlin/Byte":    "java/lang/Byte",
	"kotlin/Char":    "java/lang/Character",
	"kotlin/Boolean": "java/lang/Boolean",
	"kotlin/Float":   "java/lang/Float",
	"kotlin/Double":  "java/lang/Double",
}

// builtinVariance returns the declared variance of type parameter i of n
// of built-in class id, which its Java class doesn't have.
func builtinVariance(id string, i, n int) Variance {
	switch id {
	case "kotlin/collections/Iterable", "kotlin/collections/Collection", "kotlin/collections/List",
		"kotlin/collections/Set", "kotlin/collections/Iterator", "kotlin/collections/ListIterator",
		"kotlin/collections/Map.Entry", "kotlin/Function":
		return Out
	case "kotlin/collections/Map":
		if i == 1 {
			return Out
		}
	case "kotlin/Comparable":
		return In
	}
	if isFunctionClass(id) {
		if i == n-1 {
			return Out
		}
		return In
	}
	return Invariant
}

// anyMembers returns the members of kotlin/Any.
func anyMembers() []*Decl {
	class := func(id string, nullable bool) *Type { return &Type{Kind: TypeClass, ClassID: id, Nullable: nullable} }
	return []*Decl{
		{Kind: KindConstructor, Name: "<init>", Visibility: Public, Modality: ModalityFinal, Flags: FlagPrimary},
		{Kind: KindFunction, Name: "equals", Visibility: Public, Modality: ModalityOpen, Flags: FlagOperator,
			Type:   class("kotlin/Boolean", false),
			Params: []*Decl{{Kind: KindValueParameter, Name: "other", Type: class("kotlin/Any", true)}}},
		{Kind: KindFunction, Name: "hashCode", Visibility: Public, Modality: ModalityOpen, Type: class("kotlin/Int", false)},
		{Kind: KindFunction, Name: "toString", Visibility: Public, Modality: ModalityOpen, Type: class("kotlin/String", false)},
	}
}

// nonNull makes type t and its arguments not null, as a built-in class's
// supertypes are.
func nonNull(t *Type) *Type {
	if t == nil {
		return nil
	}
	t.Nullable, t.Flexible = false, false
	for _, a := range t.Args {
		nonNull(a.Type)
	}
	return t
}

// load reads the Kotlin metadata and the details of class file name;
// nils if it isn't a Kotlin class's.
func (l *library) load(name string) (*kmeta.Class, *classfile.Info, *classfile.Details) {
	if !l.cp.Has(name) || l.cp.IsSource(name) {
		return nil, nil, nil
	}
	info, err := l.cp.Load(name)
	if err != nil || info.Kotlin == nil || info.Kotlin.K != kmeta.KindClassFile {
		return nil, info, nil
	}
	kc, err := kmeta.ReadClass(info.Kotlin)
	if err != nil {
		return nil, info, nil
	}
	data, err := l.cp.Bytes(name)
	if err != nil {
		return nil, info, nil
	}
	det, err := classfile.ParseDetails(data)
	if err != nil {
		det = &classfile.Details{}
	}
	return kc, info, det
}

// kscope maps the ids of the type parameters in scope to their names.
type kscope map[int]string

// kotlinClass returns the model of Kotlin class id, whose class file is
// name, nested in a class whose type parameters are outer; nil if it
// isn't a Kotlin class.
func (l *library) kotlinClass(id, name string, outer kscope, depth int) *Decl {
	kc, info, det := l.load(name)
	if kc == nil || kc.Name != id {
		return nil
	}
	d := &Decl{Kind: KindClass, Name: simpleName(id), ClassID: id,
		Visibility: kvisibility(kc.Flags.Visibility()), Modality: Modality(kc.Flags.Modality())}
	switch kc.Kind() {
	case kmeta.KindInterface:
		d.Kind = KindInterface
	case kmeta.KindEnum:
		d.Kind = KindEnumClass
	case kmeta.KindAnnotation:
		d.Kind = KindAnnotationClass
	case kmeta.KindObject:
		d.Kind = KindObject
	case kmeta.KindCompanion:
		d.Kind = KindObject
		d.Flags |= FlagCompanion
	}
	set := func(bit int, f Flags) {
		if int(kc.Flags)&bit != 0 {
			d.Flags |= f
		}
	}
	set(kmeta.ClassInner, FlagInner)
	set(kmeta.ClassData, FlagData)
	set(kmeta.ClassExternal, FlagExternal)
	set(kmeta.ClassExpect, FlagExpect)
	set(kmeta.ClassValue, FlagValue)
	set(kmeta.ClassFun, FlagFun)
	if info.Inner != nil && kc.Flags&kmeta.ClassInner == 0 && depth > 0 {
		d.Flags |= FlagStatic
	}
	scope := kscope{}
	if d.Flags&FlagInner != 0 {
		for k, v := range outer {
			scope[k] = v
		}
	}
	d.TypeParams = l.ktypeParams(kc.TypeParams, scope)
	for _, st := range kc.Supers {
		d.Supertypes = append(d.Supertypes, l.ktype(st, scope, 0))
	}
	if kc.Flags&hasAnnotations != 0 {
		d.Annotations = l.annotations(det.Annotations, true)
	}
	byName, bySig := methods(det)
	for _, c := range kc.Ctors {
		md := &Decl{Kind: KindConstructor, Name: "<init>", Visibility: kvisibility(c.Flags.Visibility()),
			Modality: ModalityFinal}
		if c.Flags&kmeta.CtorSecondary == 0 {
			md.Flags |= FlagPrimary
		}
		m := bySig[c.JVM.Name+c.JVM.Desc]
		if c.Flags&hasAnnotations != 0 && m != nil {
			md.Annotations = l.annotations(m.Annotations, true)
		}
		md.Params = l.kparams(c.Params, scope, m)
		if d.Kind == KindAnnotationClass {
			for _, p := range md.Params {
				if am := byName[p.Name]; am != nil && am.Default != nil && strings.HasPrefix(am.Desc, "()") {
					p.Default = l.value(am.Default, paramHint(p.Type), 0)
				}
			}
		}
		d.Members = append(d.Members, md)
	}
	for _, p := range kc.Props {
		if p.Flags.MemberKind() != 1 {
			d.Members = append(d.Members, l.kproperty(p, scope, bySig))
		}
	}
	for _, f := range kc.Funcs {
		if f.Flags.MemberKind() != 1 {
			d.Members = append(d.Members, l.kfunction(f, scope, bySig))
		}
	}
	for _, e := range kc.EnumEntries {
		d.Members = append(d.Members, &Decl{Kind: KindEnumEntry, Name: e, ClassID: id + "." + e,
			Visibility: Public, Type: &Type{Kind: TypeClass, ClassID: id}})
	}
	if d.Kind == KindEnumClass {
		d.Members = append(d.Members, enumMembers(id, l.cp.Has("kotlin/enums/EnumEntries"), FlagStatic)...)
	}
	if depth < maxNesting {
		for _, n := range kc.Nested {
			if nd := l.kotlinClass(id+"."+n, name+"$"+n, scope, depth+1); nd != nil {
				d.Members = append(d.Members, nd)
			}
		}
	}
	return d
}

// kproperty returns the model of property p from metadata, whose class
// file's methods are bySig.
func (l *library) kproperty(p *kmeta.Property, scope kscope, bySig map[string]*classfile.MemberDetails) *Decl {
	md := &Decl{Kind: KindProperty, Name: p.Name, Visibility: kvisibility(p.Flags.Visibility()),
		Modality: Modality(p.Flags.Modality())}
	pf := int(p.Flags)
	for bit, f := range map[int]Flags{kmeta.PropVar: FlagMutable, kmeta.PropConst: FlagConst,
		kmeta.PropLateinit: FlagLateinit, propExpect: FlagExpect, propExternal: FlagExternal} {
		if pf&bit != 0 {
			md.Flags |= f
		}
	}
	if p.Field != nil {
		md.Flags |= FlagBackingField
	}
	ps := scope.with(p.TypeParams)
	md.TypeParams = l.ktypeParams(p.TypeParams, ps)
	if p.Recv != nil {
		md.Receiver = l.ktype(p.Recv, ps, 0)
	}
	md.Type = l.ktype(p.Type, ps, 0)
	if p.Flags&hasAnnotations != 0 && p.Synthetic != nil {
		if m := bySig[p.Synthetic.Name+p.Synthetic.Desc]; m != nil {
			md.Annotations = l.annotations(m.Annotations, true)
		}
	}
	return md
}

// kfunction returns the model of function f from metadata, whose class
// file's methods are bySig.
func (l *library) kfunction(f *kmeta.Function, scope kscope, bySig map[string]*classfile.MemberDetails) *Decl {
	md := &Decl{Kind: KindFunction, Name: f.Name, Visibility: kvisibility(f.Flags.Visibility()),
		Modality: Modality(f.Flags.Modality())}
	ff := int(f.Flags)
	for bit, fl := range map[int]Flags{kmeta.FunOperator: FlagOperator, kmeta.FunInfix: FlagInfix,
		kmeta.FunInline: FlagInline, kmeta.FunTailrec: FlagTailrec, kmeta.FunExternal: FlagExternal,
		kmeta.FunSuspend: FlagSuspend, kmeta.FunExpect: FlagExpect} {
		if ff&bit != 0 {
			md.Flags |= fl
		}
	}
	fs := scope.with(f.TypeParams)
	md.TypeParams = l.ktypeParams(f.TypeParams, fs)
	if f.Recv != nil {
		md.Receiver = l.ktype(f.Recv, fs, 0)
	}
	m := bySig[f.JVM.Name+f.JVM.Desc]
	md.Params = l.kparams(f.Params, fs, m)
	md.Type = l.ktype(f.Ret, fs, 0)
	if f.Flags&hasAnnotations != 0 && m != nil {
		md.Annotations = l.annotations(m.Annotations, true)
	}
	return md
}

// Bits of kmeta's Flags that it has no names for.
const (
	hasAnnotations = 1 << 0
	propExternal   = 1 << 14
	propExpect     = 1 << 16
)

// methods returns a class file's methods by name and by name and
// descriptor.
func methods(det *classfile.Details) (byName, bySig map[string]*classfile.MemberDetails) {
	byName = map[string]*classfile.MemberDetails{}
	bySig = map[string]*classfile.MemberDetails{}
	for i := range det.Methods {
		m := &det.Methods[i]
		if _, ok := byName[m.Name]; !ok {
			byName[m.Name] = m
		}
		bySig[m.Name+m.Desc] = m
	}
	return byName, bySig
}

func kvisibility(v kmeta.Visibility) Visibility {
	switch v {
	case kmeta.Internal:
		return Internal
	case kmeta.Private, kmeta.PrivateToThis, kmeta.Local:
		return Private
	case kmeta.Protected:
		return Protected
	}
	return Public
}

// with returns s with type parameters tps added, or s if there are none.
func (s kscope) with(tps []*kmeta.TypeParam) kscope {
	if len(tps) == 0 {
		return s
	}
	out := make(kscope, len(s)+len(tps))
	for k, v := range s {
		out[k] = v
	}
	for _, tp := range tps {
		out[tp.ID] = tp.Name
	}
	return out
}

// ktypeParams returns the models of type parameters tps, which it adds to
// scope.
func (l *library) ktypeParams(tps []*kmeta.TypeParam, scope kscope) []*Decl {
	for _, tp := range tps {
		scope[tp.ID] = tp.Name
	}
	var out []*Decl
	for _, tp := range tps {
		md := &Decl{Kind: KindTypeParameter, Name: tp.Name}
		switch tp.Variance {
		case kmeta.In:
			md.Variance = In
		case kmeta.Out:
			md.Variance = Out
		}
		if tp.Reified {
			md.Flags |= FlagReified
		}
		for _, b := range tp.Bounds {
			md.Bounds = append(md.Bounds, l.ktype(b, scope, 0))
		}
		out = append(out, md)
	}
	return out
}

// kparams returns the models of value parameters ps of the method m (nil
// if unknown), whose annotations its class file has.
func (l *library) kparams(ps []*kmeta.Param, scope kscope, m *classfile.MemberDetails) []*Decl {
	var out []*Decl
	for i, p := range ps {
		md := &Decl{Kind: KindValueParameter, Name: p.Name}
		if p.Vararg != nil {
			md.Flags |= FlagVararg
			md.Type = l.ktype(p.Vararg, scope, 0)
		} else {
			md.Type = l.ktype(p.Type, scope, 0)
		}
		for bit, f := range map[int]Flags{kmeta.ParamDeclaresDefault: FlagHasDefault,
			kmeta.ParamCrossinline: FlagCrossinline, kmeta.ParamNoinline: FlagNoinline} {
			if p.Flags&bit != 0 {
				md.Flags |= f
			}
		}
		// The method's parameters may start with synthetic ones: an
		// enum's name and ordinal, an inner class's outer instance, an
		// extension's receiver. Its last ones are these.
		if p.Flags&hasAnnotations != 0 && m != nil {
			if j := len(m.Params) - len(ps) + i; j >= 0 && j < len(m.Params) {
				md.Annotations = l.annotations(m.Params[j], true)
			}
		}
		out = append(out, md)
	}
	return out
}

// ktype returns the model of type t from metadata, in which the type
// parameters scope has are in scope.
func (l *library) ktype(t *kmeta.Type, scope kscope, depth int) *Type {
	if t == nil {
		return nil
	}
	if depth > 16 {
		return &Type{Kind: TypeError, Name: "<too deep>"}
	}
	out := &Type{Nullable: t.Nullable}
	if t.Upper != nil {
		out.Nullable, out.Flexible = false, true
	}
	if t.Abbrev != nil && t.Abbrev.Alias != "" {
		out.Abbrev = l.ktype(t.Abbrev, scope, depth+1)
		out.Abbrev.Nullable = out.Nullable
	}
	if t.Class == "" && t.Alias == "" {
		out.Kind = TypeParameter
		out.Name = t.ParamName
		if out.Name == "" {
			out.Name = scope[t.Param]
		}
		if out.Name == "" {
			return &Type{Kind: TypeError, Name: "<unknown type parameter>"}
		}
		out.DefinitelyNonNull = t.Flags&kmeta.TypeDefinitelyNonNull != 0
		return out
	}
	out.Kind = TypeClass
	out.ClassID = cmp.Or(t.Class, t.Alias)
	out.Extension, out.Contexts = t.ExtFunc, t.Contexts
	args := t.Args
	if t.Flags&kmeta.TypeSuspend != 0 {
		// kotlin/FunctionN+1<P1, ..., Continuation<R>, Any?>, a suspend
		// function type: suspend (P1, ...) -> R.
		if n := len(args); n >= 2 {
			if c := args[n-2].Type; c != nil && c.Class == "kotlin/coroutines/Continuation" && len(c.Args) == 1 {
				args = append(slices.Clone(args[:n-2]), c.Args[0])
				out.ClassID = "kotlin/coroutines/SuspendFunction" + itoa(int64(n-2))
			}
		}
	}
	for o := t.Outer; o != nil; o = o.Outer {
		args = append(slices.Clone(args), o.Args...)
	}
	for _, a := range args {
		ta := TypeArg{}
		switch a.Proj {
		case kmeta.In:
			ta.Variance = In
		case kmeta.Out:
			ta.Variance = Out
		case kmeta.Star:
			ta.Variance = Star
		}
		if ta.Variance != Star {
			ta.Type = l.ktype(a.Type, scope, depth+1)
		}
		out.Args = append(out.Args, ta)
	}
	return out
}

// javaClass returns the model of Java class cls.
func (l *library) javaClass(cls *types.ClassSym, depth int) *Decl {
	info, err := l.cp.Load(cls.Name)
	if err != nil {
		return nil
	}
	det := &classfile.Details{}
	source := l.cp.IsSource(cls.Name)
	if source {
		if l.java != nil {
			if dd := l.java.Details(cls.Name); dd != nil {
				det = dd
			}
		}
	} else if data, err := l.cp.Bytes(cls.Name); err == nil {
		if dd, err := classfile.ParseDetails(data); err == nil {
			det = dd
		}
	}
	id := classID(cls)
	access := info.Access
	if info.Inner != nil {
		access = info.Inner.Access
	}
	d := &Decl{Kind: KindClass, Name: simpleName(id), ClassID: id, Visibility: javaVisibility(access),
		Flags: FlagJava}
	if source {
		d.Flags |= FlagJavaSource
	}
	switch {
	case access&classfile.AccAnnotation != 0:
		d.Kind = KindAnnotationClass
	case access&classfile.AccInterface != 0:
		d.Kind = KindInterface
	case access&classfile.AccEnum != 0:
		d.Kind = KindEnumClass
	}
	switch {
	case access&(classfile.AccInterface|classfile.AccAbstract) != 0:
		d.Modality = ModalityAbstract
	case access&classfile.AccFinal != 0 || d.Kind == KindEnumClass:
		d.Modality = ModalityFinal
	default:
		d.Modality = ModalityOpen
	}
	if info.Inner != nil && info.Inner.Outer != "" {
		if access&classfile.AccStatic != 0 || d.Kind != KindClass {
			d.Flags |= FlagStatic
		} else {
			d.Flags |= FlagInner
		}
	}
	b := &fileBuilder{}
	d.TypeParams = l.javaTypeParams(b, cls.TypeParams)
	d.Supertypes = javaSupertypes(b, cls, d.Kind, platform)
	d.Annotations = l.annotations(det.Annotations, false)
	_, bySig := methods(det)
	// pos is each member's index among the class's methods, fields or
	// nested classes, in class-file (or source) order.
	pos := map[*Decl]int{}
	sigIndex := map[string]int{}
	for i, m := range det.Methods {
		sigIndex[m.Name+m.Desc] = i
	}
	method := func(f *types.FunSym, kind Kind) *Decl {
		md := l.javaMethod(b, f, bySig, d, kind)
		name := f.JVMName
		switch {
		case kind == KindConstructor:
			name = "<init>"
		case name == "":
			name = f.Name
		}
		if i, ok := sigIndex[name+f.Desc]; ok {
			pos[md] = i
		}
		return md
	}
	if d.Kind == KindAnnotationClass {
		// Its parameters are its methods, in class-file order, as the
		// parameters of a constructor.
		ctor := &Decl{Kind: KindConstructor, Name: "<init>", Visibility: Public, Modality: ModalityFinal,
			Flags: FlagPrimary | FlagJava}
		for i := range det.Methods {
			m := &det.Methods[i]
			if m.Access&classfile.AccStatic != 0 || !strings.HasPrefix(m.Desc, "()") || m.Name == "<clinit>" {
				continue
			}
			p := &Decl{Kind: KindValueParameter, Name: m.Name}
			if fs := cls.Methods[m.Name]; len(fs) > 0 {
				p.Type = platform(b.typ(fs[0].Ret, nil))
				if p.Type != nil {
					p.Type.Flexible = false // annotation parameters aren't null
				}
			}
			if m.Default != nil {
				p.Flags |= FlagHasDefault
				p.Default = l.value(m.Default, paramHint(p.Type), 0)
			}
			ctor.Params = append(ctor.Params, p)
		}
		d.Members = append(d.Members, ctor)
	} else {
		for _, f := range cls.Ctors {
			d.Members = append(d.Members, method(f, KindConstructor))
		}
	}
	for _, name := range sortedKeys(cls.Methods) {
		for _, f := range cls.Methods[name] {
			if d.Kind == KindAnnotationClass && len(f.Params) == 0 && f.Abstract {
				// A parameter, which the constructor has; Kotlin sees it
				// as a property too.
				d.Members = append(d.Members, &Decl{Kind: KindProperty, Name: f.Name, Visibility: Public,
					Modality: ModalityFinal, Flags: FlagJava, Type: platform(b.typ(f.Ret, nil))})
				if t := d.Members[len(d.Members)-1].Type; t != nil {
					t.Flexible = false
				}
				continue
			}
			d.Members = append(d.Members, method(f, KindFunction))
		}
	}
	for _, name := range sortedKeys(cls.Statics) {
		for _, f := range cls.Statics[name] {
			if d.Kind == KindEnumClass && (f.Name == "values" || f.Name == "valueOf") {
				continue // see enumMembers
			}
			md := method(f, KindFunction)
			md.Flags |= FlagStatic
			d.Members = append(d.Members, md)
		}
	}
	fieldAccess := map[string]*classfile.MemberDetails{}
	fieldIndex := map[string]int{}
	for i := range det.Fields {
		fieldAccess[det.Fields[i].Name] = &det.Fields[i]
		fieldIndex[det.Fields[i].Name] = i
	}
	for _, name := range cls.Entries {
		d.Members = append(d.Members, &Decl{Kind: KindEnumEntry, Name: name, ClassID: id + "." + name,
			Visibility: Public, Type: &Type{Kind: TypeClass, ClassID: id}})
	}
	if d.Kind == KindEnumClass {
		d.Members = append(d.Members, enumMembers(id, l.cp.Has("kotlin/enums/EnumEntries"), FlagStatic|FlagJava)...)
	}
	addField := func(v *types.VarSym, static bool) {
		if slices.Contains(cls.Entries, v.Name) {
			return
		}
		md := &Decl{Kind: KindJavaField, Name: v.Name, Visibility: Public, Modality: ModalityFinal,
			Flags: FlagJava, Type: platform(b.typ(v.Type, nil))}
		if static {
			md.Flags |= FlagStatic
		}
		if fd := fieldAccess[v.Name]; fd != nil {
			md.Visibility = javaVisibility(fd.Access)
			if fd.Access&classfile.AccFinal == 0 {
				md.Flags |= FlagMutable
			}
			md.Annotations = l.annotations(fd.Annotations, false)
			pos[md] = fieldIndex[v.Name]
		}
		d.Members = append(d.Members, md)
	}
	for _, name := range sortedKeys(cls.Fields) {
		addField(cls.Fields[name], true)
	}
	for _, name := range sortedKeys(cls.JavaFields) {
		addField(cls.JavaFields[name], false)
	}
	if h := cls.JavaHidden; h != nil {
		for _, f := range h.Methods {
			kind := KindFunction
			if f.Name == "<init>" {
				kind = KindConstructor
			}
			md := method(f, kind)
			if f.Static {
				md.Flags |= FlagStatic
			}
			d.Members = append(d.Members, md)
		}
		for _, v := range h.Fields {
			addField(v, v.Kind == types.Static)
		}
	}
	if depth < maxNesting {
		for i, n := range det.Nested {
			if nc := l.lib.Class(internalClassID(n)); nc != nil {
				nc.Ensure()
				if !nc.Missing && nc.KotlinName == "" {
					if nd := l.javaClass(nc, depth+1); nd != nil && (nd.Visibility != Private || source) {
						pos[nd] = i
						d.Members = append(d.Members, nd)
					}
				}
			}
		}
	}
	if d.Kind == KindClass || d.Kind == KindInterface {
		slices.SortStableFunc(d.Members, func(a, b *Decl) int {
			if c := cmp.Compare(javaMemberGroup(a), javaMemberGroup(b)); c != 0 {
				return c
			}
			return cmp.Compare(pos[a], pos[b])
		})
	}
	return d
}

// javaMemberGroup orders a Java class's members as FIR's declared scopes
// list them: instance fields, methods and inner classes, then
// constructors, then static fields, methods and nested classes, each in
// class-file order.
func javaMemberGroup(d *Decl) int {
	g := 0
	switch d.Kind {
	case KindJavaField:
	case KindConstructor:
		return 3
	case KindClass, KindInterface, KindEnumClass, KindAnnotationClass, KindObject:
		g = 2
	default:
		g = 1
	}
	if d.Flags&FlagStatic != 0 {
		g += 4
	}
	return g
}

// javaSupertypes returns the supertypes of Java class cls, of kind k, each
// passed through conv: its generic ones if it has a signature, and
// kotlin/Any if it has none.
func javaSupertypes(b *fileBuilder, cls *types.ClassSym, k Kind, conv func(*Type) *Type) []*Type {
	var out []*Type
	switch {
	case cls.Supers != nil:
		for _, st := range cls.Supers {
			if t := b.typ(st, nil); t != nil {
				out = append(out, conv(t))
			}
		}
	default:
		if cls.Super != nil {
			out = append(out, &Type{Kind: TypeClass, ClassID: classID(cls.Super)})
		}
		for _, i := range cls.Interfaces {
			out = append(out, &Type{Kind: TypeClass, ClassID: classID(i)})
		}
	}
	if k == KindInterface || k == KindAnnotationClass {
		// An interface's java/lang/Object superclass isn't a supertype in
		// Kotlin, unless it is the only one.
		out = slices.DeleteFunc(out, func(t *Type) bool {
			return t.ClassID == "kotlin/Any" && len(out) > 1
		})
	}
	if len(out) == 0 {
		out = []*Type{{Kind: TypeClass, ClassID: "kotlin/Any"}}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func javaVisibility(access uint16) Visibility {
	switch {
	case access&classfile.AccPublic != 0:
		return Public
	case access&classfile.AccProtected != 0:
		return Protected
	case access&classfile.AccPrivate != 0:
		return Private
	}
	return PackagePrivate
}

func (l *library) javaTypeParams(b *fileBuilder, tps []*types.TypeParamSym) []*Decl {
	var out []*Decl
	for _, tp := range tps {
		md := &Decl{Kind: KindTypeParameter, Name: tp.Name}
		for _, bt := range tp.Bounds {
			md.Bounds = append(md.Bounds, platform(b.typ(bt, nil)))
		}
		if len(md.Bounds) == 0 { // Object, which is kotlin/Any!
			md.Bounds = []*Type{{Kind: TypeClass, ClassID: "kotlin/Any", Flexible: true}}
		}
		out = append(out, md)
	}
	return out
}

// javaMethod returns the model of Java method or constructor f of class
// d, whose class file's methods are bySig.
func (l *library) javaMethod(b *fileBuilder, f *types.FunSym, bySig map[string]*classfile.MemberDetails, d *Decl, kind Kind) *Decl {
	md := &Decl{Kind: kind, Name: f.Name, Flags: FlagJava, Visibility: Public}
	if kind == KindConstructor {
		md.Name = "<init>"
	}
	name := f.JVMName
	if name == "" {
		name = f.Name
		if kind == KindConstructor {
			name = "<init>"
		}
	}
	m := bySig[name+f.Desc]
	var access uint16
	if m != nil {
		access = m.Access
		md.Visibility = javaVisibility(access)
		md.Annotations = l.annotations(m.Annotations, false)
	}
	switch {
	case kind == KindConstructor:
		md.Modality = ModalityFinal
	case f.Abstract:
		md.Modality = ModalityAbstract
	case f.Static || access&(classfile.AccFinal|classfile.AccPrivate) != 0 || d.Modality == ModalityFinal:
		md.Modality = ModalityFinal
	default:
		md.Modality = ModalityOpen
	}
	md.TypeParams = l.javaTypeParams(b, f.TypeParams)
	for i, p := range f.Params {
		pd := &Decl{Kind: KindValueParameter, Name: "p" + itoa(int64(i))}
		if m != nil && len(m.ParamNames) == len(f.Params) && m.ParamNames[i] != "" {
			pd.Name = m.ParamNames[i]
		}
		t := p.Type
		if p.Vararg {
			pd.Flags |= FlagVararg
			t = types.VarargElem(t)
		}
		pd.Type = platform(b.typ(t, nil))
		if m != nil {
			if j := len(m.Params) - len(f.Params) + i; j >= 0 && j < len(m.Params) {
				pd.Annotations = l.annotations(m.Params[j], false)
			}
		}
		md.Params = append(md.Params, pd)
	}
	if kind == KindFunction {
		md.Type = platform(b.typ(f.Ret, nil))
	}
	return md
}

// platform returns t, a type from a Java class, with its nullable types
// made platform types, as Kotlin sees them: String! for String?.
func platform(t *Type) *Type {
	if t == nil {
		return nil
	}
	if t.Nullable {
		t.Nullable, t.Flexible = false, true
	}
	for _, a := range t.Args {
		platform(a.Type)
	}
	return t
}

// skipped are the annotations of Kotlin class files that are not Kotlin
// annotations, which the Analysis API doesn't show either.
var skipped = map[string]bool{
	"kotlin/Metadata":                           true,
	"kotlin/jvm/internal/SourceDebugExtension":  true,
	"org/jetbrains/annotations/NotNull":         true,
	"org/jetbrains/annotations/Nullable":        true,
	"java/lang/annotation/Retention":            true,
	"java/lang/annotation/Target":               true,
	"java/lang/annotation/Documented":           true,
	"java/lang/annotation/Repeatable":           true,
	"kotlin/jvm/internal/RepeatableContainer":   true,
	"kotlin/annotations/jvm/internal/Container": true,
}

// annotations returns the models of class-file annotations as, of a
// Kotlin declaration if kotlin is set.
func (l *library) annotations(as []classfile.Annotation, kotlin bool) []Annotation {
	var out []Annotation
	for i := range as {
		a := l.annotation(&as[i], 0)
		if kotlin && skipped[internalName(a.ClassID)] {
			continue
		}
		out = append(out, *a)
	}
	return out
}

// annotation returns the model of class-file annotation a.
func (l *library) annotation(a *classfile.Annotation, depth int) *Annotation {
	id := descClassID(a.Type)
	out := &Annotation{ClassID: id}
	hints := l.paramTypes(id)
	for _, p := range a.Values {
		out.Args = append(out.Args, NamedValue{Name: p.Name, Value: l.value(&p.Value, hints[p.Name], depth+1)})
	}
	return out
}

// descClassID returns the class ID of the class a field descriptor
// L...; names, or of an internal name.
func descClassID(desc string) string {
	if strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";") {
		desc = desc[1 : len(desc)-1]
	}
	return internalClassID(desc)
}

// paramTypes returns the class IDs of the types of the parameters of the
// annotation class with class ID id, the element types' for arrays, by
// name: what tells an unsigned argument from a signed one in a class
// file.
func (l *library) paramTypes(id string) map[string]string {
	l.mu.Lock()
	pt, ok := l.params[id]
	l.mu.Unlock()
	if ok {
		return pt
	}
	pt = map[string]string{}
	if kc, _, _ := l.load(internalName(id)); kc != nil && kc.Name == id && len(kc.Ctors) > 0 {
		for _, p := range kc.Ctors[0].Params {
			t := p.Type
			if p.Vararg != nil {
				t = p.Vararg
			}
			pt[p.Name] = paramHint(l.ktype(t, kscope{}, 0))
		}
	}
	l.mu.Lock()
	l.params[id] = pt
	l.mu.Unlock()
	return pt
}

// paramHint returns what value needs of an annotation parameter's type:
// its class ID, or its element type's if it is an array.
func paramHint(t *Type) string {
	if t == nil || t.Kind != TypeClass {
		return ""
	}
	if t.ClassID == "kotlin/Array" && len(t.Args) == 1 && t.Args[0].Type != nil {
		return paramHint(t.Args[0].Type)
	}
	if u, ok := unsignedArrays[t.ClassID]; ok {
		return u
	}
	return t.ClassID
}

var unsignedArrays = map[string]string{
	"kotlin/UByteArray":  "kotlin/UByte",
	"kotlin/UShortArray": "kotlin/UShort",
	"kotlin/UIntArray":   "kotlin/UInt",
	"kotlin/ULongArray":  "kotlin/ULong",
}

// value returns the model of class-file element value v, of a parameter
// whose type (its element type, for an array) is the class hint.
func (l *library) value(v *classfile.ElementValue, hint string, depth int) *Value {
	if depth > 16 {
		return &Value{Kind: ValueUnsupported}
	}
	c := func(k ConstKind) *Value {
		out := &Value{Kind: ValueConst, Const: k}
		switch x := v.Const.(type) {
		case int64:
			out.Bits = x
		case float32:
			out.Float = float64(x)
		case float64:
			out.Float = x
		case string:
			out.Str = x
		}
		return out
	}
	switch v.Tag {
	case 'Z':
		return c(ConstBoolean)
	case 'C':
		return c(ConstChar)
	case 'B':
		if hint == "kotlin/UByte" {
			out := c(ConstUByte)
			out.Bits = int64(uint8(out.Bits))
			return out
		}
		return c(ConstByte)
	case 'S':
		if hint == "kotlin/UShort" {
			out := c(ConstUShort)
			out.Bits = int64(uint16(out.Bits))
			return out
		}
		return c(ConstShort)
	case 'I':
		if hint == "kotlin/UInt" {
			out := c(ConstUInt)
			out.Bits = int64(uint32(out.Bits))
			return out
		}
		return c(ConstInt)
	case 'J':
		if hint == "kotlin/ULong" {
			return c(ConstULong)
		}
		return c(ConstLong)
	case 'F':
		return c(ConstFloat)
	case 'D':
		return c(ConstDouble)
	case 's':
		return c(ConstString)
	case 'e':
		return &Value{Kind: ValueEnum, ClassID: descClassID(v.Enum[0]), Entry: v.Enum[1]}
	case 'c':
		return &Value{Kind: ValueClass, Type: descType(v.Class)}
	case '@':
		if v.Ann == nil {
			return &Value{Kind: ValueUnsupported}
		}
		return &Value{Kind: ValueAnnotation, Annotation: l.annotation(v.Ann, depth+1)}
	case '[':
		out := &Value{Kind: ValueArray}
		for i := range v.Elems {
			out.Elems = append(out.Elems, l.value(&v.Elems[i], hint, depth+1))
		}
		return out
	}
	return &Value{Kind: ValueUnsupported}
}

// primitiveDescs are the Kotlin classes of the JVM's primitive types, and
// the arrays of them.
var primitiveDescs = map[byte][2]string{
	'Z': {"kotlin/Boolean", "kotlin/BooleanArray"},
	'C': {"kotlin/Char", "kotlin/CharArray"},
	'B': {"kotlin/Byte", "kotlin/ByteArray"},
	'S': {"kotlin/Short", "kotlin/ShortArray"},
	'I': {"kotlin/Int", "kotlin/IntArray"},
	'J': {"kotlin/Long", "kotlin/LongArray"},
	'F': {"kotlin/Float", "kotlin/FloatArray"},
	'D': {"kotlin/Double", "kotlin/DoubleArray"},
}

// javaKotlin are the Kotlin classes of Java classes a class literal in a
// class file may name.
var javaKotlin = map[string]string{
	"java/lang/String":       "kotlin/String",
	"java/lang/Integer":      "kotlin/Int",
	"java/lang/Long":         "kotlin/Long",
	"java/lang/Short":        "kotlin/Short",
	"java/lang/Byte":         "kotlin/Byte",
	"java/lang/Character":    "kotlin/Char",
	"java/lang/Boolean":      "kotlin/Boolean",
	"java/lang/Float":        "kotlin/Float",
	"java/lang/Double":       "kotlin/Double",
	"java/lang/Void":         "kotlin/Unit",
	"java/util/List":         "kotlin/collections/List",
	"java/util/Set":          "kotlin/collections/Set",
	"java/util/Map":          "kotlin/collections/Map",
	"java/util/Collection":   "kotlin/collections/Collection",
	"java/lang/Iterable":     "kotlin/collections/Iterable",
	"java/util/Iterator":     "kotlin/collections/Iterator",
	"java/util/Map$Entry":    "kotlin/collections/Map.Entry",
	"java/util/ListIterator": "kotlin/collections/ListIterator",
}

// descType returns the type a class literal's descriptor names, as
// Kotlin sees it: Ljava/lang/String; is kotlin/String, [I kotlin/IntArray.
func descType(desc string) *Type {
	switch {
	case desc == "V":
		return &Type{Kind: TypeClass, ClassID: "kotlin/Unit"}
	case len(desc) == 1:
		if p, ok := primitiveDescs[desc[0]]; ok {
			return &Type{Kind: TypeClass, ClassID: p[0]}
		}
	case len(desc) == 2 && desc[0] == '[':
		if p, ok := primitiveDescs[desc[1]]; ok {
			return &Type{Kind: TypeClass, ClassID: p[1]}
		}
	case strings.HasPrefix(desc, "["):
		elem := descType(desc[1:])
		if elem.Kind != TypeClass {
			return elem
		}
		return &Type{Kind: TypeClass, ClassID: "kotlin/Array", Args: []TypeArg{{Type: elem}}}
	case strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";"):
		name := desc[1 : len(desc)-1]
		if k, ok := javaKotlin[name]; ok {
			return &Type{Kind: TypeClass, ClassID: k}
		}
		return &Type{Kind: TypeClass, ClassID: descClassID(desc)}
	}
	return &Type{Kind: TypeError, Name: desc}
}

// enumMembers returns the static members Kotlin gives enum class id:
// values(), valueOf(value) and, if withEntries, the entries property, as
// FIR has them, with flags.
func enumMembers(id string, withEntries bool, flags Flags) []*Decl {
	e := func() *Type { return &Type{Kind: TypeClass, ClassID: id} }
	out := []*Decl{
		{Kind: KindFunction, Name: "values", Visibility: Public, Modality: ModalityFinal, Flags: flags,
			Type: &Type{Kind: TypeClass, ClassID: "kotlin/Array", Args: []TypeArg{{Type: e()}}}},
		{Kind: KindFunction, Name: "valueOf", Visibility: Public, Modality: ModalityFinal, Flags: flags, Type: e(),
			Params: []*Decl{{Kind: KindValueParameter, Name: "value", Type: &Type{Kind: TypeClass, ClassID: "kotlin/String"}}}},
	}
	if withEntries {
		out = append(out, &Decl{Kind: KindProperty, Name: "entries", Visibility: Public, Modality: ModalityFinal, Flags: flags,
			Type: &Type{Kind: TypeClass, ClassID: "kotlin/enums/EnumEntries", Args: []TypeArg{{Type: e()}}}})
	}
	return out
}
