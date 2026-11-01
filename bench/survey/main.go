// Command survey reports which Kotlin features that kc doesn't support a
// real project uses, and which of them block the most files.
//
//	go run ./bench/survey -name kotlinpoet -stdlib kotlin-stdlib.jar -o report.md \
//	    main:src/commonMain,src/jvmMain tests:src/jvmTest
//
// Each argument is a source set: a label and the directories whose .kt
// files it contains. For each file, a detector that reads kc's tokens
// lists every unsupported feature the file uses, not just the first one.
// The standard library's inline-only functions, such as let and apply,
// come from its metadata. As a cross-check, kc's parser parses each file
// with error recovery: an error the detector didn't predict is listed, so
// gaps in the detector show up.
//
// The report ranks the features by the files they block, and shows how
// many files each planned milestone would unblock.
//
//	go run ./bench/survey -repos bench/survey/repos.txt -cache dir -stdlib kotlin-stdlib.jar -cp ... -o report.md
//
// surveys the repositories a manifest lists instead (see wide.go and
// bench/survey.sh).
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/inline"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// milestones groups the features by the milestone that adds them: 1 to 6
// are the class-model plan's, 7 and 8 the arrays plan's, 9 and 10 the
// inliner's (its milestones 1, 2 and 5, and 3), 11 and 12 the extensions
// plan's, 13 adds identity equality, class literals and callable
// references, 14 to 16 class delegation, secondary constructors and
// local functions, 17 to 26 are the kotlinpoet plan's, 42 to 49 the
// coroutines plan's, 52 to 63 the wide survey's, and 70 on the
// kotlinx-coroutines plan's. Features in no
// group are out of their scope.
var milestones = []struct {
	name     string
	features []string
}{
	{"1: packages, annotations", []string{"package", "annotation"}},
	{"2: interfaces, inheritance", []string{"interface", "inheritance", "modifier open", "modifier abstract",
		"modifier override", "modifier final", "modifier protected", "super", "property getter",
		"function without body", "property without initializer"}},
	{"3: type checks", []string{"is", "as", "when subject variable"}},
	{"4: exceptions", []string{"try", "throw", "require, check, error"}},
	{"5: objects", []string{"object", "modifier companion", "nested class", "modifier const"}},
	{"6: data, enum, sealed", []string{"modifier data", "modifier enum", "modifier sealed", "default value",
		"named argument", "destructuring declaration"}},
	{"7: arrays, vararg, spread", []string{"arrays", "modifier vararg", "spread operator"}},
	{"8: Char, Float, Byte, Short", []string{"Char", "Float", "Byte and Short"}},
	{"9: inliner", []string{"inline-only call"}},
	{"10: lambdas with receivers", []string{"function type with receiver", "qualified this"}},
	{"11: extensions", []string{"extension function", "extension property", "modifier infix"}},
	{"12: operator conventions", []string{"modifier operator"}},
	{"13: identity, class literals, references", []string{"identity equality", "class literal", "callable reference"}},
	{"14: class delegation", []string{"class delegation"}},
	{"15: secondary constructors", []string{"secondary constructor"}},
	{"16: local functions", []string{"local function"}},
	{"17: expect and actual", []string{"modifier expect", "modifier actual"}},
	{"18: annotation classes", []string{"modifier annotation"}},
	{"19: value classes", []string{"modifier value"}},
	{"20: unsigned integers", []string{"unsigned integers"}},
	{"definite assignment", []string{"uninitialized local variable"}},
	{"21: inline functions", []string{"modifier inline", "modifier noinline", "modifier crossinline"}},
	{"22: reified type parameters", []string{"modifier reified", "reified inline-only call"}},
	{"23: labels, do-while", []string{"labels", "do-while"}},
	{"24: delegated properties", []string{"delegated property"}},
	{"25: object expressions", []string{"object expression"}},
	{"26: destructuring lambda parameters", []string{"destructuring lambda parameter"}},
	{"42-49: coroutines", []string{"modifier suspend", "suspend function type"}},
	{"52: bitwise operators", []string{"bitwise operators"}},
	{"53: typealias", []string{"typealias"}},
	{"54: setters, backing fields", []string{"property setter", "property backing field"}},
	{"55: lateinit, tailrec", []string{"modifier lateinit", "modifier tailrec"}},
	{"56-57: collection supertypes", []string{"collection supertype"}},
	{"58: member extension functions", []string{"member extension function"}},
	{"59: reified member type parameters", []string{"reified member type parameter"}},
	{"60: inner and local classes", []string{"modifier inner", "local class"}},
	{"61: when guards, anonymous functions, local extension functions", []string{"when guard", "anonymous function", "local extension function"}},
	{"62a: @JvmField and @JvmStatic in objects", []string{"@JvmField in object", "@JvmStatic in object", "@JvmStatic property"}},
	{"62b: generic value classes", []string{"generic value class"}},
	{"63: definitely non-nullable types", []string{"definitely non-nullable type"}},
	{"70a: inline property accessors", []string{"inline property accessor"}},
	{"70b: where clauses", []string{"where clause"}},
	{"70c: fun interfaces", []string{"fun interface"}},
	{"delegated extension properties", []string{"delegated extension property"}},
	{"qualified super", []string{"qualified super"}},
	{"context parameters of top-level functions", []string{"context function", "context function type"}},
	{"context parameters of members, properties and local functions", []string{"context member", "context property", "context local function", "context anonymous function"}},
	// Supported all along, but counted by an earlier detector.
	{"also supported", []string{"prefix increment"}},
}

// intrinsics are the inline-only functions that milestone 4 builds in.
var intrinsics = map[string]bool{
	"require": true, "check": true, "error": true, "requireNotNull": true, "checkNotNull": true, "TODO": true,
}

// builtIn are inline-only functions that kc builds in: println and
// print, the array factories, and the Char and String functions of
// milestone 8.
var builtIn = map[string]bool{
	"println": true, "print": true,
	"emptyArray": true, "arrayOfNulls": true, "toTypedArray": true,
	"Char": true, "String": true, "toCharArray": true, "trim": true, "trimStart": true, "trimEnd": true,
	"isDigit": true, "isLetter": true, "isLetterOrDigit": true, "isUpperCase": true, "isLowerCase": true,
	"isTitleCase": true, "isISOControl": true, "isDefined": true, "isJavaIdentifierStart": true,
	"isJavaIdentifierPart": true, "uppercaseChar": true, "lowercaseChar": true, "titlecaseChar": true,
}

var modifierWords = map[string]bool{
	"abstract": true, "actual": true, "annotation": true, "companion": true, "const": true,
	"crossinline": true, "data": true, "enum": true, "expect": true, "external": true, "final": true,
	"infix": true, "inline": true, "inner": true, "lateinit": true, "noinline": true, "open": true,
	"operator": true, "override": true, "protected": true, "reified": true, "sealed": true,
	"suspend": true, "tailrec": true, "value": true, "vararg": true,
}

var visibility = map[string]bool{"private": true, "public": true, "internal": true, "protected": true}

// typeFeatures are names of types and functions that need a feature kc
// doesn't support.
var typeFeatures = map[string]string{
	"Char": "Char", "Float": "Float", "Byte": "Byte and Short", "Short": "Byte and Short",
	"UInt": "unsigned integers", "ULong": "unsigned integers", "UByte": "unsigned integers", "UShort": "unsigned integers",
	"Array": "arrays", "IntArray": "arrays", "LongArray": "arrays", "DoubleArray": "arrays", "BooleanArray": "arrays",
	"CharArray": "arrays", "FloatArray": "arrays", "ByteArray": "arrays", "ShortArray": "arrays",
	"arrayOf": "arrays", "arrayOfNulls": "arrays", "emptyArray": "arrays", "intArrayOf": "arrays",
	"longArrayOf": "arrays", "doubleArrayOf": "arrays", "charArrayOf": "arrays", "floatArrayOf": "arrays",
	"byteArrayOf": "arrays", "shortArrayOf": "arrays", "booleanArrayOf": "arrays",
	"toTypedArray": "arrays", "toCharArray": "arrays", "toByteArray": "arrays", "toIntArray": "arrays",
	"toLongArray": "arrays", "toDoubleArray": "arrays", "toFloatArray": "arrays", "toShortArray": "arrays",
	"toBooleanArray": "arrays",
}

var bitwise = map[string]bool{"and": true, "or": true, "xor": true, "shl": true, "shr": true, "ushr": true}

type file struct {
	path      string // as displayed
	abs       string // as given
	lines     int
	features  map[string]int // feature: occurrences, the detector's and the parser's
	detected  map[string]int // the detector's alone, for the cross-checks
	inline    map[string]int // inline-only function: calls
	parseErrs []*syntax.Error
	lexErrs   []*syntax.Error
	checker   map[string]int // with -repos: feature the checker reports: errors
}

type sourceSet struct {
	label string
	dirs  []string
	files []*file
}

func main() {
	name := flag.String("name", "project", "project name for the report")
	stdlib := flag.String("stdlib", "", "kotlin-stdlib.jar, to find the inline-only functions")
	root := flag.String("root", "", "directory that file paths in the report are relative to")
	out := flag.String("o", "", "report file (default: stdout)")
	cpFlag := flag.String("cp", "", "class path for -repos, as for kc (the standard library and kotlin-reflect)")
	repos := flag.String("repos", "", "survey the repositories this file lists (see bench/survey/repos.txt) instead of the arguments' source sets")
	cache := flag.String("cache", "", "for -repos: the directory with the clones, named name-tag")
	dir := flag.String("dir", "", "for -repos: also write each repository's report to this directory")
	checkFeatures := flag.Bool("check-features", false, "check the files whose paths are on stdin together, and print the unsupported features the checker reports (used by -repos)")
	flag.Parse()
	if *checkFeatures {
		printCheckerFeatures(*cpFlag)
		return
	}
	if flag.NArg() == 0 && *repos == "" {
		log.Fatal("usage: survey [flags] label:dir[,dir...] ...")
	}
	inlineOnly := map[string]string{}
	if *stdlib != "" {
		var err error
		if inlineOnly, err = inlineOnlyFunctions(*stdlib); err != nil {
			log.Fatal(err)
		}
	}
	if *repos != "" {
		wide(*repos, *cache, *cpFlag, *out, *dir, inlineOnly)
		return
	}
	sets, err := loadSets("", *root, flag.Args(), inlineOnly)
	if err != nil {
		log.Fatal(err)
	}
	var sb strings.Builder
	report(&sb, *name, sets, len(inlineOnly))
	if *out == "" {
		fmt.Print(sb.String())
	} else if err := os.WriteFile(*out, []byte(sb.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}

// loadSets surveys the files of the source sets that args name, each
// label:dir[,dir...]. A directory is relative to base, if set, and can
// be a glob.
func loadSets(base, root string, args []string, inlineOnly map[string]string) ([]*sourceSet, error) {
	var sets []*sourceSet
	for _, arg := range args {
		label, dirs, ok := strings.Cut(arg, ":")
		if !ok {
			return nil, fmt.Errorf("%s: want label:dir[,dir...]", arg)
		}
		s := &sourceSet{label: label}
		for _, pattern := range strings.Split(dirs, ",") {
			if base != "" {
				pattern = filepath.Join(base, pattern)
			}
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("%s: no such directory", pattern)
			}
			for _, dir := range matches {
				s.dirs = append(s.dirs, relative(root, dir))
				err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kt") {
						return err
					}
					f, err := survey(path, root, inlineOnly)
					if err == nil {
						s.files = append(s.files, f)
					}
					return err
				})
				if err != nil {
					return nil, err
				}
			}
		}
		sets = append(sets, s)
	}
	return sets, nil
}

func survey(path, root string, inlineOnly map[string]string) (*file, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	display := relative(root, path)
	f := &file{path: display, abs: path, lines: strings.Count(string(src), "\n")}
	toks, lexErrs := syntax.Tokenize(path, string(src))
	f.lexErrs = lexErrs
	d := &detector{path: display, toks: toks, features: map[string]int{}, inline: map[string]int{}, inlineOnly: inlineOnly,
		classBody: map[int]classKind{}, declParen: map[int]bool{}, enumParen: map[int]bool{}, whenBody: map[int]bool{}}
	d.run()
	f.detected, f.inline = d.features, d.inline
	if len(lexErrs) == 0 {
		_, f.parseErrs = syntax.ParseAll(path, string(src))
	}
	// The parser has the last word: a feature it reports blocks the file
	// even if the detector missed it, and so does an error that names no
	// feature, which is a construct the parser rejects within the
	// supported features.
	f.features = maps.Clone(d.features)
	for _, e := range f.parseErrs {
		if e.Feature != "" {
			if f.features[e.Feature] == 0 {
				f.features[e.Feature]++
			}
		} else {
			f.features[parserGap]++
		}
	}
	if len(lexErrs) > 0 {
		f.features[parserGap] += len(lexErrs)
	}
	return f, nil
}

// parserGap is the feature of a file with an error that the parser or
// the lexer reports without naming a feature.
const parserGap = "parser gap"

// relative returns path relative to root, if root is set.
func relative(root, path string) string {
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil {
			return rel
		}
	}
	return path
}

// detector finds the unsupported features in a file's tokens.
type detector struct {
	path       string
	i          int // the token being looked at
	toks       []syntax.Token
	features   map[string]int
	inline     map[string]int
	inlineOnly map[string]string // inline-only function: the feature a call is
	braces     []brace
	parens     []parenKind
	classBody  map[int]classKind // indices of the '{' that open class bodies
	headerEnd  int               // the end of the last class header seen
	declParen  map[int]bool      // indices of the '(' that open parameter lists
	enumParen  map[int]bool      // indices of the '(' that open enum constructors' parameters
	whenBody   map[int]bool      // indices of the '{' that open when's branches
}

type brace struct {
	class  bool // a class body
	when   bool // a when's branches
	kind   classKind
	parens int // len(parens) when it opened
}

// classKind tells what kind of class a class body belongs to.
type classKind struct {
	iface, enum, value bool
	object             bool // an object or companion object declaration, not an expression
	companion          bool
}

type parenKind uint8

const (
	callParen     parenKind = iota // a call's arguments, or a parenthesized expression
	declParen                      // a function's or constructor's parameters
	enumDeclParen                  // an enum class's constructor parameters
	annotationParen
	otherParen // if (...), for (...), ...
)

// trace, if set, makes the detector print each feature it finds.
var trace = flag.Bool("trace", false, "print each feature found, with its position")

func (d *detector) add(feature string) {
	d.features[feature]++
	if *trace {
		fmt.Fprintf(os.Stderr, "%s:%s: %s\n", d.path, d.toks[d.i].Pos, feature)
	}
}

func (d *detector) kind(i int) syntax.Kind {
	if i < 0 || i >= len(d.toks) {
		return syntax.EOF
	}
	return d.toks[i].Kind
}

func (d *detector) text(i int) string {
	if i < 0 || i >= len(d.toks) {
		return ""
	}
	return d.toks[i].Text
}

// memberLevel reports whether the tokens are at the top level of the file
// or of a class body, where declarations are members.
func (d *detector) memberLevel() bool {
	if len(d.braces) == 0 {
		return len(d.parens) == 0
	}
	top := d.braces[len(d.braces)-1]
	return top.class && len(d.parens) == top.parens
}

// inBlock reports whether the tokens are in a function body or lambda,
// outside any parentheses.
func (d *detector) inBlock() bool {
	if len(d.braces) == 0 {
		return false
	}
	top := d.braces[len(d.braces)-1]
	return !top.class && len(d.parens) == top.parens
}

// match returns the index of the token that closes the bracket at i.
func (d *detector) match(i int) int {
	open, close := d.kind(i), syntax.RPAREN
	switch open {
	case syntax.LT:
		close = syntax.GT
	case syntax.LBRACE:
		close = syntax.RBRACE
	case syntax.LBRACK:
		close = syntax.RBRACK
	}
	depth := 0
	for j := i; j < len(d.toks); j++ {
		switch d.kind(j) {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return j
			}
		case syntax.EOF:
			return j
		}
	}
	return len(d.toks) - 1
}

func (d *detector) skipNL(i int) int {
	for d.kind(i) == syntax.NL {
		i++
	}
	return i
}

func (d *detector) run() {
	for i := 0; i < len(d.toks); i++ {
		d.i = i
		t := &d.toks[i]
		next := d.kind(i + 1)
		prev := d.kind(i - 1)
		switch t.Kind {
		case syntax.STRING:
			d.templates(t)
		case syntax.LBRACE:
			kind, class := d.classBody[i]
			d.braces = append(d.braces, brace{class: class, kind: kind, when: d.whenBody[i], parens: len(d.parens)})
			if kind.enum {
				d.enumEntries(i)
			}
			// A destructuring lambda parameter: { (k, v) -> ... }.
			if j := d.skipNL(i + 1); d.kind(j) == syntax.LPAREN && !class {
				if k := d.kind(d.match(j) + 1); k == syntax.ARROW || k == syntax.COMMA {
					d.add("destructuring lambda parameter")
				}
			}
		case syntax.RBRACE:
			if len(d.braces) > 0 {
				d.braces = d.braces[:len(d.braces)-1]
			}
		case syntax.LPAREN:
			d.parens = append(d.parens, d.parenKind(i))
		case syntax.RPAREN:
			if len(d.parens) > 0 {
				d.parens = d.parens[:len(d.parens)-1]
			}
		case syntax.PACKAGE:
			d.add("package")
			i = d.lineEnd(i)
		case syntax.AT:
			d.add("annotation")
			d.jvmAnnotation(i)
		case syntax.INTERFACE:
			if prev == syntax.FUN {
				d.add("fun interface")
			} else {
				d.add("interface")
			}
			d.classDecl(i)
		case syntax.CLASS:
			if prev != syntax.COLON_COLON {
				d.classDecl(i)
			}
		case syntax.OBJECT:
			switch {
			case next == syntax.IDENT:
				if d.text(i-1) != "companion" {
					d.add("object")
				}
				d.classDecl(i)
			case d.text(i-1) == "companion":
				d.classDecl(i)
			default:
				d.add("object expression")
				d.classDecl(i)
			}
		case syntax.TYPEALIAS:
			d.add("typealias")
		case syntax.FUN:
			d.fun(i)
		case syntax.VAL, syntax.VAR:
			d.property(i)
		case syntax.FOR:
			if next == syntax.LPAREN && d.kind(i+2) == syntax.LPAREN {
				d.add("destructuring declaration")
			}
		case syntax.WHEN:
			if next == syntax.LPAREN && d.kind(i+2) == syntax.VAL {
				d.add("when subject variable")
			}
			j := i + 1
			if d.kind(j) == syntax.LPAREN {
				j = d.match(j) + 1
			}
			if j = d.skipNL(j); d.kind(j) == syntax.LBRACE {
				d.whenBody[j] = true
			}
		case syntax.IF:
			// A guard: x if cond -> ..., else if cond -> ...
			if top := len(d.braces) - 1; top >= 0 && d.braces[top].when && len(d.parens) == d.braces[top].parens &&
				prev != syntax.ARROW && prev != syntax.NL && d.arrowOnLine(i) {
				d.add("when guard")
			}
		case syntax.AMP:
			d.add("definitely non-nullable type")
		case syntax.IS, syntax.NOT_IS:
			d.add("is")
		case syntax.AS, syntax.AS_SAFE:
			d.add("as")
		case syntax.TRY:
			d.add("try")
		case syntax.THROW:
			d.add("throw")
		case syntax.SUPER:
			d.add("super")
			if strings.Contains(t.Text, "@") {
				d.add("qualified super")
			}
		case syntax.THIS:
			if strings.Contains(t.Text, "@") {
				d.add("qualified this")
			}
		case syntax.BREAK, syntax.CONTINUE, syntax.LABEL:
			switch {
			case t.Kind != syntax.LABEL:
				if strings.Contains(t.Text, "@") {
					d.add("labels")
				}
			case next == syntax.FOR || next == syntax.WHILE || next == syntax.DO || next == syntax.LBRACE:
				d.add("labels")
			default:
				d.add("labeled expression")
			}
		case syntax.COLON_COLON:
			if next == syntax.CLASS {
				d.add("class literal")
			} else {
				d.add("callable reference")
			}
		case syntax.CHAR:
			d.add("Char")
		case syntax.FLOAT:
			d.add("Float")
		case syntax.UNSIGNED:
			d.add("unsigned integers")
		case syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
			d.add("identity equality")
		case syntax.DO:
			d.add("do-while")
		case syntax.INC, syntax.DEC:
			switch prev {
			case syntax.IDENT, syntax.RPAREN, syntax.RBRACK, syntax.THIS, syntax.NOT_NULL:
			default:
				d.add("prefix increment")
			}
		case syntax.STAR:
			if (prev == syntax.LPAREN || prev == syntax.COMMA) && d.topParen() == callParen &&
				next != syntax.GT && next != syntax.COMMA && next != syntax.RPAREN {
				d.add("spread operator")
			}
		case syntax.DOT:
			// A function type with a receiver: T.() -> R.
			if next == syntax.LPAREN && (prev == syntax.IDENT || prev == syntax.GT || prev == syntax.QUEST) &&
				d.kind(d.match(i+1)+1) == syntax.ARROW {
				d.add("function type with receiver")
			}
		case syntax.IDENT:
			if t.Text == "import" && (prev == syntax.NL || prev == syntax.SEMI || i == 0) {
				i = d.lineEnd(i)
				continue
			}
			d.ident(i)
		case syntax.ASSIGN, syntax.PLUS_EQ, syntax.MINUS_EQ, syntax.STAR_EQ, syntax.SLASH_EQ, syntax.PERCENT_EQ:
			switch d.topParen() {
			case declParen, enumDeclParen:
				if t.Kind != syntax.ASSIGN {
					break
				}
				d.add("default value")
				if d.topParen() == enumDeclParen {
					d.add("enum constructor default value")
				}
			case callParen:
				if t.Kind == syntax.ASSIGN && prev == syntax.IDENT && (d.kind(i-2) == syntax.LPAREN || d.kind(i-2) == syntax.COMMA) {
					d.add("named argument")
				}
			}
			if d.inBlock() && prev == syntax.IDENT && d.kind(i-2) == syntax.SAFE_DOT {
				// a?.b = x
				d.add("safe-call assignment")
			}
		}
	}
}

// templates looks for features in a string's template expressions.
func (d *detector) templates(t *syntax.Token) {
	for _, p := range t.Parts {
		if !p.IsExpr {
			continue
		}
		toks, _ := syntax.Tokenize(d.path, p.Src)
		for i := range toks {
			if toks[i].Pos.Line == 1 {
				toks[i].Pos.Col += p.SrcPos.Col - 1
			}
			toks[i].Pos.Line += p.SrcPos.Line - 1
		}
		// An expression is never at the member level.
		sub := &detector{path: d.path, toks: toks, features: d.features, inline: d.inline, inlineOnly: d.inlineOnly,
			classBody: map[int]classKind{}, declParen: map[int]bool{}, enumParen: map[int]bool{}, whenBody: map[int]bool{}, parens: []parenKind{otherParen}}
		sub.run()
	}
}

func (d *detector) topParen() parenKind {
	if len(d.parens) == 0 {
		return otherParen
	}
	return d.parens[len(d.parens)-1]
}

// lineEnd returns the index of the newline that ends the line at i.
func (d *detector) lineEnd(i int) int {
	for d.kind(i+1) != syntax.NL && d.kind(i+1) != syntax.EOF {
		i++
	}
	return i
}

func (d *detector) parenKind(i int) parenKind {
	if d.declParen[i] {
		return declParen
	}
	switch d.kind(i - 1) {
	case syntax.IF, syntax.WHILE, syntax.FOR, syntax.WHEN:
		return otherParen
	}
	if d.text(i-1) == "catch" {
		return otherParen
	}
	// An annotation's arguments: @Name(...), @a.b.Name(...), @file:Name(...).
	j := i - 1
	for d.kind(j) == syntax.IDENT || d.kind(j) == syntax.DOT {
		j--
	}
	if j < i-1 {
		if d.kind(j) == syntax.AT || d.kind(j) == syntax.COLON && d.kind(j-1) == syntax.IDENT && d.kind(j-2) == syntax.AT {
			return annotationParen
		}
	}
	return callParen
}

// classDecl handles the header of a class, interface or object declared
// at i: where it is declared, its supertypes, and its body.
func (d *detector) classDecl(i int) {
	named := d.kind(i+1) == syntax.IDENT
	if named || d.kind(i) != syntax.OBJECT {
		switch {
		case d.memberLevel() && len(d.braces) > 0 && d.kind(i) != syntax.OBJECT:
			d.add("nested class")
		case d.inBlock() && named:
			d.add("local class")
		}
	}
	mods := d.modifiersBefore(i)
	kind := classKind{
		iface:     d.kind(i) == syntax.INTERFACE,
		enum:      mods["enum"],
		value:     mods["value"] || mods["inline"] && d.kind(i) == syntax.CLASS,
		companion: mods["companion"],
		object:    d.kind(i) == syntax.OBJECT && (named || mods["companion"]),
	}
	if mods["inline"] && d.kind(i) == syntax.CLASS {
		d.add("modifier value") // the deprecated inline class
	}
	j := i + 1
	if named {
		j++
	}
	if d.kind(j) == syntax.LT {
		if kind.value {
			d.add("generic value class")
		}
		j = d.match(j) + 1
	}
	// The header can go on after a newline, before the primary constructor
	// or the supertypes.
	if k := d.skipNL(j); d.kind(k) == syntax.LPAREN || d.kind(k) == syntax.COLON || d.primaryCtor(k) {
		j = k
	}
	// The primary constructor: private constructor(...), @Inject constructor(...).
	for d.kind(j) == syntax.IDENT && (visibility[d.text(j)] || d.text(j) == "constructor") || d.kind(j) == syntax.AT {
		if d.kind(j) == syntax.AT {
			j += 2
			for d.kind(j) == syntax.DOT {
				j += 2
			}
			if d.kind(j) == syntax.LPAREN {
				j = d.match(j) + 1
			}
			continue
		}
		j++
	}
	if d.kind(j) == syntax.LPAREN {
		d.declParen[j] = true
		if kind.enum {
			d.enumParen[j] = true
		}
		j = d.match(j) + 1
	}
	d.headerEnd = j
	if k := d.skipNL(j); d.kind(k) == syntax.COLON {
		j = k
	}
	if d.kind(j) == syntax.COLON {
		d.add("inheritance")
		start := true // at the start of a supertype
		for j++; ; j++ {
			switch d.kind(j) {
			case syntax.LT, syntax.LPAREN:
				start = false
				j = d.match(j)
				continue
			case syntax.IDENT:
				if start {
					// The supertype's name: the last of a dotted name.
					k := j
					for d.kind(k+1) == syntax.DOT && d.kind(k+2) == syntax.IDENT {
						k += 2
					}
					entry := d.text(k) == "Entry" || d.text(k) == "MutableEntry"
					if collectionTypes[d.text(k)] && (!entry || strings.HasSuffix(d.text(k-2), "Map")) {
						d.add("collection supertype")
					}
					start = false
				}
				if d.text(j) == "by" {
					d.add("class delegation")
				}
				if d.text(j) == "suspend" {
					start = true
				}
				if d.text(j) != "where" {
					continue
				}
			case syntax.COMMA:
				start = true
				continue
			case syntax.DOT, syntax.QUEST, syntax.ARROW:
				continue
			case syntax.NL:
				if k := d.kind(j + 1); k == syntax.IDENT || k == syntax.COMMA {
					continue
				}
			}
			break
		}
	}
	if d.text(j) == "where" {
		d.add("where clause")
		for d.kind(j) != syntax.LBRACE && d.kind(j) != syntax.NL && d.kind(j) != syntax.EOF {
			j++
		}
	}
	if j = d.skipNL(j); d.kind(j) == syntax.LBRACE {
		d.classBody[j] = kind
	}
}

// collectionTypes are Kotlin's collection interfaces, which kc's classes
// can't implement.
var collectionTypes = map[string]bool{
	"Iterable": true, "Collection": true, "List": true, "Set": true, "Map": true, "Iterator": true, "ListIterator": true,
	"MutableIterable": true, "MutableCollection": true, "MutableList": true, "MutableSet": true, "MutableMap": true,
	"MutableIterator": true, "MutableListIterator": true, "Entry": true, "MutableEntry": true,
	// The standard library's classes that implement them: kc's checker
	// accepts these, but the class it writes overrides their final
	// methods.
	"AbstractCollection": true, "AbstractList": true, "AbstractSet": true, "AbstractMap": true, "AbstractIterator": true,
	"AbstractMutableCollection": true, "AbstractMutableList": true, "AbstractMutableSet": true, "AbstractMutableMap": true,
}

// modifiersBefore returns the modifiers of the declaration whose keyword
// is at i, back to the start of its line or its annotations.
func (d *detector) modifiersBefore(i int) map[string]bool {
	mods := map[string]bool{}
	for j := i - 1; d.kind(j) == syntax.IDENT && (modifierWords[d.text(j)] || visibility[d.text(j)]); j-- {
		mods[d.text(j)] = true
	}
	return mods
}

// owner returns the kind of class whose body the tokens are members of.
func (d *detector) owner() (classKind, bool) {
	if !d.memberLevel() || len(d.braces) == 0 {
		return classKind{}, false
	}
	return d.braces[len(d.braces)-1].kind, true
}

// enumEntries looks at the entries of the enum class whose body opens at
// i: A, B(1), C { ... }.
func (d *detector) enumEntries(i int) {
	for j := d.skipNL(i + 1); ; j = d.skipNL(j + 1) {
		for d.kind(j) == syntax.AT {
			j = d.skipNL(d.annotationEnd(j))
		}
		if d.kind(j) != syntax.IDENT {
			return
		}
		j++
		if d.kind(j) == syntax.LPAREN {
			j = d.match(j) + 1
		}
		if d.kind(j) == syntax.LBRACE {
			d.classBody[j] = classKind{}
			j = d.match(j) + 1
		}
		if j = d.skipNL(j); d.kind(j) != syntax.COMMA {
			return
		}
	}
}

// annotationEnd returns the index after the annotation at i: @A, @a.B,
// @A(...), @get:A.
func (d *detector) annotationEnd(i int) int {
	j := i + 2
	if d.kind(j) == syntax.COLON {
		j += 2
	}
	for d.kind(j) == syntax.DOT && d.kind(j+1) == syntax.IDENT {
		j += 2
	}
	if d.kind(j) == syntax.LT {
		j = d.match(j) + 1
	}
	if d.kind(j) == syntax.LPAREN {
		j = d.match(j) + 1
	}
	return j
}

// jvmAnnotation handles the annotation at i if it is @JvmStatic or
// @JvmField where kc doesn't support them.
func (d *detector) jvmAnnotation(i int) {
	name := d.text(i + 1)
	if name != "JvmStatic" && name != "JvmField" {
		return
	}
	// The declaration it annotates.
	j := d.skipNL(d.annotationEnd(i))
	for d.kind(j) == syntax.AT || d.kind(j) == syntax.IDENT && (modifierWords[d.text(j)] || visibility[d.text(j)]) {
		if d.kind(j) == syntax.AT {
			j = d.skipNL(d.annotationEnd(j))
		} else {
			j = d.skipNL(j + 1)
		}
	}
	owner, member := d.owner()
	switch k := d.kind(j); {
	case name == "JvmStatic" && (k == syntax.VAL || k == syntax.VAR):
		d.add("@JvmStatic property")
	case name == "JvmStatic" && k == syntax.FUN && member && owner.object && !owner.companion:
		d.add("@JvmStatic in object")
	case name == "JvmField" && (k == syntax.VAL || k == syntax.VAR) && member && owner.object:
		d.add("@JvmField in object")
	}
}

// primaryCtor reports whether the tokens at j start a primary constructor
// with modifiers or annotations: private constructor(...).
func (d *detector) primaryCtor(j int) bool {
	for {
		switch {
		case d.text(j) == "constructor":
			return true
		case d.kind(j) == syntax.IDENT && visibility[d.text(j)]:
			j++
		case d.kind(j) == syntax.AT:
			j += 2
			for d.kind(j) == syntax.DOT {
				j += 2
			}
			if d.kind(j) == syntax.LPAREN {
				j = d.match(j) + 1
			}
		default:
			return false
		}
	}
}

// fun handles a function declaration at i.
func (d *detector) fun(i int) {
	j := i + 1
	if d.kind(j) == syntax.INTERFACE {
		return
	}
	owner, member := d.owner()
	if d.kind(j) == syntax.LT {
		end := d.match(j)
		for k := j; k < end; k++ {
			if d.text(k) == "reified" && member {
				d.add("reified member type parameter")
				break
			}
		}
		j = end + 1
	}
	switch {
	case d.kind(j) == syntax.LPAREN && (!d.memberLevel() || d.kind(d.match(j)+1) != syntax.ARROW):
		// fun (x: Int) = ..., also as a property's initializer; fun (A) -> B.f()
		// is an extension of a function type.
		d.add("anonymous function")
		return
	case d.inBlock():
		d.add("local function")
	}
	// The receiver and name: f, T.f, List<T>.f, String?.f, (Int) -> Unit.f.
	ext := false
	for ; ; j++ {
		switch d.kind(j) {
		case syntax.IDENT:
			continue
		case syntax.DOT, syntax.QUEST, syntax.SAFE_DOT:
			ext = true
			continue
		case syntax.LT:
			ext = true
			j = d.match(j)
			continue
		case syntax.LPAREN:
			if d.kind(d.match(j)+1) == syntax.ARROW {
				ext = true
				j = d.match(j) + 1
				continue
			}
		}
		break
	}
	if ext {
		d.add("extension function")
		if d.inBlock() {
			d.add("local extension function")
		}
		if mods := d.modifiersBefore(i); member && (owner.iface || mods["open"] || mods["abstract"] || mods["override"]) {
			d.add("member extension function")
		}
	}
	if d.kind(j) != syntax.LPAREN {
		return
	}
	d.declParen[j] = true
	j = d.match(j) + 1
	// The return type, a where clause, and the body.
	if d.kind(j) == syntax.COLON {
		for j++; ; j++ {
			switch d.kind(j) {
			case syntax.LT, syntax.LPAREN:
				j = d.match(j)
				continue
			case syntax.IDENT, syntax.DOT, syntax.QUEST, syntax.ARROW:
				if d.text(j) != "where" {
					continue
				}
			}
			break
		}
	}
	if d.text(j) == "where" {
		d.add("where clause")
		for k := d.kind(j); k != syntax.LBRACE && k != syntax.ASSIGN && k != syntax.NL && k != syntax.EOF; k = d.kind(j) {
			j++
		}
	}
	switch d.kind(d.skipNL(j)) {
	case syntax.LBRACE, syntax.ASSIGN:
	default:
		d.add("function without body")
	}
}

// property handles a val or var at i.
func (d *detector) property(i int) {
	ext := false
	switch {
	case d.kind(i+1) == syntax.LPAREN:
		d.add("destructuring declaration")
		return
	case d.kind(i+1) == syntax.LT:
		ext = true
	case d.kind(i+1) == syntax.IDENT:
		switch d.kind(i + 2) {
		case syntax.DOT, syntax.QUEST, syntax.SAFE_DOT, syntax.LT:
			ext = true
		}
	}
	if ext {
		d.add("extension property")
		if d.kind(i) == syntax.VAR {
			// A var extension property needs a setter.
			d.add("property setter")
		}
		owner, member := d.owner()
		if mods := d.modifiersBefore(i); member && (owner.iface || owner.value || mods["open"] || mods["abstract"] || mods["override"]) {
			d.add("member extension property")
		}
	}
	// A member property without an initializer is abstract, lateinit,
	// initialized in an init block, or has a getter. A local variable
	// without one is assigned later.
	local := d.inBlock()
	if !d.memberLevel() && !local {
		return
	}
	noInit := func(j int) {
		if local {
			d.add("uninitialized local variable")
		} else {
			d.noInit(j)
		}
	}
	depth := 0
	for j := i + 1; j < len(d.toks); j++ {
		switch d.kind(j) {
		case syntax.LBRACE, syntax.LPAREN, syntax.LBRACK:
			depth++
		case syntax.RBRACE, syntax.RPAREN, syntax.RBRACK:
			if depth == 0 {
				noInit(j)
				return
			}
			depth--
		case syntax.ASSIGN:
			if depth == 0 {
				return
			}
		case syntax.IDENT:
			if depth == 0 && d.text(j) == "by" {
				d.add("delegated property")
				if ext {
					d.add("delegated extension property")
				}
				return
			}
		case syntax.NL, syntax.SEMI, syntax.EOF:
			if depth == 0 {
				noInit(j)
				return
			}
		}
	}
}

// noInit counts a property without an initializer whose declaration ends
// at j, unless a getter follows it.
func (d *detector) noInit(j int) {
	j = d.skipNL(j)
	if visibility[d.text(j)] {
		j++
	}
	if d.text(j) == "get" {
		return
	}
	d.add("property without initializer")
}

// ident handles a name at i: a modifier, an accessor, a call of an
// inline-only function, and so on.
func (d *detector) ident(i int) {
	t := d.toks[i]
	prev, next := d.kind(i-1), d.kind(i+1)
	member := prev == syntax.DOT || prev == syntax.SAFE_DOT
	switch {
	case modifierWords[t.Text] && !member && declStart(next):
		if t.Text == "suspend" && next == syntax.LPAREN {
			break
		}
		d.add("modifier " + t.Text)
	case t.Text == "suspend" && next == syntax.LPAREN && !member:
		d.add("suspend function type")
	case t.Text == "context" && next == syntax.LPAREN && !member && d.contextParams(i):
		d.add(d.contextFeature(i))
	case (t.Text == "get" || t.Text == "set") && d.memberLevel():
		p := i - 1
		if visibility[d.text(p)] {
			p--
		}
		if k := d.kind(p); k != syntax.NL && k != syntax.SEMI {
			break
		}
		switch next {
		case syntax.LPAREN, syntax.ASSIGN:
			if t.Text == "get" {
				d.add("property getter")
			} else {
				d.add("property setter")
			}
			if next == syntax.LPAREN && d.usesField(d.match(i+1)+1) {
				d.add("property backing field")
			}
		}
	case t.Text == "constructor" && next == syntax.LPAREN && d.memberLevel() && i >= d.headerEnd:
		p := i - 1
		if visibility[d.text(p)] {
			p--
		}
		if k := d.kind(p); k == syntax.NL || k == syntax.SEMI || k == syntax.LBRACE {
			d.add("secondary constructor")
		}
	case t.Text == "where" && next == syntax.IDENT && d.kind(i+2) == syntax.COLON:
		d.add("where clause")
	case bitwise[t.Text] && !member:
		switch prev {
		case syntax.IDENT, syntax.RPAREN, syntax.INT, syntax.RBRACK:
			switch next {
			case syntax.IDENT, syntax.INT, syntax.LPAREN, syntax.MINUS:
				d.add("bitwise operators")
			}
		}
	case t.Text == "inv" && member && next == syntax.LPAREN:
		d.add("bitwise operators")
	}
	if t.Text == "inline" && next == syntax.IDENT && (d.text(i+1) == "get" || d.text(i+1) == "set") {
		d.add("inline property accessor")
	}
	if f, ok := typeFeatures[t.Text]; ok && (prev != syntax.DOT || strings.HasPrefix(t.Text, "to")) {
		d.add(f)
	}
	// A call of an inline-only function: let { }, x.apply { }, run<T> { }.
	call := next == syntax.LPAREN || next == syntax.LBRACE
	if next == syntax.LT {
		k := d.kind(d.match(i+1) + 1)
		call = k == syntax.LPAREN || k == syntax.LBRACE
	}
	if t.Text == "context" && call && d.kind(d.skipNL(d.match(i+1)+1)) != syntax.LBRACE {
		call = false // context parameters, not the standard library's context(a) { }
	}
	if feat := d.inlineOnly[t.Text]; feat != "" && prev != syntax.FUN && prev != syntax.COLON_COLON && call {
		d.inline[t.Text]++
		if intrinsics[t.Text] {
			d.add("require, check, error")
		} else {
			d.add(feat)
		}
	}
}

// usesField reports whether the accessor body that starts at i, after
// the parameter list, = x or { ... }, uses the backing field.
func (d *detector) usesField(i int) bool {
	if d.kind(i) == syntax.COLON { // the getter's type
		for i++; d.kind(i) == syntax.IDENT || d.kind(i) == syntax.DOT || d.kind(i) == syntax.QUEST || d.kind(i) == syntax.LT; i++ {
			if d.kind(i) == syntax.LT {
				i = d.match(i)
			}
		}
	}
	var end int
	switch d.kind(i) {
	case syntax.LBRACE:
		end = d.match(i)
	case syntax.ASSIGN:
		// An expression body: to the end of the line, past brackets.
		depth := 0
		for end = i + 1; end < len(d.toks); end++ {
			switch d.kind(end) {
			case syntax.LBRACE, syntax.LPAREN, syntax.LBRACK:
				depth++
			case syntax.RBRACE, syntax.RPAREN, syntax.RBRACK:
				depth--
			case syntax.NL, syntax.SEMI, syntax.EOF:
				if depth <= 0 {
					goto scan
				}
			}
			if depth < 0 {
				break
			}
		}
	default:
		return false
	}
scan:
	for j := i + 1; j < end; j++ {
		if d.kind(j) == syntax.IDENT && d.text(j) == "field" && d.kind(j-1) != syntax.DOT && d.kind(j-1) != syntax.SAFE_DOT {
			return true
		}
	}
	return false
}

// arrowOnLine reports whether a -> follows i on its line, outside
// brackets: the end of a when branch's conditions.
func (d *detector) arrowOnLine(i int) bool {
	depth := 0
	for j := i + 1; j < len(d.toks); j++ {
		switch d.kind(j) {
		case syntax.LPAREN, syntax.LBRACK, syntax.LBRACE:
			depth++
		case syntax.RPAREN, syntax.RBRACK, syntax.RBRACE:
			if depth--; depth < 0 {
				return false
			}
		case syntax.ARROW:
			if depth == 0 {
				return true
			}
		case syntax.NL, syntax.SEMI, syntax.EOF:
			if depth == 0 {
				return false
			}
		}
	}
	return false
}

// contextParams reports whether the context(...) at i declares context
// parameters: a declaration or a function type follows it.
func (d *detector) contextParams(i int) bool {
	j := d.skipNL(d.match(i+1) + 1)
	switch d.kind(j) {
	case syntax.FUN, syntax.VAL, syntax.VAR, syntax.CLASS, syntax.INTERFACE, syntax.OBJECT, syntax.AT, syntax.LPAREN:
		return true
	case syntax.IDENT:
		return modifierWords[d.text(j)] || visibility[d.text(j)] || d.text(j) == "context"
	}
	return false
}

// contextFeature returns the feature of the context(...) at i, which
// contextParams reports declares context parameters: those of a function,
// top-level, a member or local, or anonymous, of a property or of a
// function type, which kc supports, or those of another declaration, a
// class or a constructor, which it doesn't, nor the context receivers that
// came before them, context(A).
func (d *detector) contextFeature(i int) string {
	j := d.skipNL(d.match(i+1) + 1)
	if d.kind(j) == syntax.LPAREN {
		return "context function type"
	}
	if k := d.skipNL(i + 2); d.kind(k) != syntax.RPAREN && (d.kind(k) != syntax.IDENT || d.kind(k+1) != syntax.COLON) {
		return "context receivers"
	}
	// Past the annotations and modifiers, to the declaration.
	for {
		switch {
		case d.kind(j) == syntax.AT:
			j++
			for d.kind(j+1) == syntax.DOT || d.kind(j+1) == syntax.COLON {
				j += 2
			}
			if j++; d.kind(j) == syntax.LPAREN {
				j = d.match(j) + 1
			}
		case d.kind(j) == syntax.IDENT && (modifierWords[d.text(j)] || visibility[d.text(j)]):
			j++
		default:
			switch k := d.kind(j); {
			case k == syntax.FUN && d.anonFun(j):
				return "context anonymous function"
			case k == syntax.FUN && d.kind(j+1) == syntax.INTERFACE:
			case k == syntax.FUN && len(d.braces) == 0:
				return "context function"
			case k == syntax.FUN && d.memberLevel():
				return "context member"
			case k == syntax.FUN:
				return "context local function"
			case (k == syntax.VAL || k == syntax.VAR) && d.memberLevel():
				return "context property"
			}
			return "context parameters"
		}
		j = d.skipNL(j)
	}
}

// anonFun reports whether the fun at i starts an anonymous function, fun
// (x: Int) or fun Int.(): the parenthesis of its parameters follows fun or
// a receiver's dot, not a name.
func (d *detector) anonFun(i int) bool {
	depth := 0
	for j := i + 1; ; j++ {
		switch k := d.kind(j); {
		case k == syntax.EOF:
			return false
		case k == syntax.LT:
			depth++
		case k == syntax.GT:
			depth--
		case depth > 0:
		case k == syntax.LPAREN:
			k := d.kind(j - 1)
			return k == syntax.FUN || k == syntax.DOT || k == syntax.SAFE_DOT
		case k != syntax.IDENT && k != syntax.DOT && k != syntax.SAFE_DOT && k != syntax.QUEST:
			return false
		}
	}
}

// declStart reports whether a token can follow a modifier.
func declStart(k syntax.Kind) bool {
	switch k {
	case syntax.IDENT, syntax.AT, syntax.FUN, syntax.VAL, syntax.VAR, syntax.CLASS, syntax.INTERFACE,
		syntax.OBJECT, syntax.TYPEALIAS:
		return true
	}
	return false
}

// defaultImports are the packages Kotlin imports by default on the JVM.
var defaultImports = []string{
	"kotlin", "kotlin.annotation", "kotlin.collections", "kotlin.comparisons", "kotlin.io",
	"kotlin.ranges", "kotlin.sequences", "kotlin.text", "kotlin.jvm",
}

// inlineOnlyFunctions returns the names of the default-imported top-level
// functions of the standard library that have no method to call: every
// public declaration with the name is inline-only (its JVM method is
// private) or has a reified type parameter. Names of members of the
// built-in classes, such as isEmpty and hashCode, are left out: a call of
// one is usually the member. Each name maps to the feature its calls are:
// an inline-only call, which kc inlines, if a declaration's body can be
// inlined (internal/inline analyzes it), or else a reified inline-only
// call.
func inlineOnlyFunctions(stdlib string) (map[string]string, error) {
	cp, err := classpath.Open("", []string{stdlib})
	if err != nil {
		return nil, err
	}
	defer cp.Close()
	parts := map[string][]kmeta.Part{}
	for _, name := range cp.Resources(".kotlin_module") {
		data, err := cp.Resource(name)
		if err != nil {
			return nil, err
		}
		m, err := kmeta.ReadModule(data)
		if err != nil {
			return nil, err
		}
		for pkg, ps := range m.Packages {
			parts[pkg] = append(parts[pkg], ps...)
		}
	}
	callable := map[string]bool{}
	seen := map[string]bool{}
	inlinable := map[string]bool{}
	for _, pkg := range defaultImports {
		for _, part := range parts[pkg] {
			info, err := cp.Load(part.Class)
			if err != nil || info.Kotlin == nil {
				continue
			}
			meta, err := kmeta.ReadPackage(info.Kotlin)
			if err != nil {
				return nil, err
			}
			public := map[string]bool{}
			for _, m := range info.Methods {
				if m.Access&classfile.AccPublic != 0 {
					public[m.Name+m.Desc] = true
				}
			}
			data, err := cp.Bytes(part.Class)
			if err != nil {
				return nil, err
			}
			read := func(name, desc string) (*classfile.Method, error) { return classfile.ReadMethod(data, name, desc) }
			names, _, _ := meta.Names()
			for _, name := range names {
				fs, err := meta.Functions(name)
				if err != nil {
					return nil, err
				}
				for _, f := range fs {
					if f.Flags.Visibility() != kmeta.Public {
						continue
					}
					seen[name] = true
					reified := false
					for _, p := range f.TypeParams {
						reified = reified || p.Reified
					}
					switch {
					case public[f.JVM.Name+f.JVM.Desc] && !reified:
						callable[name] = true
					case !reified && f.IsInline():
						if m, err := read(f.JVM.Name, f.JVM.Desc); err == nil {
							if _, err := inline.Analyze(m, read); err == nil {
								inlinable[name] = true
							}
						}
					}
				}
			}
		}
	}
	for _, name := range cp.Resources(".kotlin_builtins") {
		data, err := cp.Resource(name)
		if err != nil {
			return nil, err
		}
		b, err := kmeta.ReadBuiltins(data)
		if err != nil {
			return nil, err
		}
		for _, c := range b.Classes {
			for _, f := range c.Funcs {
				callable[f.Name] = true
			}
		}
	}
	out := map[string]string{}
	for name := range seen {
		switch {
		case callable[name] || builtIn[name]:
		case inlinable[name]:
			out[name] = "inline-only call"
		default:
			out[name] = "reified inline-only call"
		}
	}
	return out, nil
}

func report(sb *strings.Builder, name string, sets []*sourceSet, nInline int) {
	fmt.Fprintf(sb, "# Survey: %s\n\n", name)
	fmt.Fprintf(sb, "Written by `bench/survey` (see `bench/survey.sh`). For each file, a detector that reads kc's tokens lists the features it uses that kc doesn't support. A file is *blocked* by each of them. The detector only sees syntax and the names of the %d inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile.\n\n", nInline)
	for _, s := range sets {
		reportSet(sb, s)
	}
}

func reportSet(sb *strings.Builder, s *sourceSet) {
	lines := 0
	for _, f := range s.files {
		lines += f.lines
	}
	fmt.Fprintf(sb, "## %s: %d files, %d lines\n\n", s.label, len(s.files), lines)
	fmt.Fprintf(sb, "Directories: %s\n\n", "`"+strings.Join(s.dirs, "`, `")+"`")

	// The milestones, cumulatively.
	fmt.Fprintf(sb, "### Milestone by milestone\n\nThe features each milestone of kc's plans added, and how many files would use no unsupported feature with them. Each row adds a milestone's features to the ones above it.\n\n")
	fmt.Fprintf(sb, "| After milestone | Files unblocked | Files still blocked | Median blockers left per blocked file |\n|:---|---:|---:|---:|\n")
	done := map[string]bool{}
	fmt.Fprintf(sb, "| (before) | %d | %d | %s |\n", unblocked(s.files, done), len(s.files)-unblocked(s.files, done), median(s.files, done))
	for _, m := range milestones {
		for _, f := range m.features {
			done[f] = true
		}
		n := unblocked(s.files, done)
		fmt.Fprintf(sb, "| %s | %d | %d | %s |\n", m.name, n, len(s.files)-n, median(s.files, done))
	}

	// The features left, by the files they block.
	type count struct {
		feature            string
		files, only, total int
	}
	counts := map[string]*count{}
	for _, f := range s.files {
		left := 0
		for feat := range f.features {
			if !done[feat] {
				left++
			}
		}
		for feat, n := range f.features {
			if done[feat] {
				continue
			}
			c := counts[feat]
			if c == nil {
				c = &count{feature: feat}
				counts[feat] = c
			}
			c.files++
			c.total += n
			if left == 1 {
				c.only++
			}
		}
	}
	var cs []*count
	for _, c := range counts {
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].files != cs[j].files {
			return cs[i].files > cs[j].files
		}
		return cs[i].feature < cs[j].feature
	})
	fmt.Fprintf(sb, "\n### Features left, by files blocked\n\n%d of %d files use no feature that kc doesn't support.\n\n", unblocked(s.files, done), len(s.files))
	fmt.Fprintf(sb, "| Feature | Files | Only blocker | Uses |\n|:---|---:|---:|---:|\n")
	for _, c := range cs {
		fmt.Fprintf(sb, "| %s | %d | %d | %d |\n", c.feature, c.files, c.only, c.total)
	}

	// Greedy order: the feature that unblocks the most files, then the
	// next, ties broken by the files the feature appears in.
	fmt.Fprintf(sb, "\n### Greedy order of the features left\n\nAt each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files.\n\n")
	fmt.Fprintf(sb, "| Step | Feature | Files unblocked |\n|---:|:---|---:|\n")
	added := map[string]bool{}
	for f := range done {
		added[f] = true
	}
	for step := 1; step <= len(cs); step++ {
		best, bestGain, bestFiles := "", -1, -1
		for _, c := range cs {
			if added[c.feature] {
				continue
			}
			added[c.feature] = true
			gain := unblocked(s.files, added)
			delete(added, c.feature)
			if gain > bestGain || gain == bestGain && c.files > bestFiles {
				best, bestGain, bestFiles = c.feature, gain, c.files
			}
		}
		added[best] = true
		fmt.Fprintf(sb, "| %d | %s | %d |\n", step, best, bestGain)
	}

	// Inline-only functions by name.
	calls, files := map[string]int{}, map[string]int{}
	for _, f := range s.files {
		for n, k := range f.inline {
			calls[n] += k
			files[n]++
		}
	}
	var names []string
	for n := range calls {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if files[names[i]] != files[names[j]] {
			return files[names[i]] > files[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) > 0 {
		fmt.Fprintf(sb, "\n### Inline-only functions called\n\n`require`, `check`, `requireNotNull`, `checkNotNull`, `error` and `TODO` are built in; kc inlines the others, since milestone 22 also those with reified type parameters.\n\n| Function | Files | Calls |\n|:---|---:|---:|\n")
		for _, n := range names {
			fmt.Fprintf(sb, "| `%s` | %d | %d |\n", n, files[n], calls[n])
		}
	}

	// Per file.
	fmt.Fprintf(sb, "\n### Files\n\nThe features each file still needs, then, in parentheses, the ones the milestones above added.\n\n| File | Lines | Blockers left |\n|:---|---:|:---|\n")
	sorted := slices.Clone(s.files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })
	for _, f := range sorted {
		var left, planned []string
		for feat := range f.features {
			if done[feat] {
				planned = append(planned, feat)
			} else {
				left = append(left, feat)
			}
		}
		sort.Strings(left)
		sort.Strings(planned)
		cell := strings.Join(left, ", ")
		if len(planned) > 0 {
			if cell != "" {
				cell += " "
			}
			cell += "(" + strings.Join(planned, ", ") + ")"
		}
		fmt.Fprintf(sb, "| %s | %d | %s |\n", f.path, f.lines, cell)
	}

	// Cross-check with the parser.
	var undetected []string
	nErrs := 0
	for _, f := range sorted {
		for _, e := range f.lexErrs {
			undetected = append(undetected, fmt.Sprintf("%s:%s: lexer: %s", f.path, e.Pos, e.Msg))
		}
		for _, e := range f.parseErrs {
			nErrs++
			if e.Feature == "" || f.detected[e.Feature] == 0 {
				undetected = append(undetected, fmt.Sprintf("%s:%s: %s", f.path, e.Pos, e.Msg))
			}
		}
	}
	fmt.Fprintf(sb, "\n### Cross-check with the parser\n\nThe parser, recovering after each declaration that fails, reported %d errors. %d of them name a feature that the detector also found in the file.", nErrs, nErrs-len(undetected))
	if len(undetected) == 0 {
		fmt.Fprintf(sb, "\n\n")
		return
	}
	fmt.Fprintf(sb, " These %d weren't predicted:\n\n", len(undetected))
	for _, u := range undetected {
		fmt.Fprintf(sb, "- %s\n", u)
	}
	fmt.Fprintln(sb)
}

// unblocked returns the number of files whose features are all in done.
func unblocked(files []*file, done map[string]bool) int {
	n := 0
	for _, f := range files {
		ok := true
		for feat := range f.features {
			if !done[feat] {
				ok = false
				break
			}
		}
		if ok {
			n++
		}
	}
	return n
}

// median returns the median number of features not in done, over the
// files that have any.
func median(files []*file, done map[string]bool) string {
	var ns []int
	for _, f := range files {
		n := 0
		for feat := range f.features {
			if !done[feat] {
				n++
			}
		}
		if n > 0 {
			ns = append(ns, n)
		}
	}
	if len(ns) == 0 {
		return "–"
	}
	sort.Ints(ns)
	return fmt.Sprint(ns[len(ns)/2])
}
