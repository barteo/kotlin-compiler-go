package codegen

import (
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// HotSpot doesn't compile a loop on stack replacement (OSR) when its
// backward branch has values on the operand stack, so a method that runs
// one long loop stays in the interpreter. Values are left there across
// code the call doesn't see: a receiver or earlier argument under an
// inlined call, s in an inlined s += f(i) where f's lambda loops. As
// kotlinc does, every inlined body and inlined lambda starts with the
// stack moved to locals (spillStack), which are put back below its result
// after it (unspill); so does a loop statement inside an expression, as
// a when's branch that is an argument.

// spillBase returns the stack depth that a spill keeps: the deepest that
// jumps from the code being generated pop to, the depth where an
// enclosing loop, inlined body or inlined lambda starts. The values below
// it belong to those, whose jumps out expect them there. Since those
// spill too, it is usually 0.
func (g *gen) spillBase() int {
	base := 0
	for _, l := range g.loops {
		base = max(base, l.depth)
	}
	for _, s := range g.srcInl {
		base = max(base, s.depth)
	}
	for _, l := range g.inl {
		base = max(base, l.depth)
	}
	return min(base, g.code.Depth())
}

// spillStack moves the values on the stack above spillBase to new locals,
// for unspill. An object that new created and isn't constructed yet
// can't be stored: the values above the topmost one are moved, and it
// reports false. Constructor calls whose arguments inline code evaluate
// them before the new (see ctorArgsFirst), so that this is rare.
func (g *gen) spillStack() ([]int, bool) {
	c := g.code
	if !c.Reachable() {
		return nil, true
	}
	base, ok := g.spillBase(), true
	stack := c.Stack()
	for i := len(stack) - 1; i >= base; i-- {
		if stack[i].Tag() == cf.VUninit {
			base, ok = i+1, false
			break
		}
	}
	slots, _ := c.SpillAbove(base)
	return slots, ok
}

// unspill puts the values that spillStack moved back on the stack, below
// the inlined code's result, of JVM descriptor ret ("V" for none).
func (g *gen) unspill(slots []int, ret string) {
	c := g.code
	if len(slots) == 0 || !c.Reachable() {
		return
	}
	tmp := -1
	if ret != "V" {
		tmp = c.AllocLocal(ret)
		c.Local(storeOpOf(ret), tmp)
	}
	c.Unspill(slots)
	if tmp >= 0 {
		c.Local(loadOpOf(ret), tmp)
		c.Retire(tmp)
	}
}

// retDesc is the JVM descriptor of a value of type t on the stack, "V"
// for none.
func retDesc(t types.Type) string {
	if size(t) == 0 {
		return "V"
	}
	return desc(t)
}
