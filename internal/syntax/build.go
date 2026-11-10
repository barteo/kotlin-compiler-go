package syntax

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/intbits"
)

// The desugarings here are shared by the token parser and by
// internal/lower, which builds the same AST from a psi tree: both must
// produce identical trees (see Equal).

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

// LabelReturns labels the returns in an anonymous function's body that
// return from it: not those of the lambdas, local functions and objects
// in it.
func LabelReturns(b *Block, label string) {
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

// DelegatedProp returns a property's declarations: for a delegated property,
// its field, x$delegate, then the property itself, with accessors that
// call the delegate. A class's passes this as the delegate's thisRef, and
// an extension property its receiver. As in kotlinc, the fields of
// extension properties with the same name, among the declarations
// before, are x$delegate$1, x$delegate$2, ..., and so are those of
// properties beside one declared as `x$delegate`.
func DelegatedProp(pd *PropDecl, member bool, before []Decl) []Decl {
	if pd.Field == nil {
		return []Decl{pd}
	}
	taken := func(name string) bool {
		for _, d := range before {
			if d, ok := d.(*PropDecl); ok && d.Name == name && (d.DelegateOf != nil || d.Recv == nil) {
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

// TypeExpr returns the expression that names a class: B or a.b.B, for
// the constructor call of a supertype.
func TypeExpr(t *TypeRef) Expr {
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

// AnnotateLambda records annotation a of lambda l if it is Compose's
// @Composable or @DontMemoize.
func AnnotateLambda(l *Lambda, a Annotation) {
	switch {
	case IsComposable(a):
		l.Annotated = true
	case a.Name == "DontMemoize" || a.Name == "androidx.compose.runtime.DontMemoize":
		l.DontMemoize = true
	}
}

// IsComposable reports whether a is Compose's @Composable.
func IsComposable(a Annotation) bool {
	return a.Name == "Composable" || a.Name == "androidx.compose.runtime.Composable"
}

// IsModifier reports whether s is a modifier keyword.
func IsModifier(s string) bool { return isModifier(s) }

// LabelLambdas gives the lambdas passed to a call the function's name as
// their label, for return@name.
func LabelLambdas(c *Call) {
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

// FinishFile sets the fields of f that follow from its declarations.
func FinishFile(f *File, path string) {
	f.Path = path
	f.Facade = facade(f, path)
	f.Part = part(f, path)
}

// Annotate records annotations as of statement or expression n in *m,
// before those it has.
func Annotate(m *map[any][]Annotation, n any, as []Annotation) {
	if len(as) == 0 {
		return
	}
	if *m == nil {
		*m = map[any][]Annotation{}
	}
	(*m)[n] = append(slices.Clip(as), (*m)[n]...)
}

// Suppressions reports which of the diagnostics the File.Suppress* flags
// stand for annotation a suppresses.
func Suppressions(a *Annotation) (invisible, shadowed, dsl bool) {
	if a.Name != "Suppress" && a.Name != "kotlin.Suppress" {
		return
	}
	for _, arg := range a.Args {
		for _, t := range arg.Toks {
			for _, part := range t.Parts {
				switch part.Lit {
				case "INVISIBLE_MEMBER", "INVISIBLE_REFERENCE":
					invisible = true
				case "RECEIVER_SHADOWED_BY_CONTEXT_PARAMETER":
					shadowed = true
				case "DSL_SCOPE_VIOLATION":
					dsl = true
				}
			}
		}
	}
	return
}

// InlineClass makes m, the modifiers of a deprecated inline class whose
// keyword is at kw, those of a value class, which kotlinc annotates
// @JvmInline.
func InlineClass(m *Modifiers, kw Pos) {
	m.Annotations = append(m.Annotations, Annotation{P: kw, Name: "kotlin.jvm.JvmInline", Implicit: true})
	m.Value, m.JvmInline = true, true
}

// CallableRefLambda returns callable reference ref at pos as a lambda for
// the checker to fill in.
func CallableRefLambda(pos Pos, ref *CallableRef) *Lambda {
	if ref.Type != nil {
		ref.X = nil
	}
	l := &Lambda{Arrow: true, Body: &Block{}, Ref: ref}
	l.P = pos
	l.Body.P = pos
	return l
}

// EnumEntryClass returns the class of entry e of enum class enum, which
// has a body: a subclass of the enum, whose superclass's constructor call
// is the entry's. The caller adds the members.
func EnumEntryClass(enum string, e *EnumEntry) *ClassDecl {
	body := &ClassDecl{P: e.P, Name: e.Name, HasPrimary: true, EnumEntry: true}
	body.Supers = []*SuperType{{Type: &TypeRef{P: e.P, Name: enum}, Call: e.Call}}
	return body
}

// IntLiteral returns the integer literal with text at pos, or false if
// its value is out of range.
func IntLiteral(pos Pos, text string, unsigned bool) (*IntLit, bool) {
	text = strings.ReplaceAll(text, "_", "")
	long := strings.HasSuffix(text, "L")
	text = strings.TrimSuffix(text, "L")
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
	ok := err == nil && (v <= math.MaxInt64 || unsigned)
	lit := &IntLit{Val: intbits.FromBits(v), Unsigned: unsigned}
	lit.P = pos
	lit.Long = long || v > math.MaxInt32 && !unsigned || v > math.MaxUint32
	return lit, ok
}

// NegateLiteral folds a minus at pos into lit, a literal written as the
// signed integer text, if it is positive: as K2, a negative literal is one
// literal, not -(1), so -1 may be a Short and -2147483648 is an Int.
func NegateLiteral(lit *IntLit, text string, pos Pos) bool {
	if lit.Unsigned || lit.Val <= 0 {
		return false
	}
	lit.Val = -lit.Val
	lit.Long = strings.HasSuffix(text, "L") || lit.Val < math.MinInt32
	lit.P = pos
	return true
}

// TokenizeAt lexes src, which starts at pos, as an expression: newlines
// are never significant. The tokens end with EOF.
func TokenizeAt(file, src string, pos Pos) ([]Token, []*Error) { return tokenizeAt(file, src, pos) }

// CharValue decodes a character literal's text, quotes included, or
// returns an error message.
func CharValue(text string) (rune, string) { return charValue(text) }

// EscapeValue decodes an escape sequence in a string: \t, \$, A, ...
func EscapeValue(seq string) (rune, bool) {
	switch {
	case len(seq) == 6 && seq[:2] == `\u`:
		return hexRune(seq[2:])
	case len(seq) == 2 && seq[0] == '\\':
		return escape(seq[1])
	}
	return 0, false
}
