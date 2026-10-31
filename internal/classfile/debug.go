package classfile

import "slices"

// Debug information: the LineNumberTable and LocalVariableTable
// attributes of a method's code.

type lineEntry struct{ pc, line int }

// localVar is an entry of the LocalVariableTable: a named local, from the
// instruction after its first store (or the method's start, for this and
// the parameters) to the end of its scope. end is -1 while the scope is
// open, and start is -1 until the first store.
type localVar struct {
	start, end, slot int
	name             string
}

// Line records that the code from here on comes from source line n. A
// line of 0 (a node without a position) and lines in unreachable code
// are ignored.
func (c *Code) Line(n int) {
	if n <= 0 || !c.reachable {
		return
	}
	pc := len(c.buf)
	if k := len(c.lines); k > 0 {
		last := &c.lines[k-1]
		if last.line == n {
			return
		}
		if last.pc == pc {
			last.line = n
			if k > 1 && c.lines[k-2].line == n {
				c.lines = c.lines[:k-1]
			}
			return
		}
	}
	c.lines = append(c.lines, lineEntry{pc, n})
}

// LastLine returns the line recorded last, or 0.
func (c *Code) LastLine() int {
	if len(c.lines) == 0 {
		return 0
	}
	return c.lines[len(c.lines)-1].line
}

// NameLocal gives an allocated local a name in the LocalVariableTable.
// Its range starts now if it holds a value (this and the parameters),
// otherwise after its next store, and ends with the scope it is named in
// (see Scope and EndScope), or with the method.
func (c *Code) NameLocal(slot int, name string) {
	if name == "" || slot >= len(c.descs) || c.descs[slot] == "" {
		return
	}
	start := -1
	if c.assigned.has(slot) {
		start = len(c.buf)
	}
	c.vars = append(c.vars, localVar{start: start, end: -1, slot: slot, name: name})
}

// stored starts the range of a named local that had no value yet.
func (c *Code) stored(slot int) {
	for i := len(c.vars) - 1; i >= 0; i-- {
		v := &c.vars[i]
		if v.slot == slot {
			if v.start < 0 && v.end < 0 {
				v.start = len(c.buf)
			}
			return
		}
	}
}

// Scope returns a mark for EndScope: the locals named after it belong to
// the scope that EndScope closes.
func (c *Code) Scope() int { return len(c.vars) }

// EndScope ends the ranges of the locals named since mark.
func (c *Code) EndScope(mark int) {
	for i := mark; i < len(c.vars); i++ {
		if c.vars[i].end < 0 {
			c.vars[i].end = len(c.buf)
		}
	}
}

// debugAttrs returns the encoded LineNumberTable and LocalVariableTable
// entries (without the attribute headers), each nil if it has none.
func (c *Code) debugAttrs() (lines, vars []byte, nlines, nvars int) {
	n := len(c.buf)
	for _, e := range c.lines {
		if e.pc >= n {
			break
		}
		lines = u2(u2(lines, uint16(e.pc)), uint16(e.line))
		nlines++
	}
	vs := c.vars
	if c.hasThis && n > 0 && !slices.ContainsFunc(vs, func(v localVar) bool { return v.slot == 0 }) {
		vs = append([]localVar{{start: 0, end: n, slot: 0, name: "this"}}, vs...)
	}
	for _, v := range vs {
		desc := "L" + c.class + ";"
		if v.slot < len(c.descs) && c.descs[v.slot] != "" {
			desc = c.descs[v.slot]
		}
		end := v.end
		if end < 0 || end > n {
			end = n
		}
		if v.start < 0 || v.start >= end {
			continue
		}
		vars = u2(vars, uint16(v.start))
		vars = u2(vars, uint16(end-v.start))
		vars = u2(vars, c.pool.Utf8(v.name))
		vars = u2(vars, c.pool.Utf8(desc))
		vars = u2(vars, uint16(v.slot))
		nvars++
	}
	return
}

// LineNumbers returns the method's line table as pairs of a code offset
// and a line, for tests.
func (c *Code) LineNumbers() [][2]int {
	var out [][2]int
	for _, e := range c.lines {
		if e.pc < len(c.buf) {
			out = append(out, [2]int{e.pc, e.line})
		}
	}
	return out
}
