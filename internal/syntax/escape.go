package syntax

import (
	"strings"
	"unicode/utf8"
)

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
