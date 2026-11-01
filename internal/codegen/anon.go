package codegen

import (
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Object expressions (see check/anon.go) are instances of anonymous
// classes, which are generated after the file's other classes. A class's
// constructor takes what its members capture, and keeps it in fields:
// this$0 for the enclosing class's this, $x for a local x (its array, if
// it is a var). Each method first copies the fields to locals, which the
// captured variables' slots then are, so that the members' code reads
// them as the enclosing code does.

// anonClasses generates the classes of the file's object expressions.
func (g *gen) anonClasses(out []Output) []Output {
	for _, cd := range g.file.Anon {
		saved := map[*types.VarSym]int{}
		for _, v := range cd.Captures {
			saved[v] = g.slot(v)
		}
		out = g.classes(cd, out)
		g.restoreSlots(saved)
	}
	return out
}

// captureField returns the field that holds a capture of an object
// expression's class.
func captureField(cd *syntax.ClassDecl, v *types.VarSym) string {
	if v == cd.OuterThis {
		return "this$0"
	}
	return "$" + v.Name
}

// anonCtorDesc returns the descriptor of an object expression's class's
// constructor, which takes its captures.
func anonCtorDesc(cd *syntax.ClassDecl) string {
	d := "("
	for _, v := range cd.Captures {
		d += storageDesc(v)
	}
	return d + ")V"
}

// objectExpr creates an object expression's object, passing its class's
// constructor what it captures. In a copy of an inline function with
// reified type parameters, it is an object of a copy of the class, with
// the copy's type arguments (see nameObjects); in the function's method,
// copiers are told to copy the class, as kotlinc's are.
func (g *gen) objectExpr(x *syntax.ObjectExpr) {
	cd := x.Decl
	reified := reifiedOwner(cd) != nil
	if oc := g.shared.copies[cd.Sym]; oc != nil {
		if !oc.done {
			oc.reified, oc.done = g.copyObject(cd), true
		}
		reified = oc.reified
	}
	if reified && g.shared.opts.Stdlib {
		g.code.Invoke(cf.INVOKESTATIC, intrinsics, "needClassReification", "()V")
	}
	name := cd.Sym.Name
	g.code.New(name)
	g.code.Op(cf.DUP)
	for _, v := range cd.Captures {
		if v == cd.OuterThis {
			g.loadThis()
		} else {
			g.loadStorage(v)
		}
	}
	g.code.Invoke(cf.INVOKESPECIAL, name, "<init>", anonCtorDesc(cd))
}

// reifiedOwner returns the inline function with reified type parameters
// that object expression cd is in, or nil.
func reifiedOwner(cd *syntax.ClassDecl) *types.FunSym {
	if f := cd.EnclosingFun; f != nil {
		if fd, ok := f.Decl.(*syntax.FunDecl); ok && reifiedInline(fd) {
			return f
		}
	}
	return nil
}

// objectCopy is the copy of an object expression's class for a copy of
// the inline function it is in, which is generated where the copy first
// creates its object; reified is set if the copy still operates on
// reified type parameters, those of the inline function the copy is in,
// whose copiers then copy it again.
type objectCopy struct {
	done, reified bool
}

// renamed is a class that nameObjects renamed, with its name and copy
// before.
type renamed struct {
	cls  *types.ClassSym
	name string
	copy *objectCopy
}

// nameObjects names the copies of the classes of the object expressions
// in inline function d, for a copy of d with reified type arguments, as
// kotlinc names them: Caller$method$$inlined$d$1. The classes are renamed
// until the copy ends (see restoreNames), so that its code, from the
// locals that hold the objects, uses the copies. The copies hold
// inlineMu, as does the classes' own file while it reads their names (see
// GenerateFile).
func (g *gen) nameObjects(d *syntax.FunDecl) {
	if g.shared.copies == nil {
		g.shared.copies = map[*types.ClassSym]*objectCopy{}
	}
	base := g.cls.Name + "$" + g.code.Name() + "$$inlined$" + d.Name
	var rename func(cd *syntax.ClassDecl, from, to string)
	rename = func(cd *syntax.ClassDecl, from, to string) {
		g.shared.renamed = append(g.shared.renamed, renamed{cd.Sym, cd.Sym.Name, g.shared.copies[cd.Sym]})
		cd.Sym.Name = to + strings.TrimPrefix(cd.Sym.Name, from)
		for _, m := range cd.Members {
			if nd, ok := m.(*syntax.ClassDecl); ok && nd.Sym != nil {
				rename(nd, from, to)
			}
		}
	}
	visit := func(n any) bool {
		x, ok := n.(*syntax.ObjectExpr)
		if !ok || x.Decl.Sym == nil {
			return true
		}
		var name string
		for name == "" || g.file.Named[name] {
			g.shared.count[base]++
			name = base + "$" + strconv.Itoa(g.shared.count[base])
		}
		rename(x.Decl, x.Decl.Sym.Name, name)
		g.shared.copies[x.Decl.Sym] = &objectCopy{}
		return true
	}
	for _, p := range d.Params {
		syntax.Inspect(p.Default, visit)
	}
	syntax.Inspect(d.Body, visit)
	syntax.Inspect(d.ExprBody, visit)
}

// restoreNames restores the names, and copies, of the classes renamed
// after the first n renames.
func (g *gen) restoreNames(n int) {
	r := g.shared.renamed
	for i := len(r) - 1; i >= n; i-- {
		r[i].cls.Name = r[i].name
		if r[i].copy != nil {
			g.shared.copies[r[i].cls] = r[i].copy
		} else {
			delete(g.shared.copies, r[i].cls)
		}
	}
	g.shared.renamed = r[:n]
}

// copyObject generates the copy of the class of object expression cd, as
// nameObjects named it, with the type arguments of the copy being
// generated, and reports whether it still operates on reified type
// parameters.
func (g *gen) copyObject(cd *syntax.ClassDecl) bool {
	saved := map[*types.VarSym]int{}
	for _, v := range cd.Captures {
		saved[v] = g.slot(v)
	}
	defer g.restoreSlots(saved)
	used := false
	cg := &gen{file: g.file, shared: g.shared, lambdaReified: g.reifiedSubst(), reifiedUse: &used}
	out := cg.classes(cd, nil)
	c := g.shared.classes[cd.Sym.Name]
	h := kmeta.WriteClass(cg.classMeta(cd))
	delete(g.shared.classes, c.Name) // added again, with what the class is in
	g.localClass(c, g.cls, g.code.Name(), g.code.Desc(), cd.P.Line, h)
	c.InnerClasses[len(c.InnerClasses)-1].Access |= cf.AccPublic
	g.cls.InnerClasses[len(g.cls.InnerClasses)-1].Access |= cf.AccPublic
	for _, o := range out[1:] {
		g.shared.later = append(g.shared.later, o.Name)
	}
	return used
}

// storeCaptures stores the captures, in the constructor's parameters, in
// their fields.
func (g *gen) storeCaptures(cd *syntax.ClassDecl) {
	for _, v := range cd.Captures {
		g.code.Local(cf.ALOAD, 0)
		g.code.Local(storageLoadOp(v), g.slot(v))
		g.code.FieldInsn(cf.PUTFIELD, cd.Sym.Name, captureField(cd, v), storageDesc(v))
	}
}

// loadCaptures copies the captures of class cd, of the instance in local
// self, to locals, which the captured variables' slots become: those of
// the class whose method is being generated, or of an inlined member's
// receiver.
func (g *gen) loadCaptures(cd *syntax.ClassDecl, self int) {
	for _, v := range cd.Captures {
		d := storageDesc(v)
		slot := g.code.AllocLocal(d)
		g.code.Local(cf.ALOAD, self)
		g.code.FieldInsn(cf.GETFIELD, cd.Sym.Name, captureField(cd, v), d)
		if v.Boxed() {
			g.code.Local(cf.ASTORE, slot)
		} else {
			g.code.Local(storeOp(v.Type), slot)
		}
		g.setSlot(v, slot)
	}
	if cd.Sym.Inner {
		g.outerCaptures(cd)
	}
}

// outerCaptures copies, in the code of inner class cd of a local class or
// an object expression's class, or of an inner class of one, that class's
// captures to locals, from its instance, as kotlinc reads them:
// this$0.$x, or this$0.this$0.$x. cd.OuterThis is in its slot.
func (g *gen) outerCaptures(cd *syntax.ClassDecl) {
	o, od := outerLocal(cd)
	if od == nil {
		return
	}
	for _, v := range od.Captures {
		d := storageDesc(v)
		g.code.Local(cf.ALOAD, g.slot(cd.OuterThis))
		for k := cd.Sym.Outer; k != o; k = k.Outer {
			g.code.FieldInsn(cf.GETFIELD, k.Name, "this$0", desc(k.Outer.This()))
		}
		g.code.FieldInsn(cf.GETFIELD, o.Name, captureField(od, v), d)
		slot := g.code.AllocLocal(d)
		if v.Boxed() {
			g.code.Local(cf.ASTORE, slot)
		} else {
			g.code.Local(storeOp(v.Type), slot)
		}
		g.setSlot(v, slot)
	}
}

// outerLocal returns the local class or object expression's class that
// inner class cd is in, through other inner classes, and its declaration,
// or nils.
func outerLocal(cd *syntax.ClassDecl) (*types.ClassSym, *syntax.ClassDecl) {
	if !cd.Sym.Inner {
		return nil, nil
	}
	o := cd.Sym.Outer
	for o != nil && o.Inner {
		o = o.Outer
	}
	if o == nil || !o.Anonymous && !o.Local {
		return nil, nil
	}
	return o, o.Decl.(*syntax.ClassDecl)
}

func storageLoadOp(v *types.VarSym) byte {
	if v.Boxed() {
		return cf.ALOAD
	}
	return loadOp(v.Type)
}
