package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports kotlinc's warnings about compare-and-set calls of
// atomic references and arrays of references, which compare values by
// identity, on values that have no consistent identity: primitives, which
// are boxed to be stored, and value classes. 127 is one box and 128 is a
// new one each time, so AtomicReference<Int>(128).compareAndSet(128, 1)
// fails. ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY is on the call, where the
// receiver's type argument is such a type, and
// ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY on each expected
// or new value of such a type.

// atomicRefKind describes a class of atomic references whose
// compare-and-set calls are judged.
type atomicRefKind struct {
	name    string          // its simple name, for the message
	members map[string]bool // the calls that compare by identity
	first   int             // the index of the first value parameter: past an array's index
	suggest map[types.Kind]string
}

var (
	javaAtomicRefMembers = map[string]bool{
		"compareAndSet": true, "weakCompareAndSet": true,
		"weakCompareAndSetAcquire": true, "weakCompareAndSetRelease": true,
		"weakCompareAndSetPlain": true, "weakCompareAndSetVolatile": true,
		"compareAndExchange": true, "compareAndExchangeAcquire": true, "compareAndExchangeRelease": true,
	}
	// atomicRefKinds are by internal name and class id: Kotlin's atomic
	// types are the Java classes on the JVM (see atomicJVM), with
	// KotlinName set.
	atomicRefKinds = map[[2]string]*atomicRefKind{
		{"java/util/concurrent/atomic/AtomicReference", ""}: {
			name: "AtomicReference", members: javaAtomicRefMembers,
			suggest: map[types.Kind]string{types.Bool: "AtomicBoolean", types.Int: "AtomicInteger", types.Long: "AtomicLong"},
		},
		{"java/util/concurrent/atomic/AtomicReferenceArray", ""}: {
			name: "AtomicReferenceArray", members: javaAtomicRefMembers, first: 1,
			suggest: map[types.Kind]string{types.Int: "AtomicIntegerArray", types.Long: "AtomicLongArray"},
		},
		{"java/util/concurrent/atomic/AtomicReference", "kotlin/concurrent/atomics/AtomicReference"}: {
			name: "AtomicReference", members: map[string]bool{"compareAndSet": true, "compareAndExchange": true},
			suggest: map[types.Kind]string{types.Bool: "AtomicBoolean", types.Int: "AtomicInt", types.Long: "AtomicLong"},
		},
		{"java/util/concurrent/atomic/AtomicReferenceArray", "kotlin/concurrent/atomics/AtomicArray"}: {
			name: "AtomicArray", members: map[string]bool{"compareAndSetAt": true, "compareAndExchangeAt": true}, first: 1,
			suggest: map[types.Kind]string{types.Int: "AtomicIntArray", types.Long: "AtomicLongArray"},
		},
	}
)

// atomicWarnings reports the compare-and-set call x on an atomic
// reference, or array of references, of values without a consistent
// identity. As kotlinc, it judges the call by its explicit receiver's
// type, not the member's class: a subclass of AtomicReference<Int> isn't
// judged.
func (c *Checker) atomicWarnings(x *syntax.Call) {
	if x.Sym == nil || x.Invoke || x.SAM || x.Ctor != nil || x.Literal {
		return
	}
	m, ok := x.Fun.(*syntax.Member)
	if !ok || m.X == nil {
		return
	}
	t := m.X.Type().NonNull()
	if t.K != types.Class || t.C == nil {
		return
	}
	k := atomicRefKinds[[2]string{t.C.Name, t.C.KotlinName}]
	if k == nil || !k.members[x.Sym.Name] {
		return
	}
	if t.A != nil && len(t.A.List) > 0 {
		if arg := t.A.Get(0).T; noIdentity(arg) {
			msg := "This call may have inconsistent results because '" + k.name + "' uses identity equality and '" +
				arg.String() + "' does not have a consistent identity."
			if s, ok := k.suggest[arg.NonNull().K]; ok {
				msg += " Consider using '" + s + "' instead."
			}
			c.warnf(syntax.StartPos(x), "ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY", "%s", msg)
		}
	}
	for i, a := range x.Args {
		if p := callParamIndex(x, i); p < k.first || p >= len(x.Sym.Params) {
			continue
		}
		if at := a.Type(); noIdentity(at) {
			c.warnf(syntax.StartPos(a), "ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY",
				"This call may have inconsistent results because the type of this argument, '%s', does not have a consistent identity.", at)
		}
	}
}

// callParamIndex returns the index of the parameter of x.Sym that call x
// passes its argument i to, or -1.
func callParamIndex(x *syntax.Call, i int) int {
	switch {
	case x.ArgMap != nil:
		if i >= len(x.ArgMap) {
			return -1
		}
		return x.ArgMap[i]
	case i < len(x.Names) && x.Names[i] != "":
		for j, p := range x.Sym.Params {
			if p.Name == x.Names[i] {
				return j
			}
		}
		return -1
	}
	return i
}

// noIdentity reports whether values of type t have no consistent
// identity: whether t, nullable or not, is a primitive type or a value
// class.
func noIdentity(t types.Type) bool {
	t = t.NonNull()
	switch {
	case types.IsPrimitiveKind(t.K):
		return true
	case t.K == types.Class && t.C != nil:
		return ensured(t.C).Value
	}
	return false
}

// synchronizedLocks reports the calls of kotlin's synchronized whose lock
// has no consistent identity, as kotlinc does
// (SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE): synchronized(1) {}
// locks a new box each time. A nullable primitive, Int?, is a box and
// isn't reported, but a nullable value class, V?, is.
func (c *Checker) synchronizedLocks() {
	walkCode(c.file.Decls, func(n any) bool {
		x, ok := n.(*syntax.Call)
		if !ok || x.Sym == nil || x.Invoke || x.Sym.Name != "synchronized" || !strings.HasPrefix(x.Sym.Facade, "kotlin/StandardKt") {
			return true
		}
		for i, a := range x.Args {
			p := i
			if i < len(x.ArgMap) {
				p = x.ArgMap[i]
			}
			if p != 0 || a == nil {
				continue
			}
			t := a.Type()
			if t.N && types.IsPrimitiveKind(t.K) || !noIdentity(t) {
				continue
			}
			c.errorf(syntax.StartPos(a), "SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE_ERROR", "synchronizing on '%s' is forbidden", t)
		}
		return true
	})
}
