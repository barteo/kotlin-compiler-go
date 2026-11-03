package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
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
		c.errorf(bf.P, "a property with context parameters cannot have a backing field")
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
	if d.Find("", "JvmField") != nil {
		c.errorf(d.P, "@JvmField cannot be applied to a property with an explicit backing field")
	}
	if bf.Type == nil && bf.Init == nil {
		c.errorf(bf.P, "an explicit backing field must have a type or an initializer")
	}
	if bf.Init == nil && owner == nil {
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
	if bf == nil || !bf.Allowed || !sym.Resolved {
		return
	}
	if bf.Type == nil {
		// Inferred before the bodies are checked, like an omitted type.
		sym.Resolved = false
		sym.Infer = func() { c.inferBackingField(d, owner) }
		return
	}
	ft := c.resolveType(bf.Type)
	if !c.fieldNarrows(ft, sym.Type, bf.P) {
		return
	}
	sym.FieldT = &ft
}

// fieldNarrows reports whether a backing field of type ft narrows a
// property of type t, and reports it if it can't hold t's values.
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

// inferBackingField checks the initializer of property d's backing field,
// which gives its type, as inferProp does an omitted type's.
func (c *Checker) inferBackingField(d *syntax.PropDecl, owner *types.ClassSym) {
	sym := d.Sym
	if sym.InProgress {
		return // read in its own initializer: as the property's type
	}
	sym.InProgress = true
	saved, trial := c.state, c.trial
	c.trial = c.trialOf(owner)
	t := c.fieldInit(d, owner, sym.Type)
	c.state, c.trial = saved, trial
	sym.InProgress, sym.Resolved = false, true
	if c.fieldNarrows(t, sym.Type, d.BackingField.P) {
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
	c.fieldInit(d, cls, want)
}

// fieldInit checks the initializer of property d's backing field, which
// has the type want if known, and returns its type.
func (c *Checker) fieldInit(d *syntax.PropDecl, cls *types.ClassSym, want types.Type) types.Type {
	c.done[d] = true
	c.onUndo(func() { delete(c.done, d) })
	c.state = state{scope: &scope{}, cls: cls, this: newThis(cls), init: cls != nil, declName: d.Name}
	if cls != nil {
		c.scope = c.ctorScope(cls)
		c.tparams = cls.TypeParams
	}
	c.inAnon(cls)
	init := d.BackingField.Init
	t := c.value(init, want)
	c.assignable(t, want, init.Pos())
	return t
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
				if x.Sym != nil && within(at, x.P, x.End) && in(x.Members, x.Sym) {
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
	if ft, ok := c.fieldRead(p, rt, x.P); ok {
		x.Cast = needsCast(t, ft)
		return ft
	}
	return t
}

// isThis reports whether a receiver of type recv is this, an instance of
// cls: its type is cls's own, with its type parameters as arguments.
func (c *Checker) isThis(recv types.Type, cls *types.ClassSym) bool {
	return c.cls != nil && c.cls.IsSubclassOf(cls) && recv == c.cls.This()
}
