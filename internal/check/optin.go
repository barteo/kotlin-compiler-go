package check

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
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
// its signature names; an annotation class needs opt-in where it is
// used as an annotation too, @EAnno fun f(). A class annotated @SubclassOptInRequired(Marker)
// needs opt-in to inherit from, which @SubclassOptInRequired(Marker) on
// the subclass also gives, and overriding a member with a marker needs
// it too.
//
// A declaration annotated @Deprecated with level ERROR, or HIDDEN, which
// is otherwise invisible, is an error to use, outside of deprecated code.
//
// The checks run after the bodies are checked, on what the checker
// resolved. They know the program's markers and deprecations from its
// declarations, and the class path's from its class files: the
// descriptors of its classes' and members' annotations, a property's on
// its $annotations method, a typealias's in its metadata, the level of a
// marker's @RequiresOptIn, and @Deprecated and @DeprecatedSinceKotlin,
// whose versions kc takes as reached. An @OptIn of the class path, a
// SOURCE annotation, isn't known: its declarations need the markers of
// those they use, too.

// optInErrors reports the program's uses of declarations that need opt-in
// or are deprecated with level ERROR or HIDDEN, and errors in the markers
// and the version strings of @SinceKotlin and the like.
func optInErrors(checkers []*Checker) {
	if len(checkers) == 0 {
		return
	}
	o := &optPass{u: checkers[0].u, annClasses: map[string][]*types.ClassSym{}, aliases: map[string][]*types.AliasSym{},
		markers: map[*types.ClassSym]*optMarker{}, descs: map[string]*optMarker{}, extClasses: map[*types.ClassSym][]*optMarker{},
		seen: map[optErrKey]bool{}, extProps: map[*types.FunSym]*types.VarSym{}}
	for _, c := range checkers {
		o.collect(c.file.Decls)
	}
	for _, c := range checkers {
		w := &optWalker{o: o, c: c, writes: map[syntax.Expr]bool{}, readWrites: map[syntax.Expr]bool{}}
		w.push(c.file.Annotations)
		w.annotationErrors(c.file.Annotations)
		w.imports(c.file.Imports)
		for _, d := range c.file.Decls {
			w.decl(d)
		}
	}
}

// imports reports the imports of what is nested in one of the program's
// markers: only the marker itself can be imported.
func (w *optWalker) imports(imps []*syntax.Import) {
	for _, imp := range imps {
		w.importDeprecation(imp)
	}
	for _, imp := range imps {
		parts := strings.Split(imp.Path, ".")
		n := len(parts) - 1
		if imp.Star {
			n = len(parts)
		}
	outer:
		for i := 1; i <= n; i++ {
			prefix := strings.Join(parts[:i], ".")
			for _, cls := range w.o.annClasses[parts[i-1]] {
				if qualName(cls) == prefix && w.o.marker(cls) != nil {
					w.errorf(imp.P, "opt-in requirement marker '%s' can only be used as an annotation or as an argument to @OptIn", cls.DisplayName())
					break outer
				}
			}
		}
	}
}

// importDeprecation reports an explicit import of a member or a nested
// class of a deprecated class: import my.C.Companion.a for a deprecated
// C. As with kotlinc, importing the deprecated class itself is no use of
// it.
func (w *optWalker) importDeprecation(imp *syntax.Import) {
	if imp.Star {
		return
	}
	name := imp.Alias
	if name == "" {
		name = imp.Path[strings.LastIndexByte(imp.Path, '.')+1:]
	}
	var cls *types.ClassSym
	if k := w.c.imports[name]; k != nil && qualName(k) == imp.Path {
		cls = w.outerOf(k)
	} else {
		for _, mi := range w.c.memberImportsOf(name) {
			if mi.p == imp.P {
				cls = mi.owner
			}
		}
	}
	for ; cls != nil; cls = w.outerOf(cls) {
		if d := notHidden(classDepr(cls)); d.level != "" {
			w.deprecatedUse(imp.P, cls.DisplayName(), d)
			return
		}
	}
}

// outerOf returns the class cls is nested in, or nil: a Java class's,
// which isn't recorded, by its JVM name, Outer$Nested.
func (w *optWalker) outerOf(cls *types.ClassSym) *types.ClassSym {
	if cls.Outer != nil || !cls.External {
		return cls.Outer
	}
	if i := strings.LastIndexByte(cls.Name, '$'); i > 0 {
		return w.c.u.ext.class(cls.Name[:i])
	}
	return nil
}

// optPass holds what the checks of all files share.
type optPass struct {
	u *Universe
	// annClasses are the program's annotation classes, and aliases its
	// typealiases, by simple name.
	annClasses map[string][]*types.ClassSym
	aliases    map[string][]*types.AliasSym
	markers    map[*types.ClassSym]*optMarker
	// descs are the markers that annotation descriptors of the class path
	// name, nil for those that aren't, and extClasses the error-level
	// markers of classes of the class path.
	descs      map[string]*optMarker
	extClasses map[*types.ClassSym][]*optMarker
	seen       map[optErrKey]bool
	// extProps are the program's extension properties, by their
	// getters' and setters' symbols, whose deprecations are theirs.
	extProps map[*types.FunSym]*types.VarSym
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
		case *syntax.PropDecl:
			if d.Recv != nil && d.Sym != nil {
				for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
					if a != nil && a.Sym != nil {
						o.extProps[a.Sym] = d.Sym
					}
				}
			}
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
	if cls.External && !neverMarker("L"+cls.Name+";") {
		cls.Ensure()
		if cls.OptInLevel != "" {
			q := qualName(cls)
			m = &optMarker{cls: cls, simple: q[strings.LastIndexByte(q, '.')+1:], qual: q, err: cls.OptInLevel != "WARNING"}
		}
	}
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

// descMarkers returns the markers among the annotations of a
// declaration of the class path, by their descriptors: La/b/C;.
func (o *optPass) descMarkers(descs []string) []*optMarker {
	var out []*optMarker
	for _, d := range descs {
		m, ok := o.descs[d]
		if !ok {
			if !neverMarker(d) && len(d) > 2 && d[0] == 'L' && d[len(d)-1] == ';' {
				m = o.marker(o.u.ext.class(d[1 : len(d)-1]))
			}
			o.descs[d] = m
		}
		if m != nil {
			out = append(out, m)
		}
	}
	return out
}

// extNamed returns the class of the class path with this qualified name,
// as written, or nil: a.b.C, or a.b.C.D for a nested class.
func (o *optPass) extNamed(name string) *types.ClassSym {
	if !strings.Contains(name, ".") {
		return nil
	}
	n := strings.ReplaceAll(name, ".", "/")
	for {
		if cls := o.u.ext.class(n); cls != nil {
			if cls.Ensure(); !cls.Missing {
				return cls
			}
		}
		i := strings.LastIndexByte(n, '/')
		if i < 0 {
			return nil
		}
		n = n[:i] + "$" + n[i+1:]
	}
}

// depr is a deprecation: its level, WARNING, ERROR or HIDDEN, and its
// message. The zero depr is no deprecation.
type depr struct {
	level, msg string
}

// isError reports whether the deprecation's level is ERROR or HIDDEN.
func (d depr) isError() bool { return d.level == "ERROR" || d.level == "HIDDEN" }

// or returns d, or e if e's level is higher.
func (d depr) or(e depr) depr {
	rank := func(l string) int { return map[string]int{"WARNING": 1, "ERROR": 2, "HIDDEN": 3}[l] }
	if rank(e.level) > rank(d.level) {
		return e
	}
	return d
}

// deprecationLevel returns a deprecation that the class path gives.
func deprecationLevel(d *types.Deprecation) depr {
	if d == nil || d.Level == "" {
		return depr{}
	}
	return depr{d.Level, d.Message}
}

// annMarkers returns the markers among annotations as whose
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
		if m := o.marker(o.annClass(a)); m != nil {
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

// classMarkers returns the markers of a class: its own, and
// those of the classes it is nested in.
func (o *optPass) classMarkers(cls *types.ClassSym) []*optMarker {
	var out []*optMarker
	for depth := 0; cls != nil && depth < 32; depth++ {
		if d, ok := cls.Decl.(*syntax.ClassDecl); ok {
			out = append(out, o.annMarkers(d.Annotations, "")...)
		} else if cls.External {
			out = append(out, o.extClassMarkers(cls)...)
			if cls.Outer == nil {
				cls = o.extOuter(cls)
				continue
			}
		}
		cls = cls.Outer
	}
	return out
}

// extOuter returns the class a nested class of the class path is declared
// in, which its Outer is only for an inner class: the class its binary
// name is nested in, Outer$Nested.
func (o *optPass) extOuter(cls *types.ClassSym) *types.ClassSym {
	i := strings.LastIndexByte(cls.Name, '$')
	if i <= strings.LastIndexByte(cls.Name, '/')+1 {
		return nil
	}
	if out := o.u.ext.class(cls.Name[:i]); out != nil {
		if out.Ensure(); !out.Missing {
			return out
		}
	}
	return nil
}

// extClassMarkers returns the markers among the annotations of
// a class of the class path.
func (o *optPass) extClassMarkers(cls *types.ClassSym) []*optMarker {
	ms, ok := o.extClasses[cls]
	if ok {
		return ms
	}
	cls.Ensure()
	for _, a := range cls.Annotations {
		if m := o.marker(a); m != nil {
			ms = append(ms, m)
		}
	}
	o.extClasses[cls] = ms
	return ms
}

// aliasMarkers returns the markers of a typealias: its own,
// and those of the classifiers it expands to that it doesn't opt in to.
func (o *optPass) aliasMarkers(a *types.AliasSym, depth int) []*optMarker {
	d, ok := a.Decl.(*syntax.TypeAliasDecl)
	if !ok && a.Decl == nil && depth <= 16 {
		// One of the class path: whether it opts in isn't known.
		return append(o.descMarkers(a.Annotations), o.typeMarkers(a.Type)...)
	}
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

// classifierMarkers returns the markers of a class or a
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

// refMarkers returns the markers of the classifiers a type
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

// typeMarkers returns the markers of the classes a type is
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

// deprecation returns the deprecation of the @Deprecated annotation among
// as, with no use-site target or one of targets.
func deprecation(as []syntax.Annotation, targets ...string) depr {
	var out depr
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
		d := depr{level: "WARNING"}
		if arg := a.Arg(0, "message"); arg != nil && len(arg.Toks) == 1 && arg.Toks[0].Kind == syntax.STRING {
			d.msg, _ = literalString(arg.Toks[0])
		}
		if arg := a.Arg(2, "level"); arg != nil && len(arg.Toks) > 0 {
			switch l := arg.Toks[len(arg.Toks)-1].Text; l {
			case "ERROR", "HIDDEN":
				d.level = l
			}
		}
		out = out.or(d)
	}
	return out
}

// classDeprecation returns the deprecation level of a class, if it is
// ERROR or HIDDEN: "ERROR", "HIDDEN" or "".
func classDeprecation(cls *types.ClassSym) string {
	if d := classDepr(cls); d.isError() {
		return d.level
	}
	return ""
}

// classDepr returns the deprecation of a class.
func classDepr(cls *types.ClassSym) depr {
	if cls == nil {
		return depr{}
	}
	// A compiler plugin's synthetic class, serialization's $serializer, is
	// deprecated with level HIDDEN so that only the plugin's own members,
	// such as serializer(), use it.
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok && !d.Synthetic {
		return deprecation(d.Annotations)
	}
	if cls.External {
		cls.Ensure()
		return deprecationLevel(cls.Deprecated)
	}
	return depr{}
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
	// inClassLit is set while the class of a class literal is walked.
	inClassLit bool
	// writes are the expressions assigned to, and readWrites those a
	// compound assignment or ++ and -- read and assign.
	writes, readWrites map[syntax.Expr]bool
	// hiddenOff is set while deprecations with level HIDDEN aren't
	// reported (see ident).
	hiddenOff bool
	// entered is the annotated node whose annotations were just pushed.
	entered any
	// refReads are the positions of the property reads that callable
	// references' bodies make, which a deprecated getter doesn't warn of.
	refReads map[syntax.Pos]bool
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
	if len(ms) == 0 {
		return
	}
	for _, m := range ms {
		if w.o.optsIn(w.scope, m, false) {
			continue
		}
		switch {
		case m.err && !w.suppressed("OPT_IN_USAGE_ERROR"):
			w.errorf(p, "this declaration needs opt-in: its usage must be marked with '@%s' or '@OptIn(%s::class)'", m.simple, m.simple)
		case !m.err:
			w.warnf(p, "OPT_IN_USAGE", "this declaration needs opt-in. Its usage should be marked with '@%s' or '@OptIn(%s::class)'", m.simple, m.simple)
		}
	}
}

// deprecatedUse reports a use at p of name, deprecated with d.
func (w *optWalker) deprecatedUse(p syntax.Pos, name string, d depr) {
	if w.hiddenOff {
		d = notHidden(d)
	}
	if d.level == "" || w.deprecated() {
		return
	}
	if d.isError() {
		if !w.suppressed("DEPRECATION_ERROR") {
			w.errorf(p, "'%s' is deprecated with level %s", name, d.level)
		}
		return
	}
	if d.msg != "" {
		w.warnf(p, "DEPRECATION", "'%s' is deprecated. %s", name, d.msg)
	} else {
		w.warnf(p, "DEPRECATION", "'%s' is deprecated", name)
	}
}

// warnf reports a warning once at a line, as errorf does an error.
func (w *optWalker) warnf(p syntax.Pos, name, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	k := optErrKey{w.c.file.Path, p.Line, name + "\x00" + msg}
	if w.o.seen[k] {
		return
	}
	w.o.seen[k] = true
	w.c.warnf(p, name, "%s", msg)
}

// typeRef checks the classifiers a type reference names.
func (w *optWalker) typeRef(ref *syntax.TypeRef) {
	refTypes(ref, func(r *syntax.TypeRef) {
		k, ok := w.o.u.typeUses.Load(r)
		if !ok {
			return
		}
		w.require(r.P, w.o.classifierMarkers(k, 0))
		w.annotationAsType(r.P, k, !w.inClassLit)
		switch k := k.(type) {
		case *types.ClassSym:
			w.deprecatedUse(r.P, k.DisplayName(), classDepr(k))
		case *types.AliasSym:
			if d, ok := k.Decl.(*syntax.TypeAliasDecl); ok {
				w.deprecatedUse(r.P, k.Name, deprecation(d.Annotations))
			} else if k.Decl == nil {
				w.deprecatedUse(r.P, k.Name, deprecationLevel(k.Deprecated))
			}
			if cls := w.expansionDeprecated(k.Type, true, 0); cls != nil {
				if !w.deprecated() && !w.suppressed("TYPEALIAS_EXPANSION_DEPRECATION_ERROR") {
					w.errorf(r.P, "the expansion of typealias '%s' uses '%s', which is deprecated", k.Name, cls.DisplayName())
				}
			} else if cls := w.expansionDeprecated(k.Type, false, 0); cls != nil && !w.deprecated() {
				w.warnf(r.P, "TYPEALIAS_EXPANSION_DEPRECATION", "the expansion of typealias '%s' uses '%s', which is deprecated", k.Name, cls.DisplayName())
			}
		}
	})
}

// classLitDeprecation reports the class of class literal n, A::class, a
// class named by its simple name or by a typealias of the program's, if
// it is deprecated, or if the typealias's expansion is.
func (w *optWalker) classLitDeprecation(n *syntax.ClassLit) {
	id, ok := n.X.(*syntax.Ident)
	if !ok || w.deprecated() {
		return
	}
	cls := n.Class.C
	if _, ok := cls.Decl.(*syntax.ClassDecl); ok && cls.Outer == nil && id.Name == cls.DisplayName() {
		if d := classDepr(cls); d.level != "" && !d.isError() {
			w.deprecatedUse(id.P, cls.DisplayName(), d)
		}
		return
	}
	for _, a := range w.o.aliases[id.Name] {
		if _, ok := a.Decl.(*syntax.TypeAliasDecl); ok && a.Class() == cls && w.expansionDeprecated(a.Type, true, 0) == nil {
			if dep := w.expansionDeprecated(a.Type, false, 0); dep != nil {
				w.warnf(id.P, "TYPEALIAS_EXPANSION_DEPRECATION", "the expansion of typealias '%s' uses '%s', which is deprecated", a.Name, dep.DisplayName())
			}
			return
		}
	}
}

// typeParams walks the bounds of type parameters tps.
func (w *optWalker) typeParams(tps []*syntax.TypeParam) {
	for _, tp := range tps {
		for _, b := range tp.Bounds {
			w.typeRef(b)
		}
	}
}

// annotationAsType reports a type that names RequiresOptIn or OptIn, or
// an opt-in requirement marker or a class nested in one, which can only
// be used as annotations (and markers as @OptIn's arguments), through a
// typealias too; but not in the marker's own code. Only with markers are
// markers reported: a class literal can name one.
func (w *optWalker) annotationAsType(p syntax.Pos, k any, markers bool) {
	var cls *types.ClassSym
	switch k := k.(type) {
	case *types.ClassSym:
		cls = k
	case *types.AliasSym:
		if t := k.Type; t.K == types.Class {
			cls = t.C
		}
	}
	if cls == nil {
		return
	}
	switch qualName(cls) {
	case "kotlin.RequiresOptIn", "kotlin.OptIn":
		w.errorf(p, "this class can only be used as an annotation or as an argument to @OptIn")
		return
	}
	if !markers {
		return
	}
	for m := cls; m != nil; m = m.Outer {
		if w.o.marker(m) == nil {
			continue
		}
		if slices.Contains(w.classes, m) {
			return
		}
		w.errorf(p, "opt-in requirement marker '%s' can only be used as an annotation or as an argument to @OptIn", m.DisplayName())
		return
	}
}

// expansionDeprecated returns a class that type t is made of that is
// deprecated with level ERROR or HIDDEN, or, unless errors, with level
// WARNING; or nil.
func (w *optWalker) expansionDeprecated(t types.Type, errors bool, depth int) *types.ClassSym {
	if t.K != types.Class || t.C == nil || depth > 16 {
		return nil
	}
	if d := classDepr(t.C); d.level != "" && d.isError() == errors {
		return t.C
	}
	if t.A != nil {
		for i := range t.A.Len() {
			if cls := w.expansionDeprecated(t.A.Get(i).T, errors, depth+1); cls != nil {
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
	w.push(as)
	for i := range as {
		w.annotationArgs(&as[i], 0)
		// An annotation class that needs opt-in, used as an annotation,
		// which the declaration's annotations may opt in to.
		if a := &as[i]; !a.Implicit {
			if cls := w.o.annClass(a); cls != nil {
				w.require(annotationNamePos(a), w.o.classMarkers(cls))
			}
		}
	}
	w.pop()
	for i := range as {
		a := &as[i]
		cls := w.o.annClass(a)
		if cls != nil {
			// A deprecated annotation class, used as an annotation.
			w.deprecatedUse(a.P, cls.DisplayName(), notHidden(classDepr(cls)))
		}
		if a.Is("OptIn") && !a.Implicit {
			w.optInArgs(a)
		}
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

// optInArgs warns of an @OptIn that opts in to nothing: one without
// arguments, and its arguments that aren't the classes of opt-in
// requirement markers.
func (w *optWalker) optInArgs(a *syntax.Annotation) {
	values, ok := w.o.u.optInValues[a]
	if !ok {
		return // not kotlin.OptIn, or its arguments have errors
	}
	if len(a.Args) == 0 {
		w.c.warnf(a.P, "OPT_IN_WITHOUT_ARGUMENTS", "'@OptIn' without arguments has no effect")
		return
	}
	for _, v := range values {
		for _, e := range v.Elems {
			lit, ok := e.X.(*syntax.ClassLit)
			if !ok || lit.Class.K != types.Class || lit.Class.C == nil || lit.Class.C.Missing || w.o.marker(lit.Class.C) != nil {
				continue
			}
			w.c.warnf(lit.P, "OPT_IN_ARGUMENT_IS_NOT_MARKER", "annotation '%s' is not annotated with '@RequiresOptIn'. '@OptIn' has no effect", lit.Class.C.DisplayName())
		}
	}
}

// annotationNamePos returns where the name of annotation a is, after
// the @ and a use-site target.
func annotationNamePos(a *syntax.Annotation) syntax.Pos {
	p := a.P
	p.Col++
	if a.Target != "" {
		p.Col += len(a.Target) + 1
	}
	return p
}

// annotationArgs checks the arguments of annotation a, and of the
// annotations among them, as the code they are.
func (w *optWalker) annotationArgs(a *syntax.Annotation, depth int) {
	if depth > 16 {
		return
	}
	for _, v := range a.Values {
		for _, e := range v.Elems {
			switch {
			case e.X != nil:
				w.node(e.X)
			case e.Nested != nil:
				w.annotationArgs(e.Nested, depth+1)
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
		if !subclassable(d) {
			kind := "a final class"
			switch {
			case d.Local || d.Anonymous:
				kind = "a local class"
			case d.Fun:
				kind = "a fun interface"
			case d.Sealed:
				kind = "a sealed class"
			case d.Enum:
				kind = "an enum class"
			case d.Object:
				kind = "an object"
			}
			w.errorf(a.P, "'@SubclassOptInRequired' is not applicable to %s", kind)
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

// subclassable reports whether class d is one that code elsewhere can
// subclass, which @SubclassOptInRequired asks an opt-in of: an open or
// abstract class or an interface, but not a sealed, local or functional
// one, an enum class or an object.
func subclassable(d *syntax.ClassDecl) bool {
	if d.Object || d.Enum || d.Annotation || d.Local || d.Anonymous || d.Sealed || d.Fun || d.Data || d.Value {
		return false
	}
	return d.Open || d.Abstract || d.Interface
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
		w.typeParams(d.TypeParams)
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
		w.annotationErrors(p.Annotations)
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
	for i := range extra {
		// A local class's annotations, which it doesn't keep.
		if a := &extra[i]; a.Is("SubclassOptInRequired") {
			w.errorf(a.P, "'@SubclassOptInRequired' is not applicable to a local class")
		}
	}
	w.push(extra)
	w.push(d.Annotations)
	if d.Sym != nil {
		w.classes = append(w.classes, d.Sym)
	}
	w.typeParams(d.TypeParams)
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
		if e.Body == nil && e.Call != nil && e.Call.Sym != nil {
			// The entry calls the enum's constructor, as kotlinc's
			// CheckOptInOnPureEnumEntries checks without arguments too.
			w.require(e.P, w.ctorMarkers(e.Call.Sym))
		}
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
		// kotlinc doesn't check a compiler plugin's code: serialization's
		// calls internal API such as createSimpleEnumSerializer.
		if generated(m) {
			continue
		}
		switch m := m.(type) {
		case *syntax.FunDecl:
			w.fun(m, d.Sym)
		case *syntax.PropDecl:
			w.prop(m, d.Sym)
		case *syntax.ClassDecl:
			if !m.EnumEntry {
				w.decl(m)
			} // an entry's body, which the entries walk with its annotations
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
	var ms []*optMarker
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok {
		for i := range d.Annotations {
			a := &d.Annotations[i]
			if !a.Is("SubclassOptInRequired") {
				continue
			}
			for _, n := range classLitNames(a) {
				k := w.o.classNamed(n)
				if k == nil {
					k = w.o.extNamed(n)
				}
				ms = append(ms, w.o.marker(k))
			}
		}
	} else if cls.External {
		cls.Ensure()
		for _, n := range cls.SubclassOptIn {
			ms = append(ms, w.o.marker(w.o.u.ext.class(n)))
		}
	}
	for _, m := range ms {
		if m == nil || w.o.optsIn(w.scope, m, true) {
			continue
		}
		if !m.err {
			if st.Type != nil {
				w.warnf(st.Type.P, "OPT_IN_TO_INHERITANCE", "this class or interface requires opt-in to be implemented. Its usage should be marked with '@%s', '@OptIn(%s::class)' or '@SubclassOptInRequired(%s::class)'", m.simple, m.simple, m.simple)
			}
			continue
		}
		if st.Type != nil {
			w.errorf(st.Type.P, "this class needs opt-in to inherit from '%s': mark the subclass with '@SubclassOptInRequired(%s::class)' or '@OptIn(%s::class)'", cls.DisplayName(), m.simple, m.simple)
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
				w.overrideOf(d.P, w.o.annMarkers(fd.Annotations, ""), o.Owner)
			} else if o.Decl == nil && o.Owner != nil && o.Owner.External {
				w.overrideOf(d.P, w.o.descMarkers(o.Annotations), o.Owner)
			}
		}
	}
	w.typeParams(d.TypeParams)
	w.typeRef(d.Recv)
	w.params(d.Params)
	w.typeRef(d.Ret)
	if p, ok := w.c.contractAt[d.Body]; ok && d.Body != nil {
		// kotlin.contracts.contract, which dropContracts took out of
		// the body, is @ExperimentalContracts.
		if m := w.o.marker(w.o.extNamed("kotlin.contracts.ExperimentalContracts")); m != nil {
			w.require(p, []*optMarker{m})
			for _, q := range w.c.contractCalls[d.Body] {
				w.require(q, []*optMarker{m})
			}
		}
	}
	if d.Body != nil {
		w.node(d.Body)
	}
	if d.ExprBody != nil {
		w.node(d.ExprBody)
	}
	w.pop()
}

// overrideOf reports an override at p of a member of class super with
// markers ms that the scope doesn't opt in to: an error, or for a
// warning-level marker, kotlinc's warning OPT_IN_OVERRIDE.
func (w *optWalker) overrideOf(p syntax.Pos, ms []*optMarker, super *types.ClassSym) {
	if w.suppressed("OPT_IN_OVERRIDE_ERROR") {
		return
	}
	for _, m := range ms {
		switch {
		case w.o.optsIn(w.scope, m, false):
		case m.err:
			w.errorf(p, "this override needs opt-in: the overridden declaration is marked with '@%s'", m.simple)
		case super != nil:
			w.warnf(p, "OPT_IN_OVERRIDE", "base declaration of supertype '%s' needs opt-in. The declaration override should be annotated with '@%s' or '@OptIn(%s::class)'", simpleName(super), m.simple, m.simple)
		}
	}
}

// propOverride checks an overriding property of class cls.
func (w *optWalker) propOverride(cls *types.ClassSym, p *types.VarSym, pos syntax.Pos) {
	cls.Supertypes(func(s *types.ClassSym) bool {
		if s == cls {
			return true
		}
		if s.External {
			if kp := s.KProps[p.Name]; kp != nil {
				w.overrideOf(pos, slices.Concat(w.o.descMarkers(kp.Getter.Annotations), w.o.descMarkers(kp.Getter.PropAnnotations)), s)
			}
			return true
		}
		if sp := s.Prop(p.Name); sp != nil {
			w.overrideOf(pos, w.propMarkers(sp, false), s)
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
	w.typeParams(d.TypeParams)
	w.typeRef(d.Recv)
	w.typeRef(d.Type)
	if d.Field != nil {
		w.prop(d.Field, owner)
	}
	if d.DelegateOf == nil && d.Init != nil {
		w.node(d.Init)
	}
	if d.DelegateOf != nil && d.Init != nil {
		// The delegate is the property's code: its opt-ins are in scope.
		w.push(d.DelegateOf.Annotations)
		w.node(d.Init)
		w.pop()
	}
	for i, acc := range []*syntax.FunDecl{d.Getter, d.Setter} {
		if acc == nil {
			continue
		}
		// An accessor's own annotations are the property's, with the
		// accessor as their target: in its body they are its own.
		target := [2]string{"get", "set"}[i]
		as := slices.Clone(acc.Annotations)
		for _, a := range d.Annotations {
			if a.Target == target {
				a.Target = ""
				as = append(as, a)
			}
		}
		w.push(as)
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
	syntax.Inspect(n, w.visit)
}

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
		if n.Op != syntax.ASSIGN {
			w.readWrites[n.Target] = true
		}
	case *syntax.Is:
		w.typeRef(n.Ref)
	case *syntax.As:
		w.typeRef(n.Ref)
	case *syntax.ClassLit:
		w.inClassLit = true
		w.typeRef(n.Ref)
		w.inClassLit = false
		if n.Ref == nil && n.X != nil && !n.Bound && n.Class.K == types.Class {
			// A class named as an expression, RequiresOptIn::class.
			w.annotationAsType(n.X.Pos(), n.Class.C, false)
			w.classLitDeprecation(n)
		}
	case *syntax.CallableRef:
		w.typeRef(n.Type)
	case *syntax.Lambda:
		if n.Ref != nil && n.Body != nil {
			syntax.Inspect(n.Body, func(m any) bool {
				switch m := m.(type) {
				case *syntax.Ident:
					if m.Name == n.Ref.Name {
						w.refRead(m.P)
					}
				case *syntax.Member:
					if m.Name == n.Ref.Name {
						w.refRead(m.P)
					}
				}
				return true
			})
		}
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
		if n.Op == syntax.INC || n.Op == syntax.DEC {
			w.writes[n.X] = true
			w.readWrites[n.X] = true
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
	w.javaPrivateKotlin(n, p)
	if n.Ctor != nil {
		w.require(p, w.o.classMarkers(n.Ctor))
		level := classDepr(n.Ctor)
		var alias *types.AliasSym
		var aliasLevel depr
		if id, ok := n.Fun.(*syntax.Ident); ok {
			for _, a := range w.o.aliases[id.Name] {
				if a.Class() == n.Ctor {
					alias = a
					w.require(p, w.o.aliasMarkers(a, 0))
					if d, ok := a.Decl.(*syntax.TypeAliasDecl); ok {
						aliasLevel = aliasLevel.or(deprecation(d.Annotations))
					}
				}
			}
		}
		if n.Sym != nil {
			w.require(p, w.ctorMarkers(n.Sym))
			level = level.or(w.ctorDeprecation(n.Sym))
		}
		if alias != nil && level.level != "" && !level.isError() {
			// A class or constructor deprecated with level WARNING that
			// a typealias names: kotlinc warns of the expansion.
			if !w.deprecated() {
				w.warnf(p, "TYPEALIAS_EXPANSION_DEPRECATION", "the expansion of typealias '%s' uses '%s', which is deprecated", alias.Name, n.Ctor.DisplayName())
			}
			level = depr{}
		} else if alias != nil && level.level == "" && w.expansionDeprecated(alias.Type, true, 0) == nil {
			// One of its type arguments, as ArrayList<Deprecated>.
			if dep := w.expansionDeprecated(alias.Type, false, 0); dep != nil && !w.deprecated() {
				w.warnf(p, "TYPEALIAS_EXPANSION_DEPRECATION", "the expansion of typealias '%s' uses '%s', which is deprecated", alias.Name, dep.DisplayName())
			}
		}
		w.deprecatedUse(p, n.Ctor.DisplayName(), level.or(aliasLevel))
		return
	}
	if n.SAM && n.T.K == types.Class {
		// A SAM constructor, Runnable { }: the interface's markers and
		// its abstract method's.
		w.require(p, w.o.classMarkers(n.T.C))
		if f, _ := w.c.samTarget(n.T); f != nil {
			if d, ok := f.Decl.(*syntax.FunDecl); ok {
				w.require(p, w.o.annMarkers(d.Annotations, ""))
			} else if f.Decl == nil {
				w.require(p, w.o.descMarkers(f.Annotations))
			}
		}
	}
	if n.Sym == nil && charToNumber(n) {
		// Char's conversions to numbers, built in: 'c'.toInt().
		w.deprecatedUse(p, n.Fun.(*syntax.Member).Name, depr{"WARNING", "Conversion of Char to Number is deprecated. Use Char.code property instead."})
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
	if w.c.deprecatedListSort(n.Sym) && !superCall(n) && !w.deprecated() && !w.suppressed("DEPRECATION_ERROR") {
		w.errorf(p, "'sort' is deprecated with level ERROR: use sortWith(comparator) instead")
	}
	if w.c.hiddenJDKMember(n.Sym) && !superCall(n) {
		w.errorf(p, "unresolved reference: %s", n.Sym.Name)
		return
	}
	w.fun1(n.Sym, p, n.T)
	if onImplicitRecv(n.Fun) && n.Sym.Owner != nil && !n.Sym.Owner.External {
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

// onImplicitRecv reports whether fun, a call's callee, is called on an
// implicit receiver: f in f(), or a lambda's receiver.
func onImplicitRecv(fun syntax.Expr) bool {
	switch f := fun.(type) {
	case *syntax.Ident:
		return true
	case *syntax.Member:
		t, ok := f.X.(*syntax.This)
		return ok && t.Implicit && t.Recv != nil
	}
	return false
}

// javaPrivateKotlin reports call n, at p, of a constructor or method of
// a Java class that extends a private Kotlin class of the program, or on
// such a class's instance: the JVM doesn't let code outside the private
// class's file use it.
func (w *optWalker) javaPrivateKotlin(n *syntax.Call, p syntax.Pos) {
	var cls *types.ClassSym
	switch {
	case n.Ctor != nil:
		cls = n.Ctor
	case n.Sym != nil && !n.Invoke:
		cls = n.Sym.Owner
	}
	if !w.c.extendsPrivateKotlin(cls) {
		if m, ok := n.Fun.(*syntax.Member); !ok || n.Sym == nil || n.Sym.Owner == nil || !w.c.extendsPrivateKotlin(m.X.Type().NonNull().C) {
			return
		}
	}
	w.errorf(p, "this Java class extends a private Kotlin class, which can't be used here")
}

// deprecatedListSort reports whether f is java.util.List's
// sort(Comparator), an override of it, or the standard library's
// MutableList<T>.sort(Comparator), which Kotlin deprecates with level
// ERROR for sortWith.
func (c *Checker) deprecatedListSort(f *types.FunSym) bool {
	if f.Name != "sort" || len(f.Params) != 1 || f.Static {
		return false
	}
	t := f.Params[0].Type.NonNull()
	comparator := t.K == types.Class && t.C.Name == "java/util/Comparator"
	if f.Ext {
		// And its sort(comparison: (T, T) -> Int).
		r := f.Recv.NonNull()
		return f.Owner == nil && r.K == types.Class && r.C.Name == "java/util/List" && (comparator || t.IsFunc())
	}
	return comparator && f.Owner != nil && c.isSubclassOf(f.Owner, "java/util/List")
}

// superCall reports whether n calls a method on super.
func superCall(n *syntax.Call) bool {
	m, ok := n.Fun.(*syntax.Member)
	if !ok {
		return false
	}
	_, ok = m.X.(*syntax.Super)
	return ok
}

// extendsPrivateKotlin reports whether cls is a class of the program's
// Java sources that extends a private Kotlin class of the program.
func (c *Checker) extendsPrivateKotlin(cls *types.ClassSym) bool {
	if cls == nil || !c.u.ext.javaSource(cls) {
		return false
	}
	for s := cls.Super; s != nil; s = s.Super {
		if d, ok := s.Decl.(*syntax.ClassDecl); ok && !s.External && d.Visibility == syntax.Private {
			return true
		}
	}
	return false
}

// fun1 checks a call at p of function f, with result type t.
func (w *optWalker) fun1(f *types.FunSym, p syntax.Pos, t types.Type) {
	w.funNamed(f, f.Name, p, t)
}

// funNamed is fun1, with the name a deprecation error gives the function.
func (w *optWalker) funNamed(f *types.FunSym, name string, p syntax.Pos, t types.Type) {
	if f.Decl == nil && f.Declared != nil {
		w.inlineTarget(p, f.Declared.Class)
	}
	var ms []*optMarker
	var level depr
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
	case nil:
		// A function of the class path: its markers, those of the
		// property it is an accessor of, and those of its signature's
		// types.
		ms = slices.Concat(w.o.descMarkers(f.Annotations), w.o.descMarkers(f.PropAnnotations), w.o.typeMarkers(f.Recv))
		for _, prm := range f.Params {
			ms = append(ms, w.o.typeMarkers(prm.Type)...)
		}
		ms = append(ms, w.o.typeMarkers(f.Ret)...)
		level = deprecationLevel(f.Deprecated)
		if p := componentProp(f); p != nil {
			// A data class's componentN reads its property.
			ms = append(ms, w.propMarkers(p, false)...)
		}
	default:
		return
	}
	if len(f.TypeParams) > 0 && t.K != types.Invalid {
		ms = append(ms, w.o.typeMarkers(t)...)
	}
	w.require(p, ms)
	w.deprecatedUse(p, name, level.or(w.c.jdkOverrideDepr(f)))
}

// componentProp returns the property that f, the componentN function
// of a data class of the program, returns, or nil.
func componentProp(f *types.FunSym) *types.VarSym {
	if f.Owner == nil || !strings.HasPrefix(f.Name, "component") {
		return nil
	}
	d, ok := f.Owner.Decl.(*syntax.ClassDecl)
	if !ok || !d.Data {
		return nil
	}
	i, err := strconv.Atoi(strings.TrimPrefix(f.Name, "component"))
	if err != nil || i < 1 || i > len(d.CtorParams) || d.CtorParams[i-1] == nil {
		return nil
	}
	return d.CtorParams[i-1].PropSym
}

// ctorMarkers returns the markers of a constructor of the
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
	case nil:
		ms := w.o.descMarkers(f.Annotations)
		for _, p := range f.Params {
			ms = append(ms, w.o.typeMarkers(p.Type)...)
		}
		return ms
	}
	return nil
}

func (w *optWalker) ctorDeprecation(f *types.FunSym) depr {
	switch d := f.Decl.(type) {
	case *syntax.ClassDecl:
		return deprecation(d.CtorMods.Annotations)
	case *syntax.SecondaryCtor:
		return deprecation(d.Annotations)
	case nil:
		return deprecationLevel(f.Deprecated)
	}
	return depr{}
}

// propMarkers returns the markers of a property of the
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
	case nil:
		// A top-level property of the class path.
		ms := w.o.descMarkers(p.Annotations)
		if !write && p.Kind == types.Global {
			ms = append(ms, w.o.typeMarkers(p.Type)...)
		}
		return ms
	}
	return nil
}

func (w *optWalker) refRead(p syntax.Pos) {
	if w.refReads == nil {
		w.refReads = map[syntax.Pos]bool{}
	}
	w.refReads[p] = true
}

func (w *optWalker) propDeprecation(p *types.VarSym) depr {
	return w.propReadDeprecation(p, false)
}

// propReadDeprecation returns the deprecation of a read of property p,
// or with ref of a reference to it, which a deprecated getter's level
// WARNING doesn't make deprecated.
func (w *optWalker) propReadDeprecation(p *types.VarSym, ref bool) depr {
	targets := []string{"property", "all", "get"}
	if ref {
		targets = targets[:2]
	}
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		if ref {
			return deprecation(d.Annotations, targets...).or(onlyErrors(deprecation(d.Annotations, "get")))
		}
		return deprecation(d.Annotations, targets...)
	case *syntax.CtorParam:
		if ref {
			return deprecation(d.Modifiers.Annotations, targets...).or(onlyErrors(deprecation(d.Modifiers.Annotations, "get")))
		}
		return deprecation(d.Modifiers.Annotations, targets...)
	case nil:
		return deprecationLevel(p.Deprecated)
	}
	return depr{}
}

// propWriteDeprecation returns the deprecation of an assignment of
// property p: its own, or its setter's.
func (w *optWalker) propWriteDeprecation(p *types.VarSym) depr {
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		return deprecation(d.Annotations, "property", "all", "set")
	case *syntax.CtorParam:
		return deprecation(d.Modifiers.Annotations, "property", "all", "set")
	case nil:
		return deprecationLevel(p.Deprecated)
	}
	return depr{}
}

// variable checks a read or an assignment at p of variable v.
func (w *optWalker) variable(v *types.VarSym, p syntax.Pos, write bool) {
	if v.Kind == types.Global && (write && v.InlineSetter || !write && v.InlineGetter) {
		class := v.Part
		if class == "" {
			class = v.Facade
		}
		w.inlineTarget(p, class)
	}
	switch v.Kind {
	case types.Global, types.Prop:
		w.require(p, w.propMarkers(v, write))
		if !write {
			w.deprecatedUse(p, v.Name, w.propReadDeprecation(v, w.refReads[p]))
		} else {
			// A property, or a setter, deprecated with level HIDDEN
			// can't be assigned, as it can't be read: K2 reports the
			// assignment's name unresolved.
			w.deprecatedUse(p, v.Name, w.propWriteDeprecation(v))
		}
	case types.Local, types.Param:
		if !write {
			w.require(p, w.o.typeMarkers(v.Type))
		}
	case types.Static:
		if !write {
			w.require(p, w.entryMarkers(v))
		}
		w.deprecatedUse(p, v.Name, w.staticDepr(v))
	}
}

// staticDepr returns the deprecation of v, a static field or an enum
// entry: of the program's enum's entry, by its annotations.
func (w *optWalker) staticDepr(v *types.VarSym) depr {
	if cls := v.Owner; cls != nil {
		if d, ok := cls.Decl.(*syntax.ClassDecl); ok {
			for _, e := range d.Entries {
				if e != nil && e.Name == v.Name {
					return deprecation(e.Annotations)
				}
			}
			return depr{}
		}
	}
	return deprecationLevel(v.Deprecated)
}

// entryMarkers returns the markers of v if it is an enum entry: those it
// is annotated with, in the program or in its class file.
func (w *optWalker) entryMarkers(v *types.VarSym) []*optMarker {
	cls := v.Owner
	if cls == nil || !slices.Contains(cls.Entries, v.Name) {
		return nil
	}
	if d, ok := cls.Decl.(*syntax.ClassDecl); ok {
		for _, e := range d.Entries {
			if e != nil && e.Name == v.Name {
				return w.o.annMarkers(e.Annotations, "")
			}
		}
		return nil
	}
	return w.o.descMarkers(v.Annotations)
}

func (w *optWalker) ident(n *syntax.Ident) {
	write := w.writes[n]
	if n.Sym != nil {
		if w.readWrites[n] {
			w.variable(n.Sym, n.P, false)
		}
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
	if n.Sym == nil && !write {
		w.accessor(n.Name, n.Getter, nil, n.P, false)
	}
	if n.Member != nil {
		// A member of an implicit receiver: test in an extension of its
		// class.
		// K2 resolves around a member deprecated with level HIDDEN,
		// which kc's resolution of these doesn't: only warnings and
		// errors are reported.
		w.hiddenOff = true
		if w.readWrites[n] {
			w.memberAccess(n.Member, false)
		}
		w.memberAccess(n.Member, write)
		w.hiddenOff = false
	}
	if n.Object != nil {
		w.object(n.Object, n.P)
		w.aliasObject(n.Name, n.Object, n.P)
	}
}

// aliasObject checks a typealias named name, used at p as the value of
// object obj or of the companion object obj of the class it expands to.
func (w *optWalker) aliasObject(name string, obj *types.ClassSym, p syntax.Pos) {
	for _, a := range w.o.aliases[name] {
		if c := a.Class(); c == nil || c != obj && c.Companion != obj {
			continue
		}
		w.require(p, w.o.aliasMarkers(a, 0))
		if d, ok := a.Decl.(*syntax.TypeAliasDecl); ok {
			w.deprecatedUse(p, a.Name, notHidden(deprecation(d.Annotations)))
		}
	}
}

// accessor checks a read, or with write an assignment, at p of property
// name, of the class path or an extension property of the program, by its
// getter and setter.
func (w *optWalker) accessor(name string, get, set *types.FunSym, p syntax.Pos, write bool) {
	if write && set == nil && get != nil {
		set = get.ExtSetter
	}
	f := get
	if write && set != nil {
		f = set
	}
	if f == nil {
		return
	}
	if v := w.o.extProps[f]; v != nil {
		// The program's extension property, or its accessor, deprecated:
		// with level HIDDEN, K2 reports the name unresolved.
		if f == set {
			w.deprecatedUse(p, name, w.propWriteDeprecation(v))
		} else {
			w.deprecatedUse(p, name, w.propReadDeprecation(v, w.refReads[p]))
		}
	} else if f.Decl == nil {
		w.funNamed(f, name, p, types.Type{})
	} else {
		// A synthetic property of the program's override of a Java
		// getter that K2 deprecates: first of getFirst().
		w.deprecatedUse(p, name, w.c.jdkOverrideDepr(f))
	}
}

func (w *optWalker) member(n *syntax.Member) {
	if w.readWrites[n] {
		w.memberAccess(n, false)
	}
	// An assignment of an implicit receiver's member, whose setter is
	// deprecated with level HIDDEN: K2 resolves around it, to a top-level
	// property of the name, which kc's resolution doesn't.
	if this, ok := n.X.(*syntax.This); ok && this.Implicit && w.writes[n] {
		w.hiddenOff = true
	}
	w.memberAccess(n, w.writes[n])
	w.hiddenOff = false
	if n.Object != nil {
		w.object(n.Object, n.P)
	}
	if n.Static != nil && n.X != nil {
		// The class of a static member: JavaClass.f().
		w.require(n.X.Pos(), w.o.classMarkers(n.Static))
		w.deprecatedUse(n.X.Pos(), n.Static.DisplayName(), classDepr(n.Static))
	}
}

// memberAccess checks a read, or with write an assignment, of the
// property or field n.
func (w *optWalker) memberAccess(n *syntax.Member, write bool) {
	switch {
	case n.Prop != nil:
		w.variable(n.Prop, n.P, write)
	case n.Field != nil:
		w.variable(n.Field, n.P, write)
	default:
		w.accessor(n.Name, n.Getter, n.Setter, n.P, write)
	}
}

// object checks a use at p of an object, or a companion object, as a
// value. A companion object is named by its class, D in D.f(), whose
// deprecation applies.
func (w *optWalker) object(cls *types.ClassSym, p syntax.Pos) {
	w.require(p, w.o.classMarkers(cls))
	w.annotationAsType(p, cls, true)
	level := classDepr(cls)
	if o := cls.CompanionOf; o != nil {
		if d := classDepr(o); level.or(d) != level {
			cls, level = o, d
		}
	}
	w.deprecatedUse(p, cls.DisplayName(), level)
}

// inlineTarget reports a use at p of an inline function or property
// accessor of class, a class file of the class path, if its version is
// higher than the program's JVM target, as kotlinc's
// INLINE_FROM_HIGHER_PLATFORM: its body, which the JVM of the target may
// not run, would be copied into the program's classes.
func (w *optWalker) inlineTarget(p syntax.Pos, class string) {
	target := w.c.u.jvmTarget
	if class == "" || w.c.u.ext.cp == nil {
		return
	}
	data, err := w.c.u.ext.cp.Bytes(class)
	if err != nil {
		return
	}
	if major := classfile.Major(data); major > target && !w.suppressed("INLINE_FROM_HIGHER_PLATFORM") {
		w.errorf(p, "cannot inline bytecode built with JVM target %s into bytecode that is being built with JVM target %s: specify a proper '-jvm-target' option",
			classfile.TargetName(major), classfile.TargetName(target))
	}
}

// generated reports whether a compiler plugin of kc's declares d.
func generated(d syntax.Decl) bool {
	switch d := d.(type) {
	case *syntax.FunDecl:
		return d.Generated
	case *syntax.PropDecl:
		return d.Generated
	case *syntax.ClassDecl:
		return d.Generated || d.Synthetic
	}
	return false
}

// onlyErrors returns d if it is an error, or no deprecation.
func onlyErrors(d depr) depr {
	if d.isError() {
		return d
	}
	return depr{}
}

// charToNumber reports whether n is a call of one of Char's conversions to
// a number, which the standard library deprecates for Char.code.
func charToNumber(n *syntax.Call) bool {
	switch n.Builtin {
	case types.ToInt, types.ToLong, types.ToDouble, types.ToFloat, types.ToByte, types.ToShort:
	default:
		return false
	}
	m, ok := n.Fun.(*syntax.Member)
	return ok && m.X != nil && m.X.Type().NonNull() == types.CharType
}

// notHidden returns d, or no deprecation if its level is HIDDEN.
func notHidden(d depr) depr {
	if d.level == "HIDDEN" {
		return depr{}
	}
	return d
}
