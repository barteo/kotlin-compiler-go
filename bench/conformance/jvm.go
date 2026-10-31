package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// A jvm is a running BoxRunner, which runs one test at a time.
type jvm struct {
	cmd   *exec.Cmd
	in    io.WriteCloser
	lines chan string
}

// runnerConfig says how to start BoxRunners.
type runnerConfig struct {
	java    string   // the java command
	classes string   // the directory BoxRunner.class is in
	libs    []string // the standard library and kotlin-test
	reflect string
	timeout time.Duration // a test's
}

// buildRunner compiles BoxRunner.java into dir.
func buildRunner(javac, src, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out, err := exec.Command(javac, "-d", dir, src).CombinedOutput()
	if err != nil {
		return fmt.Errorf("javac %s: %v\n%s", src, err, out)
	}
	return nil
}

func (rc *runnerConfig) start() (*jvm, error) {
	cmd := exec.Command(rc.java, "-XX:+UseSerialGC", "-XX:TieredStopAtLevel=1", "-cp", rc.classes, "BoxRunner",
		strings.Join(rc.libs, string(filepath.ListSeparator)), rc.reflect, fmt.Sprint(int(rc.timeout.Seconds())))
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	j := &jvm{cmd: cmd, in: in, lines: make(chan string)}
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			j.lines <- sc.Text()
		}
		close(j.lines)
	}()
	return j, nil
}

func (j *jvm) stop() {
	j.in.Close()
	j.cmd.Process.Kill()
	j.cmd.Wait()
}

// runOutcome is what running a test's box() came to: BoxRunner's status
// and the rest of its line, or DIED if the JVM ended without one.
type runOutcome struct {
	status string
	detail []string
}

// pool runs tests on n JVMs. Each request goes to a free JVM, which is
// started again after a test that didn't return.
type pool struct {
	rc   *runnerConfig
	free chan *jvm
}

func newPool(rc *runnerConfig, n int) *pool {
	p := &pool{rc: rc, free: make(chan *jvm, n)}
	for range n {
		p.free <- nil
	}
	return p
}

// run runs box() of class with the class path dirs.
func (p *pool) run(class string, dirs []string, withReflect bool) runOutcome {
	j := <-p.free
	if j == nil {
		var err error
		if j, err = p.rc.start(); err != nil {
			p.free <- nil
			return runOutcome{"DIED", []string{err.Error()}}
		}
	}
	r := "0"
	if withReflect {
		r = "1"
	}
	fmt.Fprintf(j.in, "x\t%s\t%s\t%s\n", r, class, strings.Join(dirs, string(filepath.ListSeparator)))
	var o runOutcome
	select {
	case line, ok := <-j.lines:
		if !ok {
			o = runOutcome{"DIED", nil}
			break
		}
		f := strings.Split(line, "\t")
		o = runOutcome{f[1], f[2:]}
	case <-time.After(p.rc.timeout + 30*time.Second):
		o = runOutcome{"TIMEOUT", nil}
	}
	if o.status == "OK" || o.status == "FAIL" || o.status == "VERIFY" || o.status == "LINKAGE" ||
		o.status == "EXCEPTION" || o.status == "NOBOX" {
		p.free <- j
	} else {
		j.stop()
		p.free <- nil
	}
	return o
}

func (p *pool) close() {
	for range cap(p.free) {
		if j := <-p.free; j != nil {
			j.stop()
		}
	}
}
