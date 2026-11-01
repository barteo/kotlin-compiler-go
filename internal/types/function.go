package types

import (
	"strconv"
	"strings"
	"sync"
)

// MaxArity is the largest number of parameters of a function type with an
// interface of its own in kotlin.jvm.functions. Those with more are all
// kotlin.jvm.functions.FunctionN, whose invoke takes the arguments in an
// array (VarArityDesc).
const MaxArity = 22

// MaxVarArity is the largest number of parameters a function type can
// have, as with kotlinc: a method takes at most 255 slots, one of them
// the object's.
const MaxVarArity = 254

// VarArityDesc is the descriptor of FunctionN's invoke.
const VarArityDesc = "([Ljava/lang/Object;)Ljava/lang/Object;"

// FunctionPackage holds the interfaces that function types compile to
// without the Kotlin standard library. kc writes them next to the
// program, so it needs no Kotlin runtime.
const FunctionPackage = "kc/jvm/functions/"

// KotlinFunctionPackage holds the standard library's function interfaces,
// which function types compile to when it is on the class path.
const KotlinFunctionPackage = "kotlin/jvm/functions/"

// Functions are the interfaces of function types. A function type
// (P1, ..., Pn) -> R is the class type FunctionN<P1, ..., Pn, R> of an
// interface declared as FunctionN<in P1, ..., in Pn, out R>, as in Kotlin.
// Subtyping, inference and substitution treat it like any other generic
// class.
type Functions struct {
	classes [MaxArity + 1]*ClassSym
	// suspend are the interfaces of suspend function types: suspend
	// (P1, ..., Pn) -> R is SuspendFunctionN<P1, ..., Pn, R>, which is
	// FunctionN+1 on the JVM, taking a Continuation last.
	suspend [MaxArity]*ClassSym
	// composable are the interfaces of composable function types, which
	// Compose's compiler plugin compiles: @Composable (P1, ..., Pn) -> R
	// is FunctionN+1+k on the JVM, taking a Composer and k Integers of
	// change flags last (ComposeDesc).
	composable [MaxComposableArity + 1]*ClassSym

	pkg        string
	mu         sync.Mutex
	big        map[int]*ClassSym // those with more than MaxArity parameters
	bigSuspend map[int]*ClassSym // suspend ones with MaxArity or more
}

// MaxComposableArity is the largest number of parameters, with the
// receiver, of a composable function type: its interface has two more.
const MaxComposableArity = 19

// KcFunctions and KotlinFunctions are the function interfaces without and
// with the standard library.
var (
	KcFunctions     = newFunctions(FunctionPackage)
	KotlinFunctions = newFunctions(KotlinFunctionPackage)
)

func newFunctions(pkg string) *Functions {
	fs := &Functions{pkg: pkg, big: map[int]*ClassSym{}, bigSuspend: map[int]*ClassSym{}}
	for n := range fs.classes {
		fs.classes[n] = newFunctionClass(pkg, n, false, false)
	}
	for n := range fs.suspend {
		fs.suspend[n] = newFunctionClass(pkg, n, true, false)
	}
	for n := range fs.composable {
		fs.composable[n] = newFunctionClass(pkg, n, false, true)
	}
	return fs
}

func newFunctionClass(pkg string, n int, suspend, composable bool) *ClassSym {
	jvm := n
	if suspend {
		jvm++
	}
	changed := 0
	if composable {
		// The lambda's object is a slot of the change flags too.
		changed = ChangedInts(n + 1)
		jvm += 1 + changed
	}
	name := pkg + "Function" + strconv.Itoa(jvm)
	if jvm > MaxArity {
		name = pkg + "FunctionN"
	}
	c := &ClassSym{Name: name, Func: true, Suspend: suspend, Composable: composable, Interface: true, Methods: map[string][]*FunSym{}}
	invoke := &FunSym{Name: "invoke", Owner: c, Resolved: true, Suspend: suspend, Composable: composable, Changed: changed, Abstract: true, Operator: true}
	if suspend || composable {
		// The JVM method of the larger FunctionN, which takes the
		// continuation, or the Composer and the change flags, as Objects
		// after the parameters.
		invoke.Desc = "(" + strings.Repeat("Ljava/lang/Object;", jvm) + ")Ljava/lang/Object;"
	}
	if jvm > MaxArity {
		invoke.Desc = VarArityDesc
	}
	for i := range n + 1 {
		p := &TypeParamSym{Name: "P" + strconv.Itoa(i+1), Var: In, Index: i, Owner: c}
		if i == n {
			p.Name, p.Var = "R", Out
			invoke.Ret = ParamType(p)
		} else {
			invoke.Params = append(invoke.Params, &VarSym{Name: strings.ToLower(p.Name), Type: ParamType(p), Kind: Param})
		}
		c.TypeParams = append(c.TypeParams, p)
	}
	c.Methods["invoke"] = []*FunSym{invoke}
	return c
}

// Class returns the interface for function types with n parameters. Those
// with more than MaxArity share the JVM interface FunctionN, but each n
// has a class of its own, as kotlin.Function23 is in Kotlin.
func (fs *Functions) Class(n int) *ClassSym {
	if n <= MaxArity {
		return fs.classes[n]
	}
	return fs.bigClass(fs.big, n, false)
}

// bigClass returns the class in m for function types with n parameters,
// whose JVM interface is FunctionN, creating it if it is the first.
func (fs *Functions) bigClass(m map[int]*ClassSym, n int, suspend bool) *ClassSym {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	c := m[n]
	if c == nil {
		c = newFunctionClass(fs.pkg, n, suspend, false)
		m[n] = c
	}
	return c
}

// VarArity reports whether c is the interface of function types whose
// JVM interface has more than MaxArity parameters, FunctionN, with the
// continuation of a suspend one.
func (c *ClassSym) VarArity() bool {
	n := len(c.TypeParams) - 1
	if c.Suspend {
		n++
	}
	return c.Func && !c.Composable && n > MaxArity
}

// KFunctionN reports whether c is kotlin.reflect.KFunctionN, the type of
// a reference to a function with N parameters, which is a KFunction<R>
// on the JVM.
func (c *ClassSym) KFunctionN() bool {
	return c.Name == "kotlin/reflect/KFunction" && c.KotlinName != "kotlin/reflect/KFunction"
}

// SuspendClass returns the interface for suspend function types with n
// parameters.
func (fs *Functions) SuspendClass(n int) *ClassSym {
	if n < MaxArity {
		return fs.suspend[n]
	}
	return fs.bigClass(fs.bigSuspend, n, true)
}

// ComposableClass returns the interface for composable function types
// with n parameters, or nil if there are more than MaxComposableArity.
func (fs *Functions) ComposableClass(n int) *ClassSym {
	if n > MaxComposableArity {
		return nil
	}
	return fs.composable[n]
}

// Type returns the type (params...) -> ret.
func (fs *Functions) Type(params []Type, ret Type) Type {
	return funcType(fs.Class(len(params)), params, ret)
}

// SuspendType returns the type suspend (params...) -> ret.
func (fs *Functions) SuspendType(params []Type, ret Type) Type {
	return funcType(fs.SuspendClass(len(params)), params, ret)
}

// ComposableType returns the type @Composable (params...) -> ret.
func (fs *Functions) ComposableType(params []Type, ret Type) Type {
	return funcType(fs.ComposableClass(len(params)), params, ret)
}

func funcType(c *ClassSym, params []Type, ret Type) Type {
	args := make([]Arg, len(params)+1)
	for i, p := range params {
		args[i] = Arg{T: p}
	}
	args[len(params)] = Arg{T: ret}
	return GenericType(c, args)
}

// IsFunc reports whether t is a function type, possibly nullable: a
// suspend one too.
func (t Type) IsFunc() bool { return t.K == Class && t.C.Func && t.A != nil }

// IsSuspendFunc reports whether t is a suspend function type.
func (t Type) IsSuspendFunc() bool { return t.IsFunc() && t.C.Suspend }

// IsComposableFunc reports whether t is a composable function type.
func (t Type) IsComposableFunc() bool { return t.IsFunc() && t.C.Composable }

// IsExtFunc reports whether t is a function type with a receiver.
func (t Type) IsExtFunc() bool { return t.IsFunc() && t.E }

// WithReceiver returns the function type t marked as having a receiver:
// its first parameter.
func (t Type) WithReceiver() Type {
	t.E = true
	return t
}

// WithContexts returns the function type t marked as having n context
// parameters: its first parameters, before a receiver.
func (t Type) WithContexts(n int) Type {
	t.X = uint8(n)
	return t
}

// Contexts returns the number of context parameters of function type t.
func (t Type) Contexts() int {
	if !t.IsFunc() {
		return 0
	}
	return int(t.X)
}

// funcString formats a function type as (A, B) -> R, or reports false if
// its arguments are projected against their declared variance.
func funcString(t Type) (string, bool) {
	n := t.A.Len() - 1
	ps := make([]string, n)
	for i, a := range t.A.List {
		if a.V != Inv && a.V != t.C.TypeParams[i].Var {
			return "", false
		}
		if i < n {
			ps[i] = a.T.String()
		}
	}
	ctx := ""
	if x := int(t.X); x > 0 && x <= len(ps) {
		ctx = "context(" + strings.Join(ps[:x], ", ") + ") "
		ps = ps[x:]
	}
	recv := ""
	if t.E && len(ps) > 0 {
		recv = ps[0] + "."
		if a := t.A.Get(int(t.X)).T; a.N || a.IsFunc() {
			recv = "(" + ps[0] + ")."
		}
		ps = ps[1:]
	}
	s := ctx + recv + "(" + strings.Join(ps, ", ") + ") -> " + t.A.Get(n).T.String()
	if t.C.Suspend {
		s = "suspend " + s
	}
	if t.C.Composable {
		s = "@Composable " + s
	}
	if t.N {
		s = "(" + s + ")"
	}
	return s, true
}
