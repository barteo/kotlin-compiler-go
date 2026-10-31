package codegen

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/inline"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The inliner.
//
// A call of an inline function of the class path copies the function's
// body, read from its class file and analyzed by package inline, into the
// code being generated. The arguments are evaluated as for a call, then
// stored in new locals; the body's instructions follow, with:
//
//   - its locals mapped to new locals, one per slot and kind of value. A
//     reference local is declared Object, and each load is followed by a
//     cast to the type the body's code has there: the type last stored, or
//     the type its stack map frame gives. Bodies reuse slots for values of
//     unrelated types, which locals of one declared type couldn't hold.
//   - a label with the frame of each of the body's stack map frames, below
//     which are the values that were on the stack before the call.
//   - its exception handlers, with their ranges mapped to the copy. Their
//     frames start with an empty stack, so the values on the stack before
//     the call are spilled to locals first, as for a try.
//   - each return replaced by a jump to the end, where the result is on
//     the stack as the call would leave it.
//   - its references to its own class, a multifile part, made through the
//     facade, and calls of its class's private methods inlined in turn.
//   - the null checks of its parameters and kotlinc's InlineMarker calls
//     left out.
//
// A lambda passed for a parameter that the body only invokes (see
// inline.Param.Inlinable) is inlined too: it creates no object, and the
// body's loads of the parameter are left out. At each
// invoke, the invoke's arguments are on the stack, as Objects. They are
// stored in new locals, converted to the lambda's parameter types, and the
// lambda's body is generated there, as part of the enclosing function:
// its locals, this, loops and trys are the function's. Its result is boxed,
// as invoke's is. A body that invokes a lambda twice gets two copies of it.

// invokeFun calls a top-level or extension function of the class path,
// whose arguments are on the stack, or copies its body there if it is
// inline. A mask other than 0 marks omitted arguments.
func (g *gen) invokeFun(x *syntax.Call, f *types.FunSym, mask int) {
	b := inlineBody(f, mask)
	if b == nil {
		if f.Suspend {
			g.calledSuspend(x)
		}
		g.invokeStatic(f.Facade, f, mask, false)
		return
	}
	if mask != 0 {
		// name$default takes the mask and a marker after the arguments.
		g.code.Iconst(int32(mask))
		g.code.Op(cf.ACONST_NULL)
	}
	g.inline(b, x)
	if f.Suspend {
		g.inlinedSuspend(x)
	}
}

// inlineBody returns the body that a call of f copies, or nil if it calls
// f: a public inline function whose body kc can't copy is called. With
// omitted arguments, it is f's name$default method's.
func inlineBody(f *types.FunSym, mask int) *inline.Body {
	in := f.Inline
	if in == nil {
		return nil
	}
	var b *inline.Body
	var err error
	if mask != 0 {
		b, err = in.Defaults()
	} else {
		b, err = in.Body()
	}
	switch {
	case err != nil && in.Private:
		panic("can't inline " + f.Name + ": " + err.Error()) // the checker reports it
	case err != nil:
		return nil
	}
	return b
}

// inlineLambda is a lambda whose body is being generated where an inlined
// function invokes it.
type inlineLambda struct {
	l     *syntax.Lambda
	end   *cf.Label // where its result is on the stack, as an Object
	depth int       // the stack depth where its body starts
	tries int       // len(g.tries) where its body starts
	// direct is set on a lambda inlined into a copy of the program's
	// inline function: its result is left as its own type, not boxed.
	direct bool
	// groups is the number of replace groups open where its body starts,
	// which its returns don't end: see composeInlineStart.
	groups int
}

type localKey struct {
	slot int
	kind byte // I, J, F, D or A
}

// inliner copies one body into the code being generated.
type inliner struct {
	g      *gen
	c      *cf.Code
	b      *inline.Body
	x      *syntax.Call
	locals map[localKey]int
	// refs holds the type of the reference in each of the body's slots,
	// where it is known to be more specific than Object.
	refs   map[int]cf.VType
	labels map[int]*cf.Label // the frames' labels, by offset in the body
	// point is the suspension point being copied, between its markers,
	// and gaps the code of resume points that the body's handlers don't
	// protect.
	point  *suspendPoint
	gaps   *tryCtx
	pos    map[int]int // where the copy of each instruction starts, by offset in the body
	prefix []cf.VType  // the stack below the body's values
	start  []int       // the locals that hold a value where the body starts
	// callerLocals is the number of the caller's locals, allocated before
	// the body's: frames may add those that an inlined lambda assigns.
	callerLocals int
	end          *cf.Label
	snap         cf.Snapshot
	// lambdas are the inlined lambdas, by parameter; invokes are the
	// instructions that invoke them, and skip the parameters' loads.
	lambdas map[int]*syntax.Lambda
	invokes map[int]*syntax.Lambda
	skip    map[int]bool
	// owners are the copies of the program's inline functions whose
	// callers' code forwarded lambdas are: see srcInline.owners.
	owners map[*syntax.Lambda]*srcInline
	// reified are the classes of the call's type arguments for reified
	// type parameters, by the parameters' names, for the instructions the
	// body marks: also those of the private methods it inlines.
	reified map[string]string
	// renamed are the classes the body creates that the copy creates
	// copies of instead, with the call's type arguments: see
	// inline.Template.
	renamed map[string]string
	// typed are the parameters whose locals have their arguments' types.
	typed map[int]bool
}

// cls returns the class that the copy refers to for class name.
func (in *inliner) cls(name string) string {
	if n, ok := in.renamed[name]; ok {
		return n
	}
	return name
}

// desc returns a descriptor with the renamed classes.
func (in *inliner) desc(d string) string {
	for old, n := range in.renamed {
		d = strings.ReplaceAll(d, "L"+old+";", "L"+n+";")
	}
	return d
}

// instantiate generates the copy of template t that the body's
// instruction creates, as kotlinc names it: Class$method$$inlined$f$N.
func (in *inliner) instantiate(t *inline.Template) {
	g := in.g
	method := strings.TrimSuffix(g.prefix, "$lambda")
	key := g.cls.Name + "$" + method + "$$inlined$" + in.b.Name
	g.shared.count[key]++
	name := key + "$" + strconv.Itoa(g.shared.count[key])
	reified, cloned := in.reified, false
	for p, cls := range in.reified {
		if strings.HasPrefix(cls, "?") {
			// A reified type parameter of the inline function whose method
			// this is: kc copies the object erased, where kotlinc would
			// copy it with markers for its callers.
			if !cloned {
				reified, cloned = maps.Clone(in.reified), true
			}
			reified[p] = "java/lang/Object"
		}
	}
	copies, err := t.Instantiate(name, reified)
	if err != nil {
		panic(err)
	}
	for _, c := range copies {
		g.shared.extra = append(g.shared.extra, Output{Name: c.Name, Bytes: c.Data})
	}
	if in.renamed == nil {
		in.renamed = map[string]string{}
	}
	in.renamed[t.Name] = name
}

// inline copies the body b in place of a call whose arguments are on the
// stack, in the order of b's parameters. It leaves the result, of the
// type of b's descriptor. The call's lambdas marked Inline are inlined;
// x is nil for a body inlined into another.
func (g *gen) inline(b *inline.Body, x *syntax.Call) {
	c := g.code
	if !c.Reachable() {
		return // the call is never reached
	}
	if x != nil {
		saved, savedTypes := g.reified, g.reifiedTypes
		g.reified, g.reifiedTypes = g.reifiedClasses(x)
		defer func() { g.reified, g.reifiedTypes = saved, savedTypes }()
	}
	in := &inliner{g: g, c: c, b: b, x: x, locals: map[localKey]int{}, refs: map[int]cf.VType{}, reified: g.reified,
		labels: map[int]*cf.Label{}, pos: map[int]int{},
		lambdas: map[int]*syntax.Lambda{}, invokes: map[int]*syntax.Lambda{}, skip: map[int]bool{}, typed: map[int]bool{},
		callerLocals: c.MaxLocals()}
	in.inlineLambdas()
	for i := len(b.Params) - 1; i >= 0; i-- {
		p := b.Params[i]
		if in.lambdas[i] != nil {
			c.Op(cf.POP) // the placeholder of an inlined lambda
			continue
		}
		k := kind(p.Desc)
		if k == 'A' {
			t := c.StackTop()
			in.refs[p.Slot] = t
			if t.Tag() == cf.VObject && !b.Stores(p.Slot) {
				// A parameter the body doesn't reassign keeps its
				// argument's type, so loads need no cast: kotlinc's
				// inliner follows a lambda only through plain loads.
				d := t.Class()
				if d[0] != '[' {
					d = "L" + d + ";"
				}
				l := c.AllocLocal(d)
				in.locals[localKey{p.Slot, k}] = l
				in.typed[p.Slot] = true
			}
		}
		c.Local(storeOpOf(kindDesc[k]), in.local(p.Slot, k))
		if x != nil && x.Sym.Suspend && i == len(b.Params)-1 && p.Desc == types.ContinuationDesc {
			// The caller's continuation, which its resume points restore.
			g.continuationAlias(in.local(p.Slot, k))
		}
	}
	var spilled []int
	if b.HasHandlers() {
		var ok bool
		if spilled, ok = c.Spill(); !ok {
			panic("an inlined function with exception handlers can't be an argument of a constructor call")
		}
	}
	in.prefix, in.start, in.snap = c.Stack(), c.Assigned(), c.Snapshot()
	ret := b.Ret()
	in.end = cf.NewLabel()
	if ret[0] == 'L' || ret[0] == '[' {
		in.end = cf.NewJoin(ret)
	}
	line := c.LastLine()
	in.gaps = &tryCtx{loops: len(g.loops)}
	g.tries = append(g.tries, in.gaps)
	in.copy()
	g.tries = g.tries[:len(g.tries)-1]
	c.Place(in.end)
	c.Line(line) // after the lines of inlined lambdas, the call's again
	if len(spilled) > 0 && c.Reachable() {
		tmp := -1
		if ret != "V" {
			tmp = c.AllocLocal(ret)
			c.Local(storeOpOf(ret), tmp)
		}
		c.Unspill(spilled)
		if tmp >= 0 {
			c.Local(loadOpOf(ret), tmp)
			c.Retire(tmp)
		}
	}
}

// inlineLambdas finds the call's lambdas that are inlined, and the
// instructions that load and invoke their parameters.
func (in *inliner) inlineLambdas() {
	x := in.x
	if x == nil {
		return
	}
	first := 0
	if x.Sym.Ext {
		first = 1 // the receiver comes first on the JVM
	}
	if in.b.This {
		first++ // after the dispatch receiver
	}
	for k, a := range x.Args {
		l, ok := a.(*syntax.Lambda)
		if ok && !l.Inline {
			continue
		}
		if !ok {
			var owner *srcInline
			if l, owner = in.g.forwarded(a); l == nil {
				continue
			}
			if in.owners == nil {
				in.owners = map[*syntax.Lambda]*srcInline{}
			}
			in.owners[l] = owner
		}
		i := k
		if x.ArgMap != nil {
			i = x.ArgMap[k]
		}
		p := in.b.Params[first+i]
		if !p.Inlinable {
			panic("a lambda is inlined for a parameter its function doesn't only invoke")
		}
		in.lambdas[first+i] = l
		for _, ld := range p.Loads {
			in.skip[ld] = true
		}
		for _, inv := range p.Invokes {
			in.invokes[inv] = l
		}
	}
}

// kind returns the kind of local that holds a value of a JVM type.
func kind(desc string) byte {
	switch desc[0] {
	case 'Z', 'B', 'C', 'S', 'I':
		return 'I'
	case 'J', 'F', 'D':
		return desc[0]
	}
	return 'A'
}

// kindDesc is the declared type of a local of each kind.
var kindDesc = map[byte]string{'I': "I", 'J': "J", 'F': "F", 'D': "D", 'A': "Ljava/lang/Object;"}

// vkind returns the kind of local that holds a value of a verification
// type.
func vkind(t cf.VType) byte {
	switch t.Tag() {
	case cf.VInt:
		return 'I'
	case cf.VLong:
		return 'J'
	case cf.VFloat:
		return 'F'
	case cf.VDouble:
		return 'D'
	}
	return 'A'
}

// local returns the local that holds the body's slot for values of a
// kind, allocating it on first use.
func (in *inliner) local(slot int, k byte) int {
	key := localKey{slot, k}
	l, ok := in.locals[key]
	if !ok {
		l = in.c.AllocLocal(kindDesc[k])
		in.locals[key] = l
	}
	return l
}

// label returns the label of the frame at an offset of the body. It is
// created when first needed: an object under construction on the frame's
// stack refers to the copy of its new instruction, which comes first.
func (in *inliner) label(off int) *cf.Label {
	if l := in.labels[off]; l != nil {
		return l
	}
	f := in.b.Frames[off]
	if f == nil {
		panic("inlined code jumps to an offset without a frame")
	}
	stack := slices.Clone(in.prefix)
	for k, t := range f.Stack {
		if p := in.b.FrameParams[off][k]; p >= 0 && in.lambdas[p] != nil {
			continue // an inlined lambda, which is never on the stack
		}
		if t.Tag() == cf.VUninit {
			pos, ok := in.pos[t.Off()]
			if !ok {
				panic("inlined code refers to an object it hasn't created")
			}
			t = cf.UninitType(in.cls(in.b.Insns[in.b.Index[t.Off()]].Class), pos)
		} else if t.Tag() == cf.VObject && in.renamed[t.Class()] != "" {
			t = cf.ObjectType(in.cls(t.Class()))
		}
		stack = append(stack, t)
	}
	assigned := slices.Clone(in.start)
	for slot, t := range f.Locals {
		if t.Tag() != cf.VTop && !in.lambdaSlot(slot) {
			assigned = append(assigned, in.local(slot, vkind(t)))
		}
	}
	l := in.c.NewFrameLabel(stack, assigned)
	l.Below(in.callerLocals)
	in.labels[off] = l
	return l
}

// place places the label of the frame at an offset, and takes the types
// of the references in locals from the frame.
func (in *inliner) place(off int) {
	in.c.Place(in.label(off))
	clear(in.refs)
	for slot, t := range in.b.Frames[off].Locals {
		if t.Tag() == cf.VObject || t.Tag() == cf.VNull {
			in.refs[slot] = t
		}
	}
}

// copy copies the body's instructions and exception handlers.
func (in *inliner) copy() {
	b, c := in.b, in.c
	for i := range b.Insns {
		ins := &b.Insns[i]
		if b.Frames[ins.Off] != nil {
			in.place(ins.Off)
		}
		in.pos[ins.Off] = c.Pos()
		if k, ok := b.Suspends[i]; ok && c.Reachable() {
			in.point = in.g.inlineSuspend(k, in.point)
		}
		if !b.Drop[i] && !in.skip[i] {
			in.insn(i)
		}
	}
	in.pos[len(b.Code)] = c.Pos()
	for _, h := range b.Handlers {
		c.Catch(in.snap, in.gaps.ranges(in.pos[h.Start], in.pos[h.End]), in.label(h.Handler), h.Catch)
	}
}

// insn copies the instruction at index i.
func (in *inliner) insn(i int) {
	b, c := in.b, in.c
	if !c.Reachable() {
		return // after a lambda that doesn't complete normally
	}
	ins := &b.Insns[i]
	if b.Fakes[i] {
		if in.g.sm != nil {
			in.g.currentContinuation()
			return
		}
		in.g.mark(inline.FakeContinuation) // still a placeholder, in a copy that is inlined in turn
	}
	if t := b.Templates[i]; t != nil {
		in.instantiate(t)
	}
	if t := b.Copies[i]; t != nil && in.g.publicInline && in.renamed[t.Name] == "" {
		// In the method of a public inline function, which kotlinc's
		// inliner copies into callers in other modules with the classes
		// it creates: they must be this module's, as kotlinc's copy is.
		in.instantiate(t)
	}
	if r, ok := b.Reified[i]; ok {
		cls, ok := in.reified[r.Param]
		if !ok {
			panic("no type argument for reified type parameter " + r.Param)
		}
		if name, ok := strings.CutPrefix(cls, "?"); ok {
			// Still a reified type parameter: the marker, with its name,
			// and the erased instruction, for the copies of this copy.
			if r.Nullable && !strings.HasSuffix(name, "?") {
				name += "?"
			}
			c.Iconst(int32(r.Kind))
			c.Sconst(name)
			c.Invoke(cf.INVOKESTATIC, intrinsics, "reifiedOperationMarker", "(ILjava/lang/String;)V")
			switch {
			case r.Kind == inline.TypeOf:
				c.Op(cf.ACONST_NULL)
			case r.Kind == inline.Enum && ins.Op == cf.ICONST_0:
				c.Iconst(0)
				c.ANewArray("java/lang/Enum") // the placeholder's dropped anewarray
			case r.Kind == inline.Enum:
				c.Op(cf.ACONST_NULL)
			case r.Kind == inline.SafeAs || r.Kind == inline.As:
				c.Checkcast(ins.Class)
			case r.Kind == inline.NewArray:
				c.ANewArray(ins.Class)
			case r.Kind == inline.Is:
				c.InstanceOf(ins.Class)
			case r.Kind == inline.JavaClass:
				c.LdcClass(ins.Class)
			}
			return
		}
		switch r.Kind {
		case inline.TypeOf:
			t := in.g.reifiedTypes[r.Param]
			if r.Nullable {
				t = t.Nullable()
			}
			in.g.typeOf(t)
		case inline.NewArray:
			c.ANewArray(cls)
		case inline.As:
			c.Checkcast(cls)
		case inline.SafeAs:
			// x as? C: x if it is a C, else null.
			ok, end := cf.NewLabel(), cf.NewJoin(refDesc(cls))
			c.Op(cf.DUP)
			c.InstanceOf(cls)
			c.Jump(cf.IFNE, ok)
			c.Op(cf.POP)
			c.Op(cf.ACONST_NULL)
			c.Jump(cf.GOTO, end)
			c.Place(ok)
			c.Checkcast(cls)
			c.Place(end)
		case inline.Is:
			c.InstanceOf(cls)
		case inline.JavaClass:
			c.LdcClass(cls)
		case inline.Enum:
			if ins.Op == cf.ICONST_0 {
				c.Invoke(cf.INVOKESTATIC, cls, "values", "()[L"+cls+";") // enumValues
			} else {
				c.LdcClass(cls) // enumValueOf: Enum.valueOf(C.class, name)
			}
		}
		return
	}
	switch op := ins.Op; {
	case op == cf.NOP:
	case op == cf.BIPUSH || op == cf.SIPUSH:
		c.Iconst(int32(ins.Int))
	case op == cf.LDC:
		switch ins.Const {
		case cf.ConstInt:
			c.Iconst(int32(ins.Int))
		case cf.ConstFloat:
			c.Fconst(float32(ins.Float))
		case cf.ConstLong:
			c.Lconst(ins.Int)
		case cf.ConstDouble:
			c.Dconst(ins.Float)
		case cf.ConstString:
			c.Sconst(ins.Str)
		case cf.ConstClass:
			c.LdcClass(ins.Class)
		}
	case op == cf.ALOAD:
		t, ok := in.refs[ins.Local]
		if ok && t.Tag() == cf.VNull {
			c.Op(cf.ACONST_NULL) // the verifier knows it is null
			break
		}
		c.Local(cf.ALOAD, in.local(ins.Local, 'A'))
		if ok && t.Tag() == cf.VObject && t.Class() != "java/lang/Object" && !in.typed[ins.Local] && !nullChecked(b.Insns, i) {
			c.Checkcast(in.cls(t.Class()))
		}
	case cf.ILOAD <= op && op <= cf.DLOAD:
		c.Local(op, in.local(ins.Local, "IJFD"[op-cf.ILOAD]))
	case op == cf.ASTORE:
		t := c.StackTop()
		if t.Tag() == cf.VUninit {
			panic("inlined code stores an object under construction")
		}
		in.refs[ins.Local] = t
		c.Local(cf.ASTORE, in.local(ins.Local, 'A'))
	case cf.ISTORE <= op && op <= cf.DSTORE:
		c.Local(op, in.local(ins.Local, "IJFD"[op-cf.ISTORE]))
	case op == cf.IINC:
		c.Iinc(in.local(ins.Local, 'I'), int(ins.Int))
	case cf.IFEQ <= op && op <= cf.IF_ACMPNE, op == cf.GOTO, op == cf.IFNULL, op == cf.IFNONNULL:
		c.Jump(op, in.label(ins.Target))
	case op == cf.TABLESWITCH || op == cf.LOOKUPSWITCH:
		targets := make([]*cf.Label, len(ins.Targets))
		for k, t := range ins.Targets {
			targets[k] = in.label(t)
		}
		if op == cf.TABLESWITCH {
			c.TableSwitch(ins.Keys[0], in.label(ins.Target), targets)
		} else {
			c.LookupSwitch(in.label(ins.Target), ins.Keys, targets)
		}
	case cf.IRETURN <= op && op <= cf.ARETURN:
		if c.Depth() > len(in.prefix)+1 {
			// Values the body left below its result are dropped.
			d := typeDesc(c.StackTop())
			tmp := c.AllocLocal(d)
			c.Local(storeOpOf(d), tmp)
			c.PopTo(len(in.prefix))
			c.Local(loadOpOf(d), tmp)
		}
		in.toEnd(i)
	case op == cf.RETURN:
		c.PopTo(len(in.prefix))
		in.toEnd(i)
	case cf.GETSTATIC <= op && op <= cf.PUTFIELD:
		c.FieldInsn(op, in.cls(b.Owner(ins.Owner)), ins.Name, in.desc(ins.Desc))
	case cf.INVOKEVIRTUAL <= op && op <= cf.INVOKEINTERFACE:
		in.invoke(i)
	case op == cf.INVOKEDYNAMIC:
		// String concatenation (see inline.Analyze): the same call site,
		// with the bootstrap method in this class.
		bsm, p := ins.Bootstrap, &in.g.cls.Pool
		ref := p.Method(bsm.Owner, bsm.Name, bsm.Desc)
		if bsm.Iface {
			ref = p.InterfaceMethod(bsm.Owner, bsm.Name, bsm.Desc)
		}
		args := []uint16{}
		for _, a := range bsm.Args {
			switch a.Kind {
			case cf.ConstInt:
				args = append(args, p.Int(int32(a.Int)))
			case cf.ConstFloat:
				args = append(args, p.Float(float32(a.Float)))
			case cf.ConstLong:
				args = append(args, p.Long(a.Int))
			case cf.ConstDouble:
				args = append(args, p.Double(a.Float))
			case cf.ConstString:
				args = append(args, p.String(a.Str))
			case cf.ConstClass:
				args = append(args, p.Class(in.cls(a.Str)))
			default:
				panic("invokedynamic with a method handle or method type argument") // see inline.Analyze
			}
		}
		c.InvokeDynamic(p.Bootstrap(p.MethodHandle(bsm.Kind, ref), args...), ins.Name, in.desc(ins.Desc))
	case op == cf.NEW:
		c.New(in.cls(ins.Class))
	case op == cf.NEWARRAY:
		c.NewArray(byte(ins.Int))
	case op == cf.ANEWARRAY:
		c.ANewArray(ins.Class)
	case op == cf.CHECKCAST:
		c.Checkcast(in.cls(ins.Class))
	case op == cf.INSTANCEOF:
		c.InstanceOf(in.cls(ins.Class))
	case op == cf.MULTIANEWARRAY:
		c.MultiANewArray(ins.Class, int(ins.Int))
	default:
		c.Op(op)
	}
}

// nullChecked reports whether load i is of a parameter that the next
// instructions pass to Intrinsics.checkNotNullParameter, which needs no
// cast: kotlinc's inliner, run with assertions, wants the load right
// before the parameter's name, where it removes the checks of lambdas.
func nullChecked(insns []cf.Insn, i int) bool {
	if i+2 >= len(insns) {
		return false
	}
	name, call := &insns[i+1], &insns[i+2]
	return name.Op == cf.LDC && name.Const == cf.ConstString && call.Op == cf.INVOKESTATIC && call.Owner == intrinsics &&
		(call.Name == "checkNotNullParameter" || call.Name == "checkParameterIsNotNull")
}

// lambdaSlot reports whether a slot holds the parameter of an inlined
// lambda, which the copy never assigns.
func (in *inliner) lambdaSlot(slot int) bool {
	for j := range in.lambdas {
		if in.b.Params[j].Slot == slot {
			return true
		}
	}
	return false
}

// toEnd jumps from the return at index i to the end of the copy, unless
// the return is the body's last instruction: the end follows it.
func (in *inliner) toEnd(i int) {
	if i < len(in.b.Insns)-1 {
		in.c.Jump(cf.GOTO, in.end)
	}
}

// invoke copies a method call: a private method of the body's class is
// inlined.
func (in *inliner) invoke(i int) {
	b, c := in.b, in.c
	ins := &b.Insns[i]
	if sub := b.Calls[i]; sub != nil {
		in.g.inline(sub, nil)
		return
	}
	if l := in.invokes[i]; l != nil {
		// A jump out of the lambda leaves the body's trys around the
		// invoke, whose finally blocks it runs: withLock's unlock().
		g := in.g
		fs, _ := b.Finallies(ins.Off)
		n := len(g.tries)
		for k := len(fs) - 1; k >= 0; k-- {
			f := fs[k]
			g.tries = append(g.tries, &tryCtx{loops: len(g.loops), copyFinally: func() { in.copyFinally(f) }})
		}
		if owner := in.owners[l]; owner != nil {
			// A forwarded lambda: the code of that copy's caller.
			saved, savedNoLines := g.srcInl, g.noLines
			g.srcInl, g.noLines = owner.outer, owner.noLines
			g.inlineLambda(l)
			g.srcInl, g.noLines = saved, savedNoLines
		} else {
			g.inlineLambda(l)
		}
		g.tries = g.tries[:n]
		return
	}
	owner, d := in.cls(b.Owner(ins.Owner)), in.desc(ins.Desc)
	switch {
	case ins.Op == cf.INVOKEINTERFACE:
		c.InvokeInterface(owner, ins.Name, d)
	case ins.Iface && ins.Op == cf.INVOKESTATIC:
		c.InvokeStaticInterface(owner, ins.Name, d)
	case ins.Iface && ins.Op == cf.INVOKESPECIAL:
		c.InvokeSpecialInterface(owner, ins.Name, d)
	default:
		c.Invoke(ins.Op, owner, ins.Name, d)
	}
}

// copyFinally copies the finally block f of the body, for a jump out of
// its try in an inlined lambda. The copy isn't in the range of the body's
// handlers, and the locals' references have their types at the handler.
// Its jump targets have labels of their own, whose frames the copy's
// edges give: the handler's frames have its exception in a local.
func (in *inliner) copyFinally(f inline.Finally) {
	c := in.c
	start := c.Pos()
	saved, savedLabels := maps.Clone(in.refs), in.labels
	clear(in.refs)
	for slot, t := range in.b.Frames[f.Frame].Locals {
		if t.Tag() == cf.VObject || t.Tag() == cf.VNull {
			in.refs[slot] = t
		}
	}
	in.labels = map[int]*cf.Label{}
	for k := f.Start; k < f.End; k++ {
		if off := in.b.Insns[k].Off; in.b.Frames[off] != nil {
			in.labels[off] = cf.NewLabel()
		}
	}
	for k := f.Start; k < f.End; k++ {
		if off := in.b.Insns[k].Off; in.b.Frames[off] != nil {
			in.place(off)
		}
		if !in.b.Drop[k] && !in.skip[k] {
			in.insn(k)
		}
	}
	in.refs, in.labels = saved, savedLabels
	in.gaps.gaps = append(in.gaps.gaps, [2]int{start, c.Pos()})
}

// typeDesc returns the descriptor of a local that can hold a value of a
// verification type.
func typeDesc(t cf.VType) string {
	switch t.Tag() {
	case cf.VInt:
		return "I"
	case cf.VLong:
		return "J"
	case cf.VFloat:
		return "F"
	case cf.VDouble:
		return "D"
	case cf.VObject:
		if c := t.Class(); c[0] == '[' {
			return c
		}
		return "L" + t.Class() + ";"
	}
	return "Ljava/lang/Object;"
}

// refDesc returns the descriptor of a reference type by its internal
// name: an array's is its name.
func refDesc(cls string) string {
	if strings.HasPrefix(cls, "[") {
		return cls
	}
	return "L" + cls + ";"
}

// reifiedClasses returns the classes of call x's type arguments for its
// function's reified type parameters, by name: boxes for primitives.
func (g *gen) reifiedClasses(x *syntax.Call) (map[string]string, map[string]types.Type) {
	if x.Reified == nil || x.Sym == nil {
		return nil, nil
	}
	m := map[string]string{}
	ts := map[string]types.Type{}
	for i, tp := range x.Sym.TypeParams {
		if tp.Reified && i < len(x.Reified) {
			t := g.reify(x.Reified[i])
			ts[tp.Name] = t
			if g.reifiedParam(t) {
				// A reified type parameter of the inline function whose
				// method this is: the copy keeps the markers, with its name.
				name := t.Param().Name
				if t.N {
					name += "?"
				}
				m[tp.Name] = "?" + name
				continue
			}
			m[tp.Name] = types.ClassName(types.Erase(t))
		}
	}
	return m, ts
}

// inlineLambda generates the body of an inlined lambda where the function
// invokes it. The invoke's arguments are on the stack, as Objects; the
// lambda's result is left there, as an Object.
func (g *gen) inlineLambda(l *syntax.Lambda) {
	c := g.code
	if !c.Reachable() {
		return
	}
	scope := c.Scope()
	defer c.EndScope(scope)
	composer, flags := -1, []int(nil)
	if l.Composable {
		composer, flags = g.composeInvoke(l)
	}
	params := l.ParamSyms()
	args := make([]int, len(params))
	for k := len(args) - 1; k >= 0; k-- {
		args[k] = c.AllocLocal("Ljava/lang/Object;")
		c.Local(cf.ASTORE, args[k])
	}
	for k, p := range params {
		if size(p.Type) == 0 {
			continue
		}
		c.Local(cf.ALOAD, args[k])
		g.castErased(types.AnyType.Nullable(), p.Type)
		p.Slot = c.AllocLocal(desc(p.Type))
		g.named(p)
		c.Local(storeOp(p.Type), p.Slot)
	}
	ctx := &inlineLambda{l: l, end: cf.NewJoin("Ljava/lang/Object;"), depth: c.Depth(), tries: len(g.tries)}
	g.inl = append(g.inl, ctx)
	restore := g.composeInlineStart(ctx, composer, flags, g.scope())
	if l.Ret.K == types.Unit {
		g.block(l.Body, types.UnitType, false)
	} else {
		g.block(l.Body, l.Ret, true)
	}
	g.composeInlineEnd(ctx)
	g.inlineResult(ctx)
	restore()
	g.inl = g.inl[:len(g.inl)-1]
	c.Place(ctx.end)
}

// inlineResult boxes the result of an inlined lambda, of its result type
// (nothing for Unit), as invoke returns it.
func (g *gen) inlineResult(ctx *inlineLambda) {
	if !g.code.Reachable() || ctx.direct {
		return
	}
	if ret := ctx.l.Ret; ret.K == types.Unit {
		g.code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
	} else {
		g.coerce(ret, types.AnyType.Nullable())
	}
}

// inlineReturn generates return@label from an inlined lambda, which may
// be left from inlined lambdas inside it: the value is computed, the
// finally blocks of the trys in the lambda that it leaves run, and the
// values left on the stack in the lambda are dropped.
func (g *gen) inlineReturn(x *syntax.Jump) {
	c := g.code
	var ctx *inlineLambda
	for _, l := range g.inl {
		if l.l == x.Lambda {
			ctx = l
		}
	}
	if ctx == nil {
		panic("return@" + x.Label + " leaves a lambda that isn't being generated")
	}
	ret := ctx.l.Ret
	g.resultAs(x.X, ret)
	if !c.Reachable() {
		return
	}
	tmp := -1
	if size(ret) > 0 {
		tmp = c.AllocLocal(desc(ret))
		c.Local(storeOp(ret), tmp)
	}
	g.leaveTries(ctx.tries)
	c.PopTo(ctx.depth)
	if tmp >= 0 {
		c.Local(loadOp(ret), tmp)
	}
	g.endGroups(ctx.groups)
	g.inlineResult(ctx)
	c.Jump(cf.GOTO, ctx.end)
}
