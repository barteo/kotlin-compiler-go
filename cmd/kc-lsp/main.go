// kc-lsp provides Kotlin/JVM language support over standard input and output.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/barteo/kotlin-compiler-go/internal/lsp"
)

var version = "dev"

func main() {
	stdio := flag.Bool("stdio", true, "serve LSP over standard input and output")
	show := flag.Bool("version", false, "print the version")
	flag.Parse()
	if *show {
		fmt.Println("kc-lsp", version)
		return
	}
	if !*stdio || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := lsp.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
