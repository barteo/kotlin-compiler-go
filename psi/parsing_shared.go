package psi

// Declarations shared by the ports of KotlinParsing (parsing_kotlin.go) and
// KotlinExpressionParsing (parsing_expr.go).
//
// Translation conventions:
//   - Java null IElementType is kNone.
//   - Overloads get suffixed names; cross-file calls use these signatures:
//
//     expr: isAtLabelDefinitionOrMissingIdentifier() bool,
//     parseContractDescriptionBlock(), parseExpression(), parseSimpleNameExpression(),
//     parseFunctionLiteral(), parseFunctionLiteralWith(preferBlock, collapse bool),
//     parseLabelDefinition(), parseStatements(),
//     parseStatementsScript(isScriptTopLevel bool), parseValueArgumentList()
//
//     kotlin: parseTypeRef(), parseTypeRefRS(extraRecoverySet tokenSet),
//     parseTypeRefFull(extraRecoverySet tokenSet, allowSimpleIntersectionTypes bool),
//     parseTypeRefWithoutIntersections(),
//     parseModifierList(noModifiersBefore tokenSet) bool,
//     parseModifierListC(tokenConsumer func(Kind), noModifiersBefore tokenSet, localDeclaration bool) bool,
//     parseAnnotationsList(noModifiersBefore tokenSet),
//     parseAnnotations(mode annotationParsingMode) bool,
//     advanceBalancedBlock(),
//     parseMultiDeclarationEntry(follow, recoverySet tokenSet, mode multiDeclarationMode),
//     parseFunction() Kind, parseFunctionFail(failIfIdentifierExists bool) Kind,
//     parseCommonDeclaration(d *modifierDetector, nameMode nameParsingMode, mode declarationParsingMode) Kind,
//     parseBlock(), parseBlockCollapse(collapse bool),
//     parseValueParameter(typeRequired bool),
//     parseValueParameterRB(rollbackOnFailure, typeRequired bool) bool,
//     parseObject(nameMode nameParsingMode, optionalBody bool),
//     tryParseTypeArgumentList(extraRecoverySet tokenSet) bool,
//     parseProperty(mode declarationParsingMode) Kind,
//     create(b psiBuilder) *kotlinParsing

type kotlinParsing struct {
	abstractParsing
	expr *exprParsing

	lastDotAfterReceiverLParPattern    *firstBefore
	lastDotAfterReceiverNotLParPattern *lastBefore
}

type exprParsing struct {
	abstractParsing
	kp *kotlinParsing
	// byClause is set for the expression parser of a by-clause
	// KotlinParsing, which overrides parseCallWithClosure and create.
	byClause *byClauseBuilder
}

func createForTopLevel(b psiBuilder) *kotlinParsing { return newKotlinParsing(b, true, true) }

func createForTopLevelNonLazy(b psiBuilder) *kotlinParsing { return newKotlinParsing(b, true, false) }

func createForByClause(b psiBuilder, isLazy bool) *kotlinParsing {
	return newKotlinParsing(&byClauseBuilder{psiBuilder: b}, false, isLazy)
}

func newKotlinParsing(b psiBuilder, isTopLevel, isLazy bool) *kotlinParsing {
	p := &kotlinParsing{abstractParsing: abstractParsing{b: b, isLazy: isLazy}}
	p.expr = &exprParsing{abstractParsing: abstractParsing{b: b, isLazy: isLazy}, kp: p}
	if !isTopLevel {
		p.expr.byClause = b.(*byClauseBuilder)
	}
	p.initPatterns()
	return p
}

// create is KotlinParsing.create.
func (p *kotlinParsing) create(b psiBuilder) *kotlinParsing { return createForTopLevel(b) }

// createTruncatedBuilder is AbstractKotlinParsing.createTruncatedBuilder
// for KotlinParsing.
func (p *kotlinParsing) createTruncatedBuilder(eofPosition int) *kotlinParsing {
	return p.create(&truncatedBuilder{psiBuilder: p.b, eofPos: eofPosition})
}

// create is KotlinExpressionParsing.create, overridden for by-clauses.
func (e *exprParsing) create(b psiBuilder) *kotlinParsing {
	if e.byClause != nil {
		return createForByClause(b, e.isLazy)
	}
	return e.kp.create(b)
}

// createTruncatedBuilder is AbstractKotlinParsing.createTruncatedBuilder
// for KotlinExpressionParsing.
func (e *exprParsing) createTruncatedBuilder(eofPosition int) *kotlinParsing {
	return e.create(&truncatedBuilder{psiBuilder: e.b, eofPos: eofPosition})
}

// nameParsingMode is KotlinParsing.NameParsingMode.
type nameParsingMode uint8

const (
	nameRequired nameParsingMode = iota
	nameAllowed
	nameProhibited
)

// declarationParsingMode is KotlinParsing.DeclarationParsingMode.
type declarationParsingMode uint8

const (
	declMemberOrTopLevel declarationParsingMode = iota
	declLocal
	declScriptTopLevel
)

func (m declarationParsingMode) destructuringAllowed() bool { return m != declMemberOrTopLevel }
func (m declarationParsingMode) accessorsAllowed() bool     { return m != declLocal }
func (m declarationParsingMode) canBeEnumUsedAsSoftKeyword() bool {
	return m == declMemberOrTopLevel
}

// annotationParsingMode is KotlinParsing.AnnotationParsingMode.
type annotationParsingMode uint8

const (
	annDefault annotationParsingMode = iota
	annFileAnnotationsBeforePackage
	annFileAnnotationsWhenPackageOmitted
	annTypeContext
	annWithSignificantWhitespaceBeforeArguments
	annWithSignificantWhitespaceBeforeArgumentsNoContext
	annNoAnnotationsNoContext
)

func (m annotationParsingMode) isFileAnnotationParsingMode() bool {
	return m == annFileAnnotationsBeforePackage || m == annFileAnnotationsWhenPackageOmitted
}
func (m annotationParsingMode) allowAnnotations() bool { return m != annNoAnnotationsNoContext }
func (m annotationParsingMode) allowContextList() bool {
	return m == annDefault || m == annWithSignificantWhitespaceBeforeArguments
}
func (m annotationParsingMode) typeContext() bool {
	return m == annTypeContext || m.withSignificantWhitespaceBeforeArguments()
}
func (m annotationParsingMode) withSignificantWhitespaceBeforeArguments() bool {
	return m == annWithSignificantWhitespaceBeforeArguments || m == annWithSignificantWhitespaceBeforeArgumentsNoContext
}

// multiDeclarationMode is KotlinParsing.MultiDeclarationMode.
type multiDeclarationMode uint8

const (
	multiDeclShort multiDeclarationMode = iota
	multiDeclFull
	multiDeclFullValOnly
)

// modifierDetector is KotlinParsing.ModifierDetector.
type modifierDetector struct {
	enumDetected      bool
	companionDetected bool
}

func (d *modifierDetector) consume(k Kind) {
	if k == KindENUM_KEYWORD {
		d.enumDetected = true
	} else if k == KindCOMPANION_KEYWORD {
		d.companionDetected = true
	}
}

// starProjectionNotAType is the diagnostic for '*' where a type is
// expected. Kotlin reports "Type expected"; lint matches this message.
const starProjectionNotAType = "star projection is not a type"
