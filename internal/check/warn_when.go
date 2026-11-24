package check

import (
	"fmt"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// noteRedundantElse records, for localWarnings, whether the else branch
// of when x, whose subject is of type st and whose other branches cover
// cov, is redundant: kotlinc's REDUNDANT_ELSE_IN_WHEN, of a when that
// is exhaustive without it. Its subject's type is a Boolean, an enum,
// or a sealed class of the program or of Kotlin: kc stays silent of
// other whens kotlinc finds exhaustive, as one with is Any of an Any or
// a subject of a type parameter's type. A platform type is covered only
// with null, as kotlinc has it.
func (c *Checker) noteRedundantElse(x *syntax.When, st types.Type, cov *coverage, hasElse, covered bool) {
	redundant := hasElse && covered && x.Subject != nil && enumerable(st) && !hasInvalid(st, 0)
	if hasElse && covered && x.Subject != nil && !redundant && !st.P && st.K != types.TypeVar && !st.HasVars() && !hasInvalid(st, 0) {
		// A branch that is an is check of a type the subject is of
		// covers it all: is Any of an Int, is Any? of anything.
		redundant = slices.ContainsFunc(cov.types, func(t types.Type) bool {
			return t.K != types.Invalid && !t.P && !hasInvalid(t, 0) && c.subtype(st, t)
		})
		if c.redundantElses == nil {
			c.redundantElses = map[*syntax.When]bool{}
		}
		c.redundantElses[x] = redundant
		return
	}
	if redundant && st.K == types.Class {
		cls := st.C
		redundant = !cls.External || cls.FromKotlin || cls.Entries != nil
		// covers takes an entry of another enum, of the same name, for
		// the subject's in code with an incompatible enum comparison.
		if cls.Entries != nil && !slices.ContainsFunc(cov.classes, cls.IsSubclassOf) {
			for _, e := range cls.Entries {
				redundant = redundant && cov.owned[cls][e]
			}
		}
	}
	if redundant && st.P {
		up := st
		up.P, up.N = false, true
		redundant = c.covers(up, cov)
	}
	if c.redundantElses == nil {
		c.redundantElses = map[*syntax.When]bool{}
	}
	c.redundantElses[x] = redundant
}

// redundantElseWarning reports the else branch of when x if
// noteRedundantElse found it redundant.
func (c *Checker) redundantElseWarning(x *syntax.When) {
	if !c.redundantElses[x] {
		return
	}
	for _, br := range x.Branches {
		if br.Else && br.Guard == nil {
			c.warnf(br.P, "REDUNDANT_ELSE_IN_WHEN", "'when' is exhaustive so 'else' is redundant here")
		}
	}
}

// whenNullWarnings reports a null condition of when x that never or
// always holds: SENSELESS_NULL_IN_WHEN if the subject can't be null,
// SENSELESS_COMPARISON if it's of type Nothing? and so always null.
func (c *Checker) whenNullWarnings(x *syntax.When) {
	if x.Subject == nil {
		return
	}
	t := x.Subject.Type()
	if x.SubjType != nil {
		if x.SubjSym == nil {
			return
		}
		t = x.SubjSym.Type
	}
	for _, br := range x.Branches {
		for i := range br.Conds {
			cond := &br.Conds[i]
			if cond.Is || cond.NotIs || cond.In || cond.NotIn || cond.X == nil {
				continue
			}
			if !isNullLit(cond.X) {
				// A value that is always null, as a variable smart cast
				// to Nothing?.
				if ct := cond.X.Type(); ct.K == types.Null && definitelyNonNull(t) {
					c.warnf(cond.X.Pos(), "SENSELESS_NULL_IN_WHEN", "Expression under 'when' is never equal to null.")
				}
				continue
			}
			null, noted := c.nullConds[cond]
			switch {
			case definitelyNonNull(t), noted && !null:
				c.warnf(cond.X.Pos(), "SENSELESS_NULL_IN_WHEN", "Expression under 'when' is never equal to null.")
			case t.K == types.Null, null:
				c.warnf(cond.X.Pos(), "SENSELESS_COMPARISON", "condition is always 'true'")
			}
		}
	}
}

// duplicateWhenConditions reports, of when x with a subject, a branch's
// condition that a branch before has, as kotlinc's
// DUPLICATE_BRANCH_CONDITION_IN_WHEN: an equality to the same literal
// (1 and 1L are the same value, 1 and 1.0 or 'a' and "a" aren't), enum
// entry or object, or an is or !is check of the same type. As kotlinc,
// only branches of one condition without a guard count.
func (c *Checker) duplicateWhenConditions(x *syntax.When) {
	if x.Subject == nil {
		return
	}
	type isKey struct {
		t   types.Type
		not bool
	}
	var checked []isKey
	values := map[string]bool{}
	for _, br := range x.Branches {
		if br.Else || br.Guard != nil || len(br.Conds) != 1 {
			continue
		}
		cond := &br.Conds[0]
		switch {
		case cond.Is || cond.NotIs:
			t := cond.Target
			if t.K == types.Invalid || hasInvalid(t, 0) {
				continue
			}
			k := isKey{t, cond.NotIs}
			if slices.ContainsFunc(checked, func(o isKey) bool { return o.not == k.not && o.t.N == t.N && c.sameType(o.t, t) }) {
				c.warnf(cond.P, "DUPLICATE_BRANCH_CONDITION_IN_WHEN", "duplicate branch condition in 'when'")
				continue
			}
			checked = append(checked, k)
		case cond.X != nil && !cond.In && !cond.NotIn:
			v, ok := whenConstant(unchecked(cond.X))
			if !ok {
				continue
			}
			if values[v] {
				c.warnf(syntax.StartPos(cond.X), "DUPLICATE_BRANCH_CONDITION_IN_WHEN", "duplicate branch condition in 'when'")
				continue
			}
			values[v] = true
		}
	}
}

// whenConstant returns a key of x, a when condition, if it is a literal,
// an enum entry or an object, which kotlinc compares conditions by.
func whenConstant(x syntax.Expr) (string, bool) {
	neg := ""
	if u, ok := x.(*syntax.Unary); ok && u.Op == syntax.MINUS {
		// kotlinc folds -1 into a literal.
		switch u.X.(type) {
		case *syntax.IntLit, *syntax.DoubleLit, *syntax.FloatLit:
			neg, x = "-", u.X
		}
	}
	switch x := x.(type) {
	case *syntax.IntLit:
		if x.Unsigned {
			return fmt.Sprintf("u%s%d", neg, uint64(x.Val)), true
		}
		return fmt.Sprintf("i%s%d", neg, x.Val), true
	case *syntax.DoubleLit:
		return fmt.Sprintf("d%s%v", neg, x.Val), true
	case *syntax.FloatLit:
		return fmt.Sprintf("f%s%v", neg, x.Val), true
	case *syntax.CharLit:
		return fmt.Sprintf("c%d", x.Val), true
	case *syntax.BoolLit:
		return fmt.Sprint("b", x.Val), true
	case *syntax.NullLit:
		return "null", true
	case *syntax.StringLit:
		s := ""
		for _, p := range x.Parts {
			if p.X != nil {
				return "", false
			}
			s += p.Lit
		}
		return "s" + s, true
	case *syntax.Ident:
		if x.Member != nil {
			return whenConstant(x.Member) // an imported entry
		}
		if x.Object != nil {
			return "o" + x.Object.Name, true
		}
		if x.Sym != nil && x.Sym.Kind == types.Static && x.Sym.Owner != nil && isEnumEntry(x.Sym.Owner, x.Sym.Name) {
			return "e" + x.Sym.Owner.Name + "." + x.Sym.Name, true
		}
	case *syntax.Member:
		if x.Object != nil {
			return "o" + x.Object.Name, true
		}
		if x.Field != nil && x.Field.Owner != nil && isEnumEntry(x.Field.Owner, x.Field.Name) {
			return "e" + x.Field.Owner.Name + "." + x.Field.Name, true
		}
	}
	return "", false
}
