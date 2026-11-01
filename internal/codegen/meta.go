package codegen

import (
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Kotlin metadata: the @kotlin.Metadata annotation of each class kc
// writes, as kotlinc writes it, so that kotlinc and kotlin-reflect see
// Kotlin declarations: a file facade's top-level functions and
// properties, and a class's kind, supertypes, constructors and members,
// with their Kotlin types, flags and JVM signatures.

// MetaTime is the time spent building and encoding metadata, in
// nanoseconds, summed over the files generated in parallel, for -time.
var MetaTime atomic.Int64

// writeMetadata annotates the file's classes with their metadata.
func (g *gen) writeMetadata() {
	start := time.Now()
	defer func() { MetaTime.Add(int64(time.Since(start))) }()
	if c := g.shared.classes[g.file.Class()]; c != nil {
		h := kmeta.WritePackage(g.packageMeta())
		if g.file.Part != "" {
			h.K, h.XS = kmeta.KindMultiFilePart, g.file.Facade
		}
		g.annotateMeta(c, h)
	}
	var visit func(cd *syntax.ClassDecl)
	visit = func(cd *syntax.ClassDecl) {
		if c := g.shared.classes[cd.Sym.Name]; c != nil {
			g.annotateMeta(c, kmeta.WriteClass(g.classMeta(cd)))
		}
		for _, impls := range []string{"$DefaultImpls", "$ComposeDefaultImpls"} {
			if di := g.shared.classes[cd.Sym.Name+impls]; di != nil {
				g.annotateMeta(di, kmeta.WriteSynthetic())
			}
		}
		for _, m := range cd.Members {
			if nd, ok := m.(*syntax.ClassDecl); ok {
				visit(nd)
			}
		}
	}
	for _, d := range g.file.Decls {
		if cd, ok := d.(*syntax.ClassDecl); ok {
			visit(cd)
		}
	}
	for _, cd := range g.file.Anon {
		// An object expression's class: local, as kotlinc writes it.
		if c := g.shared.classes[cd.Sym.Name]; c != nil {
			h := kmeta.WriteClass(g.classMeta(cd))
			if inInline(cd) && !cd.EnclosingFun.Private {
				h.XI |= kmeta.XIPublicABI
			}
			g.annotateMeta(c, h)
		}
	}
}

// inlineXI returns metadata h of a class of local code, with extra flags
// xi in the body of an inline function that other modules copy.
func (g *gen) inlineXI(h kmeta.Header, xi int) kmeta.Header {
	if g.publicInline {
		h.XI = xi
	}
	return h
}

func (g *gen) annotateMeta(c *cf.Class, h kmeta.Header) {
	ints := func(vs []int) cf.ElementValue {
		ev := cf.ElementValue{Tag: '['}
		for _, v := range vs {
			ev.Elems = append(ev.Elems, cf.ElementValue{Tag: 'I', Const: int64(v)})
		}
		return ev
	}
	strs := func(ss []string) cf.ElementValue {
		ev := cf.ElementValue{Tag: '['}
		for _, s := range ss {
			ev.Elems = append(ev.Elems, cf.ElementValue{Tag: 's', Const: s})
		}
		return ev
	}
	a := cf.Annotation{Type: "Lkotlin/Metadata;", Values: []cf.ElementPair{
		{Name: "mv", Value: ints(h.MV)},
		{Name: "k", Value: cf.ElementValue{Tag: 'I', Const: int64(h.K)}},
		{Name: "xi", Value: cf.ElementValue{Tag: 'I', Const: int64(h.XI)}},
	}}
	if h.K != kmeta.KindSynthetic || h.D1 != nil {
		a.Values = append(a.Values, cf.ElementPair{Name: "d1", Value: strs(h.D1)})
		if h.D2 != nil {
			a.Values = append(a.Values, cf.ElementPair{Name: "d2", Value: strs(h.D2)})
		}
	}
	if h.XS != "" {
		a.Values = append(a.Values, cf.ElementPair{Name: "xs", Value: cf.ElementValue{Tag: 's', Const: h.XS}})
	}
	c.Annotate(a, true)
}

// MultifileFacade writes the facade of a multi-file class from its
// parts, as kotlinc does: the parts' methods that aren't private, which
// call them, and metadata that lists the parts.
func MultifileFacade(name string, parts []*cf.Class) (Output, error) {
	slices.SortFunc(parts, func(a, b *cf.Class) int { return strings.Compare(a.Name, b.Name) })
	c := cf.NewClass(cf.AccPublic|cf.AccFinal|cf.AccSuper, name, "java/lang/Object")
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.Name
		if err := c.AddDelegates(p); err != nil {
			return Output{}, err
		}
	}
	g := &gen{}
	g.annotateMeta(c, kmeta.Header{K: kmeta.KindMultiFileClass, MV: kmeta.Version, XI: kmeta.XI, D1: names})
	return Output{Name: name, Bytes: c.Bytes()}, nil
}

// module returns the module's name for metadata: "" for main, which
// kotlinc leaves out.
func (g *gen) module() string {
	if m := g.shared.opts.Module; m != "main" {
		return m
	}
	return ""
}

// metaScope numbers the type parameters in scope, as metadata refers to
// them: a class's from 0, then a member's.
type metaScope struct {
	ids  map[*types.TypeParamSym]int
	next int
}

func (s *metaScope) with(tps []*types.TypeParamSym) (*metaScope, []*kmeta.TypeParam) {
	n := &metaScope{ids: map[*types.TypeParamSym]int{}, next: 0}
	if s != nil {
		for k, v := range s.ids {
			n.ids[k] = v
		}
		n.next = s.next
	}
	for _, tp := range tps {
		n.ids[tp] = n.next
		n.next++
	}
	out := make([]*kmeta.TypeParam, len(tps))
	for i, tp := range tps {
		p := &kmeta.TypeParam{ID: n.ids[tp], Name: tp.Name, Reified: tp.Reified, Variance: variance(tp.Var)}
		for _, b := range tp.Bounds {
			// Also Any?, the default, if declared, as kotlinc writes it.
			p.Bounds = append(p.Bounds, n.typ(b))
		}
		out[i] = p
	}
	return n, out
}

func variance(v types.Variance) kmeta.Projection {
	switch v {
	case types.Out:
		return kmeta.Out
	case types.In:
		return kmeta.In
	case types.Star:
		return kmeta.Star
	}
	return kmeta.Inv
}

// kotlinOfJava are the Java classes that are Kotlin types.
var kotlinOfJava = map[string]string{
	"java/lang/Object":                "kotlin/Any",
	"java/lang/String":                "kotlin/String",
	"java/lang/Comparable":            "kotlin/Comparable",
	"java/lang/CharSequence":          "kotlin/CharSequence",
	"java/lang/Number":                "kotlin/Number",
	"java/lang/Throwable":             "kotlin/Throwable",
	"java/lang/Enum":                  "kotlin/Enum",
	"java/lang/Cloneable":             "kotlin/Cloneable",
	"java/lang/annotation/Annotation": "kotlin/Annotation",
}

var kindIDs = map[types.Kind]string{
	types.Unit: "kotlin/Unit", types.Nothing: "kotlin/Nothing", types.Bool: "kotlin/Boolean", types.Int: "kotlin/Int",
	types.Long: "kotlin/Long", types.Double: "kotlin/Double", types.Char: "kotlin/Char", types.Float: "kotlin/Float",
	types.Byte: "kotlin/Byte", types.Short: "kotlin/Short", types.String: "kotlin/String", types.Any: "kotlin/Any",
}

// classID returns a class's Kotlin class id: packages separated by '/',
// nested classes by '.'.
func classID(c *types.ClassSym) string {
	switch {
	case c.KotlinName != "":
		return c.KotlinName
	case c.Func:
		return "kotlin/Function" + strconv.Itoa(len(c.TypeParams)-1)
	case c.Outer != nil:
		// The simple name, which may start with $: Point$$serializer's
		// is $serializer.
		i := strings.LastIndexByte(c.Name, '$')
		if i > 0 && c.Name[i-1] == '$' {
			i--
		}
		return classID(c.Outer) + "." + c.Name[i+1:]
	}
	if k, ok := kotlinOfJava[c.Name]; ok {
		return k
	}
	if c.External {
		return strings.ReplaceAll(c.Name, "$", ".") // a Java class
	}
	return c.Name
}

// typ converts a type to metadata.
func (s *metaScope) typ(t types.Type) *kmeta.Type {
	if t.P {
		// A platform type, T!: flexible, from T to T?, with read-only
		// collections as the upper bound.
		lower, upper := t, t
		lower.P, lower.N = false, false
		upper.P, upper.N = false, true
		if upper.K == types.Class && upper.C.ReadOnly != nil {
			upper.C = upper.C.ReadOnly
		}
		kt := s.typ(lower)
		kt.Upper = s.typ(upper)
		return kt
	}
	kt := &kmeta.Type{Param: -1, Nullable: t.N}
	if t.D {
		kt.Flags |= kmeta.TypeDefinitelyNonNull
	}
	switch {
	case t.K == types.Null:
		kt.Class, kt.Nullable = "kotlin/Nothing", true
	case t.IsParam():
		id, ok := s.ids[t.Param()]
		if !ok {
			// An enclosing function's, in an object expression's class:
			// by name.
			kt.ParamName = t.Param().Name
			break
		}
		kt.Param = id
	case t.K == types.Class && t.C.Suspend:
		// A suspend function type is its JVM interface's function type:
		// suspend (A) -> R is Function2<A, Continuation<R>, Any?>.
		n := t.A.Len() - 1
		kt.Class = "kotlin/Function" + strconv.Itoa(n+1)
		kt.ExtFunc, kt.Contexts = t.E, int(t.X)
		kt.Flags |= kmeta.TypeSuspend
		for _, a := range t.A.List[:n] {
			if a.V == types.Star {
				kt.Args = append(kt.Args, kmeta.Arg{Proj: kmeta.Star})
				continue
			}
			kt.Args = append(kt.Args, kmeta.Arg{Proj: variance(a.V), Type: s.typ(a.T)})
		}
		cont := &kmeta.Type{Param: -1, Class: "kotlin/coroutines/Continuation", Args: []kmeta.Arg{{Proj: kmeta.Inv, Type: s.typ(t.A.Get(n).T)}}}
		kt.Args = append(kt.Args, kmeta.Arg{Proj: kmeta.Inv, Type: cont}, kmeta.Arg{Proj: kmeta.Inv, Type: &kmeta.Type{Param: -1, Class: "kotlin/Any", Nullable: true}})
	case t.K == types.Class && t.C.Composable:
		// A composable function type is the function type of its
		// parameters, annotated @Composable.
		n := t.A.Len() - 1
		kt.Class = "kotlin/Function" + strconv.Itoa(n)
		kt.ExtFunc, kt.Composable, kt.Contexts = t.E, true, int(t.X)
		for _, a := range t.A.List {
			if a.V == types.Star {
				kt.Args = append(kt.Args, kmeta.Arg{Proj: kmeta.Star})
				continue
			}
			kt.Args = append(kt.Args, kmeta.Arg{Proj: variance(a.V), Type: s.typ(a.T)})
		}
	case t.K == types.Class:
		kt.Class = classID(t.C)
		kt.ExtFunc = t.E && t.C.Func
		if t.C.Func {
			kt.Contexts = int(t.X)
		}
		if t.C.Inner {
			// Its own arguments, and the outer class's in the outer type.
			n := len(t.C.TypeParams) - t.C.OuterParams
			var oargs []types.Arg
			if t.A != nil {
				oargs = t.A.List[n:]
			}
			kt.Outer = s.typ(types.GenericType(t.C.Outer, oargs))
			if t.A != nil {
				t.A = types.MakeArgs(t.A.List[:n])
			}
		}
		if t.A != nil {
			for _, a := range t.A.List {
				if a.V == types.Star {
					kt.Args = append(kt.Args, kmeta.Arg{Proj: kmeta.Star})
					continue
				}
				kt.Args = append(kt.Args, kmeta.Arg{Proj: variance(a.V), Type: s.typ(a.T)})
			}
		}
	default:
		id, ok := kindIDs[t.K]
		if !ok {
			panic("no metadata for type " + t.String())
		}
		kt.Class = id
	}
	return kt
}

func visibility(v syntax.Visibility) kmeta.Visibility {
	switch v {
	case syntax.Private:
		return kmeta.Private
	case syntax.Protected:
		return kmeta.Protected
	case syntax.Internal:
		return kmeta.Internal
	}
	return kmeta.Public
}

// annotated reports whether a declaration has annotations that class files
// keep, at the target place selects.
func annotated(as []syntax.Annotation, p placement) bool {
	for i := range as {
		a := &as[i]
		if a.Class != nil && !a.Implicit && (a.Target == p.site || a.Target == "" && p.dflt != nil && p.dflt(a.Class)) {
			return true
		}
	}
	return false
}

// lambdaMeta returns the function of a lambda, for its class's metadata.
// The type parameters of the declarations it is in are its own, as
// kotlinc writes them: D8 fails to read a type that refers to one that
// isn't declared.
func (g *gen) lambdaMeta(x *syntax.Lambda) *kmeta.Function {
	ts := []types.Type{x.Ret}
	for _, p := range x.ParamSyms() {
		ts = append(ts, p.Type)
	}
	s, tps := (*metaScope)(nil).with(enclosingParams(ts))
	f := &kmeta.Function{
		Flags:      kmeta.MakeFlags(false, kmeta.Local, kmeta.Final, kmeta.KindDeclaration),
		Name:       "<anonymous>",
		TypeParams: tps,
		Ret:        s.typ(x.Ret),
	}
	params := x.ParamSyms()[len(x.Ctx):] // as kotlinc's, without its context parameters
	if x.Recv != nil {
		f.Recv = s.typ(x.Recv.Type)
		params = params[1:]
	}
	for _, p := range params {
		f.Params = append(f.Params, &kmeta.Param{Name: p.Name, Type: s.typ(p.Type)})
	}
	return f
}

// enclosingParams returns the type parameters of the declarations whose
// type parameters types ts refer to, those of classes first: all of each
// one's, as kotlinc declares them in a lambda's function.
func enclosingParams(ts []types.Type) []*types.TypeParamSym {
	var classes, funs []*types.TypeParamSym
	seen := map[any]bool{}
	var visit func(t types.Type)
	visit = func(t types.Type) {
		t.Has(func(u types.Type) bool {
			if u.K != types.TypeParam || seen[u.Param().Owner] {
				return false
			}
			owner := u.Param().Owner
			seen[owner] = true
			var tps []*types.TypeParamSym
			switch o := owner.(type) {
			case *types.ClassSym:
				tps = o.TypeParams
				classes = append(classes, tps...)
			case *types.FunSym:
				tps = o.TypeParams
				funs = append(funs, tps...)
			default:
				tps = []*types.TypeParamSym{u.Param()}
				funs = append(funs, tps...)
			}
			for _, tp := range tps {
				for _, b := range tp.Bounds {
					visit(b)
				}
			}
			return false
		})
	}
	for _, t := range ts {
		visit(t)
	}
	return append(classes, funs...)
}

// params converts value parameters: decls are their declarations, if any.
func (s *metaScope) params(ps []*types.VarSym, decls []*syntax.Param) []*kmeta.Param {
	out := make([]*kmeta.Param, len(ps))
	for i, p := range ps {
		kp := &kmeta.Param{Name: p.Name, Type: s.typ(p.Type)}
		if p.Default && i >= len(decls) {
			// Without a declaration; an override's inherited defaults
			// aren't its own.
			kp.Flags |= kmeta.ParamDeclaresDefault
		}
		if i < len(decls) {
			d := decls[i]
			if d.Default != nil {
				kp.Flags |= kmeta.ParamDeclaresDefault
			}
			if d.Crossinline {
				kp.Flags |= kmeta.ParamCrossinline
			}
			if d.Noinline {
				kp.Flags |= kmeta.ParamNoinline
			}
			if annotated(d.Annotations, all) {
				kp.Flags |= kmeta.FlagAnnotations
			}
		}
		if p.Vararg {
			kp.Vararg = s.typ(types.VarargElem(p.Type))
		}
		out[i] = kp
	}
	return out
}

// function converts a function declared in class owner (nil at top level).
func (g *gen) funMeta(s *metaScope, fd *syntax.FunDecl, owner *types.ClassSym) *kmeta.Function {
	sym := fd.Sym
	s, tps := s.with(sym.TypeParams)
	mod := kmeta.Final
	switch {
	case sym.Abstract:
		mod = kmeta.Abstract
	case owner != nil && (owner.Interface && fd.Visibility != syntax.Private || fd.Open || fd.Override && !fd.Final):
		mod = kmeta.Open // an override is open unless it is final
	}
	f := &kmeta.Function{
		Flags:      kmeta.MakeFlags(annotated(fd.Annotations, all), visibility(fd.Visibility), mod, kmeta.KindDeclaration),
		Name:       sym.Name,
		TypeParams: tps,
		Params:     s.params(sym.Params, fd.Params),
		Ret:        s.typ(sym.Ret),
		JVM:        kmeta.JVMSig{Name: sym.JVM(), Desc: funDesc(sym)},
	}
	if sym.Ext {
		f.Recv = s.typ(sym.Recv)
	}
	for _, p := range sym.Contexts {
		name := p.Name
		if d, ok := p.Decl.(*syntax.Param); ok && d.Name == "_" {
			name = "<unused var>" // as kotlinc names it
		}
		f.ContextParams = append(f.ContextParams, &kmeta.Param{Name: name, Type: s.typ(p.Type)})
	}
	if fd.Operator || sym.KotlinOperator {
		f.Flags |= kmeta.FunOperator // overrides of operators are operators
	}
	switch sym.Inherited {
	case types.InheritProtected:
		f.Flags = f.Flags&^(7<<1) | kmeta.Flags(kmeta.Protected)<<1
	case types.InheritInternal:
		f.Flags = f.Flags&^(7<<1) | kmeta.Flags(kmeta.Internal)<<1
	}
	if fd.Infix || sym.Infix {
		f.Flags |= kmeta.FunInfix
	}
	if fd.Inline {
		f.Flags |= kmeta.FunInline
	}
	if fd.Tailrec {
		f.Flags |= kmeta.FunTailrec
	}
	if sym.External {
		f.Flags |= kmeta.FunExternal
	}
	if sym.Suspend {
		f.Flags |= kmeta.FunSuspend
	}
	if sym.Composable {
		// As kotlinc keeps them, which tells its callers the function is
		// composable.
		f.Flags |= kmeta.FlagAnnotations
		f.Annotations = append(f.Annotations, "androidx/compose/runtime/Composable")
		for _, a := range fd.Annotations {
			if a.Target == "" && composeAnno(a, "ReadOnlyComposable") {
				f.Annotations = append(f.Annotations, "androidx/compose/runtime/ReadOnlyComposable")
			}
		}
		if sym.ComposeDefaults && !sym.Abstract {
			// Abstract ones' callers know it.
			f.PluginData = append(f.PluginData, kmeta.ComposeMetadata)
		}
	}
	if owner != nil && owner.Value {
		f.JVM.Desc = "(" + desc(owner.Underlying.Type) + funDesc(sym)[1:]
	}
	if fd.SelfStatic {
		// As kotlinc's: a @JvmStatic function that takes the instance,
		// self, first.
		self := &kmeta.Param{Name: "self", Type: s.typ(owner.This())}
		f.Params = append([]*kmeta.Param{self}, f.Params...)
		f.JVM.Desc = "(L" + owner.Name + ";" + f.JVM.Desc[1:]
		f.Flags |= kmeta.FlagAnnotations
		f.Annotations = []string{"kotlin/jvm/JvmStatic"}
	}
	for i, once := range sym.Once {
		if once {
			// Its contract's callsInPlace(p, EXACTLY_ONCE).
			f.InPlace = append(f.InPlace, kmeta.InPlace{Param: i, Kind: kmeta.ExactlyOnce})
		}
	}
	for _, e := range sym.Implies {
		// returns(value) implies (p != null) or (p is T); not (p), which
		// metadata isn't written for.
		if e.Holds {
			continue
		}
		im := kmeta.Implies{Returns: kmeta.ReturnsAny, Param: e.Param}
		switch e.Returns {
		case 1:
			im.Returns = kmeta.ReturnsTrue
		case 0:
			im.Returns = kmeta.ReturnsFalse
		}
		if e.Is != nil {
			im.Is = s.typ(*e.Is)
		}
		f.Implies = append(f.Implies, im)
	}
	return f
}

// propMeta converts a property: pd is its declaration (nil for a
// constructor's property), in class owner (nil at top level).
func (g *gen) propMeta(s *metaScope, p *types.VarSym, pd *syntax.PropDecl, cp *syntax.CtorParam, owner *types.ClassSym) *kmeta.Property {
	var mods *syntax.Modifiers
	var as []syntax.Annotation
	switch {
	case pd != nil:
		mods, as = &pd.Modifiers, pd.Annotations
	case cp != nil:
		mods, as = &cp.Modifiers, cp.Modifiers.Annotations
	}
	var tps []*kmeta.TypeParam
	if pd != nil && pd.Getter != nil && pd.Getter.Sym != nil && (pd.Recv != nil || len(pd.TypeParams) > 0) {
		s, tps = s.with(pd.Getter.Sym.TypeParams)
	}
	mod := kmeta.Final
	switch {
	case owner != nil && owner.Annotation:
	case p.Abstract:
		mod = kmeta.Abstract
	case owner != nil && (owner.Interface || mods.Open || mods.Override && !mods.Final):
		mod = kmeta.Open
	}
	vis := visibility(mods.Visibility)
	if p.InheritsProtected {
		vis = kmeta.Protected // an override of a protected property
	}
	// hasAnnotations: annotations of the declaration, as kotlinc sets it,
	// @JvmField's on the field among them, but not its accessors' and
	// parameters'.
	ann := false
	for i := range as {
		switch as[i].Target {
		case "get", "set", "setparam", "param":
		default:
			if as[i].Class != nil {
				ann = true
			}
		}
	}
	flags := kmeta.MakeFlags(ann, vis, mod, kmeta.KindDeclaration) | kmeta.PropGetter
	accessor := kmeta.MakeFlags(false, vis, mod, 0)
	kp := &kmeta.Property{Name: p.Name, TypeParams: tps, Type: s.typ(p.Type), GetterFlags: accessor, SetterFlags: accessor}
	if p.Mutable {
		flags |= kmeta.PropVar | kmeta.PropSetter
	}
	if p.Lateinit {
		flags |= kmeta.PropLateinit
	}
	defer func() {
		if !p.Mutable {
			kp.SetterFlags = kp.Flags & 0x3f // left out, as kotlinc does
		}
	}()
	switch {
	case p.Const:
		flags |= kmeta.PropConst
		if _, ok := constValue(p); ok {
			flags |= kmeta.PropConstant
		}
	case !p.Mutable && pd != nil && pd.Field == nil && pd.Init != nil:
		// As kotlinc marks a val whose initializer is a constant.
		if _, ok := constOf(pd.Init, 0); ok {
			flags |= kmeta.PropConstant
		}
	}
	d := desc(types.FieldType(p))
	ctx := "" // the context parameters, which the accessors take first
	for _, q := range p.Contexts {
		ctx += paramDesc(q.Type)
		name := q.Name
		if d, ok := q.Decl.(*syntax.Param); ok && d.Name == "_" {
			name = "<unused var>" // as kotlinc names it
		}
		kp.ContextParams = append(kp.ContextParams, &kmeta.Param{Name: name, Type: s.typ(q.Type)})
	}
	getterDesc := "(" + ctx + ")" + desc(types.GetterType(p))
	var recv *types.Type
	if pd != nil && pd.Getter != nil && pd.Getter.Sym != nil && pd.Getter.Sym.Ext {
		recv = &pd.Getter.Sym.Recv
		kp.Recv = s.typ(*recv)
	}
	if pd != nil && pd.Getter != nil && pd.Getter.Sym != nil && (pd.Getter.Sym.Ext || ctx != "") {
		getterDesc = funDesc(pd.Getter.Sym)
	}
	getterDesc = composeGetterDesc(p, getterDesc)
	if owner != nil && owner.Value && p != owner.Underlying {
		getterDesc = "(" + desc(owner.Underlying.Type) + getterDesc[1:]
	}
	if annotated(as, placement{site: "get"}) || pd != nil && pd.Getter != nil && annotated(pd.Getter.Annotations, all) {
		kp.GetterFlags |= kmeta.FlagAnnotations
	}
	if annotated(as, placement{site: "set"}) || pd != nil && pd.Setter != nil && annotated(pd.Setter.Annotations, all) {
		kp.SetterFlags |= kmeta.FlagAnnotations
	}
	custom := pd != nil && pd.Getter != nil
	if pd != nil && p.Mutable && pd.SetterVis != syntax.Public {
		// A setter with a visibility of its own isn't a default one.
		kp.SetterFlags = kp.SetterFlags&^(7<<1) | kmeta.Flags(visibility(pd.SetterVis))<<1 | kmeta.AccessorNotDefault
	}
	switch {
	case pd != nil && pd.Field != nil:
		// A delegated property: its field holds the delegate.
		flags |= kmeta.PropDelegated
		kp.GetterFlags |= kmeta.AccessorNotDefault
		if p.Mutable {
			kp.SetterFlags |= kmeta.AccessorNotDefault
		}
		kp.Field = &kmeta.JVMSig{Name: pd.Field.Name, Desc: desc(pd.Field.Sym.Type)}
	case p.Abstract:
		// Default accessors, though an abstract extension property's are
		// declared, without bodies.
	default:
		if custom {
			kp.GetterFlags |= kmeta.AccessorNotDefault
		}
		if pd != nil && pd.Setter != nil {
			kp.SetterFlags |= kmeta.AccessorNotDefault
		}
		if p.HasField() && (pd == nil || pd.Recv == nil) {
			kp.Field = &kmeta.JVMSig{Name: p.Name, Desc: d}
		}
	}
	// inline val p get() = ... has an inline getter, as inline get() does.
	if pd != nil && (pd.InlineGetter || pd.Inline && pd.Getter != nil) {
		kp.GetterFlags |= kmeta.AccessorNotDefault | kmeta.AccessorInline
	}
	if pd != nil && (pd.InlineSetter || pd.Inline && pd.Setter != nil) {
		kp.SetterFlags |= kmeta.AccessorNotDefault | kmeta.AccessorInline
	}
	if pd != nil && pd.Getter != nil && pd.Getter.Sym != nil && pd.Getter.Sym.External {
		kp.GetterFlags |= kmeta.AccessorExternal
	}
	if pd != nil && pd.Setter != nil && pd.Setter.Sym != nil && pd.Setter.Sym.External {
		kp.SetterFlags |= kmeta.AccessorExternal
	}
	if p.Mutable && kp.SetterFlags&kmeta.AccessorNotDefault != 0 {
		// kotlinc reads the parameter of a setter that isn't a default one.
		name := "value"
		switch {
		case pd != nil && pd.Field != nil:
			name = "<set-?>"
		case pd != nil && pd.Setter != nil && len(pd.Setter.Params) == 1:
			name = pd.Setter.Params[0].Name
		}
		kp.SetterParam = &kmeta.Param{Name: name, Type: s.typ(p.Type)}
	}
	if owner != nil && owner.Annotation {
		kp.Getter = &kmeta.JVMSig{Name: p.Name, Desc: getterDesc}
		kp.Flags = flags
		return kp
	}
	hasAccessors := !p.Private && !p.Const && !p.JvmField
	if owner != nil && owner.Value && p == owner.Underlying && !underlyingGetter(owner) {
		hasAccessors = false
	}
	if hasAccessors {
		kp.Getter = &kmeta.JVMSig{Name: types.GetterJVM(p), Desc: getterDesc}
		if custom && pd.Getter.Sym != nil {
			kp.Getter = &kmeta.JVMSig{Name: pd.Getter.Sym.JVM(), Desc: getterDesc}
		}
		if p.Mutable && !p.SetterPrivate {
			sd := "(" + ctx + d + ")V"
			if owner != nil && owner.Value && p != owner.Underlying {
				sd = "(" + desc(owner.Underlying.Type) + d + ")V"
			}
			kp.Setter = &kmeta.JVMSig{Name: types.SetterJVM(p), Desc: sd}
			if pd != nil && pd.Setter != nil && pd.Setter.Sym != nil {
				kp.Setter.Name = pd.Setter.Sym.JVM()
				if pd.Setter.Sym.Ext || ctx != "" {
					kp.Setter.Desc = funDesc(pd.Setter.Sym)
					if owner != nil && owner.Value {
						kp.Setter.Desc = "(" + desc(owner.Underlying.Type) + kp.Setter.Desc[1:]
					}
				}
			}
		}
	}
	if m, ok := g.shared.annotated[p]; ok {
		kp.Synthetic = &kmeta.JVMSig{Name: m[0], Desc: m[1]}
	}
	kp.Flags = flags
	return kp
}

// packageMeta converts the file's top-level declarations.
func (g *gen) packageMeta() *kmeta.PackageData {
	out := &kmeta.PackageData{ModuleName: g.module()}
	for _, d := range g.file.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			out.Funcs = append(out.Funcs, g.funMeta(nil, d, nil))
		case *syntax.PropDecl:
			if d.DelegateOf != nil || d.Sym == nil {
				continue // a delegated property's field
			}
			out.Props = append(out.Props, g.propMeta(nil, d.Sym, d, nil, nil))
		case *syntax.TypeAliasDecl:
			out.Aliases = append(out.Aliases, aliasMeta(nil, d))
		}
	}
	return out
}

// aliasMeta converts a typealias. kc keeps no abbreviations in types, so
// the right side is written expanded, as the expansion is. Its
// annotations that aren't SOURCE ones are kept in metadata, as kotlinc
// keeps them: @Deprecated, and opt-in markers, which code using the
// alias then requires.
func aliasMeta(s *metaScope, d *syntax.TypeAliasDecl) *kmeta.TypeAlias {
	a := d.Sym
	s, tps := s.with(a.TypeParams)
	t := s.typ(a.Type)
	var as []*kmeta.Annotation
	for i := range d.Annotations {
		if an := &d.Annotations[i]; an.Class != nil && an.Target == "" {
			as = append(as, annotationMeta(annotationOf(an)))
		}
	}
	return &kmeta.TypeAlias{Flags: kmeta.MakeFlags(len(as) > 0, visibility(d.Visibility), kmeta.Final, 0), Name: a.Name, TypeParams: tps,
		Underlying: t, Expanded: t, Annotations: as}
}

// annotationMeta converts an annotation for metadata.
func annotationMeta(a cf.Annotation) *kmeta.Annotation {
	out := &kmeta.Annotation{Class: descClassID(a.Type)}
	for _, p := range a.Values {
		out.Args = append(out.Args, kmeta.AnnotationArg{Name: p.Name, Value: elementMeta(p.Value)})
	}
	return out
}

// elementMeta converts an annotation argument's value for metadata.
func elementMeta(v cf.ElementValue) kmeta.AnnotationValue {
	num := func() int64 {
		n, _ := v.Const.(int64)
		return n
	}
	switch v.Tag {
	case 'B':
		return kmeta.AnnotationValue{Kind: kmeta.ValueByte, Int: num()}
	case 'C':
		return kmeta.AnnotationValue{Kind: kmeta.ValueChar, Int: num()}
	case 'S':
		return kmeta.AnnotationValue{Kind: kmeta.ValueShort, Int: num()}
	case 'I':
		return kmeta.AnnotationValue{Kind: kmeta.ValueInt, Int: num()}
	case 'J':
		return kmeta.AnnotationValue{Kind: kmeta.ValueLong, Int: num()}
	case 'Z':
		return kmeta.AnnotationValue{Kind: kmeta.ValueBoolean, Int: num()}
	case 'F':
		f, _ := v.Const.(float32)
		return kmeta.AnnotationValue{Kind: kmeta.ValueFloat, Float: float64(f)}
	case 'D':
		f, _ := v.Const.(float64)
		return kmeta.AnnotationValue{Kind: kmeta.ValueDouble, Float: f}
	case 's':
		s, _ := v.Const.(string)
		return kmeta.AnnotationValue{Kind: kmeta.ValueString, Str: s}
	case 'c':
		return kmeta.AnnotationValue{Kind: kmeta.ValueClass, Class: descClassID(v.Class)}
	case 'e':
		return kmeta.AnnotationValue{Kind: kmeta.ValueEnum, Class: descClassID(v.Enum[0]), Str: v.Enum[1]}
	case '@':
		return kmeta.AnnotationValue{Kind: kmeta.ValueAnnotation, Ann: annotationMeta(*v.Ann)}
	}
	out := kmeta.AnnotationValue{Kind: kmeta.ValueArray}
	for _, e := range v.Elems {
		out.Elems = append(out.Elems, elementMeta(e))
	}
	return out
}

// descClassID returns the class id of the class of descriptor d, a
// Kotlin type's for a Java class that is one.
func descClassID(d string) string {
	if p, ok := primitiveClassIDs[d]; ok {
		return p
	}
	name := strings.TrimSuffix(strings.TrimPrefix(d, "L"), ";")
	if k, ok := kotlinOfJava[name]; ok {
		return k
	}
	return strings.ReplaceAll(name, "$", ".")
}

var primitiveClassIDs = map[string]string{
	"Z": "kotlin/Boolean", "B": "kotlin/Byte", "C": "kotlin/Char", "S": "kotlin/Short",
	"I": "kotlin/Int", "J": "kotlin/Long", "F": "kotlin/Float", "D": "kotlin/Double",
}

// inEnumEntry reports whether cls is an enum entry's class, or nested in
// one: kotlinc's metadata calls them local.
func inEnumEntry(cls *types.ClassSym) bool {
	for k := cls; k != nil; k = k.Outer {
		if cd, ok := k.Decl.(*syntax.ClassDecl); ok && cd.EnumEntry {
			return true
		}
	}
	return false
}

// classMeta converts a class.
func (g *gen) classMeta(cd *syntax.ClassDecl) *kmeta.Class {
	cls := cd.Sym
	// An inner class sees its outer class's type parameters, as numbered
	// in the outer class's metadata, and declares its own after them.
	own := cls.TypeParams[:len(cls.TypeParams)-cls.OuterParams]
	outer, _ := (*metaScope)(nil).with(cls.TypeParams[len(own):])
	if cls.OuterParams == 0 {
		outer = nil
	}
	s, tps := outer.with(own)
	mod := kmeta.Final
	kind := kmeta.KindClass
	switch {
	case cls.Annotation:
		kind = kmeta.KindAnnotation
	case cls.Interface:
		kind, mod = kmeta.KindInterface, kmeta.Abstract
	case cls.Entries != nil:
		kind = kmeta.KindEnum
	case cd.EnumEntry:
		kind = kmeta.KindEnumEntry
	case cls.CompanionOf != nil:
		kind = kmeta.KindCompanion
	case cls.Object:
		kind = kmeta.KindObject
	}
	switch {
	case cls.Sealed:
		mod = kmeta.Sealed
	case cd.Abstract:
		mod = kmeta.Abstract
	case cd.Open:
		mod = kmeta.Open
	}
	c := &kmeta.Class{
		Flags:      kmeta.MakeClassFlags(annotated(cd.Annotations, all), visibility(cd.Visibility), mod, kind),
		Name:       classID(cls),
		TypeParams: tps,
		ModuleName: g.module(),
	}
	switch {
	case cls.Anonymous:
		// Its JVM name, whose '$'s mark it local, and no constructors.
		c.Flags = kmeta.MakeClassFlags(false, kmeta.Local, mod, kind)
		c.Name = cls.Name
	case cls.Local:
		c.Flags = kmeta.MakeClassFlags(annotated(cd.Annotations, all), kmeta.Local, mod, kind)
		c.Name = cls.Name
	case inEnumEntry(cls):
		c.Flags = kmeta.MakeClassFlags(annotated(cd.Annotations, all), kmeta.Local, mod, kind)
	}
	if cd.Data {
		c.Flags |= kmeta.ClassData
	}
	if cls.Inner {
		c.Flags |= kmeta.ClassInner
	}
	if cls.Fun {
		c.Flags |= kmeta.ClassFun
	}
	if cls.Interface && !cls.Annotation {
		// Its bodies are default methods (kotlinc's -jvm-default=all),
		// and with -jvm-default=enable or @JvmDefaultWithCompatibility
		// also in DefaultImpls.
		c.JVMFlags = kmeta.JVMDefaultMode
		if cls.HasDefaultImpls() {
			c.JVMFlags = 0 // its bodies are in DefaultImpls
		} else if cls.JVMCompat {
			c.JVMFlags |= kmeta.JVMCompatibilityMode
		}
	}
	if cls.Value {
		c.Flags |= kmeta.ClassValue
		c.UnderlyingProp = cls.Underlying.Name
		if !underlyingGetter(cls) {
			// Its type, which kotlinc writes only if the property, which
			// then has no getter, isn't public.
			c.Underlying = s.typ(cls.Underlying.Type)
		}
	}
	if cls.Entries != nil {
		c.Flags |= kmeta.ClassEnumEntries
		c.EnumEntries = cls.Entries
	}
	if cls.Companion != nil {
		c.Companion = cls.Companion.Name[strings.LastIndexByte(cls.Companion.Name, '$')+1:]
	}
	c.Supers = g.supersMeta(s, cls)
	for _, m := range cd.Members {
		if nd, ok := m.(*syntax.ClassDecl); ok && !nd.EnumEntry {
			c.Nested = append(c.Nested, nd.Name) // $serializer of Point$$serializer
		}
	}
	if s := cd.Serial; s != nil && (cd.Open || cd.Abstract) && !cd.Sealed {
		for _, p := range s.All() {
			c.SerialProps = append(c.SerialProps, p.Sym().Name)
		}
	}
	for _, sub := range cls.Subclasses {
		c.SealedSubclasses = append(c.SealedSubclasses, classID(sub))
	}
	sortStrings(c.SealedSubclasses)
	if !cls.Anonymous && !cd.EnumEntry {
		c.Ctors = g.ctorsMeta(s, cd)
	}
	// Properties: the constructor's, then the body's, in order.
	for _, cp := range cd.CtorParams {
		if cp.IsProp && cp.PropSym != nil {
			c.Props = append(c.Props, g.propMeta(s, cp.PropSym, nil, cp, cls))
		}
	}
	for _, m := range cd.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			c.Funcs = append(c.Funcs, g.funMeta(s, m, cls))
		case *syntax.PropDecl:
			if m.DelegateOf != nil || m.Sym == nil || m.Plugin {
				continue
			}
			c.Props = append(c.Props, g.propMeta(s, m.Sym, m, nil, cls))
		case *syntax.TypeAliasDecl:
			c.Aliases = append(c.Aliases, aliasMeta(s, m))
		}
	}
	if cd.Data {
		c.Funcs = append(c.Funcs, g.dataMeta(s, cd)...)
	}
	if cls.Value {
		c.Funcs = append(c.Funcs, objectMembersMeta(cd, &cls.Underlying.Type)...)
	}
	for _, dl := range cd.Delegated {
		f := dl.Sym
		if f.Name == "" || strings.HasPrefix(f.Name, "<") || dl.Impls != "" {
			continue // a call of a DefaultImpls body: a fake override
		}
		if dl.ExtProp != "" {
			if len(f.Params) == 0 { // the getter
				c.Props = append(c.Props, delegatedExtPropMeta(s, dl.ExtProp, f, dl.Target))
			}
			continue
		}
		if name, ok := propertyOf(dl.Target); ok {
			if len(f.Params) == 0 { // the getter: a property, with its setter if any
				c.Props = append(c.Props, delegatedPropMeta(s, cd, name, f, dl.Target))
			}
			continue
		}
		fs, ftps := s.with(f.TypeParams)
		anyBounds(ftps)
		kf := &kmeta.Function{
			Flags:      kmeta.MakeFlags(false, kmeta.Public, kmeta.Open, kmeta.KindDelegation),
			Name:       f.Name,
			TypeParams: ftps,
			Params:     fs.params(f.Params, nil),
			Ret:        fs.typ(f.Ret),
			JVM:        kmeta.JVMSig{Name: f.JVM(), Desc: funDesc(f)},
		}
		if f.Ext {
			kf.Recv = fs.typ(f.Recv)
		}
		if f.Operator {
			kf.Flags |= kmeta.FunOperator
		}
		if f.Suspend {
			kf.Flags |= kmeta.FunSuspend
		}
		for _, p := range kf.Params {
			p.Flags &^= kmeta.ParamDeclaresDefault // the interface's, not its own
		}
		c.Funcs = append(c.Funcs, kf)
	}
	return c
}

// supersMeta returns a class's supertypes, as kotlinc writes them: Any
// for a class without supertypes.
func (g *gen) supersMeta(s *metaScope, cls *types.ClassSym) []*kmeta.Type {
	var out []*kmeta.Type
	for _, t := range cls.Supers {
		out = append(out, s.typ(t))
	}
	if len(out) == 0 {
		if cls.Annotation {
			return []*kmeta.Type{{Class: "kotlin/Annotation", Param: -1}}
		}
		return []*kmeta.Type{{Class: "kotlin/Any", Param: -1}}
	}
	return out
}

// withMarker returns constructor descriptor d with the unused marker last:
// that of the constructor that callers of one that takes a value class
// call, which metadata names, as kotlinc's does unless it is private (or
// a sealed class's, which is private on the JVM).
func withMarker(d string) string {
	return d[:len(d)-2] + "Lkotlin/jvm/internal/DefaultConstructorMarker;)V"
}

// ctorsMeta converts a class's constructors.
func (g *gen) ctorsMeta(s *metaScope, cd *syntax.ClassDecl) []*kmeta.Constructor {
	cls := cd.Sym
	if cls.Interface && !cls.Annotation {
		return nil
	}
	var out []*kmeta.Constructor
	if cls.Annotation {
		decls := make([]*syntax.Param, len(cd.CtorParams))
		for i, cp := range cd.CtorParams {
			decls[i] = &cp.Param
		}
		d := strings.ReplaceAll(methodDesc(cls.AnnotationParams, types.UnitType), "Lkotlin/reflect/KClass;", "Ljava/lang/Class;")
		return []*kmeta.Constructor{{Flags: kmeta.MakeFlags(false, kmeta.Public, 0, 0), Params: s.params(cls.AnnotationParams, decls),
			JVM: kmeta.JVMSig{Name: "<init>", Desc: d}}}
	}
	if cd.HasPrimary && cls.Ctor != nil {
		vis := visibility(cd.CtorMods.Visibility)
		switch {
		case cls.Object || cls.Entries != nil:
			vis = kmeta.Private
		case cls.Sealed && vis == kmeta.Public:
			vis = kmeta.Protected // a sealed class's constructors are
		}
		decls := make([]*syntax.Param, len(cd.CtorParams))
		for i, cp := range cd.CtorParams {
			decls[i] = &cp.Param
		}
		k := &kmeta.Constructor{Flags: kmeta.MakeFlags(annotated(cd.CtorMods.Annotations, all), vis, 0, 0),
			Params: s.params(cls.Ctor.Params, decls), JVM: kmeta.JVMSig{Name: "<init>", Desc: ctorDesc(cd)}}
		// A parameter's annotations are its modifiers', placed as
		// annotateCtor places them.
		for i, cp := range cd.CtorParams {
			pl := placement{"param", onCtorParam}
			if !cp.IsProp {
				pl = placement{"param", all.dflt}
			}
			if i < len(k.Params) && annotated(cp.Modifiers.Annotations, pl) {
				k.Params[i].Flags |= kmeta.FlagAnnotations
			}
		}
		if cls.Value {
			u := desc(cls.Underlying.Type)
			k.JVM = kmeta.JVMSig{Name: "constructor-impl", Desc: "(" + u + ")" + u}
		} else if hasValueParam(cls.Ctor) && vis != kmeta.Private && !cls.Sealed {
			k.JVM.Desc = withMarker(k.JVM.Desc)
		}
		out = append(out, k)
	}
	for _, sc := range cd.Secondaries() {
		f := sc.Sym
		jvm := kmeta.JVMSig{Name: "<init>", Desc: ctorJVMDesc(f)}
		if cls.Value {
			jvm = kmeta.JVMSig{Name: "constructor-impl", Desc: methodDesc(f.Params, cls.This())}
		} else if hasValueParam(f) && sc.Visibility != syntax.Private && !cls.Sealed {
			jvm.Desc = withMarker(jvm.Desc)
		}
		vis := visibility(sc.Visibility)
		switch {
		case cls.Entries != nil:
			vis = kmeta.Private
			jvm.Desc = "(Ljava/lang/String;I" + jvm.Desc[1:]
		case sc.Serial != nil && !cd.Open && !cd.Abstract && !cd.Sealed:
			vis = kmeta.Internal // as write$Self is
		}
		out = append(out, &kmeta.Constructor{
			Flags:  kmeta.MakeFlags(annotated(sc.Annotations, all), vis, 0, 0) | kmeta.CtorSecondary,
			Params: s.params(f.Params, sc.Params),
			JVM:    jvm,
		})
	}
	return out
}

// dataMeta returns the members a data class's compiler synthesizes, that
// it doesn't declare itself: componentN, copy, equals, hashCode and
// toString.
func (g *gen) dataMeta(s *metaScope, cd *syntax.ClassDecl) []*kmeta.Function {
	cls := cd.Sym
	var out []*kmeta.Function
	add := func(f *types.FunSym, vis kmeta.Visibility, mod kmeta.Modality, operator bool) {
		if f == nil || f.Decl != nil {
			return
		}
		kf := &kmeta.Function{Flags: kmeta.MakeFlags(false, vis, mod, kmeta.KindSynthesized), Name: f.Name,
			Params: s.params(f.Params, nil), Ret: s.typ(f.Ret), JVM: kmeta.JVMSig{Name: f.JVM(), Desc: funDesc(f)}}
		if operator {
			kf.Flags |= kmeta.FunOperator
		}
		for _, p := range kf.Params {
			if f.Name == "copy" {
				p.Flags |= kmeta.ParamDeclaresDefault
			}
		}
		out = append(out, kf)
	}
	for i, cp := range cd.CtorParams {
		// componentN has its property's visibility.
		add(synthesized(cls, componentName(i)), visibility(cp.Visibility), kmeta.Final, true)
	}
	add(synthesized(cls, "copy"), kmeta.Public, kmeta.Final, false)
	return append(out, objectMembersMeta(cd, nil)...)
}

// objectMembersMeta returns the equals, hashCode and toString that the
// compiler synthesizes for a data or value class that doesn't declare
// them; for a value class, static -impl methods that take the value u.
func objectMembersMeta(cd *syntax.ClassDecl, u *types.Type) []*kmeta.Function {
	declared := func(name string, params int) bool {
		for _, m := range cd.Members {
			if fd, ok := m.(*syntax.FunDecl); ok && fd.Name == name && len(fd.Params) == params {
				return true
			}
		}
		return false
	}
	t := func(id string, nullable bool) *kmeta.Type {
		return &kmeta.Type{Class: id, Param: -1, Nullable: nullable}
	}
	var out []*kmeta.Function
	add := func(name string, params []*kmeta.Param, ret *kmeta.Type, desc string) {
		if declared(name, len(params)) {
			return
		}
		jname := name
		if u != nil {
			jname += "-impl"
			desc = "(" + descOrPanic(*u) + desc[1:]
		}
		f := &kmeta.Function{Flags: kmeta.MakeFlags(false, kmeta.Public, kmeta.Open, kmeta.KindSynthesized), Name: name,
			Params: params, Ret: ret, JVM: kmeta.JVMSig{Name: jname, Desc: desc}}
		if name == "equals" {
			f.Flags |= kmeta.FunOperator
		}
		out = append(out, f)
	}
	add("equals", []*kmeta.Param{{Name: "other", Type: t("kotlin/Any", true)}}, t("kotlin/Boolean", false), "(Ljava/lang/Object;)Z")
	add("hashCode", nil, t("kotlin/Int", false), "()I")
	add("toString", nil, t("kotlin/String", false), "()Ljava/lang/String;")
	return out
}

func descOrPanic(t types.Type) string { return desc(t) }

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// propertyOf returns the name of the property whose getter or setter f
// is, if it is an accessor of an interface's property.
func propertyOf(f *types.FunSym) (string, bool) {
	if f == nil || f.Owner == nil {
		return "", false
	}
	for name, kp := range f.Owner.KProps {
		if kp.Getter == f || kp.Setter == f {
			return name, true
		}
	}
	for _, p := range f.Owner.Props {
		if p.Getter == f || p.Setter == f {
			return p.Name, true
		}
		if pd, ok := p.Decl.(*syntax.PropDecl); ok && (pd.Getter != nil && pd.Getter.Sym == f || pd.Setter != nil && pd.Setter.Sym == f) {
			return p.Name, true
		}
		if f.Name == types.GetterJVM(p) && len(f.Params) == 0 || f.Name == types.SetterJVM(p) && len(f.Params) == 1 {
			return p.Name, true
		}
	}
	return "", false
}

// delegatedExtPropMeta converts an extension property that class
// delegation implements, with getter getter.
func delegatedExtPropMeta(s *metaScope, name string, getter, target *types.FunSym) *kmeta.Property {
	s, tps := s.with(getter.TypeParams)
	anyBounds(tps)
	accessor := kmeta.MakeFlags(false, kmeta.Public, kmeta.Open, 0)
	// An accessor is a default one unless the interface's has a body.
	flags := func(f *types.FunSym) kmeta.Flags {
		if fd, ok := f.Decl.(*syntax.FunDecl); ok && (fd.Body != nil || fd.ExprBody != nil) {
			return accessor | kmeta.AccessorNotDefault
		}
		return accessor
	}
	kp := &kmeta.Property{Name: name, TypeParams: tps, Recv: s.typ(getter.Recv), Type: s.typ(getter.Ret), GetterFlags: flags(target),
		Flags:  kmeta.MakeFlags(false, kmeta.Public, kmeta.Open, kmeta.KindDelegation) | kmeta.PropGetter,
		Getter: &kmeta.JVMSig{Name: getter.JVM(), Desc: funDesc(getter)}}
	kp.SetterFlags = kp.Flags & 0x3f
	if set := getter.ExtSetter; set != nil {
		kp.Flags |= kmeta.PropVar | kmeta.PropSetter
		kp.SetterFlags = flags(target.ExtSetter)
		kp.Setter = &kmeta.JVMSig{Name: set.JVM(), Desc: funDesc(set)}
		if kp.SetterFlags&kmeta.AccessorNotDefault != 0 {
			kp.SetterParam = &kmeta.Param{Name: set.Params[0].Name, Type: s.typ(set.Params[0].Type)}
		}
	}
	return kp
}

// anyBounds gives the type parameters without bounds Any?, as kotlinc
// writes those of the members that class delegation implements.
func anyBounds(tps []*kmeta.TypeParam) {
	for _, tp := range tps {
		if len(tp.Bounds) == 0 {
			tp.Bounds = []*kmeta.Type{{Class: "kotlin/Any", Nullable: true, Param: -1}}
		}
	}
}

// delegatedPropMeta converts a property that class delegation implements,
// with getter getter, and a setter if the class has one.
func delegatedPropMeta(s *metaScope, cd *syntax.ClassDecl, name string, getter, target *types.FunSym) *kmeta.Property {
	accessor := kmeta.MakeFlags(false, kmeta.Public, kmeta.Open, 0)
	// The interface's property's annotations are its too.
	ann := false
	for _, p := range target.Owner.Props {
		if pd, ok := p.Decl.(*syntax.PropDecl); ok && p.Name == name {
			ann = annotated(pd.Annotations, placement{"property", onProperty})
			if pd.Getter != nil {
				accessor |= kmeta.AccessorNotDefault // its getter has a body
			}
		}
	}
	kp := &kmeta.Property{Name: name, Type: s.typ(getter.Ret), GetterFlags: accessor,
		Flags:  kmeta.MakeFlags(ann, kmeta.Public, kmeta.Open, kmeta.KindDelegation) | kmeta.PropGetter,
		Getter: &kmeta.JVMSig{Name: getter.JVM(), Desc: funDesc(getter)}}
	kp.SetterFlags = kp.Flags & 0x3f
	for _, dl := range cd.Delegated {
		if f := dl.Sym; f != getter && len(f.Params) == 1 && f.Name == types.SetterName(name) {
			kp.Flags |= kmeta.PropVar | kmeta.PropSetter
			kp.SetterFlags = accessor
			kp.Setter = &kmeta.JVMSig{Name: f.JVM(), Desc: funDesc(f)}
		}
	}
	return kp
}
