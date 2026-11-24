package classfile

import "testing"

func TestTargetMajor(t *testing.T) {
	for target, want := range map[string]uint16{"": 52, "1.8": 52, "9": 53, "11": 55, "17": 61, "25": 69} {
		if got, ok := TargetMajor(target); !ok || got != want {
			t.Errorf("TargetMajor(%q) = %d, %v, want %d", target, got, ok, want)
		}
	}
	for _, target := range []string{"8", "1.7", "26", "011", "17.0", "x"} {
		if _, ok := TargetMajor(target); ok {
			t.Errorf("TargetMajor(%q) accepted", target)
		}
	}
	for major, want := range map[int]string{52: "1.8", 55: "11", 65: "21"} {
		if got := TargetName(major); got != want {
			t.Errorf("TargetName(%d) = %q, want %q", major, got, want)
		}
	}
}

func TestSetMajor(t *testing.T) {
	data := NewClass(AccPublic|AccSuper, "C", "java/lang/Object").Bytes()
	if Major(data) != 52 {
		t.Fatalf("written version %d, want 52", Major(data))
	}
	out := SetMajor(data, 61)
	if Major(out) != 61 || Major(data) != 52 {
		t.Errorf("SetMajor: got %d, input now %d; want 61, input unchanged", Major(out), Major(data))
	}
	// A class file of another version, as a library's copied one, keeps it.
	if Major(SetMajor(out, 55)) != 61 {
		t.Errorf("SetMajor changed a version-61 class file")
	}
	if Major([]byte{1, 2}) != 0 {
		t.Errorf("Major of a short slice isn't 0")
	}
}

func TestLaterThanAPI(t *testing.T) {
	for v, want := range map[string]bool{"2.4": false, "2.3": false, "1.9.20": false, "2.4.0": false, "02.4": false,
		"2.4.1": true, "2.5": true, "2.10": true, "3.0": true} {
		if got := LaterThanAPI(v); got != want {
			t.Errorf("LaterThanAPI(%q) = %v, want %v", v, got, want)
		}
	}
}
