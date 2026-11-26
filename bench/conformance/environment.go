package main

import (
	"archive/zip"
	"io"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
)

// minimalStdlibNames are the top-level functions and properties of the
// minimal standard library that kotlinc's test infrastructure compiles the
// tests without WITH_STDLIB against (libraries/stdlib/jvm-minimal-for-test:
// Standard.kt, Library.kt, the array factories and intrinsics, contracts,
// contextOf, enumEntries, its atomics' updates and its Throwable
// extensions).
var minimalStdlibNames = map[string]bool{
	"run": true, "let": true, "also": true, "apply": true, "with": true,
	"takeIf": true, "takeUnless": true, "repeat": true, "TODO": true,
	"toString": true, "plus": true, "arrayOf": true, "arrayOfNulls": true,
	"emptyArray": true, "doubleArrayOf": true, "floatArrayOf": true,
	"longArrayOf": true, "intArrayOf": true, "charArrayOf": true,
	"shortArrayOf": true, "byteArrayOf": true, "booleanArrayOf": true,
	"enumEntries": true, "contract": true, "contextOf": true, "context": true,
	"printStackTrace": true, "stackTraceToString": true, "addSuppressed": true,
	"suppressedExceptions": true, "update": true, "fetchAndUpdate": true,
	"updateAndFetch": true, "updateAt": true, "fetchAndUpdateAt": true,
	"updateAndFetchAt": true,
}

// stdlibOnlyNames returns the names of the top-level functions and
// properties of kotlin-stdlib.jar, read from its file facades' Kotlin
// metadata (inline-only ones, such as println, are private in their class
// files), less those of the minimal standard library
// (minimalStdlibNames): the top-level declarations a test without
// WITH_STDLIB doesn't have with kotlinc, but has with kc, which gets the
// full standard library.
func stdlibOnlyNames(jar string) (map[string]bool, error) {
	z, err := zip.OpenReader(jar)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	names := map[string]bool{}
	for _, f := range z.File {
		cls, ok := strings.CutSuffix(f.Name, ".class")
		if !ok || !strings.HasPrefix(cls, "kotlin/") {
			continue
		}
		simple := cls[strings.LastIndexByte(cls, '/')+1:]
		if !strings.HasSuffix(simple, "Kt") && !strings.Contains(simple, "Kt__") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, err
		}
		info, err := classfile.Parse(data)
		if err != nil || info.Kotlin == nil {
			continue
		}
		p, err := kmeta.ReadPackage(info.Kotlin)
		if err != nil {
			continue
		}
		funcs, props, _ := p.Names()
		for _, n := range append(funcs, props...) {
			names[n] = true
		}
	}
	for n := range minimalStdlibNames {
		delete(names, n)
	}
	return names, nil
}

// environment reports whether the expected error e, which kc didn't
// report on its line, is one that only the standard library kc gets
// keeps it from: the test has no WITH_STDLIB, which gives kotlinc the
// minimal standard library, and e is an unsigned literal, or an
// unresolved name the full standard library declares at the top level
// and the minimal one doesn't, such as println, listOf or Comparable's
// rangeTo (..).
func (dt *diagTest) environment(e mark) bool {
	if dt.stdlibOnly == nil {
		return false
	}
	switch e.name {
	case "UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH":
		return true
	case "UNRESOLVED_REFERENCE":
		name := e.text
		switch name {
		case "..":
			name = "rangeTo"
		case "..<":
			name = "rangeUntil"
		}
		return dt.stdlibOnly[name]
	}
	return false
}
