package analysis

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
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
	rec := &recorder{when: map[*syntax.When]whenReport{}, stmt: map[syntax.Stmt]bool{}, unreachable: map[syntax.Stmt]bool{}, annots: map[*syntax.File]map[syntax.Pos]*types.ClassSym{}, elvis: map[*syntax.Binary]bool{}, typeRefs: map[*syntax.TypeRef]types.Type{}, annotClass: map[*syntax.Annotation]*types.ClassSym{}, stmtTypes: map[syntax.Expr]types.Type{}, aliases: map[any]*types.AliasSym{}}
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
	mu sync.Mutex
	// overridden returns the functions a method overrides, directly.
	overridden  func(*types.FunSym) []*types.FunSym
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
	// stmtTypes are the types of the ifs, whens and trys used as
	// statements.
	stmtTypes map[syntax.Expr]types.Type
	// aliases are the typealiases type references and constructor calls
	// name.
	aliases map[any]*types.AliasSym
}

// Alias implements check.AliasRecorder.
func (r *recorder) Alias(x any, a *types.AliasSym) {
	r.mu.Lock()
	r.aliases[x] = a
	r.mu.Unlock()
}

// aliasFQ returns the fully qualified name of a typealias.
func aliasFQ(a *types.AliasSym) string {
	if a.Outer != nil {
		return classFQ(a.Outer) + "." + a.Name
	}
	return qualify(a.Pkg, a.Name)
}

func (r *recorder) StmtType(x syntax.Expr, t types.Type) {
	r.mu.Lock()
	r.stmtTypes[x] = t
	r.mu.Unlock()
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
	case *syntax.PropDecl, *syntax.CtorParam, *syntax.ClassDecl, lower.WhenSubject:
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
	b.packages(entries)
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
	if f.Package == "" {
		f.Package = src.Package
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
	if f.Accessors == nil {
		f.Accessors = src.Accessors
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
	if a := b.rec.aliases[r]; a != nil {
		f.ImportableName = aliasFQ(a)
	}
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
			b.set(ElementID{Snapshot: el.Snapshot, Node: n.ID()}, Fact{Role: "type", Type: copyType(ct), ImportableName: f.ImportableName})
			break
		}
	}
}

func (b *builder) visit(n any, el ElementID) {
	switch x := n.(type) {
	case *syntax.PropDecl:
		b.propDecl(x, el)
	case lower.WhenSubject:
		// when (val x = e): x, a local the checker declares.
		if s := x.When.SubjSym; s != nil && x.When.SubjVar != "" {
			b.set(el, Fact{Role: "declaration", Symbol: b.sym(s)})
			b.typeRef(x.When.SubjType, s.Type)
		}
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
	case *syntax.Destructure:
		b.destructured(x, b.node(el), el.Snapshot)
	case *syntax.For:
		if x.Names != nil && x.Body != nil {
			d := b.node(el).ChildKind(psi.KindVALUE_PARAMETER).ChildKind(psi.KindDESTRUCTURING_DECLARATION)
			b.destructured(generatedDestructure(x.Body, x.Var), d, el.Snapshot)
		}
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
						if t, ok := lazyResult(x.Field.Init); ok && c.Sym.Name == "getValue" {
							cl.ReturnType = b.cv.typ(t)
						}
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

// lazyResult returns the result type of the lambda of delegate x, a call
// of the standard library's lazy without type arguments, which kotlinc
// infers lazy's T from, the property's type being only its upper bound:
// val a: Int? by lazy { 5 } reads a Lazy<Int>.
func lazyResult(x syntax.Expr) (types.Type, bool) {
	c, ok := x.(*syntax.Call)
	if !ok || c.Sym == nil || c.Sym.Decl != nil || c.Sym.Name != "lazy" || len(c.TypeArgs) > 0 || len(c.Args) == 0 {
		return types.Type{}, false
	}
	l, ok := c.Args[len(c.Args)-1].(*syntax.Lambda)
	if !ok || l.Ref != nil || l.Body == nil || len(l.Body.Stmts) == 0 {
		return types.Type{}, false
	}
	e, ok := l.Body.Stmts[len(l.Body.Stmts)-1].(*syntax.ExprStmt)
	if !ok || e.X.Type().K == types.Invalid {
		return types.Type{}, false
	}
	return e.X.Type(), true
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

// destructured gives each entry of DESTRUCTURING_DECLARATION d the
// declaration fact of the local that s, which the checker expanded,
// declares for it. s's first statement holds the destructured value; the
// others declare the entries in order, but for _.
func (b *builder) destructured(s *syntax.Destructure, d psi.NodeRef, snap psi.SnapshotID) {
	if s == nil || !d.Valid() || len(s.Stmts) == 0 {
		return
	}
	vars := s.Stmts[1:]
	for e := d.FirstChild(); e.Valid() && len(vars) > 0; e = e.NextSibling() {
		if e.NumericKind() != psi.KindDESTRUCTURING_DECLARATION_ENTRY || e.Name() == "_" {
			continue
		}
		if vars[0].Name == e.Name() {
			b.varStmt(vars[0], ElementID{Snapshot: snap, Node: e.ID()})
		}
		vars = vars[1:]
	}
}

// generatedDestructure returns the declaration the checker put at the
// start of body for a lambda parameter's or a for loop's destructuring,
// which reads the variable named tmp.
func generatedDestructure(body *syntax.Block, tmp string) *syntax.Destructure {
	for _, st := range body.Stmts {
		if d, ok := st.(*syntax.Destructure); ok {
			if id, ok := d.Init.(*syntax.Ident); ok && id.Name == tmp {
				return d
			}
		}
	}
	return nil
}

// expr visits an expression.
func (b *builder) expr(x syntax.Expr, el ElementID) {
	f := Fact{Role: "expression", Type: b.cv.typ(x.Type())}
	if t, ok := b.rec.stmtTypes[x]; ok && f.Type == nil {
		f.Type = b.cv.typ(t)
	}
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
		if f.Call == nil && (x.Op == syntax.EQ || x.Op == syntax.NE) {
			f.Call = b.equalsCall(x.X.Type())
		}
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
			if pel, ok := b.where[p]; ok && p.Names != nil && x.Body != nil {
				b.destructured(generatedDestructure(x.Body, p.Name), b.node(pel).ChildKind(psi.KindDESTRUCTURING_DECLARATION), pel.Snapshot)
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
		f.ImpossibleCast = !b.castPossible(x.X.Type(), x.Target)
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
	if m := safeAccess(x); m != nil && m.SelNonNull {
		// x?.a on an x that can't be null: as kotlinc's binding
		// context, the type stays a?'s, which the compiler's own
		// UNNECESSARY_SAFE_CALL reads, and data flow knows it non-null.
		if rt := m.X.Type(); rt.K != types.Invalid && !rt.P && !types.MayBeNull(rt) {
			f.FlowNonNull, f.DefinitelyNonNull = true, true
		}
	}
	b.set(el, f)
}

// safeAccess returns the safe access x is, x?.a or x?.f(), or nil.
func safeAccess(x syntax.Expr) *syntax.Member {
	switch x := x.(type) {
	case *syntax.Member:
		if x.Safe {
			return x
		}
	case *syntax.Call:
		if m, ok := x.Fun.(*syntax.Member); ok && m.Safe && !x.Invoke {
			return m
		}
	}
	return nil
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

// accessors returns the getter and setter of the property a member
// access names, with the accessors they override as Targets.
func (b *builder) accessors(x *syntax.Member) []Call {
	switch {
	case x.Getter != nil && x.Getter.Owner != nil && !x.Entries:
		// A Java getter seen as a property, and its setter.
		g := b.funCall(x.Getter)
		g.Targets = b.targets(x.Getter)
		out := []Call{g}
		set := x.Setter
		if set == nil {
			set = javaSetter(x.Getter)
		}
		if set != nil {
			s := b.funCall(set)
			s.Targets = b.targets(set)
			out = append(out, s)
		}
		return out
	case x.Prop != nil && (x.Prop.Kind == types.Prop || x.Prop.Kind == types.Global):
		get := func(p *types.VarSym) Call { return b.propAccessor(p, "get") }
		set := func(p *types.VarSym) Call { return b.propAccessor(p, "set") }
		var supers []*types.VarSym
		if p := x.Prop; p.Kind == types.Prop && p.Owner != nil {
			p.Owner.Supertypes(func(o *types.ClassSym) bool {
				if q := o.Prop(p.Name); o != p.Owner && q != nil {
					supers = append(supers, q)
				}
				return true
			})
		}
		out := []Call{get(x.Prop)}
		for _, q := range supers {
			out[0].Targets = append(out[0].Targets, get(q))
		}
		if x.Prop.Mutable {
			s := set(x.Prop)
			for _, q := range supers {
				if q.Mutable {
					s.Targets = append(s.Targets, set(q))
				}
			}
			out = append(out, s)
		}
		return out
	}
	return nil
}

// propAccessor describes a Kotlin property's getter or setter, kind get
// or set, as kotlinc names it: Owner.<get-p>.
func (b *builder) propAccessor(p *types.VarSym, kind string) Call {
	fq := varFQ(p)
	prefix := strings.TrimSuffix(fq, p.Name)
	c := Call{Name: prefix + "<" + kind + "-" + p.Name + ">", Declaration: b.decl(p.Decl), StableNames: true}
	t := b.cv.typ(p.Type)
	if t == nil {
		t = &Type{}
	}
	if kind == "get" {
		c.DeclaredReturnType = t
	} else {
		c.Parameters, c.ParameterNames, c.Varargs = []Type{*t}, []string{"value"}, []bool{false}
		c.DeclaredReturnType = b.cv.typ(types.UnitType)
	}
	return c
}

// javaSetter returns the setter that goes with Java getter g, getP or
// isP: setP of one parameter of g's return type, in g's class or one of
// its supertypes.
func javaSetter(g *types.FunSym) *types.FunSym {
	var name string
	switch {
	case strings.HasPrefix(g.Name, "get"):
		name = "set" + g.Name[3:]
	case strings.HasPrefix(g.Name, "is"):
		name = "set" + g.Name[2:]
	default:
		return nil
	}
	var found *types.FunSym
	g.Owner.Supertypes(func(o *types.ClassSym) bool {
		for _, f := range o.Methods[name] {
			if len(f.Params) == 1 && !f.Static && f.Params[0].Type.NonNull() == g.Ret.NonNull() {
				found = f
				return false
			}
		}
		return true
	})
	return found
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
	f.Accessors = b.accessors(x)
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
	if m, ok := x.Fun.(*syntax.Member); ok && f.Call == nil && !x.Invoke && x.Ctor == nil && x.Builtin != types.NotBuiltin {
		f.Call = b.builtinMember(m, x)
		if f.Call != nil {
			f.Call.ImplicitReceiver = b.implicitReceiver(x.Fun, nil, el)
			f.ImplicitReceiver = copyID(f.Call.ImplicitReceiver)
		}
	}
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
	if x.Invoke {
		b.invokeCallee(x, el)
		return
	}
	if x.Fun == nil {
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
		if a := b.rec.aliases[x]; a != nil {
			cf.ImportableName = aliasFQ(a)
		}
	}
	b.set(fe, cf)
}

// invokeCallee sets the facts of the property a call of a value of
// function type reads when the checker rewrote it, f() in an inner class
// being outer.f(): the callee, written f, is that property's reference.
func (b *builder) invokeCallee(x *syntax.Call, el ElementID) {
	m, ok := x.Fun.(*syntax.Member)
	if !ok {
		return
	}
	if _, written := b.where[m]; written {
		return
	}
	ref := b.node(el).ChildKind(psi.KindREFERENCE_EXPRESSION)
	if !ref.Valid() || ref.Text() != m.Name {
		return
	}
	cf := Fact{UsedAsExpression: true}
	b.member(m, &cf)
	cf.Type = b.cv.typ(m.Type())
	cf.ImplicitReceiver = b.implicitReceiver(m, nil, el)
	b.set(ElementID{el.Snapshot, ref.ID()}, cf)
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

// builtinMember is the call of a member function the checker builds in,
// 1.plus(2) or "a".plus(b), which has no symbol: named by its receiver's
// type and its name.
func (b *builder) builtinMember(m *syntax.Member, x *syntax.Call) *Call {
	if m.X == nil {
		return nil
	}
	recv := b.cv.typ(m.X.Type().NonNull())
	if recv == nil || recv.Name == "" {
		return nil
	}
	c := &Call{Name: recv.Name + "." + m.Name, Receiver: recv,
		ReturnType: b.cv.typ(x.Type()), DeclaredReturnType: b.cv.typ(x.Type()),
		ArgumentMapping: make([]int, len(x.Args))}
	for i := range c.ArgumentMapping {
		c.ArgumentMapping[i] = i
	}
	t := m.X.Type()
	switch ext, ok := textExtensions[x.Builtin]; {
	case x.Builtin == types.ToString && len(x.Args) == 0 && !m.Safe && types.MayBeNull(t) && !t.P:
		// toString() of a nullable value is the extension Any?.toString().
		c.Name, c.Extension = "kotlin.toString", true
		c.Receiver = b.cv.typ(t)
		c.DeclaredReceiver = &Type{Name: "kotlin.Any", Nullable: true, MarkedNullable: true}
	case ok && m.Name == ext.name:
		// A function of kotlin.text, an extension of String or
		// CharSequence the checker builds in.
		c.Name, c.Extension = "kotlin.text."+m.Name, true
		c.DeclaredReceiver = &Type{Name: ext.recv}
	}
	return c
}

// textExtensions are the built-in functions of strings that are
// extensions of kotlin.text, by name, with their declared receiver.
var textExtensions = map[types.Builtin]struct{ name, recv string }{
	types.StrIsEmpty:     {"isEmpty", "kotlin.CharSequence"},
	types.StrUppercase:   {"uppercase", "kotlin.String"},
	types.StrLowercase:   {"lowercase", "kotlin.String"},
	types.StrSubstring:   {"substring", "kotlin.String"},
	types.StrToCharArray: {"toCharArray", "kotlin.String"},
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

// castPossible reports whether a value of type l may be cast to r: as
// kotlinc's CastDiagnosticsUtil.isCastPossible, whose false is its
// CAST_NEVER_SUCCEEDS warning. Classes are related when one is the
// other's subclass; a final class's instances are only its own, and an
// interface may be implemented by any class that isn't final.
func (b *builder) castPossible(l, r types.Type) bool {
	open := func(t types.Type) bool {
		switch t.K {
		case types.Invalid, types.Any, types.TypeParam, types.TypeVar, types.Range:
			return true
		}
		return false
	}
	if open(l) || open(r) {
		return true
	}
	rNull := r.N || r.P
	if l.K == types.Null {
		return rNull
	}
	if l.K == types.Nothing || r.K == types.Nothing {
		return true
	}
	lt, rt := b.cv.typ(l), b.cv.typ(r)
	if lt == nil || rt == nil {
		return true
	}
	ln, rn := canonical(lt.Name), canonical(rt.Name)
	if ln == rn || slices.ContainsFunc(lt.Supertypes, func(s string) bool { return canonical(s) == rn }) ||
		slices.ContainsFunc(rt.Supertypes, func(s string) bool { return canonical(s) == ln }) {
		return true
	}
	if finalType(l) || finalType(r) {
		return false
	}
	return l.K == types.Class && l.C.Interface || r.K == types.Class && r.C.Interface
}

// canonical returns a type name with no nullability mark.
func canonical(name string) string { return strings.TrimSuffix(name, "?") }

// finalType reports whether type t has no subtypes but itself, as
// kotlinc's !canHaveSubtypes, approximately: a final class, its type
// arguments final too.
func finalType(t types.Type) bool {
	switch t.K {
	case types.Unit, types.Bool, types.Int, types.Long, types.Double, types.Char, types.Float, types.Byte, types.Short, types.String:
		return true
	case types.Class:
		if t.C == nil || !t.C.Final || t.C.Interface {
			return false
		}
		for i := range t.A.Len() {
			if a := t.A.Get(i); a.V != types.Inv || !finalType(a.T) {
				return false
			}
		}
		return true
	}
	return false
}

// equalsCall is the equals call == and != make on a left side of type
// t, as kotlinc resolves it: the first equals(Any?) up t's class's
// supertypes, or Any's, with the ones it overrides as Targets.
func (b *builder) equalsCall(t types.Type) *Call {
	if t.K == types.Invalid || t.K == types.Null || t.K == types.Nothing {
		return nil
	}
	t = t.NonNull()
	anyEq := Call{Name: "kotlin.Any.equals", Parameters: []Type{*b.cv.typ(types.Type{K: types.Any, N: true})}, ParameterNames: []string{"other"}, Varargs: []bool{false},
		DeclaredReturnType: b.cv.typ(types.BoolType), Operator: true, StableNames: true}
	if t.K == types.Class && t.C != nil {
		var eq *types.FunSym
		t.C.Supertypes(func(o *types.ClassSym) bool {
			for _, f := range o.Methods["equals"] {
				if len(f.Params) == 1 && !f.Static && !f.Ext && f.Params[0].Type.K == types.Any {
					eq = f
					return false
				}
			}
			return true
		})
		if eq != nil && eq.Owner != nil && classFQ(eq.Owner) != "kotlin.Any" {
			c := b.funCall(eq)
			c.Targets = b.targets(eq)
			if !slices.ContainsFunc(c.Targets, func(t Call) bool { return t.Name == anyEq.Name }) {
				c.Targets = append(c.Targets, anyEq)
			}
			return &c
		}
		if t.C.Interface || eq == nil || classFQ(eq.Owner) == "kotlin.Any" {
			return &anyEq
		}
	}
	lt := b.cv.typ(t)
	if lt == nil || canonical(lt.Name) == "kotlin.Any" {
		return &anyEq
	}
	c := anyEq
	c.Name = canonical(lt.Name) + ".equals"
	c.Targets = []Call{anyEq}
	return &c
}

// Overridden implements check.OverrideRecorder.
func (r *recorder) Overridden(fn func(*types.FunSym) []*types.FunSym) { r.overridden = fn }

// targets returns the functions f overrides, through every supertype,
// each once: as kotlinc's overriddenTreeUniqueAsSequence, f aside.
func (b *builder) targets(f *types.FunSym) []Call {
	if b.rec.overridden == nil {
		return nil
	}
	var out []Call
	seen := map[*types.FunSym]bool{f: true}
	var walk func(*types.FunSym)
	walk = func(f *types.FunSym) {
		for _, g := range b.rec.overridden(f) {
			if seen[g] {
				continue
			}
			seen[g] = true
			out = append(out, b.funCall(g))
			walk(g)
		}
	}
	walk(f)
	return out
}

// funCall describes function f as a call of it would, with no call's
// arguments, receiver or annotations.
func (b *builder) funCall(f *types.FunSym) Call {
	c := Call{Name: b.funFQ(f), Declaration: b.decl(f.Decl), Extension: f.Ext}
	if f.Ext {
		c.DeclaredReceiver = b.cv.typ(f.Recv)
	}
	for _, p := range f.Params {
		pt := b.cv.typ(p.Type)
		if pt == nil {
			pt = &Type{}
		}
		c.Parameters = append(c.Parameters, *pt)
		c.ParameterNames = append(c.ParameterNames, p.Name)
		c.Varargs = append(c.Varargs, p.Vararg)
	}
	c.DeclaredReturnType = b.cv.typ(f.Ret)
	c.TypeParameters = len(f.TypeParams)
	for _, p := range f.TypeParams {
		c.TypeParameterNames = append(c.TypeParameterNames, p.Name)
	}
	c.Suspend, c.Operator = f.Suspend, f.Operator
	c.FromJava = f.Owner != nil && f.Owner.External && f.Owner.KotlinName == "" && f.Decl == nil
	c.StableNames = !c.FromJava
	return c
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
	// kotlinc: a function type's invoke has no stable parameter names,
	// whatever names its type spells.
	c.StableNames = !c.FromJava && !x.Invoke
	if !x.Invoke {
		c.Targets = b.targets(f)
	}
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
			n := b.snaps[i].Node(id)
			ct := b.cv.typ(types.ClassType(cls))
			sid := b.snaps[i].ID()
			if ct == nil {
				continue
			}
			if n.NumericKind() == psi.KindCALL_EXPRESSION {
				// A nested annotation, ReplaceWith("x") in @Deprecated: a
				// constructor call of its class.
				b.set(ElementID{sid, n.ID()}, Fact{Role: "expression", Type: ct})
				if callee := n.ChildKind(psi.KindREFERENCE_EXPRESSION); callee.Valid() {
					b.set(ElementID{sid, callee.ID()}, Fact{Role: "reference", Classifier: true, ImportableName: classFQ(cls)})
				}
				continue
			}
			ref := n.ChildKind(psi.KindCONSTRUCTOR_CALLEE).ChildKind(psi.KindTYPE_REFERENCE)
			if !ref.Valid() {
				continue
			}
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
		sym.Shadowed = s.Shadowed
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

// classMembers sets what c's member scope holds, as kotlinc's
// unsubstitutedMemberScope does: c's and its supertypes' members that
// nothing else in the hierarchy overrides, kotlin.Any's open members and
// c's nested classes. HasAbstractMembers and HasConcreteMembers report
// whether any of them is abstract, or not.
func (b *builder) classMembers(c *types.ClassSym, sym *Symbol) {
	if c.Super != nil && c.Super.Abstract && !c.Super.Interface {
		sym.HasAbstractSuperclass = true
	}
	var funs []*types.FunSym
	var props []*types.VarSym
	hidden := map[*types.FunSym]bool{}
	var hide func(f *types.FunSym)
	hide = func(f *types.FunSym) {
		if b.rec.overridden == nil {
			return
		}
		for _, o := range b.rec.overridden(f) {
			if !hidden[o] {
				hidden[o] = true
				hide(o)
			}
		}
	}
	c.Supertypes(func(s *types.ClassSym) bool {
		if s.KotlinName == "kotlin/Any" || s.Name == "java/lang/Object" {
			return true
		}
		for _, fs := range s.Methods {
			for _, f := range fs {
				if f.Static || f.Name == "<init>" {
					continue
				}
				funs = append(funs, f)
				hide(f)
			}
		}
		for _, p := range s.Props {
			if p.Kind == types.Prop {
				props = append(props, p)
			}
		}
		return true
	})
	// kotlin.Any's equals, hashCode and toString are open unless the
	// hierarchy re-declares them abstract.
	anyOpen := map[string]bool{"equals/1": true, "hashCode/0": true, "toString/0": true}
	member := func(abstract bool) {
		if abstract {
			sym.HasAbstractMembers = true
		} else {
			sym.HasConcreteMembers = true
		}
	}
	for _, f := range funs {
		if hidden[f] {
			continue
		}
		member(f.Abstract)
		if f.Abstract {
			delete(anyOpen, f.Name+"/"+strconv.Itoa(len(f.Params)))
		}
	}
	if len(anyOpen) > 0 {
		sym.HasConcreteMembers = true
	}
	// A property is overridden by a same-named one in a subclass.
	for _, p := range props {
		overridden := false
		for _, q := range props {
			if q != p && q.Name == p.Name && q.Owner != p.Owner && q.Owner.IsSubclassOf(p.Owner) {
				overridden = true
				break
			}
		}
		if !overridden {
			member(p.Abstract)
		}
	}
	for _, n := range c.Nested {
		member(n.Abstract)
	}
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

// packages sets Package on the names of packages that qualify a
// reference to a top-level declaration: kotlin and kotlin.math in
// kotlin.math.floor(x), p in p.Outer.Inner().
func (b *builder) packages(entries [][]entry) {
	for _, es := range entries {
		for _, e := range es {
			switch x := e.n.(type) {
			case *syntax.Call:
				m, ok := x.Fun.(*syntax.Member)
				switch {
				case !ok || x.Invoke:
				case x.Ctor != nil:
					b.qualifier(m.X, classPackage(x.Ctor))
				case x.Sym != nil && x.Sym.Owner == nil && !x.Sym.Ext:
					b.qualifier(m.X, facadePackage(x.Sym.Facade))
				}
			case *syntax.Member:
				switch {
				case x.Static != nil && !x.EvalX:
					b.qualifier(x.X, classPackage(x.Static))
				case x.Object != nil:
					b.qualifier(x.X, classPackage(x.Object))
				case x.Prop != nil && x.Prop.Kind == types.Global:
					b.qualifier(x.X, facadePackage(x.Prop.Facade))
				}
			}
		}
	}
}

// qualifier sets Package on the part of x that spells pkg, and on the
// parts of that that spell its parents. The rest of x names classes.
func (b *builder) qualifier(x syntax.Expr, pkg string) {
	if pkg == "" {
		return
	}
	for {
		m, ok := x.(*syntax.Member)
		if !ok || !strings.HasPrefix(dotted(x), pkg+".") {
			break
		}
		x = m.X
	}
	if dotted(x) != pkg {
		return
	}
	for x != nil {
		el, ok := b.where[x]
		if !ok {
			return
		}
		f := b.fact(el)
		if f.Symbol != 0 || f.Type != nil || f.Classifier || f.Call != nil {
			return
		}
		f.Package = dotted(x)
		m, ok := x.(*syntax.Member)
		if !ok {
			return
		}
		x = m.X
	}
}

// dotted returns the dotted name x spells, a.b.c, or "".
func dotted(x syntax.Expr) string {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Name
	case *syntax.Member:
		if q := dotted(x.X); q != "" && !x.Safe {
			return q + "." + x.Name
		}
	}
	return ""
}

// classPackage returns the dotted package of a class.
func classPackage(c *types.ClassSym) string {
	for c.Outer != nil {
		c = c.Outer
	}
	if c.KotlinName != "" {
		return facadePackage(c.KotlinName)
	}
	return facadePackage(c.Name)
}
