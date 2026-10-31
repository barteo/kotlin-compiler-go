// Package syntax implements the lexer, AST and parser for the supported
// Kotlin subset.
package syntax

import "fmt"

// Pos is a 1-based line/column position in a source file.
type Pos struct {
	Line, Col int
}

func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Error is a positioned diagnostic.
type Error struct {
	File string
	Pos  Pos
	Msg  string
	// Feature is set when the error is about a Kotlin feature kc doesn't
	// support, to a stable name such as "interface" or "modifier data".
	Feature string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Pos.Line, e.Pos.Col, e.Msg)
}

type Kind uint8

const (
	EOF Kind = iota
	NL
	IDENT
	INT
	DOUBLE
	FLOAT    // 1f, 1.5F
	UNSIGNED // 1u, 1UL
	CHAR     // 'c'
	STRING

	// Keywords.
	FUN
	VAL
	VAR
	IF
	ELSE
	WHEN
	WHILE
	FOR
	IN
	RETURN
	BREAK
	CONTINUE
	CLASS
	THIS
	TRUE
	FALSE
	NULL
	IS
	AS
	TRY
	THROW
	INTERFACE
	OBJECT
	PACKAGE
	TYPEALIAS
	SUPER
	DO
	TYPEOF

	// Punctuation and operators.
	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACK
	RBRACK
	COMMA
	COLON
	SEMI
	DOT
	SAFE_DOT // ?.
	ELVIS    // ?:
	NOT_NULL // !!
	QUEST    // ?
	ARROW    // ->
	RANGE    // ..
	RANGE_LT // ..<
	PLUS     // +
	MINUS    // -
	STAR     // *
	SLASH    // /
	PERCENT  // %
	INC      // ++
	DEC      // --
	ASSIGN   // =
	PLUS_EQ  // +=
	MINUS_EQ // -=
	STAR_EQ  // *=
	SLASH_EQ // /=
	PERCENT_EQ
	EQ          // ==
	NE          // !=
	LT          // <
	GT          // >
	LE          // <=
	GE          // >=
	AND         // &&
	OR          // ||
	AMP         // &, in T & Any
	NOT         // !
	NOT_IN      // !in
	NOT_IS      // !is
	AS_SAFE     // as?
	EQ_EQ_EQ    // ===
	NOT_EQ_EQ   // !==
	COLON_COLON // ::
	AT          // @, starting an annotation
	LABEL       // name@, a label; Text is the name

	// Soft keywords used as infix functions; produced by the parser, not the lexer.
	UNTIL
	DOWN_TO
	STEP
)

var kindNames = [...]string{
	EOF: "end of file", NL: "newline", IDENT: "identifier", INT: "integer literal",
	DOUBLE: "double literal", FLOAT: "float literal", UNSIGNED: "unsigned literal",
	CHAR: "character literal", STRING: "string literal",
	FUN: "fun", VAL: "val", VAR: "var", IF: "if", ELSE: "else", WHEN: "when", WHILE: "while",
	FOR: "for", IN: "in", RETURN: "return", BREAK: "break", CONTINUE: "continue", CLASS: "class",
	THIS: "this", TRUE: "true", FALSE: "false", NULL: "null",
	IS: "is", AS: "as", TRY: "try", THROW: "throw", INTERFACE: "interface", OBJECT: "object",
	PACKAGE: "package", TYPEALIAS: "typealias", SUPER: "super", DO: "do", TYPEOF: "typeof",
	LPAREN: "(", RPAREN: ")", LBRACE: "{", RBRACE: "}", LBRACK: "[", RBRACK: "]", COMMA: ",",
	COLON: ":", SEMI: ";", DOT: ".", SAFE_DOT: "?.", ELVIS: "?:", NOT_NULL: "!!", QUEST: "?",
	ARROW: "->", RANGE: "..", RANGE_LT: "..<", PLUS: "+", MINUS: "-", STAR: "*", SLASH: "/",
	PERCENT: "%", INC: "++", DEC: "--", ASSIGN: "=", PLUS_EQ: "+=", MINUS_EQ: "-=", STAR_EQ: "*=",
	SLASH_EQ: "/=", PERCENT_EQ: "%=", EQ: "==", NE: "!=", LT: "<", GT: ">", LE: "<=", GE: ">=",
	AND: "&&", OR: "||", AMP: "&", NOT: "!", NOT_IN: "!in", NOT_IS: "!is", AS_SAFE: "as?",
	EQ_EQ_EQ: "===", NOT_EQ_EQ: "!==", COLON_COLON: "::", AT: "@", LABEL: "label", UNTIL: "until", DOWN_TO: "downTo", STEP: "step",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", k)
}

var keywords = map[string]Kind{
	"fun": FUN, "val": VAL, "var": VAR, "if": IF, "else": ELSE, "when": WHEN, "while": WHILE,
	"for": FOR, "in": IN, "return": RETURN, "break": BREAK, "continue": CONTINUE, "class": CLASS,
	"this": THIS, "true": TRUE, "false": FALSE, "null": NULL,
	"is": IS, "as": AS, "try": TRY, "throw": THROW, "interface": INTERFACE, "object": OBJECT,
	"package": PACKAGE, "typealias": TYPEALIAS, "super": SUPER, "do": DO, "typeof": TYPEOF,
}

// StrPart is one piece of a string literal: either literal text or an
// embedded template expression whose source is parsed later.
type StrPart struct {
	Lit    string
	IsExpr bool
	Src    string // expression source for ${...} or $name
	SrcPos Pos
}

// Token is a lexical token.
type Token struct {
	Kind  Kind
	Pos   Pos
	Text  string    // identifier name or literal text
	Parts []StrPart // STRING only
}
