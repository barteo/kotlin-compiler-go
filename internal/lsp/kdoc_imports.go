package lsp

import (
	"regexp"
	"strings"
)

const docIdentifier = "(?:`[^`\\r\\n]+`|[\\pL_][\\pL\\pM\\pN_]*)"

var docPath = regexp.MustCompile("^" + docIdentifier + "(?:\\." + docIdentifier + ")*$")

// KDoc is a single PSI token. Read Kotlin link destinations, including the
// second bracket of [caption][destination]. Backtick spans and fenced code
// are examples, and the caption of an ordinary Markdown URL isn't a symbol.
func kdocImportNames(doc string, used map[string]bool) {
	record := func(path string) {
		if docPath.MatchString(path) {
			first, _, _ := strings.Cut(path, ".")
			used[strings.Trim(first, "`")] = true
		}
	}
	for len(doc) != 0 {
		switch doc[0] {
		case '\\':
			doc = doc[min(2, len(doc)):]
		case '`', '~':
			i := 1
			for i < len(doc) && doc[i] == doc[0] {
				i++
			}
			marker := doc[:i]
			doc = doc[i:]
			if marker[0] == '~' && i < 3 {
				continue
			}
			// An inline span ends at a run of exactly the opening width.
			// This allows a literal backtick inside a double-backtick span.
			for tail := doc; ; {
				at := strings.IndexByte(tail, marker[0])
				if at < 0 {
					break
				}
				run := tail[at:]
				width := 1
				for width < len(run) && run[width] == marker[0] {
					width++
				}
				tail = run[width:]
				if width == i || i >= 3 && width > i {
					doc = tail
					break
				}
			}
		case '[':
			target, rest, ok := strings.Cut(doc[1:], "]")
			if !ok {
				return
			}
			doc = rest
			if strings.HasPrefix(doc, "(") {
				if _, rest, ok := strings.Cut(doc, ")"); ok {
					doc = rest
				}
				continue
			}
			if strings.HasPrefix(doc, "[") {
				target, rest, ok = strings.Cut(doc[1:], "]")
				if !ok {
					return
				}
				doc = rest
			}
			record(target)
		case '@':
			tag, rest, ok := strings.Cut(doc, " ")
			doc = doc[1:]
			if ok && (tag == "@see" || tag == "@throws" || tag == "@exception" || tag == "@sample") {
				if fields := strings.Fields(rest); len(fields) > 0 {
					record(fields[0])
				}
			}
		default:
			doc = doc[1:]
		}
	}
}
