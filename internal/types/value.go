package types

import (
	"crypto/md5"
	"encoding/base64"
	"strings"
)

// Value classes, @JvmInline value class Code(val code: Int), are on the
// JVM the value of their one property, the underlying value: a Code is an
// int. Where the value class's type isn't known, as an Any, an interface
// or a type argument, the value is boxed, an instance of the class.

// IsValue reports whether t is a value class type, nullable or not.
func IsValue(t Type) bool {
	return t.K == Class && t.C.Value && t.C.Underlying != nil
}

// Unboxed reports whether values of type t are held unboxed, as their
// underlying values: those of a value class type, and of the nullable
// type too if the underlying value is a reference that can't be null, so
// that null tells them apart.
func Unboxed(t Type) bool {
	if !IsValue(t) || t.B {
		return false
	}
	if !t.N {
		return true
	}
	r := Repr(t.C.Underlying.Type) // through value classes of value classes
	return !MayBeNull(r) && !r.IsPrimitive()
}

// Repr returns the type that represents values of type t on the JVM: the
// underlying type for an unboxed value class type, t for others.
func Repr(t Type) Type {
	if !Unboxed(t) {
		return t
	}
	r := Repr(t.C.Underlying.Type)
	if t.N {
		r = r.Nullable()
	}
	return r
}

// Boxed returns the boxed form of value class type t.
func Boxed(t Type) Type {
	t.B = true
	return t
}

// BoxedDesc returns the descriptor of the boxed form of values of type t:
// for a value class, its instances, whether t is unboxed or not.
func BoxedDesc(t Type) string {
	if IsValue(t) {
		return "L" + t.C.Name + ";"
	}
	d, _ := Desc(t.Nullable())
	return d
}

// mangledName returns the JVM name of a function of the program: as
// kotlinc names it, its name and a hash of its parameters' types if any
// is a value class other than Result, or of its result too if it is a
// member that returns one. A member of a value class without such a hash
// has the suffix -impl: it is a static method that takes the instance
// first.
func mangledName(f *FunSym) string {
	member := f.Owner != nil && !f.Static
	mangle := member && IsValue(f.Ret)
	if f.Ext && Mangles(f.Recv) {
		mangle = true
	}
	for _, p := range f.Params {
		if Mangles(p.Type) {
			mangle = true
		}
	}
	switch {
	case mangle:
		var ps []Type
		if f.Ext {
			ps = append(ps, f.Recv)
		}
		for _, p := range f.Params {
			ps = append(ps, p.Type)
		}
		if f.Suspend {
			ps = append(ps, AnyType) // the continuation
		}
		if f.Composable {
			// The Composer, the change flags and the masks of the
			// omitted arguments, as with kotlinc's Compose plugin.
			for range 1 + f.Changed + f.Defaults {
				ps = append(ps, AnyType)
			}
		}
		var ret *Type
		if member && IsValue(f.Ret) {
			ret = &f.Ret
		}
		return f.Name + "-" + Mangle(ps, ret)
	case member && f.Owner.Value:
		return f.Name + "-impl"
	}
	return f.Name
}

// Mangles reports whether a parameter of type t gives a function a
// mangled name: as with kotlinc, one of a value class other than
// kotlin.Result, whose functions keep their names.
func Mangles(t Type) bool {
	return IsValue(t) && t.C.KotlinName != "kotlin/Result"
}

// InstanceJVM returns the JVM name of the instance method of member f:
// for a value class's member, which is a static method, that of the
// method that overrides a supertype's, without -impl.
func InstanceJVM(f *FunSym) string {
	name := f.JVM()
	if f.Owner != nil && f.Owner.Value && name == f.Name+"-impl"+f.Module {
		return f.Name + f.Module
	}
	return name
}

// Mangle returns the hash that kotlinc appends to the name of a function
// whose parameters' types are params (an extension's receiver first) and
// whose result, if ret isn't nil, has a value class type. It is the first
// 5 bytes of the MD5 of the types, in base64: each value class type as
// L<qualified name>; with ? if nullable, the others as _, then the result
// after a colon.
func Mangle(params []Type, ret *Type) string {
	var sb strings.Builder
	for _, t := range params {
		sb.WriteString(mangleElem(t))
	}
	if ret != nil {
		sb.WriteByte(':')
		sb.WriteString(mangleElem(*ret))
	}
	sum := md5.Sum([]byte(sb.String()))
	s := base64.StdEncoding.EncodeToString(sum[:5])[:7]
	return strings.NewReplacer("/", "_", "+", "-").Replace(s)
}

func mangleElem(t Type) string {
	if !IsValue(t) {
		return "_"
	}
	name := t.C.KotlinName
	if name == "" {
		name = t.C.Name
	}
	s := "L" + strings.NewReplacer("/", ".", "$", ".").Replace(name)
	if t.N {
		s += "?"
	}
	return s + ";"
}

// GetterJVM returns the JVM name of the getter of property p: for a class
// property of a value class type, with a hash, and for a property of a
// value class other than its underlying one, the static method's, with
// -impl.
func GetterJVM(p *VarSym) string {
	if p.JVMGetter != "" {
		return p.JVMGetter
	}
	name := GetterName(p.Name)
	switch {
	case p.Owner != nil && IsValue(p.Type):
		name += "-" + Mangle(nil, &p.Type)
	case p.Owner != nil && p.Owner.Value && p != p.Owner.Underlying:
		name += "-impl"
	}
	return name + p.Module
}

// SetterJVM returns the JVM name of the setter of class property p.
func SetterJVM(p *VarSym) string {
	if p.JVMSetter != "" {
		return p.JVMSetter
	}
	name := SetterName(p.Name)
	if IsValue(p.Type) {
		name += "-" + Mangle([]Type{p.Type}, nil)
	}
	if p.Module == "" {
		return name + p.SetterModule
	}
	return name + p.Module
}
