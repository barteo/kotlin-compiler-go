package syntax

import "regexp"

// syntaxErrorNames gives kotlinc's names to the errors of kc's parser and
// lexer, and of lowering a psi tree, which share their messages: kotlinc
// reports most of them as syntax errors (SYNTAX), and those that its raw
// FIR builder or its checkers find in what its grammar accepts under
// names of their own. The first pattern that matches a message names it.
var syntaxErrorNames = []struct {
	re   *regexp.Regexp
	name string
}{}

func init() {
	for _, n := range [][2]string{
		{`^names _, __, ___, \.\.\. are reserved in Kotlin`, "UNDERSCORE_IS_RESERVED"},
		{`^names _, __, ___, \.\.\., can be used only in back-ticks`, "UNDERSCORE_USAGE_WITHOUT_BACKTICKS"},
		{`^multiple labels are forbidden`, "MULTIPLE_LABELS_ARE_FORBIDDEN"},
		{`^local enum classes are not allowed`, "NESTED_CLASS_NOT_ALLOWED"},
		{`^named object( '.*')? cannot be local`, "LOCAL_OBJECT_NOT_ALLOWED"},
		{`^'interface' is not allowed here: local interfaces`, "LOCAL_INTERFACE_NOT_ALLOWED"},
		{`^modifier '.*' is not applicable (to|here)`, "WRONG_MODIFIER_TARGET"},
		{`^repeated '.*'$`, "REPEATED_MODIFIER"},
		{`^this annotation is not applicable to target '.*' and use-site target`, "WRONG_ANNOTATION_TARGET_WITH_USE_SITE_TARGET"},
		{`^repeated context parameter lists`, "MULTIPLE_CONTEXT_LISTS"},
		{`^context parameters must be named`, "CONTEXT_PARAMETER_WITHOUT_NAME"},
		{`^context parameters cannot have default values`, "CONTEXT_PARAMETER_WITH_DEFAULT"},
		{`^context parameters (are unsupported|on enum entries are unsupported)`, "UNSUPPORTED"},
		{`^delegated property cannot have context parameters`, "UNSUPPORTED"},
		{`^named context parameters in function types are unsupported`, "NAMED_CONTEXT_PARAMETER_IN_FUNCTION_TYPE"},
		{`^anonymous functions can't have default values`, "ANONYMOUS_FUNCTION_PARAMETER_WITH_DEFAULT_VALUE"},
		{`^type parameters must be placed before the name of the function`, "DEPRECATED_TYPE_PARAMETER_SYNTAX"},
		{`^type parameter annotations are not allowed inside where clauses`, "ANNOTATION_IN_WHERE_CLAUSE_ERROR"},
		{`^'.*' does not refer to a type parameter of this declaration`, "NAME_IN_CONSTRAINT_IS_NOT_A_TYPE_PARAMETER"},
		{`^type parameters of a property must be used in its receiver type`, "INCORRECT_TYPE_PARAMETER_OF_PROPERTY"},
		{`^type parameters are not allowed for objects`, "TYPE_PARAMETERS_IN_OBJECT"},
		{`^type parameters are not allowed for anonymous functions`, "TYPE_PARAMETERS_NOT_ALLOWED"},
		{`^local variables are not allowed to have type parameters`, "LOCAL_VARIABLE_WITH_TYPE_PARAMETERS"},
		{`^setter return type must be Unit`, "WRONG_SETTER_RETURN_TYPE"},
		{`^use the 'constructor' keyword after the modifiers of the primary constructor`, "MISSING_CONSTRUCTOR_KEYWORD"},
		{`^this variable must either have an explicit type or be initialized`, "VARIABLE_WITH_NO_TYPE_NO_INITIALIZER"},
		{`^modifier form for a non-built-in 'suspend' function is not allowed`, "MODIFIER_FORM_FOR_NON_BUILT_IN_SUSPEND_FUN_ERROR"},
		{`^a class literal needs a class or a value before ::`, "UNSUPPORTED_CLASS_LITERALS_WITH_EMPTY_LHS"},
		{`^the value is out of range`, "INT_LITERAL_OUT_OF_RANGE"},
		{`^invalid (float|double) literal`, "FLOAT_LITERAL_OUT_OF_RANGE"},
		{`^use 'L' instead of 'l'`, "WRONG_LONG_SUFFIX"},
		{`^unsupported leading zeros in an integer literal`, "INT_LITERAL_WITH_LEADING_ZEROS"},
		{`^illegal underscore`, "ILLEGAL_UNDERSCORE"},
		{`^(illegal escape|invalid unicode escape)`, "ILLEGAL_ESCAPE"},
		{`^empty character literal`, "EMPTY_CHARACTER_LITERAL"},
		{`^too many characters in a character literal`, "TOO_MANY_CHARACTERS_IN_CHARACTER_LITERAL"},
		{`^unterminated character literal`, "INCORRECT_CHARACTER_LITERAL"},
		{`^('val'|'var'|'.*') on catch parameter is not allowed`, "VAL_OR_VAR_ON_CATCH_PARAMETER"},
		{`^illegal variable declaration in 'when' subject`, "ILLEGAL_DECLARATION_IN_WHEN_SUBJECT"},
		{`^deprecated syntax: use '\|\|' instead of commas in a when-condition`, "COMMA_IN_WHEN_CONDITION_WITHOUT_ARGUMENT"},
		{`^confusing branch condition`, "CONFUSING_BRANCH_CONDITION_ERROR"},
		{`^unexpected trailing lambda on a new line`, "UNEXPECTED_TRAILING_LAMBDA_ON_A_NEW_LINE"},
		{`^only one lambda expression is allowed outside a parenthesized argument list`, "MANY_LAMBDA_EXPRESSION_ARGUMENTS"},
		{`^for-loop range must have an 'iterator\(\)' method`, "ITERATOR_MISSING"},
		{`^assignments are not expressions`, "ASSIGNMENT_IN_EXPRESSION_CONTEXT"},
		{`^unsupported \[`, "UNSUPPORTED"},
	} {
		syntaxErrorNames = append(syntaxErrorNames, struct {
			re   *regexp.Regexp
			name string
		}{regexp.MustCompile(n[0]), n[1]})
	}
}

// ErrorName returns kotlinc's name for msg, an error of kc's parser or
// lexer or of lowering a psi tree: SYNTAX but for those syntaxErrorNames
// names otherwise.
func ErrorName(msg string) string {
	for _, n := range syntaxErrorNames {
		if n.re.MatchString(msg) {
			return n.name
		}
	}
	return "SYNTAX"
}
