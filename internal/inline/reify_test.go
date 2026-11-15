package inline

import (
	"testing"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// markedOps returns the reified operation markers' names in a class's
// code, and the classes its instanceof instructions take.
func markedOps(t *testing.T, data []byte) (markers, instanceofs []string) {
	t.Helper()
	methods, err := cf.ReadMethods(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range methods {
		insns, err := m.Decode()
		if err != nil {
			t.Fatal(err)
		}
		for i, in := range insns {
			switch {
			case in.Op == cf.INVOKESTATIC && in.Name == "reifiedOperationMarker":
				markers = append(markers, insns[i-1].Str)
			case in.Op == cf.INSTANCEOF:
				instanceofs = append(instanceofs, in.Class)
			}
		}
	}
	return markers, instanceofs
}

// TestInstantiateKeepsParams copies filterIsInstance's { it is R } into a
// function with reified type parameter S, as kc does in the function's
// method: the copy marks the operation on S, and a copy of the copy with
// a type argument for S is reified.
func TestInstantiateKeepsParams(t *testing.T) {
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open("", []string{jar})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	const name = "kotlin/sequences/SequencesKt___SequencesKt$filterIsInstance$1"
	tmpl, err := loadTemplate(name, cp.Bytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := tmpl.Params(); len(got) != 1 || got[0] != "R" {
		t.Fatalf("params %v, want [R]", got)
	}
	for _, tc := range []struct{ cls, marker string }{{"?S", "S"}, {"?S?", "S?"}} {
		classes := map[string]string{"R": tc.cls}
		if !tmpl.Reifiable(classes) {
			t.Fatalf("%s: not reifiable", tc.cls)
		}
		copies, err := tmpl.Instantiate("a/AKt$only$$inlined$filterIsInstance$1", classes, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(copies) != 1 {
			t.Fatalf("%s: %d copies, want 1", tc.cls, len(copies))
		}
		markers, insts := markedOps(t, copies[0].Data)
		if len(markers) != 1 || markers[0] != tc.marker || len(insts) != 1 || insts[0] != "java/lang/Object" {
			t.Fatalf("%s: markers %v, instanceof %v; want [%s], [java/lang/Object]", tc.cls, markers, insts, tc.marker)
		}
		// A copier's copy of the copy.
		again, _, err := readTemplate(copies[0].Name, copies[0].Data)
		if err != nil {
			t.Fatal(err)
		}
		if got := again.Params(); len(got) != 1 || got[0] != "S" {
			t.Fatalf("%s: copy's params %v, want [S]", tc.cls, got)
		}
		final, err := again.Instantiate("b/BKt$main$$inlined$only$1", map[string]string{"S": "java/lang/String"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		markers, insts = markedOps(t, final[0].Data)
		if len(markers) != 0 || len(insts) != 1 || insts[0] != "java/lang/String" {
			t.Fatalf("%s: final markers %v, instanceof %v; want none, [java/lang/String]", tc.cls, markers, insts)
		}
	}
	// A concrete type argument reifies the template, as before.
	if tmpl.Reifiable(map[string]string{"R": "java/lang/String"}) {
		t.Error("reifiable with a class")
	}
	copies, err := tmpl.Instantiate("X", map[string]string{"R": "java/lang/String"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if markers, insts := markedOps(t, copies[0].Data); len(markers) != 0 || len(insts) != 1 || insts[0] != "java/lang/String" {
		t.Fatalf("markers %v, instanceof %v", markers, insts)
	}
}
