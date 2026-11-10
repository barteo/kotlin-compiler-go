package psi

// Token and element types share the Kind space, as IElementType does in
// IntelliJ. Kinds the parser needs but never puts in a tree live above
// KindCount.
const (
	// kEOLOrSemicolon is KtTokens.EOL_OR_SEMICOLON: a pseudo token that
	// matches a semicolon, a preceding newline or the end of input.
	kEOLOrSemicolon Kind = 500 + iota
	// kNone is "no token", returned at end of input (Java's null).
	kNone
)

// kDocComment is KtTokens.DOC_COMMENT. psi keeps KDoc as one KDOC leaf.
const kDocComment = KindKDOC

// tokenSet is a TokenSet: a bit set over Kind.
type tokenSet [8]uint64

func newSet(kinds ...Kind) tokenSet {
	var s tokenSet
	for _, k := range kinds {
		s[k>>6] |= 1 << (k & 63)
	}
	return s
}

func (s tokenSet) has(k Kind) bool { return k < 512 && s[k>>6]&(1<<(k&63)) != 0 }

func orSet(sets ...tokenSet) tokenSet {
	var r tokenSet
	for _, s := range sets {
		for i := range r {
			r[i] |= s[i]
		}
	}
	return r
}

func (s tokenSet) minus(o tokenSet) tokenSet {
	for i := range s {
		s[i] &^= o[i]
	}
	return s
}

// Keyword tables (KtTokens).
var (
	hardKeywords = map[string]Kind{
		"package": KindPACKAGE_KEYWORD, "as": KindAS_KEYWORD, "typealias": KindTYPE_ALIAS_KEYWORD,
		"class": KindCLASS_KEYWORD, "this": KindTHIS_KEYWORD, "super": KindSUPER_KEYWORD,
		"val": KindVAL_KEYWORD, "var": KindVAR_KEYWORD, "fun": KindFUN_KEYWORD, "for": KindFOR_KEYWORD,
		"null": KindNULL_KEYWORD, "true": KindTRUE_KEYWORD, "false": KindFALSE_KEYWORD,
		"is": KindIS_KEYWORD, "in": KindIN_KEYWORD, "throw": KindTHROW_KEYWORD,
		"return": KindRETURN_KEYWORD, "break": KindBREAK_KEYWORD, "continue": KindCONTINUE_KEYWORD,
		"object": KindOBJECT_KEYWORD, "if": KindIF_KEYWORD, "try": KindTRY_KEYWORD,
		"else": KindELSE_KEYWORD, "while": KindWHILE_KEYWORD, "do": KindDO_KEYWORD,
		"when": KindWHEN_KEYWORD, "interface": KindINTERFACE_KEYWORD, "typeof": KindTYPEOF_KEYWORD,
	}

	// softKeywordTexts is AbstractKotlinParsing.SOFT_KEYWORD_TEXTS.
	softKeywordTexts = map[string]Kind{
		"file": KindFILE_KEYWORD, "import": KindIMPORT_KEYWORD, "where": KindWHERE_KEYWORD,
		"by": KindBY_KEYWORD, "get": KindGET_KEYWORD, "set": KindSET_KEYWORD,
		"abstract": KindABSTRACT_KEYWORD, "enum": KindENUM_KEYWORD, "contract": KindCONTRACT_KEYWORD,
		"open": KindOPEN_KEYWORD, "inner": KindINNER_KEYWORD, "override": KindOVERRIDE_KEYWORD,
		"private": KindPRIVATE_KEYWORD, "public": KindPUBLIC_KEYWORD, "internal": KindINTERNAL_KEYWORD,
		"protected": KindPROTECTED_KEYWORD, "catch": KindCATCH_KEYWORD, "finally": KindFINALLY_KEYWORD,
		"out": KindOUT_KEYWORD, "final": KindFINAL_KEYWORD, "vararg": KindVARARG_KEYWORD,
		"reified": KindREIFIED_KEYWORD, "dynamic": KindDYNAMIC_KEYWORD, "companion": KindCOMPANION_KEYWORD,
		"constructor": KindCONSTRUCTOR_KEYWORD, "init": KindINIT_KEYWORD, "sealed": KindSEALED_KEYWORD,
		"field": KindFIELD_KEYWORD, "property": KindPROPERTY_KEYWORD, "receiver": KindRECEIVER_KEYWORD,
		"param": KindPARAM_KEYWORD, "setparam": KindSETPARAM_KEYWORD, "delegate": KindDELEGATE_KEYWORD,
		"all":      KindALL_KEYWORD,
		"lateinit": KindLATEINIT_KEYWORD, "data": KindDATA_KEYWORD, "inline": KindINLINE_KEYWORD,
		"noinline": KindNOINLINE_KEYWORD, "tailrec": KindTAILREC_KEYWORD, "external": KindEXTERNAL_KEYWORD,
		"annotation": KindANNOTATION_KEYWORD, "crossinline": KindCROSSINLINE_KEYWORD, "const": KindCONST_KEYWORD,
		"operator": KindOPERATOR_KEYWORD, "infix": KindINFIX_KEYWORD, "suspend": KindSUSPEND_KEYWORD,
		"expect": KindEXPECT_KEYWORD,
		"actual": KindACTUAL_KEYWORD, "value": KindVALUE_KEYWORD, "context": KindCONTEXT_KEYWORD,
	}

	// keywordValue is KtKeywordToken.getValue for every keyword kind.
	keywordValue = func() map[Kind]string {
		m := map[Kind]string{}
		for s, k := range hardKeywords {
			m[k] = s
		}
		for s, k := range softKeywordTexts {
			m[k] = s
		}
		return m
	}()

	softKeywords = func() tokenSet {
		var s tokenSet
		for _, k := range softKeywordTexts {
			s = orSet(s, newSet(k))
		}
		return s
	}()

	// keywordTokens is every KtKeywordToken: hard and soft keywords, plus
	// NOT_IN, NOT_IS and AS_SAFE.
	keywordTokens = func() tokenSet {
		s := orSet(softKeywords, newSet(KindNOT_IN, KindNOT_IS, KindAS_SAFE))
		for _, k := range hardKeywords {
			s = orSet(s, newSet(k))
		}
		return s
	}()

	// KtTokens.KEYWORDS: the hard keywords.
	hardKeywordSet = func() tokenSet {
		s := newSet(KindNOT_IN, KindNOT_IS, KindAS_SAFE)
		for _, k := range hardKeywords {
			s = orSet(s, newSet(k))
		}
		return s
	}()

	// modifierKeywords is KtTokens.MODIFIER_KEYWORDS. Kotlin 2.4 dropped
	// header, impl and contract from it.
	modifierKeywords = newSet(KindABSTRACT_KEYWORD, KindENUM_KEYWORD, KindOPEN_KEYWORD,
		KindINNER_KEYWORD, KindOVERRIDE_KEYWORD, KindPRIVATE_KEYWORD, KindPUBLIC_KEYWORD, KindINTERNAL_KEYWORD,
		KindPROTECTED_KEYWORD, KindOUT_KEYWORD, KindIN_KEYWORD, KindFINAL_KEYWORD, KindVARARG_KEYWORD,
		KindREIFIED_KEYWORD, KindCOMPANION_KEYWORD, KindSEALED_KEYWORD, KindLATEINIT_KEYWORD, KindDATA_KEYWORD,
		KindINLINE_KEYWORD, KindNOINLINE_KEYWORD, KindTAILREC_KEYWORD, KindEXTERNAL_KEYWORD,
		KindANNOTATION_KEYWORD, KindCROSSINLINE_KEYWORD, KindCONST_KEYWORD, KindOPERATOR_KEYWORD,
		KindINFIX_KEYWORD, KindSUSPEND_KEYWORD, KindEXPECT_KEYWORD,
		KindACTUAL_KEYWORD, KindFUN_KEYWORD, KindVALUE_KEYWORD)

	valVar = newSet(KindVAL_KEYWORD, KindVAR_KEYWORD)

	typeModifierKeywords                   = newSet(KindSUSPEND_KEYWORD)
	typeArgumentModifierKeywords           = newSet(KindIN_KEYWORD, KindOUT_KEYWORD)
	reservedValueParameterModifierKeywords = newSet(KindOUT_KEYWORD, KindVARARG_KEYWORD)
	visibilityModifiers                    = newSet(KindPRIVATE_KEYWORD, KindPUBLIC_KEYWORD, KindINTERNAL_KEYWORD, KindPROTECTED_KEYWORD)
	modalityModifiers                      = newSet(KindABSTRACT_KEYWORD, KindFINAL_KEYWORD, KindSEALED_KEYWORD, KindOPEN_KEYWORD)

	commentTokens          = newSet(KindEOL_COMMENT, KindBLOCK_COMMENT, kDocComment, KindSHEBANG_COMMENT)
	whiteSpaceOrCommentSet = orSet(commentTokens, newSet(KindWHITE_SPACE))

	stringTokens = newSet(KindCHARACTER_LITERAL, KindREGULAR_STRING_PART)

	operationTokens = newSet(KindAS_KEYWORD, KindAS_SAFE, KindIS_KEYWORD, KindIN_KEYWORD, KindDOT,
		KindPLUSPLUS, KindMINUSMINUS, KindEXCLEXCL, KindMUL, KindPLUS, KindMINUS, KindEXCL, KindDIV, KindPERC,
		KindLT, KindGT, KindLTEQ, KindGTEQ, KindEQEQEQ, kExclEqEqEq, KindEQEQ, KindEXCLEQ, KindANDAND,
		KindOROR, KindSAFE_ACCESS, KindELVIS, KindRANGE, KindRANGE_UNTIL, KindEQ, KindMULTEQ, KindDIVEQ,
		KindPERCEQ, KindPLUSEQ, KindMINUSEQ, KindNOT_IN, KindNOT_IS, KindIDENTIFIER)

	augmentedAssignments  = newSet(KindPLUSEQ, KindMINUSEQ, KindMULTEQ, KindPERCEQ, KindDIVEQ)
	allAssignments        = newSet(KindEQ, KindPLUSEQ, KindMINUSEQ, KindMULTEQ, KindPERCEQ, KindDIVEQ)
	incrementAndDecrement = newSet(KindPLUSPLUS, KindMINUSMINUS)
	qualifiedAccess       = newSet(KindDOT_QUALIFIED_EXPRESSION, KindSAFE_ACCESS_EXPRESSION)
)

// isKeywordToken reports whether k is a KtKeywordToken, and isSoftKeyword
// whether it is a soft one.
func isKeywordToken(k Kind) bool { return keywordTokens.has(k) }
func isSoftKeyword(k Kind) bool  { return softKeywords.has(k) }

// kExclEqEqEq is KtTokens.EXCLEQEQEQ ("!=="), which psi names EXCLEQEQ.
const kExclEqEqEq = KindEXCLEQEQ
