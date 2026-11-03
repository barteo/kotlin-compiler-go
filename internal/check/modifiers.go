package check

import (
	"fmt"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file checks where modifiers and nested declarations may be
// written, as kotlinc's FirModifierChecker, FirNestedClassChecker,
// FirLocalEntityNotAllowedChecker and FirSealedSupertypeChecker do:
// WRONG_MODIFIER_TARGET, REPEATED_MODIFIER, INCOMPATIBLE_MODIFIERS,
// WRONG_MODIFIER_CONTAINING_DECLARATION, NESTED_CLASS_NOT_ALLOWED,
// LOCAL_OBJECT_NOT_ALLOWED and SEALED_SUPERTYPE_IN_LOCAL_CLASS. It runs
// as a pass of its own after the others, and reports nothing on a line
// that already has an error, nor two errors on one line.

// tgt is a set of kotlinc's KotlinTarget values: what a declaration is,
// or what a modifier applies to.
type tgt uint64

const (
	tClassOnly tgt = 1 << iota
	tLocalClass
	tInterface
	tEnumClass
	tAnnotationClass
	tObject
	tStandaloneObject
	tCompanionObject
	tClass
	tMemberProperty
	tTopLevelProperty
	tMemberFunction
	tTopLevelFunction
	tLocalFunction
	tFunction
	tGetter
	tSetter
	tValueParam
	tPropertyParam
	tLocalVariable
	tCtor
	tTypealias
	tEnumEntry
	tObjectLiteral
	tFile
	// tProperty marks a member property kc lets inline and external
	// apply to, not knowing whether it has a field.
	tProperty
)

var tgtNames = []string{
	"class", "local class", "interface", "enum class", "annotation class",
	"object", "standalone object", "companion object", "class",
	"member property", "top level property", "member function",
	"top level function", "local function", "function", "getter", "setter",
	"value parameter", "property parameter", "local variable",
	"constructor", "typealias", "enum entry", "object literal", "file",
	"property",
}

// first returns the name of the first target in t.
func (t tgt) first() string {
	for i, n := range tgtNames {
		if t&(1<<i) != 0 {
			return n
		}
	}
	return "declaration"
}

const tVisible = tClassOnly | tObject | tInterface | tEnumClass | tAnnotationClass |
	tMemberFunction | tTopLevelFunction | tGetter | tSetter | tMemberProperty |
	tTopLevelProperty | tCtor | tTypealias

// possibleTargets are the declarations each modifier applies to.
var possibleTargets = map[string]tgt{
	"enum":        tEnumClass,
	"abstract":    tClassOnly | tLocalClass | tInterface | tMemberProperty | tMemberFunction,
	"open":        tClassOnly | tLocalClass | tInterface | tMemberProperty | tMemberFunction,
	"final":       tClassOnly | tLocalClass | tEnumClass | tObject | tMemberProperty | tMemberFunction,
	"sealed":      tClassOnly | tInterface,
	"inner":       tClassOnly,
	"override":    tMemberProperty | tMemberFunction,
	"private":     tVisible,
	"public":      tVisible,
	"internal":    tVisible,
	"protected":   tVisible &^ (tTopLevelFunction | tTopLevelProperty),
	"vararg":      tValueParam | tPropertyParam,
	"companion":   tObject,
	"lateinit":    tMemberProperty | tTopLevelProperty | tLocalVariable,
	"data":        tClassOnly | tLocalClass | tStandaloneObject,
	"inline":      tFunction | tProperty | tTopLevelProperty | tGetter | tSetter | tClassOnly,
	"noinline":    tValueParam,
	"crossinline": tValueParam,
	"tailrec":     tFunction,
	"operator":    tFunction,
	"infix":       tFunction,
	"suspend":     tMemberFunction | tTopLevelFunction | tLocalFunction,
	"external":    tFunction | tProperty | tTopLevelProperty | tGetter | tSetter | tClass,
	"annotation":  tAnnotationClass,
	"const":       tMemberProperty | tTopLevelProperty,
	"value":       tClassOnly,
}

// allowedParents are the declarations a modifier may be written inside,
// for the modifiers that kotlinc restricts.
var allowedParents = map[string]tgt{
	"inner":     tClassOnly | tLocalClass | tEnumClass | tEnumEntry,
	"override":  tClassOnly | tLocalClass | tObject | tObjectLiteral | tInterface | tEnumClass | tEnumEntry,
	"protected": tClassOnly | tLocalClass | tEnumClass | tCompanionObject,
	"internal":  tClassOnly | tLocalClass | tObject | tObjectLiteral | tEnumClass | tEnumEntry | tFile,
	"private":   tClassOnly | tLocalClass | tObject | tObjectLiteral | tEnumClass | tEnumEntry | tFile | tInterface,
	"companion": tClassOnly | tInterface | tEnumClass | tAnnotationClass,
	"final":     tClassOnly | tLocalClass | tObject | tObjectLiteral | tEnumClass | tEnumEntry | tAnnotationClass | tFile,
	"vararg":    tCtor | tFunction | tClass,
}

var visibilityMods = map[string]bool{"private": true, "public": true, "internal": true, "protected": true}

var modalityMods = map[string]bool{"abstract": true, "open": true, "final": true, "sealed": true}

// incompatibleMods reports whether modifiers a and b, written on one
// declaration, are incompatible: kotlinc's INCOMPATIBLE_MODIFIERS.
func incompatibleMods(a, b string, isClass bool) bool {
	pair := func(x, y string) bool { return a == x && b == y || a == y && b == x }
	switch {
	case visibilityMods[a] && visibilityMods[b]:
		return true
	case modalityMods[a] && modalityMods[b]:
		// abstract open and sealed abstract are only redundant.
		return !pair("abstract", "open") && !pair("sealed", "abstract")
	case pair("private", "open"), pair("private", "abstract"):
		return !isClass
	}
	for _, p := range [][2]string{
		{"data", "open"}, {"data", "inner"}, {"data", "abstract"}, {"data", "sealed"},
		{"data", "inline"}, {"data", "value"}, {"inline", "value"},
		{"const", "abstract"}, {"const", "open"}, {"const", "override"},
		{"private", "override"}, {"crossinline", "noinline"},
		{"sealed", "inner"}, {"expect", "actual"},
	} {
		if pair(p[0], p[1]) {
			return true
		}
	}
	return false
}

// modScope is what declarations are written inside.
type modScope struct {
	parent tgt // the targets of the declaration they are in
	// cls is the class they are members of, or nil. local is set if a
	// class declared here is a local class.
	cls      *syntax.ClassDecl
	clsInner bool
	clsLocal bool
	clsEntry bool // cls is an enum entry's body
	inAnon   bool // inside an object expression, at any depth
	local    bool
}

type modChecker struct {
	c       *Checker
	lines   map[int]bool // lines that have an error
	locals  map[*syntax.ClassDecl]bool
	sealeds []*syntax.ClassDecl // local classes to check the supertypes of
}

// checkModifiers runs the modifier and nesting checks on the file.
func (c *Checker) checkModifiers() {
	if c.file == nil {
		return
	}
	m := &modChecker{c: c, lines: map[int]bool{}, locals: map[*syntax.ClassDecl]bool{}}
	for _, e := range c.errs {
		if e.File == c.file.Path {
			m.lines[e.Pos.Line] = true
		}
	}
	file := modScope{parent: tFile}
	for _, d := range c.file.Decls {
		m.decl(d, file)
	}
	for _, cd := range m.sealeds {
		m.sealedSupers(cd)
	}
}

func (m *modChecker) errorf(p syntax.Pos, format string, args ...any) {
	if m.lines[p.Line] {
		return
	}
	m.lines[p.Line] = true
	m.c.errs = append(m.c.errs, &syntax.Error{File: m.c.file.Path, Pos: p, Msg: fmt.Sprintf(format, args...)})
}

// mods checks the modifiers written on a declaration whose targets are
// targets, inside one whose targets are parent. skip lists modifiers
// whose target and containing declaration aren't checked.
func (m *modChecker) mods(written []syntax.Mod, targets, parent tgt, isClass bool, skip ...string) {
	reported := map[int]bool{}
	for i, w := range written {
		for j := 0; j < i; j++ {
			prev := written[j]
			if prev.Name == w.Name {
				if !reported[i] {
					m.errorf(w.P, "repeated '%s'", w.Name)
					reported[i] = true
				}
			} else if incompatibleMods(prev.Name, w.Name, isClass) {
				if !reported[j] {
					m.errorf(prev.P, "modifier '%s' is incompatible with '%s'", prev.Name, w.Name)
					reported[j] = true
				}
				if !reported[i] {
					m.errorf(w.P, "modifier '%s' is incompatible with '%s'", w.Name, prev.Name)
					reported[i] = true
				}
			}
		}
		if reported[i] || w.Name == "expect" || w.Name == "actual" || slices.Contains(skip, w.Name) {
			continue
		}
		if possible, ok := possibleTargets[w.Name]; ok && possible&targets == 0 {
			m.errorf(w.P, "modifier '%s' is not applicable to '%s'", w.Name, targets.first())
			reported[i] = true
			continue
		}
		if allowed, ok := allowedParents[w.Name]; ok && allowed&parent == 0 {
			m.errorf(w.P, "modifier '%s' is not applicable inside '%s'", w.Name, parent.first())
			reported[i] = true
		}
	}
}

// hasExpectActual reports whether a declaration is expect or actual,
// whose modifiers, the platform's concern, are only checked for
// repetition and compatibility.
func hasExpectActual(md *syntax.Modifiers) bool {
	return md.Expect || md.Actual || md.Has("expect") || md.Has("actual")
}

func (m *modChecker) decl(d syntax.Decl, s modScope) {
	member := s.cls != nil
	switch d := d.(type) {
	case *syntax.FunDecl:
		t := tTopLevelFunction | tFunction
		if member {
			t = tMemberFunction | tFunction
		}
		m.fun(d, t, s)
	case *syntax.PropDecl:
		if d.DelegateOf != nil {
			return
		}
		t := tTopLevelProperty
		if member {
			t = tMemberProperty | tProperty
		}
		if !hasExpectActual(&d.Modifiers) {
			m.mods(d.Written, t, s.parent, false)
			for _, am := range d.AccessorMods {
				m.mods(am, tGetter|tSetter, s.parent, false)
			}
		}
		m.expr(d.Init, s)
		for _, acc := range []*syntax.FunDecl{d.Getter, d.Setter} {
			if acc != nil {
				m.body(acc.Body, acc.ExprBody, s, tFunction)
			}
		}
	case *syntax.TypeAliasDecl:
		m.mods(d.Written, tTypealias, s.parent, false)
	case *syntax.InitBlock:
		m.body(d.Body, nil, s, tFile)
	case *syntax.SecondaryCtor:
		if !hasExpectActual(&d.Modifiers) {
			m.mods(d.Written, tCtor, s.parent, false)
		}
		m.params(d.Params, s)
		m.body(d.Body, nil, s, tFunction)
	case *syntax.ClassDecl:
		m.class(d, s)
	}
}

func (m *modChecker) fun(d *syntax.FunDecl, t tgt, s modScope) {
	if !hasExpectActual(&d.Modifiers) {
		m.mods(d.Written, t, s.parent, false)
	}
	m.params(d.Params, s)
	m.body(d.Body, d.ExprBody, s, tFunction)
}

func (m *modChecker) params(ps []*syntax.Param, s modScope) {
	for _, p := range ps {
		m.mods(p.Mods, tValueParam, tFunction, false, "external")
		m.expr(p.Default, s)
	}
}

// classTargets returns a class's targets, kotlinc's classActualTargets.
func classTargets(cd *syntax.ClassDecl, local bool) tgt {
	switch {
	case cd.Anonymous:
		return tObjectLiteral | tClass
	case cd.Annotation || cd.Has("annotation"):
		return tAnnotationClass | tClass
	case cd.Object && (cd.Companion || cd.Has("companion")):
		return tCompanionObject | tObject | tClass
	case cd.Object:
		return tStandaloneObject | tObject | tClass
	case cd.Interface:
		return tInterface | tClass
	case cd.Enum || cd.Has("enum"):
		if local {
			return tLocalClass | tClass
		}
		return tEnumClass | tClass
	case local && !(cd.Inner || cd.Has("inner")):
		return tLocalClass | tClass
	}
	return tClassOnly | tClass
}

func (m *modChecker) class(cd *syntax.ClassDecl, s modScope) {
	local := s.local
	if local {
		m.locals[cd] = true
		m.sealeds = append(m.sealeds, cd)
	}
	inner := cd.Inner || cd.Has("inner")
	companion := cd.Object && (cd.Companion || cd.Has("companion"))
	targets := classTargets(cd, local)
	ea := hasExpectActual(&cd.Modifiers)
	reported := false
	switch {
	case cd.Object && !companion && local:
		m.errorf(cd.P, "named object '%s' is a singleton and cannot be local; try using anonymous object instead", cd.Name)
		reported = true
	case cd.Interface && local && s.cls == nil:
		m.errorf(cd.P, "'%s' is an interface so it cannot be local", cd.Name)
		reported = true
	case s.cls != nil:
		reported = m.nested(cd, s, local, inner, companion)
	}
	if !reported && !ea {
		m.mods(cd.Written, targets, s.parent, true)
		if !hasExpectActual(&cd.CtorMods) {
			m.mods(cd.CtorMods.Written, tCtor, targets, false)
		}
	}
	in := modScope{
		parent:   targets,
		cls:      cd,
		clsInner: inner,
		clsLocal: local,
		inAnon:   s.inAnon,
		local:    local,
	}
	for _, p := range cd.CtorParams {
		if p.IsProp {
			if !ea {
				m.mods(p.Written, tPropertyParam|tMemberProperty, targets, false,
					"inline", "external", "abstract", "const", "lateinit", "noinline", "crossinline")
			}
		} else {
			m.mods(p.Written, tValueParam, targets, false, "external")
		}
		m.expr(p.Default, s)
	}
	for _, st := range cd.Supers {
		if st.Call != nil {
			for _, a := range st.Call.Args {
				m.expr(a, s)
			}
		}
		m.expr(st.Delegate, s)
	}
	for _, d := range cd.Members {
		m.decl(d, in)
	}
	for _, e := range cd.Entries {
		if e.Call != nil {
			for _, a := range e.Call.Args {
				m.expr(a, in)
			}
		}
		if e.Body != nil {
			m.entryBody(e.Body, in)
		}
	}
}

// entryBody checks the members of an enum entry's body, which is local.
func (m *modChecker) entryBody(cd *syntax.ClassDecl, s modScope) {
	in := modScope{parent: tEnumEntry, cls: cd, clsLocal: true, clsEntry: true, inAnon: s.inAnon, local: true}
	for _, d := range cd.Members {
		m.decl(d, in)
	}
}

// nested checks a class declared in class s.cls, and reports whether it
// reported an error on it.
func (m *modChecker) nested(cd *syntax.ClassDecl, s modScope, local, inner, companion bool) bool {
	if !companion && (cd.Object || cd.Enum || cd.Has("enum")) && local {
		return false
	}
	if companion && !s.clsInner {
		return false
	}
	bad := false
	if s.clsEntry {
		bad = !inner && !companion
	} else {
		bad = !inner && (s.clsInner || s.clsLocal || s.inAnon)
	}
	if bad {
		m.errorf(cd.P, "a class is not allowed here")
	}
	return bad
}

// sealedSupers reports a local class's or object expression's sealed
// supertype declared outside a function.
func (m *modChecker) sealedSupers(cd *syntax.ClassDecl) {
	for _, st := range cd.Supers {
		if st.Type == nil || st.T.K != types.Class || st.T.C == nil || !st.T.C.Sealed {
			continue
		}
		if sd, ok := st.T.C.Decl.(*syntax.ClassDecl); ok && (m.locals[sd] || sd.Local) {
			continue
		}
		m.errorf(st.Type.P, "a local class cannot extend a sealed class or interface")
		return
	}
}

// body checks the code of a function, an accessor or an init block, in
// which classes are local and declarations have parent targets parent.
func (m *modChecker) body(b *syntax.Block, x syntax.Expr, s modScope, parent tgt) {
	in := s
	in.parent = parent
	in.local = true
	if b != nil {
		syntax.Inspect(b, m.visit(in))
	}
	m.expr(x, in)
}

func (m *modChecker) expr(x syntax.Expr, s modScope) {
	if x != nil {
		syntax.Inspect(x, m.visit(s))
	}
}

func (m *modChecker) visit(s modScope) func(any) bool {
	return func(n any) bool {
		switch n := n.(type) {
		case *syntax.LocalClass:
			in := s
			in.parent = tFunction
			in.local = true
			in.cls = nil
			m.class(n.Decl, in)
			return false
		case *syntax.ObjectExpr:
			m.anon(n.Decl, s)
			return false
		case *syntax.LocalFun:
			if !hasExpectActual(&n.Decl.Modifiers) {
				m.mods(n.Decl.Written, tLocalFunction|tFunction, tFunction, false)
			}
			for _, p := range n.Decl.Params {
				m.mods(p.Mods, tValueParam, tFunction, false, "external")
			}
		case *syntax.VarStmt:
			m.mods(n.Mods, tLocalVariable, tFunction, false)
		case *syntax.Lambda:
			if n.Body != nil {
				m.body(n.Body, nil, s, tFunction)
			}
			return false
		}
		return true
	}
}

// anon checks an object expression's class: its members are those of a
// local class.
func (m *modChecker) anon(cd *syntax.ClassDecl, s modScope) {
	if cd == nil {
		return
	}
	m.locals[cd] = true
	m.sealeds = append(m.sealeds, cd)
	for _, st := range cd.Supers {
		if st.Call != nil {
			for _, a := range st.Call.Args {
				m.expr(a, s)
			}
		}
		m.expr(st.Delegate, s)
	}
	in := modScope{parent: tLocalClass | tClass, cls: cd, clsLocal: true, inAnon: true, local: true}
	for _, d := range cd.Members {
		m.decl(d, in)
	}
}
