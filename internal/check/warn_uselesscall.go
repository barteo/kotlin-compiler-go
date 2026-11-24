package check

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// uselessCallPkgs are the standard library packages whose orEmpty,
// isNullOrEmpty and isNullOrBlank, of a nullable receiver,
// UselessCallOnNotNullChecker knows.
var uselessCallPkgs = map[string]map[string]bool{
	"kotlin":             {"orEmpty": true, "isNullOrEmpty": true, "isNullOrBlank": true},
	"kotlin/text":        {"orEmpty": true, "isNullOrEmpty": true, "isNullOrBlank": true},
	"kotlin/collections": {"orEmpty": true},
	"kotlin/sequences":   {"orEmpty": true},
}

// uselessCallOnNotNull reports kotlinc's extra check
// USELESS_CALL_ON_NOT_NULL (UselessCallOnNotNullChecker), with -Wextra,
// on call x: s.orEmpty(), s.isNullOrEmpty() or s.isNullOrBlank() of the
// standard library on an explicit receiver s that isn't null, or a safe
// call's, s?.orEmpty(), whose receiver is s once it isn't null.
func (c *Checker) uselessCallOnNotNull(x *syntax.Call) {
	m, ok := x.Fun.(*syntax.Member)
	if !ok || x.Invoke || !m.Written || m.X == nil || x.Sym == nil || x.Sym.Owner != nil {
		return
	}
	f := x.Sym
	pkg := f.Facade[:max(strings.LastIndexByte(f.Facade, '/'), 0)]
	if !uselessCallPkgs[pkg][m.Name] || f.Name != m.Name {
		return
	}
	if _, ok := m.X.(*syntax.This); ok {
		return
	}
	t := m.X.Type()
	if m.Safe && !t.P {
		t.N = false
	}
	if definitelyNonNull(t) {
		c.warnf(m.P, "USELESS_CALL_ON_NOT_NULL", "useless call on a receiver of a non-null type")
	}
}
