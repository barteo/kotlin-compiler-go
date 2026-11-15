package javasrc

import "testing"

// TestUnresolvedAsKotlin checks the names kotlinc can't resolve in Java
// sources: member types inherited from two supertypes or named by two
// star imports, and classes in files named after other classes.
func TestUnresolvedAsKotlin(t *testing.T) {
	srcs := map[string]string{
		"a/X.java":    "package a; public class X { public class Z {} public static class S {} }",
		"a/I.java":    "package a; public interface I { class Z {} }",
		"a/J.java":    "package a; public interface J extends I {}",
		"a/K.java":    "package a; public interface K extends I {}",
		"a/Y.java":    "package a; public class Y extends X implements J { public Z z() { return null; } public S s() { return null; } }",
		"a/D.java":    "package a; public class D implements J, K { public Z z() { return null; } }",
		"a/P.java":    "package a; public class P { public static class S {} }",
		"b/Star.java": "package b; import a.X.*; import a.P.*; public class Star { public S s() { return null; } public Z z() { return null; } }",
		"b/One.java":  "package b; public class One { public Two two() { return null; } } class Two {}",
	}
	var files []*File
	for path, src := range srcs {
		f, err := Parse(path, src)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	s := NewSet(files)
	if s.Has("b/Two") {
		t.Errorf("b/Two, in One.java, is a class Kotlin sees")
	}
	want := map[string]string{
		"a/Y.z":     "()La/Z;",   // X.Z or I.Z (through J)
		"a/Y.s":     "()La/X$S;", // only X has S
		"a/D.z":     "()La/I$Z;", // I.Z through both J and K
		"b/Star.s":  "()Lb/S;",   // X.S or P.S
		"b/Star.z":  "()La/X$Z;", // only X has Z
		"b/One.two": "()Lb/Two;",
	}
	for key, desc := range want {
		class, method := key[:len(key)-len(after(key))-1], after(key)
		info, err := s.Info(class)
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		for _, m := range info.Methods {
			if m.Name == method {
				got = m.Desc
			}
		}
		if got != desc {
			t.Errorf("%s: %s, want %s", key, got, desc)
		}
	}
}

func after(key string) string {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '.' {
			return key[i+1:]
		}
	}
	return key
}
