package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// Qualifiers of K2's predefined enhancements of JDK members
// (predefinedEnhancementInfo.kt), which kotlinc applies whatever the
// JDK's annotations.
type jdkQual uint8

const (
	jdkKeep        jdkQual = iota // as read: a platform type
	jdkNullable                   // T?
	jdkNotPlatform                // T: its nullability is the substitution's
	jdkNotNullable                // T & Any
)

// jdkMember is a member's enhancement: the qualifiers of its return
// type and of each parameter's type, each in preorder, the type first
// and then its arguments.
type jdkMember struct {
	ret    []jdkQual
	params [][]jdkQual
}

var (
	qNP  = []jdkQual{jdkNotPlatform}
	qNPN = []jdkQual{jdkNotPlatform, jdkNotNullable}
)

// jdkEnhancements are the predefined enhancements kc applies, by class,
// name and parameter count. Of K2's table, these are the members whose
// types Kotlin doesn't already map: Optional's.
var jdkEnhancements = map[string]map[string]map[int]jdkMember{
	"java/util/Optional": {
		"empty":      {0: {ret: qNPN}},
		"of":         {1: {ret: qNPN, params: [][]jdkQual{{jdkNotNullable}}}},
		"ofNullable": {1: {ret: qNPN, params: [][]jdkQual{{jdkNullable}}}},
		"get":        {0: {ret: []jdkQual{jdkNotNullable}}},
		"ifPresent":  {1: {params: [][]jdkQual{qNPN}}},
	},
}

// enhanceJDK applies the predefined enhancement of f, a member of the
// class named cls, if it has one.
func enhanceJDK(cls string, f *types.FunSym) {
	e, ok := jdkEnhancements[cls][f.Name][len(f.Params)]
	if !ok {
		return
	}
	if e.ret != nil {
		f.Ret, _ = applyJDKQuals(f.Ret, e.ret)
	}
	for i, q := range e.params {
		if q != nil {
			p := *f.Params[i]
			p.Type, _ = applyJDKQuals(p.Type, q)
			f.Params[i] = &p
		}
	}
}

// applyJDKQuals applies qs, in preorder, to t and its arguments, and
// returns the qualifiers left over.
func applyJDKQuals(t types.Type, qs []jdkQual) (types.Type, []jdkQual) {
	if len(qs) == 0 {
		return t, qs
	}
	switch q := qs[0]; {
	case q == jdkNullable:
		t = t.Nullable()
	case q == jdkNotPlatform && t.P:
		t.N, t.P = false, false
	case q == jdkNotNullable:
		t = t.NonNull()
		if t.IsParam() {
			t.D = true
		}
	}
	qs = qs[1:]
	if t.K != types.Class || t.A == nil || len(qs) == 0 {
		return t, qs
	}
	args := append([]types.Arg(nil), t.A.List...)
	for i := range args {
		if args[i].V == types.Star {
			qs = qs[1:]
			if len(qs) == 0 {
				break
			}
			continue
		}
		args[i].T, qs = applyJDKQuals(args[i].T, qs)
		if len(qs) == 0 {
			break
		}
	}
	t.A = types.MakeArgs(args)
	return t, qs
}
