package ide

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// KDoc returns the documentation comment of declaration d as Markdown, or
// "": its text, with [name] links as code, then its tags' blocks.
func KDoc(d psi.NodeRef) string {
	for c := d.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindKDOC {
			return renderKDoc(c.Text())
		}
		if !c.IsTrivia() {
			break
		}
	}
	return ""
}

// kdocTag is a block tag: @param x text.
type kdocTag struct {
	name, subject, text string
}

// renderKDoc renders the text of a KDoc comment, /** ... */.
func renderKDoc(text string) string {
	text = strings.TrimSuffix(strings.TrimPrefix(text, "/**"), "*/")
	var body []string
	var tags []kdocTag
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "*") {
			line = strings.TrimPrefix(line, "*")
			line = strings.TrimPrefix(line, " ")
		}
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
		}
		if !inCode && strings.HasPrefix(line, "@") {
			name, rest, _ := strings.Cut(line[1:], " ")
			t := kdocTag{name: name}
			switch name {
			case "param", "property", "throws", "exception", "sample", "see":
				t.subject, rest, _ = strings.Cut(strings.TrimSpace(rest), " ")
			}
			t.text = strings.TrimSpace(rest)
			tags = append(tags, t)
			continue
		}
		if len(tags) > 0 && !inCode {
			last := &tags[len(tags)-1]
			if line = strings.TrimSpace(line); line != "" {
				last.text = strings.TrimSpace(last.text + " " + line)
			}
			continue
		}
		body = append(body, line)
	}
	var b strings.Builder
	b.WriteString(links(strings.TrimSpace(strings.Join(body, "\n"))))
	section := func(title string, names ...string) {
		var items []kdocTag
		for _, t := range tags {
			for _, n := range names {
				if t.name == n {
					items = append(items, t)
				}
			}
		}
		if len(items) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("**" + title + ":**")
		for _, t := range items {
			b.WriteString("\n- ")
			if t.subject != "" {
				b.WriteString("`" + strings.Trim(t.subject, "[]") + "`")
				if t.text != "" {
					b.WriteString(" ")
				}
			}
			b.WriteString(links(t.text))
		}
	}
	section("Receiver", "receiver")
	section("Params", "param")
	section("Properties", "property")
	section("Constructor", "constructor")
	section("Returns", "return")
	section("Throws", "throws", "exception")
	section("See also", "see")
	section("Samples", "sample")
	section("Since", "since")
	section("Author", "author")
	section("Deprecated", "deprecated")
	return b.String()
}

// links writes KDoc's [name] links as code, and leaves Markdown's
// [text](url) links and code spans alone.
func links(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '`':
			j := strings.IndexByte(s[i+1:], '`')
			if j < 0 {
				b.WriteString(s[i:])
				return b.String()
			}
			b.WriteString(s[i : i+j+2])
			i += j + 1
			continue
		case '[':
			j := strings.IndexByte(s[i+1:], ']')
			if j < 0 {
				break
			}
			end := i + 1 + j
			if end+1 < len(s) && (s[end+1] == '(' || s[end+1] == '[') {
				break // a Markdown link
			}
			b.WriteString("`" + s[i+1:end] + "`")
			i = end
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
