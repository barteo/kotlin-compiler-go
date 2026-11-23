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
// reads the variable, going through if, when and try statements but not
// loops. In a try, a catch may be entered after any statement, and the
// finally block from anywhere. For a variable that a lambda, a local
// function or a local class uses, kotlinc judges the writes before the
// first of them, which reads the variable if it uses it other than by
// assigning it; kc judges these writes too, but not in loops, where the
// capture may come before them.

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
	annot    bool       // named in an annotation's argument
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
	tries       map[*syntax.Block][]*syntax.Try // the try constructs a block is in
	postfix     []*syntax.Unary
}

// unusedVarWarnings reports the file's unused local variables and
// assignments.
func (c *Checker) unusedVarWarnings() {
	u := &unusedVars{vars: map[*types.VarSym]*localVar{}, blockParent: map[*syntax.Block]any{},
		stmtLoc: map[syntax.Stmt]stmtLoc{}, exprStmt: map[syntax.Expr]stmtLoc{}, writeLoc: map[syntax.Expr]stmtLoc{},
		tries: map[*syntax.Block][]*syntax.Try{}}
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
					declare(st.Sym, st.Name, st, n)
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
					if x, ok := m.(*syntax.Unary); ok && (x.Op == syntax.INC || x.Op == syntax.DEC) {
						u.writeLoc[x] = stmtLoc{n, i}
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
			}
		case *syntax.While:
			u.blockParent[n.Body] = n
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
					if !n.Prefix {
						u.postfix = append(u.postfix, n)
					}
					return false
				}
			}
		case *syntax.Ident:
			if v := u.local(n, true); v != nil {
				v.read = true
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
			if loc, ok := u.stmtLoc[w]; ok && u.beforeCapture(v, w.Target.Pos(), loc) && u.unreadAfter(sym, v, loc) {
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
	if !v.captured {
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
		case *syntax.Ident:
			if n.Sym != nil {
				used = append(used, n.Sym)
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
			return true
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
		default:
			return false
		}
	}
}

// effect returns what statement st does with the value of variable sym.
func (u *unusedVars) effect(st syntax.Stmt, sym *types.VarSym) int {
	if breaksOut(st) {
		return reads
	}
	switch st := st.(type) {
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
