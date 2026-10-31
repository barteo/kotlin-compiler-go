package classfile

import "sort"

// Liveness of locals.
//
// When TraceLiveness is on, the code records, as it is emitted, what
// liveness needs of each instruction: the local it reads or writes, the
// labels it jumps to, and whether control falls through. LiveAt then
// tells whether a local's value may still be read after a point, for the
// state machines of suspend functions, which don't keep dead values.

// traceKind is what a traced instruction does.
type traceKind uint8

const (
	traceUse    traceKind = iota // reads the local
	traceDef                     // writes the local
	traceUseDef                  // reads and writes it: iinc
	traceBranch                  // may jump to its labels, or fall through
	traceGoto                    // jumps to its labels
	traceEnd                     // returns or throws
)

type traceEntry struct {
	pos    int
	kind   traceKind
	slot   int
	labels []*Label
}

// TraceLiveness makes the code record what LiveAt needs.
func (c *Code) TraceLiveness() { c.tracing = true }

func (c *Code) trace(kind traceKind, slot int, labels ...*Label) {
	if c.tracing {
		c.traces = append(c.traces, traceEntry{pos: len(c.buf), kind: kind, slot: slot, labels: labels})
	}
}

// traceEnd records a return or a throw, which Op has emitted: at its
// opcode, not after it, where a jump target may start.
func (c *Code) traceEnd() {
	if c.tracing {
		c.traces = append(c.traces, traceEntry{pos: len(c.buf) - 1, kind: traceEnd, slot: -1})
	}
}

// LiveAt reports whether the value of local slot at code offset pos may
// be read later: on some path from pos, a load of the local comes before
// a store to it. Exceptions thrown in a handler's range go to the
// handler. The code must be complete.
func (c *Code) LiveAt(pos, slot int) bool {
	if c.live == nil {
		c.liveness()
	}
	i := sort.Search(len(c.traces), func(i int) bool { return c.traces[i].pos >= pos })
	return i < len(c.live) && c.live[i].has(slot)
}

// liveness computes the locals live before each traced instruction.
func (c *Code) liveness() {
	tr := c.traces
	n := len(tr)
	// at returns the first traced instruction at or after offset pos.
	at := func(pos int) int {
		return sort.Search(n, func(i int) bool { return tr[i].pos >= pos })
	}
	succ := make([][]int, n)
	for i, e := range tr {
		if e.kind != traceGoto && e.kind != traceEnd {
			succ[i] = append(succ[i], i+1)
		}
		for _, l := range e.labels {
			succ[i] = append(succ[i], at(l.pos))
		}
		for _, h := range c.handlers {
			if h.start <= e.pos && e.pos < h.end {
				succ[i] = append(succ[i], at(h.handler.pos))
			}
		}
	}
	live := make([]bitset, n+1)
	for changed := true; changed; {
		changed = false
		for i := n - 1; i >= 0; i-- {
			var out bitset
			for _, s := range succ[i] {
				out.or(live[s])
			}
			e := tr[i]
			switch e.kind {
			case traceDef:
				out.clear(e.slot)
			case traceUse, traceUseDef:
				out.set(e.slot)
			}
			if !live[i].equal(out) {
				live[i], changed = out, true
			}
		}
	}
	c.live = live
}

func (b *bitset) or(o bitset) {
	for len(*b) < len(o) {
		*b = append(*b, 0)
	}
	for i, w := range o {
		(*b)[i] |= w
	}
}

func (b bitset) clear(i int) {
	if i/64 < len(b) {
		b[i/64] &^= 1 << (i % 64)
	}
}

func (b bitset) equal(o bitset) bool {
	for i := 0; i < max(len(b), len(o)); i++ {
		var x, y uint64
		if i < len(b) {
			x = b[i]
		}
		if i < len(o) {
			y = o[i]
		}
		if x != y {
			return false
		}
	}
	return true
}

// Patch replaces n bytes of code at offset pos with nops: an instruction
// that turned out not to be needed.
func (c *Code) Patch(pos, n int) {
	for i := range n {
		c.buf[pos+i] = NOP
	}
}

// Retire records that a temporary local is dead: the code doesn't read
// it again.
func (c *Code) Retire(slot int) {
	if slot >= 0 {
		c.retired.set(slot)
	}
}

// RetireTemps retires the temporaries allocated since MaxLocals returned
// from: the locals without a name.
func (c *Code) RetireTemps(from int) {
	named := map[int]bool{}
	for _, v := range c.vars {
		named[v.slot] = true
	}
	for s := from; s < c.maxLocals; s++ {
		if s < len(c.descs) && c.descs[s] != "" && !named[s] {
			c.retired.set(s)
		}
	}
}

// MaxLocals returns the number of local slots allocated so far.
func (c *Code) MaxLocals() int { return c.maxLocals }

// Unretire undoes Retire: code that follows reads the local again.
func (c *Code) Unretire(slot int) {
	if slot >= 0 {
		c.retired.clear(slot)
	}
}

// Retired reports whether Retire was called for a local.
func (c *Code) Retired(slot int) bool { return c.retired.has(slot) }

// Named returns the name of a local, "" if it is a temporary, and
// whether its scope is open here.
func (c *Code) Named(slot int) (name string, inScope bool) {
	for i := len(c.vars) - 1; i >= 0; i-- {
		if v := c.vars[i]; v.slot == slot {
			return v.name, v.start >= 0 && v.end < 0
		}
	}
	return "", false
}

// NextLine returns the line of the first entry of the line table after
// code offset pos whose line isn't line, or -1.
func (c *Code) NextLine(pos, line int) int {
	for _, e := range c.lines {
		if e.pc > pos && e.line != line {
			return e.line
		}
	}
	return -1
}
