package psi

// Port of KotlinExpressionParsing from Kotlin 2.4.21 (see
// THIRD_PARTY_NOTICES.md). Methods keep the Java order, control flow and
// messages. Java asserts are omitted: the reference parser runs with
// assertions disabled.

var (
	epWhenConditionRecoverySet          = newSet(KindRBRACE, KindIN_KEYWORD, KindNOT_IN, KindIS_KEYWORD, KindNOT_IS, KindELSE_KEYWORD)
	epWhenConditionRecoverySetWithArrow = newSet(KindRBRACE, KindIN_KEYWORD, KindNOT_IN, KindIS_KEYWORD, KindNOT_IS, KindELSE_KEYWORD, KindARROW, KindDOT)

	// epKeywordTexts is KEYWORD_TEXTS: KtTokens.KEYWORDS keyed by their
	// debug names (the keyword text, or "NOT_IN", "NOT_IS", "AS_SAFE").
	epKeywordTexts = func() map[string]Kind {
		m := map[string]Kind{"NOT_IN": KindNOT_IN, "NOT_IS": KindNOT_IS, "AS_SAFE": KindAS_SAFE}
		for s, k := range hardKeywords {
			m[s] = k
		}
		return m
	}()

	epTokenSetToFollowAfterDestructuringDeclarationInLambda         = newSet(KindARROW, KindCOMMA, KindCOLON)
	epTokenSetToFollowAfterDestructuringDeclarationInLambdaRecovery = orSet(epTokenSetToFollowAfterDestructuringDeclarationInLambda, kpParameterNameRecoverySet)
	epEqRParSet                                                     = newSet(KindEQ, KindRPAR)
	epArrowSet                                                      = newSet(KindARROW)
	epArrowCommaSet                                                 = newSet(KindARROW, KindCOMMA)
	epInKeywordRParColonSet                                         = newSet(KindIN_KEYWORD, KindRPAR, KindCOLON)
	epInKeywordLBraceSet                                            = newSet(KindIN_KEYWORD, KindLBRACE)
	epInKeywordLBraceRecoverySet                                    = orSet(epInKeywordLBraceSet, kpParameterNameRecoverySet)
	epColonInKeywordSet                                             = newSet(KindCOLON, KindIN_KEYWORD)
	epLParLBraceRParSet                                             = newSet(KindLPAR, KindLBRACE, KindRPAR)
	epInKeywordSet                                                  = newSet(KindIN_KEYWORD)
	epTryCatchRecoveryTokenSet                                      = newSet(KindLBRACE, KindRBRACE, KindFINALLY_KEYWORD, KindCATCH_KEYWORD)

	epTypeArgumentListStoppers = newSet(
		KindINTEGER_LITERAL, KindFLOAT_LITERAL, KindCHARACTER_LITERAL, KindINTERPOLATION_PREFIX, KindOPEN_QUOTE,
		KindPACKAGE_KEYWORD, KindAS_KEYWORD, KindTYPE_ALIAS_KEYWORD, KindINTERFACE_KEYWORD, KindCLASS_KEYWORD, KindTHIS_KEYWORD,
		KindVAL_KEYWORD, KindVAR_KEYWORD, KindFUN_KEYWORD, KindFOR_KEYWORD, KindNULL_KEYWORD,
		KindTRUE_KEYWORD, KindFALSE_KEYWORD, KindIS_KEYWORD, KindTHROW_KEYWORD, KindRETURN_KEYWORD, KindBREAK_KEYWORD,
		KindCONTINUE_KEYWORD, KindOBJECT_KEYWORD, KindIF_KEYWORD, KindTRY_KEYWORD, KindELSE_KEYWORD, KindWHILE_KEYWORD, KindDO_KEYWORD,
		KindWHEN_KEYWORD, KindRBRACKET, KindRBRACE, KindRPAR, KindPLUSPLUS, KindMINUSMINUS, KindEXCLEXCL,
		// MUL,
		KindPLUS, KindMINUS, KindEXCL, KindDIV, KindPERC, KindLTEQ,
		// TODO GTEQ,   foo<bar, baz>=x
		KindEQEQEQ, kExclEqEqEq, KindEQEQ, KindEXCLEQ, KindANDAND, KindOROR, KindSAFE_ACCESS, KindELVIS,
		KindSEMICOLON, KindRANGE, KindRANGE_UNTIL, KindEQ, KindMULTEQ, KindDIVEQ, KindPERCEQ, KindPLUSEQ, KindMINUSEQ, KindNOT_IN, KindNOT_IS,
		KindCOLONCOLON,
		KindCOLON,
	)

	epExpressionFirst = newSet(
		// Prefix
		KindMINUS, KindPLUS, KindMINUSMINUS, KindPLUSPLUS,
		KindEXCL, KindEXCLEXCL, // Joining complex tokens makes it necessary to put EXCLEXCL here
		// Atomic
		KindCOLONCOLON, // callable reference
		KindLPAR,       // parenthesized
		// literal constant
		KindTRUE_KEYWORD, KindFALSE_KEYWORD,
		KindINTERPOLATION_PREFIX, KindOPEN_QUOTE,
		KindINTEGER_LITERAL, KindCHARACTER_LITERAL, KindFLOAT_LITERAL,
		KindNULL_KEYWORD,
		KindLBRACE,       // functionLiteral
		KindFUN_KEYWORD,  // expression function
		KindTHIS_KEYWORD, // this
		KindSUPER_KEYWORD,
		KindIF_KEYWORD, KindWHEN_KEYWORD, KindTRY_KEYWORD, KindOBJECT_KEYWORD,
		// jump
		KindTHROW_KEYWORD, KindRETURN_KEYWORD, KindCONTINUE_KEYWORD, KindBREAK_KEYWORD,
		// loop
		KindFOR_KEYWORD, KindWHILE_KEYWORD, KindDO_KEYWORD,
		KindIDENTIFIER, // SimpleName
		KindAT,         // Just for better recovery and maybe for annotations
		KindLBRACKET,   // Collection literal expression
	)

	epStatementFirst = orSet(
		epExpressionFirst,
		newSet(
			// declaration
			KindFUN_KEYWORD,
			KindVAL_KEYWORD, KindVAR_KEYWORD,
			KindINTERFACE_KEYWORD,
			KindCLASS_KEYWORD,
			KindTYPE_ALIAS_KEYWORD,
		),
		modifierKeywords,
	)

	epStatementNewLineQuickRecoverySet = orSet(
		epAndSet(epStatementFirst, hardKeywordSet.minus(newSet(KindIN_KEYWORD))),
		newSet(kEOLOrSemicolon))

	epExpressionFollow = newSet(kEOLOrSemicolon, KindARROW, KindCOMMA, KindRBRACE, KindRPAR, KindRBRACKET)

	// Java: TokenSet.orSet(EXPRESSION_FOLLOW, TokenSet.create(LONG_TEMPLATE_ENTRY_END)) in parseAtomicExpression.
	epAtomicRecoverySet = orSet(epExpressionFollow, newSet(KindLONG_TEMPLATE_ENTRY_END))

	epAllowNewlineOperations = newSet(
		KindDOT, KindSAFE_ACCESS,
		KindCOLON, KindAS_KEYWORD, KindAS_SAFE,
		KindELVIS,
		// Can't allow `is` and `!is` because of when entry conditions: IS_KEYWORD, NOT_IS,
		KindANDAND,
		KindOROR,
	)

	// Java: TokenSet.create(ARROW, LBRACE, RBRACE, EOL_OR_SEMICOLON) in parseWhenEntry.
	epWhenElseArrowRecoverySet = newSet(KindARROW, KindLBRACE, KindRBRACE, kEOLOrSemicolon)
	// Java: TokenSet.create(LBRACE, RBRACE, ARROW) in parseWhenEntryGuardOrSuggest.
	epLbraceRbraceArrowSet = newSet(KindLBRACE, KindRBRACE, KindARROW)
	// Java: TokenSet.create(EOL_OR_SEMICOLON, LBRACE, RBRACE) in parseStatements.
	epSeveralStatementsRecoverySet = newSet(kEOLOrSemicolon, KindLBRACE, KindRBRACE)
)

func epAndSet(a, b tokenSet) tokenSet {
	for i := range a {
		a[i] &= b[i]
	}
	return a
}

// binaryOperationPrecedence is org.jetbrains.kotlin.lang.BinaryOperationPrecedence
// (psi-api). Ordinals matter: higherPriority is the previous value, and AS
// has none.
type binaryOperationPrecedence int8

const (
	precNone binaryOperationPrecedence = iota - 1
	precAs
	precMultiplicative
	precAdditive
	precRange
	precInfix
	precElvis
	precInOrIs
	precComparison
	precEquality
	precConjunction
	precDisjunction
	precAssignment
)

const (
	minBinaryOperationPrecedence = precAs
	maxBinaryOperationPrecedence = precAssignment
)

func (p binaryOperationPrecedence) higherPriority() binaryOperationPrecedence { return p - 1 }

// tokenToBinaryPrecedenceMapWithSoftIdentifiers is
// TOKEN_TO_BINARY_PRECEDENCE_MAP_WITH_SOFT_IDENTIFIERS: every binary
// operation token, plus the soft keywords at INFIX (they are remapped to
// IDENTIFIER when used as infix function names).
var tokenToBinaryPrecedenceMapWithSoftIdentifiers = func() map[Kind]binaryOperationPrecedence {
	m := map[Kind]binaryOperationPrecedence{}
	register := func(p binaryOperationPrecedence, ks ...Kind) {
		for _, k := range ks {
			if _, dup := m[k]; dup {
				panic("duplicate binary precedence token")
			}
			m[k] = p
		}
	}
	register(precAs, KindAS_KEYWORD, KindAS_SAFE)
	register(precMultiplicative, KindMUL, KindDIV, KindPERC)
	register(precAdditive, KindPLUS, KindMINUS)
	register(precRange, KindRANGE, KindRANGE_UNTIL)
	register(precInfix, KindIDENTIFIER)
	for _, k := range softKeywordTexts {
		register(precInfix, k)
	}
	register(precElvis, KindELVIS)
	register(precInOrIs, KindIN_KEYWORD, KindNOT_IN, KindIS_KEYWORD, KindNOT_IS)
	register(precComparison, KindLT, KindGT, KindLTEQ, KindGTEQ)
	register(precEquality, KindEQEQ, KindEXCLEQ, KindEQEQEQ, kExclEqEqEq)
	register(precConjunction, KindANDAND)
	register(precDisjunction, KindOROR)
	register(precAssignment, KindEQ, KindPLUSEQ, KindMINUSEQ, KindMULTEQ, KindDIVEQ, KindPERCEQ)
	return m
}()

// KtTokenSets.POSTFIX_OPERATIONS and PREFIX_OPERATIONS.
var (
	epPostfixOperations = newSet(KindPLUSPLUS, KindMINUSMINUS, KindEXCLEXCL, KindDOT, KindSAFE_ACCESS)
	epPrefixOperations  = newSet(KindMINUS, KindPLUS, KindMINUSMINUS, KindPLUSPLUS, KindEXCL)
)

/*
 * element
 *   : annotations element
 *   : "(" element ")" // see tupleLiteral
 *   : literalConstant
 *   : functionLiteral
 *   : tupleLiteral
 *   : "null"
 *   : "this" ("<" type ">")?
 *   : expressionWithPrecedences
 *   : if
 *   : try
 *   : "typeof" "(" element ")"
 *   : "new" constructorInvocation
 *   : objectLiteral
 *   : declaration
 *   : jump
 *   : loop
 *   // block is syntactically equivalent to a functionLiteral with no parameters
 *   ;
 */
func (e *exprParsing) parseExpression() {
	e.parseExpressionMsg("Expecting an expression")
}

// parseExpressionMsg is parseExpression(String messageIfNotExpressionFirst).
func (e *exprParsing) parseExpressionMsg(messageIfNotExpressionFirst string) {
	if !e.atSet(kpExpressionFirst) {
		e.error(messageIfNotExpressionFirst)
		return
	}

	e.parseBinaryExpression(maxBinaryOperationPrecedence)
}

/*
 * element (operation element)*
 *
 * Returns the minPrecedence that should be considered during further
 * parsing. It matters for IS, which is handled in a special way since it
 * doesn't use recursive parsing on its RHS.
 */
func (e *exprParsing) parseBinaryExpression(maxPrecedence binaryOperationPrecedence) binaryOperationPrecedence {
	if maxPrecedence == precNone {
		panic("Shouldn't be here")
	}

	expression := e.mark()

	e.parsePrefixExpression()

	minPrecedence := minBinaryOperationPrecedence

	for !e.interruptedWithNewLine() {
		operation := e.tt()
		if operation == kNone {
			break
		}

		// Match all allowed operations with the current or higher priority
		// (lower precedence) at once. All Kotlin binary operations are
		// syntactically left-associative, so the recursive call always
		// uses the higher priority.
		nextPrecedence, ok := tokenToBinaryPrecedenceMapWithSoftIdentifiers[operation]
		if !ok || nextPrecedence > maxPrecedence || nextPrecedence < minPrecedence {
			break
		}

		if nextPrecedence == precInfix && isSoftKeyword(operation) {
			// Remap soft keywords and modifiers that are treated as infix functions since there is no mutating `atSetWithRemap` call
			e.b.remapCurrentToken(KindIDENTIFIER)
		}

		e.parseOperationReference()

		var resultType Kind
		switch operation {
		case KindAS_KEYWORD, KindAS_SAFE:
			e.kp.parseTypeRefWithoutIntersections()
			minPrecedence = precAs
			resultType = KindBINARY_WITH_TYPE
		case KindIS_KEYWORD, KindNOT_IS:
			e.kp.parseTypeRefWithoutIntersections()
			// The handling of `is`, it doesn't parse RHS recursively and greedily.
			// To prevent parsing of more prioritized operations next to `is` (for instance, INFIX, RANGE, and others),
			// use the `minPrecedence` in addition to `maxPrecedence`.
			minPrecedence = precInOrIs
			resultType = KindIS_EXPRESSION
		default:
			// Parse operations with higher priorities greedily.
			minPrecedence = e.parseBinaryExpression(nextPrecedence.higherPriority())
			resultType = KindBINARY_EXPRESSION
		}

		expression.Done(resultType)
		expression = expression.Precede()
	}

	expression.Drop()

	return minPrecedence
}

/*
 * label prefixExpression
 */
func (e *exprParsing) parseLabeledExpression() {
	expression := e.mark()
	e.parseLabelDefinition()
	e.parsePrefixExpression()
	expression.Done(KindLABELED_EXPRESSION)
}

/*
 * operation? prefixExpression
 */
func (e *exprParsing) parsePrefixExpression() {
	if e.at(KindAT) {
		if !e.parseLocalDeclaration( /* rollbackIfDefinitelyNotExpression = */ false, false) {
			expression := e.mark()
			e.kp.parseAnnotations(annDefault)
			e.parsePrefixExpression()
			expression.Done(KindANNOTATED_EXPRESSION)
		}
	} else {
		e.b.disableJoiningComplexTokens()
		if e.isAtLabelDefinitionOrMissingIdentifier() {
			e.b.restoreJoiningComplexTokensState()
			e.parseLabeledExpression()
		} else if e.atSet(epPrefixOperations) {
			expression := e.mark()

			e.parseOperationReference()

			e.b.restoreJoiningComplexTokensState()

			e.parsePrefixExpression()
			expression.Done(KindPREFIX_EXPRESSION)
		} else {
			e.b.restoreJoiningComplexTokensState()
			e.parsePostfixExpression()
		}
	}
}

/*
 * doubleColonSuffix
 *   : "::" SimpleName typeArguments?
 *   ;
 */
func (e *exprParsing) parseDoubleColonSuffix(expression Marker) bool {
	if !e.at(KindCOLONCOLON) {
		return false
	}

	e.advance() // COLONCOLON

	if e.at(KindCLASS_KEYWORD) {
		e.advance() // CLASS_KEYWORD

		expression.Done(KindCLASS_LITERAL_EXPRESSION)
		return true
	}

	e.parseSimpleNameExpression()

	if e.at(KindLT) {
		typeArgumentList := e.mark()
		if e.kp.tryParseTypeArgumentList(epTypeArgumentListStoppers) {
			typeArgumentList.Error("Type arguments are not allowed")
		} else {
			typeArgumentList.RollbackTo()
		}
	}

	if e.at(KindLPAR) && !e.b.newlineBeforeCurrentToken() {
		lpar := e.mark()
		e.parseCallSuffix()
		lpar.Error("This syntax is reserved for future use; to call a reference, enclose it in parentheses: (foo::bar)(args)")
	}

	expression.Done(KindCALLABLE_REFERENCE_EXPRESSION)
	return true
}

func (e *exprParsing) skipQuestionMarksBeforeDoubleColon() {
	if e.at(KindQUEST) {
		k := 1
		for e.lookahead(k) == KindQUEST {
			k++
		}
		if e.lookahead(k) == KindCOLONCOLON {
			for k > 0 {
				e.advance() // QUEST
				k--
			}
		}
	}
}

/*
 * postfixUnaryExpression
 *   : atomicExpression postfixUnaryOperation*
 *   ;
 *
 * postfixUnaryOperation
 *   : "++" : "--" : "!!"
 *   : typeArguments? valueArguments (getEntryPoint? functionLiteral)
 *   : typeArguments (getEntryPoint? functionLiteral)
 *   : arrayAccess
 *   : memberAccessOperation postfixUnaryExpression // TODO: Review
 *   ;
 */
func (e *exprParsing) parsePostfixExpression() {
	expression := e.mark()

	var firstExpressionParsed bool
	if e.at(KindCOLONCOLON) {
		firstExpressionParsed = e.parseDoubleColonSuffix(e.mark())
	} else {
		firstExpressionParsed = e.parseAtomicExpression()
	}

	for {
		if e.interruptedWithNewLine() {
			break
		} else if e.at(KindLBRACKET) {
			e.parseArrayAccess()
			expression.Done(KindARRAY_ACCESS_EXPRESSION)
		} else if e.parseCallSuffix() {
			expression.Done(KindCALL_EXPRESSION)
		} else if e.at(KindDOT) || e.at(KindSAFE_ACCESS) {
			expressionType := KindSAFE_ACCESS_EXPRESSION
			if e.at(KindDOT) {
				expressionType = KindDOT_QUALIFIED_EXPRESSION
			}
			e.advance() // DOT or SAFE_ACCESS

			if !firstExpressionParsed {
				expression.Drop()
				expression = e.mark()
				firstExpressionParsed = e.parseAtomicExpression()
				continue
			}

			e.parseSelectorCallExpression()

			expression.Done(expressionType)
		} else if e.atSet(epPostfixOperations) {
			e.parseOperationReference()
			expression.Done(KindPOSTFIX_EXPRESSION)
		} else {
			e.skipQuestionMarksBeforeDoubleColon()
			if !e.parseDoubleColonSuffix(expression) {
				break
			}
		}
		expression = expression.Precede()
	}
	expression.Drop()
}

/*
 * callSuffix
 *   : typeArguments? valueArguments annotatedLambda
 *   : typeArguments annotatedLambda
 *   ;
 */
func (e *exprParsing) parseCallSuffix() bool {
	if e.parseCallWithClosure() {
		// do nothing
	} else if e.at(KindLPAR) {
		e.parseValueArgumentList()
		e.parseCallWithClosure()
	} else if e.at(KindLT) {
		typeArgumentList := e.mark()
		if e.kp.tryParseTypeArgumentList(epTypeArgumentListStoppers) {
			typeArgumentList.Done(KindTYPE_ARGUMENT_LIST)
			if !e.b.newlineBeforeCurrentToken() && e.at(KindLPAR) {
				e.parseValueArgumentList()
			}
			e.parseCallWithClosure()
		} else {
			typeArgumentList.RollbackTo()
			return false
		}
	} else {
		return false
	}

	return true
}

/*
 * atomicExpression typeParameters? valueParameters? functionLiteral*
 */
func (e *exprParsing) parseSelectorCallExpression() {
	mark := e.mark()
	e.parseAtomicExpression()
	if !e.b.newlineBeforeCurrentToken() && e.parseCallSuffix() {
		mark.Done(KindCALL_EXPRESSION)
	} else {
		mark.Drop()
	}
}

func (e *exprParsing) parseOperationReference() {
	operationReference := e.mark()
	e.advance() // operation
	operationReference.Done(KindOPERATION_REFERENCE)
}

/*
 * annotatedLambda*
 *
 * The by-clause expression parser overrides this to parse trailing
 * lambdas only inside brackets (KotlinParsing's anonymous subclass).
 */
func (e *exprParsing) parseCallWithClosure() bool {
	if e.byClause != nil && e.byClause.stackSize <= 0 {
		return false
	}

	success := false

	for {
		argument := e.mark()

		if !e.parseAnnotatedLambda( /* preferBlock = */ false) {
			argument.Drop()
			break
		}

		argument.Done(KindLAMBDA_ARGUMENT)
		success = true
	}

	return success
}

/*
 * annotatedLambda
 *  : ("@" annotationEntry)* labelDefinition? functionLiteral
 */
func (e *exprParsing) parseAnnotatedLambda(preferBlock bool) bool {
	annotated := e.mark()

	wereAnnotations := e.kp.parseAnnotations(annDefault)
	labeled := e.mark()

	wasLabel := e.isAtLabelDefinitionOrMissingIdentifier()
	if wasLabel {
		e.parseLabelDefinition()
	}

	if !e.at(KindLBRACE) {
		annotated.RollbackTo()
		return false
	}

	e.parseFunctionLiteralWith(preferBlock /* collapse = */, true)

	epDoneOrDrop(labeled, KindLABELED_EXPRESSION, wasLabel)
	epDoneOrDrop(annotated, KindANNOTATED_EXPRESSION, wereAnnotations)

	return true
}

// Java: doneOrDrop
func epDoneOrDrop(marker Marker, typ Kind, condition bool) {
	if condition {
		marker.Done(typ)
	} else {
		marker.Drop()
	}
}

func (e *exprParsing) isAtLabelDefinitionOrMissingIdentifier() bool {
	return (e.at(KindIDENTIFIER) && e.b.rawLookup(1) == KindAT) || e.at(KindAT)
}

/*
 * atomicExpression
 *   : "this" label?
 *   : "super" ("<" type ">")? label?
 *   : objectLiteral
 *   : jump
 *   : if
 *   : when
 *   : try
 *   : loop
 *   : literalConstant
 *   : functionLiteral
 *   : declaration
 *   : SimpleName
 *   : collectionLiteral
 *   ;
 */
func (e *exprParsing) parseAtomicExpression() bool {
	ok := true

	switch e.tt() { // Java: getTokenId()
	case KindLPAR:
		e.parseParenthesizedExpression()
	case KindLBRACKET:
		e.parseCollectionLiteralExpression()
	case KindTHIS_KEYWORD:
		e.parseThisExpression()
	case KindSUPER_KEYWORD:
		e.parseSuperExpression()
	case KindOBJECT_KEYWORD:
		e.parseObjectLiteral()
	case KindTHROW_KEYWORD:
		e.parseThrow()
	case KindRETURN_KEYWORD:
		e.parseReturn()
	case KindCONTINUE_KEYWORD:
		e.parseJump(KindCONTINUE)
	case KindBREAK_KEYWORD:
		e.parseJump(KindBREAK)
	case KindIF_KEYWORD:
		e.parseIf()
	case KindWHEN_KEYWORD:
		e.parseWhen()
	case KindTRY_KEYWORD:
		e.parseTry()
	case KindFOR_KEYWORD:
		e.parseFor()
	case KindWHILE_KEYWORD:
		e.parseWhile()
	case KindDO_KEYWORD:
		e.parseDoWhile()
	case KindIDENTIFIER:
		// Try to parse anonymous function with context parameters
		if e.at(KindCONTEXT_KEYWORD) && e.lookahead(1) == KindLPAR {
			if e.parseLocalDeclaration(true, false) {
				break
			} else {
				e.at(KindIDENTIFIER)
			}
		}

		e.parseSimpleNameExpression()
	case KindLBRACE:
		e.parseFunctionLiteral()
	case KindINTERPOLATION_PREFIX, KindOPEN_QUOTE:
		e.parseStringTemplate()
	/*
	 * literalConstant
	 *   : "true" | "false"
	 *   : stringTemplate
	 *   : NoEscapeString
	 *   : IntegerLiteral
	 *   : CharacterLiteral
	 *   : FloatLiteral
	 *   : "null"
	 *   ;
	 */
	case KindTRUE_KEYWORD, KindFALSE_KEYWORD:
		e.parseOneTokenExpression(KindBOOLEAN_CONSTANT)
	case KindINTEGER_LITERAL:
		e.parseOneTokenExpression(KindINTEGER_CONSTANT)
	case KindCHARACTER_LITERAL:
		e.parseOneTokenExpression(KindCHARACTER_CONSTANT)
	case KindFLOAT_LITERAL:
		e.parseOneTokenExpression(KindFLOAT_CONSTANT)
	case KindNULL_KEYWORD:
		e.parseOneTokenExpression(KindNULL)
	case KindCLASS_KEYWORD, KindINTERFACE_KEYWORD, KindFUN_KEYWORD, KindVAL_KEYWORD, KindVAR_KEYWORD, KindTYPE_ALIAS_KEYWORD:
		if !e.parseLocalDeclaration( /* rollbackIfDefinitelyNotExpression = */ e.b.newlineBeforeCurrentToken(), false) {
			ok = false
		}
		// declaration was parsed, do nothing
	default:
		ok = false
	}

	if !ok {
		// TODO: better recovery if FIRST(element) did not match
		e.errorWithRecovery("Expecting an element", &epAtomicRecoverySet)
	}

	return ok
}

/*
 * stringTemplate
 *   : INTERPOLATION_PREFIX OPEN_QUOTE stringTemplateElement* CLOSING_QUOTE
 *   ;
 */
func (e *exprParsing) parseStringTemplate() {
	template := e.mark()

	if e.at(KindINTERPOLATION_PREFIX) {
		mark := e.mark()
		e.advance() // INTERPOLATION_PREFIX
		mark.Done(KindSTRING_INTERPOLATION_PREFIX)
	}

	e.advance() // OPEN_QUOTE

	for !e.eof() {
		if e.at(KindCLOSING_QUOTE) || e.at(KindDANGLING_NEWLINE) {
			break
		}
		e.parseStringTemplateElement()
	}

	if e.at(KindDANGLING_NEWLINE) {
		e.errorAndAdvance("Expecting '\"'")
	} else {
		e.expectMsg(KindCLOSING_QUOTE, "Expecting '\"'")
	}
	template.Done(KindSTRING_TEMPLATE)
}

/*
 * stringTemplateElement
 *   : RegularStringPart
 *   : ShortTemplateEntrySTART (SimpleName | "this")
 *   : EscapeSequence
 *   : longTemplate
 *   ;
 *
 * longTemplate
 *   : "${" expression "}"
 *   ;
 */
func (e *exprParsing) parseStringTemplateElement() {
	if e.at(KindREGULAR_STRING_PART) {
		mark := e.mark()
		e.advance() // REGULAR_STRING_PART
		mark.Done(KindLITERAL_STRING_TEMPLATE_ENTRY)
	} else if e.at(KindESCAPE_SEQUENCE) {
		mark := e.mark()
		e.advance() // ESCAPE_SEQUENCE
		mark.Done(KindESCAPE_STRING_TEMPLATE_ENTRY)
	} else if e.at(KindSHORT_TEMPLATE_ENTRY_START) {
		entry := e.mark()
		e.advance() // SHORT_TEMPLATE_ENTRY_START

		if e.at(KindTHIS_KEYWORD) {
			thisExpression := e.mark()
			reference := e.mark()
			e.advance() // THIS_KEYWORD
			reference.Done(KindREFERENCE_EXPRESSION)
			thisExpression.Done(KindTHIS_EXPRESSION)
		} else {
			keyword, isKeyword := Kind(0), false
			if text, ok := e.b.tokenText(); ok {
				keyword, isKeyword = epKeywordTexts[text]
			}
			if isKeyword {
				e.b.remapCurrentToken(keyword)
				e.errorAndAdvance("Keyword cannot be used as a reference")
			} else {
				reference := e.mark()
				e.expectMsg(KindIDENTIFIER, "Expecting a name")
				reference.Done(KindREFERENCE_EXPRESSION)
			}
		}

		entry.Done(KindSHORT_STRING_TEMPLATE_ENTRY)
	} else if e.at(KindLONG_TEMPLATE_ENTRY_START) {
		longTemplateEntry := e.mark()

		e.advance() // LONG_TEMPLATE_ENTRY_START

		for !e.eof() {
			offset := e.b.currentOffset()

			e.parseExpression()

			if e._at(KindLONG_TEMPLATE_ENTRY_END) {
				e.advance()
				break
			} else {
				e.error("Expecting '}'")
				if offset == e.b.currentOffset() {
					// Prevent hang if can't advance with parseExpression()
					e.advance()
				}
			}
		}

		longTemplateEntry.Done(KindLONG_STRING_TEMPLATE_ENTRY)
	} else {
		e.errorAndAdvance("Unexpected token in a string template")
	}
}

/*
 * when
 *   : "when" ("(" (modifiers "val" SimpleName "=")? element ")")? "{"
 *         whenEntry*
 *     "}"
 *   ;
 */
func (e *exprParsing) parseWhen() {
	when := e.mark()

	e.advance() // WHEN_KEYWORD

	// Parse condition
	e.b.disableNewlines()
	if e.at(KindLPAR) {
		e.advanceAt(KindLPAR)

		atWhenStart := e.mark()
		e.kp.parseAnnotationsList(epEqRParSet)
		if e.at(KindVAL_KEYWORD) || e.at(KindVAR_KEYWORD) {
			declType := e.kp.parseProperty(declLocal)

			atWhenStart.Done(declType)
			atWhenStart.SetCustomEdgeTokenBinders(precedingDocCommentsBinder, trailingCommentsBinder)
		} else {
			atWhenStart.RollbackTo()
			e.parseExpression()
		}

		e.expectMsg(KindRPAR, "Expecting ')'")
	}
	e.b.restoreNewlinesState()

	// Parse when block
	e.b.enableNewlines()
	if e.expectMsg(KindLBRACE, "Expecting '{'") {
		for !e.eof() && !e.at(KindRBRACE) {
			e.parseWhenEntry()
		}

		e.expectMsg(KindRBRACE, "Expecting '}'")
	}
	e.b.restoreNewlinesState()

	when.Done(KindWHEN)
}

/*
 * whenEntry
 *   // TODO : consider empty after ->
 *   : whenCondition{","} whenEntryGuard? "->" element SEMI
 *   : "else" whenEntryGuard? "->" element SEMI
 *   ;
 */
func (e *exprParsing) parseWhenEntry() {
	entry := e.mark()

	if e.at(KindELSE_KEYWORD) {
		e.advance() // ELSE_KEYWORD

		e.parseWhenEntryGuardOrSuggest()

		if !e.at(KindARROW) {
			e.errorUntil("Expecting '->'", epWhenElseArrowRecoverySet)
		}

		if e.at(KindARROW) {
			e.advance() // ARROW

			if e.atSet(epWhenConditionRecoverySet) {
				e.error("Expecting an element")
			} else {
				e.parseControlStructureBody()
			}
		} else if e.at(KindLBRACE) { // no arrow, probably it's simply missing
			e.parseControlStructureBody()
		} else if !e.atSet(epWhenConditionRecoverySet) {
			e.errorAndAdvance("Expecting '->'")
		}
	} else {
		e.parseWhenEntryNotElse()
	}

	entry.Done(KindWHEN_ENTRY)
	e.consumeIf(KindSEMICOLON)
}

/*
 * : whenCondition{","} whenEntryGuard? "->" element SEMI
 */
func (e *exprParsing) parseWhenEntryNotElse() {
	for {
		for e.at(KindCOMMA) {
			e.errorAndAdvance("Expecting a when-condition")
		}
		e.parseWhenCondition()
		if !e.at(KindCOMMA) {
			break
		}
		e.advance() // COMMA
		if e.at(KindARROW) {
			break
		}
	}

	e.parseWhenEntryGuardOrSuggest()

	e.expectRS(KindARROW, "Expecting '->'", &epWhenConditionRecoverySet)
	if e.atSet(epWhenConditionRecoverySet) {
		e.error("Expecting an element")
	} else {
		e.parseControlStructureBody()
	}
	// SEMI is consumed in parseWhenEntry
}

/*
 * whenCondition
 *   : expression
 *   : ("in" | "!in") expression
 *   : ("is" | "!is") isRHS
 *   ;
 */
func (e *exprParsing) parseWhenCondition() {
	condition := e.mark()
	e.b.disableNewlines()
	switch e.tt() { // Java: getTokenId()
	case KindIN_KEYWORD, KindNOT_IN:
		mark := e.mark()
		e.advance() // IN_KEYWORD or NOT_IN
		mark.Done(KindOPERATION_REFERENCE)

		if e.atSet(epWhenConditionRecoverySetWithArrow) {
			e.error("Expecting an element")
		} else {
			e.parseExpression()
		}
		condition.Done(KindWHEN_CONDITION_IN_RANGE)
	case KindIS_KEYWORD, KindNOT_IS:
		e.advance() // IS_KEYWORD or NOT_IS

		if e.atSet(epWhenConditionRecoverySetWithArrow) {
			e.error("Expecting a type")
		} else {
			e.kp.parseTypeRef()
		}
		condition.Done(KindWHEN_CONDITION_IS_PATTERN)
	case KindRBRACE, KindELSE_KEYWORD, KindARROW, KindDOT:
		e.error("Expecting an expression, is-condition or in-condition")
		condition.Done(KindWHEN_CONDITION_EXPRESSION)
	default:
		e.parseExpression()
		condition.Done(KindWHEN_CONDITION_EXPRESSION)
	}
	e.b.restoreNewlinesState()
}

func (e *exprParsing) parseWhenEntryGuardOrSuggest() {
	if e.at(KindANDAND) {
		e.errorUntil("Unexpected '&&', use 'if' to introduce additional conditions; see https://kotl.in/guards-in-when", epLbraceRbraceArrowSet)
	} else if e.at(KindIF_KEYWORD) {
		e.parseWhenEntryGuard()
	}
}

/*
 * whenEntryGuard
 *   : "if" expression
 *   ;
 */
func (e *exprParsing) parseWhenEntryGuard() {
	guard := e.mark()
	e.advance() // IF_KEYWORD
	e.parseExpression()
	guard.Done(KindWHEN_ENTRY_GUARD)
}

/*
 * arrayAccess
 *   : "[" element{","} "]"
 *   ;
 */
func (e *exprParsing) parseArrayAccess() {
	e.parseAsCollectionLiteralExpression(KindINDICES, false, "Expecting an index element")
}

/*
 * collectionLiteral
 *   : "[" element{","}? "]"
 *   ;
 */
func (e *exprParsing) parseCollectionLiteralExpression() {
	e.parseAsCollectionLiteralExpression(KindCOLLECTION_LITERAL_EXPRESSION, true, "Expecting an element")
}

func (e *exprParsing) parseAsCollectionLiteralExpression(nodeType Kind, canBeEmpty bool, missingElementErrorMessage string) {
	innerExpressions := e.mark()

	e.b.disableNewlines()
	e.advance() // LBRACKET

	if !canBeEmpty && e.at(KindRBRACKET) {
		e.error(missingElementErrorMessage)
	} else {
		e.parseInnerExpressions(missingElementErrorMessage)
	}

	e.expectMsg(KindRBRACKET, "Expecting ']'")
	e.b.restoreNewlinesState()

	innerExpressions.Done(nodeType)
}

func (e *exprParsing) parseInnerExpressions(missingElementErrorMessage string) {
	for {
		if e.at(KindRBRACKET) {
			break
		}
		e.parseExpressionMsg(missingElementErrorMessage)

		if !e.at(KindCOMMA) {
			break
		}
		e.advance() // COMMA
	}
}

func (e *exprParsing) parseContractDescriptionBlock() {
	e.advance() // CONTRACT_KEYWORD

	e.parseContractEffectList()
}

func (e *exprParsing) parseContractEffectList() {
	block := e.mark()

	e.expectMsg(KindLBRACKET, "Expecting '['")
	e.b.enableNewlines()

	e.parseContractEffects()

	e.expectMsg(KindRBRACKET, "Expecting ']'")
	e.b.restoreNewlinesState()

	block.Done(KindCONTRACT_EFFECT_LIST)
}

func (e *exprParsing) parseContractEffects() {
	for {
		if e.at(KindCOMMA) {
			e.errorAndAdvance("Expecting a contract effect")
		}
		if e.at(KindRBRACKET) {
			break
		}
		effect := e.mark()
		e.parseExpression()
		effect.Done(KindCONTRACT_EFFECT)

		if !e.at(KindCOMMA) {
			break
		}
		e.advance() // COMMA
	}
}

/*
 * SimpleName
 */
func (e *exprParsing) parseSimpleNameExpression() {
	simpleName := e.mark()
	e.expectMsg(KindIDENTIFIER, "Expecting an identifier")
	simpleName.Done(KindREFERENCE_EXPRESSION)
}

/*
 * modifiers declarationRest
 */
func (e *exprParsing) parseLocalDeclaration(rollbackIfDefinitelyNotExpression, isScriptTopLevel bool) bool {
	decl := e.mark()
	detector := &modifierDetector{}
	e.kp.parseModifierListC(detector.consume, tokenSet{}, true)

	declType := e.parseLocalDeclarationRest(detector, rollbackIfDefinitelyNotExpression, isScriptTopLevel)

	if declType != kNone {
		// we do not attach preceding comments (non-doc) to local variables because they are likely commenting a few statements below
		closeDeclarationWithCommentBinders(decl, declType,
			declType != KindPROPERTY && declType != KindDESTRUCTURING_DECLARATION)
		return true
	}
	decl.RollbackTo()
	return false
}

/*
 * functionLiteral  // one can use "it" as a parameter name
 *   : "{" expressions "}"
 *   : "{" (modifiers SimpleName (":" type)?){","} "->" statements "}"
 *   ;
 */
func (e *exprParsing) parseFunctionLiteral() {
	e.parseFunctionLiteralWith( /* preferBlock = */ false /* collapse = */, true)
}

// parseFunctionLiteralWith is parseFunctionLiteral(preferBlock, collapse).
// If it has no ->, it's a block, otherwise a function literal.
func (e *exprParsing) parseFunctionLiteralWith(preferBlock, collapse bool) {
	literalExpression := e.mark()

	literal := e.mark()

	e.b.enableNewlines()
	e.advance() // LBRACE

	paramsFound := false

	token := e.tt()
	if token == KindARROW {
		//   { -> ...}
		e.mark().Done(KindVALUE_PARAMETER_LIST)
		e.advance() // ARROW
		paramsFound = true
	} else if token == KindIDENTIFIER || token == KindCOLON || token == KindLPAR || token == KindLBRACKET {
		// Try to parse a simple name list followed by an ARROW
		//   {a -> ...}
		//   {a, b -> ...}
		//   {(a, b) -> ... }
		rollbackMarker := e.mark()
		nextToken := e.lookahead(1)
		preferParamsToExpressions := nextToken == KindCOMMA || nextToken == KindCOLON
		e.parseFunctionLiteralParameterList()

		if preferParamsToExpressions {
			paramsFound = e.rollbackOrDrop(rollbackMarker, KindARROW, "An -> is expected", KindRBRACE)
		} else {
			paramsFound = e.rollbackOrDropAt(rollbackMarker, KindARROW)
		}
	}

	if !paramsFound && preferBlock {
		literal.Drop()
		e.parseStatements()
		e.expectMsg(KindRBRACE, "Expecting '}'")
		literalExpression.Done(KindBLOCK)
		e.b.restoreNewlinesState()

		return
	}

	if collapse && e.isLazy {
		e.kp.advanceBalancedBlock()
		literal.Done(KindFUNCTION_LITERAL)
		literalExpression.Collapse(KindLAMBDA_EXPRESSION)
	} else {
		body := e.mark()
		e.parseStatements()

		body.Done(KindBLOCK)
		body.SetCustomEdgeTokenBinders(precedingAllCommentsBinder, trailingAllCommentsBinder)

		e.expectMsg(KindRBRACE, "Expecting '}'")
		literal.Done(KindFUNCTION_LITERAL)
		literalExpression.Done(KindLAMBDA_EXPRESSION)
	}

	e.b.restoreNewlinesState()
}

func (e *exprParsing) rollbackOrDropAt(rollbackMarker Marker, dropAt Kind) bool {
	if e.at(dropAt) {
		e.advance() // dropAt
		rollbackMarker.Drop()
		return true
	}
	rollbackMarker.RollbackTo()
	return false
}

func (e *exprParsing) rollbackOrDrop(rollbackMarker Marker, expected Kind, expectMessage string, validForDrop Kind) bool {
	if e.at(expected) {
		e.advance() // dropAt
		rollbackMarker.Drop()
		return true
	} else if e.at(validForDrop) {
		rollbackMarker.Drop()
		e.expectMsg(expected, expectMessage)
		return true
	}

	rollbackMarker.RollbackTo()
	return false
}

/*
 * lambdaParameter{","}
 *
 * lambdaParameter
 *   : variableDeclarationEntry
 *   : multipleVariableDeclarations (":" type)?
 */
func (e *exprParsing) parseFunctionLiteralParameterList() {
	parameterList := e.mark()

	for !e.eof() {
		if e.at(KindARROW) {
			break
		}
		parameter := e.mark()

		if e.at(KindCOLON) {
			e.error("Expecting parameter name")
		} else if e.at(KindLPAR) || e.at(KindLBRACKET) {
			destructuringDeclaration := e.mark()
			e.kp.parseMultiDeclarationEntry(epTokenSetToFollowAfterDestructuringDeclarationInLambda,
				epTokenSetToFollowAfterDestructuringDeclarationInLambdaRecovery,
				// No var in lambda parameter destructuring
				e.destructuringMode())
			destructuringDeclaration.Done(KindDESTRUCTURING_DECLARATION)
		} else {
			e.expectRS(KindIDENTIFIER, "Expecting parameter name", &epArrowSet)
		}

		if e.at(KindCOLON) {
			e.advance() // COLON
			e.kp.parseTypeRefRS(epArrowCommaSet)
		}
		parameter.Done(KindVALUE_PARAMETER)

		if e.at(KindARROW) {
			break
		} else if e.at(KindCOMMA) {
			e.advance() // COMMA
		} else {
			e.error("Expecting '->' or ','")
			break
		}
	}

	parameterList.Done(KindVALUE_PARAMETER_LIST)
}

/*
 * expressions
 *   : SEMI* statement{SEMI+} SEMI*
 */
func (e *exprParsing) parseStatements() {
	e.parseStatementsScript(false)
}

// parseStatementsScript is parseStatements(isScriptTopLevel).
func (e *exprParsing) parseStatementsScript(isScriptTopLevel bool) {
	for e.at(KindSEMICOLON) {
		e.advance() // SEMICOLON
	}
	for !e.eof() && !e.at(KindRBRACE) {
		if !e.atSet(epStatementFirst) {
			e.errorAndAdvance("Expecting an element")
		}
		if e.atSet(epStatementFirst) {
			e.parseStatement(isScriptTopLevel)
		}
		if e.at(KindSEMICOLON) {
			for e.at(KindSEMICOLON) {
				e.advance() // SEMICOLON
			}
		} else if e.at(KindRBRACE) {
			break
		} else if !isScriptTopLevel && !e.b.newlineBeforeCurrentToken() {
			severalStatementsError := "Unexpected tokens (use ';' to separate expressions on the same line)"

			if e.atSet(epStatementNewLineQuickRecoverySet) {
				e.error(severalStatementsError)
			} else {
				e.errorUntil(severalStatementsError, epSeveralStatementsRecoverySet)
			}
		}
	}
}

/*
 * statement
 *  : declaration
 *  : blockLevelExpression
 *  ;
 */
func (e *exprParsing) parseStatement(isScriptTopLevel bool) {
	if !e.parseLocalDeclaration( /* rollbackIfDefinitelyNotExpression = */ false /* isScriptTopLevel = */, isScriptTopLevel) {
		if !e.atSet(epExpressionFirst) {
			e.errorAndAdvance("Expecting a statement")
		} else if isScriptTopLevel {
			scriptInitializer := e.mark()
			e.parseBlockLevelExpression()
			scriptInitializer.Done(KindSCRIPT_INITIALIZER)
		} else {
			e.parseBlockLevelExpression()
		}
	}
}

/*
 * blockLevelExpression
 *  : annotations + ("\n")+ expression
 *  ;
 */
func (e *exprParsing) parseBlockLevelExpression() {
	if e.at(KindAT) {
		expression := e.mark()
		e.kp.parseAnnotations(annDefault)

		if !e.b.newlineBeforeCurrentToken() {
			expression.RollbackTo()
			e.parseExpression()
			return
		}

		e.parseBlockLevelExpression()
		expression.Done(KindANNOTATED_EXPRESSION)
		return
	}

	e.parseExpression()
}

/*
 * declaration
 *   : function
 *   : property
 *   : extension
 *   : class
 *   : typeAlias
 *   : object
 *   ;
 *
 * Returns kNone (Java null) when no declaration was parsed.
 */
func (e *exprParsing) parseLocalDeclarationRest(modifierDetector *modifierDetector, failIfDefinitelyNotExpression, isScriptTopLevel bool) Kind {
	keywordToken := e.tt()
	if failIfDefinitelyNotExpression {
		if keywordToken != KindFUN_KEYWORD {
			return kNone
		}

		return e.kp.parseFunctionFail( /* failIfIdentifierExists = */ true)
	}

	if keywordToken == KindOBJECT_KEYWORD {
		// Object expression may appear at the statement position: should parse it
		// as expression instead of object declaration
		// sample:
		// {
		//   object : Thread() {
		//   }
		// }
		lookahead := e.lookahead(1)
		if lookahead == KindCOLON || lookahead == KindLBRACE {
			return kNone
		}
	}

	mode := declLocal
	if isScriptTopLevel {
		mode = declScriptTopLevel
	}
	return e.kp.parseCommonDeclaration(modifierDetector, nameRequired, mode)
}

/*
 * doWhile
 *   : "do" element "while" "(" element ")"
 *   ;
 */
func (e *exprParsing) parseDoWhile() {
	loop := e.mark()

	e.advance() // DO_KEYWORD

	if !e.at(KindWHILE_KEYWORD) {
		e.parseLoopBody()
	}

	if e.expectMsg(KindWHILE_KEYWORD, "Expecting 'while' followed by a post-condition") {
		e.parseCondition()
	}

	loop.Done(KindDO_WHILE)
}

/*
 * while
 *   : "while" "(" element ")" element
 *   ;
 */
func (e *exprParsing) parseWhile() {
	loop := e.mark()

	e.advance() // WHILE_KEYWORD

	e.parseCondition()

	e.parseLoopBody()

	loop.Done(KindWHILE)
}

/*
 * for
 *   : "for" "(" annotations ("val" | "var")? (multipleVariableDeclarations | variableDeclarationEntry) "in" expression ")" expression
 *   ;
 *
 *   TODO: empty loop body (at the end of the block)?
 */
func (e *exprParsing) parseFor() {
	loop := e.mark()

	e.advance() // FOR_KEYWORD

	if e.expectRS(KindLPAR, "Expecting '(' to open a loop range", &epExpressionFirst) {
		e.b.disableNewlines()

		if !e.at(KindRPAR) {
			parameter := e.mark()

			if !e.at(KindIN_KEYWORD) {
				e.kp.parseModifierList(epInKeywordRParColonSet)
			}

			if e.at(KindVAL_KEYWORD) || e.at(KindVAR_KEYWORD) {
				e.advance() // VAL_KEYWORD or VAR_KEYWORD
			}

			if e.at(KindLPAR) || e.at(KindLBRACKET) {
				destructuringDeclaration := e.mark()
				e.kp.parseMultiDeclarationEntry(epInKeywordLBraceSet, epInKeywordLBraceRecoverySet,
					// No var in destructured loop parameter
					e.destructuringMode())
				destructuringDeclaration.Done(KindDESTRUCTURING_DECLARATION)
			} else {
				e.expectRS(KindIDENTIFIER, "Expecting a variable name", &epColonInKeywordSet)

				if e.at(KindCOLON) {
					e.advance() // COLON
					e.kp.parseTypeRefRS(epInKeywordSet)
				}
			}
			parameter.Done(KindVALUE_PARAMETER)

			if e.expectRS(KindIN_KEYWORD, "Expecting 'in'", &epLParLBraceRParSet) {
				rng := e.mark()
				e.parseExpression()
				rng.Done(KindLOOP_RANGE)
			}
		} else {
			e.error("Expecting a variable name")
		}

		e.expectNoAdvance(KindRPAR, "Expecting ')'")
		e.b.restoreNewlinesState()
	}

	e.parseLoopBody()

	loop.Done(KindFOR)
}

func (e *exprParsing) parseControlStructureBody() {
	if !e.parseAnnotatedLambda( /* preferBlock = */ true) {
		e.parseBlockLevelExpression()
	}
}

/*
 * element
 */
func (e *exprParsing) parseLoopBody() {
	body := e.mark()
	if !e.at(KindSEMICOLON) {
		e.parseControlStructureBody()
	}
	body.Done(KindBODY)
}

/*
 * try
 *   : "try" block catchBlock* finallyBlock?
 *   ;
 * catchBlock
 *   : "catch" "(" annotations SimpleName ":" userType ")" block
 *   ;
 *
 * finallyBlock
 *   : "finally" block
 *   ;
 */
func (e *exprParsing) parseTry() {
	tryExpression := e.mark()

	e.advance() // TRY_KEYWORD

	e.kp.parseBlock()

	catchOrFinally := false
	for e.at(KindCATCH_KEYWORD) {
		catchOrFinally = true
		catchBlock := e.mark()
		e.advance() // CATCH_KEYWORD

		if e.atSet(epTryCatchRecoveryTokenSet) {
			e.error("Expecting exception variable declaration")
		} else {
			parameters := e.mark()
			e.expectRS(KindLPAR, "Expecting '('", &epTryCatchRecoveryTokenSet)
			if !e.atSet(epTryCatchRecoveryTokenSet) {
				e.kp.parseValueParameter( /*typeRequired = */ true)
				if e.at(KindCOMMA) {
					e.advance() // trailing comma
				}
				e.expectRS(KindRPAR, "Expecting ')'", &epTryCatchRecoveryTokenSet)
			} else {
				e.error("Expecting exception variable declaration")
			}
			parameters.Done(KindVALUE_PARAMETER_LIST)
		}

		if e.at(KindLBRACE) {
			e.kp.parseBlock()
		} else {
			e.error("Expecting a block: { ... }")
		}
		catchBlock.Done(KindCATCH)
	}

	if e.at(KindFINALLY_KEYWORD) {
		catchOrFinally = true
		finallyBlock := e.mark()

		e.advance() // FINALLY_KEYWORD

		e.kp.parseBlock()

		finallyBlock.Done(KindFINALLY)
	}

	if !catchOrFinally {
		e.error("Expecting 'catch' or 'finally'")
	}

	tryExpression.Done(KindTRY)
}

/*
 * if
 *   : "if" "(" element ")" element SEMI? ("else" element)?
 *   ;
 */
func (e *exprParsing) parseIf() {
	marker := e.mark()

	e.advance() //IF_KEYWORD

	e.parseCondition()

	thenBranch := e.mark()
	if !e.at(KindELSE_KEYWORD) && !e.at(KindSEMICOLON) {
		e.parseControlStructureBody()
	}
	if e.at(KindSEMICOLON) && e.lookahead(1) == KindELSE_KEYWORD {
		e.advance() // SEMICOLON
	}
	thenBranch.Done(KindTHEN)

	// lookahead for arrow is needed to prevent capturing of whenEntry like "else -> "
	if e.at(KindELSE_KEYWORD) && e.lookahead(1) != KindARROW {
		e.advance() // ELSE_KEYWORD

		elseBranch := e.mark()
		if !e.at(KindSEMICOLON) {
			e.parseControlStructureBody()
		}
		elseBranch.Done(KindELSE)
	}

	marker.Done(KindIF)
}

/*
 * "(" element ")"
 */
func (e *exprParsing) parseCondition() {
	e.b.disableNewlines()

	if e.expectRS(KindLPAR, "Expecting a condition in parentheses '(...)'", &epExpressionFirst) {
		condition := e.mark()
		e.parseExpression()
		condition.Done(KindCONDITION)
		e.expectMsg(KindRPAR, "Expecting ')")
	}

	e.b.restoreNewlinesState()
}

/*
 * : "continue" getEntryPoint?
 * : "break" getEntryPoint?
 */
func (e *exprParsing) parseJump(typ Kind) {
	marker := e.mark()

	e.advance() // BREAK_KEYWORD or CONTINUE_KEYWORD

	e.parseLabelReferenceWithNoWhitespace()

	marker.Done(typ)
}

/*
 * "return" getEntryPoint? element?
 */
func (e *exprParsing) parseReturn() {
	returnExpression := e.mark()

	e.advance() // RETURN_KEYWORD

	e.parseLabelReferenceWithNoWhitespace()

	if e.atSet(epExpressionFirst) && !e.at(kEOLOrSemicolon) {
		e.parseExpression()
	}

	returnExpression.Done(KindRETURN)
}

/*
 * labelReference?
 */
func (e *exprParsing) parseLabelReferenceWithNoWhitespace() {
	if e.at(KindAT) && !e.b.newlineBeforeCurrentToken() {
		if whiteSpaceOrCommentSet.has(e.b.rawLookup(-1)) {
			e.error("There should be no space or comments before '@' in label reference")
		}
		e.parseLabelReference()
	}
}

/*
 * IDENTIFIER "@"
 */
func (e *exprParsing) parseLabelDefinition() {
	labelWrap := e.mark()
	mark := e.mark()

	if e.at(KindAT) {
		e.errorAndAdvance("Expecting identifier before '@' in label definition")
		labelWrap.Drop()
		mark.Drop()
		return
	}

	e.advance() // IDENTIFIER
	e.advance() // AT

	mark.Done(KindLABEL)

	labelWrap.Done(KindLABEL_QUALIFIER)
}

/*
 * "@" IDENTIFIER
 */
func (e *exprParsing) parseLabelReference() {
	labelWrap := e.mark()

	mark := e.mark()

	if e.b.rawLookup(1) != KindIDENTIFIER {
		e.errorAndAdvance("Label must be named") // AT
		labelWrap.Drop()
		mark.Drop()
		return
	}

	e.advance() // AT
	e.advance() // IDENTIFIER

	mark.Done(KindLABEL)

	labelWrap.Done(KindLABEL_QUALIFIER)
}

/*
 * : "throw" element
 */
func (e *exprParsing) parseThrow() {
	marker := e.mark()

	e.advance() // THROW_KEYWORD

	e.parseExpression()

	marker.Done(KindTHROW)
}

/*
 * "(" expression ")"
 */
func (e *exprParsing) parseParenthesizedExpression() {
	mark := e.mark()

	e.b.disableNewlines()
	e.advance() // LPAR
	if e.at(KindRPAR) {
		e.error("Expecting an expression")
	} else {
		e.parseExpression()
	}

	e.expectMsg(KindRPAR, "Expecting ')'")
	e.b.restoreNewlinesState()

	mark.Done(KindPARENTHESIZED)
}

/*
 * "this" label?
 */
func (e *exprParsing) parseThisExpression() {
	mark := e.mark()

	thisReference := e.mark()
	e.advance() // THIS_KEYWORD
	thisReference.Done(KindREFERENCE_EXPRESSION)

	e.parseLabelReferenceWithNoWhitespace()

	mark.Done(KindTHIS_EXPRESSION)
}

/*
 * "this" ("<" type ">")? label?
 */
func (e *exprParsing) parseSuperExpression() {
	mark := e.mark()

	superReference := e.mark()
	e.advance() // SUPER_KEYWORD
	superReference.Done(KindREFERENCE_EXPRESSION)

	if e.at(KindLT) {
		// This may be "super < foo" or "super<foo>", thus the backtracking
		supertype := e.mark()

		e.b.disableNewlines()
		e.advance() // LT

		e.kp.parseTypeRef()

		if e.at(KindGT) {
			e.advance() // GT
			supertype.Drop()
		} else {
			supertype.RollbackTo()
		}
		e.b.restoreNewlinesState()
	}
	e.parseLabelReferenceWithNoWhitespace()

	mark.Done(KindSUPER_EXPRESSION)
}

/*
 * valueArguments
 *   : "(" (SimpleName "=")? "*"? element{","} ")"
 *   ;
 */
func (e *exprParsing) parseValueArgumentList() {
	list := e.mark()

	e.b.disableNewlines()

	if e.expectRS(KindLPAR, "Expecting an argument list", &epExpressionFollow) {
		if !e.at(KindRPAR) {
			for {
				e.parseValueArgument()
				if e.at(KindCOLON) && e.lookahead(1) == KindIDENTIFIER {
					e.errorAndAdvanceN("Unexpected type specification", 2)
				}
				if !e.at(KindCOMMA) {
					if e.atSet(kpExpressionFirst) {
						e.error("Expecting ','")
						continue
					} else {
						break
					}
				}
				e.advance() // COMMA
				if e.at(KindRPAR) {
					break
				}
			}
		}

		e.expectRS(KindRPAR, "Expecting ')'", &epExpressionFollow)
	}

	e.b.restoreNewlinesState()

	list.Done(KindVALUE_ARGUMENT_LIST)
}

/*
 * (SimpleName "=")? "*"? element
 */
func (e *exprParsing) parseValueArgument() {
	argument := e.mark()
	if e.at(KindIDENTIFIER) && e.lookahead(1) == KindEQ {
		argName := e.mark()
		reference := e.mark()
		e.advance() // IDENTIFIER
		reference.Done(KindREFERENCE_EXPRESSION)
		argName.Done(KindVALUE_ARGUMENT_NAME)
		e.advance() // EQ
	}
	if e.at(KindMUL) {
		e.advance() // MUL
	}
	e.parseExpressionMsg("Expecting an argument")
	argument.Done(KindVALUE_ARGUMENT)
}

/*
 * "object" (":" delegationSpecifier{","})? classBody // Cannot make class body optional: foo(object : F, A)
 */
func (e *exprParsing) parseObjectLiteral() {
	literal := e.mark()
	declaration := e.mark()
	e.kp.parseObject(nameProhibited, false) // Body is not optional because of foo(object : A, B)
	declaration.Done(KindOBJECT_DECLARATION)
	literal.Done(KindOBJECT_LITERAL)
}

func (e *exprParsing) parseOneTokenExpression(typ Kind) {
	mark := e.mark()
	e.advance()
	mark.Done(typ)
}

func (e *exprParsing) interruptedWithNewLine() bool {
	return !epAllowNewlineOperations.has(e.tt()) && e.b.newlineBeforeCurrentToken()
}

// destructuringMode is `lookahead(1) == VAL_KEYWORD ? FULL_VAL_ONLY : SHORT`,
// used for lambda parameters and for-loop variables.
func (e *exprParsing) destructuringMode() multiDeclarationMode {
	if e.lookahead(1) == KindVAL_KEYWORD {
		return multiDeclFullValOnly
	}
	return multiDeclShort
}
