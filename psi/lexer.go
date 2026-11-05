package psi

import (
	"unicode"
	"unicode/utf8"
)

// The lexer is a hand-written port of Kotlin's Kotlin.flex (Kotlin
// 2.4.21; see THIRD_PARTY_NOTICES.md). It follows JFlex semantics: the
// longest match wins, and among equal lengths the rule listed first in the
// grammar wins. Offsets are UTF-8 byte offsets.

type lexState uint8

const (
	stInitial lexState = iota
	stStringPrefix
	stString
	stRawString
	stShortTemplate
	stBlockComment
	stDocComment
	stLongTemplate
	stUnmatchedBacktick
)

type lexFrame struct {
	state     lexState
	lBraces   int
	interpReq int
}

type lexer struct {
	src   string
	pos   int
	state lexState
	// lBraces counts unmatched '{' inside a long template entry;
	// interpReq is the number of '$' an interpolation needs (-1 outside a
	// string).
	lBraces   int
	interpReq int
	stack     []lexFrame

	commentStart int
	commentDepth int
	// tokStart is where the token returned by next starts; a comment
	// starts before the text matched last.
	tokStart int
}

// tokenize splits src into tokens covering it exactly. kinds[i] spans
// [starts[i], starts[i+1]); starts has one extra entry equal to len(src).
func tokenize(src string, kinds []Kind, starts []int32) ([]Kind, []int32) {
	lx := lexer{src: src, interpReq: -1}
	for {
		k, ok := lx.next()
		if !ok {
			break
		}
		kinds = append(kinds, k)
		starts = append(starts, int32(lx.tokStart))
	}
	starts = append(starts, int32(len(src)))
	return kinds, starts
}

func (lx *lexer) push(st lexState) {
	lx.stack = append(lx.stack, lexFrame{lx.state, lx.lBraces, lx.interpReq})
	lx.lBraces = 0
	lx.interpReq = -1
	lx.state = st
}

func (lx *lexer) pushInterp(prefix int) {
	lx.stack = append(lx.stack, lexFrame{lx.state, lx.lBraces, lx.interpReq})
	lx.lBraces = 0
	lx.interpReq = prefix
	lx.state = stStringPrefix
}

func (lx *lexer) pop() {
	if len(lx.stack) == 0 {
		lx.state, lx.lBraces, lx.interpReq = stInitial, 0, -1
		return
	}
	f := lx.stack[len(lx.stack)-1]
	lx.stack = lx.stack[:len(lx.stack)-1]
	lx.state, lx.lBraces, lx.interpReq = f.state, f.lBraces, f.interpReq
}

func (lx *lexer) at(i int) byte {
	if i < len(lx.src) {
		return lx.src[i]
	}
	return 0
}

func (lx *lexer) has(i int, s string) bool {
	return i+len(s) <= len(lx.src) && lx.src[i:i+len(s)] == s
}

func (lx *lexer) runeAt(i int) (rune, int) {
	if i >= len(lx.src) {
		return -1, 0
	}
	c := lx.src[i]
	if c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(lx.src[i:])
}

func isLetter(r rune) bool {
	if r < utf8.RuneSelf {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_'
	}
	return unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	if r < utf8.RuneSelf {
		return isLetter(r) || r >= '0' && r <= '9'
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// matchIdent returns the end of {IDENTIFIER} at i, or -1.
func (lx *lexer) matchIdent(i int) int {
	if lx.at(i) == '`' {
		j := i + 1
		for j < len(lx.src) && lx.src[j] != '`' && lx.src[j] != '\n' {
			j++
		}
		if j > i+1 && lx.at(j) == '`' {
			return j + 1
		}
		return -1
	}
	r, n := lx.runeAt(i)
	if n == 0 || !isLetter(r) {
		return -1
	}
	j := i + n
	for {
		r, n := lx.runeAt(j)
		if n == 0 || !isIdentPart(r) {
			return j
		}
		j += n
	}
}

// matchEscape returns the end of {ESCAPE_SEQUENCE} at i, or -1.
func (lx *lexer) matchEscape(i int) int {
	if lx.at(i) != '\\' || i+1 >= len(lx.src) || lx.src[i+1] == '\n' {
		return -1
	}
	if lx.src[i+1] == 'u' && i+6 <= len(lx.src) && isHex(lx.src[i+2]) && isHex(lx.src[i+3]) && isHex(lx.src[i+4]) && isHex(lx.src[i+5]) {
		return i + 6
	}
	_, n := lx.runeAt(i + 1)
	return i + 1 + n
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// next returns the next token's kind and sets lx.tokStart; false means
// end of input.
func (lx *lexer) next() (Kind, bool) {
	lx.tokStart = lx.pos
	for {
		if lx.pos >= len(lx.src) {
			switch lx.state {
			case stBlockComment, stDocComment:
				st := lx.state
				lx.pop()
				lx.tokStart = lx.commentStart
				if st == stDocComment {
					return kDocComment, true
				}
				return KindBLOCK_COMMENT, true
			}
			return 0, false
		}
		var k Kind
		var ok bool
		switch lx.state {
		case stStringPrefix:
			k, ok = lx.lexStringPrefix()
		case stString, stRawString:
			k, ok = lx.lexString()
		case stShortTemplate:
			k, ok = lx.lexShortTemplate()
		case stBlockComment, stDocComment:
			k, ok = lx.lexComment()
		default:
			k, ok = lx.lexMain()
		}
		if ok {
			return k, true
		}
	}
}

func (lx *lexer) lexStringPrefix() (Kind, bool) {
	if lx.has(lx.pos, `"""`) {
		lx.pos += 3
		lx.state = stRawString
		return KindOPEN_QUOTE, true
	}
	if lx.at(lx.pos) == '"' {
		lx.pos++
		lx.state = stString
		return KindOPEN_QUOTE, true
	}
	// Unreachable: STRING_PREFIX is entered with a quote pushed back.
	lx.pop()
	return 0, false
}

func (lx *lexer) lexString() (Kind, bool) {
	p := lx.pos
	c := lx.src[p]
	raw := lx.state == stRawString
	switch {
	case raw && c == '"' && lx.has(p, `"""`):
		j := p + 3
		for lx.at(j) == '"' {
			j++
		}
		if j-p <= 3 {
			lx.pos = j
			lx.pop()
			return KindCLOSING_QUOTE, true
		}
		lx.pos = j - 3
		return KindREGULAR_STRING_PART, true
	case raw && (c == '\n' || c == '"' || c == '\\'):
		lx.pos++
		return KindREGULAR_STRING_PART, true
	case c == '\n':
		lx.pop()
		return KindDANGLING_NEWLINE, true
	case c == '"':
		lx.pos++
		lx.pop()
		return KindCLOSING_QUOTE, true
	case c == '\\':
		if e := lx.matchEscape(p); e > 0 {
			lx.pos = e
			return KindESCAPE_SEQUENCE, true
		}
		lx.pos++
		return KindBAD_CHARACTER, true
	case c == '$':
		j := p
		for lx.at(j) == '$' {
			j++
		}
		dollars := j - p
		if e := lx.matchIdent(j); e > 0 {
			switch {
			case dollars == lx.interpReq:
				lx.pos = j
				lx.push(stShortTemplate)
				return KindSHORT_TEMPLATE_ENTRY_START, true
			case dollars < lx.interpReq:
				lx.pos = j
			default:
				lx.pos = p + dollars - lx.interpReq
			}
			return KindREGULAR_STRING_PART, true
		}
		if lx.at(j) == '{' {
			switch {
			case dollars == lx.interpReq:
				lx.pos = j + 1
				lx.push(stLongTemplate)
				return KindLONG_TEMPLATE_ENTRY_START, true
			case dollars < lx.interpReq:
				lx.pos = j
			default:
				lx.pos = p + dollars - lx.interpReq
			}
			return KindREGULAR_STRING_PART, true
		}
		lx.pos = j // LONELY_DOLLAR (\$+)
		return KindREGULAR_STRING_PART, true
	}
	j := p
	for j < len(lx.src) {
		c := lx.src[j]
		if c == '\\' || c == '"' || c == '\n' || c == '$' {
			break
		}
		j++
	}
	lx.pos = j
	return KindREGULAR_STRING_PART, true
}

func (lx *lexer) lexShortTemplate() (Kind, bool) {
	e := lx.matchIdent(lx.pos)
	if e < 0 {
		_, n := lx.runeAt(lx.pos)
		lx.pos += n
		return KindBAD_CHARACTER, true
	}
	k := KindIDENTIFIER
	if lx.src[lx.pos:e] == "this" {
		k = KindTHIS_KEYWORD
	}
	lx.pos = e
	lx.pop()
	return k, true
}

func (lx *lexer) lexComment() (Kind, bool) {
	p := lx.pos
	switch {
	case lx.has(p, "/*"):
		lx.pos += 2
		lx.commentDepth++
	case lx.has(p, "*/"):
		lx.pos += 2
		if lx.commentDepth > 0 {
			lx.commentDepth--
			return 0, false
		}
		st := lx.state
		lx.pop()
		lx.tokStart = lx.commentStart
		if st == stDocComment {
			return kDocComment, true
		}
		return KindBLOCK_COMMENT, true
	default:
		_, n := lx.runeAt(p)
		lx.pos += n
	}
	return 0, false
}

var singleOps = [256]Kind{
	'[': KindLBRACKET, ']': KindRBRACKET, '{': KindLBRACE, '}': KindRBRACE, '(': KindLPAR, ')': KindRPAR,
	'.': KindDOT, '*': KindMUL, '+': KindPLUS, '-': KindMINUS, '!': KindEXCL, '/': KindDIV, '%': KindPERC,
	'<': KindLT, '>': KindGT, '?': KindQUEST, ':': KindCOLON, ';': KindSEMICOLON, '=': KindEQ,
	',': KindCOMMA, '#': KindHASH, '@': KindAT, '&': KindAND,
}

var multiOps = []struct {
	text string
	kind Kind
}{
	// Three-character operators first so the longest match wins.
	{"...", KindRESERVED}, {"===", KindEQEQEQ}, {"!==", kExclEqEqEq}, {"!in", KindNOT_IN},
	{"!is", KindNOT_IS}, {"as?", KindAS_SAFE}, {"..<", KindRANGE_UNTIL},
	{"++", KindPLUSPLUS}, {"--", KindMINUSMINUS}, {"<=", KindLTEQ}, {">=", KindGTEQ}, {"==", KindEQEQ},
	{"!=", KindEXCLEQ}, {"&&", KindANDAND}, {"||", KindOROR}, {"*=", KindMULTEQ}, {"/=", KindDIVEQ},
	{"%=", KindPERCEQ}, {"+=", KindPLUSEQ}, {"-=", KindMINUSEQ}, {"->", KindARROW}, {"=>", KindDOUBLE_ARROW},
	{"..", KindRANGE}, {"::", KindCOLONCOLON}, {";;", KindDOUBLE_SEMICOLON},
}

func (lx *lexer) lexMain() (Kind, bool) {
	p := lx.pos
	c := lx.src[p]

	// String start: {INTERPOLATION}?\"
	if c == '"' || c == '$' {
		j := p
		for lx.at(j) == '$' {
			j++
		}
		if lx.at(j) == '"' {
			prefix := j - p
			lx.pos = j
			lx.pushInterp(max(prefix, 1))
			if prefix != 0 {
				return KindINTERPOLATION_PREFIX, true
			}
			return 0, false
		}
	}
	if lx.state == stLongTemplate {
		switch c {
		case '{':
			lx.lBraces++
			lx.pos++
			return KindLBRACE, true
		case '}':
			lx.pos++
			if lx.lBraces == 0 {
				lx.pop()
				return KindLONG_TEMPLATE_ENTRY_END, true
			}
			lx.lBraces--
			return KindRBRACE, true
		}
	}
	if c == '/' {
		switch {
		case lx.has(p, "/**/"):
			lx.pos += 4
			return KindBLOCK_COMMENT, true
		case lx.has(p, "/**"):
			lx.push(stDocComment)
			lx.commentDepth = 0
			lx.commentStart = p
			lx.pos += 3
			return 0, false
		case lx.has(p, "/*"):
			lx.push(stBlockComment)
			lx.commentDepth = 0
			lx.commentStart = p
			lx.pos += 2
			return 0, false
		case lx.has(p, "//"):
			j := p
			for j < len(lx.src) && lx.src[j] != '\n' {
				j++
			}
			lx.pos = j
			return KindEOL_COMMENT, true
		}
	}
	switch c {
	case ' ', '\n', '\t', '\f':
		j := p + 1
		for j < len(lx.src) && (lx.src[j] == ' ' || lx.src[j] == '\n' || lx.src[j] == '\t' || lx.src[j] == '\f') {
			j++
		}
		lx.pos = j
		return KindWHITE_SPACE, true
	case '#':
		if lx.has(p, "#!") {
			if p == 0 {
				j := p
				for j < len(lx.src) && lx.src[j] != '\n' {
					j++
				}
				lx.pos = j
				return KindSHEBANG_COMMENT, true
			}
			lx.pos++
			return KindHASH, true
		}
	case '\'':
		lx.pos = lx.matchChar(p)
		return KindCHARACTER_LITERAL, true
	}
	if isDigit(c) || c == '.' && isDigit(lx.at(p+1)) {
		return lx.lexNumber(p), true
	}
	if c == '$' {
		if e := lx.matchIdent(p + 1); e > 0 {
			lx.pos = e
			return KindFIELD_IDENTIFIER, true
		}
	}
	if e := lx.matchIdent(p); e > 0 {
		// "as?" is longer than the keyword "as".
		if e == p+2 && lx.src[p:e] == "as" && lx.at(e) == '?' {
			lx.pos = e + 1
			return KindAS_SAFE, true
		}
		lx.pos = e
		if k, ok := hardKeywords[lx.src[p:e]]; ok {
			return k, true
		}
		return KindIDENTIFIER, true
	}
	if c == '!' && (lx.has(p, "!in") || lx.has(p, "!is")) {
		if r, n := lx.runeAt(p + 3); n > 0 && isIdentPart(r) {
			lx.pos = p + 1
			return KindEXCL, true
		}
	}
	for _, op := range multiOps {
		if lx.has(p, op.text) {
			lx.pos = p + len(op.text)
			return op.kind, true
		}
	}
	if k := singleOps[c]; k != 0 {
		lx.pos = p + 1
		return k, true
	}
	if c == '`' {
		lx.push(stUnmatchedBacktick)
		lx.pos = p + 1
		return KindBAD_CHARACTER, true
	}
	_, n := lx.runeAt(p)
	lx.pos = p + n
	return KindBAD_CHARACTER, true
}

// matchChar returns the end of {CHARACTER_LITERAL} at p.
func (lx *lexer) matchChar(p int) int {
	j := p + 1
	for j < len(lx.src) {
		switch c := lx.src[j]; c {
		case '\'':
			return j + 1
		case '\n':
			return j
		case '\\':
			if e := lx.matchEscape(j); e > 0 {
				j = e
				continue
			}
			return j + 1
		default:
			_, n := lx.runeAt(j)
			j += n
		}
	}
	return j
}

// digitsEnd returns the end of {DIGITS} ([0-9][_0-9]*) at i, or -1.
func (lx *lexer) digitsEnd(i int) int {
	if !isDigit(lx.at(i)) {
		return -1
	}
	i++
	for isDigit(lx.at(i)) || lx.at(i) == '_' {
		i++
	}
	return i
}

func (lx *lexer) intSuffix(i int) int {
	if c := lx.at(i); c == 'u' || c == 'U' {
		i++
	}
	if c := lx.at(i); c == 'l' || c == 'L' {
		i++
	}
	return i
}

// exponentEnd returns the end of {EXPONENT_PART} at i, or -1.
func (lx *lexer) exponentEnd(i int) int {
	if c := lx.at(i); c != 'e' && c != 'E' {
		return -1
	}
	i++
	if c := lx.at(i); c == '+' || c == '-' {
		i++
	}
	for isDigit(lx.at(i)) || lx.at(i) == '_' {
		i++
	}
	return i
}

func (lx *lexer) floatSuffix(i int) int {
	if c := lx.at(i); c == 'f' || c == 'F' {
		return i + 1
	}
	return i
}

func (lx *lexer) lexNumber(p int) Kind {
	// Longest INTEGER_LITERAL.
	intEnd := -1
	if isDigit(lx.at(p)) {
		// DECIMAL_INTEGER_LITERAL is {DIGIT}{DIGIT_OR_UNDERSCORE}*, so a
		// leading zero no longer ends the literal (0123 is one token).
		intEnd = lx.intSuffix(lx.digitsEnd(p))
		if lx.at(p) == '0' {
			if c := lx.at(p + 1); c == 'x' || c == 'X' {
				j := p + 2
				for isHex(lx.at(j)) || lx.at(j) == '_' {
					j++
				}
				intEnd = max(intEnd, lx.intSuffix(j))
			} else if c == 'b' || c == 'B' {
				j := p + 2
				for isDigit(lx.at(j)) || lx.at(j) == '_' {
					j++
				}
				intEnd = max(intEnd, lx.intSuffix(j))
			}
		}
	}
	// Longest DOUBLE_LITERAL.
	dblEnd := -1
	if d := lx.digitsEnd(p); d > 0 {
		if lx.at(d) == '.' {
			if f := lx.digitsEnd(d + 1); f > 0 {
				e := f
				if x := lx.exponentEnd(f); x > 0 {
					e = x
				}
				dblEnd = max(dblEnd, lx.floatSuffix(e))
			}
		}
		if x := lx.exponentEnd(d); x > 0 {
			dblEnd = max(dblEnd, lx.floatSuffix(x))
		}
		if f := lx.floatSuffix(d); f > d {
			dblEnd = max(dblEnd, f)
		}
	} else if lx.at(p) == '.' {
		if f := lx.digitsEnd(p + 1); f > 0 {
			e := f
			if x := lx.exponentEnd(f); x > 0 {
				e = x
			}
			dblEnd = lx.floatSuffix(e)
		}
	}
	if intEnd > 0 && lx.has(intEnd, "..") && intEnd+2 > dblEnd {
		lx.pos = intEnd
		return KindINTEGER_LITERAL
	}
	if dblEnd > intEnd {
		lx.pos = dblEnd
		return KindFLOAT_LITERAL
	}
	lx.pos = intEnd
	return KindINTEGER_LITERAL
}
