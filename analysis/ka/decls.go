package ka

import (
	"fmt"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
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
			if md.Type != nil && md.Type.Kind == TypeError && md.Type.Name == "" {
				md.Type.Name = calleeName(d.ExprBody)
			}
		}
		md.Locals = b.locals(d.Body)
		return md
	case *syntax.PropDecl:
		if d.Plugin || d.DelegateOf != nil || b.prov.Node(d) == 0 {
			return nil
		}
		md := &Decl{Kind: KindProperty, Name: d.Name, Node: b.prov.Node(d),
			Visibility: visibility(d.Visibility), Annotations: b.propAnnotations(d.Annotations, false),
			Flags: propFlags(&d.Modifiers, d.Mutable)}
		if d.Setter != nil && len(d.Setter.Params) == 1 {
			// The setter's parameter's annotations are the property's
			// setparam ones.
			for _, a := range b.annotations(d.Setter.Params[0].Annotations) {
				a.Target = "setparam"
				md.Annotations = append(md.Annotations, a)
			}
		}
		md.Modality = memberModality(&d.Modifiers, owner, d.Init == nil && d.Getter == nil)
		if d.Sym != nil && d.Field == nil && !d.Expect && (d.Sym.HasField() || d.BackingField != nil) {
			md.Flags |= FlagBackingField
		}
		md.TypeParams = b.typeParams(d.TypeParams)
		md.Receiver = b.written(d.Recv)
		if d.Sym != nil {
			md.Type = b.typ(d.Sym.Type, d.Type)
		} else {
			md.Type = b.written(d.Type)
		}
		if md.Flags&FlagConst != 0 && d.Init != nil && d.Sym != nil {
			md.Default = b.defaultValue(d.Sym.Type, d.Init, 0)
		}
		for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
			if a != nil {
				md.Locals = append(md.Locals, b.locals(a.Body)...)
			}
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
		md.Locals = b.locals(d.Body)
		return md
	}
	return nil
}

// locals returns the models of the local declarations written directly
// in body: Decl.Locals.
func (b *fileBuilder) locals(body *syntax.Block) []*Decl {
	if body == nil {
		return nil
	}
	var out []*Decl
	for _, s := range body.Stmts {
		var md *Decl
		switch s := s.(type) {
		case *syntax.VarStmt:
			node := b.prov.Node(s)
			if node == 0 {
				continue
			}
			md = &Decl{Kind: KindProperty, Name: s.Name, Node: node,
				Annotations: b.annotations(b.f.Annotated[s])}
			if s.Mutable {
				md.Flags |= FlagMutable
			}
			if s.Lateinit {
				md.Flags |= FlagLateinit
			}
			if s.Sym != nil {
				md.Type = b.typ(s.Sym.Type, s.Type)
			} else {
				md.Type = b.written(s.Type)
			}
		case *syntax.Destructure:
			md = b.destructuring(s)
		case *syntax.LocalFun:
			md = b.decl(s.Decl, nil, "")
		case *syntax.LocalClass:
			// Its annotations are the statement's.
			if md = b.decl(s.Decl, nil, ""); md != nil {
				md.ClassID = ""
				md.Annotations = append(b.annotations(s.Annotations), md.Annotations...)
			}
		}
		if md != nil {
			md.Visibility = Local
			out = append(out, md)
		}
	}
	return out
}

// destructuring returns the model of a local destructuring declaration,
// or nil if it has no psi node. Its entries' types are the checker's
// where it checked the body, else the written ones.
func (b *fileBuilder) destructuring(s *syntax.Destructure) *Decl {
	node := b.prov.Node(s)
	if node == 0 {
		return nil
	}
	md := &Decl{Kind: KindDestructuring, Name: "<destruct>", Node: node,
		Annotations: b.annotations(b.f.Annotated[s])}
	var entries []psi.NodeID
	for c := b.snap.Node(node).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindDESTRUCTURING_DECLARATION_ENTRY {
			entries = append(entries, c.ID())
		}
	}
	for i, name := range s.Names {
		e := &Decl{Kind: KindProperty, Name: name, Visibility: Local}
		if i < len(entries) {
			e.Node = entries[i]
		}
		if i < len(s.Annotations) {
			e.Annotations = b.annotations(s.Annotations[i])
		}
		if s.Mutable {
			e.Flags |= FlagMutable
		}
		for _, vs := range s.Stmts {
			if vs.Name == name && vs.Sym != nil {
				e.Type = b.typ(vs.Sym.Type, s.Types[i])
			}
		}
		if e.Type == nil {
			e.Type = b.written(s.Types[i])
		}
		md.Params = append(md.Params, e)
	}
	return md
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
	if !d.Interface {
		// The PRIMARY_CONSTRUCTOR, if written. An object has an implicit,
		// private one, as FIR gives it.
		pc := b.snap.Node(md.Node).ChildKind(psi.KindPRIMARY_CONSTRUCTOR).ID()
		// Without one, a class that declares no constructor has an
		// implicit, empty one, and a class that does has none.
		if pc != 0 || !slices.ContainsFunc(d.Members, func(m syntax.Decl) bool {
			_, ok := m.(*syntax.SecondaryCtor)
			return ok
		}) {
			ctor := &Decl{Kind: KindConstructor, Name: "<init>", Node: pc,
				Visibility: visibility(d.CtorMods.Visibility), Annotations: b.annotations(d.CtorMods.Annotations),
				Flags: commonFlags(&d.CtorMods) | FlagPrimary}
			if d.Object || d.Enum {
				ctor.Visibility = Private // always private (enum) or implicit (object)
			}
			for _, p := range d.CtorParams {
				mp := b.param(&p.Param, b.prov.Node(p), p.Modifiers.Annotations)
				if p.IsProp {
					mp.Annotations = b.ctorParamAnnotations(p.Modifiers.Annotations)
				}
				if d.Annotation && p.Default != nil && p.Sym != nil {
					mp.Default = b.defaultValue(p.Sym.Type, p.Default, 0)
				}
				ctor.Params = append(ctor.Params, mp)
			}
			md.Members = append(md.Members, ctor)
		}
		var props []*Decl // the properties of the primary constructor
		for _, p := range d.CtorParams {
			if !p.IsProp {
				continue
			}
			mp := &Decl{Kind: KindProperty, Name: p.Name, Node: b.prov.Node(p),
				Visibility: visibility(p.Visibility), Annotations: b.propAnnotations(p.Modifiers.Annotations, true),
				Flags: propFlags(&p.Modifiers, p.Mutable) | FlagBackingField}
			mp.Modality = memberModality(&p.Modifiers, d, false)
			if p.PropSym != nil {
				mp.Type = b.typ(p.PropSym.Type, p.Type)
			} else {
				mp.Type = b.written(p.Type)
			}
			md.Members = append(md.Members, mp)
			props = append(props, mp)
		}
		defer func() {
			if d.Data {
				md.Members = append(md.Members, dataMembers(md, props)...)
			}
		}()
	}
	for _, e := range d.Entries {
		md.Members = append(md.Members, &Decl{Kind: KindEnumEntry, Name: e.Name, Node: b.prov.Node(e),
			ClassID: md.ClassID + "." + e.Name, Annotations: b.annotations(e.Annotations),
			Type: &Type{Kind: TypeClass, ClassID: md.ClassID}})
	}
	if md.Kind == KindEnumClass {
		md.Members = append(md.Members, enumMembers(md.ClassID, d.EntriesList, FlagStatic)...)
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

// propAnnotations returns the annotations of a property, written on it or,
// for a constructor's val or var, on its parameter (ctor), as Kotlin 2.4's
// default rule (param-property) places those without a use-site target:
// one that may be on a property is the property's (and, for a constructor
// property, the parameter's as well, if it may be on one); one that may be
// on a field and not on a property is the backing field's, with Target
// "field" and Implied set; one that may be on neither is the parameter's
// only, or, written on a property, stays there. Those with a target keep
// it: the JVM side splits them between the property and its accessors.
func (b *fileBuilder) propAnnotations(as []syntax.Annotation, ctor bool) []Annotation {
	var out []Annotation
	for i := range as {
		a := &as[i]
		if a.Implicit {
			continue
		}
		m := b.annotation(a)
		if cls := b.annotationClass(a); m.Target == "" && cls != nil && cls.TargetsKnown {
			switch {
			case cls.Targets&types.TargetProperty != 0:
			case cls.Targets&types.TargetField != 0:
				m.Target, m.Implied = "field", true
			case ctor:
				continue
			}
		}
		out = append(out, m)
	}
	return out
}

// ctorParamAnnotations returns the annotations of a constructor's val or
// var parameter that are the parameter's: as for any parameter, but
// without those written without a use-site target that may not be on a
// parameter (they are the property's or its field's).
func (b *fileBuilder) ctorParamAnnotations(as []syntax.Annotation) []Annotation {
	var out []Annotation
	for i := range as {
		a := &as[i]
		if a.Implicit {
			continue
		}
		if cls := b.annotationClass(a); a.Target == "" && cls != nil && cls.TargetsKnown &&
			cls.Targets&types.TargetValueParameter == 0 {
			continue
		}
		out = append(out, b.annotation(a))
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

// dataMembers returns the members Kotlin generates for data class md
// with primary constructor properties props, as FIR has them after the
// class's own members: componentN for each property, then copy.
// equals, hashCode and toString are generated later, so FIR's
// declarations don't have them.
func dataMembers(md *Decl, props []*Decl) []*Decl {
	var out []*Decl
	for i, p := range props {
		out = append(out, &Decl{Kind: KindFunction, Name: fmt.Sprintf("component%d", i+1),
			Visibility: p.Visibility, Modality: ModalityFinal, Flags: FlagOperator, Type: p.Type})
	}
	cp := &Decl{Kind: KindFunction, Name: "copy", Visibility: Public, Modality: ModalityFinal}
	for _, m := range md.Members {
		if m.Kind == KindConstructor && m.Flags&FlagPrimary != 0 {
			cp.Visibility = m.Visibility
		}
	}
	self := &Type{Kind: TypeClass, ClassID: md.ClassID}
	for _, tp := range md.TypeParams {
		self.Args = append(self.Args, TypeArg{Type: &Type{Kind: TypeParameter, Name: tp.Name, Param: tp.Node}})
	}
	cp.Type = self
	for _, p := range props {
		cp.Params = append(cp.Params, &Decl{Kind: KindValueParameter, Name: p.Name, Type: p.Type, Flags: FlagHasDefault})
	}
	return append(out, cp)
}

// calleeName returns the name an unresolved call e calls, which FIR
// names the error type of an omitted return type after, or "".
func calleeName(e syntax.Expr) string {
	if c, ok := e.(*syntax.Call); ok {
		if id, ok := c.Fun.(*syntax.Ident); ok && c.Sym == nil && c.Ctor == nil {
			return id.Name
		}
	}
	return ""
}
