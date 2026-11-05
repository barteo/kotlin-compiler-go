package ka

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// decl returns the model of declaration d, a member of class owner (nil
// for a top-level one), whose class ID, if it is a class, starts with
// prefix: a/b/ or a/b/Outer.; nil for a declaration the model doesn't
// have: one that lowering or a plugin synthesized.
func (b *fileBuilder) decl(d syntax.Decl, owner *syntax.ClassDecl, prefix string) *Decl {
	switch d := d.(type) {
	case *syntax.ClassDecl:
		if b.prov.Node(d) == 0 {
			return nil
		}
		return b.class(d, prefix)
	case *syntax.FunDecl:
		if d.Plugin || b.prov.Node(d) == 0 {
			return nil
		}
		md := &Decl{Kind: KindFunction, Name: d.Name, Node: b.prov.Node(d),
			Visibility: visibility(d.Visibility), Annotations: b.annotations(d.Annotations),
			Flags: funFlags(&d.Modifiers)}
		md.Modality = memberModality(&d.Modifiers, owner, d.Body == nil && d.ExprBody == nil)
		md.TypeParams = b.typeParams(d.TypeParams)
		md.Receiver = b.written(d.Recv)
		md.Params = b.params(d.Params)
		if d.Sym != nil {
			md.Type = b.typ(d.Sym.Ret, d.Ret)
		}
		return md
	case *syntax.PropDecl:
		if d.Plugin || d.DelegateOf != nil || b.prov.Node(d) == 0 {
			return nil
		}
		md := &Decl{Kind: KindProperty, Name: d.Name, Node: b.prov.Node(d),
			Visibility: visibility(d.Visibility), Annotations: b.annotations(d.Annotations),
			Flags: propFlags(&d.Modifiers, d.Mutable)}
		md.Modality = memberModality(&d.Modifiers, owner, d.Init == nil && d.Getter == nil)
		md.TypeParams = b.typeParams(d.TypeParams)
		md.Receiver = b.written(d.Recv)
		if d.Sym != nil {
			md.Type = b.typ(d.Sym.Type, d.Type)
		} else {
			md.Type = b.written(d.Type)
		}
		return md
	case *syntax.TypeAliasDecl:
		md := &Decl{Kind: KindTypeAlias, Name: d.Name, Node: b.prov.Node(d),
			ClassID: prefix + d.Name, Visibility: visibility(d.Visibility),
			Annotations: b.annotations(d.Annotations), Flags: commonFlags(&d.Modifiers)}
		md.TypeParams = b.typeParams(d.TypeParams)
		if d.Sym != nil && d.Sym.Resolved {
			md.Type = b.typ(d.Sym.Type, d.Type)
		}
		return md
	case *syntax.SecondaryCtor:
		md := &Decl{Kind: KindConstructor, Name: "<init>", Node: b.prov.Node(d),
			Visibility: visibility(d.Visibility), Annotations: b.annotations(d.Annotations),
			Flags: commonFlags(&d.Modifiers)}
		md.Params = b.params(d.Params)
		return md
	}
	return nil
}

func (b *fileBuilder) class(d *syntax.ClassDecl, prefix string) *Decl {
	md := &Decl{Kind: KindClass, Name: d.Name, Node: b.prov.Node(d), ClassID: prefix + d.Name,
		Visibility: visibility(d.Visibility), Annotations: b.annotations(d.Annotations),
		Flags: classFlags(d)}
	switch {
	case d.Annotation:
		md.Kind = KindAnnotationClass
	case d.Interface:
		md.Kind = KindInterface
	case d.Object:
		md.Kind = KindObject
	case d.Enum:
		md.Kind = KindEnumClass
	}
	md.Modality = classModality(d)
	md.TypeParams = b.typeParams(d.TypeParams)
	md.Supertypes = b.supertypes(d)
	if !d.Interface && !d.Object {
		// The PRIMARY_CONSTRUCTOR, if written.
		pc := b.snap.Node(md.Node).ChildKind(psi.KindPRIMARY_CONSTRUCTOR).ID()
		ctor := &Decl{Kind: KindConstructor, Name: "<init>", Node: pc,
			Visibility: visibility(d.CtorMods.Visibility), Annotations: b.annotations(d.CtorMods.Annotations),
			Flags: commonFlags(&d.CtorMods)}
		for _, p := range d.CtorParams {
			ctor.Params = append(ctor.Params, b.param(&p.Param, b.prov.Node(p), p.Modifiers.Annotations))
		}
		md.Members = append(md.Members, ctor)
		for _, p := range d.CtorParams {
			if !p.IsProp {
				continue
			}
			mp := &Decl{Kind: KindProperty, Name: p.Name, Node: b.prov.Node(p),
				Visibility: visibility(p.Visibility), Annotations: b.annotations(p.Modifiers.Annotations),
				Flags: propFlags(&p.Modifiers, p.Mutable)}
			mp.Modality = memberModality(&p.Modifiers, d, false)
			if p.PropSym != nil {
				mp.Type = b.typ(p.PropSym.Type, p.Type)
			} else {
				mp.Type = b.written(p.Type)
			}
			md.Members = append(md.Members, mp)
		}
	}
	for _, e := range d.Entries {
		md.Members = append(md.Members, &Decl{Kind: KindEnumEntry, Name: e.Name, Node: b.prov.Node(e),
			ClassID: md.ClassID + "." + e.Name, Annotations: b.annotations(e.Annotations),
			Type: &Type{Kind: TypeClass, ClassID: md.ClassID}})
	}
	for _, m := range d.Members {
		if c, ok := m.(*syntax.ClassDecl); ok && c.EnumEntry {
			continue // the body of an enum entry, which is among Entries
		}
		if mm := b.decl(m, d, md.ClassID+"."); mm != nil {
			md.Members = append(md.Members, mm)
		}
	}
	return md
}

// supertypes returns the direct supertypes of class d as Kotlin sees
// them: those written, after kotlin/Enum<E> for an enum class and
// kotlin/Annotation for an annotation class, or kotlin/Any.
func (b *fileBuilder) supertypes(d *syntax.ClassDecl) []*Type {
	var out []*Type
	switch {
	case d.Enum && !d.EnumEntry:
		self := &Type{Kind: TypeClass, ClassID: ""}
		if d.Sym != nil {
			self = b.typ(d.Sym.This(), nil)
		}
		out = append(out, &Type{Kind: TypeClass, ClassID: "kotlin/Enum", Args: []TypeArg{{Type: self}}})
	case d.Annotation:
		out = append(out, &Type{Kind: TypeClass, ClassID: "kotlin/Annotation"})
	}
	for _, st := range d.Supers {
		if d.Sym != nil {
			out = append(out, b.typ(st.T, st.Type))
		} else {
			out = append(out, b.written(st.Type))
		}
	}
	if len(out) == 0 {
		out = append(out, &Type{Kind: TypeClass, ClassID: "kotlin/Any"})
	}
	return out
}

// written returns the model of a type as written, ref, as the checker
// resolved it; nil if ref is nil, and an error type if the checker didn't
// get to it.
func (b *fileBuilder) written(ref *syntax.TypeRef) *Type {
	if ref == nil {
		return nil
	}
	if t, ok := b.refs[ref]; ok {
		return b.typ(t, ref)
	}
	if ref.Known != nil {
		return b.typ(*ref.Known, ref)
	}
	return &Type{Kind: TypeError, Name: b.refText(ref)}
}

func (b *fileBuilder) typeParams(tps []*syntax.TypeParam) []*Decl {
	var out []*Decl
	for _, tp := range tps {
		md := &Decl{Kind: KindTypeParameter, Name: tp.Name, Node: b.prov.Node(tp),
			Annotations: b.annotations(tp.Annotations), Variance: variance(tp.Variance)}
		if tp.Reified {
			md.Flags |= FlagReified
		}
		if tp.Sym != nil {
			b.tparams[tp.Sym] = md.Node
		}
		out = append(out, md)
	}
	// The bounds may mention the parameters that follow: <T : Comparable<U>, U>.
	for i, tp := range tps {
		if tp.Sym != nil && len(tp.Sym.Bounds) == len(tp.Bounds) {
			for j, bt := range tp.Sym.Bounds {
				out[i].Bounds = append(out[i].Bounds, b.typ(bt, tp.Bounds[j]))
			}
			continue
		}
		for _, ref := range tp.Bounds {
			out[i].Bounds = append(out[i].Bounds, b.written(ref))
		}
	}
	return out
}

func (b *fileBuilder) params(ps []*syntax.Param) []*Decl {
	var out []*Decl
	for _, p := range ps {
		out = append(out, b.param(p, b.prov.Node(p), p.Annotations))
	}
	return out
}

// param returns the model of value parameter p, at node, annotated as.
func (b *fileBuilder) param(p *syntax.Param, node psi.NodeID, as []syntax.Annotation) *Decl {
	md := &Decl{Kind: KindValueParameter, Name: p.Name, Node: node, Annotations: b.annotations(as)}
	if p.Vararg {
		md.Flags |= FlagVararg
	}
	if p.Noinline {
		md.Flags |= FlagNoinline
	}
	if p.Crossinline {
		md.Flags |= FlagCrossinline
	}
	if p.Default != nil {
		md.Flags |= FlagHasDefault
	}
	switch {
	case p.Vararg || p.Sym == nil:
		// A vararg's Sym has the array type; the model has the element
		// type, as written.
		md.Type = b.written(p.Type)
	default:
		md.Type = b.typ(p.Sym.Type, p.Type)
	}
	return md
}

func commonFlags(m *syntax.Modifiers) Flags {
	var f Flags
	set := func(on bool, bit Flags) {
		if on {
			f |= bit
		}
	}
	set(m.Expect, FlagExpect)
	set(m.Actual, FlagActual)
	set(m.External, FlagExternal)
	set(m.Override, FlagOverride)
	return f
}

func funFlags(m *syntax.Modifiers) Flags {
	f := commonFlags(m)
	set := func(on bool, bit Flags) {
		if on {
			f |= bit
		}
	}
	set(m.Inline, FlagInline)
	set(m.Suspend, FlagSuspend)
	set(m.Operator, FlagOperator)
	set(m.Infix, FlagInfix)
	set(m.Tailrec, FlagTailrec)
	return f
}

func propFlags(m *syntax.Modifiers, mutable bool) Flags {
	f := commonFlags(m)
	if m.Const {
		f |= FlagConst
	}
	if m.Lateinit {
		f |= FlagLateinit
	}
	if mutable {
		f |= FlagMutable
	}
	return f
}

func classFlags(d *syntax.ClassDecl) Flags {
	f := commonFlags(&d.Modifiers)
	set := func(on bool, bit Flags) {
		if on {
			f |= bit
		}
	}
	set(d.Companion, FlagCompanion)
	set(d.Data, FlagData)
	set(d.Inner, FlagInner)
	set(d.Value || d.Inline, FlagValue)
	set(d.Fun, FlagFun)
	return f
}

// classModality returns the modality of class d as Kotlin declares it.
func classModality(d *syntax.ClassDecl) Modality {
	switch {
	case d.Sealed:
		return ModalitySealed
	case d.Interface || d.Abstract:
		return ModalityAbstract
	case d.Open && !d.Object && !d.Enum && !d.Annotation:
		return ModalityOpen
	}
	return ModalityFinal
}

// memberModality returns the modality of a member of class owner (nil for
// a top-level declaration) with modifiers m; noBody is set if it has no
// body or initializer.
func memberModality(m *syntax.Modifiers, owner *syntax.ClassDecl, noBody bool) Modality {
	switch {
	case owner == nil || m.Final || m.Visibility == syntax.Private:
		return ModalityFinal
	case m.Abstract:
		return ModalityAbstract
	case owner.Interface && noBody:
		return ModalityAbstract
	case owner.Interface || m.Open || m.Override:
		return ModalityOpen
	}
	return ModalityFinal
}
