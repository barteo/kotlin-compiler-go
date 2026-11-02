package analysis

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// analyze lowers, checks and exports the facts of snapshots.
func analyze(ctx context.Context, cfg Config, snaps []*psi.Snapshot) (*Result, error) {
	jobs := max(cfg.Jobs, 1)
	n := len(snaps)
	files := make([]*syntax.File, n)
	provs := make([]*lower.Provenance, n)
	errLists := make([][]*syntax.Error, n)
	parallel(n, jobs, func(i int) {
		files[i], provs[i], errLists[i] = lower.File(snaps[i])
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res := &Result{facts: map[ElementID]*Fact{}}
	for i, es := range errLists {
		for _, e := range es {
			res.diags = append(res.diags, diagnostic(snaps[i].Path(), e))
		}
	}
	if len(res.diags) > 0 {
		return res, nil
	}
	if d := duplicateClasses(files); d != nil {
		res.diags = append(res.diags, *d)
		return res, nil
	}

	cp, lib, err := driver.OpenLibrary(driver.Options{
		JDKHome: cfg.JDKHome, NoJDK: cfg.NoJDK, ClassPath: cfg.ClassPath,
		ModuleName: cfg.ModuleName, Jobs: jobs,
	})
	if err != nil {
		return nil, fmt.Errorf("analysis: %w", err)
	}
	defer cp.Close()
	rec := &recorder{when: map[*syntax.When]whenReport{}, stmt: map[syntax.Stmt]bool{}, unreachable: map[syntax.Stmt]bool{}, annots: map[*syntax.File]map[syntax.Pos]*types.ClassSym{}, elvis: map[*syntax.Binary]bool{}, typeRefs: map[*syntax.TypeRef]types.Type{}, annotClass: map[*syntax.Annotation]*types.ClassSym{}}
	errs, err := check.CheckContext(ctx, files, lib, jobs, rec)
	if err != nil {
		return nil, err
	}
	for _, e := range errs {
		res.diags = append(res.diags, diagnostic(e.File, e))
	}
	if len(res.diags) > 0 {
		return res, nil
	}

	b := newBuilder(res, snaps, files, provs, rec)
	b.cp = cp
	b.build()
	return res, nil
}

func parallel(n, jobs int, fn func(i int)) {
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(jobs, n) {
		wg.Go(func() {
			for i := range next {
				fn(i)
			}
		})
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}

func diagnostic(path string, e *syntax.Error) Diagnostic {
	if e.File != "" {
		path = e.File
	}
	code := "source_error"
	if e.Feature != "" {
		code = "unsupported:" + e.Feature
	}
	return Diagnostic{Path: path, Line: e.Pos.Line, Column: e.Pos.Col, Code: code, Message: e.Msg}
}

// duplicateClasses reports two files with the same facade class, which
// the checker assumes there aren't.
func duplicateClasses(files []*syntax.File) *Diagnostic {
	classes := map[string]string{}
	multifile := map[string]bool{}
	for _, f := range files {
		if !f.HasFacade() {
			continue
		}
		if prev, dup := classes[f.Class()]; dup {
			return &Diagnostic{Path: f.Path, Line: 1, Column: 1, Code: "source_error",
				Message: fmt.Sprintf("duplicate JVM class name %s (also from %s)", f.Class(), prev)}
		}
		classes[f.Class()] = f.Path
		if f.Part != "" {
			multifile[f.Facade] = true
		}
	}
	for name := range multifile {
		if prev, dup := classes[name]; dup {
			return &Diagnostic{Path: prev, Line: 1, Column: 1, Code: "source_error",
				Message: fmt.Sprintf("duplicate JVM class name %s (also a multi-file class)", name)}
		}
	}
	return nil
}

type whenReport struct{ exhaustive, used bool }

// recorder keeps what the checker reports, the last report of each node.
type recorder struct {
	mu          sync.Mutex
	when        map[*syntax.When]whenReport
	stmt        map[syntax.Stmt]bool
	unreachable map[syntax.Stmt]bool
	// annots are the classes of each file's annotations, by position.
	annots map[*syntax.File]map[syntax.Pos]*types.ClassSym
	elvis  map[*syntax.Binary]bool
	// typeRefs are the types of the type references the checker resolved.
	typeRefs map[*syntax.TypeRef]types.Type
	// annotClass are the classes of the annotations, by annotation.
	annotClass map[*syntax.Annotation]*types.ClassSym
}

func (r *recorder) When(x *syntax.When, exhaustive, valueUsed bool) {
	r.mu.Lock()
	r.when[x] = whenReport{exhaustive, valueUsed}
	r.mu.Unlock()
}

func (r *recorder) Stmt(s syntax.Stmt, valueUsed bool) {
	r.mu.Lock()
	r.stmt[s] = valueUsed
	r.mu.Unlock()
}

func (r *recorder) Unreachable(s syntax.Stmt) {
	r.mu.Lock()
	r.unreachable[s] = true
	r.mu.Unlock()
}

func (r *recorder) Elvis(x *syntax.Binary, nonNullLeftSubtypeRight bool) {
	r.mu.Lock()
	r.elvis[x] = nonNullLeftSubtypeRight
	r.mu.Unlock()
}

func (r *recorder) TypeRef(ref *syntax.TypeRef, t types.Type) {
	r.mu.Lock()
	r.typeRefs[ref] = t
	r.mu.Unlock()
}

func (r *recorder) Annotation(f *syntax.File, a *syntax.Annotation, cls *types.ClassSym) {
	r.mu.Lock()
	m := r.annots[f]
	if m == nil {
		m = map[syntax.Pos]*types.ClassSym{}
		r.annots[f] = m
	}
	m[a.P] = cls
	r.annotClass[a] = cls
	r.mu.Unlock()
}

// builder walks the checked AST and writes facts on the psi nodes it was
// lowered from.
type builder struct {
	res   *Result
	snaps []*psi.Snapshot
	files []*syntax.File
	provs []*lower.Provenance
	rec   *recorder
	where map[any]ElementID // AST nodes' psi nodes
	cv    converter

	// names are the fully qualified names of the functions the checker
	// makes up for the standard library's preconditions.
	names map[*types.FunSym]string

	// cp is the class path, and pkgAnnots the annotations of its
	// packages, by internal name, as their package-info classes keep.
	cp        *classpath.Path
	pkgAnnots map[string][]string

	symIDs map[any]uint32
	syms   []any // *types.VarSym, *types.FunSym or *types.ClassSym, by ID-1

	// negatedAt are the PREFIX_EXPRESSIONs of literals folded from them:
	// -1 is the literal's statement.
	negatedAt map[*syntax.IntLit]ElementID

	// receivers are the declarations of receivers: lambdas' and
	// extension functions'.
	receivers map[*types.VarSym]ElementID
	// classAt are the classes and objects the program declares, by
	// their declarations' elements.
	classAt map[ElementID]*types.ClassSym
	// later are visits that need every node's facts: return and receiver
	// targets.
	later []func()
}

func newBuilder(res *Result, snaps []*psi.Snapshot, files []*syntax.File, provs []*lower.Provenance, rec *recorder) *builder {
	b := &builder{res: res, snaps: snaps, files: files, provs: provs, rec: rec,
		where: map[any]ElementID{}, names: map[*types.FunSym]string{}, symIDs: map[any]uint32{}, receivers: map[*types.VarSym]ElementID{}, classAt: map[ElementID]*types.ClassSym{}, negatedAt: map[*syntax.IntLit]ElementID{}, pkgAnnots: map[string][]string{}}
	b.cv = converter{decl: b.decl, supers: map[*types.ClassSym][]string{}}
	return b
}

type entry struct {
	n    any
	id   psi.NodeID
	rank int
}

// rank orders the AST nodes lowered from one psi node: the first visited
// wins where they disagree. A delegated property's generated accessors
// come from its PROPERTY too.
func rank(n any) int {
	switch n.(type) {
	case *syntax.PropDecl, *syntax.CtorParam, *syntax.ClassDecl:
		return 0
	case *syntax.FunDecl, *syntax.Param:
		return 1
	case *syntax.Call:
		return 2
	case *syntax.Member, *syntax.Ident:
		return 3
	}
	return 4
}

func (b *builder) build() {
	entries := make([][]entry, len(b.snaps))
	for i, p := range b.provs {
		sid := b.snaps[i].ID()
		p.Each(func(n any, id psi.NodeID) {
			b.where[n] = ElementID{sid, id}
			entries[i] = append(entries[i], entry{n, id, rank(n)})
		})
		slices.SortFunc(entries[i], func(a, c entry) int {
			if r := cmp.Compare(a.id, c.id); r != 0 {
				return r
			}
			return cmp.Compare(a.rank, c.rank)
		})
	}
	for i, es := range entries {
		sid := b.snaps[i].ID()
		for _, e := range es {
			b.visit(e.n, ElementID{sid, e.id})
		}
	}
	// The other type references the checker resolved, after the
	// declarations' own, which win: type arguments, function types'
	// parts, typealiases, casts and bounds.
	for _, es := range entries {
		for _, e := range es {
			if r, ok := e.n.(*syntax.TypeRef); ok {
				if t, ok := b.rec.typeRefs[r]; ok {
					b.typeRef(r, t)
				}
			}
		}
	}
	for _, f := range b.later {
		f()
	}
	b.negated()
	b.usedAsExpression()
	b.wrappers()
	b.unreachable()
	b.annotations()
	b.symbols()

	for id := range b.res.facts {
		b.res.order = append(b.res.order, id)
	}
	index := map[psi.SnapshotID]int{}
	for i, s := range b.snaps {
		index[s.ID()] = i
	}
	slices.SortFunc(b.res.order, func(a, c ElementID) int {
		if r := cmp.Compare(index[a.Snapshot], index[c.Snapshot]); r != 0 {
			return r
		}
		return cmp.Compare(a.Node, c.Node)
	})
}

// fact returns the fact of el, adding it.
func (b *builder) fact(el ElementID) *Fact {
	f := b.res.facts[el]
	if f == nil {
		f = &Fact{Element: el}
		b.res.facts[el] = f
	}
	return f
}

// decl returns the psi node of AST node d, or nil.
func (b *builder) decl(d any) *ElementID {
	if d == nil {
		return nil
	}
	if el, ok := b.where[d]; ok {
		return &el
	}
	return nil
}

func (b *builder) node(el ElementID) psi.NodeRef {
	for _, s := range b.snaps {
		if s.ID() == el.Snapshot {
			return s.Node(el.Node)
		}
	}
	return psi.NodeRef{}
}

// sym returns the ID of symbol s, interning it; 0 for nil.
func (b *builder) sym(s any) uint32 {
	switch s := s.(type) {
	case *types.VarSym:
		if s == nil {
			return 0
		}
	case *types.FunSym:
		if s == nil {
			return 0
		}
	case *types.ClassSym:
		if s == nil {
			return 0
		}
	default:
		return 0
	}
	if id, ok := b.symIDs[s]; ok {
		return id
	}
	b.syms = append(b.syms, s)
	id := uint32(len(b.syms))
	b.symIDs[s] = id
	return id
}

// merge sets the fields of f that are unset to src's.
func merge(f *Fact, src Fact) {
	if f.Role == "" {
		f.Role = src.Role
	}
	if f.Type == nil {
		f.Type = src.Type
	}
	if f.Symbol == 0 {
		f.Symbol = src.Symbol
	}
	if f.Call == nil {
		f.Call = src.Call
	}
	if f.Builtin == "" {
		f.Builtin = src.Builtin
	}
	f.Classifier = f.Classifier || src.Classifier
	if f.ImportableName == "" {
		f.ImportableName = src.ImportableName
	}
	if f.When == nil {
		f.When = src.When
	}
	if !f.FlowNonNull && !f.DefinitelyNonNull {
		f.FlowNonNull, f.DefinitelyNonNull = src.FlowNonNull, src.DefinitelyNonNull
	}
	f.UsedAsExpression = f.UsedAsExpression || src.UsedAsExpression
	f.ImpossibleCast = f.ImpossibleCast || src.ImpossibleCast
	f.Unreachable = f.Unreachable || src.Unreachable
	f.SuspendOperation = f.SuspendOperation || src.SuspendOperation
	if f.LambdaParameters == nil {
		f.LambdaParameters, f.ImplicitParameter = src.LambdaParameters, src.ImplicitParameter
	}
	if f.ReturnTarget == nil {
		f.ReturnTarget = src.ReturnTarget
	}
	if f.ImplicitReceiver == nil {
		f.ImplicitReceiver = src.ImplicitReceiver
	}
	if f.ReceiverTarget == nil {
		f.ReceiverTarget = src.ReceiverTarget
	}
	if f.DelegateType == nil {
		f.DelegateType = src.DelegateType
	}
	if f.DelegatedCalls == nil {
		f.DelegatedCalls = src.DelegatedCalls
	}
	if f.NonNullLeftSubtypeRight == nil {
		f.NonNullLeftSubtypeRight = src.NonNullLeftSubtypeRight
	}
	if f.Deprecated == nil {
		f.Deprecated = src.Deprecated
	}
}

func (b *builder) set(el ElementID, src Fact) {
	merge(b.fact(el), src)
}

// typeRef sets the type of a declaration's type reference.
func (b *builder) typeRef(r *syntax.TypeRef, t types.Type) {
	if r == nil {
		return
	}
	el, ok := b.where[r]
	if !ok {
		return
	}
	ct := b.cv.typ(t)
	if ct == nil {
		return
	}
	f := Fact{Role: "type", Type: ct}
	if t.K == types.Class && b.node(el).Kind() == "USER_TYPE" {
		f.Deprecated = deprecated(classDeprecation(t.C))
	}
	b.set(el, f)
	// The type is recorded on the USER_TYPE; the TYPE_REFERENCE around it,
	// through a NULLABLE_TYPE, is the same type.
	for n := b.node(el).Parent(); n.Valid(); n = n.Parent() {
		k := n.Kind()
		if k != "NULLABLE_TYPE" && k != "TYPE_REFERENCE" {
			break
		}
		if k == "TYPE_REFERENCE" {
			b.set(ElementID{Snapshot: el.Snapshot, Node: n.ID()}, Fact{Role: "type", Type: copyType(ct)})
			break
		}
	}
}

func (b *builder) visit(n any, el ElementID) {
	switch x := n.(type) {
	case *syntax.PropDecl:
		b.propDecl(x, el)
	case *syntax.FunDecl:
		if x.Sym == nil {
			return
		}
		b.set(el, Fact{Role: "declaration", Symbol: b.sym(x.Sym)})
		b.typeRef(x.Ret, x.Sym.Ret)
		if x.Recv != nil {
			b.typeRef(x.Recv, x.Sym.Recv)
		}
		if x.RecvSym != nil {
			b.receivers[x.RecvSym] = el
		}
	case *syntax.ClassDecl:
		if x.Sym != nil {
			b.set(el, Fact{Role: "declaration", Symbol: b.sym(x.Sym)})
			b.classAt[el] = x.Sym
		}
	case *syntax.CtorParam:
		s := x.Sym
		if x.IsProp && x.PropSym != nil {
			s = x.PropSym
		}
		if s != nil {
			b.set(el, Fact{Role: "declaration", Symbol: b.sym(s)})
			if !x.Param.Vararg {
				b.typeRef(x.Type, s.Type)
			}
		}
	case *syntax.Param:
		if x.Sym != nil {
			b.set(el, Fact{Role: "declaration", Symbol: b.sym(x.Sym)})
			if !x.Vararg {
				b.typeRef(x.Type, x.Sym.Type)
			}
		}
	case *syntax.LambdaParam:
		if x.Sym != nil {
			b.set(el, Fact{Role: "declaration", Symbol: b.sym(x.Sym)})
			b.typeRef(x.Type, x.Sym.Type)
		}
	case *syntax.Catch:
		if x.Sym != nil {
			b.typeRef(x.Type, x.Sym.Type)
		}
	case *syntax.VarStmt:
		b.varStmt(x, el)
	case *syntax.SuperType:
		b.typeRef(x.Type, x.T)
	case *syntax.Assign:
		// A compound assignment's operator call: x.plusAssign(v), or
		// the x.plus(v) of x = x.plus(v).
		b.set(el, Fact{Role: "statement", Call: b.call(operatorCall(x.Stmts))})
	case *syntax.For:
		// A loop over a collection calls its iterator's operators.
		for _, f := range []*types.FunSym{x.Iter, x.HasNext, x.Next} {
			if f != nil && f.Suspend {
				b.set(el, Fact{Role: "statement", SuspendOperation: true})
				break
			}
		}
	case syntax.Expr:
		b.expr(x, el)
	}
}

func (b *builder) propDecl(x *syntax.PropDecl, el ElementID) {
	if x.DelegateOf != nil {
		// The x$delegate field, from PROPERTY_DELEGATE: the delegate's
		// value, typed as the field.
		if x.Sym != nil {
			b.set(el, Fact{Role: "expression", Type: b.cv.typ(x.Sym.Type)})
		}
		return
	}
	if x.Sym == nil {
		return
	}
	f := Fact{Role: "declaration", Symbol: b.sym(x.Sym)}
	if x.Field != nil && x.Field.DelegateOf == x && x.Field.Sym != nil {
		f.DelegateType = b.cv.typ(x.Field.Sym.Type)
		for _, acc := range []*syntax.FunDecl{x.Getter, x.Setter} {
			if acc == nil {
				continue
			}
			visit := func(n any) bool {
				if c, ok := n.(*syntax.Call); ok && c.Sym != nil && (c.Sym.Name == "getValue" || c.Sym.Name == "setValue") {
					if cl := b.call(c); cl != nil {
						f.DelegatedCalls = append(f.DelegatedCalls, *cl)
					}
					return false
				}
				return true
			}
			// Inspect doesn't enter declarations: walk the body.
			if acc.Body != nil {
				syntax.Inspect(acc.Body, visit)
			}
			if acc.ExprBody != nil {
				syntax.Inspect(acc.ExprBody, visit)
			}
		}
	}
	b.set(el, f)
	b.typeRef(x.Type, x.Sym.Type)
	if x.Recv != nil && x.Getter != nil && x.Getter.Sym != nil {
		b.typeRef(x.Recv, x.Getter.Sym.Recv)
	}
}

func (b *builder) varStmt(x *syntax.VarStmt, el ElementID) {
	if x.Sym == nil {
		return
	}
	f := Fact{Role: "declaration", Symbol: b.sym(x.Sym)}
	if x.DelegateSym != nil {
		f.DelegateType = b.cv.typ(x.DelegateSym.Type)
		for _, c := range []*syntax.Call{x.Get, x.Set} {
			if c != nil {
				if cl := b.call(c); cl != nil {
					f.DelegatedCalls = append(f.DelegatedCalls, *cl)
				}
			}
		}
	}
	b.set(el, f)
	b.typeRef(x.Type, x.Sym.Type)
}

// expr visits an expression.
func (b *builder) expr(x syntax.Expr, el ElementID) {
	f := Fact{Role: "expression", Type: b.cv.typ(x.Type())}
	switch x := x.(type) {
	case *syntax.Ident:
		b.ident(x, el, &f)
	case *syntax.Member:
		b.member(x, &f)
	case *syntax.Call:
		b.callExpr(x, el, &f)
	case *syntax.Index:
		f.Call = b.call(x.Call)
	case *syntax.Binary:
		f.Call = b.call(x.Call)
		if sub, ok := b.rec.elvis[x]; ok {
			f.NonNullLeftSubtypeRight = &sub
			// x ?: y never evaluates y if x can't be null: kotlinc
			// reports the elvis as unreachable code.
			if t := x.X.Type(); !t.P && t.K != types.Invalid && !types.MayBeNull(t) {
				f.Unreachable = true
			}
		}
	case *syntax.Unary:
		f.Call = b.call(x.Call)
		if f.Call == nil && x.Stmts != nil {
			// x++ on other types: the x.inc() of x = x.inc().
			f.Call = b.call(operatorCall(x.Stmts))
		}
		if f.Call == nil {
			f.Call = b.builtinUnary(x.Op, x.X.Type(), x.Type())
		}
	case *syntax.Lambda:
		for _, p := range x.Params {
			if p.Sym != nil {
				f.LambdaParameters = append(f.LambdaParameters, b.sym(p.Sym))
			}
		}
		if x.It != nil && len(x.Params) == 0 {
			f.LambdaParameters = []uint32{b.sym(x.It)}
			f.ImplicitParameter = true
		}
		if f.LambdaParameters == nil {
			f.LambdaParameters = []uint32{}
		}
		if x.Recv != nil {
			b.receivers[x.Recv] = el
		}
		switch {
		case x.Ref != nil:
			// A callable reference: the call its body makes.
			f.Call = b.call(refCall(x))
		case x.SuspendMod:
			b.suspendCall(x, el)
		}
	case *syntax.When:
		f.When = b.when(x)
	case *syntax.Is:
		b.typeRef(x.Ref, x.Target)
	case *syntax.As:
		b.typeRef(x.Ref, x.Target)
	case *syntax.Jump:
		if x.Kind == syntax.RETURN {
			b.later = append(b.later, func() { b.returnTarget(x, el) })
		}
	case *syntax.This:
		b.later = append(b.later, func() { b.receiverTarget(x, el) })
	case *syntax.ObjectExpr:
		if x.Decl != nil && x.Decl.Sym != nil {
			f.Symbol = b.sym(x.Decl.Sym)
		}
	}
	b.set(el, f)
}

// ident visits a simple name.
func (b *builder) ident(x *syntax.Ident, el ElementID, f *Fact) {
	f.Role = "reference"
	switch {
	case x.Member != nil:
		b.member(x.Member, f)
		f.Role = "reference"
		f.ImplicitReceiver = b.implicitReceiver(x.Member, nil, el)
		return
	case x.Sym != nil:
		if x.Sym.Kind == types.Prop {
			f.ImplicitReceiver = b.implicitReceiver(x, x.Sym.Owner, el)
		}
		f.Symbol = b.sym(x.Sym)
		f.Deprecated = deprecated(symDeprecation(x.Sym))
		if x.Sym.Kind == types.Global {
			f.ImportableName = varFQ(x.Sym)
		}
		b.flow(x.Sym, x.Type(), f)
	case x.Getter != nil:
		f.Symbol = b.sym(x.Getter)
		f.Deprecated = deprecated(symDeprecation(x.Getter))
		if !x.Getter.Static {
			f.ImplicitReceiver = b.implicitReceiver(x, x.Getter.Owner, el)
		}
		if x.Getter.Owner == nil {
			f.ImportableName = funFQ(x.Getter)
		}
	case x.Object != nil:
		f.Symbol = b.sym(x.Object)
		f.Classifier = true
		f.ImportableName = classFQ(x.Object)
		f.Deprecated = deprecated(classDeprecation(x.Object))
	}
	if x.Call != nil {
		f.Call = b.call(x.Call)
	}
}

// member visits x.name.
func (b *builder) member(x *syntax.Member, f *Fact) {
	f.Role = "reference"
	switch {
	case x.Prop != nil:
		f.Symbol = b.sym(x.Prop)
		f.Deprecated = deprecated(symDeprecation(x.Prop))
		b.flow(x.Prop, x.Type(), f)
	case x.Field != nil:
		f.Symbol = b.sym(x.Field)
	case x.Getter != nil:
		f.Symbol = b.sym(x.Getter)
		f.Deprecated = deprecated(symDeprecation(x.Getter))
	case x.Object != nil:
		f.Symbol = b.sym(x.Object)
		f.Classifier = true
		f.ImportableName = classFQ(x.Object)
		f.Deprecated = deprecated(classDeprecation(x.Object))
	}
	if x.Static != nil && !x.EvalX {
		// X names the class: Math in Math.max.
		if el, ok := b.where[x.X]; ok {
			b.set(el, Fact{Role: "reference", Symbol: b.sym(x.Static), Classifier: true, ImportableName: classFQ(x.Static)})
		}
	}
	if x.Call != nil {
		f.Call = b.call(x.Call)
	}
}

// flow sets the facts of a read of v whose type t a smart cast may have
// made non-null: as kotlinc's binding context, the type is the declared
// one, with the cast a flag.
func (b *builder) flow(v *types.VarSym, t types.Type, f *Fact) {
	d := v.Type
	if (d.N || d.P) && !t.N && !t.P && t.K != types.Invalid && t.K != types.Nothing {
		if dt := b.cv.typ(d); dt != nil {
			f.Type = dt
		}
		f.FlowNonNull, f.DefinitelyNonNull = true, true
	}
}

// callExpr visits a call, and its callee's name.
func (b *builder) callExpr(x *syntax.Call, el ElementID, f *Fact) {
	f.Call = b.call(x)
	if f.Call != nil && !x.Invoke && x.Ctor == nil && x.Sym != nil {
		var owner *types.ClassSym
		if !x.Sym.Static {
			owner = x.Sym.Owner
		}
		f.Call.ImplicitReceiver = b.implicitReceiver(x.Fun, owner, el)
		f.ImplicitReceiver = copyID(f.Call.ImplicitReceiver)
	}
	if f.Call != nil && x.Sym != nil && x.Sym.Decl == nil && strings.HasPrefix(f.Call.Name, "kotlin.") {
		f.Builtin = f.Call.Name
	}
	var s uint32
	switch {
	case x.Ctor != nil && x.Sym == nil:
		s = b.sym(x.Ctor)
	case x.Sym != nil && !x.Invoke:
		s = b.sym(x.Sym)
	}
	if x.Invoke || x.Fun == nil {
		return
	}
	fe, ok := b.where[x.Fun]
	if !ok {
		// The checker rewrote f() to this.f() for an implicit receiver:
		// the callee is the call's name.
		ref := b.node(el).ChildKind(psi.KindREFERENCE_EXPRESSION)
		fe, ok = ElementID{el.Snapshot, ref.ID()}, ref.Valid()
	}
	if !ok || fe == el {
		return
	}
	cf := Fact{Role: "reference", Symbol: s, Call: copyCall(f.Call), Builtin: f.Builtin, ImplicitReceiver: copyID(f.ImplicitReceiver)}
	if x.Sym != nil {
		cf.Deprecated = deprecated(symDeprecation(x.Sym))
	}
	if cf.Deprecated == nil && x.Ctor != nil {
		cf.Deprecated = deprecated(classDeprecation(x.Ctor))
	}
	if x.Sym != nil && x.Sym.Owner == nil && x.Ctor == nil {
		cf.ImportableName = b.funFQ(x.Sym)
	}
	if x.Ctor != nil {
		cf.Classifier = true
		cf.ImportableName = classFQ(x.Ctor)
	}
	b.set(fe, cf)
}

func (b *builder) funFQ(f *types.FunSym) string {
	if n, ok := b.names[f]; ok {
		return n
	}
	return funFQ(f)
}

// unaryNames are the member operators of built-in unary operators.
var unaryNames = map[syntax.Kind]string{
	syntax.MINUS: "unaryMinus", syntax.PLUS: "unaryPlus", syntax.NOT: "not",
	syntax.INC: "inc", syntax.DEC: "dec",
}

// builtinUnary is the call of a unary operator the checker builds in on
// a primitive operand, Int.unaryMinus and the like; nil for others.
func (b *builder) builtinUnary(op syntax.Kind, operand, result types.Type) *Call {
	name, ok := unaryNames[op]
	operand = operand.NonNull()
	if !ok || !(operand.IsNumeric() || operand == types.CharType || operand == types.BoolType) {
		return nil
	}
	recv := b.cv.typ(operand)
	if recv == nil {
		return nil
	}
	return &Call{Name: recv.Name + "." + name, Receiver: recv,
		ReturnType: b.cv.typ(result), DeclaredReturnType: b.cv.typ(result)}
}

// negated gives the prefix expressions folded into negative literals the
// literals' facts, as calls of unaryMinus on the literals' values.
func (b *builder) negated() {
	for i, p := range b.provs {
		sid := b.snaps[i].ID()
		p.Negated(func(id psi.NodeID, lit *syntax.IntLit) {
			el, ok := b.where[lit]
			if !ok {
				return
			}
			src, ok := b.res.facts[el]
			if !ok || src.Type == nil {
				return
			}
			cp := copyFact(*src)
			cp.Element = ElementID{sid, id}
			b.negatedAt[lit] = cp.Element
			// -128 as a Byte is still Int.unaryMinus of 128.
			operand := lit.Type().NonNull()
			if operand == types.ByteType || operand == types.ShortType {
				operand = types.IntType
			}
			cp.Call = b.builtinUnary(syntax.MINUS, operand, lit.Type())
			merge(b.fact(cp.Element), cp)
		})
	}
}

// call converts a call; nil for none or one with no symbol.
func (b *builder) call(x *syntax.Call) *Call {
	if x == nil {
		return nil
	}
	f := x.Sym
	if f == nil && x.Ctor == nil {
		return nil
	}
	c := &Call{ReturnType: b.cv.typ(x.Type()), ValueInvocation: x.Invoke}
	switch x.Builtin {
	case types.Require, types.Check, types.ErrorCall, types.Todo, types.RequireNotNull, types.CheckNotNull:
		// The checker's own symbol, whose Facade is the exception thrown.
		b.names[f] = "kotlin." + f.Name
	case types.ArrayNew, types.ArrayInit, types.ArrayLiteral, types.EnumValues, types.EnumValueOf, types.ToChar, types.NewString:
		// The checker's built-in array constructors and standard library
		// factories, which have no facade: String(chars) is in kotlin.text,
		// the others in kotlin.
		if f != nil && f.Owner == nil && f.Facade == "" {
			switch {
			case x.Builtin == types.NewString:
				b.names[f] = "kotlin.text." + f.Name
			case (x.Builtin == types.ArrayNew || x.Builtin == types.ArrayInit) && strings.HasSuffix(f.Name, "Array"):
				b.names[f] = "kotlin." + f.Name + ".<init>"
			default:
				b.names[f] = "kotlin." + f.Name
			}
		}
	}
	switch {
	case x.Ctor != nil:
		c.Name = classFQ(x.Ctor) + ".<init>"
	default:
		c.Name = b.funFQ(f)
	}
	n := len(x.Args)
	if x.ArgMap != nil {
		c.ArgumentMapping = slices.Clone(x.ArgMap)
	} else {
		c.ArgumentMapping = make([]int, n)
		for i := range c.ArgumentMapping {
			c.ArgumentMapping[i] = i
		}
	}
	b.callAnnotations(c, f, x.Ctor)
	if f == nil {
		c.Declaration = b.decl(x.Ctor.Decl)
		return c
	}
	c.Declaration = b.decl(f.Decl)
	if c.Declaration == nil && f.Name == "<init>" && f.Owner != nil {
		c.Declaration = b.decl(f.Owner.Decl)
	}
	c.Extension = f.Ext
	if f.Ext {
		c.DeclaredReceiver = b.cv.typ(f.Recv)
	}
	if m, ok := x.Fun.(*syntax.Member); ok && !x.Invoke && m.Static == nil {
		c.Receiver = b.cv.typ(m.X.Type())
	}
	for _, p := range f.Params {
		t := p.Type
		pt := b.cv.typ(t)
		if pt == nil {
			pt = &Type{}
		}
		c.Parameters = append(c.Parameters, *pt)
		c.ParameterNames = append(c.ParameterNames, p.Name)
		c.Varargs = append(c.Varargs, p.Vararg)
	}
	if f.Name != "<init>" {
		c.DeclaredReturnType = b.cv.typ(f.Ret)
	}
	c.TypeParameters = len(f.TypeParams)
	for _, p := range f.TypeParams {
		c.TypeParameterNames = append(c.TypeParameterNames, p.Name)
	}
	c.Suspend = f.Suspend
	c.Operator = f.Operator
	c.FromJava = f.Owner != nil && f.Owner.External && f.Owner.KotlinName == "" && f.Decl == nil
	c.StableNames = !c.FromJava
	switch {
	case f.Inline != nil:
		c.Inline = true
		c.Crossinline = slices.Clone(f.Inline.Crossinline)
		c.Noinline = slices.Clone(f.Inline.Noinline)
	case f.Declared != nil:
		c.Inline = true
		c.Crossinline = slices.Clone(f.Declared.Crossinline)
		c.Noinline = slices.Clone(f.Declared.Noinline)
	default:
		if d, ok := f.Decl.(*syntax.FunDecl); ok && d.Inline {
			c.Inline = true
			for _, p := range d.Params {
				c.Crossinline = append(c.Crossinline, p.Crossinline)
				c.Noinline = append(c.Noinline, p.Noinline)
			}
		}
	}
	return c
}

// when describes a when expression: as kotlinc's WhenChecker, whether
// its branches cover its subject, an else branch aside, and what they
// miss.
func (b *builder) when(x *syntax.When) *When {
	w := &When{}
	r := b.rec.when[x]
	w.Exhaustive = r.exhaustive
	var st types.Type
	switch {
	case x.Subject != nil:
		st = x.Subject.Type()
	case x.SubjSym != nil:
		st = x.SubjSym.Type
	default:
		return w
	}
	switch {
	case st.K == types.Class && st.C.Entries != nil:
		w.Category = "enum class"
	case st.K == types.Class && st.C.Sealed:
		w.Category = "sealed class"
	case st.K == types.Bool:
		w.Category = "boolean"
	}
	if w.Exhaustive {
		return w
	}
	var names []string
	var isTargets []*types.ClassSym
	null, trueSeen, falseSeen := false, false, false
	for _, br := range x.Branches {
		for _, c := range br.Conds {
			switch {
			case c.Is:
				if c.Target.K == types.Class {
					isTargets = append(isTargets, c.Target.C)
				}
				if c.Target.N {
					null = true
				}
			case c.X != nil && !c.In && !c.NotIn:
				switch v := c.X.(type) {
				case *syntax.NullLit:
					null = true
				case *syntax.BoolLit:
					if v.Val {
						trueSeen = true
					} else {
						falseSeen = true
					}
				case *syntax.Member:
					names = append(names, v.Name)
					if v.Object != nil {
						isTargets = append(isTargets, v.Object)
					}
				case *syntax.Ident:
					names = append(names, v.Name)
					if v.Object != nil {
						isTargets = append(isTargets, v.Object)
					}
				}
			}
		}
	}
	switch w.Category {
	case "enum class":
		for _, e := range st.C.Entries {
			if !slices.Contains(names, e) {
				w.Missing = append(w.Missing, e)
			}
		}
	case "sealed class":
		w.Missing = missingSubclasses(st.C, isTargets)
	case "boolean":
		if !trueSeen {
			w.Missing = append(w.Missing, "true")
		}
		if !falseSeen {
			w.Missing = append(w.Missing, "false")
		}
	}
	if (st.N || st.P) && !null && w.Category != "" {
		w.Missing = append(w.Missing, "null")
	}
	return w
}

// missingSubclasses returns the subclasses of sealed class c that no
// branch covers, as kotlinc names them: is S, or O for an object.
func missingSubclasses(c *types.ClassSym, covered []*types.ClassSym) []string {
	var out []string
	var walk func(c *types.ClassSym, depth int) bool
	// walk reports whether c is covered, adding what isn't of it.
	walk = func(c *types.ClassSym, depth int) bool {
		for _, t := range covered {
			if c.IsSubclassOf(t) {
				return true
			}
		}
		if c.Sealed && depth < 16 {
			var missing []string
			saved := out
			out = nil
			all := true
			for _, s := range c.Subclasses {
				if !walk(s, depth+1) {
					all = false
				}
			}
			missing, out = out, saved
			if all {
				return true
			}
			if depth > 0 && len(missing) == len(c.Subclasses) && !c.Abstract {
				out = append(out, caseName(c))
			} else {
				out = append(out, missing...)
			}
			return false
		}
		out = append(out, caseName(c))
		return false
	}
	walk(c, 0)
	return out
}

func caseName(c *types.ClassSym) string {
	if c.Object {
		return c.DisplayName()
	}
	return "is " + c.DisplayName()
}

// returnTarget sets the function or lambda a return leaves.
func (b *builder) returnTarget(x *syntax.Jump, el ElementID) {
	if x.Lambda != nil {
		if t, ok := b.where[x.Lambda]; ok {
			// A labeled lambda is recorded on its label: target the
			// lambda itself.
			n := b.node(t)
			for n.Valid() && isWrapper(n.NumericKind()) {
				n = inner(n)
			}
			if n.Valid() {
				t.Node = n.ID()
			}
			b.fact(el).ReturnTarget = &t
			return
		}
	}
	for p := b.node(el).Parent(); p.Valid(); p = p.Parent() {
		switch p.NumericKind() {
		case psi.KindFUN, psi.KindPROPERTY_ACCESSOR:
			b.fact(el).ReturnTarget = &ElementID{el.Snapshot, p.ID()}
			return
		}
	}
}

// receiverTarget sets the declaration whose receiver a this is.
func (b *builder) receiverTarget(x *syntax.This, el ElementID) {
	if t := b.thisTarget(x); t != nil {
		b.fact(el).ReceiverTarget = t
	}
}

// thisTarget returns the declaration whose receiver this is: a lambda
// with a receiver or an extension function, by the local the checker
// keeps it in, or else the class of its type.
func (b *builder) thisTarget(x *syntax.This) *ElementID {
	if x.Recv != nil {
		if t, ok := b.receivers[x.Recv]; ok {
			return &t
		}
	}
	if t := x.Type(); t.K == types.Class && t.C != nil {
		return b.decl(t.C.Decl)
	}
	return nil
}

// implicitReceiver returns the declaration that provides the receiver
// that fun, the callee or property el names, takes without one being
// written: the receiver of this.f the checker wrote for f, or, for a
// member of owner named alone, the innermost class or object around el
// that is an owner.
func (b *builder) implicitReceiver(fun syntax.Expr, owner *types.ClassSym, el ElementID) *ElementID {
	switch x := fun.(type) {
	case *syntax.Member:
		if th, ok := x.X.(*syntax.This); ok {
			if _, written := b.where[th]; !written {
				return b.thisTarget(th)
			}
		}
	case *syntax.Ident:
		if owner == nil {
			return nil
		}
		for n := b.node(el).Parent(); n.Valid(); n = n.Parent() {
			id := ElementID{el.Snapshot, n.ID()}
			if c := b.classAt[id]; c != nil && c.IsSubclassOf(owner) {
				return &id
			}
		}
	}
	return nil
}

// wrapperKinds are the psi nodes that wrap an expression, whose value
// they are: their facts are the inner expression's.
func isWrapper(k psi.Kind) bool {
	switch k {
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION, psi.KindPARENTHESIZED,
		psi.KindANNOTATED_EXPRESSION, psi.KindLABELED_EXPRESSION:
		return true
	}
	return false
}

// inner returns the expression a wrapper wraps: its last child that
// isn't a token.
func inner(n psi.NodeRef) psi.NodeRef {
	var last psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsLeaf() && !c.IsTrivia() {
			last = c
		}
	}
	return last
}

// top returns the outermost wrapper whose value n is, or n.
func top(n psi.NodeRef) psi.NodeRef {
	for {
		p := n.Parent()
		if !p.Valid() || !isWrapper(p.NumericKind()) || inner(p).ID() != n.ID() {
			return n
		}
		n = p
	}
}

// usedAsExpression sets which expressions' values are used: what the
// checker reported of statements, and all expressions that aren't
// statements.
func (b *builder) usedAsExpression() {
	reported := map[ElementID]bool{}
	for s, used := range b.rec.stmt {
		var n any = s
		if es, ok := s.(*syntax.ExprStmt); ok {
			n = es.X
		}
		el, ok := b.where[n]
		if lit, isLit := n.(*syntax.IntLit); isLit {
			if p, folded := b.negatedAt[lit]; folded {
				el, ok = p, true
			}
		}
		if ok {
			t := top(b.node(el))
			reported[ElementID{el.Snapshot, t.ID()}] = used
		}
	}
	for el, f := range b.res.facts {
		if f.Role != "expression" && f.Role != "reference" {
			continue
		}
		t := top(b.node(el))
		tel := ElementID{el.Snapshot, t.ID()}
		if used, ok := reported[tel]; ok {
			f.UsedAsExpression = used
			continue
		}
		f.UsedAsExpression = t.Parent().NumericKind() != psi.KindBLOCK
	}
}

// wrappers gives wrappers their inner expressions' facts, innermost
// first.
func (b *builder) wrappers() {
	for _, s := range b.snaps {
		for id := psi.NodeID(s.NodeCount()); id >= 1; id-- {
			n := s.Node(id)
			if !isWrapper(n.NumericKind()) {
				continue
			}
			in := inner(n)
			if !in.Valid() {
				continue
			}
			src, ok := b.res.facts[ElementID{s.ID(), in.ID()}]
			if !ok {
				continue
			}
			el := ElementID{s.ID(), id}
			cp := copyFact(*src)
			cp.Element = el
			merge(b.fact(el), cp)
		}
	}
}

// unreachable marks the statements the checker found unreachable.
func (b *builder) unreachable() {
	for s := range b.rec.unreachable {
		var n any = s
		if es, ok := s.(*syntax.ExprStmt); ok {
			n = es.X
		}
		if el, ok := b.where[n]; ok {
			t := top(b.node(el))
			b.fact(ElementID{el.Snapshot, t.ID()}).Unreachable = true
		}
	}
}

// annotations types the callees of the annotations the checker resolved:
// the USER_TYPE and TYPE_REFERENCE of each entry's CONSTRUCTOR_CALLEE.
func (b *builder) annotations() {
	for i, f := range b.files {
		for at, cls := range b.rec.annots[f] {
			id := b.provs[i].Annotation(at)
			if id == 0 {
				continue
			}
			ref := b.snaps[i].Node(id).ChildKind(psi.KindCONSTRUCTOR_CALLEE).ChildKind(psi.KindTYPE_REFERENCE)
			ct := b.cv.typ(types.ClassType(cls))
			if !ref.Valid() || ct == nil {
				continue
			}
			sid := b.snaps[i].ID()
			b.set(ElementID{sid, ref.ID()}, Fact{Role: "type", Type: ct})
			if u := ref.ChildKind(psi.KindUSER_TYPE); u.Valid() {
				b.set(ElementID{sid, u.ID()}, Fact{Role: "type", Type: copyType(ct)})
			}
		}
	}
}

// symbols describes the interned symbols. Converting their types may
// intern more.
func (b *builder) symbols() {
	for i := 0; i < len(b.syms); i++ {
		b.res.symbols = append(b.res.symbols, b.symbol(uint32(i+1), b.syms[i]))
	}
}

func (b *builder) symbol(id uint32, s any) Symbol {
	sym := Symbol{ID: id}
	switch s := s.(type) {
	case *types.ClassSym:
		sym.Kind = "class"
		sym.Name = s.Name
		sym.QualifiedName = classFQ(s)
		if s.Outer != nil {
			sym.Owner = s.Outer.Name
		}
		if t := b.cv.typ(s.This()); t != nil {
			sym.Type = *t
		}
		sym.Declaration = b.decl(s.Decl)
		sym.Supertypes = slices.Clone(b.cv.supertypesOf(s))
		b.classMembers(s, &sym)
	case *types.FunSym:
		sym.Kind = "function"
		if s.Name == "<init>" {
			sym.Kind = "constructor"
		}
		sym.Name = s.Name
		sym.QualifiedName = b.funFQ(s)
		if s.Owner != nil {
			sym.Owner = s.Owner.Name
		}
		if t := b.cv.typ(s.Ret); t != nil {
			sym.Type = *t
		}
		sym.Declaration = b.decl(s.Decl)
		if sym.Declaration == nil && s.Name == "<init>" && s.Owner != nil {
			sym.Declaration = b.decl(s.Owner.Decl)
		}
	case *types.VarSym:
		switch s.Kind {
		case types.Local:
			sym.Kind = "local"
		case types.Param:
			sym.Kind = "parameter"
		default:
			sym.Kind = "property"
		}
		sym.Name = s.Name
		sym.QualifiedName = varFQ(s)
		if s.Owner != nil {
			sym.Owner = s.Owner.Name
		}
		if t := b.cv.typ(s.Type); t != nil {
			sym.Type = *t
		}
		sym.Declaration = b.decl(s.Decl)
	}
	return sym
}

// classMembers sets what c inherits from its supertypes: abstract
// members, members with bodies, an abstract superclass.
func (b *builder) classMembers(c *types.ClassSym, sym *Symbol) {
	if c.Super != nil && c.Super.Abstract && !c.Super.Interface {
		sym.HasAbstractSuperclass = true
	}
	c.Supertypes(func(s *types.ClassSym) bool {
		if s == c || s.KotlinName == "kotlin/Any" || s.Name == "java/lang/Object" {
			return true
		}
		for _, fs := range s.Methods {
			for _, f := range fs {
				if f.Abstract {
					sym.HasAbstractMembers = true
				} else {
					sym.HasConcreteMembers = true
				}
			}
		}
		for _, p := range s.Props {
			if p.Abstract {
				sym.HasAbstractMembers = true
			} else {
				sym.HasConcreteMembers = true
			}
		}
		return !(sym.HasAbstractMembers && sym.HasConcreteMembers)
	})
}

// operatorCall returns the first operator call of stmts, which the
// checker rewrote a compound assignment, x++ or x-- into; nil if none.
func operatorCall(stmts []syntax.Stmt) *syntax.Call {
	var c *syntax.Call
	for _, st := range stmts {
		syntax.Inspect(st, func(n any) bool {
			if c != nil {
				return false
			}
			switch x := n.(type) {
			case *syntax.Call:
				if x.Operator && x.Sym != nil {
					c = x
				}
			case *syntax.Binary:
				c = x.Call
			case *syntax.Unary:
				c = x.Call
			}
			return c == nil
		})
		if c != nil {
			break
		}
	}
	return c
}

// refCall returns the call of the function, or constructor, that callable
// reference l names, which the checker made l's body; nil for a
// property.
func refCall(l *syntax.Lambda) *syntax.Call {
	if l.KFun != nil {
		return l.KFun
	}
	var c *syntax.Call
	syntax.Inspect(l.Body, func(n any) bool {
		if x, ok := n.(*syntax.Call); ok && c == nil {
			switch {
			case x.Sym != nil && x.Sym.Name == l.Ref.Name && !x.Invoke:
				c = x
			case x.Ctor != nil:
				c = x
			}
		}
		return c == nil
	})
	return c
}

// suspendCall describes suspend { ... }, which kc reads as a lambda with
// the suspend modifier, as Kotlin does: the call of the inline function
// kotlin.suspend, whose block is noinline.
func (b *builder) suspendCall(l *syntax.Lambda, el ElementID) {
	n := b.node(el)
	for n.Valid() && n.NumericKind() != psi.KindCALL_EXPRESSION {
		if k := n.NumericKind(); k != psi.KindLAMBDA_EXPRESSION && k != psi.KindLAMBDA_ARGUMENT {
			return
		}
		n = n.Parent()
	}
	t := b.cv.typ(l.Type())
	if !n.Valid() || t == nil {
		return
	}
	r := &Type{Name: "R", Parameter: true}
	c := &Call{
		Name:               "kotlin.suspend",
		Parameters:         []Type{*t},
		ParameterNames:     []string{"block"},
		Varargs:            []bool{false},
		ArgumentMapping:    []int{0},
		ReturnType:         t,
		DeclaredReturnType: &Type{Name: "kotlin.coroutines.SuspendFunction0", Suspend: true, Arguments: []Type{*r}},
		TypeParameters:     1,
		TypeParameterNames: []string{"R"},
		Annotations:        []string{"kotlin.internal.InlineOnly"},
		Inline:             true,
		StableNames:        true,
		Crossinline:        []bool{false},
		Noinline:           []bool{true},
	}
	call := ElementID{el.Snapshot, n.ID()}
	b.set(call, Fact{Role: "expression", Type: t, Call: c, Builtin: c.Name})
	if callee := n.ChildKind(psi.KindREFERENCE_EXPRESSION); callee.Valid() {
		b.set(ElementID{el.Snapshot, callee.ID()}, Fact{Role: "reference", Call: copyCall(c), Builtin: c.Name, ImportableName: c.Name})
	}
}
