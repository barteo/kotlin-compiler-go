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
// arrays, Android's Size and sparse arrays, the collections it knows, and
// the standard library's types that it writes in ways of its own
// (special): the unsigned integers, their arrays and Duration as their
// underlying values, which the plugin's code coerces to and from with
// kc's `kc$coerce` intrinsic, ranges as their bounds and a Uuid as its
// halves; then Parcelables, binders, objects, enums, functions, data
// classes annotated @DataClass, or in one, by their properties (data),
// and Serializables; and else writeValue. Value classes of the program
// are written as any class. kotlinx's immutable collections are read as
// lists, sets and maps, and converted with its toPersistentList and the
// others, which the plugin's code calls by their qualified names
// (pluginQualified). kotlinc reports an error for a
// property written with writeValue unless its type, or one that it is an
// argument of, is annotated @RawValue, so kc leaves such a class to it.
// A generic class's properties are written as their types erased, each
// type parameter as its erased bound (parcelErase), and its Creator is of
// the class star-projected, C<*>. The class is constructed with the
// erased bounds as type arguments, which needn't be within the bounds in
// a plugin's code.
//
// A type parameter whose bound is an array or a collection, which
// kotlinc fails to compile, is unsupported, and so is a @DataClass type of
// another module's data class, for which kotlinc's plugin finds no
// properties and crashes.
//
// A custom Parceler object writes a type that @WriteWith names it for, or
// that a @TypeParceler of the class or the property does (custom). One
// that writes an array's or list's elements keeps the plugin from writing
// it with a Parcel method of its own (hasCustom). A companion object that
// is a Parceler writes and reads the class itself (parcelParceler).

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
	describe := true
	for _, m := range cd.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			if m.Name == "describeContents" && len(m.Params) == 0 {
				describe = false
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
	case d.Name == "newArray" && cls.Outer != nil:
		c.parcelNewArray(d, cls.Outer)
		return
	default:
		return
	}
	cd := target.Decl.(*syntax.ClassDecl)
	g := &parcelGen{c: c, fq: fqName(target)}
	comp, ok := c.parcelParceler(cd, target, d.Name == "writeToParcel")
	if !ok {
		return
	}
	if comp == nil {
		if g.classScope, ok = c.parcelScope(&cd.Modifiers); !ok {
			return
		}
	}
	// A generic class's properties are written as their types erased,
	// each type parameter as its bound, and read into a call of its
	// constructor with the bounds as type arguments.
	targs := ""
	if len(cd.TypeParams) > 0 {
		bounds := make([]string, len(cd.TypeParams))
		for i, tp := range cd.TypeParams {
			bounds[i] = g.typeText(parcelBound(tp.Sym))
		}
		targs = "<" + strings.Join(bounds, ", ") + ">"
	}
	switch {
	case comp != nil && d.Name == "writeToParcel":
		// The companion object, a Parceler, writes the class.
		v := g.fresh("v")
		g.stmt("val %s = this", v)
		g.stmt("kotlin.with(%s) { %s.write(dest, flags) }", fqName(comp), v)
	case comp != nil:
		g.stmt("return %s.create(parcel)", fqName(comp))
	case cd.Enum && d.Name == "writeToParcel":
		g.stmt("dest.writeString(this.name)")
	case cd.Enum:
		g.stmt("return %s.valueOf(parcel.readString())", g.fq)
	case d.Name == "writeToParcel":
		if target.Object {
			g.stmt("dest.writeInt(1)")
		}
		for _, p := range cd.CtorParams {
			if g.scope, ok = c.parcelScope(&p.Modifiers); !ok {
				return
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
			failed := false
			args := g.readAll(len(cd.CtorParams), func(i int) (string, types.Type) {
				p := cd.CtorParams[i]
				t := parcelErase(p.Sym.Type)
				if g.scope, ok = c.parcelScope(&p.Modifiers); !ok {
					failed = true
					return "null!!", t
				}
				if ser := g.serializer(t, p.Type, false); ser != nil {
					return g.read(ser, t), t
				}
				return "null!!", t
			})
			if failed {
				return
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
	members, err := parseMembers(c.file.Path, cd.P, "fun f() {\n"+g.body.String()+"}\n")
	if err != nil {
		c.errorf(cd.P, "parcelize: %v", err)
		return
	}
	body := members[0].(*syntax.FunDecl).Body
	// The CREATOR fields that the body reads, as `$c0` and so on, which
	// Kotlin can't name: a class's own, which the plugin adds.
	creator := c.u.ext.class("android/os/Parcelable$Creator")
	repl := map[string]syntax.Expr{}
	for i, cr := range g.creators {
		t := types.GenericType(creator, []types.Arg{{T: cr}})
		id := &syntax.Ident{Name: "CREATOR", Sym: &types.VarSym{Name: "CREATOR", Kind: types.Static, Owner: cr.C, Type: t, Resolved: true}}
		id.P, id.T = cd.P, t
		repl[fmt.Sprintf("$c%d", i)] = checked(id, t)
	}
	replaceIdents(reflect.ValueOf(body), repl)
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

// parcelNewArray writes the body of Creator's newArray, d, of class cls:
// its companion object's newArray, if the companion is a Parceler that
// declares one, as kotlinc's. The body that addParcelMembers declares
// makes an array of nulls.
func (c *Checker) parcelNewArray(d *syntax.FunDecl, cls *types.ClassSym) {
	cd := cls.Decl.(*syntax.ClassDecl)
	comp, ok := c.parcelParceler(cd, cls, false)
	if !ok || comp == nil {
		return
	}
	declared := false
	if ccd, ok := comp.Decl.(*syntax.ClassDecl); ok {
		for _, m := range ccd.Members {
			if f, ok := m.(*syntax.FunDecl); ok && f.Name == "newArray" && len(f.Params) == 1 && f.Recv == nil {
				declared = true
			}
		}
	}
	if !declared {
		return
	}
	// Parceler's newArray makes an Array<C>, which Creator's is of C?;
	// of C<*> for a generic class.
	name := fqName(cls)
	if n := len(cls.TypeParams); n > 0 {
		name += "<" + strings.Repeat("*, ", n-1) + "*>"
	}
	members, err := parseMembers(c.file.Path, cd.P, fmt.Sprintf(
		"fun f(): kotlin.Array<%[2]s?> {\nreturn %[1]s.newArray(size) as kotlin.Array<%[2]s?>\n}\n", fqName(comp), name))
	if err != nil {
		c.errorf(cd.P, "parcelize: %v", err)
		return
	}
	d.Body, d.ExprBody = members[0].(*syntax.FunDecl).Body, nil
}

// parcelParceler returns class cls's companion object if it is a
// kotlinx.parcelize.Parceler, which writes and reads the class, or nil.
// Without one, the constructor's parameters must be properties, which
// the class is written as: it reports, and returns false, if one isn't,
// in writeToParcel (write). It also returns false, reporting that kc
// can't, if the class star-projected isn't of the Parceler's type.
func (c *Checker) parcelParceler(cd *syntax.ClassDecl, cls *types.ClassSym, write bool) (*types.ClassSym, bool) {
	pc := c.u.ext.class("kotlinx/parcelize/Parceler")
	if cls.Companion == nil || pc == nil || !cls.Companion.IsSubclassOf(pc) {
		for _, p := range cd.CtorParams {
			if !p.IsProp && !cd.Enum {
				if write {
					c.errorf(p.P, "'Parcelable' constructor parameter should be 'val' or 'var'")
				}
				return nil, false
			}
		}
		return nil, true
	}
	if pt, ok := c.asSuper(types.ClassType(cls.Companion), pc); ok {
		self := types.ClassType(cls)
		if len(cls.TypeParams) > 0 {
			args := make([]types.Arg, len(cls.TypeParams))
			for i := range args {
				args[i] = types.Arg{V: types.Star}
			}
			self = types.GenericType(cls, args)
		}
		if !c.subtype(self, parcelArg(pt, 0)) {
			if write {
				c.unsupported(cd.P, "parcelize", "a Parceler companion object of %s, not %s, is not supported", parcelArg(pt, 0), self)
			}
			return nil, false
		}
	}
	return cls.Companion, true
}

// A parcelEntry is a @TypeParceler<T, P>: the type T and the Parceler
// object P that writes it.
type parcelEntry struct {
	t   types.Type
	obj *types.ClassSym
}

// parcelScope returns the @TypeParceler annotations among annotations
// m, of a class or a constructor property. It reports, and returns false,
// what kotlinc reports as an error, which it leaves to kotlinc: a
// Parceler that isn't an object, or one type twice.
func (c *Checker) parcelScope(m *syntax.Modifiers) ([]parcelEntry, bool) {
	var scope []parcelEntry
	for i := range m.Annotations {
		a := &m.Annotations[i]
		if a.Name != "TypeParceler" && a.Name != "kotlinx.parcelize.TypeParceler" {
			continue
		}
		if len(a.TypeArgs) != 2 || a.TypeArgs[0].Type == nil || a.TypeArgs[1].Type == nil {
			c.unsupported(a.P, "parcelize", "@TypeParceler needs a type and a Parceler")
			return nil, false
		}
		t := c.resolveType(a.TypeArgs[0].Type)
		obj, ok := c.parcelerObject(a, c.resolveType(a.TypeArgs[1].Type))
		if !ok {
			return nil, false
		}
		for _, e := range scope {
			if c.sameType(e.t, t) {
				c.unsupported(a.P, "parcelize", "@TypeParceler for %s twice", t)
				return nil, false
			}
		}
		scope = append(scope, parcelEntry{t, obj})
	}
	return scope, true
}

// parcelerObject returns the class of type p, named by annotation a, if
// it is an object that is a kotlinx.parcelize.Parceler, or reports that
// it isn't, and returns false.
func (c *Checker) parcelerObject(a *syntax.Annotation, p types.Type) (*types.ClassSym, bool) {
	pc := c.u.ext.class("kotlinx/parcelize/Parceler")
	if p.K != types.Class || pc == nil || !p.C.Object || !p.C.IsSubclassOf(pc) {
		c.unsupported(a.P, "parcelize", "the Parceler of @%s must be an object", a.Name)
		return nil, false
	}
	return p.C, true
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

// parcelBound returns the type that type parameter p erases to, as the
// plugin dispatches on it: its erased upper bound, the first bound that
// is a class rather than an interface, or else the first, with the type
// arguments that name type parameters star-projected (Comparable<*> for
// T : Comparable<T>). The reads of p's values are of that type, and the
// class's constructor is called with it as p's type argument, which
// needn't be within p's bounds in a plugin's code (state.plugin).
func parcelBound(p *types.TypeParamSym) types.Type {
	b := p.ErasureBound()
	if b.IsParam() {
		return parcelErase(b) // T : U
	}
	if b.K != types.Class || b.A == nil {
		return b
	}
	args := make([]types.Arg, b.A.Len())
	for i, a := range b.A.List {
		args[i] = a
		if a.V != types.Star && parcelHasParam(a.T) {
			args[i] = types.Arg{V: types.Star}
		}
	}
	b.A = types.MakeArgs(args)
	return b
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

// parcelNeedsArgs reports whether the plugin writes a value of type t
// by its type arguments: an array's, a sparse array's or a collection's.
// It fails to write a type parameter that erases to one, which has none.
func parcelNeedsArgs(t types.Type) bool {
	if t.K != types.Class {
		return false
	}
	for _, name := range []string{t.C.KotlinName, t.C.Name} {
		if _, ok := parcelCollections[name]; ok || name == "kotlin/Array" || name == "android/util/SparseArray" {
			return true
		}
	}
	return false
}

// parcelErase returns type t with each type parameter in it replaced by
// its bound, nullable if the parameter's value can be null, as the
// plugin dispatches on a type parameter's erased upper bound. A type
// parameter whose bound the plugin writes by its type arguments, which
// it fails to (parcelNeedsArgs), is left as it is, which kc can't write.
func parcelErase(t types.Type) types.Type {
	if t.K == types.TypeParam {
		b := parcelBound(t.Param())
		if parcelNeedsArgs(b) {
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
	// classScope and scope are the @TypeParceler annotations of the class
	// and of the property being written, which the class's win over.
	classScope, scope []parcelEntry
	// inData is set while the serializers of a data class's properties
	// are made: a data class is written by its properties in one too.
	inData bool
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
	parcelCustom                         // a Parceler object's write and create: @TypeParceler, @WriteWith
	parcelCoerce                         // the underlying value: the unsigned integers and their arrays, Duration
	parcelRange                          // the first and last values: IntRange, LongRange, CharRange
	parcelUuid                           // the two halves, with writeLong
	parcelData                           // each property: a data class annotated @DataClass, or in one
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
	// persistent is the extension that the plugin converts a collection
	// it reads with: kotlinx's toPersistentList, or another.
	persistent string
	// elem writes the elements (a map's values), and key a map's keys.
	elem, key *parcelSer
	// obj is parcelCustom's Parceler object, and x the type it writes,
	// its Parceler's type argument.
	obj *types.ClassSym
	x   types.Type
	// under writes parcelCoerce's underlying values, of type repr, and
	// parcelRange's bounds; ctor is the range's class.
	under *parcelSer
	repr  types.Type
	ctor  string
	// props are parcelData's properties, of the types fields, which
	// members write.
	props   []string
	fields  []types.Type
	members []*parcelSer
}

// parcelCollections are the classes the plugin writes as lists, sets and
// maps, by Kotlin or JVM name: the class it reads each into, whether its
// constructor takes the size, and whether a list of String or IBinder is
// written with writeStringList or writeBinderList.
var parcelCollections = map[string]struct {
	kind           parcelKind
	coll           string
	capacity, fast bool
	persistent     string // the extension that converts it, for kotlinx's immutable collections
}{
	"kotlin/collections/List":                      {parcelCollection, "java.util.ArrayList", true, true, ""},
	"kotlin/collections/MutableList":               {parcelCollection, "java.util.ArrayList", true, true, ""},
	"java/util/List":                               {parcelCollection, "java.util.ArrayList", true, true, ""},
	"java/util/ArrayList":                          {parcelCollection, "java.util.ArrayList", true, true, ""},
	"kotlin/collections/ArrayDeque":                {parcelCollection, "kotlin.collections.ArrayDeque", true, false, ""},
	"java/util/ArrayDeque":                         {parcelCollection, "java.util.ArrayDeque", true, false, ""},
	"kotlin/collections/Set":                       {parcelCollection, "java.util.LinkedHashSet", true, false, ""},
	"kotlin/collections/MutableSet":                {parcelCollection, "java.util.LinkedHashSet", true, false, ""},
	"java/util/Set":                                {parcelCollection, "java.util.LinkedHashSet", true, false, ""},
	"java/util/HashSet":                            {parcelCollection, "java.util.HashSet", true, false, ""},
	"java/util/LinkedHashSet":                      {parcelCollection, "java.util.LinkedHashSet", true, false, ""},
	"java/util/NavigableSet":                       {parcelCollection, "java.util.TreeSet", false, false, ""},
	"java/util/SortedSet":                          {parcelCollection, "java.util.TreeSet", false, false, ""},
	"kotlin/collections/Map":                       {parcelMap, "java.util.LinkedHashMap", true, false, ""},
	"kotlin/collections/MutableMap":                {parcelMap, "java.util.LinkedHashMap", true, false, ""},
	"java/util/Map":                                {parcelMap, "java.util.LinkedHashMap", true, false, ""},
	"java/util/HashMap":                            {parcelMap, "java.util.HashMap", true, false, ""},
	"java/util/LinkedHashMap":                      {parcelMap, "java.util.LinkedHashMap", true, false, ""},
	"java/util/SortedMap":                          {parcelMap, "java.util.TreeMap", false, false, ""},
	"java/util/NavigableMap":                       {parcelMap, "java.util.TreeMap", false, false, ""},
	"java/util/TreeMap":                            {parcelMap, "java.util.TreeMap", false, false, ""},
	"java/util/concurrent/ConcurrentHashMap":       {parcelMap, "java.util.concurrent.ConcurrentHashMap", true, false, ""},
	"kotlinx/collections/immutable/ImmutableList":  {parcelCollection, "java.util.ArrayList", true, true, persistentList},
	"kotlinx/collections/immutable/PersistentList": {parcelCollection, "java.util.ArrayList", true, true, persistentList},
	"kotlinx/collections/immutable/ImmutableSet":   {parcelCollection, "java.util.LinkedHashSet", true, false, persistentSet},
	"kotlinx/collections/immutable/PersistentSet":  {parcelCollection, "java.util.LinkedHashSet", true, false, persistentSet},
	"kotlinx/collections/immutable/ImmutableMap":   {parcelMap, "java.util.LinkedHashMap", true, false, persistentMap},
	"kotlinx/collections/immutable/PersistentMap":  {parcelMap, "java.util.LinkedHashMap", true, false, persistentMap},
}

// The extensions that convert the collections the plugin reads to
// kotlinx's immutable ones, by qualified name: a plugin's code calls them
// as pluginQualified describes.
const (
	persistentList = "kotlinx.collections.immutable.toPersistentList"
	persistentSet  = "kotlinx.collections.immutable.toPersistentSet"
	persistentMap  = "kotlinx.collections.immutable.toPersistentMap"
)

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
	if s, ok := g.custom(t, ref); s != nil || !ok {
		return s
	}
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
	if s := g.special(t, name); s != nil {
		return s
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
		if key == "kotlin/Array" && !g.hasCustom(e, eref) {
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
		// A primitive array, or SparseBooleanArray, whose elements a
		// custom Parceler writes is written as any array, or sparse array.
		switch {
		case cls.Elem.K != types.Invalid && cls.Array && g.hasCustom(cls.Elem, nil):
			return g.withElem(&parcelSer{kind: parcelArray, nullAware: true}, cls.Elem, nil, raw)
		case key == "android/util/SparseBooleanArray" && g.hasCustom(types.BoolType, nil):
			return g.withElem(&parcelSer{kind: parcelSparse, nullAware: true, coll: "android.util.SparseBooleanArray"}, types.BoolType, nil, raw)
		}
		s.kind = parcelSimple
		return &s
	}
	if c, ok := parcelCollections[key]; ok {
		if c.kind == parcelMap {
			k, v := parcelArg(t, 0), parcelArg(t, 1)
			s := &parcelSer{kind: parcelMap, nullAware: true, coll: c.coll + "<" + g.typeText(k) + ", " + g.typeText(v) + ">", capacity: c.capacity, persistent: c.persistent}
			if s.key = g.serializer(k, parcelArgRef(ref, t, 0), raw); s.key == nil {
				return nil
			}
			return g.withElem(s, v, parcelArgRef(ref, t, 1), raw)
		}
		e := parcelArg(t, 0)
		if c.fast && !g.hasCustom(e, parcelArgRef(ref, t, 0)) {
			// An immutable list's is converted, after a null marker.
			marker := c.persistent != ""
			switch en := e.NonNull(); {
			case en.K == types.String:
				return &parcelSer{kind: parcelSimple, write: "writeStringList", read: "createStringArrayList", nullAware: marker, persistent: c.persistent}
			case isClass(en, "android/os/IBinder"):
				return &parcelSer{kind: parcelSimple, write: "writeBinderList", read: "createBinderArrayList", nullAware: marker, persistent: c.persistent}
			}
		}
		return g.withElem(&parcelSer{kind: parcelCollection, nullAware: true, coll: c.coll + "<" + g.typeText(e) + ">", capacity: c.capacity, persistent: c.persistent}, e, parcelArgRef(ref, t, 0), raw)
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
	if cls.Data && (g.inData || ref != nil && parcelAnnotation(ref, "DataClass") != nil) {
		return g.data(t)
	}
	if s := g.c.u.ext.class("java/io/Serializable"); s != nil && cls.IsSubclassOf(s) {
		ser := &parcelSer{kind: parcelSimple, write: "writeSerializable", read: "readSerializable", cast: true}
		if cls.Value {
			ser.wcast = "java.io.Serializable" // the box, which kotlinc casts
		}
		return ser
	}
	return g.value(t, raw)
}

// special returns the serializer of a type of the standard library that
// the plugin writes in a way of its own, of class name, or nil: an
// unsigned integer, or an array of them, and Duration, as the underlying
// value, which it unsafely coerces to; a range, as its bounds; a Uuid, as
// its halves.
func (g *parcelGen) special(t types.Type, name string) *parcelSer {
	coerce := func(under *parcelSer, repr types.Type, marker bool) *parcelSer {
		return &parcelSer{kind: parcelCoerce, nullAware: marker, under: under, repr: repr}
	}
	prim := &parcelSer{kind: parcelPrimitive, nullAware: true}
	simple := func(write, read string) *parcelSer {
		return &parcelSer{kind: parcelSimple, write: write, read: read}
	}
	array := types.ArrayOf
	switch name {
	case "kotlin/UInt":
		return coerce(prim, types.IntType, true)
	case "kotlin/ULong", "kotlin/time/Duration":
		return coerce(prim, types.LongType, true)
	case "kotlin/UByte":
		return coerce(prim, types.ByteType, true)
	case "kotlin/UShort":
		return coerce(prim, types.ShortType, true)
	case "kotlin/UIntArray":
		return coerce(simple("writeIntArray", "createIntArray"), array(types.IntType), false)
	case "kotlin/ULongArray":
		return coerce(simple("writeLongArray", "createLongArray"), array(types.LongType), false)
	case "kotlin/UByteArray":
		return coerce(simple("writeByteArray", "createByteArray"), array(types.ByteType), false)
	case "kotlin/UShortArray":
		return coerce(&parcelSer{kind: parcelArray, nullAware: true, elem: prim}, array(types.ShortType), true)
	case "kotlin/ranges/IntRange":
		return &parcelSer{kind: parcelRange, nullAware: true, under: prim, repr: types.IntType, ctor: "kotlin.ranges.IntRange"}
	case "kotlin/ranges/LongRange":
		return &parcelSer{kind: parcelRange, nullAware: true, under: prim, repr: types.LongType, ctor: "kotlin.ranges.LongRange"}
	case "kotlin/ranges/CharRange":
		return &parcelSer{kind: parcelRange, nullAware: true, under: prim, repr: types.CharType, ctor: "kotlin.ranges.CharRange"}
	case "kotlin/uuid/Uuid":
		return &parcelSer{kind: parcelUuid, nullAware: true}
	}
	return nil
}

// data returns the serializer of data class type t, which the plugin
// writes by the properties of its primary constructor, in order, and reads
// with it: their serializers are those of a data class's properties too.
// kc writes only the program's data classes, whose constructor and
// properties the class can see: kotlinc reports an error for others of
// the program, and crashes on another module's, whose properties its
// plugin doesn't find.
func (g *parcelGen) data(t types.Type) *parcelSer {
	cd, ok := t.C.Decl.(*syntax.ClassDecl)
	if !ok || cd.CtorMods.Visibility == syntax.Private || cd.CtorMods.Visibility == syntax.Protected {
		g.fail(t)
		return nil
	}
	s := &parcelSer{kind: parcelData, nullAware: true}
	sub := g.c.memberSubst(t, t.C)
	in := g.inData
	g.inData = true
	defer func() { g.inData = in }()
	for _, p := range cd.CtorParams {
		if !p.IsProp {
			continue
		}
		if p.Visibility == syntax.Private || p.Visibility == syntax.Protected || p.PropSym == nil {
			g.fail(t)
			return nil
		}
		ft := sub.Apply(p.PropSym.Type)
		m := g.serializer(ft, p.Type, false)
		if m == nil {
			return nil
		}
		s.props = append(s.props, p.Name)
		s.fields = append(s.fields, ft)
		s.members = append(s.members, m)
	}
	return s
}

// custom returns the serializer of type t, written as ref, if a custom
// Parceler writes it, as kotlinc's getCustomSerializer finds one: one that
// @WriteWith names on the type; or else the class's or the property's
// @TypeParceler for the type, the class's first; or else, for a nullable
// type, one for the type non-null, after a null marker. It returns false
// if it reported that kc can't write t.
func (g *parcelGen) custom(t types.Type, ref *syntax.TypeRef) (*parcelSer, bool) {
	var obj *types.ClassSym
	wrap := false
	if a := parcelAnnotationOf(ref, "WriteWith"); a != nil {
		if len(a.TypeArgs) != 1 || a.TypeArgs[0].Type == nil {
			g.fail(t)
			return nil, false
		}
		var ok bool
		if obj, ok = g.c.parcelerObject(a, g.c.resolveType(a.TypeArgs[0].Type)); !ok {
			g.fail(t)
			return nil, false
		}
		wrap = t.N && !types.MayBeNull(g.parcelerType(obj))
	} else if obj = g.lookup(t); obj == nil && t.N {
		obj, wrap = g.lookup(t.NonNull()), true
	}
	if obj == nil {
		return nil, true
	}
	x := g.parcelerType(obj)
	v := t
	if wrap {
		v = t.NonNull()
	}
	if !g.c.subtype(v, x) {
		// kotlinc reports that the Parceler's type is incompatible.
		g.fail(t)
		return nil, false
	}
	return &parcelSer{kind: parcelCustom, nullAware: wrap, obj: obj, x: x}, true
}

// parcelAnnotationOf is parcelAnnotation for a type ref that may be nil.
func parcelAnnotationOf(ref *syntax.TypeRef, name string) *syntax.Annotation {
	if ref == nil {
		return nil
	}
	return parcelAnnotation(ref, name)
}

// lookup returns the Parceler object of the @TypeParceler for type t, of
// the class's or else the property's, or nil.
func (g *parcelGen) lookup(t types.Type) *types.ClassSym {
	for _, scope := range [][]parcelEntry{g.classScope, g.scope} {
		for _, e := range scope {
			if g.c.sameType(e.t, t) {
				return e.obj
			}
		}
	}
	return nil
}

// hasCustom reports whether a custom Parceler writes elements of type e,
// written as ref, which keeps the plugin from writing their array or list
// with a Parcel method of its own.
func (g *parcelGen) hasCustom(e types.Type, ref *syntax.TypeRef) bool {
	return parcelAnnotationOf(ref, "WriteWith") != nil || g.lookup(e) != nil || e.N && g.lookup(e.NonNull()) != nil
}

// parcelerType returns the type that Parceler object obj writes: its
// Parceler's type argument.
func (g *parcelGen) parcelerType(obj *types.ClassSym) types.Type {
	if pt, ok := g.c.asSuper(types.ClassType(obj), g.c.u.ext.class("kotlinx/parcelize/Parceler")); ok {
		return parcelArg(pt, 0)
	}
	return types.AnyType.Nullable()
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
	case parcelCustom:
		// Parceler's write is a member extension, which only an object
		// in scope as a receiver can call.
		g.stmt("kotlin.with(%s) { %s.write(dest, flags) }", fqName(s.obj), v)
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
	case parcelCoerce:
		u := g.fresh("u")
		repr := s.repr
		if t.N {
			repr = repr.Nullable() // an unsigned array without a marker
		}
		g.stmt("val %s = `kc$coerce`<%s>(%s)", u, g.typeText(repr), v)
		g.write(s.under, repr, u)
	case parcelRange:
		g.write(s.under, s.repr, v+".first")
		g.write(s.under, s.repr, v+".endInclusive")
	case parcelData:
		for i, p := range s.props {
			m := g.fresh("m")
			g.stmt("val %s = %s.`%s`", m, v, p)
			g.write(s.members[i], s.fields[i], m)
		}
	case parcelUuid:
		a, b := g.fresh("a"), g.fresh("b")
		g.stmt("%s.toLongs { %s, %s ->", v, a, b)
		g.stmt("dest.writeLong(%s)", a)
		g.stmt("dest.writeLong(%s)", b)
		g.stmt("}")
	case parcelMap:
		g.stmt("dest.writeInt(%s.size)", v)
		e, key, val := g.fresh("e"), g.fresh("k"), g.fresh("w")
		g.stmt("for (%s in %s.entries) {", e, v)
		g.stmt("val %s = %s.key", key, e)
		g.write(s.key, parcelArg(t, 0), key)
		g.stmt("val %s = %s.value", val, e)
		g.write(s.elem, parcelArg(t, 1), val)
		g.stmt("}")
	}
}

// readAll reads n values, value i, of type t, as read(i) returns, and
// returns the expressions that are them, which a call takes as its
// arguments: locals that hold them if one takes statements to read, as
// kotlinc's code then reads each before the next, and else the
// expressions read. A value class's value is held unboxed.
func (g *parcelGen) readAll(n int, read func(i int) (x string, t types.Type)) []string {
	locals, xs := make([]string, n), make([]string, n)
	mark, plain := g.body.Len(), true
	for i := range n {
		locals[i] = g.fresh("a")
		before := g.body.Len()
		x, t := read(i)
		plain = plain && g.body.Len() == before
		xs[i] = x
		if types.IsValue(t) {
			g.stmt("val %s: %s = %s", locals[i], g.typeText(t), x)
		} else {
			g.stmt("val %s = %s", locals[i], x)
		}
	}
	if plain {
		g.body.Truncate(mark)
		return xs
	}
	return locals
}

// sparseElem returns the type of the values of a sparse array of type t.
func (g *parcelGen) sparseElem(t types.Type) types.Type {
	switch t.C.Name {
	case "android/util/SparseIntArray":
		return types.IntType
	case "android/util/SparseLongArray":
		return types.LongType
	case "android/util/SparseBooleanArray":
		return types.BoolType
	}
	return parcelArg(t, 0)
}

// read writes the statements that read a value of type t with serializer
// s, and returns the expression that is the value then.
func (g *parcelGen) read(s *parcelSer, t types.Type) string {
	if s.persistent != "" && !(t.N && s.nullAware) {
		// Converted, as kotlinc's ConvertPersistent serializer does.
		c := *s
		c.persistent = ""
		return fmt.Sprintf("%s.`%s`()", g.read(&c, t), s.persistent)
	}
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
	case parcelCustom:
		x := fmt.Sprintf("%s.create(parcel)", fqName(s.obj))
		if !g.c.subtype(s.x, t) {
			return fmt.Sprintf("(%s as %s)", x, g.typeText(t))
		}
		return x
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
		case t.C != types.ArrayClass:
			g.stmt("val %s = %s(%s)", a, fqName(t.C), n) // a primitive array
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
	case parcelCoerce:
		repr := s.repr
		if t.N {
			repr = repr.Nullable()
		}
		return fmt.Sprintf("`kc$coerce`<%s>(%s)", g.typeText(t), g.read(s.under, repr))
	case parcelRange:
		return fmt.Sprintf("%s(%s, %s)", s.ctor, g.read(s.under, s.repr), g.read(s.under, s.repr))
	case parcelUuid:
		return "kotlin.uuid.Uuid.fromLongs(parcel.readLong(), parcel.readLong())"
	case parcelData:
		xs := g.readAll(len(s.props), func(i int) (string, types.Type) {
			return g.read(s.members[i], s.fields[i]), s.fields[i]
		})
		return fmt.Sprintf("%s(%s)", g.typeText(t.NonNull()), strings.Join(xs, ", "))
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
