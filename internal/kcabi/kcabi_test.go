package kcabi

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestCall(t *testing.T) {
	got, err := Call(OpVersion, nil)
	if err != nil || string(got) != Version {
		t.Fatalf("OpVersion = %q, %v", got, err)
	}
	got, err = Call(OpEcho, []byte("abc"))
	if err != nil || string(got) != "abc" {
		t.Fatalf("OpEcho = %q, %v", got, err)
	}
	_, err = Call(9999, nil)
	var e *Error
	if !errors.As(err, &e) || e.Panic || e.Msg != "unknown operation" {
		t.Fatalf("unknown op: %v", err)
	}
}

func TestCallRecoversPanics(t *testing.T) {
	_, err := Call(OpPanic, []byte("boom"))
	var e *Error
	if !errors.As(err, &e) || !e.Panic || e.Msg != "boom" || !strings.Contains(e.Stack, "kcabi") {
		t.Fatalf("OpPanic: %#v", err)
	}
}

func TestHandles(t *testing.T) {
	var hs Handles
	a, b := new(int), new(string)
	ha, hb := hs.Put(a), hs.Put(b)
	if ha == 0 || hb == 0 || ha == hb || hs.Put(a) != ha {
		t.Fatalf("handles %d %d %d", ha, hb, hs.Put(a))
	}
	if x, err := Get[*int](&hs, ha); err != nil || x != a {
		t.Fatalf("Get = %v, %v", x, err)
	}
	if _, err := Get[*int](&hs, hb); err == nil {
		t.Fatal("Get with the wrong type succeeded")
	}
	hs.Delete(ha)
	if _, err := hs.Get(ha); err == nil || hs.Len() != 1 {
		t.Fatalf("after Delete: %v, %d handles", err, hs.Len())
	}
}

func TestHandlesConcurrent(t *testing.T) {
	var hs Handles
	objs := make([]*int, 100)
	for i := range objs {
		objs[i] = new(int)
	}
	var wg sync.WaitGroup
	got := make([][]Handle, 8)
	for g := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, o := range objs {
				got[g] = append(got[g], hs.Put(o))
			}
		}()
	}
	wg.Wait()
	for g := range got {
		for i := range objs {
			if got[g][i] != got[0][i] {
				t.Fatalf("object %d got handles %d and %d", i, got[0][i], got[g][i])
			}
		}
	}
}
