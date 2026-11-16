package check

import "github.com/barteo/kotlin-compiler-go/internal/types"

// delegateRecv returns the class type that the delegate of type t, an
// unfixed variable of s that a generic provideDelegate returns, has the
// operators of: the class that its lower bounds, and the upper bounds of
// those that are variables, all are, as K2's common supertype of them is
// one of it. T of provideDelegate(MyPair<T, T>): T for a MyPair<X,
// Delegate<Y>>, where X : Delegate<Y>, is a Delegate<Y>, whose getValue
// then infers Y. Otherwise it returns t.
func (s *system) delegateRecv(t types.Type) types.Type {
	if t.K != types.TypeVar || t.N {
		return t
	}
	tv := s.byVar[t.Param()]
	if tv == nil || tv.fixed {
		return t
	}
	var recv types.Type
	same := func(u types.Type) bool {
		if u = s.apply(u); u.K != types.Class || u.N {
			return false
		}
		if recv.K == types.Invalid {
			recv = u
		}
		return u.C == recv.C
	}
	// The class-typed lower bounds first: they are the ones to use.
	for _, l := range tv.lower {
		if l := s.apply(l); l.K == types.Class && !same(l) {
			return t
		}
	}
	for _, l := range tv.lower {
		l = s.apply(l)
		switch {
		case l.K == types.Class:
		case l.K == types.TypeVar && !l.N && s.byVar[l.Param()] != nil:
			ok := false
			for _, u := range s.byVar[l.Param()].upper {
				if s.apply(u).K == types.Class {
					if !same(u) {
						return t
					}
					ok = true
				}
			}
			if !ok {
				return t
			}
		default:
			return t
		}
	}
	if recv.K == types.Invalid {
		return t
	}
	return recv
}
