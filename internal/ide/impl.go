package ide

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Paths returns the paths of the files the view analyzed, sorted.
func (v *View) Paths() []string {
	out := make([]string, 0, len(v.byPath))
	for p := range v.byPath {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// eachSymbol calls f with each symbol of the view's result.
func (v *View) eachSymbol(f func(analysis.Symbol)) {
	for id := uint32(1); ; id++ {
		s, ok := v.res.Symbol(id)
		if !ok {
			return
		}
		f(s)
	}
}

// Implementations returns the declarations in the sources that implement
// what the caret at off is on: the classes that extend a class or
// interface, through any supertype, or the functions that override a
// function. views are those of the modules whose sources hold path.
func Implementations(views []*View, path string, off int) []Loc {
	if len(views) == 0 {
		return nil
	}
	t, ok := views[0].at(path, off)
	if !ok || !t.decl.Valid() {
		return nil
	}
	k, from := keyOf(t.decl), t.decl.ContainingFile()
	var out []Loc
	for _, v := range views {
		d, ok := v.find(k, from)
		if !ok {
			continue
		}
		add := func(s analysis.Symbol) {
			if n := v.node(*s.Declaration); n.Valid() {
				out = append(out, Loc{n.ContainingFile(), nameRange(n)})
			}
		}
		switch d.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
			f, ok := v.fact(d)
			cls, ok2 := v.symbol(f.Symbol)
			if !ok || !ok2 || cls.QualifiedName == "" {
				continue
			}
			v.eachSymbol(func(s analysis.Symbol) {
				if s.Kind == "class" && s.Declaration != nil && slices.Contains(s.Supertypes, cls.QualifiedName) {
					add(s)
				}
			})
		case psi.KindFUN:
			el := analysis.ElementID{Snapshot: d.ContainingFile().ID(), Node: d.ID()}
			v.eachSymbol(func(s analysis.Symbol) {
				if s.Declaration != nil && slices.Contains(s.Overrides, el) {
					add(s)
				}
			})
		}
	}
	return sortLocs(out)
}

// SymbolMatch is a declaration a workspace symbol query finds.
type SymbolMatch struct {
	Doc       *Document
	Symbol    Symbol // without children
	Container string // the enclosing class's name, or the package's
	score     int
}

// WorkspaceSymbols returns the declarations of docs' outlines whose names
// match query, best first, at most limit: a name matches if query's
// letters occur in it in order, ignoring case. An exact name ranks
// first, then a prefix, then a match of word starts (camel humps, so "AB"
// finds ArrayBuffer), then the others.
func WorkspaceSymbols(docs []*Document, query string, limit int) []SymbolMatch {
	var out []SymbolMatch
	var walk func(d *Document, syms []Symbol, container string)
	walk = func(d *Document, syms []Symbol, container string) {
		for _, s := range syms {
			if s.Kind != ConstructorSymbol {
				if score, ok := matchScore(s.Name, query); ok {
					m := s
					m.Children = nil
					out = append(out, SymbolMatch{Doc: d, Symbol: m, Container: container, score: score})
				}
			}
			walk(d, s.Children, s.Name)
		}
	}
	for _, d := range docs {
		walk(d, Symbols(d), packageName(d.Snap))
	}
	slices.SortFunc(out, func(a, b SymbolMatch) int {
		return cmp.Or(
			cmp.Compare(b.score, a.score),
			cmp.Compare(len(a.Symbol.Name), len(b.Symbol.Name)),
			cmp.Compare(a.Symbol.Name, b.Symbol.Name),
			cmp.Compare(a.Doc.Path, b.Doc.Path),
			cmp.Compare(a.Symbol.NameRange.Start, b.Symbol.NameRange.Start),
		)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// packageName returns the name of snap's package, or "".
func packageName(snap *psi.Snapshot) string {
	for c := snap.RootNode().FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindPACKAGE_DIRECTIVE {
			var b strings.Builder
			for n := c.FirstChild(); n.Valid(); n = n.NextSibling() {
				if !n.IsTrivia() && n.NumericKind() != psi.KindPACKAGE_KEYWORD {
					b.WriteString(strings.Join(strings.Fields(n.Text()), ""))
				}
			}
			return b.String()
		}
	}
	return ""
}

// matchScore rates how well name matches query; ok is false if it
// doesn't.
func matchScore(name, query string) (score int, ok bool) {
	if query == "" {
		return 0, true
	}
	ln, lq := strings.ToLower(name), strings.ToLower(query)
	switch {
	case ln == lq:
		return 4, true
	case strings.HasPrefix(ln, lq):
		return 3, true
	case humps(name, query):
		return 2, true
	}
	i := 0
	for _, r := range lq {
		j := strings.IndexRune(ln[i:], r)
		if j < 0 {
			return 0, false
		}
		i += j + utf8.RuneLen(r)
	}
	return 1, true
}

// humps reports whether query is a sequence of prefixes of name's words,
// in order: "ArBu" and "AB" of ArrayBuffer, "fBN" of findByName.
func humps(name, query string) bool {
	var words []string
	start := 0
	prev := rune(0)
	for i, r := range name {
		if i > 0 && (unicode.IsUpper(r) && !unicode.IsUpper(prev) || r == '_' || prev == '_') {
			if w := strings.Trim(name[start:i], "_"); w != "" {
				words = append(words, strings.ToLower(w))
			}
			start = i
		}
		prev = r
	}
	if w := strings.Trim(name[start:], "_"); w != "" {
		words = append(words, strings.ToLower(w))
	}
	return matchWords(words, strings.ToLower(query))
}

// matchWords reports whether q is a concatenation of nonempty prefixes of
// some of words, in order, starting with the first word.
func matchWords(words []string, q string) bool {
	if q == "" {
		return true
	}
	if len(words) == 0 {
		return false
	}
	w := words[0]
	for n := min(len(w), len(q)); n >= 1; n-- {
		if w[:n] != q[:n] {
			continue
		}
		if n == len(q) {
			return true
		}
		for j := 1; j < len(words); j++ {
			if matchWords(words[j:], q[n:]) {
				return true
			}
		}
	}
	return false
}
