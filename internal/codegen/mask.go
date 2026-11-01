package codegen

import (
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Masks of omitted arguments.
//
// A call that leaves out arguments calls a function's name$default method,
// or a constructor for omitted arguments, with a mask whose bit i is set
// if the argument of parameter i is omitted, and an unused marker. As
// kotlinc writes them, context parameters come first in the bits, but not
// the receivers, and the mask is an int for every 32 parameters: a
// function of 70 parameters takes three.

// argMask is the mask of a call's omitted arguments, 32 bits a word. Nil
// is none.
type argMask []uint32

// objMarker and ctorMarker are the unused markers that name$default
// methods and constructors for omitted arguments take after their masks.
const (
	objMarker  = "Ljava/lang/Object;"
	ctorMarker = "Lkotlin/jvm/internal/DefaultConstructorMarker;"
)

// set sets bit i.
func (m *argMask) set(i int) {
	for len(*m) <= i/32 {
		*m = append(*m, 0)
	}
	(*m)[i/32] |= 1 << (i % 32)
}

// has reports whether bit i is set.
func (m argMask) has(i int) bool {
	return i >= 0 && i/32 < len(m) && m[i/32]&(1<<(i%32)) != 0
}

// any reports whether any bit is set.
func (m argMask) any() bool {
	for _, w := range m {
		if w != 0 {
			return true
		}
	}
	return false
}

// word returns the int of bits 32*k to 32*k+31.
func (m argMask) word(k int) int32 {
	if k < len(m) {
		return int32(m[k])
	}
	return 0
}

// bits31 returns the int of bits 31*k to 31*k+30, as a composable
// function's masks hold them.
func (m argMask) bits31(k int) int32 {
	var w int32
	for b := range 31 {
		if m.has(31*k + b) {
			w |= 1 << b
		}
	}
	return w
}

// allMask returns a mask with bits 0 to n-1 set.
func allMask(n int) argMask {
	var m argMask
	for i := range n {
		m.set(i)
	}
	return m
}

// defaultsDesc returns descriptor d with the masks of n parameters and
// marker, a descriptor, added after its parameters.
func defaultsDesc(d string, n int, marker string) string {
	i := strings.IndexByte(d, ')')
	return d[:i] + strings.Repeat("I", types.MaskInts(n)) + marker + d[i:]
}

// pushMask pushes the masks of mask of n parameters, and the marker, null.
func (g *gen) pushMask(m argMask, n int) {
	for k := range types.MaskInts(n) {
		g.code.Iconst(m.word(k))
	}
	g.code.Op(cf.ACONST_NULL)
}

// allocMasks allocates the locals of the masks of n parameters, and of the
// marker, of type marker, and returns the masks'.
func (g *gen) allocMasks(n int, marker string) []int {
	masks := make([]int, types.MaskInts(n))
	for k := range masks {
		masks[k] = g.code.AllocLocal("I")
	}
	g.code.AllocLocal(marker)
	return masks
}

// jumpIfPassed jumps to skip if bit i of the masks in locals masks is
// clear: the argument was passed.
func (g *gen) jumpIfPassed(masks []int, i int, skip *cf.Label) {
	g.code.Local(cf.ILOAD, masks[i/32])
	g.code.Iconst(int32(uint32(1) << (i % 32)))
	g.code.Op(cf.IAND)
	g.code.Jump(cf.IFEQ, skip)
}
