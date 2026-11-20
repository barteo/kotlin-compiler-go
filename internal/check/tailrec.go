package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// tailCalls marks the calls of tailrec function d to itself that are in
// tail position, which codegen turns into a jump back to the start of its
// body, as kotlinc does: the value of a return, of the expression body,
// or of the last statement of the body, through if and when branches and
// the right side of ?:, && and ||, and a statement followed by a return
// without a value, which leaves the function. In a lambda passed to an
// inline function, so are the values of its returns from the function,
// but not its last statement, which doesn't leave it: those jumps run
// the finally blocks of the inlined body's trys they leave, as kotlinc's
// do. Calls in other lambdas, local functions, object expressions and
// try blocks aren't, and stay calls, which tailWarnings reports as
// kotlinc does. Neither are a suspend function's in inline lambdas,
// which codegen can't always turn into jumps.
func (c *Checker) tailCalls(d *syntax.FunDecl) {
	c.tailPositions(d, false, func(call *syntax.Call) { call.Tail = true })
	// kotlinc finds more tail calls than codegen makes jumps of, but not
	// those in code that can't run: it warns of the calls that are none
	// of those, and that there are no tail calls only where none is one.
	if !c.tailWarnings(d) && (d.Body != nil || d.ExprBody != nil) {
		p := d.P
		for _, w := range d.Written {
			if w.Name == "tailrec" {
				p = w.P
			}
		}
		c.warnf(p, "NO_TAIL_CALLS_FOUND", "a function is marked as tail-recursive but no tail calls are found")
	}
}

// tailWarnings reports, as kotlinc does, tailrec function d's calls of
// itself that aren't tail calls: NON_TAIL_RECURSIVE_CALL where the
// function goes on after it, in lambdas, local functions and object
// expressions too, or where it is on another instance of d's class than
// this, and TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED where it is in a try,
// catch or finally block. Calls in code that can't run aren't reported.
// It reports whether any call is a tail call to kotlinc.
func (c *Checker) tailWarnings(d *syntax.FunDecl) bool {
	sym := d.Sym
	if sym == nil || d.Body == nil && d.ExprBody == nil {
		return false
	}
	var body any = d.Body
	if d.ExprBody != nil {
		body = d.ExprBody
	}
	tail := map[*syntax.Call]bool{}
	c.tailPositions(d, true, func(call *syntax.Call) { tail[call] = true })
	var calls []*syntax.Call
	inTry, dead := map[*syntax.Call]bool{}, map[*syntax.Call]bool{}
	self := func(n any) (*syntax.Call, bool) {
		call, ok := n.(*syntax.Call)
		return call, ok && call.Sym == sym && !call.Invoke
	}
	mark := func(n any, m map[*syntax.Call]bool) {
		syntax.Inspect(n, func(n any) bool {
			if call, ok := self(n); ok {
				m[call] = true
			}
			return true
		})
	}
	var visit func(n any) bool
	visit = func(n any) bool {
		switch n := n.(type) {
		case *syntax.Try:
			mark(n, inTry)
		case *syntax.Block:
			if i, ok := c.blockJumps[n]; ok && i+1 < len(n.Stmts) {
				for _, st := range n.Stmts[i+1:] {
					mark(st, dead)
				}
			}
		case *syntax.ObjectExpr:
			walkCode([]syntax.Decl{n.Decl}, visit)
			return false
		case *syntax.LocalClass:
			walkCode([]syntax.Decl{n.Decl}, visit)
			return false
		}
		if call, ok := self(n); ok {
			calls = append(calls, call)
		}
		return true
	}
	syntax.Inspect(body, visit)
	valid := false
	for _, call := range calls {
		switch {
		case otherReceiver(call, sym):
			c.warnf(syntax.StartPos(call), "NON_TAIL_RECURSIVE_CALL", "recursive call is not a tail call")
		case dead[call]:
		case inTry[call]:
			c.warnf(syntax.StartPos(call), "TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED", "tail recursion optimization inside try/catch/finally is not supported")
		case !tail[call]:
			c.warnf(syntax.StartPos(call), "NON_TAIL_RECURSIVE_CALL", "recursive call is not a tail call")
		default:
			valid = true
		}
	}
	return valid
}

// otherReceiver reports whether call of member function f, a call of
// itself, may be on another instance than this, which kotlinc doesn't
// make a tail call: other than this of f's class, unless f is a member of
// an object. An extension's receiver may be any.
func otherReceiver(call *syntax.Call, f *types.FunSym) bool {
	if f.Owner == nil || f.Ext || f.Owner.Object {
		return false
	}
	m, ok := call.Fun.(*syntax.Member)
	if !ok {
		return false
	}
	t, ok := m.X.(*syntax.This)
	return !ok || t.Recv != nil
}

// tailPositions calls mark with each call of tailrec function d to itself
// in tail position, as tailCalls describes. With loose, it also takes
// those kotlinc does that codegen doesn't make jumps of: calls on another
// receiver, the left side of ?: where d's result isn't nullable, returns labeled with d's name and those
// in a suspend function's inline lambdas.
func (c *Checker) tailPositions(d *syntax.FunDecl, loose bool, mark func(*syntax.Call)) {
	sym := d.Sym
	var tail func(e syntax.Expr)
	tailStmt := func(s syntax.Stmt) {
		if s, ok := s.(*syntax.ExprStmt); ok {
			tail(s.X)
		}
	}
	tailBlock := func(b *syntax.Block) {
		if b != nil && len(b.Stmts) > 0 {
			tailStmt(b.Stmts[len(b.Stmts)-1])
		}
	}
	tail = func(e syntax.Expr) {
		switch e := e.(type) {
		case *syntax.Call:
			if e.Sym == sym && (loose || selfCall(e, sym)) && !(sym.DefaultsOf != nil && omitsArgs(e, sym)) {
				mark(e)
			}
		case *syntax.If:
			tailBlock(e.Then)
			tailBlock(e.Else)
		case *syntax.When:
			for _, br := range e.Branches {
				tailBlock(br.Body)
			}
		case *syntax.Binary:
			if e.Op == syntax.ELVIS || e.Op == syntax.AND || e.Op == syntax.OR {
				tail(e.Y)
			}
			if loose && e.Op == syntax.ELVIS && !types.MayBeNull(sym.Ret) {
				tail(e.X)
			}
		case *syntax.Jump:
			if e.Kind == syntax.RETURN && (e.Label == "" || loose && e.Label == d.Name) && e.X != nil {
				tail(e.X)
			}
		}
	}
	walk := func(n any) bool {
		switch n := n.(type) {
		case *syntax.Lambda:
			return n.Inline && (loose || !sym.Suspend)
		case *syntax.LocalFun, *syntax.ObjectExpr, *syntax.Try:
			return false
		case *syntax.Jump:
			tail(n)
		case *syntax.Block:
			for i := 1; i < len(n.Stmts); i++ {
				if bareReturn(n.Stmts[i]) {
					tailStmt(n.Stmts[i-1])
				}
			}
		}
		return true
	}
	if d.ExprBody != nil {
		syntax.Inspect(d.ExprBody, walk)
		tail(d.ExprBody)
	} else if d.Body != nil {
		syntax.Inspect(d.Body, walk)
		tailBlock(d.Body)
	}
}

// omitsArgs reports whether call of f leaves out arguments of
// parameters with default values. An override's, inherited, come from
// the overridden function's name$default: kotlinc doesn't make such a
// call a tail call (NO_TAIL_CALLS_FOUND).
func omitsArgs(call *syntax.Call, f *types.FunSym) bool {
	passed := make([]bool, len(f.Params))
	for i := range call.Args {
		j := i
		if call.ArgMap != nil {
			j = call.ArgMap[i]
		}
		if j >= 0 && j < len(passed) {
			passed[j] = true
		}
	}
	for i, p := range f.Params {
		if !passed[i] && !p.Vararg {
			return true
		}
	}
	return false
}

// bareReturn reports whether s is a return from the function without a
// value.
func bareReturn(s syntax.Stmt) bool {
	es, ok := s.(*syntax.ExprStmt)
	if !ok {
		return false
	}
	j, ok := es.X.(*syntax.Jump)
	return ok && j.Kind == syntax.RETURN && j.Label == "" && j.Lambda == nil && j.X == nil
}

// selfCall reports whether call of f has f's own receivers: this, the
// object f is a member of, named (O.f(), C.f() for a companion's), or for
// a top-level extension function any receiver, which the jump passes as
// the new one.
func selfCall(call *syntax.Call, f *types.FunSym) bool {
	switch fn := call.Fun.(type) {
	case *syntax.Ident:
		return true
	case *syntax.Member:
		// Not this of a lambda with a receiver, with(a) { f() }: a's f.
		if t, ok := fn.X.(*syntax.This); ok && t.Recv == nil && !fn.Safe {
			return true
		}
		if f.Owner != nil && !fn.Safe && namedObject(fn.X) == f.Owner {
			return true
		}
		return f.Ext && f.Owner == nil && !fn.Safe && fn.Dispatch == nil
	}
	return false
}

// namedObject returns the object, or companion object, that x names, or
// nil.
func namedObject(x syntax.Expr) *types.ClassSym {
	switch x := x.(type) {
	case *syntax.Ident:
		return x.Object
	case *syntax.Member:
		return x.Object
	}
	return nil
}
