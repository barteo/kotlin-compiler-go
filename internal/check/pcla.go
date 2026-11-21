package check

import (
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// pclaSingle selects K2's single pass for builder lambdas, the default: a
// lambda whose check with the builder's variables unfixed (see
// stubInference) is clean keeps what that check gave the tree, and the
// variables' values are written into it (see writePCLA), instead of the
// lambda being checked again once they are fixed. A lambda whose check
// has errors, or leaves one to the check again (see skipTrial), is still
// checked again, which reports them where the variables' values say.
// KC_PCLA=twopass checks every one again, as kc did before.
var pclaSingle = os.Getenv("KC_PCLA") != "twopass"

// pclaTrace names a file to which each builder run appends whether it
// keeps its trials.
var pclaTrace = os.Getenv("KC_PCLA_TRACE")

// pclaCommit is what a builder lambda's kept check left for postponedLambda:
// the body's type, and the values that the run's solution gives its
// variables and those of the calls inferred with them.
type pclaCommit struct {
	bt      types.Type
	vars    []*types.TypeParamSym // the builder's variables, which the call's system fixes
	vals    types.Subst
	classes map[*types.ClassSym]bool // the classes its check declared
}

// pendingTrial is a group lambda's trial that stubCheck left open: the
// run decides once it is solved whether to keep or undo it.
type pendingTrial struct {
	l   *syntax.Lambda
	t   *trial
	bt  types.Type
	why string // why it can't be kept, if it can't: errors, skipped, shape
}

// keepTrial ends trial t keeping what its check changed: the trial around
// it, if any, undoes it if it is undone itself.
func (c *Checker) keepTrial(t *trial) {
	c.trial = t.outer
	if o := t.outer; o != nil {
		o.undo = append(o.undo, t.undo...)
		for cls := range t.classes {
			o.classes[cls] = true
		}
	}
}

// committable reports whether stubInference may keep the check of lambda
// l: not a reference, nor an anonymous function declaring its result
// type, which the check that counts resolves, and one whose result, for a result type of the call's other variables,
// is no call or branches that the call's system would infer with it
// (see checkTail).
func committable(l *syntax.Lambda, retVars bool) bool {
	if l.Ref != nil || l.Anon && l.AnonRet != nil {
		return false
	}
	return !retVars || !endsWithCall(l.Body) && !returnsCall(l) && !endsWithBranches(l.Body)
}

var (
	typeType   = reflect.TypeFor[types.Type]()
	varSymType = reflect.TypeFor[types.VarSym]()
	syntaxPkg  = reflect.TypeFor[syntax.Lambda]().PkgPath()
)

// writePCLA writes the values sub gives the variables of a builder's run
// into the tree of lambda l, which its kept check typed with them: the
// types of its nodes, and of the locals, parameters, functions and
// classes it declares, which an inferred type, = get(), may give them.
// The variables are the calls' own (see system.fresh), so the
// substitution changes no other declaration's types.
func (c *Checker) writePCLA(l *syntax.Lambda, sub types.Subst) {
	seen := map[any]bool{}
	fix := func(t types.Type) types.Type {
		if !t.HasVars() {
			return t
		}
		return sub.Apply(t)
	}
	fixVar := func(v *types.VarSym) {
		if v != nil && !seen[v] {
			seen[v] = true
			v.Type = fix(v.Type)
		}
	}
	fixFun := func(f *types.FunSym) {
		if f == nil || seen[f] {
			return
		}
		seen[f] = true
		for _, p := range f.Params {
			fixVar(p)
		}
		for _, p := range f.Contexts {
			fixVar(p)
		}
		f.Ret, f.Recv = fix(f.Ret), fix(f.Recv)
	}
	fixClass := func(cls *types.ClassSym) {
		if cls == nil || seen[cls] {
			return
		}
		seen[cls] = true
		for i, t := range cls.Supers {
			cls.Supers[i] = fix(t)
		}
		for _, p := range cls.CtorParams {
			fixVar(p)
		}
		for _, p := range cls.Props {
			fixVar(p)
		}
		fixFun(cls.Ctor)
		for _, f := range cls.Ctors {
			fixFun(f)
		}
		for _, fs := range cls.Methods {
			for _, f := range fs {
				fixFun(f)
			}
		}
		for _, fs := range cls.ExtMethods {
			for _, f := range fs {
				fixFun(f)
			}
		}
	}
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				return
			}
			e := v.Type().Elem()
			switch {
			case e == typeType:
				p := v.Interface().(*types.Type)
				*p = fix(*p)
			case e == varSymType:
				fixVar(v.Interface().(*types.VarSym))
			case e.PkgPath() == syntaxPkg:
				n := v.Interface()
				if seen[n] {
					return
				}
				seen[n] = true
				switch n := n.(type) {
				case *syntax.FunDecl:
					fixFun(n.Sym)
				case *syntax.ClassDecl:
					fixClass(n.Sym)
				case *syntax.Binary:
					if ts, ok := c.identTypes[n]; ok {
						for i := range ts {
							ts[i] = fix(ts[i])
						}
						c.identTypes[n] = ts
					}
				case *syntax.Assign:
					if t, ok := c.assignTypes[n]; ok {
						c.assignTypes[n] = fix(t)
					}
				}
				walk(v.Elem())
			}
		case reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			if v.Type() == typeType {
				if v.CanSet() {
					v.Set(reflect.ValueOf(fix(v.Interface().(types.Type))))
				}
				return
			}
			if v.Type().PkgPath() != syntaxPkg {
				return
			}
			st := v.Type()
			for i := range st.NumField() {
				if f := st.Field(i); f.IsExported() || f.Anonymous {
					walk(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := range v.Len() {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(l))
}

// settleTrials keeps the trials of run r's lambdas, if each is clean and
// committable, nothing in them was left for a check again, and the run's
// solution, ok, gives every variable of the run a type: postponedLambda
// then writes the values into their trees (see writePCLA). Otherwise it
// undoes them, last first, and the lambdas are checked again.
func (c *Checker) settleTrials(r *stubRun, group []stubLambda, ok bool, sol *system) {
	if len(r.pending) == 0 {
		return
	}
	pending := r.pending
	r.pending = nil
	keep := ok && len(pending) == len(group) && len(r.recvs) == 0 && !r.dropped
	for _, p := range pending {
		keep = keep && p.why == ""
	}
	vals := types.Subst{}
	if keep {
		live := func(u types.Type) bool { return u.K == types.TypeVar && c.stubs[u.Param()] == nil }
		for _, v := range append(slices.Clone(r.vars), r.extra...) {
			a, has := sol.vals[v]
			if !has || a.T.K == types.Invalid || a.T.Has(live) {
				keep = false
				break
			}
			vals[v] = a
		}
	}
	if pclaTrace != "" {
		why := ""
		switch {
		case !ok:
			why = "unsolved"
		case len(pending) != len(group):
			why = "group"
		case len(r.recvs) > 0:
			why = "recvs"
		case r.dropped:
			why = "dropped"
		}
		for _, p := range pending {
			if why == "" && p.why != "" {
				why = p.why
			}
		}
		if why == "" && !keep {
			why = "vals"
		}
		if f, err := os.OpenFile(pclaTrace, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "PCLA %s:%d keep=%v %s\n", c.file.Path, group[0].l.P.Line, keep, why)
			f.Close()
		}
	}
	if !keep {
		for i := len(pending) - 1; i >= 0; i-- {
			c.endTrial(pending[i].t)
		}
		return
	}
	if c.pclaCommits == nil {
		c.pclaCommits = map[*syntax.Lambda]*pclaCommit{}
	}
	for _, p := range pending {
		c.keepTrial(p.t)
		c.pclaCommits[p.l] = &pclaCommit{bt: p.bt, vars: r.vars, vals: vals, classes: p.t.classes}
	}
}

// writeCommit writes the values of cm's variables into lambda l's tree,
// those of the builder's from s, which fixed them, and returns the type
// of its body.
func (c *Checker) writeCommit(s *system, l *syntax.Lambda, cm *pclaCommit) types.Type {
	sub := maps.Clone(cm.vals)
	for _, v := range cm.vars {
		if t := s.final(types.VarType(v)); t.K != types.Invalid {
			sub[v] = types.Arg{T: t}
		}
	}
	c.writePCLA(l, sub)
	if len(cm.classes) > 0 {
		// Their members by name, with the variables, are found again.
		c.u.local.methods.Range(func(k, _ any) bool {
			if cm.classes[k.(methodsKey).cls] {
				c.u.local.methods.Delete(k)
			}
			return true
		})
	}
	if cm.bt.HasVars() {
		return sub.Apply(cm.bt)
	}
	return cm.bt
}

// skipTrial records that the trial running skipped a check, why, that
// the check again makes: it isn't kept (see settleTrials).
func (c *Checker) skipTrial(why string) {
	c.trialSkips++
	c.trialSkipWhy = why
}
