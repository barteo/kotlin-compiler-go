package check

import (
	"regexp"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// suppressible maps the names of the errors that @Suppress may name to
// kc's messages for each: kotlinc compiles the code without the error,
// warning of the suppression (see errorSuppressionNames). Errors whose
// code the backend can't generate, as an unresolved reference or a
// missing argument, aren't listed. kc's messages don't tell apart some errors
// kotlinc names differently: one type mismatch message is all of
// ARGUMENT_TYPE_MISMATCH, INITIALIZER_TYPE_MISMATCH and the others.
var suppressible = map[string]*regexp.Regexp{}

func init() {
	typeMismatch := `^type mismatch: inferred type is `
	notImplemented := `^(class '.*' is|object is) not abstract and does not implement abstract member `
	invisible := `^cannot access '.*': it is `
	for n, m := range map[string]string{
		"ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED":       notImplemented,
		"ABSTRACT_FUNCTION_IN_NON_ABSTRACT_CLASS":     `^abstract function '.*' in non-abstract class `,
		"ABSTRACT_MEMBER_NOT_IMPLEMENTED":             notImplemented,
		"ABSTRACT_PROPERTY_IN_NON_ABSTRACT_CLASS":     `^abstract property '.*' in non-abstract class `,
		"ARGUMENT_TYPE_MISMATCH":                      typeMismatch,
		"ASSIGNMENT_TYPE_MISMATCH":                    typeMismatch,
		"CONCURRENT_HASH_MAP_CONTAINS_OPERATOR_ERROR": `^method 'contains' from ConcurrentHashMap may have unexpected semantics`,
		"CONFLICTING_OVERLOADS":                       `^conflicting overloads: `,
		"INAPPLICABLE_JVM_NAME":                       `^'@JvmName' annotation is not applicable to this declaration`,
		"JAVA_MODULE_DOES_NOT_EXPORT_PACKAGE":         `^symbol is declared in module '.*' which does not export package `,
		"CONST_VAL_NOT_TOP_LEVEL_OR_OBJECT":           `^const 'val' are only allowed on top level`,
		"CONST_VAL_WITH_GETTER":                       `^const 'val' should not have a getter`,
		"INAPPLICABLE_JVM_FIELD":                      `^JvmField `,
		"INAPPLICABLE_OPERATOR_MODIFIER":              `^'operator' modifier is inapplicable on this function`,
		"INLINE_FROM_HIGHER_PLATFORM":                 `^cannot inline bytecode built with JVM target `,
		"NO_EXPLICIT_RETURN_TYPE_IN_API_MODE":         `^return type must be specified in explicit API mode`,
		"NO_EXPLICIT_VISIBILITY_IN_API_MODE":          `^visibility must be specified in explicit API mode`,
		"NON_PUBLIC_CALL_FROM_PUBLIC_INLINE":          `^public-API inline function cannot access non-public-API `,
		"PRIVATE_PROPERTY_IN_INTERFACE":               `^abstract property in an interface cannot be private`,
		"PRIVATE_SETTER_FOR_ABSTRACT_PROPERTY":        `^private setters are not allowed for abstract properties`,
		"SUBCLASS_OPT_IN_INAPPLICABLE":                `^'@SubclassOptInRequired' is not applicable to `,
		"TYPE_VARIANCE_CONFLICT_ERROR":                `^type parameter .* is declared as '.*' but occurs in '.*' position`,
		"UPPER_BOUND_CANNOT_BE_ARRAY":                 `^upper bound of a type parameter cannot be an array`,
		"CREATING_AN_INSTANCE_OF_ABSTRACT_CLASS":      `^cannot create an instance of an abstract class`,
		"DATA_CLASS_VARARG_PARAMETER":                 `^primary constructor vararg parameters are forbidden for data classes`,
		"DEPRECATION_ERROR":                           `' is deprecated with level ERROR`,
		"EQUALITY_NOT_APPLICABLE":                     `^operator '(==|!=|===|!==)' cannot be applied to `,
		"INITIALIZER_TYPE_MISMATCH":                   typeMismatch,
		"INTERFACE_WITH_SUPERCLASS":                   `^an interface cannot inherit from a class`,
		"INVISIBLE_MEMBER":                            invisible,
		"INVISIBLE_REFERENCE":                         invisible,
		"METHOD_OF_ANY_IMPLEMENTED_IN_INTERFACE":      `^an interface may not implement a method of 'Any'`,
		"MUST_BE_INITIALIZED":                         `^property must be initialized`,
		"MUST_BE_INITIALIZED_OR_BE_ABSTRACT":          `^property must be initialized or be abstract`,
		"NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION":   `^only 'const val' can be used in constant expressions`,
		"NOTHING_TO_OVERRIDE":                         `' overrides nothing$`,
		"OVERRIDING_FINAL_MEMBER":                     `' is final and cannot be overridden$`,
		"PROPERTY_WITH_NO_TYPE_NO_INITIALIZER":        `^this property must have a type, be initialized`,
		"REPEATED_ANNOTATION":                         `^this annotation is not repeatable`,
		"RETURN_TYPE_MISMATCH":                        typeMismatch,
		"TYPE_MISMATCH":                               typeMismatch,
		"UNSAFE_CALL":                                 `^only safe \(\?\.\) or non-null asserted \(!!\.\) calls are allowed`,
		"UPPER_BOUND_VIOLATED":                        `^type argument is not within its bounds`,
		"VAL_REASSIGNMENT":                            `^val cannot be reassigned`,
		"VALUE_CLASS_CANNOT_EXTEND_CLASSES":           `^value class cannot extend classes`,
	} {
		suppressible[n] = regexp.MustCompile(m)
	}
}

// IsNamed reports whether msg, one of kc's error messages, is of the
// error kotlinc names name, as far as suppressible tells them apart. The
// conformance harness uses it for the errors a test's directives switch
// off.
func IsNamed(msg, name string) bool {
	re, ok := suppressible[name]
	return ok && re.MatchString(msg)
}

// suppressRange is the code of a declaration, statement or expression
// whose @Suppress names diagnostics, from its first annotation to its
// last line.
type suppressRange struct {
	from, to syntax.Pos
	names    []suppressName
}

// suppressName is a string that a @Suppress names, and where.
type suppressName struct {
	name string
	p    syntax.Pos
}

// errorSuppression is kotlinc's warning about a suppressed error.
const errorSuppression = "suppression of error '%s' might compile and work, but the compiler behavior is UNSPECIFIED and WILL NOT BE PRESERVED. Please report your use case to the Kotlin issue tracker instead: https://kotl.in/issue"

// suppressErrors drops the errors and the warnings that the @Suppress of
// a declaration, statement or expression they are in, or of their file,
// names; @Suppress("warnings") suppresses all warnings. As kotlinc, it
// warns of each string that names an error (ERROR_SUPPRESSION).
func (c *Checker) suppressErrors() {
	var rs []suppressRange
	if ns := suppressStrings(c.file.Annotations); len(ns) > 0 {
		rs = append(rs, suppressRange{syntax.Pos{Line: 1}, syntax.Pos{Line: 1 << 30}, ns})
	}
	for _, d := range c.file.Decls {
		rs = suppressRanges(d, rs)
	}
	rs = c.codeSuppressRanges(rs)
	rs = append(rs, c.contractSuppress...)
	if len(rs) == 0 {
		return
	}
	keptW := c.warns[:0]
	for _, w := range c.warns {
		if !warningSuppressed(w, rs) {
			keptW = append(keptW, w)
		}
	}
	c.warns = keptW
	kept := c.errs[:0]
	for _, e := range c.errs {
		// An unsupported feature isn't kotlinc's error to suppress.
		if e.Feature != "" {
			kept = append(kept, e)
			continue
		}
		if _, ok := suppressedBy(e, rs); !ok {
			kept = append(kept, e)
		}
	}
	c.errs = kept
	// kotlinc warns of each string that names an error as it is spelled,
	// whether or not the code has the error.
	reported := map[syntax.Pos]bool{}
	for _, r := range rs {
		for _, n := range r.names {
			if errorSuppressionNames[n.name] && !reported[n.p] {
				reported[n.p] = true
				c.warnf(n.p, "ERROR_SUPPRESSION", errorSuppression, n.name)
			}
		}
	}
}

// suppressedBy returns the name of the error e that a range of rs
// suppresses, and whether one does.
func suppressedBy(e *syntax.Error, rs []suppressRange) (suppressName, bool) {
	for _, r := range rs {
		if before(e.Pos, r.from) || before(r.to, e.Pos) {
			continue
		}
		for _, n := range r.names {
			// As kotlinc, names match whatever their case.
			if e.Name != "" && strings.EqualFold(e.Name, n.name) {
				return n, true
			}
			if m := suppressible[strings.ToUpper(n.name)]; m != nil && m.MatchString(e.Msg) {
				return n, true
			}
		}
	}
	return suppressName{}, false
}

// warningSuppressed reports whether a range of rs suppresses warning w.
func warningSuppressed(w *syntax.Error, rs []suppressRange) bool {
	for _, r := range rs {
		if before(w.Pos, r.from) || before(r.to, w.Pos) {
			continue
		}
		for _, n := range r.names {
			if strings.EqualFold(n.name, w.Name) || strings.EqualFold(n.name, "warnings") {
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

// paramRanges adds to rs the ranges of the parameters ps whose @Suppress
// names diagnostics: their types and default values.
func paramRanges(rs []suppressRange, ps []*syntax.Param) []suppressRange {
	for _, p := range ps {
		if p == nil {
			continue
		}
		end := syntax.Pos{Line: p.P.Line, Col: 1 << 30}
		if p.Default != nil {
			end = nodeEnd(p.Default)
		}
		rs = annotationsRange(rs, p.Annotations, p.P, end)
	}
	return rs
}

// typeParamRanges adds to rs the ranges of the type parameters tps whose
// @Suppress names diagnostics: their bounds, to the end of the line.
func typeParamRanges(rs []suppressRange, tps []*syntax.TypeParam) []suppressRange {
	for _, tp := range tps {
		rs = annotationsRange(rs, tp.Annotations, tp.P, syntax.Pos{Line: tp.P.Line, Col: 1 << 30})
	}
	return rs
}

// funRanges adds to rs the ranges of function d and its parameters.
func funRanges(rs []suppressRange, d *syntax.FunDecl) []suppressRange {
	if d == nil {
		return rs
	}
	end := d.End
	if end.Line == 0 {
		if d.Body != nil {
			end = nodeEnd(d.Body)
		} else if d.ExprBody != nil {
			end = nodeEnd(d.ExprBody)
		}
	}
	rs = annotationsRange(rs, d.Annotations, d.P, end)
	rs = typeParamRanges(rs, d.TypeParams)
	return paramRanges(rs, d.Params)
}

// suppressRanges adds the ranges of declaration d and its members to rs.
func suppressRanges(d syntax.Decl, rs []suppressRange) []suppressRange {
	switch d := d.(type) {
	case *syntax.FunDecl:
		rs = funRanges(rs, d)
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
		rs = typeParamRanges(rs, d.TypeParams)
		rs = funRanges(rs, d.Getter)
		rs = funRanges(rs, d.Setter)
	case *syntax.ClassDecl:
		rs = annotationsRange(rs, d.Annotations, d.P, d.End)
		rs = typeParamRanges(rs, d.TypeParams)
		if as := d.CtorMods.Annotations; len(as) > 0 {
			// The primary constructor's: its parameters.
			end := syntax.Pos{Line: as[len(as)-1].P.Line, Col: 1 << 30}
			if n := len(d.CtorParams); n > 0 && before(end, d.CtorParams[n-1].P) {
				end = syntax.Pos{Line: d.CtorParams[n-1].P.Line, Col: 1 << 30}
			}
			rs = annotationsRange(rs, as, as[0].P, end)
		}
		for _, p := range d.CtorParams {
			rs = paramRanges(rs, []*syntax.Param{&p.Param})
			end := syntax.Pos{Line: p.P.Line, Col: 1 << 30}
			if p.Default != nil {
				end = nodeEnd(p.Default)
			}
			rs = annotationsRange(rs, p.Modifiers.Annotations, p.P, end)
		}
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
		rs = paramRanges(rs, d.Params)
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
			rs = paramRanges(rs, n.Decl.Params)
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
		case *syntax.Lambda:
			for _, p := range n.Params {
				if len(p.Annotations) > 0 {
					rs = annotationsRange(rs, p.Annotations, p.P, syntax.Pos{Line: p.P.Line, Col: 1 << 30})
				}
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
func suppressStrings(anns []syntax.Annotation) []suppressName {
	var ns []suppressName
	for i := range anns {
		a := &anns[i]
		// An accessor's annotations are the property's, with a get or
		// set target.
		if !a.Is("Suppress") {
			continue
		}
		for _, arg := range a.Args {
			// "DEPR" + "ECATION" is a constant too.
			var cur *suppressName
			plus := false
			for _, t := range arg.Toks {
				if t.Kind == syntax.STRING {
					if lit, ok := literalString(t); ok {
						if cur != nil && plus {
							cur.name += lit
						} else {
							ns = append(ns, suppressName{lit, t.Pos})
							cur = &ns[len(ns)-1]
						}
						plus = false
						continue
					}
				}
				plus = cur != nil && t.Kind == syntax.PLUS
				if !plus && t.Kind != syntax.NL {
					cur = nil
				}
			}
		}
	}
	return ns
}
