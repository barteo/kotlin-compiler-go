package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// suppressible maps the names of the errors that @Suppress may name to
// the start of kc's message for each, or, with a leading '*', a part of
// it. kotlinc reports a suppressed error as a warning (ERROR_SUPPRESSION).
var suppressible = map[string]string{
	"ARGUMENT_TYPE_MISMATCH": "type mismatch: inferred type is ",
	"NOTHING_TO_OVERRIDE":    "*' overrides nothing",
	"UPPER_BOUND_VIOLATED":   "type argument is not within its bounds",
}

// suppressRange is the lines of a declaration whose @Suppress names
// errors, from its first annotation to its last line.
type suppressRange struct {
	from, to syntax.Pos
	names    []string
}

// suppressErrors drops the errors that the @Suppress of a declaration they
// are in, or of their file, names.
func (c *Checker) suppressErrors() {
	var rs []suppressRange
	if ns := suppressedNames(c.file.Annotations); len(ns) > 0 {
		rs = append(rs, suppressRange{syntax.Pos{Line: 1}, syntax.Pos{Line: 1 << 30}, ns})
	}
	for _, d := range c.file.Decls {
		rs = suppressRanges(d, rs)
	}
	if len(rs) == 0 {
		return
	}
	kept := c.errs[:0]
	for _, e := range c.errs {
		// An unsupported feature isn't kotlinc's error to suppress.
		if e.Feature != "" || !suppressedBy(e, rs) {
			kept = append(kept, e)
		}
	}
	c.errs = kept
}

func suppressedBy(e *syntax.Error, rs []suppressRange) bool {
	for _, r := range rs {
		if before(e.Pos, r.from) || before(r.to, e.Pos) {
			continue
		}
		for _, n := range r.names {
			m := suppressible[n]
			if rest, ok := strings.CutPrefix(m, "*"); ok && strings.Contains(e.Msg, rest) || !ok && strings.HasPrefix(e.Msg, m) {
				return true
			}
		}
	}
	return false
}

func before(a, b syntax.Pos) bool { return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col }

// suppressRanges adds the ranges of declaration d and its members to rs.
func suppressRanges(d syntax.Decl, rs []suppressRange) []suppressRange {
	add := func(anns []syntax.Annotation, p, end syntax.Pos) {
		ns := suppressedNames(anns)
		if len(ns) == 0 {
			return
		}
		from := p
		for _, a := range anns {
			if before(a.P, from) {
				from = a.P
			}
		}
		if before(end, p) {
			end = syntax.Pos{Line: p.Line, Col: 1 << 30}
		}
		rs = append(rs, suppressRange{from, end, ns})
	}
	switch d := d.(type) {
	case *syntax.FunDecl:
		add(d.Annotations, d.P, d.End)
	case *syntax.PropDecl:
		end := syntax.Pos{}
		for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
			if a != nil && before(end, a.End) {
				end = a.End
			}
		}
		add(d.Annotations, d.P, end)
	case *syntax.ClassDecl:
		add(d.Annotations, d.P, d.End)
		for _, m := range d.Members {
			rs = suppressRanges(m, rs)
		}
	}
	return rs
}

// suppressedNames returns the names of the errors kc can suppress that
// annotations anns have a @Suppress of.
func suppressedNames(anns []syntax.Annotation) []string {
	var ns []string
	for n := range suppressible {
		if suppresses(anns, n) {
			ns = append(ns, n)
		}
	}
	return ns
}
