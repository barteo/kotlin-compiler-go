package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestPrintVersion(t *testing.T) {
	var b strings.Builder
	printVersion(&b)
	got := b.String()
	if !strings.HasPrefix(got, "kc "+version) || !strings.HasSuffix(got, " "+runtime.GOOS+"/"+runtime.GOARCH+"\n") {
		t.Errorf("printVersion wrote %q", got)
	}
}
