package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// lexer turns source into tokens. Newlines are significant in Kotlin, so
// it emits NL tokens, except inside (...) and [...] where the grammar lets
// newlines appear almost anywhere.
type lexer struct {
	src    string
	off    int
	line   int
	col    int
	file   string
	stack  []byte // open brackets: '(', '[', '{'
	errs   []*Error
	toks   []Token
	inExpr bool // lexing a ${...} template expression: newlines never matter
}

// Tokenize lexes a whole file.
func Tokenize(file, src string) ([]Token, []*Error) {
	lx := &lexer{src: src, line: 1, col: 1, file: file}
	lx.run()
	return lx.toks, lx.errs
}

func tokenizeAt(file, src string, pos Pos) ([]Token, []*Error) {
	lx := &lexer{src: src, line: pos.Line, col: pos.Col, file: file, inExpr: true}
	lx.run()
	return lx.toks, lx.errs
}

func (lx *lexer) errorf(p Pos, msg string) {
	lx.errs = append(lx.errs, &Error{File: lx.file, Pos: p, Msg: msg})
}

func (lx *lexer) pos() Pos { return Pos{lx.line, lx.col} }

func (lx *lexer) peekByte(n int) byte {
	if lx.off+n < len(lx.src) {
		return lx.src[lx.off+n]
	}
	return 0
}

func (lx *lexer) advance(n int) {
	for i := 0; i < n && lx.off < len(lx.src); i++ {
		if lx.src[lx.off] == '\n' {
			lx.line++
			lx.col = 1
		} else {
			lx.col++
		}
		lx.off++
	}
}

func (lx *lexer) emit(k Kind, p Pos, text string) {
	lx.toks = append(lx.toks, Token{Kind: k, Pos: p, Text: text})
}

func (lx *lexer) newlinesSignificant() bool {
	if lx.inExpr {
		return false
	}
	if n := len(lx.stack); n > 0 && lx.stack[n-1] != '{' {
		return false
	}
	return true
}

// skipSpace skips blanks and comments and reports whether a newline was seen.
func (lx *lexer) skipSpace() bool {
	sawNL := false
	for lx.off < len(lx.src) {
		c := lx.src[lx.off]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			lx.advance(1)
		case c == '\n':
			sawNL = true
			lx.advance(1)
		case c == '/' && lx.peekByte(1) == '/':
			for lx.off < len(lx.src) && lx.src[lx.off] != '\n' {
				lx.advance(1)
			}
		case c == '/' && lx.peekByte(1) == '*':
			start := lx.pos()
			lx.advance(2)
			depth := 1 // Kotlin block comments nest
			for depth > 0 {
				if lx.off >= len(lx.src) {
					lx.errorf(start, "unterminated comment")
					return sawNL
				}
				if lx.src[lx.off] == '/' && lx.peekByte(1) == '*' {
					depth++
					lx.advance(2)
				} else if lx.src[lx.off] == '*' && lx.peekByte(1) == '/' {
					depth--
					lx.advance(2)
				} else {
					lx.advance(1)
				}
			}
		default:
			return sawNL
		}
	}
	return sawNL
}

func isLetter(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
func isDigit(c byte) bool  { return '0' <= c && c <= '9' }
func isHex(c byte) bool    { return isDigit(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' }

// identChar returns the length in bytes of the identifier character at
// off, or 0 if there is none. Identifiers can contain non-ASCII letters
// and digits; first excludes digits.
func (lx *lexer) identChar(off int, first bool) int {
	if off >= len(lx.src) {
		return 0
	}
	if c := lx.src[off]; c < utf8.RuneSelf {
		if isLetter(c) || !first && isDigit(c) {
			return 1
		}
		return 0
	}
	r, size := utf8.DecodeRuneInString(lx.src[off:])
	if unicode.IsLetter(r) || !first && unicode.IsDigit(r) {
		return size
	}
	return 0
}

// scanIdent advances past the rest of an identifier.
func (lx *lexer) scanIdent() {
	for n := lx.identChar(lx.off, false); n > 0; n = lx.identChar(lx.off, false) {
		lx.off += n
		lx.col++
	}
}

func (lx *lexer) run() {
	for {
		p := lx.pos()
		if lx.skipSpace() && lx.newlinesSignificant() {
			if n := len(lx.toks); n > 0 && lx.toks[n-1].Kind != NL {
				lx.emit(NL, p, "")
			}
		}
		if lx.off >= len(lx.src) {
			lx.emit(EOF, lx.pos(), "")
			return
		}
		lx.next()
	}
}

func (lx *lexer) next() {
	p := lx.pos()
	c := lx.src[lx.off]
	switch {
	case lx.identChar(lx.off, true) > 0:
		start := lx.off
		lx.scanIdent()
		word := lx.src[start:lx.off]
		k, ok := keywords[word]
		switch {
		case !ok && lx.peekByte(0) == '@':
			// A label: loop@ for (...), lit@{ ... }.
			lx.advance(1)
			lx.emit(LABEL, p, word)
		case !ok:
			lx.emit(IDENT, p, word)
		case k == AS && lx.peekByte(0) == '?':
			lx.advance(1)
			lx.emit(AS_SAFE, p, "as?")
		default:
			// A label after a keyword: return@forEach, this@Outer. The
			// token's text includes it.
			switch k {
			case RETURN, BREAK, CONTINUE, THIS, SUPER:
				if lx.peekByte(0) == '@' && lx.identChar(lx.off+1, true) > 0 {
					lx.advance(1)
					lx.scanIdent()
					word = lx.src[start:lx.off]
				}
			}
			lx.emit(k, p, word)
		}
		return
	case c == '`':
		lx.advance(1)
		start := lx.off
		for lx.off < len(lx.src) && lx.src[lx.off] != '`' && lx.src[lx.off] != '\n' {
			lx.advance(1)
		}
		name := lx.src[start:lx.off]
		if lx.peekByte(0) != '`' || name == "" {
			lx.errorf(p, "unterminated backtick identifier")
		}
		lx.advance(1)
		lx.emit(IDENT, p, name)
		return
	case isDigit(c) || c == '.' && isDigit(lx.peekByte(1)):
		lx.number(p) // .5 is a Double
		return
	case c == '"':
		lx.str(p)
		return
	case c == '\'':
		lx.char(p)
		return
	case c >= utf8.RuneSelf:
		r, size := utf8.DecodeRuneInString(lx.src[lx.off:])
		lx.errorf(p, "unexpected character "+string(r))
		lx.off += size
		lx.col++
		return
	}

	type op struct {
		s string
		k Kind
	}
	// Longest match first.
	ops := [...]op{
		{"===", EQ_EQ_EQ}, {"!==", NOT_EQ_EQ}, {"..<", RANGE_LT}, {"::", COLON_COLON}, {"?.", SAFE_DOT}, {"?:", ELVIS}, {"!!", NOT_NULL}, {"->", ARROW},
		{"..", RANGE}, {"++", INC}, {"--", DEC}, {"+=", PLUS_EQ}, {"-=", MINUS_EQ}, {"*=", STAR_EQ},
		{"/=", SLASH_EQ}, {"%=", PERCENT_EQ}, {"==", EQ}, {"!=", NE}, {"<=", LE}, {">=", GE},
		{"&&", AND}, {"||", OR}, {"&", AMP},
		{"(", LPAREN}, {")", RPAREN}, {"{", LBRACE}, {"}", RBRACE}, {"[", LBRACK}, {"]", RBRACK},
		{",", COMMA}, {":", COLON}, {";", SEMI}, {".", DOT}, {"?", QUEST}, {"+", PLUS}, {"-", MINUS},
		{"*", STAR}, {"/", SLASH}, {"%", PERCENT}, {"=", ASSIGN}, {"<", LT}, {">", GT}, {"!", NOT},
		{"@", AT},
	}
	// !in and !is are single tokens when not followed by an identifier
	// character, so !isEmpty() is a negation.
	for _, o := range [...]op{{"!in", NOT_IN}, {"!is", NOT_IS}} {
		if strings.HasPrefix(lx.src[lx.off:], o.s) && lx.identChar(lx.off+3, false) == 0 {
			lx.advance(3)
			lx.emit(o.k, p, o.s)
			return
		}
	}
	// A callable reference on a nullable type, String?::plus, is ? and ::,
	// not ?: and :.
	if strings.HasPrefix(lx.src[lx.off:], "?::") {
		lx.advance(1)
		lx.emit(QUEST, p, "?")
		return
	}
	for _, o := range ops {
		if strings.HasPrefix(lx.src[lx.off:], o.s) {
			lx.advance(len(o.s))
			lx.emit(o.k, p, o.s)
			switch o.k {
			case LPAREN, LBRACK, LBRACE:
				lx.stack = append(lx.stack, o.s[0])
			case RPAREN, RBRACK, RBRACE:
				if n := len(lx.stack); n > 0 {
					lx.stack = lx.stack[:n-1]
				}
			}
			return
		}
	}
	lx.errorf(p, "unexpected character "+string(c))
	lx.advance(1)
}

func (lx *lexer) number(p Pos) {
	start := lx.off
	isDouble := false
	if lx.src[lx.off] == '0' && (lx.peekByte(1) == 'x' || lx.peekByte(1) == 'X' || lx.peekByte(1) == 'b' || lx.peekByte(1) == 'B') {
		lx.advance(2)
		for lx.off < len(lx.src) && (isHex(lx.src[lx.off]) || lx.src[lx.off] == '_') {
			lx.advance(1)
		}
	} else {
		lx.digits()
		// A '.' starts a fraction only if a digit follows; "1..2" is a range.
		if lx.peekByte(0) == '.' && isDigit(lx.peekByte(1)) {
			isDouble = true
			lx.advance(1)
			lx.digits()
		}
		if c := lx.peekByte(0); c == 'e' || c == 'E' {
			isDouble = true
			lx.advance(1)
			if c := lx.peekByte(0); c == '+' || c == '-' {
				lx.advance(1)
			}
			lx.digits()
		}
	}
	k := INT
	if isDouble {
		k = DOUBLE
	}
	switch lx.peekByte(0) {
	case 'L':
		if isDouble {
			lx.errorf(p, "invalid Long literal")
		}
		lx.advance(1)
	case 'f', 'F':
		lx.advance(1)
		k = FLOAT
	case 'u', 'U':
		lx.advance(1)
		if lx.peekByte(0) == 'L' {
			lx.advance(1)
		}
		k = UNSIGNED
	}
	lx.emit(k, p, lx.src[start:lx.off])
}

// char lexes a character literal: 'a', '\n', '\u0041'.
func (lx *lexer) char(p Pos) {
	start := lx.off
	lx.advance(1)
	switch c := lx.peekByte(0); {
	case c == '\\' && lx.peekByte(1) == 'u':
		lx.advance(6)
	case c == '\\':
		lx.advance(2)
	case c >= utf8.RuneSelf:
		_, size := utf8.DecodeRuneInString(lx.src[lx.off:])
		lx.off += size
		lx.col++
	case c != '\n' && c != '\'' && c != 0:
		lx.advance(1)
	}
	if lx.peekByte(0) != '\'' {
		lx.errorf(p, "unterminated character literal")
	} else {
		lx.advance(1)
	}
	lx.emit(CHAR, p, lx.src[start:lx.off])
}

// escape returns the character an escape sequence \e stands for, other
// than a unicode escape.
func escape(e byte) (rune, bool) {
	switch e {
	case 't':
		return '\t', true
	case 'b':
		return '\b', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case '\'', '"', '\\', '$':
		return rune(e), true
	}
	return 0, false
}

// hexRune decodes the four hex digits of a unicode escape, \uXXXX.
func hexRune(h string) (rune, bool) {
	var r rune
	for _, d := range h {
		r <<= 4
		switch {
		case '0' <= d && d <= '9':
			r |= d - '0'
		case 'a' <= d && d <= 'f':
			r |= d - 'a' + 10
		case 'A' <= d && d <= 'F':
			r |= d - 'A' + 10
		default:
			return 0, false
		}
	}
	return r, len(h) == 4
}

// charValue decodes a character literal's text, quotes included, to the
// UTF-16 code unit it stands for.
func charValue(text string) (rune, string) {
	s := strings.TrimSuffix(strings.TrimPrefix(text, "'"), "'")
	switch {
	case s == "":
		return 0, "empty character literal"
	case s[0] == '\\' && len(s) == 6 && s[1] == 'u':
		if r, ok := hexRune(s[2:]); ok {
			return r, ""
		}
		return 0, "invalid unicode escape"
	case s[0] == '\\':
		if r, ok := escape(s[1]); ok && len(s) == 2 {
			return r, ""
		}
		return 0, "illegal escape: " + s
	}
	r, size := utf8.DecodeRuneInString(s)
	if size != len(s) || r > 0xffff {
		return 0, "too many characters in a character literal"
	}
	return r, ""
}

func (lx *lexer) digits() {
	for lx.off < len(lx.src) && (isDigit(lx.src[lx.off]) || lx.src[lx.off] == '_') {
		lx.advance(1)
	}
}

// str lexes a string literal, or a raw string literal ("""..."""), which
// has no escapes and can span lines.
func (lx *lexer) str(p Pos) {
	raw := strings.HasPrefix(lx.src[lx.off:], `"""`)
	if raw {
		lx.advance(3)
	} else {
		lx.advance(1)
	}
	var parts []StrPart
	var sb strings.Builder
	flush := func() {
		if sb.Len() > 0 {
			parts = append(parts, StrPart{Lit: sb.String()})
			sb.Reset()
		}
	}
	for {
		if lx.off >= len(lx.src) || !raw && lx.src[lx.off] == '\n' {
			lx.errorf(p, "unterminated string literal")
			break
		}
		c := lx.src[lx.off]
		if raw && strings.HasPrefix(lx.src[lx.off:], `"""`) {
			// A raw string ends at the last """ of a run of quotes.
			for strings.HasPrefix(lx.src[lx.off+1:], `"""`) {
				sb.WriteByte('"')
				lx.advance(1)
			}
			lx.advance(3)
			break
		}
		if !raw && c == '"' {
			lx.advance(1)
			break
		}
		if !raw && c == '\\' {
			ep := lx.pos()
			lx.advance(1)
			e := lx.peekByte(0)
			lx.advance(1)
			switch r, ok := escape(e); {
			case ok:
				sb.WriteByte(byte(r))
			case e == 'u':
				if lx.off+4 > len(lx.src) {
					lx.errorf(ep, "invalid unicode escape")
					continue
				}
				r, ok := hexRune(lx.src[lx.off : lx.off+4])
				if !ok {
					lx.errorf(ep, "invalid unicode escape")
				}
				lx.advance(4)
				sb.WriteRune(r)
			default:
				lx.errorf(ep, "illegal escape: \\"+string(e))
			}
			continue
		}
		if c == '$' && lx.identChar(lx.off+1, true) > 0 {
			flush()
			lx.advance(1)
			sp := lx.pos()
			start := lx.off
			lx.scanIdent()
			parts = append(parts, StrPart{IsExpr: true, Src: lx.src[start:lx.off], SrcPos: sp})
			continue
		}
		if c == '$' && lx.peekByte(1) == '{' {
			flush()
			lx.advance(2)
			sp := lx.pos()
			start := lx.off
			if !lx.skipTemplate() {
				lx.errorf(sp, "unterminated template expression")
				break
			}
			parts = append(parts, StrPart{IsExpr: true, Src: lx.src[start:lx.off], SrcPos: sp})
			lx.advance(1)
			continue
		}
		if c == '\n' {
			sb.WriteByte(c)
			lx.advance(1)
			continue
		}
		_, size := utf8.DecodeRuneInString(lx.src[lx.off:])
		sb.WriteString(lx.src[lx.off : lx.off+size])
		lx.off += size
		lx.col++
	}
	flush()
	lx.toks = append(lx.toks, Token{Kind: STRING, Pos: p, Parts: parts})
}

// skipTemplate advances to the '}' that ends a ${...} template expression
// and reports whether it found one. String and character literals inside
// it are skipped whole, so their braces and quotes don't count.
func (lx *lexer) skipTemplate() bool {
	depth := 1
	for lx.off < len(lx.src) {
		switch lx.src[lx.off] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return true
			}
		case '"', '\'':
			// Lex the literal and drop its token; the template's source is
			// lexed again when it is parsed.
			n := len(lx.toks)
			if lx.src[lx.off] == '"' {
				lx.str(lx.pos())
			} else {
				lx.char(lx.pos())
			}
			lx.toks = lx.toks[:n]
			continue
		}
		lx.advance(1)
	}
	return false
}
