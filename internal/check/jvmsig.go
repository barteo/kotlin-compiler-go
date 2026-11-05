package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// checkClashes reports the program's declarations that would have the
// same JVM signature in one class or file facade, as f(List<A>) and
// f(List<B>) without @JvmName, or val x and fun getX(), and those that
// accidentally override an inherited method on the JVM.
func (c *Checker) checkClashes() {
	var top []jvmMember
	for _, d := range c.file.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			if d.Sym != nil && !d.Expect {
				top = c.funMembers(top, d, nil)
			}
		case *syntax.PropDecl:
			if d.Sym == nil || d.Expect {
				continue
			}
			if d.Recv != nil {
				if d.Getter != nil && d.Getter.Sym != nil {
					top = extPropMembers(top, d)
				}
				continue
			}
			gp, sp := d.P, d.P
			if d.Getter != nil {
				gp = d.Getter.P
			}
			if d.Setter != nil {
				sp = d.Setter.P
			}
			top = c.propMembers(top, d.Sym, d.Visibility == syntax.Private, d.P, gp, sp)
		}
	}
	c.sameContainer(top)
	for _, m := range top {
		if !m.private && objectFinals[m.sig] {
			c.errorf(m.pos, "accidental override: the following declarations have the same JVM signature (%s)", m.sig)
		}
	}
	for _, d := range c.classes {
		c.javaGetterClashes(d)
		c.classClashes(d)
	}
}

// javaGetterClashes reports the getters of a class's properties that a
// Java supertype's method has: a Kotlin property doesn't override a Java
// getter, as with kotlinc.
func (c *Checker) javaGetterClashes(d *syntax.ClassDecl) {
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
}
