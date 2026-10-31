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
// annotated too (parcelizeSealed).
//
// Types that the plugin writes in ways kc doesn't are unsupported, as
// are custom Parcelers, @TypeParceler, @WriteWith and @RawValue.

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
	case len(cd.TypeParams) > 0:
		c.unsupported(cd.P, "parcelize", "@Parcelize classes with type parameters are not supported")
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
		for _, a := range []string{"WriteWith", "RawValue"} {
			if a := p.Find("", a); a != nil {
				c.unsupported(a.P, "parcelize", "@%s is not supported", a.Name)
				return
			}
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
// writeToParcel, which writes the class's properties, or Creator's
// createFromParcel, which reads them.
func (c *Checker) parcelBody(d *syntax.FunDecl, cls *types.ClassSym) {
	if !d.Plugin || cls == nil {
		return
	}
	var target *types.ClassSym
	switch {
	case d.Name == "writeToParcel":
		target = cls
	case d.Name == "createFromParcel" && cls.Outer != nil:
		target = cls.Outer
	default:
		return
	}
	cd := target.Decl.(*syntax.ClassDecl)
	g := &parcelGen{c: c, fq: fqName(target)}
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
			v := g.fresh("v")
			g.stmt("val %s = this.`%s`", v, p.Name)
			g.write(p.Sym.Type, v)
		}
	default:
		if target.Object {
			g.stmt("parcel.readInt()")
			g.stmt("return %s", g.fq)
		} else {
			args := make([]string, len(cd.CtorParams))
			for i, p := range cd.CtorParams {
				args[i] = g.fresh("a")
				g.stmt("val %s = %s", args[i], g.read(p.Sym.Type))
			}
			g.stmt("return %s(%s)", g.fq, strings.Join(args, ", "))
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
		fmt.Fprintf(&src, "val `$c%d`: android.os.Parcelable.Creator<%s> = null!!\n", i, fqName(cr))
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
		t := types.GenericType(creator, []types.Arg{{T: types.ClassType(cr)}})
		id := &syntax.Ident{Name: "CREATOR", Sym: &types.VarSym{Name: "CREATOR", Kind: types.Static, Owner: cr, Type: t, Resolved: true}}
		id.P, id.T = vs.P, t
		vs.Init = checked(id, t)
	}
	d.Body = body
}

// parcelGen writes the Kotlin of a parcelize function's body: Parcel's
// methods called on dest, in writeToParcel, or parcel, in
// createFromParcel.
type parcelGen struct {
	c    *Checker
	fq   string // the qualified name of the class annotated @Parcelize
	body bytes.Buffer
	n    int
	// creators are the classes whose CREATOR fields the body reads, as
	// locals $c0, $c1...
	creators []*types.ClassSym
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

// A parcelKind is how the plugin writes a type.
type parcelKind uint8

const (
	parcelUnsupported  parcelKind = iota
	parcelString                  // writeString, which takes null
	parcelPrimitive               // writeInt and the others
	parcelEnum                    // writeString(name)
	parcelStringList              // writeStringList, which takes null
	parcelStringArray             // writeStringArray, which takes null
	parcelEfficient               // a final Parcelable of the module's: its writeToParcel and CREATOR
	parcelGeneric                 // writeParcelable, which takes null
	parcelSerializable            // writeSerializable, which takes null
	parcelList                    // the size and the elements
	parcelSet
	parcelMap
)

// kind returns how the plugin writes a value of (non-null) type t, and
// for a collection, the class it reads it into.
func (g *parcelGen) kind(t types.Type) (parcelKind, string) {
	switch t.K {
	case types.String:
		return parcelString, ""
	case types.Int, types.Long, types.Float, types.Double, types.Byte, types.Short, types.Char, types.Bool:
		return parcelPrimitive, ""
	case types.Class:
	default:
		return parcelUnsupported, ""
	}
	cls := t.C
	cls.Ensure()
	elem := func(i int) (types.Type, bool) {
		if t.A == nil || i >= t.A.Len() || t.A.Get(i).V == types.Star {
			return types.InvalidType, false
		}
		return t.A.Get(i).T, true
	}
	name := cls.KotlinName
	if name == "" {
		name = cls.Name
	}
	switch name {
	case "kotlin/collections/List", "kotlin/collections/MutableList", "java/util/List", "java/util/ArrayList":
		if e, ok := elem(0); ok && e.K == types.String {
			return parcelStringList, ""
		}
		return parcelList, "java.util.ArrayList"
	case "kotlin/collections/Set", "kotlin/collections/MutableSet", "java/util/Set", "java/util/LinkedHashSet":
		return parcelSet, "java.util.LinkedHashSet"
	case "kotlin/collections/Map", "kotlin/collections/MutableMap", "java/util/Map", "java/util/LinkedHashMap":
		return parcelMap, "java.util.LinkedHashMap"
	case "java/util/HashMap":
		return parcelMap, "java.util.HashMap"
	}
	if e, ok := elem(0); ok && cls.Array && e.K == types.String {
		return parcelStringArray, ""
	}
	if cls.Array || parcelSpecial(cls) {
		return parcelUnsupported, ""
	}
	if p := g.c.u.ext.class("android/os/Parcelable"); p != nil && cls.IsSubclassOf(p) {
		if g.efficient(cls) {
			return parcelEfficient, ""
		}
		return parcelGeneric, ""
	}
	if cls.Entries != nil {
		return parcelEnum, ""
	}
	if s := g.c.u.ext.class("java/io/Serializable"); s != nil && cls.IsSubclassOf(s) {
		return parcelSerializable, ""
	}
	return parcelUnsupported, ""
}

// parcelSpecials are the classes that the plugin writes in ways of their
// own, which kc doesn't, and the collections, whose other classes it
// writes in their own ways too or not at all. A class that is one of them
// isn't written as a Parcelable or a Serializable, which many of them are.
var parcelSpecials = map[string]bool{
	"android/os/Bundle": true, "android/os/PersistableBundle": true, "android/os/IBinder": true,
	"android/util/Size": true, "android/util/SizeF": true, "android/util/SparseArray": true,
	"android/util/SparseBooleanArray": true, "android/util/SparseIntArray": true, "android/util/SparseLongArray": true,
	"java/io/FileDescriptor": true, "java/lang/CharSequence": true, "kotlin/CharSequence": true,
	"java/util/Collection": true, "java/util/Map": true, "kotlin/collections/Collection": true, "kotlin/collections/Map": true,
	"kotlin/collections/ArrayDeque": true, "kotlin/ranges/IntRange": true, "kotlin/ranges/LongRange": true,
	"kotlin/ranges/CharRange": true, "kotlin/time/Duration": true, "kotlin/uuid/Uuid": true,
}

// parcelSpecial reports whether class cls is one of parcelSpecials or
// inherits from one.
func parcelSpecial(cls *types.ClassSym) bool {
	found := false
	cls.Supertypes(func(s *types.ClassSym) bool {
		found = parcelSpecials[s.Name] || s.KotlinName != "" && parcelSpecials[s.KotlinName]
		return !found
	})
	return found
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

// nullable reports whether the plugin writes a null of kind k with a
// null marker before the value, rather than as the value itself.
func nullable(k parcelKind) bool {
	return k != parcelString && k != parcelStringList && k != parcelStringArray && k != parcelGeneric && k != parcelSerializable
}

// write writes the statements that write value v, a local, of type t.
func (g *parcelGen) write(t types.Type, v string) {
	k, _ := g.kind(t.NonNull())
	if k == parcelUnsupported {
		g.fail(t)
		return
	}
	if t.N && nullable(k) {
		// A null marker, 0 or 1, and then the value.
		g.stmt("if (%s == null) dest.writeInt(0) else {", v)
		g.stmt("dest.writeInt(1)")
		g.write(t.NonNull(), v)
		g.stmt("}")
		return
	}
	switch k {
	case parcelString:
		g.stmt("dest.writeString(%s)", v)
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
	case parcelStringList:
		g.stmt("dest.writeStringList(%s)", v)
	case parcelStringArray:
		g.stmt("dest.writeStringArray(%s)", v)
	case parcelEfficient:
		g.stmt("%s.writeToParcel(dest, flags)", v)
	case parcelGeneric:
		g.stmt("dest.writeParcelable(%s, flags)", v)
	case parcelSerializable:
		g.stmt("dest.writeSerializable(%s)", v)
	case parcelList, parcelSet:
		g.stmt("dest.writeInt(%s.size)", v)
		e := g.fresh("e")
		g.stmt("for (%s in %s) {", e, v)
		g.write(t.A.Get(0).T, e)
		g.stmt("}")
	case parcelMap:
		g.stmt("dest.writeInt(%s.size)", v)
		e, key, val := g.fresh("e"), g.fresh("k"), g.fresh("w")
		g.stmt("for (%s in %s.entries) {", e, v)
		g.stmt("val %s = %s.key", key, e)
		g.stmt("val %s = %s.value", val, e)
		g.write(t.A.Get(0).T, key)
		g.write(t.A.Get(1).T, val)
		g.stmt("}")
	}
}

// read writes the statements that read a value of type t, and returns the
// expression that is the value then.
func (g *parcelGen) read(t types.Type) string {
	k, coll := g.kind(t.NonNull())
	if k == parcelUnsupported {
		g.fail(t)
		return "null!!"
	}
	if t.N && nullable(k) {
		// After a null marker. A value that takes statements to read is
		// read into a local.
		mark := g.body.Len()
		x := g.read(t.NonNull())
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
	switch k {
	case parcelString:
		return "parcel.readString()"
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
	case parcelStringList:
		return "parcel.createStringArrayList()"
	case parcelStringArray:
		return "parcel.createStringArray()"
	case parcelEfficient:
		return fmt.Sprintf("`$c%d`.createFromParcel(parcel)", g.creator(t.C))
	case parcelGeneric:
		return fmt.Sprintf("parcel.readParcelable<%s>(%s::class.java.classLoader)", g.typeText(t.NonNull()), g.fq)
	case parcelSerializable:
		return fmt.Sprintf("(parcel.readSerializable() as %s)", g.typeText(t))
	case parcelList, parcelSet, parcelMap:
		n, v, i := g.fresh("n"), g.fresh("l"), g.fresh("i")
		args := make([]string, t.A.Len())
		for j := range args {
			args[j] = g.typeText(t.A.Get(j).T)
		}
		g.stmt("val %s = parcel.readInt()", n)
		g.stmt("val %s = %s<%s>(%s)", v, coll, strings.Join(args, ", "), n)
		g.stmt("var %s = 0", i)
		g.stmt("while (%s != %s) {", i, n)
		if k == parcelMap {
			key := g.fresh("k")
			g.stmt("val %s = %s", key, g.read(t.A.Get(0).T))
			g.stmt("%s.put(%s, %s)", v, key, g.read(t.A.Get(1).T))
		} else {
			g.stmt("%s.add(%s)", v, g.read(t.A.Get(0).T))
		}
		g.stmt("%s++", i)
		g.stmt("}")
		return v
	}
	return "null!!"
}

// creator returns the number of the local that holds class cls's CREATOR.
func (g *parcelGen) creator(cls *types.ClassSym) int {
	for i, c := range g.creators {
		if c == cls {
			return i
		}
	}
	g.creators = append(g.creators, cls)
	return len(g.creators) - 1
}

// typeText writes type t as Kotlin, as kotlinType does.
func (g *parcelGen) typeText(t types.Type) string {
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
