package types

import "testing"

// TestMangle checks hashes against the names kotlinc 2.4 gives functions
// that take and return value classes.
func TestMangle(t *testing.T) {
	code := &ClassSym{Name: "Code", Value: true}
	code.Underlying = &VarSym{Name: "code", Type: IntType}
	name := &ClassSym{Name: "a/Name", Value: true}
	name.Underlying = &VarSym{Name: "name", Type: StringType}
	ct, nt := ClassType(code), ClassType(name)
	tests := []struct {
		params []Type
		ret    *Type
		want   string
	}{
		{[]Type{ct}, nil, "ggICe2c"},            // fun take(c: Code)
		{[]Type{ct.Nullable()}, nil, "NpuKPYo"}, // fun take(c: Code?)
		{nil, &ct, "d5wHzb0"},                   // a member returning Code
		{[]Type{ct}, &ct, "1wNqfbg"},            // fun plus(other: Code): Code, in Code
		{[]Type{ct, ClassType(&ClassSym{Name: "Name", Value: true, Underlying: name.Underlying}), IntType}, nil, "8aZSt2s"},
		// Compose's MeasureScope.measure(measurables, constraints) of
		// MeasurePolicy, whose metadata leaves the name out.
		{[]Type{AnyType, AnyType, ClassType(&ClassSym{Name: "androidx/compose/ui/unit/Constraints", Value: true,
			Underlying: &VarSym{Name: "value", Type: LongType}})}, nil, "3p2s80s"},
	}
	for _, tt := range tests {
		if got := Mangle(tt.params, tt.ret); got != tt.want {
			t.Errorf("Mangle(%v, %v) = %s, want %s", tt.params, tt.ret, got, tt.want)
		}
	}
	if !Unboxed(nt.Nullable()) || Unboxed(ct.Nullable()) {
		t.Errorf("a nullable value class is unboxed only over a non-null reference")
	}
	if d, _ := Desc(ct); d != "I" {
		t.Errorf("Desc(Code) = %s", d)
	}
	if d, _ := Desc(ct.Nullable()); d != "LCode;" {
		t.Errorf("Desc(Code?) = %s", d)
	}
	if d, _ := Desc(nt.Nullable()); d != "Ljava/lang/String;" {
		t.Errorf("Desc(Name?) = %s", d)
	}
}
