package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeServer is a kc -serve that answers with its process id, crashes
// as a panic does on a request with boom, hangs on one with slow, and
// exits after its answer to one with bye.
const fakeServer = `#!/bin/sh
while read -r line; do
	case "$line" in
	*boom*) echo 'panic: boom' >&2; echo 'goroutine 1 [running]:' >&2; exit 2;;
	*slow*) sleep 30;;
	*bye*) echo '{"code":1,"out":"failed","exit":true}'; exit 0;;
	*) echo '{"code":0,"out":"'$$'"}';;
	esac
done
`

func TestKcPool(t *testing.T) {
	kc := filepath.Join(t.TempDir(), "kc")
	if err := os.WriteFile(kc, []byte(fakeServer), 0o755); err != nil {
		t.Fatal(err)
	}
	p := newKcPool(kc, 1)
	defer p.close()
	compile := func(arg string, timeout time.Duration) (int, string, bool) {
		return p.compile([]string{arg}, timeout)
	}
	code, pid, _ := compile("a", time.Minute)
	if code != 0 || pid == "" {
		t.Fatalf("compile: %d %q", code, pid)
	}
	if _, again, _ := compile("b", time.Minute); again != pid {
		t.Errorf("second compile on server %s, want the first's, %s", again, pid)
	}
	code, out, timedOut := compile("boom", time.Minute)
	if code != -1 || timedOut || !strings.HasPrefix(out, "panic: boom\n") {
		t.Errorf("crash: %d %q %t", code, out, timedOut)
	}
	code, pid2, _ := compile("a", time.Minute)
	if code != 0 || pid2 == pid {
		t.Errorf("after a crash: %d on server %s, want a new one", code, pid2)
	}
	if _, _, timedOut := compile("slow", 100*time.Millisecond); !timedOut {
		t.Error("a hanging compile didn't time out")
	}
	if code, out, _ := compile("bye", time.Minute); code != 1 || out != "failed" {
		t.Errorf("exiting compile: %d %q", code, out)
	}
	code, pid3, _ := compile("a", time.Minute)
	if code != 0 || pid3 == pid2 {
		t.Errorf("after an exit: %d on server %s, want a new one", code, pid3)
	}
}
