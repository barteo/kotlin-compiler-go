package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Null checks, as kotlinc writes them: a non-private function checks that
// its parameters of non-null reference types aren't null, and a value of
// a platform type (from Java) is checked where it is used as a non-null
// type. With the standard library they call kotlin.jvm.internal.Intrinsics;
// without it, java.util.Objects.requireNonNull with the same message.

const intrinsics = "kotlin/jvm/internal/Intrinsics"

// checked reports whether a value of type t is null-checked when a
// parameter or a non-null platform value has it: a non-null reference.
func checked(t types.Type) bool {
	if types.Unboxed(t) {
		// An unboxed value is its underlying value, which may be null:
		// ChannelResult's Any?.
		t = types.Repr(t)
	}
	if types.MayBeNull(t) || t.P {
		return false
	}
	switch t.K {
	case types.Unit, types.Nothing, types.Null, types.Invalid:
		return false
	}
	d := desc(t)
	return d[0] == 'L' || d[0] == '['
}

// checkParam checks that the parameter of type t in slot isn't null, at
// the start of the method being generated. name is the parameter's name
// in the exception's message: "<this>" for an extension receiver, and
// "<set-?>" for a default setter's.
func (g *gen) checkParam(slot int, t types.Type, name string) {
	if !checked(t) {
		return
	}
	g.code.Local(cf.ALOAD, slot)
	if g.shared.opts.Stdlib {
		g.code.Sconst(name)
		g.code.Invoke(cf.INVOKESTATIC, intrinsics, "checkNotNullParameter", "(Ljava/lang/Object;Ljava/lang/String;)V")
		return
	}
	// Intrinsics names the method from the stack trace.
	method := strings.ReplaceAll(g.code.Class(), "/", ".") + "." + g.code.Name()
	g.requireNonNull("Parameter specified as non-null is null: method " + method + ", parameter " + name)
}

// checkParams checks the receiver and parameters of a function that isn't
// private.
func (g *gen) checkParams(ctx []*types.VarSym, recv *types.VarSym, params []*types.VarSym) {
	for _, p := range ctx {
		g.checkParam(p.Slot, p.Type, p.Name)
	}
	if recv != nil {
		g.checkParam(recv.Slot, recv.Type, "<this>")
	}
	for _, p := range params {
		g.checkParam(p.Slot, p.Type, p.Name)
	}
}

// requireNonNull consumes the reference on the stack, throwing a
// NullPointerException with message msg if it is null ("" for none).
func (g *gen) requireNonNull(msg string) {
	if msg == "" {
		g.code.Invoke(cf.INVOKESTATIC, "java/util/Objects", "requireNonNull", "(Ljava/lang/Object;)Ljava/lang/Object;")
	} else {
		g.code.Sconst(msg)
		g.code.Invoke(cf.INVOKESTATIC, "java/util/Objects", "requireNonNull", "(Ljava/lang/Object;Ljava/lang/String;)Ljava/lang/Object;")
	}
	g.code.Op(cf.POP)
}

// checkPlatform checks the value of x on the stack, of a platform type,
// where it is used as type to: it must not be null if to is a non-null
// reference type.
func (g *gen) checkPlatform(x syntax.Expr, to types.Type) {
	from := x.Type()
	if !from.P || !checked(to) || !g.code.Reachable() {
		return
	}
	if c, ok := x.(*syntax.Call); ok && c.Sym != nil && c.Sym.Owner != nil && c.Sym.Owner.Name == "java/lang/Object" {
		return // Kotlin sees Object's members as Any's: toString() is a String
	}
	g.code.Op(cf.DUP)
	text, ok := platformText(x)
	switch {
	case !ok && g.shared.opts.Stdlib:
		g.code.Invoke(cf.INVOKESTATIC, intrinsics, "checkNotNull", "(Ljava/lang/Object;)V")
	case !ok:
		g.requireNonNull("")
	case g.shared.opts.Stdlib:
		g.code.Sconst(text)
		g.code.Invoke(cf.INVOKESTATIC, intrinsics, "checkNotNullExpressionValue", "(Ljava/lang/Object;Ljava/lang/String;)V")
	default:
		g.requireNonNull(text + " must not be null")
	}
}

// platformText returns how kotlinc names an expression of a platform type
// in the message of its null check: the method of a call or a getter,
// with "(...)", or a field. A variable has no text: its check has no
// message.
func platformText(x syntax.Expr) (string, bool) {
	switch x := x.(type) {
	case *syntax.Call:
		if x.Sym != nil {
			return x.Sym.JVM() + "(...)", true
		}
	case *syntax.Index:
		if x.Call != nil && x.Call.Sym != nil {
			return x.Call.Sym.JVM() + "(...)", true
		}
	case *syntax.Member:
		switch {
		case x.Getter != nil:
			return x.Getter.JVM() + "(...)", true
		case x.Field != nil:
			return x.Field.Name, true
		case x.Prop != nil:
			return x.Prop.Name, true
		}
	case *syntax.Ident:
		if x.Getter != nil {
			return x.Getter.JVM() + "(...)", true
		}
		if x.Member != nil {
			return platformText(x.Member)
		}
	}
	return "", false
}

// lateinitCheck checks the value of lateinit property or local name on
// the stack, which is null until it is assigned: then, as with kotlinc,
// Intrinsics throws an UninitializedPropertyAccessException, or without
// the standard library an IllegalStateException with its message.
func lateinitCheck(code *cf.Code, name string, stdlib bool) {
	done := cf.NewLabel()
	code.Op(cf.DUP)
	code.Jump(cf.IFNONNULL, done)
	code.Op(cf.POP)
	if stdlib {
		code.Sconst(name)
		code.Invoke(cf.INVOKESTATIC, intrinsics, "throwUninitializedPropertyAccessException", "(Ljava/lang/String;)V")
		code.Op(cf.ACONST_NULL)
	} else {
		code.New("java/lang/IllegalStateException")
		code.Op(cf.DUP)
		code.Sconst("lateinit property " + name + " has not been initialized")
		code.Invoke(cf.INVOKESPECIAL, "java/lang/IllegalStateException", "<init>", "(Ljava/lang/String;)V")
		code.Op(cf.ATHROW)
	}
	code.Place(done)
}

// lateinit checks a lateinit property or local read on the stack.
func (g *gen) lateinit(p *types.VarSym) {
	if p.Lateinit {
		lateinitCheck(g.code, p.Name, g.shared.opts.Stdlib)
	}
}
