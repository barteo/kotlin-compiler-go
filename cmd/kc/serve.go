package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// serve compiles again and again, as a tool that runs many small
// compiles asks, keeping the class paths and the libraries it reads
// them with for the compiles after (see driver.Libraries): most of a
// small compile's work is indexing the JDK and reading the standard
// library's classes, which a process of its own does every time.
//
// Each line of in is a compile: its command line arguments, as a JSON
// array of strings. Each is answered with a line of out, a JSON object:
//
//	{"code": 1, "out": "...", "exit": false}
//
// code is the exit code kc would have exited with, and out what it
// would have written to stderr. exit is set when the server exits after
// the answer: after a failure that isn't a diagnostic, such as code
// generation that panicked, which a later compile mustn't see the
// state of. A panic that kc doesn't recover ends the process as it
// would end kc, with its trace on stderr. Requests are served one at a
// time; the server exits at the end of in.
func serve(in io.Reader, out io.Writer) error {
	libs := &driver.Libraries{}
	defer libs.Close()
	r := bufio.NewReader(in)
	enc := json.NewEncoder(out)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) == 0 {
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			continue
		}
		var args []string
		if err := json.Unmarshal(line, &args); err != nil {
			return fmt.Errorf("request %q: %w", bytes.TrimSpace(line), err)
		}
		var stderr bytes.Buffer
		code, cerr := compile(args, &stderr, libs)
		answer := struct {
			Code int    `json:"code"`
			Out  string `json:"out"`
			Exit bool   `json:"exit,omitempty"`
		}{code, stderr.String(), !onlyDiagnostics(cerr)}
		if err := enc.Encode(answer); err != nil {
			return err
		}
		if answer.Exit {
			return nil
		}
	}
}

// onlyDiagnostics reports whether err, a compile's, is nil or only
// diagnostics of the program.
func onlyDiagnostics(err error) bool {
	if err == nil {
		return true
	}
	errs := []error{err}
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		errs = j.Unwrap()
	}
	for _, e := range errs {
		var se *syntax.Error
		if !errors.As(e, &se) {
			return false
		}
	}
	return true
}
