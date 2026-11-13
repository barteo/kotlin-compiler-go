package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// A kcServer is a running kc -serve, which compiles one test's module
// at a time and keeps the class path and the standard library's classes
// it reads for the next: see cmd/kc/serve.go.
type kcServer struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
	stderr *lockedBuffer // what it writes to stderr: a panic's trace
}

// lockedBuffer is a bytes.Buffer that a process's stderr is copied to
// while it is read.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) take() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.buf.String()
	b.buf.Reset()
	return s
}

// kcPool runs compiles on up to n kc servers, each one at a time, so a
// compile that crashes its server is the only one that does. A server is
// started again after a compile that ended it.
type kcPool struct {
	kc   string
	free chan *kcServer // nil: none started
}

func newKcPool(kc string, n int) *kcPool {
	p := &kcPool{kc: kc, free: make(chan *kcServer, n)}
	for range n {
		p.free <- nil
	}
	return p
}

func (p *kcPool) start() (*kcServer, error) {
	cmd := exec.Command(p.kc, "-serve")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	s := &kcServer{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<16), stderr: &lockedBuffer{}}
	cmd.Stderr = s.stderr
	// Once it is stopped, don't wait for what has its stderr open.
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return s, nil
}

// stop ends the server and returns what it wrote to stderr since its
// last compile, and how it exited.
func (s *kcServer) stop() string {
	s.in.Close()
	s.cmd.Process.Kill()
	err := s.cmd.Wait()
	out := s.stderr.take()
	if err != nil && strings.TrimSpace(out) == "" {
		out = "kc -serve: " + err.Error()
	}
	return out
}

// compile runs kc with args on a free server, and returns its exit code
// and what it wrote to stderr. If the compile ended the server, as a
// panic does, the code is -1 and the output is what the server wrote
// before it ended: the panic's trace. timedOut reports that it ran for
// longer than timeout, after which its server was stopped.
func (p *kcPool) compile(args []string, timeout time.Duration) (code int, out string, timedOut bool) {
	s := <-p.free
	if s == nil {
		var err error
		if s, err = p.start(); err != nil {
			p.free <- nil
			return -1, "kc -serve: " + err.Error(), false
		}
	}
	s.stderr.take()
	type answer struct {
		Code int    `json:"code"`
		Out  string `json:"out"`
		Exit bool   `json:"exit"`
	}
	type result struct {
		a   answer
		err error
	}
	req, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	done := make(chan result, 1)
	go func() {
		if _, err := s.in.Write(append(req, '\n')); err != nil {
			done <- result{err: err}
			return
		}
		line, err := s.out.ReadBytes('\n')
		if err != nil {
			done <- result{err: err}
			return
		}
		var a answer
		if err := json.Unmarshal(line, &a); err != nil {
			done <- result{err: fmt.Errorf("answer %q: %w", line, err)}
			return
		}
		done <- result{a: a}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			p.free <- nil
			return -1, s.stop(), false
		}
		if r.a.Exit {
			s.stop()
			p.free <- nil
		} else {
			p.free <- s
		}
		return r.a.Code, r.a.Out, false
	case <-time.After(timeout):
		s.stop()
		p.free <- nil
		return -1, "", true
	}
}

func (p *kcPool) close() {
	for range cap(p.free) {
		if s := <-p.free; s != nil {
			s.stop()
		}
	}
}
