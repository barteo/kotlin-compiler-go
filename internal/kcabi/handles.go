package kcabi

import (
	"fmt"
	"sync"
)

// A Handle refers to a Go object from the JVM. Zero is never a valid handle,
// so the JVM side can use it for "none".
type Handle int64

// Handles maps handles to objects. The zero value is ready to use, and it is
// safe for concurrent use.
//
// Handles are owned by a scope (a session's analysis of one round, say):
// the scope releases all of its handles at once when it ends, so the JVM
// never has to free them one by one.
type Handles struct {
	mu   sync.RWMutex
	next Handle
	objs map[Handle]any
	ids  map[any]Handle // so that one object always gets the same handle
}

// Put returns the handle of x, adding x if it has none. x must be
// comparable (a pointer, typically).
func (t *Handles) Put(x any) Handle {
	t.mu.RLock()
	h, ok := t.ids[x]
	t.mu.RUnlock()
	if ok {
		return h
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if h, ok := t.ids[x]; ok {
		return h
	}
	if t.objs == nil {
		t.objs = map[Handle]any{}
		t.ids = map[any]Handle{}
	}
	t.next++
	t.objs[t.next] = x
	t.ids[x] = t.next
	return t.next
}

// Get returns the object of h.
func (t *Handles) Get(h Handle) (any, error) {
	t.mu.RLock()
	x, ok := t.objs[h]
	t.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid handle %d", h)
	}
	return x, nil
}

// Delete removes h.
func (t *Handles) Delete(h Handle) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if x, ok := t.objs[h]; ok {
		delete(t.objs, h)
		delete(t.ids, x)
	}
}

// Len returns the number of live handles.
func (t *Handles) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.objs)
}

// Get returns the object of h as a T.
func Get[T any](t *Handles, h Handle) (T, error) {
	var zero T
	x, err := t.Get(h)
	if err != nil {
		return zero, err
	}
	v, ok := x.(T)
	if !ok {
		return zero, fmt.Errorf("handle %d is a %T, not a %T", h, x, zero)
	}
	return v, nil
}
