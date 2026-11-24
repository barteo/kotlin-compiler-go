package check

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
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
// made when first asked for. A value class has only the $serializer and
// the companion's serializer(): its $serializer writes and reads its
// property inline.
//
// A generic class C<T, ...> gets a $serializer class rather than an
// object, whose constructor takes the type arguments' serializers,
// typeSerial0 and on, which it keeps in private fields and returns from
// typeParametersSerializers. The companion's serializer(typeSerial0, ...)
// makes one each time; an abstract or sealed class's takes them too, and
// passes those of the type parameters a subclass gives the class on to
// the subclass's serializer, a PolymorphicSerializer of Any for the
// others. write$Self takes them as well, and passes a superclass's
// write$Self the serializers of the type arguments the class gives it.
// The deserialization constructor, which has no $serializer object to
// ask, throws its missing-field exception with the class's static
// $cachedDescriptor.
//
// kc declares these as source, before the file is checked, and fills
// in the bodies that depend on the properties' types once those are
// known (serialBodies), as Kotlin that calls kotlinx.serialization's
// runtime as the plugin's code does. Codegen writes the synthetic
// constructor (syntax.Serial) and write$Self (FunDecl.SelfStatic).
//
// A property annotated @Serializable(with = S::class) is written with S,
// an object, or a new S each time, where kotlinc's plugin keeps one in the
// class's $childSerializers.
//
// A property @Contextual marks, or whose type a file's
// @UseContextualSerialization names, is written with a
// ContextualSerializer, which asks the format's module; one @Polymorphic
// marks with a PolymorphicSerializer; and one whose type a file's
// @UseSerializers names with that serializer. A class of more than 32
// properties has a mask for each 32, seen0 and on.
//
// Generic value classes, interfaces that aren't sealed, generic
// serializers of properties, annotations that formats read (@JsonNames,
// @ProtoNumber...) and the types kc knows no serializer for are
// unsupported.

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
	// encode is each of the Props' @EncodeDefault mode.
	encode []encodeMode
	// tparams are the names of a generic class's type parameters. Its
	// serializer is made of their serializers, typeSerial0 and on, which
	// its $serializer keeps, write$Self takes and the companion's
	// serializer takes.
	tparams []string
	// cached is the init block of a generic class's companion that sets
	// $cachedDescriptor, the descriptor that the synthetic constructor
	// reports missing properties with, as the class has no $serializer
	// object to ask.
	cached *syntax.InitBlock
}

// typeArgs returns the class's type parameters as type arguments, <T, U>,
// or "" if it has none.
func (in *serialInfo) typeArgs() string {
	if len(in.tparams) == 0 {
		return ""
	}
	return "<" + strings.Join(in.tparams, ", ") + ">"
}

// starType returns the class's type with star projections for its type
// arguments, C<*, *>, or its name if it has none.
func (in *serialInfo) starType() string {
	if len(in.tparams) == 0 {
		return in.fq
	}
	return in.fq + "<" + strings.TrimSuffix(strings.Repeat("*, ", len(in.tparams)), ", ") + ">"
}

// typeSerialParams returns the parameters of the serializers of type
// parameters tps: typeSerial0: KSerializer<T>, and on.
func typeSerialParams(tps []string) string {
	ps := make([]string, len(tps))
	for i, tp := range tps {
		ps[i] = fmt.Sprintf("typeSerial%d: kotlinx.serialization.KSerializer<%s>", i, tp)
	}
	return strings.Join(ps, ", ")
}

// typeSerialArgs returns the arguments typeSerial0, and on, of n
// serializers of type parameters.
func typeSerialArgs(n int) string {
	as := make([]string, n)
	for i := range as {
		as[i] = fmt.Sprintf("typeSerial%d", i)
	}
	return strings.Join(as, ", ")
}

// serializerHead returns the head of the companion's serializer() of the
// class, which for a generic class takes its type parameters'
// serializers, as the plugin's: fun <T> serializer(typeSerial0:
// KSerializer<T>): KSerializer<C<T>>.
func (in *serialInfo) serializerHead() string {
	if len(in.tparams) == 0 {
		return fmt.Sprintf("fun serializer(): kotlinx.serialization.KSerializer<%s>", in.fq)
	}
	return fmt.Sprintf("fun %s serializer(%s): kotlinx.serialization.KSerializer<%s%s>",
		in.typeArgs(), typeSerialParams(in.tparams), in.fq, in.typeArgs())
}

// copyTypeParams gives a declaration that kc parsed for the plugin, whose
// type parameters are named as class cd's, cd's type parameters, without
// their variance and, as the plugin's, their bounds.
func copyTypeParams(cd *syntax.ClassDecl, tps []*syntax.TypeParam) {
	for i, tp := range cloneTree(cd.TypeParams) {
		tp.Variance = types.Inv
		tp.Bounds, tp.DeclBound, tp.WhereBound = nil, false, false
		tp.P = tps[i].P
		tps[i] = tp
	}
}

// encodeMode is when write$Self writes a property that has a default
// value, as its @EncodeDefault says.
type encodeMode uint8

const (
	encodeIfFormat encodeMode = iota // if it isn't the default, or the format asks to
	encodeAlways                     // @EncodeDefault, or @EncodeDefault(ALWAYS)
	encodeNever                      // @EncodeDefault(NEVER): if it isn't the default
)

type serialKind uint8

const (
	serialPlain     serialKind = iota // a class with instances, which has a $serializer
	serialAbstract                    // serialized with a PolymorphicSerializer
	serialSealed                      // serialized with a SealedClassSerializer
	serialInterface                   // a sealed interface's, made each time
	serialEnum
	serialObject
	serialValue // a value class, whose $serializer writes its value inline
)

// serialDenied are the annotations of classes and properties that change
// how the plugin serializes them, or that formats read from descriptors,
// which kc doesn't support.
var serialDenied = map[string]bool{
	"KeepGeneratedSerializer": true, "MetaSerializable": true,
	"JsonNames": true, "JsonClassDiscriminator": true, "JsonIgnoreUnknownKeys": true,
	"ProtoNumber": true, "ProtoType": true, "ProtoPacked": true, "ProtoOneOf": true,
}

// serialization declares the plugin's members of the file's classes
// annotated @Serializable, if kc compiles with the plugin.
func (c *Checker) serialization() {
	if !c.u.serialization {
		return
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
	case len(cd.TypeParams) > 0 && cd.Value:
		fail("generic @Serializable value classes are not supported")
		return
	case cd.Inner:
		fail("@Serializable inner classes are not supported")
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
	for _, tp := range cd.TypeParams {
		in.tparams = append(in.tparams, tp.Name)
	}
	switch {
	case cd.Interface:
		// serialBodies writes the subclasses' serializers.
		in.kind = serialInterface
		members := c.serialCompanion(cd, in.serializerHead()+" { throw java.lang.IllegalStateException() }\n")
		if members != nil {
			in.serializer = members[0].(*syntax.FunDecl)
			copyTypeParams(cd, in.serializer.TypeParams)
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
	case cd.Value:
		in.kind = serialValue
		c.serialValueClass(in)
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
			// Required, as it has no initializer to fall back on.
			serial.Props = append(serial.Props, &syntax.SerialProp{Name: c.serialName(pd.Annotations, pd.Name), Prop: pd})
		case transient:
			if pd.Init == nil && pd.Getter == nil && !pd.Lateinit {
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
		in.encode = append(in.encode, c.encodeDefault(p.Annotations()))
	}

	// write$Self, internal in a final class, and the $serializer.
	var src strings.Builder
	vis := ""
	if !cd.Open && !cd.Abstract && !cd.Sealed {
		vis = "internal "
	}
	typeSerials := ""
	if len(in.tparams) > 0 {
		typeSerials = ", " + typeSerialParams(in.tparams)
	}
	fmt.Fprintf(&src, "%sfun `write$Self`(output: kotlinx.serialization.encoding.CompositeEncoder, serialDesc: kotlinx.serialization.descriptors.SerialDescriptor%s) {}\n", vis, typeSerials)
	if in.kind == serialPlain {
		args := ""
		if len(in.tparams) > 0 {
			args = ", " + typeSerialArgs(len(in.tparams))
		}
		serialize := "val `$d` = descriptor\n" +
			"val `$c` = encoder.beginStructure(`$d`)\n" +
			"value.`write$Self`(`$c`, `$d`" + args + ")\n" +
			"`$c`.endStructure(`$d`)\n"
		if len(in.tparams) > 0 {
			src.WriteString(genericSerializerSrc(fq, in.tparams, serialize))
		} else {
			src.WriteString(serializerObjectSrc(fq, serialize))
		}
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
		if len(in.tparams) > 0 {
			copyTypeParams(cd, in.ser.TypeParams)
		}
		for _, m := range in.ser.Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				m.Plugin = true
			case *syntax.PropDecl:
				// typeSerial0 and on, which metadata doesn't list.
				m.Plugin = strings.HasPrefix(m.Name, "typeSerial")
			}
		}
	}
	serial.Ctor = &syntax.SecondaryCtor{P: cd.P, Serial: serial}
	cd.Members = append(cd.Members, members...)
	cd.Members = append(cd.Members, serial.Ctor)
	cd.Serial = serial
	c.serials = append(c.serials, in)

	if len(in.tparams) > 0 {
		c.genericCompanion(in)
		return
	}
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

// genericCompanion declares the members of a generic class's companion:
// its serializer(typeSerial0, ...), which makes a new serializer each
// time, of the type parameters' serializers, and, for a class with a
// $serializer, $cachedDescriptor, which serialFill gives its elements.
func (c *Checker) genericCompanion(in *serialInfo) {
	var src strings.Builder
	switch in.kind {
	case serialPlain:
		fmt.Fprintf(&src, "%s = %s.`$serializer`(%s)\n", in.serializerHead(), in.fq, typeSerialArgs(len(in.tparams)))
		src.WriteString("private val `$cachedDescriptor`: kotlinx.serialization.descriptors.SerialDescriptor\n" +
			"init { `$cachedDescriptor` = kotlinx.serialization.internal.PluginGeneratedSerialDescriptor(\"\", null, 0) }\n")
	case serialAbstract:
		fmt.Fprintf(&src, "%s = kotlinx.serialization.PolymorphicSerializer(%s::class) as kotlinx.serialization.KSerializer<%[2]s%s>\n", in.serializerHead(), in.fq, in.typeArgs())
	case serialSealed:
		// sealedSerializer writes the subclasses' serializers.
		fmt.Fprintf(&src, "%s { throw java.lang.IllegalStateException() }\n", in.serializerHead())
	}
	members := c.serialCompanion(in.cd, src.String())
	if members == nil {
		return
	}
	f := members[0].(*syntax.FunDecl)
	copyTypeParams(in.cd, f.TypeParams)
	f.Plugin = true
	if in.kind == serialSealed {
		in.serializer = f
	}
	if in.kind == serialPlain {
		members[1].(*syntax.PropDecl).Plugin = true
		in.cached = members[2].(*syntax.InitBlock)
	}
}

// genericSerializerSrc returns the source of the $serializer class of
// generic class fq, whose type parameters are tps, as serializerObjectSrc
// for a class without: it keeps its type parameters' serializers,
// typeSerial0 and on, which its constructor takes.
func genericSerializerSrc(fq string, tps []string, serialize string) string {
	typ := fq + "<" + strings.Join(tps, ", ") + ">"
	var b strings.Builder
	fmt.Fprintf(&b, "@kotlin.Deprecated(message = \"This synthesized declaration should not be used directly\", level = kotlin.DeprecationLevel.HIDDEN)\n"+
		"class `$serializer`<%s>(%s) : kotlinx.serialization.internal.GeneratedSerializer<%s> {\n"+
		"final override val descriptor: kotlinx.serialization.descriptors.SerialDescriptor\n"+
		"init { descriptor = kotlinx.serialization.internal.PluginGeneratedSerialDescriptor(\"\", this, 0) }\n",
		strings.Join(tps, ", "), typeSerialParams(tps), typ)
	for i, tp := range tps {
		fmt.Fprintf(&b, "private val typeSerial%[1]d: kotlinx.serialization.KSerializer<%[2]s> = typeSerial%[1]d\n", i, tp)
	}
	args := typeSerialArgs(len(tps))
	fmt.Fprintf(&b, "final override fun childSerializers(): Array<kotlinx.serialization.KSerializer<*>> = arrayOf<kotlinx.serialization.KSerializer<*>>()\n"+
		"final override fun typeParametersSerializers(): Array<kotlinx.serialization.KSerializer<*>> = arrayOf<kotlinx.serialization.KSerializer<*>>(%[3]s)\n"+
		"final override fun serialize(encoder: kotlinx.serialization.encoding.Encoder, value: %[1]s) {\n%[2]s}\n"+
		"final override fun deserialize(decoder: kotlinx.serialization.encoding.Decoder): %[1]s { throw java.lang.IllegalStateException() }\n"+
		"}\n", typ, serialize, args)
	return b.String()
}

// serializerObjectSrc returns the source of the $serializer object of
// class fq, whose serialize has the statements serialize. serialFill
// writes its other bodies.
func serializerObjectSrc(fq, serialize string) string {
	return fmt.Sprintf("@kotlin.Deprecated(message = \"This synthesized declaration should not be used directly\", level = kotlin.DeprecationLevel.HIDDEN)\n"+
		"object `$serializer` : kotlinx.serialization.internal.GeneratedSerializer<%[1]s> {\n"+
		"final override val descriptor: kotlinx.serialization.descriptors.SerialDescriptor\n"+
		"init { descriptor = kotlinx.serialization.internal.PluginGeneratedSerialDescriptor(\"\", this, 0) }\n"+
		"final override fun childSerializers(): Array<kotlinx.serialization.KSerializer<*>> = arrayOf<kotlinx.serialization.KSerializer<*>>()\n"+
		"final override fun serialize(encoder: kotlinx.serialization.encoding.Encoder, value: %[1]s) {\n%[2]s}\n"+
		"final override fun deserialize(decoder: kotlinx.serialization.encoding.Decoder): %[1]s { throw java.lang.IllegalStateException() }\n"+
		"}\n", fq, serialize)
}

// serialValueClass declares the $serializer of a value class, and its
// companion's serializer(), which returns it. The serializer writes the
// value class's property inline, as the format writes it alone, unless the
// format writes the value class otherwise.
func (c *Checker) serialValueClass(in *serialInfo) {
	cd := in.cd
	if len(cd.CtorParams) != 1 || !cd.CtorParams[0].IsProp {
		return // an error the checker reports
	}
	cp := cd.CtorParams[0]
	a := deniedAnnotation(cp.Modifiers.Annotations)
	if a == nil {
		a = c.findSerial(cp.Modifiers.Annotations, "Contextual")
	}
	if a == nil {
		a = c.findSerial(cp.Modifiers.Annotations, "Polymorphic")
	}
	if a != nil {
		c.unsupported(a.P, "serialization", "@%s is not supported", a.Name)
		return
	}
	members, err := parseMembers(c.file.Path, cd.P, serializerObjectSrc(in.fq, ""))
	if err != nil {
		c.errorf(cd.P, "serialization: %v", err)
		return
	}
	in.ser = members[0].(*syntax.ClassDecl)
	in.ser.Synthetic = true
	cd.Members = append(cd.Members, members...)
	cd.Serial = &syntax.Serial{Name: in.name, Props: []*syntax.SerialProp{{Name: c.serialName(cp.Modifiers.Annotations, cp.Name), Param: cp,
		Optional: cp.Default != nil && c.findSerial(cp.Modifiers.Annotations, "Required") == nil}}}
	c.serials = append(c.serials, in)
	c.serialCompanion(cd, fmt.Sprintf("fun serializer(): kotlinx.serialization.KSerializer<%[1]s> = %[1]s.`$serializer`\n", in.fq))
}

// valueSerializerBodies writes the members of a value class's $serializer
// that depend on the type of its property: they write and read it with
// the encoder and decoder of the value class's descriptor, which the
// format returns.
func (c *Checker) valueSerializerBodies(in *serialInfo) {
	cd := in.cd
	p := cd.Serial.Props[0]
	t := p.Sym().Type
	ser, ok := c.propSerializer(p.Annotations(), p.Param.P, true, t, nil, c.serialFile, p.TypeRef())
	if !ok {
		return
	}
	custom := ser != ""
	if !custom {
		// The serializer of the type as written, which a typealias's
		// @Serializable(with) may name.
		ser = c.serialSerializerAt(t.NonNull(), nil, c.serialFile, p.TypeRef(), nil)
		custom = serialPrimitives[t.K] != "" && ser != "" && ser != c.serialSerializer(t.NonNull())
	}
	if ser == "" {
		c.unsupported(p.Param.P, "serialization", "serialization can't write a property of type %s", t)
		return
	}
	initBody := c.serialParse(cd, fmt.Sprintf("fun f() {\n"+
		"val `$d` = kotlinx.serialization.internal.InlineClassDescriptor(%q, this)\n"+
		"`$d`.addElement(%q, %t)\n"+
		"descriptor = `$d`\n}\n", cd.Serial.Name, p.Name, p.Optional))
	children := c.serialParse(cd, fmt.Sprintf("fun f() = arrayOf<kotlinx.serialization.KSerializer<*>>(%s)\n", serialNullable(t, ser)))
	v := "value.`" + p.Sym().Name + "`"
	typ, _ := kotlinType(t)
	var enc, dec string
	switch m := serialPrimitives[t.K]; {
	case m != "" && !t.N && !custom:
		enc = fmt.Sprintf("encode%s(%s)", m, v)
		dec = fmt.Sprintf("decode%s()", m)
	case t.N:
		enc = fmt.Sprintf("encodeNullableSerializableValue(%s, %s)", ser, v)
		dec = fmt.Sprintf("decodeNullableSerializableValue(%s) as %s", ser, typ)
	default:
		enc = fmt.Sprintf("encodeSerializableValue(%s, %s)", ser, v)
		dec = fmt.Sprintf("decodeSerializableValue(%s) as %s", ser, typ)
	}
	serialize := c.serialParse(cd, "fun f() {\nencoder.encodeInline(descriptor)?."+enc+"\n}\n")
	deserialize := c.serialParse(cd, fmt.Sprintf("fun f() {\nreturn %s(decoder.decodeInline(descriptor).%s)\n}\n", in.fq, dec))
	if initBody == nil || children == nil || serialize == nil || deserialize == nil {
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
			case "serialize":
				m.Body = serialize.Body
			case "deserialize":
				m.Body = deserialize.Body
			}
		}
	}
}

// encodeDefault returns the mode of a property's @EncodeDefault, among
// its annotations as.
func (c *Checker) encodeDefault(as []syntax.Annotation) encodeMode {
	a := c.findSerial(as, "EncodeDefault")
	if a == nil {
		return encodeIfFormat
	}
	if arg := a.Arg(0, "mode"); arg != nil && len(arg.Toks) > 0 && arg.Toks[len(arg.Toks)-1].Text == "NEVER" {
		return encodeNever
	}
	return encodeAlways
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
	comp := &syntax.ClassDecl{P: cd.P, Name: "Companion", Object: true, HasPrimary: true, Members: members, Generated: true}
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
	if len(c.serials) > 0 && !c.serialFileAnnotations() {
		return
	}
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
	case serialValue:
		c.valueSerializerBodies(in)
		return
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
	inherited := len(all) - len(serial.Props)
	scope := typeSerialScope(cls.TypeParams)
	// The properties' types as the class sees them: a generic
	// superclass's with the type arguments the class gives it.
	ts := make([]types.Type, len(all))
	for i, p := range all {
		ts[i] = p.Sym().Type
		if i < inherited {
			ts[i] = c.memberSubst(cls.This(), p.Sym().Owner).Apply(ts[i])
		}
	}
	sers := make([]string, len(all))
	custom := make([]bool, len(all))
	for i, p := range all {
		t := ts[i]
		report := i >= inherited // the superclass's are reported with it
		var pos syntax.Pos
		if p.Param != nil {
			pos = p.Param.P
		} else {
			pos = p.Prop.P
		}
		s, ok := c.propSerializer(p.Annotations(), pos, report, t, scope, c.serialFile, p.TypeRef())
		if !ok {
			return
		}
		if s != "" {
			sers[i], custom[i] = s, true
			continue
		}
		// The serializer of the type as written, whose annotations and
		// the file's may name its own: @Contextual, @Polymorphic.
		if sers[i] = c.serialSerializerAt(t.NonNull(), scope, c.serialFile, p.TypeRef(), p.Annotations()); sers[i] != "" {
			// A primitive's own serializer is called, not the encoder's
			// method for the primitive.
			custom[i] = serialPrimitives[t.K] != "" && sers[i] != c.serialSerializerIn(t.NonNull(), scope)
			continue
		}
		if report {
			c.unsupported(pos, "serialization", "serialization can't write a property of type %s", t)
		} else {
			fail("serialization can't write the superclass's property %s of type %s", p.Sym().Name, t)
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
	for i := range serialMasks(len(all)) {
		param(fmt.Sprintf("seen%d", i), types.IntType)
	}
	for i, p := range all {
		t := ts[i]
		if !t.IsPrimitive() {
			t = t.Nullable() // null where it wasn't read
		}
		param(p.Sym().Name, t)
	}
	param("serializationConstructorMarker", types.ClassType(marker).Nullable())
	sc.Sym = f
	cls.Ctors = append(cls.Ctors, f)

	// write$Self writes the superclass's properties, then its own: those
	// with default values only if they differ, or if the format asks to,
	// unless their @EncodeDefault says always or never.
	var b strings.Builder
	if sd := serial.Super; sd != nil {
		// A generic superclass's takes the serializers of the type
		// arguments the class gives it.
		st, _ := c.asSuper(cls.This(), sd.Sym)
		typ, ok := kotlinTypeOf(st, true)
		var args strings.Builder
		if st.A != nil {
			for _, a := range st.A.List {
				s := ""
				if a.V != types.Star {
					s = c.serialSerializerIn(a.T.NonNull(), scope)
				}
				if s == "" {
					ok = false
					break
				}
				if a.T.K == types.Null {
					// Nothing?, as kotlinc's plugin passes it: the
					// NothingSerializer, not nullable, for a
					// KSerializer<Nothing?>.
					s = "(" + s + " as kotlinx.serialization.KSerializer<kotlin.Nothing?>)"
				} else {
					s = serialNullable(a.T, s)
				}
				args.WriteString(", " + s)
			}
		}
		if !ok {
			fail("serialization can't write the type arguments of the superclass %s", st)
			return
		}
		fmt.Fprintf(&b, "(this as %s).`write$Self`(output, serialDesc%s)\n", typ, args.String())
	}
	repl := map[string]syntax.Expr{}
	for i, p := range serial.Props {
		j := inherited + i
		v := "this.`" + p.Sym().Name + "`"
		enc := serialEncode(ts[j], j, sers[j], v, custom[j])
		if in.defaults[i] == nil {
			b.WriteString(enc)
			continue
		}
		def := fmt.Sprintf("$default%d", i)
		repl[def] = in.defaults[i]
		switch in.encode[i] {
		case encodeAlways:
			b.WriteString(enc)
		case encodeNever:
			fmt.Fprintf(&b, "if (%s != `%s`) {\n%s}\n", v, def, enc)
		default:
			fmt.Fprintf(&b, "if (output.shouldEncodeElementDefault(serialDesc, %d) || %s != `%s`) {\n%s}\n", j, v, def, enc)
		}
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
	if in.cached != nil {
		// As the $serializer's descriptor, without the serializer, which
		// tells the type arguments' serializers.
		if f := c.serialParse(cd, "fun f() {\n"+serialDescriptorSrc(cd.Serial.Name, "null", all)+"`$cachedDescriptor` = `$d`\n}\n"); f != nil {
			in.cached.Body = f.Body
		}
	}
	c.serializerBodies(in, all, ts, sers, custom)
}

// serialDescriptorSrc returns the statements that make the descriptor
// `$d` of a class named name whose serializer is ser and whose elements
// are props.
func serialDescriptorSrc(name, ser string, props []*syntax.SerialProp) string {
	var b strings.Builder
	fmt.Fprintf(&b, "val `$d` = kotlinx.serialization.internal.PluginGeneratedSerialDescriptor(%q, %s, %d)\n", name, ser, len(props))
	for _, p := range props {
		fmt.Fprintf(&b, "`$d`.addElement(%q, %t)\n", p.Name, p.Optional)
	}
	return b.String()
}

// serialScope maps the type parameters of a generic class to the
// serializers of their types: those its serializer was made with.
type serialScope map[*types.TypeParamSym]string

// typeSerialScope returns the scope of a generic class's type parameters,
// tps, in its serializer, its write$Self and its companion's
// serializer(): typeSerial0 and on.
func typeSerialScope(tps []*types.TypeParamSym) serialScope {
	if len(tps) == 0 {
		return nil
	}
	s := serialScope{}
	for i, tp := range tps {
		s[tp] = fmt.Sprintf("typeSerial%d", i)
	}
	return s
}

// of returns the serializer of type parameter p's values, or "".
func (s serialScope) of(p *types.TypeParamSym) string {
	for ; p != nil; p = p.Origin {
		if v, ok := s[p]; ok {
			return v
		}
		if p.Origin == p {
			break
		}
	}
	return ""
}

// serializerBodies writes the members of a class's $serializer that
// depend on its properties, all, of types ts, whose serializers are sers,
// custom where the property names its own.
func (c *Checker) serializerBodies(in *serialInfo, all []*syntax.SerialProp, ts []types.Type, sers []string, custom []bool) {
	cd := in.cd
	// The descriptor names the properties.
	var b strings.Builder
	b.WriteString(serialDescriptorSrc(cd.Serial.Name, "this", all))
	b.WriteString("descriptor = `$d`\n")
	initBody := c.serialParse(cd, "fun f() {\n"+b.String()+"}\n")

	// childSerializers.
	b.Reset()
	b.WriteString("fun f() = arrayOf<kotlinx.serialization.KSerializer<*>>(")
	for i := range all {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(serialNullable(ts[i], sers[i]))
	}
	b.WriteString(")\n")
	children := c.serialParse(cd, b.String())

	// deserialize reads the properties in order, if the format can, or
	// else in the order the format finds them.
	b.Reset()
	b.WriteString("fun f() {\n" +
		"val `$d` = descriptor\n" +
		"var `$flag` = true\n" +
		"var `$index`: kotlin.Int\n")
	for i := range serialMasks(len(all)) {
		fmt.Fprintf(&b, "var `$seen%d` = 0\n", i)
	}
	for i := range all {
		t := ts[i]
		typ, _ := kotlinTypeOf(t, true)
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
		case types.Unit:
			zero = "kotlin.Unit" // which has no nullable type in kc
		}
		if !t.IsPrimitive() {
			typ, _ = kotlinTypeOf(t.Nullable(), true)
		}
		fmt.Fprintf(&b, "var `$v%d`: %s = %s\n", i, typ, zero)
	}
	b.WriteString("val `$c` = decoder.beginStructure(`$d`)\n" +
		"if (`$c`.decodeSequentially()) {\n")
	for i := range all {
		b.WriteString(serialDecode(ts[i], i, sers[i], custom[i]))
	}
	b.WriteString("} else {\n" +
		"while (`$flag`) {\n" +
		"`$index` = `$c`.decodeElementIndex(`$d`)\n" +
		"when (`$index`) {\n" +
		"-1 -> `$flag` = false\n")
	for i := range all {
		fmt.Fprintf(&b, "%d -> {\n%s}\n", i, serialDecode(ts[i], i, sers[i], custom[i]))
	}
	b.WriteString("else -> throw kotlinx.serialization.UnknownFieldException(`$index`)\n" +
		"}\n" +
		"}\n" +
		"}\n" +
		"`$c`.endStructure(`$d`)\n")
	fmt.Fprintf(&b, "return %s(", in.fq)
	for i := range serialMasks(len(all)) {
		fmt.Fprintf(&b, "`$seen%d`, ", i)
	}
	for i := range all {
		fmt.Fprintf(&b, "`$v%d`, ", i)
	}
	b.WriteString("null)\n}\n")
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
// inherit from it, directly or through sealed classes and interfaces, in
// the order of their names. As the plugin's, it leaves out those it finds
// no serializer for, as a class that isn't @Serializable, which it then
// fails to write at run time.
func (c *Checker) sealedSerializer(in *serialInfo) {
	if in.lazy == nil && in.serializer == nil {
		return
	}
	var classes, sers []string
	// A generic sealed class's serializer takes its type arguments', which
	// its subclasses' type parameters may stand for.
	scope := typeSerialScope(in.cd.Sym.TypeParams)
	var visit func(cls *types.ClassSym)
	visit = func(cls *types.ClassSym) {
		subs := slices.Clone(cls.Subclasses)
		slices.SortFunc(subs, func(a, b *types.ClassSym) int { return strings.Compare(fqName(a), fqName(b)) })
		for _, sub := range subs {
			sub.Ensure()
			if sub.Sealed {
				visit(sub)
				continue
			}
			var ser string
			if sd, ok := sub.Decl.(*syntax.ClassDecl); ok && sd.Serial != nil && sd.Abstract {
				ser = "kotlinx.serialization.PolymorphicSerializer(" + fqName(sub) + "::class)"
			} else if len(sub.TypeParams) > 0 {
				if ser = c.genericSubSerializer(sub, in.cd.Sym, scope); ser == "" {
					continue
				}
			} else if ser = c.serialSerializer(sub.This()); ser == "" {
				continue
			}
			classes = append(classes, fqName(sub)+"::class")
			sers = append(sers, ser)
		}
	}
	visit(in.cd.Sym)
	star := in.starType()
	x := fmt.Sprintf("kotlinx.serialization.SealedClassSerializer(%q, %s::class, arrayOf<kotlin.reflect.KClass<out %[3]s>>(%[4]s), arrayOf<kotlinx.serialization.KSerializer<out %[3]s>>(%[5]s))",
		in.name, in.fq, star, strings.Join(classes, ", "), strings.Join(sers, ", "))
	if len(in.tparams) > 0 {
		x += fmt.Sprintf(" as kotlinx.serialization.KSerializer<%s%s>", in.fq, in.typeArgs())
	}
	if in.serializer != nil {
		if f := c.serialParse(in.cd, fmt.Sprintf("fun f() = %s\n", x)); f != nil {
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

// genericSubSerializer returns the serializer of generic subclass sub of
// sealed class base, as the plugin writes it in base's serializer: of each
// of sub's type parameters that base's type argument is, the serializer
// of that argument, in base's scope, and of others a PolymorphicSerializer
// of Any. It returns "" if kc knows none.
func (c *Checker) genericSubSerializer(sub, base *types.ClassSym, scope serialScope) string {
	st, _ := c.asSuper(sub.This(), base)
	args := make([]string, len(sub.TypeParams))
	for i, tp := range sub.TypeParams {
		args[i] = "kotlinx.serialization.PolymorphicSerializer(Any::class)"
		if st.A == nil {
			continue
		}
		for j, a := range st.A.List {
			if a.V != types.Star && a.T.K == types.TypeParam && !a.T.N && a.T.Param() == tp && j < len(base.TypeParams) {
				if s := scope.of(base.TypeParams[j]); s != "" {
					args[i] = s
				}
				break
			}
		}
	}
	if sd, ok := sub.Decl.(*syntax.ClassDecl); ok && sd.Serial != nil && sub.Nested["$serializer"] != nil {
		return fqName(sub) + ".`$serializer`(" + strings.Join(args, ", ") + ")"
	}
	if c.hasGenericSerializerFun(sub) {
		return fqName(sub) + ".serializer(" + strings.Join(args, ", ") + ")"
	}
	return ""
}

// serialPrimitives are the types whose values the plugin writes and reads
// with encoders' and decoders' own methods, encodeIntElement and others,
// by those methods' names.
var serialPrimitives = map[types.Kind]string{
	types.Int: "Int", types.Long: "Long", types.Short: "Short", types.Byte: "Byte", types.Char: "Char",
	types.Float: "Float", types.Double: "Double", types.Bool: "Boolean", types.String: "String",
}

// serialEncode returns the statement of write$Self that writes element i,
// v, of type t, whose serializer is ser, custom if the property names it.
func serialEncode(t types.Type, i int, ser, v string, custom bool) string {
	if m := serialPrimitives[t.K]; m != "" && !t.N && !custom {
		return fmt.Sprintf("output.encode%sElement(serialDesc, %d, %s)\n", m, i, v)
	}
	if t.N {
		return fmt.Sprintf("output.encodeNullableSerializableElement(serialDesc, %d, %s, %s)\n", i, ser, v)
	}
	return fmt.Sprintf("output.encodeSerializableElement(serialDesc, %d, %s, %s)\n", i, ser, v)
}

// serialDecode returns the statements of deserialize that read element i,
// of type t, whose serializer is ser, custom if the property names it,
// into its local. A collection's serializer reads a List, Set or Map: the
// value is cast to t, as kotlinc's code does.
func serialDecode(t types.Type, i int, ser string, custom bool) string {
	var x string
	typ, _ := kotlinTypeOf(t, true)
	switch m := serialPrimitives[t.K]; {
	case m != "" && !t.N && !custom:
		x = fmt.Sprintf("`$c`.decode%sElement(`$d`, %d)", m, i)
	case t.N:
		x = fmt.Sprintf("(`$c`.decodeNullableSerializableElement(`$d`, %d, %s, `$v%d`) as %s)", i, ser, i, typ)
	default:
		x = fmt.Sprintf("(`$c`.decodeSerializableElement(`$d`, %d, %s, `$v%d`) as %s)", i, ser, i, typ)
	}
	bit := strconv.Itoa(1 << (i % 32))
	if i%32 == 31 {
		bit = "(1 shl 31)" // not an Int literal
	}
	return fmt.Sprintf("`$v%d` = %s\n`$seen%d` = `$seen%[3]d` or %s\n", i, x, i/32, bit)
}

// serialMasks returns the number of Int masks, seen0 and on, in which the
// synthetic constructor of a class with n serializable properties, and its
// deserialize, keep a bit for each property that was read: one for each 32
// properties, and at least one.
func serialMasks(n int) int {
	return max(1, (n+31)/32)
}

// propSerializer returns the Kotlin of the serializer that a property's
// @Serializable(with = S::class), among its annotations as, names, for
// the property's type t, written as ref (or nil): withSerializerOf's. It
// returns "" if the property names none, and false, after it reports it if
// report is set, if kc can't make S.
func (c *Checker) propSerializer(as []syntax.Annotation, pos syntax.Pos, report bool, t types.Type, scope serialScope, ctx *serialFileContext, ref *syntax.TypeRef) (string, bool) {
	toks := withSerializer(as)
	if toks == nil {
		return "", true
	}
	cls := c.withClass(toks, pos)
	if cls == nil {
		if report {
			c.errorf(pos, "unresolved reference '%s'", tokensText(toks))
		}
		return "", false
	}
	if ref != nil {
		ref = c.serialTypeOf(ref, 0).ref
	}
	s := c.withSerializerOf(cls, t.NonNull(), scope, ctx, ref)
	if s == "" {
		if report {
			c.unsupported(pos, "serialization", "generic serializers of properties are not supported")
		}
		return "", false
	}
	return s, true
}

// tokensText returns the source of toks, a qualified name.
func tokensText(toks []syntax.Token) string {
	var name strings.Builder
	for _, t := range toks {
		name.WriteString(t.Text)
	}
	return name.String()
}

// withClass returns the class that toks, the name S of
// @Serializable(with = S::class), names in the file, or nil.
func (c *Checker) withClass(toks []syntax.Token, pos syntax.Pos) *types.ClassSym {
	cls := c.classRef(qualifiedExpr(tokensText(toks), pos))
	if cls != nil {
		cls.Ensure()
	}
	return cls
}

// withSerializerOf returns the Kotlin of serializer class cls, which
// @Serializable(with = cls::class) names for the values of type t, which
// isn't nullable, written as ref (or nil): cls, an object, or else a new
// cls. As with the plugin, the constructor of a generic cls takes the
// serializers of t's type arguments: BoxSer(String.serializer()) of
// Box<String>. It returns "" if kc can't make it.
func (c *Checker) withSerializerOf(cls *types.ClassSym, t types.Type, scope serialScope, ctx *serialFileContext, ref *syntax.TypeRef) string {
	cls.Ensure()
	switch {
	case cls.Object:
		return fqName(cls)
	case len(cls.TypeParams) == 0:
		return fqName(cls) + "()"
	case t.K != types.Class || t.A == nil:
		return ""
	}
	args := make([]string, t.A.Len())
	for i, a := range t.A.List {
		if a.V == types.Star {
			return ""
		}
		s := c.serialSerializerAt(a.T.NonNull(), scope, ctx, refArg(ref, t, i), nil)
		if s == "" {
			return ""
		}
		args[i] = serialNullable(a.T, s)
	}
	takes := cls.Ctor != nil && len(cls.Ctor.Params) == len(args)
	for _, f := range cls.Ctors {
		takes = takes || len(f.Params) == len(args)
	}
	if !takes {
		return ""
	}
	return fqName(cls) + "(" + strings.Join(args, ", ") + ")"
}

// serialNullable returns the serializer of a nullable type t, from ser,
// its non-null type's.
func serialNullable(t types.Type, ser string) string {
	switch {
	case !t.N || t.K == types.Nothing:
		return ser
	case t.K == types.TypeParam:
		// A type argument's serializer may be nullable already: the
		// plugin's code takes it as it is then, as nullable does.
		return "(" + ser + ").`kotlinx.serialization.builtins.nullable`"
	}
	return "kotlinx.serialization.internal.NullableSerializer(" + ser + ")"
}

// serializerOf sets the serializer that call x of f is, if f is
// kotlinx.serialization's serializer<T>() and kc knows the serializer of
// T: as with the plugin, which replaces the call, it needs no lookup when
// the program runs, which a Java class's @Serializable(with) needs.
func (c *Checker) serializerOf(x *syntax.Call, f *types.FunSym) {
	if f.Name != "serializer" || f.Ext || f.Owner != nil || len(f.Params) != 0 || len(x.Reified) != 1 ||
		f.Facade != "kotlinx/serialization/SerializersKt" {
		return
	}
	x.Value = c.knownSerializer(x.P, x.Reified[0])
}

// reifiedSerializers sets x.Serializers, if f is an inline function,
// whose serializer<T>() calls kc replaces when it copies them: those of
// a library's bytecode, and those of the program's function, also in the
// inline functions it passes T on to.
func (c *Checker) reifiedSerializers(x *syntax.Call, f *types.FunSym) {
	if !c.u.serialization || x.Value != nil {
		return
	}
	for i, tp := range f.TypeParams {
		if !tp.Reified || i >= len(x.Reified) {
			continue
		}
		if v := c.knownSerializer(x.P, x.Reified[i]); v != nil {
			if x.Serializers == nil {
				x.Serializers = map[string]syntax.Expr{}
			}
			x.Serializers[tp.Name] = v
		}
	}
}

// knownSerializer returns the checked expression of the serializer of
// type t that the plugin finds, or nil.
func (c *Checker) knownSerializer(p syntax.Pos, t types.Type) syntax.Expr {
	if t.HasParams() || t.HasVars() {
		return nil
	}
	s := c.serialSerializer(t.NonNull())
	if s == "" {
		return nil
	}
	members, err := parseMembers(c.file.Path, p, "val x = "+serialNullable(t, s)+"\n")
	if err != nil {
		return nil
	}
	v := members[0].(*syntax.PropDecl).Init
	n := len(c.errs)
	if c.expr(v, noWant); len(c.errs) > n {
		c.errs = c.errs[:n]
		return nil
	}
	return v
}

// serialSerializer returns the Kotlin of a serializer of the values of
// type t, which isn't nullable, as the plugin finds it, or "" if kc
// doesn't know it.
func (c *Checker) serialSerializer(t types.Type) string {
	return c.serialSerializerIn(t, nil)
}

// serialSerializerIn is serialSerializer in the code of a generic class's
// serializer, where scope gives the serializers of its type parameters.
func (c *Checker) serialSerializerIn(t types.Type, scope serialScope) string {
	return c.serialSerializerAt(t, scope, nil, nil, nil)
}

// serialFileContext is what a file's annotations tell the plugin about the
// serializers of its classes' properties.
type serialFileContext struct {
	using      map[*types.ClassSym]string // @UseSerializers: a class's serializer
	contextual map[*types.ClassSym]bool   // @UseContextualSerialization
}

// serialFileAnnotations reads the file's @UseSerializers and
// @UseContextualSerialization into c.serialFile. It reports false, after
// reporting it, for one kc can't read.
func (c *Checker) serialFileAnnotations() bool {
	c.serialFile = nil
	for _, a := range c.file.Annotations {
		using := c.serialAnno(a, "UseSerializers")
		if !using && !c.serialAnno(a, "UseContextualSerialization") {
			continue
		}
		if c.serialFile == nil {
			c.serialFile = &serialFileContext{using: map[*types.ClassSym]string{}, contextual: map[*types.ClassSym]bool{}}
		}
		for _, arg := range a.Args {
			cls := c.classLitArg(arg, a.P)
			if cls == nil {
				c.unsupported(a.P, "serialization", "@%s's arguments must be class literals", a.Name)
				return false
			}
			if !using {
				c.serialFile.contextual[cls] = true
				continue
			}
			// The class a serializer serializes: T of its KSerializer<T>.
			ks := c.u.ext.class("kotlinx/serialization/KSerializer")
			st, ok := types.Type{}, false
			if ks != nil {
				st, ok = c.asSuper(cls.This(), ks)
			}
			if !ok || len(cls.TypeParams) > 0 || st.A == nil || st.A.Len() != 1 || st.A.Get(0).T.NonNull().K != types.Class {
				c.unsupported(a.P, "serialization", "@%s of %s is not supported", a.Name, fqName(cls))
				return false
			}
			ser := fqName(cls)
			if !cls.Object {
				ser += "()"
			}
			c.serialFile.using[st.A.Get(0).T.C] = ser
		}
	}
	return true
}

// classLitArg returns the class that an annotation's argument, X::class,
// names, or nil.
func (c *Checker) classLitArg(arg syntax.AnnotationArg, p syntax.Pos) *types.ClassSym {
	toks := arg.Toks
	n := len(toks)
	if n < 3 || toks[n-2].Kind != syntax.COLON_COLON || toks[n-1].Text != "class" {
		return nil
	}
	var name strings.Builder
	for _, t := range toks[:n-2] {
		if t.Kind != syntax.IDENT && t.Kind != syntax.DOT {
			return nil
		}
		name.WriteString(t.Text)
	}
	cls := c.classRef(qualifiedExpr(name.String(), p))
	if cls != nil {
		cls.Ensure()
	}
	return cls
}

// serialSerializerAt is serialSerializerIn of a type written as ref (or
// nil), with annotations as, in a file whose annotations are ctx (or nil):
// @Contextual and @Polymorphic, on the property or the type, name the
// serializer, as do the file's @UseSerializers and
// @UseContextualSerialization of a class.
func (c *Checker) serialSerializerAt(t types.Type, scope serialScope, ctx *serialFileContext, ref *syntax.TypeRef, as []syntax.Annotation) string {
	var ta serialTypeAnnotations
	if ref != nil && t.K != types.TypeParam {
		ta = c.serialTypeOf(ref, 0)
		ref = ta.ref
	}
	switch {
	case t.K == types.TypeParam:
	case ta.contextual || c.findSerialType(as, "Contextual"):
		return c.contextualSerializer(t, scope, ctx, ref)
	case ta.polymorphic || c.findSerialType(as, "Polymorphic"):
		if lit := serialClassLiteral(t, false); lit != "" {
			return "kotlinx.serialization.PolymorphicSerializer(" + lit + ")"
		}
		return ""
	case ta.withBad:
		return ""
	case ta.with != nil:
		return c.withSerializerOf(ta.with, t, scope, ctx, ref)
	case ctx != nil && t.K == types.Class:
		if s := ctx.using[t.C]; s != "" {
			return s
		}
		if ctx.contextual[t.C] {
			return c.contextualSerializer(t, scope, ctx, ref)
		}
	}
	return c.serialSerializerOf(t, scope, ctx, ref)
}

// serialTypeAnnotations are what the annotations of a type as written say
// of its serializer, with those of the typealiases it names, as the plugin
// sees them on the type they expand to.
type serialTypeAnnotations struct {
	contextual, polymorphic bool
	with                    *types.ClassSym // S of @Serializable(with = S::class)
	withBad                 bool            // set if kc can't find S
	// ref is the type as written, through the typealiases it names: its
	// type arguments are those of the type it expands to.
	ref *syntax.TypeRef
}

// serialTypeOf returns the serialTypeAnnotations of ref, a type as
// written in the file: those of typealias SBox<T> = @Serializable(with =
// BoxSer::class) Box<T> for SBox<String>, whose ref is then Box<String>.
// A typealias is read in the file that declares it.
func (c *Checker) serialTypeOf(ref *syntax.TypeRef, depth int) serialTypeAnnotations {
	r := serialTypeAnnotations{ref: ref}
	for _, a := range ref.Annotations {
		if a.Target != "" {
			continue
		}
		switch {
		case c.serialAnno(a, "Contextual"):
			r.contextual = true
		case c.serialAnno(a, "Polymorphic"):
			r.polymorphic = true
		case r.with == nil && !r.withBad && c.serialAnno(a, "Serializable"):
			if toks := withSerializer([]syntax.Annotation{a}); toks != nil {
				r.with = c.withClass(toks, a.P)
				r.withBad = r.with == nil
			}
		}
	}
	if ref.Ret != nil || ref.Name == "" || depth > 16 {
		return r
	}
	al := c.lookupAlias(ref.Name)
	if al == nil {
		return r
	}
	d, ok := al.Decl.(*syntax.TypeAliasDecl)
	if !ok || d.Type == nil || len(ref.Args) != len(d.TypeParams) {
		return r
	}
	ac := c.u.aliasFile[al]
	if ac == nil {
		ac = c
	}
	args := map[string]*syntax.TypeRef{}
	for i, tp := range d.TypeParams {
		args[tp.Name] = ref.Args[i].Type
	}
	in := ac.serialTypeOf(substTypeRef(d.Type, args), depth+1)
	r.contextual = r.contextual || in.contextual
	r.polymorphic = r.polymorphic || in.polymorphic
	if r.with == nil && !r.withBad {
		r.with, r.withBad = in.with, in.withBad
	}
	r.ref = in.ref
	return r
}

// substTypeRef returns ref, the right side of a typealias, with the uses
// of its type parameters replaced with args, their arguments as written,
// with the annotations of both: nil for a star projection.
func substTypeRef(ref *syntax.TypeRef, args map[string]*syntax.TypeRef) *syntax.TypeRef {
	if ref == nil {
		return nil
	}
	if a, ok := args[ref.Name]; ok && ref.Args == nil && ref.Ret == nil && ref.Outer == nil {
		if a == nil {
			return nil
		}
		cp := *a
		cp.Annotations = append(cp.Annotations[:len(cp.Annotations):len(cp.Annotations)], ref.Annotations...)
		return &cp
	}
	if ref.Args == nil {
		return ref
	}
	cp := *ref
	cp.Args = make([]*syntax.TypeArg, len(ref.Args))
	for i, a := range ref.Args {
		na := *a
		na.Type = substTypeRef(a.Type, args)
		if a.Type != nil && na.Type == nil {
			na.Variance = types.Star
		}
		cp.Args[i] = &na
	}
	return &cp
}

// findSerialType reports whether as, the annotations of a property or of
// a type, include kotlinx.serialization's annotation name.
func (c *Checker) findSerialType(as []syntax.Annotation, name string) bool {
	for i := range as {
		if (as[i].Target == "" || as[i].Target == "property" || as[i].Target == "param") && c.serialAnno(as[i], name) {
			return true
		}
	}
	return false
}

// contextualSerializer returns the ContextualSerializer of type t, which
// the format's serializers module gives at run time: with the class's own
// serializer, if it has one, to fall back on, and those of its type
// arguments. It returns "" for a type without a class.
func (c *Checker) contextualSerializer(t types.Type, scope serialScope, ctx *serialFileContext, ref *syntax.TypeRef) string {
	lit := serialClassLiteral(t, false)
	if lit == "" {
		return ""
	}
	fallback := c.serialSerializerOf(t, scope, ctx, ref)
	if fallback == "" {
		fallback = "null"
	}
	var args []string
	if t.K == types.Class && t.A != nil && !t.C.Array {
		for i, a := range t.A.List {
			if a.V == types.Star {
				return ""
			}
			s := c.serialSerializerAt(a.T.NonNull(), scope, ctx, refArg(ref, t, i), nil)
			if s == "" {
				return ""
			}
			args = append(args, serialNullable(a.T, s))
		}
	}
	return fmt.Sprintf("kotlinx.serialization.ContextualSerializer(%s, %s, arrayOf<kotlinx.serialization.KSerializer<*>>(%s))",
		lit, fallback, strings.Join(args, ", "))
}

// refArg returns the i-th type argument as written in ref, a type t, or
// nil if ref doesn't write t's arguments.
func refArg(ref *syntax.TypeRef, t types.Type, i int) *syntax.TypeRef {
	if ref == nil || t.A == nil || len(ref.Args) != t.A.Len() || i >= len(ref.Args) {
		return nil
	}
	return ref.Args[i].Type
}

// serialSerializerOf is serialSerializerAt without the annotations of the
// type itself and the file's of its class: the type's own serializer.
func (c *Checker) serialSerializerOf(t types.Type, scope serialScope, ctx *serialFileContext, ref *syntax.TypeRef) string {
	switch t.K {
	case types.TypeParam:
		return scope.of(t.Param())
	case types.Nothing, types.Null:
		return "kotlinx.serialization.internal.NothingSerializer"
	}
	if t.K == types.Bool {
		return "kotlinx.serialization.internal.BooleanSerializer"
	}
	if m := serialPrimitives[t.K]; m != "" {
		return "kotlinx.serialization.internal." + m + "Serializer"
	}
	if t.K == types.Unit {
		return c.serialRuntime("UnitSerializer")
	}
	if t.K != types.Class {
		return ""
	}
	cls := t.C
	cls.Ensure()
	if t.IsFunc() {
		return ""
	}
	if cls.Value {
		switch cls.Name {
		case "kotlin/UInt", "kotlin/ULong", "kotlin/UShort", "kotlin/UByte":
			return "kotlinx.serialization.internal." + cls.Name[len("kotlin/"):] + "Serializer"
		case "kotlin/UIntArray", "kotlin/ULongArray", "kotlin/UShortArray", "kotlin/UByteArray",
			"kotlin/time/Duration":
			// The runtime's serializers of the standard library's value
			// classes, where its version has them.
			return c.serialRuntime(cls.Name[strings.LastIndexByte(cls.Name, '/')+1:] + "Serializer")
		}
		if s := c.serializerObject(cls); s != nil {
			return fqName(s.Outer) + ".`$serializer`"
		}
		if c.hasSerializerFun(cls) {
			return fqName(cls) + ".serializer()"
		}
		return ""
	}
	// The serializers of the type arguments.
	arg := func(i int) string {
		if t.A == nil || i >= t.A.Len() || t.A.Get(i).V == types.Star {
			return ""
		}
		at := t.A.Get(i).T
		s := c.serialSerializerAt(at.NonNull(), scope, ctx, refArg(ref, t, i), nil)
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
	case "kotlin/time/Instant", "kotlin/uuid/Uuid":
		return c.serialRuntime(name[strings.LastIndexByte(name, '/')+1:] + "Serializer")
	}
	if cls.Array {
		if cls != types.ArrayClass {
			// IntArray and the other primitive arrays.
			m := serialPrimitives[cls.Elem.K]
			if m == "" || m == "String" {
				return ""
			}
			return c.serialRuntime(m + "ArraySerializer")
		}
		// Array<T>, of the elements' class and serializer.
		if t.A == nil || t.A.Get(0).V == types.Star || t.A.Get(0).V == types.In {
			return ""
		}
		elem := t.A.Get(0).T
		lit := serialClassLiteral(elem, true)
		s := arg(0)
		if lit == "" || s == "" {
			return ""
		}
		return "kotlinx.serialization.internal.ReferenceArraySerializer(" + lit + ", " + s + ")"
	}
	if len(cls.TypeParams) > 0 && c.hasGenericSerializerFun(cls) {
		// A generic class's companion's serializer() takes its type
		// arguments' serializers.
		args := make([]string, len(cls.TypeParams))
		for i := range args {
			if args[i] = arg(i); args[i] == "" {
				return ""
			}
		}
		return fqName(cls) + ".serializer(" + strings.Join(args, ", ") + ")"
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
	if s := cls.SerialWith; s != nil {
		s.Ensure()
		if s.Object {
			return fqName(s)
		}
		return fqName(s) + "()"
	}
	if cls.Entries != nil {
		return fmt.Sprintf("kotlinx.serialization.internal.createSimpleEnumSerializer(%q, %s.values())", fqName(cls), fqName(cls))
	}
	if cls.Interface && !cls.Sealed && !strings.HasPrefix(name, "kotlin/") && !strings.HasPrefix(name, "java/") {
		return "kotlinx.serialization.PolymorphicSerializer(" + fqName(cls) + "::class)"
	}
	return ""
}

// serialRuntime returns the qualified name of the serializer object name
// of kotlinx.serialization's runtime, kotlinx.serialization.internal.name,
// or "" if the runtime on the class path doesn't have it.
func (c *Checker) serialRuntime(name string) string {
	if c.u.ext.class("kotlinx/serialization/internal/"+name) == nil {
		return ""
	}
	return "kotlinx.serialization.internal." + name
}

// serialClassLiteral returns the class literal of type t's class, X::class,
// or "" for a type without one. For a primitive type, it is its box's if
// boxed is set, as the plugin passes a ReferenceArraySerializer the class
// of the array's elements.
func serialClassLiteral(t types.Type, boxed bool) string {
	t = t.NonNull()
	if t.K == types.Class {
		if t.IsFunc() {
			return ""
		}
		return fqName(t.C) + "::class"
	}
	if b := serialBoxes[t.K]; b != "" && boxed {
		// The boxed class, of which the runtime makes the array:
		// Integer[] for an Array<Int>, as kotlinc's plugin passes it.
		return "java.lang." + b + "::class"
	}
	if t.K == types.TypeParam || t.K == types.Null || t.K == types.Nothing {
		return ""
	}
	s, ok := kotlinTypeOf(t, false)
	if !ok {
		return ""
	}
	return s + "::class"
}

// serialBoxes are the Java classes of the primitive types' boxes.
var serialBoxes = map[types.Kind]string{
	types.Int: "Integer", types.Long: "Long", types.Short: "Short", types.Byte: "Byte", types.Char: "Character",
	types.Float: "Float", types.Double: "Double", types.Bool: "Boolean",
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

// hasGenericSerializerFun reports whether generic class cls's companion
// object has the serializer() that the plugin gives a generic class,
// which takes a serializer for each of its type parameters.
func (c *Checker) hasGenericSerializerFun(cls *types.ClassSym) bool {
	comp := cls.Companion
	if comp == nil {
		return false
	}
	comp.Ensure()
	n := len(cls.TypeParams)
	for _, f := range comp.Methods["serializer"] {
		if len(f.Params) == n && len(f.TypeParams) == n && !f.Ext {
			return true
		}
	}
	return false
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
