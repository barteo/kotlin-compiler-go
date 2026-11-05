package codegen

import "github.com/barteo/kotlin-compiler-go/internal/types"

// toField converts a value of property p's type on the stack to the type
// of its field, ft: an explicit backing field's, a subtype, needs a cast.
func (g *gen) toField(p *types.VarSym, ft types.Type) {
	if p.FieldT != nil {
		g.smartCast(p.Type, ft)
		return
	}
	g.coerce(p.Type, ft)
}
