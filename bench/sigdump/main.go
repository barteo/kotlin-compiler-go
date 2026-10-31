// Command sigdump prints the generic signatures of a directory of class
// files, the Signature attributes of classes, fields and methods, one per
// line, sorted, for comparing two builds with diff:
//
//	go run ./bench/sigdump kotlinc-classes/ > a.txt
//	go run ./bench/sigdump kc-classes/ > b.txt
//	diff a.txt b.txt
//
// Synthetic members, which Java compilers don't see, are left out, and so
// are members without a signature, unless -all is given.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
)

func main() {
	all := flag.Bool("all", false, "also print the members without a signature")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: sigdump [-all] classes-dir")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	var lines []string
	err := filepath.WalkDir(flag.Arg(0), func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".class") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := cf.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if info.Sig != "" {
			lines = append(lines, info.Name+" class "+info.Sig)
		}
		member := func(kind string, m cf.Member) {
			if m.Access&cf.AccSynthetic != 0 || m.Sig == "" && !*all {
				return
			}
			lines = append(lines, fmt.Sprintf("%s %s %s%s %s", info.Name, kind, m.Name, m.Desc, m.Sig))
		}
		for _, f := range info.Fields {
			member("field", f)
		}
		for _, m := range info.Methods {
			member("method", m)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Println(l)
	}
}
