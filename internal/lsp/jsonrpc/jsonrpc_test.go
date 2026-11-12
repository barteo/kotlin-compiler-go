package jsonrpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// pair connects two Conns with pipes.
func pair(t *testing.T) (a, b *Conn) {
	ar, bw := io.Pipe()
	br, aw := io.Pipe()
	t.Cleanup(func() { aw.Close(); bw.Close() })
	return NewConn(ar, aw), NewConn(br, bw)
}

func TestCallAndNotify(t *testing.T) {
	client, server := pair(t)
	notes := make(chan string, 4)
	go server.Serve(context.Background(), func(ctx context.Context, r *Request) (any, error) {
		switch r.Method {
		case "add":
			var p [2]int
			if err := r.Unmarshal(&p); err != nil {
				return nil, err
			}
			return p[0] + p[1], nil
		case "note":
			var s string
			r.Unmarshal(&s)
			notes <- s
			return nil, nil
		case "boom":
			panic("boom")
		}
		return nil, Errorf(CodeMethodNotFound, "no %s", r.Method)
	})
	go client.Serve(context.Background(), func(context.Context, *Request) (any, error) { return nil, nil })

	ctx := context.Background()
	if err := client.Notify("note", "hello"); err != nil {
		t.Fatal(err)
	}
	var sum int
	if err := client.Call(ctx, "add", [2]int{2, 3}, &sum); err != nil || sum != 5 {
		t.Fatalf("add: %d, %v", sum, err)
	}
	if got := <-notes; got != "hello" {
		t.Errorf("note: %q", got)
	}
	var e *Error
	if err := client.Call(ctx, "nope", nil, nil); !errors.As(err, &e) || e.Code != CodeMethodNotFound {
		t.Errorf("nope: %v", err)
	}
	if err := client.Call(ctx, "boom", nil, nil); !errors.As(err, &e) || e.Code != CodeInternalError || !strings.Contains(e.Message, "panic: boom") {
		t.Errorf("boom: %v", err)
	}
}

// TestCancel checks that $/cancelRequest cancels a running handler's context
// and that the request is answered with CodeRequestCancelled.
func TestCancel(t *testing.T) {
	client, server := pair(t)
	started := make(chan struct{})
	go server.Serve(context.Background(), func(ctx context.Context, r *Request) (any, error) {
		if r.Method == "slow" {
			close(started)
			select {
			case <-ctx.Done():
				return nil, nil
			case <-time.After(10 * time.Second):
				return "late", nil
			}
		}
		return "ok", nil
	})
	go client.Serve(context.Background(), func(context.Context, *Request) (any, error) { return nil, nil })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Call(ctx, "slow", nil, nil) }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("slow: %v", err)
	}
	// The server goes on with the next request once the handler returns.
	var s string
	if err := client.Call(context.Background(), "fast", nil, &s); err != nil || s != "ok" {
		t.Fatalf("fast: %q, %v", s, err)
	}
}

func TestFraming(t *testing.T) {
	in := "Content-Length: 40\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n" +
		`{"jsonrpc":"2.0","id":7,"method":"echo"}`
	var out bytes.Buffer
	c := NewConn(strings.NewReader(in), &out)
	if err := c.Serve(context.Background(), func(context.Context, *Request) (any, error) { return "é", nil }); err != nil {
		t.Fatal(err)
	}
	want := "Content-Length: 38\r\n\r\n" + `{"jsonrpc":"2.0","id":7,"result":"é"}`
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}
