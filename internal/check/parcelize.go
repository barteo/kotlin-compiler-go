package check

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// kotlinc's parcelize compiler plugin (-parcelize).
//
// A class annotated @kotlinx.parcelize.Parcelize, which implements
// android.os.Parcelable, gets from the plugin what the interface asks of
// it, over its primary constructor's properties:
//
//	override fun describeContents(): Int = 0
//	override fun writeToParcel(dest: Parcel, flags: Int) { ... }
//
// a class C$Creator, which creates its instances from a Parcel:
//
//	class Creator : Parcelable.Creator<C> {
//		override fun createFromParcel(parcel: Parcel): C = C(...)
//		override fun newArray(size: Int): Array<C?> = arrayOfNulls(size)
//	}
//
// and public static final Parcelable.Creator CREATOR = new C$Creator(),
// which Android finds by reflection. kc declares the members, and the
// class Creator, as source before the file is checked, and writes their
// bodies once the properties' types are known (parcelBody), as Kotlin that
// writes and reads each property as kotlinc's plugin does, in the same
// encoding. Codegen adds CREATOR (ClassDecl.Parcelize).
//
// An enum class's are its name, which createFromParcel looks up with
// valueOf. A sealed class or interface annotated @Parcelize gets nothing,
// but the plugin makes its direct subclasses Parcelable as if they were
// annotated too (parcelizeSealed). describeContents returns 1 if a
// property holds a java.io.FileDescriptor (parcelDescribe).
//
// How each property is written is the serializer that kotlinc's
// IrParcelSerializerFactory picks for its type (serializer): by the
// exact class for String, CharSequence, Bundle, the primitives, their
// arrays, Android's Size and sparse arrays, and the collections it
// knows; then Parcelables, binders, objects, enums, functions and
// Serializables; and else writeValue. kotlinc reports an error for a
// property written with writeValue unless its type, or one that it is an
// argument of, is annotated @RawValue, so kc leaves such a class to it.
// A generic class's properties are written as their types erased, each
// type parameter as its bound (parcelErase), and its Creator is of the
// class star-projected, C<*>.
//
// Value classes, the types that the plugin writes in ways kc doesn't
// (ranges, Duration, Uuid, the unsigned integers and kotlinx's immutable
// collections) and type parameters with several bounds, or a bound that
// names a type parameter, are unsupported, as are custom Parcelers,
// @TypeParceler and @WriteWith.

// parcelize adds the members of the file's classes annotated @Parcelize,
// if kc compiles with the plugin.
func (c *Checker) parcelize() {
	if !c.u.parcelize {
		return
	}
	prefix := ""
	if c.file.Package != "" {
		prefix = c.file.Package + "."
	}
	var visit func(decls []syntax.Decl, outer string)
	visit = func(decls []syntax.Decl, outer string) {
		for _, d := range decls {
			cd, ok := d.(*syntax.ClassDecl)
			if !ok {
				continue
			}
			name := outer + cd.Name
			visit(cd.Members, name+".")
			if c.parcelizeAnnotated(cd) || c.parcelizeInherited(cd, outer) {
				c.addParcelMembers(cd, name)
			}
		}
	}
	visit(c.file.Decls, prefix)
}

// parcelizeSealed returns the qualified names of the program's sealed
// classes and interfaces annotated @Parcelize, whose direct subclasses
// the plugin makes Parcelable. It reads the syntax, as the files declare
// their classes before any supertype resolves.
func parcelizeSealed(checkers []*Checker) map[string]bool {
	sealed := map[string]bool{}
	for _, c := range checkers {
		var visit func(decls []syntax.Decl, outer string)
		visit = func(decls []syntax.Decl, outer string) {
			for _, d := range decls {
				if cd, ok := d.(*syntax.ClassDecl); ok {
					if cd.Sealed && c.parcelizeAnnotated(cd) {
						sealed[outer+cd.Name] = true
					}
					visit(cd.Members, outer+cd.Name+".")
				}
			}
		}
		prefix := ""
		if c.file.Package != "" {
			prefix = c.file.Package + "."
		}
		visit(c.file.Decls, prefix)
	}
	return sealed
}

// parcelizeInherited reports whether a direct supertype of class cd,
// declared in scope outer (the qualified name of the package or class
// that declares it, and a dot), is one of parcelizeSealed. A sealed
// class's subclasses are in its package, where the name resolves in one
// of the scopes around cd, or through an explicit import, or is
// qualified.
func (c *Checker) parcelizeInherited(cd *syntax.ClassDecl, outer string) bool {
	if len(c.u.parcelSealed) == 0 {
		return false
	}
	for _, s := range cd.Supers {
		name := s.Type.Name
		if name == "" {
			continue
		}
		first, _, _ := strings.Cut(name, ".")
		for _, imp := range c.file.Imports {
			alias := imp.Alias
			if alias == "" {
				alias = imp.Path[strings.LastIndexByte(imp.Path, '.')+1:]
			}
			if !imp.Star && alias == first && c.u.parcelSealed[imp.Path+name[len(first):]] {
				return true
			}
		}
		for scope := outer; ; {
			if c.u.parcelSealed[scope+name] {
				return true
			}
			if scope == "" {
				break
			}
			scope = scope[:strings.LastIndexByte(scope[:len(scope)-1], '.')+1]
		}
	}
	return false
}

// parcelizeAnnotated reports whether a class of the file is annotated
// @kotlinx.parcelize.Parcelize.
func (c *Checker) parcelizeAnnotated(cd *syntax.ClassDecl) bool {
	for _, a := range cd.Annotations {
		if a.Target != "" {
			continue
		}
		if a.Name == "kotlinx.parcelize.Parcelize" {
			return true
		}
		if a.Name == "Parcelize" && c.importsParcelize() {
			return true
		}
	}
	return false
}

// importsParcelize reports whether the file can name kotlinx.parcelize's
// Parcelize as Parcelize.
func (c *Checker) importsParcelize() bool {
	if c.file.Package == "kotlinx.parcelize" {
		return true
	}
	for _, imp := range c.file.Imports {
		if imp.Star && imp.Path == "kotlinx.parcelize" || !imp.Star && imp.Path == "kotlinx.parcelize.Parcelize" && imp.Alias == "" {
			return true
		}
	}
	return false
}

// addParcelMembers declares the members that the plugin gives class cd,
// whose qualified name is name: describeContents, unless the class
// declares it, writeToParcel and the class Creator, whose bodies
// parcelBody writes.
func (c *Checker) addParcelMembers(cd *syntax.ClassDecl, name string) {
	switch {
	case cd.Sealed:
		return // its subclasses are Parcelable
	case cd.Interface || cd.Annotation:
		c.errorf(cd.P, "'Parcelable' must be a class")
		return
	case cd.Abstract:
		c.errorf(cd.P, "'Parcelable' cannot be an abstract class")
		return
	case cd.Inner:
		c.errorf(cd.P, "'Parcelable' can't be an inner class")
		return
	case !cd.Object && !cd.Enum && !cd.HasPrimary:
		c.errorf(cd.P, "'Parcelable' should have a primary constructor")
		return
	}
	if a := cd.Find("", "TypeParceler"); a != nil {
		c.unsupported(a.P, "parcelize", "@TypeParceler is not supported")
		return
	}
	for _, p := range cd.CtorParams {
		if cd.Enum {
			break
		}
		if !p.IsProp {
			c.errorf(p.P, "'Parcelable' constructor parameter should be 'val' or 'var'")
			return
		}
	}
	describe := true
	for _, m := range cd.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			if m.Name == "describeContents" && len(m.Params) == 0 {
				describe = false
			}
		case *syntax.ClassDecl:
			if m.Companion && len(m.Supers) > 0 {
				// A Parceler, which writes and reads the class itself, or
				// not: kc can't tell the supertypes apart before they
				// resolve.
				c.unsupported(m.P, "parcelize", "@Parcelize classes with a companion object that has supertypes are not supported")
				return
			}
		}
	}
	// A generic class's Creator is of the class star-projected, C<*>, as
	// kotlinc's.
	if n := len(cd.TypeParams); n > 0 {
		name += "<" + strings.Repeat("*, ", n-1) + "*>"
	}
	// As kotlinc's, the members are final in a final class.
	final := ""
	if !cd.Open {
		final = "final "
	}
	var src strings.Builder
	if describe {
		fmt.Fprintf(&src, "%soverride fun describeContents(): kotlin.Int = 0\n", final)
	}
	fmt.Fprintf(&src, "%soverride fun writeToParcel(dest: android.os.Parcel, flags: kotlin.Int) {}\n", final)
	fmt.Fprintf(&src, "class Creator : android.os.Parcelable.Creator<%[1]s> {\n"+
		"final override fun createFromParcel(parcel: android.os.Parcel): %[1]s { throw java.lang.IllegalStateException() }\n"+
		"final override fun newArray(size: kotlin.Int): kotlin.Array<%[1]s?> = arrayOfNulls<%[1]s>(size)\n"+
		"}\n", name)
	members, err := parseMembers(c.file.Path, cd.P, src.String())
	if err != nil {
		c.errorf(cd.P, "parcelize: %v", err)
		return
	}
	for _, m := range members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			m.Plugin = true
		case *syntax.ClassDecl:
			for _, cm := range m.Members {
				cm.(*syntax.FunDecl).Plugin = true
			}
		}
	}
	cd.Members = append(cd.Members, members...)
	cd.Parcelize = true
}

// parseMembers parses the members of a class, written in Kotlin, as if
// they were all at position p of a file: the code kc writes for a
// compiler plugin, which is at the class's position, as kotlinc's.
func parseMembers(path string, p syntax.Pos, src string) ([]syntax.Decl, error) {
	f, errs := syntax.Parse(path, "class X {\n"+src+"}\n")
	if len(errs) > 0 {
		return nil, errs[0]
	}
	members := f.Decls[0].(*syntax.ClassDecl).Members
	setPos(reflect.ValueOf(members), p)
	return members, nil
}

var posType = reflect.TypeFor[syntax.Pos]()

// setPos sets each position in a parsed tree to p.
func setPos(v reflect.Value, p syntax.Pos) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			setPos(v.Elem(), p)
		}
	case reflect.Slice:
		for i := range v.Len() {
			setPos(v.Index(i), p)
		}
	case reflect.Struct:
		if v.Type() == posType {
			if v.CanSet() {
				v.Set(reflect.ValueOf(p))
			}
			return
		}
		if v.Type().PkgPath() != posType.PkgPath() {
			return // a symbol, which parsing leaves nil
		}
		for i := range v.NumField() {
			setPos(v.Field(i), p)
		}
	}
}

// parcelBody writes the body of a parcelize function, d, of class cls:
// writeToParcel, which writes the class's properties, Creator's
// createFromParcel, which reads them, or describeContents.
func (c *Checker) parcelBody(d *syntax.FunDecl, cls *types.ClassSym) {
	if !d.Plugin || cls == nil {
		return
	}
	var target *types.ClassSym
	switch {
	case d.Name == "describeContents":
		c.parcelDescribe(d, cls)
		return
	case d.Name == "writeToParcel":
		target = cls
	case d.Name == "createFromParcel" && cls.Outer != nil:
		target = cls.Outer
	default:
		return
	}
	cd := target.Decl.(*syntax.ClassDecl)
	g := &parcelGen{c: c, fq: fqName(target)}
	// A generic class's properties are written as their types erased,
	// each type parameter as its bound, and read into a call of its
	// constructor with the bounds as type arguments.
	targs := ""
	if len(cd.TypeParams) > 0 {
		bounds := make([]string, len(cd.TypeParams))
		for i, tp := range cd.TypeParams {
			b, ok := parcelBound(tp.Sym)
			if !ok {
				c.unsupported(tp.P, "parcelize", "@Parcelize classes with a type parameter that has several bounds, or a bound that names a type parameter, are not supported")
				return
			}
			bounds[i] = g.typeText(b)
		}
		targs = "<" + strings.Join(bounds, ", ") + ">"
	}
	switch {
	case cd.Enum && d.Name == "writeToParcel":
		g.stmt("dest.writeString(this.name)")
	case cd.Enum:
		g.stmt("return %s.valueOf(parcel.readString())", g.fq)
	case d.Name == "writeToParcel":
		if target.Object {
			g.stmt("dest.writeInt(1)")
		}
		for _, p := range cd.CtorParams {
			for _, name := range []string{"DataClass", "WriteWith"} {
				if a := parcelTypeAnnotation(p.Type, name); a != nil {
					c.unsupported(a.P, "parcelize", "@%s is not supported", name)
					return
				}
			}
			v := g.fresh("v")
			g.stmt("val %s = this.`%s`", v, p.Name)
			t := parcelErase(p.Sym.Type)
			if ser := g.serializer(t, p.Type, false); ser != nil {
				g.write(ser, t, v)
			}
		}
	default:
		if target.Object {
			g.stmt("parcel.readInt()")
			g.stmt("return %s", g.fq)
		} else {
			args := make([]string, len(cd.CtorParams))
			for i, p := range cd.CtorParams {
				args[i] = g.fresh("a")
				x := "null!!"
				t := parcelErase(p.Sym.Type)
				if ser := g.serializer(t, p.Type, false); ser != nil {
					x = g.read(ser, t)
				}
				g.stmt("val %s = %s", args[i], x)
			}
			g.stmt("return %s%s(%s)", g.fq, targs, strings.Join(args, ", "))
		}
	}
	if g.err != "" {
		if d.Name == "writeToParcel" {
			c.unsupported(cd.P, "parcelize", "%s", g.err)
		}
		return // createFromParcel fails too, and its body throws
	}
	// The CREATOR fields that the body reads, in locals declared first.
	var src strings.Builder
	for i, cr := range g.creators {
		fmt.Fprintf(&src, "val `$c%d`: android.os.Parcelable.Creator<%s> = null!!\n", i, g.typeText(cr))
	}
	src.WriteString(g.body.String())
	members, err := parseMembers(c.file.Path, cd.P, "fun f() {\n"+src.String()+"}\n")
	if err != nil {
		c.errorf(cd.P, "parcelize: %v", err)
		return
	}
	body := members[0].(*syntax.FunDecl).Body
	creator := c.u.ext.class("android/os/Parcelable$Creator")
	for i, cr := range g.creators {
		vs := body.Stmts[i].(*syntax.VarStmt)
		t := types.GenericType(creator, []types.Arg{{T: cr}})
		id := &syntax.Ident{Name: "CREATOR", Sym: &types.VarSym{Name: "CREATOR", Kind: types.Static, Owner: cr.C, Type: t, Resolved: true}}
		id.P, id.T = vs.P, t
		vs.Init = checked(id, t)
	}
	d.Body = body
}

// parcelDescribe writes the body of describeContents, d, of class cls:
// CONTENTS_FILE_DESCRIPTOR, 1, if one of its properties holds a
// java.io.FileDescriptor, as kotlinc's, and otherwise 0, which the body
// that addParcelMembers declares returns.
func (c *Checker) parcelDescribe(d *syntax.FunDecl, cls *types.ClassSym) {
	cd := cls.Decl.(*syntax.ClassDecl)
	if cd.Enum {
		return
	}
	for _, p := range cd.CtorParams {
		if p.Sym != nil && parcelFileDescriptors(parcelErase(p.Sym.Type)) {
			members, err := parseMembers(c.file.Path, cd.P, "fun f(): kotlin.Int {\nreturn 1\n}\n")
			if err != nil {
				c.errorf(cd.P, "parcelize: %v", err)
				return
			}
			d.Body, d.ExprBody = members[0].(*syntax.FunDecl).Body, nil
			return
		}
	}
}

// parcelTypeAnnotation returns an annotation of kotlinx.parcelize's
// named name in type ref, at any depth, or nil: @DataClass, with which the
// plugin writes a data class by its properties, or @WriteWith, which
// names a Parceler.
func parcelTypeAnnotation(ref *syntax.TypeRef, name string) *syntax.Annotation {
	if ref == nil {
		return nil
	}
	if a := parcelAnnotation(ref, name); a != nil {
		return a
	}
	for _, a := range ref.Args {
		if a := parcelTypeAnnotation(a.Type, name); a != nil {
			return a
		}
	}
	for _, r := range append([]*syntax.TypeRef{ref.Ret, ref.Recv, ref.Outer, ref.And}, ref.Params...) {
		if a := parcelTypeAnnotation(r, name); a != nil {
			return a
		}
	}
	return nil
}

// parcelAnnotation returns the annotation of kotlinx.parcelize's named
// name on type ref itself, or nil.
func parcelAnnotation(ref *syntax.TypeRef, name string) *syntax.Annotation {
	for i := range ref.Annotations {
		if a := &ref.Annotations[i]; a.Name == name || a.Name == "kotlinx.parcelize."+name {
			return a
		}
	}
	return nil
}

// parcelBound returns the bound that type parameter p erases to, if the
// plugin's reads of it can be passed to the class's constructor with it
// as the type argument: if p has one bound, at most, which names no type
// parameter.
func parcelBound(p *types.TypeParamSym) (types.Type, bool) {
	if p == nil || len(p.Bounds) > 1 {
		return types.Type{}, false
	}
	b := p.Bound()
	if parcelHasParam(b) {
		return types.Type{}, false
	}
	return b, true
}

// parcelHasParam reports whether type t names a type parameter.
func parcelHasParam(t types.Type) bool {
	if t.IsParam() {
		return true
	}
	if t.K == types.Class && t.A != nil {
		for _, a := range t.A.List {
			if a.V != types.Star && parcelHasParam(a.T) {
				return true
			}
		}
	}
	return false
}

// parcelErase returns type t with each type parameter in it replaced by
// its bound, nullable if the parameter's value can be null, as the
// plugin dispatches on a type parameter's erased upper bound. A type
// parameter whose bound parcelBound rejects is left as it is.
func parcelErase(t types.Type) types.Type {
	if t.K == types.TypeParam {
		b, ok := parcelBound(t.Param())
		if !ok {
			return t
		}
		if t.N || t.Param().NullableBound() && !t.D {
			return b.Nullable()
		}
		return b.NonNull()
	}
	if t.K != types.Class || t.A == nil {
		return t
	}
	args := make([]types.Arg, t.A.Len())
	changed := false
	for i, a := range t.A.List {
		args[i] = a
		if a.V != types.Star {
			args[i].T = parcelErase(a.T)
			changed = changed || args[i].T != a.T
		}
	}
	if changed {
		t.A = types.MakeArgs(args)
	}
	return t
}

// parcelGen writes the Kotlin of a parcelize function's body: Parcel's
// methods called on dest, in writeToParcel, or parcel, in
// createFromParcel.
type parcelGen struct {
	c    *Checker
	fq   string // the qualified name of the class annotated @Parcelize
	body bytes.Buffer
	n    int
	// creators are the types of the classes whose CREATOR fields the body
	// reads, as locals $c0, $c1...: the property's type, which for a
	// generic class has type arguments that the class's CREATOR, of C<*>,
	// doesn't.
	creators []types.Type
	err      string // why a property's type can't be written, if it can't
}

func (g *parcelGen) stmt(format string, args ...any) {
	fmt.Fprintf(&g.body, format, args...)
	g.body.WriteByte('\n')
}

// fresh returns a new local's name, which no name in the program can hide.
func (g *parcelGen) fresh(prefix string) string {
	g.n++
	return fmt.Sprintf("`$%s%d`", prefix, g.n)
}

func (g *parcelGen) fail(t types.Type) {
	if g.err == "" {
		g.err = fmt.Sprintf("parcelize can't write a property of type %s", t)
	}
}

// A parcelKind is how the plugin writes a type: the IrParcelSerializer
// that kotlinc's IrParcelSerializerFactory picks for it.
type parcelKind uint8

const (
	parcelSimple       parcelKind = iota // dest.write(v) and parcel.read()
	parcelPrimitive                      // writeInt and the others; Boolean, Short and Char as Ints
	parcelLoader                         // dest.write(v) and parcel.read(classLoader): Bundle, PersistableBundle
	parcelCharSequence                   // TextUtils.writeToParcel and CHAR_SEQUENCE_CREATOR
	parcelEnum                           // writeString(name)
	parcelEfficient                      // a final Parcelable of the module's: its writeToParcel and CREATOR
	parcelGeneric                        // writeParcelable, which takes null
	parcelObject                         // writeInt(1), and the object
	parcelArray                          // the size and the elements: ShortArray and Array<T>
	parcelSparse                         // the size and each key and value: SparseIntArray, SparseLongArray, SparseArray<T>
	parcelCollection                     // the size and the elements: lists and sets
	parcelMap                            // the size and each key and value
	parcelValue                          // writeValue and readValue(classLoader): a type annotated @RawValue
)

// A parcelSer is how the plugin writes a type, and its elements'.
type parcelSer struct {
	kind parcelKind
	// nullAware is set if the plugin writes a null of a nullable type
	// as a null marker, 0, and a value as 1 before it, rather than as the
	// value itself.
	nullAware bool
	// write and read are the Parcel methods of parcelSimple and
	// parcelLoader. A parcelSimple value read is cast to the type if cast
	// is set, and a value written to wcast if it isn't empty.
	write, read string
	cast        bool
	wcast       string
	// coll is the class that parcelSparse, parcelCollection and parcelMap
	// read into, with type arguments, and capacity whether its
	// constructor takes the size.
	coll     string
	capacity bool
	// elem writes the elements (a map's values), and key a map's keys.
	elem, key *parcelSer
}

// parcelCollections are the classes the plugin writes as lists, sets and
// maps, by Kotlin or JVM name: the class it reads each into, whether its
// constructor takes the size, and whether a list of String or IBinder is
// written with writeStringList or writeBinderList.
var parcelCollections = map[string]struct {
	kind           parcelKind
	coll           string
	capacity, fast bool
}{
	"kotlin/collections/List":                {parcelCollection, "java.util.ArrayList", true, true},
	"kotlin/collections/MutableList":         {parcelCollection, "java.util.ArrayList", true, true},
	"java/util/List":                         {parcelCollection, "java.util.ArrayList", true, true},
	"java/util/ArrayList":                    {parcelCollection, "java.util.ArrayList", true, true},
	"kotlin/collections/ArrayDeque":          {parcelCollection, "kotlin.collections.ArrayDeque", true, false},
	"java/util/ArrayDeque":                   {parcelCollection, "java.util.ArrayDeque", true, false},
	"kotlin/collections/Set":                 {parcelCollection, "java.util.LinkedHashSet", true, false},
	"kotlin/collections/MutableSet":          {parcelCollection, "java.util.LinkedHashSet", true, false},
	"java/util/Set":                          {parcelCollection, "java.util.LinkedHashSet", true, false},
	"java/util/HashSet":                      {parcelCollection, "java.util.HashSet", true, false},
	"java/util/LinkedHashSet":                {parcelCollection, "java.util.LinkedHashSet", true, false},
	"java/util/NavigableSet":                 {parcelCollection, "java.util.TreeSet", false, false},
	"java/util/SortedSet":                    {parcelCollection, "java.util.TreeSet", false, false},
	"kotlin/collections/Map":                 {parcelMap, "java.util.LinkedHashMap", true, false},
	"kotlin/collections/MutableMap":          {parcelMap, "java.util.LinkedHashMap", true, false},
	"java/util/Map":                          {parcelMap, "java.util.LinkedHashMap", true, false},
	"java/util/HashMap":                      {parcelMap, "java.util.HashMap", true, false},
	"java/util/LinkedHashMap":                {parcelMap, "java.util.LinkedHashMap", true, false},
	"java/util/SortedMap":                    {parcelMap, "java.util.TreeMap", false, false},
	"java/util/NavigableMap":                 {parcelMap, "java.util.TreeMap", false, false},
	"java/util/TreeMap":                      {parcelMap, "java.util.TreeMap", false, false},
	"java/util/concurrent/ConcurrentHashMap": {parcelMap, "java.util.concurrent.ConcurrentHashMap", true, false},
}

// parcelSimples are the classes the plugin writes with a Parcel method
// of their own, by Kotlin or JVM name, and whether with a null marker.
var parcelSimples = map[string]parcelSer{
	"java/io/FileDescriptor":          {write: "writeFileDescriptor", read: "readFileDescriptor", cast: true, nullAware: true},
	"android/util/Size":               {write: "writeSize", read: "readSize", nullAware: true},
	"android/util/SizeF":              {write: "writeSizeF", read: "readSizeF", nullAware: true},
	"kotlin/IntArray":                 {write: "writeIntArray", read: "createIntArray"},
	"kotlin/LongArray":                {write: "writeLongArray", read: "createLongArray"},
	"kotlin/FloatArray":               {write: "writeFloatArray", read: "createFloatArray"},
	"kotlin/DoubleArray":              {write: "writeDoubleArray", read: "createDoubleArray"},
	"kotlin/BooleanArray":             {write: "writeBooleanArray", read: "createBooleanArray"},
	"kotlin/ByteArray":                {write: "writeByteArray", read: "createByteArray"},
	"kotlin/CharArray":                {write: "writeCharArray", read: "createCharArray"},
	"android/util/SparseBooleanArray": {write: "writeSparseBooleanArray", read: "readSparseBooleanArray"},
}

// parcelUnsupported are the classes the plugin writes in ways of its own
// that kc doesn't: ranges, Duration and Uuid, the unsigned integers and
// their arrays, and kotlinx.collections.immutable's collections.
var parcelUnsupported = map[string]bool{
	"kotlin/ranges/IntRange": true, "kotlin/ranges/LongRange": true, "kotlin/ranges/CharRange": true,
	"kotlin/time/Duration": true, "kotlin/uuid/Uuid": true,
	"kotlin/UInt": true, "kotlin/ULong": true, "kotlin/UShort": true, "kotlin/UByte": true,
	"kotlin/UIntArray": true, "kotlin/ULongArray": true, "kotlin/UShortArray": true, "kotlin/UByteArray": true,
	"kotlinx/collections/immutable/PersistentList": true, "kotlinx/collections/immutable/ImmutableList": true,
	"kotlinx/collections/immutable/PersistentSet": true, "kotlinx/collections/immutable/ImmutableSet": true,
	"kotlinx/collections/immutable/PersistentMap": true, "kotlinx/collections/immutable/ImmutableMap": true,
}

// parcelArg returns the type of type t's argument i as the plugin sees
// it: its upper bound, Any? for a star or an in projection.
func parcelArg(t types.Type, i int) types.Type {
	if t.A == nil || i >= t.A.Len() {
		return types.AnyType.Nullable()
	}
	a := t.A.Get(i)
	if a.V == types.Star || a.V == types.In {
		return types.AnyType.Nullable()
	}
	return a.T
}

// isClass reports whether type t is of the class with JVM name name.
func isClass(t types.Type, name string) bool {
	return t.K == types.Class && t.C.Name == name
}

// serializer returns how the plugin writes a value of type t, as its
// IrParcelSerializerFactory picks it: by the exact class for the types
// it knows, then Parcelables, binders, objects, enums, functions and
// Serializables, and else with writeValue. It returns nil for a type kc
// can't write.
//
// ref is the type as written, if it is known, whose type arguments are
// t's. kc writes a type with writeValue only if it, or a type it is an
// argument of, is annotated @RawValue (raw), as kotlinc reports an error
// for one that isn't, which it leaves to kotlinc.
func (g *parcelGen) serializer(t types.Type, ref *syntax.TypeRef, raw bool) *parcelSer {
	if ref != nil && parcelAnnotation(ref, "RawValue") != nil {
		raw = true
	}
	t = t.NonNull()
	switch t.K {
	case types.String:
		return &parcelSer{kind: parcelSimple, write: "writeString", read: "readString"}
	case types.Int, types.Long, types.Float, types.Double, types.Byte, types.Short, types.Char, types.Bool:
		return &parcelSer{kind: parcelPrimitive, nullAware: true}
	case types.Class:
	case types.Any:
		return g.value(t, raw)
	default:
		g.fail(t)
		return nil
	}
	if t.IsFunc() {
		if t.IsSuspendFunc() || t.IsComposableFunc() || t.Contexts() > 0 {
			g.fail(t)
			return nil
		}
		return &parcelSer{kind: parcelSimple, write: "writeSerializable", read: "readSerializable", cast: true, wcast: "java.io.Serializable"}
	}
	cls := t.C
	cls.Ensure()
	name := cls.KotlinName
	if name == "" {
		name = cls.Name
	}
	known := func(m map[string]bool) bool { return m[name] || m[cls.Name] }
	if known(parcelUnsupported) || cls.Value {
		g.fail(t)
		return nil
	}
	lookup := func(name string) string {
		if _, ok := parcelCollections[name]; ok {
			return name
		}
		if _, ok := parcelSimples[name]; ok {
			return name
		}
		return ""
	}
	key := lookup(name)
	if key == "" {
		key = lookup(cls.Name)
	}
	if key == "" {
		key = name
	}
	switch key {
	case "kotlin/CharSequence", "java/lang/CharSequence":
		return &parcelSer{kind: parcelCharSequence}
	case "android/os/Bundle":
		return &parcelSer{kind: parcelLoader, write: "writeBundle", read: "readBundle"}
	case "android/os/PersistableBundle":
		return &parcelSer{kind: parcelLoader, write: "writePersistableBundle", read: "readPersistableBundle"}
	case "kotlin/Array", "kotlin/ShortArray":
		e := types.ArrayElem(t)
		eref := parcelArgRef(ref, t, 0)
		if key == "kotlin/ShortArray" {
			eref = nil
		}
		if key == "kotlin/Array" {
			switch en := e.NonNull(); {
			case en.K == types.String:
				return &parcelSer{kind: parcelSimple, write: "writeStringArray", read: "createStringArray"}
			case isClass(en, "android/os/IBinder"):
				return &parcelSer{kind: parcelSimple, write: "writeBinderArray", read: "createBinderArray"}
			}
		}
		return g.withElem(&parcelSer{kind: parcelArray, nullAware: true}, e, eref, raw)
	case "android/util/SparseIntArray":
		return g.withElem(&parcelSer{kind: parcelSparse, nullAware: true, coll: "android.util.SparseIntArray"}, types.IntType, nil, raw)
	case "android/util/SparseLongArray":
		return g.withElem(&parcelSer{kind: parcelSparse, nullAware: true, coll: "android.util.SparseLongArray"}, types.LongType, nil, raw)
	case "android/util/SparseArray":
		e := parcelArg(t, 0)
		return g.withElem(&parcelSer{kind: parcelSparse, nullAware: true, coll: "android.util.SparseArray<" + g.typeText(e) + ">"}, e, parcelArgRef(ref, t, 0), raw)
	}
	if s, ok := parcelSimples[key]; ok {
		s.kind = parcelSimple
		return &s
	}
	if c, ok := parcelCollections[key]; ok {
		if c.kind == parcelMap {
			k, v := parcelArg(t, 0), parcelArg(t, 1)
			s := &parcelSer{kind: parcelMap, nullAware: true, coll: c.coll + "<" + g.typeText(k) + ", " + g.typeText(v) + ">", capacity: c.capacity}
			if s.key = g.serializer(k, parcelArgRef(ref, t, 0), raw); s.key == nil {
				return nil
			}
			return g.withElem(s, v, parcelArgRef(ref, t, 1), raw)
		}
		e := parcelArg(t, 0)
		if c.fast {
			switch en := e.NonNull(); {
			case en.K == types.String:
				return &parcelSer{kind: parcelSimple, write: "writeStringList", read: "createStringArrayList"}
			case isClass(en, "android/os/IBinder"):
				return &parcelSer{kind: parcelSimple, write: "writeBinderList", read: "createBinderArrayList"}
			}
		}
		return g.withElem(&parcelSer{kind: parcelCollection, nullAware: true, coll: c.coll + "<" + g.typeText(e) + ">", capacity: c.capacity}, e, parcelArgRef(ref, t, 0), raw)
	}
	if p := g.c.u.ext.class("android/os/Parcelable"); p != nil && cls.IsSubclassOf(p) {
		if g.efficient(cls) {
			return &parcelSer{kind: parcelEfficient, nullAware: true}
		}
		return &parcelSer{kind: parcelGeneric}
	}
	if b := g.c.u.ext.class("android/os/IBinder"); b != nil && cls.IsSubclassOf(b) {
		return &parcelSer{kind: parcelSimple, write: "writeStrongBinder", read: "readStrongBinder", cast: cls != b}
	}
	if cls.Object {
		return &parcelSer{kind: parcelObject}
	}
	if cls.Entries != nil {
		return &parcelSer{kind: parcelEnum, nullAware: true}
	}
	if s := g.c.u.ext.class("java/io/Serializable"); s != nil && cls.IsSubclassOf(s) {
		return &parcelSer{kind: parcelSimple, write: "writeSerializable", read: "readSerializable", cast: true}
	}
	return g.value(t, raw)
}

// value returns the serializer of a type t that the plugin writes with
// writeValue, if it is annotated @RawValue (raw), or else reports that kc
// can't write it.
func (g *parcelGen) value(t types.Type, raw bool) *parcelSer {
	if !raw {
		g.fail(t)
		return nil
	}
	return &parcelSer{kind: parcelValue}
}

// parcelArgRef returns type argument i of type ref, written, if it is
// type t's: ref is of t's class and has as many arguments. A type alias
// may have others.
func parcelArgRef(ref *syntax.TypeRef, t types.Type, i int) *syntax.TypeRef {
	if ref == nil || ref.Name == "" || t.A == nil || len(ref.Args) != t.A.Len() {
		return nil
	}
	return ref.Args[i].Type
}

// withElem returns s with the serializer of its elements, of type e,
// written as ref, or nil if kc can't write them.
func (g *parcelGen) withElem(s *parcelSer, e types.Type, ref *syntax.TypeRef, raw bool) *parcelSer {
	if s.elem = g.serializer(e, ref, raw); s.elem == nil {
		return nil
	}
	return s
}

// efficient reports whether the plugin writes a Parcelable of class cls
// with its writeToParcel, and reads it with its CREATOR, rather than with
// writeParcelable: a final class, or object, of the module's that the
// plugin makes Parcelable, or that has a CREATOR in its companion object.
// Other modules' classes, even such ones, it writes with writeParcelable.
func (g *parcelGen) efficient(cls *types.ClassSym) bool {
	cd, ok := cls.Decl.(*syntax.ClassDecl)
	if !ok || !cls.Final {
		return false
	}
	return cd.Parcelize || cls.Companion != nil && cls.Companion.Prop("CREATOR") != nil
}

// write writes the statements that write value v, a local, of type t
// with serializer s.
func (g *parcelGen) write(s *parcelSer, t types.Type, v string) {
	if t.N && s.nullAware {
		// A null marker, 0 or 1, and then the value.
		g.stmt("if (%s == null) dest.writeInt(0) else {", v)
		g.stmt("dest.writeInt(1)")
		g.write(s, t.NonNull(), v)
		g.stmt("}")
		return
	}
	switch s.kind {
	case parcelSimple:
		if s.wcast != "" {
			q := ""
			if t.N {
				q = "?"
			}
			v = fmt.Sprintf("(%s as %s%s)", v, s.wcast, q)
		}
		g.stmt("dest.%s(%s)", s.write, v)
	case parcelLoader:
		g.stmt("dest.%s(%s)", s.write, v)
	case parcelValue:
		g.stmt("dest.writeValue(%s)", v)
	case parcelCharSequence:
		g.stmt("android.text.TextUtils.writeToParcel(%s, dest, flags)", v)
	case parcelPrimitive:
		switch t.K {
		case types.Int:
			g.stmt("dest.writeInt(%s)", v)
		case types.Long:
			g.stmt("dest.writeLong(%s)", v)
		case types.Float:
			g.stmt("dest.writeFloat(%s)", v)
		case types.Double:
			g.stmt("dest.writeDouble(%s)", v)
		case types.Byte:
			g.stmt("dest.writeByte(%s)", v)
		case types.Short:
			g.stmt("dest.writeInt(%s.toInt())", v)
		case types.Char:
			g.stmt("dest.writeInt(%s.code)", v)
		case types.Bool:
			g.stmt("dest.writeInt(if (%s) 1 else 0)", v)
		}
	case parcelEnum:
		g.stmt("dest.writeString(%s.name)", v)
	case parcelEfficient:
		g.stmt("%s.writeToParcel(dest, flags)", v)
	case parcelGeneric:
		g.stmt("dest.writeParcelable(%s, flags)", v)
	case parcelObject:
		g.stmt("dest.writeInt(1)")
	case parcelArray, parcelCollection:
		g.stmt("dest.writeInt(%s.size)", v)
		e := g.fresh("e")
		g.stmt("for (%s in %s) {", e, v)
		if s.kind == parcelArray {
			g.write(s.elem, types.ArrayElem(t), e)
		} else {
			g.write(s.elem, parcelArg(t, 0), e)
		}
		g.stmt("}")
	case parcelSparse:
		n, i, x := g.fresh("n"), g.fresh("i"), g.fresh("x")
		g.stmt("val %s = %s.size()", n, v)
		g.stmt("dest.writeInt(%s)", n)
		g.stmt("var %s = 0", i)
		g.stmt("while (%s != %s) {", i, n)
		g.stmt("dest.writeInt(%s.keyAt(%s))", v, i)
		g.stmt("val %s = %s.valueAt(%s)", x, v, i)
		g.write(s.elem, g.sparseElem(t), x)
		g.stmt("%s++", i)
		g.stmt("}")
	case parcelMap:
		g.stmt("dest.writeInt(%s.size)", v)
		e, key, val := g.fresh("e"), g.fresh("k"), g.fresh("w")
		g.stmt("for (%s in %s.entries) {", e, v)
		g.stmt("val %s = %s.key", key, e)
		g.stmt("val %s = %s.value", val, e)
		g.write(s.key, parcelArg(t, 0), key)
		g.write(s.elem, parcelArg(t, 1), val)
		g.stmt("}")
	}
}

// sparseElem returns the type of the values of a sparse array of type t.
func (g *parcelGen) sparseElem(t types.Type) types.Type {
	switch t.C.Name {
	case "android/util/SparseIntArray":
		return types.IntType
	case "android/util/SparseLongArray":
		return types.LongType
	}
	return parcelArg(t, 0)
}

// read writes the statements that read a value of type t with serializer
// s, and returns the expression that is the value then.
func (g *parcelGen) read(s *parcelSer, t types.Type) string {
	if t.N && s.nullAware {
		// After a null marker. A value that takes statements to read is
		// read into a local.
		mark := g.body.Len()
		x := g.read(s, t.NonNull())
		if g.body.Len() == mark {
			return fmt.Sprintf("(if (parcel.readInt() == 0) null else %s)", x)
		}
		stmts := g.body.String()[mark:]
		g.body.Truncate(mark)
		v := g.fresh("n")
		g.stmt("var %s: %s = null", v, g.typeText(t))
		g.stmt("if (parcel.readInt() != 0) {")
		g.body.WriteString(stmts)
		g.stmt("%s = %s", v, x)
		g.stmt("}")
		return v
	}
	switch s.kind {
	case parcelSimple:
		if s.cast {
			return fmt.Sprintf("(parcel.%s() as %s)", s.read, g.typeText(t))
		}
		return fmt.Sprintf("parcel.%s()", s.read)
	case parcelLoader:
		return fmt.Sprintf("parcel.%s(%s::class.java.classLoader)", s.read, g.fq)
	case parcelValue:
		x := fmt.Sprintf("parcel.readValue(%s::class.java.classLoader)", g.fq)
		if t.K == types.Any {
			return x // as kotlinc, which doesn't check it
		}
		return fmt.Sprintf("(%s as %s)", x, g.typeText(t))
	case parcelCharSequence:
		return "android.text.TextUtils.CHAR_SEQUENCE_CREATOR.createFromParcel(parcel)"
	case parcelPrimitive:
		switch t.K {
		case types.Int:
			return "parcel.readInt()"
		case types.Long:
			return "parcel.readLong()"
		case types.Float:
			return "parcel.readFloat()"
		case types.Double:
			return "parcel.readDouble()"
		case types.Byte:
			return "parcel.readByte()"
		case types.Short:
			return "parcel.readInt().toShort()"
		case types.Char:
			return "parcel.readInt().toChar()"
		default:
			return "(parcel.readInt() != 0)"
		}
	case parcelEnum:
		return fmt.Sprintf("%s.valueOf(parcel.readString())", fqName(t.C))
	case parcelEfficient:
		return fmt.Sprintf("`$c%d`.createFromParcel(parcel)", g.creator(t))
	case parcelGeneric:
		return fmt.Sprintf("parcel.readParcelable<%s>(%s::class.java.classLoader)", g.typeText(t.NonNull()), g.fq)
	case parcelObject:
		g.stmt("parcel.readInt()")
		return fqName(t.C)
	case parcelArray:
		e := types.ArrayElem(t)
		n, a, i := g.fresh("n"), g.fresh("a"), g.fresh("i")
		g.stmt("val %s = parcel.readInt()", n)
		switch {
		case t.C.KotlinName == "kotlin/ShortArray":
			g.stmt("val %s = kotlin.ShortArray(%s)", a, n)
		case e.N:
			g.stmt("val %s = arrayOfNulls<%s>(%s)", a, g.typeText(e.NonNull()), n)
		default:
			g.stmt("val %s = arrayOfNulls<%s>(%s) as kotlin.Array<%[2]s>", a, g.typeText(e), n)
		}
		g.stmt("var %s = 0", i)
		g.stmt("while (%s != %s) {", i, n)
		g.stmt("%s[%s] = %s", a, i, g.read(s.elem, e))
		g.stmt("%s++", i)
		g.stmt("}")
		return a
	case parcelSparse:
		r, a, k := g.fresh("r"), g.fresh("a"), g.fresh("k")
		g.stmt("var %s = parcel.readInt()", r)
		g.stmt("val %s = %s(%s)", a, s.coll, r)
		g.stmt("while (%s != 0) {", r)
		g.stmt("val %s = parcel.readInt()", k)
		g.stmt("%s.put(%s, %s)", a, k, g.read(s.elem, g.sparseElem(t)))
		g.stmt("%s--", r)
		g.stmt("}")
		return a
	case parcelCollection, parcelMap:
		n, v, i := g.fresh("n"), g.fresh("l"), g.fresh("i")
		g.stmt("val %s = parcel.readInt()", n)
		if s.capacity {
			g.stmt("val %s = %s(%s)", v, s.coll, n)
		} else {
			g.stmt("val %s = %s()", v, s.coll)
		}
		g.stmt("var %s = 0", i)
		g.stmt("while (%s != %s) {", i, n)
		if s.kind == parcelMap {
			key := g.fresh("k")
			g.stmt("val %s = %s", key, g.read(s.key, parcelArg(t, 0)))
			g.stmt("%s.put(%s, %s)", v, key, g.read(s.elem, parcelArg(t, 1)))
		} else {
			g.stmt("%s.add(%s)", v, g.read(s.elem, parcelArg(t, 0)))
		}
		g.stmt("%s++", i)
		g.stmt("}")
		return v
	}
	return "null!!"
}

// parcelFileDescriptors reports whether type t, or one of its type
// arguments, is java.io.FileDescriptor, for which describeContents
// returns CONTENTS_FILE_DESCRIPTOR, 1.
func parcelFileDescriptors(t types.Type) bool {
	if t.K != types.Class {
		return false
	}
	if t.C.Name == "java/io/FileDescriptor" {
		return true
	}
	if t.A != nil {
		for _, a := range t.A.List {
			if a.V != types.Star && parcelFileDescriptors(a.T) {
				return true
			}
		}
	}
	return false
}

// creator returns the number of the local that holds the CREATOR of type
// t's class, as a Creator of t.
func (g *parcelGen) creator(t types.Type) int {
	t = t.NonNull()
	for i, c := range g.creators {
		if c == t {
			return i
		}
	}
	g.creators = append(g.creators, t)
	return len(g.creators) - 1
}

// typeText writes type t as Kotlin, as kotlinType does, and a function
// type too.
func (g *parcelGen) typeText(t types.Type) string {
	if t.IsFunc() && !t.IsSuspendFunc() && !t.IsComposableFunc() && t.Contexts() == 0 {
		args := make([]string, t.A.Len())
		for i, a := range t.A.List {
			if a.V != types.Inv {
				g.fail(t)
				return "kotlin.Any"
			}
			args[i] = g.typeText(a.T)
		}
		s := "(" + strings.Join(args[:len(args)-1], ", ") + ") -> " + args[len(args)-1]
		if t.N {
			s = "(" + s + ")?"
		}
		return s
	}
	s, ok := kotlinType(t)
	if !ok {
		g.fail(t)
		return "kotlin.Any"
	}
	return s
}

// kotlinType writes type t as Kotlin, with qualified names, which mean the
// same anywhere, or reports that it can't: function types and type
// parameters.
func kotlinType(t types.Type) (string, bool) {
	var s string
	switch t.K {
	case types.Int, types.Long, types.Float, types.Double, types.Byte, types.Short, types.Char, types.String, types.Any, types.Unit, types.Nothing:
		s = "kotlin." + t.NonNull().String()
	case types.Bool:
		s = "kotlin.Boolean"
	case types.Class:
		if t.IsFunc() {
			return "", false
		}
		s = fqName(t.C)
		if t.A != nil {
			args := make([]string, t.A.Len())
			for i, a := range t.A.List {
				var arg string
				ok := true
				if a.V != types.Star {
					arg, ok = kotlinType(a.T)
				}
				switch {
				case !ok:
					return "", false
				case a.V == types.Star:
					args[i] = "*"
				case a.V == types.Out:
					args[i] = "out " + arg
				case a.V == types.In:
					args[i] = "in " + arg
				default:
					args[i] = arg
				}
			}
			s += "<" + strings.Join(args, ", ") + ">"
		}
	default:
		return "", false
	}
	if t.N && !t.P {
		s += "?"
	}
	return s, true
}

// fqName returns the qualified name of a class, as Kotlin writes it:
// a.b.Outer.Nested.
func fqName(cls *types.ClassSym) string {
	if cls.KotlinName != "" {
		return strings.ReplaceAll(cls.KotlinName, "/", ".")
	}
	if cls.Outer != nil {
		return fqName(cls.Outer) + "." + cls.Name[strings.LastIndexByte(cls.Name, '$')+1:]
	}
	return strings.ReplaceAll(strings.ReplaceAll(cls.Name, "/", "."), "$", ".")
}
