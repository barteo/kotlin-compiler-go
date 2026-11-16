package driver

import (
	"cmp"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// syntaxErrors returns the errors to report for a file that failed to
// parse with errs, kc's own: every syntax error kotlinc reports, from
// the recovering parser in psi (s, parsed if nil), as kotlinc reports one
// for each error element, and those of lower.TreeErrors. kc's errors that psi has no counterpart for are
// kept: those about features kc doesn't support, and, on lines psi
// reports nothing on, the errors parsing went on after (which kotlinc
// reports in later phases) and the error kc's parser stopped at if it is
// before psi's first.
func syntaxErrors(path, src string, s *psi.Snapshot, errs []*syntax.Error) []*syntax.Error {
	if s == nil {
		s = psi.Parse(path, src)
	}
	var out []*syntax.Error
	for _, d := range s.Diagnostics() {
		off := d.Range.Start
		if off == len(src) {
			// kotlinc reports an error at the end of the file before the
			// whitespace that ends it.
			off = len(strings.TrimRight(src, " \t\r\n"))
		}
		line, col, err := s.Position(off)
		if err != nil {
			continue
		}
		out = append(out, &syntax.Error{File: path, Pos: syntax.Pos{Line: line, Col: col}, Msg: d.Message})
	}
	// What kotlinc's grammar accepts and its raw FIR builder doesn't:
	// kc's parser stops at the first.
	out = append(out, lower.TreeErrors(s)...)
	lines := map[int]bool{}
	var first *syntax.Error // psi's first error
	for _, e := range out {
		lines[e.Pos.Line] = true
		if first == nil || before(e.Pos, first.Pos) {
			first = e
		}
	}
	for i, e := range errs {
		// errs[0] is the error kc's parser stopped at. After psi's first
		// error it is most likely a consequence of that one; before it,
		// kc's parser is stricter than kotlinc's, which reports most such
		// code in later phases.
		stop := i == 0 && first != nil && !before(e.Pos, first.Pos)
		if e.Feature != "" || !stop && !lines[e.Pos.Line] {
			out = append(out, e)
		}
	}
	slices.SortStableFunc(out, func(a, b *syntax.Error) int {
		return cmp.Or(cmp.Compare(a.Pos.Line, b.Pos.Line), cmp.Compare(a.Pos.Col, b.Pos.Col))
	})
	return slices.CompactFunc(out, func(a, b *syntax.Error) bool {
		return a.Pos.Line == b.Pos.Line && a.Msg == b.Msg
	})
}

// before reports whether a is before b.
func before(a, b syntax.Pos) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
}
