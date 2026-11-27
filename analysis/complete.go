package analysis

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Placeholder is the identifier an editor writes at the caret of a
// completion: Options.Complete asks what the code could name there.
const Placeholder = "kc__caret"

// Complete asks an analysis what the code could name at the caret of a
// completion, where the snapshot at Path has the identifier Placeholder,
// written in place of the one being typed, if any.
type Complete struct {
	Path string
	// Match reports whether a name is worth listing, as one that starts
	// as the identifier typed before the caret; nil lists all.
	Match func(name string) bool
	// Importable also lists the top-level functions and properties, and
	// extensions of a receiver's type, that an import would bring into
	// scope.
	Importable bool
}

// Completion is what the code could name at the caret of a completion.
type Completion struct {
	// Found reports whether the checker reached the placeholder; if it
	// didn't, as in a type or an import, Items are the file's top-level
	// declarations in scope.
	Found bool
	// Receiver is the type of the explicit receiver, a in a.x.
	Receiver *Type
	// Expected is the type the code expects there, if it says.
	Expected *Type
	Items    []CompletionItem
}

// CompletionItem is a declaration the code could name at the caret.
type CompletionItem struct {
	Name string
	// Kind is the symbol's kind: class, function, property, local or
	// parameter.
	Kind string
	// Symbol is the declaration, in Result.Symbol.
	Symbol uint32
	// Type is a variable's type, smart casts applied, or a function's
	// result type, with the receiver's type arguments.
	Type *Type
	// Call is a function's signature: its parameters' types, with the
	// receiver's type arguments, and names.
	Call *Call
	// Level is how near it is: 0 for the locals, 1 for the members and
	// extensions of the receivers, explicit or implicit, 2 for the
	// top-level declarations in scope, 3 for those an import adds.
	Level int
	// Implicit is set on a member or extension of an implicit receiver,
	// Extension on an extension.
	Implicit, Extension bool
	// Import is the qualified name an import directive needs for the
	// declaration to be in scope; "" if it is.
	Import string
	// Deprecated is set on a deprecated declaration.
	Deprecated bool
	// ClassKind is a class's kind: class, interface, enum, object or
	// annotation.
	ClassKind string
	// Modality is a member's: final, open or abstract.
	Modality string
	// Mutable is set on a var.
	Mutable bool
	// Constructors are the signatures of the constructors of a class the
	// code can call.
	Constructors []Call
}

// Completion returns the completion that Options.Complete asked for, or
// nil.
func (r *Result) Completion() *Completion {
	if r == nil {
		return nil
	}
	return r.completion
}

// completion converts a probe's items.
func (b *builder) completion(p *check.Probe) *Completion {
	c := &Completion{Found: p.Found(), Items: []CompletionItem{}}
	if t, ok := p.Receiver(); ok {
		c.Receiver = b.cv.typ(t)
	}
	if t := p.Expected(); t.K != types.Invalid {
		c.Expected = b.cv.typ(t)
	}
	for _, it := range p.Items() {
		ci := CompletionItem{Name: it.Name, Level: it.Level, Implicit: it.Implicit, Extension: it.Ext, Import: it.Import, Type: b.cv.typ(it.Type)}
		switch it.Kind {
		case check.ProbeVar:
			v := it.Var
			ci.Symbol = b.sym(v)
			ci.Kind = "property"
			switch v.Kind {
			case types.Local:
				ci.Kind = "local"
			case types.Param:
				ci.Kind = "parameter"
			}
			ci.Deprecated = v.Deprecated != nil
			ci.Modality = modality(v.Abstract, v.Final || v.Owner == nil)
		case check.ProbeFun:
			f := it.Fun
			ci.Symbol = b.sym(f)
			ci.Kind = "function"
			call := b.funCall(f)
			declared := call.Parameters
			call.Parameters = nil
			for i, pt := range it.Params {
				t := b.cv.typ(pt)
				if t == nil && i < len(declared) {
					t = &declared[i]
				}
				if t == nil {
					t = &Type{}
				}
				call.Parameters = append(call.Parameters, *t)
			}
			call.DeclaredReturnType = ci.Type
			ci.Call = &call
			ci.Deprecated = f.Deprecated != nil
			ci.Modality = modality(f.Abstract, f.Final || f.Owner == nil)
		case check.ProbeClass:
			cls := it.Class
			ci.Symbol = b.sym(cls)
			ci.Kind = "class"
			ci.ClassKind = classKind(cls)
			for _, f := range it.Ctors {
				ci.Constructors = append(ci.Constructors, b.funCall(f))
			}
		}
		c.Items = append(c.Items, ci)
	}
	// The Completion is the caller's: none of the interned types.
	c.Receiver, c.Expected = copyType(c.Receiver), copyType(c.Expected)
	for i := range c.Items {
		it := &c.Items[i]
		it.Type = copyType(it.Type)
		it.Call = copyCall(it.Call)
		it.Constructors = copyCalls(it.Constructors)
	}
	return c
}

func modality(abstract, final bool) string {
	switch {
	case abstract:
		return "abstract"
	case final:
		return "final"
	}
	return "open"
}

func classKind(c *types.ClassSym) string {
	if c.External {
		c.Ensure()
	}
	switch {
	case c.Annotation || c.KotlinAnnotation() || c.External && c.Retention != types.NoRetention:
		return "annotation"
	case len(c.Entries) > 0 || c.Super != nil && c.Super.Name == "java/lang/Enum":
		return "enum"
	case c.Interface:
		return "interface"
	case c.Object:
		return "object"
	}
	return "class"
}

// placeholderDiagnostic reports whether a diagnostic is about the
// completion's placeholder.
func placeholderDiagnostic(d Diagnostic) bool {
	return strings.Contains(d.Message, Placeholder)
}
