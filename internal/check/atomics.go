package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The atomic types of kotlin.concurrent.atomics, such as AtomicInt and
// AtomicReference<T>, have no class files: like the collection types,
// they are declared in .kotlin_builtins, and on the JVM they are the
// classes of java.util.concurrent.atomic, whose methods have other
// names: AtomicInt.load() is AtomicInteger.get(). Kotlin doesn't see a
// Java AtomicInteger as an AtomicInt, though: asKotlinAtomic() converts
// it, so each is a class of its own, as a Java collection interface and
// the Kotlin type it is are.
var atomicJVM = map[string]string{
	"kotlin/concurrent/atomics/AtomicInt":       "java/util/concurrent/atomic/AtomicInteger",
	"kotlin/concurrent/atomics/AtomicLong":      "java/util/concurrent/atomic/AtomicLong",
	"kotlin/concurrent/atomics/AtomicBoolean":   "java/util/concurrent/atomic/AtomicBoolean",
	"kotlin/concurrent/atomics/AtomicReference": "java/util/concurrent/atomic/AtomicReference",
	"kotlin/concurrent/atomics/AtomicIntArray":  "java/util/concurrent/atomic/AtomicIntegerArray",
	"kotlin/concurrent/atomics/AtomicLongArray": "java/util/concurrent/atomic/AtomicLongArray",
	"kotlin/concurrent/atomics/AtomicArray":     "java/util/concurrent/atomic/AtomicReferenceArray",
}

// atomicJVMNames maps the atomic types' members to their Java methods'
// names.
var atomicJVMNames = map[string]string{
	"load": "get", "store": "set", "exchange": "getAndSet",
	"fetchAndAdd": "getAndAdd", "addAndFetch": "addAndGet",
	"loadAt": "get", "storeAt": "set", "exchangeAt": "getAndSet",
	"compareAndSetAt": "compareAndSet", "compareAndExchangeAt": "compareAndExchange",
	"fetchAndAddAt": "getAndAdd", "addAndFetchAt": "addAndGet",
	"size": "length",
}

// atomic returns the class of a built-in atomic type, by class id.
func (t *classTable) atomic(id string) *types.ClassSym {
	t = t.root()
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.atomics[id]
	if c == nil {
		if t.atomics == nil {
			t.atomics = map[string]*types.ClassSym{}
		}
		c = types.NewExternal(atomicJVM[id], t.loadAtomic)
		c.KotlinName = id
		t.atomics[id] = c
	}
	return c
}

// loadAtomic fills in an atomic type from .kotlin_builtins: a final class
// whose constructors and members are the Java class's under their Kotlin
// names.
func (t *classTable) loadAtomic(cls *types.ClassSym) {
	t = t.root()
	kc := t.builtinClasses()[cls.KotlinName]
	if kc == nil {
		cls.Missing = true
		return
	}
	cls.Final = true
	cls.Super = t.class("java/lang/Object")
	scope := tscope{}
	cls.TypeParams, _ = t.ktypeParams(kc.TypeParams, cls, scope)
	cls.Methods = map[string][]*types.FunSym{}
	cls.Statics = map[string][]*types.FunSym{}
	cls.Fields = map[string]*types.VarSym{}
	cls.KProps = map[string]*types.KProp{}
	sig := func(name string, params []*kmeta.Param, ret *kmeta.Type) kmeta.JVMSig {
		s := builtinSig(name, params, ret)
		s.Name = name
		if n, ok := atomicJVMNames[name]; ok {
			s.Name = n
		}
		return s
	}
	for _, f := range kc.Funcs {
		f.JVM = sig(f.Name, f.Params, f.Ret)
		sym := t.kfun(f, cls, scope, nil)
		sym.Final = true
		cls.Methods[f.Name] = append(cls.Methods[f.Name], sym)
	}
	for _, p := range kc.Props {
		get := sig(p.Name, nil, p.Type)
		if kp := t.kprop(p, cls, scope, nil, func(*kmeta.Property) *kmeta.JVMSig { return &get }, metaSetter); kp != nil {
			cls.KProps[p.Name] = kp
		}
	}
	for _, k := range kc.Ctors {
		s := sig("<init>", k.Params, &kmeta.Type{Class: "kotlin/Unit"})
		f := &types.FunSym{Name: "<init>", Owner: cls, Desc: s.Desc[:strings.LastIndexByte(s.Desc, ')')+1] + "V",
			Ret: types.ClassType(cls), Resolved: true}
		var ok bool
		f.Params, ok = t.kparams(k.Params, scope, true)
		f.Unsupported = !ok
		cls.Ctors = append(cls.Ctors, f)
	}
}

// atomicMismatch reports whether Java method jm, of the JVM signature of
// Kotlin member m, takes or returns a Java atomic class where m has
// its Kotlin atomic type: foo(AtomicInteger) for foo(AtomicInt). Both are
// AtomicInteger on the JVM, but Kotlin sees two members.
func atomicMismatch(jm, m *jvmMember) bool {
	var ts []types.Type
	switch {
	case m.fun != nil:
		for _, p := range m.fun.Params {
			ts = append(ts, p.Type)
		}
		ts = append(ts, m.fun.Ret)
	case m.prop != nil && m.setter:
		ts = append(ts, m.prop.Type, types.UnitType)
	case m.prop != nil:
		ts = append(ts, m.prop.Type)
	default:
		return false
	}
	js := make([]types.Type, 0, len(jm.fun.Params)+1)
	for _, p := range jm.fun.Params {
		js = append(js, p.Type)
	}
	js = append(js, jm.fun.Ret)
	if len(js) != len(ts) {
		return false
	}
	for i, t := range ts {
		a, b := t.C, js[i].C
		if t.K == types.Class && js[i].K == types.Class && a != b && (isAtomic(a) || isAtomic(b)) {
			return true
		}
	}
	return false
}

func isAtomic(c *types.ClassSym) bool {
	return c != nil && strings.HasPrefix(c.KotlinName, "kotlin/concurrent/atomics/")
}
