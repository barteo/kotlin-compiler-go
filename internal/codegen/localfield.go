package codegen

import (
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The backing field of a property, field, in a local class or an object
// expression of its accessor, which the checker gives the accessor's
// this (VarSym.OuterOf): the field is private to the property's class,
// so other classes go through its static accessors access$getX$p and
// access$setX$p, as kotlinc's do.

// outerField reads (or with store, writes) class property p's field of
// the object on the stack, from code that may be in another class.
func (g *gen) outerField(p *types.VarSym, store bool) {
	if g.cls.Name == p.Owner.Name {
		if store {
			g.putField(p)
		} else {
			g.getField(p)
		}
		return
	}
	ft := types.FieldType(p)
	d := desc(ft)
	self := "L" + p.Owner.Name + ";"
	if store {
		g.shared.access[p] |= 2
		g.toField(p, ft)
		g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, privateSetter(p), "("+self+d+")V")
		return
	}
	g.shared.access[p] |= 1
	g.code.Invoke(cf.INVOKESTATIC, p.Owner.Name, privateGetter(p), "("+self+")"+d)
	g.coerce(ft, p.Type)
}
