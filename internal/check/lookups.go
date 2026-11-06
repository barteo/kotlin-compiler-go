package check

import (
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file records the names the checker looks up in the scopes of
// packages and classes, for incremental processing: Kotlin's lookup
// tracking, which KSP's caches keep to find the files a change dirties.
//
// It follows where FIR records them (FirLookupTrackerComponent's callers):
// a simple name in each package and class scope it is searched in, a class
// in its package or outer class once resolved, a member's name in its
// receiver's class, a callable in its owner, and the classes of the types
// resolved. The checker resolves more than FIR's lazy resolution does, so
// it records a superset: lookups are only used to dirty more files, never
// fewer.

// LookupKind is the kind of scope a name is looked up in.
type LookupKind uint8

const (
	// PackageLookup is a lookup in a package: a.b, or "" for the
	// default package.
	PackageLookup LookupKind = iota
	// ClassLookup is a lookup in a class: a.b.C, its members and nested
	// classes.
	ClassLookup
)

// A LookupRecorder is a Recorder that is also told the names the checker
// looks up in packages and classes: name in scope, a qualified name as
// Kotlin writes it, a.b or a.b.Outer.Inner. Each lookup of a file is
// reported once, from the goroutine checking that file.
type LookupRecorder interface {
	Lookup(f *syntax.File, scope string, kind LookupKind, name string)
}

type lookupKey struct {
	scope, name string
	kind        LookupKind
}

// lookups is a checker's record of the lookups it reported. A typealias
// is resolved in its own file's checker, which another file's may use, so
// it is locked.
type lookups struct {
	mu   sync.Mutex
	seen map[lookupKey]bool
}

// noteLookup reports name looked up in scope, once per file.
func (c *Checker) noteLookup(scope string, kind LookupKind, name string) {
	r := c.u.lookups
	if r == nil || name == "" || strings.HasPrefix(name, "<") {
		return
	}
	k := lookupKey{scope, name, kind}
	c.looked.mu.Lock()
	if c.looked.seen[k] {
		c.looked.mu.Unlock()
		return
	}
	if c.looked.seen == nil {
		c.looked.seen = map[lookupKey]bool{}
	}
	c.looked.seen[k] = true
	c.looked.mu.Unlock()
	r.Lookup(c.file, scope, kind, name)
}

// recordsLookups reports whether lookups are being recorded.
func (c *Checker) recordsLookups() bool { return c.u.lookups != nil }

// classScope returns the qualified name of cls as a scope, or "" and false
// for a class that other files can't see: a local or anonymous one.
func classScope(cls *types.ClassSym) (string, bool) {
	for k := cls; k != nil; k = k.Outer {
		if k.Local || k.Anonymous {
			return "", false
		}
	}
	return fqName(cls), true
}

// noteClass reports cls looked up by its simple name in its package or
// outer class, as FIR's recordClassLikeLookup.
func (c *Checker) noteClass(cls *types.ClassSym) {
	if cls == nil || !c.recordsLookups() || cls.Func && cls.External {
		return
	}
	fq, ok := classScope(cls)
	if !ok {
		return
	}
	i := strings.LastIndexByte(fq, '.')
	if cls.Outer != nil {
		c.noteLookup(fq[:max(i, 0)], ClassLookup, fq[i+1:])
		return
	}
	if i < 0 {
		c.noteLookup("", PackageLookup, fq)
		return
	}
	c.noteLookup(fq[:i], PackageLookup, fq[i+1:])
}

// noteType reports the classes of t, its type arguments' too, as FIR's
// recordTypeResolveAsLookup.
func (c *Checker) noteType(t types.Type) {
	if !c.recordsLookups() || t.K != types.Class || t.C == nil {
		return
	}
	c.noteClass(t.C)
	if t.A != nil {
		for _, a := range t.A.List {
			if a.V != types.Star {
				c.noteType(a.T)
			}
		}
	}
}

// noteMember reports name looked up in the class of t, a receiver's type,
// and its supertypes, whose members it has: FIR records it in the
// receiver's class, and its candidates in the classes that declare them,
// which this covers.
func (c *Checker) noteMember(t types.Type, name string) {
	if !c.recordsLookups() {
		return
	}
	for _, s := range c.supertypes(t) {
		c.noteMemberOf(s.C, name)
	}
}

// noteMemberOf reports name looked up in cls, and in its outer class if
// it is a companion: Outer.name may call it.
func (c *Checker) noteMemberOf(cls *types.ClassSym, name string) {
	if cls == nil {
		return
	}
	if scope, ok := classScope(cls); ok {
		c.noteLookup(scope, ClassLookup, name)
	}
	if o := cls.CompanionOf; o != nil {
		if scope, ok := classScope(o); ok {
			c.noteLookup(scope, ClassLookup, name)
		}
	}
}

// noteName reports a simple name looked up in the scopes around the code,
// as FIR's recordCallLookup over its scopes' owners: the enclosing
// classes, with their supertypes and companions, the file's package, and
// the star imports. Explicit and default imports aren't recorded: FIR's
// scopes for them have no owners.
func (c *Checker) noteName(name string) {
	if !c.recordsLookups() {
		return
	}
	for k := c.scopeClass(); k != nil; k = k.Outer {
		c.noteMember(types.ClassType(k), name)
		if k.Companion != nil {
			c.noteMember(types.ClassType(k.Companion), name)
		}
	}
	c.noteLookup(c.file.Package, PackageLookup, name)
	for _, p := range c.userStars {
		c.noteLookup(p.Name, PackageLookup, name)
	}
	for _, p := range c.stars {
		c.noteLookup(strings.ReplaceAll(p, "/", "."), PackageLookup, name)
	}
	for _, o := range c.starOwners {
		c.noteMemberOf(o, name)
	}
}

// noteClassifier reports a simple name of a type or a qualifier looked up
// in the scopes around the code, and the class it names, if any.
func (c *Checker) noteClassifier(name string) {
	c.noteName(name)
	k := c.nestedClassifier(name)
	if k == (classifier{}) {
		k = c.classCache[name]
	}
	c.noteClass(k.cls)
	if a := k.alias; a != nil {
		if a.Outer != nil {
			c.noteMemberOf(a.Outer, a.Name)
		} else {
			c.noteLookup(a.Pkg, PackageLookup, a.Name)
		}
	}
}

// noteFun reports a callable that a call or a reference resolved to, in
// the class or the package that declares it, and the classes of its
// result, as FIR's recordCallableCandidateAsLookup.
func (c *Checker) noteFun(f *types.FunSym) {
	if f == nil || !c.recordsLookups() {
		return
	}
	switch {
	case f.Owner != nil:
		c.noteMemberOf(f.Owner, f.Name)
	case f.Facade != "":
		c.noteLookup(facadePackage(f.Facade), PackageLookup, f.Name)
	}
	c.noteType(f.Ret)
}

// noteVar is noteFun for a property.
func (c *Checker) noteVar(v *types.VarSym) {
	if v == nil || !c.recordsLookups() {
		return
	}
	switch v.Kind {
	case types.Prop, types.Static:
		c.noteMemberOf(v.Owner, v.Name)
	case types.Global:
		if v.Facade != "" {
			c.noteLookup(facadePackage(v.Facade), PackageLookup, v.Name)
		}
	default:
		return
	}
	c.noteType(v.Type)
}

// facadePackage returns the package of a file facade's internal name.
func facadePackage(facade string) string {
	i := strings.LastIndexByte(facade, '/')
	if i < 0 {
		return ""
	}
	return strings.ReplaceAll(facade[:i], "/", ".")
}

// noteExpr reports the lookups of expression x, checked, of type t.
func (c *Checker) noteExpr(x syntax.Expr, t types.Type) {
	switch x := x.(type) {
	case *syntax.Ident:
		c.noteIdent(x.Name, x.Sym)
		if x.Getter != nil {
			c.noteFun(x.Getter)
		}
		if x.Object != nil {
			c.noteClass(x.Object)
		}
	case *syntax.Member:
		c.noteSelector(x.X, x.Name)
		c.noteVar(x.Prop)
		if x.Object != nil {
			c.noteClass(x.Object)
		}
	case *syntax.Call:
		switch fn := x.Fun.(type) {
		case *syntax.Ident:
			if !x.Invoke {
				c.noteIdent(fn.Name, nil)
			}
		case *syntax.Member:
			if !x.Invoke {
				c.noteSelector(fn.X, fn.Name)
			}
		}
		if x.Ctor != nil {
			c.noteClass(x.Ctor)
		} else {
			c.noteFun(x.Sym)
		}
	}
	c.noteType(t)
}

// noteIdent reports a simple name read or called, unless it is a local:
// locals are found before any scope FIR records.
func (c *Checker) noteIdent(name string, sym *types.VarSym) {
	if sym != nil && (sym.Kind == types.Local || sym.Kind == types.Param) {
		return
	}
	if sym == nil && c.scope != nil {
		if v, _ := c.scope.find(name); v != nil {
			return
		}
	}
	c.noteName(name)
	c.noteVar(sym)
}

// noteSelector reports name selected from x: a member of x's class, or,
// if x names a class or a package, a member of that.
func (c *Checker) noteSelector(x syntax.Expr, name string) {
	if cls := c.classRef(x); cls != nil {
		c.noteClass(cls)
		c.noteMemberOf(cls, name)
		c.noteMemberOf(cls.Companion, name)
		return
	}
	if q, ok := dotted(x); ok && c.packageRef(x) != nil {
		c.noteLookup(q, PackageLookup, name)
		return
	}
	if q, ok := dotted(x); ok && c.u.ext.hasPackage(strings.ReplaceAll(q, ".", "/")) {
		c.noteLookup(q, PackageLookup, name)
		return
	}
	if t := x.Type(); t.K != types.Invalid {
		c.noteMember(t, name)
	}
}

// noteImport reports an explicit import's name looked up in its package
// or class, as FIR's recordFqNameLookup.
func (c *Checker) noteImport(imp *syntax.Import) {
	if !c.recordsLookups() || imp.Star {
		return
	}
	i := strings.LastIndexByte(imp.Path, '.')
	if i < 0 {
		c.noteLookup("", PackageLookup, imp.Path)
		return
	}
	scope, name := imp.Path[:i], imp.Path[i+1:]
	kind := PackageLookup
	if c.u.Pkgs[scope] == nil && !c.u.ext.hasPackage(strings.ReplaceAll(scope, ".", "/")) && c.importOwner(scope) != nil {
		kind = ClassLookup
	}
	c.noteLookup(scope, kind, name)
}
