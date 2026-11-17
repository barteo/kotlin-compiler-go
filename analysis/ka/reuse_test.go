package ka

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReuse checks that an update of the same texts reuses the last
// check's model, that one with a changed text checks again and reparses
// only that file, and that the models are what fresh checks give.
func TestReuse(t *testing.T) {
	cfg := withJDK(t)
	a := "package p\n\nclass A { fun b() = B().name }\n"
	b := "package p\n\nclass B { val name = \"b\" }\n"
	b2 := "package p\n\nclass B { val name = 1 }\n"
	update := func(s *Session, a, b string) *Model {
		t.Helper()
		m, err := s.Update(context.Background(), []Source{{Path: "a.kt", Text: &a}, {Path: "b.kt", Text: &b}})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	fresh := func(a, b string) string {
		// A configuration of its own: a library with no kept checks.
		c := cfg
		c.ModuleName = "fresh" + b
		s := Open(c)
		defer s.Close()
		m := update(s, a, b)
		defer m.Close()
		return dumpModel(m)
	}
	s := Open(cfg)
	defer s.Close()
	m1 := update(s, a, b)
	defer m1.Close()
	m2 := update(s, a, b)
	defer m2.Close()
	if m1.Files[0] != m2.Files[0] {
		t.Error("an update of the same texts didn't reuse the model")
	}
	// Another session of the same configuration reuses it too.
	s2 := Open(cfg)
	defer s2.Close()
	m3 := update(s2, a, b)
	defer m3.Close()
	if m3.Files[0] != m1.Files[0] {
		t.Error("a session of the same configuration didn't reuse the model")
	}
	if got, want := dumpModel(m1), fresh(a, b); got != want {
		t.Errorf("model:\n%s\nwant:\n%s", got, want)
	}
	m4 := update(s, a, b2)
	defer m4.Close()
	if m4.Files[0] == m1.Files[0] {
		t.Error("an update of changed texts reused the model")
	}
	if m4.Files[0].Snapshot != m1.Files[0].Snapshot {
		t.Error("the unchanged file was parsed again")
	}
	if m4.Files[1].Snapshot == m1.Files[1].Snapshot {
		t.Error("the changed file wasn't parsed again")
	}
	if got, want := dumpModel(m4), fresh(a, b2); got != want {
		t.Errorf("model after the change:\n%s\nwant:\n%s", got, want)
	}
	// The inferred type of A.b follows B.name's.
	if got := dumpModel(m4); got == dumpModel(m1) {
		t.Errorf("the change didn't change the model:\n%s", got)
	}
}

// TestReuseJava checks that an update with Java sources reuses the last
// such check, with its library, and that a changed Java text doesn't.
func TestReuseJava(t *testing.T) {
	cfg := withJDK(t)
	cfg.ModuleName = "reuseJava"
	dir := t.TempDir()
	java := filepath.Join(dir, "J.java")
	write := func(text string) {
		if err := os.WriteFile(java, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	k := "package p\n\nclass K { fun j() = q.J().n }\n"
	update := func() *Model {
		t.Helper()
		s := Open(cfg)
		defer s.Close()
		m, err := s.Update(context.Background(), []Source{{Path: "k.kt", Text: &k}, {Path: java}})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	write("package q;\npublic class J { public int n; }\n")
	m1 := update()
	m2 := update()
	if m1.Files[0] != m2.Files[0] {
		t.Error("an update of the same texts didn't reuse the model")
	}
	m1.Close()
	// The reused model's library still has the Java class.
	if d := m2.Class("q/J"); d == nil || len(d.Members) == 0 {
		t.Errorf("Class(q/J) = %v", d)
	}
	m2.Close()
	write("package q;\npublic class J { public String n; }\n")
	m3 := update()
	defer m3.Close()
	if m3.Files[0] == m2.Files[0] {
		t.Error("an update of a changed Java text reused the model")
	}
	if got := dumpModel(m3); !strings.Contains(got, "kotlin/String") {
		t.Errorf("model after the change:\n%s", got)
	}
}

// Modules with Java sources share the class path's library: each check
// puts its sources over it, which the other checks don't see.
func TestSharedLibraryJava(t *testing.T) {
	cfg := withJDK(t)
	cfg.ModuleName = "sharedJava"
	k := "package p\n\nclass K { fun s(): String = \"\" }\n"
	update := func(path, java string) *Model {
		t.Helper()
		s := Open(cfg)
		defer s.Close()
		m, err := s.Update(context.Background(), []Source{{Path: "k.kt", Text: &k}, {Path: path, Text: &java}})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	m1 := update("J.java", "package q;\npublic class J { public int n; public p.K k; }\n")
	defer m1.Close()
	m2 := update("J2.java", "package q;\npublic class J2 { public String s; }\n")
	defer m2.Close()
	if m1.shared != m2.shared {
		t.Fatal("the checks don't share the class path's library")
	}
	if m1.shared.lib.cp.HasSources() {
		t.Error("the shared class path has the Java sources")
	}
	if m1.Class("q/J") == nil || m1.Class("q/J2") != nil {
		t.Errorf("first check: Class(q/J) = %v, Class(q/J2) = %v", m1.Class("q/J"), m1.Class("q/J2"))
	}
	if m2.Class("q/J2") == nil || m2.Class("q/J") != nil {
		t.Errorf("second check: Class(q/J2) = %v, Class(q/J) = %v", m2.Class("q/J2"), m2.Class("q/J"))
	}
	if a, b := m1.Class("java/lang/Thread"), m2.Class("java/lang/Thread"); a == nil || a != b {
		t.Error("the checks don't share the class path's models")
	}
}
