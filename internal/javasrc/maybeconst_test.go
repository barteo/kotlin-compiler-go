package javasrc

import "testing"

type kotlinOutside struct{ kotlin map[string]bool }

func (k kotlinOutside) Has(name string) bool                  { return k.kotlin[name] }
func (k kotlinOutside) Supers(string) ([]string, bool)        { return nil, false }
func (k kotlinOutside) Constant(string, string) (any, string) { return nil, "" }
func (k kotlinOutside) IsKotlin(name string) bool             { return k.kotlin[name] }

// TestMaybeConstant checks that a field whose initializer reads a Kotlin
// class's field, directly or through another field, is marked as maybe
// constant, and one that isn't constant otherwise is not.
func TestMaybeConstant(t *testing.T) {
	src := `package p;
public class C {
    public static final int K = MainKt.FOO + 1, KK = K * 2;
    public static final long T = System.currentTimeMillis();
    public static final int U = UNKNOWN;
    public static final int ONE = 1;
}
`
	f, err := Parse("C.java", src)
	if err != nil {
		t.Fatal(err)
	}
	s := NewSet([]*File{f})
	s.Outside = kotlinOutside{map[string]bool{"p/MainKt": true}}
	info, err := s.Info("p/C")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"K": true, "KK": true, "T": false, "U": false, "ONE": false}
	for _, fd := range info.Fields {
		if fd.MaybeConstant != want[fd.Name] {
			t.Errorf("%s: MaybeConstant = %v", fd.Name, fd.MaybeConstant)
		}
		if fd.Name == "ONE" && fd.Constant == nil {
			t.Errorf("ONE is not a constant")
		}
	}
}
