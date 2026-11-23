package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// Generic signatures of Java classes, methods and fields (the Signature
// attribute, JVMS 4.7.9.1), mapped to types as Kotlin sees them: reference
// types are platform types, also in type arguments, and wildcards become
// projections (? extends T is out T, ? super T is in T, ? is *).
//
// Signatures are parsed when a class is loaded. Parsing may refer to
// other classes but must not load them, since loading runs inside
// ClassSym.Ensure and classes refer to each other.

type sigParser struct {
	t     *classTable
	s     string
	i     int
	scope []*types.TypeParamSym // method type parameters, then the class's
	ok    bool                  // false if a type kc doesn't support was seen
	// foreign, if set, is the class of the program's Java sources whose
	// signature this is, which may name a type variable of an enclosing
	// class where it isn't in scope, as a static nested interface's
	// method does T of its outer class's (javac rejects it, kotlinc keeps
	// T): such names are parameters that nothing substitutes.
	foreign  *types.ClassSym
	foreigns map[string]*types.TypeParamSym
}

// declaredVariance overrides the variance of Java type parameters that
// Kotlin maps to its own types: java.lang.Comparable is kotlin.Comparable<in T>.
var declaredVariance = map[string]types.Variance{
	"java/lang/Comparable": types.In,
}

func (p *sigParser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *sigParser) fail() {
	p.ok = false
	p.i = len(p.s)
}

func (p *sigParser) expect(b byte) {
	if p.peek() != b {
		p.fail()
		return
	}
	p.i++
}

// ident reads an identifier ending at one of the given delimiters.
func (p *sigParser) ident(delims string) string {
	start := p.i
	for p.i < len(p.s) {
		for j := 0; j < len(delims); j++ {
			if p.s[p.i] == delims[j] {
				return p.s[start:p.i]
			}
		}
		p.i++
	}
	p.fail()
	return ""
}

// sourceOwner reports whether a type parameter's owner is, or is a member
// of, a class of a check's Java sources, which doesn't live as long as the
// library.
func sourceOwner(owner any) bool {
	switch o := owner.(type) {
	case *types.ClassSym:
		return o.Source
	case *types.FunSym:
		return o.Owner != nil && o.Owner.Source
	}
	return false
}

// typeParams parses <T:bound;U::bound;> and returns the parameters. The
// parameters are in scope in their own bounds. annots are the descriptors
// of the type annotations on their bounds, by parameter
// (classfile.Info.BoundAnnotations).
func (p *sigParser) typeParams(owner any, annots [][]string) []*types.TypeParamSym {
	if p.peek() != '<' {
		return nil
	}
	p.i++
	// Parameters are declared before their bounds are parsed, so bounds
	// such as E extends Enum<E> can refer to them.
	start := p.i
	var ps []*types.TypeParamSym
	for p.ok && p.peek() != '>' {
		ps = append(ps, &types.TypeParamSym{Name: p.ident(":"), Index: len(ps), Owner: owner, Library: !sourceOwner(owner)})
		for p.ok && p.peek() == ':' {
			p.i++
			if p.isTypeStart() {
				p.skipRef()
			}
		}
	}
	p.i = start
	p.scope = append(ps[:len(ps):len(ps)], p.scope...)
	for i, tp := range ps {
		p.ident(":")
		n := 0 // bounds, Object too
		for p.ok && p.peek() == ':' {
			p.i++
			if !p.isTypeStart() {
				continue
			}
			n++
			b := p.refType()
			if b.K == types.Any || b.K == types.Invalid {
				continue // Object, which is no bound in Kotlin
			}
			tp.Bounds = append(tp.Bounds, b)
		}
		if n == 1 && i < len(annots) && p.t.javaTypes && notNullBound(annots[i], tp) {
			// T extends @NotNull Object is T : Any. Of several bounds, K2
			// doesn't treat a not-null one alike for every argument.
			tp.JavaNotNull = true
			if len(tp.Bounds) == 0 {
				tp.Bounds = []types.Type{types.AnyType}
			} else {
				tp.Bounds[0] = tp.Bounds[0].NonNull()
			}
		}
	}
	p.expect('>')
	return ps
}

// notNullBound reports whether annotations as, of the only bound of Java
// type parameter tp, make it non-null: a not-null one and no nullable
// one, on a bound that is a class type, not a type parameter.
func notNullBound(as []string, tp *types.TypeParamSym) bool {
	if len(tp.Bounds) > 0 && tp.Bounds[0].K != types.Class {
		return false
	}
	notNull, nullable := false, false
	for _, a := range as {
		if nn, ok := javaNullability[a]; ok {
			notNull, nullable = notNull || nn, nullable || !nn
		}
	}
	return notNull && !nullable
}

// javaParamType returns the type of a use of Java type parameter tp in a
// signature: a platform type, unless its bound is annotated not-null.
func javaParamType(tp *types.TypeParamSym) types.Type {
	if tp.JavaNotNull {
		return types.ParamType(tp)
	}
	return types.ParamType(tp).Platform()
}

// isTypeStart reports whether a bound follows a ':' in type parameters.
// A class bound may be empty when interface bounds follow.
func (p *sigParser) isTypeStart() bool {
	switch p.peek() {
	case 'L', 'T', '[':
		return true
	}
	return false
}

// skipRef skips a reference type without resolving it.
func (p *sigParser) skipRef() {
	switch p.peek() {
	case 'T':
		p.ident(";")
		p.expect(';')
	case '[':
		p.i++
		if p.isTypeStart() {
			p.skipRef()
		} else {
			p.i++
		}
	case 'L':
		depth := 0
		for ; p.i < len(p.s); p.i++ {
			switch p.s[p.i] {
			case '<':
				depth++
			case '>':
				depth--
			case ';':
				if depth == 0 {
					p.i++
					return
				}
			}
		}
		p.fail()
	default:
		p.fail()
	}
}

// javaType parses any type, including primitives.
func (p *sigParser) javaType() types.Type {
	c := p.peek()
	p.i++
	if k := types.DescKind(c); k != types.Invalid {
		return types.Type{K: k} // a primitive, or V
	}
	p.i--
	return p.refType()
}

// refType parses a class type, type variable or array type.
func (p *sigParser) refType() types.Type {
	switch p.peek() {
	case 'T':
		p.i++
		name := p.ident(";")
		p.expect(';')
		for _, tp := range p.scope {
			if tp.Name == name {
				return javaParamType(tp)
			}
		}
		if p.foreign != nil {
			tp := p.foreigns[name]
			if tp == nil {
				tp = &types.TypeParamSym{Name: name, Owner: p.foreign}
				if p.foreigns == nil {
					p.foreigns = map[string]*types.TypeParamSym{}
				}
				p.foreigns[name] = tp
			}
			return types.ParamType(tp).Platform()
		}
		p.ok = false // a type variable of an enclosing class
		return types.InvalidType
	case '[':
		p.i++
		elem := p.javaType()
		if !p.ok || elem.K == types.Unit {
			p.ok = false
			return types.InvalidType
		}
		return types.ArrayOf(elem).Platform()
	case 'L':
		p.i++
		name := p.ident("<;.")
		var args []types.Arg
		if p.peek() == '<' {
			args = p.typeArgs()
		}
		for p.ok && p.peek() == '.' { // an inner class of a generic class
			p.i++
			name += "$" + p.ident("<;.")
			var own []types.Arg
			if p.peek() == '<' {
				own = p.typeArgs()
			}
			// Its own arguments, then its outer classes', as its type
			// parameters are under Library.JavaTypes.
			args = append(own, args...)
			if !p.t.javaTypes {
				p.ok = false
			}
		}
		p.expect(';')
		if !p.ok {
			return types.InvalidType
		}
		return p.t.classType(name, args)
	}
	p.fail()
	return types.InvalidType
}

func (p *sigParser) typeArgs() []types.Arg {
	p.expect('<')
	var args []types.Arg
	for p.ok && p.peek() != '>' {
		switch p.peek() {
		case '*':
			p.i++
			args = append(args, types.Arg{V: types.Star})
		case '+':
			p.i++
			args = append(args, types.Arg{V: types.Out, T: p.refType()})
		case '-':
			p.i++
			args = append(args, types.Arg{V: types.In, T: p.refType()})
		default:
			args = append(args, types.Arg{T: p.refType()})
		}
	}
	p.expect('>')
	return args
}

// classType maps a Java class type to the platform type Kotlin sees.
func (t *classTable) classType(name string, args []types.Arg) types.Type {
	switch name {
	case "java/lang/String":
		return types.StringType.Platform()
	case "java/lang/Object":
		return types.AnyType.Platform()
	}
	if k := types.BoxKind(name); k != types.Invalid {
		return types.Type{K: k}.Nullable().Platform()
	}
	c := t.class(name)
	if c == nil {
		return types.InvalidType
	}
	if covariantMapped[name] {
		// Iterable<? super T> is a (Mutable)Iterable<in T>, which
		// conflicts with its out T: a star projection.
		for i, a := range args {
			if a.V == types.In {
				args[i] = types.Arg{V: types.Star}
			}
		}
	}
	return types.GenericType(c, args).Platform()
}

// covariantMapped are the Java classes Kotlin maps to classes whose type
// parameter is out in both their read-only and mutable forms.
var covariantMapped = map[string]bool{"java/lang/Iterable": true, "java/util/Iterator": true}

// classSignature reads a class's type parameters and generic supertypes.
// An inner class's outer type parameters, outer, are in scope and are its
// last ones.
func (t *classTable) classSignature(cls *types.ClassSym, sig string, outer []*types.TypeParamSym, bounds [][]string) {
	p := &sigParser{t: t, s: sig, ok: true, scope: outer}
	ps := p.typeParams(cls, bounds)
	var supers []types.Type
	for p.ok && p.i < len(p.s) {
		st := p.refType()
		if st.K == types.Class {
			supers = append(supers, st.NonNull())
		}
	}
	if !p.ok {
		return // keep the raw supertypes
	}
	if v, ok := declaredVariance[cls.Name]; ok && len(ps) == 1 {
		ps[0].Var = v
	}
	if len(outer) > 0 {
		ps = append(ps[:len(ps):len(ps)], outer...)
		cls.OuterParams = len(outer)
	}
	cls.TypeParams = ps
	if sig != "" {
		cls.Supers = supers
	}
}

// methodSignature fills in a method's type parameters, parameter types
// and return type from its signature, and reports whether kc supports
// them. nparams is the number of parameters in the descriptor.
func (t *classTable) methodSignature(f *types.FunSym, cls *types.ClassSym, sig string, nparams int, bounds [][]string) bool {
	p := &sigParser{t: t, s: sig, ok: true, scope: cls.TypeParams}
	if f.Static {
		p.scope = nil
	}
	if cls.Source || t.cp != nil && t.cp.IsSource(cls.Name) {
		p.foreign = cls
	}
	f.TypeParams = p.typeParams(f, bounds)
	p.expect('(')
	var params []*types.VarSym
	for p.ok && p.peek() != ')' {
		params = append(params, &types.VarSym{Name: "p", Type: p.javaType(), Kind: types.Param})
	}
	p.expect(')')
	ret := p.javaType()
	if !p.ok || len(params) != nparams {
		return false
	}
	f.Params, f.Ret = params, ret
	return true
}

// fieldSignature returns the type of a field from its signature, which
// may mention the type parameters scope: an instance field's class's, as
// T element of Ref.ObjectRef<T>.
func (t *classTable) fieldSignature(sig string, scope []*types.TypeParamSym) (types.Type, bool) {
	p := &sigParser{t: t, s: sig, ok: true, scope: scope}
	ft := p.refType()
	return ft, p.ok && p.i == len(p.s)
}

// varargElem removes the array of a varargs method's last parameter from
// its descriptor or signature, so that the parameter is read as the
// element type. It reports false if the last parameter is not an array.
func varargElem(s string) (string, bool) {
	p := &sigParser{s: s, ok: true}
	if p.peek() == '<' {
		for depth := 0; p.i < len(p.s); p.i++ {
			if p.s[p.i] == '<' {
				depth++
			} else if p.s[p.i] == '>' {
				if depth--; depth == 0 {
					p.i++
					break
				}
			}
		}
	}
	p.expect('(')
	last := -1
	for p.ok && p.peek() != ')' {
		last = p.i
		switch p.peek() {
		case 'L', 'T', '[':
			p.skipRef()
		default:
			p.i++
		}
	}
	if !p.ok || last < 0 || s[last] != '[' {
		return s, false
	}
	return s[:last] + s[last+1:], true
}
