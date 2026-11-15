package syntax

import (
	"strings"
	"unicode/utf8"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// The lexer is psi's, a port of Kotlin's own (psi.Lex). This file turns
// its lossless token stream into the parser's tokens: whitespace and
// comments are dropped, and an NL token stands for the newlines between
// two tokens where they matter; a string literal is one STRING token with
// its parts decoded; and labels, safe calls and elvis operators are one
// token each.

// lexer translates psi.Lex's tokens.
type lexer struct {
	src    string
	file   string
	kinds  []psi.Kind
	starts []int32
	i      int // the next psi token
	// cur is the offset that line and col are the position of.
	cur, line, col int
	stack          []byte // open brackets: '(', '[', '{'
	inExpr         bool   // lexing a ${...} template expression: newlines never matter
	toks           []Token
	errs           []*Error
	// discard is set while skipping a ${...} template expression inside a
	// string: its tokens are dropped, and only errors in its string and
	// character literals are kept, as when the expression is lexed again
	// to be parsed.
	discard bool
	// sawNL is set when the whitespace since the last token had a newline;
	// gapPos is where that whitespace started.
	sawNL, inGap bool
	gapPos       Pos
	// bytes is set while columns count bytes: in a template expression
	// being skipped, outside its literals, as kc's lexer counted them.
	bytes bool
}

// Tokenize lexes a whole file.
func Tokenize(file, src string) ([]Token, []*Error) {
	lx := lexer{src: src, file: file, line: 1, col: 1}
	return lx.run()
}

func tokenizeAt(file, src string, pos Pos) ([]Token, []*Error) {
	lx := lexer{src: src, file: file, line: pos.Line, col: pos.Col, inExpr: true}
	return lx.run()
}

func (lx *lexer) run() ([]Token, []*Error) {
	lx.kinds, lx.starts = psi.Lex(lx.src, make([]psi.Kind, 0, len(lx.src)/3), make([]int32, 0, len(lx.src)/3+1))
	lx.toks = make([]Token, 0, len(lx.kinds)/2+1)
	lx.tokens(false)
	lx.newline()
	lx.toks = append(lx.toks, Token{Kind: EOF, Pos: lx.at(len(lx.src))})
	return lx.toks, lx.errs
}

func (lx *lexer) errorf(p Pos, msg string) {
	lx.errs = append(lx.errs, &Error{File: lx.file, Pos: p, Msg: msg})
}

// codeError reports an error outside string and character literals.
func (lx *lexer) codeError(p Pos, msg string) {
	if !lx.discard {
		lx.errorf(p, msg)
	}
}

// at returns the position of offset off, which is at or after lx.cur.
// Columns count runes.
func (lx *lexer) at(off int) Pos {
	if lx.bytes {
		lx.atBytes(off)
		return Pos{lx.line, lx.col}
	}
	for lx.cur < off {
		c := lx.src[lx.cur]
		switch {
		case c == '\n':
			lx.line++
			lx.col = 1
			lx.cur++
		case c < utf8.RuneSelf:
			lx.col++
			lx.cur++
		default:
			_, n := utf8.DecodeRuneInString(lx.src[lx.cur:])
			lx.cur += n
			lx.col++
		}
	}
	return Pos{lx.line, lx.col}
}

// atRunes moves to off counting runes, whatever lx.bytes says.
func (lx *lexer) atRunes(off int) {
	b := lx.bytes
	lx.bytes = false
	lx.at(off)
	lx.bytes = b
}

// atBytes is at for text whose columns count bytes: comments and quoted
// names, as kc's lexer has always counted them.
func (lx *lexer) atBytes(off int) {
	for ; lx.cur < off; lx.cur++ {
		if lx.src[lx.cur] == '\n' {
			lx.line++
			lx.col = 1
		} else {
			lx.col++
		}
	}
}

func (lx *lexer) emit(k Kind, p Pos, text string) {
	lx.toks = append(lx.toks, Token{Kind: k, Pos: p, Text: text})
}

// span returns the start and end of psi token i.
func (lx *lexer) span(i int) (int, int) { return int(lx.starts[i]), int(lx.starts[i+1]) }

// next reports whether psi token i exists, has kind k and starts at off.
func (lx *lexer) next(i int, k psi.Kind, off int) bool {
	return i < len(lx.kinds) && lx.kinds[i] == k && int(lx.starts[i]) == off
}

// gap records whitespace or a comment at p; nl is whether it holds a
// newline that separates tokens.
func (lx *lexer) gap(p Pos, nl bool) {
	if !lx.inGap {
		lx.inGap = true
		lx.gapPos = p
	}
	lx.sawNL = lx.sawNL || nl
}

// newline ends a gap, emitting an NL token for it if it had a newline
// that matters.
func (lx *lexer) newline() {
	if lx.sawNL && !lx.inExpr && !lx.discard {
		if n := len(lx.stack); n == 0 || lx.stack[n-1] == '{' {
			if n := len(lx.toks); n > 0 && lx.toks[n-1].Kind != NL {
				lx.emit(NL, lx.gapPos, "")
			}
		}
	}
	lx.sawNL, lx.inGap = false, false
}

// tokens translates psi tokens up to the end of input or, with inTemplate,
// up to the LONG_TEMPLATE_ENTRY_END that closes the template expression
// being skipped, which it doesn't consume. It reports whether it found it.
func (lx *lexer) tokens(inTemplate bool) bool {
	for lx.i < len(lx.kinds) {
		k := lx.kinds[lx.i]
		s, e := lx.span(lx.i)
		p := lx.at(s)
		switch k {
		case psi.KindWHITE_SPACE:
			lx.gap(p, strings.IndexByte(lx.src[s:e], '\n') >= 0)
			lx.i++
			continue
		case psi.KindEOL_COMMENT:
			lx.gap(p, false)
			lx.atBytes(e)
			lx.i++
			continue
		case psi.KindBLOCK_COMMENT, psi.KindKDOC:
			lx.gap(p, false)
			if !commentClosed(lx.src[s:e]) {
				lx.codeError(p, "unterminated comment")
			}
			lx.atBytes(e)
			lx.i++
			continue
		case psi.KindBAD_CHARACTER:
			if lx.src[s] == '\r' {
				lx.gap(p, false)
				lx.i++
				continue
			}
		case psi.KindLONG_TEMPLATE_ENTRY_END:
			if inTemplate {
				return true
			}
		}
		lx.newline()
		lx.i++
		if t := tokKinds[k]; t != 0 {
			lx.emit(t, p, lx.src[s:e])
			switch t {
			case LPAREN, LBRACK, LBRACE:
				lx.stack = append(lx.stack, lx.src[s])
			case RPAREN, RBRACK, RBRACE:
				if n := len(lx.stack); n > 0 {
					lx.stack = lx.stack[:n-1]
				}
			case RETURN, BREAK, CONTINUE, THIS, SUPER:
				lx.label(s, e)
			}
			continue
		}
		switch k {
		case psi.KindIDENTIFIER:
			switch {
			case lx.src[s] == '`':
				lx.toks = append(lx.toks, Token{Kind: IDENT, Pos: p, Text: lx.src[s+1 : e-1], Quoted: true})
				lx.atBytes(e)
			case lx.next(lx.i, psi.KindAT, e):
				// A label: loop@ for (...), lit@{ ... }.
				lx.i++
				lx.emit(LABEL, p, lx.src[s:e])
			default:
				lx.emit(IDENT, p, lx.src[s:e])
			}
		case psi.KindINTEGER_LITERAL:
			k := INT
			if c := lx.src[e-1]; (c == 'u' || c == 'U') || (c == 'L' || c == 'l') && e-s > 1 && (lx.src[e-2] == 'u' || lx.src[e-2] == 'U') {
				k = UNSIGNED
			}
			lx.emit(k, p, lx.src[s:e])
		case psi.KindFLOAT_LITERAL:
			k := DOUBLE
			if c := lx.src[e-1]; c == 'f' || c == 'F' {
				k = FLOAT
			} else if lx.next(lx.i, psi.KindIDENTIFIER, e) && lx.src[e] == 'L' {
				lx.codeError(p, "invalid Long literal")
			}
			lx.emit(k, p, lx.src[s:e])
		case psi.KindCHARACTER_LITERAL:
			if !charClosed(lx.src[s:e]) {
				lx.errorf(p, "unterminated character literal")
			}
			lx.emit(CHAR, p, lx.src[s:e])
			lx.atRunes(e)
		case psi.KindOPEN_QUOTE, psi.KindINTERPOLATION_PREFIX:
			lx.str(p)
		case psi.KindQUEST:
			lx.quest(p, e)
		case psi.KindEXCL:
			lx.excl(p, s, e)
		case psi.KindDOUBLE_SEMICOLON:
			// Kotlin's lexer reads ;; as one token, which separates
			// nothing: println(1);; is an error wherever it is.
			lx.codeError(p, "expected newline or ';', found ';;'")
			lx.emit(SEMI, p, ";")
			lx.emit(SEMI, lx.at(s+1), ";")
		case psi.KindRESERVED: // ...
			lx.emit(RANGE, p, "..")
			lx.emit(DOT, lx.at(s+2), ".")
		case psi.KindDOUBLE_ARROW:
			lx.emit(ASSIGN, p, "=")
			lx.emit(GT, lx.at(s+1), ">")
		case psi.KindFIELD_IDENTIFIER: // $name outside a string
			lx.codeError(p, "unexpected character $")
			lx.i--
			lx.starts[lx.i]++
			lx.kinds[lx.i] = psi.KindIDENTIFIER
		case psi.KindBAD_CHARACTER:
			if lx.src[s] == '`' {
				// kc's lexer took the rest of the line as the name, or
				// nothing in ``.
				lx.codeError(p, "unterminated backtick identifier")
				end := strings.IndexAny(lx.src[s+1:], "`\n")
				if end < 0 {
					end = len(lx.src)
				} else {
					end += s + 1
				}
				for !lx.discard && lx.i < len(lx.kinds) && int(lx.starts[lx.i]) <= end {
					lx.i++
				}
			} else {
				r, _ := utf8.DecodeRuneInString(lx.src[s:])
				lx.codeError(p, "unexpected character "+string(r))
			}
		default: // HASH, SHEBANG_COMMENT
			lx.codeError(p, "unexpected character "+lx.src[s:s+1])
		}
	}
	return false
}

// label joins a label to the keyword before it, which spans [s, e):
// return@forEach, this@Outer, return@`yield`.
func (lx *lexer) label(s, e int) {
	if !lx.next(lx.i, psi.KindAT, e) || lx.i+1 >= len(lx.kinds) {
		return
	}
	ns, ne := lx.span(lx.i + 1)
	if ns != e+1 {
		return
	}
	t := &lx.toks[len(lx.toks)-1]
	switch k := lx.kinds[lx.i+1]; {
	case k == psi.KindIDENTIFIER && lx.src[ns] == '`':
		t.Text = lx.src[s:e+1] + lx.src[ns+1:ne-1]
	case k == psi.KindAS_SAFE:
		// this@as?: the label is as.
		t.Text = lx.src[s : ne-1]
		lx.i += 2
		lx.emit(QUEST, lx.at(ne-1), "?")
		return
	case k == psi.KindIDENTIFIER || tokKinds[k] != 0 && isWord(lx.src[ns]):
		t.Text = lx.src[s:ne]
	default:
		return
	}
	lx.i += 2
	if lx.src[ns] == '`' {
		lx.atBytes(ne)
	}
}

func isWord(c byte) bool { return 'a' <= c && c <= 'z' }

// quest translates a '?' that ends at e, which Kotlin's lexer leaves apart
// from a '.' or ':' after it: ?. and ?: are tokens of their own, but
// String?::plus is ? and ::, and ?.. is ?. and ., as kc has always read them.
func (lx *lexer) quest(p Pos, e int) {
	if lx.i < len(lx.kinds) && int(lx.starts[lx.i]) == e {
		_, ne := lx.span(lx.i)
		switch lx.kinds[lx.i] {
		case psi.KindDOT:
			lx.i++
			lx.emit(SAFE_DOT, p, "?.")
			return
		case psi.KindCOLON:
			lx.i++
			lx.emit(ELVIS, p, "?:")
			return
		case psi.KindRANGE, psi.KindRANGE_UNTIL, psi.KindRESERVED:
			// ?.. is ?. and ., ?..< is ?., . and <, and ?... is ?. and ..
			lx.i++
			lx.emit(SAFE_DOT, p, "?.")
			switch rest := lx.src[e+1 : ne]; rest {
			case ".", "..":
				lx.emit(tokKinds[map[string]psi.Kind{".": psi.KindDOT, "..": psi.KindRANGE}[rest]], lx.at(e+1), rest)
			default:
				lx.emit(DOT, lx.at(e+1), ".")
				lx.emit(LT, lx.at(e+2), "<")
			}
			return
		}
	}
	lx.emit(QUEST, p, "?")
}

// excl translates a '!' at [s, e). Kotlin's lexer leaves !! as two
// tokens, and !!= as ! and !=, where kc reads !! and =.
func (lx *lexer) excl(p Pos, s, e int) {
	if lx.i >= len(lx.kinds) || int(lx.starts[lx.i]) != e || lx.src[e] != '!' {
		lx.emit(NOT, p, "!")
		return
	}
	lx.emit(NOT_NULL, p, "!!")
	switch lx.kinds[lx.i] {
	case psi.KindEXCL:
		lx.i++
		return
	case psi.KindEXCLEQ:
		lx.kinds[lx.i] = psi.KindEQ
	case psi.KindEXCLEQEQ:
		lx.kinds[lx.i] = psi.KindEQEQ
	case psi.KindNOT_IN:
		lx.kinds[lx.i] = psi.KindIN_KEYWORD
	case psi.KindNOT_IS:
		lx.kinds[lx.i] = psi.KindIS_KEYWORD
	}
	lx.starts[lx.i]++
}

// str translates a string literal at p, whose OPEN_QUOTE or
// INTERPOLATION_PREFIX was the last psi token read.
func (lx *lexer) str(p Pos) {
	bytes := lx.bytes
	lx.bytes = false
	defer func() { lx.bytes = bytes }()
	// The lexer has applied an interpolation prefix ($$"...") already.
	if lx.kinds[lx.i-1] == psi.KindINTERPOLATION_PREFIX {
		lx.i++
	}
	var parts []StrPart
	var sb strings.Builder
	flush := func() {
		if sb.Len() > 0 {
			parts = append(parts, StrPart{Lit: sb.String()})
			sb.Reset()
		}
	}
loop:
	for {
		if lx.i >= len(lx.kinds) {
			lx.errorf(p, "unterminated string literal")
			break
		}
		k := lx.kinds[lx.i]
		s, e := lx.span(lx.i)
		lx.i++
		switch k {
		case psi.KindREGULAR_STRING_PART:
			sb.WriteString(lx.src[s:e])
		case psi.KindCLOSING_QUOTE:
			break loop
		case psi.KindDANGLING_NEWLINE:
			lx.errorf(p, "unterminated string literal")
			lx.gap(lx.at(s), true)
			break loop
		case psi.KindESCAPE_SEQUENCE:
			seq := lx.src[s:e]
			if r, ok := EscapeValue(seq); ok {
				sb.WriteRune(r)
			} else if seq == `\u` {
				lx.errorf(lx.at(s), "invalid unicode escape")
			} else {
				lx.errorf(lx.at(s), "illegal escape: \\"+string(rune(seq[1])))
			}
		case psi.KindBAD_CHARACTER: // a backslash at the end of a line or input
			lx.errorf(lx.at(s), "illegal escape: \\"+string(rune((lx.src[s+1:] + "\x00")[0])))
		case psi.KindSHORT_TEMPLATE_ENTRY_START:
			flush()
			ns, ne := lx.span(lx.i)
			lx.i++
			parts = append(parts, StrPart{IsExpr: true, Src: lx.src[ns:ne], SrcPos: lx.at(ns)})
		case psi.KindLONG_TEMPLATE_ENTRY_START:
			flush()
			sp := lx.at(e)
			if !lx.skipTemplate() {
				lx.errorf(sp, "unterminated template expression")
				break loop
			}
			end, _ := lx.span(lx.i)
			lx.i++
			parts = append(parts, StrPart{IsExpr: true, Src: lx.src[e:end], SrcPos: sp})
		default:
			lx.errorf(lx.at(s), "unexpected token in string literal")
		}
	}
	flush()
	if k := lx.kinds[lx.i-1]; k == psi.KindCLOSING_QUOTE {
		lx.at(int(lx.starts[lx.i]))
	}
	lx.toks = append(lx.toks, Token{Kind: STRING, Pos: p, Parts: parts})
}

// skipTemplate moves to the LONG_TEMPLATE_ENTRY_END that ends a ${...}
// template expression and reports whether there is one. The template's
// source is lexed again when it is parsed.
func (lx *lexer) skipTemplate() bool {
	n, depth := len(lx.toks), len(lx.stack)
	discard, sawNL, inGap, gapPos, bytes := lx.discard, lx.sawNL, lx.inGap, lx.gapPos, lx.bytes
	lx.discard, lx.sawNL, lx.inGap, lx.bytes = true, false, false, true
	found := lx.tokens(true)
	if found {
		lx.at(int(lx.starts[lx.i]))
	}
	lx.toks, lx.stack = lx.toks[:n], lx.stack[:depth]
	lx.discard, lx.sawNL, lx.inGap, lx.gapPos, lx.bytes = discard, sawNL, inGap, gapPos, bytes
	return found
}

// commentClosed reports whether a block comment, which nests, is closed.
func commentClosed(c string) bool {
	depth := 1
	for i := 2; i < len(c); i++ {
		switch {
		case c[i] == '/' && i+1 < len(c) && c[i+1] == '*':
			depth++
			i++
		case c[i] == '*' && i+1 < len(c) && c[i+1] == '/':
			depth--
			i++
			if depth == 0 {
				return i == len(c)-1
			}
		}
	}
	return false
}

// charClosed reports whether a character literal is one character or
// escape between quotes, or none. Kotlin's lexer takes everything up to
// the closing quote, and more than one character is left to the parser.
func charClosed(c string) bool {
	body := c[1:]
	switch {
	case body == "":
		return false
	case body == "'":
		return true
	case strings.HasPrefix(body, `\u`):
		return len(body) == 7 && body[6] == '\''
	case body[0] == '\\':
		return len(body) == 3 && body[2] == '\''
	}
	_, n := utf8.DecodeRuneInString(body)
	return len(body) == n+1 && body[n] == '\'' && body[0] != '\n' && body[0] != 0
}

// tokKinds are the parser's kinds of the psi tokens that translate one to
// one, keeping their text.
var tokKinds = func() (t [512]Kind) {
	for k, v := range map[psi.Kind]Kind{
		psi.KindPACKAGE_KEYWORD: PACKAGE, psi.KindAS_KEYWORD: AS, psi.KindTYPE_ALIAS_KEYWORD: TYPEALIAS,
		psi.KindCLASS_KEYWORD: CLASS, psi.KindTHIS_KEYWORD: THIS, psi.KindSUPER_KEYWORD: SUPER,
		psi.KindVAL_KEYWORD: VAL, psi.KindVAR_KEYWORD: VAR, psi.KindFUN_KEYWORD: FUN, psi.KindFOR_KEYWORD: FOR,
		psi.KindNULL_KEYWORD: NULL, psi.KindTRUE_KEYWORD: TRUE, psi.KindFALSE_KEYWORD: FALSE,
		psi.KindIS_KEYWORD: IS, psi.KindIN_KEYWORD: IN, psi.KindTHROW_KEYWORD: THROW,
		psi.KindRETURN_KEYWORD: RETURN, psi.KindBREAK_KEYWORD: BREAK, psi.KindCONTINUE_KEYWORD: CONTINUE,
		psi.KindOBJECT_KEYWORD: OBJECT, psi.KindIF_KEYWORD: IF, psi.KindTRY_KEYWORD: TRY,
		psi.KindELSE_KEYWORD: ELSE, psi.KindWHILE_KEYWORD: WHILE, psi.KindDO_KEYWORD: DO,
		psi.KindWHEN_KEYWORD: WHEN, psi.KindINTERFACE_KEYWORD: INTERFACE, psi.KindTYPEOF_KEYWORD: TYPEOF,

		psi.KindLBRACKET: LBRACK, psi.KindRBRACKET: RBRACK, psi.KindLBRACE: LBRACE, psi.KindRBRACE: RBRACE,
		psi.KindLPAR: LPAREN, psi.KindRPAR: RPAREN, psi.KindDOT: DOT, psi.KindMUL: STAR, psi.KindPLUS: PLUS,
		psi.KindMINUS: MINUS, psi.KindDIV: SLASH, psi.KindPERC: PERCENT, psi.KindLT: LT,
		psi.KindGT: GT, psi.KindCOLON: COLON, psi.KindSEMICOLON: SEMI, psi.KindEQ: ASSIGN,
		psi.KindCOMMA: COMMA, psi.KindAT: AT, psi.KindAND: AMP,
		psi.KindEQEQEQ: EQ_EQ_EQ, psi.KindEXCLEQEQ: NOT_EQ_EQ, psi.KindNOT_IN: NOT_IN, psi.KindNOT_IS: NOT_IS,
		psi.KindAS_SAFE: AS_SAFE, psi.KindRANGE_UNTIL: RANGE_LT, psi.KindPLUSPLUS: INC, psi.KindMINUSMINUS: DEC,
		psi.KindLTEQ: LE, psi.KindGTEQ: GE, psi.KindEQEQ: EQ, psi.KindEXCLEQ: NE, psi.KindANDAND: AND,
		psi.KindOROR: OR, psi.KindMULTEQ: STAR_EQ, psi.KindDIVEQ: SLASH_EQ, psi.KindPERCEQ: PERCENT_EQ,
		psi.KindPLUSEQ: PLUS_EQ, psi.KindMINUSEQ: MINUS_EQ, psi.KindARROW: ARROW, psi.KindRANGE: RANGE,
		psi.KindCOLONCOLON: COLON_COLON,
	} {
		t[k] = v
	}
	return
}()
