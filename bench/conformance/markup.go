package main

import "strings"

// An expected diagnostic is one the markup of a test marks:
// <!NAME!>element<!>, or <!NAME1, NAME2("arguments")!>element<!> for
// several, as compiler/testData/diagnostics/ReadMe.md describes.
type expected struct {
	name string
	line int // of the element's start, counting from 1
}

// stripMarkup removes the diagnostic markup from src, which keeps its
// lines, and returns the diagnostics it marks.
func stripMarkup(src string) (string, []expected) {
	var sb strings.Builder
	var es []expected
	line := 1
	for i := 0; i < len(src); {
		if strings.HasPrefix(src[i:], "<!>") {
			i += 3
			continue
		}
		if strings.HasPrefix(src[i:], "<!") {
			if names, n := parseOpenTag(src[i:]); n > 0 {
				for _, name := range names {
					es = append(es, expected{name: name, line: line})
				}
				i += n
				continue
			}
		}
		if src[i] == '\n' {
			line++
		}
		sb.WriteByte(src[i])
		i++
	}
	return sb.String(), es
}

// parseOpenTag parses an opening tag at the start of s, <!NAME, ...!>,
// and returns its names and length, or a length of 0 if s doesn't start
// with one. A name can have arguments in parentheses, which can quote
// strings, and a {qualifier} after it.
func parseOpenTag(s string) ([]string, int) {
	i := 2
	var names []string
	for {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		start := i
		for i < len(s) && (s[i] >= 'A' && s[i] <= 'Z' || s[i] >= '0' && s[i] <= '9' || s[i] == '_') {
			i++
		}
		if i == start || s[start] < 'A' || s[start] > 'Z' {
			return nil, 0
		}
		names = append(names, s[start:i])
		if i < len(s) && s[i] == '{' {
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return nil, 0
			}
			i += end + 1
		}
		if i < len(s) && s[i] == '(' {
			end := closeParen(s[i:])
			if end < 0 {
				return nil, 0
			}
			i += end + 1
		}
		for i < len(s) && s[i] == ' ' {
			i++
		}
		switch {
		case strings.HasPrefix(s[i:], "!>"):
			return names, i + 2
		case i < len(s) && s[i] == ',':
			i++
		default:
			return nil, 0
		}
	}
}

// closeParen returns the index of the parenthesis that closes the one at
// the start of s, skipping quoted strings, or -1. It doesn't go past the
// line.
func closeParen(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		case '\n':
			return -1
		}
	}
	return -1
}
