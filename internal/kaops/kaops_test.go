package kaops

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestOpenUpdateClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.kt")
	if err := os.WriteFile(path, []byte("package p\nannotation class A\n@A class C\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	var w wire.Writer
	w.String("")
	w.Bool(true) // no JDK
	w.String("m")
	w.Uint(1)
	w.Uint(0) // no class path
	resp, err := kcabi.Call(OpKaOpen, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	session := wire.NewReader(resp).Int()

	w = wire.Writer{}
	w.Int(session)
	w.Uint(1)
	w.String(path)
	resp, err = kcabi.Call(OpKaUpdate, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	r := wire.NewReader(resp)
	model := r.Int()
	if n := r.Uint(); n != 1 {
		t.Fatalf("%d files", n)
	}
	if p, pkg := r.String(), r.String(); p != path || pkg != "p" {
		t.Fatalf("file %s, package %s", p, pkg)
	}
	if n := r.Uint(); n != 0 {
		t.Fatalf("%d file annotations", n)
	}
	if n := r.Uint(); n != 2 {
		t.Fatalf("%d declarations", n)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}

	for _, h := range []int64{model, session} {
		w = wire.Writer{}
		w.Int(h)
		if _, err := kcabi.Call(OpKaClose, w.Message()); err != nil {
			t.Fatal(err)
		}
	}
	if handles.Len() != 0 {
		t.Errorf("%d handles left", handles.Len())
	}
	w = wire.Writer{}
	w.Int(session)
	w.Uint(0)
	if _, err := kcabi.Call(OpKaUpdate, w.Message()); err == nil {
		t.Error("update of a closed session succeeded")
	}
}

// readValue reads a value as jvm/kc-aa's KcModel does.
func readValue(r *wire.Reader) *ka.Value {
	kind := ka.ValueKind(r.Uint())
	if kind == 0 {
		return nil
	}
	v := &ka.Value{Kind: kind, Node: psi.NodeID(r.Uint())}
	switch kind {
	case ka.ValueConst:
		v.Const = ka.ConstKind(r.Uint())
		switch v.Const {
		case ka.ConstNull:
		case ka.ConstFloat, ka.ConstDouble:
			v.Float = math.Float64frombits(r.Uint())
		case ka.ConstString:
			v.Str = r.String()
		default:
			v.Bits = r.Int()
		}
	case ka.ValueEnum:
		v.ClassID, v.Entry = r.String(), r.String()
	case ka.ValueClass:
		if r.Uint() != uint64(ka.TypeClass) {
			r.Int() // fail the test below
		}
		v.Type = &ka.Type{Kind: ka.TypeClass, ClassID: r.String()}
		r.Uint()       // flags
		r.Uint()       // param
		_ = r.String() // name
		r.Uint()       // contexts
		r.Uint()       // no arguments
	case ka.ValueAnnotation:
		v.Annotation = readAnnotation(r)
	case ka.ValueArray:
		for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
			v.Elems = append(v.Elems, readValue(r))
		}
	}
	return v
}

func readAnnotation(r *wire.Reader) *ka.Annotation {
	a := &ka.Annotation{ClassID: r.String(), Node: psi.NodeID(r.Uint()), Target: r.String()}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		a.Args = append(a.Args, ka.NamedValue{Name: r.String(), Value: readValue(r)})
	}
	return a
}

func TestValues(t *testing.T) {
	in := &ka.Annotation{ClassID: "p/A", Node: 3, Target: "field", Args: []ka.NamedValue{
		{Name: "s", Value: &ka.Value{Kind: ka.ValueConst, Node: 4, Const: ka.ConstString, Str: "x"}},
		{Name: "i", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstInt, Bits: -4}},
		{Name: "u", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstULong, Bits: -1}},
		{Name: "d", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstDouble, Float: 0.1}},
		{Name: "n", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstNull}},
		{Name: "e", Value: &ka.Value{Kind: ka.ValueEnum, Node: 5, ClassID: "p/E", Entry: "B"}},
		{Name: "k", Value: &ka.Value{Kind: ka.ValueClass, Type: &ka.Type{Kind: ka.TypeClass, ClassID: "kotlin/String"}}},
		{Name: "a", Value: &ka.Value{Kind: ka.ValueArray, Elems: []*ka.Value{
			{Kind: ka.ValueAnnotation, Node: 6, Annotation: &ka.Annotation{ClassID: "p/B", Node: 6}},
			{Kind: ka.ValueUnsupported, Node: 7},
		}}},
	}}
	var w wire.Writer
	writeAnnotation(&w, in)
	writeValue(&w, nil)
	r := wire.NewReader(w.Message())
	out := readAnnotation(r)
	if v := readValue(r); v != nil {
		t.Errorf("none read as %v", v)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("read %s, want %s", out, in)
	}
}
