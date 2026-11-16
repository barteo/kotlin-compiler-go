package check

import (
	"fmt"
	"regexp"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// suppressible maps the names of the errors that @Suppress may name to
// kc's messages for each. kotlinc reports a suppressed error as a warning
// (ERROR_SUPPRESSION) and compiles the code. Errors whose code the
// backend can't generate, as an unresolved reference or a missing
// argument, aren't listed. kc's messages don't tell apart some errors
// kotlinc names differently: one type mismatch message is all of
// ARGUMENT_TYPE_MISMATCH, INITIALIZER_TYPE_MISMATCH and the others.
var suppressible = map[string]*regexp.Regexp{}

func init() {
	typeMismatch := `^type mismatch: inferred type is `
	notImplemented := `^(class '.*' is|object is) not abstract and does not implement abstract member `
	invisible := `^cannot access '.*': it is `
	for n, m := range map[string]string{
		"ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED":     notImplemented,
		"ABSTRACT_FUNCTION_IN_NON_ABSTRACT_CLASS":   `^abstract function '.*' in non-abstract class `,
		"ABSTRACT_MEMBER_NOT_IMPLEMENTED":           notImplemented,
		"ABSTRACT_PROPERTY_IN_NON_ABSTRACT_CLASS":   `^abstract property '.*' in non-abstract class `,
		"ARGUMENT_TYPE_MISMATCH":                    typeMismatch,
		"ASSIGNMENT_TYPE_MISMATCH":                  typeMismatch,
		"CONFLICTING_OVERLOADS":                     `^conflicting overloads: `,
		"CREATING_AN_INSTANCE_OF_ABSTRACT_CLASS":    `^cannot create an instance of an abstract class`,
		"DATA_CLASS_VARARG_PARAMETER":               `^primary constructor vararg parameters are forbidden for data classes`,
		"DEPRECATION_ERROR":                         `' is deprecated with level ERROR`,
		"EQUALITY_NOT_APPLICABLE":                   `^operator '(==|!=|===|!==)' cannot be applied to `,
		"INITIALIZER_TYPE_MISMATCH":                 typeMismatch,
		"INTERFACE_WITH_SUPERCLASS":                 `^an interface cannot inherit from a class`,
		"INVISIBLE_MEMBER":                          invisible,
		"INVISIBLE_REFERENCE":                       invisible,
		"METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE":    `^an interface may not implement a method of 'Any'`,
		"MUST_BE_INITIALIZED":                       `^property must be initialized`,
		"MUST_BE_INITIALIZED_OR_BE_ABSTRACT":        `^property must be initialized or be abstract`,
		"NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION": `^only 'const val' can be used in constant expressions`,
		"NOTHING_TO_OVERRIDE":                       `' overrides nothing$`,
		"OVERRIDING_FINAL_MEMBER":                   `' is final and cannot be overridden$`,
		"PROPERTY_WITH_NO_TYPE_NO_INITIALIZER":      `^this property must have a type, be initialized`,
		"REPEATED_ANNOTATION":                       `^this annotation is not repeatable`,
		"RETURN_TYPE_MISMATCH":                      typeMismatch,
		"TYPE_MISMATCH":                             typeMismatch,
		"UNSAFE_CALL":                               `^only safe \(\?\.\) or non-null asserted \(!!\.\) calls are allowed`,
		"UPPER_BOUND_VIOLATED":                      `^type argument is not within its bounds`,
		"VAL_REASSIGNMENT":                          `^val cannot be reassigned`,
		"VALUE_CLASS_CANNOT_EXTEND_CLASSES":         `^value class cannot extend classes`,
	} {
		suppressible[n] = regexp.MustCompile(m)
	}
}

// suppressRange is the code of a declaration, statement or expression
// whose @Suppress names diagnostics, from its first annotation to its
// last line.
type suppressRange struct {
	from, to syntax.Pos
	names    []string
}

// errorSuppression is kotlinc's warning about a suppressed error.
const errorSuppression = "This code uses error suppression for '%s'. While it might compile and work, the compiler behavior is UNSPECIFIED and WON'T BE PRESERVED. Please report your use case to the Kotlin issue tracker instead: https://kotl.in/issue"

// suppressErrors drops the errors and the warnings that the @Suppress of
// a declaration, statement or expression they are in, or of their file,
// names. A suppressed error is reported as a warning, ERROR_SUPPRESSION,
// as kotlinc does; @Suppress("warnings") suppresses all warnings.
func (c *Checker) suppressErrors() {
	var rs []suppressRange
	if ns := suppressStrings(c.file.Annotations); len(ns) > 0 {
		rs = append(rs, suppressRange{syntax.Pos{Line: 1}, syntax.Pos{Line: 1 << 30}, ns})
	}
	for _, d := range c.file.Decls {
		rs = suppressRanges(d, rs)
	}
	rs = c.codeSuppressRanges(rs)
	if len(rs) == 0 {
		return
	}
	kept := c.errs[:0]
	var suppressed []*syntax.Error
	for _, e := range c.errs {
		// An unsupported feature isn't kotlinc's error to suppress.
		if e.Feature != "" {
			kept = append(kept, e)
			continue
		}
		if n := suppressedBy(e, rs); n != "" {
			suppressed = append(suppressed, &syntax.Error{File: e.File, Pos: e.Pos, Msg: fmt.Sprintf(errorSuppression, n), Severity: syntax.SevWarning, Name: "ERROR_SUPPRESSION"})
		} else {
			kept = append(kept, e)
		}
	}
	c.errs = kept
	keptW := c.warns[:0]
	for _, w := range c.warns {
		if !warningSuppressed(w, rs) {
			keptW = append(keptW, w)
		}
	}
	c.warns = keptW
	for _, w := range suppressed {
		if !warningSuppressed(w, rs) {
			c.warns = append(c.warns, w)
		}
	}
}

// suppressedBy returns the name of the error e that a range of rs
// suppresses, or "".
func suppressedBy(e *syntax.Error, rs []suppressRange) string {
	for _, r := range rs {
		if before(e.Pos, r.from) || before(r.to, e.Pos) {
			continue
		}
		for _, n := range r.names {
			if e.Name != "" && e.Name == n {
				return n
			}
			if m := suppressible[n]; m != nil && m.MatchString(e.Msg) {
				return n
			}
		}
	}
	return ""
}

// warningSuppressed reports whether a range of rs suppresses warning w.
func warningSuppressed(w *syntax.Error, rs []suppressRange) bool {
	for _, r := range rs {
		if before(w.Pos, r.from) || before(r.to, w.Pos) {
			continue
		}
		for _, n := range r.names {
			if n == w.Name || n == "warnings" {
				return true
			}
		}
	}
	return false
}

func before(a, b syntax.Pos) bool { return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col }

// annotationsRange adds to rs the range of code from p to end that
// annotations anns suppress diagnostics in, if they do.
func annotationsRange(rs []suppressRange, anns []syntax.Annotation, p, end syntax.Pos) []suppressRange {
	ns := suppressStrings(anns)
	if len(ns) == 0 {
		return rs
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
	return append(rs, suppressRange{from, end, ns})
}

// suppressRanges adds the ranges of declaration d and its members to rs.
func suppressRanges(d syntax.Decl, rs []suppressRange) []suppressRange {
	switch d := d.(type) {
	case *syntax.FunDecl:
		rs = annotationsRange(rs, d.Annotations, d.P, d.End)
	case *syntax.PropDecl:
		end := syntax.Pos{}
		for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
			if a != nil && before(end, a.End) {
				end = a.End
			}
		}
		if d.Init != nil {
			if e := nodeEnd(d.Init); before(end, e) {
				end = e
			}
		}
		rs = annotationsRange(rs, d.Annotations, d.P, end)
	case *syntax.ClassDecl:
		rs = annotationsRange(rs, d.Annotations, d.P, d.End)
		for _, m := range d.Members {
			rs = suppressRanges(m, rs)
		}
		for _, e := range d.Entries {
			end := syntax.Pos{Line: e.P.Line, Col: 1 << 30}
			if e.Body != nil && before(end, e.Body.End) {
				end = e.Body.End
			}
			rs = annotationsRange(rs, e.Annotations, e.P, end)
			if e.Body != nil {
				rs = suppressRanges(e.Body, rs)
			}
		}
	case *syntax.SecondaryCtor:
		end := syntax.Pos{Line: d.P.Line, Col: 1 << 30}
		if d.Body != nil {
			end = nodeEnd(d.Body)
		}
		rs = annotationsRange(rs, d.Annotations, d.P, end)
	}
	return rs
}

// codeSuppressRanges adds to rs the ranges of the local declarations,
// statements and expressions annotated @Suppress in the file's code.
func (c *Checker) codeSuppressRanges(rs []suppressRange) []suppressRange {
	walkCode(c.file.Decls, func(n any) bool {
		switch n := n.(type) {
		case *syntax.LocalFun:
			rs = annotationsRange(rs, n.Decl.Annotations, n.Decl.P, nodeEnd(n))
		case *syntax.LocalClass:
			end := nodeEnd(n)
			rs = annotationsRange(rs, n.Annotations, n.Decl.P, end)
			rs = annotationsRange(rs, n.Decl.Annotations, n.Decl.P, end)
			for _, m := range n.Decl.Members {
				rs = suppressRanges(m, rs)
			}
		case *syntax.ObjectExpr:
			for _, m := range n.Decl.Members {
				rs = suppressRanges(m, rs)
			}
		}
		if as, ok := c.file.Annotated[n]; ok {
			rs = annotationsRange(rs, as, nodePos(n), nodeEnd(n))
		}
		return true
	})
	return rs
}

// suppressStrings returns the strings that the @Suppress annotations
// among anns name.
func suppressStrings(anns []syntax.Annotation) []string {
	var ns []string
	for i := range anns {
		a := &anns[i]
		if !a.Is("Suppress") || a.Target != "" && a.Target != "file" && a.Target != "param" {
			continue
		}
		for _, arg := range a.Args {
			for _, t := range arg.Toks {
				for _, part := range t.Parts {
					if part.Lit != "" {
						ns = append(ns, part.Lit)
					}
				}
			}
		}
	}
	return ns
}
