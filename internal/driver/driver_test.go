package driver

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// stdlib returns the path of kotlin-stdlib.jar, or "" if it isn't found.
func stdlib() string {
	path, _ := classpath.FindStdlib()
	return path
}

// skipPSIGrammar skips a test of srcs under the psi frontend if one of
// them is Kotlin kc parses but stock Kotlin 2.4.21's grammar, which psi
// follows, doesn't.
func skipPSIGrammar(t *testing.T, srcs ...string) {
	t.Helper()
	if f, _ := frontendOf(Options{}); f != "psi" {
		return
	}
	for _, path := range srcs {
		if !strings.HasSuffix(path, ".kt") {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ds := psi.Parse(path, string(b)).Diagnostics()
		if _, errs := syntax.ParseAll(path, string(b)); len(ds) > 0 && len(errs) == 0 {
			t.Skipf("psi frontend: %s: %s, which kc's parser accepts", path, ds[0].Message)
		}
	}
}

// usesStdlib reports whether a test program or error test needs the Kotlin
// standard library: its name starts with stdlib.
func usesStdlib(path string) bool { return strings.HasPrefix(filepath.Base(path), "stdlib") }

// TestRun compiles each testdata/run/X.kt, runs it on the JVM with full
// verification, and compares stdout with X.out (produced by kotlinc). A
// directory testdata/run/X/ is a program of several files, whose main
// function is in main.kt. Programs are compiled without the Kotlin
// standard library and with it, except those named stdlib*, which need it.
func TestRun(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	files, _ := filepath.Glob("../../testdata/run/*.kt")
	dirs, _ := filepath.Glob("../../testdata/run/*/main.kt")
	for _, d := range dirs {
		files = append(files, filepath.Dir(d))
	}
	if len(files) == 0 {
		t.Fatal("no test programs found")
	}
	lib := stdlib()
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".kt")
		for _, withLib := range []bool{false, true} {
			sub := name
			if withLib {
				sub += "/stdlib"
			}
			t.Run(sub, func(t *testing.T) {
				switch {
				case withLib && lib == "":
					t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
				case !withLib && usesStdlib(f):
					t.Skip("needs the standard library")
				}
				t.Parallel()
				runProgram(t, java, f, lib, withLib)
			})
		}
	}
}

func runProgram(t *testing.T, java, f, lib string, withLib bool) {
	want, err := os.ReadFile(strings.TrimSuffix(f, ".kt") + ".out")
	if err != nil {
		t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
	}
	out := t.TempDir()
	opts := Options{OutDir: out, Jobs: runtime.NumCPU()}
	cp := out
	if withLib {
		opts.ClassPath = []string{lib}
		cp += string(os.PathListSeparator) + lib
		if strings.HasPrefix(filepath.Base(f), "stdlibReflect") {
			// kotlin-reflect, which the kotlin command that made the
			// expected output adds.
			reflect := filepath.Join(filepath.Dir(lib), "kotlin-reflect.jar")
			opts.ClassPath = append(opts.ClassPath, reflect)
			cp += string(os.PathListSeparator) + reflect
		}
		if strings.HasPrefix(filepath.Base(f), "stdlibCoroutines") {
			// kotlinx.coroutines, which kotlinc ships with.
			kx := kotlinxCoroutines(lib)
			opts.ClassPath = append(opts.ClassPath, kx)
			cp += string(os.PathListSeparator) + kx
		}
	}
	srcs, main := []string{f}, f
	if !strings.HasSuffix(f, ".kt") {
		srcs, main = sources(t, f), filepath.Join(f, "main.kt")
	}
	flagOptions(&opts, kotlincFlags(main))
	skipPSIGrammar(t, srcs...)
	if err := Compile(srcs, opts); err != nil {
		t.Fatalf("compile: %v", err)
	}
	checkLines(t, out)
	checkLoopStacks(t, out)
	cmd := exec.Command(java, "-Xverify:all", "-cp", cp, mainClass(t, main))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("java: %v\n%s", err, stderr.String())
	}
	if got := stdout.String(); got != string(want) {
		t.Errorf("output mismatch\n%s", diff(string(want), got))
	}
}

// checkLines checks the line tables of the classes in dir: each method
// that isn't synthetic has one, and its offsets are in the code.
func checkLines(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".class") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ms, err := classfile.ReadMethods(data)
		if err != nil {
			return err
		}
		for _, m := range ms {
			if m.Code == nil {
				continue
			}
			for _, l := range m.Lines {
				if l[0] >= len(m.Code) || l[1] <= 0 {
					t.Errorf("%s.%s%s: line entry %v", m.Class, m.Name, m.Desc, l)
				}
			}
			// Classes copied from the standard library for inlined calls
			// keep its code.
			// As kotlinc's, a static initializer that only creates
			// $$delegatedProperties has none, nor do SAM wrappers.
			copied := strings.Contains(m.Class, "$$inlined$")
			sam := strings.Contains(m.Class, "$sam$")
			if len(m.Lines) == 0 && m.Access&(classfile.AccSynthetic|classfile.AccBridge) == 0 && !copied && !sam && m.Name != "<clinit>" {
				t.Errorf("%s.%s%s has no line numbers", m.Class, m.Name, m.Desc)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// checkLoopStacks checks that no backward branch in the classes in dir
// has values on the operand stack: HotSpot doesn't compile such a loop
// on stack replacement, so a method that runs it once stays in the
// interpreter. Inlined code starts with the stack in locals, as
// kotlinc's (see codegen's spillStack).
func checkLoopStacks(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".class") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ms, err := classfile.ReadMethods(data)
		if err != nil {
			return err
		}
		for _, m := range ms {
			if m.Code == nil {
				continue
			}
			for _, off := range loopStacks(t, m) {
				t.Errorf("%s.%s%s: backward branch to offset %d with %d values on the stack", m.Class, m.Name, m.Desc, off, len(m.Frames[off].Stack))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// loopStacks returns the targets of m's backward branches whose frames
// have values on the operand stack.
func loopStacks(t *testing.T, m *classfile.Method) []int {
	t.Helper()
	insns, err := m.Decode()
	if err != nil {
		t.Fatalf("%s.%s%s: %v", m.Class, m.Name, m.Desc, err)
	}
	var bad []int
	for _, in := range insns {
		var targets []int
		switch op := in.Op; {
		case classfile.IFEQ <= op && op <= classfile.GOTO, op == classfile.IFNULL, op == classfile.IFNONNULL:
			targets = []int{in.Target}
		case op == classfile.TABLESWITCH, op == classfile.LOOKUPSWITCH:
			targets = append([]int{in.Target}, in.Targets...)
		}
		for _, to := range targets {
			if f := m.Frames[to]; to <= in.Off && f != nil && len(f.Stack) > 0 && !slices.Contains(bad, to) {
				bad = append(bad, to)
			}
		}
	}
	return bad
}

// sources returns the .kt files in a directory and its subdirectories.
// kotlincFlags returns the flags of a first line "// kotlinc: <flags>"
// of file path.
func kotlincFlags(path string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	first, _, _ := strings.Cut(string(src), "\n")
	flags, ok := strings.CutPrefix(first, "// kotlinc: ")
	if !ok {
		return nil
	}
	return strings.Fields(flags)
}

// moduleDeps returns the output directories of the modules that a first
// line "// deps: <modules>" of the file at path names, from outs, or
// def if it has none.
func moduleDeps(path string, outs map[string]string, def []string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	first, _, _ := strings.Cut(string(src), "\n")
	deps, ok := strings.CutPrefix(first, "// deps: ")
	if !ok {
		return def
	}
	var dirs []string
	for _, m := range strings.Fields(deps) {
		dirs = append(dirs, outs[m])
	}
	return dirs
}

// flagOptions sets the options for kotlinc's flags -jvm-default,
// -Xdebug, -Xmulti-platform and -opt-in=<marker> among flags.
func flagOptions(opts *Options, flags []string) {
	opts.MultiPlatform = slices.Contains(flags, "-Xmulti-platform")
	opts.NoJVMDefault = slices.Contains(flags, "-jvm-default=disable")
	opts.JVMDefaultCompat = slices.Contains(flags, "-jvm-default=enable")
	opts.Debug = slices.Contains(flags, "-Xdebug")
	if i := slices.Index(flags, "-jvm-target"); i >= 0 && i+1 < len(flags) {
		opts.JVMTarget = flags[i+1]
	}
	for _, f := range flags {
		if m, ok := strings.CutPrefix(f, "-opt-in="); ok {
			opts.OptIns = append(opts.OptIns, m)
		}
	}
}

func sources(t *testing.T, dir string) []string {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(path, ".kt") {
			out = append(out, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// javaSources returns the Java source files in dir and its
// subdirectories.
func javaSources(t *testing.T, dir string) []string {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(path, ".java") {
			out = append(out, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// mainClass returns the class name of a file's facade, as java takes it.
func mainClass(t *testing.T, path string) string {
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, errs := syntax.Parse(path, string(src))
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	return strings.ReplaceAll(f.Facade, "/", ".")
}

// TestLambdaClasses checks that lambdas add no class files: their bodies
// are methods of the enclosing class, created through invokedynamic.
func TestLambdaClasses(t *testing.T) {
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not found on PATH")
	}
	out := t.TempDir()
	if err := Compile([]string{"../../testdata/run/lambdas.kt"}, Options{OutDir: out, Jobs: 1}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var got []string
	err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".class") {
			rel, _ := filepath.Rel(out, path)
			got = append(got, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Button.class", "Counter.class", "Holder.class", "LambdasKt.class",
		"kc/jvm/functions/Function0.class", "kc/jvm/functions/Function1.class",
		"kc/jvm/functions/Function2.class", "kc/jvm/functions/Function3.class"}
	if !slices.Equal(got, want) {
		t.Errorf("class files:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestNoJDK checks that secondary constructors of a class without a
// superclass compile without a JDK, which has java.lang.Object's class.
func TestNoJDK(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a.kt")
	code := "class A {\n    constructor(x: Int)\n    constructor() : super()\n    constructor(s: String) : this(1)\n}\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Compile([]string{src}, Options{OutDir: t.TempDir(), Jobs: 1, NoJDK: true}); err != nil {
		t.Fatalf("compile: %v", err)
	}
}

// TestLambdaClassesStdlib checks that with the Kotlin standard library,
// lambdas implement its function interfaces, so none are written.
func TestLambdaClassesStdlib(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	out := t.TempDir()
	if err := Compile([]string{"../../testdata/run/lambdas.kt"}, Options{OutDir: out, Jobs: 1, ClassPath: []string{lib}}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "kc")); !os.IsNotExist(err) {
		t.Errorf("function interfaces were written: %v", err)
	}
}

// TestStdlibParallel checks many files that use the standard library in
// parallel, which reads its metadata and the bodies of inline functions
// concurrently. Run it with -race.
func TestStdlibParallel(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	dir := t.TempDir()
	var files []string
	for i := range 16 {
		n := strconv.Itoa(i)
		src := "fun use" + n + "(): Int {\n" +
			"    val xs = java.util.ArrayList<Int>()\n" +
			"    xs.add(" + n + ")\n" +
			"    var sum = 0\n" +
			"    for (x in xs) { sum += x }\n" +
			"    println(listOf(\"a\"))\n" +
			"    val m = HashMap<String, Int>()\n" +
			"    m.put(\"k\", sum)\n" +
			"    if (\"a\".isNotEmpty() && \"b\".isNotBlank()) sum += \"%d\".format(1).length\n" +
			"    return maxOf(sum, xs.size) + Pair(1, 2).first + m.size + Regex(\"a\").pattern.length\n" +
			"}\n"
		f := filepath.Join(dir, "file"+n+".kt")
		if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if err := Compile(files, Options{OutDir: t.TempDir(), Jobs: runtime.NumCPU(), ClassPath: []string{lib}}); err != nil {
		t.Fatal(err)
	}
}

// TestParallelInlineObjects checks that files copying an inline function
// whose object expression captures its crossinline lambda compile in
// parallel as on one thread. The copies and the file of the function
// generate the object's class at once, and once overwrote each other's
// locals of the shared variables: "local 3 is not allocated", or a
// constructor passed the wrong local. Run it with -race.
func TestParallelInlineObjects(t *testing.T) {
	dir := t.TempDir()
	lib := "package repro\n\n" +
		"interface Task { fun run(): Int }\n\n" +
		"inline fun task(crossinline block: () -> Int): Task = object : Task {\n" +
		"    override fun run(): Int = block()\n" +
		"}\n"
	files := []string{filepath.Join(dir, "Lib.kt")}
	if err := os.WriteFile(files[0], []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := range 64 {
		n := strconv.Itoa(i)
		src := "package repro\n\n" +
			"fun use" + n + "(x: Int): Int {\n" +
			"    val a = x + 1\n" +
			"    val t = task { a * 2 }\n" +
			"    return t.run()\n" +
			"}\n"
		f := filepath.Join(dir, "Use"+n+".kt")
		if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	compile := func(jobs int) map[string][]byte {
		out := t.TempDir()
		if err := Compile(files, Options{OutDir: out, Jobs: jobs, NoJDK: true}); err != nil {
			t.Fatalf("-j %d: %v", jobs, err)
		}
		classes := map[string][]byte{}
		err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(path)
			classes[strings.TrimPrefix(path, out)] = b
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return classes
	}
	want := compile(1)
	jobs := max(runtime.NumCPU(), 8)
	for range 10 {
		got := compile(jobs)
		for name, b := range want {
			if !bytes.Equal(got[name], b) {
				t.Fatalf("-j %d: %s differs from -j 1's", jobs, name)
			}
		}
		if len(got) != len(want) {
			t.Fatalf("-j %d: %d classes, -j 1: %d", jobs, len(got), len(want))
		}
	}
}

// TestRetained checks that a compilation keeps nothing once it is done:
// compiling again and again in one process, as these tests do, mustn't
// make the heap grow. Each compilation used to stay reachable from the
// process-wide intern table of type arguments, with every class it read.
func TestRetained(t *testing.T) {
	var cp []string
	if lib := stdlib(); lib != "" {
		cp = []string{lib}
	}
	srcs := []string{"../../testdata/run/lambdas.kt", "../../testdata/run/frames.kt"}
	compile := func() {
		if err := Compile(srcs, Options{OutDir: t.TempDir(), Jobs: runtime.NumCPU(), ClassPath: cp}); err != nil {
			t.Fatal(err)
		}
	}
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	compile() // fills what the process keeps for all compilations
	before := heap()
	const n = 8
	for range n {
		compile()
	}
	// A compilation of these used to keep about 15 MB, with the standard
	// library.
	if grew := int64(heap()) - int64(before); grew > 4<<20 {
		t.Errorf("the heap grew by %d MB over %d compilations", grew>>20, n)
	}
}

func diff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var sb strings.Builder
	for i := 0; i < max(len(w), len(g)); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			sb.WriteString("line " + strconv.Itoa(i+1) + ":\n  want: " + wl + "\n  got:  " + gl + "\n")
		}
	}
	return sb.String()
}

var errorComment = regexp.MustCompile(`// ERROR (.+)$`)

// TestErrors checks diagnostics. Each line of testdata/errors/X.kt that
// should produce an error ends with "// ERROR <substring of message>". A
// directory testdata/errors/X/ is one module of several files. Files
// named stdlib* are compiled with the Kotlin standard library. A first
// line "// kotlinc: <flags>" gives kotlinc's flags (see flagOptions).
func TestErrors(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/errors/*.kt")
	dirs, _ := filepath.Glob("../../testdata/errors/*/*.kt")
	for _, f := range dirs {
		if d := filepath.Dir(f); !slices.Contains(files, d) {
			files = append(files, d)
		}
	}
	lib := stdlib()
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			opts := Options{OutDir: t.TempDir(), Jobs: 1, JavaTypes: true} // as kc
			if usesStdlib(f) {
				if lib == "" {
					t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
				}
				opts.ClassPath = []string{lib}
			}
			srcs := []string{f}
			if fi, err := os.Stat(f); err == nil && fi.IsDir() {
				srcs, _ = filepath.Glob(filepath.Join(f, "*.kt"))
				javas, _ := filepath.Glob(filepath.Join(f, "*.java")) // the module's Java sources
				srcs = append(srcs, javas...)
			}
			flagOptions(&opts, kotlincFlags(srcs[0]))
			skipPSIGrammar(t, srcs...)
			checkErrors(t, srcs, Compile(srcs, opts))
		})
	}
}

// checkErrors compares a compile's errors with the "// ERROR <substring>"
// comments of srcs: every marked line has an error containing its
// substring, and no other line has one.
func checkErrors(t *testing.T, srcs []string, err error) {
	t.Helper()
	// By file name and line.
	want := map[string]string{}
	for _, src := range srcs {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if m := errorComment.FindStringSubmatch(line); m != nil {
				want[fmt.Sprintf("%s:%d", filepath.Base(src), i+1)] = m[1]
			}
		}
	}
	if err == nil {
		t.Fatal("expected errors, compiled successfully")
	}
	got := map[string][]string{}
	for _, line := range strings.Split(err.Error(), "\n") {
		// path:line:col: message
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			t.Errorf("malformed diagnostic: %s", line)
			continue
		}
		at := filepath.Base(parts[0]) + ":" + parts[1]
		got[at] = append(got[at], strings.TrimSpace(parts[3]))
	}
	for at, sub := range want {
		found := false
		for _, msg := range got[at] {
			if strings.Contains(msg, sub) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want error containing %q, got %q", at, sub, got[at])
		}
	}
	for at, msgs := range got {
		if _, ok := want[at]; !ok {
			t.Errorf("%s: unexpected errors %q", at, msgs)
		}
	}
}

// TestLibraryErrors checks diagnostics about a library's declarations,
// which kc reads from class files: testdata/liberrors/X/lib/ is compiled
// first (its Java files by javac), and X/lib2/, if there is one, as a
// second library module after it, then X/use/ against them, with
// "// ERROR <substring>" comments as in TestErrors. A first line
// "// kotlinc: <flags>" of either's first file gives its flags. lib2 is
// compiled against the standard library alone, and use against every
// module, unless the first line of its first file is "// deps: <modules>",
// which names the modules on its class path: a module that a dependency
// depends on is then missing from it.
func TestLibraryErrors(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	dirs, _ := filepath.Glob("../../testdata/liberrors/*")
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			cp := []string{lib}
			outs := map[string]string{}
			for _, name := range []string{"lib", "lib2"} {
				libSrcs, _ := filepath.Glob(filepath.Join(dir, name, "*.kt"))
				if len(libSrcs) == 0 && name != "lib" {
					continue
				}
				libOut := t.TempDir()
				javas, _ := filepath.Glob(filepath.Join(dir, name, "*.java"))
				libCP := append([]string{lib}, moduleDeps(libSrcs[0], outs, nil)...)
				libOpts := Options{OutDir: libOut, Jobs: 1, ClassPath: libCP, ModuleName: name}
				flagOptions(&libOpts, kotlincFlags(libSrcs[0]))
				if err := Compile(append(libSrcs, javas...), libOpts); err != nil {
					t.Fatalf("compile %s: %v", name, err)
				}
				if len(javas) > 0 {
					javac, err := exec.LookPath("javac")
					if err != nil {
						t.Skip("javac not found on PATH")
					}
					javaCP := strings.Join(append([]string{libOut}, libCP...), string(os.PathListSeparator))
					args := append([]string{"-nowarn", "-cp", javaCP, "-d", libOut}, javas...)
					if out, err := exec.Command(javac, args...).CombinedOutput(); err != nil {
						t.Fatalf("javac: %v\n%s", err, out)
					}
				}
				outs[name] = libOut
				cp = append(cp, libOut)
			}
			srcs, _ := filepath.Glob(filepath.Join(dir, "use", "*.kt"))
			cp = append([]string{lib}, moduleDeps(srcs[0], outs, cp[1:])...)
			opts := Options{OutDir: t.TempDir(), Jobs: 1, JavaTypes: true, ClassPath: cp}
			flagOptions(&opts, kotlincFlags(srcs[0]))
			checkErrors(t, srcs, Compile(srcs, opts))
		})
	}
}

// TestLibrary compiles each library test, testdata/lib/X/: kc compiles
// the library, X/lib/, with module name lib, and the program, X/app/, is
// compiled against kc's classes, by kc and by kotlinc (if found), which
// both read their Kotlin metadata. Each program runs on the JVM with full
// verification, and its stdout must match X.out, which kotlinc's builds of
// both produce (testdata/gen_expected.sh). A first line "// kotlinc:
// <flags>" of the library's first file, or of the program's main.kt, sets
// its -jvm-default mode.
func TestLibrary(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, _ := exec.LookPath("kotlinc")
	dirs, err := filepath.Glob("../../testdata/lib/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			continue
		}
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(dir + ".out")
			if err != nil {
				t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
			}
			libOut := t.TempDir()
			deps := []string{lib}
			if filepath.Base(dir) == "coroutines" {
				deps = append(deps, kotlinxCoroutines(lib))
			}
			libSrcs := sources(t, filepath.Join(dir, "lib"))
			libOpts := Options{OutDir: libOut, Jobs: runtime.NumCPU(), ClassPath: deps, ModuleName: "lib"}
			flagOptions(&libOpts, kotlincFlags(libSrcs[0]))
			if err := Compile(libSrcs, libOpts); err != nil {
				t.Fatalf("compile library: %v", err)
			}
			run := func(t *testing.T, appOut string) {
				cmd := exec.Command(java, "-Xverify:all", "-cp", strings.Join(append([]string{appOut, libOut}, deps...), string(os.PathListSeparator)), "MainKt")
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("java: %v\n%s", err, stderr.String())
				}
				if got := stdout.String(); got != string(want) {
					t.Errorf("output mismatch\n%s", diff(string(want), got))
				}
			}
			app := sources(t, filepath.Join(dir, "app"))
			appFlags := kotlincFlags(filepath.Join(dir, "app", "main.kt"))
			t.Run("kc", func(t *testing.T) {
				appOut := t.TempDir()
				opts := Options{OutDir: appOut, Jobs: runtime.NumCPU(), ClassPath: append(deps, libOut)}
				flagOptions(&opts, appFlags)
				if err := Compile(app, opts); err != nil {
					t.Fatalf("compile program: %v", err)
				}
				run(t, appOut)
			})
			t.Run("kotlinc", func(t *testing.T) {
				if kotlinc == "" {
					t.Skip("kotlinc not found on PATH")
				}
				appOut := t.TempDir()
				cp := strings.Join(append([]string{libOut}, deps[1:]...), string(os.PathListSeparator))
				args := append(append([]string{"-nowarn", "-cp", cp, "-d", appOut}, appFlags...), app...)
				// kotlinc's own assertions check the bytecode of kc's inline functions as kotlinc's inliner reads it.
				cmd := exec.Command(kotlinc, args...)
				cmd.Env = append(os.Environ(), "JAVA_OPTS=-ea")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("kotlinc: %v\n%s", err, out)
				}
				run(t, appOut)
			})
		})
	}
}

// TestSameModuleName compiles a program against two libraries with the
// same module name, so the same META-INF/common.kotlin_module, as Gradle
// projects name modules after their directories: the top-level functions
// of both are visible.
func TestSameModuleName(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	tmp := t.TempDir()
	write := func(name, src string) string {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	var outs []string
	for i, src := range []string{"package a\n\nfun hello() = \"hello\"\n", "package b\n\nfun world() = \"world\"\n"} {
		out := filepath.Join(tmp, fmt.Sprint("lib", i))
		f := write(fmt.Sprint("lib", i, ".kt"), src)
		if err := Compile([]string{f}, Options{OutDir: out, Jobs: 1, ClassPath: []string{lib}, ModuleName: "common"}); err != nil {
			t.Fatalf("compile library %d: %v", i, err)
		}
		outs = append(outs, out)
	}
	app := write("main.kt", "import a.hello\nimport b.world\n\nfun main() {\n    println(hello() + \" \" + world())\n}\n")
	appOut := filepath.Join(tmp, "app")
	cp := append([]string{lib}, outs...)
	if err := Compile([]string{app}, Options{OutDir: appOut, Jobs: 1, ClassPath: cp}); err != nil {
		t.Fatalf("compile program: %v", err)
	}
	out, err := exec.Command(java, "-cp", strings.Join(append([]string{appOut}, cp...), string(os.PathListSeparator)), "MainKt").Output()
	if err != nil {
		t.Fatalf("java: %v", err)
	}
	if string(out) != "hello world\n" {
		t.Errorf("output %q, want %q", out, "hello world\n")
	}
}

// TestJVMTarget compiles a library for JVM 11: its class files have
// version 55, and its inline function and inline property accessors are
// errors in a program compiled for 1.8, as kotlinc's
// INLINE_FROM_HIGHER_PLATFORM, and are inlined in one compiled for 11.
func TestJVMTarget(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	tmp := t.TempDir()
	write := func(name, src string) string {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	libSrc := write("lib.kt", `package a

inline fun twice(f: () -> String) = f() + f()

@PublishedApi internal var stored = 0

var count: Int
    inline get() = stored + 1
    set(v) { stored = v }

class C {
    inline val name: String get() = "c"
}
`)
	libOut := filepath.Join(tmp, "lib")
	if err := Compile([]string{libSrc}, Options{OutDir: libOut, Jobs: 1, ClassPath: []string{lib}, JVMTarget: "11"}); err != nil {
		t.Fatalf("compile library: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(libOut, "a", "LibKt.class"))
	if err != nil {
		t.Fatal(err)
	}
	if major := classfile.Major(data); major != 55 {
		t.Errorf("library class version %d, want 55", major)
	}
	app := write("main.kt", `import a.*

fun main() {
    count = 1 // the setter isn't inline
    val x = twice { "x" }
    val n = count
    println(x + n + C().name)
}
`)
	cp := []string{lib, libOut}
	err = Compile([]string{app}, Options{OutDir: filepath.Join(tmp, "app8"), Jobs: 1, ClassPath: cp})
	if err == nil {
		t.Fatal("compiled for JVM 1.8, want errors")
	}
	var lines []string
	for _, line := range strings.Split(err.Error(), "\n") {
		// path:line:col: message
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 || !strings.Contains(parts[3], "cannot inline bytecode built with JVM target 11 into bytecode that is being built with JVM target 1.8") {
			t.Errorf("unexpected error %s", line)
			continue
		}
		lines = append(lines, parts[1])
	}
	if want := []string{"5", "6", "7"}; !slices.Equal(lines, want) {
		t.Errorf("errors on lines %v, want %v (twice, count's getter and C.name)", lines, want)
	}
	appOut := filepath.Join(tmp, "app11")
	if err := Compile([]string{app}, Options{OutDir: appOut, Jobs: 1, ClassPath: cp, JVMTarget: "11"}); err != nil {
		t.Fatalf("compile program for JVM 11: %v", err)
	}
	out, err := exec.Command(java, "-cp", strings.Join(append([]string{appOut}, cp...), string(os.PathListSeparator)), "MainKt").Output()
	if err != nil {
		t.Fatalf("java: %v", err)
	}
	if string(out) != "xx2c\n" {
		t.Errorf("output %q, want %q", out, "xx2c\n")
	}
	if data, err := os.ReadFile(filepath.Join(appOut, "MainKt.class")); err != nil || classfile.Major(data) != 55 {
		t.Errorf("program class version %d (%v), want 55", classfile.Major(data), err)
	}
	if err := Compile([]string{app}, Options{OutDir: appOut, Jobs: 1, ClassPath: cp, JVMTarget: "8"}); err == nil {
		t.Errorf("JVM target 8 accepted, want an error: kotlinc's is 1.8")
	}
}

// TestSupertypeCasts runs a program against a version of its library in
// which a class no longer extends the class the program uses it as: the
// program's class still verifies, since a value used as another class is
// cast to it, as in kotlinc's code, and fails only where that code runs.
func TestSupertypeCasts(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	tmp := t.TempDir()
	compile := func(name, src string, cp ...string) string {
		path := filepath.Join(tmp, name+".kt")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(tmp, name)
		if err := Compile([]string{path}, Options{OutDir: out, Jobs: 1, ClassPath: append([]string{lib}, cp...)}); err != nil {
			t.Fatalf("compile %s: %v", name, err)
		}
		return out
	}
	const library = `package lib

open class Descriptor

class Adapter {
    companion object%s
}

object Plain%s

class Storage {
    var last: Descriptor? = null

    fun register(descriptor: Descriptor) {
        last = descriptor
    }
}
`
	v1 := compile("v1", fmt.Sprintf(library, " : Descriptor()", " : Descriptor()"))
	v2 := compile("v2", fmt.Sprintf(library, "", ""))
	app := compile("main", `import lib.Adapter
import lib.Descriptor
import lib.Plain
import lib.Storage

class OldApi {
    val first: Descriptor = Plain

    fun register(storage: Storage) {
        storage.register(Adapter)
        storage.last = Plain
    }

    fun descriptor(): Descriptor = Adapter
}

fun main() {
    val failed = try {
        OldApi()
        false
    } catch (e: ClassCastException) {
        true
    }
    println(failed)
}
`, v1)
	for i, v := range []string{v1, v2} {
		cp := strings.Join([]string{app, v, lib}, string(os.PathListSeparator))
		out, err := exec.Command(java, "-Xverify:all", "-cp", cp, "MainKt").CombinedOutput()
		if err != nil {
			t.Fatalf("java, library %d: %v\n%s", i+1, err, out)
		}
		if want := fmt.Sprintln(i == 1); string(out) != want {
			t.Errorf("library %d: output %q, want %q", i+1, out, want)
		}
	}
}

// TestCompanionAccessors checks that a companion object's code reads and
// writes its properties whose fields the outer class holds through their
// accessors, as kotlinc's does, so a mock of the companion answers for
// them, and the private fields of those with a private accessor through
// the outer class's access$getX$cp and access$setX$cp.
func TestCompanionAccessors(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	tmp := t.TempDir()
	src := filepath.Join(tmp, "manager.kt")
	if err := os.WriteFile(src, []byte(`class Manager {
    companion object {
        @JvmStatic
        var existing: Manager? = null
            private set

        var count = 0

        private var hidden = 0

        fun viaGetter(): Manager? = existing

        fun viaSetter() {
            count = count + 1
        }

        fun direct() {
            existing = Manager()
            hidden = hidden + 1
        }
    }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out")
	if err := Compile([]string{src}, Options{OutDir: out, Jobs: 1, ClassPath: []string{lib}}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "Manager$Companion.class"))
	if err != nil {
		t.Fatal(err)
	}
	ms, err := classfile.ReadMethods(data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"viaGetter": {"getExisting"},
		"viaSetter": {"getCount", "setCount"},
		"direct":    {"access$setExisting$cp", "access$getHidden$cp", "access$setHidden$cp"},
	}
	for _, m := range ms {
		names, ok := want[m.Name]
		if !ok {
			continue
		}
		insns, err := m.Decode()
		if err != nil {
			t.Fatalf("%s: %v", m.Name, err)
		}
		var got []string
		for i := range insns {
			switch in := &insns[i]; in.Op {
			case classfile.INVOKEVIRTUAL, classfile.INVOKESTATIC, classfile.GETSTATIC, classfile.PUTSTATIC:
				if in.Owner == "Manager" || in.Owner == "Manager$Companion" {
					got = append(got, in.Name)
				}
			}
		}
		if !slices.Equal(got, names) {
			t.Errorf("%s uses %v, want %v", m.Name, got, names)
		}
		delete(want, m.Name)
	}
	if len(want) > 0 {
		t.Errorf("methods not found: %v", want)
	}
}

// TestKotlincLibrary compiles each library test of testdata/klib/X/ the
// other way round from TestLibrary: kotlinc compiles the library, X/lib/,
// with module name lib and the flags of the first line of its first file,
// "// kotlinc: <flags>" (a JVM target, for the bytecode kc then inlines),
// and kc compiles the program, X/app/, against kotlinc's classes. The
// program runs on the JVM with full verification, and its stdout must
// match X.out, which kotlinc's build of both produces
// (testdata/gen_expected.sh).
func TestKotlincLibrary(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	dirs, err := filepath.Glob("../../testdata/klib/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			continue
		}
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(dir + ".out")
			if err != nil {
				t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
			}
			srcs := sources(t, filepath.Join(dir, "lib"))
			slices.Sort(srcs)
			src, err := os.ReadFile(srcs[0])
			if err != nil {
				t.Fatal(err)
			}
			first, _, _ := strings.Cut(string(src), "\n")
			var flags string
			if f, ok := strings.CutPrefix(first, "// kotlinc: "); ok {
				flags = f
			}
			libOut, appOut := t.TempDir(), t.TempDir()
			javas := javaSources(t, filepath.Join(dir, "lib"))
			args := append(append([]string{"-nowarn", "-module-name", "lib", "-d", libOut}, strings.Fields(flags)...), append(srcs, javas...)...)
			if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
				t.Fatalf("kotlinc: %v\n%s", err, out)
			}
			if len(javas) > 0 {
				javac, err := exec.LookPath("javac")
				if err != nil {
					t.Skip("javac not found on PATH")
				}
				if out, err := exec.Command(javac, append([]string{"-nowarn", "-cp", libOut, "-d", libOut}, javas...)...).CombinedOutput(); err != nil {
					t.Fatalf("javac: %v\n%s", err, out)
				}
			}
			cp := []string{lib, libOut}
			opts := Options{OutDir: appOut, Jobs: runtime.NumCPU(), ClassPath: cp}
			flagOptions(&opts, strings.Fields(flags)) // as gen_expected.sh compiles the app
			appSrcs := sources(t, filepath.Join(dir, "app"))
			slices.Sort(appSrcs)
			if src, err := os.ReadFile(appSrcs[0]); err == nil && strings.HasPrefix(string(src), "// friend\n") {
				// The program sees the library's internal declarations.
				opts.FriendPaths = []string{libOut}
			}
			skipPSIGrammar(t, appSrcs...)
			if err := Compile(appSrcs, opts); err != nil {
				t.Fatalf("compile program: %v", err)
			}
			run := func(main string, cp ...string) string {
				cmd := exec.Command(java, "-Xverify:all", "-cp", strings.Join(cp, string(os.PathListSeparator)), main)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("java %s: %v\n%s", main, err, stderr.String())
				}
				return stdout.String()
			}
			got := run("MainKt", append([]string{appOut}, cp...)...)
			if _, err := os.Stat(filepath.Join(dir, "use")); err == nil {
				useSrcs := sources(t, filepath.Join(dir, "use"))
				// A module that kotlinc compiles against kc's classes
				// alone, without the library, as a module's tests against
				// its dependency's: kotlinc inlines kc's inline functions,
				// with assertions on, as Gradle's Kotlin daemon runs.
				useOut := t.TempDir()
				args := append([]string{"-J-ea", "-nowarn", "-cp", strings.Join([]string{lib, appOut}, string(os.PathListSeparator)), "-d", useOut}, useSrcs...)
				if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
					t.Fatalf("kotlinc use: %v\n%s", err, out)
				}
				got += run("UseKt", append([]string{useOut, appOut}, cp...)...)
			}
			if got != string(want) {
				t.Errorf("output mismatch\n%s", diff(string(want), got))
			}
		})
	}
}

// TestSignatures compiles testdata/signatures/ with kotlinc and kc, both
// with -jvm-default=disable and then both with -jvm-default=enable, whose
// interfaces have DefaultImpls classes beside their default methods, and
// compares the generic signatures, the Signature attributes, of the
// classes, fields and methods both builds have.
func TestSignatures(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	srcs := sources(t, "../../testdata/signatures")
	for _, mode := range []string{"disable", "enable"} {
		t.Run(mode, func(t *testing.T) {
			opts := Options{Jobs: runtime.NumCPU(), ClassPath: []string{lib},
				NoJVMDefault: mode == "disable", JVMDefaultCompat: mode == "enable"}
			compareSignatures(t, kotlinc, srcs, mode, opts)
		})
	}
}

func compareSignatures(t *testing.T, kotlinc string, srcs []string, mode string, opts Options) {
	kotlincOut, kcOut := t.TempDir(), t.TempDir()
	args := append([]string{"-nowarn", "-jvm-default=" + mode, "-d", kotlincOut}, srcs...)
	if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc: %v\n%s", err, out)
	}
	opts.OutDir = kcOut
	if err := Compile(srcs, opts); err != nil {
		t.Fatalf("compile: %v", err)
	}
	signatures := func(dir string) map[string]string {
		sigs := map[string]string{}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !strings.HasSuffix(path, ".class") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := classfile.Parse(data)
			if err != nil {
				return err
			}
			sigs[info.Name] = info.Sig
			for _, m := range append(info.Fields, info.Methods...) {
				sigs[info.Name+" "+m.Name+m.Desc] = m.Sig
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return sigs
	}
	want, got := signatures(kotlincOut), signatures(kcOut)
	var keys []string
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	compared := 0
	for _, k := range keys {
		g, ok := got[k]
		if !ok {
			continue // a member only kotlinc writes
		}
		compared++
		if g != want[k] {
			t.Errorf("%s: signature %q, want kotlinc's %q", k, g, want[k])
		}
	}
	if compared < 200 {
		t.Errorf("compared only %d members", compared)
	}
}

// TestLambdaInvoke compiles testdata/signatures/locals.kt with kotlinc
// and kc and compares the methods of the classes of the lambdas that are
// inline functions' default values, which copies of the functions in
// other modules read: invoke takes and returns the lambda's own types,
// and a synthetic bridge with the interface method's calls it.
func TestLambdaInvoke(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	src := "../../testdata/signatures/locals.kt"
	kotlincOut, kcOut := t.TempDir(), t.TempDir()
	if out, err := exec.Command(kotlinc, "-nowarn", "-d", kotlincOut, src).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc: %v\n%s", err, out)
	}
	if err := Compile([]string{src}, Options{Jobs: runtime.NumCPU(), ClassPath: []string{lib}, OutDir: kcOut}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	methods := func(path string) []string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := classfile.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		var ms []string
		for _, m := range info.Methods {
			ms = append(ms, fmt.Sprintf("%s%s %#x", m.Name, m.Desc, m.Access))
		}
		sort.Strings(ms)
		return ms
	}
	classes, err := filepath.Glob(filepath.Join(kotlincOut, "l", "*$dflt*$1.class"))
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) < 10 {
		t.Fatalf("kotlinc wrote only %d lambda classes", len(classes))
	}
	for _, c := range classes {
		name := filepath.Base(c)
		want, got := methods(c), methods(filepath.Join(kcOut, "l", name))
		if !slices.Equal(got, want) {
			t.Errorf("%s: methods\n%s\nwant kotlinc's\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

// TestMembers compiles testdata/members/ with kotlinc and kc, both with
// -jvm-default=disable and then both with -jvm-default=enable, and
// compares the members of the classes both write: each has the same
// fields and methods, by name and descriptor, with the same access flags,
// and each field the same ConstantValue. The methods of lambdas aren't
// compared.
func TestMembers(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	srcs := sources(t, "../../testdata/members")
	methods := func(t *testing.T, dir string) map[string]map[string]uint16 {
		out := map[string]map[string]uint16{}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !strings.HasSuffix(path, ".class") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := classfile.Parse(data)
			if err != nil {
				return err
			}
			ms := map[string]uint16{}
			for _, f := range info.Fields {
				key := f.Name + ":" + f.Desc
				if f.Constant != nil {
					key += fmt.Sprintf(" = %#v", f.Constant)
				}
				ms[key] = f.Access
			}
			for _, m := range info.Methods {
				if !strings.Contains(m.Name, "$lambda$") {
					ms[m.Name+m.Desc] = m.Access
				}
			}
			out[info.Name] = ms
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, mode := range []string{"disable", "enable"} {
		t.Run(mode, func(t *testing.T) {
			kotlincOut, kcOut := t.TempDir(), t.TempDir()
			args := append([]string{"-nowarn", "-jvm-default=" + mode, "-d", kotlincOut}, srcs...)
			if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
				t.Fatalf("kotlinc: %v\n%s", err, out)
			}
			opts := Options{OutDir: kcOut, Jobs: runtime.NumCPU(), ClassPath: []string{lib},
				NoJVMDefault: mode == "disable", JVMDefaultCompat: mode == "enable"}
			if err := Compile(srcs, opts); err != nil {
				t.Fatalf("compile: %v", err)
			}
			want, got := methods(t, kotlincOut), methods(t, kcOut)
			compared := 0
			for _, cls := range slices.Sorted(maps.Keys(want)) {
				gm, ok := got[cls]
				if !ok {
					t.Errorf("%s: only kotlinc writes the class", cls)
					continue
				}
				for _, m := range slices.Sorted(maps.Keys(want[cls])) {
					compared++
					switch access, ok := gm[m]; {
					case !ok:
						t.Errorf("%s.%s: only kotlinc writes the member", cls, m)
					case access != want[cls][m]:
						t.Errorf("%s.%s: access flags %#x, want kotlinc's %#x", cls, m, access, want[cls][m])
					}
				}
				for _, m := range slices.Sorted(maps.Keys(gm)) {
					if _, ok := want[cls][m]; !ok {
						t.Errorf("%s.%s: kotlinc writes no such member", cls, m)
					}
				}
			}
			for _, cls := range slices.Sorted(maps.Keys(got)) {
				if _, ok := want[cls]; !ok {
					t.Errorf("%s: kotlinc writes no such class", cls)
				}
			}
			if compared < 200 {
				t.Errorf("compared only %d members", compared)
			}
		})
	}
}

// TestInnerClasses compiles testdata/innerclasses/ with kotlinc and kc
// and compares the InnerClasses attributes of the classes both write: an
// entry kotlinc writes for a nested class that kc's class file names too
// must be in kc's, with the same outer class, simple name and access
// flags. A Kotlin compiler reading the class file needs them to tell a
// nested class from a top-level one with $ in its name.
// TestPermittedSubclasses compiles testdata/permitted with kotlinc and kc,
// for Java 17 and 1.8, and compares the classes' PermittedSubclasses
// attributes: from Java 17 on, a sealed class's lists its direct
// subclasses.
func TestPermittedSubclasses(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	srcs := sources(t, "../../testdata/permitted")
	read := func(dir string) map[string][]string {
		permitted := map[string][]string{}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !strings.HasSuffix(path, ".class") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := classfile.Parse(data)
			if err != nil {
				return err
			}
			permitted[info.Name] = info.Permitted
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return permitted
	}
	for _, target := range []string{"17", "1.8"} {
		t.Run(target, func(t *testing.T) {
			kotlincOut, kcOut := t.TempDir(), t.TempDir()
			args := append([]string{"-nowarn", "-jvm-target", target, "-d", kotlincOut}, srcs...)
			if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
				t.Fatalf("kotlinc: %v\n%s", err, out)
			}
			if err := Compile(srcs, Options{Jobs: runtime.NumCPU(), ClassPath: []string{lib}, OutDir: kcOut, JVMTarget: target}); err != nil {
				t.Fatalf("compile: %v", err)
			}
			want, got := read(kotlincOut), read(kcOut)
			sealed := 0
			for name, w := range want {
				g, ok := got[name]
				if !ok {
					t.Errorf("%s: no class", name)
					continue
				}
				if !slices.Equal(g, w) {
					t.Errorf("%s: PermittedSubclasses %v, want %v", name, g, w)
				}
				if len(w) > 0 {
					sealed++
				}
			}
			if wantSealed := map[string]int{"17": 6, "1.8": 0}[target]; sealed != wantSealed {
				t.Errorf("%d classes with PermittedSubclasses, want %d", sealed, wantSealed)
			}
		})
	}
}

func TestInnerClasses(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	srcs := sources(t, "../../testdata/innerclasses")
	kotlincOut, kcOut := t.TempDir(), t.TempDir()
	args := append([]string{"-nowarn", "-d", kotlincOut}, srcs...)
	if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc: %v\n%s", err, out)
	}
	if err := Compile(srcs, Options{Jobs: runtime.NumCPU(), ClassPath: []string{lib}, OutDir: kcOut}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	type class struct {
		data    []byte
		entries map[string]classfile.InnerClassEntry
	}
	read := func(dir string) map[string]class {
		classes := map[string]class{}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !strings.HasSuffix(path, ".class") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := classfile.Parse(data)
			if err != nil {
				return err
			}
			details, err := classfile.ParseDetails(data)
			if err != nil {
				return err
			}
			c := class{data, map[string]classfile.InnerClassEntry{}}
			for _, e := range details.InnerClasses {
				c.entries[e.Inner] = e
			}
			classes[info.Name] = c
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return classes
	}
	want, got := read(kotlincOut), read(kcOut)
	var names []string
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	compared := 0
	for _, name := range names {
		g, ok := got[name]
		if !ok {
			continue // a class only kotlinc writes
		}
		var inners []string
		for inner := range want[name].entries {
			inners = append(inners, inner)
		}
		sort.Strings(inners)
		for _, inner := range inners {
			w := want[name].entries[inner]
			if w.Outer == "" || !bytes.Contains(g.data, []byte(inner)) {
				continue // an anonymous or local class, or one kc's class file doesn't name
			}
			compared++
			if e, ok := g.entries[inner]; !ok {
				t.Errorf("%s: no InnerClasses entry for %s, kotlinc's is %+v", name, inner, w)
			} else if e != w {
				t.Errorf("%s: InnerClasses entry %+v, want kotlinc's %+v", name, e, w)
			}
		}
	}
	if compared < 20 {
		t.Errorf("compared only %d entries", compared)
	}
}

// TestParcelize compiles testdata/parcelize/ as kotlinc does with its
// parcelize plugin: javac compiles java/, Android's Parcelable and a fake
// of its Parcel, which logs what is written; kotlinc, with the plugin,
// compiles a library, lib/; and kc, with -parcelize, the program, app/,
// whose Parcelables it writes and reads. kotlinc then compiles use/
// against kc's classes, with the plugin, and runs it after the program.
// The output must match parcelize.out, which kotlinc's build of all of
// them produces (testdata/gen_expected.sh): the encoding is kotlinc's.
func TestParcelize(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	const dir = "../../testdata/parcelize"
	want, err := os.ReadFile(dir + ".out")
	if err != nil {
		t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
	}
	runtimeJar := filepath.Join(filepath.Dir(lib), "parcelize-runtime.jar")
	plugin := "-Xplugin=" + filepath.Join(filepath.Dir(lib), "parcelize-compiler.jar")
	path := func(elems ...string) string { return strings.Join(elems, string(os.PathListSeparator)) }
	javaOut, libOut, appOut, useOut := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	var javas []string
	filepath.WalkDir(filepath.Join(dir, "java"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".java") {
			javas = append(javas, p)
		}
		return err
	})
	if out, err := exec.Command(javac, append([]string{"-d", javaOut}, javas...)...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	kotlincArgs := func(cp, out string, srcs ...string) []string {
		return append([]string{"-nowarn", plugin, "-cp", cp, "-d", out}, srcs...)
	}
	args := kotlincArgs(path(javaOut, runtimeJar), libOut, sources(t, filepath.Join(dir, "lib"))...)
	if out, err := exec.Command(kotlinc, append([]string{"-module-name", "lib"}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc lib: %v\n%s", err, out)
	}
	cp := []string{lib, runtimeJar, javaOut, libOut}
	if err := Compile(sources(t, filepath.Join(dir, "app")), Options{OutDir: appOut, Jobs: runtime.NumCPU(), ClassPath: cp, Parcelize: true}); err != nil {
		t.Fatalf("compile program: %v", err)
	}
	args = kotlincArgs(path(javaOut, runtimeJar, appOut), useOut, sources(t, filepath.Join(dir, "use"))...)
	if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc use: %v\n%s", err, out)
	}
	run := func(main string, cp ...string) string {
		cmd := exec.Command(java, "-Xverify:all", "-cp", path(cp...), main)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("java %s: %v\n%s", main, err, stderr.String())
		}
		return stdout.String()
	}
	// A Parceler is kotlinx.parcelize's, from its runtime.
	got := run("MainKt", appOut, libOut, javaOut, lib, runtimeJar)
	got += run("UseKt", useOut, appOut, libOut, javaOut, lib, runtimeJar)
	if got != string(want) {
		t.Errorf("output mismatch\n%s", diff(string(want), got))
	}

	// A class that kc doesn't write as the plugin does is left to kotlinc:
	// one with a property of a type that the plugin writes with writeValue
	// but isn't annotated @RawValue, as a value class that isn't
	// Parcelable, for which kotlinc reports an error; or with a type
	// parameter whose bound is a collection, which kotlinc fails to
	// compile; or with a custom Parceler that kotlinc rejects, or a
	// companion one of another class; or with a @DataClass type whose
	// constructor the class can't see, or that is another module's data
	// class, which kotlinc fails to compile.
	for _, decl := range []string{
		"class A(val x: V) : Parcelable\n\n@JvmInline\nvalue class V(val n: Int)",
		"class A(val x: @kotlinx.parcelize.DataClass D) : Parcelable\n\ndata class D private constructor(val n: Int)",
		"class A(val x: Any) : Parcelable",
		"class A(val x: List<Any>) : Parcelable",
		"class A(val x: Map<String, Iterable<Int>>) : Parcelable",
		"class A(val x: List<@kotlinx.parcelize.DataClass Pair<Int, Int>>) : Parcelable",
		"class A<T>(val x: T) : Parcelable",
		"class A<L : List<String>>(val x: L) : Parcelable",
		"class A<T, L : Map<String, T>>(val x: @kotlinx.parcelize.RawValue L) : Parcelable",
		"class A(val x: Int) : Parcelable {\n    companion object : kotlinx.parcelize.Parceler<String> {\n" +
			"        override fun create(parcel: Parcel) = \"\"\n        override fun String.write(parcel: Parcel, flags: Int) {}\n    }\n}",
		"class A(val x: @kotlinx.parcelize.WriteWith<P> String) : Parcelable",
		"class A(val x: @kotlinx.parcelize.WriteWith<Q> Int) : Parcelable",
		"@kotlinx.parcelize.TypeParceler<Int, P>()\n@kotlinx.parcelize.TypeParceler<Int, P>()\nclass A(val x: Int) : Parcelable",
	} {
		t.Run("unsupported "+decl, func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "a.kt")
			os.WriteFile(src, []byte("import android.os.Parcel\nimport android.os.Parcelable\nimport kotlinx.parcelize.Parcelize\n\n"+
				"object P : kotlinx.parcelize.Parceler<Int> {\n"+
				"    override fun create(parcel: Parcel) = parcel.readInt()\n"+
				"    override fun Int.write(parcel: Parcel, flags: Int) = parcel.writeInt(this)\n}\n\n"+
				"abstract class Q : kotlinx.parcelize.Parceler<Int>\n\n"+
				"@Parcelize\n"+decl+"\n"), 0o644)
			err := Compile([]string{src}, Options{OutDir: t.TempDir(), Jobs: 1, ClassPath: cp, Parcelize: true})
			var se *syntax.Error
			if !errors.As(err, &se) || se.Feature != "parcelize" {
				t.Errorf("got %v, want a parcelize error", err)
			}
		})
	}

	// A constructor parameter that isn't a property is an error, as with
	// kotlinc, unless a Parceler companion object writes the class.
	t.Run("parameter not a property", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "a.kt")
		os.WriteFile(src, []byte("import android.os.Parcelable\nimport kotlinx.parcelize.Parcelize\n\n"+
			"@Parcelize\nclass A(val x: Int, y: Int) : Parcelable\n"), 0o644)
		err := Compile([]string{src}, Options{OutDir: t.TempDir(), Jobs: 1, ClassPath: cp, Parcelize: true})
		var se *syntax.Error
		if !errors.As(err, &se) || se.Feature != "" || !strings.Contains(err.Error(), "'Parcelable' constructor parameter should be 'val' or 'var'") {
			t.Errorf("got %v, want the val or var error", err)
		}
	})
}

// TestJavaSources compiles testdata/javasrc/, a module of Kotlin and Java
// sources that use each other, as kotlinc does: kc reads the Java
// sources' declarations and compiles the Kotlin, and javac then compiles
// the Java against kc's classes. The program's output must match
// javasrc.out, from kotlinc's build (testdata/gen_expected.sh).
func TestJavaSources(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	const dir = "../../testdata/javasrc"
	want, err := os.ReadFile(dir + ".out")
	if err != nil {
		t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
	}
	javas := javaSources(t, dir)
	out := t.TempDir()
	if err := Compile(append(sources(t, dir), javas...), Options{OutDir: out, Jobs: runtime.NumCPU(), ClassPath: []string{lib}}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	cp := strings.Join([]string{out, lib}, string(os.PathListSeparator))
	if out, err := exec.Command(javac, append([]string{"-cp", cp, "-d", out}, javas...)...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	cmd := exec.Command(java, "-Xverify:all", "-cp", cp, "MainKt")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("java: %v\n%s", err, stderr.String())
	}
	if got := stdout.String(); got != string(want) {
		t.Errorf("output mismatch\n%s", diff(string(want), got))
	}
}

// serializationJars returns the class path of kotlinx.serialization's
// runtime, its core and JSON jars: $KC_SERIALIZATION_CP, or the version
// that testdata/serialization.out was generated with in Gradle's cache.
// It returns nil if they aren't found.
func serializationJars() []string {
	if cp := os.Getenv("KC_SERIALIZATION_CP"); cp != "" {
		return filepath.SplitList(cp)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range []string{"core", "json"} {
		jars, _ := filepath.Glob(filepath.Join(home, ".gradle/caches/modules-2/files-2.1/org.jetbrains.kotlinx",
			"kotlinx-serialization-"+a+"-jvm/1.9.0/*/kotlinx-serialization-"+a+"-jvm-1.9.0.jar"))
		if len(jars) == 0 {
			return nil
		}
		out = append(out, jars[0])
	}
	return out
}

// TestSerialization compiles testdata/serialization/app with kc's
// serialization plugin, against a library, testdata/serialization/lib,
// that kotlinc compiles with kotlinc's, and testdata/serialization/use
// with kotlinc against kc's classes. It runs the program, which writes
// and reads its classes as JSON and in a format that reads elements in
// order, and then use's, and compares their output with kotlinc's,
// testdata/serialization.out.
func TestSerialization(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	runtimeJars := serializationJars()
	if runtimeJars == nil {
		t.Skip("kotlinx.serialization 1.9.0 not found in Gradle's cache: set KC_SERIALIZATION_CP to its core and JSON jars")
	}
	const dir = "../../testdata/serialization"
	want, err := os.ReadFile(dir + ".out")
	if err != nil {
		t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
	}
	plugin := "-Xplugin=" + filepath.Join(filepath.Dir(lib), "kotlinx-serialization-compiler-plugin.jar")
	path := func(elems ...string) string { return strings.Join(elems, string(os.PathListSeparator)) }
	rt := path(runtimeJars...)
	libOut, appOut, useOut := t.TempDir(), t.TempDir(), t.TempDir()
	kotlincArgs := func(cp, out string, srcs ...string) []string {
		return append([]string{"-nowarn", plugin, "-cp", cp, "-d", out}, srcs...)
	}
	// The Java classes are compiled by javac, after the Kotlin code that
	// reads their sources.
	javac := func(cp, out string, srcs []string) {
		if len(srcs) == 0 {
			return
		}
		if out, err := exec.Command("javac", append([]string{"-nowarn", "-cp", cp, "-d", out}, srcs...)...).CombinedOutput(); err != nil {
			t.Fatalf("javac: %v\n%s", err, out)
		}
	}
	libJava, appJava := javaSources(t, filepath.Join(dir, "lib")), javaSources(t, filepath.Join(dir, "app"))
	if len(libJava)+len(appJava) > 0 {
		if _, err := exec.LookPath("javac"); err != nil {
			t.Skip("javac not found on PATH")
		}
	}
	args := kotlincArgs(rt, libOut, append(sources(t, filepath.Join(dir, "lib")), libJava...)...)
	if out, err := exec.Command(kotlinc, append([]string{"-module-name", "lib"}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc lib: %v\n%s", err, out)
	}
	javac(path(rt, libOut), libOut, libJava)
	cp := append([]string{lib, libOut}, runtimeJars...)
	opts := Options{OutDir: appOut, Jobs: runtime.NumCPU(), ClassPath: cp, ModuleName: "app", Serialization: true}
	if err := Compile(append(sources(t, filepath.Join(dir, "app")), appJava...), opts); err != nil {
		t.Fatalf("compile program: %v", err)
	}
	javac(path(rt, lib, libOut, appOut), appOut, appJava)
	args = kotlincArgs(path(rt, libOut, appOut), useOut, sources(t, filepath.Join(dir, "use"))...)
	if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc use: %v\n%s", err, out)
	}
	run := func(main string, cp ...string) string {
		cmd := exec.Command(java, "-Xverify:all", "-cp", path(cp...), main)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("java %s: %v\n%s", main, err, stderr.String())
		}
		return stdout.String()
	}
	got := run("app.MainKt", appOut, libOut, rt, lib)
	got += run("UseKt", useOut, appOut, libOut, rt, lib)
	if got != string(want) {
		t.Errorf("output mismatch\n%s", diff(string(want), got))
	}

	// Classes that kc doesn't serialize as the plugin does are left to
	// kotlinc.
	for name, src := range map[string]string{
		"generic value": "@JvmInline\nvalue class A<T>(val x: T)",
		"Date":          "class A(val x: java.util.Date)",
		"Any":           "class A(val x: Any)",
		"Date list":     "class A(val x: List<java.util.Date>)",
		"star array":    "class A(val x: Array<*>)",
		"kept":          "@KeepGeneratedSerializer\nclass A(val x: Int)",
		"interface":     "interface A",
		"local":         "class A(val x: Int)\nfun f() {\n@Serializable class L(val y: Int)\n}",
		"super module":  "class A(val x: Int) : lib.LibBase()",
	} {
		t.Run("unsupported "+name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "a.kt")
			os.WriteFile(file, []byte("import kotlinx.serialization.*\n\n@Serializable\n"+src+"\n"), 0o644)
			err := Compile([]string{file}, Options{OutDir: t.TempDir(), Jobs: 1, ClassPath: cp, Serialization: true})
			var se *syntax.Error
			if !errors.As(err, &se) || se.Feature != "serialization" {
				t.Errorf("got %v, want a serialization error", err)
			}
		})
	}
}

// metroJars returns Metro's runtime jar and its compiler plugin jar:
// $KC_METRO_CP and $KC_METRO_PLUGIN, or the version that
// testdata/metro.out was generated with, 1.6.0, in Gradle's cache. It
// returns "" for either if it isn't found.
func metroJars() (runtimeJar, plugin string) {
	runtimeJar, plugin = os.Getenv("KC_METRO_CP"), os.Getenv("KC_METRO_PLUGIN")
	home, err := os.UserHomeDir()
	if err != nil {
		return runtimeJar, plugin
	}
	cache := filepath.Join(home, ".gradle/caches/modules-2/files-2.1/dev.zacsweers.metro")
	if runtimeJar == "" {
		if jars, _ := filepath.Glob(filepath.Join(cache, "runtime-jvm/1.6.0/*/runtime-jvm-1.6.0.jar")); len(jars) > 0 {
			runtimeJar = jars[0]
		}
	}
	if plugin == "" {
		if jars, _ := filepath.Glob(filepath.Join(cache, "compiler/1.6.0/*/compiler-1.6.0.jar")); len(jars) > 0 {
			plugin = jars[0]
		}
	}
	return runtimeJar, plugin
}

// TestMetro compiles testdata/metro/mid, contributions to a scope, and
// testdata/metro/app, graphs, with kc's Metro plugin, against a library,
// testdata/metro/lib, that kotlinc compiles with Metro's. It runs the
// program and compares its output with kotlinc's, testdata/metro.out.
func TestMetro(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	rt, plugin := metroJars()
	if rt == "" || plugin == "" {
		t.Skip("Metro 1.6.0 not found in Gradle's cache: set KC_METRO_CP to its runtime jar and KC_METRO_PLUGIN to its compiler jar")
	}
	const dir = "../../testdata/metro"
	want, err := os.ReadFile(dir + ".out")
	if err != nil {
		t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
	}
	path := func(elems ...string) string { return strings.Join(elems, string(os.PathListSeparator)) }
	libOut, midOut, appOut := t.TempDir(), t.TempDir(), t.TempDir()
	args := append([]string{"-nowarn", "-jvm-target", "17", "-Xplugin=" + plugin, "-module-name", "lib", "-cp", rt, "-d", libOut}, sources(t, filepath.Join(dir, "lib"))...)
	if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc lib: %v\n%s", err, out)
	}
	cp := []string{lib, rt, libOut}
	// For JVM 17, as gen_expected.sh compiles all three.
	opts := Options{OutDir: midOut, Jobs: runtime.NumCPU(), ClassPath: cp, ModuleName: "mid", Metro: true, JVMTarget: "17"}
	if err := Compile(sources(t, filepath.Join(dir, "mid")), opts); err != nil {
		t.Fatalf("compile mid: %v", err)
	}
	opts = Options{OutDir: appOut, Jobs: runtime.NumCPU(), ClassPath: append(cp, midOut), ModuleName: "app", Metro: true, JVMTarget: "17"}
	if err := Compile(sources(t, filepath.Join(dir, "app")), opts); err != nil {
		t.Fatalf("compile app: %v", err)
	}
	cmd := exec.Command(java, "-Xverify:all", "-cp", path(appOut, midOut, libOut, rt, lib), "app.MainKt")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("java: %v\n%s", err, stderr.String())
	}
	if got := stdout.String(); got != string(want) {
		t.Errorf("output mismatch\n%s", diff(string(want), got))
	}

	const header = "import dev.zacsweers.metro.*\n\n"
	for name, tc := range map[string]struct {
		src, msg string
		feature  bool
		options  map[string]string
	}{
		"missing binding": {src: "class A\n@DependencyGraph interface G { val a: A }", msg: "[Metro/MissingBinding]"},
		"cycle": {src: "@Inject class A(val b: B)\n@Inject class B(val a: A)\n@DependencyGraph interface G { val a: A }",
			msg: "[Metro/DependencyCycle]"},
		"duplicate": {src: "@DependencyGraph interface G { val s: String\n@Provides fun a(): String = \"a\"\n@Provides fun b(): String = \"b\" }",
			msg: "[Metro/DuplicateBinding]"},
		"provided and contributed": {src: "interface S\n@Inject @ContributesBinding(AppScope::class) class I : S\n@DependencyGraph(AppScope::class) interface G { val s: S\n@Provides fun s(): S = object : S {} }",
			msg: "[Metro/DuplicateBinding]"},
		"unscoped graph": {src: "@Inject @SingleIn(AppScope::class) class A\n@DependencyGraph interface G { val a: A }",
			msg: "[Metro/IncompatiblyScopedBindings]"},
		"extension with parameters": {src: "@GraphExtension interface E {\n@GraphExtension.Factory interface F { fun create(@Provides s: String): E }\n}\n@DependencyGraph interface G { val e: E }", msg: "takes parameters"},
		"custom annotations": {src: "@DependencyGraph interface G", feature: true,
			options: map[string]string{"custom-inject": "a/MyInject"}},
		"derived bindings": {src: "@DependencyGraph interface G", feature: true, msg: "enable-derived-bindings",
			options: map[string]string{"enable-derived-bindings": "true"}},
		"companion blocks": {src: "@DependencyGraph interface G", feature: true, msg: "companion-mode",
			options: map[string]string{"companion-mode": "companion-block"}},
		"no companions": {src: "@DependencyGraph interface G", feature: true, msg: "companion-mode",
			options: map[string]string{"companion-mode": "NONE"}},
		"closeable graph":     {src: "@DependencyGraph interface G : CloseableGraph", feature: true, msg: "CloseableGraph"},
		"dynamic graph":       {src: "@DependencyGraph interface G\nfun g() = createDynamicGraph<G>()", feature: true},
		"inferred graph type": {src: "@DependencyGraph interface G { companion object : G by createGraph() }", feature: true},
		"member of its own type": {src: "@Inject class S { @Inject lateinit var s: Lazy<S> }\n@DependencyGraph interface G { val s: S }",
			feature: true},
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "a.kt")
			os.WriteFile(file, []byte(header+tc.src+"\n"), 0o644)
			err := Compile([]string{file}, Options{OutDir: t.TempDir(), Jobs: 1, ClassPath: cp, Metro: true, MetroOptions: tc.options, JVMTarget: "17"})
			var se *syntax.Error
			switch {
			case err == nil:
				t.Errorf("compiled; want an error")
			case tc.feature && (!errors.As(err, &se) || se.Feature != "metro"):
				t.Errorf("got %v, want an unsupported Metro feature", err)
			case !strings.Contains(err.Error(), tc.msg):
				t.Errorf("got %v, want %s", err, tc.msg)
			}
		})
	}
}

// TestMetroClassPath compiles a library and a program that uses its
// graphs and injected classes, each with kc's Metro plugin, and runs the
// program. Each case is a module of its own, so the program's classes
// leave the library's unloaded until Metro reads them.
func TestMetroClassPath(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	lib := stdlib()
	rt, _ := metroJars()
	if lib == "" || rt == "" {
		t.Skip("kotlin-stdlib.jar or Metro 1.6.0's runtime not found")
	}
	const header = "import dev.zacsweers.metro.*\n\n"
	for name, tc := range map[string]struct{ lib, app, want string }{
		"lazy constructor parameter": {
			lib:  "@Inject class Engine { fun power() = 7 }\n@Inject class Car(private val engine: Lazy<Engine>) { fun describe() = \"car \" + engine.value.power() }",
			app:  "@DependencyGraph interface G { val car: Car }\nfun main() { println(createGraph<G>().car.describe()) }",
			want: "car 7\n",
		},
		"graph factory": {
			lib:  "@Inject class Service { fun name() = \"service\" }\n@DependencyGraph interface LibGraph {\nval service: Service\nval tag: String\n@DependencyGraph.Factory interface Factory { fun create(@Provides tag: String): LibGraph }\n}",
			app:  "fun main() {\nval g = createGraphFactory<LibGraph.Factory>().create(\"t\")\nprintln(g.service.name() + \" \" + g.tag)\n}",
			want: "service t\n",
		},
		"graph extension": {
			lib:  "abstract class UserScope private constructor()\n@Inject @SingleIn(UserScope::class) class Session { val id = \"session\" }\n@GraphExtension(UserScope::class) interface UserGraph {\nval session: Session\n@ContributesTo(AppScope::class) @GraphExtension.Factory interface Factory { fun createUser(): UserGraph }\n}",
			app:  "@DependencyGraph(AppScope::class) interface G\nfun main() {\nval u = (createGraph<G>() as UserGraph.Factory).createUser()\nprintln(u.session.id + \" \" + (u.session === u.session))\n}",
			want: "session true\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			libDir, libOut, appDir, appOut := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
			libFile, appFile := filepath.Join(libDir, "lib.kt"), filepath.Join(appDir, "main.kt")
			os.WriteFile(libFile, []byte("package lib\n\n"+header+tc.lib+"\n"), 0o644)
			os.WriteFile(appFile, []byte("package app\n\n"+header+"import lib.*\n\n"+tc.app+"\n"), 0o644)
			opts := Options{OutDir: libOut, Jobs: 1, ClassPath: []string{lib, rt}, ModuleName: "lib", Metro: true, JVMTarget: "17"}
			if err := Compile([]string{libFile}, opts); err != nil {
				t.Fatalf("compile lib: %v", err)
			}
			opts = Options{OutDir: appOut, Jobs: 1, ClassPath: []string{lib, rt, libOut}, ModuleName: "app", Metro: true, JVMTarget: "17"}
			if err := Compile([]string{appFile}, opts); err != nil {
				t.Fatalf("compile app: %v", err)
			}
			cp := strings.Join([]string{appOut, libOut, rt, lib}, string(os.PathListSeparator))
			out, err := exec.Command(java, "-Xverify:all", "-cp", cp, "app.MainKt").CombinedOutput()
			if err != nil {
				t.Fatalf("java: %v\n%s", err, out)
			}
			if string(out) != tc.want {
				t.Errorf("got %q, want %q", out, tc.want)
			}
		})
	}
}

// The options that Metro's Gradle plugin passes by default, in either
// spelling of companion-mode, and the options that only shape Metro's own
// graph implementations or diagnostics compile as no options do.
func TestMetroOptions(t *testing.T) {
	lib := stdlib()
	rt, _ := metroJars()
	if lib == "" || rt == "" {
		t.Skip("kotlin-stdlib.jar or Metro 1.6.0's runtime not found")
	}
	for name, options := range map[string]map[string]string{
		"gradle defaults":       {"enable-derived-bindings": "false", "derived-binding-overlap-severity": "NONE"},
		"companion-object":      {"companion-mode": "companion-object"},
		"COMPANION_OBJECT":      {"companion-mode": "COMPANION_OBJECT"},
		"provider getters":      {"force-provider-getters": "true"},
		"overlap severity":      {"derived-binding-overlap-severity": "ERROR"},
		"provider getters, off": {"force-provider-getters": "false"},
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "a.kt")
			os.WriteFile(file, []byte("import dev.zacsweers.metro.*\n\n@DependencyGraph interface G { val s: String\n@Provides fun s(): String = \"s\" }\n"), 0o644)
			if err := Compile([]string{file}, Options{OutDir: t.TempDir(), Jobs: 1, ClassPath: []string{lib, rt}, Metro: true, MetroOptions: options}); err != nil {
				t.Error(err)
			}
		})
	}
}

// A contribution of a class in the default package, or of an internal
// class, has a hint that names it.
func TestMetroHints(t *testing.T) {
	lib := stdlib()
	rt, _ := metroJars()
	if lib == "" || rt == "" {
		t.Skip("kotlin-stdlib.jar or Metro 1.6.0's runtime not found")
	}
	for name, src := range map[string]string{
		"default package": "import dev.zacsweers.metro.*\n\ninterface I\n@ContributesBinding(AppScope::class) @Inject class Impl : I\n",
		"internal class":  "package a\n\nimport dev.zacsweers.metro.*\n\ninterface I\n@ContributesBinding(AppScope::class) @Inject internal class Impl : I\n",
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "a.kt")
			os.WriteFile(file, []byte(src), 0o644)
			out := t.TempDir()
			if err := Compile([]string{file}, Options{OutDir: out, Jobs: 1, ClassPath: []string{lib, rt}, Metro: true}); err != nil {
				t.Fatal(err)
			}
			hints, _ := filepath.Glob(filepath.Join(out, "metro/hints/*.class"))
			if len(hints) != 1 {
				t.Errorf("got hints %v, want one", hints)
			}
		})
	}
}

// composeJars returns the class path of Compose's runtime: $KC_COMPOSE_CP,
// or the version that testdata/compose.out was generated with in Gradle's
// cache, runtime-android 1.9.2 and runtime-annotation-android 1.9.2, whose
// classes.jars it takes out of the aars into dir, with androidx.collection
// 1.5.0 and kotlinx.coroutines 1.8.1. It returns nil if they aren't found.
func composeJars(t *testing.T, dir string) []string {
	if cp := os.Getenv("KC_COMPOSE_CP"); cp != "" {
		return filepath.SplitList(cp)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	cache := filepath.Join(home, ".gradle/caches/modules-2/files-2.1")
	var found []string
	for _, pattern := range []string{
		"androidx.compose.runtime/runtime-android/1.9.2/*/runtime.aar",
		"androidx.compose.runtime/runtime-annotation-android/1.9.2/*/runtime-annotation.aar",
		"androidx.collection/collection-jvm/1.5.0/*/collection-jvm-1.5.0.jar",
		"org.jetbrains.kotlinx/kotlinx-coroutines-core-jvm/1.8.1/*/kotlinx-coroutines-core-jvm-1.8.1.jar",
	} {
		paths, _ := filepath.Glob(filepath.Join(cache, pattern))
		if len(paths) == 0 {
			return nil
		}
		found = append(found, paths[0])
	}
	// The aars' classes.
	for i, name := range []string{"compose-runtime.jar", "compose-runtime-annotation.jar"} {
		found[i] = extractClasses(t, found[i], filepath.Join(dir, name))
	}
	return found
}

// extractClasses writes the classes.jar of an aar to path, and returns
// path.
func extractClasses(t *testing.T, aarPath, path string) string {
	aar, err := zip.OpenReader(aarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer aar.Close()
	in, err := aar.Open("classes.jar")
	if err != nil {
		t.Fatalf("%s: %v", aarPath, err)
	}
	defer in.Close()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCompose compiles testdata/compose/: javac compiles java/, stubs of
// the Android classes that Compose's runtime uses, and kotlinc, with its
// Compose plugin, a library, lib/, of composables that build a tree of
// nodes, and the function that drives a composition. kc compiles the
// program, app/, which has composables of every shape, with -compose, and
// use/, compiled by kotlinc against the program, composes them, changing
// the state they read. The output, the tree after each change, must match
// compose.out, kotlinc's (testdata/gen_expected.sh). The program's
// classes must have the methods, @Composable annotations and $stable
// fields of kotlinc's.
func TestCompose(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found on PATH")
	}
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH")
	}
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	runtimeJars := composeJars(t, t.TempDir())
	if runtimeJars == nil {
		t.Skip("Compose's runtime 1.9.2 not found in Gradle's cache: set KC_COMPOSE_CP to its runtime, runtime-annotation, androidx.collection and kotlinx.coroutines jars")
	}
	const dir = "../../testdata/compose"
	want, err := os.ReadFile(dir + ".out")
	if err != nil {
		t.Fatalf("missing expected output (run testdata/gen_expected.sh): %v", err)
	}
	plugin := "-Xplugin=" + filepath.Join(filepath.Dir(lib), "compose-compiler-plugin.jar")
	path := func(elems ...string) string { return strings.Join(elems, string(os.PathListSeparator)) }
	rt := path(runtimeJars...)
	javaOut, libOut, appOut, useOut := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	var javas []string
	filepath.WalkDir(filepath.Join(dir, "java"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".java") {
			javas = append(javas, p)
		}
		return err
	})
	if out, err := exec.Command(javac, append([]string{"-d", javaOut}, javas...)...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	kotlincArgs := func(cp, out string, srcs ...string) []string {
		return append([]string{"-nowarn", plugin, "-cp", cp, "-d", out}, srcs...)
	}
	args := kotlincArgs(rt, libOut, sources(t, filepath.Join(dir, "lib"))...)
	if out, err := exec.Command(kotlinc, append([]string{"-module-name", "lib"}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc lib: %v\n%s", err, out)
	}
	cp := append([]string{lib, libOut}, runtimeJars...)
	opts := Options{OutDir: appOut, Jobs: runtime.NumCPU(), ClassPath: cp, ModuleName: "compose-app", Compose: true}
	if err := Compile(sources(t, filepath.Join(dir, "app")), opts); err != nil {
		t.Fatalf("compile program: %v", err)
	}
	args = kotlincArgs(path(rt, libOut, appOut), useOut, sources(t, filepath.Join(dir, "use"))...)
	if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
		t.Fatalf("kotlinc use: %v\n%s", err, out)
	}
	cmd := exec.Command(java, "-Xverify:all", "-cp", path(useOut, appOut, libOut, javaOut, rt, lib), "UseKt")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("java UseKt: %v\n%s", err, stderr.String())
	}
	if got := stdout.String(); got != string(want) {
		t.Errorf("output mismatch\n%s", diff(string(want), got))
	}

	t.Run("shapes", func(t *testing.T) {
		javap, err := exec.LookPath("javap")
		if err != nil {
			t.Skip("javap not found on PATH")
		}
		refOut := t.TempDir()
		args := kotlincArgs(path(rt, libOut), refOut, sources(t, filepath.Join(dir, "app"))...)
		if out, err := exec.Command(kotlinc, append([]string{"-module-name", "compose-app"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("kotlinc app: %v\n%s", err, out)
		}
		classes, _ := filepath.Glob(filepath.Join(refOut, "app", "*.class"))
		ours, _ := filepath.Glob(filepath.Join(appOut, "app", "ComposableSingletons$*.class"))
		for _, c := range ours {
			if _, err := os.Stat(filepath.Join(refOut, "app", filepath.Base(c))); err != nil {
				t.Errorf("kc's %s: kotlinc has none", filepath.Base(c))
			}
		}
		// The keys in the names of a ComposableSingletons class's lambdas
		// are kc's own.
		key := regexp.MustCompile(`ambda\$-?[0-9]+(\$[0-9]+)?`)
		for _, c := range classes {
			name := strings.TrimSuffix(filepath.Base(c), ".class")
			singletons := strings.HasPrefix(name, "ComposableSingletons$")
			if strings.Contains(name, "$") && !singletons {
				continue // kotlinc's copies of inlined lambdas' classes
			}
			want := composeShape(t, javap, refOut, "app."+name)
			got := composeShape(t, javap, appOut, "app."+name)
			if singletons {
				for _, shape := range [][]string{want, got} {
					for i, m := range shape {
						shape[i] = key.ReplaceAllString(m, "ambda$$K")
					}
					sort.Strings(shape)
				}
			}
			if !slices.Equal(want, got) {
				t.Errorf("%s: kotlinc's\n\t%s\nkc's\n\t%s", name, strings.Join(want, "\n\t"), strings.Join(got, "\n\t"))
			}
		}
	})

	// The parcelize plugin's Creator has no $stable field, as kotlinc's.
	t.Run("parcelize", func(t *testing.T) {
		javap, err := exec.LookPath("javap")
		if err != nil {
			t.Skip("javap not found on PATH")
		}
		stubs, refOut, kcOut := t.TempDir(), t.TempDir(), t.TempDir()
		var javas []string
		filepath.WalkDir("../../testdata/parcelize/java", func(p string, d fs.DirEntry, err error) error {
			if err == nil && strings.HasSuffix(p, ".java") {
				javas = append(javas, p)
			}
			return err
		})
		if out, err := exec.Command(javac, append([]string{"-d", stubs}, javas...)...).CombinedOutput(); err != nil {
			t.Fatalf("javac: %v\n%s", err, out)
		}
		file := filepath.Join(t.TempDir(), "a.kt")
		os.WriteFile(file, []byte("import android.os.Parcelable\nimport kotlinx.parcelize.Parcelize\n\n"+
			"@Parcelize\nclass A(val x: Int) : Parcelable\n\n@Parcelize\nclass B<T : Parcelable>(var x: T) : Parcelable\n\n@Parcelize\nobject C : Parcelable\n"), 0o644)
		parcelRuntime := filepath.Join(filepath.Dir(lib), "parcelize-runtime.jar")
		// After Compose's plugin: before it, kotlinc infers the stability
		// of a class from its CREATOR field too.
		args := kotlincArgs(path(rt, parcelRuntime, stubs), refOut, file)
		args = slices.Insert(args, 2, "-Xplugin="+filepath.Join(filepath.Dir(lib), "parcelize-compiler.jar"))
		if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
			t.Fatalf("kotlinc: %v\n%s", err, out)
		}
		opts := Options{OutDir: kcOut, Jobs: 1, ClassPath: append([]string{parcelRuntime, stubs}, cp...), Compose: true, Parcelize: true}
		if err := Compile([]string{file}, opts); err != nil {
			t.Fatalf("compile: %v", err)
		}
		for _, name := range []string{"A", "A$Creator", "B", "B$Creator", "C", "C$Creator"} {
			want := composeShape(t, javap, refOut, name)
			got := composeShape(t, javap, kcOut, name)
			if !slices.Equal(want, got) {
				t.Errorf("%s: kotlinc's\n\t%s\nkc's\n\t%s", name, strings.Join(want, "\n\t"), strings.Join(got, "\n\t"))
			}
		}
	})

	// What kc doesn't compile as the plugin does is left to kotlinc.
	for name, src := range map[string]string{
		"wide lambda":      "val f: @Composable (Int, Int, Int, Int, Int, Int, Int, Int, Int, Int) -> Unit = { _, _, _, _, _, _, _, _, _, _ -> }",
		"reference":        "@Composable fun B() {}\nval f: @Composable () -> Unit = ::B",
		"suspend type":     "fun f(g: @Composable suspend () -> Unit) {}",
		"implemented type": "abstract class C : @Composable () -> Unit",
		"type check":       "fun f(x: Any) = x is @Composable () -> Unit",
		"non-local return": "@Composable fun A() { lib.Row { return } }",
		"return from key":  "@Composable fun A() { androidx.compose.runtime.key(1) { return } }",
		"spread key":       "@Composable fun A(ks: Array<Any>) { androidx.compose.runtime.key(*ks) {} }",
		"local":            "fun f() { @Composable fun A() {} }",
		"interface body":   "interface I { @Composable fun A() {} }",
		"catch":            "@Composable fun A() { try { println() } catch (e: Exception) { lib.Text(\"t\") } }",
		"finally":          "@Composable fun A() { try { println() } finally { lib.Text(\"t\") } }",
	} {
		t.Run("unsupported "+name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "a.kt")
			os.WriteFile(file, []byte("import androidx.compose.runtime.Composable\n\n"+src+"\n"), 0o644)
			err := Compile([]string{file}, Options{OutDir: t.TempDir(), Jobs: 1, ClassPath: cp, Compose: true})
			var se *syntax.Error
			if !errors.As(err, &se) || se.Feature != "compose" {
				t.Errorf("got %v, want a compose error", err)
			}
		})
	}
	// Without -compose, a call of a library's composable is left to
	// kotlinc too, and calls of composables need composable callers, and
	// no try around them.
	for name, tt := range map[string]struct {
		src     string
		compose bool
		msg     string
	}{
		"without -compose":        {"fun f() { lib.Text(\"t\") }", false, ""},
		"lambda without -compose": {"fun f() = lib.drive(listOf()) {}", false, ""},
		"not composable":          {"fun f() { lib.Text(\"t\") }", true, ""},
		"plain lambda":            {"@Composable fun A() { val f = { lib.Text(\"t\") } }", true, ""},
		"try":                     {"@androidx.compose.runtime.Composable fun A() { try { lib.Text(\"t\") } finally {} }", true, "try catch is not supported"},
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "a.kt")
			os.WriteFile(file, []byte(tt.src+"\n"), 0o644)
			err := Compile([]string{file}, Options{OutDir: t.TempDir(), Jobs: 1, ClassPath: cp, Compose: tt.compose})
			if tt.msg == "" {
				tt.msg = "@Composable invocations"
			}
			var se *syntax.Error
			switch {
			case !errors.As(err, &se):
				t.Errorf("got %v, want an error", err)
			case tt.compose && (se.Feature != "" || !strings.Contains(se.Msg, tt.msg)):
				t.Errorf("got %v, want an error for the call", err)
			case !tt.compose && se.Feature != "compose":
				t.Errorf("got %v, want a compose error", err)
			}
		})
	}
}

// composeShape returns the members of a class in dir as javap shows them,
// that TestCompose compares with kotlinc's: the methods' and fields' names
// and descriptors, marked @Composable for composable functions, and the
// class's @StabilityInferred annotation, but for the lambdas' methods and
// the static initializer, which kc's composables don't need. The members
// of a ComposableSingletons class have their generic signatures too.
func composeShape(t *testing.T, javap, dir, class string) []string {
	out, err := exec.Command(javap, "-p", "-v", "-cp", dir, class).Output()
	if err != nil {
		t.Fatalf("javap %s: %v", class, err)
	}
	var shape []string
	body := string(out)
	if i := strings.Index(body, "\n{\n"); i >= 0 {
		body = body[i+3:] // after the constant pool
	}
	for _, block := range strings.Split(body, "\n\n") {
		lines := strings.Split(block, "\n")
		if len(lines) < 2 || !strings.HasPrefix(strings.TrimSpace(lines[1]), "descriptor: ") {
			continue
		}
		decl := strings.TrimSuffix(lines[0], ";")
		if i := strings.IndexByte(decl, '('); i >= 0 {
			decl = decl[:i]
		}
		name := decl[strings.LastIndexByte(decl, ' ')+1:]
		if strings.Contains(name, "$lambda$") || name == "{}" || name == "$values" {
			continue // and an enum's $values, which kc inlines
		}
		m := name + " " + strings.TrimPrefix(strings.TrimSpace(lines[1]), "descriptor: ")
		if strings.Contains(block, "androidx.compose.runtime.Composable\n") || strings.HasSuffix(block, "androidx.compose.runtime.Composable") {
			m += " @Composable"
		}
		if strings.Contains(block, "ACC_VARARGS") {
			m += " varargs"
		}
		if _, sig, ok := strings.Cut(block, "Signature: #"); ok && strings.Contains(class, "ComposableSingletons$") {
			_, sig, _ = strings.Cut(sig, "// ")
			sig, _, _ = strings.Cut(sig, "\n")
			m += " " + sig
		}
		shape = append(shape, m)
	}
	if _, a, ok := strings.Cut(string(out), "androidx.compose.runtime.internal.StabilityInferred("); ok {
		params, _, _ := strings.Cut(strings.TrimSpace(a), "\n")
		shape = append(shape, "@StabilityInferred("+params+")")
	}
	if v := stableInit(body); v != "" {
		shape = append(shape, "$stable = "+v)
	}
	sort.Strings(shape)
	return shape
}

// stableInit returns the code of a class's static initializer, in javap's
// output, that computes the value of its $stable field: constants, other
// classes' $stable fields and ors.
func stableInit(body string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if !strings.Contains(l, "putstatic") || !strings.HasSuffix(l, "Field $stable:I") {
			continue
		}
		var code []string
	scan:
		for j := i - 1; j >= 0; j-- {
			f := strings.Fields(lines[j])
			if len(f) < 2 {
				break
			}
			switch op := f[1]; {
			case op == "getstatic" && strings.HasSuffix(lines[j], ".$stable:I"):
				code = append(code, f[len(f)-1])
			case op == "bipush" || op == "ior" || strings.HasPrefix(op, "iconst_"):
				code = append(code, strings.Join(f[1:], " "))
			default:
				break scan
			}
		}
		slices.Reverse(code)
		return strings.Join(code, "; ")
	}
	return ""
}

// TestSuspendShapes checks the suspend functions of a program: in their
// metadata, they take a Continuation last and return an Object, and the
// functions in tail form, whose suspension points are calls whose results
// they return, have no continuation class.
func TestSuspendShapes(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	out := t.TempDir()
	if err := Compile([]string{"../../testdata/run/stdlibSuspend.kt"}, Options{OutDir: out, Jobs: 1, ClassPath: []string{lib}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "StdlibSuspendKt.class"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := classfile.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := kmeta.ReadPackage(info.Kotlin)
	if err != nil {
		t.Fatal(err)
	}
	funcs, _, _ := p.Names()
	n := 0
	for _, name := range funcs {
		fs, err := p.Functions(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range fs {
			if !f.IsSuspend() {
				continue
			}
			n++
			if !strings.HasSuffix(f.JVM.Desc, "Lkotlin/coroutines/Continuation;)Ljava/lang/Object;") {
				t.Errorf("%s: descriptor %s", name, f.JVM.Desc)
			}
		}
	}
	if n < 20 {
		t.Errorf("%d suspend functions in metadata", n)
	}
	for _, f := range []string{"later", "fail", "now", "tailValue", "tailIf", "tailReturn", "tailUnit", "context"} {
		if _, err := os.Stat(filepath.Join(out, "StdlibSuspendKt$"+f+"$1.class")); err == nil {
			t.Errorf("%s has a continuation class", f)
		}
	}
	for _, f := range []string{"kinds", "loops", "tries", "main", "locals$twice"} {
		if _, err := os.Stat(filepath.Join(out, "StdlibSuspendKt$"+f+"$1.class")); err != nil {
			t.Errorf("%s has no continuation class", f)
		}
	}
}

// coroutinePrograms are the test programs whose coroutine classes
// TestCoroutineShapes compares with kotlinc's.
func coroutinePrograms() []string {
	var out []string
	for _, pat := range []string{"stdlibSuspend*.kt", "stdlibSequences*.kt", "stdlibCoroutines*.kt"} {
		files, _ := filepath.Glob("../../testdata/run/" + pat)
		out = append(out, files...)
	}
	return out
}

// TestCoroutineShapes compiles the coroutine test programs with kc and
// with kotlinc, and compares the classes of their coroutines: the
// continuation classes of suspend functions and the classes of suspend
// lambdas, by name, supertype, fields and @DebugMetadata (but for its
// next lines, nl, which come from line tables). The differences
// must be the ones testdata/coroutine-shapes.golden lists, each
// explained there.
func TestCoroutineShapes(t *testing.T) {
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err != nil {
		t.Skip("kotlinc not found on PATH")
	}
	golden, err := os.ReadFile("../../testdata/coroutine-shapes.golden")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, l := range strings.Split(string(golden), "\n") {
		if l != "" && !strings.HasPrefix(l, "#") {
			allowed[l] = true
		}
	}
	var mu sync.Mutex
	seen := map[string]bool{}
	t.Run("programs", func(t *testing.T) {
		for _, f := range coroutinePrograms() {
			t.Run(filepath.Base(f), func(t *testing.T) {
				t.Parallel()
				cp := []string{lib}
				if strings.HasPrefix(filepath.Base(f), "stdlibCoroutines") {
					cp = append(cp, kotlinxCoroutines(lib))
				}
				kc, ref := t.TempDir(), t.TempDir()
				flags := kotlincFlags(f)
				opts := Options{OutDir: kc, Jobs: 1, ClassPath: cp}
				flagOptions(&opts, flags)
				if err := Compile([]string{f}, opts); err != nil {
					t.Fatal(err)
				}
				args := append(flags, "-nowarn", "-cp", strings.Join(cp, string(os.PathListSeparator)), "-d", ref, f)
				if out, err := exec.Command(kotlinc, args...).CombinedOutput(); err != nil {
					t.Fatalf("kotlinc: %v\n%s", err, out)
				}
				want, got := coroutineShapes(t, ref), coroutineShapes(t, kc)
				for l := range want {
					if !got[l] {
						d := "- " + l
						mu.Lock()
						seen[d] = true
						mu.Unlock()
						if !allowed[d] {
							t.Errorf("%s", d)
						}
					}
				}
				for l := range got {
					if !want[l] {
						d := "+ " + l
						mu.Lock()
						seen[d] = true
						mu.Unlock()
						if !allowed[d] {
							t.Errorf("%s", d)
						}
					}
				}
			})
		}
	})
	if t.Failed() {
		return
	}
	for l := range allowed {
		if !seen[l] {
			t.Errorf("golden difference no longer seen: %s", l)
		}
	}
}

// coroutineShapes describes the coroutine classes in a directory of class
// files, a line per fact.
func coroutineShapes(t *testing.T, dir string) map[string]bool {
	out := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".class") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := classfile.Parse(data)
		if err != nil {
			return err
		}
		switch info.Super {
		case "kotlin/coroutines/jvm/internal/ContinuationImpl", "kotlin/coroutines/jvm/internal/SuspendLambda",
			"kotlin/coroutines/jvm/internal/RestrictedSuspendLambda":
		default:
			return nil
		}
		c := info.Name
		out[c+" extends "+info.Super] = true
		for _, f := range info.Fields {
			out[fmt.Sprintf("%s field %s %s %#x", c, f.Name, f.Desc, f.Access)] = true
		}
		keys := make([]string, 0, len(info.DebugMetadata))
		for k := range info.DebugMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k == "nl" {
				// The lines after the suspension points, which come from
				// kotlinc's line tables before its state machine
				// transformation, unlike kc's.
				continue
			}
			out[fmt.Sprintf("%s debug %s=%s", c, k, strings.Join(info.DebugMetadata[k], ","))] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// kotlinxCoroutines returns the path of kotlinx-coroutines-core-jvm.jar,
// next to the standard library's.
func kotlinxCoroutines(lib string) string {
	return filepath.Join(filepath.Dir(lib), "kotlinx-coroutines-core-jvm.jar")
}
