package classfile_test

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
)

func TestMethodReaderConcurrentAndLazy(t *testing.T) {
	c := cf.NewClass(cf.AccPublic, "demo/A", "java/lang/Object")
	for _, name := range []string{"good", "bad"} {
		m := c.NewMethod(cf.AccPublic|cf.AccStatic, name, "()V")
		m.Op(cf.RETURN)
	}
	data := c.Bytes()
	bad, err := cf.ReadMethod(data, "bad", "()V")
	if err != nil {
		t.Fatal(err)
	}
	// The unrelated body's code length is invalid. Indexing and reading
	// good must not decode it; its own error must be repeatable.
	binary.BigEndian.PutUint32(data[bad.CodeOff-4:], uint32(len(data)))
	r, err := cf.NewMethodReader(data)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			m, err := r.ReadMethod("good", "()V")
			if err != nil || m == nil || !bytes.Equal(m.Code, []byte{cf.RETURN}) {
				t.Errorf("good: %+v, %v", m, err)
			}
			if _, err := r.ReadMethod("bad", "()V"); err == nil {
				t.Error("invalid body accepted")
			}
			if _, err := r.ReadMethod("missing", "()V"); err == nil {
				t.Error("missing method accepted")
			}
		})
	}
	wg.Wait()
}
