package psi

// Port of KotlinParsing from Kotlin 2.4.21 (see THIRD_PARTY_NOTICES.md).
// Methods keep the Java order, control flow and error messages so that
// trees match stock Kotlin. Java null element types are kNone; Java
// overloads get suffixed names (see parsing_shared.go).

var (
	kpGtCommaColonSet          = newSet(KindGT, KindCOMMA, KindCOLON)
	kpTopLevelDeclarationFirst = newSet(KindTYPE_ALIAS_KEYWORD, KindINTERFACE_KEYWORD, KindCLASS_KEYWORD,
		KindOBJECT_KEYWORD, KindFUN_KEYWORD, KindVAL_KEYWORD, KindVAR_KEYWORD, KindPACKAGE_KEYWORD)
	kpTopLevelDeclarationFirstSemicolonSet     = orSet(kpTopLevelDeclarationFirst, newSet(KindSEMICOLON))
	kpLtEqSemicolonTopLevelDeclarationFirstSet = orSet(newSet(KindLT, KindEQ, KindSEMICOLON),
		kpTopLevelDeclarationFirst)
	kpDeclarationFirst = orSet(kpTopLevelDeclarationFirst,
		newSet(KindINIT_KEYWORD, KindGET_KEYWORD, KindSET_KEYWORD, KindCONSTRUCTOR_KEYWORD))

	kpClassNameRecoverySet = orSet(newSet(KindLT, KindLPAR, KindCOLON, KindLBRACE),
		kpTopLevelDeclarationFirst)
	kpTypeParameterGtRecoverySet            = newSet(KindWHERE_KEYWORD, KindLPAR, KindCOLON, KindLBRACE, KindGT)
	kpParameterNameRecoverySet              = newSet(KindCOLON, KindEQ, KindCOMMA, KindRPAR, KindVAL_KEYWORD, KindVAR_KEYWORD)
	kpPackageNameRecoverySet                = newSet(KindDOT, kEOLOrSemicolon)
	kpImportRecoverySet                     = newSet(KindAS_KEYWORD, KindDOT, kEOLOrSemicolon)
	kpTypeRefFirst                          = newSet(KindLBRACKET, KindIDENTIFIER, KindLPAR, KindHASH, KindDYNAMIC_KEYWORD)
	kpLbraceRbraceTypeRefFirstSet           = orSet(newSet(KindLBRACE, KindRBRACE), kpTypeRefFirst)
	kpColonCommaLbraceRbraceTypeRefFirstSet = orSet(newSet(KindCOLON, KindCOMMA, KindLBRACE, KindRBRACE),
		kpTypeRefFirst)
	kpReceiverTypeTerminators = newSet(KindDOT, KindSAFE_ACCESS)
	kpValueParameterFirst     = orSet(
		newSet(KindIDENTIFIER, KindLBRACKET, KindVAL_KEYWORD, KindVAR_KEYWORD),
		modifierKeywords.minus(newSet(KindFUN_KEYWORD)))
	kpLambdaValueParameterFirst = orSet(
		newSet(KindIDENTIFIER, KindLBRACKET),
		modifierKeywords.minus(newSet(KindFUN_KEYWORD)))
	kpSoftKeywordsAtMemberStart = newSet(KindCONSTRUCTOR_KEYWORD, KindINIT_KEYWORD)
	kpAnnotationTargets         = newSet(KindALL_KEYWORD, KindFILE_KEYWORD, KindFIELD_KEYWORD, KindGET_KEYWORD,
		KindSET_KEYWORD, KindPROPERTY_KEYWORD, KindRECEIVER_KEYWORD, KindPARAM_KEYWORD, KindSETPARAM_KEYWORD,
		KindDELEGATE_KEYWORD)
	kpBlockDocCommentSet            = newSet(KindBLOCK_COMMENT, kDocComment)
	kpSemicolonSet                  = newSet(KindSEMICOLON)
	kpCommaColonGtSet               = newSet(KindCOMMA, KindCOLON, KindGT)
	kpIdentifierRbracketLbracketSet = newSet(KindIDENTIFIER, KindRBRACKET, KindLBRACKET)
	kpLbraceRbraceSet               = newSet(KindLBRACE, KindRBRACE)
	kpCommaSemicolonRbraceSet       = newSet(KindCOMMA, KindSEMICOLON, KindRBRACE)
	kpValueArgsRecoverySet          = newSet(KindLBRACE, KindSEMICOLON, KindRPAR, kEOLOrSemicolon, KindRBRACE)
	kpPropertyNameFollowSet         = newSet(KindCOLON, KindEQ, KindLBRACE, KindRBRACE, KindSEMICOLON,
		KindVAL_KEYWORD, KindVAR_KEYWORD, KindFUN_KEYWORD, KindCLASS_KEYWORD)
	kpDestructuringPropertyNameFollowSet              = kpPropertyNameFollowSet.minus(valVar)
	kpPropertyNameFollowMultiDeclarationRecoverySet   = orSet(kpPropertyNameFollowSet, kpParameterNameRecoverySet)
	kpPropertyNameFollowFunctionOrPropertyRecoverySet = orSet(kpPropertyNameFollowSet, kpLbraceRbraceSet,
		kpTopLevelDeclarationFirst)
	kpIdentifierEqColonSemicolonSet = newSet(KindIDENTIFIER, KindEQ, KindCOLON, KindSEMICOLON)
	kpCommaRparRbracketColonEqSet   = newSet(KindCOMMA, KindRPAR, KindRBRACKET, KindCOLON, KindEQ)
	kpAccessorFirstOrPropertyEnd    = orSet(modifierKeywords, newSet(KindAT, KindGET_KEYWORD, KindSET_KEYWORD,
		KindFIELD_KEYWORD, kEOLOrSemicolon, KindRBRACE))
	kpRparIdentifierColonLbraceEqSet = newSet(KindRPAR, KindIDENTIFIER, KindCOLON, KindLBRACE, KindEQ)
	kpCommaColonRparSet              = newSet(KindCOMMA, KindCOLON, KindRPAR)
	kpRparColonLbraceEqSet           = newSet(KindRPAR, KindCOLON, KindLBRACE, KindEQ)
	kpLbracketLbraceRbraceLparSet    = newSet(KindLBRACKET, KindLBRACE, KindRBRACE, KindLPAR)
	kpFunctionNameFollowSet          = newSet(KindLT, KindLPAR, KindRPAR, KindCOLON, KindEQ)
	kpFunctionNameRecoverySet        = orSet(newSet(KindLT, KindLPAR, KindRPAR, KindCOLON, KindEQ),
		kpLbraceRbraceSet, kpTopLevelDeclarationFirst)
	kpValueParametersFollowSet   = newSet(KindEQ, KindLBRACE, KindRBRACE, KindSEMICOLON, KindRPAR)
	kpContextParametersFollowSet = newSet(KindCLASS_KEYWORD, KindOBJECT_KEYWORD, KindFUN_KEYWORD,
		KindVAL_KEYWORD, KindVAR_KEYWORD)
	kpLparValueParametersFollowSet         = orSet(newSet(KindLPAR), kpValueParametersFollowSet)
	kpLparLbraceColonConstructorKeywordSet = newSet(KindLPAR, KindLBRACE, KindCOLON, KindCONSTRUCTOR_KEYWORD)
	kpDefinitelyOutOfReceiverSet           = orSet(
		newSet(KindEQ, KindCOLON, KindLBRACE, KindRBRACE, KindBY_KEYWORD),
		kpTopLevelDeclarationFirst)
	kpEOLOrSemicolonRbraceSet = newSet(kEOLOrSemicolon, KindRBRACE)
	kpClassInterfaceSet       = newSet(KindCLASS_KEYWORD, KindINTERFACE_KEYWORD)

	kpNoModifierBeforeForValueParameter = newSet(KindCOMMA, KindCOLON, KindEQ, KindRPAR)

	// kpExpressionFirst and kpExpressionFollow are KotlinParsing.EXPRESSION_FIRST
	// and EXPRESSION_FOLLOW (moved from KotlinExpressionParsing in 2.4).
	kpExpressionFirst  = epExpressionFirst
	kpExpressionFollow = epExpressionFollow

	kpUserTypeNameRecoverySet = orSet(kpExpressionFirst, kpExpressionFollow, kpDeclarationFirst)

	// kpEmpty is TokenSet.EMPTY.
	kpEmpty tokenSet
)

// kpRS turns a recovery set into the pointer form expectRS takes.
func kpRS(s tokenSet) *tokenSet { return &s }

// kpSetLeftBinder and kpSetRightBinder are setCustomEdgeTokenBinders
// with a null right or left argument (which keeps the current binder).
func kpSetLeftBinder(m Marker, left binderID) {
	m.SetCustomEdgeTokenBinders(left, m.b.markers[m.i].right)
}

func kpSetRightBinder(m Marker, right binderID) {
	m.SetCustomEdgeTokenBinders(m.b.markers[m.i].left, right)
}

func (p *kotlinParsing) initPatterns() {
	p.lastDotAfterReceiverLParPattern = newFirstBefore(
		p.atSetPred(kpReceiverTypeTerminators, kpReceiverTypeTerminators),
		func(topLevel bool) bool {
			if topLevel && p.atSet(kpDefinitelyOutOfReceiverSet) {
				return true
			}
			return topLevel && !p.at(KindQUEST) && !p.at(KindLPAR) && !p.at(KindRPAR)
		},
	)
	p.lastDotAfterReceiverNotLParPattern = newLastBefore(
		p.atSetPred(kpReceiverTypeTerminators, kpReceiverTypeTerminators),
		func(topLevel bool) bool {
			if topLevel && (p.atSet(kpDefinitelyOutOfReceiverSet) || p.at(KindLPAR)) {
				return true
			}
			if topLevel && p.at(KindIDENTIFIER) {
				la := p.lookahead(1)
				return la != KindLT && la != KindDOT && la != KindSAFE_ACCESS && la != KindQUEST
			}
			return false
		},
		false,
	)
}

/*
 * [start] kotlinFile
 *   : preamble toplevelObject* [eof]
 *   ;
 */
func (p *kotlinParsing) parseFile() {
	fileMarker := p.mark()

	p.parsePreamble()

	for !p.eof() {
		p.parseTopLevelDeclaration()
	}

	p.checkUnclosedBlockComment()
	fileMarker.Done(KindKT_FILE)
}

func (p *kotlinParsing) checkUnclosedBlockComment() {
	if kpBlockDocCommentSet.has(p.b.rawLookup(-1)) {
		startOffset := p.b.rawTokenTypeStart(-1)
		endOffset := p.b.rawTokenTypeStart(0)
		tokenChars := p.b.originalText()[startOffset:endOffset]
		if !(len(tokenChars) > 2 && tokenChars[len(tokenChars)-2:] == "*/") {
			marker := p.b.mark()
			marker.Error("Unclosed comment")
			// GREEDY_RIGHT_BINDER as the left binder is DEFAULT_LEFT_BINDER.
			kpSetLeftBinder(marker, defaultLeftBinder)
		}
	}
}

func (p *kotlinParsing) parseTypeCodeFragment() {
	marker := p.mark()
	p.parseTypeRef()

	p.checkForUnexpectedSymbols()

	marker.Done(KindTYPE_CODE_FRAGMENT)
}

func (p *kotlinParsing) parseExpressionCodeFragment() {
	marker := p.mark()
	p.expr.parseExpression()

	p.checkForUnexpectedSymbols()

	marker.Done(KindEXPRESSION_CODE_FRAGMENT)
}

func (p *kotlinParsing) parseBlockCodeFragment() {
	marker := p.mark()
	blockMarker := p.mark()

	if p.at(KindPACKAGE_KEYWORD) || p.at(KindIMPORT_KEYWORD) {
		err := p.mark()
		p.parsePreamble()
		err.Error("Package directive and imports are forbidden in code fragments")
	}

	p.expr.parseStatements()

	p.checkForUnexpectedSymbols()

	blockMarker.Done(KindBLOCK)
	marker.Done(KindBLOCK_CODE_FRAGMENT)
}

func (p *kotlinParsing) parseLambdaExpression() {
	p.expr.parseFunctionLiteralWith( /* preferBlock = */ false /* collapse = */, false)
}

func (p *kotlinParsing) parseBlockExpression() {
	p.parseBlockCollapse( /* collapse = */ false)
}

func (p *kotlinParsing) parseScript() {
	fileMarker := p.mark()

	p.parsePreamble()

	scriptMarker := p.mark()

	blockMarker := p.mark()

	p.expr.parseStatementsScript( /*isScriptTopLevel = */ true)

	p.checkForUnexpectedSymbols()

	blockMarker.Done(KindBLOCK)
	blockMarker.SetCustomEdgeTokenBinders(precedingAllBinder, trailingAllBinder)

	scriptMarker.Done(KindSCRIPT)
	scriptMarker.SetCustomEdgeTokenBinders(precedingAllBinder, trailingAllBinder)

	fileMarker.Done(KindKT_FILE)
}

func (p *kotlinParsing) checkForUnexpectedSymbols() {
	for !p.eof() {
		p.errorAndAdvance("Unexpected symbol")
	}
}

/*
 *preamble
 *  : fileAnnotationList? packageDirective? import*
 *  ;
 */
func (p *kotlinParsing) parsePreamble() {
	firstEntry := p.mark()

	/*
	 * fileAnnotationList
	 *   : fileAnnotations*
	 */
	p.parseFileAnnotationList(annFileAnnotationsBeforePackage)

	/*
	 * packageDirective
	 *   : modifiers "package" SimpleName{"."} SEMI?
	 *   ;
	 */
	packageDirective := p.mark()
	p.parseModifierList(kpEmpty)

	if p.at(KindPACKAGE_KEYWORD) {
		p.advance() // PACKAGE_KEYWORD

		p.parsePackageName()

		firstEntry.Drop()

		p.consumeIf(KindSEMICOLON)

		packageDirective.Done(KindPACKAGE_DIRECTIVE)
	} else {
		// When package directive is omitted we should not report error on non-file annotations at the beginning of the file.
		// So, we rollback the parsing position and reparse file annotation list without report error on non-file annotations.
		firstEntry.RollbackTo()

		p.parseFileAnnotationList(annFileAnnotationsWhenPackageOmitted)
		packageDirective = p.mark()
		packageDirective.Done(KindPACKAGE_DIRECTIVE)
		// Need to skip everything but shebang comment to allow comments at the start of the file to be bound to the first declaration.
		kpSetLeftBinder(packageDirective, bindFirstShebangWithWhitespaceOnly)
	}

	p.parseImportDirectives()
}

/* SimpleName{"."} */
func (p *kotlinParsing) parsePackageName() {
	qualifiedExpression := p.mark()
	simpleName := true
	for {
		if p.b.newlineBeforeCurrentToken() {
			p.errorWithRecovery("Package name must be a '.'-separated identifier list placed on a single line", &kpPackageNameRecoverySet)
			break
		}

		if p.at(KindDOT) {
			p.advance() // DOT
			qualifiedExpression.Error("Package name must be a '.'-separated identifier list")
			qualifiedExpression = p.mark()
			continue
		}

		nsName := p.mark()
		simpleNameFound := p.expectRS(KindIDENTIFIER, "Package name must be a '.'-separated identifier list", &kpPackageNameRecoverySet)
		if simpleNameFound {
			nsName.Done(KindREFERENCE_EXPRESSION)
		} else {
			nsName.Drop()
		}

		if !simpleName {
			precedingMarker := qualifiedExpression.Precede()
			qualifiedExpression.Done(KindDOT_QUALIFIED_EXPRESSION)
			qualifiedExpression = precedingMarker
		}

		if p.at(KindDOT) {
			p.advance() // DOT

			if simpleName && !simpleNameFound {
				qualifiedExpression.Drop()
				qualifiedExpression = p.mark()
			} else {
				simpleName = false
			}
		} else {
			break
		}
	}
	qualifiedExpression.Drop()
}

/*
 * import
 *   : "import" SimpleName{"."} ("." "*" | "as" SimpleName)? SEMI?
 *   ;
 */
func (p *kotlinParsing) parseImportDirective() {
	importDirective := p.mark()
	p.advance() // IMPORT_KEYWORD

	if p.closeImportWithErrorIfNewline(importDirective, nil, "Expecting qualified name") {
		return
	}

	if !p.at(KindIDENTIFIER) {
		err := p.mark()
		p.skipUntil(newSet(kEOLOrSemicolon))
		err.Error("Expecting qualified name")
		importDirective.Done(KindIMPORT_DIRECTIVE)
		p.consumeIf(KindSEMICOLON)
		return
	}

	qualifiedName := p.mark()
	reference := p.mark()
	p.advance() // IDENTIFIER
	reference.Done(KindREFERENCE_EXPRESSION)

	for p.at(KindDOT) && p.lookahead(1) != KindMUL {
		p.advance() // DOT

		if p.closeImportWithErrorIfNewline(importDirective, nil, "Import must be placed on a single line") {
			qualifiedName.Drop()
			return
		}

		reference = p.mark()
		if p.expectRS(KindIDENTIFIER, "Qualified name must be a '.'-separated identifier list", &kpImportRecoverySet) {
			reference.Done(KindREFERENCE_EXPRESSION)
		} else {
			reference.Drop()
		}

		precede := qualifiedName.Precede()
		qualifiedName.Done(KindDOT_QUALIFIED_EXPRESSION)
		qualifiedName = precede
	}
	qualifiedName.Drop()

	if p.at(KindDOT) {
		p.advance() // DOT
		p.advance() // MUL
		if p.at(KindAS_KEYWORD) {
			as := p.mark()
			p.advance() // AS_KEYWORD
			if p.closeImportWithErrorIfNewline(importDirective, nil, "Expecting identifier") {
				as.Drop()
				return
			}
			p.consumeIf(KindIDENTIFIER)
			as.Done(KindIMPORT_ALIAS)
			as.Precede().Error("Cannot rename all imported items to one identifier")
		}
	}
	if p.at(KindAS_KEYWORD) {
		alias := p.mark()
		p.advance() // AS_KEYWORD
		if p.closeImportWithErrorIfNewline(importDirective, &alias, "Expecting identifier") {
			return
		}
		p.expectRS(KindIDENTIFIER, "Expecting identifier", &kpSemicolonSet)
		alias.Done(KindIMPORT_ALIAS)
	}
	p.consumeIf(KindSEMICOLON)
	importDirective.Done(KindIMPORT_DIRECTIVE)
	kpSetRightBinder(importDirective, trailingCommentsBinder)
}

func (p *kotlinParsing) closeImportWithErrorIfNewline(importDirective Marker, importAlias *Marker, errorMessage string) bool {
	if p.b.newlineBeforeCurrentToken() {
		if importAlias != nil {
			importAlias.Done(KindIMPORT_ALIAS)
		}
		p.error(errorMessage)
		importDirective.Done(KindIMPORT_DIRECTIVE)
		return true
	}
	return false
}

func (p *kotlinParsing) parseImportDirectives() {
	importList := p.mark()
	if !p.at(KindIMPORT_KEYWORD) {
		// this is necessary to allow comments at the start of the file to be bound to the first declaration
		kpSetLeftBinder(importList, doNotBindAnything)
	}
	for p.at(KindIMPORT_KEYWORD) {
		p.parseImportDirective()
	}
	importList.Done(KindIMPORT_LIST)
}

/*
 * toplevelObject
 *   : package
 *   : class
 *   : extension
 *   : function
 *   : property
 *   : typeAlias
 *   : object
 *   ;
 */
func (p *kotlinParsing) parseTopLevelDeclaration() {
	if p.at(KindSEMICOLON) {
		p.advance() // SEMICOLON
		return
	}
	decl := p.mark()

	detector := &modifierDetector{}
	p.parseModifierListC(detector.consume, kpEmpty, false)

	declType := p.parseCommonDeclaration(detector, nameRequired, declMemberOrTopLevel)

	if declType == kNone && p.at(KindLBRACE) {
		p.error("Expecting a top level declaration")
		p.parseBlock()
		declType = KindFUN
	}

	if declType == kNone && p.at(KindIMPORT_KEYWORD) {
		p.error("imports are only allowed in the beginning of file")
		p.parseImportDirectives()
		decl.Drop()
	} else if declType == kNone {
		p.errorAndAdvance("Expecting a top level declaration")
		decl.Drop()
	} else {
		closeDeclarationWithCommentBinders(decl, declType, true)
	}
}

func (p *kotlinParsing) parseCommonDeclaration(detector *modifierDetector, nameParsingModeForObject nameParsingMode,
	declarationParsingMode declarationParsingMode) Kind {
	switch p.tt() {
	case KindCLASS_KEYWORD, KindINTERFACE_KEYWORD:
		return p.parseClass(detector.enumDetected, true)
	case KindFUN_KEYWORD:
		return p.parseFunction()
	case KindVAL_KEYWORD, KindVAR_KEYWORD:
		return p.parseProperty(declarationParsingMode)
	case KindLPAR, KindLBRACKET:
		if lookahead := p.lookahead(1); lookahead == KindVAL_KEYWORD || lookahead == KindVAR_KEYWORD {
			return p.parseProperty(declarationParsingMode)
		}
		return kNone
	case KindTYPE_ALIAS_KEYWORD:
		return p.parseTypeAlias()
	case KindOBJECT_KEYWORD:
		p.parseObject(nameParsingModeForObject, true)
		return KindOBJECT_DECLARATION
	case KindIDENTIFIER:
		if detector.enumDetected && declarationParsingMode.canBeEnumUsedAsSoftKeyword() {
			return p.parseClass(true, false)
		}
	}

	return kNone
}

/*
 * (modifier | annotation)*
 */
func (p *kotlinParsing) parseModifierList(noModifiersBefore tokenSet) bool {
	return p.parseModifierListC(nil, noModifiersBefore, false)
}

func (p *kotlinParsing) parseAnnotationsList(noModifiersBefore tokenSet) {
	p.doParseModifierList(nil, kpEmpty, annDefault, noModifiersBefore, false)
}

// parseModifierListC is parseModifierList(Consumer, TokenSet, boolean).
//
// (modifier | annotation)*
//
// Feeds modifiers (not annotations) into the passed consumer, if it is not nil.
// localDeclaration is true if we are trying to parse a local declaration.
func (p *kotlinParsing) parseModifierListC(tokenConsumer func(Kind), noModifiersBefore tokenSet, localDeclaration bool) bool {
	return p.doParseModifierList(tokenConsumer, modifierKeywords, annDefault, noModifiersBefore, localDeclaration)
}

func (p *kotlinParsing) parseFunctionTypeValueParameterModifierList() {
	p.doParseModifierList(nil, reservedValueParameterModifierKeywords, annNoAnnotationsNoContext, kpNoModifierBeforeForValueParameter, false)
}

func (p *kotlinParsing) parseTypeModifierList() {
	p.doParseModifierList(nil, typeModifierKeywords, annTypeContext, kpEmpty, false)
}

func (p *kotlinParsing) parseTypeArgumentModifierList() {
	p.doParseModifierList(nil, typeArgumentModifierKeywords, annNoAnnotationsNoContext, kpCommaColonGtSet, false)
}

func (p *kotlinParsing) doParseModifierListBody(tokenConsumer func(Kind), modifierKeywords tokenSet,
	annotationParsingMode annotationParsingMode, noModifiersBefore tokenSet, localDeclaration bool) bool {
	empty := true
	for !p.eof() {
		if p.at(KindAT) && annotationParsingMode.allowAnnotations() {
			beforeAnnotationMarker := p.mark()

			isAnnotationParsed := p.parseAnnotationOrList(annotationParsingMode)

			if !isAnnotationParsed && !annotationParsingMode.withSignificantWhitespaceBeforeArguments() {
				beforeAnnotationMarker.RollbackTo()
				// try parse again, but with significant whitespace
				newMode := annWithSignificantWhitespaceBeforeArgumentsNoContext
				if annotationParsingMode.allowContextList() {
					newMode = annWithSignificantWhitespaceBeforeArguments
				}
				p.doParseModifierListBody(tokenConsumer, modifierKeywords, newMode, noModifiersBefore, localDeclaration)
				empty = false
				break
			} else {
				beforeAnnotationMarker.Drop()
			}
		} else if p.at(KindCONTEXT_KEYWORD) && annotationParsingMode.allowContextList() && p.lookahead(1) == KindLPAR {
			contextMarker := p.mark()
			if !p.parseContextParameterOrReceiverList(false) && localDeclaration {
				// Rollback the entire context declaration to make it possible to prevent parsing of potential local declarations
				// that in fact are not declarations (we are trying to parse declarations at first and statements as second).
				contextMarker.RollbackTo()
				break
			} else {
				contextMarker.Drop()
			}
		} else if p.tryParseModifier(tokenConsumer, noModifiersBefore, modifierKeywords) {
			// modifier advanced
		} else {
			break
		}
		empty = false
	}

	return empty
}

func (p *kotlinParsing) doParseModifierList(tokenConsumer func(Kind), modifierKeywords tokenSet,
	annotationParsingMode annotationParsingMode, noModifiersBefore tokenSet, localDeclaration bool) bool {
	list := p.mark()

	empty := p.doParseModifierListBody(tokenConsumer, modifierKeywords, annotationParsingMode, noModifiersBefore, localDeclaration)

	if empty {
		list.Drop()
	} else {
		list.Done(KindMODIFIER_LIST)
	}
	return !empty
}

func (p *kotlinParsing) tryParseModifier(tokenConsumer func(Kind), noModifiersBefore, modifierKeywords tokenSet) bool {
	marker := p.mark()

	if p.atSet(modifierKeywords) {
		lookahead := p.lookahead(1)

		if p.at(KindFUN_KEYWORD) && lookahead != KindINTERFACE_KEYWORD {
			marker.RollbackTo()
			return false
		}

		if lookahead != kNone && !noModifiersBefore.has(lookahead) {
			tt := p.tt()
			if tokenConsumer != nil {
				tokenConsumer(tt)
			}
			p.advance() // MODIFIER
			marker.Collapse(tt)
			return true
		}
	}

	marker.RollbackTo()
	return false
}

/*
 * contextReceiverList
 *   : "context" "(" (contextReceiver{","})+ ")"
 *
 * Returns true if it parsed a context with value parameters. Otherwise
 * returns false if it parsed a context with type refs (that work as
 * receivers) or encountered a syntax error during parsing.
 */
func (p *kotlinParsing) parseContextParameterOrReceiverList(inFunctionType bool) bool {
	valueParameterOrTypeRefList := p.mark()
	p.advance() // CONTEXT_KEYWORD

	var noError bool

	if p.lookahead(1) == KindRPAR {
		p.advance() // LPAR
		p.error("Empty context parameter list")
		p.advance() // RPAR
		noError = false
	} else {
		// Treat parsing of context receivers (deprecated syntax) as an error,
		// But an outer caller decides if the entire list should be dropped:
		// If we're trying to parse a local declaration, we should drop it to prevent unexpected parsing of ahead declarations
		noError = p.valueParameterLoop(inFunctionType, kpContextParametersFollowSet, func() bool {
			return p.parseValueParameterOrTypeRef(inFunctionType)
		})
	}

	valueParameterOrTypeRefList.Done(KindCONTEXT_PARAMETER_LIST)
	return noError
}

/*
 * contextReceiver
 *   : label? typeReference
 *
 * Returns true if it parsed a value parameter or type ref in the correct
 * position (in function type) and false otherwise.
 */
func (p *kotlinParsing) parseValueParameterOrTypeRef(inFunctionType bool) bool {
	if p.tryParseValueParameter(true) {
		return true
	}

	contextReceiver := p.mark()
	if !inFunctionType && p.expr.isAtLabelDefinitionOrMissingIdentifier() {
		p.expr.parseLabelDefinition()
	}
	p.parseTypeRef()
	contextReceiver.Done(KindCONTEXT_RECEIVER)
	return inFunctionType
}

/*
 * fileAnnotationList
 *   : ("[" "file:" annotationEntry+ "]")*
 *   ;
 */
func (p *kotlinParsing) parseFileAnnotationList(mode annotationParsingMode) {
	fileAnnotationsList := p.mark()

	if p.parseAnnotations(mode) {
		fileAnnotationsList.Done(KindFILE_ANNOTATION_LIST)
	} else {
		fileAnnotationsList.Drop()
	}
}

/*
 * annotations
 *   : (annotation | annotationList)*
 *   ;
 */
func (p *kotlinParsing) parseAnnotations(mode annotationParsingMode) bool {
	if !p.parseAnnotationOrList(mode) {
		return false
	}

	for p.parseAnnotationOrList(mode) {
		// do nothing
	}

	return true
}

/*
 * annotation
 *   : "@" (annotationUseSiteTarget ":")? unescapedAnnotation
 *   ;
 *
 * annotationList
 *   : "@" (annotationUseSiteTarget ":")? "[" unescapedAnnotation+ "]"
 *   ;
 */
func (p *kotlinParsing) parseAnnotationOrList(mode annotationParsingMode) bool {
	if p.at(KindAT) {
		nextRawToken := p.b.rawLookup(1)
		tokenToMatch := nextRawToken
		isTargetedAnnotation := false

		if (nextRawToken == KindIDENTIFIER || kpAnnotationTargets.has(nextRawToken)) && p.lookahead(2) == KindCOLON {
			tokenToMatch = p.lookahead(3)
			isTargetedAnnotation = true
		} else if p.lookahead(1) == KindCOLON {
			// recovery for "@:ann"
			isTargetedAnnotation = true
			tokenToMatch = p.lookahead(2)
		}

		if tokenToMatch == KindIDENTIFIER {
			return p.parseAnnotation(mode)
		} else if tokenToMatch == KindLBRACKET {
			return p.parseAnnotationList(mode)
		} else {
			if isTargetedAnnotation {
				if p.lookahead(1) == KindCOLON {
					p.errorAndAdvanceN("Expected annotation identifier after ':'", 2) // AT, COLON
				} else {
					p.errorAndAdvanceN("Expected annotation identifier after ':'", 3) // AT, (ANNOTATION TARGET KEYWORD), COLON
				}
			} else {
				p.errorAndAdvanceN("Expected annotation identifier after '@'", 1) // AT
			}
		}
		return true
	}

	return false
}

func (p *kotlinParsing) parseAnnotationList(mode annotationParsingMode) bool {
	annotation := p.mark()

	p.b.disableNewlines()

	p.advance() // AT

	if !p.parseAnnotationTargetIfNeeded(mode) {
		annotation.RollbackTo()
		p.b.restoreNewlinesState()
		return false
	}

	p.advance() // LBRACKET

	if !p.at(KindIDENTIFIER) && !p.at(KindAT) {
		p.error("Expecting a list of annotations")
	} else {
		for p.at(KindIDENTIFIER) || p.at(KindAT) {
			if p.at(KindAT) {
				p.errorAndAdvance("No '@' needed in annotation list") // AT
				continue
			}

			p.parseAnnotation(annDefault)
			for p.at(KindCOMMA) {
				p.errorAndAdvance("No commas needed to separate annotations")
			}
		}
	}

	p.expectMsg(KindRBRACKET, "Expecting ']' to close the annotation list")
	p.b.restoreNewlinesState()

	annotation.Done(KindANNOTATION)
	return true
}

// Returns true if we should continue parse annotation
func (p *kotlinParsing) parseAnnotationTargetIfNeeded(mode annotationParsingMode) bool {
	expectedAnnotationTargetBeforeColon := "Expected annotation target before ':'"

	if p.at(KindCOLON) {
		// recovery for "@:ann"
		p.errorAndAdvance(expectedAnnotationTargetBeforeColon) // COLON
		return true
	}

	targetKeyword := kNone
	if p.atSet(kpAnnotationTargets) {
		targetKeyword = p.tt()
	}
	if mode == annFileAnnotationsWhenPackageOmitted && !(targetKeyword == KindFILE_KEYWORD && p.lookahead(1) == KindCOLON) {
		return false
	}

	if p.lookahead(1) == KindCOLON && targetKeyword == kNone && p.at(KindIDENTIFIER) {
		// recovery for "@fil:ann"
		p.errorAndAdvance(expectedAnnotationTargetBeforeColon) // IDENTIFIER
		p.advance()                                            // COLON
		return true
	}

	if targetKeyword == kNone && mode.isFileAnnotationParsingMode() {
		p.parseAnnotationTarget(KindFILE_KEYWORD)
	} else if targetKeyword != kNone {
		p.parseAnnotationTarget(targetKeyword)
	}

	return true
}

func (p *kotlinParsing) parseAnnotationTarget(keyword Kind) {
	marker := p.mark()

	if !p.expect(keyword) {
		p.error(generateAnnotationTargetErrorMessage(keyword))
		marker.Drop()
	} else {
		marker.Done(KindANNOTATION_TARGET)
	}

	if !p.expect(KindCOLON) {
		p.errorWithRecovery(generateAnnotationTargetErrorMessage(keyword), &kpIdentifierRbracketLbracketSet)
	}
}

func generateAnnotationTargetErrorMessage(keyword Kind) string {
	return "Expecting \"" + keywordValue[keyword] + ":" + "\" prefix for " + keywordValue[keyword] + " annotations"
}

/*
 * annotation
 *   : "@" (annotationUseSiteTarget ":")? unescapedAnnotation
 *   ;
 *
 * unescapedAnnotation
 *   : SimpleName{"."} typeArguments? valueArguments?
 *   ;
 */
func (p *kotlinParsing) parseAnnotation(mode annotationParsingMode) bool {
	annotation := p.mark()

	atAt := p.at(KindAT)
	if atAt {
		p.advance() // AT
	}

	if atAt && !p.parseAnnotationTargetIfNeeded(mode) {
		annotation.RollbackTo()
		return false
	}

	reference := p.mark()
	typeReference := p.mark()
	p.parseUserType()
	typeReference.Done(KindTYPE_REFERENCE)
	reference.Done(KindCONSTRUCTOR_CALLEE)

	p.parseTypeArgumentList()

	if p.at(KindLPAR) &&
		!valVar.has(p.lookahead(1)) &&
		!(whiteSpaceOrCommentSet.has(p.b.rawLookup(-1)) && mode.withSignificantWhitespaceBeforeArguments()) {
		p.expr.parseValueArgumentList()

		/*
		 * There are two problem cases relating to parsing of annotations on a functional type:
		 *  - Annotation on a functional type was parsed correctly with the capture parentheses of the functional type,
		 *      e.g. @Anno () -> Unit
		 *                    ^ Parse error only here: Type expected
		 *  - It wasn't parsed, e.g. @Anno (x: kotlin.Any) -> Unit
		 *                                           ^ Parse error: Expecting ')'
		 *
		 * In both cases, parser should rollback to start parsing of annotation and tries parse it with significant whitespace.
		 * A marker is set here which means that we must to rollback.
		 */
		if mode.typeContext() && (p.getLastToken() != KindRPAR || p.at(KindARROW)) {
			annotation.Done(KindANNOTATION_ENTRY)
			return false
		}
	}
	annotation.Done(KindANNOTATION_ENTRY)

	return true
}

func (p *kotlinParsing) isNextRawTokenCommentOrWhitespace() bool {
	return whiteSpaceOrCommentSet.has(p.b.rawLookup(1))
}

/*
 * class
 *   : modifiers ("class" | "interface") SimpleName
 *       typeParameters?
 *       primaryConstructor?
 *       (":" annotations delegationSpecifier{","})?
 *       typeConstraints
 *       (classBody? | enumClassBody)
 *   ;
 *
 * primaryConstructor
 *   : (modifiers "constructor")? ("(" functionParameter{","} ")")
 *   ;
 *
 * object
 *   : "object" SimpleName? primaryConstructor? ":" delegationSpecifier{","}? classBody?
 *   ;
 */
func (p *kotlinParsing) parseClassOrObject(object bool, nameMode nameParsingMode, optionalBody, enumClass, expectKindKeyword bool) Kind {
	if expectKindKeyword {
		p.advance() // CLASS_KEYWORD, INTERFACE_KEYWORD or OBJECT_KEYWORD
	} else {
		p.error("'class' keyword is expected after 'enum'")
	}

	if nameMode == nameRequired {
		p.expectRS(KindIDENTIFIER, "Name expected", &kpClassNameRecoverySet)
	} else {
		if p.at(KindIDENTIFIER) {
			if nameMode == nameProhibited {
				p.errorAndAdvance("An object expression cannot bind a name")
			} else {
				p.advance()
			}
		}
	}

	typeParametersDeclared := p.parseTypeParameterList(kpTypeParameterGtRecoverySet)

	beforeConstructorModifiers := p.mark()
	primaryConstructorMarker := p.mark()
	hasConstructorModifiers := p.parseModifierList(kpEmpty)

	// Some modifiers found, but no parentheses following: class has already ended, and we are looking at something else
	if hasConstructorModifiers && !p.atSet(kpLparLbraceColonConstructorKeywordSet) {
		beforeConstructorModifiers.RollbackTo()
		if object {
			return KindOBJECT_DECLARATION
		}
		return KindCLASS
	}

	// We are still inside a class declaration
	beforeConstructorModifiers.Drop()

	hasConstructorKeyword := p.at(KindCONSTRUCTOR_KEYWORD)
	if hasConstructorKeyword {
		p.advance() // CONSTRUCTOR_KEYWORD
	}

	if p.at(KindLPAR) {
		p.parseValueParameterList(false /* typeRequired  = */, true, kpLbraceRbraceSet)
		primaryConstructorMarker.Done(KindPRIMARY_CONSTRUCTOR)
	} else if hasConstructorModifiers || hasConstructorKeyword {
		// A comprehensive error message for cases like:
		//    class A private : Foo
		// or
		//    class A private {
		primaryConstructorMarker.Done(KindPRIMARY_CONSTRUCTOR)
		if hasConstructorKeyword {
			p.error("Expecting primary constructor parameter list")
		} else {
			p.error("Expecting 'constructor' keyword")
		}
	} else {
		primaryConstructorMarker.Drop()
	}

	if p.at(KindCOLON) {
		p.advance() // COLON
		p.parseDelegationSpecifierList()
	}

	whereMarker := p.newOptionalMarker(object)
	p.parseTypeConstraintsGuarded(typeParametersDeclared)
	whereMarker.error("Where clause is not allowed for objects")

	if p.at(KindLBRACE) {
		if enumClass {
			p.parseEnumClassBody()
		} else {
			p.parseClassBody()
		}
	} else if !optionalBody {
		fakeBody := p.mark()
		p.error("Expecting a class body")
		fakeBody.Done(KindCLASS_BODY)
	}

	if object {
		return KindOBJECT_DECLARATION
	}
	return KindCLASS
}

func (p *kotlinParsing) parseClass(enumClass, expectKindKeyword bool) Kind {
	return p.parseClassOrObject(false, nameRequired, true, enumClass, expectKindKeyword)
}

func (p *kotlinParsing) parseObject(nameMode nameParsingMode, optionalBody bool) {
	p.parseClassOrObject(true, nameMode, optionalBody, false, true)
}

/*
 * enumClassBody
 *   : "{" enumEntries (";" members)? "}"
 *   ;
 */
func (p *kotlinParsing) parseEnumClassBody() {
	if !p.at(KindLBRACE) {
		return
	}

	body := p.mark()
	p.b.enableNewlines()

	p.advance() // LBRACE

	if !p.parseEnumEntries() && !p.at(KindRBRACE) {
		p.error("Expecting ';' after the last enum entry or '}' to close enum class body")
	}
	p.parseMembers()
	p.expectMsg(KindRBRACE, "Expecting '}' to close enum class body")

	p.b.restoreNewlinesState()
	body.Done(KindCLASS_BODY)
}

// parseEnumEntries returns true if enum regular members can follow, false otherwise.
//
// enumEntries
//
//	: enumEntry{","}?
//	;
func (p *kotlinParsing) parseEnumEntries() bool {
	for !p.eof() && !p.at(KindRBRACE) {
		switch p.parseEnumEntry() {
		case kpEnumEntryFailed:
			// Special case without any enum entries but with possible members after semicolon
			if p.at(KindSEMICOLON) {
				p.advance()
				return true
			}
			return false
		case kpEnumEntryNoDelimiter:
			return false
		case kpEnumEntryCommaDelimiter:
		case kpEnumEntrySemicolonDelimiter:
			return true
		}
	}
	return false
}

// kpParseEnumEntryResult is ParseEnumEntryResult.
type kpParseEnumEntryResult uint8

const (
	kpEnumEntryFailed kpParseEnumEntryResult = iota
	kpEnumEntryNoDelimiter
	kpEnumEntryCommaDelimiter
	kpEnumEntrySemicolonDelimiter
)

/*
 * enumEntry
 *   : modifiers SimpleName ("(" arguments ")")? classBody?
 *   ;
 */
func (p *kotlinParsing) parseEnumEntry() kpParseEnumEntryResult {
	entry := p.mark()

	p.parseModifierList(kpCommaSemicolonRbraceSet)

	if !p.atSet(kpSoftKeywordsAtMemberStart) && p.at(KindIDENTIFIER) {
		p.advance() // IDENTIFIER

		if p.at(KindLPAR) {
			// Arguments should be parsed here
			// Also, "fake" constructor call tree is created,
			// with empty type name inside
			initializerList := p.mark()
			delegatorSuperCall := p.mark()

			callee := p.mark()
			typeReference := p.mark()
			typ := p.mark()
			referenceExpr := p.mark()
			referenceExpr.Done(KindENUM_ENTRY_SUPERCLASS_REFERENCE_EXPRESSION)
			typ.Done(KindUSER_TYPE)
			typeReference.Done(KindTYPE_REFERENCE)
			callee.Done(KindCONSTRUCTOR_CALLEE)

			p.expr.parseValueArgumentList()
			delegatorSuperCall.Done(KindSUPER_TYPE_CALL_ENTRY)
			initializerList.Done(KindINITIALIZER_LIST)
		}
		if p.at(KindLBRACE) {
			p.parseClassBody()
		}
		commaFound := p.at(KindCOMMA)
		if commaFound {
			p.advance()
		}
		semicolonFound := p.at(KindSEMICOLON)
		if semicolonFound {
			p.advance()
		}

		// Probably some helper function
		closeDeclarationWithCommentBinders(entry, KindENUM_ENTRY, true)
		if semicolonFound {
			return kpEnumEntrySemicolonDelimiter
		}
		if commaFound {
			return kpEnumEntryCommaDelimiter
		}
		return kpEnumEntryNoDelimiter
	}
	entry.RollbackTo()
	return kpEnumEntryFailed
}

/*
 * classBody
 *   : ("{" members "}")?
 *   ;
 */
func (p *kotlinParsing) parseClassBody() {
	body := p.mark()

	p.b.enableNewlines()

	if p.expectMsg(KindLBRACE, "Expecting a class body") {
		p.parseMembers()
		p.expectMsg(KindRBRACE, "Missing '}")
	}

	p.b.restoreNewlinesState()

	body.Done(KindCLASS_BODY)
}

/*
 * members
 *   : memberDeclaration*
 *   ;
 */
func (p *kotlinParsing) parseMembers() {
	for !p.eof() && !p.at(KindRBRACE) {
		p.parseMemberDeclaration()
	}
}

/*
 * memberDeclaration
 *   : modifiers memberDeclaration'
 *   ;
 */
func (p *kotlinParsing) parseMemberDeclaration() {
	if p.at(KindSEMICOLON) {
		p.advance() // SEMICOLON
		return
	}
	decl := p.mark()

	isCompanionBlock := p.at(KindCOMPANION_KEYWORD) && p.lookahead(1) == KindLBRACE

	detector := &modifierDetector{}
	p.parseModifierListC(detector.consume, kpEmpty, false)

	var declType Kind
	if isCompanionBlock {
		declType = p.parseCompanionBlock()
	} else {
		declType = p.parseMemberDeclarationRest(detector)
	}

	if declType == kNone {
		p.errorWithRecovery("Expecting member declaration", &kpEmpty)
		decl.Drop()
	} else {
		closeDeclarationWithCommentBinders(decl, declType, true)
	}
}

func (p *kotlinParsing) parseMemberDeclarationRest(modifierDetector *modifierDetector) Kind {
	nameMode := nameRequired
	if modifierDetector.companionDetected {
		nameMode = nameAllowed
	}
	declType := p.parseCommonDeclaration(modifierDetector, nameMode, declMemberOrTopLevel)

	if declType != kNone {
		return declType
	}

	if p.at(KindINIT_KEYWORD) {
		p.advance() // init
		if p.at(KindLBRACE) {
			p.parseBlock()
		} else {
			p.mark().Error("Expecting '{' after 'init'")
		}
		declType = KindCLASS_INITIALIZER
	} else if p.at(KindCONSTRUCTOR_KEYWORD) {
		p.parseSecondaryConstructor()
		declType = KindSECONDARY_CONSTRUCTOR
	} else if p.at(KindLBRACE) {
		p.error("Expecting member declaration")
		p.parseBlock()
		declType = KindFUN
	}
	return declType
}

func (p *kotlinParsing) parseCompanionBlock() Kind {
	p.parseClassBody()
	return KindCOMPANION_BLOCK
}

/*
 * secondaryConstructor
 *   : modifiers "constructor" valueParameters (":" constructorDelegationCall)? block
 * constructorDelegationCall
 *   : "this" valueArguments
 *   : "super" valueArguments
 */
func (p *kotlinParsing) parseSecondaryConstructor() {
	p.advance() // CONSTRUCTOR_KEYWORD

	if p.at(KindLPAR) {
		p.parseValueParameterList(false /*typeRequired = */, true, kpValueArgsRecoverySet)
	} else {
		p.errorWithRecovery("Expecting '('", kpRS(orSet(kpValueArgsRecoverySet, newSet(KindCOLON))))
	}

	if p.at(KindCOLON) {
		p.advance() // COLON

		delegationCall := p.mark()

		if p.at(KindTHIS_KEYWORD) || p.at(KindSUPER_KEYWORD) {
			p.parseThisOrSuper()
			p.expr.parseValueArgumentList()
		} else {
			p.error("Expecting a 'this' or 'super' constructor call")
			var beforeWrongDelegationCallee Marker
			hasBefore := false
			if !p.at(KindLPAR) {
				beforeWrongDelegationCallee = p.mark()
				hasBefore = true
				p.advance() // wrong delegation callee
			}
			p.expr.parseValueArgumentList()

			if hasBefore {
				if p.at(KindLBRACE) {
					beforeWrongDelegationCallee.Drop()
				} else {
					beforeWrongDelegationCallee.RollbackTo()
				}
			}
		}

		delegationCall.Done(KindCONSTRUCTOR_DELEGATION_CALL)
	} else {
		// empty constructor delegation call
		emptyDelegationCall := p.mark()
		p.mark().Done(KindCONSTRUCTOR_DELEGATION_REFERENCE)
		emptyDelegationCall.Done(KindCONSTRUCTOR_DELEGATION_CALL)
	}

	if p.at(KindLBRACE) {
		p.parseBlock()
	}
}

func (p *kotlinParsing) parseThisOrSuper() {
	mark := p.mark()

	p.advance() // THIS_KEYWORD | SUPER_KEYWORD

	mark.Done(KindCONSTRUCTOR_DELEGATION_REFERENCE)
}

/*
 * typeAlias
 *   : modifiers "typealias" SimpleName typeParameters? "=" type
 *   ;
 */
func (p *kotlinParsing) parseTypeAlias() Kind {
	p.advance() // TYPE_ALIAS_KEYWORD

	p.expectRS(KindIDENTIFIER, "Type name expected", &kpLtEqSemicolonTopLevelDeclarationFirstSet)

	p.parseTypeParameterList(kpTypeParameterGtRecoverySet)

	if p.at(KindWHERE_KEYWORD) {
		err := p.mark()
		p.parseTypeConstraints()
		err.Error("Type alias parameters can't have bounds")
	}

	p.expectRS(KindEQ, "Expecting '='", &kpTopLevelDeclarationFirstSemicolonSet)

	p.parseTypeRef()

	p.consumeIf(KindSEMICOLON)

	return KindTYPEALIAS
}

/*
 * property
 *   : modifiers ("val" | "var")
 *       typeParameters?
 *       (type ".")?
 *       ("(" variableDeclarationEntry{","} ")" | variableDeclarationEntry)
 *       typeConstraints
 *       ("by" | "=" expression SEMI?)?
 *       (getter? setter? | setter? getter?) SEMI?
 *   ;
 */
func (p *kotlinParsing) parseProperty(mode declarationParsingMode) Kind {
	isShortForm := p.at(KindVAL_KEYWORD) || p.at(KindVAR_KEYWORD)
	if isShortForm {
		p.advance()
	}

	typeParametersDeclared := p.at(KindLT) && p.parseTypeParameterList(kpIdentifierEqColonSemicolonSet)

	p.b.disableJoiningComplexTokens()

	receiver := p.mark()
	receiverTypeDeclared := p.parseReceiverType("property", kpPropertyNameFollowSet)

	multiDeclaration := p.at(KindLPAR) || p.at(KindLBRACKET)

	errorIf(receiver, multiDeclaration && receiverTypeDeclared, "Receiver type is not allowed on a destructuring declaration")

	isNameOnTheNextLine := p.eol()
	beforeName := p.mark()

	if multiDeclaration {
		multiDecl := p.mark()
		follow, mdMode := kpDestructuringPropertyNameFollowSet, multiDeclFull
		if isShortForm {
			follow, mdMode = kpPropertyNameFollowSet, multiDeclShort
		}
		p.parseMultiDeclarationEntry(follow, kpPropertyNameFollowMultiDeclarationRecoverySet, mdMode)
		errorIf(multiDecl, !mode.destructuringAllowed(), "Destructuring declarations are only allowed for local variables/values")
	} else {
		p.parseFunctionOrPropertyName(receiverTypeDeclared, "property", kpPropertyNameFollowSet,
			kpPropertyNameFollowFunctionOrPropertyRecoverySet /*nameRequired = */, true)
	}

	p.b.restoreJoiningComplexTokensState()

	noTypeReference := true
	if p.at(KindCOLON) {
		noTypeReference = false
		typ := p.mark()
		p.advance() // COLON
		p.parseTypeRef()
		errorIf(typ, multiDeclaration, "Type annotations are not allowed on destructuring declarations")
	}

	p.parseTypeConstraintsGuarded(typeParametersDeclared)

	if !p.parsePropertyDelegateOrAssignment() && isNameOnTheNextLine && noTypeReference && !receiverTypeDeclared {
		// Do not parse property identifier on the next line if declaration is invalid
		// In most cases this identifier relates to next statement/declaration
		if !multiDeclaration || isShortForm {
			beforeName.RollbackTo()
			p.error("Expecting property name or receiver type")
		} else {
			beforeName.Drop()
		}

		if multiDeclaration {
			return KindDESTRUCTURING_DECLARATION
		}
		return KindPROPERTY
	}

	beforeName.Drop()

	if mode.accessorsAllowed() {
		// It's only needed for non-local properties, because in local ones:
		// "val a = 1; b" must not be an infix call of b on "val ...;"

		p.b.enableNewlines()
		hasNewLineWithSemicolon := p.consumeIf(KindSEMICOLON) && p.b.newlineBeforeCurrentToken()
		p.b.restoreNewlinesState()

		if !hasNewLineWithSemicolon {
			var alreadyRead kpPropertyComponentCollector
			propertyComponentKind := p.parsePropertyComponent(&alreadyRead)

			for propertyComponentKind != kpComponentNone {
				alreadyRead[propertyComponentKind] = true
				propertyComponentKind = p.parsePropertyComponent(&alreadyRead)
			}

			if !p.atSet(kpEOLOrSemicolonRbraceSet) {
				if p.getLastToken() != KindSEMICOLON {
					p.errorUntil(
						"Property getter or setter expected",
						orSet(kpDeclarationFirst, newSet(kEOLOrSemicolon, KindLBRACE, KindRBRACE)))
				}
			} else {
				p.consumeIf(KindSEMICOLON)
			}
		}
	}

	if multiDeclaration {
		return KindDESTRUCTURING_DECLARATION
	}
	return KindPROPERTY
}

func (p *kotlinParsing) parsePropertyDelegateOrAssignment() bool {
	if p.at(KindBY_KEYWORD) {
		p.parsePropertyDelegate()
		return true
	} else if p.at(KindEQ) {
		p.advance() // EQ
		p.expr.parseExpression()
		return true
	}

	return false
}

/*
 * propertyDelegate
 *   : "by" expression
 *   ;
 */
func (p *kotlinParsing) parsePropertyDelegate() {
	delegate := p.mark()
	p.advance() // BY_KEYWORD
	p.expr.parseExpression()
	delegate.Done(KindPROPERTY_DELEGATE)
}

/*
 * (SimpleName (":" type){","})
 */
func (p *kotlinParsing) parseMultiDeclarationEntry(follow, recoverySet tokenSet, mode multiDeclarationMode) {
	// Parsing multi-name, e.g.
	//   val (a, b) = foo()
	//   (val a: X = aa, var b) = foo()
	//   val [a, b] = foo()
	//   [val a: X, var b] = foo()
	p.b.disableNewlines()

	isParentheses := p.at(KindLPAR)
	closingBrace := KindRBRACKET
	if isParentheses {
		closingBrace = KindRPAR
	}

	p.advance() // LPAR | LBRACKET

	if !p.atSet(follow) {
		for {
			if p.at(KindCOMMA) {
				p.errorAndAdvance("Expecting a name")
			} else if p.at(closingBrace) { // For declaration similar to `val () = somethingCall()`
				p.error("Expecting a name")
				break
			}
			property := p.mark()

			switch mode {
			case multiDeclFull:
				if p.at(KindVAL_KEYWORD) || p.at(KindVAR_KEYWORD) {
					p.advance()
				} else {
					p.errorWithRecovery("Expecting val or var keyword", &recoverySet)
				}
			case multiDeclFullValOnly:
				if p.at(KindVAL_KEYWORD) {
					p.advance()
				} else {
					p.errorWithRecovery("Expecting val keyword", &recoverySet)
				}
			default:
				p.parseModifierList(kpCommaRparRbracketColonEqSet)
			}

			p.expectRS(KindIDENTIFIER, "Expecting a name", &recoverySet)

			if p.at(KindCOLON) {
				p.advance() // COLON
				p.parseTypeRefRS(follow)
			}

			// Renaming is only allowed in name-based destructuring
			if p.at(KindEQ) && closingBrace == KindRPAR {
				p.advance()
				p.expr.parseSimpleNameExpression()
			}

			property.Done(KindDESTRUCTURING_DECLARATION_ENTRY)

			if !p.at(KindCOMMA) {
				break
			}
			p.advance() // COMMA
			if p.at(closingBrace) {
				break
			}
		}
	}

	msg := "Expecting ']'"
	if isParentheses {
		msg = "Expecting ')'"
	}
	p.expectRS(closingBrace, msg, &follow)
	p.b.restoreNewlinesState()
}

// kpPropertyComponentKind is PropertyComponentKind; kpComponentNone is null.
type kpPropertyComponentKind int8

const (
	kpComponentNone kpPropertyComponentKind = iota - 1
	kpComponentGet
	kpComponentSet
	kpComponentField
)

// kpPropertyComponentCollector is PropertyComponentKind.Collector.
type kpPropertyComponentCollector [3]bool

/*
 * propertyComponent
 *   : modifiers ("get" | "set")
 *   :
 *        (     "get" "(" ")"
 *           |
 *              "set" "(" modifiers parameter ")"
 *           |
 *              "field"
 *        ) functionBody
 *   ;
 */
func (p *kotlinParsing) parsePropertyComponent(notAllowedKind *kpPropertyComponentCollector) kpPropertyComponentKind {
	propertyComponent := p.mark()

	p.parseModifierList(kpEmpty)

	var propertyComponentKind kpPropertyComponentKind
	if p.at(KindGET_KEYWORD) {
		propertyComponentKind = kpComponentGet
	} else if p.at(KindSET_KEYWORD) {
		propertyComponentKind = kpComponentSet
	} else if p.at(KindFIELD_KEYWORD) {
		propertyComponentKind = kpComponentField
	} else {
		propertyComponent.RollbackTo()
		return kpComponentNone
	}

	if notAllowedKind[propertyComponentKind] {
		propertyComponent.RollbackTo()
		return kpComponentNone
	}

	p.advance() // GET_KEYWORD, SET_KEYWORD or FIELD_KEYWORD

	if !p.at(KindLPAR) && propertyComponentKind != kpComponentField {
		// Account for Jet-114 (val a : int get {...})
		if !p.atSet(kpAccessorFirstOrPropertyEnd) {
			p.errorUntil("Accessor body expected", orSet(kpAccessorFirstOrPropertyEnd, newSet(KindLBRACE, KindLPAR, KindEQ)))
		} else {
			closeDeclarationWithCommentBinders(propertyComponent, KindPROPERTY_ACCESSOR, true)
			return propertyComponentKind
		}
	}

	p.b.disableNewlines()

	if propertyComponentKind != kpComponentField {
		parameterList := p.mark()
		p.expectRS(KindLPAR, "Expecting '('", &kpRparIdentifierColonLbraceEqSet)
		if propertyComponentKind == kpComponentSet {
			setterParameter := p.mark()
			p.parseModifierList(kpCommaColonRparSet)
			p.expectRS(KindIDENTIFIER, "Expecting parameter name", &kpRparColonLbraceEqSet)

			if p.at(KindCOLON) {
				p.advance() // COLON
				p.parseTypeRef()
			}
			setterParameter.Done(KindVALUE_PARAMETER)
			if p.at(KindCOMMA) {
				p.advance() // COMMA
			}
		}
		if !p.at(KindRPAR) {
			p.errorUntil("Expecting ')'", newSet(KindRPAR, KindCOLON, KindLBRACE, KindRBRACE, KindEQ, kEOLOrSemicolon))
		}
		if p.at(KindRPAR) {
			p.advance()
		}
		parameterList.Done(KindVALUE_PARAMETER_LIST)
	}
	p.b.restoreNewlinesState()

	if p.at(KindCOLON) {
		p.advance()

		p.parseTypeRef()
	}

	if propertyComponentKind != kpComponentField {
		p.parseFunctionContract()
		p.parseFunctionBody()
	} else if p.at(KindEQ) {
		p.advance()
		p.expr.parseExpression()
		p.consumeIf(KindSEMICOLON)
	}

	if propertyComponentKind == kpComponentField {
		closeDeclarationWithCommentBinders(propertyComponent, KindBACKING_FIELD, true)
	} else {
		closeDeclarationWithCommentBinders(propertyComponent, KindPROPERTY_ACCESSOR, true)
	}

	return propertyComponentKind
}

func (p *kotlinParsing) parseFunction() Kind {
	return p.parseFunctionFail(false)
}

// parseFunctionFail is parseFunction(boolean failIfIdentifierExists);
// it returns kNone (Java null) only when failIfIdentifierExists is set.
//
// function
//
//	: modifiers "fun" typeParameters?
//	    (type ".")?
//	    SimpleName
//	    typeParameters? functionParameters (":" type)?
//	    typeConstraints
//	    functionBody?
//	;
func (p *kotlinParsing) parseFunctionFail(failIfIdentifierExists bool) Kind {
	p.advance() // FUN_KEYWORD

	// Recovery for the case of class A { fun| }
	if p.at(KindRBRACE) {
		p.error("Function body expected")
		return KindFUN
	}

	typeParameterListOccurred := false
	if p.at(KindLT) {
		p.parseTypeParameterList(kpLbracketLbraceRbraceLparSet)
		typeParameterListOccurred = true
	}

	p.b.disableJoiningComplexTokens()

	receiverFound := p.parseReceiverType("function", kpFunctionNameFollowSet)

	if p.at(KindIDENTIFIER) && failIfIdentifierExists {
		p.b.restoreJoiningComplexTokensState()
		return kNone
	}

	// function as expression has no name
	p.parseFunctionOrPropertyName(receiverFound, "function", kpFunctionNameFollowSet, kpFunctionNameRecoverySet /*nameRequired = */, false)

	p.b.restoreJoiningComplexTokensState()

	if p.at(KindLT) {
		err := p.mark()
		p.parseTypeParameterList(kpLparValueParametersFollowSet)
		if typeParameterListOccurred {
			finishIndex := p.b.rawTokenIndex()
			err.RollbackTo()
			err = p.mark()
			p.advanceN(finishIndex - p.b.rawTokenIndex())
			err.Error("Only one type parameter list is allowed for a function")
		} else {
			err.Drop()
		}
		typeParameterListOccurred = true
	}

	if p.at(KindLPAR) {
		p.parseValueParameterList(false /* typeRequired  = */, false, kpValueParametersFollowSet)
	} else {
		p.error("Expecting '('")
	}

	if p.at(KindCOLON) {
		p.advance() // COLON

		p.parseTypeRef()
	}

	functionContractOccurred := p.parseFunctionContract()

	p.parseTypeConstraintsGuarded(typeParameterListOccurred)

	if !functionContractOccurred {
		p.parseFunctionContract()
	}

	if p.at(KindSEMICOLON) {
		p.advance() // SEMICOLON
	} else if p.at(KindEQ) || p.at(KindLBRACE) {
		p.parseFunctionBody()
	}

	return KindFUN
}

/*
 *   (type "." | annotations)?
 */
func (p *kotlinParsing) parseReceiverType(title string, nameFollow tokenSet) bool {
	annotations := p.mark()
	annotationsPresent := p.parseAnnotations(annDefault)
	lastDot := p.lastDotAfterReceiver()
	receiverPresent := lastDot != -1
	if annotationsPresent {
		if receiverPresent {
			annotations.RollbackTo()
		} else {
			annotations.Error("Annotations are not allowed in this position")
		}
	} else {
		annotations.Drop()
	}

	if !receiverPresent {
		return false
	}

	p.createTruncatedBuilder(lastDot).parseTypeRefWithoutIntersections()

	if p.atSet(kpReceiverTypeTerminators) {
		p.advance() // expectation
	} else {
		p.errorWithRecovery("Expecting '.' before a "+title+" name", &nameFollow)
	}
	return true
}

func (p *kotlinParsing) lastDotAfterReceiver() int {
	var pattern tokenStreamPattern
	if p.at(KindLPAR) {
		pattern = p.lastDotAfterReceiverLParPattern
	} else {
		pattern = p.lastDotAfterReceiverNotLParPattern
	}
	pattern.reset()
	return p.matchTokenStreamPredicate(pattern)
}

/*
 * IDENTIFIER
 */
func (p *kotlinParsing) parseFunctionOrPropertyName(receiverFound bool, title string, nameFollow, recoverySet tokenSet, nameRequired bool) {
	if !nameRequired && p.atSet(nameFollow) {
		return // no name
	}

	if p.expect(KindIDENTIFIER) {
		return
	}

	msg := "Expecting " + title + " name"
	if !receiverFound {
		msg += " or receiver type"
	}
	p.errorWithRecovery(msg, &recoverySet)
}

/*
 * functionBody
 *   : block
 *   : "=" element
 *   ;
 */
func (p *kotlinParsing) parseFunctionBody() {
	if p.at(KindLBRACE) {
		p.parseBlock()
	} else if p.at(KindEQ) {
		p.advance() // EQ
		p.expr.parseExpression()
		p.consumeIf(KindSEMICOLON)
	} else {
		p.error("Expecting function body")
	}
}

/*
 * block
 *   : "{" (expressions)* "}"
 *   ;
 */
func (p *kotlinParsing) parseBlock() {
	p.parseBlockCollapse( /*collapse*/ true)
}

// parseBlockCollapse is parseBlock(boolean collapse).
func (p *kotlinParsing) parseBlockCollapse(collapse bool) {
	lazyBlock := p.mark()

	p.b.enableNewlines()

	hasOpeningBrace := p.expectMsg(KindLBRACE, "Expecting '{' to open a block")
	canCollapse := collapse && hasOpeningBrace && p.isLazy

	if canCollapse {
		p.advanceBalancedBlock()
	} else {
		p.expr.parseStatements()
		p.expectMsg(KindRBRACE, "Expecting '}'")
	}

	p.b.restoreNewlinesState()

	if canCollapse {
		lazyBlock.Collapse(KindBLOCK)
	} else {
		lazyBlock.Done(KindBLOCK)
	}
}

func (p *kotlinParsing) advanceBalancedBlock() {
	braceCount := 1
	for !p.eof() {
		if p._at(KindLBRACE) {
			braceCount++
		} else if p._at(KindRBRACE) {
			braceCount--
		}

		p.advance()

		if braceCount == 0 {
			break
		}
	}
}

/*
 * delegationSpecifier{","}
 */
func (p *kotlinParsing) parseDelegationSpecifierList() {
	list := p.mark()

	for {
		if p.at(KindCOMMA) {
			p.errorAndAdvance("Expecting a delegation specifier")
			continue
		}
		p.parseDelegationSpecifier()
		if !p.at(KindCOMMA) {
			break
		}
		p.advance() // COMMA
	}

	list.Done(KindSUPER_TYPE_LIST)
}

/*
 * delegationSpecifier
 *   : constructorInvocation // type and constructor arguments
 *   : userType
 *   : explicitDelegation
 *   ;
 *
 * explicitDelegation
 *   : userType "by" element
 *   ;
 */
func (p *kotlinParsing) parseDelegationSpecifier() {
	delegator := p.mark()
	reference := p.mark()
	p.parseTypeRef()

	if p.at(KindBY_KEYWORD) {
		reference.Drop()
		p.advance() // BY_KEYWORD
		createForByClause(p.b, p.isLazy).expr.parseExpression()
		delegator.Done(KindDELEGATED_SUPER_TYPE_ENTRY)
	} else if p.at(KindLPAR) {
		reference.Done(KindCONSTRUCTOR_CALLEE)
		p.expr.parseValueArgumentList()
		delegator.Done(KindSUPER_TYPE_CALL_ENTRY)
	} else {
		reference.Drop()
		delegator.Done(KindSUPER_TYPE_ENTRY)
	}
}

/*
 * typeParameters
 *   : ("<" typeParameter{","} ">"
 *   ;
 */
func (p *kotlinParsing) parseTypeParameterList(recoverySet tokenSet) bool {
	result := false
	if p.at(KindLT) {
		list := p.mark()

		p.b.disableNewlines()
		p.advance() // LT

		for {
			if p.at(KindCOMMA) {
				p.errorAndAdvance("Expecting type parameter declaration")
			}
			p.parseTypeParameter()

			if !p.at(KindCOMMA) {
				break
			}
			p.advance() // COMMA
			if p.at(KindGT) {
				break
			}
		}

		p.expectRS(KindGT, "Missing '>'", &recoverySet)
		p.b.restoreNewlinesState()
		result = true

		list.Done(KindTYPE_PARAMETER_LIST)
	}
	return result
}

/*
 * typeConstraints
 *   : ("where" typeConstraint{","})?
 *   ;
 */
func (p *kotlinParsing) parseTypeConstraintsGuarded(typeParameterListOccurred bool) {
	err := p.mark()
	constraints := p.parseTypeConstraints()
	errorIf(err, constraints && !typeParameterListOccurred, "Type constraints are not allowed when no type parameters declared")
}

func (p *kotlinParsing) parseTypeConstraints() bool {
	if p.at(KindWHERE_KEYWORD) {
		p.parseTypeConstraintList()
		return true
	}
	return false
}

/*
 * typeConstraint{","}
 */
func (p *kotlinParsing) parseTypeConstraintList() {
	p.advance() // WHERE_KEYWORD

	list := p.mark()

	for {
		if p.at(KindCOMMA) {
			p.errorAndAdvance("Type constraint expected")
		}
		p.parseTypeConstraint()
		if !p.at(KindCOMMA) {
			break
		}
		p.advance() // COMMA
	}

	list.Done(KindTYPE_CONSTRAINT_LIST)
}

/*
 * typeConstraint
 *   : annotations SimpleName ":" type
 *   ;
 */
func (p *kotlinParsing) parseTypeConstraint() {
	constraint := p.mark()

	p.parseAnnotations(annDefault)

	reference := p.mark()
	if p.expectRS(KindIDENTIFIER, "Expecting type parameter name", &kpColonCommaLbraceRbraceTypeRefFirstSet) {
		reference.Done(KindREFERENCE_EXPRESSION)
	} else {
		reference.Drop()
	}

	p.expectRS(KindCOLON, "Expecting ':' before the upper bound", &kpLbraceRbraceTypeRefFirstSet)

	p.parseTypeRef()

	constraint.Done(KindTYPE_CONSTRAINT)
}

func (p *kotlinParsing) parseFunctionContract() bool {
	if p.at(KindCONTRACT_KEYWORD) {
		p.expr.parseContractDescriptionBlock()
		return true
	}
	return false
}

/*
 * typeParameter
 *   : modifiers SimpleName (":" userType)?
 *   ;
 */
func (p *kotlinParsing) parseTypeParameter() {
	if p.atSet(kpTypeParameterGtRecoverySet) {
		p.error("Type parameter declaration expected")
		return
	}

	mark := p.mark()

	p.parseModifierList(kpGtCommaColonSet)

	p.expectRS(KindIDENTIFIER, "Type parameter name expected", &kpEmpty)

	if p.at(KindCOLON) {
		p.advance() // COLON
		p.parseTypeRef()
	}

	mark.Done(KindTYPE_PARAMETER)
}

/*
 * type
 *   : typeModifiers typeReference
 *   ;
 *
 * typeReference
 *   : functionType
 *   : userType
 *   : nullableType
 *   : "dynamic"
 *   ;
 *
 * nullableType
 *   : typeReference "?"
 *   ;
 */
func (p *kotlinParsing) parseTypeRef() {
	p.parseTypeRefRS(kpEmpty)
}

func (p *kotlinParsing) parseTypeRefWithoutIntersections() {
	p.parseTypeRefFull(kpEmpty /* allowSimpleIntersectionTypes */, false)
}

// parseTypeRefRS is parseTypeRef(TokenSet extraRecoverySet).
func (p *kotlinParsing) parseTypeRefRS(extraRecoverySet tokenSet) {
	p.parseTypeRefFull(extraRecoverySet /* allowSimpleIntersectionTypes */, true)
}

// parseTypeRefFull is parseTypeRef(TokenSet, boolean).
func (p *kotlinParsing) parseTypeRefFull(extraRecoverySet tokenSet, allowSimpleIntersectionTypes bool) {
	typeRefMarker := p.parseTypeRefContents(extraRecoverySet, allowSimpleIntersectionTypes)
	typeRefMarker.Done(KindTYPE_REFERENCE)
}

// The extraRecoverySet is needed for the foo(bar<x, 1, y>(z)) case, to tell whether we should stop
// on expression-indicating symbols or not
func (p *kotlinParsing) parseTypeRefContents(extraRecoverySet tokenSet, allowSimpleIntersectionTypes bool) Marker {
	typeRefMarker := p.mark()

	p.parseTypeModifierList()

	lookahead := p.lookahead(1)
	lookahead2 := p.lookahead(2)
	typeBeforeDot := true
	withContextReceiver := p.at(KindCONTEXT_KEYWORD) && lookahead == KindLPAR
	wasFunctionTypeParsed := false

	contextReceiversStart := p.mark()

	if withContextReceiver {
		p.parseContextParameterOrReceiverList(true)
	}

	typeElementMarker := p.mark()

	if p.at(KindIDENTIFIER) && !(lookahead == KindDOT && lookahead2 == KindIDENTIFIER) && lookahead != KindLT && p.at(KindDYNAMIC_KEYWORD) {
		dynamicType := p.mark()
		p.advance() // DYNAMIC_KEYWORD
		dynamicType.Done(KindDYNAMIC_TYPE)
	} else if p.at(KindIDENTIFIER) || p.at(KindPACKAGE_KEYWORD) || p.atParenthesizedMutableForPlatformTypes(0) {
		p.parseUserType()
	} else if p.at(KindLPAR) {
		functionOrParenthesizedType := p.mark()

		// This may be a function parameter list or just a parenthesized type
		p.advance()                                                                     // LPAR
		p.parseTypeRefContents(kpEmpty /* allowSimpleIntersectionTypes */, true).Drop() // parenthesized types, no reference element around it is needed

		if p.at(KindRPAR) && p.lookahead(1) != KindARROW {
			// It's a parenthesized type
			//    (A)
			p.advance()
			functionOrParenthesizedType.Drop()
		} else {
			// This must be a function type
			//   (A, B) -> C
			// or
			//   (a : A) -> C
			functionOrParenthesizedType.RollbackTo()
			p.parseFunctionType(contextReceiversStart.Precede())
			wasFunctionTypeParsed = true
		}
	} else {
		// Deviation: Kotlin says "Type expected" for a star projection
		// used as a type too; psi names it, as lint relies on.
		msg := "Type expected"
		if p.at(KindMUL) {
			msg = starProjectionNotAType
		}
		p.errorWithRecovery(msg,
			kpRS(orSet(kpTopLevelDeclarationFirst,
				newSet(KindEQ, KindCOMMA, KindGT, KindRBRACKET, KindDOT, KindRPAR, KindRBRACE, KindLBRACE, KindSEMICOLON),
				extraRecoverySet)))
		typeBeforeDot = false
	}

	// Disabling token merge is required for cases like
	//    Int?.(Foo) -> Bar
	p.b.disableJoiningComplexTokens()
	typeElementMarker = p.parseNullableTypeSuffix(typeElementMarker)
	p.b.restoreJoiningComplexTokensState()

	wasIntersection := false
	if allowSimpleIntersectionTypes && p.at(KindAND) {
		leftTypeRef := typeElementMarker

		typeElementMarker = typeElementMarker.Precede()
		intersectionType := leftTypeRef.Precede()

		leftTypeRef.Done(KindTYPE_REFERENCE)

		p.advance() // &
		p.parseTypeRefFull(extraRecoverySet /* allowSimpleIntersectionTypes */, true)

		intersectionType.Done(KindINTERSECTION_TYPE)
		wasIntersection = true
	}

	if typeBeforeDot && p.at(KindDOT) && !wasIntersection && !wasFunctionTypeParsed {
		// This is a receiver for a function type
		//  A.(B) -> C
		//   ^

		functionType := contextReceiversStart.Precede()

		receiverTypeRef := typeElementMarker.Precede()
		receiverType := receiverTypeRef.Precede()
		receiverTypeRef.Done(KindTYPE_REFERENCE)
		receiverType.Done(KindFUNCTION_TYPE_RECEIVER)

		p.advance() // DOT

		if p.at(KindLPAR) {
			p.parseFunctionType(functionType)
		} else {
			functionType.Drop()
			p.error("Expecting function type")
		}

		wasFunctionTypeParsed = true
	}

	if withContextReceiver && !wasFunctionTypeParsed {
		p.errorWithRecovery("Function type expected expected",
			kpRS(orSet(kpTopLevelDeclarationFirst,
				newSet(KindEQ, KindCOMMA, KindGT, KindRBRACKET, KindDOT, KindRPAR, KindRBRACE, KindLBRACE, KindSEMICOLON),
				extraRecoverySet)))
	}

	typeElementMarker.Drop()
	contextReceiversStart.Drop()
	return typeRefMarker
}

func (p *kotlinParsing) parseNullableTypeSuffix(typeElementMarker Marker) Marker {
	// ?: is joined regardless of joining state
	for p.at(KindQUEST) && p.b.rawLookup(1) != KindCOLON {
		precede := typeElementMarker.Precede()
		p.advance() // QUEST
		typeElementMarker.Done(KindNULLABLE_TYPE)
		typeElementMarker = precede
	}
	return typeElementMarker
}

/*
 * userType
 *   : simpleUserType{"."}
 *   ;
 *
 *   recovers on platform types:
 *    - Foo!
 *    - (Mutable)List<Foo>!
 *    - Array<(out) Foo>!
 */
func (p *kotlinParsing) parseUserType() {
	userType := p.mark()

	if p.at(KindPACKAGE_KEYWORD) {
		keyword := p.mark()
		p.advance() // PACKAGE_KEYWORD
		keyword.Error("Expecting an element")
		p.expectRS(KindDOT, "Expecting '.'", kpRS(newSet(KindIDENTIFIER, KindLBRACE, KindRBRACE)))
	}

	reference := p.mark()
	for {
		p.recoverOnParenthesizedWordForPlatformTypes(0, "Mutable", true)

		if p.expectRS(KindIDENTIFIER, "Expecting type name",
			&kpUserTypeNameRecoverySet) {
			reference.Done(KindREFERENCE_EXPRESSION)
		} else {
			reference.Drop()
			break
		}

		p.parseTypeArgumentList()

		p.recoverOnPlatformTypeSuffix()

		if !p.at(KindDOT) {
			break
		}
		if p.lookahead(1) == KindLPAR && !p.atParenthesizedMutableForPlatformTypes(1) {
			// This may be a receiver for a function type
			//   Int.(Int) -> Int
			break
		}

		precede := userType.Precede()
		userType.Done(KindUSER_TYPE)
		userType = precede

		p.advance() // DOT
		reference = p.mark()
	}

	userType.Done(KindUSER_TYPE)
}

func (p *kotlinParsing) atParenthesizedMutableForPlatformTypes(offset int) bool {
	return p.recoverOnParenthesizedWordForPlatformTypes(offset, "Mutable", false)
}

func (p *kotlinParsing) recoverOnParenthesizedWordForPlatformTypes(offset int, word string, consume bool) bool {
	// Array<(out) Foo>! or (Mutable)List<Bar>!
	if p.lookahead(offset) == KindLPAR && p.lookahead(offset+1) == KindIDENTIFIER &&
		p.lookahead(offset+2) == KindRPAR && p.lookahead(offset+3) == KindIDENTIFIER {
		err := p.mark()

		p.advanceN(offset)

		p.advance() // LPAR
		if text, ok := p.b.tokenText(); !ok || word != text {
			// something other than "out" / "Mutable"
			err.RollbackTo()
			return false
		}
		p.advance() // IDENTIFIER('out')
		p.advance() // RPAR

		if consume {
			err.Error("Unexpected tokens")
		} else {
			err.RollbackTo()
		}

		return true
	}
	return false
}

func (p *kotlinParsing) recoverOnPlatformTypeSuffix() {
	// Recovery for platform types
	if p.at(KindEXCL) {
		err := p.mark()
		p.advance() // EXCL
		err.Error("Unexpected token")
	}
}

/*
 *  (optionalProjection type){","}
 */
func (p *kotlinParsing) parseTypeArgumentList() {
	if !p.at(KindLT) {
		return
	}

	list := p.mark()

	p.tryParseTypeArgumentList(kpEmpty)

	list.Done(KindTYPE_ARGUMENT_LIST)
}

func (p *kotlinParsing) tryParseTypeArgumentList(extraRecoverySet tokenSet) bool {
	p.b.disableNewlines()
	p.advance() // LT

	for {
		projection := p.mark()

		p.recoverOnParenthesizedWordForPlatformTypes(0, "out", true)

		// Currently we do not allow annotations on star projections and probably we should not
		// Annotations on other kinds of type arguments should be parsed as common type annotations (within parseTypeRef call)
		p.parseTypeArgumentModifierList()

		if p.at(KindMUL) {
			p.advance() // MUL
		} else {
			p.parseTypeRefRS(extraRecoverySet)
		}
		projection.Done(KindTYPE_PROJECTION)
		if !p.at(KindCOMMA) {
			break
		}
		p.advance() // COMMA
		if p.at(KindGT) {
			break
		}
	}

	atGT := p.at(KindGT)
	if !atGT {
		p.error("Expecting a '>'")
	} else {
		p.advance() // GT
	}
	p.b.restoreNewlinesState()
	return atGT
}

/*
 * functionType
 *   : (type ".")? "(" parameter{","}? ")" "->" type?
 *   ;
 */
func (p *kotlinParsing) parseFunctionType(functionType Marker) {
	p.parseFunctionTypeContents(functionType).Done(KindFUNCTION_TYPE)
}

func (p *kotlinParsing) parseFunctionTypeContents(functionType Marker) Marker {
	p.parseValueParameterList(true /* typeRequired  = */, true, kpEmpty)

	p.expectRS(KindARROW, "Expecting '->' to specify return type of a function type", &kpTypeRefFirst)
	p.parseTypeRef()

	return functionType
}

/*
 * functionParameters
 *   : "(" functionParameter{","}? ")"
 *   ;
 *
 * functionParameter
 *   : modifiers functionParameterRest
 *   ;
 *
 * functionParameterRest
 *   : parameter ("=" element)?
 *   ;
 */
func (p *kotlinParsing) parseValueParameterList(isFunctionTypeContents, typeRequired bool, recoverySet tokenSet) {
	parameters := p.mark()

	p.b.disableNewlines()

	p.valueParameterLoop(isFunctionTypeContents, recoverySet, func() bool {
		if isFunctionTypeContents {
			if !p.tryParseValueParameter(typeRequired) {
				valueParameter := p.mark()
				p.parseFunctionTypeValueParameterModifierList()
				p.parseTypeRef()
				closeDeclarationWithCommentBinders(valueParameter, KindVALUE_PARAMETER, false)
			}
		} else {
			p.parseValueParameter(typeRequired)
		}
		return true
	})

	p.b.restoreNewlinesState()

	parameters.Done(KindVALUE_PARAMETER_LIST)
}

// valueParameterLoop parses "(" parameter{","} ")". parseParameter returns
// true if internal parsing is correct; the result is true if the parsing of
// the entire parameter loop is correct.
func (p *kotlinParsing) valueParameterLoop(inFunctionTypeContext bool, recoverySet tokenSet, parseParameter func() bool) bool {
	p.advance() // LPAR

	noError := true

	if !p.at(KindRPAR) && !p.atSet(recoverySet) {
		for {
			offsetBefore := p.b.currentOffset()
			if p.at(KindRPAR) {
				break
			}

			noError = parseParameter() && noError

			if p.at(KindCOMMA) {
				p.advance() // COMMA
			} else if p.at(KindCOLON) {
				// recovery for the case "fun bar(x: Array<Int> : Int)" when we've just parsed "x: Array<Int>"
				// error should be reported in the `parseValueParameter` call
				continue
			} else {
				if !p.at(KindRPAR) {
					p.error("Expecting comma or ')'")
					noError = false
				}
				first := kpValueParameterFirst
				if inFunctionTypeContext {
					first = kpLambdaValueParameterFirst
				}
				if !p.atSet(first) {
					break
				}
				if offsetBefore == p.b.currentOffset() {
					break
				}
			}
		}
	}

	return p.expectRS(KindRPAR, "Expecting ')'", &recoverySet) && noError
}

/*
 * functionParameter
 *   : modifiers ("val" | "var")? parameter ("=" element)?
 *   ;
 */
func (p *kotlinParsing) tryParseValueParameter(typeRequired bool) bool {
	return p.parseValueParameterRB(true, typeRequired)
}

func (p *kotlinParsing) parseValueParameter(typeRequired bool) {
	p.parseValueParameterRB(false, typeRequired)
}

// parseValueParameterRB is parseValueParameter(boolean rollbackOnFailure, boolean typeRequired).
func (p *kotlinParsing) parseValueParameterRB(rollbackOnFailure, typeRequired bool) bool {
	parameter := p.mark()

	p.parseModifierList(kpNoModifierBeforeForValueParameter)

	if p.at(KindVAR_KEYWORD) || p.at(KindVAL_KEYWORD) {
		p.advance() // VAR_KEYWORD | VAL_KEYWORD
	}

	if !p.parseFunctionParameterRest(typeRequired) && rollbackOnFailure {
		parameter.RollbackTo()
		return false
	}

	closeDeclarationWithCommentBinders(parameter, KindVALUE_PARAMETER, false)
	return true
}

/*
 * functionParameterRest
 *   : parameter ("=" element)?
 *   ;
 */
func (p *kotlinParsing) parseFunctionParameterRest(typeRequired bool) bool {
	noErrors := true

	if p.at(KindCOMMA) || p.at(KindRPAR) {
		p.error("Expecting a parameter declaration")
		noErrors = false
	} else if p.at(KindIDENTIFIER) && p.lookahead(1) == KindLT {
		// Recovery for the case 'fun foo(Array<String>) {}'
		p.error("Parameter name expected")
		noErrors = false
		p.parseTypeRef()
	} else if p.at(KindCOLON) {
		// Recovery for the case 'fun foo(: Int) {}'
		p.error("Parameter name expected")
		// We keep noErrors == true so that unnamed parameters starting with ":" are not rolled back during parsing of functional types
		p.advance() // COLON
		p.parseTypeRef()
	} else {
		p.expectRS(KindIDENTIFIER, "Parameter name expected", &kpParameterNameRecoverySet)

		if p.at(KindCOLON) {
			p.advance() // COLON

			if p.at(KindIDENTIFIER) && p.lookahead(1) == KindCOLON {
				// recovery for the case "fun foo(x: y: Int)" when we're at "y: " it's likely that this is a name of the next parameter,
				// not a type reference of the current one
				p.error("Type reference expected")
				return false
			}

			p.parseTypeRef()
		} else if typeRequired {
			p.errorWithRecovery("Parameters must have type annotation", &kpParameterNameRecoverySet)
			noErrors = false
		}
	}

	if p.at(KindEQ) {
		p.advance() // EQ
		p.expr.parseExpression()
	}

	return noErrors
}
