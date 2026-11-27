package syntax

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/intbits"
	"github.com/barteo/kotlin-compiler-go/internal/types"
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
		c := &Call{Fun: m, Args: append([]Expr{this, ref}, args...), Operator: true}
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
	label := func(a Expr) {
		l, ok := a.(*Lambda)
		switch {
		case !ok:
		case l.Label == "":
			l.Label = name
		case l.Anon && strings.HasPrefix(l.Label, "fun$"):
			// An anonymous function without a label of its own has the
			// function's name too: return@notInline in
			// notInline(fun() { ... }), as its plain returns are.
			old := l.Label
			l.Label = name
			Inspect(l.Body, func(n any) bool {
				if j, ok := n.(*Jump); ok && j.Kind == RETURN && j.Label == old {
					j.Label = name
				}
				return true
			})
		}
	}
	for _, a := range c.Args {
		label(a)
	}
	if m, ok := c.Fun.(*Member); ok && c.Infix {
		label(m.X) // { return@foo } foo { }, as in K2
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

// EnumAnnotation makes m, the modifiers of a class written both enum and
// annotation, those of an enum class, as kotlinc's class kind is: the
// modifier checks report annotation on it.
func EnumAnnotation(m *Modifiers) {
	if m.Enum && m.Annotation {
		m.Annotation = false
	}
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
	long := hasLongSuffix(text)
	text = strings.TrimRight(text, "Ll")
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

// FloatLiteral returns the Float literal, ending in f or F, or the Double
// literal with text at pos, or the error to report. Out of range, 1e400 is
// infinity and 1e-400 zero, as in kotlinc, which warns.
func FloatLiteral(pos Pos, text string) (Expr, string) {
	text = strings.ReplaceAll(text, "_", "")
	if n := len(text); n > 0 && (text[n-1] == 'f' || text[n-1] == 'F') {
		v, err := strconv.ParseFloat(text[:n-1], 32)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, "invalid float literal"
		}
		lit := &FloatLit{Val: float32(v)}
		lit.P = pos
		return lit, ""
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return nil, "invalid double literal"
	}
	lit := &DoubleLit{Val: v}
	lit.P = pos
	return lit, ""
}

// NegateLiteral folds a minus at pos into lit, a literal written as the
// signed integer text, if it is positive: as K2, a negative literal is one
// literal, not -(1), so -1 may be a Short and -2147483648 is an Int.
func NegateLiteral(lit *IntLit, text string, pos Pos) bool {
	if lit.Unsigned || lit.Val <= 0 {
		return false
	}
	lit.Val = -lit.Val
	lit.Long = hasLongSuffix(text) || lit.Val < math.MinInt32
	lit.P = pos
	return true
}

// hasLongSuffix reports whether an integer literal ends in L, or in l,
// which is an error (see IntLiteralErrors).
func hasLongSuffix(text string) bool {
	return strings.HasSuffix(text, "L") || strings.HasSuffix(text, "l")
}

// InvalidNameChars reports whether a name has characters the JVM doesn't
// allow in one, which kotlinc reports as INVALID_CHARACTERS. Only a name
// in backticks can.
func InvalidNameChars(name string) bool {
	return strings.ContainsAny(name, ".;[]/<>:\\")
}

// IntLiteralErrors returns the errors kotlinc reports for the integer
// literal text that don't stop it from being one: a lowercase l suffix
// (WRONG_LONG_SUFFIX), and leading zeros in a decimal literal
// (INT_LITERAL_WITH_LEADING_ZEROS), as 01, 0_1 and 00.
func IntLiteralErrors(text string) []string {
	var msgs []string
	if strings.HasSuffix(text, "l") {
		msgs = append(msgs, "use 'L' instead of 'l'")
	}
	digits := strings.TrimRight(text, "LluU")
	if len(digits) > 1 && digits[0] == '0' && !strings.ContainsAny(digits[1:2], "xXbB") {
		msgs = append(msgs, "unsupported leading zeros in an integer literal")
	}
	return msgs
}

// TokenizeArgAt lexes src, an argument in a call's parentheses starting
// at pos, where newlines count only in a lambda's braces.
func TokenizeArgAt(file, src string, pos Pos) ([]Token, []*Error) {
	return tokenizeArgAt(file, src, pos)
}

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

// A Reporter reports the errors of the checks both front ends share:
// Fail stops at one, Later goes on after it, and Unsupported stops at a
// feature kc doesn't support (see Error.Feature).
type Reporter interface {
	Fail(p Pos, format string, args ...any)
	Later(p Pos, format string, args ...any)
	Unsupported(p Pos, feature, format string, args ...any)
}

func (p *parser) Fail(pos Pos, format string, args ...any)  { p.failAt(pos, format, args...) }
func (p *parser) Later(pos Pos, format string, args ...any) { p.errorLater(pos, format, args...) }
func (p *parser) Unsupported(pos Pos, feature, format string, args ...any) {
	p.unsupported(pos, feature, format, args...)
}

// ParamOnly reports modifiers that only parameters take on another
// declaration, which is kept, so that the rest of it is checked.
func ParamOnly(m Modifiers, r Reporter) {
	if m.Vararg {
		r.Later(m.VarargP, "modifier 'vararg' is only allowed on parameters")
	}
}

// NoInitModifiers reports the modifiers written on an init block, which
// takes none, as kotlinc's WRONG_MODIFIER_TARGET.
func NoInitModifiers(m Modifiers, r Reporter) {
	for _, w := range m.Written {
		if w.Name != "vararg" { // which ParamOnly reports
			r.Later(w.P, "modifier '%s' is not applicable to 'initializer'", w.Name)
		}
	}
}

// NoContexts reports context parameters on a declaration that Kotlin
// doesn't allow them on, what, as kotlinc does, and drops them.
func NoContexts(m *Modifiers, what string, r Reporter) {
	if m.Contexts != nil {
		r.Later(m.ContextP, "context parameters on %s are unsupported", what)
		m.Contexts = nil
	}
}

// CheckLocalFun reports what local function fd, with its modifiers, can't
// be.
func CheckLocalFun(fd *FunDecl, r Reporter) {
	if fd.Body == nil && fd.ExprBody == nil && !fd.External {
		r.Fail(fd.P, "a local function must have a body")
	}
}

// CheckDelegated reports the accessors with bodies of delegated property
// pd, each as kotlinc does, and drops them: the property is checked as
// delegated.
func CheckDelegated(pd *PropDecl, r Reporter) {
	for _, a := range []*FunDecl{pd.Getter, pd.Setter} {
		if a != nil {
			r.Later(a.P, "delegated property cannot have accessors with non-default implementations")
		}
	}
	pd.Getter, pd.Setter = nil, nil
}

// AccessorMods records in property pd the modifiers m of its accessor
// name, get or set, written at pos: annotations without a target are the
// property's, with the accessor as their target, as @get:A is.
func AccessorMods(pd *PropDecl, name string, pos Pos, m *Modifiers, r Reporter) {
	NoContexts(m, "property accessors", r)
	if name == "set" && !pd.Mutable {
		// Checked as kotlinc does, with the setter (VAL_WITH_SETTER).
		r.Later(pos, "a 'val'-property cannot have a setter")
	}
	if pd.AccessorP.Line == 0 {
		pd.AccessorP = pos
	}
	if len(m.Written) > 0 {
		pd.AccessorMods = append(pd.AccessorMods, m.Written)
		pd.AccessorSets = append(pd.AccessorSets, name == "set")
	}
	for _, a := range m.Annotations {
		if a.Target == "" {
			a.Target, a.Accessor = name, true
		} else {
			a.OnAccessor = true
		}
		pd.Annotations = append(pd.Annotations, a)
	}
	if name == "set" {
		pd.SetterVis = m.Visibility
		for _, w := range m.Written {
			if w.Name == "public" || w.Name == "private" || w.Name == "protected" || w.Name == "internal" {
				pd.SetterVisP = w.P
			}
		}
		pd.InlineSetter = m.Inline
	} else {
		pd.InlineGetter = m.Inline
	}
}

// ValOnParam reports val or var, kw, written at p on a parameter of a
// function or a secondary constructor, of
// (VAL_OR_VAR_ON_FUN_PARAMETER, VAL_OR_VAR_ON_SECONDARY_CONSTRUCTOR_PARAMETER).
func ValOnParam(kw, of string, p Pos, r Reporter) {
	r.Later(p, "'%s' on %s parameter is not allowed", kw, of)
}

// VarianceMod adds variance modifier w, written at p, to the variance v
// of a type parameter or argument, reporting a repeated one
// (REPEATED_MODIFIER) and in with out (INCOMPATIBLE_MODIFIERS, at both,
// prevP being where v's modifier is). The first modifier written wins.
func VarianceMod(v *types.Variance, prevP *Pos, w types.Variance, p Pos, r Reporter) {
	name := func(v types.Variance) string {
		if v == types.In {
			return "in"
		}
		return "out"
	}
	switch {
	case *v == types.Inv:
		*v, *prevP = w, p
	case *v == w:
		r.Later(p, "repeated '%s'", name(w))
	default:
		if prevP.Line != 0 {
			r.Later(*prevP, "modifier '%s' is incompatible with '%s'", name(*v), name(w))
			*prevP = Pos{}
		}
		r.Later(p, "modifier '%s' is incompatible with '%s'", name(w), name(*v))
	}
}

// Invalid returns an expression of no type, at p, for one an error
// replaced, which the checker reports nothing more about.
func Invalid(p Pos) Expr {
	n := &NullLit{}
	n.P = p
	x := &As{X: n, Ref: &TypeRef{P: p, Known: &types.InvalidType}}
	x.P = p
	return x
}

// IsInvalid reports whether x is an expression Invalid made.
func IsInvalid(x Expr) bool {
	a, ok := x.(*As)
	if !ok || a.Ref == nil || a.Ref.Known == nil || a.Ref.Known.K != types.Invalid {
		return false
	}
	_, null := a.X.(*NullLit)
	return null
}

// LocalTypeParamsMsg is kotlinc's LOCAL_VARIABLE_WITH_TYPE_PARAMETERS.
const LocalTypeParamsMsg = "local variables are not allowed to have type parameters"

// NamedContextInTypeMsg is kotlinc's NAMED_CONTEXT_PARAMETER_IN_FUNCTION_TYPE,
// for context(a: A) () -> R, whose name nothing can use.
const NamedContextInTypeMsg = "named context parameters in function types are unsupported: use the syntax 'context(Type)' instead"

// WhenCommaMsg is kotlinc's COMMA_IN_WHEN_CONDITION_WITHOUT_ARGUMENT,
// for a, b -> in a when without a subject.
const WhenCommaMsg = "deprecated syntax: use '||' instead of commas in a when-condition for 'when' without an argument"

// CheckAccessor reports a second getter or setter of property pd at
// accessor name written at pos.
func CheckAccessor(pd *PropDecl, name string, pos Pos, r Reporter) {
	switch {
	case name == "get" && pd.Getter != nil:
		r.Fail(pos, "property has two getters")
	case name == "set" && pd.Setter != nil:
		r.Fail(pos, "property has two setters")
	}
}

// ExternalAccessor declares property pd's external accessor without a
// body, external get or external set at pos, with modifiers m: a native
// method, which takes the value as value. It returns the accessor.
func ExternalAccessor(pd *PropDecl, name string, pos Pos, m Modifiers, r Reporter) *FunDecl {
	CheckAccessor(pd, name, pos, r)
	a := &FunDecl{P: pos, Name: name, Property: pd.Name}
	a.External, a.ExternalP = true, m.ExternalP
	if name == "get" {
		pd.Getter = a
	} else {
		a.Visibility = m.Visibility
		a.Params = []*Param{{P: pos, Name: "value"}}
		pd.Setter = a
	}
	return a
}
