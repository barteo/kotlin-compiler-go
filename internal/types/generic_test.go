package types

import (
	"sync"
	"testing"
)

func TestArgsInterned(t *testing.T) {
	box := &ClassSym{Name: "Box"}
	var wg sync.WaitGroup
	got := make([]Type, 8)
	for i := range got {
		wg.Go(func() {
			got[i] = GenericType(box, []Arg{{T: IntType}, {V: Out, T: StringType.Nullable()}})
		})
	}
	wg.Wait()
	for _, g := range got {
		if g != got[0] {
			t.Fatalf("types with equal arguments differ: %v, %v", g, got[0])
		}
	}
	if s := got[0].String(); s != "Box<Int, out String?>" {
		t.Errorf("String() = %q", s)
	}
	if GenericType(box, []Arg{{T: LongType}}) == GenericType(box, []Arg{{T: IntType}}) {
		t.Error("types with different arguments are equal")
	}
	if MakeArgs(nil) != nil {
		t.Error("empty argument list is not nil")
	}
}

func TestSubst(t *testing.T) {
	box := &ClassSym{Name: "Box"}
	tp := &TypeParamSym{Name: "T"}
	nn := &TypeParamSym{Name: "U", Bounds: []Type{AnyType}}
	T := ParamType(tp)
	inv := Subst{tp: {T: IntType}}
	out := Subst{tp: {V: Out, T: StringType}}
	star := Subst{tp: {V: Star}}
	tests := []struct {
		name string
		s    Subst
		in   bool
		t    Type
		want Type
	}{
		{"param", inv, false, T, IntType},
		{"nullable param", inv, false, T.Nullable(), IntType.Nullable()},
		{"platform param", inv, false, T.Platform(), Type{K: Int, N: true, P: true}},
		{"nested", inv, false, GenericType(box, []Arg{{T: T}}), GenericType(box, []Arg{{T: IntType}})},
		{"out read", out, false, T, StringType},
		{"out write", out, true, T, NothingType},
		{"out nested", out, false, GenericType(box, []Arg{{T: T}}), GenericType(box, []Arg{{V: Out, T: StringType}})},
		{"star read", star, false, T, AnyType.Nullable()},
		{"star nested", star, false, GenericType(box, []Arg{{T: T}}), GenericType(box, []Arg{{V: Star}})},
		{"definitely non-null", Subst{tp: {T: StringType.Nullable()}}, false, T.NonNull(), StringType},
	}
	for _, tt := range tests {
		got := tt.s.Apply(tt.t)
		if tt.in {
			got = tt.s.ApplyIn(tt.t)
		}
		if got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
	if !MayBeNull(T) || MayBeNull(T.NonNull()) || MayBeNull(ParamType(nn)) || !MayBeNull(ParamType(nn).Nullable()) {
		t.Error("MayBeNull is wrong for type parameters")
	}
	if ParamType(nn).NonNull() != ParamType(nn) {
		t.Error("NonNull of a non-null parameter is not canonical")
	}
	if Erase(T) != AnyType.Nullable() || Erase(GenericType(box, []Arg{{T: T}})) != ClassType(box) {
		t.Error("Erase")
	}
}
