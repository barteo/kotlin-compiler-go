// Package jsonrpc is JSON-RPC 2.0 over the Language Server Protocol's base
// protocol: messages framed by a Content-Length header.
//
// A Conn is symmetric: it sends requests and notifications and serves the
// ones it receives. Received messages are handled one at a time, in order,
// as LSP requires (a request sees every change sent before it), while a
// separate goroutine keeps reading so that responses and $/cancelRequest
// are seen while a handler runs. A cancelled request's context is done.
package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
)

// Error codes of JSON-RPC and LSP.
const (
	CodeParseError           = -32700
	CodeInvalidRequest       = -32600
	CodeMethodNotFound       = -32601
	CodeInvalidParams        = -32602
	CodeInternalError        = -32603
	CodeServerNotInitialized = -32002
	CodeRequestCancelled     = -32800
	CodeRequestFailed        = -32803
)

// Error is a JSON-RPC error object. A handler returning one sends it as is;
// any other error becomes CodeInternalError.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("jsonrpc %d: %s", e.Code, e.Message) }

// Errorf returns an *Error with the given code.
func Errorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// message is any of request, notification and response.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Request is a received request or notification.
type Request struct {
	Method string
	Params json.RawMessage
	// ID is the request's ID, nil for a notification.
	ID json.RawMessage
}

// IsNotification reports whether r expects no response.
func (r *Request) IsNotification() bool { return r.ID == nil }

// Unmarshal decodes the params into v, as CodeInvalidParams on failure.
func (r *Request) Unmarshal(v any) error {
	if len(r.Params) == 0 {
		return nil
	}
	if err := json.Unmarshal(r.Params, v); err != nil {
		return Errorf(CodeInvalidParams, "%s: %v", r.Method, err)
	}
	return nil
}

// Handler handles a received request or notification. For a request, the
// result (or error) is sent as the response; for a notification both are
// ignored. A handler that panics answers with CodeInternalError.
type Handler func(ctx context.Context, r *Request) (any, error)

// Conn is one end of a connection.
type Conn struct {
	r *bufio.Reader
	w io.Writer

	wmu sync.Mutex // serializes writes

	mu       sync.Mutex
	nextID   int64
	pending  map[string]chan *message // our requests awaiting responses
	inflight map[string]context.CancelFunc
	closed   bool
	err      error
}

// NewConn returns a connection reading from r and writing to w.
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{
		r:        bufio.NewReader(r),
		w:        w,
		pending:  map[string]chan *message{},
		inflight: map[string]context.CancelFunc{},
	}
}

// queued is a received request or notification and, for a request, the
// context that $/cancelRequest cancels.
type queued struct {
	m      *message
	ctx    context.Context
	cancel context.CancelFunc
}

// Serve reads messages until the reader ends or ctx is done, handling
// requests and notifications with h in order. It returns nil at EOF and
// when ctx is done.
func (c *Conn) Serve(ctx context.Context, h Handler) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	queue := make(chan queued, 64)
	readErr := make(chan error, 1)
	go func() {
		readErr <- c.readLoop(ctx, queue)
		close(queue)
	}()
	for {
		select {
		case q, ok := <-queue:
			if ok {
				c.handle(h, q)
				continue
			}
			err := <-readErr
			c.shut(err)
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		case <-ctx.Done():
			// The reader stays blocked until its input ends; the
			// connection is over for the caller.
			c.shut(ctx.Err())
			return nil
		}
	}
}

// readLoop reads messages, routes responses and cancellations itself, and
// queues requests and notifications for Serve.
func (c *Conn) readLoop(ctx context.Context, queue chan<- queued) error {
	for {
		m, err := c.read()
		if err != nil {
			return err
		}
		switch {
		case m.Method == "" && m.ID != nil: // a response
			c.mu.Lock()
			ch := c.pending[normID(m.ID)]
			delete(c.pending, normID(m.ID))
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
			continue
		case m.Method == "$/cancelRequest":
			var p struct {
				ID json.RawMessage `json:"id"`
			}
			if json.Unmarshal(m.Params, &p) == nil {
				c.mu.Lock()
				if cancel := c.inflight[normID(p.ID)]; cancel != nil {
					cancel()
				}
				c.mu.Unlock()
			}
			continue
		}
		q := queued{m: m, ctx: ctx}
		if m.ID != nil {
			q.ctx, q.cancel = context.WithCancel(ctx)
			c.mu.Lock()
			c.inflight[normID(m.ID)] = q.cancel
			c.mu.Unlock()
		}
		select {
		case queue <- q:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// normID makes 1 and "1" distinct and spacing irrelevant.
func normID(id json.RawMessage) string {
	var v any
	if json.Unmarshal(id, &v) != nil {
		return string(id)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func (c *Conn) handle(h Handler, q queued) {
	m := q.m
	req := &Request{Method: m.Method, Params: m.Params, ID: m.ID}
	if m.ID == nil {
		_, _ = c.call(q.ctx, h, req)
		return
	}
	defer func() {
		c.mu.Lock()
		delete(c.inflight, normID(m.ID))
		c.mu.Unlock()
		q.cancel()
	}()
	var result any
	var err error
	if q.ctx.Err() == nil { // not cancelled while queued
		result, err = c.call(q.ctx, h, req)
	}
	if err == nil && q.ctx.Err() != nil {
		err = Errorf(CodeRequestCancelled, "%s: request cancelled", m.Method)
	}
	resp := &message{JSONRPC: "2.0", ID: m.ID}
	if err != nil {
		var e *Error
		if !errors.As(err, &e) {
			e = &Error{Code: CodeInternalError, Message: err.Error()}
		}
		resp.Error = e
	} else {
		b, merr := json.Marshal(result)
		if merr != nil {
			resp.Error = &Error{Code: CodeInternalError, Message: merr.Error()}
		} else {
			resp.Result = b
		}
	}
	_ = c.write(resp)
}

func (c *Conn) call(ctx context.Context, h Handler, req *Request) (result any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = Errorf(CodeInternalError, "%s: panic: %v\n%s", req.Method, p, debug.Stack())
		}
	}()
	return h(ctx, req)
}

// Notify sends a notification.
func (c *Conn) Notify(method string, params any) error {
	b, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(&message{JSONRPC: "2.0", Method: method, Params: b})
}

// Call sends a request and waits for its response, decoding the result into
// result (which may be nil). If ctx is done first, it sends $/cancelRequest
// and returns ctx's error.
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	b, err := json.Marshal(params)
	if err != nil {
		return err
	}
	ch := make(chan *message, 1)
	c.mu.Lock()
	if c.closed {
		err := c.err
		c.mu.Unlock()
		return fmt.Errorf("jsonrpc: connection closed: %w", err)
	}
	c.nextID++
	id := json.RawMessage(strconv.FormatInt(c.nextID, 10))
	c.pending[normID(id)] = ch
	c.mu.Unlock()
	if err := c.write(&message{JSONRPC: "2.0", ID: id, Method: method, Params: b}); err != nil {
		return err
	}
	select {
	case m, ok := <-ch:
		if !ok || m == nil {
			return fmt.Errorf("jsonrpc: connection closed")
		}
		if m.Error != nil {
			return m.Error
		}
		if result != nil && len(m.Result) > 0 {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	case <-ctx.Done():
		_ = c.Notify("$/cancelRequest", map[string]json.RawMessage{"id": id})
		return ctx.Err()
	}
}

// shut fails the requests still waiting for responses.
func (c *Conn) shut(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed, c.err = true, err
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

func (c *Conn) write(m *message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
		return err
	}
	_, err = c.w.Write(b)
	return err
}

func (c *Conn) read() (*message, error) {
	length := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" && length < 0 {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("jsonrpc: reading header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if length < 0 {
				continue // tolerate blank lines between messages
			}
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("jsonrpc: bad header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("jsonrpc: bad Content-Length %q", value)
			}
			length = n
		}
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return nil, fmt.Errorf("jsonrpc: reading body: %w", err)
	}
	m := new(message)
	if err := json.Unmarshal(buf, m); err != nil {
		return nil, fmt.Errorf("jsonrpc: %w", err)
	}
	return m, nil
}
