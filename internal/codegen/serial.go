package codegen

import (
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// serialCtor generates the synthetic constructor that the kotlinx.serialization
// plugin gives class cd, sc, as kotlinc writes it. It takes a bit for each
// property that was read, in seen0, the properties and a marker. It checks
// that the properties without default values were read, those of the
// superclasses too, then calls the superclass's synthetic constructor, if
// it is @Serializable too, or else its constructor without arguments. It
// then sets the fields in the order the primary constructor does: a
// property that was read to its parameter, others to their default values
// or initializers, and runs the init blocks. Those read the primary
// constructor's parameters from their fields.
func (g *gen) serialCtor(cd *syntax.ClassDecl, sc *syntax.SecondaryCtor) {
	cls, serial, f := cd.Sym, sc.Serial, sc.Sym
	access := uint16(cf.AccPublic | cf.AccSynthetic)
	if hasValueParam(f) {
		// As other constructors that take a value class: callers call
		// the one that takes a marker.
		access = cf.AccPrivate
		defer g.markerConstructor(f)
	}
	g.code = g.cls.NewMethod(access, "<init>", ctorJVMDesc(f))
	g.prefix = "_init_$lambda"
	g.retType = types.UnitType
	g.loops, g.tries = nil, nil
	g.code.AllocLocal("L" + cls.Name + ";")
	for _, p := range f.Params {
		g.setSlot(p, g.code.AllocLocal(paramDesc(p.Type)))
		g.named(p)
	}
	g.line(cd.P)
	seen := g.slot(f.Params[0])
	props := f.Params[1 : len(f.Params)-1] // those of all, in order
	marker := g.slot(f.Params[len(f.Params)-1])
	all := serial.All()
	if s := cls.Nested["$serializer"]; s != nil {
		var mask int32
		for i, p := range all {
			if !p.Optional {
				mask |= 1 << i
			}
		}
		if mask != 0 {
			read := cf.NewLabel()
			g.code.Iconst(mask)
			g.code.Iconst(mask)
			g.code.Local(cf.ILOAD, seen)
			g.code.Op(cf.IAND)
			g.code.Jump(cf.IF_ICMPEQ, read)
			g.code.Local(cf.ILOAD, seen)
			g.code.Iconst(mask)
			g.code.FieldInsn(cf.GETSTATIC, s.Name, "INSTANCE", "L"+s.Name+";")
			g.code.Invoke(cf.INVOKEVIRTUAL, s.Name, "getDescriptor", "()Lkotlinx/serialization/descriptors/SerialDescriptor;")
			g.code.Invoke(cf.INVOKESTATIC, "kotlinx/serialization/internal/PluginExceptionsKt", "throwMissingFieldException",
				"(IILkotlinx/serialization/descriptors/SerialDescriptor;)V")
			g.code.Place(read)
		}
	}
	g.code.Local(cf.ALOAD, 0)
	if sd := serial.Super; sd != nil {
		sf := sd.Serial.Ctor.Sym
		g.code.Local(cf.ILOAD, seen)
		for _, p := range props[:len(sf.Params)-2] {
			g.code.Local(loadOp(p.Type), g.slot(p))
		}
		g.code.Local(cf.ALOAD, marker)
		d := ctorJVMDesc(sf)
		if hasValueParam(sf) {
			d = g.marker(d)
		}
		g.code.Invoke(cf.INVOKESPECIAL, sd.Sym.Name, "<init>", d)
	} else {
		super, d := "java/lang/Object", "()V"
		if s := cls.Super; s != nil {
			super = s.Name
			// A sealed class's constructor is private: it is called
			// through the constructor that takes a marker.
			for _, f := range append([]*types.FunSym{s.Ctor}, s.Ctors...) {
				if f != nil && len(f.Params) == 0 && g.viaMarker(f) {
					g.useMarker(f)
					d = g.marker(d)
					break
				}
			}
		}
		g.code.Invoke(cf.INVOKESPECIAL, super, "<init>", d)
	}

	g.fieldParams = map[*types.VarSym]*types.VarSym{}
	defer func() { g.fieldParams = nil }()
	for _, cp := range cd.CtorParams {
		g.fieldParams[cp.Sym] = cp.PropSym
	}
	// The element of each property that was read.
	elems := map[any]int{}
	for i, p := range serial.Props {
		j := len(all) - len(serial.Props) + i
		if p.Param != nil {
			elems[p.Param] = j
		} else {
			elems[p.Prop] = j
		}
	}
	set := func(decl any, p *types.VarSym, def syntax.Expr) {
		put := func() { g.code.FieldInsn(cf.PUTFIELD, cls.Name, p.Name, desc(p.Type)) }
		i, ok := elems[decl]
		if !ok {
			g.code.Local(cf.ALOAD, 0)
			g.exprAs(def, p.Type)
			put()
			return
		}
		param := props[i]
		load := func() {
			g.code.Local(loadOp(param.Type), g.slot(param))
			g.coerce(param.Type, p.Type) // a value class's box to its value
		}
		if !all[i].Optional {
			g.code.Local(cf.ALOAD, 0)
			load()
			put()
			return
		}
		read, done := cf.NewLabel(), cf.NewLabel()
		g.code.Local(cf.ILOAD, seen)
		g.code.Iconst(int32(1) << i)
		g.code.Op(cf.IAND)
		g.code.Jump(cf.IFNE, read)
		g.code.Local(cf.ALOAD, 0)
		g.exprAs(def, p.Type)
		put()
		g.code.Jump(cf.GOTO, done)
		g.code.Place(read)
		g.code.Local(cf.ALOAD, 0)
		load()
		put()
		g.code.Place(done)
	}
	for _, cp := range cd.CtorParams {
		set(cp, cp.PropSym, cp.Default)
	}
	for _, m := range cd.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Init == nil || m.Sym.Const || m.Sym.StaticField() {
				continue
			}
			g.line(m.P)
			g.prefix = m.Name + "$lambda"
			set(m, m.Sym, m.Init)
		case *syntax.InitBlock:
			g.prefix = "_init_$lambda"
			g.block(m.Body, types.UnitType, false)
		}
	}
	g.line(cd.P)
	g.code.Op(cf.RETURN)
	g.check()
}
