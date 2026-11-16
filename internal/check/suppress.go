package check

import (
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
			if suppressible[n].MatchString(e.Msg) {
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
