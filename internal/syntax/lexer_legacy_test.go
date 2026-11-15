// The token lexer kc used before syntax.Tokenize was built on psi.Lex,
// kept to check that the two agree.

package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// legacyLexer turns source into tokens. Newlines are significant in Kotlin, so
// it emits NL tokens, except inside (...) and [...] where the grammar lets
// newlines appear almost anywhere.
type legacyLexer struct {
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

// legacyTokenize lexes a whole file.
func legacyTokenize(file, src string) ([]Token, []*Error) {
	lx := &legacyLexer{src: src, line: 1, col: 1, file: file}
	lx.run()
	return lx.toks, lx.errs
}

func legacyTokenizeAt(file, src string, pos Pos) ([]Token, []*Error) {
	lx := &legacyLexer{src: src, line: pos.Line, col: pos.Col, file: file, inExpr: true}
	lx.run()
	return lx.toks, lx.errs
}

func (lx *legacyLexer) errorf(p Pos, msg string) {
	lx.errs = append(lx.errs, &Error{File: lx.file, Pos: p, Msg: msg})
}

func (lx *legacyLexer) pos() Pos { return Pos{lx.line, lx.col} }

func (lx *legacyLexer) peekByte(n int) byte {
	if lx.off+n < len(lx.src) {
		return lx.src[lx.off+n]
	}
	return 0
}

func (lx *legacyLexer) advance(n int) {
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

func (lx *legacyLexer) emit(k Kind, p Pos, text string) {
	lx.toks = append(lx.toks, Token{Kind: k, Pos: p, Text: text})
}

func (lx *legacyLexer) newlinesSignificant() bool {
	if lx.inExpr {
		return false
	}
	if n := len(lx.stack); n > 0 && lx.stack[n-1] != '{' {
		return false
	}
	return true
}

// skipSpace skips blanks and comments and reports whether a newline was seen.
func (lx *legacyLexer) skipSpace() bool {
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

func legacyIsLetter(c byte) bool { return c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
func legacyIsDigit(c byte) bool  { return '0' <= c && c <= '9' }
func legacyIsHex(c byte) bool {
	return legacyIsDigit(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// identChar returns the length in bytes of the identifier character at
// off, or 0 if there is none. Identifiers can contain non-ASCII letters
// and digits; first excludes digits.
func (lx *legacyLexer) identChar(off int, first bool) int {
	if off >= len(lx.src) {
		return 0
	}
	if c := lx.src[off]; c < utf8.RuneSelf {
		if legacyIsLetter(c) || !first && legacyIsDigit(c) {
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
func (lx *legacyLexer) scanIdent() {
	for n := lx.identChar(lx.off, false); n > 0; n = lx.identChar(lx.off, false) {
		lx.off += n
		lx.col++
	}
}

func (lx *legacyLexer) run() {
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

func (lx *legacyLexer) next() {
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
				} else if lx.peekByte(0) == '@' && lx.peekByte(1) == '`' {
					// return@`yield`
					end := strings.IndexAny(lx.src[lx.off+2:], "`\n")
					if end > 0 && lx.src[lx.off+2+end] == '`' {
						name := lx.src[lx.off+2 : lx.off+2+end]
						lx.advance(end + 3)
						word += "@" + name
					}
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
		lx.toks = append(lx.toks, Token{Kind: IDENT, Pos: p, Text: name, Quoted: true})
		return
	case legacyIsDigit(c) || c == '.' && legacyIsDigit(lx.peekByte(1)):
		lx.number(p) // .5 is a Double
		return
	case c == '"':
		lx.str(p)
		return
	case c == '$' && lx.dollarPrefix() > 0:
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
	// Kotlin's legacyLexer reads ;; as one token, DOUBLE_SEMICOLON, which
	// separates nothing: println(1);; is an error wherever it is.
	if strings.HasPrefix(lx.src[lx.off:], ";;") {
		lx.errorf(p, "expected newline or ';', found ';;'")
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

func (lx *legacyLexer) number(p Pos) {
	start := lx.off
	isDouble := false
	if lx.src[lx.off] == '0' && (lx.peekByte(1) == 'x' || lx.peekByte(1) == 'X' || lx.peekByte(1) == 'b' || lx.peekByte(1) == 'B') {
		lx.advance(2)
		for lx.off < len(lx.src) && (legacyIsHex(lx.src[lx.off]) || lx.src[lx.off] == '_') {
			lx.advance(1)
		}
	} else {
		lx.digits()
		// A '.' starts a fraction only if a digit follows; "1..2" is a range.
		if lx.peekByte(0) == '.' && legacyIsDigit(lx.peekByte(1)) {
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
func (lx *legacyLexer) char(p Pos) {
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

func (lx *legacyLexer) digits() {
	for lx.off < len(lx.src) && (legacyIsDigit(lx.src[lx.off]) || lx.src[lx.off] == '_') {
		lx.advance(1)
	}
}

// dollarPrefix returns the length of the interpolation prefix of the
// multi-dollar string literal at the legacyLexer's position ($$"...", $$"""..."""),
// or 0 if there is none.
func (lx *legacyLexer) dollarPrefix() int {
	n := lx.dollars(lx.off)
	if n == 0 || lx.peekByte(n) != '"' {
		return 0
	}
	return n
}

// dollars returns the length of the run of '$' at off.
func (lx *legacyLexer) dollars(off int) int {
	n := 0
	for off+n < len(lx.src) && lx.src[off+n] == '$' {
		n++
	}
	return n
}

// templateName returns the length in bytes of the identifier that a short
// template entry, $name or $`name`, has at off, or 0 if there is none.
func (lx *legacyLexer) templateName(off int) int {
	if lx.identChar(off, true) > 0 {
		n := 0
		for k := lx.identChar(off, true); k > 0; k = lx.identChar(off+n, false) {
			n += k
		}
		return n
	}
	if off < len(lx.src) && lx.src[off] == '`' {
		end := strings.IndexAny(lx.src[off+1:], "`\n")
		if end > 0 && lx.src[off+1+end] == '`' {
			return end + 2
		}
	}
	return 0
}

// str lexes a string literal, or a raw string literal ("""..."""), which
// has no escapes and can span lines. Either can have an interpolation
// prefix of n dollars ($$"..."; none is n = 1): a template entry is then n
// dollars followed by a name or {, and dollars before those are text.
func (lx *legacyLexer) str(p Pos) {
	prefix := max(1, lx.dollars(lx.off))
	lx.advance(lx.dollars(lx.off))
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
		if c == '$' {
			n := lx.dollars(lx.off)
			after := lx.off + n
			name := lx.templateName(after)
			if n < prefix || name == 0 && (after >= len(lx.src) || lx.src[after] != '{') {
				sb.WriteString(lx.src[lx.off:after])
				lx.advance(n)
				continue
			}
			sb.WriteString(lx.src[lx.off : after-prefix])
			lx.advance(n - prefix)
			flush()
			lx.advance(prefix)
			if name > 0 {
				sp := lx.pos()
				start := lx.off
				// Columns count runes, as in scanIdent.
				for lx.off < start+name {
					_, size := utf8.DecodeRuneInString(lx.src[lx.off:])
					lx.off += size
					lx.col++
				}
				parts = append(parts, StrPart{IsExpr: true, Src: lx.src[start:lx.off], SrcPos: sp})
				continue
			}
			lx.advance(1)
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
func (lx *legacyLexer) skipTemplate() bool {
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
		case '"', '\'', '$':
			if lx.src[lx.off] == '$' && lx.dollarPrefix() == 0 {
				break
			}
			// Lex the literal and drop its token; the template's source is
			// lexed again when it is parsed.
			n := len(lx.toks)
			if lx.src[lx.off] != '\'' {
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

var keywords = map[string]Kind{
	"fun": FUN, "val": VAL, "var": VAR, "if": IF, "else": ELSE, "when": WHEN, "while": WHILE,
	"for": FOR, "in": IN, "return": RETURN, "break": BREAK, "continue": CONTINUE, "class": CLASS,
	"this": THIS, "true": TRUE, "false": FALSE, "null": NULL,
	"is": IS, "as": AS, "try": TRY, "throw": THROW, "interface": INTERFACE, "object": OBJECT,
	"package": PACKAGE, "typealias": TYPEALIAS, "super": SUPER, "do": DO, "typeof": TYPEOF,
}
