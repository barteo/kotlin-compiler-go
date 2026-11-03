package syntax

// The arrays of annotation arguments and of annotation parameters'
// default values: arrayOf(a, b), [a, b], intArrayOf(1), and the elements
// spread from such arrays into them, *arrayOf(c), which are constants.

// IsArrayOf reports whether call is arrayOf(...), a primitive or unsigned
// array's intArrayOf(...) or uintArrayOf(...), emptyArray(), or a
// collection literal [...], by name: kotlin.arrayOf(...) too. The checker
// may not have resolved it yet.
func IsArrayOf(call *Call) bool {
	if call.Literal {
		return true
	}
	var name string
	switch f := call.Fun.(type) {
	case *Ident:
		name = f.Name
	case *Member:
		if pkg, ok := f.X.(*Ident); !ok || pkg.Name != "kotlin" {
			return false
		}
		name = f.Name
	}
	switch name {
	case "arrayOf", "booleanArrayOf", "byteArrayOf", "charArrayOf", "shortArrayOf", "intArrayOf", "longArrayOf",
		"floatArrayOf", "doubleArrayOf", "emptyArray", "ubyteArrayOf", "ushortArrayOf", "uintArrayOf", "ulongArrayOf":
		return true
	}
	return false
}

// ArrayElems returns the elements of x if it is an array that IsArrayOf
// creates: its arguments, with the elements of arrays spread into it and
// of arrayOf(elements = arrayOf(...)), recursively.
func ArrayElems(x Expr) ([]Expr, bool) {
	call, ok := x.(*Call)
	if !ok || !IsArrayOf(call) {
		return nil, false
	}
	var out []Expr
	for i, a := range call.Args {
		if i < len(call.Names) && call.Names[i] == "elements" {
			elems, ok := ArrayElems(a)
			if !ok {
				return nil, false
			}
			out = append(out, elems...)
			continue
		}
		out = append(out, FlattenSpreads([]Expr{a})...)
	}
	return out, true
}

// FlattenSpreads replaces each element of elems that spreads an array
// ArrayElems recognizes, *arrayOf(a, b), with its elements.
func FlattenSpreads(elems []Expr) []Expr {
	var out []Expr
	for _, e := range elems {
		if sp, ok := e.(*Spread); ok {
			if inner, ok := ArrayElems(sp.X); ok {
				out = append(out, inner...)
				continue
			}
		}
		out = append(out, e)
	}
	return out
}
