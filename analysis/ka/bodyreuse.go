package ka

import (
	"reflect"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Shifted files. An update that only changes bodies (bodyEdits) checks
// the changed files again, against the declarations of all the files.
// But ka checks signatures only (check.Options.Signatures), which look
// into a block body for one thing: the local declarations written
// directly in it, which a model lists (Decl.Locals), and whose inferred
// types check the body. A change inside one body of a function, a
// secondary constructor or an initializer that declares no locals,
// before or after it, leaves what the check finds in the file as it
// was: the file's model is the last check's, with the psi nodes after
// the body numbered again and the diagnostics after the change moved
// (shiftedFile). An update whose changed files are all such checks
// nothing.
//
// The body must be a declaration's, in classes and objects only: one in
// an expression (an object expression's function in a property's
// initializer) may be checked to infer a type. An accessor's decides
// whether its property has a backing field, and a leading contract { }
// call what callers see; those changes are checked.

// shiftedUpdates counts the files that updates took over shifted, for
// tests.
var shiftedUpdates atomic.Int64

// bodyOwner is set for the kinds of the declarations whose block bodies
// a shifted file's change may be in.
func bodyOwner(k psi.Kind) bool {
	switch k {
	case psi.KindFUN, psi.KindSECONDARY_CONSTRUCTOR, psi.KindCLASS_INITIALIZER:
		return true
	}
	return false
}

// shiftedFile returns e's model of the file at index i, with its
// diagnostics, moved to snap, the parse of text, if the change from
// e's text is inside one body that no check looks into (see above); or
// false.
func (e *checked) shiftedFile(i int, snap *psi.Snapshot, text string) (*File, []Diagnostic, bool) {
	if e == nil || i >= len(e.files) {
		return nil, nil, false
	}
	prev, ok := e.snaps[snap.Path()]
	if !ok || prev.text == text {
		return nil, nil, false
	}
	old, oldText := prev.snap, prev.text
	// The change: oldText[a:a+lo] became text[a:a+ln].
	a := 0
	for a < len(oldText) && a < len(text) && oldText[a] == text[a] {
		a++
	}
	s := 0
	for s < len(oldText)-a && s < len(text)-a && oldText[len(oldText)-1-s] == text[len(text)-1-s] {
		s++
	}
	lo, ln := len(oldText)-a-s, len(text)-a-s
	delta := ln - lo
	// The outermost body around it.
	var block psi.NodeRef
	for n := old.NodeAt(a); n.Valid(); n = n.Parent() {
		if n.NumericKind() != psi.KindBLOCK || !bodyOwner(n.Parent().NumericKind()) {
			continue
		}
		if r := n.Range(); r.Start < a && a+lo < r.End {
			block = n
		}
	}
	if !block.Valid() || !inClasses(block.Parent()) || lower.Contract(block).Valid() || hasLocals(block) {
		return nil, nil, false
	}
	b, endOld := block.ID(), block.SubtreeEnd()
	nb := snap.Node(b)
	if nb.NumericKind() != psi.KindBLOCK || nb.Range().Start != block.Range().Start || nb.Range().End != block.Range().End+delta {
		return nil, nil, false
	}
	endNew := nb.SubtreeEnd()
	shift := int(endNew) - int(endOld)
	if snap.NodeCount()-old.NodeCount() != shift || lower.Contract(nb).Valid() || hasLocals(nb) {
		return nil, nil, false
	}
	// The trees are the same outside the body.
	for id := psi.NodeID(1); id < b; id++ {
		on, nn := old.Node(id), snap.Node(id)
		if on.NumericKind() != nn.NumericKind() || on.Range().Start != nn.Range().Start {
			return nil, nil, false
		}
	}
	for id := endOld; int(id) <= old.NodeCount(); id++ {
		on, nn := old.Node(id), snap.Node(psi.NodeID(int(id)+shift))
		if on.NumericKind() != nn.NumericKind() || on.Range().Start+delta != nn.Range().Start {
			return nil, nil, false
		}
	}
	f, ok := remapNodes(e.files[i], func(n psi.NodeID) (psi.NodeID, bool) {
		switch {
		case n < b:
			return n, true
		case n >= endOld:
			return psi.NodeID(int(n) + shift), true
		}
		return 0, false
	})
	if !ok {
		return nil, nil, false
	}
	f.Snapshot = snap
	oldLines, newLines := lineStarts(oldText), lineStarts(text)
	var diags []Diagnostic
	for _, d := range e.diags {
		if d.Path != f.Path {
			continue
		}
		if d.Line < 1 || d.Line > len(oldLines) {
			return nil, nil, false
		}
		off := oldLines[d.Line-1] + d.Col - 1
		switch {
		case off >= a+lo:
			off += delta
		case off >= a:
			return nil, nil, false
		}
		l := sort.SearchInts(newLines, off+1) - 1
		d.Line, d.Col = l+1, off-newLines[l]+1
		diags = append(diags, d)
	}
	return f, diags, true
}

// inClasses reports whether declaration d is a top-level declaration or
// a member of named classes and objects, not of an expression.
func inClasses(d psi.NodeRef) bool {
	for n := d.Parent(); n.Valid(); n = n.Parent() {
		switch n.NumericKind() {
		case psi.KindFILE:
			return true
		case psi.KindCLASS_BODY, psi.KindCLASS, psi.KindOBJECT_DECLARATION:
		default:
			return false
		}
	}
	return false
}

// hasLocals reports whether block declares a local directly: a
// variable, a destructuring, a function or a class, which a model lists
// (fileBuilder.locals), or any statement that is annotated or labeled,
// which may be one.
func hasLocals(block psi.NodeRef) bool {
	for c := block.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindPROPERTY, psi.KindDESTRUCTURING_DECLARATION, psi.KindFUN, psi.KindCLASS, psi.KindOBJECT_DECLARATION,
			psi.KindTYPEALIAS, psi.KindANNOTATED_EXPRESSION, psi.KindLABELED_EXPRESSION:
			return true
		}
	}
	return false
}

// lineStarts returns the offsets of text's lines.
func lineStarts(text string) []int {
	starts := make([]int, 1, strings.Count(text, "\n")+1)
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

var nodeIDType = reflect.TypeFor[psi.NodeID]()

// remapNodes returns a copy of f with each psi node ID mapped by fn, or
// false if fn fails for one. The copy shares nothing that holds an ID
// with f; the rest (strings, Lookups) it shares.
func remapNodes(f *File, fn func(psi.NodeID) (psi.NodeID, bool)) (*File, bool) {
	r := remapper{fn: fn, ok: true, done: map[uintptr]reflect.Value{}}
	v := r.copy(reflect.ValueOf(f))
	if !r.ok {
		return nil, false
	}
	return v.Interface().(*File), true
}

type remapper struct {
	fn   func(psi.NodeID) (psi.NodeID, bool)
	ok   bool
	done map[uintptr]reflect.Value // the copies of pointers, so shared values stay shared
}

// holdsIDs caches whether a type holds a psi.NodeID.
var holdsIDs = map[reflect.Type]bool{}

// structPlan is the fields of a struct type that hold psi.NodeIDs, which
// a copy copies (the rest it shares); or, in unexported, one it can't.
type structPlan struct {
	fields     []int
	unexported string
}

var structPlans = map[reflect.Type]*structPlan{}

func holds(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == nodeIDType {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	if t.Name() != "" && t.PkgPath() != "" && t.PkgPath() != reflect.TypeFor[File]().PkgPath() {
		return false // another package's: the Snapshot, shared
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return holds(t.Elem(), seen)
	case reflect.Map:
		return holds(t.Key(), seen) || holds(t.Elem(), seen)
	case reflect.Struct:
		for i := range t.NumField() {
			if holds(t.Field(i).Type, seen) {
				return true
			}
		}
	case reflect.Interface:
		return true
	}
	return false
}

func init() {
	// Filled once, so copies read it without a lock.
	for _, t := range []reflect.Type{reflect.TypeFor[File](), reflect.TypeFor[Decl](), reflect.TypeFor[Type](),
		reflect.TypeFor[TypeArg](), reflect.TypeFor[Annotation](), reflect.TypeFor[NamedValue](), reflect.TypeFor[Value](),
		reflect.TypeFor[Lookup]()} {
		var walk func(t reflect.Type)
		seen := map[reflect.Type]bool{}
		walk = func(t reflect.Type) {
			if _, ok := holdsIDs[t]; ok {
				return
			}
			holdsIDs[t] = holds(t, map[reflect.Type]bool{})
			if t.Kind() == reflect.Struct && holdsIDs[t] {
				p := &structPlan{}
				for i := range t.NumField() {
					f := t.Field(i)
					if !holds(f.Type, map[reflect.Type]bool{}) {
						continue
					}
					if !f.IsExported() && p.unexported == "" {
						p.unexported = t.String() + "." + f.Name
					}
					p.fields = append(p.fields, i)
				}
				structPlans[t] = p
			}
			if seen[t] {
				return
			}
			seen[t] = true
			switch t.Kind() {
			case reflect.Pointer, reflect.Slice, reflect.Array:
				walk(t.Elem())
			case reflect.Map:
				walk(t.Key())
				walk(t.Elem())
			case reflect.Struct:
				for i := range t.NumField() {
					walk(t.Field(i).Type)
				}
			}
		}
		walk(t)
	}
}

func (r *remapper) copy(v reflect.Value) reflect.Value {
	t := v.Type()
	if h, ok := holdsIDs[t]; ok && !h {
		return v
	}
	if t == nodeIDType {
		n, ok := r.fn(psi.NodeID(v.Uint()))
		if !ok {
			r.ok = false
		}
		return reflect.ValueOf(n)
	}
	switch t.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		if c, ok := r.done[v.Pointer()]; ok {
			return c
		}
		c := reflect.New(t.Elem())
		r.done[v.Pointer()] = c
		c.Elem().Set(r.copy(v.Elem()))
		return c
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		c := reflect.MakeSlice(t, v.Len(), v.Len())
		for i := range v.Len() {
			c.Index(i).Set(r.copy(v.Index(i)))
		}
		return c
	case reflect.Array:
		c := reflect.New(t).Elem()
		for i := range v.Len() {
			c.Index(i).Set(r.copy(v.Index(i)))
		}
		return c
	case reflect.Struct:
		p := structPlans[t]
		if p == nil {
			panic("ka: remapNodes: no plan for " + t.String())
		}
		if p.unexported != "" {
			panic("ka: remapNodes: unexported field " + p.unexported)
		}
		c := reflect.New(t).Elem()
		c.Set(v)
		for _, i := range p.fields {
			c.Field(i).Set(r.copy(v.Field(i)))
		}
		return c
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		c := reflect.MakeMapWithSize(t, v.Len())
		it := v.MapRange()
		for it.Next() {
			c.SetMapIndex(r.copy(it.Key()), r.copy(it.Value()))
		}
		return c
	}
	panic("ka: remapNodes: unhandled " + t.String())
}

// shiftedModel is a shifted file's model and diagnostics.
type shiftedModel struct {
	file  *File
	diags []Diagnostic
}

// shiftedModels are an update's shifted files, by index.
type shiftedModels map[int]shiftedModel

// has reports whether the file at path is one of s.
func (s shiftedModels) has(path string) bool {
	for _, m := range s {
		if m.file.Path == path {
			return true
		}
	}
	return false
}
