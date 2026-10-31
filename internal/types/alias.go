package types

// AliasSym is a typealias of the program, Name<TypeParams> = Type, at the
// top level of a package or nested in a class.
type AliasSym struct {
	Name       string
	Pkg        string    // the package's dotted name
	Outer      *ClassSym // the class it is nested in, or nil
	TypeParams []*TypeParamSym
	// Type is the expansion, with TypeParams as ParamTypes. It is valid
	// once Resolved is set, InvalidType for an alias whose right side
	// doesn't resolve or is part of a cycle.
	Type     Type
	Resolved bool
	// Resolving is set while the right side is resolved, to find cycles.
	Resolving bool
	Decl      any // *syntax.TypeAliasDecl
	// Private and Internal are the alias's visibility.
	Private, Internal bool
}

// Expand returns the type the alias stands for with these type
// arguments, one for each of its type parameters.
func (a *AliasSym) Expand(args []Arg) Type {
	if len(args) == 0 || a.Type.K == Invalid {
		return a.Type
	}
	s := Subst{}
	for i, p := range a.TypeParams {
		s[p] = args[i]
	}
	return s.Apply(a.Type)
}

// Class returns the class the alias expands to, if it expands to a class
// type: the class that Alias(...) constructs and Alias.member reads.
func (a *AliasSym) Class() *ClassSym {
	if a.Type.K != Class || a.Type.C == nil || a.Type.C.Func {
		return nil
	}
	return a.Type.C
}
