package syntax

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkLexers compares the lexer built on psi.Lex with kc's own, on
// the .kt files under $KC_BENCH_CORPUS, or under testdata, that both lex
// and parse without errors: lexing alone, and lexing and parsing.
func BenchmarkLexers(b *testing.B) {
	root := os.Getenv("KC_BENCH_CORPUS")
	if root == "" {
		root = "../../testdata"
	}
	type file struct{ path, src string }
	var files []file
	var size int64
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		if _, errs := Parse(path, src); len(errs) > 0 {
			return nil
		}
		if toks, errs := legacyTokenize(path, src); len(errs) > 0 {
			return nil
		} else if _, err := parseTokens(path, toks); err != nil {
			return nil
		}
		files = append(files, file{path, src})
		size += int64(len(src))
		return nil
	})
	if len(files) == 0 {
		b.Fatalf("no .kt files under %s", root)
	}
	b.Logf("%d files, %d bytes", len(files), size)
	run := func(name string, fn func(path, src string)) {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(size)
			b.ReportAllocs()
			for b.Loop() {
				for _, f := range files {
					fn(f.path, f.src)
				}
			}
		})
	}
	run("lex/legacy", func(path, src string) { legacyTokenize(path, src) })
	run("lex/psi", func(path, src string) { Tokenize(path, src) })
	run("parse/legacy", func(path, src string) {
		toks, _ := legacyTokenize(path, src)
		parseTokens(path, toks)
	})
	run("parse/psi", func(path, src string) {
		toks, _ := Tokenize(path, src)
		parseTokens(path, toks)
	})
}
