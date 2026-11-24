package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// This file reports the warnings of kotlinc's extra checker
// UnusedVariableAssignmentChecker (-Wextra) about local variables:
// UNUSED_VARIABLE for one that is never used, VARIABLE_NEVER_READ for one
// that is only assigned, and, for one that is read, ASSIGNED_VALUE_IS_NEVER_READ
// for an assignment and VARIABLE_INITIALIZER_IS_REDUNDANT for an
// initializer whose value no read can see. kotlinc decides the last two
// by the code's control flow graph. kc reports them only where the code's
// structure makes it certain: the value is overwritten, or the variable
// goes out of scope or the function returns, before any statement that
// reads the variable, going through if, when and try statements, and
// loops without break or continue, whose body runs again from its start.
// In a try, a catch may be entered after any statement, and the finally
// block from anywhere. For a variable that a lambda, a local function or
// a local class uses, kotlinc judges the writes before the first of
// them, which reads the variable if it uses it other than by assigning
// it; kc judges these writes too, but not in loops, where the capture may
// come before them. One that only assigns the variable can't read a
// value, so the writes outside it are judged as if it weren't there, and
// a write in a lambda, which may run any time after it is created, only
// where nothing reads the variable after the lambda begins.

// localVar is what unusedVars knows of a local variable.
type localVar struct {
	decl     any              // *syntax.VarStmt, or *syntax.When for a subject variable
	block    *syntax.Block    // the block that declares it, nil for a when's subject
	writes   []*syntax.Assign // its assignments, x = v
	updates  []*syntax.Assign // its compound assignments, x += v
	read     bool
	incDec   bool       // x++ or x-- assigns it
	captured bool       // used in a lambda, a local function or a local class
	capture  syntax.Pos // where the first of these begins
	// captureReads says that one of these reads it, rather than only
	// assigning it.
	captureReads bool
	annot        bool         // named in an annotation's argument
	readPos      []syntax.Pos // where it is read
}

// stmtLoc is a statement's place: its block and its index there.
type stmtLoc struct {
	block *syntax.Block
	index int
}

// unusedVars is the state of unusedVarWarnings.
type unusedVars struct {
	vars        map[*types.VarSym]*localVar
	order       []*types.VarSym
	blockParent map[*syntax.Block]any           // the node a block belongs to
	stmtLoc     map[syntax.Stmt]stmtLoc         // a block's statements
	exprStmt    map[syntax.Expr]stmtLoc         // an if or when that is a statement
	writeLoc    map[syntax.Expr]stmtLoc         // the innermost statement of x++ and x--
	lambdaLoc   map[*syntax.Lambda]stmtLoc      // the innermost statement of a lambda
	tries       map[*syntax.Block][]*syntax.Try // the try constructs a block is in
	postfix     []*syntax.Unary
}

// unusedVarWarnings reports the file's unused local variables and
// assignments.
func (c *Checker) unusedVarWarnings() {
	u := &unusedVars{vars: map[*types.VarSym]*localVar{}, blockParent: map[*syntax.Block]any{},
		stmtLoc: map[syntax.Stmt]stmtLoc{}, exprStmt: map[syntax.Expr]stmtLoc{}, writeLoc: map[syntax.Expr]stmtLoc{},
		lambdaLoc: map[*syntax.Lambda]stmtLoc{},
		tries:     map[*syntax.Block][]*syntax.Try{}}
	declare := func(sym *types.VarSym, name string, decl any, b *syntax.Block) {
		if sym == nil || name == "_" || strings.HasPrefix(name, "<") || strings.HasPrefix(name, "$") || u.vars[sym] != nil {
			return
		}
		u.vars[sym] = &localVar{decl: decl, block: b}
		u.order = append(u.order, sym)
	}
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Block:
			for i, st := range n.Stmts {
				u.stmtLoc[st] = stmtLoc{n, i}
				switch st := st.(type) {
				case *syntax.VarStmt:
					// Not for (x: T in xs)'s x, which the checker declares
					// as val x: T = <element>: kotlinc doesn't report a
					// loop's variable.
					if id, ok := st.Init.(*syntax.Ident); !ok || !strings.HasPrefix(id.Name, "<element") {
						declare(st.Sym, st.Name, st, n)
					}
				case *syntax.Destructure:
					for _, vs := range st.Stmts {
						declare(vs.Sym, vs.Name, vs, n)
					}
				case *syntax.ExprStmt:
					switch x := st.X.(type) {
					case *syntax.If, *syntax.When, *syntax.Try:
						u.exprStmt[x] = stmtLoc{n, i}
					}
				}
				syntax.Inspect(st, func(m any) bool {
					switch x := m.(type) {
					case *syntax.Unary:
						if x.Op == syntax.INC || x.Op == syntax.DEC {
							u.writeLoc[x] = stmtLoc{n, i}
						}
					case *syntax.Lambda:
						u.lambdaLoc[x] = stmtLoc{n, i}
					}
					return true
				})
			}
		case *syntax.If:
			u.blockParent[n.Then] = n
			if n.Else != nil {
				u.blockParent[n.Else] = n
			}
		case *syntax.When:
			for _, br := range n.Branches {
				u.blockParent[br.Body] = n
				if br.Guard != nil {
					syntax.Inspect(br.Guard, visit)
				}
			}
			if n.SubjVar != "" {
				declare(n.SubjSym, n.SubjVar, n, nil)
				// A branch's condition compares the subject with its
				// value, which reads it.
				if v := u.vars[n.SubjSym]; v != nil && slices.ContainsFunc(n.Branches, func(br *syntax.WhenBranch) bool { return len(br.Conds) > 0 }) {
					v.read = true
				}
			}
		case *syntax.While:
			u.blockParent[n.Body] = n
			if n.Do {
				// The condition sees the body's variables, declared as
				// the walk enters it.
				syntax.Inspect(n.Body, visit)
				syntax.Inspect(n.Cond, visit)
				return false
			}
		case *syntax.For:
			u.blockParent[n.Body] = n
		case *syntax.Try:
			u.blockParent[n.Body] = n
			blocks := []*syntax.Block{n.Body, n.Finally}
			for _, ct := range n.Catches {
				u.blockParent[ct.Body] = n
				blocks = append(blocks, ct.Body)
			}
			if n.Finally != nil {
				u.blockParent[n.Finally] = n
			}
			for _, b := range blocks {
				syntax.Inspect(b, func(m any) bool {
					if b, ok := m.(*syntax.Block); ok && b != nil {
						u.tries[b] = append(u.tries[b], n)
					}
					return true
				})
			}
		case *syntax.Lambda:
			if n.Ref == nil {
				u.blockParent[n.Body] = n
				u.capture(&syntax.FunDecl{Body: n.Body}, syntax.StartPos(n))
			}
		case *syntax.LocalFun:
			u.capture(n.Decl, n.Decl.P)
		case *syntax.ObjectExpr:
			u.capture(n.Decl, syntax.StartPos(n))
		case *syntax.LocalClass:
			u.capture(n.Decl, n.Decl.P)
		case *syntax.Assign:
			id, ok := n.Target.(*syntax.Ident)
			if v := u.local(id, ok); v != nil {
				if n.Op == syntax.ASSIGN {
					v.writes = append(v.writes, n)
				} else {
					v.read = true
					v.readPos = append(v.readPos, id.Pos())
					if !plusAssign(n) {
						v.updates = append(v.updates, n)
					}
				}
			} else {
				syntax.Inspect(n.Target, visit)
			}
			syntax.Inspect(n.Value, visit)
			return false // not its rewritten Stmts
		case *syntax.Unary:
			if n.Op == syntax.INC || n.Op == syntax.DEC {
				id, ok := n.X.(*syntax.Ident)
				if v := u.local(id, ok); v != nil {
					v.read, v.incDec = true, true
					v.readPos = append(v.readPos, id.Pos())
					if !n.Prefix {
						u.postfix = append(u.postfix, n)
					}
					return false
				}
			}
		case *syntax.Ident:
			if v := u.local(n, true); v != nil {
				v.read = true
				v.readPos = append(v.readPos, n.Pos())
			}
		}
		return true
	}
	walkCode(c.file.Decls, visit)
	// kotlinc counts uses in annotations' arguments, which can only name
	// constants, as reads. kc doesn't know where they are, so it doesn't
	// judge the writes of a variable of the name either.
	annotNames := map[string]bool{}
	for _, as := range c.file.Annotated {
		for _, a := range as {
			for _, arg := range a.Args {
				for _, t := range arg.Toks {
					if t.Kind == syntax.IDENT {
						annotNames[t.Text] = true
					}
				}
			}
		}
	}

	for _, sym := range u.order {
		v := u.vars[sym]
		if annotNames[sym.Name] {
			v.read, v.captured, v.annot = true, true, true
		}
		vs, _ := v.decl.(*syntax.VarStmt)
		delegated := vs != nil && vs.Delegate != nil
		if vs != nil && vs.Mutable && vs.Init != nil && !vs.Lateinit && !delegated && !v.annot && len(v.writes) == 0 && len(v.updates) == 0 && !v.incDec {
			// A var initialized and never assigned again: x += v that
			// calls plusAssign doesn't assign it.
			c.warnf(vs.P, "CAN_BE_VAL", "this 'var' property is never written to, it can be declared as 'val'")
		}
		if vs != nil && vs.Mutable && vs.Init == nil && !delegated && !v.annot && len(v.updates) == 0 && !v.incDec {
			// One assigned later, at most once on every path.
			if loc, ok := u.stmtLoc[vs]; ok && atMostOnce(sym, loc.block, loc.index) {
				if vs.Lateinit {
					c.warnf(vs.P, "CAN_BE_VAL_LATEINIT", "this 'lateinit var' property is not written to more than once, it can be declared as nullable 'val'")
				} else {
					c.warnf(vs.P, "CAN_BE_VAL_DELAYED_INITIALIZATION", "this 'var' property is not written to more than once, it can be declared as 'val'")
				}
			}
		}
		if !v.read {
			switch {
			case len(v.writes) == 0:
				c.warnf(declPos(v.decl), "UNUSED_VARIABLE", "variable '%s' is never used", sym.Name)
			case !delegated:
				c.warnf(declPos(v.decl), "VARIABLE_NEVER_READ", "variable '%s' is assigned but never accessed", sym.Name)
				for _, w := range v.writes {
					c.warnf(w.Target.Pos(), "ASSIGNED_VALUE_IS_NEVER_READ", "assigned value is never read")
				}
			}
			continue
		}
		if delegated || v.annot || v.block == nil {
			continue
		}
		if vs != nil && vs.Init != nil {
			if loc, ok := u.stmtLoc[vs]; ok && u.unreadAfter(sym, v, loc) {
				c.warnf(syntax.StartPos(vs.Init), "VARIABLE_INITIALIZER_IS_REDUNDANT", "initializer is redundant")
			}
		}
		for _, w := range append(v.writes[:len(v.writes):len(v.writes)], v.updates...) {
			if loc, ok := u.stmtLoc[w]; ok && (u.beforeCapture(v, w.Target.Pos(), loc) && u.unreadAfter(sym, v, loc) || u.unreadFromLambda(v, loc)) {
				c.warnf(w.Target.Pos(), "ASSIGNED_VALUE_IS_NEVER_READ", "assigned value is never read")
			}
		}
	}
	for _, x := range u.postfix {
		id := x.X.(*syntax.Ident)
		v := u.vars[id.Sym]
		if v.annot || v.block == nil {
			continue
		}
		if vs, _ := v.decl.(*syntax.VarStmt); vs != nil && vs.Delegate != nil {
			continue
		}
		loc, ok := u.writeLoc[x]
		if !ok || !u.beforeCapture(v, id.Pos(), loc) || varUses(loc.block.Stmts[loc.index], id.Sym) != 1 {
			continue // the statement may read the new value
		}
		if w, ok := loc.block.Stmts[loc.index].(*syntax.While); ok && readsVar(w.Cond, id.Sym) {
			continue // the condition reads it again: while (i++ < 3)
		}
		if u.unreadAfter(id.Sym, v, loc) {
			c.warnf(id.Pos(), "ASSIGNED_VALUE_IS_NEVER_READ", "assigned value is never read")
		}
	}
}

// plusAssign reports whether compound assignment x calls an operator
// such as plusAssign, which assigns nothing, rather than x = x.plus(v).
func plusAssign(x *syntax.Assign) bool {
	return len(x.Stmts) > 0 && !slices.ContainsFunc(x.Stmts, func(s syntax.Stmt) bool {
		_, ok := s.(*syntax.Assign)
		return ok
	})
}

// local returns what u knows of the local variable that id, if ok,
// names, or nil.
func (u *unusedVars) local(id *syntax.Ident, ok bool) *localVar {
	if !ok || id == nil || id.Sym == nil {
		return nil
	}
	return u.vars[id.Sym]
}

// beforeCapture reports whether the write of variable v at position p,
// in the statement at loc, comes before every lambda, local function and
// local class that uses v, outside any loop in v's scope.
func (u *unusedVars) beforeCapture(v *localVar, p syntax.Pos, loc stmtLoc) bool {
	if !v.captured || !v.captureReads && !u.inCapture(v, loc) {
		// What only assigns it can't read a value, wherever it runs.
		return true
	}
	if !before(p, v.capture) {
		return false
	}
	for b := loc.block; b != v.block; {
		switch n := u.blockParent[b].(type) {
		case *syntax.If, *syntax.When, *syntax.Try:
			l, ok := u.stmtOf(n)
			if !ok {
				return false
			}
			b = l.block
		default:
			return false
		}
	}
	return true
}

// inCapture reports whether the statement at loc, in variable v's
// scope, is in a lambda, a local function or a local class there.
func (u *unusedVars) inCapture(v *localVar, loc stmtLoc) bool {
	for b := loc.block; b != v.block; {
		n, ok := u.blockParent[b]
		if !ok {
			return true
		}
		if _, ok := n.(*syntax.Lambda); ok {
			return true
		}
		l, ok := u.stmtOf(n)
		if !ok {
			return true
		}
		b = l.block
	}
	return false
}

// unreadFromLambda reports whether the value that a statement at loc, in
// a lambda that only assigns variable v, writes to v is never read: v is
// read nowhere after the lambda begins, and the lambda isn't in a loop,
// whose next iteration may read it. The lambda may run at any time
// after it is created, so only a read before can't see the value.
func (u *unusedVars) unreadFromLambda(v *localVar, loc stmtLoc) bool {
	if !v.captured || v.captureReads || v.block == nil {
		return false
	}
	var lam *syntax.Lambda
	for b := loc.block; b != v.block; {
		switch n := u.blockParent[b].(type) {
		case *syntax.Lambda:
			lam = n
			l, ok := u.lambdaLoc[n]
			if !ok {
				return false
			}
			b = l.block
		case *syntax.If, *syntax.When, *syntax.Try:
			l, ok := u.stmtOf(n)
			if !ok {
				return false
			}
			b = l.block
		default:
			return false // a loop, or what kc doesn't follow
		}
	}
	if lam == nil {
		return false
	}
	start := syntax.StartPos(lam)
	return !slices.ContainsFunc(v.readPos, func(p syntax.Pos) bool { return !before(p, start) })
}

// stmtOf returns the place of the statement that is if, when or try
// construct n.
func (u *unusedVars) stmtOf(n any) (stmtLoc, bool) {
	if x, ok := n.(syntax.Expr); ok {
		if l, ok := u.exprStmt[x]; ok {
			return l, true
		}
	}
	if st, ok := n.(syntax.Stmt); ok {
		l, ok := u.stmtLoc[st]
		return l, ok
	}
	return stmtLoc{}, false
}

// capture marks the variables that declaration d, a function or a class
// nested in another function that begins at p, uses but doesn't declare
// as captured. The variables it declares are declared first, as its code
// comes before.
func (u *unusedVars) capture(d syntax.Decl, p syntax.Pos) {
	own := map[*types.VarSym]bool{}
	var used []*types.VarSym
	assigned := map[*syntax.Ident]bool{} // the targets of plain assignments
	reads := map[*types.VarSym]bool{}
	walkCode([]syntax.Decl{d}, func(n any) bool {
		switch n := n.(type) {
		case *syntax.VarStmt:
			own[n.Sym] = true
		case *syntax.Destructure:
			for _, vs := range n.Stmts {
				own[vs.Sym] = true
			}
		case *syntax.When:
			if n.SubjVar != "" {
				own[n.SubjSym] = true
			}
		case *syntax.Assign:
			if id, ok := n.Target.(*syntax.Ident); ok && n.Op == syntax.ASSIGN {
				assigned[id] = true
			}
		case *syntax.Ident:
			if n.Sym != nil {
				used = append(used, n.Sym)
				if !assigned[n] {
					reads[n.Sym] = true
				}
			}
		}
		return true
	})
	for _, s := range used {
		if v := u.vars[s]; v != nil && !own[s] {
			if !v.captured || before(p, v.capture) {
				v.capture = p
			}
			v.captured = true
			v.captureReads = v.captureReads || reads[s]
		}
	}
}

// declPos returns the position of a variable's declaration.
func declPos(d any) syntax.Pos {
	switch d := d.(type) {
	case *syntax.VarStmt:
		return d.P
	case *syntax.When:
		return d.Pos()
	}
	return syntax.Pos{}
}

// The effects on a variable's value of a statement that follows a write
// of it.
const (
	passes = iota // it neither reads nor overwrites the value
	kills         // it overwrites the value, or leaves the function, on every path
	reads         // it may read the value, or jump elsewhere
)

// unreadAfter reports whether the value that the statement at loc writes
// to variable sym, v, is certainly never read: the statements after it
// overwrite it or leave the function, or v goes out of scope, before
// any reads it.
func (u *unusedVars) unreadAfter(sym *types.VarSym, v *localVar, loc stmtLoc) bool {
	// An exception, after any statement, may carry the value to a catch or
	// finally block in v's scope: the catches must overwrite it, or leave,
	// before they may read it, and the finally block too, or not read it.
	// Leaving them goes through the finally block and out of the try, to
	// the next try in v's scope, or out of v's scope.
	for _, t := range u.tries[loc.block] {
		if slices.Contains(u.tries[v.block], t) {
			continue
		}
		for _, ct := range t.Catches {
			if !u.blockKills(ct.Body, sym) {
				return false
			}
		}
		if t.Finally != nil && u.blockEffect(t.Finally, sym) == reads {
			return false
		}
	}
	b, i := loc.block, loc.index
	for {
		for _, st := range b.Stmts[i+1:] {
			switch u.effect(st, sym) {
			case kills:
				return true
			case reads:
				return false
			}
		}
		if b == v.block {
			// The condition of a do-while sees its body's variables.
			w, ok := u.blockParent[b].(*syntax.While)
			return !ok || !w.Do || !readsVar(w.Cond, sym)
		}
		switch p := u.blockParent[b].(type) {
		case *syntax.If, *syntax.When:
			l, ok := u.exprStmt[p.(syntax.Expr)]
			if !ok {
				return false
			}
			b, i = l.block, l.index
		case *syntax.Try:
			// The end of its block or a catch goes through the finally
			// block.
			if p.Finally != nil && b != p.Finally {
				switch u.blockEffect(p.Finally, sym) {
				case kills:
					return true
				case reads:
					return false
				}
			}
			l, ok := u.exprStmt[p]
			if !ok {
				return false
			}
			b, i = l.block, l.index
		case *syntax.While, *syntax.For:
			// The end of a loop's body goes to its condition and then to
			// its body again, from the start, or out of the loop.
			if w, ok := p.(*syntax.While); ok && readsVar(w.Cond, sym) {
				return false
			}
			body := u.loopBody(p)
			if body != b {
				return false
			}
			if u.blockEffect(body, sym) == reads {
				return false
			}
			l, ok := u.stmtOf(p)
			if !ok {
				return false
			}
			b, i = l.block, l.index
		default:
			return false
		}
	}
}

// loopBody returns the body of loop n, a while or a for.
func (u *unusedVars) loopBody(n any) *syntax.Block {
	switch n := n.(type) {
	case *syntax.While:
		return n.Body
	case *syntax.For:
		return n.Body
	}
	return nil
}

// effect returns what statement st does with the value of variable sym.
func (u *unusedVars) effect(st syntax.Stmt, sym *types.VarSym) int {
	if breaksOut(st) {
		return reads
	}
	switch st := st.(type) {
	case *syntax.While:
		// A loop without break or continue: its body runs at least once
		// after do, and while (true) never ends; otherwise the value may
		// pass it, but not be read in it.
		switch {
		case st.Do:
			switch u.blockEffect(st.Body, sym) {
			case kills:
				return kills
			case passes:
				if !readsVar(st.Cond, sym) {
					return passes
				}
			}
			return reads
		case readsVar(st.Cond, sym) || u.blockEffect(st.Body, sym) == reads:
			return reads
		case st.Infinite && u.blockEffect(st.Body, sym) == kills:
			return kills
		}
		return passes
	case *syntax.For:
		if readsVar(st.Range, sym) || u.blockEffect(st.Body, sym) == reads {
			return reads
		}
		return passes
	case *syntax.Assign:
		if id, ok := st.Target.(*syntax.Ident); ok && id.Sym == sym && st.Op == syntax.ASSIGN {
			if varUses(st.Value, sym) > 0 {
				return reads
			}
			return kills
		}
	case *syntax.ExprStmt:
		switch x := st.X.(type) {
		case *syntax.Jump:
			if (x.Kind == syntax.RETURN || x.Kind == syntax.THROW) && varUses(x.X, sym) == 0 {
				return kills
			}
		case *syntax.If:
			if x.Else != nil && !readsVar(x.Cond, sym) && u.blockKills(x.Then, sym) && u.blockKills(x.Else, sym) {
				return kills
			}
		case *syntax.When:
			if u.whenKills(x, sym) {
				return kills
			}
		case *syntax.Try:
			if u.tryKills(x, sym) {
				return kills
			}
		}
	}
	if readsVar(st, sym) {
		return reads
	}
	return passes
}

// whenKills reports whether when statement x, which has an else branch,
// overwrites variable sym on every path, reading it nowhere before.
func (u *unusedVars) whenKills(x *syntax.When, sym *types.VarSym) bool {
	if x.Subject != nil && readsVar(x.Subject, sym) {
		return false
	}
	hasElse := false
	for _, br := range x.Branches {
		hasElse = hasElse || br.Else
		for _, cd := range br.Conds {
			if cd.X != nil && readsVar(cd.X, sym) {
				return false
			}
		}
		if br.Guard != nil && readsVar(br.Guard, sym) || !u.blockKills(br.Body, sym) {
			return false
		}
	}
	return hasElse
}

// tryKills reports whether try statement x overwrites variable sym on
// every path, reading it nowhere before: its finally block does, and
// nothing before reads it, or its block and catches do, the catches
// entered from anywhere in the block, and the finally block doesn't
// read it.
func (u *unusedVars) tryKills(x *syntax.Try, sym *types.VarSym) bool {
	catchesKill, catchesRead := true, false
	for _, ct := range x.Catches {
		catchesKill = catchesKill && u.blockKills(ct.Body, sym)
		catchesRead = catchesRead || readsVar(ct.Body, sym)
	}
	if x.Finally != nil && u.blockKills(x.Finally, sym) && !readsVar(x.Body, sym) && !catchesRead {
		return true
	}
	return u.blockKills(x.Body, sym) && catchesKill && (x.Finally == nil || !readsVar(x.Finally, sym))
}

// blockKills reports whether block b overwrites variable sym, or leaves
// the function, before it may read it.
func (u *unusedVars) blockKills(b *syntax.Block, sym *types.VarSym) bool {
	return b != nil && u.blockEffect(b, sym) == kills
}

// blockEffect returns what block b does with the value of variable sym:
// the effect of the first of its statements that does something with
// it.
func (u *unusedVars) blockEffect(b *syntax.Block, sym *types.VarSym) int {
	for _, st := range b.Stmts {
		if e := u.effect(st, sym); e != passes {
			return e
		}
	}
	return passes
}

// breaksOut reports whether node n has a break or a continue, which may take
// a value to a read after the loop or in its next iteration.
func breaksOut(n any) bool {
	found := false
	syntax.Inspect(n, func(m any) bool {
		if j, ok := m.(*syntax.Jump); ok && (j.Kind == syntax.BREAK || j.Kind == syntax.CONTINUE) {
			found = true
		}
		return !found
	})
	return found
}

// varUses returns the number of uses of variable sym in node n.
func varUses(n any, sym *types.VarSym) int {
	k := 0
	walkNode(n, func(m any) bool {
		switch m := m.(type) {
		case *syntax.Ident:
			if m.Sym == sym {
				k++
			}
		case *syntax.Assign:
			// Not its rewritten Stmts.
			k += varUses(m.Target, sym) + varUses(m.Value, sym)
			return false
		case *syntax.Unary:
			if m.Op == syntax.INC || m.Op == syntax.DEC {
				k += varUses(m.X, sym)
				return false
			}
		}
		return true
	})
	return k
}

// readsVar reports whether node n may read variable sym: it uses it
// other than as the target of a plain assignment.
func readsVar(n any, sym *types.VarSym) bool {
	found := false
	walkNode(n, func(m any) bool {
		if found {
			return false
		}
		switch m := m.(type) {
		case *syntax.Ident:
			found = m.Sym == sym
		case *syntax.Assign:
			if id, ok := m.Target.(*syntax.Ident); ok && id.Sym == sym && m.Op == syntax.ASSIGN {
				found = readsVar(m.Value, sym)
			} else {
				found = readsVar(m.Target, sym) || readsVar(m.Value, sym)
			}
			return false
		case *syntax.Unary:
			if m.Op == syntax.INC || m.Op == syntax.DEC {
				found = readsVar(m.X, sym)
				return false
			}
		}
		return !found
	})
	return found
}

// walkNode calls f for node n, a statement or an expression, and the
// statements and expressions in it, as syntax.Inspect does, also in the
// classes of object expressions and local classes and in when branches'
// guards.
func walkNode(n any, f func(any) bool) {
	var visit func(m any) bool
	visit = func(m any) bool {
		if !f(m) {
			return false
		}
		switch m := m.(type) {
		case *syntax.ObjectExpr:
			walkCode([]syntax.Decl{m.Decl}, f)
		case *syntax.LocalClass:
			walkCode([]syntax.Decl{m.Decl}, f)
		case *syntax.When:
			for _, br := range m.Branches {
				if br.Guard != nil {
					syntax.Inspect(br.Guard, visit)
				}
			}
		}
		return true
	}
	syntax.Inspect(n, visit)
}

// writeCount is how many times paths through code may assign a variable,
// as writes counts them: on those that complete it, on those that leave
// it by return, throw or break, and on those that continue a loop. Each
// count is -1 if no path does, and capped at 2, for more than once.
type writeCount struct{ cont, out, back int }

// plusCount returns a+b of two counts, either of which may be -1.
func plusCount(a, b int) int {
	if a < 0 || b < 0 {
		return -1
	}
	return min(a+b, 2)
}

// atMostOnce reports whether the variable sym, declared without an
// initializer as statement i of block b, is assigned at most once on
// every path, as kotlinc's CanBeValChecker judges it: then it can be a
// val. Loops whose bodies may assign it and run again, and lambdas,
// local functions and classes that assign it, unless the lambda is run
// once in place, make it more.
func atMostOnce(sym *types.VarSym, b *syntax.Block, i int) bool {
	w, ok := writes(sym, b.Stmts[i+1:])
	return ok && w.cont <= 1 && w.out <= 1 && w.back <= 1
}

// writes counts the assignments of sym in statements stmts, run in turn
// (see writeCount). It reports false where it can't tell.
func writes(sym *types.VarSym, stmts []syntax.Stmt) (writeCount, bool) {
	w := writeCount{cont: 0, out: -1, back: -1}
	for _, s := range stmts {
		sw, ok := stmtWrites(sym, s)
		if !ok {
			return w, false
		}
		w.out = max(w.out, plusCount(w.cont, sw.out))
		w.back = max(w.back, plusCount(w.cont, sw.back))
		w.cont = plusCount(w.cont, sw.cont)
		if w.cont < 0 {
			break // what follows can't run
		}
	}
	return w, true
}

// blockWrites is writes of block b, which may be nil.
func blockWrites(sym *types.VarSym, b *syntax.Block) (writeCount, bool) {
	if b == nil {
		return writeCount{cont: 0, out: -1, back: -1}, true
	}
	return writes(sym, b.Stmts)
}

// branchWrites joins the counts of alternatives.
func branchWrites(ws ...writeCount) writeCount {
	j := writeCount{cont: -1, out: -1, back: -1}
	for _, w := range ws {
		j.cont, j.out, j.back = max(j.cont, w.cont), max(j.out, w.out), max(j.back, w.back)
	}
	return j
}

// seqWrites is a followed by b.
func seqWrites(a, b writeCount) writeCount {
	return writeCount{
		cont: plusCount(a.cont, b.cont),
		out:  max(a.out, plusCount(a.cont, b.out)),
		back: max(a.back, plusCount(a.cont, b.back)),
	}
}

// stmtWrites counts the assignments of sym in statement s.
func stmtWrites(sym *types.VarSym, s syntax.Stmt) (writeCount, bool) {
	switch s := s.(type) {
	case *syntax.ExprStmt:
		return exprWrites(sym, s.X)
	case *syntax.VarStmt:
		if s.Delegate != nil {
			return flatWrites(sym, s)
		}
		return exprWrites(sym, s.Init)
	case *syntax.Assign:
		w, ok := exprWrites(sym, s.Value)
		if !ok {
			return w, false
		}
		if id, ok := s.Target.(*syntax.Ident); ok && id.Sym == sym {
			if s.Op != syntax.ASSIGN {
				return w, false // x += v reads it, assigned before
			}
			return seqWrites(w, writeCount{cont: 1, out: -1, back: -1}), true
		}
		t, ok := flatWrites(sym, s.Target)
		return seqWrites(t, w), ok
	case *syntax.While:
		cond, ok1 := exprWrites(sym, s.Cond)
		body, ok2 := blockWrites(sym, s.Body)
		if !ok1 || !ok2 || cond.cont != 0 || cond.out > 0 {
			return writeCount{}, false
		}
		return loopWrites(body)
	case *syntax.For:
		r, ok1 := exprWrites(sym, s.Range)
		body, ok2 := blockWrites(sym, s.Body)
		if !ok1 || !ok2 {
			return writeCount{}, false
		}
		l, ok := loopWrites(body)
		return seqWrites(r, l), ok
	}
	return flatWrites(sym, s)
}

// loopWrites counts the assignments of a loop with body counts body: none
// on a path that runs it again, and then those of its paths out of it,
// which, return or break, are taken to complete it.
func loopWrites(body writeCount) (writeCount, bool) {
	if body.cont > 0 || body.back > 0 {
		return writeCount{}, false
	}
	return writeCount{cont: max(body.out, 0), out: -1, back: -1}, true
}

// exprWrites counts the assignments of sym in expression x, which may be
// nil.
func exprWrites(sym *types.VarSym, x syntax.Expr) (writeCount, bool) {
	switch x := x.(type) {
	case nil:
		return writeCount{cont: 0, out: -1, back: -1}, true
	case *syntax.If:
		cond, ok1 := exprWrites(sym, x.Cond)
		then, ok2 := blockWrites(sym, x.Then)
		els, ok3 := blockWrites(sym, x.Else)
		return seqWrites(cond, branchWrites(then, els)), ok1 && ok2 && ok3
	case *syntax.When:
		w, ok := exprWrites(sym, x.Subject)
		if !ok {
			return w, false
		}
		var bs []writeCount
		if !whenHasElse(x) {
			bs = append(bs, writeCount{cont: 0, out: -1, back: -1}) // no branch matches
		}
		for _, br := range x.Branches {
			for _, c := range br.Conds {
				if cw, ok := flatWrites(sym, c.X); !ok || cw.cont != 0 {
					return w, false
				}
			}
			if gw, ok := flatWrites(sym, br.Guard); !ok || gw.cont != 0 {
				return w, false
			}
			bw, ok := blockWrites(sym, br.Body)
			if !ok {
				return w, false
			}
			bs = append(bs, bw)
		}
		return seqWrites(w, branchWrites(bs...)), true
	case *syntax.Try:
		// A catch may be entered after any of the body's statements.
		body, ok := blockWrites(sym, x.Body)
		if !ok {
			return body, false
		}
		before := max(body.cont, body.out, body.back, 0)
		w := body
		for _, ct := range x.Catches {
			cw, ok := blockWrites(sym, ct.Body)
			if !ok {
				return cw, false
			}
			w = branchWrites(w, seqWrites(writeCount{cont: before, out: -1, back: -1}, cw))
		}
		if x.Finally != nil {
			fw, ok := blockWrites(sym, x.Finally)
			if !ok || fw.cont < 0 {
				return fw, false
			}
			w = writeCount{cont: plusCount(w.cont, fw.cont), out: plusCount(w.out, fw.cont), back: plusCount(w.back, fw.cont)}
		}
		return w, true
	case *syntax.Jump:
		w, ok := exprWrites(sym, x.X)
		if !ok || w.cont < 0 {
			return w, ok
		}
		if x.Kind == syntax.CONTINUE {
			return writeCount{cont: -1, out: w.out, back: max(w.back, w.cont)}, true
		}
		return writeCount{cont: -1, out: max(w.out, w.cont), back: w.back}, true
	case *syntax.Lambda:
		if x.Once && x.Inline && x.Ref == nil && !x.Anon {
			// Run once, in place: its returns leave it, or more.
			w, ok := blockWrites(sym, x.Body)
			return writeCount{cont: max(w.cont, w.out), out: -1, back: w.back}, ok
		}
		if assignsVar(x, sym) {
			return writeCount{}, false // run any number of times
		}
	case *syntax.ObjectExpr:
		if assignsVar(x, sym) {
			return writeCount{}, false
		}
	}
	return flatWrites(sym, x)
}

// flatWrites counts the assignments of sym in n, which has no statements
// of its own that it may skip or repeat: there must be none, other than
// in lambdas run once in place (see exprWrites).
func flatWrites(sym *types.VarSym, n any) (writeCount, bool) {
	w := writeCount{cont: 0, out: -1, back: -1}
	ok := true
	walkNode(n, func(m any) bool {
		if !ok {
			return false
		}
		if m == n {
			return true
		}
		switch m := m.(type) {
		case *syntax.Assign:
			if id, isID := m.Target.(*syntax.Ident); isID && id.Sym == sym {
				ok = false
			}
		case *syntax.Unary:
			if id, isID := m.X.(*syntax.Ident); isID && id.Sym == sym && (m.Op == syntax.INC || m.Op == syntax.DEC) {
				ok = false
			}
		case *syntax.Lambda, *syntax.If, *syntax.When, *syntax.Try, *syntax.Jump:
			if !assignsVar(m, sym) {
				return false
			}
			if x, isExpr := m.(syntax.Expr); isExpr {
				ew, eok := exprWrites(sym, x)
				if !eok || ew.cont < 0 || ew.out >= 0 || ew.back >= 0 {
					ok = false // a branch that jumps, in the middle of n
				} else {
					w.cont = plusCount(w.cont, ew.cont)
				}
				return false
			}
		case *syntax.LocalFun, *syntax.LocalClass, *syntax.ObjectExpr:
			if assignsVar(m, sym) {
				ok = false
			}
			return false
		}
		return true
	})
	return w, ok
}

// assignsVar reports whether n assigns sym.
func assignsVar(n any, sym *types.VarSym) bool {
	found := false
	walkNode(n, func(m any) bool {
		switch m := m.(type) {
		case *syntax.Assign:
			if id, ok := m.Target.(*syntax.Ident); ok && id.Sym == sym {
				found = true
			}
		case *syntax.Unary:
			if id, ok := m.X.(*syntax.Ident); ok && id.Sym == sym && (m.Op == syntax.INC || m.Op == syntax.DEC) {
				found = true
			}
		}
		return !found
	})
	return found
}
