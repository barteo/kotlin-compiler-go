package ide

import (
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// renderType writes a type as Kotlin code, with its classes' simple
// names: List<String>?, (Int) -> Unit, T!.
func renderType(t analysis.Type) string { return typeString(t, false) }

// qualifiedType writes a type as Kotlin code with its classes' qualified
// names, Java's mapped to Kotlin's, and a platform type as not null: the
// text a shortener makes a declaration's type of.
func qualifiedType(t analysis.Type) string { return typeString(t, true) }

// javaKotlin are the Kotlin classes of Java classes a platform type may
// name.
var javaKotlin = map[string]string{
	"java.lang.Object": "kotlin.Any", "java.lang.String": "kotlin.String", "java.lang.CharSequence": "kotlin.CharSequence",
	"java.lang.Integer": "kotlin.Int", "java.lang.Long": "kotlin.Long", "java.lang.Short": "kotlin.Short",
	"java.lang.Byte": "kotlin.Byte", "java.lang.Double": "kotlin.Double", "java.lang.Float": "kotlin.Float",
	"java.lang.Boolean": "kotlin.Boolean", "java.lang.Character": "kotlin.Char", "java.lang.Number": "kotlin.Number",
	"java.lang.Comparable": "kotlin.Comparable", "java.lang.Iterable": "kotlin.collections.Iterable",
	"java.util.Iterator": "kotlin.collections.Iterator", "java.util.Collection": "kotlin.collections.Collection",
	"java.util.List": "kotlin.collections.List", "java.util.Set": "kotlin.collections.Set",
	"java.util.Map": "kotlin.collections.Map", "java.util.Map.Entry": "kotlin.collections.Map.Entry",
}

func typeString(t analysis.Type, qualified bool) string {
	if t.Projection == "*" {
		return "*"
	}
	var b strings.Builder
	if t.Projection != "" {
		b.WriteString(t.Projection + " ")
	}
	if n, ok := functionArity(t.Name); ok && len(t.Arguments) == n+1 {
		if t.Suspend || strings.HasPrefix(t.Name, "kotlin.coroutines.") {
			b.WriteString("suspend ")
		}
		if t.Nullable {
			b.WriteString("(")
		}
		params := t.Arguments[:n]
		if t.Receiver && n > 0 {
			r := typeString(argument(params[0]), qualified)
			if strings.HasPrefix(r, "(") || strings.HasSuffix(r, "?") {
				r = "(" + r + ")"
			}
			b.WriteString(r + ".")
			params = params[1:]
		}
		b.WriteString("(")
		for i, a := range params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(typeString(argument(a), qualified))
		}
		b.WriteString(") -> ")
		b.WriteString(typeString(argument(t.Arguments[n]), qualified))
		if t.Nullable {
			b.WriteString(")?")
		}
		return b.String()
	}
	switch {
	case !qualified:
		b.WriteString(simpleName(t.Name))
	case t.Parameter:
		b.WriteString(t.Name)
	case javaKotlin[t.Name] != "":
		b.WriteString(javaKotlin[t.Name])
	default:
		b.WriteString(t.Name)
	}
	if len(t.Arguments) > 0 {
		b.WriteString("<")
		for i, a := range t.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(typeString(a, qualified))
		}
		b.WriteString(">")
	}
	switch {
	case t.Platform && qualified:
	case t.Platform:
		b.WriteString("!")
	case t.MarkedNullable || t.Nullable && !t.Parameter:
		b.WriteString("?")
	}
	return b.String()
}

// argument returns a function type's argument without its projection,
// which the arrow syntax implies.
func argument(t analysis.Type) analysis.Type {
	if t.Projection != "*" {
		t.Projection = ""
	}
	return t
}

// functionArity returns n for kotlin.FunctionN and
// kotlin.coroutines.SuspendFunctionN.
func functionArity(name string) (int, bool) {
	for _, p := range []string{"kotlin.Function", "kotlin.coroutines.SuspendFunction"} {
		if s, ok := strings.CutPrefix(name, p); ok {
			n, err := strconv.Atoi(s)
			return n, err == nil
		}
	}
	return 0, false
}

// simpleName returns the last segment of a dotted name.
func simpleName(name string) string {
	return name[strings.LastIndexByte(name, '.')+1:]
}

// renderCall writes the callee of a call: fun <T> R.name(p: P): T, or
// Name(p: P) for a constructor.
func renderCall(c analysis.Call) string {
	s, _ := renderSignature(c)
	return s
}

// renderSignature is renderCall, with each parameter's [start, end) in
// it.
func renderSignature(c analysis.Call) (string, [][2]int) {
	var b strings.Builder
	var params [][2]int
	name := simpleName(c.Name)
	ctor := name == "<init>"
	if ctor {
		name = simpleName(strings.TrimSuffix(c.Name, ".<init>"))
	} else {
		if c.Suspend {
			b.WriteString("suspend ")
		}
		if c.Inline {
			b.WriteString("inline ")
		}
		if c.Operator {
			b.WriteString("operator ")
		}
		b.WriteString("fun ")
		if len(c.TypeParameterNames) > 0 {
			b.WriteString("<" + strings.Join(c.TypeParameterNames, ", ") + "> ")
		}
		if c.Extension && c.DeclaredReceiver != nil {
			b.WriteString(renderReceiver(*c.DeclaredReceiver) + ".")
		}
	}
	b.WriteString(name + "(")
	for i, p := range c.Parameters {
		if i > 0 {
			b.WriteString(", ")
		}
		start := b.Len()
		if i < len(c.Varargs) && c.Varargs[i] {
			b.WriteString("vararg ")
			if len(p.Arguments) == 1 {
				p = p.Arguments[0] // Array<out T>'s T
				p.Projection = ""
			} else if elem, ok := strings.CutSuffix(p.Name, "Array"); ok && strings.HasPrefix(p.Name, "kotlin.") && len(p.Arguments) == 0 {
				p = analysis.Type{Name: elem} // IntArray's Int
			}
		}
		if i < len(c.ParameterNames) && c.ParameterNames[i] != "" && !(c.FromJava && c.ParameterNames[i] == "p") {
			b.WriteString(c.ParameterNames[i] + ": ") // not a Java class file's placeholder
		}
		b.WriteString(renderType(p))
		params = append(params, [2]int{start, b.Len()})
	}
	b.WriteString(")")
	if !ctor && c.DeclaredReturnType != nil {
		b.WriteString(": " + renderType(*c.DeclaredReturnType))
	}
	return b.String(), params
}

// renderReceiver writes a receiver type, in parentheses if it is a
// function type.
func renderReceiver(t analysis.Type) string {
	s := renderType(t)
	if strings.Contains(s, "->") {
		return "(" + s + ")"
	}
	return s
}

// renderProperty writes a library property named name, by its
// accessors: the getter, and the setter of a var.
func renderProperty(name string, accessors []analysis.Call) string {
	kw := "val "
	if len(accessors) > 1 {
		kw = "var "
	}
	g := accessors[0]
	var b strings.Builder
	b.WriteString(kw)
	if len(g.TypeParameterNames) > 0 {
		b.WriteString("<" + strings.Join(g.TypeParameterNames, ", ") + "> ")
	}
	if g.Extension && g.DeclaredReceiver != nil {
		b.WriteString(renderReceiver(*g.DeclaredReceiver) + ".")
	}
	b.WriteString(name)
	if g.DeclaredReturnType != nil {
		b.WriteString(": " + renderType(*g.DeclaredReturnType))
	}
	return b.String()
}

// renderSymbol writes a symbol of the library that no call describes.
func renderSymbol(s analysis.Symbol) string {
	switch s.Kind {
	case "class":
		return "class " + simpleName(strings.ReplaceAll(strings.ReplaceAll(s.Name, "/", "."), "$", "."))
	case "function", "constructor":
		return "fun " + s.Name + "(): " + renderType(s.Type)
	}
	return "val " + s.Name + ": " + renderType(s.Type)
}

// header writes declaration d as its signature, from the code: what
// precedes its body, initializer or accessors, without annotations and
// comments, and with sym's type where it isn't written.
func header(d psi.NodeRef, sym *analysis.Symbol) string {
	if d.NumericKind() == psi.KindTYPE_PARAMETER {
		return typeParameterHeader(d)
	}
	var parts []string
	for c := d.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.IsTrivia() {
			continue
		}
		if d.NumericKind() == psi.KindTYPEALIAS && c.NumericKind() == psi.KindEQ {
			parts = append(parts, "=") // an alias's type is its header's
			continue
		}
		switch c.NumericKind() {
		case psi.KindBLOCK, psi.KindEQ, psi.KindCLASS_BODY, psi.KindPROPERTY_ACCESSOR, psi.KindPROPERTY_DELEGATE,
			psi.KindENUM_ENTRY_SUPERCLASS_REFERENCE_EXPRESSION, psi.KindINITIALIZER_LIST:
			return withType(d, strings.Join(parts, " "), sym)
		case psi.KindCOLON:
			if d.NumericKind() == psi.KindSECONDARY_CONSTRUCTOR {
				return strings.Join(parts, " ")
			}
		case psi.KindMODIFIER_LIST:
			var mods []string
			for m := c.FirstChild(); m.Valid(); m = m.NextSibling() {
				if !m.IsTrivia() && m.IsLeaf() {
					mods = append(mods, m.Text())
				}
			}
			if len(mods) > 0 {
				parts = append(parts, strings.Join(mods, " "))
			}
			continue
		}
		parts = append(parts, compact(c.Text()))
	}
	return withType(d, strings.Join(parts, " "), sym)
}

// withType adds sym's type to header h of d if d doesn't write it: a
// variable's, or a function's return type.
func withType(d psi.NodeRef, h string, sym *analysis.Symbol) string {
	h = strings.NewReplacer(" (", "(", " <", "<", "< ", "<", " >", ">", " :", ":", " .", ".", ". ", ".", " ,", ",").Replace(h)
	if sym == nil || sym.Type.Name == "" {
		return h
	}
	switch d.NumericKind() {
	case psi.KindFUN:
		if !d.ReturnType().Valid() {
			h += ": " + renderType(sym.Type)
		}
	case psi.KindPROPERTY, psi.KindVALUE_PARAMETER, psi.KindDESTRUCTURING_DECLARATION_ENTRY:
		if !d.TypeReference().Valid() {
			h += ": " + renderType(sym.Type)
		}
	}
	return h
}

// typeParameterHeader writes type parameter d with its variance or
// reified, and its bounds: its own and its declaration's where clause's,
// as T : A, B.
func typeParameterHeader(d psi.NodeRef) string {
	var b strings.Builder
	for m := d.ChildKind(psi.KindMODIFIER_LIST).FirstChild(); m.Valid(); m = m.NextSibling() {
		if !m.IsTrivia() && m.IsLeaf() {
			b.WriteString(m.Text() + " ")
		}
	}
	b.WriteString(d.Name())
	var bounds []string
	if t := d.ChildKind(psi.KindTYPE_REFERENCE); t.Valid() {
		bounds = append(bounds, compact(t.Text()))
	}
	owner := d.Parent().Parent()
	for c := owner.ChildKind(psi.KindTYPE_CONSTRAINT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindTYPE_CONSTRAINT && firstNonTrivia(c).Text() == d.Name() {
			if t := c.ChildKind(psi.KindTYPE_REFERENCE); t.Valid() {
				bounds = append(bounds, compact(t.Text()))
			}
		}
	}
	if len(bounds) > 0 {
		b.WriteString(" : " + strings.Join(bounds, ", "))
	}
	return b.String()
}
