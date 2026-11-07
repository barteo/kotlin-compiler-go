// Package kcabi is the language-neutral core of libkc, the shared library
// through which a JVM uses kc's analysis (cmd/libkc, docs/analysis-api.md).
//
// A host makes one kind of call: Call(op, request) returns a response or an
// error. Requests and responses are byte strings in the format of
// package wire. Objects that outlive a call, such as sessions, symbols and
// types, are referred to by handles from a Handles table. Call never panics:
// a panic in an operation comes back as an error, so a bug in kc throws an
// exception in the JVM instead of killing the process hosting it (a Gradle
// daemon, typically).
package kcabi

import (
	"fmt"
	"runtime/debug"
	"sync"
)

// Op names an operation.
type Op uint32

// Operations. The numbers are part of the ABI: append only.
const (
	OpVersion Op = 1 // → string: the ABI version
	OpEcho    Op = 2 // bytes → the same bytes (for tests of the bridge)
	OpPanic   Op = 3 // string → panics with the message (for tests of the bridge)
)

// Version is the ABI version, which the JVM side checks when it loads the
// library. It changes whenever an operation's request or response changes.
const Version = "kc-aa/0"

// handler runs one operation.
type handler func(req []byte) ([]byte, error)

var (
	mu       sync.RWMutex
	handlers = map[Op]handler{
		OpVersion: func([]byte) ([]byte, error) { return []byte(Version), nil },
		OpEcho:    func(req []byte) ([]byte, error) { return req, nil },
		OpPanic:   func(req []byte) ([]byte, error) { panic(string(req)) },
	}
)

// Register adds the handler of an operation. Packages that implement
// operations call it from init functions.
func Register(op Op, h func(req []byte) ([]byte, error)) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := handlers[op]; dup {
		panic(fmt.Sprintf("kcabi: operation %d registered twice", op))
	}
	handlers[op] = h
}

// Error is an error Call returns. Panic tells a panic in kc, which is a bug,
// from an error the request caused.
type Error struct {
	Op    Op
	Msg   string
	Panic bool
	Stack string // the stack of a panic
}

func (e *Error) Error() string {
	if e.Panic {
		return fmt.Sprintf("kc: operation %d panicked: %s\n%s", e.Op, e.Msg, e.Stack)
	}
	return fmt.Sprintf("kc: operation %d: %s", e.Op, e.Msg)
}

// Call runs operation op on req. It is safe for concurrent use.
func Call(op Op, req []byte) (resp []byte, err error) {
	mu.RLock()
	h := handlers[op]
	mu.RUnlock()
	if h == nil {
		return nil, &Error{Op: op, Msg: "unknown operation"}
	}
	defer func() {
		if r := recover(); r != nil {
			resp, err = nil, &Error{Op: op, Msg: fmt.Sprint(r), Panic: true, Stack: string(debug.Stack())}
		}
	}()
	resp, err = h(req)
	if err != nil {
		if _, ok := err.(*Error); !ok {
			err = &Error{Op: op, Msg: err.Error()}
		}
	}
	return resp, err
}
