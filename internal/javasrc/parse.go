// Package javasrc reads the declarations of Java source files, as kotlinc
// does for a module that mixes Java and Kotlin: the Kotlin code calls the
// Java classes, which javac compiles later, against the Kotlin classes.
// Method bodies and initializers are skipped, and each class becomes the
// classfile.Info a class file of it would give.
package javasrc

import (
	"fmt"
	"strings"
)

// File is a Java source file's declarations.
type File struct {
	Path    string
	Package string // dotted; "" for the default package
	Imports []Import
	Classes []*Class // top-level
}

// Import is an import declaration.
type Import struct {
	Name   string // dotted, without the .*
	Static bool
	Star   bool
}

// Kind is the kind of a type declaration.
type Kind int

const (
	KindClass Kind = iota
	KindInterface
	KindEnum
	KindRecord
	KindAnnotation
)

// Modifiers are a declaration's modifiers and annotations.
type Modifiers struct {
	Flags       uint16 // classfile access flags: public, static, final and so on
	Default     bool   // an interface's default method
	Annotations []*Annotation
}

// Annotation is an annotation, with its arguments' tokens only as names:
// enough to read @Retention and @Target.
type Annotation struct {
	Name string   // as written: Retention or java.lang.annotation.Retention
	Args []string // the identifiers among its arguments
}

// Class is a class, interface, enum, record or annotation type.
type Class struct {
	Modifiers
	Name       string
	Kind       Kind
	TypeParams []*TypeParam
	Extends    []*Type // a class's superclass, or an interface's superinterfaces
	Implements []*Type
	Components []*Param // a record's
	Fields     []*Field
	Methods    []*Method
	Constants  []string // an enum's
	// ConstantBodies is set on an enum whose constants have class bodies,
	// which subclass it.
	ConstantBodies bool
	Nested         []*Class
	Outer          *Class // nil for a top-level class
	file           *File
}

// TypeParam is a type parameter, with its bounds.
type TypeParam struct {
	Name   string
	Bounds []*Type
}

// Type is a type as written. A primitive type or void has Prim set; a
// class type has its dotted name's segments, each with its type arguments.
type Type struct {
	Prim string
	Segs []Seg
	Dims int
}

// Seg is a segment of a class type's name: Map or Entry<K, V> of
// Map.Entry<K, V>.
type Seg struct {
	Name string
	Args []*TypeArg
}

// TypeArg is a type argument: T, ?, ? extends T or ? super T.
type TypeArg struct {
	Wild byte // 0 for T, '*' for ?, '+' for ? extends T, '-' for ? super T
	Type *Type
}

// Field is a field, or one of the variables of a field declaration.
type Field struct {
	Modifiers
	Type *Type
	Name string
	// Init is the initializer's tokens, which a static final field's
	// constant value is computed from.
	Init []token
}

// Method is a method or a constructor, whose Ret is nil.
type Method struct {
	Modifiers
	TypeParams []*TypeParam
	Ret        *Type
	Name       string
	Params     []*Param
	Varargs    bool
	Throws     []*Type
	Body       bool // it has a body, which isn't kept
}

// Param is a method's parameter or a record's component.
type Param struct {
	Type *Type
	Name string
}

// Parse parses a Java source file's declarations.
func Parse(path, src string) (f *File, err error) {
	toks, err := scan(src)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	p := &parser{toks: toks, file: &File{Path: path}}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			f, err = nil, fmt.Errorf("%s:%d:%d: %s", path, e.tok.line, e.tok.col, e.msg)
		}
	}()
	p.compilationUnit()
	return p.file, nil
}

type parseError struct {
	tok token
	msg string
}

type parser struct {
	toks []token
	pos  int
	file *File
}

func (p *parser) tok() token           { return p.toks[p.pos] }
func (p *parser) peek(n int) token     { return p.toks[min(p.pos+n, len(p.toks)-1)] }
func (p *parser) is(s string) bool     { t := p.tok(); return t.kind == tPunct && t.text == s }
func (p *parser) isWord(s string) bool { t := p.tok(); return t.kind == tIdent && t.text == s }

func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tEOF {
		p.pos++
	}
	return t
}

func (p *parser) fail(format string, args ...any) {
	panic(parseError{p.tok(), fmt.Sprintf(format, args...)})
}

func (p *parser) expect(s string) {
	if !p.is(s) {
		p.fail("expected %s, found %s", s, p.describe())
	}
	p.next()
}

func (p *parser) describe() string {
	t := p.tok()
	if t.kind == tEOF {
		return "end of file"
	}
	return fmt.Sprintf("%q", t.text)
}

func (p *parser) ident() string {
	t := p.tok()
	if t.kind != tIdent {
		p.fail("expected an identifier, found %s", p.describe())
	}
	p.next()
	return t.text
}

func (p *parser) accept(s string) bool {
	if p.is(s) {
		p.next()
		return true
	}
	return false
}

// qualified parses a dotted name.
func (p *parser) qualified() string {
	name := p.ident()
	for p.is(".") && p.peek(1).kind == tIdent {
		p.next()
		name += "." + p.ident()
	}
	return name
}

func (p *parser) compilationUnit() {
	mods := p.modifiers()
	if p.isWord("package") {
		p.next()
		p.file.Package = p.qualified()
		p.expect(";")
		mods = p.modifiers()
	}
	for p.isWord("import") {
		p.next()
		imp := Import{}
		if p.isWord("static") {
			p.next()
			imp.Static = true
		}
		imp.Name = p.qualified()
		if p.accept(".") {
			p.expect("*")
			imp.Star = true
		}
		p.expect(";")
		p.file.Imports = append(p.file.Imports, imp)
		mods = p.modifiers()
	}
	for p.tok().kind != tEOF {
		if p.accept(";") {
			mods = p.modifiers()
			continue
		}
		if p.isWord("module") || p.isWord("open") {
			return // module-info.java declares no classes
		}
		c := p.typeDecl(mods, nil)
		if c == nil {
			p.fail("expected a class, interface, enum or record, found %s", p.describe())
		}
		p.file.Classes = append(p.file.Classes, c)
		mods = p.modifiers()
	}
}

var modifierFlags = map[string]uint16{
	"public": 0x0001, "private": 0x0002, "protected": 0x0004, "static": 0x0008, "final": 0x0010,
	"synchronized": 0x0020, "volatile": 0x0040, "transient": 0x0080, "native": 0x0100,
	"abstract": 0x0400, "strictfp": 0x0800,
}

// modifiers parses modifiers and annotations, which the declaration that
// follows has.
func (p *parser) modifiers() Modifiers {
	var m Modifiers
	for {
		t := p.tok()
		switch {
		case t.kind == tPunct && t.text == "@" && !(p.peek(1).kind == tIdent && p.peek(1).text == "interface"):
			p.next()
			m.Annotations = append(m.Annotations, p.annotation())
		case t.kind == tIdent && modifierFlags[t.text] != 0:
			p.next()
			m.Flags |= modifierFlags[t.text]
		case t.kind == tIdent && t.text == "default" && !(p.peek(1).kind == tPunct && p.peek(1).text == ":"):
			p.next()
			m.Default = true
		case t.kind == tIdent && (t.text == "sealed" || t.text == "non") && p.sealedModifier():
		default:
			return m
		}
	}
}

// sealedModifier skips sealed or non-sealed if they are modifiers, before
// a class or interface or another modifier.
func (p *parser) sealedModifier() bool {
	n := 1
	if p.isWord("non") {
		if !(p.peek(1).text == "-" && p.peek(2).text == "sealed") {
			return false
		}
		n = 3
	}
	if p.peek(n).kind != tIdent {
		return false
	}
	p.pos += n
	return true
}

// annotation parses an annotation after its @.
func (p *parser) annotation() *Annotation {
	a := &Annotation{Name: p.qualified()}
	if p.is("(") {
		start := p.pos
		p.skipBalanced()
		for _, t := range p.toks[start:p.pos] {
			if t.kind == tIdent {
				a.Args = append(a.Args, t.text)
			}
		}
	}
	return a
}

// skipBalanced skips a bracketed group: (...), [...] or {...}.
func (p *parser) skipBalanced() {
	depth := 0
	for {
		t := p.next()
		if t.kind == tEOF {
			p.fail("unbalanced brackets")
		}
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
		if depth == 0 {
			return
		}
	}
}

// typeDecl parses a class, interface, enum, record or annotation type
// after its modifiers, or returns nil if none follows.
func (p *parser) typeDecl(mods Modifiers, outer *Class) *Class {
	c := &Class{Modifiers: mods, Outer: outer, file: p.file}
	switch {
	case p.isWord("class"):
		c.Kind = KindClass
	case p.isWord("interface"):
		c.Kind = KindInterface
	case p.isWord("enum"):
		c.Kind = KindEnum
	case p.isWord("record") && p.peek(1).kind == tIdent && (p.peek(2).text == "(" || p.peek(2).text == "<"):
		c.Kind = KindRecord
	case p.is("@") && p.peek(1).text == "interface":
		p.next()
		c.Kind = KindAnnotation
	default:
		return nil
	}
	p.next()
	c.Name = p.ident()
	c.TypeParams = p.typeParams()
	if c.Kind == KindRecord {
		p.expect("(")
		for !p.is(")") {
			p.modifiers()
			t := p.typ()
			if p.accept("...") {
				t.Dims++
			}
			c.Components = append(c.Components, &Param{Type: t, Name: p.ident()})
			if !p.accept(",") {
				break
			}
		}
		p.expect(")")
	}
	for {
		switch {
		case p.isWord("extends"):
			p.next()
			c.Extends = p.typeList()
			continue
		case p.isWord("implements"):
			p.next()
			c.Implements = p.typeList()
			continue
		case p.isWord("permits"):
			p.next()
			p.typeList()
			continue
		}
		break
	}
	p.classBody(c)
	return c
}

func (p *parser) typeList() []*Type {
	ts := []*Type{p.typ()}
	for p.accept(",") {
		ts = append(ts, p.typ())
	}
	return ts
}

// typeParams parses <T, U extends A & B>, if present.
func (p *parser) typeParams() []*TypeParam {
	if !p.accept("<") {
		return nil
	}
	var tps []*TypeParam
	for {
		p.modifiers()
		tp := &TypeParam{Name: p.ident()}
		if p.isWord("extends") {
			p.next()
			tp.Bounds = append(tp.Bounds, p.typ())
			for p.accept("&") {
				tp.Bounds = append(tp.Bounds, p.typ())
			}
		}
		tps = append(tps, tp)
		if !p.accept(",") {
			break
		}
	}
	p.expect(">")
	return tps
}

var primitives = map[string]bool{
	"boolean": true, "byte": true, "char": true, "short": true, "int": true, "long": true,
	"float": true, "double": true, "void": true,
}

// typ parses a type, with its array dimensions but not a varargs ....
func (p *parser) typ() *Type {
	p.modifiers() // type annotations
	t := &Type{}
	if name := p.tok().text; p.tok().kind == tIdent && primitives[name] {
		p.next()
		t.Prim = name
	} else {
		for {
			p.modifiers()
			seg := Seg{Name: p.ident()}
			if p.is("<") {
				seg.Args = p.typeArgs()
			}
			t.Segs = append(t.Segs, seg)
			if !(p.is(".") && (p.peek(1).kind == tIdent || p.peek(1).text == "@")) {
				break
			}
			p.next()
		}
	}
	t.Dims = p.dims()
	return t
}

// dims parses [] pairs, maybe annotated.
func (p *parser) dims() int {
	n := 0
	for {
		save := p.pos
		p.modifiers()
		if p.is("[") && p.peek(1).text == "]" {
			p.pos += 2
			n++
			continue
		}
		p.pos = save
		return n
	}
}

func (p *parser) typeArgs() []*TypeArg {
	p.expect("<")
	var args []*TypeArg
	if p.accept(">") {
		return args // the diamond, in an initializer only
	}
	for {
		p.modifiers()
		a := &TypeArg{}
		if p.accept("?") {
			a.Wild = '*'
			if p.isWord("extends") {
				p.next()
				a.Wild, a.Type = '+', p.typ()
			} else if p.isWord("super") {
				p.next()
				a.Wild, a.Type = '-', p.typ()
			}
		} else {
			a.Type = p.typ()
		}
		args = append(args, a)
		if !p.accept(",") {
			break
		}
	}
	p.expect(">")
	return args
}

// classBody parses { members }, with an enum's constants first.
func (p *parser) classBody(c *Class) {
	p.expect("{")
	if c.Kind == KindEnum {
		for {
			p.modifiers()
			if p.tok().kind != tIdent {
				break
			}
			c.Constants = append(c.Constants, p.ident())
			if p.is("(") {
				p.skipBalanced()
			}
			if p.is("{") {
				p.skipBalanced()
				c.ConstantBodies = true
			}
			if !p.accept(",") {
				break
			}
		}
		if !p.accept(";") && !p.is("}") {
			p.fail("expected ; or } after the enum constants, found %s", p.describe())
		}
	}
	for !p.accept("}") {
		if p.tok().kind == tEOF {
			p.fail("expected }")
		}
		p.member(c)
	}
}

// member parses a member declaration of class c.
func (p *parser) member(c *Class) {
	if p.accept(";") {
		return
	}
	if p.is("{") || p.isWord("static") && p.peek(1).text == "{" {
		if p.isWord("static") {
			p.next()
		}
		p.skipBalanced() // an initializer
		return
	}
	mods := p.modifiers()
	if n := p.typeDecl(mods, c); n != nil {
		c.Nested = append(c.Nested, n)
		return
	}
	tps := p.typeParams()
	// A constructor, or a record's compact canonical constructor.
	if p.tok().kind == tIdent && p.tok().text == c.Name && (p.peek(1).text == "(" || p.peek(1).text == "{" && c.Kind == KindRecord) {
		p.next()
		m := &Method{Modifiers: mods, TypeParams: tps, Name: "<init>"}
		if p.is("{") {
			p.skipBalanced()
			return // the canonical constructor is declared anyway
		}
		p.params(m)
		p.methodRest(m)
		c.Methods = append(c.Methods, m)
		return
	}
	t := p.typ()
	name := p.ident()
	if p.is("(") {
		m := &Method{Modifiers: mods, TypeParams: tps, Ret: t, Name: name}
		p.params(m)
		t.Dims += p.dims()
		p.methodRest(m)
		c.Methods = append(c.Methods, m)
		return
	}
	for {
		f := &Field{Modifiers: mods, Type: &Type{Prim: t.Prim, Segs: t.Segs, Dims: t.Dims + p.dims()}, Name: name}
		c.Fields = append(c.Fields, f)
		if p.accept("=") {
			f.Init = p.initializer()
		}
		if p.accept(";") {
			return
		}
		p.expect(",")
		name = p.ident()
	}
}

// params parses a method's parameter list.
func (p *parser) params(m *Method) {
	p.expect("(")
	for !p.is(")") {
		p.modifiers()
		t := p.typ()
		if p.accept("...") {
			t.Dims++
			m.Varargs = true
		}
		if p.isWord("this") {
			// A receiver parameter, which isn't a parameter.
			p.next()
		} else if p.tok().kind == tIdent && p.peek(1).text == "." && p.peek(2).text == "this" {
			p.pos += 3
		} else {
			name := p.ident()
			t.Dims += p.dims()
			m.Params = append(m.Params, &Param{Type: t, Name: name})
		}
		if !p.accept(",") {
			break
		}
	}
	p.expect(")")
}

// methodRest parses what follows a method's parameters: throws, and a body
// or ; or an annotation method's default value.
func (p *parser) methodRest(m *Method) {
	if p.isWord("throws") {
		p.next()
		m.Throws = p.typeList()
	}
	if p.isWord("default") {
		p.next()
		p.initializer()
		p.expect(";")
		return
	}
	if p.accept(";") {
		return
	}
	if !p.is("{") {
		p.fail("expected a method body or ;, found %s", p.describe())
	}
	p.skipBalanced()
	m.Body = true
}

// initializer skips a field's initializer, up to the , or ; that ends it,
// and returns its tokens.
func (p *parser) initializer() []token {
	start := p.pos
	depth := 0
	for {
		t := p.tok()
		if t.kind == tEOF {
			p.fail("unterminated initializer")
		}
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			case ";":
				if depth == 0 {
					return p.toks[start:p.pos]
				}
			case ",":
				// The next variable, or a comma of type arguments as in
				// new HashMap<K, V>(), which the next variable's name
				// doesn't follow.
				if next := p.peek(2); depth == 0 && p.peek(1).kind == tIdent && next.kind == tPunct && strings.Contains("=,;[", next.text) {
					return p.toks[start:p.pos]
				}
			}
		}
		p.next()
	}
}
