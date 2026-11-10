package lower

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// block lowers a BLOCK in braces.
func (l *lowerer) block(n psi.NodeRef) *syntax.Block {
	if !n.Valid() {
		l.failAt(l.posAt(len(l.src)), "expected '{'")
	}
	lb, rb := child(n, psi.KindLBRACE), child(n, psi.KindRBRACE)
	if !lb.Valid() {
		l.fail(n, "expected '{'")
	}
	if !rb.Valid() {
		l.failAt(l.end(n), "expected '}'")
	}
	b := &syntax.Block{P: l.pos(lb), End: l.pos(rb), Stmts: l.stmts(n)}
	return record(l, b, n)
}

// stmts lowers the statements of a BLOCK.
func (l *lowerer) stmts(n psi.NodeRef) []syntax.Stmt {
	var out []syntax.Stmt
	for _, c := range kids(n) {
		switch c.NumericKind() {
		case psi.KindLBRACE, psi.KindRBRACE, psi.KindSEMICOLON:
			continue
		}
		out = append(out, l.stmt(c))
	}
	return out
}

// body lowers a control structure's body, a THEN, ELSE or BODY: a block,
// or a single statement.
func (l *lowerer) body(n psi.NodeRef) *syntax.Block {
	c := first(n)
	if !c.Valid() {
		// while (f());: the body is empty, at the semicolon.
		return record(l, &syntax.Block{P: l.pos(n)}, n)
	}
	return l.bodyOf(c)
}

// bodyOf lowers statement n as a control structure's body.
func (l *lowerer) bodyOf(n psi.NodeRef) *syntax.Block {
	if n.NumericKind() == psi.KindBLOCK {
		return l.block(n)
	}
	if n.NumericKind() == psi.KindANNOTATED_EXPRESSION {
		// -> @Suppress("x") { ... }: a block, whose annotations are
		// dropped.
		if x := exprChild(n); x.NumericKind() == psi.KindBLOCK {
			l.annotationsOf(n)
			return l.block(x)
		}
	}
	return record(l, &syntax.Block{P: l.pos(n), Stmts: []syntax.Stmt{l.stmt(n)}}, n)
}

// stmtModifiers splits the modifiers of a local declaration as kc reads
// them: the annotations before the first modifier keyword, which it
// reads as a statement's, and the modifiers from that keyword on. It
// returns the keyword's position, if there is one.
func (l *lowerer) stmtModifiers(n psi.NodeRef) (as []syntax.Annotation, m syntax.Modifiers, kw syntax.Pos, hasKw bool) {
	cs := kids(child(n, psi.KindMODIFIER_LIST))
	i := 0
	for ; i < len(cs); i++ {
		switch cs[i].NumericKind() {
		case psi.KindANNOTATION_ENTRY:
			as = append(as, l.annotation(cs[i]))
			continue
		case psi.KindANNOTATION:
			as = append(as, l.bracketAnnotation(cs[i]))
			continue
		}
		break
	}
	if i < len(cs) {
		kw, hasKw = l.pos(cs[i]), true
	}
	m = l.modifiersOf(cs[i:])
	return
}

// leftOperandKinds are the expressions whose first child is their left
// operand, where a statement's leading annotations end up in psi.
var leftOperandKinds = map[psi.Kind]bool{
	psi.KindBINARY_EXPRESSION: true, psi.KindBINARY_WITH_TYPE: true, psi.KindIS_EXPRESSION: true,
	psi.KindDOT_QUALIFIED_EXPRESSION: true, psi.KindSAFE_ACCESS_EXPRESSION: true, psi.KindCALL_EXPRESSION: true,
	psi.KindARRAY_ACCESS_EXPRESSION: true, psi.KindPOSTFIX_EXPRESSION: true,
	psi.KindCALLABLE_REFERENCE_EXPRESSION: true, psi.KindCLASS_LITERAL_EXPRESSION: true,
}

// hoist returns the annotations that start statement n: kc reads them
// as the statement's, where psi puts them on its leftmost operand.
func (l *lowerer) hoist(n psi.NodeRef) []syntax.Annotation {
	var as []syntax.Annotation
	for cur := n; cur.Valid(); {
		for leftOperandKinds[cur.NumericKind()] && isExpr(first(cur)) {
			cur = first(cur)
		}
		if cur.NumericKind() != psi.KindANNOTATED_EXPRESSION {
			break
		}
		as = append(as, l.annotationsOf(cur)...)
		l.hoisted[cur.ID()] = true
		cur = exprChild(cur)
	}
	return as
}

// stmt lowers a statement.
func (l *lowerer) stmt(n psi.NodeRef) syntax.Stmt {
	switch n.NumericKind() {
	case psi.KindPROPERTY:
		return l.localVar(n)
	case psi.KindDESTRUCTURING_DECLARATION:
		as, m, _, hasKw := l.stmtModifiers(n)
		if m.Contexts != nil {
			l.failAt(m.ContextP, "context parameters are unsupported in this position")
		}
		kw := child(n, psi.KindVAL_KEYWORD)
		if !kw.Valid() {
			kw = child(n, psi.KindVAR_KEYWORD)
		}
		d := &syntax.Destructure{P: l.pos(kw), Mutable: kw.NumericKind() == psi.KindVAR_KEYWORD}
		d.Names, d.Types = l.destructuringNames(n)
		if !has(n, psi.KindEQ) {
			l.failAt(l.end(n), "expected '='")
		}
		d.Init = l.expr(exprAfter(n, psi.KindEQ))
		if hasKw {
			syntax.Annotate(&l.annotated, d, m.Annotations)
		}
		syntax.Annotate(&l.annotated, d, as)
		return record(l, d, n)
	case psi.KindFUN:
		if !has(n, psi.KindIDENTIFIER) {
			break
		}
		as, m, _, _ := l.stmtModifiers(n)
		lf := l.localFun(n, m)
		lf.Decl.Annotations = append(as, lf.Decl.Annotations...)
		return record(l, lf, n)
	case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
		as, m, _, hasKw := l.stmtModifiers(n)
		if kw := child(n, psi.KindOBJECT_KEYWORD); kw.Valid() && hasKw {
			l.fail(kw, "named object cannot be local: use an anonymous object instead")
		}
		lc := l.localClass(n, m)
		lc.Annotations = append(as, lc.Annotations...)
		return record(l, lc, n)
	case psi.KindTYPEALIAS:
		l.fail(n, "unexpected typealias")
	}

	as := l.hoist(n)
	s := l.stmt1(n)
	if es, ok := s.(*syntax.ExprStmt); ok {
		if lam, ok := es.X.(*syntax.Lambda); ok {
			for _, a := range as {
				syntax.AnnotateLambda(lam, a)
			}
		}
		syntax.Annotate(&l.annotated, es.X, as)
	} else {
		syntax.Annotate(&l.annotated, s, as)
	}
	return s
}

func (l *lowerer) stmt1(n psi.NodeRef) syntax.Stmt {
	switch n.NumericKind() {
	case psi.KindLABELED_EXPRESSION:
		x := exprChild(n)
		switch x.NumericKind() {
		case psi.KindFOR, psi.KindWHILE, psi.KindDO_WHILE:
			s := l.stmt1(x)
			switch s := s.(type) {
			case *syntax.For:
				s.Label = l.label(n)
			case *syntax.While:
				s.Label = l.label(n)
			}
			return s
		}
	case psi.KindFOR:
		f := &syntax.For{P: l.pos(n)}
		vp := child(n, psi.KindVALUE_PARAMETER)
		if !vp.Valid() {
			l.failAt(l.end(n), "expected a name")
		}
		if m := child(vp, psi.KindMODIFIER_LIST); m.Valid() {
			l.unexpected(m)
		}
		if d := child(vp, psi.KindDESTRUCTURING_DECLARATION); d.Valid() {
			f.Names, f.Types = l.destructuringNames(d)
		} else {
			f.Var = ident(child(vp, psi.KindIDENTIFIER))
			if t := child(vp, psi.KindTYPE_REFERENCE); t.Valid() {
				f.Type = l.typeRef(t)
			}
		}
		f.Range = l.expr(exprChild(child(n, psi.KindLOOP_RANGE)))
		f.Body = l.loopBody(n)
		return record(l, f, n)
	case psi.KindWHILE:
		w := &syntax.While{P: l.pos(n), Cond: l.expr(exprChild(child(n, psi.KindCONDITION)))}
		w.Body = l.loopBody(n)
		return record(l, w, n)
	case psi.KindDO_WHILE:
		w := &syntax.While{P: l.pos(n), Do: true}
		if b := child(n, psi.KindBODY); b.Valid() {
			w.Body = l.body(b)
		} else {
			w.Body = &syntax.Block{P: l.pos(child(n, psi.KindWHILE_KEYWORD))}
		}
		w.Cond = l.expr(exprChild(child(n, psi.KindCONDITION)))
		return record(l, w, n)
	case psi.KindBINARY_EXPRESSION:
		op := opOf(n)
		if k, ok := assignOps[op.NumericKind()]; ok {
			ops := operands(n)
			if len(ops) != 2 {
				l.failAt(l.end(op), "expected an expression")
			}
			a := &syntax.Assign{P: l.pos(op), Op: k, Target: l.expr(ops[0]), Value: l.expr(ops[1])}
			return record(l, a, n)
		}
	}
	return &syntax.ExprStmt{X: l.expr(n)}
}

// loopBody lowers the BODY of a for or while loop.
func (l *lowerer) loopBody(n psi.NodeRef) *syntax.Block {
	b := child(n, psi.KindBODY)
	if !b.Valid() {
		l.failAt(l.end(n), "expected an expression")
	}
	return l.body(b)
}

// localVar lowers a local PROPERTY.
func (l *lowerer) localVar(n psi.NodeRef) *syntax.VarStmt {
	as, m, kwPos, hasKw := l.stmtModifiers(n)
	if m.Contexts != nil {
		l.failAt(m.ContextP, "context parameters are unsupported in this position")
	}
	kw := child(n, psi.KindVAL_KEYWORD)
	if !kw.Valid() {
		kw = child(n, psi.KindVAR_KEYWORD)
	}
	name := child(n, psi.KindIDENTIFIER)
	if !name.Valid() {
		l.failAt(l.end(kw), "expected a name")
	}
	for _, k := range []psi.Kind{psi.KindTYPE_PARAMETER_LIST, psi.KindPROPERTY_ACCESSOR, psi.KindTYPE_CONSTRAINT_LIST} {
		if c := child(n, k); c.Valid() {
			l.unexpected(c)
		}
	}
	if c := prev(name); c.NumericKind() == psi.KindDOT {
		l.unexpected(c)
	}
	vs := &syntax.VarStmt{P: l.pos(kw), Name: ident(name), Mutable: kw.NumericKind() == psi.KindVAR_KEYWORD}
	record(l, vs, n)
	if t := child(n, psi.KindTYPE_REFERENCE); t.Valid() {
		vs.Type = l.typeRef(t)
	}
	switch {
	case has(n, psi.KindPROPERTY_DELEGATE):
		vs.Delegate = l.expr(exprAfter(child(n, psi.KindPROPERTY_DELEGATE), psi.KindBY_KEYWORD))
	case has(n, psi.KindEQ):
		vs.Init = l.expr(exprAfter(n, psi.KindEQ))
	case vs.Type == nil:
		l.fail(name, "this variable must either have an explicit type or be initialized")
	}
	if hasKw {
		vs.Lateinit, vs.LateinitP = m.Lateinit, kwPos
		vs.Mods = m.Written
		syntax.Annotate(&l.annotated, vs, m.Annotations)
	}
	syntax.Annotate(&l.annotated, vs, as)
	return vs
}
