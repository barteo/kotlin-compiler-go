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

// minimalStdlibPackages are the packages of the built-in declarations and
// the minimal standard library (see minimalStdlibNames): a test without
// WITH_STDLIB can import from these with kotlinc, but not from the full
// standard library's others, such as kotlin.math.
var minimalStdlibPackages = map[string]bool{
	"kotlin": true, "kotlin.annotation": true, "kotlin.collections": true,
	"kotlin.coroutines": true, "kotlin.coroutines.intrinsics": true,
	"kotlin.internal": true, "kotlin.ranges": true, "kotlin.reflect": true,
	"kotlin.jvm": true, "kotlin.jvm.functions": true, "kotlin.jvm.internal": true,
	"kotlin.contracts": true, "kotlin.enums": true, "kotlin.concurrent.atomics": true,
}

// mockJDKMissing are the packages of the JDK that the test
// infrastructure's mock JDK (compiler/testData/mockJDK), which a test
// without FULL_JDK compiles against, doesn't have: those Java 8 added.
var mockJDKMissing = map[string]bool{"java.util.function": true, "java.util.stream": true, "java.time": true}

// stdlibOnlyNames returns the names of the top-level functions and
// properties of kotlin-stdlib.jar, read from its file facades' Kotlin
// metadata (inline-only ones, such as println, are private in their class
// files), less those of the minimal standard library
// (minimalStdlibNames): the top-level declarations a test without
// WITH_STDLIB doesn't have with kotlinc, but has with kc, which gets the
// full standard library. It also returns the packages of the standard
// library that the minimal one doesn't have (minimalStdlibPackages).
func stdlibOnlyNames(jar string) (map[string]bool, map[string]bool, error) {
	z, err := zip.OpenReader(jar)
	if err != nil {
		return nil, nil, err
	}
	defer z.Close()
	names, packages := map[string]bool{}, map[string]bool{}
	for _, f := range z.File {
		cls, ok := strings.CutSuffix(f.Name, ".class")
		if !ok || !strings.HasPrefix(cls, "kotlin/") {
			continue
		}
		if pkg := strings.ReplaceAll(cls[:strings.LastIndexByte(cls, '/')], "/", "."); !minimalStdlibPackages[pkg] {
			packages[pkg] = true
		}
		simple := cls[strings.LastIndexByte(cls, '/')+1:]
		if !strings.HasSuffix(simple, "Kt") && !strings.Contains(simple, "Kt__") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, nil, err
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
	return names, packages, nil
}

// importedPrefix returns the path of the import on line up to and with
// its segment elem, kotlin.math of import kotlin.math.* and math, or "".
func importedPrefix(line, elem string) string {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), "import ")
	if !ok {
		return ""
	}
	var path []string
	for _, seg := range strings.Split(strings.Fields(rest)[0], ".") {
		seg = strings.Trim(seg, "`;")
		path = append(path, seg)
		if seg == elem {
			return strings.Join(path, ".")
		}
	}
	return ""
}

// environment reports whether the expected error e, which kc didn't
// report on its line, is one that only the standard library kc gets
// keeps it from: the test has no WITH_STDLIB, which gives kotlinc the
// minimal standard library, and e is an unsigned literal, or an
// unresolved name the full standard library declares at the top level
// and the minimal one doesn't, such as println, listOf or Comparable's
// rangeTo (..), an index operator only the full one declares for its
// type (MutableMap.set), or an import of a package only it has. Or the
// test is compiled against the mock JDK, without FULL_JDK, and e is an
// import of a package the mock JDK doesn't have.
func (dt *diagTest) environment(e mark) bool {
	if e.name == "UNRESOLVED_IMPORT" && dt.mockJDK && mockJDKMissing[e.imported] {
		return true
	}
	if dt.stdlibOnly == nil {
		return false
	}
	switch e.name {
	case "UNRESOLVED_IMPORT":
		return dt.stdlibOnlyPackages[e.imported]
	case "NO_GET_METHOD":
		return dt.stdlibOnly["get"]
	case "NO_SET_METHOD":
		return dt.stdlibOnly["set"]
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
