package check

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// K2's partially constrained lambda analysis (PCLA): a builder's lambda
// is analyzed with the builder's variables unfixed, on a constraint
// system that the builder's call shares with the calls in the lambda.
// A call that depends on the variables is postponed: what of its own
// variables has no proper constraint yet joins the shared system, and is
// fixed with the builder's. A variable is fixed on demand, where the
// members of a value of its type are needed, only from the proper
// constraints it has there. The errors of that analysis are the
// lambda's: a member that isn't resolved because its receiver's variable
// can't be fixed yet stays unresolved, whatever the statements after it
// infer.
//
// kc keeps the lambda's two checks (see stubInference): the first, on
// the shared system, is K2's analysis, and the second, with the variables
// fixed, writes the types into the tree and reports what kotlinc's
// checkers find on them. The shared system isn't one that the calls add
// to in place: each run (stubRun) keeps its call's system, the systems of
// the calls that joined it and the constraints recorded on its variables,
// and buildShared puts those of the runs that are related together where
// a constraint is added, a variable fixed on demand or the run solved.
// The functions here are the first check's rules.

// pclaDebug, from $KC_PCLA_DEBUG, logs the first checks of builders'
// lambdas: their errors, the shared system where a variable is fixed on
// demand, and what each run is solved to.
var pclaDebug = os.Getenv("KC_PCLA_DEBUG") != ""

// pclaLogf logs for $KC_PCLA_DEBUG.
func pclaLogf(format string, args ...any) {
	if pclaDebug {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// pclaSingle, from $KC_PCLA, says how a builder's lambda is checked:
// "single" keeps the check with the variables unfixed where it can (see
// pclaCommit), and "recheck", the default, always checks the lambda again
// once they are fixed. bench/pcla-diff.sh compares the two.
var pclaSingle = os.Getenv("KC_PCLA") == "single"

// pclaCheck is the check of a builder's lambda with the builder's
// variables unfixed, kept until the variables are inferred: it is then
// committed, the types it gave the tree written with what they were
// inferred as, or undone, and the lambda checked again.
type pclaCheck struct {
	l *syntax.Lambda
	// t has what the check changed outside the lambda.
	t *trial
	// errs are the check's errors, which a commit reports.
	errs []*syntax.Error
	// w is what the lambda was checked against, and bt the type the
	// check gave its body.
	w  lambdaWant
	bt types.Type
}

// pclaRun returns the innermost run whose lambda is being checked, if
// any.
func (c *Checker) pclaRun() *stubRun {
	if n := len(c.pclaRuns); n > 0 {
		return c.pclaRuns[n-1]
	}
	return nil
}

// pclaDeny records that the lambdas being checked for builder inference
// can't keep their checks, because of why: something the check decided
// from a type with a variable, which only a check with the variable
// fixed decides as kotlinc does.
func (c *Checker) pclaDeny(why string) {
	for _, r := range c.pclaRuns {
		if r.deny == "" {
			r.deny = why
		}
	}
}

// pclaPostpone records s, the system of a call or another expression
// that finished in a builder's lambda being checked: its fixups, which
// ran with the builder's variables unfixed, run again once they are
// inferred (see pclaCommit), as K2 completes a postponed call with the
// builder's.
func (c *Checker) pclaPostpone(s *system) {
	if r := c.pclaRun(); r != nil && len(s.fixups) > 0 {
		r.postponed = append(r.postponed, s)
	}
}

// pclaMsg is an error's message, as errorf takes it.
type pclaMsg struct {
	format string
	args   []any
}

// pclaLate records how error e, of a check that builder inference may
// keep, is worded, if its message names a type with a variable: the
// variable is inferred after the check, and kotlinc's message names
// what it is inferred as.
func (c *Checker) pclaLate(e *syntax.Error, format string, args []any) {
	if !slices.ContainsFunc(args, func(a any) bool {
		t, ok := a.(types.Type)
		return ok && t.HasVars()
	}) {
		return
	}
	if c.pclaMsgs == nil {
		c.pclaMsgs = map[*syntax.Error]pclaMsg{}
	}
	c.pclaMsgs[e] = pclaMsg{format, slices.Clone(args)}
}

// pclaReword words the errors in errs that pclaLate recorded again,
// with sub.
func (c *Checker) pclaReword(errs []*syntax.Error, sub types.Subst) {
	for _, e := range errs {
		m, ok := c.pclaMsgs[e]
		if !ok {
			continue
		}
		open := false
		for i, a := range m.args {
			if t, ok := a.(types.Type); ok && t.HasVars() {
				t = sub.Apply(t)
				m.args[i] = t
				open = open || t.HasVars()
			}
		}
		e.Msg = fmt.Sprintf(m.format, m.args...)
		if !open {
			delete(c.pclaMsgs, e)
		}
	}
}

// pclaStale reports whether a fixup of a resolution of call x to f,
// run again by pclaCommit, is of a resolution that the check dropped:
// the call was resolved again, to another function, or isn't in the
// lambda's tree, as the call of an operator that another replaced.
func (c *Checker) pclaStale(x *syntax.Call, f *types.FunSym) bool {
	return c.pclaLive != nil && (!c.pclaLive[x] || x.Sym != f)
}

// pclaRollback undoes the checks that r keeps, last first: their lambdas
// are checked again.
func (c *Checker) pclaRollback(r *stubRun, why string) {
	for i := len(r.checks) - 1; i >= 0; i-- {
		k := r.checks[i]
		c.pclaStat("rollback", k.l.P, why)
		c.endTrial(k.t)
	}
	if len(r.checks) > 0 {
		// The lambda is checked again in the lambdas around it, which
		// their checks don't follow yet.
		c.pclaDeny("nested:" + why)
	}
	r.checks = nil
}

// pclaCommit keeps the check of lambda l that run r kept, if it can: l,
// an argument of the call with system s, whose variables in l's
// parameter types are fixed, isn't checked again. w is what a check now
// would check l against, params its parameter types and ret its result
// type. The types the check gave the tree are written with what the
// variables were inferred as (see pclaWrite), and the fixups of the
// systems that finished in l run again, with them. It returns the type
// of l's body, or false if l is to be checked again: the check is then
// undone.
func (c *Checker) pclaCommit(s *system, r *stubRun, l *syntax.Lambda, w lambdaWant, ret types.Type, params []types.Type) (types.Type, bool) {
	k := r.checks[0]
	deny := func(why string) (types.Type, bool) {
		c.pclaRollback(r, why)
		return types.InvalidType, false
	}
	_, pre := c.checked[l]
	switch {
	case len(r.checks) != 1 || k.l != l:
		return deny("group")
	case r.deny != "":
		return deny(r.deny)
	case pre:
		return deny("prechecked")
	case k.w.ret != w.ret || k.w.check != w.check:
		return deny("result")
	case ret.HasVars() && w.ret.K == types.Invalid && (endsWithCall(l.Body) || returnsCall(l) || endsWithBranches(l.Body)):
		return deny("tail")
	case len(k.w.params) != len(params):
		return deny("params")
	}
	if why := pclaScan(l); why != "" {
		return deny(why)
	}
	// What the variables were inferred as: the builder's in the call's
	// system, and those of the calls in the lambda in the shared one.
	sub := types.Subst{}
	for _, v := range r.vars {
		if tv := s.byVar[v]; tv != nil && tv.fixed {
			if a := s.vals[v]; !a.T.HasVars() && a.T.K != types.Invalid {
				sub[v] = a
			}
		}
	}
	for _, v := range r.extra {
		if tv := r.solved.byVar[v]; tv != nil && tv.fixed {
			if a := r.solved.vals[v]; !a.T.HasVars() && a.T.K != types.Invalid {
				sub[v] = a
			}
		}
	}
	for i, p := range k.w.params {
		if sub.Apply(p) != params[i] {
			return deny("params")
		}
	}
	saved := c.trial
	c.trial = k.t
	n := len(c.errs)
	// The types first: a fixup reads those of the nodes it checks.
	live := pclaWrite(l, sub)
	savedSub, savedLive := c.pclaSub, c.pclaLive
	c.pclaSub, c.pclaLive = sub, live
	for _, q := range r.postponed {
		q.pclaComplete(sub)
	}
	c.pclaSub, c.pclaLive = savedSub, savedLive
	pclaWrite(l, sub)
	for _, f := range r.defers {
		f()
	}
	c.trial = saved
	ours := func(u types.Type) bool {
		return u.K == types.TypeVar && (slices.Contains(r.vars, u.Param()) || slices.Contains(r.extra, u.Param()))
	}
	for _, t := range pclaLeft(l) {
		if t.Has(ours) {
			// A variable that nothing inferred: the check with the
			// builder's fixed reports it, or infers it on its own.
			c.errs = c.errs[:n]
			return deny("open")
		}
	}
	// An error that the fixups find again is the check's own.
	late := c.errs[n:]
	c.errs = c.errs[:n:n]
	for _, e := range late {
		if !slices.ContainsFunc(k.errs, func(o *syntax.Error) bool { return o.Pos == e.Pos }) &&
			!slices.ContainsFunc(c.errs[n:], func(o *syntax.Error) bool { return o.Pos == e.Pos && o.Msg == e.Msg }) {
			c.errs = append(c.errs, e)
		}
	}
	if len(c.errs) > n {
		c.pclaStat("commit-errors", l.P, c.errs[n].Msg)
	}
	c.pclaReword(k.errs, sub)
	c.pclaStat("commit", l.P)
	if o := k.t.outer; o != nil {
		o.undo = append(o.undo, k.t.undo...)
		maps.Copy(o.classes, k.t.classes)
	}
	c.errs = append(c.errs, k.errs...)
	if o := c.pclaRun(); o != nil {
		// What still has a variable of the builder around is its to
		// complete.
		o.postponed = append(o.postponed, r.postponed...)
		o.defers = append(o.defers, r.defers...)
	}
	r.checks = nil
	return sub.Apply(k.bt), true
}

// pclaComplete runs the fixups of s, a system that finished in a
// builder's lambda while the builder's variables were unfixed, again,
// with sub, what they and the variables of s that joined the builder's
// run were inferred as.
func (s *system) pclaComplete(sub types.Subst) {
	if s.vals == nil {
		s.vals = types.Subst{}
	}
	for v, a := range sub {
		if tv := s.byVar[v]; tv != nil {
			if tv.fixed {
				continue
			}
			tv.fixed = true
		}
		s.vals[v] = a
	}
	n := len(s.errs)
	for i := 0; i < len(s.fixups); i++ {
		s.fixups[i](s)
	}
	for ; n < len(s.errs); n++ {
		s.c.errorf(s.errs[n].pos, "%s", s.errs[n].msg)
	}
}

// pclaScan returns why the check of builder lambda l with the builder's
// variables unfixed can't be kept, if it can't: the first node in it
// whose types mention a variable for which the checker decides
// something from them that the writer of the inferred types (see
// pclaWrite) doesn't decide again. The others only have types, which it
// writes.
func pclaScan(l *syntax.Lambda) string {
	why := ""
	literal := func(a syntax.Expr) bool {
		if u, ok := a.(*syntax.Unary); ok {
			a = u.X
		}
		_, ok := a.(*syntax.IntLit)
		return ok
	}
	syntax.Inspect(l.Body, func(n any) bool {
		if why != "" {
			return false
		}
		ok := false
		switch n := n.(type) {
		case *syntax.Ident:
			ok = !n.Cast
		case *syntax.This:
			ok = !n.Cast
		case *syntax.Member:
			ok = !n.Cast && !n.Safe
		case *syntax.Call:
			ok = len(n.Reified) == 0 && n.Builtin == 0 && !slices.ContainsFunc(n.Args, literal)
		case *syntax.Lambda:
			ok = !n.Anon && n.Ref == nil
		case *syntax.VarStmt:
			ok = n.Delegate == nil
		case *syntax.Block, *syntax.ExprStmt, *syntax.StringLit:
			ok = true
		case *syntax.LocalClass, *syntax.ObjectExpr, *syntax.LocalFun:
			// Their declarations are checked for the types they have.
			why = fmt.Sprintf("%T", n)
			return false
		}
		if !ok && pclaTouches(n) {
			why = fmt.Sprintf("%T", n)
			if x, isCall := n.(*syntax.Call); isCall {
				switch {
				case x.Builtin != 0:
					why += ":builtin"
				case len(x.Reified) > 0:
					why += ":reified"
				default:
					why += ":literal"
				}
			}
		}
		return why == ""
	})
	why = strings.TrimPrefix(why, "*syntax.")
	return why
}

// pclaStats, from $KC_PCLA_STATS, is a file that the checks of builders'
// lambdas are listed in, a tab-separated line each, for measuring what
// builder inference does over a corpus (bench/pcla-diff.sh): "lambda",
// the lambda's position, the depth of its run, what it has that the
// checks treat specially, and how its first check ended; and
// "second-only", an error that only the second check of a lambda
// reports, with its message.
var pclaStats = os.Getenv("KC_PCLA_STATS")

var pclaStatsMu sync.Mutex

// pclaStat adds a line to $KC_PCLA_STATS, for position p of the file
// being checked.
func (c *Checker) pclaStat(kind string, p syntax.Pos, fields ...string) {
	if pclaStats == "" {
		return
	}
	pclaStatsMu.Lock()
	defer pclaStatsMu.Unlock()
	f, err := os.OpenFile(pclaStats, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\t%s:%d:%d\t%s\n", kind, c.file.Path, p.Line, p.Col, strings.Join(fields, "\t"))
}

// pclaFeatures lists what lambda l of run r, one of n that the run
// checks, has that builder inference treats specially, for $KC_PCLA_STATS.
func (c *Checker) pclaFeatures(l *syntax.Lambda, r *stubRun, n int) string {
	var fs []string
	add := func(on bool, name string) {
		if on && !slices.Contains(fs, name) {
			fs = append(fs, name)
		}
	}
	add(r.depth > 0, "nested")
	add(n > 1, "group")
	add(c.trial != nil, "in-trial")
	add(l.Anon, "anon-fun")
	add(l.Ref != nil, "ref")
	_, pre := c.checked[l]
	add(pre, "prechecked")
	if l.Body != nil {
		add(endsWithCall(l.Body), "tail-call")
		add(endsWithBranches(l.Body), "tail-branches")
		add(returnsCall(l), "return-call")
		syntax.Inspect(l.Body, func(n any) bool {
			switch n := n.(type) {
			case *syntax.LocalClass:
				add(true, "local-class")
			case *syntax.ObjectExpr:
				add(true, "object-expr")
			case *syntax.LocalFun:
				add(true, "local-fun")
			case *syntax.Lambda:
				add(true, "lambda")
			case *syntax.When:
				add(true, "when")
			case *syntax.For:
				add(true, "for")
			case *syntax.Jump:
				add(n.Kind == syntax.RETURN, "return")
			case *syntax.VarStmt:
				add(n.Delegate != nil, "delegate")
			}
			return true
		})
	}
	add(len(r.joined) > 0, "joined")
	add(len(r.forks) > 0, "forks")
	add(r.demanded, "demanded")
	return strings.Join(fs, ",")
}

// pclaDump logs the variables of s, for $KC_PCLA_DEBUG.
func (s *system) pclaDump() {
	if !s.ok {
		pclaLogf("  contradictory\n")
	}
	for _, e := range s.errs {
		pclaLogf("  error: %s\n", e.msg)
	}
	for _, tv := range s.vars {
		if tv.fixed {
			pclaLogf("  %s := %v\n", tv.v.Name, s.apply(types.VarType(tv.v)))
		} else {
			pclaLogf("  %v <: %s <: %v\n", tv.lower, tv.v.Name, tv.upper)
		}
	}
}

// dependentRun returns the innermost builder's run whose variables the
// receiver, the arguments or the expected type of a call of candidate cd
// mention, if any: as K2's postponed call, it isn't completed on its own.
func (c *Checker) dependentRun(cd *candidate, ts []types.Type, want types.Type) *stubRun {
	if len(c.stubs) == 0 {
		return nil
	}
	var r *stubRun
	see := func(t types.Type) {
		t.Has(func(u types.Type) bool {
			if u.K == types.TypeVar {
				if q := c.stubs[u.Param()]; q != nil && (r == nil || q.depth > r.depth) {
					r = q
				}
			}
			return false
		})
	}
	for _, a := range cd.recv {
		see(a.T)
	}
	see(cd.ext)
	for _, t := range ts {
		see(t)
	}
	see(want)
	return r
}

// completePostponed fixes the variables of s that have a proper
// constraint, as K2 completes a postponed call (PCLA_POSTPONED_CALL): the
// others are left to the builder's call, without an error.
func (s *system) completePostponed() {
	for {
		var pick *tvar
		best := -1
		for _, tv := range s.vars {
			if tv.fixed {
				continue
			}
			if r := s.readiness(tv); r&rdProper != 0 && r > best {
				pick, best = tv, r
			}
		}
		if pick == nil {
			return
		}
		s.fix(pick)
	}
}

// buildShared returns the system that r shares with the calls in its
// lambda (see build) and with the builders around and in it whose
// variables its bounds relate to: K2's one system for them all, nothing of
// it fixed, and the constraints of r that say something in it. A bound of
// an outer builder's variable is one of r's that is below it too.
func (r *stubRun) buildShared(saved map[*types.TypeParamSym]*stubRun) (*system, [][2]types.Type) {
	var all []*stubRun
	for _, q := range saved {
		if !slices.Contains(all, q) {
			all = append(all, q)
		}
	}
	slices.SortFunc(all, func(a, b *stubRun) int { return a.depth - b.depth })
	// The runs related to r: one whose variable a bound or a constraint
	// of another has.
	systems := func(q *stubRun) []*system {
		ss := []*system{q.s}
		for _, j := range q.joined {
			if j.ok && len(j.vars) > 0 {
				ss = append(ss, j)
			}
		}
		return ss
	}
	seen := []*stubRun{r}
	related := func(q *stubRun) bool {
		theirs := func(t types.Type) bool {
			return t.Has(func(u types.Type) bool {
				if u.K != types.TypeVar {
					return false
				}
				o := saved[u.Param()]
				return o != nil && o != q && slices.Contains(seen, o)
			})
		}
		ours := func(t types.Type) bool {
			return t.Has(func(u types.Type) bool { return u.K == types.TypeVar && saved[u.Param()] == q })
		}
		for _, j := range systems(q) {
			for _, tv := range j.vars {
				if !tv.fixed && (slices.ContainsFunc(tv.lower, theirs) || slices.ContainsFunc(tv.upper, theirs)) {
					return true
				}
			}
		}
		if slices.ContainsFunc(q.cons, func(k [2]types.Type) bool { return theirs(k[0]) || theirs(k[1]) }) {
			return true
		}
		for _, o := range seen {
			for _, j := range systems(o) {
				for _, tv := range j.vars {
					if !tv.fixed && (slices.ContainsFunc(tv.lower, ours) || slices.ContainsFunc(tv.upper, ours)) {
						return true
					}
				}
			}
			if slices.ContainsFunc(o.cons, func(k [2]types.Type) bool { return ours(k[0]) || ours(k[1]) }) {
				return true
			}
		}
		return false
	}
	for again := true; again; {
		again = false
		for _, q := range all {
			if !slices.Contains(seen, q) && related(q) {
				seen, again = append(seen, q), true
			}
		}
	}
	if len(seen) == 1 {
		return r.build(saved)
	}
	slices.SortFunc(seen, func(a, b *stubRun) int { return a.depth - b.depth })
	trial := r.s.clone()
	for _, q := range seen {
		for _, j := range systems(q) {
			if j == r.s || trial.byVar[j.vars[0].v] != nil {
				continue
			}
			j = j.clone()
			j.fixups, j.errs, j.lambdas = nil, nil, nil
			trial.merge(j)
		}
	}
	// A bound that is another variable of the system bounds that one
	// too, as sub relates two variables.
	for _, tv := range slices.Clone(trial.vars) {
		if tv.fixed {
			continue
		}
		t := types.VarType(tv.v)
		for _, l := range slices.Clone(tv.lower) {
			if w := trial.unfixedVar(l); w != nil && w != tv && !l.N {
				trial.addUpper(w, t)
			}
		}
		for _, u := range slices.Clone(tv.upper) {
			if w := trial.unfixedVar(u); w != nil && w != tv {
				trial.addLower(w, t)
			}
		}
	}
	var useful [][2]types.Type
	for _, q := range seen {
		ks := q.useful(trial)
		if q == r {
			useful = ks
		}
		for _, k := range ks {
			trial.sub(k[0], k[1])
		}
	}
	return trial, useful
}

// semiFix returns the type that the constraints of r so far give its
// variable v, as K2 fixes a variable on demand
// (semiFixCurrentResultIfTypeVariableAndReturnBinding): only one with a
// proper constraint of its own, from those constraints alone. A variable
// that only other variables bound isn't fixed, though fixing them would
// give it a type. A constraint with other variables in its type
// arguments is a proper one there, and they stay in the result: OTv of
// constrain(ScopeOwner(Value)) is a ScopeOwner<SOT>.
func (r *stubRun) semiFix(v *types.TypeParamSym) (types.Type, bool) {
	c := r.s.c
	saved := c.stubs
	c.stubs = nil
	defer func() { c.stubs = saved }()
	trial, _ := r.buildShared(saved)
	tv := trial.byVar[v]
	if pclaDebug {
		pclaLogf("PCLA %s fixed on demand in:\n", v.Name)
		trial.pclaDump()
	}
	if tv == nil || !trial.ok || tv.fixed {
		return types.InvalidType, false
	}
	return r.semiFixIn(trial, saved, tv, map[*tvar]bool{})
}

// semiFixIn is semiFix for variable tv of trial, the shared system: busy
// are the variables whose types are being found.
func (r *stubRun) semiFixIn(trial *system, saved map[*types.TypeParamSym]*stubRun, tv *tvar, busy map[*tvar]bool) (types.Type, bool) {
	if busy[tv] {
		return types.InvalidType, false
	}
	busy[tv] = true
	defer delete(busy, tv)
	// The other variables are types of their own here: placeholders,
	// which the result has the variables for again.
	hide, show := types.Subst{}, types.Subst{}
	back := map[*types.TypeParamSym]*tvar{}
	for _, o := range trial.vars {
		if o != tv && !o.fixed {
			p := &types.TypeParamSym{Name: o.v.Name}
			hide[o.v] = types.Arg{T: types.ParamType(p)}
			show[p] = types.Arg{T: types.VarType(o.v)}
			back[p] = o
		}
	}
	m := newSystem(r.s.c)
	mv := &tvar{v: tv.v, pos: tv.pos, flexNull: tv.flexNull}
	m.vars, m.byVar = []*tvar{mv}, map[*types.TypeParamSym]*tvar{tv.v: mv}
	// What bounds a variable that bounds v bounds v too, as K2's
	// incorporation has it: the literal of F, for A(1)'s A<T, F> that is
	// an A<X, Y>, is Y's.
	// like returns t as of a variable's type v says of the variable:
	// nullable, of a platform type, or not null.
	like := func(v, t types.Type) types.Type {
		switch {
		case v.P:
			if !t.N {
				t = t.Platform()
			}
		case v.N:
			t = t.Nullable()
		case v.D:
			t = t.NonNull()
		}
		return t
	}
	plain := func(t types.Type) types.Type { return t }
	other := func(t types.Type) *tvar {
		if t.K != types.TypeVar {
			return nil
		}
		if w := trial.byVar[t.Param()]; w != nil && !w.fixed {
			return w
		}
		return nil
	}
	seen := map[*tvar]bool{}
	var lower, upper func(o *tvar, as func(types.Type) types.Type)
	lower = func(o *tvar, as func(types.Type) types.Type) {
		if seen[o] {
			return
		}
		seen[o] = true
		mv.lits = append(mv.lits, o.lits...)
		for _, l := range o.lower {
			l = trial.apply(l)
			if w := other(l); w != nil {
				lower(w, func(t types.Type) types.Type { return as(like(l, t)) })
			} else if l = as(hide.Apply(l)); l.K != types.TypeVar && !slices.Contains(mv.lower, l) {
				mv.lower = append(mv.lower, l)
			}
		}
	}
	lower(tv, plain)
	clear(seen)
	upper = func(o *tvar, as func(types.Type) types.Type) {
		if seen[o] {
			return
		}
		seen[o] = true
		for _, u := range o.upper {
			if slices.Contains(o.declUp, u) {
				continue
			}
			u = trial.apply(u)
			if w := other(u); w != nil {
				upper(w, func(t types.Type) types.Type { return as(like(u, t)) })
			} else if u = as(hide.Apply(u)); u.K != types.TypeVar && !slices.Contains(mv.upper, u) {
				mv.upper = append(mv.upper, u)
			}
		}
	}
	upper(tv, plain)
	if len(mv.lower) == 0 && len(mv.upper) == 0 && len(mv.lits) == 0 {
		return types.InvalidType, false
	}
	m.fix(mv)
	val := m.vals[tv.v].T
	if !m.ok || len(m.errs) > 0 || val.K == types.Invalid || val.HasVars() {
		return types.InvalidType, false
	}
	// A variable in the result that isn't a builder's, as K of Data(1)
	// for T's Data<K>, which nothing fixes on demand later, is fixed
	// here, the same way.
	ok := true
	val.Has(func(u types.Type) bool {
		if u.K != types.TypeParam {
			return false
		}
		if o := back[u.Param()]; o != nil && saved[o.v] == nil {
			if t, fixed := r.semiFixIn(trial, saved, o, busy); fixed {
				show[u.Param()] = types.Arg{T: t}
			} else {
				ok = false
			}
		}
		return false
	})
	if !ok {
		return types.InvalidType, false
	}
	return show.Apply(val), true
}

// unfixedReceiver reports whether t, the type of x, the receiver of
// member name at p, after fixedStub, is still a builder's variable, and
// the error if so: as K2's type variable, it has no members, and what
// takes it, a member of Any or an extension, is not called on it
// (TypeVariablesInExplicitReceivers).
func (c *Checker) unfixedReceiver(x syntax.Expr, t types.Type, name string, p syntax.Pos) bool {
	if c.stub(t.Flex()) == nil {
		return false
	}
	switch {
	case name == "hashCode" || name == "toString" || name == "equals" || c.extCallees(name, t) != nil:
		c.errorf(syntax.StartPos(x), "the type of a receiver hasn't been inferred yet. Specify type argument for generic parameter '%s' explicitly.", t.Flex().Param().Name)
	default:
		c.errorf(p, "unresolved reference: %s", name)
	}
	return true
}

// pclaKept are the errors of the first check of a builder's lambda, for
// the call with system s: another check of the call, as with an expected
// type after one without, has its own system, and its own errors.
type pclaKept struct {
	s    *system
	errs []*syntax.Error
}

// keepPCLAErrors records the errors of the first check of lambda l of
// the call with system s, for replayPCLAErrors: as in K2, whose analysis
// of the lambda with its variables unfixed is the only one, they are the
// lambda's, whatever the variables are inferred as afterwards.
func (c *Checker) keepPCLAErrors(s *system, l *syntax.Lambda, errs []*syntax.Error) {
	if len(errs) == 0 {
		delete(c.pclaErrs, l)
		return
	}
	if c.pclaErrs == nil {
		c.pclaErrs = map[*syntax.Lambda]pclaKept{}
	}
	c.pclaErrs[l] = pclaKept{s, errs}
}

// filePos is a position in a file.
type filePos struct {
	file string
	p    syntax.Pos
}

// replayPCLAErrors reports the errors kept of the first check of lambda l
// of the call with system s where its second check, whose errors start at
// n, has none: the member that the first couldn't resolve is one the
// fixed variables then give. An error that the second check finds too is
// reported as that one words it, with the types inferred. A warning where
// an error is replayed would be about an expression that K2 never
// resolved, so warnf reports none there.
func (c *Checker) replayPCLAErrors(s *system, l *syntax.Lambda, n int) {
	k, ok := c.pclaErrs[l]
	if !ok {
		return
	}
	delete(c.pclaErrs, l)
	if k.s != s {
		return
	}
	for _, e := range k.errs {
		if slices.ContainsFunc(c.errs[n:], func(o *syntax.Error) bool { return o.Pos == e.Pos }) {
			continue
		}
		c.errs = append(c.errs, e)
		if c.pclaReplayed == nil {
			c.pclaReplayed = map[filePos]bool{}
		}
		c.pclaReplayed[filePos{e.File, e.Pos}] = true
	}
}

// elvisHint gives the variables of params, the parameter types of a
// lambda of the call with system s, that nothing bounds the type that the
// right side of the elvis the call is on the left of gives its result:
// K2's elvis(x: K?, y: K) fixes K from the right side first, and the
// call's variables from it, before the lambda is analyzed, with its
// parameter types known. T of val x = foo { it.length } ?: "" for
// fun <T : Any> foo(f: (T) -> Unit): T? is String. Only where the elvis
// is completed on its own: as an argument, println(foo { it.length } ?:
// ""), K2 analyzes the lambda before K is fixed, with T its bound, Any.
func (c *Checker) elvisHint(s *system, params []types.Type) {
	if s.elvisWant.K == types.Invalid {
		return
	}
	ret, want := s.elvisRet, s.elvisWant
	s.elvisWant = types.InvalidType
	if len(c.stubVars(s, params, nil)) == 0 {
		return
	}
	trial := s.clone()
	if trial.sub(ret, want); trial.ok {
		s.sub(ret, want)
	}
}

// elvisRightType is elvisRightFirst, for elvisHint: a literal's type is
// known too.
func (c *Checker) elvisRightType(y syntax.Expr) (types.Type, bool) {
	switch y := y.(type) {
	case *syntax.StringLit:
		if !slices.ContainsFunc(y.Parts, func(p syntax.TmplPart) bool { return p.X != nil }) {
			return types.StringType, true
		}
		return types.InvalidType, false
	case *syntax.BoolLit:
		return types.BoolType, true
	case *syntax.CharLit:
		return types.CharType, true
	}
	return c.elvisRightFirst(y)
}
