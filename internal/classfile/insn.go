package classfile

import (
	"encoding/binary"
	"fmt"
)

// Opcodes the decoder produces, besides those the code generator uses.
const (
	NOP            = 0x00
	ISTORE_0       = 0x3b
	DUP_X2         = 0x5b
	DUP2_X1        = 0x5d
	DUP2_X2        = 0x5e
	SWAP           = 0x5f
	ISHL           = 0x78
	LSHL           = 0x79
	ISHR           = 0x7a
	LSHR           = 0x7b
	IUSHR          = 0x7c
	LUSHR          = 0x7d
	LAND           = 0x7f
	IOR            = 0x80
	LOR            = 0x81
	LXOR           = 0x83
	JSR            = 0xa8
	RET            = 0xa9
	TABLESWITCH    = 0xaa
	LOOKUPSWITCH   = 0xab
	MONITORENTER   = 0xc2
	MONITOREXIT    = 0xc3
	MULTIANEWARRAY = 0xc5
	GOTO_W         = 0xc8
	JSR_W          = 0xc9
	ILOAD_0        = 0x1a
)

// Insn is a decoded instruction. Short forms are normalized: aload_1 is
// ALOAD with Local 1, ldc_w is LDC, goto_w is GOTO, and wide forms are the
// plain ones with their wide operands.
type Insn struct {
	Off int  // offset in the code
	Op  byte // opcode
	Len int  // length in bytes, including operands and padding

	Local int // loads, stores and iinc: the local's slot

	// Int is the value of iconst, bipush, sipush, an int or long ldc, and
	// iinc's increment; newarray's element type; multianewarray's
	// dimensions. Float is a float or double ldc's value.
	Int   int64
	Float float64
	Const ConstKind // ldc: what it loads

	// Class is the class of new, anewarray, checkcast, instanceof,
	// multianewarray and a class ldc; Str a string ldc's value.
	Class string
	Str   string

	// Field and method instructions.
	Owner, Name, Desc string
	Iface             bool // an InterfaceMethodref

	// Bootstrap is invokedynamic's bootstrap method; Name and Desc are
	// the call site's.
	Bootstrap *Bootstrap

	// Branches: the target offset. Switches: the default's, with Keys
	// and Targets for the cases (tableswitch's keys are consecutive).
	Target  int
	Keys    []int32
	Targets []int
}

// opInfo is an opcode's length in bytes (0: variable) and the stack
// slots it pops and pushes (-1: given by its operands).
type opInfo struct{ len, pop, push int8 }

var opTable = func() [256]opInfo {
	var t [256]opInfo
	set := func(from, to int, i opInfo) {
		for op := from; op <= to; op++ {
			t[op] = i
		}
	}
	set(NOP, NOP, opInfo{1, 0, 0})
	set(ACONST_NULL, ICONST_5, opInfo{1, 0, 1})
	set(LCONST_0, LCONST_1, opInfo{1, 0, 2})
	set(FCONST_0, FCONST_2, opInfo{1, 0, 1})
	set(DCONST_0, DCONST_1, opInfo{1, 0, 2})
	t[BIPUSH] = opInfo{2, 0, 1}
	t[SIPUSH] = opInfo{3, 0, 1}
	t[LDC] = opInfo{2, 0, -1}
	t[LDC_W] = opInfo{3, 0, -1}
	t[LDC2_W] = opInfo{3, 0, 2}
	t[ILOAD], t[FLOAD], t[ALOAD] = opInfo{2, 0, 1}, opInfo{2, 0, 1}, opInfo{2, 0, 1}
	t[LLOAD], t[DLOAD] = opInfo{2, 0, 2}, opInfo{2, 0, 2}
	set(ILOAD_0, ILOAD_0+3, opInfo{1, 0, 1})     // iload_n
	set(ILOAD_0+4, ILOAD_0+7, opInfo{1, 0, 2})   // lload_n
	set(ILOAD_0+8, ILOAD_0+11, opInfo{1, 0, 1})  // fload_n
	set(ILOAD_0+12, ILOAD_0+15, opInfo{1, 0, 2}) // dload_n
	set(ILOAD_0+16, ILOAD_0+19, opInfo{1, 0, 1}) // aload_n
	set(IALOAD, SALOAD, opInfo{1, 2, 1})
	t[LALOAD], t[DALOAD] = opInfo{1, 2, 2}, opInfo{1, 2, 2}
	t[ISTORE], t[FSTORE], t[ASTORE] = opInfo{2, 1, 0}, opInfo{2, 1, 0}, opInfo{2, 1, 0}
	t[LSTORE], t[DSTORE] = opInfo{2, 2, 0}, opInfo{2, 2, 0}
	set(ISTORE_0, ISTORE_0+3, opInfo{1, 1, 0})
	set(ISTORE_0+4, ISTORE_0+7, opInfo{1, 2, 0})
	set(ISTORE_0+8, ISTORE_0+11, opInfo{1, 1, 0})
	set(ISTORE_0+12, ISTORE_0+15, opInfo{1, 2, 0})
	set(ISTORE_0+16, ISTORE_0+19, opInfo{1, 1, 0})
	set(IASTORE, SASTORE, opInfo{1, 3, 0})
	t[LASTORE], t[DASTORE] = opInfo{1, 4, 0}, opInfo{1, 4, 0}
	t[POP], t[POP2] = opInfo{1, 1, 0}, opInfo{1, 2, 0}
	t[DUP], t[DUP_X1], t[DUP_X2] = opInfo{1, 1, 2}, opInfo{1, 2, 3}, opInfo{1, 3, 4}
	t[DUP2], t[DUP2_X1], t[DUP2_X2] = opInfo{1, 2, 4}, opInfo{1, 3, 5}, opInfo{1, 4, 6}
	t[SWAP] = opInfo{1, 2, 2}
	for op := IADD; op <= DREM; op += 4 { // add, sub, mul, div, rem
		t[op], t[op+1], t[op+2], t[op+3] = opInfo{1, 2, 1}, opInfo{1, 4, 2}, opInfo{1, 2, 1}, opInfo{1, 4, 2}
	}
	t[INEG], t[LNEG], t[FNEG], t[DNEG] = opInfo{1, 1, 1}, opInfo{1, 2, 2}, opInfo{1, 1, 1}, opInfo{1, 2, 2}
	t[ISHL], t[ISHR], t[IUSHR] = opInfo{1, 2, 1}, opInfo{1, 2, 1}, opInfo{1, 2, 1}
	t[LSHL], t[LSHR], t[LUSHR] = opInfo{1, 3, 2}, opInfo{1, 3, 2}, opInfo{1, 3, 2}
	t[IAND], t[IOR], t[IXOR] = opInfo{1, 2, 1}, opInfo{1, 2, 1}, opInfo{1, 2, 1}
	t[LAND], t[LOR], t[LXOR] = opInfo{1, 4, 2}, opInfo{1, 4, 2}, opInfo{1, 4, 2}
	t[IINC] = opInfo{3, 0, 0}
	t[I2L], t[I2F], t[I2D] = opInfo{1, 1, 2}, opInfo{1, 1, 1}, opInfo{1, 1, 2}
	t[L2I], t[L2F], t[L2D] = opInfo{1, 2, 1}, opInfo{1, 2, 1}, opInfo{1, 2, 2}
	t[F2I], t[F2L], t[F2D] = opInfo{1, 1, 1}, opInfo{1, 1, 2}, opInfo{1, 1, 2}
	t[D2I], t[D2L], t[D2F] = opInfo{1, 2, 1}, opInfo{1, 2, 2}, opInfo{1, 2, 1}
	set(I2B, I2S, opInfo{1, 1, 1})
	t[LCMP], t[FCMPL], t[FCMPG], t[DCMPL], t[DCMPG] = opInfo{1, 4, 1}, opInfo{1, 2, 1}, opInfo{1, 2, 1}, opInfo{1, 4, 1}, opInfo{1, 4, 1}
	set(IFEQ, IFLE, opInfo{3, 1, 0})
	set(IF_ICMPEQ, IF_ACMPNE, opInfo{3, 2, 0})
	t[GOTO] = opInfo{3, 0, 0}
	t[JSR], t[RET] = opInfo{3, 0, 1}, opInfo{2, 0, 0}
	t[TABLESWITCH], t[LOOKUPSWITCH] = opInfo{0, 1, 0}, opInfo{0, 1, 0}
	t[IRETURN], t[LRETURN], t[FRETURN], t[DRETURN], t[ARETURN], t[RETURN] =
		opInfo{1, 1, 0}, opInfo{1, 2, 0}, opInfo{1, 1, 0}, opInfo{1, 2, 0}, opInfo{1, 1, 0}, opInfo{1, 0, 0}
	set(GETSTATIC, INVOKESTATIC, opInfo{3, -1, -1})
	t[INVOKEINTERFACE], t[INVOKEDYNAMIC] = opInfo{5, -1, -1}, opInfo{5, -1, -1}
	t[NEW] = opInfo{3, 0, 1}
	t[NEWARRAY] = opInfo{2, 1, 1}
	t[ANEWARRAY] = opInfo{3, 1, 1}
	t[ARRAYLENGTH] = opInfo{1, 1, 1}
	t[ATHROW] = opInfo{1, 1, 0}
	t[CHECKCAST], t[INSTANCEOF] = opInfo{3, 1, 1}, opInfo{3, 1, 1}
	t[MONITORENTER], t[MONITOREXIT] = opInfo{1, 1, 0}, opInfo{1, 1, 0}
	t[WIDE] = opInfo{0, -1, -1}
	t[MULTIANEWARRAY] = opInfo{4, -1, 1}
	t[IFNULL], t[IFNONNULL] = opInfo{3, 1, 0}, opInfo{3, 1, 0}
	t[GOTO_W], t[JSR_W] = opInfo{5, 0, 0}, opInfo{5, 0, 1}
	return t
}()

// Effect returns the stack slots the instruction pops and pushes.
func (in *Insn) Effect() (pop, push int) {
	info := opTable[in.Op]
	pop, push = int(info.pop), int(info.push)
	switch in.Op {
	case LDC:
		push = 1
		if in.Const == ConstLong || in.Const == ConstDouble {
			push = 2
		}
	case GETSTATIC:
		pop, push = 0, typeOf(in.Desc).size()
	case PUTSTATIC:
		pop, push = typeOf(in.Desc).size(), 0
	case GETFIELD:
		pop, push = 1, typeOf(in.Desc).size()
	case PUTFIELD:
		pop, push = 1+typeOf(in.Desc).size(), 0
	case INVOKEVIRTUAL, INVOKESPECIAL, INVOKESTATIC, INVOKEINTERFACE, INVOKEDYNAMIC:
		_, slots, ret := scanDesc(in.Desc)
		pop, push = slots, 0
		if in.Op != INVOKESTATIC && in.Op != INVOKEDYNAMIC {
			pop++
		}
		if ret != "V" {
			push = typeOf(ret).size()
		}
	case MULTIANEWARRAY:
		pop = int(in.Int)
	}
	return pop, push
}

// UnsupportedError reports an instruction that kc can't inline.
type UnsupportedError struct {
	Op   byte
	What string
}

func (e *UnsupportedError) Error() string { return "its body uses " + e.What }

// Decode decodes a method's instructions, resolving their constants.
func (m *Method) Decode() ([]Insn, error) {
	code, p := m.Code, m.pool
	var out []Insn
	for off := 0; off < len(code); {
		in, err := decodeInsn(code, off, p)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
		off += in.Len
	}
	return out, nil
}

func decodeInsn(code []byte, off int, p *pool) (Insn, error) {
	in := Insn{Off: off, Op: code[off]}
	info := opTable[in.Op]
	if info.len == 0 && in.Op != TABLESWITCH && in.Op != LOOKUPSWITCH && in.Op != WIDE {
		return in, fmt.Errorf("unknown opcode %#x at offset %d", in.Op, off)
	}
	in.Len = int(info.len)
	need := func(n int) bool { return off+n <= len(code) }
	if in.Len > 0 && !need(in.Len) {
		return in, fmt.Errorf("truncated instruction at offset %d", off)
	}
	u1 := func(i int) int { return int(code[off+i]) }
	u2 := func(i int) uint16 { return binary.BigEndian.Uint16(code[off+i:]) }
	s2 := func(i int) int { return int(int16(u2(i))) }
	s4 := func(i int) int32 { return int32(binary.BigEndian.Uint32(code[off+i:])) }
	var err error
	switch op := in.Op; {
	case ICONST_M1 <= op && op <= ICONST_5:
		in.Int = int64(op) - ICONST_0
	case op == LCONST_0 || op == LCONST_1:
		in.Int = int64(op - LCONST_0)
	case FCONST_0 <= op && op <= FCONST_2:
		in.Float = float64(op - FCONST_0)
	case op == DCONST_0 || op == DCONST_1:
		in.Float = float64(op - DCONST_0)
	case op == BIPUSH:
		in.Int = int64(int8(code[off+1]))
	case op == SIPUSH:
		in.Int = int64(int16(u2(1)))
	case op == LDC || op == LDC_W || op == LDC2_W:
		idx := uint16(u1(1))
		if op != LDC {
			idx = u2(1)
		}
		in.Op = LDC
		in.Const, in.Int, in.Float, in.Str, err = p.constant(idx)
		if in.Const == ConstClass {
			in.Class, in.Str = in.Str, ""
		}
		if in.Const == ConstOther {
			return in, &UnsupportedError{op, "a method handle or dynamic constant"}
		}
	case ILOAD <= op && op <= ALOAD, ISTORE <= op && op <= ASTORE:
		in.Local = u1(1)
	case ILOAD_0 <= op && op < ILOAD_0+20:
		in.Op, in.Local = ILOAD+(op-ILOAD_0)/4, int(op-ILOAD_0)%4
	case ISTORE_0 <= op && op < ISTORE_0+20:
		in.Op, in.Local = ISTORE+(op-ISTORE_0)/4, int(op-ISTORE_0)%4
	case op == IINC:
		in.Local, in.Int = u1(1), int64(int8(code[off+2]))
	case IFEQ <= op && op <= GOTO, op == IFNULL || op == IFNONNULL:
		in.Target = off + s2(1)
	case op == GOTO_W:
		in.Op, in.Target = GOTO, off+int(s4(1))
	case op == JSR || op == JSR_W || op == RET:
		return in, &UnsupportedError{op, "jsr and ret"}
	case op == TABLESWITCH || op == LOOKUPSWITCH:
		pad := 3 - off%4
		if !need(1 + pad + 12) {
			return in, fmt.Errorf("truncated switch at offset %d", off)
		}
		at := 1 + pad
		in.Target = off + int(s4(at))
		if op == TABLESWITCH {
			low, high := s4(at+4), s4(at+8)
			n := int(high) - int(low) + 1
			if n < 0 || !need(at+12+4*n) {
				return in, fmt.Errorf("bad tableswitch at offset %d", off)
			}
			for i := range n {
				in.Keys = append(in.Keys, low+int32(i))
				in.Targets = append(in.Targets, off+int(s4(at+12+4*i)))
			}
			in.Len = at + 12 + 4*n
		} else {
			n := int(s4(at + 4))
			if n < 0 || !need(at+8+8*n) {
				return in, fmt.Errorf("bad lookupswitch at offset %d", off)
			}
			for i := range n {
				in.Keys = append(in.Keys, s4(at+8+8*i))
				in.Targets = append(in.Targets, off+int(s4(at+12+8*i)))
			}
			in.Len = at + 8 + 8*n
		}
	case GETSTATIC <= op && op <= INVOKEINTERFACE:
		in.Owner, in.Name, in.Desc, in.Iface, err = p.ref(u2(1))
	case op == INVOKEDYNAMIC:
		in.Name, in.Desc, in.Bootstrap, err = p.invokeDynamic(u2(1))
	case op == NEW || op == ANEWARRAY || op == CHECKCAST || op == INSTANCEOF:
		in.Class, err = p.class(u2(1))
	case op == NEWARRAY:
		in.Int = int64(u1(1))
	case op == MULTIANEWARRAY:
		in.Class, err = p.class(u2(1))
		in.Int = int64(u1(3))
	case op == WIDE:
		if !need(4) {
			return in, fmt.Errorf("truncated wide instruction at offset %d", off)
		}
		in.Op, in.Local, in.Len = code[off+1], int(u2(2)), 4
		switch {
		case in.Op == IINC:
			if !need(6) {
				return in, fmt.Errorf("truncated wide instruction at offset %d", off)
			}
			in.Int, in.Len = int64(int16(u2(4))), 6
		case in.Op == RET:
			return in, &UnsupportedError{RET, "jsr and ret"}
		case !(ILOAD <= in.Op && in.Op <= ALOAD || ISTORE <= in.Op && in.Op <= ASTORE):
			return in, fmt.Errorf("bad wide instruction at offset %d", off)
		}
	}
	return in, err
}
