package kaops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
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
