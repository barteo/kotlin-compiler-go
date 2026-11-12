package psi

import (
	"strings"
	"unicode/utf8"
)

// IsHardKeyword reports whether s is a hard keyword, which a name can be
// written as only in backticks.
func IsHardKeyword(s string) bool {
	_, ok := hardKeywords[s]
	return ok
}

// IsIdentifier reports whether s is an identifier written without
// backticks: a letter or '_', then letters and digits, and not a hard
// keyword.
func IsIdentifier(s string) bool {
	if s == "" || IsHardKeyword(s) {
		return false
	}
	for i, r := range s {
		if r == utf8.RuneError || i == 0 && !isLetter(r) || !isIdentPart(r) {
			return false
		}
	}
	return true
}

// QuoteName returns name as Kotlin source: as is if it is an identifier,
// else in backticks. ok is false if no source names it: an empty name, or
// one with a backtick, a line break, or a character the JVM forbids in
// names (. ; [ ] / < > : \).
func QuoteName(name string) (string, bool) {
	if IsIdentifier(name) {
		return name, true
	}
	if name == "" || strings.ContainsAny(name, "`\r\n.;[]/<>:\\") || !utf8.ValidString(name) {
		return "", false
	}
	return "`" + name + "`", true
}

// Unquote returns a name without its backticks, if it has them.
func Unquote(s string) string { return unquote(s) }
