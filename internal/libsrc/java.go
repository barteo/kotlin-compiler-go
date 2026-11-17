package libsrc

import (
	"regexp"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// locateJava finds t in the Java files of an archive, by text: the file
// of its outermost class, by its binary name or the longest prefix of its
// qualified name that names one, then each class's declaration, then a
// member's, the first whose parameter count matches.
func locateJava(archive string, idx *index, t Target) (Location, bool) {
	if len(idx.java) == 0 {
		return Location{}, false
	}
	parts := strings.Split(t.Name, ".")
	entry, path := "", []string(nil)
	if b := t.Binary; b != "" {
		outer, _, _ := strings.Cut(b, "$")
		if e, ok := idx.java[outer]; ok {
			entry = e
			// The classes, then a member's name.
			names := strings.Split(b[strings.LastIndexByte(outer, '/')+1:], "$")
			if t.Kind != "class" && t.Kind != "constructor" {
				names = append(names, parts[len(parts)-1])
			}
			path = names
		}
	}
	if entry == "" {
		for i := len(parts); i > 0; i-- {
			if e, ok := idx.java[strings.Join(parts[:i], "/")]; ok {
				entry, path = e, parts[i-1:]
				break
			}
		}
	}
	if entry == "" {
		return Location{}, false
	}
	text, err := Read(archive, entry)
	if err != nil {
		return Location{}, false
	}
	classes, member := path, ""
	if t.Kind != "class" && t.Kind != "constructor" && len(path) > 1 {
		classes, member = path[:len(path)-1], path[len(path)-1]
	}
	at := 0
	var r psi.Range
	for _, c := range classes {
		re := regexp.MustCompile(`\b(?:class|interface|enum|record|@interface)\s+(` + regexp.QuoteMeta(c) + `)\b`)
		m := re.FindStringSubmatchIndex(text[at:])
		if m == nil {
			return Location{}, false
		}
		r = psi.Range{Start: at + m[2], End: at + m[3]}
		at = r.End
	}
	if member != "" {
		if mr, ok := javaMember(text, at, member, t); ok {
			r = mr
		}
	}
	return Location{Archive: archive, Entry: entry, Text: text, Range: r}, true
}

// javaMember finds a method or field named name after offset at: a line
// that declares it, with the parameter count t's, if known, else the
// first.
func javaMember(text string, at int, name string, t Target) (psi.Range, bool) {
	q := regexp.QuoteMeta(name)
	re := regexp.MustCompile(`(?m)^[ \t]+(?:[\w<>\[\],.?@ ]+[ \t])?(` + q + `)\s*(\(|=|;)`)
	var first psi.Range
	found := false
	for _, m := range re.FindAllStringSubmatchIndex(text[at:], -1) {
		line := text[at+m[0] : at+m[3]]
		if strings.Contains(line, "return ") || strings.Contains(line, "new ") || strings.Contains(line, "=") {
			continue
		}
		r := psi.Range{Start: at + m[2], End: at + m[3]}
		if !found {
			first, found = r, true
		}
		if !t.Params || text[at+m[4]] != '(' {
			return r, true
		}
		if javaParams(text[at+m[5]:]) == len(t.Parameters) {
			return r, true
		}
	}
	return first, found
}

// javaParams counts the parameters of a parameter list that text starts
// after the "(" of.
func javaParams(text string) int {
	depth, n, any := 0, 0, false
	for i := 0; i < len(text); i++ {
		switch c := text[i]; c {
		case '(', '<':
			depth++
		case '>':
			depth--
		case ')':
			if depth == 0 {
				if any {
					n++
				}
				return n
			}
			depth--
		case ',':
			if depth == 0 {
				n++
			}
		default:
			if c > ' ' {
				any = true
			}
		}
	}
	return n
}
