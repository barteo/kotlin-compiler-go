package javasrc

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tNumber
	tString // a string literal or a text block, its text decoded
	tChar   // a character literal, its text decoded
	tPunct  // one character of punctuation or an operator, or "..."
)

type token struct {
	kind tokKind
	text string
	line int
	col  int
}

// scan splits Java source into tokens, without comments. Operators are
// single characters, but for "...": the parser only needs to balance
// brackets and find declarations, and Map<K, List<V>> then ends in two
// '>' tokens.
func scan(src string) ([]token, error) {
	var toks []token
	line, lineStart := 1, 0
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '\n':
			i++
			line, lineStart = line+1, i
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
			continue
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("%d:%d: unterminated comment", line, i-lineStart+1)
			}
			text := src[i : i+2+end+2]
			if n := strings.Count(text, "\n"); n > 0 {
				line += n
				lineStart = i + strings.LastIndexByte(text, '\n') + 1
			}
			i += len(text)
			continue
		}
		tok := token{line: line, col: i - lineStart + 1}
		start := i
		r, size := utf8.DecodeRuneInString(src[i:])
		switch {
		case r == '_' || r == '$' || unicode.IsLetter(r):
			for i < len(src) {
				r, size := utf8.DecodeRuneInString(src[i:])
				if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					break
				}
				i += size
			}
			tok.kind, tok.text = tIdent, src[start:i]
		case '0' <= c && c <= '9' || c == '.' && i+1 < len(src) && '0' <= src[i+1] && src[i+1] <= '9':
			hex := c == '0' && i+1 < len(src) && (src[i+1] == 'x' || src[i+1] == 'X')
			for i < len(src) {
				d := src[i]
				if d == '.' || d == '_' || '0' <= d && d <= '9' || 'a' <= d && d <= 'z' || 'A' <= d && d <= 'Z' {
					i++
					continue
				}
				// An exponent's sign: 1e-5 and 0x1p-3, but not 0x1E-5.
				if p := src[i-1]; (d == '+' || d == '-') && (!hex && (p == 'e' || p == 'E') || hex && (p == 'p' || p == 'P')) {
					i++
					continue
				}
				break
			}
			tok.kind, tok.text = tNumber, src[start:i]
		case strings.HasPrefix(src[i:], `"""`):
			end := strings.Index(src[i+3:], `"""`)
			for end >= 0 && escaped(src[i+3:i+3+end]) {
				next := strings.Index(src[i+3+end+1:], `"""`)
				if next < 0 {
					end = -1
					break
				}
				end += 1 + next
			}
			if end < 0 {
				return nil, fmt.Errorf("%d:%d: unterminated text block", line, tok.col)
			}
			text := src[i+3 : i+3+end]
			i += 3 + end + 3
			if n := strings.Count(text, "\n"); n > 0 {
				line += n
				lineStart = start + 3 + strings.LastIndexByte(text, '\n') + 1
			}
			units, err := unescape(textBlock(text))
			if err != nil {
				return nil, fmt.Errorf("%d:%d: %v", tok.line, tok.col, err)
			}
			tok.kind, tok.text = tString, string(utf16.Decode(units))
		case c == '"' || c == '\'':
			i++
			for i < len(src) && src[i] != c && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			if i >= len(src) || src[i] != c {
				return nil, fmt.Errorf("%d:%d: unterminated literal", line, tok.col)
			}
			units, err := unescape(src[start+1 : i])
			if err != nil {
				return nil, fmt.Errorf("%d:%d: %v", line, tok.col, err)
			}
			i++
			tok.kind, tok.text = tString, string(utf16.Decode(units))
			if c == '\'' {
				// A character's text is its UTF-16 code unit, in decimal:
				// it may be half of a surrogate pair.
				if len(units) != 1 {
					return nil, fmt.Errorf("%d:%d: bad character literal", line, tok.col)
				}
				tok.kind, tok.text = tChar, strconv.Itoa(int(units[0]))
			}
		case strings.HasPrefix(src[i:], "..."):
			i += 3
			tok.kind, tok.text = tPunct, "..."
		default:
			i += size
			tok.kind, tok.text = tPunct, src[start:i]
		}
		toks = append(toks, tok)
	}
	toks = append(toks, token{kind: tEOF, line: line, col: i - lineStart + 1})
	return toks, nil
}

// escaped reports whether a text block's text ends in an odd number of
// backslashes, so that the quote after it is part of it.
func escaped(s string) bool {
	n := 0
	for n < len(s) && s[len(s)-1-n] == '\\' {
		n++
	}
	return n%2 == 1
}

// textBlock strips a text block's first line and the indentation its
// lines share, as javac does, but for escapes.
func textBlock(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	lines := strings.Split(s, "\n")
	indent := -1
	for i, l := range lines {
		blank := strings.TrimLeft(l, " \t") == ""
		if blank && i != len(lines)-1 {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		if len(l) >= indent {
			l = l[indent:]
		} else {
			l = ""
		}
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

// unescape decodes the escapes of a string or character literal, into
// UTF-16 code units, as \uD83D\uDE00 are two.
func unescape(s string) ([]uint16, error) {
	var units []uint16
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			r, size := utf8.DecodeRuneInString(s[i:])
			units = utf16.AppendRune(units, r)
			i += size - 1
			continue
		}
		i++
		if i >= len(s) {
			return nil, fmt.Errorf("bad escape")
		}
		switch c := s[i]; c {
		case 'n':
			units = append(units, '\n')
		case 't':
			units = append(units, '\t')
		case 'b':
			units = append(units, '\b')
		case 'r':
			units = append(units, '\r')
		case 'f':
			units = append(units, '\f')
		case 's':
			units = append(units, ' ')
		case '\n':
			// A text block's line continuation.
		case '"', '\'', '\\':
			units = append(units, uint16(c))
		case 'u':
			for i < len(s) && s[i] == 'u' {
				i++
			}
			if i+4 > len(s) {
				return nil, fmt.Errorf("bad unicode escape")
			}
			n, err := strconv.ParseUint(s[i:i+4], 16, 16)
			if err != nil {
				return nil, fmt.Errorf("bad unicode escape")
			}
			units = append(units, uint16(n))
			i += 3
		default:
			if c < '0' || c > '7' {
				return nil, fmt.Errorf("bad escape \\%c", c)
			}
			// Octal: up to three digits, at most \377.
			j := i
			for j < len(s) && j < i+3 && '0' <= s[j] && s[j] <= '7' {
				j++
			}
			if j == i+3 && c > '3' {
				j--
			}
			n, _ := strconv.ParseUint(s[i:j], 8, 8)
			units = append(units, uint16(n))
			i = j - 1
		}
	}
	return units, nil
}
