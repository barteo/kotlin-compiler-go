package ka

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// A read cache reads a file again whenever its stamp may not say it is
// unchanged, and only then.
func TestReadCache(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("no stamps here")
	}
	defer func(w time.Duration) { racyWindow = w }(racyWindow)
	racyWindow = 50 * time.Millisecond
	settle := func() { time.Sleep(2 * racyWindow) }
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.kt"), filepath.Join(dir, "b.kt")
	old := time.Now().Add(-time.Hour)
	write := func(path, text string, mtime time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if !mtime.IsZero() {
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	var c readCache
	srcs := []Source{{Path: a}, {Path: b}}
	check := func(name string, want []string, reads int64) {
		t.Helper()
		filesRead.Store(0)
		texts, errs := c.texts(srcs, 2)
		for i := range srcs {
			if errs[i] != nil {
				t.Fatalf("%s: %v", name, errs[i])
			}
			if texts[i] != want[i] {
				t.Errorf("%s: %s is %q, want %q", name, srcs[i].Path, texts[i], want[i])
			}
		}
		if n := filesRead.Load(); n != reads {
			t.Errorf("%s: read %d files, want %d", name, n, reads)
		}
	}
	write(a, "package a", old)
	write(b, "package b", time.Time{})
	check("first", []string{"package a", "package b"}, 2)
	check("racy", []string{"package a", "package b"}, 2) // both changed just now
	settle()
	check("settled", []string{"package a", "package b"}, 2) // read last within the window
	check("unchanged", []string{"package a", "package b"}, 0)
	// Same size, same mtime: the change time moves.
	write(a, "package x", old)
	check("same size and mtime", []string{"package x", "package b"}, 1)
	settle()
	check("settled again", []string{"package x", "package b"}, 1)
	check("unchanged again", []string{"package x", "package b"}, 0)
	// Renamed over, with the old mtime: the inode moves.
	tmp := filepath.Join(dir, "tmp")
	write(tmp, "package y", old)
	if err := os.Rename(tmp, a); err != nil {
		t.Fatal(err)
	}
	check("renamed over", []string{"package y", "package b"}, 1)
	// A file that is gone is an error, and a Text is used as given.
	text := "package z"
	srcs = []Source{{Path: a, Text: &text}, {Path: filepath.Join(dir, "gone.kt")}}
	filesRead.Store(0)
	texts, errs := c.texts(srcs, 2)
	if texts[0] != text || errs[0] != nil || errs[1] == nil {
		t.Errorf("text %q, errors %v", texts[0], errs)
	}
}

// A daemon's runs each open a session: the next one's update reads only
// the files that changed since the last one's.
func TestReadCacheSessions(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("no stamps here")
	}
	cfg := withStdlib(t)
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	var srcs []Source
	for i, text := range []string{"package p\nclass A", "package p\nclass B(val a: A)"} {
		path := filepath.Join(dir, fmt.Sprintf("f%d.kt", i))
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		srcs = append(srcs, Source{Path: path})
	}
	time.Sleep(10 * time.Millisecond)
	defer func(w time.Duration) { racyWindow = w }(racyWindow)
	racyWindow = time.Millisecond
	update := func() *Model {
		t.Helper()
		s := Open(cfg)
		defer s.Close()
		m, err := s.Update(context.Background(), srcs)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	update().Close()
	filesRead.Store(0)
	m := update()
	defer m.Close()
	if n := filesRead.Load(); n != 0 {
		t.Errorf("the second session read %d files, want 0", n)
	}
	if len(m.Files) != 2 || len(m.Files[1].Decls) != 1 || m.Files[1].Decls[0].ClassID != "p/B" {
		t.Errorf("model: %d files", len(m.Files))
	}
}
