package psi

import "slices"

// attempt is a speculative parse at a token index, with the state the
// parse reads besides the token kinds: the newline and token joining
// modes, and whether a by-clause's expressions take trailing lambdas.
type attempt struct {
	what                      attemptKind
	tok                       int
	newlines, join, byNoTrail bool
}

type attemptKind uint8

const (
	parenthesizedType attemptKind = iota
	valueParameter
	typedValueParameter
)

// failure is a failed attempt: the kinds of the tokens it read, from
// index lo, and the kinds it set, which its rollback kept.
type failure struct {
	lo     int
	kinds  []Kind
	remaps []remap
}

// try runs parse, which reports whether it succeeded and rolls back when it
// did not. A failed attempt is not run again at the same token index and
// state while the tokens it read have the same kinds, as it would fail the
// same way: only the kinds it set are set again. Otherwise an attempt nested
// in the text that a failed attempt rolled back runs once per level of
// nesting, and the time doubles with each level. The kinds can differ, as
// remapCurrentToken turns a soft keyword into an identifier and back.
func (p *kotlinParsing) try(what attemptKind, parse func() bool) bool {
	b := p.b.base()
	by := p.expr.byClause
	at := attempt{what, b.cur, b.newlines[len(b.newlines)-1], b.joining(), by != nil && by.stackSize <= 0}
	if f, ok := p.failed[at]; ok && slices.Equal(b.types[f.lo:f.lo+len(f.kinds)], f.kinds) {
		b.readLo = min(b.readLo, f.lo)
		b.readHi = max(b.readHi, f.lo+len(f.kinds)-1)
		for _, r := range f.remaps {
			if b.attempts > 0 {
				b.remaps = append(b.remaps, remap{r.tok, b.types[r.tok], r.kind})
			}
			b.types[r.tok] = r.kind
		}
		return false
	}
	lo, hi := b.readLo, b.readHi
	b.readLo, b.readHi = b.cur, b.cur-1
	start := len(b.remaps)
	b.attempts++
	ok := parse()
	b.attempts--
	if !ok {
		remaps := slices.Clone(b.remaps[start:])
		kinds := slices.Clone(b.types[b.readLo : b.readHi+1])
		for i := len(remaps) - 1; i >= 0; i-- {
			if r := remaps[i]; r.tok >= b.readLo && r.tok <= b.readHi {
				kinds[r.tok-b.readLo] = r.old
			}
		}
		if p.failed == nil {
			p.failed = map[attempt]failure{}
		}
		p.failed[at] = failure{b.readLo, kinds, remaps}
	}
	b.readLo, b.readHi = min(lo, b.readLo), max(hi, b.readHi)
	if b.attempts == 0 {
		b.remaps = b.remaps[:0]
	}
	return ok
}
