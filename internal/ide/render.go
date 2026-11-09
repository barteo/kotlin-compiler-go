package ide

import (
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// renderType writes a type as Kotlin code, with its classes' simple
// names: List<String>?, (Int) -> Unit, T!.
func renderType(t analysis.Type) string {
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
		b.WriteString("(")
		for i, a := range t.Arguments[:n] {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(renderType(argument(a)))
		}
		b.WriteString(") -> ")
		b.WriteString(renderType(argument(t.Arguments[n])))
		if t.Nullable {
			b.WriteString(")?")
		}
		return b.String()
	}
	b.WriteString(simpleName(t.Name))
	if len(t.Arguments) > 0 {
		b.WriteString("<")
		for i, a := range t.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(renderType(a))
		}
		b.WriteString(">")
	}
	switch {
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
	var b strings.Builder
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
		if i < len(c.Varargs) && c.Varargs[i] {
			b.WriteString("vararg ")
			if len(p.Arguments) == 1 {
				p = p.Arguments[0] // Array<out T>'s T
				p.Projection = ""
			}
		}
		if i < len(c.ParameterNames) && c.ParameterNames[i] != "" {
			b.WriteString(c.ParameterNames[i] + ": ")
		}
		b.WriteString(renderType(p))
	}
	b.WriteString(")")
	if !ctor && c.DeclaredReturnType != nil {
		b.WriteString(": " + renderType(*c.DeclaredReturnType))
	}
	return b.String()
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
	var parts []string
	for c := d.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.IsTrivia() {
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
