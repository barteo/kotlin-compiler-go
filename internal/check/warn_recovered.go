package check

// recoveredWarnings are the warnings reported of a file with syntax
// errors, which is checked as far as the parser recovered it: without
// the statements and declarations that have a syntax error
// (Options.Recovered). What is left out may use, assign or jump from
// what is kept, so only the warnings are reported that a statement or
// declaration left out can't be the reason for: those judged by the
// syntax and modifiers of a declaration or expression that is kept, by
// the types written in its header, or by two declarations that are both
// kept.
//
// Not the warnings of what isn't used or read, of code that can't be
// reached, or of calls that aren't made (UNUSED_VARIABLE,
// UNREACHABLE_CODE, NO_TAIL_CALLS_FOUND); nor those judged by an
// expression's type, which a smart cast that an assignment left out
// would end may have narrowed (USELESS_IS_CHECK, SENSELESS_COMPARISON,
// UNNECESSARY_SAFE_CALL); nor those of what a name resolves to, which a
// declaration left out may change (DEPRECATION, OPT_IN_USAGE); nor those
// of the members a class inherits and overrides
// (PARAMETER_NAME_CHANGED_ON_OVERRIDE).
var recoveredWarnings = map[string]bool{
	// Literals and syntax.
	"REDUNDANT_INTERPOLATION_PREFIX":                        true,
	"NOT_NULL_ASSERTION_ON_LAMBDA_EXPRESSION":               true,
	"NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE":              true,
	"RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING":       true,
	"REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION":   true,
	"REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_ANNOTATION": true,
	"DANGEROUS_CHARACTERS":                                  true,
	"ERROR_SUPPRESSION":                                     true,
	// Modifiers.
	"REDUNDANT_MODIFIER":                  true,
	"REDUNDANT_OPEN_IN_INTERFACE":         true,
	"REDUNDANT_MODIFIER_FOR_TARGET":       true,
	"DEPRECATED_MODIFIER_FOR_TARGET":      true,
	"REDUNDANT_ANNOTATION_TARGET":         true,
	"OVERLOADS_PRIVATE":                   true,
	"OVERLOADS_WITHOUT_DEFAULT_ARGUMENTS": true,
	"DSL_MARKER_PROPAGATES_TO_MANY":       true,
	"OPERATOR_RENAMED_ON_IMPORT":          true,
	"INCOMPATIBLE_ANNOTATION_TARGETS":     true,
	"REDUNDANT_REPEATABLE_ANNOTATION":     true,
	"NON_FINAL_MEMBER_IN_FINAL_CLASS":     true,
	"NON_FINAL_MEMBER_IN_OBJECT":          true,
	"USELESS_VARARG_ON_PARAMETER":         true,
	"INLINE_CLASS_DEPRECATED":             true,
	"NOTHING_TO_INLINE":                   true,
	"REDUNDANT_EXPLICIT_BACKING_FIELD":    true,
	// The types of a declaration's header.
	"FINAL_UPPER_BOUND":                    true,
	"MISPLACED_TYPE_PARAMETER_CONSTRAINTS": true,
	"REDUNDANT_NULLABLE":                   true,
	"REDUNDANT_PROJECTION":                 true,
	"PLATFORM_CLASS_MAPPED_TO_KOTLIN":      true,
	// Annotations.
	"DEPRECATED_JAVA_ANNOTATION":         true,
	"ANNOTATION_TARGETS_ONLY_IN_JAVA":    true,
	"DSL_MARKER_APPLIED_TO_WRONG_TARGET": true,
	// Two declarations.
	"CONTEXTUAL_OVERLOAD_SHADOWED":                               true,
	"EXTENSION_SHADOWED_BY_MEMBER":                               true,
	"EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE": true,
}
