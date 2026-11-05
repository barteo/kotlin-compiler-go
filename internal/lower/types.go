package lower

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
	"strings"
)

// typeRef lowers a TYPE_REFERENCE, or any node holding a type with its
// modifiers, parentheses and '?': NULLABLE_TYPE, FUNCTION_TYPE_RECEIVER.
func (l *lowerer) typeRef(n psi.NodeRef) *syntax.TypeRef {
	t, as := l.typeWith(n, false)
	if len(as) > 0 {
		l.failAt(as[0].P, "this annotation is not applicable to target 'type usage' and use-site target '@receiver'")
	}
	return t
}

// recvType lowers the receiver type of an extension, and returns the
// annotations before it with the use-site target receiver, which are the
// receiver parameter's.
func (l *lowerer) recvType(n psi.NodeRef) (*syntax.TypeRef, []syntax.Annotation) {
	return l.typeWith(n, true)
}

func (l *lowerer) typeWith(n psi.NodeRef, recv bool) (*syntax.TypeRef, []syntax.Annotation) {
	var (
		elem      psi.NodeRef
		quest     bool
		mods      []psi.NodeRef
		recvAs    []syntax.Annotation
		leading   = true // still before the type's other annotations
		suspendAt psi.NodeRef
	)
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindMODIFIER_LIST:
			mods = append(mods, c)
		case psi.KindUSER_TYPE, psi.KindFUNCTION_TYPE, psi.KindNULLABLE_TYPE, psi.KindDYNAMIC_TYPE,
			psi.KindINTERSECTION_TYPE, psi.KindTYPE_REFERENCE:
			elem = c
		case psi.KindQUEST:
			quest = true
		case psi.KindLPAR, psi.KindRPAR:
		default:
			l.unexpected(c)
		}
	}
	unsafe, composable, noInfer := false, false, false
	var wild syntax.Wildcards
	var as []syntax.Annotation
	for _, m := range mods {
		for _, c := range kids(m) {
			switch c.NumericKind() {
			case psi.KindANNOTATION_ENTRY:
				a := l.annotation(c)
				if a.Target == "receiver" {
					if !recv || !leading {
						l.failAt(a.P, "this annotation is not applicable to target 'type usage' and use-site target '@receiver'")
					}
					recvAs = append(recvAs, a)
					continue
				}
				leading = false
				as = append(as, a)
				unsafe = unsafe || a.Is("UnsafeVariance")
				noInfer = noInfer || a.Name == "NoInfer" || a.Is("internal.NoInfer")
				composable = composable || syntax.IsComposable(a)
				if w := syntax.WildcardsOf([]syntax.Annotation{a}); w != syntax.DefaultWildcards {
					wild = w
				}
			case psi.KindANNOTATION:
				leading = false
				as = append(as, l.bracketAnnotation(c))
			case psi.KindSUSPEND_KEYWORD:
				suspendAt = c
			default:
				l.unexpected(c)
			}
		}
	}
	if !elem.Valid() {
		l.fail(n, "expected a type")
	}
	t := l.typeElem(elem, recv)
	if quest {
		t.Nullable = true
	}
	if suspendAt.Valid() {
		if t.Ret == nil {
			l.fail(suspendAt, "expected a function type after 'suspend'")
		}
		t.P, t.Suspend = l.pos(suspendAt), true
	}
	if as != nil {
		t.Annotations = append(as, t.Annotations...)
	}
	t.UnsafeVariance = t.UnsafeVariance || unsafe
	t.NoInfer = t.NoInfer || noInfer
	t.Composable = t.Composable || composable
	if wild != syntax.DefaultWildcards {
		t.Wildcards = wild
	}
	return t, recvAs
}

// typeElem lowers a type element: a user type, a function type, a
// nullable or intersection type, or a type reference in parentheses.
func (l *lowerer) typeElem(n psi.NodeRef, recv bool) *syntax.TypeRef {
	switch n.NumericKind() {
	case psi.KindTYPE_REFERENCE, psi.KindNULLABLE_TYPE:
		t, _ := l.typeWith(n, false)
		return t
	case psi.KindUSER_TYPE:
		return l.userType(n)
	case psi.KindDYNAMIC_TYPE:
		return record(l, &syntax.TypeRef{P: l.pos(n), Name: "dynamic"}, n)
	case psi.KindINTERSECTION_TYPE:
		ks := kids(n)
		var left, right psi.NodeRef
		for _, c := range ks {
			if c.NumericKind() == psi.KindTYPE_REFERENCE {
				if !left.Valid() {
					left = c
				} else {
					right = c
				}
			}
		}
		t := l.typeRef(left)
		t.And = l.typeRef(right)
		return t
	case psi.KindFUNCTION_TYPE:
		ft := &syntax.TypeRef{P: l.pos(n)}
		record(l, ft, n)
		var recvNode psi.NodeRef
		for _, c := range kids(n) {
			switch c.NumericKind() {
			case psi.KindCONTEXT_PARAMETER_LIST:
				// A context function type: context(A, B) R.(C) -> T.
				ft.Contexts = []*syntax.TypeRef{}
				for _, k := range kids(c) {
					switch k.NumericKind() {
					case psi.KindCONTEXT_RECEIVER:
						ft.Contexts = append(ft.Contexts, l.typeRef(child(k, psi.KindTYPE_REFERENCE)))
					case psi.KindVALUE_PARAMETER:
						// kc reads only types here: context(a: A) () -> R
						// fails at the colon.
						l.fail(child(k, psi.KindCOLON), "expected ')', found ':'")
					}
				}
				if len(ft.Contexts) == 0 {
					l.fail(child(c, psi.KindRPAR), "expected a context parameter type")
				}
			case psi.KindFUNCTION_TYPE_RECEIVER:
				recvNode = c
			case psi.KindVALUE_PARAMETER_LIST:
				ft.Params = []*syntax.TypeRef{}
				for _, p := range kids(c) {
					if p.NumericKind() == psi.KindVALUE_PARAMETER {
						ft.Params = append(ft.Params, l.typeRef(child(p, psi.KindTYPE_REFERENCE)))
					}
				}
				if len(ft.Params) == 0 {
					ft.Params = nil
				}
			case psi.KindTYPE_REFERENCE:
				ft.Ret = l.typeRef(c)
			}
		}
		if recvNode.Valid() {
			ft.Recv = l.typeRef(child(recvNode, psi.KindTYPE_REFERENCE))
		}
		if ft.Ret == nil {
			l.fail(n, "expected '->'")
		}
		return ft
	}
	l.unexpected(n)
	return nil
}

// userType lowers a USER_TYPE: a.b.C<T>, or Outer<A>.Inner<B>, whose
// outer type is that of the generic class it is in.
func (l *lowerer) userType(n psi.NodeRef) *syntax.TypeRef {
	type seg struct {
		name string
		args psi.NodeRef
	}
	var segs []seg
	var flatten func(u psi.NodeRef)
	flatten = func(u psi.NodeRef) {
		var s seg
		for _, c := range kids(u) {
			switch c.NumericKind() {
			case psi.KindUSER_TYPE:
				flatten(c)
			case psi.KindREFERENCE_EXPRESSION:
				s.name = ident(first(c))
			case psi.KindTYPE_ARGUMENT_LIST:
				s.args = c
			case psi.KindDOT:
			default:
				l.unexpected(c)
			}
		}
		segs = append(segs, s)
	}
	flatten(n)
	ref := &syntax.TypeRef{P: l.pos(n), Name: segs[0].name}
	i := 0
	for segs[i].args == (psi.NodeRef{}) && i+1 < len(segs) {
		i++
		ref.Name += "." + segs[i].name
	}
	if segs[i].args.Valid() {
		ref.Args = l.typeArgs(segs[i].args)
	}
	for ref.Args != nil && i+1 < len(segs) {
		outer := ref
		i++
		ref = &syntax.TypeRef{P: outer.P, Name: outer.Name + "." + segs[i].name, Outer: outer}
		for !segs[i].args.Valid() && i+1 < len(segs) {
			i++
			ref.Name += "." + segs[i].name
		}
		if segs[i].args.Valid() {
			ref.Args = l.typeArgs(segs[i].args)
		}
	}
	return record(l, ref, n)
}

// typeArgs lowers a TYPE_ARGUMENT_LIST: <A, out B, in C, *>.
func (l *lowerer) typeArgs(n psi.NodeRef) []*syntax.TypeArg {
	args := []*syntax.TypeArg{}
	for _, c := range kids(n) {
		if c.NumericKind() != psi.KindTYPE_PROJECTION {
			continue
		}
		a := record(l, &syntax.TypeArg{P: l.pos(c)}, c)
		if has(c, psi.KindMUL) {
			a.Variance = types.Star
		} else {
			if m := child(c, psi.KindMODIFIER_LIST); m.Valid() {
				for _, k := range kids(m) {
					switch k.NumericKind() {
					case psi.KindIN_KEYWORD:
						a.Variance = types.In
					case psi.KindOUT_KEYWORD:
						a.Variance = types.Out
					}
				}
			}
			tr := child(c, psi.KindTYPE_REFERENCE)
			if a.Variance == types.Inv && tr.Text() == "_" {
				a.Underscore = true
			}
			a.Type = l.typeRef(tr)
			if m := child(c, psi.KindMODIFIER_LIST); m.Valid() {
				l.projectionAnnotations(m, a.Type)
			}
		}
		args = append(args, a)
	}
	return args
}

// projectionAnnotations applies the annotations in a type projection's
// modifiers to its type, as those before the type.
func (l *lowerer) projectionAnnotations(m psi.NodeRef, t *syntax.TypeRef) {
	for _, c := range kids(m) {
		if c.NumericKind() != psi.KindANNOTATION_ENTRY {
			continue
		}
		a := l.annotation(c)
		t.UnsafeVariance = t.UnsafeVariance || a.Is("UnsafeVariance")
		t.NoInfer = t.NoInfer || a.Name == "NoInfer" || a.Is("internal.NoInfer")
		t.Composable = t.Composable || syntax.IsComposable(a)
		if w := syntax.WildcardsOf([]syntax.Annotation{a}); w != syntax.DefaultWildcards {
			t.Wildcards = w
		}
	}
}

// typeParams lowers a TYPE_PARAMETER_LIST: <T, out U : Bound>.
func (l *lowerer) typeParams(n psi.NodeRef) []*syntax.TypeParam {
	if !n.Valid() {
		return nil
	}
	var ps []*syntax.TypeParam
	for _, c := range kids(n) {
		if c.NumericKind() != psi.KindTYPE_PARAMETER {
			continue
		}
		name := child(c, psi.KindIDENTIFIER)
		tp := record(l, &syntax.TypeParam{P: l.pos(name), Name: ident(name)}, c)
		if m := child(c, psi.KindMODIFIER_LIST); m.Valid() {
			for _, k := range kids(m) {
				switch k.NumericKind() {
				case psi.KindREIFIED_KEYWORD:
					tp.Reified = true
				case psi.KindIN_KEYWORD:
					tp.Variance = types.In
				case psi.KindOUT_KEYWORD:
					tp.Variance = types.Out
				case psi.KindANNOTATION_ENTRY:
					a := l.annotation(k) // kc writes no type annotations
					tp.Annotations = append(tp.Annotations, a)
					tp.OnlyInput = tp.OnlyInput || a.Is("internal.OnlyInputTypes") // for the inference
				case psi.KindANNOTATION:
					tp.Annotations = append(tp.Annotations, l.bracketAnnotation(k))
				case psi.KindCONTEXT_PARAMETER_LIST:
					l.fail(child(k, psi.KindLPAR), "expected '>', found '('")
				}
			}
		}
		if b := child(c, psi.KindTYPE_REFERENCE); b.Valid() {
			tp.Bounds = []*syntax.TypeRef{l.typeRef(b)}
		}
		ps = append(ps, tp)
	}
	return ps
}

// whereClause adds the bounds of a TYPE_CONSTRAINT_LIST to tps.
func (l *lowerer) whereClause(n psi.NodeRef, tps []*syntax.TypeParam) {
	if !n.Valid() {
		return
	}
	for _, c := range kids(n) {
		if c.NumericKind() != psi.KindTYPE_CONSTRAINT {
			continue
		}
		for _, k := range kids(c) {
			if k.NumericKind() == psi.KindANNOTATION_ENTRY {
				l.annotation(k)
			}
		}
		ref := child(c, psi.KindREFERENCE_EXPRESSION)
		name := ident(first(ref))
		var tp *syntax.TypeParam
		for _, q := range tps {
			if q.Name == name {
				tp = q
			}
		}
		if tp == nil {
			l.fail(ref, "'%s' does not refer to a type parameter of this declaration", name)
		}
		tp.Bounds = append(tp.Bounds, l.typeRef(child(c, psi.KindTYPE_REFERENCE)))
	}
}

// ---- annotations and modifiers ----

// annotation lowers an ANNOTATION_ENTRY.
func (l *lowerer) annotation(n psi.NodeRef) syntax.Annotation {
	a := syntax.Annotation{P: l.pos(n)}
	l.annots[a.P] = n.ID()
	if t := child(n, psi.KindANNOTATION_TARGET); t.Valid() {
		a.Target = t.Text()
	}
	callee := child(n, psi.KindCONSTRUCTOR_CALLEE)
	u := child(child(callee, psi.KindTYPE_REFERENCE), psi.KindUSER_TYPE)
	for _, s := range l.segments(u) {
		if a.Name != "" {
			a.Name += "."
		}
		a.Name += ident(first(s))
		if ta := child(s.Parent(), psi.KindTYPE_ARGUMENT_LIST); ta.Valid() {
			a.TypeArgs = l.typeArgs(ta)
		}
	}
	if args := child(n, psi.KindVALUE_ARGUMENT_LIST); args.Valid() {
		if pv := args.PreviousSibling(); pv.Valid() && pv.IsTrivia() {
			l.fail(args, "unexpected '('")
		}
		a.Args = l.annotationArgs(args)
	}
	inv, sh, dsl := syntax.Suppressions(&a)
	l.suppressInvisible = l.suppressInvisible || inv
	l.suppressShadowed = l.suppressShadowed || sh
	l.suppressDSL = l.suppressDSL || dsl
	return a
}

// segments returns the REFERENCE_EXPRESSIONs of a USER_TYPE's names, in
// order.
func (l *lowerer) segments(u psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	for _, c := range kids(u) {
		switch c.NumericKind() {
		case psi.KindUSER_TYPE:
			out = append(out, l.segments(c)...)
		case psi.KindREFERENCE_EXPRESSION:
			out = append(out, c)
		}
	}
	return out
}

// annotationArgs lowers an annotation's VALUE_ARGUMENT_LIST to its
// arguments' tokens.
func (l *lowerer) annotationArgs(n psi.NodeRef) []syntax.AnnotationArg {
	var args []syntax.AnnotationArg
	for _, c := range kids(n) {
		if c.NumericKind() != psi.KindVALUE_ARGUMENT {
			continue
		}
		var arg syntax.AnnotationArg
		start := first(c)
		if start.NumericKind() == psi.KindVALUE_ARGUMENT_NAME {
			arg.Name = ident(first(first(start)))
			start = next(next(start)) // after the '='
		}
		psi.WalkNodes(c, func(k psi.NodeRef) bool {
			if k.NumericKind() == psi.KindCALL_EXPRESSION {
				if list := child(k, psi.KindVALUE_ARGUMENT_LIST); list.Valid() {
					l.annots[l.pos(list)] = k.ID()
				}
			}
			return true
		})
		if start.Valid() {
			from := l.tokenStart(start)
			toks, _ := syntax.TokenizeAt(l.path, l.src[from:c.Range().End], l.posAt(from))
			for _, t := range toks {
				if t.Kind != syntax.NL && t.Kind != syntax.EOF {
					arg.Toks = append(arg.Toks, t)
				}
			}
		}
		args = append(args, arg)
	}
	return args
}

// modifiers lowers a MODIFIER_LIST, as kc's parser reads a declaration's
// modifiers. n may be invalid: no modifiers.
func (l *lowerer) modifiers(n psi.NodeRef) syntax.Modifiers {
	if !n.Valid() {
		return syntax.Modifiers{}
	}
	return l.modifiersOf(kids(n))
}

// modifiersOf lowers the children of a MODIFIER_LIST.
func (l *lowerer) modifiersOf(cs []psi.NodeRef) syntax.Modifiers {
	var m syntax.Modifiers
	for _, c := range cs {
		if c.IsLeaf() && syntax.IsModifier(c.Text()) {
			m.Written = append(m.Written, syntax.Mod{Name: c.Text(), P: l.pos(c)})
		}
		k := c.NumericKind()
		if k == psi.KindIDENTIFIER {
			// A companion object's detached name: see detachName.
			k = psi.KindForName(strings.ToUpper(c.Text()) + "_KEYWORD")
		}
		switch k {
		case psi.KindANNOTATION_ENTRY:
			a := l.annotation(c)
			if a.Target == "" && a.Is("JvmInline") {
				m.JvmInline = true
			}
			m.Annotations = append(m.Annotations, a)
		case psi.KindANNOTATION:
			// @[A B]: kc skips the annotations in brackets.
		case psi.KindCONTEXT_PARAMETER_LIST:
			if m.Contexts != nil {
				l.fail(c, "repeated context parameter lists")
			}
			m.Contexts, m.ContextP = l.contextParams(c), l.pos(c)
		case psi.KindFUN_KEYWORD:
			// fun interface: the class sets Fun.
		case psi.KindPRIVATE_KEYWORD:
			m.Visibility, m.Explicit = syntax.Private, true
		case psi.KindPUBLIC_KEYWORD:
			m.Visibility, m.Explicit = syntax.Public, true
		case psi.KindINTERNAL_KEYWORD:
			m.Visibility, m.Explicit = syntax.Internal, true
		case psi.KindPROTECTED_KEYWORD:
			m.Visibility, m.Explicit = syntax.Protected, true
		case psi.KindOPEN_KEYWORD:
			m.Open = true
		case psi.KindABSTRACT_KEYWORD:
			m.Abstract = true
		case psi.KindOVERRIDE_KEYWORD:
			m.Override = true
		case psi.KindFINAL_KEYWORD:
			m.Final = true
		case psi.KindCOMPANION_KEYWORD:
			m.Companion = true
		case psi.KindDATA_KEYWORD:
			m.Data = true
		case psi.KindENUM_KEYWORD:
			m.Enum = true
		case psi.KindSEALED_KEYWORD:
			m.Sealed = true
		case psi.KindCONST_KEYWORD:
			m.Const = true
		case psi.KindVARARG_KEYWORD:
			m.Vararg, m.VarargP = true, l.pos(c)
		case psi.KindOPERATOR_KEYWORD:
			m.Operator = true
		case psi.KindINFIX_KEYWORD:
			m.Infix = true
		case psi.KindEXPECT_KEYWORD:
			m.Expect, m.ExpectP = true, l.pos(c)
		case psi.KindACTUAL_KEYWORD:
			m.Actual = true
		case psi.KindANNOTATION_KEYWORD:
			m.Annotation = true
		case psi.KindVALUE_KEYWORD:
			m.Value = true
		case psi.KindINLINE_KEYWORD:
			m.Inline = true
		case psi.KindNOINLINE_KEYWORD:
			m.Noinline = true
		case psi.KindCROSSINLINE_KEYWORD:
			m.Crossinline = true
		case psi.KindSUSPEND_KEYWORD:
			m.Suspend = true
		case psi.KindLATEINIT_KEYWORD:
			m.Lateinit = true
		case psi.KindINNER_KEYWORD:
			m.Inner = true
		case psi.KindTAILREC_KEYWORD:
			m.Tailrec = true
		case psi.KindEXTERNAL_KEYWORD:
			m.External, m.ExternalP = true, l.pos(c)
		default:
			l.fail(c, "'%s' is not supported", c.Text())
		}
	}
	return m
}

// paramOnly reports modifiers that only parameters take on another
// declaration.
func (l *lowerer) paramOnly(m syntax.Modifiers) {
	if m.Vararg {
		l.failAt(m.VarargP, "modifier 'vararg' is only allowed on parameters")
	}
}

// contextParams lowers a CONTEXT_PARAMETER_LIST of a declaration, as kc's
// parser reads context(a: A, _: B): every entry must be named.
func (l *lowerer) contextParams(n psi.NodeRef) []*syntax.Param {
	ps := []*syntax.Param{}
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindCONTEXT_RECEIVER:
			l.fail(c, "context parameters must be named: use '_' for an anonymous one, context(_: Type)")
		case psi.KindVALUE_PARAMETER:
		default:
			continue
		}
		p := &syntax.Param{}
		if mods := child(c, psi.KindMODIFIER_LIST); mods.Valid() {
			for _, k := range kids(mods) {
				switch k.NumericKind() {
				case psi.KindANNOTATION_ENTRY:
					p.Annotations = append(p.Annotations, l.annotation(k))
				case psi.KindANNOTATION:
				case psi.KindNOINLINE_KEYWORD:
					p.Noinline = true
				case psi.KindCROSSINLINE_KEYWORD:
					p.Crossinline = true
				case psi.KindCONTEXT_PARAMETER_LIST:
					l.unexpected(k)
				default: // a modifier keyword
					l.errorLater(l.pos(k), "modifier '%s' is not applicable to 'context parameter'", k.Text())
				}
			}
		}
		name := child(c, psi.KindIDENTIFIER)
		if !name.Valid() || prev(name).NumericKind() == psi.KindVAL_KEYWORD || prev(name).NumericKind() == psi.KindVAR_KEYWORD {
			l.fail(c, "context parameters must be named: use '_' for an anonymous one, context(_: Type)")
		}
		tr := child(c, psi.KindTYPE_REFERENCE)
		if !tr.Valid() {
			l.failAt(l.end(name), "expected ':'")
		}
		p.P, p.Name, p.Type = l.pos(name), ident(name), l.typeRef(tr)
		if eq := child(c, psi.KindEQ); eq.Valid() {
			l.errorLater(p.P, "context parameters cannot have default values")
			l.expr(exprAfter(c, psi.KindEQ))
		}
		ps = append(ps, record(l, p, c))
	}
	if len(ps) == 0 {
		l.fail(child(n, psi.KindRPAR), "expected a context parameter")
	}
	return ps
}

// noContexts reports context parameters on a declaration that Kotlin
// doesn't allow them on, what, as kc's parser does, and drops them.
func (l *lowerer) noContexts(m *syntax.Modifiers, what string) {
	if m.Contexts != nil {
		l.errorLater(m.ContextP, "context parameters on %s are unsupported", what)
		m.Contexts = nil
	}
}
