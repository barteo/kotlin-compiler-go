package check

import (
	"reflect"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// TestPCLAWrite checks that the results writer replaces a variable in
// the types of nodes, of the nodes the checker makes of them, and of the
// symbols they use, and follows no reference out of the subtree.
func TestPCLAWrite(t *testing.T) {
	list := &types.ClassSym{Name: "kotlin/collections/List"}
	v := &types.TypeParamSym{Name: "E"}
	other := &types.TypeParamSym{Name: "T"}
	e := types.VarType(v)
	listOf := func(a types.Type) types.Type {
		return types.Type{K: types.Class, C: list, A: types.MakeArgs([]types.Arg{{T: a}})}
	}

	local := &types.VarSym{Name: "x", Type: listOf(e)}
	shared := &types.VarSym{Name: "y", Type: types.IntType}
	fun := &types.FunSym{Name: "f", Ret: e.Nullable(), Params: []*types.VarSym{{Name: "p", Type: e}}}
	id := &syntax.Ident{Name: "x", Sym: local}
	id.SetType(listOf(e))
	opCall := &syntax.Call{Sym: fun, Reified: []types.Type{e}}
	opCall.SetType(e.Nullable())
	bin := &syntax.Binary{X: id, Y: &syntax.Ident{Name: "y", Sym: shared}, OpType: e, Call: opCall}
	bin.SetType(types.VarType(other))
	outer := &syntax.Lambda{}
	outer.SetType(e) // around the subtree: not written
	jump := &syntax.Jump{Kind: syntax.RETURN, X: bin, Lambda: outer}
	decl := &syntax.VarStmt{Name: "x", Sym: local, Init: jump}
	body := &syntax.Block{Stmts: []syntax.Stmt{decl}, T: e}
	l := &syntax.Lambda{Body: body, Ret: e}
	outer.Body = &syntax.Block{Stmts: []syntax.Stmt{&syntax.ExprStmt{X: l}}}

	if left := pclaLeft(l); len(left) == 0 {
		t.Fatal("no variable found before writing")
	}
	pclaWrite(l, types.Subst{v: types.Arg{T: types.StringType}})

	str := types.StringType
	for _, k := range []struct {
		what      string
		got, want types.Type
	}{
		{"node type", id.Type(), listOf(str)},
		{"local's type", local.Type, listOf(str)},
		{"block type", body.T, str},
		{"lambda result", l.Ret, str},
		{"operand type", bin.OpType, str},
		{"operator call type", opCall.Type(), str.Nullable()},
		{"reified argument", opCall.Reified[0], str},
		{"function result", fun.Ret, str.Nullable()},
		{"function parameter", fun.Params[0].Type, str},
		{"another variable", bin.Type(), types.VarType(other)},
		{"the lambda around", outer.Type(), e},
		{"a symbol without variables", shared.Type, types.IntType},
	} {
		if k.got != k.want {
			t.Errorf("%s: %v, want %v", k.what, k.got, k.want)
		}
	}
	if left := pclaLeft(l); len(left) != 1 || left[0] != types.VarType(other) {
		t.Errorf("variables left: %v, want only T", left)
	}
}

// TestPCLARefs checks that the fields the writer doesn't follow exist.
func TestPCLARefs(t *testing.T) {
	j := pclaTypeFields(syntax.Jump{})
	for name := range pclaRefs {
		if !j[name] {
			t.Errorf("pclaRefs names %s, which isn't a field", name)
		}
	}
}

// pclaTypeFields returns the fields of node, a node's struct, as
// pclaRefs names them.
func pclaTypeFields(node any) map[string]bool {
	t := reflect.TypeOf(node)
	fs := map[string]bool{}
	for i := range t.NumField() {
		fs[t.Name()+"."+t.Field(i).Name] = true
	}
	return fs
}
