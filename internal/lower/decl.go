package lower

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
	"strings"
)

// file lowers the FILE node, as parseFile parses a file.
func (l *lowerer) file(root psi.NodeRef) *syntax.File {
	f := &syntax.File{}
	for _, c := range kids(root) {
		switch c.NumericKind() {
		case psi.KindFILE_ANNOTATION_LIST:
			for _, e := range kids(c) {
				l.decl(e, func() {
					var as []syntax.Annotation
					switch e.NumericKind() {
					case psi.KindANNOTATION_ENTRY:
						as = []syntax.Annotation{l.annotation(e)}
					case psi.KindANNOTATION:
						as = l.bracketAnnotations(e)
					default:
						l.unexpected(e)
					}
					for _, a := range as {
						if a.Target == "file" && a.Is("JvmName") {
							if s, ok := a.Str(""); ok {
								f.JvmName = s
							}
						}
						if a.Target == "file" && a.Name != "" {
							f.Annotations = append(f.Annotations, a)
						}
					}
				})
			}
		case psi.KindPACKAGE_DIRECTIVE:
			if c.FirstChild().Valid() {
				l.decl(c, func() {
					f.Package = l.dotted(c)
					f.PackageP = l.pos(next(child(c, psi.KindPACKAGE_KEYWORD)))
				})
			}
		case psi.KindIMPORT_LIST:
			for _, d := range kids(c) {
				l.decl(d, func() { f.Imports = append(f.Imports, l.importDirective(d)) })
			}
		case psi.KindSEMICOLON:
		default:
			l.decl(c, func() { l.topLevel(f, c) })
		}
	}
	return f
}

// dotted returns the dotted name of the identifiers under n, without
// backticks.
func (l *lowerer) dotted(n psi.NodeRef) string {
	name := ""
	end := n.SubtreeEnd()
	for id := n.ID(); id < end; id++ {
		c := l.s.Node(id)
		if c.NumericKind() == psi.KindIDENTIFIER {
			if name != "" {
				name += "."
			}
			name += ident(c)
		}
	}
	return name
}

func (l *lowerer) importDirective(n psi.NodeRef) *syntax.Import {
	imp := record(l, &syntax.Import{P: l.pos(n)}, n)
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindREFERENCE_EXPRESSION:
			imp.Path = l.dotted(c)
		case psi.KindMUL:
			imp.Star = true
		case psi.KindIMPORT_ALIAS:
			imp.Alias = ident(child(c, psi.KindIDENTIFIER))
		}
	}
	return imp
}

// topLevel lowers a top-level declaration.
func (l *lowerer) topLevel(f *syntax.File, n psi.NodeRef) {
	m := l.modifiers(child(n, psi.KindMODIFIER_LIST))
	syntax.ParamOnly(m, l)
	add := func(d syntax.Decl) {
		if m.Expect {
			f.Expects = append(f.Expects, d)
		} else {
			f.Decls = append(f.Decls, d)
		}
	}
	switch n.NumericKind() {
	case psi.KindFUN:
		fd := l.fun(n)
		fd.Modifiers, fd.End = m, l.lastTok(n)
		add(fd)
	case psi.KindPROPERTY:
		pd := l.prop(n, m)
		for _, d := range syntax.DelegatedProp(pd, false, f.Decls) {
			add(d)
		}
	case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
		syntax.NoContexts(&m, "classes", l)
		add(l.class(n, m))
	case psi.KindTYPEALIAS:
		syntax.NoContexts(&m, "type aliases", l)
		add(l.typeAlias(n, m))
	case psi.KindDESTRUCTURING_DECLARATION:
		l.newDestructuring(n)
		l.unsupported(l.pos(child(n, psi.KindLPAR)), "destructuring declaration", "destructuring declarations are not supported")
	default:
		l.fail(n, "unexpected declaration")
	}
}

// fun lowers a FUN, a named function, but for its modifiers.
func (l *lowerer) fun(n psi.NodeRef) *syntax.FunDecl {
	if has(n, psi.KindINTERFACE_KEYWORD) {
		l.fail(child(n, psi.KindINTERFACE_KEYWORD), "'interface' is not allowed here: local interfaces are not supported")
	}
	fd := record(l, &syntax.FunDecl{P: l.pos(child(n, psi.KindFUN_KEYWORD))}, n)
	var tpl, where psi.NodeRef
	seenName, seenColon := false, false
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindTYPE_PARAMETER_LIST:
			tpl = c
		case psi.KindTYPE_REFERENCE:
			switch {
			case !seenName:
				fd.Recv, fd.RecvAnnotations = l.recvType(c)
			case seenColon:
				fd.Ret = l.typeRef(c)
			}
		case psi.KindIDENTIFIER:
			seenName = true
			fd.Name = ident(c)
		case psi.KindVALUE_PARAMETER_LIST:
			fd.Params = l.params(c)
		case psi.KindCOLON:
			seenColon = true
		case psi.KindTYPE_CONSTRAINT_LIST:
			where = c
		case psi.KindBLOCK:
			fd.Body = l.block(c)
		case psi.KindEQ:
			fd.ExprBody = l.expr(exprAfter(n, psi.KindEQ))
		}
		if c.NumericKind() == psi.KindTYPE_PARAMETER_LIST {
			fd.TypeParams = l.typeParams(tpl)
		}
		if c.NumericKind() == psi.KindTYPE_CONSTRAINT_LIST {
			l.whereClause(where, fd.TypeParams)
		}
	}
	if !seenName {
		l.fail(child(n, psi.KindVALUE_PARAMETER_LIST), "expected a name")
	}
	if fd.Params == nil && !has(n, psi.KindVALUE_PARAMETER_LIST) {
		l.fail(n, "expected '('")
	}
	return fd
}

// params lowers the VALUE_PARAMETER_LIST of a function or a secondary
// constructor.
func (l *lowerer) params(n psi.NodeRef) []*syntax.Param {
	var ps []*syntax.Param
	for _, c := range kids(n) {
		if c.NumericKind() != psi.KindVALUE_PARAMETER {
			continue
		}
		m := l.modifiers(child(c, psi.KindMODIFIER_LIST))
		syntax.NoContexts(&m, "value parameters", l)
		name := child(c, psi.KindIDENTIFIER)
		if !name.Valid() {
			l.fail(c, "expected a name")
		}
		p := record(l, &syntax.Param{P: l.pos(name), Name: ident(name), Type: l.paramType(c, name), Vararg: m.Vararg,
			Noinline: m.Noinline, Crossinline: m.Crossinline, Annotations: m.Annotations}, c)
		p.External, p.ExternalP = m.External, m.ExternalP
		p.Mods = m.Written
		if d := exprAfter(c, psi.KindEQ); d.Valid() {
			p.Default = l.expr(d)
		}
		ps = append(ps, p)
	}
	return ps
}

// prop lowers a PROPERTY with modifiers m.
func (l *lowerer) prop(n psi.NodeRef, m syntax.Modifiers) *syntax.PropDecl {
	kw := first(n)
	for kw.Valid() && kw.NumericKind() == psi.KindMODIFIER_LIST {
		kw = next(kw)
	}
	pd := record(l, &syntax.PropDecl{Modifiers: m, P: l.pos(kw), Mutable: kw.NumericKind() == psi.KindVAR_KEYWORD}, n)
	seenName, seenColon := false, false
	var delegate psi.NodeRef
	var accessors []psi.NodeRef
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindTYPE_PARAMETER_LIST:
			pd.TypeParams = l.typeParams(c)
		case psi.KindTYPE_REFERENCE:
			switch {
			case !seenName:
				pd.Recv, pd.RecvAnnotations = l.recvType(c)
			case seenColon:
				pd.Type = l.typeRef(c)
				if e := unclosedTypeArgs(c); e.Valid() {
					pos, msg := l.syntaxMessage(e)
					l.errorLater(pos, "%s", msg)
				}
			}
		case psi.KindIDENTIFIER:
			seenName = true
			pd.Name = ident(c)
			if pd.TypeParams != nil && pd.Recv == nil && m.Contexts == nil {
				l.failAt(pd.P, "type parameters of a property must be used in its receiver type")
			}
		case psi.KindCOLON:
			seenColon = true
		case psi.KindTYPE_CONSTRAINT_LIST:
			l.whereClause(c, pd.TypeParams)
		case psi.KindEQ:
			pd.Init = l.expr(exprAfter(n, psi.KindEQ))
		case psi.KindPROPERTY_DELEGATE:
			delegate = c
		case psi.KindPROPERTY_ACCESSOR, psi.KindBACKING_FIELD:
			accessors = append(accessors, c)
		}
		if c.NumericKind() == psi.KindPROPERTY_DELEGATE {
			by := child(c, psi.KindBY_KEYWORD)
			if m.Contexts != nil {
				l.fail(by, "delegated property cannot have context parameters")
			}
			pd.Field = record(l, &syntax.PropDecl{P: l.pos(by), Name: pd.Name + "$delegate", DelegateOf: pd}, c)
			pd.Field.Init = l.expr(exprAfter(c, psi.KindBY_KEYWORD))
		}
	}
	if !seenName {
		l.fail(n, "expected a name")
	}
	for _, a := range accessors {
		if a.NumericKind() == psi.KindBACKING_FIELD {
			l.backingField(pd, a)
		} else {
			l.accessor(pd, a)
		}
	}
	if delegate.Valid() {
		syntax.CheckDelegated(pd, l)
	}
	return pd
}

// backingField lowers the BACKING_FIELD of property pd, field: T = init.
// Its annotations without a target are the field's.
func (l *lowerer) backingField(pd *syntax.PropDecl, n psi.NodeRef) {
	m := l.modifiers(child(n, psi.KindMODIFIER_LIST))
	kw := child(n, psi.KindFIELD_KEYWORD)
	if pd.BackingField != nil {
		l.fail(kw, "property has two backing fields")
	}
	f := &syntax.BackingField{P: l.pos(kw), Mods: m.Written}
	for _, a := range m.Annotations {
		if a.Target == "" {
			a.Target = "field"
		}
		pd.Annotations = append(pd.Annotations, a)
	}
	if t := child(n, psi.KindTYPE_REFERENCE); t.Valid() {
		f.Type = l.typeRef(t)
	}
	if has(n, psi.KindEQ) {
		f.Init = l.expr(exprAfter(n, psi.KindEQ))
	}
	pd.BackingField = f
}

// accessor lowers a PROPERTY_ACCESSOR of property pd.
func (l *lowerer) accessor(pd *syntax.PropDecl, n psi.NodeRef) {
	m := l.modifiers(child(n, psi.KindMODIFIER_LIST))
	kw := child(n, psi.KindGET_KEYWORD)
	name := "get"
	if !kw.Valid() {
		kw, name = child(n, psi.KindSET_KEYWORD), "set"
	}
	pos := l.pos(kw)
	syntax.AccessorMods(pd, name, pos, &m, l)
	ps := child(n, psi.KindVALUE_PARAMETER_LIST)
	if !ps.Valid() {
		if has(n, psi.KindBLOCK) || has(n, psi.KindEQ) {
			l.fail(next(kw), "unexpected %s", next(kw).Text())
		}
		if m.External {
			record(l, syntax.ExternalAccessor(pd, name, pos, m, l), n)
		}
		return
	}
	fd := record(l, &syntax.FunDecl{P: pos, Name: name, Property: pd.Name}, n)
	fd.External, fd.ExternalP = m.External, m.ExternalP
	ret := psi.NodeRef{}
	if c := child(n, psi.KindCOLON); c.Valid() {
		ret = next(c)
	}
	syntax.CheckAccessor(pd, name, pos, l)
	if name == "get" {
		if ret.Valid() {
			fd.Ret = l.typeRef(ret)
			if pd.Type == nil {
				pd.Type = fd.Ret
			}
		}
	} else {
		fd.Visibility = m.Visibility
		vp := child(ps, psi.KindVALUE_PARAMETER)
		if !vp.Valid() {
			l.fail(child(ps, psi.KindRPAR), "expected a name")
		}
		pn := child(vp, psi.KindIDENTIFIER)
		param := record(l, &syntax.Param{P: l.pos(pn), Name: ident(pn)}, vp)
		if t := child(vp, psi.KindTYPE_REFERENCE); t.Valid() {
			param.Type = l.typeRef(t)
		}
		if mods := child(vp, psi.KindMODIFIER_LIST); mods.Valid() {
			for _, k := range kids(mods) {
				if k.NumericKind() == psi.KindANNOTATION_ENTRY {
					param.Annotations = append(param.Annotations, l.annotation(k))
				} else if k.NumericKind() == psi.KindANNOTATION {
					param.Annotations = append(param.Annotations, l.bracketAnnotations(k)...)
				} else {
					l.unexpected(k)
				}
			}
		}
		fd.Params = []*syntax.Param{param}
		if ret.Valid() {
			if t := l.typeRef(ret); t.Name != "Unit" || t.Nullable {
				l.failAt(t.P, "setter return type must be Unit")
			}
		}
	}
	switch {
	case has(n, psi.KindBLOCK):
		fd.Body = l.block(child(n, psi.KindBLOCK))
	case has(n, psi.KindEQ):
		fd.ExprBody = l.expr(exprAfter(n, psi.KindEQ))
	default:
		l.failAt(l.end(n), "unexpected %s body", name+"ter")
	}
	fd.End = l.lastTok(n)
	if name == "get" {
		pd.Getter = fd
	} else {
		pd.Setter = fd
	}
}

// typeAlias lowers a TYPEALIAS.
func (l *lowerer) typeAlias(n psi.NodeRef, m syntax.Modifiers) *syntax.TypeAliasDecl {
	d := record(l, &syntax.TypeAliasDecl{Modifiers: m, P: l.pos(child(n, psi.KindTYPE_ALIAS_KEYWORD)),
		Name: ident(child(n, psi.KindIDENTIFIER))}, n)
	d.TypeParams = l.typeParams(child(n, psi.KindTYPE_PARAMETER_LIST))
	t := child(n, psi.KindTYPE_REFERENCE)
	if !t.Valid() {
		l.failAt(l.end(n), "expected '='")
	}
	d.Type = l.typeRef(t)
	return d
}

// NoNameProvided names an object declaration written without a name, as
// FIR's SpecialNames.NO_NAME_PROVIDED does.
const NoNameProvided = "<no name provided>"

// class lowers a CLASS or an OBJECT_DECLARATION with modifiers m.
func (l *lowerer) class(n psi.NodeRef, m syntax.Modifiers) *syntax.ClassDecl {
	var kw psi.NodeRef
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindCLASS_KEYWORD, psi.KindINTERFACE_KEYWORD, psi.KindOBJECT_KEYWORD:
			kw = c
		}
		if kw.Valid() {
			break
		}
	}
	kpos := l.pos(kw)
	isObject := kw.NumericKind() == psi.KindOBJECT_KEYWORD
	if m.Inline && kw.NumericKind() == psi.KindCLASS_KEYWORD {
		syntax.InlineClass(&m, kpos)
	}
	cd := record(l, &syntax.ClassDecl{Modifiers: m, P: kpos, Interface: kw.NumericKind() == psi.KindINTERFACE_KEYWORD, Object: isObject}, n)
	if mods := child(n, psi.KindMODIFIER_LIST); mods.Valid() && has(mods, psi.KindFUN_KEYWORD) {
		cd.Fun = true
	}
	name := child(n, psi.KindIDENTIFIER)
	switch {
	case m.Companion && !isObject:
		l.failAt(kpos, "modifier 'companion' is only allowed on objects")
	case m.Companion && !name.Valid():
		cd.Name = "Companion"
	case m.Companion && strings.Contains(l.src[kw.Range().End:name.Range().Start], "\n"):
		// companion object, then a name on the next line: kc ends the
		// header at the newline, where psi reads on.
		cd.Name = "Companion"
		l.detachName(n, name)
	case !name.Valid() && isObject && kw.Valid():
		// An object declaration without its name: FIR keeps it, named
		// <no name provided>, and KSP lists it.
		pos, msg := l.end(kw), "expected a name"
		if e := child(n, psi.KindERROR_ELEMENT); e.Valid() {
			pos, msg = l.syntaxMessage(e)
		}
		l.errorLater(pos, "%s", msg)
		cd.Name = NoNameProvided
	case !name.Valid():
		l.failAt(l.end(kw), "expected a name")
	default:
		cd.Name = ident(name)
	}
	cd.TypeParams = l.typeParams(child(n, psi.KindTYPE_PARAMETER_LIST))
	if pc := child(n, psi.KindPRIMARY_CONSTRUCTOR); pc.Valid() {
		cd.CtorMods = l.modifiers(child(pc, psi.KindMODIFIER_LIST))
		syntax.NoContexts(&cd.CtorMods, "primary constructors", l)
		if ps := child(pc, psi.KindVALUE_PARAMETER_LIST); ps.Valid() {
			cd.HasPrimary = true
			cd.CtorParams = l.ctorParams(ps)
		}
	}
	if st := child(n, psi.KindSUPER_TYPE_LIST); st.Valid() {
		cd.Supers = l.supertypes(st)
	}
	l.whereClause(child(n, psi.KindTYPE_CONSTRAINT_LIST), cd.TypeParams)
	if body := child(n, psi.KindCLASS_BODY); body.Valid() {
		l.classBody(cd, body)
		cd.End = l.pos(child(body, psi.KindRBRACE))
	}
	if !cd.HasPrimary && cd.Secondaries() == nil {
		cd.HasPrimary = true
	}
	return cd
}

// detachName lowers the name psi gives companion object n from the line
// after it as kc reads it: a modifier of the next member, which member
// takes from carry.
func (l *lowerer) detachName(n, name psi.NodeRef) {
	if next(name).Valid() || !syntax.IsModifier(name.Text()) || n.Parent().NumericKind() != psi.KindCLASS_BODY {
		l.fail(name, "expected member declaration, found '%s'", name.Text())
	}
	l.carry = name
}

// ctorParams lowers a primary constructor's VALUE_PARAMETER_LIST.
func (l *lowerer) ctorParams(n psi.NodeRef) []*syntax.CtorParam {
	var ps []*syntax.CtorParam
	for _, c := range kids(n) {
		if c.NumericKind() != psi.KindVALUE_PARAMETER {
			continue
		}
		m := l.modifiers(child(c, psi.KindMODIFIER_LIST))
		syntax.NoContexts(&m, "value parameters", l)
		cp := record(l, &syntax.CtorParam{Modifiers: m}, c)
		if has(c, psi.KindVAL_KEYWORD) || has(c, psi.KindVAR_KEYWORD) {
			cp.IsProp = true
			cp.Mutable = has(c, psi.KindVAR_KEYWORD)
		}
		name := child(c, psi.KindIDENTIFIER)
		if !name.Valid() {
			l.fail(c, "expected a name")
		}
		cp.P, cp.Name, cp.Type, cp.Param.Vararg = l.pos(name), ident(name), l.paramType(c, name), m.Vararg
		if !cp.IsProp {
			cp.Param.External, cp.Param.ExternalP = m.External, m.ExternalP
		}
		if d := exprAfter(c, psi.KindEQ); d.Valid() {
			cp.Default = l.expr(d)
		}
		ps = append(ps, cp)
	}
	return ps
}

// supertypes lowers a SUPER_TYPE_LIST.
func (l *lowerer) supertypes(n psi.NodeRef) []*syntax.SuperType {
	var out []*syntax.SuperType
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindSUPER_TYPE_ENTRY:
			out = append(out, record(l, &syntax.SuperType{Type: l.typeRef(child(c, psi.KindTYPE_REFERENCE))}, c))
		case psi.KindSUPER_TYPE_CALL_ENTRY:
			st := record(l, &syntax.SuperType{Type: l.typeRef(child(child(c, psi.KindCONSTRUCTOR_CALLEE), psi.KindTYPE_REFERENCE))}, c)
			st.Call = l.callOn(syntax.TypeExpr(st.Type), st.Type.Args, c)
			out = append(out, st)
		case psi.KindDELEGATED_SUPER_TYPE_ENTRY:
			st := record(l, &syntax.SuperType{Type: l.typeRef(child(c, psi.KindTYPE_REFERENCE))}, c)
			st.Delegate = l.expr(exprAfter(c, psi.KindBY_KEYWORD))
			out = append(out, st)
		case psi.KindCOMMA:
		default:
			l.unexpected(c)
		}
	}
	return out
}

// classBody lowers a CLASS_BODY: an enum class's entries, then members.
func (l *lowerer) classBody(cd *syntax.ClassDecl, n psi.NodeRef) {
	members := false
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindLBRACE, psi.KindRBRACE, psi.KindSEMICOLON, psi.KindCOMMA:
		case psi.KindENUM_ENTRY:
			if !cd.Enum || members {
				l.decl(c, func() { l.fail(c, "unexpected enum entry") })
				continue
			}
			l.enumEntry(cd, c)
		default:
			members = true
			l.decl(c, func() { l.member(cd, c) })
		}
	}
	l.noCarry()
}

// noCarry reports a modifier detachName left for a member that doesn't
// follow.
func (l *lowerer) noCarry() {
	if c := l.carry; c.Valid() {
		l.carry = psi.NodeRef{}
		l.failAt(l.end(c), "expected member declaration, found newline")
	}
}

// enumEntry lowers an ENUM_ENTRY of enum class cd.
func (l *lowerer) enumEntry(cd *syntax.ClassDecl, n psi.NodeRef) {
	l.syntaxError(n)
	var anns []syntax.Annotation
	if mods := child(n, psi.KindMODIFIER_LIST); mods.Valid() {
		for _, c := range kids(mods) {
			switch c.NumericKind() {
			case psi.KindANNOTATION_ENTRY:
				anns = append(anns, l.annotation(c))
			case psi.KindANNOTATION:
				anns = append(anns, l.bracketAnnotations(c)...)
			case psi.KindEXTERNAL_KEYWORD:
			case psi.KindCONTEXT_PARAMETER_LIST:
				l.contextParams(c)
				l.errorLater(l.pos(c), "context parameters on enum entries are unsupported")
			default:
				l.unexpected(c)
			}
		}
	}
	name := child(n, psi.KindIDENTIFIER)
	e := record(l, &syntax.EnumEntry{P: l.pos(name), Name: ident(name), Annotations: anns}, n)
	id := &syntax.Ident{Name: cd.Name}
	id.P = e.P
	e.Call = &syntax.Call{Fun: id}
	e.Call.P = e.P
	if il := child(n, psi.KindINITIALIZER_LIST); il.Valid() {
		if entry := child(il, psi.KindSUPER_TYPE_CALL_ENTRY); entry.Valid() && has(entry, psi.KindVALUE_ARGUMENT_LIST) {
			e.Call = l.callOn(id, nil, entry)
		}
	}
	if body := child(n, psi.KindCLASS_BODY); body.Valid() {
		e.Body = record(l, syntax.EnumEntryClass(cd.Name, e), body)
		for _, c := range kids(body) {
			switch c.NumericKind() {
			case psi.KindLBRACE, psi.KindRBRACE, psi.KindSEMICOLON:
			default:
				l.decl(c, func() { l.member(e.Body, c) })
			}
		}
		l.noCarry()
		e.Body.End = l.pos(child(body, psi.KindRBRACE))
		cd.Members = append(cd.Members, e.Body)
	}
	cd.Entries = append(cd.Entries, e)
}

// paramType lowers the type of VALUE_PARAMETER c named name. A parameter
// written without one is an error, reported with the checker's, and has
// an invalid type, so the declaration it is in is kept.
func (l *lowerer) paramType(c, name psi.NodeRef) *syntax.TypeRef {
	tr := child(c, psi.KindTYPE_REFERENCE)
	if !tr.Valid() {
		pos, msg := l.end(name), "a type annotation is required on a value parameter"
		if e := child(c, psi.KindERROR_ELEMENT); e.Valid() {
			pos, msg = l.syntaxMessage(e)
		}
		l.errorLater(pos, "%s", msg)
		return &syntax.TypeRef{P: pos, Known: &types.InvalidType}
	}
	return l.typeRef(tr)
}

// bracketAnnotations lowers an ANNOTATION, @[A B] or @t:[A B], to its
// entries, each with the ANNOTATION's target. Without entries it is an
// annotation without a name, which kc skips.
func (l *lowerer) bracketAnnotations(n psi.NodeRef) []syntax.Annotation {
	target := ""
	if t := child(n, psi.KindANNOTATION_TARGET); t.Valid() {
		target = t.Text()
	}
	var out []syntax.Annotation
	for _, e := range kids(n) {
		if e.NumericKind() == psi.KindANNOTATION_ENTRY {
			a := l.annotation(e)
			if a.Target == "" {
				a.Target = target
			}
			out = append(out, a)
		}
	}
	if out == nil {
		out = []syntax.Annotation{{P: l.pos(n), Target: target}}
	}
	return out
}

// member lowers a member of class cd.
func (l *lowerer) member(cd *syntax.ClassDecl, n psi.NodeRef) {
	ms := kids(child(n, psi.KindMODIFIER_LIST))
	if c := l.carry; c.Valid() {
		l.carry = psi.NodeRef{}
		ms = append([]psi.NodeRef{c}, ms...)
	}
	m := l.modifiersOf(ms)
	syntax.ParamOnly(m, l)
	switch n.NumericKind() {
	case psi.KindFUN:
		fd := l.fun(n)
		fd.Modifiers, fd.End = m, l.lastTok(n)
		cd.Members = append(cd.Members, fd)
	case psi.KindPROPERTY:
		pd := l.prop(n, m)
		cd.Members = append(cd.Members, syntax.DelegatedProp(pd, true, cd.Members)...)
	case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
		syntax.NoContexts(&m, "classes", l)
		cd.Members = append(cd.Members, l.class(n, m))
	case psi.KindTYPEALIAS:
		syntax.NoContexts(&m, "type aliases", l)
		cd.Members = append(cd.Members, l.typeAlias(n, m))
	case psi.KindSECONDARY_CONSTRUCTOR:
		syntax.NoContexts(&m, "constructors", l)
		cd.Members = append(cd.Members, l.secondaryCtor(n, m))
	case psi.KindCLASS_INITIALIZER:
		syntax.NoContexts(&m, "initializers", l)
		b := child(n, psi.KindBLOCK)
		if !b.Valid() {
			l.failAt(l.end(n), "expected '{'")
		}
		cd.Members = append(cd.Members, record(l, &syntax.InitBlock{P: l.pos(child(n, psi.KindINIT_KEYWORD)), Body: l.block(b),
			Annotations: m.Annotations}, n))
	case psi.KindCOMPANION_BLOCK:
		pos := l.pos(n)
		for _, c := range kids(child(n, psi.KindMODIFIER_LIST)) {
			if c.NumericKind() == psi.KindCOMPANION_KEYWORD {
				pos = l.pos(c)
			}
		}
		l.unsupported(pos, "companion block", "companion blocks are not supported")
	default:
		l.fail(n, "unexpected member declaration")
	}
}

// secondaryCtor lowers a SECONDARY_CONSTRUCTOR with modifiers m.
func (l *lowerer) secondaryCtor(n psi.NodeRef, m syntax.Modifiers) *syntax.SecondaryCtor {
	sc := record(l, &syntax.SecondaryCtor{Modifiers: m, P: l.pos(child(n, psi.KindCONSTRUCTOR_KEYWORD))}, n)
	sc.Params = l.params(child(n, psi.KindVALUE_PARAMETER_LIST))
	if dc := child(n, psi.KindCONSTRUCTOR_DELEGATION_CALL); dc.Valid() {
		ref := child(dc, psi.KindCONSTRUCTOR_DELEGATION_REFERENCE)
		if kw := first(ref); kw.Valid() {
			id := &syntax.Ident{Name: kw.Text()}
			id.P = l.pos(kw)
			record(l, id, ref)
			sc.Delegation = l.callOn(id, nil, dc)
			sc.ToThis = kw.NumericKind() == psi.KindTHIS_KEYWORD
		}
	}
	if b := child(n, psi.KindBLOCK); b.Valid() {
		sc.Body = l.block(b)
	}
	return sc
}

// localClass lowers a local CLASS or OBJECT_DECLARATION with modifiers
// m.
func (l *lowerer) localClass(n psi.NodeRef, m syntax.Modifiers) *syntax.LocalClass {
	syntax.NoContexts(&m, "classes", l)
	switch {
	case has(n, psi.KindINTERFACE_KEYWORD):
		l.fail(child(n, psi.KindINTERFACE_KEYWORD), "'interface' is not allowed here: local interfaces are not supported")
	case has(n, psi.KindOBJECT_KEYWORD):
		l.fail(child(n, psi.KindOBJECT_KEYWORD), "named object '%s' cannot be local: use an anonymous object instead",
			ident(child(n, psi.KindIDENTIFIER)))
	case m.Enum:
		l.fail(child(n, psi.KindCLASS_KEYWORD), "local enum classes are not allowed")
	}
	cd := l.class(n, m)
	cd.Local = true
	return &syntax.LocalClass{Decl: cd}
}

// localFun lowers a local FUN with modifiers m.
func (l *lowerer) localFun(n psi.NodeRef, m syntax.Modifiers) *syntax.LocalFun {
	syntax.ParamOnly(m, l)
	fd := l.fun(n)
	fd.Modifiers = m
	syntax.CheckLocalFun(fd, l)
	return &syntax.LocalFun{Decl: fd}
}

// objectExpr lowers an OBJECT_LITERAL.
func (l *lowerer) objectExpr(n psi.NodeRef) *syntax.ObjectExpr {
	od := child(n, psi.KindOBJECT_DECLARATION)
	kw := child(od, psi.KindOBJECT_KEYWORD)
	cd := record(l, &syntax.ClassDecl{P: l.pos(kw), Anonymous: true, HasPrimary: true}, od)
	if mods := child(od, psi.KindMODIFIER_LIST); mods.Valid() {
		l.fail(mods, "unexpected modifiers")
	}
	if st := child(od, psi.KindSUPER_TYPE_LIST); st.Valid() {
		cd.Supers = l.supertypes(st)
	}
	if body := child(od, psi.KindCLASS_BODY); body.Valid() {
		for _, c := range kids(body) {
			switch c.NumericKind() {
			case psi.KindLBRACE, psi.KindRBRACE, psi.KindSEMICOLON:
			default:
				l.decl(c, func() { l.member(cd, c) })
			}
		}
		cd.End = l.pos(child(body, psi.KindRBRACE))
	}
	x := &syntax.ObjectExpr{Decl: cd}
	x.P = cd.P
	return record(l, x, n)
}
