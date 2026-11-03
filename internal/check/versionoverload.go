package check

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// Version overloading, @IntroducedAt("v") on optional parameters, has
// kotlinc generate an overload of the function without the parameters of
// each newer version. kotlinc's FirVersionOverloadsChecker requires the
// function to be final, not local and in a final, non-annotation class;
// the versions to ascend; the parameters that have one to be optional
// and not vararg, and no parameter without one after them to be required
// unless it is a trailing lambda; and the default values not to use
// newer parameters, nor object expressions. versionOverloads checks
// those of the file's functions and constructors.

// versionSite is where a function with parameters is declared.
type versionSite struct {
	cls   *syntax.ClassDecl // the class it is a member or constructor of, or nil
	local bool              // in a function, or a local class or object expression
	ctor  bool
}

// versionOverloads checks the version overloads of the file's
// declarations.
func (c *Checker) versionOverloads() {
	c.versionDecls(c.file.Decls, nil, false)
}

func (c *Checker) versionDecls(ds []syntax.Decl, cls *syntax.ClassDecl, local bool) {
	site := versionSite{cls: cls, local: local}
	for _, d := range ds {
		switch d := d.(type) {
		case *syntax.FunDecl:
			c.versionFun(d, site)
			c.versionLocals(d, local)
		case *syntax.PropDecl:
			c.versionLocals(d.Init, local)
			for _, acc := range []*syntax.FunDecl{d.Getter, d.Setter} {
				if acc != nil {
					c.versionFun(acc, site)
					c.versionLocals(acc, local)
				}
			}
		case *syntax.InitBlock:
			c.versionLocals(d.Body, local)
		case *syntax.SecondaryCtor:
			c.versionParams(&d.Modifiers, d.P, d.Params, nil, versionSite{cls: cls, local: local, ctor: true})
			c.versionLocals(d.Body, local)
		case *syntax.ClassDecl:
			if len(d.CtorParams) > 0 {
				ps := make([]*syntax.Param, len(d.CtorParams))
				for i, cp := range d.CtorParams {
					ps[i] = &cp.Param
				}
				c.versionParams(&d.CtorMods, d.P, ps, d.CtorParams, versionSite{cls: d, local: local, ctor: true})
				for _, cp := range d.CtorParams {
					c.versionLocals(cp.Default, local)
				}
			}
			c.versionDecls(d.Members, d, local)
		}
	}
}

// versionLocals checks the local functions, local classes and object
// expressions in n, the body of a function or an initializer.
func (c *Checker) versionLocals(n any, local bool) {
	if d, ok := n.(*syntax.FunDecl); ok {
		if d == nil {
			return
		}
		for _, p := range d.Params {
			c.versionLocals(p.Default, local)
		}
		c.versionLocals(d.Body, local)
		c.versionLocals(d.ExprBody, local)
		return
	}
	if x, ok := n.(syntax.Expr); ok && x == nil {
		return
	}
	syntax.Inspect(n, func(n any) bool {
		switch n := n.(type) {
		case *syntax.LocalFun:
			c.versionFun(n.Decl, versionSite{local: true})
		case *syntax.LocalClass:
			c.versionDecls([]syntax.Decl{n.Decl}, nil, true)
		case *syntax.ObjectExpr:
			c.versionDecls(n.Decl.Members, n.Decl, true)
		}
		return true
	})
}

// versionFun checks the version overloads of function d.
func (c *Checker) versionFun(d *syntax.FunDecl, site versionSite) {
	if d == nil {
		return
	}
	c.versionParams(&d.Modifiers, d.P, d.Params, nil, site)
}

// introducedAt returns the @IntroducedAt of annotations anns.
func introducedAt(anns []syntax.Annotation) *syntax.Annotation {
	for i := range anns {
		if a := &anns[i]; a.Is("IntroducedAt") && (a.Target == "" || a.Target == "param") {
			return a
		}
	}
	return nil
}

// versionParams checks the version overloads of the function or
// constructor at p with modifiers m and parameters ps, which are those
// of the constructor ctor if it is a primary one.
func (c *Checker) versionParams(m *syntax.Modifiers, p syntax.Pos, ps []*syntax.Param, ctor []*syntax.CtorParam, site versionSite) {
	anns := make([]*syntax.Annotation, len(ps))
	found := false
	for _, cp := range m.Contexts {
		if a := introducedAt(cp.Annotations); a != nil {
			c.errorf(a.P, "'@IntroducedAt' cannot be used on receiver or context parameters")
			found = true
		}
	}
	for i, q := range ps {
		anns[i] = introducedAt(q.Annotations)
		if anns[i] == nil && ctor != nil {
			anns[i] = introducedAt(ctor[i].Modifiers.Annotations)
		}
		found = found || anns[i] != nil
	}
	if !found {
		return
	}
	suppressed := func(name string) bool {
		return suppresses(c.file.Annotations, name) || suppresses(m.Annotations, name) ||
			site.cls != nil && suppresses(site.cls.Annotations, name)
	}

	cls := site.cls
	switch {
	case !site.ctor && (m.Abstract || m.Open || m.Override && !m.Final || cls != nil && cls.Interface && m.Visibility != syntax.Private):
		c.errorf(p, "functions with '@IntroducedAt' parameters must be final")
	case site.local:
		c.errorf(p, "local functions cannot have '@IntroducedAt' parameters")
	case !site.ctor && cls != nil && (cls.Open || cls.Abstract || cls.Sealed):
		c.errorf(p, "functions with '@IntroducedAt' parameters must be in final classes")
	case cls != nil && cls.Annotation:
		c.errorf(p, "annotation class constructors cannot have '@IntroducedAt' parameters")
	}

	versions := map[any]mavenVersion{}
	var highest *mavenVersion
	for i, q := range ps {
		a := anns[i]
		if a == nil {
			last := i == len(ps)-1 && q.Type != nil && q.Type.Ret != nil
			if q.Default == nil && highest != nil && !last {
				c.errorf(q.P, "a parameter without '@IntroducedAt' after one with it must be optional")
			}
			continue
		}
		s, _ := a.Str("version")
		v := parseMavenVersion(s)
		if q.Sym != nil {
			versions[q.Sym] = v
		}
		switch {
		case q.Vararg || ctor != nil && ctor[i].Modifiers.Vararg:
			c.errorf(a.P, "'@IntroducedAt' cannot be used on vararg parameters")
		case ctor != nil && ctor[i].IsProp && cls != nil && (cls.Value || cls.Inline):
			c.errorf(a.P, "'@IntroducedAt' cannot be used on value class properties")
		case q.Default == nil && !m.Actual:
			c.errorf(a.P, "'@IntroducedAt' can only be used on optional parameters")
		}
		if highest == nil || highest.compare(v) <= 0 {
			highest = &v
		} else if !suppressed("NON_ASCENDING_VERSION_ANNOTATION") {
			c.errorf(a.P, "version %s of '@IntroducedAt' is lower than the earlier %s", s, highest.s)
		}
	}

	for i, q := range ps {
		if q.Default == nil {
			continue
		}
		var own *mavenVersion
		if q.Sym != nil {
			if v, ok := versions[q.Sym]; ok {
				own = &v
			}
		} else if anns[i] != nil {
			continue
		}
		syntax.Inspect(q.Default, func(n any) bool {
			switch n := n.(type) {
			case *syntax.ObjectExpr:
				c.errorf(n.P, "default values of version-overloaded functions cannot have object expressions")
			case *syntax.Ident:
				if n.Sym == nil {
					break
				}
				if v, ok := versions[n.Sym]; ok && (own == nil || own.compare(v) < 0) {
					c.errorf(n.P, "a default value cannot use a parameter of a newer version")
				}
			}
			return true
		})
	}
}

// mavenVersion is a version as Maven's ComparableVersion orders them:
// numbers numerically, and qualifiers, as alpha and rc, before the
// release, which is 1, 1.0 and 1-final alike.
type mavenVersion struct {
	s     string
	items []mavenItem
}

type mavenItem struct {
	num  int
	qual string // "" for a number
	isQ  bool
}

func parseMavenVersion(s string) mavenVersion {
	v := mavenVersion{s: s}
	t := strings.ToLower(s)
	for len(t) > 0 {
		r := rune(t[0])
		switch {
		case r == '.' || r == '-' || r == '_':
			t = t[1:]
		case unicode.IsDigit(r):
			j := 0
			for j < len(t) && unicode.IsDigit(rune(t[j])) {
				j++
			}
			n, _ := strconv.Atoi(t[:j])
			v.items = append(v.items, mavenItem{num: n})
			t = t[j:]
		default:
			j := 0
			for j < len(t) && !unicode.IsDigit(rune(t[j])) && !strings.ContainsRune(".-_", rune(t[j])) {
				j++
			}
			v.items = append(v.items, mavenItem{qual: t[:j], isQ: true})
			t = t[j:]
		}
	}
	// 1.0.0 and 1-final are 1.
	for len(v.items) > 0 && v.items[len(v.items)-1].isNull() {
		v.items = v.items[:len(v.items)-1]
	}
	return v
}

func (it mavenItem) isNull() bool {
	if !it.isQ {
		return it.num == 0
	}
	return qualRank(it.qual) == qualRank("")
}

// qualRank orders Maven's qualifiers; unknown ones come after them all,
// alphabetically.
func qualRank(q string) string {
	switch q {
	case "a", "alpha":
		return "0"
	case "b", "beta":
		return "1"
	case "m", "milestone":
		return "2"
	case "rc", "cr":
		return "3"
	case "snapshot":
		return "4"
	case "", "ga", "final", "release":
		return "5"
	case "sp":
		return "6"
	}
	return "7-" + q
}

// compare returns -1, 0 or 1 as item it is before, the same as or after
// item o, nil for an item that isn't there.
func (it *mavenItem) compare(o *mavenItem) int {
	switch {
	case it == nil && o == nil:
		return 0
	case it == nil:
		return -o.compare(nil)
	case o == nil:
		if !it.isQ {
			if it.num == 0 {
				return 0
			}
			return 1
		}
		return strings.Compare(qualRank(it.qual), qualRank(""))
	case !it.isQ && !o.isQ:
		switch {
		case it.num < o.num:
			return -1
		case it.num > o.num:
			return 1
		}
		return 0
	case !it.isQ:
		return 1 // a number is after a qualifier
	case !o.isQ:
		return -1
	}
	return strings.Compare(qualRank(it.qual), qualRank(o.qual))
}

func (v mavenVersion) compare(o mavenVersion) int {
	for i := 0; i < len(v.items) || i < len(o.items); i++ {
		var a, b *mavenItem
		if i < len(v.items) {
			a = &v.items[i]
		}
		if i < len(o.items) {
			b = &o.items[i]
		}
		if r := a.compare(b); r != 0 {
			return r
		}
	}
	return 0
}
