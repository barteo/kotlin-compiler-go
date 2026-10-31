package javasrc

import (
	"sort"
	"strings"
	"unicode"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// Set is the classes of a module's Java sources, which name each other,
// the class path's classes and the module's Kotlin classes.
type Set struct {
	classes map[string]*Class // by internal name
	// Outside are the classes the sources name that aren't theirs: the
	// class path's and the module's Kotlin classes. It is set before Info
	// is called.
	Outside Classes
}

// Classes looks up classes by internal name.
type Classes interface {
	// Has reports whether a class exists.
	Has(name string) bool
	// Supers returns a class's superclass and interfaces, and reports
	// whether it is an interface. A class it doesn't know has none.
	Supers(name string) (supers []string, iface bool)
	// Constant returns the ConstantValue of a class's field and its
	// descriptor, or "" if the class has no such field.
	Constant(class, field string) (value any, desc string)
}

// NewSet indexes the classes of files, nested ones too.
func NewSet(files []*File) *Set {
	s := &Set{classes: map[string]*Class{}}
	var add func(prefix string, c *Class)
	add = func(prefix string, c *Class) {
		name := prefix + c.Name
		if _, dup := s.classes[name]; !dup {
			s.classes[name] = c
		}
		for _, n := range c.Nested {
			add(name+"$", n)
		}
	}
	for _, f := range files {
		prefix := ""
		if f.Package != "" {
			prefix = strings.ReplaceAll(f.Package, ".", "/") + "/"
		}
		for _, c := range f.Classes {
			add(prefix, c)
		}
	}
	return s
}

// Names returns the internal names of the classes, sorted.
func (s *Set) Names() []string {
	names := make([]string, 0, len(s.classes))
	for n := range s.classes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Has reports whether a class is one of the Java sources', by internal
// name.
func (s *Set) Has(name string) bool { return s.classes[name] != nil }

func (s *Set) isInterface(name string) bool {
	if c := s.classes[name]; c != nil {
		return c.Kind == KindInterface || c.Kind == KindAnnotation
	}
	if s.Outside == nil {
		return false
	}
	_, iface := s.Outside.Supers(name)
	return iface
}

func (s *Set) known(name string) bool {
	return s.classes[name] != nil || s.Outside != nil && s.Outside.Has(name)
}

// internalName returns a class's internal name.
func internalName(c *Class) string {
	if c.Outer != nil {
		return internalName(c.Outer) + "$" + c.Name
	}
	if c.file.Package == "" {
		return c.Name
	}
	return strings.ReplaceAll(c.file.Package, ".", "/") + "/" + c.Name
}

// static reports whether a nested class has no outer instance: declared
// static, or an interface, enum, record or annotation, or a member of an
// interface.
func static(c *Class) bool {
	return c.Outer == nil || c.Flags&classfile.AccStatic != 0 || c.Kind != KindClass ||
		c.Outer.Kind == KindInterface || c.Outer.Kind == KindAnnotation
}

// resolver resolves the names of the types used in a class, or in one of
// its methods, with the method's type parameters.
type resolver struct {
	s       *Set
	c       *Class
	mparams []*TypeParam
	// noInherit is set while resolving a top-level class's supertypes,
	// which the member types it inherits from them can't name.
	noInherit bool
}

// lookup resolves a simple type name to a class's internal name, or to a
// type variable.
func (r *resolver) lookup(name string) (string, *TypeParam) {
	for _, tp := range r.mparams {
		if tp.Name == name {
			return "", tp
		}
	}
	for k := r.c; k != nil; k = k.Outer {
		if k.Name == name {
			return internalName(k), nil
		}
		for _, n := range k.Nested {
			if n.Name == name {
				return internalName(n), nil
			}
		}
		for _, tp := range k.TypeParams {
			if tp.Name == name {
				return "", tp
			}
		}
		if !r.noInherit {
			if n := r.inherited(k, name, 0); n != "" {
				return n, nil
			}
		}
	}
	f := r.c.file
	for _, imp := range f.Imports {
		if !imp.Static && !imp.Star && (imp.Name == name || strings.HasSuffix(imp.Name, "."+name)) {
			return r.s.qualified(imp.Name), nil
		}
	}
	pkg := ""
	if f.Package != "" {
		pkg = strings.ReplaceAll(f.Package, ".", "/") + "/"
	}
	if r.s.known(pkg + name) {
		return pkg + name, nil
	}
	for _, imp := range f.Imports {
		if !imp.Star {
			continue
		}
		// import a.b.* names a package's classes, and import a.b.C.* and
		// import static a.b.C.* the nested classes of C.
		if p := strings.ReplaceAll(imp.Name, ".", "/") + "/" + name; !imp.Static && r.s.known(p) {
			return p, nil
		}
		if n := r.s.memberType(r.s.qualified(imp.Name), name, 0); n != "" {
			return n, nil
		}
	}
	for _, imp := range f.Imports {
		if imp.Static && !imp.Star && strings.HasSuffix(imp.Name, "."+name) {
			if n := r.s.qualified(imp.Name); r.s.known(n) {
				return n, nil
			}
		}
	}
	if r.s.known("java/lang/" + name) {
		return "java/lang/" + name, nil
	}
	return "", nil
}

// inherited returns the internal name of a member type that class k
// inherits from its supertypes, or "".
func (r *resolver) inherited(k *Class, name string, depth int) string {
	if depth > 16 {
		return ""
	}
	sr := superResolver(r.s, k)
	for _, t := range append(append([]*Type{}, k.Extends...), k.Implements...) {
		super, _ := sr.classType(t)
		if super == "" {
			continue
		}
		if n := r.s.memberType(super, name, depth+1); n != "" {
			return n
		}
	}
	return ""
}

// memberType returns the internal name of a member type of a class,
// declared or inherited, or "".
func (s *Set) memberType(class, name string, depth int) string {
	if depth > 16 {
		return ""
	}
	if n := class + "$" + name; s.known(n) {
		return n
	}
	if c := s.classes[class]; c != nil {
		return (&resolver{s: s, c: c}).inherited(c, name, depth)
	}
	if s.Outside == nil {
		return ""
	}
	supers, _ := s.Outside.Supers(class)
	for _, super := range supers {
		if n := s.memberType(super, name, depth+1); n != "" {
			return n
		}
	}
	return ""
}

// qualified returns the internal name of a class by its dotted name: the
// longest package prefix a class exists in, or, if none does, the
// segments after the first capitalized one are nested classes.
func (s *Set) qualified(name string) string {
	segs := strings.Split(name, ".")
	join := func(k int) string {
		return strings.Join(append([]string{strings.Join(segs[:k], "/")}, strings.Join(segs[k:], "$")), "/")
	}
	for k := len(segs) - 1; k >= 1; k-- {
		if n := join(k); s.known(n) {
			return n
		}
	}
	if s.known(strings.Join(segs, "$")) {
		return strings.Join(segs, "$")
	}
	for k, seg := range segs {
		if r, _ := utf8First(seg); unicode.IsUpper(r) {
			if k == 0 {
				return strings.Join(segs, "$")
			}
			return join(k)
		}
	}
	return strings.Join(segs, "/")
}

func utf8First(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

// classType resolves a class type's name, or returns the type variable
// it names.
func (r *resolver) classType(t *Type) (string, *TypeParam) {
	n, tp := r.lookup(t.Segs[0].Name)
	if tp != nil {
		return "", tp
	}
	if n != "" {
		for _, seg := range t.Segs[1:] {
			if m := r.s.memberType(n, seg.Name, 0); m != "" {
				n = m
			} else {
				n += "$" + seg.Name
			}
		}
		return n, nil
	}
	names := make([]string, len(t.Segs))
	for i, seg := range t.Segs {
		names[i] = seg.Name
	}
	if len(names) == 1 {
		// Unknown: in the same package, as javac would complain.
		pkg := ""
		if r.c.file.Package != "" {
			pkg = strings.ReplaceAll(r.c.file.Package, ".", "/") + "/"
		}
		return pkg + names[0], nil
	}
	return r.s.qualified(strings.Join(names, ".")), nil
}

var primDesc = map[string]string{
	"boolean": "Z", "byte": "B", "char": "C", "short": "S", "int": "I", "long": "J",
	"float": "F", "double": "D", "void": "V",
}

// desc returns a type's descriptor: a type variable's is its first bound's.
func (r *resolver) desc(t *Type) string {
	return strings.Repeat("[", t.Dims) + r.erasure(t, 0)
}

func (r *resolver) erasure(t *Type, depth int) string {
	if t.Prim != "" {
		return primDesc[t.Prim]
	}
	n, tp := r.classType(t)
	if tp != nil {
		if len(tp.Bounds) == 0 || depth > 8 {
			return "Ljava/lang/Object;"
		}
		b := tp.Bounds[0]
		return strings.Repeat("[", b.Dims) + r.erasure(b, depth+1)
	}
	return "L" + n + ";"
}

// generic reports whether a type needs a generic signature: it has type
// arguments or is a type variable.
func (r *resolver) generic(t *Type) bool {
	if t.Prim != "" {
		return false
	}
	for _, seg := range t.Segs {
		if len(seg.Args) > 0 {
			return true
		}
	}
	_, tp := r.classType(t)
	return tp != nil
}

// sig returns a type's generic signature. Of a nested class's segments,
// only the last one's type arguments are kept.
func (r *resolver) sig(t *Type) string {
	var b strings.Builder
	b.WriteString(strings.Repeat("[", t.Dims))
	if t.Prim != "" {
		b.WriteString(primDesc[t.Prim])
		return b.String()
	}
	n, tp := r.classType(t)
	if tp != nil {
		b.WriteString("T" + tp.Name + ";")
		return b.String()
	}
	b.WriteString("L" + n)
	if args := t.Segs[len(t.Segs)-1].Args; len(args) > 0 {
		b.WriteByte('<')
		for _, a := range args {
			switch a.Wild {
			case '*':
				b.WriteByte('*')
				continue
			case '+', '-':
				b.WriteByte(a.Wild)
			}
			b.WriteString(r.sig(a.Type))
		}
		b.WriteByte('>')
	}
	b.WriteByte(';')
	return b.String()
}

func (r *resolver) boundInterface(t *Type) bool {
	if t.Prim != "" || t.Dims > 0 {
		return false
	}
	n, tp := r.classType(t)
	return tp == nil && r.s.isInterface(n)
}

func (r *resolver) typeParamsSig(tps []*TypeParam) string {
	if len(tps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('<')
	for _, tp := range tps {
		b.WriteString(tp.Name + ":")
		if len(tp.Bounds) == 0 {
			b.WriteString("Ljava/lang/Object;")
		}
		for i, bound := range tp.Bounds {
			// An interface bound follows the class bound's place.
			if i > 0 || i == 0 && r.boundInterface(bound) {
				b.WriteByte(':')
			}
			b.WriteString(r.sig(bound))
		}
	}
	b.WriteByte('>')
	return b.String()
}

// Info returns the class file view of a class, by internal name.
func (s *Set) Info(name string) (*classfile.Info, error) {
	c := s.classes[name]
	if c == nil {
		return nil, &missingError{name}
	}
	r := &resolver{s: s, c: c}
	info := &classfile.Info{Name: name, StabilityInferred: -1}
	// A nested class's access is public if it is protected, and package
	// if it is private: the InnerClasses attribute has its own.
	flags := c.Flags & (classfile.AccPublic | classfile.AccFinal | classfile.AccAbstract)
	if c.Outer != nil && (c.Outer.Kind == KindInterface || c.Outer.Kind == KindAnnotation || c.Flags&classfile.AccProtected != 0) {
		flags |= classfile.AccPublic
	}
	var supers []*Type
	switch c.Kind {
	case KindClass:
		if name != "java/lang/Object" {
			info.Super = "java/lang/Object"
		}
		if len(c.Extends) > 0 {
			info.Super, _ = r.superType(c.Extends[0])
		}
		supers = c.Implements
	case KindInterface:
		flags |= classfile.AccInterface | classfile.AccAbstract
		info.Super = "java/lang/Object"
		supers = c.Extends
	case KindAnnotation:
		flags |= classfile.AccInterface | classfile.AccAbstract | classfile.AccAnnotation
		info.Super = "java/lang/Object"
		info.Interfaces = []string{"java/lang/annotation/Annotation"}
	case KindEnum:
		flags |= classfile.AccEnum | classfile.AccFinal
		info.Super = "java/lang/Enum"
		supers = c.Implements
	case KindRecord:
		flags |= classfile.AccFinal
		info.Super = "java/lang/Record"
		supers = c.Implements
	}
	if c.Kind == KindEnum && c.ConstantBodies {
		flags &^= classfile.AccFinal
		for _, m := range c.Methods {
			if m.Flags&classfile.AccAbstract != 0 {
				flags |= classfile.AccAbstract
			}
		}
	}
	info.Access = flags | classfile.AccSuper
	if c.Kind == KindInterface || c.Kind == KindAnnotation {
		info.Access &^= classfile.AccSuper
	}
	for _, t := range supers {
		n, _ := r.superType(t)
		info.Interfaces = append(info.Interfaces, n)
	}
	if c.Outer != nil {
		access := c.Flags
		if static(c) {
			access |= classfile.AccStatic
		}
		if c.Outer.Kind == KindInterface || c.Outer.Kind == KindAnnotation {
			access |= classfile.AccPublic
		}
		info.Inner = &classfile.InnerClass{Outer: internalName(c.Outer), Access: access}
	}

	// The class's signature, if it is generic or has generic supertypes,
	// as an enum's Enum<E> is.
	generic := len(c.TypeParams) > 0 || c.Kind == KindEnum
	for _, t := range append(append([]*Type{}, c.Extends...), c.Implements...) {
		generic = generic || r.generic(t)
	}
	if generic {
		var b strings.Builder
		b.WriteString(r.typeParamsSig(c.TypeParams))
		switch {
		case c.Kind == KindClass && len(c.Extends) > 0:
			b.WriteString(r.sig(c.Extends[0]))
		case c.Kind == KindEnum:
			b.WriteString("Ljava/lang/Enum<L" + name + ";>;")
		case c.Kind == KindRecord:
			b.WriteString("Ljava/lang/Record;")
		default:
			b.WriteString("Ljava/lang/Object;")
		}
		for _, t := range supers {
			b.WriteString(r.sig(t))
		}
		info.Sig = b.String()
	}

	for _, a := range c.Annotations {
		switch a.Name {
		case "Retention", "java.lang.annotation.Retention":
			for _, arg := range a.Args {
				if arg == "RUNTIME" || arg == "CLASS" || arg == "SOURCE" {
					info.Retention = arg
				}
			}
		case "Target", "java.lang.annotation.Target":
			for _, arg := range a.Args {
				if arg != "ElementType" && strings.ToUpper(arg) == arg {
					info.JavaTargets = append(info.JavaTargets, arg)
				}
			}
		}
	}

	iface := c.Kind == KindInterface || c.Kind == KindAnnotation
	for _, f := range c.Fields {
		access := f.Flags
		if iface {
			access |= classfile.AccPublic | classfile.AccStatic | classfile.AccFinal
		}
		m := classfile.Member{Access: access, Name: f.Name, Desc: r.desc(f.Type)}
		if r.generic(f.Type) {
			m.Sig = r.sig(f.Type)
		}
		if access&classfile.AccStatic != 0 && access&classfile.AccFinal != 0 && f.Init != nil {
			m.Constant = s.constant(c, f, nil)
		}
		info.Fields = append(info.Fields, m)
	}
	for _, k := range c.Constants {
		info.Fields = append(info.Fields, classfile.Member{Access: classfile.AccPublic | classfile.AccStatic | classfile.AccFinal | classfile.AccEnum,
			Name: k, Desc: "L" + name + ";"})
	}
	for _, p := range c.Components {
		m := classfile.Member{Access: classfile.AccPrivate | classfile.AccFinal, Name: p.Name, Desc: r.desc(p.Type)}
		if r.generic(p.Type) {
			m.Sig = r.sig(p.Type)
		}
		info.Fields = append(info.Fields, m)
	}

	hasCtor := false
	declared := map[string]bool{}
	for _, m := range c.Methods {
		mr := &resolver{s: s, c: c, mparams: m.TypeParams}
		access := m.Flags
		if m.Varargs {
			access |= classfile.AccVarargs
		}
		if iface && m.Ret != nil {
			if access&classfile.AccPrivate == 0 {
				access |= classfile.AccPublic
			}
			if !m.Body && access&classfile.AccStatic == 0 && !m.Default {
				access |= classfile.AccAbstract
			}
		}
		var params []*Type
		var outer *Type
		if m.Ret == nil {
			hasCtor = true
			if c.Kind == KindEnum {
				access = access&^(classfile.AccPublic|classfile.AccProtected) | classfile.AccPrivate
			}
			outer = outerParam(c)
		}
		for _, p := range m.Params {
			params = append(params, p.Type)
		}
		ret := m.Ret
		if ret == nil {
			ret = &Type{Prim: "void"}
		}
		desc, sig := mr.methodDesc(m.TypeParams, outer, params, ret, m.Throws)
		declared[m.Name+desc] = true
		info.Methods = append(info.Methods, classfile.Member{Access: access, Name: m.Name, Desc: desc, Sig: sig})
	}
	var canonical []*Type // a record's canonical constructor's parameters
	for _, p := range c.Components {
		canonical = append(canonical, p.Type)
	}
	if desc, _ := r.methodDesc(nil, nil, canonical, &Type{Prim: "void"}, nil); c.Kind == KindRecord && declared["<init>"+desc] {
		hasCtor = true
	} else if c.Kind == KindRecord {
		hasCtor = false
	}
	if !hasCtor && (c.Kind == KindClass || c.Kind == KindRecord) {
		// The default constructor has the class's access.
		access := c.Flags & (classfile.AccPublic | classfile.AccProtected | classfile.AccPrivate)
		if c.Outer != nil && (c.Outer.Kind == KindInterface || c.Outer.Kind == KindAnnotation) {
			access = classfile.AccPublic
		}
		var params []*Type
		if c.Kind == KindRecord {
			access, params = classfile.AccPublic, canonical
		}
		desc, sig := r.methodDesc(nil, outerParam(c), params, &Type{Prim: "void"}, nil)
		info.Methods = append(info.Methods, classfile.Member{Access: access, Name: "<init>", Desc: desc, Sig: sig})
	}
	for _, p := range c.Components {
		desc, sig := r.methodDesc(nil, nil, nil, p.Type, nil)
		if !declared[p.Name+desc] {
			info.Methods = append(info.Methods, classfile.Member{Access: classfile.AccPublic, Name: p.Name, Desc: desc, Sig: sig})
		}
	}
	if c.Kind == KindRecord {
		for _, m := range []classfile.Member{
			{Name: "toString", Desc: "()Ljava/lang/String;"},
			{Name: "hashCode", Desc: "()I"},
			{Name: "equals", Desc: "(Ljava/lang/Object;)Z"},
		} {
			if !declared[m.Name+m.Desc] {
				m.Access = classfile.AccPublic | classfile.AccFinal
				info.Methods = append(info.Methods, m)
			}
		}
	}
	if c.Kind == KindEnum {
		info.Methods = append(info.Methods,
			classfile.Member{Access: classfile.AccPublic | classfile.AccStatic, Name: "values", Desc: "()[L" + name + ";"},
			classfile.Member{Access: classfile.AccPublic | classfile.AccStatic, Name: "valueOf", Desc: "(Ljava/lang/String;)L" + name + ";"})
	}
	return info, nil
}

// superResolver resolves the names in class k's supertypes: in the scope
// of its outer class, or of its file, where the member types k inherits
// aren't.
func superResolver(s *Set, k *Class) *resolver {
	if k.Outer != nil {
		return &resolver{s: s, c: k.Outer}
	}
	return &resolver{s: s, c: k, noInherit: true}
}

// superType resolves a supertype's name.
func (r *resolver) superType(t *Type) (string, *TypeParam) {
	n, tp := superResolver(r.s, r.c).classType(t)
	if n == "" && tp == nil {
		return "java/lang/Object", nil
	}
	return n, tp
}

// outerParam returns the type of the outer instance an inner class's
// constructors take first, or nil.
func outerParam(c *Class) *Type {
	if static(c) {
		return nil
	}
	return &Type{Segs: []Seg{{Name: c.Outer.Name}}}
}

// methodDesc returns a method's descriptor and its generic signature, or
// "" if it needs none. An inner class's constructor's outer instance is
// in the descriptor only.
func (r *resolver) methodDesc(tps []*TypeParam, outer *Type, params []*Type, ret *Type, throws []*Type) (string, string) {
	var d, s strings.Builder
	generic := len(tps) > 0
	d.WriteByte('(')
	if outer != nil {
		d.WriteString(r.desc(outer))
	}
	s.WriteString(r.typeParamsSig(tps))
	s.WriteByte('(')
	for _, p := range params {
		d.WriteString(r.desc(p))
		s.WriteString(r.sig(p))
		generic = generic || r.generic(p)
	}
	d.WriteByte(')')
	s.WriteByte(')')
	d.WriteString(r.desc(ret))
	s.WriteString(r.sig(ret))
	generic = generic || r.generic(ret)
	// The exceptions, if one is a type variable.
	for _, t := range throws {
		if _, tp := r.classType(t); tp != nil {
			generic = true
			for _, t := range throws {
				s.WriteString("^" + r.sig(t))
			}
			break
		}
	}
	if !generic {
		return d.String(), ""
	}
	return d.String(), s.String()
}

type missingError struct{ name string }

func (e *missingError) Error() string { return "class " + e.name + " not found" }
