package syntax

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// kotlinSources returns every .kt and .kts file in the repository.
func kotlinSources(t testing.TB) map[string]string {
	srcs := map[string]string{}
	filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".kts")) {
			return err
		}
		b, err := os.ReadFile(path)
		if err == nil {
			srcs[path] = string(b)
		}
		return nil
	})
	if len(srcs) == 0 {
		t.Fatal("no Kotlin sources")
	}
	return srcs
}

// parseTokens parses toks as Parse parses a file's tokens, and returns
// the file or the first error.
func parseTokens(path string, toks []Token) (f *File, err *Error) {
	p := &parser{file: path, toks: toks}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			f, err = nil, p.err
		}
	}()
	return p.parseFile(), nil
}

// lexDiff compares the two lexers' results on path. They agree if they
// give the same tokens, or the same first error. Where the tokens differ
// with no error from one of them, they agree if the parser accepts
// neither token stream, or gives the same AST for both.
func lexDiff(path string, a, b []Token, ae, be []*Error) string {
	return lexDiffMode(path, a, b, ae, be, true)
}

// lexDiffMode is lexDiff; with sameError false, any two errors agree.
func lexDiffMode(path string, a, b []Token, ae, be []*Error, sameError bool) string {
	if len(ae) > 0 && len(be) > 0 {
		if sameError && *ae[0] != *be[0] {
			return fmt.Sprintf("first error:\n legacy %v\n new    %v", ae[0], be[0])
		}
		return ""
	}
	if len(ae) == 0 && len(be) == 0 && reflect.DeepEqual(a, b) {
		return ""
	}
	var fa, fb *File
	var pa, pb *Error
	if len(ae) == 0 {
		fa, pa = parseTokens(path, a)
	}
	if len(be) == 0 {
		fb, pb = parseTokens(path, b)
	}
	switch {
	case fa == nil && fb == nil:
		return ""
	case fa != nil && fb != nil && Equal(fa, fb, false):
		return ""
	}
	i := 0
	for i < len(a) && i < len(b) && reflect.DeepEqual(a[i], b[i]) {
		i++
	}
	var x, y any = "-", "-"
	if i < len(a) {
		x = a[i]
	}
	if i < len(b) {
		y = b[i]
	}
	return fmt.Sprintf("token %d:\n legacy %+v %v %v\n new    %+v %v %v", i, x, ae, pa, y, be, pb)
}

// TestLexerAgrees checks that the lexer built on psi.Lex gives the tokens
// kc's own lexer gave, and the same first error, on every Kotlin file in
// the repository, whole and cut short.
func TestLexerAgrees(t *testing.T) {
	same, diff := 0, 0
	report := os.Getenv("LEXDIFF_ALL") != ""
	for path, src := range kotlinSources(t) {
		cuts := []int{len(src)}
		for i := 1; i < 24; i++ {
			cuts = append(cuts, len(src)*i/24)
		}
		for _, n := range cuts {
			text := src[:n]
			a, ae := legacyTokenize(path, text)
			b, be := Tokenize(path, text)
			if d := lexDiff(path, a, b, ae, be); d != "" {
				diff++
				if report || diff <= 10 {
					t.Errorf("%s cut at %d of %d: %s", path, n, len(src), d)
				}
				continue
			}
			same++
			for _, tok := range a {
				for _, part := range tok.Parts {
					if !part.IsExpr {
						continue
					}
					a, ae := legacyTokenizeAt(path, part.Src, part.SrcPos)
					b, be := tokenizeAt(path, part.Src, part.SrcPos)
					if d := lexDiff(path, a, b, ae, be); d != "" {
						t.Errorf("%s template %q: %s", path, part.Src, d)
					}
				}
			}
		}
	}
	t.Logf("%d inputs agree, %d differ", same, diff)
}

var lexerSeeds = []string{
	"a?.b ?: c?::d x?..y z?..<w q?...r",
	"x!! y!!=z a!!in b !!is c !!!d e!==f !in !is !isEmpty() !inside",
	"loop@ for (i in x) { break@loop; continue@`x y`; return@as?; this@if }",
	"`a` `` `unterminated\nnext $x $$y #! # \\ \r\n",
	"1 1L 1l 1u 1uL 1UL 0x1F 0b101 1.5 1.5f .5 1e5 1e 1.5L 1..2 0123 1_000",
	`'a' '\n' 'A' '' 'ab' '\' '\\' 'x`,
	`"a$b ${c} $$ $` + "`q`" + ` \t A \u12 \q"` + "\n\"unterminated\n\"\\\n\"",
	`"""raw $x ${y} """"` + ` $$"a $b $$c $${d}" $$$"""$$x $$$y"""`,
	`"${ "nested ${ 'c' } \q" + /* } */ 1 }" "${ unterminated`,
	"/* nested /* comment */ still */ x /* unterminated /* x */",
	"/** kdoc */ /**/ /***/ // eol\n;; => ... ::",
	"f(a\n, b)[c\n] { d\ne }",
	"a /* \n */ b // c\n d",
	"é `é` \"é${é}\" é@ return@é /* é */ x",
}

func FuzzLexerAgrees(f *testing.F) {
	for _, s := range lexerSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// kc's lexer didn't see comments in a ${...} template expression,
		// where Kotlin's does: "${ f(/* } */) }".
		if strings.Contains(src, "${") && (strings.Contains(src, "/*") || strings.Contains(src, "//")) {
			t.Skip()
		}
		a, ae := legacyTokenize("f.kt", src)
		b, be := Tokenize("f.kt", src)
		// Both must accept the same inputs and give the same AST; their
		// errors on input they both reject may differ.
		if d := lexDiffMode("f.kt", a, b, ae, be, false); d != "" {
			t.Errorf("%q: %s", src, d)
		}
	})
}
