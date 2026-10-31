// Package inline analyzes the bodies of the Kotlin standard library's
// inline functions, read from its class files, so that code generation
// can copy them into their callers, as kotlinc's inliner does.
//
// A body is the bytecode of a method: a static one, or an instance
// method, whose receiver is its first parameter. The analysis decodes it and
// finds, for each function-typed parameter, how the body uses it. A
// parameter that is only null-checked and invoked is inlinable: a lambda
// passed for it needs no object, since its body can be emitted at each
// invoke. The analysis also finds what code generation leaves out (the
// InlineMarker calls kotlinc leaves in bodies, and the parameters' null
// checks) and the calls of private methods of the same class, which are
// inlined in turn.
package inline

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// Body is the analyzed code of an inline function.
type Body struct {
	*cf.Method
	Insns []cf.Insn
	// Params are the parameters of the method's descriptor, in order,
	// after the receiver of an instance method.
	Params []*Param
	// This is set on an instance method's body: Params[0] is its
	// receiver, which a copy takes on the stack before the arguments.
	This bool
	// Drop marks the instructions that are not copied: InlineMarker calls
	// and the constants they take, and the null checks of parameters.
	Drop []bool
	// Calls are the private methods of the same class that instructions
	// call, by instruction index. They are inlined in place of the call.
	Calls map[int]*Body
	// FrameParams gives, for each frame, the parameter that each value on
	// its stack is (-1: none).
	FrameParams map[int][]int
	// Index maps offsets to instruction indexes.
	Index map[int]int
	// Reified are the instructions whose class is a reified type
	// parameter, by index: a copy uses the call's type argument.
	Reified map[int]Reified
	// Templates are the classes that instructions create, a new or the
	// getstatic of an object's instance, whose copies use the call's
	// type arguments (see Template), by index; LoadTemplates reads them.
	Templates map[int]*Template
	// Copies are the other lambdas and objects of the function's class
	// that instructions create, by index, as a default lambda's INSTANCE
	// (Fixture$invoke$1): kotlinc's inliner copies them into a caller in
	// another module, so a copy whose code kotlinc inlines in turn, a
	// public inline function's, copies them too, as templates without
	// reified operations. LoadTemplates reads them.
	Copies map[int]*Template
	// Suspends are the InlineMarker.mark calls of a suspend function's
	// body that bracket a suspension point, by index: BeforeSuspend
	// before its call, AfterSuspend after it, where the call's result,
	// possibly COROUTINE_SUSPENDED, is on the stack.
	Suspends map[int]int
	// Fakes are the aconst_null instructions that follow an
	// InlineMarker.mark(FakeContinuation) call, by index: a copy loads
	// the caller's continuation in their place, as for the continuation
	// of a suspend call in the body, or for coroutineContext.
	Fakes map[int]bool
	reify map[int]string
	copy  map[int]string
}

// The markers of suspension points, of a placeholder for the
// continuation, and around the creation of a continuation that a suspend
// method's copy for inliners makes, which tells them its class.
const (
	BeforeSuspend          = 0
	AfterSuspend           = 1
	FakeContinuation       = 3
	BeforeFakeContinuation = 4
	AfterFakeContinuation  = 5
)

// Reified is an instruction that kotlinc marks, with a call of
// InlineMarker.reifiedOperationMarker, as an operation on a reified type
// parameter: anewarray (NewArray), checkcast (As), instanceof (Is) or the
// ldc of a class (JavaClass), whose class is the parameter's erasure.
type Reified struct {
	Kind     int
	Param    string // the type parameter's name
	Nullable bool   // the type is the parameter's nullable type, T?: typeOf<T?>()
}

// The kinds of reified operations that kc supports, as kotlinc numbers
// them: SafeAs marks the checkcast of an as?, Enum the iconst_0 of
// enumValues' empty array placeholder (whose anewarray is dropped) or the
// aconst_null of enumValueOf's Enum.valueOf(null, name), and TypeOf the
// aconst_null that stands for typeOf<T>()'s KType.
const (
	NewArray  = 0
	As        = 1
	SafeAs    = 2
	Is        = 3
	JavaClass = 4
	Enum      = 5
	TypeOf    = 6
)

// reifiedOps are the instructions that each kind marks; Enum's are either.
var reifiedOps = map[int]byte{NewArray: cf.ANEWARRAY, As: cf.CHECKCAST, SafeAs: cf.CHECKCAST, Is: cf.INSTANCEOF,
	JavaClass: cf.LDC, Enum: cf.ICONST_0}

// Param is a parameter of an inline function.
type Param struct {
	Slot  int    // the local it is in
	Desc  string // its JVM type
	Arity int    // N for a kotlin.jvm.functions.FunctionN, or -1
	// Inlinable is set on a function-typed parameter that the body only
	// invokes. Loads and Invokes are its uses, as instruction indexes:
	// Invokes are the calls of FunctionN.invoke on it.
	Inlinable bool
	Why       string // why a function-typed parameter isn't inlinable
	Loads     []int
	Invokes   []int
	// InTry is set for the invokes in the range of an exception handler
	// of any exception whose finally block can't be copied: see Finally.
	InTry []bool
}

// Finally is the finally block of a handler of any exception, which a
// jump out of its range runs: the instructions [Start, End), by index,
// between the handler's InlineMarker.finallyStart and finallyEnd calls.
// Frame is the offset of the handler, whose frame gives the types of the
// locals there.
type Finally struct {
	Start, End, Frame int
}

// Finallies returns the finally blocks that a jump from the instruction
// at offset off out of the body runs, innermost first. It reports false
// if a handler of any exception covers off whose finally block kc
// can't copy: one without kotlinc's markers, or with jumps.
func (b *Body) Finallies(off int) ([]Finally, bool) {
	var out []Finally
	seen := map[int]bool{}
	for _, h := range b.Handlers {
		if h.Catch != "" || off < h.Start || off >= h.End || seen[h.Handler] {
			continue
		}
		seen[h.Handler] = true
		f, ok := b.finallyAt(h.Handler)
		if !ok {
			return nil, false
		}
		out = append(out, f)
	}
	return out, true
}

// finallyAt finds the finally block of the handler at offset off: it
// stores the exception, runs the block between the markers, then loads
// and rethrows the exception. The block may branch within itself, as the
// loops of ReentrantReadWriteLock.write's do.
func (b *Body) finallyAt(off int) (Finally, bool) {
	i, ok := b.Index[off]
	if !ok || b.Insns[i].Op != cf.ASTORE {
		return Finally{}, false
	}
	exc := b.Insns[i].Local
	marker := func(k int, name string) bool {
		in := &b.Insns[k]
		return in.Op == cf.INVOKESTATIC && in.Owner == "kotlin/jvm/internal/InlineMarker" && in.Name == name
	}
	start := -1
	for k := i + 1; k < len(b.Insns); k++ {
		in := &b.Insns[k]
		switch {
		case marker(k, "finallyStart"):
			start = k + 1
		case marker(k, "finallyEnd"):
			if start < 0 || k+2 >= len(b.Insns) || b.Insns[k+1].Op != cf.ALOAD || b.Insns[k+1].Local != exc || b.Insns[k+2].Op != cf.ATHROW {
				return Finally{}, false
			}
			f := Finally{Start: start, End: k, Frame: off}
			return f, b.closed(i+1, f)
		case in.Op >= cf.IFEQ && in.Op <= cf.IF_ACMPNE, in.Op == cf.GOTO, in.Op == cf.IFNULL, in.Op == cf.IFNONNULL:
			if start < 0 {
				return Finally{}, false
			}
		case in.Op >= cf.JSR && in.Op <= cf.RETURN, in.Op == cf.ATHROW, in.Op == cf.GOTO_W, in.Op == cf.JSR_W:
			return Finally{}, false
		}
	}
	return Finally{}, false
}

// closed reports whether the finally block f only branches within
// itself, to targets with an empty stack, and nothing else jumps into it
// or into its handler's code before it, from index from: a copy of the
// block has labels of its own.
func (b *Body) closed(from int, f Finally) bool {
	inside := func(off int) bool {
		k, ok := b.Index[off]
		return ok && f.Start <= k && k < f.End
	}
	for k := from; k < f.End; k++ {
		in := &b.Insns[k]
		if fr := b.Frames[in.Off]; fr != nil && (k < f.Start || len(fr.Stack) > 0) {
			return false
		}
		if isBranch(in.Op) && !inside(in.Target) {
			return false
		}
	}
	for k := range b.Insns {
		if in := &b.Insns[k]; (k < f.Start || k >= f.End) && isBranch(in.Op) && inside(in.Target) {
			return false
		}
	}
	return true
}

// isBranch reports whether op is a conditional branch or a goto.
func isBranch(op byte) bool {
	return op >= cf.IFEQ && op <= cf.IF_ACMPNE || op == cf.GOTO || op == cf.IFNULL || op == cf.IFNONNULL
}

// Ret returns the descriptor of the method's result.
func (b *Body) Ret() string { return b.Desc[strings.IndexByte(b.Desc, ')')+1:] }

// HasHandlers reports whether the body, or a private method it inlines,
// has exception handlers. Their frames start with an empty stack.
func (b *Body) HasHandlers() bool {
	if len(b.Handlers) > 0 {
		return true
	}
	for _, c := range b.Calls {
		if c.HasHandlers() {
			return true
		}
	}
	return false
}

// Owner returns the class to refer to for a member of class owner. A
// multifile part's members are referred to through its facade, which
// inherits them: the part itself isn't public.
func (b *Body) Owner(owner string) string {
	if owner == b.Class && b.Facade != "" {
		return b.Facade
	}
	return owner
}

// anonymousObjects records the lambdas and objects of the body's class
// that its instructions create without a needClassReification call: a
// new, or the getstatic of an object's INSTANCE, of a class named as
// kotlinc names them, Class$f$1.
func (b *Body) anonymousObjects() {
	for i, in := range b.Insns {
		name := ""
		switch {
		case in.Op == cf.NEW:
			name = in.Class
		case in.Op == cf.GETSTATIC && in.Name == "INSTANCE" && in.Desc == "L"+in.Owner+";":
			name = in.Owner
		default:
			continue
		}
		if _, ok := b.reify[i]; ok || !anonymousOf(b.Class, name) {
			continue
		}
		b.copy[i] = name
	}
}

// anonymousOf reports whether class name is a lambda or object of class
// owner, as kotlinc names them: owner$f$1, owner$f$1$2.
func anonymousOf(owner, name string) bool {
	rest, ok := strings.CutPrefix(name, owner+"$")
	if !ok {
		return false
	}
	last := rest[strings.LastIndexByte(rest, '$')+1:]
	if last == "" || last == rest {
		return false
	}
	for _, r := range last {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Error is a body that can't be inlined.
type Error struct{ Why string }

func (e *Error) Error() string { return e.Why }

func fail(format string, args ...any) error { return &Error{fmt.Sprintf(format, args...)} }

// maxDepth limits how deeply private methods are inlined into each other.
const maxDepth = 8

// Analyze decodes and analyzes a method's body. read reads the other
// methods of its class, by name and descriptor.
func Analyze(m *cf.Method, read func(name, desc string) (*cf.Method, error)) (*Body, error) {
	return analyze(m, read, 0)
}

func analyze(m *cf.Method, read func(name, desc string) (*cf.Method, error), depth int) (*Body, error) {
	insns, err := m.Decode()
	if ue, ok := errors.AsType[*cf.UnsupportedError](err); ok {
		return nil, fail("%s", ue.Error())
	}
	if err != nil {
		return nil, err
	}
	b := &Body{Method: m, Insns: insns, Drop: make([]bool, len(insns)), Calls: map[int]*Body{},
		FrameParams: map[int][]int{}, Index: map[int]int{}, reify: map[int]string{}, copy: map[int]string{}}
	for i, in := range insns {
		b.Index[in.Off] = i
		// The copy makes a call site of its own, with the bootstrap
		// method's static arguments, which string concatenation's are:
		// constants. A lambda's (LambdaMetafactory) refers to a private
		// method of the body's class.
		if in.Op == cf.INVOKEDYNAMIC && (in.Bootstrap.Owner != "java/lang/invoke/StringConcatFactory" ||
			slices.ContainsFunc(in.Bootstrap.Args, func(c cf.Const) bool { return c.Kind == cf.ConstOther })) {
			return nil, fail("its body uses invokedynamic")
		}
	}
	slot := 0
	ds := params(m.Desc)
	if m.Access&cf.AccStatic == 0 {
		b.This = true
		ds = append([]string{"L" + m.Class + ";"}, ds...)
	}
	for _, d := range ds {
		p := &Param{Slot: slot, Desc: d, Arity: -1}
		if n, ok := functionArity(d); ok {
			p.Arity = n
		}
		b.Params = append(b.Params, p)
		slot++
		if d == "J" || d == "D" {
			slot++
		}
	}
	for _, in := range insns {
		targets := in.Targets
		if in.Op == cf.GOTO || cf.IFEQ <= in.Op && in.Op <= cf.IF_ACMPNE || in.Op == cf.IFNULL || in.Op == cf.IFNONNULL ||
			in.Op == cf.TABLESWITCH || in.Op == cf.LOOKUPSWITCH {
			targets = append(targets, in.Target)
		}
		for _, t := range targets {
			if m.Frames[t] == nil {
				return nil, fail("its class file has no stack map frames")
			}
		}
	}
	for _, h := range m.Handlers {
		if m.Frames[h.Handler] == nil {
			return nil, fail("its class file has no stack map frames")
		}
	}
	for off, f := range m.Frames {
		for _, t := range f.Locals {
			if t.Tag() == cf.VUninit || t.Tag() == cf.VUninitThis {
				return nil, fail("it keeps an object under construction in a local")
			}
		}
		for _, t := range f.Stack {
			if t.Tag() == cf.VUninitThis {
				return nil, fail("it branches in a constructor")
			}
			if t.Tag() == cf.VUninit {
				i, ok := b.Index[t.Off()]
				if !ok || insns[i].Op != cf.NEW {
					return nil, fail("a frame at %d refers to no new instruction", off)
				}
			}
		}
	}
	if err := b.calls(read, depth); err != nil {
		return nil, err
	}
	if err := b.markers(); err != nil {
		return nil, err
	}
	b.anonymousObjects()
	b.nullChecks()
	// Parameters whose values escape are found by running the analysis
	// until no more do.
	escaped := map[int]string{}
	for i, p := range b.Params {
		switch {
		case p.Arity < 0:
		case b.stores(p.Slot):
			escaped[i] = "the body assigns its parameter"
		}
	}
	for {
		n := len(escaped)
		if err := b.flow(escaped); err != nil {
			return nil, err
		}
		if len(escaped) == n {
			break
		}
	}
	for i, p := range b.Params {
		if p.Arity < 0 {
			continue
		}
		if why, ok := escaped[i]; ok {
			p.Why = why
			p.Loads, p.Invokes, p.InTry = nil, nil, nil
			continue
		}
		p.Inlinable = true
		for _, inv := range p.Invokes {
			_, ok := b.Finallies(insns[inv].Off)
			p.InTry = append(p.InTry, !ok)
		}
	}
	return b, nil
}

// params splits a method descriptor's parameter types.
func params(d string) []string {
	var out []string
	for i := 1; d[i] != ')'; {
		j := i
		for d[j] == '[' {
			j++
		}
		if d[j] == 'L' {
			j += strings.IndexByte(d[j:], ';')
		}
		out = append(out, d[i:j+1])
		i = j + 1
	}
	return out
}

const functionPrefix = "Lkotlin/jvm/functions/Function"

// functionArity returns N for the descriptor of kotlin.jvm.functions.FunctionN.
func functionArity(d string) (int, bool) {
	s, ok := strings.CutPrefix(d, functionPrefix)
	if !ok || !strings.HasSuffix(s, ";") {
		return 0, false
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	return n, err == nil && n >= 0 && n <= 22
}

// isInvoke reports whether an instruction is FunctionN.invoke with its
// erased signature, and returns N.
func isInvoke(in *cf.Insn) (int, bool) {
	if in.Op != cf.INVOKEINTERFACE || in.Name != "invoke" {
		return 0, false
	}
	n, ok := functionArity("L" + in.Owner + ";")
	return n, ok && in.Desc == "("+strings.Repeat("Ljava/lang/Object;", n)+")Ljava/lang/Object;"
}

// Reads reports whether the body reads parameter i, other than in the
// instructions it drops, such as null checks.
func (b *Body) Reads(i int) bool {
	slot := b.Params[i].Slot
	for j, in := range b.Insns {
		if !b.Drop[j] && (cf.ILOAD <= in.Op && in.Op <= cf.ALOAD || in.Op == cf.IINC) && in.Local == slot {
			return true
		}
	}
	return false
}

// stores reports whether the body assigns a local.
// Stores reports whether the body stores to a local: one of its
// parameters' that it never stores to keeps its argument.
func (b *Body) Stores(slot int) bool { return b.stores(slot) }

func (b *Body) stores(slot int) bool {
	for _, in := range b.Insns {
		if (cf.ISTORE <= in.Op && in.Op <= cf.ASTORE || in.Op == cf.IINC) && in.Local == slot {
			return true
		}
	}
	return false
}

// calls finds the calls of the class's own methods: a private or
// package-private one is inlined, and a public one is called through the
// facade.
func (b *Body) calls(read func(name, desc string) (*cf.Method, error), depth int) error {
	for i, in := range b.Insns {
		if in.Owner != b.Class || in.Op < cf.INVOKEVIRTUAL || in.Op > cf.INVOKEINTERFACE {
			continue
		}
		m, err := read(in.Name, in.Desc)
		if err != nil {
			return fail("it calls %s%s, which can't be read: %v", in.Name, in.Desc, err)
		}
		if m.Access&cf.AccPublic != 0 {
			continue
		}
		if in.Op == cf.INVOKEINTERFACE || depth >= maxDepth {
			return fail("it calls the private method %s", in.Name)
		}
		c, err := analyze(m, read, depth+1)
		if err != nil {
			return err
		}
		b.Calls[i] = c
	}
	return nil
}

// markers marks the calls of kotlin.jvm.internal.InlineMarker and the
// reified operation markers of Intrinsics, which kotlinc's inliner reads
// and removes, and the constants they take. A reified operation marker's
// instruction is recorded in Reified.
func (b *Body) markers() error {
	for i, in := range b.Insns {
		if in.Op != cf.INVOKESTATIC {
			continue
		}
		switch {
		case in.Owner == "kotlin/jvm/internal/InlineMarker":
		case in.Owner == "kotlin/jvm/internal/Intrinsics" && (in.Name == "reifiedOperationMarker" || in.Name == "needClassReification"):
			// Markers of reified operations, which kotlinc's inliner reads
			// and removes too.
		default:
			continue
		}
		b.Drop[i] = true
		switch in.Desc {
		case "()V":
			if in.Name != "needClassReification" {
				break
			}
			// The class that the next instruction creates operates on a
			// reified type parameter: copies of it are created.
			next := &b.Insns[min(i+1, len(b.Insns)-1)]
			switch {
			case i+1 < len(b.Insns) && next.Op == cf.NEW:
				b.reify[i+1] = next.Class
			case i+1 < len(b.Insns) && next.Op == cf.GETSTATIC && next.Name == "INSTANCE" && next.Desc == "L"+next.Owner+";":
				b.reify[i+1] = next.Owner
			default:
				return fail("its needClassReification call doesn't precede an object's creation")
			}
		case "(I)V", "(Ljava/lang/String;)V":
			if i == 0 || !isConst(&b.Insns[i-1]) || b.Frames[b.Insns[i].Off] != nil {
				return fail("its InlineMarker.%s call doesn't follow a constant", in.Name)
			}
			b.Drop[i-1] = true
			if k := constInt(&b.Insns[i-1]); in.Name == "mark" && in.Desc == "(I)V" && (k == BeforeSuspend || k == AfterSuspend) {
				if b.Suspends == nil {
					b.Suspends = map[int]int{}
				}
				b.Suspends[i] = k
			}
			if k := constInt(&b.Insns[i-1]); in.Name == "mark" && in.Desc == "(I)V" && k == FakeContinuation &&
				i+1 < len(b.Insns) && b.Insns[i+1].Op == cf.ACONST_NULL {
				if b.Fakes == nil {
					b.Fakes = map[int]bool{}
				}
				b.Fakes[i+1] = true
			}
		case "(ILjava/lang/String;)V":
			// reifiedOperationMarker(kind, "T"), before the operation.
			kind, name := &b.Insns[max(i-2, 0)], &b.Insns[max(i-1, 0)]
			if i < 2 || !isConst(kind) || name.Op != cf.LDC || name.Const != cf.ConstString || i+1 >= len(b.Insns) {
				return fail("its InlineMarker.%s call doesn't follow constants", in.Name)
			}
			k := constInt(kind)
			next := b.Insns[i+1].Op
			switch op, ok := reifiedOps[k]; {
			case k == TypeOf && next == cf.ACONST_NULL:
			case k == Enum && next == cf.ACONST_NULL:
			case k == Enum && next == cf.ICONST_0:
				if i+2 >= len(b.Insns) || b.Insns[i+2].Op != cf.ANEWARRAY {
					return fail("its enumValues placeholder isn't an empty array")
				}
				b.Drop[i+2] = true
			case !ok || next != op:
				return fail("it has a reified operation kc doesn't support (%d)", k)
			}
			b.Drop[i-1], b.Drop[i-2] = true, true
			if b.Reified == nil {
				b.Reified = map[int]Reified{}
			}
			b.Reified[i+1] = Reified{Kind: k, Param: strings.TrimSuffix(name.Str, "?"), Nullable: strings.HasSuffix(name.Str, "?")}
		default:
			return fail("it calls InlineMarker.%s%s", in.Name, in.Desc)
		}
	}
	return nil
}

// nullChecks marks the null checks of parameters, which kotlinc's inliner
// leaves out too: the arguments come from Kotlin code, which is checked.
// A check is a load of the parameter, a string constant (its name) and a
// call of Intrinsics.checkNotNullParameter.
func (b *Body) nullChecks() {
	params := map[int]bool{}
	for _, p := range b.Params {
		if p.Desc[0] == 'L' || p.Desc[0] == '[' {
			params[p.Slot] = !b.stores(p.Slot)
		}
	}
	for i := 2; i < len(b.Insns); i++ {
		in, name, load := &b.Insns[i], &b.Insns[i-1], &b.Insns[i-2]
		if in.Op == cf.INVOKESTATIC && in.Owner == "kotlin/jvm/internal/Intrinsics" && in.Name == "checkNotNullParameter" &&
			name.Op == cf.LDC && name.Const == cf.ConstString && load.Op == cf.ALOAD && params[load.Local] &&
			b.Frames[name.Off] == nil && b.Frames[in.Off] == nil {
			b.Drop[i], b.Drop[i-1], b.Drop[i-2] = true, true, true
		}
	}
}

// constInt returns the value of an int constant instruction.
func constInt(in *cf.Insn) int {
	switch {
	case in.Op == cf.ICONST_M1:
		return -1
	case cf.ICONST_0 <= in.Op && in.Op <= cf.ICONST_5:
		return int(in.Op - cf.ICONST_0)
	}
	return int(in.Int)
}

func isConst(in *cf.Insn) bool {
	switch in.Op {
	case cf.ICONST_M1, cf.ICONST_0, cf.ICONST_0 + 1, cf.ICONST_0 + 2, cf.ICONST_0 + 3, cf.ICONST_0 + 4, cf.ICONST_5,
		cf.BIPUSH, cf.SIPUSH:
		return true
	case cf.LDC:
		return in.Const == cf.ConstInt || in.Const == cf.ConstString
	}
	return false
}

// val is a value on the operand stack during the analysis: its size in
// slots, and the parameter it is, or -1.
type val struct {
	size  int8
	param int8
}

// flow runs the analysis: it follows the control flow from the start
// and from each handler, with the operand stack of each instruction. A
// function-typed parameter that is used other than by a null check or an
// invoke is added to escaped, with the reason.
func (b *Body) flow(escaped map[int]string) error {
	for _, p := range b.Params {
		p.Loads, p.Invokes = nil, nil
	}
	bySlot := map[int]int{}
	for i, p := range b.Params {
		if p.Arity >= 0 {
			if _, ok := escaped[i]; !ok {
				bySlot[p.Slot] = i
			}
		}
	}
	escape := func(v val, why string) {
		if v.param >= 0 {
			if _, ok := escaped[int(v.param)]; !ok {
				escaped[int(v.param)] = why
			}
		}
	}
	states := map[int][]val{} // by offset: the stack where it is reached
	var work []int
	reach := func(off int, st []val) error {
		i, ok := b.Index[off]
		if !ok {
			return fmt.Errorf("branch to %d, which is not an instruction", off)
		}
		prev, seen := states[off]
		if !seen {
			states[off] = st
			work = append(work, i)
			return nil
		}
		if len(prev) != len(st) {
			return fmt.Errorf("inconsistent stack depth at %d", off)
		}
		for k := range st {
			if prev[k].size != st[k].size {
				return fmt.Errorf("inconsistent stack at %d", off)
			}
			if prev[k].param != st[k].param {
				escape(prev[k], "it is used where two paths meet")
				escape(st[k], "it is used where two paths meet")
			}
		}
		return nil
	}
	if err := reach(0, nil); err != nil {
		return err
	}
	for len(work) > 0 {
		i := work[len(work)-1]
		work = work[:len(work)-1]
		in := &b.Insns[i]
		st := append([]val(nil), states[in.Off]...)
		for _, h := range b.Handlers {
			if h.Start <= in.Off && in.Off < h.End {
				if err := reach(h.Handler, []val{{1, -1}}); err != nil {
					return err
				}
			}
		}
		pop := func(slots int) error {
			for slots > 0 {
				if len(st) == 0 {
					return fmt.Errorf("stack underflow at %d", in.Off)
				}
				v := st[len(st)-1]
				st = st[:len(st)-1]
				escape(v, "the body passes it on or stores it")
				slots -= int(v.size)
			}
			if slots < 0 {
				return fmt.Errorf("inconsistent stack at %d", in.Off)
			}
			return nil
		}
		top := func(k int) (val, error) {
			if k >= len(st) {
				return val{}, fmt.Errorf("stack underflow at %d", in.Off)
			}
			return st[len(st)-1-k], nil
		}
		next := true
		var targets []int
		switch op := in.Op; {
		case b.Drop[i]:
			// A marker's call or constant, or enumValues' placeholder
			// anewarray, which takes the length and leaves the array.
			pops, pushes := in.Effect()
			if err := pop(pops); err != nil {
				return err
			}
			if pushes > 0 {
				st = append(st, val{int8(pushes), -1})
			}
		case op == cf.ALOAD:
			v := val{1, -1}
			if pi, ok := bySlot[in.Local]; ok {
				v.param = int8(pi)
				b.Params[pi].Loads = append(b.Params[pi].Loads, i)
			}
			st = append(st, v)
		case op == cf.INVOKEINTERFACE:
			n, ok := isInvoke(in)
			if ok {
				recv, err := top(n)
				if err != nil {
					return err
				}
				if recv.param >= 0 && b.Params[recv.param].Arity == n {
					p := b.Params[recv.param]
					p.Invokes = append(p.Invokes, i)
					for _, v := range st[len(st)-n:] {
						escape(v, "the body passes it to another function")
					}
					st = append(st[:len(st)-n-1], val{1, -1})
					break
				}
			}
			fallthrough
		default:
			pops, pushes := in.Effect()
			switch op {
			case cf.DUP, cf.DUP_X1, cf.DUP_X2, cf.DUP2, cf.DUP2_X1, cf.DUP2_X2, cf.SWAP:
				var err error
				if st, err = permute(op, st); err != nil {
					return fmt.Errorf("%v at %d", err, in.Off)
				}
			default:
				if err := pop(pops); err != nil {
					return err
				}
				if pushes > 0 {
					st = append(st, val{int8(pushes), -1})
				}
			}
			switch {
			case cf.IFEQ <= op && op <= cf.IF_ACMPNE, op == cf.IFNULL, op == cf.IFNONNULL:
				targets = append(targets, in.Target)
			case op == cf.GOTO:
				targets, next = append(targets, in.Target), false
			case op == cf.TABLESWITCH, op == cf.LOOKUPSWITCH:
				targets, next = append(append(targets, in.Target), in.Targets...), false
			case cf.IRETURN <= op && op <= cf.RETURN, op == cf.ATHROW:
				next = false
			}
		}
		for _, t := range targets {
			if err := reach(t, st); err != nil {
				return err
			}
		}
		if next {
			if i+1 >= len(b.Insns) {
				return fmt.Errorf("code falls off the end")
			}
			if err := reach(b.Insns[i+1].Off, st); err != nil {
				return err
			}
		}
	}
	for off, f := range b.Frames {
		st, ok := states[off]
		ps := make([]int, len(f.Stack))
		for k := range ps {
			ps[k] = -1
			if ok && k < len(st) {
				ps[k] = int(st[k].param)
			}
		}
		if ok && len(st) != len(f.Stack) {
			return fmt.Errorf("the frame at %d has %d stack values, the code %d", off, len(f.Stack), len(st))
		}
		b.FrameParams[off] = ps
	}
	return nil
}

// permute applies a stack instruction to the analysis's stack.
func permute(op byte, st []val) ([]val, error) {
	n := len(st)
	get := func(k int) val { return st[n-1-k] }
	if n == 0 {
		return nil, errors.New("stack underflow")
	}
	need := func(k int) error {
		if n < k {
			return errors.New("stack underflow")
		}
		return nil
	}
	var out []val
	switch op {
	case cf.DUP:
		out = append(st, get(0))
	case cf.SWAP:
		if err := need(2); err != nil {
			return nil, err
		}
		out = append(st[:n-2:n-2], get(0), get(1))
	case cf.DUP_X1:
		if err := need(2); err != nil {
			return nil, err
		}
		out = append(st[:n-2:n-2], get(0), get(1), get(0))
	case cf.DUP_X2:
		if err := need(2); err != nil {
			return nil, err
		}
		if get(1).size == 2 {
			out = append(st[:n-2:n-2], get(0), get(1), get(0))
			break
		}
		if err := need(3); err != nil {
			return nil, err
		}
		out = append(st[:n-3:n-3], get(0), get(2), get(1), get(0))
	case cf.DUP2:
		if get(0).size == 2 {
			out = append(st, get(0))
			break
		}
		if err := need(2); err != nil {
			return nil, err
		}
		out = append(st, get(1), get(0))
	case cf.DUP2_X1:
		if err := need(2); err != nil {
			return nil, err
		}
		if get(0).size == 2 {
			out = append(st[:n-2:n-2], get(0), get(1), get(0))
			break
		}
		if err := need(3); err != nil {
			return nil, err
		}
		out = append(st[:n-3:n-3], get(1), get(0), get(2), get(1), get(0))
	case cf.DUP2_X2:
		if err := need(2); err != nil {
			return nil, err
		}
		switch {
		case get(0).size == 2 && get(1).size == 2:
			out = append(st[:n-2:n-2], get(0), get(1), get(0))
		case get(0).size == 2:
			if err := need(3); err != nil {
				return nil, err
			}
			out = append(st[:n-3:n-3], get(0), get(2), get(1), get(0))
		default:
			if err := need(3); err != nil {
				return nil, err
			}
			if get(2).size == 2 {
				out = append(st[:n-3:n-3], get(1), get(0), get(2), get(1), get(0))
				break
			}
			if err := need(4); err != nil {
				return nil, err
			}
			out = append(st[:n-4:n-4], get(1), get(0), get(3), get(2), get(1), get(0))
		}
	}
	return out, nil
}

// ReturnsSuspended reports whether a suspend function's body returns the
// result of each of its suspension points as it is: after each
// AfterSuspend marker, the code only jumps to a return of it. A caller
// can then return the result of a call in tail position, without a state
// machine, as kotlinc does.
func (b *Body) ReturnsSuspended() bool {
	if len(b.Suspends) == 0 {
		return false
	}
	for i, k := range b.Suspends {
		if k != AfterSuspend {
			continue
		}
		seen := map[int]bool{}
		for j := i + 1; ; j++ {
			if j >= len(b.Insns) || seen[j] {
				return false
			}
			seen[j] = true
			in := &b.Insns[j]
			switch {
			case b.Drop[j] || in.Op == cf.NOP:
			case in.Op == cf.GOTO:
				t, ok := b.Index[in.Target]
				if !ok {
					return false
				}
				j = t - 1
			case in.Op == cf.ARETURN:
				goto next
			default:
				return false
			}
		}
	next:
	}
	return true
}
