package classfile

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// Opcodes used by the code generator.
const (
	ACONST_NULL     = 0x01
	ICONST_M1       = 0x02
	ICONST_0        = 0x03
	ICONST_5        = 0x08
	LCONST_0        = 0x09
	LCONST_1        = 0x0a
	FCONST_0        = 0x0b
	FCONST_1        = 0x0c
	FCONST_2        = 0x0d
	DCONST_0        = 0x0e
	DCONST_1        = 0x0f
	BIPUSH          = 0x10
	SIPUSH          = 0x11
	LDC             = 0x12
	LDC_W           = 0x13
	LDC2_W          = 0x14
	ILOAD           = 0x15
	LLOAD           = 0x16
	FLOAD           = 0x17
	DLOAD           = 0x18
	ALOAD           = 0x19
	IALOAD          = 0x2e
	LALOAD          = 0x2f
	FALOAD          = 0x30
	DALOAD          = 0x31
	AALOAD          = 0x32
	BALOAD          = 0x33
	CALOAD          = 0x34
	SALOAD          = 0x35
	ISTORE          = 0x36
	LSTORE          = 0x37
	FSTORE          = 0x38
	DSTORE          = 0x39
	ASTORE          = 0x3a
	IASTORE         = 0x4f
	LASTORE         = 0x50
	FASTORE         = 0x51
	DASTORE         = 0x52
	AASTORE         = 0x53
	BASTORE         = 0x54
	CASTORE         = 0x55
	SASTORE         = 0x56
	POP             = 0x57
	POP2            = 0x58
	DUP             = 0x59
	DUP_X1          = 0x5a
	DUP2            = 0x5c
	IADD            = 0x60
	LADD            = 0x61
	FADD            = 0x62
	DADD            = 0x63
	ISUB            = 0x64
	LSUB            = 0x65
	FSUB            = 0x66
	DSUB            = 0x67
	IMUL            = 0x68
	LMUL            = 0x69
	FMUL            = 0x6a
	DMUL            = 0x6b
	IDIV            = 0x6c
	LDIV            = 0x6d
	FDIV            = 0x6e
	DDIV            = 0x6f
	IREM            = 0x70
	LREM            = 0x71
	FREM            = 0x72
	DREM            = 0x73
	INEG            = 0x74
	LNEG            = 0x75
	FNEG            = 0x76
	DNEG            = 0x77
	IAND            = 0x7e
	IXOR            = 0x82
	IINC            = 0x84
	I2L             = 0x85
	I2F             = 0x86
	I2D             = 0x87
	L2I             = 0x88
	L2F             = 0x89
	L2D             = 0x8a
	F2I             = 0x8b
	F2L             = 0x8c
	F2D             = 0x8d
	D2I             = 0x8e
	D2L             = 0x8f
	D2F             = 0x90
	I2B             = 0x91
	I2C             = 0x92
	I2S             = 0x93
	LCMP            = 0x94
	FCMPL           = 0x95
	FCMPG           = 0x96
	DCMPL           = 0x97
	DCMPG           = 0x98
	IFEQ            = 0x99
	IFNE            = 0x9a
	IFLT            = 0x9b
	IFGE            = 0x9c
	IFGT            = 0x9d
	IFLE            = 0x9e
	IF_ICMPEQ       = 0x9f
	IF_ICMPNE       = 0xa0
	IF_ICMPLT       = 0xa1
	IF_ICMPGE       = 0xa2
	IF_ICMPGT       = 0xa3
	IF_ICMPLE       = 0xa4
	IF_ACMPEQ       = 0xa5
	IF_ACMPNE       = 0xa6
	GOTO            = 0xa7
	IRETURN         = 0xac
	LRETURN         = 0xad
	FRETURN         = 0xae
	DRETURN         = 0xaf
	ARETURN         = 0xb0
	RETURN          = 0xb1
	GETSTATIC       = 0xb2
	PUTSTATIC       = 0xb3
	GETFIELD        = 0xb4
	PUTFIELD        = 0xb5
	INVOKEVIRTUAL   = 0xb6
	INVOKESPECIAL   = 0xb7
	INVOKESTATIC    = 0xb8
	INVOKEINTERFACE = 0xb9
	INVOKEDYNAMIC   = 0xba
	NEW             = 0xbb
	NEWARRAY        = 0xbc
	ANEWARRAY       = 0xbd
	ARRAYLENGTH     = 0xbe
	ATHROW          = 0xbf
	CHECKCAST       = 0xc0
	INSTANCEOF      = 0xc1
	WIDE            = 0xc4
	IFNULL          = 0xc6
	IFNONNULL       = 0xc7
)

// Label is a branch target. It holds the state of the edges that reach
// it: the first edge's, merged with each later one.
type Label struct {
	pos    int // -1 until placed
	fixups []fixup
	frame  *state // nil until an edge reaches it
	top    *vtype // see NewJoin
	fixed  bool   // the frame is given: see NewFrameLabel
	// min is a fixed label's given locals, which every edge must cover.
	// With below set (see Below), its frame also has the locals under
	// below that every edge before its placement assigns: ext, once
	// reached.
	min     bitset
	below   int
	ext     bitset
	reached bool
	jumped  bool // a jump targets it, so it needs a frame in StackMapTable
}

// fixup is a branch offset to patch once its label is placed: at patch,
// relative to the branch instruction at at, in 2 bytes or, in a switch, 4.
type fixup struct {
	at, patch int
	wide      bool
}

func NewLabel() *Label { return &Label{pos: -1} }

// NewJoin returns a label where branches meet that each leave a value on
// the stack, of reference types that may differ. desc, the value's static
// type, is a supertype of them all, and the frame uses it.
func NewJoin(desc string) *Label {
	t := typeOf(desc)
	return &Label{pos: -1, top: &t}
}

// NewFrameLabel returns a label whose frame is given up front: the types
// on the operand stack, and the locals that hold a value, which have their
// declared types. It is for code copied from a class file, whose frames
// come from its StackMapTable. Edges to the label are checked against the
// frame instead of merged into it: their locals must cover its locals,
// and their stack values must have the same kinds (a reference type is
// taken to be a subtype of the frame's), but see Below. Placing the label
// makes the code after it reachable, even before any edge reaches it, as
// a loop tested at the bottom needs.
func (c *Code) NewFrameLabel(stack []VType, assigned []int) *Label {
	f := &state{stack: slices.Clone(stack), uninitThis: c.uninitThis}
	for _, s := range assigned {
		f.assigned.set(s)
	}
	return &Label{pos: -1, frame: f, fixed: true, min: slices.Clone(f.assigned)}
}

// Below lets the edges that reach a frame label before it is placed add
// the locals under slot n that each of them assigns to its frame: an
// inlined body's frames know the caller's locals that hold a value where
// it starts, but not those that an inlined lambda assigns, as ktor's
// var result: Int that the lambda passed to readFromHead does.
func (l *Label) Below(n int) { l.below = n }

// Code is a method body. It tracks the types on the operand stack and
// which locals hold a value, to compute max_stack and the StackMapTable
// frames. It drops instructions in unreachable code (after a jump, return
// or throw, until a label that is jumped to).
//
// Each local has a declared type, given when it is allocated, and frames
// use that type for every local that holds a value. A local is never
// reused for another type, so merging two paths never needs the class
// hierarchy.
type Code struct {
	pool       *Pool
	buf        []byte
	stack      []vtype // one entry per value
	depth      int     // the stack's size in slots
	max        int
	locals     []vtype // declared type of each slot; Top for the second slot of a Long or Double
	assigned   bitset
	maxLocals  int
	params     int     // slots taken by this and the parameters
	entry      []vtype // the locals of the method's implicit first frame
	class      string
	uninitThis bool // in a constructor, before the super constructor call
	// thisCopies are the locals that Spill stored this in before the super
	// constructor call: uninitializedThis in the frames there, as local 0.
	thisCopies []int
	reachable  bool
	placed     []*Label // labels reached by an edge, in placement order
	handlers   []handler
	frames     int // StackMapTable entries, set by encodeFrames
	smt        []byte
	err        error
	lines      []lineEntry
	vars       []localVar
	descs      []string // the type descriptor of each slot allocated by AllocLocal
	hasThis    bool     // an instance method, whose local 0 is named this
	name       string   // the method's name
	desc       string   // and descriptor
	afterCall  func()   // see OnCall
	// Liveness (see live.go): the trace of the code, the locals live at
	// each traced instruction, and the temporaries known to be dead.
	tracing bool
	traces  []traceEntry
	live    []bitset
	retired bitset
}

func newCode(pool *Pool, class string, access uint16, name, desc string) *Code {
	c := &Code{pool: pool, class: class, name: name, desc: desc, reachable: true, hasThis: access&AccStatic == 0}
	if access&AccStatic == 0 {
		t := object(class)
		if name == "<init>" {
			t, c.uninitThis = vtype{tag: tUninitThis}, true
		}
		c.locals = append(c.locals, t)
	}
	for _, t := range paramTypes(desc) {
		c.locals = append(c.locals, t)
		if t.size() == 2 {
			c.locals = append(c.locals, vtype{})
		}
	}
	c.params = len(c.locals)
	for i := range c.params {
		c.assigned.set(i)
	}
	c.entry = c.frameLocals(c.assigned, c.uninitThis)
	return c
}

func (c *Code) Reachable() bool { return c.reachable }

// Class returns the internal name of the method's class.
func (c *Code) Class() string { return c.class }

// Name returns the method's name.
func (c *Code) Name() string { return c.name }

// Desc returns the method's descriptor.
func (c *Code) Desc() string { return c.desc }

func (c *Code) fail(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf(format, args...)
	}
}

func (c *Code) push(t vtype) {
	c.stack = append(c.stack, t)
	c.depth += t.size()
	if c.depth > c.max {
		c.max = c.depth
	}
}

func (c *Code) pop() vtype {
	if len(c.stack) == 0 {
		c.fail("operand stack underflow at offset %d", len(c.buf))
		return vtype{}
	}
	t := c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	c.depth -= t.size()
	return t
}

func (c *Code) popN(n int) {
	for range n {
		c.pop()
	}
}

// AllocLocal reserves a local of the given type descriptor and returns its
// slot. The first calls must allocate this and the parameters, in order.
func (c *Code) AllocLocal(desc string) int {
	slot := c.maxLocals
	if desc == "V" {
		return slot // a Unit value takes no slot
	}
	t := typeOf(desc)
	n := t.size()
	for len(c.descs) < slot+n {
		c.descs = append(c.descs, "")
	}
	c.descs[slot] = desc
	switch {
	case slot >= c.params:
		c.locals = append(c.locals, t)
		if n == 2 {
			c.locals = append(c.locals, vtype{})
		}
	case slot+n > c.params || c.locals[slot].size() != n:
		c.fail("local %d (%s) does not match the parameters", slot, desc)
	}
	c.maxLocals += n
	return slot
}

// LocalDesc returns the type descriptor a local was allocated with.
func (c *Code) LocalDesc(slot int) string {
	if slot < len(c.descs) {
		return c.descs[slot]
	}
	return ""
}

// OnCall makes fn run once, right after the next call instruction is
// emitted: a suspend function's call is followed by the check of its
// result. fn nil cancels it.
func (c *Code) OnCall(fn func()) { c.afterCall = fn }

// Op emits a no-operand instruction.
func (c *Code) Op(op byte) {
	if !c.reachable {
		return
	}
	c.buf = append(c.buf, op)
	switch {
	case op == ACONST_NULL:
		c.push(nullType)
		return
	case ICONST_M1 <= op && op <= ICONST_5:
		c.push(intType)
		return
	case op == LCONST_0 || op == LCONST_1:
		c.push(longType)
		return
	case FCONST_0 <= op && op <= FCONST_2:
		c.push(floatType)
		return
	case op == DCONST_0 || op == DCONST_1:
		c.push(doubleType)
		return
	}
	switch op {
	case NOP:
	case IALOAD, BALOAD, CALOAD, SALOAD:
		c.popN(2)
		c.push(intType)
	case FALOAD:
		c.popN(2)
		c.push(floatType)
	case LALOAD:
		c.popN(2)
		c.push(longType)
	case DALOAD:
		c.popN(2)
		c.push(doubleType)
	case AALOAD:
		c.pop()
		arr := c.pop()
		if arr.tag == tObject && len(arr.name) > 1 && arr.name[0] == '[' {
			c.push(typeOf(arr.name[1:]))
		} else {
			c.push(nullType)
		}
	case IASTORE, LASTORE, FASTORE, DASTORE, AASTORE, BASTORE, CASTORE, SASTORE:
		c.popN(3)
	case POP:
		c.pop()
	case POP2:
		if c.pop().size() == 1 {
			c.pop()
		}
	case DUP:
		t := c.pop()
		c.push(t)
		c.push(t)
	case DUP_X1:
		v1, v2 := c.pop(), c.pop()
		c.push(v1)
		c.push(v2)
		c.push(v1)
	case DUP_X2:
		v1, v2 := c.pop(), c.pop()
		if v2.size() == 2 {
			c.pushAll(v1, v2, v1)
			break
		}
		v3 := c.pop()
		c.pushAll(v1, v3, v2, v1)
	case DUP2:
		v1 := c.pop()
		if v1.size() == 2 {
			c.push(v1)
			c.push(v1)
			break
		}
		v2 := c.pop()
		c.push(v2)
		c.push(v1)
		c.push(v2)
		c.push(v1)
	case DUP2_X1:
		v1, v2 := c.pop(), c.pop()
		if v1.size() == 2 {
			c.pushAll(v1, v2, v1)
			break
		}
		v3 := c.pop()
		c.pushAll(v2, v1, v3, v2, v1)
	case DUP2_X2:
		v1, v2 := c.pop(), c.pop()
		switch {
		case v1.size() == 2 && v2.size() == 2:
			c.pushAll(v1, v2, v1)
		case v1.size() == 2:
			v3 := c.pop()
			c.pushAll(v1, v3, v2, v1)
		default:
			v3 := c.pop()
			if v3.size() == 2 {
				c.pushAll(v2, v1, v3, v2, v1)
				break
			}
			v4 := c.pop()
			c.pushAll(v2, v1, v4, v3, v2, v1)
		}
	case SWAP:
		v1, v2 := c.pop(), c.pop()
		c.pushAll(v1, v2)
	case IADD, ISUB, IMUL, IDIV, IREM, IAND, IOR, IXOR, ISHL, ISHR, IUSHR:
		c.popN(2)
		c.push(intType)
	case LADD, LSUB, LMUL, LDIV, LREM, LAND, LOR, LXOR, LSHL, LSHR, LUSHR:
		c.popN(2)
		c.push(longType)
	case MONITORENTER, MONITOREXIT:
		c.pop()
	case FADD, FSUB, FMUL, FDIV, FREM:
		c.popN(2)
		c.push(floatType)
	case DADD, DSUB, DMUL, DDIV, DREM:
		c.popN(2)
		c.push(doubleType)
	case INEG, LNEG, FNEG, DNEG, I2B, I2C, I2S:
	case I2L, F2L, D2L:
		c.pop()
		c.push(longType)
	case I2F, L2F, D2F:
		c.pop()
		c.push(floatType)
	case I2D, L2D, F2D:
		c.pop()
		c.push(doubleType)
	case L2I, F2I, D2I:
		c.pop()
		c.push(intType)
	case LCMP, FCMPL, FCMPG, DCMPL, DCMPG:
		c.popN(2)
		c.push(intType)
	case ARRAYLENGTH:
		c.pop()
		c.push(intType)
	case IRETURN, LRETURN, FRETURN, DRETURN, ARETURN, ATHROW:
		c.traceEnd()
		c.pop()
		c.reachable = false
	case RETURN:
		c.traceEnd()
		c.reachable = false
	default:
		c.fail("opcode %#x is not an instruction without operands", op)
	}
}

func (c *Code) pushAll(ts ...vtype) {
	for _, t := range ts {
		c.push(t)
	}
}

// Depth returns the number of values on the operand stack.
func (c *Code) Depth() int { return len(c.stack) }

// Stack returns the types of the values on the operand stack, bottom
// first.
func (c *Code) Stack() []VType { return slices.Clone(c.stack) }

// StackTop returns the type of the value on top of the operand stack.
func (c *Code) StackTop() VType {
	if len(c.stack) == 0 {
		c.fail("operand stack underflow at offset %d", len(c.buf))
		return vtype{}
	}
	return c.stack[len(c.stack)-1]
}

// PopTo pops values off the operand stack until depth values are left.
func (c *Code) PopTo(depth int) {
	for c.reachable && len(c.stack) > depth {
		if c.stack[len(c.stack)-1].size() == 2 {
			c.Op(POP2)
		} else {
			c.Op(POP)
		}
	}
}

func (c *Code) op2(op byte, v uint16) {
	c.buf = append(c.buf, op, byte(v>>8), byte(v))
}

func (c *Code) Iconst(v int32) {
	if !c.reachable {
		return
	}
	switch {
	case -1 <= v && v <= 5:
		c.Op(byte(ICONST_0 + v))
	case math.MinInt8 <= v && v <= math.MaxInt8:
		c.buf = append(c.buf, BIPUSH, byte(v))
		c.push(intType)
	case math.MinInt16 <= v && v <= math.MaxInt16:
		c.op2(SIPUSH, uint16(v))
		c.push(intType)
	default:
		c.ldc(c.pool.Int(v), intType)
	}
}

func (c *Code) Lconst(v int64) {
	if v == 0 || v == 1 {
		c.Op(byte(LCONST_0 + v))
		return
	}
	if !c.reachable {
		return
	}
	c.op2(LDC2_W, c.pool.Long(v))
	c.push(longType)
}

func (c *Code) Dconst(v float64) {
	if (v == 0 && !math.Signbit(v)) || v == 1 {
		c.Op(byte(DCONST_0 + int(v)))
		return
	}
	if !c.reachable {
		return
	}
	c.op2(LDC2_W, c.pool.Double(v))
	c.push(doubleType)
}

func (c *Code) Fconst(v float32) {
	if (v == 0 && !math.Signbit(float64(v))) || v == 1 || v == 2 {
		c.Op(byte(FCONST_0 + int(v)))
		return
	}
	if !c.reachable {
		return
	}
	c.ldc(c.pool.Float(v), floatType)
}

func (c *Code) Sconst(s string) {
	if !c.reachable {
		return
	}
	c.ldc(c.pool.String(s), stringType)
}

// LdcClass pushes the Class object of a class: Foo::class.java.
func (c *Code) LdcClass(name string) {
	if !c.reachable {
		return
	}
	c.ldc(c.pool.Class(name), object("java/lang/Class"))
}

func (c *Code) ldc(idx uint16, t vtype) {
	if idx <= 255 {
		c.buf = append(c.buf, LDC, byte(idx))
	} else {
		c.op2(LDC_W, idx)
	}
	c.push(t)
}

// Local emits a load or store (ILOAD/LLOAD/FLOAD/DLOAD/ALOAD or the store
// forms).
func (c *Code) Local(op byte, slot int) {
	if !c.reachable {
		return
	}
	if slot >= len(c.locals) {
		c.fail("local %d is not allocated", slot)
		return
	}
	if op >= ISTORE && op <= ASTORE {
		c.trace(traceDef, slot)
	} else {
		c.trace(traceUse, slot)
	}
	switch op {
	case ILOAD:
		c.push(intType)
	case LLOAD:
		c.push(longType)
	case DLOAD:
		c.push(doubleType)
	case FLOAD:
		c.push(floatType)
	case ALOAD:
		c.push(c.locals[slot])
	case ISTORE, LSTORE, FSTORE, DSTORE, ASTORE:
		c.pop()
		c.assigned.set(slot)
	default:
		c.fail("opcode %#x is not a load or store", op)
		return
	}
	if slot > 255 {
		c.buf = append(c.buf, WIDE, op, byte(slot>>8), byte(slot))
	} else {
		c.buf = append(c.buf, op, byte(slot))
	}
	if op >= ISTORE && op <= ASTORE && len(c.vars) > 0 {
		c.stored(slot)
	}
}

func (c *Code) Iinc(slot int, inc int) {
	if !c.reachable {
		return
	}
	c.trace(traceUseDef, slot)
	if slot > 255 || inc < math.MinInt8 || inc > math.MaxInt8 {
		c.buf = append(c.buf, WIDE, IINC, byte(slot>>8), byte(slot), byte(inc>>8), byte(inc))
		return
	}
	c.buf = append(c.buf, IINC, byte(slot), byte(int8(inc)))
}

// Jump emits a branch to l.
func (c *Code) Jump(op byte, l *Label) {
	if !c.reachable {
		return
	}
	switch op {
	case IFEQ, IFNE, IFLT, IFGE, IFGT, IFLE, IFNULL, IFNONNULL:
		c.pop()
	case IF_ICMPEQ, IF_ICMPNE, IF_ICMPLT, IF_ICMPGE, IF_ICMPGT, IF_ICMPLE, IF_ACMPEQ, IF_ACMPNE:
		c.popN(2)
	}
	at := len(c.buf)
	if op == GOTO {
		c.trace(traceGoto, -1, l)
	} else {
		c.trace(traceBranch, -1, l)
	}
	c.buf = append(c.buf, op, 0, 0)
	c.branch(l, fixup{at: at, patch: at + 1})
	if op == GOTO {
		c.reachable = false
	}
}

// branch records an edge from the current state to l, whose offset goes
// where f says.
func (c *Code) branch(l *Label, f fixup) {
	l.jumped = true
	switch {
	case l.pos >= 0:
		c.checkBackward(l)
		c.patch(f, l.pos)
	default:
		c.merge(l)
		l.fixups = append(l.fixups, f)
	}
}

func (c *Code) patch(f fixup, target int) {
	off := target - f.at
	if f.wide {
		binary.BigEndian.PutUint32(c.buf[f.patch:], uint32(int32(off)))
		return
	}
	if off < math.MinInt16 || off > math.MaxInt16 {
		c.fail("method too large: branch offset %d", off)
		return
	}
	c.buf[f.patch], c.buf[f.patch+1] = byte(off>>8), byte(off)
}

// TableSwitch emits a tableswitch on the int on the stack: keys low,
// low+1, ... go to targets, others to dflt.
func (c *Code) TableSwitch(low int32, dflt *Label, targets []*Label) {
	if !c.reachable {
		return
	}
	c.trace(traceGoto, -1, append([]*Label{dflt}, targets...)...)
	at := c.switchStart(TABLESWITCH)
	// Each offset's bytes are added before the branch, which patches them
	// at once if the target is placed.
	c.buf = binary.BigEndian.AppendUint32(c.buf, 0)
	c.branch(dflt, fixup{at: at, patch: len(c.buf) - 4, wide: true})
	c.buf = binary.BigEndian.AppendUint32(c.buf, uint32(low))
	c.buf = binary.BigEndian.AppendUint32(c.buf, uint32(low+int32(len(targets))-1))
	for _, l := range targets {
		c.buf = binary.BigEndian.AppendUint32(c.buf, 0)
		c.branch(l, fixup{at: at, patch: len(c.buf) - 4, wide: true})
	}
	c.reachable = false
}

// LookupSwitch emits a lookupswitch on the int on the stack: keys, in
// increasing order, go to targets, others to dflt.
func (c *Code) LookupSwitch(dflt *Label, keys []int32, targets []*Label) {
	if !c.reachable {
		return
	}
	c.trace(traceGoto, -1, append([]*Label{dflt}, targets...)...)
	at := c.switchStart(LOOKUPSWITCH)
	c.buf = binary.BigEndian.AppendUint32(c.buf, 0)
	c.branch(dflt, fixup{at: at, patch: len(c.buf) - 4, wide: true})
	c.buf = binary.BigEndian.AppendUint32(c.buf, uint32(len(keys)))
	for i, l := range targets {
		c.buf = binary.BigEndian.AppendUint32(c.buf, uint32(keys[i]))
		c.buf = binary.BigEndian.AppendUint32(c.buf, 0)
		c.branch(l, fixup{at: at, patch: len(c.buf) - 4, wide: true})
	}
	c.reachable = false
}

// switchStart pops a switch's key and emits its opcode and padding. It
// returns the opcode's offset.
func (c *Code) switchStart(op byte) int {
	c.pop()
	at := len(c.buf)
	c.buf = append(c.buf, op)
	for len(c.buf)%4 != 0 {
		c.buf = append(c.buf, 0)
	}
	return at
}

// merge adds the current state to the states reaching l, which is not
// placed yet. Locals hold a value only if they do on every edge. Stack
// values must have the same type, except that null merges with any
// reference, and the top value of a join takes the join's type.
func (c *Code) merge(l *Label) {
	if l.fixed {
		c.checkFrame(l)
		if l.below > 0 {
			var ext bitset
			for s := range l.below {
				if c.assigned.has(s) {
					ext.set(s)
				}
			}
			if l.reached {
				l.ext.and(ext)
			} else {
				l.ext, l.reached = ext, true
			}
			l.frame.assigned = slices.Clone(l.min)
			l.frame.assigned.or(l.ext)
		}
		return
	}
	f := l.frame
	if f == nil {
		f = &state{stack: slices.Clone(c.stack), assigned: slices.Clone(c.assigned), uninitThis: c.uninitThis}
		if l.top != nil && len(f.stack) > 0 {
			f.stack[len(f.stack)-1] = *l.top
		}
		l.frame = f
		return
	}
	if len(f.stack) != len(c.stack) {
		c.fail("inconsistent stack depth at label: %d vs %d values", len(f.stack), len(c.stack))
		return
	}
	if f.uninitThis != c.uninitThis {
		c.fail("paths to a label differ in whether the super constructor was called")
		return
	}
	for i, t := range c.stack {
		ft := f.stack[i]
		switch {
		case ft == t, l.top != nil && i == len(f.stack)-1, t.tag == tNull && ft.isRef():
		case ft.tag == tNull && t.isRef():
			f.stack[i] = t
		default:
			c.fail("stack value %d at label has types %v and %v", i, ft, t)
		}
	}
	f.assigned.and(c.assigned)
}

// checkBackward checks a jump to the placed label l: its frame, and the
// code after it, must hold for the current state too.
func (c *Code) checkBackward(l *Label) {
	if l.fixed {
		c.checkFrame(l)
		return
	}
	f := l.frame
	if f == nil {
		c.fail("jump back to unreachable code")
		return
	}
	if len(f.stack) != len(c.stack) {
		c.fail("inconsistent stack depth at branch to label: %d vs %d values", len(f.stack), len(c.stack))
		return
	}
	if f.uninitThis != c.uninitThis {
		c.fail("branch to a label across the super constructor call")
		return
	}
	for i, t := range c.stack {
		ft := f.stack[i]
		if ft != t && !(l.top != nil && i == len(f.stack)-1) && !(t.tag == tNull && ft.isRef()) {
			c.fail("stack value %d at branch to label has type %v, not %v", i, t, ft)
		}
	}
	if !c.assigned.covers(f.assigned) {
		c.fail("branch to label from a state where fewer locals hold a value")
	}
}

// checkFrame checks an edge to a label whose frame is given: the stack
// must have the frame's depth and kinds of values, and the locals that
// hold a value must cover the frame's. Reference types aren't compared:
// the frame comes from verified code, which only the JVM, with the class
// hierarchy, can check.
func (c *Code) checkFrame(l *Label) {
	f := l.frame
	if len(f.stack) != len(c.stack) {
		c.fail("inconsistent stack depth at branch to frame: %d vs %d values", len(f.stack), len(c.stack))
		return
	}
	if f.uninitThis != c.uninitThis {
		c.fail("branch to a frame across the super constructor call")
		return
	}
	for i, t := range c.stack {
		ft := f.stack[i]
		if ft != t && !(ft.isRef() && t.isRef()) {
			c.fail("stack value %d at branch to frame has type %v, not %v", i, t, ft)
		}
	}
	if !c.assigned.covers(l.min) || l.pos >= 0 && !c.assigned.covers(f.assigned) {
		c.fail("branch to frame from a state where fewer locals hold a value")
	}
}

// Place binds l to the current position. The code after it starts from
// l's merged state.
func (c *Code) Place(l *Label) {
	l.pos = len(c.buf)
	for _, f := range l.fixups {
		c.patch(f, l.pos)
	}
	l.fixups = nil
	switch {
	case c.reachable:
		c.merge(l)
	case l.fixed:
		// Code after an unconditional jump needs a frame.
		l.jumped = true
	}
	if l.frame == nil {
		return // unreachable, and nothing jumps here yet
	}
	c.reachable = true
	c.stack = append(c.stack[:0], l.frame.stack...)
	c.depth = 0
	for _, t := range c.stack {
		c.depth += t.size()
	}
	if c.depth > c.max {
		c.max = c.depth // a handler starts with its exception on the stack
	}
	c.assigned = append(c.assigned[:0], l.frame.assigned...)
	c.uninitThis = l.frame.uninitThis
	c.placed = append(c.placed, l)
}

// handler is an entry of the exception table: exceptions of class
// catchType (0: any) thrown by the code in [start, end) go to handler.
type handler struct {
	start, end int
	handler    *Label
	catchType  uint16
}

// Snapshot is the state at the start of a try block, where its handlers
// start from.
type Snapshot struct {
	assigned  bitset
	reachable bool
	// uninitThis is set before the super constructor call: a try in the
	// arguments of this(...) or super(...).
	uninitThis bool
}

// Snapshot returns the current state, for Catch.
func (c *Code) Snapshot() Snapshot {
	return Snapshot{slices.Clone(c.assigned), c.reachable, c.uninitThis}
}

// Assigned returns the locals that hold a value.
func (c *Code) Assigned() []int {
	var out []int
	for s := range c.locals {
		if c.assigned.has(s) {
			out = append(out, s)
		}
	}
	return out
}

// Pos returns the offset of the next instruction.
func (c *Code) Pos() int { return len(c.buf) }

// Catch makes l the handler of the exceptions of class catchType ("" for
// any) thrown by the code in ranges of offsets, [start, end). Unless l is
// a frame label (NewFrameLabel), whose frame is kept, the handler
// starts with the exception on the stack, and the locals that held a value
// at s, the start of the try: locals are never reused, so they hold the
// same values throughout the try. Entries are written in the order of
// the calls, which must put the handlers of inner trys first. Empty ranges
// are dropped; with none left, nothing reaches l.
func (c *Code) Catch(s Snapshot, ranges [][2]int, l *Label, catchType string) {
	if !s.reachable {
		return
	}
	var ct uint16
	exc := object("java/lang/Throwable")
	if catchType != "" {
		ct = c.pool.Class(catchType)
		exc = object(catchType)
	}
	n := len(c.handlers)
	for _, r := range ranges {
		if r[0] < r[1] {
			c.handlers = append(c.handlers, handler{r[0], r[1], l, ct})
		}
	}
	if len(c.handlers) == n {
		return
	}
	l.jumped = true
	if !l.fixed {
		l.frame = &state{stack: []vtype{exc}, assigned: slices.Clone(s.assigned), uninitThis: s.uninitThis}
	}
}

// Spill moves the values on the operand stack to new locals, so that a
// try block, whose handlers start with an empty stack, can start there.
// Unspill puts them back. It reports false if a value is an object that
// new created and isn't constructed yet, which can't be stored; this
// before the super constructor call can.
func (c *Code) Spill() ([]int, bool) { return c.SpillAbove(0) }

// SpillAbove is Spill for the values above the first depth on the stack,
// which stay.
func (c *Code) SpillAbove(depth int) ([]int, bool) {
	if !c.reachable {
		return nil, true
	}
	for _, t := range c.stack[depth:] {
		if t.tag == tUninit {
			return nil, false
		}
	}
	var slots []int
	for i := len(c.stack) - 1; i >= depth; i-- {
		t := c.stack[i]
		var desc string
		var op byte
		switch t.tag {
		case tNull:
			c.Op(POP)
			slots = append(slots, -1) // null is pushed again
			continue
		case tInt:
			desc, op = "I", ISTORE
		case tLong:
			desc, op = "J", LSTORE
		case tDouble:
			desc, op = "D", DSTORE
		case tFloat:
			desc, op = "F", FSTORE
		case tUninitThis:
			// this before the super constructor call, as the receiver of
			// this(...) or super(...): its local is uninitializedThis.
			slot := c.AllocLocal("L" + c.class + ";")
			c.locals[slot] = t
			c.thisCopies = append(c.thisCopies, slot)
			c.Local(ASTORE, slot)
			slots = append(slots, slot)
			continue
		default:
			desc, op = "L"+t.name+";", ASTORE
			if t.name[0] == '[' {
				desc = t.name
			}
		}
		slot := c.AllocLocal(desc)
		c.Local(op, slot)
		slots = append(slots, slot)
	}
	return slots, true
}

// Unspill pushes back the values that Spill stored, whose locals are
// then dead.
func (c *Code) Unspill(slots []int) {
	for i := len(slots) - 1; i >= 0; i-- {
		if slots[i] < 0 {
			c.Op(ACONST_NULL)
			continue
		}
		defer c.Retire(slots[i])
		var op byte = ALOAD
		switch t := c.locals[slots[i]]; t.tag {
		case tInt:
			op = ILOAD
		case tLong:
			op = LLOAD
		case tDouble:
			op = DLOAD
		case tFloat:
			op = FLOAD
		}
		c.Local(op, slots[i])
	}
}

func (c *Code) Invoke(op byte, owner, name, desc string) {
	if !c.reachable {
		return
	}
	c.op2(op, c.pool.Method(owner, name, desc))
	c.call(op, name, desc)
}

// InvokeStaticInterface calls a static method of an interface.
func (c *Code) InvokeStaticInterface(owner, name, desc string) {
	if !c.reachable {
		return
	}
	c.op2(INVOKESTATIC, c.pool.InterfaceMethod(owner, name, desc))
	c.call(INVOKESTATIC, name, desc)
}

// InvokeSpecialInterface calls an interface's default method directly, as
// super<I>.f() does.
func (c *Code) InvokeSpecialInterface(owner, name, desc string) {
	if !c.reachable {
		return
	}
	c.op2(INVOKESPECIAL, c.pool.InterfaceMethod(owner, name, desc))
	c.call(INVOKESPECIAL, name, desc)
}

// InvokeInterface calls an interface method; the receiver is below the
// arguments on the stack.
func (c *Code) InvokeInterface(owner, name, desc string) {
	if !c.reachable {
		return
	}
	_, slots, _ := scanDesc(desc)
	idx := c.pool.InterfaceMethod(owner, name, desc)
	c.buf = append(c.buf, INVOKEINTERFACE, byte(idx>>8), byte(idx), byte(slots+1), 0)
	c.call(INVOKEINTERFACE, name, desc)
}

// InvokeDynamic emits an invokedynamic call site with the given bootstrap
// method; desc is the type of the call.
func (c *Code) InvokeDynamic(bsm uint16, name, desc string) {
	if !c.reachable {
		return
	}
	idx := c.pool.InvokeDynamic(bsm, name, desc)
	c.buf = append(c.buf, INVOKEDYNAMIC, byte(idx>>8), byte(idx), 0, 0)
	c.call(INVOKEDYNAMIC, name, desc)
}

// call pops the arguments and receiver of a call and pushes its result. A
// constructor call initializes its receiver: every copy of it, on the
// stack and in the locals, gets its class's type.
func (c *Code) call(op byte, name, desc string) {
	n, _, ret := scanDesc(desc)
	c.popN(n)
	if op != INVOKESTATIC && op != INVOKEDYNAMIC {
		recv := c.pop()
		if op == INVOKESPECIAL && name == "<init>" {
			c.initialize(recv)
		}
	}
	if ret != "V" {
		c.push(typeOf(ret))
	}
	if fn := c.afterCall; fn != nil {
		c.afterCall = nil
		fn()
	}
}

func (c *Code) initialize(recv vtype) {
	var t vtype
	switch recv.tag {
	case tUninitThis:
		t, c.uninitThis = object(c.class), false
	case tUninit:
		t = object(recv.name)
	default:
		return
	}
	for i := range c.stack {
		if c.stack[i] == recv {
			c.stack[i] = t
		}
	}
	for i := range c.locals {
		if c.locals[i] == recv {
			c.locals[i] = t
		}
	}
}

func (c *Code) FieldInsn(op byte, owner, name, desc string) {
	if !c.reachable {
		return
	}
	c.op2(op, c.pool.Field(owner, name, desc))
	switch op {
	case GETSTATIC:
		c.push(typeOf(desc))
	case PUTSTATIC:
		c.pop()
	case GETFIELD:
		c.pop()
		c.push(typeOf(desc))
	case PUTFIELD:
		c.popN(2)
	}
}

func (c *Code) New(class string) {
	if !c.reachable {
		return
	}
	at := len(c.buf)
	c.op2(NEW, c.pool.Class(class))
	c.push(vtype{tag: tUninit, name: class, off: at})
}

// Array types for NewArray.
const (
	TBoolean = 4
	TChar    = 5
	TFloat   = 6
	TDouble  = 7
	TByte    = 8
	TShort   = 9
	TInt     = 10
	TLong    = 11
)

// NewArray creates an array of a primitive type; the length is on the stack.
func (c *Code) NewArray(atype byte) {
	if !c.reachable {
		return
	}
	c.buf = append(c.buf, NEWARRAY, atype)
	c.pop()
	c.push(object(map[byte]string{TBoolean: "[Z", TChar: "[C", TFloat: "[F", TDouble: "[D", TByte: "[B", TShort: "[S",
		TInt: "[I", TLong: "[J"}[atype]))
}

// ANewArray creates an array of class elements; the length is on the stack.
func (c *Code) ANewArray(class string) {
	if !c.reachable {
		return
	}
	c.op2(ANEWARRAY, c.pool.Class(class))
	c.pop()
	if class[0] == '[' {
		c.push(object("[" + class))
	} else {
		c.push(object("[L" + class + ";"))
	}
}

// MultiANewArray creates an array of an array type (a descriptor) with
// dims dimensions, whose lengths are on the stack.
func (c *Code) MultiANewArray(desc string, dims int) {
	if !c.reachable {
		return
	}
	c.op2(MULTIANEWARRAY, c.pool.Class(desc))
	c.buf = append(c.buf, byte(dims))
	c.popN(dims)
	c.push(object(desc))
}

// InstanceOf replaces the reference on the stack with 1 if it is an
// instance of class, and 0 otherwise.
func (c *Code) InstanceOf(class string) {
	if !c.reachable {
		return
	}
	c.op2(INSTANCEOF, c.pool.Class(class))
	c.pop()
	c.push(intType)
}

func (c *Code) Checkcast(class string) {
	if !c.reachable {
		return
	}
	c.op2(CHECKCAST, c.pool.Class(class))
	c.pop()
	c.push(object(class))
}

// Uncast removes the checkcast at offset at, which must be the last
// instruction, and puts back top, the type of the value before it. It
// reports false and changes nothing if the code since refers to the
// offset after the checkcast: a label, a line, a local's range, a
// handler or a liveness trace.
func (c *Code) Uncast(at int, top VType) bool {
	end := at + 3
	if !c.reachable || len(c.buf) != end || c.buf[at] != CHECKCAST || len(c.stack) == 0 {
		return false
	}
	if n := len(c.placed); n > 0 && c.placed[n-1].pos >= end {
		return false
	}
	if n := len(c.lines); n > 0 && c.lines[n-1].pc >= end {
		return false
	}
	if n := len(c.traces); n > 0 && c.traces[n-1].pos >= end {
		return false
	}
	for _, v := range c.vars {
		if v.start >= end || v.end >= end {
			return false
		}
	}
	for _, h := range c.handlers {
		if h.end >= end {
			return false
		}
	}
	c.buf = c.buf[:at]
	c.stack[len(c.stack)-1] = top
	return true
}

func (c *Code) finish() error {
	if c.err != nil {
		return c.err
	}
	if c.reachable {
		return fmt.Errorf("code falls off the end of the method")
	}
	if len(c.buf) > 65535 {
		return fmt.Errorf("method too large")
	}
	return nil
}

// Err reports the first assembly error, if any.
func (c *Code) Err() error { return c.finish() }
