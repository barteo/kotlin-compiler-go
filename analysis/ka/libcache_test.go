package ka

import (
	"context"
	"github.com/barteo/kotlin-compiler-go/internal/kasession"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const sharedSrc = `package p

import java.util.ArrayList

abstract class Repo<T : Comparable<T>>(val items: MutableList<T> = ArrayList()) : Iterable<T> {
    override fun iterator() = items.iterator()
    fun sorted() = items.sorted()
    fun firstOrNull() = items.firstOrNull()
    val size get() = items.size
}

class Names : Repo<String>() {
    fun upper() = map { it.uppercase() }
    fun index() = associateBy { it.length }
    fun both(x: CharSequence) = if (x.isEmpty()) items else listOf(x)
}

fun <K, V : Any> Map<K, V?>.compact() = filterValues { it != null }
val pairs = mapOf(1 to "a", 2 to null).compact()
`

func dumpModel(m *Model) string {
	var sb strings.Builder
	for _, f := range m.Files {
		dump(&sb, f.Snapshot, f.Decls, "")
		dumpTypes(&sb, f.Snapshot, f.Decls, "")
	}
	for _, d := range m.Diagnostics {
		sb.WriteString(d.Message + "\n")
	}
	return sb.String()
}

func updateSession(t *testing.T, s *Session, src string) string {
	t.Helper()
	m, err := s.Update(context.Background(), []Source{{Path: "f.kt", Text: &src}})
	if err != nil {
		t.Error(err)
		return ""
	}
	defer m.Close()
	return dumpModel(m)
}

// TestSharedLibrary checks that updates of a session, and sessions with the
// same configuration, share one library, and that what they find is the
// same as with a library of their own, one after another or at once.
func TestSharedLibrary(t *testing.T) {
	cfg := withJDK(t)
	s1 := Open(cfg)
	defer s1.Close()
	want := updateSession(t, s1, sharedSrc)
	if !strings.Contains(want, "upper") || strings.Contains(want, "<error>") {
		t.Fatalf("unexpected model:\n%s", want)
	}
	if got := updateSession(t, s1, sharedSrc); got != want {
		t.Errorf("second update differs:\n%s\nwant:\n%s", got, want)
	}
	var wg sync.WaitGroup
	got := make([]string, 4)
	sessions := make([]*Session, len(got))
	for i := range got {
		sessions[i] = Open(cfg)
		wg.Go(func() { got[i] = updateSession(t, sessions[i], sharedSrc) })
	}
	wg.Wait()
	for i, g := range got {
		if g != want {
			t.Errorf("session %d differs:\n%s\nwant:\n%s", i, g, want)
		}
		if sessions[i].lib != s1.lib {
			t.Errorf("session %d has a library of its own", i)
		}
		sessions[i].Close()
	}
	// Edits to the program aren't seen through the shared library.
	edited := strings.ReplaceAll(sharedSrc, "Repo<String>", "Repo<Int>")
	edited = strings.ReplaceAll(edited, "it.uppercase()", "it + 1")
	edited = strings.ReplaceAll(edited, "it.length", "it")
	if got := updateSession(t, s1, edited); !strings.Contains(got, "p/Repo<kotlin/Int>") || strings.Contains(got, "p/Repo<kotlin/String>") {
		t.Errorf("edited update:\n%s", got)
	}
	if got := updateSession(t, s1, sharedSrc); got != want {
		t.Errorf("update after the edit differs:\n%s\nwant:\n%s", got, want)
	}
}

// TestLibraryKey checks that a changed jar gets a library of its own, and
// that unused libraries are closed past maxIdle.
func TestLibraryKey(t *testing.T) {
	cfg := withStdlib(t)
	jar := filepath.Join(t.TempDir(), "stdlib.jar")
	copyFile(t, cfg.ClassPath[0], jar)
	cfg.ClassPath = []string{jar}
	src := "package p\nfun f() = listOf(1).first()\n"
	s1 := Open(cfg)
	want := updateSession(t, s1, src)
	s2 := Open(cfg)
	updateSession(t, s2, src)
	if s1.lib != s2.lib {
		t.Error("the same jar has two libraries")
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(jar, later, later); err != nil {
		t.Fatal(err)
	}
	s3 := Open(cfg)
	if got := updateSession(t, s3, src); got != want {
		t.Errorf("changed jar's model differs:\n%s\nwant:\n%s", got, want)
	}
	if s3.lib == s1.lib {
		t.Error("a changed jar has the old library")
	}
	old := s1.lib
	s1.Close()
	s2.Close()
	s3.Close()
	// Fill the idle libraries with others: the oldest is closed.
	for i := range kasession.MaxIdle {
		cfg := cfg
		cfg.ModuleName = "m" + string(rune('a'+i))
		s := Open(cfg)
		updateSession(t, s, src)
		s.Close()
	}
	list, refs := kasession.Listed()
	idle := 0
	for i, l := range list {
		if l == old.l {
			t.Error("the least recently used library is still open")
		}
		if refs[i] == 0 {
			idle++
		}
	}
	if idle > kasession.MaxIdle {
		t.Errorf("%d idle libraries, want at most %d", idle, kasession.MaxIdle)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	r, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	w, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(w, r); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
