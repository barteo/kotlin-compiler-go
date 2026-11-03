package check

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// narrow records that a variable is known to have a more specific type
// (a smart cast) for the rest of a scope. With recv, it is the property
// sym of the stable value recv: a.b.
type narrow struct {
	recv *types.VarSym
	sym  *types.VarSym
	t    types.Type
}

type scope struct {
	parent  *scope
	vars    []*types.VarSym
	funs    []*types.FunSym   // local functions
	classes []*types.ClassSym // local classes
	narrows []narrow
	depth   int // the number of lambdas the scope is in
}

// declareFun declares a local function. If the scope has one with the
// same name and parameters, it returns that one instead.
func (s *scope) declareFun(f *types.FunSym) *types.FunSym {
	k := localKey(f)
	for _, g := range s.funs {
		if g.Name == f.Name && localKey(g) == k {
			return g
		}
	}
	s.funs = append(s.funs, f)
	return nil
}

// localKey is paramKey for a local function, whose context parameters
// tell it apart by their types, not erased, and in any order: local
// functions' JVM names differ.
func localKey(f *types.FunSym) string {
	if len(f.Contexts) == 0 {
		return paramKey(f)
	}
	ctx := make([]string, len(f.Contexts))
	for i, p := range f.Contexts {
		ctx[i] = p.Type.String()
	}
	slices.Sort(ctx)
	return paramKey2(f) + "c" + strings.Join(ctx, ",")
}

// findFuns finds the local functions named name in the innermost scope
// that declares one, and returns the depth of that scope.
func (s *scope) findFuns(name string) ([]*types.FunSym, int) {
	for sc := s; sc != nil; sc = sc.parent {
		var fs []*types.FunSym
		for _, f := range sc.funs {
			if f.Name == name {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 {
			return fs, sc.depth
		}
	}
	return nil, 0
}

// findFunsFor is findFuns for a call, whose functions fits reports may
// take its arguments: as in Kotlin, where each level of scope is tried in
// turn, a scope's local functions give way to an outer scope's when none
// fits. Without any that fits, it is findFuns.
func (s *scope) findFunsFor(name string, fits func([]*types.FunSym) bool) ([]*types.FunSym, int) {
	first, depth := s.findFuns(name)
	if first == nil || fits(first) {
		return first, depth
	}
	for sc := s; sc != nil; sc = sc.parent {
		var fs []*types.FunSym
		for _, f := range sc.funs {
			if f.Name == name {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 && fits(fs) {
			return fs, sc.depth
		}
	}
	return first, depth
}

func (s *scope) declare(v *types.VarSym) bool {
	for _, w := range s.vars {
		if w.Name == v.Name {
			return false
		}
	}
	s.vars = append(s.vars, v)
	return true
}

// lookup finds a local by name and returns it with its smart-cast type.
func (s *scope) lookup(name string) (*types.VarSym, types.Type, bool) {
	v, _ := s.find(name)
	if v == nil {
		return nil, types.Type{}, false
	}
	return v, s.narrowed(v), true
}

// find finds a local by name and returns the depth of the scope that
// declares it.
// findClass finds a local class by its simple name.
func (s *scope) findClass(name string) *types.ClassSym {
	for sc := s; sc != nil; sc = sc.parent {
		for _, k := range sc.classes {
			if k.Decl.(*syntax.ClassDecl).Name == name {
				return k
			}
		}
	}
	return nil
}

func (s *scope) find(name string) (*types.VarSym, int) {
	for sc := s; sc != nil; sc = sc.parent {
		for i := len(sc.vars) - 1; i >= 0; i-- {
			if v := sc.vars[i]; v.Name == name {
				return v, sc.depth
			}
		}
	}
	return nil, 0
}

func (s *scope) narrowed(v *types.VarSym) types.Type {
	return s.narrowedMember(nil, v, v.Type)
}

// narrowedMember returns the smart-cast type of property v of the stable
// value recv, or of variable v if recv is nil, or declared.
func (s *scope) narrowedMember(recv, v *types.VarSym, declared types.Type) types.Type {
	for sc := s; sc != nil; sc = sc.parent {
		for i := len(sc.narrows) - 1; i >= 0; i-- {
			if n := sc.narrows[i]; n.sym == v && n.recv == recv {
				return n.t
			}
		}
	}
	return declared
}
