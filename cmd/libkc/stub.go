//go:build !libkc

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "libkc: build with -tags libkc -buildmode=c-shared (see go doc ./cmd/libkc)")
	os.Exit(2)
}
