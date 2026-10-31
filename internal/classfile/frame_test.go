package classfile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// frames formats the frames of a finished method, one per line, as
// "offset: [locals] [stack]".
func frames(t *testing.T, c *Code) string {
	t.Helper()
	if err := c.Err(); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, f := range c.frameList() {
		fmt.Fprintf(&sb, "%d: %v %v\n", f.pos, f.locals, f.stack)
	}
	return sb.String()
}

func check(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("frames:\n%s\nwant:\n%s", got, want)
	}
}

// loop: while (n > 0) { val s = "x"; n-- }. s holds a value only inside
// the body, so it is not in the frames. It has line numbers, from line 10,
// and its locals are named.
func loop(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "loop", "(I)V")
	n := code.AllocLocal("I")
	code.NameLocal(n, "n")
	top, exit := NewLabel(), NewLabel()
	code.Place(top)
	code.Line(10)
	code.Local(ILOAD, n)
	code.Jump(IFLE, exit)
	scope := code.Scope()
	s := code.AllocLocal("Ljava/lang/String;")
	code.NameLocal(s, "s")
	code.Line(11)
	code.Sconst("x")
	code.Local(ASTORE, s)
	code.Line(12)
	code.Iinc(n, -1)
	code.EndScope(scope)
	code.Line(10)
	code.Jump(GOTO, top)
	code.Place(exit)
	code.Line(14)
	code.Op(RETURN)
	code.Line(15) // after the last instruction: dropped
	return code
}

// TestDebugInfo checks loop's line and local variable tables, as written
// and read back.
func TestDebugInfo(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "DebugTest", "java/lang/Object")
	code := loop(c)
	want := [][2]int{{0, 10}, {5, 11}, {9, 12}, {12, 10}, {15, 14}}
	if got := code.LineNumbers(); !slices.Equal(got, want) {
		t.Errorf("lines %v, want %v", got, want)
	}
	_, vars, _, nvars := code.debugAttrs()
	if nvars != 2 {
		t.Errorf("%d locals, want n and s", nvars)
	}
	// s: from after its store (9) to the end of its scope (12), in slot 1.
	if len(vars) != 20 || vars[10] != 0 || vars[11] != 9 || vars[13] != 3 || vars[19] != 1 {
		t.Errorf("local variable table % x", vars)
	}
	m, err := ReadMethod(c.Bytes(), "loop", "(I)V")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Lines, want) {
		t.Errorf("read lines %v, want %v", m.Lines, want)
	}
}

// pick: if (b) "s" else Integer.valueOf(1), as Any.
func pick(c *Class, end *Label) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "pick", "(Z)Ljava/lang/Object;")
	code.AllocLocal("Z")
	els := NewLabel()
	code.Local(ILOAD, 0)
	code.Jump(IFEQ, els)
	code.Sconst("s")
	code.Jump(GOTO, end)
	code.Place(els)
	code.Iconst(1)
	code.Invoke(INVOKESTATIC, "java/lang/Integer", "valueOf", "(I)Ljava/lang/Integer;")
	code.Place(end)
	code.Op(ARETURN)
	return code
}

// build: StringBuilder(if (b) "a" else "b"), which branches while the
// new object is uninitialized.
func build(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "build", "(Z)Ljava/lang/Object;")
	code.AllocLocal("Z")
	els, end := NewLabel(), NewLabel()
	code.New("java/lang/StringBuilder")
	code.Op(DUP)
	code.Local(ILOAD, 0)
	code.Jump(IFEQ, els)
	code.Sconst("a")
	code.Jump(GOTO, end)
	code.Place(els)
	code.Sconst("b")
	code.Place(end)
	code.Invoke(INVOKESPECIAL, "java/lang/StringBuilder", "<init>", "(Ljava/lang/String;)V")
	code.Op(ARETURN)
	return code
}

// superArg: class SuperArg(b: Boolean) : Exception(if (b) "a" else "b"),
// which branches before the super constructor call.
func superArg() *Class {
	c := NewClass(AccPublic|AccSuper, "SuperArg", "java/lang/Exception")
	code := c.NewMethod(AccPublic, "<init>", "(Z)V")
	code.AllocLocal("LSuperArg;")
	code.AllocLocal("Z")
	els, end := NewLabel(), NewJoin("Ljava/lang/String;")
	code.Local(ALOAD, 0)
	code.Local(ILOAD, 1)
	code.Jump(IFEQ, els)
	code.Sconst("a")
	code.Jump(GOTO, end)
	code.Place(els)
	code.Sconst("b")
	code.Place(end)
	code.Invoke(INVOKESPECIAL, "java/lang/Exception", "<init>", "(Ljava/lang/String;)V")
	code.Op(RETURN)
	return c
}

// sumArray: var s = 0; for (x in a) s += x, over an IntArray by index.
func sumArray(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "sumArray", "([I)I")
	a := code.AllocLocal("[I")
	s, i := code.AllocLocal("I"), code.AllocLocal("I")
	code.Iconst(0)
	code.Local(ISTORE, s)
	code.Iconst(0)
	code.Local(ISTORE, i)
	top, exit := NewLabel(), NewLabel()
	code.Place(top)
	code.Local(ILOAD, i)
	code.Local(ALOAD, a)
	code.Op(ARRAYLENGTH)
	code.Jump(IF_ICMPGE, exit)
	code.Local(ILOAD, s)
	code.Local(ALOAD, a)
	code.Local(ILOAD, i)
	code.Op(IALOAD)
	code.Op(IADD)
	code.Local(ISTORE, s)
	code.Iinc(i, 1)
	code.Jump(GOTO, top)
	code.Place(exit)
	code.Local(ILOAD, s)
	code.Op(IRETURN)
	return code
}

// clamp: var x = f; if (x > 1f) x = 1f; return x, with a float local
// and fcmpl.
func clamp(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "clamp", "(F)F")
	f := code.AllocLocal("F")
	x := code.AllocLocal("F")
	code.Local(FLOAD, f)
	code.Local(FSTORE, x)
	done := NewLabel()
	code.Local(FLOAD, x)
	code.Fconst(1)
	code.Op(FCMPL)
	code.Jump(IFLE, done)
	code.Fconst(1)
	code.Local(FSTORE, x)
	code.Place(done)
	code.Local(FLOAD, x)
	code.Op(FRETURN)
	return code
}

func TestFloatFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	check(t, frames(t, clamp(c)), "14: [F F] []\n")
}

func TestArrayFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	check(t, frames(t, sumArray(c)), "6: [[I I I] []\n30: [[I I I] []\n")
}

func TestUninitializedThisFrames(t *testing.T) {
	c := superArg()
	code := c.methods[0].code
	check(t, frames(t, code), "12: [uninitialized this I] [uninitialized this]\n14: [uninitialized this I] [uninitialized this java/lang/String]\n")
}

// safeParse: try { Integer.parseInt(s) } catch (e: NumberFormatException)
// { -1 }. The handler starts with the exception on the stack.
func safeParse(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "safeParse", "(Ljava/lang/String;)I")
	code.AllocLocal("Ljava/lang/String;")
	snap := code.Snapshot()
	end, h := NewLabel(), NewLabel()
	start := code.Pos()
	code.Local(ALOAD, 0)
	code.Invoke(INVOKESTATIC, "java/lang/Integer", "parseInt", "(Ljava/lang/String;)I")
	stop := code.Pos()
	code.Jump(GOTO, end)
	code.Catch(snap, [][2]int{{start, stop}, {stop, stop}}, h, "java/lang/NumberFormatException")
	code.Place(h)
	code.Op(POP)
	code.Iconst(-1)
	code.Place(end)
	code.Op(IRETURN)
	return code
}

func TestHandlerFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	code := safeParse(c)
	check(t, frames(t, code), "8: [java/lang/String] [java/lang/NumberFormatException]\n10: [java/lang/String] [I]\n")
	if len(code.handlers) != 1 {
		t.Errorf("%d handlers, want 1: the empty range is dropped", len(code.handlers))
	}
}

func TestLoopFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	check(t, frames(t, loop(c)), "0: [I] []\n15: [I] []\n")
}

func TestJoinFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	check(t, frames(t, pick(c, NewJoin("Ljava/lang/Object;"))), "10: [I] []\n14: [I] [java/lang/Object]\n")
}

func TestJoinNeedsType(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	err := pick(c, NewLabel()).Err()
	if err == nil || !strings.Contains(err.Error(), "has types java/lang/String and java/lang/Integer") {
		t.Errorf("got %v, want a type conflict", err)
	}
}

func TestUninitializedFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	u := "uninitialized java/lang/StringBuilder@0"
	check(t, frames(t, build(c)), fmt.Sprintf("14: [I] [%s %s]\n16: [I] [%[1]s %[2]s java/lang/String]\n", u, u))
}

func TestWideLocalFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	code := c.NewMethod(AccPublic|AccStatic, "wide", "(J)V")
	code.AllocLocal("J")
	i := code.AllocLocal("I")
	end := NewLabel()
	code.Iconst(0)
	code.Local(ISTORE, i)
	code.Local(LLOAD, 0)
	code.Lconst(0)
	code.Op(LCMP)
	code.Jump(IFEQ, end)
	code.Iinc(i, 1)
	code.Place(end)
	code.Op(RETURN)
	check(t, frames(t, code), "13: [J I] []\n")
}

// Two labels at one offset get one frame, with the locals assigned on
// every edge to either.
func TestSharedOffsetFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	code := c.NewMethod(AccPublic|AccStatic, "shared", "(Z)V")
	code.AllocLocal("Z")
	x := code.AllocLocal("I")
	a, b := NewLabel(), NewLabel()
	code.Local(ILOAD, 0)
	code.Jump(IFEQ, b)
	code.Iconst(1)
	code.Local(ISTORE, x)
	code.Local(ILOAD, 0)
	code.Jump(IFNE, a)
	code.Place(a)
	code.Place(b)
	code.Op(RETURN)
	check(t, frames(t, code), "13: [I] []\n")
}

// classify: when (n) { 0, 1, 2 -> 10 + n; 5 -> 50; 1000 -> 99; else -> -1 },
// with a tableswitch whose default is a lookupswitch.
func classify(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "classify", "(I)I")
	n := code.AllocLocal("I")
	cases := []*Label{NewLabel(), NewLabel(), NewLabel()}
	other, five, big, none, end := NewLabel(), NewLabel(), NewLabel(), NewLabel(), NewLabel()
	code.Local(ILOAD, n)
	code.TableSwitch(0, other, cases)
	for i, l := range cases {
		code.Place(l)
		code.Iconst(int32(10 + i))
		code.Jump(GOTO, end)
	}
	code.Place(other)
	code.Local(ILOAD, n)
	code.LookupSwitch(none, []int32{5, 1000}, []*Label{five, big})
	code.Place(five)
	code.Iconst(50)
	code.Jump(GOTO, end)
	code.Place(big)
	code.Iconst(99)
	code.Jump(GOTO, end)
	code.Place(none)
	code.Iconst(-1)
	code.Place(end)
	code.Op(IRETURN)
	return code
}

// count: var i = 0; while (i < n) i++; return i, tested at the bottom:
// the body's label is placed after a goto, before anything jumps to it,
// and has its frame given.
func count(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "count", "(I)I")
	n := code.AllocLocal("I")
	i := code.AllocLocal("I")
	code.Iconst(0)
	code.Local(ISTORE, i)
	body, test := code.NewFrameLabel(nil, []int{n, i}), code.NewFrameLabel(nil, []int{n, i})
	code.Jump(GOTO, test)
	code.Place(body)
	code.Iinc(i, 1)
	code.Place(test)
	code.Local(ILOAD, i)
	code.Local(ILOAD, n)
	code.Jump(IF_ICMPLT, body)
	code.Local(ILOAD, i)
	code.Op(IRETURN)
	return code
}

// ops: the stack instructions, shifts and bitwise operators, and a
// monitor, which return (b - a) + dup_x2 + dup2_x1 + dup2_x2 + bits, all
// of whose parts are 1 for a = 1, b = 2.
func ops(c *Class) *Code {
	code := c.NewMethod(AccPublic|AccStatic, "ops", "(II)I")
	a, b := code.AllocLocal("I"), code.AllocLocal("I")
	code.Local(ILOAD, a)
	code.Local(ILOAD, b)
	code.Op(SWAP)
	code.Op(ISUB) // b - a
	// [3, 1, 2] dup_x2: [2, 3, 1, 2], then 2 - (3 - (1 - 2)) = -2, plus 3.
	code.Iconst(3)
	code.Iconst(1)
	code.Iconst(2)
	code.Op(DUP_X2)
	code.Op(ISUB)
	code.Op(ISUB)
	code.Op(ISUB)
	code.Iconst(3)
	code.Op(IADD)
	code.Op(IADD)
	// [1, 2, 3] dup2_x1: [2, 3, 1, 2, 3], then 2 - (3 - (1 - (2 - 3))) = 1.
	code.Iconst(1)
	code.Iconst(2)
	code.Iconst(3)
	code.Op(DUP2_X1)
	for range 4 {
		code.Op(ISUB)
	}
	code.Op(IADD)
	// [5L, 7L] dup2_x2: [7L, 5L, 7L], then 7 - (5 - 7) = 9, minus 8.
	code.Lconst(5)
	code.Lconst(7)
	code.Op(DUP2_X2)
	code.Op(LSUB)
	code.Op(LSUB)
	code.Lconst(8)
	code.Op(LSUB)
	code.Op(L2I)
	code.Op(IADD)
	// ((a << 3 | b) >> 1 >>> 2) ^ 0 = 1, and the same with longs, & 1.
	code.Local(ILOAD, a)
	code.Iconst(3)
	code.Op(ISHL)
	code.Local(ILOAD, b)
	code.Op(IOR)
	code.Iconst(1)
	code.Op(ISHR)
	code.Iconst(2)
	code.Op(IUSHR)
	code.Iconst(0)
	code.Op(IXOR)
	code.Op(IADD)
	code.Lconst(1)
	code.Iconst(3)
	code.Op(LSHL)
	code.Lconst(2)
	code.Op(LOR)
	code.Iconst(1)
	code.Op(LSHR)
	code.Iconst(2)
	code.Op(LUSHR)
	code.Lconst(0)
	code.Op(LXOR)
	code.Lconst(1)
	code.Op(LAND)
	code.Op(L2I)
	code.Op(IADD)
	// A monitor around nothing.
	lock := code.AllocLocal("Ljava/lang/Object;")
	code.New("java/lang/Object")
	code.Op(DUP)
	code.Invoke(INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	code.Op(DUP)
	code.Local(ASTORE, lock)
	code.Op(MONITORENTER)
	code.Local(ALOAD, lock)
	code.Op(MONITOREXIT)
	code.Op(IRETURN)
	return code
}

func TestSwitchFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	code := classify(c)
	check(t, frames(t, code), "28: [I] []\n33: [I] []\n38: [I] []\n43: [I] []\n72: [I] []\n77: [I] []\n82: [I] []\n83: [I] [I]\n")
	// The tableswitch at 2 is padded to 4; its default is at 43.
	if got := code.buf[4:8]; got[3] != 41 {
		t.Errorf("default offset %v", got)
	}
}

func TestFrameLabelFrames(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	check(t, frames(t, count(c)), "6: [I I] []\n9: [I I] []\n")
}

func TestFrameLabelChecks(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	code := c.NewMethod(AccPublic|AccStatic, "bad", "(I)V")
	n := code.AllocLocal("I")
	x := code.AllocLocal("I")
	l := code.NewFrameLabel(nil, []int{n, x})
	code.Local(ILOAD, n)
	code.Jump(IFEQ, l) // x holds no value yet
	code.Place(l)
	code.Op(RETURN)
	if err := code.Err(); err == nil || !strings.Contains(err.Error(), "fewer locals") {
		t.Errorf("got %v, want an error for the unassigned local", err)
	}
}

func TestOpStack(t *testing.T) {
	c := NewClass(AccPublic|AccSuper, "T", "java/lang/Object")
	code := ops(c)
	if err := code.Err(); err != nil {
		t.Fatal(err)
	}
	if code.max != 7 {
		t.Errorf("max stack %d, want 7", code.max)
	}
	code = c.NewMethod(AccPublic|AccStatic, "bad", "()V")
	code.Op(IINC)
	if err := code.Err(); err == nil || !strings.Contains(err.Error(), "not an instruction without operands") {
		t.Errorf("got %v, want an error for an opcode Op doesn't model", err)
	}
}

// TestVerify runs the methods above on the JVM with full verification.
func TestVerify(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	c := NewClass(AccPublic|AccSuper, "FrameTest", "java/lang/Object")
	loop(c)
	pick(c, NewJoin("Ljava/lang/Object;"))
	build(c)
	safeParse(c)
	sumArray(c)
	clamp(c)
	classify(c)
	count(c)
	ops(c)
	code := c.NewMethod(AccPublic|AccStatic, "main", "([Ljava/lang/String;)V")
	code.AllocLocal("[Ljava/lang/String;")
	println := func(push func(), d string) {
		code.FieldInsn(GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
		push()
		code.Invoke(INVOKEVIRTUAL, "java/io/PrintStream", "println", "("+d+")V")
	}
	for _, n := range []int32{0, 2, 3, 5, 1000} {
		println(func() {
			code.Iconst(n)
			code.Invoke(INVOKESTATIC, "FrameTest", "classify", "(I)I")
		}, "I")
	}
	println(func() {
		code.Iconst(4)
		code.Invoke(INVOKESTATIC, "FrameTest", "count", "(I)I")
	}, "I")
	println(func() {
		code.Iconst(1)
		code.Iconst(2)
		code.Invoke(INVOKESTATIC, "FrameTest", "ops", "(II)I")
	}, "I")
	for _, f := range []float32{0.5, 3} {
		code.FieldInsn(GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
		code.Fconst(f)
		code.Invoke(INVOKESTATIC, "FrameTest", "clamp", "(F)F")
		code.Invoke(INVOKEVIRTUAL, "java/io/PrintStream", "println", "(F)V")
	}
	code.FieldInsn(GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
	code.Iconst(3)
	code.NewArray(TInt)
	for i := range int32(3) {
		code.Op(DUP)
		code.Iconst(i)
		code.Iconst(i + 1)
		code.Op(IASTORE)
	}
	code.Invoke(INVOKESTATIC, "FrameTest", "sumArray", "([I)I")
	code.Invoke(INVOKEVIRTUAL, "java/io/PrintStream", "println", "(I)V")
	code.Iconst(3)
	code.Invoke(INVOKESTATIC, "FrameTest", "loop", "(I)V")
	for _, s := range []string{"12", "x"} {
		code.FieldInsn(GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
		code.Sconst(s)
		code.Invoke(INVOKESTATIC, "FrameTest", "safeParse", "(Ljava/lang/String;)I")
		code.Invoke(INVOKEVIRTUAL, "java/io/PrintStream", "println", "(I)V")
	}
	for _, b := range []int32{1, 0} {
		code.FieldInsn(GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
		code.New("SuperArg")
		code.Op(DUP)
		code.Iconst(b)
		code.Invoke(INVOKESPECIAL, "SuperArg", "<init>", "(Z)V")
		code.Invoke(INVOKEVIRTUAL, "SuperArg", "getMessage", "()Ljava/lang/String;")
		code.Invoke(INVOKEVIRTUAL, "java/io/PrintStream", "println", "(Ljava/lang/String;)V")
	}
	for _, m := range []string{"pick", "build"} {
		for _, b := range []int32{1, 0} {
			code.FieldInsn(GETSTATIC, "java/lang/System", "out", "Ljava/io/PrintStream;")
			code.Iconst(b)
			code.Invoke(INVOKESTATIC, "FrameTest", m, "(Z)Ljava/lang/Object;")
			code.Invoke(INVOKEVIRTUAL, "java/io/PrintStream", "println", "(Ljava/lang/Object;)V")
		}
	}
	code.Op(RETURN)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "FrameTest.class"), c.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SuperArg.class"), superArg().Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(java, "-Xverify:all", "-cp", dir, "FrameTest").CombinedOutput()
	if err != nil {
		t.Fatalf("java: %v\n%s", err, out)
	}
	if got := string(out); got != "10\n12\n-1\n50\n99\n4\n6\n0.5\n1.0\n6\n12\n-1\na\nb\ns\n1\na\nb\n" {
		t.Errorf("output %q", got)
	}
}
