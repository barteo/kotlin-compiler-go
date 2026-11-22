package check

import (
	"reflect"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// The results writer of builder inference: a builder's lambda is checked
// once, with the builder's variables unfixed (see pcla.go), so the types
// the check gives the tree mention them. Once they are inferred, the
// writer replaces them, in the lambda's subtree and in the symbols the
// check made for it, by what they were inferred as: K2's
// FirCallCompletionResultsWriterTransformer for a PCLA lambda.

// pclaWriter substitutes sub in a subtree.
type pclaWriter struct {
	sub types.Subst
	// seen are the nodes and symbols written: a symbol is reached from
	// each of its uses, and a node from the nodes the checker made of it.
	seen map[any]bool
	// left, if set, is called with each type that still has a variable
	// once sub is applied.
	left func(t types.Type)
}

// pclaWrite replaces the variables of sub in the types of the subtree at
// root, a node, and in those of the symbols it declares and uses.
// It returns the nodes and the symbols it wrote.
func pclaWrite(root any, sub types.Subst) map[any]bool {
	w := &pclaWriter{sub: sub, seen: map[any]bool{}}
	w.value(reflect.ValueOf(root))
	return w.seen
}

// pclaLeft returns the types in the subtree at root that mention a
// variable, each once: after pclaWrite, a variable that nothing inferred.
func pclaLeft(root any) []types.Type {
	var ts []types.Type
	w := &pclaWriter{seen: map[any]bool{}}
	w.left = func(t types.Type) {
		for _, u := range ts {
			if u == t {
				return
			}
		}
		ts = append(ts, t)
	}
	w.value(reflect.ValueOf(root))
	return ts
}

var (
	pclaTypeT     = reflect.TypeOf(types.Type{})
	pclaVarSymT   = reflect.TypeOf((*types.VarSym)(nil))
	pclaFunSymT   = reflect.TypeOf((*types.FunSym)(nil))
	pclaClassDecl = reflect.TypeOf((*syntax.ClassDecl)(nil))
	pclaSyntaxPkg = reflect.TypeOf(syntax.File{}).PkgPath()
	pclaTypesPkg  = pclaTypeT.PkgPath()
)

// pclaRefs are the fields of nodes that refer to a node that isn't a
// part of theirs, which the writer doesn't follow: the lambda or the loop
// that a jump leaves is around it.
var pclaRefs = map[string]bool{
	"Jump.Lambda": true,
	"Jump.Loop":   true,
}

// typ returns t with the variables replaced.
func (w *pclaWriter) typ(t types.Type) types.Type {
	if !t.HasVars() {
		return t
	}
	if len(w.sub) > 0 {
		t = w.sub.Apply(t)
	}
	if w.left != nil && t.HasVars() {
		w.left(t)
	}
	return t
}

// value writes the types in v: a node, a symbol, a type, or a slice, a
// map or a struct of them.
func (w *pclaWriter) value(v reflect.Value) {
	switch v.Type() {
	case pclaTypeT:
		t := v.Interface().(types.Type)
		if u := w.typ(t); u != t && v.CanSet() {
			v.Set(reflect.ValueOf(u))
		}
		return
	case pclaVarSymT:
		if !v.IsNil() {
			w.varSym(v.Interface().(*types.VarSym))
		}
		return
	case pclaFunSymT:
		if !v.IsNil() {
			w.funSym(v.Interface().(*types.FunSym))
		}
		return
	case pclaClassDecl:
		// A class in the lambda: its members' types are its own
		// declaration's (see the deny list of stubCheck).
		return
	}
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			w.value(v.Elem())
		}
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		et := v.Type().Elem()
		switch {
		case et == pclaTypeT:
			w.value(v.Elem())
		case et.Kind() == reflect.Struct && et.PkgPath() == pclaSyntaxPkg:
			if p := v.Interface(); !w.seen[p] {
				w.seen[p] = true
				w.fields(v.Elem())
			}
		}
	case reflect.Struct:
		if v.Type().PkgPath() == pclaSyntaxPkg {
			w.fields(v)
		}
	case reflect.Slice, reflect.Array:
		if !pclaHolds(v.Type().Elem()) {
			return
		}
		for i := range v.Len() {
			w.value(v.Index(i))
		}
	case reflect.Map:
		if !pclaHolds(v.Type().Elem()) {
			return
		}
		for it := v.MapRange(); it.Next(); {
			// A map's values aren't addressable: a copy is written, and
			// put back.
			e := reflect.New(v.Type().Elem()).Elem()
			e.Set(it.Value())
			w.value(e)
			if e.Kind() != reflect.Pointer && e.Kind() != reflect.Interface {
				v.SetMapIndex(it.Key(), e)
			}
		}
	}
}

// fields writes the types in the fields of v, a node's struct.
func (w *pclaWriter) fields(v reflect.Value) {
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() && !f.Anonymous || pclaRefs[t.Name()+"."+f.Name] || !pclaHolds(f.Type) {
			continue
		}
		w.value(v.Field(i))
	}
}

// pclaHolds reports whether a value of type t can hold a type, a symbol
// or a node.
func pclaHolds(t reflect.Type) bool {
	for {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			t = t.Elem()
			continue
		case reflect.Interface:
			return true
		case reflect.Struct:
			return t.PkgPath() == pclaSyntaxPkg || t.PkgPath() == pclaTypesPkg
		}
		return false
	}
}

// varSym writes the types of s, a local, a parameter or a property that
// the check made, or one declared elsewhere, whose types have no
// variables.
func (w *pclaWriter) varSym(s *types.VarSym) {
	if w.seen[s] {
		return
	}
	w.seen[s] = true
	// Only what changes is written: a symbol declared elsewhere is read
	// by the checks of other files.
	if t := w.typ(s.Type); t != s.Type {
		s.Type = t
	}
	if s.FieldT != nil {
		if t := w.typ(*s.FieldT); t != *s.FieldT {
			*s.FieldT = t
		}
	}
	for _, x := range s.Contexts {
		w.varSym(x)
	}
	if s.Delegate != nil {
		w.varSym(s.Delegate)
	}
}

// funSym writes the types of f, a function that the check made, as a
// local one whose result is inferred: one declared elsewhere has no
// variables.
func (w *pclaWriter) funSym(f *types.FunSym) {
	if w.seen[f] {
		return
	}
	w.seen[f] = true
	if t := w.typ(f.Ret); t != f.Ret {
		f.Ret = t
	}
	if t := w.typ(f.Recv); t != f.Recv {
		f.Recv = t
	}
	for _, p := range f.Params {
		w.varSym(p)
	}
	for _, p := range f.Contexts {
		w.varSym(p)
	}
}

// pclaTouches reports whether a type of node n, of an expression that is
// a part of it or of a symbol it uses mentions a variable.
func pclaTouches(n any) bool {
	v := reflect.ValueOf(n)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Type().Elem().Kind() != reflect.Struct {
		return false
	}
	if x, ok := n.(syntax.Expr); ok && x.Type().HasVars() {
		return true
	}
	var has func(v reflect.Value, depth int) bool
	has = func(v reflect.Value, depth int) bool {
		switch x := v.Interface().(type) {
		case types.Type:
			return x.HasVars()
		case *types.VarSym:
			return x != nil && x.Type.HasVars()
		case syntax.Expr:
			return x != nil && !reflect.ValueOf(x).IsNil() && x.Type().HasVars()
		}
		switch v.Kind() {
		case reflect.Slice:
			if !pclaHolds(v.Type().Elem()) {
				return false
			}
			for i := range v.Len() {
				if has(v.Index(i), depth) {
					return true
				}
			}
		case reflect.Struct:
			if depth > 0 || v.Type().PkgPath() != pclaSyntaxPkg {
				return false
			}
			t := v.Type()
			for i := range t.NumField() {
				if f := t.Field(i); f.IsExported() && !pclaRefs[t.Name()+"."+f.Name] && has(v.Field(i), depth+1) {
					return true
				}
			}
		}
		return false
	}
	return has(v.Elem(), 0)
}
