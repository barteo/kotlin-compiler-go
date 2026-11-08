// Command kc-lsp is a Kotlin language server on kc. It speaks the Language
// Server Protocol on standard input and output; see docs/lsp.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/barteo/kotlin-compiler-go/internal/lsp"
)

func main() {
	version := flag.Bool("version", false, "print the version and exit")
	logFile := flag.String("log", "", "append the server's log to this file (default: standard error)")
	flag.Bool("stdio", true, "use standard input and output (the only transport; accepted for clients that pass it)")
	flag.Parse()
	if *version {
		fmt.Println("kc-lsp", lsp.Version)
		return
	}
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(f)
	}
	code, err := lsp.Serve(context.Background(), os.Stdin, os.Stdout)
	if err != nil {
		log.Print(err)
	}
	os.Exit(code)
}
