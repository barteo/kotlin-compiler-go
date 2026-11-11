package syntax

// Clone returns a copy of f that shares nothing the checker may change
// with f: every node, slice and map of the tree is copied (clone_gen.go,
// made from the package's types by TestCloneGenerated); strings and the
// checker's symbols and types, which a lowered tree doesn't hold yet, are
// not. A node that the tree holds twice, as a delegated property's
// accessors hold the property, is copied once if shared is set, which
// costs a map of the copies; shares reports whether f had such a node,
// so that a caller can leave shared unset for later copies of f.
func Clone(f *File, shared bool) (c *File, shares bool) {
	cl := &cloner{}
	if shared || len(f.Annotated) > 0 {
		cl.memo = map[any]any{}
	}
	c = cl.cloneFile(f)
	for _, m := range cl.keys {
		// File.Annotated's keys are nodes of the tree: the copy's are
		// their copies.
		r := make(map[any][]Annotation, len(*m))
		for k, v := range *m {
			if y, ok := cl.memo[k]; ok {
				k = y
			}
			r[k] = v
		}
		*m = r
	}
	return c, cl.hits > 0 || len(f.Annotated) > 0
}

type cloner struct {
	memo map[any]any // node -> its copy, if a node may be held twice
	hits int         // nodes found in memo
	keys []*map[any][]Annotation
}

// cloneAny copies a node held as any.
func (c *cloner) cloneAny(x any) any {
	switch x := x.(type) {
	case Stmt:
		return c.cloneStmt(x)
	case Expr:
		return c.cloneExpr(x)
	}
	return x
}
