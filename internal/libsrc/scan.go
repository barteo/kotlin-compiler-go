package libsrc

import (
	"strings"
)

// scan returns a Kotlin file's package and the names it declares at the
// top level, from the lines that start a declaration at their first
// column: a quick scan, not a parse, that may miss a declaration whose
// keyword isn't on its first line, and finds no more than it declares.
func scan(text string) (pkg string, names []string) {
	for line := range strings.Lines(text) {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '/' || line[0] == '*' {
			continue
		}
		line = strings.TrimSpace(line)
		if p, ok := strings.CutPrefix(line, "package "); ok {
			pkg = strings.Trim(strings.TrimSpace(strings.TrimSuffix(p, ";")), "`")
			continue
		}
		if name := declName(line); name != "" {
			names = append(names, name)
		}
	}
	return pkg, names
}

// declName returns the name a line that starts a declaration declares,
// or "".
func declName(line string) string {
	rest := line
	for {
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "@") {
			rest = skipAnnotation(rest)
			continue
		}
		word, after := firstWord(rest)
		switch word {
		case "":
			return ""
		case "class", "interface", "object", "typealias":
			name, _ := firstWord(strings.TrimSpace(after))
			return strings.Trim(name, "`")
		case "fun":
			after = strings.TrimSpace(after)
			if w, a := firstWord(after); w == "interface" {
				name, _ := firstWord(strings.TrimSpace(a))
				return strings.Trim(name, "`")
			}
			return funName(after)
		case "val", "var":
			return propertyName(strings.TrimSpace(after))
		}
		if !modifiers[word] {
			return ""
		}
		rest = after
	}
}

var modifiers = map[string]bool{}

func init() {
	for _, m := range strings.Fields(`public private internal protected expect actual abstract open final sealed
		data enum annotation inline value inner const lateinit override suspend tailrec operator infix external
		noinline crossinline reified vararg companion`) {
		modifiers[m] = true
	}
}

// firstWord returns the identifier rest starts with, a backquoted one
// whole, and what follows it.
func firstWord(rest string) (string, string) {
	if strings.HasPrefix(rest, "`") {
		if i := strings.IndexByte(rest[1:], '`'); i >= 0 {
			return rest[:i+2], rest[i+2:]
		}
		return "", rest
	}
	i := 0
	for i < len(rest) && isIdent(rest[i]) {
		i++
	}
	return rest[:i], rest[i:]
}

func isIdent(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

// skipAnnotation returns rest after the annotation it starts with:
// @Name, @a.b.Name, @file:Name, with its arguments.
func skipAnnotation(rest string) string {
	i := 1
	for i < len(rest) && (isIdent(rest[i]) || rest[i] == '.' || rest[i] == ':') {
		i++
	}
	if i < len(rest) && rest[i] == '(' {
		depth := 0
		for ; i < len(rest); i++ {
			switch rest[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				return rest[i+1:]
			}
		}
		return ""
	}
	return rest[i:]
}

// skipTypeParams returns rest after the type parameters it starts with.
func skipTypeParams(rest string) string {
	if !strings.HasPrefix(rest, "<") {
		return rest
	}
	depth := 0
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return strings.TrimSpace(rest[i+1:])
			}
		}
	}
	return ""
}

// funName returns the name of a function from what follows "fun": the
// identifier before the first "(" that follows one, outside type
// arguments.
func funName(rest string) string {
	rest = skipTypeParams(rest)
	depth := 0
	for i := 0; i < len(rest); i++ {
		switch c := rest[i]; c {
		case '<':
			depth++
		case '>':
			if i > 0 && rest[i-1] == '-' {
				continue // ->
			}
			depth--
		case '(':
			if depth == 0 && i > 0 && (isIdent(rest[i-1]) || rest[i-1] == '`') {
				j := i
				if rest[i-1] == '`' {
					j = strings.LastIndexByte(rest[:i-1], '`')
					if j < 0 {
						return ""
					}
					return rest[j+1 : i-1]
				}
				for j > 0 && isIdent(rest[j-1]) {
					j--
				}
				return rest[j:i]
			}
		}
	}
	return ""
}

// propertyName returns the name of a property from what follows "val" or
// "var": the last name before its type, initializer or delegate.
func propertyName(rest string) string {
	rest = skipTypeParams(rest)
	if strings.HasPrefix(rest, "`") {
		name, _ := firstWord(rest)
		return strings.Trim(name, "`")
	}
	depth := 0
	end := len(rest)
loop:
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '<', '(':
			depth++
		case '>', ')':
			depth--
		case ':', '=', ' ', '\n':
			if depth == 0 {
				end = i
				break loop
			}
		}
	}
	head := rest[:end]
	if i := strings.LastIndexByte(head, '.'); i >= 0 {
		head = head[i+1:]
	}
	return strings.Trim(head, "`")
}
