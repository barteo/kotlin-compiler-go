package check

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// kotlinx.serialization's compiler plugin (-serialization).
//
// A class annotated @Serializable gets from kotlinc's plugin a serializer
// and what the serializer needs. A class C with instances gets:
//
//	// its serializer, an object
//	object $serializer : GeneratedSerializer<C> {
//		override val descriptor: SerialDescriptor // the properties' names
//		override fun childSerializers(): Array<KSerializer<*>>
//		override fun serialize(encoder: Encoder, value: C)
//		override fun deserialize(decoder: Decoder): C
//	}
//	// which its companion object, declared if it has none, returns
//	companion object { fun serializer(): KSerializer<C> = $serializer }
//	// which serialize calls: a static method, internal in a final class
//	fun write$Self(self: C, output: CompositeEncoder, serialDesc: SerialDescriptor)
//	// and which deserialize calls with the properties it read, and a bit
//	// set in seen0 for each: it sets the properties' fields, the default
//	// values of those it didn't read, rather than call the primary
//	// constructor
//	constructor(seen0: Int, ..., serializationConstructorMarker: SerializationConstructorMarker?)
//
// The properties are the primary constructor's and the body's that have
// backing fields, but for those annotated @Transient, after those of the
// class's superclass if it is @Serializable too. An abstract or sealed
// class has write$Self and the constructor, for its subclasses, and its
// companion's serializer() returns a PolymorphicSerializer, or a
// SealedClassSerializer of its subclasses. A sealed interface has only
// the companion's serializer(), which makes a new SealedClassSerializer
// each time. An enum's and an object's serializer() return serializers
// made when first asked for.
//
// kc declares these as source, before the file is checked, and fills
// in the bodies that depend on the properties' types once those are
// known (serialBodies), as Kotlin that calls kotlinx.serialization's
// runtime as the plugin's code does. Codegen writes the synthetic
// constructor (syntax.Serial) and write$Self (FunDecl.SelfStatic).
//
// Generic classes, interfaces that aren't sealed, custom serializers of properties
// (@Serializable(with = ...), @Contextual), annotations that formats read
// (@JsonNames, @ProtoNumber...) and the types kc knows no serializer for
// are unsupported.

// serialInfo is a class annotated @Serializable, to which the checker adds
// the plugin's members.
type serialInfo struct {
	cd   *syntax.ClassDecl
	fq   string // its qualified name
	name string // its serial name
	kind serialKind
	// writeSelf is its write$Self, and ser its $serializer object, for a
	// class with instances.
	writeSelf *syntax.FunDecl
	ser       *syntax.ClassDecl
	// lazy is a sealed class's companion's $cachedSerializer$delegate,
	// whose serializer has its subclasses'.
	lazy *syntax.PropDecl
	// serializer is a sealed interface's companion's serializer().
	serializer *syntax.FunDecl
	// defaults are copies of the Props' default values, nil for those
	// without one, which write$Self compares the properties with.
	defaults []syntax.Expr
}

type serialKind uint8

const (
	serialPlain     serialKind = iota // a class with instances, which has a $serializer
	serialAbstract                    // serialized with a PolymorphicSerializer
	serialSealed                      // serialized with a SealedClassSerializer
	serialInterface                   // a sealed interface's, made each time
	serialEnum
	serialObject
)

// serialDenied are the annotations of classes and properties that change
// how the plugin serializes them, or that formats read from descriptors,
// which kc doesn't support.
var serialDenied = map[string]bool{
	"Contextual": true, "Polymorphic": true, "EncodeDefault": true, "KeepGeneratedSerializer": true, "MetaSerializable": true,
	"JsonNames": true, "JsonClassDiscriminator": true, "JsonIgnoreUnknownKeys": true,
	"ProtoNumber": true, "ProtoType": true, "ProtoPacked": true, "ProtoOneOf": true,
}

// serialization declares the plugin's members of the file's classes
// annotated @Serializable, if kc compiles with the plugin.
func (c *Checker) serialization() {
	if !c.u.serialization {
		return
	}
	for _, a := range c.file.Annotations {
		if c.serialAnno(a, "UseSerializers") || c.serialAnno(a, "UseContextualSerialization") {
			c.unsupported(a.P, "serialization", "@%s is not supported", a.Name)
			return
		}
	}
	prefix := ""
	if c.file.Package != "" {
		prefix = c.file.Package + "."
	}
	seen := map[*syntax.ClassDecl]bool{}
	var visit func(decls []syntax.Decl, outer string)
	visit = func(decls []syntax.Decl, outer string) {
		for _, d := range decls {
			cd, ok := d.(*syntax.ClassDecl)
			if !ok {
				continue
			}
			seen[cd] = true
			name := outer + cd.Name
			visit(cd.Members, name+".")
			if c.serialAnnotated(cd.Annotations) {
				c.serialClass(cd, name)
			}
		}
	}
	visit(c.file.Decls, prefix)
	// Classes declared in blocks, which the plugin serializes too.
	eachNode(reflect.ValueOf(c.file.Decls), map[uintptr]bool{}, func(n any) {
		switch n := n.(type) {
		case *syntax.ClassDecl:
			if !seen[n] && c.serialAnnotated(n.Annotations) {
				c.unsupported(n.P, "serialization", "local @Serializable classes are not supported")
			}
		case *syntax.LocalClass:
			if c.serialAnnotated(n.Annotations) {
				c.unsupported(n.Decl.P, "serialization", "local @Serializable classes are not supported")
			}
		}
	})
}

// eachNode calls fn for each node of a parsed tree.
func eachNode(v reflect.Value, seen map[uintptr]bool, fn func(any)) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] || v.Type().Elem().PkgPath() != posType.PkgPath() {
			return
		}
		seen[v.Pointer()] = true
		fn(v.Interface())
		eachNode(v.Elem(), seen, fn)
	case reflect.Interface:
		if !v.IsNil() {
			eachNode(v.Elem(), seen, fn)
		}
	case reflect.Slice:
		for i := range v.Len() {
			eachNode(v.Index(i), seen, fn)
		}
	case reflect.Struct:
		if v.Type().PkgPath() != posType.PkgPath() {
			return
		}
		for i := range v.NumField() {
			eachNode(v.Field(i), seen, fn)
		}
	}
}

// serialAnnotated reports whether annotations as, a class's, have
// @Serializable, without a serializer of its own (serializableWith
// declares those).
func (c *Checker) serialAnnotated(as []syntax.Annotation) bool {
	for _, a := range as {
		if a.Target == "" && c.serialAnno(a, "Serializable") {
			return len(a.Args) == 0
		}
	}
	return false
}

// serialAnno reports whether annotation a is kotlinx.serialization's
// annotation name, as the file names it.
func (c *Checker) serialAnno(a syntax.Annotation, name string) bool {
	return c.pluginAnno(a, "kotlinx.serialization", name)
}

// pluginAnno reports whether annotation a is the annotation name of
// package pkg, as the file names it.
func (c *Checker) pluginAnno(a syntax.Annotation, pkg, name string) bool {
	if a.Name == pkg+"."+name {
		return true
	}
	if a.Name != name {
		return false
	}
	if c.file.Package == pkg {
		return true
	}
	for _, imp := range c.file.Imports {
		if imp.Star && imp.Path == pkg || !imp.Star && imp.Path == pkg+"."+name && imp.Alias == "" {
			return true
		}
	}
	return false
}

// findSerial returns the annotation of a declaration that is
// kotlinx.serialization's annotation name, or nil.
func (c *Checker) findSerial(as []syntax.Annotation, name string) *syntax.Annotation {
	for i := range as {
		if as[i].Target == "" && c.serialAnno(as[i], name) {
			return &as[i]
		}
	}
	return nil
}

// serialName returns the serial name of a declaration named name: its
// @SerialName, or the name.
func (c *Checker) serialName(as []syntax.Annotation, name string) string {
	if a := c.findSerial(as, "SerialName"); a != nil {
		if s, ok := a.Str("value"); ok {
			return s
		}
	}
	return name
}

// deniedAnnotation returns an annotation among as that kc doesn't
// support on a serializable declaration, or nil.
func deniedAnnotation(as []syntax.Annotation) *syntax.Annotation {
	for i := range as {
		name := as[i].Name[strings.LastIndexByte(as[i].Name, '.')+1:]
		if serialDenied[name] {
			return &as[i]
		}
	}
	return nil
}

// serialClass declares the plugin's members of class cd annotated
// @Serializable, whose qualified name is fq.
func (c *Checker) serialClass(cd *syntax.ClassDecl, fq string) {
	fail := func(format string, args ...any) { c.unsupported(cd.P, "serialization", format, args...) }
	switch {
	case cd.Annotation:
		c.errorf(cd.P, "annotation classes can't be @Serializable")
		return
	case cd.Interface && !cd.Sealed:
		fail("@Serializable interfaces that aren't sealed are not supported")
		return
	case len(cd.TypeParams) > 0:
		fail("@Serializable classes with type parameters are not supported")
		return
	case cd.Inner:
		fail("@Serializable inner classes are not supported")
		return
	case cd.Value:
		fail("@Serializable value classes are not supported")
		return
	case cd.Companion:
		fail("@Serializable companion objects are not supported")
		return
	}
	if a := deniedAnnotation(cd.Annotations); a != nil {
		fail("@%s is not supported", a.Name)
		return
	}
	name := c.serialName(cd.Annotations, fq)
	in := &serialInfo{cd: cd, fq: fq, name: name}
	switch {
	case cd.Interface:
		// serialBodies writes the subclasses' serializers.
		in.kind = serialInterface
		members := c.serialCompanion(cd, fmt.Sprintf("fun serializer(): kotlinx.serialization.KSerializer<%s> { throw java.lang.IllegalStateException() }\n", fq))
		if members != nil {
			in.serializer = members[0].(*syntax.FunDecl)
			c.u.serialInterfaces[cd] = true
			c.serials = append(c.serials, in)
		}
		return
	case cd.Enum:
		in.kind = serialEnum
		c.serialEnum(in, name)
		return
	case cd.Object:
		in.kind = serialObject
		c.serialLazy(in, cd, fmt.Sprintf("kotlinx.serialization.internal.ObjectSerializer(%q, %s)", name, fq))
		return
	case cd.Sealed:
		in.kind = serialSealed
	case cd.Abstract:
		in.kind = serialAbstract
	}
	if !cd.HasPrimary {
		fail("@Serializable classes without a primary constructor are not supported")
		return
	}
	for _, st := range cd.Supers {
		if st.Delegate != nil {
			fail("@Serializable classes that delegate interfaces are not supported")
			return
		}
	}
	serial := &syntax.Serial{Name: name}
	for _, cp := range cd.CtorParams {
		if !cp.IsProp {
			c.errorf(cp.P, "this class is not serializable automatically because it has primary constructor parameters that are not properties")
			return
		}
		if a := deniedAnnotation(cp.Modifiers.Annotations); a != nil {
			c.unsupported(a.P, "serialization", "@%s is not supported", a.Name)
			return
		}
		if c.findSerial(cp.Modifiers.Annotations, "Transient") != nil {
			if cp.Default == nil {
				c.errorf(cp.P, "this property is marked as @Transient and therefore must have an initializing expression")
				return
			}
			continue
		}
		serial.Props = append(serial.Props, &syntax.SerialProp{Name: c.serialName(cp.Modifiers.Annotations, cp.Name), Param: cp,
			Optional: cp.Default != nil && c.findSerial(cp.Modifiers.Annotations, "Required") == nil})
	}
	for _, m := range cd.Members {
		pd, ok := m.(*syntax.PropDecl)
		if !ok {
			continue
		}
		if a := deniedAnnotation(pd.Annotations); a != nil {
			c.unsupported(a.P, "serialization", "@%s is not supported", a.Name)
			return
		}
		transient := c.findSerial(pd.Annotations, "Transient") != nil
		switch {
		case pd.DelegateOf != nil || pd.Field != nil || pd.Recv != nil || pd.Const:
			// A delegate's field, set always, and properties without
			// fields of their own.
		case pd.Lateinit && !transient:
			c.unsupported(pd.P, "serialization", "lateinit properties of @Serializable classes are not supported")
			return
		case transient:
			if pd.Init == nil && pd.Getter == nil {
				c.errorf(pd.P, "this property is marked as @Transient and therefore must have an initializing expression")
				return
			}
		case pd.Init == nil && (pd.Getter != nil || pd.Abstract):
			// Without a field.
		case pd.Init == nil:
			c.unsupported(pd.P, "serialization", "serializable properties without initializers are not supported")
			return
		case pd.Getter != nil || pd.Setter != nil:
			c.unsupported(pd.P, "serialization", "serializable properties with accessors are not supported")
			return
		default:
			serial.Props = append(serial.Props, &syntax.SerialProp{Name: c.serialName(pd.Annotations, pd.Name), Prop: pd,
				Optional: c.findSerial(pd.Annotations, "Required") == nil})
		}
	}
	for _, p := range serial.Props {
		var x syntax.Expr
		if p.Optional {
			x = cloneTree(p.Default())
		}
		in.defaults = append(in.defaults, x)
	}

	// write$Self, internal in a final class, and the $serializer.
	var src strings.Builder
	vis := ""
	if !cd.Open && !cd.Abstract && !cd.Sealed {
		vis = "internal "
	}
	fmt.Fprintf(&src, "%sfun `write$Self`(output: kotlinx.serialization.encoding.CompositeEncoder, serialDesc: kotlinx.serialization.descriptors.SerialDescriptor) {}\n", vis)
	if in.kind == serialPlain {
		fmt.Fprintf(&src, "@kotlin.Deprecated(message = \"This synthesized declaration should not be used directly\", level = kotlin.DeprecationLevel.HIDDEN)\n"+
			"object `$serializer` : kotlinx.serialization.internal.GeneratedSerializer<%[1]s> {\n"+
			"final override val descriptor: kotlinx.serialization.descriptors.SerialDescriptor\n"+
			"init { descriptor = kotlinx.serialization.internal.PluginGeneratedSerialDescriptor(\"\", this, 0) }\n"+
			"final override fun childSerializers(): Array<kotlinx.serialization.KSerializer<*>> = arrayOf<kotlinx.serialization.KSerializer<*>>()\n"+
			"final override fun serialize(encoder: kotlinx.serialization.encoding.Encoder, value: %[1]s) {\n"+
			"val `$d` = descriptor\n"+
			"val `$c` = encoder.beginStructure(`$d`)\n"+
			"value.`write$Self`(`$c`, `$d`)\n"+
			"`$c`.endStructure(`$d`)\n"+
			"}\n"+
			"final override fun deserialize(decoder: kotlinx.serialization.encoding.Decoder): %[1]s { throw java.lang.IllegalStateException() }\n"+
			"}\n", fq)
	}
	members, err := parseMembers(c.file.Path, cd.P, src.String())
	if err != nil {
		c.errorf(cd.P, "serialization: %v", err)
		return
	}
	in.writeSelf = members[0].(*syntax.FunDecl)
	in.writeSelf.Plugin, in.writeSelf.SelfStatic = true, true
	if in.kind == serialPlain {
		in.ser = members[1].(*syntax.ClassDecl)
		in.ser.Synthetic = true
	}
	serial.Ctor = &syntax.SecondaryCtor{P: cd.P, Serial: serial}
	cd.Members = append(cd.Members, members...)
	cd.Members = append(cd.Members, serial.Ctor)
	cd.Serial = serial
	c.serials = append(c.serials, in)

	switch in.kind {
	case serialPlain:
		c.serialCompanion(cd, fmt.Sprintf("fun serializer(): kotlinx.serialization.KSerializer<%[1]s> = %[1]s.`$serializer`\n", fq))
	case serialAbstract:
		c.serialLazy(in, nil, fmt.Sprintf("kotlinx.serialization.PolymorphicSerializer(%s::class)", fq))
	case serialSealed:
		// serialBodies writes the subclasses' serializers.
		c.serialLazy(in, nil, "throw java.lang.IllegalStateException()")
	}
}

// serialEnum declares an enum's serializer(), which returns the serializer
// of an enum named name: kotlinx.serialization's, with the entries'
// @SerialNames if they have them.
func (c *Checker) serialEnum(in *serialInfo, name string) {
	named := false
	names := make([]string, len(in.cd.Entries))
	for i, e := range in.cd.Entries {
		if a := deniedAnnotation(e.Annotations); a != nil {
			c.unsupported(a.P, "serialization", "@%s is not supported", a.Name)
			return
		}
		names[i] = "null"
		if c.findSerial(e.Annotations, "SerialName") != nil {
			names[i] = fmt.Sprintf("%q", c.serialName(e.Annotations, e.Name))
			named = true
		}
	}
	if !named {
		c.serialLazy(in, nil, fmt.Sprintf("kotlinx.serialization.internal.createSimpleEnumSerializer(%q, %s.values())", name, in.fq))
		return
	}
	nulls := strings.TrimSuffix(strings.Repeat("null, ", len(names)), ", ")
	c.serialLazy(in, nil, fmt.Sprintf("kotlinx.serialization.internal.createAnnotatedEnumSerializer(%q, %s.values(), arrayOf<kotlin.String?>(%s), arrayOf<Array<kotlin.Annotation>?>(%s), null)",
		name, in.fq, strings.Join(names, ", "), nulls))
}

// serialLazy declares serializer() in object obj, or else in the class's
// companion object: it returns the serializer that x makes, made when first
// asked for and kept in $cachedSerializer$delegate, as the plugin's.
func (c *Checker) serialLazy(in *serialInfo, obj *syntax.ClassDecl, x string) {
	src := fmt.Sprintf("private val `$cachedSerializer$delegate`: kotlin.Lazy<kotlinx.serialization.KSerializer<*>> = kotlin.lazy(kotlin.LazyThreadSafetyMode.PUBLICATION) { %s }\n"+
		"fun serializer(): kotlinx.serialization.KSerializer<%[2]s> = `$cachedSerializer$delegate`.value as kotlinx.serialization.KSerializer<%[2]s>\n", x, in.fq)
	if obj != nil {
		members, err := parseMembers(c.file.Path, obj.P, src)
		if err != nil {
			c.errorf(obj.P, "serialization: %v", err)
			return
		}
		members[0].(*syntax.PropDecl).Plugin = true
		obj.Members = append(obj.Members, members...)
		return
	}
	if members := c.serialCompanion(in.cd, src); members != nil {
		in.lazy = members[0].(*syntax.PropDecl)
		in.lazy.Plugin = true
	}
}

// serialCompanion declares members, src, in the companion object of class
// cd, which it declares if cd has none, and returns them.
func (c *Checker) serialCompanion(cd *syntax.ClassDecl, src string) []syntax.Decl {
	members, err := parseMembers(c.file.Path, cd.P, src)
	if err != nil {
		c.errorf(cd.P, "serialization: %v", err)
		return nil
	}
	for _, m := range cd.Members {
		if comp, ok := m.(*syntax.ClassDecl); ok && comp.Companion {
			comp.Members = append(comp.Members, members...)
			return members
		}
	}
	comp := &syntax.ClassDecl{P: cd.P, Name: "Companion", Object: true, HasPrimary: true, Members: members}
	comp.Companion = true
	cd.Members = append(cd.Members, comp)
	return members
}

// cloneTree returns a copy of a parsed tree, not yet checked.
func cloneTree[T any](x T) T {
	v := reflect.ValueOf(&x).Elem()
	out := reflect.New(v.Type()).Elem()
	cloneValue(out, v, map[uintptr]reflect.Value{})
	return out.Interface().(T)
}

func cloneValue(dst, src reflect.Value, seen map[uintptr]reflect.Value) {
	switch src.Kind() {
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		if src.Type().Elem().PkgPath() != posType.PkgPath() {
			dst.Set(src) // a symbol, which parsing leaves nil
			return
		}
		if p, ok := seen[src.Pointer()]; ok {
			dst.Set(p)
			return
		}
		p := reflect.New(src.Type().Elem())
		seen[src.Pointer()] = p
		cloneValue(p.Elem(), src.Elem(), seen)
		dst.Set(p)
	case reflect.Interface:
		if src.IsNil() {
			return
		}
		e := reflect.New(src.Elem().Type()).Elem()
		cloneValue(e, src.Elem(), seen)
		dst.Set(e)
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		s := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
		for i := range src.Len() {
			cloneValue(s.Index(i), src.Index(i), seen)
		}
		dst.Set(s)
	case reflect.Map:
		if src.IsNil() {
			return
		}
		m := reflect.MakeMapWithSize(src.Type(), src.Len())
		for it := src.MapRange(); it.Next(); {
			e := reflect.New(src.Type().Elem()).Elem()
			cloneValue(e, it.Value(), seen)
			m.SetMapIndex(it.Key(), e)
		}
		dst.Set(m)
	case reflect.Struct:
		if src.Type().PkgPath() != posType.PkgPath() {
			dst.Set(src)
			return
		}
		for i := range src.NumField() {
			if f := dst.Field(i); f.CanSet() {
				cloneValue(f, src.Field(i), seen)
			}
		}
	default:
		dst.Set(src)
	}
}

// replaceIdents replaces the names in a parsed tree that are keys of
// repl, used as expressions, with repl's expressions.
func replaceIdents(v reflect.Value, repl map[string]syntax.Expr) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			replaceIdents(v.Elem(), repl)
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		if id, ok := v.Interface().(*syntax.Ident); ok && v.CanSet() {
			if x, ok := repl[id.Name]; ok && v.Type() == reflect.TypeFor[syntax.Expr]() {
				v.Set(reflect.ValueOf(&x).Elem())
				return
			}
		}
		replaceIdents(v.Elem(), repl)
	case reflect.Slice:
		for i := range v.Len() {
			replaceIdents(v.Index(i), repl)
		}
	case reflect.Struct:
		if v.Type().PkgPath() != posType.PkgPath() {
			return
		}
		for i := range v.NumField() {
			replaceIdents(v.Field(i), repl)
		}
	}
}

// serialBodies writes what the plugin's members of the file's classes need
// the properties' types for: the synthetic constructors' parameters, and
// the bodies that write and read the properties. It runs once the types
// are inferred, and before bodies are checked.
func (c *Checker) serialBodies() {
	for _, in := range c.serials {
		c.serialFill(in)
	}
}

// serialFill writes the plugin's members of one class that need types.
func (c *Checker) serialFill(in *serialInfo) {
	cd := in.cd
	cls := cd.Sym
	if cls == nil {
		return
	}
	fail := func(format string, args ...any) { c.unsupported(cd.P, "serialization", format, args...) }
	switch in.kind {
	case serialInterface:
		c.sealedSerializer(in)
		return
	case serialSealed:
		c.sealedSerializer(in)
	}
	serial := cd.Serial
	// The superclass's properties come first, if it is @Serializable.
	// Otherwise, the constructor calls its constructor without arguments.
	var all []*syntax.SerialProp
	if s := cls.Super; s != nil {
		s.Ensure()
		switch sd, _ := s.Decl.(*syntax.ClassDecl); {
		case sd != nil && sd.Serial != nil:
			serial.Super = sd
			all = serialProps(sd)
		case sd == nil && (c.serializerObject(s) != nil || c.hasSerializerFun(s)):
			fail("@Serializable classes with a @Serializable superclass from another module are not supported")
			return
		case !hasNoArgCtor(superCands(cls)):
			fail("a @Serializable class's superclass that isn't serializable needs a constructor without parameters")
			return
		}
	}
	all = append(all, serial.Props...)
	if len(all) > 32 {
		fail("@Serializable classes with more than 32 properties are not supported")
		return
	}
	inherited := len(all) - len(serial.Props)
	sers := make([]string, len(all))
	for i, p := range all {
		t := p.Sym().Type
		if sers[i] = c.serialSerializer(t.NonNull()); sers[i] != "" {
			continue
		}
		if i >= inherited { // the superclass's are reported with it
			var pos syntax.Pos
			if p.Param != nil {
				pos = p.Param.P
			} else {
				pos = p.Prop.P
			}
			c.unsupported(pos, "serialization", "serialization can't write a property of type %s", t)
		}
		return
	}
	marker := c.u.ext.class("kotlinx/serialization/internal/SerializationConstructorMarker")
	if marker == nil {
		fail("kotlinx.serialization is not on the class path")
		return
	}

	// The synthetic constructor.
	sc := serial.Ctor
	f := &types.FunSym{Name: "<init>", Owner: cls, Ret: cls.This(), Resolved: true, Decl: sc}
	param := func(name string, t types.Type) {
		p := &syntax.Param{P: cd.P, Name: name}
		p.Sym = c.paramSym(p, t, p)
		sc.Params = append(sc.Params, p)
		f.Params = append(f.Params, p.Sym)
	}
	param("seen0", types.IntType)
	for _, p := range all {
		t := p.Sym().Type
		if !t.IsPrimitive() {
			t = t.Nullable() // null where it wasn't read
		}
		param(p.Sym().Name, t)
	}
	param("serializationConstructorMarker", types.ClassType(marker).Nullable())
	sc.Sym = f
	cls.Ctors = append(cls.Ctors, f)

	// write$Self writes the superclass's properties, then its own: those
	// with default values only if they differ, or if the format asks to.
	var b strings.Builder
	if sd := serial.Super; sd != nil {
		fmt.Fprintf(&b, "(this as %s).`write$Self`(output, serialDesc)\n", fqName(sd.Sym))
	}
	repl := map[string]syntax.Expr{}
	for i, p := range serial.Props {
		j := inherited + i
		v := "this.`" + p.Sym().Name + "`"
		enc := serialEncode(p.Sym().Type, j, sers[j], v)
		if in.defaults[i] == nil {
			b.WriteString(enc)
			continue
		}
		def := fmt.Sprintf("$default%d", i)
		repl[def] = in.defaults[i]
		fmt.Fprintf(&b, "if (output.shouldEncodeElementDefault(serialDesc, %d) || %s != `%s`) {\n%s}\n", j, v, def, enc)
	}
	body := c.serialParse(cd, "fun f() {\n"+b.String()+"}\n")
	if body == nil {
		return
	}
	replaceIdents(reflect.ValueOf(body), repl)
	in.writeSelf.Body = body.Body

	if in.ser == nil {
		return // abstract
	}
	c.serializerBodies(in, all, sers)
}

// serializerBodies writes the members of a class's $serializer that
// depend on its properties, all, whose serializers are sers.
func (c *Checker) serializerBodies(in *serialInfo, all []*syntax.SerialProp, sers []string) {
	cd := in.cd
	// The descriptor names the properties.
	var b strings.Builder
	fmt.Fprintf(&b, "val `$d` = kotlinx.serialization.internal.PluginGeneratedSerialDescriptor(%q, this, %d)\n", cd.Serial.Name, len(all))
	for _, p := range all {
		fmt.Fprintf(&b, "`$d`.addElement(%q, %t)\n", p.Name, p.Optional)
	}
	b.WriteString("descriptor = `$d`\n")
	initBody := c.serialParse(cd, "fun f() {\n"+b.String()+"}\n")

	// childSerializers.
	b.Reset()
	b.WriteString("fun f() = arrayOf<kotlinx.serialization.KSerializer<*>>(")
	for i, p := range all {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(serialNullable(p.Sym().Type, sers[i]))
	}
	b.WriteString(")\n")
	children := c.serialParse(cd, b.String())

	// deserialize reads the properties in order, if the format can, or
	// else in the order the format finds them.
	b.Reset()
	b.WriteString("fun f() {\n" +
		"val `$d` = descriptor\n" +
		"var `$flag` = true\n" +
		"var `$index`: kotlin.Int\n" +
		"var `$seen0` = 0\n")
	for i, p := range all {
		t := p.Sym().Type
		typ, _ := kotlinType(t)
		zero := "null"
		switch t.K {
		case types.Int:
			zero = "0"
		case types.Long:
			zero = "0L"
		case types.Short:
			zero = "0.toShort()"
		case types.Byte:
			zero = "0.toByte()"
		case types.Char:
			zero = "'\\u0000'"
		case types.Float:
			zero = "0.0f"
		case types.Double:
			zero = "0.0"
		case types.Bool:
			zero = "false"
		}
		if !t.IsPrimitive() {
			typ, _ = kotlinType(t.Nullable())
		}
		fmt.Fprintf(&b, "var `$v%d`: %s = %s\n", i, typ, zero)
	}
	b.WriteString("val `$c` = decoder.beginStructure(`$d`)\n" +
		"if (`$c`.decodeSequentially()) {\n")
	for i, p := range all {
		b.WriteString(serialDecode(p.Sym().Type, i, sers[i]))
	}
	b.WriteString("} else {\n" +
		"while (`$flag`) {\n" +
		"`$index` = `$c`.decodeElementIndex(`$d`)\n" +
		"when (`$index`) {\n" +
		"-1 -> `$flag` = false\n")
	for i, p := range all {
		fmt.Fprintf(&b, "%d -> {\n%s}\n", i, serialDecode(p.Sym().Type, i, sers[i]))
	}
	b.WriteString("else -> throw kotlinx.serialization.UnknownFieldException(`$index`)\n" +
		"}\n" +
		"}\n" +
		"}\n" +
		"`$c`.endStructure(`$d`)\n")
	fmt.Fprintf(&b, "return %s(`$seen0`", in.fq)
	for i := range all {
		fmt.Fprintf(&b, ", `$v%d`", i)
	}
	b.WriteString(", null)\n}\n")
	deserialize := c.serialParse(cd, b.String())
	if initBody == nil || children == nil || deserialize == nil {
		return
	}
	for _, m := range in.ser.Members {
		switch m := m.(type) {
		case *syntax.InitBlock:
			m.Body = initBody.Body
		case *syntax.FunDecl:
			switch m.Name {
			case "childSerializers":
				m.ExprBody = children.ExprBody
			case "deserialize":
				m.Body = deserialize.Body
			}
		}
	}
}

// serialParse parses a function that kc writes for class cd.
func (c *Checker) serialParse(cd *syntax.ClassDecl, src string) *syntax.FunDecl {
	members, err := parseMembers(c.file.Path, cd.P, src)
	if err != nil {
		c.errorf(cd.P, "serialization: %v", err)
		return nil
	}
	return members[0].(*syntax.FunDecl)
}

// serialProps returns the properties that the serializer of class cd
// writes: its superclass's, then its own.
func serialProps(cd *syntax.ClassDecl) []*syntax.SerialProp {
	var out []*syntax.SerialProp
	if s := cd.Sym.Super; s != nil {
		if sd, ok := s.Decl.(*syntax.ClassDecl); ok && sd.Serial != nil {
			out = serialProps(sd)
		}
	}
	return append(out, cd.Serial.Props...)
}

// sealedSerializer writes the serializer of a sealed class or interface:
// a SealedClassSerializer of its subclasses, the classes and objects that
// inherit from it, directly or through sealed classes and interfaces.
func (c *Checker) sealedSerializer(in *serialInfo) {
	if in.lazy == nil && in.serializer == nil {
		return
	}
	var classes, sers []string
	var visit func(cls *types.ClassSym) bool
	visit = func(cls *types.ClassSym) bool {
		for _, sub := range cls.Subclasses {
			sd, ok := sub.Decl.(*syntax.ClassDecl)
			switch {
			case !ok:
				return false
			case sd.Sealed && (sd.Serial != nil || c.u.serialInterfaces[sd]):
				if !visit(sub) {
					return false
				}
				continue
			case sd.Serial != nil && !sd.Abstract:
				sers = append(sers, fqName(sub)+".`$serializer`")
			case sd.Object && c.hasSerializerFun(sub):
				sers = append(sers, fqName(sub)+".serializer()")
			default:
				c.unsupported(sd.P, "serialization", "subclasses of @Serializable sealed classes that aren't @Serializable are not supported")
				return false
			}
			classes = append(classes, fqName(sub)+"::class")
		}
		return true
	}
	if !visit(in.cd.Sym) {
		return
	}
	x := fmt.Sprintf("kotlinx.serialization.SealedClassSerializer(%q, %[2]s::class, arrayOf<kotlin.reflect.KClass<out %[2]s>>(%[3]s), arrayOf<kotlinx.serialization.KSerializer<out %[2]s>>(%[4]s))",
		in.name, in.fq, strings.Join(classes, ", "), strings.Join(sers, ", "))
	if in.serializer != nil {
		if f := c.serialParse(in.cd, fmt.Sprintf("fun f(): kotlinx.serialization.KSerializer<%s> = %s\n", in.fq, x)); f != nil {
			in.serializer.Body, in.serializer.ExprBody = nil, f.ExprBody
		}
		return
	}
	src := fmt.Sprintf("val x: kotlin.Lazy<kotlinx.serialization.KSerializer<*>> = kotlin.lazy(kotlin.LazyThreadSafetyMode.PUBLICATION) { %s }\n", x)
	members, err := parseMembers(c.file.Path, in.cd.P, src)
	if err != nil {
		c.errorf(in.cd.P, "serialization: %v", err)
		return
	}
	in.lazy.Init = members[0].(*syntax.PropDecl).Init
}

// serialPrimitives are the types whose values the plugin writes and reads
// with encoders' and decoders' own methods, encodeIntElement and others,
// by those methods' names.
var serialPrimitives = map[types.Kind]string{
	types.Int: "Int", types.Long: "Long", types.Short: "Short", types.Byte: "Byte", types.Char: "Char",
	types.Float: "Float", types.Double: "Double", types.Bool: "Boolean", types.String: "String",
}

// serialEncode returns the statement of write$Self that writes element i,
// v, of type t, whose serializer is ser.
func serialEncode(t types.Type, i int, ser, v string) string {
	if m := serialPrimitives[t.K]; m != "" && !t.N {
		return fmt.Sprintf("output.encode%sElement(serialDesc, %d, %s)\n", m, i, v)
	}
	if t.N {
		return fmt.Sprintf("output.encodeNullableSerializableElement(serialDesc, %d, %s, %s)\n", i, ser, v)
	}
	return fmt.Sprintf("output.encodeSerializableElement(serialDesc, %d, %s, %s)\n", i, ser, v)
}

// serialDecode returns the statements of deserialize that read element i,
// of type t, whose serializer is ser, into its local. A collection's
// serializer reads a List, Set or Map: the value is cast to t, as
// kotlinc's code does.
func serialDecode(t types.Type, i int, ser string) string {
	var x string
	typ, _ := kotlinType(t)
	switch m := serialPrimitives[t.K]; {
	case m != "" && !t.N:
		x = fmt.Sprintf("`$c`.decode%sElement(`$d`, %d)", m, i)
	case t.N:
		x = fmt.Sprintf("(`$c`.decodeNullableSerializableElement(`$d`, %d, %s, `$v%d`) as %s)", i, ser, i, typ)
	default:
		x = fmt.Sprintf("(`$c`.decodeSerializableElement(`$d`, %d, %s, `$v%d`) as %s)", i, ser, i, typ)
	}
	return fmt.Sprintf("`$v%d` = %s\n`$seen0` = `$seen0` or %d\n", i, x, 1<<i)
}

// serialNullable returns the serializer of a nullable type t, from ser,
// its non-null type's.
func serialNullable(t types.Type, ser string) string {
	if t.N {
		return "kotlinx.serialization.internal.NullableSerializer(" + ser + ")"
	}
	return ser
}

// serialSerializer returns the Kotlin of a serializer of the values of
// type t, which isn't nullable, as the plugin finds it, or "" if kc
// doesn't know it.
func (c *Checker) serialSerializer(t types.Type) string {
	if t.K == types.Bool {
		return "kotlinx.serialization.internal.BooleanSerializer"
	}
	if m := serialPrimitives[t.K]; m != "" {
		return "kotlinx.serialization.internal." + m + "Serializer"
	}
	if t.K != types.Class {
		return ""
	}
	cls := t.C
	cls.Ensure()
	if cls.Value || t.IsFunc() {
		return ""
	}
	// The serializers of the type arguments.
	arg := func(i int) string {
		if t.A == nil || i >= t.A.Len() || t.A.Get(i).V == types.Star {
			return ""
		}
		at := t.A.Get(i).T
		s := c.serialSerializer(at.NonNull())
		if s == "" {
			return ""
		}
		return serialNullable(at, s)
	}
	of := func(class string, n int) string {
		args := make([]string, n)
		for i := range args {
			if args[i] = arg(i); args[i] == "" {
				return ""
			}
		}
		return "kotlinx.serialization.internal." + class + "(" + strings.Join(args, ", ") + ")"
	}
	name := cls.KotlinName
	if name == "" {
		name = cls.Name
	}
	switch name {
	case "kotlin/collections/List", "kotlin/collections/MutableList", "kotlin/collections/Collection", "kotlin/collections/MutableCollection",
		"java/util/List", "java/util/ArrayList", "java/util/Collection":
		return of("ArrayListSerializer", 1)
	case "kotlin/collections/Set", "kotlin/collections/MutableSet", "java/util/Set", "java/util/LinkedHashSet":
		return of("LinkedHashSetSerializer", 1)
	case "java/util/HashSet":
		return of("HashSetSerializer", 1)
	case "kotlin/collections/Map", "kotlin/collections/MutableMap", "java/util/Map", "java/util/LinkedHashMap":
		return of("LinkedHashMapSerializer", 2)
	case "java/util/HashMap":
		return of("HashMapSerializer", 2)
	case "kotlin/collections/Map.Entry", "kotlin/collections/MutableMap.MutableEntry", "java/util/Map$Entry":
		return of("MapEntrySerializer", 2)
	case "kotlin/Pair":
		return of("PairSerializer", 2)
	case "kotlin/Triple":
		return of("TripleSerializer", 3)
	}
	if len(cls.TypeParams) > 0 || cls.Array {
		return ""
	}
	if s := c.serializerObject(cls); s != nil {
		return fqName(s.Outer) + ".`$serializer`"
	}
	if c.hasSerializerFun(cls) {
		return fqName(cls) + ".serializer()"
	}
	if cls.Entries != nil {
		return fmt.Sprintf("kotlinx.serialization.internal.createSimpleEnumSerializer(%q, %s.values())", fqName(cls), fqName(cls))
	}
	if cls.Interface && !cls.Sealed && !strings.HasPrefix(name, "kotlin/") && !strings.HasPrefix(name, "java/") {
		return "kotlinx.serialization.PolymorphicSerializer(" + fqName(cls) + "::class)"
	}
	return ""
}

// serializerObject returns the $serializer object that the plugin gave
// class cls, or nil.
func (c *Checker) serializerObject(cls *types.ClassSym) *types.ClassSym {
	var s *types.ClassSym
	if cls.External {
		s = c.u.ext.class(cls.Name + "$$serializer")
		if s != nil {
			s.Ensure()
			s.Outer = cls
		}
	} else if cd, ok := cls.Decl.(*syntax.ClassDecl); ok && cd.Serial != nil {
		s = cls.Nested["$serializer"]
	}
	if s == nil || !s.Object {
		return nil
	}
	return s
}

// hasSerializerFun reports whether class cls, an object, or its companion
// object has a function serializer() without parameters: what the plugin
// gives classes that its serializers serialize.
func (c *Checker) hasSerializerFun(cls *types.ClassSym) bool {
	target := cls
	if !cls.Object {
		target = cls.Companion
	}
	if target == nil {
		return false
	}
	target.Ensure()
	for _, f := range target.Methods["serializer"] {
		if len(f.Params) == 0 && len(f.TypeParams) == 0 && !f.Ext {
			return true
		}
	}
	return false
}
