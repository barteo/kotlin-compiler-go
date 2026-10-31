package codegen

import (
	"maps"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/inline"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Suspend functions.
//
// As with kotlinc, a suspend function takes a Continuation last and
// returns its result as an Object, or COROUTINE_SUSPENDED if it suspended:
// its caller then returns that too, and the continuation is resumed later
// with the result. A function whose every suspension point is a call
// whose result it returns, in tail form, just passes its continuation on.
// Any other is a state machine, whose state is kept in an instance of its
// continuation class, Owner$fun$1, a ContinuationImpl:
//
//   - on entry, the continuation it is passed is its own if it is being
//     resumed, which invokeSuspend marks by setting label's sign bit;
//     otherwise it creates one that will resume the caller's. Then, if
//     label isn't 0, it jumps to the dispatch, at the end of the method,
//     which jumps to the resume point label says.
//   - each suspension point stores the values on the operand stack in
//     locals, stores every local that holds a value in a field of the
//     continuation (L$0, I$0, J$0, F$0, D$0, ... by kind), sets label,
//     and makes the call. If the call suspended, the function returns
//     COROUTINE_SUSPENDED. Otherwise, or at the resume point, where the
//     locals are restored from the fields and the result is the one
//     invokeSuspend stored, the code goes on with the result.
//
// The code is generated in one pass: a resume point is a label with the
// frame of the dispatch, placed after the suspension's return, and the
// dispatch, a tableswitch, comes last, once the points are known.

const (
	continuationImpl = "kotlin/coroutines/jvm/internal/ContinuationImpl"
	continuationName = "kotlin/coroutines/Continuation"
	objectDesc       = "Ljava/lang/Object;"
)

// stateMachine is the suspend function being generated.
type stateMachine struct {
	name, desc string // the method
	// cls is the continuation class, or "" in tail form, where tails are
	// the calls whose results the function returns.
	cls   string
	tails map[*syntax.Call]bool
	unit  bool       // the function returns Unit
	ret   types.Type // the function's result
	// forInline is set on an inline function's body for inlining, which
	// marks its suspension points with InlineMarker calls instead, as
	// kotlinc's do: the inliner makes them the caller's.
	forInline bool
	// public is set on a function whose continuation class is public, as
	// kotlinc's is: one that a name$$forInline method creates.
	public bool
	// lambda is set on a suspend lambda's invokeSuspend, whose this is
	// the continuation. skip are the locals it doesn't spill: its
	// captures, which it loads from its fields.
	lambda bool
	skip   map[int]bool
	// aliases are the locals that hold the continuation: the parameters
	// of copies of inline suspend functions.
	aliases map[int]bool
	line    int // the function's declaration's
	// this is set on a member, whose continuation keeps this, to call it,
	// and thisParam if this is the method's first parameter.
	this, thisParam bool
	completion      int // the continuation it is passed
	// The locals of a state machine: its continuation, the result it is
	// resumed with and COROUTINE_SUSPENDED.
	cont, result, susp int
	entry              []int // the locals that hold a value at the dispatch
	dispatch           *cf.Label
	resumes            []*cf.Label
	fields             []string // the spill fields, in order of first use
	points             []*suspendPoint
	saved              []*suspendPoint // the points whose locals are saved, in order
	nextLines          []int           // the line after each of them, for DebugMetadata
	call               func(g *gen)    // invokeSuspend's call of the function, its arguments on the stack
}

// suspendPoint is a call of a suspend function being generated.
type suspendPoint struct {
	call  *syntax.Call // nil for a for loop's operator calls
	f     *types.FunSym
	stack []int // the locals that hold the values below the call
	// late is set on a call on a nullable receiver, x?.f(): the values
	// below go back after the null check's join (see safeCall).
	late  bool
	saved []savedLocal
	// zero are the locals that hold dead values: on resuming, they are
	// zero. nullOuts store dead references as null, unless the
	// references turn out to be live after the point, at join.
	zero     []savedLocal
	nullOuts []savedLocal
	aliases  []int // the locals that hold the continuation, which resuming restores
	join     int
	// resumed is where the code goes on with the locals restored, before
	// the result is checked: a failure goes to the handlers of the trys
	// around the point.
	resumed int
	// line is the call's line, and pos where the call starts.
	line, pos int
	armed     bool // the continuation is passed, and the call comes next
	// inline is set on the call of an inline function: its suspension
	// points are in its body.
	inline bool
}

// inlineSuspend generates a marker of a suspension point in the copy of
// an inline suspend function's body: before its call, the locals are
// saved, and after it, the result is checked. It returns the point that
// BeforeSuspend starts.
func (g *gen) inlineSuspend(kind int, p *suspendPoint) *suspendPoint {
	if g.sm != nil && g.sm.forInline {
		g.mark(kind) // for the inliner of this copy's copies
		return nil
	}
	if g.sm != nil && g.sm.cls == "" {
		// A call in tail position: the body returns its result, which
		// the function returns.
		return nil
	}
	if g.sm == nil || g.sm.cls == "" || !g.code.Reachable() {
		panic("a suspension point of an inlined function outside a state machine")
	}
	switch kind {
	case inline.BeforeSuspend:
		p = &suspendPoint{}
		g.saveLocals(p)
	case inline.AfterSuspend:
		if p == nil {
			panic("an inlined function's suspension point ends before it starts")
		}
		g.suspendCheck(p)
		p = nil
	}
	return p
}

// inlinedSuspend ends the call of an inline suspend function, copied in
// place of call x, whose result, an Object, is on the stack.
func (g *gen) inlinedSuspend(x *syntax.Call) {
	sm := g.sm
	if sm == nil || len(sm.points) == 0 {
		return
	}
	if p := sm.points[len(sm.points)-1]; p.call == x && p.inline && g.code.Reachable() {
		if sm.cls == "" {
			g.suspended(p) // returns it
			return
		}
		g.suspendFinish(p)
	}
}

// calledSuspend prepares the call of an inline suspend function that
// isn't copied, whose continuation is passed: the call is a suspension
// point.
func (g *gen) calledSuspend(x *syntax.Call) {
	sm := g.sm
	if sm == nil || len(sm.points) == 0 {
		return
	}
	if p := sm.points[len(sm.points)-1]; p.call == x && p.inline {
		p.inline = false
		if sm.forInline {
			g.mark(inline.BeforeSuspend)
		}
		g.saveLocals(p)
		g.code.OnCall(func() { g.suspended(p) })
	}
}

// forInline generates the name$$forInline method of the program's inline
// suspend function fd, as kotlinc does: its body, as inliners copy it,
// without a state machine, whose suspension points are calls between
// InlineMarker.mark(0) and mark(1). access are its flags: a member's, as
// kotlinc's, is a private method of its class, final if fd is, which
// other modules' inliners copy too. A private function has none: its
// own method is the body to inline.
func (g *gen) forInline(fd *syntax.FunDecl, access uint16) {
	g.lockInline()
	defer g.unlockInline()
	sym := fd.Sym
	if !sym.Private {
		saved := g.publicInline
		g.publicInline = true
		defer func() { g.publicInline = saved }()
	}
	g.code = g.cls.NewMethod(access, sym.JVM()+"$$forInline", funDesc(sym))
	if access&cf.AccStatic == 0 {
		g.code.AllocLocal("L" + g.cls.Name + ";") // this
	}
	g.resume = &resumeCall{forInline: true}
	g.functionBody(fd)
}

// objectForInline reports whether suspend member fd of class cls gets a
// name$$forInline method, as kotlinc's do: cls is the class of an object
// expression in an inline function that other modules copy, which
// captures a crossinline parameter, whose lambda kotlinc's inliner
// inlines into its copy of the class.
func objectForInline(cls *types.ClassSym, fd *syntax.FunDecl) bool {
	if cls == nil || !cls.Anonymous || fd.Sym.Owner != cls || !fd.Sym.Suspend || fd.Sym.Abstract {
		return false
	}
	cd, ok := cls.Decl.(*syntax.ClassDecl)
	if !ok || !inInline(cd) || cd.EnclosingFun.Private {
		return false
	}
	for _, v := range cd.Captures {
		if p, ok := v.Decl.(*syntax.Param); ok && p.Crossinline {
			return true
		}
	}
	return false
}

// memberForInline generates the name$$forInline method of suspend member
// fd of an object expression's class, with flags access, for kotlinc's
// inliner, which copies the class with the caller's lambda inlined: the
// body without the state machine, as forInline's, which creates the
// member's continuation between InlineMarker.mark(4) and mark(5), as
// suspendSAMMethod's copy does. The copy of the method is then a state
// machine with a copy of that class, which a member in tail form gets
// too, as kotlinc's does.
func (g *gen) memberForInline(fd *syntax.FunDecl, access uint16) {
	sym := fd.Sym
	saved := g.publicInline
	g.publicInline = true
	defer func() { g.publicInline = saved }()
	if g.shared.classes[fd.Continuation] == nil {
		name, d, owner := sym.JVM(), funDesc(sym), g.owner
		g.continuationClass(&stateMachine{name: name, desc: d, cls: fd.Continuation, this: true, public: true, line: fd.P.Line,
			ret: types.JVMRet(sym), call: func(g *gen) { g.callMethod(sym, owner, name, d) }})
	}
	g.code = g.cls.NewMethod(access, sym.JVM()+"$$forInline", funDesc(sym))
	g.code.AllocLocal("L" + g.cls.Name + ";") // this
	g.resume = &resumeCall{this: true, forInline: true, cont: fd.Continuation}
	g.functionBody(fd)
}

// fakeContinuation creates continuation class cont, whose constructor has
// descriptor desc, for the continuation in local k, between the markers
// that tell kotlinc's inliner the class to copy, and drops it.
func (g *gen) fakeContinuation(cont, desc string, k int) {
	code := g.code
	g.mark(inline.BeforeFakeContinuation)
	code.New(cont)
	code.Op(cf.DUP)
	code.Local(cf.ALOAD, 0)
	code.Local(cf.ALOAD, k)
	code.Invoke(cf.INVOKESPECIAL, cont, "<init>", desc)
	g.mark(inline.AfterFakeContinuation)
	code.Op(cf.POP)
}

// savedLocal is a local stored in a field of the continuation: a named
// variable, or a temporary. A null-out is the call that stores a dead
// reference as null, at pos.
type savedLocal struct {
	slot              int
	field, desc, name string
	pos               int
}

// suspendFun starts the body of suspend function fd, whose parameters,
// and its continuation last, are allocated.
func (g *gen) suspendFun(fd *syntax.FunDecl, name, desc string, completion int, resume *resumeCall) {
	sm := &stateMachine{name: name, desc: desc, completion: completion, unit: fd.Sym.Ret.K == types.Unit, this: resume.this,
		thisParam: resume.thisParam, call: resume.call, line: fd.P.Line, forInline: resume.forInline, public: resume.public, ret: types.JVMRet(fd.Sym)}
	g.sm = sm
	if sm.forInline {
		if resume.cont != "" {
			g.fakeContinuation(resume.cont, sm.ctorDesc(g.cls.Name), completion)
		}
		return
	}
	tail, tails := scanTail(fd, resume.lambda)
	if tail {
		sm.tails = tails
		return
	}
	sm.cls = fd.Continuation
	sm.skip = map[int]bool{}
	g.code.TraceLiveness()
	g.stateMachineEntry(sm)
}

// stateMachineEntry generates the start of a state machine: it finds or
// creates the continuation and jumps to the dispatch if it resumes.
func (g *gen) stateMachineEntry(sm *stateMachine) {
	c := g.code
	cls := sm.cls
	if sm.lambda {
		sm.susp = c.AllocLocal(objectDesc)
	} else {
		g.continuationEntry(sm)
	}
	c.Invoke(cf.INVOKESTATIC, "kotlin/coroutines/intrinsics/IntrinsicsKt", "getCOROUTINE_SUSPENDED", "()"+objectDesc)
	c.Local(cf.ASTORE, sm.susp)
	sm.entry = c.Assigned()
	sm.dispatch = cf.NewLabel()
	c.Local(cf.ALOAD, sm.cont)
	c.FieldInsn(cf.GETFIELD, cls, "label", "I")
	c.Jump(cf.IFNE, sm.dispatch)
	g.throwOnFailure(sm.result)
}

// continuationEntry finds the continuation of a suspend function, or
// creates it.
func (g *gen) continuationEntry(sm *stateMachine) {
	c := g.code
	cls := sm.cls
	sm.cont = c.AllocLocal("L" + cls + ";")
	c.NameLocal(sm.cont, "$continuation")
	sm.result = c.AllocLocal(objectDesc)
	c.NameLocal(sm.result, "$result")
	sm.susp = c.AllocLocal(objectDesc)
	create, got := cf.NewLabel(), cf.NewLabel()
	c.Local(cf.ALOAD, sm.completion)
	c.InstanceOf(cls)
	c.Jump(cf.IFEQ, create)
	c.Local(cf.ALOAD, sm.completion)
	c.Checkcast(cls)
	c.Local(cf.ASTORE, sm.cont)
	c.Local(cf.ALOAD, sm.cont)
	c.FieldInsn(cf.GETFIELD, cls, "label", "I")
	c.Iconst(math.MinInt32)
	c.Op(cf.IAND)
	c.Jump(cf.IFEQ, create)
	c.Local(cf.ALOAD, sm.cont)
	c.Op(cf.DUP)
	c.FieldInsn(cf.GETFIELD, cls, "label", "I")
	c.Iconst(math.MinInt32)
	c.Op(cf.ISUB)
	c.FieldInsn(cf.PUTFIELD, cls, "label", "I")
	c.Jump(cf.GOTO, got)
	c.Place(create)
	c.New(cls)
	c.Op(cf.DUP)
	if sm.this {
		c.Local(cf.ALOAD, 0)
	}
	c.Local(cf.ALOAD, sm.completion)
	c.Invoke(cf.INVOKESPECIAL, cls, "<init>", sm.ctorDesc(g.cls.Name))
	c.Local(cf.ASTORE, sm.cont)
	c.Place(got)
	c.Local(cf.ALOAD, sm.cont)
	c.FieldInsn(cf.GETFIELD, cls, "result", objectDesc)
	c.Local(cf.ASTORE, sm.result)
}

// suspendImpl generates an overridable suspend member fd of the class
// being generated, as kotlinc does: its body is a static method,
// name$suspendImpl, which takes this first, and which the method, in
// g.code, calls. The continuation resumes it, not an override.
func (g *gen) suspendImpl(fd *syntax.FunDecl, access uint16) bool {
	sym := fd.Sym
	if !sym.Suspend || sym.Owner == nil || sym.Owner != g.owner || sym.Final || sym.Private || access&cf.AccStatic != 0 || !openClass(sym.Owner) {
		return false
	}
	owner := g.cls.Name
	iface := sym.Owner.Interface
	d := funDesc(sym)
	name, implDesc := sym.JVM()+"$suspendImpl", "(L"+owner+";"+d[1:]
	if sym.JVMName != "" {
		name = sym.JVMName // kotlinc copies @JvmName to the static method
	}
	invoke := func(c *cf.Code) {
		if iface {
			c.InvokeStaticInterface(owner, name, implDesc)
		} else {
			c.Invoke(cf.INVOKESTATIC, owner, name, implDesc)
		}
	}
	c := g.code
	g.line(fd.P)
	for _, pd := range descParams(implDesc) {
		c.Local(loadOpOf(pd), c.AllocLocal(pd))
	}
	invoke(c)
	c.Op(cf.ARETURN)
	g.check()

	implAccess := uint16(cf.AccStatic | cf.AccSynthetic)
	if iface {
		implAccess |= cf.AccPublic
	}
	g.code = g.cls.NewMethod(implAccess, name, implDesc)
	g.code.NameLocal(g.code.AllocLocal("L"+owner+";"), "$this")
	g.resume = &resumeCall{this: true, thisParam: true, call: func(g *gen) { invoke(g.code) }}
	g.functionBody(fd)
	return true
}

// openClass reports whether a class of the program can be subclassed or
// implemented.
func openClass(cls *types.ClassSym) bool {
	cd, ok := cls.Decl.(*syntax.ClassDecl)
	return ok && (cd.Interface || cd.Open || cd.Abstract || cd.Sealed) && !cls.Anonymous && !cls.Value
}

// ctorDesc returns the descriptor of the continuation class's
// constructor: it takes the function's this, of class owner, if it has
// one, and the continuation to resume when it completes.
func (sm *stateMachine) ctorDesc(owner string) string {
	if sm.this {
		return "(L" + owner + ";" + types.ContinuationDesc + ")V"
	}
	return "(" + types.ContinuationDesc + ")V"
}

func (g *gen) throwOnFailure(result int) {
	g.code.Local(cf.ALOAD, result)
	g.code.Invoke(cf.INVOKESTATIC, "kotlin/ResultKt", "throwOnFailure", "(Ljava/lang/Object;)V")
}

// endSuspend ends the method of a suspend function: a state machine's
// dispatch, and its continuation class.
func (g *gen) endSuspend() {
	sm := g.sm
	g.sm = nil
	if sm == nil || sm.cls == "" {
		return
	}
	c := g.code
	ise := cf.NewLabel()
	c.Place(sm.dispatch)
	if len(sm.resumes) > 0 {
		c.Local(cf.ALOAD, sm.cont)
		c.FieldInsn(cf.GETFIELD, sm.cls, "label", "I")
		c.TableSwitch(1, ise, sm.resumes)
	} else {
		c.Jump(cf.GOTO, ise)
	}
	c.Place(ise)
	c.New("java/lang/IllegalStateException")
	c.Op(cf.DUP)
	c.Sconst("call to 'resume' before 'invoke' with coroutine")
	c.Invoke(cf.INVOKESPECIAL, "java/lang/IllegalStateException", "<init>", "(Ljava/lang/String;)V")
	c.Op(cf.ATHROW)
	g.check()
	for _, p := range sm.saved {
		sm.nextLines = append(sm.nextLines, c.NextLine(p.pos, p.line))
	}
	// A reference live after its point isn't nulled out: in a catch
	// block too, which the result being a failure goes to.
	for _, p := range sm.saved {
		for _, n := range p.nullOuts {
			if c.LiveAt(p.resumed, n.slot) {
				c.Patch(n.pos, 3)
			}
		}
	}
	if !sm.lambda {
		g.continuationClass(sm)
	}
}

// continuationClass generates the continuation class of a state machine:
// its fields, a constructor, and invokeSuspend, which resumes the
// function, calling it with this as its continuation.
func (g *gen) continuationClass(sm *stateMachine) {
	owner := g.cls.Name
	access := uint16(0)
	if sm.public {
		access = cf.AccPublic
	}
	c := cf.NewClass(access|cf.AccFinal|cf.AccSuper, sm.cls, continuationImpl)
	for _, f := range sm.fields {
		c.AddField(0, f, fieldDesc(f))
	}
	c.AddField(cf.AccSynthetic, "result", objectDesc)
	if sm.this {
		c.AddField(cf.AccFinal|cf.AccSynthetic, "this$0", "L"+owner+";")
	}
	c.AddField(0, "label", "I")
	g.debugMetadata(c, sm, sm.name, owner)

	code := c.NewMethod(access, "<init>", sm.ctorDesc(owner))
	code.AllocLocal("L" + sm.cls + ";")
	if sm.this {
		code.Local(cf.ALOAD, 0)
		code.Local(cf.ALOAD, code.AllocLocal("L"+owner+";"))
		code.FieldInsn(cf.PUTFIELD, sm.cls, "this$0", "L"+owner+";")
	}
	k := code.AllocLocal(types.ContinuationDesc)
	code.NameLocal(k, "$completion")
	code.Local(cf.ALOAD, 0)
	code.Local(cf.ALOAD, k)
	code.Invoke(cf.INVOKESPECIAL, continuationImpl, "<init>", "("+types.ContinuationDesc+")V")
	code.Op(cf.RETURN)
	if err := code.Err(); err != nil {
		panic(err)
	}

	saved := *g
	g.cls, g.owner, g.code = c, nil, c.NewMethod(cf.AccPublic|cf.AccFinal, "invokeSuspend", "(Ljava/lang/Object;)Ljava/lang/Object;")
	code = g.code
	code.AllocLocal("L" + sm.cls + ";")
	r := code.AllocLocal(objectDesc)
	code.NameLocal(r, "$result")
	code.Local(cf.ALOAD, 0)
	code.Local(cf.ALOAD, r)
	code.FieldInsn(cf.PUTFIELD, sm.cls, "result", objectDesc)
	code.Local(cf.ALOAD, 0)
	code.Op(cf.DUP)
	code.FieldInsn(cf.GETFIELD, sm.cls, "label", "I")
	code.Iconst(math.MinInt32)
	code.Op(cf.IOR)
	code.FieldInsn(cf.PUTFIELD, sm.cls, "label", "I")
	if sm.this {
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, sm.cls, "this$0", "L"+owner+";")
	}
	ps := descParams(sm.desc)
	ps = ps[:len(ps)-1]
	if sm.thisParam {
		ps = ps[1:]
	}
	for _, d := range ps {
		g.zero(d)
	}
	code.Local(cf.ALOAD, 0)
	code.Checkcast(continuationName)
	sm.call(g)
	if rawSuspendRet(sm.ret) {
		// The function returns the value unboxed: the continuation it
		// resumes gets it boxed.
		done := cf.NewLabel()
		code.Op(cf.DUP)
		code.Invoke(cf.INVOKESTATIC, "kotlin/coroutines/intrinsics/IntrinsicsKt", "getCOROUTINE_SUSPENDED", "()"+objectDesc)
		code.Jump(cf.IF_ACMPNE, done)
		code.Op(cf.ARETURN)
		code.Place(done)
		g.fromDirect(sm.ret)
		g.boxValue(sm.ret)
	}
	code.Op(cf.ARETURN)
	g.check()
	*g = saved

	g.localClass(c, g.cls, sm.name, sm.desc, sm.line, g.inlineXI(kmeta.WriteLocalSynthetic(), kmeta.XI|kmeta.XIPublicABI|kmeta.XILocal))
	c.InnerClasses[len(c.InnerClasses)-1].Access |= access
	g.cls.InnerClasses[len(g.cls.InnerClasses)-1].Access |= access
}

// localClass adds a class of local code that a method of class owner
// creates, such as a continuation class: its metadata, and the
// attributes that say where it is.
func (g *gen) localClass(c *cf.Class, owner *cf.Class, method, desc string, line int, meta kmeta.Header) {
	e := cf.InnerClassEntry{Inner: c.Name, Access: cf.AccStatic | cf.AccFinal}
	c.InnerClasses = append(c.InnerClasses, e)
	owner.InnerClasses = append(owner.InnerClasses, e)
	c.Enclosing = &cf.EnclosingMethod{Class: owner.Name, Name: method, Desc: desc}
	c.SourceFile = filepath.Base(g.file.Path)
	c.Line = line
	g.annotateMeta(c, meta)
	g.shared.classes[c.Name] = c
	g.shared.later = append(g.shared.later, c.Name)
}

// currentContinuation pushes the continuation of the suspend code being
// generated, as a Continuation.
func (g *gen) currentContinuation() {
	sm := g.sm
	if sm == nil {
		panic("coroutineContext outside suspend code")
	}
	if sm.cls == "" {
		g.code.Local(cf.ALOAD, sm.completion)
		return
	}
	g.code.Local(cf.ALOAD, sm.cont)
}

// coroutineContext reads kotlin.coroutines.coroutineContext: the context
// of the continuation.
func (g *gen) coroutineContext() {
	g.currentContinuation()
	g.code.InvokeInterface(continuationName, "getContext", "()Lkotlin/coroutines/CoroutineContext;")
}

// suspendMain generates the JVM entry point of suspend fun main(), as
// kotlinc does: it runs main with runSuspend, passing it a lambda that
// calls main, a singleton unless it passes main the args.
func (g *gen) suspendMain(fd *syntax.FunDecl) {
	facade := g.cls
	name := fd.Entry
	self := "L" + name + ";"
	mainDesc := funDesc(fd.Sym)
	const argsDesc = "[Ljava/lang/String;"
	args := len(fd.Params) == 1
	c := cf.NewClass(cf.AccFinal|cf.AccSuper|cf.AccSynthetic, name, "kotlin/jvm/internal/Lambda")
	c.Interfaces = []string{"kotlin/jvm/functions/Function1"}
	if args {
		c.AddField(cf.AccPrivate|cf.AccSynthetic, "args", argsDesc)
	} else {
		c.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal, "INSTANCE", self)
	}
	finish := func(code *cf.Code) {
		if err := code.Err(); err != nil {
			panic(err)
		}
	}

	code := c.NewMethod(cf.AccPublic|cf.AccFinal, "invoke", "("+types.ContinuationDesc+")"+objectDesc)
	code.AllocLocal(self)
	k := code.AllocLocal(types.ContinuationDesc)
	code.NameLocal(k, "$completion")
	if args {
		code.Local(cf.ALOAD, 0)
		code.FieldInsn(cf.GETFIELD, name, "args", argsDesc)
	}
	code.Local(cf.ALOAD, k)
	code.Invoke(cf.INVOKESTATIC, facade.Name, "main", mainDesc)
	done := cf.NewLabel()
	code.Op(cf.DUP)
	code.Invoke(cf.INVOKESTATIC, "kotlin/coroutines/intrinsics/IntrinsicsKt", "getCOROUTINE_SUSPENDED", "()"+objectDesc)
	code.Jump(cf.IF_ACMPNE, done)
	code.Op(cf.ARETURN)
	code.Place(done)
	code.Op(cf.POP)
	code.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
	code.Op(cf.ARETURN)
	finish(code)

	if args {
		code = c.NewMethod(0, "<init>", "("+argsDesc+")V")
		code.AllocLocal(self)
		a := code.AllocLocal(argsDesc)
		code.NameLocal(a, "args")
		code.Local(cf.ALOAD, a)
		code.Sconst("args")
		code.Invoke(cf.INVOKESTATIC, intrinsics, "checkNotNullParameter", "(Ljava/lang/Object;Ljava/lang/String;)V")
	} else {
		code = c.NewMethod(0, "<init>", "()V")
		code.AllocLocal(self)
	}
	code.Local(cf.ALOAD, 0)
	code.Iconst(1)
	code.Invoke(cf.INVOKESPECIAL, "kotlin/jvm/internal/Lambda", "<init>", "(I)V")
	if args {
		code.Local(cf.ALOAD, 0)
		code.Local(cf.ALOAD, 1)
		code.FieldInsn(cf.PUTFIELD, name, "args", argsDesc)
	}
	code.Op(cf.RETURN)
	finish(code)

	code = c.NewMethod(cf.AccPublic|cf.AccBridge|cf.AccSynthetic, "invoke", "("+objectDesc+")"+objectDesc)
	code.AllocLocal(self)
	code.Local(cf.ALOAD, 0)
	code.Local(cf.ALOAD, code.AllocLocal(objectDesc))
	code.Checkcast(continuationName)
	code.Invoke(cf.INVOKEVIRTUAL, name, "invoke", "("+types.ContinuationDesc+")"+objectDesc)
	code.Op(cf.ARETURN)
	finish(code)

	if !args {
		code = c.NewMethod(cf.AccStatic, "<clinit>", "()V")
		code.New(name)
		code.Op(cf.DUP)
		code.Invoke(cf.INVOKESPECIAL, name, "<init>", "()V")
		code.FieldInsn(cf.PUTSTATIC, name, "INSTANCE", self)
		code.Op(cf.RETURN)
		finish(code)
	}
	g.localClass(c, facade, "main", mainDesc, fd.P.Line, kmeta.WriteLocalSynthetic())

	g.code = facade.NewMethod(cf.AccPublic|cf.AccStatic|cf.AccSynthetic, "main", "("+argsDesc+")V")
	a := g.code.AllocLocal(argsDesc)
	g.code.NameLocal(a, "args")
	if args {
		g.code.New(name)
		g.code.Op(cf.DUP)
		g.code.Local(cf.ALOAD, a)
		g.code.Invoke(cf.INVOKESPECIAL, name, "<init>", "("+argsDesc+")V")
	} else {
		g.code.FieldInsn(cf.GETSTATIC, name, "INSTANCE", self)
	}
	g.code.Checkcast("kotlin/jvm/functions/Function1")
	g.code.Invoke(cf.INVOKESTATIC, "kotlin/coroutines/jvm/internal/RunSuspendKt", "runSuspend", "(Lkotlin/jvm/functions/Function1;)V")
	g.code.Op(cf.RETURN)
	g.check()
}

// debugMetadata annotates the class of a state machine's continuation
// with @DebugMetadata, as kotlinc does, which stack traces and debuggers
// read: the source file, the lines of the suspension points and of the
// code after them, the named variables each saves, by field, and the
// method and class.
func (g *gen) debugMetadata(c *cf.Class, sm *stateMachine, method, class string) {
	ints := func(vs []int) cf.ElementValue {
		ev := cf.ElementValue{Tag: '['}
		for _, v := range vs {
			ev.Elems = append(ev.Elems, cf.ElementValue{Tag: 'I', Const: int64(v)})
		}
		return ev
	}
	strs := func(ss []string) cf.ElementValue {
		ev := cf.ElementValue{Tag: '['}
		for _, s := range ss {
			ev.Elems = append(ev.Elems, cf.ElementValue{Tag: 's', Const: s})
		}
		return ev
	}
	str := func(s string) cf.ElementValue { return cf.ElementValue{Tag: 's', Const: s} }
	var lines, labels []int
	var fields, names []string
	for k, p := range sm.saved {
		lines = append(lines, p.line)
		for _, s := range p.saved {
			if s.name != "" {
				labels = append(labels, k)
				fields = append(fields, s.field)
				names = append(names, s.name)
			}
		}
	}
	c.Annotate(cf.Annotation{Type: "Lkotlin/coroutines/jvm/internal/DebugMetadata;", Values: []cf.ElementPair{
		{Name: "f", Value: str(filepath.Base(g.file.Path))},
		{Name: "l", Value: ints(lines)},
		{Name: "nl", Value: ints(sm.nextLines)},
		{Name: "i", Value: ints(labels)},
		{Name: "s", Value: strs(fields)},
		{Name: "n", Value: strs(names)},
		{Name: "m", Value: str(method)},
		{Name: "c", Value: str(strings.ReplaceAll(class, "/", "."))},
		{Name: "v", Value: cf.ElementValue{Tag: 'I', Const: int64(2)}},
	}}, true)
}

// fieldDesc returns the descriptor of a spill field, by its kind: L$n
// fields are Objects.
func fieldDesc(field string) string {
	if field[0] == 'L' {
		return objectDesc
	}
	return field[:1]
}

// fieldKind returns the kind of spill field that holds a local of type
// descriptor d: L for references, and the primitive's descriptor for
// primitives.
func fieldKind(d string) byte {
	switch d[0] {
	case 'L', '[':
		return 'L'
	}
	return d[0]
}

// Suspension points.

// suspendStart starts a call of suspend function f, if it is a
// suspension point of the function being generated: in a state machine,
// or a body for inlining, the values on the stack go to locals, as the
// code after the call starts from an empty stack when it resumes. It returns the point, which
// suspendEnd ends, or nil.
func (g *gen) suspendStart(x *syntax.Call, f *types.FunSym) *suspendPoint {
	if f == nil || !f.Suspend {
		return nil
	}
	sm := g.sm
	if sm == nil {
		panic("suspend call " + f.Name + " outside suspend code")
	}
	p := &suspendPoint{call: x, f: f}
	switch {
	case sm.forInline:
		// As kotlinc's: the inliner's state machine needs an empty stack
		// at the call too.
		if stack, ok := g.code.Spill(); ok {
			p.stack = stack
		}
	case sm.cls != "":
		stack, ok := g.code.Spill()
		if !ok {
			panic("a suspension point in the arguments of a constructor call")
		}
		p.stack = stack
	case x == nil || !sm.tails[x]:
		panic("suspension point " + f.Name + " is not a tail call")
	}
	sm.points = append(sm.points, p)
	return p
}

// suspendEnd ends a suspension point, after its call.
func (g *gen) suspendEnd(p *suspendPoint) {
	sm := g.sm
	sm.points = sm.points[:len(sm.points)-1]
	g.code.OnCall(nil) // in case the call was unreachable
}

// continuationArg passes the continuation, after the other arguments, if
// x is the call of the current suspension point.
func (g *gen) continuationArg(x *syntax.Call) {
	sm := g.sm
	if sm == nil || len(sm.points) == 0 {
		return
	}
	p := sm.points[len(sm.points)-1]
	if p.call != x || p.armed {
		return
	}
	if x.Sym.Inline != nil {
		// Copied, if its body can be: the suspension points are in the
		// body (see inlineSuspend).
		p.armed, p.inline = true, true
		g.currentContinuation()
		return
	}
	g.passContinuation(p)
	g.code.OnCall(func() { g.suspended(p) })
}

// passContinuation pushes the continuation, the last argument of the
// call of a suspension point. In a state machine, the locals are saved
// and the label set first.
func (g *gen) passContinuation(p *suspendPoint) {
	p.armed = true
	g.currentContinuation()
	if g.sm.forInline {
		g.mark(inline.BeforeSuspend)
	}
	g.saveLocals(p)
}

// continuationAlias records that a local holds the continuation.
func (g *gen) continuationAlias(slot int) {
	if sm := g.sm; sm != nil && sm.cls != "" {
		if sm.aliases == nil {
			sm.aliases = map[int]bool{}
		}
		sm.aliases[slot] = true
	}
}

// mark emits a call of kotlin.jvm.internal.InlineMarker.mark(kind).
func (g *gen) mark(kind int) {
	g.code.Iconst(int32(kind))
	g.code.Invoke(cf.INVOKESTATIC, "kotlin/jvm/internal/InlineMarker", "mark", "(I)V")
}

// saveLocals stores the locals that hold a value in the continuation's
// fields, and sets its label to the next resume point's, in a state
// machine.
func (g *gen) saveLocals(p *suspendPoint) {
	sm := g.sm
	c := g.code
	if sm.cls == "" {
		return
	}
	// As kotlinc does, the named variables in scope are saved, and the
	// temporaries that aren't dead: references first, then primitives,
	// each in the order of their locals. A variable is saved even if it
	// is dead, for debuggers, but a dead reference as null, unless the
	// debugger keeps it: see nullOuts.
	var refs, prims []savedLocal
	for _, slot := range c.Assigned() {
		if sm.aliases[slot] {
			p.aliases = append(p.aliases, slot)
		}
		if g.notSpilled(slot) {
			continue
		}
		d := c.LocalDesc(slot)
		name, inScope := c.Named(slot)
		if name != "" && !inScope || name == "" && c.Retired(slot) {
			// Its value isn't used again: it is zero when the code
			// resumes.
			p.zero = append(p.zero, savedLocal{slot: slot, desc: d})
			continue
		}
		s := savedLocal{slot: slot, desc: d, name: name}
		if fieldKind(d) == 'L' {
			refs = append(refs, s)
		} else {
			prims = append(prims, s)
		}
	}
	counts := map[byte]int{}
	for _, s := range append(refs, prims...) {
		k := fieldKind(s.desc)
		s.field = string(k) + "$" + strconv.Itoa(counts[k])
		counts[k]++
		sm.useField(s.field)
		c.Local(cf.ALOAD, sm.cont)
		c.Local(loadOpOf(s.desc), s.slot)
		if k == 'L' && s.name != "" && g.shared.opts.Spilling {
			p.nullOuts = append(p.nullOuts, savedLocal{slot: s.slot, pos: c.Pos()})
			c.Invoke(cf.INVOKESTATIC, "kotlin/coroutines/jvm/internal/SpillingKt", "nullOutSpilledVariable", "(Ljava/lang/Object;)Ljava/lang/Object;")
		}
		c.FieldInsn(cf.PUTFIELD, sm.cls, s.field, fieldDesc(s.field))
		p.saved = append(p.saved, s)
	}
	c.Local(cf.ALOAD, sm.cont)
	c.Iconst(int32(len(sm.resumes) + 1))
	c.FieldInsn(cf.PUTFIELD, sm.cls, "label", "I")
	p.line, p.pos = c.LastLine(), c.Pos()
	sm.saved = append(sm.saved, p)
}

// notSpilled reports whether a local is left out of the continuation's
// fields: this, and the state machine's own locals.
func (g *gen) notSpilled(slot int) bool {
	sm := g.sm
	switch slot {
	case sm.completion, sm.cont, sm.result, sm.susp:
		return true
	case 0:
		// A member's this, but name$suspendImpl's this is a parameter,
		// which kotlinc saves as $this.
		return sm.this && !sm.thisParam
	}
	if sm.skip[slot] || sm.aliases[slot] {
		return true
	}
	return g.code.LocalDesc(slot) == ""
}

// useField records that the continuation class has a spill field.
func (sm *stateMachine) useField(field string) {
	for _, f := range sm.fields {
		if f == field {
			return
		}
	}
	// Fields are grouped by kind, in the order the kinds are first used.
	i := len(sm.fields)
	for j := len(sm.fields) - 1; j >= 0; j-- {
		if sm.fields[j][0] == field[0] {
			i = j + 1
			break
		}
	}
	sm.fields = append(sm.fields[:i], append([]string{field}, sm.fields[i:]...)...)
}

// suspended follows the call of a suspension point, whose result is on
// the stack. In tail form, the function returns it. In a state machine,
// it returns it if it is COROUTINE_SUSPENDED; otherwise, or at the resume
// point, it converts it to the call's JVM result, the values on the stack
// before the call below it.
func (g *gen) suspended(p *suspendPoint) {
	sm := g.sm
	c := g.code
	switch {
	case sm.forInline:
		g.mark(inline.AfterSuspend)
		g.suspendFinish(p)
		return
	case sm.cls != "":
		g.suspendCheck(p)
		g.suspendFinish(p)
		return
	}
	if !sm.unit {
		c.Op(cf.ARETURN)
		return
	}
	// A Unit function returns Unit, which the call returns unless it
	// suspended.
	done := cf.NewLabel()
	c.Op(cf.DUP)
	c.Invoke(cf.INVOKESTATIC, "kotlin/coroutines/intrinsics/IntrinsicsKt", "getCOROUTINE_SUSPENDED", "()"+objectDesc)
	c.Jump(cf.IF_ACMPNE, done)
	c.Op(cf.ARETURN)
	c.Place(done)
	g.fromDirect(types.JVMRet(p.f))
}

// suspendCheck follows the call of a suspension point of a state machine,
// whose result is on the stack: it returns it if it is
// COROUTINE_SUSPENDED. The resume point follows, where the result is the
// one the continuation was resumed with.
func (g *gen) suspendCheck(p *suspendPoint) {
	sm := g.sm
	c := g.code
	join := cf.NewLabel()
	raw := p.f != nil && rawSuspendRet(types.JVMRet(p.f))
	if raw {
		join = cf.NewJoin(objectDesc) // the unboxed value, of its underlying type
	}
	start := c.Pos()
	c.Op(cf.DUP)
	c.Local(cf.ALOAD, sm.susp)
	c.Jump(cf.IF_ACMPNE, join)
	c.Local(cf.ALOAD, sm.susp)
	c.Op(cf.ARETURN)
	resume := c.NewFrameLabel(nil, sm.entry)
	sm.resumes = append(sm.resumes, resume)
	c.Place(resume)
	for i := len(p.saved) - 1; i >= 0; i-- {
		s := p.saved[i]
		c.Local(cf.ALOAD, sm.cont)
		c.FieldInsn(cf.GETFIELD, sm.cls, s.field, fieldDesc(s.field))
		if s.field[0] == 'L' && s.desc != objectDesc {
			c.Checkcast(className(s.desc))
		}
		c.Local(storeOpOf(s.desc), s.slot)
	}
	for _, z := range p.zero {
		g.zero(z.desc)
		c.Local(storeOpOf(z.desc), z.slot)
	}
	for _, slot := range p.aliases {
		c.Local(cf.ALOAD, sm.cont)
		c.Local(cf.ASTORE, slot)
	}
	// The handlers of the trys around the point don't protect the code
	// that returns or restores the locals, which hold no values yet.
	gap := [2]int{start, c.Pos()}
	for _, t := range g.tries {
		t.gaps = append(t.gaps, gap)
	}
	p.resumed = c.Pos()
	g.throwOnFailure(sm.result)
	c.Local(cf.ALOAD, sm.result)
	if raw {
		// Resumed with the value boxed; the call returns it unboxed.
		c.Checkcast(p.f.Ret.C.Name)
		g.unboxValue(p.f.Ret)
	}
	c.Place(join)
	p.join = c.Pos()
}

// suspendFinish puts the values that were on the stack before the call of
// a suspension point back below its result, and converts the result.
func (g *gen) suspendFinish(p *suspendPoint) {
	c := g.code
	if len(p.stack) > 0 && !p.late {
		tmp := c.AllocLocal(objectDesc)
		c.Local(cf.ASTORE, tmp)
		c.Unspill(p.stack)
		c.Local(cf.ALOAD, tmp)
		c.Retire(tmp)
	}
	g.fromDirect(types.JVMRet(p.f))
}

// rawSuspendRet reports whether a suspend function whose result has type
// t returns it unboxed, as kotlinc compiles it: a value class over a
// reference type. Its continuation boxes it for the caller it resumes,
// whose resume point unboxes it. One over a primitive is always boxed.
func rawSuspendRet(t types.Type) bool {
	return types.Unboxed(t) && !t.N && !types.Repr(t).IsPrimitive()
}

// fromDirect converts the Object that a call of a suspend function with
// result type t returns, when it doesn't suspend, to its value on the
// JVM.
func (g *gen) fromDirect(t types.Type) {
	if !rawSuspendRet(t) {
		g.fromObject(t)
		return
	}
	if d := desc(types.Repr(t)); d != objectDesc {
		g.code.Checkcast(className(d))
	}
}

// fromObject converts the Object result of a suspend function, of type t,
// to the value a function of type t returns on the JVM.
func (g *gen) fromObject(t types.Type) {
	c := g.code
	switch {
	case t.K == types.Unit:
		c.Op(cf.POP)
	case t.K == types.Nothing && !t.N:
		c.Checkcast("java/lang/Void")
	case types.Unboxed(t):
		c.Checkcast(t.C.Name)
		g.unboxValue(t)
	case t.IsPrimitive():
		switch t.K {
		case types.Bool:
			c.Checkcast("java/lang/Boolean")
			c.Invoke(cf.INVOKEVIRTUAL, "java/lang/Boolean", "booleanValue", "()Z")
		case types.Char:
			c.Checkcast("java/lang/Character")
			c.Invoke(cf.INVOKEVIRTUAL, "java/lang/Character", "charValue", "()C")
		default:
			d := desc(t)
			c.Checkcast("java/lang/Number")
			c.Invoke(cf.INVOKEVIRTUAL, "java/lang/Number", numberValue[d], "()"+d)
		}
	default:
		if d := desc(t); d != objectDesc {
			c.Checkcast(className(d))
		}
	}
}

// numberValue are the methods of Number that return its value as a
// primitive, by the primitive's descriptor.
var numberValue = map[string]string{
	"I": "intValue", "J": "longValue", "F": "floatValue", "D": "doubleValue", "B": "byteValue", "S": "shortValue",
}

// boxing are the methods of kotlin.coroutines.jvm.internal.Boxing that box
// a suspend function's primitive result, by its descriptor.
var boxing = map[string]string{
	"I": "boxInt", "J": "boxLong", "F": "boxFloat", "D": "boxDouble", "Z": "boxBoolean", "C": "boxChar", "B": "boxByte", "S": "boxShort",
}

// ret returns the value of type t on the stack from the method being
// generated: a suspend function's as an Object.
func (g *gen) ret(t types.Type) {
	if c := g.compose; c != nil && g.lam == nil {
		g.endGroups(0)
		if c.end != nil {
			// To the end of the restart group: nothing else is on the stack.
			g.code.PopTo(0)
			g.code.Jump(cf.GOTO, c.end)
			return
		}
	}
	if g.sm == nil {
		g.code.Op(returnOp(t))
		return
	}
	c := g.code
	switch {
	case t.K == types.Unit || t.K == types.Nothing:
		c.FieldInsn(cf.GETSTATIC, "kotlin/Unit", "INSTANCE", "Lkotlin/Unit;")
	case rawSuspendRet(t) && !g.sm.lambda:
		// Unboxed, as kotlinc returns it.
	case types.Unboxed(t):
		g.boxValue(t)
	case t.IsPrimitive():
		d := desc(t)
		c.Invoke(cf.INVOKESTATIC, "kotlin/coroutines/jvm/internal/Boxing", boxing[d], "("+d+")L"+types.Boxes[t.K]+";")
	}
	c.Op(cf.ARETURN)
}

// suspendIntrinsic generates suspendCoroutineUninterceptedOrReturn { c ->
// ... }: a suspension point whose call is the lambda, inlined, with the
// continuation as its argument.
func (g *gen) suspendIntrinsic(x *syntax.Call) {
	if neverSuspends(x) && g.sm != nil && !g.sm.forInline {
		// Not a suspension point: nothing is saved.
		g.currentContinuation()
		g.inlineLambda(x.Args[0].(*syntax.Lambda))
		return
	}
	if g.sm != nil && g.sm.cls == "" && !g.sm.forInline && !g.sm.tails[x] {
		panic("suspendCoroutineUninterceptedOrReturn outside a state machine")
	}
	p := g.suspendStart(x, x.Sym)
	g.passContinuation(p)
	g.inlineLambda(x.Args[0].(*syntax.Lambda))
	if g.code.Reachable() {
		g.suspended(p)
	}
	g.suspendEnd(p)
	if g.code.Reachable() {
		g.coerce(types.JVMRet(x.Sym), x.T) // its T, an Object
	}
}

// suspendOperator calls an operator function f without parameters, such
// as a for loop's iterator(), hasNext() and next(), on the receiver that
// load pushes, of type rt.
func (g *gen) suspendOperator(f *types.FunSym, rt types.Type, load func()) {
	p := g.suspendStart(nil, f)
	if p == nil {
		load()
		g.invokeOperator(f, rt)
		return
	}
	load()
	if f.Ext {
		g.coerce(rt, f.Recv)
	}
	g.passContinuation(p)
	g.code.OnCall(func() { g.suspended(p) })
	if f.Ext {
		g.invokeStatic(f.Facade, f, 0, false)
	} else {
		g.invokeOperatorMember(f, rt)
	}
	g.suspendEnd(p)
}

// The tail form.

// scanTail reports whether suspend function fd is in tail form: each of
// its suspension points is a call whose result it returns, which isn't in
// a try. It returns those calls.
//
// As kotlinc, which looks at the code after inlining, it looks into the
// copies of the program's inline functions whose results it returns: a
// suspension point in tail position of a copy's body is in tail form, and
// so is one in tail position of an inlined lambda the copy invokes in tail
// position, as receiveImpl { ... receiveOnNoWaiterSuspend() } is. The
// body of fd is lambda l's if it is a fun interface's method that l
// implements: its value, and its returns, are the function's.
func scanTail(fd *syntax.FunDecl, l *syntax.Lambda) (bool, map[*syntax.Call]bool) {
	s := &tailScan{ret: fd.Sym.Ret, tails: map[*syntax.Call]bool{}, invokes: map[*syntax.Call]bool{}, walked: map[*syntax.Lambda]bool{}, tail: true}
	if l != nil {
		s.tailBlock(fd.Body, &tailCtx{}, s.ret)
		s.walk(fd.Body, &tailCtx{}, true, l)
	} else {
		s.body(fd, &tailCtx{})
	}
	return s.tail, s.tails
}

// tailScan scans a suspend function, of result type ret, and the copies of
// inline functions in tail position in it.
type tailScan struct {
	ret     types.Type
	tails   map[*syntax.Call]bool // the suspension points and copies in tail position
	invokes map[*syntax.Call]bool // the invokes of inlined lambdas in tail position
	walked  map[*syntax.Lambda]bool
	tail    bool
}

// tailCtx is the code being scanned: the function's, or a copy's, whose
// lambda parameters are inlined lambdas.
type tailCtx struct {
	lambdas map[*types.VarSym]inlinedLambda
}

// inlinedLambda is a lambda passed to an inline function, and the code it
// is in.
type inlinedLambda struct {
	l   *syntax.Lambda
	ctx *tailCtx
}

// body scans the body of fd in tail position: the function's, or a
// copy's.
func (s *tailScan) body(fd *syntax.FunDecl, ctx *tailCtx) {
	if fd.ExprBody != nil {
		s.tailExpr(fd.ExprBody, ctx, s.ret)
		s.walk(fd.ExprBody, ctx, true, nil)
		return
	}
	if fd.Sym.Ret.K == types.Unit && s.ret.K == types.Unit {
		// The last statement of a Unit function, a call of a Unit
		// function.
		s.tailBlock(fd.Body, ctx, s.ret)
	}
	s.walk(fd.Body, ctx, true, nil)
}

func (s *tailScan) tailBlock(b *syntax.Block, ctx *tailCtx, ret types.Type) {
	if b == nil || len(b.Stmts) == 0 {
		return
	}
	if es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt); ok {
		s.tailExpr(es.X, ctx, ret)
	}
}

// tailExpr records the calls whose result the function returns as x's:
// the suspension points, the copies, and the invokes of inlined lambdas.
func (s *tailScan) tailExpr(x syntax.Expr, ctx *tailCtx, ret types.Type) {
	switch x := x.(type) {
	case *syntax.Call:
		if _, ok := ctx.invoked(x); ok {
			s.invokes[x] = true
			break
		}
		if x.Sym == nil || !returnable(x, ret) {
			break
		}
		if d := syntax.SourceInline(x.Sym); d != nil {
			if suspensionPoint(x) || len(inlinedLambdas(x, d, ctx)) > 0 {
				s.copy(x, d, ctx)
			}
		} else if suspensionPoint(x) && tailInline(x) {
			s.tails[x] = true
		}
	case *syntax.If:
		s.tailBlock(x.Then, ctx, ret)
		s.tailBlock(x.Else, ctx, ret)
	case *syntax.When:
		for _, b := range x.Branches {
			s.tailBlock(b.Body, ctx, ret)
		}
	case *syntax.As:
		if noopCast(x) {
			s.tailExpr(x.X, ctx, ret) // await() as T
		}
	}
}

// copy scans the copy of inline function d for call x in tail position.
// The call is in tail form if the copy is.
func (s *tailScan) copy(x *syntax.Call, d *syntax.FunDecl, ctx *tailCtx) {
	ls := inlinedLambdas(x, d, ctx)
	// The results returned in the copy are its own, which x's is.
	sub := &tailScan{ret: d.Sym.Ret, tails: map[*syntax.Call]bool{}, invokes: map[*syntax.Call]bool{}, walked: s.walked, tail: true}
	sub.body(d, &tailCtx{lambdas: ls})
	for _, il := range ls {
		// One the copy doesn't invoke, but passes on to code that isn't
		// scanned, is walked with the call.
		sub.tail = sub.tail && s.walked[il.l]
	}
	if !sub.tail || len(sub.tails) == 0 {
		return
	}
	s.tails[x] = true
	maps.Copy(s.tails, sub.tails)
}

// inlinedLambdas returns the inlined lambdas that call x of inline
// function d passes, by d's parameters: its lambda arguments, and the
// lambdas that the code being scanned forwards.
func inlinedLambdas(x *syntax.Call, d *syntax.FunDecl, ctx *tailCtx) map[*types.VarSym]inlinedLambda {
	ls := map[*types.VarSym]inlinedLambda{}
	for i, p := range d.Sym.Params {
		if l := inlinedArg(x, i); l != nil {
			ls[p] = inlinedLambda{l, ctx}
		}
	}
	for k, a := range x.Args {
		if id, ok := a.(*syntax.Ident); ok && id.Sym != nil {
			if il, ok := ctx.lambdas[id.Sym]; ok {
				i := k
				if x.ArgMap != nil {
					i = x.ArgMap[k]
				}
				ls[d.Sym.Params[i]] = il
			}
		}
	}
	return ls
}

// invoked returns the inlined lambda that x invokes, if it is an invoke of
// one of the copy's lambda parameters.
func (ctx *tailCtx) invoked(x *syntax.Call) (inlinedLambda, bool) {
	if id, ok := x.Fun.(*syntax.Ident); ok && x.Invoke && id.Sym != nil {
		il, ok := ctx.lambdas[id.Sym]
		return il, ok
	}
	return inlinedLambda{}, false
}

// walk finds the suspension points of n, the code of ctx, that aren't in
// tail form. Outside trys, plain is set, and returns are the function's,
// or those from lambda l, inlined in tail position.
func (s *tailScan) walk(n any, ctx *tailCtx, plain bool, l *syntax.Lambda) {
	if n == nil {
		return
	}
	syntax.Inspect(n, func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			if n.Inline {
				s.walk(n.Body, ctx, false, nil)
			}
			return false
		case *syntax.LocalFun:
			return false
		case *syntax.Try:
			s.walk(n.Body, ctx, false, nil)
			for _, c := range n.Catches {
				s.walk(c.Body, ctx, false, nil)
			}
			s.walk(n.Finally, ctx, false, nil)
			return false
		case *syntax.Jump:
			if plain && n.Kind == syntax.RETURN && (n.Lambda == nil || n.Lambda == l) && n.X != nil {
				ret := s.ret
				if n.Lambda != nil && n.Lambda.Ret.K == types.Unit {
					ret = types.UnitType
				}
				s.tailExpr(n.X, ctx, ret)
			}
		case *syntax.Call:
			if il, ok := ctx.invoked(n); ok {
				for _, a := range n.Args {
					s.walk(a, ctx, plain, l)
				}
				s.lambda(il, plain && s.invokes[n])
				return false
			}
			if suspends(n) && !(plain && s.tails[n]) {
				s.tail = false
			}
			if plain && s.tails[n] && syntax.SourceInline(n.Sym) != nil {
				// A copy whose lambdas were scanned with it.
				s.walkArgs(n, ctx, l)
				return false
			}
		case *syntax.For:
			for _, f := range []*types.FunSym{n.Iter, n.HasNext, n.Next} {
				if f != nil && f.Suspend {
					s.tail = false
				}
			}
		case *syntax.Binary:
			s.walkCall(n.Call, ctx, plain, l)
		case *syntax.Index:
			s.walkCall(n.Call, ctx, plain, l)
		case *syntax.Unary:
			s.walkCall(n.Call, ctx, plain, l)
		}
		return true
	})
}

func (s *tailScan) walkCall(x *syntax.Call, ctx *tailCtx, plain bool, l *syntax.Lambda) {
	if x != nil {
		s.walk(x, ctx, plain, l)
	}
}

// walkArgs walks the receiver and arguments of call x in tail form, but
// for its inlined lambdas, which the scan of its copy has.
func (s *tailScan) walkArgs(x *syntax.Call, ctx *tailCtx, l *syntax.Lambda) {
	if m, ok := x.Fun.(*syntax.Member); ok {
		s.walk(m.X, ctx, true, l)
	}
	for _, a := range x.Args {
		if il, ok := a.(*syntax.Lambda); ok && il.Inline {
			continue
		}
		s.walk(a, ctx, true, l)
	}
}

// lambda walks the body of an inlined lambda, in the code it is in, where
// a copy invokes it: in tail position if tail is set.
func (s *tailScan) lambda(il inlinedLambda, tail bool) {
	l := il.l
	s.walked[l] = true
	if !tail {
		s.walk(l.Body, il.ctx, false, nil)
		return
	}
	ret := s.ret
	if l.Ret.K == types.Unit {
		ret = types.UnitType // its value, Unit, is the copy's
	}
	s.tailBlock(l.Body, il.ctx, ret)
	s.walk(l.Body, il.ctx, true, l)
}

// tailInline reports whether a call can be in tail position if it is of
// an inline suspend function of the class path: its copied body returns
// its suspension points' results.
func tailInline(x *syntax.Call) bool {
	if x.Sym.Inline == nil {
		return true
	}
	for i := range x.ArgMap {
		if x.ArgMap[i] != i {
			return false
		}
	}
	b := inlineBody(x.Sym, 0)
	return b != nil && len(x.Args) == len(x.Sym.Params) && b.ReturnsSuspended()
}

// noopCast reports whether cast x generates no code: an unchecked cast of
// a reference to a type parameter that erases to Object.
func noopCast(x *syntax.As) bool {
	t, from := x.Target, x.X.Type()
	if x.Safe || !t.IsParam() || t.Param().Reified || types.Erase(t) != types.Erase(types.AnyType.Nullable()) {
		return false
	}
	return !from.IsPrimitive() && from.K != types.Unit && !types.IsValue(from)
}

// suspends reports whether call x is a suspension point after inlining,
// as kotlinc sees it: not suspendCoroutineUninterceptedOrReturn if its
// lambda never returns, which leaves its suspension unreachable, or a copy
// of the program's inline function without other suspension points, as
// recoverAndThrow is.
func suspends(x *syntax.Call) bool {
	if !suspensionPoint(x) {
		return false
	}
	if neverSuspends(x) {
		return false
	}
	d := syntax.SourceInline(x.Sym)
	if d == nil {
		return true
	}
	found := false
	visit := func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			return n.Inline
		case *syntax.LocalFun:
			return false
		case *syntax.Call:
			found = found || suspends(n)
		case *syntax.For:
			found = found || n.HasNext != nil && (n.HasNext.Suspend || n.Next.Suspend || n.Iter != nil && n.Iter.Suspend)
		case *syntax.Binary:
			found = found || n.Call != nil && suspends(n.Call)
		case *syntax.Index:
			found = found || n.Call != nil && suspends(n.Call)
		case *syntax.Unary:
			found = found || n.Call != nil && suspends(n.Call)
		}
		return !found
	}
	syntax.Inspect(d.Body, visit)
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, visit)
	}
	return found
}

// neverSuspends reports whether x is a call of
// suspendCoroutineUninterceptedOrReturn whose lambda never returns.
func neverSuspends(x *syntax.Call) bool {
	if x.Builtin != types.SuspendIntrinsic || len(x.Args) != 1 {
		return false
	}
	l, ok := x.Args[0].(*syntax.Lambda)
	if !ok || l.Body == nil || len(l.Body.Stmts) == 0 {
		return false
	}
	es, ok := l.Body.Stmts[len(l.Body.Stmts)-1].(*syntax.ExprStmt)
	return ok && es.X.Type().K == types.Nothing && !es.X.Type().N
}

// suspensionPoint reports whether x is a call of a suspend function.
func suspensionPoint(x *syntax.Call) bool { return x.Sym != nil && x.Sym.Suspend }

// returnable reports whether a suspend function can return the result of
// call x unchanged, as its result of type ret: in a Unit function, x's is
// Unit too.
func returnable(x *syntax.Call, ret types.Type) bool {
	f := x.Sym
	switch {
	case x.T.K == types.Nothing && !x.T.N:
		return false // kotlinc throws after it
	case invokesCrossinline(x):
		// In an object of an inline function, where the lambda's copy
		// can suspend anywhere: kotlinc's method is a state machine.
		return false
	case ret.K == types.Unit:
		// Also a call of a generic function whose T is Unit, as
		// coroutineScope { ... }: the function returns Unit itself. Not
		// an inline one's, as suspendCancellableCoroutine's, whose copy
		// kotlinc gives a state machine.
		return f.Ret.K == types.Unit && !f.Ret.N || x.T.K == types.Unit && !x.T.N && f.Inline == nil && syntax.SourceInline(f) == nil
	case types.Unboxed(ret) || types.Unboxed(f.Ret) || types.Unboxed(x.T):
		return false
	}
	return true
}

// invokesCrossinline reports whether x invokes a crossinline parameter of
// an inline function.
func invokesCrossinline(x *syntax.Call) bool {
	id, ok := x.Fun.(*syntax.Ident)
	if !x.Invoke || !ok || id.Sym == nil {
		return false
	}
	p, ok := id.Sym.Decl.(*syntax.Param)
	return ok && p.Crossinline
}

// hasSuspension reports whether the expressions have a suspension point
// outside lambdas that aren't inlined.
func hasSuspension(xs []syntax.Expr) bool {
	found := false
	for _, x := range xs {
		syntax.Inspect(x, func(n any) bool {
			switch n := n.(type) {
			case *syntax.Lambda:
				return n.Inline
			case *syntax.LocalFun:
				return false
			case *syntax.Call:
				found = found || suspensionPoint(n)
			case *syntax.For:
				found = found || n.HasNext != nil && (n.HasNext.Suspend || n.Next.Suspend || n.Iter != nil && n.Iter.Suspend)
			case *syntax.Binary:
				found = found || n.Call != nil && hasSuspension([]syntax.Expr{n.Call})
			case *syntax.Index:
				found = found || n.Call != nil && hasSuspension([]syntax.Expr{n.Call})
			case *syntax.Unary:
				found = found || n.Call != nil && hasSuspension([]syntax.Expr{n.Call})
			}
			return !found
		})
	}
	return found
}
