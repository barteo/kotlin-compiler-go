package check

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Opt-in requirements and deprecation errors.
//
// An annotation class annotated @RequiresOptIn is a requirement marker:
// a declaration annotated with it, at level ERROR, the default, may only
// be used where the code opts in to it, as kotlinc checks. Code opts in
// where it, or a declaration it is in, is annotated with the marker
// itself or @OptIn(Marker::class), or where the module does (-opt-in).
// A declaration needs opt-in for its own markers, those of the classes
// it is nested in or constructs, and those of the classes and typealiases
// its signature names. A class annotated @SubclassOptInRequired(Marker)
// needs opt-in to inherit from, which @SubclassOptInRequired(Marker) on
// the subclass also gives, and overriding a member with a marker needs
// it too.
//
// A declaration annotated @Deprecated with level ERROR, or HIDDEN, which
// is otherwise invisible, is an error to use, outside of deprecated code.
//
// The checks run after the bodies are checked, on what the checker
// resolved, and only know the program's markers and deprecations: those
// of the class path aren't read from class files.

// optInErrors reports the program's uses of declarations that need opt-in
// or are deprecated with level ERROR or HIDDEN, and errors in the markers
// and the version strings of @SinceKotlin and the like.
func optInErrors(checkers []*Checker) {
	if len(checkers) == 0 {
		return
	}
	o := &optPass{u: checkers[0].u, annClasses: map[string][]*types.ClassSym{}, aliases: map[string][]*types.AliasSym{},
		markers: map[*types.ClassSym]*optMarker{}, seen: map[optErrKey]bool{}}
	for _, c := range checkers {
		o.collect(c.file.Decls)
	}
	for _, c := range checkers {
		w := &optWalker{o: o, c: c, writes: map[syntax.Expr]bool{}}
		w.push(c.file.Annotations)
		w.annotationErrors(c.file.Annotations)
		for _, d := range c.file.Decls {
			w.decl(d)
		}
	}
}

// optPass holds what the checks of all files share.
type optPass struct {
	u *Universe
	// annClasses are the program's annotation classes, and aliases its
	// typealiases, by simple name.
	annClasses map[string][]*types.ClassSym
	aliases    map[string][]*types.AliasSym
	markers    map[*types.ClassSym]*optMarker
	seen       map[optErrKey]bool
}

type optErrKey struct {
	file string
	line int
	msg  string
}

// optMarker is a requirement marker: an annotation class annotated
// @RequiresOptIn. Err is set if its level is ERROR.
type optMarker struct {
	cls          *types.ClassSym
	simple, qual string
	err          bool
}

func (o *optPass) collect(decls []syntax.Decl) {
	for _, d := range decls {
		switch d := d.(type) {
		case *syntax.ClassDecl:
			if d.Sym != nil && d.Annotation {
				o.annClasses[d.Name] = append(o.annClasses[d.Name], d.Sym)
			}
			o.collect(d.Members)
		case *syntax.TypeAliasDecl:
			if d.Sym != nil {
				o.aliases[d.Name] = append(o.aliases[d.Name], d.Sym)
			}
		}
	}
}

// qualName returns the dotted qualified name of a class: a.b.Outer.Inner.
func qualName(cls *types.ClassSym) string {
	return strings.NewReplacer("/", ".", "$", ".").Replace(cls.Name)
}

// nameMatches reports whether name, as written, simple or qualified,
// names the class with qualified name qual.
func nameMatches(name, qual string) bool {
	return name == qual || strings.HasSuffix(qual, "."+name)
}

// annClass returns the class of annotation a, if it is the program's or
// the checker resolved it.
func (o *optPass) annClass(a *syntax.Annotation) *types.ClassSym {
	if a.Class != nil {
		return a.Class
	}
	if a.Source != nil {
		return a.Source
	}
	return o.classNamed(a.Name)
}

// classNamed returns the program's annotation class with this name, as
// written.
func (o *optPass) classNamed(name string) *types.ClassSym {
	simple := name[strings.LastIndexByte(name, '.')+1:]
	for _, cls := range o.annClasses[simple] {
		if nameMatches(name, qualName(cls)) {
			return cls
		}
	}
	return nil
}

// marker returns the requirement marker cls is, or nil.
func (o *optPass) marker(cls *types.ClassSym) *optMarker {
	if cls == nil {
		return nil
	}
	if m, ok := o.markers[cls]; ok {
		return m
	}
	var m *optMarker
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok && d.Annotation {
		if a := d.Find("", "RequiresOptIn"); a != nil {
			q := qualName(cls)
			m = &optMarker{cls: cls, simple: q[strings.LastIndexByte(q, '.')+1:], qual: q, err: true}
			if arg := a.Arg(1, "level"); arg != nil {
				for _, t := range arg.Toks {
					if t.Text == "WARNING" {
						m.err = false
					}
				}
			}
		}
	}
	o.markers[cls] = m
	return m
}

// annMarkers returns the error-level markers among annotations as whose
// use-site targets are among targets.
func (o *optPass) annMarkers(as []syntax.Annotation, targets ...string) []*optMarker {
	var out []*optMarker
	for i := range as {
		a := &as[i]
		ok := false
		for _, t := range targets {
			ok = ok || a.Target == t
		}
		if !ok {
			continue
		}
		if m := o.marker(o.annClass(a)); m != nil && m.err {
			out = append(out, m)
		}
	}
	return out
}

// classLitNames returns the names of the class literals among the
// arguments of a: M in @OptIn(M::class).
func classLitNames(a *syntax.Annotation) []string {
	var out []string
	for _, arg := range a.Args {
		name := ""
		for i, t := range arg.Toks {
			switch t.Kind {
			case syntax.IDENT:
				if name != "" && !strings.HasSuffix(name, ".") {
					name = ""
				}
				name += t.Text
			case syntax.DOT:
				name += "."
			case syntax.COLON_COLON:
				if i+1 < len(arg.Toks) && arg.Toks[i+1].Kind == syntax.CLASS && name != "" {
					out = append(out, name)
				}
				name = ""
			default:
				name = ""
			}
		}
	}
	return out
}

// classMarkers returns the error-level markers of a class: its own, and
// those of the classes it is nested in.
func (o *optPass) classMarkers(cls *types.ClassSym) []*optMarker {
	var out []*optMarker
	for depth := 0; cls != nil && depth < 32; depth++ {
		if d, ok := cls.Decl.(*syntax.ClassDecl); ok {
			out = append(out, o.annMarkers(d.Annotations, "")...)
		}
		cls = cls.Outer
	}
	return out
}

// aliasMarkers returns the error-level markers of a typealias: its own,
// and those of the classifiers it expands to that it doesn't opt in to.
func (o *optPass) aliasMarkers(a *types.AliasSym, depth int) []*optMarker {
	d, ok := a.Decl.(*syntax.TypeAliasDecl)
	if !ok || depth > 16 {
		return nil
	}
	out := o.annMarkers(d.Annotations, "")
	for _, m := range o.refMarkers(d.Type, depth+1) {
		if !o.optsIn([][]syntax.Annotation{d.Annotations}, m, false) {
			out = append(out, m)
		}
	}
	return out
}

// classifierMarkers returns the error-level markers of a class or a
// typealias.
func (o *optPass) classifierMarkers(k any, depth int) []*optMarker {
	switch k := k.(type) {
	case *types.ClassSym:
		return o.classMarkers(k)
	case *types.AliasSym:
		return o.aliasMarkers(k, depth)
	}
	return nil
}

// refTypes calls fn on each type a type reference is made of: itself,
// its type arguments, and a function type's parameter and result types.
func refTypes(ref *syntax.TypeRef, fn func(*syntax.TypeRef)) {
	if ref == nil {
		return
	}
	fn(ref)
	for _, a := range ref.Args {
		if a != nil {
			refTypes(a.Type, fn)
		}
	}
	for _, p := range ref.Params {
		refTypes(p, fn)
	}
	for _, p := range ref.Contexts {
		refTypes(p, fn)
	}
	refTypes(ref.Ret, fn)
	refTypes(ref.Recv, fn)
	refTypes(ref.Outer, fn)
}

// refMarkers returns the error-level markers of the classifiers a type
// reference names.
func (o *optPass) refMarkers(ref *syntax.TypeRef, depth int) []*optMarker {
	var out []*optMarker
	refTypes(ref, func(r *syntax.TypeRef) {
		if k, ok := o.u.typeUses.Load(r); ok {
			out = append(out, o.classifierMarkers(k, depth)...)
		}
	})
	return out
}

// typeMarkers returns the error-level markers of the classes a type is
// made of, its type arguments' too.
func (o *optPass) typeMarkers(t types.Type) []*optMarker {
	var out []*optMarker
	var walk func(t types.Type, depth int)
	walk = func(t types.Type, depth int) {
		if t.K != types.Class || t.C == nil || depth > 16 {
			return
		}
		out = append(out, o.classMarkers(t.C)...)
		if t.A != nil {
			for i := range t.A.Len() {
				walk(t.A.Get(i).T, depth+1)
			}
		}
	}
	walk(t, 0)
	return out
}

// optsIn reports whether the scope, the annotations of the code and the
// declarations it is in, opts in to marker m. With inherit,
// @SubclassOptInRequired(M::class) does too.
func (o *optPass) optsIn(scope [][]syntax.Annotation, m *optMarker, inherit bool) bool {
	for _, name := range o.u.optIns {
		if name == m.qual {
			return true
		}
	}
	for _, as := range scope {
		for i := range as {
			a := &as[i]
			switch {
			case a.Is("OptIn") || inherit && a.Is("SubclassOptInRequired"):
				for _, n := range classLitNames(a) {
					if nameMatches(n, m.qual) {
						return true
					}
				}
			case a.Target == "" || a.Target == "file":
				if nameMatches(a.Name, m.qual) && o.annClass(a) == m.cls {
					return true
				}
			}
		}
	}
	return false
}

// deprecation returns the level of the @Deprecated annotation among as,
// with no use-site target or one of targets, if it is ERROR or HIDDEN:
// "ERROR", "HIDDEN" or "".
func deprecation(as []syntax.Annotation, targets ...string) string {
	for i := range as {
		a := &as[i]
		if !a.Is("Deprecated") {
			continue
		}
		ok := a.Target == ""
		for _, t := range targets {
			ok = ok || a.Target == t
		}
		if !ok {
			continue
		}
		if arg := a.Arg(2, "level"); arg != nil && len(arg.Toks) > 0 {
			switch l := arg.Toks[len(arg.Toks)-1].Text; l {
			case "ERROR", "HIDDEN":
				return l
			}
		}
	}
	return ""
}

// classDeprecation returns the deprecation level of a program's class.
func classDeprecation(cls *types.ClassSym) string {
	if cls == nil {
		return ""
	}
	// A compiler plugin's synthetic class, serialization's $serializer, is
	// deprecated with level HIDDEN so that only the plugin's own members,
	// such as serializer(), use it.
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok && !d.Synthetic {
		return deprecation(d.Annotations)
	}
	return ""
}

// optWalker walks a file's declarations and code, with the annotations
// of those it is in.
type optWalker struct {
	o *optPass
	c *Checker
	// scope are the annotations of the file, the declarations and the
	// annotated code the walk is in, and classes the classes.
	scope   [][]syntax.Annotation
	classes []*types.ClassSym
	// writes are the expressions assigned to.
	writes map[syntax.Expr]bool
	// entered is the annotated node whose annotations were just pushed.
	entered any
}

func (w *optWalker) push(as []syntax.Annotation) { w.scope = append(w.scope, as) }
func (w *optWalker) pop()                        { w.scope = w.scope[:len(w.scope)-1] }

func (w *optWalker) errorf(p syntax.Pos, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	k := optErrKey{w.c.file.Path, p.Line, msg}
	if w.o.seen[k] {
		return
	}
	w.o.seen[k] = true
	w.c.errorf(p, "%s", msg)
}

// suppressed reports whether the scope suppresses the diagnostic name.
func (w *optWalker) suppressed(name string) bool {
	for _, as := range w.scope {
		if suppresses(as, name) {
			return true
		}
	}
	return false
}

// deprecated reports whether the code is in a deprecated declaration,
// where uses of deprecated declarations aren't reported.
func (w *optWalker) deprecated() bool {
	for _, as := range w.scope {
		for i := range as {
			if as[i].Is("Deprecated") && (as[i].Target == "" || as[i].Target == "file") {
				return true
			}
		}
	}
	return false
}

// require reports a use at p of a declaration with markers ms that the
// scope doesn't opt in to.
func (w *optWalker) require(p syntax.Pos, ms []*optMarker) {
	if len(ms) == 0 || w.suppressed("OPT_IN_USAGE_ERROR") {
		return
	}
	for _, m := range ms {
		if !w.o.optsIn(w.scope, m, false) {
			w.errorf(p, "this declaration needs opt-in: its usage must be marked with '@%s' or '@OptIn(%s::class)'", m.simple, m.simple)
		}
	}
}

// deprecatedUse reports a use at p of name, deprecated with level.
func (w *optWalker) deprecatedUse(p syntax.Pos, name, level string) {
	if level == "" || w.deprecated() || w.suppressed("DEPRECATION_ERROR") {
		return
	}
	w.errorf(p, "'%s' is deprecated with level %s", name, level)
}

// typeRef checks the classifiers a type reference names.
func (w *optWalker) typeRef(ref *syntax.TypeRef) {
	refTypes(ref, func(r *syntax.TypeRef) {
		k, ok := w.o.u.typeUses.Load(r)
		if !ok {
			return
		}
		w.require(r.P, w.o.classifierMarkers(k, 0))
		switch k := k.(type) {
		case *types.ClassSym:
			w.deprecatedUse(r.P, k.DisplayName(), classDeprecation(k))
		case *types.AliasSym:
			if d, ok := k.Decl.(*syntax.TypeAliasDecl); ok {
				w.deprecatedUse(r.P, k.Name, deprecation(d.Annotations))
			}
			if cls := w.expansionDeprecated(k.Type, 0); cls != nil && !w.deprecated() && !w.suppressed("TYPEALIAS_EXPANSION_DEPRECATION_ERROR") {
				w.errorf(r.P, "the expansion of typealias '%s' uses '%s', which is deprecated", k.Name, cls.DisplayName())
			}
		}
	})
}

// expansionDeprecated returns a class deprecated with level ERROR or
// HIDDEN that type t is made of, or nil.
func (w *optWalker) expansionDeprecated(t types.Type, depth int) *types.ClassSym {
	if t.K != types.Class || t.C == nil || depth > 16 {
		return nil
	}
	if classDeprecation(t.C) != "" {
		return t.C
	}
	if t.A != nil {
		for i := range t.A.Len() {
			if cls := w.expansionDeprecated(t.A.Get(i).T, depth+1); cls != nil {
				return cls
			}
		}
	}
	return nil
}

// annotationErrors checks annotations of a declaration: the version
// strings of @SinceKotlin and the like, and @DeprecatedSinceKotlin
// outside the kotlin package.
func (w *optWalker) annotationErrors(as []syntax.Annotation) {
	for i := range as {
		a := &as[i]
		var versions []*syntax.AnnotationArg
		switch {
		case a.Is("SinceKotlin"):
			versions = append(versions, a.Arg(0, "version"))
		case a.Is("RequireKotlin") || a.Name == "kotlin.internal.RequireKotlin":
			versions = append(versions, a.Arg(0, "version"))
		case a.Is("DeprecatedSinceKotlin"):
			if pkg := w.c.file.Package; pkg != "kotlin" && !strings.HasPrefix(pkg, "kotlin.") {
				w.errorf(a.P, "@DeprecatedSinceKotlin is only allowed in the kotlin package and its subpackages")
			}
			for j := range a.Args {
				versions = append(versions, &a.Args[j])
			}
		}
		for _, arg := range versions {
			if arg == nil || len(arg.Toks) != 1 || arg.Toks[0].Kind != syntax.STRING {
				continue
			}
			s, ok := literalString(arg.Toks[0])
			if ok && !kotlinVersion.MatchString(s) {
				w.errorf(arg.Toks[0].Pos, "invalid Kotlin version %q: it must be major.minor or major.minor.patch", s)
			}
		}
	}
}

var kotlinVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))?$`)

// literalString returns the value of a string token without templates.
func literalString(t syntax.Token) (string, bool) {
	var b strings.Builder
	for _, p := range t.Parts {
		if p.IsExpr {
			return "", false
		}
		b.WriteString(p.Lit)
	}
	return b.String(), true
}

// markerTargetErrors reports requirement markers among annotations as
// that are on a target markers can't be on: a getter, a value parameter,
// a local variable, a receiver, a setter's parameter or a delegate.
func (w *optWalker) markerTargetErrors(as []syntax.Annotation, param bool) {
	if suppresses(as, "OPT_IN_MARKER_ON_WRONG_TARGET") || w.suppressed("OPT_IN_MARKER_ON_WRONG_TARGET") {
		return
	}
	for i := range as {
		a := &as[i]
		bad := false
		switch a.Target {
		case "get", "setparam", "receiver", "delegate", "param":
			bad = true
		case "":
			bad = param
		}
		if !bad {
			continue
		}
		if m := w.o.marker(w.o.annClass(a)); m != nil {
			w.errorf(a.P, "opt-in requirement marker '%s' can't be used on this target", m.simple)
		}
	}
}

// markerDeclErrors checks the declaration of an annotation class: a
// marker's targets and retention, and the arguments of
// @SubclassOptInRequired on any class.
func (w *optWalker) markerDeclErrors(d *syntax.ClassDecl) {
	if d.Annotation && d.Find("", "RequiresOptIn") != nil {
		for i := range d.Annotations {
			a := &d.Annotations[i]
			switch {
			case a.Is("Target") || a.Name == "kotlin.annotation.Target":
				for _, arg := range a.Args {
					for _, t := range arg.Toks {
						switch t.Text {
						case "FILE", "EXPRESSION", "TYPE_PARAMETER", "TYPE":
							w.errorf(a.P, "opt-in requirement marker can't be used on target %s", t.Text)
						}
					}
				}
			case a.Is("Retention") || a.Name == "kotlin.annotation.Retention":
				for _, arg := range a.Args {
					for _, t := range arg.Toks {
						if t.Text == "SOURCE" {
							w.errorf(a.P, "opt-in requirement marker can't have SOURCE retention")
						}
					}
				}
			}
		}
	}
	for i := range d.Annotations {
		a := &d.Annotations[i]
		if !a.Is("SubclassOptInRequired") {
			continue
		}
		for _, arg := range a.Args {
			for _, n := range classLitNames(&syntax.Annotation{Args: []syntax.AnnotationArg{arg}}) {
				if cls := w.o.classNamed(n); cls != nil && w.o.marker(cls) == nil {
					p := a.P
					if len(arg.Toks) > 0 {
						p = arg.Toks[0].Pos
					}
					w.errorf(p, "'%s' is not an opt-in requirement marker", n)
				}
			}
		}
	}
}

func (w *optWalker) decl(d syntax.Decl) {
	switch d := d.(type) {
	case *syntax.ClassDecl:
		w.class(d, nil)
	case *syntax.FunDecl:
		w.fun(d, nil)
	case *syntax.PropDecl:
		w.prop(d, nil)
	case *syntax.TypeAliasDecl:
		if d == nil {
			return
		}
		w.annotationErrors(d.Annotations)
		w.push(d.Annotations)
		w.typeRef(d.Type)
		w.pop()
	case *syntax.SecondaryCtor:
		if d == nil {
			return
		}
		w.annotationErrors(d.Annotations)
		w.push(d.Annotations)
		w.params(d.Params)
		if d.Delegation != nil {
			w.node(d.Delegation)
		}
		if d.Body != nil {
			w.node(d.Body)
		}
		w.pop()
	case *syntax.InitBlock:
		if d != nil && d.Body != nil {
			w.node(d.Body)
		}
	}
}

func (w *optWalker) params(ps []*syntax.Param) {
	for _, p := range ps {
		if p == nil {
			continue
		}
		w.markerTargetErrors(p.Annotations, true)
		w.push(p.Annotations)
		w.typeRef(p.Type)
		if p.Default != nil {
			w.node(p.Default)
		}
		w.pop()
	}
}

func (w *optWalker) class(d *syntax.ClassDecl, extra []syntax.Annotation) {
	if d == nil {
		return
	}
	w.annotationErrors(d.Annotations)
	w.markerDeclErrors(d)
	w.push(extra)
	w.push(d.Annotations)
	if d.Sym != nil {
		w.classes = append(w.classes, d.Sym)
	}
	for _, st := range d.Supers {
		if st == nil {
			continue
		}
		w.typeRef(st.Type)
		w.inheritance(st)
		if st.Call != nil {
			// The call is the primary constructor's.
			w.push(d.CtorMods.Annotations)
			w.node(st.Call)
			w.pop()
		}
		if st.Delegate != nil {
			w.node(st.Delegate)
		}
	}
	w.push(d.CtorMods.Annotations)
	ctorScope := len(w.scope) - 1
	for _, p := range d.CtorParams {
		if p == nil {
			continue
		}
		w.annotationErrors(p.Modifiers.Annotations)
		param := !p.IsProp
		if p.IsProp && w.paramTarget(p.Modifiers.Annotations) {
			param = true
		}
		w.markerTargetErrors(p.Modifiers.Annotations, param)
		w.push(p.Modifiers.Annotations)
		w.typeRef(p.Type)
		if p.Default != nil {
			w.node(p.Default)
		}
		if p.IsProp && p.Override && p.PropSym != nil && d.Sym != nil {
			// The constructor's markers don't opt the property in.
			saved := w.scope
			w.scope = append(slices.Clone(saved[:ctorScope]), saved[ctorScope+1:]...)
			w.propOverride(d.Sym, p.PropSym, p.P)
			w.scope = saved
		}
		w.pop()
	}
	w.pop()
	for _, e := range d.Entries {
		if e == nil {
			continue
		}
		w.push(e.Annotations)
		if e.Call != nil {
			for _, a := range e.Call.Args {
				w.node(a)
			}
		}
		if e.Body != nil {
			w.class(e.Body, nil)
		}
		w.pop()
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.FunDecl:
			w.fun(m, d.Sym)
		case *syntax.PropDecl:
			w.prop(m, d.Sym)
		default:
			w.decl(m)
		}
	}
	if d.Sym != nil {
		w.classes = w.classes[:len(w.classes)-1]
	}
	w.pop()
	w.pop()
}

// paramTarget reports whether annotations without a use-site target on
// a constructor property apply to its parameter: those whose class
// allows it, as one without @Target does.
func (w *optWalker) paramTarget(as []syntax.Annotation) bool {
	for i := range as {
		a := &as[i]
		if a.Target != "" {
			continue
		}
		cls := w.o.annClass(a)
		if w.o.marker(cls) == nil {
			continue
		}
		d, ok := cls.Decl.(*syntax.ClassDecl)
		if !ok {
			continue
		}
		t := d.Find("", "Target")
		if t == nil {
			return true
		}
		for _, arg := range t.Args {
			for _, tok := range arg.Toks {
				if tok.Text == "VALUE_PARAMETER" {
					return true
				}
			}
		}
	}
	return false
}

// inheritance checks a supertype that needs opt-in to inherit from.
func (w *optWalker) inheritance(st *syntax.SuperType) {
	var cls *types.ClassSym
	if k, ok := w.o.u.typeUses.Load(st.Type); ok {
		switch k := k.(type) {
		case *types.ClassSym:
			cls = k
		case *types.AliasSym:
			cls = k.Class()
		}
	} else if st.T.K == types.Class {
		cls = st.T.C
	}
	if cls == nil || w.suppressed("OPT_IN_TO_INHERITANCE_ERROR") {
		return
	}
	d, ok := cls.Decl.(*syntax.ClassDecl)
	if !ok {
		return
	}
	for i := range d.Annotations {
		a := &d.Annotations[i]
		if !a.Is("SubclassOptInRequired") {
			continue
		}
		for _, n := range classLitNames(a) {
			m := w.o.marker(w.o.classNamed(n))
			if m == nil || !m.err || w.o.optsIn(w.scope, m, true) {
				continue
			}
			if st.Type != nil {
				w.errorf(st.Type.P, "this class needs opt-in to inherit from '%s': mark the subclass with '@SubclassOptInRequired(%s::class)' or '@OptIn(%s::class)'", cls.DisplayName(), m.simple, m.simple)
			}
		}
	}
}

func (w *optWalker) fun(d *syntax.FunDecl, owner *types.ClassSym) {
	if d == nil {
		return
	}
	w.annotationErrors(d.Annotations)
	w.markerTargetErrors(d.RecvAnnotations, false)
	w.push(d.Annotations)
	if d.Override && owner != nil && d.Sym != nil && !w.suppressed("OPT_IN_OVERRIDE_ERROR") {
		for _, o := range w.c.overridden(owner, d.Sym) {
			if fd, ok := o.Decl.(*syntax.FunDecl); ok {
				w.overrideOf(d.P, w.o.annMarkers(fd.Annotations, ""))
			}
		}
	}
	w.typeRef(d.Recv)
	w.params(d.Params)
	w.typeRef(d.Ret)
	if d.Body != nil {
		w.node(d.Body)
	}
	if d.ExprBody != nil {
		w.node(d.ExprBody)
	}
	w.pop()
}

// overrideOf reports an override at p of a member with markers ms that
// the scope doesn't opt in to.
func (w *optWalker) overrideOf(p syntax.Pos, ms []*optMarker) {
	if w.suppressed("OPT_IN_OVERRIDE_ERROR") {
		return
	}
	for _, m := range ms {
		if !w.o.optsIn(w.scope, m, false) {
			w.errorf(p, "this override needs opt-in: the overridden declaration is marked with '@%s'", m.simple)
		}
	}
}

// propOverride checks an overriding property of class cls.
func (w *optWalker) propOverride(cls *types.ClassSym, p *types.VarSym, pos syntax.Pos) {
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls || s.External {
			return true
		}
		if sp := s.Prop(p.Name); sp != nil {
			w.overrideOf(pos, w.propMarkers(sp, false))
		}
		return true
	})
}

func (w *optWalker) prop(d *syntax.PropDecl, owner *types.ClassSym) {
	if d == nil {
		return
	}
	w.annotationErrors(d.Annotations)
	w.markerTargetErrors(d.Annotations, false)
	w.markerTargetErrors(d.RecvAnnotations, false)
	w.push(d.Annotations)
	if d.Override && owner != nil && d.Sym != nil {
		w.propOverride(owner, d.Sym, d.P)
	}
	w.typeRef(d.Recv)
	w.typeRef(d.Type)
	if d.Field != nil {
		w.prop(d.Field, owner)
	}
	if d.DelegateOf == nil && d.Init != nil {
		w.node(d.Init)
	}
	if d.DelegateOf != nil && d.Init != nil {
		w.node(d.Init)
	}
	for _, acc := range []*syntax.FunDecl{d.Getter, d.Setter} {
		if acc == nil {
			continue
		}
		w.push(acc.Annotations)
		if acc.Body != nil {
			w.node(acc.Body)
		}
		if acc.ExprBody != nil {
			w.node(acc.ExprBody)
		}
		w.pop()
	}
	w.pop()
}

// node walks code.
func (w *optWalker) node(n any) {
	if b, ok := n.(*syntax.Block); ok {
		for _, p := range w.c.extContracts[b] {
			w.require(p, []*optMarker{extendedContracts})
		}
	}
	syntax.Inspect(n, w.visit)
}

// extendedContracts is the marker of the contract DSL's cond implies
// returnsNotNull() and cond holdsIn lambda, whose statements the
// checker drops with their contract { }.
var extendedContracts = &optMarker{simple: "ExperimentalExtendedContracts", qual: "kotlin.contracts.ExperimentalExtendedContracts", err: true}

func (w *optWalker) visit(n any) bool {
	if as := w.c.file.Annotated[n]; len(as) > 0 && w.entered != n {
		w.annotationErrors(as)
		if _, ok := n.(*syntax.VarStmt); ok {
			w.markerTargetErrors(as, true)
		}
		w.push(as)
		prev := w.entered
		w.entered = n
		syntax.Inspect(n, w.visit)
		w.entered = prev
		w.pop()
		return false
	}
	switch n := n.(type) {
	case *syntax.LocalFun:
		if n.Decl != nil {
			w.fun(n.Decl, nil)
		}
		return false
	case *syntax.LocalClass:
		w.class(n.Decl, n.Annotations)
		return false
	case *syntax.ObjectExpr:
		w.class(n.Decl, nil)
		return false
	case *syntax.VarStmt:
		w.typeRef(n.Type)
	case *syntax.Destructure:
		for _, t := range n.Types {
			w.typeRef(t)
		}
	case *syntax.For:
		w.typeRef(n.Type)
		for _, t := range n.Types {
			w.typeRef(t)
		}
		if n.Range != nil {
			for _, f := range []*types.FunSym{n.Iter, n.HasNext, n.Next} {
				if f != nil {
					w.fun1(f, n.Range.Pos(), types.InvalidType)
				}
			}
		}
	case *syntax.Assign:
		w.writes[n.Target] = true
	case *syntax.Is:
		w.typeRef(n.Ref)
	case *syntax.As:
		w.typeRef(n.Ref)
	case *syntax.ClassLit:
		w.typeRef(n.Ref)
	case *syntax.CallableRef:
		w.typeRef(n.Type)
	case *syntax.Lambda:
		for _, p := range n.Params {
			if p == nil {
				continue
			}
			w.typeRef(p.Type)
			for _, t := range p.Types {
				w.typeRef(t)
			}
		}
	case *syntax.Call:
		w.call(n)
	case *syntax.Binary:
		if n.Call != nil {
			w.call(n.Call)
		}
	case *syntax.Unary:
		if n.Call != nil {
			w.call(n.Call)
		}
	case *syntax.Index:
		if n.Call != nil {
			w.call(n.Call)
		}
	case *syntax.Ident:
		w.ident(n)
	case *syntax.Member:
		w.member(n)
	}
	return true
}

// call checks a call.
func (w *optWalker) call(n *syntax.Call) {
	if n == nil {
		return
	}
	for _, a := range n.TypeArgs {
		if a != nil {
			w.typeRef(a.Type)
		}
	}
	p := n.Pos()
	if n.Fun != nil {
		p = n.Fun.Pos()
		if m, ok := n.Fun.(*syntax.Member); ok {
			p = m.P
		}
	}
	if n.Ctor != nil {
		w.require(p, w.o.classMarkers(n.Ctor))
		level := classDeprecation(n.Ctor)
		if id, ok := n.Fun.(*syntax.Ident); ok {
			for _, a := range w.o.aliases[id.Name] {
				if a.Class() == n.Ctor {
					w.require(p, w.o.aliasMarkers(a, 0))
					if d, ok := a.Decl.(*syntax.TypeAliasDecl); ok && level == "" {
						level = deprecation(d.Annotations)
					}
				}
			}
		}
		if n.Sym != nil {
			w.require(p, w.ctorMarkers(n.Sym))
			if level == "" {
				level = w.ctorDeprecation(n.Sym)
			}
		}
		w.deprecatedUse(p, n.Ctor.DisplayName(), level)
		return
	}
	if n.Sym == nil || n.Invoke {
		return
	}
	if n.Sym.Name == "<init>" {
		// A delegation to another constructor: this(...) or super(...).
		w.require(p, w.ctorMarkers(n.Sym))
		w.deprecatedUse(p, "constructor", w.ctorDeprecation(n.Sym))
		return
	}
	w.fun1(n.Sym, p, n.T)
	if _, ok := n.Fun.(*syntax.Ident); ok && n.Sym.Owner != nil && !n.Sym.Owner.External {
		// A member called on an implicit receiver, which is the class's
		// instance only outside of the class and its subclasses.
		inside := false
		for _, cls := range w.classes {
			inside = inside || cls.IsSubclassOf(n.Sym.Owner)
		}
		if !inside {
			w.require(p, w.o.classMarkers(n.Sym.Owner))
		}
	}
}

// fun1 checks a call at p of function f, with result type t.
func (w *optWalker) fun1(f *types.FunSym, p syntax.Pos, t types.Type) {
	var ms []*optMarker
	level := ""
	switch d := f.Decl.(type) {
	case *syntax.FunDecl:
		if d.AccessorOf != nil {
			ms = w.propMarkers(d.AccessorOf, strings.HasPrefix(f.Name, "set"))
			level = w.propDeprecation(d.AccessorOf)
			break
		}
		ms = w.o.annMarkers(d.Annotations, "")
		ms = append(ms, w.o.refMarkers(d.Recv, 0)...)
		for _, prm := range d.Params {
			if prm != nil {
				ms = append(ms, w.o.refMarkers(prm.Type, 0)...)
			}
		}
		if d.Ret != nil {
			ms = append(ms, w.o.refMarkers(d.Ret, 0)...)
		} else {
			ms = append(ms, w.o.typeMarkers(f.Ret)...)
		}
		level = deprecation(d.Annotations)
	case *syntax.LocalFun:
		if d.Decl != nil {
			ms = w.o.annMarkers(d.Decl.Annotations, "")
			level = deprecation(d.Decl.Annotations)
		}
	default:
		return
	}
	if len(f.TypeParams) > 0 && t.K != types.Invalid {
		ms = append(ms, w.o.typeMarkers(t)...)
	}
	w.require(p, ms)
	w.deprecatedUse(p, f.Name, level)
}

// ctorMarkers returns the error-level markers of a constructor of the
// program: its own and those of its parameters' types.
func (w *optWalker) ctorMarkers(f *types.FunSym) []*optMarker {
	switch d := f.Decl.(type) {
	case *syntax.ClassDecl:
		ms := w.o.annMarkers(d.CtorMods.Annotations, "")
		for _, p := range d.CtorParams {
			if p != nil {
				ms = append(ms, w.o.refMarkers(p.Type, 0)...)
			}
		}
		return ms
	case *syntax.SecondaryCtor:
		ms := w.o.annMarkers(d.Annotations, "")
		for _, p := range d.Params {
			if p != nil {
				ms = append(ms, w.o.refMarkers(p.Type, 0)...)
			}
		}
		return ms
	}
	return nil
}

func (w *optWalker) ctorDeprecation(f *types.FunSym) string {
	switch d := f.Decl.(type) {
	case *syntax.ClassDecl:
		return deprecation(d.CtorMods.Annotations)
	case *syntax.SecondaryCtor:
		return deprecation(d.Annotations)
	}
	return ""
}

// propMarkers returns the error-level markers of a property of the
// program, read or, with write, assigned: its own, its getter's or
// setter's, and those of its type.
func (w *optWalker) propMarkers(p *types.VarSym, write bool) []*optMarker {
	acc := "get"
	if write {
		acc = "set"
	}
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		ms := w.o.annMarkers(d.Annotations, "", "property", acc)
		if !write {
			if d.Type != nil {
				ms = append(ms, w.o.refMarkers(d.Type, 0)...)
			} else {
				ms = append(ms, w.o.typeMarkers(p.Type)...)
			}
		}
		return ms
	case *syntax.CtorParam:
		ms := w.o.annMarkers(d.Modifiers.Annotations, "property", acc)
		if !w.paramTarget(d.Modifiers.Annotations) {
			ms = append(ms, w.o.annMarkers(d.Modifiers.Annotations, "")...)
		}
		if !write {
			ms = append(ms, w.o.refMarkers(d.Type, 0)...)
		}
		return ms
	}
	return nil
}

func (w *optWalker) propDeprecation(p *types.VarSym) string {
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return deprecation(d.Annotations, "property", "get")
	case *syntax.CtorParam:
		return deprecation(d.Modifiers.Annotations, "property", "get")
	}
	return ""
}

// variable checks a read or an assignment at p of variable v.
func (w *optWalker) variable(v *types.VarSym, p syntax.Pos, write bool) {
	switch v.Kind {
	case types.Global, types.Prop:
		w.require(p, w.propMarkers(v, write))
		if !write {
			w.deprecatedUse(p, v.Name, w.propDeprecation(v))
		}
	case types.Local, types.Param:
		if !write {
			w.require(p, w.o.typeMarkers(v.Type))
		}
	}
}

func (w *optWalker) ident(n *syntax.Ident) {
	write := w.writes[n]
	if n.Sym != nil {
		w.variable(n.Sym, n.P, write)
		if n.Sym.Kind == types.Prop && n.Sym.Owner != nil && !n.Sym.Owner.External {
			inside := false
			for _, cls := range w.classes {
				inside = inside || cls.IsSubclassOf(n.Sym.Owner)
			}
			if !inside {
				w.require(n.P, w.o.classMarkers(n.Sym.Owner))
			}
		}
	}
	if n.Object != nil {
		w.object(n.Object, n.P)
	}
}

func (w *optWalker) member(n *syntax.Member) {
	if n.Prop != nil {
		w.variable(n.Prop, n.P, w.writes[n])
	}
	if n.Object != nil {
		w.object(n.Object, n.P)
	}
}

// object checks a use at p of an object, or a companion object, as a
// value.
func (w *optWalker) object(cls *types.ClassSym, p syntax.Pos) {
	w.require(p, w.o.classMarkers(cls))
	w.deprecatedUse(p, cls.DisplayName(), classDeprecation(cls))
}
