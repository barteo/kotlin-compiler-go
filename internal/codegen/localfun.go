package codegen

import (
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Local functions.
//
// As with kotlinc, a local function g declared in main is a private static
// method of the class being generated, main$g, written where the
// declaration is. A second g in main is main$g$0, and a function or lambda
// declared in g is named after it: main$g$h, main$g$lambda$0. Like a
// lambda's method, it takes the enclosing object if it uses this, then
// what it captures, then its parameters; a call passes them along. A
// local function with default values gets a static main$g$default method
// with the same first parameters.

// localOf returns the local function that f is, or nil.
func localOf(f *types.FunSym) *syntax.LocalFun {
	if f == nil {
		return nil
	}
	lf, _ := f.Decl.(*syntax.LocalFun)
	return lf
}

// localDesc returns the descriptor of a local function's method. As
// kotlinc's, its context parameters and an extension's receiver come
// first.
func (g *gen) localDesc(lf *syntax.LocalFun) string {
	if lf.Desc != "" {
		return lf.Desc // from code in another class, an object expression's
	}
	f := lf.Decl.Sym
	return "(" + leadDesc(f) + g.frameDesc(lf.Frame) + sigDesc(f)[1:]
}

// localFun generates the method of a local function.
func (g *gen) localFun(lf *syntax.LocalFun) {
	base := strings.TrimSuffix(g.prefix, "$lambda") + "$" + lf.Decl.Name
	key := g.cls.Name + ".fun." + base
	name := base
	if n := g.shared.count[key]; n > 0 {
		name += "$" + strconv.Itoa(n-1)
	}
	g.shared.count[key]++
	lf.Method, lf.Owner = name, g.cls.Name // before the body, which may call it

	fd := lf.Decl
	f := fd.Sym
	access := uint16(cf.AccPrivate | cf.AccStatic | cf.AccFinal)
	if g.inInterface() {
		access &^= cf.AccFinal // interface methods can't be final
	}
	if varargs(f) {
		access |= cf.AccVarargs
	}
	lc := &lambdaCtx{slots: map[*types.VarSym]int{}, fun: true}
	lg := &gen{file: g.file, cls: g.cls, owner: g.owner, shared: g.shared, prefix: name + "$lambda", lam: lc, noLines: g.noLines,
		publicInline: g.publicInline, lambdaReified: g.reifiedSubst()} // copies of an inline function copy its lambdas
	d := g.localDesc(lf)
	lf.Desc, lf.This = d, g.thisClass()
	lg.code = g.cls.NewMethod(access, name, d)
	lg.retType = f.Ret
	for _, p := range f.Contexts {
		g.setSlot(p, lg.code.AllocLocal(paramDesc(p.Type)))
		lg.named(p)
	}
	if r := fd.RecvSym; r != nil {
		g.setSlot(r, lg.code.AllocLocal(desc(r.Type)))
		lg.named(r)
	}
	lg.allocFrame(lf.Frame, lc)
	for _, p := range f.Params {
		g.setSlot(p, lg.code.AllocLocal(paramDesc(p.Type)))
		lg.named(p)
	}
	if f.Suspend {
		k := lg.code.AllocLocal(types.ContinuationDesc)
		lg.code.NameLocal(k, "$completion")
		owner, iface := g.cls.Name, g.inInterface()
		lg.suspendFun(fd, name, d, k, &resumeCall{call: func(g *gen) {
			if iface {
				g.code.InvokeStaticInterface(owner, name, d)
				return
			}
			g.shared.methods[privateMethod{class: owner, name: name, desc: d, static: true}] = true
			g.code.Invoke(cf.INVOKESTATIC, owner, "access$"+name, d)
		}})
	}
	if fd.Tailrec {
		// After the state machine's entry, as in a member function; the
		// captures in the frame stay the same across the jumps.
		lg.tail = &tailrec{fd: fd, start: cf.NewLabel()}
		lg.code.Place(lg.tail.start)
	}
	if fd.Body != nil {
		lg.block(fd.Body, types.UnitType, false)
		if lg.code.Reachable() {
			lg.line(fd.Body.End)
			lg.ret(types.UnitType)
		}
	} else {
		lg.line(syntax.StartPos(fd.ExprBody))
		lg.exprAs(fd.ExprBody, f.Ret)
		lg.ret(f.Ret)
	}
	lg.endSuspend()
	lg.check()
	if hasDefaults(fd.Params) {
		g.localDefaults(lf)
	}
}

// localDefaults generates a local function's name$default method, which
// computes the omitted arguments' default values, then calls it.
func (g *gen) localDefaults(lf *syntax.LocalFun) {
	f := lf.Decl.Sym
	d := g.localDesc(lf)
	access := uint16(cf.AccStatic | cf.AccSynthetic)
	if g.inInterface() {
		access |= cf.AccPrivate // interface methods are public or private
	}
	lc := &lambdaCtx{slots: map[*types.VarSym]int{}, fun: true}
	lg := &gen{file: g.file, cls: g.cls, owner: g.owner, shared: g.shared, prefix: lf.Method + "$default$lambda", lam: lc, noLines: g.noLines,
		publicInline: g.publicInline, lambdaReified: g.reifiedSubst()} // copies of an inline function copy its lambdas
	lg.code = g.cls.NewMethod(access, lf.Method+"$default", defaultsDesc(d, types.DefaultBits(f), objMarker))
	lg.retType = f.Ret
	for _, p := range f.Contexts {
		g.setSlot(p, lg.code.AllocLocal(paramDesc(p.Type)))
	}
	if r := lf.Decl.RecvSym; r != nil {
		g.setSlot(r, lg.code.AllocLocal(desc(r.Type)))
	}
	lg.allocFrame(lf.Frame, lc)
	k := lg.fillDefaultsCont(lf.Decl.Params, f.Params, len(f.Contexts), f.Suspend, nil)
	for _, p := range f.Contexts {
		lg.code.Local(loadOp(p.Type), g.slot(p))
	}
	if r := lf.Decl.RecvSym; r != nil {
		lg.code.Local(loadOp(r.Type), g.slot(r))
	}
	lg.loadFrame(lf.Frame)
	for _, p := range f.Params {
		lg.code.Local(loadOp(p.Type), g.slot(p))
	}
	if k >= 0 {
		lg.code.Local(cf.ALOAD, k) // a suspend function's continuation
	}
	lg.invokeLocal(lf.Method, d)
	lg.code.Op(returnOpOf(cf.RetDesc(d))) // an Object for a suspend function
	lg.check()
}

// callLocal generates a call of a local function.
func (g *gen) callLocal(x *syntax.Call, lf *syntax.LocalFun) {
	f := x.Sym
	g.contextArgs(x, f)
	if m, ok := x.Fun.(*syntax.Member); ok && f.Ext {
		g.exprAs(m.X, f.Recv) // the receiver, x in x.g()
	}
	g.loadLocalFrame(lf)
	name, d := lf.Method, g.localDesc(lf)
	if mask := g.callArgs(x, f); mask.any() {
		g.pushMask(mask, types.DefaultBits(f))
		name, d = name+"$default", defaultsDesc(d, types.DefaultBits(f), objMarker)
	}
	if lf.Owner != g.cls.Name && !g.inInterface() {
		// From another class, a function reference's: through an
		// accessor, as the method is private.
		g.shared.methods[privateMethod{class: lf.Owner, name: name, desc: d, static: true}] = true
		g.code.Invoke(cf.INVOKESTATIC, lf.Owner, "access$"+name, d)
	} else {
		g.invokeLocal(name, d)
	}
	g.coerce(f.Ret, x.T)
}

// invokeLocal calls a static method of the class being generated.
func (g *gen) invokeLocal(name, d string) {
	if g.inInterface() {
		g.code.InvokeStaticInterface(g.cls.Name, name, d)
	} else {
		g.code.Invoke(cf.INVOKESTATIC, g.cls.Name, name, d)
	}
}

// loadLocalFrame pushes what local function lf's frame captures. In an
// object expression's code, its this is the object's capture of the
// enclosing class's this.
func (g *gen) loadLocalFrame(lf *syntax.LocalFun) {
	if lf.Frame.This && lf.This != nil && lf.This != g.thisClass() && g.owner != nil {
		if cd, ok := g.owner.Decl.(*syntax.ClassDecl); ok && cd.OuterThis != nil && cd.OuterThis.Type.NonNull() == lf.This.This() {
			g.loadStorage(cd.OuterThis)
			for _, v := range lf.Frame.Captures {
				g.loadStorage(v)
			}
			return
		}
	}
	g.loadFrame(lf.Frame)
}
