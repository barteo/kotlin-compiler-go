package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"slices"
)

// Explicit backing fields, val x: List<T> field = mutableListOf(): the
// property's field has a type of its own, a subtype of the property's,
// which reading the property gives where the field is visible, in the
// property's private scope.

// backingFieldDecl reports the uses of an explicit backing field that
// Kotlin forbids, before property d is resolved. A field it allows
// initializes the property with its own initializer, if it has one.
func (c *Checker) backingFieldDecl(d *syntax.PropDecl, owner *types.ClassSym) {
	bf := d.BackingField
	if bf == nil || bf.Checked {
		return
	}
	bf.Checked = true
	n := len(c.errs)
	for _, m := range bf.Mods {
		c.errorf(m.P, "modifier '%s' is not applicable to 'backing field'", m.Name)
	}
	if d.Mutable {
		c.errorf(d.P, "a 'var' property cannot have an explicit backing field")
	}
	switch {
	case d.Field != nil:
		c.errorf(bf.P, "a delegated property cannot have a backing field")
	case d.Recv != nil:
		c.errorf(bf.P, "an extension property cannot have an explicit backing field")
	case owner != nil && owner.Interface:
		c.errorf(bf.P, "an explicit backing field is not allowed in an interface")
	case d.Abstract:
		c.errorf(bf.P, "an abstract property cannot have an explicit backing field")
	case d.Expect:
		c.errorf(bf.P, "an expect property cannot have an explicit backing field")
	case d.Const:
		c.errorf(bf.P, "a const property cannot have an explicit backing field")
	case len(d.Contexts) > 0:
		c.contextsWithField(d)
	case d.Open || d.Override && !d.Final && owner != nil && !owner.Final:
		c.errorf(bf.P, "a property with an explicit backing field must be final")
	}
	if d.Getter != nil || d.Setter != nil || d.AccessorP.Line != 0 {
		p := d.AccessorP
		if p.Line == 0 {
			p = bf.P
		}
		c.errorf(p, "a property with an explicit backing field cannot have accessors")
	}
	if d.Init != nil {
		c.errorf(d.Init.Pos(), "a property with an explicit backing field cannot have an initializer")
	}
	if d.Type == nil {
		c.errorf(d.P, "the type of a property with an explicit backing field must be written")
	}
	if d.Visibility == syntax.Private {
		c.errorf(d.P, "a private property's explicit backing field must be less visible than it")
	}
	if d.Lateinit {
		c.errorf(d.P, "'lateinit' modifier is not allowed on properties with an explicit backing field")
	}
	if a := d.Find("", "JvmField"); a != nil {
		c.errorf(a.P, "@JvmField cannot be applied to a property with an explicit backing field")
	}
	if bf.Type == nil && bf.Init == nil {
		// Of no type, which kotlinc reports as not one of the property's.
		c.errorf(d.P, "the type of the backing field must be a subtype of the property's type")
		c.errorf(bf.P, "an explicit backing field must have a type or an initializer")
	}
	if bf.Init == nil && d.Init == nil && (owner == nil || !owner.Interface && (d.Getter != nil || d.Setter != nil)) {
		// A class's is checked by definite assignment, but for one with
		// accessors, which the constructors can't assign.
		c.errorf(bf.P, "an explicit backing field must be initialized")
	}
	bf.Allowed = len(c.errs) == n
	if d.Init == nil && d.Field == nil && d.Getter == nil && !d.Abstract && (owner == nil || !owner.Interface) {
		d.Init = bf.Init // checked as the property's, if the field isn't allowed
	}
}

// backingFieldType gives property d, once resolved, the type of its
// explicit backing field: the one written, or its initializer's, which is
// checked first.
func (c *Checker) backingFieldType(d *syntax.PropDecl, owner *types.ClassSym) {
	bf, sym := d.BackingField, d.Sym
	if bf == nil || !sym.Resolved {
		return
	}
	if !bf.Allowed {
		c.disallowedFieldType(d, owner)
		return
	}
	if bf.Type == nil {
		// Inferred before the bodies are checked, like an omitted type.
		sym.Resolved = false
		sym.Infer = c.lazily(&sym.Resolved, func() { c.inferBackingField(d, owner) })
		return
	}
	ft := c.resolveType(bf.Type)
	bf.Redundant = c.sameFieldType(ft, sym.Type)
	if !c.fieldNarrows(ft, sym.Type, d.P) {
		if ft.K != types.Invalid && ft != sym.Type {
			// Reported: its initializer is still checked with the type
			// written, as kotlinc does.
			sym.FieldT = &ft
		}
		return
	}
	sym.FieldT = &ft
}

// disallowedFieldType reports an explicit backing field reported for
// what else property d has, as accessors, whose type isn't a subtype of
// the property's written type, as kotlinc does too. Its initializer is
// checked here unless it is checked as the property's.
func (c *Checker) disallowedFieldType(d *syntax.PropDecl, owner *types.ClassSym) {
	bf, sym := d.BackingField, d.Sym
	if d.Type == nil || d.Field != nil || d.Recv != nil || sym.Type.K == types.Invalid {
		return
	}
	var ft types.Type
	switch {
	case bf.Type != nil:
		ft = c.resolveType(bf.Type)
	case bf.Init != nil && d.Init != bf.Init && !c.done[d]:
		saved := c.state
		ft = c.fieldInit(d, owner, sym.Type, false)
		c.state = saved
	default:
		return
	}
	c.fieldNarrows(ft, sym.Type, d.P)
}

// fieldNarrows reports whether a backing field of type ft narrows a
// property of type t, and reports it if it can't hold t's values.
// It is reported at p, the property, as kotlinc does.
func (c *Checker) fieldNarrows(ft, t types.Type, p syntax.Pos) bool {
	switch {
	case ft.K == types.Invalid || t.K == types.Invalid:
		return false
	case !c.subtype(ft, t):
		c.errorf(p, "the type of the backing field, %s, must be a subtype of the property's, %s", ft, t)
		return false
	}
	return ft != t // a field of the property's type is redundant
}

// sameFieldType reports whether a backing field of type ft is of t, the
// property's type, as kotlinc's REDUNDANT_EXPLICIT_BACKING_FIELD judges
// it: each a subtype of the other, so that a field of (Int) -> Unit is
// one of Int.() -> Unit.
func (c *Checker) sameFieldType(ft, t types.Type) bool {
	if ft.K == types.Invalid || t.K == types.Invalid || ft.P || t.P || ft.HasVars() || t.HasVars() {
		return false
	}
	return ft == t || c.subtype(ft, t) && c.subtype(t, ft)
}

// redundantBackingFields reports the explicit backing fields of the
// file's properties that are of the property's type: kotlinc's
// REDUNDANT_EXPLICIT_BACKING_FIELD.
func (c *Checker) redundantBackingFields() {
	var visit func(ds []syntax.Decl)
	visit = func(ds []syntax.Decl) {
		for _, d := range ds {
			switch d := d.(type) {
			case *syntax.PropDecl:
				if bf := d.BackingField; bf != nil && bf.Allowed && bf.Redundant {
					c.warnf(bf.P, "REDUNDANT_EXPLICIT_BACKING_FIELD", "explicit backing field declaration is unnecessary if it has the same type as the property")
				}
			case *syntax.ClassDecl:
				if !d.Generated && !d.Synthetic {
					visit(d.Members)
				}
			}
		}
	}
	visit(c.file.Decls)
}

// inferBackingField checks the initializer of property d's backing field,
// which gives its type, as inferProp does an omitted type's.
func (c *Checker) inferBackingField(d *syntax.PropDecl, owner *types.ClassSym) {
	sym := d.Sym
	if sym.InProgress {
		return // read in its own initializer: as the property's type
	}
	sym.InProgress = true
	saved := c.state
	t := c.fieldInit(d, owner, sym.Type, false)
	c.state = saved
	sym.InProgress, sym.Resolved = false, true
	d.BackingField.Redundant = c.sameFieldType(t, sym.Type)
	if c.fieldNarrows(t, sym.Type, d.P) {
		sym.FieldT = &t
	}
}

// checkBackingField checks the initializer of property d's backing field
// with the field's type, unless inferBackingField has.
func (c *Checker) checkBackingField(d *syntax.PropDecl, cls *types.ClassSym) {
	if bf := d.BackingField; bf == nil || bf.Init == nil || !bf.Allowed || c.done[d] {
		return
	}
	want := d.Sym.Type
	if ft := d.Sym.FieldT; ft != nil {
		want = *ft
	}
	c.fieldInit(d, cls, want, true)
}

// fieldInit checks the initializer of property d's backing field, which
// has the type want if known, and returns its type. Unless assign, its
// type isn't checked against want: an inferred field's type that the
// property's doesn't take is reported as such (fieldNarrows).
func (c *Checker) fieldInit(d *syntax.PropDecl, cls *types.ClassSym, want types.Type, assign bool) types.Type {
	c.done[d] = true
	c.state = state{scope: &scope{}, cls: cls, this: newThis(cls), init: cls != nil, declName: d.Name}
	if cls != nil {
		c.scope = c.initScope(cls)
		c.tparams = cls.TypeParams
	}
	c.inAnon(cls)
	init := d.BackingField.Init
	t := c.value(init, want)
	if assign {
		c.assignable(t, want, init.Pos())
	}
	if cls == nil {
		c.uninitializedTop(d, init)
	}
	return t
}

// uninitializedTop reports the reads of top-level properties that top-level
// property d's backing field initializer, init, makes before they are
// initialized: of d itself or of a property declared after it in the
// file, where init runs them, not in its lambdas or its objects' members.
func (c *Checker) uninitializedTop(d *syntax.PropDecl, init syntax.Expr) {
	order := map[*syntax.PropDecl]int{}
	for i, x := range c.file.Decls {
		if pd, ok := x.(*syntax.PropDecl); ok {
			order[pd] = i
		}
	}
	at := order[d]
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			return false
		case *syntax.ObjectExpr:
			for _, st := range n.Decl.Supers {
				if st.Call != nil {
					syntax.Inspect(st.Call, visit)
				}
			}
			return false
		case *syntax.Ident:
			if n.Sym != nil && n.Sym.Kind == types.Global {
				if pd, ok := n.Sym.Decl.(*syntax.PropDecl); ok {
					if i, ok := order[pd]; ok && i >= at && pd.Getter == nil && pd.Field == nil && !pd.Const {
						c.errorf(n.P, "variable '%s' must be initialized", n.Name)
					}
				}
			}
		}
		return true
	}
	syntax.Inspect(init, visit)
}

// fieldRead returns the type that reading property p on a receiver of
// type recv at p gives, if it has an explicit backing field visible
// there: the field's.
func (c *Checker) fieldRead(p *types.VarSym, recv types.Type, at syntax.Pos) (types.Type, bool) {
	if p == nil || p.FieldT == nil || at.Line == 0 || c.inRef > 0 || !c.fieldVisible(p, at) {
		return types.InvalidType, false
	}
	ft := *p.FieldT
	if p.Owner != nil {
		if !c.isThis(recv, p.Owner) && ft.Has(func(u types.Type) bool { return u.K == types.TypeParam && u.Param().Var == types.In }) {
			// An in parameter's type argument, which a value of another
			// object's field needn't have.
			return types.InvalidType, false
		}
		ft = c.memberSubst(recv, p.Owner).Apply(ft)
	}
	return ft, ft.K != types.Invalid
}

// fieldVisible reports whether property p's explicit backing field is
// visible at position at of the file: in the body of p's class, or its
// file for a top-level property, but not in public API inline functions,
// which other modules inline.
func (c *Checker) fieldVisible(p *types.VarSym, at syntax.Pos) bool {
	d, ok := p.Decl.(*syntax.PropDecl)
	if !ok {
		return false
	}
	if p.Owner == nil {
		if !inDecls(c.file.Decls, d) {
			return false
		}
	} else if cd, ok := p.Owner.Decl.(*syntax.ClassDecl); !ok || p.Owner.File != c.file.Class() || !within(at, cd.P, cd.End) {
		return false
	}
	return !c.inPublicInline(at)
}

func inDecls(ds []syntax.Decl, d *syntax.PropDecl) bool {
	for _, x := range ds {
		if x == syntax.Decl(d) {
			return true
		}
	}
	return false
}

func within(at, start, end syntax.Pos) bool {
	after := at.Line > start.Line || at.Line == start.Line && at.Col >= start.Col
	before := at.Line < end.Line || at.Line == end.Line && at.Col <= end.Col
	return after && before
}

// inPublicInline reports whether position at of the file is in a public
// API inline function or accessor, declared at the top level or in a
// class nested in one: not in a local class, an object expression's or an
// enum entry's.
func (c *Checker) inPublicInline(at syntax.Pos) bool {
	var in func(ds []syntax.Decl, cls *types.ClassSym) bool
	in = func(ds []syntax.Decl, cls *types.ClassSym) bool {
		for _, x := range ds {
			switch x := x.(type) {
			case *syntax.ClassDecl:
				if x.Sym != nil && !x.EnumEntry && within(at, x.P, x.End) && in(x.Members, x.Sym) {
					return true
				}
			case *syntax.FunDecl:
				if x.Inline && publicAPIInline(x, cls) && within(at, x.P, x.End) {
					return true
				}
			case *syntax.PropDecl:
				for _, a := range []*syntax.FunDecl{x.Getter, x.Setter} {
					inline := x.Inline || a == x.Getter && x.InlineGetter || a == x.Setter && x.InlineSetter
					if a != nil && inline && publicAPIInline(&syntax.FunDecl{Modifiers: x.Modifiers}, cls) && within(at, a.P, a.End) {
						return true
					}
				}
			}
		}
		return false
	}
	return in(c.file.Decls, nil)
}

// identField returns the type and the smart cast of a read of sym by its
// name at p, of type t: its backing field's type where that is visible.
func (c *Checker) identField(sym *types.VarSym, t types.Type, cast bool, p syntax.Pos) (types.Type, bool) {
	if cast || sym.Kind != types.Global && (sym.Kind != types.Prop || c.cls == nil) {
		return t, cast
	}
	var recv types.Type
	if sym.Kind == types.Prop {
		recv = c.cls.This()
	}
	if ft, ok := c.fieldRead(sym, recv, p); ok {
		return ft, needsCast(t, ft)
	}
	return t, cast
}

// memberField gives a read of a property, x of type t on a receiver of
// type rt, its backing field's type where that is visible.
func (c *Checker) memberField(x *syntax.Member, rt, t types.Type) types.Type {
	if _, super := x.X.(*syntax.Super); super || x.Cast || x.Safe {
		return t
	}
	p := x.Prop
	if id, ok := x.X.(*syntax.Ident); ok && id.Sym != nil && (p == nil || p.FieldT == nil) {
		// A smart-cast variable's property, an interface's, which its
		// declared type's, with a field, implements.
		if d := c.classOf(id.Sym.Type.NonNull()); d != nil && !d.External {
			if q := c.memberProp(d, x.Name); q != nil && q.FieldT != nil {
				p, rt = q, id.Sym.Type.NonNull()
			}
		}
	}
	if crt, ok := c.captureReceiver(x, p, rt); ok {
		rt = crt
	}
	if ft, ok := c.fieldRead(p, rt, x.P); ok {
		x.Cast = needsCast(t, ft)
		return ft
	}
	return t
}

// captureReceiver captures the projected type arguments of a stable
// receiver of type rt whose property p's explicit backing field it reads,
// and smart casts the receiver to the captured type, as K2 does: after
// val v = o.p, for an o of type C<*> and a field of type E, v is of the
// capture of the star, which o's members take, o.consume(v).
func (c *Checker) captureReceiver(x *syntax.Member, p *types.VarSym, rt types.Type) (types.Type, bool) {
	if p == nil || p.FieldT == nil || p.Owner == nil || c.inRef > 0 || !c.fieldVisible(p, x.P) {
		return rt, false
	}
	r := rt.NonNull()
	if r.K != types.Class || r.C != p.Owner || r.A == nil || r.A.Len() != len(p.Owner.TypeParams) {
		return rt, false
	}
	args := slices.Clone(r.A.List)
	changed := false
	for i, a := range args {
		tp := p.Owner.TypeParams[i]
		if a.V == types.Inv || a.V == types.In || !p.FieldT.Has(func(u types.Type) bool { return u.K == types.TypeParam && u.Param() == tp }) {
			continue
		}
		bounds := tp.Bounds
		if a.V != types.Star {
			bounds = []types.Type{a.T}
		}
		capt := &types.TypeParamSym{Name: "Captured(" + a.String() + ")", Var: types.Inv, Bounds: bounds, Index: tp.Index}
		args[i] = types.Arg{T: types.ParamType(capt)}
		changed = true
	}
	if !changed {
		return rt, false
	}
	k, ok := c.narrowKeyOf(x.X)
	if !ok {
		return rt, false
	}
	k.t = types.GenericType(r.C, args)
	k.not = nil
	c.scope.narrows = append(c.scope.narrows, k)
	return k.t, true
}

// isThis reports whether a receiver of type recv is this, an instance of
// cls: its type is cls's own, with its type parameters as arguments.
func (c *Checker) isThis(recv types.Type, cls *types.ClassSym) bool {
	return c.cls != nil && c.cls.IsSubclassOf(cls) && recv == c.cls.This()
}

// fieldShadowedByParam reports whether a setter's parameter is named
// field, set(field), which hides the backing field in its body.
func fieldShadowedByParam(a *syntax.FunDecl) bool {
	for _, p := range a.Params {
		if p.Name == "field" {
			return true
		}
	}
	return false
}

// fieldInBlock walks a block's statements with f in order, up to a local
// named field, which hides the backing field from the statements after
// it (val field = 2; return field).
func fieldInBlock(b *syntax.Block, f func(any) bool) {
	for _, st := range b.Stmts {
		if v, ok := st.(*syntax.VarStmt); ok && v.Name == "field" {
			syntax.Inspect(v.Init, f)
			syntax.Inspect(v.Delegate, f)
			return
		}
		syntax.Inspect(st, f)
	}
}
