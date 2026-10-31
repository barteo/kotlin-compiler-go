package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// checkClashes reports the program's functions that would have the same
// JVM signature in one class or file facade, as f(List<A>) and
// f(List<B>) without @JvmName.
func (c *Checker) checkClashes() {
	var top []*syntax.FunDecl
	for _, d := range c.file.Decls {
		if fd, ok := d.(*syntax.FunDecl); ok {
			top = append(top, fd)
		}
	}
	c.clashes(top, nil)
	for _, d := range c.classes {
		var fs []*syntax.FunDecl
		for _, m := range d.Members {
			if fd, ok := m.(*syntax.FunDecl); ok {
				fs = append(fs, fd)
			}
		}
		c.clashes(fs, c.getterSigs(d))
	}
}

// getterSigs returns the JVM names and descriptors of the getters of a
// class's properties, and reports those that a Java supertype's method
// has: a Kotlin property doesn't override a Java getter, as with kotlinc.
func (c *Checker) getterSigs(d *syntax.ClassDecl) map[string]bool {
	var props []*types.VarSym
	var pos []syntax.Pos
	var override []bool
	for _, cp := range d.CtorParams {
		if cp.PropSym != nil {
			props, pos, override = append(props, cp.PropSym), append(pos, cp.P), append(override, cp.Override)
		}
	}
	for _, m := range d.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && pd.Recv == nil && pd.Sym != nil {
			props, pos, override = append(props, pd.Sym), append(pos, pd.P), append(override, pd.Override)
		}
	}
	sigs := map[string]bool{}
	for i, p := range props {
		if p.Private || p.JvmField || p.Const {
			continue
		}
		ensureVar(p)
		desc, ok := types.Desc(types.GetterType(p))
		if !ok {
			continue
		}
		name := types.GetterJVM(p)
		sig := name + "()" + desc
		sigs[sig] = true
		if override[i] {
			continue // it overrides a Kotlin property, or nothing
		}
		d.Sym.Supertypes(func(s *types.ClassSym) bool {
			if s == d.Sym || !s.External || s.KotlinName != "" || builtinJava[s.Name] {
				return true
			}
			for _, g := range s.Methods[name] {
				if len(g.Params) == 0 && !g.Static && !g.Private && g.JVM()+types.FunDesc(g) == sig {
					c.errorf(pos[i], "accidental override: the following declarations have the same JVM signature (%s)", sig)
					return false
				}
			}
			return true
		})
	}
	return sigs
}

// clashes reports the functions fs whose JVM signatures are the same, or
// one of the signatures seen, a class's getters.
func (c *Checker) clashes(fs []*syntax.FunDecl, seen map[string]bool) {
	if len(fs)+len(seen) < 2 {
		return
	}
	if seen == nil {
		seen = map[string]bool{}
	}
	for _, d := range fs {
		sig, ok := jvmSig(d.Sym)
		if !ok {
			continue
		}
		if seen[sig] {
			c.errorf(d.P, "platform declaration clash: the following declarations have the same JVM signature (%s)", sig)
			continue
		}
		seen[sig] = true
	}
}

// jvmSig returns the JVM name and descriptor of one of the program's
// functions, if its types have descriptors.
func jvmSig(f *types.FunSym) (string, bool) {
	if f == nil {
		return "", false
	}
	params, ok := types.ParamsDesc(f.Params)
	if !ok {
		return "", false
	}
	ret, ok := types.Desc(f.Ret)
	if !ok {
		return "", false
	}
	if f.Ext {
		recv, ok := types.ParamDesc(f.Recv)
		if !ok {
			return "", false
		}
		params = "(" + recv + params[1:]
	}
	if f.Suspend {
		// With its continuation: @JvmName("next") suspend fun next0() is
		// next(Continuation), which next() doesn't clash with.
		params = params[:len(params)-1] + "Lkotlin/coroutines/Continuation;)"
		ret = "Ljava/lang/Object;"
	}
	return f.JVM() + params + ret, true
}
