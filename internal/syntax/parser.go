package syntax

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/barteo/kotlin-compiler-go/internal/intbits"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

type parser struct {
	file string
	toks []Token
	i    int
	err  *Error
	// recover is set by ParseAll: an error in a declaration is recorded in
	// errs, and parsing goes on with the next declaration.
	recover bool
	errs    []*Error
	// noTrailing is set while parsing a delegate in a class header, where a
	// brace starts the class body rather than a trailing lambda.
	noTrailing bool
	// anonFuns numbers the file's anonymous functions, whose labels are
	// fun$1, fun$2, ...
	anonFuns int
	// suppressInvisible is set once an annotation suppresses
	// INVISIBLE_MEMBER or INVISIBLE_REFERENCE (see File.SuppressInvisible).
	suppressInvisible bool
	// suppressShadowed is set once an annotation suppresses
	// RECEIVER_SHADOWED_BY_CONTEXT_PARAMETER (see File.SuppressShadowed).
	suppressShadowed bool
	// suppressDSL is set once an annotation suppresses
	// DSL_SCOPE_VIOLATION (see File.SuppressDSL).
	suppressDSL bool
	// later are the errors parsing goes on after (see File.Errors).
	later []*Error
}

type bailout struct{}

// Parse parses one source file. Parsing stops at the first syntax error.
func Parse(path, src string) (f *File, errs []*Error) {
	toks, lexErrs := Tokenize(path, src)
	if len(lexErrs) > 0 {
		return nil, lexErrs
	}
	p := &parser{file: path, toks: toks}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			f, errs = nil, []*Error{p.err}
		}
	}()
	f = p.parseFile()
	f.Path = path
	f.Errors = p.later
	f.SuppressInvisible = p.suppressInvisible
	f.SuppressShadowed = p.suppressShadowed
	f.SuppressDSL = p.suppressDSL
	f.Facade = facade(f, path)
	f.Part = part(f, path)
	return f, nil
}

// facade returns the JVM internal name of a file's facade class: from the
// file name, or @file:JvmName, in the file's package.
func facade(f *File, path string) string {
	name := f.JvmName
	if name == "" {
		name = FacadeName(path)
	}
	if f.Package == "" {
		return name
	}
	return strings.ReplaceAll(f.Package, ".", "/") + "/" + name
}

// part returns the JVM internal name of a file's part of a multi-file
// class, Facade__FileKt, if it is annotated @file:JvmMultifileClass and
// named with @file:JvmName, or "".
func part(f *File, path string) string {
	if f.JvmName == "" {
		return ""
	}
	for _, a := range f.Annotations {
		if a.Is("JvmMultifileClass") {
			return f.Facade + "__" + FacadeName(path)
		}
	}
	return ""
}

// ParseAll parses a file like Parse, but doesn't stop at the first error.
// After an error in a top-level declaration or a class member, it skips
// to the next one, so it returns the first error of each declaration that
// fails. bench/survey uses it to see past unsupported constructs.
func ParseAll(path, src string) (*File, []*Error) {
	toks, lexErrs := Tokenize(path, src)
	if len(lexErrs) > 0 {
		return nil, lexErrs
	}
	p := &parser{file: path, toks: toks, recover: true}
	f := p.parseFile()
	f.Path = path
	f.Errors = p.later
	f.SuppressInvisible = p.suppressInvisible
	f.SuppressShadowed = p.suppressShadowed
	f.SuppressDSL = p.suppressDSL
	f.Facade = facade(f, path)
	f.Part = part(f, path)
	return f, p.errs
}

// decl parses one declaration with parse. With p.recover set, an error is
// recorded and the rest of the declaration skipped.
func (p *parser) decl(parse func()) {
	if !p.recover {
		parse()
		return
	}
	start := p.i
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			// A declaration after annotations on their own lines is parsed
			// again after them, and fails again.
			if n := len(p.errs); n == 0 || *p.errs[n-1] != *p.err {
				p.errs = append(p.errs, p.err)
			}
			p.i = start
			p.skipDecl()
		}
	}()
	parse()
}

// skipDecl skips the declaration at p.i: up to a newline or ';' outside
// braces that comes before the start of another declaration, or up to the
// '}' that closes the enclosing body.
func (p *parser) skipDecl() {
	depth := 0
	braces := false
	p.next()
	for {
		switch p.tok().Kind {
		case EOF:
			return
		case LBRACE:
			depth++
			braces = true
		case RBRACE:
			if depth == 0 {
				return
			}
			depth--
		case NL, SEMI:
			if depth == 0 {
				// An initializer can start on the next line, as an
				// anonymous function can: val f =\n fun (x: Int) ...
				cont := p.toks[p.i-1].Kind == ASSIGN
				p.skipSemis()
				// A class header can go on with its primary constructor:
				// class A\nprivate constructor(...).
				if t := p.tok(); t.Kind == RBRACE || t.Kind == EOF || !cont && startsDecl(t) && (braces || !p.primaryCtorAt(p.i)) && !p.accessorAt(p.i) {
					return
				}
				continue
			}
		}
		p.next()
	}
}

// accessorAt reports whether a property's accessor starts at j, after
// its annotations and visibility: private set, @A get() = ... It only
// looks at tokens, so that it can't fail while skipping a declaration.
func (p *parser) accessorAt(j int) bool {
	for {
		switch t := p.toks[j]; {
		case t.Kind == AT:
			// @A, @a.B or @get:A
			j += 2
			for (p.toks[j].Kind == DOT || p.toks[j].Kind == COLON) && p.toks[j+1].Kind == IDENT {
				j += 2
			}
			if p.toks[j].Kind == LPAREN {
				for depth := 0; p.toks[j].Kind != EOF; j++ {
					if k := p.toks[j].Kind; k == LPAREN {
						depth++
					} else if k == RPAREN {
						if depth--; depth == 0 {
							j++
							break
						}
					}
				}
			}
			for p.toks[j].Kind == NL {
				j++
			}
		case t.Kind == IDENT && (isVisibility(t.Text) || t.Text == "external"):
			j++
		case t.Kind == IDENT && (t.Text == "get" || t.Text == "set"):
			switch p.toks[j+1].Kind {
			case LPAREN, NL, SEMI, RBRACE, EOF:
				return true
			}
			return false
		default:
			return false
		}
	}
}

// startsDecl reports whether a declaration can start with t.
func startsDecl(t *Token) bool {
	switch t.Kind {
	case FUN, VAL, VAR, CLASS, INTERFACE, OBJECT, TYPEALIAS, PACKAGE, AT:
		return true
	case IDENT:
		switch t.Text {
		case "import", "constructor", "init", "private", "public", "internal", "external":
			return true
		}
	}
	return false
}

// FacadeName returns the JVM class kotlinc uses for top-level declarations
// in a file: Foo.kt -> FooKt, fizz-buzz.kt -> Fizz_buzzKt, -Ext.kt ->
// _ExtKt, 1st.kt -> _1stKt.
func FacadeName(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".kt")
	var sb strings.Builder
	for i, r := range base {
		switch {
		case i == 0 && unicode.IsLetter(r):
			sb.WriteRune(unicode.ToUpper(r))
		case i == 0 && unicode.IsDigit(r):
			sb.WriteByte('_')
			sb.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String() + "Kt"
}

func (p *parser) tok() *Token    { return &p.toks[p.i] }
func (p *parser) at(k Kind) bool { return p.toks[p.i].Kind == k }

func (p *parser) next() Token {
	t := p.toks[p.i]
	if t.Kind != EOF {
		p.i++
	}
	return t
}

func (p *parser) failAt(pos Pos, format string, args ...any) {
	p.err = &Error{File: p.file, Pos: pos, Msg: fmt.Sprintf(format, args...)}
	panic(bailout{})
}

// errorLater records an error that parsing goes on after.
func (p *parser) errorLater(pos Pos, format string, args ...any) {
	p.later = append(p.later, &Error{File: p.file, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// unsupported reports the use of a Kotlin feature kc doesn't support.
// feature is a stable name for it, for tools that count them.
func (p *parser) unsupported(pos Pos, feature, format string, args ...any) {
	p.err = &Error{File: p.file, Pos: pos, Msg: fmt.Sprintf(format, args...), Feature: feature}
	panic(bailout{})
}

// notSupported reports an unsupported keyword or construct as
// "'<text>' is not supported".
func (p *parser) notSupported(t *Token, feature string) {
	text := t.Text
	if text == "" {
		text = t.Kind.String()
	}
	p.unsupported(t.Pos, feature, "'%s' is not supported", text)
}

func (p *parser) unexpected(what string) {
	t := p.tok()
	desc := t.Kind.String()
	if t.Kind == IDENT {
		desc = "'" + t.Text + "'"
	} else if t.Kind > STRING {
		desc = "'" + desc + "'"
	}
	p.failAt(t.Pos, "expected %s, found %s", what, desc)
}

func (p *parser) expect(k Kind) Token {
	if !p.at(k) {
		p.unexpected("'" + k.String() + "'")
	}
	return p.next()
}

func (p *parser) ident() Token {
	if !p.at(IDENT) {
		p.unexpected("identifier")
	}
	return p.next()
}

func (p *parser) skipNL() {
	for p.at(NL) {
		p.i++
	}
}

func (p *parser) skipSemis() {
	for p.at(NL) || p.at(SEMI) {
		p.i++
	}
}

// peekAfterNL returns the kind of the first non-newline token at or after
// index i.
func (p *parser) peekAfterNL(i int) Kind {
	for p.toks[i].Kind == NL {
		i++
	}
	return p.toks[i].Kind
}

// peekPastNL returns the kind of the first non-newline token.
func (p *parser) peekPastNL() Kind { return p.peekAfterNL(p.i) }

// endOfStatement requires a statement separator or the end of the block.
func (p *parser) endOfStatement() {
	switch p.tok().Kind {
	case NL, SEMI, RBRACE, EOF:
		return
	}
	p.unexpected("newline or ';'")
}

var visibilities = map[string]Visibility{"public": Public, "private": Private, "protected": Protected, "internal": Internal}

func isVisibility(s string) bool { _, ok := visibilities[s]; return ok }

// modifiers parses a declaration's modifiers and annotations. kc records
// visibility modifiers but doesn't check them. A modifier is a soft keyword: it
// is a name when no declaration follows it, as in fun f(value: Int).
func (p *parser) modifiers() Modifiers {
	var m Modifiers
	start := p.i
	for {
		// A modifier can end its line, as kotlinpoet wraps long
		// declarations: public\n    val x: T.
		if p.i > start && p.at(NL) && declStart(p.peekPastNL()) {
			p.skipNL()
		}
		switch t := p.tok(); {
		case t.Kind == AT:
			a := p.annotation()
			if a.Target == "" && a.Is("JvmInline") {
				m.JvmInline = true
			}
			if a.Name != "" {
				m.Annotations = append(m.Annotations, a)
			}
			p.skipNL()
		case t.Kind != IDENT:
			return m
		case t.Text == "context" && p.toks[p.i+1].Kind == LPAREN:
			if m.Contexts != nil {
				p.failAt(t.Pos, "repeated context parameter lists")
			}
			m.Contexts, m.ContextP = p.contextParams(), t.Pos
			p.skipNL()
		case !declStart(p.toks[p.i+1].Kind):
			return m
		case t.Text == "private" || t.Text == "public" || t.Text == "internal" || t.Text == "protected":
			m.Visibility, m.Explicit = visibilities[t.Text], true
			p.next()
		case t.Text == "open":
			m.Open = true
			p.next()
		case t.Text == "abstract":
			m.Abstract = true
			p.next()
		case t.Text == "override":
			m.Override = true
			p.next()
		case t.Text == "final":
			m.Final = true
			p.next()
		case t.Text == "companion":
			m.Companion = true
			p.next()
		case t.Text == "data":
			m.Data = true
			p.next()
		case t.Text == "enum":
			m.Enum = true
			p.next()
		case t.Text == "sealed":
			m.Sealed = true
			p.next()
		case t.Text == "const":
			m.Const = true
			p.next()
		case t.Text == "vararg":
			m.Vararg, m.VarargP = true, t.Pos
			p.next()
		case t.Text == "operator":
			m.Operator = true
			p.next()
		case t.Text == "infix":
			m.Infix = true
			p.next()
		case t.Text == "expect":
			m.Expect, m.ExpectP = true, t.Pos
			p.next()
		case t.Text == "actual":
			m.Actual = true
			p.next()
		case t.Text == "annotation":
			m.Annotation = true
			p.next()
		case t.Text == "value":
			m.Value = true
			p.next()
		case t.Text == "inline":
			m.Inline = true
			p.next()
		case t.Text == "noinline":
			m.Noinline = true
			p.next()
		case t.Text == "crossinline":
			m.Crossinline = true
			p.next()
		case t.Text == "suspend":
			m.Suspend = true
			p.next()
		case t.Text == "lateinit":
			m.Lateinit = true
			p.next()
		case t.Text == "inner":
			m.Inner = true
			p.next()
		case t.Text == "tailrec":
			m.Tailrec = true
			p.next()
		case t.Text == "external":
			m.External, m.ExternalP = true, t.Pos
			p.next()
		default:
			return m
		}
	}
}

// contextParams parses a declaration's context parameters, context(a: A,
// _: B). The context receivers that came before them, context(A), are an
// error in Kotlin 2.4.
func (p *parser) contextParams() []*Param {
	p.next()
	p.expect(LPAREN)
	ps := []*Param{}
	if p.skipNL(); p.at(RPAREN) {
		p.failAt(p.tok().Pos, "expected a context parameter")
	}
	for p.skipNL(); !p.at(RPAREN); p.skipNL() {
		for j := p.i; ; j++ {
			if t := p.toks[j]; t.Kind == AT {
				j = p.annotationEnd(j) - 1
			} else if t.Kind != IDENT || !isModifier(t.Text) || p.toks[j+1].Kind == COLON {
				break
			} else if t.Text != "noinline" && t.Text != "crossinline" {
				p.errorLater(t.Pos, "modifier '%s' is not applicable to 'context parameter'", t.Text)
			}
		}
		m := p.modifiers()
		if !p.at(IDENT) || p.toks[p.i+1].Kind != COLON {
			p.failAt(p.tok().Pos, "context parameters must be named: use '_' for an anonymous one, context(_: Type)")
		}
		name := p.next()
		p.next()
		ps = append(ps, &Param{P: name.Pos, Name: name.Text, Type: p.parseType(), Noinline: m.Noinline, Crossinline: m.Crossinline,
			Annotations: m.Annotations})
		if p.skipNL(); p.at(ASSIGN) {
			p.errorLater(name.Pos, "context parameters cannot have default values")
			p.next()
			p.skipNL()
			p.parseExpr()
			p.skipNL()
		}
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN)
	return ps
}

// contextDecl reports whether a statement is a local declaration with
// context parameters, context(a: A) fun f(), rather than a call.
func (p *parser) contextDecl() bool {
	if p.toks[p.i+1].Kind != LPAREN {
		return false
	}
	depth, j := 0, p.i+1
	for ; p.toks[j].Kind != EOF; j++ {
		if k := p.toks[j].Kind; k == LPAREN {
			depth++
		} else if k == RPAREN {
			if depth--; depth == 0 {
				break
			}
		}
	}
	for j++; p.toks[j].Kind == NL; j++ {
	}
	k := p.toks[j]
	return k.Kind == FUN && !p.anonFunAt(j) || k.Kind == AT || k.Kind == IDENT && isModifier(k.Text) && declStart(p.toks[j+1].Kind)
}

// contextAnonFun reports whether the context at the parser's position
// starts the context parameters of an anonymous function, context(a: A)
// fun () = a, rather than a call.
func (p *parser) contextAnonFun() bool {
	if p.toks[p.i+1].Kind != LPAREN {
		return false
	}
	depth, j := 0, p.i+1
	for ; p.toks[j].Kind != EOF; j++ {
		if k := p.toks[j].Kind; k == LPAREN {
			depth++
		} else if k == RPAREN {
			if depth--; depth == 0 {
				break
			}
		}
	}
	for j++; p.toks[j].Kind == NL; j++ {
	}
	return p.toks[j].Kind == FUN && p.anonFunAt(j)
}

// noContexts reports context parameters on a declaration that Kotlin
// doesn't allow them on, what, as kotlinc does, and drops them.
func (p *parser) noContexts(m *Modifiers, what string) {
	if m.Contexts != nil {
		p.errorLater(m.ContextP, "context parameters on %s are unsupported", what)
		m.Contexts = nil
	}
}

// danglingContexts reports context parameters that no declaration
// follows.
func (p *parser) danglingContexts(m Modifiers) {
	if m.Contexts != nil {
		p.failAt(m.ContextP, "expected a declaration after the context parameters")
	}
}

// anonFun parses an anonymous function, fun (x: Int): Int { ... },
// fun Int.() = ..., as a lambda with a label of its own, which the
// returns in its body return to, and with typed parameters. The label is
// label, from label@ fun () { ... }, or one of its own.
func (p *parser) anonFun(label string) *Lambda {
	pos := p.expect(FUN).Pos
	l := &Lambda{Arrow: true, Anon: true, Label: label}
	l.P = pos
	if label == "" {
		p.anonFuns++
		l.Label = "fun$" + strconv.Itoa(p.anonFuns)
	}
	if !p.at(LPAREN) {
		// A receiver type, up to the dot before the parameters.
		depth, j := 0, p.i
		for ; p.toks[j].Kind != EOF; j++ {
			k := p.toks[j].Kind
			if k == LT || k == LPAREN {
				depth++
			} else if k == GT || k == RPAREN {
				depth--
			} else if depth == 0 && (k == DOT || k == SAFE_DOT) && p.toks[j+1].Kind == LPAREN {
				break
			}
		}
		kind := p.toks[j].Kind
		p.toks[j].Kind = SEMI
		func() {
			defer func() { p.toks[j].Kind = kind }()
			l.AnonRecv = p.parseType()
		}()
		if p.at(SAFE_DOT) {
			l.AnonRecv.Nullable = true // ?. is one token
			p.next()
		} else {
			p.expect(DOT)
		}
	}
	// The parameters, whose types the expected type can give.
	p.expect(LPAREN)
	for !p.at(RPAREN) {
		p.modifiers()
		name := p.ident()
		lp := &LambdaParam{P: name.Pos, Name: name.Text}
		if p.at(COLON) {
			p.next()
			lp.Type = p.parseType()
		}
		if p.at(ASSIGN) {
			p.failAt(p.tok().Pos, "anonymous functions can't have default values")
		}
		l.Params = append(l.Params, lp)
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN)
	if p.peekPastNL() == COLON {
		p.skipNL()
		p.next()
		p.skipNL()
		l.AnonRet = p.parseType()
	}
	switch p.peekPastNL() {
	case LBRACE:
		p.skipNL()
		l.Body = p.parseBlock()
		l.AnonUnit = l.AnonRet == nil // a block body returns a value only with return
	case ASSIGN:
		p.skipNL()
		p.next()
		p.skipNL()
		x := p.parseExpr()
		l.Body = &Block{P: x.Pos(), Stmts: []Stmt{&ExprStmt{X: x}}}
	default:
		p.unexpected("the body of an anonymous function")
	}
	labelReturns(l.Body, l.Label)
	return l
}

// labelReturns labels the returns in an anonymous function's body that
// return from it: not those of the lambdas, local functions and objects
// in it.
func labelReturns(b *Block, label string) {
	Inspect(b, func(n any) bool {
		switch n := n.(type) {
		case *Lambda, *LocalFun, *ObjectExpr, *LocalClass:
			return false
		case *Jump:
			if n.Kind == RETURN && n.Label == "" {
				n.Label = label
			}
		}
		return true
	})
}

// anonFunAt reports whether the fun at i starts an anonymous function,
// fun (x: Int) ... or fun Int.() ...: the parenthesis of its parameters
// follows fun or a receiver's dot, not a name.
func (p *parser) anonFunAt(i int) bool {
	depth := 0
	for j := i + 1; ; j++ {
		switch k := p.toks[j].Kind; {
		case k == EOF:
			return false
		case k == LT:
			depth++
		case k == GT:
			depth--
		case depth > 0:
		case k == LPAREN:
			k := p.toks[j-1].Kind
			return k == FUN || k == DOT || k == SAFE_DOT
		case k != IDENT && k != DOT && k != SAFE_DOT && k != QUEST:
			return false
		}
	}
}

// localClass parses a local class, after its modifiers. Kotlin has no
// local interfaces, objects or enum classes.
func (p *parser) localClass(m Modifiers) *LocalClass {
	p.noContexts(&m, "classes")
	t := p.tok()
	switch {
	case t.Kind == INTERFACE:
		p.failAt(t.Pos, "'interface' is not allowed here: local interfaces are not supported")
	case t.Kind == OBJECT:
		p.failAt(t.Pos, "named object cannot be local: use an anonymous object instead")
	case m.Enum:
		p.failAt(t.Pos, "local enum classes are not allowed")
	}
	cd := p.parseClass(m)
	cd.Local = true
	return &LocalClass{Decl: cd}
}

// localFun parses a local function, after its modifiers.
func (p *parser) localFun(m Modifiers) *LocalFun {
	p.paramOnly(m)
	pos := p.tok().Pos
	fd := p.parseFun()
	fd.Modifiers = m
	if m.External {
		p.unsupported(m.ExternalP, "local external function", "local external functions are not supported")
	}
	if fd.Body == nil && fd.ExprBody == nil {
		p.failAt(pos, "a local function must have a body")
	}
	return &LocalFun{Decl: fd}
}

// isModifier reports whether s is a modifier keyword.
func isModifier(s string) bool {
	switch s {
	case "private", "public", "internal", "protected", "open", "abstract", "override", "final",
		"companion", "data", "enum", "sealed", "const", "vararg", "operator", "infix", "expect", "actual", "annotation", "value", "inline", "noinline", "crossinline", "suspend",
		"lateinit", "tailrec", "inner", "external":
		return true
	}
	return false
}

// paramOnly reports modifiers that only parameters take on another
// declaration.
func (p *parser) paramOnly(m Modifiers) {
	if m.Vararg {
		p.failAt(m.VarargP, "modifier 'vararg' is only allowed on parameters")
	}
}

// declStart reports whether a token can follow a modifier: another
// modifier or a name, an annotation, or a declaration keyword.
func declStart(k Kind) bool {
	switch k {
	case IDENT, AT, FUN, VAL, VAR, CLASS, INTERFACE, OBJECT, TYPEALIAS, NL:
		return true
	}
	return false
}

func (p *parser) parseFile() *File {
	f := &File{}
	p.skipSemis()
	// File annotations: @file:JvmName("Name") names the facade class.
	// Others annotate the first declaration.
	for p.at(AT) && p.toks[p.i+1].Text == "file" && p.toks[p.i+2].Kind == COLON {
		p.decl(func() {
			a := p.annotation()
			if a.Target == "file" && a.Is("JvmName") {
				if s, ok := a.Str(""); ok {
					f.JvmName = s
				}
			}
			if a.Target == "file" {
				f.Annotations = append(f.Annotations, a)
			}
		})
		p.skipSemis()
	}
	if p.at(PACKAGE) {
		p.decl(func() {
			p.next()
			name := p.ident().Text
			for p.at(DOT) {
				p.next()
				name += "." + p.ident().Text
			}
			f.Package = name
			p.endOfStatement()
		})
		p.skipSemis()
	}
	for t := p.tok(); t.Kind == IDENT && t.Text == "import"; t = p.tok() {
		p.decl(func() {
			f.Imports = append(f.Imports, p.parseImport())
			p.endOfStatement()
		})
		p.skipSemis()
	}
	for !p.at(EOF) {
		p.decl(func() {
			m := p.modifiers()
			p.paramOnly(m)
			add := func(d Decl) {
				if m.Expect {
					f.Expects = append(f.Expects, d)
				} else {
					f.Decls = append(f.Decls, d)
				}
			}
			switch p.tok().Kind {
			case VAL, VAR:
			case FUN:
				if p.toks[p.i+1].Kind == INTERFACE {
					p.noContexts(&m, "classes")
				}
			case TYPEALIAS:
				p.noContexts(&m, "type aliases")
			case CLASS, INTERFACE, OBJECT:
				p.noContexts(&m, "classes")
			case PACKAGE:
				if m.Contexts != nil {
					p.failAt(m.ContextP, "context parameters are unsupported in this position")
				}
			default:
				p.danglingContexts(m)
			}
			switch t := p.tok(); t.Kind {
			case FUN:
				if p.toks[p.i+1].Kind == INTERFACE {
					add(p.funInterface(m))
					break
				}
				fd := p.parseFun()
				fd.Modifiers = m
				add(fd)
			case VAL, VAR:
				pd := p.parseProp(m)
				for _, d := range delegated(pd, false, f.Decls) {
					add(d)
				}
			case CLASS, INTERFACE, OBJECT:
				add(p.parseClass(m))
			case TYPEALIAS:
				add(p.parseTypeAlias(m))
			case PACKAGE:
				p.notSupported(t, t.Kind.String())
			case IDENT:
				if t.Text == "import" {
					p.failAt(t.Pos, "imports must come before declarations")
				}
				p.unexpected("declaration")
			default:
				p.unexpected("declaration")
			}
			p.endOfStatement()
		})
		p.skipSemis()
	}
	return f
}

func (p *parser) parseImport() *Import {
	imp := &Import{P: p.next().Pos}
	parts := []string{p.ident().Text}
	for p.at(DOT) {
		p.next()
		if p.at(STAR) {
			p.next()
			imp.Star = true
			break
		}
		parts = append(parts, p.ident().Text)
	}
	imp.Path = strings.Join(parts, ".")
	if !imp.Star && p.at(AS) {
		p.next()
		imp.Alias = p.ident().Text
	}
	return imp
}

func (p *parser) parseType() *TypeRef {
	unsafe, composable, noInfer := false, false, false
	var wild Wildcards
	for p.at(AT) {
		a := p.annotation()
		if a.Target == "receiver" {
			// receiver takes those of a receiver type.
			p.failAt(a.P, "this annotation is not applicable to target 'type usage' and use-site target '@receiver'")
		}
		unsafe = unsafe || a.Is("UnsafeVariance")
		noInfer = noInfer || a.Name == "NoInfer" || a.Is("internal.NoInfer")
		composable = composable || composableAnno(a)
		if w := WildcardsOf([]Annotation{a}); w != DefaultWildcards {
			wild = w
		}
	}
	if unsafe || composable || noInfer || wild != DefaultWildcards {
		t := p.parseType()
		t.UnsafeVariance = t.UnsafeVariance || unsafe
		t.NoInfer = t.NoInfer || noInfer
		t.Composable = t.Composable || composable
		if wild != DefaultWildcards {
			t.Wildcards = wild
		}
		return t
	}
	if t := p.tok(); t.Kind == IDENT && t.Text == "suspend" && (p.toks[p.i+1].Kind == LPAREN || p.toks[p.i+1].Kind == IDENT) {
		// A suspend function type: suspend (A) -> R, suspend R.(A) -> T.
		p.next()
		ft := p.parseType()
		if ft.Ret == nil {
			p.failAt(t.Pos, "expected a function type after 'suspend'")
		}
		ft.P, ft.Suspend = t.Pos, true
		return ft
	}
	if t := p.tok(); t.Kind == IDENT && t.Text == "context" && p.toks[p.i+1].Kind == LPAREN {
		// A context function type: context(A, B) R.(C) -> T.
		p.next()
		p.next()
		var ctx []*TypeRef
		if p.skipNL(); p.at(RPAREN) {
			p.failAt(p.tok().Pos, "expected a context parameter type")
		}
		for p.skipNL(); !p.at(RPAREN); p.skipNL() {
			ctx = append(ctx, p.parseType())
			p.skipNL()
			if !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RPAREN)
		ft := p.parseType()
		if ft.Ret == nil || ft.Nullable || len(ft.Contexts) > 0 {
			p.failAt(t.Pos, "expected a function type after the context parameters")
		}
		ft.P, ft.Contexts = t.Pos, ctx
		return ft
	}
	if p.at(LPAREN) {
		return p.parseParenType()
	}
	t := p.ident()
	ref := &TypeRef{P: t.Pos, Name: t.Text}
	for p.at(DOT) && p.toks[p.i+1].Kind != LPAREN {
		p.next()
		ref.Name += "." + p.ident().Text
	}
	if p.at(LT) {
		ref.Args = p.parseTypeArgs()
	}
	// The type of an inner class of a generic class: Outer<A>.Inner<B>.
	for ref.Args != nil && p.at(DOT) && p.toks[p.i+1].Kind == IDENT {
		p.next()
		outer := ref
		ref = &TypeRef{P: outer.P, Name: outer.Name + "." + p.ident().Text, Outer: outer}
		for p.at(DOT) && p.toks[p.i+1].Kind == IDENT {
			p.next()
			ref.Name += "." + p.ident().Text
		}
		if p.at(LT) {
			ref.Args = p.parseTypeArgs()
		}
	}
	if p.at(QUEST) {
		p.next()
		ref.Nullable = true
	}
	if p.at(AMP) {
		// A definitely non-nullable type, T & Any, which the checker
		// checks.
		p.next()
		ref.And = p.parseType()
	}
	if p.at(SAFE_DOT) && p.toks[p.i+1].Kind == LPAREN {
		// A nullable receiver, R?.(A) -> T: ?. is one token.
		ref.Nullable = true
		p.toks[p.i].Kind = DOT
	}
	if p.at(DOT) && p.toks[p.i+1].Kind == LPAREN {
		// A function type with a receiver: R.(A) -> T.
		p.next()
		ft := p.parseParenType()
		if ft.Ret == nil || ft.Recv != nil {
			p.failAt(ft.P, "expected a function type after the receiver type")
		}
		ft.P, ft.Recv = ref.P, ref
		return ft
	}
	return ref
}

// parseParenType parses a function type, (A, B) -> R, or a type in
// parentheses, as in ((A) -> R)?. Parameters may be named: (x: A) -> R.
func (p *parser) parseParenType() *TypeRef {
	pos := p.expect(LPAREN).Pos
	var params []*TypeRef
	for {
		p.skipNL()
		if p.at(RPAREN) {
			break
		}
		if p.at(IDENT) && p.toks[p.i+1].Kind == COLON {
			p.i += 2
		}
		params = append(params, p.parseType())
		p.skipNL()
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN)
	if p.peekPastNL() == ARROW {
		p.skipNL()
		p.next()
		p.skipNL()
		return &TypeRef{P: pos, Params: params, Ret: p.parseType()}
	}
	if len(params) != 1 {
		p.unexpected("'->'")
	}
	t := params[0]
	if p.at(QUEST) {
		p.next()
		t.Nullable = true
	}
	if p.at(DOT) && p.toks[p.i+1].Kind == LPAREN {
		// A receiver in parentheses: (A).(B) -> R.
		p.next()
		ft := p.parseParenType()
		if ft.Ret == nil || ft.Recv != nil {
			p.failAt(ft.P, "expected a function type after the receiver type")
		}
		ft.P, ft.Recv = pos, t
		return ft
	}
	return t
}

// parseTypeArgs parses <A, out B, in C, *>.
func (p *parser) parseTypeArgs() []*TypeArg {
	p.expect(LT)
	var args []*TypeArg
	for {
		p.skipNL()
		if p.at(GT) && args != nil {
			break // a trailing comma
		}
		a := &TypeArg{P: p.tok().Pos}
		switch {
		case p.at(STAR):
			p.next()
			a.Variance = types.Star
		default:
			a.Variance = p.variance()
			a.Type = p.parseType()
		}
		args = append(args, a)
		p.skipNL()
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(GT)
	return args
}

// variance parses an optional `in` or `out` before a type argument or
// type parameter. `out` is only a keyword when a name follows it.
func (p *parser) variance() types.Variance {
	switch t := p.tok(); {
	case t.Kind == IN:
		p.next()
		return types.In
	case t.Kind == IDENT && t.Text == "out" && (p.toks[p.i+1].Kind == IDENT || p.toks[p.i+1].Kind == AT):
		p.next()
		return types.Out
	}
	return types.Inv
}

// parseTypeParams parses <T, out U : Bound>.
func (p *parser) parseTypeParams() []*TypeParam {
	p.expect(LT)
	var ps []*TypeParam
	for {
		p.skipNL()
		if p.at(GT) && ps != nil {
			break // a trailing comma
		}
		for p.at(AT) {
			p.annotation() // ignored: kc writes no type annotations
			p.skipNL()
		}
		reified := false
		if t := p.tok(); t.Kind == IDENT && t.Text == "reified" {
			reified = true
			p.next()
		}
		tp := &TypeParam{Variance: p.variance(), Reified: reified}
		name := p.ident()
		tp.P, tp.Name = name.Pos, name.Text
		if p.peekPastNL() == COLON {
			p.skipNL()
			p.next()
			p.skipNL()
			tp.Bounds = []*TypeRef{p.parseType()}
		}
		ps = append(ps, tp)
		p.skipNL()
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(GT)
	return ps
}

// typeArgsAhead reports whether the tokens at p.i are type arguments of
// a call, as in f<Int>(x): a balanced <...> of tokens that can appear in
// types, followed by '('. Otherwise '<' is a comparison. Types can be
// annotated, as in mutableListOf<@Composable () -> Unit>(), and long lists
// wrapped over lines.
func (p *parser) typeArgsAhead() bool {
	depth := 0
	for j := p.i; ; j++ {
		switch p.toks[j].Kind {
		case LT:
			depth++
		case GT:
			depth--
			if depth == 0 {
				k := p.toks[j+1].Kind
				// f<T> sc@{ ... } has a labeled trailing lambda.
				return k == LPAREN || k == LBRACE || k == NL && p.peekAfterNL(j+1) == LBRACE ||
					k == LABEL && p.toks[j+2].Kind == LBRACE
			}
		case AT:
			// An annotation, whose arguments can be any expressions.
			for p.toks[j+1].Kind == IDENT || p.toks[j+1].Kind == DOT || p.toks[j+1].Kind == COLON {
				j++
			}
			if p.toks[j+1].Kind == LPAREN {
				for n := 0; ; {
					j++
					if k := p.toks[j].Kind; k == LPAREN {
						n++
					} else if k == RPAREN {
						if n--; n == 0 {
							break
						}
					} else if k == EOF {
						return false
					}
				}
			}
		case IDENT, DOT, COMMA, QUEST, STAR, IN, LPAREN, RPAREN, ARROW, COLON, AMP, NL:
		default:
			return false
		}
	}
}

// classRefAhead reports whether type arguments and :: follow, as in
// List<*>::class or Map.Entry<K, V>::key.
func (p *parser) classRefAhead() bool {
	depth := 0
	for j := p.i; ; j++ {
		switch p.toks[j].Kind {
		case LT:
			depth++
		case GT:
			depth--
			if depth == 0 {
				return p.toks[j+1].Kind == COLON_COLON
			}
		case IDENT, DOT, COMMA, QUEST, STAR, IN, LPAREN, RPAREN, ARROW, AMP:
		default:
			return false
		}
	}
}

// funInterface parses a fun interface, after its modifiers: an interface
// with a single abstract method, to which lambdas convert.
func (p *parser) funInterface(m Modifiers) *ClassDecl {
	p.expect(FUN)
	cd := p.parseClass(m)
	cd.Fun = true
	return cd
}

func (p *parser) parseFun() *FunDecl {
	pos := p.expect(FUN).Pos
	if t := p.tok(); t.Kind == INTERFACE {
		p.failAt(t.Pos, "'interface' is not allowed here: local interfaces are not supported")
	}
	var tps []*TypeParam
	p.skipNL()
	if p.at(LT) {
		tps = p.parseTypeParams()
		p.skipNL()
	}
	recv, recvAs := p.receiver(true)
	name := p.ident()
	fd := &FunDecl{P: pos, Name: name.Text, TypeParams: tps, Recv: recv, RecvAnnotations: recvAs, Params: p.params()}
	// The return type can go on the next line, and its ':' too.
	if p.peekPastNL() == COLON {
		p.skipNL()
		p.next()
		p.skipNL()
		fd.Ret = p.parseType()
	}
	p.whereClause(tps)
	switch p.peekPastNL() {
	case LBRACE:
		p.skipNL()
		fd.Body = p.parseBlock()
	case ASSIGN:
		p.skipNL()
		p.next()
		p.skipNL()
		fd.ExprBody = p.parseExpr()
	default:
		// No body: an abstract function.
		if !p.at(NL) && !p.at(SEMI) && !p.at(RBRACE) && !p.at(EOF) {
			p.unexpected("function body")
		}
	}
	return fd
}

// params parses the parameters of a function or a secondary constructor,
// in parentheses.
func (p *parser) params() []*Param {
	var ps []*Param
	p.expect(LPAREN)
	for !p.at(RPAREN) {
		m := p.modifiers()
		pn := p.ident()
		p.expect(COLON)
		param := &Param{P: pn.Pos, Name: pn.Text, Type: p.parseType(), Vararg: m.Vararg, Noinline: m.Noinline, Crossinline: m.Crossinline,
			Annotations: m.Annotations}
		param.External, param.ExternalP = m.External, m.ExternalP
		if p.at(ASSIGN) {
			p.next()
			p.skipNL()
			param.Default = p.parseExpr()
		}
		ps = append(ps, param)
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN)
	return ps
}

// secondaryCtor parses a secondary constructor, after its modifiers:
// constructor(x: Int) : this(x, 0) { ... }.
func (p *parser) secondaryCtor(m Modifiers) *SecondaryCtor {
	sc := &SecondaryCtor{Modifiers: m, P: p.next().Pos}
	sc.Params = p.params()
	if p.peekPastNL() == COLON {
		p.skipNL()
		p.next()
		p.skipNL()
		t := p.tok()
		if t.Kind != THIS && t.Kind != SUPER {
			p.unexpected("'this' or 'super'")
		}
		p.next()
		id := &Ident{Name: t.Kind.String()}
		id.P = t.Pos
		sc.Delegation = p.parseCall(id, nil).(*Call)
		sc.ToThis = t.Kind == THIS
	}
	if p.peekPastNL() == LBRACE {
		p.skipNL()
		sc.Body = p.parseBlock()
	}
	return sc
}

// receiver parses the receiver type of an extension function or property,
// and the dot after it, if the declaration has one: String in
// fun String.f(), List<T>? in val <T> List<T>?.x. The receiver ends at the
// last dot (or ?.) before the name, which is followed by the parameters of
// a function, or by what follows a property's name. A receiver that is a
// function type is in parentheses: fun ((Int) -> Int).f().
// It also returns the annotations before the type with the use-site
// target receiver, which are the receiver parameter's.
func (p *parser) receiver(fun bool) (*TypeRef, []Annotation) {
	var as []Annotation
	for start := p.i; p.at(AT); start = p.i {
		a := p.annotation()
		if a.Target != "receiver" {
			p.i = start // the type's
			break
		}
		as = append(as, a)
	}
	start := p.i
	for p.at(AT) {
		p.annotation() // parseType's
	}
	dot := p.receiverDot(fun)
	p.i = start
	if dot < 0 {
		if as != nil {
			p.unexpected("a receiver type")
		}
		return nil, nil
	}
	// Parse the type up to the dot, which parseType would otherwise take
	// as part of a qualified name.
	kind := p.toks[dot].Kind
	p.toks[dot].Kind = SEMI
	defer func() { p.toks[dot].Kind = kind }() // also when parseType fails
	ref := p.parseType()
	if p.i != dot {
		p.unexpected("'.' before the name")
	}
	p.next()
	if kind == SAFE_DOT {
		ref.Nullable = true // ?. is one token
	}
	return ref, as
}

// receiverDot returns the index of the token that separates a receiver
// type from a declaration's name, or -1 if the declaration at p.i has no
// receiver. Angle brackets and parentheses nest; at the outer level, a
// function's name is the name before the parenthesis that opens its
// parameters, and a property's the last name before its type, initializer,
// accessors or the end of the line.
func (p *parser) receiverDot(fun bool) int {
	depth := 0
	for j := p.i; ; j++ {
		switch k := p.toks[j].Kind; {
		case k == EOF:
			return -1
		case k == LT || k == LPAREN && (!fun || depth > 0 || j == p.i || p.toks[j-1].Kind != IDENT):
			depth++
			continue
		case k == GT || k == RPAREN:
			if depth--; depth < 0 {
				return -1
			}
			continue
		case depth > 0:
			continue
		case fun && k == LPAREN:
		case !fun && (k == COLON || k == ASSIGN || k == NL || k == SEMI || k == RBRACE ||
			k == IDENT && j > p.i && p.toks[j-1].Kind == IDENT):
		default:
			continue
		}
		// j follows the name.
		if j-2 < p.i || p.toks[j-1].Kind != IDENT {
			return -1
		}
		if k := p.toks[j-2].Kind; k == DOT || k == SAFE_DOT {
			return j - 2
		}
		return -1
	}
}

// whereClause parses a generic where clause, which can start on the next
// line: where T : A, T : B, U : C. Its bounds are added to those of the
// declaration's type parameters tps.
func (p *parser) whereClause(tps []*TypeParam) {
	j := p.i
	for p.toks[j].Kind == NL {
		j++
	}
	if t := p.toks[j]; t.Kind != IDENT || t.Text != "where" {
		return
	}
	p.i = j + 1
	for {
		p.skipNL()
		for p.at(AT) {
			p.annotation() // ignored, as on type parameters
			p.skipNL()
		}
		name := p.ident()
		var tp *TypeParam
		for _, q := range tps {
			if q.Name == name.Text {
				tp = q
			}
		}
		if tp == nil {
			p.failAt(name.Pos, "'%s' does not refer to a type parameter of this declaration", name.Text)
		}
		p.skipNL()
		p.expect(COLON)
		p.skipNL()
		tp.Bounds = append(tp.Bounds, p.parseType())
		if p.peekPastNL() != COMMA {
			return
		}
		p.skipNL()
		p.next()
	}
}

// delegated returns a property's declarations: for a delegated property,
// its field, x$delegate, then the property itself, with accessors that
// call the delegate. A class's passes this as the delegate's thisRef, and
// an extension property its receiver. As in kotlinc, the fields of
// extension properties with the same name, among the declarations
// before, are x$delegate$1, x$delegate$2, ...
func delegated(pd *PropDecl, member bool, before []Decl) []Decl {
	if pd.Field == nil {
		return []Decl{pd}
	}
	taken := func(name string) bool {
		for _, d := range before {
			if d, ok := d.(*PropDecl); ok && d.DelegateOf != nil && d.Name == name {
				return true
			}
		}
		return false
	}
	for i := 1; taken(pd.Field.Name); i++ {
		pd.Field.Name = pd.Name + "$delegate$" + strconv.Itoa(i)
	}
	pos := pd.Field.P
	ref := &PropRef{Name: pd.Name, Mutable: pd.Mutable, Recv: member, Ext: pd.Recv != nil, Index: -1}
	ref.P = pos
	pd.Ref = ref
	call := func(name string, args ...Expr) *Call {
		var this Expr
		if member || pd.Recv != nil {
			t := &This{}
			t.P = pos
			this = t
		} else {
			n := &NullLit{}
			n.P = pos
			this = n
		}
		id := &Ident{Name: pd.Field.Name}
		id.P = pos
		var field Expr = id
		if member && pd.Recv != nil {
			// The class's field, not the receiver's, which may be of
			// the same class.
			t := &This{Class: true}
			t.P = pos
			f := &Member{X: t, Name: pd.Field.Name}
			f.P = pos
			field = f
		}
		m := &Member{X: field, Name: name}
		m.P = pos
		c := &Call{Fun: m, Args: append([]Expr{this, ref}, args...)}
		c.P = pos
		return c
	}
	pd.Getter = &FunDecl{P: pos, Name: "get", Property: pd.Name, Ret: pd.Type, ExprBody: call("getValue")}
	if pd.Mutable {
		value := &Ident{Name: "value"}
		value.P = pos
		pd.Setter = &FunDecl{P: pos, Name: "set", Property: pd.Name, Params: []*Param{{P: pos, Name: "value", Type: pd.Type}},
			ExprBody: call("setValue", value)}
	}
	return []Decl{pd.Field, pd}
}

func (p *parser) parseProp(m Modifiers) *PropDecl {
	kw := p.next()
	var tps []*TypeParam
	switch {
	case p.at(LT):
		tps = p.parseTypeParams()
	case p.at(LPAREN) && p.receiverDot(false) < 0: // not a receiver: val (Int.() -> T).x
		p.unsupported(p.tok().Pos, "destructuring declaration", "destructuring declarations are not supported")
	}
	recv, recvAs := p.receiver(false)
	if tps != nil && recv == nil && m.Contexts == nil {
		p.failAt(kw.Pos, "type parameters of a property must be used in its receiver type")
	}
	name := p.ident()
	pd := &PropDecl{Modifiers: m, P: kw.Pos, Name: name.Text, Mutable: kw.Kind == VAR, TypeParams: tps, Recv: recv, RecvAnnotations: recvAs}
	// The type can go on the next line.
	if p.at(COLON) {
		p.next()
		p.skipNL()
		pd.Type = p.parseType()
	}
	p.whereClause(tps)
	// by can go on the next line.
	if t := p.toks[p.i]; t.Kind == NL {
		if j := p.i + 1; p.peekAfterNL(j) == IDENT {
			for p.toks[j].Kind == NL {
				j++
			}
			if p.toks[j].Text == "by" {
				p.i = j
			}
		}
	}
	if t := p.tok(); t.Kind == IDENT && t.Text == "by" {
		if m.Contexts != nil {
			p.failAt(t.Pos, "delegated property cannot have context parameters")
		}
		p.next()
		p.skipNL()
		pd.Field = &PropDecl{P: t.Pos, Name: pd.Name + "$delegate", Init: p.parseExpr(), DelegateOf: pd}
		p.accessors(pd)
		for _, a := range []*FunDecl{pd.Getter, pd.Setter} {
			if a != nil {
				p.failAt(a.P, "delegated property cannot have accessors with non-default implementations")
			}
		}
		return pd
	}
	if p.peekPastNL() == ASSIGN {
		p.skipNL()
		p.next()
		p.skipNL()
		pd.Init = p.parseExpr()
	}
	p.accessors(pd)
	return pd
}

// accessors parses a property's getter and setter, which can follow it on
// the next lines: get() = ..., get() { ... }, private set. kc supports
// getters with a body, and accessors without one, which only change
// visibility or annotations, or are external. An inline accessor is an ordinary one, whose
// metadata says it is inline. An accessor's annotations, which can be on
// the lines before it, are the property's, with the accessor as their
// target, as @get:A is.
func (p *parser) accessors(pd *PropDecl) {
	for {
		j := p.i
		for p.toks[j].Kind == NL || p.toks[j].Kind == SEMI {
			j++
		}
		start := j
		for {
			if t := p.toks[j]; t.Kind == AT {
				j = p.annotationEnd(j)
				for p.toks[j].Kind == NL {
					j++
				}
			} else if t.Kind == IDENT && (t.Text == "private" || t.Text == "protected" || t.Text == "internal" || t.Text == "public") && p.toks[j+1].Kind == IDENT {
				j++
			} else if t.Kind == IDENT && (t.Text == "inline" || t.Text == "external") && p.toks[j+1].Kind == IDENT {
				j++
			} else if t.Kind == IDENT && t.Text == "context" && p.toks[j+1].Kind == LPAREN {
				for j = p.matching(j+1) + 1; p.toks[j].Kind == NL; j++ {
				}
			} else {
				break
			}
		}
		t := &p.toks[j]
		if t.Kind != IDENT || t.Text != "get" && t.Text != "set" {
			return
		}
		switch p.toks[j+1].Kind {
		case LPAREN, NL, SEMI, RBRACE, EOF:
		default:
			return
		}
		p.i = start
		m := p.modifiers()
		p.noContexts(&m, "property accessors")
		for _, a := range m.Annotations {
			if a.Target == "" {
				a.Target = t.Text
			}
			pd.Annotations = append(pd.Annotations, a)
		}
		if t.Text == "set" {
			pd.SetterVis = m.Visibility
			pd.InlineSetter = m.Inline
		} else {
			pd.InlineGetter = m.Inline
		}
		if p.toks[j+1].Kind != LPAREN {
			p.i = j + 1 // an accessor without a body
			if m.External {
				p.externalAccessor(pd, t, m)
			}
			continue
		}
		if t.Text == "set" {
			p.i = j + 1
			pd.Setter = p.setter(pd, t.Pos, m)
			continue
		}
		if pd.Getter != nil {
			p.failAt(t.Pos, "property has two getters")
		}
		p.i = j + 1
		p.expect(LPAREN)
		p.expect(RPAREN)
		g := &FunDecl{P: t.Pos, Name: "get", Property: pd.Name}
		g.External, g.ExternalP = m.External, m.ExternalP
		if p.at(COLON) {
			p.next()
			g.Ret = p.parseType()
			if pd.Type == nil {
				pd.Type = g.Ret // val x get(): T { ... } is a T
			}
		}
		switch p.peekPastNL() {
		case LBRACE:
			p.skipNL()
			g.Body = p.parseBlock()
		case ASSIGN:
			p.skipNL()
			p.next()
			p.skipNL()
			g.ExprBody = p.parseExpr()
		default:
			p.unexpected("getter body")
		}
		pd.Getter = g
	}
}

// externalAccessor declares an external accessor without a body,
// external get or external set: a native method, which takes the value
// as value.
func (p *parser) externalAccessor(pd *PropDecl, t *Token, m Modifiers) {
	a := &FunDecl{P: t.Pos, Name: t.Text, Property: pd.Name}
	a.External, a.ExternalP = true, m.ExternalP
	switch {
	case t.Text == "get" && pd.Getter != nil:
		p.failAt(t.Pos, "property has two getters")
	case t.Text == "get":
		pd.Getter = a
	case !pd.Mutable:
		p.failAt(t.Pos, "a 'val'-property cannot have a setter")
	case pd.Setter != nil:
		p.failAt(t.Pos, "property has two setters")
	default:
		a.Visibility = m.Visibility
		a.Params = []*Param{{P: t.Pos, Name: "value"}}
		pd.Setter = a
	}
}

// setter parses a setter with a body after set: (value) { ... },
// (v: T) = ... Its parameter's type, which may be omitted, is the
// property's.
func (p *parser) setter(pd *PropDecl, pos Pos, m Modifiers) *FunDecl {
	switch {
	case !pd.Mutable:
		p.failAt(pos, "a 'val'-property cannot have a setter")
	case pd.Setter != nil:
		p.failAt(pos, "property has two setters")
	}
	st := &FunDecl{P: pos, Name: "set", Property: pd.Name}
	st.Visibility = m.Visibility
	st.External, st.ExternalP = m.External, m.ExternalP
	p.expect(LPAREN)
	p.skipNL()
	for p.at(AT) {
		p.annotation() // ignored, as on locals
		p.skipNL()
	}
	name := p.ident()
	param := &Param{P: name.Pos, Name: name.Text}
	if p.at(COLON) {
		p.next()
		param.Type = p.parseType()
	}
	st.Params = []*Param{param}
	p.skipNL()
	if p.at(COMMA) {
		p.next()
		p.skipNL()
	}
	p.expect(RPAREN)
	if p.at(COLON) {
		p.next()
		if t := p.parseType(); t.Name != "Unit" || t.Nullable {
			p.failAt(t.P, "setter return type must be Unit")
		}
	}
	switch p.peekPastNL() {
	case LBRACE:
		p.skipNL()
		st.Body = p.parseBlock()
	case ASSIGN:
		p.skipNL()
		p.next()
		p.skipNL()
		st.ExprBody = p.parseExpr()
	default:
		p.unexpected("setter body")
	}
	return st
}

// parseTypeAlias parses typealias Name<T> = Type, after its modifiers.
func (p *parser) parseTypeAlias(m Modifiers) *TypeAliasDecl {
	d := &TypeAliasDecl{Modifiers: m, P: p.expect(TYPEALIAS).Pos, Name: p.ident().Text}
	if p.at(LT) {
		d.TypeParams = p.parseTypeParams()
	}
	p.expect(ASSIGN)
	p.skipNL()
	d.Type = p.parseType()
	return d
}

// parseClass parses a class, an interface or an object declaration. A
// companion object's name is optional: it is Companion by default.
func (p *parser) parseClass(m Modifiers) *ClassDecl {
	kw := p.next()
	if m.Inline && kw.Kind == CLASS {
		// The deprecated inline class is a value class, which kotlinc
		// annotates @JvmInline.
		m.Annotations = append(m.Annotations, Annotation{P: kw.Pos, Name: "kotlin.jvm.JvmInline", Implicit: true})
		m.Value, m.JvmInline = true, true
	}
	cd := &ClassDecl{Modifiers: m, P: kw.Pos, Interface: kw.Kind == INTERFACE, Object: kw.Kind == OBJECT}
	defer func() {
		// A class without secondary constructors has a primary one, if
		// only the implicit one without parameters.
		if !cd.HasPrimary && cd.Secondaries() == nil {
			cd.HasPrimary = true
		}
	}()
	switch {
	case m.Companion && kw.Kind != OBJECT:
		p.failAt(kw.Pos, "modifier 'companion' is only allowed on objects")
	case m.Companion && !p.at(IDENT):
		cd.Name = "Companion"
	default:
		// The name can go on the next line: kotlinpoet wraps long ones.
		p.skipNL()
		cd.Name = p.ident().Text
	}
	if p.at(LT) {
		cd.TypeParams = p.parseTypeParams()
	}
	// The header can go on after a newline, before the primary constructor
	// or the supertypes.
	if p.at(NL) {
		j := p.i
		for p.toks[j].Kind == NL {
			j++
		}
		if k := p.toks[j].Kind; k == LPAREN || k == COLON || p.primaryCtorAt(j) {
			p.i = j
		}
	}
	// The primary constructor's modifiers, as in
	// class A private constructor(...).
	if t := p.tok(); t.Kind == AT || t.Kind == IDENT && t.Text != "where" {
		cd.CtorMods = p.modifiers()
		p.noContexts(&cd.CtorMods, "primary constructors")
		if t := p.tok(); t.Kind == IDENT && t.Text == "constructor" {
			p.next()
		}
	}
	if p.at(LPAREN) {
		cd.HasPrimary = true
		p.next()
		for !p.at(RPAREN) {
			m := p.modifiers()
			cp := &CtorParam{Modifiers: m}
			if p.at(VAL) || p.at(VAR) {
				cp.IsProp = true
				cp.Mutable = p.next().Kind == VAR
			}
			pn := p.ident()
			p.expect(COLON)
			cp.P, cp.Name, cp.Type, cp.Param.Vararg = pn.Pos, pn.Text, p.parseType(), m.Vararg
			if !cp.IsProp {
				cp.Param.External, cp.Param.ExternalP = m.External, m.ExternalP
			}
			if p.at(ASSIGN) {
				p.next()
				p.skipNL()
				cp.Default = p.parseExpr()
			}
			cd.CtorParams = append(cd.CtorParams, cp)
			if !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RPAREN)
	}
	if p.peekPastNL() == COLON {
		p.skipNL()
		p.next()
		p.supertypes(cd)
	}
	p.whereClause(cd.TypeParams)
	if p.peekPastNL() != LBRACE {
		return cd
	}
	p.skipNL()
	p.expect(LBRACE)
	if cd.Enum {
		p.enumEntries(cd)
	}
	p.skipSemis()
	for !p.at(RBRACE) && !p.at(EOF) {
		p.decl(func() { p.member(cd) })
		p.skipSemis()
	}
	p.expect(RBRACE)
	return cd
}

// objectExpr parses an object expression, object : A(x), I { body }.
func (p *parser) objectExpr() *ObjectExpr {
	kw := p.next()
	cd := &ClassDecl{P: kw.Pos, Anonymous: true, HasPrimary: true}
	if p.peekPastNL() == COLON {
		p.skipNL()
		p.next()
		// The body's brace isn't a trailing lambda.
		saved := p.noTrailing
		p.noTrailing = true
		p.supertypes(cd)
		p.noTrailing = saved
	}
	if p.peekPastNL() == LBRACE {
		p.skipNL()
		p.next()
		p.skipSemis()
		for !p.at(RBRACE) && !p.at(EOF) {
			p.decl(func() { p.member(cd) })
			p.skipSemis()
		}
		p.expect(RBRACE)
	}
	x := &ObjectExpr{Decl: cd}
	x.P = kw.Pos
	return x
}

// enumEntryEnd reports whether an enum entry's name can be followed by k.
func enumEntryEnd(k Kind) bool {
	switch k {
	case LPAREN, COMMA, SEMI, RBRACE, NL, LBRACE:
		return true
	}
	return false
}

// enumEntries parses the entries at the start of an enum class's body:
// RED, GREEN(0x00ff00), separated by commas, and ended by ';' if members
// follow.
func (p *parser) enumEntries(cd *ClassDecl) {
	for {
		p.skipNL()
		var anns []Annotation
		for p.at(AT) {
			anns = append(anns, p.annotation())
			p.skipNL()
		}
		if t := p.tok(); t.Kind == IDENT && t.Text == "context" && p.toks[p.i+1].Kind == LPAREN {
			j := p.matching(p.i+1) + 1
			for p.toks[j].Kind == NL {
				j++
			}
			if p.toks[j].Kind == IDENT && enumEntryEnd(p.toks[j+1].Kind) {
				p.contextParams()
				p.errorLater(t.Pos, "context parameters on enum entries are unsupported")
				p.skipNL()
			}
		}
		if !p.at(IDENT) {
			break
		}
		if p.tok().Text == "external" && p.toks[p.i+1].Kind == IDENT && enumEntryEnd(p.toks[p.i+2].Kind) {
			p.next() // deprecated on an entry, where it means nothing
		}
		if !enumEntryEnd(p.toks[p.i+1].Kind) {
			return // a member
		}
		name := p.next()
		e := &EnumEntry{P: name.Pos, Name: name.Text, Annotations: anns}
		id := &Ident{Name: cd.Name}
		id.P = name.Pos
		e.Call = &Call{Fun: id}
		e.Call.P = name.Pos
		if p.at(LPAREN) {
			e.Call = p.parseCall(id, nil).(*Call)
		}
		if p.at(LBRACE) {
			e.Body = p.enumEntryBody(cd, e)
			cd.Members = append(cd.Members, e.Body)
		}
		cd.Entries = append(cd.Entries, e)
		p.skipNL()
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.skipNL()
	if p.at(SEMI) {
		p.next()
	}
}

// enumEntryBody parses the body of entry e of enum class cd: the class
// of the entry, a subclass of the enum, whose superclass's constructor
// call is the entry's.
func (p *parser) enumEntryBody(cd *ClassDecl, e *EnumEntry) *ClassDecl {
	body := &ClassDecl{P: e.P, Name: e.Name, HasPrimary: true, EnumEntry: true}
	body.Supers = []*SuperType{{Type: &TypeRef{P: e.P, Name: cd.Name}, Call: e.Call}}
	p.expect(LBRACE)
	p.skipSemis()
	for !p.at(RBRACE) && !p.at(EOF) {
		p.decl(func() { p.member(body) })
		p.skipSemis()
	}
	p.expect(RBRACE)
	return body
}

// supertypes parses a class's supertypes, after the ':': B(x), I, J<T>.
func (p *parser) supertypes(cd *ClassDecl) {
	for {
		p.skipNL()
		st := &SuperType{Type: p.parseType()}
		if p.at(LPAREN) {
			st.Call = p.parseCall(typeExpr(st.Type), st.Type.Args).(*Call)
		}
		j := p.i
		for p.toks[j].Kind == NL {
			j++
		}
		if t := p.toks[j]; t.Kind == IDENT && t.Text == "by" {
			p.i = j // by can go on the next line
			if st.Call != nil {
				p.unexpected("',' or '{'")
			}
			p.next()
			p.skipNL()
			// The class body's brace isn't a trailing lambda.
			saved := p.noTrailing
			p.noTrailing = true
			st.Delegate = p.parseExpr()
			p.noTrailing = saved
		}
		cd.Supers = append(cd.Supers, st)
		j = p.i
		for p.toks[j].Kind == NL {
			j++
		}
		if p.toks[j].Kind != COMMA {
			return
		}
		p.i = j + 1
	}
}

// typeExpr returns the expression that names a class: B or a.b.B, for
// the constructor call of a supertype.
func typeExpr(t *TypeRef) Expr {
	parts := strings.Split(t.Name, ".")
	id := &Ident{Name: parts[0]}
	id.P = t.P
	var x Expr = id
	for _, name := range parts[1:] {
		m := &Member{X: x, Name: name}
		m.P = t.P
		x = m
	}
	return x
}

// member parses a class member.
func (p *parser) member(cd *ClassDecl) {
	m := p.modifiers()
	p.paramOnly(m)
	switch t := p.tok(); t.Kind {
	case FUN:
		if p.toks[p.i+1].Kind == INTERFACE {
			p.noContexts(&m, "classes")
			cd.Members = append(cd.Members, p.funInterface(m))
			break
		}
		fd := p.parseFun()
		fd.Modifiers = m
		cd.Members = append(cd.Members, fd)
	case VAL, VAR:
		pd := p.parseProp(m)
		cd.Members = append(cd.Members, delegated(pd, true, cd.Members)...)
	case CLASS, INTERFACE, OBJECT:
		p.noContexts(&m, "classes")
		cd.Members = append(cd.Members, p.parseClass(m))
	case TYPEALIAS:
		p.noContexts(&m, "type aliases")
		cd.Members = append(cd.Members, p.parseTypeAlias(m))
	case IDENT:
		if t.Text == "constructor" && p.toks[p.i+1].Kind == LPAREN {
			p.noContexts(&m, "constructors")
			cd.Members = append(cd.Members, p.secondaryCtor(m))
			break
		}
		if t.Text == "init" && p.peekAfterNL(p.i+1) == LBRACE {
			// init is a soft keyword: elsewhere, it is a name.
			p.noContexts(&m, "initializers")
			p.next()
			p.skipNL()
			cd.Members = append(cd.Members, &InitBlock{P: t.Pos, Body: p.parseBlock()})
			break
		}
		p.danglingContexts(m)
		p.unexpected("member declaration")
	default:
		p.danglingContexts(m)
		p.unexpected("member declaration")
	}
	p.endOfStatement()
}

// primaryCtorAt reports whether the tokens at j start a primary
// constructor with modifiers or annotations: private constructor(...).
func (p *parser) primaryCtorAt(j int) bool {
	for {
		switch t := p.toks[j]; {
		case t.Kind == IDENT && t.Text == "constructor":
			return true
		case t.Kind == IDENT && (t.Text == "private" || t.Text == "public" || t.Text == "internal" || t.Text == "protected" || t.Text == "actual"):
			j++
		case t.Kind == AT:
			j = p.annotationEnd(j)
			for p.toks[j].Kind == NL {
				j++
			}
		default:
			return false
		}
	}
}

// annotation parses an annotation: @Name, @a.b.Name(args), @file:Name(args)
// or @[A B]. The arguments are kept as text: kc reads only a few
// annotations, whose arguments are literals or names. An annotation in
// brackets has an empty Name.
func (p *parser) annotation() Annotation {
	var a Annotation
	a.P = p.expect(AT).Pos
	if p.at(IDENT) && p.toks[p.i+1].Kind == COLON {
		a.Target = p.next().Text
		p.next()
	}
	if p.at(LBRACK) {
		p.i = p.matching(p.i) + 1
		return a
	}
	last := p.ident()
	a.Name = last.Text
	for p.at(DOT) {
		p.next()
		last = p.ident()
		a.Name += "." + last.Text
	}
	if p.at(LT) {
		p.parseTypeArgs()
		last = p.toks[p.i-1]
	}
	// Arguments follow the name with no space between: @A(x), not @A (x).
	if t := p.tok(); t.Kind == LPAREN && t.Pos.Line == last.Pos.Line && t.Pos.Col == last.Pos.Col+utf8.RuneCountInString(last.Text) {
		end := p.matching(p.i)
		a.Args = p.annotationArgs(p.i+1, end)
		p.i = end + 1
	}
	if a.Name == "Suppress" || a.Name == "kotlin.Suppress" {
		for _, arg := range a.Args {
			for _, t := range arg.Toks {
				for _, part := range t.Parts {
					switch part.Lit {
					case "INVISIBLE_MEMBER", "INVISIBLE_REFERENCE":
						p.suppressInvisible = true
					case "RECEIVER_SHADOWED_BY_CONTEXT_PARAMETER":
						p.suppressShadowed = true
					case "DSL_SCOPE_VIOLATION":
						p.suppressDSL = true
					}
				}
			}
		}
	}
	return a
}

// annotationArgs splits the tokens from i to end, an annotation's
// arguments, at the commas between them.
func (p *parser) annotationArgs(i, end int) []AnnotationArg {
	var args []AnnotationArg
	for i < end {
		for p.toks[i].Kind == NL {
			i++
		}
		if i >= end {
			break
		}
		var arg AnnotationArg
		if p.toks[i].Kind == IDENT && p.toks[i+1].Kind == ASSIGN {
			arg.Name = p.toks[i].Text
			i += 2
		}
		start := i
		for i < end && p.toks[i].Kind != COMMA {
			switch p.toks[i].Kind {
			case LPAREN, LBRACK, LBRACE:
				i = p.matching(i)
			}
			i++
		}
		for _, t := range p.toks[start:i] {
			if t.Kind == NL {
				continue
			}
			arg.Toks = append(arg.Toks, t)
		}
		args = append(args, arg)
		i++ // the comma
	}
	return args
}

// ParseAnnotationArg parses an annotation's argument: an expression, or
// an array literal [a, b], whose elements it returns with array set.
func ParseAnnotationArg(path string, arg *AnnotationArg) (elems []Expr, array bool, err *Error) {
	toks := arg.Toks
	if len(toks) == 0 {
		return nil, false, nil
	}
	end := Token{Kind: EOF, Pos: toks[len(toks)-1].Pos}
	p := &parser{file: path, toks: append(slices.Clone(toks), end)}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			elems, err = nil, p.err
		}
	}()
	if p.at(LBRACK) {
		array = true
		p.next()
		for !p.at(RBRACK) {
			elems = append(elems, p.parseExpr())
			if !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RBRACK)
	} else if p.at(STAR) {
		// *arrayOf(a, b), a vararg parameter's elements
		pos := p.next().Pos
		sp := &Spread{X: p.parseExpr()}
		sp.P = pos
		elems = []Expr{sp}
	} else {
		elems = []Expr{p.parseExpr()}
	}
	if !p.at(EOF) {
		p.unexpected("the end of the argument")
	}
	return elems, array, nil
}

// annotationEnd returns the index of the token after the annotation at j,
// without reading it.
func (p *parser) annotationEnd(j int) int {
	saved := p.i
	defer func() { p.i = saved }()
	p.i = j
	p.annotation()
	return p.i
}

// matching returns the index of the token that closes the bracket at j.
func (p *parser) matching(j int) int {
	open := p.toks[j].Kind
	close := map[Kind]Kind{LPAREN: RPAREN, LBRACK: RBRACK, LBRACE: RBRACE}[open]
	depth := 0
	for ; p.toks[j].Kind != EOF; j++ {
		switch p.toks[j].Kind {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	p.failAt(p.toks[j].Pos, "expected '%s'", close)
	return j
}

func (p *parser) parseBlock() *Block {
	b := &Block{P: p.expect(LBRACE).Pos}
	p.skipSemis()
	for !p.at(RBRACE) {
		b.Stmts = append(b.Stmts, p.parseStmt())
		p.endOfStatement()
		p.skipSemis()
	}
	b.End = p.next().Pos
	return b
}

// parseBody parses a control structure body: a block or a single statement.
func (p *parser) parseBody() *Block {
	pos := p.tok().Pos
	if p.at(SEMI) {
		return &Block{P: pos} // an empty body: while (f());
	}
	p.skipNL()
	pos = p.tok().Pos
	// A brace that starts with lambda parameters is a lambda, as in
	// else { x -> x }: Kotlin prefers a block only without them. So is an
	// annotated one, -> @Suppress("x") { ... }, whose annotations are
	// dropped.
	start := p.i
	for p.at(AT) {
		p.annotation()
		p.skipNL()
	}
	if p.at(LBRACE) && !p.lambdaParamsAt(p.i+1) {
		return p.parseBlock()
	}
	p.i = start
	return &Block{P: pos, Stmts: []Stmt{p.parseStmt()}}
}

func (p *parser) parseStmt() Stmt {
	// Annotations on local declarations and expressions are ignored, but
	// for a local class's, which LocalClass keeps, and a local function's
	// and a lambda's, which the checker and codegen look at for Compose's.
	var as []Annotation
	for p.at(AT) {
		as = append(as, p.annotation())
		p.skipNL()
	}
	s := p.parseStmtAfterAnnotations()
	switch s := s.(type) {
	case *LocalClass:
		s.Annotations = append(as, s.Annotations...)
	case *LocalFun:
		s.Decl.Annotations = append(as, s.Decl.Annotations...)
	case *ExprStmt:
		if l, ok := s.X.(*Lambda); ok {
			for _, a := range as {
				annotateLambda(l, a)
			}
		}
	}
	return s
}

// annotateLambda records annotation a of lambda l if it is Compose's
// @Composable or @DontMemoize.
func annotateLambda(l *Lambda, a Annotation) {
	switch {
	case composableAnno(a):
		l.Annotated = true
	case a.Name == "DontMemoize" || a.Name == "androidx.compose.runtime.DontMemoize":
		l.DontMemoize = true
	}
}

// composableAnno reports whether a is Compose's @Composable.
func composableAnno(a Annotation) bool {
	return a.Name == "Composable" || a.Name == "androidx.compose.runtime.Composable"
}

func (p *parser) parseStmtAfterAnnotations() Stmt {
	switch t := p.tok(); t.Kind {
	case VAL, VAR:
		p.next()
		if p.at(LPAREN) {
			// val (a, b) = e
			d := &Destructure{P: t.Pos, Mutable: t.Kind == VAR}
			d.Names, d.Types = p.destructuringNames()
			p.expect(ASSIGN)
			p.skipNL()
			d.Init = p.parseExpr()
			return d
		}
		name := p.ident()
		vs := &VarStmt{P: t.Pos, Name: name.Text, Mutable: t.Kind == VAR}
		if p.at(COLON) {
			p.next()
			p.skipNL()
			vs.Type = p.parseType()
		}
		if t := p.tok(); t.Kind == IDENT && t.Text == "by" {
			p.next()
			p.skipNL()
			vs.Delegate = p.parseExpr()
			return vs
		}
		if !p.at(ASSIGN) {
			// Assigned later: the checker checks that it is before it is
			// read.
			if vs.Type == nil {
				p.failAt(name.Pos, "this variable must either have an explicit type or be initialized")
			}
			return vs
		}
		p.next()
		p.skipNL()
		vs.Init = p.parseExpr()
		return vs
	case FOR:
		p.next()
		p.expect(LPAREN)
		f := &For{P: t.Pos}
		if p.at(LPAREN) {
			f.Names, f.Types = p.destructuringNames() // for ((k, v) in m)
		} else {
			f.Var = p.ident().Text
			if p.at(COLON) {
				p.next()
				f.Type = p.parseType()
			}
		}
		p.expect(IN)
		f.Range = p.parseExpr()
		p.expect(RPAREN)
		f.Body = p.parseBody()
		return f
	case WHILE:
		p.next()
		p.expect(LPAREN)
		cond := p.parseExpr()
		p.expect(RPAREN)
		return &While{P: t.Pos, Cond: cond, Body: p.parseBody()}
	case DO:
		p.next()
		body := &Block{P: p.tok().Pos} // do while (f()) has no body
		if !p.at(WHILE) {
			body = p.parseBody()
		}
		p.skipNL()
		p.expect(WHILE)
		p.expect(LPAREN)
		cond := p.parseExpr()
		p.expect(RPAREN)
		return &While{P: t.Pos, Do: true, Cond: cond, Body: body}
	case LABEL:
		// loop@ for, loop@ while, loop@ do, also with the loop on the
		// next line.
		switch p.peekAfterNL(p.i + 1) {
		case FOR, WHILE, DO:
			p.next()
			p.skipNL()
			switch s := p.parseStmt().(type) {
			case *For:
				s.Label = t.Text
				return s
			case *While:
				s.Label = t.Text
				return s
			}
		}
	case FUN:
		if k := p.toks[p.i+1].Kind; (k == IDENT || k == LT) && !p.anonFunAt(p.i) {
			return p.localFun(Modifiers{})
		}
	case IDENT:
		if t.Text == "context" && p.contextDecl() {
			return p.localFun(p.modifiers())
		}
		// Modifiers of a local function: infix fun, private fun.
		j := p.i
		for p.toks[j].Kind == IDENT && isModifier(p.toks[j].Text) {
			j++
		}
		if j > p.i {
			switch p.toks[j].Kind {
			case FUN:
				return p.localFun(p.modifiers())
			case CLASS, INTERFACE, OBJECT:
				// data class, open class, ... in a block.
				return p.localClass(p.modifiers())
			case VAL, VAR:
				// lateinit var x: T, or annotations and modifiers that
				// mean nothing on locals.
				m := p.modifiers()
				s := p.parseStmt()
				if vs, ok := s.(*VarStmt); ok {
					vs.Lateinit, vs.LateinitP = m.Lateinit, t.Pos
				}
				return s
			}
		}
	case CLASS, INTERFACE:
		return p.localClass(Modifiers{})
	case OBJECT:
		if p.toks[p.i+1].Kind == IDENT {
			p.failAt(t.Pos, "named object '%s' cannot be local: use an anonymous object instead", p.toks[p.i+1].Text)
		}
	}
	x := p.parseExpr()
	switch k := p.tok().Kind; k {
	case ASSIGN, PLUS_EQ, MINUS_EQ, STAR_EQ, SLASH_EQ, PERCENT_EQ:
		pos := p.next().Pos
		p.skipNL()
		return &Assign{P: pos, Op: k, Target: x, Value: p.parseExpr()}
	}
	return &ExprStmt{X: x}
}

// Expressions, from lowest to highest precedence.

func (p *parser) parseExpr() Expr { return p.parseDisjunction() }

func (p *parser) binary(op Token, x, y Expr) Expr {
	b := &Binary{Op: op.Kind, X: x, Y: y}
	b.P = op.Pos
	return b
}

func (p *parser) parseDisjunction() Expr {
	x := p.parseConjunction()
	for p.peekPastNL() == OR {
		p.skipNL()
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseConjunction())
	}
	return x
}

func (p *parser) parseConjunction() Expr {
	x := p.parseEquality()
	for p.peekPastNL() == AND {
		p.skipNL()
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseEquality())
	}
	return x
}

func (p *parser) parseEquality() Expr {
	x := p.parseComparison()
	for p.at(EQ) || p.at(NE) || p.at(EQ_EQ_EQ) || p.at(NOT_EQ_EQ) {
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseComparison())
	}
	return x
}

func (p *parser) parseComparison() Expr {
	x := p.parseNamedCheck()
	for p.at(LT) || p.at(GT) || p.at(LE) || p.at(GE) {
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseNamedCheck())
	}
	return x
}

func (p *parser) parseNamedCheck() Expr {
	x := p.parseElvis()
	for p.at(IN) || p.at(NOT_IN) || p.at(IS) || p.at(NOT_IS) {
		op := p.next()
		p.skipNL()
		if op.Kind == IS || op.Kind == NOT_IS {
			is := &Is{X: x, Ref: p.parseType(), Not: op.Kind == NOT_IS}
			is.P = op.Pos
			x = is
			continue
		}
		x = p.binary(op, x, p.parseElvis())
	}
	return x
}

func (p *parser) parseElvis() Expr {
	x := p.parseInfix()
	for p.peekPastNL() == ELVIS {
		p.skipNL()
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseInfix())
	}
	return x
}

// parseInfix parses infix calls, a name b, which are a.name(b). The range
// forms until, downTo and step are operators.
func (p *parser) parseInfix() Expr {
	x := p.parseRange()
	for p.at(IDENT) {
		var k Kind
		switch p.tok().Text {
		case "until":
			k = UNTIL
		case "downTo":
			k = DOWN_TO
		case "step":
			k = STEP
		default:
			name := p.next()
			p.skipNL()
			m := &Member{X: x, Name: name.Text}
			m.P = name.Pos
			c := &Call{Fun: m, Args: []Expr{p.parseRange()}, Infix: true}
			c.P = name.Pos
			x = c
			continue
		}
		op := p.next()
		op.Kind = k
		p.skipNL()
		x = p.binary(op, x, p.parseRange())
	}
	return x
}

func (p *parser) parseRange() Expr {
	x := p.parseAdditive()
	for p.at(RANGE) || p.at(RANGE_LT) {
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseAdditive())
	}
	return x
}

func (p *parser) parseAdditive() Expr {
	x := p.parseMultiplicative()
	for p.at(PLUS) || p.at(MINUS) {
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseMultiplicative())
	}
	return x
}

func (p *parser) parseMultiplicative() Expr {
	x := p.parseAs()
	for p.at(STAR) || p.at(SLASH) || p.at(PERCENT) {
		op := p.next()
		p.skipNL()
		x = p.binary(op, x, p.parseAs())
	}
	return x
}

// parseAs parses x as T and x as? T. A newline may come before as.
func (p *parser) parseAs() Expr {
	x := p.parsePrefix()
	for k := p.peekPastNL(); k == AS || k == AS_SAFE; k = p.peekPastNL() {
		p.skipNL()
		op := p.next()
		p.skipNL()
		as := &As{X: x, Ref: p.parseType(), Safe: op.Kind == AS_SAFE}
		as.P = op.Pos
		x = as
	}
	return x
}

func (p *parser) parsePrefix() Expr {
	switch t := p.tok(); t.Kind {
	case MINUS, PLUS, NOT:
		op := p.next()
		x := p.parsePrefix()
		if lit, ok := x.(*IntLit); ok && op.Kind == MINUS && !lit.Unsigned && lit.Val > 0 && p.toks[p.i-1].Kind == INT {
			// As K2, a negative literal is one literal, not (-1) or -(1):
			// -1 may be a Short and -2147483648 is an Int.
			lit.Val = -lit.Val
			lit.Long = strings.HasSuffix(p.toks[p.i-1].Text, "L") || lit.Val < math.MinInt32
			lit.P = op.Pos
			return lit
		}
		u := &Unary{Op: op.Kind, X: x}
		u.P = op.Pos
		return u
	case NOT_NULL:
		// !!x before an operand is two prefix nots.
		op := p.next()
		inner := &Unary{Op: NOT, X: p.parsePrefix()}
		inner.P = Pos{op.Pos.Line, op.Pos.Col + 1}
		u := &Unary{Op: NOT, X: inner}
		u.P = op.Pos
		return u
	case INC, DEC:
		op := p.next()
		u := &Unary{Op: op.Kind, X: p.parsePrefix(), Prefix: true}
		u.P = op.Pos
		return u
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() Expr {
	var paren Expr // the primary expression, if it is in parentheses
	inParens := p.at(LPAREN)
	x := p.parsePrimary()
	if inParens {
		paren = x
	}
	for {
		switch t := p.tok(); t.Kind {
		case LPAREN:
			x = p.parseCall(x, nil)
		case LT:
			switch x.(type) {
			case *Ident, *Member:
				if name, ok := dottedName(x); ok && p.classRefAhead() {
					// List<*>::class: the type arguments of a class name.
					ref := &TypeRef{P: x.Pos(), Name: name, Args: p.parseTypeArgs()}
					x = p.parseColonColon(x, ref)
					continue
				}
				if p.typeArgsAhead() {
					x = p.parseCall(x, p.parseTypeArgs())
					continue
				}
			}
			return x
		case NL:
			switch k := p.peekPastNL(); {
			case k == DOT || k == SAFE_DOT:
			case k == LBRACE && !p.noTrailing && trailableAfterNL(x):
				// As with kotlinc, a lambda on the next line after a
				// call's arguments is a trailing lambda: Kotlin has no
				// blocks as statements.
			default:
				return x
			}
			p.skipNL()
		case DOT, SAFE_DOT:
			op := p.next()
			p.skipNL()
			if op.Kind == DOT && p.at(LPAREN) {
				// r.(f)(args) calls an extension function value with
				// receiver r: it is (f)(r, args), and like kotlinc it
				// evaluates f before r.
				fn := p.parsePrimary()
				if !p.at(LPAREN) && !p.at(LBRACE) {
					p.expect(LPAREN)
				}
				c := p.parseCall(fn, nil).(*Call)
				c.Args = append([]Expr{x}, c.Args...)
				c.Recv = true
				if c.Names != nil {
					c.Names = append([]string{""}, c.Names...)
				}
				x = c
				continue
			}
			name := p.ident()
			m := &Member{X: x, Name: name.Text, Safe: op.Kind == SAFE_DOT}
			m.P = name.Pos
			x = m
		case LBRACK:
			p.next()
			ix := &Index{X: x}
			ix.P = t.Pos
			for {
				p.skipNL()
				ix.Args = append(ix.Args, p.parseExpr())
				p.skipNL()
				if !p.at(COMMA) {
					break
				}
				p.next()
				if p.peekPastNL() == RBRACK {
					p.skipNL() // a trailing comma
					break
				}
			}
			p.expect(RBRACK)
			x = ix
		case COLON_COLON:
			x = p.parseColonColon(x, nil)
		case QUEST:
			// A callable reference on a nullable type: String?::plus.
			name, ok := dottedName(x)
			if !ok || p.toks[p.i+1].Kind != COLON_COLON {
				return x
			}
			p.next()
			x = p.parseColonColon(x, &TypeRef{P: x.Pos(), Name: name, Nullable: true})
		case NOT_NULL:
			p.next()
			n := &NotNull{X: x}
			n.P = t.Pos
			x = n
		case INC, DEC:
			p.next()
			u := &Unary{Op: t.Kind, X: x}
			u.P = t.Pos
			x = u
		case LBRACE, LABEL, AT:
			// A brace after a call or name is a trailing lambda, which may
			// have a label, and annotations before them.
			if p.noTrailing || t.Kind == LABEL && p.toks[p.i+1].Kind != LBRACE {
				return x
			}
			// After an expression in parentheses, (a to b) { ... }, the
			// lambda is passed to its value's invoke.
			if x != paren && !trailable(x) {
				return x
			}
			fn, ok := x.(*Call)
			ok = ok && x != paren
			start := p.i
			var as []Annotation
			for p.at(AT) {
				as = append(as, p.annotation())
			}
			if !p.at(LBRACE) && (!p.at(LABEL) || p.toks[p.i+1].Kind != LBRACE) {
				p.i = start
				return x
			}
			l := p.trailingLambda()
			for _, a := range as {
				annotateLambda(l, a)
			}
			if !ok {
				fn = &Call{Fun: x}
				fn.P = t.Pos
				x = fn
			}
			fn.Args = append(fn.Args, l)
			fn.trailing = true
			labelLambdas(fn)
		default:
			return x
		}
	}
}

// trailingLambda parses a trailing lambda, with its label if it has one.
func (p *parser) trailingLambda() *Lambda {
	if t := p.tok(); t.Kind == LABEL {
		p.next()
		l := p.parseLambda()
		l.Label = t.Text
		return l
	}
	return p.parseLambda()
}

func (p *parser) parseCall(fn Expr, targs []*TypeArg) Expr {
	c := &Call{Fun: fn, TypeArgs: targs}
	if targs != nil && p.at(NL) && p.peekPastNL() == LBRACE {
		p.skipNL() // f<T>\n{ ... }
	}
	if p.at(LBRACE) || targs != nil && p.at(LABEL) { // f<T> { ... }, f<T> sc@{ ... }
		c.P = p.tok().Pos
		return c
	}
	c.P = p.expect(LPAREN).Pos
	saved := p.noTrailing
	p.noTrailing = false
	defer func() { p.noTrailing = saved }()
	for !p.at(RPAREN) {
		if t := p.tok(); t.Kind == IDENT && p.toks[p.i+1].Kind == ASSIGN {
			// A named argument: name = value.
			for len(c.Names) < len(c.Args) {
				c.Names = append(c.Names, "")
			}
			c.Names = append(c.Names, t.Text)
			p.i += 2
			p.skipNL()
		}
		if t := p.tok(); t.Kind == STAR {
			// The spread operator: *xs passes an array's elements.
			p.next()
			s := &Spread{X: p.parseExpr()}
			s.P = t.Pos
			c.Args = append(c.Args, s)
		} else {
			c.Args = append(c.Args, p.parseExpr())
		}
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN)
	labelLambdas(c)
	return c
}

// parseColonColon parses ::class after x, or after the generic class name
// ref, with x its name.
func (p *parser) parseColonColon(x Expr, ref *TypeRef) Expr {
	t := p.expect(COLON_COLON)
	if p.at(CLASS) {
		p.next()
		cl := &ClassLit{Ref: ref}
		if ref == nil {
			cl.X = x
		}
		cl.P = t.Pos
		return cl
	}
	return p.callableRef(t.Pos, &CallableRef{X: x, Type: ref, Name: p.ident().Text})
}

// callableRef returns a callable reference, as a lambda for the checker to
// fill in.
func (p *parser) callableRef(pos Pos, ref *CallableRef) *Lambda {
	if ref.Type != nil {
		ref.X = nil
	}
	l := &Lambda{Arrow: true, Body: &Block{}, Ref: ref}
	l.P = pos
	l.Body.P = pos
	return l
}

// trailable reports whether a lambda after x is a trailing lambda: x is
// a call without one yet, a name, or an indexing, a[i] { ... }, which
// calls the result's invoke.
func trailable(x Expr) bool {
	switch x := x.(type) {
	case *Call:
		return !x.trailing
	case *Member, *Ident, *Index:
		return true
	}
	return false
}

// trailableAfterNL reports whether a lambda on the line after x is a
// trailing lambda: x is a call with arguments in parentheses, f(x), and
// without one yet. After a name, as kotlinc's parser, the line ends the
// expression: val u = Unit, then a lambda.
func trailableAfterNL(x Expr) bool {
	c, ok := x.(*Call)
	return ok && !c.trailing
}

// dottedName returns the name a.b.C that x spells, if it is one.
func dottedName(x Expr) (string, bool) {
	switch x := x.(type) {
	case *Ident:
		return x.Name, true
	case *Member:
		if x.Safe {
			return "", false
		}
		s, ok := dottedName(x.X)
		return s + "." + x.Name, ok
	}
	return "", false
}

// labelLambdas gives the lambdas passed to a call the function's name as
// their label, for return@name.
func labelLambdas(c *Call) {
	var name string
	switch fn := c.Fun.(type) {
	case *Ident:
		name = fn.Name
	case *Member:
		name = fn.Name
	default:
		return
	}
	for _, a := range c.Args {
		if l, ok := a.(*Lambda); ok && l.Label == "" {
			l.Label = name
		}
	}
}

func (p *parser) parsePrimary() Expr {
	t := p.tok()
	switch t.Kind {
	case INT:
		p.next()
		return p.intLit(*t)
	case DOUBLE:
		p.next()
		v, err := strconv.ParseFloat(strings.ReplaceAll(t.Text, "_", ""), 64)
		if err != nil {
			p.failAt(t.Pos, "invalid double literal")
		}
		d := &DoubleLit{Val: v}
		d.P = t.Pos
		return d
	case STRING:
		p.next()
		return p.stringLit(*t)
	case TRUE, FALSE:
		p.next()
		b := &BoolLit{Val: t.Kind == TRUE}
		b.P = t.Pos
		return b
	case NULL:
		p.next()
		n := &NullLit{}
		n.P = t.Pos
		return n
	case LBRACK:
		// A collection literal, [a, b], which Kotlin allows in annotations:
		// arrayOf(a, b).
		p.next()
		fn := &Ident{Name: "arrayOf"}
		fn.P = t.Pos
		call := &Call{Fun: fn, Literal: true}
		call.P = t.Pos
		p.skipNL()
		for !p.at(RBRACK) {
			call.Args = append(call.Args, p.parseExpr())
			p.skipNL()
			if !p.at(COMMA) {
				break
			}
			p.next()
			p.skipNL()
		}
		p.expect(RBRACK)
		return call
	case THIS:
		p.next()
		th := &This{}
		if _, label, ok := strings.Cut(t.Text, "@"); ok {
			th.Label = label
		}
		th.P = t.Pos
		return th
	case IDENT:
		if t.Text == "context" && p.contextAnonFun() {
			// context(a: A) fun () = a: an anonymous function with
			// context parameters.
			ctx := p.contextParams()
			p.skipNL()
			l := p.anonFun("")
			l.P, l.AnonCtx = t.Pos, ctx
			return l
		}
		if t.Text == "suspend" && p.toks[p.i+1].Kind == LBRACE {
			// suspend { x: Int -> x }, a lambda with the suspend modifier,
			// as kotlinc's ParseLambdaWithSuspendModifier parses it, rather
			// than a call of kotlin.suspend.
			p.next()
			l := p.parseLambda()
			l.P, l.SuspendMod = t.Pos, true
			return l
		}
		p.next()
		id := &Ident{Name: t.Text}
		id.P = t.Pos
		return id
	case LPAREN:
		p.next()
		x := p.parseExpr()
		p.expect(RPAREN)
		return x
	case IF:
		return p.parseIf()
	case WHEN:
		return p.parseWhen()
	case RETURN, BREAK, CONTINUE:
		p.next()
		j := &Jump{Kind: t.Kind}
		j.P = t.Pos
		if _, label, ok := strings.Cut(t.Text, "@"); ok {
			j.Label = label
		}
		if t.Kind == RETURN {
			switch p.tok().Kind {
			case NL, SEMI, RBRACE, RPAREN, COMMA, EOF:
			default:
				j.X = p.parseExpr()
			}
		}
		return j
	case LBRACE:
		return p.parseLambda()
	case CHAR:
		p.next()
		r, msg := charValue(t.Text)
		if msg != "" {
			p.failAt(t.Pos, "%s", msg)
		}
		c := &CharLit{Val: r}
		c.P = t.Pos
		return c
	case FLOAT:
		p.next()
		text := strings.TrimRight(strings.ReplaceAll(t.Text, "_", ""), "fF")
		v, err := strconv.ParseFloat(text, 32)
		if err != nil {
			p.failAt(t.Pos, "invalid float literal")
		}
		f := &FloatLit{Val: float32(v)}
		f.P = t.Pos
		return f
	case UNSIGNED:
		p.next()
		return p.intLit(*t)
	case SUPER:
		p.next()
		x := &Super{}
		x.P = t.Pos
		_, x.Label, _ = strings.Cut(t.Text, "@")
		if p.at(LT) {
			p.next()
			x.Iface = p.parseType()
			p.expect(GT)
			if p.at(AT) && x.Label == "" {
				// super<I>@Outer: the label after the type.
				p.next()
				x.Label = p.ident().Text
			}
		}
		return x
	case THROW:
		p.next()
		p.skipNL()
		j := &Jump{Kind: THROW, X: p.parseExpr()}
		j.P = t.Pos
		return j
	case TRY:
		return p.parseTry()
	case OBJECT:
		return p.objectExpr()
	case FUN:
		return p.anonFun("")
	case COLON_COLON:
		p.next()
		if p.at(CLASS) {
			p.failAt(t.Pos, "a class literal needs a class or a value before ::")
		}
		return p.callableRef(t.Pos, &CallableRef{Name: p.ident().Text})
	case AT:
		// An annotated expression, whose annotation can be on the line
		// before it: the annotation is ignored, but for Compose's on a
		// lambda (see annotateLambda).
		a := p.annotation()
		p.skipNL()
		x := p.parsePrefix()
		if l, ok := x.(*Lambda); ok {
			annotateLambda(l, a)
		}
		return x
	case LABEL:
		// lit@{ ... }: the label replaces the function's name.
		if p.toks[p.i+1].Kind == LBRACE {
			return p.trailingLambda()
		}
		if p.toks[p.i+1].Kind == FUN {
			p.next()
			return p.anonFun(t.Text)
		}
		// l@ x: a label on any other expression has no effect, as nothing
		// can jump to it.
		p.next()
		p.skipNL()
		return p.parsePrefix()
	}
	p.unexpected("expression")
	return nil
}

// parseLambda parses { a, b: T -> body }, { -> body } or { body }.
func (p *parser) parseLambda() *Lambda {
	l := &Lambda{}
	l.P = p.expect(LBRACE).Pos
	saved := p.noTrailing
	p.noTrailing = false
	defer func() { p.noTrailing = saved }()
	p.skipNL()
	switch t := p.tok(); {
	case t.Kind == ARROW:
		p.next()
		l.Arrow = true
	case p.lambdaParamsAt(p.i):
		for {
			p.skipNL()
			var lp *LambdaParam
			if p.at(LPAREN) {
				lp = &LambdaParam{P: p.tok().Pos}
				lp.Names, lp.Types = p.destructuringNames()
			} else {
				name := p.ident()
				lp = &LambdaParam{P: name.Pos, Name: name.Text}
			}
			if p.at(COLON) {
				p.next()
				lp.Type = p.parseType()
			}
			l.Params = append(l.Params, lp)
			p.skipNL()
			if !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(ARROW)
		l.Arrow = true
	}
	l.Body = &Block{P: p.tok().Pos}
	p.skipSemis()
	for !p.at(RBRACE) {
		l.Body.Stmts = append(l.Body.Stmts, p.parseStmt())
		p.endOfStatement()
		p.skipSemis()
	}
	l.Body.End = p.next().Pos
	return l
}

func (p *parser) intLit(t Token) Expr {
	text := strings.ReplaceAll(t.Text, "_", "")
	long := strings.HasSuffix(text, "L")
	text = strings.TrimSuffix(text, "L")
	unsigned := t.Kind == UNSIGNED
	if unsigned {
		text = strings.TrimRight(text, "uU")
	}
	base := 10
	switch {
	case strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X"):
		base, text = 16, text[2:]
	case strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0B"):
		base, text = 2, text[2:]
	}
	v, err := strconv.ParseUint(text, base, 64)
	if err != nil || v > math.MaxInt64 && !unsigned {
		p.failAt(t.Pos, "the value is out of range")
	}
	lit := &IntLit{Val: intbits.FromBits(v), Unsigned: unsigned}
	lit.P = t.Pos
	lit.Long = long || v > math.MaxInt32 && !unsigned || v > math.MaxUint32
	return lit
}

func (p *parser) stringLit(t Token) Expr {
	s := &StringLit{}
	s.P = t.Pos
	for _, part := range t.Parts {
		if !part.IsExpr {
			s.Parts = append(s.Parts, TmplPart{Lit: part.Lit})
			continue
		}
		toks, errs := tokenizeAt(p.file, part.Src, part.SrcPos)
		if len(errs) > 0 {
			p.err = errs[0]
			panic(bailout{})
		}
		sub := &parser{file: p.file, toks: toks}
		func() {
			defer func() {
				if r := recover(); r != nil {
					p.err = sub.err
					panic(r)
				}
			}()
			x := sub.parseExpr()
			if !sub.at(EOF) {
				sub.unexpected("'}'")
			}
			s.Parts = append(s.Parts, TmplPart{X: x})
		}()
	}
	return s
}

// destructuredParam reports whether the ( that the parser is at starts a
// lambda's destructured parameter, (a, b) ->, rather than its body: the
// matching ) is followed by -> or a comma.
// lambdaParamsAt reports whether the lambda parameters or the -> of a
// lambda start at i, the token after its brace: nothing else can start
// with a name and one of ->, ',' and ':', or with (a, b) and one of these.
func (p *parser) lambdaParamsAt(i int) bool {
	for p.toks[i].Kind == NL {
		i++
	}
	switch p.toks[i].Kind {
	case ARROW:
		return true
	case IDENT:
		k := p.toks[i+1].Kind
		return k == ARROW || k == COMMA || k == COLON || k == NL && p.peekAfterNL(i+1) == ARROW
	case LPAREN:
		saved := p.i
		p.i = i
		defer func() { p.i = saved }()
		return p.destructuredParam()
	}
	return false
}

func (p *parser) destructuredParam() bool {
	depth := 0
	for i := p.i; i < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case LPAREN:
			depth++
		case RPAREN:
			if depth--; depth == 0 {
				k := p.peekAfterNL(i + 1)
				return k == ARROW || k == COMMA || k == COLON
			}
		case LBRACE, RBRACE, EOF:
			return false
		}
	}
	return false
}

// destructuringNames parses the names of a destructuring declaration,
// (a, _, c: C), and their types, nil where omitted.
func (p *parser) destructuringNames() ([]string, []*TypeRef) {
	p.expect(LPAREN)
	var names []string
	var types []*TypeRef
	for !p.at(RPAREN) {
		names = append(names, p.ident().Text)
		var t *TypeRef
		if p.at(COLON) {
			p.next()
			t = p.parseType()
		}
		types = append(types, t)
		if !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN)
	if len(names) == 0 {
		p.unexpected("name")
	}
	return names, types
}

// parseTry parses try { } catch (e: E) { } finally { }, with any number of
// catch blocks and at most one finally block, and at least one of them.
func (p *parser) parseTry() Expr {
	x := &Try{}
	x.P = p.expect(TRY).Pos
	p.skipNL()
	x.Body = p.parseBlock()
	for {
		j := p.i
		for p.toks[j].Kind == NL {
			j++
		}
		t := p.toks[j]
		if t.Kind != IDENT || t.Text != "catch" && t.Text != "finally" || x.Finally != nil {
			break
		}
		p.i = j + 1
		p.skipNL()
		if t.Text == "finally" {
			x.Finally = p.parseBlock()
			continue
		}
		p.expect(LPAREN)
		for p.at(AT) {
			p.annotation() // ignored, as on locals
			p.skipNL()
		}
		name := p.ident()
		p.expect(COLON)
		c := &Catch{P: t.Pos, Name: name.Text, Type: p.parseType()}
		p.expect(RPAREN)
		p.skipNL()
		c.Body = p.parseBlock()
		x.Catches = append(x.Catches, c)
	}
	if len(x.Catches) == 0 && x.Finally == nil {
		p.failAt(x.P, "'try' must have at least one 'catch' or 'finally' block")
	}
	return x
}

func (p *parser) parseIf() Expr {
	n := &If{}
	n.P = p.expect(IF).Pos
	p.skipNL()
	p.expect(LPAREN)
	n.Cond = p.parseExpr()
	p.expect(RPAREN)
	n.Then = p.parseBody()
	j := p.i
	for p.toks[j].Kind == NL || p.toks[j].Kind == SEMI {
		j++
	}
	// An else on a later line followed by -> is the next when branch's.
	if p.toks[j].Kind == ELSE && p.toks[j+1].Kind != ARROW {
		p.i = j + 1
		n.Else = p.parseBody()
	}
	return n
}

func (p *parser) parseWhen() Expr {
	n := &When{}
	n.P = p.expect(WHEN).Pos
	if p.at(LPAREN) {
		p.next()
		if p.at(VAL) {
			// when (val x = e): x is the subject, in scope in the branches.
			p.next()
			n.SubjVar = p.ident().Text
			if p.at(COLON) {
				p.next()
				n.SubjType = p.parseType()
			}
			p.expect(ASSIGN)
			p.skipNL()
		}
		n.Subject = p.parseExpr()
		p.expect(RPAREN)
	}
	p.skipNL()
	p.expect(LBRACE)
	p.skipSemis()
	for !p.at(RBRACE) {
		br := &WhenBranch{P: p.tok().Pos}
		if p.at(ELSE) {
			p.next()
			br.Else = true
		} else {
			for {
				var c WhenCond
				switch p.tok().Kind {
				case IN:
					p.next()
					c.In = true
				case NOT_IN:
					p.next()
					c.NotIn = true
				case IS, NOT_IS:
					c.Is, c.NotIs = p.at(IS), p.at(NOT_IS)
					c.P = p.next().Pos
					p.skipNL()
					c.Type = p.parseType()
				}
				if c.Type == nil {
					c.X = p.parseExpr()
				}
				br.Conds = append(br.Conds, c)
				if !p.at(COMMA) {
					break
				}
				p.next()
				p.skipNL()
				if p.at(ARROW) {
					break // a trailing comma
				}
			}
		}
		if p.at(IF) {
			// A guard: is T if x.ok -> ..., else if x.ok -> ...
			p.next()
			br.Guard = p.parseExpr()
		}
		p.skipNL()
		p.expect(ARROW)
		br.Body = p.parseBody()
		n.Branches = append(n.Branches, br)
		// Entries need no separator: when { a -> x else -> y }.
		p.skipSemis()
	}
	p.expect(RBRACE)
	return n
}
